# Phase 16: Dual engine (Go packages on a garbage collector, next to gp's reference counting)

> Import Go modules such as gin from gp code, unchanged. gocpp translates their `.go` files to
> C++ the way it translates `.gp` files, but for a **second engine** that has Go's own memory
> layout and a **tracing garbage collector** (Boehm GC first). gp code, C and C++ stay on
> today's engine: reference counting, no collector. Both engines live in one program, and
> values are converted where they cross. These are your decisions of 2026-10-21 and 2026-10-22.
> **This is a plan only. Nothing is scheduled, and the phase starts when you decide.**

| | |
|---|---|
| Depends on | Phase 15 (v1.0 ships first). Builds on 05 (type descriptors, panics), 06 (modules, cache, `gocpp get`), 07 (generics), 08 (threads, channels, `select`, timers), 09 (stdlib, `gocpp test`, the counting rule), 11 (the editor), 13 (vet, `explain`, debugger printers), 14 (Windows), 15.2 (escape analysis), 15.10 (Go's `test/` programs), 15.14 (MVS over `go.mod`) |
| Unlocks | Go modules from gp code without porting them; Go's own `net/http` and `crypto/tls` for gp programs |
| Milestones | **M10: pure-Go libraries pass their own tests** · **M11: `net/http`, TLS, the netpoller and gin on Linux and Windows** |
| Size | very large: about 52–77 weeks for one person (the parts are sized in §3) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

### 1.1 Your decisions (2026-10-21 and 2026-10-22)

| Question | Decision |
|---|---|
| How Go code is built | **Translated to C++ by gocpp**, like gp code. The Go toolchain is not needed. Building with the real `go` (option B1 of the first draft, kept in STATE.md) stays only an idea for packages that the translator cannot handle. |
| Memory | **Two engines in one program.** gp code stays on reference counting, also when Go packages sit next to it. Go packages, and a second copy of the standard library for them, run on a garbage collector. Values are converted at the boundary. |
| Collector | **Boehm GC (bdwgc) first**, reached only through a small `gp::gc` interface, so that an own collector can replace it later. |
| Order | **Libraries first, then gin.** M10: pure-Go libraries pass their own tests. M11: `net/http`, `crypto/tls` and the netpoller, then gin, on Linux and Windows. |
| Windows in M10 | **Yes** (2026-10-22). The libraries pass their own tests on the Windows PC too, so bdwgc on Windows and the Windows natives of the standard library are on M10's path. |
| Standard imports | **gp's library first, else Go's** (2026-10-22, §2.3). When gp's standard library later gains a package, programs that imported Go's copy move to gp's; the release notes name each such package, and `gocpp explain` shows the engine of each import. |
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
  (375k lines). gp's standard library lacks 141 of those 203, and a port would have to add 122
  of them (the others are imported only by Go's versions of packages that gp already has).

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

Words used below:

- **Root**: a reference that the collector must treat as live. gp code that keeps a Go object
  holds it through a root (`gp::bridge::hold`, one per object), so the object lives as long as
  gp code needs it.
- **Proxy**: a small GC object that stands for a gp object inside the GC engine. It holds a
  reference count on the gp object, and its cleanup drops the count when the collector frees it.
- **Twin**: a standard type that both standard libraries have and that gp code sees as gp's own,
  from a short allowlist (`time.Time`, `time.Duration`, the `io` interfaces, `error`, …; §2.6).
- **Mirror**: a GC copy of a gp struct in Go's layout, with the same fields and tags, generated
  as Go source, so Go's `reflect` and `encoding/json` see a normal Go struct (§2.6).
- **gp view**: a Go package's API as gp code sees it: its declarations type-checked again
  against gp's standard library, with the twins substituted (§2.6).

### 1.4 What does not change

- gp code: the same semantics, the same C++, the same speed. A program that imports no Go
  package does not link the collector and pays nothing for it.
- Goroutines stay OS threads in both engines, with no scheduler (Q4, ADR-007).
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
- Millions of goroutines or connections (each goroutine is an OS thread: ADR-007, measured in
  ADR-104).
- In the first version, static and shared libraries that contain the GC engine. Executables and
  test binaries come first.
- The MSVC ABI (`clang-cl`, Phase 14's later target) for the GC engine: Go's layout needs
  `[[no_unique_address]]`, which that ABI ignores (§2.2).
- Go's profilers and `testing/synctest`: `runtime/pprof` and `runtime/trace` build and report
  that they are not supported, and `synctest.Test` skips with that reason (step 16.10).

### 1.6 Alternatives looked at

| Alternative | Why not |
|---|---|
| Go packages on today's ARC engine | cycles leak, `unsafe` layout tricks break, every package needs porting (§1.2) |
| The whole program on the collector, gp included | your decision: gp stays deterministic, on reference counting, also next to Go packages |
| The real Go toolchain (`go build -buildmode=c-archive`) behind a cgo-style bridge (B1) | two runtimes and two schedulers in one process, a cgo crossing per call, and Go must be installed; kept only as an idea for packages the translator cannot handle |
| An own collector from the start | bdwgc already works with OS threads, conservative stacks and Windows in LLGo and Crystal (§7.2); the `gp::gc` interface keeps the door open |

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

`ShouldBindJSON` takes `any` and reaches the struct through interface calls, so no escape
analysis can prove that it keeps no pointer (Go's own compiler reports `moved to heap: req`).
It is on gocpp's list of parameters that are used only during the call (§2.6), so `&req` crosses
as a mirror that is copied back after the call. A gp pointer passed to Go anywhere else is
refused, with a fix.

A sketch of `main.gp`'s C++, in the ARC engine. The exact shapes are decided in steps 16.8 and
16.14. `gin::` stands for `::gopkg_github_dcom__gin_hgonic__gin`, the GC engine's namespace for
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
| `reflect` | gocpp's subset (ADR-035) | Go's own `reflect`, translated (§2.8) |
| Cycles | leak unless broken with `weak.Pointer` | collected |
| Finalizers, `AddCleanup` | `SetFinalizer` is accepted and never runs; no `AddCleanup` | Go's semantics (§2.4) |
| Type descriptors | gocpp's (`type_desc`, Phase 05) | the layout of Go's `internal/abi.Type` (§2.8) |
| C++ names | `gp::`, `gpkg_<path>` | `gp::gc::`, `gopkg_<path>` |
| Threads, channels, `select`, timers, panics, the deadlock detector | one implementation, shared by both engines | |

**Go's exact layout.** The GC engine's structs have Go's sizes, alignments and field offsets,
because `unsafe` code, Go's `reflect` and Go's runtime packages depend on them. Plain C++
structs do not give that: an empty struct takes 1 byte in C++ and 0 in Go, two zero-size fields
get different offsets in C++ and the same one in Go, and Go pads a struct whose last field has
size zero. These shapes are common in Go's standard library (`noCopy` in `sync.Mutex` and
`sync.WaitGroup`, `_ [0]*T` in `atomic.Pointer` and `weak.Pointer`, `_ [0]func()` in
`slog.Value`, the slots of a `map[K]struct{}`). So zero-size fields become `[[no_unique_address]]`
members of distinct empty types, trailing padding is explicit, element strides come from Go's
sizes and never from `sizeof`, and a `static_assert` checks each field offset against Go's
(step 16.5). The ARC engine keeps today's rule, which forbids `[[no_unique_address]]`.

### 2.3 Which engine builds a package

1. A package of `.gp` files is built by the ARC engine, a package of `.go` files by the GC
   engine. A folder with both kinds of files is an error.
2. A module with a `gocpp.mod` is a gp module. A module with only a `go.mod` is a Go module, and
   all its packages are Go packages. Inside a gp module, a folder of `.go` files is a Go package
   (for example generated protobuf code).
3. **Module files.** A module may have a `go.mod` next to its `gocpp.mod` (a Go project that
   adds gp code). The `gocpp.mod` holds the requirements of the `.gp` packages, the `go.mod`
   those of the Go packages and their `go` line, and one MVS run over both (step 15.14) picks
   one version of each module for the whole program. In a gp module without a `go.mod`, the Go
   packages use the `gocpp.mod`'s requirements and the pinned Go version's language rules. As
   for gp modules (`docs/spec/modules.md`), only the main module's `replace`, `exclude` and
   `vendor/` count; those of the modules that it requires, in either file, are ignored.
4. Imports in a `.go` file always resolve in the GC engine's world: Go's standard library (Go's
   source) or Go modules. Go code cannot import gp packages or C/C++ headers. gp code reaches Go
   code only through the values that it passes in: functions, interface values and pointers.
   The standard library's vendored packages keep their `vendor/` paths, so they are not the
   module of the same name: Go 1.25's `net/http` uses its vendored `golang.org/x/net` (v0.41
   plus fixes, in go1.25.14), gin requires the `golang.org/x/net` module at v0.51.0, and one
   program has both, under two `gopkg_` names, as a Go binary does.
5. Imports in a `.gp` file use gp's standard library first (`strings`, `encoding/json`, `time`).
   A standard package that gp's library lacks (`net/http`, `crypto/tls`) comes from the GC
   engine. A module path comes from the module that provides it, gp or Go. This is your decision
   of 2026-10-22, and it has a consequence that is documented, not hidden: when gp's standard
   library gains a package later (ADR-107's waves), programs that imported Go's copy move to
   gp's with the next gocpp release, with gp's semantics and the boundary in between, and code
   that passed that package's values to Go may stop compiling. The release notes name each such
   package, and `gocpp explain` shows which engine each standard import comes from.
6. So a standard package can exist twice in one program (`gpkg_strings` and `gopkg_strings`):
   two namespaces, no clash, but twice the code (§2.10 has the sizes).
7. A program without a Go package is exactly today's program: no collector and no hooks.

Whether gp code may also ask for the GC engine's copy of a package that gp has
(`import gojson "go:encoding/json"`, which would also keep a program on Go's copy when gp's
library gains the package) is a question for the ADR of step 16.2.

### 2.4 The collector: `gp::gc` over bdwgc

**The library.** bdwgc 8.2.12 (2026-02-05, an MIT-style license, the notice ships with
gocpp's other notices). It is vendored as source in `runtime/third_party/bdwgc` and built by
gocpp, with fixed flags, into the runtime cache: once per toolchain and sanitizer (bdwgc adapts
to ASan and TSan only when it is built with them), and only for programs that have Go packages.
The same build serves Linux and Windows (MinGW-w64, llvm-mingw). The flags:

- `GC_THREADS`, `PARALLEL_MARK` (16 marker threads by default: `MAX_MARKERS` is a build setting)
  and `THREAD_LOCAL_ALLOC`. Without the last one, allocating threads in the V language did not
  scale: 16 threads took 16.3 times as long as one (vlang/v#27486, fixed by #27544).
- `GC_BUILTIN_ATOMIC`: the compiler's atomic builtins, so `libatomic_ops` is not needed.
- `LARGE_CONFIG`, for heaps above about 4 GB.
- `USE_MMAP` and `USE_MUNMAP`. A build with fixed flags gets neither (only bdwgc's CMake and
  configure builds turn them on), and without them the heap never returns memory to the system.
- `ALL_INTERIOR_POINTERS`, with bdwgc's extra byte at the end of each object kept (no
  `DONT_ADD_BYTE_AT_END`) until step 16.21 measures whether it can go. Go code keeps interior
  pointers (`&s.f`, a slice into an array). The GC engine never makes a past-the-end pointer at
  the Go level (a slice expression whose capacity becomes 0 keeps the base pointer, as gc does:
  golang.org/issue/8404), but the C++ optimizer, `std` algorithms and libc do form end
  pointers, and without the extra byte such a pointer lands in the next object. The cost: an
  object whose size is a multiple of 16 B takes 16 B more.
- Incremental mode off: it write-protects pages and handles `SIGSEGV`, which gocpp's runtime
  owns (TinyGo turns it off for the same reason). bdwgc also mostly skips the parallel marker in
  incremental mode.

**What is scanned.**

- Stacks, registers and static data: conservatively, always. Generated C++ has no stack maps,
  and gccgo scanned its stacks the same way for years.
- The GC heap, in two stages. Stage 1 (M10): pointer-free objects (the bytes of a string, a
  `[]byte`, a struct of numbers) use `GC_malloc_atomic` and are never scanned. It does not clear
  memory, so the engine zeroes it unless every byte is written at once (a string concatenation,
  a copy). Other objects use `GC_malloc` and are scanned conservatively. Stage 2 (step 16.21, if
  the numbers ask for it): the heap is precise. Each type's pointer bitmap (Go's `GCData`, which
  the type descriptors carry anyway for `reflect`) becomes a bdwgc descriptor, so a number that
  looks like a pointer no longer keeps an object alive. TinyGo 0.43 made the same change
  (2026-10-09). bdwgc's typed allocation (`GC_malloc_explicitly_typed`) does not use the
  thread-local free lists and adds a word to each object, which is the slow case of the V
  numbers above. So stage 2 needs gcj-style allocation (which is thread-local) or a patched
  kind, and step 16.21 picks one.
- **Not the ARC heap.** Memory from `malloc` and `new` (gp objects, C++ containers) is never
  scanned, so a GC pointer that is kept there must be a root (§2.6). That includes the runtime's
  own structures: the task of a `go` statement until its thread runs, defer records that spill
  past their 64 B inline buffer, channel buffers, timer records and `AfterFunc` closures. Each of
  them is allocated in the GC heap or rooted for its whole life (step 16.4 audits the runtime
  for this). A `chan_core` in the GC heap has no members that need a destructor, because the
  collector runs none.
- **Not thread-local storage.** LLGo hit this: a defer chain that only pthread TLS held was
  freed. The runtime keeps a goroutine's GC pointers in a rooted per-thread record instead. The
  value of a panic in flight is rooted from the panic's construction to its destruction (a
  `GC_malloc_uncollectable` cell), because C++ exception objects and `std::exception_ptr` live
  in `malloc` memory.

**Go's API on top.**

| Go | GC engine |
|---|---|
| `runtime.GC()` | `GC_gcollect` |
| `GOGC`, `debug.SetGCPercent` | `GC_set_min_bytes_allocd`, recomputed from the live heap after each collection; `off` disables collection. bdwgc's own knob, `GC_set_free_space_divisor`, is an integer divisor of a weighted heap size in which pointer-free bytes count a quarter, so it cannot express `GOGC` above about 200 |
| `GOMEMLIMIT`, `debug.SetMemoryLimit` | a soft limit on the bytes in use, emulated: a collection when they pass it, with a cap on the CPU time spent collecting, as in Go. bdwgc's hard cap (`GC_set_max_heap_size`) is not used, because it makes allocation fail |
| `runtime.SetFinalizer` | `GC_register_finalizer`, which has Go's order: if A points to B, A is finalized first, and a cycle with finalizers is never finalized. Finalizers run on demand, on **one finalizer goroutine**, as in Go. Java-style finalization is set explicitly (`GC_set_java_finalization`), because its default depends on a build macro |
| `runtime.AddCleanup` | a list of cleanups per object: several per object, interior pointers allowed, cycles do not leak. bdwgc keeps one finalizer per object (the last one registered), so `gp::gc` owns the list. Without a Go finalizer, one unordered finalizer on the object's base runs the cleanups. With one, the Go finalizer runs first and then registers the unordered one, so the cleanups run only after the object becomes unreachable again (Go's rule) |
| `weak.Pointer`, `unique` | short disappearing links, which are cleared when the object first becomes unreachable (Go's rule). A weak handle per (base, offset) makes two weak pointers made from the same pointer compare equal, also after the object is gone. The handle is dropped when its link clears, so a new object at the same address gets a new one. Reading a link takes bdwgc's lock, so `Value` and `unique.Make` do too |
| `runtime.ReadMemStats`, `debug.FreeOSMemory` | bdwgc's heap statistics, plus per-thread counters of allocations in `gp::gc` (bdwgc counts only bytes, and `testing.AllocsPerRun` reads `Mallocs`); `GC_gcollect_and_unmap` |
| `runtime.KeepAlive` | `GC_reachable_here`: no instructions, but an optimization barrier, so the pointer stays visible until that point and a finalizer cannot run early (`os.File` relies on it) |
| `runtime.Pinner` | a root (the collector never moves objects) |

**The interface.** `gp::gc::heap` is small: allocate with a type, allocate pointer-free, enter
and leave a thread, run without the collector (a blocking section), make and drop roots,
finalizers, cleanups, weak links, collect, statistics and limits. Its shape follows Whippet's
API, so the backend can become an own collector, a port of Go's collector, or Whippet (MIT, but
without Windows support today) without touching the engine. Nothing else calls bdwgc.

### 2.5 Threads and stopping the world

- Goroutines stay OS threads in both engines. bdwgc stops the world with signals on Linux (one
  signal per running thread, then a handshake) and with `SuspendThread` on Windows.
- **Registration.** Every thread that can touch GC memory registers with the collector
  (`GC_register_my_thread`, after `GC_allow_register_threads` in `run_main`, before gocpp starts
  any other thread). The registration is explicit, not bdwgc's macros that redirect
  `pthread_create`, so the same code covers threads that a C or C++ library starts, and
  Windows. That means goroutine threads (`runtime/src/goroutine.cpp`) and the runtime's own
  threads that touch GC memory: the timer thread (`timer.cpp`, a plain `std::thread` today), the
  poller thread and the finalizer goroutine, which wait in blocking sections too. A thread that
  gocpp did not start but that runs Go code (a callback from a C library) registers in the
  boundary stub, before it allocates.
- **Every registration is undone on every exit path** (`GC_unregister_my_thread`): a worker
  that returns, an idle thread that ends after its timeout (5 s by default, `goroutine.cpp`),
  `exit_thread`, and a thread-exit callback for foreign threads (a pthread key destructor on
  Linux, a fiber-local-storage callback on Windows). A registered thread that exits makes the
  next collection abort: tried with bdwgc 8.2.12 on this machine, the collection waited 16 s,
  then stopped the program with "Signals delivery fails constantly". The stack-overflow
  handler's alternate stack is announced with `GC_register_altstack` after `sigaltstack`.
- **Every wait that can last is a blocking section** (`GC_do_blocking`): parking on
  `gp::waiter` (channels, `select`, `sync`), `Sleep`, an idle reused thread, timed waits, the
  netpoller and blocking I/O (`runtime/src/os/linux/posix_file.cpp`,
  `runtime/src/os/windows/win_syscall.cpp`). A blocked thread is not signalled, and its stack is
  scanned from the point where the section began. The frames inside the section are not
  scanned, so the callback only waits: it is `noexcept`, never allocates, is entered with no
  runtime mutex held, and leaves `pred()` and hand-off targets in scanned frames.
- **Blocking sections are not free.** Each one takes bdwgc's global lock twice. Measured on
  this machine (128 cores): 156 ns per section on one thread, 9 µs at 32 threads and 19 µs at
  64, so at most 3 to 6 million sections per second for the whole process. Short system calls
  are therefore not wrapped: `syscall.Syscall` and `Syscall6` (Go's calls that may block)
  become one native that runs the call inside a section, `RawSyscall` stays a plain call, and
  Go's `entersyscall` and `exitsyscall`, a pair that does not fit a callback API, do nothing.
  Step 16.1 measures parks and system calls per second at 1 to 64 threads; a lock-free patch to
  the vendored bdwgc is the fallback.
- **What a stop costs.** One signal per running thread, plus the scan of every registered
  thread's stack. With a tiny heap, a stop took 0.6 ms with 1,000 blocked threads, 5.6 ms with
  1,000 running ones and 22 ms with 10,000 blocked ones (this machine). Each registered thread
  also keeps bdwgc's thread-local free lists alive, about 37 KB per blocked thread (1,000
  threads: 39 MB; 10,000: about 370 MB, extrapolated), where Go keeps one allocation cache per
  processor. The budgets of step 16.2 have a per-thread term for pauses and memory; long waits
  flush the lists (a small patch to bdwgc) or idle workers unregister.
- Signals: bdwgc's defaults are `SIGPWR` and `SIGXCPU`. gocpp chooses two real-time signals
  before `GC_INIT` and documents them, except under `-race`, where bdwgc's TSan build uses
  `SIGSYS`: TSan holds back asynchronous signals while a thread waits in `pthread_mutex_lock`.
  gocpp's own system calls already retry on `EINTR`. C libraries that do not retry meet the same
  problem as under Go's preemption signal (since Go 1.14).
- A goroutine in a blocking section counts for the deadlock detector exactly as today. A
  goroutine that waits in the netpoller is not asleep: the network can wake it, as in Go. The
  finalizer goroutine is a system goroutine except while it runs user code, as in Go:
  `NumGoroutine` and the deadlock detector do not count it, and pending finalizers can wake
  goroutines, as timers can.

### 2.6 The boundary: what crosses and how

Three rules:

1. **A value has the engine of its type, and Go memory stays Go memory.** Types declared in Go
   packages (`gin.Engine`, `http.Request`) belong to the GC engine, types declared in gp
   packages to the ARC engine. Unnamed and built-in values that gp code makes (`[]byte`,
   `map[string]any`) are ARC values and convert when they cross. A Go map, slice or pointer that
   gp code gets from Go (a field, a result) keeps the GC engine while gp code holds it:
   indexing, field access, `*p`, `delete`, `clear`, `range` and `x.f = append(x.f, …)` work in
   place on the Go memory, and only the element converts. So `c.Keys[k] = v` changes gin's map,
   and pflag's `fs.String()` (a `*string` that `Parse` fills later) reads the parsed value. Such
   a value converts (a copy) only when it flows into a gp parameter, field or element; for a
   pointer, that is an error with a fix (pass `*p`). go/types cannot tell the two engines'
   unnamed types apart, so the checker tracks the engine of each expression next to go/types,
   as the interop checker tracks C types.
2. **gp code sees one standard library, for a short list of types: the twins.** When gp code
   looks at a Go package's API, the standard types on an allowlist appear as gp's own: named
   numbers, strings and bools (`time.Duration`, `fs.FileMode`), `time.Time`, the interfaces of
   `io`, `fmt.Stringer`, `context.Context` and `error` (through proxies), and the exported error
   variables and types (`io.EOF`, `*fs.PathError`). gocpp type-checks the Go package's
   declarations a second time against gp's standard library, with function bodies skipped and
   the twins substituted; this is the package's **gp view**. Every other standard type keeps
   its Go type in the gp view, also when it is embedded (messages show it as `go:sync.Mutex`),
   because a type with identity cannot be copied: `sync.Mutex`, `sync.WaitGroup`,
   `strings.Builder`, `*os.File`, `*bufio.ReadWriter`, `reflect.Value`. A gp value goes where a
   Go one of those types is expected only through an explicit adapter, for example `gc.File(f)`:
   a Go `*os.File` over the same descriptor, with no finalizer. A tool checks the allowlist: no
   pointers, no `noCopy`, no self-address checks and no finalizers in a twin. Types that only
   the GC engine has (`*http.Request`) stay Go types.
3. **The collector never scans the ARC heap, and the ARC engine never counts GC objects.** A GC
   pointer kept in ARC memory is pinned by a root. An ARC object kept by the GC engine is held
   by a proxy, whose cleanup drops the count.

| Kind | gp → Go (an argument, a field write) | Go → gp (a result, a field read) |
|---|---|---|
| numbers, `bool`, arrays and structs of them | a copy (the same bits) | a copy |
| `string` | borrowed for the call when the Go parameter does not escape, otherwise copied into the GC heap; string literals are never copied | no copy: the gp string points into the Go string's bytes, and its owner (what keeps a gp string's memory alive) holds a root on the GC object |
| `[]byte` and other slices of numbers | borrowed for the call when the parameter does not escape, so Go's writes are seen by gp, as in Go. Otherwise copied, and copied back over `[0:len)` after the call (also on a panic), so the callee's writes are seen; writes after the call are not shared (a documented difference). A slice that came from Go crosses back as itself | no copy: the owner is a root, and both sides share the array, as in Go |
| slices and maps of other elements | made by gp code: converted element by element (a copy) | Go's stay Go memory while gp code holds them (rule 1) and convert when they flow into gp memory |
| a pointer to a Go object (`*gin.Engine`) | the raw pointer | a `gp::ptr<T>` whose control block is a root (`gp::bridge::hold`, one root per object); `==` compares addresses, as in Go |
| a pointer to a gp value (`&req`) | **copy in, copy out** when the Go parameter is on the list of parameters used only during the call (below): for the call, a GC copy in Go layout (a mirror), copied back after it, also on a panic. Any other crossing of a gp pointer is refused with a fix: allocate the value in the GC engine with `gc.New[T]()` (its name is decided in step 16.2), which returns a pointer to `T`'s mirror in the GC heap that gp code uses like any Go pointer (fields convert as they are read and written) | (not applicable) |
| interface values | a gp value in a Go interface (`any`, an element of `gin.H`): its dynamic type decides. A gp struct, array or named basic value crosses as a mirror value (a copy, as Go copies a value into an interface), with forwarders for its exported methods. A gp pointer crosses as a copy-in/copy-out mirror on a listed call, otherwise as a canonical pointer proxy with the same forwarders. Equality and hashing are the gp value's, and crossing back returns the original gp value | a Go value in a gp interface carries gp's type descriptor for its Go type (one per type, with thunks for its methods and Go's equality and hash) and holds a root, so `case *gin.Context:` and `x.(*gin.Context)` work. Unnamed and built-in values convert. Pointers, maps, channels and funcs compare and hash by address, other values by value |
| `error` | a proxy; `errors.Is` and `errors.As` see through proxies; the standard sentinels (`io.EOF`, `context.Canceled`, `fs.ErrNotExist`, …) map to their twins, so `err == io.EOF` holds on both sides; twin error types (`*fs.PathError`) convert when `errors.As` asks for one | the same |
| `func` values (`gin.HandlerFunc`) | a GC closure that calls the gp closure (`gp::bridge::go_func`); its cleanup drops the gp closure | a gp closure that holds a root on the Go closure |
| `context.Context` | a Go adapter over the gp context: canceled through gp's `context.AfterFunc`, and implementing `AfterFunc(func()) func() bool` itself, so a Go context derived from it starts no goroutine (a thread here) to watch it | a gp adapter over the Go context, the same way. Adapters are canonical, unwrap when they cross back, and forward `Value` with keys that round-trip |
| `time.Time` | a field copy: both `time` packages share one monotonic base (one `startNano` native), so a monotonic reading stays right. Locations: `UTC` and `Local` by identity, fixed zones by name and offset, others by name with their zone data, cached both ways | the same |
| named numbers (`time.Duration`, `fs.FileMode`) | a copy | a copy |
| channels | a channel whose element has one layout in both engines (`struct{}`, `bool`, numbers) crosses as it is: both engines share one `chan_core`. Other element types are a compile error that names the fix: a small `.go` file in the gp module (a Go package, §2.3) that forwards | the same |
| generic Go functions and types | instantiated with GC-engine types or convertible ones; gp types as type arguments are not in the first version, and an inferred gp type argument is an error that names it | |
| panics | one C++ exception type for both engines, so a panic unwinds through frames of both, and so does `runtime.Goexit`, running both sides' defers. A recovered value that crosses back is the same value (`net/http` checks `err != http.ErrAbortHandler`) | |

More rules:

- **The list of parameters used only during the call** ships with gocpp: the decoders and
  encoders of the standard library and of common modules (`json.Unmarshal`,
  `json.Decoder.Decode`, `json.Marshal`, `encoding/xml`, `yaml.v3`, BurntSushi/toml,
  `proto.Unmarshal`), gin's `Bind*`, `ShouldBind*` and renderers (`c.JSON`, …), and validators
  (`validator.Struct`). A module can extend it with a directive in its `gocpp.mod`. gocpp's
  escape analysis cannot prove these, because they take `any` and go through interface calls:
  Go's own compiler reports `moved to heap: req` for the program of §2.1.
- **Escape summaries.** Borrowing a string or a slice needs to know that the Go parameter does
  not escape. ADR-114's analysis summarizes only `*T` parameters today, so step 16.14 adds
  summaries for string, slice and interface parameters (their contents and results). Calls
  through interfaces and func values count as escaping.
- **Copy-in, copy-out over a graph.** One memo per call, so aliases and cycles map to one
  mirror. `mirror_out` updates the original gp objects in place, and a pointer that Go replaced
  becomes a new gp object. gp callbacks during the call see the old values. A field with no Go
  form (a C++ value, a channel of another layout) is an error that names it.
- **Proxies are canonical**: one proxy per object while the object lives (a weak table), and
  `gp::bridge::hold` gives one root per Go object. So `==` keeps Go's meaning:
  `err == http.ErrServerClosed` holds in gp code when Go returned that error.
- **Mirror types are written as Go source** by the boundary generator (the gp struct's fields
  and tags, in the GC engine's world under the gp package's own path) and translated by the GC
  engine like any Go package. Go's `reflect`, `encoding/json` and validators see a normal Go
  struct, and their messages name `Login.pass` as Go's would. A mirror has forwarders for the gp
  type's exported methods, so Go's checks for `error`, `json.Marshaler`, `io.WriterTo` or
  `http.Flusher` see them.
- **Every Go pointer outside a stack frame is rooted or in the GC heap**: a field of a gp
  struct, a captured variable, an element of a gp slice or of a C++ container, a gp channel's
  buffer, a defer record, the arguments of a `go` statement before the new thread registers. A
  struct of a Go type stored by value is boxed in the GC heap first and keeps value semantics:
  copying the gp variable copies the box. A Go pointer in a gp local that does not escape stays
  a plain pointer, because stacks are scanned.
- **gp objects held by Go lose deterministic release.** A gp object that Go code holds (through
  a proxy, a func adapter or a context adapter) is released only when the collector finds the
  proxy unreachable and its cleanup runs on the finalizer goroutine, or never, if the program
  exits first. bdwgc does not see the ARC bytes that proxies keep alive, so `gp::gc` counts them
  and starts a collection when they grow.
- **gp's `fmt` and `encoding/json` hand Go values to Go's.** `%v` of a `*gin.Context` prints what
  Go prints. Other users of gp's `reflect` see a Go value as opaque: its type name, no fields.
- **Cycles across the boundary** (a gp object → a root → a Go object → a proxy → the same gp
  object) are collected by neither engine, and they are common: a gin handler that captures `r`,
  a cobra `RunE` that captures its command. They leak like ARC cycles today unless they are
  broken: gp's `weak.Pointer` to a Go object is a GC weak link (§2.4), so it breaks such a cycle
  as it breaks ARC cycles. `gocpp vet` sees one gp function at a time; it warns about a func
  literal passed to Go that captures a Go pointer whose type reaches func fields. ADR-103
  proposes a cycle report (not built); to name these shapes, it would have to follow roots and
  proxies through the GC heap.
- **Names of functions across the boundary.** `runtime.Caller` and `Callers` skip the
  boundary's thunks, and `runtime.FuncForPC` of a gp function that crossed as a Go func names
  the gp function, not the thunk (gin's route log and `HandlerName` use it). gp's `runtime.GC`
  and `debug.SetGCPercent` reach the collector too.
- **Cost**: a call from gp to Go with numbers is a plain C++ call, which the compiler can inline.
  Each converted string, slice or struct costs a copy, and a root costs one small allocation and
  an atomic count. Step 16.1 measures each shape, and step 16.2 sets the budget.

### 2.7 Go's standard library on the GC engine

**Source.** Go's own `src/` at the pinned Go version (go1.25 today; ADR-110 moves it), never gp's
ports, because Go packages expect Go's exact behavior. gocpp downloads the pinned source archive
once (its checksum is in gocpp) or uses a local Go tree named by `GOCPP_GOROOT`, whose `VERSION`
file must match the pinned version. Nothing needs the `go` command.

**Build constraints.** The real `GOOS` and `GOARCH` (`linux` or `windows`, `amd64` or `arm64`),
`unix` on Linux, the release tags `go1.1` to `go1.25`, the compiler tag `gocpp` (neither `gc`
nor `gccgo`), the tags `purego` and `math_big_pure_go`, and the pinned version's tool tags: its
default experiments (`goexperiment.swissmap`, `goexperiment.synchashtriemap`,
`goexperiment.aliastypeparams`, …) and the architecture level (`amd64.v1`, `arm64.v8.0`). The
tool tags are read from that version's `internal/buildcfg` and recorded per version in `gostd/`,
never taken from the Go that built gocpp: without them, Go 1.25 builds its old maps and its old
`sync.Map`, and `internal/runtime/maps` drops out. `.s` files are never used. With these tags,
the crypto packages, `math/big` and the vendored `x/crypto` drop their assembly, except the
markers of `crypto/internal/boring/sig`. Because the compiler tag is neither `gc` nor `gccgo`, a
few packages lose files that they need and get overlays (below): `internal/goroot` (so that
`go/build` builds), `x/sys/cpu` and `x/sys/unix`.

**Three kinds of replacement**, all kept in the gocpp repository under `gostd/`:

- **The `runtime` package is gocpp's own**: Go source for the public API (`GC`, `SetFinalizer`,
  `AddCleanup`, `Caller`, `Callers`, `Stack`, `NumGoroutine`, `GOMAXPROCS`, `LockOSThread`,
  `ReadMemStats`, `Error`, …) over natives. Go's `runtime` (71.7k lines, over 120 assembly
  functions) is never translated. For modules that pull the internal functions that Go's runtime
  keeps for them ("Do not remove or change the type signature", go.dev/issue/67401:
  `mapaccess2`, `mapassign`, `mallocgc`, `growslice`, `typedmemmove`, …), it provides those
  functions where they make sense without Go's scheduler. The goal is that `ugorji/go/codec`
  (gin's msgpack binding) builds without `-tags codec.safe`: under the `gocpp` tag it leaves out
  its `gc`-only file and pulls 10 runtime functions instead of 20. Sonic's JIT and
  anything that needs Go's write barrier never will.
- **The hook table**: the runtime functions that the other standard packages reach with
  `//go:linkname`. The dry run of 2026-10-24 (`tools/gostddryrun`,
  `gostd/dryrun/go1.25.14`, linux/amd64) counts 303 distinct two-argument `//go:linkname`
  targets in the `net/http` + `crypto/tls` closure. 169 of them are runtime hooks: 151 pushed
  by the runtime into closure packages and 18 pulled from the runtime; 9 are only for
  `internal/synctest`, and 14 are linker symbols. The plan's earlier 166/149/17 came from a
  one-off survey whose counting method was not recorded; the dry run's data replaces it. 163
  targets are bodyless in their target package, 154 provided by the runtime and 9 not; 5 of
  those 9 need a native or overlay of their own (`internal/runtime/atomic.StoreReluintptr`,
  `sync/atomic.{CompareAndSwapUintptr,StoreUintptr,SwapUintptr}`, `time.now`). The runtime ones
  include the semaphores, spinning and `notifyList` of `sync` and `internal/sync`,
  `internal/poll`'s nine `runtime_poll*` functions, `time`'s timers and `now`, `syscall`'s
  `entersyscall` and `exitsyscall` (they do nothing, §2.5), 40 for `reflect`, `iter`'s
  coroutines, `weak`, `unique` and the bubble checks of `synctest`. The dry run counts 151
  runtime pushes for `net/http` + `crypto/tls` (the plan's earlier 154 is not reproducible);
  the 187 with `testing` and its test main is not measured yet, because the dry run does not
  walk `testing`'s closure. Windows adds its own: `loadlibrary`, `getprocaddress`, `SyscallN` and
  `Syscall6` to `Syscall18`, `compileCallback`, `QueryPerformanceCounter`, and more (17
  bodyless functions that only Windows has). Each one is a C++ native over the shared runtime:
  threads, `gp::waiter`, the timer thread and the semaphore table of Phase 08.
- **Overlays**, GopherJS's "natives" idea: replacement `.go` files for the packages whose
  assembly has no pure-Go fallback, or whose files the `gocpp` tag drops. They are `sync/atomic`
  (C++ atomics), `internal/bytealg`, `internal/cpu`, `math`, `hash/crc32`,
  `crypto/internal/boring/sig` (assembly markers on every target), `internal/runtime/syscall`
  (Linux's `Syscall6`, which `syscall.RawSyscall6` calls), `internal/runtime/sys` (arm64's DIT
  assembly, which `crypto/subtle` uses), `syscall` (its fork and exec code:
  `forkAndExecInChild1` runs Go code after the assembly `rawVforkSyscall`), `internal/goroot`,
  `reflect`'s two assembly stubs, `internal/chacha8rand`, and `testing/internal/testdeps`:
  gocpp's own test main helper, as gp's `stdlib/testing/internal/testdeps` is, so that tests do
  not pull `internal/fuzz`, `os/signal` and `runtime/pprof`. `internal/runtime/atomic` needs
  none: only `runtime` imports it, and the two functions that `sync.Pool` pulls from it are
  natives. The Windows `Syscall` entry points and `os/signal`'s hooks are pushes from the
  runtime, so they are natives too. A tool checks the list against Go's source: every bodyless
  function must have a native or an overlay. After a Go upgrade, it shows what is new.

**Module overlays.** A few very common modules need the same treatment. `golang.org/x/sys`
(`unix`, `windows`, `cpu`) has assembly `Syscall` entry points, declares `unix.Syscall` only in
a `gc` file, needs `cpuid` and `xgetbv` in `cpu`, and has about 228 system-call arguments that
turn a pointer into a `uintptr`. Those arguments are fine here, because the stack scan keeps the
objects alive. An overlay is chosen by module path and version range, and the same overlays serve
the copies that Go's standard library vendors (`vendor/golang.org/x/sys/cpu`).

**Drift.** `gostd/` records each native's Go signature and each overlay's original-file hash,
and `tools/gostdcheck` fails when either changes. go.dev/issue/67401 keeps only some hooks
stable: `notifyListWait`, `pollWait`, `time.newTimer` and `synctest.IsInBubble` can change in
any release. A Go point release (a security fix) is taken like a version move, with the same
check.

**`//go:linkname` in Go packages.** Go 1.25 uses it in five ways, and all are handled: pulls (a
declaration without a body that names another package's symbol: `x/net` pulls
`syscall.getsockopt`), pushes (a package defines a symbol under another package's name: `os` →
`net.newUnixFile`, `sync` → `syscall.hasWaitingReaders`, and 17 from `internal/runtime/maps`
into the runtime, `runtime.mapaccess1` and its variants — the dry run counts 17, the plan said
19), one-argument linknames that only
export a symbol, linknamed variables (`runtime.zeroVal`, `math/bits`' `overflowError` and
`divideError`), and the linker symbol `go:fipsinfo` of the FIPS 140 module. A pull or a push of
a symbol that exists in translated code or in the runtime is resolved to that symbol's C++ name;
anything else is an error that names the symbol. The compiler's intrinsics (`abi.FuncPCABI0`,
`abi.FuncPCABIInternal`, `sys.GetCallerPC`, `sys.GetCallerSP`, `sys.GetClosurePtr`) are
natives as well.

### 2.8 `reflect`, maps and interfaces

- **Type descriptors** have the layout of Go's `internal/abi.Type`: size, `PtrBytes`, hash, kind,
  alignment, the equality function, `GCData`, the name, `PtrToThis`, and the parts for each kind
  (`abi.StructType`, `abi.SwissMapType`, …). They are constant data with no dynamic
  initializers (checked with g++ and clang++: `Type` is 48 B, `StructType` 80 B and
  `UncommonType` 16 B, as in Go), except a writable `GCData` word for the types whose pointer
  mask is built on demand (`TFlagGCMaskOnDemand`, masks over 2,048 bytes).
- **Offsets and lookups.** Go resolves `NameOff`, `TypeOff` and `TextOff` through module data,
  and `reflect` also adds `typelinks` offsets to section addresses itself (`rtypeOff`,
  `typesByString`), to find the compiler's own descriptor for a type that it builds
  (`PointerTo`, `SliceOf`, `MapOf`, `ArrayOf`, `ChanOf`, `FuncOf`, `StructOf`). Without that,
  `reflect` makes a duplicate, and `x.([]string)` fails on a slice that `reflect` made.
  Program-wide indexes cannot be baked into objects that are cached per package, and an unnamed
  type such as `[]int` needs one descriptor per program. The ADR of step 16.2 picks the scheme.
  One candidate: the `resolve*Off` natives index a pointer table stored next to each descriptor
  or name blob; `typelinks`, `rtypeOff` and `typesByString` are overlaid to return
  `[]*abi.Type` from a table that the link step sorts by string; and the descriptors of unnamed
  types are C++ inline variables, so the linker keeps one per program.
- With them, Go's own `reflect`, `internal/reflectlite`, `internal/abi` and
  `internal/runtime/maps` are translated rather than rewritten, and `MapOf`, `ArrayOf`,
  `SliceOf`, `StructOf`, `MakeChan`, `Select`, `Send`, `Recv`, `MethodByName` and `NewAt` (the
  survey's gaps) come with them.
- **Calls through `reflect`** (`Value.Call`, `Method`) use assembly in Go (`reflectcall`). The GC
  engine generates one C++ call thunk for each function signature that the program has: the
  method sets of types that reach an interface or `reflect`, and the function types. This is
  ADR-106's idea, for the whole engine. `reflectcall` receives no function type, so the thunk is
  found from the function's code pointer; with `internal/abi`'s register counts overlaid to 0
  (as on targets without a register ABI), every `reflect` call frame is in memory, which keeps
  the thunks simple. A signature that was built at run time with `FuncOf` has no thunk, and
  calling it panics with a clear message. `MakeFunc` (`makeFuncStub`, `methodValueCall`) is used
  by none of the 17 surveyed modules; until one needs it, it panics with a clear message too.
- **Maps** are Go's own Swiss tables (`internal/runtime/maps`, Go 1.24+), hashed with the portable
  `memhash` and `strhash` (no AES instructions needed), in random order, as in Go. The generated
  code calls them as gc's code does (`mapaccess1_faststr`, …): `internal/runtime/maps` pushes
  those functions into the runtime's names, and gocpp resolves the pushes (§2.7).
- **Interfaces** are `{itab, data}` and `{type, data}`, as in Go. itabs for the conversions that
  the compiler sees are generated, and the others are built at run time and cached, as gc's
  `getitab` does.
- **Escape analysis** (ADR-114) decides between the stack and the GC heap for address-taken
  locals and closures, as in Go.
- **`unsafe` and C++'s rules.** Go code may make misaligned pointers with `unsafe`: crypto's
  generic XOR reads a misaligned `[]byte` as `[]uintptr` on amd64 and arm64. A misaligned access
  is undefined behavior in C++, and UBSan (part of `-asan`) stops the program there. The ADR of
  step 16.2 picks between element types with `aligned(1)` for pointers to numbers in the GC
  engine (plain moves on amd64 and arm64) and `-fno-sanitize=alignment` with a documented risk.
  `-fwrapv` and `-fno-strict-aliasing` are already on for every compile
  (`internal/build/flags.go`).

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
- Scale: one thread per connection (ADR-007). Tens of thousands of connections are fine. With
  Linux's default `vm.max_map_count` (65,530) and without `MADV_GUARD_INSTALL` stack guards
  (Linux 6.13 and glibc 2.42), a program stops at about 20,000 to 30,000 threads (Phase 08's
  estimate). Each blocked connection thread also keeps about 37 KB of bdwgc's free lists and
  adds to every stop (§2.5).

### 2.10 Build, cache and size

- The engine, the collector's flags and the sanitizer are part of the key of every cached
  object. A Go package is translated once for each version and set of flags, and cached by
  content hash like a gp package.
- A first build of a gin program translates and compiles about 675k lines of Go: gin's 300k
  non-standard lines in 101 packages and 375k standard lines in 203 packages (about 190k + 370k
  with `-tags nomsgpack`). Expect a cold build in minutes, not seconds. Step 16.1 measures it,
  step 16.2 sets the budget, and later builds use the cache. The GC engine's standard library
  can be prebuilt per toolchain.
- Release builds use `-O2 -flto -g0` (ADR-136; `-strip` is a separate flag), and every link
  already uses `--gc-sections` with `-ffunction-sections -fdata-sections`, so the unused
  functions of big packages are dropped. Methods are different: Go's linker keeps a method only
  if an interface call or a `reflect` method lookup can reach it, while the C++ linker keeps
  every method that a method table points to. Without a pruning pass at link time, binaries are
  bigger than Go's, and the size budget of step 16.2 assumes that there is none.

### 2.11 Tools

- **An error in a Go package is gocpp's problem, not the user's**, since the package already
  builds with `go`. There is one report per package with everything that blocks it (assembly
  without a fallback, cgo, an unknown `//go:linkname`, a missing native) and the fix if there is
  one (`-tags purego`).
- **`gocpp test`** runs a Go package's own `_test.go` tests: internal and external test
  packages, `TestMain`, and examples with `// Output:`, with the GC engine's `testing`. This is
  how M10 is proved.
- **`go.mod` and `go.sum`.** Go modules come from `GOPROXY` (`proxy.golang.org` by default,
  `direct` through git) and are checked against the checksum database (`sum.golang.org`).
  `GOPRIVATE`, `GONOSUMDB`, `GONOPROXY` and `GOINSECURE` mean what they mean for Go. Versions
  are chosen by MVS (step 15.14 already reads `go.mod`). A module whose `go` line is newer than
  gocpp's Go version is refused, and the message names both versions.
- **Language versions.** Loop variables follow each file's language version as go/types reports
  it (`types.Info.FileVersions`): the module's `go` line, or the file's own `//go:build go1.N`
  (never below go1.21). Before go1.22 a loop has one variable, from go1.22 on one per iteration.
  Every M10 library is older than go1.22 (pflag 1.12, cobra 1.15, BurntSushi/toml 1.18; yaml.v3
  and uuid have no `go` line, which means go1.16), so both rules are on M10's path. `GODEBUG`
  defaults are computed as `cmd/go` computes them: from the main module's `go` line (for a gp
  main module, the pinned Go version), its `godebug` lines and the main package's `//go:debug`
  lines.
- **`gocpp vet`** warns at the boundary: a copy that may surprise, a func literal that may close
  a cross-engine cycle (§2.6). `gocpp explain` and `emit -annotate` show each conversion and its
  cost, and which engine each standard import comes from (§2.3).
- **Editor**: hover, completion and go-to-definition on Go packages from gp files, through the
  gp view; a definition opens the `.go` file in the module cache. The editor loads gin's closure
  (675k lines) and its gp view, so step 16.2 gives it a budget next to Phase 11's. `#line` for
  `.go` files, and debugger printers for the GC engine's strings, slices, maps and interfaces.
- The vet checks, `explain`, `emit -annotate`, the editor and the debugger printers are step
  16.20.

## 3. Steps

The parts are sized for one person working full-time, like the other phases. Windows is not a
separate step: every runtime step runs on the real Windows PC (`GOCPP_WINDOWS_SSH`), each step in
STATE.md has a Windows item, and both milestones need Windows (your decision of 2026-10-22). The
large suites (Go's standard tests in GC mode, the module tests of M10 and M11, the load tests) get
their own opt-in make targets; they are not part of the default runs.

The order follows one fact: in the GC engine, every import is Go's own standard library. So the
compiler (Part C) is first tested on programs that import nothing, `gocpp test` comes with the
standard packages that every test needs (step 16.12), and the boundary (Part E) comes after the
standard packages that it converts.

### Part A: decide (4–6 weeks)

1. **16.1 Spike: one Go package on bdwgc, called from gp** (3–4 weeks).
   - Build bdwgc 8.2.12 from source with the flags of §2.4 on Linux (GCC, Clang) and Windows
     (llvm-mingw, on the real PC), and once per sanitizer.
   - A throwaway GC mode in the emitter, behind a hidden flag, good enough for
     `container/list`, `strings`, `strconv`, `sort` and a Go version of binarytrees that prints
     with the built-in `println` (`fmt` alone pulls 60 packages). Hand-written stubs stand in
     for `runtime`, `internal/bytealg` and `internal/reflectlite`. It covers Go-layout values,
     `gp::gc::new_`, maps in GC memory, closures, interfaces and goroutines.
   - Five boundary shapes by hand: a rooted `gp::ptr`, a string both ways, a borrowed `[]byte`,
     copy-in/copy-out of a struct, a gp closure as a Go func value.
   - Measure, on Linux and on the Windows PC: allocation speed and peak memory of binarytrees
     (GC engine, ARC engine, Go); the stop-the-world pause and the memory per thread with 1,000
     and 10,000 goroutines, blocked and running, with realistic stack depths; parks and system
     calls per second through blocking sections at 1 to 64 threads; on Windows, thread starts
     (never measured, ADR-104) and the scan of static data in each collection; the cost of each
     boundary shape; compile time per 1,000 lines; the size that the collector adds; whether
     ASan and TSan work with the per-sanitizer builds of bdwgc.
   - A dry run of Go's standard library: type-check `reflect`, `internal/runtime/maps`, `sync`,
     `time`, `os`, `net`, `net/http` and `testing` under the build constraints of §2.7, and list
     every bodyless function, every `//go:linkname` of each kind and every unsupported
     construct. That list is the input of steps 16.9 to 16.11.
2. **16.2 ADR: the design** (1–2 weeks). From the spike's numbers, decide the open points:
   - the names and the namespaces of §2.2;
   - the type descriptors: `abi.Type` layout (so Go's `reflect` is translated) or gocpp's own
     descriptors with its own `reflect`; for `abi.Type`, the scheme for offsets, `typelinks`
     and unnamed types (§2.8);
   - stage 1 of the heap scan (§2.4), when stage 2 is needed, and when bdwgc's extra byte per
     object can go;
   - misaligned `unsafe` access: `aligned(1)` element types or `-fno-sanitize=alignment` (§2.8);
   - the import rules of §2.3, and whether `"go:..."` imports exist;
   - the gp view and the allowlist of twins; the boundary table of §2.6; the list of parameters
     used only during the call and its directive; `gc.New[T]` and `gc.File`; canonical proxies;
     mirrors with forwarders; which channels cross; gp types as type arguments out of the first
     version;
   - how Go's source is delivered (download, `GOCPP_GOROOT`, or embedded);
   - the overlay and `//go:linkname` policies; whether `-race` and `-asan` cover the GC engine,
     or an exception to WORKFLOW's sanitizer rule for the GC engine's tests;
   - the budgets that §5 refers to: pauses and memory with a per-thread term, boundary costs,
     parks and system calls per second, cold and warm builds, binary size, the gin program
     against Go, the editor on gin's closure;
   - what changes in ADR-004 (extended: ARC stays for gp), ADR-007 (kept: the GC engine also
     has one thread per goroutine), ADR-035 and ADR-106 (the GC engine has Go's `reflect`),
     ADR-103 (the GC engine collects Go's cycles; its cycle report would have to follow roots and
     proxies), ADR-107 (`net/http` reaches gp through the GC engine instead of a C++ wrapper
     module; its waves for `os/exec`, signals, a poller, sockets and crypto overlap with steps
     16.10, 16.16 and 16.17, and each package that gp's library gains moves programs off Go's
     copy, §2.3), ADR-109 (the checksum database is needed now) and ADR-110 (Go modules raise
     their `go` line, so the yearly move matters more; its option 4, a language version per
     module, becomes needed for Go code, while `gocpp.mod`'s `go` line stays ignored for gp
     code); and the notes in Phase 15 §2.8.

### Part B: the engine's runtime (8–11 weeks)

3. **16.3 `gp::gc` and bdwgc** (2–3 weeks): the vendored library with its notice, built into the
   runtime cache per toolchain and sanitizer; `gp::gc::heap` (§2.4) with roots, finalizers on
   demand and the finalizer goroutine, cleanups after the Go finalizer (Go's rule), weak handles
   per (base, offset), allocation counters, `GOGC` through `GC_set_min_bytes_allocd`, the soft
   memory limit, and memory returned with `USE_MUNMAP`. Tests in `runtime/test` on GCC, Clang
   and Windows: roots, the order of finalizers and cleanups (a cleanup runs only after the
   finalizer and a second unreachability), several cleanups per object, cleanups on interior
   pointers, weak links cleared before cleanups run, weak equality after the object is gone and
   after its address is reused, the heap shrinking after `FreeOSMemory`, statistics.
4. **16.4 Threads and stops** (3–4 weeks):
   - Registration in the thread start, for the timer thread, the poller thread and the
     finalizer goroutine, and for foreign threads in the boundary stub; unregistration on every
     exit path (a worker's return, the idle timeout, `exit_thread`, the thread-exit callbacks);
     `GC_register_altstack`; the choice of signals (`SIGSYS` under `-race`).
   - Blocking sections around parking, `Sleep`, idle threads, timed waits and blocking I/O
     (`waiter.cpp`, the Linux and Windows `thread.cpp`, `posix_file.cpp`, `win_syscall.cpp`);
     `syscall.Syscall` and `Syscall6` as one native; the callback rules of §2.5.
   - An audit of the runtime: every structure that can hold a GC pointer is GC-allocated or
     rooted for its whole life (`go` tasks, spilled defer records, channel buffers, timer
     records, `AfterFunc` closures); rooted per-thread records instead of TLS; a rooted panic
     value.
   - Tests: a thread blocked on a channel is never signalled (signals are counted); 1,000
     threads that start, register and exit, then a collection, which must not stall; 10,000
     goroutines that allocate; a collection during a deep recursion, while a goroutine is in a C
     call, and while values sit in channel buffers and defer records; the deadlock detector
     unchanged; the same on the Windows PC.
5. **16.5 The GC engine's values** (2–3 weeks): `gp::gc::string`, `slice<T>`, `iface` and
   `eface`, `func` (one pointer to a closure object), `map` and `chan` pointers, arrays, and
   structs in Go's exact layout (§2.2: zero-size fields as `[[no_unique_address]]` members of
   distinct empty types, explicit trailing padding, a `static_assert` per field offset, strides
   from Go's sizes). `internal/check/sizes.go` gets the GC engine's sizes; `golayout.hpp` already
   has Go's sizes and `grow_cap` in `slice.hpp` Go's growth. `new_`, `make`, `append` with Go's
   growth, slicing without past-the-end pointers, zeroed pointer-free allocations, strings,
   conversions, comparisons, and nil, bounds and division checks that panic like Go. Channels are
   `chan_core` objects in the GC heap with their buffers, with an element table (size, move,
   zero, pointer bitmap) and no members that need a destructor; `select` is unchanged. Tests for
   each type, and Go's layout checked against `unsafe.Sizeof` and `Offsetof` from real Go (a
   generated table that includes the zero-size shapes of `sync`, `sync/atomic`, `weak` and
   `log/slog`).
6. **16.6 Panics, `defer`, `recover` and `Goexit` across the engines** (1 week): one exception
   type; the panic value rooted while it unwinds; `recover` on either side converts, and a value
   that crosses back is the same value; `Goexit` runs both sides' defers; runtime errors have
   Go's messages and `runtime.Error` types on the GC side; a goroutine's panic exits with code 2,
   as today.

### Part C: the compiler (7–10 weeks)

7. **16.7 Loading Go packages and modules** (2–3 weeks):
   - `.go` packages through the stock `go/parser` (Go files have no gp syntax); build
     constraints with the tags of §2.7 (`//go:build`, the old `// +build`, file name suffixes,
     the pinned version's tool tags, `-tags`); `_test.go` files; `//go:embed` (the gp loader's
     code); cgo files dropped as with `CGO_ENABLED=0`, or an error that names the package if
     nothing is left; `.s` files ignored; the directives (`//go:linkname` of each kind,
     `//go:noescape`, `//go:nosplit`, `//go:debug`, …).
   - The module rules of §2.3: requirements by MVS (step 15.14), with the main module's
     `replace`, `exclude` and `retract`; `gocpp.mod` and `go.mod` side by side; the standard
     library's vendored packages under their `vendor/` paths.
   - Fetching through the `GOPROXY` protocol and `direct` (git, as `gocpp get` already does),
     checked against `GOSUMDB`, with Go's meaning of `GOPRIVATE`, `GONOSUMDB`, `GONOPROXY` and
     `GOINSECURE`; the hashes go in `gocpp.sum`, as the same `h1:` lines as `go.sum`.
   - Language versions (§2.11): each file's version for go/types (`FileVersions`); the refusal of
     a module that needs a newer Go than gocpp (`github.com/x/y@v1.2.0 requires go >= 1.26;
     gocpp is go1.25`); `GODEBUG` defaults as `cmd/go` computes them.
   - Go's source for the standard library: the pinned archive, downloaded once into the cache,
     or `GOCPP_GOROOT` with a matching `VERSION`.
8. **16.8 GC-mode lowering and emission** (5–7 weeks):
   - The engine of a package flows through `internal/lower` and `internal/cppgen`: types
     (`cxxType`) in Go's exact layout, allocation and boxing (the escape analysis decides stack
     or GC heap), closures (a GC environment), method values, interfaces and itabs, maps (calls
     into `internal/runtime/maps`), strings, `defer` and `recover` (the shared frames), `go`,
     channels, `select`, generics (C++ templates over GC types), range over functions, loop
     variables by file version (one per loop before go1.22), and `unsafe` with Go's rules
     (`Pointer` arithmetic, `Add`, `Slice`, `String`, `StringData`, `SliceData`, header casts,
     misaligned access as step 16.2 decides).
   - What the ARC engine gates but Go code needs: generic types declared inside functions
     (hoisted with their own type parameters plus those of an enclosing generic function, and
     named as gc names them: `%T` prints `main.pair[string;int]`), `//go:linkname` of each kind
     (§2.7), the compiler's intrinsics, all of `unsafe`. The gate table
     (`internal/check/gates.go`) gets an engine column.
   - Type descriptors in `abi.Type` layout, with `GCData`, names and method tables, and the
     scheme of step 16.2 for offsets, `typelinks` and unnamed types.
   - `#line` for `.go` files; the names `gopkg_<escaped path>`.
   - Tests: Go's own `test/` programs that `tools/gotest` runs (step 15.10), built in GC mode as
     Go `main` packages, with gc's expected output. First the ones that import nothing (55 of
     the 138 top-level `// run` programs), with the spike's stubs until steps 16.10 and 16.11;
     the others in step 16.12. The ARC engine's output is unchanged: the C++ of the examples is
     byte for byte what it was before Phase 16, and a program without Go packages links no
     bdwgc symbol.

### Part D: Go's standard library on the GC engine (12–17 weeks)

9. **16.9 The replacement checker** (1 week): `gostd/` (overlays as `.go` files and C++ natives,
   the runtime, module overlays, the tool tags per Go version, each native's Go signature and
   each overlay's original-file hash) and `tools/gostdcheck`, which lists every bodyless
   function, every `//go:linkname` of each kind, every intrinsic and every assembly-only path in
   the pinned Go source under gocpp's constraints, for Linux and Windows on amd64 and arm64. It
   fails on one without a native or an overlay, and on a changed signature or original file. It
   runs on every Go version move and point release.
10. **16.10 `runtime` and the hook table** (3–4 weeks):
    - The GC engine's `runtime`: the public API of §2.4 and §2.7; `runtime/debug`
      (`SetGCPercent`, `SetMemoryLimit`, `FreeOSMemory`, `ReadBuildInfo` from the module graph
      that gocpp built, `Stack`, `PrintStack`); `runtime/trace` and `runtime/pprof` build and
      report that they are not supported.
    - Natives for the 169 runtime hook targets (151 pushes and 18 pulls; the dry run of
      2026-10-24) and for the runtime's pushes:
      - `sync` and `internal/sync`: semaphores, spinning and `notifyList`. `procPin` is a
        per-slot lock. The pools are emptied before the collections that gocpp starts, as in
        Go, outside the stop and outside bdwgc's lock, with all slots held; collections that
        bdwgc starts by itself skip it. Go's `sync.Pool` assumes that no collection happens
        while a goroutine is pinned, and bdwgc can stop a thread at any instruction.
      - `time`: `now`; timers on the shared timer thread, held through weak links so that an
        unreferenced `Timer` is collected (Go 1.23's rule); one `startNano` for both `time`
        packages.
      - `internal/poll`: stubs that say "not pollable" until 16.16.
      - `syscall`: `Syscall` and `Syscall6` in a blocking section (§2.5).
      - `os` (arguments, environment, `runtime_beforeExit`), `internal/godebug` (the defaults of
        §2.11), the per-goroutine indicator of `crypto/internal/fips140`, `iter` (`newcoro` and
        `coroswitch` from step 7.10), `weak` and `unique`.
      - `internal/synctest` and the runtime's bubble checks, which `sync.WaitGroup.Add`,
        `time.Since` and `time.Until` call: natives that say "no bubble". `testing/synctest.Test`
        skips with a reason.
    - The Windows natives: `loadlibrary`, `getprocaddress`, `SyscallN` and `Syscall6` to
      `Syscall18`, `compileCallback`, `QueryPerformanceCounter`, and the rest of the 17 bodyless
      functions that only Windows has.
    - The overlays of §2.7.
11. **16.11 `reflect`, maps and interfaces** (3–4 weeks): Go's `reflect`, `internal/reflectlite`,
    `internal/abi` and `internal/runtime/maps` translated, with the scheme of step 16.2 for
    offsets and `typelinks`; call thunks for `Value.Call` and `Method` (an overlay for
    `reflectcall`, the thunk found from the code pointer); `MakeFunc` and a signature without a
    thunk panic with a clear message. Go's own tests of `reflect` and `internal/runtime/maps`
    pass by the counting rule of §5; the gc-specific ones (`TestFuncLayout`, `TestGCBits`,
    `TestTypelinksSorted`) and the tests of `MakeFunc` are on the list with their reasons.
12. **16.12 The bootstrap closure and `gocpp test` for Go packages** (2–3 weeks): the standard
    packages that every test needs build and run on the GC engine: `runtime`, `sync`, `time`,
    `os`, `syscall`, `reflect`, `fmt`, `flag`, `regexp`, `strings`, `strconv`, `io`, `errors`
    and `testing`, with gocpp's `testing/internal/testdeps` (§2.7). Then `gocpp test` for Go
    packages: internal and external test packages, `TestMain`, examples with `// Output:`,
    benchmarks, fuzz seeds run as tests, `testdata/`, `-run`, `-v`, `-count`, `-short`,
    `-timeout`, `t.Parallel`; gp and Go packages in one run; and the rest of Go's `test/`
    programs (step 16.8). A comparison mode for the conformance work runs the same package with
    `go test` and `gocpp test` and compares the results (a tool for 16.13 and 16.15, not for
    users).
13. **16.13 The standard packages that M10 needs** (3–5 weeks):
    - Go's own tests (from Go's source) of `errors`, `fmt`, `strings`, `bytes`, `strconv`,
      `unicode/...`, `sort`, `slices`, `maps`, `container/...`, `math`, `math/big`,
      `encoding/...` (`json`, `base64`, `hex`, `binary`, `xml`), `bufio`, `io`, `io/fs`, `os`,
      `path/filepath`, `time`, `sync`, `context`, `regexp`, `text/template`, `go/token`,
      `embed`, `testing`, `flag`, `log`, `hash/...`, `crypto/{md5,sha1,sha256,rand}`, and `net`
      without the netpoller: parsing, `net/netip`, and `net.Interfaces` (netlink on Linux,
      `GetAdaptersAddresses` on Windows), which uuid's tests need.
    - What the libraries' tests also use: `go/parser` and `go/printer` (`gopkg.in/check.v1`),
      `testing/fstest` and `net/mail` (BurntSushi/toml), `encoding/csv` (pflag) build and run;
      `net/http`, `crypto/tls`, `go/build` and `go/format` build and initialize, without
      sockets, because `x/text`'s test helper imports them (`internal/testtext` imports
      `internal/gen` to call `gen.IsLocal()`). In all, M10's test code needs 223 standard
      packages, against 122 for the libraries alone.
    - The reviewed list may drop whole test files (those that pull runtime test hooks with
      `//go:linkname`), and the tests run with no `go` command on PATH, so `internal/testenv`
      skips the ones that build helper programs. Each failure is fixed or listed with a reason.

### Part E: the boundary and M10 (7–11 weeks)

14. **16.14 The gp view and the boundary** (4–6 weeks):
    - The gp view of every Go package that gp code imports, and of its API closure (also the
      standard packages that only Go has, such as `net/http`): its declarations type-checked
      again against gp's standard library (function bodies skipped), with the twins of the
      allowlist substituted. One importer per world plus a gp-view importer; view types mapped
      to their GC types by (path, name, type arguments); the GC engine's sizes for
      `unsafe.Sizeof` constants; the engine of each expression tracked next to go/types (§2.6,
      rule 1). A declaration that does not check in the gp view gets a reason, reported only
      when gp code uses it. A tool checks the allowlist of twins.
    - Escape summaries for string, slice and interface parameters (contents and results), next
      to ADR-114's summaries for `*T` parameters; the list of parameters used only during the
      call, and its `gocpp.mod` directive.
    - Every row of §2.6, both ways, generated per call site and per type: roots
      (`gp::bridge::hold`, one per object), borrowed, copied and copied-back strings and slices,
      Go memory in place, mirrors with forwarders (synthesized Go source), copy-in/copy-out over
      graphs with its memo, `gc.New[T]` and `gc.File`, canonical proxies, gp type descriptors
      for Go types, errors, sentinels and twin error types, functions, the context adapters,
      twins (`time` with one monotonic base, the `io` interfaces, named numbers), channels of
      one layout, panics and `Goexit`; gp's `fmt` and `json` handing Go values to Go's;
      `Caller`, `Callers` and `FuncForPC` across the thunks; GC weak links for gp's
      `weak.Pointer` to Go objects; the count of ARC bytes behind proxies.
    - A Go pointer in a non-escaping gp local stays a plain pointer; one stored in ARC memory
      becomes a root.
    - Tests: one run test per row and direction, an error test for every refused crossing,
      `c.JSON` of a gp struct, `c.Keys` and pflag's `*string` in place, `io.ReadAll` of a Go
      reader into gp, a cycle test (documented as a leak, then broken with `weak.Pointer`), a
      stress test of roots from many threads.
15. **16.15 M10: pure-Go libraries pass their own tests** (3–5 weeks).
    - `gopkg.in/yaml.v3`, `github.com/BurntSushi/toml`, `golang.org/x/text` (`cases` and what it
      needs), `google.golang.org/protobuf` (`proto`, `protojson`, `prototext`),
      `github.com/google/uuid`, and `github.com/spf13/cobra` with `spf13/pflag`: their
      `_test.go` tests pass by the counting rule of §5 with `gocpp test` on Linux (GCC, Clang)
      and on the Windows PC.
    - Their tests bring modules of their own: `gopkg.in/check.v1` (yaml.v3),
      `github.com/google/go-cmp` (protobuf; it reaches `reflect.NewAt` over `unsafe`) and about
      68 generated protobuf test packages. With their tests, the libraries grow from about 81k
      to about 271k lines. cobra's `TestDeadcodeElimination` needs the `go` command, and
      `cobra/doc` (go-md2man, blackfriday) is not in scope.
    - A gp program uses each of them through the boundary: YAML and TOML into gp structs, a
      protobuf message, a cobra command tree whose flags are read after `Parse` (pflag's
      pointers in place).
    - Build time, binary size and test time are recorded next to `go test`'s.

### Part F: M11 (10–14 weeks)

16. **16.16 The netpoller** (2–3 weeks): §2.9 on Linux (`epoll`) and Windows (IOCP), the real
    `internal/poll` hooks, deadlines, 10,000 idle connections, and the tests of Go's
    `internal/poll` and `net` that use local sockets.
17. **16.17 `net`, `crypto`, `crypto/tls`, `crypto/x509`** (3–4 weeks):
    - `net`: TCP, UDP, Unix sockets, the pure-Go resolver.
    - The crypto tree with `purego` (it builds since 16.13, with the FIPS 140 module as plain
      Go and its `go:fipsinfo` symbol) and its tests; `crypto/tls`; `crypto/x509` with the
      system's root certificates (files on Linux, the certificate store through `syscall` on
      Windows).
    - The module overlays for `golang.org/x/sys` (`unix`, `windows`, `cpu`; §2.7), also for the
      copies that the standard library vendors.
    - Go's tests of these packages pass by the counting rule of §5. TLS works with `curl`,
      `openssl s_client` and a Go-built peer.
18. **16.18 `net/http`** (2–3 weeks): server and client, HTTP/1.1 and HTTP/2 (Go's bundled
    `h2_bundle.go`), `httptest`, `httputil`, `mime/multipart`, `compress/gzip`. Go's `net/http`
    tests pass by the counting rule of §5 (the tests that use `testing/synctest` skip with that
    reason); a gp program serves and fetches.
19. **16.19 gin on Linux and Windows** (3–4 weeks): gin v1.12.0 (quic-go, `x/net` and `x/sys`
    through the module overlays) and v1.9.1; `-tags nomsgpack` documented as the smaller build;
    the runtime functions that `ugorji/go/codec` pulls (§2.7); the program of §2.1; gin's own
    tests by the counting rule of §5; `chi`, `gorilla/websocket` and `echo` as checks of the same
    stack; the load test of §5 against the same program built with Go; the budgets of step 16.2
    measured against Go and recorded in `docs/benchmarks/gc-engine.md`.

### Part G: finish (4–8 weeks)

20. **16.20 Tools** (2–3 weeks): the tools of §2.11: the boundary checks of `gocpp vet`,
    `explain` and `emit -annotate` for conversions and engines, the editor on Go packages through
    the gp view within the budget of step 16.2, and debugger printers for the GC engine's values.
21. **16.21 Speed and size** (1–3 weeks): the precise heap (stage 2 of §2.4, with gcj-style
    allocation or a patched kind) if steps 16.1, 16.15 or 16.19 showed that false retention or
    slow marking matters; whether bdwgc's extra byte per object can go; tuning against the
    numbers of 16.19.
22. **16.22 Docs, packaging and the list of tested modules** (1–2 weeks): a guide "Using Go
    packages from gp" (the rules of §2.6, the errors and their fixes, the engine moves of §2.3),
    "How the GC engine works", the tested modules with versions and skip lists, and the GC
    engine's column in `docs/spec/feature-matrix.md`. Packaging (step 15.13) ships bdwgc's
    source and notice, fetches Go's source archive, and can ship a prebuilt GC-engine standard
    library.

## 4. Deliverables

- The GC engine's runtime: `gp::gc` over a vendored bdwgc, Go-layout values, thread
  registration and unregistration, blocking sections, finalizers, cleanups, weak pointers, the
  netpoller.
- The compiler: loading `.go` packages and Go modules (`GOPROXY`, the checksum database, the
  `go` line and file versions, `GODEBUG`), GC-mode lowering and emission, the gp view, the
  generated boundary.
- `gostd/`: the GC engine's `runtime`, the hook table, the overlays, the module overlays, the
  tool tags per Go version, and `tools/gostdcheck`.
- Go's standard library on the GC engine, tested with Go's own tests.
- `gocpp test` for Go packages, and opt-in make targets for the large suites.
- The tools: vet checks, `explain`, the editor and the debugger printers for Go packages.
- The M10 and M11 programs and suites, an ADR, `docs/benchmarks/gc-engine.md`, two guides, the
  list of tested modules and the packaging.

## 5. Exit criteria

**The counting rule** (Phase 09's, ADR-113). A skipped test counts as failing unless its reason
is a documented divergence: the internet, the `go` command, gc's runtime internals (test files
that pull runtime hooks, gc's register ABI and GC bits), gc's exact allocation counts, timing,
or `testing/synctest` bubbles. A standard package passes with at least 90 % of the test
functions in its selected files, a module package with at least 95 %.

**M10: pure-Go libraries pass their own tests**

- The GC engine's runtime tests pass on Linux (GCC, Clang) and on the Windows PC (llvm-mingw):
  roots, the order of finalizers and cleanups, weak and `unique`, registration and
  unregistration (threads that exit never stall a collection), blocking sections (a thread
  blocked on a channel is never signalled), runtime structures that hold GC pointers, 10,000
  goroutines that allocate.
- The stop-the-world pause and the memory per thread are measured and recorded with 10,000
  goroutines, blocked and running, on Linux and on the Windows PC, and stay under the budgets
  that step 16.2 sets.
- Go's own tests of the standard packages of step 16.13 pass by the counting rule.
- The libraries of step 16.15 pass their own tests by the counting rule on Linux and on the
  Windows PC, and a gp program uses each of them through the boundary.
- The boundary suite is green: every row of §2.6, both ways, and an error test for every refused
  crossing.
- A gp program that imports no Go package builds to the same C++ as before Phase 16 and links no
  collector (the test of step 16.8).

**M11: `net/http`, TLS, the netpoller and gin on Linux and Windows**

- Go's tests of `net`, `net/http`, `crypto/tls` and `crypto/x509` pass by the counting rule.
- 10,000 idle connections, and a load test of 60 s at 1,000 concurrent keep-alive connections
  with no errors, on Linux and on the Windows PC. A deadline or a close wakes exactly the waiters
  it should (tests).
- An HTTPS server built by gocpp talks to `curl` and to a Go-built client that trust a test CA,
  on Linux and Windows. A gocpp client verifies a saved public certificate chain against the
  system's root certificates (the files on Linux, the Windows verifier), without the internet.
- gin v1.12.0 and v1.9.1: the program of §2.1 serves requests on Linux and Windows, and gin's
  own tests pass by the counting rule.
- The gin program, against the same program built with Go (both release builds): requests per
  second, p99 latency, memory and GC pauses within the budgets of step 16.2, recorded in
  `docs/benchmarks/gc-engine.md` (step 16.19).
- Cold build time, warm rebuild time after an edit of `main.gp` (the target for a debug build is
  2 s) and binary size within the budgets of step 16.2.

**Phase done**

- M10 and M11 are proved, the ADR is accepted, the tools of step 16.20 work on the gin program
  within the editor budget, and the guides, the packaging and the list of tested modules are
  published.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Stopping the world with thousands of OS threads: bdwgc signals each running thread, scans every registered thread's stack and keeps each thread's free lists (with a tiny heap: 0.6 ms and 39 MB with 1,000 blocked threads, 22 ms with 10,000) | Every wait that can last is a blocking section (§2.5), so only running threads are signalled; a per-thread term in the budgets; long waits flush the free lists, or idle workers unregister; measured with 10,000 threads in 16.1 and 16.4, on Linux and Windows; the limit documented (tens of thousands of connections, ADR-104) |
| bdwgc's global lock: blocking sections, weak-link reads and typed allocation take it (a blocking section costs 9 to 19 µs at 32 to 64 threads) | Only waits that can last are wrapped; system calls go through one native; parks and system calls per second measured in 16.1; a lock-free patch to the vendored bdwgc as the fallback; gcj-style allocation for stage 2 |
| Long pauses on big heaps: no concurrent marking, 16 markers by default; hundreds of milliseconds and more on heaps of several GB (an estimate; step 16.1 measures it) | Parallel marking; budgets measured on gin-sized heaps; `gp::gc` keeps the backend replaceable (an own collector, Go's, or Whippet once it supports Windows) |
| False retention from conservative scanning | Pointer-free objects are never scanned (stage 1); a precise heap (stage 2) when the numbers ask for it; stacks stay conservative, as in gccgo |
| GC pointers where the collector does not look: runtime structures in `malloc` memory, threads that exit while registered | The audit of step 16.4, unregistration on every exit path, and tests that collect while values sit in channel buffers and defer records |
| Surprises at the boundary (copies, snapshots, proxies, Go memory held by gp code) | A short list of rules (§2.6): Go memory kept in place, an allowlist of twins, a list of call-only parameters, refusals with a named fix instead of silent copies; `gocpp vet` warnings; a guide |
| Cross-engine cycles leak, and they are common (a gin handler that captures `r`) | gp's `weak.Pointer` to a Go object breaks them; `gocpp vet` warns about the common shape; documented, the same class as ARC cycles today |
| A package that gp's library gains later moves programs from Go's copy to gp's (§2.3, your decision of 2026-10-22) | The release notes name each such package; `gocpp explain` shows the engine of each standard import; the ADR of step 16.2 decides whether `"go:..."` imports exist |
| Go's layout and `unsafe` in C++: zero-size fields, misaligned access | `[[no_unique_address]]` members and a `static_assert` per offset (16.5); `aligned(1)` types or `-fno-sanitize=alignment` (16.2); the MSVC ABI is out for the GC engine |
| Build time and binary size: two standard libraries and big module graphs (gin's closure is about 675k lines); method tables keep every method | Content-hash cache per package, a prebuilt GC-engine standard library, parallel builds, `--gc-sections` and LTO in release builds, budgets from 16.1 that assume no method pruning |
| Go modules need a newer Go than gocpp's (go-redis v9.23 already needs Go 1.26, and so do `x/net` v0.61.0 and quic-go v0.64.1, which newer gin releases may require) | A clear refusal that names both versions; the yearly version move of ADR-110 |
| Code that cannot be translated: assembly without a fallback, cgo, `//go:linkname` into runtime internals, generated unsafe code (`modernc.org/sqlite`) | One report per package with the reason; build tags (`purego`, `nomsgpack`, `codec.safe`); module overlays for `x/sys`; a published "does not work" list |
| Signals: a C library that does not retry on `EINTR`, a thread that blocks the collector's signals (bdwgc retries for a while, then aborts with "Signals delivery fails constantly") | Real-time signals chosen and documented (`SIGSYS` under `-race`); gocpp's own system calls retry; foreign threads unblock the signals when they register |
| ASan, TSan and LeakSanitizer with a conservative collector | bdwgc built per sanitizer; `SIGSYS` under TSan; `-asan` code compiled with `-fsanitize-address-use-after-return=never` (GCC: `--param asan-use-after-return=0`), because ASan's fake stacks are not scanned; the GC heap registered with LeakSanitizer (`__lsan_register_root_region`); tried in 16.1. If one still does not work, the ADR records an exception to WORKFLOW's sanitizer rule for the GC engine's tests |
| Windows: bdwgc with threads, `SuspendThread` with llvm-mingw, and static roots found again in each collection (`VirtualQuery` over the address space and every DLL's writable data) | Tested from 16.1 on, on the real PC; LLGo and Crystal ship bdwgc with threads on Windows (§7.2); `GC_set_no_dls` plus the executable's own sections if the scan costs too much |
| Some declarations of a Go package do not check in the gp view (gp's standard library lacks an API) | Only those symbols are unusable from gp, with the reason; the Go side is not affected |
| Overlays and natives drift with each Go version: go.dev/issue/67401 keeps only some hooks stable (`notifyListWait`, `pollWait`, `time.newTimer` and `synctest.IsInBubble` can change) | `gostd/` records each native's Go signature and each overlay's original-file hash, and `tools/gostdcheck` fails when either changes; point releases get the same check |
| Licenses | bdwgc's notice ships with the others; with `GC_BUILTIN_ATOMIC`, `libatomic_ops` is not needed; Go's source is BSD-3, like the gp ports |

## 7. Evidence (2026-10-21, checked 2026-10-22)

### 7.1 Survey of 17 popular Go modules

Each module was resolved with its dependencies, and every package was type-checked with the
go/types of go1.25.7 (433 packages for the 17 modules and 253 for gin v1.9.1, no type errors)
for linux/amd64 with `CGO_ENABLED=0` and gc's build tags. The GC engine uses the `gocpp` tag
instead (§2.7), which changes a few files: ugorji's runtime linknames go from 20 to 10.
"Missing std" counts the standard packages that a port to gp's standard library would have to
add: the walk stops at packages that gp's library has, so the count is lower than the number of
the closure's standard packages that gp lacks (for gin v1.12.0, 122 against 141). "Plain %" is
the share of non-standard lines in files that use no `unsafe` at all, no `//go:linkname`, no
assembly and no cgo, and the second number is with the tags
`purego,noasm,appengine,safe,codec.safe,disableunsafe,nounsafe`.

| Module | Version | Non-std pkgs / lines | Missing std pkgs / lines | net, http, tls | Plain % | Main blockers |
|---|---|---|---|---|---|---|
| gin | v1.12.0 | 101 / 300,472 | 122 / 195,169 | yes, yes, yes | 95.8 (96.4) | `ugorji/go/codec` (112,685 lines: 20 runtime linknames, 10 under the `gocpp` tag, `ArrayOf`, `MakeChan`, `Select`), quic-go, `x/sys`, `bytesconv` |
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

go-redis, sqlite and `x/text` are at the newest versions that support Go 1.25; every later
release needs Go 1.26. bubbletea v2 and echo v5 (new module paths) were not surveyed.

What the survey found:

- **The standard library is the biggest blocker**: 14 of 17 modules need standard packages that
  a port would have to add (140 packages and 212,780 lines for all 17 together). The crypto tree
  alone is 65 packages: even `crypto/rand`, `sha1` or `md5` pull about 25 crypto packages, 15 of
  them in the FIPS 140 module.
- **`net`** is needed by 11 modules and **`net/http`** by 6. The `net/http` + `crypto/tls` closure
  is 176 packages (the dry run of 2026-10-24; `go list -deps` gives 177 with `unsafe`, which is
  not a package, and the plan's earlier 178 came from the one-off survey) and 324,441 lines (a
  port would have to add 103 of them, 157,885 lines; gp lacks 123 in all). Its standard packages
  reach **303 distinct two-argument `//go:linkname` targets**, 169 of them runtime hooks (151
  pushed by the runtime, 18 pulled from it); by package, 40 for `reflect`, 18 for `sync`, 12 for
  `syscall`, 11 for `internal/poll`, 7 for `time`, and more (§2.7). The plan's earlier
  166/149/17 and 37/18/15/12/7 do not reproduce under any recorded definition; the dry run's
  numbers replace them.
- **`unsafe` beyond `Sizeof`** appears in 8 modules, zero-copy `string` ↔ `[]byte` in 6, and
  `//go:linkname` in 5. Outside `x/sys`, only gin (20 pulls from ugorji into runtime internals,
  10 under the `gocpp` tag, and 2 from `x/net` into `syscall`) and sqlite (7 into `math/big`'s
  assembly) use `//go:linkname`.
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
- **Tests need more than the libraries** (counted with `go list -deps -test` on 2026-10-22). With
  their `_test.go` files and their test-only modules (`gopkg.in/check.v1`,
  `github.com/google/go-cmp`, about 68 generated protobuf test packages), the M10 libraries grow
  from about 81k to about 271k lines, and the standard packages that they need from 122 to 223.
  `x/text`'s test helper `internal/testtext` imports `internal/gen` only to call `gen.IsLocal()`,
  which pulls `net/http`, `crypto/tls`, `go/build` and `go/format`; uuid's `TestNode` needs a
  real `net.Interfaces`.

### 7.2 Prior art

| Project | What it does | What Phase 16 takes |
|---|---|---|
| **LLGo** (Go through LLVM) | conservative bdwgc by default (the system's libgc, 8.0 or later); goroutines 1:1 on OS threads; claims the whole standard library on Linux, macOS and Windows (MSVC and MinGW); per-thread `poll()` with one shared wake pipe on Linux, an IOCP dispatcher on Windows | the closest precedent: threads + bdwgc work; root per-goroutine state instead of TLS; avoid the shared wake pipe |
| **TinyGo** | bdwgc on Linux, macOS and Windows hosts, but built without `GC_THREADS`: every call into bdwgc is serialized with a lock, and TinyGo stops the world itself; a thread per goroutine on Linux since 0.38, one thread on Windows; precise layouts with Boehm since 0.43 (2026-10-09); its own `reflect` (no `Call`) and a replaced `runtime`, `reflect`, `net/http` | not a precedent for bdwgc with threads; incremental mode off (so that bdwgc leaves `SIGSEGV` alone); precise layouts as a second stage; its replaced packages show where Go's own code is hard |
| **gccgo / libgo** | Go's runtime reused, with conservative stacks and a precise heap; the real epoll netpoller. In 2010 its first stop-the-world collector "works but it is far too slow"; in 2011 a thread per goroutine was "inefficient for programs with lots of goroutines", and goroutines moved onto fewer threads | conservative stacks are workable; the thread cost is the known price of ADR-104 |
| **go2cpp** (Hajime Hoshi; archived, last commit December 2021) | Go's Wasm output turned into C++: "bad performance and long compiling time", GC pauses on one thread; replaced by the real Go compiler with overlays (Hitsumabushi) | build time is a first-class budget (§5) |
| **GopherJS** | `go/types`-checked Go as JavaScript; standard packages augmented with "natives" overlays | the overlay mechanism of §2.7 |
| **Nim** | the memory manager is a per-program switch (`--mm:orc`, `arc`, `boehm`, `go`, …) | the engine is a property of the code, chosen once |
| **Crystal** | bdwgc (8.2+) with threads (`GC_pthread_create`, `GC_beginthreadex`) and fibers | bdwgc's thread API on Linux and Windows in a production language |
| **Whippet** (Andy Wingo, MIT) | an embeddable collector library (parallel, generational, conservative or precise roots), with a bdw backend; no Windows or macOS support yet | the shape of the `gp::gc` interface; a later backend |
| **MPS** (Ravenbrook) | an incremental collector; Windows with MSVC only, and it needs to own `SIGSEGV` | not a fit today |

bdwgc facts used above, checked in the 8.2.12 source: release 8.2.12 (2026-02-05), canonical
repository github.com/bdwgc/bdwgc; `GC_do_blocking` leaves a thread out of the stop and scans its
stack from where it blocked, and takes the global allocation lock twice per call; the stop
handshake is linear in the running threads; 16 marker threads by default (`MAX_MARKERS`, a
build setting); with a collection time limit, incremental collection mostly avoids the parallel
marker; on Linux, a thread that registered itself must unregister, or the next collection
stalls and aborts with "Signals delivery fails constantly" (tested); `GC_malloc_atomic` does
not clear memory; finalizers can run on demand from one thread, and an object keeps only the
last finalizer registered for it; short disappearing links match Go's `weak` rule;
thread-local storage is not a root; `REGISTER_LIBRARIES_EARLY` exists only with glibc, so on
Windows each collection finds the static roots again. LLGo, TinyGo and Crystal each use another
bdwgc version; none pins 8.2.12.

Sources (read 2026-10-21, checked again 2026-10-22):

- bdwgc: https://github.com/bdwgc/bdwgc/tree/v8.2.12 (`doc/gcdescr.md`, `doc/scale.md`,
  `doc/finalization.md`, `doc/README.macros`, `include/gc.h`, `include/gc_typed.h`); V's
  slowdown without `THREAD_LOCAL_ALLOC`: https://github.com/vlang/v/issues/27486 (fixed by
  #27544)
- LLGo: https://github.com/xgo-dev/llgo (formerly `goplus/llgo`; `README.md`,
  `doc/defer-tls-gc.md`, `runtime/internal/lib/runtime/poll_linkname_llgo.go`,
  `poll_windows_llgo.go`)
- TinyGo: https://github.com/tinygo-org/tinygo (`CHANGELOG.md`, `builder/bdwgc.go`,
  `src/runtime/gc_boehm.go`, `compileopts/target.go`, release v0.43.0)
- gccgo: https://go.googlesource.com/gofrontend/+/0fb550083ae474fb964435927b899ec8e4b62771,
  https://gcc.gnu.org/legacy-ml/gcc-patches/2010-07/msg01890.html,
  https://gcc.gnu.org/legacy-ml/gcc-patches/2011-12/msg01085.html
- go2cpp: https://ebitengine.org/en/blog/native_compiling_for_nintendo_switch.html
- GopherJS: https://github.com/gopherjs/gopherjs (`build/build.go`, `compiler/natives`)
- Nim: https://nim-lang.org/docs/mm.html · Crystal:
  https://github.com/crystal-lang/crystal/blob/master/src/gc/boehm.cr
- Whippet: https://github.com/wingo/whippet · MPS:
  https://memory-pool-system.readthedocs.io/en/latest/topic/platform.html,
  https://memory-pool-system.readthedocs.io/en/latest/topic/thread.html (`SIGSEGV`)
- glibc: the `NEWS` file of glibc 2.42 (`MADV_GUARD_INSTALL` stack guards in `pthread_create`)
- Go: https://go.dev/doc/gc-guide, https://pkg.go.dev/runtime (`SetFinalizer`, `AddCleanup`),
  https://pkg.go.dev/weak, https://go.dev/issue/67401 (`//go:linkname` lock-down),
  https://golang.org/issue/8404 (no past-the-end pointers)

The survey was a one-off script (`go list -deps` and go/types over the modules above, with gc's
build tags); its raw data is not kept in the repository. The review of 2026-10-22 counted the
test closures with `go list -deps -test` and checked the `gocpp` tag's effect on ugorji by hand.
