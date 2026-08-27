package permission_test

import (
	"context"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/clock"
	"github.com/stretchr/testify/assert"
)

func TestUserAssignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User AssignRole", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId  string
				userID   int
				roleName string
				actorID  int
			}
			type expected struct {
				success    bool
				message    string
				countDelta int // expected change in the target's role count
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

				// Roles super_user/member are seeded by migration. Register
				// one more via SOP to exercise a non-builtin role.
				app.Helper.AddRole(ctx, t, "editor")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				initialRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				output := app.Service.AssignRole(ctx, &permission.AssignRoleInput{
					TraceId:  r.input.traceId,
					UserID:   r.input.userID,
					RoleName: r.input.roleName,
					ActorId:  r.input.actorID,
				})

				afterRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, len(initialRoles)+r.expected.countDelta, len(afterRoles), r.name)

				if !r.expected.success {
					assert.Equal(t, initialRoles, afterRoles, r.name)
					return
				}

				if r.expected.countDelta > 0 {
					assert.Contains(t, afterRoles, r.input.roleName, r.name)
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
						name: "Should assign role successfully",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: permission.RoleMember,
							actorID:  0,
						},
						expected: &expected{success: true, message: "Role assigned successfully", countDelta: 1},
					},
					{
						name: "Should be a no-op success when the role is already assigned",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: permission.RoleMember,
							actorID:  0,
						},
						expected: &expected{success: true, message: "Role assigned successfully", countDelta: 0},
					},
					{
						name: "Should assign another non-builtin role to the same user",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: "editor",
							actorID:  0,
						},
						expected: &expected{success: true, message: "Role assigned successfully", countDelta: 1},
					},
					{
						name: "Should assign role to a different user",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[1].ID,
							roleName: permission.RoleMember,
							actorID:  300,
						},
						expected: &expected{success: true, message: "Role assigned successfully", countDelta: 1},
					},

					// ===== Policy Tests =====
					{
						name: "Should fail when assigning the bootstrap-only super_user role",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: permission.RoleSuperUser,
							actorID:  300,
						},
						expected: &expected{success: false, message: "Cannot assign super_user role"},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:  "",
							userID:   Users[0].ID,
							roleName: permission.RoleMember,
						},
						expected: &expected{success: false, message: "TraceId is mandatory"},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId:  "trace-test",
							userID:   0,
							roleName: permission.RoleMember,
						},
						expected: &expected{success: false, message: "User ID is mandatory"},
					},
					{
						name: "Should fail when RoleName is empty",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: "",
						},
						expected: &expected{success: false, message: "Role name is mandatory"},
					},
					{
						name: "Should fail when role does not exist",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: "nonexistent_role",
						},
						expected: &expected{success: false, message: "Unknown role"},
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

				// User 0: editor role carrying two permissions.
				app.Helper.AddRole(ctx, t, "editor")
				app.Helper.AddRolePermission(ctx, t, "editor", "read:profile")
				app.Helper.AddRolePermission(ctx, t, "editor", "write:profile")
				app.Helper.SetUserRole(ctx, t, Users[0].ID, "editor")

				// User 1: bootstrap-style super_user (seeded directly, as the
				// service layer refuses to assign it).
				app.Helper.SetUserRole(ctx, t, Users[1].ID, permission.RoleSuperUser)
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
			suite.Run(t, "CheckPermission scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should return true when a role of the user carries the permission",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{success: true, message: "Permission check completed", hasPermission: true},
					},
					{
						name: "Should return true for another permission carried by the same role",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "write:profile",
						},
						expected: &expected{success: true, message: "Permission check completed", hasPermission: true},
					},
					{
						name: "Should return false when no role of the user carries the permission",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "delete:profile",
						},
						expected: &expected{success: true, message: "Permission check completed", hasPermission: false},
					},
					{
						name: "Should return true for a super_user holder (wildcard override)",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[1].ID,
							permission: "delete:profile",
						},
						expected: &expected{success: true, message: "Permission check completed", hasPermission: true},
					},
					{
						name: "Should return true when checking the super_user permission itself",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[1].ID,
							permission: permission.RoleSuperUser,
						},
						expected: &expected{success: true, message: "Permission check completed", hasPermission: true},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:    "",
							userID:     Users[0].ID,
							permission: "read:profile",
						},
						expected: &expected{success: false, message: "TraceId is mandatory"},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     0,
							permission: "read:profile",
						},
						expected: &expected{success: false, message: "User ID is mandatory"},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							traceId:    "trace-test",
							userID:     Users[0].ID,
							permission: "",
						},
						expected: &expected{success: false, message: "Permission is mandatory"},
					},
				})
			})
		})
	})
}

func TestUserUnassignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User UnassignRole", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId  string
				userID   int
				roleName string
				actorID  int
			}
			type expected struct {
				success    bool
				message    string
				countDelta int
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

				app.Helper.AddRole(ctx, t, "editor")
				app.Helper.SetUserRole(ctx, t, Users[0].ID, "editor")
				app.Helper.SetUserRole(ctx, t, Users[0].ID, permission.RoleMember)
				app.Helper.SetUserRole(ctx, t, Users[1].ID, permission.RoleMember)
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				output := app.Service.UnassignRole(ctx, &permission.UnassignRoleInput{
					TraceId:  r.input.traceId,
					UserID:   r.input.userID,
					RoleName: r.input.roleName,
					ActorId:  r.input.actorID,
				})

				afterRoles := app.Helper.GetUserRoles(ctx, t, r.input.userID)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, len(initialRoles)+r.expected.countDelta, len(afterRoles), r.name)

				if !r.expected.success {
					assert.Equal(t, initialRoles, afterRoles, r.name)
					return
				}

				if r.expected.countDelta > 0 {
					assert.NotContains(t, afterRoles, r.input.roleName, r.name)
				}
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "UnassignRole scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should unassign a held role successfully",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: "editor",
							actorID:  300,
						},
						expected: &expected{success: true, message: "Role unassigned successfully", countDelta: -1},
					},
					{
						name: "Should succeed as a no-op when the user does not hold an existing role",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: permission.RoleSuperUser,
							actorID:  300,
						},
						expected: &expected{success: true, message: "Role unassigned successfully", countDelta: 0},
					},
					{
						name: "Should unassign for a different user",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[1].ID,
							roleName: permission.RoleMember,
							actorID:  300,
						},
						expected: &expected{success: true, message: "Role unassigned successfully", countDelta: -1},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:  "",
							userID:   Users[0].ID,
							roleName: "editor",
						},
						expected: &expected{success: false, message: "TraceId is mandatory"},
					},
					{
						name: "Should fail when UserID is empty",
						input: &input{
							traceId:  "trace-test",
							userID:   0,
							roleName: "editor",
						},
						expected: &expected{success: false, message: "User ID is mandatory"},
					},
					{
						name: "Should fail when role does not exist",
						input: &input{
							traceId:  "trace-test",
							userID:   Users[0].ID,
							roleName: "nonexistent_role",
						},
						expected: &expected{success: false, message: "Unknown role"},
					},
				})
			})

			suite.Run(t, "refuses to strip the last super_user", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				admin := &DataUser{ID: 999}
				app.Helper.SetUserRole(ctx, t, admin.ID, permission.RoleSuperUser)

				out := app.Service.UnassignRole(ctx, &permission.UnassignRoleInput{
					TraceId:  "trace-guard",
					UserID:   admin.ID,
					RoleName: permission.RoleSuperUser,
					ActorId:  300,
				})
				assert.False(t, out.Success)
				assert.Equal(t, "Cannot remove the last super_user role", out.Message)
				assert.Equal(t, permission.ErrorCodeValidation, out.ErrorCode)
				assert.Contains(t, app.Helper.GetUserRoles(ctx, t, admin.ID), permission.RoleSuperUser)

				// Second holder unblocks removal
				second := &DataUser{ID: 998}
				app.Helper.SetUserRole(ctx, t, second.ID, permission.RoleSuperUser)
				out = app.Service.UnassignRole(ctx, &permission.UnassignRoleInput{
					TraceId:  "trace-guard",
					UserID:   admin.ID,
					RoleName: permission.RoleSuperUser,
					ActorId:  300,
				})
				assert.True(t, out.Success)
				assert.NotContains(t, app.Helper.GetUserRoles(ctx, t, admin.ID), permission.RoleSuperUser)
			})
		})
	})
}

