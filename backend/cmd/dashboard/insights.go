package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/analytics"
	"github.com/webstats/backend/internal/auth"
	"github.com/webstats/backend/internal/model"
)

func insightsHandler(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		siteID := c.Params("id")
		if !siteAccessByUser(c, db, siteID) {
			return errJSON(c, 404, "site not found")
		}
		period := c.Query("period", "7d")
		ts, err := analytics.Q.Timeseries(c.Context(), db, auth.UserID(c), siteID, period, "", "", analytics.Filters{})
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		top, err := analytics.Q.Top(c.Context(), db, auth.UserID(c), siteID, period, "path", 5, "", "", analytics.Filters{})
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		src, err := analytics.Q.Top(c.Context(), db, auth.UserID(c), siteID, period, "referrer", 5, "", "", analytics.Filters{})
		if err != nil {
			return errJSON(c, 500, "query failed")
		}
		out := buildInsights(ts, top, src)
		return c.JSON(out)
	}
}

func buildInsights(ts []model.TimePoint, top, src []model.Row) model.Insights {
	var cur, prev int64
	bestDay := model.TimePoint{}
	for _, p := range ts {
		cur += p.Pageviews
		prev += p.PrevPageviews
		if p.Pageviews > bestDay.Pageviews {
			bestDay = p
		}
	}
	var pct float64
	if prev > 0 {
		pct = (float64(cur) - float64(prev)) / float64(prev) * 100
	}
	var b strings.Builder
	fmt.Fprintf(&b, "This period saw %d pageviews", cur)
	if prev > 0 {
		if pct >= 0 {
			fmt.Fprintf(&b, ", up %.0f%% from the previous period", pct)
		} else {
			fmt.Fprintf(&b, ", down %.0f%% from the previous period", math.Abs(pct))
		}
	} else {
		b.WriteString(", no previous period data to compare")
	}
	if bestDay.Pageviews > 0 {
		fmt.Fprintf(&b, ". Your best day was %s with %d pageviews", bestDay.Date, bestDay.Pageviews)
	}
	out := model.Insights{Summary: b.String()}
	out.Highlights = []model.InsightHighlight{
		{Kind: "traffic", Title: "Traffic vs previous period", Text: fmt.Sprintf("%+d pageviews (%.0f%%)", cur-prev, pct), DeltaPct: pct},
	}
	if len(top) > 0 {
		out.Highlights = append(out.Highlights, model.InsightHighlight{
			Kind: "page", Title: "Top page", Text: fmt.Sprintf("%s with %d pageviews", top[0].Key, top[0].Value),
		})
	}
	if len(src) > 0 && src[0].Key != "(direct)" && src[0].Key != "" {
		out.Highlights = append(out.Highlights, model.InsightHighlight{
			Kind: "referrer", Title: "Top referrer", Text: fmt.Sprintf("%s brought %d visitors", src[0].Key, src[0].Value),
		})
	}
	return out
}

func anomalyLoop(ctx context.Context, db *pgxpool.Pool) {
	tick := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			runAnomalyCheck(ctx, db)
		}
	}
}

func runAnomalyCheck(ctx context.Context, db *pgxpool.Pool) {
	type siteRow struct {
		ID   string
		User string
	}
	rows, err := db.Query(ctx, `SELECT id, user_id FROM sites`)
	if err != nil {
		return
	}
	var sites []siteRow
	for rows.Next() {
		var s siteRow
		if err := rows.Scan(&s.ID, &s.User); err == nil {
			sites = append(sites, s)
		}
	}
	rows.Close()
	for _, s := range sites {
		var cur, prev int64
		if err := db.QueryRow(ctx, `
			SELECT count(*) FROM pageviews
			WHERE site_id = $1 AND visited_at >= now() - interval '3 hours'`, s.ID).Scan(&cur); err != nil {
			continue
		}
		if err := db.QueryRow(ctx, `
			SELECT count(*) FROM pageviews
			WHERE site_id = $1 AND visited_at >= now() - interval '27 hours'
			  AND visited_at < now() - interval '24 hours'`, s.ID).Scan(&prev); err != nil {
			continue
		}
		if cur < 20 || prev < 20 {
			continue
		}
		pct := (float64(cur) - float64(prev)) / float64(prev) * 100
		if pct < 100 && pct > -50 {
			continue
		}
		var dup bool
		db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM notif_logs
				WHERE site_id = $1 AND event = 'anomaly' AND created_at > now() - interval '6 hours'
			)`, s.ID).Scan(&dup)
		if dup {
			continue
		}
		dir := "up"
		if pct < 0 {
			dir = "down"
		}
		db.Exec(ctx, `
			INSERT INTO notif_logs (user_id, site_id, event, channel, status, detail)
			VALUES ($1, $2, 'anomaly', 'inapp', 'new', $3)`,
			s.User, s.ID, fmt.Sprintf("Traffic %s %.0f%% in the last 3 hours compared to the same window yesterday", dir, math.Abs(pct)))
	}
}
