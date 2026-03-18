package user

import (
	"context"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// Compile-time check to ensure service implements Service interface
var _ Service = (*service)(nil)

// service implements the Service interface
type service struct {
	storage Storage
}

// NewService creates a new user service instance
func NewService(storage Storage) Service {
	return &service{
		storage: storage,
	}
}

// Register creates a new user account
func (s *service) Register(ctx context.Context, input *RegisterInput) *RegisterOutput {
	resp := &RegisterOutput{TraceId: input.TraceId}

	if input.Username == "" {
		log.Warn().Msg("Username empty")
		resp.Message = "Username is mandatory"
		return resp
	}

	if len(input.Username) < 5 {
		log.Warn().Msg("Username too short")
		resp.Message = "Username must be at least 5 characters long"
		return resp
	}
	if input.Email == "" {
		log.Warn().Msg("Email empty")
		resp.Message = "Email is mandatory"
		return resp
	}

	err := validateEmail(input.Email)
	if err != nil {
		log.Warn().Msg("Invalid email")
		resp.Message = "Invalid email"
		return resp
	}

	if input.Password == "" {
		log.Warn().Msg("Password empty")
		resp.Message = "Password is mandatory"
		return resp
	}

	if len(input.Password) < 7 {
		log.Warn().Msg("Password too short")
		resp.Message = "Password must be at least 7 characters long"
		return resp
	}

	if input.FullName == "" {
		log.Warn().Msg("FullName empty")
		resp.Message = "FullName is mandatory"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		return resp
	}
	defer db.Rollback()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to hash password")
		return resp
	}

	insertedId, errType, err := db.InsertUser(ctx, input.Username, input.Email, input.FullName, string(hashedPassword))
	if err != nil {
		if errType == ErrTypeUniqueConstraint {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert user")
			resp.Message = "Username or email already exists"
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert user")
		return resp
	}

	data, err := db.GetUserById(ctx, insertedId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to get user")
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Str("username", input.Username).
		Str("email", input.Email).
		Msg("User registered successfully")

	resp.Success = true
	resp.Message = "User registered successfully"
	resp.User = data
	return resp
}

// Login authenticates a user
func (s *service) Login(ctx context.Context, input *LoginInput) *LoginOutput {
	resp := &LoginOutput{TraceId: input.TraceId}

	// Validate input
	if input.Username == "" {
		log.Warn().Msg("Username empty")
		resp.Message = "Username is mandatory"
		return resp
	}

	if input.Password == "" {
		log.Warn().Msg("Password empty")
		resp.Message = "Password is mandatory"
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		return resp
	}
	defer db.Rollback()

	// Get user by username
	user, err := db.GetUserByUsername(ctx, input.Username)
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Str("username", input.Username).
			Msg("User not found")
		resp.Message = "Invalid username or password"
		return resp
	}

	// Get stored password
	storedPassword, err := db.GetUserPassword(ctx, user.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user password")
		resp.Message = "Invalid username or password"
		return resp
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(input.Password))
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Str("username", input.Username).
			Msg("Invalid password attempt")
		resp.Message = "Invalid username or password"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Str("username", input.Username).
		Msg("User logged in successfully")

	resp.Success = true
	resp.Message = "Login successful"
	resp.User = user

	return resp
}

// UpdateUsername updates a user's username
func (s *service) UpdateUsername(ctx context.Context, input *UpdateUsernameInput) *UpdateUsernameOutput {
	resp := &UpdateUsernameOutput{TraceId: input.TraceId}

	// Validate input
	if input.NewUsername == "" {
		log.Warn().Msg("New username empty")
		resp.Message = "New username is mandatory"
		return resp
	}

	if len(input.NewUsername) < 5 {
		log.Warn().Msg("New username too short")
		resp.Message = "Username must be at least 5 characters long"
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		return resp
	}
	defer db.Rollback()

	// Lock user row for update (pessimistic lock)
	_, errType, err := db.LockUserById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
			resp.Message = "No Username Found"
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to update username"
		return resp
	}

	// Update username
	err = db.UpdateUsername(ctx, input.Id, input.NewUsername)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update username")
		resp.Message = "Failed to update username"
		return resp
	}

	// Get updated user
	data, err := db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user")
		resp.Message = "No Username Found"
		return resp
	}

	// Commit transaction
	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update username"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("id", input.Id).
		Str("newUsername", input.NewUsername).
		Msg("Username updated successfully")

	resp.Success = true
	resp.Message = "Username updated successfully"
	resp.User = data

	return resp
}

