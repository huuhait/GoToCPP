# Handoff: where the work is

## Active continuation — 2026-10-20

This section supersedes the older checkpoints below. The user asked to continue the work
and plan, keeping the latest limit of **five active workers total**, including reviewers.

At recovery, `dev` was `94c4a6d9`. Integrated work now includes 9.12 docs, 11.14 guarded
checks, the safe PCH/prefetch portion of 12.14, 15.2 escape analysis, 15.7a compiler-cache
documentation/tests, and 9.13a release preparation. The global LSP and cold-probing
performance criteria remain open. The 11.14b current-implementation measurements passed
independent review and manager recomputation: eight consistent wins, four inconclusive small
typing rows, no qualifying slowdown or code change. Step 15.6a concrete-interface dispatch
is integrated after clean review and focused source/integration checks. Step 15.7b generic
body placement is also integrated: measured headers are smaller, but whole-build speed
benefit remains inconclusive. Active jobs are 12.13a background header indexing/completion
review fixes and bounded 15.6b bounds-check help.
The 9.13a changes passed independent re-review and focused source/integrated manager
checks. Full 9.13/M6 remain open; no release or tag is authorized.
User-owned `c4/` has not been changed by the manager.

| Job | Existing worktree / branch | Resume gate |
|---|---|---|
| 11.14 LSP speed pass | `../GoToCPP-11.14` / `job/11.14` | Guarded-check implementation integrated from `4e444985` after clean whole-net review and passing focused manager checks. Remaining final-build benchmarks and every-row gopls goal are open. |
| 12.14 cold typed probing | `../GoToCPP-12.14` / `job/12.14` | Safe partial implementation integrated from `d13cf67a` after clean round-4 review and focused/combined manager checks. Full type preambles retained; prefetch cancellation and immutable sharing fixed. Cold < 0.5 s remains unmet. |
| 15.2 escape analysis | `../GoToCPP-15.2` / `job/15.2` | Integrated from `b68eb6ac` after clean whole-net review and passing focused/combined manager checks. Array aliases, shared stack budgets, generic facts and shadow-dependent holders are covered. Full benchmarks and platform acceptance remain open. |
| 15.6a concrete interface calls | `../GoToCPP-15.6a` / `job/15.6a` | Integrated from `01ec0226` after clean whole-net review and focused source/integrated manager checks. Ownership, ordering, nil/recover and fallback behavior preserved; kernel-only performance evidence. Bounds checks and full Phase 15 acceptance remain open. |
| 9.12 stdlib documentation | `../GoToCPP-9.12` / `job/9.12` | Integrated from `b5c96982` after clean round-2 review and passing manager inventory/link/snippet/whitespace checks. The release candidate and full Phase 09 acceptance remain open. |
| 15.7a compiler-cache integration | `../GoToCPP-15-15.7` / `job/15-15.7` | Existing launcher support documented/tested and integrated from `a67c4e69` after clean review and manager checks. No production change, real cache-hit evidence or speedup claim. Remaining 15.7 work is open. |
| 15.7b generic body placement | `../GoToCPP-15.7b` / `job/15.7b` | Integrated from `724fac60` after clean review and focused source/integrated checks. Larger bodies emitted once, small methods kept inline. Measured header size falls about 14%; whole-build speed benefit is inconclusive. Modules and full 15.7 acceptance remain open. |
| 9.13a release preparation | `../GoToCPP-9.13` / `job/9.13` | Integrated from `235ca9ba` after clean round-2 review and focused source/integrated manager checks. Notices, installation instructions and byte-identical reserved-filename repair included; real offline local-proxy installation passes. No release archives, Nix installation, new platform run or publication. Full 9.13/M6 remain open. |

One coder and one independent reviewer per job, sequentially: no subject edits during review,
no delegation by workers, and final reports only except genuinely blocked decisions. Two
workers are active: the 12.13a coder addressing the complete first-review findings and the
15.6b coder. The bounds-check worktree is
`../GoToCPP-15.6b` on `job/15.6b`, based on `a07dda2a`; existing safe range and branch-hint
behavior is checked before considering any new optimization. The new 15.7b worktree is
`../GoToCPP-15.7b` on `job/15.7b`, based on `1c7081e9`; larger generic-type bodies are its
bounded scope, not C++20 modules. Start the next review when its coder finishes; never fill
slots with blocked work. Continue those subjobs, then the remaining
12.13 editor features, 15.6, 15.7 and 15.4 in dependency order. An unmet exit criterion is
not waived by merging an independently reviewed partial improvement.

