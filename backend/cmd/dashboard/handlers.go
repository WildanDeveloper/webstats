package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"net"
	"regexp"
	"strings"
	"time"

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

func filtersFromQuery(c *fiber.Ctx) analytics.Filters {
	return analytics.Filters{
		Page:    c.Query("page"),
		Source:  c.Query("source"),
		Country: c.Query("country"),
		Device:  c.Query("device"),
		Browser: c.Query("browser"),
		OS:      c.Query("os"),
	}
}

var SiteColors = []string{
	"#ef4444",
	"#3b82f6",
	"#10b981",
	"#f59e0b",
	"#8b5cf6",
	"#ec4899",
	"#06b6d4",
	"#84cc16",
	"#f97316",
	"#0ea5e9",
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

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
			RETURNING id, email, name, role, created_at`,
			in.Email, hash, in.Name).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Created)
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
			SELECT id, email, name, role, password_hash FROM users WHERE email = $1`,
			strings.ToLower(strings.TrimSpace(in.Email))).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Password)
		if err != nil || !m.CheckPassword(u.Password, in.Password) {
			return errJSON(c, 401, "wrong email or password")
		}
		token, err := m.Issue(u.ID, u.Email, u.Role)
		if err != nil {
			return errJSON(c, 500, "token issue failed")
		}
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
			SELECT id, email, name, role, created_at FROM users WHERE id = $1`,
			auth.UserID(c)).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Created)
		if err != nil {
			return errJSON(c, 404, "user not found")
		}
		return c.JSON(u)
	}
}

func overviewHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.RootOverview(c.Context(), db, auth.UserID(c), c.Query("period", "7d"), c.Query("from"), c.Query("to"))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func listSitesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), `
			SELECT s.id, s.user_id, s.name, s.domain, s.site_key, s.color, s.created_at,
			       COALESCE(sc.status, ''), COALESCE(sc.latency_ms, 0), COALESCE(sc.checked_at, TIMESTAMPTZ 'epoch')
			FROM sites s
			LEFT JOIN LATERAL (
				SELECT status, latency_ms, checked_at FROM site_checks
				WHERE site_id = s.id ORDER BY checked_at DESC LIMIT 1
			) sc ON true
			WHERE s.user_id = $1 OR s.id IN (SELECT site_id FROM site_members WHERE user_id = $1)
			ORDER BY s.created_at DESC`, auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var sites []model.Site
		for rows.Next() {
			var s model.Site
			var status string
			var latency int64
			var checked time.Time
			if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Domain, &s.SiteKey, &s.Color, &s.CreatedAt, &status, &latency, &checked); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			s.Status = status
			s.LatencyMs = latency
			if !checked.IsZero() && checked.After(time.Time{}) {
				s.CheckedAt = checked
			}
			sites = append(sites, s)
		}
		if sites == nil {
			sites = []model.Site{}
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
		var n int
		_ = db.QueryRow(c.Context(), `SELECT count(*) FROM sites WHERE user_id = $1`, auth.UserID(c)).Scan(&n)
		color := SiteColors[n%len(SiteColors)]
		var s model.Site
		err = db.QueryRow(c.Context(), `
			INSERT INTO sites (user_id, name, domain, site_key, color)
			VALUES ($1,$2,$3,$4,$5) RETURNING id, user_id, name, domain, site_key, color, created_at`,
			auth.UserID(c), in.Name, in.Domain, key, color).
			Scan(&s.ID, &s.UserID, &s.Name, &s.Domain, &s.SiteKey, &s.Color, &s.CreatedAt)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(s)
	}
}

func getSiteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var s model.Site
		var status string
		var latency int64
		var checked time.Time
		err := db.QueryRow(c.Context(), `
			SELECT s.id, s.user_id, s.name, s.domain, s.site_key, s.color, s.created_at,
			       COALESCE(sc.status, ''), COALESCE(sc.latency_ms, 0), COALESCE(sc.checked_at, TIMESTAMPTZ 'epoch')
			FROM sites s
			LEFT JOIN LATERAL (
				SELECT status, latency_ms, checked_at FROM site_checks
				WHERE site_id = s.id ORDER BY checked_at DESC LIMIT 1
			) sc ON true
			WHERE s.id = $1 AND s.user_id = $2`, c.Params("id"), auth.UserID(c)).
			Scan(&s.ID, &s.UserID, &s.Name, &s.Domain, &s.SiteKey, &s.Color, &s.CreatedAt, &status, &latency, &checked)
		if err != nil {
			return errJSON(c, 404, "site not found")
		}
		s.Status = status
		s.LatencyMs = latency
		if !checked.IsZero() && checked.After(time.Time{}) {
			s.CheckedAt = checked
		}
		return c.JSON(s)
	}
}

func updateSiteHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name   *string `json:"name"`
			Domain *string `json:"domain"`
			Color  *string `json:"color"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		if in.Color != nil && !hexColor.MatchString(*in.Color) {
			return errJSON(c, 400, "color must be a hex value like #ef4444")
		}
		tag, err := db.Exec(c.Context(), `
			UPDATE sites SET
				name    = COALESCE(NULLIF($1, ''), name),
				domain  = COALESCE(NULLIF($2, ''), domain),
				color   = COALESCE(NULLIF($3, ''), color)
			WHERE id = $4 AND user_id = $5`,
			orEmpty(in.Name), orEmpty(in.Domain), orEmpty(in.Color), c.Params("id"), auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "update failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "site not found")
		}
		return getSiteHandler(db)(c)
	}
}

func orEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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

func sslCheckHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var domain string
		err := db.QueryRow(c.Context(), `
			SELECT domain FROM sites WHERE id = $1 AND user_id = $2`,
			c.Params("id"), auth.UserID(c)).Scan(&domain)
		if err != nil {
			return errJSON(c, 404, "site not found")
		}
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return errJSON(c, 400, "set a domain for this site first")
		}
		host := domain
		if i := strings.Index(host, "://"); i >= 0 {
			host = host[i+3:]
		}
		host = strings.TrimSuffix(host, "/")

		res := fiber.Map{"url": "https://" + host}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 6 * time.Second},
			"tcp", net.JoinHostPort(host, "443"), &tls.Config{ServerName: host})
		if err != nil {
			res["valid"] = false
			res["error"] = err.Error()
			return c.JSON(res)
		}
		defer conn.Close()
		if err := conn.VerifyHostname(host); err != nil {
			res["valid"] = false
			res["error"] = err.Error()
			return c.JSON(res)
		}
		cert := conn.ConnectionState().PeerCertificates[0]
		res["valid"] = true
		res["issuer"] = cert.Issuer.CommonName
		res["subject"] = cert.Subject.CommonName
		res["expires_at"] = cert.NotAfter.Format(time.RFC3339)
		res["days_left"] = int(time.Until(cert.NotAfter).Hours() / 24)
		return c.JSON(res)
	}
}

func siteOverviewHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		o, err := analytics.Q.Overview(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
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
		out, err := analytics.Q.Timeseries(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
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
		out, err := analytics.Q.Top(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), column, 15, c.Query("from"), c.Query("to"), filtersFromQuery(c))
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
		out, err := analytics.Q.TopEvents(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), c.Query("from"), c.Query("to"))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func eventDetailsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.EventDetails(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), c.Query("from"), c.Query("to"))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func eventOccurrencesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.EventOccurrences(c.Context(), db, auth.UserID(c), c.Params("id"), c.Params("name"), c.Query("period", "7d"), c.Query("from"), c.Query("to"), 10)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func validRole(r string) bool { return r == "admin" || r == "user" }

func listUsersHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), `
			SELECT id, email, name, role, created_at FROM users ORDER BY created_at DESC`)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var users []model.User
		for rows.Next() {
			var u model.User
			if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Created); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			users = append(users, u)
		}
		return c.JSON(users)
	}
}

func createUserHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Email = strings.ToLower(strings.TrimSpace(in.Email))
		if !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
			return errJSON(c, 400, "email invalid or password < 8 chars")
		}
		if !validRole(in.Role) {
			in.Role = "user"
		}
		hash, err := m.HashPassword(in.Password)
		if err != nil {
			return errJSON(c, 500, "hashing failed")
		}
		var u model.User
		err = db.QueryRow(c.Context(), `
			INSERT INTO users (email, password_hash, name, role)
			VALUES ($1,$2,$3,$4) RETURNING id, email, name, role, created_at`,
			in.Email, hash, in.Name, in.Role).
			Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Created)
		if err != nil {
			if strings.Contains(err.Error(), "23505") {
				return errJSON(c, 409, "email already exists")
			}
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(u)
	}
}