func TestRoleAuditTrail(t *testing.T) {
	// Pin time so created_at is asserted exactly.
	fixedNow := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	clock.SetSource(clock.Fixed(fixedNow))
	t.Cleanup(clock.Reset)

	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Role audit trail", func() {

			// ========== 1. Declare Fixture Variables ==========
			var (
				Actor  DataUser
				System DataUser // ID 0: no acting user
				Target DataUser
			)

			// ========== 2. Define Test Structures ==========
			type input struct {
				op       string // AuditActionGrant or AuditActionRevoke
				traceId  string
				actorID  int
				userID   int
				roleName string
			}
			type expectedAudit struct {
				actorID  *int
				targetID int
				roleName string
				action   string
			}
			type expected struct {
				success      bool
				message      string
				newAuditRows int            // rows appended to the target's trail
				lastAudit    *expectedAudit // checked when newAuditRows > 0
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				Actor = DataUser{Idx: 0, ID: 300}
				Target = DataUser{Idx: 1, ID: 100}

				app.Helper.SetUserRole(ctx, t, Target.ID, permission.RoleMember)
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				before := app.Helper.GetPermissionAudit(ctx, t, r.input.userID)

				var success bool
				var message string
				switch r.input.op {
				case permission.AuditActionGrant:
					out := app.Service.AssignRole(ctx, &permission.AssignRoleInput{
						TraceId:  r.input.traceId,
						UserID:   r.input.userID,
						RoleName: r.input.roleName,
						ActorId:  r.input.actorID,
					})
					success, message = out.Success, out.Message
				case permission.AuditActionRevoke:
					out := app.Service.UnassignRole(ctx, &permission.UnassignRoleInput{
						TraceId:  r.input.traceId,
						UserID:   r.input.userID,
						RoleName: r.input.roleName,
						ActorId:  r.input.actorID,
					})
					success, message = out.Success, out.Message
				}

				after := app.Helper.GetPermissionAudit(ctx, t, r.input.userID)

				assert.Equal(t, r.expected.success, success, r.name)
				assert.Equal(t, r.expected.message, message, r.name)

				// Assert audit-trail delta matches expectations
				assert.Equal(t, len(before)+r.expected.newAuditRows, len(after), r.name)
				if r.expected.newAuditRows == 0 {
					return
				}

				want := r.expected.lastAudit
				got := after[len(after)-1]
				assert.Equal(t, want.actorID, got.ActorId, r.name)
				assert.Equal(t, want.targetID, got.TargetId, r.name)
				assert.Equal(t, want.roleName, got.Permission, r.name)
				assert.Equal(t, want.action, got.Action, r.name)
				assert.True(t, got.CreatedAt.Equal(fixedNow), "%s: created_at = %v, want %v", r.name, got.CreatedAt, fixedNow)
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "audit trail scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should record an audit row when an actor assigns a role",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-audit",
							actorID:  Actor.ID,
							userID:   Target.ID,
							roleName: permission.RoleMember,
						},
						expected: &expected{
							success:      true,
							message:      "Role assigned successfully",
							newAuditRows: 1,
							lastAudit: &expectedAudit{
								actorID:  &Actor.ID,
								targetID: Target.ID,
								roleName: permission.RoleMember,
								action:   permission.AuditActionGrant,
							},
						},
					},
					{
						name: "Should record an audit row when an actor removes a role",
						input: &input{
							op:       permission.AuditActionRevoke,
							traceId:  "trace-audit",
							actorID:  Actor.ID,
							userID:   Target.ID,
							roleName: permission.RoleMember,
						},
						expected: &expected{
							success:      true,
							message:      "Role unassigned successfully",
							newAuditRows: 1,
							lastAudit: &expectedAudit{
								actorID:  &Actor.ID,
								targetID: Target.ID,
								roleName: permission.RoleMember,
								action:   permission.AuditActionRevoke,
							},
						},
					},
					{
						name: "Should store NULL actor when the assignment is system-initiated",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-audit",
							actorID:  System.ID, // 0: no acting user
							userID:   Target.ID,
							roleName: permission.RoleMember,
						},
						expected: &expected{
							success:      true,
							message:      "Role assigned successfully",
							newAuditRows: 1,
							lastAudit: &expectedAudit{
								actorID:  nil,
								targetID: Target.ID,
								roleName: permission.RoleMember,
								action:   permission.AuditActionGrant,
							},
						},
					},

					// ===== Rejection Tests =====
					{
						name: "Should not write an audit row when assigning super_user is refused",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-audit",
							actorID:  Actor.ID,
							userID:   Target.ID,
							roleName: permission.RoleSuperUser,
						},
						expected: &expected{
							success:      false,
							message:      "Cannot assign super_user role",
							newAuditRows: 0,
						},
					},
					{
						name: "Should not write an audit row on validation failure",
						input: &input{
							op:       permission.AuditActionRevoke,
							traceId:  "",
							actorID:  Actor.ID,
							userID:   Target.ID,
							roleName: permission.RoleMember,
						},
						expected: &expected{
							success:      false,
							message:      "TraceId is mandatory",
							newAuditRows: 0,
						},
					},
				})
			})
		})
	})
}

