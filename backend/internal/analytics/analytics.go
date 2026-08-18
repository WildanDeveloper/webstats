package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/model"
)

type PageviewRow struct {
	SiteID       string    `json:"site_id"`
	SessionID    string    `json:"session_id"`
	Path         string    `json:"path"`
	Title        string    `json:"title"`
	Referrer     string    `json:"referrer"`
	ReferrerHost string    `json:"referrer_host"`
	UA           string    `json:"ua"`
	Browser      string    `json:"browser"`
	OS           string    `json:"os"`
	Device       string    `json:"device"`
	Country      string    `json:"country"`
	Screen       string    `json:"screen"`
	Lang         string    `json:"lang"`
	IPHash       string    `json:"ip_hash"`
	VisitedAt    time.Time `json:"visited_at"`
}

type EventRowIn struct {
	SiteID    string         `json:"site_id"`
	SessionID string         `json:"session_id"`
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	Props     map[string]any `json:"props"`
	CreatedAt time.Time      `json:"created_at"`
}

func InsertPageviews(ctx context.Context, db *pgxpool.Pool, rows []PageviewRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, r := range rows {
		if _, err := tx.Exec(ctx, `INSERT INTO pageviews
				(site_id, session_id, path, title, referrer, referrer_host, ua, browser, os, device, country, screen, lang, ip_hash, visited_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			r.SiteID, r.SessionID, r.Path, r.Title, r.Referrer, r.ReferrerHost,
			r.UA, r.Browser, r.OS, r.Device, r.Country, r.Screen, r.Lang, r.IPHash, r.VisitedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func InsertEvents(ctx context.Context, db *pgxpool.Pool, rows []EventRowIn) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, r := range rows {
		if _, err := tx.Exec(ctx, `INSERT INTO events (site_id, session_id, name, url, props, created_at)
				VALUES ($1,$2,$3,$4,$5,$6)`,
			r.SiteID, r.SessionID, r.Name, r.URL, r.Props, r.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AggregateDaily upserts daily rollups from raw pageviews for the given range.
func AggregateDaily(ctx context.Context, db *pgxpool.Pool, siteID string, from, to time.Time) error {
	_, err := db.Exec(ctx, `
		WITH sess AS (
			SELECT session_id, count(*) AS c
			FROM pageviews
			WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3
			GROUP BY session_id
		)
		INSERT INTO site_daily (site_id, day, pageviews, visitors, sessions, bounces)
		SELECT p.site_id, p.visited_at::date,
			count(*),
			count(DISTINCT p.session_id),
			count(DISTINCT p.session_id),
			(SELECT count(*) FROM sess WHERE c = 1)
		FROM pageviews p
		WHERE p.site_id = $1 AND p.visited_at >= $2 AND p.visited_at < $3
		GROUP BY p.site_id, p.visited_at::date
		ON CONFLICT (site_id, day) DO UPDATE SET
			pageviews = EXCLUDED.pageviews,
			visitors  = EXCLUDED.visitors,
			sessions  = EXCLUDED.sessions,
			bounces   = EXCLUDED.bounces`,
		siteID, from, to)
	return err
}

func siteOwned(ctx context.Context, db *pgxpool.Pool, userID, siteID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sites WHERE id = $1 AND user_id = $2)`,
		siteID, userID).Scan(&ok)
	return ok, err
}

func bounds(period string) (from time.Time, to time.Time, hourly bool) {
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

func (q *Queries) Overview(ctx context.Context, db *pgxpool.Pool, userID, siteID, period string) (model.Overview, error) {
	if ok, err := siteOwned(ctx, db, userID, siteID); err != nil {
		return model.Overview{}, err
	} else if !ok {
		return model.Overview{}, pgx.ErrNoRows
	}
	from, to, _ := bounds(period)

	var out model.Overview
	err := db.QueryRow(ctx, `
		WITH s AS (
			SELECT session_id, count(*) AS c
			FROM pageviews
			WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3
			GROUP BY session_id
		)
		SELECT
			(SELECT count(*) FROM pageviews WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3),
			(SELECT count(*) FROM s),
			(SELECT count(*) FROM s WHERE c = 1)`,
		siteID, from, to).Scan(&out.Pageviews, &out.Sessions, &out.Bounces)
	if err != nil {
		return out, err
	}
	out.Visitors = out.Sessions
	days := int64(to.Sub(from).Hours()/24) + 1
	if days > 0 {
		out.AvgPerDay = float64(out.Pageviews) / float64(days)
	}
	if out.Sessions > 0 {
		out.BounceRate = float64(out.Bounces) / float64(out.Sessions) * 100
	}
	return out, nil
}

func (q *Queries) Timeseries(ctx context.Context, db *pgxpool.Pool, userID, siteID, period string) ([]model.TimePoint, error) {
	if ok, err := siteOwned(ctx, db, userID, siteID); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, hourly := bounds(period)
	trunc, fmt := "day", "YYYY-MM-DD"
	if hourly {
		trunc, fmt = "hour", "YYYY-MM-DD HH24:00"
	}
	rows, err := db.Query(ctx, `
		SELECT to_char(date_trunc('`+trunc+`', visited_at), '`+fmt+`'),
		       count(*), count(DISTINCT session_id)
		FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3
		GROUP BY 1 ORDER BY 1`,
		siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.TimePoint
	for rows.Next() {
		var p model.TimePoint
		if err := rows.Scan(&p.Date, &p.Pageviews, &p.Visitors); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q *Queries) Top(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, column string, limit int) ([]model.Row, error) {
	if ok, err := siteOwned(ctx, db, userID, siteID); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := bounds(period)
	order := "count(*) DESC"
	if column == "referrer" {
		column = "referrer_host"
		order = "count(*) DESC NULLS LAST"
	}
	fill := "'(direct)'"
	if column == "country" {
		fill = "'unknown'"
	}
	sql := `
		SELECT COALESCE(NULLIF(` + column + `, ''), ` + fill + `) AS key, count(*)
		FROM pageviews
		WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3
		GROUP BY ` + column + ` ORDER BY ` + order + ` LIMIT ` + itoa(limit)
	rows, err := db.Query(ctx, sql, siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Row
	for rows.Next() {
		var r model.Row
		if err := rows.Scan(&r.Key, &r.Value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) TopEvents(ctx context.Context, db *pgxpool.Pool, userID, siteID, period string) ([]model.EventRow, error) {
	if ok, err := siteOwned(ctx, db, userID, siteID); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := bounds(period)
	rows, err := db.Query(ctx, `
		SELECT name, count(*) FROM events
		WHERE site_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY name ORDER BY count(*) DESC LIMIT 20`,
		siteID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.EventRow
	for rows.Next() {
		var r model.EventRow
		if err := rows.Scan(&r.Name, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Queries struct{}

var Q = &Queries{}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
