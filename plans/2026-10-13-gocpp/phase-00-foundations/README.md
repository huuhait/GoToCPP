# Phase 00: Foundations & Language Spec

> Lock the decisions everything else depends on, write them down, and build the empty
> skeleton (repo, dev shell, test harness, target programs) that every later phase plugs into.

| | |
|---|---|
| Depends on | nothing |
| Unlocks | Phase 01 |
| Size | small (about 1 week, time-boxed) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

A language project fails most often in two ways. One is decisions that keep moving ("is `int`
32 or 64 bits?", "GC or not?"). The other is having no way to prove that a feature works. Phase
00 fixes both before any compiler code is written:

1. **Decisions** go into short ADRs (Architecture Decision Records), one file per decision
   with context, choice, alternatives and consequences.
2. **The language** is written down as a *diff against the Go spec*: "gocpp is Go (1.25
   syntax) except for these points". That is much shorter than writing a full spec and keeps us
   honest about every divergence.
3. **The test harness** exists from day one, so every later phase adds tests and never
   discusses "does it work?".
4. **Target programs** (`examples/`) show what success looks like for each phase.

## 2. How it works

### 2.1 Decision records (ADRs)

Each ADR lives in `docs/adr/NNN-short-title.md`:

```markdown
# ADR-004: Memory model is automatic reference counting
Status: accepted (2026-10-xx)
Context: Go has a GC; C++ has none; users write `&T{}` everywhere...
Decision: `*T` → gp::ptr<T> (std::shared_ptr based) ...
Alternatives: Boehm GC (rejected: ...), manual memory (rejected: ...)
Consequences: cycles leak → provide weak.Pointer; atomic refcount cost; ...
```

The initial set (summaries; the full text is written in step 0.2):

