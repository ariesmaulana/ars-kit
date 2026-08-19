-- +goose Up
-- Soft-deactivate accounts: an inactive user cannot log in. Existing rows
-- default to active (TRUE) so no account is locked out by the migration.
ALTER TABLE users ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE users DROP COLUMN is_active;
