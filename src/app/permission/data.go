package permission

import "time"

// Actions recorded in the permission_audit table.
const (
	AuditActionGrant  = "grant"
	AuditActionRevoke = "revoke"
)

type UserPermission struct {
	Id         int
	UserId     int
	Permission string
	CreatedAt  time.Time
}

// PermissionAudit is one entry of the grant/revoke audit trail.
type PermissionAudit struct {
	Id int
	// ActorId is the user who performed the action; nil for system-initiated
	// changes (e.g. background workflow steps, which have no acting user).
	ActorId    *int
	TargetId   int
	Permission string
	Action     string
	CreatedAt  time.Time
}
