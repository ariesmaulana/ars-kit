# ARS Kit API

A REST API starter kit built with Go, Echo framework, and PostgreSQL.

## TLDR

### 1. Setup

```bash
git clone <repository-url>
cd ars-kit
go mod download

# copy and edit .env
cp .env.example .env
```

`.env` requires:
```env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASS=postgres
DB_NAME=your_database
JWT_SECRET=your_secret
```

Run:
```bash
make run
```

### 2. Migrate

Migrations live per-domain in `src/app/<domain>/sql/` using [goose](https://github.com/pressly/goose).

```bash
# apply all domains
make migrate-up

# apply single domain
make migrate-up user

# rollback one version
make migrate-down

# check status
make migrate-status

# create new migration
make migrate-create user add_roles
```

### 2.5 Bootstrap the first super user

There is no HTTP endpoint that creates an admin (it would be remotely
abusable). Instead, run the seed command once on a fresh deploy:

```bash
SUPERUSER_USERNAME=admin \
SUPERUSER_EMAIL=admin@example.com \
SUPERUSER_FULL_NAME="Admin" \
SUPERUSER_PASSWORD='change-me-now' \
make superuser
```

The command creates the account and grants it the `super_user` permission
(every permission, including user management). Re-running with the same
username is safe: the existing account is reused and the grant is repeated.
Then unset the `SUPERUSER_*` variables.

### 3. Admin user management

All admin endpoints require a `super_user` (JWT-authenticated) caller:

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/admin/users?page=1&page_size=10` | Paginated user list (`data` + `pagination`) |
| GET | `/api/v1/admin/users/:id` | Look up one user |
| POST | `/api/v1/admin/users/:id/deactivate` | Revoke the account's ability to log in (leavers) |
| POST | `/api/v1/admin/users/:id/reactivate` | Restore the ability to log in |

Deactivating an account blocks future logins (existing JWTs expire on their
own); an admin cannot deactivate their own account.

### 4. Test

```bash
# create test database
psql -U postgres -c "CREATE DATABASE go_test_your_app;"

# run all tests
make test
```

Each test gets an isolated PostgreSQL schema — no cleanup needed.

---

## Project Structure

```
ars-kit/
├── cmd/migrate/         # Migration CLI
├── config/              # Configuration
├── database/            # DB connection + migration registry
├── docs/                # Swagger docs
├── src/
│   ├── app/
│   │   └── user/        # User domain
│   │       ├── sql/     # Goose migrations
│   │       └── ...      # Handler, service, storage
│   └── main.go
└── testing/             # Test suite with DB isolation
```

## Tech Stack

- Go, Echo v4, PostgreSQL (pgx/v5), Goose (migrations), Zerolog, Swagger

## License

Not specified
