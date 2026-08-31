package permission_test

import (
	"context"
	"testing"

	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
)

func TestStorageRoleQueries(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage role queries", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				Users = []DataUser{
					{Idx: 0, ID: 100},
					{Idx: 1, ID: 200},
				}

				// editor carries read:profile; user 0 holds it.
				app.Helper.AddRole(ctx, t, "editor")
				app.Helper.AddRolePermission(ctx, t, "editor", "read:profile")
				app.Helper.SetUserRole(ctx, t, Users[0].ID, "editor")
			})

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "UserHasPermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				scenarios := []struct {
					name       string
					userID     int
					permission string
					expected   bool
				}{
					{"role of the user carries the permission", Users[0].ID, "read:profile", true},
					{"no role of the user carries the permission", Users[0].ID, "write:profile", false},
					{"user without any role", Users[1].ID, "read:profile", false},
					{"non-existent user", 99999, "read:profile", false},
				}

				for _, sc := range scenarios {
					tx, err := app.Storage.BeginTx(ctx)
					assert.Nil(t, err, sc.name)

					has, err := tx.UserHasPermission(ctx, sc.userID, sc.permission)
					assert.Nil(t, err, sc.name)
					assert.Equal(t, sc.expected, has, sc.name)
					assert.Nil(t, tx.Rollback(), sc.name)
				}
			})

			suite.Run(t, "UserHasRole scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				scenarios := []struct {
					name     string
					userID   int
					roleName string
					expected bool
				}{
					{"user holds the role", Users[0].ID, "editor", true},
					{"user does not hold the role", Users[1].ID, "editor", false},
					{"role does not exist", Users[0].ID, "nonexistent_role", false},
				}

				for _, sc := range scenarios {
					tx, err := app.Storage.BeginTx(ctx)
					assert.Nil(t, err, sc.name)

					has, err := tx.UserHasRole(ctx, sc.userID, sc.roleName)
					assert.Nil(t, err, sc.name)
					assert.Equal(t, sc.expected, has, sc.name)
					assert.Nil(t, tx.Rollback(), sc.name)
				}
			})

			suite.Run(t, "RoleExists scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				exists, err := tx.RoleExists(ctx, "editor")
				assert.Nil(t, err)
				assert.True(t, exists)

				exists, err = tx.RoleExists(ctx, "nonexistent_role")
				assert.Nil(t, err)
				assert.False(t, exists)

				// Built-in roles seeded by migration.
				exists, err = tx.RoleExists(ctx, "super_user")
				assert.Nil(t, err)
				assert.True(t, exists)
			})
		})
	})
}

func TestStorageAssignUnassignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage AssignRole / UnassignRole", func() {

			suite.Runs(t, "Should persist an assignment on commit and stay idempotent", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initPermissionApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				err = tx.AssignRole(ctx, 100, "member")
				assert.Nil(t, err)
				err = tx.AssignRole(ctx, 100, "member") // duplicate: no-op
				assert.Nil(t, err)
				assert.Nil(t, tx.Commit())

				roles := app.Helper.GetUserRoles(ctx, t, 100)
				assert.Equal(t, []string{"member"}, roles)
			})

			suite.Runs(t, "Should not persist when the transaction is rolled back", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initPermissionApp(appCtx)
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				err = tx.AssignRole(ctx, 100, "member")
				assert.Nil(t, err)
				assert.Nil(t, tx.Rollback())

				assert.Empty(t, app.Helper.GetUserRoles(ctx, t, 100))
			})

			suite.Runs(t, "Should remove only the targeted assignment", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initPermissionApp(appCtx)
				ctx := context.Background()

				app.Helper.SetUserRole(ctx, t, 100, "member")
				app.Helper.SetUserRole(ctx, t, 100, "super_user")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				err = tx.UnassignRole(ctx, 100, "member")
				assert.Nil(t, err)
				assert.Nil(t, tx.Commit())

				assert.Equal(t, []string{"super_user"}, app.Helper.GetUserRoles(ctx, t, 100))
			})
		})
	})
}
