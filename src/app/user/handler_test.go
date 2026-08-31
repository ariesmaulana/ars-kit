package user_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/ariesmaulana/ars-kit/src/app/user/userfakes"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────

var handlerJWTConfig = user.JWTConfig{
	SecretKey:       "handler-test-secret",
	ExpirationHours: 24,
	CookieName:      "auth_token",
}

// newHandlerSetup creates an Echo instance wired with a fake service.
func newHandlerSetup() (*echo.Echo, *userfakes.ServiceFake) {
	e := echo.New()
	fake := &userfakes.ServiceFake{}
	jwtService := user.NewJWTService(handlerJWTConfig)
	h := user.NewHandler(fake, jwtService)
	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1)
	return e, fake
}

// bearerToken generates a valid JWT for userID and returns the Authorization header value.
func bearerToken(userID int) string {
	svc := user.NewJWTService(handlerJWTConfig)
	token, _ := svc.GenerateToken(userID, "testuser", 0)
	return "Bearer " + token
}

// tokenPair generates a valid access + refresh pair for userID, as the real
// service would hand back from Login/Register/Refresh.
func tokenPair(userID int) (access, refresh string) {
	svc := user.NewJWTService(handlerJWTConfig)
	access, _ = svc.GenerateToken(userID, "testuser", 0)
	refresh, _ = svc.GenerateRefreshToken()
	return access, refresh
}

// jsonBody encodes v as JSON into a buffer suitable for request bodies.
func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}

// decodeJSON decodes an HTTP response body into dst.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, dst interface{}) {
	t.Helper()
	assert.NoError(t, json.NewDecoder(rec.Body).Decode(dst))
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/users/register
// ──────────────────────────────────────────────────────────────

