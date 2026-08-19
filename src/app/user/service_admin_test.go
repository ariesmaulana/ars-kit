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
		suite.Describe(t, "User ListUsers (admin)", func() {

			var Users []DataUser

			type input struct {
				actorId  int
				page     int
				pageSize int
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
				expectedUsers     int // number of rows expected in the page
				expectedTotal     int // total users in the database
			}

			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
			}

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{Idx: 0, Username: "adminuser", Email: "admin@example.com", FullName: "Admin User", Password: "password123"},
					{Idx: 1, Username: "userone", Email: "one@example.com", FullName: "User One", Password: "password123"},
					{Idx: 2, Username: "usertwo", Email: "two@example.com", FullName: "User Two", Password: "password123"},
					{Idx: 3, Username: "userthree", Email: "three@example.com", FullName: "User Three", Password: "password123"},
					{Idx: 4, Username: "userfour", Email: "four@example.com", FullName: "User Four", Password: "password123"},
				}
				for i, userData := range Users {
					inserted := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = inserted.Id
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					if r.permissionCheck != nil {
						return r.permissionCheck
					}
					return createFailedPermissionCheck()
				}

				output := app.Service.ListUsers(ctx, &user.ListUsersInput{
					TraceId:  "trace-test",
					ActorId:  r.input.actorId,
					Page:     r.input.page,
					PageSize: r.input.pageSize,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")

				if r.expected.success {
					assert.Equal(t, r.expected.expectedUsers, len(output.Users), r.name)
					assert.Equal(t, r.expected.expectedTotal, output.Total, r.name)
					// Rows must come back in ascending ID order.
					for i := 1; i < len(output.Users); i++ {
						assert.Less(t, output.Users[i-1].Id, output.Users[i].Id, r.name+" - users must be ordered by id")
					}
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
						name: "Should list all 5 users on page 1 with default page size",
						input: &input{
							actorId:  Users[0].Id,
							page:     1,
							pageSize: 20,
						},
						expected: &expected{
							success:           true,
							message:           "Users retrieved successfully",
							expectedCountMock: 1,
							expectedUsers:     5,
							expectedTotal:     5,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should return only the requested page of users",
						input: &input{
							actorId:  Users[0].Id,
							page:     1,
							pageSize: 2,
						},
						expected: &expected{
							success:           true,
							message:           "Users retrieved successfully",
							expectedCountMock: 1,
							expectedUsers:     2,
							expectedTotal:     5,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should return the last partial page",
						input: &input{
							actorId:  Users[0].Id,
							page:     3,
							pageSize: 2,
						},
						expected: &expected{
							success:           true,
							message:           "Users retrieved successfully",
							expectedCountMock: 1,
							expectedUsers:     1,
							expectedTotal:     5,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should return an empty page beyond the last one",
						input: &input{
							actorId:  Users[0].Id,
							page:     99,
							pageSize: 2,
						},
						expected: &expected{
							success:           true,
							message:           "Users retrieved successfully",
							expectedCountMock: 1,
							expectedUsers:     0,
							expectedTotal:     5,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when actor ID is zero",
						input: &input{
							actorId:  0,
							page:     1,
							pageSize: 20,
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when page is below 1",
						input: &input{
							actorId:  Users[0].Id,
							page:     0,
							pageSize: 20,
						},
						expected: &expected{
							success:           false,
							message:           "Page must be at least 1",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when page size is zero",
						input: &input{
							actorId:  Users[0].Id,
							page:     1,
							pageSize: 0,
						},
						expected: &expected{
							success:           false,
							message:           "Page size must be between 1 and 100",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when page size exceeds 100",
						input: &input{
							actorId:  Users[0].Id,
							page:     1,
							pageSize: 101,
						},
						expected: &expected{
							success:           false,
							message:           "Page size must be between 1 and 100",
							expectedCountMock: 0,
						},
					},

					// ===== Permission Tests =====
					{
						name: "Should fail when actor does not hold super user permission",
						input: &input{
							actorId:  Users[1].Id,
							page:     1,
							pageSize: 20,
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
							actorId:  Users[1].Id,
							page:     1,
							pageSize: 20,
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

func TestUserAdminGetUserById(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User AdminGetUserById", func() {

			var Users []DataUser

			type input struct {
				actorId int
				id      int
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
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
					{Idx: 0, Username: "adminuser", Email: "admin@example.com", FullName: "Admin User", Password: "password123"},
					{Idx: 1, Username: "targetuser", Email: "target@example.com", FullName: "Target User", Password: "password123"},
				}
				for i, userData := range Users {
					inserted := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = inserted.Id
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					if r.permissionCheck != nil {
						return r.permissionCheck
					}
					return createFailedPermissionCheck()
				}

				output := app.Service.AdminGetUserById(ctx, &user.AdminGetUserByIdInput{
					TraceId: "trace-test",
					ActorId: r.input.actorId,
					Id:      r.input.id,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")

				if r.expected.success {
					assert.Equal(t, r.input.id, output.User.Id, r.name)
					assert.Equal(t, r.expected.username, output.User.Username, r.name)
					assert.True(t, output.User.IsActive, r.name+" - fresh users are active")
				}
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "AdminGetUserById scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should look up an existing user",
						input: &input{
							actorId: Users[0].Id,
							id:      Users[1].Id,
						},
						expected: &expected{
							success:           true,
							message:           "User retrieved successfully",
							expectedCountMock: 1,
							username:          "targetuser",
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when actor ID is zero",
						input: &input{
							actorId: 0,
							id:      Users[1].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when user ID is zero",
						input: &input{
							actorId: Users[0].Id,
							id:      0,
						},
						expected: &expected{
							success:           false,
							message:           "User ID is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when user does not exist",
						input: &input{
							actorId: Users[0].Id,
							id:      99999,
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
							actorId: Users[1].Id,
							id:      Users[0].Id,
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can look up users",
							expectedCountMock: 1,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
				})
			})

		})
	})
}

func TestUserSetUserActive(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User SetUserActive", func() {

			var Users []DataUser

			type input struct {
				actorId  int
				userId   int
				isActive bool
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
				skipStateCheck    bool // set when the target row cannot exist (e.g. not-found case)
			}

			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
			}

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{Idx: 0, Username: "adminuser", Email: "admin@example.com", FullName: "Admin User", Password: "password123"},
					{Idx: 1, Username: "leaveruser", Email: "leaver@example.com", FullName: "Leaver User", Password: "password123"},
				}
				for i, userData := range Users {
					inserted := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = inserted.Id
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					if r.permissionCheck != nil {
						return r.permissionCheck
					}
					return createFailedPermissionCheck()
				}

				output := app.Service.SetUserActive(ctx, &user.SetUserActiveInput{
					TraceId:  "trace-test",
					ActorId:  r.input.actorId,
					UserId:   r.input.userId,
					IsActive: r.input.isActive,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")

				if r.expected.success {
					assert.Equal(t, r.input.userId, output.User.Id, r.name)
					assert.Equal(t, r.input.isActive, output.User.IsActive, r.name)
					// The change must be persisted.
					assert.Equal(t, r.input.isActive, app.Helper.IsUserActive(ctx, t, r.input.userId), r.name+" - db state")
				} else if !r.expected.skipStateCheck {
					// Failed operations must not change the stored state.
					before := app.Helper.IsUserActive(ctx, t, r.input.userId)
					app.Service.SetUserActive(ctx, &user.SetUserActiveInput{
						TraceId:  "trace-test",
						ActorId:  r.input.actorId,
						UserId:   r.input.userId,
						IsActive: !before, // attempt the opposite flip
					})
					assert.Equal(t, before, app.Helper.IsUserActive(ctx, t, r.input.userId), r.name+" - db state unchanged after failure")
				}
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "SetUserActive scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should deactivate a target user",
						input: &input{
							actorId:  Users[0].Id,
							userId:   Users[1].Id,
							isActive: false,
						},
						expected: &expected{
							success:           true,
							message:           "User deactivated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should reactivate a deactivated user",
						input: &input{
							actorId:  Users[0].Id,
							userId:   Users[1].Id,
							isActive: true,
						},
						expected: &expected{
							success:           true,
							message:           "User activated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when actor ID is zero",
						input: &input{
							actorId:  0,
							userId:   Users[1].Id,
							isActive: false,
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when user ID is zero",
						input: &input{
							actorId:  Users[0].Id,
							userId:   0,
							isActive: false,
						},
						expected: &expected{
							success:           false,
							message:           "User ID is mandatory",
							expectedCountMock: 0,
							skipStateCheck:    true,
						},
					},
					{
						name: "Should fail when the target user does not exist",
						input: &input{
							actorId:  Users[0].Id,
							userId:   99999,
							isActive: false,
						},
						expected: &expected{
							success:           false,
							message:           "User not found",
							expectedCountMock: 1,
							skipStateCheck:    true,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should fail when an admin tries to manage their own account",
						input: &input{
							actorId:  Users[0].Id,
							userId:   Users[0].Id,
							isActive: false,
						},
						expected: &expected{
							success:           false,
							message:           "You cannot manage your own account",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Permission Tests =====
					{
						name: "Should fail when actor does not hold super user permission",
						input: &input{
							actorId:  Users[1].Id,
							userId:   Users[0].Id,
							isActive: false,
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can manage user accounts",
							expectedCountMock: 1,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
				})
			})

		})
	})
}

func TestUserBootstrapSuperUser(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User BootstrapSuperUser", func() {

			type input struct {
				traceId  string
				username string
				email    string
				fullName string
				password string
			}
			type expected struct {
				success              bool
				message              string
				grantCalls           int
				userCreated          bool
				userCreatedOnFailure bool // account is committed before the grant, so a grant failure leaves a user behind (re-run repairs it)
				username             string
			}

			type testRow struct {
				name        string
				input       *input
				expected    *expected
				grantOutput *permission.GrantPermissionOutput
			}

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				grantCounter := &testsuite.Counter{}
				app.PermissionSvcMock.GrantPermissionStub = func(ctx context.Context, input *permission.GrantPermissionInput) *permission.GrantPermissionOutput {
					assert.Equal(t, "super_user", input.Permission, r.name)
					grantCounter.Inc()
					if r.grantOutput != nil {
						return r.grantOutput
					}
					return createSuccessfulGrant()
				}

				initialCount := app.Helper.CountUsers(ctx, t)

				output := app.Service.BootstrapSuperUser(ctx, &user.BootstrapSuperUserInput{
					TraceId:  r.input.traceId,
					Username: r.input.username,
					Email:    r.input.email,
					FullName: r.input.fullName,
					Password: r.input.password,
				})

				afterCount := app.Helper.CountUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.grantCalls, grantCounter.Total(), r.name+" - grant call count")

				if r.expected.success {
					assert.Equal(t, r.expected.username, output.User.Username, r.name)
					assert.True(t, output.User.IsActive, r.name)
					if r.expected.userCreated {
						assert.Equal(t, initialCount+1, afterCount, r.name+" - exactly one user created")
					} else {
						assert.Equal(t, initialCount, afterCount, r.name+" - existing user reused")
					}
					// The bootstrap user must be able to log in (password path).
					login := app.Service.Login(ctx, &user.LoginInput{
						TraceId:  "trace-test",
						Username: r.input.username,
						Password: r.input.password,
					})
					assert.True(t, login.Success, r.name+" - bootstrap user can log in")
				} else if !r.expected.userCreatedOnFailure {
					assert.Equal(t, initialCount, afterCount, r.name+" - no user created on failure")
				} else {
					// Grant failure happens after the user row is committed (the
					// account and the permission live in separate transactions).
					// The account exists; a re-run upgrades it and completes the grant.
					assert.Equal(t, initialCount+1, afterCount, r.name+" - account created before grant failure")
				}
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "BootstrapSuperUser scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should create the first super user and grant super_user",
						input: &input{
							traceId:  "trace-test",
							username: "rootadmin",
							email:    "root@example.com",
							fullName: "Root Admin",
							password: "password123",
						},
						expected: &expected{
							success:     true,
							message:     "Super user bootstrapped successfully",
							grantCalls:  1,
							userCreated: true,
							username:    "rootadmin",
						},
						grantOutput: createSuccessfulGrant(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when username is too short",
						input: &input{
							traceId:  "trace-test",
							username: "abcd",
							email:    "short@example.com",
							fullName: "Short User",
							password: "password123",
						},
						expected: &expected{
							success:    false,
							message:    "Username must be at least 5 characters long",
							grantCalls: 0,
						},
					},
					{
						name: "Should fail when password is too short",
						input: &input{
							traceId:  "trace-test",
							username: "weakpass",
							email:    "weak@example.com",
							fullName: "Weak Pass",
							password: "pass12",
						},
						expected: &expected{
							success:    false,
							message:    "Password must be at least 7 characters long",
							grantCalls: 0,
						},
					},
					{
						name: "Should fail when email is invalid",
						input: &input{
							traceId:  "trace-test",
							username: "bademail",
							email:    "not-an-email",
							fullName: "Bad Email",
							password: "password123",
						},
						expected: &expected{
							success:    false,
							message:    "Invalid email",
							grantCalls: 0,
						},
					},

					// ===== Permission Module Tests =====
					{
						name: "Should forward the message when the permission grant fails",
						input: &input{
							traceId:  "trace-test",
							username: "grantfail",
							email:    "fail@example.com",
							fullName: "Grant Fail",
							password: "password123",
						},
						expected: &expected{
							success:              false,
							message:              "Failed to grant permission",
							grantCalls:           1,
							userCreatedOnFailure: true,
						},
						grantOutput: createFailedGrant(),
					},
				})
			})

		})
	})
}

