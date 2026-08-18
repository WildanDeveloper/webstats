package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/notify"
)

const mask = "••••••••"

var providerSecrets = map[string][]string{
	"resend":   {"api_key"},
	"sendgrid": {"api_key"},
	"mailgun":  {"api_key"},
	"postmark": {"server_token"},
	"brevo":    {"api_key"},
	"smtp":     {"pass"},
}

func maskConfig(kind string, cfg map[string]any) map[string]any {
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		out[k] = v
	}
	for _, key := range providerSecrets[kind] {
		if s, ok := out[key].(string); ok && s != "" {
			out[key] = mask
		}
	}
	return out
}

func validProviderKind(k string) bool {
	for _, kd := range notify.ProviderKinds {
		if kd == k {
			return true
		}
	}
	return false
}

// ---------- providers ----------

func listProvidersHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), `
			SELECT id, name, kind, config, from_email, created_at
			FROM notif_providers WHERE user_id = $1 ORDER BY created_at`, auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		type provider struct {
			ID        string         `json:"id"`
			Name      string         `json:"name"`
			Kind      string         `json:"kind"`
			Config    map[string]any `json:"config"`
			FromEmail string         `json:"from_email"`
			CreatedAt time.Time      `json:"created_at"`
		}
		var out []provider
		for rows.Next() {
			var p provider
			if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.Config, &p.FromEmail, &p.CreatedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			p.Config = maskConfig(p.Kind, p.Config)
			out = append(out, p)
		}
		return c.JSON(out)
	}
}

func createProviderHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name      string         `json:"name"`
			Kind      string         `json:"kind"`
			Config    map[string]any `json:"config"`
			FromEmail string         `json:"from_email"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Name = strings.TrimSpace(in.Name)
		in.FromEmail = strings.TrimSpace(in.FromEmail)
		if in.Name == "" || !validProviderKind(in.Kind) {
			return errJSON(c, 400, "name and a valid kind (smtp, resend, sendgrid, mailgun, postmark, brevo) required")
		}
		if !strings.Contains(in.FromEmail, "@") {
			return errJSON(c, 400, "a valid from email is required")
		}
		if _, err := notify.NewSender(in.Kind, in.Config, in.FromEmail); err != nil {
			return errJSON(c, 400, "provider config invalid: "+err.Error())
		}
		var id string
		err := db.QueryRow(c.Context(), `
			INSERT INTO notif_providers (user_id, name, kind, config, from_email)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			auth.UserID(c), in.Name, in.Kind, in.Config, in.FromEmail).Scan(&id)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(fiber.Map{"id": id})
	}
}

func updateProviderHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Name      *string        `json:"name"`
			FromEmail *string        `json:"from_email"`
			Config    map[string]any `json:"config"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		id := c.Params("id")
		uid := auth.UserID(c)
		var kind string
		var oldConfig map[string]any
		err := db.QueryRow(c.Context(), `
			SELECT kind, config FROM notif_providers WHERE id = $1 AND user_id = $2`, id, uid).Scan(&kind, &oldConfig)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "provider not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		cfg := oldConfig
		if in.Config != nil {
			cfg = in.Config
			for _, key := range providerSecrets[kind] {
				if v, ok := cfg[key].(string); ok && (v == "" || v == mask) {
					cfg[key] = oldConfig[key]
				}
			}
			if _, err := notify.NewSender(kind, cfg, "x@x.com"); err != nil {
				return errJSON(c, 400, "provider config invalid: "+err.Error())
			}
		}
		fromEmail := ""
		if in.FromEmail != nil {
			fromEmail = strings.TrimSpace(*in.FromEmail)
			if !strings.Contains(fromEmail, "@") {
				return errJSON(c, 400, "invalid from email")
			}
		}
		tag, err := db.Exec(c.Context(), `
			UPDATE notif_providers SET
				name = COALESCE(NULLIF($1, ''), name),
				from_email = COALESCE(NULLIF($2, ''), from_email),
				config = $3
			WHERE id = $4 AND user_id = $5`,
			orEmpty(in.Name), fromEmail, cfg, id, uid)
		if err != nil {
			return errJSON(c, 500, "update failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "provider not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func deleteProviderHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tag, err := db.Exec(c.Context(), `
			DELETE FROM notif_providers WHERE id = $1 AND user_id = $2`, c.Params("id"), auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "provider not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func testProviderHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		uid := auth.UserID(c)
		var kind string
		var cfg map[string]any
		var fromEmail string
		var userEmail string
		err := db.QueryRow(c.Context(), `
			SELECT p.kind, p.config, p.from_email, u.email
			FROM notif_providers p JOIN users u ON u.id = p.user_id
			WHERE p.id = $1 AND p.user_id = $2`, id, uid).Scan(&kind, &cfg, &fromEmail, &userEmail)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "provider not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		sender, err := notify.NewSender(kind, cfg, fromEmail)
		if err != nil {
			return errJSON(c, 400, "provider config invalid: "+err.Error())
		}
		msg := notify.Message{
			From:    fromEmail,
			To:      userEmail,
			Subject: "[WebStats] Test notification",
			HTML:    "<p>This is a test email from your WebStats notification provider <b>" + kind + "</b>.</p>",
			Text:    "This is a test email from WebStats.",
		}
		err = sender.Send(c.Context(), msg)
		status, detail := "ok", ""
		if err != nil {
			status, detail = "fail", err.Error()
		}
		_, _ = db.Exec(c.Context(), `
			INSERT INTO notif_logs (user_id, event, channel, status, detail)
			VALUES ($1, 'provider_test', 'email', $2, $3)`, uid, status, detail)
		if err != nil {
			return errJSON(c, 502, "send failed: "+err.Error())
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ---------- rules ----------

func listRulesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q := `
			SELECT r.id, r.site_id, s.name, s.domain, r.event, r.channel, r.target,
			       COALESCE(r.provider_id::text, ''), COALESCE(p.name, ''), r.params, r.enabled, r.last_sent_at
			FROM notif_rules r
			JOIN sites s ON s.id = r.site_id
			LEFT JOIN notif_providers p ON p.id = r.provider_id
			WHERE r.user_id = $1`
		args := []any{auth.UserID(c)}
		if siteID := c.Query("site_id"); siteID != "" {
			q += " AND r.site_id = $2"
			args = append(args, siteID)
		}
		q += " ORDER BY r.created_at DESC"
		rows, err := db.Query(c.Context(), q, args...)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		type rule struct {
			ID         string         `json:"id"`
			SiteID     string         `json:"site_id"`
			SiteName   string         `json:"site_name"`
			Domain     string         `json:"domain"`
			Event      string         `json:"event"`
			Channel    string         `json:"channel"`
			Target     string         `json:"target"`
			ProviderID string         `json:"provider_id"`
			Provider   string         `json:"provider_name"`
			Params     map[string]any `json:"params"`
			Enabled    bool           `json:"enabled"`
			LastSentAt *time.Time     `json:"last_sent_at"`
		}
		var out []rule
		for rows.Next() {
			var r rule
			if err := rows.Scan(&r.ID, &r.SiteID, &r.SiteName, &r.Domain, &r.Event, &r.Channel, &r.Target,
				&r.ProviderID, &r.Provider, &r.Params, &r.Enabled, &r.LastSentAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, r)
		}
		return c.JSON(out)
	}
}

func createRuleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			SiteID     string         `json:"site_id"`
			Event      string         `json:"event"`
			Channel    string         `json:"channel"`
			ProviderID string         `json:"provider_id"`
			Target     string         `json:"target"`
			Params     map[string]any `json:"params"`
			Enabled    *bool          `json:"enabled"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		if in.SiteID == "" || (in.Event != "site_down" && in.Event != "site_up" && in.Event != "traffic_spike") {
			return errJSON(c, 400, "valid site_id and event (site_down, site_up, traffic_spike) required")
		}
		if in.Channel != "email" && in.Channel != "webhook" {
			return errJSON(c, 400, "channel must be email or webhook")
		}
		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		if in.Params == nil {
			in.Params = map[string]any{}
		}
		uid := auth.UserID(c)
		if in.Channel == "email" {
			if in.ProviderID == "" {
				return errJSON(c, 400, "choose an email provider")
			}
			var ok bool
			_ = db.QueryRow(c.Context(), `
				SELECT EXISTS(SELECT 1 FROM notif_providers WHERE id = $1 AND user_id = $2)`,
				in.ProviderID, uid).Scan(&ok)
			if !ok {
				return errJSON(c, 404, "provider not found")
			}
			if !strings.Contains(in.Target, "@") {
				return errJSON(c, 400, "recipient email required")
			}
		} else {
			if !strings.HasPrefix(in.Target, "http://") && !strings.HasPrefix(in.Target, "https://") {
				return errJSON(c, 400, "webhook URL must start with http(s)://")
			}
		}
		var owned bool
		_ = db.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM sites WHERE id = $1 AND user_id = $2)`,
			in.SiteID, uid).Scan(&owned)
		if !owned {
			return errJSON(c, 404, "site not found")
		}
		var id string
		err := db.QueryRow(c.Context(), `
			INSERT INTO notif_rules (user_id, site_id, event, channel, provider_id, target, params, enabled)
			VALUES ($1,$2,$3,$4,NULLIF($5, '')::uuid,$6,$7,$8) RETURNING id`,
			uid, in.SiteID, in.Event, in.Channel, in.ProviderID, strings.TrimSpace(in.Target), in.Params, enabled).Scan(&id)
		if err != nil {
			return errJSON(c, 500, "insert failed: "+err.Error())
		}
		return c.Status(201).JSON(fiber.Map{"id": id})
	}
}

func updateRuleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var in struct {
			Event      *string        `json:"event"`
			Channel    *string        `json:"channel"`
			ProviderID *string        `json:"provider_id"`
			Target     *string        `json:"target"`
			Params     map[string]any `json:"params"`
			Enabled    *bool          `json:"enabled"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		id := c.Params("id")
		uid := auth.UserID(c)
		var channel string
		err := db.QueryRow(c.Context(), `SELECT channel FROM notif_rules WHERE id = $1 AND user_id = $2`, id, uid).Scan(&channel)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "rule not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		if in.Channel != nil {
			channel = *in.Channel
			if channel != "email" && channel != "webhook" {
				return errJSON(c, 400, "channel must be email or webhook")
			}
		}
		if in.ProviderID != nil && *in.ProviderID != "" {
			var ok bool
			_ = db.QueryRow(c.Context(), `
				SELECT EXISTS(SELECT 1 FROM notif_providers WHERE id = $1 AND user_id = $2)`,
				*in.ProviderID, uid).Scan(&ok)
			if !ok {
				return errJSON(c, 404, "provider not found")
			}
		}
		params := map[string]any{}
		if in.Params != nil {
			params = in.Params
		}
		target := ""
		if in.Target != nil {
			target = strings.TrimSpace(*in.Target)
			if channel == "email" && !strings.Contains(target, "@") {
				return errJSON(c, 400, "recipient email required")
			}
			if channel == "webhook" && !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
				return errJSON(c, 400, "webhook URL must start with http(s)://")
			}
		}
		tag, err := db.Exec(c.Context(), `
			UPDATE notif_rules SET
				event = COALESCE(NULLIF($1, ''), event),
				channel = $2,
				provider_id = NULLIF($3, '')::uuid,
				target = COALESCE(NULLIF($4, ''), target),
				params = $5,
				enabled = COALESCE($6, enabled)
			WHERE id = $7 AND user_id = $8`,
			orEmpty(in.Event), channel, orEmpty(in.ProviderID), target, params, in.Enabled, id, uid)
		if err != nil {
			return errJSON(c, 500, "update failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "rule not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func deleteRuleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tag, err := db.Exec(c.Context(), `
			DELETE FROM notif_rules WHERE id = $1 AND user_id = $2`, c.Params("id"), auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "rule not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ---------- test delivery ----------

func testRuleHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		uid := auth.UserID(c)
		var siteID, domain, siteName, event, channel, target string
		var providerID *string
		var params map[string]any
		err := db.QueryRow(c.Context(), `
			SELECT r.site_id, s.domain, s.name, r.event, r.channel, r.target, r.provider_id, r.params
			FROM notif_rules r JOIN sites s ON s.id = r.site_id
			WHERE r.id = $1 AND r.user_id = $2`, id, uid).
			Scan(&siteID, &domain, &siteName, &event, &channel, &target, &providerID, &params)
		if errors.Is(err, pgx.ErrNoRows) {
			return errJSON(c, 404, "rule not found")
		}
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		payload := notify.AlertPayload{
			Event: event, SiteID: siteID, SiteName: siteName, Domain: domain,
			Status: "test", Time: time.Now().Format(time.RFC3339),
		}
		detail, err := deliverRule(c.Context(), db, uid, channel, target, providerID, params, payload, true)
		if err != nil {
			return errJSON(c, 502, "delivery failed: "+err.Error())
		}
		return c.JSON(fiber.Map{"ok": true, "detail": detail})
	}
}

// deliverRule sends the payload through the rule channel and logs the attempt.
// Returns the log detail (ok message or the error message).
func deliverRule(ctx context.Context, db *pgxpool.Pool, uid, channel, target string,
	providerID *string, params map[string]any, payload notify.AlertPayload, isTest bool) (string, error) {

	status, detail := "ok", ""
	if channel == "webhook" {
		secret := ""
		if s, ok := params["secret"].(string); ok {
			secret = s
		}
		err := (&notify.Webhook{URL: target, Secret: secret}).Send(ctx, payloadMap(payload))
		if err != nil {
			status, detail = "fail", err.Error()
		}
	} else {
		var kind string
		var cfg map[string]any
		var fromEmail string
		err := db.QueryRow(ctx, `
			SELECT kind, config, from_email FROM notif_providers WHERE id = $1`, *providerID).
			Scan(&kind, &cfg, &fromEmail)
		if err != nil {
			status, detail = "fail", "provider not found"
		} else {
			sender, err := notify.NewSender(kind, cfg, fromEmail)
			if err != nil {
				status, detail = "fail", err.Error()
			} else if err := sender.Send(ctx, notify.Message{
				From: fromEmail, To: target,
				Subject: notify.EmailSubject(payload),
				HTML:    notify.Email(payload),
			}); err != nil {
				status, detail = "fail", err.Error()
			}
		}
	}
	if isTest {
		if detail == "" {
			detail = "test ok"
		} else {
			detail = "test fail: " + detail
		}
	}
	_, _ = db.Exec(ctx, `
		INSERT INTO notif_logs (user_id, site_id, event, channel, status, detail)
		VALUES ($1,$2,$3,$4,$5,$6)`, uid, payload.SiteID, payload.Event, channel, status, detail)
	if status == "fail" {
		return detail, errors.New(detail)
	}
	return detail, nil
}

func payloadMap(p notify.AlertPayload) map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// ---------- logs ----------

func logsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(c.Context(), `
			SELECT l.id, l.event, l.channel, l.status, l.detail, l.created_at,
			       COALESCE(s.name, ''), COALESCE(s.domain, '')
			FROM notif_logs l
			LEFT JOIN sites s ON s.id = l.site_id
			WHERE l.user_id = $1
			ORDER BY l.created_at DESC LIMIT 50`, auth.UserID(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		type entry struct {
			ID        int64     `json:"id"`
			Event     string    `json:"event"`
			Channel   string    `json:"channel"`
			Status    string    `json:"status"`
			Detail    string    `json:"detail"`
			CreatedAt time.Time `json:"created_at"`
			SiteName  string    `json:"site_name"`
			Domain    string    `json:"domain"`
		}
		var out []entry
		for rows.Next() {
			var e entry
			if err := rows.Scan(&e.ID, &e.Event, &e.Channel, &e.Status, &e.Detail, &e.CreatedAt, &e.SiteName, &e.Domain); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, e)
		}
		return c.JSON(out)
	}
}
