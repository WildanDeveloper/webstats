package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/geo"
	"github.com/webstats/backend/internal/ua"
)

const RedisList = "webstats:pageviews"

type Record struct {
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
	IPHash    string            `json:"ip_hash"`
	Ts        time.Time         `json:"ts"`
	EventName string            `json:"event_name"`
	URL       string            `json:"url"`
	Props     map[string]any    `json:"props"`
	UTM       map[string]string `json:"-"`
}

type Buffer struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	geo     *geo.Resolver
	redis   *redis.Client
	ch      chan Record
	siteIDs map[string]string
	hashing map[string]bool
	stop    chan struct{}
}

func NewBuffer(cfg *config.Config, db *pgxpool.Pool, g *geo.Resolver) *Buffer {
	b := &Buffer{
		cfg:     cfg,
		db:      db,
		geo:     g,
		ch:      make(chan Record, cfg.BufferSize),
		siteIDs: map[string]string{},
		hashing: map[string]bool{},
		stop:    make(chan struct{}),
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
	if v, ok := b.hashing[siteID]; ok {
		return v
	}
	var ok bool
	err := b.db.QueryRow(ctx, `SELECT ip_hashing FROM site_settings WHERE site_id = $1`, siteID).Scan(&ok)
	if err != nil {
		ok = true
	}
	b.hashing[siteID] = ok
	return ok
}

func (b *Buffer) SiteExists(ctx context.Context, key string) bool {
	return b.SiteID(ctx, key) != ""
}

func (b *Buffer) SiteID(ctx context.Context, key string) string {
	if id, ok := b.siteIDs[key]; ok {
		return id
	}
	var id string
	err := b.db.QueryRow(ctx, `SELECT id::text FROM sites WHERE site_key = $1`, key).Scan(&id)
	if err != nil {
		return ""
	}
	b.siteIDs[key] = id
	return id
}

func (b *Buffer) Stop() { close(b.stop) }

func (b *Buffer) Run(ctx context.Context) {
	go b.flusher(ctx)
}

func (b *Buffer) Push(r Record) {
	if b.redis != nil {
		data, err := json.Marshal(r)
		if err == nil {
			if err := b.redis.RPush(context.Background(), RedisList, data).Err(); err != nil {
				log.Printf("redis push failed: %v", err)
			}
			return
		}
	}
	select {
	case b.ch <- r:
	default:
		log.Printf("buffer full, dropping record")
	}
}

func (b *Buffer) flusher(ctx context.Context) {
	ticker := time.NewTicker(b.cfg.FlushEvery)
	defer ticker.Stop()
	batch := make([]Record, 0, b.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := b.flush(ctx, batch); err != nil {
			log.Printf("flush failed: %v", err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-b.stop:
			flush()
			return
		case r := <-b.ch:
			batch = append(batch, r)
			if len(batch) >= b.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (b *Buffer) flush(ctx context.Context, recs []Record) error {
	pvs := make([]analytics.PageviewRow, 0, len(recs))
	evs := make([]analytics.EventRowIn, 0, 8)
	minDay := time.Now()
	maxDay := time.Time{}
	sites := map[string]bool{}

	for _, r := range recs {
		if r.SiteID == "" {
			continue
		}
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
				IPHash: r.IPHash, VisitedAt: r.Ts, UTM: r.UTM,
			})
			sites[r.SiteID] = true
			d := r.Ts.UTC().Truncate(24 * time.Hour)
			if d.Before(minDay) {
				minDay = d
			}
			if d.After(maxDay) {
				maxDay = d
			}
		}
	}

	if err := analytics.InsertPageviews(ctx, b.db, pvs); err != nil {
		return fmt.Errorf("insert pageviews: %w", err)
	}
	if err := analytics.InsertEvents(ctx, b.db, evs); err != nil {
		return fmt.Errorf("insert events: %w", err)
	}
	for site := range sites {
		if err := analytics.AggregateDaily(ctx, b.db, site, minDay, maxDay.Add(24*time.Hour)); err != nil {
			return fmt.Errorf("aggregate: %w", err)
		}
	}
	return nil
}

func (b *Buffer) Normalize(raw map[string]any, ip string) Record {
	uaStr := str(raw["ua"])
	cc := ""
	if b.geo != nil {
		cc = b.geo.CountryCode(ip)
	}
	ts := time.Now().UTC()
	if t, ok := raw["ts"].(float64); ok && t > 0 {
		ts = time.UnixMilli(int64(t)).UTC()
	}
	ipHash := hashIP(ip, b.cfg.IPHashSalt)
	siteID := str(raw["site_id"])
	if !b.IPHashing(context.Background(), siteID) {
		ipHash = ""
	}
	return Record{
		Kind:      str(raw["kind"]),
		SiteID:    siteID,
		SessionID: str(raw["session_id"]),
		Path:      str(raw["path"]),
		Title:     str(raw["title"]),
		Referrer:  str(raw["referrer"]),
		UA:        uaStr,
		Screen:    str(raw["screen"]),
		Lang:      str(raw["lang"]),
		Country:   cc,
		IPHash:    ipHash,
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
