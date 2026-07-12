-- Backfill / coverage tracking. The crawler is forward-only from high_watermark;
-- backfill walks a single back_watermark pointer downward toward server_low,
-- filling history. Because the plugin crawls serially (one connection, monotonic),
-- a single pointer is exact — no gap/range table needed (that's prod's concern,
-- where parallel workers leave holes).
--
-- Coverage bar over [server_low .. server_high]:
--   server_low  → back_watermark   = pending backfill (not yet fetched)
--   back_watermark → high_watermark = indexed (have)
--   high_watermark → server_high    = new (not yet fetched forward)

ALTER TABLE newsgroups ADD COLUMN IF NOT EXISTS back_watermark      BIGINT;
ALTER TABLE newsgroups ADD COLUMN IF NOT EXISTS back_watermark_date TIMESTAMPTZ;
ALTER TABLE newsgroups ADD COLUMN IF NOT EXISTS high_watermark_date TIMESTAMPTZ;
ALTER TABLE newsgroups ADD COLUMN IF NOT EXISTS backfill_done       BOOLEAN NOT NULL DEFAULT FALSE;

-- Existing groups predate backfill: assume caught-up (no pending history) so they
-- don't paint a misleading orange bar. A fresh crawl / ResetBackfill re-arms them.
UPDATE newsgroups SET back_watermark = high_watermark WHERE back_watermark IS NULL;
