package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
)

// This file holds the admin-facing service methods (user list, lookup,
// activate/deactivate) and the first-super-user bootstrap used by the
// "ars-kit superuser" command. They live apart from service.go so the
// self-service user contract and the admin contract each change for one
// reason: admin operations are gated by the "<actorId>:super_user"
// permission and never read the actor's own profile.

// ListUsers returns a page of users for the admin user list.
func (s *service) ListUsers(ctx context.Context, input *ListUsersInput) *ListUsersOutput {
	resp := &ListUsersOutput{TraceId: input.TraceId}

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
		resp.Message = "Unauthorized: only super user can list users"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to list users"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	total, err := db.CountUsers(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to count users")
		resp.Message = "Failed to list users"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	users, err := db.ListUsers(ctx, input.Page, input.PageSize)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to list users")
		resp.Message = "Failed to list users"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to list users"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("actorId", input.ActorId).
		Int("page", input.Page).
		Int("pageSize", input.PageSize).
		Int("total", total).
		Msg("Users listed")

	resp.Success = true
	resp.Message = "Users retrieved successfully"
	resp.Users = users
	resp.Total = total
	return resp
}

// AdminGetUserById retrieves a user by ID for the admin user lookup.
func (s *service) AdminGetUserById(ctx context.Context, input *AdminGetUserByIdInput) *AdminGetUserByIdOutput {
	resp := &AdminGetUserByIdOutput{TraceId: input.TraceId}

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
	if input.Id < 1 {
		log.Warn().Msg("User ID below 1")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can look up users"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to fetch user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	data, err := db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("id", input.Id).
			Msg("User not found")
		resp.Message = "User not found"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to fetch user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("actorId", input.ActorId).
		Int("id", input.Id).
		Msg("User looked up by admin")

	resp.Success = true
	resp.Message = "User retrieved successfully"
	resp.User = data
	return resp
}

// SetUserActive activates or deactivates a target user's account. Deactivating
// revokes the ability to log in (existing JWTs keep working until they expire
// — full token revocation is tracked separately, see the token-revocation
// task). An admin cannot manage their own account through this endpoint.
func (s *service) SetUserActive(ctx context.Context, input *SetUserActiveInput) *SetUserActiveOutput {
	resp := &SetUserActiveOutput{TraceId: input.TraceId}

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
	if input.UserId < 1 {
		log.Warn().Msg("User ID below 1")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can manage user accounts"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	if input.ActorId == input.UserId {
		log.Warn().Str("traceId", input.TraceId).Int("actorId", input.ActorId).Msg("Admin tried to manage own account")
		resp.Message = "You cannot manage your own account"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to update account"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	data, errType, err := db.SetUserActive(ctx, input.UserId, input.IsActive)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Int("id", input.UserId).Msg("User not found")
			resp.Message = "User not found"
			resp.ErrorCode = ErrorCodeValidation
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update account state")
		resp.Message = "Failed to update account"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update account"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("actorId", input.ActorId).
		Int("id", input.UserId).
		Bool("isActive", input.IsActive).
		Msg("Account state updated by admin")

	resp.Success = true
	resp.User = data
	if input.IsActive {
		resp.Message = "User activated successfully"
	} else {
		resp.Message = "User deactivated successfully"
	}
	return resp
}

// BootstrapSuperUser creates the first super user — or upgrades an existing
// user on a re-run — by creating the account and granting the "super_user"
// permission. It is the documented path to bootstrap an admin on a fresh
// deploy (invoked by the "ars-kit superuser" command). There is intentionally
// no HTTP endpoint, so the operation cannot be abused remotely.
func (s *service) BootstrapSuperUser(ctx context.Context, input *BootstrapSuperUserInput) *BootstrapSuperUserOutput {
	resp := &BootstrapSuperUserOutput{TraceId: input.TraceId}

	if msg := validateRegisterInput(&RegisterInput{
		Username: input.Username,
		Email:    input.Email,
		FullName: input.FullName,
		Password: input.Password,
	}); msg != "" {
		resp.Message = msg
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to bootstrap super user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Re-run safety: if the username already exists (e.g. a previous partial
	// failure created the account but not the permission), upgrade the
	// existing row instead of failing on the unique constraint.
	existing, findErr := db.GetUserByUsername(ctx, input.Username)
	switch {
	case findErr == nil:
		// Keep the existing row; only the permission grant below is missing.
	case errors.Is(findErr, pgx.ErrNoRows):
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to hash password")
			resp.Message = "Failed to bootstrap super user"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
		insertedId, errType, err := db.InsertUser(ctx, input.Username, input.Email, input.FullName, string(hashedPassword))
		if err != nil {
			if errType == ErrTypeUniqueConstraint {
				log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert user")
				resp.Message = "Username or email already exists"
				resp.ErrorCode = ErrorCodeValidation
				return resp
			}
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert user")
			resp.Message = "Failed to bootstrap super user"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
		existing, err = db.GetUserById(ctx, insertedId)
		if err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to get user")
			resp.Message = "Failed to bootstrap super user"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
	default:
		log.Err(findErr).Str("traceId", input.TraceId).Msg("failed to look up user")
		resp.Message = "Failed to bootstrap super user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to bootstrap super user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if s.permissionService == nil {
		log.Warn().Str("traceId", input.TraceId).Msg("Permission service not wired")
		resp.Message = "Permission service not wired"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	grantOut := s.permissionService.GrantPermission(ctx, &permission.GrantPermissionInput{
		TraceId:    input.TraceId,
		UserID:     existing.Id,
		Permission: PermissionSuperUser,
	})
	if !grantOut.Success {
		log.Warn().Str("traceId", input.TraceId).Msg("failed to grant super user permission")
		resp.Message = grantOut.Message
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("id", existing.Id).
		Str("username", existing.Username).
		Msg("Super user bootstrapped")

	resp.Success = true
	resp.Message = "Super user bootstrapped successfully"
	resp.User = existing
	return resp
}
