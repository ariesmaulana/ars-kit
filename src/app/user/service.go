package user

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/clock"
)

// Compile-time check to ensure service implements Service interface
var _ Service = (*service)(nil)

// LoginThrottleConfig configures the per-account login throttle and lockout.
// Zero values are invalid; use DefaultLoginThrottleConfig for sensible
// defaults.
type LoginThrottleConfig struct {
	// MaxFailedAttempts is the number of consecutive failed logins inside
	// FailedWindow that locks the account.
	MaxFailedAttempts int
	// FailedWindow is the counting window: failures older than this reset the
	// counter.
	FailedWindow time.Duration
	// LockoutDuration is how long the account stays locked once the threshold
	// is reached.
	LockoutDuration time.Duration
}

// DefaultLoginThrottleConfig returns the default policy: 5 failed attempts
// within 15 minutes lock the account for 15 minutes.
func DefaultLoginThrottleConfig() LoginThrottleConfig {
	return LoginThrottleConfig{
		MaxFailedAttempts: 5,
		FailedWindow:      15 * time.Minute,
		LockoutDuration:   15 * time.Minute,
	}
}

// service implements the Service interface
type service struct {
	storage           Storage
	permissionService permission.Service
	throttle          LoginThrottleConfig
	jwtService        *JWTService
	clockSource       clock.Source
}

const (
	minPasswordLength      = 12
	passwordHistoryDepth   = 5
	passwordPolicyErrorMsg = "Password must be at least 12 characters long"
)

// NewService creates a new user service instance. A zero LoginThrottleConfig
// falls back to DefaultLoginThrottleConfig so callers that omit the policy
// still get a sane lockout instead of locking every account after one failure.
// jwtService issues access and refresh tokens; the service persists every
// refresh token hash it hands out so rotation and revocation are enforced
// server-side.
func NewService(storage Storage, permissionService permission.Service, throttle LoginThrottleConfig, jwtService *JWTService, clockSource ...clock.Source) Service {
	if throttle.MaxFailedAttempts <= 0 || throttle.FailedWindow <= 0 || throttle.LockoutDuration <= 0 {
		throttle = DefaultLoginThrottleConfig()
	}
	var cs clock.Source = clock.Real()
	if len(clockSource) > 0 && clockSource[0] != nil {
		cs = clockSource[0]
	}
	return &service{
		storage:           storage,
		permissionService: permissionService,
		throttle:          throttle,
		jwtService:        jwtService,
		clockSource:       cs,
	}
}

