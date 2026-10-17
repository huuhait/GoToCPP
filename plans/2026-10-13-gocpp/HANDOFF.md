# Handoff: where the work is

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
Known fix in flight: `fix/pch-ice` (g++ crashed with the precompiled header in `TestProgramParallel`: the header
build named its temp file by pid, so Builders in one process shared it; fixed with unique temp names, a
per-process single-flight and a fallback when the compiler dies with the header).

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
