package permission

import "context"

type Storage interface {
	BeginTx(ctx context.Context) (StorageTx, error)
}

type StorageTx interface {
	AddPermission(ctx context.Context, userID int, permission string) error
	HasPermission(ctx context.Context, userID int, permission string) (bool, error)
	RemovePermission(ctx context.Context, userID int, permission string) error
	PermissionExists(ctx context.Context, permission string) (bool, error)
	Commit() error
	Rollback() error
}
