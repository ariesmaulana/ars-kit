# Testing Guide

## Overview

Tests use **real PostgreSQL** with schema-level isolation. Each test scenario gets its own randomly-named schema (e.g., `test_a1b2c3d4`), runs the full migration stack against it, then drops the schema after the test completes. This ensures:

- Tests always run against the **latest schema** (same migration files as production)
- Tests are **fully isolated** — no cross-test data leakage
- Tests can run **in parallel** safely

---

## Prerequisites

- PostgreSQL running locally (default: `localhost:5432`)
- A test database created (default: `go_test_your_app`)
- Go 1.21+

### Create the test database

```sql
CREATE DATABASE go_test_your_app;
```

Or via psql:

```bash
psql -U postgres -c "CREATE DATABASE go_test_your_app;"
```

---

## Configuration

Tests read from environment variables (same as the app). If none are set, they fall back to these defaults:

| Variable   | Default             |
|------------|---------------------|
| `DB_HOST`  | `localhost`         |
| `DB_PORT`  | `5432`              |
| `DB_USER`  | `postgres`          |
| `DB_PASS`  | `postgres`          |
| `DB_NAME`  | `go_test_your_app`  |

To override, set env vars or create a `.env` file before running tests:

```bash
export DB_HOST=localhost
export DB_NAME=my_test_db
```

---

## Running Tests

```bash
# Run all tests
make test

# Run with verbose output
make test-verbose

# Run with coverage report (outputs coverage.html)
make test-coverage

# Run a specific package
go test ./src/app/user/... -count=1

# Run a specific test
go test ./src/app/user/... -run TestStorageInsertUser -count=1 -v
```

---

## How Isolation Works

Each call to `suite.Runs()` or `suite.Run()`:

1. Generates a random schema name: `test_<16 hex chars>`
2. Creates the schema in the test database
3. Runs all migration files from `database/migrations/` into that schema
4. Executes the test function
5. Drops the schema (cleanup)

This means **every test scenario starts from a clean, fully-migrated database**.

---

## Writing Tests

### Basic pattern

```go
func TestMyFeature(t *testing.T) {
    RunTest(t, func(t *testing.T, suite *TestSuite) {
        suite.Describe(t, "My Feature", func() {
            suite.Run(t, "should do something", func(t *testing.T, ctx context.Context, app *UserApp) {
                // arrange
                user := app.Helper.InsertUser(ctx, t, "alice", "alice@example.com", "Alice", "pw")

                // act
                result, err := app.Service.SomeMethod(ctx, ...)

                // assert
                assert.Nil(t, err)
                assert.Equal(t, expected, result)
            })
        })
    })
}
```

### Using setup hooks

```go
func TestWithSetup(t *testing.T) {
    RunTest(t, func(t *testing.T, suite *TestSuite) {
        // Runs before each scenario in this suite
        suite.Setup(func(ctx context.Context, app *UserApp) {
            app.Helper.InsertUser(ctx, t, "seed-user", "seed@example.com", "Seed", "pw")
        })

        suite.Describe(t, "Feature with seed data", func() {
            suite.Run(t, "scenario 1", func(t *testing.T, ctx context.Context, app *UserApp) {
                // seed-user already exists here
            })
        })
    })
}
```



---

## Adding a New Migration and Keeping Tests in Sync

Tests automatically pick up new migrations — no test code changes needed. When you add a new migration file:

1. Run `make migrate-create NAME=description` to generate the numbered file
2. Write your SQL in the generated file
3. Run `make test` — each test schema will include the new migration automatically

See [migration-guide.md](migration-guide.md) for migration workflow details.
