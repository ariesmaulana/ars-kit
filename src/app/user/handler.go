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
	public.POST("/forgot-password", h.ForgotPassword)
	public.POST("/reset-password", h.ResetPassword)
	public.POST("/send-verification", h.SendVerificationEmail)
	public.POST("/verify-email", h.VerifyEmail)

	// Protected routes
	protected := users.Group("")
	protected.Use(h.jwtService.JWTMiddleware())
	protected.GET("/profile", h.Profile)
	protected.PUT("/profile/username", h.UpdateUsername)
	protected.PUT("/profile/password", h.UpdatePassword)
	protected.POST("/roles/assign", h.AssignRole)
	protected.POST("/roles/unassign", h.UnassignRole)
	protected.POST("/roles/permissions/grant", h.AssignPermissionToRole)
	protected.POST("/roles/permissions/revoke", h.RemovePermissionFromRole)

	// Admin routes (super_user-gated inside the service)
	protected.GET("", h.ListUsers)
	protected.GET("/:id", h.GetUser)
	protected.PUT("/:id/status", h.UpdateUserStatus)
	protected.DELETE("/:id", h.DeleteUser)
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

// ForgotPasswordRequest represents the HTTP request body for forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents the HTTP request body for resetting a
// password using the token emailed by forgot-password.
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// SendVerificationRequest represents the HTTP request body for requesting a
// verification email.
type SendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// VerifyEmailRequest represents the HTTP request body for verifying an email
// using the token emailed after sign-up.
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

// MessageResponse is a success/message envelope for actions that return no
// data (forgot-password, reset-password, email verification).
type MessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
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

// UpdateUserStatusRequest represents the HTTP request body for setting a
// user's status. Values: "active", "disabled", "suspended".
type UpdateUserStatusRequest struct {
	Status string `json:"status" validate:"required"`
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

// ForgotPassword handles POST /api/v1/users/forgot-password
func (h *Handler) ForgotPassword(c echo.Context) error {
	traceID := xid.New().String()

	var req ForgotPasswordRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.ForgotPassword(c.Request().Context(), &ForgotPasswordInput{
		TraceId: traceID,
		Email:   req.Email,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), MessageResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Success: true,
		Message: output.Message,
	})
}

// ResetPassword handles POST /api/v1/users/reset-password
func (h *Handler) ResetPassword(c echo.Context) error {
	traceID := xid.New().String()

	var req ResetPasswordRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.ResetPassword(c.Request().Context(), &ResetPasswordInput{
		TraceId:     traceID,
		Token:       req.Token,
		NewPassword: req.NewPassword,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), MessageResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Success: true,
		Message: output.Message,
	})
}

// SendVerificationEmail handles POST /api/v1/users/send-verification
func (h *Handler) SendVerificationEmail(c echo.Context) error {
	traceID := xid.New().String()

	var req SendVerificationRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.SendVerificationEmail(c.Request().Context(), &SendVerificationEmailInput{
		TraceId: traceID,
		Email:   req.Email,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), MessageResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Success: true,
		Message: output.Message,
	})
}

