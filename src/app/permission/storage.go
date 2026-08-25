package permission

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Storage = (*storage)(nil)

type storage struct {
	pool *pgxpool.Pool
}

func NewStorage(pool *pgxpool.Pool) Storage {
	return &storage{pool: pool}
}

func (s *storage) BeginTx(ctx context.Context) (StorageTx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &storageTx{tx: tx}, nil
}

var _ StorageTx = (*storageTx)(nil)

type storageTx struct {
	tx pgx.Tx
}

func (st *storageTx) AddPermission(ctx context.Context, userID int, permission string) error {
	_, err := st.tx.Exec(ctx,
		`INSERT INTO user_permissions (user_id, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, permission,
	)
	if err != nil {
		return fmt.Errorf("failed to add permission: %w", err)
	}
	return nil
}

func (st *storageTx) HasPermission(ctx context.Context, userID int, permission string) (bool, error) {
	var exists bool
	err := st.tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_permissions WHERE user_id = $1 AND permission = $2)`,
		userID, permission,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}
	return exists, nil
}

func (st *storageTx) PermissionExists(ctx context.Context, permission string) (bool, error) {
	var exists bool
	err := st.tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM permissions WHERE permission = $1)`,
		permission,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check permission catalog: %w", err)
	}
	return exists, nil
}

func (st *storageTx) RemovePermission(ctx context.Context, userID int, permission string) error {
	_, err := st.tx.Exec(ctx,
		`DELETE FROM user_permissions WHERE user_id = $1 AND permission = $2`,
		userID, permission,
	)
	if err != nil {
		return fmt.Errorf("failed to remove permission: %w", err)
	}
	return nil
}

func (st *storageTx) Commit() error {
	return st.tx.Commit(context.Background())
}

func (st *storageTx) Rollback() error {
	return st.tx.Rollback(context.Background())
}
