package permission_test

import (
	"context"
	"testing"

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