Only necessary focused checks may run. Nobody runs `make ci`, `verify.sh`, full test,
interop, phase or sanitizer sweeps; final broad verification belongs to the user. Heavy work
uses the existing `testslot.sh`, bounded timeouts and isolated caches. Windows testing uses
the real PC over SSH, never Wine. Never delete another job's temporary files. The manager
pushes `origin dev` after every dev commit, never force-pushing.

The manager's local recovery notes retain the original tickets, test receipts and review
findings. The previous queue stopped updating before the four resumed jobs were launched;
use this checkpoint and the latest reviewed state rather than its historical worker list.

## Earlier resume checkpoint — 2026-10-20

This checkpoint supersedes the historical focus, resource and validation instructions below.
At recovery, `dev` is `69364ffc` (9.11 merged); `origin/dev` has no local commit difference.
The only untracked path in the manager checkout is `c4/`; leave it alone.

Latest integration checkpoint: native fmt `31514e86`, round-6 core `e745f037`, normal parsing
`9a2b0871`, restricted rename `ea1005e4`, and receiver/parameter optimization `b9d4b506` are
committed and pushed. The final `!`/macro editor integration is accepted in this checkpoint.
All four originally resumed jobs have now passed their coder/reviewer loops and focused
manager gates; this does not close the project's remaining phase/release/performance criteria.

Four unfinished jobs were recovered from the final previous-session messages. Prior workers
are no longer reachable; continue in the existing worktrees, not replacement branches:

| Job | Existing worktree / branch | Last known gate (not a clean review) |
|---|---|---|
| fmt audit | `../GoToCPP-09-fmtaudit` / `job/09-fmtaudit` | Integrated after clean whole-diff review at `16dcf25d` and passing manager focused checks: ten fixtures on GCC/Clang plus real Go, close O0/O2/ASan/TSan, cleanup race tests, build/vet and port hashes. Three close findings and seven Scan/lowering/tool-cleanup findings are fixed. No full CI or upstream sweep; remaining limits are recorded in Phase 09 STATE. |
| 11.13, `!`/macro editor support | `../GoToCPP-11-11.13` / `job/11-11.13` | Integrated after whole-current-net review clean at `c69600da` and passing manager feature/receiver checks. Selected post-rename limits met: big-package typing 97.755 ms, hover 22.409 ms, peak RSS 160.566 MiB, complete valid replies. Missing editor deps were installed from the unchanged lockfile and the affected check passed. Full-project/gopls/manual editor acceptance remains open; source Windows evidence retains its cleanup-overlay qualification. |
| LSP performance round 6 | `../GoToCPP-11-lspperf6` / `job/11-lspperf6` | Integrated as `e745f037` after clean review at `051eb2bf` and focused checks. The subsequent normal-loader parallel-parser change is also integrated after clean review at `541db0ab` and passing manager loader/cache/checker/multi-package/line/native-fmt checks. Matched small/medium evidence showed 6.6–13.3% lower p95, with slightly higher memory. Type checking, rename and beat-gopls/all-project acceptance remain open. |
| 15.3, receiver/parameter optimizations | `../GoToCPP-15-recvparams` / `job/15-recvparams` | Integrated after whole-net production/test-reliability review clean at `e91d0ced` and passing manager focused checks. All 19 hot fixtures and the cross-package module are synchronized, with isolated fact/copy-lend controls. Corrected pointer/value generator smoke passed against Go and both ASan compilers. Full performance/platform/campaign acceptance remains open; see Phase 15 STATE. |

The resumed jobs are integrated. Future jobs follow one coder and one independent reviewer
per job, looping to clean. Agents work quietly: final reports only,
except genuine blocked decisions; no mid-round relays or delegation without manager assignment.
Each coder owns a separate existing worktree. The two LSP branches overlap in the benchmark
harness; reconcile only during integration. Do not overwrite uncommitted work. Recovered fmt reproductions and logs are under
`~/.cache/gocpp-resume.rPhZsx/recovery/fmt-review/`; the shared timing lock is
`~/.cache/gocpp-resume.rPhZsx/perf.lock`. Preserve these artifacts. The saved stdin-cancellation
comparison already differs at the earlier round-2 baseline; do not present it as a newly
introduced deferred-close defect.

