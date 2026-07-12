-- Plugin settings editable from the host's /admin/settings page. Keys mirror
-- the config.yml knobs; a row here overrides the config default at job run
-- time (config stays the boot-time default / documentation of intent).
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
