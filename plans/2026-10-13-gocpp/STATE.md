# gocpp plan · STATE

**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Current focus:** 12.13a opt-in background header/namespace completion is integrated after clean revision-5 review and focused manager checks. The bounded 12.13b1 receiver-member primitive is integrated after clean review and manager checks, with explicit partial results for unprovable candidate groups. The bounded 12.13b2 background member integration is also merged after clean review and manager checks. Next is 12.13c1 proven foreign-variable type hover; broader header navigation stays separate. Coordinated parallel jobs cover release-platform workflows, derive acceptance, matched LSP measurements and focused Windows test portability, with at most five active workers total. Default opaque mode and full latency/platform acceptance remain unchanged. The bounded 15.4 ownership experiment is integrated after clean review and manager checks; retain the isolated prototype but defer production adoption. The bounded 15.7c header-unit evaluation is integrated after clean review and manager checks: warm reuse improves, but construction plus one replay is slower, so production keeps its existing PCH. Step 15.6b documents and regression-tests existing range-bound help after clean review and integrated checks; no additional compiler/runtime optimization was needed. Step 15.7b generic body placement is integrated as a structural improvement; whole-build speed benefit remains inconclusive. Step 15.6a concrete-interface optimization is integrated after clean review and focused source/integration checks; bounds-check work remains separate. The 11.14b current-implementation LSP measurements are reviewed: eight consistent wins and four inconclusive small typing rows; no qualifying slowdown or source change. The guarded-check portion of 11.14, safe PCH portion of 12.14, 15.2 escape analysis, 15.7a compiler-cache documentation/tests and 9.13a release preparation are integrated after clean reviews and focused manager checks. Every-row gopls and cold-probing performance targets remain open. Step 9.12 documentation is integrated; full 9.13/M6, including tier-1/JSON, final platform/archive validation and explicit release approval, remain open. Phases 00-05 and 07 are done. Nearly done: 06 (manual clangd acceptance) and 08 (TSan exit criterion). Other open work: 9.13; 10 remaining edge-case/overhead/derive acceptance and optional error traces; 11 final performance vs gopls and editor acceptance; 12.13; 13 vet acceptance; 14 stdlib/platform/editor acceptance; 15.7, 15.12 and 15.16; production ownership compatibility and global performance acceptance. Steps 9.10, 12.12 and 14.6 are merged. No full verification is claimed; broad final checks remain the user's manual work. See [HANDOFF.md](HANDOFF.md).
**Last updated:** 2026-10-22
**Migration:** done; the project runs on the larger server (128 cores, about 200 GB RAM). [HANDOFF.md](HANDOFF.md) says what is where.

---

## Planning

