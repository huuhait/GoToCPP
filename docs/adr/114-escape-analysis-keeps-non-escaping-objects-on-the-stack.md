# ADR-114: Escape analysis keeps objects whose address does not escape on the C++ stack, and borrows slice elements and local closures

Status: accepted (2026-10-20). Builds on [ADR-004](004-memory-arc.md) (pointers are `gp::ptr<T>`, counted),
[ADR-086](086-borrowed-pointer-parameters-and-receivers.md) (borrow summaries); step 15.2 of Phase 15 (escape
analysis), and the part of step 15.3 that ADR-086 left (the copies out of a slice element).

## Context

A variable whose address is taken (`p := &x`, `x.Inc()` with a pointer method, `f(&x)`) was **boxed**: a
`gp::ptr<T>` from `gp::new_`, one heap allocation and a reference count for every use. `&T{...}` and `new(T)` were
always `gp::new_`. A func literal was always a `gp::func` that owns a lambda, with a box for each variable that it
changes. Go's compiler does none of this when the address cannot outlive the function (escape analysis,
`go build -gcflags=-m`). ADR-086 already computes, per function, which parameters are borrowed (the callee keeps no
copy) and which functions are pure (they cannot drop an owner), which is the interprocedural half of an escape
analysis. `v := vals[i]` and `for _, s := range shapes` still copied an interface value or a pointer out of the
slice: a count up and down for each element.

## Decision

### 1. Stack objects (`internal/lower/escape.go`)

An **object** is a variable whose address is taken, declared by a statement of a block (not a parameter, result or
loop-header variable), or an allocation `&T{...}` / `new(T)` whose pointer is held by a local pointer variable or
passed straight to a call. Every use of the address is checked with the walker of ADR-086 (`useOK`, `methodOK`,
`argOK`), so the rules are the ones for a borrowed parameter: the object may be read and written through, compared,
used as the receiver or argument of a callee that borrows that parameter. A pointer variable that holds the address
(`p := &x`) must itself be never assigned, copied (`q := p`), stored, returned, sent, captured, converted to an
interface, passed to an owning parameter, to unknown code (C/C++, `unsafe.Pointer`, an interface method that class
hierarchy analysis does not resolve, a func value) or used by `go` and `defer`. Anything else is **escaping**.

Lowering of what does not escape:

| Go | C++ |
|---|---|
| `var c T; c.Inc()` (pointer method) | `T c{}; T::Inc(gp::borrow(&c))`: the variable is on the C++ stack |
| `p := &c` | `gp::ptr<T> p = gp::stack_ptr(c)`: a `gp::ptr` with no owner (aliasing constructor of an empty `shared_ptr`) |
| `p := &T{...}`, `p := new(T)` | `gp::local_ptr<T> p = gp::local_new<T>(T{...})`: the object is a member of the pointer variable, whose copy and move constructors are deleted |
| `f(&T{...})` | `f(gp::borrow_tmp(T{...}))`: a temporary that lives to the end of the full expression |

The variable stays in the map of boxed variables with the new reason `boxStack`, because "memory that a call can
change" is what the order of evaluation and the lending of arguments need to know (`isBoxed` true, `inBox` false,
`onStack` true). An object is decided as a whole: if one use escapes, all of its variables are boxed as before.
Generic functions and methods of generic types are not analysed (and have no local closures either: each
instance has variable objects of its own, so a generic declaration is boxed as before, `boxInfoOf`).

An allocation binding whose initializer uses the new variable's name keeps an owning pointer. For example,
`p := &T{n: p.n + 1}` in an inner block reads the outer `p` in Go, but C++ puts the new `p` in scope in its own
initializer. Lowering may stage the allocation in a temporary and move it into the final variable. A
`local_ptr` cannot move without invalidating its address, so escape analysis conservatively declines this
binding and `-gp:m=2` explains the heap fallback. Multiple-value bindings are already not stack holders.

A C++ stack does not grow and a function can call itself, so an object is kept only when it is small: at most
2 KiB (`maxStackObjectSize`), and at most 16 KiB of charged storage for one function (`maxStackObjectsSum`).
The budget uses the C++ layouts from `check.Sizes`, charging an additional pointer and alignment padding for
every object (including plain locals and temporaries, conservatively). Formerly boxed local-closure captures
reserve from the same budget first, in source order; addresses and allocations use what remains. Oversized
captures keep their boxes too, whether they change or are read-only. This limits added object storage, not the
whole compiler-generated frame or call chain: temporaries, spill slots and sanitizer redzones can cost more.
A variable that was already on the stack has no additional limit beyond the 128 KiB of the boxing. Arbitrarily
deep recursion can still overflow the fixed C++ stack, unlike Go's growing stacks.

### 2. Class hierarchy analysis (`internal/lower/chapure.go`)

ADR-086 treats a call through an interface as unknown code. For the region walker (not for the function summaries,
which are unchanged) such a call is **pure** when every method of every named type of the program and of the
packages it imports (also natives) with the same name and signature is pure by the summaries of ADR-086. A generic
type with a method of that name, a method of a native package without summary, and an interface method that
nothing implements are impure.

