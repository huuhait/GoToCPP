# gocpp · TODO

**Legend:** `[ ]` todo · `[~]` doing · `[x]` done
**Last updated:** 2026-10-13

Keep this list short: only what is next. The full step lists live in each phase's `STATE.md`.
Every item is built through the manager → coder → reviewer loop in [WORKFLOW.md](WORKFLOW.md).

---

## Now: Phase 01 (front-end)

- [ ] Turn the front-end spike (header packages, namespace rewrite + namespace probe, `FakeImportC`) into `internal/load` + `internal/check`
- [x] After M1: language server v0 (Phase 11 early track: diagnostics, hover, go-to-definition, completion, formatting; about 2 weeks, see STATE timeline) — done 2026-10-14

## Small follow-ups (give them to the next job that touches the file)

- [ ] `docs/CONTRIBUTING.md` §2: list `tools/stubgen` (the stdlib stub generator) under `tools/` (from job 1.9b)
- [ ] Tests in `internal/interop` (TestProbeCommand) and `internal/load` (TestLoadModule, TestTestFiles, TestNoFiles) fail when TMPDIR contains `..`: clean the temp paths (`filepath.Clean`/`EvalSymlinks`) before comparing (from the 2.7 review); also `internal/load` TestTestOnlyExternal failed once in `make ci` with TMPDIR in the scratchpad (its `$ROOT` substitution missed) and passed on reruns (from job 2.4)
- [x] FakeImportC false errors (from job 1.11): dropped by `internal/check/falseerrors.go` when the operand is a C/C++ value (job 1.15; Phase 01 README §2.8 "As built"); covered by TestFalseErrors and the corpus in `internal/check/testdata/corpus`
- [ ] `docs/spec/cpp-mapping.md` §package initialization (line ~71): `run_main` gets `&gp_init_all` as soon as a package has variables or `init` functions (not only from Phase 06), and the `#line` example shows a directive only where the line is not previous + 1 (from job 2.1)
- [ ] Phase 01 README §2.1/§2.5 step 5 and ADR-002: `gp_as[T]` is now `gp_cgoh.As[T]` from the reserved path `gocpp/cgoh` (plus `Unused`/`Value`), and "has C" includes imports (from job 1.10b, committed dbbf080; STATE has the "as built" note)
- [ ] One rule for "is this C" in the gates (from the 1.11 review): `embedsOpaque` only sees this package's C type names, so a struct embedding `lib.T` (with `type T C.Foo` in lib) misses the Phase 12 promoted-member gate. Use `Package.Opaque.TypeExpr(obj) != nil` for this package and the opaque pass's other-package rule (give it to the next job that touches `internal/check/gatewalk.go`; 1.11 is in)
- [ ] `internal/load` tests fail on Windows (under Wine, 41 FAIL lines, also on 1301b21): expectations with `/` where the output has `\`, Wine's "File not found." message, TestFset assuming `/opt/gocpp/stdlib` is absolute. Also a design question: the default StdlibDir `$GOCPP/stdlib` is not an absolute path on Windows, while diag's short names expect absolute file names. Owner: Phase 14 (Windows) at the latest; earlier if a job touches the loader tests (from the 1.14 review)
- [ ] The gates reject a call through a parenthesized function name, `(g)(1)` and `(fmt.Println)(1)`, as "function values … (Phase 03)" at Phase 02: unparen the callee before the function-value check (from job 2.3); give it to the next job that touches `internal/check/gatewalk.go`
- [ ] Phase 02 README §2.2 file table: add `ops.go` (operators, conversions, `s[i]`, `len`, `panic`; from job 2.3)
- [ ] `docs/spec/cpp-mapping.md:80` maps `uintptr` to `uintptr_t`; the emitter writes `uint64_t` (job 2.1 decision): fix the doc (from the 2.3 review)
- [ ] The GOCPP_CXX/GOCPP_REQUIRE_CXX compiler-matrix test code exists four times (runtime/cxx_test.go, tests/harness_test.go, internal/interop/nsprobe_test.go, internal/check/nsprobe_test.go) with slightly different messages: one shared internal test helper (from the 1.10c review)
- [x] `gocpp fmt` (an upstream go/printer bug, also in gofmt with Go generics) drops the parentheses in `range (std.vector[int]{7, 8})`, and the result doesn't parse ("expected 1 expression"): keep the parentheses when the range expression is a composite literal (from job 1.15) — fixed 2026-10-14 (internal/format/parens.go; left: a type that still needs several lines or has a comment in it gives an error; nits: say "can't be written on one line" and "move the comment out of the type" in the two messages)
- [ ] gocpp's own error "x (a C/C++ value) is not an interface" for `x.(T)` and `switch x.(type)` on any C value (local or not, also named over C): go/types is silent or gives a garbage message there, and job 1.15 drops the garbage one (owner: Phase 04 or 05)
- [ ] internal/cppgen (from the 2.10b review; give to the next emitter job): `chain` in control.go closes the shadow blocks of an if/else-if init as separate `}` lines (join them like `closeShadows`); in `x, y := x+1, 5`, `y` enters the outer funcNames scope but is written inside the new C++ block (enter all of a declaration's new variables into the inner scope once a block is opened)
- [ ] `docs/adr/README.md`: describe dated amendments (like ADR-002's printer line), not only superseding (from job 1.5)
- [ ] A probe's compiler that exits NON-zero while its output was cut hides it: os/exec returns ErrWaitDelay only for a command that otherwise succeeded, so a failing exit gives the ExitError and a run whose output copy did not finish in time looks complete. It matters where a failing run is the intended one and its output is read: internal/interop/nsprobe.go runCompiler (the retry only sees success + ErrWaitDelay), the kind probe (runProbeOnce: include errors), and externc.go's parse-as-C check (missingPlaces/reportsMissing decide from `out` after a compile that fails by design: a cut output could misclassify). Fix: own the pipe (os.Pipe + a reader with an EOF flag) so the truncation is known for every exit status, then retry like the success case. (WaitDelay review, 2026-10-17)
- [ ] Load-sensitive tests still left (WaitDelay review, 2026-10-17): cmd/gocpp `TestLSPProcess` ("initialize answered after 330ms, want under 100ms"); internal/build `TestProgramParallel` fails on dev with `g++: internal compiler error: Segmentation fault ... cc1plus` when compiling with the PCH (another job is investigating; do not paper over it).

- [ ] Step 4.16 review: should a `.h` with a same-stem `.cpp` in the package be treated as C++ (not wrapped in `extern "C"`) automatically? Today the pair fails to link until the user writes `//gocpp:no-extern-c ./x.h` or guards in the header; the link-error hint says so (cmd/gocpp `externCClashHints`). A user-facing design decision.
- [ ] Step 4.16 review: a helper header whose default name is unusable gets no hint when used by that name: `main.h` next to `main.gp` (the name is Go's `func main`: bare "main.mn undefined (type func() has no field or method mn)") and `./3d.h` (not a Go name: "undefined: d3"). The docs tell the user to name the header (`import mh "./main.h"`). A hint needs check to relate a type error on `main.x` to an unnamed header with `UsedByPackage` (internal/load/imports.go `defaultNames`).

- [ ] For later Phase 08 steps (from the 8.1/8.15 review): 8.11 (deadlock detection) needs a count of blocked goroutines and a registry of the live goroutines: the goroutine record (`gp::detail::goroutine`, `runtime/include/gocpp/goroutine.hpp`) has no "blocked" flag and `gp::waiter` has no park hook yet, so add both there. 8.10 (`Goexit`) must catch Phase 05's `goexit_signal` in `goroutine_main` (`runtime/src/goroutine.cpp`) so the goroutine ends and counts down. 8.2 (`go` codegen): `gp::detail::go_start` takes a `std::unique_ptr<task>` and a stack size; `gp::go(f)` is the template wrapper. Step 8.2 also adds `goroutine.hpp`/`waiter.hpp` to `rt.hpp` and the new standard headers (`<condition_variable>`, `<mutex>`, `<chrono>`) to `internal/lower/cxx_macros.txt` (`go test ./internal/cppgen -run TestMacroList -update`).

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
- [x] **Q14 C/C++ libraries from the internet (2026-10-13):** modules like Go's; headers imported by full path (`"github.com/ocornut/imgui/imgui.h"`); `<...>` only for system headers; built from a small `cxx` recipe in `gocpp.mod`, no CMake. (Phase 06 §2.11)
- [x] **Q15 Toolchain like Go's (2026-10-15):** gocpp downloads its pinned llvm-mingw (UCRT, SHA-256 in the binary) when no Windows toolchain is installed, the way `GOTOOLCHAIN` fetches Go; `GOCPP_TOOLCHAIN=auto|local|pinned`. Windows needs only `gocpp.exe`. (Phase 06 §2.10, Phase 14 §2.2, Phase 15 §2.4)
- [x] **Q13 Macros on top of a func:** both styles. Start style (runs at the start; a returned `func()` runs at the end) and wrap style (`#body()`: the macro decides how the function's code runs, e.g. `#VM()`, `#Recover()`, `#InThread()`, `#Once()`). (Phase 10 §2.2)

- [x] **Q16 C/C++ libraries from the internet are built NOW (2026-10-17):** module downloads (tagged archive or commit, hash in `gocpp.sum`, read-only module cache, `gocpp mod vendor`), reading and building the `cxx` recipes, and module-header imports come before Phase 15 as jobs 6.1b (import rule), 6.1c (downloads) and 6.1d (recipes). Phase 06 §2.11 "download comes later" is superseded.
- [x] **Q17 Local headers import like Go packages (2026-10-17):** a quoted import whose first path element has a dot is a module header, and the module can be the project's OWN module: with `module github.com/huuhait/project`, `import "github.com/huuhait/project/native/x.h"` is `native/x.h` in the module root; required modules use the same rule (longest module path prefix wins); `"./x.h"` and `"native/x.h"` stay relative to the package folder. (This also settles the shape of the `.h` + same-stem `.cpp` question: the automatic-C++ guess is still not made; it stays a directive.)

## Open questions (no decision needed yet)

- [ ] testify (Phase 09 §2.7): a gocpp drop-in with testify's API first, or wait until the real module compiles? Fuzzing (`testing.F`) and `-cover` into the plan proper?

- [ ] Module headers: symlinks inside a DOWNLOADED (cached) module are followed by the loader: add EvalSymlinks + prefix check against the module dir for cached modules once 6.1c merges (6.1c rejects links at extraction, so low risk).

- [ ] Module headers: Windows reserved device names (`con.h`, `aux.h`, `nul.h`) are not rejected by the path check (modules.md, "Limits"); reject them with the Windows work (Phase 14).

- [ ] C/C++ modules (Phase 06 §2.11): a shared recipe index? an opt-in CMake fallback for big libraries? download them before Phase 15?

- [ ] Operator overloading or destructors for gocpp types? Go has neither; revisit after v0.1.
- [x] Is `C` available in every file? **Decided: yes**, for C's built-in types and helpers; the front-end adds `import "C"` only to files that use it (D2 in Phase 00).
- [x] Default optimization level for `gocpp build`: **`-O2 -g`** (Phase 06 §2.9); re-checked with the Phase 15 benchmarks.
- [x] Namespaces for generated C++: **one flat `gpkg_<escaped import path>` per package** (`gpkg_main`, `gpkg_fmt`), and C/C++ names fully qualified (`::time`) (ADR-010, ADR-019; changed by the audit from nested `gpkg::…`).
- [x] Default interop mode: **opaque until typed mode passes the whole Phase 04 suite, then typed** (Phase 12 §2.8).
- [ ] `!` for comma-ok results too (`v := m[k]!` → error when the key is missing)? Not in v1; revisit after Phase 10.
- [ ] `err := #body()` so wrap macros can look at the function's error (for a `#Retry(3)` macro)? Planned right after v1 (Phase 10 §2.2).

## Ideas / backlog (not scheduled)

- [ ] A downloaded toolchain for Linux targets too (a pinned clang + lld + libc++, or `zig c++`), so a Linux machine without `g++`/`clang++` also needs only gocpp (follows Q15)
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
- [x] 2026-10-13: Phase 01 step 1.16: `gocpp fmt` (gofmt's output, header import groups kept as written)
- [x] 2026-10-13: Phase 01 step 1.4: parser patch 3 (`std.chrono.seconds`, `std.vector[int].iterator`)
- [x] 2026-10-13: Phase 01 step 1.9: the package loader (`load.Load`) and the stdlib stubs
- [x] Completion nits from the 11.6 review — done in job 11.6b (2026-10-14); left nits: TestFixDangling builds cache.ParsedFile without Mode (later passes parse with mode 0), a stray blank line before AdditionalTextEdits in completion.go
- [ ] Re-measure the formatting and organize-imports benchmarks on a quiet machine (11.7's later rounds ran at load 5–48), in 11.11's harness
- [x] TestProbeCache (internal/interop/nsprobe_test.go:672-680) may be flaky under heavy load: the "vec.h just changed" / "is old now" steps assume a probe run ends within nscache.go's 2 s racyWindow; set the mtimes explicitly instead of relying on wall time (seen once in a 3.7b `make ci` at load 28; not reproduced in 8 runs) — fixed 2026-10-14 (job fix-flaky)
- [x] TestHoverOpaqueDisk (internal/lsp/server/opaque_test.go:153, "the second server read no summary from disk") failed once in a loaded `make ci` (job 3.8 review); passed 5 times alone. Likely the disk-cache write racing the second server: make the test wait for the first server's writes (Pipeline.Close) before starting the second (Phase 11) — fixed 2026-10-14 (job fix-flaky)
- [ ] Ask the user whether to file the gc bug upstream (draft: [upstream/gc-shift-count-issue.md](upstream/gc-shift-count-issue.md)); TestRandomOrder avoids it with the `gz` guard. Nit: the `gz` comment and the two diff test headers say "amd64" and "c >= 1<<63"; the rule is generic (arm64, 386 with uint64 counts), only signed x is hit, and c = 1<<63 gives the right answer by luck
- [ ] Load-sensitive tests left (from the fix-flaky review): runtime TestCXX's g++ -O2 compile of a unit test was killed by the command timeout (182 s) at machine load ~29 (cmdTimeout raised to 10 min on 2026-10-14); internal/check TestProbeCache uses the shared defaultCache (64 sets, 84 made per package run) and could in principle be evicted between its two Checks (very unlikely; a private cache needs a new interop API)
- [ ] Flaky under load (found 2026-10-17 by the object-cache coder, not caused by it): `TestCHeaderOffHotPath` in internal/lsp/cache fails in `make ci` at machine load 30-50 (parallel jobs on the 128-core server); it fails the same way on the commit before the object cache with `go test -count=200 -run TestCHeaderOffHotPath ./internal/lsp/cache`, and passes alone and in small package sets. Look at what it times (a hot-path budget?) and make it load-proof the way internal/interop's TestProbeCache was (job fix-flaky).
- [ ] More load-sensitive tests (2026-10-17, machine load 400-550 from parallel jobs): cmd/gocpp TestLSPProcess ('initialize answered after 330ms, want under 100ms'), internal/interop TestProbeCancelStopsTheCompiler (200 ms budget; a race in the test: the /proc scan can miss the compiler) and the kind probe's 1-minute timeout (`kind probe: g++ was stopped: context deadline exceeded` when a compile takes over a minute at load 500). The fix/waitdelay branch fixes the cancel test and TestProbeOutputLimitStops; TestLSPProcess and the probe timeout (scale with load or make it configurable for tests) are still open.
- [ ] Two more timing tests that fail at load 270-400 and pass alone (2026-10-17, dev 60f6be7 `make ci`): `TestScales` in internal/load/gocppmod/review_test.go (a time RATIO between 3k and 18k lines, 5 retries all hit the same load stall: make it deterministic: count lookups/comparisons or allocations instead of wall time, or compare best-of-N with larger sizes) and `CHECK(big < 10000)` in runtime/test/fmt_test.cpp (fixed 2026-10-17: best of three, ratio).
- [ ] `runtimeKey` (internal/build/runtime.go) keys the runtime's cached object files on the text of an `@file` response-file argument in the compiler's args or GOCPP_CXXFLAGS, not on its content, so editing the file leaves old runtime objects in use while the program compiles with the new flags (pre-existing; found by the object-cache review 2026-10-17, low severity). Fix: hash the content of any `@file` argument (build.readsDriverFile finds them) or skip the runtime object cache for those flags.
- [ ] Speed up `make ci`: the `runtime` package (about 600 s) is the critical path because `TestCXX` compiles about 12 programs one after another in each of 6 (compiler, mode) chains: compile them in parallel (128 cores) and share the precompiled header; split `cmd/gocpp`, whose tests are serial because of `t.Chdir`.
- [ ] The runtime package's tests take ~5 min alone at load 21 and passed go test's 10 min default inside a loaded `make ci` (after 3.15b's fmt diff cases); `make test` now passes -timeout 30m. Make runtime/cxx_test.go faster: compile the C++ tests in parallel subtests (t.Parallel per compiler and mode) and share one runtime object build per mode
- [ ] runtime TestCXX failed once in 3.8 s inside a loaded `make ci` (2026-10-14, after the filecache fix merged; the log was lost) and passed alone and in the next full run: if it happens again, keep the log (a killed compile or an ENOSPC under /tmp are the likely causes)
- [x] tests/run/defer_repanicked.gp (1,001 levels of deferred panics, from 3.13) makes real Go itself crash now and then in runtime.printpanics (2/40 runs at GOAMD64=v1): TestRunVsGo (opt-in, GOCPP_RUN_VS_GO=1) is flaky on it. Lower the depth to where Go is stable (e.g. 200) or skip it in TestRunVsGo with a note; possibly an upstream Go bug worth a look — fixed in job 3.18 (depth 200)
- [ ] Evaluation order, D22's inliner corner (from job 3.12): in `h(gs[j], mi[a[i]])`, gc checks i first when h isn't inlined and j first when it is; gocpp always gives the inlined order. Func-value calls are never inlined by gc, so they could get the exact non-inlined order as a small later step (Phase 03 follow-up or Phase 15)

- [x] docs/spec/language.md D25 wording (nit from the last 3.12 review): the slow-key list ("a bool, int8, int16, uint8 or uint16 key, a float, most arrays and structs") leaves out complex keys and maps whose value is over 128 bytes (every key is slow then), and only keys gc keeps in registers are copied (arrays of at most one element, structs of at most 4 fields: a `[3]int32` key prints the new value in Go and gocpp). Suggested: "a key that the map call takes by address (see orderKey), of a type that fits in registers". Give it to the next job that touches language.md
- [ ] Back-end fallback messages still name the finished Phase 03: left in internal/lower/ops.go:400 and :465, expr.go:358 and expr.go's node-kind table (~577-582) (fixups-1 did the rest, the Never gate test and the examplePhase comment; give these to job 4.4's next round)
- [x] A test run writes the real `~/.cache/gocpp/rt` (the runtime extraction of `gocpp build`) when XDG_CACHE_HOME isn't overridden (found in the 4.1 review; older than 4.1). Tests must never write the user's cache: find the tests (cmd/gocpp, tests/, internal/build) and give them a temporary cache folder, with a check like the LSP's — fixed in job 4.13 (the harness sets XDG_CACHE_HOME)
- [ ] `import "<sys/select.h>"` names the package `select`, a Go keyword, and a use is a parse error ("expected operand, found 'select'"): the Phase 01 import classifier should ask for a rename (`import sel "<sys/select.h>"`), as it does for other names that can't be package names (found in the 4.2 review)

- [x] `gocpp build -o /dev/null` fails ("open /dev/.null.tmp-…: permission denied"): internal/build MoveFile writes a temp file next to the target; `go build -o /dev/null` works. Write directly when the target isn't a regular file (found in job 4.13)
- [ ] C-only flags: a package's `//gocpp:cflags` go to C and C++ alike (gocpp's generated code is C++, so they must reach C++), so a C-only flag such as `-std=gnu11` has no place yet; decide on a `//gocpp:conlyflags` or `GOCPP_CONLYFLAGS` if users need one (found in job 4.13)
- [ ] Load-sensitive tests seen at load 40–55 (fixups-1 review, 2026-10-15): internal/cppgen TestOrderLarge has a 1-second limit (order_test.go:762; 1.05 s under load, 0.03 s alone); internal/lsp/server TestOrganizeImportsAction failed once in a loaded `make ci` (the log was lost; it passed 5 runs alone). Make the time limit relative or generous, and catch the lsp failure's text next time; also internal/interop TestProbeOutputLimitStops under -race at load ~47 ("the probe took 51 s", bound 25 s; 26–29 s alone with -race, close to the bound: found in the 4.14 review)

- [ ] Header problems no probe sees (4.14 review, also on dev before it): a header that ends mid-declaration without `;` (`int f(void)` at EOF) passes `gocpp check` (the scope-check line's leading `;` completes it) and then fails the build as an internal error; conflicts between headers imported by different files of one package (the kind probe runs per include set, the macro probe only preprocesses), e.g. one function declared in a C++ `.h` in a.gp and in a wrapped C `.h` in b.gp ("conflicting declaration ... with 'C' linkage"), are reported as gocpp bugs. Options: a probe of the whole unit's includes once per build, and/or 4.17's classifier counting errors inside headers at their include lines as the user's
- [ ] The interop error-test harness (tests/harness_test.go interopErrorsTest, othersLineRx) accepts the "(and N more errors in gocpp's code: …)" line silently, so a seed that also triggers a gocpp bug still passes: treat that line as unexpected unless the test opts in (4.17 review nit); also reflow the "Trimming" bullet in docs/spec/interop.md
- [ ] From the 4.4 review (older than 4.4 or small): (1) `for i, x = range arr` with `x` an existing C-typed variable and `arr [3]C.int` gives "internal error: expression has no type: x" from internal/check/opaque.go (Phase 01's opaque pass doesn't label that target; shows in `gocpp check` and the editor too): fix with a check test; (2) the "x + 1 (C/C++ value) is not constant" error lives in lowering (cguard.go constsOK), so `gocpp check` and the editor accept `const d = x + 1`: move it into check; (3) check's gate says "C macros in constant expressions are not supported yet (Phase 12)" for `const d = h2.counter + 1` where counter is a C variable, not a macro: reword; (4) a comment in order.go's nameOperands that a `*p` place doesn't name what p points into (a pointer into a dead temporary, as in plain C++)

- [ ] Comparing two C char arrays (`r.name == "bob"` where both sides are C, or a C array with a C string) is a C++ array-address comparison: silently always false on g++ (clang only warns). Add `-Werror=address` (g++) and `-Werror=string-compare -Werror=deprecated-array-compare` (clang) to the generated-code flags and measure their effect on other generated code (found in the 4.9 review); a gp::string compared with a C char array compares all N bytes (documented; Phase 12 makes it exact)
- [ ] Reading a C bit-field through a conversion fails: `int(f.a)` lowers to `gp::cgo::as<int64_t>(f.a)`, whose forwarding reference can't bind a bit-field ("cannot bind bit-field"); fmt_arg/print_arg may have the same problem. Take scalars by value in those helpers (found in job 4.6)

- [ ] Named gocpp containers with C elements: `type Chars [1]C.char; var a Chars; a[0] = 200` reaches the checker's "expression has no type: a[0]" error before lowering (confirmed on clean 99e75ad and the 4.9 candidate). Fix the opaque-checker handling with a check test; do not weaken constant representability checks to work around it.

- [ ] Phase 11 and step 6.1 (found in the 6.1 review): the language server loads each package alone (internal/lsp/cache/packages.go loadPattern: pattern and Dir are the package folder), so for it the "main module" is the package's own module. Two effects: a package of a replaced module that imports a module replaced only in the application's gocpp.mod is reported "module ... is required, but is not available" in the editor (the CLI is fine), and vendored packages are loaded by their `vendor/<module>` folders. The LSP must pass the main module folder to the loader (a `load.Config` field, not there yet), and then use it for every package; the loader already gives a `vendor/<module>/...` folder the import path of the vendored module when vendoring is consistent (`vendoredFolder` in internal/load/modules.go), but nobody has run the server against that. Listed in docs/spec/modules.md "Known limits: the language server"

- [ ] Interprocedural escape analysis for opaque locals (step 4.8 review): in a function with defers, the address of a C/C++ local of an inner block or loop that a Go function stores (`register(&a)` with `global = p`, `add(&r, &a)`) is not followed into the callee, so the local dies with its block and a deferred call can read it (documented in docs/spec/interop.md, "not tracked"; the same for C++ functions, which follow C++'s lifetime rules). A sound fix needs the callee's escape summary; the cheap rules were measured to hold too much (loops like `defer_hold_scopes.gp` bounded())
- [ ] Narrow the rename method family to gopls's use-tied unification (Phase 11, step 11.8 review): gocpp renames every method that has the name and shape of a method of an interface of the module (internal/lsp/server/refs.go `relatedMethods`), gopls only the methods that a use ties to the interface (a value passed, assigned, returned or asserted where the interface is needed). Renaming `Close` or `String` of an interface renames unrelated types of the module; rename now tells the user which declarations it added (family.go). Anonymous interface literals and dynamic assertions are not in the family in either.
- [ ] Phase 11 step 11.11 (benchmark harness on a quiet machine): measure references and rename on packages that were never opened (cold: 40-package module, 0.6 s references and 1.1 s rename at machine load 400-800, then 0.25-0.45 s warm) against the 300 ms budget, which has no row for them; the 11.8 numbers in docs/guides/editors.md were taken under heavy load.
- [ ] Tests leak busy-loop processes (found 2026-10-17): 150 orphaned `bash -c 'while :; do :; done'` processes, all started at the same second 18 h earlier, took about 115 cores for hours (load 400+, random timing failures). They come from the fake compilers of internal/interop (`nsproc_linux_test.go` "time limit" and "left behind" cases, `nsparse_test.go:785`) that leave a looping child on purpose: when the test binary is killed (go test -timeout, Ctrl-C, a crash) the child is never reaped. Fix: start such fake compilers with `Pdeathsig: SIGKILL` (or in a process group the test kills in a `t.Cleanup` AND from a watchdog), and make the loop exit by itself (`timeout 120 bash -c ...` / `sleep`-based loops with a bound). Also: have `make ci` print any leftover `while :; do :; done` processes at the end.

