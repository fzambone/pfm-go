# Implement a User Story

Implement a GitHub issue end-to-end following strict TDD: issue loading, branch creation, codebase
assessment, plan writing, decision surfacing, Red→Green cycles, and CI gate execution.

Run this skill at the **start of every new story**. Do not begin implementation without it.

---

## Phase 1 — Setup

### 1.1 Identify the issue

- Read the argument passed to this skill (e.g. `/project:implement 42`)
- If no argument, check the current branch name for a numeric suffix (e.g. `feat/domain-households-12` → issue 12)
- If still not found, ask: "Which GitHub issue should I implement?"

### 1.2 Load the issue

```
gh issue view <N>
```

Read the full issue body carefully. Extract:
- The stated **goal** — what problem this solves
- Explicit **acceptance criteria** (checklist items in the issue body)
- Implicit requirements in prose
- Explicitly **out-of-scope** items

### 1.3 Create the branch

```
git checkout main && git pull
git checkout -b feat/<scope>-<description>-<N>
```

Where `<scope>` is the domain or platform concern and `<N>` is the issue number.

---

## Phase 2 — Codebase Assessment

Explore the relevant packages before writing any code:

- Read the domain packages touched by this story
- Search for existing functions, fakes, ports, and message constants that can be reused
- Identify which files will need to change and which will be new
- Check `internal/message/` for existing error sentinels and messages
- Check `internal/platform/` for existing utilities (validate, clock, money, ctxutil)

---

## Phase 3 — Implementation Plan

Write a plan to `.claude/plans/<branch-name>.md` with these sections:

```
## Summary
One paragraph: what this story delivers and why.

## Domain changes
- New types, methods, or constants in `internal/domain/`

## Port changes
- New or updated interfaces in `internal/port/`

## Adapter / platform changes
- Postgres repo, platform utilities, middleware, wiring in main.go

## SQL
- New migrations (db/migrations/)
- New queries (db/queries/)
- sqlc regeneration needed? yes/no

## Test strategy
- Unit tests: which domain logic, which fakes, which factories
- Integration tests: which repos, tagged //go:build integration

## Implementation order
1. Domain type + unit test
2. Port interface
3. Fake implementation (for other unit tests)
4. Adapter (repo / platform)
5. Integration test
6. Wire in main.go

## Open decisions
- List any architectural ambiguities that require input before coding starts
```

**Stop here and present the plan.** Surface all open decisions. Wait for confirmation before Phase 4.

---

## Phase 4 — TDD Implementation

Follow strict **RED → GREEN → REFACTOR** for every unit — both unit tests and integration tests.

### Two-tier TDD

**Tier 1 — Unit tests** (fakes, `fstest.MapFS`, or other in-memory doubles):
- Write the failing test first: `go test -run TestName ./internal/package/`
- Confirm it compiles and fails (RED) before writing any production code.
- Write the minimum production code to make it pass (GREEN).
- Refactor, then move to the next unit.

**Tier 2 — Integration tests** (testcontainers, real Postgres):
- Tagged `//go:build integration`. Run with: `go test -tags integration -run TestName ./internal/package/`
- Same RED → GREEN discipline: write the failing integration test first, confirm it fails, then implement.
- Integration tests exercise real I/O (SQL, migrations, external services) — never substitute with fakes here.

Rules:
- One unit at a time. Do not write the next test until the current one passes.
- Never write production code without a failing test first — this applies to both tiers.
- Follow the implementation order from Phase 3.

### Sequence

1. **Domain** — new types, validation, business logic. Unit tests only (no DB needed).
2. **Ports** — define interfaces at the consumer (domain/logic layer). Keep them small (1–3 methods).
3. **Fakes** — in-memory implementations of ports, thread-safe (`sync.RWMutex`). Unit tests use these.
4. **Adapters** — postgres repos, platform utilities. Integration tests (testcontainers) go here.
5. **Wiring** — connect everything in `cmd/pfm/main.go`.

### Go idioms to apply

- `fmt.Errorf("operation: %w", err)` — always wrap errors with context
- Sentinel errors in `internal/message/errors.go`
- Validation via `platform/validate/` — never ad-hoc `if x == ""`
- All messages (errors, logs, validation text) in `internal/message/`
- `Clock` interface — never `time.Now()` in production code
- `context.Context` as first argument to every I/O function

---

## Phase 5 — CI Gates

All three must PASS before the story is considered done.

### Gate 1 — Build and test

```
make ci
```

`make ci` runs: lint → unit tests with coverage gate → vuln scan → integration tests (`-race -count=1`) → build. Fix all failures before proceeding.

### Gate 2 — Acceptance verification

```
/project:verify-issue
```

Every acceptance criterion must be COVERED. PARTIAL or MISSING = FAIL. Fix and re-run.

### Gate 3 — Code quality

```
/project:review
```

All 8 categories must PASS. Fix every violation before proceeding.

---

## Phase 6 — Wrap Up

Report to the user:
- Story summary: what was implemented
- Files changed (list with one-line description of each)
- Test coverage added
- Any deferred items or follow-up issues to open

Ready for `/project:ship`.
