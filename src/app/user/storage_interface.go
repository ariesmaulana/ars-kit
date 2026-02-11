package user

import (
	"context"
)

type StorageErrorType string

const (
	ErrTypeNone             StorageErrorType = ""
	ErrTypeUniqueConstraint StorageErrorType = "unique_constraint"
	ErrTypeNotFound         StorageErrorType = "not_found"
	ErrTypeCommon           StorageErrorType = "common"
)

// Storage defines the interface for user data access layer
type Storage interface {
	// BeginTx starts a new database transaction
	BeginTx(ctx context.Context) (StorageTx, error)
}

// StorageTx defines the interface for transactional user operations
type StorageTx interface {
	// InsertUser inserts a new user and returns the user ID
	InsertUser(ctx context.Context, username, email, fullName, password string) (int, StorageErrorType, error)

	// GetUserById retrieves a user by ID
	GetUserById(ctx context.Context, id int) (User, error)

	// GetUserByUsername retrieves a user by username
	GetUserByUsername(ctx context.Context, username string) (User, error)

	// GetUserPassword retrieves a user's hashed password
	GetUserPassword(ctx context.Context, id int) (string, error)

	// UpdateUsername updates a user's username
	UpdateUsername(ctx context.Context, id int, newUsername string) error

	// UpdatePassword updates a user's password
	UpdatePassword(ctx context.Context, id int, newPassword string) error

	InsertMember(ctx context.Context, userId int, name string, monthlyIncome int) (int, error)

	GetMembersByUserId(ctx context.Context, userId int) ([]Member, error)

	GetMemberById(ctx context.Context, memberId int) (Member, StorageErrorType, error)

	UpdateMemberInfo(ctx context.Context, memberId int, name string, monthlyIncome int) error

	DeleteMemberById(ctx context.Context, memberId int) error

	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error
}
