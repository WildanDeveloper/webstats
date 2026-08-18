package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/model"
)

func errJSON(c *fiber.Ctx, code int, msg string) error {
	return c.Status(code).JSON(fiber.Map{"error": msg})
}

func registerHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Email = strings.ToLower(strings.TrimSpace(in.Email))
		if !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
			return errJSON(c, 400, "email invalid or password < 8 chars")
		}
		hash, err := m.HashPassword(in.Password)
		if err != nil {
			return errJSON(c, 500, "hashing failed")
		}
		var u model.User
		err = db.QueryRow(c.Context(), `
			INSERT INTO users (email, password_hash, name) VALUES ($1,$2,$3)
			RETURNING id, email, name, created_at`,
			in.Email, hash, in.Name).Scan(&u.ID, &u.Email, &u.Name, &u.Created)
		if err != nil {
			if strings.Contains(err.Error(), "23505") {
				return errJSON(c, 409, "email already registered")
			}
			return errJSON(c, 500, "register failed")
		}
		return c.Status(201).JSON(u)
	}
}

func loginHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		var u model.User
		err := db.QueryRow(c.Context(), `
			SELECT id, email, name, password_hash FROM users WHERE email = $1`,
			strings.ToLower(strings.TrimSpace(in.Email))).Scan(&u.ID, &u.Email, &u.Name, &u.Password)
		if err != nil || !m.CheckPassword(u.Password, in.Password) {
			return errJSON(c, 401, "wrong email or password")
		}
		token, err := m.Issue(u.ID, u.Email)
		if err != nil {
			return errJSON(c, 500, "token issue failed")
		}
		// audit session row (stateless JWT, kept for revocation list)
		_, _ = db.Exec(c.Context(), `
			INSERT INTO sessions (user_id, token_hash, user_agent, ip, expires_at)
			VALUES ($1,$2,$3,$4, now() + ($5 || ' seconds')::interval)`,
			u.ID, m.HashToken(token), c.Get("User-Agent"), c.IP(), int64(m.SessionTTL().Seconds()))
		return c.JSON(fiber.Map{"token": token, "user": u})
	}
}

func logoutHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tok := auth.BearerToken(c.Get("Authorization"))
		if tok != "" {
			_, _ = db.Exec(c.Context(), `DELETE FROM sessions WHERE token_hash = $1`, m.HashToken(tok))
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func meHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var u model.User
		err := db.QueryRow(c.Context(), `
			SELECT id, email, name, created_at FROM users WHERE id = $1`,
			auth.UserID(c)).Scan(&u.ID, &u.Email, &u.Name, &u.Created)
		if err != nil {
			return errJSON(c, 404, "user not found")
		}
		return c.JSON(u)
	}
}

func listSitesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), `
			SELECT id, user_id, name, domain, site_key, created_at
			FROM sites WHERE user_id = $1 ORDER BY created_at DESC`, auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var sites []model.Site
		for rows.Next() {
			var s model.Site
			if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Domain, &s.SiteKey, &s.CreatedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			sites = append(sites, s)
		}
		return c.JSON(sites)
	}
}

func createSiteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name   string `json:"name"`
			Domain string `json:"domain"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			return errJSON(c, 400, "name required")
		}
		key, err := randHex(10)
		if err != nil {
			return errJSON(c, 500, "key gen failed")
		}
		var s model.Site
		err = db.QueryRow(c.Context(), `
			INSERT INTO sites (user_id, name, domain, site_key)
			VALUES ($1,$2,$3,$4) RETURNING id, user_id, name, domain, site_key, created_at`,
			auth.UserID(c), in.Name, in.Domain, key).
			Scan(&s.ID, &s.UserID, &s.Name, &s.Domain, &s.SiteKey, &s.CreatedAt)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(s)
	}
}

func getSiteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var s model.Site
		err := db.QueryRow(c.Context(), `
			SELECT id, user_id, name, domain, site_key, created_at
			FROM sites WHERE id = $1 AND user_id = $2`, c.Params("id"), auth.UserID(c)).
			Scan(&s.ID, &s.UserID, &s.Name, &s.Domain, &s.SiteKey, &s.CreatedAt)
		if err != nil {
			return errJSON(c, 404, "site not found")
		}
		return c.JSON(s)
	}
}

func deleteSiteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tag, err := db.Exec(c.Context(), `
			DELETE FROM sites WHERE id = $1 AND user_id = $2`, c.Params("id"), auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "site not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func overviewHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		o, err := analytics.Q.Overview(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(o)
	}
}

func timeseriesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.Timeseries(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func topHandler(db *pgxpool.Pool, column string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.Top(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), column, 15)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func eventsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.TopEvents(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
