package analytics

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"sort"
	"sync"
	"time"

	"github.com/webstats/backend/internal/geo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/model"
)

type Filters struct {
	Page    string
	Source  string
	Country string
	Device  string
	Browser string
	OS      string
}

func (f Filters) fragment(base int) (string, []any) {
	conds := []string{}
	args := []any{}
	p := func(v, col string) {
		if v == "" {
			return
		}
		switch v {
		case "(direct)", "unknown":
			conds = append(conds, fmt.Sprintf("(%s = '' OR %s IS NULL)", col, col))
		default:
			args = append(args, v)
			conds = append(conds, fmt.Sprintf("%s = $%d", col, base+len(args)))
		}
	}
	p(f.Page, "path")
	p(f.Source, "referrer_host")
	p(f.Country, "country")
	p(f.Device, "device")
	p(f.Browser, "browser")
	p(f.OS, "os")
	if len(conds) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(conds, " AND "), args
}

type PageviewRow struct {
	SiteID       string            `json:"site_id"`
	SessionID    string            `json:"session_id"`
	Path         string            `json:"path"`
	Title        string            `json:"title"`
	Referrer     string            `json:"referrer"`
	ReferrerHost string            `json:"referrer_host"`
	UA           string            `json:"ua"`
	Browser      string            `json:"browser"`
	OS           string            `json:"os"`
	Device       string            `json:"device"`
	Country      string            `json:"country"`
	ISP          string            `json:"isp"`
	Screen       string            `json:"screen"`
	Lang         string            `json:"lang"`
	IPHash       string            `json:"ip_hash"`
	IP           string            `json:"ip"`
	VisitedAt    time.Time         `json:"visited_at"`
	UTM          map[string]string `json:"-"`
}

type EventRowIn struct {
	SiteID    string         `json:"site_id"`
	SessionID string         `json:"session_id"`
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	Props     map[string]any `json:"props"`
	CreatedAt time.Time      `json:"created_at"`
}

// pageviewCols/columns order is shared by the COPY-based bulk inserts below.
var pageviewCols = []string{
	"site_id", "session_id", "path", "title", "referrer", "referrer_host",
	"ua", "browser", "os", "device", "country", "isp", "screen", "lang",
	"ip_hash", "ip", "visited_at",
	"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term",
}

// EnsurePageviewPartitions makes sure monthly partitions exist for the
// current and next month before bulk-inserting. A DEFAULT partition created
// in migration 010 catches everything else, so inserts never fail. Results
// are memoized per process per month.
var (
	pvPartMu     sync.Mutex
	pvLastMonth  string
)

