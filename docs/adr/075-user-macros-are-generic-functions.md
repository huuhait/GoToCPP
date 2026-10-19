# ADR-075: User macros are generic functions, written by the loader

Status: accepted (2026-10-19), step 10.9. Builds [ADR-014](014-macros.md) (what a macro is) and
[ADR-058](058-builtin-macros-and-embed-in-the-loader.md) (the built-in macros as a rewrite in the
loader). It changes three details of ADR-014 and the plan (README §2.2): the place of a use is
passed as constants and not as a `gocpp/macro.Site`, a macro is not an "inline template in the
header" but a generic function that the package that uses it instantiates, and the function keeps the
name of the macro.

## Context

The plan turns a macro into a function with a hidden `gp_site *Site` parameter, a use into a call with
one static `Site`, a `lazy T` into `func() T`, a wrap macro's code into a `func()` argument, and emits
the macro "as an inline template in the package header" with the lazy parameters and `gp_body` as
template parameters. Building it showed what the pieces cost:

- **A `Site` struct needs a type that the lowering does not know.** `gocpp/macro` is a hidden package
  made with `go/types` calls (like `gocpp/must`); its `Site` would have to become a C++ struct, with
  one static object per use and `Outer` pointers for nested macros. What a macro reads from it is
  four things: the file, the line, the function of the outermost use, and the text of some arguments.
- **A header template would lower a body outside any package's unit.** The lowering writes a function
  into the translation unit of its package and can write another package's generic function into the
  unit that asks for it ([ADR-030](030-generic-instances.md): names fully qualified, descriptors asked
  from their owners). That already is "an inline template that the C++ compiler inlines across
  packages", without LTO and without a second way to lower a body.
- **A `func()` parameter allocates.** `gp::func` is a shared owner of a closure object: a lazy argument
  that captures a variable would be one `make_shared` per use.

## Decision

1. **The rewrite is in the loader** (`internal/load/macros.go`, in the walk of the built-in macros,
   `builtins.go`), after the imports are known and before `go/types` sees anything, in the Go AST.
   A macro is type-checked once, where it is declared; the lowering and every later pass see plain Go.

2. **A macro is a generic function with hidden parameters, with the macro's own name.**
   `macro Assert(cond bool, msg lazy string) { ... }` becomes
   `func Assert[gp_L_msg ~func() string](gp_file string, gp_line int, gp_func string, gp_text_cond string, cond bool, msg gp_L_msg)`:
   - `gp_file`, `gp_line`, `gp_func`: the place of the **outermost** use. `#file()`, `#line()` and
     `#func()` in the body read them. A macro used inside a macro passes its own three on, so the place
     stays the user's; this is the `Outer` chain of the plan, flattened.
   - `gp_text_x` for each parameter `x` that the body gives to `#text(x)`: the text that the use wrote
     for it, a string constant. The inner use passes the text of its own arguments (as written in the
     macro's body), so `#text` is the innermost. `#text(expr)` of anything else is the constant text, as
     for the built-in.
   - a `lazy T` parameter has a type parameter `gp_L_name ~func() T`, and the use passes
     `func() T { return arg }`. `T` has to be written at the use, so it is a predeclared type, a type of
     the macro's package that the use can name (exported when the use is in another package), or a type
     of a package that the file of the use imports (the loader says "add the import"); a struct or
     interface type with members, an array whose length is not a number and the macro's own type
     parameter are errors at the declaration.
   - a wrap macro (its body uses `#body()`) has `gp_body gp_B` (`gp_B ~func()`) before the other
     parameters, and `#body()` is `gp_body()`.
   - the hidden type parameters come after the macro's own, so `#Dbg[float64](1)` keeps meaning
     `T = float64`.
   A use `#util.Assert(c, m)` is `util.Assert("main.gp", 12, "main.f", "c", c, func() string { return m })`.
   The function keeps the macro's name, so the editor finds it (hover, definition, references and
   rename work on a macro as on a func, with the right ranges: the loader gives the name the place of
   the name in the source), and a macro is exported exactly when Go would export a func of that name.
   Names that start with `gp_` are reserved in macros (an error) so that no variable hides a hidden
   parameter.

