-- users.name should never have been UNIQUE (it's a display field with no
-- lookup-by-name anywhere); users.email needs to stay UNIQUE but allow NULL
-- so self-service sign-up (email optional) doesn't collide on '' for every
-- account after the first.
--
-- The name column's UNIQUE constraint was declared inline (no explicit
-- name) back in 01_create_init_table, so its system-generated name is
-- looked up dynamically here rather than assumed (e.g. "users_name_key")
-- to stay correct regardless of Postgres's naming convention or any prior
-- manual renaming.
DO $$
DECLARE
    name_unique_constraint text;
BEGIN
    SELECT con.conname INTO name_unique_constraint
    FROM pg_constraint con
    JOIN pg_class rel ON rel.oid = con.conrelid
    WHERE rel.relname = 'users'
      AND con.contype = 'u'
      AND con.conkey = (
          SELECT array_agg(att.attnum)
          FROM pg_attribute att
          WHERE att.attrelid = rel.oid AND att.attname = 'name'
      );

    IF name_unique_constraint IS NOT NULL THEN
        EXECUTE format('ALTER TABLE users DROP CONSTRAINT %I', name_unique_constraint);
    END IF;
END $$;

UPDATE users SET name = NULL WHERE name = '';

ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
UPDATE users SET email = NULL WHERE email = '';
