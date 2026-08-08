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
	Success bool
	Message string
	TraceId string
	User    User
}

// LoginInput represents input for user login
type LoginInput struct {
	TraceId  string
	Username string
	Password string
}

// LoginOutput represents output after user login
type LoginOutput struct {
	Success bool
	Message string
	TraceId string
	User    User
}

// UpdateUsernameInput represents input for updating username
type UpdateUsernameInput struct {
	TraceId     string
	Id          int
	NewUsername string
}

// UpdateUsernameOutput represents output after updating username
type UpdateUsernameOutput struct {
	Success bool
	Message string
	TraceId string
	User    User
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
	Success bool
	Message string
	TraceId string
}

// GetProfileByIdInput represents input for getting user profile
type GetProfileByIdInput struct {
	TraceId string
	Id      int
}

// GetProfileByIdOutput represents output after getting user profile
type GetProfileByIdOutput struct {
	Success bool
	Message string
	TraceId string
	User    User
}

// Permission action tokens used in "<user_id>:<module>:<action>" permission strings.
const (
	// ModuleUser is how user-module permissions are namespaced.
	ModuleUser = "user"

	// ActionUpdateProfile gates UpdateUsername.
	ActionUpdateProfile = "profile_update"
	// ActionUpdatePassword gates UpdatePassword.
	ActionUpdatePassword = "password_update"

	// PermissionSuperUser is the special permission that grants a user access
	// to every action (wildcard) and the right to manage other users'
	// permissions. It is checked as "<user_id>:super_user".
	PermissionSuperUser = "super_user"
)

// GrantPermissionInput represents input for assigning a permission to a user.
type GrantPermissionInput struct {
	TraceId      string
	ActorId      int // Must hold the "<actorId>:super_user" permission
	TargetUserId int
	Permission   string // e.g. "user:profile_update" or "super_user"
}

// GrantPermissionOutput represents output after assigning a permission.
type GrantPermissionOutput struct {
	Success bool
	Message string
	TraceId string
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
	Success bool
	Message string
	TraceId string
}
