# ADR-127: Language server speed, round 10: a cloned state shares its tables, and references and rename allocate less

Status: accepted (2026-10-21)

## Context

The user's priority for the language server is that it beats gopls on every measured row (the
contract allows 1.5 times gopls; the aim is to win). [ADR-125](125-lsp-speed-round-9-the-tails-of-large-are-collector-cycles-and-fewer-bytes-allocated-lowers-them.md)
proved that the tails of `large` are the collector's cycles and that fewer bytes allocated means fewer
cycles; its reviewer's independent run of 6 rounds still lost `large references-local` (p95 1.77 ms
against 0.377), `large rename` (5.8 against 3.78) and `large keystroke-imported-new` (0.96 against 0.95).
This job (round 10 of 11.lspgap, `11.lspgap4`) went on with the cause ADR-125 named: allocate less in
the requests whose rows lost, and in the snapshot clone that every edit pays.

## Measurement provenance

Everything uses `lspbench` the way ADR-119, ADR-123 and ADR-125 do: one invocation per (round, arm),
arms rotating by round, gopls an arm of its own, each under the shared `perf.lock`, cores 16 to 23,
`GOMAXPROCS=8`, gopls v0.21.1 with `diagnosticsDelay` 30 ms, `-scale 2`, one run per invocation (AMD
Ryzen Threadripper PRO 9995WX, Linux 6.19.6, go1.25). The rule for a won row is unchanged: the pooled p95
of gocpp is under 0.95 times the pooled p95 of gopls (nearest rank over the pooled samples of all rounds)
and gocpp's p95 is the lower one in more than half of the rounds.

- **base**: `defa4df8` (dev when the job started), `gocpp` sha256 `70afaa29...`.
- **new**: `f70595a8` (the last code commit of this job), `gocpp` sha256 `7f886af1...`. The same `lspbench`
  binary (built from the new tree; the benchmark code did not change) drove every arm.
- **Campaigns**: `C1`, `C2` (5 rounds each) and `C3` (6 rounds): all the rows of the table (the 33 rows of
  ADR-125), the four projects `small`, `medium`, `large` and `cpp`, arms base, new and gopls. 48 invocations
  planned, 4 skipped by the load gate (`C1` round 5 gopls, `C2` round 1 base and new, `C2` round 5 base).
