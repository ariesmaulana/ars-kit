package user

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// policy returns a *service wired only with a throttle policy. Storage and
// permission service stay nil: applyFailedAttempt does not touch them.
func policy(cfg LoginThrottleConfig) *service {
	return NewService(nil, nil, cfg, nil).(*service)
}

func TestNewServiceFallsBackToDefaultThrottle(t *testing.T) {
	t.Run("zero config gets defaults", func(t *testing.T) {
		s := policy(LoginThrottleConfig{})
		assert.Equal(t, DefaultLoginThrottleConfig(), s.throttle)
	})

	t.Run("partially invalid config gets defaults", func(t *testing.T) {
		s := policy(LoginThrottleConfig{MaxFailedAttempts: 3}) // window and lockout zero
		assert.Equal(t, DefaultLoginThrottleConfig(), s.throttle)
	})

	t.Run("valid config is kept", func(t *testing.T) {
		cfg := LoginThrottleConfig{MaxFailedAttempts: 3, FailedWindow: time.Hour, LockoutDuration: time.Hour}
		s := policy(cfg)
		assert.Equal(t, cfg, s.throttle)
	})
}

func TestApplyFailedAttempt(t *testing.T) {
	cfg := LoginThrottleConfig{
		MaxFailedAttempts: 3,
		FailedWindow:      time.Hour,
		LockoutDuration:   15 * time.Minute,
	}
	s := policy(cfg)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	t.Run("first failure counts and anchors the window", func(t *testing.T) {
		state := s.applyFailedAttempt(LoginState{}, now)
		assert.Equal(t, 1, state.FailedAttempts)
		require.NotNil(t, state.LastFailedLoginAt)
		assert.Equal(t, now, *state.LastFailedLoginAt)
		assert.Nil(t, state.LockedUntil)
	})

	t.Run("failures below the threshold do not lock", func(t *testing.T) {
		state := s.applyFailedAttempt(LoginState{FailedAttempts: 1, LastFailedLoginAt: &now}, now.Add(time.Minute))
		assert.Equal(t, 2, state.FailedAttempts)
		assert.Nil(t, state.LockedUntil)
	})

	t.Run("reaching the threshold locks for the configured duration", func(t *testing.T) {
		lockedAt := now.Add(10 * time.Minute)
		state := s.applyFailedAttempt(LoginState{FailedAttempts: 2, LastFailedLoginAt: &now}, lockedAt)
		assert.Equal(t, 3, state.FailedAttempts)
		require.NotNil(t, state.LockedUntil)
		assert.Equal(t, lockedAt.Add(15*time.Minute), *state.LockedUntil)
	})

	t.Run("a failure after the window elapsed restarts the counter", func(t *testing.T) {
		old := now.Add(-2 * time.Hour) // older than the 1h window
		state := s.applyFailedAttempt(LoginState{FailedAttempts: 2, LastFailedLoginAt: &old}, now)
		assert.Equal(t, 1, state.FailedAttempts, "stale counter must reset before counting")
		assert.Nil(t, state.LockedUntil)
	})

	t.Run("an expired lock is cleared and counting restarts", func(t *testing.T) {
		expired := now.Add(-30 * time.Minute)
		state := s.applyFailedAttempt(LoginState{
			FailedAttempts:    3,
			LastFailedLoginAt: &expired,
			LockedUntil:       &expired,
		}, now)
		assert.Equal(t, 1, state.FailedAttempts)
		assert.Nil(t, state.LockedUntil)
	})

	t.Run("an active lock is left untouched", func(t *testing.T) {
		lockedUntil := now.Add(15 * time.Minute)
		lastFail := now.Add(-time.Minute)
		before := LoginState{FailedAttempts: 3, LastFailedLoginAt: &lastFail, LockedUntil: &lockedUntil}
		after := s.applyFailedAttempt(before, now.Add(time.Minute))
		assert.Equal(t, before, after, "attempts while locked must not count or extend the lock")
	})
}
