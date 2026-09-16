package main

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/model"
	"github.com/webstats/backend/internal/notify"
)

// ---------- C1: heartbeats (dead man's switch) ----------

func listHeartbeatsHandler(db *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		rows, err := db.Query(c.Context(), `
			SELECT id, site_id, name, period_seconds, grace_seconds, ping_key, last_ping_at, status, created_at
			FROM heartbeats WHERE site_id = $1 ORDER BY created_at`, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		out := make([]model.Heartbeat, 0)
		for rows.Next() {
			var h model.Heartbeat
			if err := rows.Scan(&h.ID, &h.SiteID, &h.Name, &h.PeriodSeconds, &h.GraceSeconds,
				&h.PingKey, &h.LastPingAt, &h.Status, &h.CreatedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			h.PingURL = strings.TrimRight(cfg.APIPublicURL, "/") + "/api/ping/" + h.PingKey
			out = append(out, h)
		}
		return c.JSON(out)
	}
}

func createHeartbeatHandler(db *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, auth.UserID(c)) {
			return errJSON(c, 404, "site not found")
		}
		var in struct {
			Name          string `json:"name"`
			PeriodSeconds int    `json:"period_seconds"`
			GraceSeconds  int    `json:"grace_seconds"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		in.Name = strings.TrimSpace(in.Name)
		if in.Name == "" {
			return errJSON(c, 400, "name required")
		}
		if len(in.Name) > 80 {
			in.Name = in.Name[:80]
		}
		if in.PeriodSeconds < 60 {
			in.PeriodSeconds = 3600
		}
		if in.GraceSeconds < 0 {
			in.GraceSeconds = 600
		}
		key, err := randHex(16)
		if err != nil {
			return errJSON(c, 500, "key gen failed")
		}
		var h model.Heartbeat
		err = db.QueryRow(c.Context(), `
			INSERT INTO heartbeats (site_id, name, period_seconds, grace_seconds, ping_key)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING id, site_id, name, period_seconds, grace_seconds, ping_key, last_ping_at, status, created_at`,
			siteID, in.Name, in.PeriodSeconds, in.GraceSeconds, key).
			Scan(&h.ID, &h.SiteID, &h.Name, &h.PeriodSeconds, &h.GraceSeconds,
				&h.PingKey, &h.LastPingAt, &h.Status, &h.CreatedAt)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		h.PingURL = strings.TrimRight(cfg.APIPublicURL, "/") + "/api/ping/" + h.PingKey
		return c.Status(201).JSON(h)
	}
}

func deleteHeartbeatHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, auth.UserID(c)) {
			return errJSON(c, 404, "site not found")
		}
		tag, err := db.Exec(c.Context(), `
			DELETE FROM heartbeats WHERE id = $1 AND site_id = $2`, c.Params("hid"), siteID)
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "heartbeat not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// heartbeatPingHandler is the public dead man's switch endpoint. Cron jobs
// and workers curl it on every run; the loop alerts when beats go silent.
func heartbeatPingHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Params("key")
		if key == "" || len(key) > 64 {
			return errJSON(c, 400, "missing key")
		}
		tag, err := db.Exec(c.Context(), `
			UPDATE heartbeats SET last_ping_at = now(), status = 'up' WHERE ping_key = $1`, key)
		if err != nil {
			return errJSON(c, 500, "ping failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "unknown ping key")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// heartbeatLoop marks beats late once silence exceeds period + grace and
// fires heartbeat_missed / heartbeat_ok rules (1h cooldown for misses).
func heartbeatLoop(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	fire := func(siteID, event, name string) {
		if inMaintenance(ctx, db, siteID, time.Now().UTC()) {
			return
		}
		rows, err := db.Query(ctx, `
			SELECT r.user_id, r.id, r.channel, r.target, r.provider_id, r.params, s.name, s.domain
			FROM notif_rules r JOIN sites s ON s.id = r.site_id
			WHERE r.site_id = $1 AND r.event = $2 AND r.enabled
			  AND (r.last_sent_at IS NULL OR r.last_sent_at < now() - interval '5 minutes')`, siteID, event)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var userID, ruleID, channel, target string
			var providerID *string
			var params map[string]any
			var siteName, domain string
			if rows.Scan(&userID, &ruleID, &channel, &target, &providerID, &params, &siteName, &domain) != nil {
				continue
			}
			payload := notify.AlertPayload{
				Event: event, SiteID: siteID, SiteName: siteName, Domain: domain,
				Name:   name,
				Status: "late", Time: time.Now().UTC().Format(time.RFC3339),
			}
			if event == "heartbeat_ok" {
				payload.Status = "ok"
			}
			_, _ = deliverRule(ctx, db, userID, channel, target, providerID, params, payload, false)
			_, _ = db.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, ruleID)
		}
	}
	run := func() {
		// Going late: was up/new, silence exceeded period+grace.
		rows, err := db.Query(ctx, `
			SELECT id, site_id, name, COALESCE(last_alert_at, to_timestamp(0)) FROM heartbeats
			WHERE status <> 'late'
			  AND (last_ping_at IS NULL OR last_ping_at < now() - (period_seconds + grace_seconds) * interval '1 second')
			  AND (last_ping_at IS NOT NULL OR created_at < now() - (period_seconds + grace_seconds) * interval '1 second')`)
		if err != nil {
			return
		}
		type lateRow struct {
			id, siteID, name string
			lastAlert        time.Time
		}
		var late []lateRow
		for rows.Next() {
			var r lateRow
			if rows.Scan(&r.id, &r.siteID, &r.name, &r.lastAlert) == nil {
				late = append(late, r)
			}
		}
		rows.Close()
		for _, r := range late {
			if _, err := db.Exec(ctx, `UPDATE heartbeats SET status = 'late', last_alert_at = now() WHERE id = $1`, r.id); err != nil {
				continue
			}
			// A beat that never arrived at all (new heartbeat) stays silent
			// to the owner until the first ping, so a typo'd cron command
			// does not page anyone; afterwards every gap alerts.
			var seen bool
			_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM heartbeats WHERE id = $1 AND last_ping_at IS NOT NULL)`, r.id).Scan(&seen)
			if seen && time.Since(r.lastAlert) > time.Hour {
				fire(r.siteID, "heartbeat_missed", r.name)
			}
		}
		// Recovery: late beat received a fresh ping again.
		rows2, err := db.Query(ctx, `
			SELECT id, site_id, name FROM heartbeats
			WHERE status = 'late' AND last_ping_at >= now() - (period_seconds + grace_seconds) * interval '1 second'`)
		if err != nil {
			return
		}
		type okRow struct {
			id, siteID, name string
		}
		var okRows []okRow
		for rows2.Next() {
			var r okRow
			if rows2.Scan(&r.id, &r.siteID, &r.name) == nil {
				okRows = append(okRows, r)
			}
		}
		rows2.Close()
		for _, r := range okRows {
			if _, err := db.Exec(ctx, `UPDATE heartbeats SET status = 'up', last_alert_at = now() WHERE id = $1`, r.id); err != nil {
				continue
			}
			fire(r.siteID, "heartbeat_ok", r.name)
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

// ---------- C4: maintenance windows ----------

// inMaintenance reports whether a site is inside one of its maintenance
// windows right now (UTC). Used to suppress down/up alerts for planned work.
func inMaintenance(ctx context.Context, db *pgxpool.Pool, siteID string, now time.Time) bool {
	rows, err := db.Query(ctx, `
		SELECT days_csv, start_minute, end_minute FROM maintenance_windows WHERE site_id = $1`, siteID)
	if err != nil {
		return false
	}
	defer rows.Close()
	day := strings.ToLower(now.UTC().Format("Mon"))
	minute := now.UTC().Hour()*60 + now.UTC().Minute()
	for rows.Next() {
		var daysCSV string
		var start, end int
		if rows.Scan(&daysCSV, &start, &end) != nil {
			continue
		}
		match := false
		for _, d := range strings.Split(daysCSV, ",") {
			if strings.ToLower(strings.TrimSpace(d)) == day {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		if start <= end {
			if minute >= start && minute <= end {
				return true
			}
		} else {
			// Window wraps midnight (e.g. 23:00-04:00).
			if minute >= start || minute <= end {
				return true
			}
		}
	}
	return false
}

var validDays = map[string]bool{"mon": true, "tue": true, "wed": true, "thu": true, "fri": true, "sat": true, "sun": true}

func listMaintenanceHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		rows, err := db.Query(c.Context(), `
			SELECT id, site_id, days_csv, start_minute, end_minute, created_at
			FROM maintenance_windows WHERE site_id = $1 ORDER BY created_at`, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		out := make([]model.MaintenanceWindow, 0)
		for rows.Next() {
			var w model.MaintenanceWindow
			var daysCSV string
			if err := rows.Scan(&w.ID, &w.SiteID, &daysCSV, &w.StartMinute, &w.EndMinute, &w.CreatedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			for _, d := range strings.Split(daysCSV, ",") {
				if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
					w.Days = append(w.Days, d)
				}
			}
			out = append(out, w)
		}
		return c.JSON(out)
	}
}

func createMaintenanceHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, auth.UserID(c)) {
			return errJSON(c, 404, "site not found")
		}
		var in struct {
			Days        []string `json:"days"`
			StartMinute *int     `json:"start_minute"`
			EndMinute   *int     `json:"end_minute"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		var days []string
		for _, d := range in.Days {
			d = strings.ToLower(strings.TrimSpace(d))
			if validDays[d] {
				days = append(days, d)
			}
		}
		if len(days) == 0 {
			return errJSON(c, 400, "at least one valid day (mon..sun) required")
		}
		start, end := 0, 1439
		if in.StartMinute != nil {
			start = *in.StartMinute
		}
		if in.EndMinute != nil {
			end = *in.EndMinute
		}
		if start < 0 || start > 1439 || end < 0 || end > 1439 {
			return errJSON(c, 400, "minutes must be 0-1439")
		}
		var w model.MaintenanceWindow
		var daysCSV string
		err := db.QueryRow(c.Context(), `
			INSERT INTO maintenance_windows (site_id, days_csv, start_minute, end_minute)
			VALUES ($1,$2,$3,$4)
			RETURNING id, site_id, days_csv, start_minute, end_minute, created_at`,
			siteID, strings.Join(days, ","), start, end).
			Scan(&w.ID, &w.SiteID, &daysCSV, &w.StartMinute, &w.EndMinute, &w.CreatedAt)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		for _, d := range strings.Split(daysCSV, ",") {
			w.Days = append(w.Days, d)
		}
		return c.Status(201).JSON(w)
	}
}

func deleteMaintenanceHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, auth.UserID(c)) {
			return errJSON(c, 404, "site not found")
		}
		tag, err := db.Exec(c.Context(), `
			DELETE FROM maintenance_windows WHERE id = $1 AND site_id = $2`, c.Params("wid"), siteID)
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "maintenance window not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ---------- C6: incident timeline ----------

func incidentsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		limit := c.QueryInt("limit", 30)
		if limit < 1 {
			limit = 1
		}
		if limit > 100 {
			limit = 100
		}
		rows, err := db.Query(c.Context(), `
			SELECT id, site_id, monitor_id, kind, started_at, resolved_at, reason
			FROM incidents WHERE site_id = $1
			ORDER BY started_at DESC LIMIT $2`, siteID, limit)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		out := make([]model.Incident, 0)
		for rows.Next() {
			var i model.Incident
			if err := rows.Scan(&i.ID, &i.SiteID, &i.MonitorID, &i.Kind, &i.StartedAt, &i.ResolvedAt, &i.Reason); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, i)
		}
		return c.JSON(out)
	}
}
