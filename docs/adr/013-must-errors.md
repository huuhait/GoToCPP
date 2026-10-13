# ADR-013: `!` (must) returns errors for you, only where it is written

Status: accepted (2026-10-13)

## Context

Go's error handling is explicit but noisy: `if err != nil { return err }` after almost every
call. The Go team tried `check`/`handle` (2018), `try` (2019) and `?` (2024–25), and in 2025
decided not to change Go's syntax. gocpp can add it. The user decided where `!` works
([Q10](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)) and what `main` does
([Q11](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)).

## Decision

`!` works in two places, and only where it is written:
- **On a function** (a must function), `func loadConfig!(path string) Config`: every call inside
  it whose `error` result the code drops is handled. A non-nil error is returned, the other
  results become zero values, and defers still run. Callers see
  `func loadConfig(path string) (Config, error)`. `return cfg` means success, and
  `return cfg, err` fails on purpose. If the last declared result is already `error`, nothing
  is added. Methods can be must functions too: `func (s *Server) Start!()` satisfies an
  interface's `Start() error`.
- **On one call**, in any function: `F!(...)`. `F`'s last result must be exactly `error`, and
  `F!` gives back the other results.

Without `!` it is plain Go. In `func main!()` and `func init!()` an error is printed as
`<program>: <error>` and the program exits with code 1: for `main!`, main's defers run first;
for `init!`, the program stops before `main` starts. `F!` inside a plain `main` acts like
`main!`. In other functions without an error result, `F!` calls `t.Fatal(err)` in a test and
panics anywhere else (a plain `init` included).

Parser patch #4 makes a non-keyword identifier followed by `!(` or `![` one name (`ReadFile!`),
so `if!(a)` keeps its meaning. Before `go/types`, must calls become hidden generic helpers, and
must functions get their `error` result and a second type check. Lowering makes plain early
returns in the single-exit shape ([ADR-011](011-errors-panic-defer.md)), so no helper call is
left in the C++.

```go
func loadConfig!(path string) Config { // a must function
	var cfg Config
	data := os.ReadFile(path)  // error → returned for you
	json.Unmarshal(data, &cfg) // error → returned for you
	return cfg
}

func main() {
	cfg := loadConfig!("app.json") // a must call; in main: error → printed, exit 1
	fmt.Println("port:", cfg.Port)
}
```

## Alternatives

- **A `try(...)` builtin** (Go's 2019 proposal; rejected): the user chose `!` (Q10).
- **A `?` suffix** (Rust style; rejected): the same idea, but the user chose `!` (Q10).
- **Implicit handling without a marker** (rejected): it would hide control flow.

## Consequences

Good: much less boilerplate at zero runtime cost; the Go-style standard library works unchanged
(`os.Open!(p)`, `strconv.Atoi!(s)`); the editor marks every call that a must function handles.

Bad:
- Inside a must function every dropped error is handled, even `fmt.Println`'s.
- A concrete error type as the last result (`*MyErr`) is a compile error (Go's typed-nil trap),
  and so are `go F!()` and `!` inside a lazy macro argument.
- Not in v1: `!` on arbitrary expressions (`fns[i]!()`), on function literals or on C functions.

## Evidence

- Spikes 2026-10-13 on stock `go/types`: `os.ReadFile!` gives `[]byte` and a non-error last
  result is rejected ([must-calls](../../spikes/2026-10-13/must-calls/)); must functions had 5
  expected errors in the first check and 0 in the second, and the program ran right for a good
  config, a missing port and a missing file ([must-functions](../../spikes/2026-10-13/must-functions/)).
- Audit: a nil `*MyErr` became a non-nil `error`, so success looked like failure.

See: [Phase 10 §1](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#1-idea), [§2.1](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#21-part-a--must), [§2.4](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#24-tooling), [Phase 05 §2.5](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#25-error-and-the-errors-model), divergence [D12](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
