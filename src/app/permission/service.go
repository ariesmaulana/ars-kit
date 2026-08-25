package permission

import (
	"context"
	"fmt"

	"github.com/ariesmaulana/ars-kit/src/clock"
	"github.com/rs/zerolog/log"
)

var _ Service = (*service)(nil)

// key builds the "<user_id>:<permission>" string stored in the database.
// Check, grant, and revoke all route through it so a granted permission is
// always exactly what a later check looks for.
func key(userID int, permission string) string {
	return fmt.Sprintf("%d:%s", userID, permission)
}

type service struct {
	storage Storage
}

func NewService(storage Storage) Service {
	return &service{storage: storage}
}

func (s *service) CheckPermission(ctx context.Context, input *CheckPermissionInput) *CheckPermissionOutput {
	resp := &CheckPermissionOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		return resp
	}
	if input.UserID == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to check permission"
		return resp
	}
	defer db.Rollback()

	has, err := db.HasPermission(ctx, input.UserID, key(input.UserID, input.Permission))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check permission")
		resp.Message = "Failed to check permission"
		return resp
	}

	// A user holding the "<user_id>:super_user" permission is allowed to do
	// anything: it acts as a wildcard for every other permission check.
	if !has {
		has, err = db.HasPermission(ctx, input.UserID, key(input.UserID, PermissionSuperUser))
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to check super user permission")
			resp.Message = "Failed to check permission"
			return resp
		}
	}

	resp.Success = true
	resp.Message = "Permission check completed"
	resp.HasPermission = has
	return resp
}

func (s *service) GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput {
	resp := &GrantPermissionOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		return resp
	}
	if input.UserID == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		return resp
	}

	// Policy guard: super_user is bootstrap-only. It is never granted
	// at runtime — not by admins, not by workflows — only seeded directly in
	// the database via SOP. Every grant flows through this method, so this is
	// the single choke point that makes the invariant structural.
	if input.Permission == PermissionSuperUser {
		log.Warn().Str("traceId", input.TraceId).Int("targetUserId", input.UserID).Msg("Rejected grant of super_user")
		resp.Message = "Cannot grant super_user"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to grant permission"
		return resp
	}
	defer db.Rollback()

	// FIX: we should using lock for this kind of operation, to avoid when some parallel request delete the permission
	known, err := db.PermissionExists(ctx, input.Permission)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check permission catalog")
		resp.Message = "Failed to grant permission"
		return resp
	}
	if !known {
		log.Warn().Str("permission", input.Permission).Str("traceId", input.TraceId).Msg("Unknown permission")
		resp.Message = "Unknown permission"
		return resp
	}

	err = db.AddPermission(ctx, input.UserID, key(input.UserID, input.Permission))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to grant permission")
		resp.Message = "Failed to grant permission"
		return resp
	}

	// Audit trail: who granted what, when. Written inside the same transaction
	// as the permission change, so a rolled-back grant never leaves an audit
	// row (and vice versa).
	err = db.InsertPermissionAudit(ctx, input.ActorId, input.UserID, input.Permission, AuditActionGrant, clock.Now())
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to write grant audit row")
		resp.Message = "Failed to grant permission"
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to grant permission"
		return resp
	}

	resp.Success = true
	resp.Message = "Permission granted successfully"
	return resp
}

func (s *service) RevokePermission(ctx context.Context, input *RevokePermissionInput) *RevokePermissionOutput {
	resp := &RevokePermissionOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		return resp
	}
	if input.UserID == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to revoke permission"
		return resp
	}
	defer db.Rollback()

	err = db.RemovePermission(ctx, input.UserID, key(input.UserID, input.Permission))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to revoke permission")
		resp.Message = "Failed to revoke permission"
		return resp
	}

	// Audit trail: who revoked what, when. Written inside the same transaction
	// as the permission change.
	err = db.InsertPermissionAudit(ctx, input.ActorId, input.UserID, input.Permission, AuditActionRevoke, clock.Now())
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to write revoke audit row")
		resp.Message = "Failed to revoke permission"
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to revoke permission"
		return resp
	}

	resp.Success = true
	resp.Message = "Permission revoked successfully"
	return resp
}
