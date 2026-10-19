# Phase 15: Optimization, Hardening & Release (v1.0)

> Make gocpp fast, robust, documented and easy to install, then ship **v1.0** with a
> compatibility promise. Afterwards, choose the next big directions based on real usage.

| | |
|---|---|
| Depends on | all previous phases |
| Milestone | **M9: v1.0 released** |
| Size | large (about 9–11 weeks for one person; the four streams below can only overlap with more people) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

Earlier phases chose **correct and simple** over fast whenever there was a conflict:
conservative boxing, `std::shared_ptr` everywhere, `std::unordered_map`, OS-thread goroutines.
That was on purpose, because a correct baseline makes optimization safe: every speedup is
checked against the same test suites, with the real Go toolchain as the reference.

This phase has four streams: **performance**, **robustness**, **platforms and packaging**, and
**docs and release**. They are independent, so they can be done in any order or by different
people, but one person still needs the full 9–11 weeks. Windows is already done (Phase 14), so
the platform stream is mostly Linux arm64 and packaging.

## 2. How it works

### 2.1 Performance: measure first

**Benchmark suite** (`bench/`), each benchmark in gocpp, Go and idiomatic C++:
binary-trees (allocation/refcounting), fannkuch (arrays), n-body and spectral-norm (floats),
mandelbrot, k-nucleotide (maps + strings), regex-redux (`regexp`), a JSON-ish parser, channel
ping-pong (concurrency), plus the `examples/`. Results are tracked per commit on a fixed machine
(shared CI runners are too noisy for a 5 % gate), and a > 5 % regression fails CI.

