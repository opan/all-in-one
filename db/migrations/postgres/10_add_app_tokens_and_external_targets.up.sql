CREATE TABLE IF NOT EXISTS app_tokens (
    id           TEXT PRIMARY KEY,
    app          TEXT NOT NULL,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    scope_prefix TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    created_by   TEXT,
    last_used_at TIMESTAMP,
    revoked_at   TIMESTAMP
);

CREATE UNIQUE INDEX idx_app_tokens_hash ON app_tokens(token_hash);
CREATE INDEX idx_app_tokens_app ON app_tokens(app);

ALTER TABLE rate_limit_rules ADD COLUMN app         TEXT NOT NULL DEFAULT 'all-in-one';
ALTER TABLE rate_limit_rules ADD COLUMN is_external BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE rate_limit_rules ADD COLUMN name        TEXT;
ALTER TABLE rate_limit_rules ADD COLUMN description TEXT;
ALTER TABLE rate_limit_rules ADD COLUMN scope       TEXT;
ALTER TABLE rate_limit_rules ADD COLUMN kind        TEXT;
