package user_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
)

// smallThrottle returns a login-throttle policy with small values so the
// lockout scenarios complete fast and deterministically instead of waiting on
// the 15-minute production defaults.
func smallThrottle() user.LoginThrottleConfig {
	return user.LoginThrottleConfig{
		MaxFailedAttempts: 3,
		FailedWindow:      time.Hour,
		LockoutDuration:   time.Hour,
	}
}

func TestUserLoginLockout(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Login lockout", func() {

			suite.Runs(t, "Locks the account after MaxFailedAttempts failed logins", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserAppWithThrottle(appCtx, smallThrottle())
				ctx := context.Background()

				u := app.Helper.InsertUserWithHashedPassword(ctx, t, "lockme", "lockme@example.com", "Lock Me", "password123")

				// Attempts 1..3 fail and count toward the threshold.
				for i := 0; i < 3; i++ {
					out := app.Service.Login(ctx, &user.LoginInput{
						TraceId:  "trace-lockout",
						Username: "lockme",
						Password: "wrongpassword",
					})
					assert.False(t, out.Success, "attempt %d should fail", i+1)
					assert.Equal(t, "Invalid username or password", out.Message, "attempt %d", i+1)
					assert.Equal(t, user.ErrorCodeUnauthorized, out.ErrorCode, "attempt %d", i+1)
				}

				// The 4th attempt, even with the correct password, is rejected
				// because the account is now locked.
				out := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-lockout",
					Username: "lockme",
					Password: "password123",
				})
				assert.False(t, out.Success)
				assert.Contains(t, out.Message, "Account temporarily locked")
				assert.Equal(t, user.ErrorCodeLocked, out.ErrorCode)

				// Lockout state is exposed on the response for clients.
				assert.NotNil(t, out.LockedUntil, "locked response should expose LockedUntil")
				assert.True(t, out.LockedUntil.After(time.Now().UTC()), "LockedUntil should be in the future")
				assert.Greater(t, out.RetryAfterSeconds, 0, "locked response should expose RetryAfterSeconds")

				// The persisted state matches: threshold reached, lock set.
				state := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 3, state.FailedAttempts)
				assert.NotNil(t, state.LockedUntil)
				assert.True(t, state.LockedUntil.After(time.Now().UTC()), "locked_until should be in the future")
			})

			suite.Runs(t, "Does not count or extend the lock while locked", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserAppWithThrottle(appCtx, smallThrottle())
				ctx := context.Background()

				u := app.Helper.InsertUserWithHashedPassword(ctx, t, "stayslocked", "stayslocked@example.com", "Stays Locked", "password123")

				// Reach the threshold.
				for i := 0; i < 3; i++ {
					app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "stayslocked", Password: "wrong"})
				}

				lockedBefore := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.NotNil(t, lockedBefore.LockedUntil)

				// More failures while locked must neither count nor extend the lock.
				for i := 0; i < 2; i++ {
					out := app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "stayslocked", Password: "wrong"})
					assert.Equal(t, user.ErrorCodeLocked, out.ErrorCode, "attempt %d", i+1)
				}

				lockedAfter := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, lockedBefore.FailedAttempts, lockedAfter.FailedAttempts)
				assert.Equal(t, *lockedBefore.LockedUntil, *lockedAfter.LockedUntil, "lock must not be extended by further attempts")
			})

			suite.Runs(t, "Unlocks after the lock expires", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserAppWithThrottle(appCtx, smallThrottle())
				ctx := context.Background()

				u := app.Helper.InsertUserWithHashedPassword(ctx, t, "expiredlock", "expiredlock@example.com", "Expired Lock", "password123")

				// Reach the threshold, then force the lock into the past as if
				// the lockout duration had elapsed.
				for i := 0; i < 3; i++ {
					app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "expiredlock", Password: "wrong"})
				}
				expired := time.Now().UTC().Add(-2 * time.Hour)
				app.Helper.SetLoginState(ctx, t, u.Id, user.LoginState{
					FailedAttempts:    3,
					LastFailedLoginAt: &expired,
					LockedUntil:       &expired,
				})

				out := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-expired",
					Username: "expiredlock",
					Password: "password123",
				})
				assert.True(t, out.Success, "login should succeed once the lock expires")
				assert.Equal(t, "Login successful", out.Message)

				state := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 0, state.FailedAttempts)
				assert.Nil(t, state.LockedUntil)
			})
		})
	})
}

