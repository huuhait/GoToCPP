# gocpp · TODO

**Legend:** `[ ]` todo · `[~]` doing · `[x]` done
**Last updated:** 2026-10-13

Keep this list short: only what is next. The full step lists live in each phase's `STATE.md`.
Every item is built through the manager → coder → reviewer loop in [WORKFLOW.md](WORKFLOW.md).

---

## Now: Phase 00 actions

- [ ] Write ADR-001 … ADR-021 in `docs/adr/` (summaries in Phase 00 README §2.1, your answers already in them)
- [ ] Write `docs/spec/language.md` (diff against the Go spec), `cpp-mapping.md`, `interop.md` (v0)
- [ ] Repo skeleton: `go.mod` (`github.com/huuhait/gocpp`), folders, `LICENSE` (BSD-3-Clause), `Makefile`, `.gitignore`, `README.md`
- [ ] `flake.nix` dev shell (Go 1.25, clang 21, gcc 15, gdb, lldb, ninja, pkg-config, SDL2, raylib, sqlite, zlib, MinGW-w64 cross compiler, wine64)
- [ ] Test harness runners (`run`, `errors`, `golden`, `diff`, `interop`) green with zero tests
- [ ] All 19 target programs in `examples/`, tagged with their phase (Go-compatible outputs generated with real `go run`)
- [ ] Commit the plan and the spikes (nothing is committed yet)

## Then: first Phase 01 tasks

- [ ] Fork `go/scanner` + `go/parser` (Go 1.25.7) into `internal/syntax` with `FORK.md`
- [ ] Parser patch 1: Go keywords allowed after `.` on the same line (+ tests)
- [ ] Parser patch 2: constant expressions in type-argument lists (+ tests)
- [ ] Parser patch 3: dotted type names such as `std.chrono.seconds` (+ tests)
- [ ] Turn the front-end spike (header packages, namespace rewrite + namespace probe, `FakeImportC`) into `internal/load` + `internal/check`
- [ ] `gocpp fmt` (Phase 01 step 1.16), so formatting works from the first week
- [ ] After M1: language server v0 (Phase 11 early track: diagnostics, hover, go-to-definition, completion, formatting; about 2 weeks, see STATE timeline)

## Any time (parallel, low risk)

- [ ] VS Code TextMate grammar for `.gp`, including `name!(`, `func name!(`, `#Name`, `#body()` and `macro` (Phase 11 step 11.10)
- [x] Header zoo libraries chosen: C/C++ standard headers + SDL2, raylib, sqlite3, zlib (Phase 04 step 4.18)
- [ ] Choose the LSP reference project: a real ~30k-line Go library to copy as `.gp` for the gopls side-by-side benchmark (Phase 11 step 11.11)

## Decisions (answered 2026-10-13)

