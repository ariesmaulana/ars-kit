package user_test

import (
	"context"
	"testing"

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

// InsertMember inserts a single member and returns it
func (h *TestHelper) InsertMember(ctx context.Context, t *testing.T, userId int, name string, monthlyIncome int) *user.Member {
	query := `
		INSERT INTO members (user_id, name, monthly_income, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, user_id, name, monthly_income, created_at, updated_at
	`

	var m user.Member
	err := h.pool.QueryRow(ctx, query, userId, name, monthlyIncome).Scan(
		&m.Id,
		&m.UserId,
		&m.Name,
		&m.MonthlyIncome,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	assert.Nil(t, err)

	return &m
}

// InsertMembers inserts multiple members and returns them
func (h *TestHelper) InsertMembers(ctx context.Context, t *testing.T, members []MemberFixture) []*user.Member {
	result := make([]*user.Member, 0, len(members))

	for _, fixture := range members {
		m := h.InsertMember(ctx, t, fixture.UserId, fixture.Name, fixture.MonthlyIncome)
		result = append(result, m)
	}

	return result
}

// MemberFixture represents a member fixture for testing
type MemberFixture struct {
	UserId        int
	Name          string
	MonthlyIncome int
}

// DataMember represents a member fixture for testing
type DataMember struct {
	Idx           int // Index in the fixture array
	Id            int // Actual database ID (populated after insert)
	UserId        int
	Name          string
	MonthlyIncome int
}

// GetMemberById retrieves a member by ID
func (h *TestHelper) GetMemberById(ctx context.Context, t *testing.T, id int) user.Member {
	query := `
		SELECT id, user_id, name, monthly_income, created_at, updated_at
		FROM members
		WHERE id = $1
	`

	var m user.Member
	err := h.pool.QueryRow(ctx, query, id).Scan(
		&m.Id,
		&m.UserId,
		&m.Name,
		&m.MonthlyIncome,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	assert.Nil(t, err)

	return m
}

// GetAllMembers retrieves all members from the database
func (h *TestHelper) GetAllMembers(ctx context.Context, t *testing.T) map[int]user.Member {
	query := `
		SELECT id, user_id, name, monthly_income, created_at, updated_at
		FROM members
		ORDER BY id
	`

	rows, err := h.pool.Query(ctx, query)
	assert.Nil(t, err)
	defer rows.Close()

	members := make(map[int]user.Member)
	for rows.Next() {
		var m user.Member
		err := rows.Scan(
			&m.Id,
			&m.UserId,
			&m.Name,
			&m.MonthlyIncome,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		assert.Nil(t, err)
		members[m.Id] = m
	}

	assert.Nil(t, rows.Err())
	return members
}
