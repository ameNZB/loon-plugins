-- Catalog plugin schema. The taxonomy itself is static Go data; the only state
-- is which top-level categories an admin has turned OFF (default: all on). We
-- store the DISABLED set so a fresh install surfaces everything.
CREATE TABLE IF NOT EXISTS category_disabled (
    category_id INT PRIMARY KEY,
    disabled_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
