package permission

import "context"

// Service is the public contract of the permission module. Access is
// role-based: users hold roles, roles carry permissions, checks resolve
// through the chain user_roles → role_permissions.
type Service interface {
	// CheckPermission reports whether the target user effectively holds the
	// permission, either via one of their roles or via the super_user
	// wildcard.
	CheckPermission(ctx context.Context, input *CheckPermissionInput) *CheckPermissionOutput

	// AssignRole gives a target user a role. Bootstrap-only roles
	// (super_user) are refused; every assignment is audited.
	AssignRole(ctx context.Context, input *AssignRoleInput) *AssignRoleOutput

	// UnassignRole removes a role from a target user. Every removal is
	// audited.
	UnassignRole(ctx context.Context, input *UnassignRoleInput) *UnassignRoleOutput

	// AssignPermissionToRole adds a permission to a role's meaning. The
	// permission must exist in the catalog; the super_user role cannot be
	// modified (wildcard by design, bootstrap-only). Audited.
	AssignPermissionToRole(ctx context.Context, input *AssignPermissionToRoleInput) *AssignPermissionToRoleOutput

	// RemovePermissionFromRole removes a permission from a role's meaning.
	// The super_user role cannot be modified. Audited.
	RemovePermissionFromRole(ctx context.Context, input *RemovePermissionFromRoleInput) *RemovePermissionFromRoleOutput
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
	ErrorCode     string
	HasPermission bool
}

type AssignRoleInput struct {
	TraceId  string
	UserID   int
	RoleName string
	// ActorId is the user performing the assignment; 0 means a
	// system-initiated change (no acting user). Recorded in the audit log.
	ActorId int
}

type AssignRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode string
}

type UnassignRoleInput struct {
	TraceId  string
	UserID   int
	RoleName string
	ActorId  int
}

type UnassignRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode string
}

type AssignPermissionToRoleInput struct {
	TraceId    string
	RoleName   string
	Permission string
	// ActorId is the user performing the change; 0 means system-initiated.
	ActorId int
}

type AssignPermissionToRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode string
}

type RemovePermissionFromRoleInput struct {
	TraceId    string
	RoleName   string
	Permission string
	ActorId    int
}

type RemovePermissionFromRoleOutput struct {
	Success   bool
	Message   string
	TraceId   string
	ErrorCode string
}
