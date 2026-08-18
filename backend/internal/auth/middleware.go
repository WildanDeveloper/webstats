package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (m *Manager) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if h == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
		}
		claims, err := m.Parse(BearerToken(h))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		if !strings.EqualFold(claims.Issuer, "webstats") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		c.Locals("uid", claims.UserID)
		c.Locals("email", claims.Email)
		return c.Next()
	}
}

func UserID(c *fiber.Ctx) string {
	if v, ok := c.Locals("uid").(string); ok {
		return v
	}
	return ""
}
