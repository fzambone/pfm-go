# Audit Go Test Coverage and TDD Compliance

Run this skill after implementing any feature or fix. It runs in two phases:
1. **Coverage** — measure and report per-package coverage
2. **TDD compliance** — audit naming, structure, isolation, and race safety

---

## Phase 1 — Coverage

### 1.1 Run tests with coverage

```
go test -race -count=1 -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### 1.2 Analyze coverage

Parse the `go tool cover -func` output and report coverage per package.

**Thresholds:**

| Package group | Minimum |
|---------------|---------|
| `internal/domain/...` | ≥ 80% |
| `internal/adapter/...` | ≥ 70% |

**Exclude from gate** (these are expected to have low or zero unit coverage):
- `cmd/pfm/` — composition root, tested via integration
- Generated code (`db/` sqlc output)
- Pure interface files (no executable statements)
- Packages that require testcontainers (run only with `//go:build integration`)

**Flag:**
- Any non-excluded package with 0% coverage → must have at least one unit test
- Any domain package below 80%
- Any adapter package below 70%

### 1.3 Report

```
Coverage Report
===============
internal/domain/household/    92.3%  ✓
internal/domain/account/      78.1%  ✓
internal/adapter/postgres/    71.4%  ✓
internal/platform/validate/   85.0%  ✓
...

Packages below threshold:
- internal/domain/ledger/  62.0%  ✗  (needs +18%)
```

---

## Phase 2 — TDD Compliance

Read every `*_test.go` file in changed packages. Check each item below.

### Naming

- [ ] Test functions follow `Test<Unit>_<Scenario>` pattern (`TestCreate_WhenNameIsEmpty`)
- [ ] Subtests use lowercase with spaces (`t.Run("when name is empty", ...)`)
- [ ] Fakes named `Fake<Interface>` (`FakeRepository`, `FakeClock`)
- [ ] Factories named `<Domain>Factory` (`HouseholdFactory`, `AccountFactory`)

### Structure

- [ ] 3+ cases testing the same behavior use table-driven format
- [ ] Each table row has a `name` field used in `t.Run`
- [ ] `t.Helper()` called in assertion helpers to surface failures at the call site
- [ ] `t.Cleanup()` used for teardown (not `defer` + manual cleanup)

### Isolation

- [ ] Unit tests use fakes only — no real database, no real clock, no HTTP calls
- [ ] Integration tests tagged `//go:build integration` at the top of the file
- [ ] No shared mutable state between tests (each test constructs its own fake)
- [ ] Fakes implement the port interface and are thread-safe (`sync.RWMutex`)

### Race safety

- [ ] Tests pass with `-race` flag (no DATA RACE output)
- [ ] Fakes use `sync.RWMutex`: `RLock`/`RUnlock` for reads, `Lock`/`Unlock` for writes

### Coverage of error paths

- [ ] Happy path tested
- [ ] Error path tested (every returned error has at least one test case)
- [ ] Edge cases: empty input, boundary values, nil, duplicates

### Factories

- [ ] Factory functions exist for domain types used across multiple tests
- [ ] Factories provide sensible defaults; individual tests override only what matters
- [ ] Factories defined in `*_test.go` files (not exported to production code)

---

## Final Report

```
TDD Compliance
==============
Naming:         PASS / FAIL
Structure:      PASS / FAIL
Isolation:      PASS / FAIL
Race safety:    PASS / FAIL
Error coverage: PASS / FAIL
Factories:      PASS / FAIL

For each FAIL: <file>:<line> — <description of violation>

Overall: PASS / FAIL
```

Do NOT suggest committing until both phases report PASS.
