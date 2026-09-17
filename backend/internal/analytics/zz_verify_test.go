package analytics

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/model"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testSite(t *testing.T, db *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	var uid, sid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name)
		VALUES (gen_random_uuid()::text || '@test.invalid', 'x', 'Analytics test') RETURNING id::text`).Scan(&uid))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid); err != nil {
			t.Errorf("fixture cleanup: %v", err)
		}
	})
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key)
		VALUES ($1, 'Analytics test', 'analytics.invalid', gen_random_uuid()::text) RETURNING id::text`, uid).Scan(&sid))
	return uid, sid
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Fixture: two sessions over the last day.
//
//	s1: /home -> /pricing (+60s) -> /signup (+60s)   (3 pages, 120s)
//	s2: /home only                                    (1 page, 0s, bounce)
func seedSessionData(t *testing.T, db *pgxpool.Pool, siteID string) {
	t.Helper()
	now := time.Now().UTC().Add(-time.Hour)
	rows := []PageviewRow{
		{SiteID: siteID, SessionID: "s1", Path: "/home", VisitedAt: now},
		{SiteID: siteID, SessionID: "s1", Path: "/pricing", VisitedAt: now.Add(60 * time.Second)},
		{SiteID: siteID, SessionID: "s1", Path: "/signup", VisitedAt: now.Add(120 * time.Second)},
		{SiteID: siteID, SessionID: "s2", Path: "/home", VisitedAt: now.Add(10 * time.Second)},
	}
	must(t, InsertPageviews(context.Background(), db, rows))
}

// Verifies A2: session duration, depth and bounce rate.
func TestPeriodBoundsCustom(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		span     time.Duration
	}{
		{"2026-09-01 10:00", "2026-09-01 11:00", time.Hour},
		{"2026-09-01", "2026-09-01", 24 * time.Hour},
		{"2026-09-01", "2026-09-02", 48 * time.Hour},
		{"2026-09-02", "2026-09-01", 0},
		{"2026-09-01 11:00", "2026-09-01 10:00", 0},
		{"2026-09-01 10:00", "2026-09-01 10:00", 0},
	} {
		from, to, _ := PeriodBounds("all", tc.from, tc.to)
		if to.Sub(from) != tc.span || from.After(to) {
			t.Errorf("%s to %s: got %v", tc.from, tc.to, to.Sub(from))
		}
	}
}

func TestSpreadsheetSafe(t *testing.T) {
	for _, value := range []string{"=text", "+text", "-text", "@text", " \t=text"} {
		if got := spreadsheetSafe(value); got != "'"+value {
			t.Errorf("neutralization: got %q", got)
		}
	}
	for _, value := range []string{"", "/home", "plain text", "123"} {
		if got := spreadsheetSafe(value); got != value {
			t.Errorf("text changed: got %q", got)
		}
	}
}

func TestGoalFilteredCohort(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	now := time.Now().UTC().Add(-time.Hour)
	must(t, InsertPageviews(ctx, db, []PageviewRow{
		{SiteID: sid, SessionID: "selected", Path: "/home", Country: "US", VisitedAt: now},
		{SiteID: sid, SessionID: "selected", Path: "/signup", VisitedAt: now.Add(time.Minute)},
		{SiteID: sid, SessionID: "bounce", Path: "/home", Country: "US", VisitedAt: now},
		{SiteID: sid, SessionID: "excluded", Path: "/signup", Country: "DE", VisitedAt: now},
	}))
	_, err := db.Exec(ctx, `INSERT INTO goals (site_id, name, path, match_type) VALUES ($1, 'Signup', '/signup', 'exact')`, sid)
	must(t, err)
	for _, filter := range []Filters{{Page: "/home"}, {Country: "US"}} {
		out, err := Q.GoalSummaries(ctx, db, uid, sid, "24h", "", "", filter)
		must(t, err)
		if len(out) != 1 || out[0].Conversions != 1 || out[0].ConversionPct != 50 {
			t.Fatalf("filtered goals: %+v", out)
		}
	}
}

func TestFunnelLaterTraversal(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	now := time.Now().UTC().Add(-time.Hour)
	var rows []PageviewRow
	for i, path := range []string{"/b", "/a", "/b", "/a"} {
		rows = append(rows, PageviewRow{SiteID: sid, SessionID: "later", Path: path, VisitedAt: now.Add(time.Duration(i) * time.Second)})
		rows = append(rows, PageviewRow{SiteID: sid, SessionID: "tied", Path: path, VisitedAt: now})
	}
	must(t, InsertPageviews(ctx, db, rows))
	out, err := Q.Funnel(ctx, db, uid, sid, "24h", "", "", []string{"/a", "/b", "/a"}, Filters{})
	must(t, err)
	if len(out.Steps) != 3 {
		t.Fatalf("funnel: %+v", out)
	}
	for _, step := range out.Steps {
		if step.Sessions != 2 {
			t.Fatalf("later traversal or ID tie-break lost: %+v", out)
		}
	}
}

