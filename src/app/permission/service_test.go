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
				assert.Contains(t, afterPerms, r.input.permission, r.name)
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
				app.Helper.AddPermission(ctx, t, Users[0].ID, "read:profile")
				app.Helper.AddPermission(ctx, t, Users[0].ID, "write:profile")

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
				app.Helper.AddPermission(ctx, t, Users[0].ID, "read:profile")
				app.Helper.AddPermission(ctx, t, Users[0].ID, "write:profile")
				app.Helper.AddPermission(ctx, t, Users[1].ID, "read:profile")
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
					assert.NotContains(t, afterPerms, r.input.permission, r.name)
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
