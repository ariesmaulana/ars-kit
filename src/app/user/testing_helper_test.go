package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// TestHelper provides utility methods for test fixtures
type TestHelper struct {
	pool *pgxpool.Pool
}

// NewTestHelper creates a new helper instance
func NewTestHelper(pool *pgxpool.Pool) *TestHelper {
	return &TestHelper{
		pool: pool,
	}
}

// DataUser represents a user fixture for testing
type DataUser struct {
	Idx      int // Index in the fixture array
	Id       int // Actual database ID (populated after insert)
	Username string
	Email    string
	FullName string
	Password string // Plain text password for testing
}

// InsertUser inserts a single user and returns it
func (h *TestHelper) InsertUser(ctx context.Context, t *testing.T, username, email, fullName, password string) *user.User {
	query := `
		INSERT INTO users (username, email, full_name, password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, username, email, full_name, created_at, updated_at
	`

	var u user.User
	err := h.pool.QueryRow(ctx, query, username, email, fullName, password).Scan(
		&u.Id,
		&u.Username,
		&u.Email,
		&u.FullName,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	assert.Nil(t, err)

	return &u
}

// InsertUserWithHashedPassword inserts a user with a plain text password that gets hashed
func (h *TestHelper) InsertUserWithHashedPassword(ctx context.Context, t *testing.T, username, email, fullName, plainPassword string) *user.User {
	// Hash the password using bcrypt
	hashedPassword, err := hashPassword(plainPassword)
	assert.Nil(t, err, "Failed to hash password")

	return h.InsertUser(ctx, t, username, email, fullName, hashedPassword)
}

// hashPassword hashes a plain text password using bcrypt
func hashPassword(password string) (string, error) {
	// Import golang.org/x/crypto/bcrypt in the import section
	// Using bcrypt default cost (10)
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// InsertUsers inserts multiple users and returns them
func (h *TestHelper) InsertUsers(ctx context.Context, t *testing.T, users []UserFixture) []*user.User {
	result := make([]*user.User, 0, len(users))

	for _, fixture := range users {
		u := h.InsertUser(ctx, t, fixture.Username, fixture.Email, fixture.FullName, fixture.Password)
		result = append(result, u)
	}

	return result
}

// UserFixture represents a user fixture for testing
type UserFixture struct {
	Username string
	Email    string
	FullName string
	Password string
}

// ClearUsers removes all users from the database
func (h *TestHelper) ClearUsers(ctx context.Context, t *testing.T) {
	_, err := h.pool.Exec(ctx, "DELETE FROM users")
	assert.Nil(t, err)
}

// GetUserById retrieves a user by ID
func (h *TestHelper) GetUserById(ctx context.Context, t *testing.T, id int) *user.User {
	query := `
		SELECT id, username, email, full_name, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var u user.User
	err := h.pool.QueryRow(ctx, query, id).Scan(
		&u.Id,
		&u.Username,
		&u.Email,
		&u.FullName,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	assert.Nil(t, err)

	return &u
}

// GetUserByUsername retrieves a user by username
func (h *TestHelper) GetUserByUsername(ctx context.Context, t *testing.T, username string) *user.User {
	query := `
		SELECT id, username, email, full_name, created_at, updated_at
		FROM users
		WHERE username = $1
	`

	var u user.User
	err := h.pool.QueryRow(ctx, query, username).Scan(
		&u.Id,
		&u.Username,
		&u.Email,
		&u.FullName,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	assert.Nil(t, err)

	return &u
}

// GetUserPassword retrieves a user's password by ID
func (h *TestHelper) GetUserPassword(ctx context.Context, t *testing.T, id int) string {
	query := `SELECT password FROM users WHERE id = $1`
	var password string
	err := h.pool.QueryRow(ctx, query, id).Scan(&password)
	assert.Nil(t, err)
	return password
}

// CountUsers returns the total number of users
func (h *TestHelper) CountUsers(ctx context.Context, t *testing.T) int {
	var count int
	err := h.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	assert.Nil(t, err)
	return count
}

// GetAllUsers retrieves all users as a map indexed by user ID
func (h *TestHelper) GetAllUsers(ctx context.Context, t *testing.T) map[int]user.User {
	query := `
		SELECT id, username, email, full_name, created_at, updated_at
		FROM users
		ORDER BY id
	`

	rows, err := h.pool.Query(ctx, query)
	assert.Nil(t, err)
	defer rows.Close()

	users := make(map[int]user.User)
	for rows.Next() {
		var u user.User
		err := rows.Scan(
			&u.Id,
			&u.Username,
			&u.Email,
			&u.FullName,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		assert.Nil(t, err)
		users[u.Id] = u
	}

	assert.Nil(t, rows.Err())

	return users
}

// GetPool returns the connection pool
func (h *TestHelper) GetPool() *pgxpool.Pool {
	return h.pool
}

// GetLoginState reads the persisted login-throttle state for a user.
func (h *TestHelper) GetLoginState(ctx context.Context, t *testing.T, id int) user.LoginState {
	query := `SELECT failed_login_attempts, last_failed_login_at, locked_until FROM users WHERE id = $1`

	var state user.LoginState
	err := h.pool.QueryRow(ctx, query, id).Scan(
		&state.FailedAttempts,
		&state.LastFailedLoginAt,
		&state.LockedUntil,
	)
	assert.Nil(t, err)
	return state
}

// SetLoginState overwrites the persisted login-throttle state for a user,
// letting tests simulate expired windows and locks without waiting.
func (h *TestHelper) SetLoginState(ctx context.Context, t *testing.T, id int, state user.LoginState) {
	query := `UPDATE users SET failed_login_attempts = $1, last_failed_login_at = $2, locked_until = $3 WHERE id = $4`
	_, err := h.pool.Exec(ctx, query, state.FailedAttempts, state.LastFailedLoginAt, state.LockedUntil, id)
	assert.Nil(t, err)
}

// createGrantedPermissionCheck returns a mock result where the user holds the
// requested permission.
func createGrantedPermissionCheck() *permission.CheckPermissionOutput {
	return &permission.CheckPermissionOutput{
		Success:       true,
		Message:       "Permission check completed",
		HasPermission: true,
	}
}

// createDeniedPermissionCheck returns a mock result where the user does not
// hold the requested permission.
func createDeniedPermissionCheck() *permission.CheckPermissionOutput {
	return &permission.CheckPermissionOutput{
		Success:       true,
		Message:       "Permission check completed",
		HasPermission: false,
	}
}

// createFailedPermissionCheck returns a mock result where the permission
// module itself fails to confirm the check.
func createFailedPermissionCheck() *permission.CheckPermissionOutput {
	return &permission.CheckPermissionOutput{
		Success: false,
		Message: "Failed to check permission",
	}
}

// createSuccessfulGrant returns a mock result where the permission module
// granted the permission.
func createSuccessfulGrant() *permission.GrantPermissionOutput {
	return &permission.GrantPermissionOutput{
		Success: true,
		Message: "Permission granted successfully",
	}
}

// createFailedGrant returns a mock result where the permission module failed
// to grant the permission.
func createFailedGrant() *permission.GrantPermissionOutput {
	return &permission.GrantPermissionOutput{
		Success: false,
		Message: "Failed to grant permission",
	}
}

// createSuccessfulRevoke returns a mock result where the permission module
// revoked the permission.
func createSuccessfulRevoke() *permission.RevokePermissionOutput {
	return &permission.RevokePermissionOutput{
		Success: true,
		Message: "Permission revoked successfully",
	}
}

// createFailedRevoke returns a mock result where the permission module failed
// to revoke the permission.
func createFailedRevoke() *permission.RevokePermissionOutput {
	return &permission.RevokePermissionOutput{
		Success: false,
		Message: "Failed to revoke permission",
	}
}
