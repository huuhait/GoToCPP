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

## Local branches that are NOT merged (performance work, 2026-10-16)

These live in sibling worktrees and are not pushed. They are not plan steps.

| Branch | What | State |
|---|---|---|
| `perf/build-speed` (`2a935ab`) | Precompiled header of `gocpp/rt.hpp` for the generated C++ (`internal/build/pch.go`); warm `hello` build about 1.3 s to 0.5 s | clean `make ci` passed; not reviewed by a second agent; not merged |
| `perf/test-parallel` | the same header in the `cppgen` tests (`internal/cppgen/cxx_test.go`); that package 161 s to 131 s | passes normally; uncommitted; not checked with the sanitizer flags of `make ci-sanitize` |
| `perf/probe-disk-cache` | probe results on disk (Phase 06 §2.10, early) | coder job in progress |
| `perf/object-cache` | content-addressed object cache for the generated C++ (Phase 06 §2.5, early) | coder job in progress |

Measured and rejected: scratch files in `/dev/shm` (no change); `mold` as the linker saves only
40–90 ms per build (use `GOCPP_LDFLAGS=-fuse-ld=mold` with `mold` in the dev shell if wanted).
`make ci` takes about 10–13 minutes; the slowest packages are `internal/cppgen`,
`cmd/gocpp` (serial: its tests use `t.Chdir`, so it can't use `t.Parallel`) and
`internal/check`. Work on the performance branches goes through the same
manager → coder → reviewer loop (WORKFLOW.md) before it merges.

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
