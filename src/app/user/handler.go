package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

// Handler handles HTTP requests for user operations
type Handler struct {
	service    Service
	jwtService *JWTService
}

// statusForError maps an operation ErrorCode to an HTTP status. Client errors
// default to 400; only real system failures map to 500. 401/403 stay distinct
// because end users rely on them; 429 signals a throttled/locked account.
func statusForError(code ErrorCode) int {
	switch code {
	case ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrorCodeForbidden:
		return http.StatusForbidden
	case ErrorCodeLocked:
		return http.StatusTooManyRequests
	case ErrorCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// bindJSON binds the JSON request body into dst, tolerating curl bodies that
// got line-wrapped when pasted. A raw CR/LF is only legal in JSON as
// whitespace between tokens, so stripping it never breaks valid JSON — it only
// repairs the common wrapped-string mistake, e.g.
//
//	"full_name":"Jane\nWorkflow" → "full_name":"Jane Workflow"
func bindJSON(c echo.Context, dst any) error {
	// Reject non-JSON payloads, mirroring echo's default binder.
	if !strings.Contains(c.Request().Header.Get(echo.HeaderContentType), "application/json") {
		return fmt.Errorf("unsupported media type %q", c.Request().Header.Get(echo.HeaderContentType))
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	body = bytes.ReplaceAll(body, []byte{'\r'}, nil)
	body = bytes.ReplaceAll(body, []byte{'\n'}, nil)
	return json.Unmarshal(body, dst)
}

// NewHandler creates a new user handler
func NewHandler(service Service, jwtService *JWTService) *Handler {
	return &Handler{
		service:    service,
		jwtService: jwtService,
	}
}

// RegisterRoutes registers all user routes under the provided version group.
// Example: pass e.Group("/api/v1") from main — routes become /api/v1/users/...
func (h *Handler) RegisterRoutes(g *echo.Group) {
	users := g.Group("/users")

	// Stricter rate limiting on auth endpoints: 2 req/s per IP, burst of 5
	authLimiter := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      2,
				Burst:     5,
				ExpiresIn: 5 * time.Minute,
			},
		),
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
				"success": false,
				"message": "Too many requests, please try again later",
			})
		},
	})

	// Public routes
	public := users.Group("", authLimiter)
	public.POST("/register", h.Register)
	public.POST("/register-workflow", h.RegisterWorkflow)
	public.POST("/login", h.Login)
	public.POST("/refresh", h.Refresh)
	public.POST("/logout", h.Logout)

	// Protected routes
	protected := users.Group("")
	protected.Use(h.jwtService.JWTMiddleware())
	protected.GET("/profile", h.Profile)
	protected.PUT("/profile/username", h.UpdateUsername)
	protected.PUT("/profile/password", h.UpdatePassword)
	protected.POST("/permissions/grant", h.GrantPermission)
	protected.POST("/permissions/revoke", h.RevokePermission)
	protected.GET("/audit-logs", h.ListAuditLogs)
}

// HTTP Request/Response structs

