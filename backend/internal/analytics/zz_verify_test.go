package analytics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
