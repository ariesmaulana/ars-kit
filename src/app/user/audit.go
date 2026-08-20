package user

import "time"

// Audit events recorded in the audit_log table. Each constant is the value
// stored in the `event` column; the admin read endpoint filters on them.
const (
	// AuditEventGrant records a permission being granted to a user.
	AuditEventGrant = "grant"
	// AuditEventRevoke records a permission being revoked from a user.
	AuditEventRevoke = "revoke"
	// AuditEventPassword records a password change.
	AuditEventPassword = "password"
	// AuditEventUsername records a username change.
	AuditEventUsername = "username"
	// AuditEventEmail records an email change (wired by the email update
	// feature once it lands; the table and event name already exist).
	AuditEventEmail = "email"
	// AuditEventLogin records a successful login.
	AuditEventLogin = "login"
	// AuditEventAccount records an account activation or deactivation.
	AuditEventAccount = "account"
)

// AuditEntry is one persisted audit log row: who did what to whom, when.
type AuditEntry struct {
	Id int64
	// Event is one of the AuditEvent* constants.
	Event string
	// ActorId is the user who performed the action. Nil for system actions.
	ActorId *int
	// TargetUserId is the user the action affected. Same as ActorId for
	// self-service actions; nil for system-wide actions.
	TargetUserId *int
	// Metadata carries event-specific details (e.g. the permission string
	// for grant/revoke, old/new username for renames). Never store secrets
	// (password hashes, tokens) here.
	Metadata map[string]any
	// CreatedAt is when the event happened.
	CreatedAt time.Time
}

// AuditLogFilter narrows audit log queries. Zero values mean "no filter".
type AuditLogFilter struct {
	Event        string
	ActorId      int
	TargetUserId int
	Page         int
	PageSize     int
}

// intPtr returns a pointer to v for the nullable audit log id columns.
func intPtr(v int) *int {
	return &v
}
