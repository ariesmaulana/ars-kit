# Workflow Engine

A lightweight, opinionated workflow engine for long-running, multi-step
background processes (checkout, user registration, refund, inventory sync, …).
PostgreSQL is the queue; one workflow lives in one file; the engine only
orchestrates — domain services own the business logic.

This document describes the **current implementation** in
`src/app/workflow/`. It does not describe a planned framework: the engine is
deliberately not a generic workflow framework.

---

## How it runs

The binary has two subcommands (same codebase, separate processes):

| Command | What it does |
|---|---|
| `./ars-kit serve` (or `make run`) | Runs the HTTP server. Business services **enqueue** workflow jobs here via `workflow.Register`. |
| `./ars-kit worker` (or `make worker`) | Runs the workflow engine **workers**. They poll `workflow_job` and execute jobs enqueued by `serve` (or any other process). |

Both share the same PostgreSQL database — `workflow_job` is the queue between
them.

```bash
# terminal 1
go run src/main.go serve

# terminal 2
go run src/main.go worker
```

Shutdown: SIGTERM/`Ctrl-C` on `worker` stops workers from acquiring new jobs,
lets the in-flight step finish (bounded by `WORKFLOW_DRAIN_TIMEOUT`), then
exits. Anything left `processing` is reclaimed by the next `worker` start via
stale reclaim (see below) — no operator intervention.

---

## Core concepts

### Workflow Definition

```go
type Definition struct {
	Name       string
	MaxRetries int
	Steps      []Step
	NewPayload func() any // returns a pointer to a zero-value payload struct
}
```

- `Name` is the stable workflow identifier, persisted as
  `workflow_job.workflow_name`.
- `Steps` run in order. `current_step` is a step **name** (string), never an
  index.
- `NewPayload` is a hand-written factory so the Executor knows the concrete Go
  type to unmarshal the persisted JSONB payload into. No reflection — a plain
  function per workflow.
- `MaxRetries` is the number of **re-runs** after a failure (total attempts =
  `MaxRetries + 1`).

### Step

```go
type Step interface {
	Name() string                       // stable across deploys; persisted as current_step
	Run(ctx context.Context, run *Run) error
}
```

A step may be any type implementing `Step`, or a plain function adapted with
`StepFunc(name, fn)` (used when steps are method values on a workflow struct).
A step:

- validates the payload,
- calls a domain service through an interface,
- mutates the payload **in memory only**,
- returns `nil` / `error`.

A step **never** touches `workflow_job` — persistence is exclusively the
Executor's job.

### Run

```go
type Run struct {
	WorkflowID  int64
	TraceID     string
	Payload     any // pointer to the workflow's concrete payload struct
	CurrentStep string
	RetryCount  int
}
```

No business logic.

### Mutable payload

The payload is the workflow's **execution state**, not read-only input. Every
step may enrich it; later steps — and a worker resuming after a crash — reuse
the persisted payload instead of re-querying a service.

Initial payload at registration:

```json
{ "Email": "jane@example.com", "Username": "janedoe", "UserID": 0 }
```

After `RegisterUser` step (persisted atomically with the advance):

```json
{ "Email": "jane@example.com", "Username": "janedoe", "UserID": 13 }
```

## Payload persistence rule

Payload is persisted **only when a step returns success**. A failed attempt's
in-memory mutations are discarded — never marshaled, never written to
`workflow_job.payload`. On retry the step re-runs against the last
**successfully persisted** payload. This is what makes step idempotency
sufficient: a re-run sees the same inputs, and the step/service can recognize
"already done" (e.g. `UserID != 0`, or a unique constraint) and skip the side
effect.

---

## Registering jobs (business side)

Business services enqueue jobs via the package-level API — no engine reference
needed:

```go
err := workflow.Register(ctx, workflow.NewRegisterDemoWorkflow(traceID, workflow.DemoWorkflowInput{
	Email:    input.Email,
	Username: input.Username,
}))
```

- `workflow.Register` uses an engine installed at bootstrap with
  `workflow.SetDefault(engine)`.
- Registration is **idempotent per logical action**: the same
  `(workflow_name, trace_id)` pair returns the existing row (upsert, unique
  constraint) instead of inserting a duplicate. `trace_id` must uniquely
  identify the logical business action, not a fresh id per request.
- Registration never blocks on execution and never resolves the first step
  itself — the engine fails fast (error, no row written) if the workflow
  definition is not registered.

Job types implement the `Job` interface:

```go
type Job interface {
	WorkflowName() string
	TraceId() string
	Payload() any
}
```

Each workflow file ships its own job type + constructor (e.g.
`RegisterDemoWorkflow` + `NewRegisterDemoWorkflow`).

---

## The demo workflow (end to end)

`src/app/workflow/demo_workflow.go` — the reference example. The user module's
service is the domain seam (`workflow.UserService` interface), satisfied by
`user.Service`.

