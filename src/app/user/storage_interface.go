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

	// LockUserById locks a user row for update and returns the user
	// This implements pessimistic locking to prevent concurrent modifications
	LockUserById(ctx context.Context, id int) (User, StorageErrorType, error)

	// LockUserLoginState locks a user row for update and returns its persisted
	// login-throttle state (failed attempts, last failure, lock expiry). Callers
	// must hold the returned lock while reading and writing the state so
	// concurrent login attempts for the same account are serialized.
	LockUserLoginState(ctx context.Context, id int) (LoginState, StorageErrorType, error)

	// RecordFailedLogin persists an incremented failed-attempt counter, the
	// last-failure timestamp and, when the account just crossed the threshold,
	// the lock expiry. Callers must hold the row lock (LockUserLoginState).
	RecordFailedLogin(ctx context.Context, id int, state LoginState) error

	// ResetLoginState clears the failed-attempt counter and lock after a
	// successful login. Callers must hold the row lock (LockUserLoginState).
	ResetLoginState(ctx context.Context, id int) error

	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error
}
