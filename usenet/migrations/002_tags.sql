-- Quality tags parsed from release titles (resolution/source/codec/audio/lang).
-- Set at build time; the NZB Tag Fill job retrofits rows when the parser changes.
ALTER TABLE nzbs ADD COLUMN IF NOT EXISTS resolution  TEXT NOT NULL DEFAULT '';
ALTER TABLE nzbs ADD COLUMN IF NOT EXISTS source      TEXT NOT NULL DEFAULT '';
ALTER TABLE nzbs ADD COLUMN IF NOT EXISTS video_codec TEXT NOT NULL DEFAULT '';
ALTER TABLE nzbs ADD COLUMN IF NOT EXISTS audio       TEXT NOT NULL DEFAULT '';
ALTER TABLE nzbs ADD COLUMN IF NOT EXISTS language    TEXT NOT NULL DEFAULT '';
