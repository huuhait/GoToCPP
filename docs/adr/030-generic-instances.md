# ADR-030: A generic function is lowered once for each instance, by the package that uses it; a generic type is one shared item for each instance

Status: accepted (2026-10-18; generic types added in step 7.3). Replaces the template design of [ADR-017](017-generics-templates.md) for
generic functions and generic types (ADR-017's "our own monomorphizer (rejected)" and its C++ templates, hidden descriptor
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
   that panics. (A type declaration inside a generic function is a type of each instance, as Go
   makes it: point 5.)
5. **A generic type is one C++ struct, method set and descriptor for each instance** (step 7.3,
   `internal/lower/insttypes.go`). `Pair[string, []int]` is a type of its own that `go/types` makes
   (`types.Named` with type arguments), and the lowering treats it as it treats an unnamed struct
   (ADR-025): a **shared item** that every package that mentions it writes the same text for. What is
   different from an anonymous struct is that the text has code in it, so it is made by the **package
   that declares the generic type** (`makeInstance`, in `finishShared`), with the substitution layer of
   point 1: the struct (`gp_inst_<8, 10 or 12 hex digits of a hash of the identity text>_<Name>`, in the
   namespace of the declaring package, where the type's unexported members and helpers are visible),
   its method bodies (value and pointer receivers, as for any struct; `inline`), the forwarders of
   promoted methods, the method tables and thunks, and the descriptor, whose name is Go's `%T` text
   (`main.Pair[string,[]int]`: type arguments with their import paths, no space after a comma;
   `canon.go: inArgs`). The methods of an instance are lowered with it (a method that returns another
   instance asks for that one, and so on to a fixed point: a recursive generic type is one instance).
   The hash is of the *identity* text, not of the asker, so two packages, and the declaring package, name
   the same C++ type: `Box[b.Feet]` of `a` is the same type in `a`, `b`, `c` and `main` (the header of
   each one has the text under the macro guards `GP_INST_<hash>`, `_IMPL` for the descriptors and thunks
   and `_POST` for the bodies, which go after the header's tail where the functions they call are
   declared). An importer does not change the declaring package's header: the owner's tables are saved
   and restored while the instance is made. A hash that clashes with another instance takes more digits
   (clash lengthening, tested), and `runtime/src/trace.cpp` shows `main.Stack[...].Peek` and
   `main.(*Stack[...]).Pop` for them, as Go does. Generic aliases (Go 1.24) are `go/types` aliases of
   instances and cost nothing.

   **Limits.** Instances need the split layout (a package's `pkg.hpp`), which `gocpp build` and `emit`
   use; the unit layout of the lowering's tests gives a `notYet` error. A **C/C++ type as a type argument
   of a generic type** (`Box[std.string]`) is a `notYet` user error with a message that says so: the
   field types, the equality, the descriptor and the method tables of an instance are made from Go
   types, and a placeholder cannot be baked into a text that every package shares (the placeholder's
   spelling would have to be part of the identity, and only the declaring package could ask for the
   instance: designed, not built). A generic *function* with a C++ type argument that makes an instance
   of a generic type of that argument is the same error, at the instantiation. 

   **Types declared in generic functions** (`type box struct{ v T }` in `F[T any]`, also in a method of a
   generic type) are types of each instance in Go: `%T` shows `main.box[string]`, also for a local type
   that does not mention `T`, and the `box` of `F[int]` is not the `box` of `F[string]` nor of another
   function. `internal/lower/localtypes.go` gives each a synthetic generic origin (type parameters: fresh
   copies of the function's, the underlying type substituted by the same layer) and the substituter
   replaces every mention in an instance's body by the instance of that origin with the instance's type
   arguments; from there it is an instance of a generic type of point 5 (no methods; a struct or a
   named non-struct type). The identity text carries a per-origin tag, so two local types of the same
   name with the same arguments are two instances.

   **The standard packages' generic types**: `atomic.Pointer[T]` is written in Go over the spin lock
   of `atomic.Value` (a native cannot be generic); `iter.Seq` and `iter.Seq2` are the ported package's.
   A generic *function* over `S ~[]E` whose `S` C++ deduces (a slice of a C++ type) gets `E` by default
   from `S` (`gp::detail::slice_elem_of`) and the slice operations.

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
- A type declared in a generic function is a type of each instance in Go (`main.box[string]`, also when it does not mention a type parameter): it is a generic type with the function's type parameters, made by the lowering (`internal/lower/localtypes.go`) and shared like any instance. It must be a struct or a named non-struct type that gocpp can write (a defined pointer type is not).
- A generic function of another package that uses C/C++ names is instantiated only in its own
  package until Phase 12 (the asking package's C/C++ state would be the wrong one).
- A type parameter named `C` was an error (the loader's rule: `C` is reserved in every file); step 7.3
  exempts the type parameters of functions, types and receivers (`[A, B, C any]`, `[C ~chan E, E any]`):
  they shadow the pseudo-package in their scope, and `C.int` in such a body is Go's own "C.int is not a
  type". A variable, a parameter or a type named `C` is still the error.
- An instance's text is in every translation unit that includes the header of a package that mentions
  it (the structs, thunks, descriptors and inline methods), which costs compile time: 20 generic types with
  10 instances each, 3 methods each, with `fmt.Sprint` of each instance: 17k lines of C++, g++ 69 s and clang++ 56 s
  against 38 s and 35 s for the same 200 types written out (on a machine with a load average above 300:
  the ratio, about 1.8, is what to read).

## Evidence

- `tests/run/generics_*.gp` (more than 100 programs, outputs from real Go; `generics_types_*` for
  step 7.3), `tests/pkgs/generics_types_*` (declared in one package, instantiated in another, three-package
  chains, embedding, constraints, interfaces, init order), `tests/interop/generics_cxx_*` and
  `generics_types_cxx_*`, `tests/errors/generics_*`, `tests/golden/generics_types`, `internal/lower` unit
  tests for the substitution and the instance names (`insttypes_test.go`); everything with g++, clang++,
  and under ASan/UBSan.
- Phase 07 README §2.4: "go/types records no Instances entry when a type argument is opaque
  (verified)".
