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
}

func (s *dummyStep) Name() string { return s.name }

func (s *dummyStep) Run(ctx context.Context, run *workflow.Run) error { return nil }

func TestEngineRegisterPanicsOnDuplicateDefinition(t *testing.T) {
	engine := workflow.NewEngine(newFakeStore(), workflow.Config{})
	engine.Register(workflow.DemoWorkflow(nil))

	assert.Panics(t, func() {
		engine.Register(workflow.DemoWorkflow(nil))
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
