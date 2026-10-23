# ADR-142: the language server's soft memory limit adds the Go runtime's own memory, and its memory budget is gopls's

Status: accepted (2026-10-23; job 11.lspgap5; the memory cost and the budget relative to gopls accepted by
the user on 2026-10-23).

## Context

`gocpp lsp` sets a soft memory limit (`debug.SetMemoryLimit`) that follows the live heap: 256 MiB, or 1.4 times
the live heap and 64 MiB, whichever is more ([ADR-112](112-lsp-speed-round-6-the-miss-path-the-rename-check-and-memory.md),
kept by [ADR-119](119-lsp-speed-round-7-typing-rows-and-the-proofs-of-a-rename.md) and
[ADR-125](125-lsp-speed-round-9-the-tails-of-large-are-collector-cycles-and-fewer-bytes-allocated-lowers-them.md)).
The target of the language server since 2026-10-22 is every benchmark row at most 2 times gopls's p95 on the same
run (user).

`make lsp-bench` runs with the machine's default GOMAXPROCS (192 threads on the benchmark machine). Its 2026-10-23
run had `large keystroke-imported` at 4.69 times gopls (13.6 against 2.9 ms) and `large keystroke-new` at 2.12.

What was measured (internal/lsp/server/idlegc.go has the numbers too):

- `GODEBUG=gctrace=1` on `large`: a heap goal of 156 MiB for 146 MiB live, so a 70 ms cycle with mutator assists
  every keystroke or two.
- The limit counts all of the runtime's memory. The part that is no heap object (the per-P span caches,
  fragmentation, metadata, stacks; `runtime/metrics`: total − released − free − heap objects) was 86 to 93 MiB
  for a live heap of 150 to 200 MiB with 192 threads. A limit made from the live heap alone left the heap about
  10 MiB of room.
- With the same build, `GOMAXPROCS=8` (2 runs of the large keystroke rows) and `GOMEMLIMIT=4GiB` (4 runs) had no
  such tails: large keystroke-new p95 2.45-2.69 ms and 2.89-7.17 ms, keystroke-imported 0.94-1.00 and 1.00-1.65,
  against 7.08-9.31 and 2.11-28.6 for the build as it was. These are p95 figures; the gctrace (cycle counts and
  goals) was taken only for the build as it was and for this ADR's.

This does not contradict ADR-125, which kept this limit and rejected a looser one (factor 2.6) on its measurements:
its campaigns ran pinned (`taskset`, `GOMAXPROCS=8`). With 8 threads the overhead is small and the limit leaves the
room it was meant to. The many-thread case had not been measured.

## Decision

1. The limit is `max(256 MiB, 1.4 × live + 64 MiB + overhead)`, where overhead is the runtime's memory that is no
   heap object and is not free (`runtimeOverhead`), at most once more the live heap. That bound keeps the limit
   under what the pacer's own goal needs. The overhead is measured after each collection, with the live heap.
   GOMEMLIMIT, when set, still wins.
