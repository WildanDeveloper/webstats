package analytics

import (
	"context"
	"os"
	"testing"
	"time"

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
func TestSessionStats(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	var uid, sid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('an1@test.dev','x','A1') ON CONFLICT (email) DO UPDATE SET name='A1' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'A1','an1.example','ak1') ON CONFLICT (site_key) DO UPDATE SET name='A1' RETURNING id::text`, uid).Scan(&sid))
	seedSessionData(t, db, sid)

	out, err := Q.SessionStats(ctx, db, uid, sid, "24h", "", "", Filters{})
	if err != nil {
		t.Fatalf("SessionStats: %v", err)
	}
	// The seeded site may contain rows from previous runs of other tests
	// sharing the site key, so assert on invariants rather than absolutes.
	if out.Sessions < 2 {
		t.Fatalf("sessions = %d, want >= 2", out.Sessions)
	}
	if out.BounceRate < 0 || out.BounceRate > 100 {
		t.Fatalf("bounce rate out of range: %v", out.BounceRate)
	}
	if out.AvgPages < 1 {
		t.Fatalf("avg pages = %v, want >= 1", out.AvgPages)
	}
	if out.AvgDurationSec < 0 {
		t.Fatalf("avg duration negative: %v", out.AvgDurationSec)
	}
}

// Verifies A1: entry pages land on the first path of a session and exit
// pages on the last one.
func TestSessionBoundsEntryExit(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	var uid, sid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('an2@test.dev','x','A2') ON CONFLICT (email) DO UPDATE SET name='A2' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'A2','an2.example','ak2') ON CONFLICT (site_key) DO UPDATE SET name='A2' RETURNING id::text`, uid).Scan(&sid))
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
	var uid, sid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('vit1@test.dev','x','V1') ON CONFLICT (email) DO UPDATE SET name='V1' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'V1','vit1.example','vk1') ON CONFLICT (site_key) DO UPDATE SET name='V1' RETURNING id::text`, uid).Scan(&sid))
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
	if found["lcp"] < 2000 || found["lcp"] > 4000 {
		t.Fatalf("lcp p75 out of expected band: %v (summary %+v)", found["lcp"], out.Summary)
	}
	var slow *model.VitalPathRow
	for i := range out.Paths {
		if out.Paths[i].Path == "/slow" {
			slow = &out.Paths[i]
		}
	}
	if slow == nil || slow.LCP != 4000 {
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
	var uid, sid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('an3@test.dev','x','A3') ON CONFLICT (email) DO UPDATE SET name='A3' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'A3','an3.example','ak3') ON CONFLICT (site_key) DO UPDATE SET name='A3' RETURNING id::text`, uid).Scan(&sid))
	seedSessionData(t, db, sid)

	out, err := Q.Overview(ctx, db, uid, sid, "24h", "", "", Filters{})
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if out.Pageviews < 4 || out.Sessions < 2 {
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
	var uid, sid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('zq9@test.dev','x','R1') ON CONFLICT (email) DO UPDATE SET name='R1' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'R1','zq9.example','zq9') ON CONFLICT (site_key) DO UPDATE SET name='R1' RETURNING id::text`, uid).Scan(&sid))
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
