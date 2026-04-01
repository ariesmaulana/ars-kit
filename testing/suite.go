package testing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/ariesmaulana/ars-kit/database"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Suite provides testing utilities with database isolation
type Suite struct {
	config    *config.Config
	pool      *pgxpool.Pool
	schema    string
	domains   []database.Domain
	beforeFns []func(*AppContext)
}

// AppContext holds initialized app components for testing
type AppContext struct {
	Pool *pgxpool.Pool
}

// NewSuite creates a new test suite instance
func NewSuite(cfg *config.Config, domains []database.Domain) (*Suite, error) {
	// Connect to database without schema isolation initially
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	s := &Suite{
		config:  cfg,
		pool:    pool,
		domains: domains,
	}

	return s, nil
}

// Describe starts a test group
func (s *Suite) Describe(t *testing.T, description string, fn func()) {
	t.Run(description, func(t *testing.T) {
		fn()
	})
}

// Before registers a function to run before each test scenario
// The function receives the AppContext to set up fixtures
func (s *Suite) Before(fn func(*AppContext)) {
	s.beforeFns = append(s.beforeFns, fn)
}

// Runs executes a test scenario with isolated database schema
func (s *Suite) Runs(t *testing.T, scenario string, fn func(t *testing.T, app *AppContext)) {
	t.Run(scenario, func(t *testing.T) {
		t.Parallel()

		// Create random schema for isolation
		schema := s.createRandomSchema()
		s.schema = schema

		// Create schema
		if err := s.createSchema(schema); err != nil {
			t.Fatalf("Failed to create schema: %v", err)
		}

		// Setup connection pool with schema
		schemaPool, err := s.createSchemaPool(schema)
		if err != nil {
			t.Fatalf("Failed to create schema pool: %v", err)
		}
		defer schemaPool.Close()

		// Run migrations via goose
		if err := s.runMigrations(schema); err != nil {
			t.Fatalf("Failed to run migrations: %v", err)
		}

		// Create app context
		app := &AppContext{
			Pool: schemaPool,
		}

		// Run before hooks with app context
		for _, beforeFn := range s.beforeFns {
			beforeFn(app)
		}

		// Run the actual test
		fn(t, app)

		// Cleanup: drop schema
		if err := s.dropSchema(schema); err != nil {
			t.Logf("Warning: Failed to drop schema %s: %v", schema, err)
		}
	})
}

// createRandomSchema generates a random schema name
func (s *Suite) createRandomSchema() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "test_" + hex.EncodeToString(b)
}

// createSchema creates a new schema in the database
func (s *Suite) createSchema(schema string) error {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	return err
}

// dropSchema drops a schema from the database
func (s *Suite) dropSchema(schema string) error {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
	return err
}

// createSchemaPool creates a new connection pool for a specific schema
func (s *Suite) createSchemaPool(schema string) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		s.config.DBUser,
		s.config.DBPass,
		s.config.DBHost,
		s.config.DBPort,
		s.config.DBName,
		schema,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return pool, nil
}

// runMigrations runs goose migrations into the isolated schema
func (s *Suite) runMigrations(schema string) error {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		s.config.DBUser,
		s.config.DBPass,
		s.config.DBHost,
		s.config.DBPort,
		s.config.DBName,
		schema,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open db for migrations: %w", err)
	}
	defer db.Close()

	return database.Run(db, schema, s.domains)
}

// Close closes the suite's database connection
func (s *Suite) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
