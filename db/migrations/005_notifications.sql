CREATE TABLE IF NOT EXISTS notif_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    kind VARCHAR(30) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    from_email VARCHAR(254) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notif_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    event VARCHAR(30) NOT NULL,
    channel VARCHAR(10) NOT NULL,
    provider_id UUID REFERENCES notif_providers(id) ON DELETE CASCADE,
    target VARCHAR(500) NOT NULL DEFAULT '',
    params JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notif_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_id UUID REFERENCES notif_rules(id) ON DELETE SET NULL,
    site_id UUID,
    event VARCHAR(30),
    channel VARCHAR(10),
    status VARCHAR(10) NOT NULL,
    detail TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notif_rules_site ON notif_rules (site_id);
CREATE INDEX IF NOT EXISTS idx_notif_logs_user ON notif_logs (user_id, created_at DESC);