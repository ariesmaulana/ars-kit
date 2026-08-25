package permission

import (
	"context"
	"fmt"
	"time"

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

func (st *storageTx) UserHasPermission(ctx context.Context, userID int, permission string) (bool, error) {
	var exists bool
	err := st.tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			WHERE ur.user_id = $1 AND rp.permission = $2
		)`,
		userID, permission,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}
	return exists, nil
}

func (st *storageTx) UserHasRole(ctx context.Context, userID int, roleName string) (bool, error) {
	var exists bool
	err := st.tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.name = $2
		)`,
		userID, roleName,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check role: %w", err)
	}
	return exists, nil
}

func (st *storageTx) RoleExists(ctx context.Context, roleName string) (bool, error) {
	var exists bool
	err := st.tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`,
		roleName,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check role catalog: %w", err)
	}
	return exists, nil
}

func (st *storageTx) AssignRole(ctx context.Context, userID int, roleName string) error {
	_, err := st.tx.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id)
		 SELECT $1, r.id FROM roles r WHERE r.name = $2
		 ON CONFLICT DO NOTHING`,
		userID, roleName,
	)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}
	return nil
}

func (st *storageTx) UnassignRole(ctx context.Context, userID int, roleName string) error {
	_, err := st.tx.Exec(ctx,
		`DELETE FROM user_roles ur
		 USING roles r
		 WHERE ur.role_id = r.id AND ur.user_id = $1 AND r.name = $2`,
		userID, roleName,
	)
	if err != nil {
		return fmt.Errorf("failed to unassign role: %w", err)
	}
	return nil
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

func (st *storageTx) AssignPermissionToRole(ctx context.Context, roleName string, permission string) error {
	_, err := st.tx.Exec(ctx,
		`INSERT INTO role_permissions (role_id, permission)
		 SELECT r.id, $2 FROM roles r WHERE r.name = $1
		 ON CONFLICT DO NOTHING`,
		roleName, permission,
	)
	if err != nil {
		return fmt.Errorf("failed to assign permission to role: %w", err)
	}
	return nil
}

func (st *storageTx) RemovePermissionFromRole(ctx context.Context, roleName string, permission string) error {
	_, err := st.tx.Exec(ctx,
		`DELETE FROM role_permissions rp
		 USING roles r
		 WHERE rp.role_id = r.id AND r.name = $1 AND rp.permission = $2`,
		roleName, permission,
	)
	if err != nil {
		return fmt.Errorf("failed to remove permission from role: %w", err)
	}
	return nil
}

func (st *storageTx) InsertPermissionAudit(ctx context.Context, actorID int, targetID int, roleName string, action string, at time.Time) error {
	var actor *int
	if actorID != 0 {
		actor = &actorID
	}
	_, err := st.tx.Exec(ctx,
		`INSERT INTO permission_audit (actor_id, target_id, permission, action, created_at) VALUES ($1, $2, $3, $4, $5)`,
		actor, targetID, roleName, action, at,
	)
	if err != nil {
		return fmt.Errorf("failed to insert permission audit: %w", err)
	}
	return nil
}

func (st *storageTx) Commit() error {
	return st.tx.Commit(context.Background())
}

func (st *storageTx) Rollback() error {
	return st.tx.Rollback(context.Background())
}
