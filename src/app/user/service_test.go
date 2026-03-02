package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/user"
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

				initialUsers := app.Helper.GetAllUsers(ctx, t)

				output := app.Service.UpdateUsername(ctx, &user.UpdateUsernameInput{
					TraceId:     "trace-test",
					Id:          r.input.userID,
					NewUsername: r.input.newUsername,
				})

				afterUsers := app.Helper.GetAllUsers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

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
							success: true,
							message: "Username updated successfully",
						},
					},
					{
						name: "Should update username to minimum length (5 characters)",
						input: &input{
							userID:      Users[1].Id,
							newUsername: "user5",
						},
						expected: &expected{
							success: true,
							message: "Username updated successfully",
						},
					},
					{
						name: "Should update username to long name",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "verylongusername12345",
						},
						expected: &expected{
							success: true,
							message: "Username updated successfully",
						},
					},

					// ===== Validation Tests: NewUsername =====
					{
						name: "Should fail when new username is empty",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "",
						},
						expected: &expected{
							success: false,
							message: "New username is mandatory",
						},
					},
					{
						name: "Should fail when new username is too short (4 characters)",
						input: &input{
							userID:      Users[0].Id,
							newUsername: "user",
						},
						expected: &expected{
							success: false,
							message: "Username must be at least 5 characters long",
						},
					},
					{
						name: "Should fail when new username is too short (3 characters)",
						input: &input{
							userID:      Users[1].Id,
							newUsername: "abc",
						},
						expected: &expected{
							success: false,
							message: "Username must be at least 5 characters long",
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
							success: false,
							message: "No Username Found",
						},
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
							success: true,
							message: "Password updated successfully",
						},
					},
					{
						name: "Should update password to minimum length (7 characters)",
						input: &input{
							userID:      Users[1].Id,
							oldPassword: "password123",
							newPassword: "pass123",
						},
						expected: &expected{
							success: true,
							message: "Password updated successfully",
						},
					},
					{
						name: "Should update password to long password",
						input: &input{
							userID:      Users[0].Id,
							oldPassword: "newpass123",
							newPassword: "verylongpassword12345",
						},
						expected: &expected{
							success: true,
							message: "Password updated successfully",
						},
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
							success: false,
							message: "Old password is mandatory",
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
							success: false,
							message: "Invalid old password",
						},
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
							success: false,
							message: "New password is mandatory",
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
							success: false,
							message: "Password must be at least 7 characters long",
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
							success: false,
							message: "Password must be at least 7 characters long",
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
							success: false,
							message: "User not found",
						},
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

