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
	token, _ := svc.GenerateToken(userID, "testuser")
	return "Bearer " + token
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

	now := time.Now().Truncate(time.Second)
	fake.RegisterReturns(&user.RegisterOutput{
		Success: true,
		Message: "User registered successfully",
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
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, 42, resp.User.Id)
	assert.Equal(t, "alice", resp.User.Username)
	assert.Equal(t, "alice@example.com", resp.User.Email)
	assert.Equal(t, "Alice Smith", resp.User.FullName)
}

func TestHandlerRegister_ServiceFailReturns400(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.RegisterReturns(&user.RegisterOutput{
		Success: false,
		Message: "Username or email already exists",
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

	fake.LoginReturns(&user.LoginOutput{
		Success: true,
		Message: "Login successful",
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
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, 7, resp.User.Id)
	assert.Equal(t, "bob", resp.User.Username)
	assert.Equal(t, "bob@example.com", resp.User.Email)
}

func TestHandlerLogin_ServiceFailReturns401(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.LoginReturns(&user.LoginOutput{
		Success: false,
		Message: "Invalid username or password",
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
// GET /api/v1/users/members
// ──────────────────────────────────────────────────────────────

func TestHandlerGetMembers_MapsMemberDTOsAndPagination(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.GetMembersByUserIdReturns(&user.GetMembersByUserIdOutput{
		Success: true,
		Message: "Member retrieved successfully",
		Members: []user.Member{
			{Id: 10, UserId: 1, Name: "Alice", MonthlyIncome: 5000000},
			{Id: 11, UserId: 1, Name: "Bob", MonthlyIncome: 3000000},
		},
		Total:      5,
		TotalPages: 3,
		Page:       1,
		PageSize:   2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/members?page=1&page_size=2", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp user.MembersResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, 10, resp.Data[0].Id)
	assert.Equal(t, "Alice", resp.Data[0].Name)
	assert.Equal(t, 5000000, resp.Data[0].MonthlyIncome)
	assert.Equal(t, 11, resp.Data[1].Id)

	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 2, resp.Pagination.PageSize)
	assert.Equal(t, 5, resp.Pagination.Total)
	assert.Equal(t, 3, resp.Pagination.TotalPages)
}

func TestHandlerGetMembers_PageQueryParamsPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.GetMembersByUserIdReturns(&user.GetMembersByUserIdOutput{
		Success: true, Page: 3, PageSize: 20,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/members?page=3&page_size=20", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, input := fake.GetMembersByUserIdArgsForCall(0)
	assert.Equal(t, 3, input.Page)
	assert.Equal(t, 20, input.PageSize)
}

// ──────────────────────────────────────────────────────────────
// POST /api/v1/users/members
// ──────────────────────────────────────────────────────────────

func TestHandlerAddMember_MapsMemberDTO(t *testing.T) {
	e, fake := newHandlerSetup()

	fake.AddMemberReturns(&user.AddMemberOutput{
		Success: true,
		Message: "Member added successfully",
		Member: user.Member{
			Id:            55,
			UserId:        1,
			Name:          "Dave",
			MonthlyIncome: 4000000,
		},
	})

	body := jsonBody(t, map[string]interface{}{"name": "Dave", "monthly_income": 4000000})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/members", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp user.MemberResponse
	decodeJSON(t, rec, &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, 55, resp.Data.Id)
	assert.Equal(t, 1, resp.Data.UserId)
	assert.Equal(t, "Dave", resp.Data.Name)
	assert.Equal(t, 4000000, resp.Data.MonthlyIncome)
}

func TestHandlerAddMember_BodyFieldsPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()
	fake.AddMemberReturns(&user.AddMemberOutput{Success: true, Member: user.Member{Id: 1}})

	body := jsonBody(t, map[string]interface{}{"name": "Eve", "monthly_income": 7500000})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/members", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(5))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	_, input := fake.AddMemberArgsForCall(0)
	assert.Equal(t, 5, input.Id) // user ID from JWT
	assert.Equal(t, "Eve", input.Name)
	assert.Equal(t, 7500000, input.MonthlyIncome)
}

// ──────────────────────────────────────────────────────────────
// PUT /api/v1/users/members/:id
// ──────────────────────────────────────────────────────────────

func TestHandlerUpdateMemberInfo_PathParamAndBodyPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()
	fake.UpdateMemberInfoReturns(&user.UpdateMemberInfoOutput{Success: true})

	body := jsonBody(t, map[string]interface{}{"name": "Updated Name", "monthly_income": 8000000})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/members/77", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, input := fake.UpdateMemberInfoArgsForCall(0)
	assert.Equal(t, 77, input.Id)           // member ID from path
	assert.Equal(t, 3, input.RequesterId)   // user ID from JWT
	assert.Equal(t, "Updated Name", input.Name)
	assert.Equal(t, 8000000, input.MonthlyIncome)
}

func TestHandlerUpdateMemberInfo_ServiceFailReturns400(t *testing.T) {
	e, fake := newHandlerSetup()
	fake.UpdateMemberInfoReturns(&user.UpdateMemberInfoOutput{
		Success: false,
		Message: "Member not found",
	})

	body := jsonBody(t, map[string]interface{}{"name": "X", "monthly_income": 0})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/members/999", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ──────────────────────────────────────────────────────────────
// DELETE /api/v1/users/members/:id
// ──────────────────────────────────────────────────────────────

func TestHandlerDeleteMember_PathParamPassedToService(t *testing.T) {
	e, fake := newHandlerSetup()
	fake.DeleteMemberReturns(&user.DeleteMemberOutput{Success: true})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/members/88", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(2))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, input := fake.DeleteMemberArgsForCall(0)
	assert.Equal(t, 88, input.Id)         // member ID from path
	assert.Equal(t, 2, input.RequesterId) // user ID from JWT
}

func TestHandlerDeleteMember_UnauthorizedReturns403(t *testing.T) {
	e, fake := newHandlerSetup()
	fake.DeleteMemberReturns(&user.DeleteMemberOutput{
		Success: false,
		Message: "Unauthorized delete",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/members/88", nil)
	req.Header.Set(echo.HeaderAuthorization, bearerToken(2))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
