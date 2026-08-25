package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/ariesmaulana/ars-kit/src/clock"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
)

func TestUserRegister(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Register", func() {

			type input struct {
				username string
				email    string
				fullName string
				password string
			}
			type expected struct {
				success     bool
				message     string
				userCreated bool
			}

			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.Register(ctx, &user.RegisterInput{
					TraceId:  "trace-test",
					Username: r.input.username,
					Email:    r.input.email,
					FullName: r.input.fullName,
					Password: r.input.password,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no users were created
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}

				// For successful registration
				assert.Equal(t, len(initialUsers)+1, len(afterUsers), r.name)
				assert.NotZero(t, output.User.Id, r.name)
				assert.Equal(t, r.input.username, output.User.Username, r.name)
				assert.Equal(t, r.input.email, output.User.Email, r.name)
				assert.Equal(t, r.input.fullName, output.User.FullName, r.name)
				assert.NotZero(t, output.User.CreatedAt, r.name)
				assert.NotZero(t, output.User.UpdatedAt, r.name)

				// Verify password is hashed (not stored as plain text)
				storedPassword := app.Helper.GetUserPassword(ctx, t, output.User.Id)
				assert.NotEqual(t, r.input.password, storedPassword, r.name+" - password should be hashed")
				assert.NotEmpty(t, storedPassword, r.name)
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "Register scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should register user successfully with valid inputs",
						input: &input{
							username: "testuser",
							email:    "test@example.com",
							fullName: "Test User",
							password: "password12345",
						},
						expected: &expected{
							success:     true,
							message:     "User registered successfully",
							userCreated: true,
						},
					},
					{
						name: "Should register user with minimum username length (5 characters)",
						input: &input{
							username: "user5",
							email:    "user5@example.com",
							fullName: "User Five",
							password: "password12345",
						},
						expected: &expected{
							success:     true,
							message:     "User registered successfully",
							userCreated: true,
						},
					},
					{
						name: "Should register user with minimum password length (12 characters)",
						input: &input{
							username: "user12char",
							email:    "user12@example.com",
							fullName: "User Twelve",
							password: "pass12345678",
						},
						expected: &expected{
							success:     true,
							message:     "User registered successfully",
							userCreated: true,
						},
					},
					{
						name: "Should register user with long username",
						input: &input{
							username: "verylongusername12345",
							email:    "longuser@example.com",
							fullName: "Long Username User",
							password: "password12345",
						},
						expected: &expected{
							success:     true,
							message:     "User registered successfully",
							userCreated: true,
						},
					},
					{
						name: "Should register user with complex email",
						input: &input{
							username: "complexemail",
							email:    "complex.email+tag@subdomain.example.com",
							fullName: "Complex Email User",
							password: "password12345",
						},
						expected: &expected{
							success:     true,
							message:     "User registered successfully",
							userCreated: true,
						},
					},

					// ===== Validation Tests: Username =====
					{
						name: "Should fail when username is empty",
						input: &input{
							username: "",
							email:    "test@example.com",
							fullName: "Test User",
							password: "password123",
						},
						expected: &expected{
							success:     false,
							message:     "Username is mandatory",
							userCreated: false,
						},
					},
					{
						name: "Should fail when username is too short (4 characters)",
						input: &input{
							username: "user",
							email:    "test@example.com",
							fullName: "Test User",
							password: "password123",
						},
						expected: &expected{
							success:     false,
							message:     "Username must be at least 5 characters long",
							userCreated: false,
						},
					},

					// ===== Validation Tests: Email =====
					{
						name: "Should fail when email is empty",
						input: &input{
							username: "testuser",
							email:    "",
							fullName: "Test User",
							password: "password123",
						},
						expected: &expected{
							success:     false,
							message:     "Email is mandatory",
							userCreated: false,
						},
					},
					{
						name: "Should fail when email is invalid (missing @)",
						input: &input{
							username: "testuser",
							email:    "invalidemail.com",
							fullName: "Test User",
							password: "password123",
						},
						expected: &expected{
							success:     false,
							message:     "Invalid email",
							userCreated: false,
						},
					},
					{
						name: "Should fail when email is invalid (missing domain)",
						input: &input{
							username: "testuser",
							email:    "test@",
							fullName: "Test User",
							password: "password123",
						},
						expected: &expected{
							success:     false,
							message:     "Invalid email",
							userCreated: false,
						},
					},
					{
						name: "Should fail when email is invalid (missing local part)",
						input: &input{
							username: "testuser",
							email:    "@example.com",
							fullName: "Test User",
							password: "password123",
						},
						expected: &expected{
							success:     false,
							message:     "Invalid email",
							userCreated: false,
						},
					},

					// ===== Validation Tests: Password =====
					{
						name: "Should fail when password is empty",
						input: &input{
							username: "testuser",
							email:    "test@example.com",
							fullName: "Test User",
							password: "",
						},
						expected: &expected{
							success:     false,
							message:     "Password is mandatory",
							userCreated: false,
						},
					},
					{
						name: "Should fail when password is too short (11 characters)",
						input: &input{
							username: "testuser",
							email:    "test@example.com",
							fullName: "Test User",
							password: "pass1234567",
						},
						expected: &expected{
							success:     false,
							message:     "Password must be at least 12 characters long",
							userCreated: false,
						},
					},

					// ===== Validation Tests: FullName =====
					{
						name: "Should fail when full name is empty",
						input: &input{
							username: "testuser",
							email:    "test@example.com",
							fullName: "",
							password: "password12345",
						},
						expected: &expected{
							success:     false,
							message:     "FullName is mandatory",
							userCreated: false,
						},
					},
				})
			})

		})
	})
}

