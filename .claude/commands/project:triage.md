# Triage a Bug or Error

Investigate a bug report, error message, or unexpected behavior using a structured process.
Systematically search the codebase, identify root causes, and optionally create a GitHub issue.

Run this skill when a bug is reported or an error surfaces — before jumping to a fix.

---

## Phase 1 — Input

### 1.1 Capture the problem

Read the argument passed to this skill (e.g. `/project:triage "500 error on POST /api/v1/households"`).

If no argument, ask: "Describe the bug, paste the error message, or provide a stack trace."

### 1.2 Extract signals

From the input, identify all available signals:

- **Error message** — the exact text
- **HTTP status code** — if applicable
- **Endpoint** — the route or gRPC method
- **Stack trace** — file:line references
- **Reproduction steps** — how to trigger it
- **Environment** — local, CI, production (Fly.io)

Record what you have and what's missing. If critical context is missing, ask before proceeding.

---

## Phase 2 — Investigate

Search the codebase systematically, layer by layer. Do NOT guess — read the code.

### 2.1 Entry point

Find the handler or entrypoint where the error surfaces:

```
# Find the HTTP handler for the endpoint
grep -rn "HandleFunc\|Handle\|.POST\|.GET\|.PUT\|.DELETE" cmd/pfm/main.go internal/adapter/http/
```

### 2.2 Trace the error chain

Go errors wrap with `%w`. Trace the chain from surface to source:

1. **Handler layer** (`internal/adapter/http/`) — how does it translate errors to HTTP status?
2. **Middleware** (`internal/middleware/`) — does auth, logging, or tracing interfere?
3. **Domain logic** (`internal/domain/`) — what business rule produces this error?
4. **Repository** (`internal/adapter/postgres/`) — is it a SQL/database issue?
5. **Message constants** (`internal/message/errors.go`) — is there a sentinel for this error?

```
# Search for the error message text
grep -rn "<error text>" internal/

# Search for the sentinel error
grep -rn "Err<Domain>" internal/message/errors.go

# Trace a specific function
grep -rn "<FunctionName>" internal/
```

### 2.3 Check related SQL

If the error involves data persistence:

```
# Find the sqlc query
grep -rn "<entity>" db/queries/

# Check the migration
grep -rn "<table_name>" db/migrations/
```

### 2.4 Check middleware stack

```
# Trace the middleware chain in main.go
grep -n "middleware\|Use\|Wrap\|Chain" cmd/pfm/main.go
```

### 2.5 Check configuration

If the error might be environment-related:

```
# Check config loading
grep -rn "Config\|ENV\|getenv" internal/platform/config/
```

---

## Phase 3 — Diagnose

### 3.1 List hypotheses

Based on the investigation, list all plausible root causes. For each:

```
Hypothesis #N: <one-line description>
  Layer:      handler / middleware / domain / repo / config / infrastructure
  Evidence:   <what points to this cause>
  Likelihood: HIGH / MEDIUM / LOW
  File:       <file_path:line_number>
```

### 3.2 Rank and narrow

Order hypotheses by likelihood. For the top 1-2:

- Read the specific code path end-to-end
- Check if the error is reproducible from the evidence
- Identify the exact line where the bug originates

### 3.3 Confirm or rule out

For each top hypothesis, state:
- **CONFIRMED** — the code clearly shows the bug
- **LIKELY** — evidence is strong but needs reproduction
- **RULED OUT** — evidence contradicts this hypothesis

---

## Phase 4 — Test Gap Analysis

### 4.1 Find existing tests

```
# Find tests for the affected domain
ls internal/domain/<package>/*_test.go

# Find integration tests
grep -rn "func Test.*<Entity>" internal/adapter/postgres/
```

### 4.2 Identify the gap

For the confirmed or likely root cause:

- Which test file should contain a test for this scenario?
- What test case is missing? (describe the table-driven test row)
- Is it a unit test gap (domain logic) or integration test gap (SQL/wiring)?

Report:
```
Test gap: <description>
  Should be in:  <file_path>
  Missing case:  <scenario description>
  Type:          unit / integration
```

---

## Phase 5 — Report

Present findings to the user:

```
## Triage Report

**Problem:** <one-line summary>
**Root cause:** <confirmed or most likely hypothesis>
**Layer:** <handler / middleware / domain / repo / config>
**File:** <file_path:line_number>

### Evidence
<bullet points of what was found>

### Test gap
<what test should have caught this>

### Suggested fix
<brief description of the fix approach — do NOT implement>
```

**Stop here.** Ask the user:
1. Should I create a GitHub issue for this?
2. Should I proceed to fix it with `/project:implement`?

---

## Phase 6 — Create Issue (optional)

If the user wants a GitHub issue:

```
gh issue create --title "fix(<scope>): <description>" --body "$(cat <<'EOF'
## Bug Report

**Error:** <exact error message>
**Endpoint:** <if applicable>
**Environment:** <local / CI / production>

## Root Cause

<description from triage>

## Evidence

<file paths and line numbers>

## Suggested Fix

<approach from triage report>

## Test Gap

<what test to add>

## Acceptance Criteria

1. When <trigger scenario>, the error no longer occurs.
2. A test exists that covers this exact scenario.
EOF
)"
```

Report the issue number and URL back to the user.
