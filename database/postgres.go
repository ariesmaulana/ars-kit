package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type PostgresDB struct {
	Pool *pgxpool.Pool
}

// NewPostgresDB creates a new PostgreSQL connection pool with best practices
func NewPostgresDB(cfg *config.Config) (*PostgresDB, error) {
	// Set schema to public if not provided
	schema := cfg.DBSchema
	if schema == "" {
		schema = "public"
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
		schema,
	)

	// Parse the connection string and configure pool settings
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Connection pool configuration optimized for pgBouncer
	// When using pgBouncer in transaction mode, keep these conservative
	poolConfig.MaxConns = 25                      // Maximum connections in pool
	poolConfig.MinConns = 5                       // Minimum idle connections
	poolConfig.MaxConnLifetime = time.Hour        // Connection lifetime
	poolConfig.MaxConnIdleTime = 30 * time.Minute // Idle connection timeout
	poolConfig.HealthCheckPeriod = time.Minute    // Health check interval
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second

	// Create the connection pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Verify connection with ping
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Info().
		Str("host", cfg.DBHost).
		Str("port", cfg.DBPort).
		Str("database", cfg.DBName).
		Str("schema", schema).
		Msg("Successfully connected to PostgreSQL")

	return &PostgresDB{Pool: pool}, nil
}

// Close gracefully closes the database connection pool
func (db *PostgresDB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		log.Info().Msg("Database connection pool closed")
	}
}

// Ping checks if the database connection is alive
func (db *PostgresDB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