func TestUserLogin(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Login", func() {

			// Pin "now" so the stamped last_login_at is deterministic.
			t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			oldLogin := t0.Add(-24 * time.Hour)
			suite.Clock = clock.Fixed(t0)

			var Users []DataUser

			type input struct {
				uid      int // fixture user id (0 = no user), for last_login_at checks
				username string
				password string
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

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:         0,
						Username:    "testuser1",
						Email:       "test1@example.com",
						FullName:    "Test User 1",
						Password:    "password123",
						Status:      user.UserStatusActive,
						LastLoginAt: &oldLogin,
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
						Status:   user.UserStatusActive,
					},
					{
						Idx:      2,
						Username: "disableduser",
						Email:    "disabled@example.com",
						FullName: "Disabled User",
						Password: "password123",
						Status:   user.UserStatusDisabled,
					},
				}

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password, userData.Status)
					Users[i].Id = insertedUser.Id // Store actual database ID
					if userData.LastLoginAt != nil {
						app.Helper.SetLastLoginAt(ctx, t, insertedUser.Id, *userData.LastLoginAt)
					}
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				// Snapshot so a failed login can be asserted unchanged.
				var before *time.Time
				if !r.expected.success {
					before = app.Helper.GetLastLoginAt(ctx, t, r.input.uid)
				}

				output := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-test",
					Username: r.input.username,
					Password: r.input.password,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				// Successful login stamps last_login_at with the mocked now.
				if r.expected.success {
					assert.Equal(t, &t0, output.User.LastLoginAt, r.name)
					assert.WithinDuration(t, t0, *app.Helper.GetLastLoginAt(ctx, t, r.input.uid), time.Second, r.name)
					return
				}

				// Failed login never changes last_login_at (NULL stays NULL,
				// an existing value stays as-is).
				stored := app.Helper.GetLastLoginAt(ctx, t, r.input.uid)
				if before == nil {
					assert.Nil(t, stored, r.name)
				} else {
					// DB round-trip returns the instant in the local zone, so
					// compare instants rather than wall-clock representation.
					assert.WithinDuration(t, *before, *stored, time.Second, r.name)
				}
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "Login scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should login successfully with valid credentials",
						input: &input{
							uid:      Users[0].Id,
							username: "testuser1",
							password: "password123",
						},
						expected: &expected{
							success: true,
							message: "Login successful",
						},
					},
					{
						name: "Should login successfully with different user",
						input: &input{
							uid:      Users[1].Id,
							username: "testuser2",
							password: "password123",
						},
						expected: &expected{
							success: true,
							message: "Login successful",
						},
					},

					// ===== Validation Tests: Username =====
					{
						name: "Should fail when username is empty",
						input: &input{
							username: "",
							password: "password123",
						},
						expected: &expected{
							success: false,
							message: "Username is mandatory",
						},
					},
					{
						name: "Should fail when username does not exist",
						input: &input{
							username: "nonexistentuser",
							password: "password123",
						},
						expected: &expected{
							success: false,
							message: "Invalid username or password",
						},
					},
					// ===== Validation Tests: Password =====
					{
						name: "Should fail when password is empty",
						input: &input{
							uid:      Users[0].Id,
							username: "testuser1",
							password: "",
						},
						expected: &expected{
							success: false,
							message: "Password is mandatory",
						},
					},
					{
						name: "Should fail when password is incorrect",
						input: &input{
							uid:      Users[0].Id,
							username: "testuser1",
							password: "wrongpassword",
						},
						expected: &expected{
							success: false,
							message: "Invalid username or password",
						},
					},
					{
						name: "Should fail when password is partially correct",
						input: &input{
							uid:      Users[0].Id,
							username: "testuser1",
							password: "password12",
						},
						expected: &expected{
							success: false,
							message: "Invalid username or password",
						},
					},
					{
						name: "Should fail with case-sensitive password",
						input: &input{
							uid:      Users[0].Id,
							username: "testuser1",
							password: "PASSWORD123",
						},
						expected: &expected{
							success: false,
							message: "Invalid username or password",
						},
					},
					{
						name: "Should fail when account is disabled",
						input: &input{
							uid:      Users[2].Id,
							username: "disableduser",
							password: "password123",
						},
						expected: &expected{
							success: false,
							message: "Account disabled",
						},
					},
				})
			})

		})
	})
}

