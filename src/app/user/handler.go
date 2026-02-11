package user

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/xid"
)

// Handler handles HTTP requests for user operations
type Handler struct {
	service    Service
	jwtService *JWTService
}

// NewHandler creates a new user handler
func NewHandler(service Service, jwtService *JWTService) *Handler {
	return &Handler{
		service:    service,
		jwtService: jwtService,
	}
}

// RegisterRoutes registers all user routes
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/users")

	// Public routes
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/logout", h.Logout)

	// Protected routes
	protected := g.Group("")
	protected.Use(h.jwtService.JWTMiddleware())
	protected.GET("/profile", h.Profile)
	protected.PUT("/profile/username", h.UpdateUsername)
	protected.PUT("/profile/password", h.UpdatePassword)
	protected.GET("/members", h.GetMembers)
	protected.POST("/members", h.AddMember)
	protected.PUT("/members/:id", h.UpdateMemberInfo)
	protected.DELETE("/members/:id", h.DeleteMember)
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

// AddMemberRequest represents the HTTP request body for adding a member
type AddMemberRequest struct {
	Name          string `json:"name" validate:"required,min=2"`
	MonthlyIncome int    `json:"monthly_income" validate:"gte=0"`
}

// UpdateMemberInfoRequest represents the HTTP request body for updating member info
type UpdateMemberInfoRequest struct {
	Name          string `json:"name" validate:"required,min=2"`
	MonthlyIncome int    `json:"monthly_income" validate:"gte=0"`
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

// MemberDTO represents a member in HTTP responses
type MemberDTO struct {
	Id            int       `json:"id"`
	UserId        int       `json:"user_id"`
	Name          string    `json:"name"`
	MonthlyIncome int       `json:"monthly_income"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AuthResponse represents the HTTP response for authentication operations
type AuthResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Token   string   `json:"token,omitempty"`
	User    *UserDTO `json:"user,omitempty"`
}

// UserResponse represents the HTTP response for user operations
type UserResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Data    *UserDTO `json:"data,omitempty"`
}

// MembersResponse represents the HTTP response for members operations
type MembersResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    []MemberDTO `json:"data,omitempty"`
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

func toMemberDTO(member Member) MemberDTO {
	return MemberDTO{
		Id:            member.Id,
		UserId:        member.UserId,
		Name:          member.Name,
		MonthlyIncome: member.MonthlyIncome,
		CreatedAt:     member.CreatedAt,
		UpdatedAt:     member.UpdatedAt,
	}
}

func toMemberDTOs(members []Member) []MemberDTO {
	dtos := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		dtos = append(dtos, toMemberDTO(member))
	}
	return dtos
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
// @Router /api/v1/users/register [post]
func (h *Handler) Register(c echo.Context) error {
	traceID := xid.New().String()

	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
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
		return c.JSON(http.StatusBadRequest, AuthResponse{
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
// @Router /api/v1/users/login [post]
func (h *Handler) Login(c echo.Context) error {
	traceID := xid.New().String()

	var req LoginRequest
	if err := c.Bind(&req); err != nil {
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
		return c.JSON(http.StatusUnauthorized, AuthResponse{
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
// @Failure 401 {object} UserResponse
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
		status := http.StatusInternalServerError
		if output.Message == "User not found" {
			status = http.StatusNotFound
		}
		return c.JSON(status, UserResponse{
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
	if err := c.Bind(&req); err != nil {
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
		return c.JSON(http.StatusBadRequest, UserResponse{
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
	if err := c.Bind(&req); err != nil {
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
		status := http.StatusBadRequest
		if output.Message == "Invalid old password" {
			status = http.StatusUnauthorized
		}

		return c.JSON(status, UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: "Password updated successfully",
	})
}

// GetMembers handles GET /api/v1/users/members
// @Summary Get user members
// @Description Get all members for the authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MembersResponse
// @Failure 401 {object} MembersResponse
// @Failure 404 {object} MembersResponse
// @Router /api/v1/users/members [get]
func (h *Handler) GetMembers(c echo.Context) error {
	traceID := xid.New().String()

	userID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, MembersResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	output := h.service.GetMembersByUserId(c.Request().Context(), &GetMembersByUserIdInput{
		TraceId: traceID,
		UserId:  userID,
	})

	if !output.Success {
		status := http.StatusInternalServerError
		if output.Message == "User not found" {
			status = http.StatusNotFound
		}
		return c.JSON(status, MembersResponse{
			Success: false,
			Message: output.Message,
		})
	}

	dtos := toMemberDTOs(output.Members)
	return c.JSON(http.StatusOK, MembersResponse{
		Success: true,
		Message: output.Message,
		Data:    dtos,
	})
}

// AddMember handles POST /api/v1/users/members
// @Summary Add a new member
// @Description Add a new member for the authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param member body AddMemberRequest true "Member data"
// @Success 201 {object} MembersResponse
// @Failure 400 {object} MembersResponse
// @Failure 401 {object} MembersResponse
// @Router /api/v1/users/members [post]
func (h *Handler) AddMember(c echo.Context) error {
	traceID := xid.New().String()

	userID, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, MembersResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	var req AddMemberRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, MembersResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.AddMember(c.Request().Context(), &AddMemberInput{
		TraceId:       traceID,
		Id:            userID,
		Name:          req.Name,
		MonthlyIncome: req.MonthlyIncome,
	})

	if !output.Success {
		status := http.StatusBadRequest
		if output.Message == "User not found" {
			status = http.StatusNotFound
		}
		return c.JSON(status, MembersResponse{
			Success: false,
			Message: output.Message,
		})
	}

	dtos := toMemberDTOs(output.Members)
	return c.JSON(http.StatusCreated, MembersResponse{
		Success: true,
		Message: output.Message,
		Data:    dtos,
	})
}

// UpdateMemberInfo handles PUT /api/v1/users/members/:id
// @Summary Update member information
// @Description Update name and monthly income for a specific member
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Member ID"
// @Param member body UpdateMemberInfoRequest true "Member update data"
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 404 {object} UserResponse
// @Router /api/v1/users/members/{id} [put]
func (h *Handler) UpdateMemberInfo(c echo.Context) error {
	traceID := xid.New().String()

	userId, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	memberId := 0
	if err := echo.PathParamsBinder(c).Int("id", &memberId).BindError(); err != nil {
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid member ID",
		})
	}

	var req UpdateMemberInfoRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	output := h.service.UpdateMemberInfo(c.Request().Context(), &UpdateMemberInfoInput{
		TraceId:       traceID,
		RequesterId:   userId,
		Id:            memberId,
		Name:          req.Name,
		MonthlyIncome: req.MonthlyIncome,
	})

	if !output.Success {
		status := http.StatusBadRequest
		if output.Message == "Member not found" {
			status = http.StatusNotFound
		}
		return c.JSON(status, UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: "Member information updated successfully",
	})
}

// DeleteMember handles DELETE /api/v1/users/members/:id
// @Summary Delete a member
// @Description Delete a specific member by ID
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Member ID"
// @Success 200 {object} UserResponse
// @Failure 400 {object} UserResponse
// @Failure 401 {object} UserResponse
// @Failure 404 {object} UserResponse
// @Router /api/v1/users/members/{id} [delete]
func (h *Handler) DeleteMember(c echo.Context) error {
	traceID := xid.New().String()

	userId, err := GetUserIdFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, UserResponse{
			Success: false,
			Message: "User not authenticated",
		})
	}

	memberId := 0
	if err := echo.PathParamsBinder(c).Int("id", &memberId).BindError(); err != nil {
		return c.JSON(http.StatusBadRequest, UserResponse{
			Success: false,
			Message: "Invalid member ID",
		})
	}

	output := h.service.DeleteMember(c.Request().Context(), &DeleteMemberInput{
		TraceId:     traceID,
		RequesterId: userId,
		Id:          memberId,
	})

	if !output.Success {
		status := http.StatusBadRequest
		if output.Message == "Member not found" {
			status = http.StatusNotFound
		} else if output.Message == "Unauthorized delete" {
			status = http.StatusForbidden
		}
		return c.JSON(status, UserResponse{
			Success: false,
			Message: output.Message,
		})
	}

	return c.JSON(http.StatusOK, UserResponse{
		Success: true,
		Message: "Member deleted successfully",
	})
}
