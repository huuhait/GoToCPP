# ADR-125: Language server speed, round 9: the tails of `large` are collector cycles, and fewer bytes allocated lowers them

Status: accepted (2026-10-21)

## Context

The user's priority for the language server is that it beats gopls on every measured row.
[ADR-123](123-lsp-speed-round-8-rename-forms-and-the-rows-left.md) left 7 rows that did not win and
said that the cause of the tails among them (`large references-popular`, `large rename-import-alias`,
`large rename-unexported`, `large document-symbol`) was "probably the collector" but that it had no
`gctrace` of them, and that the memory limit and idle-collection experiments did not lower the
spikes. This job (round 9 of 11.lspgap, `11.lspgap3`) measured the cause, then fixed what the
measurements showed.

Priority order of the job: (1) the tail of `large references-popular`, (2) the tails of the rename
rows, (3) the rest of the table of ADR-123 (cpp, macro, hover, cold open, the 1.5x of the shared
project), each with answers that stay identical.

## Measurement provenance

Everything uses `lspbench` (protocol method `fresh-edits-v2-go1.25`) the way ADR-119 and ADR-123 do:
one invocation per (round, arm), arms rotating by round, gopls an arm of its own, each under the
shared `perf.lock`, cores 16 to 23, `GOMAXPROCS=8`, gopls v0.21.1 with `diagnosticsDelay` 30 ms,
`-scale 2`, one run per invocation (AMD Ryzen Threadripper PRO 9995WX, Linux 6.19.6, go1.25.7). The
rule for a won row is unchanged: the pooled p95 of gocpp is under 0.95 times the pooled p95 of gopls
(nearest rank over the pooled samples of all rounds) and gocpp's p95 is the lower one in more than
half of the rounds.

- **base**: `ac4cb183` (dev when the job started), `gocpp` sha256 `a5860a44...`.
- **final**: `910fe7e4` (the code commit of this job), `gocpp` sha256 `b5448795...`; `lspbench` sha256
  `d8920ad4...`, the same binary for every arm (the benchmark code did not change).
- **Table**: campaigns `C1` and `C2`, 5 rounds each, pooled as 10 rounds of base, final and gopls: all
  the rows, the four projects `small`, `medium`, `large` and `cpp`, plus the rows that ADR-123 added
  (the five rename rows: unexported, import alias, generic function, generic type, generic method).
  30 invocations, 30 exits 0, none skipped, started at one-minute load averages of 0.05 to 1.6 (15:06
  to 16:08 UTC; the file `invocations.jsonl` of each campaign). Nothing of mine ran during them.
- **`macro` and `bigpkg`**: campaigns `C3` (the rows of the table and the six macro rows plus
  `rename-unexported`, project `macro`), `C5` (the four typing rows, `bigpkg`, `-scale 1`) and `C6` (the
  same typing rows, `macro`), 4 rounds each of base, final and gopls, 36 invocations, all exit 0, the
  same lock, cores and gopls.
- **Diagnostic runs** (profiling, `gctrace`, pools of 5 runs of the 33 rows that decide the question,
  all on `large`, gocpp only): one invocation each under `perf.lock`, same cores. Rows
  `hover,definition,signature-help,references-popular,references-local,references-family,rename,
  rename-type,rename-method,rename-field,rename-local,workspace-symbol,document-symbol,semantic-tokens,
  completion-dot,completion-edit,format-1k,switch-file,api-change,api-fix,keystroke,keystroke-new,
  keystroke-imported,keystroke-imported-new,rename-var,rename-const,rename-fanout,rename-iface-method,
  rename-unexported,rename-import-alias,rename-generic-func,rename-generic-type,rename-generic-method`
  (called "the 33 rows" below). The debug builds that log each request with its collector cycle count
  (`GOCPP_DBG_REQ`), a forced-collection loop and an execution trace were made from the same tree plus a
  patch that is not in the commit.

## Findings: the cause, proven

