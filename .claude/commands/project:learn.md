# Go Concept Deep Dive

The user wants to understand a Go concept more deeply. They may specify a topic,
or reference code they just wrote but didn't fully understand.

Follow this structure for every explanation:

## 1. What It Is
Explain the concept in plain language. No jargon soup. One paragraph max.

## 2. How It Works in Go
Show the mechanics with a minimal code example. Prefer examples from our PFM-Go codebase
when possible. If the concept hasn't appeared in our code yet, use a self-contained example.

## 3. Why Go Does It This Way
Connect to Go's design philosophy:
- Simplicity over cleverness
- Explicit over implicit
- Composition over inheritance
- "Clear is better than clever"
- Fast compilation matters

Every Go idiom exists for a reason. Explain the tradeoff.

## 4. Coming From Other Languages
Compare with Java, Python, or TypeScript equivalents. Use this table format:

| Other Language | Go Equivalent | Why Go Differs |
|---------------|--------------|----------------|
| ... | ... | ... |

Highlight:
- What's genuinely better in Go's approach
- What tradeoffs Go makes (every design has a cost)
- The specific muscle-memory trap to watch for

## 5. In Our Codebase
Point to specific files in PFM-Go where this concept is used or will be used.
Reference file paths and line numbers when applicable.

## 6. Common Mistakes
List the 2-3 most common mistakes developers make with this concept,
especially mistakes that come from Java/OOP habits.

---

## Reference: Go Concepts by Category

### Language Mechanics
- `:=` vs `var` vs `const` — when each is appropriate
- Multiple return values and the `(T, error)` pattern
- Named return values — when useful, when harmful
- Type assertion `x.(Type)` and type switch
- Blank identifier `_` patterns (ignore value, import side effects)
- Composite literals (struct, slice, map initialization)
- Slice internals: length, capacity, backing array, `[low:high:max]`
- Pointer semantics: `*T` vs `T`, when to use each, escape analysis
- `iota` for enumerations — how it differs from Java enums
- Closures and variable capture (especially in goroutines and defer)

### Interface System
- Implicit satisfaction — no `implements` keyword
- Why interfaces belong at the consumer, not the provider
- The empty interface `any` (formerly `interface{}`)
- Interface composition (embedding interfaces)
- Method sets: why `*T` satisfies more interfaces than `T`
- The nil interface trap (type + value both must be nil)

### Error Handling
- Why `if err != nil` instead of exceptions
- Error wrapping with `fmt.Errorf("...: %w", err)`
- Sentinel errors: `var ErrNotFound = errors.New(...)`
- Typed errors: custom structs implementing `error`
- `errors.Is` vs `errors.As` — when to use each
- Why error strings are for humans, not code

### Concurrency
- Goroutines vs threads — the scheduling model
- Channels: buffered vs unbuffered, directional types
- `select` statement for multiplexing channels
- `sync.Mutex` / `sync.RWMutex` — when mutexes beat channels
- `sync.WaitGroup` and `errgroup.Group` — goroutine lifecycle
- `context.Context` — cancellation propagation
- The race detector: what `-race` catches and why it matters

### Package Design
- Package = directory = namespace (no sub-packages)
- `internal/` visibility enforcement
- Import cycle prohibition as architectural enforcer
- Package-level functions vs methods (no static methods)
- `init()` functions — how they work, why we avoid them

### Testing
- Table-driven tests — philosophy and mechanics
- `t.Run()` subtests — parallel execution, focused runs
- `t.Helper()` — clean failure stack traces
- `t.Cleanup()` — deterministic teardown
- Build tags (`//go:build integration`) — separating test tiers
- `-race` flag — concurrent test safety

### Standard Library Patterns
- `io.Reader` / `io.Writer` — the composition backbone
- `http.Handler` / `http.HandlerFunc` — the adapter pattern in stdlib
- `context.Context` — the request-scoped value carrier
- `fmt.Stringer` — Go's `toString()`
- `sort.Interface` — the three-method contract
- `encoding/json` — struct tags, `Marshal`/`Unmarshal`

### Java-to-Go Quick Reference

| Java Pattern | Go Idiom |
|-------------|----------|
| `toString()` | `String() string` (implements `fmt.Stringer`) |
| `implements Interface` | Implicit — just have the methods |
| `final class` | No equivalent needed (no inheritance) |
| `Optional<T>` | `(T, bool)` or `*T` (nil = absent) |
| `List<T>` | `[]T` (slices) |
| `Map<K,V>` | `map[K]V` |
| `Stream().filter().map()` | `for` loop (explicit is idiomatic) |
| Builder pattern | Functional options or plain constructors |
| `@Override` | Not needed (implicit interface satisfaction) |
| Static methods | Package-level functions |
| `enum` with methods | `const` + `iota` + methods on the type |
| `throw new Exception()` | `return fmt.Errorf("...: %w", err)` |
| `try/catch` | `if err != nil` at each call site |
| `class extends Base` | Struct embedding or composition |
| Dependency injection | Constructor functions (`New...`) |
| `getX()` / `setX()` | `X()` (no Get prefix) / `SetX()` only if needed |
| `null` checks | Zero value checks + factory function validation |

---

If the user doesn't specify a topic, ask: "What Go concept would you like to understand better?"
