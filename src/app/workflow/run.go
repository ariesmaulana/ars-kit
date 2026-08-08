package workflow

// Run carries the execution state handed to each Step. It contains no
// business logic.
type Run struct {
	WorkflowID  int64
	TraceID     string
	Payload     any // pointer to the workflow's concrete payload struct
	CurrentStep string
	RetryCount  int
}