Current user rules:
- At most **10 workers**, and no force-filling slots with blocked jobs.
- Run **only necessary focused tests**. Nobody runs `make ci`, `verify.sh`, full test,
  interop, phase or sanitizer sweeps; the user performs the full final review manually.
- Independent review must be clean, followed by manager focused checks, before squash merge.
- Windows checks use the real Windows PC over SSH, never Wine; do not commit its address.
- Heavy work uses the shared `testslot.sh` wrapper and capped `gocpp-work.slice` (150 CPU cores,
  372 GiB memory maximum); use bounded waits and isolated caches. Never delete another job's
  temporary files.
- Workers do not push. The manager pushes `origin dev` after **every** dev commit, never force.
- Preserve benchmark workload/config/binary identity and actual run order; do not mix historical
  Go 1.22 workload results with fresh Go 1.25 results or report a median of peaks as a peak.

Current session queue:
`/tmp/claude-1000/-home-huuhait-GoToCPP/630caf9c-9db4-4c66-8c7e-a72ff32a86f2/scratchpad/agent-queue.md`.
Historical scratch reports and test wrappers remain under the previous session's
`fa18916c-20de-43da-b276-dbf712f662ca/scratchpad/` directory in the same parent.

## Historical handoff

Updated: 2026-10-16. The server migration is done; work resumed on the larger server
(128 cores, about 200 GB RAM: use high `-parallel` and run independent checks at once).

## On `dev`

Phase 04 steps 4.1–4.7, 4.9, 4.12–4.15 and 4.17, plus the runtime helpers. Phase 04 is not
done: C++-value closures/defers (4.8), `unsafe`/pointers (4.10), callbacks (4.11), helper
headers (4.16) and the closeout (4.18/4.19) remain. Full Phase 04 examples still fail until
4.8 and 4.10 land.

- 4.7 (range over C/C++ values, `gp::values`) merged as `3a34afc` after a clean round-3
  review and manager re-verification; see the dated note in
  [phase-04-cpp-interop/STATE.md](phase-04-cpp-interop/STATE.md).

## Performance work (2026-10-16): not plan steps

Merged into `dev` (each after independent review and manager re-verification: `make ci`, staticcheck
and the Phase 04 interop tests on g++ and clang++, normally and with ASan/UBSan):

- `10208fb` **`make quick`** (`tools/quick`): the edit-test loop. Only what differs from the merge base
  with `dev`: gofmt, vet and the tests of the changed Go packages, the changed test fixtures with one
  compiler, a small smoke set, and for `runtime/` changes its fast tests and the debug-mode C++ unit
  tests. About 10 s for a small change. **Not a gate**: `make ci` (about 11 minutes), `make staticcheck`
  and, for runtime or emitter changes, `make ci-sanitize` are still required before a merge.
- `0b73545` **Probe results on disk** (`internal/interop/diskcache.go`; Phase 06 §2.10, early): a second
  gocpp process, or the editor after a restart, answers the kind, namespace and macro probes and the C
  header check from `$XDG_CACHE_HOME/gocpp/probe` (about 1-1.5 s to about 45 ms for the cpp-std include
  set). Entries older than one hour are a miss (a header that appears earlier on the search path is
  otherwise not noticed); results with include errors, with `-march=native`, or from a missing header are
  not stored. `GOCPP_NOPROBECACHE=1` turns it off.

- `9e371df` **Precompiled runtime header and object cache** (`internal/build/pch.go`, `objcache.go`; Phase
  06 §2.5 and §2.10, early). The header of `gocpp/rt.hpp` is built once per compiler and flags and used
  for the compile of the generated C++: a warm `hello` build about 1.3 s to 0.5 s. The object cache skips
  the C++ compile of an unchanged program (median 0.88 s to 0.23 s over 40 programs); **any edit misses**,
  since a program is one translation unit until Phase 06 splits it. `-x` explains every compile that is
  not cached (`# not cached: <reason>`). `GOCPP_NOOBJCACHE=1` turns it off; `GOCPP_OBJCACHE_SIZE` bounds it
  (2 GiB). Five independent review rounds found, among others, a Windows/macOS build break and several
  stale-hit cases, all fixed; known limits are in the comment at the top of `objcache.go` (a new file
  earlier on CPATH or the compiler's own folders, a system header shadowed by a file in an -I folder, more
  than 10000 include candidates are never stored).
