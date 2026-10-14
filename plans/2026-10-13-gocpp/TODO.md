# gocpp · TODO

**Legend:** `[ ]` todo · `[~]` doing · `[x]` done
**Last updated:** 2026-10-13

Keep this list short: only what is next. The full step lists live in each phase's `STATE.md`.
Every item is built through the manager → coder → reviewer loop in [WORKFLOW.md](WORKFLOW.md).

---

## Now: Phase 01 (front-end)

- [~] Parser patch 3: dotted type names such as `std.chrono.seconds` (+ tests) (step 1.4, started)
- [ ] Turn the front-end spike (header packages, namespace rewrite + namespace probe, `FakeImportC`) into `internal/load` + `internal/check`
- [~] `gocpp fmt` (Phase 01 step 1.16), so formatting works from the first week
- [ ] After M1: language server v0 (Phase 11 early track: diagnostics, hover, go-to-definition, completion, formatting; about 2 weeks, see STATE timeline)

## Small follow-ups (give them to the next job that touches the file)

- [ ] `internal/syntax/FORK.md` §7: "Don't use a number twice" instead of "would overwrite that patch"; §8 step 3: say where `local.patch` is written (`internal/syntax/local.patch`) (nits from the 1.2 review; job 1.4 edits FORK.md)
- [ ] `internal/syntax/parser/gocpp_typeargs_test.go` TestTypeArgStartTokens: add one expression form per token that also starts a type (`chan int(nil)`, `map[int]int{}`, `struct{}{}`, `interface{}(nil)`, `func() {}`, `[1]int{}`, `*p`, `<-ch`), so dropping one from the list fails (nit from the 1.3 review; job 1.4 edits this area)
- [ ] `docs/adr/README.md`: describe dated amendments (like ADR-002's printer line), not only superseding (from job 1.5)
- [ ] `internal/syntax/FORK.md` §5: "printing it with `go/format`" changes when step 1.16 moves the round-trip check to `gocpp fmt`'s printer

## Any time (parallel, low risk)

- [x] VS Code TextMate grammar for `.gp`, including `name!(`, `func name!(`, `#Name`, `#body()` and `macro` (Phase 11 step 11.10, job 11.10a, done 2026-10-13)
- [x] Header zoo libraries chosen: C/C++ standard headers + SDL2, raylib, sqlite3, zlib (Phase 04 step 4.18)
- [ ] Choose the LSP reference project: a real ~30k-line Go library to copy as `.gp` for the gopls side-by-side benchmark (Phase 11 step 11.11)

## Decisions (answered 2026-10-13)

- [x] **Q1 Memory:** reference counting (ARC), no garbage collector. Cycles are broken with `weak.Pointer` (Phase 03).
- [x] **Q2 `int`:** 64-bit, like Go. `C.int` is there for C APIs.
- [x] **Q3 C/C++ imports:** like Go packages. A header is a package named after its file (`"<raylib.h>"` → `raylib.InitWindow`, `"./physics.h"` → `physics.step`), and C++ namespaces are packages that need no import (`std.vector[int]`). `C` keeps only C's built-in types and helpers (`C.int`, `C.CString`). No inline preamble: a helper goes into a header next to the `.gp` file. (Phase 01 §2.3, Phase 04)
- [x] **Q4 Concurrency:** `go` starts a real OS thread. No goroutine scheduler, now or later. (Phase 08)
- [x] **Q5 Runtime checks:** on, like Go: a bad index, nil pointer or division by zero stops with Go's message and the `.gp` line; `-nochecks` turns off the bounds and nil checks for speed (division and shift checks stay, because C++ has undefined behavior without them). (Phase 02 §2.3, Phase 03 §2.9)
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
- [ ] GitHub Actions running `make ci` once the repo is on GitHub (decision Q9: not yet)
- [ ] MSVC ABI on Windows (`clang-cl`) for C++ libraries built with Visual Studio; Windows arm64
- [ ] Inline C++ preamble (cgo style) as a shortcut for tiny helpers
- [ ] Before v1.0: decide whether the runtime gets a license exception (like LLVM's), so programs built with gocpp owe no notice

## Done

- [x] 2026-10-13: Plan written in `plans/2026-10-13-gocpp/` (README, STATE, TODO, phases) and committed with the spikes
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
- [x] 2026-10-13: Phase 00 steps 0.2–0.7, 0.9, 0.10: ADR-001 … ADR-021, the spec docs v0, the repo skeleton, the Nix dev shell, the 19 target programs, the contributing guide (one commit per step on branch `dev`)
- [x] 2026-10-13: Phase 00 steps 0.8 + 0.11: the test harness (six runners, phase gating, both compilers) and `make ci`
- [x] 2026-10-13: Phase 00 job 0.6b (stale docs, `xvfb-run`) and the Phase 00 exit check: **Phase 00 done**
- [x] 2026-10-13: Phase 01 step 1.13: `internal/diag` prints errors like `go build`
- [x] 2026-10-13: Phase 01 step 1.1: `go/scanner` + `go/parser` forked into `internal/syntax` (`FORK.md`, `update.sh`)
- [x] 2026-10-13: Phase 01 step 1.2: parser patch 1 (`std.map`, `node.type()`)
- [x] 2026-10-13: Phase 01 step 1.5: the round-trip test over the Go standard library (6240 files)
- [x] 2026-10-13: Phase 01 step 1.6: the import classifier (headers and namespaces as packages, the reserved `C`)
- [x] 2026-10-13: Phase 01 step 1.7: the `//gocpp:` directive parser
- [x] 2026-10-13: Phase 01 step 1.3: parser patch 2 (`std.array[int, 5]`)
- [x] 2026-10-13: Phase 11 job 11.10a: the VS Code language + TextMate grammar for `.gp`
