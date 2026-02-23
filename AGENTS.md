# PFM-Go — Personal Financial Manager in Go

## Project Identity

- **Module:** `github.com/zambone/pfm-go` | **Repo:** `github.com/fzambone/pfm-go`
- **Go:** 1.25 | **PostgreSQL:** 18 | **Alpine:** 3.23
- **Commits:** Conventional Commits — `type(scope): description`, closes #N
  Prefixes: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`. Scope = domain or platform concern.
- **Git:** Trunk-based, squash-merge, short-lived branches: `feat/<scope>-<description>`
- **CI Gate:** `go test ./... -race -count=1` + `golangci-lint run` + `go build`. All must pass.
- **Tags:** `v0.1.0` (M1), `v0.2.0` (M2), etc. See `MILESTONES.md` for issue breakdown.

## Non-Negotiables — Blocking Issues

1. **No Magic.** No frameworks, ORMs, DI containers. All wiring in `main.go` (Composition Root).
2. **Strict TDD.** Red -> Green -> Refactor. No production code without a failing test first.
3. **Explicit Over Convenient.** Readable without external docs.
4. **Compile-Time Safety.** Types and interfaces. No runtime reflection.
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

**Messages:** ALL strings in `internal/message/` — errors, logs, validation. No hardcoded strings in business logic.

**Transactions:** `Transactor` port in `platform/database/`. Domain calls `RunAtomic(ctx, func)`. Domain never sees `*sql.Tx` — only `context.Context`. Commit on nil return, rollback on error.

**Household Guard:** AuthN middleware -> AuthZ middleware -> domain. If request reaches domain, caller is already authenticated and authorized. Domain trusts context, never checks membership.

**Observability:** `log/slog` JSON handler with `trace_id`, `span_id`, `user_id`, `household_id` from context. OpenTelemetry manual spans on repos + logic. Health check endpoints from day one.

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

## Logging Conventions

- Levels: `DEBUG` (internal state), `INFO` (business events), `WARN` (recoverable), `ERROR` (action required).
- Always context-aware: `slog.InfoContext(ctx, "message")`. Never create new `slog.Logger` in business logic.
- Attributes: `snake_case`. Errors: `slog.Error("failed", "error", err)`. Never log PII/secrets.

## Go Traps — Enforce on Every Review

1. **Nil interface:** Never return typed nil pointer as interface. Return bare `nil`.
2. **Slice append:** Always reassign `s = append(s, v)`. Beware shared backing arrays.
3. **Map concurrency:** Maps are NOT concurrent-safe. Mutex or `sync.Map` required.
4. **Defer eval:** Args captured at defer-time, not execution-time. Use closures for final values.
5. **Receiver consistency:** All methods same receiver type. Default: pointer receivers.
6. **Factory functions:** `New...` for every struct with deps. Validate required deps with `panic`.
7. **Zero values:** Fields default to zero (`""`, `0`, `false`, `nil`), not null. Guard with factories.

## Code Review — Run Before Every Commit

Run `/project:review` for the full checklist. Summary categories:

- [ ] **Errors:** Wrapped, tested, not swallowed, sentinels in `message/`
- [ ] **Interfaces:** Consumer-side, small (1-3), no stuttering
- [ ] **Concurrency:** Managed goroutines, protected maps, context propagated
- [ ] **Style:** Godoc on exports, naming conventions, no getters
- [ ] **Tests:** Table-driven, fakes not mocks, edges covered, `-race` clean
- [ ] **Architecture:** Domain imports nothing from adapter/platform
- [ ] **Go traps:** No typed nil returns, correct defer, consistent receivers, factory validation

## Build & Run

All commands via `Makefile`: `make test`, `make lint`, `make build`, `make up`, `make down`, `make ci`.
CI pipeline: lint + test + build. All must pass before merge. No bypassing.


---

## Local Claude Context (.claude/CLAUDE.md)

# Personal Preferences & Teaching Mandate

## Workflow Rules

- **Guide only, don't execute.** I create files in my IDE. Provide guidance on what to create and what content to add.
- **One step at a time.** Never dump large code blocks. Give one small task, wait for my confirmation before continuing.
- **No cat/touch commands.** Describe the file path and content — I type it myself.
- **I run commands myself** unless I explicitly say "you can run it" or "do it yourself."
- **Always pin image versions.** Never use `latest` tag in Docker/container images.
- **Branch before coding.** Before starting work on any issue, create a feature branch from `main`:
  `feat/<scope>-<description>` (e.g., `feat/observe-structured-logging`). Run `/project:review`
  against the branch diff before suggesting a commit. One branch = one issue = one squash-merge.

## Teaching Mandate — Go Mastery

I am transitioning to Go from other languages. When guiding me through writing code,
**proactively explain Go-specific idioms and patterns** that differ from mainstream languages.

### Always Explain These When They Appear

- **Go-specific syntax:** `:=` short declaration vs `var`, named return values, blank identifier `_`,
  multiple return values `(T, error)`, type assertion `x.(Type)`, type switch, composite literals,
  slice expressions `[low:high:max]`, channel operations `<-`, select statement, struct embedding,
  method sets, pointer vs value receiver implications
- **Interface mechanics:** Implicit satisfaction (no `implements`), why this enables decoupling,
  how a concrete type satisfies multiple interfaces simultaneously
- **Error patterns:** Why `if err != nil` instead of try/catch, wrapping with `%w`,
  sentinel vs typed errors, `errors.Is`/`errors.As` unwrapping chains
- **Package design:** Why Go packages differ from Java packages/namespaces, import cycle
  prohibition as architectural enforcer, `internal/` visibility rules
- **Memory model:** Stack vs heap (escape analysis), pointers without arithmetic,
  when `*T` vs `T` matters for performance and semantics
- **Testing idioms:** Why table-driven tests are the Go way, `t.Run()` subtests,
  `t.Helper()` for clean stack traces, `t.Cleanup()` for teardown, `testdata/` convention
- **Standard library patterns:** `io.Reader`/`io.Writer` composition, `context.Context`
  propagation, `http.Handler`/`http.HandlerFunc`, `sort.Interface`
- **Build system:** Build tags (`//go:build`), `//go:generate`, `//go:embed`,
  `internal/` package scoping
- **Unique to Go:** `defer` LIFO stack, `init()` functions (and why we avoid them),
  `iota` for enums, blank imports for side effects (`_ "pkg"`), goroutine scheduling model

### How to Explain

- **Inline with guidance:** When a step uses a Go-specific pattern, add 2-3 sentences on
  WHY it works that way in Go — not just what to type.
- **Compare to familiar concepts:** "In Java you'd use X, in Go the idiomatic approach is Y
  because Go favors Z."
- **Connect to philosophy:** Go's design favors simplicity, explicitness, and fast compilation.
  Link idioms back to these values so the patterns stick.
- **Use `/project:learn`** when I ask for a deeper dive on any concept.

### Do NOT Explain

- Basic control flow (if, for, switch, range) unless a Go-specific nuance applies
- Variable declarations unless comparing `:=` vs `var` vs `const`
- Function signatures unless involving variadic params, named returns, or multiple returns
- Things I've already demonstrated understanding of in previous interactions
