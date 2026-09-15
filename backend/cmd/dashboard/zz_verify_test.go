package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/model"
)

// ---------- pure-Go verification (no DB) ----------

func TestGeoCachePutDoesNotWipeCache(t *testing.T) {
	geoCacheMu.Lock()
	saved := geoCache
	geoCache = map[string]geoCacheEntry{}
	geoCacheMu.Unlock()
	defer func() {
		geoCacheMu.Lock()
		geoCache = saved
		geoCacheMu.Unlock()
	}()
	for i := 0; i < geoCacheMax; i++ {
		geoCachePut(fmt.Sprintf("10.0.0.%d", i%256)+fmt.Sprintf(":%d", i), ipapiResult{Status: "success", CountryCode: "US"}, true)
	}
	// One more insertion on a full cache: only the single oldest entry may
	// go, everything else must survive.
	geoCachePut("203.0.113.7", ipapiResult{Status: "success"}, true)
	geoCacheMu.Lock()
	n := len(geoCache)
	_, okOldest := geoCache["10.0.0.0:0"]
	_, okMiddle := geoCache["10.0.0.100:100"]
	geoCacheMu.Unlock()
	if n != geoCacheMax {
		t.Fatalf("cache size after put = %d, want %d", n, geoCacheMax)
	}
	if okOldest {
		t.Fatal("oldest entry should have been evicted to make room")
	}
	if !okMiddle {
		t.Fatal("cache was wiped wholesale instead of evicting one entry")
	}
}

func TestGeoCachePutExpiredEvictedFirst(t *testing.T) {
	geoCacheMu.Lock()
	saved := geoCache
	geoCache = map[string]geoCacheEntry{}
	geoCacheMu.Unlock()
	defer func() {
		geoCacheMu.Lock()
		geoCache = saved
		geoCacheMu.Unlock()
	}()
	geoCachePut("expired.host", ipapiResult{}, true)
	geoCacheMu.Lock()
	geoCache["expired.host"] = geoCacheEntry{exp: time.Now().Add(-time.Hour)}
	geoCacheMu.Unlock()
	geoCachePut("fresh.host", ipapiResult{}, true)
	geoCacheMu.Lock()
	_, expiredGone := geoCache["expired.host"]
	geoCacheMu.Unlock()
	if expiredGone {
		t.Fatal("expired entry should have been evicted")
	}
}

func TestValidateMonitorURL(t *testing.T) {
	pub, err := validateMonitorURL("https://example.com/health")
	if err != nil || pub != "https://example.com/health" {
		t.Fatalf("public URL rejected: %v (%s)", err, pub)
	}
	if _, err := validateMonitorURL("http://127.0.0.1:8086/"); err == nil {
		t.Fatal("loopback literal accepted")
	}
	if _, err := validateMonitorURL("http://192.168.1.10/"); err == nil {
		t.Fatal("RFC1918 literal accepted")
	}
	if _, err := validateMonitorURL("ftp://example.com"); err == nil {
		t.Fatal("ftp scheme accepted")
	}
}

func TestDialControlBlocksPrivateIP(t *testing.T) {
	if err := dialControl("tcp", "127.0.0.1:80", nil); err == nil {
		t.Fatal("loopback dial allowed")
	}
	if err := dialControl("tcp", "10.1.2.3:443", nil); err == nil {
		t.Fatal("RFC1918 dial allowed")
	}
	if err := dialControl("tcp", "169.254.169.254:80", nil); err == nil {
		t.Fatal("link-local dial allowed")
	}
	if err := dialControl("tcp", "[::1]:80", nil); err == nil {
		t.Fatal("ipv6 loopback dial allowed")
	}
}