- **The machine was not idle.** The earlier campaigns of this series ran at one-minute load averages of 0.05
  to 1.6. Other jobs ran their test sweeps (unpinned, up to 5,000% cpu) during these: the load at the END of
  each invocation (`invocations.jsonl` is written after `lspbench` exits, so it includes the benchmark's own load)
  was 0.8 to 88. So the table below
  pools only the rounds in which all three arms ran and the load average at the end of each was under 40:
  12 of the 16 rounds (`C1` rounds 1 to 3, `C2` rounds 2 to 4, `C3` rounds 1 to 6). The rule was made after
  seeing that rounds 4 and 5 of `C1` (loads 45 to 73) were slow for every arm, and before looking at which
  rows it changes. The same table with all 13 complete rounds, and with the 8 rounds under load 25, is
  given below as a check of that choice.
- **Diagnostic runs** (profiles, `gctrace`, the debug build that logs each request with its collector cycle
  count, `GOCPP_DBG_REQ`), all on `large`, gocpp only, the 28 rows `hover` to `rename-iface-method` of
  ADR-125 (called "the 28 rows"): one invocation each, same lock and cores. The debug code and the
  experiment patches are not in the commits.
- **Answers**: `tools/lspdiff` of base against new, on `small`, `medium` and `large`: 6,312, 6,366 and
  6,753 answers compared, 0 differ (the same replies and refusals in both).

## Findings

1. **What a mark phase costs a request is the assist, not the phase.** A mark phase with nothing
   allocating barely slows a tiny request: `definition` has a maximum of 0.06 ms in a whole cycle. The cost is
   the mutator assist: the pacer leaves a runway of about 30 MiB between the trigger and the heap goal for the
   about 200 MiB of live data to scan, so a request that allocates pays about 5 bytes of scan work for each byte
   it allocates. A request inside a cycle takes about its normal time plus (bytes allocated) times that ratio:
   `rename` allocated 0.5 MiB per request (the 1.5 MiB of the first profile is the average over all the rename rows), so about 10 ms in a cycle (its p50 is 0.7 ms); `references-local`
   0.25 MiB, 1 to 4 ms; the first `references-popular` request 16 MiB, 24 to 28 ms. So the way to lower the
   tails is to allocate less *in those requests* (this lowers the penalty of a cycle that lands on them) and
   less overall (fewer cycles).
2. **Where a cycle lands is fixed by the allocation sequence** (ADR-125, finding 4), and it is the same
   in the runs of a build: cycle 12 of base lands on the `references` rows in 4 of 4 runs. A change that
   moves cycles is a throw of the dice. In the final build a session of the 28 rows has 12 cycles (15 in
   base); cycle 11 lands on `references-popular` in 3 of 4 runs (its first request, 16 MiB, took 26 ms in
   two of them and 4 ms in one) and cycle 12 on `format-1k` or on `rename-var`.
3. **The snapshot clone was 5.4% of the bytes.** `state.Clone` copied the table of files, the table of
   folders, the modules and the handles of the previous state for every edit (61 MiB in the 28 rows: files 27,
   the reverse index of importers 19.5, handles 7, modules 5, folders 2).
4. **The `references` and `rename` requests rebuilt per request what depends on the snapshot alone**: the
   packages of the module with their imports and the reverse index (1,500 packages for `large`), the table of
   files at the size of the files of the views (instead of the number of hits the names allow), and (rename) a
   copy of the text of every edited file for the check of the result even when a proof stands in for the check.

## Decisions

All are about the bytes a session allocates; none changes an answer (lspdiff above).

1. **A cloned state shares its tables** (`internal/lsp/cache/layered.go`). The tables of files, folders,
   modules and handles are a `layered` map: an immutable `base` shared by all the states that descend from
   it, the keys the state `hidden`s from it, and a small mutable `own`. `clone(drop)` shares the base,
   copies only the own entries and the hidden keys, and makes a new base (`flatten`) when own plus hidden
   pass 32 plus a quarter of the base. The handles are cloned by `cloneHandles`, one pass over the
   handles in the order they were stored (`state.order`), repeated while a handle with a cut edge changes:
   the importers index that was built from scratch for every edit (19.5 MiB in the 28 rows) is gone. A
   released state's tables are nil (the methods answer for nil), as before the state had none.
2. **A snapshot has a memo** (`Snapshot.Memo`, errors not kept): a value that depends on the snapshot alone
   is made once. `searcher.moduleImports` (packages, imports, reverse index, path and folder maps of the
   module) and the list of the symbols of all the files (`workspaceFileSymbols`, not kept when a file did
   not parse) use it.
3. **The table of files of a search is made at the size of the hit capacity** (`fileInfoSize`:
   at most the files of the views and at most the hits the names allow).
4. **A rename makes the text of an edited file only when the check needs it** (`fileEdits.newText`, from
   byte offsets sorted once, into a `strings.Builder` grown to the size; the quadratic second loop of `edits`
   is gone). The proofs of ADR-119 and ADR-123 return before it. The rename gives its list of hits back to
   the pool as `references` does (`recycleHits`).
5. **The warning "not renamed in" does not read a folder that has all its files** (`SourceFileCount`: the
   files of the folder minus its tests equal the files of the view).

Effect on the 28 rows on `large` (one profiled run each): bytes allocated 1,136 MiB to 860 MiB (-24%) in the profiled run; the review measured the 28 rows three times alternating and got 978 to 1,021 MiB for base against 836 for new, so about -17%;
collector cycles of a session (`gctrace`, 3 runs each) 15 to 12; per request, `rename` (all rename rows averaged) 1.5 to 0.5 MiB, the `rename` row alone 0.54 to 0.19 MiB,
`references-local` 0.25 to 0.07 MiB, `references-popular` 0.7 to 0.8 MiB after its first request, which allocates 16
MiB; resident memory at the end of a session
(median, table below) 362 to 363 MiB on `large`, 223 to 219 on `medium`, 161 to 158 on `cpp`, 144 on `small`
(gopls: 688, 511, 339 and 339).

## What was tried and not applied

- **The collector's policy.** The idle collector (`idle-gc` with a quiet time of 5 ms or 100 ms and growth
  of 3 or 5), a collection before the diagnostic runs, a collection after the quick publish: each moved
  the cycles to other rows and none lowered the share of slow samples (2.07%, 1.76%, 1.09% and 1.30%
  against 0.84% to 1.38% for base, 28 rows). The reason for the idle collector is the protocol: there are
  no quiet gaps (a debounced run is pending or the next request is imminent), as ADR-125 read it.
  A looser limit (`GOMEMLIMIT=2GiB`, the pacer's own goal of twice the live heap) gives 9 cycles instead
  of 12, costs 40 MiB (384 against 340 on `large`; gopls has 688) and lowers the cost of a cycle that lands
  on `references-popular` from 26 to 4 ms. In four alternating pairs of runs its share of slow samples
  (0.48%) was the same as the default's (0.50%), and ADR-125 measured the same trade for a factor of 26
  tenths. Not applied: no measured gain on the rows, and the memory is the contract's (400 MiB for 50,000
  lines). It is the first thing to try when the machine is quiet enough to measure 1% differences.
