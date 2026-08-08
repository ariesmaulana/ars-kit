package workflow

import (
	"encoding/json"
	"time"
)

// Entity is one persisted row of the workflow_job queue.
type Entity struct {
	ID           int64
	WorkflowName string
	TraceID      string
	Payload      json.RawMessage
	Status       Status
	CurrentStep  string
	RetryCount   int
	LastError    string
	LockedAt     *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