```go
func DemoWorkflow(userService UserService) *Definition {
	w := &demoWorkflow{userService: userService}

	return &Definition{
		Name:       "demo",
		MaxRetries: 3,
		NewPayload: func() any { return &DemoWorkflowInput{} },
		Steps: []Step{
			StepFunc("RegisterUser", w.RegisterUser),
			StepFunc("GrantPermission", w.GrantPermission),
		},
	}
}
```

- **Step 1 `RegisterUser`** — validates the payload, checks idempotency
  (`UserID != 0` → already done, return nil), calls
  `userService.RegisterUser`, stores `payload.UserID`.
- **Step 2 `GrantPermission`** — validates `UserID`, calls
  `userService.GrantPermissionSystem(..., "default")`, which grants the
  `default` permission to the new user.

The user module exposes the seams the workflow needs:

- `user.DemoWorkflow(ctx, input)` — the handler-facing method that just calls
  `workflow.Register(...)`.
- `user.RegisterUser(ctx, *workflow.RegisterUserInput)` — creates the user,
  idempotent on unique constraint (returns the existing user on re-run).
- `user.GrantPermissionSystem(ctx, *workflow.GrantPermissionInput)` — grants
  without the super-user actor check, returning a typed `Success`/`Message`/
  `ErrorCode` output like the rest of the service contract. It is **not** the
  admin `user.GrantPermission` (actor-gated, backs the admin endpoint) — a
  background step has no actor.

Trigger it:

```bash
curl -X POST http://localhost:8080/api/v1/users/register-workflow \
     -H 'Content-Type: application/json' \
     -d '{"username":"wf_jane","email":"wf_jane@example.com","full_name":"Jane Workflow","password":"supersecret"}'
```

→ `202 {"success":true,"message":"Demo workflow queued"}` — the `serve`
process enqueues; the `worker` process executes.

---

## Lifecycle & persistence

### Table

```sql
CREATE TABLE workflow_job (
    id            bigserial PRIMARY KEY,
    workflow_name varchar     NOT NULL,
    trace_id      varchar     NOT NULL,
    payload       jsonb       NOT NULL,
    status        varchar     NOT NULL DEFAULT 'waiting',
    current_step  varchar     NOT NULL,
    retry_count   int         NOT NULL DEFAULT 0,
    last_error    text,
    locked_at     timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_workflow_job_name_trace UNIQUE (workflow_name, trace_id)
);

CREATE INDEX idx_workflow_job_status ON workflow_job (status, updated_at);
```

Statuses: `waiting` → `processing` → `waiting` (next step) / `done` /
`failed`.

Migration lives in `src/app/workflow/sql/` (goose, registered in
`database.All`): `make migrate-up workflow`.

### Store

```go
type Store interface {
	Insert(ctx, workflowName, traceID string, payload json.RawMessage, currentStep string) (*Entity, error)
	AcquireBatch(ctx, staleTimeout time.Duration, limit int) ([]*Entity, error)
	AdvanceStep(ctx, id int64, payload json.RawMessage, nextStep string) error
	Complete(ctx, id int64, payload json.RawMessage) error
	UpdateRetry(ctx, id int64, retryCount int, lastErr string) error // never touches payload
	Fail(ctx, id int64, lastErr string) error                        // never touches payload
}
```

The Store only handles `json.RawMessage` bytes — it never unmarshals payloads
into concrete types (that is exclusively the Executor's job, via
`Definition.NewPayload`).

### Atomicity

Payload + step advance (or completion) are persisted in **one** SQL
statement:

```sql
-- success, more steps remain
UPDATE workflow_job SET payload = $2, current_step = $3, status = 'waiting',
       retry_count = 0, updated_at = now() WHERE id = $1;

-- success, last step
UPDATE workflow_job SET payload = $2, status = 'done', updated_at = now() WHERE id = $1;
```

`retry_count` resets on a successful advance — retries are scoped to a single
step, not the whole workflow.

### Retry rule

```
step fails → retry_count + 1
             ├─ retry_count <= MaxRetries → status = waiting (same step, last-known-good payload)
             └─ otherwise                → status = failed (last_error set)
```

No delayed/backoff retry: `PollInterval` is the de facto retry delay. Retry
never restarts the workflow.

### Stale reclaim (crash recovery)

There is **no reaper process**. The acquire query treats a `processing` row
older than `StaleTimeout` as re-acquirable, in the same query as picking up
`waiting` rows (`FOR UPDATE SKIP LOCKED`). A crashed worker's jobs are
naturally picked up on the next poll by any live worker. This is why step
idempotency is mandatory: two workers can legitimately execute the same step if
the first was slow rather than crashed. `StaleTimeout` must exceed the
worst-case batch drain (`BatchSize × slowest step`) or a legitimately-draining
batch can be re-acquired.

---

## Worker lifecycle

```
worker start → acquire batch → execute each job → acquire again if batch was
non-empty → sleep PollInterval only when the queue is empty → …
```

