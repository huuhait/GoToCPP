# ADR-037: `!` is a rewrite of the Go AST around go/types, and the lowerer sees plain Go

Status: accepted (2026-10-18); the Gate paragraph is superseded by [ADR-095](095-must-and-macros-are-always-available.md). Amended by [ADR-124](124-error-return-traces-are-a-register-of-hops-per-goroutine-matched-by-error-identity.md) (proposed): the error branch gets one more statement, a call of the second helper `gp_must.Hop`, which stays in the output like `Exit`. Builds [ADR-013](013-must-errors.md) (what `!` means) with
steps 10.2 to 10.6 of [Phase 10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md).
It replaces the README's plan to lower the helper calls in the C++ lowerer (README §2.1 items 5
to 7) with a rewrite of the Go AST. Everything the user sees (the rules of
[language.md](../spec/language.md#must-functions-and-must-calls)) is as in ADR-013.

## Context

The parser (step 10.1) makes `F!(x)` and `func f!()` stock `go/ast` nodes whose names end with
`!` (package `gpast`); `go/types` is silent about them. README §2.1 says: before `go/types`,
replace `F!(a)` by a call of a hidden generic helper (`gp_must.M1(F(a))`) so that the stock
checker infers the types; after the check, lower each helper into control flow. It does not
say where the lowering runs. Doing it in the C++ lowerer (`internal/lower`) would mean writing
the early return, the zero values, the defers, the temporaries and the order of evaluation a
second time for every statement, expression and loop shape that the lowerer supports, and
keeping the two in step as the lowerer grows (generics, select, range-over-func).

## Decision

**The rewrite is in `internal/check` (files `must*.go`) and the lowerer never sees a `!`.** Four
steps, around the `go/types` checks that `checkPackage` already does (they are repeated when a
rewrite changes the package):

1. **prepare** (`must.go`): a must function gets its `error` result (unless the last declared
   result is `error`), and `return x` becomes `return x, nil`; `main!`/`init!` are left alone
   (no results). Each `F!(a)` becomes a call of a helper of the hidden package `gocpp/must`
   (`mustpkg.go`, made in Go code like `gocpp/cgoh`; the loader rejects the path, no `.gp` file
   imports it): `M1`, `M2`, `M3` for the calls that give values, `Any` for a statement. The
   import is added with the name `gp_must` (`gp_must_N` if the package uses it).
2. **rewriteDropped** (`mustdrop.go`): inside a must function a call that drops its error (a
   statement, or fewer values than it returns) gets a helper too. It needs the types of the
   callees, so it runs after a check, and again after each check until nothing is left (a call
   on a value that the first check could not type, `v := os.Open(p); v.Close()`).
3. **postCheck** (`mustdrop.go`): every helper call is checked. The last result of the call
   must be `types.Identical` to `error` (not only assignable: a `*MyErr` would make a nil
   pointer a non-nil error), and the number of values must be the one the helper gives. Each
   failure gets a message in the words of the user's code ("`typed! requires the last result of
   typed to be exactly error, got *MyErr: ...`"), and the `go/types` errors about the helper
   ("not enough arguments in call to gp_must.M1") go.
4. **lower** (`mustlower.go`): when the checks are clean, every helper call becomes the code a
   programmer writes by hand, in the Go AST, one more check types the new code, and the C++
   lowerer lowers plain Go. The contexts are those of README §2.1 item 6, each with a run test and a
   hand-written plain-Go twin: statements and assignments (`v, must_e := F(a); if must_e != nil
   { return ..., must_e }`), the left operands of a call (spilled into temporaries first, so Go's
   order holds), `&&`/`||` (a temporary and an `if` around the right side), `if`/`else if` (the
   code of an `else if` is inside the `else` block), `for` (condition: `for init; ; post {
   code; if !cond { break }; ... }`; post: a first-pass flag, `continue` still runs it), `switch`
   (an `if` chain, lazily in source order, when a case has code), `defer F!(a)` (a deferred
   closure; the function, receiver and arguments are evaluated at the `defer`), closures (own
   results), package-level `var` initializers (a function literal called at once, panic
   form), and generic functions.

**What happens on error** is picked by the function that holds the call: a last result `error`
returns zero values and the error (defers run); a `*testing.T`/`B`/`F` parameter calls
`t.Fatal(err)`; anything else panics with the error. `func main!()` and `func init!()` are
renamed in the first step (`prepare`) to `must_main`/`must_init1` and get the result `error` (so `return err`
works in them, and a bare `return` means `return nil`); the real `main`/`init` is a
wrapper that calls `gp_must.Exit(must_main())`, the one helper that stays: the lowerer emits
`gp::must_exit(err)` (`runtime/src/rt.cpp`: `<program>: <error>` on stderr, the program name cut at `/` or `\`,
`os::exit(1)`). `main`'s defers have run by then, because `must_main` returned. A plain `main`
that uses `F!` gets the same wrapper at the lowering.

**A deferred call that drops an error** (`defer r.Close!()`, or `defer r.Close()` in a must function)
in a function with no error result recovers a running panic first, makes the call, and
panics again with the value it recovered, so the error of the call is dropped and the panic goes on
(README §2.1); without a panic, the error is the function's panic as for any `!` call.

**Names.** The new variables are `must_t1`, `must_e1`, `must_f1`, `must_err`, `must_main`, with the
next free number if the user's code has such a name (`must_t1` taken: `must_t2`). The prefix is
not `gp_` because that is the prefix of the compiler's C++ names.

**The check and the soft errors.** The lowering runs only when the first checks had no error at all, soft ones (no new
variables, declared and not used) included: the check of the lowered code would drop them, and
its messages would name the code the lowering wrote. A helper call with no types and no error of its own
is a C or C++ function: "`std.accumulate!(...)`: ! is not supported on C and C++ functions yet".

**Synthetic names.** The names the pass creates have the places of the user's text (go/types and the
capture analysis of the C++ lowerer need places). `load.File.Synthetic` lists them (the names that are not in the tree
before `prepare`), and hover, definition, references, rename, highlight, semantic tokens and hints skip them.

**Positions.** The nodes the lowering makes have no position of their own; `anchor` gives each
a position that maps to the statement of the user's code it comes from (names that are used get
the end of the statement, names that are declared the start), so diagnostics, the C++ `#line`
and the capture analysis of lambdas (a variable whose declaration is before the literal) see the
original source. A helper call has the span of `F!(a)`. The language server's diagnostics are
the same as `gocpp check`'s for every program of `tests/errors` (its differential test now loads
with the Phase 10 syntax when its gate phase is 10 or later).

**`errors.Wrapf`** is `stdlib/errors/wrapf.gp`: `func Wrapf(errp *error, format string, args
...any)` sets `*errp` to an error with the text `context: original` that has `Unwrap` (so `Is`
and `As` work), and does nothing when `*errp` is nil. It is gocpp's own (D17). Its row is gone
from the gates (the member exists); `errors.Trace` stays gated.

**`defer f.Close()` in a function that returns an error** changes the error only when it is
nil, and leaves the other results as they were (`return v, nil` with a failing `Close` gives
`v, closeErr`). That is the classic Go pattern; the README's "other results become zero values"
is for returns that `!` itself makes.

**Gate.** `must calls` and `must functions` are gate rows of Phase 10; `version.Phase` stays 5.
The loader parses with `Phase10Syntax` only when the gate phase is 10 or later, and before that
`F!(x)` is the syntax error that the gates explain. Tests use `// phase: 10` and
`GOCPP_TEST_PHASE=10`.

## Tests

- 127 run tests `tests/run/must_*.gp`, each with a hand-written plain-Go twin in
  `tests/run/testdata/must_twin/`. The harness's `TestRunVsGo` runs the twin with real Go, and
  `// Output:` is what real Go printed, so the expected output of a program with `!` is never
  written from what gocpp prints (`tests/musttwin_test.go`, `tests/run/README.md`).
- `tests/mustgen` generates programs of random `!` forms (assignments, nested calls,
  `if`/`else if`/`switch`/`for` conditions, `&&`/`||`, defers, must functions with no `!` in the body) and
  their twins; `must_gen_00` to `must_gen_19` are the output for seed 1
  (`go run ./tests/mustgen -n 20 -seed 1 -out tests/run`).
- error tests in `tests/errors/must_*.gp` for every misuse that has a message.
- mutation checks (a mutant must fail a test): zero values of a propagation, the defer
  override of an error, the order of hoisting, `||` and `&&` short-circuiting, `else if`
  placement, the post statement of a loop, the condition of a loop, `Identical` to
  `error`, `Wrapf`, the exit code, the `return x, nil` rewrite, the panic form.

## Not built (gaps)

- `return two(bad)` and `return two!(a)` in a function whose declared results are not `error` and the call
  gives those results alone (forwarding): the pass cannot tell it from `return a, b` without types, and `!`
  gives back the values without the error, so `, nil` has no place to go.
- A call whose last result is another interface type than `error` (`type NamedErr error`) is not
  handled by a must function and is not rejected when dropped; `named!()` is rejected.
- `!` in the channel operation of a `select` case: rejected with a message (evaluate it before the `select`).
- `!` on C and C++ functions: rejected with a message. `go f()` in a must function: not handled, as README says;
  `gocpp vet` warns about it (the `gobang` analyzer, `internal/vet/testdata/gobang.txtar`). The `t.Fatal` form is run by
  the `gocpp test` fixtures `tests/gotest/testdata/must_testing` and `must_bench` (`TestGoTest/must_testing`).
- Hover text, inlay hints and go-to-definition that explain `!` (what happens on error): Phase 11's later work.
  The positions map to the original source and the synthetic names are skipped, which is all this step needed.
  Stepping through `for i := f!(); i < g!(); i++` in a debugger stops on the closing brace for the error checks.
- `gocpp expand` (prints the rewritten code) and the error return traces (`errors.Trace`): not built. The traces are designed in [ADR-124](124-error-return-traces-are-a-register-of-hops-per-goroutine-matched-by-error-identity.md): `failStmts` would add `gp_must.Hop(err, file, line, func)` as the first statement of each error branch.

## Test notes (2026-10-19, step 10.13; evidence only, the decisions above are unchanged)

- `tests/pkgs/must_cross`, `must_cross_initfail`, `must_cross_varfail`: must functions, methods, generic functions, `errors.Wrapf`,
  `defer Close!`, `init!` and package-level must calls across packages. Each has a Go twin module in `twin/` that
  `TestPkgsVsGo` builds in place of the module (`tests/pkgs/README.md`).
- The `t.Fatal` / `b.Fatal` form is tested now (the `Not built` note above is out of date): the `gocpp test` fixtures
  `tests/gotest/testdata/must_testing` (tests, a helper with `t.Helper()`, a subtest, a closure inside a test, a helper with `testing.TB`) and
  `must_bench` (benchmarks, `-run=^$ -bench=.`) compare the output with `go test` on a Go twin. `testing.TB` is a target of Fatal like `*testing.T`,
  `*testing.B` and `*testing.F`.
- `!` in the channel operation of a `select` case: `tests/errors/must_select_case.gp`; in a constant, an array length or a type:
  `must_places.gp` (the message says so since 10.13). `func TestX!(t *testing.T)` is an error of `gocpp test`.
- New run tests: `must_call_forms`, `must_ctx_select_body`, `must_ctx_order_recv` (a receive before a must call keeps its place),
  `must_keyword_not`, `must_spike_good`, `must_spike_noport`, `must_spike_missing` (the spike's three runs).
- `tests/golden/must.gp`, and `TestPhase10NoHelpers` (no `gocpp/must` or `gocpp/macro` helper in the C++ of every Phase 10 test program, the
  benchmark programs and the example).
- Overhead: `bench/phase10` and `docs/benchmarks/must-macros.md`: `!` is at the hand-written speed (1.00 with both compilers; a must function is
  faster with g++ because every call gets a fresh error variable).
