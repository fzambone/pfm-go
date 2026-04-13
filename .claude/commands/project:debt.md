# Tech Debt Audit

Scan the codebase for tech debt markers (TODO, FIXME, HACK, XXX), cross-reference against
open GitHub issues, and report what's tracked vs untracked. Optionally create issues for
untracked debt.

---

## Phase 1 — Scan

### 1.1 Find all debt markers

Search all source files for debt comments:

```
grep -rn "TODO\|FIXME\|HACK\|XXX" internal/ cmd/ db/ --include="*.go" --include="*.sql"
```

### 1.2 Parse each marker

For each match, extract:
- **File** — path relative to repo root
- **Line** — line number
- **Category** — TODO, FIXME, HACK, or XXX
- **Text** — the full comment text after the marker
- **Issue reference** — if the comment contains `#<number>`, extract the issue number

Build a list of all markers with these fields.

---

## Phase 2 — Cross-Reference

### 2.1 Load open issues

```
gh issue list --state open --limit 200 --json number,title
```

### 2.2 Match markers to issues

For each debt marker:

1. **Explicit reference:** If the comment contains `#<number>`, check if that issue exists and is open → **TRACKED**
2. **Text match:** Search the open issue titles for keywords from the comment text → **POSSIBLY TRACKED** (note the candidate issue)
3. **No match:** No issue number and no title match → **UNTRACKED**

---

## Phase 3 — Report

Present the findings:

```
## Tech Debt Report

### Summary
- Total markers: <count>
- Tracked (linked to open issue): <count>
- Possibly tracked (text match): <count>
- Untracked (no issue): <count>

### By Category
- TODO:  <count>
- FIXME: <count>
- HACK:  <count>
- XXX:   <count>

### Tracked Debt
| File:Line | Category | Issue | Text |
|-----------|----------|-------|------|
| <path>:<line> | TODO | #<N> | <text> |

### Possibly Tracked Debt
| File:Line | Category | Candidate Issue | Text |
|-----------|----------|-----------------|------|
| <path>:<line> | FIXME | #<N> <title> | <text> |

### Untracked Debt
| File:Line | Category | Text |
|-----------|----------|------|
| <path>:<line> | TODO | <text> |
```

**Stop here.** Ask the user:
1. Should I create GitHub issues for any of the untracked items?
2. Should I re-categorize any "possibly tracked" items?

---

## Phase 4 — Create Issues (optional)

If the user wants GitHub issues for untracked debt, create them one at a time:

```
gh issue create --title "chore(<scope>): <description from debt comment>" --body "$(cat <<'EOF'
## Tech Debt

**Source:** `<file_path>:<line_number>`
**Category:** <TODO/FIXME/HACK/XXX>
**Comment:** <full comment text>

## Context

<brief description of what the code does and why this debt exists>

## Suggested Action

<what needs to change — extracted from the comment and surrounding code>
EOF
)"
```

After creating each issue, report the number and suggest updating the source comment:
```
// TODO(#<new_issue_number>): <original text>
```

### Summary

After all issues are created:
```
Created <N> issues for untracked debt:
  #<number> — <title>
  #<number> — <title>
  ...

Remaining untracked: <count>
```
