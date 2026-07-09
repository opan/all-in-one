DROP INDEX IF EXISTS idx_users_group_id;
DROP INDEX IF EXISTS idx_user_feature_overrides_feature_id;
DROP INDEX IF EXISTS idx_group_features_feature_id;

ALTER TABLE users DROP COLUMN group_id;

DROP TABLE IF EXISTS user_feature_overrides;
DROP TABLE IF EXISTS group_features;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS features;
