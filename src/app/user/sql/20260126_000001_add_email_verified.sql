-- +goose Up
-- Email verification: NULL = unverified.
ALTER TABLE users
    ADD COLUMN email_verified_at TIMESTAMPTZ NULL;

-- Single-purpose one-time tokens (password reset, email verification).
-- Only the SHA-256 hash of the opaque token is stored, never the token.
CREATE TABLE IF NOT EXISTS email_tokens (
    id         SERIAL PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    VARCHAR(32) NOT NULL,           -- 'password_reset' | 'email_verification'
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_tokens_user_purpose
    ON email_tokens (user_id, purpose);

-- +goose Down
DROP TABLE IF EXISTS email_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
