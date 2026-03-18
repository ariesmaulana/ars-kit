package testing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Suite provides testing utilities with database isolation
type Suite struct {
	config    *config.Config
	pool      *pgxpool.Pool
	schema    string
	beforeFns []func(*AppContext)
}

// AppContext holds initialized app components for testing
type AppContext struct {
	Pool *pgxpool.Pool
}

// NewSuite creates a new test suite instance
func NewSuite(cfg *config.Config) (*Suite, error) {
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
		config: cfg,
		pool:   pool,
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

		// Run migrations via golang-migrate
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

// runMigrations runs golang-migrate migrations into the isolated schema
func (s *Suite) runMigrations(schema string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	migrationsPath := "file://" + filepath.Join(projectRoot, "database", "migrations")

	// pgx/v5 driver DSN with search_path to target the isolated schema
	dsn := fmt.Sprintf(
		"pgx5://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		s.config.DBUser,
		s.config.DBPass,
		s.config.DBHost,
		s.config.DBPort,
		s.config.DBName,
		schema,
	)

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("failed to initialize migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// findProjectRoot walks up directories until go.mod is found
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}

// Close closes the suite's database connection
func (s *Suite) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