- **A cache of the header reads of `go/build`'s `MatchFile`** (`bufio.NewReaderSize`, 32 MiB of the 860,
  3.8%): a key of the file name alone is unsound (the header errors depend on the content); a key of the
  content hash is sound and would take 0.2 to 0.3 ms of cpu off a check by the profile of ADR-125. Not
  done; it is the first candidate for `keystroke-imported-new`.
- **`newInfoSized`** (56 MiB, 6.5%): the maps are the check's result, sized by the calibrated hints
  (the filling of the types map is 1,747 of 1,730 and so on). Nothing to remove.

## Results

Pooled over the 12 rounds in which all three arms ran at a load under 40, the rule of ADR-125 for a won
row (p95 under 0.95 of gopls, and lower in more than half of the rounds). p50 / p95 in milliseconds. The
rows `open-*` and `typo-*` are not in the table (their budgets are checked by the contract tests).
`large rename-method` has no samples in any arm (the scenario does not run on that project; in base and
gopls too).


### small (33 of 33 rows won)

| row | base p50 / p95 ms | new p50 / p95 ms | gopls p50 / p95 ms | base/gopls p95 | new/gopls p95 | rounds new p95 < gopls | verdict |
|---|---|---|---|---|---|---|---|
| api-change | 0.218 / 0.319 | 0.198 / 0.283 | 32.376 / 32.704 | 0.01 | 0.01 | 12/12 | win |
| api-fix | 0.195 / 0.331 | 0.182 / 0.247 | 32.451 / 32.692 | 0.01 | 0.01 | 12/12 | win |
| completion-dot | 0.097 / 0.154 | 0.096 / 0.118 | 0.400 / 0.831 | 0.19 | 0.14 | 12/12 | win |
| completion-edit | 2.161 / 3.287 | 2.140 / 2.862 | 2.917 / 5.846 | 0.56 | 0.49 | 11/12 | win |
| definition | 0.043 / 0.049 | 0.043 / 0.073 | 0.068 / 0.224 | 0.22 | 0.32 | 11/12 | win |
| document-symbol | 0.060 / 0.081 | 0.060 / 0.093 | 0.152 / 0.326 | 0.25 | 0.29 | 12/12 | win |
| format-1k | 3.116 / 5.144 | 3.085 / 4.861 | 3.822 / 6.365 | 0.81 | 0.76 | 10/12 | win |
| hover | 0.050 / 0.141 | 0.050 / 0.143 | 0.228 / 8.118 | 0.02 | 0.02 | 12/12 | win |
| keystroke | 0.271 / 0.497 | 0.261 / 0.418 | 0.871 / 1.126 | 0.44 | 0.37 | 12/12 | win |
| keystroke-imported | 0.251 / 0.524 | 0.250 / 0.392 | 0.851 / 1.109 | 0.47 | 0.35 | 12/12 | win |
| keystroke-imported-new | 1.729 / 2.222 | 1.719 / 2.247 | 2.187 / 3.754 | 0.59 | 0.60 | 11/12 | win |
| keystroke-new | 1.871 / 3.244 | 1.829 / 2.229 | 2.157 / 2.864 | 1.13 | 0.78 | 11/12 | win |
| references-family | 0.087 / 0.115 | 0.071 / 0.116 | 0.409 / 1.170 | 0.10 | 0.10 | 12/12 | win |
| references-local | 0.066 / 0.090 | 0.052 / 0.084 | 0.234 / 0.703 | 0.13 | 0.12 | 12/12 | win |
| references-popular | 0.108 / 0.147 | 0.089 / 0.140 | 0.307 / 0.771 | 0.19 | 0.18 | 12/12 | win |
| rename | 0.138 / 0.177 | 0.110 / 0.161 | 0.470 / 1.005 | 0.18 | 0.16 | 12/12 | win |
| rename-const | 0.107 / 0.138 | 0.078 / 0.112 | 0.569 / 1.334 | 0.10 | 0.08 | 12/12 | win |
| rename-fanout | 0.172 / 0.232 | 0.126 / 0.164 | 0.675 / 1.499 | 0.15 | 0.11 | 12/12 | win |
| rename-field | 0.114 / 0.155 | 0.091 / 0.122 | 0.400 / 0.870 | 0.18 | 0.14 | 12/12 | win |
| rename-generic-func | 0.136 / 0.175 | 0.089 / 0.142 | 0.584 / 4.145 | 0.04 | 0.03 | 12/12 | win |
| rename-generic-method | 0.215 / 0.324 | 0.172 / 0.243 | 0.737 / 0.942 | 0.34 | 0.26 | 12/12 | win |
| rename-generic-type | 0.131 / 0.213 | 0.099 / 0.142 | 0.483 / 0.604 | 0.35 | 0.23 | 12/12 | win |
| rename-iface-method | 0.549 / 0.621 | 0.524 / 0.606 | 0.811 / 1.658 | 0.37 | 0.37 | 12/12 | win |
| rename-import-alias | 0.095 / 0.211 | 0.086 / 0.189 | 0.294 / 0.390 | 0.54 | 0.48 | 12/12 | win |
| rename-local | 0.100 / 0.115 | 0.097 / 0.127 | 0.280 / 0.734 | 0.16 | 0.17 | 12/12 | win |
| rename-method | 0.234 / 0.272 | 0.201 / 0.270 | 0.737 / 1.799 | 0.15 | 0.15 | 12/12 | win |
| rename-type | 0.139 / 0.172 | 0.114 / 0.153 | 0.511 / 1.011 | 0.17 | 0.15 | 12/12 | win |
| rename-unexported | 0.066 / 0.113 | 0.063 / 0.082 | 0.176 / 0.237 | 0.47 | 0.35 | 12/12 | win |
| rename-var | 0.195 / 0.266 | 0.143 / 0.202 | 0.729 / 1.035 | 0.26 | 0.20 | 12/12 | win |
| semantic-tokens | 0.048 / 0.056 | 0.047 / 0.054 | 0.064 / 0.120 | 0.47 | 0.45 | 12/12 | win |
| signature-help | 0.046 / 0.062 | 0.047 / 0.081 | 0.078 / 0.132 | 0.47 | 0.61 | 12/12 | win |
| switch-file | 0.418 / 1.019 | 0.467 / 1.001 | 1.408 / 2.598 | 0.39 | 0.39 | 12/12 | win |
| workspace-symbol | 0.125 / 0.168 | 0.032 / 0.074 | 1.670 / 3.822 | 0.04 | 0.02 | 12/12 | win |

