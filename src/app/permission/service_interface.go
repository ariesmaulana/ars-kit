package permission

import "context"

// PermissionSuperUser is the special permission that grants a user access to
// every action (wildcard) and the right to manage other users' permissions.
// It is stored and checked as "<user_id>:super_user".
const PermissionSuperUser = "super_user"

type Service interface {
	CheckPermission(ctx context.Context, input *CheckPermissionInput) *CheckPermissionOutput
	GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput
	RevokePermission(ctx context.Context, input *RevokePermissionInput) *RevokePermissionOutput
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
	HasPermission bool
}

type GrantPermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
	// ActorId is the user performing the grant; 0 means a system-initiated
	// change (no acting user, e.g. a background workflow step). Recorded in
	// the audit log.
	ActorId int
}

type GrantPermissionOutput struct {
	Success bool
	Message string
	TraceId string
}

type RevokePermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
	// ActorId is the user performing the revoke; 0 means a system-initiated
	// change (no acting user). Recorded in the audit log.
	ActorId int
}

type RevokePermissionOutput struct {
	Success bool
	Message string
	TraceId string
}
