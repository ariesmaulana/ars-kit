package user

import (
	"context"
	"encoding/json"
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
		if isUniqueViolation(err) {
			return 0, ErrTypeUniqueConstraint, fmt.Errorf("failed to insert user: %w", err)
		}
		return 0, ErrTypeCommon, fmt.Errorf("failed to insert user: %w", err)
	}
	return id, ErrTypeNone, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation
// (SQLSTATE 23505). The same check backs InsertUser, UpdateUsername, and
// UpdateEmail, which all rely on the LOWER(username)/LOWER(email) indexes.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" || strings.Contains(pgErr.Message, "duplicate key")
	}
	return false
}

func (st *storageTx) GetUserById(ctx context.Context, id int) (User, error) {
	query := `SELECT id, username, email, full_name, is_active, created_at, updated_at FROM users WHERE id = $1`
	row := st.tx.QueryRow(ctx, query, id)
	user, err := convertUserRow(row)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id: %w", err)
	}
	return user, nil
}

func (st *storageTx) GetUserByUsername(ctx context.Context, username string) (User, error) {
	// Case-insensitive lookup (M8): the LOWER() unique index guarantees at
	// most one match, so login works with any casing of the stored username.
	query := `SELECT id, username, email, full_name, is_active, created_at, updated_at FROM users WHERE LOWER(username) = LOWER($1)`
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

func (st *storageTx) UpdateUsername(ctx context.Context, id int, newUsername string) (User, StorageErrorType, error) {
	query := `UPDATE users SET username = $1, updated_at = NOW() WHERE id = $2 RETURNING id, username, email, full_name, is_active, created_at, updated_at`
	row := st.tx.QueryRow(ctx, query, newUsername, id)
	user, err := convertUserRow(row)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrTypeUniqueConstraint, fmt.Errorf("failed to update username: %w", err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrTypeNotFound, fmt.Errorf("failed to update username: %w", err)
		}
		return User{}, ErrTypeCommon, fmt.Errorf("failed to update username: %w", err)
	}
	return user, ErrTypeNone, nil
}

func (st *storageTx) UpdateFullName(ctx context.Context, id int, fullName string) (User, StorageErrorType, error) {
	query := `UPDATE users SET full_name = $1, updated_at = NOW() WHERE id = $2 RETURNING id, username, email, full_name, is_active, created_at, updated_at`
	row := st.tx.QueryRow(ctx, query, fullName, id)
	user, err := convertUserRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrTypeNotFound, fmt.Errorf("failed to update full name: %w", err)
		}
		return User{}, ErrTypeCommon, fmt.Errorf("failed to update full name: %w", err)
	}
	return user, ErrTypeNone, nil
}

func (st *storageTx) UpdateEmail(ctx context.Context, id int, email string) (User, StorageErrorType, error) {
	query := `UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2 RETURNING id, username, email, full_name, is_active, created_at, updated_at`
	row := st.tx.QueryRow(ctx, query, email, id)
	user, err := convertUserRow(row)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrTypeUniqueConstraint, fmt.Errorf("failed to update email: %w", err)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrTypeNotFound, fmt.Errorf("failed to update email: %w", err)
		}
		return User{}, ErrTypeCommon, fmt.Errorf("failed to update email: %w", err)
	}
	return user, ErrTypeNone, nil
}

func (st *storageTx) UpdatePassword(ctx context.Context, id int, newPassword string) error {
	query := `UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2`
	_, err := st.tx.Exec(ctx, query, newPassword, id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

// LockUserById locks a user row for update and returns the user
// This implements pessimistic locking to prevent concurrent modifications
func (st *storageTx) LockUserById(ctx context.Context, id int) (User, StorageErrorType, error) {
	query := `SELECT id, username, email, full_name, is_active, created_at, updated_at FROM users WHERE id = $1 FOR UPDATE`
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

func (st *storageTx) InsertAuditLog(ctx context.Context, entry AuditEntry) (int64, error) {
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal audit metadata: %w", err)
	}

	query := `
		INSERT INTO audit_log (event, actor_id, target_user_id, metadata)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id
	`
	var id int64
	err = st.tx.QueryRow(ctx, query, entry.Event, entry.ActorId, entry.TargetUserId, string(metadata)).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert audit log: %w", err)
	}
	return id, nil
}

func (st *storageTx) CountAuditLogs(ctx context.Context, filter AuditLogFilter) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM audit_log
		WHERE ($1 = '' OR event = $1)
		  AND ($2 = 0 OR actor_id = $2)
		  AND ($3 = 0 OR target_user_id = $3)
	`
	var count int
	err := st.tx.QueryRow(ctx, query, filter.Event, filter.ActorId, filter.TargetUserId).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs: %w", err)
	}
	return count, nil
}

func (st *storageTx) ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]AuditEntry, error) {
	query := `
		SELECT id, event, actor_id, target_user_id, metadata, created_at
		FROM audit_log
		WHERE ($1 = '' OR event = $1)
		  AND ($2 = 0 OR actor_id = $2)
		  AND ($3 = 0 OR target_user_id = $3)
		ORDER BY id DESC
		LIMIT $4 OFFSET $5
	`
	rows, err := st.tx.Query(ctx, query, filter.Event, filter.ActorId, filter.TargetUserId, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]AuditEntry, 0)
	for rows.Next() {
		var entry AuditEntry
		var metadata []byte
		if err := rows.Scan(&entry.Id, &entry.Event, &entry.ActorId, &entry.TargetUserId, &metadata, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &entry.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal audit metadata: %w", err)
			}
		} else {
			entry.Metadata = map[string]any{}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate audit log rows: %w", err)
	}
	return entries, nil
}

func convertUserRow(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.Id, &user.Username, &user.Email, &user.FullName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return User{}, err
	}
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

// CountUsers returns the total number of users.
func (st *storageTx) CountUsers(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var total int
	err := st.tx.QueryRow(ctx, query).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return total, nil
}

// ListUsers returns one page of users, ordered by id ascending so the admin
// list is stable across requests.
func (st *storageTx) ListUsers(ctx context.Context, page, pageSize int) ([]User, error) {
	query := `SELECT id, username, email, full_name, is_active, created_at, updated_at
		FROM users
		ORDER BY id
		LIMIT $1 OFFSET $2`
	offset := (page - 1) * pageSize
	rows, err := st.tx.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0, pageSize)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Id, &user.Username, &user.Email, &user.FullName, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate users: %w", err)
	}
	return users, nil
}

// SetUserActive flips a user's is_active flag and returns the updated row.
func (st *storageTx) SetUserActive(ctx context.Context, id int, isActive bool) (User, StorageErrorType, error) {
	query := `UPDATE users SET is_active = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, username, email, full_name, is_active, created_at, updated_at`
	row := st.tx.QueryRow(ctx, query, isActive, id)
	user, err := convertUserRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrTypeNotFound, fmt.Errorf("failed to set user active: %w", err)
		}
		return User{}, ErrTypeCommon, fmt.Errorf("failed to set user active: %w", err)
	}
	return user, ErrTypeNone, nil
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
