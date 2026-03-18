# Migration Guide

## Overview

Database migrations are managed with [golang-migrate](https://github.com/golang-migrate/migrate). Migration files live in `database/migrations/` and are applied in order — both in production (via `make migrate-up`) and in tests (automatically, per isolated schema).

There are **no down migrations**. If you need to revert a change, create a new forward migration that undoes it. This keeps history linear and auditable.

---

## File Naming Convention

Each migration is a single SQL file using a **Unix timestamp prefix**:

```
database/migrations/
  20260318120000_initial_schema.up.sql
  20260318143022_add_avatar_url_to_users.up.sql
  20260318150811_drop_avatar_url_from_users.up.sql   ← revert by going forward
```

Rules:
- Version prefix is a Unix timestamp (`YYYYMMDDHHmmss`) — generated automatically by `make migrate-create`
- Name must be `snake_case`
- Files are applied in ascending version order (oldest timestamp first)

**Why timestamps instead of sequential numbers?**

Sequential numbers (`0001`, `0002`, …) cause conflicts when two developers create migrations on separate branches at the same time — both get the same number. Timestamps are unique per second, making conflicts practically impossible without any coordination needed.

---

## Creating a New Migration

```bash
make migrate-create NAME=your_description
```

Example:

```bash
make migrate-create NAME=add_avatar_url_to_users
# Created: database/migrations/0002_add_avatar_url_to_users.up.sql
```

Then edit the generated file with your SQL.

---

## Running Migrations

### Apply all pending migrations

```bash
make migrate-up
```

### Check current version

```bash
make migrate-status
```

---

## How It Works in Production

`cmd/migrate/main.go` reads the same `.env` / environment variables as the app and connects to the production database. It targets the `public` schema.

```
database/migrations/*.up.sql  →  make migrate-up  →  production DB (public schema)
```

The `schema_migrations` table is created automatically to track which migrations have been applied.

---

## How It Works in Tests

The test suite (`testing/suite.go`) runs migrations automatically for every test scenario:

1. A random schema is created: `test_<16 hex chars>`
2. golang-migrate runs all `.up.sql` files into that schema (`search_path=<schema>`)
3. A `schema_migrations` table is created inside that isolated schema
4. After the test, the schema is dropped entirely

**Tests always use the latest schema automatically** — no manual steps needed when you add a new migration.

```
database/migrations/*.up.sql  →  suite.Runs()  →  isolated test schema (per test)
```

---

## Dirty State Recovery

If a migration fails partway through, the database is marked **dirty** and future migrations are blocked.

To check:
```bash
make migrate-status
# Output: Dirty: true
```

To fix:
1. Manually inspect and repair the partial change in the database
2. Force the version back to the last clean state:
   ```bash
   go run ./cmd/migrate force <version>
   ```
   Example: `go run ./cmd/migrate force 1`

---

## Schema Migrations Table

golang-migrate creates a `schema_migrations` table with two columns:

| Column    | Type    | Description                        |
|-----------|---------|------------------------------------|
| `version` | bigint  | Migration version number           |
| `dirty`   | boolean | `true` if migration failed midway  |

Do not modify this table manually unless recovering from a dirty state.
