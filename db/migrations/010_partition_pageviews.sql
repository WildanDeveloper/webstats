-- Drop the never-read daily rollup table (queries always hit pageviews
-- directly) and convert pageviews to monthly RANGE partitions.
BEGIN;

DROP TABLE IF EXISTS site_daily;

-- Per-report unsubscribe tokens for scheduled email reports.
ALTER TABLE notif_reports
    ADD COLUMN IF NOT EXISTS unsub_token TEXT NOT NULL DEFAULT encode(gen_random_bytes(16), 'hex');
CREATE UNIQUE INDEX IF NOT EXISTS idx_notif_reports_unsub ON notif_reports (unsub_token);

ALTER TABLE pageviews RENAME TO pageviews_old;

CREATE TABLE pageviews (
    id            BIGSERIAL,
    site_id       UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    session_id    TEXT NOT NULL,
    path          TEXT NOT NULL DEFAULT '/',
    title         TEXT NOT NULL DEFAULT '',
    referrer      TEXT NOT NULL DEFAULT '',
    referrer_host TEXT NOT NULL DEFAULT '',
    ua            TEXT NOT NULL DEFAULT '',
    browser       TEXT NOT NULL DEFAULT '',
    os            TEXT NOT NULL DEFAULT '',
    device        TEXT NOT NULL DEFAULT '',
    country       TEXT NOT NULL DEFAULT '',
    screen        TEXT NOT NULL DEFAULT '',
    lang          TEXT NOT NULL DEFAULT '',
    ip_hash       TEXT NOT NULL DEFAULT '',
    ip            TEXT NOT NULL DEFAULT '',
    isp           TEXT NOT NULL DEFAULT '',
    region        TEXT NOT NULL DEFAULT '',
    city          TEXT NOT NULL DEFAULT '',
    lat           DOUBLE PRECISION NOT NULL DEFAULT 0,
    lon           DOUBLE PRECISION NOT NULL DEFAULT 0,
    visited_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    utm_source    VARCHAR(200) NOT NULL DEFAULT '',
    utm_medium    VARCHAR(200) NOT NULL DEFAULT '',
    utm_campaign  VARCHAR(200) NOT NULL DEFAULT '',
    utm_content   VARCHAR(200) NOT NULL DEFAULT '',
    utm_term      VARCHAR(200) NOT NULL DEFAULT '',
    -- On partitioned tables the PK must include the partition key.
    PRIMARY KEY (id, visited_at)
) PARTITION BY RANGE (visited_at);

-- Safety net: rows whose month partition does not exist (yet) still land here.
CREATE TABLE pageviews_default PARTITION OF pageviews DEFAULT;

INSERT INTO pageviews (
    id, site_id, session_id, path, title, referrer, referrer_host,
    ua, browser, os, device, country, screen, lang, ip_hash, ip,
    isp, region, city, lat, lon, visited_at,
    utm_source, utm_medium, utm_campaign, utm_content, utm_term
)
SELECT
    id, site_id, session_id, path, title, referrer, referrer_host,
    ua, browser, os, device, country, screen, lang, ip_hash, ip,
    isp, region, city, lat, lon, visited_at,
    utm_source, utm_medium, utm_campaign, utm_content, utm_term
FROM pageviews_old;
DROP TABLE pageviews_old;

CREATE INDEX IF NOT EXISTS idx_pv_site_time   ON pageviews(site_id, visited_at);
CREATE INDEX IF NOT EXISTS idx_pv_site_path   ON pageviews(site_id, path);
CREATE INDEX IF NOT EXISTS idx_pv_site_sess   ON pageviews(site_id, session_id);
CREATE INDEX IF NOT EXISTS idx_pv_site_ref    ON pageviews(site_id, referrer_host);
CREATE INDEX IF NOT EXISTS idx_pv_site_ua     ON pageviews(site_id, device, browser, os);
CREATE INDEX IF NOT EXISTS idx_pv_site_country ON pageviews(site_id, country);
CREATE INDEX IF NOT EXISTS idx_pv_site_ip     ON pageviews(site_id, ip, visited_at);
CREATE INDEX IF NOT EXISTS idx_pageviews_utm  ON pageviews(site_id, utm_source, utm_campaign, visited_at);

COMMIT;
