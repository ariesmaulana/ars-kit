package database

import (
	"fmt"

	"github.com/ariesmaulana/ars-kit/config"
)

// DSN builds a PostgreSQL connection string from config.
// Falls back to "public" schema when DBSchema is empty.
func DSN(cfg *config.Config) string {
	return DSNWithSchema(cfg, cfg.DBSchema)
}

// DSNWithSchema builds a PostgreSQL connection string for the given schema.
// Falls back to "public" when schema is empty.
func DSNWithSchema(cfg *config.Config, schema string) string {
	if schema == "" {
		schema = "public"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName, schema,
	)
}
