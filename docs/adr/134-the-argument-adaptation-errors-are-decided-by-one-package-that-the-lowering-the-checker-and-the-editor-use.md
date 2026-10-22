# ADR-134: The argument adaptation errors are decided by one package that the lowering, the checker and the editor use

Status: accepted (2026-10-21; user order "go for it" for the open gap of [ADR-130](130-argument-adaptations-are-one-table-over-the-parameter-type-and-the-go-argument.md))

## Context

ADR-130 added the table of argument adaptations to the lowering. Some rows refuse an argument instead of adapting it: a
constant that does not fit the parameter (`take_char(200)`), a constant outside the values of an enum, a runtime string at
the format place of a `printf`-like function, a string that a function keeps, `&p` for a `void**` in a deferred call. Those
are errors of the language, certain from the Go program and the declarations. They were reported only while the call was
lowered, so only `gocpp build` and `gocpp run` showed them: `gocpp check` printed nothing for `take_char(200)` and exited
0, `gocpp vet` too, and the editor had no such diagnostic. ADR-130 named this as an open gap with an estimate of 2.5 to 3
days, and recommended mirroring the constant range and the printf guard. The user ordered it done, for the rows in the order: the
constant ranges, the printf guard, then the other refusals if cheap.

The decisions of the table lived in `internal/lower` and read the state of its `gen` (the syntax tree's parents, the
holders of `C.cstr`, the probe's values, a mutating `srcText`). A second implementation in the checker would drift from
the first, with two sets of messages.

## Decision

**One decision function, in a package of data.** `internal/typedinterop/adapt` has the table (the rows, their parameter
verdicts, the keeps tables, the number and enum rules) and `Decide(mapper, call, i)`, which says for the argument `i` of a
call what happens to it: `Leave` (C++'s own rules decide), `Convert` (the row adapts it; the key says which row and what
detail) or `Refuse` (the message and where it points, the call or an argument). Its inputs are plain data: the callee's
overloads (`Sig`: the function type clang gave, its scope, the format attribute, the array spellings), the arguments
(`Arg`: the class, the constant, the variable of `&x`, the character type behind `&p`), the text of the call and its
arguments (`Call.Source`), and two questions about the program that need its syntax tree (`AliasUseOK`,
`MethodKeepsCharPtr`). It knows no syntax tree, no C++ and no compiler. `adapt.Describe` classifies a Go argument from a
`GoFacts` (what go/types recorded, whether it is a C/C++ value, a holder, a variable), which the lowering and the checker
each implement over their own state.

**The lowering applies the result.** `typedAdaptArgs` asks `Decide` for each argument that has a class, reports a `Refuse`
with the message it returns, and for a `Convert` calls the C++ writer of the row (`lowerers[key.Row]`). The tests of the rows
(`err_adapt_*`, `adapt_*`, the emit test) did not change: messages and positions are the same. The lowering keeps its own refusal
reporting: a cross build has no checker pass, and it is the defense if the two ever differ.

**`gocpp check` and `gocpp vet` report the same refusals.** `icheck.CheckReport` runs a second pass after the failed and the
compiled probes (`internal/typedinterop/icheck/adapt.go`). It goes over every call of the package, builds the same `adapt.Call`
from the checked package and the probe's report, asks `Decide`, and adds an error at the position of the decision with
the words of the decision. It reads the callee through `Report.Typed` (the typed view of an expression, shared with the
lowering's `tvalOf`), `probe.FuncNameOf` (the name that the declaration queries use) and `adapt.FuncSigs`/`MethodSigs`, so a
free function and a method of a C++ object are covered. These errors are **errors with either compiler**: the other
messages of the checker are warnings when the build uses g++ (clang's answer may differ from g++'s), but a constant that
does not fit is refused by the lowering whichever compiler builds, so a warning would be false. `gocpp vet` runs the checker and
shows them (`TestTypedVetReportsAdaptationErrors`).

The pass reports nothing where the lowering would not: a package that declares a generic function or type (the lowering is
not typed there, `typedOn`), a call with `...`, a callee without declarations or with a template among its overloads, an
argument whose class needs a fact that is not at hand. Three facts come from shared code in `internal/check`
(`Package.SourceText`, the user's spelling of an expression without changing the tree, which the lowering's messages now
use too; `Package.Holders`; `Package.IsCTypeExpr`/`DeclaredTypeExpr`), and `adapt.AliasUseOK`, `MethodKeepsCharPtr` and
`IsVariableName` serve both callers.

**The editor shows them from a background analysis, and never waits.** `internal/lsp/cache/adaptindex.go` keeps one entry per
package folder, for the package value of its last check. A package is scheduled after its ordinary check, when it has no error and has a call
that an adaptation could refuse (`icheck.HasAdaptableCalls`, a pass over the checked package with no compiler). The worker
runs what `gocpp check` runs (the probe and the declaration queries, `typedinterop.Analyze` with `Decls`) and keeps the
diagnostics of `icheck.Adaptations`, not the report: the entry is small. `Snapshot.AdaptDiagnostics` reads what is ready, without
a compiler, without disk access and without scheduling, and gives nothing for a package that is not ready, whose analysis failed
or found a header that does not compile (`skipReason`), or whose value is not the one that was analyzed. When an analysis ends the
pipeline calls `Config.OnAdapt(dir)` and the server runs the diagnostics of that package again, which publishes only what
changed. The diagnostics of a package value are kept across new snapshots (an edit in another folder does not make them flicker).
An edit of the package makes a new value; its diagnostic is gone until its analysis ends. `HeadersChanged` runs the analyses again.

## What `gocpp check` and the editor report now

| Row | Error | `check` | build | editor |
|---|---|---|---|---|
| `numconst` | a constant that overflows, is negative for an unsigned, or truncates, at a C number parameter | yes | yes | yes |
| `enum` | a constant outside the enum's values (range by [dcl.enum]) | yes | yes | yes |
| `cstr`/`stdstring`/`wide`: printf guard | a runtime string at a format place | yes | yes | yes |
| `cstr`/`wide`: keeps | a string for a function that keeps the pointer or returns one into it (names, methods of a class with a char pointer member) | yes | yes | yes |
| `scalarptr` | `&n` whose Go type cannot hold what the parameter writes, or is not the pointee | yes | yes | yes |
| `voidout` | `&p` for a `void**` in a `defer` or `go` call; `&end` of a char pointer next to a string | yes (a variable declared with its type) | yes | yes (same) |
| the address of an element, `&x` for an array parameter, overloads that disagree, a typed constant for an enum | C++'s own error | no (the compiler's error at build) | C++ error | no |

The last row is not a refusal of the table: the argument is not adapted, so the C++ compiler refuses the call at build time, and
`gocpp check` leaves it to C++ as it leaves every call the probe cannot mirror (the typed constant for an enum is a warning
with g++, an error with clang). They stay a gap, and it was looked at again after 12.args round 2 (typedef'd array parameters):
a rule "this call cannot compile" is not sound from the Go side alone. The address of an element or of a variable whose Go type has
the width of the parameter's pointee (`&xs[i]` of an `[]int32` for an `int*`) is accepted by C++, so refusing every such call
would be a false positive, and telling the two apart needs the C++ type rules (the lowering would have to decide what the
compiler decides). The exact list is the `leftToCpp` map of `cmd/gocpp/typedcheckadapt_test.go`, 12 of the 29 `err_adapt_*`
programs: `err_adapt_array_const`, `_array_float`, `_array_method`, `_array_voidptr`, `_array_writer`, `_array_typedef`,
`_array_typedef_const`, `_array_typedef_matrix`, `_array_typedef_method` (an array parameter, written with its brackets or through a
typedef), `err_adapt_element`, `err_adapt_overloads`, `err_adapt_typedconst`. The test does not compare these (check passes, the
build fails with a C++ line); it compares the other 17 and requires at least 16 of them.

## Costs

`gocpp check`, typed mode, compared with the same tree before this job (cold: an empty probe cache; warm: the best of 7):

| example | before cold / warm | after cold / warm |
|---|---|---|
| `cpp-std` | 1.98 s / 57 ms | 2.02 s / 61 ms |
| `mixed-sources` | 1.43 s / 51 ms | 1.41 s / 52 ms |
| `raylib` | 0.72 s / 58 ms | 0.76 s / 57 ms |
| `sqlite` | 0.72 s / 58 ms | 0.73 s / 58 ms |

The differences are the noise of the runs. The pass is a walk over the calls and one decision for each: 3.3 ms for a package of 4000
calls into C++ with a constant argument (`BenchmarkAdaptations`), and it runs only in a package that has headers. The editor's
analysis is the compiler time of `gocpp check` (the probe and the declaration queries) on a worker. Measured with
`TestAdaptRefusalReachesTheEditor` (a header of two functions, the real clang++, three rounds): the refusal is in the editor 2.7 to 2.8 s
after `didOpen` for the first analysis of a server (the probe cache is off in the test) and 0.53 s after an edit of the package; the
first publish of the package does not wait for it (under a millisecond with a compiler that is held:
`TestAdaptCompilerBlockedDoesNotBlockDiagnostics`). A header like Dear ImGui's is slower, as the other background data (ADR-126); the limit of an
analysis is 60 s.

## Rejected

- **Run the lowering inside `check`.** About a day, but `check` gets slower (it would write the whole C++), shows lowering "unsupported"
  errors as check errors, and ends the rule that `check` is the fast path.
- **A second copy of the rules in `icheck`.** The two would drift; the point of the job is one decision.
- **The language server keeps the probe's report and runs the pass on request.** It would hold the mapper, the declarations and the values of every
  package in memory; the entry keeps the diagnostics only.
- **A warning for g++.** See above: the lowering refuses with either compiler.

## Consequences

- A row of the table is added once, in `adapt`, with its refusals; `check`, `vet` and the editor get them. The test that keeps this
  true is `TestTypedCheckReportsAdaptationErrors`: every `err_adapt_*` program that the lowering refuses is refused by `check` with the same
  line, with either compiler.
- `gocpp vet`'s own copy of the keeps tables (`internal/vet/cinterop.go`) still has its drift test against the shared tables; the narrowing
  warnings that ADR-130 proposed for vet are still not built.
- Known gaps: the calls of a package that declares a generic function or type are not reported (they are not lowered typed
  either: `typedOn` is false there, a limit of the lowering that this ADR keeps and does not fix); the char pointer rules (`&end` next to a string, a C pointer variable) use the probe's type of
  the expression and else the written declaration, so a variable that has neither is left to the build, which knows more (`&p` of a parameter is
  refused by `check`: the probe types it); `gocpp check` for another GOOS/GOARCH than the host's runs no pass (the probes would read the wrong
  headers), so a cross build with C++ standard headers (`GOOS=darwin`) is lowered typed and refused at build only, while the editor, which analyzes
  the standard headers of the target, shows them; the editor's
  diagnostic leaves with the edit and comes back with the analysis; the array and element cases above.
