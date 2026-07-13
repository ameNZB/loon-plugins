-- Scraped metadata store. catalog_entry holds one row per external metadata
-- record (an AniDB anime, a ThePornDB scene); release_cover links an indexer
-- release id to a cover URL for the release page. The scraper's match job
-- writes both via the catalog plugin's CatalogSink + CatalogCovers.
CREATE TABLE IF NOT EXISTS catalog_entry (
    id            BIGSERIAL PRIMARY KEY,
    kind          TEXT        NOT NULL,
    ext_namespace TEXT        NOT NULL DEFAULT '',
    ext_id        TEXT        NOT NULL DEFAULT '',
    title         TEXT        NOT NULL,
    norm_title    TEXT        NOT NULL DEFAULT '',
    cover_url     TEXT        NOT NULL DEFAULT '',
    year          INT         NOT NULL DEFAULT 0,
    fields        JSONB,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, ext_namespace, ext_id)
);
CREATE INDEX IF NOT EXISTS catalog_entry_norm ON catalog_entry (norm_title);

CREATE TABLE IF NOT EXISTS release_cover (
    release_id BIGINT PRIMARY KEY,
    cover_url  TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