The loop is a **hot loop**: after draining a non-empty batch the worker acquires
again immediately, so step-to-step latency is not bound by `PollInterval` — it
only sleeps when an acquire comes back empty, which keeps it self-limiting
under load. Each job still executes synchronously on a background context. On
`ctx` cancellation the worker stops acquiring; the currently-executing step
runs to completion.

Acquire query (batch + stale reclaim inline):

```sql
UPDATE workflow_job
SET status = 'processing', locked_at = now(), updated_at = now()
WHERE id = (
    SELECT id FROM workflow_job
    WHERE status = 'waiting'
       OR (status = 'processing' AND locked_at < now() - $1::interval)
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT $2
)
RETURNING id, workflow_name, trace_id, payload, current_step, retry_count, last_error, locked_at, created_at, updated_at;
```

---

## Executor

The Executor is the core and the only component allowed to modify workflow
execution state:

1. resolve the definition by name (missing → **fail loudly**, `status = failed`)
2. resolve `current_step` in `Definition.Steps` (missing → fail loudly with the
   step list in `last_error`)
3. unmarshal the persisted payload via `NewPayload` (failure → fail loudly, no
   panic)
4. run the step — panics are recovered and converted into step errors
5. success → marshal the mutated payload, atomic `AdvanceStep` / `Complete`
6. failure → `UpdateRetry` / `Fail` — payload column untouched

The "fail loudly" paths exist for **definition drift across deploys**: if a
deploy renames/removes a step or changes a payload struct while jobs are
in-flight, the job becomes a visible `failed` row instead of being silently
stuck. Operators drain in-flight jobs before breaking deploys.

---

## Configuration

```go
type Config struct {
	Workers      int           // default 3
	PollInterval time.Duration // default 5s — idle sleep when queue is empty
	StaleTimeout time.Duration // default 3m — must exceed BatchSize × slowest step
	DrainTimeout time.Duration // default 30s — shutdown drain wait
	BatchSize    int           // default 5 — jobs claimed per poll
}
```

Env vars (loaded in `config/config.go`, applied in `main.go` → `buildApp`):

| Env var | Unit | Default |
|---|---|---|
| `WORKFLOW_WORKERS` | workers | 3 |
| `WORKFLOW_POLL_INTERVAL` | seconds | 5 |
| `WORKFLOW_STALE_TIMEOUT` | minutes | 3 |
| `WORKFLOW_DRAIN_TIMEOUT` | seconds | 30 |
| `WORKFLOW_BATCH_SIZE` | jobs per poll | 5 |

`StaleTimeout` must be comfortably larger than the slowest expected step
duration, otherwise a second worker may pick up a job whose first worker is
still legitimately running.

---

## Failure visibility

No dead-letter table, no alerting. Failed and stuck jobs are visible via
direct SQL:

```sql
-- exhausted retries
SELECT id, workflow_name, trace_id, current_step, retry_count, last_error, updated_at
FROM workflow_job WHERE status = 'failed' ORDER BY updated_at DESC;

-- stuck in processing longer than StaleTimeout allows
SELECT id, workflow_name, trace_id, current_step, locked_at
FROM workflow_job WHERE status = 'processing'
  AND locked_at < now() - interval '5 minutes' ORDER BY locked_at ASC;
```

---

## Writing a new workflow

1. **Create one file** in `src/app/workflow/`, e.g. `refund_workflow.go`.
2. Define the **payload struct** (exported fields, `json` tags; zero-valued
   "later" fields must serialize as JSON zeros — no `omitempty`).
3. Define the **domain seam interfaces** the steps need (satisfied
   structurally by the domain module's service).
4. Define the **steps** (types implementing `Step`, or `StepFunc(name, fn)` on
   a workflow struct holding the services).
5. Define the **job type** (implements `Job`) + a constructor.
6. Build the **`Definition`** (`Name`, `MaxRetries`, `NewPayload`, `Steps`).
7. In `main.go` → `buildApp`: `engine.Register(workflow.MyWorkflow(service))`
   (must happen before `workflow.SetDefault`/`Run`).
8. Business code enqueues with `workflow.Register(ctx, workflow.NewMyJob(...))`.

Keep steps idempotent — re-execution is normal (retry + stale reclaim).

---

## Non-goals (deliberately not implemented)

- Parallel steps, saga rollback, compensation
- Delayed/backoff retry (poll interval is the retry delay)
- Priority queue, cron, distributed workers
- Event bus, CQRS, reflection, dependency-injection framework
- Dead-letter table / alerting (query the table directly)
- Workflow definition versioning / payload schema migration
- A separate stale-job reaper process (stale reclaim is inline)
- Steps writing to `workflow_job` directly (Executor only)

## Design rules

- **One workflow = one file.**
- **PostgreSQL is the queue** — shared `pgxpool.Pool`, no dedicated pool.
- **Workflow never owns a transaction** — every domain service manages its own.
- **The workflow engine never accesses another module's storage** — only
  through services/interfaces.
- **Every step must be idempotent** (retry + stale reclaim can run the same
  step twice for the same job).
- **Payload is mutable execution state**, persisted only on step success,
  atomically with the step advance.
