-- Newznab category id per release, assigned at build time by the catalog
-- plugin's Categorize heuristic. Default 8010 = Other/Misc (used when the
-- catalog capability isn't present). Indexed for the Newznab cat= filter.
ALTER TABLE nzbs ADD COLUMN IF NOT EXISTS category_id INT NOT NULL DEFAULT 8010;
CREATE INDEX IF NOT EXISTS nzbs_category ON nzbs (category_id);
