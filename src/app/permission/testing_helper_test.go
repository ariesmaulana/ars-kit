package permission_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

type TestHelper struct {
	pool *pgxpool.Pool
}

func NewTestHelper(pool *pgxpool.Pool) *TestHelper {
	return &TestHelper{pool: pool}
}

// AddRole inserts a role, simulating the SOP step of registering a role.
func (h *TestHelper) AddRole(ctx context.Context, t *testing.T, roleName string) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO roles (name) VALUES ($1) ON CONFLICT DO NOTHING`,
		roleName,
	)
	assert.Nil(t, err)
}

// AddRolePermission grants a permission to a role, simulating the SOP step
// of defining what a role means.
func (h *TestHelper) AddRolePermission(ctx context.Context, t *testing.T, roleName string, permission string) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO role_permissions (role_id, permission)
		 SELECT r.id, $2 FROM roles r WHERE r.name = $1
		 ON CONFLICT DO NOTHING`,
		roleName, permission,
	)
	assert.Nil(t, err)
}

// AddKnownPermission inserts a row into the permissions catalog, simulating
// the SOP step of registering a new feature's permission.
func (h *TestHelper) AddKnownPermission(ctx context.Context, t *testing.T, perm string) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO permissions (permission) VALUES ($1) ON CONFLICT DO NOTHING`,
		perm,
	)
	assert.Nil(t, err)
}

// GetRolePermissions returns the permissions carried by a role, ordered.
func (h *TestHelper) GetRolePermissions(ctx context.Context, t *testing.T, roleName string) []string {
	rows, err := h.pool.Query(ctx,
		`SELECT rp.permission FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
		 WHERE r.name = $1 ORDER BY rp.permission`,
		roleName,
	)
	assert.Nil(t, err)
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		err := rows.Scan(&p)
		assert.Nil(t, err)
		perms = append(perms, p)
	}
	assert.Nil(t, rows.Err())
	return perms
}

// SetUserRole assigns a role to a user directly, simulating a bootstrap /
// SOP grant outside the service layer.
func (h *TestHelper) SetUserRole(ctx context.Context, t *testing.T, userID int, roleName string) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id)
		 SELECT $1, r.id FROM roles r WHERE r.name = $2
		 ON CONFLICT DO NOTHING`,
		userID, roleName,
	)
	assert.Nil(t, err)
}

// GetUserRoles returns the role names held by a user, ordered by name.
func (h *TestHelper) GetUserRoles(ctx context.Context, t *testing.T, userID int) []string {
	rows, err := h.pool.Query(ctx,
		`SELECT r.name FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		 WHERE ur.user_id = $1 ORDER BY r.name`,
		userID,
	)
	assert.Nil(t, err)
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		assert.Nil(t, err)
		roles = append(roles, name)
	}
	assert.Nil(t, rows.Err())
	return roles
}

// GetPermissionAudit returns all audit rows recorded against a target user,
// ordered by insertion. ActorId is nil for system-initiated actions.
func (h *TestHelper) GetPermissionAudit(ctx context.Context, t *testing.T, targetID int) []permission.PermissionAudit {
	rows, err := h.pool.Query(ctx,
		`SELECT id, actor_id, target_id, permission, action, created_at FROM permission_audit WHERE target_id = $1 ORDER BY id`,
		targetID,
	)
	assert.Nil(t, err)
	defer rows.Close()

	var audits []permission.PermissionAudit
	for rows.Next() {
		var a permission.PermissionAudit
		err := rows.Scan(&a.Id, &a.ActorId, &a.TargetId, &a.Permission, &a.Action, &a.CreatedAt)
		assert.Nil(t, err)
		audits = append(audits, a)
	}
	assert.Nil(t, rows.Err())
	return audits
}

type DataUser struct {
	Idx int
	ID  int
}
