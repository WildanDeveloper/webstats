package ingest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/geo"
	"github.com/webstats/backend/internal/ua"
)

const RedisList = "webstats:pageviews"
const RedisProcessing = "webstats:pageviews:processing"

var ErrBufferFull = errors.New("ingest buffer full")

type Record struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	SiteID    string            `json:"site_id"`
	SessionID string            `json:"session_id"`
	Path      string            `json:"path"`
	Title     string            `json:"title"`
	Referrer  string            `json:"referrer"`
	UA        string            `json:"ua"`
	Screen    string            `json:"screen"`
	Lang      string            `json:"lang"`
	Country   string            `json:"country"`
	ISP       string            `json:"isp"`
	IPHash    string            `json:"ip_hash"`
	IP        string            `json:"ip"`
	Ts        time.Time         `json:"ts"`
	EventName string            `json:"event_name"`
	URL       string            `json:"url"`
	Props     map[string]any    `json:"props"`
	UTM       map[string]string `json:"utm"`
}

type Buffer struct {
	cfg           *config.Config
	db            *pgxpool.Pool
	geo           *geo.Resolver
	asn           *geo.ASNResolver
	redis         *redis.Client
	ch            chan Record
	mu            sync.Mutex
	siteIDs       map[string]siteIDEntry
	hashing       map[string]boolEntry
	limiter       *siteLimiter
	stop          chan struct{}
	done          chan struct{}
	runOnce       sync.Once
	stopOnce      sync.Once
	acceptMu      sync.RWMutex
	stopped       bool
	stopErr       error
	persist       func(context.Context, []Record) error
	schemaMu      sync.Mutex
	schemaReady   bool
	retryEvery    time.Duration
	shutdownGrace time.Duration
}

type siteIDEntry struct {
	id     string
	domain string
	at     time.Time
}

type boolEntry struct {
	v  bool
	at time.Time
}

const cacheTTL = 5 * time.Minute

func NewBuffer(cfg *config.Config, db *pgxpool.Pool, g *geo.Resolver, a *geo.ASNResolver) *Buffer {
	b := &Buffer{
		cfg:     cfg,
		db:      db,
		geo:     g,
		asn:     a,
		ch:      make(chan Record, cfg.BufferSize),
		siteIDs: map[string]siteIDEntry{},
		hashing: map[string]boolEntry{},
		limiter: newSiteLimiter(cfg.RateLimitPerMin),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Fatalf("invalid REDIS_URL: %v", err)
		}
		b.redis = redis.NewClient(opt)
	}
	return b
}

func (b *Buffer) IPHashing(ctx context.Context, siteID string) bool {
	b.mu.Lock()
	if e, ok := b.hashing[siteID]; ok && time.Since(e.at) < cacheTTL {
		v := e.v
		b.mu.Unlock()
		return v
	}
	b.mu.Unlock()
	var ok bool
	err := b.db.QueryRow(ctx, `SELECT ip_hashing FROM site_settings WHERE site_id = $1`, siteID).Scan(&ok)
	if err != nil {
		ok = true
	}
	b.mu.Lock()
	b.hashing[siteID] = boolEntry{v: ok, at: time.Now()}
	b.mu.Unlock()
	return ok
}

func (b *Buffer) SiteExists(ctx context.Context, key string) bool {
	return b.SiteID(ctx, key) != ""
}

func (b *Buffer) SiteID(ctx context.Context, key string) string {
	id, _ := b.SiteInfo(ctx, key)
	return id
}

// SiteInfo resolves a site key to its id and registered domain (used for
// origin validation). Cached with the same TTL as the id itself.
func (b *Buffer) SiteInfo(ctx context.Context, key string) (string, string) {
	b.mu.Lock()
	if e, ok := b.siteIDs[key]; ok && time.Since(e.at) < cacheTTL {
		id, domain := e.id, e.domain
		b.mu.Unlock()
		return id, domain
	}
	b.mu.Unlock()
	var id, domain string
	err := b.db.QueryRow(ctx, `SELECT id::text, COALESCE(domain, '') FROM sites WHERE site_key = $1`, key).Scan(&id, &domain)
	if err != nil || id == "" {
		return "", ""
	}
	b.mu.Lock()
	b.siteIDs[key] = siteIDEntry{id: id, domain: strings.ToLower(strings.TrimSpace(domain)), at: time.Now()}
	b.mu.Unlock()
	return id, domain
}

func (b *Buffer) Stop() error {
	return b.Shutdown(context.Background())
}

// Shutdown stops accepting new records, drains the buffered queue with the
// caller-provided bounded context, and waits for the flusher to finish.
func (b *Buffer) Shutdown(ctx context.Context) error {
	b.Run(context.Background())
	b.stopOnce.Do(func() {
		b.acceptMu.Lock()
		b.stopped = true
		close(b.ch)
		close(b.stop)
		b.acceptMu.Unlock()
	})
	<-b.done
	return b.stopErr
}

