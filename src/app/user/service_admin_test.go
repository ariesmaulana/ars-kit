package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
)

func TestUserListUsers(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User ListUsers", func() {

			var Users []DataUser

			type input struct {
				traceId string
				actorId int
				page    int
				size    int
				filter  string
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
				total             int
				usernames         []string
			}

			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
			}

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "testuser1",
						Email:    "test1@example.com",
						FullName: "Test User 1",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
				}

				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password, userData.Status)
					Users[i].Id = insertedUser.Id
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					return r.permissionCheck
				}

				output := app.Service.ListUsers(ctx, &user.ListUsersInput{
					TraceId: r.input.traceId,
					ActorId: r.input.actorId,
					Page:    r.input.page,
					Size:    r.input.size,
					Filter:  r.input.filter,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")

				if r.expected.success == false {
					return
				}

				assert.Equal(t, r.expected.total, output.Total, r.name)
				listed := make([]string, 0, len(output.Users))
				for _, u := range output.Users {
					listed = append(listed, u.Username)
				}
				for _, want := range r.expected.usernames {
					assert.Contains(t, listed, want, r.name)
				}
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "ListUsers scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should list users successfully when actor holds super user permission",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							page:    1,
							size:    10,
							filter:  "",
						},
						expected: &expected{
							success:           true,
							message:           "Users retrieved successfully",
							expectedCountMock: 1,
							total:             2,
							usernames:         []string{"testuser1", "testuser2"},
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should filter users by username or email substring",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							page:    1,
							size:    10,
							filter:  "testuser1",
						},
						expected: &expected{
							success:           true,
							message:           "Users retrieved successfully",
							expectedCountMock: 1,
							total:             1,
							usernames:         []string{"testuser1"},
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							actorId: Users[0].Id,
							page:    1,
							size:    10,
							filter:  "",
						},
						expected: &expected{
							success:           false,
							message:           "TraceId is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when Actor ID is zero",
						input: &input{
							traceId: "trace-test",
							actorId: 0,
							page:    1,
							size:    10,
							filter:  "",
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
						},
					},

					// ===== Permission Tests =====
					{
						name: "Should fail when actor does not hold super user permission",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							page:    1,
							size:    10,
							filter:  "",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can list users",
							expectedCountMock: 1,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
					{
						name: "Should fail when permission module cannot confirm the check",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							page:    1,
							size:    10,
							filter:  "",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can list users",
							expectedCountMock: 1,
						},
						permissionCheck: createFailedPermissionCheck(),
					},
				})
			})

		})
	})
}

func TestUserGetUser(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User GetUser", func() {

			var Users []DataUser

			type input struct {
				traceId string
				actorId int
				userID  int
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
				userID            int
				username          string
			}

			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
			}

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "testuser1",
						Email:    "test1@example.com",
						FullName: "Test User 1",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
				}

				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password, userData.Status)
					Users[i].Id = insertedUser.Id
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					return r.permissionCheck
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.GetUser(ctx, &user.GetUserInput{
					TraceId: r.input.traceId,
					ActorId: r.input.actorId,
					Id:      r.input.userID,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")

				if r.expected.success == false {
					// Read operation must not mutate the users table
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}

				assert.NotZero(t, output.User.Id, r.name)
				assert.Equal(t, r.expected.userID, output.User.Id, r.name)
				assert.Equal(t, r.expected.username, output.User.Username, r.name)
				assert.Equal(t, initialUsers, afterUsers, r.name)
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "GetUser scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should get user successfully for first user",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  Users[0].Id,
						},
						expected: &expected{
							success:           true,
							message:           "User retrieved successfully",
							expectedCountMock: 1,
							userID:            Users[0].Id,
							username:          "testuser1",
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should get user successfully for second user",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  Users[1].Id,
						},
						expected: &expected{
							success:           true,
							message:           "User retrieved successfully",
							expectedCountMock: 1,
							userID:            Users[1].Id,
							username:          "testuser2",
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests: User ID =====
					{
						name: "Should fail when user ID is zero",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  0,
						},
						expected: &expected{
							success:           false,
							message:           "User ID is mandatory",
							expectedCountMock: 0,
						},
					},

					// ===== Validation Tests: TraceId / ActorId =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							actorId: Users[0].Id,
							userID:  Users[0].Id,
						},
						expected: &expected{
							success:           false,
							message:           "TraceId is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when Actor ID is zero",
						input: &input{
							traceId: "trace-test",
							actorId: 0,
							userID:  Users[0].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
						},
					},

					// ===== Not Found Test =====
					{
						name: "Should succeed when user does not exist",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  99999,
						},
						expected: &expected{
							success:           false,
							message:           "User not found",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Permission Tests =====
					{
						name: "Should fail when actor does not hold super user permission",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  Users[1].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can view users",
							expectedCountMock: 1,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
					{
						name: "Should fail when permission module cannot confirm the check",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  Users[1].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can view users",
							expectedCountMock: 1,
						},
						permissionCheck: createFailedPermissionCheck(),
					},
				})
			})

		})
	})
}