func updateUserHandler(db *pgxpool.Pool, m *auth.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name     *string `json:"name"`
			Email    *string `json:"email"`
			Password *string `json:"password"`
			Role     *string `json:"role"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		id := c.Params("id")
		uid := auth.UserID(c)

		if in.Role != nil && !validRole(*in.Role) {
			return errJSON(c, 400, "role must be admin or user")
		}
		if id == uid && in.Role != nil && *in.Role != "admin" {
			return errJSON(c, 400, "you cannot demote your own account")
		}

		if in.Role != nil && *in.Role == "user" {
			var isAdmin bool
			_ = db.QueryRow(c.Context(), `SELECT role = 'admin' FROM users WHERE id = $1`, id).Scan(&isAdmin)
			if isAdmin {
				var admins int
				_ = db.QueryRow(c.Context(), `SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&admins)
				if admins <= 1 {
					return errJSON(c, 400, "cannot remove the last admin")
				}
			}
		}
		if in.Email != nil {
			in2 := strings.ToLower(strings.TrimSpace(*in.Email))
			if !strings.Contains(in2, "@") {
				return errJSON(c, 400, "invalid email")
			}
			in.Email = &in2
		}

		tag, err := db.Exec(c.Context(), `
			UPDATE users SET
				name  = COALESCE(NULLIF($1, ''), name),
				email = COALESCE(NULLIF($2, ''), email),
				role  = COALESCE(NULLIF($3, ''), role)
			WHERE id = $4`,
			orEmpty(in.Name), orEmpty(in.Email), orEmpty(in.Role), id)
		if err != nil {
			if strings.Contains(err.Error(), "23505") {
				return errJSON(c, 409, "email already exists")
			}
			return errJSON(c, 500, "update failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "user not found")
		}
		if in.Password != nil && len(*in.Password) >= 8 {
			hash, err := m.HashPassword(*in.Password)
			if err != nil {
				return errJSON(c, 500, "hashing failed")
			}
			if _, err := db.Exec(c.Context(), `UPDATE users SET password_hash = $1 WHERE id = $2`, hash, id); err != nil {
				return errJSON(c, 500, "password update failed")
			}
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func deleteUserHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == auth.UserID(c) {
			return errJSON(c, 400, "you cannot delete your own account")
		}
		var isAdmin bool
		_ = db.QueryRow(c.Context(), `SELECT role = 'admin' FROM users WHERE id = $1`, id).Scan(&isAdmin)
		if isAdmin {
			var admins int
			_ = db.QueryRow(c.Context(), `SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&admins)
			if admins <= 1 {
				return errJSON(c, 400, "cannot delete the last admin")
			}
		}
		tag, err := db.Exec(c.Context(), `DELETE FROM users WHERE id = $1`, id)
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "user not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func adminStatsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var out struct {
			Users     int64 `json:"users"`
			Sites     int64 `json:"sites"`
			Pageviews int64 `json:"pageviews"`
			Events    int64 `json:"events"`
		}
		err := db.QueryRow(c.Context(), `
			SELECT
				(SELECT count(*) FROM users),
				(SELECT count(*) FROM sites),
				(SELECT count(*) FROM pageviews),
				(SELECT count(*) FROM events)`).
			Scan(&out.Users, &out.Sites, &out.Pageviews, &out.Events)
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

func realtimeHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.Realtime(c.Context(), db, auth.UserID(c), c.Params("id"))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func checksHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.LatestChecks(c.Context(), db, auth.UserID(c), c.Params("id"), 30)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func visitorsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.RecentVisitors(c.Context(), db, auth.UserID(c), c.Params("id"), 50)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func worldHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.World(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func exportHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.ExportCSV(c.Context(), db, auth.UserID(c), c.Params("id"), c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="webstats-export.csv"`)
		return c.Send(out)
	}
}

func writeCSVSection(w *csv.Writer, title string, header []string, rows [][]string) {
	w.Write([]string{title})
	w.Write(header)
	for _, r := range rows {
		w.Write(r)
	}
	w.Write([]string{})
}
