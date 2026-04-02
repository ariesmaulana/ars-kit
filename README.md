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

### 3. Test

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
