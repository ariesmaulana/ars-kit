package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// TestAddMember
// ============================================================

func TestAddMember(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Add Member", func() {

			var existingUser *user.User

			suite.Setup(func(ctx context.Context, app *UserApp) {
				existingUser = app.Helper.InsertUser(ctx, t, "memberowner", "owner@example.com", "Member Owner", "password123")
			})

			type input struct {
				userId        int
				name          string
				monthlyIncome int
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

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				initialMembers := app.Helper.GetAllMembers(ctx, t)

				output := app.Service.AddMember(ctx, &user.AddMemberInput{
					TraceId:       "trace-test",
					Id:            r.input.userId,
					Name:          r.input.name,
					MonthlyIncome: r.input.monthlyIncome,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if !r.expected.success {
					assert.Equal(t, len(initialMembers), len(afterMembers), r.name)
					return
				}

				assert.Equal(t, len(initialMembers)+1, len(afterMembers), r.name)
				assert.NotZero(t, output.Member.Id, r.name)
				assert.Equal(t, r.input.userId, output.Member.UserId, r.name)
				assert.Equal(t, r.input.name, output.Member.Name, r.name)
				assert.Equal(t, r.input.monthlyIncome, output.Member.MonthlyIncome, r.name)
				assert.NotZero(t, output.Member.CreatedAt, r.name)
				assert.NotZero(t, output.Member.UpdatedAt, r.name)
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "Add member scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should add member successfully with valid inputs",
						input: &input{
							userId:        existingUser.Id,
							name:          "New Member",
							monthlyIncome: 5000000,
						},
						expected: &expected{success: true, message: "Member added successfully"},
					},
					{
						name: "Should add member with minimum name length (2 characters)",
						input: &input{
							userId:        existingUser.Id,
							name:          "AB",
							monthlyIncome: 1000000,
						},
						expected: &expected{success: true, message: "Member added successfully"},
					},
					{
						name: "Should add member with zero monthly income",
						input: &input{
							userId:        existingUser.Id,
							name:          "Zero Income Member",
							monthlyIncome: 0,
						},
						expected: &expected{success: true, message: "Member added successfully"},
					},

					// ===== Validation: User =====
					{
						name: "Should fail when user does not exist",
						input: &input{
							userId:        99999,
							name:          "Ghost Member",
							monthlyIncome: 5000000,
						},
						expected: &expected{success: false, message: "User not found"},
					},

					// ===== Validation: Name =====
					{
						name: "Should fail when name is empty",
						input: &input{
							userId:        existingUser.Id,
							name:          "",
							monthlyIncome: 5000000,
						},
						expected: &expected{success: false, message: "Member name is mandatory"},
					},
					{
						name: "Should fail when name is too short (1 character)",
						input: &input{
							userId:        existingUser.Id,
							name:          "A",
							monthlyIncome: 5000000,
						},
						expected: &expected{success: false, message: "Member name must be at least 2 characters long"},
					},

					// ===== Validation: Income =====
					{
						name: "Should fail when monthly income is negative",
						input: &input{
							userId:        existingUser.Id,
							name:          "Negative Income Member",
							monthlyIncome: -1,
						},
						expected: &expected{success: false, message: "Monthly income cannot be negative"},
					},
				})
			})
		})
	})
}

// ============================================================
// TestGetMemberById
// ============================================================

