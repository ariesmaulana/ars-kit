package user

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/ariesmaulana/ars-kit/src/app/notification/email"
	"github.com/ariesmaulana/ars-kit/src/app/workflow"
)

// tokenExpiry returns the configured token lifetime or the default.
func (s *service) tokenExpiry() time.Duration {
	if s.emailCfg.TokenExpiry > 0 {
		return s.emailCfg.TokenExpiry
	}
	return defaultEmailTokenExpiry
}

// queueEmail enqueues an email for delivery by the send_email workflow worker.
// The send is fire-and-forget: send failures are handled by the worker's
// retry policy, never by the HTTP request that enqueued it. When no engine is
// installed (e.g. unit tests), the enqueue fails and is logged, not fatal.
func (s *service) queueEmail(ctx context.Context, traceId string, msg email.EmailMessage) {
	if _, err := workflow.Register(ctx, workflow.NewSendEmailJob(traceId, msg)); err != nil {
		log.Warn().
			Err(err).
			Str("traceId", traceId).
			Strs("to", msg.To).
			Msg("failed to enqueue email workflow")
	}
}

// emailTokenResult bundles the outputs of validateEmailToken so callers
// avoid unpacking a 4-tuple.
type emailTokenResult struct {
	Token     EmailToken
	User      User
	ErrorCode ErrorCode
	Message   string
}

// validateEmailToken validates a single-purpose token and returns the
// resolved token row, the owning user, and an error code + message on failure.
func (s *service) validateEmailToken(ctx context.Context, db StorageTx, purpose EmailTokenPurpose, token string) emailTokenResult {
	hash := s.jwtService.HashRefreshToken(token)
	t, err := db.GetEmailToken(ctx, purpose, hash)
	if err != nil {
		return emailTokenResult{ErrorCode: ErrorCodeValidation, Message: "Invalid or expired token"}
	}
	if t.UsedAt != nil {
		return emailTokenResult{ErrorCode: ErrorCodeValidation, Message: "Invalid or expired token"}
	}
	if s.clockSource.Now().After(t.ExpiresAt) {
		return emailTokenResult{ErrorCode: ErrorCodeValidation, Message: "Invalid or expired token"}
	}
	u, err := db.GetUserById(ctx, t.UserId)
	if err != nil {
		return emailTokenResult{ErrorCode: ErrorCodeInternal, Message: "Failed to process request"}
	}
	return emailTokenResult{Token: t, User: u}
}

// generateAndStoreToken generates a new opaque token, stores its hash, and
// returns the plaintext token for inclusion in an email link.
func (s *service) generateAndStoreToken(ctx context.Context, db StorageTx, userID int, purpose EmailTokenPurpose) (string, error) {
	plain, err := s.jwtService.GenerateRefreshToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	hash := s.jwtService.HashRefreshToken(plain)
	expiresAt := s.clockSource.Now().Add(s.tokenExpiry())
	if err := db.InsertEmailToken(ctx, userID, purpose, hash, expiresAt); err != nil {
		return "", fmt.Errorf("store token: %w", err)
	}
	return plain, nil
}

func (s *service) buildResetLink(token string) string {
	return s.emailCfg.AppURL + "/reset-password?token=" + url.QueryEscape(token)
}

func (s *service) buildVerifyLink(token string) string {
	return s.emailCfg.AppURL + "/verify-email?token=" + url.QueryEscape(token)
}

const genericEmailSuccess = "If an email exists for this account, a link has been sent."

func fmtDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 {
		days := hours / 24
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", hours)
}

// ForgotPassword emails a password-reset link to the account's email address.
// Responds identically whether or not the email exists.
func (s *service) ForgotPassword(ctx context.Context, input *ForgotPasswordInput) *ForgotPasswordOutput {
	resp := &ForgotPasswordOutput{TraceId: input.TraceId}

	if input.Email == "" {
		log.Warn().Msg("Email empty")
		resp.Message = "Email is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if err := validateEmail(input.Email); err != nil {
		log.Warn().Str("email", input.Email).Msg("Invalid email")
		resp.Message = "Invalid email"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = genericEmailSuccess
		resp.Success = true
		return resp
	}
	defer db.Rollback()

	u, err := db.GetUserByEmail(ctx, input.Email)
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Str("email", input.Email).
			Msg("forgot password: email not found")
		if err := db.Commit(); err != nil {
			log.Err(err).Msg("failed to commit")
		}
		resp.Success = true
		resp.Message = genericEmailSuccess
		return resp
	}

	token, err := s.generateAndStoreToken(ctx, db, u.Id, EmailTokenPurposePasswordReset)
	if err != nil {
		log.Err(err).
			Str("traceId", input.TraceId).
			Int("userId", u.Id).
			Msg("failed to create reset token")
		resp.Success = true
		resp.Message = genericEmailSuccess
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Success = true
		resp.Message = genericEmailSuccess
		return resp
	}

	s.queueEmail(ctx, input.TraceId, email.EmailMessage{
		To:      []string{u.Email},
		Subject: "Reset your password",
		Text: "Hi " + u.FullName + ",\n\n" +
			"We received a request to reset your password.\n\n" +
			"Open the link below to set a new password. It expires in " +
			fmtDuration(s.tokenExpiry()) + ":\n\n" +
			s.buildResetLink(token) + "\n\n" +
			"If you didn't request this, ignore this email.\n",
	})

	log.Info().
		Str("traceId", input.TraceId).
		Int("userId", u.Id).
		Msg("forgot password: reset email queued")
	resp.Success = true
	resp.Message = genericEmailSuccess
	return resp
}

