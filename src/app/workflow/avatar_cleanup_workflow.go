package workflow

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// Domain seam
// ============================================================================

// AvatarDeleter removes a stored avatar object by key. It is satisfied by the
// upload module's Uploader implementation.
type AvatarDeleter interface {
	Delete(ctx context.Context, key string) error
}

// ============================================================================
// AvatarCleanup Workflow
// ============================================================================

// AvatarCleanupWorkflow creates the workflow definition for deleting an old
// avatar file after a replacement is uploaded. The job is fire-and-forget:
// a failed delete is retried up to MaxRetries times, then the orphaned file
// is dropped by object storage lifecycle rules.
func AvatarCleanupWorkflow(deleter AvatarDeleter) *Definition {
	w := &avatarCleanupWorkflow{deleter: deleter}

	return &Definition{
		Name:       "avatar_cleanup",
		MaxRetries: 3,

		NewPayload: func() any {
			return &DeleteAvatarPayload{}
		},

		Steps: []Step{
			StepFunc("DeleteAvatar", w.DeleteAvatar),
		},
	}
}

// ============================================================================
// Job type
// ============================================================================

// DeleteAvatarJob is the business-facing job type for the avatar_cleanup
// workflow. Delete is idempotent (NotFound = nil), so re-runs are harmless.
type DeleteAvatarJob struct {
	traceId string
	payload DeleteAvatarPayload
}

// NewDeleteAvatarJob builds a DeleteAvatarJob for a stale avatar key.
func NewDeleteAvatarJob(traceId, key string) DeleteAvatarJob {
	return DeleteAvatarJob{traceId: traceId, payload: DeleteAvatarPayload{Key: key}}
}

func (DeleteAvatarJob) WorkflowName() string { return "avatar_cleanup" }
func (j DeleteAvatarJob) TraceId() string    { return j.traceId }
func (j DeleteAvatarJob) Payload() any       { return j.payload }

// ============================================================================
// Payload
// ============================================================================

// DeleteAvatarPayload carries the stale avatar key through the workflow.
type DeleteAvatarPayload struct {
	Key string
}

// ============================================================================
// Step
// ============================================================================

type avatarCleanupWorkflow struct {
	deleter AvatarDeleter
}

// DeleteAvatar removes the stale avatar object. The request path never blocks
// on it; the worker does the delete out-of-band.
func (w *avatarCleanupWorkflow) DeleteAvatar(ctx context.Context, run *Run) error {
	payload, ok := run.Payload.(*DeleteAvatarPayload)
	if !ok {
		return errors.New("invalid avatar_cleanup workflow payload")
	}

	if err := w.deleter.Delete(ctx, payload.Key); err != nil {
		log.Err(err).
			Str("trace_id", run.TraceID).
			Str("key", payload.Key).
			Msg("workflow avatar_cleanup: delete failed")
		return err
	}

	log.Info().
		Str("trace_id", run.TraceID).
		Str("key", payload.Key).
		Msg("workflow avatar_cleanup: deleted")
	return nil
}