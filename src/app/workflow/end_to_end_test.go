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

// TestEndToEndDemoRecoversFromCrash drives a full demo job through the real
// store, simulating a worker crash between steps: each step is executed by a
// fresh executor seeded only from the persisted row, proving the workflow
// resumes from persisted payload alone.
func TestEndToEndDemoRecoversFromCrash(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "End-to-end demo", func() {
			suite.Run(t, "completes across worker acquisitions, resuming from persisted payload after a simulated crash", func(t *testing.T, ctx context.Context, app *WorkflowApp) {
				userSvc := successUserSvc(7)
				engine := workflow.NewEngine(app.Store, workflow.Config{})
				engine.Register(workflow.DemoWorkflow(userSvc))

				// Register a job the way a business service would.
				job, err := engine.RegisterJob(ctx, workflow.NewRegisterDemoWorkflow("demo-trace-1", workflow.DemoWorkflowInput{
					Email:    "jane@example.com",
					Username: "janedoe",
				}))
				require.NoError(t, err)
				require.Equal(t, workflow.StatusWaiting, job.Status)
				require.Equal(t, "RegisterUser", job.CurrentStep)

				// Worker A: acquire + execute step 1, then "crash" before
				// running anything else. The persisted row now carries user_id
				// and points at GrantPermission.
				first, err := app.Store.AcquireBatch(ctx, 3*time.Minute, 5)
				require.NoError(t, err)
				require.Len(t, first, 1)
				workflow.NewExecutor(engine, app.Store).Execute(context.Background(), first[0])

				persisted, err := app.Store.AcquireBatch(ctx, 3*time.Minute, 5)
				require.NoError(t, err)
				require.Len(t, persisted, 1)
				assert.Equal(t, "GrantPermission", persisted[0].CurrentStep)

				var payload workflow.DemoWorkflowInput
				require.NoError(t, json.Unmarshal(persisted[0].Payload, &payload))
				assert.Equal(t, int64(7), payload.UserID, "step 1 mutation must survive in the persisted payload")

				// Worker B: a fresh executor with no in-memory state. It must
				// resume at GrantPermission from the persisted payload alone.
				workflow.NewExecutor(engine, app.Store).Execute(context.Background(), persisted[0])

				// The grant saw the user created in a previous "process
				// lifetime" — proof the resume used persisted state.
				require.Len(t, userSvc.grantCalls, 1)
				assert.Equal(t, int64(7), userSvc.grantCalls[0].UserID)

				// Nothing left to acquire → the job completed.
				leftover, err := app.Store.AcquireBatch(ctx, 3*time.Minute, 5)
				require.NoError(t, err)
				assert.Empty(t, leftover)

				done := app.getJob(ctx, t, job.ID)
				assert.Equal(t, workflow.StatusDone, done.Status)
				assert.Zero(t, done.RetryCount)

				final := workflow.DemoWorkflowInput{}
				require.NoError(t, json.Unmarshal(done.Payload, &final))
				assert.Equal(t, int64(7), final.UserID)
			})
		})
	})
}
