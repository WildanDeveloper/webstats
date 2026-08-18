


CREATE TABLE IF NOT EXISTS pageviews (
    id            BIGSERIAL PRIMARY KEY,
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
    visited_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pv_site_time ON pageviews(site_id, visited_at);
CREATE INDEX IF NOT EXISTS idx_pv_site_path  ON pageviews(site_id, path);
CREATE INDEX IF NOT EXISTS idx_pv_site_sess  ON pageviews(site_id, session_id);
CREATE INDEX IF NOT EXISTS idx_pv_site_ref   ON pageviews(site_id, referrer_host);
CREATE INDEX IF NOT EXISTS idx_pv_site_ua    ON pageviews(site_id, device, browser, os);
CREATE INDEX IF NOT EXISTS idx_pv_site_country ON pageviews(site_id, country);

CREATE TABLE IF NOT EXISTS events (
    id         BIGSERIAL PRIMARY KEY,
    site_id    UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    url        TEXT NOT NULL DEFAULT '',
    props      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_events_site_name ON events(site_id, name, created_at);

CREATE TABLE IF NOT EXISTS site_daily (
    site_id    UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    day        DATE NOT NULL,
    pageviews  BIGINT NOT NULL DEFAULT 0,
    visitors   BIGINT NOT NULL DEFAULT 0,
    sessions   BIGINT NOT NULL DEFAULT 0,
    bounces    BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (site_id, day)
);
CREATE INDEX IF NOT EXISTS idx_site_daily_day ON site_daily(day);
