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

## The standard library, vet and func fields with C++ values (2026-10-23, job dev.genericstd)

- **`make(S, n)` of a deduced `S ~[]E`** (`slices.Insert(c, 1, std.string("q"))`, whose `append(s[:i], make(S, n)...)`
  was `internal error: not supported yet: make of slices.gp_cxx_N_S`): `make`, the `append` of a `make` and the
  conversions `S(nil)`, `S(t)` and `[]E(s)` use the stand-in's core slice `[]E`, as `len`, `append` and `copy` did. A
  deduced `S` is a `gp::slice` in C++: gocpp has no named type over a slice of C/C++ values. `slices.Insert`, `Replace`,
  `Grow`, `Clone`, `Concat`, `Repeat`, `Compact`, `Delete`, `DeleteFunc`, `Reverse`, `Clip`, `Index`, `Equal` and
  `SortFunc` run on a `[]std.string`, deduced and written (`slices.Insert[[]std.string]`). `slices.Collect`,
  `Sorted`, `Values` and `maps.Collect`, `All` take or make an `iter.Seq[E]`, a generic type instance with a C++ type
  argument, which check reports at the call (above).
- **`maps.Clone` of a map with a C++ value** (`map[int]std.string`) panicked with `maps.Clone: the map type can't be
  copied`: the map is a foreign value in the `any` that `maps.clone` gets, and the foreign descriptor had no
  `gp_map_clone`. `foreign_ops_of` sets it for a `gp::map` as `ops_of` does (`typeops.hpp`).
- **`sync.OnceValue` and `OnceValues`** with a C++ result: Go 1.25 keeps their state in one unnamed struct with a field
  of type `T`, and gocpp has no unnamed struct type with a C/C++ type argument (a positioned limit inside
  `oncefunc.gp`). The port keeps the state in local variables that the closures capture and makes the function that
  `once.Do` runs once, Go 1.22's form with Go 1.25's `f = nil` in the deferred function (`stdlib/sync/PORT.md`): the
  same behavior; with `-release` a call takes 2 ns instead of 10-12 ns (the struct form made `Do`'s closure on every
  call), and making the function about 55-70 ns more (five boxes and `g` instead of one struct).
- **The name of a generic function of another package is no source of invalid** in the conversion analysis
  (`convanalysis.go`), as the name of one of the package is not: go/types gives `sync.OnceValue` the instance's
  signature with the invalid `T`, and `int(v.size())` of `v := once()` was left for the build, which said `internal
  error: ... passing C/C++ values to this function`. Its sources are the arguments of the call.
- **vet**: go/types records no type for a call that it could not instantiate (`Grow(words, std.string("x"))`, whose
  `T` C++ deduces): it records an invalid operand not at all, and a C/C++ value with the invalid type. The pruner of
  printf and nilness (`internal/vet/guard.go: touchesInvalid`) counts such a call as touching a C/C++ value, so it is
  replaced by an unknown value or its statement is left out; `refactor/satisfy` crashed on it (`no type for
  *ast.CallExpr`).
- **A field of func type with a C++ parameter.** A call of a func value in a field (`h.f(std.string("x"))`, `p.f(x)`)
  has the field's signature for the argument checks (`cguard.go: calledSig`), as a call of a func variable has; the
  build said `passing C/C++ values to this function`. In an instance of a generic type (`type H[T any] struct{ f
  func(std.string) T }`, `H[int]`), go/types makes a new field with `T` replaced, which has no type expression: its C++
  type is the generic declaration's type expression lowered with the types of its parts replaced by the type arguments
  (`typedecl.go: fieldType`), as the body of a generic function is for an instance. A local, a parameter or a result of a
  generic function whose written type has a C++ part and a type parameter (`var cb func(std.string) T`, `f
  func(std.string) T`, `func Ret[T any](v T) func(std.string) T`) gets its declaration's type expression the same way (the lowering keeps the variable each instance variable replaces, `Program.varOrigins`). The build's
  message for a field or an instance variable whose type it still cannot write no longer speaks of package variables.
  A generic struct of another package (`lib.H[int]`) takes the type expression and its types from the declaring
  package's lowering, and a type declared in a generic function (`type L struct{ f func(std.string) T }`) maps the
  fields of its synthetic origin back to the declared ones and substitutes the function's type parameters, read from the
  generic function's snapshot (an instance has replaced them in the package's maps).
