package user

import "time"

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusDisabled  UserStatus = "disabled"
	UserStatusSuspended UserStatus = "suspended"
)

type User struct {
	Id               int
	Username         string
	Email            string
	FullName         string
	Status           UserStatus
	EmailVerifiedAt  *time.Time
	LastLoginAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// EmailTokenPurpose identifies what a single-purpose email token is for.
type EmailTokenPurpose string

const (
	// EmailTokenPurposePasswordReset is the forgot-password reset token.
	EmailTokenPurposePasswordReset EmailTokenPurpose = "password_reset"
	// EmailTokenPurposeEmailVerification is the account email-verification token.
	EmailTokenPurposeEmailVerification EmailTokenPurpose = "email_verification"
)

// EmailToken is a single-purpose one-time token (password reset or email
// verification). Only its SHA-256 hash is stored; the plaintext token is sent
// to the user and never persisted.
type EmailToken struct {
	Id        int
	UserId    int
	Purpose   EmailTokenPurpose
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// LoginState is the persisted per-account login-throttle state.
//
// FailedAttempts counts consecutive failed logins inside the counting window.
// LastFailedLoginAt anchors that window: a failure that happens after the
// window has elapsed resets the counter. LockedUntil is non-nil while the
// account is locked (set when FailedAttempts reaches the configured
// threshold); nil means the account is not locked.
type LoginState struct {
	FailedAttempts    int
	LastFailedLoginAt *time.Time
	LockedUntil       *time.Time
}

// RefreshToken is the server-side record of an issued refresh token. Only the
// SHA-256 hash of the opaque token is stored, never the token itself.
// TokenVersion snapshots users.token_version at issuance; a refresh is only
// accepted while the snapshot matches the user's current version.
type RefreshToken struct {
	Id           int
	UserId       int
	TokenHash    string
	TokenVersion int
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
