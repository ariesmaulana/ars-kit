# Migration Guide

## Overview

Database migrations are managed with [Goose](https://github.com/pressly/goose) and embedded into the binary via `database/migrator.go`. Each domain owns its SQL files under `src/app/<domain>/sql/`, applied in order both in production (`make migrate-up`) and in tests (automatically, per isolated schema).

---

## File Naming Convention

Each migration uses a **Goose timestamp prefix** and the standard Goose markers:

```
src/app/<domain>/
  sql/
    20260318120000_initial_schema.sql
    20260318143022_add_avatar_url_to_users.sql
```

Inside each file:

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN avatar_url TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN avatar_url;
```

Rules:
- Version prefix is a timestamp (`YYYYMMDDHHmmss`) — `make migrate-create` generates it
- Name must be `snake_case`
- Each file must contain both `-- +goose Up` and `-- +goose Down` markers
- Down sections are currently left empty; create forward-only undo migrations instead when possible

---

## Creating a New Migration

```bash
make migrate-create NAME=your_description
```

Place the generated file under the relevant domain's `sql/` directory (e.g. `src/app/user/sql/`).

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

`database/migrator.go` embeds every `sql/*.sql` file under `src/app/*/sql/` at compile time. At startup `migrate-up` (or the `migrate-up` Make target) runs Goose against the `public` schema.

```
src/app/<domain>/sql/*.sql  →  database/migrator.go (embed)  →  Goose  →  production DB (public schema)
```

Each domain keeps its own Goose version table (`goose_db_version_<domain>`) so migrations are tracked independently and can be added without conflicts between domains.

---

## How It Works in Tests

The test suite (`testing/suite.go`) runs Goose migrations automatically for every test scenario:

1. A random schema is created: `test_<16 hex chars>`
2. Goose applies all embedded migrations into that schema
3. Per-domain `goose_db_version_*` tables are created inside the isolated schema
4. After the test, the schema is dropped entirely

**Tests always use the latest schema automatically** — no manual steps needed when you add a new migration.

---

## Dirty State Recovery

If a migration fails partway through, Goose marks the version **dirty** and future migrations are blocked.

To check:
```bash
make migrate-status
```

To fix:
1. Manually inspect and repair the partial change in the database
2. Force the version back to the last clean state:
   ```bash
   goose -dir src/app/<domain>/sql postgres $DATABASE_URL force <version>
   ```
