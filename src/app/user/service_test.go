package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
							password: "password123",
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
							password: "password123",
						},
						expected: &expected{
							success:     true,
							message:     "User registered successfully",
							userCreated: true,
						},
					},
					{
						name: "Should register user with minimum password length (7 characters)",
						input: &input{
							username: "user7char",
							email:    "user7@example.com",
							fullName: "User Seven",
							password: "pass123",
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
							password: "password123",
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
							password: "password123",
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
						name: "Should fail when password is too short (6 characters)",
						input: &input{
							username: "testuser",
							email:    "test@example.com",
							fullName: "Test User",
							password: "pass12",
						},
						expected: &expected{
							success:     false,
							message:     "Password must be at least 7 characters long",
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
							password: "password123",
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

			var Users []DataUser

			type input struct {
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

				output := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-test",
					Username: r.input.username,
					Password: r.input.password,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

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
							username: "testuser1",
							password: "PASSWORD123",
						},
						expected: &expected{
							success: false,
							message: "Invalid username or password",
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
							newPassword: "newpass123",
						},
						expected: &expected{
							success:           true,
							message:           "Password updated successfully",
							expectedCountMock: 1,
						},
						permissionCheck: createGrantedPermissionCheck(),
					},
					{
						name: "Should update password to minimum length (7 characters)",
						input: &input{
							userID:      Users[1].Id,
							oldPassword: "password123",
							newPassword: "pass123",
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
							oldPassword: "newpass123",
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
							newPassword: "newpass123",
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
							newPassword: "newpass123",
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
						name: "Should fail when new password is too short (6 characters)",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "verylongpassword12345",
							newPassword: "pass12",
						},
						expected: &expected{
							success:           false,
							message:           "Password must be at least 7 characters long",
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
							message:           "Password must be at least 7 characters long",
							expectedCountMock: 0,
						},
					},

					// ===== Validation Tests: User ID =====
					{
						name: "Should fail when user does not exist",
						input: &input{
							userID:      99999,
							oldPassword: "anypassword",
							newPassword: "newpass123",
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
							newPassword: "newpass123",
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
							newPassword: "newpass123",
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

func TestUserGrantPermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User GrantPermission", func() {

			var Users []DataUser

			type input struct {
				traceId      string
				actorId      int
				targetUserId int
				permission   string
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
				grantOutput     *permission.GrantPermissionOutput
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

				app.PermissionSvcMock.GrantPermissionStub = func(ctx context.Context, input *permission.GrantPermissionInput) *permission.GrantPermissionOutput {
					assert.Equal(t, r.input.targetUserId, input.UserID, r.name)
					assert.Equal(t, r.input.permission, input.Permission, r.name)
					grantCounter.Inc()
					if r.grantOutput != nil {
						return r.grantOutput
					}
					return createFailedGrant()
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.GrantPermission(ctx, &user.GrantPermissionInput{
					TraceId:      r.input.traceId,
					ActorId:      r.input.actorId,
					TargetUserId: r.input.targetUserId,
					Permission:   r.input.permission,
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

			suite.Run(t, "GrantPermission scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should grant permission when actor holds super user permission",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							permission:   "user:profile_update",
						},
						expected: &expected{
							success:           true,
							message:           "Permission granted successfully",
							expectedCountMock: 1,
							grantCalls:        1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						grantOutput:     createSuccessfulGrant(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:      "",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
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
							permission:   "",
						},
						expected: &expected{
							success:           false,
							message:           "Permission is mandatory",
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
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Failed to grant permission",
							expectedCountMock: 1,
							grantCalls:        1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						grantOutput:     createFailedGrant(),
					},
				})
			})

		})
	})
}