// ResetPassword sets a new password authenticated by a single-use reset token.
func (s *service) ResetPassword(ctx context.Context, input *ResetPasswordInput) *ResetPasswordOutput {
	resp := &ResetPasswordOutput{TraceId: input.TraceId}

	if input.Token == "" {
		resp.Message = "Invalid or expired token"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if input.NewPassword == "" {
		resp.Message = "New password is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if len(input.NewPassword) < minPasswordLength {
		resp.Message = passwordPolicyErrorMsg
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to reset password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	tokenResult := s.validateEmailToken(ctx, db, EmailTokenPurposePasswordReset, input.Token)
	if tokenResult.Message != "" {
		resp.Message = tokenResult.Message
		resp.ErrorCode = tokenResult.ErrorCode
		return resp
	}

	recentHashes, err := db.GetRecentPasswordHashes(ctx, tokenResult.User.Id, passwordHistoryDepth)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to fetch password history")
		resp.Message = "Failed to reset password"
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

	if err := s.rotatePassword(ctx, db, tokenResult.User.Id, input.NewPassword); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to rotate password")
		resp.Message = "Failed to reset password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if err := db.MarkEmailTokenUsed(ctx, tokenResult.Token.Id); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to mark reset token used")
		resp.Message = "Failed to reset password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to reset password"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("userId", tokenResult.User.Id).
		Msg("password reset successfully")
	resp.Success = true
	resp.Message = "Password reset successfully"
	return resp
}

// SendVerificationEmail sends an email-verification link. Already-verified
// accounts are a no-op. Never reveals whether the email exists.
func (s *service) SendVerificationEmail(ctx context.Context, input *SendVerificationEmailInput) *SendVerificationEmailOutput {
	resp := &SendVerificationEmailOutput{TraceId: input.TraceId}

	if input.Email == "" {
		resp.Message = "Email is mandatory"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}
	if err := validateEmail(input.Email); err != nil {
		resp.Message = "Invalid email"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Success = true
		resp.Message = genericEmailSuccess
		return resp
	}
	defer db.Rollback()

	u, err := db.GetUserByEmail(ctx, input.Email)
	if err != nil {
		log.Info().
			Str("traceId", input.TraceId).
			Str("email", input.Email).
			Msg("send verification: email not found")
		if err := db.Commit(); err != nil {
			log.Err(err).Msg("failed to commit")
		}
		resp.Success = true
		resp.Message = genericEmailSuccess
		return resp
	}
	if u.EmailVerifiedAt != nil {
		if err := db.Commit(); err != nil {
			log.Err(err).Msg("failed to commit")
		}
		resp.Success = true
		resp.Message = "Email already verified"
		return resp
	}

	token, err := s.generateAndStoreToken(ctx, db, u.Id, EmailTokenPurposeEmailVerification)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Int("userId", u.Id).
			Msg("failed to create verification token")
		resp.Success = true
		resp.Message = genericEmailSuccess
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Success = true
		resp.Message = genericEmailSuccess
		return resp
	}

	s.queueEmail(ctx, input.TraceId, email.EmailMessage{
		To:      []string{u.Email},
		Subject: "Verify your email address",
		Text: "Hi " + u.FullName + ",\n\n" +
			"Please verify your email address by opening the link below.\n" +
			"It expires in " + fmtDuration(s.tokenExpiry()) + ":\n\n" +
			s.buildVerifyLink(token) + "\n\n" +
			"If you didn't create this account, ignore this email.\n",
	})

	log.Info().
		Str("traceId", input.TraceId).
		Int("userId", u.Id).
		Msg("send verification: email queued")
	resp.Success = true
	resp.Message = genericEmailSuccess
	return resp
}

// VerifyEmail marks the account's email as verified using a single-use token.
func (s *service) VerifyEmail(ctx context.Context, input *VerifyEmailInput) *VerifyEmailOutput {
	resp := &VerifyEmailOutput{TraceId: input.TraceId}

	if input.Token == "" {
		resp.Message = "Invalid or expired token"
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("Failed to begin transaction")
		resp.Message = "Failed to verify email"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	defer db.Rollback()

	tokenResult := s.validateEmailToken(ctx, db, EmailTokenPurposeEmailVerification, input.Token)
	if tokenResult.Message != "" {
		resp.Message = tokenResult.Message
		resp.ErrorCode = tokenResult.ErrorCode
		return resp
	}

	if err := db.UpdateEmailVerified(ctx, tokenResult.Token.UserId, s.clockSource.Now()); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to mark email verified")
		resp.Message = "Failed to verify email"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}
	if err := db.MarkEmailTokenUsed(ctx, tokenResult.Token.Id); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to mark token used")
		resp.Message = "Failed to verify email"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to commit")
		resp.Message = "Failed to verify email"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int("userId", tokenResult.Token.UserId).
		Msg("email verified")
	resp.Success = true
	resp.Message = "Email verified successfully"
	return resp
}
