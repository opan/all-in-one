ALTER TABLE rate_limit_rules DROP COLUMN IF EXISTS kind;
ALTER TABLE rate_limit_rules DROP COLUMN IF EXISTS scope;
ALTER TABLE rate_limit_rules DROP COLUMN IF EXISTS description;
ALTER TABLE rate_limit_rules DROP COLUMN IF EXISTS name;
ALTER TABLE rate_limit_rules DROP COLUMN IF EXISTS is_external;
ALTER TABLE rate_limit_rules DROP COLUMN IF EXISTS app;

DROP INDEX IF EXISTS idx_app_tokens_app;
DROP INDEX IF EXISTS idx_app_tokens_hash;
DROP TABLE IF EXISTS app_tokens;
