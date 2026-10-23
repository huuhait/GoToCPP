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
`int` index (`int(i)`). `iter.Seq[T]` with a C++ `T` is still a generic type with a C++ type argument (not built).

`func Wrap[T any, S ~[]T](xs S)` with a C++ element: C++ cannot deduce `T`; it defaulted only when `T` followed
`S`. The template parameters of an instance are now written with each element after its slice, so the default
`slice_elem_of<S>` can name `S` whatever the order of the Go type parameters, and the explicit template
arguments of a call inside another instance use the same order.

## Consequences

- One row of the feature matrix and one gate go; `tests/errors/generics_local_generic_type.gp` is replaced by
  `generics_local_type_misuse.gp` (go/types' own errors).
- The tests `tests/run/generics_local_type_*.gp` are compared with real Go (`TestRunVsGo`); the interop tests are
  `tests/interop/generics_cxx_range_func.gp`, `generics_cxx_range_adapters.gp` and `generics_cxx_range_explicit.gp`
  (g++ and clang++, typed and opaque).
