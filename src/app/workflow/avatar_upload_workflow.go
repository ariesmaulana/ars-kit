package workflow

import (
	"context"
	"errors"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/ariesmaulana/ars-kit/src/app/upload"
)

// ============================================================================
// Domain seams
// ============================================================================

// AvatarUploader persists avatar files and removes stale ones. It is
// satisfied by the upload module's Uploader implementation.
type AvatarUploader interface {
	Upload(ctx context.Context, req upload.UploadRequest) (*upload.UploadResult, error)
	Delete(ctx context.Context, key string) error
}

// AvatarKeyUpdater persists a user's avatar key. It is satisfied by an
// adapter over the user module's Storage (BeginTx → UpdateAvatarKey →
// Commit). The workflow package defines it here so it never imports the
// user package (user already imports workflow).
type AvatarKeyUpdater interface {
	UpdateAvatarKey(ctx context.Context, userID int, avatarKey string) error
}

// ============================================================================
// AvatarUpload Workflow
// ============================================================================

// AvatarUploadWorkflow creates the workflow definition for persisting a
// staged avatar file. The serve process spools the multipart bytes to a
// local staging file (fast, no network) and enqueues this job; the worker
// does the slow object-store PUT out-of-band.
//
// Step 1 (UploadFile) reads the staged file, uploads it under the
// service-minted key, and persists users.avatar_key. Step 2 (Cleanup)
// deletes the old avatar object and the staged temp file.
func AvatarUploadWorkflow(uploader AvatarUploader, keyUpdater AvatarKeyUpdater) *Definition {
	w := &avatarUploadWorkflow{uploader: uploader, keyUpdater: keyUpdater}

	return &Definition{
		Name:       "avatar_upload",
		MaxRetries: 3,

		NewPayload: func() any {
			return &AvatarUploadPayload{}
		},

		Steps: []Step{
			StepFunc("UploadFile", w.UploadFile),
			StepFunc("Cleanup", w.Cleanup),
		},
	}
}

// ============================================================================
// Job type
// ============================================================================

// AvatarUploadJob is the business-facing job type for the avatar_upload
// workflow.
type AvatarUploadJob struct {
	traceId string
	payload AvatarUploadPayload
}

// NewAvatarUploadJob builds an AvatarUploadJob. Key is minted by the service
// (xid + extension) so request-time reads (old key) and the worker share one
// stable storage key across retries. StagedPath is the absolute path of the
// locally spooled file; OldKey is the previous avatar key ("" when none).
func NewAvatarUploadJob(traceId string, payload AvatarUploadPayload) AvatarUploadJob {
	return AvatarUploadJob{traceId: traceId, payload: payload}
}

func (AvatarUploadJob) WorkflowName() string { return "avatar_upload" }
func (j AvatarUploadJob) TraceId() string    { return j.traceId }
func (j AvatarUploadJob) Payload() any       { return j.payload }

// ============================================================================
// Payload
// ============================================================================

// AvatarUploadPayload carries the staged upload through the workflow system.
// MIME and Size are filled in by the UploadFile step from the upload result.
type AvatarUploadPayload struct {
	UserID     int
	Key        string
	StagedPath string
	Filename   string
	SizeHint   int64
	OldKey     string
	MIME       string
	Size       int64
}

// ============================================================================
// Steps
// ============================================================================

type avatarUploadWorkflow struct {
	uploader   AvatarUploader
	keyUpdater AvatarKeyUpdater
}

// UploadFile reads the staged file, persists it to object storage under the
// service-minted key, and records the key on the user row. On failure the
// payload is untouched (executor discards in-memory mutations), so a retry
// re-reads the same staged file and re-runs the whole step.
func (w *avatarUploadWorkflow) UploadFile(ctx context.Context, run *Run) error {
	payload, ok := run.Payload.(*AvatarUploadPayload)
	if !ok {
		return errors.New("invalid avatar_upload workflow payload")
	}

	f, err := os.Open(payload.StagedPath)
	if err != nil {
		log.Err(err).
			Str("trace_id", run.TraceID).
			Str("staged_path", payload.StagedPath).
			Msg("workflow avatar_upload: cannot open staged file")
		return err
	}
	defer f.Close()

	result, err := w.uploader.Upload(ctx, upload.UploadRequest{
		Reader:      f,
		Filename:    payload.Filename,
		SizeHint:    payload.SizeHint,
		KeyOverride: payload.Key,
	})
	if err != nil {
		log.Err(err).
			Str("trace_id", run.TraceID).
			Str("key", payload.Key).
			Msg("workflow avatar_upload: upload failed")
		return err
	}

	if err := w.keyUpdater.UpdateAvatarKey(ctx, payload.UserID, result.Key); err != nil {
		log.Err(err).
			Str("trace_id", run.TraceID).
			Int("user_id", payload.UserID).
			Str("key", result.Key).
			Msg("workflow avatar_upload: failed to persist avatar key")
		return err
	}

	payload.MIME = result.MIME
	payload.Size = result.Size

	log.Info().
		Str("trace_id", run.TraceID).
		Int("user_id", payload.UserID).
		Str("key", result.Key).
		Msg("workflow avatar_upload: uploaded")
	return nil
}

// Cleanup removes the old avatar object and the staged temp file. Both
// deletes are idempotent (missing object/file = nil), so re-runs are
// harmless. Failures return an error so the step is retried; a permanently
// failed cleanup only leaks disk/object bytes, never corrupts user data.
func (w *avatarUploadWorkflow) Cleanup(ctx context.Context, run *Run) error {
	payload, ok := run.Payload.(*AvatarUploadPayload)
	if !ok {
		return errors.New("invalid avatar_upload workflow payload")
	}

	if payload.OldKey != "" && payload.OldKey != payload.Key {
		if err := w.uploader.Delete(ctx, payload.OldKey); err != nil {
			log.Err(err).
				Str("trace_id", run.TraceID).
				Str("key", payload.OldKey).
				Msg("workflow avatar_upload: old avatar delete failed")
			return err
		}
	}

	if err := os.Remove(payload.StagedPath); err != nil && !os.IsNotExist(err) {
		log.Err(err).
			Str("trace_id", run.TraceID).
			Str("staged_path", payload.StagedPath).
			Msg("workflow avatar_upload: staged file delete failed")
		return err
	}

	log.Info().
		Str("trace_id", run.TraceID).
		Str("key", payload.Key).
		Msg("workflow avatar_upload: cleaned up")
	return nil
}
