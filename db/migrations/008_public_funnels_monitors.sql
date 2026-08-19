ALTER TABLE sites ADD COLUMN IF NOT EXISTS public_token TEXT;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS public_enabled BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS funnel_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    position INT NOT NULL,
    label TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_funnel_steps_site ON funnel_steps(site_id, position);

CREATE TABLE IF NOT EXISTS monitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    interval_seconds INT NOT NULL DEFAULT 60,
    expected_status INT NOT NULL DEFAULT 200,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_status INT,
    last_ok BOOLEAN,
    last_check_at TIMESTAMPTZ,
    uptime_pct REAL NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_monitors_site ON monitors(site_id);

CREATE TABLE IF NOT EXISTS monitor_checks (
    id BIGSERIAL PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status_code INT,
    ok BOOLEAN NOT NULL,
    latency_ms INT NOT NULL DEFAULT 0,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_monitor_checks_monitor ON monitor_checks(monitor_id, checked_at DESC);