-- +goose Up
CREATE TABLE IF NOT EXISTS audit_log (
    id             BIGSERIAL PRIMARY KEY,
    event          VARCHAR(50) NOT NULL,
    actor_id       INT,
    target_user_id INT,
    metadata       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor_id ON audit_log (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_target_user_id ON audit_log (target_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_event ON audit_log (event);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
