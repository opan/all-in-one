DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS email_verifications;
ALTER TABLE sessions DROP COLUMN password_change_required;
ALTER TABLE users DROP COLUMN email_prompted_at;
ALTER TABLE users DROP COLUMN email_verified_at;
