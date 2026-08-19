package main

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/model"
)

func isSiteOwner(ctx *fiber.Ctx, db *pgxpool.Pool, siteID, userID string) bool {
	var ok bool
	_ = db.QueryRow(ctx.Context(),
		`SELECT EXISTS(SELECT 1 FROM sites WHERE id = $1 AND user_id = $2)`,
		siteID, userID).Scan(&ok)
	return ok
}

func membersHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := auth.UserID(c)
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, uid) {
			return errJSON(c, 404, "site not found")
		}
		rows, err := db.Query(c.Context(), `
			SELECT u.id, u.email, u.name, COALESCE(m.role, 'owner'),
			       u.id = s.user_id, COALESCE(m.created_at, s.created_at)
			FROM sites s
			JOIN users u ON u.id = s.user_id OR u.id IN (
				SELECT user_id FROM site_members WHERE site_id = s.id
			)
			LEFT JOIN site_members m ON m.site_id = s.id AND m.user_id = u.id
			WHERE s.id = $1
			ORDER BY (u.id = s.user_id) DESC, m.created_at`, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var out []model.Member
		for rows.Next() {
			var m model.Member
			if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.IsOwner, &m.CreatedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, m)
		}
		if out == nil {
			out = []model.Member{}
		}
		return c.JSON(out)
	}
}

func createInviteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Email = strings.ToLower(strings.TrimSpace(in.Email))
		if !strings.Contains(in.Email, "@") {
			return errJSON(c, 400, "invalid email")
		}
		if in.Role != "viewer" && in.Role != "editor" {
			in.Role = "viewer"
		}
		uid := auth.UserID(c)
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, uid) {
			return errJSON(c, 404, "site not found")
		}
		var existingID string
		err := db.QueryRow(c.Context(), `SELECT id::text FROM users WHERE email = $1`, in.Email).Scan(&existingID)
		if err == nil {
			_, err = db.Exec(c.Context(), `
				INSERT INTO site_members (site_id, user_id, role) VALUES ($1, $2, $3)
				ON CONFLICT (site_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
				siteID, existingID, in.Role)
			if err != nil {
				return errJSON(c, 500, "member add failed")
			}
			return c.Status(201).JSON(fiber.Map{"added": true, "email": in.Email})
		}
		token, err := randHex(16)
		if err != nil {
			return errJSON(c, 500, "token gen failed")
		}
		var inv model.Invite
		err = db.QueryRow(c.Context(), `
			INSERT INTO invites (site_id, email, role, token, expires_at)
			VALUES ($1, $2, $3, $4, now() + interval '7 days')
			RETURNING id, site_id, email, role, token, created_at, expires_at`,
			siteID, in.Email, in.Role, token).
			Scan(&inv.ID, &inv.SiteID, &inv.Email, &inv.Role, &inv.Token, &inv.CreatedAt, &inv.ExpiresAt)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		inv.InviteURL = "/invite/" + inv.Token
		return c.Status(201).JSON(inv)
	}
}

func listInvitesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := auth.UserID(c)
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, uid) {
			return errJSON(c, 404, "site not found")
		}
		rows, err := db.Query(c.Context(), `
			SELECT i.id, i.site_id, s.name, i.email, i.role, i.token, i.created_at, i.expires_at
			FROM invites i JOIN sites s ON s.id = i.site_id
			WHERE i.site_id = $1 AND i.expires_at > now()
			ORDER BY i.created_at DESC`, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var out []model.Invite
		for rows.Next() {
			var inv model.Invite
			if err := rows.Scan(&inv.ID, &inv.SiteID, &inv.SiteName, &inv.Email, &inv.Role, &inv.Token, &inv.CreatedAt, &inv.ExpiresAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			inv.InviteURL = "/invite/" + inv.Token
			out = append(out, inv)
		}
		if out == nil {
			out = []model.Invite{}
		}
		return c.JSON(out)
	}
}

func deleteInviteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := auth.UserID(c)
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, uid) {
			return errJSON(c, 404, "site not found")
		}
		tag, err := db.Exec(c.Context(), `
			DELETE FROM invites WHERE id = $1 AND site_id = $2`, c.Params("invite_id"), siteID)
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "invite not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func removeMemberHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := auth.UserID(c)
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, uid) {
			return errJSON(c, 404, "site not found")
		}
		tag, err := db.Exec(c.Context(), `
			DELETE FROM site_members WHERE site_id = $1 AND user_id = $2`, siteID, c.Params("user_id"))
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "member not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func inviteInfoHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var out struct {
			Email    string `json:"email"`
			SiteName string `json:"site_name"`
			Role     string `json:"role"`
		}
		err := db.QueryRow(c.Context(), `
			SELECT i.email, s.name, i.role
			FROM invites i JOIN sites s ON s.id = i.site_id
			WHERE i.token = $1 AND i.expires_at > now()`, c.Params("token")).
			Scan(&out.Email, &out.SiteName, &out.Role)
		if err != nil {
			return errJSON(c, 404, "invite not found or expired")
		}
		return c.JSON(out)
	}
}

func acceptInviteHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Password string `json:"password"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		if len(in.Password) < 8 {
			return errJSON(c, 400, "password must be at least 8 characters")
		}
		var inv struct {
			SiteID string
			Email  string
			Role   string
		}
		err := db.QueryRow(c.Context(), `
			SELECT site_id, email, role FROM invites
			WHERE token = $1 AND expires_at > now()`, c.Params("token")).
			Scan(&inv.SiteID, &inv.Email, &inv.Role)
		if err != nil {
			return errJSON(c, 404, "invite not found or expired")
		}
		hash, err := m.HashPassword(in.Password)
		if err != nil {
			return errJSON(c, 500, "hashing failed")
		}
		var userID string
		err = db.QueryRow(c.Context(), `
			INSERT INTO users (email, password_hash, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
			RETURNING id::text`, inv.Email, hash, inv.Email).Scan(&userID)
		if err != nil {
			return errJSON(c, 500, "user create failed")
		}
		_, err = db.Exec(c.Context(), `
			INSERT INTO site_members (site_id, user_id, role) VALUES ($1, $2, $3)
			ON CONFLICT (site_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
			inv.SiteID, userID, inv.Role)
		if err != nil {
			return errJSON(c, 500, "member add failed")
		}
		_, _ = db.Exec(c.Context(), `DELETE FROM invites WHERE token = $1`, c.Params("token"))
		token, err := m.Issue(userID, inv.Email, "user")
		if err != nil {
			return errJSON(c, 500, "token issue failed")
		}
		return c.JSON(fiber.Map{"token": token, "email": inv.Email})
	}
}

func getSettingsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var s model.SiteSettings
		err := db.QueryRow(c.Context(), `
			SELECT ss.site_id, ss.ip_hashing, ss.retention_days,
			       COALESCE(s.public_token, ''), COALESCE(s.public_enabled, false)
			FROM site_settings ss
			JOIN sites s ON s.id = ss.site_id
			WHERE ss.site_id = $1 AND (
				$2::uuid IN (SELECT user_id FROM sites WHERE id = ss.site_id)
				OR $2::uuid IN (SELECT user_id FROM site_members WHERE site_id = ss.site_id)
			)`, c.Params("id"), auth.UserID(c)).
			Scan(&s.SiteID, &s.IPHashing, &s.RetentionDays, &s.PublicToken, &s.PublicEnabled)
		if err != nil {
			return errJSON(c, 404, "site not found")
		}
		return c.JSON(s)
	}
}

func updateSettingsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			IPHashing     *bool   `json:"ip_hashing"`
			RetentionDays *int    `json:"retention_days"`
			PublicToken   *string `json:"public_token"`
			PublicEnabled *bool   `json:"public_enabled"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		uid := auth.UserID(c)
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, uid) {
			return errJSON(c, 404, "site not found")
		}
		if in.RetentionDays != nil && (*in.RetentionDays < 0 || *in.RetentionDays > 730) {
			return errJSON(c, 400, "retention days must be between 0 and 730")
		}
		if in.PublicToken != nil && *in.PublicToken == "" {
			return errJSON(c, 400, "invalid token")
		}
		_, err := db.Exec(c.Context(), `
			INSERT INTO site_settings (site_id, ip_hashing, retention_days)
			VALUES ($1, COALESCE($2, true), COALESCE($3, 0))
			ON CONFLICT (site_id) DO UPDATE SET
				ip_hashing = COALESCE($2, site_settings.ip_hashing),
				retention_days = COALESCE($3, site_settings.retention_days)`,
			siteID, in.IPHashing, in.RetentionDays)
		if err != nil {
			return errJSON(c, 500, "update failed")
		}
		if in.PublicToken != nil || in.PublicEnabled != nil {
			tok := in.PublicToken
			if tok == nil {
				tok = new(string)
				if err := db.QueryRow(c.Context(), `
					SELECT COALESCE(public_token, '') FROM sites WHERE id = $1`, siteID).Scan(tok); err != nil {
					return errJSON(c, 500, "query failed")
				}
			}
			enabled := in.PublicEnabled
			if enabled == nil {
				enabled = new(bool)
				if err := db.QueryRow(c.Context(), `
					SELECT public_enabled FROM sites WHERE id = $1`, siteID).Scan(enabled); err != nil {
					return errJSON(c, 500, "query failed")
				}
			}
			if _, err := db.Exec(c.Context(), `
				UPDATE sites SET public_token = $1, public_enabled = $2 WHERE id = $3`,
				*tok, *enabled, siteID); err != nil {
				return errJSON(c, 500, "update failed")
			}
		}
		return getSettingsHandler(db)(c)
	}
}

func retentionLoop(ctx context.Context, pool *pgxpool.Pool) {
	ticker := time.NewTicker(time.Hour)
	run := func() {
		rows, err := pool.Query(ctx, `SELECT site_id, retention_days FROM site_settings WHERE retention_days > 0`)
		if err != nil {
			return
		}
		type r struct {
			siteID string
			days   int
		}
		var rules []r
		for rows.Next() {
			var x r
			if rows.Scan(&x.siteID, &x.days) == nil {
				rules = append(rules, x)
			}
		}
		rows.Close()
		for _, x := range rules {
			cut := time.Now().UTC().AddDate(0, 0, -x.days)
			pool.Exec(ctx, `DELETE FROM pageviews WHERE site_id = $1 AND visited_at < $2`, x.siteID, cut)
			pool.Exec(ctx, `DELETE FROM events WHERE site_id = $1 AND created_at < $2`, x.siteID, cut)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
