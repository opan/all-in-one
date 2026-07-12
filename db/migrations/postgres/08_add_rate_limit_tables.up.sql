CREATE TABLE IF NOT EXISTS rate_limit_rules (
    target_key   TEXT PRIMARY KEY,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    limit_count  INTEGER NOT NULL,
    window_value INTEGER NOT NULL,
    window_unit  TEXT    NOT NULL,
    updated_at   TIMESTAMP NOT NULL,
    updated_by   TEXT
);

CREATE TABLE IF NOT EXISTS rate_limit_counters (
    target_key TEXT NOT NULL,
    bucket_key TEXT NOT NULL,
    day        TEXT NOT NULL,
    count      INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (target_key, bucket_key, day)
);

CREATE INDEX idx_rate_limit_counters_day ON rate_limit_counters(day);