| ADR | Decision | Main alternative rejected |
|---|---|---|
| 001 | Transpiler is written in **Go** | C++ (the language we want to avoid), Rust |
| 002 | Front-end = **fork of `go/scanner` + `go/parser`** (3 patches in Phase 01, 3 more in Phase 10) + a small **`go/printer` fork** (Phase 10) + stock `go/ast`, `go/types`, `go/constant`. `go/types` stays stock: x/tools (`gcexportdata`, `go/analysis`) only works with the stock package. Phase 12 solves its gaps without a fork; a fork would need its own ADR | Hand-written parser and checker (months of work, subtle Go semantics bugs) |
| 003 | Output is **readable C++20** (GCC ≥ 13, Clang ≥ 17) with `#line` directives | Unreadable SSA-style C++ (hard to debug), C (loses templates/RAII/interop) |
| 004 | Memory = **ARC**: `*T` → `gp::ptr<T>` (wraps `std::shared_ptr<T>`, aliasing constructor for interior pointers); `*pkg.T` for a C/C++ type (`*cstdio.FILE`) → raw `T*` | Boehm GC (non-deterministic, C++ interop trouble), manual memory (unsafe, not Go-like) |
| 005 | Interop = header imports (`"<hdr>"`, `"./x.h"`): **a header is a package named after its file** (`raylib.InitWindow`), **C++ namespaces are packages** (`std.vector`), `C` keeps only C's built-in types and helpers (`C.int`, `C.CString`); **opaque first**, typed via clang later | every C/C++ name under `C.` (cgo style), cgo-only preamble, mandatory bindings (can't cover all of C++) |
| 006 | Numbers: `int` = `int64_t`; signed overflow wraps (`-fwrapv`); results of `int8`/`int16`/`uint8`/`uint16` arithmetic are cast back to their type (C++ promotes them to `int`); bounds/nil/div-by-zero/shift checks panic like Go; `-ffp-contract=off` | C++ `int` (surprises Go users), UB on overflow |
| 007 | Goroutines = **OS threads**: `go f()` starts a real thread, and there is no goroutine scheduler, now or later | Fibers / an M:N scheduler (complex, not needed), no `go` at all (not Go-like) |
| 008 | Build = **own driver** calling `$CXX` with a content-hash cache; exports `compile_commands.json` + CMake | Generate CMake and require it (slow, extra dependency) |
| 009 | Runtime = header-mostly C++20 library in `namespace gp`, **embedded in the gocpp binary** (`go:embed`) | Separate install step (version-skew bugs) |
| 010 | Naming: package → one **flat** namespace `gpkg_<escaped import path>` (`gpkg_main`, `gpkg_github_dcom__huuhait__game__engine`); C/C++ names are emitted fully qualified (`::time`, `::std::vector`) except macros; escape C++ keywords and colliding macros with a `_` suffix; also rename a method named like its type (`func (U) U()`, which C++ would read as a constructor) and user names starting with `gp_` (reserved for generated code) | Global names (collisions with C libraries); nested `gpkg::a::b` namespaces (they hide C names such as `time()` and clash with sub-packages) |
| 011 | `error` = Go interface; `panic` = C++ exception `gp::panic_error`; `defer` = per-function frame | `setjmp/longjmp` (skips C++ destructors) |
| 012 | Supported subset = Go 1.25 syntax + a **small, explicit** set of extensions (divergence list below) | Inventing a lot of new syntax |
| 013 | Error propagation = **`!` (must)**, only where it is written: on a function (`func f!() T`: every error inside it that the code drops is returned; callers see `(T, error)`) and on one call (`F!(...)`). Rewritten before `go/types` (must functions need a second check), lowered to early returns; in `main` the error is printed and the program exits 1 | A `try` builtin, a `?` suffix, implicit error handling without a marker |
| 014 | Macros = **`macro Name(args) { ... }` declarations** (type-checked like funcs, with call-site info and `lazy` params), used as `#Name(...)` inside code or on top of a func, in **start style** (runs at the start; a returned `func()` runs at the end) or **wrap style** (`#body()` runs the function's code where the macro wants); `#Derive(...)` on types | C-style text macros (unsafe, invisible to tools), Rust-style `macro_rules!` (custom syntax breaks gofmt and the editor) |
| 015 | Language server = **gopls architecture with written speed budgets** enforced in CI | Separate "light" editor parser; features first and speed later |
| 016 | Interfaces = **Go-style type descriptors + itabs** (matched by method name and signature, cached); descriptors are chosen by the **Go** type, so `int` and `int64` stay different although both are `int64_t`; embedding without C++ inheritance | C++ virtual functions (need explicit "implements"; inheritance would make embedding a subtype) |
| 017 | Generics = **C++ templates, always with explicit type arguments** taken from `go/types` | C++ template argument deduction (different rules from Go); our own monomorphizer |
| 018 | Standard library = **ported from Go's own source** (tool + patch files) + a **`reflect` subset** on type descriptors; system parts native or via interop | Writing a stdlib from scratch; wrapping the C++ standard library (different semantics) |
| 019 | Output layout = **one `pkg.hpp` per package, one `.cpp` per `.gp` file, `pkg_init.cpp`** with `gp_var_init()`/`gp_init_funcs()`; the driver emits `gp_init_all()` in Go 1.21's global init order; namespace `gpkg_<escaped import path>`, package `main` → `gpkg_main` | One giant translation unit (slow incremental builds); header-only output |
| 020 | Modules = **`gocpp.mod` in `go.mod` syntax**; `replace`/`vendor` first, downloads with minimal version selection later (Phase 15) | A new module format; requiring CMake or Conan |
| 021 | Platforms = **Linux and Windows** (x86-64 first). Linux first; Windows test builds from Phase 06, the full port in Phase 14, both before v1.0. Windows uses **MinGW-w64** (clang or gcc) first, the MSVC ABI (`clang-cl`) later. The runtime reaches the OS through one layer | macOS (not a target), MSVC's own compiler (no `-fwrapv`) |

### 2.2 The language as a diff against Go

`docs/spec/language.md` starts with: *"gocpp follows The Go Programming Language
Specification (go1.25) except for the points below."* The initial divergence list:

| # | Go | gocpp |
|---|---|---|
| D1 | Garbage collector | Reference counting. Cycles leak unless broken with `weak.Pointer[T]` (Go 1.24 API) |
| D2 | `import "C"` + cgo preamble | Header imports `"<hdr>"`/`"./x.h"`: each header is a package named after its file (`raylib.InitWindow`), and C++ namespaces are packages without an import (`std.vector`). `C` holds only C's built-in types and the boundary helpers (`C.int`, `C.CString`, `C.cstr`) and can be used in every `.gp` file. No inline preamble: a small helper goes into a header next to the `.gp` file |
| D3 | Go keywords cannot follow `.` | Allowed after `.` so C++ names work: `std.map`, `node.type()`, `posix.select()` |
| D4 | Type arguments must be types | Constant expressions are allowed for C++ templates: `std.array[int, 5]` |
| D5 | Conversion `T(x)` has 1 argument | For C/C++ types `T(a, b, ...)` calls the constructor |
| D6 | Full `reflect`, `plugin`, `runtime` internals | `reflect` is a **subset** built on compiler-generated type descriptors (enough for `fmt`, `encoding/json`, `sort`, `errors`, `context`; see Phase 09). No `MakeFunc`, `StructOf` or calling methods by name. No `plugin` |
| D7 | Goroutines are cheap (millions) | Each goroutine is an OS thread: tens of thousands are fine, millions are not (decided: no scheduler) |
| D8 | Map iteration order randomized | Unspecified but not randomized |
| D9 | `int` is 32 or 64 bits by platform | 64-bit targets only in v1 |
| D10 | Data races: undefined | Same, but a race on a pointer/slice/string can corrupt refcounts; use `-race` (TSan) |
| D11 | `#cgo CFLAGS/LDFLAGS` lines | `//gocpp:cflags`, `//gocpp:ldflags`, `//gocpp:link`, `//gocpp:pkg-config` directives + `gocpp.mod` |
| D12 | Errors handled with `if err != nil` | Also **`!` (must)**: `func f!() T` returns every error dropped inside it (callers see `(T, error)`), and `F!(...)` returns one call's error; where there is no error result it panics, and in `main` it prints the error and exits 1 (Phase 10) |
| D13 | No macros | **`macro Name(args) { ... }`** declarations, used as `#Name(...)` inside code or on top of a func (start style or wrap style); built-ins `#file()`, `#line()`, `#func()`, `#text()`, `#body()`, `#embed()`, `#Derive(...)` (Phase 10) |
| D14 | Every value follows Go's rules | Values of **C/C++ types** follow C++ rules: `m["k"]` on a `std::map` inserts, C++ containers have no bounds checks, raw C pointers have C++ lifetimes (Phase 04) |
| D15 | `unsafe.Sizeof`: `string` 16, `[]T` 24, `*T` 8 bytes (64-bit) | gocpp v1 sizes: `string` 32, `[]T` 40, `*T` 16 bytes (Phase 03; may shrink in Phase 15) |
| D16 | `unsafe.Pointer` keeps its target alive (the GC sees it) | Same for gocpp objects (`unsafe.Pointer` is an owning `gp::ptr<void>`), but a pointer that comes **from C** owns nothing, and C code never keeps gocpp memory alive: a gocpp pointer passed to C is only borrowed (Phase 04) |
| D17 | Standard packages are exactly Go's | A few **gocpp additions**: `errors.Wrapf(&err, ...)`, `errors.Trace(err)`, `C.cstr(...)`, `C.wstr(...)` (Windows), the hidden `gocpp/macro` package (Phase 10) |
| D18 | `macro` and `lazy` are ordinary identifiers | `macro` starts a declaration **at package level** and `lazy` is a modifier **inside macro parameter lists**. Everywhere else they stay ordinary identifiers (Phase 10) |
| D19 | `C` is an ordinary identifier | `C` is reserved for C's built-in types and the boundary helpers (`C.int`, `C.CString`): a local or package-level declaration named `C` is an error (Phase 01 §2.3) |
| D20 | A type name has at most one dot (`pkg.T`) | Type names may have more dots for C++ types: `std.chrono.seconds`, `std.vector[int].iterator` (Phase 01 patch 3) |
| D21 | `range` works on arrays, slices, strings, maps, channels, integers and functions | Also on C++ containers and C arrays (anything with `begin()`/`end()`): the value (and a map's key) gets C++'s element type, and a random-access container is walked by index with its size read once, as Go reads the length once (Phase 04 §2.5) |

### 2.3 Test harness design

One Go test binary (`go test ./tests/...`) discovers test programs by folder:

| Folder | What a test is | How it passes |
|---|---|---|
| `tests/run/` | A `.gp` program with a trailing `// Output:` comment block (like Go examples); optional `// ExitCode: 2`, `// Stderr: regexp` | `gocpp run` output equals the expected output |
| `tests/errors/` | A `.gp` file with `// ERROR "regexp"` on lines that must fail (the same format as Go's own `test/` dir) | `gocpp check` reports exactly those errors on those lines |
| `tests/golden/` | A `.gp` file + `.cpp.golden` snapshot of generated C++ | Generated C++ matches; `-update` rewrites snapshots (keep this set small) |
| `tests/diff/` | A `.gp` program using only the Go-compatible subset | **Differential test:** copy it to `.go`, run with real Go, run with `gocpp run`, compare stdout and the exit code. A `// diff: stderr` header line also compares stderr, and `// diff: combined` compares both streams merged into one (for output-order tests). The real Go toolchain is our reference implementation |
| `tests/interop/` | Programs that use C/C++ headers/libs | Like `run/`; skipped with a message if a library is missing (`// pkg-config: sdl2` line, checked with `pkg-config --exists`) |
| `examples/` | One folder per target program (§2.4) with `main.gp` | Runs `gocpp run . <args>` **inside the example's folder** (so relative paths like `testdata/poem.txt` work) and compares stdout with `output.txt`; optional `args.txt`, `stdin.txt` and `env.txt` (`KEY=VALUE` lines, e.g. `GOCPP_SMOKE=1` for GUI smoke runs); header lines `// pkg-config: …`, `// display: required` (skipped without a display) and `// ExitCode: N`; `//go:build` lines are honored (the `win32` example runs only for Windows targets, under Wine from Phase 14) |

Every test file starts with a `// phase: NN` line: a test whose phase is later than the last
finished phase is skipped with "needs Phase NN" (a coder working on phase NN sets
`GOCPP_TEST_PHASE=NN` to run the tests up to that phase). Every run test executes with **both** `g++` and
`clang++` (`GOCPP_CXX=...`). Outside CI a missing compiler is a skip; `make ci` sets
`GOCPP_REQUIRE_CXX=g++,clang++`, so there a missing compiler fails. `GOCPP_FLAGS` passes extra
flags to `gocpp`, e.g. `-asan` (AddressSanitizer + UBSan, Phase 02 §2.7) for the sanitizer run that CI adds.

Example run test:

```go
// phase: 03

// tests/run/slices_append.gp
package main

import "fmt"

func main() {
	s := []int{1, 2}
	t := append(s, 3)
	s[0] = 9
	fmt.Println(s, t, len(t), cap(t) >= 3)
}

// Output:
// [9 2] [1 2 3] 3 true
```

The expected output comes from real `go run`, not from memory: `s` was full (capacity 2), so
`append` copied it and `t` kept the old values. It's easy to guess this wrong, so for
Go-compatible programs the expected outputs are always generated with real Go.

### 2.4 Target programs (acceptance examples)

`examples/` holds small, real programs. Each one names the phase that must make it pass.
They act as the "definition of done" for the whole project:

| Example | Shows | Must pass at end of |
|---|---|---|
| `hello` | print | Phase 02 |
| `fizzbuzz`, `primes` | loops, if, switch | Phase 02 |
| `structs` | structs, value + pointer methods | Phase 03 |
| `wordfreq-core` | slices, maps, strings, range, sort by hand | Phase 03 |
| `closures` | counters, func values, defer | Phase 03 |
| `cpp-std` | `std.vector`, `std.map`, `std.string`, `std::filesystem` | Phase 04 |
| `sqlite` | sqlite3 C API, `C.CString`, callbacks | Phase 04 |
| `raylib` or `sdl` | window + game loop with a C library | Phase 04 |
| `mixed-sources` | package with `.gp` + `.c` + `.cpp` + `.h` | Phase 04 |
| `shapes` | interfaces, type switch, `error`, embedding | Phase 05 |
| `multi-pkg` | app + 3 packages + C sources | Phase 06 |
| `generics` | `Map/Filter/Reduce`, `Stack[T]`, `iter.Seq` | Phase 07 |
| `workers` | goroutines, channels, `select`, `WaitGroup` | Phase 08 |
| `wc`, `grep-lite` | `os`, `bufio`, `strings`, `flag`, `regexp` (grep-lite), tests | Phase 09 |
| `must-errors` | `func loadConfig!()` must functions, `F!(...)` calls, `func main!()` exit 1, `defer f.Close()` | Phase 10 |
| `macros` | `macro` declarations; `#Assert`/`#Dbg` inside code; `#Trace`/`#Locked` (start style) and `#Recover`/`#InThread` (wrap style) on top of funcs; `#Derive(String)` on an enum and `#Derive(JSON)` on a struct | Phase 10 |
| `win32` | a Windows program: `MessageBoxW` and a small window through the `<windows.h>` header package | Phase 14 |

### 2.5 Dev environment

You are on NixOS, so a `flake.nix` dev shell makes the toolchain reproducible:
`go_1_25`, `clang_21`, `gcc15`, `gdb`, `lldb`, `cmake`, `ninja`, `pkg-config`, `python3`
(gdb pretty printers), plus `SDL2`, `raylib`, `sqlite` and `zlib` for the interop examples and
the Phase 04 header zoo, and for Windows builds (Phase 06 on) the MinGW-w64 cross compiler and
64-bit Wine (`wine`) to run the tests, and `xvfb-run`, so GUI examples can run their smoke mode
without a real display. nixpkgs is pinned to `nixos-26.05`, because Go 1.25 is end-of-life
upstream and was removed from nixos-unstable (found while building the dev shell). (Ninja is not
installed on the machine itself; the dev shell adds it.)

Note found in the spike: the NixOS clang wrapper prints `-Wunused-command-line-argument`
warnings in `-fsyntax-only` mode. The build driver should pass `-Qunused-arguments` for clang.

## 3. Steps

1. **0.1 Review this plan** and answer the open questions in [../TODO.md](../TODO.md).
   Done on 2026-10-13: every answer is already written into the ADRs and phases.
2. **0.2 Write ADR-001 … ADR-021** in `docs/adr/` from section 2.1 (your answers are already in it).
3. **0.3 Write `docs/spec/language.md`** as a diff against the Go spec (section 2.2).
4. **0.4 Write `docs/spec/cpp-mapping.md` v0** from the type table in the root README. Each
   later phase extends it.
5. **0.5 Write `docs/spec/interop.md` v0**: header packages and namespaces, the `C` built-in
   types and helpers, pointer and string rules (Phase 01 and Phase 04 READMEs have the content).
6. **0.6 Create the repo skeleton**: `go.mod` (`module github.com/huuhait/gocpp`, confirmed),
   the directories from the root README, `LICENSE` (BSD-3-Clause for now, decided; the runtime
   license is looked at again before v1.0), `.gitignore`, `Makefile` (`make build test lint ci`),
   `README.md`.
7. **0.7 Add `flake.nix`** (dev shell) and `.envrc` (`use flake`) for direnv users.
8. **0.8 Build the test harness** (`tests/harness_test.go`) with the six runners from 2.3 (the
   five test folders plus `examples/`). With no compiler yet, the runners find the example
   tests, skip them by phase, and pass; they fail clearly when a test is malformed.
9. **0.9 Write the target programs** from 2.4 with their expected outputs. Each starts
   with a `// phase: NN` comment so the harness can skip programs whose phase isn't reached.
   Expected outputs of Go-compatible programs are produced by real `go run`, never by hand.
10. **0.10 Conventions doc** (`docs/CONTRIBUTING.md`): the manager → coder → reviewer loop
    from [../WORKFLOW.md](../WORKFLOW.md), Go style (gofmt, `go vet`, staticcheck), C++ runtime
    style (`.clang-format`), test-first rule, commit messages.
11. **0.11 CI**: `make ci` runs locally with a GCC + Clang matrix (the repo is not on GitHub yet,
    decision Q9). A GitHub Actions workflow comes when the repo is pushed.
12. **0.12 Keep the spikes**: the experiments behind this plan live in `spikes/2026-10-13/`
    with a README on how to re-run them (done on 2026-10-13).

## 4. Deliverables

- `docs/adr/001…021-*.md`, `docs/spec/{language,cpp-mapping,interop}.md`
- Repo skeleton that builds (`go build ./...`) with a passing (empty) test suite
- `flake.nix` dev shell
- `examples/*` target programs with expected outputs
- `docs/CONTRIBUTING.md`, `Makefile`, optional CI workflow

## 5. Exit criteria

- Open questions in TODO.md answered; ADRs marked `accepted`.
- `go build ./... && go test ./...` passes on a clean checkout.
- `nix develop` gives a shell where `go`, `clang++`, `g++`, `gdb`, `ninja`, `pkg-config` work.
- All 19 target programs from 2.4 committed, each tagged with its phase.
- Spec docs v0 committed.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Over-designing before writing code | Time-box to about 1 week. Specs are living documents that later phases update |
| A decision turns out wrong later | ADRs make the change explicit: write ADR-0xx "supersedes ADR-004" and move on |
| Test harness too clever | Keep the formats identical to Go's (`// Output:`, `// ERROR`); no custom DSL |
