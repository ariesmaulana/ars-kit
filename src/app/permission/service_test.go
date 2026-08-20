package permission_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/stretchr/testify/assert"
)

func TestUserGrantPermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User GrantPermission", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId    string
				userID     int
				permission string
			}
			type expected struct {
				success bool
				message string
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
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				initialPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)

				// Execute the service method being tested
				output := app.Service.GrantPermission(ctx, &permission.GrantPermissionInput{
					TraceId:    r.input.traceId,
					UserID:     r.input.userID,
					Permission: r.input.permission,
				})

				// Get state AFTER operation
				afterPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)

				// Assert response matches expectations
				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no permissions were modified on failure
					assert.Equal(t, initialPerms, afterPerms, r.name)
					return
				}

				// For successful grant, verify permission was added
				assert.Equal(t, len(initialPerms)+1, len(afterPerms), r.name)
				assert.Contains(t, afterPerms, fmt.Sprintf("%d:%s", r.input.userID, r.input.permission), r.name)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "GrantPermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should grant permission successfully",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success: true,
							message: "Permission granted successfully",
						},
					},
					{
						name: "Should grant multiple permissions to same user",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "write:profile",
						},
						expected: &expected{
							success: true,
							message: "Permission granted successfully",
						},
					},
					{
						name: "Should grant permission to different user",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[1].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success: true,
							message: "Permission granted successfully",
						},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:    "",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     0,
							permission: "read:profile",
						},
						expected: &expected{
							success: false,
							message: "User ID is mandatory",
						},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "",
						},
						expected: &expected{
							success: false,
							message: "Permission is mandatory",
						},
					},
					{
						name: "Should fail when TraceId, UserID, and Permission are all empty",
						input: &input{
							traceId:    "",
							userID:     0,
							permission: "",
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserCheckPermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User CheckPermission", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId    string
				userID     int
				permission string
			}
			type expected struct {
				success       bool
				message       string
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

				// Seed permissions for user 0
				app.Helper.AddPermission(ctx, t, Users[0].ID, fmt.Sprintf("%d:read:profile", Users[0].ID))
				app.Helper.AddPermission(ctx, t, Users[0].ID, fmt.Sprintf("%d:write:profile", Users[0].ID))

				// Seed super_user for user 1 (acts as a wildcard for all checks)
				app.Helper.AddPermission(ctx, t, Users[1].ID, fmt.Sprintf("%d:super_user", Users[1].ID))
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				initialPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)

				// Execute the service method being tested
				output := app.Service.CheckPermission(ctx, &permission.CheckPermissionInput{
					TraceId:    r.input.traceId,
					UserID:     r.input.userID,
					Permission: r.input.permission,
				})

				// Get state AFTER operation
				afterPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)

				// Assert response matches expectations
				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.hasPermission, output.HasPermission, r.name)

				// CheckPermission is read-only, no state should change
				assert.Equal(t, initialPerms, afterPerms, r.name)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "CheckPermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should return true when user has permission",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: true,
						},
					},
					{
						name: "Should return true for another existing permission",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "write:profile",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: true,
						},
					},
					{
						name: "Should return false when user does not have requested permission",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "delete:profile",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: false,
						},
					},
					{
						name: "Should return true for a different user holding super_user (wildcard override)",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[1].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: true, // via super_user override
						},
					},
					{
						name: "Should return true when checking the super_user key itself",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[1].ID,
							permission: fmt.Sprintf("%d:super_user", Users[1].ID),
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: true,
						},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:    "",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success:       false,
							message:       "TraceId is mandatory",
							hasPermission: false,
						},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     0,
							permission: "read:profile",
						},
						expected: &expected{
							success:       false,
							message:       "User ID is mandatory",
							hasPermission: false,
						},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "",
						},
						expected: &expected{
							success:       false,
							message:       "Permission is mandatory",
							hasPermission: false,
						},
					},
				})
			})
		})
	})
}

func TestUserRevokePermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User RevokePermission", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId    string
				userID     int
				permission string
			}
			type expected struct {
				success      bool
				message      string
				permsRemoved int // Number of permissions expected to be removed (0 or 1)
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

				// Seed permissions to revoke
				app.Helper.AddPermission(ctx, t, Users[0].ID, fmt.Sprintf("%d:read:profile", Users[0].ID))
				app.Helper.AddPermission(ctx, t, Users[0].ID, fmt.Sprintf("%d:write:profile", Users[0].ID))
				app.Helper.AddPermission(ctx, t, Users[1].ID, fmt.Sprintf("%d:read:profile", Users[1].ID))
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				initialPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)

				// Execute the service method being tested
				output := app.Service.RevokePermission(ctx, &permission.RevokePermissionInput{
					TraceId:    r.input.traceId,
					UserID:     r.input.userID,
					Permission: r.input.permission,
				})

				// Get state AFTER operation
				afterPerms := app.Helper.GetAllPermissions(ctx, t, r.input.userID)

				// Assert response matches expectations
				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no permissions were modified on failure
					assert.Equal(t, initialPerms, afterPerms, r.name)
					return
				}

				// For successful revoke, verify permission was removed (if it existed)
				assert.Equal(t, len(initialPerms)-r.expected.permsRemoved, len(afterPerms), r.name)
				if r.expected.permsRemoved > 0 {
					assert.NotContains(t, afterPerms, fmt.Sprintf("%d:%s", r.input.userID, r.input.permission), r.name)
				}
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "RevokePermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should revoke permission successfully",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success:      true,
							message:      "Permission revoked successfully",
							permsRemoved: 1,
						},
					},
					{
						name: "Should revoke another permission for same user",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "write:profile",
						},
						expected: &expected{
							success:      true,
							message:      "Permission revoked successfully",
							permsRemoved: 1,
						},
					},
					{
						name: "Should revoke permission for different user",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[1].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success:      true,
							message:      "Permission revoked successfully",
							permsRemoved: 1,
						},
					},
					{
						name: "Should succeed when revoking non-existent permission",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "nonexistent:permission",
						},
						expected: &expected{
							success:      true,
							message:      "Permission revoked successfully",
							permsRemoved: 0,
						},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:    "",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     0,
							permission: "read:profile",
						},
						expected: &expected{
							success: false,
							message: "User ID is mandatory",
						},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "",
						},
						expected: &expected{
							success: false,
							message: "Permission is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserCreateRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User CreateRole", func() {

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId     string
				name        string
				description string
			}
			type expected struct {
				success bool
				message string
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				// A role with the reserved name is seeded so duplicate-name
				// rows can be exercised.
				app.Helper.InsertRole(ctx, t, "admin", "Seeded admin")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialRoles := app.Helper.GetAllRoles(ctx, t)

				output := app.Service.CreateRole(ctx, &permission.CreateRoleInput{
					TraceId:     r.input.traceId,
					Name:        r.input.name,
					Description: r.input.description,
				})

				afterRoles := app.Helper.GetAllRoles(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					// Verify no roles were created on failure
					assert.Equal(t, initialRoles, afterRoles, r.name)
					return
				}

				// Verify exactly one role was created with the right fields
				assert.Equal(t, len(initialRoles)+1, len(afterRoles), r.name)
				created, ok := afterRoles[output.Role.Id]
				assert.True(t, ok, r.name+" - created role should exist")
				assert.Equal(t, r.input.name, created.Name, r.name)
				assert.Equal(t, r.input.description, created.Description, r.name)
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
					// ===== Success Tests =====
					{
						name: "Should create role with description",
						input: &input{
							traceId:     "trace-test",
							name:        "editor",
							description: "Can edit content",
						},
						expected: &expected{
							success: true,
							message: "Role created successfully",
						},
					},
					{
						name: "Should create role without description",
						input: &input{
							traceId: "trace-test",
							name:    "viewer",
						},
						expected: &expected{
							success: true,
							message: "Role created successfully",
						},
					},

					// ===== Duplicate Name Tests =====
					{
						name: "Should fail when role name already exists",
						input: &input{
							traceId:     "trace-test",
							name:        "admin",
							description: "Duplicate",
						},
						expected: &expected{
							success: false,
							message: "Role already exists",
						},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							name:    "solo",
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when name is empty",
						input: &input{
							traceId: "trace-test",
							name:    "",
						},
						expected: &expected{
							success: false,
							message: "Role name is mandatory",
						},
					},
					{
						name: "Should fail when name is only whitespace",
						input: &input{
							traceId: "trace-test",
							name:    "   ",
						},
						expected: &expected{
							success: false,
							message: "Role name is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserAddRolePermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User AddRolePermission", func() {

			// ========== 1. Declare Fixture Variables ==========
			var RoleID int

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId    string
				roleID     int
				permission string
			}
			type expected struct {
				success bool
				message string
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				RoleID = app.Helper.InsertRole(ctx, t, "editor", "")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialPerms := app.Helper.GetRolePermissions(ctx, t, r.input.roleID)

				output := app.Service.AddRolePermission(ctx, &permission.AddRolePermissionInput{
					TraceId:    r.input.traceId,
					RoleId:     r.input.roleID,
					Permission: r.input.permission,
				})

				afterPerms := app.Helper.GetRolePermissions(ctx, t, r.input.roleID)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					assert.Equal(t, initialPerms, afterPerms, r.name)
					return
				}

				if r.input.roleID == RoleID {
					assert.Contains(t, afterPerms, r.input.permission, r.name)
				}
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "AddRolePermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should add permission to role",
						input: &input{
							traceId:    "trace-test",
							roleID:     RoleID,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: true,
							message: "Role permission added successfully",
						},
					},
					{
						name: "Should add another permission to same role",
						input: &input{
							traceId:    "trace-test",
							roleID:     RoleID,
							permission: "user:password_update",
						},
						expected: &expected{
							success: true,
							message: "Role permission added successfully",
						},
					},
					{
						name: "Should re-adding same permission be idempotent",
						input: &input{
							traceId:    "trace-test",
							roleID:     RoleID,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: true,
							message: "Role permission added successfully",
						},
					},

					// ===== Role Not Found =====
					{
						name: "Should fail when role does not exist",
						input: &input{
							traceId:    "trace-test",
							roleID:     99999,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: false,
							message: "Role not found",
						},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:    "",
							roleID:     RoleID,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when RoleID is empty",
						input: &input{
							traceId:    "trace-test",
							roleID:     0,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: false,
							message: "Role ID is mandatory",
						},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							traceId:    "trace-test",
							roleID:     RoleID,
							permission: "",
						},
						expected: &expected{
							success: false,
							message: "Permission is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserAssignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User AssignRole", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser
			var RoleID int

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId string
				userID  int
				roleID  int
			}
			type expected struct {
				success bool
				message string
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
				RoleID = app.Helper.InsertRole(ctx, t, "editor", "")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				output := app.Service.AssignRole(ctx, &permission.AssignRoleInput{
					TraceId: r.input.traceId,
					UserID:  r.input.userID,
					RoleId:  r.input.roleID,
				})

				afterRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					assert.Equal(t, initialRoles, afterRoles, r.name)
					return
				}

				// Verify the role is present after assignment, and the count only
				// grows when it was not already assigned (idempotent re-assign).
				alreadyAssigned := false
				for _, role := range initialRoles {
					if role.Id == RoleID {
						alreadyAssigned = true
						break
					}
				}
				assigned := false
				for _, role := range afterRoles {
					if role.Id == RoleID && role.Name == "editor" {
						assigned = true
						break
					}
				}
				assert.True(t, assigned, r.name+" - editor role should be assigned")
				if alreadyAssigned {
					assert.Equal(t, len(initialRoles), len(afterRoles), r.name)
				} else {
					assert.Equal(t, len(initialRoles)+1, len(afterRoles), r.name)
				}
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "AssignRole scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should assign role to user",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
							roleID:  RoleID,
						},
						expected: &expected{
							success: true,
							message: "Role assigned successfully",
						},
					},
					{
						name: "Should assign same role to another user",
						input: &input{
							traceId: "trace-test",
							userID:  Users[1].ID,
							roleID:  RoleID,
						},
						expected: &expected{
							success: true,
							message: "Role assigned successfully",
						},
					},
					{
						name: "Should re-assigning same role be idempotent",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
							roleID:  RoleID,
						},
						expected: &expected{
							success: true,
							message: "Role assigned successfully",
						},
					},

					// ===== Role Not Found =====
					{
						name: "Should fail when role does not exist",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
							roleID:  99999,
						},
						expected: &expected{
							success: false,
							message: "Role not found",
						},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							userID:  Users[0].ID,
							roleID:  RoleID,
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId: "trace-test",
							userID:  0,
							roleID:  RoleID,
						},
						expected: &expected{
							success: false,
							message: "User ID is mandatory",
						},
					},
					{
						name: "Should fail when RoleID is empty",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
							roleID:  0,
						},
						expected: &expected{
							success: false,
							message: "Role ID is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserCheckPermissionRoleFallback(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User CheckPermission role fallback", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId    string
				userID     int
				permission string
			}
			type expected struct {
				success       bool
				message       string
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
					{Idx: 0, ID: 100}, // role editor  -> user:profile_update
					{Idx: 1, ID: 200}, // role admin   -> super_user (wildcard)
					{Idx: 2, ID: 300}, // role viewer  -> no permissions
				}

				editorRoleID := app.Helper.InsertRole(ctx, t, "editor", "")
				app.Helper.AddRolePermissionToRole(ctx, t, editorRoleID, "user:profile_update")
				app.Helper.AssignRoleToUser(ctx, t, Users[0].ID, editorRoleID)

				adminRoleID := app.Helper.InsertRole(ctx, t, "admin", "")
				app.Helper.AddRolePermissionToRole(ctx, t, adminRoleID, permission.PermissionSuperUser)
				app.Helper.AssignRoleToUser(ctx, t, Users[1].ID, adminRoleID)

				viewerRoleID := app.Helper.InsertRole(ctx, t, "viewer", "")
				app.Helper.AssignRoleToUser(ctx, t, Users[2].ID, viewerRoleID)
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				output := app.Service.CheckPermission(ctx, &permission.CheckPermissionInput{
					TraceId:    r.input.traceId,
					UserID:     r.input.userID,
					Permission: r.input.permission,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.hasPermission, output.HasPermission, r.name)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "CheckPermission role fallback scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should allow permission granted by role",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "user:profile_update",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: true,
						},
					},
					{
						name: "Should deny permission not granted by role",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "user:password_update",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: false,
						},
					},
					{
						name: "Should allow any permission for role holding super_user wildcard",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[1].ID,
							permission: "user:password_update",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: true,
						},
					},
					{
						name: "Should deny for role without permissions",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[2].ID,
							permission: "user:profile_update",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: false,
						},
					},
					{
						name: "Should deny for user without any role",
						input: &input{
							traceId:    "trace-test",
							userID:     99999,
							permission: "user:profile_update",
						},
						expected: &expected{
							success:       true,
							message:       "Permission check completed",
							hasPermission: false,
						},
					},
				})
			})
		})
	})
}

