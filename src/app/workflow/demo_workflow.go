package workflow

import (
	"context"
	"errors"
	"fmt"
)

// ============================================================================
// Domain seam
// ============================================================================

// UserService is the user-domain seam the demo workflow depends on. It is
// satisfied by the user module's service.
//
// The grant seam is deliberately not the user module's admin GrantPermission:
// that one requires a super-user actor (it backs the admin endpoint), while a
// background workflow step has no actor. GrantPermissionSystem is the
// workflow-facing system path (no actor gate) implemented by the user service.
type UserService interface {
	RegisterUser(ctx context.Context, input *RegisterUserInput) *RegisterUserOutput
	GrantPermissionSystem(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput
}

// RegisterUserInput is what RegisterUser needs to create a user account.
type RegisterUserInput struct {
	Email    string
	Username string
}

// RegisterUserOutput is the typed result of RegisterUser, following the domain
// module convention that every service output carries Success, Message and
// ErrorCode.
type RegisterUserOutput struct {
	Success   bool
	Message   string
	ErrorCode ErrorCode
	User      User
}

// User is the result of RegisterUser.
type User struct {
	ID int64
}

// GrantPermissionInput describes a role assignment.
type GrantPermissionInput struct {
	TraceId  string
	UserID   int64
	RoleName string
}

// GrantPermissionOutput is the typed result of GrantPermissionSystem.
type GrantPermissionOutput struct {
	Success   bool
	Message   string
	ErrorCode ErrorCode
}

// ErrorCode categorises a failed workflow seam call, mirroring the domain
// module convention (validation vs internal). Steps only need Success/Message,
// but the convention keeps the machine category explicit.
type ErrorCode string

const (
	ErrorCodeValidation ErrorCode = "validation"
	ErrorCodeInternal   ErrorCode = "internal"
)

// ============================================================================
// Demo Workflow
// ============================================================================

// demoRole is the role assigned by the second step. It carries the
// 'default' permission, seeded by the permission module's migration.
const demoRole = "member"

// DemoWorkflow creates the workflow definition and injects the domain
// services required by this workflow.
func DemoWorkflow(userService UserService) *Definition {
	w := &demoWorkflow{
		userService: userService,
	}

	return &Definition{
		Name:       "demo",
		MaxRetries: 3,

		NewPayload: func() any {
			return &DemoWorkflowInput{}
		},

		Steps: []Step{
			StepFunc("RegisterUser", w.RegisterUser),
			StepFunc("GrantPermission", w.GrantPermission),
		},
	}
}

// RegisterDemoWorkflow is the business-facing job type for the demo workflow.
// Services register it via workflow.Register:
//
//	workflow.Register(ctx, workflow.NewRegisterDemoWorkflow(traceID, payload))
//
// The fields are private because the Job interface accessors share their
// names (Go forbids a field and a method with the same name); the constructor
// keeps construction one expression.
type RegisterDemoWorkflow struct {
	traceId string
	payload DemoWorkflowInput
}

// NewRegisterDemoWorkflow builds a RegisterDemoWorkflow for the demo workflow.
func NewRegisterDemoWorkflow(traceId string, payload DemoWorkflowInput) RegisterDemoWorkflow {
	return RegisterDemoWorkflow{traceId: traceId, payload: payload}
}

func (RegisterDemoWorkflow) WorkflowName() string { return "demo" }
func (j RegisterDemoWorkflow) TraceId() string    { return j.traceId }
func (j RegisterDemoWorkflow) Payload() any       { return j.payload }

// ============================================================================
// Workflow
// ============================================================================

type demoWorkflow struct {
	userService UserService
}

// ============================================================================
// Payload
// ============================================================================

// DemoWorkflowInput is the state carried between workflow steps.
//
// Some fields are available when the workflow is registered.
// Other fields are populated by previous steps.
type DemoWorkflowInput struct {
	// Initial input
	Email    string
	Username string

	// Filled by RegisterUser()
	UserID int64
}

// ============================================================================
// Step 1
// ============================================================================

// RegisterUser creates the user.
//
// After this step succeeds:
//
//	payload.UserID
//
// is populated and persisted by the workflow executor.
//
// The next step can therefore use UserID directly.
func (w *demoWorkflow) RegisterUser(
	ctx context.Context,
	run *Run,
) error {

	payload, ok := run.Payload.(*DemoWorkflowInput)
	if !ok {
		return errors.New("invalid demo workflow payload")
	}

	// ------------------------------------------------------------------------
	// Validation
	// ------------------------------------------------------------------------

	if payload.Email == "" {
		return errors.New("email is required")
	}

	if payload.Username == "" {
		return errors.New("username is required")
	}

	// ------------------------------------------------------------------------
	// Idempotency
	// ------------------------------------------------------------------------

	// The worker may execute the same step again after a crash/retry.
	//
	// If UserID already exists, RegisterUser has already completed.
	if payload.UserID != 0 {
		return nil
	}

	// ------------------------------------------------------------------------
	// Domain operation
	// ------------------------------------------------------------------------

	out := w.userService.RegisterUser(
		ctx,
		&RegisterUserInput{
			Email:    payload.Email,
			Username: payload.Username,
		},
	)
	if !out.Success {
		return fmt.Errorf("register user: %s", out.Message)
	}

	// ------------------------------------------------------------------------
	// Update workflow state
	// ------------------------------------------------------------------------

	payload.UserID = out.User.ID

	return nil
}

// ============================================================================
// Step 2
// ============================================================================

// GrantPermission grants the required permission to the user created
// by RegisterUser().
func (w *demoWorkflow) GrantPermission(
	ctx context.Context,
	run *Run,
) error {

	payload, ok := run.Payload.(*DemoWorkflowInput)
	if !ok {
		return errors.New("invalid demo workflow payload")
	}

	// ------------------------------------------------------------------------
	// Validation
	// ------------------------------------------------------------------------

	if payload.UserID == 0 {
		return errors.New("user ID is required")
	}

	// ------------------------------------------------------------------------
	// Domain operation
	// ------------------------------------------------------------------------

	out := w.userService.GrantPermissionSystem(
		ctx,
		&GrantPermissionInput{
			TraceId:  run.TraceID,
			UserID:   payload.UserID,
			RoleName: demoRole,
		},
	)
	if !out.Success {
		return fmt.Errorf("grant permission: %s", out.Message)
	}

	return nil
}
