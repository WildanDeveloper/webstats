package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func changePasswordHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Current string `json:"current"`
			New     string `json:"new"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		if len(in.New) < 8 {
			return errJSON(c, 400, "password must be at least 8 characters")
		}
		var hash string
		if err := db.QueryRow(c.Context(), `
			SELECT password_hash FROM users WHERE id = $1`, auth.UserID(c)).Scan(&hash); err != nil {
			return errJSON(c, 500, "query failed")
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Current)) != nil {
			return errJSON(c, 400, "current password is incorrect")
		}
		newHash, err := bcrypt.GenerateFromPassword([]byte(in.New), 10)
		if err != nil {
			return errJSON(c, 500, "hash failed")
		}
		if _, err := db.Exec(c.Context(), `
			UPDATE users SET password_hash = $1 WHERE id = $2`, string(newHash), auth.UserID(c)); err != nil {
			return errJSON(c, 500, "update failed")
		}
		// Revoke every other session; the current device stays signed in.
		current := m.HashToken(auth.BearerToken(c.Get("Authorization")))
		_, _ = db.Exec(c.Context(),
			`DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`,
			auth.UserID(c), current)
		return c.JSON(fiber.Map{"ok": true})
	}
}

func hashKey(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

func generateApiKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "wsk_" + hex.EncodeToString(b), nil
}

func listApiKeysHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), `
			SELECT id, name, left(key_hash, 8), created_at, last_used_at
			FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`, auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var out []model.ApiKey
		for rows.Next() {
			var k model.ApiKey
			if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.CreatedAt, &k.LastUsedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, k)
		}
		if out == nil {
			out = []model.ApiKey{}
		}
		return c.JSON(out)
	}
}

func createApiKeyHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			in.Name = "API key"
		}
		plain, err := generateApiKey()
		if err != nil {
			return errJSON(c, 500, "key generation failed")
		}
		if _, err := db.Exec(c.Context(), `
			INSERT INTO api_keys (user_id, name, key_hash) VALUES ($1, $2, $3)`,
			auth.UserID(c), in.Name, hashKey(plain)); err != nil {
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(fiber.Map{"key": plain, "name": in.Name})
	}
}

func deleteApiKeyHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tag, err := db.Exec(c.Context(), `
			DELETE FROM api_keys WHERE id = $1 AND user_id = $2`,
			c.Params("kid"), auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "key not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func apiKeyFallback(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if h == "" {
			return c.Next()
		}
		tok := strings.TrimSpace(auth.BearerToken(h))
		if !strings.HasPrefix(tok, "wsk_") {
			return c.Next()
		}
		var uid, email, role string
		err := db.QueryRow(c.Context(), `
			SELECT k.user_id, u.email, u.role FROM api_keys k
			JOIN users u ON u.id = k.user_id
			WHERE k.key_hash = $1`, hashKey(tok)).Scan(&uid, &email, &role)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid api key"})
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = db.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE key_hash = $1`, hashKey(tok))
		}()
		c.Locals("uid", uid)
		c.Locals("email", email)
		c.Locals("role", role)
		c.Locals("claims", &auth.Claims{
			UserID: uid,
			Email:  email,
			Role:   role,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "webstats",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		return c.Next()
	}
}