func TestEventOnlyAllTime(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	now := time.Now().UTC().Add(-48 * time.Hour)
	var events []EventRowIn
	for _, value := range []any{0, 10, "not numeric", nil, true, map[string]any{}, "1e9999", "NaN"} {
		events = append(events, EventRowIn{SiteID: sid, SessionID: "event", Name: "web_vitals", URL: "/", Props: map[string]any{"metric": "lcp", "value": value}, CreatedAt: now})
	}
	must(t, InsertEvents(ctx, db, events))
	for _, withPageview := range []bool{false, true} {
		if withPageview {
			must(t, InsertPageviews(ctx, db, []PageviewRow{{SiteID: sid, SessionID: "page", Path: "/", VisitedAt: now.Add(time.Hour)}}))
		}
		top, err := Q.TopEvents(ctx, db, uid, sid, "all", "", "")
		must(t, err)
		if len(top) != 1 || top[0].Count != int64(len(events)) {
			t.Fatalf("top events: %+v", top)
		}
		details, err := Q.EventDetails(ctx, db, uid, sid, "all", "", "")
		must(t, err)
		if len(details) != 1 || details[0].AvgValue != 5 || details[0].MinValue != 0 || details[0].MaxValue != 10 {
			t.Fatalf("event values: %+v", details)
		}
		occurrences, err := Q.EventOccurrences(ctx, db, uid, sid, "web_vitals", "all", "", "", 20)
		must(t, err)
		if len(occurrences) != len(events) {
			t.Fatalf("occurrences: %d", len(occurrences))
		}
		vitals, err := Q.Vitals(ctx, db, uid, sid, "all", "", "")
		must(t, err)
		if len(vitals.Summary) != 1 || vitals.Summary[0].P75 != 7.5 || vitals.Summary[0].Samples != 2 || len(vitals.Paths) != 1 || vitals.Paths[0].LCP != 7.5 || len(vitals.Trend) != 1 || vitals.Trend[0].LCP != 7.5 {
			t.Fatalf("vitals: %+v", vitals)
		}
		root, err := Q.RootOverview(ctx, db, uid, "all", "", "")
		must(t, err)
		if root.Events != int64(len(events)) {
			t.Fatalf("root events: %d", root.Events)
		}
	}
}

func TestPartitionMaintenance(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	_, sid := testSite(t, db)
	future := time.Now().UTC().AddDate(0, 2, 0)
	future = time.Date(future.Year(), future.Month(), 15, 12, 0, 0, 0, time.UTC)
	must(t, InsertPageviews(ctx, db, []PageviewRow{{SiteID: sid, SessionID: "defaulted", Path: "/", VisitedAt: future}}))
	var defCount int64
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM pageviews_default WHERE site_id = $1`, sid).Scan(&defCount))
	if defCount != 1 {
		t.Fatalf("row should land in DEFAULT partition: %d", defCount)
	}
	must(t, ensurePageviewPartition(ctx, db, time.Date(future.Year(), future.Month(), 1, 0, 0, 0, 0, time.UTC)))
	must(t, db.QueryRow(ctx, `SELECT count(*) FROM pageviews_default WHERE site_id = $1`, sid).Scan(&defCount))
	if defCount != 0 {
		t.Fatalf("DEFAULT rows not redistributed: %d", defCount)
	}
	part := pgx.Identifier{fmt.Sprintf("pageviews_%04d_%02d", future.Year(), int(future.Month()))}.Sanitize()
	var partCount int64
	must(t, db.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE site_id = $1`, part), sid).Scan(&partCount))
	if partCount != 1 {
		t.Fatalf("partition rows: %d", partCount)
	}
	if _, err := db.Exec(ctx, fmt.Sprintf(`DROP TABLE %s`, part)); err != nil {
		t.Fatalf("drop partition: %v", err)
	}
}

func TestSessionStats(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	seedSessionData(t, db, sid)

	out, err := Q.SessionStats(ctx, db, uid, sid, "24h", "", "", Filters{})
	if err != nil {
		t.Fatalf("SessionStats: %v", err)
	}
	if out.Sessions != 2 || out.BounceRate != 50 || out.AvgPages != 2 || out.AvgDurationSec != 60 {
		t.Fatalf("unexpected session stats: %+v", out)
	}
}

