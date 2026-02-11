package database

import (
	"context"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresDB(t *testing.T) {
	t.Run("should create postgres connection with default schema", func(t *testing.T) {
		cfg := &config.Config{
			DBHost: "localhost",
			DBPort: "5432",
			DBUser: "postgres",
			DBPass: "postgres",
			DBName: "test_db",
		}

		db, err := NewPostgresDB(cfg)
		if err != nil {
			t.Skip("Database not available, skipping test")
		}
		defer db.Close()

		assert.NotNil(t, db)
		assert.NotNil(t, db.Pool)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = db.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("should create postgres connection with custom schema", func(t *testing.T) {
		cfg := &config.Config{
			DBHost:   "localhost",
			DBPort:   "5432",
			DBUser:   "postgres",
			DBPass:   "postgres",
			DBName:   "test_db",
			DBSchema: "custom_schema",
		}

		db, err := NewPostgresDB(cfg)
		if err != nil {
			t.Skip("Database not available, skipping test")
		}
		defer db.Close()

		assert.NotNil(t, db)
		assert.NotNil(t, db.Pool)
	})

	t.Run("should return error for invalid connection", func(t *testing.T) {
		cfg := &config.Config{
			DBHost: "invalid_host",
			DBPort: "9999",
			DBUser: "invalid_user",
			DBPass: "invalid_pass",
			DBName: "invalid_db",
		}

		db, err := NewPostgresDB(cfg)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func TestPostgresDB_Close(t *testing.T) {
	t.Run("should close connection pool gracefully", func(t *testing.T) {
		cfg := &config.Config{
			DBHost: "localhost",
			DBPort: "5432",
			DBUser: "postgres",
			DBPass: "postgres",
			DBName: "test_db",
		}

		db, err := NewPostgresDB(cfg)
		if err != nil {
			t.Skip("Database not available, skipping test")
		}

		require.NotNil(t, db)
		db.Close()
	})

	t.Run("should handle nil pool gracefully", func(t *testing.T) {
		db := &PostgresDB{Pool: nil}
		assert.NotPanics(t, func() {
			db.Close()
		})
	})
}

func TestPostgresDB_Ping(t *testing.T) {
	t.Run("should ping successfully", func(t *testing.T) {
		cfg := &config.Config{
			DBHost: "localhost",
			DBPort: "5432",
			DBUser: "postgres",
			DBPass: "postgres",
			DBName: "test_db",
		}

		db, err := NewPostgresDB(cfg)
		if err != nil {
			t.Skip("Database not available, skipping test")
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = db.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("should timeout on context cancellation", func(t *testing.T) {
		cfg := &config.Config{
			DBHost: "localhost",
			DBPort: "5432",
			DBUser: "postgres",
			DBPass: "postgres",
			DBName: "test_db",
		}

		db, err := NewPostgresDB(cfg)
		if err != nil {
			t.Skip("Database not available, skipping test")
		}
		defer db.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = db.Ping(ctx)
		assert.Error(t, err)
	})
}
