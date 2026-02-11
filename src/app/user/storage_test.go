package user_test

import (
	"context"
	"testing"

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

func TestStorageInsertMember(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage InsertMember", func() {
			suite.Runs(t, "Should insert member successfully", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "memberuser", "member@example.com", "Member User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				id, err := tx.InsertMember(ctx, existingUser.Id, "Test Member", 1000)
				assert.Nil(t, err)
				assert.NotZero(t, id)

				err = tx.Commit()
				assert.Nil(t, err)

				insertedMember := app.Helper.GetMemberById(ctx, t, id)
				assert.Equal(t, existingUser.Id, insertedMember.UserId)
				assert.Equal(t, "Test Member", insertedMember.Name)
				assert.Equal(t, 1000, insertedMember.MonthlyIncome)

			})
		})
	})
}

func TestStorageGetMembersByUserId(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage GetMembersByUserId", func() {
			suite.Runs(t, "Should get members for user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "userwithmembers", "withmembers@example.com", "User With Members", "password123")

				member1 := app.Helper.InsertMember(ctx, t, existingUser.Id, "Member One", 5000000)
				member2 := app.Helper.InsertMember(ctx, t, existingUser.Id, "Member Two", 6000000)

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				members, err := tx.GetMembersByUserId(ctx, existingUser.Id)
				assert.Nil(t, err)
				assert.Len(t, members, 2)
				assert.Equal(t, member1.Id, members[0].Id)
				assert.Equal(t, member1.Name, members[0].Name)
				assert.Equal(t, member2.Id, members[1].Id)
				assert.Equal(t, member2.Name, members[1].Name)
			})

			suite.Runs(t, "Should return empty list for user with no members", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "nomembers", "nomembers@example.com", "No Members", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				members, err := tx.GetMembersByUserId(ctx, existingUser.Id)
				assert.Nil(t, err)
				assert.Len(t, members, 0)
			})
		})
	})
}

func TestStorageUpdateMemberInfo(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateMemberInfo", func() {
			suite.Runs(t, "Should update member info successfully", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "infouser", "info@example.com", "Info User", "password123")
				existingMember := app.Helper.InsertMember(ctx, t, existingUser.Id, "Original Name", 5000000)

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				err = tx.UpdateMemberInfo(ctx, existingMember.Id, "Updated Name", 7000000)
				assert.Nil(t, err)

				err = tx.Commit()
				assert.Nil(t, err)

				updatedMember := app.Helper.GetMemberById(ctx, t, existingMember.Id)
				assert.Equal(t, "Updated Name", updatedMember.Name)
				assert.Equal(t, 7000000, updatedMember.MonthlyIncome)
			})

		})
	})
}

func TestStorageDeleteMemberById(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage DeleteMemberById", func() {
			suite.Runs(t, "Should delete member successfully", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				existingUser := app.Helper.InsertUser(ctx, t, "deleteuser", "delete@example.com", "Delete User", "password123")
				existingMember := app.Helper.InsertMember(ctx, t, existingUser.Id, "Member To Delete", 5000000)

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				err = tx.DeleteMemberById(ctx, existingMember.Id)
				assert.Nil(t, err)

				err = tx.Commit()
				assert.Nil(t, err)

			})

			// Note: PostgreSQL DELETE doesn't error on 0 rows affected
			// Would need to check affected rows count to validate
		})
	})
}
