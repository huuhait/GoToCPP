# ADR-079: The typed interop checker reports only what the C++ compiler is going to report

Status: accepted (2026-10-19). Builds on [ADR-060](060-typed-interop-probes.md) (the probes) and
[ADR-074](074-typed-interop-declarations-and-foreign-types.md) (declarations and the foreign view); the
contract is [docs/spec/typed-interop.md](../spec/typed-interop.md) §7.

## Context

Stock `go/types` is silent about every expression that touches a C/C++ value, so opaque mode leaves
all of them to the C++ compiler: its errors come after the whole program is lowered, in C++ words,
and (when the lowerer stops at an error of its own) sometimes not at all until the first error is
fixed. Steps 12.1 to 12.4 give the checker what it needs: the C++ type of every such expression
(clang's answer to a probe), the error clang printed when the probe did not compile, the declarations
of the records and functions, and a Go view of the types (`ftype`).

The checker (step 12.5) has one rule above all others, the compatibility rule of the Phase 12 plan:
typed mode never rejects what opaque mode accepts, except where Go's own typing rules reject the
program (a void C++ call used as a value: assignment to blank, a switch tag, ...), these being the same
errors the checker gives for every other void context. A probe is not the generated code. It is the same
expression written with the same types, except where it cannot be, and a false error is worse than a
late one.

## Decision

1. **An error is reported only when it is certain, and everything else is silent.** The package
   `internal/typedinterop/icheck` has no warning level for doubt: a failure that is not certain is
   recorded in `Result.Silent` with its reason (for the tests and `gocpp __typed-check -v`) and the C++
   compiler decides, exactly as in opaque mode.
2. **Certain means an exact mirror** (`exact.go`). The reference must be written with real types only.
   Not exact, so silent: a stand-in of Go's slice, array or map (they are close to the runtime's types,
   not equal: `std::hash<gp::string>` exists in the runtime and not in the probe); a composite literal
   (the opaque build initializes a keyed C struct by assignments, not with braces: `c_literals.gp`
   has three probes that fail and a build that compiles); a part with no C++ spelling; the element of a
   range or a tuple; the alias of a local variable's type; a name that can be read two ways. The
   stand-in of `string` is the one exception, and only for the failures that it has exactly as the
   runtime's string (it converts to `std::string_view` and to nothing else): a refused conversion at the
   call, and the number of arguments. Per use, silent: inside a `defer` or `go` statement (the call is made
   from a lambda with the arguments stored in the frame) and inside a generic function or a method of a
   generic type (compiled only for the instances that are used, so an error in code that is never
   instantiated is no error of the program).
3. **The compiler must have said it.** The failure needs a diagnostic of the compiler for the expression
   (not a limit of the probe, an error that follows from another probe, or "no result"), and an error that
   follows from a failed part of the expression is the part's error, reported once. A package whose
   headers do not compile, or that the kind probe could not include, is skipped as a whole. A cross
   target is skipped (the probes read this machine's headers).
4. **Clang recovers from some errors, and the probe must say so.** A member of a pointer
   ("member reference type 'Point *' is a pointer"), a private member, and other errors are diagnosed and the
   alias is still in the dump with a type. `probe.Result.Recovered` (with `RecoveredAt` and
   `RecoveredNotes`) keeps the error of an OK probe, and the checker treats it as a failure. Without it the
   "pointer vs value receiver" and private-member errors were invisible.
5. **g++ builds get warnings.** The probe is clang++ because only clang has the JSON dump; headers
   branch on `__clang__` and the two compilers disagree on the edges, so with `Options.Build == build.GCC` (the
   default is the compiler `gocpp build` uses) every diagnostic is a warning with a line that says why.
6. **The typed rules on probes that did compile** (`rules.go`) use the declarations: a void result used as
   a value (`f() (no value) used as value`); a value that C++ cannot convert to the Go type it goes to
   (`ftype.Value.Compat` says `No`, refined by the declaration for a class: it has no conversion
   operator, also not in its bases, and no operator template); an assignment to a scalar C++ value that is not
   an lvalue or is const. Explicit conversions are not checked this way (the probe of the conversion is).
   The void rule is the one deliberate exception to "never rejects what opaque accepts": `_ = f()` and
   `switch f() {}` with a void C++ `f` build and run in opaque mode, and gc rejects both, so typed mode does
   too (`gorule_void.gp` pins the two contexts and the member-call form; `TestGoRuleZoo`).
7. **Messages are Go's words.** The wording of `go/types` where Go has the same error ("cannot use x
   (variable of type T) as U value in argument to f", "too many arguments in call to f (have 2, want 1)",
   "x.f undefined (type T has no field or method f)"), types in the foreign view's spelling (`C.int`,
   `*C.char`, `std.vector[C.long]`), the candidates of an overload failure as related lines, one line that
   says which mistake it looks like (an integer for an enum, a Go string for a `char*`), and a fallback
   ("X is not valid C++: ...") that keeps clang's message for the rest. Every template is a constant in
   `templates.go`, listed in `icheck.Templates`, and has an entry in `gocpp explain`
   (`TestInteropCheckerMessages`).
