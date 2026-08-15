package workflow_test

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
)

// fakeStore is an in-memory Store used by engine and worker tests. It behaves
// like the real one: AdvanceStep/Complete/UpdateRetry/Fail mutate the stored
// entity and record their calls for assertions.
type fakeStore struct {
	mu       sync.Mutex
	nextID   int64
	entities map[int64]*workflow.Entity

	acquires  int
	advances  []advanceCall
	completes []completeCall
	retries   []retryCall
	fails     []failCall

	// advanceErr / completeErr inject persistence failures, simulating a DB
	// outage during Execute so tests can exercise the fail-on-persist-error path.
	advanceErr  error
	completeErr error
}

type advanceCall struct {
	id       int64
	payload  json.RawMessage
	nextStep string
}

type completeCall struct {
	id      int64
	payload json.RawMessage
}

type retryCall struct {
	id         int64
	retryCount int
	lastErr    string
}

type failCall struct {
	id      int64
	lastErr string
}

var _ workflow.Store = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{entities: make(map[int64]*workflow.Entity)}
}

func (f *fakeStore) Insert(ctx context.Context, workflowName, traceID string, payload json.RawMessage, currentStep string) (*workflow.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	e := &workflow.Entity{
		ID:           f.nextID,
		WorkflowName: workflowName,
		TraceID:      traceID,
		Payload:      append(json.RawMessage(nil), payload...),
		Status:       workflow.StatusWaiting,
		CurrentStep:  currentStep,
	}
	f.entities[e.ID] = e
	return e, nil
}

func (f *fakeStore) AcquireBatch(ctx context.Context, staleTimeout time.Duration, limit int) ([]*workflow.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acquires++
	var batch []*workflow.Entity
	for _, e := range f.entities {
		if len(batch) == limit {
			break
		}
		if e.Status == workflow.StatusWaiting {
			e.Status = workflow.StatusProcessing
			batch = append(batch, e)
		}
	}
	return batch, nil
}

// entity returns the stored row for id, materialising an empty one on the fly
// so tests may hand an Entity straight to the executor (in production the row
// always exists once a job was acquired).
func (f *fakeStore) entity(id int64) *workflow.Entity {
	e, ok := f.entities[id]
	if !ok {
		e = &workflow.Entity{ID: id}
		f.entities[id] = e
	}
	return e
}

func (f *fakeStore) AdvanceStep(ctx context.Context, id int64, payload json.RawMessage, nextStep string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.advanceErr != nil {
		return f.advanceErr
	}
	e := f.entity(id)
	e.Payload = append(json.RawMessage(nil), payload...)
	e.CurrentStep = nextStep
	e.Status = workflow.StatusWaiting
	e.RetryCount = 0
	f.advances = append(f.advances, advanceCall{
		id: id, payload: append(json.RawMessage(nil), payload...), nextStep: nextStep,
	})
	return nil
}

func (f *fakeStore) Complete(ctx context.Context, id int64, payload json.RawMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.completeErr != nil {
		return f.completeErr
	}
	e := f.entity(id)
	e.Payload = append(json.RawMessage(nil), payload...)
	e.Status = workflow.StatusDone
	f.completes = append(f.completes, completeCall{
		id: id, payload: append(json.RawMessage(nil), payload...),
	})
	return nil
}

func (f *fakeStore) UpdateRetry(ctx context.Context, id int64, retryCount int, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := f.entity(id)
	e.RetryCount = retryCount
	e.LastError = lastErr
	e.Status = workflow.StatusWaiting
	f.retries = append(f.retries, retryCall{id: id, retryCount: retryCount, lastErr: lastErr})
	return nil
}

func (f *fakeStore) Fail(ctx context.Context, id int64, lastErr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := f.entity(id)
	e.Status = workflow.StatusFailed
	e.LastError = lastErr
	f.fails = append(f.fails, failCall{id: id, lastErr: lastErr})
	return nil
}

// fakeUserSvc implements workflow.UserService for workflow tests, returning
// scripted typed outputs and recording every call.
type fakeUserSvc struct {
	registerOutput *workflow.RegisterUserOutput
	grantOutput    *workflow.GrantPermissionOutput

	registerCalls []*workflow.RegisterUserInput
	grantCalls    []*workflow.GrantPermissionInput
}

var _ workflow.UserService = (*fakeUserSvc)(nil)

func (f *fakeUserSvc) RegisterUser(ctx context.Context, input *workflow.RegisterUserInput) *workflow.RegisterUserOutput {
	f.registerCalls = append(f.registerCalls, input)
	return f.registerOutput
}

func (f *fakeUserSvc) GrantPermissionSystem(ctx context.Context, input *workflow.GrantPermissionInput) *workflow.GrantPermissionOutput {
	f.grantCalls = append(f.grantCalls, input)
	return f.grantOutput
}