**Optimizations, in expected-value order** (each one behind the `gp::` API or a lowering
decision, so they're isolated and testable):

| Optimization | What it does |
|---|---|
| **Escape analysis** | Go-style: find locals whose address or closure never escapes (not stored on the heap, not returned, not sent on a channel, not captured by an escaping closure or goroutine, not passed to a "leaking" parameter). Unknown code counts as escaping: C/C++ calls, `unsafe.Pointer` conversions, interface method calls that can't be devirtualized. Locals that don't escape stay on the **stack** with a non-owning `gp::ptr`, so there is no allocation and no refcount traffic. Per-function parameter summaries make it inter-procedural. `-gp:m` prints the decisions, like `go build -gcflags=-m` |
| Pointer receivers as `this` | a non-escaping pointer method becomes a normal member function using `this` (Phase 03 note). **The object must stay alive during the call:** unless the receiver is a caller-local variable whose address is never taken and that no closure captures, the call site holds a strong reference (a copy of the `gp::ptr`) until the call returns. Otherwise the method could drop the last reference itself (`global = nil`) and keep using a freed `this` |
| `const&` parameters | large ARC values (strings, slices, maps, pointers) passed by `const&` instead of a copy, **only** when the argument is a caller-local variable whose address is never taken and that no closure captures. Anything else (globals, fields, elements, captured locals) can be changed or freed by the callee or another goroutine while the call runs: the audit reproduced `f(head)` seeing `n` turn null after `head = nil`, and an ASan use-after-free through `f(box->Child)`. Those arguments are still copied |
| Intrusive refcount (`gp::rc`) | replaces `std::shared_ptr`: string 32 → 24 B, slice 40 → 32 B, cheaper inc/dec. Generated code is unchanged; keep it only if the benchmarks say so |
| Swiss-table maps | replace `std::unordered_map` (Go 1.24 made the same switch); Go-style iteration without the Phase 03 key snapshot |
| Devirtualization | an interface call right after a static conversion calls the method directly |
| Bounds-check help | hoist `len` in range loops, `[[likely]]` on check branches; the C++ optimizer removes the rest |
| Compile time | PCH (Phase 06), `extern template` for hot instances, ccache/sccache support (`GOCPP_CCACHE`), evaluate C++20 modules for the runtime |

The random differential tests (§2.2) get generators aimed at exactly these optimizations:
callees that clear globals or fields their arguments came from, closures that change captured
arguments, and methods that drop the last reference to their receiver. All of them also run
under ASan.

### 2.2 Robustness

- **Fuzzing** (Go native `testing.F`) on parse → check → lower → emit, seeded with Go's own
  test programs. gocpp must never crash. A C++ compile takes about a second, far too slow for
  every fuzz input, so only a **sample** is compiled (every Nth input plus every input that
  reaches new coverage), and that C++ must always compile.
- **Random-program differential testing.** A generator of random *valid* programs in the
  supported subset (in the spirit of Csmith, and of gosmith, which was used to test Go
  compilers). Each program runs under `go run` and `gocpp run`, and the outputs must match. Many
  generated functions are batched into one program so compile time is shared. This catches
  evaluation-order, overflow, conversion and escape-analysis bugs that humans don't think to
  test. Behavior the spec calls implementation-defined (for example out-of-range float → int
  conversions, see §2.3) is kept out of the generator.
- **Go's own `test/` suite.** The Go repo has hundreds of "run" test programs with expected
  output. Port the applicable ones as a conformance suite and track the pass rate (GopherJS and
  gccgo run this suite too). Installed Go toolchains don't always include `test/` (NixOS's
  doesn't, checked in the audit), so the suite downloads the Go source of the pinned version
  into the cache.
- **Nightly sanitizers** over everything: ASan, UBSan, TSan, LSan. The harness fails on any
  sanitizer report, not only on the exit code (Phase 08 uses `_exit`, which sanitizers see, but
  checking the output too is cheap insurance).
- **Leak audit** on large programs to find reference cycles in the runtime and stdlib.

### 2.3 Platforms

Linux and Windows are the targets (your decision); macOS is not.

| Platform | Notes |
|---|---|
| Linux x86-64 | primary, since Phase 02 |
| Windows x86-64 | since Phase 14 (MinGW-w64; tests under Wine in CI, real Windows before releases) |
| Linux arm64 | CI + cross-compile (`-target`). For out-of-range float → int conversions gocpp copies Go's amd64 results everywhere (Phase 02), but Go on arm64 saturates, so on arm64 those cases are skipped in the differential tests |
| Windows arm64 | after 1.0 (llvm-mingw supports it) |
| macOS | not a target. The code stays portable (standard C++, one OS layer), so a port is possible later if wanted |

### 2.4 Packaging and install

- `go install github.com/huuhait/gocpp/cmd/gocpp@latest`. The runtime and stdlib are embedded,
  so it's one binary. For Windows targets (from Linux or on Windows) gocpp downloads its pinned
  llvm-mingw when no toolchain is installed (Phase 06 §2.10); for Linux targets the system
  `g++`/`clang++` is used. `gocpp env` (Phase 06) shows which one was found.
- Release binaries via GoReleaser (linux amd64/arm64, windows amd64) with checksums. The
  Windows zip contains `gocpp.exe`; nothing else is needed (the toolchain is downloaded on
  first use). Each release's notes name its pinned llvm-mingw version.
- Nix flake package (`nix run github:huuhait/gocpp`). On NixOS there's no compiler on the
  `PATH` by default, so the package wraps `gocpp` with Nix's wrapped `clang++`/`g++` (they add
  the right include and library paths). AUR, Scoop or winget packages if people ask.
- `gocpp version` prints the gocpp version, the embedded runtime hash, and the Go version used to
  build it.

### 2.5 Module downloads: `gocpp get`