- [x] Plan written: overview ([README.md](README.md)), 16 phases (00–15), [TODO.md](TODO.md)
- [x] Added on request (2026-10-13): `!` (must) + macros (Phase 10), fast language server (Phase 11)
- [x] Riskiest assumptions checked with spikes (details in README §7 and the phase notes)
  - [x] Front-end: stock `go/parser` + `go/types` (`FakeImportC`, after the import rewrite) type-check interop-heavy `.gp` files with 0 false errors
  - [x] Syntax gaps found: `std.map`, `std.array[int, 5]`, `std.chrono.seconds` as a type → the 3 Phase 01 parser patches
  - [x] clang probes: expression types (`decltype`) and macro values (`gp_val<BUFSIZ>` → `8192`)
  - [x] Stock `go/printer` prints the gocpp AST, including `F!(...)` and `#Name(...)` (no printer fork needed until Phase 10's `macro`/`lazy`/on-top syntax)
  - [x] Go output formats checked (`fmt` floats, builtin `println`, float → int conversion)
  - [x] Stdlib porting blockers listed with `go list`
  - [x] Key C++ idioms (ARC aliasing, single-exit defer/recover shape, `requires` dispatch, designated init, tuples) compiled and run on GCC 15 + Clang 21
  - [x] `F!()` via hidden generic helpers works on stock `go/types`; macros as functions with call site + lazy args work
  - [x] `go/types` re-check speed measured (`net/http`, 28.5k lines: ~29 ms with cached deps)
  - [x] All Go code examples in the plan are gofmt-clean
- [x] Plan audited by 5 independent reviewers (122 findings: 34 high, 65 medium, 23 low); findings checked, then fixed in the phase docs (2026-10-13)
- [x] Your decisions Q1–Q13 answered and applied to every phase (2026-10-13), with new spikes for the three new designs:
  - [x] header packages + C++ namespaces as packages, with a namespace probe (clang++ and g++; typos stay early errors)
  - [x] must functions `func f!() T` (two type checks; the program ran right in 3 cases)
  - [x] wrap-style macros with `#body()` (`#VM`, `#Recover`, `#InThread`, `#Once`; race-clean)
- [x] Phase 16 planned in full (2026-10-21): your four answers applied (two engines in one program,
  bdwgc first behind `gp::gc`, M10 libraries then M11 gin, start when you decide), backed by a
  survey of 17 Go modules and a prior-art study; not scheduled
  - [x] Reviewed by five reviewers and corrected (2026-10-22): 22 steps, 52–77 weeks; your two
    answers applied (Windows stays in M10; standard imports use gp's library first, and the
    engine move that a later port causes is documented)

## Phases

- [x] **Phase 00:** Foundations & spec → [STATE](phase-00-foundations/STATE.md)
- [x] **Phase 01:** Front-end (parse, load, type-check, `gocpp fmt`) → [STATE](phase-01-frontend/STATE.md)
- [x] **Phase 02:** Walking skeleton → [STATE](phase-02-walking-skeleton/STATE.md) · ends with **M1**
- [x] **Phase 03:** Core language → [STATE](phase-03-core-language/STATE.md) · ends with **M2**
- [x] **Phase 04:** C/C++ interop (opaque) → [STATE](phase-04-cpp-interop/STATE.md) · ends with **M3**
- [x] **Phase 05:** Interfaces, embedding, errors, panic/recover → [STATE](phase-05-interfaces-errors/STATE.md)
- [x] **Phase 06:** Packages, modules & build system → [STATE](phase-06-packages-build/STATE.md) · ends with **M4**
- [x] **Phase 07:** Generics → [STATE](phase-07-generics/STATE.md)
- [x] **Phase 08:** Concurrency → [STATE](phase-08-concurrency/STATE.md) · ends with **M5**
- [~] **Phase 09:** Stdlib & `gocpp test` → [STATE](phase-09-stdlib-testing/STATE.md) · ends with **M6 (v0.1)**
- [~] **Phase 10:** Beyond Go: `!` (must) & macros → [STATE](phase-10-must-and-macros/STATE.md)
- [~] **Phase 11:** Fast language server (gopls speed) → [STATE](phase-11-language-server/STATE.md) · v0 starts after Phase 01 · ends with **M7**
- [~] **Phase 12:** Typed C++ interop via clang → [STATE](phase-12-typed-interop/STATE.md)
- [~] **Phase 13:** Developer tooling (diagnostics, debugging, vet, doc) → [STATE](phase-13-tooling/STATE.md)
- [~] **Phase 14:** Windows → [STATE](phase-14-windows/STATE.md) · ends with **M8**
- [~] **Phase 15:** Optimization, hardening & release → [STATE](phase-15-hardening-release/STATE.md) · ends with **M9 (v1.0)**
- [ ] **Phase 16:** Dual engine: Go packages on a garbage collector (planned, starts when you decide) → [STATE](phase-16-dual-engine/STATE.md) · ends with **M10** and **M11**

## Milestones

| | Milestone | Proof | Rough time from start* | Likely range |
|---|---|---|---|---|
| [x] | **M1** `hello.gp` runs | `gocpp run examples/hello` on GCC + Clang | ~6.5 weeks | 6–8 |
| [x] | **L0** editor v0 (early track) | diagnostics, hover, go-to-definition, completion, `gocpp fmt` in VS Code | ~8.5 weeks (2 weeks after M1) | 8–10 |
| [x] | **M2** real non-interop programs | `structs`, `wordfreq-core`, `closures` + 150 run tests | ~15 weeks | 14–18 |
| [x] | **M3** any C/C++ header usable | `sqlite`, `raylib`/`sdl`, `cpp-std`, `mixed-sources` | ~20 weeks | 19–24 |
| [x] | **M4** real multi-package projects | `multi-pkg`, no-op rebuild < 200 ms; first `hello.exe` (checked on the real Windows PC, not Wine, by order of 2026-10-20) | ~29.5 weeks | 28–36 |
| [x] | **M5** Go-style concurrency | `workers`, TSan-clean suite (`TestRace`, 247 PASS with both compilers, audit 2026-10-22) | ~37.5 weeks | 35–46 |
| [ ] | **M6** v0.1 usable (Linux) | tier-1 stdlib + `flag`/`regexp`, `gocpp test`, `wc`, `grep-lite` | ~45.5 weeks | 42–56 |
| [ ] | **M7** Go-plus features + fast editor | `must-errors`, `macros`; all LSP speed budgets met | ~57 weeks | 52–71 |
| [ ] | **M8** Windows works | Phase 14 exit criteria (`win32` example, suites under Wine and on real Windows) | ~71.5 weeks | 64–91 |
| [ ] | **M9** v1.0 (Linux + Windows) | Phase 15 exit criteria | ~81.5 weeks | 73–104 |
| [ ] | **M10** pure-Go libraries pass their own tests | yaml.v3, BurntSushi/toml, `x/text/cases`, protobuf, uuid, cobra + pflag (Linux + Windows) | ~47 weeks after Phase 16 starts\*\* | 38–64 |
| [ ] | **M11** `net/http`, TLS, the netpoller and gin | Phase 16's M11 exit criteria (Linux + Windows) | ~59 weeks after Phase 16 starts\*\* | 48–80 |

\* One person full-time, adding up the middle of each phase's "Size" in order: 00 → 01 → 02 (M1)
→ language server v0 (2 weeks, L0) → 03 … 09 → 10 → the rest of 11 (5 weeks) → 12 → 13 → 14
(Windows) → 15. The low end adds up the smallest sizes; the high end adds up the largest plus
15 % for surprises, which grow the further out a milestone is. The audit raised several sizes, and
your decisions added the Windows phase (about 5 weeks) and about a week to Phase 10 (`!`
functions and wrap macros), so v1.0 is now about a year and a half to two years away for one
person. AI-assisted work can be much faster. The table "What each phase needs" in the root README
§8 shows what can overlap if more people help.

\*\* Phase 16 is not scheduled: it starts when you decide, so M10 and M11 count from its start,
with the same rule (the sizes of its parts A–E for M10, A–F for M11, plus 15 % on the high end).

## Rules for updating

1. Steps are built with the 3 coders → PM merge → review-until-clean loop in [WORKFLOW.md](WORKFLOW.md).
   The manager marks a step `[~]` when its job starts, and `[x]` only when the review is clean
   and the step's tests pass.
2. A phase is `[~]` as soon as one step is `[~]`, and `[x]` only when **all exit criteria** are `[x]`.
3. Update this file's phase line and "Current focus" at the same time.
4. Write decisions and surprises in the phase's "Decisions / notes" with the date.
5. Keep [TODO.md](TODO.md) as the short "what's next" list.

## Log

- 2026-10-13: Plan created. Spikes validated the front-end (`go/parser` + `go/types` with
  `FakeImportC`), clang type/constant probing, stock `go/printer`, Go output formats, the
  stdlib porting blockers, and the core C++ idioms on both compilers. Phase 00 is in progress,
  waiting for decisions in TODO.md.
- 2026-10-13: Added Phase 10 (`F!()` must-calls + macros) and Phase 11 (fast language server).
  Old phases 10/11/12 became 12/13/14, the LSP moved out of tooling into Phase 11, and
  `gocpp fmt` moved into Phase 01. Both new designs were validated by spikes on stock `go/types`,
  and the `go/types` re-check speed was measured.
- 2026-10-13: Full audit by 5 independent reviewers (root + 00, 01–03, 04–06, 07–09, 10–14):
  122 findings (34 high, 65 medium, 23 low). Findings were checked before fixing, the important
  ones with small experiments; the fixes are listed per phase in each STATE.md "Decisions /
  notes". Phases 03–09 became a strict chain, and the timeline above was recomputed from the
  new sizes.
- 2026-10-13: Your answers to Q1–Q13 applied everywhere. Headers are packages and C++ namespaces
  are packages (`raylib.InitWindow`, `std.vector`); `!` works on functions and on calls; on-top
  macros have a start style and a wrap style (`#body()`); `go` is an OS thread with no scheduler;
  Linux and Windows are the targets (new Phase 14: Windows; hardening moved to Phase 15, v1.0 is
  now M9). Three new spikes back the new designs. Timeline recomputed.
- 2026-10-13: Added [WORKFLOW.md](WORKFLOW.md): every step is built by a manager (assigns the job),
  a coder (builds it) and a reviewer (reviews it and gives it back), looping until the review is
  clean; templates for tickets, reports and reviews, and a gocpp review checklist.
- 2026-10-21: Phase 16 (dual engine) planned in full from your four answers: gp stays on reference
  counting, Go packages and a second standard library run on bdwgc behind `gp::gc`, values are
  converted at the boundary; 21 steps, M10 (Go libraries pass their own tests) and M11 (`net/http`,
  TLS and gin on Linux and Windows). Not scheduled.
- 2026-10-22: Phase 16 reviewed by five reviewers (15 serious, about 30 medium and about 20 low
  findings; the serious ones checked against gocpp's code, Go's source and bdwgc's source) and
  corrected: the boundary keeps Go memory in place and copies gp pointers only for a list of
  call-only parameters, threads unregister from the collector, Go's tool tags and exact layout,
  a bootstrap step before the boundary, the counting rule in the exit criteria. Now 22 steps
  and 52–77 weeks; M10 ~47 weeks after Phase 16 starts (38–64), M11 ~59 (48–80). Your answers:
  Windows stays in M10, and standard imports use gp's library first, with the engine move that
  a later port causes documented.