// VerifyEmail handles POST /api/v1/users/verify-email
func (h *Handler) VerifyEmail(c echo.Context) error {
	traceID := xid.New().String()

	var req VerifyEmailRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.VerifyEmail(c.Request().Context(), &VerifyEmailInput{
		TraceId: traceID,
		Token:   req.Token,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), MessageResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, MessageResponse{
		Success: true,
		Message: output.Message,
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

// ManageRoleRequest represents the HTTP request body for assigning/unassigning a role
type ManageRoleRequest struct {
	TargetUserId int    `json:"user_id" validate:"required"`
	RoleName     string `json:"role" validate:"required"`
}

// ManageRoleResponse represents the HTTP response for assigning/unassigning a role
type ManageRoleResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// AssignRole handles POST /api/v1/users/roles/assign
// @Summary Assign role
// @Description Assign a role to a target user. Only super user may do this.
// @Description Bootstrap-only roles (super_user) are refused.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param role body ManageRoleRequest true "Role assignment data"
// @Success 200 {object} ManageRoleResponse
// @Failure 400 {object} ManageRoleResponse
// @Failure 401 {object} ManageRoleResponse
// @Failure 403 {object} ManageRoleResponse
// @Failure 500 {object} ManageRoleResponse
// @Router /api/v1/users/roles/assign [post]
func (h *Handler) AssignRole(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ManageRoleResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req ManageRoleRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, ManageRoleResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.AssignRole(c.Request().Context(), &AssignRoleInput{
		TraceId:      traceID,
		ActorId:      actorID,
		TargetUserId: req.TargetUserId,
		RoleName:     req.RoleName,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), ManageRoleResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, ManageRoleResponse{
		Success: true,
		Message: output.Message,
	})
}

// UnassignRole handles POST /api/v1/users/roles/unassign
// @Summary Unassign role
// @Description Remove a role from a target user. Only super user may do this.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ManageRoleRequest true "Role removal data"
// @Success 200 {object} ManageRoleResponse
// @Failure 400 {object} ManageRoleResponse
// @Failure 401 {object} ManageRoleResponse
// @Failure 403 {object} ManageRoleResponse
// @Failure 500 {object} ManageRoleResponse
// @Router /api/v1/users/roles/unassign [post]
func (h *Handler) UnassignRole(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ManageRoleResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req ManageRoleRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, ManageRoleResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.UnassignRole(c.Request().Context(), &UnassignRoleInput{
		TraceId:      traceID,
		ActorId:      actorID,
		TargetUserId: req.TargetUserId,
		RoleName:     req.RoleName,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), ManageRoleResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, ManageRoleResponse{
		Success: true,
		Message: output.Message,
	})
}

// ManageRolePermissionRequest represents the HTTP request body for
// granting/revoking a permission on a role.
type ManageRolePermissionRequest struct {
	RoleName   string `json:"role" validate:"required"`
	Permission string `json:"permission" validate:"required"`
}

// AssignPermissionToRole handles POST /api/v1/users/roles/permissions/grant
// @Summary Grant a permission to a role
// @Description Add a catalog-registered permission to a role. Only super user
// @Description may do this; the super_user role cannot be modified.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ManageRolePermissionRequest true "Role permission grant data"
// @Success 200 {object} ManageRoleResponse
// @Failure 400 {object} ManageRoleResponse
// @Failure 401 {object} ManageRoleResponse
// @Failure 403 {object} ManageRoleResponse
// @Failure 500 {object} ManageRoleResponse
// @Router /api/v1/users/roles/permissions/grant [post]
func (h *Handler) AssignPermissionToRole(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ManageRoleResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req ManageRolePermissionRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, ManageRoleResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.AssignPermissionToRole(c.Request().Context(), &AssignPermissionToRoleInput{
		TraceId:    traceID,
		ActorId:    actorID,
		RoleName:   req.RoleName,
		Permission: req.Permission,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), ManageRoleResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, ManageRoleResponse{
		Success: true,
		Message: output.Message,
	})
}

// RemovePermissionFromRole handles POST /api/v1/users/roles/permissions/revoke
// @Summary Revoke a permission from a role
// @Description Remove a permission from a role. Only super user may do this;
// @Description the super_user role cannot be modified.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ManageRolePermissionRequest true "Role permission revoke data"
// @Success 200 {object} ManageRoleResponse
// @Failure 400 {object} ManageRoleResponse
// @Failure 401 {object} ManageRoleResponse
// @Failure 403 {object} ManageRoleResponse
// @Failure 500 {object} ManageRoleResponse
// @Router /api/v1/users/roles/permissions/revoke [post]
func (h *Handler) RemovePermissionFromRole(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ManageRoleResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req ManageRolePermissionRequest
	if err := bindJSON(c, &req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, ManageRoleResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.RemovePermissionFromRole(c.Request().Context(), &RemovePermissionFromRoleInput{
		TraceId:    traceID,
		ActorId:    actorID,
		RoleName:   req.RoleName,
		Permission: req.Permission,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), ManageRoleResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, ManageRoleResponse{
		Success: true,
		Message: output.Message,
	})
}

// UserListResponse is the HTTP response for listing users.
type UserListResponse struct {
	Success    bool               `json:"success"`
	Message    string             `json:"message"`
	Data       []UserDTO          `json:"data,omitempty"`
	Pagination PaginationResponse `json:"pagination,omitempty"`
}

// UpdateUserStatus handles PUT /api/v1/users/:id/status
// @Summary Update a user's status
// @Description Set a user's status to active, disabled, or suspended. Requires
// @Description the super_user permission. Disabling or suspending also revokes
// @Description all the target's active refresh tokens.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param body body UpdateUserStatusRequest true "New status"
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 403 {object} UserResponse
// @Failure 404 {object} UserResponse
// @Failure 500 {object} UserResponse
// @Router /api/v1/users/{id}/status [put]
func (h *Handler) UpdateUserStatus(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid user id",
		})
	}

	req := new(UpdateUserStatusRequest)
	if err := bindJSON(c, req); err != nil {
		log.Err(err).Str("path", c.Path()).Msg("failed to bind JSON request body")
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}
	if req.Status == "" {
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Status is mandatory",
		})
	}

	output := h.service.UpdateUserStatus(c.Request().Context(), &UpdateUserStatusInput{
		TraceId:      traceID,
		ActorId:      actorID,
		TargetUserId: id,
		Status:       UserStatus(req.Status),
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: output.Message,
	})
}

