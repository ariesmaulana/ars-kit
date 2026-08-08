package workflow

import "time"

// Config controls the worker pool.
type Config struct {
	// Workers is the number of concurrent worker goroutines.
	Workers int

	// PollInterval is the idle sleep between polls when nothing is queued. It
	// is also the de facto retry delay: a failed step returns to 'waiting' and
	// is picked up on the next poll cycle. There is no separate backoff
	// scheduler.
	PollInterval time.Duration

	// StaleTimeout is how long a job may sit in 'processing' before another
	// worker may re-acquire it (stale reclaim happens inline in the acquire
	// query — there is no separate reaper process). It must be comfortably
	// larger than the worst-case drain of a batch: BatchSize × slowest step
	// duration, plus the current step.
	StaleTimeout time.Duration

	// DrainTimeout bounds how long a caller waits for in-flight jobs to finish
	// after cancelling the engine context during shutdown. Jobs still running
	// past the deadline are left 'processing' and reclaimed by the next
	// deployment's stale logic.
	DrainTimeout time.Duration

	// BatchSize is how many jobs a worker claims per poll. A worker that
	// drains a non-empty batch polls again immediately (hot loop); it only
	// sleeps PollInterval when nothing is queued.
	BatchSize int
}

// DefaultConfig returns the recommended production defaults.
func DefaultConfig() Config {
	return Config{
		Workers:      3,
		PollInterval: 5 * time.Second,
		StaleTimeout: 3 * time.Minute,
		DrainTimeout: 30 * time.Second,
		BatchSize:    5,
	}
}
