package main

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/model"
)

func publicSiteID(ctx context.Context, db *pgxpool.Pool, token string) (string, string, error) {
	var id, owner string
	err := db.QueryRow(ctx, `
		SELECT id, user_id FROM sites WHERE public_token = $1 AND public_enabled`, token).Scan(&id, &owner)
	return id, owner, err
}

func withPublicSite(db *pgxpool.Pool) func(*fiber.Ctx) (string, string, error) {
	return func(c *fiber.Ctx) (string, string, error) {
		return publicSiteID(c.Context(), db, c.Params("token"))
	}
}

func publicOverviewHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.Overview(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicTimeseriesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.Timeseries(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicTopHandler(db *pgxpool.Pool, column string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.Top(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), column, c.QueryInt("limit", 10),
			c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicWorldHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.World(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicCampaignsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.Campaigns(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicGoalSummariesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.GoalSummaries(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), filtersFromQuery(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicFunnelHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		paths, err := funnelPaths(c.Context(), db, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		out, err := analytics.Q.Funnel(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"), paths, filtersFromQuery(c))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		// Same shape as the authenticated /funnel/data endpoint so the
		// frontend can parse both identically.
		return c.JSON(fiber.Map{"steps": paths, "report": out})
	}
}

func publicEventDetailsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.EventDetails(c.Context(), db, owner, siteID,
			c.Query("period", "7d"), c.Query("from"), c.Query("to"))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicEventOccurrencesHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		out, err := analytics.Q.EventOccurrences(c.Context(), db, owner, siteID,
			c.Params("name"), c.Query("period", "7d"), c.Query("from"), c.Query("to"), c.QueryInt("limit", 50))
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(out)
	}
}

func publicInsightsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		period := c.Query("period", "7d")
		ts, err := analytics.Q.Timeseries(c.Context(), db, owner, siteID, period, "", "", analytics.Filters{})
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		top, err := analytics.Q.Top(c.Context(), db, owner, siteID, period, "path", 5, "", "", analytics.Filters{})
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		src, err := analytics.Q.Top(c.Context(), db, owner, siteID, period, "referrer", 5, "", "", analytics.Filters{})
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		return c.JSON(buildInsights(ts, top, src))
	}
}

func publicStatusHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID, owner, err := withPublicSite(db)(c)
		if err != nil {
			return errJSON(c, 404, "dashboard not found")
		}
		_ = owner
		var info model.PublicSiteInfo
		if err := db.QueryRow(c.Context(), `
			SELECT name, domain, color FROM sites WHERE id = $1`, siteID).Scan(&info.Name, &info.Domain, &info.Color); err != nil {
			return errJSON(c, 404, "site not found")
		}
		rows, err := db.Query(c.Context(), `
			SELECT id, url, interval_seconds, expected_status, enabled,
			       last_status, last_ok, last_check_at, uptime_pct
			FROM monitors WHERE site_id = $1 ORDER BY created_at`, siteID)
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		defer rows.Close()
		var out []model.Monitor
		ids := []string{}
		for rows.Next() {
			var m model.Monitor
			if err := rows.Scan(&m.ID, &m.URL, &m.IntervalSeconds, &m.ExpectedStatus, &m.Enabled,
				&m.LastStatus, &m.LastOK, &m.LastCheckAt, &m.UptimePct); err != nil {
				return errJSON(c, 500, "scan failed")
			}
			out = append(out, m)
			ids = append(ids, m.ID)
		}
		if out == nil {
			out = []model.Monitor{}
		}
		if len(ids) > 0 {
			attachMonitorDays(c.Context(), db, out, ids)
		}
		return c.JSON(fiber.Map{"site": info, "monitors": out})
	}
}

// attachMonitorDays buckets each monitor's checks per day for the last 90
// days so the public status page can draw uptime bars.
func attachMonitorDays(ctx context.Context, db *pgxpool.Pool, monitors []model.Monitor, ids []string) {
	rows, err := db.Query(ctx, `
		SELECT monitor_id, checked_at::date::text, count(*), count(*) FILTER (WHERE ok)
		FROM monitor_checks
		WHERE monitor_id = ANY($1::uuid[]) AND checked_at >= now() - interval '90 days'
		GROUP BY 1, 2 ORDER BY 2`, ids)
	if err != nil {
		return
	}
	defer rows.Close()
	byID := map[string]map[string]model.MonitorDay{}
	for rows.Next() {
		var mid, date string
		var total, up int64
		if err := rows.Scan(&mid, &date, &total, &up); err != nil {
			continue
		}
		if byID[mid] == nil {
			byID[mid] = map[string]model.MonitorDay{}
		}
		byID[mid][date] = model.MonitorDay{Date: date, Up: up, Total: total}
	}
	rows.Close()
	for i := range monitors {
		days := byID[monitors[i].ID]
		if len(days) == 0 {
			continue
		}
		series := make([]model.MonitorDay, 0, len(days))
		for _, d := range days {
			series = append(series, d)
		}
		monitors[i].Days = series
	}
}