func TestRolePermissionManagement(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Assign/RemovePermissionToRole", func() {

			// ========== 1. Declare Fixture Variables ==========
			var (
				Actor DataUser
				Known string // catalog-registered permission
			)

			// ========== 2. Define Test Structures ==========
			type input struct {
				op       string // AuditActionGrant or AuditActionRevoke
				traceId  string
				roleName string
				perm     string
			}
			type expected struct {
				success    bool
				message    string
				countDelta int
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *PermissionApp) {
				Actor = DataUser{Idx: 0, ID: 300}
				Known = "user:profile_update"

				app.Helper.AddKnownPermission(ctx, t, Known)
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initial := app.Helper.GetRolePermissions(ctx, t, r.input.roleName)

				var success bool
				var message string
				switch r.input.op {
				case permission.AuditActionGrant:
					out := app.Service.AssignPermissionToRole(ctx, &permission.AssignPermissionToRoleInput{
						TraceId:    r.input.traceId,
						RoleName:   r.input.roleName,
						Permission: r.input.perm,
						ActorId:    Actor.ID,
					})
					success, message = out.Success, out.Message
				case permission.AuditActionRevoke:
					out := app.Service.RemovePermissionFromRole(ctx, &permission.RemovePermissionFromRoleInput{
						TraceId:    r.input.traceId,
						RoleName:   r.input.roleName,
						Permission: r.input.perm,
						ActorId:    Actor.ID,
					})
					success, message = out.Success, out.Message
				}

				after := app.Helper.GetRolePermissions(ctx, t, r.input.roleName)

				assert.Equal(t, r.expected.success, success, r.name)
				assert.Equal(t, r.expected.message, message, r.name)
				assert.Equal(t, len(initial)+r.expected.countDelta, len(after), r.name)

				if !r.expected.success {
					assert.Equal(t, initial, after, r.name)
					return
				}
				if r.input.op == permission.AuditActionGrant && r.expected.countDelta > 0 {
					assert.Contains(t, after, r.input.perm, r.name)
				}
				if r.input.op == permission.AuditActionRevoke && r.expected.countDelta > 0 {
					assert.NotContains(t, after, r.input.perm, r.name)
				}
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "RolePermissionManagement scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should grant a catalog permission to a role",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							roleName: permission.RoleMember,
							perm:     Known,
						},
						expected: &expected{success: true, message: "Permission assigned to role successfully", countDelta: 1},
					},
					{
						name: "Should be a no-op success when the role already carries the permission",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							roleName: permission.RoleMember,
							perm:     Known,
						},
						expected: &expected{success: true, message: "Permission assigned to role successfully", countDelta: 0},
					},
					{
						name: "Should revoke a permission from a role",
						input: &input{
							op:       permission.AuditActionRevoke,
							traceId:  "trace-test",
							roleName: permission.RoleMember,
							perm:     Known,
						},
						expected: &expected{success: true, message: "Permission removed from role successfully", countDelta: -1},
					},

					// ===== Policy Tests =====
					{
						name: "Should refuse granting on the super_user role",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							roleName: permission.RoleSuperUser,
							perm:     Known,
						},
						expected: &expected{success: false, message: "Cannot modify super_user role"},
					},
					{
						name: "Should refuse revoking on the super_user role",
						input: &input{
							op:       permission.AuditActionRevoke,
							traceId:  "trace-test",
							roleName: permission.RoleSuperUser,
							perm:     Known,
						},
						expected: &expected{success: false, message: "Cannot modify super_user role"},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when the permission is not in the catalog",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							roleName: permission.RoleMember,
							perm:     "garbage:permission",
						},
						expected: &expected{success: false, message: "Unknown permission"},
					},
					{
						name: "Should fail when the role does not exist",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							roleName: "nonexistent_role",
							perm:     Known,
						},
						expected: &expected{success: false, message: "Unknown role"},
					},
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "",
							roleName: permission.RoleMember,
							perm:     Known,
						},
						expected: &expected{success: false, message: "TraceId is mandatory"},
					},
					{
						name: "Should fail when RoleName is empty",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							roleName: "",
							perm:     Known,
						},
						expected: &expected{success: false, message: "Role name is mandatory"},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							op:       permission.AuditActionRevoke,
							traceId:  "trace-test",
							roleName: permission.RoleMember,
							perm:     "",
						},
						expected: &expected{success: false, message: "Permission is mandatory"},
					},
				})
			})
		})
	})
}
