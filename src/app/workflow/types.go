package workflow

import "errors"

// Status represents the lifecycle state of a workflow job row.
type Status string

const (
	// StatusWaiting means the job is queued and eligible to be picked up.
	StatusWaiting Status = "waiting"
	// StatusProcessing means a worker has acquired the job and is executing it.
	StatusProcessing Status = "processing"
	// StatusDone means every step completed successfully.
	StatusDone Status = "done"
	// StatusFailed means the job exhausted its retries or hit an
	// unrecoverable error, and will not be executed again.
	StatusFailed Status = "failed"
)

// Sentinel errors returned by RegisterJob.
var (
	// ErrTraceIDRequired is returned when a job is registered with an empty trace id.
	ErrTraceIDRequired = errors.New("workflow: trace_id is required")

	// ErrWorkflowNotRegistered is returned when a job is registered for a
	// workflow that has not been registered with the Engine.
	ErrWorkflowNotRegistered = errors.New("workflow: definition not registered")

	// ErrEngineNotInstalled is returned by the package-level Register when no
	// engine has been installed via SetDefault.
	ErrEngineNotInstalled = errors.New("workflow: engine not installed")
)
