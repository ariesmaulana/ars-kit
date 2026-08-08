package permission_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStorageAddPermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage AddPermission", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				userID     int
				permission string
			}
			type testRow struct {
				name  string
				input *input
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				Users = []DataUser{
					{Idx: 0, ID: 100},
					{Idx: 1, ID: 200},
				}
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				initialPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)

				// Begin transaction and add permission
				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err, r.name)
				defer tx.Rollback()

				err = tx.AddPermission(ctx, r.input.userID, r.input.permission)
				assert.Nil(t, err, r.name)

				// Verify permission exists inside the transaction
				has, err := tx.HasPermission(ctx, r.input.userID, r.input.permission)
				assert.Nil(t, err, r.name)
				assert.True(t, has, r.name)

				// Verify state is unchanged outside the transaction (not committed)
				afterPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)
				assert.Equal(t, initialPerms, afterPerms, r.name)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "AddPermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should add permission for user",
						input: &input{
							userID:     Users[0].ID,
							permission: "read:profile",
						},
					},
					{
						name: "Should add another permission for same user",
						input: &input{
							userID:     Users[0].ID,
							permission: "write:profile",
						},
					},
					{
						name: "Should add permission for different user",
						input: &input{
							userID:     Users[1].ID,
							permission: "read:profile",
						},
					},
				})
			})
		})
	})
}

func TestStorageHasPermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage HasPermission", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				userID     int
				permission string
			}
			type expected struct {
				hasPermission bool
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				Users = []DataUser{
					{Idx: 0, ID: 100},
					{Idx: 1, ID: 200},
				}

				// Seed permissions directly via pool
				app.Helper.AddPermission(ctx, t, Users[0].ID, "read:profile")
				app.Helper.AddPermission(ctx, t, Users[0].ID, "write:profile")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Begin transaction to check permission
				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err, r.name)
				defer tx.Rollback()

				has, err := tx.HasPermission(ctx, r.input.userID, r.input.permission)
				assert.Nil(t, err, r.name)
				assert.Equal(t, r.expected.hasPermission, has, r.name)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "HasPermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Existing permissions =====
					{
						name: "Should find existing permission for user",
						input: &input{
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{
							hasPermission: true,
						},
					},
					{
						name: "Should find another existing permission for same user",
						input: &input{
							userID:     Users[0].ID,
							permission: "write:profile",
						},
						expected: &expected{
							hasPermission: true,
						},
					},

					// ===== Non-existent permissions =====
					{
						name: "Should not find non-existent permission for user that has some permissions",
						input: &input{
							userID:     Users[0].ID,
							permission: "delete:profile",
						},
						expected: &expected{
							hasPermission: false,
						},
					},
					{
						name: "Should not find permission for user without any permissions",
						input: &input{
							userID:     Users[1].ID,
							permission: "read:profile",
						},
						expected: &expected{
							hasPermission: false,
						},
					},
					{
						name: "Should not find permission for non-existent user",
						input: &input{
							userID:     99999,
							permission: "anything",
						},
						expected: &expected{
							hasPermission: false,
						},
					},
				})
			})
		})
	})
}
