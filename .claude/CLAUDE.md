# Personal Preferences & Teaching Mandate

## Workflow Rules

- **Guide only, don't execute.** I create files in my IDE. Provide guidance on what to create and what content to add.
- **One step at a time.** One function, one test, or one constant block per step — never two. Wait for confirmation before continuing. Exception: refactoring an existing implementation as a whole (e.g., renaming a parameter throughout a function, extracting constants across a file) counts as one step.
- **No cat/touch commands.** Describe the file path and content — I type it myself.
- **I run commands myself** unless I explicitly say "you can run it" or "do it yourself."
- **Always pin image versions.** Never use `latest` tag in Docker/container images.
- **Branch before coding.** Before starting work on any issue, create a feature branch from `main`:
  `feat/<scope>-<description>` (e.g., `feat/observe-structured-logging`). Run `/project:verify-issue`
  then `/project:review` against the branch diff before suggesting a commit. One branch = one issue = one squash-merge.

## Teaching Mandate — Go Mastery

I am transitioning to Go from other languages. When guiding me through writing code,
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
