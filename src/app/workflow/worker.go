package workflow

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// worker is a scheduler: poll, acquire, execute, sleep. It contains almost no
// business logic and knows nothing about retries, idempotency, payload
// mutation, or step lookup — that is the Executor's job.
type worker struct {
	store    Store
	executor *Executor
	cfg      Config
}

// Run polls until ctx is cancelled. A job acquired before cancellation is
// executed on a background context so it finishes even mid-shutdown; only the
// next poll is skipped.
func (w *worker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		// Poll immediately on start, then once per tick.
		if !w.poll(ctx) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// poll acquires one job and executes it synchronously. It returns false when
// the context is cancelled so the caller can stop.
func (w *worker) poll(ctx context.Context) bool {
	job, err := w.store.AcquireNext(ctx, w.cfg.StaleTimeout)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		log.Debug().Err(err).Msg("workflow: failed to acquire next job")
		return true
	}
	if job == nil {
		return true
	}

	// Deliberately background context: once a job is acquired, let it finish
	// even if the parent ctx is cancelled mid-step.
	w.executor.Execute(context.Background(), job)
	return true
}
