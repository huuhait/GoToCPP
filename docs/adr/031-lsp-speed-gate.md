# ADR-031: The speed gate of the language server: 3x the budget and 2.5x gopls, opt-in; the 1.5x goal is not met yet

Status: accepted (2026-10-18)

## Context

[ADR-015](015-language-server.md) says that the speed budgets are measured in CI, that "a regression
over 10 % fails CI", and that on the same code every row must be within 1.5x of gopls. Steps 11.11 and
11.12 built the harness (`tools/lspbench`, `internal/lsp/bench`) and measured
([docs/benchmarks/lsp.md](../benchmarks/lsp.md)). Three things in ADR-015 did not survive contact with a
shared machine and with the numbers:

- **A 10 % regression gate cannot work here.** The machine has 128 cores and several agents; the load
  average was 28 to 120 during the runs, and the figures move by tens of percent between runs (the
  small ones, under a millisecond, by more). A gate tighter than the noise flakes; the repo has had
  many timing-test flakes already.
- **The 1.5x row is not met.** All p95 budgets of ADR-015 are met on the three reference projects, with
  one exception (references of a method that all types share, 0.7 to 0.95 s against 300 ms), but several
  rows are slower than gopls by more than 1.5x: keystroke 14-18x on the small and medium projects
  (31 ms against 1.5-2 ms), references-popular 3x, references-local up to 12x on the large project,
  rename 6x, workspace-symbol 2-2.5x.
- **The keystroke row differs by design.** gocpp waits 30 ms after the last keystroke before it checks
  (ADR-015: "diagnostics wait about 30 ms"); gopls type-checks the edited package at once (its
  `diagnosticsDelay` only delays the slow analyses). The budget (100 ms) is met; the comparison with
  gopls is not, and cannot be while the debounce is fixed.

## Decision

1. **The gate** is the Go test `TestLSPBudgets` (`make lsp-budgets`). It starts the real server (and gopls
   when it is installed) on the small and medium reference projects and takes, per row, the median of five
   sessions' p95. A row fails only when its p95 is over **3x its budget** and, when gopls ran in the same
   test, also over **2.5x gopls's p95**; a failing measurement is repeated once, and only a repeated
   failure fails the test. A scenario that fails more than 5 % of its repetitions fails, and so does a row marked `Known` that now
   meets its budget (an exemption does not outlive its fix). `Known` is per project: today it covers
   references-family on the medium and large projects, where the problem was measured, not on the small
   one, which meets the budget. Rows whose p95 is under 5 ms never fail the gate (the ratio to gopls is
   timer noise there). The medium project's
   peak memory is held to 400 MiB.
2. **It is opt-in** (`GOCPP_LSP_BUDGETS=on`, which `make lsp-budgets` sets): it takes a minute and a
   half, builds gocpp and uses gopls, so it is not part of `go test ./...` or `make ci`. It skips itself
   when the load average is above a limit (`GOCPP_LSP_MAXLOAD`, 16 on a big machine, at least 4), and
   reports "inconclusive" instead of failing when the machine got busy while it ran.
3. **Regressions** over 10 % are not gated. They are found by running `make lsp-bench` on a quiet machine
   and comparing the committed table; the p95 budgets stay the contract.
4. **The 1.5x-of-gopls criterion stays open.** It is not met today, and is tracked by the language-server
   performance job in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md) (the user wants the server optimized
   as far as it goes: keystroke without the debounce when a check is fast, the references family narrowed
   to gopls's use-tied unification, rename and workspace symbols, then tighter budgets and no `Known`
   exclusion). Phase 11's exit criterion keeps the 1.5x row until then. (Done in part:
   [ADR-052](052-lsp-speed-work.md).)

## Alternatives

- **Store the committed medians and fail at +10 %** (rejected): the table comes from one machine at one
  load; another machine or a busier minute would fail or pass for no reason.
- **Absolute bounds only** (rejected): the same code is slower on a loaded or small machine; comparing
  with gopls on the same run is what makes a row robust when gopls is installed.
- **Run the gate in `make ci`** (rejected): it needs a quiet machine and a minute and a half, builds
  binaries and starts a second server; an unconditional run would flake or skip, and cost time on every
  change.

## Consequences

Good: a budget that is exceeded by a real regression (a request that takes 5x longer, a cache that stops
working) fails, with the same code that produces the table; the test cannot flake on a busy machine.

Bad: nothing runs it by default, so a regression is found only when someone runs `make lsp-budgets` or
`make lsp-bench` (the reviewer's checklist has the speed row). A slowdown of 2x that stays under the
safety factor is not caught by the gate, only by reading the table.

A note on the random-edit test that stands behind the speed work: a file read that fails (no file
descriptors, no memory) is retried a few times (about 100 ms) and, if it keeps failing, is read again
by the next snapshot, that is, on the next change; the server does not promise to heal before that.

## Evidence

- [docs/benchmarks/lsp.md](../benchmarks/lsp.md): 5 runs, load 28-60, gopls 0.21.1 on the same generated
  code; all four reference projects.
- The mutation tests of the gate (`internal/lsp/bench/run_test.go`): a hover that takes 150 ms, and
  diagnostics that take 400 ms, fail it; a fast server does not.

See: [Phase 11 §2.1, §2.5 and the exit criteria](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md).
