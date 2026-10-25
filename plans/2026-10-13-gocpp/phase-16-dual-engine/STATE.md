# Phase 16: Dual engine · STATE

**Status:** `[~]` started 2026-10-24 (your decision): step 16.1's spike runs as jobs 16.1a (bdwgc), 16.1b (Go stdlib dry run) and 16.1c (GC-engine walking skeleton) in parallel worktrees under `~/worktrees/GoToCPP-Gopkg`
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-24

## Steps

Every runtime step has a Windows item: it runs on the real Windows PC (`GOCPP_WINDOWS_SSH`).

### Part A: decide (4–6 weeks)
- [~] 16.1 Spike: one Go package on bdwgc, called from gp (3–4 weeks)
  - [x] bdwgc 8.2.12 built with the flags of README §2.4 (Linux GCC + Clang, Windows MinGW-w64 on the real PC), once per sanitizer (jobs 16.1a + 16.1a-win, both clean and merged: Linux GCC/Clang + ASan/TSan; Windows MinGW-w64 with bdwgc's CMake pthreads+WIN32 branch, 25 sources, smoke test on the real PC. Not tested: llvm-mingw (the PC has no clang) and ASan/TSan on Windows; the ADR of 16.2 records both.)
  - [~] throwaway GC mode: `container/list`, `strings`, `strconv`, `sort`, binarytrees with `println`; stubs for `runtime`, `internal/bytealg`, `internal/reflectlite` (job 16.1c merged the walking skeleton: hidden `-gc-engine`, stock-`go/parser` loading of `.go` files, `go/types`, a minimal GC-mode emitter with Go's layout/arithmetic/evaluation order and `#line`, and `gp::gc` string/print/heap with a malloc test backend; the four packages and the stubs are 16.1f, the bdwgc backend is 16.1d, the values are 16.1e)
  - [ ] five boundary shapes by hand (rooted `gp::ptr`, string both ways, borrowed `[]byte`, copy-in/copy-out, gp closure as a Go func)
  - [~] measurements: allocation and peak memory; stop-the-world pause and memory per thread with 1k/10k goroutines (blocked, running, realistic stacks); parks and system calls per second at 1–64 threads; boundary costs; compile time; collector size; ASan/TSan with per-sanitizer builds (jobs 16.1h-base + 16.1h-prog merged: the harness is `tools/gcbench`, the programs `bench/gc` (binarytrees, parks, syscalls, goroutines: 1k/10k, blocked/running/mixed, depth), the Go/ARC baselines are `docs/benchmarks/gc-engine-spike.md`, re-measured on the new host. At 10k blocked goroutines, peak RSS per goroutine is 3.5 KiB (Go) vs 13.2 KiB (ARC). The GC rows and the stop-the-world pause hook, the boundary costs, compile time/size and the Windows runs are 16.1h)
  - [~] Windows: the same measurements, plus thread starts and the scan of static roots in each collection (job 16.1a-win measured the thread starts and the static-root scan on the real PC: ~61–65 µs per registered thread start, ~0.53 ms per collection scanning the static data by default, ~0.36 ms with `GC_set_no_dls(1)`, which alone drops the executable's own roots, so it needs `GC_add_roots` of the executable's writable image; the binarytrees/pause/park rows are 16.1h)
  - [x] dry run of Go's standard library under the tags of README §2.7: bodyless functions, `//go:linkname` of each kind, unsupported constructs (jobs 16.1b + 16.1b-tc, both clean and merged; data in `gostd/dryrun/go1.25.14`, tool `tools/gostddryrun`. The type check finds **0 real errors** in 177/178 selected packages under the gocpp tags, so the input of 16.9–16.11 is the syntax/translation work, not a type-system repair; `internal/goroot` and vendored `x/sys/cpu` need overlays only when named, and `x/sys/unix` is not vendored in GOROOT)
- [~] 16.2 ADR: the design (1–2 weeks; the **proposed** ADR is merged as `docs/adr/143-dual-engine.md` after a clean review. It is accepted after 16.1h's budgets and before 16.3; the gin-against-Go and editor targets stay TBD in the accepted text and are recorded in `docs/benchmarks/gc-engine.md` or a dated amendment note, because an accepted ADR is not rewritten)
  - [ ] names; descriptors and the scheme for offsets, `typelinks` and unnamed types; the heap scan stages and the extra byte; misaligned access
  - [ ] import rules and `"go:..."` imports; the gp view, the allowlist of twins, the boundary table, the call-only list and its directive, `gc.New[T]`, `gc.File`, which channels cross
  - [ ] Go's source delivery, the overlay and `//go:linkname` policies, sanitizers for the GC engine (or a WORKFLOW exception)
  - [ ] budgets: pauses and memory with a per-thread term, boundary costs, parks and system calls per second, cold and warm builds, size, gin against Go, the editor
  - [ ] what changes in ADR-004, 007, 035, 103, 106, 107, 109, 110 and Phase 15 §2.8

### Part B: the engine's runtime (8–11 weeks)
- [ ] 16.3 `gp::gc` and bdwgc (2–3 weeks)
  - [ ] vendored, built per toolchain and sanitizer into the runtime cache
  - [ ] roots, the finalizer goroutine, cleanups after the Go finalizer, weak handles per (base, offset), allocation counters, `GOGC` through `GC_set_min_bytes_allocd`, the soft memory limit, `USE_MUNMAP`
  - [ ] tests: order of finalizers and cleanups, weak equality after reuse, heap shrinking, statistics
  - [ ] Windows
- [ ] 16.4 Threads and stops (3–4 weeks)
  - [ ] registration (workers, timer thread, poller thread, finalizer goroutine, foreign threads in the boundary stub) and unregistration on every exit path; `GC_register_altstack`; signals (`SIGSYS` under `-race`)
  - [ ] blocking sections only around waits that can last; `syscall.Syscall` and `Syscall6` as one native; the callback rules
  - [ ] the audit: every runtime structure that can hold a GC pointer is GC-allocated or rooted; rooted per-thread records; a rooted panic value
  - [ ] tests: no signal to a blocked thread; 1,000 threads that exit, then a collection; 10,000 allocating goroutines; collections during deep recursion, C calls, and with values in channel buffers and defer records
  - [ ] Windows
- [ ] 16.5 The GC engine's values (2–3 weeks)
  - [ ] Go's exact layout: zero-size fields as `[[no_unique_address]]` tag members, explicit padding, a `static_assert` per field offset, strides from Go's sizes; the GC engine's sizes in `internal/check/sizes.go`
  - [ ] `new_`, `make`, `append`, zeroed pointer-free allocations, strings, conversions, checks that panic like Go
  - [ ] `chan_core` and its buffer in the GC heap, with an element table and no destructors
  - [ ] the layout table from real Go, with the zero-size shapes of `sync`, `sync/atomic`, `weak`, `log/slog`
  - [ ] Windows
- [ ] 16.6 Panics, `defer`, `recover` and `Goexit` across the engines (1 week)
  - [ ] Windows

### Part C: the compiler (7–10 weeks)
- [ ] 16.7 Loading Go packages and modules (2–3 weeks)
  - [ ] `.go` packages: stock `go/parser`, build constraints with the pinned version's tool tags, `_test.go`, `//go:embed`, cgo files, `.s` files, directives
  - [ ] module rules (README §2.3): `gocpp.mod` and `go.mod` side by side, MVS with the main module's `replace`, `exclude`, `retract`; the standard library's `vendor/` paths
  - [ ] `GOPROXY` + `direct`, `GOSUMDB`, `GOPRIVATE` and friends, hashes in `gocpp.sum`
  - [ ] language versions per file (`types.Info.FileVersions`), refusal of modules that need a newer Go, `GODEBUG` defaults as `cmd/go` computes them
  - [ ] Go's source for the standard library: the pinned archive, or `GOCPP_GOROOT` with a matching `VERSION`
- [ ] 16.8 GC-mode lowering and emission (5–7 weeks)
  - [ ] the engine flows through `internal/lower` and `internal/cppgen` (types in Go's layout, allocation, closures, interfaces, maps, `go`, channels, `select`, generics, loop variables by file version, `unsafe`)
  - [ ] what the ARC engine gates: local generic types (hoisted with the outer type parameters, named as gc names them), `//go:linkname` of each kind, intrinsics, all of `unsafe`; an engine column in the gate table
  - [ ] type descriptors in `abi.Type` layout with the scheme of 16.2
  - [ ] `#line` for `.go` files, `gopkg_` names
  - [ ] tests: the 55 Go `test/` programs that import nothing (with the spike's stubs); the ARC engine's C++ unchanged; no bdwgc symbol without Go packages

### Part D: Go's standard library on the GC engine (12–17 weeks)
- [ ] 16.9 The replacement checker: `gostd/` and `tools/gostdcheck` (signatures, original-file hashes; Linux and Windows, amd64 and arm64) (1 week)
- [ ] 16.10 `runtime` and the hook table (3–4 weeks)
  - [ ] the public `runtime` API, `runtime/debug`; `runtime/trace` and `runtime/pprof` report "not supported"
  - [ ] natives for the 149 runtime symbols and the pushes: `sync` and `internal/sync` (`procPin` as a per-slot lock, pools emptied before gocpp's collections), `time` (one `startNano`), `internal/poll` stubs, `syscall`, `os`, `internal/godebug`, the FIPS 140 indicator, `iter`, `weak`, `unique`, `internal/synctest` ("no bubble")
  - [ ] the overlays of README §2.7
  - [ ] Windows natives (the 17 bodyless functions that only Windows has)
- [ ] 16.11 `reflect`, maps and interfaces: Go's own code, call thunks, Go's tests by the counting rule (3–4 weeks)
- [ ] 16.12 The bootstrap closure and `gocpp test` for Go packages (2–3 weeks)
  - [ ] `runtime`, `sync`, `time`, `os`, `syscall`, `reflect`, `fmt`, `flag`, `regexp`, `strings`, `strconv`, `io`, `errors`, `testing` with gocpp's `testdeps`
  - [ ] `gocpp test` for Go packages, the rest of Go's `test/` programs, the `go test` comparison tool
  - [ ] Windows
- [ ] 16.13 The standard packages that M10 needs (3–5 weeks)
  - [ ] Go's own tests of the M10 list by the counting rule, with `net` without the poller and `net.Interfaces` (netlink, `GetAdaptersAddresses`)
  - [ ] for the libraries' tests: `go/parser`, `go/printer`, `testing/fstest`, `net/mail`, `encoding/csv`; `net/http`, `crypto/tls`, `go/build`, `go/format` build and initialize
  - [ ] Windows

### Part E: the boundary and M10 (7–11 weeks)
- [ ] 16.14 The gp view and the boundary (4–6 weeks)
  - [ ] the gp view (allowlist of twins, one importer per world, the engine of each expression, reasons) and the allowlist checker
  - [ ] escape summaries for string, slice and interface parameters; the call-only list and its directive
  - [ ] every row of README §2.6, both ways: mirrors with forwarders, graph copies with a memo, `gc.New[T]`, `gc.File`, canonical proxies, gp descriptors for Go types, context adapters with `AfterFunc`, one `startNano`, channels of one layout, `Goexit`, `FuncForPC`, weak links, counted ARC bytes
  - [ ] tests: per row and direction, refusals, `c.JSON` of a gp struct, `c.Keys` and pflag in place, `io.ReadAll`, cycles, root stress
  - [ ] Windows
- [ ] 16.15 M10 libraries: yaml.v3, BurntSushi/toml, `x/text/cases`, protobuf, google/uuid, cobra + pflag (3–5 weeks)
  - [ ] their tests by the counting rule on Linux (GCC, Clang), with check.v1, go-cmp and the generated protobuf test packages
  - [ ] the same on the Windows PC
  - [ ] a gp program uses each through the boundary; build time, size and test time next to `go test`'s

### Part F: M11 (10–14 weeks)
- [ ] 16.16 The netpoller: `epoll`, IOCP, deadlines, 10k idle connections (2–3 weeks)
- [ ] 16.17 `net`, `crypto`, `crypto/tls`, `crypto/x509` and the `x/sys` module overlays (3–4 weeks)
  - [ ] Windows (system roots through the certificate store)
- [ ] 16.18 `net/http`, with the `synctest` tests skipped for that reason (2–3 weeks)
- [ ] 16.19 gin v1.12.0 and v1.9.1 on Linux and Windows (3–4 weeks)
  - [ ] ugorji's runtime pulls, `nomsgpack`, gin's tests by the counting rule; chi, gorilla/websocket, echo
  - [ ] the load test against Go; the budgets measured and recorded in `docs/benchmarks/gc-engine.md`

### Part G: finish (4–8 weeks)
- [ ] 16.20 Tools: vet checks, `explain`, `emit -annotate`, the editor within its budget, debugger printers (2–3 weeks)
- [ ] 16.21 Speed and size: the precise heap if 16.1, 16.15 or 16.19 ask for it, the extra byte, tuning (1–3 weeks)
- [ ] 16.22 Docs, packaging (15.13) and the list of tested modules (1–2 weeks)

## Exit criteria

Tests are counted by Phase 09's rule (ADR-113): a skip counts as a failure unless its reason is
a documented divergence (the internet, the `go` command, gc's runtime internals, gc's allocation
counts, timing, `synctest` bubbles); at least 90 % per standard package and 95 % per module
package.

**M10: pure-Go libraries pass their own tests**

- [ ] GC engine runtime tests pass on Linux (GCC, Clang) and on the Windows PC: roots, finalizers and cleanups, weak and `unique`, registration and unregistration, blocking sections, runtime structures, 10,000 allocating goroutines
- [ ] Stop-the-world pause and memory per thread measured with 10,000 goroutines (blocked and running) on Linux and the Windows PC, under the 16.2 budgets
- [ ] Go's own tests of the 16.13 standard packages pass by the counting rule
- [ ] The 16.15 libraries pass their own tests by the counting rule on Linux and the Windows PC; a gp program uses each through the boundary
- [ ] Boundary suite green: every README §2.6 row both ways, an error test for each refused crossing
- [ ] A program without Go packages builds to the same C++ as before and links no collector (16.8's test)

**M11: `net/http`, TLS, the netpoller and gin on Linux and Windows**

- [ ] Go's tests of `net`, `net/http`, `crypto/tls`, `crypto/x509` pass by the counting rule
- [ ] 10,000 idle connections, and 60 s at 1,000 keep-alive connections with no errors, on Linux and the Windows PC; deadline and close wake-up tests
- [ ] HTTPS: a gocpp server works with `curl` and a Go-built client that trust a test CA, on Linux and Windows; a gocpp client verifies a saved public chain against the system roots without the internet
- [ ] gin v1.12.0 and v1.9.1: the README §2.1 program serves on Linux and Windows; gin's tests pass by the counting rule
- [ ] The gin program against Go: requests per second, p99 latency, memory, GC pauses within the 16.2 budgets, recorded in `docs/benchmarks/gc-engine.md` (16.19)
- [ ] Cold build, warm rebuild (2 s target for a debug build) and binary size within the 16.2 budgets

**Phase done**

- [ ] M10 and M11 proved, the ADR accepted, the 16.20 tools work on the gin program within the editor budget, the guides, the packaging and the list of tested modules published

## Decisions / notes

- 2026-10-24 (your direction): **the phase starts now.** Phase 16 is built in its own clone
  `/home/huuhait/GoToCPP-Gopkg` on branch `p16`, with one worktree per job under
  `~/worktrees/GoToCPP-Gopkg/<job>` (branches `job/<job>`); merges go to `p16`, never to `dev`,
  because another agent builds the old phases in `/home/huuhait/GoToCPP`. No file of an old
  phase is touched. Every build, test and tool run is capped at **16 threads** (`-j16`,
  `go test -parallel 16`, `scripts/gp-focused -cpus 16`). The spike of 16.1 is split into
  independent jobs: 16.1a (vendor and build bdwgc 8.2.12), 16.1b (the dry run of Go's standard
  library) and 16.1c (the GC-engine walking skeleton in the emitter, with the minimal `gp::gc`
  values and a test heap). Further jobs follow when these are reviewed: 16.1d (the bdwgc
  backend of `gp::gc`: roots, thread registration, blocking sections, collection), 16.1e
  (Go-layout values: structs, pointers, slices, maps, interfaces, funcs, `new_`/`make`/`append`),
  16.1f (the four spike packages and the stubs for `runtime`, `internal/bytealg`,
  `internal/reflectlite`), 16.1g (the five boundary shapes), 16.1h (the measurements) and
  16.1i (the Windows items, blocked until the real PC is reachable). The real versions of this
  code land in steps 16.3–16.8.
- 2026-10-24 (16.1b, clean after 2 rounds plus 4 nits): the dry run measured, for go1.25.14
  (identical on go1.25.7), linux/amd64: 303 distinct two-argument `//go:linkname` targets in the
  `net/http` + `crypto/tls` closure; 169 runtime hooks (151 pushed by the runtime, 18 pulled);
  9 synctest-only; 14 linker symbols; 163 bodyless targets (154 provided by the runtime, 9 not,
  and 5 of those need a native or overlay); 176 closure packages; 17 pushes from
  `internal/runtime/maps`; by package 40 `reflect`, 18 `sync`, 12 `syscall`, 11
  `internal/poll`, 7 `time`. The plan's §2.7/§7.1 counts (166/149/17, 178 packages, 19 maps
  pushes, 154/187, 37/18/15/12/7) did not reproduce under any recorded definition; the README
  now carries the measured numbers and says so.
- 2026-10-24 (16.1a, Windows backend): the vendored tree is complete for both of bdwgc's
  Windows thread backends: Win32 threads (23 files, the current `BdwgcSources("windows")`) and
  winpthreads (`CMAKE_USE_PTHREADS_INIT + WIN32`, 25 files, adding `gc_dlopen.c` and
  `specific.c`). MinGW-w64 and llvm-mingw both ship winpthreads, so a CMake configure would take
  the second; M10 records which backend it builds. The cache lives in
  `<cache>/bdwgc/<version>-<hash>/lib/<key>/`, next to `<cache>/rt/`, so a runtime change does
  not invalidate bdwgc; 16.2 decides whether that is the intended "runtime cache".
- 2026-10-24 (16.1a, round 1 review): the build did **not** actually disable bdwgc's
  incremental mode — `gcconfig.h` defines `MPROTECT_VDB`/`SOFT_VDB` itself, and
  `GC_ENABLE_INCREMENTAL=1` turned it on, so bdwgc could take `SIGSEGV`/`SIGBUS` from gocpp's
  runtime. Round 2 adds `-DGC_DISABLE_INCREMENTAL` (verified: `GC_is_incremental_mode()` stays
  0 with `GC_ENABLE_INCREMENTAL=1`), keeps the Windows sources in the vendored tree
  (`win32_threads.c`, a goos-aware source list; the MinGW build itself stays M10 work), and
  leaves bdwgc's `SIGSYS` default under TSan instead of overriding the signal.
- 2026-10-25 (host change): the Linux host was replaced (an AMD EPYC 7W83, 64
  logical cores, about 103 GiB, instead of the 192-core Threadripper). Job 16.1h-prog
  re-measured the whole `docs/benchmarks/gc-engine-spike.md` on the new host, so the
  document is consistent within it; the earlier 16.1h-base numbers were from the old host
  and are not comparable. The 16-thread cap still applies, and the budgets of 16.2 must be
  set from the new host's numbers.
- 2026-10-24 (environment, updated): the real Windows PC is reachable (`GOCPP_WINDOWS_SSH`,
  key auth; the address is not written into the repository). It runs Windows 11 Pro on an
  Intel i5-14400 (10 cores / 16 logical CPUs, 32 GB), with WinLibs MinGW-w64 GCC/G++ 16.1.0,
  Go 1.27.1, cmake 4.4.3 and ninja 1.13.2; it has no clang/llvm-mingw, so MinGW-w64 is the
  Windows toolchain (the plan's primary one). The Linux dev shell also has
  `x86_64-w64-mingw32-gcc` 15.3 and wine 11 for cross-checks. Job 16.1a-win builds bdwgc for
  windows/amd64, runs the smoke test on the PC, and measures thread starts and the static-root
  scan; the full Windows measurements are 16.1h. The dry run for `windows/amd64` (16.1b) needs
  no PC.
- 2026-10-21: phase created from your idea ("a non-GC engine for gp and C/C++, another GC
  engine for Go; then we can import any Go package in the world"). The first draft recorded
  options B1–B4 and eight questions.
- 2026-10-21 (your direction): Go packages are translated to C++ as well (B2), all of them on the
  collector. The real Go toolchain is not part of the picture; B1 stays only an idea for packages
  that the translator cannot handle.
- 2026-10-21 (your answers to the four questions):
  (1) **Memory: two engines in one program.** gp code stays on reference counting, also next to
  Go packages. Go packages and a second copy of the standard library run on the collector, and
  values are converted at the boundary. This was chosen over the whole program on one collector.
  (2) **Collector: Boehm GC (bdwgc) first**, behind a `gp::gc` interface; an own collector
  maybe later.
  (3) **Order: libraries first, then gin.** M10: pure-Go libraries pass their own tests. M11:
  `net/http`, `crypto/tls`, the netpoller, then gin on Linux and Windows.
  (4) **Start: when you decide.** For now the plan only; nothing is scheduled.
- 2026-10-21 (plan written): the README became a full phase: decisions, design, 21 steps in
  parts A–G (41–60 weeks for one person), exit criteria for M10 and M11, risks, evidence. These
  design points are proposals for the ADR of step 16.2, not decisions yet (the review of
  2026-10-22, below, changed several of them):
  - the GC engine uses Go's value layout and type descriptors in `internal/abi.Type` layout,
    so that Go's own `reflect`, `internal/reflectlite` and `internal/runtime/maps` are
    translated rather than rewritten;
  - Go's standard library is Go's own source, with gocpp's `runtime`, a hook table of natives
    (166 `//go:linkname` targets in the `net/http` + TLS closure, 149 of them in the runtime)
    and overlays for the packages whose assembly has no pure-Go fallback;
  - gp code sees one standard library: the gp view type-checks a Go package's declarations
    against gp's standard library, and the boundary converts the twin types;
  - pointers to gp values cross by copy-in/copy-out, and a call whose Go callee keeps the
    pointer is refused with a fix; proxies are canonical; mirror types are written as Go
    source; channels and gp types as Go type arguments are not in the first version;
  - heap scanning in two stages: pointer-free objects never scanned and the rest conservative
    first, a precise heap when the numbers ask for it; stacks always conservative.
- 2026-10-21 (survey of 17 popular modules, 433 packages, and gin v1.9.1, 253 more; type-checked
  with go1.25.7 and gc's build tags, no type errors): gin v1.12.0 is 101 non-standard packages
  (300,472 lines) and 203 standard ones (374,763 lines); gp lacks 141 of those, and a port
  would have to add 122. The standard library is the main blocker (14 of 17 modules; a port
  would add 140 packages for all 17), then `net` (11), the crypto tree (11), `unsafe` (8),
  `reflect` gaps (8) and `net/http` (6). No module needs cgo. 9 modules are 100 % plain Go;
  `modernc.org/sqlite` (7.5 %) is not a target. gin v1.12.0 imports quic-go from `gin.go`, so
  quic-go, `x/net` and `x/sys` must at least build. Details in README §7.1.
- 2026-10-21 (prior art, corrected 2026-10-22): bdwgc runs with OS threads in LLGo (1:1
  threads, the system's libgc 8.0 or later, Windows with MSVC and MinGW) and in Crystal (libgc
  8.2 or later, fibers on threads, `GC_beginthreadex`). TinyGo uses bdwgc without its thread
  support (it serializes the calls and stops the world itself) and runs one thread on Windows.
  None of them pins 8.2.12. `GC_do_blocking` keeps a stop bounded by the running threads, but
  takes the global lock twice per call. Incremental mode stays off (it would take over
  `SIGSEGV`); `THREAD_LOCAL_ALLOC` is essential; thread-local storage is not a root. Whippet
  has no Windows support yet, and MPS supports Windows only with MSVC. Details and sources in
  README §7.2.
- 2026-10-22 (review): five reviewers (the collector and threads, the boundary, the standard
  library and the compiler, the plan as a project, the facts) found 15 serious, about 30 medium
  and about 20 low problems. The serious ones were checked against gocpp's code, Go 1.25's
  source and bdwgc 8.2.12's source; one was re-run here (a registered thread that exits makes
  the next collection abort after about 17 s). All were fixed in the README:
  - the boundary (§2.6): Go memory stays Go memory (maps, slices and pointers work in place); an
    allowlist of twins; copy-in/copy-out only for a shipped list of parameters used only during
    the call (`ShouldBindJSON`, decoders, encoders), with `gc.New[T]` as the fix elsewhere;
    interface values by their dynamic type; slices copied back; graph copies with a memo;
    `time.Time` with one monotonic base; context adapters with `AfterFunc`; channels of one
    layout; the loss of deterministic release and the cross-engine cycles stated;
  - the collector and threads (§2.4, §2.5): unregistration on every exit path; an audit of the
    runtime structures that hold GC pointers; blocking sections only for long waits (the global
    lock); `USE_MMAP` and `USE_MUNMAP`; `GOGC` and the memory limit; cleanups after finalizers;
    weak handles per (base, offset); per-sanitizer builds and `SIGSYS` under `-race`; measured
    per-thread costs;
  - Go's standard library (§2.7, §2.8): the tool tags (`goexperiment.*`), five kinds of
    `//go:linkname` plus intrinsics, a corrected overlay list, `synctest` natives that say "no
    bubble", `procPin` as a lock, Go's exact layout with `[[no_unique_address]]`, a descriptor
    scheme for the ADR, loop variables per file, `GODEBUG`, misaligned access, drift checks;
  - the plan (§3–§6): a bootstrap step for the standard packages that every test needs, the
    boundary after them, the packages that the M10 libraries' tests need moved into M10, a
    tools step, the counting rule and a defined load test, a test CA for HTTPS, Windows items in
    every runtime step, more ADRs for 16.2;
  - the facts of §7: 141 against 122, 149 + 17 link targets, TinyGo, LLGo's new URL, bdwgc's
    8.2.12 paths.
  Now 22 steps and 52–77 weeks; M10 about 47 weeks after the start (38–64), M11 about 59
  (48–80).
- 2026-10-22 (your answers): Windows stays in M10. Standard imports stay "gp's library first,
  else Go's"; when a later port moves a program from Go's copy to gp's, that is documented
  (README §2.3 rule 5, the release notes, `gocpp explain`). Apply all the fixes.
- Open questions for the ADR of 16.2: `abi.Type` descriptors or gocpp's own, and the scheme for
  offsets, `typelinks` and unnamed types; when stage 2 of the heap scan is needed and when the
  extra byte per object can go; misaligned access (`aligned(1)` or `-fno-sanitize=alignment`);
  whether `"go:..."` imports exist; how Go's source is delivered (download, `GOCPP_GOROOT`,
  embedded); the allowlist of twins, the call-only list and its directive, the names of
  `gc.New` and `gc.File`; which channels cross; `-race` and `-asan` for the GC engine, or a
  WORKFLOW exception; the budgets, with per-thread terms and the editor.

## Log

- 2026-10-25: job 16.1h-prog merged into `p16` after a clean review (2 rounds): the goroutine
  program (1k/10k, blocked/running/mixed, depth) and its rows (wall, CPU, RSS, RSS per
  goroutine) are in `tools/gcbench`/`bench/gc`, and the whole benchmark document was
  re-measured on the new host. Go and ARC agree on every checksum and observed count; at 10k
  blocked goroutines the peak RSS per goroutine is 3.5 KiB for Go and 13.2 KiB for ARC
  (128.7 MiB), and the ARC blocked CPU is much higher (5.2 s vs 0.12 s). The stop-the-world
  pause cannot be measured for Go/ARC; it needs the GC engine (16.1h), and the document says so.
- 2026-10-25: job 16.2-draft merged into `p16` after a clean review (3 rounds): the dual-engine
  ADR is recorded as **proposed** in `docs/adr/143-dual-engine.md` with its row in
  `docs/adr/README.md`. It decides every open point of step 16.2 with a recommendation and the
  evidence, keeps the user's fixed decisions fixed, lists the ADR changes (004/007/035/103/106/
  107/109/110, Phase 15 §2.8), and marks every budget TBD with the step that fills it. The
  lifecycle is one: acceptance after 16.1h's budgets and before 16.3; gin/editor targets are
  recorded later in `docs/benchmarks/gc-engine.md` or a dated amendment note.
- 2026-10-25: job 16.1b-tc merged into `p16` after a clean review (2 rounds): the dry run's
  type-check half. A source importer over the tool's own selection (GOROOT/src, `internal/...`,
  per-importing-package `vendor/`, `unsafe` predeclared) type-checks the selected packages with
  `go/types` under the explicit §2.7 tags. Result for go1.25.14: linux/amd64 177 packages
  selected, 182 loaded and checked, **0 real type errors, 0 expected gaps, 0 import failures**;
  windows/amd64 178/183, the same. The expected-gap classes (tag-dropped files, cgo files,
  imports dropped by the tag, dependency failures) are proven by the fake GOROOT and by an
  overlay run; the README states the coverage limits (`_test.go`, arm64, module overlays outside
  GOROOT).
- 2026-10-24: job 16.1c merged into `p16` after a clean review (4 rounds): the GC engine's
  walking skeleton. A hidden `-gc-engine` takes a separate load/check/emit path: stock
  `go/parser` for `.go` files (a `.gp`+`.go` folder is an error, §2.3 rule 1), `go/types` with
  Go's sizes, and a minimal emitter (`gopkg_main`, Go layout, int/string/bool, functions, calls,
  if/for, Go's shift/division semantics, Go's left-to-right evaluation order through preludes,
  string ordering/`+=`, `#line`, a plain `int main()`). `gp::gc` has `string` (16 B),
  `print`/`println`, and the `heap` interface with a malloc test backend. Differential tests
  against `go run` pass with both compilers; a `.gp` program's C++ is unchanged and links no
  `gp::gc` symbol. The reviews forced: `func init` rejected, the arithmetic helpers, the
  evaluation order, string ordering, the flag-aware switch, pure `len`, and positioned errors
  for integer literals that do not fit int64.
- 2026-10-24: job 16.1a-win merged into `p16` after a clean review (3 rounds): bdwgc builds
  for windows/amd64 and the smoke test runs on the real PC and under Wine. CMake on the PC
  showed that MinGW-w64 (winpthreads) takes bdwgc's pthreads+WIN32 branch (25 sources), not the
  23-file Win32-threads list; the Windows defines are pinned. Probes: thread starts ~61–65 µs
  registered at 1,000 (~115 µs at 10,000), static-root scan ~0.53 ms/collection by default and
  ~0.36 ms with `GC_set_no_dls(1)`, which alone also drops the executable's own static roots —
  the executable's writable image must be added with `GC_add_roots`.
- 2026-10-24: job 16.1h-base merged into `p16` after a clean review (2 rounds): the measurement
  harness (`tools/gcbench`), the three Go/gp programs (`bench/gc`) and the Go/ARC baselines
  (`docs/benchmarks/gc-engine-spike.md`). The machine was loaded (1-minute load 44–77 against a
  quiet limit of 24), which the document records; the numbers are a baseline, not a budget.
- 2026-10-24: job 16.1a merged into `p16` after a clean review (3 rounds): bdwgc 8.2.12 is
  vendored with its notice, built opt-in into the cache with the §2.4 flags plus
  `-DGC_DISABLE_INCREMENTAL` and `GC_NO_THREAD_REDIRECTS`, and covered by a threads/roots/
  finalizer/weak-link smoke test at O0/O2/ASan+TSan with both compilers. The Windows sources
  are in the tree; job 16.1a-win builds and runs them on the real PC.
- 2026-10-24: job 16.1b merged into `p16` after a clean review (3 rounds): the dry run and its
  data are in `gostd/dryrun/go1.25.14`, the tool in `tools/gostddryrun`; the plan's §2.7/§7.1
  counts were corrected to the measured ones (see "Decisions / notes").
- 2026-10-24: the phase started (your decision). Jobs 16.1a, 16.1b and 16.1c assigned; the
  workspace and the 16-thread cap are recorded in "Decisions / notes".
- 2026-10-21: phase created (the idea, the options, eight questions).
- 2026-10-21: the full plan written from your four answers, with a survey of 17 Go modules and a
  prior-art study.
- 2026-10-22: reviewed by five reviewers; your two answers (Windows in M10; gp's library first,
  with the engine move documented); all fixes applied: 22 steps, 52–77 weeks.
