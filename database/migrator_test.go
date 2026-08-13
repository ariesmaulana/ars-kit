package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"testing/fstest"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// migrationTestDB opens a scratch connection for migration tests. It skips the
// test when no database is reachable so the suite still works on machines
// without Postgres (same convention as postgres_test.go).
func migrationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	env := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		env("DB_USER", "postgres"),
		env("DB_PASS", "postgres"),
		env("DB_HOST", "localhost"),
		env("DB_PORT", "5432"),
		env("DB_NAME", "go_test_your_app"),
	)
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skip("database not available, skipping migration isolation test")
	}
	return db
}

// TestDomainsTrackVersionsIndependently guards the per-domain goose version
// tables: every domain used to share one goose_db_version table, so a later
// domain whose migration version matched (or was older than) an earlier
// domain's was silently skipped or rejected as out-of-order.
func TestDomainsTrackVersionsIndependently(t *testing.T) {
	db := migrationTestDB(t)
	defer db.Close()

	ctx := context.Background()
	const schema = "migtest_isolation"

	_, err := db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema))
	require.NoError(t, err)
	defer db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))

	// Both domains ship a migration with the SAME version number. With a shared
	// version table the second one would be treated as already applied; with
	// per-domain tables both must run.
	alpha := Domain{
		Name:      "alpha",
		TableName: "goose_db_version_alpha",
		FS: fstest.MapFS{
			"sql/20240101_create_alpha.sql": &fstest.MapFile{Data: []byte(
				"-- +goose Up\nCREATE TABLE alpha_table (id int);\n\n-- +goose Down\nDROP TABLE alpha_table;",
			)},
		},
	}
	beta := Domain{
		Name:      "beta",
		TableName: "goose_db_version_beta",
		FS: fstest.MapFS{
			"sql/20240101_create_beta.sql": &fstest.MapFile{Data: []byte(
				"-- +goose Up\nCREATE TABLE beta_table (id int);\n\n-- +goose Down\nDROP TABLE beta_table;",
			)},
		},
	}

	_, err = db.ExecContext(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)

	require.NoError(t, Run(db, schema, []Domain{alpha, beta}))

	// Both side effects exist: the second domain's migration was NOT skipped.
	for _, table := range []string{"alpha_table", "beta_table"} {
		var n int
		err := db.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT count(*) FROM information_schema.tables WHERE table_schema = '%s' AND table_name = '%s'",
			schema, table,
		)).Scan(&n)
		require.NoError(t, err)
		assert.Equal(t, 1, n, "table %s should exist after migrations", table)
	}

	// Each domain records its own version history in its own table.
	for _, d := range []Domain{alpha, beta} {
		var version int64
		err := db.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT version_id FROM %s.%s WHERE version_id <> 0 ORDER BY version_id DESC LIMIT 1",
			schema, d.TableName,
		)).Scan(&version)
		require.NoError(t, err)
		assert.Equal(t, int64(20240101), version, "domain %s should record its own version", d.Name)
	}
}
