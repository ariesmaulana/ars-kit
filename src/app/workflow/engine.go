package workflow

import (
	"context"
	"sync"
)

// Engine owns the registered workflow definitions and the worker pool.
// It stays small: definitions in, workers out.
type Engine struct {
	store       Store
	cfg         Config
	definitions map[string]*Definition
	workers     []*worker

	mu sync.RWMutex
}

// NewEngine creates an Engine with the given store and config. Zero-valued
// config fields fall back to DefaultConfig values.
func NewEngine(store Store, cfg Config) *Engine {
	def := DefaultConfig()
	if cfg.Workers <= 0 {
		cfg.Workers = def.Workers
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = def.PollInterval
	}
	if cfg.StaleTimeout <= 0 {
		cfg.StaleTimeout = def.StaleTimeout
	}
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = def.DrainTimeout
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = def.BatchSize
	}
	if cfg.StepTimeout <= 0 {
		cfg.StepTimeout = def.StepTimeout
	}

	e := &Engine{
		store:       store,
		cfg:         cfg,
		definitions: make(map[string]*Definition),
	}

	executor := NewExecutor(e, store)
	for i := 0; i < cfg.Workers; i++ {
		e.workers = append(e.workers, &worker{store: store, executor: executor, cfg: cfg})
	}
	return e
}

// Register adds workflow definitions to the engine. It panics on duplicate
// names or ill-formed definitions — startup misconfiguration should be loud.
// Definitions must be registered before Run and before RegisterJob is called.
func (e *Engine) Register(defs ...*Definition) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range defs {
		d.validate()
		if _, exists := e.definitions[d.Name]; exists {
			panic("workflow: definition already registered: " + d.Name)
		}
		e.definitions[d.Name] = d
	}
}

// definition returns the registered definition for name.
func (e *Engine) definition(name string) (*Definition, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	d, ok := e.definitions[name]
	return d, ok
}

// Run starts every worker and blocks until ctx is cancelled and all workers
// have stopped. On cancellation, workers stop acquiring new jobs; the
// currently-executing step (run on a background context) is allowed to finish.
func (e *Engine) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, w := range e.workers {
		wg.Add(1)
		go w.Run(ctx, &wg)
	}
	wg.Wait()
}
