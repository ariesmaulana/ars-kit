package config

import (
	"bufio"
	"errors"
	"os"
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