func (b *Buffer) Run(ctx context.Context) {
	b.runOnce.Do(func() {
		go b.flusher(ctx)
	})
}

func (b *Buffer) Push(r Record) error {
	b.acceptMu.RLock()
	defer b.acceptMu.RUnlock()
	if b.stopped {
		return errors.New("ingest shutting down")
	}
	if b.redis != nil {
		data, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = b.redis.LPush(ctx, RedisList, data).Err()
		cancel()
		if err == nil {
			return nil
		}
		log.Printf("redis push failed, buffering locally: %v", err)
	}
	select {
	case b.ch <- r:
		return nil
	default:
		return ErrBufferFull
	}
}

func (b *Buffer) flusher(ctx context.Context) {
	defer close(b.done)
	ticker := time.NewTicker(b.cfg.FlushEvery)
	defer ticker.Stop()
	batch := make([]Record, 0, b.cfg.BatchSize)
	retryEvery := b.retryEvery
	if retryEvery <= 0 {
		retryEvery = time.Second
	}
	grace := b.shutdownGrace
	if grace <= 0 {
		grace = 100 * time.Millisecond
	}
	flush := func(ctx context.Context) error {
		if len(batch) == 0 {
			return nil
		}
		persist := b.persist
		if persist == nil {
			persist = b.flush
		}
		if err := persist(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}
	var input chan Record = b.ch
	var retryC <-chan time.Time
	for {
		if len(batch) >= b.cfg.BatchSize {
			input = nil
		}
		select {
		case <-ctx.Done():
			ctx = context.Background()
			b.stopOnce.Do(func() {
				b.acceptMu.Lock()
				b.stopped = true
				close(b.ch)
				close(b.stop)
				b.acceptMu.Unlock()
			})
			input = nil
		case <-b.stop:
			drainCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			for {
				for len(batch) < b.cfg.BatchSize {
					r, ok := <-b.ch
					if !ok {
						break
					}
					batch = append(batch, r)
				}
				if err := flush(drainCtx); err != nil {
					if drainCtx.Err() != nil {
						b.stopErr = fmt.Errorf("shutdown drain incomplete: %d records left: %w", len(batch)+len(b.ch), err)
						cancel()
						return
					}
					select {
					case <-drainCtx.Done():
					case <-time.After(grace):
					}
				} else if len(b.ch) == 0 {
					cancel()
					return
				}
			}
		case <-retryC:
			retryC = nil
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := flush(writeCtx); err != nil {
				log.Printf("flush failed, retaining batch: %v", err)
				timer := time.NewTimer(retryEvery)
				retryC = timer.C
			}
			cancel()
		case r, ok := <-input:
			if !ok {
				input = nil
				continue
			}
			batch = append(batch, r)
			if len(batch) < b.cfg.BatchSize {
				continue
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := flush(writeCtx); err != nil {
				log.Printf("flush failed, retaining batch: %v", err)
				timer := time.NewTimer(retryEvery)
				retryC = timer.C
			}
			cancel()
		case <-ticker.C:
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := flush(writeCtx); err != nil {
				log.Printf("flush failed, retaining batch: %v", err)
				timer := time.NewTimer(retryEvery)
				retryC = timer.C
			}
			cancel()
		}
	}
}

func newRecordID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", id[:])
}

func (b *Buffer) EnsureStorage(ctx context.Context) error {
	b.schemaMu.Lock()
	defer b.schemaMu.Unlock()
	if b.schemaReady {
		return nil
	}
	_, err := b.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS ingest_receipts (id text PRIMARY KEY, site_id uuid NOT NULL REFERENCES sites(id) ON DELETE CASCADE)`)
	if err == nil {
		b.schemaReady = true
	}
	return err
}

func (b *Buffer) flush(ctx context.Context, recs []Record) error {
	if len(recs) == 0 {
		return nil
	}
	if b.db == nil {
		return errors.New("no database pool configured")
	}
	if err := b.EnsureStorage(ctx); err != nil {
		return fmt.Errorf("prepare ingest receipts: %w", err)
	}
	analytics.EnsurePageviewPartitions(ctx, b.db)
	tx, err := b.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	ids := make([]string, 0, len(recs))
	sites := make([]string, 0, len(recs))
	for i := range recs {
		if recs[i].SiteID == "" {
			continue
		}
		if recs[i].ID == "" {
			recs[i].ID = newRecordID()
		}
		ids = append(ids, recs[i].ID)
		sites = append(sites, recs[i].SiteID)
	}
	rows, err := tx.Query(ctx, `INSERT INTO ingest_receipts (id, site_id) SELECT id, site_id::uuid FROM unnest($1::text[], $2::text[]) AS r(id, site_id) ON CONFLICT DO NOTHING RETURNING id`, ids, sites)
	if err != nil {
		return err
	}
	accepted := make(map[string]bool, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		accepted[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	pvs := make([]analytics.PageviewRow, 0, len(recs))
	evs := make([]analytics.EventRowIn, 0, 8)
	for _, r := range recs {
		if !accepted[r.ID] {
			continue
		}
		delete(accepted, r.ID)
		switch r.Kind {
		case "event":
			evs = append(evs, analytics.EventRowIn{
				SiteID: r.SiteID, SessionID: r.SessionID, Name: r.EventName,
				URL: r.URL, Props: r.Props, CreatedAt: r.Ts,
			})
		default:
			info := ua.Parse(r.UA)
			pvs = append(pvs, analytics.PageviewRow{
				SiteID: r.SiteID, SessionID: r.SessionID, Path: r.Path,
				Title: r.Title, Referrer: r.Referrer, ReferrerHost: hostOf(r.Referrer),
				UA: r.UA, Browser: info.Browser, OS: info.OS, Device: info.Device,
				Country: r.Country, Screen: r.Screen, Lang: r.Lang,
				IPHash: r.IPHash, ISP: r.ISP, IP: r.IP, VisitedAt: r.Ts, UTM: r.UTM,
			})
		}
	}

	pvRows := make([][]any, 0, len(pvs))
	for _, r := range pvs {
		pvRows = append(pvRows, []any{
			r.SiteID, r.SessionID, r.Path, r.Title, r.Referrer, r.ReferrerHost,
			r.UA, r.Browser, r.OS, r.Device, r.Country, r.ISP, r.Screen, r.Lang,
			r.IPHash, r.IP, r.VisitedAt, r.UTM["utm_source"], r.UTM["utm_medium"],
			r.UTM["utm_campaign"], r.UTM["utm_content"], r.UTM["utm_term"],
		})
	}
	if len(pvRows) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"pageviews"}, []string{
			"site_id", "session_id", "path", "title", "referrer", "referrer_host",
			"ua", "browser", "os", "device", "country", "isp", "screen", "lang",
			"ip_hash", "ip", "visited_at", "utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term",
		}, pgx.CopyFromRows(pvRows))
		if err != nil {
			return fmt.Errorf("insert pageviews: %w", err)
		}
	}
	evRows := make([][]any, 0, len(evs))
	for _, r := range evs {
		evRows = append(evRows, []any{r.SiteID, r.SessionID, r.Name, r.URL, r.Props, r.CreatedAt})
	}
	if len(evRows) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"events"}, []string{"site_id", "session_id", "name", "url", "props", "created_at"}, pgx.CopyFromRows(evRows))
		if err != nil {
			return fmt.Errorf("insert events: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (b *Buffer) Normalize(raw map[string]any, ip string) Record {
	uaStr := str(raw["ua"])
	cc := ""
	if b.geo != nil {
		cc = b.geo.CountryCode(ip)
	}
	isp := ""
	if b.asn != nil {
		isp = b.asn.Org(ip)
	}
	ts := time.Now().UTC()
	if t, ok := raw["ts"].(float64); ok && t > 0 && t <= float64(ts.UnixMilli()) {
		ts = time.UnixMilli(int64(t)).UTC()
	}
	// Geo/ISP are resolved from the live request IP above, but the raw IP is
	// never persisted. When IP hashing is enabled we keep only a salted hash
	// (used as the visitor identifier); when disabled we keep nothing.
	siteID := str(raw["site_id"])
	ipHash := ""
	if b.IPHashing(context.Background(), siteID) {
		ipHash = hashIP(ip, b.cfg.IPHashSalt)
	}
	referrer := str(raw["referrer"])
	// Referrer spam is dropped at the door: the referrer itself and its
	// derived host are blanked so neither the raw column nor the
	// aggregated referrer_host ever sees the spam domain.
	if spam := hostOf(referrer); isReferrerSpam(spam) {
		referrer = ""
	}
	id := str(raw["id"])
	if id == "" || len(id) > 128 {
		id = newRecordID()
	}
	return Record{
		ID:        siteID + ":" + id,
		Kind:      str(raw["kind"]),
		SiteID:    siteID,
		SessionID: str(raw["session_id"]),
		Path:      str(raw["path"]),
		Title:     str(raw["title"]),
		Referrer:  referrer,
		UA:        uaStr,
		Screen:    str(raw["screen"]),
		Lang:      str(raw["lang"]),
		Country:   cc,
		ISP:       isp,
		IPHash:    ipHash,
		IP:        ipHash,
		Ts:        ts,
		EventName: str(raw["event_name"]),
		URL:       str(raw["url"]),
		Props:     mapAny(raw["props"]),
		UTM: map[string]string{
			"utm_source":   str(raw["utm_source"]),
			"utm_medium":   str(raw["utm_medium"]),
			"utm_campaign": str(raw["utm_campaign"]),
			"utm_content":  str(raw["utm_content"]),
			"utm_term":     str(raw["utm_term"]),
		},
	}
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func mapAny(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return m
}

func hostOf(referrer string) string {
	if referrer == "" {
		return ""
	}
	u, err := url.Parse(referrer)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func (b *Buffer) FlushNow(ctx context.Context, recs []Record) error {
	return b.flush(ctx, recs)
}
