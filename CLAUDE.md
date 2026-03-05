# PFM-Go — Personal Financial Manager in Go

## Project Identity

- **Module:** `github.com/zambone/pfm-go` | **Repo:** `github.com/fzambone/pfm-go`
- **Go:** 1.25 | **PostgreSQL:** 18 | **Alpine:** 3.23
- **Commits:** Conventional Commits — three-part format:
  ```
  type(scope): imperative description

  - Bullet per meaningful change (what and why, not how)

  closes #N
  ```
  Prefixes: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`. Scope = domain or platform concern.
  Subject: lowercase, no period, imperative mood (`add`, `fix`, `wire`, not `added`, `fixes`).
  Footer `closes #N` on its own line after a blank line.
- **Git:** Trunk-based, squash-merge, short-lived branches: `feat/<scope>-<description>`
- **CI Gate:** `go test ./... -race -count=1` + `golangci-lint run` + `go build`. All must pass.
- **Tags:** `v0.1.0` (M1), `v0.2.0` (M2), etc. See `MILESTONES.md` for issue breakdown.

## Non-Negotiables — Blocking Issues

1. **No Magic.** No frameworks, ORMs, DI containers. All wiring in `main.go` (Composition Root).
2. **Strict TDD.** Red -> Green -> Refactor. No production code without a failing test first.
3. **Explicit Over Convenient.** Readable without external docs.
4. **Compile-Time Safety.** Types and interfaces. No runtime reflection. Prefer typed constants over raw strings/ints for any value with a known domain (log levels, statuses, units). Parse strings at config/API boundaries only; use typed values everywhere inside the app.
5. **Hermetic Tests.** Unit = fakes (in-memory). Integration = testcontainers. Never shared state.
6. **Google Go Style.** Enforced by golangci-lint. No exceptions.

## Architecture — Hexagonal (Ports & Adapters)

```
cmd/pfm/              -> main.go (Composition Root — wires everything)
internal/
  domain/             -> Pure business logic, zero external imports
    household/ account/ user/ ledger/
  port/               -> Shared interface definitions (auth, repository)
  adapter/
    postgres/ auth/ grpc/ http/
  middleware/         -> Guards, AuthN, AuthZ, logging, tracing
  platform/           -> Cross-cutting: config, observe, database, clock, money, validate, ctxutil
  message/            -> ALL messages: errors, logs, validation
db/
  migrations/         -> Goose SQL (timestamped)
  queries/            -> sqlc SQL files
deploy/docker/        -> Dockerfile, docker-compose.yml
```

### Import Rules — Violations Are Blocking

| Package | Can Import | NEVER Imports |
|---------|-----------|---------------|
| `domain/*` | `message/`, stdlib only | `adapter/`, `platform/`, `middleware/` |
| `message/` | stdlib only | Everything else |
| `platform/*` | `message/`, stdlib, third-party | `domain/`, `adapter/` |
| `adapter/*` | `domain/` (types/ports), `platform/`, `message/` | Other adapters |
| `middleware/` | `domain/` (types), `platform/`, `message/` | `adapter/` |
| `cmd/pfm/` | Everything (composition root) | — |

**The domain layer has zero knowledge of infrastructure.** No `database/sql`, `net/http`,
`encoding/json`, or adapter imports in `domain/`. Define a port (interface) instead.

## Naming Conventions

| Thing | Convention | Example |
|-------|-----------|---------|
| Package | Single lowercase word | `household`, `observe` |
| File | snake_case | `household_logic.go` |
| Interface | Noun (no `I` prefix) | `Repository`, `TokenService` |
| Struct | PascalCase noun | `HouseholdLogic` |
| Method | PascalCase verb | `Create`, `FindByID` |
| Unexported | camelCase | `validateSettings` |
| Test function | `Test<Unit>_<Scenario>` | `TestCreate_WhenNameIsEmpty` |
| Fake | `Fake<Interface>` | `FakeRepository` |
| Factory (test) | `<Domain>Factory` | `HouseholdFactory` |
| Error constant | `Err<Domain><What>` | `ErrHouseholdNotFound` |
| No getters | `Name()` not `GetName()` | — |
| No stuttering | `household.Repository` not `household.HouseholdRepository` | — |