func TestUserDeleteUser(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User DeleteUser", func() {

			var Users []DataUser

			type input struct {
				traceId string
				actorId int
				userID  int
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
			}

			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
			}

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "testuser1",
						Email:    "test1@example.com",
						FullName: "Test User 1",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
				}

				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password, userData.Status)
					Users[i].Id = insertedUser.Id
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					return r.permissionCheck
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.DeleteUser(ctx, &user.DeleteUserInput{
					TraceId: r.input.traceId,
					ActorId: r.input.actorId,
					Id:      r.input.userID,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")

				if r.expected.success == false {
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}
				if r.input.userID == 99999 {
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}

				assert.Equal(t, len(initialUsers)-1, len(afterUsers), r.name)
				_, exists := afterUsers[r.input.userID]
				assert.False(t, exists, r.name+" - deleted user should be gone")
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "DeleteUser scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					{
						name: "Should delete user successfully when actor holds super user permission",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  Users[0].Id,
						},
						expected: &expected{
							success:           true,
							message:           "User deleted successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should fail when user ID is zero",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  0,
						},
						expected: &expected{
							success:           false,
							message:           "User ID is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							actorId: Users[0].Id,
							userID:  Users[0].Id,
						},
						expected: &expected{
							success:           false,
							message:           "TraceId is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when Actor ID is zero",
						input: &input{
							traceId: "trace-test",
							actorId: 0,
							userID:  Users[0].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should succeed when user does not exist",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  99999,
						},
						expected: &expected{
							success:           true,
							message:           "User deleted successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should fail when actor does not hold super user permission",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  Users[1].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can delete users",
							expectedCountMock: 1,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
					{
						name: "Should fail when permission module cannot confirm the check",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							userID:  Users[1].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can delete users",
							expectedCountMock: 1,
						},
						permissionCheck: createFailedPermissionCheck(),
					},
				})
			})

		})
	})
}

