package workflow_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const demoPayload = `{"Email":"jane@example.com","Username":"janedoe","UserID":0}`

// newTestEngine builds an engine with defaults and registers the given definitions.
func newTestEngine(fs workflow.Store, defs ...*workflow.Definition) *workflow.Engine {
	engine := workflow.NewEngine(fs, workflow.Config{})
	engine.Register(defs...)
	return engine
}

// seedJob inserts a job into the fake store and marks it processing, the state
// a real worker would have produced via AcquireBatch.
func seedJob(t *testing.T, fs *fakeStore, workflowName, traceID, payload, currentStep string, retryCount int) *workflow.Entity {
	t.Helper()
	entity, err := fs.Insert(context.Background(), workflowName, traceID, json.RawMessage(payload), currentStep)
	require.NoError(t, err)
	entity.Status = workflow.StatusProcessing
	entity.RetryCount = retryCount
	return entity
}

// successUserSvc returns a fakeUserSvc whose seams always succeed, creating
// users with the given ID.
func successUserSvc(userID int64) *fakeUserSvc {
	return &fakeUserSvc{
		registerOutput: &workflow.RegisterUserOutput{
			Success: true,
			Message: "User registered",
			User:    workflow.User{ID: userID},
		},
		grantOutput: &workflow.GrantPermissionOutput{Success: true, Message: "Permission granted"},
	}
}

func TestExecutorWorkflowNotFound(t *testing.T) {
	fs := newFakeStore()
	executor := workflow.NewExecutor(newTestEngine(fs), fs)

	entity := &workflow.Entity{
		ID: 1, WorkflowName: "does_not_exist", TraceID: "trace-1",
		Payload: json.RawMessage(`{}`), Status: workflow.StatusProcessing, CurrentStep: "RegisterUser",
	}
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.fails, 1)
	assert.Equal(t, int64(1), fs.fails[0].id)
	assert.Contains(t, fs.fails[0].lastErr, "does_not_exist")
	assert.Empty(t, fs.advances)
	assert.Empty(t, fs.completes)
	assert.Empty(t, fs.retries)
}

func TestExecutorStepNotFoundFailsLoudly(t *testing.T) {
	fs := newFakeStore()
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(nil)), fs)

	entity := &workflow.Entity{
		ID: 1, WorkflowName: "demo", TraceID: "trace-1",
		Payload: json.RawMessage(`{}`), Status: workflow.StatusProcessing, CurrentStep: "missingStep",
	}
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.fails, 1)
	assert.Contains(t, fs.fails[0].lastErr, `current_step "missingStep" not found`)
	assert.Contains(t, fs.fails[0].lastErr, "RegisterUser") // step names listed for the operator
	assert.Empty(t, fs.advances)
	assert.Empty(t, fs.retries)
}

func TestExecutorAdvancesOnSuccess(t *testing.T) {
	fs := newFakeStore()
	userSvc := successUserSvc(42)
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	entity := seedJob(t, fs, "demo", "trace-1", demoPayload, "RegisterUser", 0)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.advances, 1)
	assert.Equal(t, entity.ID, fs.advances[0].id)
	assert.Equal(t, "GrantPermission", fs.advances[0].nextStep)
	assert.Empty(t, fs.completes)
	assert.Empty(t, fs.retries)
	assert.Empty(t, fs.fails)

	// The mutated payload was persisted atomically with the advance.
	var payload workflow.DemoWorkflowInput
	require.NoError(t, json.Unmarshal(fs.advances[0].payload, &payload))
	assert.Equal(t, int64(42), payload.UserID)
	assert.Equal(t, "jane@example.com", payload.Email)
	assert.Equal(t, "janedoe", payload.Username)
}

