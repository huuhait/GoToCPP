# The gocpp language

gocpp follows The Go Programming Language Specification (go1.25) except for the points below.

The Go spec is at <https://go.dev/ref/spec>. gocpp pins the go1.25 language version: the type
checker runs with `GoVersion: "go1.25"` ([Phase 01 §2.7](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#27-gotypes-configuration)).
Everything this document doesn't mention works as the Go spec says.

| | |
|---|---|
| Status | Written in Phase 00 from the plan; each phase updates it when it builds a feature. The features of Phases 00 to 05 are built and their gates are lifted (`gocpp version` prints `phase 05`): the core language, C/C++ interop, interfaces, embedding, `error`, `panic` and `recover`, `fmt` on any value, and programs of several packages. **Generics (Phase 07) are built and open from Phase 05 on** ([Generics and range over functions](#generics-and-range-over-functions)): generic functions, types and aliases, constraints, `comparable`, range over functions, and Go's own `cmp`, `slices`, `maps` and `iter`. **`!` (must), must functions, macros (`macro`, `lazy`, `#Name(...)`, the built-ins) and `#Derive` (Phase 10, divergences D12, D13, D17's `errors.Wrapf` and D18) are always available**: they have no gate and need no `GOCPP_GATE_PHASE` ([ADR-095](../adr/095-must-and-macros-are-always-available.md)). Goroutines and channels exist but stay gated until Phase 08 is finished (`select`, `sync` and `time` are missing) |
| Source | [Phase 00 §2.2](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go) (the divergence list D1–D21; D22 from the Phase 02 evaluation-order work, D23 from Phase 03 steps 3.5 and 3.7, D24 from step 3.6, D25 from step 3.15, D26 from Phase 04 step 4.12, D27 from step 4.11, D28 from Phase 05 step 5.10) and the phase READMEs linked in each section |
| See also | [cpp-mapping.md](cpp-mapping.md) (what the C++ output looks like), [interop.md](interop.md) (using C and C++), [feature-matrix.md](feature-matrix.md) (the phase of each feature) |

**How to read it.** Each section starts with an example. Then it says what Go does, what gocpp
does, and which phase of the plan builds it. Source files end in `.gp`. Until a phase is done, the
compiler rejects its features with a clear error: "not supported yet (Phase NN)"
([Phase 01 §2.9](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#29-unsupported-feature-checker-phase-gates)).
The [feature matrix](feature-matrix.md) lists every feature with the phase that brings it.

## Contents

| # | Go | gocpp | Phase |
|---|---|---|---|
| [D1](#d1-reference-counting-instead-of-a-garbage-collector) | garbage collector | reference counting, `weak.Pointer` for cycles | 03 |
| [D2](#d2-header-imports-instead-of-cgo) | `import "C"` and a cgo preamble | header imports: a header is a package, C++ namespaces are packages | 01, 04 |
| [D3](#d3-go-keywords-as-names-after-a-dot) | no keyword after `.` | `std.map`, `node.type()` | 01 |
| [D4](#d4-constants-as-type-arguments) | type arguments are types | `std.array[int, 5]` | 01, 04 |
| [D5](#d5-constructor-calls-for-cc-types) | `T(x)` has one argument | `T(a, b)` calls a C++ constructor | 04 |
| [D6](#d6-a-reflect-subset-and-no-plugin) | full `reflect`, `plugin` | a `reflect` subset, no `plugin` | 05, 09 |
| [D7](#d7-goroutines-are-os-threads) | millions of cheap goroutines | one OS thread per goroutine | 08 |
| [D8](#d8-map-order-is-random-as-in-go) | random map order | random, as in Go (a Swiss table with a seed for each map) | 03, 15 |
| [D9](#d9-64-bit-targets-only) | `int` is 32 or 64 bits | 64-bit targets only | 01, 02 |
| [D10](#d10-data-races-can-break-reference-counts) | data races are undefined | they can also corrupt reference counts | 03, 06, 08 |
| [D11](#d11-gocpp-directives-instead-of-cgo-lines) | `#cgo` lines | `//gocpp:` directives and `gocpp.mod` | 01, 04, 06, 14 |
| [D12](#d12-must-for-errors) | `if err != nil` | also `!` (must) | 10 |
| [D13](#d13-macros) | no macros | `macro` declarations and `#Name(...)` | 10 |
| [D14](#d14-cc-values-follow-c-rules) | Go rules for every value | C++ rules for C/C++ values | 04 |
| [D15](#d15-different-sizes) | `string` is 16 bytes, … | `string` is 32 bytes, … | 03 |
| [D16](#d16-unsafepointer-and-pointers-from-c) | `unsafe.Pointer` keeps its target alive | the same, but C pointers own nothing | 04 |
| [D17](#d17-a-few-additions-to-the-standard-packages) | the standard packages | a few additions | 04, 10, 14 |
| [D18](#d18-macro-and-lazy-are-keywords-in-two-places) | `macro`, `lazy` are names | keywords in two places | 10 |
| [D19](#d19-c-is-a-reserved-name) | `C` is an ordinary name | `C` is reserved for C's built-in types and helpers | 01 |
| [D20](#d20-type-names-with-more-than-one-dot) | a type name has at most one dot | `std.chrono.seconds`, `std.vector[int].iterator` | 01, 04 |
| [D21](#d21-range-over-c-containers) | `range` over Go types only | also over C++ containers and C arrays | 04 |
| [D22](#d22-which-of-two-run-time-panics-is-reported) | one panic message, fixed by the compiler | the same one, except in three rare corners (assignments and declarations with several targets, calls that gc doesn't inline) | 02, 03 |
| [D23](#d23-print-and-println-of-a-slice-or-a-map) | `println(s)` prints `[3/3]0xc000012345`, `println(m)` `0xc000012345` | an error: use `fmt.Println` | 03 |
| [D24](#d24-large-values-on-the-c-stack) | the stack grows for any value | a result or a temporary array or struct over 128 KiB can overflow the C++ stack | 03 |
| [D25](#d25-fmt-arguments-that-a-closure-changes) | whether `fmt.Println(ok, set())` prints the old `ok` depends on gc's inliner | a rule on the code approximates it; a closure gc doesn't inline can print differently | 03 |
| [D26](#d26-c-exceptions) | a C++ exception that leaves C++ code under cgo aborts the program | a panic where it reaches a function with defers (the deferred calls run, `recover()` stops it), else a Go-style report | 04, 05 |
| [D27](#d27-a-panic-that-leaves-a-callback) | a panic in a callback unwinds through the C frames to the Go code above, and `recover` there can stop it | the program ends at the callback, like an unrecovered panic: the deferred calls outside the C call don't run | 04 |
| [D28](#d28-recover-through-a-function-that-gc-inlines) | whether `recover` works through a function that gc inlines into the deferred call depends on the inliner | always the rule: only the deferred function itself recovers (what `go build -gcflags=-l` does) | 05 |

The three things gocpp adds have their own sections:
[header imports](#header-imports), [must functions and must calls](#must-functions-and-must-calls)
and [macros](#macros). Points the plan leaves open are listed at the end
([open points](#open-points)).

---

## D1. Reference counting instead of a garbage collector

```go
type Node struct {
	Next *Node
	Prev weak.Pointer[Node] // weak: does not keep the previous node alive
}

func link(a, b *Node) {
	a.Next = b
	b.Prev = weak.Make(a) // a strong pointer back to a would make a cycle
}
```

- **Go:** a garbage collector frees memory that nothing can reach any more, cycles included.
- **gocpp:** there is no garbage collector. Every heap object is reference counted: what `&T{}`
  and `new` make, slice arrays, string bytes, maps, and variables that closures share. An object
  is freed when the last pointer to it goes away, so cleanup is deterministic. A reference cycle
  (A points to B, and B points to A) is never freed. Break it with Go 1.24's `weak` package:
  `weak.Make(p)` and `wp.Value()`. `gocpp vet` warns about obvious cycles, such as a closure that
  refers to itself (Phase 13).
- `wp.Value()` is nil as soon as the last strong pointer to the target goes away, at a known
  point of the program. In Go it is nil only after a garbage collection has found the target
  unreachable, which may be much later (and, for small objects that share their memory with
  others, maybe never). So a program that prints whether a weak pointer is still alive prints
  `false` in gocpp where Go may print `true` (tests/run/weak_expire.gp shows the gocpp side).
  While the program still uses a strong pointer to the target, both agree; Go may already return
  nil when the only strong pointer is a variable that is still in scope but no longer used (its
  docs point to `runtime.KeepAlive`). The other way round, a target inside a leaked cycle stays
  alive forever in gocpp, so its weak pointers never become nil, while Go frees the cycle and
  returns nil after a garbage collection.
- A weak pointer doesn't keep its target alive, but it keeps its memory: `new` and `&T{}` put
  the value and its counts in one allocation, so after the last strong pointer goes, the value
  is destroyed and its memory (`unsafe.Sizeof` of the value) stays until the last weak pointer
  to it goes too. That matters for a cache of weak pointers to big values. Phase 15 may
  allocate large values apart from their counts.
- gocpp code has no use-after-free, except with data races ([D10](#d10-data-races-can-break-reference-counts)),
  `unsafe`, and C/C++ values ([D14](#d14-cc-values-follow-c-rules)).
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#21-one-ownership-system-for-everything-v1).
  How pointers look in C++: [cpp-mapping.md](cpp-mapping.md#pointers-and-boxing).

## D2. Header imports instead of cgo

```go
package main

import (
	"fmt"

	"<raylib.h>" // a C header: package raylib
	"<vector>"   // a C++ header: its namespace std can be used
)

func main() {
	raylib.InitWindow(800, 450, "demo")
	nums := std.vector[C.int]{1, 2, 3}
	fmt.Println(nums.size())
}
```

- **Go:** C is reached through `import "C"` and a cgo preamble (C code in a comment above the
  import). Every C name is written `C.name`.
- **gocpp:** you import headers. A header is a package named after its file
  (`raylib.InitWindow`). C++ namespaces declared by the imported headers are packages that need no
  import (`std.vector`). `C` holds only C's built-in types and the boundary helpers (`C.int`,
  `C.CString`, `C.cstr`), and every `.gp` file can use it, so the name `C` is reserved
  ([D19](#d19-c-is-a-reserved-name)). There is no inline preamble: a small helper goes into a
  header next to the `.gp` file.
- **Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification)
  (imports and type checking), [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md)
  (C++ output and building).
- **More:** [Header imports](#header-imports) below, and all the details in [interop.md](interop.md).

## D3. Go keywords as names after a dot

```go
ages := std.map[std.string, int]{}          // std::map
kind := node.type()                         // a C++ method named type
n := posix.select(fd+1, &rd, nil, nil, nil) // the C function select()
```

- **Go:** a keyword is never a name, so `std.map` and `node.type()` are syntax errors.
- **gocpp:** a Go keyword right after `.` **on the same line** is a name. So C++ names that are
  Go keywords work: `std::map`, a method `type()`, the C function `select()`. The type switch
  `x.(type)` still works, because there the token after `.` is `(`. A keyword on the line after a
  `.` is still an error, so an unfinished `fmt.` in the editor doesn't swallow the next line.
- In a chain of calls, a keyword name can't start the next line. When `b.` ends a line and
  `select()` starts the next one, that is a syntax error: "expected selector or type assertion,
  found 'select'". Keep `select` on the line of its `.` (the real line in the file, not the line
  that a `/*line …*/` directive gives):

  ```go
  w := a.type().select().
  	Next()
  ```
- In the example, `posix` is `<sys/select.h>` imported with a name (`import posix "<sys/select.h>"`).
  It gets no name by default, because `select` is a Go keyword ([header imports](#header-imports)).
- **Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#22-the-fork-three-patches-to-goscanner--goparser)
  (parser patch 1).

## D4. Constants as type arguments

```go
var buf std.array[byte, 256] // std::array<uint8_t, 256>
bits := std.bitset[64]{}     // std::bitset<64>
```

- **Go:** type arguments must be types.
- **gocpp:** a type-argument list may also hold constant expressions, because C++ templates take
  values. For gocpp's own generic functions and types this is still an error (`5 is not a type`).
- **Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#22-the-fork-three-patches-to-goscanner--goparser)
  (parser patch 2), [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#24-cc-types-in-gocpp-code)
  (C++ output).

## D5. Constructor calls for C/C++ types

```go
s := std.string("hi")       // std::string("hi")
v := std.vector[int](10, 0) // std::vector<int64_t>(10, 0): ten zeros
```

- **Go:** a conversion `T(x)` takes exactly one argument.
- **gocpp:** for a C/C++ type, `T(a, b, ...)` calls its C++ constructor, with any number of
  arguments.
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#25-values-and-operations-opaque).

## D6. A reflect subset and no plugin

```go
type Point struct{ X, Y int }

func main() {
	t := reflect.TypeOf(Point{})
	fmt.Println(t.Kind(), t.NumField()) // struct 2
}
```

- **Go:** the full `reflect` package, the `plugin` package, and access to runtime internals
  (for example with `//go:linkname`).
- **gocpp:** `reflect` is a **subset**, built on the type descriptors that the compiler generates.
  It is enough for `fmt`, `encoding/json`, `sort`, `errors` and `context`. The API list is in
  [Phase 09 §2.2](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#22-a-reflect-subset-on-top-of-type-descriptors).
  Not in the subset: `reflect.MakeFunc`, `reflect.StructOf`, `reflect.FuncOf`, `reflect.MapOf`,
  `ArrayOf` and `ChanOf`, `Value.Call`, `Value.Method` and `MethodByName` on values (calling methods
  by name), channel `Send`/`Recv`, and anything else that needs new code at run time. The type of
  a method (`Type.Method(i).Type`) is nil. There is no `plugin` package, and
  user code can't use `//go:linkname`.
- **Phase:** [05](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#22-type-descriptors-chosen-by-the-go-type)
  (type descriptors), [09](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#22-a-reflect-subset-on-top-of-type-descriptors)
  (the subset).

## D7. Goroutines are OS threads

```go
var wg sync.WaitGroup
for i := range 1000 {
	wg.Add(1)
	go func() { // starts a new OS thread
		defer wg.Done()
		work(i)
	}()
}
wg.Wait()
```

- **Go:** goroutines are cheap. A program can run millions of them.
- **gocpp:** every goroutine is a real OS thread. There is no goroutine scheduler, now or later.
  Tens of thousands of goroutines that exist *at the same time* are fine, and a hundred thousand
  work on a machine whose limits allow it (below); millions are not. Goroutines that come and go
  cost much less: the thread of a finished goroutine waits for the next `go` statement, so a loop
  that starts a million short goroutines uses a few hundred threads and about 6 µs for each.
- Each thread gets an 8 MiB stack by default (memory is only used when a page is touched). A
  stack overflow, on a goroutine's stack or the main one, prints `fatal error: stack overflow`
  and exits with code 2. An unrecovered panic in any goroutine prints `goroutine N [running]:`
  and exits with code 2, as in Go.
- What a blocked goroutine costs, measured on Linux 6.19 (`vm.max_map_count` 1M, `kernel.threads-max`
  2M, the machine shared and loaded): about 14 KB of resident memory and 8 MiB of address space, and
  10 to 130 µs to start. 1,000 goroutines blocked on a channel start in 0.1 s, 10,000 in 0.8 s (65
  MB), 100,000 in 13 s (1.3 GB). The limits of the system apply: `ulimit -u`, `kernel.threads-max`,
  a cgroup's `pids.max`, `ulimit -v` (8 MiB of address space each) and, on a kernel without
  guard-page support in `madvise` (before Linux 6.13) or with the default `vm.max_map_count` of
  65,530, about 20,000 to 30,000 threads. When the system refuses a thread, the program ends with
  Go's `fatal error: newosproc`.
- Handing a value between two goroutines is slower than in Go: a ping-pong on unbuffered channels
  takes about 1 µs for each operation when both threads are running on idle cores (the waiting
  goroutine spins for about 4 µs first) and 7 to 15 µs when the machine is loaded and the receiver
  must be woken by the kernel, against 0.2 µs in Go. A buffered channel between a producer and a
  consumer costs 0.1 to 0.25 µs for a value (a buffer of 128 or more), 0.9 µs with a buffer of 1.
- The Go names stay: `go`, "goroutine", `runtime.NumGoroutine`. `runtime.LockOSThread` always
  holds, because every goroutine already has its own thread. `runtime.GOMAXPROCS` remembers the
  value you set, but gocpp doesn't use it for scheduling.
- **Phase:** [08](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#22-goroutines--os-threads).

## D8. Map order is random, as in Go

```go
m := map[string]int{"a": 1, "b": 2, "c": 3}
for k, v := range m {
	fmt.Println(k, v) // some order, different in each loop: don't depend on it
}
fmt.Println(m) // map[a:1 b:2 c:3]: fmt sorts the keys, as in Go
```

- **Go:** the spec leaves the iteration order unspecified, and the Go runtime randomizes it on
  purpose (a random seed for each map, and a random start for each loop).
- **gocpp:** the same since step 15.5 ([ADR-059](../adr/059-swiss-table-maps.md)): every map has
  its own random hash seed (so the layout of its table differs from map to map and from run to
  run, and an input chosen to collide can't know it), and every `range` starts at a random slot
  of the table. Programs must not depend on any order. (Before 15.5 the order was unspecified but
  fixed by the keys' hashes; the phase 03 text of D8 said "not randomized".) NaN keys are spread
  with a count kept by each map.
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#23-strings-slices-arrays-maps),
  [15](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md).

## D9. 64-bit targets only

```go
var n int = 1 << 40 // always fits: int is 64 bits
var c C.int = 7     // C's int, for C APIs
```

- **Go:** `int`, `uint` and `uintptr` have 32 or 64 bits, depending on the platform.
- **gocpp:** v1 has only 64-bit targets, so `int` and `uint` always have 64 bits (`int64_t`,
  `uint64_t` in C++). Where a C API wants C's own sizes, use the `C` types (`C.int`, `C.long`, …).
  The targets are Linux and Windows on x86-64 first (linux/arm64 is best effort until Phase 15).
- **Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#27-gotypes-configuration)
  (sizes for the type checker), [02](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#23-expressions-the-important-rules)
  (C++ types).

## D10. Data races can break reference counts

```go
s := []int{1, 2, 3}
go func() { s = append(s, 4) }() // writes s
fmt.Println(len(s))              // reads s at the same time: a data race
```

- **Go:** data races violate the memory model; an implementation may report the race and terminate.
  If it continues, single-word racy reads still have constraints (they must observe a value actually written),
  whereas races on multiword values can corrupt memory. This is not C++'s blanket undefined-behavior rule.
- **gocpp:** generated C++ has no data-race safety guarantee. A race on a pointer, slice or string can
  corrupt its reference count, so memory can be freed too early or never. Find races with `-race`
  (ThreadSanitizer). `-race` works on Linux only.
- **Hot-swapped pointers.** A pointer that one goroutine replaces while another reads it
  (`cfg = newConfig()` or `srv.cfg = ...` against `use(cfg)`, no lock) is a race that mostly works in Go,
  because the collector keeps the old object. In gocpp a callee that only reads its pointer parameter gets
  the caller's pointer without a count (ADR-086), so the replacement could free the object under it. In a
  program that can run a second thread (a `go` statement, `time.AfterFunc`, `sync.WaitGroup.Go`,
  `context.AfterFunc` or `runtime.SetFinalizer`, anywhere in the program or in the standard code it reaches;
  unknown func/interface calls and native callbacks also count; a library build always) the compiler therefore
  keeps its own reference to a pointer loaded through an assigned package variable, field or slice/map/array
  element for the whole call. Implicit promoted fields count too; only proven thread-free initialization is
  exempt. A program proved to have no second thread has no such copies. **The copy is not atomic and does
  not make a racing load/store safe.** Not covered: writes through `reflect` or `unsafe`, aliases held by C or
  C++, foreign threads, and threads that a linked library starts without a visible callback. Library builds
  conservatively copy visible field/element loads and exported-variable loads, but do not analyse foreign
  aliases. Use `atomic.Pointer[T]`, a lock or a channel to hand a pointer to another goroutine, as
  Go's memory model asks; see [ADR-086](../adr/086-borrowed-pointer-parameters-and-receivers.md#4-soundness).
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#21-one-ownership-system-for-everything-v1)
  (reference counts), [06](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#29-cli-after-this-phase)
  (the `-race` flag), [08](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md)
  (ThreadSanitizer over the concurrency tests).

## D11. gocpp directives instead of cgo lines

```go
//gocpp:pkg-config sdl2
//gocpp:link m pthread
//gocpp:cflags -DUSE_FAST_MATH
//gocpp:ldflags -L/opt/foo/lib
package game
```

- **Go:** cgo reads `#cgo CFLAGS: ...`, `#cgo LDFLAGS: ...` and `#cgo pkg-config: ...` lines
  in the preamble.
- **gocpp:** `//gocpp:` line comments before the `package` clause, like `//go:build`:
  `pkg-config`, `link`, `cflags`, `ldflags`, `extern-c` / `no-extern-c` for C headers, and
  `no-win32-defaults` for Windows headers (Phase 14). An unknown directive is an error with a
  "did you mean" hint. Settings for the whole module go into `gocpp.mod`, which uses `go.mod`
  syntax. What each directive does: [interop.md](interop.md#building-c-and-c-sources).
- **Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#24-gocpp-directives-parsed-now-used-in-phases-04-and-06)
  (parsed and checked), [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#29-building-the-cc-side)
  (used for the build), [06](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#21-the-module-file-gocppmod)
  (`gocpp.mod`), [14](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#24-interop-with-windows-headers)
  (`no-win32-defaults`).

## D12. Must for errors

```go
func loadConfig!(path string) Config {
	var cfg Config
	data := os.ReadFile(path)  // error → returned for you
	json.Unmarshal(data, &cfg) // error → returned for you
	return cfg
}

func port(path string) (int, error) {
	cfg := loadConfig!(path) // this call's error → returned for you
	return cfg.Port, nil
}
```

- **Go:** you check errors by hand: `if err != nil { return err }`.
- **gocpp:** that still works, and `!` (must) can do it for you, but only where you write it. A
  must function `func f!() T` returns every error that is dropped inside it; callers see
  `func f() (T, error)`. A must call `F!(...)` returns the error of that one call. In a function
  without an error result, a failing must call panics (in a test it calls `t.Fatal`). In `main`,
  the error is printed and the program exits with code 1.
- **Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#21-part-a--must). Built, and always available: no phase gate and no `GOCPP_GATE_PHASE` ([ADR-095](../adr/095-must-and-macros-are-always-available.md)).
- **More:** [Must functions and must calls](#must-functions-and-must-calls) below.

## D13. Macros

```go
package main

import "fmt"

macro Dbg[T any](v T) T {
	fmt.Printf("[line %d] %s = %v\n", #line(), #text(v), v)
	return v
}

func main() {
	port := 8080
	fmt.Println("port:", #Dbg(port)) // prints "[line 12] port = 8080", then "port: 8080"
}
```

- **Go:** there are no macros. Code generators are separate tools (`go generate`, `stringer`).
- **gocpp:** a macro is declared with `macro Name(args) { ... }`. It looks like a func and is
  type-checked like one. You use it with `#Name(...)`: inside code, or on top of a func (start
  style, or wrap style with `#body()`). `#Derive(...)` on top of a type generates methods. The
  built-ins are `#file()`, `#line()`, `#func()`, `#text()`, `#body()`, `#embed()` and
  `#Derive(...)`.
- **Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros). Built, and always available: no phase gate and no `GOCPP_GATE_PHASE` ([ADR-095](../adr/095-must-and-macros-are-always-available.md)).
- **More:** [Macros](#macros) below.

## D14. C/C++ values follow C++ rules

```go
ages := std.map[std.string, int]{}
fmt.Println(ages["bob"]) // 0, and "bob" is now in the map: C++ operator[] inserts
fmt.Println(ages.size()) // 1

v := std.vector[int]{1, 2, 3}
fmt.Println(v[5])    // no bounds check: undefined behavior, as in C++
fmt.Println(v.at(5)) // at() checks: it throws a C++ exception, which becomes a panic
```

- **Go:** every value follows Go's rules. Reading a missing map key never inserts it, every index
  is checked, and memory stays alive while something points to it.
- **gocpp:** values of C/C++ types follow C++ rules, also when you use Go syntax on them. `m["k"]`
  on a `std::map` inserts the key. C++ containers get no gocpp bounds checks (use `.at(i)` if you
  want one). Raw C pointers have C++ lifetimes, so they can dangle. gocpp's safety covers gocpp
  types, not C/C++ types.
- Go syntax works on C/C++ values: operators, indexing, member calls, constructor calls
  ([D5](#d5-constructor-calls-for-cc-types)) and `range` loops ([D21](#d21-range-over-c-containers)).
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#25-values-and-operations-opaque).
  More in [interop.md](interop.md#values-and-operations).

## D15. Different sizes

```go
var s string
var xs []int
var p *int
fmt.Println(unsafe.Sizeof(s), unsafe.Sizeof(xs), unsafe.Sizeof(p))
// Go prints 16 24 8. gocpp v1 prints 32 40 16.

fmt.Println(unsafe.Sizeof(struct{}{}), unsafe.Sizeof(struct {
	a struct{}
	b int64
}{}))
// Go prints 0 8. gocpp prints 1 16.
```

| Type | Go (64-bit) | gocpp v1 | Why |
|---|---|---|---|
| `string` | 16 | 32 | owner, data pointer, length |
| `[]T` | 24 | 40 | owner, data pointer, length, capacity |
| `*T`, `unsafe.Pointer` | 8 | 16 | a `std::shared_ptr` (pointer + owner) |
| `weak.Pointer[T]` | 8 | 24 | a `std::weak_ptr` and the address it was made from |
| `map[K]V` | 8 | 16 | a `std::shared_ptr` to the map table |
| `chan T` | 8 | 16 | a `std::shared_ptr` to the channel ([Phase 08](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md)) |
| `func(...)` | 8 | 24 | the owner of the closure and its code pointer ([Phase 03 §2.6](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#26-functions-func-values-closures)) |
| interface values | 16 | 32 | data, type descriptor, method table ([Phase 05 §2.3](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#23-interface-values-and-itabs)) |
| `struct{}`, `[0]T` | 0 | 1 | in C++, every object takes at least 1 byte |

- **Go:** `unsafe.Sizeof` gives the sizes in the "Go" column.
- **gocpp:** reference-counted values carry their owner, so they are bigger. The type checker
  gets the real gocpp sizes, so `unsafe.Sizeof`, `unsafe.Alignof` and `unsafe.Offsetof` give
  what the C++ gives, and C++ `static_assert`s keep both sides in sync. The numbers, `bool`
  and `complex` types have Go's sizes (`int` is 8 bytes).
- **Structs** are laid out like C++ structs, which is also Go's rule: the fields in source
  order, each one at the next multiple of its alignment, and the size rounded up to the
  largest alignment of a field. The difference is the empty struct: it takes 1 byte, also as
  a field, so `struct{ a struct{}; b int64 }` is 16 bytes (Go: 8) and `[3]struct{}` is 3.
  `[0]T` is 1 byte too, with alignment 1. An array `[N]T` is `N` times the size of `T`. Each
  generated struct has a `static_assert` that its C++ size and alignment are these.
  A slice of empty structs therefore uses memory: `make([]struct{}, n)` allocates `n` bytes, and a
  huge `n` (Go allows `math.MaxInt`, because its zero-size elements take no memory) fails with an
  out-of-memory error in gocpp. (`append` is not affected: it gives a zero-size element no
  memory and `cap == len`, as Go does.)
- **The capacity after `append`** is Go's, although the sizes above are not: the runtime
  computes Go's size and pointer-ness of the element type (`gocpp/golayout.hpp`), so a
  `[]string` grows `1 2 4 8 16 32 71 143 ...` as in Go (`tests/run/append_cap_*.gp`).
  `[]byte(s)` and `[]rune(s)` give what a slice that escapes gets. Where gocpp can still differ
  from gc: a slice that does not escape in gc may start in a 32-byte stack buffer or share the
  string's bytes (gc's escape analysis, which gocpp does not copy), and an element type from C
  or C++ has no Go layout (its C++ size is used).
- The size of `chan T` is that of `gp::chan<T>` (a `std::shared_ptr`: 16 bytes), asserted in
  `sizes.hpp` since Phase 08; func values and interface values are what Phase 03 step 3.11 and
  Phase 05 built.
- Only the C++ compiler knows the size of a C/C++ type (`C.int`, `*stdio.FILE`,
  `std.vector[int]`), so `unsafe.Sizeof`, `Alignof` and `Offsetof` of a value or a type that holds
  one directly or in a field or an array element (also nested) are not Go constants: they are the
  C++ compiler's `sizeof`, `alignof` and `offsetof`, as values of type `uintptr`
  ([Phase 04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#27-pointers-unsafe-and-slices-at-the-boundary)
  step 4.10). In a constant context (`const n = unsafe.Sizeof(x)`, an array length) the error is
  Go's "is not constant", with that reason. A Go pointer, slice or map of such a type is fine: its
  size doesn't depend on what it points to, and stays a constant. `Offsetof(x.f)` is a constant
  when the C/C++ fields come after `f`.
- Inside a C++ template argument (`std.array[byte, unsafe.Sizeof(s)]`) and next to other
  C/C++ expressions, `unsafe.Sizeof`, `Alignof` and `Offsetof` are an error when Go's size
  differs from gocpp's: the type checker can only use Go's sizes there. Declare a constant
  (`const n = unsafe.Sizeof(s)`) and use it: `std.array[byte, n]`.
- Phase 15 may make `string` and `[]T` smaller (24 and 32 bytes) if a cheaper reference count
  wins the benchmarks.
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#21-one-ownership-system-for-everything-v1),
  maybe smaller in [15](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first).

## D16. unsafe.Pointer and pointers from C

```go
// gocpp memory: unsafe keeps the target alive, as in Go.
// This is Go's own strings.Builder.String, which gocpp ports unchanged.
func (b *Builder) String() string {
	return unsafe.String(unsafe.SliceData(b.buf), len(b.buf)) // shares b.buf's array
}

// C code never keeps gocpp memory alive: it only borrows the pointer.
func sum(data []byte) C.uint {
	return lib.sum_bytes(&data[0], C.size_t(len(data))) // fine: C reads data during the call
}
```

- **Go:** an `unsafe.Pointer` keeps its target alive, because the garbage collector sees it.
- **gocpp:** the same for gocpp objects. `unsafe.Pointer` is an owning pointer, and
  `unsafe.String`, `unsafe.Slice` and `unsafe.SliceData` keep the owner too. But:
  - an ordinary raw pointer from C, or `unsafe.Pointer(uintptr)` at an unregistered address,
    does not recover the original owner. There is a specific exception: conversion at the exact
    address of a registered, still-live gocpp-owned C/C++ box recovers and shares that box's owner
    (`unsafe_from_addr` / `raw_owned` in [the runtime](../../runtime/include/gocpp/unsafe.hpp) and
    [the box registry](../../runtime/include/gocpp/ptr.hpp)). A `uintptr` itself never owns memory;
    keep an owning reference alive across the raw-address interval. Arbitrary or interior addresses
    have no such owner-recovery guarantee;
  - `(*T)(p)` of a misaligned `unsafe.Pointer` ends the program ("fatal error: checkptr:
    misaligned pointer conversion", as Go's `-race` or `-d=checkptr` modes report it), where Go
    reads it without a word on amd64: in C++ the read would be undefined behavior. The natural
    false failure is a packed C struct (`#pragma pack(1)`): `(*C.int)(unsafe.Pointer(&r.x))` of
    its unaligned field, and `unsafe.Pointer(uintptr(buf) + 1)` read as a `uint32`;
  - `unsafe.Pointer(uintptr(unsafe.Pointer(&x)) + off)` in one expression is Go's valid pattern and
    points into the object that `x` keeps alive. Unless the exact-address box recovery above applies,
    storing the reconstructed pointer does not retain `x`'s owner, unlike Go's garbage collector:
    keep an owning reference to `x` for as long as the reconstructed pointer is used;
  - a `*T` of a C/C++ type that gocpp code declares (a variable, parameter, result, field or
    element) is a `gp::cptr<T>`: it owns or aliases what it points to, as Go's pointer does, and
    converts to the raw `T*` that C takes (so `&T{...}`, `&s.f` and `&xs[i]` can be returned, stored
    and appended, and stored into a member of a C struct that a `*T` variable owns: the struct keeps the object alive); the members of C structs and what C functions return stay raw
    ([interop.md](interop.md#pointers-unsafe-and-slices-at-the-boundary));
  - C code never keeps gocpp memory alive. A gocpp pointer passed to C is only **borrowed**, as
    in cgo's pointer-passing rules. C must not use it after the gocpp object is gone. For a
    `void*` that C keeps (user data for callbacks), keep the owner yourself (Phase 09: a `cgo.Handle` from `runtime/cgo`, not built yet)
    ([interop.md](interop.md#user-data)).
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#27-pointers-unsafe-and-slices-at-the-boundary).

## D17. A few additions to the standard packages

```go
func loadConfig!(path string) (cfg Config, err error) {
	defer errors.Wrapf(&err, "load config %s", path) // a gocpp addition
	data := os.ReadFile(path)
	json.Unmarshal(data, &cfg)
	return cfg, nil
}

// error text: "load config app.json: open app.json: no such file or directory"
```

- **Go:** the standard packages are exactly Go's.
- **gocpp** adds a few things:

  | Addition | What it does | Phase |
  |---|---|---|
  | `errors.Wrapf(&err, format, args...)` | used with `defer`: puts context in front of the error that the function returns. It wraps like `fmt.Errorf` with `%w`, so `errors.Is(err, fs.ErrNotExist)` and `errors.As` still find the original error | 10 |
  | `errors.Trace(err)` | shows the path an error took (an **optional** step, see [open points](#open-points)) | 10 |
  | `C.cstr(s)` | a NUL-terminated `const char*` for `s`, valid until the end of the statement | 04 |
  | `C.wstr(s)`, `C.WString(s)`, `C.GoStringW(p)`, `C.GoStringWN(p, n)` | wide strings (UTF-16 on Windows) for the `W` functions | 14 |
  | hidden helper packages `gocpp/must`, `gocpp/cgoh` (the path `gocpp/macro` is reserved) | the compiler adds and uses them for its rewrites; you don't import them | 04, 10 |

- Not every Go standard package exists. The tiers are in
  [Phase 09 §2.6](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#26-package-tiers)
  (for example, `net/http` and `plugin` are not planned).
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#26-strings-at-the-boundary),
  [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#21-part-a--must),
  [14](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#24-interop-with-windows-headers).

## D18. macro and lazy are keywords in two places

```go
package util

macro Assert(cond bool, msg lazy string) { // macro: a declaration; lazy: a modifier
	if !cond {
		panic(msg())
	}
}

func count() int {
	macro, lazy := 1, 2 // inside a function: ordinary names
	return macro + lazy
}
```

- **Go:** `macro` and `lazy` are ordinary identifiers.
- **gocpp:** at package level, `macro Name(` starts a macro declaration. Inside the parameter list
  of a macro, `lazy` is a modifier (`msg lazy string`). Everywhere else both stay ordinary
  identifiers, so old code that uses them as names still works. A macro parameter can't be named
  `lazy`: `macro M(lazy string)` is a clear error.
- **Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros)
  (parser patch 6). Built, and always available: no phase gate and no `GOCPP_GATE_PHASE` ([ADR-095](../adr/095-must-and-macros-are-always-available.md)).

## D19. C is a reserved name

```go
var size C.size_t = 16 // fine: C holds C's built-in types

func area() int {
	C := 3 // error: "cannot declare C - C is reserved for C's built-in types"
	return C * C
}
```

- **Go:** `C` is an ordinary identifier.
- **gocpp:** `C` is reserved for C's built-in types and the boundary helpers (`C.int`,
  `C.CString`), and every `.gp` file can use it without an import. A local or package-level
  declaration named `C` is an error. An import can't be named `C` either
  (`import C "<x.h>"`), and `package C` is an error too. A dot import of a package that exports `C`
  (`import . "unicode"`: the category `unicode.C`) is an error as well, so Go's own `unicode` tests
  reach the package by its name.
- **Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification).
  What `C` holds: [interop.md](interop.md#what-c-holds).

## D20. Type names with more than one dot

```go
type Timer struct {
	Every std.chrono.seconds       // std::chrono::seconds
	Pos   std.vector[int].iterator // std::vector<int64_t>::iterator
}

func base(p std.filesystem.path) std.filesystem.path {
	return p.filename()
}
```

- **Go:** a type name has at most one dot (`pkg.T`).
- **gocpp:** the type name of a C++ type may have more dots: a type in a nested namespace
  (`std.chrono.seconds`, `std.filesystem.path`) or a member type (`std.vector[int].iterator`).
  This works in every type position: variables, parameters, results, struct fields and element
  types (`[]std.chrono.seconds`). In expressions, Go already allows more dots. In a type, only
  names can follow the type arguments: `std.vector[int].iterator` works, but a second list of
  type arguments, as in `std.allocator_traits[A].rebind_alloc[T]`, is a syntax error.
- **Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#22-the-fork-three-patches-to-goscanner--goparser)
  (parser patch 3), [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#24-cc-types-in-gocpp-code)
  (C++ output).

## D21. range over C++ containers

```go
nums := std.vector[int]{5, 2, 9}
for i, n := range nums { // i is the index, n is a copy of the element
	nums.push_back(n) // a vector is walked by index, size read once: still 3 times
	fmt.Println(i, n)
}
```

- **Go:** `range` works on arrays, slices, strings, maps, channels, integers and functions.
- **gocpp:** `range` also works on C++ containers (anything with `begin()` and `end()`) and on
  C arrays. The value (and a map's key) gets C++'s element type, and the index is a C/C++ value
  too (convert it, `int(i)`). As in Go, with one loop variable you get the index (the key for
  maps), and the value is a copy. A `std::string` gives its bytes as C chars, not runes. A
  random-access container is walked by index with its size read once, as Go reads the length
  once, so a `push_back` in the body doesn't break the loop; a body that shrinks it panics with
  Go's "index out of range" when the walk reaches an index past the new end. Lists, sets and
  maps are walked with their iterators, which follow C++'s invalidation rules (erasing the
  current element is undefined behavior, as in C++).
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#25-values-and-operations-opaque).
  The C++ form is in [interop.md](interop.md#operations-on-cc-values).

## D22. Which of two run-time panics is reported

```go
s, i, z := "abc", 5, 0
fmt.Println(s[i], 7/z) // Go and gocpp: "index out of range [5] with length 3"
x, y, n := []int{1}, []int{1, 2}, 0
n, x[x[i]+y[i]] = 1, 2 // Go checks x[i] first; gocpp may check y[i] first
```

- **Go:** the spec leaves this order open; Go's compiler (gc) checks from left to right, after
  the calls, with its own rules for assignments (see
  [cpp-mapping.md](cpp-mapping.md#order-of-evaluation)).
- **gocpp:** since Phase 03 step 3.12 it reports the same panic as gc: an operation that can
  panic (an integer division or remainder by zero, a negative shift count, an index or a slice
  bound out of range, a nil pointer) goes into a temporary before the statement when C++ could
  evaluate another one first. Three corners are left, where the other message can appear:
  - two such operations inside one operand of a target of an assignment with several targets
    (`s[a[i]+b[j]], x = 1, 2`);
  - an assignment or a declaration with several targets (`var x, y = s[i], a[j]`) whose values
    gc evaluates early or late by whether a variable's address is taken, where gocpp asks
    whether the variable is boxed: the two differ only for a variable that a closure changes, as
    in [D25](#d25-fmt-arguments-that-a-closure-changes);
  - the arguments of a call of a function that gc doesn't inline, `h(s[i], m[a[j]])`: gc then
    evaluates the panics in a map key (or a string comparison) of a later argument before the
    earlier arguments, while for a function that it inlines it goes from left to right, as gocpp
    always does.

  The program still panics and exits with code 2, and every call, all output and every value
  read happen in Go's order; only the panic message can differ.
- **Phase:** [02](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#25-evaluation-order-minimal-version-here-already),
  narrowed in [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#27-evaluation-order).

## D23. print and println of a slice or a map

```go
s := []int{1, 2, 3}
println(s) // Go prints [3/3]0xc000012345; gocpp: error: cannot println s ([]int): ... use fmt.Println
m := map[string]int{"a": 1}
println(m) // Go prints 0xc000012345; gocpp: error: cannot println m (map[string]int): ... use fmt.Println
```

- **Go:** the builtins `print` and `println` write a slice as its length, its capacity and the
  address of its array in Go's heap, and a map as the address of its table (`0x0` for a nil map).
- **gocpp:** those addresses can't be the same, so `gocpp check` rejects a slice or map argument
  of `print` and `println`; `fmt.Println(s)` and `fmt.Println(m)` print the elements, as in Go.
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#23-strings-slices-arrays-maps).

## D24. Large values on the C++ stack

```go
func zeros() [1 << 21]int { return [1 << 21]int{} } // 16 MiB result

func main() {
	a := zeros() // Go: works; gocpp: the result is a temporary on the C++ stack, which can overflow
	_ = a
}
```

- **Go:** goroutine stacks grow, and Go's compiler moves large values to the heap, so an array of
  any size can be a local variable, a parameter, a result or a temporary.
- **gocpp:** C++ stacks have a fixed size (8 MiB for the main thread on Linux by default). gocpp
  keeps values larger than 128 KiB (Go's own limit for stack variables) off the stack where that
  is cheap. A local variable of such a type is boxed (it lives on the heap), and is built in its
  box when a literal or a conversion initializes it (`a := [1 << 21]int{5: 3}`,
  `a := [1 << 21]int(s)`, also in a declaration of several variables or one that shadows
  another). `&[1 << 21]int{...}` is built in its new block. A parameter is passed by reference
  and copied into a box when the function starts, and `range` copies a large array to the heap.
  The other values of such a type that are C++ temporaries stay on the stack: a result returned
  by value, a literal or a conversion that doesn't initialize a new variable (`a = [1 << 21]int{}`,
  `f([1 << 21]int{})`, and a variable that `:=` redeclares, which it assigns: `x, z :=
  [1 << 21]int{2: 1}, 4` where `x` exists), and the temporaries of the order of evaluation. At -O0
  each of them takes its own place in the function's stack frame, and a program whose temporaries
  add up to more than the stack crashes with a segmentation fault (exit code 139), where Go runs.
  Return a pointer to the array (`*[N]T`, for example `return &[1 << 21]int{...}`) or a slice
  instead, as Go code usually does for large arrays.
- **Structs** larger than 128 KiB (a struct with a large array field) follow the same rules: a
  local variable is boxed, and a parameter is passed by reference and copied into a box. A
  literal that initializes a new variable or `&Big{...}` (`b := Big{n: 3}`, `&Big{n: 3}`, also
  `var b Big`, `new(Big)`) is made in its block: the zero value first, then the given fields are
  stored, so the literal is never a temporary. A literal anywhere else (`b = Big{n: 3}`,
  `f(Big{n: 3})`) is a temporary on the stack, and so is the copy that an assignment of a struct
  makes when its value is reached through a pointer or a slice (`b = *p`, which gocpp copies
  first so that the store can't free the value while it reads it).
- **Package variables** of such types are in static storage, never on the stack. A literal that
  initializes one, also inside a conversion (`block([1 << 21]int{...})`), is stored into the
  variable element by element and field by field when the package is initialized (`var grid =
  [1 << 21]int{0: f()}`), so it is never a temporary either. A package variable initialized by
  a call's result (`var grid = makeGrid()`) gets it through a temporary on the stack, like a
  local's result.
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#23-strings-slices-arrays-maps).


## D25. fmt arguments that a closure changes

```go
ok := false
set := func() bool { ok = true; return ok }
fmt.Println(ok, set()) // Go and gocpp: false true
```

- **Go:** the spec leaves open when `ok` is read. gc copies an argument of `fmt`'s `...any` whose
  value needs an address (a `bool`, `int8`, `uint8`, an array or a struct that isn't 2, 4 or 8
  bytes without pointers) into a temporary in its place among the calls, also when it is a local
  variable (not a parameter or a result) whose address is not taken and that fits in registers
  (at most 32 bytes, an array of at most one element, a struct of at most 4 fields), and also
  for a part of such a local (so `fmt.Println(one[k], setk())` of a `var one [1]bool` indexes
  with the old `k` even when `setk` changes `k`; gocpp does the same). Whether `ok` is such a
  local depends on gc's inliner and escape analysis: when gc inlines every call of the closure,
  `ok` stays a plain local and the old value prints; when it doesn't, the closure captures `ok`
  by reference, and the new value prints. gc
  never inlines a closure whose own body has a `defer` or `go` statement or calls `recover`, and
  inlines one whose cost is at most 80 nodes, 160 for a closure (doubled), 800 for a closure that
  is called once; inside a function of more than 5,000 nodes the limits are 20 and 40, and still
  800 for a closure called once.
- **gocpp:** it has no inliner, so a rule on the code decides. The old value is printed when every
  func literal that captures the local is called at once (`func() { ... }()`), or initializes a
  local (`f := func() ...`) that is only ever called, in the same function (never assigned again,
  stored, passed, returned, deferred, used as a value such as `_ = f`, or used in another func
  literal), and none of them has a `defer` or `go` statement or a `recover` call in its own body.
  Otherwise the new value is printed. Where gc decides by the cost instead, the output differs:
  gocpp prints the old value where gc doesn't inline the closure because it is too large, which
  mostly happens for a closure called more than once (over 160 nodes) or inside a very large
  function (over 40), and it prints the new value where gc keeps a local for a closure stored in
  a local that doesn't escape. gocpp also reads a parameter of a func literal (and a part of one)
  after the calls, while gc copies it when it inlines the literal, whose parameters then become
  locals of the caller: `f := func(a [1]bool) { fmt.Println(a[gk], setk()) }; f([1]bool{})`
  prints `false 0` in Go, where gocpp indexes with the new `gk` (and here panics). The same rule
  and the same differences hold for a map key that is such a local (or a part of one, as above)
  when the map call takes the key by address. gc's map call does that for every key of a map
  whose value is over 128 bytes, and otherwise for a key that is neither a string nor 4 or 8
  bytes of plain memory (an `int32`, an `int`, a pointer, a channel, or an array or a struct of
  integers, bools, pointers and channels without padding and without blank `_` fields, such as
  `[2]int16`; an array or a struct that holds just one string counts as a string): a `bool`,
  `int8`, `int16`, `uint8` or `uint16` key, a float, a complex number, an interface, most arrays
  and structs. With `mb` a `map[bool]int`, `fmt.Println(mb[ok], set())` reads the old `ok` when
  gc inlines `set` (step 3.12). On such a slow call, a key that gc doesn't keep in registers is
  never copied: with `ka` a `[3]int32` (an array of more than one element), `mk[ka]` reads the
  new `ka` in Go and gocpp (a 4- or 8-byte array or struct key of a fast call is always copied,
  in Go and gocpp: [cpp-mapping.md](cpp-mapping.md#order-of-evaluation)). The rule is `orderKey`
  in `internal/lower/maps.go`. A program that wants one answer evaluates the call first (`r :=
  set(); fmt.Println(ok, r)`) or passes the value it means.
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#210-builtins-printing-and-package-initialization).
  The rule is in `internal/lower/inplace.go`; `tests/diff/fmt_closure_locals.gp` has the cases
  where both agree. Phase 15's escape analysis may get closer to gc's.

## D26. C++ exceptions

```go
func main() {
	defer fmt.Println("deferred")             // runs: the exception is a panic here (Phase 05)
	fmt.Println(std.stoi(std.string("x")))    // throws std::invalid_argument
}
// deferred
// panic: C++ exception std::invalid_argument: stoi
//
// goroutine 1 [running]:
// main.main()
// 	/home/me/prog/main.gp:5 +0x6b
// exit status 2
```

- **Go:** Go has no exceptions. Under cgo, a C++ exception that leaves C++ code ends the program:
  the C++ library's "terminate called after throwing an instance of 'std::invalid_argument'",
  then Go's `SIGABRT: abort` dump, exit status 2; the deferred calls don't run.
- **gocpp:** a function with a `defer` statement catches every C++ exception that reaches it
  (Phase 05, step 5.10) and turns it into a Go panic: the deferred calls of that function and of
  the frames outside it run, as for any panic, and `recover()` stops it. Its value is a
  `runtime.CppException` (`gp::CppException`): an `error` whose `Error()` is the text that an
  unrecovered one prints, `C++ exception <type>: <what()>` (`of type int` for an exception that is
  no `std::exception`). Unrecovered, the program ends with that report, the trace of the frame that
  caught it (the C++ frames the exception already unwound are gone: Go has no such frames) and exit
  code 2. A stack without any function that has a `defer` has nothing to run and nothing to
  recover with: the program ends at the throw, with the trace of the throw (the report of Phase
  04). `std::bad_alloc` is Go's fatal error `out of memory` (not recoverable) and a forced unwind
  (`pthread_exit`, `pthread_cancel`) is never caught. A C++ destructor that throws while a Go
  panic unwinds its frame ends the program with that exception's report, without the Go panic
  ([interop.md](interop.md#c-exceptions)).
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#28-callbacks-and-panics-crossing-c-code),
  [05](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#27-panic--recover--defer-full-version).

## D27. A panic that leaves a callback

```go
func visit(user unsafe.Pointer, i C.int) C.int {
	defer fmt.Println("callback defer")
	panic("stop")
}

func main() {
	defer fmt.Println("main defer") // Go: runs; gocpp: not run
	lib.walk(5, visit, nil)         // a C function that calls visit
}
// callback defer
// panic: stop
//
// goroutine 1 [running]:
// main.visit(...)
// 	/home/me/prog/main.gp:3 +0x8c
// walk(...)
// 	/home/me/prog/walk.c:12 +0x35
// main.main()
// 	/home/me/prog/main.gp:9 +0x26
// exit status 2
```

- **Go:** under cgo, a panic in a callback unwinds through the C frames (they are skipped: their
  cleanup does not run) to the Go function that called C. The deferred calls on the way run, and a
  `recover` in one of them stops the panic; unrecovered, it ends the program with the usual report.
- **gocpp:** a panic or C++ exception that leaves a callback (a gocpp function that C holds as a
  function pointer) ends the program at once, with the same report as an unrecovered panic and exit
  code 2. The callback's own deferred calls run first; the deferred calls of the functions outside the
  C call do not run, and nothing outside the callback can recover it. The reason is C: a C library compiled
  without exception tables would be unwound unsoundly (a lock stays held, memory leaks), and the
  alternatives (carry the panic across, `longjmp`) leave C running with invented results or skip C++
  destructors. A callback that must survive recovers inside itself (Phase 05). A later phase can carry
  the panic out of the trampoline and re-raise it at the Go call site of the C function, so that a
  `recover` outside the C call works as under cgo. Through C++ code
  (`std::function`, a template) the panic unwinds normally.
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#28-callbacks-and-panics-crossing-c-code)
  ([interop.md](interop.md#panics)).

---

## D28. recover through a function that gc inlines

```go
func call(f func()) { f() }

func helper() { fmt.Println(recover()) }

func main() {
	defer call(helper) // Go 1.25 (default build): recovers; with -gcflags=-l, or gocpp: prints <nil>
	panic("boom")
}
```

- **Go:** `recover` returns the panic only when the function that calls it was called directly by
  the deferred-call machinery. When gc inlines a function into the deferred wrapper, the inlined
  code's callees look like that function's callers, so `recover` there can succeed. In Go 1.25's
  default build this happens for `defer call(h)` with `func call(f func()) { f() }`, `defer
  s.Run()` where `Run` calls a func field, a method value `f := t.M; defer f()` where `M` calls a
  helper that recovers, and `defer i.M()` with a value receiver. It depends on the inliner, so it
  changes with the Go version and with `-gcflags=-l`.
- **gocpp:** always the rule of the spec and of `-gcflags=-l`: only the deferred function itself
  can recover; a function that it calls, inlined or not, gets nil (docs/spec/cpp-mapping.md,
  "Defer"). Programs that depend on the inliner's result print something else; the tests avoid
  those shapes (`tests/diff/recover_*`).
- **Phase:** [05](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#27-panic--recover--defer-full-version).

## Generics and range over functions

```go
type Number interface{ ~int | ~int8 | ~float64 }

func Sum[T Number](xs ...T) (s T) {
	for _, x := range xs {
		s += x
	}
	return
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func Count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	fmt.Println(Sum[int8](100, 27, 1)) // -128: the sum wraps at the width of the type argument
	var st Stack[string]
	st.Push("a")
	fmt.Printf("%T\n", st) // main.Stack[string]
	for x := range Count(10) {
		if x == 3 {
			break
		}
	}
}
```

Generics follow the Go spec (go1.25, with generic type aliases) and the type checker of Go
(`go/types`) checks them: the constraints, the type sets, `comparable` (every interface satisfies it,
and `==` on an uncomparable dynamic type panics), inference, and the messages of the errors. What
gocpp adds is only how the code is compiled, so the programs behave as the same code written with the
type arguments does ([ADR-030](../adr/030-generic-instances.md), [cpp-mapping.md](cpp-mapping.md#generics-steps-71-to-73)):

- Each instance (a function or a type with its list of type arguments) is **ordinary C++ written with
  those types**, not a C++ template. `int8(100) + int8(50)` wraps to `-106` inside `Sum[int8]`, `a / b`
  panics with `integer divide by zero` for integers and gives `±Inf` for floats, `any(x)`, `x.(T)`,
  `case T:` and `%T` see the type argument, and `int` and `int64` stay two types.
- `%T`, `%v`, `%+v` and `%#v` of an instance print what Go prints: `main.Pair[string,[]int]`,
  `*main.Stack[net/url.URL]`, `main.Box[main.point·1]` for a type declared in a function (the number
  counts such types in the package, as gc does), `main.loc[int]` for a type declared in a generic function.
- **Range over functions** (`for x := range f`, Go 1.23) is Go's own rewrite: the loop body becomes the
  `yield` function, `break`, `continue`, `return`, labels and `goto` work across any number of nested loops,
  a `defer` in the body belongs to the function with the loop, and an iterator that misuses `yield` gets
  Go's four panics (`range function continued iteration after function for loop body returned false`,
  `... after whole loop exit`, `... after loop body panic`, and `range function recovered a loop body panic
  and did not resume panicking`). A `yield` that is kept and called after the loop is over panics before it
  touches anything (no dangling reads of the finished frame). `iter.Pull` and `iter.Pull2` run the iterator on
  a goroutine of its own ([D7](#d7-goroutines-are-os-threads)): a switch costs about a microsecond.
- `cmp`, `slices`, `maps` and `iter` are Go's own packages ([ADR-078](../adr/078-cmp-slices-maps-iter-ports-and-pull-coroutines.md)).

**Differences.** None of them is a new divergence of the list above; they are consequences of D4, D14,
D19 and D7, and of the stage of the interop:

| What | Go | gocpp |
|---|---|---|
| A C or C++ type as a type argument of a **function** (`Max[std.string]`, `Id(v)` with a `std::vector`) | not applicable ([D14](#d14-cc-values-follow-c-rules)) | allowed: the argument is spelled in the instance, or the C++ compiler deduces it. `go/types` cannot check the constraint, so the C++ compiler does, and its errors are the user's |
| A C or C++ type as a type argument of a generic **type** (`Box[std.string]`, `iter.Seq[std.string]`) | not applicable | an error that says so; the fields, descriptor and equality of an instance are made from Go types (typed interop, Phase 12) |
| A generic function of another package whose body uses C/C++ names, instantiated by an importer | not applicable | an error at the call (`it can only be instantiated in its own package`, Phase 12) |
| A constant as a type argument ([D4](#d4-constants-as-type-arguments)) | not allowed | allowed for C++ templates only; for generic Go code it is still an error |
| A type parameter named `C` ([D19](#d19-c-is-a-reserved-name)) | fine | fine for a type parameter (it shadows the pseudo-package in its scope); an error for every other declaration |
| Stack traces | `main.f-range1` for a loop body | the body (a generated helper) is left out; a func literal in it is `main.f.func1` |
| The same instance in many files | one copy | one copy for each translation unit that asks for it (not shared between units); a change to a generic function recompiles its users |

Where gc and the Go spec differ, gocpp follows the **spec** and notes it: a method value `x.M` with a
pointer type argument copies `*x` when it is evaluated; a `defer` in a loop body inside a func literal that
gc inlines runs when the literal returns; calling a value method through a nil pointer in generic code
panics with the nil dereference text where gc says `value method main.Base.String called using nil *Base pointer`.

Source: [Phase 07](../../plans/2026-10-13-gocpp/phase-07-generics/README.md),
[ADR-030](../adr/030-generic-instances.md), [ADR-033](../adr/033-range-over-func.md),
[cpp-mapping.md](cpp-mapping.md#generics-steps-71-to-73). Compile time:
[the guide](../guides/generics-compile-time.md).

---

## Header imports

```go
package main

import (
	"fmt"

	"./physics.h"   // a local header: package physics
	"<cmath>"       // a C++ header: brings the namespace std
	rl "<raylib.h>" // a header with a name you choose
)

func main() {
	rl.InitWindow(800, 450, "demo")
	physics.step(0.016)
	fmt.Println(std.sqrt(2.0), C.int(42))
}
```

**A header is a package.**
- The shape of the import path says what it is. A path in `<...>` is a system header. A path
  that ends in `.h`, `.hh`, `.hpp`, `.hxx`, `.h++` or `.inl` is a local header (found next to the
  `.gp` file, then in the include directories). A path that starts with `::` is a C++ namespace
  that you rename (below). Every other path is a gocpp package, but a path that starts with `./`
  or `../` must be a header.
- The package name is the file name without its directory and its last extension:
  `<SDL2/SDL.h>` is `SDL`, `"./physics.h"` is `physics`, `<foo.bar.h>` is `foo_bar`. Characters
  that Go doesn't allow in names become `_`.
- The members are the header's C-level names: functions, types, global variables, enum constants
  and macros (`raylib.InitWindow`, `raylib.Color`, `raylib.RAYWHITE`).
- Rename a header like a Go import (`import rl "<raylib.h>"`), or import it without a name
  (`import _ "<GL/gl.h>"`). A name you choose can't be `C`, `init` or a predeclared Go name such
  as `string`: use another one, like `cstring`.
- A header gets **no name** when its name would be a Go keyword (`<map>`, `<sys/select.h>`), a
  predeclared Go name (`<string>`, `<string.h>`, `<new>`), a name that another import or a
  package-level declaration already uses (`<math.h>` next to `"math"`), empty, a name that starts
  with a digit (`<3dmath.h>`), or `C`, `_` or `init`. Give it a name if you need its C names:
  `import cstring "<string.h>"`. If you use it by its file name anyway, the error tells you how.
  All the rules are in [interop.md](interop.md#header-packages-and-their-names).
- A header import never causes an "imported and not used" error, because it also brings
  namespaces and macros.

**C++ namespaces are packages.**
- Every namespace that the imported headers declare can be used directly, with no import:
  `std.vector[int]`, `std.chrono.seconds`, `ImGui.Begin`. This works in files that import at least
  one header. In other files, such a name is a normal `undefined` error.
- In a type, a C++ name may have more than one dot: `var d std.chrono.seconds`
  ([D20](#d20-type-names-with-more-than-one-dot)).
- A local name hides a namespace, as Go's scoping says (a variable named `std`).
- A typo such as `stdd.vector` is reported as `undefined: stdd` before any C++ is generated: gocpp
  asks the C++ compiler which names are namespaces (the namespace probe).
- Rename a namespace that is long or clashes: `import fs "::std::filesystem"`, or
  `import cfmt "::fmt"` next to gocpp's `fmt`. A namespace import without a name
  (`import "::std"`) is an error, because namespaces need no import. The message says: use
  `std.X` directly, or rename it: `import cstd "::std"`.
- If a header's package name is also a namespace (`<glm/glm.hpp>` is package `glm`, and `glm`
  is a namespace), `glm.X` looks in the namespace first, then in the C-level names.

**`C` holds only C's built-in types and the boundary helpers**, as in cgo: `C.int`, `C.char`,
`C.size_t`, …, `C.CString`, `C.GoString`, `C.cstr`, …. Everything a header declares comes through
its package or a namespace. Every `.gp` file can use `C` without importing it, and the name `C`
is reserved ([D19](#d19-c-is-a-reserved-name)).

**Style:** put the header imports in their own group, after the gocpp imports. `gocpp fmt` keeps a
group that contains a header or a namespace import in the order you wrote it, because the order
of `#include` lines can matter in C and C++.

**No preamble.** gocpp has no inline C/C++ code in comments. A small helper goes into a header
next to the `.gp` file, and that header is a package like any other.

**Phase:** [01](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification)
(imports, names, type checking), [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md)
(C++ output, building), [12](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md)
(real C++ types). The full rules are in [interop.md](interop.md).

---

## Must functions and must calls

```go
// Go today
func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

```go
// gocpp: ! on the function (a must function)
func loadConfig!(path string) Config {
	var cfg Config
	data := os.ReadFile(path)  // error → returned for you
	json.Unmarshal(data, &cfg) // error → returned for you
	if cfg.Port == 0 {
		return Config{}, errors.New("port is missing") // one extra value: fail on purpose
	}
	return cfg
}

// a normal function: ! on one call
func port(path string) (int, error) {
	cfg := loadConfig!(path) // error → returned for you
	return cfg.Port, nil
}

// no ! anywhere: plain Go
func portByHand(path string) (int, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		return 0, err
	}
	return cfg.Port, nil
}

func main!() {
	p := port("app.json") // error → printed, exit 1
	fmt.Println("listening on", p)
}
```

`!` (must) works in exactly two places, and only where you write it: on a function and on one
call. Without `!`, the code is plain Go.

**Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#21-part-a--must) (built, always available, [ADR-095](../adr/095-must-and-macros-are-always-available.md)).
`!` is pure sugar: it is rewritten into the same early returns you would write by hand, so it
costs nothing at run time.

### Must functions: `func f!(...) T`

- **What is handled.** Inside a must function, every call whose `error` result you **drop** is
  handled for you. If the error is not nil, the function returns it. The other results become
  zero values, and defers still run.
- An error is dropped when the call is a statement (`json.Unmarshal(data, &cfg)`), or when its
  results are used as fewer values than it returns (`data := os.ReadFile(path)`,
  `n := strconv.Atoi(s) * 2`).
- If you take the error yourself (`data, err := os.ReadFile(path)`), or drop it on purpose
  (`_ = f.Close()`), nothing changes.
- **The signature.** A must function gets an extra `error` result, so callers see
  `func loadConfig(path string) (Config, error)`. Inside it, `return cfg` means success
  (`return cfg, nil`), and `return cfg, err` (one extra value) returns an error on purpose.
  `func run!()` with no results becomes `func run() error`, and its `return` means `return nil`.
- **Declaring the error yourself.** If the last declared result is already `error`, nothing is
  added and the returns are normal Go. Do this when you need the name, for example to add
  context with `defer errors.Wrapf(&err, ...)` ([D17](#d17-a-few-additions-to-the-standard-packages)).
- **`func main!()` and `func init!()`:** an error is printed to stderr as `<program>: <error>`
  (`<program>` is the base name of `os.Args[0]`), and the program exits with code 1. For
  `main!`, `main`'s defers run first. For `init!`, the program stops before `main` starts.
- **Methods** can be must functions too: `func (s *Server) Start!() { ... }`. An interface
  spells the real signature (`Start() error`), and the `!` method satisfies it.
- **`defer f.Close()`** inside a must function: if `Close` fails while the function returns a nil
  error, the function returns `Close`'s error.
- **Closures** inside a must function are separate functions, so nothing is handled inside them
  automatically. Use must calls there.
- **`go f()`:** the error can't come back from another thread, so it is not handled. `gocpp vet`
  warns about it (analyzer `gobang`).
- **Every dropped error means every one**, `fmt.Println(...)` included. That is harmless and keeps
  the rule simple. The editor marks every handled call with an inlay hint (Phase 11).
- A call whose last result is a **concrete** error type (`*MyErr`, `syscall.Errno`) is not
  handled. If you drop its error, the compiler asks you to take it yourself (see below why).

### Must calls: `F!(...)`

```go
data := os.ReadFile!(path) // (T, error): you get T
n := strconv.Atoi!(s) * 2  // inside an expression
a, b := split!(s)          // (A, B, error): you get a, b
f.Close!()                 // only an error: use it as a statement
v := parse![int](s)        // with explicit type arguments
```

- `F!` works on any named function or method whose **last result is exactly the type `error`**:
  `os.Open!(p)`, `strconv.Atoi!(s)`, `w.Write!(b)`, `f.Close!()`, must functions
  (`loadConfig!(path)`), and generic functions with explicit type arguments (`parse![int](s)`).
- `F!` gives back every result except the error:

  | `F` returns | Use `F!` as |
  |---|---|
  | `(T, error)` | `x := F!(...)`, or inside an expression: `g(F!(...))` |
  | `(A, B, error)` | `a, b := F!(...)` |
  | `(A, B, C, error)` | `a, b, c := F!(...)` |
  | `error` | a statement: `F!(...)` |

- A **concrete** error type as the last result (`*MyErr`, `syscall.Errno`) is a compile error.
  Go's typed-nil rule would turn a nil `*MyErr` into a non-nil `error`, so success would look like
  failure. Convert it to `error` yourself.
- **Not in v1:** `!` on arbitrary expressions (`fns[i]!()`; write `f := fns[i]` and then `f!()`),
  `!` on function literals, and `!` on C functions (they don't return Go errors).

### What happens when the error is not nil

This holds for `F!` and for the calls a must function handles.

| Where the call is | What happens |
|---|---|
| a function whose last result is `error` (every must function too) | the other results become zero values, the error result becomes `F`'s error, and the function returns. Defers still run, so they can wrap the error |
| `main.main` (`F!` in `func main()`, or `func main!()`) | `main`'s defers run, then `<program>: <error>` is printed to stderr and the program exits with code 1 |
| a function with a `*testing.T`, `*testing.B`, `*testing.F` or `testing.TB` parameter and no error result | `t.Fatal(err)`: the test fails and stops |
| any other function with no error result (also `init` and package-level `var` initializers) | panic with the error, like Go's `Must` helpers (`regexp.MustCompile`) |
| `defer f.Close!()` | if `Close` fails and the function was about to return a nil error, it returns `Close`'s error instead. In a function without an error result the failure panics; in `main` it is printed and the program exits 1. If a panic is already running, the `Close` error is dropped and the panic goes on |
| inside a closure | the error leaves the closure (through the closure's own results), not the outer function |
| `go F!()` | compile error: nobody could receive the error |
| inside a `lazy` macro argument | compile error: `!` would silently turn into a panic there |

`go func() error { ... F!() ... }()` drops the error just like `go F!()` would. `gocpp vet` warns
about it (`gobang`).

### Reading `!`

| You write | It means |
|---|---|
| `os.ReadFile!(path)`, `parse![int](s)` | a must call: `ReadFile!` and `parse!` are one name each |
| `func loadConfig!(path string)`, `func (s *Server) Start!()` | a must function or method |
| `if!(a && b)`, `return!(x)`, `case!(b):`, `for!(a)` | plain Go: `!` means "not", because `if`, `return`, `case` and `for` are keywords |
| `x != y`, `!ok` | plain Go |

- `name!(` and `name![` are one name only when `name` is **not** a Go keyword. That is why the
  keyword forms keep their Go meaning.
- `x != y` and `!ok` are not affected: `!=` is a different token, and a prefix `!` never comes
  directly after a name.

### Error messages

Mistakes with `!` get messages that talk about your code, not about the rewrite (the hidden
helper functions never show):

```text
typed! requires the last result of typed to be exactly error, got *MyErr: a nil *MyErr would turn into a non-nil error; convert it explicitly
fmt.Sprint! requires the last result of fmt.Sprint to be error, got string
os.Remove!(...) returns only an error: use it as a statement
two!(...) gives 1 value besides the error, but here 2 values are wanted
three!(...) gives 4 values besides the error: assign them to 4 variables (a, b := three!(...))
len!(...) is a call of a built-in function: ! needs a function whose last result is error
go two!(...): the error of two can't be returned to anyone from another goroutine; call it in a func literal that handles the error
the last result of typed is *MyErr, not error: a must function does not handle it (a nil *MyErr would be a non-nil error); take it yourself, as in err := typed(...), or write _ = ...
func main! must have no parameters and no results (an error is printed and the program exits with 1)
```

`func main!()` and `func init!()` take no parameters. Like every must function they get an error
result, so `return err` works in them and a bare `return` means success; the error is printed
and the program exits with 1.

A `defer f.Close()` (or `defer f.Close!()`) that fails while a panic is already running, in a function
without an error result, drops the `Close` error: the panic goes on. A `defer f.Close()` that
fails in a function that returns an error leaves the other results as the function set them: only
the error changes. A `!` on a C or C++ function is an error for now, and a `!` in the channel
operation of a `select` case too (evaluate it before the `select`).

### Tools

- `gocpp expand file.gp` prints the code after `!` and macros are rewritten (the rewritten code is plain
  Go that `go/types` checks, see [ADR-037](../adr/037-must-rewrite.md) and the
  [guide](../guides/expand.md)).
- In the editor (Phase 11, step 11.13, [ADR-099](../adr/099-the-editor-shows-the-bang-and-macro-forms-from-the-lowerings-own-lists.md)),
  hover on a must function shows its real signature, hover on `F!` says what happens on error at that
  spot (return, panic, exit 1 or `t.Fatal`) with the code that stands for the statement, an inlay hint
  after every handled call (`→ return err`, and `!→ return err` for a call that a must function handles
  without a `!`), and *Show expansion* prints what `gocpp expand` prints
  ([the editors guide](../guides/editors.md#-macros-and-derive)).
- `gocpp vet` warns about `!` that would panic inside a library package (libraries should return
  errors: `panicbang`, which also follows calls into other packages), about `go` statements that
  lose errors (`gobang`), about a dropped error where the function could return it (`ignorederr`),
  and about lazy parameters that are never used (`unusedlazy`). Phase 13, step 13.3b
  ([guide](../guides/vet.md)).

---

## Macros

### Declaring a macro

```go
package util

import (
	"fmt"
	"os"
	"sync"
)

// Inside a macro, #file(), #line(), #func() and #text(arg) describe the place where the macro is USED.

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Dbg[T any](v T) T {
	fmt.Fprintf(os.Stderr, "[%s:%d] %s = %v\n", #file(), #line(), #text(v), v)
	return v
}

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}
```

- A macro looks like a func, with the keyword `macro`. It is type-checked **once, where it is
  declared**, so its errors show up in the macro, not at every use.
- It can be generic (`Dbg[T any]`) and return values.
- A `lazy` parameter (`msg lazy string`) is **not** evaluated at the use. The macro calls `msg()`
  when, and only if, it needs the value. Other parameters are evaluated once, like normal
  function arguments, so there are no C-style double-evaluation surprises.
- A macro is a real function: its variables can't clash with yours, and the editor sees it
  (completion inside it, go to definition from `#util.Dbg`).

### Using a macro: three places

```go
#util.Trace() // on top of a func: runs at the start; the func() it returns runs at the end
func loadConfig!(path string) Config {
	#util.Assert(path != "", "path is empty") // inside: as a statement
	data := os.ReadFile(path)
	return parse(data)
}

#util.Locked(&s.mu) // on top of a method: lock on entry, unlock on exit
func (s *Store) Put(k string, v int) {
	s.m[k] = v
}

#Derive(String) // on top of a type: generate methods
type Color int

func main!() {
	cfg := loadConfig("app.json")
	port := #util.Dbg(cfg.Port) // inside: as an expression; also prints "[file:line] cfg.Port = 8080"
	fmt.Println("listening on", port)
}
```

| Where | What it does |
|---|---|
| **inside** a function, as a statement or an expression | a normal call of the macro, which also gets the place of the use |
| **on top of a func or method**, start style (package level only) | the macro runs as the function's **first statement**. If it returns a `func()`, that runs when the function ends, even when it panics; a nil result is skipped. The arguments are evaluated inside the function, so they can use the parameters and the receiver (`&s.mu`) |
| **on top of a func or method**, wrap style (the macro uses `#body()`) | the macro gets the function's code and decides how to run it (next section) |
| **on top of a type** (package level only) | `#Derive(...)` generates methods next to the type |

- Several macros on top of one func apply from top to bottom: the first one is the outermost,
  like stacked decorators. Start style and wrap style mix freely.
- A doc comment goes **above** the `#` lines, so `go/doc` still attaches it to the function.
- Inside a function body, `#Name(...)` is always a use inside code.

### Wrap style: `#body()`

```go
macro VM() {
	vmEnter(#func())
	defer vmExit(#func())
	#body() // the function's own code runs here
}

macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

macro InThread() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		#body()
	}()
	<-done
}

macro Once(o *sync.Once) {
	o.Do(func() { #body() })
}
```

```go
#util.Trace() // start style: runs first; its func() runs last
#util.VM()    // wrap style: the code below runs inside VM
func secret(key string) (int, error) {
	if key == "" {
		return 0, errors.New("empty key")
	}
	return len(key) * 42, nil
}
```

- A macro whose body uses `#body()` is a **wrap macro**. It can only be used on top of a function.
- `#body()` runs the function's code. A `return` in that code sets the function's results and
  ends `#body()`.
- The macro decides how the code runs: once (`VM`), maybe never (`Once` on later calls; the
  results stay zero values), on another thread (`InThread`, which waits for it), several times (a
  retry loop), or under `recover` (`Recover`: a panic stops there and the function returns zero
  values).
- The `defer`s in the function's code run when that code ends, that is when `#body()` returns,
  before the macro goes on.
- A macro that starts `#body()` on another thread must wait for it before it returns, as
  `InThread` does. `gocpp vet` (analyzer `unwaitedbody`) warns when `#body()` runs in a `go` statement
  that nothing waits for.
- **v1 limit:** `#body()` gives the macro no access to the function's results, so a `#Retry(3)`
  that looks at the returned error is not possible yet ([open points](#open-points)).

### Built-in macros

| Macro | Gives |
|---|---|
| `#file()`, `#line()`, `#func()` | where it is written; inside a macro body, the **outermost** use site (your code) |
| `#text(x)` | the source text of `x`; for a macro parameter, the text the caller wrote (`"path != \"\""`) |
| `#body()` | inside a macro used on top of a function: runs that function's code (wrap style) |
| `#embed("logo.png")` | a string constant with the file's bytes (Go's `//go:embed` works too) |
| `#Derive(...)` with `String`, `Equal`, `Clone` or `JSON` | on top of a type: generated methods (below) |

**How the built-ins work (step 10.8, [ADR-058](../adr/058-builtin-macros-and-embed-in-the-loader.md)).** Each
use is replaced by a Go constant in parentheses, before type checking, and keeps the position of the
use, so the built-ins are untyped constants (`const here = #line()`, `[#line() % 7]int`, `case #line():`).

- `#file()`: the path of the file from the root of its module with slashes (`util/util.gp`), or the
  file name for a program without `gocpp.mod`. A `//line` comment changes it (the module-relative or base name of what it says, `??` when it has none: a relative or an absolute name can differ from Go's runtime).
- `#line()`: the line of the use (an integer constant).
- `#func()`: the name that the Go runtime prints for code that gc does not inline: `main.f`,
  `main.(*T).M`, `main.T.M`, `main.f.func1`, `main.f.func2.1` (a literal in a literal),
  `main.init.0` (the first `func init()`), `main.init.func1` (a literal in a package-level
  initializer, counted across the files of the package), `main.F[...]`, `main.(*G[...]).M`; for a
  package other than `main`, its import path.
- `#text(x)`: the source text of `x` from its first token to its last, with each line break (and its
  indentation) and each comment inside it turned into one space. `x` is not evaluated and not checked.
- `#embed("file")`: an untyped string constant with the bytes of a file of the folder of the source
  file or below it. One file name, not a pattern; at most 16 MiB (64 MiB for a package).
- `//go:embed pattern...` on a package-level `string` or `[]byte` variable gives it the bytes, with
  Go's rules and error messages ([guide](../guides/embed.md)). `embed.FS` waits for `io/fs` (Phase 09).
- Mistakes (an argument to `#line()`, `#text()` without an argument, `#body()` outside a macro,
  a file that does not exist) are errors at the use. Used inside a macro declaration, `#file()`,
  `#line()`, `#func()` and `#text(param)` describe the use of the macro (step 10.9, below).

**Nested macros keep both places.** If a macro `Check(v int)` uses `#Assert(v > 0, ...)`, then
inside `Assert`, `#text(cond)` is `"v > 0"` (the text of the inner use), while `#file()` and
`#line()` point at your call of `Check`.

### `#Derive`

```go
#Derive(String)
type Color int

const (
	Red Color = iota
	Green
	Blue
)
```

generates (`gocpp expand` shows it):

```go
func (c Color) String() string {
	switch c {
	case Red:
		return "Red"
	case Green:
		return "Green"
	case Blue:
		return "Blue"
	}
	return "Color(" + strconv.Itoa(int(c)) + ")"
}
```

`Equal` and `Clone` are for structs and for named array, slice and map types:

```go
#Derive(Equal, Clone)
type Order struct {
	ID    int
	Items []Item
	Owner *Person
	Attrs map[string][]string
}

a := order.Clone()    // a deep copy: changing a.Items[0] leaves order alone
order.Equal(a)        // true: slices, maps and pointers are compared by what they hold
```

The methods are written when the package has been type-checked and they are ordinary methods: they are
visible to every caller (also in other packages), `gocpp expand` prints them, and hover and go to
definition on `order.Equal` land on the name in the `#Derive` line. The generated code imports
nothing (`#Derive(JSON)` is the exception: it calls the `encoding/json` that the file imports), so `#Derive(String, Equal, Clone)` works in a file that imports nothing.

| Derive | Generates |
|---|---|
| `String` | `String() string` for **integer types with constants**, in the same style as Go's `stringer` tool: the name of the constant, or `Type(n)` for another value. Constants that share a value use the first name in declaration order. A struct is an error (`fmt` prints structs by itself: `%+v` shows the field names) |
| `Equal` | `Equal(other T) bool`, deep equality: a field whose type has an `Equal(T) bool` method uses it (`time.Time`, other derived types, your own); arrays, slices and maps are compared element by element (a nil slice or map equals an empty one); a pointer is equal to the same pointer, or to a pointer whose pointee is equal; a pointer to a type of another package without `Equal` is compared by identity; an interface field is compared with `==` (or with its `Equal` method); a field of a type parameter with `==` (`comparable`), its `Equal` method, or `any(a) == any(b)` (**panics at run time when the type argument is a slice or a map**). A func field, a C or C++ type, a type of another package that cannot be compared, and a type that holds pointers, slices or maps and has no `Equal` method are errors that say which field |
| `Clone` | `Clone() T`, a deep copy: slices, maps, arrays, pointers and structs are copied all the way down (nil stays nil, an empty slice stays empty); a type with a `Clone() T` method of its own gets it called (for a field, an element or a pointee); a pointer to a type of another package without `Clone` is shared (so a copy `Equal`s its original: `*time.Location`, `*sync.Mutex`); channels, funcs, interface values and values of type parameters are shared (a `V` that is a slice is shared; an interface with `Clone() I` is cloned through it); a value of another package is a value (`time.Time`) unless it shows a slice, map or pointer in an exported field and has no `Clone` (an error that asks for one); a struct of this package that refers to memory and has no `Clone` is an error that says to add `#Derive(Clone)` to it. Sharing is not kept: a node reached twice becomes two. A struct with a `sync.Mutex` copies the lock state (`gocpp vet` reports it at the `#Derive` line) |
| `JSON` | for **structs**: `MarshalJSON() ([]byte, error)` and `UnmarshalJSON(data []byte) error`, which call `encoding/json` on a copy of the struct type that has no methods (`type gp_plain T`), and one more, `GoCPPJSONPlainPtr`, with which `Marshal` and `Unmarshal` take the value as that struct without calling the first two (so a nested value costs what a plain one costs, not the square of its depth; `reflect` shows the extra method: `NumMethod` of the pointer type is 3, not 2), so tags, `omitempty`, embedded structs, escaping, number formats and errors are `encoding/json`'s own ([ADR-102](../adr/102-encoding-json-is-go-s-own-package-and-derive-json-calls-it.md)). **It is an API shim: no faster than a plain struct** (it goes through the same reflection: `Marshal` 1.05 to 1.15 times and `Unmarshal` 1.06 to 1.10 times a plain struct's time, measured on slices of 300000 structs and on a small nested one with g++ -release; the type also adds generated code: 100 derived types of five fields are 2.21 MB of generated C++ against 1.70 MB without `#Derive`, and the build time follows). **The file must import `encoding/json`** (any name but `_`); the import needs no other use. A type that is not a struct is an error. **Differences from a plain struct, to know before you use it:** `Decoder.DisallowUnknownFields`, `UseNumber` and `Encoder.SetEscapeHTML(false)` reach a derived value as they reach a plain struct, but not when the program calls `UnmarshalJSON` or `MarshalJSON` itself; a by-value field whose type has a pointer-receiver `MarshalJSON` is marshaled by value in the derived struct and through the pointer in a plain one that is marshaled through `&v`; decoding goes on after a type mismatch (the error is kept), as for a plain struct, where the old method stopped at the first error; error texts say `gp_plain` for the type name; the methods called by hand give offsets of syntax errors relative to the value and wrap a failure inside a field's `MarshalJSON` once more |

A type that has the method already (declared by you) is an error: remove one of them. `#Derive` works
on generic types (`#Derive(Equal, Clone) type Pair[K comparable, V any] struct { ... }`). **A value that
refers to itself (a cycle) is not supported**: `Clone`, and `Equal` of two different cyclic values, overflow
the stack like the hand-written code would ([ADR-080](../adr/080-derive-writes-methods-from-the-types.md)).

### Rules (v1)

The compiler checks each of these rules and gives a clear error (`gocpp explain` knows each message):
- `macro` declarations only at package level; a macro has no receiver, is declared once, and is not
  named like a built-in macro (`file`, `line`, `func`, `text`, `embed`, `body`, `Derive`); names that
  start with `gp_` are reserved in a macro.
- A `lazy` parameter type must be concrete and written the same way at every use: a predeclared type,
  a type of the macro's package (exported, when the use is in another package), or a type of a package
  that the file of the use imports; not `lazy T` of the macro's own type parameter, not a struct or
  interface type with members.
- A macro is only used with `#`: `#Name(...)`, `#pkg.Name(...)`, on top of a func. A plain call
  `util.Dbg(x)` or a macro used as a value (`f := util.Dbg`) is an error ("Dbg is a macro: write
  #Dbg(...)").
- A use has one argument for each parameter (a variadic parameter takes any number, or one slice with
  `...`).
- A macro that returns `func()` can't be a statement inside code (`#util.Locked(&mu)` on its own
  line would drop the unlock). Use it on top of a function, or call the result.
- A macro can't be the call of a `defer` or `go` statement.
- `#body()` only inside macros, and a macro that uses it only on top of a function (it can't be used
  on a must function, `func f!()`, yet).
- `!` can't be used in a lazy argument.
- A macro can't `return` or `break` out of the code that uses it (use `!` for errors).
- An unexported macro (`macro helper()`) can only be used in its package.
- A variable (or parameter) with the name of a macro, in a function that uses the macro, hides it:
  `show := 5` next to `#show(1)` is an error at the use. A macro and a func of a package can't share a name.
- On types, only the built-in `#Derive` exists in v1: `#Derive(String, Equal, Clone)`, written on one type
  declaration (not on a group `type ( ... )`, an alias, or an interface, func, channel or pointer type);
  each generator is written once, and the type has no method of that name.

Also good to know:
- A `defer` inside a macro body runs when the **macro** returns, not when the caller returns (the
  macro is a function). To run code when the caller ends, return a `func()` and use the macro on
  top of the function.
- A lazy argument is an expression of the caller's code: it is type-checked at the use, and it sees the
  variables as they are when the macro calls it (it is a closure).
- Each run of the code of a wrapped function (a macro may run `#body()` several times) is a call of the
  function's body: named results start at zero, and the results of the function are those of the last run.
- A function with a wrap macro can not call `recover()` itself (outside a func literal): its code runs
  in a func literal that the macro calls, so `recover()` would not see the panic. The compiler says so;
  call `recover()` in a deferred func literal inside the function, which works.
- C macros stay usable for C APIs, through their header packages (`raylib.RAYWHITE`).

### How macros are compiled

([ADR-075](../adr/075-user-macros-are-generic-functions.md).) The loader rewrites macros into plain Go
before `go/types` runs, so a macro is type-checked once, where it is declared.

- A macro becomes a generic function of the same name with hidden parameters: the file, the line and the
  function of the outermost use (`gp_file`, `gp_line`, `gp_func`), the text of each parameter that the
  body gives to `#text` (`gp_text_x`), and for a wrap macro the function's code (`gp_body`). A `lazy T`
  parameter becomes a type parameter constrained to `func() T`.
- `#util.Dbg(a * 2)` becomes `util.Dbg("main.gp", 12, "main.f", "a * 2", a*2)`, and a lazy argument a
  func literal: `util.Assert("main.gp", 12, "main.f", "n > 0", n > 0, func() string { return msg })`.
- A start-style macro becomes the first statement of the function (a `func()` it returns is deferred
  unless it is nil). A wrap-style macro becomes `util.VM("main.gp", 12, "main.f", func() { <the function's
  code> }); return`: the function gets new results, and the code runs unchanged as a func literal with the
  old results, whose values the func literal passes on (`gp_r0, gp_r1 = func() (n int, err error) { <code> }()`),
  so Go's rules for results (a hidden result, a defer that changes it, `return f()`) apply to the code.
- In C++, a generic function is lowered by the package that uses it, in the unit that uses it, so the C++
  compiler inlines a macro across packages. The parameters for a lazy argument and for `#body()` are C++
  template parameters of that instance: the argument is the lambda itself, and nothing is allocated.
- `gocpp expand file.gp` shows the code after expansion ([guide](../guides/expand.md), step 10.11).
- **Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros) (built, always available, [ADR-095](../adr/095-must-and-macros-are-always-available.md)).

### Syntax: the exact rules

This is what the parser accepts (parser patches 4 to 6, Phase 10 step 10.1), and how `gocpp fmt`
prints it. The rest of this document says what the forms mean.

**Names with `!` and `#`.** The scanner makes two kinds of names that Go doesn't have:

| Written | Is one name | When |
|---|---|---|
| `F!(`, `os.ReadFile!(`, `parse![` | `F!`, `ReadFile!`, `parse!` | a name that is not a Go keyword, with the `!` **directly** after it (no space, no comment), and `(` or `[` **directly** after the `!` |
| `#Name`, `#util.Dbg(`, `#func(` | `#Name`, `#util`, `#func` | a `#` directly followed by a letter or `_`. Keywords too: `#func`, `#type` |

Nothing else changes: `a != b`, `a!=b`, `!ok`, `a !(b)`, `a! (b)` and `if!(a)` are scanned as in Go, a `#`
that is not followed by a letter is an illegal character as in Go, and a `#` or a `!(` in a string, a rune or
a comment is text. These names exist only in gocpp's syntax (the parser option `Phase10Syntax` of the fork, which every gocpp tool sets); the parser, `gocpp fmt` and the
language server read it always: there is no switch and no phase gate (before step 10.12 the compiler said
"must calls are not supported yet (Phase 10)", [ADR-095](../adr/095-must-and-macros-are-always-available.md)). The standard library is parsed the same way, which
changes nothing for Go code.

A keyword after a dot is a name (D3), but never a must call: `x.type!(1)` is `x.type`, `!` and
`(1)`. In the parameter list of a macro, `lazy` right after a parameter name is always the
modifier, so a type named `lazy` can't be used there (`macro M(a lazy.T)` is an error if a package is
named `lazy`; everywhere else `lazy` is an ordinary name).

**Where each form may stand.** A name with a `!` is the function or method that is called (`F![int].x(1)` calls `x`, so it is an
error), or the name of a `func` declaration; a name with a `#` starts a macro call (`#a.b.c(x)` and
`#a[int](x)` are calls, `#a[int].b(x)` is an error). The parser rejects them anywhere else, with a
message that names the place:

```text
x.gp:2:9: F! must be called: write F!(...)            // x := F![int]
x.gp:2:5: cannot use f! here: a name with ! can only be a called function or method, or a func declaration
                                                       // var f!(int)
x.gp:2:11: cannot use #T here: a name with # can only be the start of a macro call
                                                       // var _ #T, x.#y(z), func g(#a int)
```

`F!` may have type arguments: `parse![int](s)`, `pkg.parse![int, string](s)`. A `!` after anything but a
name (`fns[i]!()`, `f()!()`, `(F)!(x)`) stays a Go syntax error: "expected ';', found '!'".

**Macro declarations.** `macro Name[TypeParams](params) results { body }` at package level, where the word
`macro` followed by a name starts a declaration. It has no receiver, and it needs a body. Inside a
function, a type or a parameter list `macro` is an ordinary name. A `lazy` parameter is `name lazy T`
(names may be grouped: `a, b lazy int`), only in the parameter list of a macro: not in its results, its
type parameters or a func type inside it, where `lazy` is an ordinary name.

**Macros on top of a declaration.** One or more lines `#Name(...)`, `#pkg.Name(...)` or `#Name[T](...)` at
package level, directly above a `func`, a method or a `type`. Each line is one call. They must be followed by
one of those: not by `var`, `const`, `import`, another `macro` or the end of the file ("macro calls must be on
top of a func or type declaration"). A doc comment goes above the lines and belongs to the declaration.

**`gocpp fmt`.** It formats this syntax with its own fork of `go/printer` (`internal/syntax/printer`),
and prints Go exactly as `gofmt` does (the tests compare it with `gofmt` on the whole Go standard
library). The lines of macros stay directly above their declaration (no empty line after them), with one
empty line before them as before a `func`; an empty line you write after them stays. A comment between the
lines of macros and the declaration gets an empty line before it, as gofmt does for a comment between two
declarations. `gocpp fmt` formats a file with this syntax before the compiler accepts it, as it formats a file
with interfaces before Phase 05.

**Trees.** The parser makes stock `go/ast` nodes: the name of a must call or function ends with `!`, a
macro use starts with `#`, a macro declaration is a `*ast.FuncDecl` whose name starts with `#`, a lazy type is
a `*ast.UnaryExpr` with the operator `token.ILLEGAL`, and a line of macros on top is a `*ast.GenDecl` with the
token `token.ILLEGAL` before the declaration. `internal/syntax/gpast` documents and checks these conventions.

---

## Open points

The plan has not settled these yet. This document will describe each one when a phase builds it.

| Topic | State | Where it is discussed |
|---|---|---|
| Operator overloading or destructors for gocpp types | open; revisit after v0.1 (Go has neither) | [TODO: open questions](../../plans/2026-10-13-gocpp/TODO.md#open-questions-no-decision-needed-yet) |
| `!` for comma-ok results (`v := m[k]!` fails when the key is missing) | not in v1; revisit after Phase 10 | [TODO: open questions](../../plans/2026-10-13-gocpp/TODO.md#open-questions-no-decision-needed-yet) |
| `err := #body()`, so wrap macros can look at the function's error (a `#Retry(3)` macro) | planned right after v1 | [Phase 10 §2.2](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros) |
| Error return traces (`errors.Trace(err)`, `%+v`) | optional step; it starts with a design note (when a trace starts, how it is attached, when it is cleared) | [Phase 10 §2.1](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#21-part-a--must) |
| User-written macros on types, macros that generate declarations, procedural macros | backlog | [Phase 10 §2.2](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros), [TODO: backlog](../../plans/2026-10-13-gocpp/TODO.md#ideas--backlog-not-scheduled) |
| An inline C++ preamble for tiny helpers | backlog (v1: a header next to the `.gp` file) | [TODO: backlog](../../plans/2026-10-13-gocpp/TODO.md#ideas--backlog-not-scheduled) |
| `#!` shebang lines, so `.gp` files can run as scripts | backlog | [TODO: backlog](../../plans/2026-10-13-gocpp/TODO.md#ideas--backlog-not-scheduled) |
