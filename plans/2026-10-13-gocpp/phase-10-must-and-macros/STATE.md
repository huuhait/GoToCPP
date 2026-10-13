# Phase 10: Beyond Go: `!` (must) & Macros · STATE

**Status:** `[ ]` not started (all designs validated by spikes; your decisions on `!` and macros applied)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike: `F!()` rewrite into hidden generic helpers works with **stock** go/types (types inferred, wrong last result rejected, statement form post-checked) (2026-10-13)
- [x] Spike: first macro design (functions with an explicit site parameter + lazy arguments), generic `Dbg`, lazy argument evaluated 0 times on success (2026-10-13)
- [x] Spike: stock go/printer prints `os.ReadFile!(path)` and `#assert(n > 0)` (2026-10-13)
- [x] Spike: `macro` declarations + on-top macros (`Trace` enter/exit incl. on panic, `Locked` with 100 concurrent calls, race-clean) as plain Go after rewrite (2026-10-13)
- [x] Spike: must functions `func f!() T` on stock go/types: check 1 has 5 expected errors, check 2 has 0; the program runs right for a good config, a missing port and a missing file (2026-10-13)
- [x] Spike: wrap-style macros with `#body()` (`VM`, `Recover`, `InThread`, `Once`) as plain Go after rewrite, race-clean (2026-10-13)
- [ ] 10.1 Parser + printer
  - [ ] patch #4: `name!(`, `name![` (non-keyword identifiers only; `if!(a)` etc. unchanged), in calls and `func` declarations
  - [ ] patch #5: `#Name`
  - [ ] patch #6: `macro` declarations (package level), `lazy T` params, package-level on-top `#Name(...)` lines
  - [ ] `go/printer` fork printing `macro`, `lazy`, `func f!()`, on-top macros; `gocpp fmt` switches to it
  - [ ] tests + round-trip
- [ ] 10.2 Hidden packages `gocpp/must` (M1, M2, M3, Any) and `gocpp/macro` (Site + `Outer`), aliased `gp_must`/`gp_macro`
- [ ] 10.3 `F!` call rewrite + post-check on every form (`Identical` to `error`) + friendly error messages
- [ ] 10.4 Must functions
  - [ ] signature (`error` added unless declared) + `return x` → `return x, nil`
  - [ ] two checks: find dropped errors with the first check's signatures, rewrite, check again
  - [ ] declared-error form (`(cfg Config, err error)` + `errors.Wrapf`)
  - [ ] `main!` / `init!` (print + exit 1), methods
  - [ ] `defer f.Close()` inside; closures not handled; `go f()` → vet warning
- [ ] 10.5 `!` lowering
  - [ ] propagate (zero values + error, defers run)
  - [ ] panic without an error result (incl. `init`, package-level vars); `t.Fatal` in tests
  - [ ] `main`: print error (`<program>` = base name of `os.Args[0]`), exit 1
  - [ ] per-context shapes: left-hoisting, loop condition + post, `else if`, `case`, `&&`/`||`
  - [ ] `defer F!()` as a deferred closure (args evaluated at the defer)
  - [ ] closures; reject `go F!()` and `!` inside lazy arguments
- [ ] 10.6 `errors.Wrapf(&err, ...)`
- [ ] 10.7 (optional) Error return traces: design note, then `errors.Trace` / `%+v`
- [ ] 10.8 Built-in macros (`#file`, `#line`, `#func`, `#text`, `#body`, `#embed`) + `//go:embed`
- [ ] 10.9 User macros
  - [ ] `macro` decl → inline header template with hidden `gp_site` param; `#file/#line/#func/#text` → use site
  - [ ] uses inside code → calls with one static `Site` each
  - [ ] `lazy T` → template callable (no `gp::func` allocation); concrete, spellable types only
  - [ ] on-top, start style → first statement, nil-safe `defer` form, top-to-bottom order
  - [ ] on-top, wrap style → code moved into a closure (returns rewritten), `gp_body` as a template callable, `#body()` rules
  - [ ] nested macros via `Site.Outer` (`#text` innermost, `#file/#line/#func` outermost)
  - [ ] cross-package macros via package summaries
  - [ ] v1 rules: no plain calls/values, no statement use of `func()`-returning macros, package-level only
- [ ] 10.10 `#Derive(...)` in two steps (stubs before go/types, bodies after): `String`, `Equal`, then `Clone`, struct-only `JSON`
- [ ] 10.11 `gocpp expand`
- [ ] 10.12 Lift gates; spec D12/D13/D17/D18; feature matrix
- [ ] 10.13 Tests: rules, every lowering context, differential vs hand-written Go twins (must calls, must functions, wrap macros), golden C++, overhead benchmarks (incl. cross-package)

## Exit criteria

- [ ] `must-errors` and `macros` examples pass on GCC and Clang
- [ ] Every `!` rule and lowering context tested; outputs equal their Go twins under real `go run`
- [ ] The must-function spike's three runs pass as gocpp programs
- [ ] No helper calls left in generated C++; no measurable overhead (incl. macros across packages)
- [ ] Macro errors point at call site and definition
- [ ] On-top macros: exit part runs on panic; `Locked`-style test race-clean; wrap macros behave as in the spike, race-clean
- [ ] `gocpp fmt` round-trips `macro`, `lazy`, `func f!()`, on-top macros
- [ ] `#Derive(String)` handles shared values; `#Derive(JSON)` matches `encoding/json`

## Decisions / notes

- 2026-10-13: `F!()` is a call-site feature (Rust `?`-style, written with `!`), not a declaration
  form. That keeps control flow visible at every call. *(Extended below: must functions were
  added by your decision.)*
- 2026-10-13: Macros are typed functions + call-site info + lazy arguments, invoked with `#`
  (Swift uses `#` for macros too). `!` is reserved for must-calls.
- 2026-10-13 (your choice): declare macros with a `macro Name(args) { ... }` keyword (replaces the
  `//gocpp:macro` comment idea). Use them in three places: inside code, on top of a func (inserted
  as the first statement, `defer` form for a returned `func()`), and `#Derive(...)` on top of a
  type (replaces `//gocpp:derive`).
- 2026-10-13 (audit): `F!` requires the last result to be exactly `error` (typed-nil trap);
  per-context lowering shapes; `defer F!()` via a deferred closure; `t.Fatal` in tests; scanner
  rule skips keywords; `#Derive` in two steps; `Site.Outer` for nested macros; macros as inline
  header templates (no allocation, cross-package inlining); several unsafe macro uses rejected;
  `gp_macro` alias; size raised to 5–6 weeks; error traces need a design note first.
- 2026-10-13 (your decisions, Q10/Q11/Q13): `!` works in two places and only where written: on a
  function (`func f!() T`, every dropped error inside is returned; callers see `(T, error)`) and
  on one call (`F!()`). `main` prints `<program>: <error>` and exits 1. On-top macros have a start
  style and a wrap style (`#body()`, e.g. `#VM`). Both new designs were validated by spikes; the
  must functions need a second type check (only for packages that use them). Size raised to 6–7
  weeks.
