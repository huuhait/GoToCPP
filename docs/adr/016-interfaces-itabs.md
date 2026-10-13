# ADR-016: Interfaces use Go-style type descriptors and itabs

Status: accepted (2026-10-13)

## Context

Go interfaces are satisfied implicitly ("if it has the methods, it fits"), and values can be
checked at run time (`x.(T)`, type switches, `%T`). C++ virtual functions need explicit
inheritance. Also, several Go types share one C++ type: `int` and `int64` are both `int64_t`,
and `chan T`, `<-chan T` and `chan<- T` are all `gp::chan<T>`. Go still tells them apart.

## Decision

gocpp copies Go's own runtime design:
- Every type that flows into an interface gets a **type descriptor**: its Go name, kind, size,
  `equal`, `hash`, `format` and a method table sorted by name.
- An interface value is `{data, type, itab}` (32 bytes). Pointer values share their owner;
  other values are copied into a new box, like Go.
- An **itab** is built by matching each interface method by name and identical signature, then
  cached (a global cache, plus a static one at each conversion site), like Go's `getitab`.
- **Descriptors are chosen by the Go type, never the C++ type**: `gp_td_int` and `gp_td_int64`
  are different objects. Composite descriptors are interned, so pointer equality means type
  identity.
- Method sets follow Go (`T` has the value methods, `*T` has all of them), and unexported method
  names carry their package path.
- **Embedding without C++ inheritance**: promoted fields use the full path (`u->Base.ID`), a
  pointer to an embedded field shares ownership with the outer object, and the outer struct
  gets small forwarding member functions for promoted methods.

```go
x := any(int(1))
_, ok := x.(int64)
fmt.Println(ok) // false: int and int64 are different Go types
```

## Alternatives

- **C++ virtual functions** (rejected): they need an explicit "implements" (inheritance), and
  inheritance would make embedding a subtype. Go embedding is not subtyping.

## Consequences

Good: Go's exact behavior (implicit interfaces, type switches, `%T`, a typed nil inside an
interface is not nil, Go's panic messages). Only types that flow into interfaces pay for
descriptors. The same descriptors carry generics' type information
([ADR-017](017-generics-templates.md)) and the `reflect` subset ([ADR-018](018-stdlib-ported-from-go.md)).

Bad:
- An itab lookup costs time (cached; measured in Phase 15).
- `go/types` gives every C type the type `invalid`, and two invalid types compare equal, so it
  accepts `Get() C.int` as an implementation of `Get() C.double`. A `static_assert` on the thunk
  type and a check of the source spelling catch this until typed interop (Phase 12).
- An embedded field needs a fully qualified type in C++ (`::gpkg_main::Base Base;`).

## Evidence

- Audit: in Go, `any(1).(int64)` fails. With descriptors per C++ type it would succeed, and
  `%T` would print the wrong name.
- Audit: GCC rejects `Base Base;` as a field, hence the fully qualified field types.

See: [Phase 05 §1](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#1-idea), [§2.2](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#22-type-descriptors-chosen-by-the-go-type), [§2.3](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#23-interface-values-and-itabs), [§2.6](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#26-struct-embedding), [root README §6](../../plans/2026-10-13-gocpp/README.md#6-type-mapping-cheat-sheet).
