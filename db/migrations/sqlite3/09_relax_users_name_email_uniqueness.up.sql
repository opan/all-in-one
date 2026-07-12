-- users.name should never have been UNIQUE (it's a display field with no
-- lookup-by-name anywhere); users.email needs to stay UNIQUE but allow NULL
-- so self-service sign-up (email optional) doesn't collide on '' for every
-- account after the first. SQLite has no ALTER TABLE ... DROP CONSTRAINT /
-- ALTER COLUMN, so this rebuilds the table (standard SQLite pattern).
--
-- Full column set carried over: id/username/email/name/password_hash from
-- 01_create_init_table, totp_enabled/totp_secret_encrypted/totp_verified_at
-- from 04_add_2fa_tables, group_id from 06_add_rbac_tables, blocked from
-- 07_add_user_blocked.
PRAGMA foreign_keys=OFF;

CREATE TABLE users_new (
  id TEXT PRIMARY KEY UNIQUE,
  username TEXT NOT NULL UNIQUE,
  email TEXT UNIQUE,
  name TEXT,
  password_hash TEXT NOT NULL,
  last_login TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  totp_enabled INTEGER NOT NULL DEFAULT 0,
  totp_secret_encrypted TEXT,
  totp_verified_at TIMESTAMP,
  blocked INTEGER NOT NULL DEFAULT 0,
  group_id TEXT REFERENCES groups(id) ON DELETE SET NULL
);

INSERT INTO users_new (id, username, email, name, password_hash, last_login, created_at, updated_at,
                        totp_enabled, totp_secret_encrypted, totp_verified_at, blocked, group_id)
SELECT id, username, NULLIF(email, ''), NULLIF(name, ''), password_hash, last_login, created_at, updated_at,
       totp_enabled, totp_secret_encrypted, totp_verified_at, blocked, group_id
FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

CREATE INDEX idx_users_group_id ON users(group_id);

PRAGMA foreign_keys=ON;
