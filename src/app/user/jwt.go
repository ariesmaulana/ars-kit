package user

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

var (
	ErrMissingToken   = errors.New("missing authentication token")
	ErrInvalidToken   = errors.New("invalid authentication token")
	ErrExpiredToken   = errors.New("token has expired")
)

// JWTClaims represents the JWT claims
type JWTClaims struct {
	UserId   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey       string
	ExpirationHours int
	CookieName      string
	CookieDomain    string
	CookieSecure    bool
	CookieHTTPOnly  bool
}

// JWTService handles JWT operations
type JWTService struct {
	config JWTConfig
}

// NewJWTService creates a new JWT service
func NewJWTService(config JWTConfig) *JWTService {
	if config.ExpirationHours == 0 {
		config.ExpirationHours = 24 // default 24 hours
	}
	if config.CookieName == "" {
		config.CookieName = "auth_token"
	}
	return &JWTService{
		config: config,
	}
}

// GenerateToken generates a new JWT token
func (j *JWTService) GenerateToken(userId int, username string) (string, error) {
	claims := JWTClaims{
		UserId:   userId,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(j.config.ExpirationHours))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.config.SecretKey))
}

// ValidateToken validates a JWT token and returns the claims
func (j *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(j.config.SecretKey), nil
	})

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

// ClearTokenCookie clears the authentication cookie
func (j *JWTService) ClearTokenCookie(c echo.Context) {
	cookie := &http.Cookie{
		Name:     j.config.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: j.config.CookieHTTPOnly,
	}
	c.SetCookie(cookie)
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
