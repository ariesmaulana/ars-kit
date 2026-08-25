package permission_test

import (
	"context"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/clock"
	"github.com/stretchr/testify/assert"
)

// TestPermissionAuditTrail verifies that GrantPermission and RevokePermission
// record who granted/revoked what, when — in the same transaction as the
// permission change itself. Time comes from the internal clock package,
// pinned here so created_at is asserted exactly.
func TestPermissionAuditTrail(t *testing.T) {
	// Pin time BEFORE RunTest so every scenario observes the same instant.
	fixedNow := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	clock.SetSource(clock.Fixed(fixedNow))
	t.Cleanup(clock.Reset)

	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Permission Audit Trail", func() {

			// ========== 1. Declare Fixture Variables ==========
			var (
				Actor       DataUser
				SystemActor DataUser // ID 0: no acting user (workflow/system)
				Target      DataUser
			)

			// ========== 2. Define Test Structures ==========
			type input struct {
				op      string // permission.AuditActionGrant or AuditActionRevoke
				traceId string
				actorID int
				userID  int
				perm    string
			}
			type expectedAudit struct {
				actorID    *int
				targetID   int
				permission string
				action     string
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

				// Catalog rows would be inserted via SOP in production.
				app.Helper.AddKnownPermission(ctx, t, "read:profile")
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *PermissionApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				before := app.Helper.GetPermissionAudit(ctx, t, r.input.userID)

				// Execute the service method being tested
				var success bool
				var message string
				switch r.input.op {
				case permission.AuditActionGrant:
					out := app.Service.GrantPermission(ctx, &permission.GrantPermissionInput{
						TraceId:    r.input.traceId,
						UserID:     r.input.userID,
						Permission: r.input.perm,
						ActorId:    r.input.actorID,
					})
					success, message = out.Success, out.Message
				case permission.AuditActionRevoke:
					out := app.Service.RevokePermission(ctx, &permission.RevokePermissionInput{
						TraceId:    r.input.traceId,
						UserID:     r.input.userID,
						Permission: r.input.perm,
						ActorId:    r.input.actorID,
					})
					success, message = out.Success, out.Message
				}

				// Get state AFTER operation
				after := app.Helper.GetPermissionAudit(ctx, t, r.input.userID)

				// Assert response matches expectations
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
				assert.Equal(t, want.permission, got.Permission, r.name)
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
			suite.Run(t, "AuditTrail scenarios", func(t *testing.T, ctx context.Context, app *PermissionApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should record audit row when actor grants permission",
						input: &input{
							op:      permission.AuditActionGrant,
							traceId: "trace-test",
							actorID: Actor.ID,
							userID:  Target.ID,
							perm:    "read:profile",
						},
						expected: &expected{
							success:      true,
							message:      "Permission granted successfully",
							newAuditRows: 1,
							lastAudit: &expectedAudit{
								actorID:    &Actor.ID,
								targetID:   Target.ID,
								permission: "read:profile",
								action:     permission.AuditActionGrant,
							},
						},
					},
					{
						name: "Should record audit row when actor revokes permission",
						input: &input{
							op:      permission.AuditActionRevoke,
							traceId: "trace-test",
							actorID: Actor.ID,
							userID:  Target.ID,
							perm:    "read:profile",
						},
						expected: &expected{
							success:      true,
							message:      "Permission revoked successfully",
							newAuditRows: 1,
							lastAudit: &expectedAudit{
								actorID:    &Actor.ID,
								targetID:   Target.ID,
								permission: "read:profile",
								action:     permission.AuditActionRevoke,
							},
						},
					},
					{
						name: "Should store NULL actor when grant is system-initiated",
						input: &input{
							op:      permission.AuditActionGrant,
							traceId: "trace-test",
							actorID: SystemActor.ID, // 0: no acting user
							userID:  Target.ID,
							perm:    "read:profile",
						},
						expected: &expected{
							success:      true,
							message:      "Permission granted successfully",
							newAuditRows: 1,
							lastAudit: &expectedAudit{
								actorID:    nil,
								targetID:   Target.ID,
								permission: "read:profile",
								action:     permission.AuditActionGrant,
							},
						},
					},

					// ===== Rejection Tests =====
					{
						name: "Should not record audit row when permission is not in the catalog",
						input: &input{
							op:      permission.AuditActionGrant,
							traceId: "trace-test",
							actorID: Actor.ID,
							userID:  Target.ID,
							perm:    "garbage:permission",
						},
						expected: &expected{
							success:      false,
							message:      "Unknown permission",
							newAuditRows: 0,
						},
					},
					{
						name: "Should not record audit row when TraceId is empty",
						input: &input{
							op:      permission.AuditActionRevoke,
							traceId: "",
							actorID: Actor.ID,
							userID:  Target.ID,
							perm:    "read:profile",
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
