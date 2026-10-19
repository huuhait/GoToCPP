# ADR-095: `!`, macros and `#Derive` are always available: no gate, no `GOCPP_GATE_PHASE`

Status: accepted (2026-10-20), step 10.12. Supersedes the gate paragraph of
[ADR-037](037-must-rewrite.md) ("Gate. `must calls` and `must functions` are gate rows of Phase 10 ...")
and the "only when the parser took the syntax of Phase 10 (`Config.Phase10Syntax`, set when the phase
gates allow it)" clause of [ADR-058](058-builtin-macros-and-embed-in-the-loader.md). Nothing else in
those ADRs, in [ADR-013](013-must-errors.md), [ADR-014](014-macros.md),
[ADR-075](075-user-macros-are-generic-functions.md) or [ADR-080](080-derive-writes-methods-from-the-types.md)
changes: what `!`, a macro and `#Derive` mean is the same.

## Context

Phase 10 was built while `version.Phase` was 5, so its features sat behind the phase gates like every
other feature of a later phase: `internal/check` had the rows "must calls", "must functions", "macros" and
"//go:embed" (all Phase 10), the loader parsed the syntax only when `check.Config.Phase >= 10`
(`load.Config.Phase10Syntax`), and `gateUnchecked` turned the parser's syntax errors in a file with `F!(x)`,
`func f!()`, `#Name` or `macro` into "must calls are not supported yet (Phase 10)". Every program of
Phase 10 needed `GOCPP_GATE_PHASE=10`, and the test harness passed it for each test marked `// phase: 10`.

The gates exist so that nobody gets broken C++ from a feature that is not built (Phase 01 README §2.9).
That reason is gone for these features: they are built, tested against hand-written Go twins, and they
are the part of gocpp that is most different from Go; the user's order is that they work by default in
every command (`fmt`, `check`, `build`, `run`, `test`, `vet`, `expand`, `lsp`). Keeping the gate also made the
tools disagree: `gocpp fmt` and the language server's parser always read the syntax, `gocpp check` did
not, and the first thing a user who tried `examples/macros` saw was a "not supported yet" error for the
feature the examples are about.

## Decision

1. **The syntax is always on.** `load.Config.Phase10Syntax` is gone; the loader always parses with
   `parser.Phase10Syntax` and always runs the built-in macros, the user macros and `#Derive` (the passes
   that were behind the field). The parser's own option stays: it is how the fork keeps Go code
   parsing exactly like upstream's parser (`if!(a)`, `a!=b` and `macro := 1` are Go in either mode, and
   the patches' tests check that), and every gocpp tool sets it (`gocpp fmt`, `doc`, `expand`, `parse`, the
   language server's cache). The standard library is parsed with it too; it changes nothing for Go code.
2. **The gate rows are gone**: "must calls", "must functions", "macros" and "//go:embed" are no longer in
   `features`, `phase10Syntax` (the scan of a file that failed to parse) and its tests are gone, and
   `gateUnchecked` keeps only its other job (naming the phase of a standard package that has no stub). The
   hidden imports of the checker (`gocpp/must`, `gocpp/macro`) are skipped by the import gate: before,
   they were allowed because `otherStdPhase` (9) was below the gate phase 10.
3. **`embed`** is a row with `Phase: 10, Since: 5` (the stub says Phase 10 replaces it; its use for strings
   and byte slices works from Phase 05 on, like the generic packages). `embed.FS` still needs `io/fs`
   (Phase 09) and says so. **`errors.Trace`** stays a gate row at Phase 10: it is not built (the open
   points of the language spec). Mistakes in using the forms keep their messages (`F! must be called`,
   `cannot use f! here`, the macro, derive and embed errors): they are not gates.
4. **`version.Phase` and `GOCPP_GATE_PHASE` stay.** The gates of the other phases work as before
   (`GOCPP_GATE_PHASE=NN` can still open them), and `gocpp version` prints `phase 05` until the user
   decides that Phase 06 to 10 are finished. What a program of Phase 10 needs from other phases is
   still gated by those phases: a must program that imports `strings` or starts a goroutine gets the
   gate of Phase 09 or 08, not one of Phase 10.
5. **Tests.** The `// phase: NN` line of a test says the phase that makes the test pass (tests/README.md).
   A test of `!`, macros or `#Derive` no longer needs Phase 10, so its line now names the latest gate
   that its program meets (found by running `gocpp check` at each phase): Phase 05 for the programs that
   use only what the finished phases have, Phase 08 or 09 for the ones that use goroutines, `strings`,
   `os` and the like. The first group runs in a plain `go test ./tests`; the second runs as before with
   `GOCPP_TEST_PHASE`, and gets the gate of its own phase from the harness. `make ci` still runs up to
   Phase 10 (`CI_TEST_PHASE`), because other Phase 10 tests wait for a step (`errors.Trace`).

## Alternatives

- **Keep the gate and lift it by raising `version.Phase` to 10.** Rejected: that says Phases 06 to 09 are
  finished, which is not decided (the ports are being reviewed) and which would open their gates too.
- **A separate switch for the syntax (a `-must` flag, a `//gocpp:macros` directive).** Rejected: the user
  asked for no switch, and a second way to say "yes" is one more thing for the editor, `fmt` and the
  tests to disagree about.
- **Keep the parser option off for the standard library.** Rejected: nothing in the standard library is
  Go code that the syntax changes, the fork's tests check this on Go's own source, and a second mode in the
  loader is a second set of cached trees.
- **Leave the `// phase: 10` lines and add a harness exception for must, macro and derive programs.**
  Rejected: the line would then not say what makes the test pass, and the harness would need a list of
  names.

## Consequences

- Good: one behaviour in every tool; a user can try `examples/macros` with no environment; the tests of
  `!` and macros are part of the default `go test ./tests`; the loader and the checker have less code (the
  scan of a failed parse, two flags, a gate row for each form).
- Bad: a syntax error in a file with `!` or `#` is now only the parser's error (before Phase 10 it was
  replaced by the gate message, and the real error hidden): it shows with the usual position and text.
  A program that used `macro` or `lazy` as ordinary names still works (D18), and a program that has
  `#` or `name!(` outside a string was never Go.
- Other branches that touch these places need the changes listed in the report of step 10.12: tests that
  set `Config.Phase10Syntax`, that start `GOCPP_GATE_PHASE=10` for must or macro programs, or that expect
  "must calls are not supported yet (Phase 10)".

## Evidence

- `TestPhase10Always` (internal/check): must calls and functions, a macro with a lazy parameter, a
  `#Derive(String)` line, `//go:embed` and `#embed` check with no error at `version.Phase`.
- `tests/run` must, macro and derive programs run in a plain `go test ./tests` with g++ and clang++.
- A mutant that turns the gate back on (the loader without the parser option, or the rows back) fails the
  programs and the `gocpp check` tests at once.

## See

[Phase 10 README](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md), step 10.12;
[language.md D12, D13, D18](../spec/language.md#d12-must-for-errors); [feature-matrix.md](../spec/feature-matrix.md).
