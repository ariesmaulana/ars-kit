package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	query := `SELECT id, username, email, full_name, created_at, updated_at FROM users WHERE id = $1`
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
	query := `SELECT id, username, email, full_name, created_at, updated_at FROM users WHERE LOWER(username) = LOWER($1)`
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
	query := `UPDATE users SET username = $1, updated_at = NOW() WHERE id = $2 RETURNING id, username, email, full_name, created_at, updated_at`
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
	query := `UPDATE users SET full_name = $1, updated_at = NOW() WHERE id = $2 RETURNING id, username, email, full_name, created_at, updated_at`
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
	query := `UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2 RETURNING id, username, email, full_name, created_at, updated_at`
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
	query := `SELECT id, username, email, full_name, created_at, updated_at FROM users WHERE id = $1 FOR UPDATE`
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

func convertUserRow(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.Id, &user.Username, &user.Email, &user.FullName, &user.CreatedAt, &user.UpdatedAt)
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
