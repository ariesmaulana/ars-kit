package user

import (
	"context"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
)

// Compile-time check to ensure service implements Service interface
var _ Service = (*service)(nil)

// service implements the Service interface
type service struct {
	storage           Storage
	permissionService permission.Service
}

// NewService creates a new user service instance
func NewService(storage Storage, permissionService permission.Service) Service {
	return &service{
		storage:           storage,
		permissionService: permissionService,
	}
}

// Register creates a new user account
func (s *service) Register(ctx context.Context, input *RegisterInput) *RegisterOutput {
	resp := &RegisterOutput{TraceId: input.TraceId}

	if msg := validateRegisterInput(input); msg != "" {
		resp.Message = msg
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to hash password")
		resp.Message = "Failed to register user"
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
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	data, err := db.GetUserById(ctx, insertedId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to get user")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Str("username", input.Username).
		Str("email", input.Email).
		Msg("User registered successfully")

	resp.Success = true
	resp.Message = "User registered successfully"
	resp.User = data
	return resp
}

// validateRegisterInput returns "" when the input is valid, otherwise the
// user-facing validation message. Shared by the synchronous Register and the
// async RegisterAsync so both flows enforce the same rules.
func validateRegisterInput(input *RegisterInput) string {
	if input.Username == "" {
		log.Warn().Msg("Username empty")
		return "Username is mandatory"
	}
	if len(input.Username) < 5 {
		log.Warn().Msg("Username too short")
		return "Username must be at least 5 characters long"
	}
	if input.Email == "" {
		log.Warn().Msg("Email empty")
		return "Email is mandatory"
	}
	if err := validateEmail(input.Email); err != nil {
		log.Warn().Msg("Invalid email")
		return "Invalid email"
	}
	if input.Password == "" {
		log.Warn().Msg("Password empty")
		return "Password is mandatory"
	}
	if len(input.Password) < 7 {
		log.Warn().Msg("Password too short")
		return "Password must be at least 7 characters long"
	}
	if input.FullName == "" {
		log.Warn().Msg("FullName empty")
		return "FullName is mandatory"
	}
	return ""
}

// Login authenticates a user
func (s *service) Login(ctx context.Context, input *LoginInput) *LoginOutput {
	resp := &LoginOutput{TraceId: input.TraceId}

	// Validate input
	if input.Username == "" {
		log.Warn().Msg("Username empty")
		resp.Message = "Username is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if input.Password == "" {
		log.Warn().Msg("Password empty")
		resp.Message = "Password is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Get user by username
	user, err := db.GetUserByUsername(ctx, input.Username)
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Str("username", input.Username).
			Msg("User not found")
		resp.Message = "Invalid username or password"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	// Get stored password
	storedPassword, err := db.GetUserPassword(ctx, user.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user password")
		resp.Message = "Invalid username or password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(input.Password))
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Str("username", input.Username).
			Msg("Invalid password attempt")
		resp.Message = "Invalid username or password"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Str("username", input.Username).
		Msg("User logged in successfully")

	resp.Success = true
	resp.Message = "Login successful"
	resp.User = user

	return resp
}

// hasPermission reports whether the acting user holds the given permission,
// which is either an action permission ("user:profile_update") or the super
// user permission (PermissionSuperUser). The permission module builds the
// "<user_id>:<permission>" key itself and also grants access to users holding
// "<user_id>:super_user". It logs and returns false when the permission
// module cannot confirm it.
func (s *service) hasPermission(ctx context.Context, traceId string, userID int, perm string) bool {
	if s.permissionService == nil {
		log.Warn().Str("traceId", traceId).Int("userId", userID).Str("permission", perm).Msg("Permission service not wired")
		return false
	}

	output := s.permissionService.CheckPermission(ctx, &permission.CheckPermissionInput{
		TraceId:    traceId,
		UserID:     userID,
		Permission: perm,
	})
	if !output.Success {
		log.Warn().
			Str("traceId", traceId).
			Int("userId", userID).
			Str("permission", perm).
			Msg("Permission check failed")
		return false
	}
	return output.HasPermission
}

// UpdateUsername updates a user's username
func (s *service) UpdateUsername(ctx context.Context, input *UpdateUsernameInput) *UpdateUsernameOutput {
	resp := &UpdateUsernameOutput{TraceId: input.TraceId}

	// Validate input
	if input.NewUsername == "" {
		log.Warn().Msg("New username empty")
		resp.Message = "New username is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if len(input.NewUsername) < 5 {
		log.Warn().Msg("New username too short")
		resp.Message = "Username must be at least 5 characters long"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.Id, PermissionUpdateProfile) {
		resp.Message = "Unauthorized: you do not have permission to update profile"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Lock user row for update (pessimistic lock)
	_, errType, err := db.LockUserById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
			resp.Message = "No Username Found"
			resp.ErrorCode = ErrorCodeValidation
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Update username
	err = db.UpdateUsername(ctx, input.Id, input.NewUsername)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update username")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Get updated user
	data, err := db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user")
		resp.Message = "No Username Found"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	// Commit transaction
	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("id", input.Id).
		Str("newUsername", input.NewUsername).
		Msg("Username updated successfully")

	resp.Success = true
	resp.Message = "Username updated successfully"
	resp.User = data

	return resp
}

// UpdatePassword updates a user's password
func (s *service) UpdatePassword(ctx context.Context, input *UpdatePasswordInput) *UpdatePasswordOutput {
	resp := &UpdatePasswordOutput{TraceId: input.TraceId}

	// Validate input
	if input.OldPassword == "" {
		log.Warn().Msg("Old password empty")
		resp.Message = "Old password is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if input.NewPassword == "" {
		log.Warn().Msg("New password empty")
		resp.Message = "New password is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if len(input.NewPassword) < 7 {
		log.Warn().Msg("New password too short")
		resp.Message = "Password must be at least 7 characters long"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.Id, PermissionUpdatePassword) {
		resp.Message = "Unauthorized: you do not have permission to update password"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Lock user row for update (pessimistic lock)
	_, errType, err := db.LockUserById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
			resp.Message = "User not found"
			resp.ErrorCode = ErrorCodeValidation
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Get stored password
	storedPassword, err := db.GetUserPassword(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user password")
		resp.Message = "User not found"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	// Verify old password
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(input.OldPassword))
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Int("id", input.Id).
			Msg("Invalid old password attempt")
		resp.Message = "Invalid old password"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to hash new password")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Update password
	err = db.UpdatePassword(ctx, input.Id, string(hashedPassword))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update password")
		resp.Message = err.Error()
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Commit transaction
	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("id", input.Id).
		Msg("Password updated successfully")

	resp.Success = true
	resp.Message = "Password updated successfully"

	return resp
}

// GetProfileById retrieves a user profile by ID
func (s *service) GetProfileById(ctx context.Context, input *GetProfileByIdInput) *GetProfileByIdOutput {
	resp := &GetProfileByIdOutput{TraceId: input.TraceId}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to fetch profile"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Get user by ID
	user, err := db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("id", input.Id).
			Msg("User not found")
		resp.Message = "User not found"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("id", input.Id).
		Msg("Profile retrieved successfully")

	resp.Success = true
	resp.Message = "Profile retrieved successfully"
	resp.User = user

	return resp
}

// GrantPermission assigns a permission to a target user.
// Only an actor holding the "<actorId>:super_user" permission may do this.
func (s *service) GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput {
	resp := &GrantPermissionOutput{TraceId: input.TraceId}

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
	if input.TargetUserId == 0 {
		log.Warn().Msg("Target user ID empty")
		resp.Message = "Target user ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can grant permissions"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	output := s.permissionService.GrantPermission(ctx, &permission.GrantPermissionInput{
		TraceId:    input.TraceId,
		UserID:     input.TargetUserId,
		Permission: input.Permission,
	})
	if !output.Success {
		resp.Message = output.Message
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permission granted successfully"
	return resp
}

// RevokePermission removes a permission from a target user.
// Only the *holding the <actorId>:super_user permission may do this.
func (s *service) RevokePermission(ctx context.Context, input *RevokePermissionInput) *RevokePermissionOutput {
	resp := &RevokePermissionOutput{TraceId: input.TraceId}

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
	if input.TargetUserId == 0 {
		log.Warn().Msg("Target user ID empty")
		resp.Message = "Target user ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can revoke permissions"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	output := s.permissionService.RevokePermission(ctx, &permission.RevokePermissionInput{
		TraceId:    input.TraceId,
		UserID:     input.TargetUserId,
		Permission: input.Permission,
	})
	if !output.Success {
		resp.Message = output.Message
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permission revoked successfully"
	return resp
}
