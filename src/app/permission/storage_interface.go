package permission

import (
	"context"
	"time"
)

type Storage interface {
	BeginTx(ctx context.Context) (StorageTx, error)
}

type StorageTx interface {
	// UserHasPermission reports whether any of the user's roles carries the
	// permission, within the current transaction.
	UserHasPermission(ctx context.Context, userID int, permission string) (bool, error)

	// UserHasRole reports whether the user holds the named role, within the
	// current transaction.
	UserHasRole(ctx context.Context, userID int, roleName string) (bool, error)

	// RoleExists reports whether the role name exists.
	RoleExists(ctx context.Context, roleName string) (bool, error)

	// PermissionExists reports whether the permission string is registered in
	// the catalog.
	PermissionExists(ctx context.Context, permission string) (bool, error)

	// AssignPermissionToRole grants a permission to a role. No-op when
	// already present; callers validate the role exists and the permission is
	// in the catalog first.
	AssignPermissionToRole(ctx context.Context, roleName string, permission string) error

	// RemovePermissionFromRole revokes a permission from a role.
	RemovePermissionFromRole(ctx context.Context, roleName string, permission string) error

	// AssignRole assigns the named role to the user. No-op when already
	// assigned; callers validate that the role exists first.
	AssignRole(ctx context.Context, userID int, roleName string) error

	// UnassignRole removes the named role from the user.
	UnassignRole(ctx context.Context, userID int, roleName string) error

	// CountRoleHolders returns the number of users currently holding the named role.
	CountRoleHolders(ctx context.Context, roleName string) (int, error)

	// InsertPermissionAudit records one assign/revoke entry in the audit
	// trail, within the caller's transaction. actorID is the acting user;
	// pass 0 for system-initiated changes (stored as NULL). at is the event
	// timestamp, passed in by the caller (e.g. clock.Now()) so the write is
	// deterministic and testable instead of relying on SQL NOW().
	InsertPermissionAudit(ctx context.Context, actorID int, targetID int, roleName string, action string, at time.Time) error

	Commit() error
	Rollback() error
}