func TestUserAddMember(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User AddMember", func() {

			var Users []DataUser

			type input struct {
				userID        int
				name          string
				monthlyIncome int
			}
			type expected struct {
				success       bool
				message       string
				memberCreated bool
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

				initialMembers := app.Helper.GetAllMembers(ctx, t)

				output := app.Service.AddMember(ctx, &user.AddMemberInput{
					TraceId:       "trace-test",
					Id:            r.input.userID,
					Name:          r.input.name,
					MonthlyIncome: r.input.monthlyIncome,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no members were created
					assert.Equal(t, initialMembers, afterMembers, r.name)
					return
				}

				// For successful member addition
				assert.Equal(t, len(initialMembers)+1, len(afterMembers), r.name)

				// The output now contains only the created member
				foundMember := output.Member
				assert.NotZero(t, foundMember.Id, r.name)
				assert.Equal(t, r.input.userID, foundMember.UserId, r.name)
				assert.Equal(t, r.input.name, foundMember.Name, r.name)
				assert.Equal(t, r.input.monthlyIncome, foundMember.MonthlyIncome, r.name)
				assert.NotZero(t, foundMember.CreatedAt, r.name)
				assert.NotZero(t, foundMember.UpdatedAt, r.name)

				// Verify the newly added member exists in afterMembers map
				addedMember, exists := afterMembers[foundMember.Id]
				assert.True(t, exists, r.name+" - member should exist in afterMembers")
				assert.Equal(t, r.input.name, addedMember.Name, r.name)

				// Add the new member to initial members map to reflect expected changes
				initialMembers[foundMember.Id] = addedMember

				// Verify all other members remain unchanged
				assert.Equal(t, initialMembers, afterMembers, r.name+" - only the new member should be added")
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "AddMember scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should add member successfully with valid inputs",
						input: &input{
							userID:        Users[0].Id,
							name:          "John Doe",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success:       true,
							message:       "Member added successfully",
							memberCreated: true,
						},
					},
					{
						name: "Should add member with minimum name length (2 characters)",
						input: &input{
							userID:        Users[0].Id,
							name:          "Jo",
							monthlyIncome: 3000000,
						},
						expected: &expected{
							success:       true,
							message:       "Member added successfully",
							memberCreated: true,
						},
					},
					{
						name: "Should add member with zero monthly income",
						input: &input{
							userID:        Users[0].Id,
							name:          "Jane Doe",
							monthlyIncome: 0,
						},
						expected: &expected{
							success:       true,
							message:       "Member added successfully",
							memberCreated: true,
						},
					},
					{
						name: "Should add member with long name",
						input: &input{
							userID:        Users[1].Id,
							name:          "Very Long Member Name For Testing",
							monthlyIncome: 10000000,
						},
						expected: &expected{
							success:       true,
							message:       "Member added successfully",
							memberCreated: true,
						},
					},
					{
						name: "Should add multiple members to same user",
						input: &input{
							userID:        Users[1].Id,
							name:          "Second Member",
							monthlyIncome: 7500000,
						},
						expected: &expected{
							success:       true,
							message:       "Member added successfully",
							memberCreated: true,
						},
					},

					// ===== Validation Tests: Name =====
					{
						name: "Should fail when name is empty",
						input: &input{
							userID:        Users[0].Id,
							name:          "",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success:       false,
							message:       "Member name is mandatory",
							memberCreated: false,
						},
					},
					{
						name: "Should fail when name is too short (1 character)",
						input: &input{
							userID:        Users[0].Id,
							name:          "A",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success:       false,
							message:       "Member name must be at least 2 characters long",
							memberCreated: false,
						},
					},

					// ===== Validation Tests: User ID =====
					{
						name: "Should fail when user does not exist",
						input: &input{
							userID:        99999,
							name:          "John Doe",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success:       false,
							message:       "User not found",
							memberCreated: false,
						},
					},
				})
			})

		})
	})
}

