package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check to ensure storage implements Storage interface
var _ Storage = (*storage)(nil)

// storage implements the Storage interface
type storage struct {
	pool *pgxpool.Pool
}

// NewStorage creates a new storage instance
func NewStorage(pool *pgxpool.Pool) Storage {
	return &storage{
		pool: pool,
	}
}

func (st *storageTx) ListUsers(ctx context.Context, page, size int, filter, status string) ([]User, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	offset := (page - 1) * size

	countQuery := `
		SELECT count(*) FROM users
		WHERE ($1 = '' OR username ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
		AND ($2 = '' OR status::text = $2)
	`
	var total int
	if err := st.tx.QueryRow(ctx, countQuery, filter, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	query := `
		SELECT id, username, email, full_name, status, email_verified_at, last_login_at, created_at, updated_at
		FROM users
		WHERE ($1 = '' OR username ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
		AND ($4 = '' OR status::text = $4)
		ORDER BY id
		LIMIT $2 OFFSET $3
	`
	rows, err := st.tx.Query(ctx, query, filter, size, offset, status)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		u, err := convertUserRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate user rows: %w", err)
	}
	return users, total, nil
}

// GetUserTokenVersion reads a user's current token_version outside a
// transaction. Used by the JWT middleware to reject tokens minted before a
// security event (password change).
func (s *storage) GetUserTokenVersion(ctx context.Context, id int) (int, error) {
	query := `SELECT token_version FROM users WHERE id = $1`
	var version int
	err := s.pool.QueryRow(ctx, query, id).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get user token version: %w", err)
	}
	return version, nil
}

// BeginTx starts a new database transaction
func (s *storage) BeginTx(ctx context.Context) (StorageTx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &storageTx{
		tx: tx,
	}, nil
}

// Compile-time check to ensure storageTx implements StorageTx interface
var _ StorageTx = (*storageTx)(nil)

// storageTx implements the StorageTx interface
type storageTx struct {
	tx pgx.Tx
}

func (st *storageTx) InsertUser(ctx context.Context, username, email, fullName, password string) (int, StorageErrorType, error) {
	query := `INSERT INTO users (username, email, full_name, password) VALUES ($1, $2, $3, $4) RETURNING id`
	var id int
	err := st.tx.QueryRow(ctx, query, username, email, fullName, password).Scan(&id)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			// 23505 is the PostgreSQL error code for unique_violation
			if pgErr.Code == "23505" || strings.Contains(pgErr.Message, "duplicate key") {
				return 0, ErrTypeUniqueConstraint, fmt.Errorf("failed to insert user: %w", err)
			}
		}
		return 0, ErrTypeCommon, fmt.Errorf("failed to insert user: %w", err)
	}
	return id, ErrTypeNone, nil
}
func (st *storageTx) GetUserById(ctx context.Context, id int) (User, error) {
	query := `	SELECT id, username, email, full_name, status, email_verified_at, last_login_at, created_at, updated_at FROM users WHERE id = $1`
	row := st.tx.QueryRow(ctx, query, id)
	user, err := convertUserRow(row)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id: %w", err)
	}
	return user, nil
}

func (st *storageTx) GetUserByEmail(ctx context.Context, email string) (User, error) {
	query := `SELECT id, username, email, full_name, status, email_verified_at, last_login_at, created_at, updated_at FROM users WHERE LOWER(email) = LOWER($1)`
	row := st.tx.QueryRow(ctx, query, email)
	user, err := convertUserRow(row)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

func (st *storageTx) GetUserByUsername(ctx context.Context, username string) (User, error) {
	query := `SELECT id, username, email, full_name, status, email_verified_at, last_login_at, created_at, updated_at FROM users WHERE username = $1`
	row := st.tx.QueryRow(ctx, query, username)
	user, err := convertUserRow(row)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by username: %w", err)
	}
	return user, nil
}

func (st *storageTx) GetUserPassword(ctx context.Context, id int) (string, error) {
	query := `SELECT password FROM users WHERE id = $1`
	var password string
	err := st.tx.QueryRow(ctx, query, id).Scan(&password)
	if err != nil {
		return "", fmt.Errorf("failed to get user password: %w", err)
	}
	return password, nil
}

func (st *storageTx) UpdateUsername(ctx context.Context, id int, newUsername string) error {
	query := `UPDATE users SET username = $1, updated_at = NOW() WHERE id = $2`
	_, err := st.tx.Exec(ctx, query, newUsername, id)
	if err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}
	return nil
}

