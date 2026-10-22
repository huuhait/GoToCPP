# Phase 16: Dual engine · STATE

**Status:** `[ ]` planned in full (2026-10-21), not scheduled: the phase starts when you decide
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-21

## Steps

- [ ] 16.1 Spike: one Go package on bdwgc, called from gp
  - [ ] bdwgc 8.2.12 built with the flags of README §2.4 (Linux GCC + Clang, Windows llvm-mingw on the real PC)
  - [ ] throwaway GC mode: `container/list`, `strings`, `strconv`, `sort`, binarytrees
  - [ ] five boundary shapes by hand (rooted `gp::ptr`, string both ways, borrowed `[]byte`, copy-in/copy-out, gp closure as a Go func)
  - [ ] measurements: allocation and peak memory, stop-the-world with 1k/10k goroutines, boundary costs, compile time, collector size, ASan/TSan
  - [ ] dry run of Go's standard library: bodyless functions, `//go:linkname`, unsupported constructs
- [ ] 16.2 ADR: the design, the budgets, and what changes in ADR-004, 103, 106, 107, 109, 110
- [ ] 16.3 `gp::gc` and bdwgc (vendored, runtime cache, roots, finalizers + cleanups + weak links, statistics, limits)
- [ ] 16.4 Threads and stops (registration, blocking sections, signals, rooted per-thread records, rooted panic value)
- [ ] 16.5 The GC engine's values (Go layout, allocation, `append`, slicing, checks, `chan_core` in the GC heap)
- [ ] 16.6 Panics, `defer`, `recover` across the engines
- [ ] 16.7 Loading Go packages and modules
  - [ ] `.go` packages: stock `go/parser`, build constraints, `_test.go`, `//go:embed`, cgo files, `.s` files, directives
  - [ ] module rules (README §2.3), MVS with `replace`, `exclude`, `retract`
  - [ ] `GOPROXY` + `direct`, `GOSUMDB`, `GOPRIVATE` and friends, hashes in `gocpp.sum`
  - [ ] the `go` line: language version, loop variables, refusal of modules that need a newer Go
  - [ ] Go's source for the standard library: the pinned archive or `GOCPP_GOROOT`
- [ ] 16.8 GC-mode lowering and emission
  - [ ] the engine flows through `internal/lower` and `internal/cppgen` (types, allocation, closures, interfaces, maps, `go`, channels, `select`, generics, `unsafe`)
  - [ ] Go features that the ARC engine gates (generic types inside functions, `//go:linkname` pulls, all of `unsafe`); an engine column in the gate table
  - [ ] type descriptors in `abi.Type` layout, with the `resolve*Off` tables
  - [ ] `#line` for `.go` files, `gopkg_` names
  - [ ] Go's `test/` programs (`tools/gotest`) in GC mode
