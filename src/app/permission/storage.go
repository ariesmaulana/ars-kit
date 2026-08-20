package permission

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

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

// ---------------------------------------------------------------------------
// Roles
// ---------------------------------------------------------------------------

func (st *storageTx) CreateRole(ctx context.Context, name, description string) (int, error) {
	var id int
	// ON CONFLICT DO NOTHING + RETURNING makes the duplicate-name check
	// race-free: a conflicting insert returns no row instead of erroring.
	err := st.tx.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING RETURNING id`,
		name, description,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrRoleNameTaken
	}
	if err != nil {
		return 0, fmt.Errorf("failed to create role: %w", err)
	}
	return id, nil
}

func (st *storageTx) GetRoleById(ctx context.Context, roleID int) (*Role, error) {
	var r Role
	err := st.tx.QueryRow(ctx,
		`SELECT id, name, description, created_at, updated_at FROM roles WHERE id = $1`,
		roleID,
	).Scan(&r.Id, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get role by id: %w", err)
	}
	return &r, nil
}

func (st *storageTx) DeleteRole(ctx context.Context, roleID int) error {
	tag, err := st.tx.Exec(ctx, `DELETE FROM roles WHERE id = $1`, roleID)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func (st *storageTx) AddRolePermission(ctx context.Context, roleID int, permission string) error {
	_, err := st.tx.Exec(ctx,
		`INSERT INTO role_permissions (role_id, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permission,
	)
	if err != nil {
		return fmt.Errorf("failed to add role permission: %w", err)
	}
	return nil
}

func (st *storageTx) RemoveRolePermission(ctx context.Context, roleID int, permission string) error {
	_, err := st.tx.Exec(ctx,
		`DELETE FROM role_permissions WHERE role_id = $1 AND permission = $2`,
		roleID, permission,
	)
	if err != nil {
		return fmt.Errorf("failed to remove role permission: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// User -> role assignment
// ---------------------------------------------------------------------------

func (st *storageTx) AssignRole(ctx context.Context, userID, roleID int) error {
	_, err := st.tx.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, roleID,
	)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}
	return nil
}

func (st *storageTx) UnassignRole(ctx context.Context, userID, roleID int) error {
	_, err := st.tx.Exec(ctx,
		`DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`,
		userID, roleID,
	)
	if err != nil {
		return fmt.Errorf("failed to unassign role: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Role-aware checks and listing
// ---------------------------------------------------------------------------

func (st *storageTx) HasRolePermission(ctx context.Context, userID int, permission string) (bool, error) {
	var exists bool
	err := st.tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			WHERE ur.user_id = $1 AND (rp.permission = $2 OR rp.permission = $3)
		)`,
		userID, permission, PermissionSuperUser,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check role permission: %w", err)
	}
	return exists, nil
}

func (st *storageTx) ListUserRoles(ctx context.Context, userID int) ([]Role, error) {
	rows, err := st.tx.Query(ctx,
		`SELECT r.id, r.name, r.description, r.created_at, r.updated_at
		 FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = $1
		 ORDER BY r.id`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list user roles: %w", err)
	}
	defer rows.Close()

	roles := []Role{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.Id, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user roles: %w", err)
	}
	return roles, nil
}

func (st *storageTx) ListRolePermissions(ctx context.Context, roleID int) ([]string, error) {
	rows, err := st.tx.Query(ctx,
		`SELECT permission FROM role_permissions WHERE role_id = $1 ORDER BY permission`,
		roleID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list role permissions: %w", err)
	}
	defer rows.Close()

	perms := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan role permission: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate role permissions: %w", err)
	}
	return perms, nil
}

// stripKey removes the "<user_id>:" prefix the module prepends to direct
// permissions (see key()), so callers receive the same bare permission string
// the role table holds.
func stripKey(userID int, permission string) string {
	prefix := fmt.Sprintf("%d:", userID)
	if strings.HasPrefix(permission, prefix) {
		return strings.TrimPrefix(permission, prefix)
	}
	return permission
}

func (st *storageTx) ListDirectPermissions(ctx context.Context, userID int) ([]string, error) {
	rows, err := st.tx.Query(ctx,
		`SELECT permission FROM user_permissions WHERE user_id = $1 ORDER BY permission`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list direct permissions: %w", err)
	}
	defer rows.Close()

	perms := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan direct permission: %w", err)
		}
		perms = append(perms, stripKey(userID, p))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate direct permissions: %w", err)
	}
	return perms, nil
}

func (st *storageTx) ListUserPermissions(ctx context.Context, userID int) ([]string, error) {
	direct, err := st.ListDirectPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	rows, err := st.tx.Query(ctx,
		`SELECT DISTINCT rp.permission
		 FROM role_permissions rp
		 JOIN user_roles ur ON ur.role_id = rp.role_id
		 WHERE ur.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list role-derived permissions: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]struct{}, len(direct))
	merged := make([]string, 0, len(direct))
	for _, p := range direct {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			merged = append(merged, p)
		}
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan role-derived permission: %w", err)
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			merged = append(merged, p)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate role-derived permissions: %w", err)
	}

	sort.Strings(merged)
	return merged, nil
}

func (st *storageTx) Commit() error {
	return st.tx.Commit(context.Background())
}

func (st *storageTx) Rollback() error {
	return st.tx.Rollback(context.Background())
}