// RegisterRequest represents the HTTP request body for user registration
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	FullName string `json:"full_name" validate:"required"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginRequest represents the HTTP request body for user login
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest represents the HTTP request body for rotating a refresh
// token. The field is optional: when omitted the refresh cookie is used.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// UpdateUsernameRequest represents the HTTP request body for updating username
type UpdateUsernameRequest struct {
	NewUsername string `json:"new_username" validate:"required,min=3,max=50"`
}

// UpdatePasswordRequest represents the HTTP request body for updating password
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// UserDTO represents a user in HTTP responses (without password)
type UserDTO struct {
	Id        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PaginationResponse holds pagination metadata for list responses
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// AuthResponse represents the HTTP response for authentication operations
// (register, login, refresh).
//
// LockedUntil and RetryAfterSeconds are populated when the account is locked
// so clients can show when it unlocks instead of guessing from the message.
// AccessToken is a short-lived JWT; RefreshToken is an opaque token that can
// be exchanged once at POST /users/refresh.
type AuthResponse struct {
	Success           bool       `json:"success"`
	Message           string     `json:"message"`
	Token             string     `json:"token,omitempty"`
	RefreshToken      string     `json:"refresh_token,omitempty"`
	User              *UserDTO   `json:"user,omitempty"`
	LockedUntil       *time.Time `json:"locked_until,omitempty"`
	RetryAfterSeconds int        `json:"retry_after_seconds,omitempty"`
}

// UserResponse represents the HTTP response for user operations
type UserResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Data    *UserDTO `json:"data,omitempty"`
}

func toUserDTO(user User) UserDTO {
	return UserDTO{
		Id:        user.Id,
		Username:  user.Username,
		Email:     user.Email,
		FullName:  user.FullName,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// Register handles POST /api/v1/users/register
// @Summary Register a new user
// @Description Create a new user account
// @Tags users
// @Accept json
// @Produce json
// @Param user body RegisterRequest true "User registration data"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/v1/users/register [post]
func (h *Handler) Register(c echo.Context) error {
	traceID := xid.New().String()

	var req RegisterRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.Register(c.Request().Context(), &RegisterInput{
		TraceId:  traceID,
		Username: req.Username,
		Email:    req.Email,
		FullName: req.FullName,
		Password: req.Password,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), AuthResponse{
			Success: false,
			Message: output.Message,
		})
	}

	// Set tokens in cookies (access + refresh)
	h.jwtService.SetTokenCookie(c, output.AccessToken)
	h.jwtService.SetRefreshTokenCookie(c, output.RefreshToken)

	dto := toUserDTO(output.User)
	return c.JSON(http.StatusCreated, AuthResponse{
		Success:      true,
		Message:      "User registered successfully",
		Token:        output.AccessToken,
		RefreshToken: output.RefreshToken,
		User:         &dto,
	})
}

// RegisterWorkflow handles POST /api/v1/users/register-workflow
// @Summary Register a user asynchronously via the workflow engine
// @Description Validate the input and enqueue a register_user workflow job.
// @Description The user is created and granted its permission by background
// @Description workers instead of synchronously in the request.
// @Tags users
// @Accept json
// @Produce json
// @Param user body RegisterRequest true "User registration data"
// @Success 202 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/v1/users/register-workflow [post]
func (h *Handler) RegisterWorkflow(c echo.Context) error {
	traceID := xid.New().String()

	var req RegisterRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.DemoWorkflow(c.Request().Context(), &DemoWorkflowInput{
		TraceId:  traceID,
		Email:    req.Email,
		Username: req.Username,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), AuthResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusAccepted, AuthResponse{
		Success: true,
		Message: "Demo workflow queued",
	})
}

// Login handles POST /api/v1/users/login
// @Summary Login user
// @Description Authenticate user and return JWT token
// @Tags users
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "User credentials"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/v1/users/login [post]
func (h *Handler) Login(c echo.Context) error {
	traceID := xid.New().String()

	var req LoginRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.Login(c.Request().Context(), &LoginInput{
		TraceId:  traceID,
		Username: req.Username,
		Password: req.Password,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), AuthResponse{
			Success:           false,
			Message:           output.Message,
			LockedUntil:       output.LockedUntil,
			RetryAfterSeconds: output.RetryAfterSeconds,
		})
	}

	// Set tokens in cookies (access + refresh)
	h.jwtService.SetTokenCookie(c, output.AccessToken)
	h.jwtService.SetRefreshTokenCookie(c, output.RefreshToken)

	dto := toUserDTO(output.User)
	return c.JSON(http.StatusOK, AuthResponse{
		Success:      true,
		Message:      "Login successful",
		Token:        output.AccessToken,
		RefreshToken: output.RefreshToken,
		User:         &dto,
	})
}

// Refresh handles POST /api/v1/users/refresh
// @Summary Rotate refresh token
// @Description Exchange a refresh token for a fresh access + refresh pair.
// @Description The presented refresh token is revoked server-side (rotation),
// @Description so each refresh token can be used exactly once.
// @Tags users
// @Accept json
// @Produce json
// @Param credentials body RefreshRequest true "Refresh token (optional if refresh cookie is sent)"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} AuthResponse
// @Failure 401 {object} AuthResponse
// @Failure 500 {object} AuthResponse
// @Router /api/v1/users/refresh [post]
func (h *Handler) Refresh(c echo.Context) error {
	traceID := xid.New().String()

	var req RefreshRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// The body wins when present; otherwise fall back to the refresh cookie.
	refreshToken := req.RefreshToken
	if refreshToken == "" {
		refreshToken, _ = h.jwtService.ExtractRefreshToken(c)
	}

	output := h.service.Refresh(c.Request().Context(), &RefreshInput{
		TraceId:      traceID,
		RefreshToken: refreshToken,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), AuthResponse{
			Success: false,
			Message: output.Message,
		})
	}

	// Set tokens in cookies (access + refresh)
	h.jwtService.SetTokenCookie(c, output.AccessToken)
	h.jwtService.SetRefreshTokenCookie(c, output.RefreshToken)

	dto := toUserDTO(output.User)
	return c.JSON(http.StatusOK, AuthResponse{
		Success:      true,
		Message:      "Session refreshed",
		Token:        output.AccessToken,
		RefreshToken: output.RefreshToken,
		User:         &dto,
	})
}

// Logout handles POST /api/v1/users/logout
// @Summary Logout user
// @Description Revoke the refresh token server-side and clear authentication
// @Description cookies. The refresh token is read from the request body or the
// @Description refresh cookie.
// @Tags users
// @Accept json
// @Produce json
// @Param credentials body RefreshRequest false "Refresh token (optional if refresh cookie is sent)"
// @Success 200 {object} UserResponse
// @Router /api/v1/users/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	traceID := xid.New().String()

	var req RefreshRequest
	_ = bindJSON(c, &req)

	refreshToken := req.RefreshToken
	if refreshToken == "" {
		refreshToken, _ = h.jwtService.ExtractRefreshToken(c)
	}

	// Revoke server-side when a refresh token is presented; revoking is
	// idempotent, so a missing/expired token still ends the client session.
	if refreshToken != "" {
		output := h.service.Logout(c.Request().Context(), &LogoutInput{
			TraceId:      traceID,
			RefreshToken: refreshToken,
		})
		if !output.Success {
			return c.JSON(statusForError(output.ErrorCode), UserResponse{
				Success: false,
				Message: output.Message,
			})
		}
	}

	h.jwtService.ClearTokenCookie(c)

	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}

// Profile handles GET /api/v1/users/profile
// @Summary Get user profile
// @Description Get the authenticated user's profile
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 500 {object} UserResponse
// @Router /api/v1/users/profile [get]
func (h *Handler) Profile(c echo.Context) error {
	traceID := xid.New().String()

	userID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	output := h.service.GetProfileById(c.Request().Context(), &GetProfileByIdInput{
		TraceId: traceID,
		Id:      userID,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	dto := toUserDTO(output.User)
	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: output.Message,
		Data:    &dto,
	})
}

// UpdateUsername handles PUT /api/v1/users/profile/username
// @Summary Update username
// @Description Update the authenticated user's username
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username body UpdateUsernameRequest true "New username"
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 403 {object} UserResponse
// @Failure 500 {object} UserResponse
// @Router /api/v1/users/profile/username [put]
func (h *Handler) UpdateUsername(c echo.Context) error {
	traceID := xid.New().String()

	userID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req UpdateUsernameRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.UpdateUsername(c.Request().Context(), &UpdateUsernameInput{
		TraceId:     traceID,
		Id:          userID,
		NewUsername: req.NewUsername,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	dto := toUserDTO(output.User)
	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: "Username updated successfully",
		Data:    &dto,
	})
}

// UpdatePassword handles PUT /api/v1/users/profile/password
// @Summary Update password
// @Description Update the authenticated user's password
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param password body UpdatePasswordRequest true "Password update data"
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 403 {object} UserResponse
// @Failure 500 {object} UserResponse
// @Router /api/v1/users/profile/password [put]
func (h *Handler) UpdatePassword(c echo.Context) error {
	traceID := xid.New().String()

	userID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req UpdatePasswordRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.UpdatePassword(c.Request().Context(), &UpdatePasswordInput{
		TraceId:     traceID,
		Id:          userID,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: "Password updated successfully",
	})
}

// ManagePermissionRequest represents the HTTP request body for granting/revoking a permission
type ManagePermissionRequest struct {
	TargetUserId int    `json:"user_id" validate:"required"`
	Permission   string `json:"permission" validate:"required"`
}

// ManagePermissionResponse represents the HTTP response for granting/revoking a permission
type ManagePermissionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// AuditLogDTO represents an audit log entry in HTTP responses.
type AuditLogDTO struct {
	Id           int64          `json:"id"`
	Event        string         `json:"event"`
	ActorId      *int           `json:"actor_id"`
	TargetUserId *int           `json:"target_user_id"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

