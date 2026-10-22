# Phase 16: Dual engine (a garbage-collected engine for Go, the current engine for gp and C/C++)

> **Status: idea, for discussion. Nothing here is decided and nothing is scheduled.** It was
> raised by the user on 2026-10-21 ("a non-GC engine for gp and C/C++, another GC engine for Go;
> then we can import any Go package in the world"). This page records the idea, what it would
> take, the options, and the questions that must be answered before a real plan is written.

| | |
|---|---|
| Depends on | Phase 15 (v1.0 ships first); Phases 06 (modules, `gocpp get`), 07 (generics), 08 (goroutines), 09 (stdlib), 12 (typed interop) |
| Unlocks | Importing Go packages as they are (no porting to `.gp`) |
| Milestone | none yet (after M9) |
| Size | unknown: the options below differ by an order of magnitude |
| Progress | [STATE.md](STATE.md) |

---

## 1. The idea

gocpp has one engine today: **no garbage collector**, automatic reference counting
(`gp::ptr<T>`), cycles broken by hand with `weak.Pointer` (Q1 of the plan, README §6). That is
what makes gp and C/C++ code fast, deterministic and easy to mix.

The idea: keep that engine for gp and C/C++, and add a **second engine with a tracing garbage
collector and real Go semantics**, used for packages that are written in Go. A gp program could
then import any Go module (`import "github.com/someone/lib"`) without anyone porting it.

```
 gp + C/C++ code  ──►  engine A (today): C++20, reference counting, no GC
        │ boundary (values, handles, callbacks, errors, panics)
 Go packages      ──►  engine B (new): Go semantics, tracing GC
```

## 2. Why it is not free (what breaks when Go code is moved to the engine A today)

Go packages are written for a tracing GC and the real Go runtime. The standard library already
shows the cost: each package needed a hand-made port (`portstd`, native shims, Phase 09, 14.3).
Typical things that do not survive a straight translation to reference counting:

- **Cycles** (doubly linked lists, parent pointers, graphs, caches): they leak under ARC.
- **`unsafe`** arithmetic, `uintptr` tricks, `reflect.SliceHeader`, `go:linkname` into the
  runtime, assembly files, cgo.
- **`reflect`** (gocpp has a subset), finalizers, `sync.Pool` assumptions, `runtime.GC` and
  memory stats, interior pointers kept alive by a GC.
- Go's goroutine scheduler and stack model against gocpp's own thread layer (Phase 08).
- Build time and binary size of large dependency trees through a C++ compiler.

## 2b. Direction given by the user (2026-10-21)

Go packages are **translated to C++ as well** (this is option B2 below), and all of them use the
GC. The real Go toolchain is not part of the picture, B1 stays only as a possible fallback for
packages the translator cannot handle (assembly, cgo). What follows from that:

- The transpiler gets a **memory mode** per package: engine A emits `gp::ptr<T>` (reference
  counting), engine B emits GC pointers. Slices, maps, strings, closures, channels and
  interfaces of engine B need GC-flavoured runtime types (or the same templates parameterised
  by a memory policy).
- The **standard library has to exist for both engines** (a Go package imports `strings`, `io`,
  `fmt`): translate it twice, or once with a policy parameter. This costs build time and binary
  size, and `error`, `io.Reader` and friends must be **one type** across the engines.
- The collector must scan **C/C++ stacks and registers** (C++ code holds raw pointers into GC
  objects), so a **conservative** collector is the natural first choice; a precise one needs stack
  maps that generated C++ cannot give. The collector must stop and scan every goroutine thread
  (signals on Linux, `SuspendThread` on Windows).
- **Engine A objects that point into the GC heap** need to be roots; **GC objects that own engine
  A objects** (a `shared_ptr` inside) need finalizers or handles.

## 3. Options for engine B (to be compared, not chosen)

| | Option | Idea | Good | Risky |
|---|---|---|---|---|
| B1 | **The real Go toolchain as the engine** | Build the Go packages with `go build -buildmode=c-archive`/`c-shared` and call them through a generated boundary (cgo style). The Go runtime and its GC live in the same process. | Any package, exact semantics, no translation work, no new GC | Two runtimes and two schedulers in one process; each call crosses cgo (tens to a hundred ns); no inlining across; Go must be installed (or shipped); callbacks from Go into gp go through cgo too |
| B2 | **Same transpiler, tracing-GC runtime** | Translate the Go packages with the current front-end into C++ again, but emit GC-managed pointers (precise or conservative collector) and keep real Go semantics (cycles, `unsafe`, full `reflect` metadata) | One toolchain (just a C++ compiler), tight integration with gocpp types and the goroutine layer | Largest piece of work (a collector, type metadata, write barriers or stop-the-world, `reflect`, `unsafe`), C++ build time for big trees, every Go corner case becomes ours |
| B3 | **Hybrid: B2 for most, B1 for the rest** | Translate when the package is simple, fall back to the real toolchain for packages with assembly, cgo or runtime tricks | Best coverage | Two mechanisms to maintain |
| B4 | Conservative collector (Boehm style) under B2 | Faster to build than a precise collector | Interior pointers and `unsafe` mostly work | False retention, no compaction, interaction with C++ ownership |

## 4. The boundary between the engines (the hard part under every option)

Each of these needs a rule, a test and an error message:

- **Values:** numbers and strings are easy; slices, maps and structs need copy, share or pin
  rules; strings and `[]byte` are the hot case.
- **Pointers and ownership:** can an engine-B object hold an ARC pointer to an engine-A object and
  the other way round? Cross-engine cycles cannot be collected by either engine without a rule
  (handles with explicit release, or roots registered with the GC).
- **Callbacks:** a gp closure passed to a Go package, and Go interfaces implemented by gp types.
- **Errors and panics:** `error` values, `panic` crossing the boundary, `defer` and `recover`.
- **Concurrency:** channels, `sync`, `context`, goroutines started on one side and waited on the
  other; one scheduler or two.
- **Generics:** instantiating a Go generic function with a gp type and the other way round.
- **Type identity:** `fmt`, `errors.Is/As`, type switches across the boundary.

## 5. Questions to answer before a plan can be written

1. **Why:** which packages do you want first (a database driver, a web framework, protobuf, crypto,
   a whole application)? A list of 10 real targets decides more than any design.
2. **Priority:** exact Go semantics and full compatibility, or speed of the Go side, or fast builds
   and small binaries? B1 wins the first, B2 can win the second and third.
3. **Toolchain:** is it acceptable that the full Go toolchain is a requirement for Go packages
   (B1/B3)? Today gocpp needs only a C++ compiler.
4. **Process model:** one process with two runtimes is acceptable? Or must goroutines, channels and
   `sync` be one system (B2)?
5. **Direction of calls:** gp calls Go only, or Go also calls back into gp and C++?
6. **Source form:** Go packages stay `.go` files in the module cache, fetched by `gocpp get`, and
   gp files import them with the same syntax? Does `gocpp` ever compile plain `.go` as an
   application entry point?
7. **Order:** before or after v1.0? (The plan puts it after M9.)
8. **Where does the line sit for the standard library?** Keep the translated std (engine A) as
   it is, or switch part of it to engine B later?

## 6. Steps (placeholders until the questions above are answered)

- 16.1 Research spike (B2): translate one small real Go package in a GC memory mode with a
  conservative collector (for example the bdwgc library through a `cxx` recipe) and a minimal
  GC-flavoured `string`/`slice`/`map`/pointer, call it from a gp program, and measure build time,
  binary size, call overhead and what breaks; keep B1 (c-archive) as a quick comparison only.
- 16.2 Decision record (ADR): which option, the boundary rules, what is out of scope.
- 16.3 and later: written after 16.2.
