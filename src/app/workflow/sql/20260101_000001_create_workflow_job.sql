-- +goose Up
CREATE TABLE workflow_job (
    id            bigserial PRIMARY KEY,
    workflow_name varchar     NOT NULL,
    trace_id      varchar     NOT NULL,
    payload       jsonb       NOT NULL,
    status        varchar     NOT NULL DEFAULT 'waiting',
    current_step  varchar     NOT NULL,
    retry_count   int         NOT NULL DEFAULT 0,
    last_error    text,
    locked_at     timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_workflow_job_name_trace UNIQUE (workflow_name, trace_id)
);

CREATE INDEX idx_workflow_job_status ON workflow_job (status, updated_at);

-- +goose Down
DROP TABLE IF EXISTS workflow_job;