- `71cc4fd` the same header in the `cppgen` tests (that package 161 s to 131 s).
- `4940a6e` **`runtime` tests compile in parallel** (`runtime/parallel_test.go`, a priority gate `GOCPP_TEST_JOBS`):
  that package 570 s to about 85 s on a quiet machine; `make ci` about 11 min to about 2m40s quiet. Coverage compared
  subtest by subtest: 736 passed before, 744 after, none skipped.
- `c16cdb2` cmd/gocpp tests no longer depend on the object cache (`TestMain` turns it off; `TestObjCacheOffByDefault`
  guards it): a `-x` test failed or passed depending on machine speed.
- 4.16 (helper headers as packages) merged as `b72404c`; see phase-04 STATE.
- `1477a68` **WaitDelay and core dumps**: a compiler that exited 0 but kept its output open was reported as a
  failure under load (`gocpp's runtime did not compile ... WaitDelay expired`); the tests that crash a program on
  purpose no longer stall in systemd-coredump (prctl(PR_SET_DUMPABLE, 0)); the probe cancel and output-limit tests are
  load-proof. Verified on `dev`: `make ci` green (1404 s at load ~400), staticcheck, Phase 04 interop (ASan
  fully green; the normal run had only a `g++ --version` detection timeout under load, which passes alone).

**Machine load warning (2026-10-17):** with a dozen coder, reviewer and verification jobs at once the load average
reached 400-550 on 128 cores. At that load timing-based tests fail at random (probe timeouts, `TestLSPProcess`,
`TestProbeCancelStopsTheCompiler`, deliberate-crash tests stalling in systemd-coredump, the `WaitDelay` bug): rerun
a failing test alone before believing it, and keep the number of simultaneous heavy jobs near the core count.
The g++ crash with the precompiled header (`fix/pch-ice`) is fixed and merged (unique temp names, per-process single-flight, fallback when the compiler dies).

Measured and rejected: scratch files in `/dev/shm` (no change); `mold` saves only 40-90 ms per build
(`GOCPP_LDFLAGS=-fuse-ld=mold` with `mold` in the dev shell if wanted). The object cache only helps an
unchanged program: any edit changes the single translation unit, so it misses (Phase 06's one unit per
package or function is what fixes that). Why `make ci` is still about 11 minutes: the `runtime` package takes about 600 s,
the critical path, because `TestCXX` compiles about 12 programs one after another in each of 6
(compiler, mode) chains, leaving most of the 128 cores idle; compiling them in parallel is the next
candidate. `cmd/gocpp` (about 130-250 s) is serial because its tests use `t.Chdir`.

## Rules that still hold

- Follow `WORKFLOW.md`: coder, then an independent reviewer, fixes until clean, manager
  re-runs the checks, then the completion markers. Green CI alone is not approval.
- Outstanding design questions in TODO.md remain unanswered; resuming work is not approval for
  license changes, upstream reports, automatic C++ dependency recipes or moving module
  downloads.
- Nothing is pushed unless the user asks.

## Next plan jobs

4.8, 4.10, 4.11 and 4.16, then the Phase 04 tests/docs closeout (4.18/4.19).

## Validation

Use the repository's Nix development shell.

```sh
nix develop -c make ci
nix develop -c make staticcheck
GOCPP_TEST_PHASE=04 GOCPP_REQUIRE_CXX=g++,clang++ \
  nix develop -c go test -count=1 -timeout 30m -parallel 32 \
  -run '^TestInterop$' ./tests
GOCPP_FLAGS=-asan GOCPP_TEST_PHASE=04 GOCPP_REQUIRE_CXX=g++,clang++ \
  nix develop -c go test -count=1 -timeout 30m -parallel 32 \
  -run '^TestInterop$' ./tests
```

Use isolated test caches as in `tests/README.md`; never write the user's real
`~/.cache/gocpp` from tests. Do not run GUI programs, Xvfb or xvfb-run. Select only the
non-GUI cpp-std, mixed-sources and sqlite examples for run checks; build/link raylib only.
NixOS has no `/bin/bash`: run scripts with `bash script.sh`.

