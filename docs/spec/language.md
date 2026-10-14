# The gocpp language

gocpp follows The Go Programming Language Specification (go1.25) except for the points below.

The Go spec is at <https://go.dev/ref/spec>. gocpp pins the go1.25 language version: the type
checker runs with `GoVersion: "go1.25"` ([Phase 01 §2.7](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#27-gotypes-configuration)).
Everything this document doesn't mention works as the Go spec says.

| | |
|---|---|
| Status | v0, written in Phase 00 from the plan. Each phase updates it when it builds a feature |
| Source | [Phase 00 §2.2](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go) (the divergence list D1–D21; D22 from the Phase 02 evaluation-order work) and the phase READMEs linked in each section |
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
| [D8](#d8-map-order-is-not-randomized) | random map order | unspecified, not randomized | 03 |
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
| [D22](#d22-which-of-two-run-time-panics-is-reported) | one panic message, fixed by the compiler | with two possible panics and no call between them, the other message can appear | 02 |

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
  Not in the subset: `reflect.MakeFunc`, `reflect.StructOf`, `reflect.FuncOf`, calling methods
  by name, and anything else that needs new code at run time. There is no `plugin` package, and
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
- **gocpp:** every `go` statement starts a real OS thread. There is no goroutine scheduler, now or
  later. Tens of thousands of goroutines are fine; millions are not.
- Each thread gets an 8 MiB stack by default (memory is only used when a page is touched). A
  stack overflow prints `fatal error: stack overflow` and exits with code 2.
- The Go names stay: `go`, "goroutine", `runtime.NumGoroutine`. `runtime.LockOSThread` always
  holds, because every goroutine already has its own thread. `runtime.GOMAXPROCS` remembers the
  value you set, but gocpp doesn't use it for scheduling.
- **Phase:** [08](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#22-goroutines--os-threads).

## D8. Map order is not randomized

```go
m := map[string]int{"a": 1, "b": 2, "c": 3}
for k, v := range m {
	fmt.Println(k, v) // some order: don't depend on it
}
fmt.Println(m) // map[a:1 b:2 c:3]: fmt sorts the keys, as in Go
```

- **Go:** the spec leaves the iteration order unspecified, and the Go runtime randomizes it on
  purpose.
- **gocpp:** the order is unspecified too, but gocpp does **not** randomize it. Programs still
  must not depend on any order.
- **Phase:** [03](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#23-strings-slices-arrays-maps).

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

- **Go:** a program with a data race has undefined behavior.
- **gocpp:** the same. On top of that, a race on a pointer, a slice or a string can corrupt its
  reference count, so memory can be freed too early or never. Find races with `-race`
  (ThreadSanitizer). `-race` works on Linux only.
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
- **Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#21-part-a--must).
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
- **Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros).
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
| `func(...)` | 8 | 32 | a `std::function` wrapper ([Phase 03 §2.6](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#26-functions-func-values-closures)) |
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
  `[0]T` is 1 byte too, with alignment 1. An array `[N]T` is `N` times the size of `T`.
  A slice of empty structs therefore uses memory: `make([]struct{}, n)` allocates `n` bytes, and a
  huge `n` (Go allows `math.MaxInt`, because its zero-size elements take no memory) fails with an
  out-of-memory error in gocpp.
- The sizes of `chan T`, func values and interface values are placeholders until their
  phases build them (08, 03 step 3.11, 05); they can still change there.
- Only the C++ compiler knows the size of a C/C++ type (`C.int`, `*stdio.FILE`,
  `std.vector[int]`), so until
  [Phase 04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md) `unsafe.Sizeof`,
  `Alignof` and `Offsetof` are an error when their value depends on one: a type that holds a
  C/C++ value directly or in a field or an array element, also nested
  (`cannot use unsafe.Sizeof(x): its type has a C/C++ part (field n C.int), whose size gocpp
  only knows from Phase 04`). A Go pointer, slice or map of such a type is fine: its size
  doesn't depend on what it points to. `Offsetof(x.f)` is fine when the C/C++ fields come
  after `f`.
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
  - a pointer that comes **from C** owns nothing;
  - C code never keeps gocpp memory alive. A gocpp pointer passed to C is only **borrowed**, as
    in cgo's pointer-passing rules. C must not use it after the gocpp object is gone. For a
    `void*` that C keeps (user data for callbacks), pass a `cgo.Handle` instead
    ([interop.md](interop.md#callbacks-and-panics)).
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
  | `C.wstr(s)`, `C.GoStringW(p)` | UTF-16 strings for Windows `W` functions | 14 |
  | hidden helper packages `gocpp/macro`, `gocpp/must`, `gocpp/cgoh` | the compiler adds and uses them for its rewrites; you don't import them | 04, 10 |

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
  (parser patch 6).

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
  (`import C "<x.h>"`), and `package C` is an error too.
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
  C arrays. The value (and a map's key) gets C++'s element type. As in Go, with one loop
  variable you get the index (the key for maps), and the value is a copy. A random-access
  container is walked by index with its size read once, as Go reads the length once, so a
  `push_back` in the body doesn't break the loop.
- **Phase:** [04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#25-values-and-operations-opaque).
  The C++ form is in [interop.md](interop.md#operations-on-cc-values).

## D22. Which of two run-time panics is reported

```go
s, i, z := "abc", 5, 0
fmt.Println(s[i], 7/z) // Go: "index out of range [5] with length 3"; gocpp may report the division
```

- **Go:** the spec leaves this order open; Go's compiler (gc) happens to check the index first.
- **gocpp:** when one expression holds two operations that can panic at run time (an integer
  division or remainder by zero, a negative shift count, an index out of range) and no function
  call between them decides which runs first, gocpp may report the other of the two panics. The
  program still panics and exits with code 2, and every call, all output and every value read
  happen in Go's order; only the panic message can differ. Keeping gc's order here would need a
  temporary for every such operation, which makes the C++ much harder to read.
- **Phase:** [02](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#25-evaluation-order-minimal-version-here-already).

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

**Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#21-part-a--must).
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
  warns about it.
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
| a function with a `*testing.T`, `*testing.B` or `*testing.F` parameter and no error result | `t.Fatal(err)`: the test fails and stops |
| any other function with no error result (also `init` and package-level `var` initializers) | panic with the error, like Go's `Must` helpers (`regexp.MustCompile`) |
| `defer f.Close!()` | if `Close` fails and the function was about to return a nil error, it returns `Close`'s error instead. In a function without an error result the failure panics; in `main` it is printed and the program exits 1. If a panic is already running, the `Close` error is dropped and the panic goes on |
| inside a closure | the error leaves the closure (through the closure's own results), not the outer function |
| `go F!()` | compile error: nobody could receive the error |
| inside a `lazy` macro argument | compile error: `!` would silently turn into a panic there |

`go func() error { ... F!() ... }()` drops the error just like `go F!()` would. `gocpp vet` warns
about it.

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

Mistakes with `!` get messages that talk about your code, not about the rewrite. For example:

```text
strconv.Itoa!(n): Itoa does not return an error, so "!" can't be used
os.Remove!(p) returns only an error; use it as a statement
F!: the last result of F is *MyErr, not error; convert it explicitly
```

### Tools

- `gocpp expand file.gp` prints the code after `!` and macros are rewritten.
- In the editor, hover on a must function shows its real signature, and hover on `F!` says what
  happens on error at that spot (return, panic, exit 1 or `t.Fatal`). Phase 11.
- `gocpp vet` warns about `!` that would panic inside a library package (libraries should return
  errors) and about `go` statements that lose errors. Phase 13.

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
  `InThread` does. `gocpp vet` warns when `#body()` runs in a `go` statement that nothing waits
  for.
- **v1 limit:** `#body()` gives the macro no access to the function's results, so a `#Retry(3)`
  that looks at the returned error is not possible yet ([open points](#open-points)).

### Built-in macros

| Macro | Gives |
|---|---|
| `#file()`, `#line()`, `#func()` | where it is written; inside a macro body, the **outermost** use site (your code) |
| `#text(x)` | the source text of `x`; for a macro parameter, the text the caller wrote (`"path != \"\""`) |
| `#body()` | inside a macro used on top of a function: runs that function's code (wrap style) |
| `#embed("logo.png")` | a string constant with the file's bytes (Go's `//go:embed` works too) |
| `#Derive(...)` with `String`, `Equal`, `Clone`, `JSON` | on top of a type: generated methods (below) |

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

`JSON` is for structs:

```go
#Derive(JSON)
type Config struct {
	Port int    `json:"port"`
	Name string `json:"name,omitempty"`
}
```

| Derive | Generates |
|---|---|
| `String` | `String()` for constant enums, in the same style as Go's `stringer` tool. Constants that share a value use the first name in declaration order, as `stringer` does |
| `Equal` | `Equal(other T) bool`, deep equality (slices and maps are compared too) |
| `Clone` | `Clone() T`, a deep copy (reference cycles are not supported) |
| `JSON` | for **structs** only: `MarshalJSON` and `UnmarshalJSON` from the struct tags (`json:"name,omitempty"`), without reflection. `encoding/json` picks them up. On a non-struct type it is an error |

`Clone` and `JSON` are the first derives to move after v1 if time is short
([Phase 10 §2.2](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros)).

### Rules (v1)

The compiler checks each of these rules and gives a clear error:
- `macro` declarations only at package level.
- A `lazy` parameter type must be concrete and spellable at every use: a predeclared or an
  exported type, not `lazy T` and not an unexported type of another package.
- A macro is only used with `#`. A plain call `util.Dbg(x)` or a macro used as a value
  (`f := util.Dbg`) is an error.
- A macro that returns `func()` can't be a statement inside code (`#util.Locked(&mu)` on its own
  line would drop the unlock). Use it on top of a function, or call the result.
- `#body()` only inside macros, and a macro that uses it only on top of a function.
- A macro can't `return` or `break` out of the code that uses it (use `!` for errors).
- On types, only the built-in `#Derive` exists in v1.

Also good to know:
- A `defer` inside a macro body runs when the **macro** returns, not when the caller returns (the
  macro is a function). To run code when the caller ends, return a `func()` and use the macro on
  top of the function.
- C macros stay usable for C APIs, through their header packages (`raylib.RAYWHITE`).

### How macros are compiled

- Every macro becomes a function with a hidden first parameter that holds the place of the use
  (one static object per use, so it costs nothing at run time). A `lazy T` parameter becomes a
  `func() T`, and a wrap macro gets the function's code as a hidden `func()` parameter.
- `#util.Dbg(a * 2)` becomes a call `util.Dbg(<place>, a*2)`. A start-style macro becomes the
  first statement of the function. A wrap-style macro becomes
  `util.VM(<place>, func() { <the function's code> })`.
- In C++, macros are inline templates in the package header. The C++ compiler inlines them, also
  across packages, and lazy arguments and `#body()` don't allocate.
- `gocpp expand file.gp` shows the code after expansion.
- **Phase:** [10](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros).

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
