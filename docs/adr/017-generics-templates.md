# ADR-017: Generics are C++ templates with explicit type arguments

Status: accepted (2026-10-13)

## Context

Go generics and C++ templates are both compile-time generic code, and C++ makes one copy per
type. But C++'s rules differ from Go's: template argument deduction can't see through
`gp::func<U(T)>` when it gets a plain function, untyped constants deduce differently, small
integers are promoted, and method lookup works differently. Meanwhile `go/types` has already
checked the constraints and inferred every type argument.

## Decision

- Type parameters become C++ templates in the package header (`pkg.hpp`).
- **Every use passes explicit type arguments** from `go/types` (`types.Info.Instances` for
  functions and types, `types.Info.Selections` for methods of generic types). C++ deduction is
  never used.
- Template bodies follow Go's rules: arithmetic is cast back to `T`, `/ % << >>` use the checked
  helpers, `T(x)` becomes `gp::convert<T>(x)`, and a method call on a type-parameter value goes
  through a generated `gp_call_<Method>` helper that checks "is `T` a pointer" first.
- Every template gets one hidden descriptor argument per type parameter
  ([ADR-016](016-interfaces-itabs.md)), so `any(x)`, type switches and `%T` see the Go type.
- C/C++ type arguments have no `Instances` entry: written ones are spelled from the syntax,
  inferred ones use `decltype`, and otherwise the user writes them out (until Phase 12).
- Range-over-func loops copy the design of Go's own rewrite (`rangefunc/rewrite.go`).

```go
func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func main() {
	names := Map([]int{1, 2}, strconv.Itoa) // T=int, U=string, inferred by go/types
	fmt.Println(names)
}
```

The call in `main` becomes (the hidden descriptor arguments are not shown):

```cpp
gp::slice<gp::string> names =
    Map<int64_t, gp::string>(gp::slice<int64_t>{1, 2}, &::gpkg_strconv::Itoa);
```

## Alternatives

- **C++ template argument deduction** (rejected): its rules differ from Go's (see Context).
- **Our own monomorphizer** (rejected): the C++ compiler already makes one copy per type.

## Consequences

Good: Go's exact typing; method dispatch is static and can be inlined; Go's generic packages
(`slices`, `maps`, `cmp`, `iter`) can be ported.

Bad:
- Templates live in headers, which costs compile time (`extern template` for hot instances).
  Generic bodies carry `#line`, so editing above one rebuilds its dependents.
- A template error in pure gocpp code is an internal compiler error, and it can be huge. With
  C/C++ type arguments the C++ compiler checks the constraints, so its errors are user errors.

## Evidence

- Spike 2026-10-13: `requires`-based method dispatch ran the same on GCC 15 and Clang 21
  ([cpp-idioms](../../spikes/2026-10-13/cpp-idioms/)). Verified: `FakeImportC` skips
  instantiation when a type argument is opaque.
- Checked with Go 1.25.7: inside type arguments `%T` uses full import paths and no space after
  commas (`main.Pair[string,[]int]`, `main.Stack[net/url.URL]`).

See: [Phase 07 §1](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#1-idea), [§2.1](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#21-example), [§2.2](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#22-operations-on-type-parameters), [§2.3](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#23-method-calls-on-type-parameter-values), [§2.4](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#24-generic-types-descriptors-aliases-interop), [§2.5](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#25-range-over-func-iterators-go-123), [§6](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#6-risks).
