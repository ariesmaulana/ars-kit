package config_test

import (
	"strings"
	"testing"

	"github.com/ariesmaulana/ars-kit/config"
)

// setRequiredEnv sets the env vars InitConfig validates as required, so tests
// can focus on the login-throttle keys. t.Setenv forbids parallel tests, and
// these do not use t.Parallel.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"APP_NAME":          "ars-kit",
		"DB_HOST":           "localhost",
		"DB_USER":           "postgres",
		"DB_NAME":           "ars_kit_test",
		"JWT_SECRET":        "test-secret",
		"CORS_ALLOW_ORIGIN": "http://localhost:3000",
	} {
		t.Setenv(k, v)
	}
}

func TestInitConfigLoginThrottleDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.LoginMaxFailedAttempts != 5 {
		t.Errorf("LoginMaxFailedAttempts = %d, want default 5", cfg.LoginMaxFailedAttempts)
	}
	if cfg.LoginFailedWindowMinutes != 15 {
		t.Errorf("LoginFailedWindowMinutes = %d, want default 15", cfg.LoginFailedWindowMinutes)
	}
	if cfg.LoginLockoutMinutes != 15 {
		t.Errorf("LoginLockoutMinutes = %d, want default 15", cfg.LoginLockoutMinutes)
	}
}

func TestInitConfigLoginThrottleCustom(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LOGIN_MAX_FAILED_ATTEMPTS", "7")
	t.Setenv("LOGIN_FAILED_WINDOW_MINUTES", "30")
	t.Setenv("LOGIN_LOCKOUT_MINUTES", "60")

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.LoginMaxFailedAttempts != 7 {
		t.Errorf("LoginMaxFailedAttempts = %d, want 7", cfg.LoginMaxFailedAttempts)
	}
	if cfg.LoginFailedWindowMinutes != 30 {
		t.Errorf("LoginFailedWindowMinutes = %d, want 30", cfg.LoginFailedWindowMinutes)
	}
	if cfg.LoginLockoutMinutes != 60 {
		t.Errorf("LoginLockoutMinutes = %d, want 60", cfg.LoginLockoutMinutes)
	}
}

func TestInitConfigWorkflowStepTimeoutDefault(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.WorkflowStepTimeoutSec != 60 {
		t.Errorf("WorkflowStepTimeoutSec = %d, want default 60", cfg.WorkflowStepTimeoutSec)
	}
}

func TestInitConfigWorkflowStepTimeoutCustom(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("WORKFLOW_STEP_TIMEOUT", "15")

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.WorkflowStepTimeoutSec != 15 {
		t.Errorf("WorkflowStepTimeoutSec = %d, want 15", cfg.WorkflowStepTimeoutSec)
	}
}

func TestInitConfigWorkflowStepTimeoutInvalid(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("WORKFLOW_STEP_TIMEOUT", "soon")

	if _, err := config.InitConfig(); err == nil {
		t.Fatal("InitConfig() error = nil, want an error for non-numeric WORKFLOW_STEP_TIMEOUT")
	}
}

func TestInitConfigEmailDisabledByDefault(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.EmailProvider != "" {
		t.Errorf("EmailProvider = %q, want empty (disabled)", cfg.EmailProvider)
	}
	// SMTP host/port still loaded from env (or defaults) even when disabled.
	if cfg.SMTPHost != "smtp.gmail.com" {
		t.Errorf("SMTPHost = %q, want default smtp.gmail.com", cfg.SMTPHost)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want default 587", cfg.SMTPPort)
	}
}

func TestInitConfigEmailSMTPWithCreds(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("EMAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_USERNAME", "user@test.com")
	t.Setenv("SMTP_PASSWORD", "pass")
	t.Setenv("SMTP_FROM", "from@test.com")

	cfg, err := config.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}
	if cfg.EmailProvider != "smtp" {
		t.Errorf("EmailProvider = %q, want smtp", cfg.EmailProvider)
	}
}

func TestInitConfigEmailSMTPMissingCreds(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("EMAIL_PROVIDER", "smtp")

	_, err := config.InitConfig()
	if err == nil {
		t.Fatal("InitConfig() error = nil, want error for missing SMTP_USERNAME")
	}
}

func TestInitConfigEmailProviderValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string // error substring, empty = success
	}{
		{
			name: "resend ok",
			env:  map[string]string{"EMAIL_PROVIDER": "resend", "RESEND_API_KEY": "k", "RESEND_FROM": "f@e.com"},
		},
		{
			name: "resend missing key",
			env:  map[string]string{"EMAIL_PROVIDER": "resend", "RESEND_FROM": "f@e.com"},
			want: "RESEND_API_KEY",
		},
		{
			name: "brevo ok",
			env:  map[string]string{"EMAIL_PROVIDER": "brevo", "BREVO_API_KEY": "k", "BREVO_FROM": "f@e.com"},
		},
		{
			name: "brevo missing from",
			env:  map[string]string{"EMAIL_PROVIDER": "brevo", "BREVO_API_KEY": "k"},
			want: "BREVO_FROM",
		},
		{
			name: "unknown provider",
			env:  map[string]string{"EMAIL_PROVIDER": "ses"},
			want: "EMAIL_PROVIDER",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredEnv(t)
			// Strip default smtp creds so only the provider under test matters.
			t.Setenv("SMTP_USERNAME", "")
			t.Setenv("SMTP_PASSWORD", "")
			t.Setenv("SMTP_FROM", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := config.InitConfig()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("InitConfig() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("InitConfig() error = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("InitConfig() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestInitConfigLoginThrottleInvalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{
			name: "zero max failed attempts",
			env:  map[string]string{"LOGIN_MAX_FAILED_ATTEMPTS": "0"},
		},
		{
			name: "zero failed window",
			env:  map[string]string{"LOGIN_FAILED_WINDOW_MINUTES": "0"},
		},
		{
			name: "zero lockout duration",
			env:  map[string]string{"LOGIN_LOCKOUT_MINUTES": "0"},
		},
		{
			name: "non-numeric value",
			env:  map[string]string{"LOGIN_MAX_FAILED_ATTEMPTS": "many"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := config.InitConfig()
			if err == nil {
				t.Fatalf("InitConfig() error = nil, want an error for %s", tc.name)
			}
		})
	}
}
