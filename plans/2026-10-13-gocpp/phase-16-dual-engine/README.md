# Phase 16: Dual engine (Go packages on a garbage collector, next to gp's reference counting)

> Import Go modules such as gin from gp code, unchanged. gocpp translates their `.go` files to
> C++ the way it translates `.gp` files, but for a **second engine** that has Go's own memory
> layout and a **tracing garbage collector** (Boehm GC first). gp code, C and C++ stay on
> today's engine: reference counting, no collector. Both engines live in one program, and
> values are converted where they cross. These are your decisions of 2026-10-21.
> **This is a plan only. Nothing is scheduled, and the phase starts when you decide.**

| | |
|---|---|
| Depends on | Phase 15 (v1.0 ships first). Builds on 06 (modules, cache, `gocpp get`), 07 (generics), 08 (threads, channels, `select`, timers), 09 (stdlib, `gocpp test`), 14 (Windows), 15.2 (escape analysis), 15.14 (MVS over `go.mod`) |
| Unlocks | Go modules from gp code without porting them; Go's own `net/http` and `crypto/tls` for gp programs |
| Milestones | **M10: Go libraries pass their own tests** · **M11: `net/http`, TLS and gin on Linux and Windows** |
| Size | very large: about 41–60 weeks for one person (the parts are sized in §3) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

### 1.1 Your decisions (2026-10-21)

| Question | Decision |
|---|---|
| How Go code is built | **Translated to C++ by gocpp**, like gp code (option B2 of the first draft). The Go toolchain is not needed. Building with the real `go` (option B1) stays only an idea for packages that the translator cannot handle. |
| Memory | **Two engines in one program.** gp code stays on reference counting, also when Go packages sit next to it. Go packages, and a second copy of the standard library for them, run on a garbage collector. Values are converted at the boundary. |
| Collector | **Boehm GC (bdwgc) first**, reached only through a small `gp::gc` interface, so that an own collector can replace it later. |
| Order | **Libraries first, then gin.** M10: pure-Go libraries pass their own tests. M11: `net/http`, `crypto/tls` and the netpoller, then gin, on Linux and Windows. |
| Start | When you decide. This page is the plan. |

### 1.2 Why Go code needs its own engine

gocpp has one engine today: automatic reference counting (ADR-004, decision Q1). It is right for
gp code and wrong for code that was written for Go:

- **Cycles.** Go code builds them all the time and lets the collector free them. gin's `New()`
  does `engine.engine = engine` (gin.go:228): the engine points to itself through its embedded
  `RouterGroup`, and every `Context` points back to the engine. With reference counting, none of
  that memory is ever freed. ADR-103 lists the shapes.
- **Layout.** gp's values are bigger than Go's: `*T` is 16 B, `string` 32 B, `[]T` 40 B and an
  interface 32 B, against Go's 8, 16, 24 and 16. `unsafe` code that knows Go's layout breaks. 6 of
  the 17 surveyed modules convert `string` ↔ `[]byte` through `unsafe` (gin's
  `internal/bytesconv` has no safe variant), and protobuf reads the layout of interfaces.
- **Exactness.** Go packages expect Go's standard library as it is. They use `reflect`
  (`MapOf`, `ArrayOf`, `MethodByName` + `Call`, `MakeChan`, `Select`: 8 of 17 modules),
  finalizers (`os.File` and `net.netFD` close their descriptors with `SetFinalizer`),
  `runtime.AddCleanup`, and `unique` (`net/netip.Addr` holds a `unique.Handle`). gp's standard
  library is a careful port, not Go's: it differs where ARC needs it to.
- **Porting does not scale.** Each package of gp's standard library was ported by hand
  (Phase 09). gin's closure is 101 non-standard packages (300k lines) and 203 standard ones
  (375k lines), and gp's standard library lacks 122 of those 203.

A collector gives Go code what it was written for. Keeping ARC for gp keeps what gp is for: it
is deterministic, fast, and easy to mix with C++.

### 1.3 The picture

```
   .gp files, C and C++                           .go files (Go modules, Go's std)
          │                                                     │
          ▼                                                     ▼
 ┌─ ARC engine (today) ─────────────┐           ┌─ GC engine (new) ───────────────────┐
 │ gp::ptr<T>, gp::string (32 B), … │           │ T*, gp::gc::string (16 B), …        │
 │ namespaces gpkg_<path>           │ ◄───────► │ namespaces gopkg_<path>             │
 │ gp's standard library (ported)   │  boundary │ Go's standard library (Go's source) │
 │ freed when the last count drops  │           │ freed by the collector (bdwgc)      │
 └──────────────────────────────────┘           └─────────────────────────────────────┘
    shared by both: one OS thread per goroutine, channels, select, timers, panics as C++
    exceptions, the C++ compiler and linker, one executable
```

### 1.4 What does not change

- gp code: the same semantics, the same C++, the same speed. A program that imports no Go
  package does not link the collector and pays nothing for it.
- Goroutines stay OS threads in both engines, with no scheduler (Q4, ADR-104).
- One toolchain: gocpp and a C++ compiler. No Go installation is needed.

### 1.5 Not goals

