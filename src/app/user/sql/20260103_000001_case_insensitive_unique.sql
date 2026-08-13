-- +goose Up
-- Case-insensitive uniqueness for username and email (exploration gap M8).
--
-- PostgreSQL's default UNIQUE constraint compares bytes, so "Foo" and "foo"
-- (or "Foo@Bar.com" and "foo@bar.com") could coexist as two accounts. Replace
-- the column-level UNIQUE constraints with unique indexes over LOWER(...):
-- the database then rejects any two rows that differ only by case, and the
-- app layer normalizes on write (lowercased email, LOWER() lookups) so the
-- stored data and the lookups stay consistent.
--
-- NOTE: this fails if the table already contains rows that differ only by
-- case; on a fresh database (or after deduping) it applies cleanly.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_idx ON users (LOWER(username));
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_idx ON users (LOWER(email));

-- +goose Down
DROP INDEX IF EXISTS users_username_lower_idx;
DROP INDEX IF EXISTS users_email_lower_idx;
-- Restoring the original byte-wise unique constraints fails if case-variant
-- duplicates were created while the LOWER() indexes were in place (they
-- cannot be), so this is safe after the Up migration ran cleanly.
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
