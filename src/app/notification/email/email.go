package email

import (
	"context"
	"errors"
)

// Sentinel errors categorize why a send failed. Callers can branch on
// either form:
//
//	if errors.Is(err, email.ErrAuth) { ... }
//	if se, ok := err.(*email.SendError); ok && se.Code == email.ErrAuth { ... }
var (
	ErrAuth        = errors.New("auth")
	ErrRateLimited = errors.New("rate_limited")
	ErrBadRequest  = errors.New("bad_request")
	ErrTransport   = errors.New("transport")
	ErrTimeout     = errors.New("timeout")
)

// SendError wraps a failed send with a code, provider name, and cause.
type SendError struct {
	// Code is one of the package sentinel errors (ErrAuth, ...).
	Code     error
	Provider string
	Err      error
}

func (e *SendError) Error() string {
	if e.Provider != "" {
		return e.Provider + ": " + e.Err.Error()
	}
	return e.Err.Error()
}

func (e *SendError) Unwrap() error {
	return e.Err
}

// wrapError creates a SendError whose Code and unwrap chain carry the
// sentinel, so both errors.Is and .Code-based switching work.
func wrapError(code error, provider string, err error) error {
	if err == nil {
		return nil
	}
	return &SendError{
		Code:     code,
		Provider: provider,
		Err:      errors.Join(code, err),
	}
}

// EmailMessage represents an outbound email. Text is required; HTML is
// optional — when empty, the message is sent as text-only.
type EmailMessage struct {
	To      []string
	Cc      []string
	Bcc     []string
	From    string // optional; falls back to provider's configured default
	Subject string
	Text    string // required
	HTML    string // optional
}

// EmailSender sends email messages. Both methods are backed by the same
// internal send — SendText strips HTML, SendHTML sends multipart (or
// text-only when HTML is empty).
type EmailSender interface {
	SendText(ctx context.Context, msg EmailMessage) error
	SendHTML(ctx context.Context, msg EmailMessage) error
}

// resolveFrom returns msg.From if set, otherwise falls back to defaultFrom.
func resolveFrom(msg EmailMessage, defaultFrom string) string {
	if msg.From != "" {
		return msg.From
	}
	return defaultFrom
}

// validate checks the trust boundary: a message must address someone and
// carry a text body. Returns ErrBadRequest on violation.
func validate(msg EmailMessage) error {
	if len(msg.To) == 0 {
		return wrapError(ErrBadRequest, "", errors.New("email: no To recipient"))
	}
	if msg.Text == "" {
		return wrapError(ErrBadRequest, "", errors.New("email: Text body is required"))
	}
	return nil
}