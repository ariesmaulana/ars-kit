package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

// Config menyimpan semua nilai konfigurasi utama aplikasi.
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

	JWTSecret       string
	CORSAllowOrigin string
}

// InitConfig membaca konfigurasi dari .env file (jika ada) atau OS environment.
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

		// Database pool configuration with defaults
		DBMaxConns:          parseInt32Env("DB_MAX_CONNS", envs, 25),
		DBMinConns:          parseInt32Env("DB_MIN_CONNS", envs, 5),
		DBMaxConnLifetime:   parseIntEnv("DB_MAX_CONN_LIFETIME", envs, 60),
		DBMaxConnIdleTime:   parseIntEnv("DB_MAX_CONN_IDLE_TIME", envs, 30),
		DBHealthCheckPeriod: parseIntEnv("DB_HEALTH_CHECK_PERIOD", envs, 60),
		DBConnectTimeout:    parseIntEnv("DB_CONNECT_TIMEOUT", envs, 5),

		JWTSecret:       getEnv("JWT_SECRET", envs),
		CORSAllowOrigin: getEnv("CORS_ALLOW_ORIGIN", envs),
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

// loadDotEnv membaca file .env ke dalam map[string]string.
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

// parseIntEnv parses an integer environment variable with a default value
func parseIntEnv(key string, dotEnvMap map[string]string, defaultValue int) int {
	valStr := getEnv(key, dotEnvMap)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}

// parseInt32Env parses an int32 environment variable with a default value
func parseInt32Env(key string, dotEnvMap map[string]string, defaultValue int32) int32 {
	valStr := getEnv(key, dotEnvMap)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.ParseInt(valStr, 10, 32)
	if err != nil {
		return defaultValue
	}
	return int32(val)
}