func EnsurePageviewPartitions(ctx context.Context, db *pgxpool.Pool) {
	cur := time.Now().UTC().Truncate(24 * time.Hour)
	cur = time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, time.UTC)
	key := cur.Format("200601")
	pvPartMu.Lock()
	if pvLastMonth == key {
		pvPartMu.Unlock()
		return
	}
	pvLastMonth = key
	pvPartMu.Unlock()
	for _, m := range []time.Time{cur, cur.AddDate(0, 1, 0)} {
		name := pgx.Identifier{"pageviews_" + m.Format("2006_01")}.Sanitize()
		start := m.Format("2006-01-02")
		end := m.AddDate(0, 1, 0).Format("2006-01-02")
		// Best-effort: concurrent creators may hit duplicate_table races.
		_, _ = db.Exec(ctx, fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s PARTITION OF pageviews FOR VALUES FROM ('%s') TO ('%s')`,
			name, start, end))
	}
}

func InsertPageviews(ctx context.Context, db *pgxpool.Pool, rows []PageviewRow) error {
	if len(rows) == 0 {
		return nil
	}
	EnsurePageviewPartitions(ctx, db)
	src := make([][]any, 0, len(rows))
	for _, r := range rows {
		src = append(src, []any{
			r.SiteID, r.SessionID, r.Path, r.Title, r.Referrer, r.ReferrerHost,
			r.UA, r.Browser, r.OS, r.Device, r.Country, r.ISP, r.Screen, r.Lang,
			r.IPHash, r.IP, r.VisitedAt,
			utm(r.UTM, "utm_source"), utm(r.UTM, "utm_medium"), utm(r.UTM, "utm_campaign"),
			utm(r.UTM, "utm_content"), utm(r.UTM, "utm_term"),
		})
	}
	_, err := db.CopyFrom(ctx, pgx.Identifier{"pageviews"}, pageviewCols, pgx.CopyFromRows(src))
	return err
}

func utm(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}

func InsertEvents(ctx context.Context, db *pgxpool.Pool, rows []EventRowIn) error {
	if len(rows) == 0 {
		return nil
	}
	src := make([][]any, 0, len(rows))
	for _, r := range rows {
		src = append(src, []any{r.SiteID, r.SessionID, r.Name, r.URL, r.Props, r.CreatedAt})
	}
	_, err := db.CopyFrom(ctx, pgx.Identifier{"events"},
		[]string{"site_id", "session_id", "name", "url", "props", "created_at"},
		pgx.CopyFromRows(src))
	return err
}

func siteAccess(ctx context.Context, db *pgxpool.Pool, userID, siteID, publicToken string) (bool, error) {
	if publicToken != "" {
		var ok bool
		err := db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM sites WHERE id = $1 AND public_token = $2 AND public_enabled
			)`, siteID, publicToken).Scan(&ok)
		return ok, err
	}
	var ok bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM sites s
			LEFT JOIN site_members m ON m.site_id = s.id
			WHERE s.id = $1 AND (s.user_id = $2 OR m.user_id = $2)
		)`, siteID, userID).Scan(&ok)
	return ok, err
}

func PeriodBounds(period, fromStr, toStr string) (from time.Time, to time.Time, hourly bool) {
	if fromStr != "" && toStr != "" {
		for _, layout := range []string{"2006-01-02 15:04", "2006-01-02"} {
			if f, err := time.Parse(layout, fromStr); err == nil {
				if t, err2 := time.Parse(layout, toStr); err2 == nil {
					return f.UTC(), t.UTC().Add(24 * time.Hour), false
				}
			}
		}
	}
	to = time.Now().UTC()
	switch period {
	case "24h", "today":
		from = time.Now().UTC().Add(-24 * time.Hour)
		hourly = true
	case "7d":
		from = time.Now().UTC().AddDate(0, 0, -7)
	case "30d":
		from = time.Now().UTC().AddDate(0, 0, -30)
	default:
		from = time.Time{}
	}
	return from, to, hourly
}

// earliestVisit returns the oldest pageview timestamp for a site, used to
// clamp open-ended periods ("all") so series generation stays bounded.
func earliestVisit(ctx context.Context, db *pgxpool.Pool, siteID string) time.Time {
	var t time.Time
	_ = db.QueryRow(ctx, `SELECT MIN(visited_at) FROM pageviews WHERE site_id = $1`, siteID).Scan(&t)
	return t
}

// clampFrom resolves a zero "from" (open-ended period) against stored data.
// When there is no data at all it collapses the window so queries return
// empty results instead of scanning unbounded ranges.
func clampFrom(ctx context.Context, db *pgxpool.Pool, siteID string, from, to time.Time) time.Time {
	if !from.IsZero() {
		return from
	}
	if e := earliestVisit(ctx, db, siteID); !e.IsZero() {
		return e
	}
	return to
}

func (q *Queries) Overview(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) (model.Overview, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return model.Overview{}, err
	} else if !ok {
		return model.Overview{}, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)

	var out model.Overview
	prevFrom := from.Add(-(to.Sub(from)))
	cond, fargs := f.fragment(4)
	args := []any{siteID, from, to, prevFrom}
	args = append(args, fargs...)
	err := db.QueryRow(ctx, `
		WITH s AS (
			SELECT session_id, count(*) AS c
			FROM pageviews
			WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`
			GROUP BY session_id
		), sprev AS (
			SELECT session_id, count(*) AS c
			FROM pageviews
			WHERE site_id = $1 AND visited_at >= $4 AND visited_at < $2`+cond+`
			GROUP BY session_id
		)
		SELECT
			(SELECT count(*) FROM pageviews WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`),
			(SELECT count(*) FROM s),
			(SELECT count(*) FROM s WHERE c = 1),
			(SELECT count(*) FROM pageviews WHERE site_id = $1 AND visited_at >= $4 AND visited_at < $2`+cond+`),
			(SELECT count(DISTINCT session_id) FROM pageviews WHERE site_id = $1 AND visited_at >= $4 AND visited_at < $2`+cond+`),
			(SELECT count(*) FROM sprev),
			(SELECT count(*) FROM sprev WHERE c = 1)`,
		args...).Scan(&out.Pageviews, &out.Sessions, &out.Bounces, &out.PrevPageviews, &out.PrevVisitors,
		&out.PrevSessions, &out.PrevBounces)
	if err != nil {
		return out, err
	}
	out.Visitors = out.Sessions
	// The window is [from, to), i.e. exactly to-from long; do not add an
	// extra day or short custom ranges understate avg_per_day.
	days := int64(to.Sub(from).Hours() / 24)
	if days < 1 {
		days = 1
	}
	out.AvgPerDay = float64(out.Pageviews) / float64(days)
	if out.Sessions > 0 {
		out.BounceRate = float64(out.Bounces) / float64(out.Sessions) * 100
	}
	return out, nil
}

func (q *Queries) Timeseries(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) ([]model.TimePoint, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, hourly := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	trunc, layout := "day", "YYYY-MM-DD"
	if hourly {
		trunc, layout = "hour", "YYYY-MM-DD HH24:00"
	}
	cond, fargs := f.fragment(3)
	args := []any{siteID, from, to}
	args = append(args, fargs...)
	rows, err := db.Query(ctx, `
		SELECT to_char(date_trunc('`+trunc+`', visited_at), '`+layout+`'),
		       count(*), count(DISTINCT session_id)
		FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`
		GROUP BY 1 ORDER BY 1`,
		args...)
	if err != nil {
		return nil, err
	}
	curC := map[string]int64{}
	curV := map[string]int64{}
	for rows.Next() {
		var d string
		var n, v int64
		if err := rows.Scan(&d, &n, &v); err != nil {
			rows.Close()
			return nil, err
		}
		curC[d] = n
		curV[d] = v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	goLayout := "2006-01-02"
	if hourly {
		goLayout = "2006-01-02 15:00"
	}
	// Previous-period counts are keyed by the same bucket times shifted back
	// by the window length, so both series stay aligned even when the range
	// is not a whole number of steps (custom from/to with a time component).
	span := to.Sub(from)
	prevFrom := from.Add(-span)
	prevC := map[string]int64{}
	prevV := map[string]int64{}
	if !from.IsZero() {
		pargs := []any{siteID, prevFrom, from}
		pargs = append(pargs, fargs...)
		rows, err := db.Query(ctx, `
			SELECT to_char(date_trunc('`+trunc+`', visited_at), '`+layout+`'),
			       count(*), count(DISTINCT session_id)
			FROM pageviews
			WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`
			GROUP BY 1 ORDER BY 1`,
			pargs...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var d string
			var n, v int64
			if err := rows.Scan(&d, &n, &v); err != nil {
				rows.Close()
				return nil, err
			}
			prevC[d] = n
			prevV[d] = v
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	step := 24 * time.Hour
	if hourly {
		step = time.Hour
	}
	out := make([]model.TimePoint, 0)
	for t := from.Truncate(step); t.Before(to); t = t.Add(step) {
		k := t.Format(goLayout)
		pk := t.Add(-span).Format(goLayout)
		out = append(out, model.TimePoint{
			Date:          k,
			Pageviews:     curC[k],
			Visitors:      curV[k],
			PrevPageviews: prevC[pk],
			PrevVisitors:  prevV[pk],
		})
	}
	return out, nil
}

func (q *Queries) Top(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, column string, limit int, fromStr, toStr string, f Filters) ([]model.Row, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	order := "count(*) DESC"
	fill := "'unknown'"
	switch column {
	case "referrer":
		// Rewrite the column AND the fill value together; checking the
		// original name afterwards would be dead code.
		column = "referrer_host"
		order = "count(*) DESC NULLS LAST"
		fill = "'(direct)'"
	}
	cond, fargs := f.fragment(3)
	args := []any{siteID, from, to}
	args = append(args, fargs...)
	sql := `
		SELECT COALESCE(NULLIF(` + column + `, ''), ` + fill + `) AS key, count(*)
		FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3` + cond + `
		GROUP BY ` + column + ` ORDER BY ` + order + ` LIMIT ` + itoa(int64(limit))
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Row, 0)
	for rows.Next() {
		var r model.Row
		if err := rows.Scan(&r.Key, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) TopEvents(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string) ([]model.EventRow, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	rows, err := db.Query(ctx, `
		SELECT name, count(*), COALESCE(max(created_at)::text, '') FROM events
		WHERE site_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY name ORDER BY count(*) DESC LIMIT 20`,
		siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.EventRow, 0)
	for rows.Next() {
		var r model.EventRow
		if err := rows.Scan(&r.Name, &r.Count, &r.LastAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SessionStats summarizes visitor engagement over the window: how long a
// session lasts, how deep it goes, and how often it is a single-pageview
// bounce. Previous-window deltas let the UI show trends.
type SessionStats struct {
	AvgDurationSec     float64 `json:"avg_duration_sec"`
	MedianDurationSec  float64 `json:"median_duration_sec"`
	AvgPages           float64 `json:"avg_pages"`
	BounceRate         float64 `json:"bounce_rate"`
	Sessions           int64   `json:"sessions"`
	PrevAvgDurationSec float64 `json:"prev_avg_duration_sec"`
	PrevAvgPages       float64 `json:"prev_avg_pages"`
}

func (q *Queries) SessionStats(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) (SessionStats, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return SessionStats{}, err
	} else if !ok {
		return SessionStats{}, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	prevFrom := from.Add(-(to.Sub(from)))
	cond, fargs := f.fragment(4)
	args := []any{siteID, from, to, prevFrom}
	args = append(args, fargs...)
	var out SessionStats
	var sessions, bounces int64
	err := db.QueryRow(ctx, `
		WITH cur AS (
			SELECT session_id,
			       count(*)::int AS pages,
			       extract(epoch FROM max(visited_at) - min(visited_at)) AS dur
			FROM pageviews
			WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`
			GROUP BY session_id
		), prev AS (
			SELECT avg(dur) AS dur, avg(pages) AS pages
			FROM (
				SELECT session_id,
				       extract(epoch FROM max(visited_at) - min(visited_at)) AS dur,
				       count(*)::int AS pages
				FROM pageviews
				WHERE site_id = $1 AND visited_at >= $4 AND visited_at < $2`+cond+`
				GROUP BY session_id
			) x
		)
		SELECT
			COALESCE((SELECT avg(dur) FROM cur), 0),
			COALESCE((SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY dur) FROM cur), 0),
			COALESCE((SELECT avg(pages) FROM cur), 0),
			COALESCE((SELECT count(*) FROM cur WHERE pages = 1), 0),
			COALESCE((SELECT count(*) FROM cur), 0),
			COALESCE((SELECT dur FROM prev), 0),
			COALESCE((SELECT pages FROM prev), 0)`,
		args...).Scan(&out.AvgDurationSec, &out.MedianDurationSec, &out.AvgPages,
		&bounces, &sessions, &out.PrevAvgDurationSec, &out.PrevAvgPages)
	if err != nil {
		return out, err
	}
	out.Sessions = sessions
	if sessions > 0 {
		out.BounceRate = float64(bounces) / float64(sessions) * 100
	}
	return out, nil
}

// SessionBounds returns the first (entry) or last (exit) pageview of every
// session in the window, aggregated per path. kind must be "entry" or "exit".
func (q *Queries) SessionBounds(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr, kind string, limit int, f Filters) ([]model.Row, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	order := "ASC"
	if kind == "exit" {
		order = "DESC"
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	cond, fargs := f.fragment(3)
	args := []any{siteID, from, to}
	args = append(args, fargs...)
	args = append(args, limit)
	limitParam := fmt.Sprintf("$%d", len(args))
	rows, err := db.Query(ctx, `
		WITH ends AS (
			SELECT DISTINCT ON (session_id) path
			FROM pageviews
			WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`
			ORDER BY session_id, visited_at `+order+`
		)
		SELECT path, count(*) FROM ends GROUP BY path ORDER BY count(*) DESC LIMIT `+limitParam,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Row, 0)
	for rows.Next() {
		var r model.Row
		if err := rows.Scan(&r.Key, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type SiteSeries struct {
	SiteID string            `json:"site_id"`
	Name   string            `json:"name"`
	Color  string            `json:"color"`
	Points []model.TimePoint `json:"points"`
}

type RootOverview struct {
	Pageviews int64        `json:"pageviews"`
	Visitors  int64        `json:"visitors"`
	Sites     int64        `json:"sites"`
	Events    int64        `json:"events"`
	Series    []SiteSeries `json:"series"`
	Ranking   []model.SiteRank `json:"ranking"`
}

func (q *Queries) Realtime(ctx context.Context, db *pgxpool.Pool, userID, siteID string) (model.Realtime, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return model.Realtime{}, err
	} else if !ok {
		return model.Realtime{}, pgx.ErrNoRows
	}
	cut := time.Now().UTC().Add(-5 * time.Minute)
	var out model.Realtime
	err := db.QueryRow(ctx, `
		SELECT count(DISTINCT session_id), count(*)
		FROM pageviews WHERE site_id = $1 AND visited_at >= $2`,
		siteID, cut).Scan(&out.Visitors, &out.Pageviews)
	if err != nil {
		return out, err
	}
	rows, err := db.Query(ctx, `
		SELECT COALESCE(path, '/'), count(*) FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 GROUP BY 1 ORDER BY 2 DESC LIMIT 5`, siteID, cut)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Pages = make([]model.Row, 0)
	for rows.Next() {
		var r model.Row
		if err := rows.Scan(&r.Key, &r.Value); err != nil {
			return out, err
		}
		out.Pages = append(out.Pages, r)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = db.Query(ctx, `
		SELECT COALESCE(country, 'unknown'), count(*) FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 GROUP BY 1 ORDER BY 2 DESC LIMIT 5`, siteID, cut)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Countries = make([]model.Row, 0)
	for rows.Next() {
		var r model.Row
		if err := rows.Scan(&r.Key, &r.Value); err != nil {
			return out, err
		}
		out.Countries = append(out.Countries, r)
	}
	return out, rows.Err()
}

func (q *Queries) LatestChecks(ctx context.Context, db *pgxpool.Pool, userID, siteID string, limit int) ([]model.Check, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	rows, err := db.Query(ctx, `
		SELECT id, site_id, status, latency_ms, checked_at
		FROM site_checks WHERE site_id = $1 ORDER BY checked_at DESC LIMIT $2`, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Check, 0)
	for rows.Next() {
		var c model.Check
		if err := rows.Scan(&c.ID, &c.SiteID, &c.Status, &c.LatencyMs, &c.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type Visitor struct {
	IP        string    `json:"ip"`
	SessionID string    `json:"session_id"`
	Country   string    `json:"country"`
	Region    string    `json:"region"`
	City      string    `json:"city"`
	Browser   string    `json:"browser"`
	OS        string    `json:"os"`
	Device    string    `json:"device"`
	Path      string    `json:"path"`
	VisitedAt time.Time `json:"visited_at"`
}

func (q *Queries) RecentVisitors(ctx context.Context, db *pgxpool.Pool, userID, siteID string, limit int) ([]Visitor, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	rows, err := db.Query(ctx, `
		SELECT ip, session_id, country, region, city, browser, os, device, path, visited_at
		FROM (
			SELECT DISTINCT ON (ip)
			       ip, session_id, COALESCE(NULLIF(country, ''), 'unknown') AS country,
			       COALESCE(region, '') AS region, COALESCE(city, '') AS city,
			       COALESCE(NULLIF(browser, ''), 'unknown') AS browser,
			       COALESCE(NULLIF(os, ''), 'unknown') AS os,
			       COALESCE(NULLIF(device, ''), 'unknown') AS device,
			       COALESCE(path, '/') AS path, visited_at
			FROM pageviews
			WHERE site_id = $1 AND ip <> ''
			ORDER BY ip, visited_at DESC
		) latest
		ORDER BY latest.visited_at DESC
		LIMIT $2`, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Visitor, 0)
	for rows.Next() {
		var v Visitor
		if err := rows.Scan(&v.IP, &v.SessionID, &v.Country, &v.Region, &v.City, &v.Browser, &v.OS, &v.Device, &v.Path, &v.VisitedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type VisitorDetail struct {
	IP          string    `json:"ip"`
	ISP         string    `json:"isp"`
	Country     string    `json:"country"`
	Region      string    `json:"region"`
	City        string    `json:"city"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	Browser     string    `json:"browser"`
	OS          string    `json:"os"`
	Device      string    `json:"device"`
	Screen      string    `json:"screen"`
	Lang        string    `json:"lang"`
	SessionID   string    `json:"session_id"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Pageviews   int64     `json:"pageviews"`
	Sessions    int64     `json:"sessions"`
	Paths       []model.Row  `json:"paths"`
	History     []Visitor `json:"history"`
	CountryCode string    `json:"country_code"`
}

func (q *Queries) VisitorDetail(ctx context.Context, db *pgxpool.Pool, userID, siteID, ip string) (VisitorDetail, error) {
	var d VisitorDetail
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return d, err
	} else if !ok {
		return d, pgx.ErrNoRows
	}
	err := db.QueryRow(ctx, `
		SELECT ip, COALESCE(NULLIF(MAX(isp), ''), 'unknown'),
		       COALESCE(NULLIF(MAX(country), ''), 'unknown'),
		       COALESCE(MAX(region), ''), COALESCE(MAX(city), ''),
		       COALESCE(MAX(lat), 0), COALESCE(MAX(lon), 0),
		       COALESCE(NULLIF(MAX(browser), ''), 'unknown'),
		       COALESCE(NULLIF(MAX(os), ''), 'unknown'),
		       COALESCE(NULLIF(MAX(device), ''), 'unknown'),
		       COALESCE(MAX(screen), ''), COALESCE(MAX(lang), ''),
		       MIN(visited_at), MAX(visited_at), count(*), count(DISTINCT session_id)
		FROM pageviews
		WHERE site_id = $1 AND ip = $2
		GROUP BY ip`, siteID, ip).
		Scan(&d.IP, &d.ISP, &d.Country, &d.Region, &d.City, &d.Lat, &d.Lon,
			&d.Browser, &d.OS, &d.Device, &d.Screen, &d.Lang,
			&d.FirstSeen, &d.LastSeen, &d.Pageviews, &d.Sessions)
	if err != nil {
		return d, err
	}
	d.SessionID, _ = func() (string, error) {
		var s string
		err := db.QueryRow(ctx, `SELECT session_id FROM pageviews WHERE site_id=$1 AND ip=$2 ORDER BY visited_at DESC LIMIT 1`, siteID, ip).Scan(&s)
		return s, err
	}()
	rows, err := db.Query(ctx, `
		SELECT COALESCE(path, '/'), count(*) FROM pageviews
		WHERE site_id = $1 AND ip = $2 GROUP BY 1 ORDER BY 2 DESC LIMIT 20`, siteID, ip)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	d.Paths = make([]model.Row, 0)
	for rows.Next() {
		var r model.Row
		if err := rows.Scan(&r.Key, &r.Value); err != nil {
			return d, err
		}
		d.Paths = append(d.Paths, r)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}
	rows2, err := db.Query(ctx, `
		SELECT ip, session_id, COALESCE(NULLIF(country, ''), 'unknown'),
		       COALESCE(region, ''), COALESCE(city, ''),
		       COALESCE(NULLIF(browser, ''), 'unknown'), COALESCE(NULLIF(os, ''), 'unknown'),
		       COALESCE(NULLIF(device, ''), 'unknown'), COALESCE(path, '/'), visited_at
		FROM pageviews WHERE site_id = $1 AND ip = $2
		ORDER BY visited_at DESC LIMIT 50`, siteID, ip)
	if err != nil {
		return d, err
	}
	defer rows2.Close()
	d.History = make([]Visitor, 0)
	for rows2.Next() {
		var v Visitor
		if err := rows2.Scan(&v.IP, &v.SessionID, &v.Country, &v.Region, &v.City, &v.Browser, &v.OS, &v.Device, &v.Path, &v.VisitedAt); err != nil {
			return d, err
		}
		d.History = append(d.History, v)
	}
	d.CountryCode = d.Country
	return d, rows2.Err()
}

func (q *Queries) World(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) ([]model.WorldPoint, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	cond, fargs := f.fragment(3)
	args := []any{siteID, from, to}
	args = append(args, fargs...)
	rows, err := db.Query(ctx, `
		SELECT COALESCE(NULLIF(country, ''), 'unknown'), count(*)
		FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`
		GROUP BY 1 ORDER BY 2 DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.WorldPoint, 0)
	for rows.Next() {
		var cc string
		var n int64
		if err := rows.Scan(&cc, &n); err != nil {
			return nil, err
		}
		if cc == "unknown" || cc == "" {
			continue
		}
		lat, lng := geo.LatLng(cc)
		if lat == 0 && lng == 0 {
			continue
		}
		out = append(out, model.WorldPoint{Country: cc, Count: n, Lat: lat, Lng: lng})
	}
	return out, rows.Err()
}

func (q *Queries) ExportCSV(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) ([]byte, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	writeSection := func(title string, header []string, rows [][]string) {
		w.Write([]string{title})
		w.Write(header)
		for _, r := range rows {
			w.Write(r)
		}
		w.Write([]string{})
	}
	times, err := q.Timeseries(ctx, db, userID, siteID, period, fromStr, toStr, f)
	if err != nil {
		return nil, err
	}
	tr := make([][]string, 0, len(times))
	for _, p := range times {
		tr = append(tr, []string{p.Date, itoa(p.Pageviews), itoa(p.Visitors)})
	}
	writeSection("Timeseries", []string{"date", "pageviews", "visitors"}, tr)

	for _, def := range []struct{ col, title string }{
		{"path", "Top pages"}, {"referrer", "Referrers"}, {"country", "Countries"},
		{"device", "Devices"}, {"browser", "Browsers"}, {"os", "OS"},
	} {
		top, err := q.Top(ctx, db, userID, siteID, period, def.col, 500, fromStr, toStr, f)
		if err != nil {
			return nil, err
		}
		tr := make([][]string, 0, len(top))
		for _, r := range top {
			tr = append(tr, []string{r.Key, itoa(r.Value)})
		}
		writeSection(def.title, []string{"key", "count"}, tr)
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func (q *Queries) RootOverview(ctx context.Context, db *pgxpool.Pool, userID, period, fromStr, toStr string) (RootOverview, error) {
	from, to, hourly := PeriodBounds(period, fromStr, toStr)
	if from.IsZero() {
		// Open-ended period: bound it by the oldest pageview across the
		// user's sites so the series stays a sane size.
		var e time.Time
		_ = db.QueryRow(ctx, `
			SELECT MIN(p.visited_at) FROM pageviews p
			JOIN sites s ON s.id = p.site_id
			WHERE s.user_id = $1 OR s.id IN (SELECT site_id FROM site_members WHERE user_id = $1)`,
			userID).Scan(&e)
		if !e.IsZero() {
			from = e.UTC().Truncate(24 * time.Hour)
		} else {
			from = to
		}
	}
	trunc, sqlLayout := "day", "YYYY-MM-DD"
	if hourly {
		trunc, sqlLayout = "hour", "YYYY-MM-DD HH24:00"
	}
	goLayout := "2006-01-02"
	if hourly {
		goLayout = "2006-01-02 15:00"
	}
	dateExpr := "to_char(date_trunc('" + trunc + "', p.visited_at), '" + sqlLayout + "')"

	var out RootOverview
	member := ` OR s.id IN (SELECT site_id FROM site_members WHERE user_id = $1)`
	err := db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM pageviews p JOIN sites s ON s.id = p.site_id WHERE (s.user_id = $1`+member+`) AND p.visited_at >= $2 AND p.visited_at < $3),
			(SELECT count(DISTINCT p.session_id) FROM pageviews p JOIN sites s ON s.id = p.site_id WHERE (s.user_id = $1`+member+`) AND p.visited_at >= $2 AND p.visited_at < $3),
			(SELECT count(*) FROM sites s WHERE user_id = $1`+member+`),
			(SELECT count(*) FROM events e JOIN sites s ON s.id = e.site_id WHERE (s.user_id = $1`+member+`) AND e.created_at >= $2 AND e.created_at < $3)`,
		userID, from, to).Scan(&out.Pageviews, &out.Visitors, &out.Sites, &out.Events)
	if err != nil {
		return out, err
	}

	rows, err := db.Query(ctx, `
		SELECT s.id, s.name, s.color,
		       COALESCE(`+dateExpr+`, ''),
		       count(p.id),
		       count(DISTINCT p.session_id)
		FROM sites s
		LEFT JOIN pageviews p ON p.site_id = s.id AND p.visited_at >= $1 AND p.visited_at < $2
		WHERE s.user_id = $3 OR s.id IN (SELECT site_id FROM site_members WHERE user_id = $3)
		GROUP BY s.id, s.name, s.color, `+dateExpr+`
		ORDER BY s.created_at`, from, to, userID)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	type acc struct {
		siteID   string
		name     string
		color    string
		counts   map[string]int64
		visitors map[string]int64
	}
	order := []string{}
	byID := map[string]*acc{}
	for rows.Next() {
		var id, name, color, date string
		var n, v int64
		if err := rows.Scan(&id, &name, &color, &date, &n, &v); err != nil {
			return out, err
		}
		if _, ok := byID[id]; !ok {
			byID[id] = &acc{siteID: id, name: name, color: color, counts: map[string]int64{}, visitors: map[string]int64{}}
			order = append(order, id)
		}
		if date != "" {
			byID[id].counts[date] = n
			byID[id].visitors[date] = v
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	for _, id := range order {
		a := byID[id]
		points := fillSeries(from, to, hourly, goLayout, a.counts, a.visitors)
		out.Series = append(out.Series, SiteSeries{
			SiteID: a.siteID, Name: a.name, Color: a.color, Points: points,
		})
	}
	if out.Series == nil {
		out.Series = []SiteSeries{}
	}

	// Per-site ranking for the period with previous-period pageviews so the
	// home page can show movement at a glance.
	rows, err = db.Query(ctx, `
		SELECT s.id, s.name, s.color, count(p.id), count(DISTINCT p.session_id)
		FROM sites s
		LEFT JOIN pageviews p ON p.site_id = s.id AND p.visited_at >= $1 AND p.visited_at < $2
		WHERE s.user_id = $3 OR s.id IN (SELECT site_id FROM site_members WHERE user_id = $3)
		GROUP BY s.id, s.name, s.color`, from, to, userID)
	if err != nil {
		return out, nil
	}
	defer rows.Close()
	totals := map[string][2]int64{}
	ids := []string{}
	meta := map[string][2]string{}
	for rows.Next() {
		var id, name, color string
		var n, v int64
		if rows.Scan(&id, &name, &color, &n, &v) != nil {
			continue
		}
		ids = append(ids, id)
		totals[id] = [2]int64{n, v}
		meta[id] = [2]string{name, color}
	}
	rows.Close()
	if len(ids) > 0 {
		prevFrom := from.Add(-to.Sub(from))
		rows, err = db.Query(ctx, `
			SELECT p.site_id, count(*)
			FROM pageviews p
			WHERE p.site_id = ANY($1::uuid[]) AND p.visited_at >= $2 AND p.visited_at < $3
			GROUP BY 1`, ids, prevFrom, from)
		if err == nil {
			defer rows.Close()
			prevTotals := map[string]int64{}
			for rows.Next() {
				var id string
				var n int64
				if rows.Scan(&id, &n) == nil {
					prevTotals[id] = n
				}
			}
			rows.Close()
			for _, id := range ids {
				t := totals[id]
				m := meta[id]
				out.Ranking = append(out.Ranking, model.SiteRank{
					SiteID: id, Name: m[0], Color: m[1],
					Pageviews: t[0], Visitors: t[1], PrevPageviews: prevTotals[id],
				})
			}
			sort.Slice(out.Ranking, func(i, j int) bool {
				if out.Ranking[i].Pageviews != out.Ranking[j].Pageviews {
					return out.Ranking[i].Pageviews > out.Ranking[j].Pageviews
				}
				return out.Ranking[i].Name < out.Ranking[j].Name
			})
		}
	}
	if out.Ranking == nil {
		out.Ranking = []model.SiteRank{}
	}
	return out, nil
}

func fillSeries(from, to time.Time, hourly bool, layout string, counts, visitors map[string]int64) []model.TimePoint {
	pts := []model.TimePoint{}
	step := 24 * time.Hour
	if hourly {
		step = time.Hour
	}
	for t := from.Truncate(step); t.Before(to); t = t.Add(step) {
		k := t.Format(layout)
		pts = append(pts, model.TimePoint{Date: k, Pageviews: counts[k], Visitors: visitors[k]})
	}
	return pts
}

type Queries struct{}

var Q = &Queries{}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

type Campaign struct {
	Source   string `json:"source"`
	Medium   string `json:"medium"`
	Campaign string `json:"campaign"`
	Content  string `json:"content"`
	Count    int64  `json:"count"`
	Visitors int64  `json:"visitors"`
}

func (q *Queries) Campaigns(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) ([]Campaign, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	cond, fargs := f.fragment(3)
	args := []any{siteID, from, to}
	args = append(args, fargs...)
	rows, err := db.Query(ctx, `
		SELECT COALESCE(NULLIF(utm_source, ''), '(none)'),
		       COALESCE(NULLIF(utm_medium, ''), '(none)'),
		       COALESCE(NULLIF(utm_campaign, ''), '(none)'),
		       COALESCE(NULLIF(utm_content, ''), '(none)'),
		       count(*), count(DISTINCT session_id)
		FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`
		  AND (utm_source <> '' OR utm_medium <> '' OR utm_campaign <> '')
		GROUP BY 1,2,3,4 ORDER BY count(*) DESC LIMIT 50`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Campaign, 0)
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.Source, &c.Medium, &c.Campaign, &c.Content, &c.Count, &c.Visitors); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type Goal struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	MatchType string    `json:"match_type"`
	CreatedAt time.Time `json:"created_at"`
}

type GoalSummary struct {
	Goal
	Pageviews     int64   `json:"pageviews"`
	Conversions   int64   `json:"conversions"`
	ConversionPct float64 `json:"conversion_pct"`
}

func (q *Queries) Goals(ctx context.Context, db *pgxpool.Pool, userID, siteID string) ([]Goal, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	rows, err := db.Query(ctx, `
		SELECT id, site_id, name, path, match_type, created_at
		FROM goals WHERE site_id = $1 ORDER BY created_at`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Goal, 0)
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.SiteID, &g.Name, &g.Path, &g.MatchType, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (q *Queries) GoalSummaries(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) ([]GoalSummary, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	cond, fargs := f.fragment(3)
	args := []any{siteID, from, to}
	args = append(args, fargs...)
	rows, err := db.Query(ctx, `
		SELECT g.id, g.name, g.path, g.match_type,
		       count(DISTINCT p.session_id),
		       (SELECT count(DISTINCT session_id) FROM pageviews WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`)
		FROM goals g
		LEFT JOIN pageviews p ON p.site_id = g.site_id
			AND p.visited_at >= $2 AND p.visited_at < $3
			AND (g.match_type = 'contains' AND p.path LIKE '%' || g.path || '%'
			  OR g.match_type = 'exact' AND p.path = g.path)
		WHERE g.site_id = $1
		GROUP BY g.id, g.name, g.path, g.match_type
		ORDER BY g.created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GoalSummary, 0)
	for rows.Next() {
		var g GoalSummary
		var visitors int64
		if err := rows.Scan(&g.ID, &g.Name, &g.Path, &g.MatchType, &g.Conversions, &visitors); err != nil {
			return nil, err
		}
		if visitors > 0 {
			g.ConversionPct = float64(g.Conversions) / float64(visitors) * 100
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type FunnelReport struct {
	Steps []struct {
		Path     string `json:"path"`
		Label    string `json:"label"`
		Sessions int64  `json:"sessions"`
	} `json:"steps"`
}

func (q *Queries) Funnel(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, paths []string, f Filters) (FunnelReport, error) {
	var out FunnelReport
	if len(paths) == 0 {
		return out, nil
	}
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return out, err
	} else if !ok {
		return out, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)

	// Ordered funnel: step N only counts when its first visit happens after
	// the first visit of step N-1. One query runs per funnel prefix, so the
	// filter placeholders are numbered per step (right after the path params
	// of that step). A shared numbering would leave unused parameters in the
	// shorter prefix queries and PostgreSQL rejects those (SQLSTATE 42P18).
	for i := range paths {
		k := i + 1
		cond, fargs := f.fragment(4 + k)
		step := paths[:k]
		label := ""
		if i == len(paths)-1 {
			label = "converted"
		}

		args := []any{siteID, from, to, step}
		var sel []string
		var wheres []string
		for j := range step {
			args = append(args, step[j])
			sel = append(sel, fmt.Sprintf("MIN(visited_at) FILTER (WHERE path = $%d) AS t%d", 5+j, j+1))
			if j > 0 {
				wheres = append(wheres, fmt.Sprintf("t%d > t%d", j+1, j))
			}
		}
		where := "t1 IS NOT NULL"
		if len(wheres) > 0 {
			where += " AND " + strings.Join(wheres, " AND ")
		}
		args = append(args, fargs...)

		sql := `
			WITH pv AS (
				SELECT session_id, path, visited_at FROM pageviews
				WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3
				  AND path = ANY($4::text[])` + cond + `
			),
			step_times AS (
				SELECT session_id, ` + strings.Join(sel, ", ") + `
				FROM pv GROUP BY session_id
			)
			SELECT count(*) FROM step_times WHERE ` + where
		var n int64
		if err := db.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			return out, err
		}
		out.Steps = append(out.Steps, struct {
			Path     string `json:"path"`
			Label    string `json:"label"`
			Sessions int64  `json:"sessions"`
		}{Path: paths[i], Label: label, Sessions: n})
	}
	return out, nil
}

func (q *Queries) EventDetails(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string) ([]model.EventDetail, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	rows, err := db.Query(ctx, `
		SELECT e.name, count(*), count(DISTINCT e.session_id),
		       COALESCE(avg(NULLIF((e.props->>'value')::numeric, 0)), 0),
		       COALESCE(max(NULLIF((e.props->>'value')::numeric, 0)), 0),
		       COALESCE(min(NULLIF((e.props->>'value')::numeric, 0)), 0)
		FROM events e
		WHERE e.site_id = $1 AND e.created_at >= $2 AND e.created_at < $3
		GROUP BY e.name ORDER BY count(*) DESC LIMIT 30`,
		siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.EventDetail, 0)
	for rows.Next() {
		var d model.EventDetail
		if err := rows.Scan(&d.Name, &d.Count, &d.Visitors, &d.AvgValue, &d.MaxValue, &d.MinValue); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (q *Queries) EventOccurrences(ctx context.Context, db *pgxpool.Pool, userID, siteID, name, period, fromStr, toStr string, limit int) ([]model.EventOccurrence, error) {	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)
	rows, err := db.Query(ctx, `
		SELECT name, session_id, COALESCE(url, ''), COALESCE(props, '{}'::jsonb), created_at
		FROM events
		WHERE site_id = $1 AND name = $4 AND created_at >= $2 AND created_at < $3
		ORDER BY created_at DESC LIMIT $5`,
		siteID, from, to, name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.EventOccurrence, 0)
	for rows.Next() {
		var o model.EventOccurrence
		if err := rows.Scan(&o.Name, &o.SessionID, &o.URL, &o.Props, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------- A3: Web Vitals ----------
// Vitals aggregate the web_vitals events (props: metric, value) emitted by
// the tracker: p75 per metric, per-path breakdown and a daily trend.

func vitalsExpr(metric string) string {
	return fmt.Sprintf(`COALESCE(percentile_cont(0.75) WITHIN GROUP (ORDER BY (props->>'value')::float) FILTER (WHERE props->>'metric' = '%s'), 0)`, metric)
}

func (q *Queries) Vitals(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string) (model.Vitals, error) {
	var out model.Vitals
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return out, err
	} else if !ok {
		return out, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
	from = clampFrom(ctx, db, siteID, from, to)

	rows, err := db.Query(ctx, `
		SELECT props->>'metric',
		       percentile_cont(0.75) WITHIN GROUP (ORDER BY (props->>'value')::float),
		       count(*)
		FROM events
		WHERE site_id = $1 AND name = 'web_vitals' AND created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY 1`, siteID, from, to)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Summary = make([]model.VitalStat, 0)
	for rows.Next() {
		var s model.VitalStat
		if err := rows.Scan(&s.Metric, &s.P75, &s.Samples); err != nil {
			return out, err
		}
		out.Summary = append(out.Summary, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = db.Query(ctx, `
		SELECT COALESCE(NULLIF(url, ''), '/') AS path, count(*),
		       `+vitalsExpr("lcp")+`, `+vitalsExpr("cls")+`, `+vitalsExpr("inp")+`
		FROM events
		WHERE site_id = $1 AND name = 'web_vitals' AND created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY count(*) DESC LIMIT 10`, siteID, from, to)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Paths = make([]model.VitalPathRow, 0)
	for rows.Next() {
		var p model.VitalPathRow
		if err := rows.Scan(&p.Path, &p.N, &p.LCP, &p.CLS, &p.INP); err != nil {
			return out, err
		}
		out.Paths = append(out.Paths, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = db.Query(ctx, `
		SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD'), count(*),
		       `+vitalsExpr("lcp")+`, `+vitalsExpr("cls")+`, `+vitalsExpr("inp")+`
		FROM events
		WHERE site_id = $1 AND name = 'web_vitals' AND created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY 1`, siteID, from, to)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Trend = make([]model.VitalTrendPoint, 0)
	for rows.Next() {
		var t model.VitalTrendPoint
		var n int64
		if err := rows.Scan(&t.Date, &n, &t.LCP, &t.CLS, &t.INP); err != nil {
			return out, err
		}
		out.Trend = append(out.Trend, t)
	}
	return out, rows.Err()
}