### medium (32 of 33 rows won)

| row | base p50 / p95 ms | new p50 / p95 ms | gopls p50 / p95 ms | base/gopls p95 | new/gopls p95 | rounds new p95 < gopls | verdict |
|---|---|---|---|---|---|---|---|
| api-change | 0.281 / 0.388 | 0.272 / 1.219 | 32.772 / 33.369 | 0.01 | 0.04 | 12/12 | win |
| api-fix | 0.250 / 0.335 | 0.244 / 0.420 | 32.897 / 33.617 | 0.01 | 0.01 | 12/12 | win |
| completion-dot | 0.102 / 0.163 | 0.102 / 0.145 | 0.394 / 0.565 | 0.29 | 0.26 | 11/12 | win |
| completion-edit | 2.675 / 5.084 | 2.666 / 5.137 | 3.793 / 5.502 | 0.92 | 0.93 | 8/12 | win |
| definition | 0.044 / 0.056 | 0.044 / 0.052 | 0.063 / 0.089 | 0.63 | 0.59 | 12/12 | win |
| document-symbol | 0.063 / 0.100 | 0.062 / 0.078 | 0.147 / 0.176 | 0.57 | 0.44 | 12/12 | win |
| format-1k | 3.190 / 5.216 | 3.197 / 5.267 | 3.744 / 5.793 | 0.90 | 0.91 | 9/12 | win |
| hover | 0.052 / 0.145 | 0.053 / 0.150 | 0.202 / 6.975 | 0.02 | 0.02 | 12/12 | win |
| keystroke | 0.348 / 0.691 | 0.335 / 0.646 | 1.002 / 1.389 | 0.50 | 0.47 | 12/12 | win |
| keystroke-imported | 0.338 / 0.587 | 0.316 / 0.993 | 0.957 / 1.322 | 0.44 | 0.75 | 12/12 | win |
| keystroke-imported-new | 2.186 / 3.118 | 2.177 / 3.130 | 2.854 / 3.229 | 0.97 | 0.97 | 9/12 | NOT |
| keystroke-new | 2.339 / 3.310 | 2.319 / 3.338 | 2.992 / 4.040 | 0.82 | 0.83 | 9/12 | win |
| references-family | 0.184 / 0.268 | 0.144 / 0.255 | 0.857 / 0.972 | 0.28 | 0.26 | 11/12 | win |
| references-local | 0.112 / 0.148 | 0.075 / 0.132 | 0.272 / 0.327 | 0.45 | 0.40 | 12/12 | win |
| references-popular | 0.555 / 0.777 | 0.522 / 0.806 | 1.167 / 1.493 | 0.52 | 0.54 | 11/12 | win |
| rename | 0.244 / 0.364 | 0.196 / 0.327 | 0.611 / 0.739 | 0.49 | 0.44 | 12/12 | win |
| rename-const | 0.260 / 0.323 | 0.173 / 0.252 | 15.647 / 34.359 | 0.01 | 0.01 | 12/12 | win |
| rename-fanout | 1.168 / 1.609 | 0.734 / 1.067 | 16.929 / 31.122 | 0.05 | 0.03 | 12/12 | win |
| rename-field | 0.248 / 0.362 | 0.185 / 0.301 | 4.117 / 12.910 | 0.03 | 0.02 | 12/12 | win |
| rename-generic-func | 0.242 / 0.437 | 0.169 / 0.317 | 0.745 / 2.809 | 0.16 | 0.11 | 12/12 | win |
| rename-generic-method | 0.384 / 0.606 | 0.318 / 0.477 | 5.509 / 11.387 | 0.05 | 0.04 | 12/12 | win |
| rename-generic-type | 0.236 / 0.394 | 0.167 / 0.214 | 4.994 / 11.986 | 0.03 | 0.02 | 12/12 | win |
| rename-iface-method | 4.213 / 5.331 | 3.908 / 4.815 | 5.041 / 10.553 | 0.51 | 0.46 | 11/12 | win |
| rename-import-alias | 0.117 / 0.399 | 0.110 / 0.260 | 0.373 / 0.493 | 0.81 | 0.53 | 12/12 | win |
| rename-local | 0.125 / 0.197 | 0.121 / 0.140 | 0.358 / 0.516 | 0.38 | 0.27 | 12/12 | win |
| rename-method | 0.444 / 0.682 | 0.395 / 0.690 | 5.096 / 13.329 | 0.05 | 0.05 | 12/12 | win |
| rename-type | 0.241 / 0.295 | 0.177 / 0.308 | 5.006 / 9.402 | 0.03 | 0.03 | 11/12 | win |
| rename-unexported | 0.065 / 0.102 | 0.065 / 0.107 | 0.176 / 0.230 | 0.45 | 0.47 | 11/12 | win |
| rename-var | 1.460 / 3.210 | 0.920 / 1.432 | 17.076 / 30.376 | 0.11 | 0.05 | 12/12 | win |
| semantic-tokens | 0.050 / 0.088 | 0.049 / 0.060 | 0.059 / 0.079 | 1.12 | 0.76 | 10/12 | win |
| signature-help | 0.048 / 0.081 | 0.048 / 0.067 | 0.075 / 0.097 | 0.84 | 0.69 | 12/12 | win |
| switch-file | 0.487 / 1.121 | 0.523 / 1.167 | 1.587 / 2.087 | 0.54 | 0.56 | 12/12 | win |
| workspace-symbol | 0.250 / 0.420 | 0.125 / 0.175 | 1.928 / 3.383 | 0.12 | 0.05 | 12/12 | win |

