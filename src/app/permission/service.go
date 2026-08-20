package permission

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID <= 0 {
		log.Warn().Msg("User ID invalid")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
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

	has, err := db.HasPermission(ctx, input.UserID, key(input.UserID, input.Permission))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to check permission")
		resp.Message = "Failed to check permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// A user holding the "<user_id>:super_user" permission is allowed to do
	// anything: it acts as a wildcard for every other permission check.
	if !has {
		has, err = db.HasPermission(ctx, input.UserID, key(input.UserID, PermissionSuperUser))
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to check super user permission")
			resp.Message = "Failed to check permission"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
	}

	// Fall back to roles: a role assigned to the user may grant the exact
	// permission, or the "super_user" wildcard. Roles store bare permissions
	// (no "<user_id>:" prefix) because they are shared across users.
	if !has {
		has, err = db.HasRolePermission(ctx, input.UserID, input.Permission)
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to check role permission")
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

func (s *service) GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput {
	resp := &GrantPermissionOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID <= 0 {
		log.Warn().Msg("User ID invalid")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to grant permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	err = db.AddPermission(ctx, input.UserID, key(input.UserID, input.Permission))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to grant permission")
		resp.Message = "Failed to grant permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to grant permission"
		resp.ErrorCode = ErrorCodeInternal
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
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID <= 0 {
		log.Warn().Msg("User ID invalid")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to revoke permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	err = db.RemovePermission(ctx, input.UserID, key(input.UserID, input.Permission))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to revoke permission")
		resp.Message = "Failed to revoke permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to revoke permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permission revoked successfully"
	return resp
}

func (s *service) CreateRole(ctx context.Context, input *CreateRoleInput) *CreateRoleOutput {
	resp := &CreateRoleOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if strings.TrimSpace(input.Name) == "" {
		log.Warn().Msg("Role name empty")
		resp.Message = "Role name is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to create role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	id, err := db.CreateRole(ctx, strings.TrimSpace(input.Name), input.Description)
	if err != nil {
		if errors.Is(err, ErrRoleNameTaken) {
			log.Warn().Str("traceId", input.TraceId).Str("name", input.Name).Msg("role name already exists")
			resp.Message = "Role already exists"
			resp.ErrorCode = ErrorCodeConflict
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to create role")
		resp.Message = "Failed to create role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	role, err := db.GetRoleById(ctx, id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Int("roleId", id).Msg("failed to load created role")
		resp.Message = "Failed to create role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to create role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Role created successfully"
	resp.Role = *role
	return resp
}

func (s *service) AddRolePermission(ctx context.Context, input *AddRolePermissionInput) *AddRolePermissionOutput {
	resp := &AddRolePermissionOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleId <= 0 {
		log.Warn().Msg("Role ID invalid")
		resp.Message = "Role ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to add role permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	if _, err := db.GetRoleById(ctx, input.RoleId); err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			log.Warn().Str("traceId", input.TraceId).Int("roleId", input.RoleId).Msg("role not found")
			resp.Message = "Role not found"
			resp.ErrorCode = ErrorCodeNotFound
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to load role")
		resp.Message = "Failed to add role permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.AddRolePermission(ctx, input.RoleId, input.Permission)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to add role permission")
		resp.Message = "Failed to add role permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to add role permission"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Role permission added successfully"
	return resp
}

func (s *service) AssignRole(ctx context.Context, input *AssignRoleInput) *AssignRoleOutput {
	resp := &AssignRoleOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID <= 0 {
		log.Warn().Msg("User ID invalid")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.RoleId <= 0 {
		log.Warn().Msg("Role ID invalid")
		resp.Message = "Role ID is mandatory"
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

	if _, err := db.GetRoleById(ctx, input.RoleId); err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			log.Warn().Str("traceId", input.TraceId).Int("roleId", input.RoleId).Msg("role not found")
			resp.Message = "Role not found"
			resp.ErrorCode = ErrorCodeNotFound
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to load role")
		resp.Message = "Failed to assign role"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.AssignRole(ctx, input.UserID, input.RoleId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to assign role")
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

func (s *service) ListUserPermissions(ctx context.Context, input *ListUserPermissionsInput) *ListUserPermissionsOutput {
	resp := &ListUserPermissionsOutput{TraceId: input.TraceId}
	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.UserID <= 0 {
		log.Warn().Msg("User ID invalid")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to begin transaction")
		resp.Message = "Failed to list permissions"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	direct, err := db.ListDirectPermissions(ctx, input.UserID)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to list direct permissions")
		resp.Message = "Failed to list permissions"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	roles, err := db.ListUserRoles(ctx, input.UserID)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to list user roles")
		resp.Message = "Failed to list permissions"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	rolePermissions := make([]RolePermissions, 0, len(roles))
	for _, role := range roles {
		perms, err := db.ListRolePermissions(ctx, role.Id)
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Int("roleId", role.Id).Msg("failed to list role permissions")
			resp.Message = "Failed to list permissions"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
		rolePermissions = append(rolePermissions, RolePermissions{
			Role:        role,
			Permissions: perms,
		})
	}

	effective, err := db.ListUserPermissions(ctx, input.UserID)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to list effective permissions")
		resp.Message = "Failed to list permissions"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to list permissions"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permissions retrieved successfully"
	resp.Direct = direct
	resp.Roles = rolePermissions
	resp.Effective = effective
	return resp
}
