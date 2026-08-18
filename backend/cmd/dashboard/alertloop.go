package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/notify"
)

func alertLoop(ctx context.Context, pool *pgxpool.Pool) {
	run := func() {
		sites := map[string]string{}
		rows, err := pool.Query(ctx, `SELECT id, domain FROM sites WHERE domain <> ''`)
		if err != nil {
			return
		}
		for rows.Next() {
			var id, domain string
			if rows.Scan(&id, &domain) == nil {
				sites[id] = domain
			}
		}
		rows.Close()

		for id := range sites {
			var cur, prev string
			err := pool.QueryRow(ctx, `
				SELECT s1.status, COALESCE(s2.status, '')
				FROM (
					SELECT status, checked_at FROM site_checks WHERE site_id = $1
					ORDER BY checked_at DESC LIMIT 1
				) s1
				LEFT JOIN (
					SELECT status, checked_at FROM site_checks WHERE site_id = $1
					ORDER BY checked_at DESC OFFSET 1 LIMIT 1
				) s2 ON true`, id).Scan(&cur, &prev)
			if err != nil || prev == "" {
				continue
			}
			if cur != prev {
				event := "site_down"
				if cur == "up" {
					event = "site_up"
				}
				fireSiteEvent(ctx, pool, id, event)
			}
		}

		checkSpikes(ctx, pool)
	}

	run()
	ticker := time.NewTicker(60 * time.Second)
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

func fireSiteEvent(ctx context.Context, pool *pgxpool.Pool, siteID, event string) {
	rows, err := pool.Query(ctx, `
		SELECT r.user_id, r.id, r.channel, r.target, r.provider_id, r.params, s.name, s.domain
		FROM notif_rules r JOIN sites s ON s.id = r.site_id
		WHERE r.site_id = $1 AND r.event = $2 AND r.enabled`, siteID, event)
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
			Status: event, Time: time.Now().Format(time.RFC3339),
		}
		_, _ = deliverRule(ctx, pool, userID, channel, target, providerID, params, payload, false)
		_, _ = pool.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, ruleID)
	}
}

func checkSpikes(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `
		SELECT r.user_id, r.id, r.site_id, r.channel, r.target, r.provider_id, r.params, s.name, s.domain
		FROM notif_rules r JOIN sites s ON s.id = r.site_id
		WHERE r.event = 'traffic_spike' AND r.enabled
		  AND (r.last_sent_at IS NULL OR r.last_sent_at < now() - (COALESCE(NULLIF(r.params->>'cooldown_min', ''), '30') || ' minutes')::interval)`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var userID, ruleID, siteID, channel, target string
		var providerID *string
		var params map[string]any
		var siteName, domain string
		if rows.Scan(&userID, &ruleID, &siteID, &channel, &target, &providerID, &params, &siteName, &domain) != nil {
			continue
		}
		threshold := 3
		if t, ok := params["threshold"].(float64); ok && t >= 1 {
			threshold = int(t)
		}
		var cur, avg int64
		_ = pool.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM pageviews WHERE site_id = $1 AND created_at > now() - interval '1 hour'),
				(SELECT count(*) / 168.0 FROM pageviews
				 WHERE site_id = $1 AND created_at > now() - interval '7 days'
				   AND created_at <= now() - interval '1 hour')`, siteID).Scan(&cur, &avg)
		if avg >= 1 && cur >= avg*int64(threshold) {
			payload := notify.AlertPayload{
				Event: "traffic_spike", SiteID: siteID, SiteName: siteName, Domain: domain,
				Status: "spike", Count: cur, Avg: avg, Threshold: int64(threshold),
				Time: time.Now().Format(time.RFC3339),
			}
			_, _ = deliverRule(ctx, pool, userID, channel, target, providerID, params, payload, false)
			_, _ = pool.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, ruleID)
		}
	}
}
