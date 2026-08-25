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

// AddKnownPermission inserts a row into the permissions catalog, simulating
// the SOP step of registering a new feature's permission.
func (h *TestHelper) AddKnownPermission(ctx context.Context, t *testing.T, permission string) {
	_, err := h.pool.Exec(ctx,
		`INSERT INTO permissions (permission) VALUES ($1) ON CONFLICT DO NOTHING`,
		permission,
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

func (h *TestHelper) ClearPermissions(ctx context.Context, t *testing.T) {
	_, err := h.pool.Exec(ctx, `DELETE FROM user_permissions`)
	assert.Nil(t, err)
}

type DataUser struct {
	Idx int
	ID  int
}

// DataPermission represents a permission catalog fixture for testing
type DataPermission struct {
	Idx        int // Index in the fixture array
	Permission string
}