### 3. Element references (`internal/lower/elemref.go`)

`v := xs[i]` (a slice, an index and path without calls, the region is the rest of the block) and
`for _, v := range xs` (the region is the body) declare `v` as `const T&` when the element type owns something
(`refType`: interface, pointer, string, slice, map, func), `v` is never assigned, address-taken, boxed, captured or
returned, the region is pure (strictly pure, `pureV`, for strings) by the region walker with class hierarchy
analysis, it stores into no array whose elements a reference can point to (a slice may alias a local array,
which pure code writes with private stores; that includes arrays of strings, which are not owners for the
purity of ADR-086, and arrays of arrays and structs of them). The root variable of the path is not assigned
in the region (the index form), and, in a program that can run a second thread, nothing stores into elements
of that type anywhere (`hotLoad`, `hotElems`, the thread
analysis of ADR-086 section 4). The thread analysis is coarse: nearly every program that imports `fmt` or calls an
interface method counts as threaded, so the references appear in practice for element types that are only written
by literals. That is a gap, see Consequences.

### 4. Local closures (`internal/lower/localclosure.go`)

A func literal that is only called where it is made, `func() {...}()` or `f := func() {...}` with every use of `f`
a call that runs at once (not `go`, not `defer`, not inside another func literal or a range-over-func body), and
that does not call `recover`, is **local**. It is emitted as a raw lambda that captures by reference
(`auto f = [&n]<int = 1>(...) mutable {...}`), with no `gp::func` and no allocation. A variable that only
local closures capture is not boxed when a statement of a block declares it or the header of a `for` or `range`
loop does (a loop variable is one variable per iteration in Go, but a local closure runs within its iteration
and keeps nothing, so one C++ variable shows it the same values); a parameter or a result keeps its box.
The size and shared storage budget above can also keep a captured variable boxed. An unboxed changed
variable is `boxRefCaptured`: not a box, but memory that a call can change for the order of evaluation, and
copied in place for fmt arguments as gc does (inplace.go). Anything else that captures a variable boxes it as
before and the local lambda captures the box. This depends on the use of the literal alone, not on what its body
does.

### 5. Diagnostics and switches

`gocpp build -gp:m` prints the decisions like `go build -gcflags=-m`: `&x does not escape`, `moved to heap: x`,
`&T{...} escapes to heap`, `leaking param: p`, `p does not escape`, `v borrows xs[i]`, `func literal does not
escape` or `escapes to heap`. `-gp:m=2` adds the reason of each escape and the `v copies xs[i]` notes. Packages of
the standard library are not reported. `-gp:noescape` turns the analysis off (stack objects, element references and
local closures), which is the baseline for the differential tests and the benchmarks.

## Alternatives

- **Pointer analysis (points-to sets) for every pointer**: finds more objects, but needs a whole-program fixpoint
  and the shape of the heap; the walker works on the uses of one variable and the existing summaries.
- **Reference-count elision only** (no stack objects): keeps the allocation, which is the bigger cost.
- **Stack objects for parameters, results, loop variables and receivers**: each needs a copy per call or iteration
  that the boxing already keeps; not done here.
- **Refining `threaded` with class hierarchy analysis** (so that programs with `fmt` borrow elements): a follow-up.

## Consequences

Good: an address-taken local and a `&T{}` that stay in the function cost no allocation and no count; a local
closure costs nothing; fewer counts for interface and pointer elements read from a slice.

Bad: the lowering of a variable now depends on the uses of its address in the whole function; a non-escaping
object is not a `shared_ptr` target, so code that wants a `gp::ptr` to it gets a pointer that owns nothing (the
analysis proves that it never outlives the object); the element references are rare in threaded programs; things
not done: loop variables, parameters, receivers and named results are not kept on the stack; a use of `&x` inside
a closure counts as escaping; native types (`sync.Mutex`, `bytes.Buffer`) still escape; generic functions are not
analysed; `go` and `defer` uses escape; a struct or array parameter is still copied; a `defer` closure is not made
local.

## Evidence

`TestEscapeAnalysis`, `TestElemRefs` (threaded and single-thread subtests, one negative case per rule),
`TestLocalClosures` (`internal/lower`), `cmd/gocpp` flag tests, the run tests `tests/run/escape_*.gp` (g++, clang++,
`-asan`, and output compared with real Go), the golden `tests/golden/escape.gp`, the unit test `test_ptr_stack` of
the runtime, and mutations of every rule (each must fail a test). The benchmark `bench/programs/dispatch/escape.gp` (O2, 5 runs, medians) went from 8.2x (g++) and 9.0x (clang++)
slower than Go to 1.19x and 1.23x; by kernel (g++): stack objects 1.4 s to 0.056 s (Go 0.05 s), local closures 3.5 s to
0.2 s (Go 0.21 s), interface elements 3.0 s to 1.4 s (Go 1.4 s). More in
[Phase 15 §3.4](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md).

See: [Phase 15 §2.1](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first),
[cpp-mapping.md, Pointers and boxing](../spec/cpp-mapping.md#pointers-and-boxing).
