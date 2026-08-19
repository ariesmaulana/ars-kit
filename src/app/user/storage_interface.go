package user

import (
	"context"
	"time"
)

type StorageErrorType string

const (
	ErrTypeNone             StorageErrorType = ""
	ErrTypeUniqueConstraint StorageErrorType = "unique_constraint"
	ErrTypeNotFound         StorageErrorType = "not_found"
	ErrTypeCommon           StorageErrorType = "common"
)

// Storage defines the interface for user data access layer
//
// Pool-level (non-transactional) reads live here; everything that writes
// goes through StorageTx so updates participate in a transaction.
type Storage interface {
	// BeginTx starts a new database transaction
	BeginTx(ctx context.Context) (StorageTx, error)

	// GetUserTokenVersion reads a user's current token_version outside a
	// transaction. Used by the JWT middleware to reject tokens minted before
	// a security event (password change).
	GetUserTokenVersion(ctx context.Context, id int) (int, error)
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

	// UpdateUsername updates a user's username and returns the updated row.
	// ErrTypeUniqueConstraint is returned when the new username is already
	// taken (byte-wise or case-insensitively via the LOWER() index).
	UpdateUsername(ctx context.Context, id int, newUsername string) (User, StorageErrorType, error)

	// UpdateFullName updates a user's full_name and returns the updated row.
	UpdateFullName(ctx context.Context, id int, fullName string) (User, StorageErrorType, error)

	// UpdateEmail updates a user's email and returns the updated row.
	// ErrTypeUniqueConstraint is returned when the new email is already in
	// use (byte-wise or case-insensitively via the LOWER() index).
	UpdateEmail(ctx context.Context, id int, email string) (User, StorageErrorType, error)

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

	// GetUserTokenVersion reads a user's current token_version within the
	// transaction.
	GetUserTokenVersion(ctx context.Context, id int) (int, error)

	// BumpUserTokenVersion increments a user's token_version, invalidating
	// every access and refresh token issued at an earlier version. Call it on
	// security-sensitive events (password change).
	BumpUserTokenVersion(ctx context.Context, id int) error

	// InsertRefreshToken records a newly issued refresh token hash. tokenHash
	// must be the SHA-256 hash of the opaque token; tokenVersion is a snapshot
	// of users.token_version at issuance.
	InsertRefreshToken(ctx context.Context, userID int, tokenHash string, tokenVersion int, expiresAt time.Time) error

	// GetRefreshToken reads a refresh token row by its hash, locking it FOR
	// UPDATE so concurrent refreshes with the same token cannot both rotate
	// it. Any error (including pgx.ErrNoRows) means the token is unknown.
	GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error)

	// RevokeRefreshToken marks a refresh token row revoked by id.
	RevokeRefreshToken(ctx context.Context, id int) error

	// RevokeAllUserRefreshTokens marks every active refresh token of a user
	// revoked (cleanup when token_version is bumped).
	RevokeAllUserRefreshTokens(ctx context.Context, userID int) error

	// CountUsers returns the total number of users.
	CountUsers(ctx context.Context) (int, error)

	// ListUsers returns one page of users, ordered by id ascending.
	ListUsers(ctx context.Context, page, pageSize int) ([]User, error)

	// SetUserActive flips a user's is_active flag and returns the updated row.
	// ErrTypeNotFound is returned when no user has the given id.
	SetUserActive(ctx context.Context, id int, isActive bool) (User, StorageErrorType, error)

	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error
}