func TestUserRevokePermission(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User RevokePermission", func() {

			var Users []DataUser

			type input struct {
				traceId      string
				actorId      int
				targetUserId int
				permission   string
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
				revokeOutput    *permission.RevokePermissionOutput
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

				app.PermissionSvcMock.RevokePermissionStub = func(ctx context.Context, input *permission.RevokePermissionInput) *permission.RevokePermissionOutput {
					assert.Equal(t, r.input.targetUserId, input.UserID, r.name)
					assert.Equal(t, r.input.permission, input.Permission, r.name)
					revokeCounter.Inc()
					if r.revokeOutput != nil {
						return r.revokeOutput
					}
					return createFailedRevoke()
				}

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.RevokePermission(ctx, &user.RevokePermissionInput{
					TraceId:      r.input.traceId,
					ActorId:      r.input.actorId,
					TargetUserId: r.input.targetUserId,
					Permission:   r.input.permission,
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

			suite.Run(t, "RevokePermission scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should revoke permission when actor holds super user permission",
						input: &input{
							traceId:      "trace-test",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							permission:   "user:profile_update",
						},
						expected: &expected{
							success:           true,
							message:           "Permission revoked successfully",
							expectedCountMock: 1,
							revokeCalls:       1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						revokeOutput:    createSuccessfulRevoke(),
					},

					// ===== Validation Tests =====
					{
						name: "Should fail when TraceId is empty",
						input: &input{
							traceId:      "",
							actorId:      Users[0].Id,
							targetUserId: Users[1].Id,
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
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
							permission:   "",
						},
						expected: &expected{
							success:           false,
							message:           "Permission is mandatory",
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
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
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
							permission:   "user:profile_update",
						},
						expected: &expected{
							success:           false,
							message:           "Failed to revoke permission",
							expectedCountMock: 1,
							revokeCalls:       1,
						},
						permissionCheck: createGrantedPermissionCheck(),
						revokeOutput:    createFailedRevoke(),
					},
				})
			})

		})
	})
}

func TestUserAuditLogWrites(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User Audit Log Writes", func() {

			// insertUsers creates the two fixture users in the scenario's own
			// schema and returns their ids. Each suite.Run runs in its own
			// parallel schema, so the fixtures live per scenario — never shared.
			insertUsers := func(ctx context.Context, app *UserApp) (actorID, targetID int) {
				actor := app.Helper.InsertUserWithHashedPassword(ctx, t, "actoruser", "actor@example.com", "Actor User", "password123")
				target := app.Helper.InsertUserWithHashedPassword(ctx, t, "targetuser", "target@example.com", "Target User", "password123")
				return actor.Id, target.Id
			}

			suite.Run(t, "Login records an audit entry", func(t *testing.T, ctx context.Context, app *UserApp) {
				actorID, _ := insertUsers(ctx, app)
				output := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-login",
					Username: "actoruser",
					Password: "password123",
				})
				assert.True(t, output.Success)

				entries := app.Helper.GetAuditLogs(ctx, t)
				require.Len(t, entries, 1)
				assert.Equal(t, user.AuditEventLogin, entries[0].Event)
				assert.NotNil(t, entries[0].ActorId)
				assert.Equal(t, actorID, *entries[0].ActorId)
				assert.NotNil(t, entries[0].TargetUserId)
				assert.Equal(t, actorID, *entries[0].TargetUserId)
			})

			suite.Run(t, "Login failure records no audit entry", func(t *testing.T, ctx context.Context, app *UserApp) {
				insertUsers(ctx, app)
				output := app.Service.Login(ctx, &user.LoginInput{
					TraceId:  "trace-login-fail",
					Username: "actoruser",
					Password: "wrongpassword",
				})
				assert.False(t, output.Success)

				entries := app.Helper.GetAuditLogs(ctx, t)
				assert.Empty(t, entries)
			})

			suite.Run(t, "Username update records an audit entry with old and new values", func(t *testing.T, ctx context.Context, app *UserApp) {
				actorID, _ := insertUsers(ctx, app)
				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					return createGrantedPermissionCheck()
				}

				output := app.Service.UpdateUsername(ctx, &user.UpdateUsernameInput{
					TraceId:     "trace-username",
					Id:          actorID,
					NewUsername: "renameduser",
				})
				assert.True(t, output.Success)

				entries := app.Helper.GetAuditLogs(ctx, t)
				require.Len(t, entries, 1)
				assert.Equal(t, user.AuditEventUsername, entries[0].Event)
				assert.Equal(t, actorID, *entries[0].ActorId)
				assert.Equal(t, "actoruser", entries[0].Metadata["old_username"])
				assert.Equal(t, "renameduser", entries[0].Metadata["new_username"])
			})

			suite.Run(t, "Password update records an audit entry without the hash", func(t *testing.T, ctx context.Context, app *UserApp) {
				actorID, _ := insertUsers(ctx, app)
				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					return createGrantedPermissionCheck()
				}

				output := app.Service.UpdatePassword(ctx, &user.UpdatePasswordInput{
					TraceId:     "trace-password",
					Id:          actorID,
					OldPassword: "password123",
					NewPassword: "newpassword123",
				})
				assert.True(t, output.Success)

				entries := app.Helper.GetAuditLogs(ctx, t)
				require.Len(t, entries, 1)
				assert.Equal(t, user.AuditEventPassword, entries[0].Event)
				assert.Equal(t, actorID, *entries[0].ActorId)
				assert.Empty(t, entries[0].Metadata)
			})

			suite.Run(t, "Grant permission records an audit entry with the permission string", func(t *testing.T, ctx context.Context, app *UserApp) {
				actorID, targetID := insertUsers(ctx, app)
				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					return createGrantedPermissionCheck()
				}
				app.PermissionSvcMock.GrantPermissionStub = func(ctx context.Context, input *permission.GrantPermissionInput) *permission.GrantPermissionOutput {
					return createSuccessfulGrant()
				}

				output := app.Service.GrantPermission(ctx, &user.GrantPermissionInput{
					TraceId:      "trace-grant",
					ActorId:      actorID,
					TargetUserId: targetID,
					Permission:   "user:profile_update",
				})
				assert.True(t, output.Success)

				entries := app.Helper.GetAuditLogs(ctx, t)
				require.Len(t, entries, 1)
				assert.Equal(t, user.AuditEventGrant, entries[0].Event)
				assert.Equal(t, actorID, *entries[0].ActorId)
				assert.Equal(t, targetID, *entries[0].TargetUserId)
				assert.Equal(t, "user:profile_update", entries[0].Metadata["permission"])
			})

			suite.Run(t, "Failed grant records no audit entry", func(t *testing.T, ctx context.Context, app *UserApp) {
				actorID, targetID := insertUsers(ctx, app)
				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					return createGrantedPermissionCheck()
				}
				app.PermissionSvcMock.GrantPermissionStub = func(ctx context.Context, input *permission.GrantPermissionInput) *permission.GrantPermissionOutput {
					return createFailedGrant()
				}

				output := app.Service.GrantPermission(ctx, &user.GrantPermissionInput{
					TraceId:      "trace-grant-fail",
					ActorId:      actorID,
					TargetUserId: targetID,
					Permission:   "user:profile_update",
				})
				assert.False(t, output.Success)

				entries := app.Helper.GetAuditLogs(ctx, t)
				assert.Empty(t, entries)
			})

			suite.Run(t, "Revoke permission records an audit entry with the permission string", func(t *testing.T, ctx context.Context, app *UserApp) {
				actorID, targetID := insertUsers(ctx, app)
				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					return createGrantedPermissionCheck()
				}
				app.PermissionSvcMock.RevokePermissionStub = func(ctx context.Context, input *permission.RevokePermissionInput) *permission.RevokePermissionOutput {
					return createSuccessfulRevoke()
				}

				output := app.Service.RevokePermission(ctx, &user.RevokePermissionInput{
					TraceId:      "trace-revoke",
					ActorId:      actorID,
					TargetUserId: targetID,
					Permission:   "user:profile_update",
				})
				assert.True(t, output.Success)

				entries := app.Helper.GetAuditLogs(ctx, t)
				require.Len(t, entries, 1)
				assert.Equal(t, user.AuditEventRevoke, entries[0].Event)
				assert.Equal(t, actorID, *entries[0].ActorId)
				assert.Equal(t, targetID, *entries[0].TargetUserId)
				assert.Equal(t, "user:profile_update", entries[0].Metadata["permission"])
			})
		})
	})
}

