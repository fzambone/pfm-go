# Ship the Current Story

Commit, push, create a GitHub PR, squash-merge, delete the branch, and sync main.

Run this skill **after all three CI gates pass** (`make ci` + `/project:verify-issue` + `/project:review`).
Do not run this on `main` directly.

---

## Preconditions

- Must be on a `feat/` branch (never `main`)
- All CI gates must have passed in this session
- No uncommitted changes that belong to a different story

If any precondition fails, stop and report what needs to be resolved first.

---

## Step 1 — Business Alignment Review

Before committing, verify we built the **right thing**:

- **Scope fidelity:** Does the implementation match what the issue asked for — no more, no less?
- **Interpretation fidelity:** Did we interpret ambiguous requirements correctly?
- **Domain language:** Do type names, method names, and error messages reflect the ubiquitous language in the issue?
- **Acceptance criteria:** Is every criterion in the issue body fully addressed?

Report the verdict:
```
Business Alignment: PASS / NEEDS DISCUSSION
- [criterion 1] → addressed / not addressed
- ...
```

If NEEDS DISCUSSION, stop and ask before proceeding.

---

## Step 2 — Stage files

Stage only files relevant to this story. Never use `git add -A` or `git add .`.

```
git status
git add <file1> <file2> ...
```

Exclude:
- `.claude/plans/` — plan files are session artifacts, not repo history
- Binary files, `*.out` coverage files, build artifacts

Verify the staging area with `git diff --cached --stat`.

---

## Step 3 — Commit

Use Conventional Commits format:

```
type(scope): imperative description

- Bullet per meaningful change (what and why, not how)

closes #N
```

Rules:
- `type`: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`
- `scope`: domain or platform concern (e.g. `household`, `database`, `auth`)
- Subject: lowercase, no period, imperative mood (`add`, `fix`, `wire`)
- Footer `closes #N` on its own line after a blank line
- One commit per story

```
git commit -m "$(cat <<'EOF'
type(scope): description

- bullet 1
- bullet 2

closes #N
EOF
)"
```

---

## Step 4 — Push

```
git push -u origin <branch-name>
```

---

## Step 5 — Create Pull Request

```
gh pr create --title "<type>(scope): description" --body "$(cat <<'EOF'
## Summary
- bullet 1
- bullet 2

## Test plan
- [ ] `make ci` passes (lint + test -race + build)
- [ ] `/project:verify-issue` verdict: PASS
- [ ] `/project:review` verdict: PASS (all 8 categories)
- [ ] Integration tests pass with testcontainers
EOF
)"
```

---

## Step 6 — Squash-merge

```
gh pr merge --squash --delete-branch
```

Confirm the PR number before merging.

---

## Step 6.5 — Cross-repo API impact check

After merging, inspect the PR diff for API-affecting changes.

```
gh pr diff <PR-number> --name-only
```

**API-affecting paths:**
- `internal/adapter/http/` — handlers, middleware, response mappings
- `api/swagger.yaml` — spec changes (including docs-only updates)

**If none of these paths appear in the diff:** skip this step silently. Proceed to Step 7.

**If any of these paths appear:**

1. List the affected files from the diff output.
2. Extract the changed endpoints: scan the diff for lines like `+func (h *Handler)` or changed path annotations (`// @Router`).
3. Ask: **"API-affecting changes detected. Create a tracking issue in pfm-ui-react? (y/n)"**

If **no**: skip. Proceed to Step 7.

If **yes**: create the issue:

```
gh issue create --repo fzambone/pfm-ui-react \
  --title "API update: sync frontend with pfm-go #<PR-number>" \
  --body "$(cat <<'EOF'
## Context

pfm-go PR #<PR-number> shipped changes that affect the API contract.
Frontend may need corresponding updates.

## pfm-go PR
<PR-url>

## API-affecting files changed
- <file 1>
- <file 2>
...

## Endpoints affected
- <endpoint or "see diff for details">
...

## Action required
Review the pfm-go diff and update:
- API client calls in `src/`
- TypeScript types if request/response shapes changed
- Any UI that surfaces affected endpoints
EOF
)"
```

Note the created issue URL. Include it in the Step 8 report.

---

## Step 7 — Sync main

```
git checkout main
git pull
```

---

## Step 8 — Report

```
Branch:           feat/<scope>-<description>-<N>
PR:               #<number> — <url>
Merged:           squash-merge ✓
Branch:           deleted ✓
Main:             synced ✓
Business:         PASS / NEEDS DISCUSSION
Frontend impact:  <issue url> created / none detected / skipped

Story #<N> is done.
```
