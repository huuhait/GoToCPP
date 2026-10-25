# ADR-143: Two engines in one program: Go packages on a tracing collector, gp on reference counting

Status: proposed (2026-10-25). Draft for Phase 16 step 16.2. It is accepted after step 16.1h's
report and before step 16.3, with the 16.1h budgets filled in; the gin-against-Go and editor rows
stay **TBD** in the accepted text and are recorded later in docs/benchmarks/gc-engine.md or a dated
amendment note, because an accepted ADR is not rewritten. The user's decisions of 2026-10-21 and
2026-10-22 are fixed and are recorded here, not reopened. Would extend
[ADR-004](004-memory-arc.md) and [ADR-007](007-goroutines-os-threads.md) and change the scope of
[ADR-035](035-reflect-subset-on-descriptors.md),
[ADR-103](103-reference-cycles-stay-counted-common-shapes-get-compiler-fixes-and-a-cycle-report-comes-before-any-collector.md),
[ADR-106](106-reflect-grows-method-and-call-for-types-the-program-already-has.md),
[ADR-107](107-the-standard-library-beyond-tier-2-goes-by-blockers-not-by-popularity.md),
[ADR-109](109-after-1-0-the-pipeline-and-arm64-come-first-macos-needs-a-machine-and-a-checksum-database-needs-an-ecosystem.md),
[ADR-110](110-the-go-language-version-moves-once-a-year-and-the-v1-promise-names-what-it-covers.md) and
[Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen).

## Context

