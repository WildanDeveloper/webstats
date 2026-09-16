-- 013_reliability.sql
-- C1: heartbeat / dead man's switch for cron jobs, workers and backups.
-- A beat is a GET or POST to /api/ping/<ping_key>; when no beat arrives
-- within period_seconds + grace_seconds the heartbeat goes 'late'.
CREATE TABLE IF NOT EXISTS heartbeats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL DEFAULT '',
    period_seconds INT NOT NULL DEFAULT 3600,
    grace_seconds INT NOT NULL DEFAULT 600,
    ping_key TEXT NOT NULL UNIQUE,
    last_ping_at TIMESTAMPTZ,
    last_alert_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'new',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_heartbeats_site ON heartbeats(site_id);

-- C4: maintenance windows suppress site_down/site_up and heartbeat alerts.
-- days_csv is a comma-separated list of lowercase 3-letter day names;
-- start/end are minutes from midnight UTC, end <= start wraps midnight.
CREATE TABLE IF NOT EXISTS maintenance_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    days_csv TEXT NOT NULL DEFAULT 'mon,tue,wed,thu,fri,sat,sun',
    start_minute INT NOT NULL DEFAULT 0,
    end_minute INT NOT NULL DEFAULT 1439,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_maintenance_site ON maintenance_windows(site_id);

-- C6: incident timeline. One row per outage window, opened by the alert
-- loop on an up->down transition and closed on recovery.
CREATE TABLE IF NOT EXISTS incidents (
    id BIGSERIAL PRIMARY KEY,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    monitor_id UUID REFERENCES monitors(id) ON DELETE CASCADE,
    kind TEXT NOT NULL DEFAULT 'site',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    reason TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_incidents_site ON incidents(site_id, started_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_incidents_open_site ON incidents(site_id) WHERE resolved_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_incidents_open_monitor ON incidents(monitor_id) WHERE resolved_at IS NULL AND monitor_id IS NOT NULL;