func TestHandlerRegister_SuccessMapsUserDTO(t *testing.T) {
	e, fake := newHandlerSetup()

	access, refresh := tokenPair(42)
	now := time.Now().Truncate(time.Second)
	fake.RegisterReturns(&user.RegisterOutput{
		Success:      true,
		Message:      "User registered successfully",
		AccessToken:  access,
		RefreshToken: refresh,
		User: user.User{
			Id:        42,
			Username:  "alice",
			Email:     "alice@example.com",
			FullName:  "Alice Smith",
			CreatedAt: now,
			UpdatedAt: now,
		},
	})

	body := jsonBody(t, map[string]string{
		"username":  "alice",
		"email":     "alice@example.com",
		"full_name": "Alice Smith",
		"password":  "secret123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp user.AuthResponse
	decodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.Equal(t, access, resp.Token)
	assert.Equal(t, refresh, resp.RefreshToken)
	assert.Equal(t, 42, resp.User.Id)
	assert.Equal(t, "alice", resp.User.Username)
	assert.Equal(t, "alice@example.com", resp.User.Email)
	assert.Equal(t, "Alice Smith", resp.User.FullName)

	// The refresh token is delivered as an HTTP-only cookie too.
	assert.Len(t, rec.Result().Cookies(), 2)
}

func TestHandlerRegister_ServiceFailReturns400(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.RegisterReturns(&user.RegisterOutput{
		Success:   false,
		Message:   "Username or email already exists",
		ErrorCode: user.ErrorCodeValidation,
	})

	body := jsonBody(t, map[string]string{
		"username": "alice", "email": "alice@example.com",
		"full_name": "Alice", "password": "secret123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp user.AuthResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Username or email already exists", resp.Message)
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/users/login
// ──────────────────────────────────────────────────────────────

func TestHandlerLogin_SuccessMapsUserDTO(t *testing.T) {
	e, fake := newHandlerSetup()

	access, refresh := tokenPair(7)
	fake.LoginReturns(&user.LoginOutput{
		Success:      true,
		Message:      "Login successful",
		AccessToken:  access,
		RefreshToken: refresh,
		User: user.User{
			Id:       7,
			Username: "bob",
			Email:    "bob@example.com",
			FullName: "Bob Jones",
		},
	})

	body := jsonBody(t, map[string]string{"username": "bob", "password": "pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.AuthResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, access, resp.Token)
	assert.Equal(t, refresh, resp.RefreshToken)
	assert.Equal(t, 7, resp.User.Id)
	assert.Equal(t, "bob", resp.User.Username)
	assert.Equal(t, "bob@example.com", resp.User.Email)
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/users/refresh
// ──────────────────────────────────────────────────────────────

func TestHandlerRefresh_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	access, refresh := tokenPair(9)
	fake.RefreshReturns(&user.RefreshOutput{
		Success:      true,
		Message:      "Session refreshed",
		AccessToken:  access,
		RefreshToken: refresh,
		User: user.User{
			Id:       9,
			Username: "carol",
			Email:    "carol@example.com",
			FullName: "Carol White",
		},
	})

	body := jsonBody(t, map[string]string{"refresh_token": "old.refresh.token"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.AuthResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, access, resp.Token)
	assert.Equal(t, refresh, resp.RefreshToken)
	assert.Equal(t, 9, resp.User.Id)

	// The presented refresh token is passed to the service for rotation.
	_, input := fake.RefreshArgsForCall(0)
	assert.Equal(t, "old.refresh.token", input.RefreshToken)
}

func TestHandlerRefresh_ReadsTokenFromCookieWhenBodyEmpty(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.RefreshReturns(&user.RefreshOutput{
		Success: true,
		Message: "Session refreshed",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", bytes.NewBufferString("{}"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie.refresh.token"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	_, input := fake.RefreshArgsForCall(0)
	assert.Equal(t, "cookie.refresh.token", input.RefreshToken)
}

func TestHandlerRefresh_ServiceFailReturns401(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.RefreshReturns(&user.RefreshOutput{
		Success:   false,
		Message:   "Invalid or expired refresh token",
		ErrorCode: user.ErrorCodeUnauthorized,
	})

	body := jsonBody(t, map[string]string{"refresh_token": "stale.token"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp user.AuthResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Invalid or expired refresh token", resp.Message)
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/users/logout
// ──────────────────────────────────────────────────────────────

func TestHandlerLogout_RevokesServerSideAndClearsCookies(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.LogoutReturns(&user.LogoutOutput{
		Success: true,
		Message: "Logged out successfully",
	})

	body := jsonBody(t, map[string]string{"refresh_token": "token.to.revoke"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/logout", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	_, input := fake.LogoutArgsForCall(0)
	assert.Equal(t, "token.to.revoke", input.RefreshToken)

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)

	// Both cookies (auth + refresh) are cleared.
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 2)
	for _, cookie := range cookies {
		assert.Equal(t, -1, cookie.MaxAge)
	}
}

func TestHandlerLogout_WithoutRefreshTokenStillClearsCookies(t *testing.T) {
	e, fake := newHandlerSetup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/logout", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Zero(t, fake.LogoutCallCount(), "no refresh token, so nothing to revoke server-side")

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
}

func TestHandlerLogin_ServiceFailReturns401(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.LoginReturns(&user.LoginOutput{
		Success:   false,
		Message:   "Invalid username or password",
		ErrorCode: user.ErrorCodeUnauthorized,
	})

	body := jsonBody(t, map[string]string{"username": "bob", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp user.AuthResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Invalid username or password", resp.Message)
}

func TestHandlerLogin_LockedAccountReturns429WithLockoutState(t *testing.T) {
	e, fake := newHandlerSetup()

	lockedUntil := time.Now().Add(15 * time.Minute)
	fake.LoginReturns(&user.LoginOutput{
		Success:           false,
		Message:           "Account temporarily locked. Try again in 15 minute(s).",
		ErrorCode:         user.ErrorCodeLocked,
		LockedUntil:       &lockedUntil,
		RetryAfterSeconds: 900,
	})

	body := jsonBody(t, map[string]string{"username": "bob", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	var resp user.AuthResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Account temporarily locked. Try again in 15 minute(s).", resp.Message)
	assert.NotNil(t, resp.LockedUntil)
	assert.Equal(t, 900, resp.RetryAfterSeconds)
}

// ──────────────────────────────────────────────────────────────
// GET /api/v1/users/profile
// ──────────────────────────────────────────────────────────────

func TestHandlerProfile_SuccessMapsUserDTO(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.GetProfileByIdReturns(&user.GetProfileByIdOutput{
		Success: true,
		Message: "Profile retrieved successfully",
		User: user.User{
			Id:       1,
			Username: "carol",
			Email:    "carol@example.com",
			FullName: "Carol White",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.Data.Id)
	assert.Equal(t, "carol", resp.Data.Username)
	assert.Equal(t, "carol@example.com", resp.Data.Email)
	assert.Equal(t, "Carol White", resp.Data.FullName)
}

func TestHandlerProfile_UserIDFromJWTPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.GetProfileByIdReturns(&user.GetProfileByIdOutput{
		Success: true,
		User:    user.User{Id: 99},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(99))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, input := fake.GetProfileByIdArgsForCall(0)
	assert.Equal(t, 99, input.Id)
}

func TestHandlerProfile_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ──────────────────────────────────────────────────────────────
// PUT /api/v1/users/profile/username
// ──────────────────────────────────────────────────────────────

func TestHandlerUpdateUsername_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UpdateUsernameReturns(&user.UpdateUsernameOutput{
		Success: true,
		Message: "Username updated successfully",
		User: user.User{
			Id:       5,
			Username: "newname",
			Email:    "carol@example.com",
			FullName: "Carol White",
		},
	})

	body := jsonBody(t, map[string]string{"new_username": "newname"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/username", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(5))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "Username updated successfully", resp.Message)
	assert.Equal(t, 5, resp.Data.Id)
	assert.Equal(t, "newname", resp.Data.Username)
}

func TestHandlerUpdateUsername_UserIDFromJWTPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UpdateUsernameReturns(&user.UpdateUsernameOutput{
		Success: true,
		Message: "Username updated successfully",
		User:    user.User{Id: 99},
	})

	body := jsonBody(t, map[string]string{"new_username": "newname"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/username", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(99))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, input := fake.UpdateUsernameArgsForCall(0)
	assert.Equal(t, 99, input.Id)
	assert.Equal(t, "newname", input.NewUsername)
}

func TestHandlerUpdateUsername_PermissionDeniedReturns403(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UpdateUsernameReturns(&user.UpdateUsernameOutput{
		Success:   false,
		Message:   "Unauthorized: you do not have permission to update profile",
		ErrorCode: user.ErrorCodeForbidden,
	})

	body := jsonBody(t, map[string]string{"new_username": "newname"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/username", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(5))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Unauthorized: you do not have permission to update profile", resp.Message)
}

func TestHandlerUpdateUsername_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	body := jsonBody(t, map[string]string{"new_username": "newname"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/username", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ──────────────────────────────────────────────────────────────
// PUT /api/v1/users/profile/password
// ──────────────────────────────────────────────────────────────

func TestHandlerUpdatePassword_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UpdatePasswordReturns(&user.UpdatePasswordOutput{
		Success: true,
		Message: "Password updated successfully",
	})

	body := jsonBody(t, map[string]string{"old_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/password", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(5))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "Password updated successfully", resp.Message)
}

func TestHandlerUpdatePassword_InvalidOldPasswordReturns401(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UpdatePasswordReturns(&user.UpdatePasswordOutput{
		Success:   false,
		Message:   "Invalid old password",
		ErrorCode: user.ErrorCodeUnauthorized,
	})

	body := jsonBody(t, map[string]string{"old_password": "wrong", "new_password": "newpass123"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/password", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(5))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Invalid old password", resp.Message)
}

func TestHandlerUpdatePassword_PermissionDeniedReturns403(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UpdatePasswordReturns(&user.UpdatePasswordOutput{
		Success:   false,
		Message:   "Unauthorized: you do not have permission to update password",
		ErrorCode: user.ErrorCodeForbidden,
	})

	body := jsonBody(t, map[string]string{"old_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/password", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(5))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp user.UserResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Unauthorized: you do not have permission to update password", resp.Message)
}

func TestHandlerUpdatePassword_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	body := jsonBody(t, map[string]string{"old_password": "oldpass123", "new_password": "newpass123"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/profile/password", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/users/roles/assign
// ──────────────────────────────────────────────────────────────

func TestHandlerAssignRole_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AssignRoleReturns(&user.AssignRoleOutput{
		Success: true,
		Message: "Role assigned successfully",
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id": 2,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/assign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.ManageRoleResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "Role assigned successfully", resp.Message)
}

func TestHandlerAssignRole_InputsPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AssignRoleReturns(&user.AssignRoleOutput{
		Success: true,
		Message: "Role assigned successfully",
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id": 11,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/assign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(7))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, input := fake.AssignRoleArgsForCall(0)
	assert.Equal(t, 7, input.ActorId)         // from JWT
	assert.Equal(t, 11, input.TargetUserId)   // from body
	assert.Equal(t, "member", input.RoleName) // from body
}

func TestHandlerAssignRole_ServiceFailReturns403(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AssignRoleReturns(&user.AssignRoleOutput{
		Success:   false,
		Message:   "Unauthorized: only super user can assign roles",
		ErrorCode: user.ErrorCodeForbidden,
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id": 2,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/assign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp user.ManageRoleResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Unauthorized: only super user can assign roles", resp.Message)
}

func TestHandlerAssignRole_ValidationFailReturns400(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AssignRoleReturns(&user.AssignRoleOutput{
		Success:   false,
		Message:   "Target user ID is mandatory",
		ErrorCode: user.ErrorCodeValidation,
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id": 0,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/assign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp user.ManageRoleResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Target user ID is mandatory", resp.Message)
}

func TestHandlerAssignRole_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	body := jsonBody(t, map[string]interface{}{
		"user_id": 2,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/assign", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/users/roles/unassign
// ──────────────────────────────────────────────────────────────

func TestHandlerUnassignRole_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UnassignRoleReturns(&user.UnassignRoleOutput{
		Success: true,
		Message: "Role unassigned successfully",
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id": 2,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/unassign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.ManageRoleResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "Role unassigned successfully", resp.Message)
}

func TestHandlerUnassignRole_InputsPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UnassignRoleReturns(&user.UnassignRoleOutput{
		Success: true,
		Message: "Role unassigned successfully",
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id": 11,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/unassign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(7))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, input := fake.UnassignRoleArgsForCall(0)
	assert.Equal(t, 7, input.ActorId)         // from JWT
	assert.Equal(t, 11, input.TargetUserId)   // from body
	assert.Equal(t, "member", input.RoleName) // from body
}

func TestHandlerUnassignRole_ServiceFailReturns403(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UnassignRoleReturns(&user.UnassignRoleOutput{
		Success:   false,
		Message:   "Unauthorized: only super user can revoke permissions",
		ErrorCode: user.ErrorCodeForbidden,
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id": 2,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/unassign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp user.ManageRoleResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Unauthorized: only super user can revoke permissions", resp.Message)
}

func TestHandlerUnassignRole_ValidationFailReturns400(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.UnassignRoleReturns(&user.UnassignRoleOutput{
		Success:   false,
		Message:   "Permission is mandatory",
		ErrorCode: user.ErrorCodeValidation,
	})

	body := jsonBody(t, map[string]interface{}{
		"user_id":    2,
		"permission": "",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/unassign", body)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp user.ManageRoleResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Permission is mandatory", resp.Message)
}

func TestHandlerUnassignRole_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	body := jsonBody(t, map[string]interface{}{
		"user_id": 2,
		"role":    "member",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/roles/unassign", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
