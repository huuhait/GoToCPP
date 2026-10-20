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

*Built in step 15.1* (docs/benchmarks/README.md has the full description): 54 programs in `bench/programs/`
(cpu, data, dispatch, runtime, generics; each one `.gp` file that is also valid `.go`), 32 with a C++20 twin in
`bench/cpp/`, run by `tools/gobench` on gocpp (g++ and clang++, in the flavors `default` = -O0 -g, `O2`, and `release` =
`build -release`), Go and the twins, with outputs compared across all of them, medians/MAD, memory, binary
sizes and compile times. `make bench-smoke` (in `make ci`), `make bench`, `make bench-check` (the 5 % gate: median,
fastest run and MAD must agree, drift-corrected by Go and the twins, must reproduce, inconclusive on a busy or different
machine), `make bench-history` (`bench/history/*.jsonl`). Every program builds today; regex-redux (`regexp`) and the JSON-ish parser
(`encoding/json`) are to be added when those packages build. The baseline (`bench/baseline.json`) committed now comes from a
loaded shared machine (load 67 to 240 on 192 cores, unpinned): the authoritative one is written on the fixed, quiet benchmark
machine at the end of this phase. First findings (default build 9 times slower than Go, optimized builds 1.5 times Go and at Go's
speed on the CPU kernels; the worst gaps are panic/recover, calls through embedded pointers and interfaces, goroutine
hand-offs, slice allocation against C++): docs/benchmarks/README.md.

**Optimizations, in expected-value order** (each one behind the `gp::` API or a lowering
decision, so they're isolated and testable):

| Optimization | What it does |
|---|---|
| **Escape analysis** | Go-style: find locals whose address or closure never escapes (not stored on the heap, not returned, not sent on a channel, not captured by an escaping closure or goroutine, not passed to a "leaking" parameter). Unknown code counts as escaping: C/C++ calls, `unsafe.Pointer` conversions, interface method calls that can't be devirtualized. Locals that don't escape stay on the **stack** with a non-owning `gp::ptr`, so there is no allocation and no refcount traffic. Per-function parameter summaries make it inter-procedural. `-gp:m` prints the decisions, like `go build -gcflags=-m` |
| Pointer receivers as `this` (*built in 15.3 as borrowed pointers, [ADR-086](../../../docs/adr/086-borrowed-pointer-parameters-and-receivers.md): a raw `gp::bptr<T>`, not `this`*) | a non-escaping pointer method becomes a normal member function using `this` (Phase 03 note). **The object must stay alive during the call:** unless the receiver is a caller-local variable whose address is never taken and that no closure captures, the call site holds a strong reference (a copy of the `gp::ptr`) until the call returns. Otherwise the method could drop the last reference itself (`global = nil`) and keep using a freed `this` |
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
  *Built in 15.8* (`tools/fuzz`, `docs/guides/fuzzing.md`): targets for the parser, the formatter,
  the checker, lower+emit, the emitter with the sampled compile (g++ and clang++,
  `-fsyntax-only`, one program in N picked by a hash of its bytes: a test process cannot see the
  engine's coverage, so "every input that reaches new coverage" is not done; the seed corpus is
  compiled by the dedicated tests instead), a generator of valid programs (`tools/fuzz/gen`,
  tested against `go/types`), the language server, the C++ error mapper and `gocpp.mod`;
  `make fuzz-short` and `make fuzz-long` run campaigns that dedupe findings by stack signature.
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
  *Built in 15.11* (`tools/sanitize`, [guide](../../../docs/guides/sanitizers.md),
  [ADR-090](../../../docs/adr/090-nightly-sanitizers-and-a-bounded-leak-policy.md)): `make sanitize-nightly`
  (stages `canary`, `leaks`, `asan`, `ubsan`, `tsan`, `deep-sanitize`, `deep-race`) runs `tests/run` under `-asan`, UBSan alone and
  `-race` with both compilers and fails on any report in a program's stderr; canaries (programs that are
  wrong on purpose) must be reported or the job is blind; the leak audit bounds the 18 programs that say
  `// lsan: cycle` by a ceiling per compiler in `tools/sanitize/leaks.txt` (more than the ceiling, or no leak
  at all, fails). `tools/sanitize/nightly.sh` and `.github/workflows/sanitizers-nightly.yml` start it.

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

