package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (m *Manager) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if uid, ok := c.Locals("uid").(string); ok && uid != "" {
			return c.Next()
		}
		h := c.Get("Authorization")
		if h == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
		}
		tok := BearerToken(h)
		claims, err := m.Parse(tok)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		if !strings.EqualFold(claims.Issuer, "webstats") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		if m.sessionCheck != nil && !m.sessionCheck(c.Context(), m.HashToken(tok)) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "session expired"})
		}
		c.Locals("uid", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)
		c.Locals("claims", claims)
		return c.Next()
	}
}

func (m *Manager) AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals("claims").(*Claims)
		if !ok || claims.Role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin access required"})
		}
		return c.Next()
	}
}

func UserID(c *fiber.Ctx) string {
	if v, ok := c.Locals("uid").(string); ok {
		return v
	}
	return ""
}

func UserRole(c *fiber.Ctx) string {
	if v, ok := c.Locals("role").(string); ok {
		return v
	}
	return "user"
}
