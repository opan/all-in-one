-- Reverts to NOT NULL UNIQUE email / UNIQUE name. Will fail if the data no
-- longer fits those constraints (e.g. two accounts with no email/name) --
-- that failure is expected, not a bug: it means real data now depends on
-- the relaxed schema and can't be losslessly reversed.
PRAGMA foreign_keys=OFF;

CREATE TABLE users_old (
  id TEXT PRIMARY KEY UNIQUE,
  username TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL UNIQUE,
  name TEXT UNIQUE,
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

INSERT INTO users_old (id, username, email, name, password_hash, last_login, created_at, updated_at,
                        totp_enabled, totp_secret_encrypted, totp_verified_at, blocked, group_id)
SELECT id, username, COALESCE(email, ''), COALESCE(name, ''), password_hash, last_login, created_at, updated_at,
       totp_enabled, totp_secret_encrypted, totp_verified_at, blocked, group_id
FROM users;

DROP TABLE users;
ALTER TABLE users_old RENAME TO users;

CREATE INDEX idx_users_group_id ON users(group_id);

PRAGMA foreign_keys=ON;
