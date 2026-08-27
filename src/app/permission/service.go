package permission

import (
	"context"

	"github.com/ariesmaulana/ars-kit/src/clock"
	"github.com/rs/zerolog/log"
)

var _ Service = (*service)(nil)

type service struct {
	storage Storage
}

func NewService(storage Storage) Service {
	return &service{storage: storage}
}

// CheckPermission reports whether the target user effectively holds the
// permission. Resolution order: one of the user's roles carries the
// permission, or the user holds the super_user role (wildcard).
func (s *service) CheckPermission(ctx context.Context, input *CheckPermissionInput) *CheckPermissionOutput {
	resp := &CheckPermissionOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission is mandatory")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to check permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	has, err := db.UserHasPermission(ctx, input.UserID, input.Permission)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check permission")
		resp.Message = "Failed to check permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// A user holding the super_user role is allowed to do anything: it acts
	// as a wildcard for every other permission check.
	if !has {
		has, err = db.UserHasRole(ctx, input.UserID, RoleSuperUser)
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to check super user role")
			resp.Message = "Failed to check permission"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
	}

	resp.Success = true
	resp.Message = "Permission check completed"
	resp.HasPermission = has
	return resp
}

// AssignRole gives a target user a role. super_user is bootstrap-only and is
// refused here (P0-13): no runtime path may hand it out — bootstrap happens
// once via SOP / direct DB. The assignment and its audit row share one
// transaction.
func (s *service) AssignRole(ctx context.Context, input *AssignRoleInput) *AssignRoleOutput {
	resp := &AssignRoleOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleName == "" {
		log.Warn().Msg("Role name empty")
		resp.Message = "Role name is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	// Policy guard (P0-13): super_user is bootstrap-only. Every assignment
	// flows through this method, so this is the single choke point that
	// makes the invariant structural.
	if input.RoleName == RoleSuperUser {
		log.Warn().Str("traceId", input.TraceId).Int("targetUserId", input.UserID).Msg("Rejected assignment of super_user role")
		resp.Message = "Cannot assign super_user role"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to assign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	known, err := db.RoleExists(ctx, input.RoleName)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check role catalog")
		resp.Message = "Failed to assign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if !known {
		log.Warn().Str("role", input.RoleName).Str("traceId", input.TraceId).Msg("Unknown role")
		resp.Message = "Unknown role"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	err = db.AssignRole(ctx, input.UserID, input.RoleName)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to assign role")
		resp.Message = "Failed to assign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Audit trail: who assigned what for whom, when. Written inside the same
	// transaction, so a rolled-back change never leaves an audit row.
	err = db.InsertPermissionAudit(ctx, input.ActorId, input.UserID, input.RoleName, AuditActionGrant, clock.Now())
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to write audit row")
		resp.Message = "Failed to assign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to assign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Role assigned successfully"
	return resp
}

// UnassignRole removes a role from a target user. Removing super_user stays
// allowed: stripping a bootstrap admin is a legitimate remediation. The
// removal and its audit row share one transaction.
func (s *service) UnassignRole(ctx context.Context, input *UnassignRoleInput) *UnassignRoleOutput {
	resp := &UnassignRoleOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleName == "" {
		log.Warn().Msg("Role name empty")
		resp.Message = "Role name is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to unassign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	known, err := db.RoleExists(ctx, input.RoleName)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check role catalog")
		resp.Message = "Failed to unassign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if !known {
		log.Warn().Str("role", input.RoleName).Str("traceId", input.TraceId).Msg("Unknown role")
		resp.Message = "Unknown role"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if input.RoleName == RoleSuperUser {
		// If the user holds super_user and is the only holder, removing it
		// would permanently lock out the system (super_user grants are
		// blocked, so no recovery path exists).
		has, err := db.UserHasRole(ctx, input.UserID, input.RoleName)
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to check role membership")
			resp.Message = "Failed to unassign role"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
		if has {
			holders, err := db.CountRoleHolders(ctx, input.RoleName)
			if err != nil {
				log.Err(err).Str("traceId", input.TraceId).Msg("failed to count role holders")
				resp.Message = "Failed to unassign role"
				resp.ErrorCode = ErrorCodeInternal
				return resp
			}
			if holders <= 1 {
				log.Warn().Str("traceId", input.TraceId).Msg("Refusing to remove the last super_user")
				resp.Message = "Cannot remove the last super_user role"
				resp.ErrorCode = ErrorCodeValidation
				return resp
			}
		}
	}

	err = db.UnassignRole(ctx, input.UserID, input.RoleName)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to unassign role")
		resp.Message = "Failed to unassign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Audit trail: who removed what for whom, when. Same transaction.
	err = db.InsertPermissionAudit(ctx, input.ActorId, input.UserID, input.RoleName, AuditActionRevoke, clock.Now())
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to write audit row")
		resp.Message = "Failed to unassign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to unassign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Role unassigned successfully"
	return resp
}

// AssignPermissionToRole adds a permission to a role's meaning. The
// permission must be registered in the catalog; the super_user role cannot
// be modified — it is a wildcard by design and bootstrap-only (P0-13). The
// change and its audit row share one transaction.
func (s *service) AssignPermissionToRole(ctx context.Context, input *AssignPermissionToRoleInput) *AssignPermissionToRoleOutput {
	resp := &AssignPermissionToRoleOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleName == "" {
		log.Warn().Msg("Role name empty")
		resp.Message = "Role name is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleName == RoleSuperUser {
		log.Warn().Str("traceId", input.TraceId).Msg("Rejected modification of super_user role")
		resp.Message = "Cannot modify super_user role"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to assign permission to role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	known, err := db.RoleExists(ctx, input.RoleName)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check role catalog")
		resp.Message = "Failed to assign permission to role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if !known {
		log.Warn().Str("role", input.RoleName).Str("traceId", input.TraceId).Msg("Unknown role")
		resp.Message = "Unknown role"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	registered, err := db.PermissionExists(ctx, input.Permission)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check permission catalog")
		resp.Message = "Failed to assign permission to role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if !registered {
		log.Warn().Str("permission", input.Permission).Str("traceId", input.TraceId).Msg("Unknown permission")
		resp.Message = "Unknown permission"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	err = db.AssignPermissionToRole(ctx, input.RoleName, input.Permission)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to assign permission to role")
		resp.Message = "Failed to assign permission to role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.InsertPermissionAudit(ctx, input.ActorId, auditTargetRoleScope, input.Permission, AuditActionGrant, clock.Now())
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to write audit row")
		resp.Message = "Failed to assign permission to role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to assign permission to role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permission assigned to role successfully"
	return resp
}

// RemovePermissionFromRole removes a permission from a role's meaning. Same
// rules as AssignPermissionToRole; super_user cannot be modified. The change
// and its audit row share one transaction.
func (s *service) RemovePermissionFromRole(ctx context.Context, input *RemovePermissionFromRoleInput) *RemovePermissionFromRoleOutput {
	resp := &RemovePermissionFromRoleOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleName == "" {
		log.Warn().Msg("Role name empty")
		resp.Message = "Role name is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleName == RoleSuperUser {
		log.Warn().Str("traceId", input.TraceId).Msg("Rejected modification of super_user role")
		resp.Message = "Cannot modify super_user role"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to remove permission from role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	known, err := db.RoleExists(ctx, input.RoleName)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check role catalog")
		resp.Message = "Failed to remove permission from role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if !known {
		log.Warn().Str("role", input.RoleName).Str("traceId", input.TraceId).Msg("Unknown role")
		resp.Message = "Unknown role"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	registered, err := db.PermissionExists(ctx, input.Permission)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check permission catalog")
		resp.Message = "Failed to remove permission from role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if !registered {
		log.Warn().Str("permission", input.Permission).Str("traceId", input.TraceId).Msg("Unknown permission")
		resp.Message = "Unknown permission"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	err = db.RemovePermissionFromRole(ctx, input.RoleName, input.Permission)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to remove permission from role")
		resp.Message = "Failed to remove permission from role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.InsertPermissionAudit(ctx, input.ActorId, auditTargetRoleScope, input.Permission, AuditActionRevoke, clock.Now())
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to write audit row")
		resp.Message = "Failed to remove permission from role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to remove permission from role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permission removed from role successfully"
	return resp
}
