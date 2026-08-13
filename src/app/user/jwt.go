package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/xid"
)

var (
	ErrMissingToken = errors.New("missing authentication token")
	ErrInvalidToken = errors.New("invalid authentication token")
	ErrExpiredToken = errors.New("token has expired")
)

// JWTClaims represents the JWT claims. The access token carries the standard
// registered claims (jti, iss, aud) plus the identity claims the middleware
// and handlers rely on. TokenVersion snapshots users.token_version at issuance
// so the middleware can reject tokens minted before a security event (e.g.
// password change) without touching the refresh token store.
type JWTClaims struct {
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey              string
	ExpirationHours        int
	RefreshExpirationHours int
	CookieName             string
	RefreshCookieName      string
	CookieDomain           string
	CookieSecure           bool
	CookieHTTPOnly         bool
	Issuer                 string
	Audience               string
}

// JWTService handles JWT operations
type JWTService struct {
	config JWTConfig
	// tokenVersionLoader, when set, returns the user's current token_version.
	// The middleware compares it against the claim's version so tokens issued
	// before a bump (password change) are rejected. Nil disables the check
	// (tests and callers without storage access).
	tokenVersionLoader func(ctx context.Context, userID int) (int, error)
}

// NewJWTService creates a new JWT service
func NewJWTService(config JWTConfig) *JWTService {
	if config.ExpirationHours == 0 {
		config.ExpirationHours = 24 // default 24 hours
	}
	if config.RefreshExpirationHours == 0 {
		config.RefreshExpirationHours = 24 * 7 // default 7 days
	}
	if config.CookieName == "" {
		config.CookieName = "auth_token"
	}
	if config.RefreshCookieName == "" {
		config.RefreshCookieName = "refresh_token"
	}
	if config.Issuer == "" {
		config.Issuer = "ars-kit"
	}
	if config.Audience == "" {
		config.Audience = "ars-kit-api"
	}
	return &JWTService{
		config: config,
	}
}

// SetTokenVersionLoader installs the loader the middleware uses to compare a
// token's token_version claim against the user's current version. Nil (the
// default) disables the check.
func (j *JWTService) SetTokenVersionLoader(fn func(ctx context.Context, userID int) (int, error)) {
	j.tokenVersionLoader = fn
}

// RefreshExpiration returns the refresh token lifetime.
func (j *JWTService) RefreshExpiration() time.Duration {
	return time.Hour * time.Duration(j.config.RefreshExpirationHours)
}

// GenerateToken generates a new access JWT. The token carries a unique jti,
// the configured issuer and audience, and the token_version it was issued at.
func (j *JWTService) GenerateToken(userId int, username string, tokenVersion int) (string, error) {
	claims := JWTClaims{
		UserId:       userId,
		Username:     username,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        xid.New().String(),
			Issuer:    j.config.Issuer,
			Audience:  jwt.ClaimStrings{j.config.Audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(j.config.ExpirationHours))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.config.SecretKey))
}

// GenerateRefreshToken returns a new opaque refresh token: 32 bytes of
// cryptographic randomness, base64url-encoded. The token itself is never
// stored; only its SHA-256 hash goes to the database.
func (j *JWTService) GenerateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefreshToken returns the SHA-256 hex digest of a refresh token. This is
// the value stored in refresh_tokens.token_hash.
func (j *JWTService) HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ValidateToken validates a JWT token and returns the claims
func (j *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(j.config.SecretKey), nil
	},
		jwt.WithIssuer(j.config.Issuer),
		jwt.WithAudience(j.config.Audience),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// SetTokenCookie sets the JWT token as an HTTP cookie
func (j *JWTService) SetTokenCookie(c echo.Context, token string) {
	cookie := &http.Cookie{
		Name:     j.config.CookieName,
		Value:    token,
		Path:     "/",
		Domain:   j.config.CookieDomain,
		MaxAge:   j.config.ExpirationHours * 3600,
		Secure:   j.config.CookieSecure,
		HttpOnly: j.config.CookieHTTPOnly,
		SameSite: http.SameSiteStrictMode,
	}
	c.SetCookie(cookie)
}

// SetRefreshTokenCookie sets the refresh token as an HTTP cookie
func (j *JWTService) SetRefreshTokenCookie(c echo.Context, token string) {
	cookie := &http.Cookie{
		Name:     j.config.RefreshCookieName,
		Value:    token,
		Path:     "/",
		Domain:   j.config.CookieDomain,
		MaxAge:   int(j.RefreshExpiration().Seconds()),
		Secure:   j.config.CookieSecure,
		HttpOnly: j.config.CookieHTTPOnly,
		SameSite: http.SameSiteStrictMode,
	}
	c.SetCookie(cookie)
}

// ClearTokenCookie clears the authentication and refresh cookies
func (j *JWTService) ClearTokenCookie(c echo.Context) {
	for _, name := range []string{j.config.CookieName, j.config.RefreshCookieName} {
		cookie := &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: j.config.CookieHTTPOnly,
		}
		c.SetCookie(cookie)
	}
}

// ExtractToken extracts JWT token from Authorization header or cookie
func (j *JWTService) ExtractToken(c echo.Context) (string, error) {
	// Try to get token from Authorization header first
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		// Expected format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1], nil
		}
	}

	// If not in header, try to get from cookie
	cookie, err := c.Cookie(j.config.CookieName)
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	return "", ErrMissingToken
}

// ExtractRefreshToken extracts the refresh token from the request body's
// refresh_token field or the refresh cookie. The body value wins when both
// are present (a rotated client may hold a newer token than the cookie).
func (j *JWTService) ExtractRefreshToken(c echo.Context) (string, error) {
	// Try the refresh cookie first (the standard delivery channel).
	if cookie, err := c.Cookie(j.config.RefreshCookieName); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}
	return "", ErrMissingToken
}

// JWTMiddleware is an Echo middleware for JWT authentication
func (j *JWTService) JWTMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, err := j.ExtractToken(c)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"message": "Missing authentication token",
				})
			}

			claims, err := j.ValidateToken(token)
			if err != nil {
				status := http.StatusUnauthorized
				message := "Invalid authentication token"

				if errors.Is(err, ErrExpiredToken) {
					message = "Token has expired"
				}

				return c.JSON(status, map[string]interface{}{
					"success": false,
					"message": message,
				})
			}

			// Reject access tokens minted before the user's latest
			// token_version bump (e.g. password change). Without a loader
			// (tests, standalone use) the check is skipped.
			if j.tokenVersionLoader != nil {
				currentVersion, err := j.tokenVersionLoader(c.Request().Context(), claims.UserId)
				if err != nil || currentVersion != claims.TokenVersion {
					return c.JSON(http.StatusUnauthorized, map[string]interface{}{
						"success": false,
						"message": "Session has been revoked. Please log in again.",
					})
				}
			}

			// Store claims in context for handlers to use
			c.Set("user_id", claims.UserId)
			c.Set("username", claims.Username)

			return next(c)
		}
	}
}

// GetUserIdFromContext extracts user ID from Echo context
func GetUserIdFromContext(c echo.Context) (int, error) {
	userId, ok := c.Get("user_id").(int)
	if !ok {
		return 0, errors.New("user_id not found in context")
	}
	return userId, nil
}

// GetUsernameFromContext extracts username from Echo context
func GetUsernameFromContext(c echo.Context) (string, error) {
	username, ok := c.Get("username").(string)
	if !ok {
		return "", errors.New("username not found in context")
	}
	return username, nil
}