### large (29 of 32 rows won)

| row | base p50 / p95 ms | new p50 / p95 ms | gopls p50 / p95 ms | base/gopls p95 | new/gopls p95 | rounds new p95 < gopls | verdict |
|---|---|---|---|---|---|---|---|
| api-change | 0.447 / 0.928 | 0.378 / 0.478 | 33.587 / 34.063 | 0.03 | 0.01 | 12/12 | win |
| api-fix | 0.401 / 1.010 | 0.346 / 0.436 | 33.860 / 34.183 | 0.03 | 0.01 | 12/12 | win |
| completion-dot | 0.088 / 0.147 | 0.085 / 0.104 | 0.344 / 0.557 | 0.26 | 0.19 | 12/12 | win |
| completion-edit | 2.308 / 4.516 | 2.212 / 4.450 | 3.136 / 3.838 | 1.18 | 1.16 | 7/12 | NOT |
| definition | 0.037 / 0.061 | 0.037 / 0.041 | 0.064 / 0.091 | 0.66 | 0.45 | 12/12 | win |
| document-symbol | 0.039 / 0.051 | 0.039 / 0.047 | 0.105 / 0.130 | 0.39 | 0.36 | 12/12 | win |
| format-1k | 3.328 / 5.275 | 3.244 / 5.312 | 3.885 / 6.276 | 0.84 | 0.85 | 11/12 | win |
| hover | 0.056 / 0.205 | 0.050 / 0.144 | 0.196 / 7.174 | 0.03 | 0.02 | 12/12 | win |
| keystroke | 0.541 / 0.961 | 0.443 / 0.842 | 0.971 / 1.150 | 0.84 | 0.73 | 11/12 | win |
| keystroke-imported | 0.512 / 1.194 | 0.446 / 0.960 | 0.941 / 1.193 | 1.00 | 0.80 | 10/12 | win |
| keystroke-imported-new | 1.964 / 3.196 | 1.857 / 3.045 | 2.193 / 2.752 | 1.16 | 1.11 | 5/12 | NOT |
| keystroke-new | 2.033 / 3.422 | 1.922 / 3.067 | 2.291 / 4.134 | 0.83 | 0.74 | 9/12 | win |
| references-family | 0.404 / 0.599 | 0.282 / 1.791 | 2.516 / 6.732 | 0.09 | 0.27 | 10/12 | win |
| references-local | 0.284 / 0.432 | 0.144 / 0.365 | 0.320 / 0.391 | 1.10 | 0.93 | 10/12 | win |
| references-popular | 4.002 / 5.560 | 3.775 / 4.147 | 6.070 / 6.470 | 0.86 | 0.64 | 12/12 | win |
| rename | 0.737 / 1.158 | 0.460 / 3.188 | 3.494 / 8.579 | 0.13 | 0.37 | 8/12 | win |
| rename-const | 0.983 / 1.526 | 0.527 / 0.573 | 59.197 / 62.968 | 0.02 | 0.01 | 12/12 | win |
| rename-fanout | 0.927 / 1.355 | 0.510 / 0.565 | 58.835 / 64.491 | 0.02 | 0.01 | 12/12 | win |
| rename-field | 1.018 / 4.846 | 0.600 / 1.251 | 15.626 / 28.017 | 0.17 | 0.04 | 12/12 | win |
| rename-generic-func | 0.584 / 2.001 | 0.317 / 0.601 | 3.688 / 10.328 | 0.19 | 0.06 | 12/12 | win |
| rename-generic-method | 1.114 / 1.820 | 0.970 / 1.968 | 15.823 / 30.859 | 0.06 | 0.06 | 12/12 | win |
| rename-generic-type | 0.553 / 0.740 | 0.308 / 3.007 | 15.609 / 32.349 | 0.02 | 0.09 | 12/12 | win |
| rename-iface-method | 1.507 / 2.274 | 1.271 / 1.648 | 16.055 / 22.587 | 0.10 | 0.07 | 12/12 | win |
| rename-import-alias | 0.106 / 0.679 | 0.110 / 0.379 | 0.301 / 0.392 | 1.73 | 0.97 | 7/12 | NOT |
| rename-local | 0.112 / 0.224 | 0.106 / 0.139 | 0.292 / 0.417 | 0.54 | 0.33 | 12/12 | win |
| rename-type | 0.581 / 0.880 | 0.317 / 0.359 | 15.620 / 32.135 | 0.03 | 0.01 | 12/12 | win |
| rename-unexported | 0.067 / 0.133 | 0.069 / 0.091 | 0.175 / 0.224 | 0.59 | 0.41 | 12/12 | win |
| rename-var | 22.563 / 39.957 | 6.664 / 7.260 | 70.325 / 82.002 | 0.49 | 0.09 | 12/12 | win |
| semantic-tokens | 0.033 / 0.043 | 0.033 / 0.041 | 0.061 / 0.080 | 0.54 | 0.51 | 12/12 | win |
| signature-help | 0.044 / 0.088 | 0.044 / 0.074 | 0.075 / 0.095 | 0.92 | 0.78 | 10/12 | win |
| switch-file | 0.727 / 1.594 | 0.669 / 1.474 | 2.867 / 3.672 | 0.43 | 0.40 | 12/12 | win |
| workspace-symbol | 0.749 / 1.159 | 0.379 / 0.580 | 3.563 / 4.429 | 0.26 | 0.13 | 12/12 | win |