func TestUserUpdateUsername(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User UpdateUsername", func() {

			var Users []DataUser

			type input struct {
				userID      int
				newUsername string
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

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = insertedUser.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.userID, input.UserID, r.name)
					assert.Equal(t, "user:profile_update", input.Permission, r.name)
					counter.Inc()
					return r.permissionCheck
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.UpdateUsername(ctx, &user.UpdateUsernameInput{
					TraceId:     "trace-test",
					Id:          r.input.userID,
					NewUsername: r.input.newUsername,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - permission check call count")

				if r.expected.success == false {
					// Verify no users were modified
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}

				// For successful update
				assert.Equal(t, len(initialUsers), len(afterUsers), r.name)
				assert.NotZero(t, output.User.Id, r.name)
				assert.Equal(t, r.input.newUsername, output.User.Username, r.name)
				assert.NotEmpty(t, output.User.Email, r.name)
				assert.NotEmpty(t, output.User.FullName, r.name)
				assert.NotZero(t, output.User.CreatedAt, r.name)
				assert.NotZero(t, output.User.UpdatedAt, r.name)

				// Verify the username was actually updated in afterUsers map
				updatedUser, exists := afterUsers[r.input.userID]
				assert.True(t, exists, r.name+" - user should exist in afterUsers")
				assert.Equal(t, r.input.newUsername, updatedUser.Username, r.name)

				// Update the initial users map to reflect the expected changes
				// (Username and UpdatedAt will change for the target user)
				if initialUser, exists := initialUsers[r.input.userID]; exists {
					initialUser.Username = r.input.newUsername
					initialUser.UpdatedAt = updatedUser.UpdatedAt
					initialUsers[r.input.userID] = initialUser
				}

				// Verify all other users remain unchanged
				assert.Equal(t, initialUsers, afterUsers, r.name+" - only the target user should be modified")
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "UpdateUsername scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should update username successfully",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "newusername1",
						},
						expected: &expected{
							success:           true,
							message:           "Username updated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should update username to minimum length (5 characters)",
						input: &input{
							userID:      Users[1].Id,
							newUsername: "user5",
						},
						expected: &expected{
							success:           true,
							message:           "Username updated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should update username to long name",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "verylongusername12345",
						},
						expected: &expected{
							success:           true,
							message:           "Username updated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests: NewUsername =====
					{
						name: "Should fail when new username is empty",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "",
						},
						expected: &expected{
							success:           false,
							message:           "New username is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when new username is too short (4 characters)",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "user",
						},
						expected: &expected{
							success:           false,
							message:           "Username must be at least 5 characters long",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when new username is too short (3 characters)",
						input: &input{
							userID:      Users[1].Id,
							newUsername: "abc",
						},
						expected: &expected{
							success:           false,
							message:           "Username must be at least 5 characters long",
							expectedCountMock: 0,
						},
					},

					// ===== Validation Tests: User ID =====
					{
						name: "Should fail when user does not exist",
						input: &input{
							userID:      99999,
							newUsername: "validusername",
						},
						expected: &expected{
							success:           false,
							message:           "No Username Found",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Permission Tests =====
					{
						name: "Should fail when user does not hold profile_update permission",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "validusername",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: you do not have permission to update profile",
							expectedCountMock: 1,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
					{
						name: "Should fail when permission module cannot confirm the check",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "validusername",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: you do not have permission to update profile",
							expectedCountMock: 1,
						},
						permissionCheck: createFailedPermissionCheck(),
					},
				})
			})

		})
	})
}