// issueTokenPair mints a fresh access + refresh token pair, persists the
// refresh token hash at the given token_version, and returns both tokens. It
// must be called inside a transaction the caller commits; on error the caller
// rolls back.
func (s *service) issueTokenPair(ctx context.Context, db StorageTx, tokenVersion int, user User) (string, string, error) {
	accessToken, err := s.jwtService.GenerateToken(user.Id, user.Username, tokenVersion)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	expiresAt := s.clockSource.Now().Add(s.jwtService.RefreshExpiration())
	if err := db.InsertRefreshToken(ctx, user.Id, s.jwtService.HashRefreshToken(refreshToken), tokenVersion, expiresAt); err != nil {
		return "", "", fmt.Errorf("persist refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// Register creates a new user account
func (s *service) Register(ctx context.Context, input *RegisterInput) *RegisterOutput {
	resp := &RegisterOutput{TraceId: input.TraceId}

	if msg := validateRegisterInput(input); msg != "" {
		resp.Message = msg
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to hash password")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	insertedId, errType, err := db.InsertUser(ctx, input.Username, input.Email, input.FullName, string(hashedPassword))
	if err != nil {
		if errType == ErrTypeUniqueConstraint {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert user")
			resp.Message = "Username or email already exists"
			resp.ErrorCode = ErrorCodeValidation
			return resp
		}
		if err := db.InsertPasswordHistory(ctx, insertedId, string(hashedPassword)); err != nil {
			log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert password history")
			resp.Message = "Failed to register user"
			resp.ErrorCode = ErrorCodeInternal
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to insert user")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	data, err := db.GetUserById(ctx, insertedId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to get user")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// A fresh account starts at token_version 0. Issue the access + refresh
	// pair and persist the refresh token hash in the same transaction.
	tokenVersion, err := db.GetUserTokenVersion(ctx, insertedId)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to get token version")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	accessToken, refreshToken, err := s.issueTokenPair(ctx, db, tokenVersion, data)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to issue token pair")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	resp.AccessToken = accessToken
	resp.RefreshToken = refreshToken

	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to register user"
		resp.ErrorCode = ErrorCodeInternal
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

// validateRegisterInput returns "" when the input is valid, otherwise the
// user-facing validation message. Shared by the synchronous Register and the
// async RegisterAsync so both flows enforce the same rules.
func validateRegisterInput(input *RegisterInput) string {
	if input.Username == "" {
		log.Warn().Msg("Username empty")
		return "Username is mandatory"
	}
	if len(input.Username) < 5 {
		log.Warn().Msg("Username too short")
		return "Username must be at least 5 characters long"
	}
	if input.Email == "" {
		log.Warn().Msg("Email empty")
		return "Email is mandatory"
	}
	if err := validateEmail(input.Email); err != nil {
		log.Warn().Msg("Invalid email")
		return "Invalid email"
	}
	if input.Password == "" {
		log.Warn().Msg("Password empty")
		return "Password is mandatory"
	}
	if len(input.Password) < minPasswordLength {
		log.Warn().Msg("Password too short")
		return passwordPolicyErrorMsg
	}
	if input.FullName == "" {
		log.Warn().Msg("FullName empty")
		return "FullName is mandatory"
	}
	return ""
}

// Login authenticates a user. Failed attempts are counted per account inside
// a configurable window; reaching the threshold locks the account for the
// configured duration. A successful login resets the counter and lock.
//
// The whole check-and-update sequence runs inside one explicit transaction
// (BeginTx ... Commit, with Rollback on every early return). The user row is
// locked (SELECT ... FOR UPDATE) before the throttle state is read or written,
// so concurrent login attempts for the same account are serialized and the
// failed-attempt counter can never under-count below the lockout threshold.
func (s *service) Login(ctx context.Context, input *LoginInput) *LoginOutput {
	resp := &LoginOutput{TraceId: input.TraceId}

	// Validate input
	if input.Username == "" {
		log.Warn().Msg("Username empty")
		resp.Message = "Username is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if input.Password == "" {
		log.Warn().Msg("Password empty")
		resp.Message = "Password is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	// Begin transaction — every throttle state read/write happens inside it.
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
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
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}
	if user.Status != UserStatusActive {
		log.Info().
			Str("traceId", input.TraceId).
			Str("username", input.Username).
			Str("status", string(user.Status)).
			Msg("Login blocked: account disabled")
		resp.Message = "Account disabled"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	// Lock the row for the rest of the login attempt. Serializing attempts per
	// account makes the failed-attempt counter updates atomic and prevents a
	// race where concurrent failures under-count below the threshold.
	state, errType, err := db.LockUserLoginState(ctx, user.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Info().
				Str("traceId", input.TraceId).
				Str("username", input.Username).
				Msg("User disappeared between reads")
			resp.Message = "Invalid username or password"
			resp.ErrorCode = ErrorCodeUnauthorized
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock login state")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	now := s.clockSource.Now().UTC()

	// Reject while locked without comparing the password or counting the
	// attempt: brute-force attempts must not burn CPU or extend the lock.
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		lockedUntil := *state.LockedUntil
		retryAfter := int(math.Ceil(lockedUntil.Sub(now).Seconds()))
		if retryAfter < 1 {
			retryAfter = 1
		}
		minutes := int(math.Ceil(lockedUntil.Sub(now).Minutes()))
		if minutes < 1 {
			minutes = 1
		}
		log.Info().
			Str("traceId", input.TraceId).
			Int("id", user.Id).
			Str("username", input.Username).
			Msg("Login blocked: account locked")
		resp.Message = fmt.Sprintf("Account temporarily locked. Try again in %d minute(s).", minutes)
		resp.ErrorCode = ErrorCodeLocked
		resp.LockedUntil = &lockedUntil
		resp.RetryAfterSeconds = retryAfter
		return resp
	}

	// Get stored password
	storedPassword, err := db.GetUserPassword(ctx, user.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user password")
		resp.Message = "Invalid username or password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(input.Password))
	if err != nil {
		// Persist the failure (and lock the account when the threshold is hit)
		// even though the login itself fails. The update is committed so the
		// counter survives across attempts.
		newState := s.applyFailedAttempt(state, now)
		if recordErr := db.RecordFailedLogin(ctx, user.Id, newState); recordErr != nil {
			log.Err(recordErr).Str("traceId", input.TraceId).Msg("Failed to record failed login")
		} else if commitErr := db.Commit(); commitErr != nil {
			log.Err(commitErr).Str("traceId", input.TraceId).Msg("Failed to commit failed login")
		}

		if newState.LockedUntil != nil && now.Before(*newState.LockedUntil) {
			log.Warn().
				Str("traceId", input.TraceId).
				Int("id", user.Id).
				Str("username", input.Username).
				Msg("Account locked after too many failed login attempts")
		} else {
			log.Info().
				Str("traceId", input.TraceId).
				Int("id", user.Id).
				Str("username", input.Username).
				Msg("Invalid password attempt")
		}
		resp.Message = "Invalid username or password"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	// Reset the counter and lock on success. The commit happens once, after
	// the refresh token is persisted, so the whole success path is atomic.
	if err := db.ResetLoginState(ctx, user.Id); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to reset login state")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Stamp last login for audit/traceability. Runs in the same transaction as
	// the login so it commits atomically with the issued token.
	lastLogin := s.clockSource.Now().UTC()
	if err := db.UpdateLastLogin(ctx, user.Id, lastLogin); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update last login")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Str("username", input.Username).
		Msg("User logged in successfully")

	// Issue the access + refresh pair and persist the refresh token hash in
	// the same transaction. The refresh token is the server-side handle that
	// rotation and logout revoke.
	tokenVersion, err := db.GetUserTokenVersion(ctx, user.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to get token version")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	accessToken, refreshToken, err := s.issueTokenPair(ctx, db, tokenVersion, user)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to issue token pair")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	resp.AccessToken = accessToken
	resp.RefreshToken = refreshToken

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to login"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Login successful"
	resp.User = user
	// Reflect the just-stamped login time on the returned user so callers
	// (and tests) see it without a second lookup.
	resp.User.LastLoginAt = &lastLogin

	return resp
}

// applyFailedAttempt applies the counting-window policy to a failed login and
// returns the state to persist. A lock that has already expired gives the
// account a fresh window; a failure older than the configured window resets
// the counter; once the counter reaches MaxFailedAttempts the account locks
// for LockoutDuration.
func (s *service) applyFailedAttempt(state LoginState, now time.Time) LoginState {
	// An expired lock unlocks the account and restarts counting.
	if state.LockedUntil != nil {
		if now.Before(*state.LockedUntil) {
			// Still locked: the caller rejects locked accounts before calling,
			// so treat this as a no-op rather than extending the lock.
			return state
		}
		state.LockedUntil = nil
		state.FailedAttempts = 0
	}

	// Failures older than the window reset the counter.
	if state.LastFailedLoginAt != nil && now.Sub(*state.LastFailedLoginAt) > s.throttle.FailedWindow {
		state.FailedAttempts = 0
	}

	state.FailedAttempts++
	state.LastFailedLoginAt = &now
	if state.FailedAttempts >= s.throttle.MaxFailedAttempts {
		lockedUntil := now.Add(s.throttle.LockoutDuration)
		state.LockedUntil = &lockedUntil
	}
	return state
}

// Refresh rotates a refresh token: it revokes the presented token and issues a
// fresh access + refresh pair, so each refresh token can be used exactly once.
// The presented token is rejected when it is unknown, already revoked, expired,
// or was issued before the user's current token_version (password change).
func (s *service) Refresh(ctx context.Context, input *RefreshInput) *RefreshOutput {
	resp := &RefreshOutput{TraceId: input.TraceId}

	if input.RefreshToken == "" {
		log.Warn().Str("traceId", input.TraceId).Msg("Refresh token empty")
		resp.Message = "Invalid or expired refresh token"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to refresh session"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Look the token up by hash with a FOR UPDATE row lock so two concurrent
	// refreshes with the same token cannot both rotate it.
	tokenHash := s.jwtService.HashRefreshToken(input.RefreshToken)
	rt, err := db.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Msg("Refresh token not found")
		resp.Message = "Invalid or expired refresh token"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	if rt.RevokedAt != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Int("refreshTokenId", rt.Id).
			Msg("Refresh token already revoked")
		resp.Message = "Invalid or expired refresh token"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	if s.clockSource.Now().After(rt.ExpiresAt) {
		log.Info().
			Str("traceId", input.TraceId).
			Int("refreshTokenId", rt.Id).
			Msg("Refresh token expired")
		resp.Message = "Invalid or expired refresh token"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	user, err := db.GetUserById(ctx, rt.UserId)
	if err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("userId", rt.UserId).
			Msg("Failed to get refresh token owner")
		resp.Message = "Invalid or expired refresh token"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	currentVersion, err := db.GetUserTokenVersion(ctx, rt.UserId)
	if err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("userId", rt.UserId).
			Msg("Failed to get user token version")
		resp.Message = "Failed to refresh session"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if currentVersion != rt.TokenVersion {
		// The token predates a security event (e.g. password change) that
		// invalidated every earlier session.
		log.Info().
			Str("traceId", input.TraceId).
			Int("userId", rt.UserId).
			Int("tokenVersion", rt.TokenVersion).
			Int("currentVersion", currentVersion).
			Msg("Refresh token issued at stale token version")
		resp.Message = "Invalid or expired refresh token"
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}

	// Revoke the presented token first, then issue the rotated pair, so a
	// replayed token can never be used again.
	if err := db.RevokeRefreshToken(ctx, rt.Id); err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("refreshTokenId", rt.Id).
			Msg("Failed to revoke refresh token")
		resp.Message = "Failed to refresh session"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	accessToken, refreshToken, err := s.issueTokenPair(ctx, db, currentVersion, user)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to issue rotated token pair")
		resp.Message = "Failed to refresh session"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to refresh session"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("userId", rt.UserId).
		Msg("Session refreshed")

	resp.Success = true
	resp.Message = "Session refreshed"
	resp.User = user
	resp.AccessToken = accessToken
	resp.RefreshToken = refreshToken
	return resp
}

// Logout revokes the presented refresh token server-side so it cannot be
// replayed. It is idempotent: an unknown, expired, or already-revoked token
// still reports success, since the client clears its cookies either way.
func (s *service) Logout(ctx context.Context, input *LogoutInput) *LogoutOutput {
	resp := &LogoutOutput{TraceId: input.TraceId}

	if input.RefreshToken == "" {
		resp.Success = true
		resp.Message = "Logged out successfully"
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to logout"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	tokenHash := s.jwtService.HashRefreshToken(input.RefreshToken)
	rt, err := db.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		// Unknown token: nothing to revoke, still end the session.
		log.Info().
			Str("traceId", input.TraceId).
			Msg("Logout with unknown refresh token")
		resp.Success = true
		resp.Message = "Logged out successfully"
		return resp
	}

	// Revoking an already-revoked row is a no-op on the outcome, so the
	// operation stays idempotent.
	if err := db.RevokeRefreshToken(ctx, rt.Id); err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("refreshTokenId", rt.Id).
			Msg("Failed to revoke refresh token")
		resp.Message = "Failed to logout"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to logout"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Msg("Logged out successfully")

	resp.Success = true
	resp.Message = "Logged out successfully"
	return resp
}

// hasPermission reports whether the acting user holds the given permission,
// which is either an action permission ("user:profile_update") or the super
// user permission (PermissionSuperUser). The permission module builds the
// "<user_id>:<permission>" key itself and also grants access to users holding
// "<user_id>:super_user". It logs and returns false when the permission
// module cannot confirm it.
func (s *service) hasPermission(ctx context.Context, traceId string, userID int, perm string) bool {
	if s.permissionService == nil {
		log.Warn().Str("traceId", traceId).Int("userId", userID).Str("permission", perm).Msg("Permission service not wired")
		return false
	}

	output := s.permissionService.CheckPermission(ctx, &permission.CheckPermissionInput{
		TraceId:    traceId,
		UserID:     userID,
		Permission: perm,
	})
	if !output.Success {
		log.Warn().
			Str("traceId", traceId).
			Int("userId", userID).
			Str("permission", perm).
			Msg("Permission check failed")
		return false
	}
	return output.HasPermission
}

// UpdateUsername updates a user's username
func (s *service) UpdateUsername(ctx context.Context, input *UpdateUsernameInput) *UpdateUsernameOutput {
	resp := &UpdateUsernameOutput{TraceId: input.TraceId}

	// Validate input
	if input.NewUsername == "" {
		log.Warn().Msg("New username empty")
		resp.Message = "New username is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if len(input.NewUsername) < 5 {
		log.Warn().Msg("New username too short")
		resp.Message = "Username must be at least 5 characters long"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.Id, PermissionUpdateProfile) {
		resp.Message = "Unauthorized: you do not have permission to update profile"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Lock user row for update (pessimistic lock)
	_, errType, err := db.LockUserById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
			resp.Message = "No Username Found"
			resp.ErrorCode = ErrorCodeValidation
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Update username
	err = db.UpdateUsername(ctx, input.Id, input.NewUsername)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update username")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Get updated user
	data, err := db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user")
		resp.Message = "No Username Found"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	// Commit transaction
	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update username"
		resp.ErrorCode = ErrorCodeInternal
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
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if input.NewPassword == "" {
		log.Warn().Msg("New password empty")
		resp.Message = "New password is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if len(input.NewPassword) < minPasswordLength {
		log.Warn().Msg("New password too short")
		resp.Message = passwordPolicyErrorMsg
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.Id, PermissionUpdatePassword) {
		resp.Message = "Unauthorized: you do not have permission to update password"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	// Begin transaction
	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	// Lock user row for update (pessimistic lock)
	_, errType, err := db.LockUserById(ctx, input.Id)
	if err != nil {
		if errType == ErrTypeNotFound {
			log.Err(err).Str("traceId", input.TraceId).Msg("User not found")
			resp.Message = "User not found"
			resp.ErrorCode = ErrorCodeValidation
			return resp
		}
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Get stored password
	storedPassword, err := db.GetUserPassword(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to get user password")
		resp.Message = "User not found"
		resp.ErrorCode = ErrorCodeValidation
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
		resp.ErrorCode = ErrorCodeUnauthorized
		return resp
	}
	recentHashes, err := db.GetRecentPasswordHashes(ctx, input.Id, passwordHistoryDepth)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to fetch password history")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	for _, oldHash := range recentHashes {
		if bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(input.NewPassword)) == nil {
			resp.Message = "New password must be different from recent passwords"
			resp.ErrorCode = ErrorCodeValidation
			return resp
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to hash new password")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Update password
	err = db.UpdatePassword(ctx, input.Id, string(hashedPassword))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to update password")
		resp.Message = err.Error()
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if err := db.InsertPasswordHistory(ctx, input.Id, string(hashedPassword)); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to insert password history")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// A password change invalidates every session: bump token_version so all
	// access tokens minted at an earlier version are rejected by the JWT
	// middleware, and revoke every active refresh token server-side.
	if err := db.BumpUserTokenVersion(ctx, input.Id); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to bump token version")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if err := db.RevokeAllUserRefreshTokens(ctx, input.Id); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to revoke user refresh tokens")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	// Commit transaction
	err = db.Commit()
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to update password"
		resp.ErrorCode = ErrorCodeInternal
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
		resp.ErrorCode = ErrorCodeInternal
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
		resp.ErrorCode = ErrorCodeValidation
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

// ListUsers lists users for an admin. The actor must hold the super_user
// permission; without it the call is rejected before any query runs.
func (s *service) ListUsers(ctx context.Context, input *ListUsersInput) *ListUsersOutput {
	resp := &ListUsersOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.ActorId == 0 {
		log.Warn().Msg("Actor ID empty")
		resp.Message = "Actor ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can list users"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	page, size := input.Page, input.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to list users"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	users, total, err := db.ListUsers(ctx, page, size, input.Filter)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to list users")
		resp.Message = "Failed to list users"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Users retrieved successfully"
	resp.Users = users
	resp.Total = total
	resp.Page = page
	resp.Size = size
	return resp
}

// GetUser fetches any user by id for an admin. The actor must hold the
// super_user permission.
func (s *service) GetUser(ctx context.Context, input *GetUserInput) *GetUserOutput {
	resp := &GetUserOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.ActorId == 0 {
		log.Warn().Msg("Actor ID empty")
		resp.Message = "Actor ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Id == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can view users"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to fetch user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	user, err := db.GetUserById(ctx, input.Id)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Int("id", input.Id).Msg("User not found")
		resp.Message = "User not found"
		resp.ErrorCode = ErrorCodeNotFound
		return resp
	}

	resp.Success = true
	resp.Message = "User retrieved successfully"
	resp.User = user
	return resp
}

// DeleteUser hard-deletes a user for an admin (GDPR erasure). The actor must
// hold the super_user permission. Refresh tokens cascade via the foreign key.
func (s *service) DeleteUser(ctx context.Context, input *DeleteUserInput) *DeleteUserOutput {
	resp := &DeleteUserOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.ActorId == 0 {
		log.Warn().Msg("Actor ID empty")
		resp.Message = "Actor ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Id == 0 {
		log.Warn().Msg("User ID empty")
		resp.Message = "User ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can delete users"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to delete user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	_, errType, err := db.LockUserById(ctx, input.Id)
	if errType == ErrTypeNotFound {
		log.Info().Str("traceId", input.TraceId).Int("id", input.Id).Msg("User not found; treating delete as successful no-op")
		resp.Success = true
		resp.Message = "User deleted successfully"
		return resp
	}
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to lock user")
		resp.Message = "Failed to delete user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.DeleteUser(ctx, input.Id); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to delete user")
		resp.Message = "Failed to delete user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to commit")
		resp.Message = "Failed to delete user"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().Str("traceId", input.TraceId).Int("id", input.Id).Msg("User deleted successfully")

	resp.Success = true
	resp.Message = "User deleted successfully"
	return resp
}

// GrantPermission assigns a permission to a target user.
// Only an actor holding the "<actorId>:super_user" permission may do this.
func (s *service) GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput {
	resp := &GrantPermissionOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.ActorId == 0 {
		log.Warn().Msg("Actor ID empty")
		resp.Message = "Actor ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.TargetUserId == 0 {
		log.Warn().Msg("Target user ID empty")
		resp.Message = "Target user ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can grant permissions"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	output := s.permissionService.GrantPermission(ctx, &permission.GrantPermissionInput{
		TraceId:    input.TraceId,
		UserID:     input.TargetUserId,
		Permission: input.Permission,
		ActorId:    input.ActorId,
	})
	if !output.Success {
		resp.Message = output.Message
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permission granted successfully"
	return resp
}

// RevokePermission removes a permission from a target user.
// Only the *holding the <actorId>:super_user permission may do this.
func (s *service) RevokePermission(ctx context.Context, input *RevokePermissionInput) *RevokePermissionOutput {
	resp := &RevokePermissionOutput{TraceId: input.TraceId}

	if input.TraceId == "" {
		log.Warn().Msg("TraceId empty")
		resp.Message = "TraceId is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.ActorId == 0 {
		log.Warn().Msg("Actor ID empty")
		resp.Message = "Actor ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.TargetUserId == 0 {
		log.Warn().Msg("Target user ID empty")
		resp.Message = "Target user ID is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.Permission == "" {
		log.Warn().Msg("Permission empty")
		resp.Message = "Permission is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	if !s.hasPermission(ctx, input.TraceId, input.ActorId, PermissionSuperUser) {
		resp.Message = "Unauthorized: only super user can revoke permissions"
		resp.ErrorCode = ErrorCodeForbidden
		return resp
	}

	output := s.permissionService.RevokePermission(ctx, &permission.RevokePermissionInput{
		TraceId:    input.TraceId,
		UserID:     input.TargetUserId,
		Permission: input.Permission,
		ActorId:    input.ActorId,
	})
	if !output.Success {
		resp.Message = output.Message
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	resp.Success = true
	resp.Message = "Permission revoked successfully"
	return resp
}
