package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// Executor is the core of the workflow engine and the only component allowed
// to modify workflow execution state. Almost all workflow logic lives here.
type Executor struct {
	engine *Engine
	store  Store
}

// NewExecutor creates an Executor bound to an engine's definition registry.
func NewExecutor(engine *Engine, store Store) *Executor {
	return &Executor{engine: engine, store: store}
}

// Execute runs the job's current step and persists the outcome. On success the
// mutated payload is persisted atomically with the step advance/completion; on
// failure the payload column is left untouched (retry or fail). The step runs
// on the provided ctx — workers pass a background context so an in-flight step
// survives engine shutdown.
func (ex *Executor) Execute(ctx context.Context, entity *Entity) {
	def, ok := ex.engine.definition(entity.WorkflowName)
	if !ok {
		ex.fail(ctx, entity, fmt.Errorf("workflow %q not registered", entity.WorkflowName))
		return
	}

	step := findStep(def, entity.CurrentStep)
	if step == nil {
		ex.fail(ctx, entity, fmt.Errorf(
			"current_step %q not found in workflow %q definition (%d steps: %s)",
			entity.CurrentStep, def.Name, len(def.Steps), stepNames(def),
		))
		return
	}

	payload := def.NewPayload()
	if err := json.Unmarshal(entity.Payload, payload); err != nil {
		ex.fail(ctx, entity, fmt.Errorf("workflow %q: unmarshal payload: %w", def.Name, err))
		return
	}

	run := &Run{
		WorkflowID:  entity.ID,
		TraceID:     entity.TraceID,
		Payload:     payload,
		CurrentStep: entity.CurrentStep,
		RetryCount:  entity.RetryCount,
	}

	log.Debug().
		Int64("job_id", entity.ID).
		Str("workflow_name", def.Name).
		Str("trace_id", entity.TraceID).
		Str("step", step.Name()).
		Int("retry_count", entity.RetryCount).
		Msg("workflow: executing step")

	if err := ex.runStep(ctx, step, run); err != nil {
		log.Warn().
			Int64("job_id", entity.ID).
			Str("workflow_name", def.Name).
			Str("trace_id", entity.TraceID).
			Str("step", step.Name()).
			Err(err).
			Msg("workflow: step failed")
		ex.handleStepFailure(ctx, entity, def, err)
		return
	}

	raw, err := json.Marshal(run.Payload)
	if err != nil {
		ex.fail(ctx, entity, fmt.Errorf("workflow %q: marshal payload after step %q: %w", def.Name, step.Name(), err))
		return
	}

	if next := nextStep(def, entity.CurrentStep); next != nil {
		log.Debug().
			Int64("job_id", entity.ID).
			Str("workflow_name", def.Name).
			Str("trace_id", entity.TraceID).
			Str("step", step.Name()).
			Str("next_step", next.Name()).
			Msg("workflow: step completed, advancing")
		if err := ex.store.AdvanceStep(ctx, entity.ID, raw, next.Name()); err != nil {
			log.Error().Err(err).Int64("job_id", entity.ID).Msg("workflow: failed to advance step")
		}
		return
	}

	log.Debug().
		Int64("job_id", entity.ID).
		Str("workflow_name", def.Name).
		Str("trace_id", entity.TraceID).
		Str("step", step.Name()).
		Msg("workflow: final step completed")
	if err := ex.store.Complete(ctx, entity.ID, raw); err != nil {
		log.Error().Err(err).Int64("job_id", entity.ID).Msg("workflow: failed to complete job")
	}
}

// handleStepFailure applies the retry policy: re-run the same step up to
// MaxRetries times (total attempts = MaxRetries + 1), then fail the job.
// The payload column is never touched on failure — a failed attempt's
// in-memory mutations are discarded, and the step re-runs against the last
// successfully persisted payload on the next poll.
func (ex *Executor) handleStepFailure(ctx context.Context, entity *Entity, def *Definition, stepErr error) {
	nextRetry := entity.RetryCount + 1
	if nextRetry <= def.MaxRetries {
		if err := ex.store.UpdateRetry(ctx, entity.ID, nextRetry, stepErr.Error()); err != nil {
			log.Error().Err(err).Int64("job_id", entity.ID).Msg("workflow: failed to record retry")
		}
		return
	}
	ex.fail(ctx, entity, stepErr)
}

func (ex *Executor) fail(ctx context.Context, entity *Entity, err error) {
	if err := ex.store.Fail(ctx, entity.ID, err.Error()); err != nil {
		log.Error().Err(err).Int64("job_id", entity.ID).Msg("workflow: failed to record failure")
	}
}

// runStep executes one step, converting panics into errors so a buggy step
// fails through the normal retry/fail path instead of killing the worker.
func (ex *Executor) runStep(ctx context.Context, step Step, run *Run) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("step %q panicked: %v", step.Name(), r)
		}
	}()
	return step.Run(ctx, run)
}

func findStep(def *Definition, name string) Step {
	for _, s := range def.Steps {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func nextStep(def *Definition, current string) Step {
	for i, s := range def.Steps {
		if s.Name() == current && i+1 < len(def.Steps) {
			return def.Steps[i+1]
		}
	}
	return nil
}

func stepNames(def *Definition) string {
	names := make([]string, 0, len(def.Steps))
	for _, s := range def.Steps {
		names = append(names, s.Name())
	}
	return strings.Join(names, ", ")
}
