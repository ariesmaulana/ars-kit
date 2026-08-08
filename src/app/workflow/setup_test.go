package workflow_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ariesmaulana/ars-kit/database"
	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/require"
)

// WorkflowApp holds the initialized workflow components for testing.
type WorkflowApp struct {
	*testsuite.AppContext
	Store workflow.Store
}

// TestSuite wraps testsuite.Suite for workflow tests.
type TestSuite struct {
	*testsuite.Suite
}

// Run executes a test scenario with an initialized WorkflowApp.
func (ts *TestSuite) Run(t *testing.T, scenario string, fn func(t *testing.T, ctx context.Context, app *WorkflowApp)) {
	ts.Runs(t, scenario, func(t *testing.T, appCtx *testsuite.AppContext) {
		ctx := context.Background()
		app := &WorkflowApp{AppContext: appCtx, Store: workflow.NewStore(appCtx.Pool)}
		fn(t, ctx, app)
	})
}

// RunTest is a wrapper that sets up and tears down the workflow test suite.
func RunTest(t *testing.T, testFunc func(t *testing.T, suite *TestSuite)) {
	t.Parallel()
	cfg := testsuite.InitTestConfig()

	baseSuite, err := testsuite.NewSuite(cfg, database.WorkflowOnly)
	if err != nil {
		t.Fatalf("Failed to create test suite: %v", err)
	}

	t.Cleanup(func() {
		baseSuite.Close()
	})

	suite := &TestSuite{Suite: baseSuite}
	testFunc(t, suite)
}

// getJob reads a workflow_job row back directly. The Store intentionally has
// no read-by-id method; this is a test helper.
func (app *WorkflowApp) getJob(ctx context.Context, t *testing.T, id int64) *workflow.Entity {
	t.Helper()
	var e workflow.Entity
	var payload []byte
	var status string
	var lastErr *string
	err := app.Pool.QueryRow(ctx, `
SELECT id, workflow_name, trace_id, payload, status, current_step, retry_count, last_error, locked_at, created_at, updated_at
FROM workflow_job
WHERE id = $1`, id).
		Scan(&e.ID, &e.WorkflowName, &e.TraceID, &payload, &status, &e.CurrentStep,
			&e.RetryCount, &lastErr, &e.LockedAt, &e.CreatedAt, &e.UpdatedAt)
	require.NoError(t, err)
	e.Payload = json.RawMessage(payload)
	e.Status = workflow.Status(status)
	if lastErr != nil {
		e.LastError = *lastErr
	}
	return &e
}

// countJobs returns the number of rows in workflow_job.
func (app *WorkflowApp) countJobs(ctx context.Context, t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, app.Pool.QueryRow(ctx, "SELECT count(*) FROM workflow_job").Scan(&n))
	return n
}