*Built in step 15.13* ([ADR-089](../../../docs/adr/089-packaging-go-install-goreleaser-nix.md), `docs/guides/packaging.md`):
`.goreleaser.yaml` (static `CGO_ENABLED=0` builds for linux amd64/arm64 and windows amd64, tar.gz and zip with `LICENSE` and
`README.md`, `checksums.txt`, a draft release; `goreleaser check` and `release --snapshot` pass), the flake's `packages`/`apps`
(`buildGoModule` with `proxyVendor`, wrapped with Nix's g++, clang++, binutils, llvm-ar and pkg-config; `nix build` and `nix run`
work, the install check builds and runs a program with both compilers), and `go install ...@version`, which the repository did
not allow before: 11 test files that differed from others only in case and one file name with an apostrophe were refused by the
module zip rules (renamed; the apostrophe is now made by a test). `internal/packaging` runs a real `go install @v0.0.1-rc.1`
against a local `file://` proxy, builds with the GoReleaser ldflags, and guards go.mod, the config and the flake's `vendorHash`
(`make package-check` adds goreleaser and nix). `internal/version.Version` takes the installed module version, and `gocpp version`
prints a second line `runtime <hash>`. Not done here: the Windows host's pinned toolchain download (the zip needs MinGW-w64 on the
`PATH`), MinGW in the Nix package, the release pipeline (15.12 and 15.16); nothing was published.

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
The proposals written for step 15.17 are in [§2.9](#29-post-10-roadmap).

### 2.9 Post-1.0 roadmap

Step 15.17 turns the directions above, and the open questions found in the repository, into
**proposed** ADRs: each one has the facts it rests on (with the files they come from), the options, a
recommendation and the trigger that would make the decision urgent. None of them is decided: the user
accepts, changes or drops each one. The numbers are measurements of `dev` on 2026-10-20 and are marked
where they are estimates.

| ADR | Direction | Recommendation in one line |
|---|---|---|
| [103](../../../docs/adr/103-reference-cycles-stay-counted-common-shapes-get-compiler-fixes-and-a-cycle-report-comes-before-any-collector.md) | Memory: reference cycles (D1) | stay counted; fix recursive closures and `sync.Cond` at compile time; a cycle report before any collector |
| [104](../../../docs/adr/104-goroutines-stay-threads-the-main-goroutine-gets-a-large-stack-and-a-scheduler-needs-new-evidence.md) | Goroutines and stacks (D7, D24) | keep threads (Q4); give the main goroutine a large stack; a scheduler needs new evidence |
| [105](../../../docs/adr/105-generic-type-instances-get-out-of-line-bodies-and-c-20-modules-wait-for-a-measurement.md) | Generics compile cost, C++20 modules | out-of-line bodies for generic type instances; modules only after a measured prototype |
| [106](../../../docs/adr/106-reflect-grows-method-and-call-for-types-the-program-already-has.md) | `reflect` completeness (D6) | `Method`/`Call` for types the program has, when `text/template` is ported; never `StructOf`/`FuncOf` |
| [107](../../../docs/adr/107-the-standard-library-beyond-tier-2-goes-by-blockers-not-by-popularity.md) | Standard library beyond tier 2 | waves by blocker class; `net/http` and TLS stay C++ library modules |
| [108](../../../docs/adr/108-typed-interop-becomes-the-default-when-three-measurements-pass-and-the-clang-policy-is-a-matrix.md) | Typed interop default, clang policy | flip to `auto` after a clang matrix, cold probing and Windows probes pass |
| [109](../../../docs/adr/109-after-1-0-the-pipeline-and-arm64-come-first-macos-needs-a-machine-and-a-checksum-database-needs-an-ecosystem.md) | Platforms, release pipeline, module trust, debugger | Q9 and the pipeline, arm64, macOS only with a Mac maintainer, sumdb when modules exist |
| [110](../../../docs/adr/110-the-go-language-version-moves-once-a-year-and-the-v1-promise-names-what-it-covers.md) | Go version policy, what the v1 promise covers | one Go version, moved yearly; the promise lists covered and uncovered surfaces |

Candidates that were looked at and not written as ADRs: a goroutine scheduler as its own direction
(ADR-104 covers why not), self-hosting, WebAssembly and `-fno-exceptions` mode (no evidence in the
repository beyond the backlog line), the MSVC ABI and Windows arm64 (one line each in ADR-109).

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
    Built as `tools/gotest` (docs/guides/conformance.md): classifier + manifest, runners for the
    run/errorcheck/compile/dir directives, failure clusters with owner steps (`expect.txt`), snapshot
    `docs/conformance/go-test.md`, opt-in gate `make conformance-check` (`passing.txt` may not shrink).
11. **15.11 Nightly sanitizers + leak audit.**
12. **15.12 Platforms**: Linux arm64 in CI; Windows (from Phase 14) in the release pipeline.
13. **15.13 Packaging**: `go install`, GoReleaser, Nix flake with wrapped compilers.
14. **15.14 `gocpp get`** + `gocpp.sum` + MVS (+ the `x/mod` workarounds).
15. **15.15 Docs site** (tour, spec, guides, cookbook, stdlib reference).
16. **15.16 v1.0**: compatibility promise, changelog, release.
17. **15.17 Post-1.0 roadmap** ADRs. Written as proposals ADR-103 to ADR-110, listed in [§2.9](#29-post-10-roadmap).

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
### Notes on step 15.9 (random-program differential testing)

**15.9 Random-program differential testing** ([guide](../../../docs/guides/difftest.md),
`tools/difftest`): a seeded generator of valid Go programs whose behavior the specification
defines (`tools/difftest/gen`: integer wrap, shifts and division of every width, strings and
runes, slice aliasing with a length/capacity model, arrays and structs, maps, pointers, methods
and interfaces, embedding, type switches, closures with Go 1.22 loop variables, defer, panic and
recover, the run-time panics, labels, constants, generic functions, goroutines with channels and
`select`, `fmt` verbs, complex numbers, the fmt methods, map keys, float conversions, invalid UTF-8,
`println`, the ported standard packages), parameterized by the features of the phase that gocpp has reached
(`-phase`, `-features`); a runner that builds 100 to 200 programs as one bundle of 8 files with
real Go and with gocpp (`tools/difftest/diff`), compares the output of each program, finds
crashes, hangs and build failures by program, resumes from per-bundle state files, and minimizes
every mismatch with an AST-level reducer (at most 100 gocpp builds) before classifying it
against `tools/difftest/known.txt` or writing a finding to `tools/difftest/findings/` (ignored
by git). `-asan` builds the programs with the sanitizers: any report fails. The generator is
checked three ways: unit tests (determinism, 2000 programs type-check, a static lint for
forbidden constructs, feature gating), real Go on thousands of programs (every program must
build and run twice with the same output: counted as `generator bugs`), and seeded bugs in a
scratch copy of the compiler (`DIFFTEST_MUTANTS=1 go test -run SeededBugs ./tools/difftest/diff`).
Campaign numbers, the seven injected bugs, the rate (212 programs a minute, 3.5 CPU seconds a
program) and the known differences are in the guide. The 1M-program exit criterion is a nightly
job (`go run ./tools/difftest -n 1000000 -j 8`, resumable, about 960 CPU hours) that adds up over
many nights.

### 3.2 Notes on step 15.3 (borrowed pointers, [ADR-086](../../../docs/adr/086-borrowed-pointer-parameters-and-receivers.md))

**Evidence qualification (2026-10-20):** the performance table and early development reports
below predate the final ownership, ordering and native-fmt integration fixes; they are not fresh
measurements of the final integrated tree. Whole-net correctness review is clean at `e91d0ced`.
The corrected differential runner requires successful generation, Go and candidate execution,
matching output and no sanitizer diagnostic. A bounded manager smoke used one fixed pointer
seed and one value seed (eight generated cases each), with four successful GCC/Clang ASan
executions against two successful Go references. This is not a repeat of the historical campaign
or completion of the v1.0 fuzzing/benchmark exit criteria. The formerly intentional-race hot-load
fixtures are now synchronized finite positives, with separate isolated emission controls.

Built: a pointer parameter or receiver that the callee only reads and writes through, compares, or lends on to
other borrowing callees (it is never stored, returned, captured, converted to an interface, used by `go`/`defer`,
passed to an owning parameter or reassigned) is a `gp::bptr<T>`, a raw pointer with `gp::ptr`'s nil checks and no
reference count (`internal/lower/borrow.go`: an intra-procedural analysis of the callee's body with a package-wide
fixpoint over the call graph, the packages in the order of their imports). The call lends the caller's pointer when
the object stays alive during the call: a temporary, a local that no call can change, a box, a package variable, the
address of a field or element of such an object (`gp::borrow(&p->f)`, instead of a `gp::field_ptr` aliasing pointer
for every promoted method), or any pointer loaded from memory when the callee is *pure* (it stores no pointer,
slice, map, interface, func or channel except into its locals and runs no unknown code); otherwise it copies the
pointer first, as every call did (`internal/lower/borrowemit.go`). Interface thunks borrow from the interface value
(`gp::unbox_borrow`), and a call copies an interface value that the method could change. `go`, `defer`, func values,
method values and promoted-method forwarders adapt. `gocpp emit -annotate` says for each pointer parameter whether it
is borrowed and why not, and how each call lends. Tests: `TestBorrow` (the analysis), `tests/run/borrow_*`
(a callee that drops the last other reference to its receiver or argument while it runs, nil receivers, embedding,
func values, goroutines, interfaces) and `tests/pkgs/borrow_cross`, under `-asan`, `-race` and on the real Windows
PC, and mutations of the analysis and of the lending that each fail a test.

Measured on the 15.1 suite (`taskset` on one idle CPU, median of 9 alternating runs, CPU seconds; the machine was
shared with other jobs at a load of 100 to 180 on 192 cores, so a ratio is good to about 15%; Go 1.25.14). Each
cell is the time as a multiple of Go's: before 15.3 / after the pointer parameters (15.3) / after the const references
(15.3b, below):

| benchmark | Go (s) | g++ -O2 | g++ -release | clang++ -O2 | clang++ -release |
|---|---|---|---|---|---|
| `embedding` | 0.128 | 21.7 / 3.4 / 3.4 | 20.7 / 3.1 / 3.2 | 24.2 / 3.1 / 3.0 | 22.9 / 2.8 / 3.0 |
| `iface_call` | 0.393 | 5.9 / 5.0 / 5.1 | 4.4 / 3.2 / 3.2 | 7.1 / 3.7 / 4.1 | 6.1 / 3.0 / 3.4 |
| `method_value` | 0.362 | 2.8 / 1.5 / 1.4 | 2.7 / 1.5 / 1.5 | 3.0 / 1.4 / 1.4 | 2.8 / 1.2 / 1.1 |
| `type_switch` | 0.338 | 4.7 / 4.8 / 3.1 | 4.4 / 4.4 / 2.6 | 5.2 / 5.2 / 3.2 | 4.7 / 4.8 / 2.8 |
| `closures` | 0.189 | 3.2 / 3.1 / 3.4 | 3.4 / 3.3 / 3.3 | 3.6 / 3.7 / 3.6 | 3.2 / 3.3 / 3.6 |
| `defer_loop` | 0.087 | 7.5 / 7.2 / 6.6 | 6.5 / 5.4 / 4.9 | 8.4 / 8.0 / 7.2 | 6.3 / 6.4 / 5.4 |
| `bytes_buffer` | 0.157 | 17.0 / 10.0 / 9.5 | 10.4 / 6.4 / 6.2 | 9.1 / 6.1 / 6.0 | 8.4 / 5.8 / 5.1 |
| `tree_alloc` | 0.279 | 2.0 / 1.5 / 1.5 | 1.9 / 1.5 / 1.4 | 1.4 / 1.4 / 1.3 | 1.5 / 1.3 / 1.7 |
| `list_alloc` | 0.207 | 1.4 / 1.4 / 1.4 | 1.9 / 1.8 / 1.7 | 2.0 / 1.9 / 2.0 | 1.9 / 2.0 / 1.9 |
| `binarytrees` | 0.989 | 0.7 / 0.6 / 0.5 | 0.7 / 0.6 / 0.6 | 0.7 / 0.6 / 0.6 | 0.7 / 0.6 / 0.6 |

`embedding` and `method_value` are the pointer-receiver cases. `iface_call` stays at 3.7 to 5 times Go's at `-O2`
and 3 times in `-release`: the thunk and the method are in different translation units (LTO inlines them in
`-release`), and the loop copies the interface value out of the slice (`s := shapes[i]`), a count increment and
decrement that only escape analysis (15.2) can remove. `closures` is about captured variables and `defer_loop` is the
defer frame; none is a pointer parameter.

Step 15.3b (same ADR, section 5) is the other half of the plan's `const&` row: a string, slice, map, channel, func,
interface or kept pointer parameter that the body never assigns or returns as it is is a `const T&`, and the call
passes a temporary, a constant or an unchangeable local as it is and anything else as a copy (or as it is, to a
callee that stores no string, pointer, slice, map, interface, func or channel). `type_switch` (an `any` parameter)
and `bytes_buffer` (slices) gain; a parameter that is returned as it is stays by value, because `insert(t.left, k)`
of a tree moves it out (`tree_alloc` regressed by 15% when that was not so). A func value or a callback of such a
function is `gp::adapt<&f>::call`, one adapter for each function, so `%p` of the function stays one address.

Fix round of 15.3 (review: two blockers in call-site lowering, both found by the reviewer's random programs). (1) A
variadic callee's fixed pointer parameters were never lent or copied (`lendArgs` returned early for every variadic
call), so `use(g, 1, 2)` with a `use` that drops `g` freed the object it was reading; now the fixed positions are
processed and only the packed trailing arguments are left alone. (2) The lambda of a directly called method
expression (`(*T).M(&T{}, g)`) bound its `auto&&` parameters to the caller's variables; it now takes the receiver and
every borrowed or const reference parameter by value. Also: `unsafe.Pointer` is an owner (`mayOwn`), the lldb printer
knows `gp::bptr`, and the permanent tests `borrow_{variadic,methodexpr,calls,unlink,elems,callpaths,closures,
ifacebump}` (outputs from real Go; `-asan` and `-race` runs; four are in the Windows sample) plus
`tools/borrowfuzz` (the early runner reported 80 pointer and 30 value seeds as equal to Go;
its incomplete stage/failure accounting means that report alone does not establish successful
execution—see the corrected-runner evidence above). Mutants that survived the first round now fail a test (package variable always stable,
interface thunk not copying, `pathStable` always true, always lending to an impure callee, no copy of a const
reference receiver); `noCalls` always true survives by design: `order.go` already moves every call next to a lent
operand into a temporary, so the guard is unreachable today (logged over the borrow tests, the goldens and 110
random programs: no site changes) and stays as a guard against a change of the order pass. `map_int` was reported
+17-29% with byte-identical generated C++; re-measured against current dev with the rebased branch (alternating
builds, 15 runs, two rounds, `-O2`): 70-73 ms against 73-94 ms for dev, no regression, so it was runtime header
layout noise of the first measurement.

What was left of 15.3 and 15.2 (the copy of an interface value or pointer out of a slice element or a range
variable is done by step 15.2, see §3.4); natives of the runtime (`sync.Mutex.Lock`) take owning pointers, so a pure
native allow-list (math, bits, utf8) would make more callees pure; struct and array parameters are still copies.

### 3.3 Notes on step 15.15 (the docs site)

Built: `tools/docsite`, a static site generator in pure Go with no dependency (go.mod and go.sum are
unchanged, so the flake's `vendorHash` stays). `make docs` writes `site/` (git-ignored), `make docs-check`
only checks. It renders every `docs/**/*.md` with its own small Markdown renderer (headings with GitHub's
anchors, nested lists, tables, fenced code, links, images, emphasis), takes the sidebar from
`docs/site/nav.txt`, adds the standard library reference (one page for each non-internal package of
`stdlib/`, made with `internal/doc`, the code of `gocpp doc -all`, with an anchor for every declaration), a
client-side search over titles, headings and library names (works from `file://`) and previous/next
links. A link that leaves `docs/` must lead to a repository file and is written as a link to it on the web
(`-repo`, `-branch`). A broken link in a page that is never rewritten (one ADR) is listed in
`docs/site/known-links.txt`.

New pages: `docs/index.md` (home), `docs/tour/` (thirteen lessons), `docs/guides/go-developers.md`,
`cpp-developers.md`, `interop-examples.md` (SQLite, raylib, C++ containers and mixed sources, quoted from
`examples/`) and `docs-site.md`. The interop guide and the cookbook are `docs/spec/interop.md` and
`interop-cookbook.md`, which already existed and have their own test (`TestDocSnippets`).

The docs are tests: in the tour and the three guides every `go` block carries a marker (`program: PHASE`,
`excerpt: FILE`, `illustration: reason`), a program is written to a folder, checked with `gocpp check` at
its phase, run, compared with its `output` block and, when it says `go`, with the real go command; a
`cpp` block marked `emitted` must be lines of `gocpp emit`; a `rejected` block must be lines that `gocpp
check` prints without the switch. A page never describes a feature as available that the compiler
rejects: lessons 7, 8, 11 and 12 and the Go-developer guide are marked experimental with the real switch
(`GOCPP_GATE_PHASE=08`, `09`, `10`; typed interop is `GOCPP_TYPED_INTEROP=1`). `TestGateNotesCurrent`
fails once `version.Phase` reaches 10, as a reminder to remove those notes (step 15.16).

Review round 1 added: a FAQ page; the facts that pages state about gocpp are checked against it (the
`gocpp` commands they name must exist, so a planned command such as `gocpp expand` is not described as
working; the standard packages they list as present are exactly those of `stdlib/` and the missing ones
really fail to import; "Phases 00 to NN are finished" is `version.Phase`; a lesson's switch is its
program's phase, and the program is rejected one phase lower; the platforms are the hosts of
`internal/toolchain`); the renderer makes what it cannot render (unclosed comment or fence, footnote,
undefined reference, table cells that GitHub would drop) an error of `docsite -check`, supports setext
headings, consecutive reference definitions, text after a comment and `<pre>` blocks, and keeps ids unique;
`-out` is refused under `docs/` or above it, and the site is swapped in as a whole.
**Publishing is not done**: there is no workflow or hosting setup that uploads `site/`; `make docs` builds it
and the manager decides where it goes (the plan's "published as a static site from `docs/`" is open).

### 3.4 Notes on step 15.2 (escape analysis, [ADR-114](../../../docs/adr/114-escape-analysis-keeps-non-escaping-objects-on-the-stack.md))

Built (`internal/lower/escape.go`, `chapure.go`, `elemref.go`, `localclosure.go`, runtime `gp::stack_ptr`, `gp::local_ptr`,
`gp::borrow_tmp`): address-taken locals, `&T{}` and `new(T)` whose pointers only reach borrowed parameters or are read and written
through can stay on the C++ stack (a direct `new(T)` argument still allocates); `v := xs[i]` and
`for _, v := range xs` are `const T&` when the region is pure by class hierarchy analysis and no thread stores
the elements. This excludes local-array stores too, including nested arrays and string slots. Func literals
that are only called where they are made are raw by-reference lambdas. Small variables that only they capture
(also loop variables) can be plain C++ variables. Formerly boxed objects and captures have a 2 KiB object limit
and share a 16 KiB storage budget, including conservative pointer/alignment overhead. These are not bounds on
the entire generated frame or arbitrary recursion. `-gp:m`, `-gp:m=2` and `-gp:noescape` give the decisions
like `go build -gcflags=-m` and the baseline.

Measured (`bench/programs/dispatch/escape.gp`, O2, 5 runs, median ms, a busy shared machine): Go 396 (base run) / 481
(current run), gocpp before 3234 (g++) and 3557 (clang++), gocpp after 574 and 592: from 8.2x and 9.0x slower than Go to
1.19x and 1.23x. By kernel (scratch program, g++ O2, 100M, 400M and 100M rounds): stack objects 1.4 s to 0.056 s (Go
0.05 s), local closures 3.5 s to 0.2 s (Go 0.21 s), interface elements 3.0 s to 1.4 s (Go 1.4 s). The `dispatch` programs
that do not use these (closures that escape, interface calls on fields) did not change beyond noise.

Tests: `TestEscapeAnalysis`, `TestElemRefs`, `TestElemRefsStrings`, `TestLocalClosures`, and
`TestLocalClosureStackBudget` cover positive and negative decisions. Earlier mutation rounds exercised the
analysis and local closures; the review's adversarial programs also killed the `inBox` mutation. The direct
`new(T)` diagnostic is now pinned at both `-gp:m` levels against the generated allocation. The run tests
`escape_elem_arrays`, `escape_stack_size`, and `escape_generic` cover array mutation, bounded stack storage,
and instance-local generic boxing. `tests/run/escape_*.gp` (g++, clang++, `-asan`, output as real Go's),
`tests/golden/escape.gp`, runtime
`test_ptr_stack` (also on the real Windows PC). A regression run of the 678 `tests/run` programs whose C++ differs
between `-gp:noescape` and the default found two order-of-evaluation regressions (a local that a local closure changes
was no longer copied in place as gc does; `boxRefCaptured` fixed it), then only failures that the base also has.

Not done (honest gaps): the thread analysis of ADR-086 marks nearly every program with `fmt` or an interface call as
threaded, so element references appear only for element types that are never stored (a CHA-based refinement of
`threaded` would widen it); loop variables, parameters, receivers and named results are not stack objects (a
parameter or result captured by a local closure stays boxed); `&x` inside a closure counts as escaping; natives
(`sync.Mutex`, `bytes.Buffer`) still escape; generic functions are not analysed; `go` and `defer` uses escape; a
deferred closure is not local; struct and array parameters are still copies.

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