## Godoc — Mandatory for Exports

- Start with the name: `// Create validates the input and persists...`
- First sentence = complete summary. Document behavior, not implementation.
- Unexported: optional unless non-obvious.

## Technology Stack

| Concern | Choice |
|---------|--------|
| Language | Go 1.25 (container-aware GOMAXPROCS) |
| Database | PostgreSQL 18 (UUIDv7 native) |
| Migrations | goose v3 (SQL-first, embeddable) |
| SQL Codegen | sqlc (compile-time type-safe, zero reflection) |
| Auth Tokens | PASETO v4 |
| Logging | log/slog (stdlib, structured) |
| Tracing | OpenTelemetry SDK (manual instrumentation only) |
| Testing | testing (stdlib) + testify assertions |
| Containers | testcontainers-go |
| Linting | golangci-lint (Google style) |
| UUID | google/uuid (tests and fakes only) |
| Password Hash | alexedwards/argon2id |

### Dependency Policy

**Allowed:** Single-purpose libs (uuid, argon2id, sqlc, goose, testcontainers, OTEL SDK).
**Forbidden:** Frameworks (gin, fiber, echo), ORMs (gorm, ent), DI containers, struct-tag magic, auto-instrumentation.
**HTTP:** `net/http` + `http.ServeMux`. **gRPC:** `google.golang.org/grpc` directly. No wrappers.

## Design Decisions

**Money:** `int64` minor units (cents/centavos). `BIGINT` in DB. No floats, no DECIMAL. Formatting in `platform/money/`.

**Clock:** All code uses `Clock` interface, never `time.Now()`. Fakes enable deterministic tests. Domain receives timestamps from logic layer.

**Optimistic Concurrency:** `version INTEGER NOT NULL DEFAULT 1` on every mutable table. `UPDATE ... WHERE version = $expected`, `SET version = version + 1`. Zero rows = conflict error.

**IDs:** PostgreSQL `uuidv7()` as column defaults in production. `google/uuid` in fakes/tests only. App never generates IDs in production.

**Validation:** Centralized engine in `platform/validate/`. Composable rules, collects ALL violations (not fail-fast). Never ad-hoc `if x == ""`. Runs at domain logic entry point only — not in repos, not in transport.

**Messages:** ALL user-facing strings in `internal/message/` — errors, logs, validation. No hardcoded strings in business logic. Protocol or technical constants used within a single package (e.g., HTTP header names, MIME types) are defined as unexported package-level constants in that package — they are not user-facing and do not belong in `message/`.

**Transactions:** `Transactor` port in `platform/database/`. Domain calls `RunAtomic(ctx, func)`. Domain never sees `*sql.Tx` — only `context.Context`. Commit on nil return, rollback on error.

**Household Guard:** AuthN middleware -> AuthZ middleware -> domain. If request reaches domain, caller is already authenticated and authorized. Domain trusts context, never checks membership.

**Observability:** `log/slog` JSON handler with `trace_id`, `span_id`, `user_id`, `household_id` from context. OpenTelemetry manual spans on repos + logic. Health check endpoints from day one.

**Auditable Fields:** Every mutable domain entity and its DB table MUST include `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`, `created_by UUID`, `updated_by UUID`. Domain structs carry these as value fields (not pointers). Repos populate `created_by`/`updated_by` from `ctxutil.UserID(ctx)`. Every `UPDATE` sets `updated_at = NOW()` and `updated_by = $caller`. Read-only join tables (e.g. `household_members`) use `joined_at`/`invited_by` instead of the full audit set.

**Graceful Shutdown:** `signal.NotifyContext` for SIGTERM/SIGINT. Never `os.Exit()` in production code. DB pool closes after HTTP server. OTEL flushes before exit. Timeout configurable (default 15s).

