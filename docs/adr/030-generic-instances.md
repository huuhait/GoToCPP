# ADR-030: A generic function is lowered once for each instance, by the package that uses it

Status: accepted (2026-10-18). Replaces the template design of [ADR-017](017-generics-templates.md) for
generic functions (ADR-017's "our own monomorphizer (rejected)" and its C++ templates, hidden descriptor
arguments and `gp_call_<Method>` helpers); the rest of ADR-017 (explicit type arguments from `go/types`,
the range-over-func design, the ported packages) stands.

## Context

[ADR-017](017-generics-templates.md) and [Phase 07 §2](../../plans/2026-10-13-gocpp/phase-07-generics/README.md)
make a generic function a C++ function template, with the body lowered once in terms of `T`. Building
step 7.1 showed what that asks of the lowering:

- **`int` and `int64` are the same C++ type.** A template cannot tell them apart, so `any(x)`, `%T`,
  `case int:` and `x.(T)` need a hidden descriptor argument for each type parameter (README §2.4), and
  a descriptor of `[]T` or `map[string]T` has to be built at run time from `T`'s.
- **Every operation of the lowering depends on the Go type**, and there are hundreds of them: `a + b`
  of strings and of `int8` (the cast back), `T(x)` between all pairs of numbers, strings, byte slices
  and defined types, `len`, `append`, `make`, `copy`, `range` and `x[i]` of `S ~[]E`, `M ~map[K]V` and
  `T ~string | ~[]byte`, `==` of structs, arrays and interfaces (with Go's panic for the uncomparable
  ones), a method call that is `x.M()`, `p->M()`, `X::M(p)` or a promoted one. The README plans a
  polymorphic C++ helper for each (`gp::convert`, `gp_call_<Method>`, overloaded `gp::len`, ...), each a
  place where the generic code can differ from the same code written with the type arguments.
- **Named types over slices** (`type IDs []int` as `S ~[]E`) are a `gp::newtype` struct, not a
  `gp::slice`: a template body that appends to `S` needs both.

Go's own compiler monomorphizes (by shapes) for the same reason: the semantics of an operation depend
on the type.

## Decision

A generic function is **lowered once for each list of type arguments** that a call or a func value
names (an *instance*), as an ordinary C++ function, by the package that names it.

1. **The body is lowered as if it were written with the type arguments.** Every expression has the
   type that `go/types` recorded, with each type parameter replaced by its type argument
   (`internal/lower/subst.go`); so each operation is the one the lowering already has for that type, and
   `fmt.Printf("%T", x)` in `Max[int8]` prints `int8`. `go/types` has already checked the program: the
   instance cannot be ill-typed. The substitution replaces the `Types`, `Defs`, `Uses`, `Implicits`,
   `Instances` and `Selections` of the nodes of the generic function by those of the instance
   (`generics.go: apply`); the variables of the instance are new objects (the lowering's tables are
   keyed by variable and a variable has its type); a selection `x.String()` with `x` of type `T` is
   looked up again on the type argument (`types.LookupFieldOrMethod`), which finds the value method,
   the pointer method or the promoted one. A `go/types` selection can only be made by the checker, so
   the lowering has its own (`tinfo.go`).
2. **Where an instance is written.** In the translation unit (the `.gp` file, or `pkg_init.cpp`) of the
   function that asks for it, before that unit's functions, with a prototype in front. It is in the
   namespace of the *asking* package, even when the generic function is another package's: the body
   names that package's functions, variables and types fully qualified, as any use of them does
   (unexported ones too: they are in its header), and the descriptors it needs are asked from their
   owners as for any conversion. So a change in a package's generic function body changes the units
   that instantiate it, never a header. Two units that need the same instance have one copy each; the
   name of an instance, `gp_inst_<8 hex digits of a hash of the unit and the type arguments>_<name>`,
   has the hash so that two units' copies have different external names (no ODR question), and
   `gp_` names are generated names (a user name that starts with `gp_` gets a `_`, ADR-010), so no
   name clashes. The stack trace shows `main.Map[...]` for it, and `main.Map[...].func1` for a closure
   in it, as Go does (`runtime/src/trace.cpp`).
