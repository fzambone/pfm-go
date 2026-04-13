# Code Review Checklist

Run this complete checklist against all changed files. Every item must pass.
Do NOT suggest a commit until all categories report PASS.

Review the actual code — read every changed file, check imports, verify tests exist.

## Error Handling
- [ ] Every error is wrapped with context: `fmt.Errorf("operation: %w", err)`
- [ ] No bare `return err` — always add context about what was being attempted
- [ ] No error strings inspected programmatically (only `errors.Is` / `errors.As`)
- [ ] No errors silently swallowed (intentional ignores have `// best-effort` comment)
- [ ] Error return paths are tested (not just happy paths)
- [ ] Sentinel errors defined in `internal/message/errors.go`
- [ ] Typed errors carry structured data the caller needs (e.g., `ValidationError` with violations)
- [ ] Error wrapping chain is clear: repo -> logic -> handler each add their context

## Interface Design
- [ ] Interfaces defined at consumer (domain/logic), never at provider (adapter)
- [ ] Functions accept interfaces, return concrete structs
- [ ] No interface has more than 5 methods (prefer 1-3)
- [ ] No stuttering: `household.Repository` not `household.HouseholdRepository`
- [ ] Interface segregation: consumers only depend on methods they actually call

## Concurrency
- [ ] Every goroutine has lifecycle management (WaitGroup, errgroup, or context cancellation)
- [ ] No fire-and-forget goroutines
- [ ] Maps accessed from multiple goroutines are protected with `sync.RWMutex`
- [ ] Fakes use `sync.RWMutex` for thread safety (tests run with `-race`)
- [ ] `context.Context` propagated through every I/O call
- [ ] No goroutines in domain logic (concurrency is platform/infrastructure concern)

## Naming and Style
- [ ] Every exported function, type, method, and constant has godoc starting with its name
- [ ] No getters: `Name()` not `GetName()`
- [ ] Package names are single lowercase words
- [ ] No stuttering in qualified names
- [ ] Test functions follow `Test<Unit>_<Scenario>` pattern
- [ ] Error constants follow `Err<Domain><What>` pattern
- [ ] Log messages: sentence case, no period
- [ ] slog attribute keys: `snake_case`

## Observability — Logging
- [ ] WARN logged for every recoverable failure: retries, fallbacks, degraded states — silence implies success
- [ ] ERROR logged at the handler/entrypoint that catches and resolves the error, not in functions that return it
- [ ] No double-logging: functions that return an error do not also log it
- [ ] Retry loops log each failed attempt at WARN with `"attempt"` and `"error"` attributes
- [ ] INFO logged for significant lifecycle events (service ready, connection established, shutdown started)
- [ ] All log message strings defined in `internal/message/`, never hardcoded inline
- [ ] Context-aware calls only: `slog.XxxContext(ctx, ...)` — never `slog.Xxx(...)` in production code
- [ ] Attribute keys are `snake_case`
- [ ] Errors included as structured attribute `"error", err` — never interpolated into the message string
- [ ] No PII or secrets in log attributes (email, password hash, tokens, card numbers)

## Observability — Tracing
- [ ] Every adapter method (postgres repo, HTTP client, external service call) opens a span:
      `ctx, span := otel.Tracer("package").Start(ctx, "Type.Method")`
- [ ] Every span is closed: `defer span.End()` immediately after `Start`
- [ ] Every error path records the error on the span before returning:
      `span.RecordError(err)` + `span.SetStatus(codes.Error, err.Error())`
- [ ] Happy paths set status OK: `span.SetStatus(codes.Ok, "")` (not strictly required by OTel spec but signals intent)
- [ ] Span names follow `"Domain.Method"` convention — e.g. `"UserRepo.FindByID"`, `"HouseholdLogic.Create"`
- [ ] Span attributes use `snake_case` keys and carry enough context to diagnose without the logs:
      e.g. `"user_id"`, `"household_id"`, `"query_rows_affected"`
- [ ] No PII or secrets in span attributes (same rule as logging)
- [ ] Health check endpoints (`/healthz`, `/health/live`, `/health/ready`) are NOT traced —
      they would pollute trace data with noise
- [ ] `context.Context` is the first argument of every traced function so the span propagates
- [ ] Domain logic does NOT create spans — tracing is adapter/platform concern only

## Testing
- [ ] Table-driven format for 3+ cases testing same behavior
- [ ] Fakes used, never mocks with expectations
- [ ] Test factories provide sensible defaults
- [ ] Edge cases covered: empty input, boundary values, nil, duplicates, concurrency
- [ ] Tests pass with `-race` flag
- [ ] Integration tests guarded by `//go:build integration`
- [ ] Benchmarks exist for hot paths, using `b.ReportAllocs()`

## Architecture
- [ ] Domain packages have ZERO imports from `adapter/`, `platform/`, `middleware/`
- [ ] No business logic in adapters — they translate and delegate only
- [ ] All dependencies injected through `New...` constructor functions
- [ ] Validation uses centralized engine (`platform/validate/`), never ad-hoc checks
- [ ] All messages defined in `internal/message/`, not hardcoded as string literals
- [ ] `message/` imports stdlib only — nothing else

## Go-Specific Traps
- [ ] No typed nil pointers returned as interface values (always return bare `nil`)
- [ ] `defer` argument evaluation is correct (closures used when capturing final values)
- [ ] All struct methods use the same receiver type (pointer or value, not mixed)
- [ ] `New...` factory functions validate required dependencies with `panic`
- [ ] Zero values considered: struct works correctly if partially initialized?
- [ ] `append` results always reassigned: `s = append(s, v)`
- [ ] No `time.Now()` calls — all time through `Clock` interface

## SQL & Database (when applicable)
- [ ] Every SELECT includes `WHERE deleted_at IS NULL` (unless querying deleted records)
- [ ] Every UPDATE includes `AND version = sqlc.arg('expected_version')` and `SET version = version + 1`
- [ ] Every UPDATE sets `updated_at = NOW()` and `updated_by = sqlc.arg('updated_by')`
- [ ] INSERT/UPDATE use `RETURNING *` to avoid second query
- [ ] No `SELECT *` in SELECT queries — columns listed explicitly
- [ ] `sqlc.arg()` used for named parameters (not positional `$1`)
- [ ] SQL keywords UPPERCASE, tables/columns lowercase snake_case
- [ ] Migration has both `-- +goose Up` and `-- +goose Down` blocks
- [ ] Migration is backward-compatible (no direct column renames or drops)

## Performance
- [ ] No N+1 queries — repo methods are not called inside loops; use batch queries or JOINs
- [ ] Queries filtering on non-indexed columns have a justification or a migration adding the index
- [ ] No unbounded result sets — every list query that could return large datasets has a LIMIT or cursor pagination
- [ ] Large payloads stream rather than buffer — avoid loading entire result sets into memory before responding

## Final Verification
- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `go test ./... -race -count=1`
- [ ] Linter clean: `golangci-lint run ./...`
- [ ] Commit message follows: `type(scope): description` + `closes #N`

---

**Report format:** List each category as PASS or FAIL.
For failures, state the specific violation and the file:line where it occurs.
Do NOT suggest committing until every category passes.

Categories:
1. Error Handling
2. Interface Design
3. Concurrency
4. Naming and Style
5. Observability — Logging
6. Observability — Tracing
7. Testing
8. Architecture
9. Go-Specific Traps
10. SQL & Database (when applicable)
11. Performance

**Tracing scope:** Apply the Tracing category only to files in `internal/adapter/` and `internal/platform/`.
Domain packages (`internal/domain/`) must NOT have tracing — flag it as a violation if they do.
