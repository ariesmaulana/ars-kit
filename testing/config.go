package testing

import (
	"github.com/ariesmaulana/ars-kit/config"
)

// InitTestConfig initializes configuration for testing
// Uses environment variables or defaults for test database
func InitTestConfig() *config.Config {
	cfg, err := config.InitConfig()
	if err != nil {
		// Fallback to defaults for testing
		cfg = &config.Config{
			AppName:  "MonthlyExpense",
			AppEnv:   "test",
			Port:     "8080",
			DBHost:   "localhost",
			DBPort:   "5432",
			DBUser:   "postgres",
			DBPass:   "postgres",
			DBName:   "go_test_your_app",
			DBSchema: "",
		}
	}
	return cfg
}
