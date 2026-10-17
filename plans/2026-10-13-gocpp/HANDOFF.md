# Server migration checkpoint

Updated: 2026-10-16. **Work is paused.**

## Status at the pause

The user requested a pause after the current batch (4.6, 4.7 and 4.9), before
starting another plan job, to move the project to a larger server.

- **4.6 is done and merged.** Commit `99e75ad`.
- **4.9 is done and merged.** Commit `9695323`.
- **4.7 is saved as unapproved WIP, not merged.** Branch `job/04-4.7`, commit
  `b3300cf945087a55e09ed630d860786b8b6f43b4`. Service errors prevented completion
  of its last verification check; it has not been marked done.
- No later plan jobs, push or transfer were started. Resume only when the user
  asks. Outstanding design questions in TODO remain unanswered; migration is
  not approval for license changes, upstream reports, automatic C++ dependency
  recipes or moving module downloads.

All source changes are committed, either on `dev` or on the explicitly labelled
WIP branch. Linked working directories were removed only after confirming they
were clean; the commits and local branch refs remain. There is no need to copy
the former absolute worktree paths.

## Committed and verified on dev

`dev` includes Phase 04 steps 4.1–4.6, 4.9, 4.12–4.15 and 4.17, plus the runtime
helpers. Phase 04 itself is not done: C++-value closures/defers, unsafe/pointers,
callbacks and the phase closeout remain unfinished.

### 4.6: C/C++ composite literals

- Clean independent review in round 3.
- Covers positional/keyed C/C++ literals, map-only key:value forms, bit-field
  initialization, Go containers holding C values, and inferred package
  ellipsis arrays.
- Manager ran the package-array regression on GCC and Clang normally and with
  ASan/UBSan, then `make ci` and staticcheck on dev. All exited 0.
- Inferred package `[...]C.int` arrays use their checked length through
  `typeWith`; package positional/keyed variants have regression coverage.

### 4.9: typed helpers and the string boundary

- Clean independent review in round 4.
- Typed `C.GoString`, `GoStringN`, `GoBytes`, `CString`, `free` and `cstr`
  helpers; holder ownership through ordered and variadic calls; C++ string
  conversions; public helper hover/signature/completion names.
- Known C numeric targets get overflow/truncation checks, including typed
  constant conversions and known container-element/dereference targets.
  Large zero-imaginary integer constants remain exact, without a double
  intermediate. Opaque arithmetic result types are not guessed from operands.
- Direct and copied C.cstr holders in keyed or positional C/C++ literals are
  rejected with a lifetime diagnostic. Retained CString pointer fields have
  positive tests. This closes the cross-job 4.6/4.9 lifetime checkpoint.
- Manager reran `make ci`, staticcheck and all **71 Phase 04 interop fixtures**
  on GCC and Clang normally and with ASan/UBSan. All commands exited 0:
  **284 compiler/mode subcases**, no skips. The tested implementation was
  unchanged by the subsequent rebase, which incorporated only plan commits.
- This is not a claim that all Phase 04 examples pass. Unfinished range/unsafe
  work still blocks examples.

## 4.7: preserve and finish verification before merging

- Branch: `job/04-4.7`.
- WIP commit: `b3300cf945087a55e09ed630d860786b8b6f43b4`.
- Base: `99e75ad`; it predates merged 4.9. Rebase carefully onto current dev
  before final integration, retaining both jobs' changes in shared lowering
  and runtime files.
- All eight original review findings have implementation fixes and regression
  coverage. The reviewer independently reran the original probes on GCC and
  Clang with sanitizers; they now pass or give the expected user errors.
- Final-source focused units, full Phase 04 interop tests normally and under
  sanitizers, retention tests and staticcheck passed. Full final-source CI
  also finished with exit 0 after the reviewer handed back the work. The
  manager read `ci-final.status` and the successful end of `ci-final.log`.
- The coder and reviewer encountered repeated HTTP 502 policy-service errors.
  Retries were stopped; no clean verdict was fabricated.
- One additional **unconfirmed** edge remains: `nameOperands` can hoist literal
  `0` passed to a C++ range factory into an integer variable, losing its
  null-pointer-constant property. Check a factory taking `const char*` with
  `for range lib.f(0)`. Inspect `internal/lower/order.go` around `nameOperands`
  (the saved review cited lines 1274–1295). Passing `nil`, emitted as
  `nullptr`, is a different case.
- Finish that edge check, rebase/integrate with 4.9, obtain an independent
  clean verdict and run manager validation before merging. **Green CI alone
  is not approval.** Do not mark this branch complete merely because it has
  a commit.

## What to carry to the server

Carry the repository **including `.git`, its history and local job branches**.
An ordinary clone of a remote, or copying only the current dev files, does not
carry local WIP branches. In particular, preserve `job/04-4.7` / `b3300cf`.
A Git bundle containing all refs is another possible transfer format, but none
has been created or transferred without the user's request.

There are no linked worktrees left to relocate. On the server, recreate a
working tree from the WIP branch when resuming it. Runtime/build caches are
disposable; they are not required source artifacts.

Logs currently live under `/dev/shm/manager-batch`,
`/dev/shm/resume-review-46`, `/dev/shm/resume-review-47`,
`/dev/shm/resume-review-49`, `/dev/shm/resume-coder-46`,
`/dev/shm/resume-coder-47` and `/dev/shm/resume-coder-49`.
These are temporary memory-backed folders and will not survive a reboot or a
repository-only transfer. Preserve desired logs separately if requested.
Source regressions are committed. Final manager 4.9 logs use the `49-r4-`
prefix; final range CI is `resume-coder-47/ci-final.log`.

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
`~/.cache/gocpp` from tests. Do not run GUI programs, Xvfb or xvfb-run. Select
only the non-GUI cpp-std, mixed-sources and sqlite examples for run checks;
build/link raylib only. Full Phase 04 example success is not yet expected.

## Resume order after the migration

1. Verify and integrate 4.7; do not treat the saved WIP as approved.
2. Follow `WORKFLOW.md`: manager → coder → independent reviewer, fixes until
   clean, manager verification, then completion markers.
3. Only after the user resumes plan execution, start 4.8, 4.10, 4.11 and 4.16,
   then the Phase 04 tests/docs closeout (4.18/4.19).
