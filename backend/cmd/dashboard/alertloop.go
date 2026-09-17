package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webstats/backend/internal/notify"
)

func ensureAlertState(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS site_alert_state (
			site_id UUID PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
			check_id BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS dashboard_alert_events (
			id BIGSERIAL PRIMARY KEY,
			site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
			event TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	return err
}

func alertLoop(ctx context.Context, pool *pgxpool.Pool) {
	run := func() {
		if err := ensureAlertState(ctx, pool); err != nil {
			log.Printf("alert state initialization failed: %v", err)
			return
		}
		rows, err := pool.Query(ctx, `SELECT id FROM sites WHERE domain <> ''`)
		if err != nil {
			log.Printf("alert site selection failed: %v", err)
			return
		}
		var sites []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return
			}
			sites = append(sites, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			log.Printf("alert site selection failed: %v", err)
			return
		}
		for _, id := range sites {
			if err := processSiteChecks(ctx, pool, id); err != nil {
				log.Printf("alert check processing failed: %v", err)
			}
		}
		drainAlertEvents(ctx, pool)
		checkSpikes(ctx, pool)
		checkVitals(ctx, pool)
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

func siteTransition(previous, current string) string {
	if previous == current || (previous == "" && current == "up") {
		return ""
	}
	if current == "down" {
		return "site_down"
	}
	if current == "up" {
		return "site_up"
	}
	return ""
}

func processSiteChecks(ctx context.Context, pool *pgxpool.Pool, siteID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO site_alert_state (site_id) VALUES ($1) ON CONFLICT DO NOTHING`, siteID); err != nil {
		return err
	}
	var cursor int64
	var previous string
	if err := tx.QueryRow(ctx, `SELECT check_id, status FROM site_alert_state WHERE site_id = $1 FOR UPDATE`, siteID).Scan(&cursor, &previous); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id, status, checked_at FROM site_checks WHERE site_id = $1 AND id > $2 ORDER BY id LIMIT 1000`, siteID, cursor)
	if err != nil {
		return err
	}
	type check struct {
		id     int64
		status string
		at     time.Time
	}
	var checks []check
	for rows.Next() {
		var c check
		if err := rows.Scan(&c.id, &c.status, &c.at); err != nil {
			rows.Close()
			return err
		}
		checks = append(checks, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, c := range checks {
		event := siteTransition(previous, c.status)
		if event != "" {
			if err := trackSiteIncidentTx(ctx, tx, siteID, event, c.at); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO dashboard_alert_events (site_id, event, created_at) VALUES ($1,$2,$3)`, siteID, event, c.at); err != nil {
				return err
			}
		}
		cursor, previous = c.id, c.status
	}
	if _, err := tx.Exec(ctx, `UPDATE site_alert_state SET check_id = $2, status = $3 WHERE site_id = $1`, siteID, cursor, previous); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func drainAlertEvents(ctx context.Context, pool *pgxpool.Pool) {
	for i := 0; i < 1000; i++ {
		if !deliverNextAlertEvent(ctx, pool) {
			return
		}
	}
}

func deliverNextAlertEvent(ctx context.Context, pool *pgxpool.Pool) bool {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)
	var id int64
	var siteID, event, name string
	var at time.Time
	err = tx.QueryRow(ctx, `SELECT id, site_id, event, name, created_at FROM dashboard_alert_events ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &siteID, &event, &name, &at)
	if err != nil {
		return false
	}
	if !inMaintenance(ctx, pool, siteID, at) {
		if err := fireAlertEvent(ctx, pool, siteID, event, name, at); err != nil {
			log.Printf("alert event delivery failed: %v", err)
			return false
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM dashboard_alert_events WHERE id = $1`, id); err != nil {
		return false
	}
	return tx.Commit(ctx) == nil
}

func fireAlertEvent(ctx context.Context, pool *pgxpool.Pool, siteID, event, name string, at time.Time) error {
	rows, err := pool.Query(ctx, `
		SELECT r.user_id, r.id, r.channel, r.target, r.provider_id, r.params, s.name, s.domain
		FROM notif_rules r JOIN sites s ON s.id = r.site_id
		WHERE r.site_id = $1 AND r.event = $2 AND r.enabled`, siteID, event)
	if err != nil {
		return err
	}
	type rule struct {
		userID, id, channel, target, siteName, domain string
		providerID                                    *string
		params                                        map[string]any
	}
	var rules []rule
	for rows.Next() {
		var r rule
		if err := rows.Scan(&r.userID, &r.id, &r.channel, &r.target, &r.providerID, &r.params, &r.siteName, &r.domain); err != nil {
			rows.Close()
			return err
		}
		rules = append(rules, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range rules {
		payload := notify.AlertPayload{Event: event, SiteID: siteID, SiteName: r.siteName, Domain: r.domain, Name: name, Status: event, Time: at.Format(time.RFC3339)}
		if _, err := deliverRule(ctx, pool, r.userID, r.channel, r.target, r.providerID, r.params, payload, false); err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, r.id); err != nil {
			return err
		}
	}
	return nil
}

func trackSiteIncidentTx(ctx context.Context, tx pgx.Tx, siteID, event string, at time.Time) error {
	if event == "site_down" {
		_, err := tx.Exec(ctx, `INSERT INTO incidents (site_id, kind, started_at) VALUES ($1, 'site', $2)
			ON CONFLICT (site_id) WHERE resolved_at IS NULL AND monitor_id IS NULL DO NOTHING`, siteID, at)
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE incidents SET resolved_at = $2 WHERE site_id = $1 AND resolved_at IS NULL AND monitor_id IS NULL`, siteID, at)
	return err
}

func trackSiteIncident(ctx context.Context, pool *pgxpool.Pool, siteID, event string) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if err := trackSiteIncidentTx(ctx, tx, siteID, event, time.Now()); err != nil {
		log.Printf("incident update failed: %v", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("incident commit failed: %v", err)
	}
}

func checkVitals(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `
		SELECT r.user_id, r.id, r.site_id, r.channel, r.target, r.provider_id, r.params, s.name, s.domain
		FROM notif_rules r JOIN sites s ON s.id = r.site_id
		WHERE r.event = 'vitals_lcp' AND r.enabled
		  AND (r.last_sent_at IS NULL OR r.last_sent_at < now()
		    - CASE WHEN r.params->>'cooldown_min' ~ '^[0-9]{1,5}$'
		         THEN LEAST(10080, GREATEST(1, (r.params->>'cooldown_min')::int)) ELSE 720 END * interval '1 minute')`)
	if err != nil {
		log.Printf("vitals rule selection failed: %v", err)
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
		if validateRuleParams("vitals_lcp", params) != nil {
			continue
		}
		threshold := 2500.0
		if t, ok := params["threshold"].(float64); ok {
			threshold = t
		}
		var p75 float64
		var samples int64
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(percentile_cont(0.75) WITHIN GROUP (ORDER BY
				CASE WHEN jsonb_typeof(props->'value') = 'number' THEN (props->>'value')::float END), 0), count(*)
			FROM events
			WHERE site_id = $1 AND name = 'web_vitals' AND props->>'metric' = 'lcp'
			  AND jsonb_typeof(props->'value') = 'number'
			  AND created_at >= now() - interval '24 hours'`, siteID).Scan(&p75, &samples)
		if err != nil || samples < 20 {
			continue
		}
		if p75 > threshold {
			payload := notify.AlertPayload{
				Event: "vitals_lcp", SiteID: siteID, SiteName: siteName, Domain: domain,
				Status: "degraded", Metric: "lcp", Value: int64(p75), Threshold: int64(threshold), Time: time.Now().Format(time.RFC3339),
			}
			if _, err := deliverRule(ctx, pool, userID, channel, target, providerID, params, payload, false); err == nil {
				if _, err := pool.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, ruleID); err != nil {
					log.Printf("vitals cooldown update failed: %v", err)
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("vitals rule selection failed: %v", err)
	}
}

func checkSpikes(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `
		SELECT r.user_id, r.id, r.site_id, r.channel, r.target, r.provider_id, r.params, s.name, s.domain
		FROM notif_rules r JOIN sites s ON s.id = r.site_id
		WHERE r.event = 'traffic_spike' AND r.enabled
		  AND (r.last_sent_at IS NULL OR r.last_sent_at < now()
		    - CASE WHEN r.params->>'cooldown_min' ~ '^[0-9]{1,5}$'
		         THEN LEAST(10080, GREATEST(1, (r.params->>'cooldown_min')::int)) ELSE 30 END * interval '1 minute')`)
	if err != nil {
		log.Printf("spike rule selection failed: %v", err)
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
		if validateRuleParams("traffic_spike", params) != nil {
			continue
		}
		threshold := 3.0
		if t, ok := params["threshold"].(float64); ok {
			threshold = t
		}
		var cur int64
		var avg float64
		err := pool.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM pageviews WHERE site_id = $1 AND visited_at > now() - interval '1 hour' AND visited_at <= now()),
				(SELECT count(*) / 167.0 FROM pageviews
				 WHERE site_id = $1 AND visited_at > now() - interval '7 days'
				   AND visited_at <= now() - interval '1 hour')`, siteID).Scan(&cur, &avg)
		if err != nil {
			log.Printf("spike baseline query failed: %v", err)
			continue
		}
		if avg >= 1 && float64(cur) >= avg*threshold {
			payload := notify.AlertPayload{
				Event: "traffic_spike", SiteID: siteID, SiteName: siteName, Domain: domain,
				Status: "spike", Count: cur, Avg: int64(avg), Threshold: int64(threshold), Time: time.Now().Format(time.RFC3339),
			}
			if _, err := deliverRule(ctx, pool, userID, channel, target, providerID, params, payload, false); err == nil {
				if _, err := pool.Exec(ctx, `UPDATE notif_rules SET last_sent_at = now() WHERE id = $1`, ruleID); err != nil {
					log.Printf("spike cooldown update failed: %v", err)
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("spike rule selection failed: %v", err)
	}
}
