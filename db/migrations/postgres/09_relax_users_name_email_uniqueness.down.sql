-- Reverts to NOT NULL UNIQUE email / UNIQUE name. Will fail if the data no
-- longer fits those constraints (e.g. two accounts with no email/name) --
-- that failure is expected, not a bug: it means real data now depends on
-- the relaxed schema and can't be losslessly reversed.
UPDATE users SET email = '' WHERE email IS NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

UPDATE users SET name = '' WHERE name IS NULL;
ALTER TABLE users ADD CONSTRAINT users_name_key UNIQUE (name);
