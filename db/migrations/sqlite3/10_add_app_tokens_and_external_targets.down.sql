ALTER TABLE rate_limit_rules DROP COLUMN kind;
ALTER TABLE rate_limit_rules DROP COLUMN scope;
ALTER TABLE rate_limit_rules DROP COLUMN description;
ALTER TABLE rate_limit_rules DROP COLUMN name;
ALTER TABLE rate_limit_rules DROP COLUMN is_external;
ALTER TABLE rate_limit_rules DROP COLUMN app;

DROP INDEX IF EXISTS idx_app_tokens_app;
DROP INDEX IF EXISTS idx_app_tokens_hash;
DROP TABLE IF EXISTS app_tokens;
