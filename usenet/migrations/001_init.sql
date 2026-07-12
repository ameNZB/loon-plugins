-- usenet plugin schema. Applied by loon into the "usenet" schema with
-- search_path scoped, so unqualified names resolve here. Idempotent.

-- The NNTP server (single row for the demo; the setup wizard edits it).
CREATE TABLE IF NOT EXISTS servers (
    id         SERIAL PRIMARY KEY,
    host       TEXT        NOT NULL,
    port       INT         NOT NULL DEFAULT 119,
    tls        BOOLEAN     NOT NULL DEFAULT FALSE,
    username   TEXT        NOT NULL DEFAULT '',
    password   TEXT        NOT NULL DEFAULT '',
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Newsgroups the indexer watches. active=false by default so a bulk import
-- from NNTP LIST is a pick-list, not "crawl everything".
CREATE TABLE IF NOT EXISTS newsgroups (
    name           TEXT PRIMARY KEY,
    active         BOOLEAN     NOT NULL DEFAULT FALSE,
    high_watermark BIGINT      NOT NULL DEFAULT 0,
    server_low     BIGINT      NOT NULL DEFAULT 0,
    server_high    BIGINT      NOT NULL DEFAULT 0,
    last_crawl     TIMESTAMPTZ,
    added_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Article overview staging (replaces prod's Redis). The crawler upserts here;
-- the builder drains complete (group, base_subject) sets; the prune job clears
-- stale rows that never complete.
CREATE TABLE IF NOT EXISTS articles (
    message_id   TEXT PRIMARY KEY,
    subject      TEXT        NOT NULL,
    base_subject TEXT        NOT NULL,
    poster       TEXT        NOT NULL DEFAULT '',
    bytes        BIGINT      NOT NULL DEFAULT 0,
    posted       TIMESTAMPTZ,
    group_name   TEXT        NOT NULL,
    part_num     INT         NOT NULL DEFAULT 1,
    total_parts  INT         NOT NULL DEFAULT 1,
    seg_total    INT         NOT NULL DEFAULT 0,
    file_num     INT         NOT NULL DEFAULT 0,
    total_files  INT         NOT NULL DEFAULT 0,
    file_parts   BOOLEAN     NOT NULL DEFAULT FALSE,
    added_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS articles_group_base ON articles (group_name, base_subject);
CREATE INDEX IF NOT EXISTS articles_added ON articles (added_at);

-- Assembled releases (the ~11 essential columns + the gzipped NZB XML).
CREATE TABLE IF NOT EXISTS nzbs (
    id             SERIAL PRIMARY KEY,
    title          TEXT        NOT NULL,
    filename       TEXT        NOT NULL,
    size           BIGINT      NOT NULL DEFAULT 0,
    status         TEXT        NOT NULL DEFAULT 'completed',
    group_name     TEXT        NOT NULL DEFAULT '',
    content_hash   TEXT        NOT NULL UNIQUE,
    posted_at      TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    nzb_data       BYTEA,
    nzb_data_bytes BIGINT      NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS nzbs_title ON nzbs (title);
CREATE INDEX IF NOT EXISTS nzbs_created ON nzbs (created_at);
