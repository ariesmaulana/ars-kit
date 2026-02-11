package user

import (
	"context"
)

// Service defines the interface for user business logic
type Service interface {
	// Register creates a new user account
	Register(ctx context.Context, input *RegisterInput) *RegisterOutput

	// Login authenticates a user
	Login(ctx context.Context, input *LoginInput) *LoginOutput

	// UpdateUsername updates a user's username
	UpdateUsername(ctx context.Context, input *UpdateUsernameInput) *UpdateUsernameOutput

	// UpdatePassword updates a user's password
	UpdatePassword(ctx context.Context, input *UpdatePasswordInput) *UpdatePasswordOutput

	// GetProfileById retrieves a user profile by ID
	GetProfileById(ctx context.Context, input *GetProfileByIdInput) *GetProfileByIdOutput

	AddMember(ctx context.Context, input *AddMemberInput) *AddMemberOutput

	GetMemberById(ctx context.Context, input *GetMemberByIdInput) *GetMemberByIdOutput

	// GetMembersByUserId retrieves all members for a user by user ID
	GetMembersByUserId(ctx context.Context, input *GetMembersByUserIdInput) *GetMembersByUserIdOutput

	UpdateMemberInfo(ctx context.Context, input *UpdateMemberInfoInput) *UpdateMemberInfoOutput

	DeleteMember(ctx context.Context, input *DeleteMemberInput) *DeleteMemberOutput
}

// RegisterInput represents input for user registration
type RegisterInput struct {
	TraceId  string
	Username string
	Email    string
	FullName string
	Password string
}

// RegisterOutput represents output after user registration
type RegisterOutput struct {
	Success bool
	Message string
	TraceId string
	User    User
}

// LoginInput represents input for user login
type LoginInput struct {
	TraceId  string
	Username string
	Password string
}

// LoginOutput represents output after user login
type LoginOutput struct {
	Success bool
	Message string
	TraceId string
	User    User
}

// UpdateUsernameInput represents input for updating username
type UpdateUsernameInput struct {
	TraceId     string
	Id          int
	NewUsername string
}

// UpdateUsernameOutput represents output after updating username
type UpdateUsernameOutput struct {
	Success bool
	Message string
	TraceId string
	User    User
}

// UpdatePasswordInput represents input for updating password
type UpdatePasswordInput struct {
	TraceId     string
	Id          int
	OldPassword string
	NewPassword string
}

// UpdatePasswordOutput represents output after updating password
type UpdatePasswordOutput struct {
	Success bool
	Message string
	TraceId string
}

// GetProfileByIdInput represents input for getting user profile
type GetProfileByIdInput struct {
	TraceId string
	Id      int
}

// GetProfileByIdOutput represents output after getting user profile
type GetProfileByIdOutput struct {
	Success bool
	Message string
	TraceId string
	User    User
}

type AddMemberInput struct {
	TraceId       string
	Id            int
	Name          string
	MonthlyIncome int
}

type AddMemberOutput struct {
	Success bool
	Message string
	TraceId string
	Members []Member
}

type GetMemberByIdInput struct {
	TraceId  string
	MemberId int
}

type GetMemberByIdOutput struct {
	Success bool
	Message string
	TraceId string
	Member  Member
}

type GetMembersByUserIdInput struct {
	TraceId string
	UserId  int
}

type GetMembersByUserIdOutput struct {
	Success bool
	Message string
	TraceId string
	Members []Member
}

type UpdateMemberInfoInput struct {
	TraceId       string
	RequesterId   int
	Id            int
	Name          string
	MonthlyIncome int
}

type UpdateMemberInfoOutput struct {
	Success bool
	Message string
	TraceId string
}

type DeleteMemberInput struct {
	TraceId     string
	RequesterId int
	Id          int
}

type DeleteMemberOutput struct {
	Success bool
	Message string
	TraceId string
}
