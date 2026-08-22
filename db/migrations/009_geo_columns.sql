-- Geo enrichment columns used by ingestion and the visitor views.
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS ip     TEXT NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS isp    TEXT NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS region TEXT NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS city   TEXT NOT NULL DEFAULT '';
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS lat    DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE pageviews ADD COLUMN IF NOT EXISTS lon    DOUBLE PRECISION NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_pv_site_ip ON pageviews (site_id, ip, visited_at);

-- Public tokens must be unique across sites so a public link can never
-- resolve to more than one site. Empty tokens (public sharing disabled)
-- are excluded from the constraint.
CREATE UNIQUE INDEX IF NOT EXISTS idx_sites_public_token
    ON sites (public_token)
    WHERE public_token IS NOT NULL AND public_token <> '';