3. **Type arguments that are C/C++ types** (`Map[std.vector[int], string]`, `Id(cv)` with a C++ value
   `cv`): `go/types` records no instance (it cannot know the type), so the instance is made from the
   syntax (`generics.go: opaqueInstRef`). A written C++ type argument is spelled by the C type
   translator and baked into the instance; every other type argument is a **C++ template parameter** of
   the instance, which the C++ compiler deduces from the arguments of the call (`template <typename T>
   T gp_inst_..._Id(T x)`); a type argument that is written as a Go type or inferred by `go/types` is
   baked in as for any instance. Inside the body such a type is a named type with an empty struct as its
   underlying type, which the lowering writes as the C++ type (`types.go`); what a constraint of `any`
   or `comparable` allows (copy, pass, `==`, `var z T`, slices and maps of it) is plain C++; a method
   of a constraint called on it is a C++ member call (`x.size()`); boxing it in an interface and
   `fmt` use the foreign descriptors of Phase 04. `go/types` cannot check these constraints, so the C++
   compiler does, and its errors are user errors at the `.gp` line. The checker's opaque pass learned
   that a generic call with an opaque type argument or argument is opaque, and its conversion analysis
   that the result of such a call is a source of opaque values.
4. **Gates.** The standard packages `cmp`, `slices`, `maps` and `iter` are still stubs (`panic("stub")`
   bodies) until step 7.10: a call of one of their generic functions is a user error, not a program
   that panics. A type declaration inside a generic function (Go makes it a type of each instance, and
   `%T` shows `main.box[string]`) is not supported yet. Generic types are step 7.3.

## Alternatives

- **C++ templates with hidden descriptor arguments** (ADR-017, the README's plan): rejected for the
  reasons in the Context. A template can still be the right shape where the type argument is a C++
  type, and that is what point 3 does.
- **An instance in the generic function's package** (its own header and unit): a package would have to
  name its importers' types, and its header would change when an importer did (ADR-025 avoids that).
- **One copy of an instance for the whole program** (an `inline` function with a content-derived name
  in every unit that needs it): a silent ODR violation if two units' copies ever differ by
  something the name does not hash; units do not share more than the type information anyway.
- **Function templates for every instance** (`template <typename T> ...` with `T` deduced from the
  arguments): the template is a template over what C++ can see, which loses `int` versus `int64`.

## Consequences

Good:
- A generic function is as exact as the same function written out: the same arithmetic, conversions,
  panics, method dispatch and `%T`; no `gp_call_<Method>`, `gp::convert`, hidden descriptors, or
  `static_assert`s of constraints (nothing can be instantiated with a type that does not satisfy one,
  `go/types` checked it); steps 7.4 and 7.5 of the plan have nothing left to build for functions.
- Readable C++: the instance is the function with its types written (`gp::slice<int64_t> ...`), and
  the debugger shows it at the `.gp` lines.

Bad:
- A generic function cannot be called from C++ with an arbitrary type (a `cpp-lib` user): only the
  instances the Go code names exist. (Instantiating on demand from C++ would need a template, which
  point 3 builds for C++ types only.)
- Compile time is proportional to the number of instances in a unit, not shared across units
  (`docs/spec/cpp-mapping.md`, "Generics", has the measurement).
- The lowering of a generic function body is repeated for each instance (a few milliseconds each).
- Local types in generic functions need per-instance type declarations: not built.
- A generic function of another package that uses C/C++ names is instantiated only in its own
  package until Phase 12 (the asking package's C/C++ state would be the wrong one).
- A type parameter named `C` is an error (the loader's rule: `C` is reserved in every file). Go
  generic code often writes `[A, B, C any]`; kept as it is, documented in cpp-mapping.md.

## Evidence

- `tests/run/generics_*.gp` (more than 60 programs, outputs from real Go), `tests/pkgs/generics_*`,
  `tests/interop/generics_cxx_*`, `tests/errors/generics_*`, `internal/lower` unit tests for the
  substitution; everything with g++, clang++, and under ASan/UBSan.
- Phase 07 README §2.4: "go/types records no Instances entry when a type argument is opaque
  (verified)".
