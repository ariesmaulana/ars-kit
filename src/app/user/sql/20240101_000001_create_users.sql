-- +goose Up

CREATE TYPE user_status AS ENUM ('active', 'disabled', 'suspended');

CREATE TABLE IF NOT EXISTS users (
    id                     SERIAL PRIMARY KEY,
    username               VARCHAR(255) NOT NULL UNIQUE,
    email                  VARCHAR(255) NOT NULL UNIQUE,
    full_name              VARCHAR(255) NOT NULL,
    password               VARCHAR(255) NOT NULL,
    status                 user_status NOT NULL DEFAULT 'active',
    token_version          INT NOT NULL DEFAULT 0,
    failed_login_attempts  INT NOT NULL DEFAULT 0,
    last_failed_login_at   TIMESTAMP NULL,
    locked_until           TIMESTAMP NULL,
    last_login_at          TIMESTAMPTZ NULL,
    email_verified_at      TIMESTAMPTZ NULL,
    avatar_key             VARCHAR(255) NULL,
    created_at             TIMESTAMP DEFAULT NOW(),
    updated_at             TIMESTAMP DEFAULT NOW()
);

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

CREATE TABLE IF NOT EXISTS password_history (
    id            SERIAL PRIMARY KEY,
    user_id       INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_history_user_created_at
    ON password_history (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS email_tokens (
    id         SERIAL PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    VARCHAR(32) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_tokens_user_purpose
    ON email_tokens (user_id, purpose);

-- +goose Down
