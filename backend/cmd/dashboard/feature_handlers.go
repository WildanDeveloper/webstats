package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/notify"
)

func campaignsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.Campaigns(c.Context(), db, auth.UserID(c), c.Params("id"),
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func listGoalsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.Goals(c.Context(), db, auth.UserID(c), c.Params("id"))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func createGoalHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			MatchType string `json:"match_type"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Path = strings.TrimSpace(in.Path)
		if in.Name == "" || in.Path == "" {
			return errJSON(c, 400, "name and path required")
		}
		if in.MatchType != "contains" && in.MatchType != "exact" {
			in.MatchType = "contains"
		}
		var ok bool
		_ = db.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM sites WHERE id = $1 AND user_id = $2)`,
			c.Params("id"), auth.UserID(c)).Scan(&ok)
		if !ok {
			return errJSON(c, 404, "site not found")
		}
		var g analytics.Goal
		err := db.QueryRow(c.Context(), `
			INSERT INTO goals (site_id, name, path, match_type)
			VALUES ($1,$2,$3,$4) RETURNING id, site_id, name, path, match_type, created_at`,
			c.Params("id"), in.Name, in.Path, in.MatchType).
			Scan(&g.ID, &g.SiteID, &g.Name, &g.Path, &g.MatchType, &g.CreatedAt)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(g)
	}
}

func deleteGoalHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tag, err := db.Exec(c.Context(), `
			DELETE FROM goals WHERE id = $1 AND site_id = $2 AND
			site_id IN (SELECT id FROM sites WHERE user_id = $3)`,
			c.Params("goal_id"), c.Params("id"), auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "goal not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func goalSummariesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		out, err := analytics.Q.GoalSummaries(c.Context(), db, auth.UserID(c), c.Params("id"),
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func funnelHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Paths []string `json:"paths"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		if len(in.Paths) < 2 {
			return errJSON(c, 400, "at least 2 funnel steps required")
		}
		for i := range in.Paths {
			in.Paths[i] = strings.TrimSpace(in.Paths[i])
		}
		out, err := analytics.Q.Funnel(c.Context(), db, auth.UserID(c), c.Params("id"),
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), in.Paths, filtersFromQuery(c))
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "site not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

type Report struct {
	ID         string     `json:"id"`
	SiteID     string     `json:"site_id"`
	SiteName   string     `json:"site_name"`
	Domain     string     `json:"domain"`
	ProviderID string     `json:"provider_id"`
	Provider   string     `json:"provider_name"`
	Recipient  string     `json:"recipient"`
	Frequency  string     `json:"frequency"`
	Day        string     `json:"day"`
	Hour       int        `json:"hour"`
	Enabled    bool       `json:"enabled"`
	LastSentAt *time.Time `json:"last_sent_at"`
}

func listReportsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), `
			SELECT r.id, r.site_id, s.name, s.domain, r.provider_id, p.name, r.recipient,
			       r.frequency, r.day, r.hour, r.enabled, r.last_sent_at
			FROM notif_reports r
			JOIN sites s ON s.id = r.site_id
			JOIN notif_providers p ON p.id = r.provider_id
			WHERE r.user_id = $1 ORDER BY r.created_at DESC`, auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		out := make([]Report, 0)
		for rows.Next() {
			var r Report
			if err := rows.Scan(&r.ID, &r.SiteID, &r.SiteName, &r.Domain, &r.ProviderID, &r.Provider,
				&r.Recipient, &r.Frequency, &r.Day, &r.Hour, &r.Enabled, &r.LastSentAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, r)
		}
		return c.JSON(out)
	}
}

func createReportHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			SiteID     string `json:"site_id"`
			ProviderID string `json:"provider_id"`
			Recipient  string `json:"recipient"`
			Frequency  string `json:"frequency"`
			Day        string `json:"day"`
			Hour       int    `json:"hour"`
			Enabled    *bool  `json:"enabled"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		uid := auth.UserID(c)
		if in.SiteID == "" || in.ProviderID == "" || !strings.Contains(in.Recipient, "@") {
			return errJSON(c, 400, "site, provider and recipient email required")
		}
		if in.Frequency != "daily" && in.Frequency != "weekly" && in.Frequency != "monthly" {
			return errJSON(c, 400, "frequency must be daily, weekly or monthly")
		}
		if in.Hour < 0 || in.Hour > 23 {
			in.Hour = 8
		}
		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		var ok bool
		_ = db.QueryRow(c.Context(), `SELECT EXISTS(
			SELECT 1 FROM sites s JOIN notif_providers p ON p.user_id = s.user_id
			WHERE s.id = $1 AND s.user_id = $2 AND p.id = $3)`,
			in.SiteID, uid, in.ProviderID).Scan(&ok)
		if !ok {
			return errJSON(c, 404, "site or provider not found")
		}
		var id string
		err := db.QueryRow(c.Context(), `
			INSERT INTO notif_reports (user_id, site_id, provider_id, recipient, frequency, day, hour, enabled)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			uid, in.SiteID, in.ProviderID, in.Recipient, in.Frequency, in.Day, in.Hour, enabled).Scan(&id)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(fiber.Map{"id": id})
	}
}

func updateReportHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Recipient *string `json:"recipient"`
			Frequency *string `json:"frequency"`
			Day       *string `json:"day"`
			Hour      *int    `json:"hour"`
			Enabled   *bool   `json:"enabled"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		id := c.Params("id")
		uid := auth.UserID(c)
		if in.Frequency != nil && *in.Frequency != "daily" && *in.Frequency != "weekly" && *in.Frequency != "monthly" {
			return errJSON(c, 400, "frequency must be daily, weekly or monthly")
		}
		if in.Recipient != nil && !strings.Contains(*in.Recipient, "@") {
			return errJSON(c, 400, "invalid recipient email")
		}
		if in.Hour != nil && (*in.Hour < 0 || *in.Hour > 23) {
			return errJSON(c, 400, "hour must be 0-23")
		}
		tag, err := db.Exec(c.Context(), `
			UPDATE notif_reports SET
				recipient = COALESCE(NULLIF($1, ''), recipient),
				frequency = COALESCE(NULLIF($2, ''), frequency),
				day       = COALESCE(NULLIF($3, ''), day),
				hour      = COALESCE($4, hour),
				enabled   = COALESCE($5, enabled)
			WHERE id = $6 AND user_id = $7`,
			orEmpty(in.Recipient), orEmpty(in.Frequency), orEmpty(in.Day), in.Hour, in.Enabled, id, uid)
		if err != nil {
			return errJSON(c, 500, "update failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "report not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func deleteReportHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tag, err := db.Exec(c.Context(), `
			DELETE FROM notif_reports WHERE id = $1 AND user_id = $2`, c.Params("id"), auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "report not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func testReportHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		uid := auth.UserID(c)
		var siteID, domain, siteName, recipient, kind string
		var cfg map[string]any
		var fromEmail string
		err := db.QueryRow(c.Context(), `
			SELECT r.site_id, s.domain, s.name, r.recipient, p.kind, p.config, p.from_email
			FROM notif_reports r
			JOIN sites s ON s.id = r.site_id
			JOIN notif_providers p ON p.id = r.provider_id
			WHERE r.id = $1 AND r.user_id = $2`, id, uid).
			Scan(&siteID, &domain, &siteName, &recipient, &kind, &cfg, &fromEmail)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "report not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		ctxT, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		body, err := buildReport(ctxT, db, siteID, "30d", siteName, domain, kind, cfg, fromEmail, recipient)
		cancel()
		if err != nil {
			return errJSON(c, 502, "report build failed: "+err.Error())
		}
		status, detail := "ok", ""
		sendErr := sendReportEmail(context.Background(), body)
		if sendErr != nil {
			status, detail = "fail", sendErr.Error()
		}
		_, _ = db.Exec(c.Context(), `
			INSERT INTO notif_logs (user_id, site_id, event, channel, status, detail)
			VALUES ($1,$2,'report_test','email',$3,$4)`, uid, siteID, status, detail)
		if sendErr != nil {
			return errJSON(c, 502, "send failed: "+sendErr.Error())
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func reportLoop(ctx context.Context, pool *pgxpool.Pool) {
	run := func() {
		now := time.Now().UTC()
		rows, err := pool.Query(ctx, `
			SELECT r.id, r.user_id, r.site_id, s.name, s.domain, r.recipient,
			       p.kind, p.config, p.from_email, r.frequency, r.day, r.hour
			FROM notif_reports r
			JOIN sites s ON s.id = r.site_id
			JOIN notif_providers p ON p.id = r.provider_id
			WHERE r.enabled AND r.hour = $1
			  AND (r.last_sent_at IS NULL OR r.last_sent_at < date_trunc('day', now()))
			  AND (
			    r.frequency = 'daily'
			    OR (r.frequency = 'weekly' AND lower(r.day) = lower(to_char(now(), 'Day')::text))
			    OR (r.frequency = 'monthly' AND r.day = to_char(now(), 'DD'))
			  )`, now.Hour())
		if err != nil {
			return
		}
		defer rows.Close()
		type row struct {
			id, userID, siteID, name, domain, recipient, kind string
			cfg                                               map[string]any
			fromEmail                                         string
		}
		var due []row
		for rows.Next() {
			var r row
			if rows.Scan(&r.id, &r.userID, &r.siteID, &r.name, &r.domain, &r.recipient,
				&r.kind, &r.cfg, &r.fromEmail, new(string), new(string), new(int)) == nil {
				due = append(due, r)
			}
		}
		rows.Close()
		for _, r := range due {
			period := "30d"
			if rows2, err := pool.Query(ctx, `SELECT 1 FROM notif_reports WHERE id = $1`, r.id); err == nil {
				rows2.Close()
			}
			_ = period
			body, err := buildReport(ctx, pool, r.siteID, "30d", r.name, r.domain, r.kind, r.cfg, r.fromEmail, r.recipient)
			if err != nil {
				logReport(ctx, pool, r.userID, r.siteID, "report", "email", "fail", "build: "+err.Error())
				continue
			}
			if err := sendReportEmail(ctx, body); err != nil {
				logReport(ctx, pool, r.userID, r.siteID, "report", "email", "fail", err.Error())
				continue
			}
			logReport(ctx, pool, r.userID, r.siteID, "report", "email", "ok", "sent "+r.recipient)
			_, _ = pool.Exec(ctx, `UPDATE notif_reports SET last_sent_at = now() WHERE id = $1`, r.id)
		}
	}
	run()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func logReport(ctx context.Context, pool *pgxpool.Pool, userID, siteID, event, channel, status, detail string) {
	_, _ = pool.Exec(ctx, `
		INSERT INTO notif_logs (user_id, site_id, event, channel, status, detail)
		VALUES ($1,$2,$3,$4,$5,$6)`, userID, siteID, event, channel, status, detail)
}

func buildReport(ctx context.Context, pool *pgxpool.Pool, siteID, period, siteName, domain, kind string,
	cfg map[string]any, fromEmail, recipient string) (reportBody, error) {

	var b reportBody
	var out struct {
		Pageviews int64 `json:"pageviews"`
		Visitors  int64 `json:"visitors"`
		Sessions  int64 `json:"sessions"`
	}
	ov, err := analytics.Q.Overview(ctx, pool, ownerID(ctx, pool, siteID), siteID, period, "", "", analytics.Filters{})
	if err != nil {
		return b, err
	}
	pages, _ := analytics.Q.Top(ctx, pool, ownerID(ctx, pool, siteID), siteID, period, "path", 5, "", "", analytics.Filters{})
	refs, _ := analytics.Q.Top(ctx, pool, ownerID(ctx, pool, siteID), siteID, period, "referrer", 5, "", "", analytics.Filters{})
	out.Pageviews = ov.Pageviews
	out.Visitors = ov.Visitors
	out.Sessions = ov.Sessions

	var rows string
	add := func(k, v string) {
		rows += fmt.Sprintf(`<tr><td style="padding:6px 14px;color:#6b7280;font-size:13px">%s</td><td style="padding:6px 14px;color:#111827;font-size:13px;font-weight:600;text-align:right">%s</td></tr>`, k, v)
	}
	add("Pageviews", fmt.Sprint(out.Pageviews))
	add("Visitors", fmt.Sprint(out.Visitors))
	add("Sessions", fmt.Sprint(out.Sessions))
	topPages := ""
	for _, p := range pages {
		topPages += `<tr><td style="padding:4px 14px;color:#111827;font-size:13px">` + p.Key + `</td><td style="padding:4px 14px;color:#6b7280;font-size:13px;text-align:right">` + fmt.Sprint(p.Value) + `</td></tr>`
	}
	topRefs := ""
	for _, r := range refs {
		topRefs += `<tr><td style="padding:4px 14px;color:#111827;font-size:13px">` + r.Key + `</td><td style="padding:4px 14px;color:#6b7280;font-size:13px;text-align:right">` + fmt.Sprint(r.Value) + `</td></tr>`
	}
	b.Subject = fmt.Sprintf("[WebStats] %s — 30 day report", siteName)
	b.HTML = `<div style="max-width:560px;margin:24px auto;background:#fff;border:1px solid #e5e7eb;border-radius:12px;overflow:hidden;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif">
<div style="background:#111827;color:#fff;padding:16px 24px;font-size:16px;font-weight:700">` + siteName + ` — 30 day report</div>
<div style="padding:16px 24px">
<p style="color:#6b7280;font-size:13px">` + domain + ` · generated ` + time.Now().UTC().Format("02 Jan 2006 15:04 UTC") + `</p>
<table style="width:100%;border-collapse:collapse">` + rows + `</table>
<h3 style="font-size:13px;color:#111827;margin:18px 0 6px">Top pages</h3>
<table style="width:100%;border-collapse:collapse">` + topPages + `</table>
<h3 style="font-size:13px;color:#111827;margin:18px 0 6px">Top referrers</h3>
<table style="width:100%;border-collapse:collapse">` + topRefs + `</table>
</div>
<div style="padding:12px 24px;color:#9ca3af;font-size:12px">Sent by WebStats</div>
</div>`
	b.From = fromEmail
	b.To = recipient
	b.Kind = kind
	b.Cfg = cfg
	return b, nil
}

type reportBody struct {
	From, To, Subject, HTML, Kind string
	Cfg                           map[string]any
}

func sendReportEmail(ctx context.Context, b reportBody) error {
	sender, err := notify.NewSender(b.Kind, b.Cfg, b.From)
	if err != nil {
		return err
	}
	return sender.Send(ctx, notify.Message{From: b.From, To: b.To, Subject: b.Subject, HTML: b.HTML})
}

func ownerID(ctx context.Context, pool *pgxpool.Pool, siteID string) string {
	var id string
	_ = pool.QueryRow(ctx, `SELECT user_id FROM sites WHERE id = $1`, siteID).Scan(&id)
	return id
}