- [ ] 16.9 The gp view and the boundary
  - [ ] the gp view (declarations against gp's standard library, twin types, reasons)
  - [ ] every row of README §2.6, both ways
  - [ ] plain pointers for Go values in non-escaping gp locals
  - [ ] tests: per row and direction, refusals, cycle leak, root stress
- [ ] 16.10 `gocpp test` for Go packages (+ the `go test` comparison tool)
- [ ] 16.11 The replacement checker (`gostd/`, `tools/gostdcheck`)
- [ ] 16.12 `runtime` and the hook table
  - [ ] public `runtime` API, `runtime/debug`, the functions notable modules pull (go.dev/issue/67401)
  - [ ] natives for the 166 runtime symbols (`sync` incl. `procPin`/`Pool`, `time`, `internal/poll` stubs, `syscall`, `os`, `iter`, `weak`, `unique`)
  - [ ] overlays (`sync/atomic`, `internal/bytealg`, `internal/cpu`, `math`, `hash/crc32`, `syscall`, `internal/chacha8rand`, `os/signal`, `os/exec`)
- [ ] 16.13 `reflect`, maps and interfaces (Go's own code, call thunks, Go's tests)
- [ ] 16.14 The standard packages that M10 needs (Go's own tests, reviewed list)
- [ ] 16.15 M10 libraries: yaml.v3, BurntSushi/toml, `x/text/cases`, protobuf, google/uuid, cobra + pflag
- [ ] 16.16 The netpoller (`epoll`, IOCP, deadlines, 10k idle connections)
- [ ] 16.17 `net`, `crypto`, `crypto/tls`, `crypto/x509`
- [ ] 16.18 `net/http`
- [ ] 16.19 gin v1.12.0 and v1.9.1 on Linux and Windows (+ chi, gorilla/websocket, echo; load test against Go)
- [ ] 16.20 Speed and size (precise heap if needed, budgets, `docs/benchmarks/gc-engine.md`)
- [ ] 16.21 Docs and the list of tested modules

## Exit criteria

**M10: Go libraries pass their own tests**

- [ ] GC engine runtime tests pass on Linux (GCC, Clang) and on the Windows PC
- [ ] Stop-the-world pause measured with 10,000 goroutines (blocked and running), under the 16.2 budget
- [ ] Go's own tests of the 16.14 standard packages pass (reviewed list with reasons)
- [ ] The 16.15 libraries pass their own tests on Linux and the Windows PC; a gp program uses each through the boundary
- [ ] Boundary suite green: every README §2.6 row both ways, a refusal test for each refused crossing
- [ ] A program without Go packages builds to the same C++ as before and links no collector

**M11: `net/http`, TLS and gin on Linux and Windows**

- [ ] Go's tests of `net`, `net/http`, `crypto/tls`, `crypto/x509` pass (reviewed list)
- [ ] 10,000 idle connections and a load test on Linux and the Windows PC; deadline and close wake-up tests
- [ ] An HTTPS server built by gocpp works with `curl` and a Go-built client, with the system's roots, on Linux and Windows
- [ ] gin v1.12.0 and v1.9.1: the README §2.1 program serves on Linux and Windows; gin's tests pass (reviewed list)
- [ ] The gin program against Go: requests per second, p99 latency, memory, GC pauses within the 16.2 budgets
- [ ] Cold build, warm rebuild and binary size within the 16.2 budgets

**Phase done**

- [ ] M10 and M11 proved, the ADR accepted, the guides and the list of tested modules published

## Decisions / notes

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
  design points are proposals for the ADR of step 16.2, not decisions yet:
  - the GC engine uses Go's value layout and type descriptors in `internal/abi.Type` layout,
    so that Go's own `reflect`, `internal/reflectlite` and `internal/runtime/maps` are
    translated rather than rewritten;
  - Go's standard library is Go's own source, with gocpp's `runtime`, a hook table of natives
    (166 symbols in the `net/http` + TLS closure) and overlays for the packages whose assembly
    has no pure-Go fallback;
  - gp code sees one standard library: the gp view type-checks a Go package's declarations
    against gp's standard library, and the boundary converts the twin types;
  - pointers to gp values cross by copy-in/copy-out, and a call whose Go callee keeps the
    pointer is refused with a fix; proxies are canonical; mirror types are written as Go
    source; channels and gp types as Go type arguments are not in the first version;
  - heap scanning in two stages: pointer-free objects never scanned and the rest conservative
    first, a precise heap when the numbers ask for it; stacks always conservative.
- 2026-10-21 (survey of 17 popular modules plus gin v1.9.1, 433 packages type-checked, no type
  errors): gin v1.12.0 is 101 non-standard packages (300,472 lines) and 203 standard ones
  (374,763 lines), and gp lacks 122 of those. The standard library is the main blocker (14 of
  17 modules; 140 missing packages for all 17), then `net` (11), the crypto tree (11), `unsafe`
  (8), `reflect` gaps (8) and `net/http` (6). No module needs cgo. 9 modules are 100 % plain Go;
  `modernc.org/sqlite` (7.5 %) is not a target. gin v1.12.0 imports quic-go from `gin.go`, so
  quic-go, `x/net` and `x/sys` must at least build. Details in README §7.1.
- 2026-10-21 (prior art): bdwgc 8.2.12 is the only collector proven on Linux and Windows with
  MinGW-w64 and llvm-mingw and with a thread per goroutine (LLGo, TinyGo, Crystal ship it).
  `GC_do_blocking` keeps a stop bounded by the running threads. Incremental and parallel
  marking do not combine; `THREAD_LOCAL_ALLOC` is essential; thread-local storage is not a
  root. Whippet has no Windows support yet, and MPS supports Windows only with MSVC. Details and
  sources in README §7.2.
- Open questions for the ADR of 16.2: `abi.Type` descriptors or gocpp's own; when stage 2 of the
  heap scan is needed; whether `"go:..."` imports exist; how Go's source is delivered (download,
  `GOCPP_GOROOT`, embedded); whether the copy-in/copy-out refusal is precise enough for
  `ShouldBindJSON` and `json.Unmarshal`; whether mirrors get forwarding methods; `-race` and
  `-asan` for the GC engine; the budgets.

## Log

- 2026-10-21: phase created (the idea, the options, eight questions).
- 2026-10-21: the full plan written from your four answers, with a survey of 17 Go modules and a
  prior-art study.
