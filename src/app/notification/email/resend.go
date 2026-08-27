package email

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

const providerResend = "resend"

const resendBaseURL = "https://api.resend.com"

// resendSender delivers mail through the Resend REST API.
type resendSender struct {
	apiKey  string
	from    string
	client  *http.Client
	baseURL string
}

func newResendSender(cfg ResendConfig, client *http.Client) *resendSender {
	return &resendSender{
		apiKey:  cfg.APIKey,
		from:    cfg.From,
		client:  client,
		baseURL: resendBaseURL,
	}
}

func (s *resendSender) SendText(ctx context.Context, msg EmailMessage) error {
	return s.send(ctx, msg, false)
}

func (s *resendSender) SendHTML(ctx context.Context, msg EmailMessage) error {
	return s.send(ctx, msg, true)
}

func (s *resendSender) send(ctx context.Context, msg EmailMessage, withHTML bool) error {
	if err := validate(msg); err != nil {
		return err
	}
	from := resolveFrom(msg, s.from)

	payload := map[string]any{
		"from":    from,
		"to":      msg.To,
		"subject": msg.Subject,
		"text":    msg.Text,
	}
	if len(msg.Cc) > 0 {
		payload["cc"] = msg.Cc
	}
	if len(msg.Bcc) > 0 {
		payload["bcc"] = msg.Bcc
	}
	if withHTML && msg.HTML != "" {
		payload["html"] = msg.HTML
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return wrapError(ErrBadRequest, providerResend, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return wrapError(ErrBadRequest, providerResend, err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	return doREST(ctx, s.client, req, providerResend)
}
