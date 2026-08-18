ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS utm_source VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS utm_medium VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS utm_campaign VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS utm_content VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS utm_term VARCHAR(200) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_pageviews_utm ON pageviews (site_id, utm_source, utm_campaign, visited_at);

CREATE TABLE IF NOT EXISTS goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    path VARCHAR(500) NOT NULL,
    match_type VARCHAR(10) NOT NULL DEFAULT 'contains',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notif_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES notif_providers(id) ON DELETE CASCADE,
    recipient VARCHAR(254) NOT NULL,
    frequency VARCHAR(10) NOT NULL DEFAULT 'weekly',
    day VARCHAR(10) NOT NULL DEFAULT 'monday',
    hour INT NOT NULL DEFAULT 8,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_goals_site ON goals (site_id);
CREATE INDEX IF NOT EXISTS idx_notif_reports_enabled ON notif_reports (enabled, frequency);