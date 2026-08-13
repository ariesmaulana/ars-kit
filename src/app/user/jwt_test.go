package user

import (
	"context"
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
		token, err := service.GenerateToken(123, "testuser", 0)

		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := service.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, 123, claims.UserId)
		assert.Equal(t, "testuser", claims.Username)
	})

	t.Run("should generate different tokens for different users", func(t *testing.T) {
		token1, err1 := service.GenerateToken(1, "user1", 0)
		token2, err2 := service.GenerateToken(2, "user2", 0)

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
		token, _ := service.GenerateToken(123, "testuser", 0)

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
		token, _ := wrongService.GenerateToken(123, "testuser", 0)

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
		require.Len(t, cookies, 2)

		// Both the auth cookie and the refresh cookie are cleared.
		for _, cookie := range cookies {
			assert.Equal(t, "", cookie.Value)
			assert.Equal(t, -1, cookie.MaxAge)
			assert.True(t, cookie.HttpOnly)
		}
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
		token, _ := service.GenerateToken(123, "testuser", 0)

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

// ──────────────────────────────────────────────────────────────
// A5: jti/iss/aud claims, refresh tokens, token-version revocation
// ──────────────────────────────────────────────────────────────

func TestJWTService_GenerateTokenClaims(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey: "test-secret-key",
		Issuer:    "custom-issuer",
		Audience:  "custom-audience",
	})

	t.Run("should include jti, iss, aud and token_version claims", func(t *testing.T) {
		token, err := service.GenerateToken(123, "testuser", 7)
		require.NoError(t, err)

		parsed, err := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte("test-secret-key"), nil
		})
		require.NoError(t, err)

		claims, ok := parsed.Claims.(*JWTClaims)
		require.True(t, ok)
		require.True(t, parsed.Valid)

		assert.NotEmpty(t, claims.ID, "jti must be set")
		assert.Equal(t, "custom-issuer", claims.Issuer)
		assert.Equal(t, jwt.ClaimStrings{"custom-audience"}, claims.Audience)
		assert.Equal(t, 7, claims.TokenVersion)
		assert.Equal(t, 123, claims.UserId)
		assert.Equal(t, "testuser", claims.Username)
	})

	t.Run("should mint a unique jti for every token", func(t *testing.T) {
		token1, _ := service.GenerateToken(1, "user1", 0)
		token2, _ := service.GenerateToken(1, "user1", 0)

		parse := func(tokenStr string) *JWTClaims {
			parsed, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte("test-secret-key"), nil
			})
			require.NoError(t, err)
			return parsed.Claims.(*JWTClaims)
		}

		assert.NotEqual(t, parse(token1).ID, parse(token2).ID)
	})
}

func TestJWTService_ValidateTokenRejectsWrongIssuerOrAudience(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey: "test-secret-key",
		Issuer:    "expected-issuer",
		Audience:  "expected-audience",
	})
	foreign := NewJWTService(JWTConfig{
		SecretKey: "test-secret-key", // same key, different iss/aud
		Issuer:    "foreign-issuer",
		Audience:  "foreign-audience",
	})

	t.Run("should reject token with wrong issuer", func(t *testing.T) {
		token, err := foreign.GenerateToken(1, "user1", 0)
		require.NoError(t, err)

		claims, err := service.ValidateToken(token)
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("should accept token with matching issuer and audience", func(t *testing.T) {
		token, err := service.GenerateToken(1, "user1", 0)
		require.NoError(t, err)

		claims, err := service.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, 1, claims.UserId)
	})
}

func TestJWTService_GenerateRefreshToken(t *testing.T) {
	service := NewJWTService(JWTConfig{SecretKey: "test-secret"})

	t.Run("should generate unique opaque tokens", func(t *testing.T) {
		token1, err := service.GenerateRefreshToken()
		require.NoError(t, err)
		token2, err := service.GenerateRefreshToken()
		require.NoError(t, err)

		assert.NotEmpty(t, token1)
		assert.NotEmpty(t, token2)
		assert.NotEqual(t, token1, token2)
	})
}

func TestJWTService_HashRefreshToken(t *testing.T) {
	service := NewJWTService(JWTConfig{SecretKey: "test-secret"})

	t.Run("should hash deterministically and differ across tokens", func(t *testing.T) {
		token, _ := service.GenerateRefreshToken()

		assert.Equal(t, service.HashRefreshToken(token), service.HashRefreshToken(token))
		assert.Equal(t, 64, len(service.HashRefreshToken(token))) // SHA-256 hex
		assert.NotEqual(t, service.HashRefreshToken(token), service.HashRefreshToken(token+"x"))
	})
}

func TestJWTService_JWTMiddlewareTokenVersion(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:  "test-secret-key",
		CookieName: "auth_token",
	})

	t.Run("should allow token when version matches the user's current version", func(t *testing.T) {
		service.SetTokenVersionLoader(func(ctx context.Context, userID int) (int, error) {
			return 3, nil
		})
		token, _ := service.GenerateToken(123, "testuser", 3)

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
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 123, c.Get("user_id"))
	})

	t.Run("should reject token minted at a stale version (password change)", func(t *testing.T) {
		service.SetTokenVersionLoader(func(ctx context.Context, userID int) (int, error) {
			return 4, nil // user's password changed since the token was minted
		})
		token, _ := service.GenerateToken(123, "testuser", 3)

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
		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &response)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["message"].(string), "revoked")
	})
}

func TestJWTService_SetRefreshTokenCookie(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:         "test-secret",
		RefreshCookieName: "refresh_token",
		CookieHTTPOnly:    true,
	})

	t.Run("should set refresh cookie with the refresh TTL", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		service.SetRefreshTokenCookie(c, "refresh.jwt.token")

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)

		cookie := cookies[0]
		assert.Equal(t, "refresh_token", cookie.Name)
		assert.Equal(t, "refresh.jwt.token", cookie.Value)
		assert.Equal(t, int(service.RefreshExpiration().Seconds()), cookie.MaxAge)
		assert.True(t, cookie.HttpOnly)
	})

	t.Run("should clear both auth and refresh cookies", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		service.ClearTokenCookie(c)

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 2)
		names := map[string]bool{}
		for _, cookie := range cookies {
			names[cookie.Name] = true
			assert.Equal(t, -1, cookie.MaxAge)
		}
		assert.True(t, names["auth_token"])
		assert.True(t, names["refresh_token"])
	})
}

func TestJWTService_ExtractRefreshToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:         "test-secret",
		RefreshCookieName: "refresh_token",
	})

	t.Run("should read refresh token from cookie", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "refresh.value"})
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token, err := service.ExtractRefreshToken(c)
		assert.NoError(t, err)
		assert.Equal(t, "refresh.value", token)
	})

	t.Run("should error when no refresh cookie", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		token, err := service.ExtractRefreshToken(c)
		assert.Error(t, err)
		assert.Empty(t, token)
	})
}
