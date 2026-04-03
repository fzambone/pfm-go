# Break Down an Epic Issue

Expand a parent epic issue into implementable child issues. The epic already defines the
breakdown pattern (typically 7 issues), key operations, acceptance criteria, and scope
boundaries. This skill's job is to produce high-quality individual issues from that blueprint.

Each child issue must be self-contained and implementable with `/project:implement`.

---

## Phase 1 — Load the Epic

### 1.1 Identify the issue

- Read the argument (e.g. `/project:breakdown 25`)
- If no argument, ask: "Which GitHub issue should I break down?"

### 1.2 Fetch and extract

```
gh issue view <N>
```

The epic already contains:
- **Standard Domain Pattern** — the numbered list of issues to create (typically 7)
- **Key Operations** — the behaviors each issue must support
- **Acceptance Criteria** — behavioral requirements to distribute across child issues
- **Scope Boundaries** — what's explicitly out
- **Depends On** — external prerequisites
- **Special concerns** — e.g. atomic transactions, multi-entity operations

Extract all of these. They are the inputs to the breakdown.

### 1.3 Assess the codebase

Before writing issues, understand what already exists for this domain:
- Check if a basic entity struct already exists (e.g. from a prior milestone)
- Check `internal/message/errors.go` for existing sentinels
- Check `db/migrations/` for the relevant tables and their columns
- Check if any ports, fakes, or factories already exist
- Look at the completed User domain (`internal/domain/user/`) as the reference implementation

This determines whether issues say "Define" (new) or "Extend" (already exists).

---

## Phase 2 — Expand Into Child Issues

### 2.1 Map the epic's pattern to issues

The epic lists its own breakdown (e.g. "7 separate issues following the pattern"). Use that
list as the skeleton. For each item:

1. Take the one-line description from the epic
2. Distribute the relevant **key operations** and **acceptance criteria** from the epic
3. Add edge cases specific to that layer
4. Set scope boundaries so issues don't overlap

### 2.2 Issue structure

Each child issue MUST have these sections:

```markdown
## Context
One paragraph: why this issue exists, what it solves, how it fits the domain.

## Depends On
- #N — description of what's needed from that issue

## What This Enables
- Bullet list of capabilities unlocked by completing this work

## Acceptance Criteria
1. When <action>, <expected behavior>.
2. When <condition>, <expected outcome>.
...

## Edge Cases to Handle
- [ ] Description of edge case and expected behavior

## Scope Boundaries
- No <thing explicitly out of scope for this issue>.
```

### 2.3 Quality standards

**Behavioral, not prescriptive:**
- GOOD: "When a household is created, the creating user becomes an ADMIN member atomically"
- BAD: "Add a `Create` method that calls `Transactor.RunAtomic` and inserts into both tables"

**Acceptance criteria describe WHAT, not HOW:**
- GOOD: "When examining the port, it defines: `Create`, `FindByID`, and `ListForUser`"
- BAD: Pasting a Go interface definition or SQL query into the issue body

**No code snippets.** No Go structs, no SQL. Describe the behavior and constraints —
the implementer decides the code shape during `/project:implement`.

**Each issue is independently testable.** After implementing issue N, you can run tests
that prove it works without needing issue N+1.

**No overlapping scope.** Each behavior lives in exactly one issue. If the adapter issue
naturally includes integration tests for its methods, don't create a separate integration
test issue for the same tests. Only create a separate test issue when there are additional
scenarios (e.g. cross-method workflows, transactional integrity) beyond single-method tests.

### 2.4 Dependency chain

- Issues form a DAG (directed acyclic graph), not a flat list
- Each issue's `Depends On` must reference concrete issue numbers (filled in after creation)
  or the epic's own dependencies for the first issue
- Allow parallel work where possible (e.g. test factory and fake repo can be independent)

---

## Phase 3 — Present for Review

Before creating any issues, present the full breakdown:

```
Epic #N: <title>
Milestone: <milestone>
Label: <label>

| # | Title | Layer | Depends On | Key behaviors |
|---|-------|-------|------------|---------------|
| 1 | ...   | ...   | ...        | ...           |
| 2 | ...   | ...   | ...        | ...           |
...
```

Then show the **full body** of each issue so the user can review the acceptance criteria,
edge cases, and scope boundaries before anything is created.

**Stop here and wait for approval.** The user may want to:
- Reorder issues
- Merge or split issues
- Adjust scope boundaries
- Add or remove acceptance criteria
- Change dependency chains

---

## Phase 4 — Create Issues

After approval, create each issue using `gh issue create`:

```
gh issue create \
  --title "<title>" \
  --milestone "<milestone>" \
  --label "<label>" \
  --body "$(cat <<'ISSUE_EOF'
<full issue body>
ISSUE_EOF
)"
```

Rules:
- Create issues in dependency order (leaves first, so later issues can reference earlier ones)
- Use the exact milestone name from the epic
- Apply the same label as the epic
- Each issue body starts with `Parent epic: #N`
- After all issues are created, go back and update `Depends On` sections with real issue numbers

---

## Phase 5 — Update the Epic

After all child issues are created, append a checklist to the epic body:

```
gh issue edit <N> --body "$(cat <<'EPIC_EOF'
<original epic body — preserve it exactly>

## Child Issues
- [ ] #A — <title>
- [ ] #B — <title>
...
EPIC_EOF
)"
```

---

## Phase 6 — Report

```
Epic #N: <title>
Created <count> child issues:
- #A — <title>
- #B — <title>
...

Dependency graph:
#A → #B → #E
#A → #C → #E
#A → #D → #E
              #E → #F → #G

Ready for implementation with /project:implement <first-issue>.
```