// Verifies A1: entry pages land on the first path of a session and exit
// pages on the last one.
func TestSessionBoundsEntryExit(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	seedSessionData(t, db, sid)

	entry, err := Q.SessionBounds(ctx, db, uid, sid, "24h", "", "", "entry", 10, Filters{})
	if err != nil {
		t.Fatalf("entry: %v", err)
	}
	foundHome := false
	for _, r := range entry {
		if r.Key == "/home" {
			foundHome = true
		}
		if r.Key == "/signup" || r.Key == "/pricing" {
			t.Fatalf("/signup or /pricing must never be an entry page: %+v", entry)
		}
	}
	if !foundHome {
		t.Fatalf("/home missing from entry pages: %+v", entry)
	}

	exit, err := Q.SessionBounds(ctx, db, uid, sid, "24h", "", "", "exit", 10, Filters{})
	if err != nil {
		t.Fatalf("exit: %v", err)
	}
	foundSignup, foundHomeExit := false, false
	for _, r := range exit {
		switch r.Key {
		case "/signup":
			foundSignup = true
		case "/home":
			foundHomeExit = true
		case "/pricing":
			t.Fatalf("/pricing must never be an exit page: %+v", exit)
		}
	}
	if !foundSignup || !foundHomeExit {
		t.Fatalf("exit pages incomplete: %+v", exit)
	}

	// Clamp: limit=0 must not error or return negative rows.
	small, err := Q.SessionBounds(ctx, db, uid, sid, "24h", "", "", "entry", 0, Filters{})
	if err != nil || len(small) > 1 {
		t.Fatalf("limit clamp failed: err=%v rows=%d", err, len(small))
	}
}

// Verifies A3: web_vitals events aggregate to p75 per metric, per-path
// breakdown and a daily trend.
func TestVitals(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	now := time.Now().UTC().Add(-time.Hour)
	evs := []EventRowIn{
		{SiteID: sid, SessionID: "v1", Name: "web_vitals", URL: "/slow", Props: map[string]any{"metric": "lcp", "value": 4000}, CreatedAt: now},
		{SiteID: sid, SessionID: "v2", Name: "web_vitals", URL: "/slow", Props: map[string]any{"metric": "lcp", "value": 2000}, CreatedAt: now},
		{SiteID: sid, SessionID: "v3", Name: "web_vitals", URL: "/", Props: map[string]any{"metric": "lcp", "value": 1000}, CreatedAt: now},
		{SiteID: sid, SessionID: "v4", Name: "web_vitals", URL: "/", Props: map[string]any{"metric": "cls", "value": 0.05}, CreatedAt: now},
	}
	must(t, InsertEvents(ctx, db, evs))

	out, err := Q.Vitals(ctx, db, uid, sid, "24h", "", "")
	if err != nil {
		t.Fatalf("Vitals: %v", err)
	}
	found := map[string]float64{}
	for _, s := range out.Summary {
		found[s.Metric] = s.P75
		if s.Samples < 1 {
			t.Fatalf("metric %s has no samples: %+v", s.Metric, out.Summary)
		}
	}
	if found["lcp"] != 3000 || found["cls"] != 0.05 {
		t.Fatalf("lcp p75 out of expected band: %v (summary %+v)", found["lcp"], out.Summary)
	}
	var slow *model.VitalPathRow
	for i := range out.Paths {
		if out.Paths[i].Path == "/slow" {
			slow = &out.Paths[i]
		}
	}
	if slow == nil || slow.LCP != 3500 {
		t.Fatalf("/slow path missing or wrong p75: %+v", out.Paths)
	}
	if len(out.Trend) == 0 {
		t.Fatalf("expected at least one trend point")
	}
}

// Verifies A16: overview returns previous-window counters alongside the
// current ones so the UI can compute deltas.
func TestOverviewPrevWindow(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	seedSessionData(t, db, sid)

	out, err := Q.Overview(ctx, db, uid, sid, "24h", "", "", Filters{})
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if out.Pageviews != 4 || out.Sessions != 2 {
		t.Fatalf("current window incomplete: %+v", out)
	}
	if out.PrevPageviews < 0 || out.PrevSessions < 0 || out.PrevBounces < 0 {
		t.Fatalf("prev-window counters must never be negative: %+v", out)
	}
}

// Verifies F4: root overview ranks sites by pageviews with prev values.
func TestRootOverviewRanking(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	uid, sid := testSite(t, db)
	seedSessionData(t, db, sid)

	out, err := Q.RootOverview(ctx, db, uid, "7d", "", "")
	if err != nil {
		t.Fatalf("RootOverview: %v", err)
	}
	if len(out.Series) == 0 {
		t.Fatalf("expected series for owned site")
	}
	desc := true
	for i := 1; i < len(out.Ranking); i++ {
		if out.Ranking[i-1].Pageviews < out.Ranking[i].Pageviews {
			desc = false
		}
	}
	if !desc {
		t.Fatalf("ranking not sorted by pageviews desc: %+v", out.Ranking)
	}
	for _, r := range out.Ranking {
		if r.PrevPageviews < 0 {
			t.Fatalf("negative prev pageviews: %+v", r)
		}
	}
}
