# ADR-090: The nightly sanitizer job fails on any report, and a leaked reference cycle is allowed only inside a measured ceiling

Status: accepted (2026-10-19)

## Context

Step 15.11 asks for nightly AddressSanitizer, UndefinedBehaviorSanitizer, ThreadSanitizer and
LeakSanitizer runs that fail on any report, and a leak audit. What existed: `make ci-sanitize` (the
tests that run programs with `-asan`, and the emitter's tests with the sanitizers in
`GOCPP_TEST_CXXFLAGS`), `make ci-race` (ThreadSanitizer, step 8.12), and the test header line
`// lsan: cycle`. Three things were missing:

1. **A report could hide behind a test's expectations.** `-asan` builds recover from UBSan
   reports (they print and go on), and a test that does not compare stderr passes with a report in
   it. The jobs also had no proof that they still see an error: a changed option or a flag that
   quietly turns a check off would make every night green.
2. **`// lsan: cycle` switches LeakSanitizer off for the whole program.** Reference counting never
   frees a cycle (divergence D1, ADR-004), so some tests leak on purpose: 18 today (recursive
   closures, mutually pointing structs, a `sync.Cond` whose locker is a field of the struct that holds
   the Cond). The marker also hides a real leak that someone adds to the same program later, forever.
3. **Nothing ran the sanitizers on a schedule**, and the repository has no CI service yet (Q9).

## Decision

1. **`tools/sanitize` is the job.** It builds each program of a corpus with a sanitizer, runs it with
   options that stop on the first report, scans the program's stderr for reports, and fails on any
   report (and on a build failure, a timeout, or an exit status that the header does not give). Modes:
   `asan` (gocpp's `-asan`: AddressSanitizer, LeakSanitizer and UBSan, leaks on), `ubsan` (UBSan
   alone, `-fno-sanitize-recover=undefined`, the real allocator), `tsan` (`-race`). A report is
   classified (`asan`, `ubsan`, `lsan`, `tsan`) from the text the sanitizer prints; the scan is
   independent of what a test expects, and a Go panic is not mistaken for a UBSan line (UBSan's lines
   have `file:line:col:` in front).
2. **Canaries.** `tools/sanitize/testdata/canary` holds programs that are wrong on purpose (a heap
   buffer overflow, a shift by 40, a leaked block, a data race) and clean controls. The job runs
   them first in every mode: a canary that is not reported fails the night (`canary-silent`: the
   sanitizer is blind), a control that is reported fails it. The tests of the tool run the same
   programs without the canary line and require the job to fail.
3. **The leak policy is a list with ceilings.** A program may say `// lsan: cycle` only if it is
   listed in `tools/sanitize/leaks.txt` with a kind of cycle, a reason, and the most it may leak, in
   bytes and allocations, for each compiler. `go test` fails when a marker and a line disagree (the
   policy test), and the audit (`tools/sanitize leaks`) builds each listed program with LeakSanitizer
   **on** and fails when it leaks more than its ceiling (a leak on top of the cycle), when it leaks
   nothing (a stale marker: remove it, so that a leak that comes back is reported), or when it makes
   another sanitizer report. A ceiling is lowered by `leaks -update` and raised only with
   `-allow-grow`, which is a review decision ("this is more cycle, not a leak"). A program whose leak
   depends on how its goroutines interleave may have a byte `slack` of at most 4 KiB; the allocation slack must be 0, which the policy test enforces. No line has a slack today: the two programs that varied were made deterministic.
4. **The job runs where the author runs it, and the workflow is ready.** `make sanitize-nightly`
   (and `tools/sanitize/nightly.sh` for cron or a systemd timer) runs the stages `canary`, `leaks`,
   `asan`, `ubsan`, `tsan` with g++ and clang++ and optionally `deep-sanitize` and `deep-race` (`make ci-sanitize` and
   `make ci-race`: the interop, diff, zoo and example programs, the emitter's own tests, the runtime's
   TSan unit tests). `.github/workflows/sanitizers-nightly.yml` runs the same targets, sharded; it
   does nothing until the repository is on GitHub (Q9), and a test keeps its make targets and flags
   real.

## Alternatives

- **Only `make ci-sanitize` on a timer.** It has none of the four properties above: no canaries, the
  marker is unbounded, and a report is only as visible as the test's stderr check.
- **Remove the marker and suppress the known cycles with an LSan suppression file.** A suppression
  names a stack, and a cycle made by a recursive closure has the same allocation stack as the leak
  of any other closure. Ceilings are coarser but cannot be fooled by a stack that looks alike.
- **Ceilings in the test header.** The numbers differ per compiler and change with the runtime;
  one list is easier to review, and the policy test sees every marker at once.
- **A tracing collector for cycles, or `weak` in the tests.** The tests leak cycles on purpose to show
  what the language does (D1); making them cycle-free would stop testing it.
- **MemorySanitizer.** GCC has none, and clang's needs an instrumented standard library.

## Consequences

- Good: a sanitizer that stops seeing errors, a marker that outlived its cycle, a real leak hidden
  behind a marker (above the ceiling), and a report in a test that ignores stderr all fail a
  night. `make sanitize-smoke` (14 programs, both compilers, about two minutes) shows on any machine
  that the job works; `make sanitize-selftest` shows that it fails when it must.
- Bad: the ceilings are exact allocation counts of the runtime and the standard library of the
  pinned compilers, so a runtime change that allocates differently moves them (`leaks -update`
  and a look at the diff). They bound a leak in a marked program to its ceiling; a leak smaller than
  a byte slack (none today) could hide a small one. The job costs hours of CPU time per night for all of
  `tests/run` with two compilers and three modes.
- Not covered: interop programs (they need C libraries; the `deep` stage runs them), Windows (MinGW
  has no sanitizers), Linux arm64 (step 15.12), a program that needs a display.

## Evidence

- `tools/sanitize/sanitize_test.go` (the scanner on real sanitizer texts, the runner against a fake
  gocpp, the policy test, the audit's verdicts and `-update`), `e2e_test.go` (`make sanitize-selftest`:
  real builds with injected errors).
- Measured on this machine (2026-10-19, g++ 15.3, clang++ 21.1): the 18 programs leak between 72 bytes
  in 1 allocation and 35,704 bytes in 175; g++ and clang++ agree except for threaded programs, whose
  numbers also move from run to run (`sync_cond_queue.gp`: 224 to 9,664 bytes in 3 allocations).

## See

[docs/guides/sanitizers.md](../guides/sanitizers.md), [tests/README.md](../../tests/README.md#sanitizers),
[ADR-004](004-memory-arc.md), [ADR-062](062-race-detector-on-tsan.md),
[Phase 15 §3](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#3-steps).
