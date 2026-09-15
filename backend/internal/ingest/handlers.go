package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/webstats/backend/internal/ua"
)

// hostFromHeader extracts a lowercase hostname from an Origin or Referer
// header value. Returns "" when the value is absent or not a URL.
func hostFromHeader(v string) string {
	if v == "" {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
}

// originAllowed enforces that browser traffic claiming a site key actually
// comes from the domain registered for that site. The tracker always sends
// Origin (cross-origin POST + sendBeacon), so honest clients pass.
//
//   - registered domain exact match or subdomain of it: allowed
//   - site without a registered domain: cannot enforce, allowed
//   - no Origin AND no Referer (curl, server-side scripts): rejected
func originAllowed(registeredDomain, origin, referer string) bool {
	domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(registeredDomain), "."))
	host := hostFromHeader(origin)
	if host == "" {
		host = hostFromHeader(referer)
	}
	if host == "" {
		return domain == ""
	}
	if domain == "" {
		return true
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func (b *Buffer) CollectHandler(c *fiber.Ctx) error {
	var raw map[string]any
	if err := c.BodyParser(&raw); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad json"})
	}
	key := str(raw["site_id"])
	if key == "" || len(key) > 64 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing site_id"})
	}
	id, domain := b.SiteInfo(c.Context(), key)
	if id == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "unknown site_id"})
	}
	// Anti-spoofing: only browsers on the site's own domain may write data.
	// Without this, anyone who reads the site_key out of the page source
	// could fabricate pageviews with a plain curl.
	if !originAllowed(domain, c.Get("Origin"), c.Get("Referer")) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "origin not allowed"})
	}
	raw["site_id"] = id
	if raw["kind"] == "" {
		raw["kind"] = "pageview"
	}
	rec := b.Normalize(raw, c.IP())
	if rec.SessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing session_id"})
	}
	// Silently accept and drop crawler/bot traffic so it never reaches the DB.
	if ua.IsBot(rec.UA) {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	}
	b.Push(rec)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
}

func hashIP(ip, salt string) string {
	sum := sha256.Sum256([]byte(ip + ":" + salt))
	return hex.EncodeToString(sum[:8])
}