func TestUserBootstrapSuperUser_RerunIsIdempotent(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User BootstrapSuperUser re-run", func() {
			suite.Runs(t, "Should reuse an existing user and keep granting super_user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				app.PermissionSvcMock.GrantPermissionReturns(createSuccessfulGrant())

				first := app.Service.BootstrapSuperUser(ctx, &user.BootstrapSuperUserInput{
					TraceId:  "trace-test",
					Username: "rootadmin",
					Email:    "root@example.com",
					FullName: "Root Admin",
					Password: "password123",
				})
				assert.True(t, first.Success)

				// Second run with the same username must not create a duplicate.
				before := app.Helper.CountUsers(ctx, t)
				second := app.Service.BootstrapSuperUser(ctx, &user.BootstrapSuperUserInput{
					TraceId:  "trace-test",
					Username: "rootadmin",
					Email:    "different@example.com", // ignored: the user already exists
					FullName: "Root Admin",
					Password: "password123",
				})
				after := app.Helper.CountUsers(ctx, t)

				assert.True(t, second.Success)
				assert.Equal(t, before, after, "no duplicate user on re-run")
				assert.Equal(t, first.User.Id, second.User.Id, "same user id on re-run")
			})

			suite.Runs(t, "Should fail when the email is taken by another user", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				app.PermissionSvcMock.GrantPermissionReturns(createSuccessfulGrant())

				app.Service.BootstrapSuperUser(ctx, &user.BootstrapSuperUserInput{
					TraceId:  "trace-test",
					Username: "existing",
					Email:    "taken@example.com",
					FullName: "Existing",
					Password: "password123",
				})

				output := app.Service.BootstrapSuperUser(ctx, &user.BootstrapSuperUserInput{
					TraceId:  "trace-test",
					Username: "another",
					Email:    "taken@example.com",
					FullName: "Another",
					Password: "password123",
				})
				assert.False(t, output.Success)
				assert.Equal(t, "Username or email already exists", output.Message)
			})
		})
	})
}

func TestUserLogin_DeactivatedAccountRejected(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Login with deactivated account", func() {
			suite.Runs(t, "Should reject login for an inactive account", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUserWithHashedPassword(ctx, t, "leaver", "leaver@example.com", "Leaver", "password123")
				app.Helper.SetUserActiveDirect(ctx, t, u.Id, false)

				output := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-test",
					Username: "leaver",
					Password: "password123",
				})

				assert.False(t, output.Success)
				assert.Equal(t, "Account is deactivated", output.Message)
				assert.Equal(t, user.ErrorCodeForbidden, output.ErrorCode)
			})

			suite.Runs(t, "Should allow login again after reactivation", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUserWithHashedPassword(ctx, t, "returner", "returner@example.com", "Returner", "password123")
				app.Helper.SetUserActiveDirect(ctx, t, u.Id, false)
				app.Helper.SetUserActiveDirect(ctx, t, u.Id, true)

				output := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-test",
					Username: "returner",
					Password: "password123",
				})

				assert.True(t, output.Success)
			})
		})
	})
}
