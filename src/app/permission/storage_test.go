package permission_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
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

func TestStorageCreateRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage CreateRole", func() {

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				// Seed an "admin" role so the duplicate-name row can be exercised.
				app.Helper.InsertRole(ctx, t, "admin", "Seeded")
			})
			type input struct {
				name        string
				description string
			}
			type expected struct {
				roleNameTaken bool
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialRoles := app.Helper.GetAllRoles(ctx, t)

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err, r.name)
				defer tx.Rollback()

				id, err := tx.CreateRole(ctx, r.input.name, r.input.description)
				if r.expected.roleNameTaken {
					assert.Equal(t, permission.ErrRoleNameTaken, err, r.name)
					return
				}
				assert.Nil(t, err, r.name)
				assert.Greater(t, id, 0, r.name)

				// Verify the role is visible inside the transaction
				role, err := tx.GetRoleById(ctx, id)
				assert.Nil(t, err, r.name)
				assert.Equal(t, r.input.name, role.Name, r.name)
				assert.Equal(t, r.input.description, role.Description, r.name)

				// Verify state is unchanged outside the transaction (not committed)
				afterRoles := app.Helper.GetAllRoles(ctx, t)
				assert.Equal(t, initialRoles, afterRoles, r.name)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "CreateRole scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should create role",
						input: &input{
							name:        "editor",
							description: "Can edit",
						},
						expected: &expected{},
					},
					{
						name: "Should create role without description",
						input: &input{
							name: "viewer",
						},
						expected: &expected{},
					},
					{
						name: "Should fail on duplicate role name",
						input: &input{
							name:        "admin",
							description: "Duplicate",
						},
						expected: &expected{
							roleNameTaken: true,
						},
					},
				})
			})
		})
	})
}

func TestStorageRolePermissions(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage role permissions", func() {

			// ========== 1. Declare Fixture Variables ==========
			var RoleID int

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				RoleID = app.Helper.InsertRole(ctx, t, "editor", "")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp) {
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				// Add two permissions to the role
				err = tx.AddRolePermission(ctx, RoleID, "user:profile_update")
				assert.Nil(t, err)
				err = tx.AddRolePermission(ctx, RoleID, "user:password_update")
				assert.Nil(t, err)

				perms, err := tx.ListRolePermissions(ctx, RoleID)
				assert.Nil(t, err)
				assert.Equal(t, []string{"user:password_update", "user:profile_update"}, perms)

				// Remove one and verify
				err = tx.RemoveRolePermission(ctx, RoleID, "user:password_update")
				assert.Nil(t, err)
				perms, err = tx.ListRolePermissions(ctx, RoleID)
				assert.Nil(t, err)
				assert.Equal(t, []string{"user:profile_update"}, perms)

				// Verify state is unchanged outside the transaction (not committed)
				outsidePerms := app.Helper.GetRolePermissions(ctx, t, RoleID)
				assert.Empty(t, outsidePerms)
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "Role permissions scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runtest(t, app)
			})
		})
	})
}

func TestStorageAssignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage AssignRole", func() {

			// ========== 1. Declare Fixture Variables ==========
			var RoleID int

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				RoleID = app.Helper.InsertRole(ctx, t, "editor", "")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp) {
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				err = tx.AssignRole(ctx, 100, RoleID)
				assert.Nil(t, err)

				roles, err := tx.ListUserRoles(ctx, 100)
				assert.Nil(t, err)
				assert.Len(t, roles, 1)
				assert.Equal(t, RoleID, roles[0].Id)
				assert.Equal(t, "editor", roles[0].Name)

				// Verify a user without assignment has no roles
				roles, err = tx.ListUserRoles(ctx, 200)
				assert.Nil(t, err)
				assert.Len(t, roles, 0)

				// Unassign and verify
				err = tx.UnassignRole(ctx, 100, RoleID)
				assert.Nil(t, err)
				roles, err = tx.ListUserRoles(ctx, 100)
				assert.Nil(t, err)
				assert.Len(t, roles, 0)

				// Verify state is unchanged outside the transaction (not committed)
				outsideRoles := app.Helper.GetUserRoles(ctx, t, 100)
				assert.Len(t, outsideRoles, 0)
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "AssignRole scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runtest(t, app)
			})
		})
	})
}

func TestStorageHasRolePermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage HasRolePermission", func() {

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				// user 100: role granting user:profile_update
				editorRoleID := app.Helper.InsertRole(ctx, t, "editor", "")
				app.Helper.AddRolePermissionToRole(ctx, t, editorRoleID, "user:profile_update")
				app.Helper.AssignRoleToUser(ctx, t, 100, editorRoleID)

				// user 200: role granting super_user (wildcard)
				adminRoleID := app.Helper.InsertRole(ctx, t, "admin", "")
				app.Helper.AddRolePermissionToRole(ctx, t, adminRoleID, permission.PermissionSuperUser)
				app.Helper.AssignRoleToUser(ctx, t, 200, adminRoleID)
			})

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

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err, r.name)
				defer tx.Rollback()

				has, err := tx.HasRolePermission(ctx, r.input.userID, r.input.permission)
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
			suite.Run(t, "HasRolePermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should find permission granted by role",
						input: &input{
							userID:     100,
							permission: "user:profile_update",
						},
						expected: &expected{hasPermission: true},
					},
					{
						name: "Should not find permission not granted by role",
						input: &input{
							userID:     100,
							permission: "user:password_update",
						},
						expected: &expected{hasPermission: false},
					},
					{
						name: "Should find any permission for role holding super_user",
						input: &input{
							userID:     200,
							permission: "report:view",
						},
						expected: &expected{hasPermission: true},
					},
					{
						name: "Should not find permission for user without roles",
						input: &input{
							userID:     300,
							permission: "user:profile_update",
						},
						expected: &expected{hasPermission: false},
					},
				})
			})
		})
	})
}

func TestStorageListUserPermissions(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage ListUserPermissions", func() {

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				// user 100: direct perms + overlapping role perm + unique role perm
				app.Helper.AddPermission(ctx, t, 100, "100:user:profile_update")
				app.Helper.AddPermission(ctx, t, 100, "100:user:password_update")

				editorRoleID := app.Helper.InsertRole(ctx, t, "editor", "")
				app.Helper.AddRolePermissionToRole(ctx, t, editorRoleID, "user:profile_update")
				app.Helper.AssignRoleToUser(ctx, t, 100, editorRoleID)

				viewerRoleID := app.Helper.InsertRole(ctx, t, "viewer", "")
				app.Helper.AddRolePermissionToRole(ctx, t, viewerRoleID, "report:view")
				app.Helper.AssignRoleToUser(ctx, t, 100, viewerRoleID)
			})

			// ========== 2. Define Test Structures ==========
			type input struct {
				userID int
			}
			type expected struct {
				permissions []string
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err, r.name)
				defer tx.Rollback()

				perms, err := tx.ListUserPermissions(ctx, r.input.userID)
				assert.Nil(t, err, r.name)
				assert.Equal(t, r.expected.permissions, perms, r.name)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "ListUserPermissions scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should merge direct and role permissions deduplicated and sorted",
						input: &input{
							userID: 100,
						},
						expected: &expected{
							permissions: []string{"report:view", "user:password_update", "user:profile_update"},
						},
					},
					{
						name: "Should return empty for user without permissions",
						input: &input{
							userID: 200,
						},
						expected: &expected{
							permissions: []string{},
						},
					},
				})
			})
		})
	})
}
