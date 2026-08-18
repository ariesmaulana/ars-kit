package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
// failure the payload column is left untouched (retry or fail).
//
// Step-level errors — including a step exceeding StepTimeout — are handled
// internally through the retry/fail policy and return nil. Execute returns a
// non-nil error only when persisting the success outcome (AdvanceStep/Complete)
// fails: the step ran and mutated its in-memory payload, but that mutation can
// never reach the database, so the caller (the worker) must fail the job
// instead of leaving it 'processing' until stale reclaim.
//
// The step runs on a child of the provided ctx with a per-step timeout. Workers
// pass a background context so an in-flight step survives engine shutdown, and
// the timeout is layered on top of that rather than replacing it.
func (ex *Executor) Execute(ctx context.Context, entity *Entity) error {
	def, ok := ex.engine.definition(entity.WorkflowName)
	if !ok {
		ex.fail(ctx, entity, fmt.Errorf("workflow %q not registered", entity.WorkflowName))
		return nil
	}

	step := findStep(def, entity.CurrentStep)
	if step == nil {
		ex.fail(ctx, entity, fmt.Errorf(
			"current_step %q not found in workflow %q definition (%d steps: %s)",
			entity.CurrentStep, def.Name, len(def.Steps), stepNames(def),
		))
		return nil
	}

	payload := def.NewPayload()
	if err := json.Unmarshal(entity.Payload, payload); err != nil {
		ex.fail(ctx, entity, fmt.Errorf("workflow %q: unmarshal payload: %w", def.Name, err))
		return nil
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

	stepTimeout := ex.stepTimeout()
	stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()

	stepErr := ex.runStep(stepCtx, step, run)
	if stepCtx.Err() == context.DeadlineExceeded {
		// The deadline fired whether or not the step noticed. Normalise the
		// error so operators see a clear timeout signal instead of a bare
		// "context deadline exceeded" — persisting the outcome of a timed-out
		// step would be wrong, and retrying on a fresh payload is the safe path.
		if stepErr == nil {
			stepErr = fmt.Errorf("step returned nil after the deadline passed")
		}
		stepErr = fmt.Errorf("workflow %q: step %q exceeded timeout %s: %w", def.Name, step.Name(), stepTimeout, stepErr)
		log.Warn().
			Int64("job_id", entity.ID).
			Str("workflow_name", def.Name).
			Str("trace_id", entity.TraceID).
			Str("step", step.Name()).
			Err(stepErr).
			Msg("workflow: step exceeded timeout")
		ex.handleStepFailure(ctx, entity, def, stepErr)
		return nil
	}

	if stepErr != nil {
		log.Warn().
			Int64("job_id", entity.ID).
			Str("workflow_name", def.Name).
			Str("trace_id", entity.TraceID).
			Str("step", step.Name()).
			Err(stepErr).
			Msg("workflow: step failed")
		ex.handleStepFailure(ctx, entity, def, stepErr)
		return nil
	}

	raw, err := json.Marshal(run.Payload)
	if err != nil {
		ex.fail(ctx, entity, fmt.Errorf("workflow %q: marshal payload after step %q: %w", def.Name, step.Name(), err))
		return nil
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
			// The step succeeded in memory but its mutation was never persisted;
			// the job's state is inconsistent. Surface the error so the worker
			// fails the job instead of letting it linger 'processing'.
			return fmt.Errorf("workflow %q: advance step %q to %q: %w", def.Name, step.Name(), next.Name(), err)
		}
		return nil
	}

	log.Debug().
		Int64("job_id", entity.ID).
		Str("workflow_name", def.Name).
		Str("trace_id", entity.TraceID).
		Str("step", step.Name()).
		Msg("workflow: final step completed")
	if err := ex.store.Complete(ctx, entity.ID, raw); err != nil {
		return fmt.Errorf("workflow %q: complete final step %q: %w", def.Name, step.Name(), err)
	}
	return nil
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

// stepTimeout returns the per-step execution timeout from the engine config.
// The engine defaults it (60s) when zero, so a bare NewExecutor always has a
// sane bound.
func (ex *Executor) stepTimeout() time.Duration {
	return ex.engine.cfg.StepTimeout
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