## Error Handling

1. **Always wrap:** `fmt.Errorf("context: %w", err)`. Bare `return err` is always wrong.
2. **Never inspect strings:** Use `errors.Is`/`errors.As`, never `err.Error() == "..."`.
3. **Sentinels** in `message/errors.go`: `var ErrNotFound = errors.New(...)` for known conditions.
4. **Typed errors** when caller needs data: `ValidationError` with field violations.
5. **Never swallow:** Intentional ignores documented: `_ = f.Close() // best-effort`.
6. **Wrap chain:** repo adds detail -> logic adds operation -> handler logs + translates to HTTP/gRPC status.

## Interface Design

1. **Consumer-side:** Interfaces defined where consumed (domain), not implemented (adapter).
2. **Accept interfaces, return structs.** Never return an interface without compelling reason.
3. **1-3 methods** ideal. 4-5 acceptable. 6+ must be split. This is Go, not Java service interfaces.
4. **Interface segregation:** If consumer only needs `FindByID`, don't force `Save`/`Delete`/`List`.

## Testing

| Layer | Tool | Speed | Tests |
|-------|------|-------|-------|
| Domain | `testing` + fakes | ms | Business rules, edges |
| Repository | testcontainers-go | s | SQL, migrations |
| Integration | testcontainers-go | s | Wiring, middleware |
| Benchmark | `testing.B` | varies | Hot path performance |

- **Table-driven** for 3+ cases. **Fakes**, never mocks. **Factory** defaults. `-race` always.
- Integration tests guarded by `//go:build integration` tag.
- Fakes are thread-safe (`sync.RWMutex`). Benchmarks use `b.ReportAllocs()`.
- Test naming: `Test<Unit>_<Scenario>`, subtests lowercase with spaces.

## Database & SQL Conventions

- **PKs:** `uuidv7()` default. **Timestamps:** `TIMESTAMPTZ` UTC. **Money:** `BIGINT`.
- **Soft deletes:** `deleted_at IS NULL` on all queries. Unique indexes partial.
- **Audit:** `created_by`, `updated_by` (UUID). **Tables:** plural snake_case.
- Every SELECT: `WHERE deleted_at IS NULL`. Every UPDATE: `AND version = $expected`, `SET version = version + 1`, `updated_at = NOW()`, `updated_by`.
- `sqlc.arg()` for named params. `RETURNING *` on INSERT/UPDATE. No `SELECT *` in SELECTs — list columns.
- SQL keywords UPPERCASE. Tables/columns lowercase snake_case. One clause per line.

### Migration Safety

1. **Backward-compatible:** Never rename/drop columns directly. Add new -> migrate -> drop old later.
2. **Always rollback:** Every `-- +goose Up` has `-- +goose Down`.
3. **No data in schema files:** Data migrations in separate files.
4. **Immutable once merged:** Never edit applied migrations.
5. **Large tables:** `CONCURRENTLY` for indexes, `NULL` first then `NOT NULL`.

## Concurrency

- **No goroutines in domain logic.** Concurrency is platform/infrastructure.
- Every I/O function takes `context.Context` first and respects cancellation.
- Every goroutine has lifecycle management (`errgroup`, `WaitGroup`, context). No fire-and-forget.
- Maps require `sync.RWMutex` for concurrent access. Fakes are thread-safe.

## Observability

### Logging

