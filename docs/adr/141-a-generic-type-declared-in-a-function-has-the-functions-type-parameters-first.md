# ADR-141: a generic type declared in a function has the function's type parameters first

Status: accepted (2026-10-22; job dev.genericgaps).

## Context

Go accepts a generic type declared inside a function body (`func f() { type Box[T any] struct{ v T } }`), also
inside a generic function, where it can use the function's type parameters. gc 1.25 builds and runs
`type Box[U any] struct{ u U; t T }` in `func g[T any]`, and its `%T` of `Box[string]` in `g[int]` is
`main.Box[int;string]`: the function's type arguments, a semicolon, the type's own. Such a type has no methods (a
method is declared at package level, on a package-level type). gocpp refused it with a gate error
(`generic types declared inside functions are not supported in gocpp`, phase `Never`), a row of the feature matrix
and a TODO line (L275).

## Decision

- The gate and the matrix row are gone. A generic type declared in a function that is not generic is lowered like
  a package-level generic type: ADR-030's instances, shared by the program, named like `%T`. The existing rule for
  local types (moved to the namespace with a free name, the identity text carries a tag, so two functions that
  declare a `Box` make two types, also with the same arguments) applies to the origin.
- In a generic function (or a method of a generic type) the type is what ADR-030 already makes of a local type: a
  synthetic generic origin (`internal/lower/localtypes.go`) whose type parameters are fresh copies of the
  function's, **followed by the type's own** fresh copies (constraints substituted the same way). A mention
  `Box[string]` in the body of `g[int]` becomes the instance `[int, string]` of that origin (the substituter's
  `local` callback gets the type arguments of the mention, already replaced, and appends them). Mentions inside
  the type itself (`next *Box[U]`) and in other local types of the function go through the same callback, so
  recursion works.
- `%T` and the identity text write the function's type arguments, `;`, then the type's own
  (`canon.localImplicit`): `main.Box[int,string;bool]`. A local type with no type parameter of its own keeps
  `main.loc[int,string]`. The `·N` number of a type that is a type argument counts generic and plain local
  types together, as gc's does (checked against `go run`).
- A C/C++ type argument (`Box[std.string]`, or a local generic type inside a generic function whose type
  argument is a C/C++ type that C++ deduces) stays the user error of ADR-030 point 5: the same limit as the
  package-level generic types.

## Range over a generic iterator with deduced C++ type arguments

`for s := range Seq(v)` with `v []std.string` and `Seq[T any](xs []T) func(yield func(T) bool)`: the instance is a
C++ function template (ADR-030) and go/types types the call as invalid, so the lowering has no C++ type for the
parameter of `yield`. It failed with `package variables of C/C++ values without a type` (the unnamed parameter was
taken for a package variable). Now the loop body is a lambda with `auto` parameters (the lambda already has a
template header), the iterator call gets it without the `gp::func<...>` wrapper (a `gp::func` converts from any
callable), the range variables are `auto` locals as `x := lib.f()` is, and a parameter that a closure captures is
boxed with `decltype`.

With written type arguments (`Seq[std.string](v)`, a variable that holds that call, the instance
`Each[std.string]` itself) go/types types nothing, and the loop was lowered as a range over a C++ container.
Before the functions are lowered, `typeExplicitIterators` records the instance's signature (from
`opaqueInstRef`, with the written C++ types; a deduced one is left invalid) as the type of each such range
expression, so every part of the lowering sees a range over a func. Expressions of the body that use the range
variables keep go/types' invalid types, so a range variable is a C/C++ value for the lowering's checks, also an
`int` index (`int(i)`). `iter.Seq[T]` with a C++ `T` is still a generic type with a C++ type argument (not built;
check reports it since dev.genericcxx, below).

`func Wrap[T any, S ~[]T](xs S)` with a C++ element: C++ cannot deduce `T`; it defaulted only when `T` followed
`S`. The template parameters of an instance are now written with each element after its slice, so the default
`slice_elem_of<S>` can name `S` whatever the order of the Go type parameters, and the explicit template
arguments of a call inside another instance use the same order.

## C/C++ type arguments: what check reports, and three more shapes (2026-10-23, job dev.genericcxx)

