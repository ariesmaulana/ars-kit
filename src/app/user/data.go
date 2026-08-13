package user

import "time"

type User struct {
	Id        int
	Username  string
	Email     string
	FullName  string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
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
