-- +goose Up
CREATE TABLE IF NOT EXISTS password_history (
    id            SERIAL PRIMARY KEY,
    user_id       INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_history_user_created_at
    ON password_history (user_id, created_at DESC);

-- +goose Down