func TestUserListAuditLogs(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User ListAuditLogs", func() {

			// setupAdminLog seeds one login by Target and one grant by Admin to
			// Target in the scenario's own schema, then returns the two ids.
			setupAdminLog := func(ctx context.Context, app *UserApp) (adminID, targetID int) {
				admin := app.Helper.InsertUserWithHashedPassword(ctx, t, "adminuser", "admin@example.com", "Admin User", "password123")
				target := app.Helper.InsertUserWithHashedPassword(ctx, t, "targetuser", "target@example.com", "Target User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()
				_, err = tx.InsertAuditLog(ctx, user.AuditEntry{
					Event:        user.AuditEventLogin,
					ActorId:      intPtrForTest(target.Id),
					TargetUserId: intPtrForTest(target.Id),
					Metadata:     map[string]any{},
				})
				assert.Nil(t, err)
				_, err = tx.InsertAuditLog(ctx, user.AuditEntry{
					Event:        user.AuditEventGrant,
					ActorId:      intPtrForTest(admin.Id),
					TargetUserId: intPtrForTest(target.Id),
					Metadata:     map[string]any{"permission": "user:profile_update"},
				})
				assert.Nil(t, err)
				err = tx.Commit()
				assert.Nil(t, err)
				return admin.Id, target.Id
			}

			runtest := func(t *testing.T, app *UserApp, r struct {
				name            string
				actorId         int
				permissionCheck *permission.CheckPermissionOutput
				page            int
				pageSize        int
				event           string
				filterActorId   int
				filterTargetId  int
				expected        struct {
					success    bool
					message    string
					entryCount int
					total      int
					pageSize   int
				}
			}) {
				ctx := context.Background()

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					if r.permissionCheck != nil {
						return r.permissionCheck
					}
					return createFailedPermissionCheck()
				}

				output := app.Service.ListAuditLogs(ctx, &user.ListAuditLogsInput{
					TraceId:        "trace-list",
					ActorId:        r.actorId,
					Event:          r.event,
					FilterActorId:  r.filterActorId,
					FilterTargetId: r.filterTargetId,
					Page:           r.page,
					PageSize:       r.pageSize,
				})

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)
				if r.expected.success {
					assert.Len(t, output.Entries, r.expected.entryCount, r.name)
					assert.Equal(t, r.expected.total, output.Total, r.name)
					assert.Equal(t, r.expected.pageSize, output.PageSize, r.name)
				}
			}

			suite.Run(t, "ListAuditLogs scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				adminID, targetID := setupAdminLog(ctx, app)

				rows := []struct {
					name            string
					actorId         int
					permissionCheck *permission.CheckPermissionOutput
					page            int
					pageSize        int
					event           string
					filterActorId   int
					filterTargetId  int
					expected        struct {
						success    bool
						message    string
						entryCount int
						total      int
						pageSize   int
					}
				}{
					{
						name:            "Should fail when actor does not hold super user permission",
						actorId:         targetID,
						permissionCheck: createDeniedPermissionCheck(),
						page:            1,
						pageSize:        20,
						expected: struct {
							success    bool
							message    string
							entryCount int
							total      int
							pageSize   int
						}{
							success: false,
							message: "Unauthorized: only super user can read audit logs",
						},
					},
					{
						name:            "Should list newest first for a super user",
						actorId:         adminID,
						permissionCheck: createGrantedPermissionCheck(),
						page:            1,
						pageSize:        20,
						expected: struct {
							success    bool
							message    string
							entryCount int
							total      int
							pageSize   int
						}{
							success:    true,
							message:    "Audit logs retrieved successfully",
							entryCount: 2,
							total:      2,
							pageSize:   20,
						},
					},
					{
						name:            "Should filter by event",
						actorId:         adminID,
						permissionCheck: createGrantedPermissionCheck(),
						page:            1,
						pageSize:        20,
						event:           user.AuditEventLogin,
						expected: struct {
							success    bool
							message    string
							entryCount int
							total      int
							pageSize   int
						}{
							success:    true,
							message:    "Audit logs retrieved successfully",
							entryCount: 1,
							total:      1,
							pageSize:   20,
						},
					},
					{
						name:            "Should filter by target user",
						actorId:         adminID,
						permissionCheck: createGrantedPermissionCheck(),
						page:            1,
						pageSize:        20,
						filterTargetId:  targetID,
						expected: struct {
							success    bool
							message    string
							entryCount int
							total      int
							pageSize   int
						}{
							success:    true,
							message:    "Audit logs retrieved successfully",
							entryCount: 2,
							total:      2,
							pageSize:   20,
						},
					},
					{
						name:            "Should paginate",
						actorId:         adminID,
						permissionCheck: createGrantedPermissionCheck(),
						page:            1,
						pageSize:        1,
						expected: struct {
							success    bool
							message    string
							entryCount int
							total      int
							pageSize   int
						}{
							success:    true,
							message:    "Audit logs retrieved successfully",
							entryCount: 1,
							total:      2,
							pageSize:   1,
						},
					},
				}

				for _, r := range rows {
					runtest(t, app, r)
				}
			})

			suite.Run(t, "ListAuditLogs validation scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				adminID, _ := setupAdminLog(ctx, app)

				output := app.Service.ListAuditLogs(ctx, &user.ListAuditLogsInput{
					TraceId:  "trace-list",
					ActorId:  adminID,
					Page:     0,
					PageSize: 20,
				})
				assert.False(t, output.Success)
				assert.Equal(t, "Page must be at least 1", output.Message)

				output = app.Service.ListAuditLogs(ctx, &user.ListAuditLogsInput{
					TraceId:  "trace-list",
					ActorId:  adminID,
					Page:     1,
					PageSize: 101,
				})
				assert.False(t, output.Success)
				assert.Equal(t, "Page size must be between 1 and 100", output.Message)

				output = app.Service.ListAuditLogs(ctx, &user.ListAuditLogsInput{
					TraceId:  "trace-list",
					ActorId:  0,
					Page:     1,
					PageSize: 20,
				})
				assert.False(t, output.Success)
				assert.Equal(t, "Actor ID is mandatory", output.Message)
			})
		})
	})
}