func TestGetMemberById(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Get Member By ID", func() {

			var existingMember *user.Member

			suite.Setup(func(ctx context.Context, app *UserApp) {
				u := app.Helper.InsertUser(ctx, t, "memberuser", "memberuser@example.com", "Member User", "password")
				existingMember = app.Helper.InsertMember(ctx, t, u.Id, "Test Member", 5000000)
			})

			suite.Run(t, "Should get existing member by ID", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMemberById(ctx, &user.GetMemberByIdInput{
					TraceId:  "trace-test",
					MemberId: existingMember.Id,
				})

				assert.True(t, output.Success)
				assert.Equal(t, "Member retrieved successfully", output.Message)
				assert.Equal(t, existingMember.Id, output.Member.Id)
				assert.Equal(t, existingMember.UserId, output.Member.UserId)
				assert.Equal(t, existingMember.Name, output.Member.Name)
				assert.Equal(t, existingMember.MonthlyIncome, output.Member.MonthlyIncome)
				assert.NotZero(t, output.Member.CreatedAt)
				assert.NotZero(t, output.Member.UpdatedAt)
			})

			suite.Run(t, "Should fail when member ID is 0", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMemberById(ctx, &user.GetMemberByIdInput{
					TraceId:  "trace-test",
					MemberId: 0,
				})

				assert.False(t, output.Success)
				assert.Equal(t, "Member ID is mandatory", output.Message)
			})

			suite.Run(t, "Should fail when member does not exist", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMemberById(ctx, &user.GetMemberByIdInput{
					TraceId:  "trace-test",
					MemberId: 99999,
				})

				assert.False(t, output.Success)
				assert.Equal(t, "Member not found", output.Message)
			})
		})
	})
}

// ============================================================
// TestGetMembersByUserIdPagination — covers pagination behaviour
// added after GetMembersByUserId was extended with page/page_size.
// Basic CRUD scenarios are covered by TestGetMembersByUserId in service_test.go.
// ============================================================

func TestGetMembersByUserIdPagination(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Get Members By User ID - Pagination", func() {

			var userWithMembers *user.User
			var member1, member2, member3 *user.Member

			suite.Setup(func(ctx context.Context, app *UserApp) {
				userWithMembers = app.Helper.InsertUser(ctx, t, "pguser", "pguser@example.com", "Pg User", "password")
				member1 = app.Helper.InsertMember(ctx, t, userWithMembers.Id, "Member One", 1000000)
				member2 = app.Helper.InsertMember(ctx, t, userWithMembers.Id, "Member Two", 2000000)
				member3 = app.Helper.InsertMember(ctx, t, userWithMembers.Id, "Member Three", 3000000)
			})

			suite.Run(t, "Should return page 1 of 2 with correct metadata", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMembersByUserId(ctx, &user.GetMembersByUserIdInput{
					TraceId:  "trace-test",
					UserId:   userWithMembers.Id,
					Page:     1,
					PageSize: 2,
				})

				assert.True(t, output.Success)
				assert.Equal(t, 3, output.Total)
				assert.Equal(t, 2, output.TotalPages)
				assert.Equal(t, 1, output.Page)
				assert.Equal(t, 2, output.PageSize)
				assert.Len(t, output.Members, 2)
				assert.Equal(t, member1.Id, output.Members[0].Id)
				assert.Equal(t, member2.Id, output.Members[1].Id)
			})

			suite.Run(t, "Should return page 2 of 2 with remaining member", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMembersByUserId(ctx, &user.GetMembersByUserIdInput{
					TraceId:  "trace-test",
					UserId:   userWithMembers.Id,
					Page:     2,
					PageSize: 2,
				})

				assert.True(t, output.Success)
				assert.Equal(t, 3, output.Total)
				assert.Equal(t, 2, output.TotalPages)
				assert.Equal(t, 2, output.Page)
				assert.Equal(t, 2, output.PageSize)
				assert.Len(t, output.Members, 1)
				assert.Equal(t, member3.Id, output.Members[0].Id)
			})

			suite.Run(t, "Should default to page 1 and size 10 when inputs are zero", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMembersByUserId(ctx, &user.GetMembersByUserIdInput{
					TraceId: "trace-test",
					UserId:  userWithMembers.Id,
					// Page and PageSize intentionally omitted (zero values → defaults)
				})

				assert.True(t, output.Success)
				assert.Equal(t, 1, output.Page)
				assert.Equal(t, 10, output.PageSize)
				assert.Len(t, output.Members, 3)
			})

			suite.Run(t, "Should cap page_size at 100", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMembersByUserId(ctx, &user.GetMembersByUserIdInput{
					TraceId:  "trace-test",
					UserId:   userWithMembers.Id,
					Page:     1,
					PageSize: 9999,
				})

				assert.True(t, output.Success)
				assert.Equal(t, 100, output.PageSize)
			})

			suite.Run(t, "Should return total and total_pages in response", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.GetMembersByUserId(ctx, &user.GetMembersByUserIdInput{
					TraceId:  "trace-test",
					UserId:   userWithMembers.Id,
					Page:     1,
					PageSize: 10,
				})

				assert.True(t, output.Success)
				assert.Equal(t, 3, output.Total)
				assert.Equal(t, 1, output.TotalPages)
			})
		})
	})
}

