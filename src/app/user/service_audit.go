package user

import (
	"context"

	"github.com/rs/zerolog/log"
)

// writeAudit records a security-relevant event in the audit log. It runs in
// its own transaction so it can be called from flows whose primary action has
// already committed (login, grant, revoke). A failure is logged but never
// fails the primary operation: the audit trail must not be able to lock a
// user out of their account or undo a permission change.
func (s *service) writeAudit(ctx context.Context, traceId string, entry AuditEntry) error {
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", traceId).Str("event", entry.Event).Msg("Failed to begin audit transaction")
		return err
	}
	defer db.Rollback()

	if _, err := db.InsertAuditLog(ctx, entry); err != nil {
		log.Err(err).Str("traceId", traceId).Str("event", entry.Event).Msg("Failed to insert audit log")
		return err
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", traceId).Str("event", entry.Event).Msg("Failed to commit audit log")
		return err
	}
	return nil
}

// ListAuditLogs returns one page of audit log entries, newest first. Only an
// actor holding the "<actorId>:super_user" permission may read the log.
func (s *service) ListAuditLogs(ctx context.Context, input *ListAuditLogsInput) *ListAuditLogsOutput {
	resp := &ListAuditLogsOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.ActorId == 0 {
		log.Warn().Msg("Actor ID empty")
		resp.Message = "Actor ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Page < 1 {
		log.Warn().Msg("Page below 1")
		resp.Message = "Page must be at least 1"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		log.Warn().Msg("Page size out of range")
		resp.Message = "Page size must be between 1 and 100"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can read audit logs"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to list audit logs"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	filter := AuditLogFilter{
		Event:        input.Event,
		ActorId:      input.FilterActorId,
		TargetUserId: input.FilterTargetId,
		Page:         input.Page,
		PageSize:     input.PageSize,
	}

	total, err := db.CountAuditLogs(ctx, filter)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to count audit logs")
		resp.Message = "Failed to list audit logs"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	entries, err := db.ListAuditLogs(ctx, filter)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to list audit logs")
		resp.Message = "Failed to list audit logs"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to list audit logs"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("actorId", input.ActorId).
		Str("event", input.Event).
		Int("page", input.Page).
		Int("pageSize", input.PageSize).
		Int("total", total).
		Msg("Audit logs listed")

	resp.Success = true
	resp.Message = "Audit logs retrieved successfully"
	resp.Entries = entries
	resp.Page = input.Page
	resp.PageSize = input.PageSize
	resp.Total = total
	return resp
}
