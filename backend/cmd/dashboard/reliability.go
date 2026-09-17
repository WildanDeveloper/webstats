package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/config"
	"github.com/webstats/backend/internal/model"
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
		if in.PeriodSeconds == 0 {
			in.PeriodSeconds = 3600
		}
		if in.PeriodSeconds < 60 || in.PeriodSeconds > 2592000 || in.GraceSeconds < 0 || in.GraceSeconds > 604800 {
			return errJSON(c, 400, "period must be 60-2592000 seconds and grace 0-604800 seconds")
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
			UPDATE heartbeats SET last_ping_at = now() WHERE ping_key = $1`, key)
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
	run := func() {
		if err := ensureAlertState(ctx, db); err != nil {
			log.Printf("heartbeat state initialization failed: %v", err)
			return
		}
		if err := processHeartbeats(ctx, db); err != nil {
			log.Printf("heartbeat transition failed: %v", err)
			return
		}
		drainAlertEvents(ctx, db)
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

func processHeartbeats(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		WITH changed AS (
			UPDATE heartbeats SET status = 'late', last_alert_at = now()
			WHERE status <> 'late'
			  AND COALESCE(last_ping_at, created_at) < now() - (period_seconds::bigint + grace_seconds::bigint) * interval '1 second'
			RETURNING site_id, name, last_ping_at
		)
		INSERT INTO dashboard_alert_events (site_id, event, name)
		SELECT site_id, 'heartbeat_missed', name FROM changed WHERE last_ping_at IS NOT NULL`)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		WITH changed AS (
			UPDATE heartbeats SET status = 'up', last_alert_at = now()
			WHERE status = 'late'
			  AND last_ping_at >= now() - (period_seconds::bigint + grace_seconds::bigint) * interval '1 second'
			RETURNING site_id, name
		)
		INSERT INTO dashboard_alert_events (site_id, event, name)
		SELECT site_id, 'heartbeat_ok', name FROM changed`)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `UPDATE heartbeats SET status = 'up' WHERE status = 'new'
		AND last_ping_at >= now() - (period_seconds::bigint + grace_seconds::bigint) * interval '1 second'`)
	return err
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
	for rows.Next() {
		var daysCSV string
		var start, end int
		if rows.Scan(&daysCSV, &start, &end) != nil {
			continue
		}
		if maintenanceMatches(daysCSV, start, end, now) {
			return true
		}
	}
	return false
}

func maintenanceMatches(daysCSV string, start, end int, now time.Time) bool {
	now = now.UTC()
	minute := now.Hour()*60 + now.Minute()
	if start <= end {
		if minute < start || minute > end {
			return false
		}
	} else {
		if minute < start && minute > end {
			return false
		}
		if minute <= end {
			now = now.AddDate(0, 0, -1)
		}
	}
	day := strings.ToLower(now.Format("Mon"))
	for _, d := range strings.Split(daysCSV, ",") {
		if strings.ToLower(strings.TrimSpace(d)) == day {
			return true
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
