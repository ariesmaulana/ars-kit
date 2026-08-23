-- +goose Up
-- P0-2: Add account status so users can be disabled/suspended without soft-delete.
CREATE TYPE user_status AS ENUM ('active', 'disabled', 'suspended');

ALTER TABLE users
    ADD COLUMN status user_status NOT NULL DEFAULT 'active';

-- +goose Down
