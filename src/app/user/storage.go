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
	query := `SELECT id, username, email, full_name, created_at, updated_at FROM users WHERE id = $1`
	row := st.tx.QueryRow(ctx, query, id)
	user, err := convertUserRow(row)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user by id: %w", err)
	}
	return user, nil
}

func (st *storageTx) GetUserByUsername(ctx context.Context, username string) (User, error) {
	query := `SELECT id, username, email, full_name, created_at, updated_at FROM users WHERE username = $1`
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