- **More of a deduced `S ~[]E`** (round 2): the empty variadic of a call (`slices.Insert(e, 0)`, `Replace(e, 0, 1)`, `f(nil...)`)
  is `{}` where `E` is a template parameter out of scope at the call (it was `gp::slice<E>()`, a C++ error), so `E`
  takes its default from `S`; `S{...}` is the slice literal `[]E{...}`, empty and not nil (`slices.Clone` of an empty
  slice was nil).
- **The result of a deduced generic call is a gocpp container** (`deducedres.go`): go/types types neither the call
  (`slices.Insert(c, 1, cv)`) nor a variable declared from it, so `len(r)`, `r[1:]` and `append(r, x)` were refused as
  operations on a C/C++ value (with the advice `r.size()`, a C++ error). Before the C/C++ checks, the lowering gives
  the call, and the uses of a `:=` variable of it, the instance's result type with every type argument that C++ deduces
  replaced by the invalid type (an `S` by the type of its argument): `[]invalid`, the type go/types gives a
  `[]std.string`, or `map[int]invalid`, and a tuple (`v, ok = Get(m, k)`) whose Go parts are typed. A member of such a
  value is a Go-style error (`r.size undefined (r is a gocpp slice of C/C++ values, ...)`); check does not know the
  type, so that error is the build's.
- **Cross-package C/C++ APIs**: a function of another package with C/C++ parameters (`lib.Len(std.string("x"))`), a
  method of another package's type (`n.Call(cv)`) and a package variable with C/C++ parts (`lib.Ws`) are lowered as
  the package's own are; a range over a variable of another package declared from an instance with written C++ type
  arguments (`var Iter = Seq[std.string](words)`) gets the instance's signature, with the C/C++ types invalid.
- **A pointer to a gocpp func, slice, map or channel of C/C++ values** (`pf := &f`, `f func(std.string) int`) is a
  gocpp pointer to the boxed variable, not a raw C++ address, and `(*pf)(x)` is a call, not a conversion to a C
  pointer type. The box of `f := func(...)` takes the func literal's type expression.
