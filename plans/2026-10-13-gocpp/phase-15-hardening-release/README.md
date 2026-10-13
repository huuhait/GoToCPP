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
  so it's one binary. A C++20 compiler must be installed separately; `gocpp env` (Phase 06)
  shows which one was found.
- Release binaries via GoReleaser (linux amd64/arm64, windows amd64) with checksums. The
  Windows zip contains `gocpp.exe` and a note on installing llvm-mingw (or MSYS2).
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
