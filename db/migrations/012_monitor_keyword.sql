-- 012_monitor_keyword.sql
-- Content checks: a monitor may require a keyword to be present in (or
-- absent from) the response body. 'present' catches defacements and error
-- pages that still return HTTP 200; 'absent' catches "temporarily
-- unavailable" banners.
ALTER TABLE monitors ADD COLUMN IF NOT EXISTS keyword TEXT NOT NULL DEFAULT '';
ALTER TABLE monitors ADD COLUMN IF NOT EXISTS keyword_mode TEXT NOT NULL DEFAULT 'present';
