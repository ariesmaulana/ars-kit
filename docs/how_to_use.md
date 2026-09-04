# How To Use — ars-kit

How to install, migrate, and run the app (HTTP server + workflow worker), and
what external services/keys it needs. Everything configurable lives in
`.env` — start from `.env.example`.

---

## 1. Install

Requirements:

- **Go 1.25+** (see `go.mod`)
- **PostgreSQL 14+** reachable from the machine running the app
- (Optional) object storage (S3 / Cloudflare R2) and an email provider — see
  [Requirements](#4-requirements) below

```bash
# clone
git clone <repository-url>
cd ars-kit

# install Go dependencies
go mod download
go mod tidy

# copy and edit the environment file
cp .env.example .env
vim .env

# run the HTTP server (dev)
make run
# or: go run src/main.go serve
```

The server listens on `PORT` (default from `.env`, e.g. `8181`). Swagger docs
are generated under `docs/` (`make swagger`).

Build a production binary:

```bash
make build        # binary for the current OS  -> bin/ars-kit
make build-linux  # linux/amd64 -> bin/ars-kit-linux-amd64
```

---

## 2. Migrate

Migrations are goose files embedded per domain in
`src/app/<domain>/sql/` and each domain tracks its own version history
table. Domains (in dependency order): `user`, `permission`, `workflow`.

```bash
# apply ALL domains
make migrate-up

# apply a single domain
make migrate-up user        # permission, workflow work the same

# roll back one version (all domains or a specific one)
make migrate-down
make migrate-down user

# show applied/pending versions
make migrate-status
make migrate-status user

# create a new migration file for a domain
make migrate-create user add_roles
# creates src/app/user/sql/<timestamp>_<name>.sql
```

Raw commands (same behavior):

```bash
go run ./cmd/migrate up          # all domains
go run ./cmd/migrate up user     # only the user domain
go run ./cmd/migrate down        # rollback one version
go run ./cmd/migrate status
go run ./cmd/migrate create user add_roles
```

Migrations connect using the `DB_*` variables in `.env` and run inside the
schema named by `DB_SCHEMA` (default `public`). 

---

## 3. Run the worker

The workflow engine (PostgreSQL-backed background jobs) executes everything
the user module queues asynchronously:

```bash
make worker
# or: go run src/main.go worker
```

Run one worker process per machine/environment. A worker and the server can
share one binary (same `buildApp` wiring) — start two processes:

```bash
./bin/ars-kit serve
./bin/ars-kit worker
```

Worker behavior is tuned in `.env`:

| Variable | Default | Meaning |
|---|---|---|
| `WORKFLOW_WORKERS` | 3 | worker goroutines per process |
| `WORKFLOW_POLL_INTERVAL` | 5 (s) | idle poll delay, also the de facto retry delay |
| `WORKFLOW_STALE_TIMEOUT` | 3 (min) | reclaim jobs stuck in `processing` (must exceed batch × slowest step) |
| `WORKFLOW_DRAIN_TIMEOUT` | 30 (s) | graceful-shutdown wait for in-flight steps |
| `WORKFLOW_BATCH_SIZE` | 5 | jobs claimed per poll |
| `WORKFLOW_STEP_TIMEOUT` | 60 (s) | per-step execution timeout (must be < `WORKFLOW_STALE_TIMEOUT`) |

---

## 4. Requirements

### 4.1 Database (PostgreSQL) — required

| Variable | Required | Notes |
|---|---|---|
| `DB_HOST` | yes | host |
| `DB_PORT` | yes | usually `5432` |
| `DB_USER` | yes | |
| `DB_PASS` | yes | |
| `DB_NAME` | yes | database must already exist (e.g. `CREATE DATABASE ars_kit;`) |
| `DB_SCHEMA` | no | defaults to `public` |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | no | pool sizing (defaults 25 / 5) |
| `DB_MAX_CONN_LIFETIME` / `DB_MAX_CONN_IDLE_TIME` | no | minutes (defaults 60 / 30) |

The workflow engine is PostgreSQL-backed too — no separate queue service.

### 4.2 App keys

| Variable | Required | Notes |
|---|---|---|
| `JWT_SECRET` | yes | secret used to sign/verify JWTs. Use a long random value (`openssl rand -base64 32`) and **never** commit it. Changing it invalidates all tokens. |
| `PORT` | no | HTTP port |
| `APP_ENV` | no | `production` makes cookies `Secure` and enables prod behavior |
| `CORS_ALLOW_ORIGIN` | yes* | your frontend origin (`http://localhost:3000` in dev). Do **not** use `*`. |
| `APP_URL` | no* | frontend base URL embedded in email links (reset/verify). Defaults to `http://localhost:3000`. |
| `EMAIL_TOKEN_EXPIRY_HOURS` | no | lifetime of reset/verify tokens (default 24 h) |

\* required for that feature to work end-to-end (CORS for browsers, `APP_URL`
for usable email links).

### 4.3 Email provider — optional (needed for forgot-password / email verification)

| Variable | Required | Notes |
|---|---|---|
| `EMAIL_PROVIDER` | — | empty = email **disabled** (reset/verify flows have no sender). One of `smtp`, `resend`, `brevo`. |

**SMTP** (`EMAIL_PROVIDER=smtp`) — e.g. Gmail app password:

| Variable | Notes |
|---|---|
| `SMTP_HOST` | e.g. `smtp.gmail.com` |
| `SMTP_PORT` | e.g. `587` |
| `SMTP_USERNAME` | e.g. `you@gmail.com` |
| `SMTP_PASSWORD` | Gmail: an **app password**, not the account password |
| `SMTP_FROM` | sender address, e.g. `noreply@yourdomain.com` |

**Resend** (`EMAIL_PROVIDER=resend`):

| Variable | Notes |
|---|---|
| `RESEND_API_KEY` | Resend API key |
| `RESEND_FROM` | verified sender, e.g. `noreply@yourdomain.com` |

**Brevo** (`EMAIL_PROVIDER=brevo`):

| Variable | Notes |
|---|---|
| `BREVO_API_KEY` | Brevo (Sendinblue) API key |
| `BREVO_FROM` | sender address |

### 4.4 Avatar storage — optional (needed for avatar upload)

Backend chosen by `UPLOAD_STORAGE`: `local` (default, dev) or `s3`.

**Local:**

| Variable | Notes |
|---|---|
| `UPLOAD_STORAGE=local` | |
| `UPLOAD_LOCAL_BASE_DIR` | e.g. `./storage/avatars` (created on demand) |

**S3 — AWS S3** (`UPLOAD_STORAGE=s3`):

| Variable | Notes |
|---|---|
| `UPLOAD_S3_BUCKET` | bucket name |
| `UPLOAD_S3_REGION` | e.g. `ap-southeast-1` |
| `UPLOAD_S3_ENDPOINT` | leave **empty** to use the AWS default endpoint |
| `UPLOAD_S3_ACCESS_KEY_ID` | IAM access key with `s3:PutObject` / `s3:DeleteObject` on the bucket |
| `UPLOAD_S3_SECRET_ACCESS_KEY` | matching secret |
| `UPLOAD_S3_PREFIX` | key prefix, e.g. `avatars` |
| `UPLOAD_S3_USE_PATH_STYLE` | `false` for AWS S3 |

**S3 — Cloudflare R2** (`UPLOAD_STORAGE=s3`, same S3 client):

| Variable | Notes |
|---|---|
| `UPLOAD_S3_BUCKET` | R2 bucket name |
| `UPLOAD_S3_REGION` | `auto` |
| `UPLOAD_S3_ENDPOINT` | `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` (create an R2 API token in the Cloudflare dashboard) |
| `UPLOAD_S3_ACCESS_KEY_ID` | R2 token access key ID |
| `UPLOAD_S3_SECRET_ACCESS_KEY` | R2 token secret |
| `UPLOAD_S3_PREFIX` | e.g. `avatars` |
| `UPLOAD_S3_USE_PATH_STYLE` | `true` for R2/MinIO |

Accepted avatar images: `jpeg` / `png` / `webp`, max **2 MB** (enforced in
`src/main.go`).

---

## 5. Quick start checklist

```bash
# 1. environment
cp .env.example .env
#    set: DB_*, JWT_SECRET, CORS_ALLOW_ORIGIN, APP_URL
#    optional: EMAIL_PROVIDER + keys, UPLOAD_STORAGE=s3 + R2/S3 keys

# 2. create the database (once)
psql -U <DB_USER> -c "CREATE DATABASE ars_kit;"

# 3. dependencies + migrate
go mod download
make migrate-up

# 4. run server + worker (two terminals)
make run       # HTTP API on :8181
make worker    # background workflow jobs (emails, avatar cleanup, demo)
```