func TestExecutorCompletesOnLastStep(t *testing.T) {
	fs := newFakeStore()
	userSvc := successUserSvc(42)
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	entity := seedJob(t, fs, "demo", "trace-1", `{"Email":"jane@example.com","Username":"janedoe","UserID":42}`, "GrantPermission", 0)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.completes, 1)
	assert.Equal(t, entity.ID, fs.completes[0].id)
	assert.Empty(t, fs.advances)
	assert.Empty(t, fs.retries)
	assert.Empty(t, fs.fails)

	// Step 2 received the UserID persisted by step 1.
	require.Len(t, userSvc.grantCalls, 1)
	assert.Equal(t, int64(42), userSvc.grantCalls[0].UserID)
	assert.Equal(t, "member", userSvc.grantCalls[0].RoleName)
}

func TestExecutorRetriesOnStepError(t *testing.T) {
	fs := newFakeStore()
	userSvc := &fakeUserSvc{
		registerOutput: &workflow.RegisterUserOutput{Message: "database down", ErrorCode: workflow.ErrorCodeInternal},
	}
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	entity := seedJob(t, fs, "demo", "trace-1", demoPayload, "RegisterUser", 0)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.retries, 1)
	assert.Equal(t, entity.ID, fs.retries[0].id)
	assert.Equal(t, 1, fs.retries[0].retryCount)
	assert.Equal(t, "register user: database down", fs.retries[0].lastErr)
	assert.Empty(t, fs.advances)
	assert.Empty(t, fs.completes)
	assert.Empty(t, fs.fails)

	// The payload column is byte-for-byte untouched after a failed attempt.
	assert.Equal(t, json.RawMessage(demoPayload), fs.entities[entity.ID].Payload)
}

func TestExecutorRetriesUpToMaxRetries(t *testing.T) {
	fs := newFakeStore()
	userSvc := &fakeUserSvc{registerOutput: &workflow.RegisterUserOutput{Message: "boom"}}
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	// RetryCount == MaxRetries-1 → one more retry is allowed (becomes MaxRetries).
	entity := seedJob(t, fs, "demo", "trace-1", demoPayload, "RegisterUser", 2)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.retries, 1)
	assert.Equal(t, 3, fs.retries[0].retryCount)
	assert.Empty(t, fs.fails)
}

func TestExecutorFailsAfterMaxRetries(t *testing.T) {
	fs := newFakeStore()
	userSvc := &fakeUserSvc{registerOutput: &workflow.RegisterUserOutput{Message: "boom"}}
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	// MaxRetries = 3 means 3 re-runs; the 4th attempt (RetryCount == 3) fails.
	entity := seedJob(t, fs, "demo", "trace-1", demoPayload, "RegisterUser", 3)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.fails, 1)
	assert.Equal(t, entity.ID, fs.fails[0].id)
	assert.Equal(t, "register user: boom", fs.fails[0].lastErr)
	assert.Empty(t, fs.retries)
	assert.Empty(t, fs.advances)
	assert.Equal(t, json.RawMessage(demoPayload), fs.entities[entity.ID].Payload)
}

// mutateThenFailStep mutates the payload and then returns an error — proving
// that a failed attempt's in-memory mutations are never persisted.
type mutateThenFailStep struct{}

func (s *mutateThenFailStep) Name() string { return "mutateThenFailStep" }

func (s *mutateThenFailStep) Run(ctx context.Context, run *workflow.Run) error {
	payload := run.Payload.(*workflow.DemoWorkflowInput)
	payload.UserID = 999 // in-memory mutation, must be discarded
	return errors.New("kaboom")
}

func TestExecutorDiscardsMutationOnStepError(t *testing.T) {
	fs := newFakeStore()
	executor := workflow.NewExecutor(newTestEngine(fs, &workflow.Definition{
		Name:       "mutator",
		MaxRetries: 3,
		NewPayload: func() any { return &workflow.DemoWorkflowInput{} },
		Steps:      []workflow.Step{&mutateThenFailStep{}},
	}), fs)

	entity := seedJob(t, fs, "mutator", "trace-1", demoPayload, "mutateThenFailStep", 0)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.retries, 1)
	assert.Empty(t, fs.advances, "no payload write may happen on failure")
	assert.Empty(t, fs.completes)
	assert.Equal(t, json.RawMessage(demoPayload), fs.entities[entity.ID].Payload,
		"failed attempt's mutations must never reach the payload column")
}

