package workflow_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreInsert(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Store Insert", func() {
			suite.Run(t, "inserts a waiting job with the initial payload and first step", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				job, err := app.Store.Insert(ctx, "demo", "trace-1", json.RawMessage(`{"total":10000}`), "RegisterUser")
				require.NoError(t, err)
				require.NotNil(t, job)

				assert.NotZero(t, job.ID)
				assert.Equal(t, "demo", job.WorkflowName)
				assert.Equal(t, "trace-1", job.TraceID)
				assert.Equal(t, workflow.StatusWaiting, job.Status)
				assert.Equal(t, "RegisterUser", job.CurrentStep)
				assert.Zero(t, job.RetryCount)
				assert.JSONEq(t, `{"total":10000}`, string(job.Payload))
			})

			suite.Run(t, "duplicate (workflow_name, trace_id) returns the existing row without overwriting it", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				first, err := app.Store.Insert(ctx, "demo", "trace-dup", json.RawMessage(`{"total":10000}`), "RegisterUser")
				require.NoError(t, err)

				second, err := app.Store.Insert(ctx, "demo", "trace-dup", json.RawMessage(`{"total":99999}`), "RegisterUser")
				require.NoError(t, err)

				assert.Equal(t, first.ID, second.ID)
				assert.JSONEq(t, `{"total":10000}`, string(second.Payload))
				assert.Equal(t, 1, app.countJobs(ctx, t))
			})
		})
	})
}

func TestStoreAcquireNext(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Store AcquireNext", func() {
			suite.Run(t, "picks up a waiting job and marks it processing", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				job, err := app.Store.Insert(ctx, "demo", "trace-1", json.RawMessage(`{}`), "RegisterUser")
				require.NoError(t, err)

				acquired, err := app.Store.AcquireNext(ctx, 5*time.Minute)
				require.NoError(t, err)
				require.NotNil(t, acquired)
				assert.Equal(t, job.ID, acquired.ID)
				assert.Equal(t, workflow.StatusProcessing, acquired.Status)
			})

			suite.Run(t, "re-acquires a processing job older than staleTimeout", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				job, err := app.Store.Insert(ctx, "demo", "trace-1", json.RawMessage(`{}`), "RegisterUser")
				require.NoError(t, err)

				acquired, err := app.Store.AcquireNext(ctx, 5*time.Minute)
				require.NoError(t, err)
				require.NotNil(t, acquired)

				// Simulate a crashed worker: age the lock beyond staleTimeout.
				_, err = app.Pool.Exec(ctx, "UPDATE workflow_job SET locked_at = now() - interval '10 minutes' WHERE id = $1", job.ID)
				require.NoError(t, err)

				reclaimed, err := app.Store.AcquireNext(ctx, 5*time.Minute)
				require.NoError(t, err)
				require.NotNil(t, reclaimed)
				assert.Equal(t, job.ID, reclaimed.ID)
				assert.Equal(t, workflow.StatusProcessing, reclaimed.Status)
				assert.WithinDuration(t, time.Now(), *reclaimed.LockedAt, time.Minute)
			})

			suite.Run(t, "does not re-acquire a fresh processing job", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				_, err := app.Store.Insert(ctx, "demo", "trace-1", json.RawMessage(`{}`), "RegisterUser")
				require.NoError(t, err)

				acquired, err := app.Store.AcquireNext(ctx, 5*time.Minute)
				require.NoError(t, err)
				require.NotNil(t, acquired)

				// No waiting jobs, and the processing job is still fresh.
				again, err := app.Store.AcquireNext(ctx, 5*time.Minute)
				require.NoError(t, err)
				assert.Nil(t, again)
			})

			suite.Run(t, "returns nil when nothing is eligible", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				job, err := app.Store.AcquireNext(ctx, 5*time.Minute)
				require.NoError(t, err)
				assert.Nil(t, job)
			})
		})
	})
}

func TestStoreAdvanceStep(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Store AdvanceStep", func() {
			suite.Run(t, "persists payload, advances step, and resets retry_count atomically", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				job, err := app.Store.Insert(ctx, "demo", "trace-1", json.RawMessage(`{"total":10000}`), "RegisterUser")
				require.NoError(t, err)

				// Simulate a previously failed attempt so the reset is observable.
				require.NoError(t, app.Store.UpdateRetry(ctx, job.ID, 2, "boom"))

				require.NoError(t, app.Store.AdvanceStep(ctx, job.ID,
					json.RawMessage(`{"total":10000,"order_id":42}`), "GrantPermission"))

				row := app.getJob(ctx, t, job.ID)
				assert.JSONEq(t, `{"total":10000,"order_id":42}`, string(row.Payload))
				assert.Equal(t, "GrantPermission", row.CurrentStep)
				assert.Equal(t, workflow.StatusWaiting, row.Status)
				assert.Zero(t, row.RetryCount)
			})
		})
	})
}

func TestStoreComplete(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Store Complete", func() {
			suite.Run(t, "persists the final payload and marks the job done", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				job, err := app.Store.Insert(ctx, "demo", "trace-1", json.RawMessage(`{}`), "GrantPermission")
				require.NoError(t, err)

				require.NoError(t, app.Store.Complete(ctx, job.ID, json.RawMessage(`{"invoice_id":789}`)))

				row := app.getJob(ctx, t, job.ID)
				assert.JSONEq(t, `{"invoice_id":789}`, string(row.Payload))
				assert.Equal(t, workflow.StatusDone, row.Status)
			})
		})
	})
}

func TestStoreUpdateRetryAndFailLeavePayloadUntouched(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Store UpdateRetry / Fail", func() {
			suite.Run(t, "retry and fail never write the payload column", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				job, err := app.Store.Insert(ctx, "demo", "trace-1", json.RawMessage(`{"total":10000}`), "RegisterUser")
				require.NoError(t, err)

				before := app.getJob(ctx, t, job.ID)
				beforePayload := append([]byte(nil), before.Payload...)

				require.NoError(t, app.Store.UpdateRetry(ctx, job.ID, 1, "temporary failure"))
				mid := app.getJob(ctx, t, job.ID)
				assert.Equal(t, workflow.StatusWaiting, mid.Status)
				assert.Equal(t, 1, mid.RetryCount)
				assert.Equal(t, "temporary failure", mid.LastError)
				assert.Equal(t, beforePayload, []byte(mid.Payload), "payload must be byte-for-byte identical")

				require.NoError(t, app.Store.Fail(ctx, job.ID, "permanent failure"))
				after := app.getJob(ctx, t, job.ID)
				assert.Equal(t, workflow.StatusFailed, after.Status)
				assert.Equal(t, "permanent failure", after.LastError)
				assert.Equal(t, beforePayload, []byte(after.Payload), "payload must be byte-for-byte identical")
			})
		})
	})
}
