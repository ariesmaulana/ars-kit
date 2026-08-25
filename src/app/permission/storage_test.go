package permission_test

import (
	"context"
	"testing"
	"time"

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

func TestStorageInsertPermissionAudit(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage InsertPermissionAudit", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser
			var now time.Time

			// ========== 2. Define Test Structures ==========
			type input struct {
				actorID  int
				targetID int
				perm     string
				action   string
				at       time.Time
			}
			type expected struct {
				audit permission.PermissionAudit
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
				now = time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC)
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				initialLen := len(app.Helper.GetPermissionAudit(ctx, t, r.input.targetID))

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err, r.name)
				defer tx.Rollback()

				err = tx.InsertPermissionAudit(ctx, r.input.actorID, r.input.targetID, r.input.perm, r.input.action, r.input.at)
				assert.Nil(t, err, r.name)

				assert.Nil(t, tx.Commit(), r.name)

				// Scenarios share the database; assert only on the row this
				// scenario just appended.
				audits := app.Helper.GetPermissionAudit(ctx, t, r.input.targetID)
				if assert.Len(t, audits, initialLen+1, r.name) {
					got := audits[len(audits)-1]
					assert.Equal(t, r.expected.audit.ActorId, got.ActorId, r.name)
					assert.Equal(t, r.expected.audit.TargetId, got.TargetId, r.name)
					assert.Equal(t, r.expected.audit.Permission, got.Permission, r.name)
					assert.Equal(t, r.expected.audit.Action, got.Action, r.name)
					assert.True(t, got.CreatedAt.Equal(r.expected.audit.CreatedAt), "%s: created_at mismatch: got %v want %v", r.name, got.CreatedAt, r.expected.audit.CreatedAt)
				}
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *PermissionApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "InsertPermissionAudit scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should record a grant by an acting user",
						input: &input{
							actorID:  Users[1].ID,
							targetID: Users[0].ID,
							perm:     "read:profile",
							action:   permission.AuditActionGrant,
							at:       now,
						},
						expected: &expected{audit: permission.PermissionAudit{
							ActorId:    &Users[1].ID,
							TargetId:   Users[0].ID,
							Permission: "read:profile",
							Action:     permission.AuditActionGrant,
							CreatedAt:  now,
						}},
					},
					{
						name: "Should record a revoke by an acting user",
						input: &input{
							actorID:  Users[1].ID,
							targetID: Users[0].ID,
							perm:     "read:profile",
							action:   permission.AuditActionRevoke,
							at:       now,
						},
						expected: &expected{audit: permission.PermissionAudit{
							ActorId:    &Users[1].ID,
							TargetId:   Users[0].ID,
							Permission: "read:profile",
							Action:     permission.AuditActionRevoke,
							CreatedAt:  now,
						}},
					},
					{
						name: "Should store NULL actor for system-initiated grant (actorID 0)",
						input: &input{
							actorID:  0,
							targetID: Users[1].ID,
							perm:     "write:profile",
							action:   permission.AuditActionGrant,
							at:       now,
						},
						expected: &expected{audit: permission.PermissionAudit{
							ActorId:    nil,
							TargetId:   Users[1].ID,
							Permission: "write:profile",
							Action:     permission.AuditActionGrant,
							CreatedAt:  now,
						}},
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