// UpdatePassword updates a user's password
func (s *service) UpdatePassword(ctx context.Context, input *UpdatePasswordInput) *UpdatePasswordOutput {
	resp := &UpdatePasswordOutput{TraceId: input.TraceId}

	// Validate input
	if input.OldPassword == "" {
		log.Warn().Msg("Old password empty")
		resp.Message = "Old password is mandatory"
		return resp
	}

	if input.NewPassword == "" {
		log.Warn().Msg("New password empty")
		resp.Message = "New password is mandatory"
		return resp
	}

	if len(input.NewPassword) < 7 {
		log.Warn().Msg("New password too short")
		resp.Message = "Password must be at least 7 characters long"
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		return resp
	}
	defer db.Rollback()

	// Lock user row for update (pessimistic lock)
	_, errType, err := db.LockUserById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
			resp.Message = "User not found"
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to update password"
		return resp
	}

	// Get stored password
	storedPassword, err := db.GetUserPassword(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user password")
		resp.Message = "User not found"
		return resp
	}

	// Verify old password
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(input.OldPassword))
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Int("id", input.Id).
			Msg("Invalid old password attempt")
		resp.Message = "Invalid old password"
		return resp
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to hash new password")
		resp.Message = "Failed to update password"
		return resp
	}

	// Update password
	err = db.UpdatePassword(ctx, input.Id, string(hashedPassword))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update password")
		resp.Message = err.Error()
		return resp
	}

	// Commit transaction
	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update password"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("id", input.Id).
		Msg("Password updated successfully")

	resp.Success = true
	resp.Message = "Password updated successfully"

	return resp
}

// GetProfileById retrieves a user profile by ID
func (s *service) GetProfileById(ctx context.Context, input *GetProfileByIdInput) *GetProfileByIdOutput {
	resp := &GetProfileByIdOutput{TraceId: input.TraceId}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to fetch profile"
		return resp
	}
	defer db.Rollback()

	// Get user by ID
	user, err := db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("id", input.Id).
			Msg("User not found")
		resp.Message = "User not found"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("id", input.Id).
		Msg("Profile retrieved successfully")

	resp.Success = true
	resp.Message = "Profile retrieved successfully"
	resp.User = user

	return resp
}

// AddMember adds a new member to a user's account
func (s *service) AddMember(ctx context.Context, input *AddMemberInput) *AddMemberOutput {
	resp := &AddMemberOutput{TraceId: input.TraceId}

	// Validate input
	if input.Name == "" {
		log.Warn().Msg("Member name empty")
		resp.Message = "Member name is mandatory"
		return resp
	}

	if len(input.Name) < 2 {
		log.Warn().Msg("Member name too short")
		resp.Message = "Member name must be at least 2 characters long"
		return resp
	}

	if input.MonthlyIncome < 0 {
		log.Warn().Msg("Monthly income cannot be negative")
		resp.Message = "Monthly income cannot be negative"
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to add member"
		return resp
	}
	defer db.Rollback()

	// Verify user exists
	_, err = db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
		resp.Message = "User not found"
		return resp
	}

	// Insert member
	memberId, err := db.InsertMember(ctx, input.Id, input.Name, input.MonthlyIncome)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to insert member")
		resp.Message = "Failed to add member"
		return resp
	}

	// Get the created member
	member, _, err := db.GetMemberById(ctx, memberId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get created member")
		resp.Message = "Failed to add member"
		return resp
	}

	// Commit transaction
	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to add member"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("userId", input.Id).
		Int("memberId", memberId).
		Str("memberName", input.Name).
		Msg("Member added successfully")

	resp.Success = true
	resp.Message = "Member added successfully"
	resp.Member = member

	return resp
}

// GetMemberById retrieves a member by ID
func (s *service) GetMemberById(ctx context.Context, input *GetMemberByIdInput) *GetMemberByIdOutput {
	resp := &GetMemberByIdOutput{TraceId: input.TraceId}

	if input.MemberId == 0 {
		log.Warn().Msg("Member ID empty")
		resp.Message = "Member ID is mandatory"
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to get member"
		return resp
	}
	defer db.Rollback()

	// Get member by ID
	member, _, err := db.GetMemberById(ctx, input.MemberId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Member not found")
		resp.Message = "Member not found"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("memberId", input.MemberId).
		Msg("Member retrieved successfully")

	resp.Success = true
	resp.Message = "Member retrieved successfully"
	resp.Member = member

	return resp
}

func (s *service) GetMembersByUserId(ctx context.Context, input *GetMembersByUserIdInput) *GetMembersByUserIdOutput {
	resp := &GetMembersByUserIdOutput{TraceId: input.TraceId}

	if input.UserId == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		return resp
	}

	// Normalize pagination params
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to get member"
		return resp
	}
	defer db.Rollback()

	//Defensive Code
	// Check if user exists
	_, err = db.GetUserById(ctx, input.UserId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
		resp.Message = "User not found"
		return resp
	}

	// Get paginated members for user
	members, total, err := db.GetMembersByUserId(ctx, input.UserId, pageSize, offset)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Member not found")
		resp.Message = "Member not found"
		return resp
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("userId", input.UserId).
		Int("page", page).
		Int("pageSize", pageSize).
		Int("total", total).
		Msg("Member retrieved successfully")

	resp.Success = true
	resp.Message = "Member retrieved successfully"
	resp.Members = members
	resp.Total = total
	resp.TotalPages = totalPages
	resp.Page = page
	resp.PageSize = pageSize

	return resp
}

