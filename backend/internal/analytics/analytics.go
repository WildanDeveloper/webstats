package analytics

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
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
				(site_id, session_id, path, title, referrer, referrer_host, ua, browser, os, device, country, screen, lang, ip_hash, ip, visited_at,
				 utm_source, utm_medium, utm_campaign, utm_content, utm_term)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
			r.SiteID, r.SessionID, r.Path, r.Title, r.Referrer, r.ReferrerHost,
			r.UA, r.Browser, r.OS, r.Device, r.Country, r.Screen, r.Lang, r.IPHash, r.IP, r.VisitedAt,
			utm(r.UTM, "utm_source"), utm(r.UTM, "utm_medium"), utm(r.UTM, "utm_campaign"),
			utm(r.UTM, "utm_content"), utm(r.UTM, "utm_term")); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
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

func (q *Queries) Overview(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) (model.Overview, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return model.Overview{}, err
	} else if !ok {
		return model.Overview{}, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)

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
		)
		SELECT
			(SELECT count(*) FROM pageviews WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3`+cond+`),
			(SELECT count(*) FROM s),
			(SELECT count(*) FROM s WHERE c = 1),
			(SELECT count(*) FROM pageviews WHERE site_id = $1 AND visited_at >= $4 AND visited_at < $2`+cond+`),
			(SELECT count(DISTINCT session_id) FROM pageviews WHERE site_id = $1 AND visited_at >= $4 AND visited_at < $2`+cond+`)`,
		args...).Scan(&out.Pageviews, &out.Sessions, &out.Bounces, &out.PrevPageviews, &out.PrevVisitors)
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

func (q *Queries) Timeseries(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) ([]model.TimePoint, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, hourly := PeriodBounds(period, fromStr, toStr)
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
	out := fillSeries(from, to, hourly, goLayout, curC, curV)

	prevFrom := from.Add(-(to.Sub(from)))
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
			return out, err
		}
		prevC := map[string]int64{}
		prevV := map[string]int64{}
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
			return out, err
		}
		prev := fillSeries(prevFrom, from, hourly, goLayout, prevC, prevV)
		for i := range out {
			if i < len(prev) {
				out[i].PrevPageviews = prev[i].Pageviews
				out[i].PrevVisitors = prev[i].Visitors
			}
		}
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
	order := "count(*) DESC"
	if column == "referrer" {
		column = "referrer_host"
		order = "count(*) DESC NULLS LAST"
	}
	fill := "'unknown'"
	if column == "referrer" {
		fill = "'(direct)'"
	}
	if column == "country" {
		fill = "'unknown'"
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
	rows, err := db.Query(ctx, `
		SELECT name, count(*) FROM events
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
		if err := rows.Scan(&r.Name, &r.Count); err != nil {
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
		SELECT ip, session_id, COALESCE(NULLIF(country, ''), 'unknown'),
		       COALESCE(NULLIF(browser, ''), 'unknown'), COALESCE(NULLIF(os, ''), 'unknown'),
		       COALESCE(NULLIF(device, ''), 'unknown'), COALESCE(path, '/'), visited_at
		FROM pageviews
		WHERE site_id = $1 AND ip <> ''
		ORDER BY visited_at DESC LIMIT $2`, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Visitor, 0)
	for rows.Next() {
		var v Visitor
		if err := rows.Scan(&v.IP, &v.SessionID, &v.Country, &v.Browser, &v.OS, &v.Device, &v.Path, &v.VisitedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (q *Queries) World(ctx context.Context, db *pgxpool.Pool, userID, siteID, period, fromStr, toStr string, f Filters) ([]model.WorldPoint, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
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
	cond, fargs := f.fragment(4)
	for i := range paths {
		step := paths[:i+1]
		label := ""
		if i == len(paths)-1 {
			label = "converted"
		}
		var n int64
		args := []any{siteID, from, to, step}
		args = append(args, fargs...)
		err := db.QueryRow(ctx, `
			SELECT count(*) FROM (
				SELECT session_id FROM pageviews
				WHERE site_id = $1 AND visited_at >= $2 AND visited_at < $3
				  AND path = ANY($4::text[])`+cond+`
				GROUP BY session_id
				HAVING count(DISTINCT path) = array_length($4, 1)
			) s`, args...).Scan(&n)
		if err != nil {
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

func (q *Queries) EventOccurrences(ctx context.Context, db *pgxpool.Pool, userID, siteID, name, period, fromStr, toStr string, limit int) ([]model.EventOccurrence, error) {
	if ok, err := siteAccess(ctx, db, userID, siteID, ""); err != nil {
		return nil, err
	} else if !ok {
		return nil, pgx.ErrNoRows
	}
	from, to, _ := PeriodBounds(period, fromStr, toStr)
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