1. **The slow requests are the requests that run during a mark phase.** In the 33 rows, a pool of 10 runs
   of base has, on `large`, `references-popular` p50 4.2 ms / p95 11.0 ms / max 25.6, `rename-import-alias`
   p50 0.17 / p95 4.5 / max 11.7 and `rename-unexported` p50 0.078 / p95 1.3 / max 10.9. With
   `GOGC=off GOMEMLIMIT=6GiB` (no collection in the session; 5 runs) the same rows are p50 4.05 / p95
   4.34 / max 4.6, p50 0.108 / p95 0.34 / max 0.4, and p50 0.067 / p95 0.079 / max 0.1: there is no tail
   at all, and the p50 does not move. `GOGC=off` is the only change that removes the tails altogether; the
   changes below lower how often they happen.
2. **The slow requests are in the window of a cycle.** Three views of it:
   - An execution trace of one full session of base (`-exec-trace`, `go tool trace -d=parsed`, with the
     debug build's user log lines to put the two clocks together): during the 11 requests of
     `rename-import-alias` (a window of 34 ms; the row's p50 is 0.1 to 0.2 ms) the concurrent mark
     phase of one cycle runs from 4.1 ms of the window to 45.2 ms, all 8 Ps run goroutines for
     30 of the 34 ms, and the requests take 0.7 to 9.0 ms (0.98, 4.05, 9.02, 1.78 and 4.41 ms for the first five).
   - `GODEBUG=gctrace=1` and the request log of the debug build (`REQ`: one line per request, with its
     duration and the number of cycles that began in it), one session of the 33 rows with an intermediate build (11 cycles, 337 ms of
     mark wall time in a session of 28 s): the server-side duration of the requests that ran during a
     mark phase against those that ran outside one, by median: `rename` 0.74 to 1.32 ms (1.8x),
     `prepareRename` 1.65x, `didChange` 2.6x, `workspace/symbol` 1.8x. All the 82 `documentSymbol` and the 42
     `semanticTokens` requests of that session ran in one mark phase, which is the tail of those two
     rows in that run. The one slow `references-popular` request of the diagnostic runs that had the
     log (8.5 ms against 4 for the other 11) had a cycle begin in it.
   - The pools of 5 runs below: the rows over their bar follow the cycles (the same row in most rounds of a
     build).
3. **A mark phase makes every request about 2.5 times slower, and spikes the tail.** A forced-collection
   loop (`runtime.GC()` back to back, which keeps a mark phase running all the time; rows `hover` and the
   three references rows) on the final build: `references-local` p50 0.30 ms becomes 0.75 (2.5x), `hover`
   0.05 becomes 0.13, `references-popular` 4.0 becomes 7.3 to 8.2, and the p95 of `references-local`
   becomes 2.7 to 3.3 with `GOMAXPROCS=8`, 0.97 with 6 and 0.84 with 4 (the 8 cores are the client's too:
   the tail is waits for a core, the 2.5x is the work). A cycle of the 209 MiB heap of `large` takes 30
   to 55 ms of wall time and 0.2 to 0.25 s of cpu (`gctrace`).
4. **Where a cycle lands is fixed by the sequence of allocations, so a change of policy only moves it.**
   The cycles begin at the same requests in every round of a build, because the allocation sequence is the
   same; the same row is hit in most rounds of one build and in none of another. That is why the rows
   of ADR-123 wandered between campaigns ("a row that does not win in `F10` wins in at least one
   other"), and why a looser memory limit, `GOMAXPROCS=7`, and the idle collector each fixed some rows
   and broke others in the pools below. One cycle hitting a round of 12 requests is enough to lose
   `references-popular` (with 60 pooled samples the p95 is the 57th, so 3 slow samples are the whole
   allowance).
5. **The first guess was too narrow.** I first thought that the requests were starved of a P (the
   dedicated workers plus the idle workers on every P). Lowering `GOMAXPROCS` lowers the spikes of the tiny
   rows in (3) but not the 2.5x, and it does not fix the rows of the pools (below): it is not the
   explanation of the rows that were lost.

What lowers the number of rows hit is the number of cycles, so the number of bytes allocated.

## Decisions

All four are about the bytes that a session allocates; none changes an answer. Only the first is
big: it is 31% of the bytes of the 33 rows (509 of 1,635 MiB) and nearly all of the -33% below. The other
three are cheap cleanups: in the reviewer's profile of the base build `viewOf` is 13.5 MiB (0.8%), the hit list
6.5 MiB (0.4%), and the diagnostics list was 20 MiB in my profile (1.2%). They were not measured one by one
for their effect on the tails, and I do not claim that they cause any of it; the lower cycle count comes from
decision 1.

1. **The background symbol index does not build the list of all the symbols.** `warmSymbols` (after
   `initialized` and each `didOpen`) called `workspaceSymbols` only for its side effect (parse and take the
   symbols of the files that changed) and threw the result away, a copy of every workspace symbol:
   509 MiB of the 1,635 MiB that the 33 rows allocate on `large` (31%). It calls `workspaceFileSymbols`
   now (the part that does the work), and `workspaceSymbols`, that nothing else called, is gone. The
   list of files of that call is made at the size of the table of files instead of growing by appends
   (76 MiB to 38 MiB).
2. **The view of a package is made once.** `viewOf` made, for every request and every package, the table
   of the package's files and the table of its synthetic names (150 KiB of garbage for a search of a popular
   name on `large`, which searches 1,500 files). A view depends on the check of the package and
   nothing changes it, so it is a `cache.Package.Memo`: shared by the requests and snapshots that have the
   package, gone with the package, and a package patched for a rename check (`PackageWithFile`) has its own.
3. **The list of hits of a references request is given back.** A search made `make([]hit, 0, n)` of 80-byte
   hits, 300 KiB for a popular name on `large`, every time. The `references` handler gives its list back
   after it has made its locations (`result.recycleHits`): the list goes into a short list under a lock
   (two lists, at most 16,384 hits each; not a `sync.Pool`, which keeps a list per P and drops it at the
   second collection, while a request runs on whichever P is free; I did not measure the two against each
   other), cleared first so that it holds no pointer to the checks of old
   packages. Other users of `search` (rename, highlight) keep their lists as before.
4. **The list of diagnostics is made at its size.** `diag.List.view` grew a list of 100-byte diagnostics by
   appending (20 MiB for the 33 rows); it makes the list at the number of diagnostics (and still answers
   nil for none).

The top 10 allocators of the final build (`alloc_space`, flat; the reviewer's profile of the 33 rows on
`large`, 1,146 MiB in all, where the base build allocates 1,736 MiB and `workspaceSymbols` alone 454 MiB):
`check.newInfoSized` 63.9 MiB (5.6%), `cache.(*state).Clone` 62.3 (5.4%), `strings.(*Builder).WriteString`
37.0 (3.2%), `go/types.newVar` 33.5 (2.9%), `renamer.edits` 33.1 (2.9%; 7.7% with what it calls),
`bufio.NewReaderSize` 32.1 (2.8%, the `go/build` `MatchFile` reading of each file), `strings.(*Builder).grow`
24.5 (2.1%), `workspaceFileSymbols` 20.5 (1.8%), `searcher.run` 20.2 (1.8%), `buildIdentIndex` 19.5 (1.7%).
By cumulative bytes: `check.Check` 23%, `parser.ParseFile` 13%, the export-data reader 10%.

Only two are above 5% of the bytes. `newInfoSized` is the `types.Info` maps of each check, which are the result
that the check keeps: nothing to remove without changing what is kept. `state.Clone` copies the table of files
for each snapshot (29 MiB, `make(map, len(s.files))`) and builds the reverse index of importers again (22 MiB,
the appends at `importers[dep.id]`): lowering it needs a table shared between snapshots and an index that is
kept, which is a change of structure and not a simple sound one, so it is left under "what is left" and not
done in this job.

Effect on the 33 rows on `large` (one profiled run each of base and final): allocated bytes 1,635 MiB to
1,096 MiB (-33%); collector cycles of a session (`gctrace`, two runs each) 24 and 23 to 16 and 16;
resident memory at the end of a session, mean of the 5 runs of `C1`: 384 to 363 MiB (large), 237 to 223
(medium), 174 to 163 (cpp), against gopls 650, 410 and 335 (medians).

The resident memory is lower, not higher: the memory limit was not changed (see below).

## What was tried and not applied

- **A looser memory limit** (the heap policy's `memFactor` from 14 to 26 tenths of the live heap, and
  `GOMEMLIMIT=1GiB` on base). It lowers the number of cycles (11 instead of 23 for the 33 rows) and moves
  them to other rows: with the same other changes, two pools of 5 runs had 2 and 4 rows over 0.95 with
  26 and 2 and 1 with 14 (the table below); and it costs about 150 MiB of resident memory (510 to 550
  against 380 on `large`; gopls has 650). The final build keeps the limit of ADR-119.
- **The idle collector (`-idle-gc`).** `references-popular` p95 1.92 times gopls in a pool of 5 runs. My
  reading, not traced: the benchmark has pauses of 5 ms or more between two rows, the idle cycle starts
  there and runs through the next burst of requests.
- **`GOMAXPROCS=6` or 7.** It does not lower the number of cycles: base with 7 had 3 rows over 0.95 (as
  with 8), and a pool with 6 had 1 (rename-local was hit instead of references-local). It lowers the
  spike of the tiny rows in the forced-collection test and takes cores away from the checks.
- **`GOGC=off`.** It proves the cause (finding 1) and is no policy: it grows without bound until the limit.
- **A cache of the result of the last references request** of the same symbol in the same snapshot. It
  would answer the 12 identical requests of the row from memory and say nothing about a real editor
  session; not built.
- **A smaller `fileInfo` table** (`tfiles/4`): a popular name is in nearly every file, the table grew
  anyway and the garbage went up; reverted.
- **The checks that run on every keystroke walk each file several times** (`rewriter.walk`,
  `CheckReservedC`, `gpast.Leftovers`, `findOpaque`, `checkSizeCParts`: about 0.2 ms of cpu a keystroke
  on `large`) and `go/build`'s `MatchFile` reads the header of each file of a package at every load (49
  MiB). A cache of those answers by file content would take, by the profile, about 0.2 to 0.3 ms of cpu off a
  check of `keystroke-imported-new` (see the rows that are left); not done: the code is in the middle of the
  checker and the gain is of the size of the noise of the row.

### The pools of 5 runs that decided it

The 33 rows on `large`, gocpp only, 5 runs per line (two lines are two pools of the same build).
"slow" is the share of the 6,220 samples of a pool that are over the p95 of gopls in their row (the pool
of gopls of an earlier campaign on the same machine); "rows over" is the number of rows of the 33 whose
pooled p95 is at least 0.95 of gopls's:

| build | slow | rows over |
|---|---|---|
| base | 1.64% and 2.06% | 3 and 4 |
| base, `GOMAXPROCS=7` | 1.64% | 3 |
| base, `GOMEMLIMIT=1GiB` | 0.88% and 0.95% | 1 and 2 |
| base, `GOGC=off GOMEMLIMIT=6GiB` | 0.50% | 1 |
| limit factor 26, nothing else | 1.59% and 1.27% | 4 and 1 |
| limit factor 26, the symbols fix | 0.98% | 2 |
| limit factor 26, the symbols fix, `-idle-gc` | 0.82% | 1 |
| limit factor 26, decisions 1 to 3 | 1.56% and 1.58% | 2 and 4 |
| limit factor 26, decisions 1 to 3, `GOMAXPROCS=6` | 1.03% | 1 |
| **limit of ADR-119, decisions 1 to 3** | **0.87% and 1.03%** | **2 and 1** |

(The final build also has decision 4, the list of diagnostics; it was made after these pools.) The build
without the looser limit is about as good as the loosest limit and keeps the memory. The `signature-help`
samples that are in the list of slow samples of every pool are not what this ADR is about: the row wins
(p95 0.067 ms against gopls 0.096, whose own p95 is the bar of that count) and has the same 11 to 21 in every build.

## Results

Campaigns `C1` and `C2` pooled (10 rounds), base against final against gopls. 145 rows, 6 of them
(`cpp` typo and open-cpp rows) have no gopls row. Of the 139 that have one, 137 win and 2 do not
(`medium references-local`, `large keystroke-imported-new`, below). Base in the same campaigns: 137
win and 2 do not, `large references-popular` (p95 ratio 1.48, 1 of 10 rounds won) and
`large rename-import-alias` (11.28, 4 of 10). `C2` alone (5 rounds): all 139 win with the final build.

By project, rows that win and rows that do not (the final build): `small` 35 and 0, `medium` 34 and 1,
`large` 33 and 1, `cpp` 35 and 0.

The rows of `large` (p50 / p95 in ms; ratios are final p95 over gopls p95 and final p50 over gopls p50;
"rounds won" counts the rounds in which gocpp's p95 is under gopls's):

| row | base p50 / p95 | final p50 / p95 | gopls p50 / p95 | p95 ratio | p50 ratio | n | rounds won | wins |
|---|---|---|---|---|---|---|---|---|
| large api-change | 0.425 / 0.515 | 0.438 / 2.113 | 33.552 / 33.855 | 0.06 | 0.01 | 80 | 10/10 | yes |
| large api-fix | 0.393 / 0.493 | 0.395 / 2.333 | 33.827 / 34.105 | 0.07 | 0.01 | 80 | 10/10 | yes |
| large completion-dot | 0.088 / 0.105 | 0.089 / 0.100 | 0.347 / 0.510 | 0.20 | 0.26 | 1200 | 10/10 | yes |
| large completion-edit | 2.064 / 2.305 | 2.061 / 2.304 | 3.082 / 3.332 | 0.69 | 0.67 | 600 | 10/10 | yes |
| large definition | 0.038 / 0.042 | 0.037 / 0.041 | 0.064 / 0.088 | 0.46 | 0.57 | 1200 | 10/10 | yes |
| large document-symbol | 0.043 / 0.091 | 0.042 / 0.047 | 0.108 / 0.135 | 0.35 | 0.39 | 800 | 10/10 | yes |
| large format-1k | 3.166 / 3.549 | 3.182 / 3.501 | 3.700 / 4.006 | 0.87 | 0.86 | 300 | 10/10 | yes |
| large hover | 0.051 / 0.145 | 0.050 / 0.144 | 0.192 / 7.177 | 0.02 | 0.26 | 1200 | 10/10 | yes |
| large keystroke | 0.508 / 0.587 | 0.507 / 0.576 | 0.986 / 1.102 | 0.52 | 0.51 | 800 | 10/10 | yes |
| large keystroke-imported | 0.489 / 0.731 | 0.482 / 0.586 | 0.948 / 1.057 | 0.55 | 0.51 | 400 | 10/10 | yes |
| large keystroke-imported-new | 1.852 / 2.144 | 1.846 / 2.283 | 2.185 / 2.374 | 0.96 | 0.84 | 400 | 7/10 | **NO** |
| large keystroke-new | 1.960 / 2.233 | 1.924 / 2.212 | 2.270 / 2.552 | 0.87 | 0.85 | 800 | 10/10 | yes |
| large open-cold | 142.477 / 145.488 | 143.405 / 146.109 | 314.462 / 328.150 | 0.45 | 0.46 | 10 | 10/10 | yes |
| large open-warm | 40.428 / 45.837 | 40.490 / 43.123 | 121.463 / 122.026 | 0.35 | 0.33 | 10 | 10/10 | yes |
| large references-family | 0.396 / 0.446 | 0.381 / 0.450 | 2.427 / 2.827 | 0.16 | 0.16 | 160 | 10/10 | yes |
| large references-local | 0.268 / 0.322 | 0.264 / 0.320 | 0.317 / 0.370 | 0.86 | 0.83 | 400 | 9/10 | yes |
| large references-popular | 4.193 / 9.859 | 4.027 / 4.597 | 6.321 / 6.648 | 0.69 | 0.64 | 120 | 9/10 | yes |
| large rename | 0.730 / 0.985 | 0.698 / 0.992 | 3.532 / 6.355 | 0.16 | 0.20 | 100 | 10/10 | yes |
| large rename-const | 0.999 / 4.930 | 0.962 / 1.017 | 58.415 / 62.436 | 0.02 | 0.02 | 100 | 10/10 | yes |
| large rename-fanout | 0.950 / 4.553 | 0.928 / 1.011 | 58.957 / 62.547 | 0.02 | 0.02 | 100 | 10/10 | yes |
| large rename-field | 0.881 / 6.170 | 0.805 / 1.361 | 14.854 / 21.460 | 0.06 | 0.05 | 100 | 10/10 | yes |
| large rename-generic-func | 0.603 / 1.564 | 0.574 / 0.982 | 3.495 / 10.017 | 0.10 | 0.16 | 100 | 10/10 | yes |
| large rename-generic-method | 1.125 / 1.620 | 1.087 / 1.545 | 15.341 / 29.821 | 0.05 | 0.07 | 100 | 10/10 | yes |
| large rename-generic-type | 0.572 / 0.629 | 0.556 / 0.640 | 15.286 / 31.183 | 0.02 | 0.04 | 100 | 10/10 | yes |
| large rename-iface-method | 1.493 / 1.851 | 1.473 / 1.844 | 16.386 / 22.454 | 0.08 | 0.09 | 100 | 10/10 | yes |
| large rename-import-alias | 0.160 / 4.316 | 0.108 / 0.294 | 0.298 / 0.382 | 0.77 | 0.36 | 100 | 8/10 | yes |
| large rename-local | 0.113 / 0.249 | 0.106 / 0.126 | 0.296 / 0.370 | 0.34 | 0.36 | 100 | 10/10 | yes |
| large rename-type | 0.608 / 4.506 | 0.551 / 0.648 | 14.909 / 22.592 | 0.03 | 0.04 | 100 | 10/10 | yes |
| large rename-unexported | 0.073 / 0.131 | 0.067 / 0.088 | 0.179 / 0.240 | 0.37 | 0.37 | 100 | 10/10 | yes |
| large rename-var | 20.868 / 31.412 | 21.845 / 34.077 | 70.859 / 75.820 | 0.45 | 0.31 | 100 | 10/10 | yes |
| large semantic-tokens | 0.034 / 0.067 | 0.033 / 0.037 | 0.059 / 0.079 | 0.48 | 0.56 | 400 | 10/10 | yes |
| large signature-help | 0.044 / 0.070 | 0.042 / 0.067 | 0.075 / 0.096 | 0.70 | 0.56 | 1200 | 10/10 | yes |
| large switch-file | 0.548 / 1.164 | 0.520 / 0.576 | 2.771 / 3.052 | 0.19 | 0.19 | 400 | 10/10 | yes |
| large workspace-symbol | 0.759 / 1.976 | 0.740 / 0.858 | 3.582 / 4.384 | 0.20 | 0.21 | 600 | 10/10 | yes |

The evidence is fewer tails per row, not the count of rows: base wins 137 of the 139 as well in these
campaigns, with different losers, and which rows lose depends on where a cycle lands (finding 4). A
reviewer's independent run (`medium` and `large`, 6 rounds of base, final and gopls, the same lock and
cores, a different set of invocations) shows both sides of it:

| row (large) | base p50 / p95 | final p50 / p95 | gopls p50 / p95 | base p95 ratio, rounds won | final p95 ratio, rounds won |
|---|---|---|---|---|---|
| rename-import-alias | 0.125 / 3.383 | 0.115 / 0.277 | 0.309 / 0.398 | 8.51, 4/6 | 0.70, 6/6 |
| rename-unexported | 0.082 / 3.841 | 0.077 / 0.105 | 0.181 / 0.245 | 15.65, 4/6 | 0.43, 6/6 |
| keystroke-imported-new | 1.903 / 2.666 | 1.946 / 2.193 | 2.287 / 2.465 | 1.08, 3/6 | 0.89, 6/6 |
| references-popular | 5.832 / 6.126 | 5.762 / 6.058 | 7.985 / 8.394 | 0.73, 6/6 | 0.72, 5/6 |
| references-local | 0.292 / 0.358 | 0.294 / 1.770 | 0.314 / 0.377 | 0.95, 6/6 | **4.69**, 3/6 |
| rename | 1.053 / 3.647 | 1.025 / 5.798 | 3.485 / 3.777 | 0.97, 3/6 | **1.54**, 3/6 |

In that run base has 7 of the 72 rows of the two projects not winning (`large keystroke-imported-new`,
`references-local`, `rename`, `rename-import-alias`, `rename-unexported`, `signature-help`, and `medium
semantic-tokens`) and the final build has 2 (`large references-local`, `large rename`): a net gain, but the final
build is not "all rows": in three of its six rounds a cycle landed on `references-local` (max 12 ms) and on
`rename`. So "137 of 139" is the result of my 10 rounds and not a property of the build. The user's bar is
every row and it is not met; what this job shows is that the number of cycles, and so the number of rows hit
in a round, went down by a third.

