package ingest

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
	"github.com/webstats/backend/internal/ua"
)

func (b *Buffer) CollectHandler(c *fiber.Ctx) error {
	var raw map[string]any
	if err := c.BodyParser(&raw); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad json"})
	}
	key := str(raw["site_id"])
	if key == "" || len(key) > 64 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing site_id"})
	}
	id := b.SiteID(c.Context(), key)
	if id == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "unknown site_id"})
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
