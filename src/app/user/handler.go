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
	public.POST("/logout", h.Logout)

	// Protected routes
	protected := users.Group("")
	protected.Use(h.jwtService.JWTMiddleware())
	protected.GET("/profile", h.Profile)
	protected.PUT("/profile/username", h.UpdateUsername)
	protected.PUT("/profile/password", h.UpdatePassword)
	protected.POST("/permissions/grant", h.GrantPermission)
	protected.POST("/permissions/revoke", h.RevokePermission)
	protected.GET("/permissions", h.ListPermissions)
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
//
// LockedUntil and RetryAfterSeconds are populated when the account is locked
// so clients can show when it unlocks instead of guessing from the message.
type AuthResponse struct {
	Success           bool       `json:"success"`
	Message           string     `json:"message"`
	Token             string     `json:"token,omitempty"`
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

	// Generate JWT token
	token, err := h.jwtService.GenerateToken(output.User.Id, output.User.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, AuthResponse{
			Success: false,
			Message: "Failed to generate authentication token",
		})
	}

	// Set token in cookie
	h.jwtService.SetTokenCookie(c, token)

	dto := toUserDTO(output.User)
	return c.JSON(http.StatusCreated, AuthResponse{
		Success: true,
		Message: "User registered successfully",
		Token:   token,
		User:    &dto,
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

	// Generate JWT token
	token, err := h.jwtService.GenerateToken(output.User.Id, output.User.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, AuthResponse{
			Success: false,
			Message: "Failed to generate authentication token",
		})
	}

	// Set token in cookie
	h.jwtService.SetTokenCookie(c, token)

	dto := toUserDTO(output.User)
	return c.JSON(http.StatusOK, AuthResponse{
		Success: true,
		Message: "Login successful",
		Token:   token,
		User:    &dto,
	})
}

// Logout handles POST /api/v1/users/logout
// @Summary Logout user
// @Description Clear authentication token
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {object} UserResponse
// @Router /api/v1/users/logout [post]
func (h *Handler) Logout(c echo.Context) error {
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

// ListPermissionsResponse represents the HTTP response for GET /users/permissions
type ListPermissionsResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Data    *UserPermissionsDTO `json:"data,omitempty"`
}

// UserPermissionsDTO is the permission listing payload: direct grants, the
// roles the user holds (with each role's permissions), and the deduplicated
// effective union of both.
type UserPermissionsDTO struct {
	UserId               int                 `json:"user_id"`
	DirectPermissions    []string            `json:"direct_permissions"`
	Roles                []PermissionRoleDTO `json:"roles"`
	EffectivePermissions []string            `json:"effective_permissions"`
}

// PermissionRoleDTO is a role in a permission listing.
type PermissionRoleDTO struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// ListPermissions handles GET /api/v1/users/permissions
// @Summary List permissions
// @Description List a user's effective permissions: direct grants plus
// @Description role-derived permissions, with roles broken out separately.
// @Description Lists the authenticated user by default; an optional user_id
// @Description query parameter lists another user and requires super user.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id query int false "Target user id (defaults to the authenticated user)"
// @Success 200 {object} ListPermissionsResponse
// @Failure 400 {object} ListPermissionsResponse
// @Failure 401 {object} ListPermissionsResponse
// @Failure 403 {object} ListPermissionsResponse
// @Failure 500 {object} ListPermissionsResponse
// @Router /api/v1/users/permissions [get]
func (h *Handler) ListPermissions(c echo.Context) error {
	traceID := xid.New().String()

	actorID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ListPermissionsResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	targetUserID := actorID
	if v := c.QueryParam("user_id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ListPermissionsResponse{
				Success: false,
				Message: "Invalid user_id",
			})
		}
		targetUserID = id
	}

	output := h.service.ListPermissions(c.Request().Context(), &ListPermissionsInput{
		TraceId:      traceID,
		ActorId:      actorID,
		TargetUserId: targetUserID,
	})

	if !output.Success {
		return c.JSON(statusForError(output.ErrorCode), ListPermissionsResponse{
			Success: false,
			Message: output.Message,
		})
	}

	roles := make([]PermissionRoleDTO, 0, len(output.Roles))
	for _, r := range output.Roles {
		roles = append(roles, PermissionRoleDTO{
			Id:          r.Id,
			Name:        r.Name,
			Description: r.Description,
			Permissions: r.Permissions,
		})
	}

	return c.JSON(http.StatusOK, ListPermissionsResponse{
		Success: true,
		Message: "Permissions retrieved successfully",
		Data: &UserPermissionsDTO{
			UserId:               targetUserID,
			DirectPermissions:    output.Direct,
			Roles:                roles,
			EffectivePermissions: output.Effective,
		},
	})
}