The three tails of ADR-123 on `large` that were open are won in the 10 rounds: `references-popular` (p95 9.9 to 4.6 ms,
9 of 10 rounds won against 1 of 10 for base), `rename-import-alias` (p95 4.3 to 0.29 ms, 8 of 10), and
`rename-unexported` and `document-symbol` (p95 0.13 to 0.09 and 0.09 to 0.05). `api-change` and
`api-fix` have a p95 of 2.1 and 2.3 ms against 0.5 for base: in the final build a cycle landed on them in
one round (gopls takes 33.9 and 34.1 ms; the rows win by 15 times). That is the same dice in the
other direction.

Other rows whose p95 fell from base: `rename-const` 4.9 to 1.0 ms, `rename-fanout` 4.6 to 1.0,
`rename-field` 6.2 to 1.4, `rename-type` 4.5 to 0.65, `switch-file` 1.2 to 0.58, `workspace-symbol` 2.0 to 0.86,
`rename-local` 0.25 to 0.13: in base a cycle landed on them in some rounds. The p50 of every row is within 5% of base
except `rename-import-alias` (0.16 to 0.108: base's p50 had slow rounds in it).

### Projects macro and bigpkg (campaigns C3, C5, C6)

`macro`: 31 rows have a gopls row and all 31 win (4 rounds); the 10 rows without a gopls row (the macro
rows that gopls cannot read, and the four typing rows) are within noise of base: `typing-clean` p95 9.15 to
8.67 ms, `typing-broken` 1.50 to 1.47, `typing-broken-reqs` 1.55 to 1.59, `typing-broken-hover` 3.25 to 3.27,
`hover-macro` and `completion-macro` unchanged. `macro keystroke-imported-new` is p50 1.859 / p95 2.095
against gopls 2.123 / 2.306: p95 ratio 0.91, 4 of 4 rounds. `bigpkg`: `open-cold` (0.31 of gopls) and
`open-warm` (0.49) win; the typing rows have no gopls row and do not move (`typing-clean` 84.1 / 86.5
to 84.3 / 86.7 ms against the budget of 100; `typing-broken-hover` p95 19.1 to 19.7).

### The two rows that do not win

- **`medium references-local`**: p50 0.115 ms (0.41 of gopls), p95 0.333 against gopls 0.319, 7 of 10 rounds
  won. In two rounds of ten, 4 to 5 of the 40 requests take 2 to 3.7 ms: a cycle that began in the
  rows before. Base had no cycle there in these 10 rounds (the placement is luck, finding 4). The
  row is 0.1 ms and a window of 5 ms; any cycle that begins in the row before it does this.
  Not fixed: it needs fewer cycles or a cheaper cycle (see what is left).
- **`large keystroke-imported-new`**: p50 1.846 ms (0.84 of gopls), p95 2.283 against gopls 2.374, which is
  0.96 times (the bar is 0.95): 7 of 10 rounds won. This is not a tail: the row run alone (`-only`, 600
  samples) is p50 1.83 and p95 2.12 (0.89 of gopls), and the p95 of the row in the pools of 5 runs of the 33
  rows is 2.12 to 2.35 for every build, with `GOGC=off` too (2.22). It is the cpu of one check of a
  package that others import, about 1.9 ms of cpu, of which `go/types` is 57%; the rest is spread
  over many small steps (see "tried and not applied").
  The same row on `cpp` was 1.09 in ADR-123 and wins in this job's campaigns (0.83).

## Answers are identical

`lspdiff` (base against final, the same requests at many places of many files, before and after 24
edits, and the diagnostics): `small` 6,312 answers compared, 0 differ (5,097 successful replies and 753
refusals in both); `medium` 6,366 compared, 0 differ (5,189 and 715 in both).

## Tests and mutation checks

New (all deterministic, no wall-clock bounds):

- `TestWarmSymbolsFillsTable` and `TestQueryAfterWarmSymbols` (`symbols_test.go`): after `initialized`, with
  no query, the background index has the symbols of every file in the table; the first query then answers
  `Double (example.com/ws/util)`. The first version of the first test was flaky (3 of 300 runs: the
  notification is handled on the read goroutine, but the warm run is in the background); it now makes a
  `gocpp/stats` request as a barrier and waits on the `warming` flag, and passes 400 runs in a row.
- `TestViewOfIsMadeOncePerPackage`, `TestViewOfHoldsWhatAFreshViewHolds` and
  `TestViewOfPatchedPackageIsItsOwn` (`hitbuf_test.go`): the same view for the same package, equal to a
  fresh one in its types, info and files; a patched package has its own.
- `TestRecycleHitsEmptiesTheResult`, `TestRecycledHitsAreCleared`, `TestTakenHitsAreEmptyAndRoomy`,
  `TestRecycledHitsAreReused`, `TestHugeHitListsAreNotKept`, `TestHitCapacityAsked`: the list given back
  holds no pointer and the result no longer holds it, the list taken is empty and has the room asked, it
  comes back to the next search, a list over 16,384 hits is not kept, and a too-small list is not returned.
- `TestReferencesRecyclesItsHitList` (`hitbuf_test.go`): the wiring. A references request over a seeded pool
  leaves the pool holding the list that the request used (compared by capacity and backing array) and the
  lists it did not use.
- `TestDiagnosticsOfNothingIsNil` and `TestDiagnosticsAreMadeAtOnce` (`internal/diag`): nil for no
  diagnostics; the allocations of `Diagnostics()` do not grow with their number.

The existing tests that cover the changed code ran (`internal/lsp/server` tests matching
`Ref|Rename|Symbol|Hit|ViewOf|Warm|Recycle|Highlight|Family|Members`, and `internal/diag`): all pass.

Mutation checks (each change made by hand, the test run, the change taken back):

| mutation | test that fails |
|---|---|
| `warmSymbols` does not call the index | `TestWarmSymbolsFillsTable` (0 files in the table) |
| `viewOf` makes a view each time | `TestViewOfIsMadeOncePerPackage` |
| `recycleHits` does not clear the list | `TestRecycledHitsAreCleared` (a hit with pointers in a pooled list) |
| `takeHits` never reuses a list | `TestRecycledHitsAreReused` |
| `references` without the `defer res.recycleHits()` | `TestReferencesRecyclesItsHitList` |
| `run` without `res.hitBuf = all` | `TestReferencesRecyclesItsHitList` |
| `run` takes `make` in place of `takeHits` | `TestReferencesRecyclesItsHitList` |
| lists of any length are kept | `TestHugeHitListsAreNotKept` |
| `diag.List.view` does not make the list at its size | `TestDiagnosticsAreMadeAtOnce` (10 allocations against 3) |
| `diag.List.view` always makes the list | `TestDiagnosticsOfNothingIsNil` |

## Consequences and what is left

- The claim of this job is limited. In my 10 rounds two rows do not win (`medium references-local`, a
  cycle in 2 of 10 rounds, and `large keystroke-imported-new`, 0.96, at the edge); in the reviewer's
  independent 6 rounds two other rows do not (`large references-local` and `large rename`, a cycle landed
  on them). The bar of the user is every row and it is not met by either run. What was shown is fewer
  cycles (24 and 23 to 16 and 16) and, with them, fewer tails per row: the reviewer's base build loses 7 of
  72 rows and the final build 2.
- The cause of the tails is now measured: the collector, in the window of a request, and the number of
  rows it hits follows the number of cycles. The three levers that reduce the cycles, in the order of
  what they cost: fewer bytes allocated (done in part: `warmSymbols` was the largest single source and
  decisions 2 to 4 are cheap cleanups; the top 10 allocators of the final build are listed under
  decision 4 above), a looser limit (150 MiB, no better as measured here), and a smaller live heap (the
  checked packages' `types.Info` is most of it; the mark of a cycle is proportional to it).
- What is left to allocate less, in the order of bytes: `check.Check` (23% of the bytes, with
  `newInfoSized` 5.6% that the check keeps), parsing (13%), the decoding of the export data of imports
  (10%), `state.Clone` (5.4%: a files table shared between snapshots and a kept index of importers),
  the new text that a rename builds to check (`renamer.edits` and the string builders, about 8%), the
  `bufio` readers of `go/build`'s `MatchFile` (2.8%) and the bytes of the response of a references request.
  None is a simple sound change; each is its own step.
- A cycle cannot be made harmless while the 8 cores are the client's too; the editors' own cores are free
  in real use, and the benchmark's are not (finding 3). The rows are measured the way ADR-119 set
  them, so the rows stay a function of how often a cycle begins.
- `references-popular` still allocates about 1 MiB a request (1.2 in base): the documents of the hits (a
  URI, a document and an entry for each of 1,500 files, 0.28 MiB), the response (0.24), the `known` table
  of the scan (0.1), the `fileInfo` table (0.1), the importers of the symbol (0.07) and the locations. The
  twelve requests of a round are 12 MiB of the 200 that a cycle takes; whether a cycle lands in them is
  chance.