8. **Positions.** The error is at the argument (cannot use ...) or the selector (x.m undefined), as go/types
   puts it, found in the syntax tree; a name that is called is not a use of the name (the call has its own
   reference), so a method value `m := c.inc` is one error and the calls `c.inc()` are none. An interrupted
   run exits with the signal's status and says "interrupted" (it is not a skipped package).
9. **Integration.** `icheck.Check(ctx, pkg, fset, Options)` is the entry point. The only caller is the hidden
   command `gocpp __typed-check` (exit status 1 on errors, 3 without clang++); the mode switch and the
   clang-missing fallback are step 12.11, and nothing in the default pipeline, the language server or the
   build calls it, so opaque mode and the server are unchanged and typed mode is not on by default.

## Alternatives

- *A warning level for the doubtful cases.* Rejected: a warning that shows up for a program that builds
  teaches people to ignore the checker, and typed mode is meant to be quiet about every program of the
  corpus.
- *Fix the probes until they are exact* (a real `gp::string` and `gp::slice` in the probe file). Rejected
  for this step: the runtime headers are heavy (an include of them costs more than the probe), and they
  change with the runtime. The stand-ins stay, and the checker knows what they are not.
- *Only report errors in the lowering's words.* Rejected: the lowering stops at its first error, so a
  program with two mistakes shows one; the checker reports every use.

## Evidence

- Accepted corpus (`TestAcceptCorpus`): every program of `tests/interop`, `tests/zoo` (the C and C++
  headers, sqlite3, zlib, SDL2, raylib), `tests/run`, `tests/golden`, `examples/*` and `tests/pkgs/*` that
  opaque mode accepts and that imports headers: 251 programs, 4,353 probes that compiled, 5,976 uses,
  93 uses left to the compiler (86 inside `defer`/`go`, 8 inside generic functions, 3 composite literals):
  no error and no warning.
- Rejected zoo (`testdata/zoo`, 21 programs, 76 diagnostics, exact messages in `.golden` files): argument
  type, arity, enum, overload and ambiguity, missing and private members, const-correctness, reference
  parameters, operators, assignments and conversions to Go types, void values, assignments to temporaries,
  non-functions, unknown names, deleted functions, templates, pointers to pointers, and sqlite3, zlib, SDL2
  and raylib. `TestTypedCheckZoo` (cmd/gocpp) builds each program with the real `gocpp build`, with clang++ and g++:
  every line the checker reports fails to compile (the lowering stops at its own errors, so a line the whole
  build does not reach is built alone), and the accepted programs build.
- Mutation checks: 33 mutants of the exactness, site, gating and typed rules (composite literals, stand-ins, alternative readings, cascades, defer/go, generics, g++ warnings, header errors, recovered errors, void, Compat, class conversions, left sides, argument finding, rule order): all killed after the tests were added for the ones that first survived; one mutant is equivalent (the check for an unmapped base class, which `MembersOf` already makes Unknown).

## Consequences

- The checker finds the errors of the probes that mirror the code and is silent about `defer`, `go`,
  generic code, composite literals and the other stand-ins, so those still fail late and in C++'s words.
  Making them certain is a matter of exact probes (the real runtime types, the real `defer` lambda), not of the
  checker.
- clang does not diagnose everything the build does (`Result.Recovered` found two cases); a case that
  neither clang nor the declarations see is a late error, never a false one.
- Not done in this step: the narrowing warnings of README §2.3 (they belong to `gocpp vet`, Phase 13), the
  "name used through the wrong header" warning (it needs the header location of each name, step 12.13), and
  a check of an incomplete type used as a value (`var o sqlite3.sqlite3`).