- Replacing `go build` for Go applications. A Go `main` package does build (that is how a Go
  package's tests run), but the target is gp programs that use Go packages.
- cgo in Go packages. None of the 17 surveyed modules uses it outside the standard library, and
  building with `CGO_ENABLED=0` loses nothing. Also not: assembly without a pure-Go fallback,
  `//go:linkname` into runtime internals beyond the list of §2.7, and `plugin`.
- Modules that are mostly machine-translated unsafe code: `modernc.org/sqlite` is 7.5 % plain Go,
  keeps a C heap in `mmap`ed memory and has 1,875 assembly wrappers. Use SQLite's C library
  through interop instead.
- One memory layout for both engines. Each engine keeps its own, and values convert.
- Millions of goroutines or connections (ADR-104).
- In the first version, static and shared libraries that contain the GC engine. Executables and
  test binaries come first.

### 1.6 Alternatives looked at

| Alternative | Why not |
|---|---|
| Go packages on today's ARC engine | cycles leak, `unsafe` layout tricks break, every package needs porting (§1.2) |
| The whole program on the collector, gp included | your decision: gp stays deterministic, on reference counting, also next to Go packages |
| The real Go toolchain (`go build -buildmode=c-archive`) behind a cgo-style bridge (B1) | two runtimes and two schedulers in one process, a cgo crossing per call, and Go must be installed; kept only as an idea for packages the translator cannot handle |
| An own collector from the start | bdwgc already works with one OS thread per goroutine, conservative stacks and Windows (§7.2); the `gp::gc` interface keeps the door open |

## 2. How it works

### 2.1 Example

```go
// main.gp: gp code (reference counting) that uses gin (a Go module, garbage collected)
package main

import (
	"fmt"      // gp's fmt
	"net/http" // gp's standard library has no net/http, so this is the GC engine's (Go's own)

	"github.com/gin-gonic/gin" // a Go module: its .go files are translated for the GC engine
)

type Login struct {
	User string `json:"user" binding:"required"`
	Pass string `json:"pass"`
}

func main() {
	r := gin.Default()                      // *gin.Engine: an object in the GC heap
	r.POST("/login", func(c *gin.Context) { // the gp closure becomes a Go func value
		var req Login                                  // a gp struct, reference counted
		if err := c.ShouldBindJSON(&req); err != nil { // Go fills a copy, which is copied back
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fmt.Println("login:", req.User)
		c.JSON(http.StatusOK, gin.H{"hello": req.User}) // gin.H is built in the GC heap
	})
	r.Run(":8080")
}
```

A sketch of `main.gp`'s C++, in the ARC engine. The exact shapes are decided in steps 16.8 and
16.9. `gin::` stands for `::gopkg_github_dcom__gin_hgonic__gin`, the GC engine's namespace for
the module. gp packages keep their `gpkg_` names.

```cpp
gp::ptr<gin::Engine> r = gp::bridge::hold(gin::Default()); // Engine* → gp::ptr: its control
                                                           // block is a GC root, so the engine
                                                           // lives as long as r does
gin::HandlerFunc h = gp::bridge::go_func<gin::HandlerFunc>( // a GC closure that calls the gp
    [](gin::Context* c) {                                   // lambda; when it is collected, a
        gpkg_main::Login req{};                             // cleanup drops the lambda's count
        auto* m = gp::bridge::mirror_in(req);      // a GC copy of req: Go layout, same tags
        gp::gc::iface err = gin::Context::ShouldBindJSON(c, gp::gc::to_any(m));
        gp::bridge::mirror_out(m, req);            // fields copied back, also on a panic
        // ...
    });
gin::RouterGroup::POST(&r->RouterGroup, gp::gc::lit("/login"), gp::gc::slice_of({h}));
```

Inside a Go package, the GC engine writes Go's layout and plain pointers. A small Go function and
both of its translations:

```go
type node struct {
	next, prev *node // a cycle: n.next.prev == n
	name       string
}

func push(head *node, name string) *node {
	n := &node{next: head, name: name}
	head.prev = n
	return n
}
```

```cpp
// ARC engine (what gocpp writes today, for gp code)
struct node { gp::ptr<node> next; gp::ptr<node> prev; gp::string name; };  // 16 + 16 + 32 = 64 B
gp::ptr<node> push(gp::ptr<node> head, gp::string name) {
    gp::ptr<node> n = gp::new_<node>(node{.next = head, .name = name});
    head->prev = n;   // a nil check and a count: head and n now keep each other alive (a leak)
    return n;
}

// GC engine (for Go code)
struct node { node* next; node* prev; gp::gc::string name; };  // 8 + 8 + 16 = 32 B, Go's layout
node* push(node* head, gp::gc::string name) {
    node* n = gp::gc::new_<node>(node{.next = head, .name = name});  // allocated with node's type
    gp::gc::deref(head)->prev = n;   // a nil check, then a plain store: no count, no barrier
    return n;
}
```

### 2.2 The two engines side by side

| | ARC engine (today) | GC engine (new) |
|---|---|---|
| Builds | `.gp` packages and gp's standard library; calls C and C++ | `.go` packages of Go modules and Go's standard library |
| Frees memory | when the last reference goes (`std::shared_ptr`) | when the collector finds it unreachable (bdwgc) |
| `*T` | `gp::ptr<T>`, 16 B | `T*`, 8 B |
| `string`, `[]T`, interface, `func`, `map`, `chan` | 32, 40, 32, 24, 16, 16 B | 16, 24, 16, 8, 8, 8 B (Go's) |
| `unsafe` | `Sizeof`, `Offsetof`, `Alignof`, no layout tricks | Go's rules: `unsafe.Pointer` arithmetic, `unsafe.String`, `unsafe.Slice`, header casts |
| `reflect` | gocpp's subset (ADR-106) | Go's own `reflect`, translated (§2.8) |
| Cycles | leak unless broken with `weak.Pointer` | collected |
| Finalizers, `AddCleanup` | `SetFinalizer` is accepted and never runs; no `AddCleanup` | Go's semantics (§2.4) |
| Type descriptors | gocpp's (`type_desc`, Phase 05) | the layout of Go's `internal/abi.Type` (§2.8) |
| C++ names | `gp::`, `gpkg_<path>` | `gp::gc::`, `gopkg_<path>` |
| Threads, channels, `select`, timers, panics, the deadlock detector | one implementation, shared by both engines | |

### 2.3 Which engine builds a package

1. A package of `.gp` files is built by the ARC engine, a package of `.go` files by the GC
   engine. A folder with both kinds of files is an error.
2. A module with a `gocpp.mod` is a gp module. A module with only a `go.mod` is a Go module, and
   all its packages are Go packages. Inside a gp module, a folder of `.go` files is a Go package
   (for example generated protobuf code), and its imports resolve in the GC engine's world with
   the module's requirements.
3. Imports in a `.go` file always resolve in the GC engine's world: Go's standard library (Go's
   source) or Go modules. Go code cannot import gp packages or C/C++ headers. gp code reaches Go
   code only through the values that it passes in: functions, interface values and pointers.
4. Imports in a `.gp` file use gp's standard library first (`strings`, `encoding/json`, `time`).
   A standard package that gp's library lacks (`net/http`, `crypto/tls`) comes from the GC
   engine. A module path comes from the module that provides it, gp or Go.
5. So a standard package can exist twice in one program (`gpkg_strings` and `gopkg_strings`):
   two namespaces, no clash, but twice the code (§2.10 has the sizes).
6. A program without a Go package is exactly today's program: no collector and no hooks.

Whether gp code may ask for the GC engine's copy of a package that gp also has
(`import gojson "go:encoding/json"`) is a question for the ADR of step 16.2.

### 2.4 The collector: `gp::gc` over bdwgc

**The library.** bdwgc 8.2.12 (2026-02-05, an MIT-style license, the notice ships with
gocpp's other notices). It is vendored as source in `runtime/third_party/bdwgc` and built by
gocpp, with fixed flags, into the runtime cache, once per toolchain and only for programs that
have Go packages. The same build serves Linux and Windows (MinGW-w64, llvm-mingw). The flags:

- `GC_THREADS`, `PARALLEL_MARK` (up to 16 marker threads) and `THREAD_LOCAL_ALLOC`. Without the
  last one, allocation-heavy threads in the V language ran 16 times slower (vlang/v#27486).
- `GC_BUILTIN_ATOMIC`, so the GPL part of `libatomic_ops` is never linked.
- `LARGE_CONFIG`, for heaps above about 4 GB.
- `ALL_INTERIOR_POINTERS` with `DONT_ADD_BYTE_AT_END`. Go code keeps interior pointers
  (`&s.f`, a slice into an array). The collector does not need a byte of padding after each
  object because the GC engine never makes a past-the-end pointer: a slice expression whose
  capacity becomes 0 keeps the base pointer, as gc does (golang.org/issue/8404).
- Incremental mode off. TinyGo does the same, and bdwgc cannot mark in parallel and
  incrementally at the same time.

**What is scanned.**

- Stacks, registers and static data: conservatively, always. Generated C++ has no stack maps,
  and gccgo scanned its stacks the same way for years.
- The GC heap, in two stages. Stage 1 (M10): pointer-free objects (the bytes of a string, a
  `[]byte`, a struct of numbers) use `GC_malloc_atomic` and are never scanned; other objects
  are scanned conservatively. Stage 2 (step 16.20, if the numbers ask for it): the heap is
  precise. Each type's pointer bitmap (Go's `GCData`, which the type descriptors carry anyway
  for `reflect`) becomes a bdwgc descriptor, so a number that looks like a pointer no longer
  keeps an object alive. TinyGo 0.43 made the same change (2026-10-09).
- **Not the ARC heap.** Memory from `malloc` and `new` (gp objects, C++ containers) is never
  scanned, so a GC pointer that is kept there must be a root (§2.6).
- **Not thread-local storage.** LLGo hit this: a defer chain that only pthread TLS held was
  freed. The runtime keeps a goroutine's GC pointers in a rooted per-thread record instead. The
  same goes for the value of a panic in flight, because C++ exception objects are `malloc`ed.

**Go's API on top.**

| Go | GC engine |
|---|---|
| `runtime.GC()` | `GC_gcollect` |
| `GOGC`, `debug.SetGCPercent` | `GC_set_free_space_divisor` (bdwgc's heap growth knob); `off` disables collection |
| `GOMEMLIMIT`, `debug.SetMemoryLimit` | a soft limit, emulated: a collection when the heap passes it. bdwgc's hard cap (`GC_set_max_heap_size`) is not used, because it makes allocation fail |
| `runtime.SetFinalizer` | `GC_register_finalizer`, which has Go's order: if A points to B, A is finalized first, and a cycle with finalizers is never finalized. Finalizers run on demand, on **one finalizer goroutine**, as in Go |
| `runtime.AddCleanup` | a list of cleanups per object plus one unordered finalizer on its base: several cleanups per object, interior pointers allowed, cycles do not leak |
| `weak.Pointer`, `unique` | short disappearing links, which are cleared when the object first becomes unreachable (Go's rule), plus a table that makes two weak pointers to the same address compare equal |
| `runtime.ReadMemStats`, `debug.FreeOSMemory` | bdwgc's heap statistics; `GC_gcollect_and_unmap` |
| `runtime.KeepAlive`, `runtime.Pinner` | free, because the collector never moves objects; `Pinner` makes a root |

**The interface.** `gp::gc::heap` is small: allocate with a type, allocate pointer-free, enter
and leave a thread, run without the collector (a blocking section), make and drop roots,
finalizers, cleanups, weak links, collect, statistics and limits. Its shape follows Whippet's
API, so the backend can become an own collector, a port of Go's collector, or Whippet (MIT, but
without Windows support today) without touching the engine. Nothing else calls bdwgc.

### 2.5 Threads and stopping the world

- Goroutines stay OS threads in both engines. bdwgc stops the world with signals on Linux (one
  signal per running thread, then a handshake) and with `SuspendThread` on Windows.
- Every goroutine thread registers with the collector when it starts (`GC_register_my_thread`,
  after `GC_allow_register_threads` in `run_main`), in gocpp's thread start
  (`runtime/src/goroutine.cpp`). The registration is explicit, not bdwgc's macros that redirect
  `pthread_create`, so the same code covers threads that a C or C++ library starts, and Windows.
- **Every wait is a blocking section** (`GC_do_blocking`): parking on `gp::waiter` (channels,
  `select`, `sync`), `Sleep`, an idle reused thread, blocking system calls
  (`runtime/src/os/linux/posix_file.cpp`, `runtime/src/os/windows/win_syscall.cpp`), timed waits
  and the netpoller. A blocked thread is not signalled, and its stack is still scanned from the
  point where it blocked. So a stop costs one signal per *running* thread: 10,000 connection
  threads that wait on the network cost only the scan of their stacks.
- A thread that gocpp did not start but that runs Go code (a callback from a C library)
  registers on entry, in the same place where the deadlock detector notices foreign threads.
- Signals: bdwgc's defaults are `SIGPWR` and `SIGXCPU`. gocpp chooses two real-time signals
  before `GC_INIT` and documents them. gocpp's own system calls already retry on `EINTR`. C
  libraries that do not retry meet the same problem as under Go's preemption signal (since
  Go 1.14).
- A goroutine in a blocking section counts for the deadlock detector exactly as today. A
  goroutine that waits in the netpoller is not asleep: the network can wake it, as in Go.

### 2.6 The boundary: what crosses and how

Three rules:

1. **A value has the engine of its type.** Types declared in Go packages (`gin.Engine`,
   `http.Request`) belong to the GC engine, types declared in gp packages to the ARC engine.
   Unnamed and built-in types (`string`, `[]byte`, `map[string]any`, `error`) take the engine of
   the code that holds them, and are converted when they cross.
2. **gp code sees one standard library.** When gp code looks at a Go package's API, the types
   from the standard packages that gp also has (`time.Time`, `context.Context`, `io.Reader`)
   appear as gp's own. gocpp type-checks the Go package's declarations a second time
   against gp's standard library, with function bodies skipped; this is the package's **gp
   view**. The generated boundary converts between the two types at each crossing. Types that
   only the GC engine has (`*http.Request`) stay Go types.
3. **The collector never scans the ARC heap, and the ARC engine never counts GC objects.** A GC
   pointer kept in ARC memory is pinned by a root. An ARC object kept by the GC engine is held
   by a proxy, whose cleanup drops the count.

| Kind | gp → Go (an argument, a field write) | Go → gp (a result, a field read) |
|---|---|---|
| numbers, `bool`, arrays and structs of them | a copy (the same bits) | a copy |
| `string` | borrowed for the call when the Go parameter does not escape (gocpp's escape analysis, ADR-114, runs on the Go code), otherwise copied into the GC heap; string literals are never copied | no copy: the gp string's owner is a root on the GC object |
| `[]byte` and other slices of numbers | borrowed for the call, so Go's writes are seen by gp, as in Go; copied when the parameter escapes (later writes are then not shared: a documented difference) | no copy: the owner is a root, and both sides share the array, as in Go |
| slices, arrays and maps of other elements | converted element by element (a copy) | the same |
| a pointer to a Go object (`*gin.Engine`) | the raw pointer | a `gp::ptr<T>` whose control block is a root (`gp::bridge::hold`); `==` compares addresses, as in Go |
| a pointer to a gp value (`&req`) | **copy in, copy out**: for the call, a GC copy in Go layout (a *mirror* type with the same fields and tags), copied back after it, also on a panic. If the Go function keeps the pointer (the parameter escapes, and the analysis cannot show that it is only used during the call), gocpp refuses the call and names the fix: allocate the value in the GC engine (a helper such as `gc.New[T]()`; its name is decided in step 16.2) | (not applicable) |
| an interface that holds a gp value | a proxy in the GC heap that holds a count on the gp value and forwards each call, converting the arguments | a gp proxy that holds a root on the Go value |
| `error` | a proxy; `errors.Is` and `errors.As` see through proxies; the standard sentinels (`io.EOF`, `context.Canceled`, `fs.ErrNotExist`, …) map to their twins, so `err == io.EOF` holds on both sides | the same |
| `func` values (`gin.HandlerFunc`) | a GC closure that calls the gp closure (`gp::bridge::go_func`); its cleanup drops the gp closure | a gp closure that holds a root on the Go closure |
| `context.Context` | an adapter: a Go context that the gp one cancels through `context.AfterFunc`, with no extra thread | the same, the other way |
| `time.Time`, named numbers (`time.Duration`, `fs.FileMode`) | a field copy; the `UTC` and `Local` locations by identity, others by name | the same |
| channels | not in the first version: a compile error that suggests a forwarding goroutine | |
| generic Go functions and types | instantiated with GC-engine types or convertible ones; gp types as type arguments are not in the first version | |
| panics | one C++ exception type for both engines, so a panic unwinds through frames of both; the panic value converts when it is recovered on the other side | |

More rules:

- **Proxies are canonical**: one proxy per object while the object lives (a weak table). So `==`
  keeps Go's meaning: `err == http.ErrServerClosed` holds in gp code when Go returned that error.
- **Mirror types are written as Go source** by the boundary generator (the gp struct's fields
  and tags, in the GC engine's world under the gp package's own path) and translated by the GC
  engine like any Go package. Go's `reflect`, `encoding/json` and validators see a normal Go
  struct, and their messages name `Login.pass` as Go's would. In the first version, a mirror has
  no methods: Go's `json` does not see a gp type's `MarshalJSON`.
- **A Go value stored in ARC memory** (a field of a gp struct, a captured variable, an element of
  a gp slice or of a C++ container) is a root. A struct of a Go type stored by value is boxed in
  the GC heap first. A Go pointer in a gp local that does not escape stays a plain pointer,
  because stacks are scanned.
- **gp's `fmt` and `encoding/json` hand Go values to Go's.** `%v` of a `*gin.Context` prints what
  Go prints. Other users of gp's `reflect` see a Go value as opaque: its type name, no fields.
- **Cycles across the boundary** (a gp object → a root → a Go object → a proxy → the same gp
  object) are collected by neither engine. They leak like ARC cycles today, and `gocpp vet` and
  the cycle report of ADR-103 name the shapes.
- **Cost**: a call from gp to Go with numbers is a plain C++ call, which the compiler can inline.
  Each converted string, slice or struct costs a copy, and a root costs one small allocation
  and an atomic count. Step 16.1 measures each shape, and §5 has the budget.

### 2.7 Go's standard library on the GC engine

**Source.** Go's own `src/` at the pinned Go version (go1.25 today; ADR-110 moves it), never gp's
ports, because Go packages expect Go's exact behavior. gocpp downloads the pinned source archive
once (its checksum is in gocpp) or uses a local Go tree named by `GOCPP_GOROOT`. Nothing needs the
`go` command.

**Build constraints.** The real `GOOS` and `GOARCH` (`linux` or `windows`, `amd64` or `arm64`),
the compiler tag `gocpp` (neither `gc` nor `gccgo`), and the tags `purego` and
`math_big_pure_go`. `.s` files are never used. With these tags, the crypto packages, `math/big`
and the vendored `x/crypto` drop their assembly.

**Three kinds of replacement**, all kept in the gocpp repository under `gostd/`:

- **The `runtime` package is gocpp's own**: Go source for the public API (`GC`, `SetFinalizer`,
  `AddCleanup`, `Caller`, `Callers`, `Stack`, `NumGoroutine`, `GOMAXPROCS`, `LockOSThread`,
  `ReadMemStats`, `Error`, …) over natives. Go's `runtime` (71.7k lines, 139 assembly functions)
  is never translated. It also provides the internal functions that Go's runtime marks as
  pulled by well-known modules ("Do not remove or change the type signature",
  go.dev/issue/67401: `mapaccess1`, `mapassign`, `mallocgc`, `growslice`, `typedmemmove`, …),
  where they make sense without Go's scheduler. The goal is that `ugorji/go/codec` (gin's
  msgpack binding) builds without `-tags codec.safe`. Sonic's JIT and anything that needs Go's
  write barrier never will.
- **The hook table**: the runtime functions that the other standard packages pull with
  `//go:linkname`. The `net/http` + `crypto/tls` closure reaches 166 distinct ones: `sync`'s
  semaphores and `notifyList`, `internal/poll`'s nine `runtime_poll*` functions, `time`'s
  timers and `now`, `syscall`'s `entersyscall`, 37 for `reflect`, `iter`'s coroutines, `weak`,
  `unique`, and more. Each one is a C++ native over the shared runtime: threads, `gp::waiter`,
  the timer thread and the semaphore table of Phase 08.
- **Overlays**, GopherJS's "natives" idea: replacement `.go` files for the packages whose
  assembly has no pure-Go fallback. They are `sync/atomic` and `internal/runtime/atomic` (C++
  atomics), `internal/bytealg`, `internal/cpu`, `math`, `hash/crc32`, `syscall` (the raw
  `Syscall` entry points: `syscall(2)` on Linux, `SyscallN` on Windows), `reflect`'s two
  assembly stubs and `internal/chacha8rand`. A tool checks the list against Go's source: every
  bodyless function must have a native or an overlay. After a Go upgrade, it shows what is new.

**Module overlays.** A few very common modules need the same treatment. `golang.org/x/sys`
(`unix`, `windows`, `cpu`) has assembly `Syscall` entry points and about 228 system-call
arguments that turn a pointer into a `uintptr`. Those arguments are fine here, because the stack
scan keeps the objects alive. An overlay is chosen by module path and version range.

**`//go:linkname` in Go packages.** A pull of a symbol that exists in translated code (`x/net`
pulls `syscall.getsockopt`) or in the runtime is resolved to that symbol's C++ name. Anything
else is an error that names the symbol.

### 2.8 `reflect`, maps and interfaces

- **Type descriptors** have the layout of Go's `internal/abi.Type`: size, `PtrBytes`, hash, kind,
  alignment, the equality function, `GCData`, the name, `PtrToThis`, and the parts for each kind
  (`abi.StructType`, `abi.SwissMapType`, …). The offsets that Go resolves through module data
  (`NameOff`, `TypeOff`, `TextOff`) are indexes into tables of the program, resolved by hooks.
- With them, Go's own `reflect`, `internal/reflectlite`, `internal/abi` and
  `internal/runtime/maps` are translated rather than rewritten, and `MapOf`, `ArrayOf`,
  `SliceOf`, `StructOf`, `MakeChan`, `Select`, `Send`, `Recv`, `MethodByName` and `NewAt` (the
  survey's gaps) come with them.
- **Calls through `reflect`** (`Value.Call`, `Method`, `MakeFunc`) use assembly in Go
  (`reflectcall`, `makeFuncStub`). The GC engine generates one C++ call thunk for each function
  signature that the program has: the method sets of types that reach an interface or
  `reflect`, and the function types. This is ADR-106's idea, for the whole engine. A signature
  that was built at run time with `FuncOf` has no thunk, and calling it panics with a clear
  message. None of the 17 surveyed modules calls `MakeFunc`.
- **Maps** are Go's own Swiss tables (`internal/runtime/maps`, Go 1.24+), hashed with the portable
  `memhash` and `strhash` (no AES instructions needed), in random order, as in Go. The generated
  code calls them as gc's code does (`mapaccess1_faststr`, …).
- **Interfaces** are `{itab, data}` and `{type, data}`, as in Go. itabs for the conversions that
  the compiler sees are generated, and the others are built at run time and cached, as gc's
  `getitab` does.
- **Escape analysis** (ADR-114) decides between the stack and the GC heap for address-taken
  locals and closures, as in Go.

### 2.9 The netpoller (M11)

- Go's `internal/poll` and `net` are translated as they are. They expect non-blocking descriptors
  and the runtime's `runtime_poll*` functions.
- gocpp's netpoller is Go's design, with threads instead of goroutine records: one poller thread
  per process (`epoll` on Linux, an I/O completion port on Windows, as in Go's
  `netpoll_epoll.go` and `netpoll_windows.go`). `pollWait` parks the calling goroutine's
  `gp::waiter` in a blocking section, and the poller thread wakes exactly the waiter of a ready
  descriptor. There is no thundering herd: LLGo's per-thread `poll()` on one shared wake pipe
  wakes every waiter. Deadlines are timers of the shared timer thread that wake the waiter with
  a timeout. Closing a descriptor and changing a deadline wake waiters as Go's code does.
- On Windows: overlapped I/O through `internal/poll`'s Windows code, with the completions taken
  from `GetQueuedCompletionStatusEx` on the poller thread.
- DNS: Go's pure-Go resolver (no cgo resolver).
- Scale: one thread per connection (ADR-104). Tens of thousands of connections are fine. With
  Linux's default `vm.max_map_count` (65,530) and a kernel before 6.13, a program stops at about
  20,000 to 30,000 threads (Phase 08's measurement).

### 2.10 Build, cache and size

- The engine and the collector's flags are part of the key of every cached object. A Go package
  is translated once for each version and set of flags, and cached by content hash like a gp
  package.
- A first build of a gin program translates and compiles about 675k lines of Go: gin's 300k
  non-standard lines in 101 packages and 375k standard lines in 203 packages (about 190k + 375k
  with `-tags nomsgpack`). Expect a cold build in minutes, not seconds. Step 16.1 measures it, §5
  has the budget, and later builds use the cache. The GC engine's standard library can be
  prebuilt per toolchain.
- Release builds use `-O2 -flto -g0` and `-strip` (ADR-136), plus `--gc-sections`, so that the
  unused functions of big packages are dropped.

### 2.11 Tools

- **Errors in Go packages are gocpp's**, since a Go package already builds with `go`. There is
  one report per package with everything that blocks it (assembly without a fallback, cgo, an
  unknown `//go:linkname`, a missing native) and the fix if there is one (`-tags purego`).
- **`gocpp test`** runs a Go package's own `_test.go` tests: internal and external test
  packages, `TestMain`, and examples with `// Output:`, with the GC engine's `testing`. This
  is how M10 is proved.
- **`go.mod` and `go.sum`.** Go modules come from `GOPROXY` (`proxy.golang.org` by default,
  `direct` through git) and are checked against the checksum database (`sum.golang.org`).
  `GOPRIVATE`, `GONOSUMDB`, `GONOPROXY` and `GOINSECURE` mean what they mean for Go. Versions
  are chosen by MVS (step 15.14 already reads `go.mod`). A module whose `go` line is newer than
  gocpp's Go version is refused, and the message names both versions. Loop variables follow the
  module's `go` line: one per iteration from go1.22 on, one per loop before.
- **`gocpp vet`** warns at the boundary: a copy that may surprise, a cross-engine cycle shape.
  `gocpp explain` and `emit -annotate` show each conversion and its cost.
- **Editor**: hover, completion and go-to-definition on Go packages from gp files, through the
  gp view; a definition opens the `.go` file in the module cache. `#line` for `.go` files, and
  debugger printers for the GC engine's strings, slices, maps and interfaces.

## 3. Steps

The parts are sized for one person working full-time, like the other phases. Windows is not a
separate step: every runtime step runs on the real Windows PC (`GOCPP_WINDOWS_SSH`), and both
milestones need Windows.

### Part A: decide (3–4 weeks)

1. **16.1 Spike: one Go package on bdwgc, called from gp** (2–3 weeks).
   - Build bdwgc 8.2.12 from source with the flags of §2.4 on Linux (GCC, Clang) and Windows
     (llvm-mingw, on the real PC).
   - A throwaway GC mode in the emitter, behind a hidden flag, good enough for
     `container/list`, `strings`, `strconv`, `sort` and a binarytrees program: Go-layout values,
     `gp::gc::new_`, maps in GC memory, closures, interfaces, goroutines.
   - Five boundary shapes by hand: a rooted `gp::ptr`, a string both ways, a borrowed `[]byte`,
     copy-in/copy-out of a struct, a gp closure as a Go func value.
   - Measure: allocation speed and peak memory of binarytrees (GC engine, ARC engine, Go); the
     stop-the-world pause with 1,000 and 10,000 goroutines, blocked and running; the cost of
     each boundary shape; compile time per 1,000 lines; the size that the collector adds;
     whether ASan and TSan work with bdwgc.
   - A dry run of Go's standard library: type-check `reflect`, `internal/runtime/maps`, `sync`,
     `time`, `os`, `net` and `net/http` under gocpp's constraints, and list every bodyless
     function, `//go:linkname` and unsupported construct. That list is the input of steps 16.11
     to 16.13.
2. **16.2 ADR: the design** (1 week). From the spike's numbers, decide the open points:
   - the names and the namespaces of §2.2;
   - the type descriptors: `abi.Type` layout (so Go's `reflect` is translated) or gocpp's own
     descriptors with its own `reflect`;
   - stage 1 of the heap scan (§2.4), and when stage 2 is needed;
   - the import rules of §2.3, and whether `"go:..."` imports exist;
   - the gp view and the list of twin types; the boundary table of §2.6; copy-in/copy-out and
     the escape refusal, checked on `ShouldBindJSON` and `json.Unmarshal`; canonical proxies;
     mirrors without methods; channels and generics out of the first version;
   - how Go's source is delivered (download, `GOCPP_GOROOT`, or embedded);
   - the overlay and `//go:linkname` policies; whether `-race` and `-asan` cover the GC engine;
   - the budgets of §5;
   - what changes in ADR-004 (extended: ARC stays for gp), ADR-103 (the GC engine collects Go's
     cycles), ADR-106 (the GC engine has Go's `reflect`), ADR-107 (`net/http` reaches gp through
     the GC engine instead of a C++ wrapper module), ADR-109 (the checksum database is needed
     now) and ADR-110 (Go modules raise their `go` line, so the yearly move matters more).

### Part B: the engine's runtime (7–10 weeks)

3. **16.3 `gp::gc` and bdwgc** (2–3 weeks): the vendored library with its notice, built into the
   runtime cache; `gp::gc::heap` (§2.4) with roots, finalizers on demand and the finalizer
   goroutine, cleanups, weak links, statistics and limits. bdwgc keeps only one finalizer per
   object, so that one finalizer runs the Go finalizer and the object's cleanups. Tests in
   `runtime/test` on GCC, Clang and Windows: roots, the order of finalizers (Go's rules), several
   cleanups per object, cleanups on interior pointers, weak links cleared before cleanups run,
   statistics.
4. **16.4 Threads and stops** (2–3 weeks): registration in the thread start and for foreign
   threads; blocking sections around parking, `Sleep`, idle threads, system calls and timed
   waits (`waiter.cpp`, the Linux and Windows `thread.cpp`, `posix_file.cpp`,
   `win_syscall.cpp`); the choice of signals; rooted per-thread records instead of TLS; a rooted
   panic value. Tests: a thread blocked on a channel is never signalled (signals are counted);
   10,000 goroutines that allocate; a collection during a deep recursion and while a goroutine
   is in a C call; the deadlock detector unchanged; the same on the Windows PC.
5. **16.5 The GC engine's values** (2–3 weeks): `gp::gc::string`, `slice<T>`, `iface` and
   `eface`, `func` (one pointer to a closure object), `map` and `chan` pointers, arrays, and
   structs in Go's layout (`internal/check/sizes.go` gets the GC engine's sizes; `golayout.hpp`
   already has Go's growth numbers). `new_`, `make`, `append` with Go's growth, slicing without
   past-the-end pointers, strings, conversions, comparisons, and nil, bounds and division
   checks that panic like Go. Channels are `chan_core` objects in the GC heap, with an element
   table (size, move, zero, pointer bitmap); `select` is unchanged. Tests for each type, and
   Go's layout checked against `unsafe.Sizeof` and `Offsetof` from real Go (a generated table).
6. **16.6 Panics, `defer`, `recover` across the engines** (1 week): one exception type, the panic
   value rooted while it unwinds, `recover` on either side converts; runtime errors have Go's
   messages and `runtime.Error` types on the GC side; a goroutine's panic exits with code 2, as
   today.

### Part C: the compiler (9–12 weeks)

7. **16.7 Loading Go packages and modules** (2–3 weeks):
   - `.go` packages through the stock `go/parser` (Go files have no gp syntax); build
     constraints (`//go:build`, the old `// +build`, file name suffixes, the `gocpp` and
     `purego` tags, `-tags`); `_test.go` files; `//go:embed` (the gp loader's code); cgo files
     dropped as with `CGO_ENABLED=0`, or an error that names the package if nothing is left;
     `.s` files ignored; the directives (`//go:linkname`, `//go:noescape`, `//go:nosplit`, …).
   - The module rules of §2.3. Requirements by MVS (step 15.14), with `replace`, `exclude` and
     `retract`.
   - Fetching through the `GOPROXY` protocol and `direct` (git, as `gocpp get` already does),
     checked against `GOSUMDB`, with Go's meaning of `GOPRIVATE`, `GONOSUMDB`, `GONOPROXY` and
     `GOINSECURE`; the hashes go in `gocpp.sum`, as the same `h1:` lines as `go.sum`.
   - The `go` line: the language version of each module for go/types; loop variables by
     version; the refusal of a module that needs a newer Go than gocpp
     (`github.com/x/y@v1.2.0 requires go >= 1.26; gocpp is go1.25`).
   - Go's source for the standard library: the pinned archive, downloaded once into the cache,
     or `GOCPP_GOROOT`.
8. **16.8 GC-mode lowering and emission** (3–4 weeks):
   - The engine of a package flows through `internal/lower` and `internal/cppgen`: types
     (`cxxType`), allocation and boxing (the escape analysis decides stack or GC heap),
     closures (a GC environment), method values, interfaces and itabs, maps (calls into
     `internal/runtime/maps`), strings, `defer` and `recover` (the shared frames), `go`,
     channels, `select`, generics (C++ templates over GC types), range over functions, and
     `unsafe` with Go's rules (`Pointer` arithmetic, `Add`, `Slice`, `String`, `StringData`,
     `SliceData`, header casts).
   - What the ARC engine gates but Go code needs: generic types declared inside functions
     (hoisted with their type parameters), `//go:linkname` pulls (§2.7), all of `unsafe`. The
     gate table (`internal/check/gates.go`) gets an engine column.
   - Type descriptors in `abi.Type` layout, with `GCData`, names and method tables, and the
     tables behind `resolveNameOff`, `resolveTypeOff` and `resolveTextOff`.
   - `#line` for `.go` files; the names `gopkg_<escaped path>`.
   - Tests: Go's own `test/` programs that `tools/gotest` runs (step 15.10), built in GC mode
     as Go `main` packages, with gc's expected output.
9. **16.9 The gp view and the boundary** (3–4 weeks):
   - The gp view of every Go package that gp code imports: its declarations type-checked again
     against gp's standard library (function bodies skipped), with the twin types substituted.
     A declaration that does not check in the gp view gets a reason, reported only when gp code
     uses it.
   - Every row of §2.6, both ways, generated per call site and per type: roots
     (`gp::bridge::hold`), borrowed and copied strings and slices (driven by the escape analysis
     of the Go callee), mirrors (synthesized Go source), copy-in/copy-out with the escape check
     and its error, canonical proxies, errors and sentinels, functions, twins (`context`,
     `time`, the `io` interfaces, named numbers), panics; gp's `fmt` and `json` handing Go
     values to Go's.
   - A Go pointer in a non-escaping gp local stays a plain pointer; one stored in ARC memory
     becomes a root.
   - Tests: one run test per row and direction, an error test for every refused crossing, a
     cycle test documented as a leak, a stress test of roots from many threads.
10. **16.10 `gocpp test` for Go packages** (1 week): internal and external test packages,
    `TestMain`, examples with `// Output:`, benchmarks, fuzz seeds run as tests, `testdata/`,
    `-run`, `-v`, `-count`, `-short`, `-timeout`, `t.Parallel`, with the GC engine's
    `testing`; gp and Go packages in one run. A comparison mode for the conformance work runs
    the same package with `go test` and `gocpp test` and compares the results (a tool for
    16.14 and 16.15, not for users).

### Part D: Go's standard library on the GC engine (9–13 weeks)

11. **16.11 The replacement checker** (1 week): `gostd/` (overlays as `.go` files and C++
    natives, the runtime, module overlays) and `tools/gostdcheck`, which lists every bodyless
    function, runtime `//go:linkname` and assembly-only path in the pinned Go source under
    gocpp's constraints, and fails on one without a native or an overlay. It runs on every Go
    version move.
12. **16.12 `runtime` and the hook table** (3–4 weeks):
    - The GC engine's `runtime`: the public API of §2.4; `runtime/debug` (`SetGCPercent`,
      `SetMemoryLimit`, `FreeOSMemory`, `ReadBuildInfo` from the module graph that gocpp built,
      `Stack`, `PrintStack`); the functions that notable modules pull (§2.7).
    - Natives for the 166 runtime symbols: `sync` (semaphores, `notifyList`; `procPin` must give
      two threads that pin at the same time different slots, because `sync.Pool` relies on it;
      the pools are emptied at the start of each collection, as in Go), `time` (`now`; timers on
      the shared timer thread, held through weak links so that an unreferenced `Timer` is
      collected, Go 1.23's rule), `internal/poll` (stubs that say "not pollable" until 16.16),
      `syscall` (`entersyscall` and `exitsyscall` are blocking sections), `os` (arguments,
      environment, `runtime_beforeExit`), `internal/godebug`, `iter` (`newcoro` and
      `coroswitch` from step 7.10), `weak`, `unique`; `internal/synctest` is an error if used.
    - Overlays (§2.7), plus `os/signal` (signals from the shared runtime) and `os/exec`
      (spawning a process).
13. **16.13 `reflect`, maps and interfaces** (3–4 weeks): Go's `reflect`, `internal/reflectlite`,
    `internal/abi` and `internal/runtime/maps` translated; call thunks for `Value.Call`,
    `Method` and `MakeFunc` (an overlay for `reflectcall`, `makeFuncStub` and
    `methodValueCall`); a signature without a thunk panics with a clear message. Go's own tests
    of `reflect` and `internal/runtime/maps` pass, except a reviewed list.
14. **16.14 The standard packages that M10 needs** (2–4 weeks): Go's own tests (from Go's source)
    of `errors`, `fmt`, `strings`, `bytes`, `strconv`, `unicode/...`, `sort`, `slices`, `maps`,
    `container/...`, `math`, `math/big`, `encoding/...` (`json`, `base64`, `hex`, `binary`,
    `xml`), `bufio`, `io`, `io/fs`, `os`, `path/filepath`, `time`, `sync`, `context`, `regexp`,
    `text/template`, `go/token`, `embed`, `testing`, `flag`, `log`, `hash/...`,
    `crypto/{md5,sha1,sha256,rand}`, and `net` without sockets (`net/netip`, parsing addresses).
    Each failure is fixed or listed with a reason.

### Part E: M10 (2–4 weeks)

15. **16.15 M10: Go libraries pass their own tests.**
    - `gopkg.in/yaml.v3`, `github.com/BurntSushi/toml`, `golang.org/x/text` (`cases` and what it
      needs), `google.golang.org/protobuf` (`proto`, `protojson`, `prototext`),
      `github.com/google/uuid`, and `github.com/spf13/cobra` with `spf13/pflag`: their
      `_test.go` tests pass with `gocpp test` on Linux (GCC, Clang) and on the Windows PC,
      except a reviewed skip list (the network, the `go` command, timing).
    - A gp program uses each of them through the boundary: YAML and TOML into gp structs, a
      protobuf message, a cobra command tree whose flags are bound to GC-engine variables.
    - Build time, binary size and test time are recorded next to `go test`'s.

### Part F: M11 (9–13 weeks)

16. **16.16 The netpoller** (2–3 weeks): §2.9 on Linux (`epoll`) and Windows (IOCP), the real
    `internal/poll` hooks, deadlines, 10,000 idle connections, and the tests of Go's
    `internal/poll` and `net` that use local sockets.
17. **16.17 `net`, `crypto`, `crypto/tls`, `crypto/x509`** (3–4 weeks): `net` (TCP, UDP, Unix
    sockets, the pure-Go resolver, `net.Interfaces`); the crypto tree with `purego`, so the FIPS
    140 module builds as plain Go; `crypto/tls`; `crypto/x509` with the system's root
    certificates (files on Linux, the certificate store through `syscall` on Windows). Go's
    tests of these packages pass, except a reviewed list. TLS works with `curl`,
    `openssl s_client` and a Go-built peer.
18. **16.18 `net/http`** (2–3 weeks): server and client, HTTP/1.1 and HTTP/2 (Go's bundled
    `h2_bundle.go`), `httptest`, `httputil`, `mime/multipart`, `compress/gzip`. Go's `net/http`
    tests pass, except a reviewed list; a gp program serves and fetches.
19. **16.19 gin on Linux and Windows** (2–3 weeks): gin v1.12.0 (quic-go, `x/net` and `x/sys`
    through the module overlays) and v1.9.1; `-tags nomsgpack` documented as the smaller build;
    the program of §2.1; gin's own tests, except a reviewed list; `chi`, `gorilla/websocket` and
    `echo` as checks of the same stack; a load test against the same program built with Go.

### Part G: finish (2–4 weeks)

20. **16.20 Speed and size** (1–2 weeks): the precise heap (stage 2 of §2.4) if steps 16.1 or
    16.15 showed that false retention or slow marking matters; the budgets of §5 against Go;
    `docs/benchmarks/gc-engine.md`.
21. **16.21 Docs and the list of tested modules** (1–2 weeks): a guide "Using Go packages from
    gp" (the rules of §2.6, the errors and their fixes), "How the GC engine works", the tested
    modules with versions and skip lists, and the GC engine's column in
    `docs/spec/feature-matrix.md`.

## 4. Deliverables

- The GC engine's runtime: `gp::gc` over a vendored bdwgc, Go-layout values, thread
  registration and blocking sections, finalizers, cleanups, weak pointers, the netpoller.
- The compiler: loading `.go` packages and Go modules (`GOPROXY`, the checksum database, the
  `go` line), GC-mode lowering and emission, the gp view, the generated boundary.
- `gostd/`: the GC engine's `runtime`, the hook table, the overlays, the module overlays, and
  `tools/gostdcheck`.
- Go's standard library on the GC engine, tested with Go's own tests.
- `gocpp test` for Go packages.
- The M10 and M11 programs and suites, an ADR, `docs/benchmarks/gc-engine.md`, two guides and
  the list of tested modules.

## 5. Exit criteria

**M10: Go libraries pass their own tests**

- The GC engine's runtime tests pass on Linux (GCC, Clang) and on the Windows PC (llvm-mingw):
  roots, finalizer order, cleanups, weak and `unique`, blocking sections (a thread blocked on a
  channel is never signalled), 10,000 goroutines that allocate.
- The stop-the-world pause is measured and recorded with 10,000 goroutines, blocked and running,
  and stays under the budget that step 16.2 sets.
- Go's own tests of the standard packages of step 16.14 pass, and each test on the reviewed list
  has a reason.
- The libraries of step 16.15 pass their own tests on Linux and on the Windows PC, except the
  reviewed skip list, and a gp program uses each of them through the boundary.
- The boundary suite is green: every row of §2.6, both ways, and an error test for every refused
  crossing.
- A gp program that imports no Go package builds to the same C++ as before Phase 16 and links no
  collector.

**M11: `net/http`, TLS and gin on Linux and Windows**

- Go's tests of `net`, `net/http`, `crypto/tls` and `crypto/x509` pass, except a reviewed list
  (tests that need the internet, the `go` command or gc's runtime internals).
- 10,000 idle connections and a load test run without failures on Linux and on the Windows PC.
  A deadline or a close wakes exactly the waiters it should (tests).
- An HTTPS server built by gocpp talks to `curl` and to a Go-built client, with the system's root
  certificates, on Linux and Windows.
- gin v1.12.0 and v1.9.1: the program of §2.1 serves requests on Linux and Windows, and gin's
  own tests pass, except a reviewed list.
- The gin program, against the same program built with Go (both release builds): requests per
  second, p99 latency, memory and GC pauses within the budgets of step 16.2, recorded in
  `docs/benchmarks/gc-engine.md`.
- Cold build time, warm rebuild time after an edit of `main.gp` (the target for a debug build
  is 2 s) and binary size within the budgets of step 16.2.

**Phase done**

- M10 and M11 are proved, the ADR is accepted, the guides and the list of tested modules are
  published.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Stopping the world with thousands of OS threads: bdwgc signals each running thread in turn, and every thread's stack is scanned | Every wait is a blocking section (§2.5), so only running threads are signalled. Measure with 10,000 threads in 16.1 and 16.4; tune the heap growth; document the limit (tens of thousands of connections, ADR-104) |
| Long pauses on big heaps: no concurrent marking, at most 16 markers, hundreds of milliseconds and more on heaps of several GB | Parallel marking; budgets measured on gin-sized heaps; `gp::gc` keeps the backend replaceable (an own collector, Go's, or Whippet once it supports Windows) |
| False retention from conservative scanning | Pointer-free objects are never scanned (stage 1); a precise heap (stage 2) when the numbers ask for it; stacks stay conservative, as in gccgo |
| Surprises at the boundary (copies, snapshots, proxies) | A short list of rules (§2.6), refusals with a named fix instead of silent copies, `gocpp vet` warnings, a guide |
| Cross-engine cycles leak | Documented, the same class as ARC cycles today; the cycle report of ADR-103 names the shapes |
| Build time and binary size: two standard libraries and big module graphs (gin's closure is about 675k lines) | Content-hash cache per package, a prebuilt GC-engine standard library, parallel builds, `--gc-sections` and LTO in release builds, budgets from 16.1 |
| Go modules need a newer Go than gocpp's (go-redis v9.23 already needs Go 1.26) | A clear refusal that names both versions; the yearly version move of ADR-110 |
| Code that cannot be translated: assembly without a fallback, cgo, `//go:linkname` into runtime internals, generated unsafe code (`modernc.org/sqlite`) | One report per package with the reason; build tags (`purego`, `nomsgpack`, `codec.safe`); module overlays for `x/sys`; a published "does not work" list |
| Signals: a C library that does not retry on `EINTR`, a thread that blocks the collector's signals (bdwgc retries for a while, then aborts with "Signals delivery fails constantly") | Real-time signals chosen and documented; gocpp's own system calls retry; foreign threads unblock the signals when they register |
| ASan and TSan with a conservative collector | Tried in 16.1; if they do not work, the GC engine's tests run without them and the ARC engine keeps its sanitizer jobs; the ADR records it |
| Windows: thread registration and `SuspendThread` with llvm-mingw | Tested from 16.3 on, on the real PC; LLGo, TinyGo and Crystal ship bdwgc on Windows (§7.2) |
| Some declarations of a Go package do not check in the gp view (gp's standard library lacks an API) | Only those symbols are unusable from gp, with the reason; the Go side is not affected |
| Overlays and natives drift with each Go version | `tools/gostdcheck` lists what is new; Go's runtime keeps the signatures of the functions that notable modules pull stable (go.dev/issue/67401) |
| Licenses | bdwgc's notice ships with the others; `GC_BUILTIN_ATOMIC` keeps `libatomic_ops_gpl` out; Go's source is BSD-3, like the gp ports |

## 7. Evidence (2026-10-21)

### 7.1 Survey of 17 popular Go modules

Each module was resolved with its dependencies, and every package was type-checked with go/types
(433 packages, no type errors) for linux/amd64 with `CGO_ENABLED=0`. "Missing std" counts the
standard packages that gp's standard library lacks. "Plain %" is the share of non-standard lines
in files without `unsafe` (beyond `Sizeof`), `//go:linkname`, assembly or cgo, and the second
number is with the tags `purego,noasm,appengine,safe,codec.safe,disableunsafe,nounsafe`.

| Module | Version | Non-std pkgs / lines | Missing std pkgs / lines | net, http, tls | Plain % | Main blockers |
|---|---|---|---|---|---|---|
| gin | v1.12.0 | 101 / 300,472 | 122 / 195,169 | yes, yes, yes | 95.8 (96.4) | `ugorji/go/codec` (112,685 lines: 20 runtime linknames, `ArrayOf`, `MakeChan`, `Select`), quic-go, `x/sys`, `bytesconv` |
| gin | v1.9.1 | 53 / 143,068 | 119 / 191,451 | yes, yes, yes | 93.6 (99.8) | `ugorji/go/codec`, `bytesconv` |
| chi | v5.5.0 | 1 / 1,844 | 103 / 157,885 | yes, yes, yes | 100 | `net/http` only |
| echo | v4.16.0 | 21 / 70,757 | 104 / 162,238 | yes, yes, yes | 89.2 (100) | `x/sys` through `go-isatty` |
| gorilla/websocket | v1.5.3 | 1 / 3,540 | 104 / 158,036 | yes, yes, yes | 98.4 (100) | one unsafe file (safe variant by tag) |
| yaml.v3 | v3.0.1 | 1 / 11,285 | 0 | no | 100 | none |
| BurntSushi/toml | v1.6.0 | 2 / 4,385 | 0 | no | 100 | none |
| protobuf (proto, protojson) | v1.36.12 | 22 / 19,201 | 1 (`go/token`) | no | 99.2 | interface layout through `unsafe`, `MethodByName` + `Call` |
| go-redis | v9.22.0 | 18 / 61,869 | 84 / 96,600 | yes, no, yes | 99.7 (99.8) | `AddCleanup`, `x/sys/cpu`, xxhash assembly (`purego`) |
| pgx | v5.11.0 | 26 / 73,240 | 90 / 107,004 | yes, no, yes | 100 | `ArrayOf` |
| gorm | v1.31.2 | 17 / 37,044 | 39 / 28,295 | no | 100 | `StructOf`, `MethodByName` + `Call`, `database/sql` |
| cobra | v1.10.2 | 2 / 12,329 | 13 / 26,528 | yes, no, no | 100 | `net` (pflag's IP flags) |
| zap | v1.28.0 | 10 / 8,443 | 103 / 157,885 | yes, yes, yes | 100 | `net/http` (its level handler) |
| google/uuid | v1.6.0 | 1 / 1,311 | 38 / 37,882 | yes, no, no | 100 | `net.Interfaces`, crypto hashes |
| testify (assert, require) | v1.12.1 | 6 / 23,800 | 105 / 158,691 | yes, yes, yes | 99.4 (100) | `net/http` (HTTP assertions) |
| bubbletea | v1.3.10 | 18 / 65,560 | 16 / 17,464 | no | 87.6 | `x/sys/unix`, `os/exec`, `os/signal` |
| x/text/cases | v0.41.0 | 8 / 24,820 | 0 | no | 100 | none |
| modernc.org/sqlite | v1.59.0 | 12 / 445,931 | 43 / 45,776 | yes, no, no | 7.5 | generated unsafe code, 1,875 assembly wrappers: not a target |

go-redis, sqlite and `x/text` are one version below their latest, which already needs Go 1.26.
bubbletea v2 and echo v5 (new module paths) were not surveyed.

What the survey found:

- **The standard library is the biggest blocker**: 14 of 17 modules need standard packages that
  gp lacks (140 packages and 212,780 lines for all 17 together). The crypto tree alone is 65
  packages: even `crypto/rand`, `sha1` or `md5` pull about 25 FIPS 140 packages.
- **`net`** is needed by 11 modules and **`net/http`** by 6. The `net/http` + `crypto/tls` closure
  is 178 packages and 324,441 lines (gp lacks 103 of them, 157,885 lines). Its standard packages
  pull **166 distinct runtime symbols**: 37 for `reflect`, 18 for `sync`, 15 for `syscall`, 12
  for `internal/poll`, 7 for `time`, and more.
- **`unsafe` beyond `Sizeof`** appears in 8 modules, zero-copy `string` ↔ `[]byte` in 6, and
  `//go:linkname` in 5. Outside `x/sys`, only gin (20 pulls from ugorji into runtime internals,
  2 from `x/net` into `syscall`) and sqlite (7 into `math/big`'s assembly) use `//go:linkname`.
- **`reflect` APIs that gp's `reflect` lacks** are used by 8 modules. Never used by any of the
  17: `MakeFunc`, `FuncOf`, `ChanOf`, `CallSlice`, `Value.Method(i)`, `TrySend`, `TryRecv`.
- **No cgo** outside the standard library; `CGO_ENABLED=0` changes only `net`'s DNS resolver and
  `os/user`.
- **Already in gocpp, and must keep working in the GC engine**: `select` (11 modules),
  `sync.Pool` (11), goroutines (9), generics (9), `runtime.Callers` and `FuncForPC` (9),
  `sync/atomic` (7; go-redis alone has 530 uses), range over functions (4).
- **gin's JSON**: the default backend is `encoding/json`. The tag-selected ones are out: `jsoniter`
  (12 linknames into `reflect`), `go_json` (46 linknames), `sonic` (a JIT with about 28 runtime
  linknames).

### 7.2 Prior art

| Project | What it does | What Phase 16 takes |
|---|---|---|
| **LLGo** (Go through LLVM) | conservative bdwgc by default; goroutines 1:1 on OS threads; claims the whole standard library on Linux, macOS and Windows; per-thread `poll()` with one shared wake pipe on Linux, an IOCP dispatcher on Windows | the closest precedent: threads + bdwgc work; root per-goroutine state instead of TLS; avoid the shared wake pipe |
| **TinyGo** | bdwgc on Linux, macOS and Windows hosts; a thread per goroutine on Linux since 0.38; precise layouts with Boehm since 0.43 (2026-10-09); its own `reflect` (no `Call`) and a replaced `runtime`, `reflect`, `net/http` | incremental mode off; precise layouts as a second stage; its replaced packages show where Go's own code is hard |
| **gccgo / libgo** | Go's runtime reused, with conservative stacks and a precise heap; the real epoll netpoller. In 2010 its first stop-the-world collector "works but it is far too slow"; in 2011 a thread per goroutine was "inefficient for programs with lots of goroutines", and goroutines moved onto fewer threads | conservative stacks are workable; the thread cost is the known price of ADR-104 |
| **go2cpp** (Hajime Hoshi, archived 2022) | Go's Wasm output turned into C++: "bad performance and long compiling time", GC pauses on one thread; replaced by the real Go compiler with overlays (Hitsumabushi) | build time is a first-class budget (§5) |
| **GopherJS** | `go/types`-checked Go as JavaScript; standard packages augmented with "natives" overlays | the overlay mechanism of §2.7 |
| **Nim** | the memory manager is a per-program switch (`--mm:orc`, `arc`, `boehm`, `go`, …) | the engine is a property of the code, chosen once |
| **Crystal** | bdwgc (8.2+) with threads (`GC_pthread_create`, `GC_beginthreadex`) and fibers | bdwgc's thread API on Linux and Windows in a production language |
| **Whippet** (Andy Wingo, MIT) | an embeddable collector library (parallel, generational, conservative or precise roots), with a bdw backend; no Windows or macOS support yet | the shape of the `gp::gc` interface; a later backend |
| **MPS** (Ravenbrook) | an incremental collector; Windows with MSVC only, and it needs to own `SIGSEGV` | not a fit today |

bdwgc facts used above: release 8.2.12 (2026-02-05), canonical repository github.com/bdwgc/bdwgc;
`GC_do_blocking` leaves a thread out of the stop and scans its stack from where it blocked; the
stop handshake is linear in the running threads; at most 16 markers; incremental and parallel
marking do not combine; finalizers can run on demand from one thread; short disappearing links
match Go's `weak` rule; thread-local storage is not a root.

Sources (read 2026-10-21):

- bdwgc: https://github.com/bdwgc/bdwgc (`docs/gcdescr.md`, `docs/scale.md`,
  `docs/finalization.md`, `include/gc/gc.h`, `include/gc/gc_typed.h`); V's slowdown without
  `THREAD_LOCAL_ALLOC`: https://github.com/vlang/v/issues/27486
- LLGo: https://github.com/goplus/llgo (`README.md`, `doc/defer-tls-gc.md`,
  `runtime/internal/lib/runtime/poll_linkname_llgo.go`, `poll_windows_llgo.go`)
- TinyGo: https://github.com/tinygo-org/tinygo (`CHANGELOG.md`, `builder/bdwgc.go`,
  `src/runtime/gc_boehm.go`, release v0.43.0)
- gccgo: https://go.googlesource.com/gofrontend/+/0fb550083ae474fb964435927b899ec8e4b62771,
  https://gcc.gnu.org/legacy-ml/gcc-patches/2010-07/msg01890.html,
  https://gcc.gnu.org/legacy-ml/gcc-patches/2011-12/msg01085.html
- go2cpp: https://ebitengine.org/en/blog/native_compiling_for_nintendo_switch.html
- GopherJS: https://github.com/gopherjs/gopherjs (`build/build.go`, `compiler/natives`)
- Nim: https://nim-lang.org/docs/mm.html · Crystal:
  https://github.com/crystal-lang/crystal/blob/master/src/gc/boehm.cr
- Whippet: https://github.com/wingo/whippet · MPS:
  https://memory-pool-system.readthedocs.io/en/latest/topic/platform.html
- Go: https://go.dev/doc/gc-guide, https://pkg.go.dev/runtime (`SetFinalizer`, `AddCleanup`),
  https://pkg.go.dev/weak, https://go.dev/issue/67401 (`//go:linkname` lock-down),
  https://golang.org/issue/8404 (no past-the-end pointers)

The survey was a one-off script (`go list -deps` and go/types over the modules above); its raw
data is not kept in the repository.
