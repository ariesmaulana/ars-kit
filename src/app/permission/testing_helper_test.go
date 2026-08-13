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

func (h *TestHelper) AddPermission(ctx context.Context, t *testing.T, userID int, permission string) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO user_permissions (user_id, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, permission,
	)
	assert.Nil(t, err)
}

func (h *TestHelper) GetAllPermissions(ctx context.Context, t *testing.T, userID int) []string {
	rows, err := h.pool.Query(ctx,
		`SELECT permission FROM user_permissions WHERE user_id = $1 ORDER BY permission`,
		userID,
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

func (h *TestHelper) ClearPermissions(ctx context.Context, t *testing.T) {
	_, err := h.pool.Exec(ctx, `DELETE FROM user_permissions`)
	assert.Nil(t, err)
}

type DataUser struct {
	Idx int
	ID  int
}

// ---------------------------------------------------------------------------
// Role fixtures
// ---------------------------------------------------------------------------

// InsertRole inserts a role and returns its id.
func (h *TestHelper) InsertRole(ctx context.Context, t *testing.T, name, description string) int {
	var id int
	err := h.pool.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ($1, $2) RETURNING id`,
		name, description,
	).Scan(&id)
	assert.Nil(t, err)
	return id
}

func (h *TestHelper) AddRolePermissionToRole(ctx context.Context, t *testing.T, roleID int, permission string) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO role_permissions (role_id, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permission,
	)
	assert.Nil(t, err)
}

func (h *TestHelper) AssignRoleToUser(ctx context.Context, t *testing.T, userID, roleID int) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, roleID,
	)
	assert.Nil(t, err)
}

// GetAllRoles returns every role as a map[roleID]Role.
func (h *TestHelper) GetAllRoles(ctx context.Context, t *testing.T) map[int]permission.Role {
	rows, err := h.pool.Query(ctx,
		`SELECT id, name, description, created_at, updated_at FROM roles ORDER BY id`,
	)
	assert.Nil(t, err)
	defer rows.Close()

	roles := make(map[int]permission.Role)
	for rows.Next() {
		var r permission.Role
		err := rows.Scan(&r.Id, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
		assert.Nil(t, err)
		roles[r.Id] = r
	}
	assert.Nil(t, rows.Err())
	return roles
}

// GetUserRoles returns every role assigned to a user.
func (h *TestHelper) GetUserRoles(ctx context.Context, t *testing.T, userID int) []permission.Role {
	rows, err := h.pool.Query(ctx,
		`SELECT r.id, r.name, r.description, r.created_at, r.updated_at
		 FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = $1
		 ORDER BY r.id`,
		userID,
	)
	assert.Nil(t, err)
	defer rows.Close()

	var roles []permission.Role
	for rows.Next() {
		var r permission.Role
		err := rows.Scan(&r.Id, &r.Name, &r.Description, &r.CreatedAt, &r.UpdatedAt)
		assert.Nil(t, err)
		roles = append(roles, r)
	}
	assert.Nil(t, rows.Err())
	return roles
}

// GetRolePermissions returns the bare permissions a role holds.
func (h *TestHelper) GetRolePermissions(ctx context.Context, t *testing.T, roleID int) []string {
	rows, err := h.pool.Query(ctx,
		`SELECT permission FROM role_permissions WHERE role_id = $1 ORDER BY permission`,
		roleID,
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
