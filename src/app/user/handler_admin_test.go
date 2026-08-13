package user_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// ──────────────────────────────────────────────────────────────
// GET /api/v1/admin/users
// ──────────────────────────────────────────────────────────────

func TestHandlerAdminListUsers_SuccessMapsDTOsAndPagination(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.ListUsersReturns(&user.ListUsersOutput{
		Success: true,
		Message: "Users retrieved successfully",
		Users: []user.User{
			{Id: 1, Username: "alice", Email: "alice@example.com", FullName: "Alice", IsActive: true},
			{Id: 2, Username: "bob", Email: "bob@example.com", FullName: "Bob", IsActive: false},
		},
		Total: 12,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=2&page_size=5", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(9))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.AdminUserListResponse
	decodeJSON(t, rec, &resp)

	assert.True(t, resp.Success)
	assert.Equal(t, 2, len(resp.Data))
	assert.Equal(t, 1, resp.Data[0].Id)
	assert.True(t, resp.Data[0].IsActive)
	assert.False(t, resp.Data[1].IsActive)
	assert.Equal(t, 2, resp.Pagination.Page)
	assert.Equal(t, 5, resp.Pagination.PageSize)
	assert.Equal(t, 12, resp.Pagination.Total)
	assert.Equal(t, 3, resp.Pagination.TotalPages)
}

func TestHandlerAdminListUsers_InputsPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.ListUsersReturns(&user.ListUsersOutput{
		Success: true,
		Message: "Users retrieved successfully",
		Total:   0,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=3&page_size=7", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(42))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	_, input := fake.ListUsersArgsForCall(0)
	assert.Equal(t, 42, input.ActorId)
	assert.Equal(t, 3, input.Page)
	assert.Equal(t, 7, input.PageSize)
}

func TestHandlerAdminListUsers_DefaultsPageAndPageSize(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.ListUsersReturns(&user.ListUsersOutput{
		Success: true,
		Message: "Users retrieved successfully",
		Total:   0,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	_, input := fake.ListUsersArgsForCall(0)
	assert.Equal(t, 1, input.Page)
	assert.Equal(t, 10, input.PageSize)

	var resp user.AdminUserListResponse
	decodeJSON(t, rec, &resp)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 10, resp.Pagination.PageSize)
}

func TestHandlerAdminListUsers_ServiceFailReturns403(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.ListUsersReturns(&user.ListUsersOutput{
		Success:   false,
		Message:   "Unauthorized: only super user can list users",
		ErrorCode: user.ErrorCodeForbidden,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp user.AdminUserListResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Unauthorized: only super user can list users", resp.Message)
}

func TestHandlerAdminListUsers_InvalidPageReturns400(t *testing.T) {
	e, _ := newHandlerSetup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=abc", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp user.AdminUserListResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Page must be a positive integer", resp.Message)
}

func TestHandlerAdminListUsers_InvalidPageSizeReturns400(t *testing.T) {
	e, fake := newHandlerSetup()

	// Range validation (1..100) lives in the service layer; the handler maps
	// its validation error to 400.
	fake.ListUsersReturns(&user.ListUsersOutput{
		Success:   false,
		Message:   "Page size must be between 1 and 100",
		ErrorCode: user.ErrorCodeValidation,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page_size=-1", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp user.AdminUserListResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Page size must be between 1 and 100", resp.Message)
}

func TestHandlerAdminListUsers_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ──────────────────────────────────────────────────────────────
// GET /api/v1/admin/users/:id
// ──────────────────────────────────────────────────────────────

func TestHandlerAdminGetUserById_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AdminGetUserByIdReturns(&user.AdminGetUserByIdOutput{
		Success: true,
		Message: "User retrieved successfully",
		User: user.User{
			Id:       7,
			Username: "carol",
			Email:    "carol@example.com",
			FullName: "Carol White",
			IsActive: true,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/7", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(9))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.AdminUserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, 7, resp.Data.Id)
	assert.Equal(t, "carol", resp.Data.Username)
	assert.True(t, resp.Data.IsActive)

	_, input := fake.AdminGetUserByIdArgsForCall(0)
	assert.Equal(t, 9, input.ActorId)
	assert.Equal(t, 7, input.Id)
}

func TestHandlerAdminGetUserById_InvalidIdReturns400(t *testing.T) {
	e, _ := newHandlerSetup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/notanumber", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(9))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp user.AdminUserResponse
	decodeJSON(t, rec, &resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "User ID is mandatory", resp.Message)
}

func TestHandlerAdminGetUserById_NotFoundReturns400(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AdminGetUserByIdReturns(&user.AdminGetUserByIdOutput{
		Success:   false,
		Message:   "User not found",
		ErrorCode: user.ErrorCodeValidation,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/99999", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(9))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandlerAdminGetUserById_ForbiddenReturns403(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AdminGetUserByIdReturns(&user.AdminGetUserByIdOutput{
		Success:   false,
		Message:   "Unauthorized: only super user can look up users",
		ErrorCode: user.ErrorCodeForbidden,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/7", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandlerAdminGetUserById_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/7", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/admin/users/:id/deactivate + /reactivate
// ──────────────────────────────────────────────────────────────

func TestHandlerAdminDeactivateUser_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.SetUserActiveReturns(&user.SetUserActiveOutput{
		Success: true,
		Message: "User deactivated successfully",
		User: user.User{
			Id:       7,
			Username: "leaver",
			IsActive: false,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/7/deactivate", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(9))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.AdminUserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "User deactivated successfully", resp.Message)
	assert.Equal(t, 7, resp.Data.Id)
	assert.False(t, resp.Data.IsActive)

	_, input := fake.SetUserActiveArgsForCall(0)
	assert.Equal(t, 9, input.ActorId)
	assert.Equal(t, 7, input.UserId)
	assert.False(t, input.IsActive)
}

func TestHandlerAdminReactivateUser_Success(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.SetUserActiveReturns(&user.SetUserActiveOutput{
		Success: true,
		Message: "User activated successfully",
		User: user.User{
			Id:       7,
			Username: "returner",
			IsActive: true,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/7/reactivate", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(9))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.AdminUserResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.True(t, resp.Data.IsActive)

	_, input := fake.SetUserActiveArgsForCall(0)
	assert.True(t, input.IsActive)
}

func TestHandlerAdminSetUserActive_InvalidIdReturns400(t *testing.T) {
	e, _ := newHandlerSetup()

	for _, path := range []string{"/api/v1/admin/users/0/deactivate", "/api/v1/admin/users/abc/reactivate"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set(echo.HeaderAuthorization, bearerToken(9))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code, path)

		var resp user.AdminUserResponse
		decodeJSON(t, rec, &resp)
		assert.False(t, resp.Success)
		assert.Equal(t, "User ID is mandatory", resp.Message, path)
	}
}

func TestHandlerAdminSetUserActive_ServiceFailReturns403(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.SetUserActiveReturns(&user.SetUserActiveOutput{
		Success:   false,
		Message:   "Unauthorized: only super user can manage user accounts",
		ErrorCode: user.ErrorCodeForbidden,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/7/deactivate", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandlerAdminSetUserActive_NoTokenReturns401(t *testing.T) {
	e, _ := newHandlerSetup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/7/deactivate", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
