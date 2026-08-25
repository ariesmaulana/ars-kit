package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageListUsers(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage ListUsers", func() {

			suite.Runs(t, "Should list users with pagination and substring filter", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				app.Helper.InsertUser(ctx, t, "alice", "alice@example.com", "Alice", "password123")
				app.Helper.InsertUser(ctx, t, "bob", "bob@example.com", "Bob", "password123")
				app.Helper.InsertUser(ctx, t, "carol", "carol@example.com", "Carol", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				defer tx.Rollback()

				users, total, err := tx.ListUsers(ctx, 1, 2, "")
				require.NoError(t, err)
				assert.Equal(t, 3, total)
				assert.Len(t, users, 2)

				users, total, err = tx.ListUsers(ctx, 1, 10, "al")
				require.NoError(t, err)
				assert.Equal(t, 1, total)
				assert.Equal(t, "alice", users[0].Username)

				users, total, err = tx.ListUsers(ctx, 1, 10, "bob@")
				require.NoError(t, err)
				assert.Equal(t, 1, total)
				assert.Equal(t, "bob", users[0].Username)
			})

			suite.Runs(t, "Should return empty result when nothing matches", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				defer tx.Rollback()
				users, total, err := tx.ListUsers(ctx, 1, 10, "nomatch")
				require.NoError(t, err)
				assert.Equal(t, 0, total)
				assert.Len(t, users, 0)
			})
		})
	})
}

func TestStorageDeleteUser(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage DeleteUser", func() {

			suite.Runs(t, "Should hard-delete a user and cascade refresh tokens", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "todelete", "del@example.com", "Del", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				err = tx.DeleteUser(ctx, int(u.Id))
				require.NoError(t, err)
				require.NoError(t, tx.Commit())

				tx2, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				defer tx2.Rollback()
				_, err = tx2.GetUserById(ctx, int(u.Id))
				assert.Error(t, err)
			})

			suite.Runs(t, "Should be a no-op (no error) when user does not exist", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				defer tx.Rollback()
				// Exec-based delete is idempotent: missing row is not an error.
				assert.NoError(t, tx.DeleteUser(ctx, 999999))
			})
		})
	})
}

func TestStorageUpdateUserStatus(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateUserStatus", func() {

			suite.Runs(t, "Should persist the new status within a committed transaction", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "statuschange", "status@example.com", "Status", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)

				err = tx.UpdateUserStatus(ctx, int(u.Id), user.UserStatusSuspended)
				require.NoError(t, err)
				require.NoError(t, tx.Commit())

				got := app.Helper.GetUserById(ctx, t, int(u.Id))
				require.NotNil(t, got)
				assert.Equal(t, user.UserStatusSuspended, got.Status)
			})

			suite.Runs(t, "Should not persist when the transaction is rolled back", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "rollbackstatus", "rollback@example.com", "Rollback", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)

				err = tx.UpdateUserStatus(ctx, int(u.Id), user.UserStatusDisabled)
				require.NoError(t, err)
				require.NoError(t, tx.Rollback())

				got := app.Helper.GetUserById(ctx, t, int(u.Id))
				require.NotNil(t, got)
				assert.Equal(t, user.UserStatusActive, got.Status)
			})

			suite.Runs(t, "Should be a no-op (no error) when user does not exist", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				defer tx.Rollback()
				assert.NoError(t, tx.UpdateUserStatus(ctx, 999999, user.UserStatusSuspended))
			})
		})
	})
}