Go-modules style: `require` versions come from git tags (`vX.Y.Z`), downloads go to
`$GOCPPMODCACHE`, and content hashes are kept in `gocpp.sum`. Versions are chosen with **minimal
version selection** (Go's algorithm, small to implement). `golang.org/x/mod` provides `semver`,
`module`, `zip` and `sumdb/dirhash`, with two catches found in the audit:
`module.CheckPath` rejects paths without a dot (fine for downloaded modules like
`github.com/...`, but local module names like `myapp` must skip it), and `x/mod/zip` only treats
`go.mod` as a module boundary. gocpp therefore filters out nested `gocpp.mod` directories itself
before zipping and hashing. A proxy is a later option.

*Built in 15.14* ([ADR-070](../../../docs/adr/070-version-selection-and-gocpp-get.md), [guide](../../../docs/guides/modules.md)):
`golang.org/x/mod` is **not** a dependency; gocpp has its own semver compare, module path checks, module zip rules
(`modfetch.CreateZip`) and `h1:` hashing (the same hashes as `go.sum`), and MVS over `gocpp.mod` and `go.mod`
files (`internal/mvs`, `internal/modgraph`), tested against the real go command. `gocpp get` (`internal/modget`) is
module-level; `GOCPP_MODPROXY` accepts a `file://` folder.

### 2.6 Documentation

- **A Tour of gocpp** (like A Tour of Go, with interop chapters).
- **Spec**: the language as a diff against Go (from Phase 00, kept current).
- **Interop guide + cookbook**: sqlite, raylib/SDL, std containers, Dear ImGui, wrapping a C library
  as a gocpp package.
- **gocpp for Go developers**: what's different and why (ARC, threads, the reflect subset, ...).
- **gocpp for C++ developers**: how each construct maps to C++, and reading the generated code.
- Stdlib reference generated with `gocpp doc`; FAQ.
- Published as a static site from `docs/`.

### 2.7 v1.0 compatibility promise

Like Go 1: **valid gocpp 1.x programs keep compiling and behaving the same** across 1.x
releases. Not promised: the exact generated C++ and the runtime ABI. Generated code must be
regenerated with the matching gocpp version, and `cpp-lib` headers are versioned.

### 2.8 After 1.0 (each becomes an ADR when chosen)

The MSVC ABI on Windows (`clang-cl`, for C++ libraries built with Visual Studio) · Windows arm64 ·
WebAssembly via emscripten · an optional tracing GC for cycle-heavy programs ·
`-fno-exceptions` embedded mode (panic = abort) · C++20 module output · `net/http` through a C++
library wrapper · self-hosting (compiling the gocpp compiler with gocpp once the stdlib can carry
`go/*`). (A goroutine scheduler is deliberately *not* on this list: `go` stays an OS thread.)

## 3. Steps

1. **15.1 Benchmark suite** + per-commit tracking on a fixed machine + regression gate.
2. **15.2 Escape analysis** + `-gp:m` diagnostics.
3. **15.3 Receiver and parameter optimizations** (`this` with a strong reference at the call
   site, `const&` only for unchangeable caller locals) + the targeted random generators.
4. **15.4 Intrusive refcount experiment** (keep it only if it wins).
5. **15.5 Swiss-table maps.**
6. **15.6 Devirtualization + bounds-check help.**
7. **15.7 Compile-time work** (`extern template`, ccache, modules evaluation).
8. **15.8 Fuzzing** of the whole pipeline (sampled C++ compiles).
9. **15.9 Random-program differential testing** (batched programs).
10. **15.10 Port Go's `test/` run-tests** as a conformance suite (pinned Go source download).
11. **15.11 Nightly sanitizers + leak audit.**
12. **15.12 Platforms**: Linux arm64 in CI; Windows (from Phase 14) in the release pipeline.
13. **15.13 Packaging**: `go install`, GoReleaser, Nix flake with wrapped compilers.
14. **15.14 `gocpp get`** + `gocpp.sum` + MVS (+ the `x/mod` workarounds).
15. **15.15 Docs site** (tour, spec, guides, cookbook, stdlib reference).
16. **15.16 v1.0**: compatibility promise, changelog, release.
17. **15.17 Post-1.0 roadmap** ADRs.

### 3.1 Notes on step 15.5 (Swiss-table maps, [ADR-059](../../../docs/adr/059-swiss-table-maps.md))

Built: `gp::map` is a Swiss table (`runtime/include/gocpp/map.hpp`, `swiss.hpp`): one block of control
bytes and slots, SSE2 (or portable 64-bit) group probing, 7/8 load, a random seed per map, a random
start per `range`, and ranges that hold the table's block, so growth, deletes and `clear` during a loop
keep Go's rules without a key snapshot. The API and the emitted code are unchanged. Tests:
`runtime/test/map_swiss_test.cpp` (and `map_portable_test.cpp`, the same with the portable group):
differential runs against `std::unordered_map`, a model of the range rules, churn, exception safety,
the concurrent-write flag, move-only values; `map_test.cpp` was adapted to the random order.

Measured on this machine (192 cores, shared; `uptime` load average 21 to 118 during the runs, CPU time,
best of 2 to 5 repeats; `tools/mapbench/map_bench.cpp` and `.go`, Go 1.25 with `GOGC=off`, gocpp at
`-O2` with g++ 15). Each cell is gocpp (Swiss) / the old `std::unordered_map` table (dev before 15.5) /
Go, in ns per operation, and the last row is heap bytes per entry:


##### `int` keys, `map[int]int64`: ns per operation (bytes per entry for the last row), gocpp / old gocpp (std::unordered_map) / Go

| operation | n = 10 | n = 1,000 | n = 100,000 | n = 10,000,000 |
|---|---|---|---|---|
| insert (no hint) | 11 / 17 / 34 | 18 / 40 / 55 | 18 / 61 / 46 | 64 / 475 / 119 |
| insert (make with n) | 11 / 18 / 20 | 4.7 / 37 / 18 | 4.9 / 41 / 21 | 54 / 452 / 90 |
| lookup hit | 1.8 / 3.9 / 3.8 | 2.1 / 4.2 / 5.5 | 3.4 / 9.5 / 10 | 46 / 57 / 96 |
| lookup miss | 1.7 / 5.2 / 3.5 | 2.7 / 5.2 / 5.9 | 4.8 / 15 / 21 | 17 / 77 / 74 |
| delete | 9.3 / 13 / 15 | 28 / 51 / 36 | 21 / 57 / 41 | 73 / 174 / 124 |
| `m[k]++` | 1.9 / 3.4 / 7.6 | 2.1 / 4.8 / 8.7 | 3.5 / 11 / 11 | 44 / 68 / 83 |
| range | 2.7 / 1.6 / 8.1 | 1.1 / 2.9 / 6.7 | 2.1 / 8.3 / 4.5 | 5.3 / 94 / 8.1 |
| delete + insert (churn) | 22 / 26 / 20 | 11 / 32 / 33 | 13 / 69 / 34 | 120 / 395 / 263 |
| bytes per entry | - | 35 / 57 / 20 | 22 / 62 / 24 | 28 / 58 / 30 |

##### `string` keys, `map[string]int64`: ns per operation (bytes per entry for the last row), gocpp / old gocpp (std::unordered_map) / Go

| operation | n = 10 | n = 1,000 | n = 100,000 | n = 10,000,000 |
|---|---|---|---|---|
| insert (no hint) | 18 / 34 / 38 | 43 / 60 / 61 | 54 / 107 / 56 | 239 / 626 / 256 |
| insert (make with n) | 21 / 34 / 28 | 19 / 56 / 21 | 14 / 67 / 23 | 170 / 654 / 101 |
| lookup hit | 5.8 / 24 / 6.8 | 5.9 / 28 / 6.7 | 9.1 / 42 / 12 | 80 / 342 / 86 |
| lookup miss | 5.1 / 23 / 5.8 | 5.4 / 24 / 6.6 | 9.9 / 68 / 20 | 33 / 447 / 69 |
| delete | 12 / 32 / 21 | 67 / 94 / 50 | 57 / 112 / 47 | 109 / 422 / 126 |
| `m[k]++` | 7.0 / 24 / 12 | 7.0 / 24 / 11 | 12 / 42 / 15 | 100 / 321 / 107 |
| range | 4.1 / 3.9 / 7.6 | 1.4 / 4.5 / 5.7 | 2.5 / 16 / 4.4 | 7.4 / 123 / 8.8 |
| delete + insert (churn) | 24 / 60 / 24 | 21 / 67 / 37 | 42 / 101 / 43 | 219 / 901 / 305 |
| bytes per entry | - | 84 / 72 / 38 | 54 / 78 / 35 | 69 / 74 / 45 |

##### `struct` keys, `map[struct]int64`: ns per operation (bytes per entry for the last row), gocpp / old gocpp (std::unordered_map) / Go

| operation | n = 10 | n = 1,000 | n = 100,000 | n = 10,000,000 |
|---|---|---|---|---|
| insert (no hint) | 13 / 20 / 41 | 23 / 47 / 56 | 34 / 65 / 55 | 77 / 463 / 140 |
| insert (make with n) | 13 / 20 / 29 | 7.7 / 38 / 20 | 8.9 / 58 / 32 | 68 / 479 / 106 |
| lookup hit | 6.0 / 6.2 / 8.2 | 5.4 / 7.4 / 7.2 | 8.5 / 18 / 14 | 83 / 101 / 85 |
| lookup miss | 5.1 / 7.0 / 5.9 | 4.6 / 7.5 / 6.2 | 8.8 / 19 / 21 | 24 / 155 / 65 |
| delete | 13 / 15 / 19 | 52 / 54 / 48 | 38 / 59 / 50 | 95 / 220 / 127 |
| `m[k]++` | 6.3 / 7.4 / 8.8 | 5.8 / 9.1 / 8.4 | 8.9 / 17 / 16 | 85 / 121 / 83 |
| range | 3.6 / 1.7 / 7.7 | 1.1 / 3.0 / 5.8 | 2.3 / 7.8 / 4.6 | 7.0 / 104 / 8.2 |
| delete + insert (churn) | 24 / 32 / 24 | 26 / 36 / 33 | 27 / 69 / 48 | 164 / 542 / 291 |
| bytes per entry | - | 51 / 57 / 38 | 33 / 62 / 35 | 42 / 58 / 45 |

load at the runs: new/Go min 21 max 118; old min 21 max 118

Honest gaps: a string key costs 54 to 84 bytes per entry against Go's 35 to 45 (gocpp's `string` is 32
bytes, D15); deleting string keys from a 1,000-entry map is slower than Go (67 against 50 ns); tiny maps
(10 entries) range slower than the old table (the loop's atomic increment on the block). Everywhere
else gocpp is ahead of Go or within noise of it (tiny-map churn: 22 against 20 ns), and ahead of the old
table by 1.5x to 20x (insert 7x and range 18x at 10 million entries: the old table's per-entry nodes). The 10-million-entry columns are the most affected by the
machine's load (cache and memory bandwidth are shared).

## 4. Deliverables

- Benchmark suite with history; optimized runtime and lowering
- Fuzzers, random-program differential tester, Go `test/` conformance suite
- Release binaries, Nix package, `gocpp get`
- Documentation site; v1.0 release

## 5. Exit criteria (v1.0)

- Benchmarks are published. Goal: geometric mean at or below Go's runtime on CPU-bound
  benchmarks, with no benchmark more than 2× slower than Go (allocation-heavy ones are the risk).
  Hard rule: no unexplained regression > 5 % between releases.
- ≥ 90 % of the applicable Go `test/` run-tests pass.
- 24 h of fuzzing without a crash; ≥ 1M random differential programs (over many nightly runs)
  with every mismatch triaged; the optimization-targeted generators are clean under ASan.
- CI green on Linux x86-64, Linux arm64 and Windows x86-64 (Wine in CI, plus the real-Windows
  release checklist from Phase 14).
- Installable via `go install`, release binary, and `nix run` (also on NixOS). The docs site is
  live.

## 6. Risks

| Risk | Mitigation |
|---|---|
| ARC is slower than Go's GC on allocation-heavy code | Escape analysis + intrusive RC + safe `const&`; document the trade-off honestly; optional GC after 1.0 |
| Escape analysis or `const&`/`this` bugs → use-after-free | Conservative by default (unknown code escapes, only unchangeable caller locals skip the copy); ASan nightly; targeted random generators |
| Platform differences (libstdc++ vs libc++, MinGW vs glibc) | One OS layer in the runtime; every platform in CI; runtime abstractions for stack traces and float formatting |
| Release work crowds out quality | The release is gated only by the exit criteria |
