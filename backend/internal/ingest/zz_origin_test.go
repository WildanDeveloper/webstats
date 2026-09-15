package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/webstats/backend/internal/config"
)

func background() context.Context { return context.Background() }

func TestOriginAllowed(t *testing.T) {
	cases := []struct {
		domain, origin, referer string
		want                    bool
	}{
		// Registered domain, honest browser
		{"shop.misal.com", "https://shop.misal.com", "", true},
		// Subdomain of registered apex
		{"misal.com", "https://shop.misal.com", "", true},
		// www variant
		{"misal.com", "https://www.misal.com", "", true},
		// Origin missing, Referer present and valid
		{"shop.misal.com", "", "https://shop.misal.com/checkout", true},
		// Attacker domain that merely contains the registered name
		{"misal.com", "https://evilmisal.com", "", false},
		// Attacker subdomain trick
		{"shop.misal.com", "https://shop.misal.com.evil.io", "", false},
		// No origin, no referer (curl)
		{"shop.misal.com", "", "", false},
		// Site without registered domain cannot be enforced
		{"", "", "", true},
		{"", "https://anything.example", "", true},
		// Case and trailing dot normalization
		{"Shop.Misal.com.", "https://SHOP.MISAL.COM", "", true},
		// Malformed origin, valid referer
		{"shop.misal.com", "null", "https://shop.misal.com", true},
		{"shop.misal.com", "null", "https://evil.io", false},
	}
	for _, tc := range cases {
		if got := originAllowed(tc.domain, tc.origin, tc.referer); got != tc.want {
			t.Errorf("originAllowed(%q, %q, %q) = %v, want %v",
				tc.domain, tc.origin, tc.referer, got, tc.want)
		}
	}
}

// End-to-end through the fiber handler against the real test database.
func TestCollectHandlerOriginEnforced(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool := mustPool(t, url)
	ctx := background()
	var uid, sid, key string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, name) VALUES ('og@test.dev','x','OG') ON CONFLICT (email) DO UPDATE SET name='OG' RETURNING id::text`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO sites (user_id, name, domain, site_key) VALUES ($1,'OG','shop.misal.com',$2) ON CONFLICT (site_key) DO UPDATE SET domain='shop.misal.com' RETURNING site_key`,
		uid, "okey"+time.Now().Format("150405")).Scan(&key); err != nil {
		t.Fatal(err)
	}
	_ = sid

	cfg := &config.Config{BufferSize: 8, BatchSize: 10, FlushEvery: time.Hour}
	b := NewBuffer(cfg, pool, nil, nil)
	t.Cleanup(func() { b.Stop() })

	app := fiber.New()
	app.Post("/api/collect", b.CollectHandler)

	post := func(origin, referer string) int {
		body, _ := json.Marshal(map[string]any{
			"site_id":    key,
			"session_id": "sess-1",
			"path":       "/",
			"ua":         "Mozilla/5.0 (X11; Linux x86_64) Chrome/126",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/collect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if code := post("https://shop.misal.com", ""); code != 200 {
		t.Fatalf("honest origin rejected: %d", code)
	}
	if code := post("https://evil.io", ""); code != 403 {
		t.Fatalf("foreign origin accepted: %d", code)
	}
	if code := post("", ""); code != 403 {
		t.Fatalf("origin-less curl accepted: %d", code)
	}

	// Unknown key still 404 (and cannot be probed without origin either).
	body := strings.NewReader(`{"site_id":"nonexistent","session_id":"s"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collect", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("unknown site: %d, want 404", resp.StatusCode)
	}
}
