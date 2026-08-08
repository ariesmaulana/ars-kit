package workflow_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingStore never has jobs but counts AcquireNext calls.
type countingStore struct {
	mu       sync.Mutex
	acquires int
}

var _ workflow.Store = (*countingStore)(nil)

func (c *countingStore) Insert(ctx context.Context, workflowName, traceID string, payload json.RawMessage, currentStep string) (*workflow.Entity, error) {
	return nil, nil
}

func (c *countingStore) AcquireBatch(ctx context.Context, staleTimeout time.Duration, limit int) ([]*workflow.Entity, error) {
	c.mu.Lock()
	c.acquires++
	c.mu.Unlock()
	return nil, nil
}

func (c *countingStore) AdvanceStep(ctx context.Context, id int64, payload json.RawMessage, nextStep string) error {
	return nil
}
func (c *countingStore) Complete(ctx context.Context, id int64, payload json.RawMessage) error {
	return nil
}
func (c *countingStore) UpdateRetry(ctx context.Context, id int64, retryCount int, lastErr string) error {
	return nil
}
func (c *countingStore) Fail(ctx context.Context, id int64, lastErr string) error { return nil }

func (c *countingStore) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.acquires
}

func TestWorkerStopsAcquiringAfterCancel(t *testing.T) {
	cs := &countingStore{}
	engine := workflow.NewEngine(cs, workflow.Config{
		Workers: 1, PollInterval: time.Millisecond, StaleTimeout: time.Minute,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		engine.Run(ctx)
		close(done)
	}()

	// Wait for the worker to poll at least twice, then cancel.
	require.Eventually(t, func() bool { return cs.count() >= 2 }, 2*time.Second, 5*time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("engine.Run did not return after ctx cancellation")
	}

	after := cs.count()
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, after, cs.count(), "worker kept acquiring after ctx was cancelled")
}

// blockingStep signals when it starts, then blocks until release, and signals
// when it actually finishes.
type blockingStep struct {
	started chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (s *blockingStep) Name() string { return "blockingStep" }

func (s *blockingStep) Run(ctx context.Context, run *workflow.Run) error {
	close(s.started)
	<-s.release
	close(s.done)
	return nil
}

func TestWorkerFinishesAcquiredJobAfterCancel(t *testing.T) {
	fs := newFakeStore()
	step := &blockingStep{started: make(chan struct{}), release: make(chan struct{}), done: make(chan struct{})}
	engine := workflow.NewEngine(fs, workflow.Config{
		Workers: 1, PollInterval: time.Millisecond, StaleTimeout: time.Minute,
	})
	engine.Register(&workflow.Definition{
		Name:       "blocking",
		MaxRetries: 0,
		NewPayload: func() any { return &struct{}{} },
		Steps:      []workflow.Step{step},
	})

	_, err := fs.Insert(context.Background(), "blocking", "trace-1", json.RawMessage(`{}`), "blockingStep")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		engine.Run(ctx)
		close(done)
	}()

	// The worker acquired the job and the step is running.
	<-step.started
	cancel()

	// Give the worker a moment to observe the cancellation: the in-flight step
	// must NOT be aborted.
	time.Sleep(20 * time.Millisecond)
	select {
	case <-step.done:
		t.Fatal("step was aborted by cancellation; it must run to completion")
	default:
	}

	close(step.release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("engine.Run did not return after the step completed")
	}

	select {
	case <-step.done:
	default:
		t.Fatal("step did not run to completion despite the cancellation")
	}

	// The job was completed, not left dangling in 'processing'.
	require.Len(t, fs.completes, 1)
	assert.Equal(t, workflow.StatusDone, fs.entities[1].Status)
}
