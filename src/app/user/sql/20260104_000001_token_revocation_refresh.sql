-- +goose Up
-- A5: Token revocation + refresh rotation.
--
-- users.token_version is bumped on security-sensitive events (password
-- change). Every access token carries the version it was issued at, and every
-- refresh token snapshots it; any token whose version is behind the user's
-- current one is rejected. A password change therefore invalidates every
-- previously issued token (access and refresh) in one shot.
ALTER TABLE users ADD COLUMN token_version INT NOT NULL DEFAULT 0;

-- refresh_tokens is the server-side source of truth for refresh tokens. Only
-- the SHA-256 hash of the opaque token is stored, never the token itself.
-- Rotation (POST /users/refresh) revokes the old row and inserts a new one, so
-- a stolen refresh token can only be used once. Logout revokes the row so the
-- token cannot be replayed. token_version snapshots users.token_version at
-- issuance; a mismatch at refresh time means the session was invalidated
-- (e.g. password change).
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id            SERIAL PRIMARY KEY,
    user_id       INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash    VARCHAR(64) NOT NULL UNIQUE,
    token_version INT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS token_version;