// AuditLogListResponse represents the HTTP response for the audit log list endpoint.
type AuditLogListResponse struct {
	Success    bool                `json:"success"`
	Message    string              `json:"message"`
	Data       []AuditLogDTO       `json:"data,omitempty"`
	Pagination *PaginationResponse `json:"pagination,omitempty"`
}

func toAuditLogDTO(entry AuditEntry) AuditLogDTO {
	return AuditLogDTO{
		Id:           entry.Id,
		Event:        entry.Event,
		ActorId:      entry.ActorId,
		TargetUserId: entry.TargetUserId,
		Metadata:     entry.Metadata,
		CreatedAt:    entry.CreatedAt,
	}
}

// GrantPermission handles POST /api/v1/users/permissions/grant
// @Summary Grant permission
// @Description Grant a permission string to a target user. Only super user may do this.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param permission body ManagePermissionRequest true "Permission grant data"
// @Success 200 {object} ManagePermissionResponse
// @Failure 400 {object} ManagePermissionResponse
// @Failure 401 {object} ManagePermissionResponse
// @Failure 403 {object} ManagePermissionResponse
// @Failure 500 {object} ManagePermissionResponse
// @Router /api/v1/users/permissions/grant [post]
func (h *Handler) GrantPermission(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ManagePermissionResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req ManagePermissionRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, ManagePermissionResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.GrantPermission(c.Request().Context(), &GrantPermissionInput{
		TraceId:      traceID,
		ActorId:      actorID,
		TargetUserId: req.TargetUserId,
		Permission:   req.Permission,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), ManagePermissionResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, ManagePermissionResponse{
		Success: true,
		Message: output.Message,
	})
}