func (st *storageTx) UpdatePassword(ctx context.Context, id int, newPassword string) error {
	query := `UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2`
	_, err := st.tx.Exec(ctx, query, newPassword, id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

func (st *storageTx) InsertPasswordHistory(ctx context.Context, userID int, passwordHash string) error {
	query := `INSERT INTO password_history (user_id, password_hash) VALUES ($1, $2)`
	if _, err := st.tx.Exec(ctx, query, userID, passwordHash); err != nil {
		return fmt.Errorf("failed to insert password history: %w", err)
	}
	return nil
}

func (st *storageTx) GetRecentPasswordHashes(ctx context.Context, userID int, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 1
	}
	query := `
		SELECT password_hash
		FROM password_history
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`
	rows, err := st.tx.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent password hashes: %w", err)
	}
	defer rows.Close()

	hashes := make([]string, 0, limit)
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("failed to scan password history row: %w", err)
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate password history rows: %w", err)
	}
	return hashes, nil
}

// LockUserById locks a user row for update and returns the user
// This implements pessimistic locking to prevent concurrent modifications
func (st *storageTx) LockUserById(ctx context.Context, id int) (User, StorageErrorType, error) {
	query := `SELECT id, username, email, full_name, status, email_verified_at, last_login_at, created_at, updated_at FROM users WHERE id = $1 FOR UPDATE`
	row := st.tx.QueryRow(ctx, query, id)
	user, err := convertUserRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrTypeNotFound, fmt.Errorf("failed to lock user by id: %w", err)
		}
		return User{}, ErrTypeCommon, fmt.Errorf("failed to lock user by id: %w", err)
	}
	return user, ErrTypeNone, nil
}

func (st *storageTx) GetUserTokenVersion(ctx context.Context, id int) (int, error) {
	query := `SELECT token_version FROM users WHERE id = $1`
	var version int
	err := st.tx.QueryRow(ctx, query, id).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get user token version: %w", err)
	}
	return version, nil
}

// BumpUserTokenVersion increments a user's token_version, invalidating every
// access and refresh token issued at an earlier version.
func (st *storageTx) BumpUserTokenVersion(ctx context.Context, id int) error {
	query := `UPDATE users SET token_version = token_version + 1, updated_at = NOW() WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to bump user token version: %w", err)
	}
	return nil
}

// InsertRefreshToken records a newly issued refresh token hash. tokenHash must
// be the SHA-256 hash of the opaque token; tokenVersion is a snapshot of
// users.token_version at issuance.
func (st *storageTx) InsertRefreshToken(ctx context.Context, userID int, tokenHash string, tokenVersion int, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, token_version, expires_at) VALUES ($1, $2, $3, $4)`
	_, err := st.tx.Exec(ctx, query, userID, tokenHash, tokenVersion, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to insert refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken reads a refresh token row by its hash, locking it FOR
// UPDATE so two concurrent refreshes with the same token cannot both rotate
// it. Any error (including pgx.ErrNoRows) means the token is unknown.
func (st *storageTx) GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, token_version, expires_at, revoked_at, created_at, updated_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`
	var rt RefreshToken
	err := st.tx.QueryRow(ctx, query, tokenHash).Scan(
		&rt.Id,
		&rt.UserId,
		&rt.TokenHash,
		&rt.TokenVersion,
		&rt.ExpiresAt,
		&rt.RevokedAt,
		&rt.CreatedAt,
		&rt.UpdatedAt,
	)
	if err != nil {
		return RefreshToken{}, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return rt, nil
}

// RevokeRefreshToken marks a refresh token row revoked by id.
func (st *storageTx) RevokeRefreshToken(ctx context.Context, id int) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllUserRefreshTokens marks every active refresh token of a user
// revoked (cleanup when token_version is bumped).
func (st *storageTx) RevokeAllUserRefreshTokens(ctx context.Context, userID int) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW(), updated_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := st.tx.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user refresh tokens: %w", err)
	}
	return nil
}

func convertUserRow(row pgx.Row) (User, error) {
	var user User
	var status string
	var lastLoginAt *time.Time
	err := row.Scan(&user.Id, &user.Username, &user.Email, &user.FullName, &status, &user.EmailVerifiedAt, &lastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	user.Status = UserStatus(status)
	user.LastLoginAt = lastLoginAt
	return user, nil
}

// LockUserLoginState locks the user row for update and returns the persisted
// login-throttle state. NULL columns are scanned into nil pointers.
func (st *storageTx) LockUserLoginState(ctx context.Context, id int) (LoginState, StorageErrorType, error) {
	query := `SELECT failed_login_attempts, last_failed_login_at, locked_until FROM users WHERE id = $1 FOR UPDATE`
	row := st.tx.QueryRow(ctx, query, id)

	var state LoginState
	err := row.Scan(&state.FailedAttempts, &state.LastFailedLoginAt, &state.LockedUntil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoginState{}, ErrTypeNotFound, fmt.Errorf("failed to lock login state: %w", err)
		}
		return LoginState{}, ErrTypeCommon, fmt.Errorf("failed to lock login state: %w", err)
	}
	return state, ErrTypeNone, nil
}

// RecordFailedLogin persists the failed-attempt state computed by the service.
// Callers must hold the row lock (LockUserLoginState) so concurrent attempts
// cannot overwrite each other's increments.
func (st *storageTx) RecordFailedLogin(ctx context.Context, id int, state LoginState) error {
	query := `UPDATE users SET failed_login_attempts = $1, last_failed_login_at = $2, locked_until = $3 WHERE id = $4`
	_, err := st.tx.Exec(ctx, query, state.FailedAttempts, state.LastFailedLoginAt, state.LockedUntil, id)
	if err != nil {
		return fmt.Errorf("failed to record failed login: %w", err)
	}
	return nil
}

// ResetLoginState clears the failed-attempt counter and lock after a
// successful login. Callers must hold the row lock (LockUserLoginState).
func (st *storageTx) ResetLoginState(ctx context.Context, id int) error {
	query := `UPDATE users SET failed_login_attempts = 0, last_failed_login_at = NULL, locked_until = NULL WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to reset login state: %w", err)
	}
	return nil
}