3. **On top of a func.** The lines of macros are taken out of the declaration list and applied from the
   last line to the first, so the first is the outermost:
   - start style: the first statement of the function. A macro whose only result is exactly `func()`
     becomes `if gp_deferN := M(...); gp_deferN != nil { defer gp_deferN() }` (the variable is declared
     at the function's brace, which matters for the closure capture analysis of a wrap macro written
     after it); any other macro is an expression statement.
   - wrap style: the code of the function moves, **unchanged**, into a func literal with the function's
     old results, which a func literal that starts at the function's brace calls and whose values it
     assigns to the function's new results (`gp_r0`, ...): the function's body is
     `M(..., func() { gp_r0, gp_r1 = func() (n int, err error) { code }() }); return`. Go's own rules
     then apply to the code (a variable that hides a result, a defer that changes a result,
     `return f()`, a bare return); the first version rewrote each `return` into assignments to the
     result names, which was wrong when an inner variable hid a name (`if err := g(); err != nil { return 7, err }`
     returned `7 <nil>`), and its twins repeated the same rewrite, so no test saw it. A function with
     no results has no inner literal. Every run of `#body()` is a call of the body: named results start at
     zero. Several wrap macros nest literals that have the same range (the code's braces): only the
     first, innermost, one declares variables (the old results of the code), so the lowering's capture by
     position needs nothing else, and the number of wrap macros does not depend on how the source is laid out. `recover()` called by the function itself
     is an error (its code runs one level deeper than recover needs).
   - the declaration and its body start at the first line of macros (`FuncType.Func` and
     `Body.Lbrace`), so the language server finds the calls that were the lines.
   - a wrap macro on a must function (`func f!()`) is an error for now: the two rewrites do not combine.

4. **The lowering** (`internal/lower/generics.go`, `closures.go`, `expr.go`, `box.go`): a parameter whose type
   is a type parameter named `gp_L_*`, `gp_LN_*`, `gp_B` or `gp_BN` is a **C++ template parameter of the
   instance** (`template <typename gp_L_msg> void gp_inst_..._Assert(..., gp_L_msg msg)`), and a func
   literal passed to it is written as the lambda itself, not as `gp::func<...>(lambda)`: no allocation,
   and the C++ compiler sees the code of the lazy argument and of the wrapped function at the call.
   The instance is written, as for every generic function, in the unit of the package that uses it, which is
   why a macro is inlined across packages. Everything else about the body is the lowering of a generic
   function.
   **By reference.** The loader decides, from the body of the macro, whether a closure parameter *escapes*
   (`onlyCalled`): it does not if the body only calls it where it stands, and not inside a func
   literal, a `go` or `defer`, an argument of another macro, or as a value (a store, a return, passing on).
   Such a parameter is named `gp_LN_x` / `gp_BN`. The lambda for it captures the caller's variables **by
   reference** (`[&x, &i]`), and the boxing analysis does not box a variable for such a lambda (a
   variable that a lazy argument reads and the loop changes used to get a new heap box in each
   iteration, 9 s for 10^9 iterations; and the results of a wrapped function a box for each call). The
   decision is in the type parameter's name, so it crosses packages with the export data and the summary,
   and the loader and the lowering decide the same way (`refArgs`). Any other use of the parameter keeps
   the copy (the lambda may outlive the call).

5. **Packages.** `load.Package.Macros` holds, for each macro, a `MacroSig` (the names and the laziness of
   the parameters, the spelled lazy types, which parameters ask for their text, whether the macro is
   a wrap macro or returns `func()`, its place and its declaration as written, for messages and hover).
   An importing package is processed after its imports, so the rewrite of `#pkg.Name` reads
   `dep.Macros`; a summary of a package keeps them (`Package.Summary`, `NewSummary`, the language
   server's disk cache, format 5). A dot import brings the macros too.

6. **Messages.** The function that stands for a macro is a function to `go/types`, so `internal/check`
   (`macromsg.go`) gives the messages back the macro's words: "in argument to #util.Assert", "macro Assert
   is declared at util/util.gp:5" (the error is at the use; the note is the definition), and for the
   uses that are not calls of the rewrite: `Assert(x)` and `f := util.Assert` are "Assert is a macro:
   write #Assert(...)" (the errors of `go/types` inside that call are dropped), and a call of the rewrite
   whose name a local variable hides (`show := 5` next to `#show(1)`) is "the macro show is hidden here
   by a variable". The loader's own errors (arity, where a use may stand, `!` in a lazy argument, the
   rules of §2.2) have entries in `gocpp explain`.

## Rules (v1), as built

A macro: package level only (the parser); the name is not a built-in macro's and is declared once; no
receiver; parameters do not start with `gp_`; a lazy parameter has a type that every use can write. A
use: `#Name(...)` or `#pkg.Name(...)` (also with type arguments), with one argument for each
parameter (variadic as for a func, `xs...` only for the variadic parameter); not as the call of `defer`
or `go`; a wrap macro only on top of a func; a macro whose result is `func()` is not a statement inside
code; no `!` in a lazy argument (outside a func literal in it); only `#Derive` on a type (step 10.10); an
unexported macro only in its package.

## Alternatives

- **The plan's `Site` and header templates** (above): more work in the lowering for what four constants
  and an existing mechanism give.
- **A suffix on the function's name** (`Assert_macro`): a plain call then does not find the name, which
  needs no check, and a local variable `assert` cannot hide the macro. It made every range that the
  language server reports (hover, rename, highlights, semantic tokens) too long by the suffix, and
  rename of a macro would have edited the text after its name; fixing each of those places in the
  editor was more work than the check in `internal/check`. The price of the choice: **a macro
  is not hygienic about its own name** (a variable of the function with the macro's name is an error at
  the use, not a silent call of the variable), and a macro and a func of a package can't share a name.
- **`lazy T` as `func() T` with `gp::func`:** one allocation for each lazy argument that captures.

## Consequences

- Cost of a macro, measured at `-release` with 10^9 iterations of a loop (g++ / clang++ 21, same machine,
  the numbers are the best of 3 runs; "hand" is the same code written out):
  - `#Assert(sum >= 0, fmt.Sprint("neg ", sum, " at ", i))` in the loop: 9323 / 10942 ms before the lambdas
    captured by reference, 750 / 380 ms now; hand-written 280 / 350 ms. (g++ keeps `sum` in memory because a
    reference to it is held by the lambda; clang++ is at the hand-written speed.)
  - a lazy message that captures only the loop variable: 8703 / 10641 ms before, 303 / 200 ms now (hand 218 / 175).
  - a passing assert with a constant message, and a generic `Dbg` with `on == false`: at the hand-written speed.
  - a wrap macro (`#Count`, 10^8 calls): 1798 / 1199 ms before. Now the macro itself is at the hand-written speed
    (a wrap macro that does nothing: 138 / 187 ms against 155 / 119 ms for 10^8 calls). What remains in that
    benchmark is the `&counter` argument (a `gp::ptr` to a package variable, 400 ms for 10^8 calls with a plain
    func too).
- What is not free: a macro with no lazy parameter, no type parameter and no `#body()` is an ordinary
  function of its package (a call that the C++ compiler cannot inline without LTO), like any function of another
  package; a lazy parameter or a wrap macro that the macro lets escape (`Once`, `InThread`, a stored lazy)
  keeps copying, and its captured variables that are changed are boxed (the escape analysis of Phase 15 will
  help); the hidden `gp::string` arguments (file, function, text) are built for every call unless the C++
  compiler removes them.
- `gocpp/macro` of step 10.2 (`Site`) is not used (its package and its check code are removed; the path
  stays reserved).
- A macro shows in `go doc`-like tools (`gocpp doc`, which parses the source itself) as `macro Name(...)`,
  and in the editor's hover and completion as the declaration as written; the function with its hidden
  parameters shows only in messages that are not about a use.
- Positions: the numbering of func literals in stack traces (`main.f.func1`) counts the literals that the
  rewrite writes (a lazy argument, the code of a wrap macro). `#func()` is computed before, so it is
  right; only the names in a stack trace shift.
- A macro is not hygienic about its own name, and a name that is predeclared (`min`, `error`) can not be a
  macro's.

## Tests

`tests/run/umacro_*.gp` (eight programs, each with a hand-written Go twin in
`tests/run/testdata/macro_twin/` that real Go runs: `TestRunVsGo` takes the expected output from it),
`tests/pkgs/macro_user_cross` (three packages, a dot import, another name for a package, generic
macros instantiated in main, macros that use macros of another package, private names of the macro's
package in an instance; its output was checked with a hand-expanded Go module),
`tests/errors/macro_user_*.gp`, `tests/pkgs/errors/macro_user_cross*`, `tests/golden/macros_user.gp`,
`internal/load/macros_test.go`, the language server's `TestMacroNavigation`, `TestMacroWrappedBody` and
`TestDiagnosticsMacros`, and `internal/explain`'s `TestMacroMessagesCovered`. (`umacro_must` mixes macros
with `!`.) Mutations that the tests catch (each was made in a copy and
a test failed; 23 for the first version and 12 more for the review fixes): the order of stacked macros, the nil test of a deferred `func()` and the detection of a `func()` result, the
text of the arguments and the hidden text parameter, the place that a nested use passes (its own instead of
the outermost), a lazy argument not wrapped as a func literal, the lowering's raw lambda and its template
parameters, the check for a plain use of a macro and for a macro hidden by a variable, the dot import, the
qualifier of a lazy type of another package, the macros of a summary, the position of the wrapped code's func
literal and the span of its call, the hover of a macro, `!` in a lazy argument, a `func()` macro as a
statement. Review fixes: the new results of a wrapped function, the inner literal's old results, stacked wrap macros (3, 8 and 20), `recover()` in a wrapped function, the on-top names for header packages, the signature of an opaque instance, the escape analysis (both ways), the boxer and the capture by reference, C types in a lazy parameter, a predeclared macro name.