- Levels: `DEBUG` (internal state), `INFO` (business events), `WARN` (recoverable), `ERROR` (action required).
- Always context-aware: `slog.InfoContext(ctx, "message")`. Never create new `slog.Logger` in business logic.
- Attributes: `snake_case`. Errors: `slog.Error("failed", "error", err)`. Never log PII/secrets.
- **WARN on every recoverable failure.** Retries, fallbacks, and degraded states must emit `slog.WarnContext`. Silence implies success — a recoverable failure with no log is invisible to operators.
- **Errors are logged once — at the caller that handles them.** Functions that return an error do not log it. The handler or entrypoint that catches and resolves the error is responsible for logging at ERROR. This prevents double-logging of the same event.
- **Exception: retry loops.** A function that internally retries logs each failed attempt at WARN (it handles each attempt itself). The final unrecoverable error is returned to the caller, which logs it at ERROR.
- **INFO for lifecycle events.** Significant state transitions — service ready, connection established, shutdown started — must emit `slog.InfoContext`. These are the breadcrumbs operators use to understand startup and shutdown sequences.

### Tracing (OpenTelemetry)

Tracing lives in `internal/adapter/` and `internal/platform/` only. Domain logic is never traced directly.

**Every adapter method** (postgres repo method, HTTP client call, external service call) must:

```go
ctx, span := otel.Tracer("package").Start(ctx, "Type.Method")
defer span.End()
```

**Every error path** must record the error before returning:

```go
span.RecordError(err)
span.SetStatus(codes.Error, err.Error())
return fmt.Errorf("context: %w", err)
```

**Rules:**
- Span names: `"Domain.Method"` — e.g. `"UserRepo.FindByID"`, `"HouseholdLogic.Create"`
- Span attributes: `snake_case` keys, carry enough context to diagnose without the logs (`"user_id"`, `"household_id"`, `"rows_affected"`)
- No PII or secrets in span attributes (email, password hash, tokens, card numbers)
- Health endpoints (`/healthz`, `/health/live`, `/health/ready`) must NOT be traced — noise
- `context.Context` is always the first argument so spans propagate through the call chain
- Import: `"go.opentelemetry.io/otel"` and `"go.opentelemetry.io/otel/codes"`

## Go Traps — Enforce on Every Review

1. **Nil interface:** Never return typed nil pointer as interface. Return bare `nil`.
2. **Slice append:** Always reassign `s = append(s, v)`. Beware shared backing arrays.
3. **Map concurrency:** Maps are NOT concurrent-safe. Mutex or `sync.Map` required.
4. **Defer eval:** Args captured at defer-time, not execution-time. Use closures for final values.
5. **Receiver consistency:** All methods same receiver type. Default: pointer receivers.
6. **Factory functions:** `New...` for every struct with deps. Validate required deps with `panic`.
7. **Zero values:** Fields default to zero (`""`, `0`, `false`, `nil`), not null. Guard with factories.
8. **Typed constants over strings:** Never use raw `string` or `int` for values with a known domain (log levels, status codes, units, directions). Use existing stdlib types (`slog.Level`, `time.Duration`) or define your own (`type Status string; const StatusActive Status = "active"`). Parse at the boundary (config load, HTTP decode); pass typed values through the rest of the application.

## Pre-Commit Gates — Run Before Every Commit

**Step 1:** Run `/project:verify-issue` to confirm the implementation satisfies
the issue's acceptance criteria. Verdict must be PASS before proceeding.

**Step 2:** Run `/project:review` for the full code quality checklist. Summary categories:

- [ ] **Errors:** Wrapped, tested, not swallowed, sentinels in `message/`
- [ ] **Interfaces:** Consumer-side, small (1-3), no stuttering
- [ ] **Concurrency:** Managed goroutines, protected maps, context propagated
- [ ] **Style:** Godoc on exports, naming conventions, no getters
- [ ] **Tests:** Table-driven, fakes not mocks, edges covered, `-race` clean
- [ ] **Architecture:** Domain imports nothing from adapter/platform
- [ ] **Go traps:** No typed nil returns, correct defer, consistent receivers, factory validation
- [ ] **Logging:** WARN on every recoverable failure, ERROR at handler only, no double-logging, INFO on lifecycle events

## Build & Run

All commands via `Makefile`: `make test`, `make lint`, `make build`, `make up`, `make down`, `make ci`.
CI pipeline: lint + test + build. All must pass before merge. No bypassing.
