package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWTService(t *testing.T) {
	t.Run("should create JWT service with default values", func(t *testing.T) {
		config := JWTConfig{
			SecretKey: "test-secret",
		}

		service := NewJWTService(config)

		assert.NotNil(t, service)
		assert.Equal(t, 24, service.config.ExpirationHours)
		assert.Equal(t, "auth_token", service.config.CookieName)
	})

	t.Run("should create JWT service with custom values", func(t *testing.T) {
		config := JWTConfig{
			SecretKey:       "test-secret",
			ExpirationHours: 48,
			CookieName:      "custom_token",
		}

		service := NewJWTService(config)

		assert.NotNil(t, service)
		assert.Equal(t, 48, service.config.ExpirationHours)
		assert.Equal(t, "custom_token", service.config.CookieName)
	})
}

func TestJWTService_GenerateToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		ExpirationHours: 24,
	})

	t.Run("should generate valid token", func(t *testing.T) {
		token, err := service.GenerateToken(123, "testuser")

		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := service.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, 123, claims.UserId)
		assert.Equal(t, "testuser", claims.Username)
	})

	t.Run("should generate different tokens for different users", func(t *testing.T) {
		token1, err1 := service.GenerateToken(1, "user1")
		token2, err2 := service.GenerateToken(2, "user2")

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotEqual(t, token1, token2)
	})
}

func TestJWTService_ValidateToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		ExpirationHours: 24,
	})

	t.Run("should validate valid token", func(t *testing.T) {
		token, _ := service.GenerateToken(123, "testuser")

		claims, err := service.ValidateToken(token)

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, 123, claims.UserId)
		assert.Equal(t, "testuser", claims.Username)
	})

	t.Run("should reject invalid token", func(t *testing.T) {
		claims, err := service.ValidateToken("invalid.token.string")

		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.True(t, errors.Is(err, ErrInvalidToken))
	})

	t.Run("should reject token with wrong secret", func(t *testing.T) {
		wrongService := NewJWTService(JWTConfig{
			SecretKey: "wrong-secret",
		})
		token, _ := wrongService.GenerateToken(123, "testuser")

		claims, err := service.ValidateToken(token)

		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("should reject expired token", func(t *testing.T) {
		claims := JWTClaims{
			UserId:   123,
			Username: "testuser",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret-key"))

		result, err := service.ValidateToken(tokenString)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrExpiredToken))
	})
}

func TestJWTService_SetTokenCookie(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret",
		ExpirationHours: 24,
		CookieName:      "test_token",
		CookieDomain:    "example.com",
		CookieSecure:    true,
		CookieHTTPOnly:  true,
	})

	t.Run("should set cookie with correct attributes", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token := "test.jwt.token"
		service.SetTokenCookie(c, token)

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)

		cookie := cookies[0]
		assert.Equal(t, "test_token", cookie.Name)
		assert.Equal(t, token, cookie.Value)
		assert.Equal(t, "/", cookie.Path)
		assert.Equal(t, "example.com", cookie.Domain)
		assert.Equal(t, 24*3600, cookie.MaxAge)
		assert.True(t, cookie.Secure)
		assert.True(t, cookie.HttpOnly)
		assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
	})
}

func TestJWTService_ClearTokenCookie(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:      "test-secret",
		CookieName:     "test_token",
		CookieHTTPOnly: true,
	})

	t.Run("should clear cookie", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		service.ClearTokenCookie(c)

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)

		cookie := cookies[0]
		assert.Equal(t, "test_token", cookie.Name)
		assert.Equal(t, "", cookie.Value)
		assert.Equal(t, -1, cookie.MaxAge)
		assert.True(t, cookie.HttpOnly)
	})
}

func TestJWTService_ExtractToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:  "test-secret",
		CookieName: "auth_token",
	})

	t.Run("should extract token from Authorization header", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer test.jwt.token")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token, err := service.ExtractToken(c)

		assert.NoError(t, err)
		assert.Equal(t, "test.jwt.token", token)
	})

	t.Run("should extract token from cookie", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "auth_token",
			Value: "test.jwt.token",
		})
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token, err := service.ExtractToken(c)

		assert.NoError(t, err)
		assert.Equal(t, "test.jwt.token", token)
	})

	t.Run("should prefer Authorization header over cookie", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer header.token")
		req.AddCookie(&http.Cookie{
			Name:  "auth_token",
			Value: "cookie.token",
		})
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token, err := service.ExtractToken(c)

		assert.NoError(t, err)
		assert.Equal(t, "header.token", token)
	})

	t.Run("should return error when no token provided", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token, err := service.ExtractToken(c)

		assert.Error(t, err)
		assert.Empty(t, token)
		assert.True(t, errors.Is(err, ErrMissingToken))
	})

	t.Run("should return error for malformed Authorization header", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token, err := service.ExtractToken(c)

		assert.Error(t, err)
		assert.Empty(t, token)
	})
}

func TestJWTService_JWTMiddleware(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:  "test-secret-key",
		CookieName: "auth_token",
	})

	t.Run("should allow valid token", func(t *testing.T) {
		token, _ := service.GenerateToken(123, "testuser")

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := service.JWTMiddleware()(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		err := handler(c)

		assert.NoError(t, err)
		assert.Equal(t, 123, c.Get("user_id"))
		assert.Equal(t, "testuser", c.Get("username"))
	})

	t.Run("should reject missing token", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := service.JWTMiddleware()(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		err := handler(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &response)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Missing authentication token", response["message"])
	})

	t.Run("should reject invalid token", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer invalid.token")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := service.JWTMiddleware()(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		err := handler(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &response)
		assert.False(t, response["success"].(bool))
		assert.Equal(t, "Invalid authentication token", response["message"])
	})
}

func TestGetUserIdFromContext(t *testing.T) {
	t.Run("should get user ID from context", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", 123)

		userID, err := GetUserIdFromContext(c)

		assert.NoError(t, err)
		assert.Equal(t, 123, userID)
	})

	t.Run("should return error when user_id not in context", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		userID, err := GetUserIdFromContext(c)

		assert.Error(t, err)
		assert.Equal(t, 0, userID)
	})
}

func TestGetUsernameFromContext(t *testing.T) {
	t.Run("should get username from context", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("username", "testuser")

		username, err := GetUsernameFromContext(c)

		assert.NoError(t, err)
		assert.Equal(t, "testuser", username)
	})

	t.Run("should return error when username not in context", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		username, err := GetUsernameFromContext(c)

		assert.Error(t, err)
		assert.Empty(t, username)
	})
}
