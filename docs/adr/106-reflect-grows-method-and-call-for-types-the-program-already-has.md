# ADR-106: `reflect` grows `Method` and `Call` for types the program already has, and stops there

Status: proposed (2026-10-20). Post-1.0 direction. Would amend [ADR-035](035-reflect-subset-on-descriptors.md) item 9 and [D6](../spec/language.md#d6-a-reflect-subset-and-no-plugin); it supersedes nothing.

## Context

`reflect` is a subset written in `.gp` over the type descriptors ([ADR-035](035-reflect-subset-on-descriptors.md)). Missing by design: `MakeFunc`, `Value.Call`/`CallSlice`, `Value.Method`/`MethodByName` (methods are listed by `NumMethod` and `Method(i).Name`, but `Method(i).Type` is nil), `StructOf`, `FuncOf`, `MapOf`, `ArrayOf`, `ChanOf`, `Select`, `Send`/`Recv`/`Close`, `Seq`. A missing function is a compile error, never a silent difference. `PointerTo` and `SliceOf` do work, by making and interning a descriptor at run time; `MapOf`, `ChanOf` and `ArrayOf` cannot, because the C++ template they need cannot be instantiated at run time.

What the subset already carries: `fmt`, `sort`, `flag`, `context`, the `DeepEqual` of 104 `tests/run/reflect_*` programs compared with real Go, and `encoding/json`, which needed nothing added (it uses interface assertions for `Marshaler`, step 9.11, ADR-102).

Who needs the rest, checked in the Go 1.25.14 sources (non-test files outside `reflect`, `internal` and `cmd`): **`text/template`** (`MethodByName`, `Method`, `Call` in `exec.go` and `funcs.go`), and so `html/template`, which is built on it; **`testing/fuzz.go`** (`testing.F.Fuzz` calls the fuzz function with `Call`; fuzzing in `testing` is a backlog item in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md)); **`testing/quick`**; **`net/rpc`**. `encoding/gob` and `encoding/xml` do not use `Call`, `Method`, `MakeFunc`, `MapOf`, `StructOf` or `Select` at all, so their gate is not here. 18 tests of Go's own `test/` folder are classified `skip:reflect-beyond-subset` ([go-test.md](../conformance/go-test.md)). The ported `math/rand` test `TestRegress` calls every method through `Value.Method` and `Call` and is left out for that reason (ADR-102).

Two facts bound the design. gocpp lowers all packages of a program together ([ADR-025](025-split-layout.md)), so the compiler knows whether any package of a build can reach `Method`, `MethodByName` or `Call`. And the method table of a descriptor already holds a thunk for every method (for itabs, [ADR-016](016-interfaces-itabs.md)); whether that thunk can be called from a uniform argument array is not checked here, and the signature is not in the table today (`Method(i).Type` is nil).

## Options

1. **Stay a subset** (D6 as written). `text/template`, `html/template`, `testing.F` and `net/rpc` are then never ported; templates are a module that wraps a C++ template engine, or code generation by `#Derive`.
2. **Stage 1: call what exists.** `Value.Method`, `MethodByName`, `Type.Method(i).Type`/`Func`, `Value.Call`, `CallSlice`, `Send`/`Recv`/`TrySend`/`TryRecv`/`Close`, `Seq`/`Seq2`, `Select` over channel types that have descriptors. Mechanism sketch: per-method and per-func-type erased call thunks (`void(void* receiver, void** args, void* results)`), grouped behind one pointer in the descriptor like `slice_refl` and `map_refl` (ADR-035 item 3), and emitted only when the build can reach the reflective entry points, so a program that never calls them pays nothing.
3. **Stage 2: `MakeFunc` for signatures that have a descriptor**, `MapOf`/`ChanOf`/`ArrayOf` that return an existing descriptor or panic with a clear message. Needs a `make_func<Sig>` instantiated for each func type in the program.
4. **Never: `StructOf`, `FuncOf`, `plugin`**, `MakeFunc` for a signature the program never mentions. They need code generation at run time.
5. **Port Go's real `reflect`** (`abi.Type` layouts). Rejected in ADR-035 and still rejected: gocpp's objects are C++ objects, not Go's layouts.

## Decision (proposed)

Do option 2 when the standard library plan ([ADR-107](107-the-standard-library-beyond-tier-2-goes-by-blockers-not-by-popularity.md)) reaches `text/template`, and not before: the reason to do it is a package that needs it, and the check that it is worth it is a measurement (below). Option 3 only for a concrete user (`MakeFunc` is rare in application code). Option 4 stays out, and D6 keeps saying so. Foreign (C++) values stay as in ADR-035 item 8: methods of a C++ class are not reflectively callable and panic with the existing "opaque" message.

## Trigger

- The decision to port `text/template`, `html/template` or `testing.F` (fuzzing in `testing`), or a user request for `net/rpc`.
- A measurement of the thunk cost on a program with about 100 types with methods that calls `MethodByName` once: binary size and C++ compile time, against the same program without the call. If the cost is large, restrict thunks to types that are converted to an interface and to exported methods (Go's linker does the same), or give up option 2.

## Consequences

- Good: unlocks four standard packages and the `testing.F` API; closes most of the 18 `skip:reflect-beyond-subset` tests; keeps the rule that anything not supported is a compile error.
- Bad: a program that uses reflective calls pays code size for thunks; calls through `reflect` go through erased argument arrays and are slow (as in Go); every new field of `type_desc` or `type_ops` must again be mirrored in `reflect.hpp` and `runtime/test/reflect_test.cpp` (ADR-035 consequences).
- Uncertain: that the existing method-table thunks can be reused; the size cost; that interop types behave (they are opaque).

## Evidence

[ADR-035](035-reflect-subset-on-descriptors.md) (items 3, 8, 9, 10), the `Call`/`Method` usage in `text/template`, `testing/fuzz.go`, `testing/quick`, `net/rpc` of go1.25.14, [go-test.md](../conformance/go-test.md) (`skip:reflect-beyond-subset` 18), the 7.3 low "`reflect` `Method(i).Type` is nil ... and `MethodByName` is missing" ([TODO.md](../../plans/2026-10-13-gocpp/TODO.md)).

## See

[Phase 09 §2.2](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md), [feature-matrix.md](../spec/feature-matrix.md).
