package workflow

import (
	"context"
	"fmt"
)

// Step is a single unit of work in a workflow.
type Step interface {
	// Name must be stable across deploys. It is persisted as
	// workflow_job.current_step and used to resume a workflow after a crash.
	Name() string

	// Run may mutate the payload in place via run.Payload (type-asserted to
	// the workflow's concrete payload struct). Run must never touch the Store:
	// persistence is exclusively the Executor's job.
	Run(ctx context.Context, run *Run) error
}

// StepFunc adapts a plain function (e.g. a method value like
// w.RegisterUser) into a Step with an explicit, stable name. Method values
// are convenient for workflows that hold their dependencies on a struct.
func StepFunc(name string, fn func(ctx context.Context, run *Run) error) Step {
	return stepFunc{name: name, fn: fn}
}

type stepFunc struct {
	name string
	fn   func(ctx context.Context, run *Run) error
}

func (s stepFunc) Name() string { return s.name }

func (s stepFunc) Run(ctx context.Context, run *Run) error { return s.fn(ctx, run) }

// Definition describes one workflow: its name, retry policy, and ordered steps.
type Definition struct {
	// Name is the stable workflow identifier, persisted as
	// workflow_job.workflow_name. It must match NewJob.WorkflowName.
	Name string

	// MaxRetries is the number of times a failed step is re-run before the job
	// is marked failed. Total attempts for a step = MaxRetries + 1.
	MaxRetries int

	// Steps are executed in order. current_step advances one step at a time.
	Steps []Step

	// NewPayload returns a pointer to a zero-value payload struct for this
	// workflow. The Executor calls it once per execution to know what concrete
	// type to unmarshal the persisted JSONB payload into.
	NewPayload func() any
}

// validate panics on definitions that cannot run safely. It runs at startup
// registration time so misconfiguration fails loudly.
func (d *Definition) validate() {
	if d.Name == "" {
		panic("workflow: definition name must not be empty")
	}
	if d.NewPayload == nil {
		panic(fmt.Sprintf("workflow: definition %q has no NewPayload factory", d.Name))
	}
	if len(d.Steps) == 0 {
		panic(fmt.Sprintf("workflow: definition %q has no steps", d.Name))
	}
	seen := make(map[string]struct{}, len(d.Steps))
	for _, s := range d.Steps {
		if s == nil {
			panic(fmt.Sprintf("workflow: definition %q contains a nil step", d.Name))
		}
		if _, ok := seen[s.Name()]; ok {
			panic(fmt.Sprintf("workflow: definition %q has duplicate step name %q", d.Name, s.Name()))
		}
		seen[s.Name()] = struct{}{}
	}
}
