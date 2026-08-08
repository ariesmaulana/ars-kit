package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Job describes a workflow job to be enqueued. Each workflow definition file
// ships a concrete job type implementing it (see RegisterDemoWorkflow), so
// business code registers jobs without holding an engine reference:
//
//	workflow.Register(ctx, workflow.RegisterDemoWorkflow{TraceId: ..., Payload: ...})
type Job interface {
	WorkflowName() string
	TraceId() string
	Payload() any
}

// defaultEngine is the engine used by the package-level Register function,
// installed once at bootstrap via SetDefault.
var defaultEngine *Engine

// SetDefault installs the engine used by the package-level Register. It is
// called once at bootstrap, after the engine is created and its definitions
// are registered.
func SetDefault(e *Engine) {
	defaultEngine = e
}

// Register enqueues a workflow job through the default engine. It is the
// business-facing API: domain services call it to register a job and never
// hold an engine reference. It never blocks on execution.
func Register(ctx context.Context, job Job) (*Entity, error) {
	if defaultEngine == nil {
		return nil, ErrEngineNotInstalled
	}
	return defaultEngine.RegisterJob(ctx, job)
}

// RegisterJob enqueues a job for the named workflow. It resolves the first
// step from the registered definition, so it fails fast when the workflow has
// not been registered (instead of writing a row the executor cannot run).
func (e *Engine) RegisterJob(ctx context.Context, job Job) (*Entity, error) {
	if strings.TrimSpace(job.TraceId()) == "" {
		return nil, ErrTraceIDRequired
	}

	def, ok := e.definition(job.WorkflowName())
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrWorkflowNotRegistered, job.WorkflowName())
	}

	payload, err := json.Marshal(job.Payload())
	if err != nil {
		return nil, fmt.Errorf("workflow %q: marshal payload: %w", job.WorkflowName(), err)
	}

	return e.store.Insert(ctx, job.WorkflowName(), job.TraceId(), payload, def.Steps[0].Name())
}
