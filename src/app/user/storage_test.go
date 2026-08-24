package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/ariesmaulana/ars-kit/src/clock"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageInsertUser(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage InsertUser", func() {
			suite.Runs(t, "Should insert user successfully", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				id, errType, err := tx.InsertUser(ctx, "testuser", "test@example.com", "Test User", "hashedpassword")
				assert.Nil(t, err)
				assert.NotZero(t, id)
				assert.Equal(t, user.ErrTypeNone, errType)

				err = tx.Commit()
				assert.Nil(t, err)

				insertedUser := app.Helper.GetUserById(ctx, t, id)
				assert.Equal(t, "testuser", insertedUser.Username)
				assert.Equal(t, "test@example.com", insertedUser.Email)
				assert.Equal(t, "Test User", insertedUser.FullName)
			})
		})
	})
}

func TestStorageGetUserById(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage GetUserById", func() {
			suite.Runs(t, "Should get existing user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "existinguser", "existing@example.com", "Existing User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				retrievedUser, err := tx.GetUserById(ctx, int(existingUser.Id))
				assert.Nil(t, err)
				assert.Equal(t, existingUser.Id, retrievedUser.Id)
				assert.Equal(t, existingUser.Username, retrievedUser.Username)
				assert.Equal(t, existingUser.Email, retrievedUser.Email)
				assert.Equal(t, existingUser.FullName, retrievedUser.FullName)
			})

			suite.Runs(t, "Should return error for non-existent user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, err = tx.GetUserById(ctx, 99999)
				assert.NotNil(t, err)
			})
		})
	})
}

func TestStorageGetUserByUsername(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage GetUserByUsername", func() {
			suite.Runs(t, "Should get existing user by username", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "findme", "findme@example.com", "Find Me", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				retrievedUser, err := tx.GetUserByUsername(ctx, "findme")
				assert.Nil(t, err)
				assert.Equal(t, existingUser.Id, retrievedUser.Id)
				assert.Equal(t, existingUser.Username, retrievedUser.Username)
				assert.Equal(t, existingUser.Email, retrievedUser.Email)
			})

			suite.Runs(t, "Should return error for non-existent username", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, err = tx.GetUserByUsername(ctx, "nonexistent")
				assert.NotNil(t, err)
			})
		})
	})
}

func TestStorageGetUserPassword(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage GetUserPassword", func() {
			suite.Runs(t, "Should get user password", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "passuser", "pass@example.com", "Pass User", "mysecretpassword")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				password, err := tx.GetUserPassword(ctx, existingUser.Id)
				assert.Nil(t, err)
				assert.Equal(t, "mysecretpassword", password)
			})

			suite.Runs(t, "Should return error for non-existent user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, err = tx.GetUserPassword(ctx, 99999)
				assert.NotNil(t, err)
			})
		})
	})
}

func TestStorageUpdateUsername(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateUsername", func() {
			suite.Runs(t, "Should update username successfully", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "oldusername", "old@example.com", "Old User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				err = tx.UpdateUsername(ctx, existingUser.Id, "newusername")
				assert.Nil(t, err)

				err = tx.Commit()
				assert.Nil(t, err)

				updatedUser := app.Helper.GetUserById(ctx, t, existingUser.Id)
				assert.Equal(t, "newusername", updatedUser.Username)
			})

			// Note: PostgreSQL UPDATE doesn't error on 0 rows affected
			// Would need to check affected rows count to validate
		})
	})
}

func TestStorageUpdatePassword(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdatePassword", func() {
			suite.Runs(t, "Should update password successfully", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "passchange", "passchange@example.com", "Pass Change", "oldpassword")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				err = tx.UpdatePassword(ctx, existingUser.Id, "newpassword")
				assert.Nil(t, err)

				err = tx.Commit()
				assert.Nil(t, err)

				newPassword := app.Helper.GetUserPassword(ctx, t, existingUser.Id)
				assert.Equal(t, "newpassword", newPassword)
			})

			// Note: PostgreSQL UPDATE doesn't error on 0 rows affected
			// Would need to check affected rows count to validate
		})
	})
}

func TestStorageTransactionRollback(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage Transaction Rollback", func() {
			suite.Runs(t, "Should rollback changes when transaction is not committed", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				initialCount := app.Helper.CountUsers(ctx, t)

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				_, _, err = tx.InsertUser(ctx, "rollbacktest", "rollback@example.com", "Rollback Test", "password")
				assert.Nil(t, err)

				tx.Rollback()

				finalCount := app.Helper.CountUsers(ctx, t)
				assert.Equal(t, initialCount, finalCount)
			})
		})
	})
}

func TestStorageTransactionCommit(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage Transaction Commit", func() {
			suite.Runs(t, "Should persist changes when transaction is committed", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				initialCount := app.Helper.CountUsers(ctx, t)

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, _, err = tx.InsertUser(ctx, "committest", "commit@example.com", "Commit Test", "password")
				assert.Nil(t, err)

				err = tx.Commit()
				assert.Nil(t, err)

				finalCount := app.Helper.CountUsers(ctx, t)
				assert.Equal(t, initialCount+1, finalCount)
			})
		})
	})
}

func TestStorageLockUserLoginState(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage LockUserLoginState", func() {
			suite.Runs(t, "Returns zero state for a fresh user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "freshstate", "fresh@example.com", "Fresh State", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				state, errType, err := tx.LockUserLoginState(ctx, u.Id)
				assert.Nil(t, err)
				assert.Equal(t, user.ErrTypeNone, errType)
				assert.Equal(t, 0, state.FailedAttempts)
				assert.Nil(t, state.LastFailedLoginAt)
				assert.Nil(t, state.LockedUntil)
			})

			suite.Runs(t, "Returns ErrTypeNotFound for a non-existent user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, errType, err := tx.LockUserLoginState(ctx, 99999)
				assert.NotNil(t, err)
				assert.Equal(t, user.ErrTypeNotFound, errType)
			})
		})
	})
}

