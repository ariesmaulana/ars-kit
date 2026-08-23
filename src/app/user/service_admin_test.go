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
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
					},
				}

				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
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
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
					},
				}

				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
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
							success:           true,
							message:           "User deleted successfully",
							expectedCountMock: 0,
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
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
					},
				}

				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
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