### cpp (31 of 33 rows won)

| row | base p50 / p95 ms | new p50 / p95 ms | gopls p50 / p95 ms | base/gopls p95 | new/gopls p95 | rounds new p95 < gopls | verdict |
|---|---|---|---|---|---|---|---|
| api-change | 0.270 / 0.476 | 0.260 / 0.388 | 32.648 / 33.769 | 0.01 | 0.01 | 12/12 | win |
| api-fix | 0.242 / 0.533 | 0.229 / 0.327 | 32.758 / 33.832 | 0.02 | 0.01 | 12/12 | win |
| completion-dot | 0.097 / 0.133 | 0.097 / 0.127 | 0.401 / 0.560 | 0.24 | 0.23 | 12/12 | win |
| completion-edit | 2.351 / 3.771 | 2.498 / 4.639 | 3.368 / 4.618 | 0.82 | 1.00 | 5/12 | NOT |
| definition | 0.042 / 0.050 | 0.040 / 0.047 | 0.065 / 0.100 | 0.50 | 0.47 | 12/12 | win |
| document-symbol | 0.059 / 0.075 | 0.058 / 0.069 | 0.146 / 0.179 | 0.42 | 0.39 | 12/12 | win |
| format-1k | 3.141 / 5.173 | 3.105 / 4.998 | 3.871 / 6.483 | 0.80 | 0.77 | 11/12 | win |
| hover | 0.050 / 0.144 | 0.047 / 0.139 | 0.211 / 7.168 | 0.02 | 0.02 | 12/12 | win |
| keystroke | 0.326 / 0.718 | 0.304 / 0.526 | 0.893 / 1.151 | 0.62 | 0.46 | 12/12 | win |
| keystroke-imported | 0.319 / 1.112 | 0.302 / 0.456 | 0.911 / 1.298 | 0.86 | 0.35 | 12/12 | win |
| keystroke-imported-new | 2.042 / 2.940 | 2.043 / 3.023 | 2.488 / 3.074 | 0.96 | 0.98 | 7/12 | NOT |
| keystroke-new | 2.049 / 3.110 | 2.016 / 2.745 | 2.607 / 3.556 | 0.87 | 0.77 | 10/12 | win |
| references-family | 0.142 / 0.207 | 0.095 / 0.140 | 0.597 / 1.495 | 0.14 | 0.09 | 12/12 | win |
| references-local | 0.089 / 0.113 | 0.056 / 0.076 | 0.270 / 0.426 | 0.26 | 0.18 | 12/12 | win |
| references-popular | 0.269 / 0.416 | 0.228 / 0.335 | 0.611 / 0.703 | 0.59 | 0.48 | 10/12 | win |
| rename | 0.197 / 0.271 | 0.133 / 0.193 | 0.776 / 1.276 | 0.21 | 0.15 | 12/12 | win |
| rename-const | 0.171 / 0.193 | 0.114 / 0.147 | 5.479 / 8.634 | 0.02 | 0.02 | 12/12 | win |
| rename-fanout | 0.509 / 0.630 | 0.350 / 0.738 | 5.947 / 9.007 | 0.07 | 0.08 | 12/12 | win |
| rename-field | 0.164 / 0.197 | 0.115 / 0.149 | 3.267 / 3.967 | 0.05 | 0.04 | 12/12 | win |
| rename-generic-func | 0.173 / 0.251 | 0.120 / 0.185 | 0.877 / 4.123 | 0.06 | 0.04 | 12/12 | win |
| rename-generic-method | 0.294 / 0.410 | 0.251 / 0.780 | 4.094 / 6.354 | 0.06 | 0.12 | 12/12 | win |
| rename-generic-type | 0.179 / 0.208 | 0.136 / 0.183 | 3.591 / 7.756 | 0.03 | 0.02 | 12/12 | win |
| rename-iface-method | 1.314 / 1.658 | 1.206 / 1.638 | 4.230 / 5.340 | 0.31 | 0.31 | 12/12 | win |
| rename-import-alias | 0.102 / 0.274 | 0.097 / 0.274 | 0.349 / 0.540 | 0.51 | 0.51 | 12/12 | win |
| rename-local | 0.109 / 0.266 | 0.100 / 0.119 | 0.333 / 0.453 | 0.59 | 0.26 | 12/12 | win |
| rename-method | 0.335 / 0.436 | 0.267 / 0.325 | 4.064 / 5.719 | 0.08 | 0.06 | 12/12 | win |
| rename-type | 0.186 / 0.470 | 0.134 / 0.184 | 3.628 / 5.256 | 0.09 | 0.03 | 12/12 | win |
| rename-unexported | 0.065 / 0.120 | 0.063 / 0.076 | 0.181 / 0.230 | 0.52 | 0.33 | 12/12 | win |
| rename-var | 0.586 / 0.778 | 0.408 / 0.586 | 6.248 / 9.644 | 0.08 | 0.06 | 12/12 | win |
| semantic-tokens | 0.045 / 0.054 | 0.045 / 0.055 | 0.062 / 0.081 | 0.66 | 0.67 | 12/12 | win |
| signature-help | 0.046 / 0.071 | 0.044 / 0.058 | 0.076 / 0.103 | 0.69 | 0.56 | 12/12 | win |
| switch-file | 0.479 / 1.103 | 0.449 / 1.076 | 1.493 / 2.013 | 0.55 | 0.53 | 12/12 | win |
| workspace-symbol | 0.184 / 0.239 | 0.063 / 0.120 | 1.744 / 2.388 | 0.10 | 0.05 | 12/12 | win |

