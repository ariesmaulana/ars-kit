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

func (st *storageTx) InsertMember(ctx context.Context, userId int, name string, monthlyIncome int) (int, error) {
	query := `INSERT INTO members (user_id, name, monthly_income) VALUES ($1, $2, $3) RETURNING id`
	var id int
	err := st.tx.QueryRow(ctx, query, userId, name, monthlyIncome).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert member: %w", err)
	}
	return id, nil
}

func (st *storageTx) GetMembersByUserId(ctx context.Context, userId int) ([]Member, error) {
	query := `SELECT id, user_id, name, monthly_income, created_at, updated_at FROM members WHERE user_id = $1`
	rows, err := st.tx.Query(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get members: %w", err)
	}
	defer rows.Close()

	return convertListMember(rows)
}

func (st *storageTx) GetMemberById(ctx context.Context, memberId int) (Member, StorageErrorType, error) {
	query := `SELECT id, user_id, name, monthly_income, created_at, updated_at FROM members WHERE id = $1`
	row := st.tx.QueryRow(ctx, query, memberId)
	member, err := convertMember(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrTypeNotFound, fmt.Errorf("failed to get member by id: %w", err)
		}
		return Member{}, ErrTypeCommon, fmt.Errorf("failed to get member by id: %w", err)
	}
	return member, ErrTypeNone, nil
}

func (st *storageTx) UpdateMemberInfo(ctx context.Context, memberId int, name string, monthlyIncome int) error {
	query := `UPDATE members SET name = $1, monthly_income = $2, updated_at = NOW() WHERE id = $3`
	_, err := st.tx.Exec(ctx, query, name, monthlyIncome, memberId)
	if err != nil {
		return fmt.Errorf("failed to update member info: %w", err)
	}
	return nil
}

func (st *storageTx) DeleteMemberById(ctx context.Context, memberId int) error {
	query := `DELETE FROM members WHERE id = $1`
	_, err := st.tx.Exec(ctx, query, memberId)
	if err != nil {
		return fmt.Errorf("failed to delete member: %w", err)
	}
	return nil
}

func convertUserRow(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.Id, &user.Username, &user.Email, &user.FullName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func convertListMember(rows pgx.Rows) ([]Member, error) {
	var members []Member
	for rows.Next() {
		member, err := convertMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over rows: %w", err)
	}
	return members, nil
}

func convertMember(row pgx.Row) (Member, error) {
	var member Member
	err := row.Scan(&member.Id, &member.UserId, &member.Name, &member.MonthlyIncome, &member.CreatedAt, &member.UpdatedAt)
	if err != nil {
		return Member{}, fmt.Errorf("failed to scan member: %w", err)
	}
	return member, nil
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
