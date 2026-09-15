package ingest

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/config"
)

func mustPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testBuffer(t *testing.T) *Buffer {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	cfg := &config.Config{BufferSize: 64, BatchSize: 10, FlushEvery: time.Hour, IPHashSalt: "s"}
	b := NewBuffer(cfg, mustPool(t, url), nil, nil)
	t.Cleanup(func() { b.Stop() })
	return b
}

// Verifies bug #7 fix: concurrent SiteID/IPHashing do not race.
func TestCacheConcurrentAccess(t *testing.T) {
	b := testBuffer(t)
	ctx := context.Background()
	var uid string
	if err := b.db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('ing@test.dev','x','I') ON CONFLICT (email) DO UPDATE SET name='I' RETURNING id::text`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	var sid string
	if err := b.db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'S','ing.example','ik1') ON CONFLICT (site_key) DO UPDATE SET name='S' RETURNING id::text`, uid).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.SiteID(ctx, "ik1")
			_ = b.IPHashing(ctx, sid)
		}()
	}
	wg.Wait()
	if got := b.SiteID(ctx, "ik1"); got != sid {
		t.Fatalf("SiteID = %q, want %q", got, sid)
	}
}

// Verifies bug #7 fix: entries expire after cacheTTL.
func TestCacheTTLExpiry(t *testing.T) {
	b := testBuffer(t)
	ctx := context.Background()
	var uid string
	if err := b.db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('ing2@test.dev','x','I2') ON CONFLICT (email) DO UPDATE SET name='I2' RETURNING id::text`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	var sid string
	if err := b.db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'S','ing2.example','ik2') ON CONFLICT (site_key) DO UPDATE SET name='S' RETURNING id::text`, uid).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	if b.SiteID(ctx, "ik2") == "" {
		t.Fatal("expected site id")
	}
	b.mu.Lock()
	b.siteIDs["ik2"] = siteIDEntry{id: b.siteIDs["ik2"].id, at: time.Now().Add(-2 * cacheTTL)}
	b.mu.Unlock()
	// Expired entry: lookup must re-query the DB (still returns the id, but
	// via a fresh entry with a recent timestamp).
	id := b.SiteID(ctx, "ik2")
	if id == "" {
		t.Fatal("re-query after expiry failed")
	}
	b.mu.Lock()
	at := b.siteIDs["ik2"].at
	b.mu.Unlock()
	if time.Since(at) > cacheTTL {
		t.Fatal("expired entry was not refreshed")
	}
}

// Verifies bug #7 fix: Redis push failure falls back to the local channel.
func TestPushFallsBackOnRedisError(t *testing.T) {
	cfg := &config.Config{BufferSize: 8, BatchSize: 10, FlushEvery: time.Hour, RedisURL: "redis://127.0.0.1:1/0"}
	b := NewBuffer(cfg, nil, nil, nil)
	defer b.Stop()
	b.Push(Record{Kind: "pageview", SiteID: "s", Path: "/"})
	select {
	case r := <-b.ch:
		if r.SiteID != "s" {
			t.Fatalf("record SiteID = %q", r.SiteID)
		}
	default:
		t.Fatal("record lost: no local fallback after redis failure")
	}
}

// Verifies bug #7 fix: successful Redis push does not also hit the channel.
// Uses a real redis if REDIS_TEST_URL is set; otherwise skipped.