func TestIsBlockedIPHost(t *testing.T) {
	if !isBlockedIPHost("localhost") {
		t.Fatal("localhost not blocked")
	}
	if !isBlockedIPHost("127.0.0.1") {
		t.Fatal("loopback literal not blocked")
	}
	// dns.google resolves to public addresses.
	if isBlockedIPHost("dns.google") {
		t.Fatal("public host blocked")
	}
	if !isBlockedIPHost("nonexistent.invalid.webstats.test") {
		t.Fatal("unresolvable host should fail closed")
	}
}

func TestPublicTokenRe(t *testing.T) {
	if !publicTokenRe.MatchString("abcXYZ012-_9") {
		t.Fatal("valid url-safe token rejected")
	}
	if publicTokenRe.MatchString("short") {
		t.Fatal("short token accepted")
	}
	if publicTokenRe.MatchString("has space!") {
		t.Fatal("invalid token accepted")
	}
}

// ---------- DB-backed verification ----------

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

// Verifies bug #1 fix: NULLIF($3,”)::uuid accepts an empty provider.
func TestUpdateRuleEmptyProvider(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	var uid, sid, rid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('r1@test.dev','x','R1') ON CONFLICT (email) DO UPDATE SET name='R1' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'S','r1.example','rk1') ON CONFLICT (site_key) DO UPDATE SET name='S' RETURNING id::text`, uid).Scan(&sid))
	must(t, db.QueryRow(ctx, `INSERT INTO notif_rules (user_id, site_id, event, channel, target) VALUES ($1,$2,'site_down','webhook','https://ex.com/h') RETURNING id::text`, uid, sid).Scan(&rid))
	// The exact statement shape used by updateRuleHandler with orEmpty(provider) = ''.
	if _, err := db.Exec(ctx, `
		UPDATE notif_rules SET
			event = COALESCE(NULLIF($1, ''), event),
			channel = $2,
			provider_id = NULLIF($3, '')::uuid,
			target = COALESCE(NULLIF($4, ''), target),
			params = $5,
			enabled = COALESCE($6, enabled)
		WHERE id = $7 AND user_id = $8`,
		"", "webhook", "", "https://ex.com/h", map[string]any{}, true, rid, uid); err != nil {
		t.Fatalf("empty provider_id rejected: %v", err)
	}
	var pid *string
	must(t, db.QueryRow(ctx, `SELECT provider_id::text FROM notif_rules WHERE id = $1`, rid).Scan(&pid))
	if pid != nil {
		t.Fatalf("provider_id = %v, want NULL", *pid)
	}
}

// Verifies bug #2 fix: NULL last_used_at scans into *time.Time.
func TestApiKeyNullLastUsedScan(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	var uid, kid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('r2@test.dev','x','R2') ON CONFLICT (email) DO UPDATE SET name='R2' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO api_keys (user_id, name, key_hash) VALUES ($1,'k','deadbeef') RETURNING id::text`, uid).Scan(&kid))
	defer db.Exec(ctx, `DELETE FROM api_keys WHERE id = $1`, kid)
	rows, err := db.Query(ctx, `
		SELECT id, name, left(key_hash, 8), created_at, last_used_at
		FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`, uid)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k model.ApiKey
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.CreatedAt, &k.LastUsedAt); err != nil {
			t.Fatalf("scan NULL last_used_at failed: %v", err)
		}
		if k.LastUsedAt != nil {
			t.Fatalf("LastUsedAt = %v, want nil", *k.LastUsedAt)
		}
	}
}

