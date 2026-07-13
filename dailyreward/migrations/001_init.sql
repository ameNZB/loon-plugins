-- dailyreward plugin schema. Applied by loon into the "dailyreward" schema
-- with search_path scoped, so unqualified names resolve here. Idempotent.

-- One row per user tracking their daily-claim streak.
CREATE TABLE IF NOT EXISTS daily_rewards (
    user_id      BIGINT PRIMARY KEY,
    last_claim   DATE,
    streak       INT NOT NULL DEFAULT 0,
    longest      INT NOT NULL DEFAULT 0,
    total_claims INT NOT NULL DEFAULT 0
);
