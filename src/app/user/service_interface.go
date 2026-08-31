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

	// ForgotPassword emails a password-reset link to the account's email
	// address. Responds identically whether or not the email exists, so it
	// cannot be used to enumerate accounts. Sends nothing for unknown emails.
	ForgotPassword(ctx context.Context, input *ForgotPasswordInput) *ForgotPasswordOutput

	// ResetPassword sets a new password authenticated by a single-use reset
	// token from the forgot-password email. Invalidates every existing session
	// (token_version bump + refresh-token revocation).
	ResetPassword(ctx context.Context, input *ResetPasswordInput) *ResetPasswordOutput

	// SendVerificationEmail emails an email-verification link to the account's
	// address. Already-verified accounts are a no-op. Like ForgotPassword it
	// never reveals whether the email exists.
	SendVerificationEmail(ctx context.Context, input *SendVerificationEmailInput) *SendVerificationEmailOutput

	// VerifyEmail marks the account's email as verified, authenticated by a
	// single-use verification token from the verification email.
	VerifyEmail(ctx context.Context, input *VerifyEmailInput) *VerifyEmailOutput

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
	// ErrorCodeNone is a sentinel for "no error" (success path).
	ErrorCodeNone ErrorCode = ""
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

// ForgotPasswordInput is the forgot-password request: just the email.
type ForgotPasswordInput struct {
	TraceId string
	Email   string
}

// ForgotPasswordOutput is the forgot-password result. It always reports
// success with a generic message, regardless of whether the email exists, to
// avoid leaking account existence.
type ForgotPasswordOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// ResetPasswordInput carries the reset token from the email and the new
// password.
type ResetPasswordInput struct {
	TraceId     string
	Token       string
	NewPassword string
}

// ResetPasswordOutput is the reset-password result.
type ResetPasswordOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// SendVerificationEmailInput requests a fresh verification email.
type SendVerificationEmailInput struct {
	TraceId string
	Email   string
}

// SendVerificationEmailOutput reports whether the request was accepted. It
// does not reveal whether the email exists or was already verified.
type SendVerificationEmailOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// VerifyEmailInput carries the verification token from the email.
type VerifyEmailInput struct {
	TraceId string
	Token   string
}

// VerifyEmailOutput is the email-verification result.
type VerifyEmailOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

// EmailConfig groups the configuration the user service needs for
// forgot-password and email-verification flows. A zero value is usable: an
// empty AppURL yields a relative link, and a zero TokenExpiry falls back to
// 24 hours. Email delivery itself is enqueued to the send_email workflow, so
// the service holds no sender reference.
type EmailConfig struct {
	// AppURL is the frontend base URL used to build reset/verify links.
	AppURL string
	// TokenExpiry is how long a single-purpose email token stays valid.
	TokenExpiry time.Duration
}