type panicStep struct{}

func (s *panicStep) Name() string { return "panicStep" }

func (s *panicStep) Run(ctx context.Context, run *workflow.Run) error {
	panic("step bug")
}

func TestExecutorConvertsPanicToFailure(t *testing.T) {
	fs := newFakeStore()
	executor := workflow.NewExecutor(newTestEngine(fs, &workflow.Definition{
		Name:       "panicky",
		MaxRetries: 0,
		NewPayload: func() any { return &workflow.DemoWorkflowInput{} },
		Steps:      []workflow.Step{&panicStep{}},
	}), fs)

	entity := seedJob(t, fs, "panicky", "trace-1", `{}`, "panicStep", 0)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.fails, 1)
	assert.Contains(t, fs.fails[0].lastErr, "panicked")
	assert.Contains(t, fs.fails[0].lastErr, "step bug")
}

func TestExecutorFailsLoudlyOnUnmarshalError(t *testing.T) {
	fs := newFakeStore()
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(nil)), fs)

	// A payload whose shape no longer matches the definition's NewPayload type
	// (breaking payload deploy) must fail loudly, not panic or zero-fill.
	entity := seedJob(t, fs, "demo", "trace-1", `"not an object"`, "RegisterUser", 0)
	executor.Execute(context.Background(), entity)

	require.Len(t, fs.fails, 1)
	assert.Contains(t, fs.fails[0].lastErr, "unmarshal payload")
	assert.Empty(t, fs.advances)
	assert.Empty(t, fs.retries)
}

// hangStep signals when it starts, then blocks until its context is cancelled
// (as a real step observing cancellation would) — simulating a hung step that
// gets reclaimed by the StepTimeout.
type hangStep struct {
	started chan struct{}
}

func (s *hangStep) Name() string { return "hangStep" }

