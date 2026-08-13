-- +goose Up
-- Per-account login throttling / lockout.
--
-- failed_login_attempts counts consecutive failed logins inside the counting
-- window. last_failed_login_at anchors the window: a failure that happens
-- after the window has elapsed resets the counter. locked_until is non-NULL
-- while the account is locked (set when the counter reaches the threshold).
-- Existing rows start unlocked with a zero counter so no account is locked by
-- the migration.
ALTER TABLE users
    ADD COLUMN failed_login_attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN last_failed_login_at TIMESTAMP NULL,
    ADD COLUMN locked_until TIMESTAMP NULL;

-- +goose Down
ALTER TABLE users
    DROP COLUMN IF EXISTS failed_login_attempts,
    DROP COLUMN IF EXISTS last_failed_login_at,
    DROP COLUMN IF EXISTS locked_until;