gocpp has one engine: automatic reference counting ([ADR-004](004-memory-arc.md)). It is right for
gp code and wrong for code written for Go, in four ways that the survey of 17 modules and gin v1.9.1
measures ([Phase 16 §1.2 and §7.1](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#12-why-go-code-needs-its-own-engine)):

- **Cycles.** gin's `New()` does `engine.engine = engine` (gin.go:228), and every `Context` points
  back to the engine. Under counting that memory is never freed.
- **Layout.** gp values are bigger than Go's: `*T` 16 B, `string` 32 B, `[]T` 40 B and an interface
  32 B, against Go's 8, 16, 24 and 16. 6 of the 17 modules convert `string` ↔ `[]byte` through
  `unsafe` (gin's `internal/bytesconv` has no safe variant), and protobuf reads the layout of
  interfaces.
- **Exactness.** Go packages expect Go's own standard library: `reflect` (`MapOf`, `ArrayOf`,
  `MethodByName` + `Call`, `MakeChan`, `Select`) in 8 of 17 modules, finalizers (`os.File`,
  `net.netFD`), `runtime.AddCleanup`, `unique` (`net/netip.Addr`).
- **Porting does not scale.** gin v1.12.0's closure is 101 non-standard packages (300,472 lines)
  and 203 standard ones (374,763); gp's library lacks 141 of the 203, and a port would have to add
  122.

The phase's answer is a second engine that gives Go code Go's layout and a tracing collector, next
to gp's ARC engine, in one program and with fixed boundary rules
([Phase 16 §2](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#2-how-it-works)).

### The user's decisions (fixed, 2026-10-21 and 2026-10-22)

| Question | Decision |
|---|---|
| How Go code is built | Translated to C++ by gocpp, like gp code. No Go toolchain is needed; building with the real `go` stays only an idea for packages the translator cannot handle |
| Memory | Two engines in one program: gp and C/C++ on reference counting, Go packages and a second standard library on a collector; values convert at the boundary |
| Collector | bdwgc first, reached only through a small `gp::gc` interface, so a different collector can replace it |
| Order | Libraries first, then gin; M10 pure-Go libraries pass their own tests, M11 `net/http`, TLS, the netpoller and gin |
| Windows | In M10: the libraries pass their tests on the real Windows PC, so bdwgc on Windows is on M10's path |
| Standard imports in gp code | gp's library first, else Go's (Phase 16 §2.3 rule 5); when gp's library later gains a package, programs move to it, the release notes name it, and `gocpp explain` shows the engine of each import |
| Start | Phase 16 started 2026-10-24; the README is the plan, this ADR decides its open points |

### The spike: what is evidence today, and what is not

| Job | What it produced | State |
|---|---|---|
| 16.1a | bdwgc 8.2.12 vendored, built into the cache with the §2.4 flags plus `-DGC_DISABLE_INCREMENTAL` and `GC_NO_THREAD_REDIRECTS`, smoke test at O0/O2, ASan+UBSan and TSan with GCC and Clang | merged |
| 16.1a-win | Windows MinGW-w64 build and smoke test on the real PC; thread starts ~61–65 µs at 1,000 (~115 µs at 10,000); static-root scan ~0.53 ms per collection, ~0.36 ms with `GC_set_no_dls(1)`, which alone also drops the executable's own roots | merged |
| 16.1b | The dry run of Go's standard library under the §2.7 constraints (`gostd/dryrun/go1.25.14`) | merged |
| 16.1c | The GC engine's walking skeleton: hidden `-gc-engine`, `gopkg_` names, a minimal emitter, `gp::gc` strings/heap with a malloc test backend | merged |
| 16.1h-base | The measurement harness (`tools/gcbench`) and the Go/ARC baselines (`docs/benchmarks/gc-engine-spike.md`) | merged |
| 16.1d | The bdwgc backend of `gp::gc`, thread registration and blocking sections | branch `job/16.1d`, not merged into `p16` when this was written |
| 16.1e | The GC engine's Go-layout values and placeholder descriptors | branch `job/16.1e`, not merged into `p16` when this was written |
| 16.1f, 16.1g, 16.1h | The four spike packages and stubs, the five boundary shapes, the GC-engine numbers | **not done**: no GC-engine number exists yet. The hidden `-gc-engine` switch and the skeleton are in since 16.1c (16.1h-base's message was from before it); 16.1d (branch) adds the bdwgc backend and 16.1e (branch) the value layer (with placeholder descriptors), so a GC-engine program can be built but not yet measured on the spike's packages |

So this ADR can fix the direction and the rules, and it can say which measurements will fill each
budget, but it cannot set a target: the Go/ARC baselines are baselines, and the machine was loaded
(1-minute load 43.9 before and 77.2 after the run, quiet limit 24). No number below is invented;
the ones quoted come from the committed baseline document, the merged jobs, or the phase README,
and the latter are marked as plan notes.

## Decision (proposed)

**The direction.** Go's `.go` source (Go modules and Go's standard library at the pinned version)
is translated to C++ with Go's value layout and allocates only in a GC heap owned by bdwgc,
reached only through `gp::gc`. gp code and C/C++ keep ARC and their own layout; the collector never
scans the ARC heap. Values convert at the boundary by the rules of
[Phase 16 §2.6](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#26-the-boundary-what-crosses-and-how).
A program that imports no Go package links no `gp::gc` symbol (16.1c's `TestGCEngineNoGCSymbols`
checks `_ZN2gp2gc`), and bdwgc reaches only GC-mode programs (16.1d's branch extends the same test
to bdwgc symbols; 16.8's test checks that the C++ is unchanged and no collector is linked).

The open points of step 16.2, with the recommendation and where the alternatives are weighed:

1. **Names and namespaces.** Keep the plan's scheme: `gp::gc::` for the engine's runtime
   (`string`, `slice<T>`, `iface`, `heap`, …), one flat C++ namespace `gopkg_<escaped import
   path>` per Go package, and `gpkg_` unchanged for gp packages. The escaping is ADR-010's
   (`/` → `__`, `.` → `_d`, `-` → `_h`, `_` → `_u`, …) with the prefix changed, so
   `github.com/gin-gonic/gin` is `gopkg_github_dcom__gin_hgonic__gin`. It is implemented and
   tested in 16.1c (`internal/cppgen/gc.go`, `TestGCNamespace`).
2. **Type descriptors.** Use Go's `internal/abi.Type` layout, so Go's own `reflect`,
   `internal/reflectlite`, `internal/abi` and `internal/runtime/maps` are translated rather than
   rewritten. The scheme for the three hard parts is the candidate of
   [Phase 16 §2.8](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#28-reflect-maps-and-interfaces):
   `resolve*Off` natives index a pointer table stored next to each descriptor or name blob;
   `typelinks`, `rtypeOff` and `typesByString` are overlaid to read one program-wide table that
   the link step sorts by string; the descriptors of unnamed types (`[]int`) are C++ inline
   variables, so the linker keeps one per program. `GCData` is carried from the start (stage 2
   needs it) but not used for scanning in M10. The placeholders of 16.1e are replaced by 16.5
   (the value types), 16.8 (the descriptors) and 16.11 (the real maps), which settle their three
   known differences from Go: `int` and `int64` share one C++ type
   today and so one descriptor identity; Go's direct-interface rule includes a single-element
   struct or array of a direct type, which the placeholder boxes; the map and channel tables are
   minimal. Names are Go's (`[]T`, `main.Box[int]`), and `reflect`'s equality and hashing come
   from the descriptor's `Equal` and hash.
3. **Heap scanning and the extra byte.** M10 uses stage 1: pointer-free allocations go through
   `alloc_atomic` and are never scanned; every other object is scanned conservatively; stacks,
   registers and static data are conservative always. Zeroing is the backend's job (16.1d zeroes
   both kinds; bdwgc's `GC_malloc_atomic` does not clear). Stage 2 (precise bitmaps from `GCData`
   as bdwgc descriptors) is decided at step 16.21 from measurements, not now; it would need
   gcj-style thread-local typed allocation or a patched bdwgc kind. The extra byte per object
   stays (`ALL_INTERIOR_POINTERS`, `DONT_ADD_BYTE_AT_END` not defined) until 16.21 measures
   whether it can go; the cost of keeping it is that an object whose size is a multiple of 16 B
   takes 16 B more.
4. **Misaligned `unsafe` access.** Use `aligned(1)` element types for pointers to numbers in the
   GC engine, which are plain moves on amd64 and arm64, so ASan+UBSan stay on. `-fno-sanitize=alignment`
   stays the fallback for a shape that `aligned(1)` cannot express, with the risk documented in
   the guide; refusing the code is not an option, because crypto's generic XOR reads a misaligned
   `[]byte` as `[]uintptr`.
5. **Import rules and `"go:..."` imports.** The seven rules of
   [Phase 16 §2.3](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#23-which-engine-builds-a-package)
   stand as written, including "gp's library first, else Go's". `"go:..."` imports **exist**, for
   standard packages only, in `.gp` files: `import gojson "go:encoding/json"` pins that import to
   the GC engine's copy, so a program can stay on Go's copy when gp's library gains the package.
   There is no `"gp:..."` counterpart (gp's copy is the default), no `go:` for modules (a module
   path already names its engine), and an unavailable `go:` path is an error naming the engine.
   `gocpp explain` shows the engine of every standard import, with or without the prefix.
6. **The gp view and the boundary.** The table of
   [§2.6](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#26-the-boundary-what-crosses-and-how)
   is the contract, and step 16.14's suite tests every row both ways plus a refusal for every
   refused crossing. Its open details are fixed as follows:
   - **Twins.** The allowlist is the one §2.6 names: named numbers, strings and bools
     (`time.Duration`, `fs.FileMode`), `time.Time`, the `io` interfaces, `fmt.Stringer`,
     `context.Context` and `error`, and the exported error variables and types (`io.EOF`,
     `*fs.PathError`). A tool checks the allowlist (no pointers, no `noCopy`, no self-address
     check, no finalizer). Everything else keeps its Go type in the gp view (`go:sync.Mutex`,
     `reflect.Value`, `*os.File`, …) and crosses only through an explicit adapter.
   - **The call-only list.** gocpp ships the list §2.6 names (JSON, XML, YAML, TOML and protobuf
     decoders and encoders, gin's `Bind*`/`ShouldBind*`/renderers, validators), and a module
     extends it with a `call-only` statement in its `gocpp.mod`:
     `call-only encoding/json.Unmarshal` or `call-only github.com/gin-gonic/gin.Context.ShouldBindJSON`,
     with an optional parameter selector when only some pointer parameters are call-only.
     `gocpp.mod` keeps unknown verbs, so a module with this line still reads in an older gocpp.
   - **Names.** The new gp standard package `gc` provides `gc.New[T any]() *T` (a pointer to `T`'s
     mirror in the GC heap, which gp code uses like any Go pointer) and `gc.File(f *os.File)
     *os.File` (a Go `*os.File` over the same descriptor, with no finalizer). They are ordinary
     imports in gp code, not compiler magic.
   - **Proxies and roots.** Proxies are canonical: one proxy per object while it lives (a weak
     table), and `gp::bridge::hold` gives one root per Go object (16.1d implements a root as a
     `GC_MALLOC_UNCOLLECTABLE` cell), so `==` keeps Go's meaning. Context adapters unwrap on the
     way back.
   - **Mirrors.** Mirror types are written as Go source (the gp struct's fields and tags, under
     the gp package's own path, translated by the GC engine) with forwarders for the gp type's
     exported methods, so Go's `reflect`, `encoding/json` and validators see a normal Go struct.
   - **Channels.** A channel crosses only when its element has one layout in both engines:
     `struct{}`, `bool` and numbers (and named types of those). Other element types are a compile
     error that names the fix (a small `.go` forwarding package in the gp module).
   - **Generic type arguments.** gp types as Go type arguments are out of the first version; an
     inferred gp type argument is an error that names it. Instantiation with GC-engine types or
     convertible ones works.
7. **Go's source.** Download the pinned Go source archive once, with its checksum in gocpp, or use
   a local tree named by `GOCPP_GOROOT` whose `VERSION` file matches the pinned version. Do not
   embed the tree in the gocpp binary: it is versioned data that would grow every release, and the
   BSD-3 notice must travel with it anyway. `GOCPP_OFFLINE=1` ([ADR-138](138-gocpp-offline-has-one-reader-for-the-toolchain-and-the-modules.md))
   with no cached source is an error that names the two ways to provide it.
8. **Overlays and `//go:linkname`.** The policy of
   [§2.7](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#27-gos-standard-library-on-the-gc-engine)
   stands: Go's `runtime` is gocpp's own over natives; the runtime's pushes and pulls become one
   hook table; a pull or push of a symbol that exists in translated code or the runtime resolves to
   its C++ name, and anything else is an error naming the symbol; the five kinds of `//go:linkname`
   and the five compiler intrinsics are handled; overlays replace only files that cannot be built
   otherwise: assembly with no pure-Go fallback, or files the `gocpp` tag drops; `gostd/` records
   each native's Go signature and each overlay's original-file hash, checked by `tools/gostdcheck`.
   Go's linkname lock-down (go.dev/issue/67401) is why the record is per Go version, and a point
   release is taken like a version move.
9. **Sanitizers.** On Linux, `-asan` (ASan+UBSan) and `-race` (TSan) cover the GC engine, with
   bdwgc built once per sanitizer, `SIGSYS` under TSan and `-fsanitize-address-use-after-return=never`,
   and the GC heap registered with LeakSanitizer. On Windows they do not cover it yet: the real PC
   has MinGW-w64 and no clang, Windows sanitizers and llvm-mingw are untested, so **this ADR
   records a WORKFLOW exception**: the Windows GC-engine suites (M10) run without sanitizers; the
   Linux runs carry the sanitizer job. The exception ends when a Windows sanitizer toolchain is
   tested (llvm-mingw or a MinGW sanitizer build), which is a Windows item, not a new ADR.
10. **The engine's shape.** The real GC engine goes through `internal/lower` and
    `internal/cppgen` (steps 16.7/16.8), with an engine column in the gate table; the separate
    emitter of 16.1c stays a throwaway for the spike and never ships. The backend decisions of
    16.1d are kept: `gp::gc::heap` has a malloc backend for the unit tests and a bdwgc backend
    selected by `GOCPP_GC_BDWGC`, exactly one `process_heap`; roots are uncollectable cells;
    `SIGRTMIN+6` and `SIGRTMIN+7` are chosen before `GC_INIT` (bdwgc's `SIGSYS` under TSan, no
    signal on Windows); incremental mode is off (`-DGC_DISABLE_INCREMENTAL`, verified); the shared
    runtime reaches the collector through constant-initialized hooks in `gp::detail` that are null
    in the ARC engine, and a `gc_thread_guard` registers a goroutine's thread or the timer thread
    and unregisters it on every exit path. bdwgc lives under `<cache>/bdwgc/<version>-<hash>/`,
    next to `<cache>/rt/`, so a runtime change does not rebuild it.
11. **Budgets.** All TBD; the form of each is fixed now. Step 16.1h fills the rows it can (pauses,
    memory per thread, boundary costs, parks and system calls, and the form and scale of the build
    and size rows); 16.19 fills gin's numbers and 16.20 the editor's. The 16.1h numbers are accepted
    with this ADR before 16.3; the later rows are recorded in docs/benchmarks/gc-engine.md or a
    dated amendment note:

    | Budget (§5 uses it) | What is measured | Target |
    |---|---|---|
    | Stop-the-world pause | p50 and p99 with 1,000 and 10,000 goroutines, blocked and running, realistic stacks, on Linux and the Windows PC; a per-thread term fitted from the two counts | TBD (16.1h) |
    | Memory per thread | resident bytes per blocked and per running thread at 1,000 and 10,000 (the plan notes ~37 KB per blocked thread, §2.5); per-thread term | TBD (16.1h) |
    | Boundary costs | ns per shape of §2.6: a root, a string both ways, a borrowed `[]byte`, copy-in/copy-out of a struct, a gp closure as a Go func | TBD (16.1g, 16.1h) |
    | Parks and system calls per second | blocking-section throughput at 1 to 64 threads (the plan notes 156 ns at 1 thread, 9 µs at 32, 19 µs at 64, §2.5); parks and syscalls per second | TBD (16.1h) |
    | Cold and warm builds | cold build of gin's closure (about 675k lines: 300k non-standard + 375k standard, §2.10); warm rebuild after an edit of `main.gp` (the target for a debug build is 2 s, §5) | TBD (form and scale from 16.1h, gin's numbers from 16.19) |
    | Binary size | a gin program's release build against Go's, assuming no method pruning at link time (§2.10) | TBD (form and scale from 16.1h, gin's numbers from 16.19) |
    | gin against Go | requests per second, p99 latency, memory and GC pauses, release builds, 60 s at 1,000 keep-alive connections | TBD (16.19, from 16.1h) |
    | The editor | hover, completion and definition on gin's closure (675k lines) and its gp view, next to Phase 11's budget | TBD (16.20) |

    The plan notes quoted in the table are the plan's, not a job's; 16.1h confirms or replaces
    them. The Go/ARC baselines are the comparison, not a target: `docs/benchmarks/gc-engine-spike.md`
    has them (for example `binarytrees` at 1 thread: Go 609.1 ms, ARC 6,969.6 ms; ARC's build
    13.2 s and binary 27.2 MiB against Go's 0.1 s and 1.9 MiB), and it says no engine comparison is
    claimed from them.
12. **What changes in the existing decisions.** See the table below. ADR-107's `net/http` and
    `crypto/tls` item is replaced, a partial supersession; the others are extended or narrowed,
    and none is superseded as a whole.

| Existing decision | What this ADR does |
|---|---|
| [ADR-004](004-memory-arc.md) (accepted, ARC) | Extends it: ARC stays every gp and C/C++ value; its rejection of Boehm applies to gp code only; Go packages get the GC heap. A short amendment note is added when this ADR is accepted. |
| [ADR-007](007-goroutines-os-threads.md) (accepted, one OS thread per goroutine) | Kept in both engines. Adds: every wait that can last is a blocking section, so a stop is bounded by the running threads; threads register and unregister with the collector; chosen real-time signals, `SIGSYS` under TSan; bdwgc's per-thread free lists count in the memory budget. |
| [ADR-035](035-reflect-subset-on-descriptors.md) (accepted) | Unchanged for gp. The GC engine does not use gp's subset: it translates Go's `reflect` over `abi.Type`. In a Go package's gp view, `reflect.Value`, `reflect.Type` and the other identity types stay Go types (not twins). |
| [ADR-103](103-reference-cycles-stay-counted-common-shapes-get-compiler-fixes-and-a-cycle-report-comes-before-any-collector.md) (proposed) | Its collector question is answered for Go packages only: the GC engine collects Go cycles. Its cycle report, if built, would have to follow roots and proxies through the GC heap; cross-engine cycles still leak and gp's `weak.Pointer` to a Go object breaks them. |
| [ADR-106](106-reflect-grows-method-and-call-for-types-the-program-already-has.md) (proposed) | Unchanged for gp's own ports. Its motivation shrinks: `text/template`, `testing.F` and the like can run as Go packages on the GC engine, with Go's `reflect`; ADR-106 remains the answer only for a package ported into gp's library. |
| [ADR-107](107-the-standard-library-beyond-tier-2-goes-by-blockers-not-by-popularity.md) (proposed) | Its `net/http`/`crypto/tls` item is replaced (partial supersession): they reach gp through the GC engine instead of a C++ wrapper module, and the feature matrix's `net/http` "never" becomes "through the GC engine". Its other waves for `os/exec`, signals, a poller, sockets and crypto overlap with steps 16.10, 16.16 and 16.17; each package gp's library gains moves programs to gp's copy (§2.3 rule 5), unless the import is `go:`-pinned. |
| [ADR-109](109-after-1-0-the-pipeline-and-arm64-come-first-macos-needs-a-machine-and-a-checksum-database-needs-an-ecosystem.md) (proposed) | Go modules come from `GOPROXY` and are checked against `sum.golang.org` (§2.11), so its "keep trust on first use" covers gp modules only; the Go-module trust story is Go's, and step 16.7 records it. |
| [ADR-110](110-the-go-language-version-moves-once-a-year-and-the-v1-promise-names-what-it-covers.md) (proposed) | Its option 4 (a language version per module) is needed for Go code: a Go module's `go` line decides file versions and `GODEBUG` defaults. `gocpp.mod`'s `go` line stays ignored for gp code. The yearly move now also moves the pinned Go source, `gostd/`'s tool tags, natives and overlays, and the dry run. |
| [Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen) | The item "an optional tracing GC for cycle-heavy programs" is now the GC engine for Go packages (gp stays on ARC), and "`net/http` through a C++ library wrapper" is replaced by Go's own `net/http` on the GC engine. Both point at this ADR. The manager keeps the plan; the user decides the edit. |

## Options

1. **Names and namespaces.** (a) The plan's flat `gopkg_` + ADR-010 escape (recommended): it is
   already implemented and tested (16.1c), and sharing one escape with `gpkg_` means one rule for
   the compiler, the debugger and `gocpp explain`. (b) Nested C++ namespaces
   (`gopkg::github::com::gin`): names become unstable with the module's path, C++ has no
   `::`-free alias for every use, and ADR-010 already chose flat names for gp for the same
   reasons. (c) A second escape of its own: more code for no gain, and the two engines' names
   would stop looking alike. `gp::gc::` for the runtime follows the existing `gp::` C++ namespace
   and keeps the engine's types apart from `gp::ptr` and `gpkg_`.
2. **Type descriptors.** (a) `abi.Type` layout and Go's translated `reflect` (recommended):
   reflect is the point of the engine ("Go's own `reflect`, translated", §2.2), 8 of 17 modules
   need APIs that gp's subset lacks, and the dry run counts 40 runtime pushes into `reflect` for
   `net/http` + TLS, so a rewritten reflect would be a large, permanent divergence. The plan
   records a g++/clang++ check that `abi.Type` is 48 B, `abi.StructType` 80 B and
   `abi.UncommonType` 16 B, as in Go, and that the descriptors can be constant data. (b) gocpp's
   own descriptors with its own reflect: rejected, it repeats the subset problem ADR-035 has and
   makes every Go package that uses reflect a port. (c) A hybrid (Go layout for some kinds): the
   worst of both, since `reflect` reads every kind's layout. For the scheme, (i) Go's own module
   data with load-time relocation: rejected because gocpp caches package objects independently and
   has no Go linker; (ii) pointer tables next to descriptors and names, with the three typelinks
   entry points overlaid and unnamed descriptors as inline variables (recommended); (iii) a hash
   map built at run time: rejected, it needs an initialization order and makes every lookup a
   hash. The 16.1e placeholders are not an option: they would make `int` and `int64` one dynamic
   type in an interface, box a single-element struct of a direct type, and cannot express the
   method tables `reflect` needs.
3. **Heap scanning.** (a) Conservative from the start, no stage 1: simplest, but every byte of a
   `[]byte` or a string is scanned, which costs time and retains false pointers. (b) Stage 1
   (pointer-free via `alloc_atomic`, everything else conservative) in M10 (recommended): it is
   already what 16.1d's backend does, and its test proves a pointer-free object does not retain a
   pointer-shaped number. (c) A precise heap now: rejected for M10 because bdwgc's typed
   allocation does not use the thread-local free lists and adds a word per object (the slow case
   of the V numbers in §2.4), and it needs gcj-style allocation or a patched kind; TinyGo's
   precise layouts (0.43) are not a precedent for threads. Stage 2 then waits for 16.21's
   measurements. The extra byte: keeping it is the conservative choice with interior pointers
   (`&s.f`, a slice into an array) and libc's end pointers; dropping it saves 16 B only on objects
   whose size is a multiple of 16 B and needs a measurement plus the end-pointer argument, so it
   waits for 16.21 too.
4. **Misaligned `unsafe`.** (a) `aligned(1)` element types (recommended): ASan+UBSan stay on for
   the whole engine, and the plan says the accesses are plain moves on amd64 and arm64. (b)
   `-fno-sanitize=alignment`: the fallback, because it turns off a class of checks for every
   misaligned access in the engine, including ones that are real bugs; it is documented and the
   guide names the risk. (c) Refusing the code: rejected, crypto's generic XOR reads a misaligned
   `[]byte` as `[]uintptr`, and `unsafe` beyond `Sizeof` appears in 8 of the 17 surveyed modules.
   Which shapes `aligned(1)` can express is checked in 16.8's tests; no measurement of its cost
   exists yet.
5. **Imports and Go's source.** The §2.3 rules stand as this ADR's decision; rule 5 records the
   user's 2026-10-22 decision; the open half is `go:`. (a) No `go:`: then a program cannot stay on
   Go's copy when gp's library gains a package,
   and the only fix is a new import path. (b) `go:` for standard packages only (recommended):
   minimal, at the import site, checkable by `gocpp explain`, and it does not open a second way to
   name modules. (c) A `prefer-go` statement in `gocpp.mod`: more machinery for the same effect,
   and it cannot express one file's choice. For Go's source: (a) the pinned archive downloaded
   once with a checksum in gocpp (recommended default), (b) `GOCPP_GOROOT` for a local tree with a
   matching `VERSION` (recommended as the offline/developer route), (c) embedding: rejected as
   versioned data in every binary, with the update and license cost, (d) requiring a Go
   installation: rejected by the fixed decision. `GOCPP_OFFLINE` keeps its meaning (ADR-138).
6. **The boundary.** The table of §2.6 is the design of record; the alternatives were weighed in
   the 2026-10-22 review and are not reopened. The remaining choices: allowlist (a) the fixed list
   §2.6 names (recommended), (b) every standard type whose Go and gp shapes match: rejected, a
   type with identity (`sync.Mutex`, `strings.Builder`) cannot be copied safely; (c) none: gp code
   could not call a Go API that mentions a standard type. Call-only list: (a) ship the list and
   extend it with `call-only` in `gocpp.mod` (recommended); (b) prove non-escape with summaries:
   not enough, because the decoders take `any` and go through interface calls (Go's own compiler
   reports `moved to heap: req`), and ADR-114's analysis summarizes only `*T` parameters; (c)
   refuse every gp pointer that crosses: too strict for the program of §2.1. Proxies: canonical
   (recommended) because otherwise `err == http.ErrServerClosed` stops holding, which Gin, HTTP
   and the standard sentinels rely on; per-call proxies are rejected. Mirrors: generated Go source
   (recommended) because Go's `reflect`, `encoding/json` and validators need Go metadata and tags;
   a hand-written C++ struct would be invisible to them. Channels: only the one-layout elements
   (recommended); a relay through another thread per channel is an extra copy and hand-off per
   value and changes channel identity (`==`, `len`, `cap`); refusing all channels would fit
   neither `sync` nor Gin. Generic type arguments: gp
   types out (recommended); the alternatives are converting a gp type to its mirror (wrong
   identity and wrong `reflect` view) or instantiating per gp type (unbounded code).
7. **Overlays and sanitizers.** The overlay policy is the plan's; the dry run fixes its input: 15
   packages with `.s` files and no pure-Go fallback on linux/amd64 (13 on windows/amd64), 9
   bodyless linkname targets outside the runtime (5 with no push at all, needing a native or
   overlay), 17 Windows-only bodyless functions outside the runtime. For sanitizers, (a) require
   them on Windows too: blocked today, the PC has no clang and Windows sanitizers are untested;
   (b) the WORKFLOW exception recorded above, with Linux carrying ASan+UBSan and TSan
   (recommended); (c) make llvm-mingw or a MinGW sanitizer build a Windows item, which ends the
   exception when it is tested. bdwgc's own verdicts are the evidence: 16.1a builds and passes its
   smoke test with GCC and Clang at O0/O2, ASan+UBSan and TSan; 16.1a-win builds and runs on the
   real PC under MinGW-w64; 16.1d's tests pass under ASan+UBSan and TSan on Linux.
8. **The engine's shape.** (a) A second emitter path, as 16.1c's skeleton (rejected as the real
   engine): two lowerers diverge, and the gates, the interop checker and the debugger printers
   would be written twice; 16.1c itself reports the question. (b) Thread the engine through
   `internal/lower` and `internal/cppgen` with an engine column in the gate table (recommended,
   step 16.8). (c) A separate compiler driver: rejected, the loader, cache and module rules are
   shared. The 16.1c skeleton keeps its limits as a spike: `int`, `string` and `bool`,
   package-local functions, `if`/`for`, no imports, methods, generics, pointers, slices, maps,
   interfaces, structs or closures, and `func init` is an error; all of them say a later job adds
   them, and it does not use `gp::run_main`. Its hidden flag is not a shipped feature.
9. **Budgets.** There is no option between the targets yet: no GC-engine number exists. The form
   in the table above is the decision (which quantity, at which thread counts, on which targets),
   and the reports of 16.1h, 16.19 and 16.20 are the input. The alternatives that are rejected:
   absolute targets invented from the plan's estimates, and targets relative to the noisy baselines
   without saying so. The 16.1h numbers are inserted here before the ADR is accepted; the 16.19 and
   16.20 rows stay TBD in the accepted text and are recorded later in docs/benchmarks/gc-engine.md
   or a dated amendment note, not by rewriting this ADR.

## Trigger

- **Acceptance before step 16.3.** Part B builds `gp::gc` and the runtime on these decisions; the
  ADR is accepted (with or without changes) after 16.1h's report and before 16.3 starts.
- **The budgets.** Step 16.1h's report fills its rows before acceptance; the gin-against-Go and
  editor targets stay TBD in the accepted text and are recorded later in
  docs/benchmarks/gc-engine.md, or in a dated amendment note, since an accepted ADR is not
  rewritten.
- **Reopening one point, not the direction.** Step 16.21 decides stage 2 and the extra byte from
  its measurements; 16.15 or 16.19 may ask for stage 2 earlier. A refused crossing, a measured
  budget break, or a Windows sanitizer result reopens the specific point.
- **Go's drift.** A new Go version changes the linkname surface (go.dev/issue/67401), the tool
  tags or the overlays; the `gostd/` record and the dry run move with it, not this ADR.

## Consequences

- **Good.** Go code gets the engine it was written for: cycles are collected, Go's layout makes
  `unsafe` work, Go's `reflect`, maps, interfaces, finalizers, `cleanup`, `weak` and `unique` come
  from Go's own source, and the module ecosystem is reachable without a Go installation. gp keeps
  what ARC is for: deterministic release, C++ interop, its own layout. bdwgc is behind `gp::gc`,
  so it can be replaced; a program without Go packages pays nothing.
- **Bad.** Two standard libraries and two engines in one binary (§2.10: about 675k lines for gin's
  closure; the binary-size and build budgets are TBD). Every goroutine is an OS thread and keeps
  bdwgc's thread-local free lists, so pauses and memory carry a per-thread term; the plan notes
  that with a tiny heap 1,000 blocked threads stopped the world in 0.6 ms on this machine and
  10,000 in 22 ms, and that each blocked thread kept about 37 KB (§2.5) — 16.1h measures these,
  and the budgets hold them. Conservative scanning retains false pointers; a precise heap waits
  for 16.21. The boundary copies strings, slices and structs in some directions, and a gp pointer
  must be on the call-only list or refused. Cross-engine cycles leak, as ARC cycles do. A gp object held by Go
  loses deterministic release. Windows GC-engine suites run without sanitizers until the exception
  ends. bdwgc's global lock is taken twice per blocking section (the plan notes 156 ns at one
  thread and 9 to 19 µs at 32 to 64); only long waits are wrapped.
- **Uncertain.** The budget numbers themselves (16.1h, 16.19, 16.20), the precise heap's need and
  cost (16.21), whether `aligned(1)` covers every misaligned shape (16.8's tests; no measurement),
  the memory per thread at 10,000 goroutines on Windows (16.1h), and the editor's cost on gin's closure
  (16.20). The C++ name stability and the generated code are not part of the v1 promise
  ([ADR-110](110-the-go-language-version-moves-once-a-year-and-the-v1-promise-names-what-it-covers.md)),
  so the engine can change its C++ as long as programs keep behaving.

## Evidence

- [Phase 16 §1.2, §2 and §7.1](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#12-why-go-code-needs-its-own-engine):
  why Go code needs the engine; the survey of 17 modules and gin v1.9.1 (433 + 253 packages,
  go1.25.7, no type errors); the gin v1.12.0 closure and the 141/122 numbers. §2.2 to §2.11 are
  the design this ADR decides; §3 step 16.2 is its list; §5 uses its budgets.
- [Phase 16 STATE](../../plans/2026-10-13-gocpp/phase-16-dual-engine/STATE.md): the workspace and
  the 16-thread cap; the 16.1a/16.1a-win/16.1b/16.1c/16.1h-base logs and their review findings;
  the 2026-10-22 review's fixes; the Windows PC's toolchain; the open questions this ADR answers.
- [gostd/dryrun/go1.25.14](../../gostd/dryrun/go1.25.14/README.md) and its two JSON files: 176/177
  packages; 303/299 distinct two-argument linkname targets; 169/173 runtime hooks (151+18,
  159+14); 9 synctest-only; 14/11 linker; 163 bodyless targets (154 provided by the runtime, 9
  not, 5 by no push); 40 `reflect`, 18 `sync`, 12 `syscall`, 11 `internal/poll`, 7 `time` pushes;
  17 pushes from `internal/runtime/maps`; 15/13 packages with `.s` files; the 17 Windows-only
  bodyless functions outside the runtime; the five intrinsics. It also says the plan's earlier
  166/149/17 did not reproduce and why.
- [docs/benchmarks/gc-engine-spike.md](../benchmarks/gc-engine-spike.md): the harness and the
  Go/ARC baselines (builds, binary sizes, `binarytrees`, `parks`, `syscalls` at 1 to 64 threads),
  and what 16.1h adds (the GC columns and the goroutine rows). The machine was loaded, so the
  figures are baselines.
- [runtime/third_party/bdwgc/README.gocpp](../../runtime/third_party/bdwgc/README.gocpp): bdwgc
  8.2.12 (commit, sha256, license), the 27 Linux and 25 Windows sources, the fixed defines
  (`GC_DISABLE_INCREMENTAL`, `NO_EXECUTE_PERMISSION`, `GC_GCJ_SUPPORT`, `GC_ENABLE_SUSPEND_THREAD`,
  …) and the update procedure.
- Jobs 16.1a and 16.1a-win (merged): the Linux/Windows builds, the ASan/TSan verdicts on Linux,
  the real-PC smoke test and the probes (thread starts ~61–65 µs at 1,000, static scan ~0.53 ms
  per collection and ~0.36 ms with `GC_set_no_dls`, which needs `GC_add_roots` of the executable's
  writable image).
- Job 16.1c (merged): `internal/cppgen/gc.go` (the `gopkg_` namespace and the separate skeleton
  path, with the design question reported), `internal/cppgen/gc_test.go` (`TestGCNamespace`), the
  differential tests against `go run`, and the `nm` test (`TestGCEngineNoGCSymbols`) that a `.gp`
  program links no `gp::gc` symbol; 16.1d's branch extends the same test to bdwgc symbols.
- Job 16.1d (branch `job/16.1d`): `runtime/include/gocpp/gc/heap.hpp` (the `gp::gc::heap`
  interface: `alloc`, `alloc_atomic`, `enter_thread`, `leave_thread`, `blocking`, roots, `collect`,
  `stats`; the malloc and bdwgc backends), `runtime/include/gocpp/gc/hooks.hpp` (the
  constant-initialized hooks in `gp::detail`, null in ARC, and `gc_thread_guard`),
  `runtime/src/gc/heap_bdwgc.cpp`, and `runtime/test/gc_heap_bdwgc_test.cpp` (zeroing, a root, a
  cycle, pointer-free non-retention, 1,000 threads that exit then a collection, the timer thread,
  collection during blocking sections, exception rethrow, statistics; O0/O2, ASan+UBSan, TSan).
- Job 16.1e (branch `job/16.1e`): `runtime/include/gocpp/gc/iface.hpp` (the placeholder `type_desc`,
  `itab`, `eface`/`iface`, boxing, equality and the note on its gaps),
  `runtime/include/gocpp/gc/golayout.hpp`, `slice.hpp`, `map.hpp`, `chan.hpp` (Go's sizes and
  placeholder tables), `internal/check/gcsizes.go` (the generated Go's amd64 sizes and the layout
  test from real Go), `runtime/test/gc_values_test.cpp` and `gc_layout_test.cpp`.
- [ADR-004](004-memory-arc.md), [ADR-007](007-goroutines-os-threads.md),
  [ADR-035](035-reflect-subset-on-descriptors.md), [ADR-103](103-reference-cycles-stay-counted-common-shapes-get-compiler-fixes-and-a-cycle-report-comes-before-any-collector.md),
  [ADR-106](106-reflect-grows-method-and-call-for-types-the-program-already-has.md),
  [ADR-107](107-the-standard-library-beyond-tier-2-goes-by-blockers-not-by-popularity.md),
  [ADR-109](109-after-1-0-the-pipeline-and-arm64-come-first-macos-needs-a-machine-and-a-checksum-database-needs-an-ecosystem.md),
  [ADR-110](110-the-go-language-version-moves-once-a-year-and-the-v1-promise-names-what-it-covers.md),
  [ADR-138](138-gocpp-offline-has-one-reader-for-the-toolchain-and-the-modules.md), and
  [Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen).
- Not evidence yet: 16.1f, 16.1g and 16.1h for the engine's rows; the gin-against-Go row waits
  for 16.19 and the editor row for 16.20, while the build and size rows get their form and scale
  from 16.1h and their gin numbers from 16.19.

## See

[Phase 16 §1.1](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#11-your-decisions-2026-10-21-and-2026-10-22),
[§2.3](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#23-which-engine-builds-a-package),
[§2.4](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#24-the-collector-gpgc-over-bdwgc),
[§2.5](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#25-threads-and-stopping-the-world),
[§2.6](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#26-the-boundary-what-crosses-and-how),
[§2.7](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#27-gos-standard-library-on-the-gc-engine),
[§2.8](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#28-reflect-maps-and-interfaces),
[§2.10](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#210-build-cache-and-size),
[§3 step 16.2](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#3-steps) and
[§5](../../plans/2026-10-13-gocpp/phase-16-dual-engine/README.md#5-exit-criteria).