func (s *hangStep) Run(ctx context.Context, run *workflow.Run) error {
	close(s.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestExecutorTimesOutHungStep(t *testing.T) {
	fs := newFakeStore()
	step := &hangStep{started: make(chan struct{})}
	engine := workflow.NewEngine(fs, workflow.Config{StepTimeout: 50 * time.Millisecond})
	engine.Register(&workflow.Definition{
		Name:       "hangy",
		MaxRetries: 0,
		NewPayload: func() any { return &struct{}{} },
		Steps:      []workflow.Step{step},
	})
	executor := workflow.NewExecutor(engine, fs)

	entity := seedJob(t, fs, "hangy", "trace-1", `{}`, "hangStep", 0)
	err := executor.Execute(context.Background(), entity)

	// A timed-out step is a handled step failure: the job is failed, and
	// Execute itself returns nil (nothing to propagate).
	require.NoError(t, err)
	require.Len(t, fs.fails, 1)
	assert.Equal(t, entity.ID, fs.fails[0].id)
	assert.Contains(t, fs.fails[0].lastErr, "exceeded timeout")
	assert.Empty(t, fs.retries)
	assert.Empty(t, fs.advances)
	assert.Empty(t, fs.completes)
	assert.Equal(t, json.RawMessage(`{}`), fs.entities[entity.ID].Payload, "timeout must never persist a partial mutation")
}

func TestExecutorTimeoutRecordsRetryUntilMaxRetries(t *testing.T) {
	fs := newFakeStore()
	step := &hangStep{started: make(chan struct{})}
	engine := workflow.NewEngine(fs, workflow.Config{StepTimeout: 50 * time.Millisecond})
	engine.Register(&workflow.Definition{
		Name:       "hangy",
		MaxRetries: 3,
		NewPayload: func() any { return &struct{}{} },
		Steps:      []workflow.Step{step},
	})
	executor := workflow.NewExecutor(engine, fs)

	entity := seedJob(t, fs, "hangy", "trace-1", `{}`, "hangStep", 0)
	require.NoError(t, executor.Execute(context.Background(), entity))

	require.Len(t, fs.retries, 1)
	assert.Equal(t, 1, fs.retries[0].retryCount)
	assert.Contains(t, fs.retries[0].lastErr, "exceeded timeout")
	assert.Empty(t, fs.fails, "with retries left a timeout must retry, not fail")
}

func TestExecutorReturnsErrorWhenAdvanceFails(t *testing.T) {
	fs := newFakeStore()
	fs.advanceErr = errors.New("connection reset")
	userSvc := successUserSvc(42)
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	entity := seedJob(t, fs, "demo", "trace-1", demoPayload, "RegisterUser", 0)
	err := executor.Execute(context.Background(), entity)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "advance step")
	assert.Contains(t, err.Error(), "connection reset")
	// The step ran and mutated the in-memory payload, but nothing reached the
	// store. The executor surfaces the error; the worker decides to fail.
	assert.Empty(t, fs.fails)
	assert.Empty(t, fs.retries)
	assert.Empty(t, fs.advances)
	assert.Empty(t, fs.completes)
	assert.Equal(t, json.RawMessage(demoPayload), fs.entities[entity.ID].Payload,
		"a failed advance must not write a partial payload")
}

func TestExecutorReturnsErrorWhenCompleteFails(t *testing.T) {
	fs := newFakeStore()
	fs.completeErr = errors.New("connection reset")
	userSvc := successUserSvc(42)
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	entity := seedJob(t, fs, "demo", "trace-1", `{"Email":"jane@example.com","Username":"janedoe","UserID":42}`, "GrantPermission", 0)
	err := executor.Execute(context.Background(), entity)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "complete final step")
	assert.Contains(t, err.Error(), "connection reset")
	assert.Empty(t, fs.fails)
	assert.Empty(t, fs.completes)
	assert.Equal(t, workflow.StatusProcessing, fs.entities[entity.ID].Status,
		"the job stays processing until the worker decides its fate")
}

func TestExecutorResumesFromPersistedPayload(t *testing.T) {
	// The concrete proof that a workflow is resumable from persisted state
	// alone: a fresh, independent Execute for step 2 seeded only from the
	// entity as persisted by step 1's AdvanceStep call — no leftover in-memory
	// reference to step 1's payload struct.
	fs := newFakeStore()
	userSvc := successUserSvc(42)
	executor := workflow.NewExecutor(newTestEngine(fs, workflow.DemoWorkflow(userSvc)), fs)

	step1 := seedJob(t, fs, "demo", "trace-1", demoPayload, "RegisterUser", 0)
	executor.Execute(context.Background(), step1)
	require.Len(t, fs.advances, 1)

	// Reconstruct the next acquisition purely from the persisted bytes.
	persisted := fs.entities[step1.ID]
	step2 := &workflow.Entity{
		ID:           persisted.ID,
		WorkflowName: persisted.WorkflowName,
		TraceID:      persisted.TraceID,
		Payload:      append(json.RawMessage(nil), persisted.Payload...),
		Status:       workflow.StatusWaiting,
		CurrentStep:  persisted.CurrentStep,
		RetryCount:   persisted.RetryCount,
	}
	executor.Execute(context.Background(), step2)

	// Step 2 (GrantPermission) must have seen step 1's mutation from the
	// persisted payload alone.
	require.Len(t, userSvc.grantCalls, 1)
	assert.Equal(t, int64(42), userSvc.grantCalls[0].UserID)

	// The job completed.
	require.Len(t, fs.completes, 1)
	assert.Equal(t, workflow.StatusDone, fs.entities[step1.ID].Status)
}
