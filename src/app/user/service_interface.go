package user

import (
	"context"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
)

// Service defines the interface for user business logic
type Service interface {
	// Register creates a new user account
	Register(ctx context.Context, input *RegisterInput) *RegisterOutput

	// DemoWorkflow validates the input and enqueues a demo workflow job
	// instead of creating the user synchronously. Background workers create
	// the user and grant it the workflow permission.
	DemoWorkflow(ctx context.Context, input *DemoWorkflowInput) *DemoWorkflowOutput

	// RegisterUser creates a user account. It is the seam the demo workflow's
	// RegisterUser step calls.
	RegisterUser(ctx context.Context, input *workflow.RegisterUserInput) *workflow.RegisterUserOutput

	// GrantPermissionSystem grants a permission without the super-user actor
	// check. It is the seam the demo workflow's GrantPermission step calls.
	GrantPermissionSystem(ctx context.Context, input *workflow.GrantPermissionInput) *workflow.GrantPermissionOutput

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

	// ListUsers returns a paginated page of users for the admin user list.
	// Only a user holding the "<actorId>:super_user" permission may do this.
	ListUsers(ctx context.Context, input *ListUsersInput) *ListUsersOutput

	// AdminGetUserById retrieves any user by ID for the admin user lookup.
	// Only a user holding the "<actorId>:super_user" permission may do this.
	AdminGetUserById(ctx context.Context, input *AdminGetUserByIdInput) *AdminGetUserByIdOutput

	// SetUserActive activates or deactivates a target account. Deactivating
	// revokes the ability to log in. Only a super user may do this, and never
	// on their own account.
	SetUserActive(ctx context.Context, input *SetUserActiveInput) *SetUserActiveOutput

	// BootstrapSuperUser creates the first super user — or upgrades an
	// existing account on a re-run — and grants it the super_user permission.
	// There is deliberately no HTTP endpoint for it; the "ars-kit superuser"
	// command is the documented bootstrap path.
	BootstrapSuperUser(ctx context.Context, input *BootstrapSuperUserInput) *BootstrapSuperUserOutput
}

// DemoWorkflowInput represents input for the async demo registration. The
// user is not created synchronously — a demo workflow job is enqueued instead.
type DemoWorkflowInput struct {
	TraceId  string
	Email    string
	Username string
}

// DemoWorkflowOutput represents output after enqueuing the demo workflow.
type DemoWorkflowOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
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
	// ErrorCodeLocked covers accounts temporarily locked by repeated failed
	// login attempts. Clients should surface it as a throttle/retry state.
	ErrorCodeLocked ErrorCode = "locked"
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
//
// When ErrorCode is ErrorCodeLocked, LockedUntil and RetryAfterSeconds expose
// the lockout state so clients can show when the account unlocks.
type LoginOutput struct {
	Success           bool
	Message           string
	TraceId           string
	ErrorCode         ErrorCode
	User              User
	LockedUntil       *time.Time
	RetryAfterSeconds int
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

// ListUsersInput represents input for the admin user list.
type ListUsersInput struct {
	TraceId  string
	ActorId  int // Must hold the "<actorId>:super_user" permission
	Page     int // 1-based
	PageSize int // 1..100
}

// ListUsersOutput represents output after listing users.
type ListUsersOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	Users     []User
	Total     int
}

// AdminGetUserByIdInput represents input for the admin user lookup.
type AdminGetUserByIdInput struct {
	TraceId string
	ActorId int // Must hold the "<actorId>:super_user" permission
	Id      int
}

// AdminGetUserByIdOutput represents output after looking up a user.
type AdminGetUserByIdOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}

// SetUserActiveInput represents input for activating/deactivating a user.
type SetUserActiveInput struct {
	TraceId  string
	ActorId  int // Must hold the "<actorId>:super_user" permission
	UserId   int
	IsActive bool
}

// SetUserActiveOutput represents output after changing a user's active state.
type SetUserActiveOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}

// BootstrapSuperUserInput represents input for the first-super-user bootstrap.
type BootstrapSuperUserInput struct {
	TraceId  string
	Username string
	Email    string
	FullName string
	Password string
}

// BootstrapSuperUserOutput represents output after bootstrapping a super user.
type BootstrapSuperUserOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}