// RevokePermission handles POST /api/v1/users/permissions/revoke
// @Summary Revoke permission
// @Description Revoke a permission from a target user. Only superuser may do this.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ManagePermissionRequest true "Permission revoke data"
// @Success 200 {object} ManagePermissionResponse
// @Failure 400 {object} ManagePermissionResponse
// @Failure 401 {object} ManagePermissionResponse
// @Failure 403 {object} ManagePermissionResponse
// @Failure 500 {object} ManagePermissionResponse
// @Router /api/v1/users/permissions/revoke [post]
func (h *Handler) RevokePermission(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ManagePermissionResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req ManagePermissionRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, ManagePermissionResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.RevokePermission(c.Request().Context(), &RevokePermissionInput{
		TraceId:      traceID,
		ActorId:      actorID,
		TargetUserId: req.TargetUserId,
		Permission:   req.Permission,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), ManagePermissionResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, ManagePermissionResponse{
		Success: true,
		Message: output.Message,
	})
}

// ListAuditLogs handles GET /api/v1/users/audit-logs
// @Summary List audit logs
// @Description Return a page of audit log entries, newest first. Only a super user may read the log.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (1-based), default 1"
// @Param page_size query int false "Page size (1-100), default 20"
// @Param event query string false "Filter by event (grant|revoke|password|username|email|login)"
// @Param actor_id query int false "Filter by the user who performed the action"
// @Param target_user_id query int false "Filter by the user the action affected"
// @Success 200 {object} AuditLogListResponse
// @Failure 400 {object} AuditLogListResponse
// @Failure 401 {object} AuditLogListResponse
// @Failure 403 {object} AuditLogListResponse
// @Failure 500 {object} AuditLogListResponse
// @Router /api/v1/users/audit-logs [get]
func (h *Handler) ListAuditLogs(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, AuditLogListResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	page := 1
	pageSize := 20
	if v, err := strconv.Atoi(c.QueryParam("page")); err == nil && v >= 1 {
		page = v
	}
	if v, err := strconv.Atoi(c.QueryParam("page_size")); err == nil && v >= 1 {
		pageSize = v
	}

	output := h.service.ListAuditLogs(c.Request().Context(), &ListAuditLogsInput{
		TraceId:        traceID,
		ActorId:        actorID,
		Event:          c.QueryParam("event"),
		FilterActorId:  parseIntQuery(c.QueryParam("actor_id")),
		FilterTargetId: parseIntQuery(c.QueryParam("target_user_id")),
		Page:           page,
		PageSize:       pageSize,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), AuditLogListResponse{
			Success: false,
			Message: output.Message,
		})
	}

	data := make([]AuditLogDTO, 0, len(output.Entries))
	for _, entry := range output.Entries {
		data = append(data, toAuditLogDTO(entry))
	}

	totalPages := 0
	if output.PageSize > 0 {
		totalPages = (output.Total + output.PageSize - 1) / output.PageSize
	}

	return c.JSON(http.StatusOK, AuditLogListResponse{
		Success: true,
		Message: output.Message,
		Data:    data,
		Pagination: &PaginationResponse{
			Page:       output.Page,
			PageSize:   output.PageSize,
			Total:      output.Total,
			TotalPages: totalPages,
		},
	})
}

// parseIntQuery parses an optional integer query parameter, returning 0 when
// the parameter is absent or not a number.
func parseIntQuery(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
