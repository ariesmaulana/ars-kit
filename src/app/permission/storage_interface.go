package permission

import (
	"context"
	"time"
)

type Storage interface {
	BeginTx(ctx context.Context) (StorageTx, error)
}

type StorageTx interface {
	AddPermission(ctx context.Context, userID int, permission string) error
	HasPermission(ctx context.Context, userID int, permission string) (bool, error)
	RemovePermission(ctx context.Context, userID int, permission string) error
	PermissionExists(ctx context.Context, permission string) (bool, error)

	// InsertPermissionAudit records one grant/revoke entry in the audit trail,
	// within the caller's transaction. actorID is the acting user; pass 0 for
	// system-initiated changes (stored as NULL). at is the event timestamp,
	// passed in by the caller (e.g. clock.Now()) so the write is deterministic
	// and testable instead of relying on SQL NOW().
	InsertPermissionAudit(ctx context.Context, actorID int, targetID int, permission string, action string, at time.Time) error
	Commit() error
	Rollback() error
}
