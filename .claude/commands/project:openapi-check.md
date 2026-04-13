# Check OpenAPI Contract Consistency

Run after modifying HTTP handlers or route annotations. Verifies that `api/swagger.yaml`
matches the current handler annotations and flags any breaking changes in the diff.

---

## Phase 1 — Detect Drift

Run the spec check against a temp directory (non-destructive — never modifies `api/swagger.yaml`):

```
make check-openapi
```

If the output is `OpenAPI spec is up to date.` → report:

```
OpenAPI contract: CLEAN
No API changes detected on this branch.
```

Stop here. Nothing more to do.

---

## Phase 2 — Analyze the Diff

If `make check-openapi` exits non-zero, it prints a unified diff directly to stdout.
Read that diff output and classify each hunk:

### Breaking changes (flag explicitly)

| Pattern in diff | Classification |
|---|---|
| `-  /some/path:` (removed path block) | **BREAKING — endpoint removed** |
| `-    delete:` / `-    get:` / etc. under an existing path | **BREAKING — HTTP method removed** |
| `-      required:` entry removed or field removed from `required:` list | **BREAKING — required field removed** |
| Property removed from a `definitions` block (`-        fieldName:`) | **BREAKING — response field removed** |
| `type:` line changed (`-        type: string` → `+        type: integer`) | **BREAKING — field type changed** |
| Parameter removed from `parameters:` block | **BREAKING — request parameter removed** |

### Non-breaking changes (note, don't flag)

| Pattern in diff | Classification |
|---|---|
| New path added | Non-breaking — new endpoint |
| New property added to a definition | Non-breaking — additive field |
| Description or summary text changed | Non-breaking — docs only |
| New parameter added with no `required: true` | Non-breaking — optional param |

---

## Phase 3 — Report

Produce a structured report:

```
OpenAPI contract: DRIFT DETECTED
Branch: <current branch>

Breaking changes:
  [BREAKING] <path or definition> — <what changed>
  ...

Non-breaking changes:
  [ADDED]   <path or field> — <description>
  ...

Action required:
  If these changes are intentional: commit api/swagger.yaml alongside the handler change.
  If not intentional: revert the handler annotation or run `make generate` to sync.

Run `make generate` to regenerate api/swagger.yaml from current annotations.
```

If there are breaking changes, end with:

```
WARNING: This branch introduces breaking API changes. Coordinate with consumers before merging.
```

---

## Rules

- Never modify `api/swagger.yaml` directly — it is generated output.
- Never run `make generate` on behalf of the user — report and let them decide.
- The spec must be committed alongside the handler change that caused it.