- **A C/C++ map key** (`map[std.string]int`): `gp::map` hashes it with `std::hash` and compares it with `==`;
  `gp::cgo::map_key<K>` (interop.hpp) checks that it has them, so `map[std.vector[int]]int` is an error at the map's
  line (the build's; check does not know the C++ type). A struct key with a C/C++ field stays a limit (its `==`).
- **Round 3.** The retyping keeps the package's type maps shared: `setTypeTemp` writes into them and `lower` puts
  back what was there at its end (also the opaque marks it takes away, `check.Opaque.SetOpaque`), because a package
  whose maps are its own has typed interop off (`typed.go: inShared0`), which changed the checks of all its C/C++
  values (`return s.empty()` into a `bool` was refused). It covers `var r = ...` in functions and the bodies of generic
  functions (calls that do not depend on their type parameters; the function's snapshot keeps the types for its
  instances), and the Go values made of such results: `len(r)` and `cap(r)` are `int`, so is `len(r) + cap(r[:1])`, a
  comparison is a `bool` (no longer C/C++ values: `typeDeducedOperands`). Calls whose C++ type arguments are all
  written are not retyped (their C++ is exact). `len` of such a result that the lowering could not type no longer
  advises `r.size()`. **check reports what the build does**: a member of such a result (`r.size undefined (r is a gocpp
  slice of C/C++ values, ...)`, `checkDeducedMembers`), and a use of a generic function of another package whose body
  uses C/C++ names (`lib.Ret uses C/C++ names, so it can only be instantiated in its own package (Phase 12)`,
  `checkForeignCxxGenerics`; the editor, whose dependencies are export data without bodies, leaves this one to the
  build). Whether a C/C++ key type has `std::hash` and `==` only the C++ compiler knows: that error stays the
  build's (`gp::cgo::map_key`), outside typed mode.
- **Keys with C/C++ parts** (round 3): an array of C/C++ values (`map[[2]std.string]int`) is hashed element by
  element (`gp::array`'s `gp_hash`; `gp::cgo::key_ok` follows arrays), and a gocpp struct with a C/C++ field
  (`type K struct{ name std.string; n int }`) gets a defaulted `==` (deleted when a field has none) and a `gp_hash`
  template that requires `key_ok` of its C/C++ fields (`typedecl.go: cKeyMembers`), so neither is an error where the
  struct is no key. fmt sorts the keys as Go's fmtsort: a C++
  string by its compare, a C++ class by its `operator<` (`type_ops::gp_cmp`); a class key without one is an error at
  the print (`gp::cgo::fmt_arg`: `fmt prints the keys of a map in order, and a C++ class key needs an operator<`).
- **`==` of a struct or an array with C/C++ parts** (round 4, manager decision): the members above are also Go's `==`.
  `x == y`, `x != y` and the cases of a switch on such a value are `gp::cgo::value_eq(x, y)` (interop.hpp), which
  calls the struct's `operator==` (or `gp::array`'s) and, for a C/C++ field type without `==`, is an error at the
  comparison's line instead of the build's former `internal error: not supported yet: == on values with C/C++ parts
  (Phase 12)`. `any == any` and `map[any]T` keys of such values compare the same way, as in Go (dev panicked with
  "comparing uncomparable type"). The members' constraints start with `std::same_as<gp_S, S>`, so a comparison
  with another type is no match, not an error. A struct that Go could not compare apart from its C/C++ parts (a slice,
  map or func field, also in a nested struct or an array of them) gets no `==` and no `gp_hash` (round 6:
  `comparableApartFromC`; the members compared every field but constrained only the C/C++ ones, so such a struct in an
  interface was a C++ error), and `any == any` and `map[any]T` keys of it panic as in Go. The types that the lowering gives deduced generic results are put back
  at the end of `Program.Lower`, not of each package's `lower`: the instances of generic types (their methods'
  bodies) are lowered after every package (`finishShared`).
- **Local type tags** (`localtypes.go`): the place of a type declared in a generic function, in its tag, is the offset
  from the function's start in one file, and the file's name with the offset across files (a `#Derive` method), so
  the names of instances no longer change when an unrelated file grows.

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
  their error at the use of the generic function. dev.genericstd:
  `tests/interop/generics_cxx_stdlib.gp`, `generics_cxx_slice_make.gp` and `generics_cxx_func_field.gp` (run tests), the
  printf case `opaque_generic_call` of `internal/vet/testdata/printf.txtar`, and `TestConversionQualifiedGeneric` of
  `internal/check`. Round 2:
  `tests/interop/generics_cxx_results.gp`, `generics_cxx_results_error.gp`, `cxx_xpkg_api/`, `cxx_func_pointer.gp`,
  `map_cxx_key.gp`, `map_cxx_key_error.gp`, more cases in `generics_cxx_stdlib.gp` and `generics_cxx_func_field.gp`,
  and `TestPlaceInFuncAcrossFiles` of `internal/lower`. Round 3: `tests/typed/generics_deduced_typed_checks.gp`,
  `tests/interop/map_cxx_key_class.gp` (with `map_cxx_key_class.hpp`), `map_cxx_key_print_error.gp`,
  `tests/errors/generics_cxx_deduced_member.gp`, `tests/pkgs/errors/generics_cxx_foreign/`, and more cases in
  `generics_cxx_results.gp`. Round 4: `tests/interop/cxx_struct_eq.gp`,
  `cxx_struct_eq_error.gp`, `generics_cxx_results_methods.gp`, `generics_cxx_results_xpkg/`. Round 6: `tests/interop/cxx_struct_uncomparable.gp`.
