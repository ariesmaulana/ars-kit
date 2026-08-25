-- +goose Up
-- Audit trail of every permission grant/revoke: who (actor), whom (target),
-- which permission, what action, when. actor_id is NULL for system-initiated
-- changes (e.g. background workflow steps, which have no acting user).
CREATE TABLE IF NOT EXISTS permission_audit (
    id SERIAL PRIMARY KEY,
    actor_id INTEGER,
    target_id INTEGER NOT NULL,
    permission VARCHAR(255) NOT NULL,
    action VARCHAR(10) NOT NULL CHECK (action IN ('grant', 'revoke')),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_permission_audit_target_id ON permission_audit(target_id);
CREATE INDEX IF NOT EXISTS idx_permission_audit_actor_id ON permission_audit(actor_id);

-- +goose Down
DROP TABLE IF EXISTS permission_audit;