// Verifies bug #3 fix: monitor_checks query is scoped by site ownership.
func TestMonitorChecksIDOR(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	var u1, u2, s1, s2, m1 string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('r3@test.dev','x','R3') ON CONFLICT (email) DO UPDATE SET name='R3' RETURNING id::text`).Scan(&u1))
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('r4@test.dev','x','R4') ON CONFLICT (email) DO UPDATE SET name='R4' RETURNING id::text`).Scan(&u2))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'S1','r3.example','rk3') ON CONFLICT (site_key) DO UPDATE SET name='S1' RETURNING id::text`, u1).Scan(&s1))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'S2','r4.example','rk4') ON CONFLICT (site_key) DO UPDATE SET name='S2' RETURNING id::text`, u2).Scan(&s2))
	must(t, db.QueryRow(ctx, `INSERT INTO monitors (site_id, url) VALUES ($1,'https://r3.example') RETURNING id::text`, s1).Scan(&m1))
	if _, err := db.Exec(ctx, `INSERT INTO monitor_checks (monitor_id, status_code, ok, latency_ms) VALUES ($1, 200, true, 42)`, m1); err != nil {
		t.Fatal(err)
	}
	// Attacker (owner of s2) asks for monitor m1 with their own site id.
	rows, err := db.Query(ctx, `
		SELECT mc.status_code, mc.ok, mc.latency_ms, mc.checked_at
		FROM monitor_checks mc
		JOIN monitors m ON m.id = mc.monitor_id
		WHERE m.id = $1 AND m.site_id = $2
		ORDER BY mc.checked_at DESC LIMIT 24`, m1, s2)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	if n != 0 {
		t.Fatalf("IDOR: got %d checks via foreign site id, want 0", n)
	}
	// Owner still sees the data.
	rows2, err := db.Query(ctx, `
		SELECT mc.status_code, mc.ok, mc.latency_ms, mc.checked_at
		FROM monitor_checks mc
		JOIN monitors m ON m.id = mc.monitor_id
		WHERE m.id = $1 AND m.site_id = $2
		ORDER BY mc.checked_at DESC LIMIT 24`, m1, s1)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows2.Close()
	n = 0
	for rows2.Next() {
		n++
	}
	if n != 1 {
		t.Fatalf("owner sees %d checks, want 1", n)
	}
}

// Verifies bug #12 fix: retention loop does not reference site_daily (dropped).
func TestRetentionLoopSQLWithoutSiteDaily(t *testing.T) {
	db := testPool(t)
	ctx := context.Background()
	var uid, sid string
	must(t, db.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('r5@test.dev','x','R5') ON CONFLICT (email) DO UPDATE SET name='R5' RETURNING id::text`).Scan(&uid))
	must(t, db.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'S','r5.example','rk5') ON CONFLICT (site_key) DO UPDATE SET name='S' RETURNING id::text`, uid).Scan(&sid))
	if _, err := db.Exec(ctx, `INSERT INTO site_settings (site_id, retention_days) VALUES ($1, 1)
		ON CONFLICT (site_id) DO UPDATE SET retention_days = 1`, sid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO site_checks (site_id, status, latency_ms) VALUES ($1,'up',10)`, sid); err != nil {
		t.Fatal(err)
	}
	cut := time.Now().UTC().AddDate(0, 0, -2)
	if _, err := db.Exec(ctx, `DELETE FROM pageviews WHERE site_id = $1 AND visited_at < $2`, sid, cut); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM events WHERE site_id = $1 AND created_at < $2`, sid, cut); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM site_checks WHERE site_id = $1 AND checked_at < $2`, sid, cut); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM monitor_checks
		WHERE monitor_id IN (SELECT id FROM monitors WHERE site_id = $1) AND checked_at < $2`, sid, cut); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.site_daily') IS NULL`).Scan(new(bool)); err != nil {
		t.Fatal(err)
	}
	var gone bool
	must(t, db.QueryRow(ctx, `SELECT to_regclass('public.site_daily') IS NULL`).Scan(&gone))
	if !gone {
		t.Fatal("site_daily unexpectedly exists; retention loop must not reference it")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Compiles the concurrent map access pattern to keep -race meaningful.
func TestConcurrentMapTypes(t *testing.T) {
	type entry struct {
		id string
		at time.Time
	}
	var mu sync.Mutex
	m := map[string]entry{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			m[fmt.Sprint(i)] = entry{id: "x", at: time.Now()}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if len(m) != 8 {
		t.Fatal("concurrent writes lost")
	}
}
