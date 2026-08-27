package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all main application configuration values.
type Config struct {
	AppName string
	AppEnv  string
	Port    string

	DBHost   string
	DBPort   string
	DBUser   string
	DBPass   string
	DBName   string
	DBSchema string

	// Database connection pool settings
	DBMaxConns          int32 // Maximum connections in pool (default: 25)
	DBMinConns          int32 // Minimum idle connections (default: 5)
	DBMaxConnLifetime   int   // Connection lifetime in minutes (default: 60)
	DBMaxConnIdleTime   int   // Idle connection timeout in minutes (default: 30)
	DBHealthCheckPeriod int   // Health check interval in seconds (default: 60)
	DBConnectTimeout    int   // Connection timeout in seconds (default: 5)

	// Workflow engine (PostgreSQL-backed background jobs)
	WorkflowWorkers         int // Worker goroutines (default: 3)
	WorkflowPollIntervalSec int // Idle poll delay in seconds (default: 5)
	WorkflowStaleTimeoutMin int // Processing lock lifetime in minutes (default: 3)
	WorkflowDrainTimeoutSec int // Shutdown drain wait in seconds (default: 30)
	WorkflowBatchSize       int // Jobs claimed per poll (default: 5)
	WorkflowStepTimeoutSec  int // Per-step execution timeout in seconds (default: 60)

	JWTSecret       string
	CORSAllowOrigin string

	// Per-account login throttling / lockout. A failed login increments a
	// persisted counter; failures older than LoginFailedWindowMinutes reset it;
	// reaching LoginMaxFailedAttempts locks the account for
	// LoginLockoutMinutes. The counter resets on a successful login.
	LoginMaxFailedAttempts   int // Consecutive failures before lockout (default: 5)
	LoginFailedWindowMinutes int // Counting window in minutes (default: 15)
	LoginLockoutMinutes      int // Lock duration in minutes (default: 15)

	// Email notification (foundation module)
	EmailProvider string // "smtp" (default), "resend", "brevo"
	SMTPHost      string // default: smtp.gmail.com
	SMTPPort      int    // default: 587
	SMTPUsername   string // required if provider=smtp
	SMTPPassword  string // Gmail: app password, not account password
	SMTPFrom      string // required if provider=smtp
	ResendAPIKey  string // required if provider=resend
	ResendFrom    string // required if provider=resend
	BrevoAPIKey   string // required if provider=brevo
	BrevoFrom     string // required if provider=brevo
}

