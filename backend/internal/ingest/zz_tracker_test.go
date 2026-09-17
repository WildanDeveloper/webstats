package ingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/webstats/backend/internal/config"
)

// End-to-end check that the collector accepts the tracker's real request
// shape: fetch with Content-Type application/json (B01) and sendBeacon with
// an Origin header. The payload mirrors what tracker/track.js builds.
func TestCollectHandlerTrackerRequestShape(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool := mustPool(t, url)
	ctx := background()
	var uid string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('tracker@test.dev','x','T') ON CONFLICT (email) DO UPDATE SET name='T' RETURNING id::text`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	key := "tkey" + time.Now().Format("150405.000000000")
	if err := pool.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'T','tracker.example',$2) ON CONFLICT (site_key) DO UPDATE SET domain='tracker.example' RETURNING site_key`, uid, key).Scan(&key); err != nil {
		t.Fatal(err)
	}

	workerCfg := &config.Config{BufferSize: 16, BatchSize: 4, FlushEvery: time.Hour, IPHashSalt: "s"}
	b := NewBuffer(workerCfg, pool, nil, nil)
	t.Cleanup(func() { b.Stop() })
	app := fiber.New()
	app.Post("/api/collect", b.CollectHandler)
	app.Post("/api/event", b.CollectHandler)

	post := func(target string, payload map[string]any, contentType string) int {
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Origin", "https://tracker.example")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	pageview := map[string]any{
		"kind": "pageview", "site_id": key, "session_id": "sess-1",
		"path": "/a#h1", "title": "T", "referrer": "",
		"ua": "Mozilla/5.0 (X11; Linux x86_64) Chrome/126", "ts": time.Now().UnixMilli(),
	}
	if code := post("/api/collect", pageview, "application/json"); code != 200 {
		t.Fatalf("pageview with application/json rejected: %d", code)
	}

	event := map[string]any{
		"kind": "event", "site_id": key, "session_id": "sess-1", "event_name": "cta",
		"props": map[string]any{"x": "1"}, "url": "/a#h1",
		"ua": "Mozilla/5.0 (X11; Linux x86_64) Chrome/126", "ts": time.Now().UnixMilli(),
	}
	if code := post("/api/event", event, "application/json"); code != 200 {
		t.Fatalf("event with Content-Type json rejected: %d", code)
	}

	// text/plain (the old tracker bug shape) is rejected by BodyParser.
	if code := post("/api/collect", pageview, "text/plain; charset=UTF-8"); code != 400 {
		t.Fatalf("text/plain body should be rejected: %d", code)
	}
}
