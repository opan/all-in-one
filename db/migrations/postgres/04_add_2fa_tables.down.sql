DROP INDEX IF EXISTS idx_totp_challenges_expires_at;
DROP INDEX IF EXISTS idx_totp_challenges_user_id;
DROP TABLE IF EXISTS totp_challenges;

DROP INDEX IF EXISTS idx_recovery_codes_user_id;
DROP TABLE IF EXISTS recovery_codes;

ALTER TABLE users DROP COLUMN totp_verified_at;
ALTER TABLE users DROP COLUMN totp_secret_encrypted;
ALTER TABLE users DROP COLUMN totp_enabled;