// ============================================================
// TestUpdateMemberInfo
// ============================================================

func TestUpdateMemberInfo(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Update Member Info", func() {

			var ownerUser *user.User
			var otherUser *user.User
			var existingMember *user.Member

			suite.Setup(func(ctx context.Context, app *UserApp) {
				ownerUser = app.Helper.InsertUser(ctx, t, "memberowner", "owner@example.com", "Member Owner", "password")
				otherUser = app.Helper.InsertUser(ctx, t, "otheruser", "other@example.com", "Other User", "password")
				existingMember = app.Helper.InsertMember(ctx, t, ownerUser.Id, "Original Name", 3000000)
			})

			type input struct {
				requesterId   int
				memberId      int
				name          string
				monthlyIncome int
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

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				output := app.Service.UpdateMemberInfo(ctx, &user.UpdateMemberInfoInput{
					TraceId:       "trace-test",
					RequesterId:   r.input.requesterId,
					Id:            r.input.memberId,
					Name:          r.input.name,
					MonthlyIncome: r.input.monthlyIncome,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success {
					updated := app.Helper.GetMemberById(ctx, t, r.input.memberId)
					assert.Equal(t, r.input.name, updated.Name, r.name)
					assert.Equal(t, r.input.monthlyIncome, updated.MonthlyIncome, r.name)
				}
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "Update member info scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should update member info successfully",
						input: &input{
							requesterId:   ownerUser.Id,
							memberId:      existingMember.Id,
							name:          "Updated Name",
							monthlyIncome: 9000000,
						},
						expected: &expected{success: true, message: "Member info updated successfully"},
					},
					{
						name: "Should update member with minimum name length (2 characters)",
						input: &input{
							requesterId:   ownerUser.Id,
							memberId:      existingMember.Id,
							name:          "XY",
							monthlyIncome: 0,
						},
						expected: &expected{success: true, message: "Member info updated successfully"},
					},
					{
						name: "Should update member with zero monthly income",
						input: &input{
							requesterId:   ownerUser.Id,
							memberId:      existingMember.Id,
							name:          "Zero Income",
							monthlyIncome: 0,
						},
						expected: &expected{success: true, message: "Member info updated successfully"},
					},

					// ===== Authorization =====
					{
						name: "Should fail when requester is not the owner",
						input: &input{
							requesterId:   otherUser.Id,
							memberId:      existingMember.Id,
							name:          "Stolen Name",
							monthlyIncome: 0,
						},
						expected: &expected{success: false, message: "Unauthorized update"},
					},

					// ===== Not Found =====
					{
						name: "Should fail when member does not exist",
						input: &input{
							requesterId:   ownerUser.Id,
							memberId:      99999,
							name:          "Ghost Member",
							monthlyIncome: 0,
						},
						expected: &expected{success: false, message: "Member not found"},
					},

					// ===== Validation: Name =====
					{
						name: "Should fail when name is empty",
						input: &input{
							requesterId:   ownerUser.Id,
							memberId:      existingMember.Id,
							name:          "",
							monthlyIncome: 0,
						},
						expected: &expected{success: false, message: "Member name is mandatory"},
					},
					{
						name: "Should fail when name is too short (1 character)",
						input: &input{
							requesterId:   ownerUser.Id,
							memberId:      existingMember.Id,
							name:          "A",
							monthlyIncome: 0,
						},
						expected: &expected{success: false, message: "Member name must be at least 2 characters long"},
					},

					// ===== Validation: Income =====
					{
						name: "Should fail when monthly income is negative",
						input: &input{
							requesterId:   ownerUser.Id,
							memberId:      existingMember.Id,
							name:          "Valid Name",
							monthlyIncome: -100,
						},
						expected: &expected{success: false, message: "Monthly income cannot be negative"},
					},
				})
			})
		})
	})
}

