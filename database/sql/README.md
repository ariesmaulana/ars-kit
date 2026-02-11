# SQL Files for Testing

This directory contains SQL schema files that are used during testing.

## Usage

When setting up a test suite, register SQL files that should be executed:

```go
suite, err := testsuite.NewSuite(cfg, "todo.sql", "user.sql")
```

## How it works

1. Test suite creates a random isolated schema (e.g., `test_abc123`)
2. Connects to the database and selects the test schema
3. Executes registered SQL files in order (todo.sql, user.sql, etc.)
4. Runs the tests
5. Cleans up by dropping the schema after each test run

## Available SQL Files

- `todo.sql` - Creates the todos table