func (s *service) UpdateMemberInfo(ctx context.Context, input *UpdateMemberInfoInput) *UpdateMemberInfoOutput {
	resp := &UpdateMemberInfoOutput{TraceId: input.TraceId}

	if input.Id == 0 {
		log.Warn().Msg("Member ID empty")
		resp.Message = "Member ID is mandatory"
		return resp
	}

	if input.RequesterId == 0 {
		log.Warn().Msg("Requester ID empty")
		resp.Message = "Requester ID is mandatory"
		return resp
	}

	if input.Name == "" {
		log.Warn().Msg("Member name empty")
		resp.Message = "Member name is mandatory"
		return resp
	}

	if len(input.Name) < 2 {
		log.Warn().Msg("Member name too short")
		resp.Message = "Member name must be at least 2 characters long"
		return resp
	}

	if input.MonthlyIncome < 0 {
		log.Warn().Msg("Monthly income cannot be negative")
		resp.Message = "Monthly income cannot be negative"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to update member info"
		return resp
	}
	defer db.Rollback()

	// LOCK ORDERING RULE: Lock user first (hierarchy), then member
	// This prevents deadlocks when multiple transactions access these tables

	// Step 1: Lock user first (higher in hierarchy)
	user, errType, err := db.LockUserById(ctx, input.RequesterId)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("Requester not found")
			resp.Message = "Unauthorized update"
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to update member info"
		return resp
	}

	// Step 2: Lock member (lower in hierarchy)
	member, errType, err := db.LockMemberById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("Member not found")
			resp.Message = "Member not found"
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock member")
		resp.Message = "Failed to update member info"
		return resp
	}

	// Verify ownership using locked entities
	if member.UserId != user.Id {
		log.Warn().Str("traceId", input.TraceId).Msg("Unauthorized update")
		resp.Message = "Unauthorized update"
		return resp
	}

	err = db.UpdateMemberInfo(ctx, input.Id, input.Name, input.MonthlyIncome)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update member info")
		resp.Message = "Failed to update member info"
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update member info"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("memberId", input.Id).
		Str("name", input.Name).
		Int("monthlyIncome", input.MonthlyIncome).
		Msg("Member info updated successfully")

	resp.Success = true
	resp.Message = "Member info updated successfully"

	return resp
}

func (s *service) DeleteMember(ctx context.Context, input *DeleteMemberInput) *DeleteMemberOutput {
	resp := &DeleteMemberOutput{TraceId: input.TraceId}

	if input.Id == 0 {
		log.Warn().Msg("Member ID empty")
		resp.Message = "Member ID is mandatory"
		return resp
	}

	if input.RequesterId == 0 {
		log.Warn().Msg("Requester ID empty")
		resp.Message = "Requester ID is mandatory"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to delete member"
		return resp
	}
	defer db.Rollback()

	// LOCK ORDERING RULE: Lock user first (hierarchy), then member
	// This prevents deadlocks when multiple transactions access these tables

	// Step 1: Lock user first (higher in hierarchy)
	user, errType, err := db.LockUserById(ctx, input.RequesterId)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("Requester not found")
			resp.Message = "Unauthorized delete"
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to delete member"
		return resp
	}

	// Step 2: Lock member (lower in hierarchy)
	member, errType, err := db.LockMemberById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Info().
				Str("traceId", input.TraceId).
				Int("memberId", input.Id).
				Msg("Member already deleted, returning success")
			resp.Success = true
			resp.Message = "Member deleted successfully"
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock member")
		resp.Message = "Failed to delete member"
		return resp
	}

	// Verify ownership using locked entities
	if member.UserId != user.Id {
		log.Warn().Str("traceId", input.TraceId).Msg("Unauthorized delete")
		resp.Message = "Unauthorized delete"
		return resp
	}

	err = db.DeleteMemberById(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to delete member")
		resp.Message = "Failed to delete member"
		return resp
	}

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to delete member"
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("memberId", input.Id).
		Int("requesterId", input.RequesterId).
		Msg("Member deleted successfully")

	resp.Success = true
	resp.Message = "Member deleted successfully"

	return resp
}
