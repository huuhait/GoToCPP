# ADR-094: `gocpp vet` reads the `!` forms from the checker, and the macros from the loader

Status: accepted (2026-10-20), step 13.3b. Builds [ADR-037](037-must-rewrite.md) (`!` is a rewrite of
the Go AST), [ADR-075](075-user-macros-are-generic-functions.md) (macros are generic functions) and the
analyzers of [Phase 13 §2.3](../../plans/2026-10-13-gocpp/phase-13-tooling/README.md).

## Context

Step 13.3 built `gocpp vet` over the trees and types that `load` and `check` produce. Phase 10 had
not landed, so `panicbang` was a placeholder. Phase 10 did land, and by design the analyzers cannot
see its forms in the trees: the rewrite replaces `F!(x)` by `v, must_e := F(x); if must_e != nil {
return ..., must_e }` (or a `t.Fatal`, or a `panic`), gives a must function its `error` result, and
turns a macro into a generic function with hidden parameters, all before `go/types` and for good. An
analyzer for "a `!` that panics inside a library" has nothing to match: the `panic(must_e)` it could
find is also what a programmer writes by hand, and the programmer's `!` and the compiler's rewrite
of a dropped error in a must function look the same.

## Decision

1. **The checker keeps what the rewrite takes out** (`check.Package.Must`, `internal/check/mustinfo.go`):
   the must functions (the declaration in the tree, the name the user wrote, whether it is `main!` or
   `init!`) and each explicit `F!(...)` (its span in the original text, the `*ast.FuncDecl` or
   `*ast.FuncLit` that holds it, nil for the initializer of a package variable, whether it is a
   `defer`, and what it does on an error: return, `t.Fatal`, panic or exit). The lowering records a call
   when it replaces it, with the function context it already has (`fnCtx.node`, `fnCtx.exit`), so the
   kind is the one the generated code has by construction and cannot drift from it. A call that a
   must function makes with its error dropped (the rewrite's own) is not recorded: nothing in the source
   says `!` there. The info is nil for a package without `!` forms and for one with errors.
2. **The macros come from the loader's summary** (`load.Package.Macros`, which ADR-075 keeps for the
   uses of macros in other packages): the names, which parameters are lazy, which are given to
   `#text`, whether the macro is a wrap macro. The function that a macro became is found by its
   name; `gp_body` is the hidden parameter that `#body()` calls, and a lazy parameter is the
   parameter of the user's name.
3. **The analyzers get both through a hidden analyzer** (`gocppinputs`, `internal/vet/inputs.go`)
   that they `Require`. The driver of `internal/vet` takes its packages from `load` and `check`,
   not from `packages.Package`, so the inputs travel in a table keyed by the `*types.Package`, which
   `vet.Run` fills before the analysis and empties after it. The analyzers declare
   `RunDespiteErrors`, so the guard for packages with C/C++ names lets them run next to opaque
   values; they read the shapes they need (`C.f`, `gp_cgoh.Cstr`, the declared type of a variable) and
   skip anything else.
4. **`panicbang` is whole-program through facts**: a function that panics through a `!` (directly, or
   by a static call of one that does, in any package) carries a fact, and a library package that calls
   such a function of another package is told at the call. A function that calls `recover()` stops the
   chain. The direct `!` is reported where it is; the fact is only for the calls across packages, so
   nothing is said twice.
5. **`ignorederr` is for packages that use `!`.** A call statement that drops an error is reported only
   in a package that writes `!` somewhere: there the author has chosen to handle errors visibly, and a
   dropped one stands out. In a package of plain Go the analyzer says nothing (go vet has no such check;
   in the ports of the Go library it found the usual `f.Close()` in an error path and nothing worth
   a change).
6. **No suppression comment.** `go vet` has none; the way to say "this is intended" is in the code
   (`_ = f()` for `ignorederr`), or `-<analyzer>=false`.

## Alternatives

- **Parse the files again for the `!`** and match them with the types by position: the trees that
  `go/types` checked are the lowered ones, so the match would be the position of `F!(x)` against the
  lowered `F(x)`, and the function kind would have to be worked out a second time from the signature, with the
  rules for `*testing.T`, closures and `main` copied from the lowering. One source of truth is better.
- **Keep the `!` in the trees for vet** (`check.Config.KeepBang`): the lowering and the second type
  check are what give `go/types` the code to check; analyzers on the unlowered trees would see
  `gp_must` helper calls and half-typed expressions. A flag that changes the trees every analyzer
  reads is a bigger risk than a side table.
- **Analyzers on the `must_e` pattern of the lowered code** (a `panic(must_e)` is a panicking `!`): the
  names are fresh and not stable, and hand-written code of the same shape would be reported.

## Consequences

- The kinds are tested at the source (`TestMustInfo`, and `TestMustInfoCountsEveryCall`, which compares
  the number of entries with the number of `F!(...)` in the source of every must program of
  `tests/run`, so a lowering path that forgets to record a call fails it), and every analyzer has a
  testdata archive with positive and negative cases. Each new check was mutated (the condition removed
  or inverted, in a copy of the source) until a test failed; the equivalent mutants that remain are the
  repeat guard of `noteCall` and nothing else.
- `check.Package.Must` is available to other tools (the language server's hover on `F!`, the inlay
  hints of Phase 11) without a second walk of the rewrite.
- `narrowing` covers only the conversions that the user writes (`C.int(n)`); the implicit narrowing of
  typed interop needs the C++ types from the probes (step 12.11 makes typed mode the default) and is not
  reported yet. `cstr` keeps a copy of the "keeps the pointer" tables of `internal/lower/keeps.go`;
  `lower.KeeperFuncs`, `DestructorArgs` and `ConventionWords` (`keeps_export.go`) are read-only views of
  the lowering's tables, and a test of `internal/vet` compares them with the copy, so the lists cannot
  drift. Two rules of typed mode are not in `cstr`: the address of a char pointer given next to the
  string (`strtol(s, &end, 10)`) and the functions whose result points into the string (`strstr`): they need
  the C++ declarations of the probes.
