package email

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

const providerBrevo = "brevo"

const brevoBaseURL = "https://api.brevo.com/v3"

// brevoSender delivers mail through the Brevo REST API.
type brevoSender struct {
	apiKey  string
	from    string
	client  *http.Client
	baseURL string
}

func newBrevoSender(cfg BrevoConfig, client *http.Client) *brevoSender {
	return &brevoSender{
		apiKey:  cfg.APIKey,
		from:    cfg.From,
		client:  client,
		baseURL: brevoBaseURL,
	}
}

func (s *brevoSender) SendText(ctx context.Context, msg EmailMessage) error {
	return s.send(ctx, msg, false)
}

func (s *brevoSender) SendHTML(ctx context.Context, msg EmailMessage) error {
	return s.send(ctx, msg, true)
}

func (s *brevoSender) send(ctx context.Context, msg EmailMessage, withHTML bool) error {
	if err := validate(msg); err != nil {
		return err
	}
	from := resolveFrom(msg, s.from)

	type recipient struct {
		Email string `json:"email"`
		Name  string `json:"name,omitempty"`
	}

	payload := map[string]any{
		"sender":      map[string]string{"email": from},
		"to":          toRecipients(msg.To),
		"subject":     msg.Subject,
		"textContent": msg.Text,
	}
	if len(msg.Cc) > 0 {
		payload["cc"] = toRecipients(msg.Cc)
	}
	if len(msg.Bcc) > 0 {
		payload["bcc"] = toRecipients(msg.Bcc)
	}
	if withHTML && msg.HTML != "" {
		payload["htmlContent"] = msg.HTML
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return wrapError(ErrBadRequest, providerBrevo, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/smtp/email", bytes.NewReader(body))
	if err != nil {
		return wrapError(ErrBadRequest, providerBrevo, err)
	}
	req.Header.Set("api-key", s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	return doREST(ctx, s.client, req, providerBrevo)
}

// toRecipients maps a flat email slice to Brevo's [{email}] shape.
func toRecipients(addrs []string) []map[string]string {
	out := make([]map[string]string, len(addrs))
	for i, addr := range addrs {
		out[i] = map[string]string{"email": addr}
	}
	return out
}