// ============================================================
// TestDeleteMember
// ============================================================

func TestDeleteMember(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Delete Member", func() {

			var ownerUser *user.User
			var otherUser *user.User
			var ownedMember *user.Member
			var otherMember *user.Member

			suite.Setup(func(ctx context.Context, app *UserApp) {
				ownerUser = app.Helper.InsertUser(ctx, t, "deleteowner", "deleteowner@example.com", "Delete Owner", "password")
				otherUser = app.Helper.InsertUser(ctx, t, "deleteother", "deleteother@example.com", "Delete Other", "password")
				ownedMember = app.Helper.InsertMember(ctx, t, ownerUser.Id, "Owned Member", 5000000)
				otherMember = app.Helper.InsertMember(ctx, t, otherUser.Id, "Other Member", 3000000)
			})

			suite.Run(t, "Should delete member successfully", func(t *testing.T, ctx context.Context, app *UserApp) {
				initialMembers := app.Helper.GetAllMembers(ctx, t)

				output := app.Service.DeleteMember(ctx, &user.DeleteMemberInput{
					TraceId:     "trace-test",
					RequesterId: ownerUser.Id,
					Id:          ownedMember.Id,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.True(t, output.Success)
				assert.Equal(t, "Member deleted successfully", output.Message)
				assert.Equal(t, len(initialMembers)-1, len(afterMembers))
				_, exists := afterMembers[ownedMember.Id]
				assert.False(t, exists, "member should no longer exist in DB")
			})

			suite.Run(t, "Should return success when deleting an already-deleted member (idempotent)", func(t *testing.T, ctx context.Context, app *UserApp) {
				// Create a temp member and delete it twice
				tempMember := app.Helper.InsertMember(ctx, t, ownerUser.Id, "Temp Idempotent Member", 0)

				output1 := app.Service.DeleteMember(ctx, &user.DeleteMemberInput{
					TraceId:     "trace-test",
					RequesterId: ownerUser.Id,
					Id:          tempMember.Id,
				})
				assert.True(t, output1.Success)

				// Second delete — member is already gone, should still succeed
				output2 := app.Service.DeleteMember(ctx, &user.DeleteMemberInput{
					TraceId:     "trace-test",
					RequesterId: ownerUser.Id,
					Id:          tempMember.Id,
				})

				assert.True(t, output2.Success)
				assert.Equal(t, "Member deleted successfully", output2.Message)
			})

			suite.Run(t, "Should fail when requester is not the owner", func(t *testing.T, ctx context.Context, app *UserApp) {
				initialMembers := app.Helper.GetAllMembers(ctx, t)

				// ownerUser tries to delete a member that belongs to otherUser
				output := app.Service.DeleteMember(ctx, &user.DeleteMemberInput{
					TraceId:     "trace-test",
					RequesterId: ownerUser.Id,
					Id:          otherMember.Id,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.False(t, output.Success)
				assert.Equal(t, "Unauthorized delete", output.Message)
				assert.Equal(t, len(initialMembers), len(afterMembers), "no member should be deleted")
			})

			suite.Run(t, "Should fail when requester does not exist", func(t *testing.T, ctx context.Context, app *UserApp) {
				output := app.Service.DeleteMember(ctx, &user.DeleteMemberInput{
					TraceId:     "trace-test",
					RequesterId: 99999,
					Id:          ownedMember.Id,
				})

				assert.False(t, output.Success)
				assert.Equal(t, "Unauthorized delete", output.Message)
			})
		})
	})
}
