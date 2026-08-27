package email

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewEmailSender constructs an EmailSender for the configured provider.
// An empty provider yields a no-op sender so deployments that do not send
// email yet pay no credential or runtime cost.
func NewEmailSender(cfg Config) (EmailSender, error) {
	switch cfg.Provider {
	case "":
		return &noopSender{}, nil
	case ProviderSMTP:
		return newSMTPSender(cfg.SMTP), nil
	case ProviderResend:
		return newResendSender(cfg.Resend, newHTTPClient()), nil
	case ProviderBrevo:
		return newBrevoSender(cfg.Brevo, newHTTPClient()), nil
	default:
		return nil, fmt.Errorf("notification/email: unknown provider %q", cfg.Provider)
	}
}

// noopSender discards everything. Used when EMAIL_PROVIDER is empty.
type noopSender struct{}

func (noopSender) SendText(context.Context, EmailMessage) error { return nil }
func (noopSender) SendHTML(context.Context, EmailMessage) error { return nil }

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// doREST executes a prepared request and maps the status to a SendError.
func doREST(ctx context.Context, client *http.Client, req *http.Request, provider string) error {
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return wrapError(ErrTimeout, provider, err)
		}
		return wrapError(ErrTransport, provider, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return wrapError(classifyHTTPStatus(resp.StatusCode), provider,
		fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(respBody))))
}

func classifyHTTPStatus(code int) error {
	switch {
	case code == 401 || code == 403:
		return ErrAuth
	case code == 429:
		return ErrRateLimited
	case code >= 400 && code < 500:
		return ErrBadRequest
	default:
		return ErrTransport
	}
}