func TestUserListUserPermissions(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User ListUserPermissions", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId string
				userID  int
			}
			type expected struct {
				success   bool
				message   string
				direct    []string
				effective []string
				roleNames []string
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				Users = []DataUser{
					{Idx: 0, ID: 100}, // direct perms + two roles
					{Idx: 1, ID: 200}, // nothing
				}

				// User 0: direct permissions (stored as "<user_id>:<permission>" keys)
				app.Helper.AddPermission(ctx, t, Users[0].ID, fmt.Sprintf("%d:user:profile_update", Users[0].ID))
				app.Helper.AddPermission(ctx, t, Users[0].ID, fmt.Sprintf("%d:user:password_update", Users[0].ID))

				// User 0: editor role granting profile_update (overlaps direct)
				editorRoleID := app.Helper.InsertRole(ctx, t, "editor", "Can edit")
				app.Helper.AddRolePermissionToRole(ctx, t, editorRoleID, "user:profile_update")
				app.Helper.AssignRoleToUser(ctx, t, Users[0].ID, editorRoleID)

				// User 0: viewer role granting a report permission
				viewerRoleID := app.Helper.InsertRole(ctx, t, "viewer", "Can view")
				app.Helper.AddRolePermissionToRole(ctx, t, viewerRoleID, "report:view")
				app.Helper.AssignRoleToUser(ctx, t, Users[0].ID, viewerRoleID)
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				output := app.Service.ListUserPermissions(ctx, &permission.ListUserPermissionsInput{
					TraceId: r.input.traceId,
					UserID:  r.input.userID,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					return
				}

				assert.Equal(t, r.expected.direct, output.Direct, r.name)
				assert.Equal(t, r.expected.effective, output.Effective, r.name)

				roleNames := make([]string, 0, len(output.Roles))
				for _, rp := range output.Roles {
					roleNames = append(roleNames, rp.Role.Name)
				}
				assert.Equal(t, r.expected.roleNames, roleNames, r.name)
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
						name: "Should list direct, role, and effective permissions",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
						},
						expected: &expected{
							success:   true,
							message:   "Permissions retrieved successfully",
							direct:    []string{"user:password_update", "user:profile_update"},
							effective: []string{"report:view", "user:password_update", "user:profile_update"},
							roleNames: []string{"editor", "viewer"},
						},
					},
					{
						name: "Should list empty permissions for user without grants",
						input: &input{
							traceId: "trace-test",
							userID:  Users[1].ID,
						},
						expected: &expected{
							success:   true,
							message:   "Permissions retrieved successfully",
							direct:    []string{},
							effective: []string{},
							roleNames: []string{},
						},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							userID:  Users[0].ID,
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId: "trace-test",
							userID:  0,
						},
						expected: &expected{
							success: false,
							message: "User ID is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserRemoveRolePermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User RemoveRolePermission", func() {

			var RoleID int

			type input struct {
				traceId    string
				roleID     int
				permission string
			}
			type expected struct {
				success        bool
				message        string
				permsRemaining int
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				RoleID = app.Helper.InsertRole(ctx, t, "editor", "")
				app.Helper.AddRolePermissionToRole(ctx, t, RoleID, "user:profile_update")
				app.Helper.AddRolePermissionToRole(ctx, t, RoleID, "user:password_update")
			})

			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialPerms := app.Helper.GetRolePermissions(ctx, t, r.input.roleID)

				output := app.Service.RemoveRolePermission(ctx, &permission.RemoveRolePermissionInput{
					TraceId:    r.input.traceId,
					RoleId:     r.input.roleID,
					Permission: r.input.permission,
				})

				afterPerms := app.Helper.GetRolePermissions(ctx, t, r.input.roleID)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					assert.Equal(t, initialPerms, afterPerms, r.name)
					return
				}

				assert.Equal(t, r.expected.permsRemaining, len(afterPerms), r.name)
				assert.NotContains(t, afterPerms, r.input.permission, r.name)
			}

			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "RemoveRolePermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should remove a permission from a role",
						input: &input{
							traceId:    "trace-test",
							roleID:     RoleID,
							permission: "user:profile_update",
						},
						expected: &expected{
							success:        true,
							message:        "Role permission removed successfully",
							permsRemaining: 1,
						},
					},
					{
						name: "Should succeed removing a non-existent permission",
						input: &input{
							traceId:    "trace-test",
							roleID:     RoleID,
							permission: "nonexistent:perm",
						},
						expected: &expected{
							success:        true,
							message:        "Role permission removed successfully",
							permsRemaining: 1,
						},
					},
					{
						name: "Should fail when role does not exist",
						input: &input{
							traceId:    "trace-test",
							roleID:     99999,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: false,
							message: "Role not found",
						},
					},
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:    "",
							roleID:     RoleID,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when RoleID is empty",
						input: &input{
							traceId:    "trace-test",
							roleID:     0,
							permission: "user:profile_update",
						},
						expected: &expected{
							success: false,
							message: "Role ID is mandatory",
						},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							traceId:    "trace-test",
							roleID:     RoleID,
							permission: "",
						},
						expected: &expected{
							success: false,
							message: "Permission is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserUnassignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User UnassignRole", func() {

			var Users []DataUser
			var RoleID int

			type input struct {
				traceId string
				userID  int
				roleID  int
			}
			type expected struct {
				success bool
				message string
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				Users = []DataUser{
					{Idx: 0, ID: 100},
					{Idx: 1, ID: 200},
				}
				RoleID = app.Helper.InsertRole(ctx, t, "editor", "")
				app.Helper.AssignRoleToUser(ctx, t, Users[0].ID, RoleID)
			})

			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				output := app.Service.UnassignRole(ctx, &permission.UnassignRoleInput{
					TraceId: r.input.traceId,
					UserID:  r.input.userID,
					RoleId:  r.input.roleID,
				})

				afterRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					assert.Equal(t, initialRoles, afterRoles, r.name)
					return
				}

				stillAssigned := false
				for _, role := range afterRoles {
					if role.Id == RoleID {
						stillAssigned = true
						break
					}
				}
				assert.False(t, stillAssigned, r.name+" - role should no longer be assigned")
			}

			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "UnassignRole scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should unassign a role from a user",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
							roleID:  RoleID,
						},
						expected: &expected{
							success: true,
							message: "Role unassigned successfully",
						},
					},
					{
						name: "Should succeed unassigning a never-assigned role",
						input: &input{
							traceId: "trace-test",
							userID:  Users[1].ID,
							roleID:  RoleID,
						},
						expected: &expected{
							success: true,
							message: "Role unassigned successfully",
						},
					},
					{
						name: "Should fail when role does not exist",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
							roleID:  99999,
						},
						expected: &expected{
							success: false,
							message: "Role not found",
						},
					},
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							userID:  Users[0].ID,
							roleID:  RoleID,
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId: "trace-test",
							userID:  0,
							roleID:  RoleID,
						},
						expected: &expected{
							success: false,
							message: "User ID is mandatory",
						},
					},
					{
						name: "Should fail when RoleID is empty",
						input: &input{
							traceId: "trace-test",
							userID:  Users[0].ID,
							roleID:  0,
						},
						expected: &expected{
							success: false,
							message: "Role ID is mandatory",
						},
					},
				})
			})
		})
	})
}

func TestUserDeleteRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User DeleteRole", func() {

			var RoleID int
			var UserID int

			type input struct {
				traceId string
				roleID  int
			}
			type expected struct {
				success bool
				message string
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				UserID = 100
				RoleID = app.Helper.InsertRole(ctx, t, "editor", "")
				app.Helper.AddRolePermissionToRole(ctx, t, RoleID, "user:profile_update")
				app.Helper.AssignRoleToUser(ctx, t, UserID, RoleID)
			})

			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialRoles := app.Helper.GetAllRoles(ctx, t)

				output := app.Service.DeleteRole(ctx, &permission.DeleteRoleInput{
					TraceId: r.input.traceId,
					RoleId:  r.input.roleID,
				})

				afterRoles := app.Helper.GetAllRoles(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					assert.Equal(t, initialRoles, afterRoles, r.name)
					return
				}

				_, stillExists := afterRoles[r.input.roleID]
				assert.False(t, stillExists, r.name+" - role should be gone")
				// ON DELETE CASCADE must drop the dependent rows too.
				assert.Empty(t, app.Helper.GetRolePermissions(ctx, t, r.input.roleID), r.name+" - role_permissions cascade")
				assert.Empty(t, app.Helper.GetUserRoles(ctx, t, UserID), r.name+" - user_roles cascade")
			}

			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "DeleteRole scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should delete a role and cascade its mappings",
						input: &input{
							traceId: "trace-test",
							roleID:  RoleID,
						},
						expected: &expected{
							success: true,
							message: "Role deleted successfully",
						},
					},
					{
						name: "Should fail when role does not exist",
						input: &input{
							traceId: "trace-test",
							roleID:  99999,
						},
						expected: &expected{
							success: false,
							message: "Role not found",
						},
					},
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							roleID:  RoleID,
						},
						expected: &expected{
							success: false,
							message: "TraceId is mandatory",
						},
					},
					{
						name: "Should fail when RoleID is empty",
						input: &input{
							traceId: "trace-test",
							roleID:  0,
						},
						expected: &expected{
							success: false,
							message: "Role ID is mandatory",
						},
					},
				})
			})
		})
	})
}
