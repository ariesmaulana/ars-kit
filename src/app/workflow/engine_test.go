package workflow_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	"github.com/stretchr/testify/assert"
)

// dummyStep is a configurable Step for definition-level tests.
type dummyStep struct {
	name string
	fn   func(ctx context.Context, run *workflow.Run) error
}

func (s *dummyStep) Name() string { return s.name }

func (s *dummyStep) Run(ctx context.Context, run *workflow.Run) error {
	if s.fn != nil {
		return s.fn(ctx, run)
	}
	return nil
}

// demoDefinition builds a small two-step workflow standing in for domain
// workflows (SendEmail/AvatarCleanup) in engine/worker tests. It must stay
// self-contained: workflow is a foundation lib and cannot depend on domains.
func demoDefinition() *workflow.Definition {
	return &workflow.Definition{
		Name:       "demo",
		MaxRetries: 2,
		NewPayload: func() any { return &struct{}{} },
		Steps: []workflow.Step{
			&dummyStep{name: "First"},
			&dummyStep{name: "Second"},
		},
	}
}

func TestEngineRegisterPanicsOnDuplicateDefinition(t *testing.T) {
	engine := workflow.NewEngine(newFakeStore(), workflow.Config{})
	engine.Register(demoDefinition())

	assert.Panics(t, func() {
		engine.Register(demoDefinition())
	})
}

func TestEngineRegisterPanicsOnInvalidDefinition(t *testing.T) {
	engine := workflow.NewEngine(newFakeStore(), workflow.Config{})

	assert.Panics(t, func() {
		engine.Register(&workflow.Definition{Name: "no-steps"}) // no Steps, no NewPayload
	}, "a definition without steps must be rejected at registration")

	assert.Panics(t, func() {
		engine.Register(&workflow.Definition{
			Name:       "dup-steps",
			MaxRetries: 1,
			NewPayload: func() any { return &struct{}{} },
			Steps:      []workflow.Step{&dummyStep{name: "a"}, &dummyStep{name: "a"}},
		})
	}, "duplicate step names must be rejected at registration")
}
