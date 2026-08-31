-- +goose Up
-- P0-4: Track last successful login for traceability/audit.
ALTER TABLE users
    ADD COLUMN last_login_at TIMESTAMPTZ NULL;

-- +goose Down