NOT won:
  cpp completion-edit new/gopls 1.00 (base 0.82) rounds 5/12
  cpp keystroke-imported-new new/gopls 0.98 (base 0.96) rounds 7/12
  large completion-edit new/gopls 1.16 (base 1.18) rounds 7/12
  large keystroke-imported-new new/gopls 1.11 (base 1.16) rounds 5/12
  large rename-import-alias new/gopls 0.97 (base 1.73) rounds 7/12
  medium keystroke-imported-new new/gopls 0.97 (base 0.97) rounds 9/12

RSS MiB median end:
  cpp base 161
  cpp gopls 339
  cpp new 158
  large base 362
  large gopls 688
  large new 363
  medium base 223
  medium gopls 511
  medium new 219
  small base 144
  small gopls 339
  small new 144

## Rows that do not win

With the 12 rounds (and with the other two choices of rounds):

| row | 12 rounds, load < 40 | 13 rounds, all complete | 8 rounds, load < 25 |
|---|---|---|---|
| `large completion-edit` | 1.16 (base 1.18) | 1.06 | 1.17 |
| `large keystroke-imported-new` | 1.11 (base 1.16) | 1.05 | 1.05 |
| `large rename-import-alias` | 0.97 (base 1.73) | 0.96 | won |
| `large references-local` | won (0.93) | 1.37 (base 1.10) | won |
| `cpp completion-edit` | 1.00 (base 0.82) | 1.01 | 1.10 |
| `cpp keystroke-imported-new` | 0.98 | 1.01 | 1.15 |
| `medium keystroke-imported-new` | 0.97 | 1.27 | won |

