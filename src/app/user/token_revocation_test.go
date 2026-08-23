package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A5: token revocation + refresh rotation, exercised end to end through the
// real service against the isolated test schema.

// registerUser creates a user through the real service and returns the account
// plus the issued access/refresh pair.
func registerUser(t *testing.T, app *UserApp, username, email, password string) (user.User, string, string) {
	t.Helper()
	out := app.Service.Register(context.Background(), &user.RegisterInput{
		TraceId:  "trace-register",
		Username: username,
		Email:    email,
		FullName: "Token Test User",
		Password: password,
	})
	require.True(t, out.Success, out.Message)
	require.NotEmpty(t, out.AccessToken, "register must issue an access token")
	require.NotEmpty(t, out.RefreshToken, "register must issue a refresh token")
	return out.User, out.AccessToken, out.RefreshToken
}

// loginUser authenticates through the real service and returns the account
// plus the issued access/refresh pair.
func loginUser(t *testing.T, app *UserApp, username, password string) (user.User, string, string) {
	t.Helper()
	out := app.Service.Login(context.Background(), &user.LoginInput{
		TraceId:  "trace-login",
		Username: username,
		Password: password,
	})
	require.True(t, out.Success, out.Message)
	require.NotEmpty(t, out.AccessToken, "login must issue an access token")
	require.NotEmpty(t, out.RefreshToken, "login must issue a refresh token")
	return out.User, out.AccessToken, out.RefreshToken
}

// refreshToken rotates a refresh token through the real service.
func refreshToken(t *testing.T, app *UserApp, token string) *user.RefreshOutput {
	t.Helper()
	return app.Service.Refresh(context.Background(), &user.RefreshInput{
		TraceId:      "trace-refresh",
		RefreshToken: token,
	})
}

// logoutToken revokes a refresh token through the real service.
func logoutToken(t *testing.T, app *UserApp, token string) *user.LogoutOutput {
	t.Helper()
	return app.Service.Logout(context.Background(), &user.LogoutInput{
		TraceId:      "trace-logout",
		RefreshToken: token,
	})
}

func TestTokenRevocation_RegisterPersistsRefreshToken(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Register persists refresh token", func() {
			suite.Run(t, "register issues a persisted refresh token", func(t *testing.T, ctx context.Context, app *UserApp) {
				userData, _, _ := registerUser(t, app, "freshreg", "freshreg@example.com", "password12345")
				assert.Equal(t, 1, app.Helper.CountRefreshTokens(ctx, t, userData.Id))
				assert.Equal(t, 1, app.Helper.CountActiveRefreshTokens(ctx, t, userData.Id))
			})
		})
	})
}

func TestTokenRevocation_LoginPersistsRefreshToken(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Login persists refresh token", func() {
			suite.Setup(func(ctx context.Context, app *UserApp) {
				app.Helper.InsertUserWithHashedPassword(ctx, t, "tokenuser1", "token1@example.com", "Token User", "password123")
			})
			suite.Run(t, "login issues a persisted refresh token", func(t *testing.T, ctx context.Context, app *UserApp) {
				userData, _, _ := loginUser(t, app, "tokenuser1", "password123")
				assert.Equal(t, 1, app.Helper.CountRefreshTokens(ctx, t, userData.Id))
				assert.Equal(t, 1, app.Helper.CountActiveRefreshTokens(ctx, t, userData.Id))
			})
		})
	})
}

