package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
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

			suite.Runs(t, "Should find user with different casing (M8 case-insensitive lookup)", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "MixedCaseUser", "mixed@example.com", "Mixed Case", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				retrievedUser, err := tx.GetUserByUsername(ctx, "mixedcaseuser")
				assert.Nil(t, err)
				assert.Equal(t, existingUser.Id, retrievedUser.Id)
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

				updated, errType, err := tx.UpdateUsername(ctx, existingUser.Id, "newusername")
				assert.Nil(t, err)
				assert.Equal(t, user.ErrTypeNone, errType)
				assert.Equal(t, "newusername", updated.Username)

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

func TestStorageUpdateFullName(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateFullName", func() {
			suite.Runs(t, "Should update full name and return the updated row", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "fullnameuser", "fullname@example.com", "Old Name", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				updated, errType, err := tx.UpdateFullName(ctx, existingUser.Id, "New Name")
				assert.Nil(t, err)
				assert.Equal(t, user.ErrTypeNone, errType)
				assert.Equal(t, existingUser.Id, updated.Id)
				assert.Equal(t, "New Name", updated.FullName)
				assert.Equal(t, existingUser.Username, updated.Username)

				err = tx.Commit()
				assert.Nil(t, err)

				fetched := app.Helper.GetUserById(ctx, t, existingUser.Id)
				assert.Equal(t, "New Name", fetched.FullName)
			})

			suite.Runs(t, "Should return not found for a missing user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, errType, err := tx.UpdateFullName(ctx, 99999, "Some Name")
				assert.NotNil(t, err)
				assert.Equal(t, user.ErrTypeNotFound, errType)
			})
		})
	})
}

func TestStorageUpdateEmail(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateEmail", func() {
			suite.Runs(t, "Should update email and return the updated row", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "emailuser", "old@example.com", "Email User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				updated, errType, err := tx.UpdateEmail(ctx, existingUser.Id, "new@example.com")
				assert.Nil(t, err)
				assert.Equal(t, user.ErrTypeNone, errType)
				assert.Equal(t, existingUser.Id, updated.Id)
				assert.Equal(t, "new@example.com", updated.Email)

				err = tx.Commit()
				assert.Nil(t, err)

				fetched := app.Helper.GetUserById(ctx, t, existingUser.Id)
				assert.Equal(t, "new@example.com", fetched.Email)
			})

			suite.Runs(t, "Should reject an email already in use (unique violation)", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				app.Helper.InsertUser(ctx, t, "takenuser", "taken@example.com", "Taken User", "password123")
				existingUser := app.Helper.InsertUser(ctx, t, "emailuser2", "old2@example.com", "Email User 2", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, errType, err := tx.UpdateEmail(ctx, existingUser.Id, "taken@example.com")
				assert.NotNil(t, err)
				assert.Equal(t, user.ErrTypeUniqueConstraint, errType)
			})

			suite.Runs(t, "Should reject a case-variant duplicate email (M8)", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				app.Helper.InsertUser(ctx, t, "takenuser2", "taken@example.com", "Taken User 2", "password123")
				existingUser := app.Helper.InsertUser(ctx, t, "emailuser3", "old3@example.com", "Email User 3", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, errType, err := tx.UpdateEmail(ctx, existingUser.Id, "TAKEN@EXAMPLE.COM")
				assert.NotNil(t, err)
				assert.Equal(t, user.ErrTypeUniqueConstraint, errType)
			})

			suite.Runs(t, "Should return not found for a missing user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, errType, err := tx.UpdateEmail(ctx, 99999, "new@example.com")
				assert.NotNil(t, err)
				assert.Equal(t, user.ErrTypeNotFound, errType)
			})
		})
	})
}

func TestStorageUpdateUsername_UniqueViolation(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateUsername unique violation", func() {
			suite.Runs(t, "Should reject a username already in use", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				app.Helper.InsertUser(ctx, t, "takenuser", "taken@example.com", "Taken User", "password123")
				existingUser := app.Helper.InsertUser(ctx, t, "renameuser", "rename@example.com", "Rename User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, errType, err := tx.UpdateUsername(ctx, existingUser.Id, "takenuser")
				assert.NotNil(t, err)
				assert.Equal(t, user.ErrTypeUniqueConstraint, errType)
			})

			suite.Runs(t, "Should reject a case-variant duplicate username (M8)", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				app.Helper.InsertUser(ctx, t, "TakenUser", "taken2@example.com", "Taken User", "password123")
				existingUser := app.Helper.InsertUser(ctx, t, "renameuser2", "rename2@example.com", "Rename User 2", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				_, errType, err := tx.UpdateUsername(ctx, existingUser.Id, "takenuser")
				assert.NotNil(t, err)
				assert.Equal(t, user.ErrTypeUniqueConstraint, errType)
			})
		})
	})
}
