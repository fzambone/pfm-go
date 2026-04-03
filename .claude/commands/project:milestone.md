# Create a New Milestone

Plan and create a new GitHub milestone with an epic issue and fully broken-down child
issues. This skill is **conversational first** — it asks questions to get the milestone
right before touching GitHub.

The output must match the quality and structure of all previous milestones (M1–M14):
epic issue with full context + acceptance criteria, child issues that are behavioral,
independently testable, and dependency-ordered.

---

## Phase 1 — Gather Information

Before proposing anything, ask the following questions **in a single message**. Wait for
all answers before proceeding.

```
I need to understand the milestone before drafting it. Please answer:

1. **Goal** — One sentence: what does this milestone deliver?
2. **Motivation** — Why now? What problem does it solve or what capability does it unlock?
3. **What it enables** — What becomes possible after this milestone that wasn't before?
4. **Depends on** — Which previous milestones or issues must be complete first?
5. **Scope** — What is explicitly IN scope? What is explicitly OUT of scope?
6. **Rough size** — Is this a small milestone (2–4 stories) or a larger one (5–10 stories)?
7. **Label** — What GitHub label should be applied? (e.g. `infrastructure`, `ci`, `deployment`, `feature`)
8. **Any constraints** — Technology choices, standards, or non-negotiables I should know about?
```

If the user has already provided most of this context in their message, extract what you
can and only ask for the gaps.

---

## Phase 2 — Determine the Milestone Number

```
gh api repos/{owner}/{repo}/milestones --jq '.[].number' | sort -n | tail -1
```

The new milestone number is `max + 1`. Name format: `M{N}: {Title}`.

---

## Phase 3 — Draft the Epic and Breakdown

### 3.1 Epic issue structure

Every epic follows this exact structure (no deviations):

```markdown
## Context
{Two to four sentences explaining the current state, the gap, and why this milestone
addresses it. Be concrete — reference previous milestones by name if relevant.}

## What This Enables
- {Bullet: capability unlocked}
- {Bullet: confidence gained}
- {Bullet: future milestone unblocked}

## Depends On
- M{N} — {reason this prior milestone is a prerequisite}

## {Domain-specific section if applicable}
{E.g. "CI/CD Requirements", "Infrastructure Constraints", "Security Requirements"}
{Bullet list of the key requirements the child issues must satisfy.}

## Acceptance Criteria (Epic-Level)
1. When {condition}, {outcome}.
2. When {condition}, {outcome}.
...

## Scope Boundaries
- No {thing explicitly excluded}.
- No {thing explicitly excluded}.

**Action Required:** Break this into {N} separate issues following the pattern below
before starting implementation.

## Child Issues
(populated after child issues are created)
```

### 3.2 Child issue structure

Every child issue follows this exact structure:

```markdown
Parent epic: #{epic_number}

## Context
{One paragraph: why this issue exists, what layer it operates in, how it fits the
milestone. Reference the epic by number.}

## Depends On
- #{N} — {what's needed from that issue}

## What This Enables
- {Bullet: what this issue unlocks for the next one}

## Acceptance Criteria
1. When {action}, {expected behavior}.
2. When {condition}, {expected outcome}.
...

## Edge Cases to Handle
- [ ] {Edge case and expected behavior}

## Scope Boundaries
- No {thing explicitly out of scope for this issue}.
```

### 3.3 Quality standards (non-negotiable)

**Behavioral, not prescriptive.**
- GOOD: "When a deployment is triggered on merge to main, the app is live within 5 minutes"
- BAD: "Configure the `fly deploy` command in the GitHub Actions workflow"

**No code snippets.** No YAML, no shell commands, no Go structs. The implementer decides
the shape during `/project:implement`.

**Each issue is independently testable.** After completing issue N, there is something
concrete to verify without needing N+1.

**No overlapping scope.** Each concern lives in exactly one issue.

**Dependency chain is a DAG.** Issues form layers — infrastructure before app, app before
verification. Allow parallel work where the layers permit.

---

## Phase 4 — Present for Review

Present the complete proposal **before creating anything**:

```
Milestone: M{N}: {Title}

Epic: {one-line summary}

Child issues ({count} total):

| # | Title | Layer | Depends On | Key behaviors |
|---|-------|-------|------------|---------------|
| 1 | ...   | ...   | epic deps  | ...           |
| 2 | ...   | ...   | #1         | ...           |
...
```

Then show the **full body** of every issue — epic first, then each child — so the user
can review acceptance criteria, edge cases, and scope boundaries.

**Stop here. Wait for explicit approval.** The user may want to:
- Rename the milestone or epic
- Add, remove, or merge child issues
- Adjust scope boundaries or acceptance criteria
- Reorder the dependency chain

Do not create anything until the user says to proceed.

---

## Phase 5 — Create the Milestone and Issues

After approval:

### 5.1 Create the GitHub milestone

```
gh api repos/{owner}/{repo}/milestones \
  --method POST \
  --field title="M{N}: {Title}" \
  --field description="{one-line description}"
```

### 5.2 Create the epic issue

```
gh issue create \
  --title "{epic title}" \
  --milestone "M{N}: {Title}" \
  --label "{label}" \
  --body "$(cat <<'ISSUE_EOF'
{full epic body — Child Issues section left empty for now}
ISSUE_EOF
)"
```

### 5.3 Create child issues in dependency order

Create in leaf-first order so later issues can reference real issue numbers.

```
gh issue create \
  --title "{title}" \
  --milestone "M{N}: {Title}" \
  --label "{label}" \
  --body "$(cat <<'ISSUE_EOF'
Parent epic: #{epic_number}

{full issue body}
ISSUE_EOF
)"
```

After each creation, note the assigned issue number. Update `Depends On` sections in
subsequent issues with real numbers before creating them.

### 5.4 Update the epic with the child issue checklist

```
gh issue edit {epic_number} --body "$(cat <<'EPIC_EOF'
{original epic body — preserved exactly}

## Child Issues
- [ ] #{A} — {title}
- [ ] #{B} — {title}
...
EPIC_EOF
)"
```

---

## Phase 6 — Report

```
Milestone: M{N}: {Title}
Epic:      #{epic_number} — {title}
Issues:    {count} child issues created

Child issues:
- #{A} — {title}
- #{B} — {title}
...

Dependency graph:
#{A} → #{B} → #{D}
#{A} → #{C} → #{D}
               #{D} → #{E}

Ready for /project:implement #{first_leaf_issue}.
```
