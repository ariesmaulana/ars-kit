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

	// Login authenticates a user. On success the output carries the freshly
	// issued access and refresh tokens (the refresh token is persisted
	// server-side for rotation and revocation).
	Login(ctx context.Context, input *LoginInput) *LoginOutput

	// Refresh rotates a refresh token: it revokes the presented token and
	// issues a fresh access + refresh pair, so a stolen refresh token can only
	// be used once. The refresh fails when the token is unknown, revoked,
	// expired, or the user's token_version has moved past the version the
	// token was issued at (password change).
	Refresh(ctx context.Context, input *RefreshInput) *RefreshOutput

	// Logout revokes the presented refresh token server-side so it cannot be
	// replayed. It is idempotent: revoking an already-revoked or unknown token
	// still reports success (the client clears its cookies either way).
	Logout(ctx context.Context, input *LogoutInput) *LogoutOutput

	// UpdateUsername updates a user's username
	UpdateUsername(ctx context.Context, input *UpdateUsernameInput) *UpdateUsernameOutput

	// UpdatePassword updates a user's password
	UpdatePassword(ctx context.Context, input *UpdatePasswordInput) *UpdatePasswordOutput

	// GetProfileById retrieves a user profile by ID
	GetProfileById(ctx context.Context, input *GetProfileByIdInput) *GetProfileByIdOutput

	// AssignRole assigns a role to a target user (admin).
	// Only a user holding super_user may do this.
	AssignRole(ctx context.Context, input *AssignRoleInput) *AssignRoleOutput

	// UnassignRole removes a role from a target user (admin).
	// Only a user holding super_user may do this.
	UnassignRole(ctx context.Context, input *UnassignRoleInput) *UnassignRoleOutput

	// AssignPermissionToRole adds a permission to a role's meaning (admin).
	// Only a user holding super_user may do this.
	AssignPermissionToRole(ctx context.Context, input *AssignPermissionToRoleInput) *AssignPermissionToRoleOutput

	// RemovePermissionFromRole removes a permission from a role's meaning (admin).
	// Only a user holding super_user may do this.
	RemovePermissionFromRole(ctx context.Context, input *RemovePermissionFromRoleInput) *RemovePermissionFromRoleOutput

	// ListUsers lists users (admin). Requires the super_user permission.
	ListUsers(ctx context.Context, input *ListUsersInput) *ListUsersOutput

	// GetUser fetches any user by id (admin). Requires the super_user permission.
	GetUser(ctx context.Context, input *GetUserInput) *GetUserOutput

	// DeleteUser hard-deletes a user (admin). Requires the super_user permission.
	DeleteUser(ctx context.Context, input *DeleteUserInput) *DeleteUserOutput

	// UpdateUserStatus sets a target user's status (active/disabled/suspended)
	// (admin). Requires the super_user permission. Disabling or suspending a
	// user also revokes all their active refresh tokens so existing sessions
	// die immediately; an actor cannot disable/suspend their own account.
	UpdateUserStatus(ctx context.Context, input *UpdateUserStatusInput) *UpdateUserStatusOutput
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
	// ErrorCodeNotFound covers lookups that matched no row (e.g. a user id
	// that does not exist). The handler maps it to HTTP 404.
	ErrorCodeNotFound ErrorCode = "not_found"
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
// On success it also carries the freshly issued access and refresh tokens.
type RegisterOutput struct {
	Success      bool
	Message      string
	TraceId      string
	ErrorCode    ErrorCode
	User         User
	AccessToken  string
	RefreshToken string
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
// the lockout state so clients can show when the account unlocks. On success
// it also carries the freshly issued access and refresh tokens.
type LoginOutput struct {
	Success           bool
	Message           string
	TraceId           string
	ErrorCode         ErrorCode
	User              User
	LockedUntil       *time.Time
	RetryAfterSeconds int
	AccessToken       string
	RefreshToken      string
}

// RefreshInput represents input for rotating a refresh token.
type RefreshInput struct {
	TraceId      string
	RefreshToken string
}

// RefreshOutput represents output after rotating a refresh token. On success
// it carries the new access and refresh tokens plus the authenticated user.
type RefreshOutput struct {
	Success      bool
	Message      string
	TraceId      string
	ErrorCode    ErrorCode
	User         User
	AccessToken  string
	RefreshToken string
}

// LogoutInput represents input for revoking a refresh token.
type LogoutInput struct {
	TraceId      string
	RefreshToken string
}

// LogoutOutput represents output after logging out.
type LogoutOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
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

// ListUsersInput lists users (admin). ActorId must hold super_user. Status
// optionally filters by account status (empty = any).
type ListUsersInput struct {
	TraceId string
	ActorId int
	Page    int
	Size    int
	Filter  string
	Status  UserStatus
}

// ListUsersOutput is the paginated admin user list.
type ListUsersOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	Users     []User
	Total     int
	Page      int
	Size      int
}

// GetUserInput fetches a user by id (admin). ActorId must hold super_user.
type GetUserInput struct {
	TraceId string
	ActorId int
	Id      int
}

// GetUserOutput is the admin user fetch result.
type GetUserOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	User      User
}

// DeleteUserInput deletes a user by id (admin). ActorId must hold super_user.
type DeleteUserInput struct {
	TraceId string
	ActorId int
	Id      int
}

// DeleteUserOutput is the admin user deletion result.
type DeleteUserOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// AssignRoleInput represents input for assigning a role to a user.
type AssignRoleInput struct {
	TraceId      string
	ActorId      int // Must hold super_user
	TargetUserId int
	RoleName     string // e.g. "member"; bootstrap-only roles are refused
}

// AssignRoleOutput represents output after assigning a role.
type AssignRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// UnassignRoleInput represents input for removing a role from a user.
type UnassignRoleInput struct {
	TraceId      string
	ActorId      int // Must hold super_user
	TargetUserId int
	RoleName     string
}

// UnassignRoleOutput represents output after removing a role.
type UnassignRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// AssignPermissionToRoleInput represents input for adding a permission to a role.
type AssignPermissionToRoleInput struct {
	TraceId    string
	ActorId    int // Must hold super_user
	RoleName   string
	Permission string
}

// AssignPermissionToRoleOutput represents output after the change.
type AssignPermissionToRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// RemovePermissionFromRoleInput represents input for removing a permission from a role.
type RemovePermissionFromRoleInput struct {
	TraceId    string
	ActorId    int // Must hold super_user
	RoleName   string
	Permission string
}

// RemovePermissionFromRoleOutput represents output after the change.
type RemovePermissionFromRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// UpdateUserStatusInput represents input for setting a user's status.
type UpdateUserStatusInput struct {
	TraceId      string
	ActorId      int // Must hold the "<actorId>:super_user" permission
	TargetUserId int
	Status       UserStatus // active | disabled | suspended
}

// UpdateUserStatusOutput represents output after setting a user's status.
type UpdateUserStatusOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}
