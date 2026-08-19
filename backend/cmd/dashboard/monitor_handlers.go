package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/model"
)

func listMonitorsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		rows, err := db.Query(c.Context(), `
			SELECT id, site_id, url, interval_seconds, expected_status, enabled,
			       last_status, last_ok, last_check_at, uptime_pct, created_at
			FROM monitors WHERE site_id = $1 ORDER BY created_at`, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var out []model.Monitor
		for rows.Next() {
			var m model.Monitor
			if err := rows.Scan(&m.ID, &m.SiteID, &m.URL, &m.IntervalSeconds, &m.ExpectedStatus, &m.Enabled,
				&m.LastStatus, &m.LastOK, &m.LastCheckAt, &m.UptimePct, &m.CreatedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, m)
		}
		if out == nil {
			out = []model.Monitor{}
		}
		return c.JSON(out)
	}
}

func createMonitorHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, auth.UserID(c)) {
			return errJSON(c, 404, "site not found")
		}
		var in struct {
			URL             string `json:"url"`
			IntervalSeconds int    `json:"interval_seconds"`
			ExpectedStatus  int    `json:"expected_status"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		if in.URL == "" {
			return errJSON(c, 400, "url required")
		}
		if in.IntervalSeconds < 30 {
			in.IntervalSeconds = 60
		}
		if in.ExpectedStatus == 0 {
			in.ExpectedStatus = 200
		}
		var m model.Monitor
		err := db.QueryRow(c.Context(), `
			INSERT INTO monitors (site_id, url, interval_seconds, expected_status)
			VALUES ($1, $2, $3, $4)
			RETURNING id, site_id, url, interval_seconds, expected_status, enabled,
			          last_status, last_ok, last_check_at, uptime_pct, created_at`,
			siteID, in.URL, in.IntervalSeconds, in.ExpectedStatus).Scan(
			&m.ID, &m.SiteID, &m.URL, &m.IntervalSeconds, &m.ExpectedStatus, &m.Enabled,
			&m.LastStatus, &m.LastOK, &m.LastCheckAt, &m.UptimePct, &m.CreatedAt)
		if err != nil {
			return errJSON(c, 500, "insert failed")
		}
		return c.Status(201).JSON(m)
	}
}

func updateMonitorHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, auth.UserID(c)) {
			return errJSON(c, 404, "site not found")
		}
		var in struct {
			Enabled         *bool  `json:"enabled"`
			IntervalSeconds *int   `json:"interval_seconds"`
			ExpectedStatus  *int   `json:"expected_status"`
			URL             string `json:"url"`
		}
		if err := c.BodyParser(&in); err != nil {
			return errJSON(c, 400, "bad json")
		}
		cur := model.Monitor{}
		if err := db.QueryRow(c.Context(), `
			SELECT interval_seconds, expected_status FROM monitors WHERE id = $1 AND site_id = $2`,
			c.Params("mid"), siteID).Scan(&cur.IntervalSeconds, &cur.ExpectedStatus); err != nil {
			return errJSON(c, 404, "monitor not found")
		}
		interval := cur.IntervalSeconds
		if in.IntervalSeconds != nil && *in.IntervalSeconds >= 30 {
			interval = *in.IntervalSeconds
		}
		status := cur.ExpectedStatus
		if in.ExpectedStatus != nil && *in.ExpectedStatus > 0 {
			status = *in.ExpectedStatus
		}
		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		if _, err := db.Exec(c.Context(), `
			UPDATE monitors SET enabled = $1, interval_seconds = $2, expected_status = $3,
			       url = CASE WHEN $4 = '' THEN url ELSE $4 END
			WHERE id = $5 AND site_id = $6`,
			enabled, interval, status, in.URL, c.Params("mid"), siteID); err != nil {
			return errJSON(c, 500, "update failed")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func deleteMonitorHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !isSiteOwner(c, db, siteID, auth.UserID(c)) {
			return errJSON(c, 404, "site not found")
		}
		tag, err := db.Exec(c.Context(), `
			DELETE FROM monitors WHERE id = $1 AND site_id = $2`, c.Params("mid"), siteID)
		if err != nil {
			return errJSON(c, 500, "delete failed")
		}
		if tag.RowsAffected() == 0 {
			return errJSON(c, 404, "monitor not found")
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func monitorChecksHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		limit := c.QueryInt("limit", 24)
		rows, err := db.Query(c.Context(), `
			SELECT status_code, ok, latency_ms, checked_at
			FROM monitor_checks WHERE monitor_id = $1 ORDER BY checked_at DESC LIMIT $2`,
			c.Params("mid"), limit)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var out []model.MonitorCheck
		for rows.Next() {
			var mc model.MonitorCheck
			if err := rows.Scan(&mc.StatusCode, &mc.OK, &mc.LatencyMs, &mc.CheckedAt); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, mc)
		}
		if out == nil {
			out = []model.MonitorCheck{}
		}
		return c.JSON(out)
	}
}

func monitorLoop(ctx context.Context, db *pgxpool.Pool) {
	client := &http.Client{Timeout: 5 * time.Second}
	tick := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			runMonitors(ctx, db, client)
		}
	}
}

func runMonitors(ctx context.Context, db *pgxpool.Pool, client *http.Client) {
	type mrow struct {
		ID   string
		URL  string
		Code int
	}
	rows, err := db.Query(ctx, `
		SELECT id, url, expected_status FROM monitors
		WHERE enabled AND (last_check_at IS NULL OR last_check_at + interval_seconds * interval '1 second' <= now())`)
	if err != nil {
		return
	}
	var due []mrow
	for rows.Next() {
		var m mrow
		if err := rows.Scan(&m.ID, &m.URL, &m.Code); err == nil {
			due = append(due, m)
		}
	}
	rows.Close()
	for _, m := range due {
		start := time.Now()
		resp, err := client.Get(m.URL)
		latency := int(time.Since(start).Milliseconds())
		status := 0
		ok := false
		if err == nil {
			status = resp.StatusCode
			resp.Body.Close()
			ok = status == m.Code
		}
		db.Exec(ctx, `
			INSERT INTO monitor_checks (monitor_id, status_code, ok, latency_ms) VALUES ($1, $2, $3, $4)`,
			m.ID, status, ok, latency)
		db.Exec(ctx, `
			UPDATE monitors SET last_status = $1, last_ok = $2, last_check_at = now(),
			       uptime_pct = (SELECT round(avg(ok::int) * 100, 2) FROM monitor_checks
			                     WHERE monitor_id = $3 AND checked_at > now() - interval '30 days')
			WHERE id = $3`, status, ok, m.ID)
	}
}

func authUserID2(c *fiber.Ctx) string {
	if v, ok := c.Locals("uid").(string); ok {
		return v
	}
	return ""
}
