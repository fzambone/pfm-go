# Personal Preferences & Teaching Mandate

## Workflow Rules

- **I orchestrate, Claude codes.** I make architectural and product decisions. Claude writes all code, creates all files, and produces all implementation. I do not type code.
- **One step at a time.** One file or one logical unit per step. Pause at decisions that affect architecture or approach — don't proceed through them silently. Wait for confirmation before the next step.
- **Claude runs git and make commands autonomously.** Before making any file changes, create the feature branch. Use `make` targets and `git` commands to verify work (test, lint, build, status, diff, log). Commits and pushes require my explicit instruction.
- **Always pin image versions.** Never use `latest` tag in Docker/container images.
- **Start every story with `/project:implement`.** This skill handles: issue loading, branch creation, codebase assessment, plan writing, decision surfacing, TDD Red→Green cycles, and all CI gate execution. Do not begin implementation without running it first.
- **Branch before coding.** Before touching any file, create the feature branch from `main`: `feat/<scope>-<description>-<N>` where `<N>` is the issue number (e.g., `feat/observe-structured-logging-12`). No edits on `main` — ever. Run `make ci`, then `/project:verify-issue`, then `/project:review` against the branch diff before suggesting a commit. All three must PASS. One branch = one issue = one squash-merge.
- **Ship with `/project:ship`.** After all CI gates pass, use `/project:ship` to run the business-alignment review, commit, push, create the PR, squash-merge, delete the branch, and sync main. Do not do these steps manually.
- **Fetch GitHub issues via `gh`.** Use `gh issue view <N>` to read issue details and acceptance criteria.

## Decision Points — Always Stop and Ask

Stop and surface a decision when:
- Two valid architectural approaches exist and the choice has long-term consequences
- A new dependency is needed that isn't already in `go.mod`
- A design decision in `CLAUDE.md` is ambiguous for the current context
- Something in the acceptance criteria is unclear or potentially in conflict

## Teaching Mandate — Go Mastery

I am transitioning to Go from other languages. When writing code,
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

- **Comment every line in guided snippets.** Every meaningful line in a code snippet must have
  an inline comment explaining what it does and why Go works that way. These comments are for
  learning and do not need to be committed to the repo — I decide what to keep.
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