- **Check reports the limit of ADR-030 point 5**, so `gocpp check`, `vet`, the editor and the build agree (the build
  runs check first and stops at its errors; the lowering's own messages stay as a fallback):
  - an instance of a generic type written with a C/C++ type argument (`Box[std.string]`, `iter.Seq[std.string]`,
    `Box[G]` with `type G struct{ s std.string }`, an alias of an instance, a generic type declared in a function), in
    any declaration, literal or conversion, at the instance, once for the outermost one, with the build's words
    (`internal/check/genericc.go`). An alias of an unnamed type (`type Vec[T any] = []T`) and a constraint interface
    (`SliceOf[T]` in a type parameter list) are not instances. **Not inside a generic declaration** (a generic function,
    a method of a generic type, a generic type; manager decision, dev.genericcxx round 2): the lowering meets it only
    when it makes an instance, and a declaration that is never instantiated is never lowered, so `func Never[T any]() {
    var b Box[std.string] }` and `type W[T any] struct{ b Box[std.string] }` check clean until they are used;
  - a use of a generic function (a call or a func value) whose instance would make such an instance, at the use, with
    what the function does: `an instance of the generic type Box with a C/C++ type argument of a generic function is not
    supported yet (Wrap makes Box[T])`, `(Outer calls Wrap, which makes Box[T])`, `(Loc declares pair)`, `(Never makes
    Box[C.std.string])`, and for an instance of a generic type whose declaration has one, `(W[int] has
    Box[C.std.string])`. `genericcinst.go` keeps a summary per generic function and type: for each type parameter,
    what a C/C++ type argument for it makes (a generic type instantiated with it in the parameters, results or body, a
    type declared in the function, which takes the function's type parameters, not a local alias; through the generic
    functions it calls), and what every instance makes (an instance written with a C/C++ type argument in the
    declaration). Declarations that use each other are summarized to a fixpoint. A function of a package that the
    checker only has as export data (the editor's dependencies, `slices.Values`) is summarized from its signature
    (`iter.Seq[E]`). A type argument counts as a C/C++ type as the lowering makes it a stand-in or bakes a C/C++ part
    in. The build used to report this inside the generic function, at a place that depended on the lowering;
  - a package variable declared without a type from a call of a generic function whose type argument C++ deduces
    (`var it = Seq(words)`), whose C++ type the lowering cannot write in the header: `package variables of C/C++
    values without a type are not supported yet: write the type of it (var it T = ...) or the type arguments of Seq
    (Seq[std.string](...))` (`pkgcvars.go`; the build's `internal error` before): the fewest type arguments that
    make the others known, spelled from the argument they come from when it has a written type, else the type
    parameter's name (`Keys[map[int]std.string]`). With the type arguments written (`var it =
    Seq[std.string](words)`, also two variables from two results) the variable has the instance's result type. Any
    other C/C++ value without a type (`var s = std.to_string(5)`) is still refused by the build alone, now with the
    same words as a limit, not as an internal error: check's own tests check such packages, which are never built.

  Like the elided literal's limit (ADR-137), these are not reported next to a type error of go/types (an invalid type
  argument may be the program's own error, `Box[Undefined]`), and they are added after the opaque pass, so its sanity
  rule still holds for the program.
- **Partly written, partly deduced type arguments** (`Mixed[int](7, words)`, `words []std.string`): go/types makes the
  instance with the written `int` and an invalid `T`, and gives the generic name its generic signature (typed) and
  `Mixed[int]` the instance's (not complete). The opaque pass labels such an index expression opaque, and the
  conversion analysis counts it as no source of invalid (its sources are the call's arguments): the sanity rule's
  `internal error: expression has no type` is gone, and the instance is the template of point 3 of ADR-030.
- **Written or inferred core slice and map types with C/C++ parts.** `F[[]std.string](words)` for `S ~[]T`, and
  `Keys[map[int]std.string](m)` or `Keys(m)` for `M ~map[K]V`: the stand-in of `S` or `M` had no core type, and a range
  over it was an internal error. `generics.go: shapeArgs` makes the type argument `[]T` or `map[K]V` of the type
  arguments of the element parameters, as Go's core type unification does: a written part gives `T`, `K` or `V`
  (a written stand-in, or the Go type); an inferred map gives its Go key and leaves a C++ element open, which C++
  deduces from `gp::map<K, V>`. When go/types records no instance (an argument that is a C++ value), the map comes from
  the type of the argument of the parameter whose type is `M`. An inferred slice keeps the open `S` with its core type.
  A written slice or map of a Go type with C/C++ parts (`Count[[]Holder]`, which go/types types completely) stays as
  written and gives its parts to `T`, `K` and `V`, baked in as written type arguments are. The core type is one
  function for the lowering and check, `check.CoreType` (also through embedded interfaces).

## Consequences

- One row of the feature matrix and one gate go; `tests/errors/generics_local_generic_type.gp` is replaced by
  `generics_local_type_misuse.gp` (go/types' own errors).
- The tests `tests/run/generics_local_type_*.gp` are compared with real Go (`TestRunVsGo`); the interop tests are
  `tests/interop/generics_cxx_range_func.gp`, `generics_cxx_range_adapters.gp` and `generics_cxx_range_explicit.gp`
  (g++ and clang++, typed and opaque).
- dev.genericcxx: `tests/interop/generics_cxx_partial.gp`, `generics_cxx_shape.gp`, `generics_cxx_pkgvar.gp` (run tests),
  `generics_cxx_never.gp` (run tests), `generics_types_cxx_seq.gp`, `generics_types_cxx_inst.gp`,
  `generics_cxx_pkgvar_errors.gp`, `pkgvar_cvalue_error.gp` (build error tests),
  `tests/errors/generics_cxx_limits.gp` (check), and the unit tests `TestGenericCArgs`, `TestGenericInstancesWithCArgs`
  and `TestPkgCVars` of `internal/check`, and `TestDiagnosticsGenericCxxLimits` and
  `TestDiagnosticsGenericCxxStdIterators` (the editor). `generics_types_c_inferred.gp` and `generics_types_cxx_in_func.gp` now expect
  their error at the use of the generic function.
