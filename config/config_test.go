package config_test

import (
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
