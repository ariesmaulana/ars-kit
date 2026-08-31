package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/ariesmaulana/ars-kit/src/app/notification/email"
)

// ============================================================================
// Domain seam
// ============================================================================

// EmailService is the email-domain seam this workflow depends on. It is
// satisfied by the notification/email.EmailSender implementation.
type EmailService interface {
	SendText(ctx context.Context, msg email.EmailMessage) error
}

// ============================================================================
// SendEmail Workflow
// ============================================================================

// SendEmailWorkflow creates the workflow definition for sending a single
// email via the configured provider. The job is fire-and-forget: a failed
// send is retried up to MaxRetries times, then silently dropped.
func SendEmailWorkflow(sender EmailService) *Definition {
	w := &sendEmailWorkflow{sender: sender}

	return &Definition{
		Name:       "send_email",
		MaxRetries: 3,

		NewPayload: func() any {
			return &SendEmailPayload{}
		},

		Steps: []Step{
			StepFunc("SendEmail", w.SendEmail),
		},
	}
}

// ============================================================================
// Job type
// ============================================================================

// SendEmailJob is the business-facing job type for the send-email workflow.
type SendEmailJob struct {
	traceId string
	payload SendEmailPayload
}

// NewSendEmailJob builds a SendEmailJob from an email.EmailMessage. The
// constructor converts the message into the workflow's JSON-serialisable
// payload struct.
func NewSendEmailJob(traceId string, msg email.EmailMessage) SendEmailJob {
	return SendEmailJob{
		traceId: traceId,
		payload: SendEmailPayload{
			To:      msg.To,
			Cc:      msg.Cc,
			Bcc:     msg.Bcc,
			From:    msg.From,
			Subject: msg.Subject,
			Text:    msg.Text,
			HTML:    msg.HTML,
		},
	}
}

func (SendEmailJob) WorkflowName() string { return "send_email" }
func (j SendEmailJob) TraceId() string    { return j.traceId }
func (j SendEmailJob) Payload() any       { return j.payload }

// ============================================================================
// Payload
// ============================================================================

// SendEmailPayload carries the outbound email through the workflow system.
type SendEmailPayload struct {
	To      []string
	Cc      []string
	Bcc     []string
	From    string
	Subject string
	Text    string
	HTML    string
}

// ============================================================================
// Step
// ============================================================================

type sendEmailWorkflow struct {
	sender EmailService
}

// SendEmail delivers the email via the configured provider.
func (w *sendEmailWorkflow) SendEmail(ctx context.Context, run *Run) error {
	payload, ok := run.Payload.(*SendEmailPayload)
	if !ok {
		return errors.New("invalid send_email workflow payload")
	}

	msg := email.EmailMessage{
		To:      payload.To,
		Cc:      payload.Cc,
		Bcc:     payload.Bcc,
		From:    payload.From,
		Subject: payload.Subject,
		Text:    payload.Text,
		HTML:    payload.HTML,
	}

	if err := w.sender.SendText(ctx, msg); err != nil {
		log.Err(err).
			Str("trace_id", run.TraceID).
			Strs("to", msg.To).
			Str("subject", msg.Subject).
			Msg("workflow send_email: send failed")
		return fmt.Errorf("send email: %w", err)
	}

	log.Info().
		Str("trace_id", run.TraceID).
		Strs("to", msg.To).
		Str("subject", msg.Subject).
		Msg("workflow send_email: sent")
	return nil
}
