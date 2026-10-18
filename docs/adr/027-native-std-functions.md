# ADR-027: Standard packages call native functions through `//gocpp:native`

Status: accepted (2026-10-18)

## Context

The standard library is written in `.gp` where it can be (ADR-018). Some functions can not be: Go's
`errors.As` needs `internal/reflectlite` to ask whether a dynamic type is assignable to the type a
pointer points to, and Phase 09 has `os`, `time`, `sync` and others that need the system. Before step
5.7 a function without a body in a standard package meant one thing: the package is a Phase 01
**declaration stub** (it type-checks, nothing can run), and `cmd/gocpp` did not lower any package that
had one. Phase 09 needs a marker that says "this body is C++ code", different from "this is a stub".

## Decision

A function of a standard package that is written in C++ has no body and a `//gocpp:native` line in its
doc comment, naming the C++ function that its calls become:

```go
// assign reports whether ...
//
//gocpp:native gp::errors::assign
func assign(err error, target any) bool
```

- It is the one `//gocpp:` directive that goes after the `package` clause, because it belongs to a function.
  `internal/load/native.go` checks it (a plain function without a body, one qualified C++ name, nothing
  else); the loader rejects it outside the standard library.
- A call `assign(e, t)` is written `::gp::errors::assign(e, t)` in every package that calls the function,
  with the arguments converted to the C++ parameter types as for any call. Nothing is declared or defined
  for it by the generated code: the C++ function is declared by a header that `gocpp/rt.hpp` includes
  (`gocpp/errors.hpp`) and defined in `runtime/src/`.
- A package with bodyless functions that all have the directive is a real package: `cmd/gocpp` lowers it
  with the project's packages (`notLowered`). One bodyless function without it makes the whole package a
  stub, as before; a call of a stub's function is still "not supported yet".
- Phase 09 extends the directive: with no argument, `//gocpp:native` will mean that the body is in a `.cpp`
  file next to the `.gp` file (the declaration is then written into `pkg.hpp`). Today the argument is
  required.

Natives that need a hook per type use the type descriptors (`gp::type_desc`): `type_desc::gp_store` assigns
the value an interface holds to an object of the descriptor's C++ type, which is how `errors.As` stores into
`*target` (ADR-016).

## Alternatives

- **A name convention** (a function named `native_x`, or `gp_` + name): invisible to tools, no place for the C++
  name, and a typo makes a stub instead of a native function.
- **Port `reflectlite`** for `errors`: Phase 09's `reflect` subset on the descriptors will be a package of
  its own; `errors` needs three questions of the descriptors, not a Value/Type API.
- **An intrinsic in the lowerer** (as `weak` and `fmt` have): code in Go for each package, hard to extend,
  and it hides the package from `go/types` users.

## Consequences

- `tools/stubgen` lists `errors` as `Real`: it makes the stub in memory (the tests compare the real package's
  exported declarations with it and with Go's), and writes no file.
- The directive is std-only; user code calls C++ through header imports (Phase 04), not through this.
