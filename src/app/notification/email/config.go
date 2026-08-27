package email

// Provider identifies the transport backend.
type Provider string

const (
	// ProviderSMTP sends through an SMTP server (Gmail by default).
	ProviderSMTP Provider = "smtp"
	// ProviderResend sends through the Resend REST API.
	ProviderResend Provider = "resend"
	// ProviderBrevo sends through the Brevo REST API.
	ProviderBrevo Provider = "brevo"
)

// Config selects the active provider and holds every provider's settings.
type Config struct {
	Provider Provider
	SMTP     SMTPConfig
	Resend   ResendConfig
	Brevo    BrevoConfig
}

// SMTPConfig configures the net/smtp backend.
type SMTPConfig struct {
	Host     string // default smtp.gmail.com
	Port     int    // default 587
	Username string
	Password string // Gmail: app password, not account password
	From     string
}

// ResendConfig configures the Resend REST backend.
type ResendConfig struct {
	APIKey string
	From   string // must be a verified domain sender
}

// BrevoConfig configures the Brevo REST backend.
type BrevoConfig struct {
	APIKey string
	From   string
}