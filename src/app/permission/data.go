package permission

import "time"

// RoleSuperUser is the bootstrap-only role whose members pass every
// permission check (wildcard). It is seeded by migration; AssignRole refuses
// to hand it out at runtime.
const RoleSuperUser = "super_user"

// RoleMember is the default role of self-registered users.
const RoleMember = "member"

// Actions recorded in the permission_audit table.
const (
	AuditActionGrant  = "grant"
	AuditActionRevoke = "revoke"
)

// auditTargetRoleScope marks permission_audit rows for role-content changes
// (assign/remove a permission to/from a role). Those have no single user as
// target, so target_id is 0; the affected role is identified by the actor's
// context and the permission column.
const auditTargetRoleScope = 0

// Role is one entry of the roles table.
type Role struct {
	Id        int
	Name      string
	CreatedAt time.Time
}

// PermissionAudit is one entry of the grant/revoke audit trail. It records
// role assignments: who assigned/removed which role for which user, when.
type PermissionAudit struct {
	Id int
	// ActorId is the user who performed the action; nil for system-initiated
	// changes (e.g. background workflow steps, which have no acting user).
	ActorId    *int
	TargetId   int
	Permission string // the role name that was assigned or removed
	Action     string
	CreatedAt  time.Time
}