func TestUserGetMemberById(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User GetMemberById", func() {

			var Users []DataUser
			var Members []DataMember

			type input struct {
				memberID int
			}
			type expected struct {
				success       bool
				message       string
				memberID      int
				userID        int
				name          string
				monthlyIncome int
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

				Members = []DataMember{
					{
						Idx:           0,
						UserId:        Users[0].Id,
						Name:          "John Doe",
						MonthlyIncome: 5000000,
					},
					{
						Idx:           1,
						UserId:        Users[0].Id,
						Name:          "Jane Doe",
						MonthlyIncome: 3000000,
					},
					{
						Idx:           2,
						UserId:        Users[1].Id,
						Name:          "Bob Smith",
						MonthlyIncome: 7000000,
					},
				}

				// Insert members and store actual database IDs
				for i, memberData := range Members {
					insertedMember := app.Helper.InsertMember(ctx, t, memberData.UserId, memberData.Name, memberData.MonthlyIncome)
					Members[i].Id = insertedMember.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				initialMembers := app.Helper.GetAllMembers(ctx, t)

				output := app.Service.GetMemberById(ctx, &user.GetMemberByIdInput{
					TraceId:  "trace-test",
					MemberId: r.input.memberID,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no members were modified (read operation should not change anything)
					assert.Equal(t, initialMembers, afterMembers, r.name)
					return
				}

				// For successful retrieval
				assert.Equal(t, len(initialMembers), len(afterMembers), r.name)
				assert.NotZero(t, output.Member.Id, r.name)
				assert.Equal(t, r.expected.memberID, output.Member.Id, r.name)
				assert.Equal(t, r.expected.userID, output.Member.UserId, r.name)
				assert.Equal(t, r.expected.name, output.Member.Name, r.name)
				assert.Equal(t, r.expected.monthlyIncome, output.Member.MonthlyIncome, r.name)
				assert.NotZero(t, output.Member.CreatedAt, r.name)
				assert.NotZero(t, output.Member.UpdatedAt, r.name)

				// Verify no members were modified (read operation should not change anything)
				assert.Equal(t, initialMembers, afterMembers, r.name)
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "GetMemberById scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should get member successfully for first member",
						input: &input{
							memberID: Members[0].Id,
						},
						expected: &expected{
							success:       true,
							message:       "Member retrieved successfully",
							memberID:      Members[0].Id,
							userID:        Users[0].Id,
							name:          "John Doe",
							monthlyIncome: 5000000,
						},
					},
					{
						name: "Should get member successfully for second member",
						input: &input{
							memberID: Members[1].Id,
						},
						expected: &expected{
							success:       true,
							message:       "Member retrieved successfully",
							memberID:      Members[1].Id,
							userID:        Users[0].Id,
							name:          "Jane Doe",
							monthlyIncome: 3000000,
						},
					},
					{
						name: "Should get member successfully for third member",
						input: &input{
							memberID: Members[2].Id,
						},
						expected: &expected{
							success:       true,
							message:       "Member retrieved successfully",
							memberID:      Members[2].Id,
							userID:        Users[1].Id,
							name:          "Bob Smith",
							monthlyIncome: 7000000,
						},
					},

					// ===== Validation Tests: Member ID =====
					{
						name: "Should fail when member does not exist",
						input: &input{
							memberID: 99999,
						},
						expected: &expected{
							success: false,
							message: "Member not found",
						},
					},
					{
						name: "Should fail when member ID is zero",
						input: &input{
							memberID: 0,
						},
						expected: &expected{
							success: false,
							message: "Member ID is mandatory",
						},
					},
					{
						name: "Should fail when member ID is negative",
						input: &input{
							memberID: -1,
						},
						expected: &expected{
							success: false,
							message: "Member not found",
						},
					},
				})
			})

		})
	})
}

