-- pointstore plugin schema. Applied by loon into the "pointstore" schema with
-- search_path scoped, so unqualified names resolve here. Idempotent.

-- One equipped flair per user (buying a new one replaces the old).
CREATE TABLE IF NOT EXISTS user_flair (
    user_id   BIGINT PRIMARY KEY,
    flair_id  TEXT        NOT NULL,
    bought_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