// InitConfig loads configuration from .env file (if present) or OS environment.
// Priority: .env file > OS environment variables
func InitConfig() (*Config, error) {
	envs := loadDotEnv(".env")

	cfg := &Config{
		AppName: getEnv("APP_NAME", envs),
		AppEnv:  getEnv("APP_ENV", envs),
		Port:    getEnv("PORT", envs),

		DBHost:   getEnv("DB_HOST", envs),
		DBPort:   getEnv("DB_PORT", envs),
		DBUser:   getEnv("DB_USER", envs),
		DBPass:   getEnv("DB_PASS", envs),
		DBName:   getEnv("DB_NAME", envs),
		DBSchema: getEnv("DB_SCHEMA", envs),

		JWTSecret:       getEnv("JWT_SECRET", envs),
		CORSAllowOrigin: getEnv("CORS_ALLOW_ORIGIN", envs),
	}

	var errs []error

	var err error

	cfg.DBMaxConns, err = parseInt32Env("DB_MAX_CONNS", envs, 25)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DBMinConns, err = parseInt32Env("DB_MIN_CONNS", envs, 5)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DBMaxConnLifetime, err = parseIntEnv("DB_MAX_CONN_LIFETIME", envs, 60)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DBMaxConnIdleTime, err = parseIntEnv("DB_MAX_CONN_IDLE_TIME", envs, 30)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DBHealthCheckPeriod, err = parseIntEnv("DB_HEALTH_CHECK_PERIOD", envs, 60)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DBConnectTimeout, err = parseIntEnv("DB_CONNECT_TIMEOUT", envs, 5)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.WorkflowWorkers, err = parseIntEnv("WORKFLOW_WORKERS", envs, 3)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.WorkflowPollIntervalSec, err = parseIntEnv("WORKFLOW_POLL_INTERVAL", envs, 5)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.WorkflowStaleTimeoutMin, err = parseIntEnv("WORKFLOW_STALE_TIMEOUT", envs, 5)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.WorkflowDrainTimeoutSec, err = parseIntEnv("WORKFLOW_DRAIN_TIMEOUT", envs, 30)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.WorkflowBatchSize, err = parseIntEnv("WORKFLOW_BATCH_SIZE", envs, 5)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.WorkflowStepTimeoutSec, err = parseIntEnv("WORKFLOW_STEP_TIMEOUT", envs, 60)
	if err != nil {
		errs = append(errs, err)
	}

	cfg.LoginMaxFailedAttempts, err = parseIntEnv("LOGIN_MAX_FAILED_ATTEMPTS", envs, 5)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.LoginFailedWindowMinutes, err = parseIntEnv("LOGIN_FAILED_WINDOW_MINUTES", envs, 15)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.LoginLockoutMinutes, err = parseIntEnv("LOGIN_LOCKOUT_MINUTES", envs, 15)
	if err != nil {
		errs = append(errs, err)
	}

	// Email notification
	cfg.EmailProvider = getEnvOrDefault("EMAIL_PROVIDER", envs, "smtp")
	cfg.SMTPHost = getEnvOrDefault("SMTP_HOST", envs, "smtp.gmail.com")
	cfg.SMTPPort, err = parseIntEnv("SMTP_PORT", envs, 587)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.SMTPUsername = getEnv("SMTP_USERNAME", envs)
	cfg.SMTPPassword = getEnv("SMTP_PASSWORD", envs)
	cfg.SMTPFrom = getEnv("SMTP_FROM", envs)
	cfg.ResendAPIKey = getEnv("RESEND_API_KEY", envs)
	cfg.ResendFrom = getEnv("RESEND_FROM", envs)
	cfg.BrevoAPIKey = getEnv("BREVO_API_KEY", envs)
	cfg.BrevoFrom = getEnv("BREVO_FROM", envs)

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	requiredFields := map[string]string{
		"APP_NAME":          c.AppName,
		"DB_HOST":           c.DBHost,
		"DB_USER":           c.DBUser,
		"DB_NAME":           c.DBName,
		"JWT_SECRET":        c.JWTSecret,
		"CORS_ALLOW_ORIGIN": c.CORSAllowOrigin,
	}

	for field, value := range requiredFields {
		if value == "" {
			return errors.New("missing required config: " + field)
		}
	}

	if c.LoginMaxFailedAttempts <= 0 {
		return errors.New("invalid config: LOGIN_MAX_FAILED_ATTEMPTS must be positive")
	}
	if c.LoginFailedWindowMinutes <= 0 {
		return errors.New("invalid config: LOGIN_FAILED_WINDOW_MINUTES must be positive")
	}
	if c.LoginLockoutMinutes <= 0 {
		return errors.New("invalid config: LOGIN_LOCKOUT_MINUTES must be positive")
	}

	// Email notification: validate required fields per configured provider.
	switch c.EmailProvider {
	case "smtp":
		if c.SMTPUsername == "" {
			return errors.New("missing required config: SMTP_USERNAME")
		}
		if c.SMTPPassword == "" {
			return errors.New("missing required config: SMTP_PASSWORD")
		}
		if c.SMTPFrom == "" {
			return errors.New("missing required config: SMTP_FROM")
		}
	case "resend":
		if c.ResendAPIKey == "" {
			return errors.New("missing required config: RESEND_API_KEY")
		}
		if c.ResendFrom == "" {
			return errors.New("missing required config: RESEND_FROM")
		}
	case "brevo":
		if c.BrevoAPIKey == "" {
			return errors.New("missing required config: BREVO_API_KEY")
		}
		if c.BrevoFrom == "" {
			return errors.New("missing required config: BREVO_FROM")
		}
	default:
		return fmt.Errorf("invalid config: EMAIL_PROVIDER %q not in [smtp, resend, brevo]", c.EmailProvider)
	}

	return nil
}

func getEnv(key string, dotEnvMap map[string]string) string {
	// Priority 1: Check .env file first
	if val, ok := dotEnvMap[key]; ok && val != "" {
		return val
	}
	// Priority 2: Check OS environment variables
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return ""
}

// loadDotEnv loads a .env file into a map[string]string.
func loadDotEnv(filename string) map[string]string {
	envMap := make(map[string]string)
	file, err := os.Open(filename)
	if err != nil {
		return envMap
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		envMap[key] = value
	}
	return envMap
}

// parseIntEnv parses an integer environment variable with a default value.
// Returns error if the env var is set but not a valid integer.
func parseIntEnv(key string, dotEnvMap map[string]string, defaultValue int) (int, error) {
	valStr := getEnv(key, dotEnvMap)
	if valStr == "" {
		return defaultValue, nil
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue, fmt.Errorf("invalid %s value %q: %w", key, valStr, err)
	}
	return val, nil
}

// parseInt32Env parses an int32 environment variable with a default value.
// Returns error if the env var is set but not a valid integer.
func parseInt32Env(key string, dotEnvMap map[string]string, defaultValue int32) (int32, error) {
	valStr := getEnv(key, dotEnvMap)
	if valStr == "" {
		return defaultValue, nil
	}
	val, err := strconv.ParseInt(valStr, 10, 32)
	if err != nil {
		return defaultValue, fmt.Errorf("invalid %s value %q: %w", key, valStr, err)
	}
	return int32(val), nil
}

// getEnvOrDefault returns the value of key from dotEnvMap or OS env,
// falling back to defaultValue when the key is absent or empty.
func getEnvOrDefault(key string, dotEnvMap map[string]string, defaultValue string) string {
	if val := getEnv(key, dotEnvMap); val != "" {
		return val
	}
	return defaultValue
}
