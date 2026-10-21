# Phase 10: Beyond Go: `!` (must) & Macros · STATE

**Status:** `[~]` in progress (10.1-10.6, 10.8-10.10 merged; open: 10.7 (optional), 10.11, 10.12, 10.13; `#Derive(JSON)` waits for `encoding/json` (9.11))
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-19

## Steps

- [x] Spike: `F!()` rewrite into hidden generic helpers works with **stock** go/types (types inferred, wrong last result rejected, statement form post-checked) (2026-10-13)
- [x] Spike: first macro design (functions with an explicit site parameter + lazy arguments), generic `Dbg`, lazy argument evaluated 0 times on success (2026-10-13)
- [x] Spike: stock go/printer prints `os.ReadFile!(path)` and `#assert(n > 0)` (2026-10-13)
- [x] Spike: `macro` declarations + on-top macros (`Trace` enter/exit incl. on panic, `Locked` with 100 concurrent calls, race-clean) as plain Go after rewrite (2026-10-13)
- [x] Spike: must functions `func f!() T` on stock go/types: check 1 has 5 expected errors, check 2 has 0; the program runs right for a good config, a missing port and a missing file (2026-10-13)
- [x] Spike: wrap-style macros with `#body()` (`VM`, `Recover`, `InThread`, `Once`) as plain Go after rewrite, race-clean (2026-10-13)
- [x] 10.1 Parser + printer
  - [x] patch #4: `name!(`, `name![` (non-keyword identifiers only; `if!(a)` etc. unchanged), in calls and `func` declarations
  - [x] patch #5: `#Name`
  - [x] patch #6: `macro` declarations (package level), `lazy T` params, package-level on-top `#Name(...)` lines
  - [x] `go/printer` fork printing `macro`, `lazy`, `func f!()`, on-top macros; `gocpp fmt` switches to it
  - [x] tests + round-trip
- [x] 10.2 Hidden packages `gocpp/must` (M1, M2, M3, Any) and `gocpp/macro` (Site + `Outer`), aliased `gp_must`/`gp_macro`
- [x] 10.3 `F!` call rewrite + post-check on every form (`Identical` to `error`) + friendly error messages
- [x] 10.4 Must functions
  - [x] signature (`error` added unless declared) + `return x` → `return x, nil`
  - [x] two checks: find dropped errors with the first check's signatures, rewrite, check again
  - [x] declared-error form (`(cfg Config, err error)` + `errors.Wrapf`)
  - [x] `main!` / `init!` (print + exit 1), methods
  - [x] `defer f.Close()` inside; closures not handled; `go f()` → vet warning
- [x] 10.5 `!` lowering
  - [x] propagate (zero values + error, defers run)
  - [x] panic without an error result (incl. `init`, package-level vars); `t.Fatal` in tests
  - [x] `main`: print error (`<program>` = base name of `os.Args[0]`), exit 1
  - [x] per-context shapes: left-hoisting, loop condition + post, `else if`, `case`, `&&`/`||`
  - [x] `defer F!()` as a deferred closure (args evaluated at the defer)
  - [x] closures; reject `go F!()` and `!` inside lazy arguments
- [x] 10.6 `errors.Wrapf(&err, ...)`
- [ ] 10.7 (optional) Error return traces: design note, then `errors.Trace` / `%+v`
- [x] 10.8 Built-in macros (`#file`, `#line`, `#func`, `#text`, `#body`, `#embed`) + `//go:embed`
- [x] 10.9 User macros (merged 2026-10-19; design deviates from the sub-steps below: generic functions with hidden constant arguments instead of header templates and `Site`: ADR-075)
  - [x] `macro` decl → inline header template with hidden `gp_site` param; `#file/#line/#func/#text` → use site
  - [x] uses inside code → calls with one static `Site` each
  - [x] `lazy T` → template callable (no `gp::func` allocation); concrete, spellable types only
  - [x] on-top, start style → first statement, nil-safe `defer` form, top-to-bottom order
  - [x] on-top, wrap style → code moved into a closure (returns rewritten), `gp_body` as a template callable, `#body()` rules
  - [x] nested macros via `Site.Outer` (`#text` innermost, `#file/#line/#func` outermost)
  - [x] cross-package macros via package summaries
  - [x] v1 rules: no plain calls/values, no statement use of `func()`-returning macros, package-level only
- [x] 10.10 `#Derive(...)` in two steps (stubs before go/types, bodies after): `String`, `Equal`, then `Clone`, struct-only `JSON`
- [x] 10.11 `gocpp expand` (merged 2026-10-19, ADR-091)
- [x] 10.12 Lift gates; spec D12/D13/D17/D18; feature matrix
- [x] 10.13 Tests: rules, every lowering context, differential vs hand-written Go twins (must calls, must functions, wrap macros), golden C++, overhead benchmarks (incl. cross-package)

## Exit criteria