func TestUserLoginResetOnSuccess(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Login reset on success", func() {

			suite.Runs(t, "A successful login clears the failed-attempt counter", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserAppWithThrottle(appCtx, smallThrottle())
				ctx := context.Background()

				u := app.Helper.InsertUserWithHashedPassword(ctx, t, "resetter", "resetter@example.com", "Resetter", "password123")

				// Two failures (below the threshold of 3).
				for i := 0; i < 2; i++ {
					app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "resetter", Password: "wrong"})
				}
				state := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 2, state.FailedAttempts)
				assert.NotNil(t, state.LastFailedLoginAt)
				assert.Nil(t, state.LockedUntil)

				// Correct login resets the counter.
				out := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-reset",
					Username: "resetter",
					Password: "password123",
				})
				assert.True(t, out.Success)

				state = app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 0, state.FailedAttempts)
				assert.Nil(t, state.LastFailedLoginAt)
				assert.Nil(t, state.LockedUntil)

				// Counting restarts from zero after the reset.
				app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "resetter", Password: "wrong"})
				state = app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 1, state.FailedAttempts)
			})
		})
	})
}

func TestUserLoginWindowExpiry(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Login counting window", func() {

			suite.Runs(t, "A failure after the window elapsed restarts the counter", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserAppWithThrottle(appCtx, smallThrottle())
				ctx := context.Background()

				u := app.Helper.InsertUserWithHashedPassword(ctx, t, "windowuser", "window@example.com", "Window User", "password123")

				// Two failures, then push the last failure outside the 1h window
				// as if the user stopped trying for a while.
				for i := 0; i < 2; i++ {
					app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "windowuser", Password: "wrong"})
				}
				old := time.Now().UTC().Add(-2 * time.Hour)
				app.Helper.SetLoginState(ctx, t, u.Id, user.LoginState{
					FailedAttempts:    2,
					LastFailedLoginAt: &old,
				})

				// One more failure: the counter resets (2 -> 0 -> 1) instead of
				// reaching the threshold of 3 and locking the account.
				out := app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "windowuser", Password: "wrong"})
				assert.False(t, out.Success)
				assert.Equal(t, "Invalid username or password", out.Message)

				state := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 1, state.FailedAttempts, "counter should restart after the window elapsed")
				assert.Nil(t, state.LockedUntil)
			})
		})
	})
}

func TestUserLoginLockedUnknownUser(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Login unknown user", func() {

			suite.Runs(t, "Unknown usernames keep the generic message", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserAppWithThrottle(appCtx, smallThrottle())
				ctx := context.Background()

				out := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-unknown",
					Username: "doesnotexist",
					Password: "password123",
				})
				assert.False(t, out.Success)
				assert.Equal(t, "Invalid username or password", out.Message)
				assert.Equal(t, user.ErrorCodeUnauthorized, out.ErrorCode)
			})

			suite.Runs(t, "Locked message mentions the remaining time", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserAppWithThrottle(appCtx, smallThrottle())
				ctx := context.Background()

				app.Helper.InsertUserWithHashedPassword(ctx, t, "msgcheck", "msgcheck@example.com", "Msg Check", "password123")
				for i := 0; i < 3; i++ {
					app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "msgcheck", Password: "wrong"})
				}

				out := app.Service.Login(ctx, &user.LoginInput{TraceId: "t", Username: "msgcheck", Password: "password123"})
				assert.True(t, strings.Contains(out.Message, "minute(s)"), "message should include remaining minutes, got %q", out.Message)
				assert.Equal(t, user.ErrorCodeLocked, out.ErrorCode)
				assert.NotNil(t, out.LockedUntil)
				assert.Greater(t, out.RetryAfterSeconds, 0)
			})
		})
	})
}
