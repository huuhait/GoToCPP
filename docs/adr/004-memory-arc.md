# ADR-004: Memory is automatic reference counting (ARC)

Status: accepted (2026-10-13)

## Context

Go has a garbage collector; C++ has none. Go code uses `&T{}`, returns pointers and keeps slices
of pointers without thinking about lifetimes. gocpp wants that comfort too, with native speed,
deterministic cleanup and easy use of C++ libraries. The user decided: reference counting and
no garbage collector ([Q1](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)).

## Decision

- Every heap block (boxed variables, `new(T)`, slice arrays, string bytes, map tables, closure
  cells) is owned through `std::shared_ptr`: one ownership system for everything in v1.
- `*T` for a gocpp type `T` is `gp::ptr<T>`: a `std::shared_ptr<T>` plus nil checks. A nil
  dereference panics with Go's message.
- A pointer *into* a block shares that block's owner (the aliasing constructor): `&p.f` is
  `gp::ptr<F>(p, &p->f)`, and `&s[i]` keeps the slice's array alive. `&globalVar` shares a static
  owner that is never freed.
- The compiler moves a local to the heap ("boxing") when its address is taken, a pointer method is
  called on it, an array in it is sliced, or a closure captures it and it changes anywhere.
- A pointer to a C/C++ type (`*cstdio.FILE`, `*SDL.SDL_Window`, `*C.char`) is a raw `T*` with
  C++ rules ([ADR-005](005-interop-header-packages.md)).
- Reference cycles are broken with Go 1.24's `weak` package, built on `std::weak_ptr`.

```go
func NewCounter() *Counter {
	c := Counter{step: 1} // address taken below → c is boxed
	return &c
}
```

```cpp
gp::ptr<Counter> NewCounter() {
    gp::ptr<Counter> c = gp::new_<Counter>(Counter{.step = 1});  // lives on the heap
    return c;                                                     // &c is the box itself
}
```

## Alternatives

- **Boehm GC** (rejected): not deterministic, and trouble with C++ interop.
- **Manual memory** (rejected): unsafe and not Go-like.

## Consequences

Good: no GC pauses and deterministic cleanup; no use-after-free in gocpp code, except data races,
`unsafe` and C/C++ values; gocpp pointers inside C++ containers stay counted.

Bad:
- Cycles leak unless broken with `weak.Pointer[T]` (D1); self-recursive closures leak too.
  `gocpp vet` warns about obvious cycles (Phase 13).
- Atomic refcount traffic, and bigger values than Go: `*T` 16 B, `string` 32 B, `[]T` 40 B (D15).
  Phase 15 measures escape analysis, safe `const&` parameters and an intrusive refcount.
- A data race on a pointer, slice or string can corrupt refcounts (D10; use `-race`). A gocpp
  pointer passed to C is only borrowed, and a pointer from C owns nothing (D16).

## Evidence

- Spike 2026-10-13: `shared_ptr` aliasing for interior pointers and `make_shared<T[]>` ran with
  identical results on GCC 15 and Clang 21 ([cpp-idioms](../../spikes/2026-10-13/cpp-idioms/)).
- Audit: value receivers passed as `const&` gave wrong values under aliasing and an ASan
  use-after-free, so value receivers and parameters are real copies.

See: [Phase 03 §2.1](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#21-one-ownership-system-for-everything-v1), [§2.2](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#22-pointers-and-boxing), [§2.5](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#25-methods), [Phase 04 §2.7](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#27-pointers-unsafe-and-slices-at-the-boundary), [Phase 15 §2.1](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first), divergences [D1, D10, D15, D16](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