func TestStorageRecordFailedLogin(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage RecordFailedLogin", func() {
			suite.Runs(t, "Persists the failed-attempt state when committed", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "recordfail", "record@example.com", "Record Fail", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				now := time.Now().UTC()
				lockedUntil := now.Add(15 * time.Minute)
				err = tx.RecordFailedLogin(ctx, u.Id, user.LoginState{
					FailedAttempts:    2,
					LastFailedLoginAt: &now,
					LockedUntil:       &lockedUntil,
				})
				assert.Nil(t, err)
				err = tx.Commit()
				assert.Nil(t, err)

				state := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 2, state.FailedAttempts)
				assert.NotNil(t, state.LastFailedLoginAt)
				assert.NotNil(t, state.LockedUntil)
			})

			suite.Runs(t, "Rolls back the state when the transaction is not committed", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "rollbackfail", "rollback@example.com", "Rollback Fail", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				err = tx.RecordFailedLogin(ctx, u.Id, user.LoginState{FailedAttempts: 1})
				assert.Nil(t, err)
				tx.Rollback()

				state := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 0, state.FailedAttempts, "uncommitted state must not persist")
			})
		})
	})
}

func TestStorageResetLoginState(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage ResetLoginState", func() {
			suite.Runs(t, "Clears the counter and lock", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "resetstate", "reset@example.com", "Reset State", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				now := time.Now().UTC()
				err = tx.RecordFailedLogin(ctx, u.Id, user.LoginState{
					FailedAttempts:    4,
					LastFailedLoginAt: &now,
					LockedUntil:       &now,
				})
				assert.Nil(t, err)

				err = tx.ResetLoginState(ctx, u.Id)
				assert.Nil(t, err)
				err = tx.Commit()
				assert.Nil(t, err)

				state := app.Helper.GetLoginState(ctx, t, u.Id)
				assert.Equal(t, 0, state.FailedAttempts)
				assert.Nil(t, state.LastFailedLoginAt)
				assert.Nil(t, state.LockedUntil)
			})
		})
	})
}

func TestStorageUpdateLastLogin(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateLastLogin", func() {
			suite.Runs(t, "Stamps last_login_at on commit and is idempotent", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "lastlogin", "lastlogin@example.com", "Last Login", "password123")

				// Fresh user has never logged in.
				assert.Nil(t, app.Helper.GetLastLoginAt(ctx, t, u.Id))

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				assert.Nil(t, tx.UpdateLastLogin(ctx, u.Id, clock.Now().UTC()))
				// Uncommitted change is not visible outside the transaction.
				assert.Nil(t, app.Helper.GetLastLoginAt(ctx, t, u.Id))
				assert.Nil(t, tx.Commit())

				stamped := app.Helper.GetLastLoginAt(ctx, t, u.Id)
				assert.NotNil(t, stamped)

				// A second call advances the timestamp within a new transaction.
				tx2, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				assert.Nil(t, tx2.UpdateLastLogin(ctx, u.Id, clock.Now().UTC()))
				assert.Nil(t, tx2.Commit())
				stamped2 := app.Helper.GetLastLoginAt(ctx, t, u.Id)
				assert.NotNil(t, stamped2)
				assert.True(t, stamped2.After(*stamped))
			})
		})
	})
}

func TestStorageRefreshTokenLifecycle(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage RefreshToken", func() {
			suite.Runs(t, "Should insert, lock-read, revoke, bump version and revoke-all", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "rttokenuser", "rt@example.com", "RT User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				defer tx.Rollback()

				// A fresh user starts at token_version 0.
				version, err := tx.GetUserTokenVersion(ctx, existingUser.Id)
				require.NoError(t, err)
				assert.Equal(t, 0, version)

				expiresAt := time.Now().Add(time.Hour)
				require.NoError(t, tx.InsertRefreshToken(ctx, existingUser.Id, "abcd1234", version, expiresAt))

				rt, err := tx.GetRefreshToken(ctx, "abcd1234")
				require.NoError(t, err)
				assert.Equal(t, existingUser.Id, rt.UserId)
				assert.Equal(t, "abcd1234", rt.TokenHash)
				assert.Equal(t, version, rt.TokenVersion)
				assert.Nil(t, rt.RevokedAt)

				require.NoError(t, tx.RevokeRefreshToken(ctx, rt.Id))
				rtAfter, err := tx.GetRefreshToken(ctx, "abcd1234")
				require.NoError(t, err)
				require.NotNil(t, rtAfter.RevokedAt)

				// Bumping the version and revoking all rows invalidates every
				// refresh token of the user in one shot.
				require.NoError(t, tx.InsertRefreshToken(ctx, existingUser.Id, "efgh5678", version, expiresAt))
				require.NoError(t, tx.BumpUserTokenVersion(ctx, existingUser.Id))
				require.NoError(t, tx.RevokeAllUserRefreshTokens(ctx, existingUser.Id))

				bumped, err := tx.GetUserTokenVersion(ctx, existingUser.Id)
				require.NoError(t, err)
				assert.Equal(t, 1, bumped)

				require.NoError(t, tx.Commit())

				assert.Equal(t, 2, app.Helper.CountRefreshTokens(ctx, t, existingUser.Id))
				assert.Equal(t, 0, app.Helper.CountActiveRefreshTokens(ctx, t, existingUser.Id))
			})

			suite.Runs(t, "Should error when the token hash is unknown", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				defer tx.Rollback()

				_, err = tx.GetRefreshToken(ctx, "nosuchhash")
				assert.Error(t, err)
			})
		})
	})
}
