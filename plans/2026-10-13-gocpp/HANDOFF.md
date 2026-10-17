# Server migration checkpoint

Updated: 2026-10-16.

## Stop boundary

The user asked to finish the current batch (4.6, 4.7 and 4.9), then pause before
starting another plan job so they can move the project to a larger server.
No push or transfer is authorized. This checkpoint is being prepared while 4.9
is still being repaired; it does not mean the whole batch is finished.

## Committed and verified

- `dev` includes Phase 04 steps 4.1–4.6, 4.12–4.15 and 4.17, plus the runtime helpers.
- Step 4.6: `99e75ad` (C/C++ composite literals), followed by `2778200`
  (verified completion recorded in STATE).
- For 4.6, the manager ran the package ellipsis-array regression on GCC and
  Clang normally and with ASan/UBSan, then `make ci` and staticcheck on dev.
  Every command exited 0. The independent review was clean in round 3.
- Phase 04 itself is not done. In particular, `unsafe`, callbacks and
  C++-value closures/defers remain unfinished.

## Work that must not be lost

### 4.7: preserved, not approved for merge

- Branch: `job/04-4.7`.
- WIP commit: `b3300cf945087a55e09ed630d860786b8b6f43b4`.
- Base: `99e75ad`.
- Local worktree: `/home/huuhait/GoToCPP-worktrees/wt-04-4.7`.
- All eight original review findings have implementation fixes and regression
  coverage. The reviewer independently reran the original probes on GCC and
  Clang with sanitizers: they now pass or give the expected user errors.
- Final-source focused units, full Phase 04 interop tests normally and under
  sanitizers, retention tests and staticcheck passed. Full final-source CI was
  still running when the reviewer handed back the work. An earlier CI pass
  predates the final lifetime repairs and is not a substitute.
- The coder and reviewer were interrupted by repeated HTTP 502 policy-service
  errors. Retries were stopped; no approval was fabricated.
- One additional **unconfirmed** edge remains: `nameOperands` can hoist the
  literal `0` passed to a C++ range factory into an integer variable, losing
  its null-pointer-constant property. Check a factory taking `const char*`
  with `for range lib.f(0)`. Inspect `internal/lower/order.go` around
  `nameOperands` (the saved review cited lines 1274–1295). Distinguish this
  from passing `nil`, which is emitted as `nullptr`.
- Finish that check and the final CI result, then obtain an independent clean
  verdict and manager verification before merging this branch.

### 4.9: round-3 repair in progress

- Branch: `job/04-4.9`.
- Local worktree: `/home/huuhait/GoToCPP-worktrees/wt-04-4.9`.
- This work is not yet approved or merged. Do not move only the `dev` branch
  and assume it contains these changes.
- Round-2 review confirmed three blockers:
  1. Known C array/slice/map elements and dereferenced targets can bypass
     constant representability checks; the same applies to some indexed
     operands and known C conversion/function results.
  2. Zero-imaginary integer constants were emitted through `double`, losing
     integers above 2^53 and potentially producing undefined conversions.
     Keep exact `go/constant` integer values at known integer targets.
  3. Explicit C conversions of typed constants can bypass overflow/truncation
     checks because dispatch checked only untyped constants.
- The repair also rebases onto 4.6 and must test both direct `C.cstr(s)` and
  holder-variable values inside keyed C literals. A temporary holder must
  never die inside a field-assignment lambda before the containing call.
  Rejection with a clear error is an acceptable safe policy.
- After the repair: independent review, manager checks, then commit/merge.

## What to carry to the server

Carry the repository **including its Git history and local job branches**.
An ordinary clone of a remote or copying only the current `dev` files will not
carry local WIP branches or uncommitted work. Wait for the final checkpoint,
or explicitly preserve the 4.9 worktree too if moving before it is committed.

Linked worktrees refer back to this machine's main `.git` directory. On the
server, recreate worktrees from the retained job branches rather than assuming
the absolute worktree paths remain valid. Runtime/build caches are disposable;
they are not required source artifacts.

Logs currently live under `/dev/shm/manager-batch`,
`/dev/shm/resume-review-46`, `/dev/shm/resume-review-47`,
`/dev/shm/resume-review-49`, `/dev/shm/resume-coder-46`,
`/dev/shm/resume-coder-47` and `/dev/shm/resume-coder-49`.
These are temporary memory-backed folders and will not survive a reboot or a
repository-only transfer. Preserve any desired logs separately when the user
requests the actual transfer. The source regressions are in the job branches.

## Validation on the server

Use the repository's Nix development shell. Start with bounded parallelism;
raise it after checking the new server's memory use.

```sh
nix develop -c make ci
nix develop -c make staticcheck
GOCPP_TEST_PHASE=04 GOCPP_REQUIRE_CXX=g++,clang++ \
  nix develop -c go test -count=1 -timeout 30m -parallel 2 \
  -run '^TestInterop$' ./tests
GOCPP_FLAGS=-asan GOCPP_TEST_PHASE=04 GOCPP_REQUIRE_CXX=g++,clang++ \
  nix develop -c go test -count=1 -timeout 30m -parallel 2 \
  -run '^TestInterop$' ./tests
```

Use isolated test caches as in `tests/README.md`; never write the user's real
`~/.cache/gocpp` from tests. Do not run GUI programs, Xvfb or xvfb-run. For
example checks, select the non-GUI cpp-std, mixed-sources and sqlite examples;
build/link raylib only. Full Phase 04 example success is not yet expected:
unfinished steps include 4.10.

## Resume order after the migration

1. Finish and verify any remaining 4.7/4.9 work; do not mark WIP complete.
2. Follow `WORKFLOW.md`: manager → coder → independent reviewer, fixes until
   clean, manager verification, then completion markers.
3. Only after the user resumes plan execution, start the next steps: 4.8,
   4.10, 4.11 and 4.16, then the Phase 04 test/docs closeout (4.18/4.19).
4. Outstanding design questions in TODO remain unanswered. Do not infer
   approval for automatic C++ dependency recipes, moving module downloads,
   license changes or upstream reports from the migration request.
