package workflow_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterJob(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "RegisterJob", func() {
			suite.Run(t, "inserts a waiting job at the first step with the exact initial payload", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				engine := workflow.NewEngine(app.Store, workflow.Config{})
				engine.Register(workflow.DemoWorkflow(nil))

				input := workflow.DemoWorkflowInput{Email: "jane@example.com", Username: "janedoe"}
				job, err := engine.RegisterJob(ctx, workflow.NewRegisterDemoWorkflow("trace-1", input))
				require.NoError(t, err)
				require.NotNil(t, job)

				assert.Equal(t, "demo", job.WorkflowName)
				assert.Equal(t, "trace-1", job.TraceID)
				assert.Equal(t, workflow.StatusWaiting, job.Status)
				assert.Equal(t, "RegisterUser", job.CurrentStep)
				assert.Zero(t, job.RetryCount)

				// The initial payload is stored exactly as given — no mutation
				// at registration time.
				expected, err := json.Marshal(input)
				require.NoError(t, err)
				assert.JSONEq(t, string(expected), string(job.Payload))
			})

			suite.Run(t, "re-registering the same (workflow_name, trace_id) returns the same job", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				engine := workflow.NewEngine(app.Store, workflow.Config{})
				engine.Register(workflow.DemoWorkflow(nil))

				first, err := engine.RegisterJob(ctx, workflow.NewRegisterDemoWorkflow("trace-dup", workflow.DemoWorkflowInput{Email: "jane@example.com", Username: "janedoe"}))
				require.NoError(t, err)

				// Same trace, different payload: the existing row wins untouched.
				second, err := engine.RegisterJob(ctx, workflow.NewRegisterDemoWorkflow("trace-dup", workflow.DemoWorkflowInput{Email: "other@example.com", Username: "someoneelse"}))
				require.NoError(t, err)

				assert.Equal(t, first.ID, second.ID)
				assert.JSONEq(t, `{"Email":"jane@example.com","Username":"janedoe","UserID":0}`, string(second.Payload))
				assert.Equal(t, 1, app.countJobs(ctx, t))
			})

			suite.Run(t, "rejects an empty trace id", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				engine := workflow.NewEngine(app.Store, workflow.Config{})
				engine.Register(workflow.DemoWorkflow(nil))

				_, err := engine.RegisterJob(ctx, workflow.NewRegisterDemoWorkflow("", workflow.DemoWorkflowInput{Email: "jane@example.com", Username: "janedoe"}))
				assert.ErrorIs(t, err, workflow.ErrTraceIDRequired)
				assert.Equal(t, 0, app.countJobs(ctx, t))
			})

			suite.Run(t, "rejects an unregistered workflow before touching the database", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				engine := workflow.NewEngine(app.Store, workflow.Config{})

				_, err := engine.RegisterJob(ctx, workflow.NewRegisterDemoWorkflow("trace-1", workflow.DemoWorkflowInput{Email: "jane@example.com", Username: "janedoe"}))
				assert.ErrorIs(t, err, workflow.ErrWorkflowNotRegistered)
				assert.Equal(t, 0, app.countJobs(ctx, t))
			})
		})
	})
}

func TestPackageRegister(t *testing.T) {
	fs := newFakeStore()
	engine := workflow.NewEngine(fs, workflow.Config{})
	engine.Register(workflow.DemoWorkflow(nil))

	workflow.SetDefault(engine)
	defer workflow.SetDefault(nil)

	job, err := workflow.Register(context.Background(), workflow.NewRegisterDemoWorkflow("trace-1", workflow.DemoWorkflowInput{Email: "jane@example.com", Username: "janedoe"}))
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "demo", job.WorkflowName)
	assert.Equal(t, "RegisterUser", job.CurrentStep)

	// Without an installed engine, the package-level Register fails fast.
	workflow.SetDefault(nil)
	_, err = workflow.Register(context.Background(), workflow.NewRegisterDemoWorkflow("trace-2", workflow.DemoWorkflowInput{}))
	assert.ErrorIs(t, err, workflow.ErrEngineNotInstalled)
}
