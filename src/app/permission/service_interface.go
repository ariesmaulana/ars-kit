package permission

import "context"

// PermissionSuperUser is the special permission that grants a user access to
// every action (wildcard) and the right to manage other users' permissions.
// It is stored and checked as "<user_id>:super_user" for direct grants and as
// the bare "super_user" when granted through a role.
const PermissionSuperUser = "super_user"

// ErrorCode categorizes permission-service failures so callers do not need to
// parse human-readable messages.
type ErrorCode string

const (
	ErrorCodeValidation ErrorCode = "validation"
	ErrorCodeConflict   ErrorCode = "conflict"
	ErrorCodeNotFound   ErrorCode = "not_found"
	ErrorCodeInternal   ErrorCode = "internal"
)

type Service interface {
	CheckPermission(ctx context.Context, input *CheckPermissionInput) *CheckPermissionOutput
	GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput
	RevokePermission(ctx context.Context, input *RevokePermissionInput) *RevokePermissionOutput

	// CreateRole creates a named role. The name is unique; a duplicate name
	// fails with "Role already exists".
	CreateRole(ctx context.Context, input *CreateRoleInput) *CreateRoleOutput

	// AddRolePermission maps a bare permission (e.g. "user:profile_update" or
	// "super_user") onto a role. Every user assigned the role inherits it for
	// CheckPermission and listings.
	AddRolePermission(ctx context.Context, input *AddRolePermissionInput) *AddRolePermissionOutput

	// AssignRole assigns a role to a user. The user inherits every permission
	// the role holds for CheckPermission and listings.
	AssignRole(ctx context.Context, input *AssignRoleInput) *AssignRoleOutput

	// ListUserPermissions lists a user's effective permissions: their direct
	// grants plus everything their roles grant, with the roles broken out
	// separately. Read-only.
	ListUserPermissions(ctx context.Context, input *ListUserPermissionsInput) *ListUserPermissionsOutput
}

type CheckPermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
}

type CheckPermissionOutput struct {
	Success       bool
	Message       string
	TraceId       string
	ErrorCode     ErrorCode
	HasPermission bool
}

type GrantPermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
}

type GrantPermissionOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

type RevokePermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
}

type RevokePermissionOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

type CreateRoleInput struct {
	TraceId     string
	Name        string
	Description string
}

type CreateRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	Role      Role
}

type AddRolePermissionInput struct {
	TraceId    string
	RoleId     int
	Permission string
}

type AddRolePermissionOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

type AssignRoleInput struct {
	TraceId string
	UserID  int
	RoleId  int
}

type AssignRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
}

type ListUserPermissionsInput struct {
	TraceId string
	UserID  int
}

// RolePermissions pairs a role with the bare permissions it holds.
type RolePermissions struct {
	Role        Role
	Permissions []string
}

type ListUserPermissionsOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode ErrorCode
	Direct    []string
	Roles     []RolePermissions
	Effective []string
}