func TestUserUpdatePassword(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User UpdatePassword", func() {

			var Users []DataUser

			type input struct {
				userID      int
				oldPassword string
				newPassword string
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

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = insertedUser.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.userID, input.UserID, r.name)
					assert.Equal(t, "user:password_update", input.Permission, r.name)
					counter.Inc()
					return r.permissionCheck
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.UpdatePassword(ctx, &user.UpdatePasswordInput{
					TraceId:     "trace-test",
					Id:          r.input.userID,
					OldPassword: r.input.oldPassword,
					NewPassword: r.input.newPassword,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - permission check call count")

				if r.expected.success == false {
					// Verify no users were modified
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}

				// For successful update
				assert.Equal(t, len(initialUsers), len(afterUsers), r.name)

				// Verify the new password is hashed
				storedPassword := app.Helper.GetUserPassword(ctx, t, r.input.userID)
				assert.NotEqual(t, r.input.oldPassword, storedPassword, r.name+" - password should be hashed")
				assert.NotEqual(t, r.input.newPassword, storedPassword, r.name+" - password should be hashed")
				assert.NotEmpty(t, storedPassword, r.name)

				// Verify the user exists in afterUsers map
				updatedUser, exists := afterUsers[r.input.userID]
				assert.True(t, exists, r.name+" - user should exist in afterUsers")

				// Update the initial users map to reflect the expected changes
				// (Only UpdatedAt will change for the target user, password is not in User struct)
				if initialUser, exists := initialUsers[r.input.userID]; exists {
					initialUser.UpdatedAt = updatedUser.UpdatedAt
					initialUsers[r.input.userID] = initialUser
				}

				// Verify all other users remain unchanged
				assert.Equal(t, initialUsers, afterUsers, r.name+" - only the target user should be modified")
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "UpdatePassword scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should update password successfully",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "password123",
							newPassword: "newpassword123",
						},
						expected: &expected{
							success:           true,
							message:           "Password updated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should update password to minimum length (12 characters)",
						input: &input{
							userID:      Users[1].Id,
							oldPassword: "password123",
							newPassword: "pass12345678",
						},
						expected: &expected{
							success:           true,
							message:           "Password updated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should update password to long password",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "newpassword123",
							newPassword: "verylongpassword12345",
						},
						expected: &expected{
							success:           true,
							message:           "Password updated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests: OldPassword =====
					{
						name: "Should fail when old password is empty",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "",
							newPassword: "newpassword123",
						},
						expected: &expected{
							success:           false,
							message:           "Old password is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when old password is incorrect",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "wrongpassword",
							newPassword: "newpassword123",
						},
						expected: &expected{
							success:           false,
							message:           "Invalid old password",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests: NewPassword =====
					{
						name: "Should fail when new password is empty",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "verylongpassword12345",
							newPassword: "",
						},
						expected: &expected{
							success:           false,
							message:           "New password is mandatory",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when new password is too short (11 characters)",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "verylongpassword12345",
							newPassword: "pass1234567",
						},
						expected: &expected{
							success:           false,
							message:           "Password must be at least 12 characters long",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when new password is too short (3 characters)",
						input: &input{
							userID:      Users[1].Id,
							oldPassword: "pass123",
							newPassword: "abc",
						},
						expected: &expected{
							success:           false,
							message:           "Password must be at least 12 characters long",
							expectedCountMock: 0,
						},
					},
					{
						name: "Should fail when new password reuses recent password",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "verylongpassword12345",
							newPassword: "verylongpassword12345",
						},
						expected: &expected{
							success:           false,
							message:           "New password must be different from recent passwords",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},

					// ===== Validation Tests: User ID =====
					{
						name: "Should fail when user does not exist",
						input: &input{
							userID:      99999,
							oldPassword: "anypassword",
							newPassword: "newpassword123",
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
						name: "Should fail when user does not hold password_update permission",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "password123",
							newPassword: "newpassword123",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: you do not have permission to update password",
							expectedCountMock: 1,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
					{
						name: "Should fail when permission module cannot confirm the check",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "password123",
							newPassword: "newpassword123",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: you do not have permission to update password",
							expectedCountMock: 1,
						},
						permissionCheck: createFailedPermissionCheck(),
					},
				})
			})

		})
	})
}

