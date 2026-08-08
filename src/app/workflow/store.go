package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the persistence layer for workflow jobs. It only ever handles
// json.RawMessage bytes — it never unmarshals payloads into concrete types
// (that is exclusively the Executor's job, via Definition.NewPayload), and it
// never knows about workflow definitions.
type Store interface {
	// Insert creates a job, or returns the existing row when one already
	// exists for the same (workflow_name, trace_id) pair (idempotent
	// registration — the existing payload is never overwritten).
	Insert(ctx context.Context, workflowName, traceID string, payload json.RawMessage, currentStep string) (*Entity, error)

	// AcquireNext claims one eligible job: a 'waiting' job, or a 'processing'
	// job whose lock is older than staleTimeout (stale reclaim — no separate
	// reaper process). Returns nil when nothing is eligible.
	AcquireNext(ctx context.Context, staleTimeout time.Duration) (*Entity, error)

	// AdvanceStep atomically persists the mutated payload and moves the job to
	// the next step, resetting retry_count and status to 'waiting'.
	AdvanceStep(ctx context.Context, id int64, payload json.RawMessage, nextStep string) error

	// Complete atomically persists the final mutated payload and marks the job done.
	Complete(ctx context.Context, id int64, payload json.RawMessage) error

	// UpdateRetry records a failed attempt. It does NOT touch payload — a
	// failed attempt's in-memory mutations are never persisted.
	UpdateRetry(ctx context.Context, id int64, retryCount int, lastErr string) error

	// Fail marks the job permanently failed. It does NOT touch payload.
	Fail(ctx context.Context, id int64, lastErr string) error
}

// NewStore creates a Store backed by the shared PostgreSQL pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &pgStore{pool: pool}
}

type pgStore struct {
	pool *pgxpool.Pool
}

const entityColumns = "id, workflow_name, trace_id, payload, status, current_step, retry_count, last_error, locked_at, created_at, updated_at"

func scanEntity(row pgx.Row) (*Entity, error) {
	var e Entity
	var payload []byte
	var status string
	var lastErr *string
	err := row.Scan(
		&e.ID, &e.WorkflowName, &e.TraceID, &payload, &status,
		&e.CurrentStep, &e.RetryCount, &lastErr, &e.LockedAt,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.Payload = json.RawMessage(payload)
	e.Status = Status(status)
	if lastErr != nil {
		e.LastError = *lastErr
	}
	return &e, nil
}

func (s *pgStore) Insert(ctx context.Context, workflowName, traceID string, payload json.RawMessage, currentStep string) (*Entity, error) {
	const q = `
INSERT INTO workflow_job (workflow_name, trace_id, payload, status, current_step)
VALUES ($1, $2, $3, 'waiting', $4)
ON CONFLICT (workflow_name, trace_id) DO UPDATE
    SET workflow_name = EXCLUDED.workflow_name
RETURNING ` + entityColumns
	// The no-op DO UPDATE forces RETURNING to fire on conflict too, avoiding a
	// second round trip. On conflict the existing row (and payload) is returned
	// untouched.
	return scanEntity(s.pool.QueryRow(ctx, q, workflowName, traceID, payload, currentStep))
}

func (s *pgStore) AcquireNext(ctx context.Context, staleTimeout time.Duration) (*Entity, error) {
	const q = `
UPDATE workflow_job
SET status = 'processing',
    locked_at = now(),
    updated_at = now()
WHERE id = (
    SELECT id
    FROM workflow_job
    WHERE status = 'waiting'
       OR (status = 'processing' AND locked_at < now() - $1::interval)
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING ` + entityColumns

	interval := pgtype.Interval{Microseconds: staleTimeout.Microseconds(), Valid: true}
	row := s.pool.QueryRow(ctx, q, interval)
	e, err := scanEntity(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (s *pgStore) AdvanceStep(ctx context.Context, id int64, payload json.RawMessage, nextStep string) error {
	const q = `
UPDATE workflow_job
SET payload = $2,
    current_step = $3,
    status = 'waiting',
    retry_count = 0,
    updated_at = now()
WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, payload, nextStep)
	return err
}

func (s *pgStore) Complete(ctx context.Context, id int64, payload json.RawMessage) error {
	const q = `
UPDATE workflow_job
SET payload = $2,
    status = 'done',
    updated_at = now()
WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, payload)
	return err
}

func (s *pgStore) UpdateRetry(ctx context.Context, id int64, retryCount int, lastErr string) error {
	const q = `
UPDATE workflow_job
SET status = 'waiting',
    retry_count = $2,
    last_error = $3,
    locked_at = NULL,
    updated_at = now()
WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, retryCount, lastErr)
	return err
}

func (s *pgStore) Fail(ctx context.Context, id int64, lastErr string) error {
	const q = `
UPDATE workflow_job
SET status = 'failed',
    last_error = $2,
    locked_at = NULL,
    updated_at = now()
WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, lastErr)
	return err
}
