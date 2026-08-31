package user

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/workflow"
)

// This file holds the workflow-facing service methods. They live apart from
// service.go so the HTTP-facing contract and the background workflow seams
// each change for one reason: DemoWorkflow enqueues the job, and RegisterUser
// / GrantPermissionSystem are the operations the demo workflow steps call.

// DemoWorkflow validates the input and enqueues a demo workflow job. The
// user is created and granted the workflow permission by background workers
// instead of synchronously in the request.
func (s *service) DemoWorkflow(ctx context.Context, input *DemoWorkflowInput) *DemoWorkflowOutput {
	resp := &DemoWorkflowOutput{TraceId: input.TraceId}

	if msg := validateUserIdentity(input.Email, input.Username); msg != "" {
		resp.Message = msg
		resp.ErrorCode = ErrorCodeValidation
		return resp
	}

	_, err := workflow.Register(ctx, workflow.NewRegisterDemoWorkflow(input.TraceId, workflow.DemoWorkflowInput{
		Email:    input.Email,
		Username: input.Username,
	}))
	if err != nil {
		log.Err(err).Str("traceId", input.TraceId).Msg("failed to queue demo workflow")
		resp.Message = "Failed to queue demo workflow"
		resp.ErrorCode = ErrorCodeInternal
		return resp
	}

	log.Info().
		Str("traceId", input.TraceId).
		Str("username", input.Username).
		Msg("Demo workflow queued")

	resp.Success = true
	resp.Message = "Demo workflow queued"
	return resp
}

// validateUserIdentity checks the identity fields shared by the demo
// registration input and the RegisterUser workflow seam.
func validateUserIdentity(email, username string) string {
	if email == "" {
		log.Warn().Msg("Email empty")
		return "Email is mandatory"
	}
	if username == "" {
		log.Warn().Msg("Username empty")
		return "Username is mandatory"
	}
	return ""
}

// RegisterUser creates a user account and is the seam the demo workflow's
// RegisterUser step calls. It is idempotent: on a unique constraint (the step
// re-runs after a crash or retry) it returns the existing user instead of
// failing or creating a duplicate.
func (s *service) RegisterUser(ctx context.Context, input *workflow.RegisterUserInput) *workflow.RegisterUserOutput {
	out := &workflow.RegisterUserOutput{}

	if msg := validateUserIdentity(input.Email, input.Username); msg != "" {
		out.Message = msg
		out.ErrorCode = workflow.ErrorCodeValidation
		return out
	}

	db, err := s.storage.BeginTx(ctx)
	if err != nil {
		log.Err(err).Msg("failed to begin transaction")
		out.Message = "Failed to register user"
		out.ErrorCode = workflow.ErrorCodeInternal
		return out
	}
	defer db.Rollback()

	// The demo payload carries no password; insert an empty one.
	insertedID, errType, err := db.InsertUser(ctx, input.Username, input.Email, "", "")
	if err != nil {
		if errType == ErrTypeUniqueConstraint {
			// Idempotency: the step may re-run with the same input; return the
			// existing user instead of failing.
			existing, getErr := db.GetUserByUsername(ctx, input.Username)
			if getErr == nil {
				if commitErr := db.Commit(); commitErr != nil {
					log.Err(commitErr).Msg("failed to commit")
					out.Message = "Failed to register user"
					out.ErrorCode = workflow.ErrorCodeInternal
					return out
				}
				out.Success = true
				out.Message = "User already exists"
				out.User = workflow.User{ID: int64(existing.Id)}
				return out
			}
		}
		log.Err(err).Str("username", input.Username).Msg("failed to insert user")
		out.Message = "Failed to register user"
		out.ErrorCode = workflow.ErrorCodeInternal
		return out
	}

	if err := db.Commit(); err != nil {
		log.Err(err).Msg("failed to commit")
		out.Message = "Failed to register user"
		out.ErrorCode = workflow.ErrorCodeInternal
		return out
	}

	log.Info().
		Str("username", input.Username).
		Int("id", insertedID).
		Msg("User created by workflow")

	out.Success = true
	out.Message = "User registered"
	out.User = workflow.User{ID: int64(insertedID)}
	return out
}

// GrantPermissionSystem grants a permission to a user without the super-user
// actor check. It is the seam the demo workflow's GrantPermission step calls —
// the admin GrantPermission is actor-gated, and a background workflow step has
// no actor.
func (s *service) GrantPermissionSystem(ctx context.Context, input *workflow.GrantPermissionInput) *workflow.GrantPermissionOutput {
	out := &workflow.GrantPermissionOutput{}

	if input.TraceId == "" {
		out.Message = "TraceId is mandatory"
		out.ErrorCode = workflow.ErrorCodeValidation
		return out
	}
	if input.UserID == 0 {
		out.Message = "User ID is mandatory"
		out.ErrorCode = workflow.ErrorCodeValidation
		return out
	}
	if input.RoleName == "" {
		out.Message = "Role name is mandatory"
		out.ErrorCode = workflow.ErrorCodeValidation
		return out
	}
	if s.permissionService == nil {
		log.Warn().Str("traceId", input.TraceId).Msg("Permission service not wired")
		out.Message = "Permission service not wired"
		out.ErrorCode = workflow.ErrorCodeInternal
		return out
	}

	output := s.permissionService.AssignRole(ctx, &permission.AssignRoleInput{
		TraceId:  input.TraceId,
		UserID:   int(input.UserID),
		RoleName: input.RoleName,
	})
	if !output.Success {
		out.Message = output.Message
		out.ErrorCode = workflow.ErrorCodeInternal
		return out
	}

	log.Info().
		Str("traceId", input.TraceId).
		Int64("userId", input.UserID).
		Str("role", input.RoleName).
		Msg("Role assigned by workflow")

	out.Success = true
	out.Message = "Role assigned"
	return out
}