func TestUserGetProfileById(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User GetProfileById", func() {

			var Users []DataUser

			type input struct {
				userID int
			}
			type expected struct {
				success  bool
				message  string
				userID   int
				username string
				email    string
				fullName string
			}

			type testRow struct {
				name     string
				input    *input
				expected *expected
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

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = insertedUser.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.GetProfileById(ctx, &user.GetProfileByIdInput{
					TraceId: "trace-test",
					Id:      r.input.userID,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no users were modified (read operation should not change anything)
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}

				// For successful retrieval
				assert.Equal(t, len(initialUsers), len(afterUsers), r.name)
				assert.NotZero(t, output.User.Id, r.name)
				assert.Equal(t, r.expected.userID, output.User.Id, r.name)
				assert.Equal(t, r.expected.username, output.User.Username, r.name)
				assert.Equal(t, r.expected.email, output.User.Email, r.name)
				assert.Equal(t, r.expected.fullName, output.User.FullName, r.name)
				assert.NotZero(t, output.User.CreatedAt, r.name)
				assert.NotZero(t, output.User.UpdatedAt, r.name)

				// Verify no users were modified (read operation should not change anything)
				assert.Equal(t, initialUsers, afterUsers, r.name)
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "GetProfileById scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should get profile successfully for first user",
						input: &input{
							userID: Users[0].Id,
						},
						expected: &expected{
							success:  true,
							message:  "Profile retrieved successfully",
							userID:   Users[0].Id,
							username: "testuser1",
							email:    "test1@example.com",
							fullName: "Test User 1",
						},
					},
					{
						name: "Should get profile successfully for second user",
						input: &input{
							userID: Users[1].Id,
						},
						expected: &expected{
							success:  true,
							message:  "Profile retrieved successfully",
							userID:   Users[1].Id,
							username: "testuser2",
							email:    "test2@example.com",
							fullName: "Test User 2",
						},
					},

					// ===== Validation Tests: User ID =====
					{
						name: "Should fail when user does not exist",
						input: &input{
							userID: 99999,
						},
						expected: &expected{
							success: false,
							message: "User not found",
						},
					},
					{
						name: "Should fail when user ID is zero",
						input: &input{
							userID: 0,
						},
						expected: &expected{
							success: false,
							message: "User not found",
						},
					},
					{
						name: "Should fail when user ID is negative",
						input: &input{
							userID: -1,
						},
						expected: &expected{
							success: false,
							message: "User not found",
						},
					},
				})
			})

		})
	})
}
func TestUserAssignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User GrantPermission", func() {

			var Users []DataUser

			type input struct {
				traceId      string
				actorId      int
				targetUserId int
				roleName     string
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
				grantCalls        int
			}

			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
				grantOutput     *permission.AssignRoleOutput
			}

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "actoruser",
						Email:    "actor@example.com",
						FullName: "Actor User",
						Password: "password123",
					},
					{
						Idx:      1,
						Username: "targetuser",
						Email:    "target@example.com",
						FullName: "Target User",
						Password: "password123",
					},
				}

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = insertedUser.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}
				grantCounter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					if r.permissionCheck != nil {
						return r.permissionCheck
					}
					return createFailedPermissionCheck()
				}

				app.PermissionSvcMock.AssignRoleStub = func(ctx context.Context, input *permission.AssignRoleInput) *permission.AssignRoleOutput {
					assert.Equal(t, r.input.targetUserId, input.UserID, r.name)
					assert.Equal(t, r.input.roleName, input.RoleName, r.name)
					grantCounter.Inc()
					if r.grantOutput != nil {
						return r.grantOutput
					}
					return createFailedAssign()
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.AssignRole(ctx, &user.AssignRoleInput{
					TraceId:      r.input.traceId,
					ActorId:      r.input.actorId,
					TargetUserId: r.input.targetUserId,
					RoleName:     r.input.roleName,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")
				assert.Equal(t, r.expected.grantCalls, grantCounter.Total(), r.name+" - grant call count")

				// GrantPermission never touches the users table directly
				assert.Equal(t, initialUsers, afterUsers, r.name+" - users table must not change")
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "AssignRole scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should grant permission when actor holds super user permission",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           true,
							message:           "Role assigned successfully",
							expectedCountMock: 1,
							grantCalls:        1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						grantOutput:     createSuccessfulAssign(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:      "",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "TraceId is mandatory",
							expectedCountMock: 0,
							grantCalls:        0,
						},
					},
					{
						name: "Should fail when actor ID is zero",
						input: &input{
							traceId:      "trace-test",
							actorId:      0,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
							grantCalls:        0,
						},
					},
					{
						name: "Should fail when target user ID is zero",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: 0,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Target user ID is mandatory",
							expectedCountMock: 0,
							grantCalls:        0,
						},
					},
					{
						name: "Should fail when permission is empty",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "",
						},
						expected: &expected{
							success:           false,
							message:           "Role name is mandatory",
							expectedCountMock: 0,
							grantCalls:        0,
						},
					},

					// ===== Permission Tests =====
					{
						name: "Should fail when actor does not hold super user permission",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can grant permissions",
							expectedCountMock: 1,
							grantCalls:        0,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
					{
						name: "Should fail when permission module cannot confirm super user check",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can grant permissions",
							expectedCountMock: 1,
							grantCalls:        0,
						},
						permissionCheck: createFailedPermissionCheck(),
					},
					{
						name: "Should forward message when permission module fails to grant",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Failed to assign role",
							expectedCountMock: 1,
							grantCalls:        1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						grantOutput:     createFailedAssign(),
					},
				})
			})

		})
	})
}
func TestUserUnassignRole(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User RevokePermission", func() {

			var Users []DataUser

			type input struct {
				traceId      string
				actorId      int
				targetUserId int
				roleName     string
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
				revokeCalls       int
			}

			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
				revokeOutput    *permission.UnassignRoleOutput
			}

			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "actoruser",
						Email:    "actor@example.com",
						FullName: "Actor User",
						Password: "password123",
					},
					{
						Idx:      1,
						Username: "targetuser",
						Email:    "target@example.com",
						FullName: "Target User",
						Password: "password123",
					},
				}

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = insertedUser.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}
				revokeCounter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					if r.permissionCheck != nil {
						return r.permissionCheck
					}
					return createFailedPermissionCheck()
				}

				app.PermissionSvcMock.UnassignRoleStub = func(ctx context.Context, input *permission.UnassignRoleInput) *permission.UnassignRoleOutput {
					assert.Equal(t, r.input.targetUserId, input.UserID, r.name)
					assert.Equal(t, r.input.roleName, input.RoleName, r.name)
					revokeCounter.Inc()
					if r.revokeOutput != nil {
						return r.revokeOutput
					}
					return createFailedUnassign()
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.UnassignRole(ctx, &user.UnassignRoleInput{
					TraceId:      r.input.traceId,
					ActorId:      r.input.actorId,
					TargetUserId: r.input.targetUserId,
					RoleName:     r.input.roleName,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")
				assert.Equal(t, r.expected.revokeCalls, revokeCounter.Total(), r.name+" - revoke call count")

				// RevokePermission never touches the users table directly
				assert.Equal(t, initialUsers, afterUsers, r.name+" - users table must not change")
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "UnassignRole scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should revoke permission when actor holds super user permission",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           true,
							message:           "Role unassigned successfully",
							expectedCountMock: 1,
							revokeCalls:       1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						revokeOutput:    createSuccessfulUnassign(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:      "",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "TraceId is mandatory",
							expectedCountMock: 0,
							revokeCalls:       0,
						},
					},
					{
						name: "Should fail when actor ID is zero",
						input: &input{
							traceId:      "trace-test",
							actorId:      0,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Actor ID is mandatory",
							expectedCountMock: 0,
							revokeCalls:       0,
						},
					},
					{
						name: "Should fail when target user ID is zero",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: 0,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Target user ID is mandatory",
							expectedCountMock: 0,
							revokeCalls:       0,
						},
					},
					{
						name: "Should fail when permission is empty",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "",
						},
						expected: &expected{
							success:           false,
							message:           "Role name is mandatory",
							expectedCountMock: 0,
							revokeCalls:       0,
						},
					},

					// ===== Permission Tests =====
					{
						name: "Should fail when actor does not hold super user permission",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can revoke permissions",
							expectedCountMock: 1,
							revokeCalls:       0,
						},
						permissionCheck: createDeniedPermissionCheck(),
					},
					{
						name: "Should fail when permission module cannot confirm super user check",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can revoke permissions",
							expectedCountMock: 1,
							revokeCalls:       0,
						},
						permissionCheck: createFailedPermissionCheck(),
					},
					{
						name: "Should forward message when permission module fails to revoke",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							roleName:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Failed to unassign role",
							expectedCountMock: 1,
							revokeCalls:       1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						revokeOutput:    createFailedUnassign(),
					},
				})
			})

		})
	})
}

func TestUserManageRolePermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User ManageRolePermission", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				op       string // AuditActionGrant or AuditActionRevoke
				traceId  string
				actorId  int
				roleName string
				perm     string
			}
			type expected struct {
				success           bool
				message           string
				expectedCountMock int
				downstreamCalls   int
			}
			type testRow struct {
				name            string
				input           *input
				expected        *expected
				permissionCheck *permission.CheckPermissionOutput
				downstreamOK    bool // permission-module call succeeds
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "actoruser",
						Email:    "actor@example.com",
						FullName: "Actor User",
						Password: "password123",
					},
				}

				for i, userData := range Users {
					inserted := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = inserted.Id
				}
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				counter := &testsuite.Counter{}
				downstreamCounter := &testsuite.Counter{}

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					assert.Equal(t, r.input.actorId, input.UserID, r.name)
					assert.Equal(t, "super_user", input.Permission, r.name)
					counter.Inc()
					if r.permissionCheck != nil {
						return r.permissionCheck
					}
					return createFailedPermissionCheck()
				}
				app.PermissionSvcMock.AssignPermissionToRoleStub = func(ctx context.Context, input *permission.AssignPermissionToRoleInput) *permission.AssignPermissionToRoleOutput {
					assert.Equal(t, r.input.roleName, input.RoleName, r.name)
					assert.Equal(t, r.input.perm, input.Permission, r.name)
					assert.Equal(t, r.input.actorId, input.ActorId, r.name)
					downstreamCounter.Inc()
					if r.downstreamOK {
						return &permission.AssignPermissionToRoleOutput{Success: true, Message: "Permission assigned to role successfully"}
					}
					return &permission.AssignPermissionToRoleOutput{Success: false, Message: "Unknown permission"}
				}
				app.PermissionSvcMock.RemovePermissionFromRoleStub = func(ctx context.Context, input *permission.RemovePermissionFromRoleInput) *permission.RemovePermissionFromRoleOutput {
					assert.Equal(t, r.input.roleName, input.RoleName, r.name)
					assert.Equal(t, r.input.perm, input.Permission, r.name)
					assert.Equal(t, r.input.actorId, input.ActorId, r.name)
					downstreamCounter.Inc()
					if r.downstreamOK {
						return &permission.RemovePermissionFromRoleOutput{Success: true, Message: "Permission removed from role successfully"}
					}
					return &permission.RemovePermissionFromRoleOutput{Success: false, Message: "Unknown permission"}
				}

				var success bool
				var message string
				switch r.input.op {
				case permission.AuditActionGrant:
					out := app.Service.AssignPermissionToRole(ctx, &user.AssignPermissionToRoleInput{
						TraceId:    r.input.traceId,
						ActorId:    r.input.actorId,
						RoleName:   r.input.roleName,
						Permission: r.input.perm,
					})
					success, message = out.Success, out.Message
				case permission.AuditActionRevoke:
					out := app.Service.RemovePermissionFromRole(ctx, &user.RemovePermissionFromRoleInput{
						TraceId:    r.input.traceId,
						ActorId:    r.input.actorId,
						RoleName:   r.input.roleName,
						Permission: r.input.perm,
					})
					success, message = out.Success, out.Message
				}

				assert.Equal(t, r.expected.success, success, r.name)
				assert.Equal(t, r.expected.message, message, r.name)
				assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - super user check call count")
				assert.Equal(t, r.expected.downstreamCalls, downstreamCounter.Total(), r.name+" - downstream call count")
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "ManageRolePermission scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should grant a permission to a role when actor holds super user",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							actorId:  Users[0].Id,
							roleName: permission.RoleMember,
							perm:     "user:profile_update",
						},
						expected: &expected{
							success:           true,
							message:           "Permission assigned to role successfully",
							expectedCountMock: 1,
							downstreamCalls:   1,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
						downstreamOK:    true,
					},
					{
						name: "Should revoke a permission from a role when actor holds super user",
						input: &input{
							op:       permission.AuditActionRevoke,
							traceId:  "trace-test",
							actorId:  Users[0].Id,
							roleName: permission.RoleMember,
							perm:     "user:profile_update",
						},
						expected: &expected{
							success:           true,
							message:           "Permission removed from role successfully",
							expectedCountMock: 1,
							downstreamCalls:   1,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
						downstreamOK:    true,
					},

					// ===== Authorization Tests =====
					{
						name: "Should fail when actor does not hold super user (grant)",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							actorId:  Users[0].Id,
							roleName: permission.RoleMember,
							perm:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Unauthorized: only super user can manage role permissions",
							expectedCountMock: 1,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: false},
					},

					// ===== Downstream Failure Tests =====
					{
						name: "Should surface unknown-permission rejection from the permission module",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							actorId:  Users[0].Id,
							roleName: permission.RoleMember,
							perm:     "garbage:permission",
						},
						expected: &expected{
							success:           false,
							message:           "Unknown permission",
							expectedCountMock: 1,
							downstreamCalls:   1,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
						downstreamOK:    false,
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "",
							actorId:  Users[0].Id,
							roleName: permission.RoleMember,
							perm:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "TraceId is mandatory",
							expectedCountMock: 0,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should fail when RoleName is empty",
						input: &input{
							op:       permission.AuditActionGrant,
							traceId:  "trace-test",
							actorId:  Users[0].Id,
							roleName: "",
							perm:     "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Role name is mandatory",
							expectedCountMock: 0,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
					{
						name: "Should fail when Permission is empty",
						input: &input{
							op:       permission.AuditActionRevoke,
							traceId:  "trace-test",
							actorId:  Users[0].Id,
							roleName: permission.RoleMember,
							perm:     "",
						},
						expected: &expected{
							success:           false,
							message:           "Permission is mandatory",
							expectedCountMock: 0,
						},
						permissionCheck: &permission.CheckPermissionOutput{Success: true, HasPermission: true},
					},
				})
			})
		})
	})
}