// ListUsers handles GET /api/v1/users
// @Summary List users
// @Description List users with pagination and optional username/email filter
// @Description and account-status filter. Requires the super_user permission.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page (1-based, default 1)"
// @Param size query int false "Page size (default 10)"
// @Param q query string false "Filter by username or email substring"
// @Param status query string false "Filter by status: active, disabled, suspended"
// @Success 200 {object} UserListResponse
// @Failure 401 {object} UserListResponse
// @Failure 403 {object} UserListResponse
// @Failure 500 {object} UserListResponse
// @Router /api/v1/users [get]
func (h *Handler) ListUsers(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserListResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	size, _ := strconv.Atoi(c.QueryParam("size"))
	filter := c.QueryParam("q")
	status := c.QueryParam("status")

	output := h.service.ListUsers(c.Request().Context(), &ListUsersInput{
		TraceId: traceID,
		ActorId: actorID,
		Page:    page,
		Size:    size,
		Filter:  filter,
		Status:  UserStatus(status),
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), UserListResponse{
			Success: false,
			Message: output.Message,
		})
	}

	users := make([]UserDTO, 0, len(output.Users))
	for _, u := range output.Users {
		users = append(users, toUserDTO(u))
	}

	return c.JSON(http.StatusOK, UserListResponse{
		Success:    true,
		Message:    output.Message,
		Data:       users,
		Pagination: PaginationResponse{Page: output.Page, PageSize: output.Size, Total: output.Total},
	})
}

// GetUser handles GET /api/v1/users/:id
// @Summary Get a user by id
// @Description Fetch any user by id. Requires the super_user permission.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 403 {object} UserResponse
// @Failure 500 {object} UserResponse
// @Router /api/v1/users/{id} [get]
func (h *Handler) GetUser(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid user id",
		})
	}

	output := h.service.GetUser(c.Request().Context(), &GetUserInput{
		TraceId: traceID,
		ActorId: actorID,
		Id:      id,
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

// DeleteUser handles DELETE /api/v1/users/:id
// @Summary Delete a user
// @Description Hard-delete a user (GDPR erasure). Requires the super_user
// @Description permission.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 403 {object} UserResponse
// @Failure 500 {object} UserResponse
// @Router /api/v1/users/{id} [delete]
func (h *Handler) DeleteUser(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid user id",
		})
	}

	output := h.service.DeleteUser(c.Request().Context(), &DeleteUserInput{
		TraceId: traceID,
		ActorId: actorID,
		Id:      id,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: output.Message,
	})
}