- [x] `must-errors` and `macros` examples pass on GCC and Clang (TestPhase10ExamplesBothCompilers, with the real-Go twin)
- [x] Every `!` rule and lowering context tested; outputs equal their Go twins under real `go run`
- [x] The must-function spike's three runs pass as gocpp programs
- [x] No helper calls left in generated C++; no measurable overhead (incl. macros across packages): holds for `!` and generic-instance macros; a plain cross-package macro is a real call (ADR-075) and the lazy `#Assert` cold path differs (see the note of 2026-10-21)
- [x] Macro errors point at call site and definition
- [x] On-top macros: exit part runs on panic; `Locked`-style test race-clean; wrap macros behave as in the spike, race-clean
- [x] `gocpp fmt` round-trips `macro`, `lazy`, `func f!()`, on-top macros
- [x] `#Derive(String)` handles shared values; `#Derive(JSON)` matches `encoding/json`

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
- 2026-10-17: Job 10.1 merged after a clean round-2 review (parser patches 4-6, printer fork, gpast, ParserMode).
  Decisions: (1) macros and `!`/`#` forms are marked INSIDE the stock go/ast nodes (names with `!`/`#`, a macro
  declaration is a FuncDecl named `#Name`, lazy is a UnaryExpr with Op token.ILLEGAL, an on-top line is a GenDecl
  with Tok token.ILLEGAL holding a ValueSpec named `_`), not in a side table: a node a pass copies keeps its mark;
  the price is that go/types is SILENT about `F!(x)` and `#M(x)`, so every pass from 10.2 on must call
  `gpast.Leftovers` before go/types as an invariant, and use `gpast.DeclNameEnd` for a macro declaration's name
  span and `gpast.MacroBase` for export filters; (2) the mode bit `Phase10Syntax` is off by default and nothing
  but `gocpp fmt` and the LSP's formatting sets it: internal/load must pass it when the gate phase is >= 10;
  (3) `gocpp fmt` formats the new syntax before Phase 10 exists (like interfaces before Phase 05); (4) the printer
  fork is needed only for macro, lazy and on-top lines (the stock printer prints `func f!()`, `F!(x)`, `#Name(...)`
  as written: the spike was right); (5) known: a comment between an on-top line and its declaration gets a blank
  line before it (gofmt's rule for comments between top-level declarations), on-top macro arguments are not
  walked by the legacy object resolver (only matters with SkipObjectResolution off), internal/format/parens.go's
  workaround could now be simplified.

- 2026-10-20: Exit criteria review (job 10.derive, independent review clean after two rounds). The
  `#Derive` line is ticked: `#Derive(String)` means constants that share a value (ADR-080: integer
  enums only; the first constant in file order names the value), now tested across three files
  (`tests/pkgs/derive_enum_files`), and `#Derive(JSON)` output equals a plain struct under real Go
  for the `Marshal`/`Unmarshal`/`Encode`/`Decode` route (`tests/run/derive_json_shapes`, real-Go
  compared on g++ and clang++). Known difference kept as a documented limitation (ADR-102): error
  texts of a derived type name the hidden struct `gp_plain`. No compiler, macro or stdlib change.
  The other open lines (examples on both compilers, no helper calls and overhead, on-top macro exit
  and race) had focused-test evidence from the job (364 focused tests passed; `TestPhase10NoHelpers`
  passes), but they stay unticked until the user's final review; the clang `cross` macro/hand
  ratio is unstable across builds (0.97 to 1.27, layout effect, `docs/benchmarks/must-macros.md`).

- 2026-10-20: The two stale unticked sub-items of 10.4 and 10.5 were verified by job 13.vetexit (clean review): `defer f.Close()` in a must function, closures as their own context (a bare dropped-error call inside a closure stays dropped: `must_closure_bare_call_dropped`), the `go f()` vet warning (`gobang`), panic forms for functions without an error result, `init` (`must_init_plain_panic`), package-level variables and `t.Fatal` in tests (`TestGoTest/must_testing`, `must_bench`) are all implemented and tested against real Go.

- 2026-10-21: 10.evidence integrated from reviewed source (clean first-round review): the
  three open exit lines now have reproducible tests. Examples `must-errors` and `macros` run on g++
  and clang++ and match `output.txt`, which the real-Go twin also prints. Expansion tests assert
  structurally that `!` and macro instances use no hidden helper, no `gp::func`/`std::function`
  and no allocation; `!` equals hand-written code; and the machine code of a wrap macro, a must
  call and a cross-package wrap macro plus must call is identical to hand-written code on both
  compilers (everything inlines into `main`). Four `umacro_*` programs with Go twins pin that
  exit parts run on panic (exit status 2, order of defers and exit parts, recover) and the spike
  program; two race tests (`Locked` on a panicking method, the wrap spike from 16 goroutines)
  pass under TSan on both compilers and under real Go's detector for the twins. Mutation checks
  by coder and reviewer all failed as intended. Not covered and stated in the test header: a plain
  macro with no lazy parameter, type parameter or body is an ordinary cross-package call
  (ADR-075), and the cold path of a lazy `#Assert` differs from hand code (its overhead evidence
  stays the timing table in docs/benchmarks/must-macros.md). The machine-code test is Linux-only
  and takes about 140 s. Manager focused check: gofmt of the test file; no full sweeps.
