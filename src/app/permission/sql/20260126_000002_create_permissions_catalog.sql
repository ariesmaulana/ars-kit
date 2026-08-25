-- +goose Up
-- Catalog of permissions that may be granted. Source of truth for the strings
-- is each module's const.go; rows are inserted manually via SOP when a new
-- feature ships. GrantPermission rejects any permission not present here.
CREATE TABLE IF NOT EXISTS permissions (
    permission VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP DEFAULT NOW()
);

-- +goose Down