## State at 2026-10-17 (end of the second big wave)

`dev` (60f6be7) now also has: 10.1 (parser/printer for `!` and macros), 6.1 (gocpp.mod), 5.1-5.3 (runtime type descriptors and
interfaces), 4.8 (opaque objects in closures/defers), the precompiled-header crash fix, and the cmd/gocpp object-cache test fix.
Verified on `dev` at 60f6be7: staticcheck, the Phase 04 interop tests (normal and ASan) green; `make ci` had only three timing
failures that each pass alone (TestLSPProcess, TestScales, the fmt speed check: see TODO.md).

Waiting in worktrees next to the main checkout (agents resumable; details in the dated STATE notes when they merge): 4.10 (unsafe and
pointers: reviewers asked to make every gocpp-side `*C.T` a `gp::cptr<T>`; reconciling with 4.8), 8.15/8.1 (thread layer and goroutine
runtime: confirmation review), 11.8/11.9 (language server: confirmation review), 6.1b (module header imports) and 6.1c (module
downloads) just started. Decisions Q16/Q17 (TODO.md): libraries from the internet are built NOW; local headers import by the
project's module path like Go packages (`import "github.com/huuhait/project/native/x.h"`). Next after 6.1b: 6.1d (read and build
the `cxx` recipes).

## Update 2026-10-17 (late)

`dev` also has 11.8/11.9, 8.1/8.15, 6.1b and 6.1c (downloads, ADR-023). Full verification of dev at e098b21: staticcheck green;
`make ci` and the Phase 04 interop tests (normal and ASan) had one REAL failure, fixed afterwards (TestFatal/stack-overflow in
internal/cppgen: 8.1's new `goroutine 1 [running]:` line; the coder of 8.1 had not run that package: lesson: a change to the
runtime's fatal-error output needs `go test ./internal/cppgen` too) and timing failures that pass alone (TestOrderLarge, two interop
tests at the 2-minute limit under load 365, the fmt speed check now a ratio). In flight: 4.10 (review round 4 found a lost owner when
a raw member is READ; coder adding `load_keep`, the free path, slice/struct owners), 6.1d (cxx recipes, coder running).

## State at 2026-10-20 (new PC "ServerLab": 192 cores, 503 GB; work capped at 150 cores / 372 GB RAM by the user-level cgroup `gocpp-work.slice`)

Merged to `dev` today (squash, each after an independent review; NOT run: `make ci`, interop, ASan and phase sweeps: the user runs `make ci` by hand as the final review):
13.3 vet, 13.4 doc, the named-result shadow fix, 7.8 range-over-func, 12.1/12.2 typed interop collector+probes, 9.3 reflect subset, 6.5-6.7 stamp/up-to-date check, 11.11/11.12 LSP benchmarks. After the merges a focused smoke on dev passed (build, vet, internal/lower, check, stdlib, stubgen, build, typedinterop/probe, TestRun reflect_/rangefunc_/named_result_).
Rules in force (user): max 10 workers; run only needed tests; nobody runs `make ci` or full sweeps; agents commit often; ADR numbers hand-allocated (used: 035 reflect, 036 build stamp, 037 must (10.2 branch), 038 exit/runtime (8.9 branch), 040 sync (8.7 branch), 045 shims (9.2 branch), 053 Windows (14.1 branch), 060 typed interop, 062 math (9.5 branch), 063 doc, 067 os (9.6 branch)).
In flight: 13.1 (round 2 done, awaits small re-review), 14.1 (recheck), 10.2 (review), 8.7 (review), 8.9/8.10 (review), 9.5 (review), 9.2 (coder), 8.8 (coder), 7.9 (coder). Waiting for prerequisites: 9.6 os (needs 8.7, 8.8, 8.9, 9.2), 6.10/6.11/6.13/6.8 (rebase on 6.5, now merged), lspperf (11.11 merged: rebase now), 9.9, 13.2, 13.5, 15.x.
Scripts (scratchpad): heavy.sh (12 slots), testslot.sh (24), common-brief.txt (rules for agents), /tmp/claude-1000/mergefull.sh (rebase keep-both on list files, zero-D check, squash merge).
