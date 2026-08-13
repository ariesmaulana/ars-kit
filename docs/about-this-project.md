# About This Project

ars-kit — a Go web service (Echo + pgx + PostgreSQL). The structure is
influenced by **Django apps**, with an MVC-like layering inside each module.
This document explains the design intent and how the current code maps to it.
Test details live in [testing-guide.md](testing-guide.md).

## Design Philosophy

### 1. Modular — Django-app style

Everything under `src/app/` is a **module**. Two kinds of modules exist:

- **Foundation modules** — internal, reusable, required by application
  modules. They provide cross-cutting capability, not business features of
  their own. `permission, `notif` and `worker` (if exists) as examples.
- **Application modules** — where most of the business logic lives: the
  features and the actual application. `user` is one example.

Each module follows the same layout:

| File | Responsibility |
|---|---|
| `service_interface.go` | The contract — interface + named input/output types |
| `service.go` | The brain (see MVC-like layering) |
| `handler.go` | The HTTP adapter (application modules) |
| `storage.go` / `storage_interface.go` | The SQL repository |
| `const.go` | The module's permission enum |
| `data.go` | Domain data types |
| `migrate.go` + `sql/` | goose migrations |
| `*_test.go`, `fakes/` | Table-driven tests + generated fakes |

### 2. MVC-like layering

Responsibilities are delegated to layers inside each module:

- **handler.go — the stupid layer.** Receives the request, binds it, calls the
  service, responds to the client. No business logic; at most it maps an
  `ErrorCode` to an HTTP status.
- **service.go — the main brain.** All functionality lives here. The contract
  in `service_interface.go` names explicit input and output types because this
  layer may be called by the handler **or other modules** across seams.
- **storage.go — the repository.** All SQL queries live here, behind a narrow
  interface. Storage errors are categorised (`ErrTypeNotFound`,
  `ErrTypeUniqueConstraint`) so the service can decide intent without parsing
  driver errors.

### 3. Testing

Table-driven tests, per [testing-guide.md](testing-guide.md): service layer
runs against a real DB with external modules mocked per test row; the handler
layer is httptest against a generated fake; storage is tested directly.

## Core Patterns

### Permission strings

- Stored in `user_permissions` as `<user_id>:<permission>` — the full key with
  the user id embedded (e.g. `5:user:profile_update`).
- Each module declares the permissions it checks as constants in `const.go`
  (e.g. `src/app/user/const.go`) using the bare string (`"user:profile_update"`,
  `"super_user"`).
- The **permission module owns the key format**: check, grant, and revoke all
  build the key through one internal `key()` helper, so a granted permission
  always matches a later check. Callers never construct keys.
- **`super_user` is a wildcard**: a user holding `<user_id>:super_user` passes
  every check. Grant/revoke themselves require the actor to hold `super_user`.
- **Audit log (D2)**: the `audit_log` table records security-relevant events
  (grant/revoke/password/username/email/login) with the actor, the affected
  user, and event metadata. Self-service changes (username/password) write
  their trail inside the same transaction as the change; login/grant/revoke
  writes are best-effort and never fail the primary operation. Only a
  `super_user` can read the log via `GET /api/v1/users/audit-logs`.

### Typed operation results

Every service output carries `Success`, `Message` (human prose), and
`ErrorCode` (machine category). The handler maps codes to status in one switch
(`validation` → 400, `unauthorized` → 401, `forbidden` → 403, `internal` →
500). **No status is decided by string-matching the message.**

### Auth

Bcrypt-hashed passwords; JWT in an httpOnly cookie; protected routes behind
the JWT middleware; stricter rate limiting on auth endpoints.

## Key Choices & Trade-offs

1. **Permission keys embed the user id** — self-contained strings, easy to
   store and query, but raw strings without type safety. Mitigated by the
   `const.go` enums and the permission module owning construction.
2. **`super_user` wildcard** — one grant covers everything (simple) vs.
   least-privilege (a super user implicitly holds every permission).
3. **Pragmatic HTTP statuses** — 400 default, 500 only for real system
   failures, 401/403 preserved because end users depend on them. No 404/409
   detail: fewer statuses to learn, simpler mapping.
4. **Real DB in service tests, mocks only at cross-module seams** — high
   fidelity (goose migrations run per scenario) at the cost of requiring
   PostgreSQL. External modules (e.g. permission) are mocked with
   counterfeiter fakes.
5. **Schema isolation per test scenario** — each `suite.Run` creates a random
   schema and runs migrations, so scenarios run in parallel without
   interference; slower than in-memory tests but trustworthy.
6. **`ErrorCode` + `Message` on every output** — two fields to maintain, but
   renaming prose never changes HTTP status and tests assert codes, not text.
7. **One module owns the key format** — locality (format logic lives once) vs.
   coupling (the user module depends on the permission module).
8. **Member feature removed** — the permission change also deleted the member
   endpoints and their tests to keep the surface small.
9. **DSN builders centralised** in `database` — production and tests build
   connection strings from one place.
10. **No ADRs / CONTEXT.md** — architectural decisions live in code, READMEs,
    and this document.

## Testing Strategy

- **Service layer**: suite-based, real DB, external module mocked per test row
  (stub + `testsuite.Counter` + `expectedCountMock`).
- **Handler layer**: httptest + generated fake service, no DB, status asserted
  from `ErrorCode`.
- **Storage**: direct DB tests through the suite.
- **JWT / validator**: pure unit tests.

## Conventions

- `const.go` per module listing the available permission strings.
- `DataXxx` fixtures + a `TestHelper` per module under test.
- Scenarios follow the `Describe` → `Setup` → `Run` + `runtest`/`runRows`
  structure.
- Every change: `gofmt`, `go vet`, `go test ./...`.