2. The memory budget of the benchmarks is gopls's: gocpp's peak resident set on a project is at most gopls's peak
   on the same project in the same report (`bench.MemoryBudgetMet`). It replaces the fixed 400 MiB for a 50k-line
   project (README §2.1, ADR-015) in the tables (`docs/benchmarks/lsp.md`) and in the opt-in gate
   (`TestLSPBudgets`; [ADR-031](031-lsp-speed-gate.md)'s 400 MiB for medium). The gate keeps its memory check but
   makes it relative. It has a verdict only where gopls ran, the small project: the gate skips gopls on medium, so
   medium's memory has no verdict there (accepted).

## Evidence

Matched A/B on the `large` keystroke rows: 3 alternating rounds per binary (`lspbench -targets gocpp -projects
large -runs 3 -only keystroke,keystroke-imported,keystroke-new,keystroke-imported-new`, default GOMAXPROCS of 192).
p95 in ms. The 1-minute load at the end of each round is in brackets; the 5- and 15-minute loads were 1,195-1,664
and 2,783-3,098, so the machine had just come out of an overload.

| row | base (load 18, 21, 13) | this ADR (load 13, 17, 20) |
|---|---|---|
| keystroke | 0.77, 0.69, 0.77 | 0.85, 0.89, 0.66 |
| keystroke-imported | 4.13, 18.6, 14.8 | 1.03, 1.74, 0.88 |
| keystroke-new | 10.4, 6.90, 6.66 | 2.84, 2.99, 2.99 |
| keystroke-imported-new | 5.59, 2.92, 4.66 | 2.75, 2.64, 2.85 |
| resident set at the end, MiB | 338, 351, 353 | 375, 391, 385 |

Whole table, gocpp against gopls on the same run (`lspbench -projects small,medium,large,cpp`, 5 runs; load 14 to 31
before, 0.1 to 3.9 after): no row over 2 times gopls after (three before). Large `keystroke-imported` went from 4.69
to 0.74 and large `keystroke-new` from 2.12 to 1.11. Part of the keystroke-new changes on large and cpp comes from the
harness fix of the same job, not from this ADR: the tree is written again before each run, and before that, runs 2
to 5 had the generic file of the rename rows in the hot package.

## Measurements

gocpp against gopls on the same run, p95 in ms (the median over the runs of each run's p95), 5 runs, default
GOMAXPROCS (192). gopls v0.21.1 with diagnosticsDelay 30 ms. Loads are 1, 5 and 15 minutes, at the start and at the
end of the run.

| run | gocpp built at | date (UTC) | load at start / end |
|---|---|---|---|
| before: whole table | da170a8f (dev and the symbols fix) | 2026-10-23 02:38-02:50 | 14.4 48.9 58.6 / 31.6 52.2 53.3 |
| after: whole table | cb12b41d (this ADR and the harness fix) | 2026-10-23 03:59-04:11 | 0.1 67.4 1097 / 3.9 9.0 531 |
| new rows (`-only`) | 5a9e8fc5 (and the generic rows, rename-method on large) | 2026-10-23 19:21-19:28 | 30.6 393 733 / 25.5 124 485 |

| project / row | before: gocpp / gopls / ratio | after: gocpp / gopls / ratio |
|---|---|---|
| large keystroke-imported | 13.6 / 2.90 / 4.69 | 1.08 / 1.46 / 0.74 |
| large keystroke-new | 14.0 / 6.61 / 2.12 | 2.89 / 2.60 / 1.11 |
| large keystroke-imported-new | 7.75 / 4.72 / 1.64 | 2.76 / 2.64 / 1.05 |
| large keystroke | 2.92 / 2.07 / 1.41 | 0.79 / 1.55 / 0.51 |
| cpp keystroke-new | 9.11 / 3.89 / 2.34 | 2.94 / 3.35 / 0.88 |
| medium completion-edit | 9.88 / 5.78 / 1.71 | 6.44 / 4.50 / 1.43 |
| small completion-edit (the highest ratio after) | 4.46 / 4.64 / 0.96 | 5.44 / 3.37 / 1.62 |
| large memory at the end, MiB (peak) | 356 (391) / 709 (984) | 474 (482) / 795 (1044) |

The other rows of the four projects were at most 2 times gopls in both runs (after: at most 1.52, large completion-edit; cpp completion-edit 1.47). cpp's
keystroke-new change comes from the harness fix: cpp's live heap is under the 256 MiB floor, where the limit did not
change. The generic rows and large rename-method, from the third run (the 5- and 15-minute loads were still high:
every p95 there, gopls's too, is about twice the quiet run's):

| row | small | medium | large | cpp |
|---|---|---|---|---|
| keystroke-generic: gocpp / gopls / ratio | 8.39 / 6.04 / 1.39 | 13.0 / 7.76 / 1.67 | 12.5 / 6.11 / 2.05 | 10.1 / 6.27 / 1.62 |
| completion-generic | 11.1 / 8.21 / 1.35 | 13.8 / 9.19 / 1.51 | 10.3 / 9.73 / 1.06 | 12.5 / 9.56 / 1.31 |
| rename-method | 0.59 / 0.98 / 0.61 | 1.10 / 11.0 / 0.10 | 2.12 / 29.4 / 0.07 | 0.92 / 9.85 / 0.09 |

The generic rows are slower because ADR-115's focused check declines for a package with generics; that is not this
ADR's subject. The large project changed in the same job: its hot file has the method Collect now (20 lines with its comment), so
rename-method can run there. Large rows are therefore not strictly comparable with earlier campaigns.

docs/benchmarks/lsp.md is not regenerated by this job (the machine was overloaded). The next quiet `make lsp-bench`
regenerates it, with the memory budget of this ADR and the new rows.

## Consequences

- Memory: `large` ends the full session at 474 MiB instead of 356 (gopls: 795; peak 482 against 1044). Small, medium
  and cpp did not change measurably (153, 261 and 216 MiB). The user accepted the cost on 2026-10-23, along with
  the budget relative to gopls.
- On a machine with few threads the overhead is small and the limit is about what it was.
- Not done: tuning the factor or the slack again. The limit now leaves the room that ADR-112 meant, whatever the
  thread count.