func TestTokenRevocation_RefreshRotation(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Refresh rotation", func() {
			suite.Setup(func(ctx context.Context, app *UserApp) {
				app.Helper.InsertUserWithHashedPassword(ctx, t, "rotateuser", "rotate@example.com", "Rotate User", "password123")
			})
			suite.Run(t, "rotated token is revoked and the new pair works", func(t *testing.T, ctx context.Context, app *UserApp) {
				userData, _, refresh1 := loginUser(t, app, "rotateuser", "password123")

				out1 := refreshToken(t, app, refresh1)
				require.True(t, out1.Success, out1.Message)
				assert.NotEmpty(t, out1.AccessToken)
				assert.NotEmpty(t, out1.RefreshToken)
				assert.NotEqual(t, refresh1, out1.RefreshToken, "rotation must mint a new refresh token")
				assert.Equal(t, 1, app.Helper.CountActiveRefreshTokens(ctx, t, userData.Id))

				// The old token is revoked and can never be used again.
				replay := refreshToken(t, app, refresh1)
				assert.False(t, replay.Success)
				assert.Equal(t, user.ErrorCodeUnauthorized, replay.ErrorCode)
				assert.Equal(t, "Invalid or expired refresh token", replay.Message)

				// The rotated token is still good and rotates again.
				out2 := refreshToken(t, app, out1.RefreshToken)
				require.True(t, out2.Success, out2.Message)
				assert.NotEqual(t, out1.RefreshToken, out2.RefreshToken)
				assert.Equal(t, 1, app.Helper.CountActiveRefreshTokens(ctx, t, userData.Id))
				assert.Equal(t, 3, app.Helper.CountRefreshTokens(ctx, t, userData.Id))

				// Replay of the first rotated token is rejected too.
				assert.False(t, refreshToken(t, app, out1.RefreshToken).Success)
			})
		})
	})
}

func TestTokenRevocation_Logout(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Logout", func() {
			suite.Setup(func(ctx context.Context, app *UserApp) {
				app.Helper.InsertUserWithHashedPassword(ctx, t, "logoutuser", "logout@example.com", "Logout User", "password123")
			})
			suite.Run(t, "logout revokes the refresh token server-side", func(t *testing.T, ctx context.Context, app *UserApp) {
				userData, _, refresh := loginUser(t, app, "logoutuser", "password123")

				out := logoutToken(t, app, refresh)
				assert.True(t, out.Success)
				assert.Equal(t, 0, app.Helper.CountActiveRefreshTokens(ctx, t, userData.Id))

				// A revoked token cannot be used to refresh.
				refreshOut := refreshToken(t, app, refresh)
				assert.False(t, refreshOut.Success)
				assert.Equal(t, user.ErrorCodeUnauthorized, refreshOut.ErrorCode)

				// Logout is idempotent: already-revoked, unknown, and missing
				// tokens still report success.
				assert.True(t, logoutToken(t, app, refresh).Success)
				assert.True(t, logoutToken(t, app, "never-issued-token").Success)
				assert.True(t, logoutToken(t, app, "").Success)
			})
		})
	})
}

func TestTokenRevocation_UpdatePasswordInvalidatesSessions(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "UpdatePassword invalidates sessions", func() {
			suite.Setup(func(ctx context.Context, app *UserApp) {
				app.Helper.InsertUserWithHashedPassword(ctx, t, "passuser1", "pass1@example.com", "Pass User", "password123")
			})
			suite.Run(t, "password change revokes every refresh token and bumps token_version", func(t *testing.T, ctx context.Context, app *UserApp) {
				userData, _, refresh := loginUser(t, app, "passuser1", "password123")
				rotated := refreshToken(t, app, refresh)
				require.True(t, rotated.Success, rotated.Message)

				app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
					return createGrantedPermissionCheck()
				}

				out := app.Service.UpdatePassword(ctx, &user.UpdatePasswordInput{
					TraceId:     "trace-pass",
					Id:          userData.Id,
					OldPassword: "password123",
					NewPassword: "newpassword123",
				})
				require.True(t, out.Success, out.Message)

				// Every refresh token is revoked and the version is bumped.
				assert.Equal(t, 0, app.Helper.CountActiveRefreshTokens(ctx, t, userData.Id))
				assert.Equal(t, 1, app.Helper.GetUserTokenVersion(ctx, t, userData.Id))

				// Old refresh tokens (pre- and post-rotation) are all invalid.
				for _, old := range []string{refresh, rotated.RefreshToken} {
					refreshOut := refreshToken(t, app, old)
					assert.False(t, refreshOut.Success, "old token must be rejected after password change")
					assert.Equal(t, user.ErrorCodeUnauthorized, refreshOut.ErrorCode)
				}

				// A fresh login with the new password works and its token can rotate.
				_, _, newRefresh := loginUser(t, app, "passuser1", "newpassword123")
				assert.Equal(t, 1, app.Helper.CountActiveRefreshTokens(ctx, t, userData.Id))
				assert.True(t, refreshToken(t, app, newRefresh).Success)
			})
		})
	})
}