func TestGetMembersByUserId(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Get Members By User ID", func() {

			type input struct {
				userID int
			}
			type expected struct {
				success      bool
				message      string
				membersCount int
				memberIDs    []int
			}

			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			var Users []DataUser
			var Members []DataMember

			suite.Setup(func(ctx context.Context, app *UserApp) {
				// Setup test data
				Users = []DataUser{
					{
						Idx:      0,
						Username: "user1",
						Email:    "user1@test.com",
						FullName: "User One",
						Password: "password123",
					},
					{
						Idx:      1,
						Username: "user2",
						Email:    "user2@test.com",
						FullName: "User Two",
						Password: "password123",
					},
					{
						Idx:      2,
						Username: "user3",
						Email:    "user3@test.com",
						FullName: "User Three",
						Password: "password123",
					},
				}

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUser(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].Id = insertedUser.Id // Store actual database ID
				}

				Members = []DataMember{
					{
						Idx:           0,
						UserId:        Users[0].Id,
						Name:          "John Doe",
						MonthlyIncome: 5000000,
					},
					{
						Idx:           1,
						UserId:        Users[0].Id,
						Name:          "Jane Doe",
						MonthlyIncome: 3000000,
					},
					{
						Idx:           2,
						UserId:        Users[1].Id,
						Name:          "Bob Smith",
						MonthlyIncome: 7000000,
					},
				}

				// Insert members and store actual database IDs
				for i, memberData := range Members {
					insertedMember := app.Helper.InsertMember(ctx, t, memberData.UserId, memberData.Name, memberData.MonthlyIncome)
					Members[i].Id = insertedMember.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				initialMembers := app.Helper.GetAllMembers(ctx, t)

				output := app.Service.GetMembersByUserId(ctx, &user.GetMembersByUserIdInput{
					TraceId: "trace-test",
					UserId:  r.input.userID,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no members were modified (read operation should not change anything)
					assert.Equal(t, initialMembers, afterMembers, r.name)
					return
				}

				// For successful retrieval
				assert.Equal(t, len(initialMembers), len(afterMembers), r.name)
				assert.Equal(t, r.expected.membersCount, len(output.Members), r.name)

				// Verify all expected member IDs are present
				foundMemberIds := make([]int, 0, len(output.Members))
				for _, member := range output.Members {
					foundMemberIds = append(foundMemberIds, member.Id)
					assert.NotZero(t, member.Id, r.name)
					assert.Equal(t, r.input.userID, member.UserId, r.name)
					assert.NotEmpty(t, member.Name, r.name)
					assert.NotZero(t, member.MonthlyIncome, r.name)
					assert.NotZero(t, member.CreatedAt, r.name)
					assert.NotZero(t, member.UpdatedAt, r.name)
				}

				// Verify expected member IDs match
				if len(r.expected.memberIDs) > 0 {
					assert.ElementsMatch(t, r.expected.memberIDs, foundMemberIds, r.name)
				}

				// Verify no members were modified (read operation should not change anything)
				assert.Equal(t, initialMembers, afterMembers, r.name)
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "GetMembersByUserId scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should get multiple members for user with two members",
						input: &input{
							userID: Users[0].Id,
						},
						expected: &expected{
							success:      true,
							message:      "Member retrieved successfully",
							membersCount: 2,
							memberIDs:    []int{Members[0].Id, Members[1].Id},
						},
					},
					{
						name: "Should get single member for user with one member",
						input: &input{
							userID: Users[1].Id,
						},
						expected: &expected{
							success:      true,
							message:      "Member retrieved successfully",
							membersCount: 1,
							memberIDs:    []int{Members[2].Id},
						},
					},
					{
						name: "Should get empty list for user with no members",
						input: &input{
							userID: Users[2].Id,
						},
						expected: &expected{
							success:      true,
							message:      "Member retrieved successfully",
							membersCount: 0,
							memberIDs:    []int{},
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
							message: "User ID is mandatory",
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

func TestUserUpdateMemberInfo(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User UpdateMemberInfo", func() {

			var Users []DataUser
			var Members []DataMember

			type input struct {
				requesterID   int
				memberID      int
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

				Members = []DataMember{
					{
						Idx:           0,
						UserId:        Users[0].Id,
						Name:          "John Doe",
						MonthlyIncome: 5000000,
					},
					{
						Idx:           1,
						UserId:        Users[0].Id,
						Name:          "Jane Doe",
						MonthlyIncome: 3000000,
					},
					{
						Idx:           2,
						UserId:        Users[1].Id,
						Name:          "Bob Smith",
						MonthlyIncome: 7000000,
					},
				}

				// Insert members and store actual database IDs
				for i, memberData := range Members {
					insertedMember := app.Helper.InsertMember(ctx, t, memberData.UserId, memberData.Name, memberData.MonthlyIncome)
					Members[i].Id = insertedMember.Id // Store actual database ID
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				initialMembers := app.Helper.GetAllMembers(ctx, t)

				output := app.Service.UpdateMemberInfo(ctx, &user.UpdateMemberInfoInput{
					TraceId:       "trace-test",
					RequesterId:   r.input.requesterID,
					Id:            r.input.memberID,
					Name:          r.input.name,
					MonthlyIncome: r.input.monthlyIncome,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no members were modified
					assert.Equal(t, initialMembers, afterMembers, r.name)
					return
				}

				// For successful update
				assert.Equal(t, len(initialMembers), len(afterMembers), r.name)

				// Verify the member info was actually updated in afterMembers map
				updatedMember, exists := afterMembers[r.input.memberID]
				assert.True(t, exists, r.name+" - member should exist in afterMembers")
				assert.Equal(t, r.input.name, updatedMember.Name, r.name)
				assert.Equal(t, r.input.monthlyIncome, updatedMember.MonthlyIncome, r.name)

				// Update the initial members map to reflect the expected changes
				// (Name, MonthlyIncome and UpdatedAt will change for the target member)
				if initialMember, exists := initialMembers[r.input.memberID]; exists {
					initialMember.Name = r.input.name
					initialMember.MonthlyIncome = r.input.monthlyIncome
					initialMember.UpdatedAt = updatedMember.UpdatedAt
					initialMembers[r.input.memberID] = initialMember
				}

				// Verify all other members remain unchanged
				assert.Equal(t, initialMembers, afterMembers, r.name+" - only the target member should be modified")
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "UpdateMemberInfo scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should update member info successfully",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[0].Id,
							name:          "John Updated",
							monthlyIncome: 6000000,
						},
						expected: &expected{
							success: true,
							message: "Member info updated successfully",
						},
					},
					{
						name: "Should update only name",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[0].Id,
							name:          "John Smith",
							monthlyIncome: 6000000,
						},
						expected: &expected{
							success: true,
							message: "Member info updated successfully",
						},
					},
					{
						name: "Should update only monthly income",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[1].Id,
							name:          "Jane Doe",
							monthlyIncome: 4000000,
						},
						expected: &expected{
							success: true,
							message: "Member info updated successfully",
						},
					},
					{
						name: "Should update member info to zero income",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[1].Id,
							name:          "Jane Doe Updated",
							monthlyIncome: 0,
						},
						expected: &expected{
							success: true,
							message: "Member info updated successfully",
						},
					},
					{
						name: "Should update member info to large amount",
						input: &input{
							requesterID:   Users[1].Id,
							memberID:      Members[2].Id,
							name:          "Bob Johnson",
							monthlyIncome: 100000000,
						},
						expected: &expected{
							success: true,
							message: "Member info updated successfully",
						},
					},
					{
						name: "Should update same member multiple times",
						input: &input{
							requesterID:   Users[1].Id,
							memberID:      Members[2].Id,
							name:          "Robert Johnson",
							monthlyIncome: 50000000,
						},
						expected: &expected{
							success: true,
							message: "Member info updated successfully",
						},
					},
					{
						name: "Should update with minimum name length (2 characters)",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[0].Id,
							name:          "Jo",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: true,
							message: "Member info updated successfully",
						},
					},

					// ===== Validation Tests: Name =====
					{
						name: "Should fail when name is empty",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[0].Id,
							name:          "",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: false,
							message: "Member name is mandatory",
						},
					},
					{
						name: "Should fail when name is too short (1 character)",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[1].Id,
							name:          "A",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: false,
							message: "Member name must be at least 2 characters long",
						},
					},

					// ===== Validation Tests: Monthly Income =====
					{
						name: "Should fail when monthly income is negative",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[0].Id,
							name:          "John Doe",
							monthlyIncome: -1000000,
						},
						expected: &expected{
							success: false,
							message: "Monthly income cannot be negative",
						},
					},
					{
						name: "Should fail when monthly income is negative (-1)",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      Members[1].Id,
							name:          "Jane Doe",
							monthlyIncome: -1,
						},
						expected: &expected{
							success: false,
							message: "Monthly income cannot be negative",
						},
					},

					// ===== Validation Tests: Requester ID =====
					{
						name: "Should fail when requester ID is zero",
						input: &input{
							requesterID:   0,
							memberID:      Members[0].Id,
							name:          "John Doe",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: false,
							message: "Requester ID is mandatory",
						},
					},
					{
						name: "Should fail when requester is not the member owner",
						input: &input{
							requesterID:   Users[1].Id,
							memberID:      Members[0].Id,
							name:          "Unauthorized Update",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: false,
							message: "Unauthorized update",
						},
					},

					// ===== Validation Tests: Member ID =====
					{
						name: "Should fail when member does not exist",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      99999,
							name:          "Non Existent",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: false,
							message: "Member not found",
						},
					},
					{
						name: "Should fail when member ID is zero",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      0,
							name:          "Zero ID",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: false,
							message: "Member ID is mandatory",
						},
					},
					{
						name: "Should fail when member ID is negative",
						input: &input{
							requesterID:   Users[0].Id,
							memberID:      -1,
							name:          "Negative ID",
							monthlyIncome: 5000000,
						},
						expected: &expected{
							success: false,
							message: "Member not found",
						},
					},
				})
			})

		})
	})
}

