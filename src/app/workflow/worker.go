package workflow

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// worker is a scheduler: acquire a batch, execute each job, sleep only when the
// queue is empty. It contains almost no business logic and knows nothing about
// retries, idempotency, payload mutation, or step lookup — that is the
// Executor's job.
type worker struct {
	store    Store
	executor *Executor
	cfg      Config
}

// Run polls until ctx is cancelled. Jobs acquired before cancellation are
// executed on a background context so they finish even mid-shutdown; only the
// next poll is skipped.
//
// The loop is a hot loop: after draining a non-empty batch it acquires again
// immediately, so step-to-step latency is not bound by PollInterval. It only
// sleeps PollInterval when an acquire comes back empty, which keeps the loop
// self-limiting under load.
func (w *worker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		batch, err := w.store.AcquireBatch(ctx, w.cfg.StaleTimeout, w.cfg.BatchSize)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Debug().Err(err).Msg("workflow: failed to acquire next jobs")
		}

		if len(batch) == 0 {
			// Nothing queued: wait for the next poll.
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			continue
		}

		for _, job := range batch {
			// Deliberately background context: once a job is acquired, let it
			// finish even if the parent ctx is cancelled mid-step.
			w.executor.Execute(context.Background(), job)
		}
	}
}
