package user

import (
	"context"
)

// Service defines the interface for user business logic
type Service interface {
	// Register creates a new user account
	Register(ctx context.Context, input *RegisterInput) *RegisterOutput

	// Login authenticates a user
	Login(ctx context.Context, input *LoginInput) *LoginOutput

	// UpdateUsername updates a user's username
	UpdateUsername(ctx context.Context, input *UpdateUsernameInput) *UpdateUsernameOutput

	// UpdatePassword updates a user's password
	UpdatePassword(ctx context.Context, input *UpdatePasswordInput) *UpdatePasswordOutput

	// GetProfileById retrieves a user profile by ID
	GetProfileById(ctx context.Context, input *GetProfileByIdInput) *GetProfileByIdOutput

	// GrantPermission assigns a permission to a target user.
	// Only a user holding the "<actorId>:super_user" permission may do this.
	GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput

	// RevokePermission removes a permission from a target user.
	// Only a user holding the "<actorId>:super_user" permission may do this.
	RevokePermission(ctx context.Context, input *RevokePermissionInput) *RevokePermissionOutput
}

// ErrorCode categorizes why an operation failed so adapters can map it to an
// HTTP status without string-matching the human-readable message.
type ErrorCode string

const (
	// ErrorCodeValidation covers bad input, missing entities, and duplicates.
	ErrorCodeValidation ErrorCode = "validation"
	// ErrorCodeUnauthorized covers bad credentials (e.g. wrong password).
	ErrorCodeUnauthorized ErrorCode = "unauthorized"
	// ErrorCodeForbidden covers authenticated users without the required permission.
	ErrorCodeForbidden ErrorCode = "forbidden"
	// ErrorCodeInternal covers real system failures (storage, hashing, commit).
	ErrorCodeInternal ErrorCode = "internal"
)

// RegisterInput represents input for user registration
type RegisterInput struct {
	TraceId  string
	Username string
	Email    string
	FullName string
	Password string
}

// RegisterOutput represents output after user registration
type RegisterOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}

// LoginInput represents input for user login
type LoginInput struct {
	TraceId  string
	Username string
	Password string
}

// LoginOutput represents output after user login
type LoginOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}

// UpdateUsernameInput represents input for updating username
type UpdateUsernameInput struct {
	TraceId     string
	Id          int
	NewUsername string
}

// UpdateUsernameOutput represents output after updating username
type UpdateUsernameOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}

// UpdatePasswordInput represents input for updating password
type UpdatePasswordInput struct {
	TraceId     string
	Id          int
	OldPassword string
	NewPassword string
}

// UpdatePasswordOutput represents output after updating password
type UpdatePasswordOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// GetProfileByIdInput represents input for getting user profile
type GetProfileByIdInput struct {
	TraceId string
	Id      int
}

// GetProfileByIdOutput represents output after getting user profile
type GetProfileByIdOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}

// GrantPermissionInput represents input for assigning a permission to a user.
type GrantPermissionInput struct {
	TraceId      string
	ActorId      int // Must hold the "<actorId>:super_user" permission
	TargetUserId int
	Permission   string // e.g. "user:profile_update" or "super_user"
}

// GrantPermissionOutput represents output after assigning a permission.
type GrantPermissionOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// RevokePermissionInput represents input for removing a permission from a user.
type RevokePermissionInput struct {
	TraceId      string
	ActorId      int // Must hold the "<actorId>:super_user" permission
	TargetUserId int
	Permission   string
}

// RevokePermissionOutput represents output after removing a permission.
type RevokePermissionOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}