func TestUserUpdateStatus(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User UpdateUserStatus", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				traceId string
				actorId int
				target  int
				status  user.UserStatus
			}
			type expected struct {
				success             bool
				message             string
				errorCode           user.ErrorCode
				statusAfter         user.UserStatus
				activeTokensRevoked bool // expect active refresh tokens to be revoked
			}
			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "admin1",
						Email:    "admin1@example.com",
						FullName: "Admin One",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
					{
						Idx:      1,
						Username: "target1",
						Email:    "target1@example.com",
						FullName: "Target One",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
				}

				for i, userData := range Users {
					inserted := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password, userData.Status)
					Users[i].Id = inserted.Id
				}
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, user.PermissionSuperUser, input.Permission, r.name)
					return r.permissionCheck
				}

				// Seed an active refresh token for the target so revocation is observable.
				if r.input.target != 0 && r.expected.activeTokensRevoked {
					app.Helper.InsertActiveRefreshToken(ctx, t, r.input.target)
				}

				beforeActive := app.Helper.CountActiveRefreshTokens(ctx, t, r.input.target)

				output := app.Service.UpdateUserStatus(ctx, &user.UpdateUserStatusInput{
					TraceId:      r.input.traceId,
					ActorId:      r.input.actorId,
					TargetUserId: r.input.target,
					Status:       r.input.status,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.errorCode, output.ErrorCode, r.name)

				if r.input.target == 0 || !r.permissionCheck.HasPermission {
					return
				}

				got := app.Helper.GetUserById(ctx, t, r.input.target)
				if got == nil {
					return
				}
				assert.Equal(t, r.expected.statusAfter, got.Status, r.name)

				if r.expected.activeTokensRevoked && beforeActive > 0 {
					assert.Zero(t, app.Helper.CountActiveRefreshTokens(ctx, t, r.input.target), r.name)
				}
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "UpdateUserStatus scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should suspend a user successfully and revoke sessions",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  Users[1].Id,
							status:  user.UserStatusSuspended,
						},
						expected: &expected{
							success:             true,
							message:             "User status updated successfully",
							statusAfter:         user.UserStatusSuspended,
							activeTokensRevoked: true,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should disable a user successfully",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  Users[1].Id,
							status:  user.UserStatusDisabled,
						},
						expected: &expected{
							success:             true,
							message:             "User status updated successfully",
							statusAfter:         user.UserStatusDisabled,
							activeTokensRevoked: true,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should reactivate a suspended user without revoking tokens",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  Users[1].Id,
							status:  user.UserStatusActive,
						},
						expected: &expected{
							success:             true,
							message:             "User status updated successfully",
							statusAfter:         user.UserStatusActive,
							activeTokensRevoked: false,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},

					// ===== Authorization Tests =====
					{
						name: "Should fail when actor lacks super user permission",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  Users[1].Id,
							status:  user.UserStatusSuspended,
						},
						expected: &expected{
							success:     false,
							message:     "Unauthorized: only super user can update user status",
							errorCode:   user.ErrorCodeForbidden,
							statusAfter: user.UserStatusActive,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: false},
					},
					{
						name: "Should reject disabling own account",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  Users[0].Id,
							status:  user.UserStatusSuspended,
						},
						expected: &expected{
							success:     false,
							message:     "Cannot disable your own account",
							errorCode:   user.ErrorCodeValidation,
							statusAfter: user.UserStatusActive,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId: "",
							actorId: Users[0].Id,
							target:  Users[1].Id,
							status:  user.UserStatusSuspended,
						},
						expected: &expected{
							success:     false,
							message:     "TraceId is mandatory",
							errorCode:   user.ErrorCodeValidation,
							statusAfter: user.UserStatusActive,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should fail when ActorId is empty",
						input: &input{
							traceId: "trace-test",
							actorId: 0,
							target:  Users[1].Id,
							status:  user.UserStatusSuspended,
						},
						expected: &expected{
							success:     false,
							message:     "Actor ID is mandatory",
							errorCode:   user.ErrorCodeValidation,
							statusAfter: user.UserStatusActive,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should fail when TargetUserId is empty",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  0,
							status:  user.UserStatusSuspended,
						},
						expected: &expected{
							success:     false,
							message:     "Target user ID is mandatory",
							errorCode:   user.ErrorCodeValidation,
							statusAfter: user.UserStatusActive,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should fail when Status is empty",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  Users[1].Id,
							status:  "",
						},
						expected: &expected{
							success:     false,
							message:     "Status is mandatory",
							errorCode:   user.ErrorCodeValidation,
							statusAfter: user.UserStatusActive,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should fail when Status is not a known enum value",
						input: &input{
							traceId: "trace-test",
							actorId: Users[0].Id,
							target:  Users[1].Id,
							status:  user.UserStatus("banned"),
						},
						expected: &expected{
							success:     false,
							message:     "Invalid status",
							errorCode:   user.ErrorCodeValidation,
							statusAfter: user.UserStatusActive,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
				})
			})
		})
	})
}