In the three views the new build has 6, 7 and 4 rows that are not won (base: 9 in the first view), and
none is over 1.4 times gopls (the contract's 1.5). Base has, in the first view, `small keystroke-new` 1.13,
`medium semantic-tokens` 1.12, `large completion-edit` 1.18, `large keystroke-imported` 1.00, `large
keystroke-imported-new` 1.16, `large references-local` 1.10, `large rename-import-alias` 1.73, and `medium` and
`cpp keystroke-imported-new` 0.97 and 0.96. What the job did: the rows of base over the bar that are won now
are `small keystroke-new`, `medium semantic-tokens`, `large keystroke-imported`, `large references-local` (in
two of the three views) and, nearly, `large rename-import-alias` (1.73 to 0.97); `large rename-var` p95 went
from 40.0 to 7.3 ms, `large workspace-symbol` from 1.16 to 0.58, `rename-const`, `rename-fanout`, `rename-field`,
`rename-generic-func`, `rename-iface-method` and `rename-type` p95 by 27% to 74%, and the p50 of `rename`,
`rename-const`, `rename-fanout`, `rename-field`, `rename-generic-func`, `rename-generic-type`, `rename-type` and
`rename-var` on `large` by 38% to 70% (the p50 of the other rename rows is the same within 5%, or 13 to 16% lower). What it did not:

- **The cost of a check on a keystroke**: `completion-edit` and `keystroke-imported-new` on `large` and `cpp`
  (a few tenths of a millisecond over the p95 of gopls, p50 under it), the same rows that ADR-123 and ADR-125
  left. The cpu profile of `keystroke-imported-new` on `large` (one run, scale 8) is the parser 18%, `check.Check` 17%
  and the collector's mark workers 35% of the samples: nothing in it of the size of the gap that I found.
- **The p95 of several rows is worse than base's** in this pool (more than 1.25 times for the first four below and the first of the next list), while their p50 is better. Also in this pool: `medium api-change` (0.388 to 1.219 ms), `medium keystroke-imported-new` (0.587 to 0.993), `cpp rename-generic-method` (0.41 to 0.78) and `medium api-fix` (0.335 to 0.42); all are still won against gopls (the largest is 0.75 of gopls' p95) and the reviewer's quieter runs did not reproduce them, so they look like where the collector cycle landed. The first four: `large rename`
  (1.16 to 3.19 ms; gopls 8.58), `large references-family` (0.60 to 1.79; gopls 6.73), `large
  rename-generic-type` (0.74 to 3.01; gopls 32.3) and `large rename-generic-method` (1.82 to 1.97). They are
  the tails of a cycle that now lands on them: still won against gopls by 2 to 10 times, and a lost row if
  the cycle lands on a row that is close to gopls (`large references-local`, 0.365 against 0.391 ms in the
  12 rounds, 1.37 times in the 13 rounds).

- **The 13-round view is a wash on latency** (new has 7 rows not won, base 8): the reliable gain of this round is the allocation count, not the latency percentiles.
- **`Snapshot.Memo` and the module index.** `didSave` bumps the index generation without making a new snapshot, so a memoized value (module imports, symbol list) can be one generation stale until the next edit; only the package name in the index is affected, which the search does not use. A transient `ImportsOf` failure is kept for the snapshot's life, and a waiter on `Memo` cannot be cancelled while another caller computes.

## Risks for the reviewer

- The `layered` tables replace plain maps in `state`: a bug is a stale or missing file, folder, module or
  handle. They are tested against a plain map under random operations (compaction required), `cloneHandles`
  against the old importers-index algorithm on random graphs with cycles and cut dependencies, and every
  mutation of those (a hidden key ignored, the size, a dropped own entry, no compaction, one pass instead of
  repeated, cut edges ignored, the order) fails a test. The package tests pass in full.
- `Snapshot.Memo` keeps a value for the life of the snapshot: the module imports and the symbol list are
  small (a few hundred KiB on `large`). A list of symbols with a file that did not parse is not kept.
- The load of the machine: 4 of 48 invocations were skipped and the table drops 4 of 16 rounds by a rule made
  after seeing the loads. The rows over the bar are not the same in the three views; the two that are
  in all three are the check-cost rows named above.
