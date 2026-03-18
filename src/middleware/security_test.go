package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appmw "github.com/ariesmaulana/ars-kit/src/middleware"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	e := echo.New()
	e.Use(appmw.SecurityHeaders())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	assert.Equal(t, "max-age=63072000; includeSubDomains; preload", rec.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "geolocation=(), microphone=(), camera=()", rec.Header().Get("Permissions-Policy"))
	assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", rec.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeaders_AppliedOnAllStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(appmw.SecurityHeaders())
			e.GET("/", func(c echo.Context) error {
				return c.JSON(tc.statusCode, map[string]bool{"success": false})
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tc.statusCode, rec.Code)
			assert.NotEmpty(t, rec.Header().Get("X-Content-Type-Options"), "header missing on %d", tc.statusCode)
			assert.NotEmpty(t, rec.Header().Get("X-Frame-Options"), "header missing on %d", tc.statusCode)
			assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"), "header missing on %d", tc.statusCode)
			assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"), "header missing on %d", tc.statusCode)
		})
	}
}

func TestSecurityHeaders_NextHandlerIsCalled(t *testing.T) {
	called := false

	e := echo.New()
	e.Use(appmw.SecurityHeaders())
	e.GET("/", func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.True(t, called, "next handler should have been called")
}