// UpdateLastLogin stamps the user's last_login_at (and updated_at) with the
// given time. The timestamp is passed in by the caller (typically clock.Now())
// so the write is deterministic and testable, rather than using SQL NOW().
func (st *storageTx) UpdateLastLogin(ctx context.Context, id int, at time.Time) error {
	query := `UPDATE users SET last_login_at = $2::timestamptz, updated_at = $2::timestamp WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, id, at)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

func (st *storageTx) UpdateUserStatus(ctx context.Context, id int, status UserStatus) error {
	query := `UPDATE users SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	return nil
}

func (st *storageTx) DeleteUser(ctx context.Context, id int) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

func (st *storageTx) InsertEmailToken(ctx context.Context, userID int, purpose EmailTokenPurpose, tokenHash string, expiresAt time.Time) error {
	query := `INSERT INTO email_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1, $2, $3, $4)`
	_, err := st.tx.Exec(ctx, query, userID, purpose, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to insert email token: %w", err)
	}
	return nil
}

func (st *storageTx) GetEmailToken(ctx context.Context, purpose EmailTokenPurpose, tokenHash string) (EmailToken, error) {
	query := `	SELECT id, user_id, purpose, token_hash, expires_at, used_at, created_at
	FROM email_tokens
	WHERE token_hash = $1 AND purpose = $2
	FOR UPDATE`
	var t EmailToken
	err := st.tx.QueryRow(ctx, query, tokenHash, purpose).Scan(
		&t.Id, &t.UserId, &t.Purpose, &t.TokenHash,
		&t.ExpiresAt, &t.UsedAt, &t.CreatedAt,
	)
	if err != nil {
		return EmailToken{}, fmt.Errorf("failed to get email token: %w", err)
	}
	return t, nil
}

func (st *storageTx) MarkEmailTokenUsed(ctx context.Context, id int) error {
	query := `UPDATE email_tokens SET used_at = NOW() WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark email token used: %w", err)
	}
	return nil
}

func (st *storageTx) UpdateEmailVerified(ctx context.Context, userID int, at time.Time) error {
	query := `UPDATE users SET email_verified_at = $2, updated_at = NOW() WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, userID, at)
	if err != nil {
		return fmt.Errorf("failed to update email verified: %w", err)
	}
	return nil
}

// Commit commits the transaction
func (st *storageTx) Commit() error {
	err := st.tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction
func (st *storageTx) Rollback() error {
	err := st.tx.Rollback(context.Background())
	if err != nil && err != pgx.ErrTxClosed {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}