func TestUserDeleteMember(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User DeleteMember", func() {

			var Users []DataUser
			var Members []DataMember

			type input struct {
				requesterID int
				memberID    int
			}
			type expected struct {
				success bool
				noOp    bool
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
					Users[i].Id = insertedUser.Id
				}

				Members = []DataMember{
					{
						Idx:           0,
						UserId:        Users[0].Id,
						Name:          "John Doe",
						MonthlyIncome: 5000000,
					},
					{
						Idx:           1,
						UserId:        Users[0].Id,
						Name:          "Jane Doe",
						MonthlyIncome: 3000000,
					},
					{
						Idx:           2,
						UserId:        Users[1].Id,
						Name:          "Bob Smith",
						MonthlyIncome: 7000000,
					},
				}

				// Insert members and store actual database IDs
				for i, memberData := range Members {
					insertedMember := app.Helper.InsertMember(ctx, t, memberData.UserId, memberData.Name, memberData.MonthlyIncome)
					Members[i].Id = insertedMember.Id
				}
			})

			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				initialMembers := app.Helper.GetAllMembers(ctx, t)

				output := app.Service.DeleteMember(ctx, &user.DeleteMemberInput{
					TraceId:     "trace-test",
					RequesterId: r.input.requesterID,
					Id:          r.input.memberID,
				})

				afterMembers := app.Helper.GetAllMembers(ctx, t)

				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false || r.expected.noOp {
					// Verify no members were deleted
					assert.Equal(t, initialMembers, afterMembers, r.name)
					return
				}

				// Member existed and should now be deleted
				assert.Equal(t, len(initialMembers)-1, len(afterMembers), r.name)

				// Verify the member no longer exists
				_, exists := afterMembers[r.input.memberID]
				assert.False(t, exists, r.name+" - member should not exist after deletion")

				// Verify all other members remain unchanged
				for id, initialMember := range initialMembers {
					if id == r.input.memberID {
						continue
					}
					afterMember, exists := afterMembers[id]
					assert.True(t, exists, r.name+" - other members should still exist")
					assert.Equal(t, initialMember, afterMember, r.name+" - other members should be unchanged")
				}
			}

			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			suite.Run(t, "DeleteMember scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Validation Tests: Requester ID =====
					{
						name: "Should fail when requester ID is zero",
						input: &input{
							requesterID: 0,
							memberID:    Members[0].Id,
						},
						expected: &expected{
							success: false,
							message: "Requester ID is mandatory",
						},
					},

					// ===== Validation Tests: Member ID =====
					{
						name: "Should fail when member ID is zero",
						input: &input{
							requesterID: Users[0].Id,
							memberID:    0,
						},
						expected: &expected{
							success: false,
							message: "Member ID is mandatory",
						},
					},

					// ===== Authorization Tests =====
					{
						name: "Should fail when requester is not the member owner",
						input: &input{
							requesterID: Users[1].Id,
							memberID:    Members[0].Id,
						},
						expected: &expected{
							success: false,
							message: "Unauthorized delete",
						},
					},
					{
						name: "Should fail when user tries to delete another user's member",
						input: &input{
							requesterID: Users[0].Id,
							memberID:    Members[2].Id,
						},
						expected: &expected{
							success: false,
							message: "Unauthorized delete",
						},
					},

					// ===== Success Tests =====
					{
						name: "Should delete member successfully",
						input: &input{
							requesterID: Users[0].Id,
							memberID:    Members[0].Id,
						},
						expected: &expected{
							success: true,
							message: "Member deleted successfully",
						},
					},
					{
						name: "Should delete another member from same user",
						input: &input{
							requesterID: Users[0].Id,
							memberID:    Members[1].Id,
						},
						expected: &expected{
							success: true,
							message: "Member deleted successfully",
						},
					},
					{
						name: "Should delete member from different user",
						input: &input{
							requesterID: Users[1].Id,
							memberID:    Members[2].Id,
						},
						expected: &expected{
							success: true,
							message: "Member deleted successfully",
						},
					},

					// ===== Idempotency Test =====
					{
						name: "Should return success when deleting non-existent member (idempotent)",
						input: &input{
							requesterID: Users[0].Id,
							memberID:    99999,
						},
						expected: &expected{
							success: true,
							noOp:    true,
							message: "Member deleted successfully",
						},
					},
					{
						name: "Should return success when deleting already deleted member (idempotent)",
						input: &input{
							requesterID: Users[0].Id,
							memberID:    Members[0].Id,
						},
						expected: &expected{
							success: true,
							noOp:    true,
							message: "Member deleted successfully",
						},
					},
				})
			})

		})
	})
}