- [x] **Q1 Memory:** reference counting (ARC), no garbage collector. Cycles are broken with `weak.Pointer` (Phase 03).
- [x] **Q2 `int`:** 64-bit, like Go. `C.int` is there for C APIs.
- [x] **Q3 C/C++ imports:** like Go packages. A header is a package named after its file (`"<raylib.h>"` → `raylib.InitWindow`, `"./physics.h"` → `physics.step`), and C++ namespaces are packages that need no import (`std.vector[int]`). `C` keeps only C's built-in types and helpers (`C.int`, `C.CString`). No inline preamble: a helper goes into a header next to the `.gp` file. (Phase 01 §2.3, Phase 04)
- [x] **Q4 Concurrency:** `go` starts a real OS thread. No goroutine scheduler, now or later. (Phase 08)
- [x] **Q5 Runtime checks:** on, like Go: a bad index, nil pointer or division by zero stops with Go's message and the `.gp` line; `-nochecks` turns them off for speed. (Phase 02 §2.3, Phase 03 §2.9)
- [x] **Q6 Module path:** `github.com/huuhait/gocpp`.
- [x] **Q7 License:** BSD-3-Clause for now. Look at the runtime license again before v1.0 (see backlog).
- [x] **Q8 Platforms:** Linux and Windows, Linux first; Windows test builds from Phase 06, the full port in Phase 14, both before v1.0. macOS is not a target. (ADR-021)
- [x] **Q9 GitHub:** not yet. CI is `make ci` on your machine for now.
- [x] **Q10 `!` (must):** only where you write it. On a function (`func loadConfig!(path string) Config`): every error inside it that you don't take yourself is returned; callers see `(Config, error)`. On one call (`cfg := loadConfig!(path)`) in any function. Without `!`, plain Go. (Phase 10 §2.1)
- [x] **Q11 `!` in `main`:** print `<program>: <error>` and exit 1 (for `func main!()` and for `F!()` in `main`).
- [x] **Q12 Macro style:** `macro Name(args) { ... }`, used as `#Name(...)` inside code or on top of a func, and `#Derive(...)` on a type.
- [x] **Q13 Macros on top of a func:** both styles. Start style (runs at the start; a returned `func()` runs at the end) and wrap style (`#body()`: the macro decides how the function's code runs, e.g. `#VM()`, `#Recover()`, `#InThread()`, `#Once()`). (Phase 10 §2.2)

## Open questions (no decision needed yet)

- [ ] Operator overloading or destructors for gocpp types? Go has neither; revisit after v0.1.
- [x] Is `C` available in every file? **Decided: yes**, for C's built-in types and helpers; the front-end adds `import "C"` only to files that use it (D2 in Phase 00).
- [x] Default optimization level for `gocpp build`: **`-O2 -g`** (Phase 06 §2.9); re-checked with the Phase 15 benchmarks.
- [x] Namespaces for generated C++: **one flat `gpkg_<escaped import path>` per package** (`gpkg_main`, `gpkg_fmt`), and C/C++ names fully qualified (`::time`) (ADR-010, ADR-019; changed by the audit from nested `gpkg::…`).
- [x] Default interop mode: **opaque until typed mode passes the whole Phase 04 suite, then typed** (Phase 12 §2.8).
- [ ] `!` for comma-ok results too (`v := m[k]!` → error when the key is missing)? Not in v1; revisit after Phase 10.
- [ ] `err := #body()` so wrap macros can look at the function's error (for a `#Retry(3)` macro)? Planned right after v1 (Phase 10 §2.2).

## Ideas / backlog (not scheduled)

- [ ] Web playground: the transpiler compiled to WebAssembly, showing `.gp` → C++ live
- [ ] `gocpp test -cover` via clang source-based coverage mapped through `#line`
- [ ] Fuzzing in `testing` (`testing.F`)
- [ ] `net/http` package wrapping a C++ HTTP library
- [ ] Optional tracing-GC mode for cycle-heavy programs
- [ ] WebAssembly target through emscripten
- [ ] Self-hosting: compile the gocpp compiler with gocpp
- [ ] Procedural macros: user code generators run at compile time (AST in, AST out), after v1.0
- [ ] `#!` shebang support so `.gp` files can run as scripts (`#!/usr/bin/env -S gocpp run`)
- [ ] MSVC ABI on Windows (`clang-cl`) for C++ libraries built with Visual Studio; Windows arm64
- [ ] Inline C++ preamble (cgo style) as a shortcut for tiny helpers
- [ ] Before v1.0: decide whether the runtime gets a license exception (like LLVM's), so programs built with gocpp owe no notice

## Done

- [x] 2026-10-13: Plan written in `plans/2026-10-13-gocpp/` (README, STATE, TODO, phases)
- [x] 2026-10-13: Added to the plan: `F!()` must-calls + macros (Phase 10), fast language server (Phase 11); renumbered later phases
- [x] 2026-10-13: Spike: `F!()` via hidden generic helpers on stock `go/types` (types inferred, bad uses rejected)
- [x] 2026-10-13: Spike: macros = functions + hidden call site + lazy args (generic `#Dbg` works; lazy arg evaluated 0 times on success)
- [x] 2026-10-13: Spike: `macro` declarations + on-top macros (`#Trace` enter/exit even on panic, `#Locked` race-clean with 100 concurrent calls)
- [x] 2026-10-13: Measured `go/types` re-check speed with cached deps (`net/http` 28.5k lines ≈ 29 ms)
- [x] 2026-10-13: Spike: stock `go/parser` + `go/types` (`FakeImportC`) type-check interop-heavy `.gp` with 0 false errors
- [x] 2026-10-13: Spike: clang probes for C++ types (`decltype`) and macro values (`gp_val<V>`)
- [x] 2026-10-13: Checked that stock `go/printer` prints gocpp syntax, plus Go's `fmt`/`println` formats and the stdlib porting blockers
- [x] 2026-10-13: Compiled and ran the plan's key C++ idioms on GCC 15 and Clang 21 (identical results)
- [x] 2026-10-13: Spike programs kept in `spikes/2026-10-13/` (with a re-run README)
- [x] 2026-10-13: Full audit of all plans by 5 independent reviewers; verified findings fixed (see STATE log)
- [x] 2026-10-13: Your decisions Q1–Q13 recorded and applied to every phase; new Phase 14 (Windows), hardening is now Phase 15
- [x] 2026-10-13: Spikes for the decisions: header packages + namespace probe (clang++ and g++), must functions (two type checks, 3 runs), wrap macros (`#VM`, `#Recover`, `#InThread`, `#Once`, race-clean)
