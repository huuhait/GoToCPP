# ADR-123: Language server speed, round 8: the rename forms that were not covered, and the rows that are left

Status: accepted (2026-10-21)

## Context

The user's priority for the language server is that it beats gopls on every measured row.
[ADR-119](119-lsp-speed-round-7-typing-rows-and-the-proofs-of-a-rename.md) won the typing rows and the
rename of a function, type, method, field and local variable on `small` and `medium`. This job (round 2
of 11.lspgap) measured the rows that had not been measured or not been won (the projects `large`,
`cpp` and `macro`, the cold open, hover, references, definition, document symbols, completion, and the
other forms of rename), and fixed what lost.

The predeclared rule for a row to be won: the pooled p95 of gocpp is under 0.95 times the pooled p95 of
gopls, and the p95 of gocpp is the lower one in most of the rounds. Percentiles are nearest-rank over
the pooled samples of all rounds. The first measurement (campaign `M1`, base) found these rows lost or
not measured:

- the rename of a method that an interface of the project declares (`rename-iface-method`): 5.5 to 38
  ms against gopls 0.8 to 5 ms. gocpp re-checked the packages after the edit because any tie to an
  interface that has the old name made the method rename unprovable;
- the rename of an exported variable or constant, of a package variable with many importers
  (`rename-var`, `rename-const`, `rename-fanout`): 0.7 to 25 ms where the function rename was 0.15 ms,
  because the proofs of ADR-119 covered functions and types only;
- the rename of an import alias (2.6 to 3.9 ms against gopls 0.3 to 0.4), of a generic function or
  type (2.5 to 3.9 ms against 0.5 to 0.8 for `small`) and of a name that is not exported (0.7 ms
  against 0.18): not in the benchmark, and measured when the rows were added;
- `large`, `references-popular` p95 10.7 against gopls 6.5, and a few tails of a row of 50 samples;
- `format-1k` at parity (p50 0.94, p95 about 1.0 of gopls) on every project, and `semantic-tokens` at
  parity (p50 about 1.0).

## Measurement provenance

Everything was measured with `lspbench` (protocol method `fresh-edits-v2-go1.25`) by one invocation per
(round, arm), the arms rotating by round, gopls an arm of its own, each under the shared `perf.lock`, on
cores 16 to 23 with `GOMAXPROCS=8` (AMD Ryzen Threadripper PRO 9995WX, Linux 6.19.6, go1.25.7), gopls
v0.21.1 with `diagnosticsDelay` 30 ms, `-scale 2` (`-scale 1` for the typing rows), one run per
invocation, and only when the one-minute load average was under 50 (the invocations of the table
started at loads of 0.7 to 6.7; the load is in the `invocations.jsonl` of each campaign). A failed or
skipped invocation is recorded and not retried.

- **base**: `3e3c3c20` (the origin/dev this job started from), `gocpp` sha256 `7510207b...`.
- **final**: `7824a821` (the last commit of this job), `gocpp` sha256 `7006e5b5...`; `lspbench` sha256
  `bf6afb1c...`, the same binary for every arm of the table.
- The table below is campaign `F10`: all the rows, the four projects `small`, `medium`, `large` and
  `cpp`, 5 rounds of base and gopls and 4 of final. Round 5 of the final arm has no data: the
  invocation waited ten minutes for `perf.lock`, which another job held, and the wait timed out; it was
  not retried. Nothing else of mine ran during `F10` (the campaigns before it, `F2` to `F4`, overlapped
  with my own focused tests and profiling on other cores, which is why the table is a campaign of its
  own and why their noisier rows are not used). Earlier campaigns (`M1` base and gopls, `F1` the
  intermediate build, `F2`, `F3`, `F8` the rename rows alone) agree with `F10` except for the rows listed
  with their ratios in the table of the rows that do not win, below. The `macro` and `bigpkg` projects (campaigns `F4`, `F5`, `F6`, 4 rounds, build
  `e476d056`: the later commits touched no code that those rows use except the printer and the tokens,
  see `F7` below) are the second table.
- `F7` (6 rounds, build `1ad0368f`) measured `format-1k`, `document-symbol` and `semantic-tokens` after
  the printer change: `format-1k` p50 / p95 on `small` 3.12 / 3.32 against gopls 3.60 / 3.80 (0.87),
  `medium` 3.02 / 3.22 against 3.65 / 3.93 (0.82), `large` 3.17 / 3.51 against 3.67 / 3.92 (0.89), `cpp`
  3.10 / 3.44 against 3.65 / 3.95 (0.87), every round won; `F10` has the same rows on the final build.
- The correctness gate (`lspdiff`, final against base, `small` and `medium`) compared 6,312 and 6,366
  answers (hover, definition, references, highlights, rename, completion, signatures, symbols, tokens,
  formatting, links, code actions, hints, workspace symbols and the diagnostics of every open file,
  before and after 24 edits of the hot file): none differs, and the replies are the same in kind (5,097
  and 5,189 successful, 753 and 715 JSON-RPC refusals, none failed) in both. That is the build of
  `1ad0368f`; the commit after it changes only the encoding of one answer, which a test covers.

## Decisions

1. **A package-level variable or constant, generic functions and types, and a name that is not exported
   are renamed without the scratch check** (`declaringScopeProvesResult`). The guards that exist
   (conflicts, captures, the members of the type, the embedders, the test declarations) are asked first,
   the search must be complete, and then the proof is the one of an exported function: renaming a
   package-scope name changes no type and no method set, the importers use it by a qualified name only
   (`qualifiedOnly`) and the own package has every occurrence of the name as an edit (the identifier
   index of the check, not a hit count). A variable is a package value (`packageValue`: declared by the
   file's own declaration, not a field); the new name is not `main`, `init` or `C`. A name that is not
   exported, renamed to another one that is not, is seen by its package alone and needs no importer; a
   generic function or type is looked up like any other name (its type parameters are names of its own
   scope, which the capture guard already looks at). Everything else (a method, an alias, a name that
   a directive or a macro touches, a package of the standard library) keeps the scratch check.
2. **A `#` in a string or a comment no longer disables the proofs** (`hasMacroName`, a scan of the
   tokens of the file with the scanner of the language). The proofs refused a package when a file had
   a `#` byte anywhere, which `core` of the benchmark has in a string or a comment, so the proofs of ADR-119 never
   ran for it. A macro name is a `#` followed by a name, as the scanner reads it.
3. **The rename of the name of an import is proved by the scope of its file**
   (`importNameProvesResult`). The edits are the occurrences of the import name in one file; the proof
   needs every occurrence an edit (the identifier index and the specs without a name), no declaration
   of the new name in the file scope or the package scope, and no use of the new name in that file
   (except as a selected name or a key). The new name `_`, `init` and `C` keep the check. The proof
   does not apply when the package has a `_test.gp` file (internal or external) at all: the check of the
   result does not see those files, and the full validator refuses a new name that a test file of the
   package declares ("q already declared through import of package lib"), so the form falls back to the
   check. For the same reason `checkTestDecls` now covers the name of an import, which it skipped as a
   name that is not at package level; without it the real server accepted such a rename whenever the
   test file did not mention the old name (the full validator reads a test file only when it does).
   The import of a test file that has the same name does not count (imports are names of their own
   file); only its package-level declarations do.
4. **A method of an interface that the project declares is proved when both ends of every tie are
   renamed together** (`tieIsRenamedAlike`, used by `mayBreak`). A tie is a value of one type used where
   an interface is wanted. With a tie to an interface that has the old method name, the rename could
   leave the value without the method the interface needs. If the method of the interface and the one
   that the value's type has under that name are both in the family that the rename changes (the search
   closes the family over the ties it sees), or neither is, the tie does not see the rename; a tie with
   no value (a type assertion) or whose ends the lookup cannot place is not alike. Every tie of an
   interface that has the new name stays a reason for the check.
5. **The hits of a search are collected into one list made at its size** (`hitCapacity`,
   `maxHitCapacity`; the sort and the removal of repeated places run once over it; the table of file
   information and of documents is made at its size), **a big answer is sized once and framed without
   a second copy** (`LocationList.compactJSON` presized, `trustedHead`, `WriteParts`: the head of the
   message and the result go to the writer as two parts), and **the answer for the tokens of a whole
   file is encoded once with the tokens** (`protocol.EncodeTokens`, kept in the package's memo; the
   answer of the range and of a stale text still go through encoding/json). These take garbage and
   bytes out of the requests that answer in thousands of places (the heap profile of a whole run on
   `large` allocated 666 MB against 776 MB for the base, 14% less) and, for the tokens, a request
   of 0.062 / 0.239 ms (p50 / p95) on `small` became 0.048 / 0.063 against gopls 0.067 / 0.081.
6. **The printer finds positions with its own line table** (`internal/syntax/printer/linetable.go`).
   The profile of `format-1k` had 10% in `token.FileSet.PositionFor`, a lock and a search from scratch
   for every token the printer asked about. The printer now keeps the line table of its file (shared
   with the printers that measure the size of a node) and tries the line found last first; the result
   is exactly `PositionFor(pos, false)`. `computeEdits` keys a line without building a string for it.
   The formatting of 1,000 lines (`BenchmarkFormatBig`, without the protocol) went from 2.21 to 1.91 ms,
   and the row from p50 parity to 0.81-0.89 of gopls.
7. **The benchmark asks for the forms**: the scenarios `rename-var`, `rename-const`, `rename-fanout`,
   `rename-iface-method`, `rename-unexported`, `rename-import-alias`, `rename-generic-func`,
   `rename-generic-type` and `rename-generic-method` (the last four in a file the scenario adds to the
   hot package, which imports the core package under an alias and declares the generics). gopls refuses
   to rename a method of a type that an interface of the project ties, so the interface row renames the
   method on the interface.

## What was tried and not applied

- **The soft memory limit and the GC pacing for a big heap.** My hypothesis for the remaining tails on
  `large` (below) is the cycles of the collector: a run of 3 to 10 consecutive requests that take 2 to
  20 times their median, at the same phase of the allocation sequence in every round. I did not take a
  `gctrace` of those requests, so this is a hypothesis, not a finding; the two experiments below did
  not support it (they did not lower the spikes either). A looser memory limit
  (`GOMEMLIMIT=700MiB`, which makes the pacer's GOGC 100 the goal), three full runs against three:
  22 requests of 3,534 over three times their row's median against 25 with the limit of the server
  (the rows that improved were the rows where the cycle did not land that round), and 533 MiB resident
  against 360. The idle collector (`-idle-gc`): 28 spikes. Neither lowered the tail, and the
  first costs 170 MiB; both left as they are. The cause of the tail rows is therefore not established.
- **Fewer bytes allocated by `references`.** A popular name has about 1,500 hits in 1,500 files on `large`
  (a document entry, a URI and a mapper per file). Pooling the lists would lower the bytes of the request,
  but only moves where a cycle lands; it is not a fix for the p95 of a row of 12 requests per round. Not
  done.

## Results

Median / p95 in milliseconds, pooled over the rounds, nearest-rank. "before" is the base build, "after"
the final one, in the same campaign. `n` is the samples per arm of the final build. "rounds won" counts
the rounds in which the p95 of gocpp is lower than the p95 of gopls of the same round. "wins" is the
predeclared rule: p95 ratio under 0.95 and more than half of the rounds won. Rows with no gopls
(`open-cpp-*`, `typo-*`: gopls cannot read the file) are gocpp's own.

### Projects small, medium, large, cpp (campaign F10)

| row | gocpp before p50 / p95 ms | gocpp after p50 / p95 ms | gopls p50 / p95 ms | p95 ratio | p50 ratio | n | rounds won | wins |
|---|---|---|---|---|---|---|---|---|
| small api-change | 0.495 / 0.728 | 0.209 / 0.279 | 32.465 / 32.751 | 0.01 | 0.01 | 32 | 4/4 | yes |
| small api-fix | 0.341 / 0.726 | 0.182 / 0.235 | 32.446 / 32.729 | 0.01 | 0.01 | 32 | 4/4 | yes |
| small completion-dot | 0.096 / 0.115 | 0.095 / 0.108 | 0.390 / 0.561 | 0.19 | 0.24 | 480 | 4/4 | yes |
| small completion-edit | 2.048 / 2.755 | 2.008 / 2.297 | 2.788 / 3.340 | 0.69 | 0.72 | 240 | 4/4 | yes |
| small definition | 0.034 / 0.046 | 0.038 / 0.043 | 0.062 / 0.090 | 0.48 | 0.62 | 480 | 4/4 | yes |
| small document-symbol | 0.061 / 0.289 | 0.059 / 0.064 | 0.151 / 0.184 | 0.35 | 0.39 | 320 | 4/4 | yes |
| small format-1k | 3.208 / 3.723 | 2.986 / 3.397 | 3.691 / 4.050 | 0.84 | 0.81 | 120 | 3/4 | yes |
| small hover | 0.050 / 0.147 | 0.050 / 0.146 | 0.195 / 6.890 | 0.02 | 0.26 | 480 | 4/4 | yes |
| small keystroke | 0.253 / 0.393 | 0.241 / 0.371 | 0.863 / 0.982 | 0.38 | 0.28 | 320 | 4/4 | yes |
| small keystroke-imported | 0.242 / 0.384 | 0.237 / 0.328 | 0.843 / 0.945 | 0.35 | 0.28 | 160 | 4/4 | yes |
| small keystroke-imported-new | 1.683 / 1.990 | 1.690 / 2.016 | 2.153 / 2.453 | 0.82 | 0.79 | 160 | 4/4 | yes |
| small keystroke-new | 1.854 / 2.213 | 1.815 / 2.112 | 2.155 / 2.404 | 0.88 | 0.84 | 320 | 4/4 | yes |
| small open-cold | 58.730 / 59.290 | 58.049 / 59.158 | 179.426 / 188.767 | 0.31 | 0.32 | 4 | 4/4 | yes |
| small open-warm | 26.981 / 28.485 | 26.757 / 27.164 | 45.801 / 49.329 | 0.55 | 0.58 | 4 | 4/4 | yes |
| small references-family | 0.088 / 0.246 | 0.085 / 0.135 | 0.396 / 0.431 | 0.31 | 0.21 | 64 | 4/4 | yes |
| small references-local | 0.067 / 0.152 | 0.065 / 0.081 | 0.229 / 0.256 | 0.32 | 0.28 | 160 | 4/4 | yes |
| small references-popular | 0.114 / 0.279 | 0.103 / 0.162 | 0.290 / 0.348 | 0.47 | 0.35 | 48 | 4/4 | yes |
| small rename | 0.143 / 0.290 | 0.134 / 0.185 | 0.483 / 0.553 | 0.33 | 0.28 | 40 | 4/4 | yes |
| small rename-const | 0.706 / 1.401 | 0.101 / 0.135 | 0.565 / 0.635 | 0.21 | 0.18 | 40 | 4/4 | yes |
| small rename-fanout | 0.785 / 1.601 | 0.167 / 0.187 | 0.670 / 0.745 | 0.25 | 0.25 | 40 | 4/4 | yes |
| small rename-field | 0.116 / 0.241 | 0.116 / 0.142 | 0.395 / 0.440 | 0.32 | 0.29 | 40 | 4/4 | yes |
| small rename-generic-func | 2.522 / 2.870 | 0.116 / 0.167 | 0.595 / 3.273 | 0.05 | 0.19 | 40 | 4/4 | yes |
| small rename-generic-method | 0.209 / 0.293 | 0.199 / 0.242 | 0.759 / 0.925 | 0.26 | 0.26 | 40 | 4/4 | yes |
| small rename-generic-type | 2.599 / 3.478 | 0.125 / 0.149 | 0.503 / 0.602 | 0.25 | 0.25 | 40 | 4/4 | yes |
| small rename-iface-method | 5.489 / 13.646 | 0.540 / 0.602 | 0.804 / 0.893 | 0.67 | 0.67 | 40 | 4/4 | yes |
| small rename-import-alias | 2.551 / 2.848 | 0.081 / 0.166 | 0.310 / 0.409 | 0.41 | 0.26 | 40 | 4/4 | yes |
| small rename-local | 0.102 / 0.255 | 0.101 / 0.116 | 0.275 / 0.326 | 0.36 | 0.37 | 40 | 4/4 | yes |
| small rename-method | 0.221 / 0.423 | 0.216 / 0.252 | 0.723 / 0.823 | 0.31 | 0.30 | 40 | 4/4 | yes |
| small rename-type | 0.140 / 0.244 | 0.139 / 0.182 | 0.505 / 0.592 | 0.31 | 0.28 | 40 | 4/4 | yes |
| small rename-unexported | 0.658 / 0.732 | 0.064 / 0.076 | 0.180 / 0.232 | 0.33 | 0.35 | 40 | 4/4 | yes |
| small rename-var | 0.851 / 1.725 | 0.191 / 0.262 | 0.726 / 0.838 | 0.31 | 0.26 | 40 | 4/4 | yes |
| small semantic-tokens | 0.062 / 0.239 | 0.048 / 0.063 | 0.067 / 0.081 | 0.77 | 0.71 | 160 | 4/4 | yes |
| small signature-help | 0.047 / 0.091 | 0.047 / 0.058 | 0.074 / 0.099 | 0.59 | 0.63 | 480 | 4/4 | yes |
| small switch-file | 0.371 / 0.469 | 0.365 / 0.421 | 1.357 / 1.534 | 0.27 | 0.27 | 160 | 4/4 | yes |
| small workspace-symbol | 0.127 / 0.482 | 0.126 / 0.165 | 1.602 / 2.172 | 0.08 | 0.08 | 240 | 4/4 | yes |
| medium api-change | 0.589 / 2.162 | 0.293 / 0.360 | 32.636 / 32.864 | 0.01 | 0.01 | 32 | 4/4 | yes |
| medium api-fix | 0.544 / 1.431 | 0.243 / 0.338 | 32.847 / 33.061 | 0.01 | 0.01 | 32 | 4/4 | yes |
| medium completion-dot | 0.098 / 0.115 | 0.097 / 0.120 | 0.401 / 0.571 | 0.21 | 0.24 | 480 | 4/4 | yes |
| medium completion-edit | 2.544 / 2.960 | 2.524 / 2.907 | 3.713 / 4.259 | 0.68 | 0.68 | 240 | 4/4 | yes |
| medium definition | 0.039 / 0.047 | 0.039 / 0.046 | 0.061 / 0.090 | 0.51 | 0.64 | 480 | 4/4 | yes |
| medium document-symbol | 0.061 / 0.095 | 0.062 / 0.073 | 0.150 / 0.196 | 0.37 | 0.41 | 320 | 4/4 | yes |
| medium format-1k | 3.418 / 3.888 | 3.110 / 3.537 | 3.645 / 4.019 | 0.88 | 0.85 | 120 | 4/4 | yes |
| medium hover | 0.052 / 0.150 | 0.052 / 0.151 | 0.205 / 6.986 | 0.02 | 0.25 | 480 | 4/4 | yes |
| medium keystroke | 0.316 / 0.417 | 0.298 / 0.423 | 0.984 / 1.148 | 0.37 | 0.30 | 320 | 4/4 | yes |
| medium keystroke-imported | 0.300 / 0.396 | 0.300 / 0.387 | 0.942 / 1.057 | 0.37 | 0.32 | 160 | 4/4 | yes |
| medium keystroke-imported-new | 2.143 / 2.793 | 2.129 / 2.644 | 2.843 / 3.057 | 0.86 | 0.75 | 160 | 4/4 | yes |
| medium keystroke-new | 2.234 / 2.590 | 2.266 / 2.639 | 2.953 / 3.546 | 0.74 | 0.77 | 320 | 4/4 | yes |
| medium open-cold | 91.228 / 92.693 | 92.279 / 93.195 | 209.851 / 223.853 | 0.42 | 0.44 | 4 | 4/4 | yes |
| medium open-warm | 31.117 / 31.533 | 30.997 / 31.113 | 55.634 / 56.114 | 0.55 | 0.56 | 4 | 4/4 | yes |
| medium references-family | 0.181 / 0.253 | 0.184 / 0.221 | 0.866 / 0.902 | 0.24 | 0.21 | 64 | 4/4 | yes |
| medium references-local | 0.111 / 0.134 | 0.110 / 0.131 | 0.279 / 0.319 | 0.41 | 0.39 | 160 | 4/4 | yes |
| medium references-popular | 0.609 / 0.689 | 0.536 / 0.626 | 1.166 / 1.479 | 0.42 | 0.46 | 48 | 4/4 | yes |
| medium rename | 0.241 / 0.402 | 0.239 / 0.385 | 0.621 / 0.756 | 0.51 | 0.38 | 40 | 4/4 | yes |
| medium rename-const | 0.969 / 1.149 | 0.259 / 0.297 | 15.830 / 21.389 | 0.01 | 0.02 | 40 | 4/4 | yes |
| medium rename-fanout | 2.394 / 2.754 | 1.106 / 1.396 | 16.556 / 21.619 | 0.06 | 0.07 | 40 | 4/4 | yes |
| medium rename-field | 0.238 / 0.456 | 0.233 / 0.296 | 4.158 / 8.629 | 0.03 | 0.06 | 40 | 4/4 | yes |
| medium rename-generic-func | 3.944 / 6.383 | 0.227 / 0.454 | 0.725 / 2.656 | 0.17 | 0.31 | 40 | 4/4 | yes |
| medium rename-generic-method | 0.395 / 1.667 | 0.373 / 0.548 | 5.453 / 13.509 | 0.04 | 0.07 | 40 | 4/4 | yes |
| medium rename-generic-type | 3.891 / 4.075 | 0.229 / 0.259 | 4.526 / 10.446 | 0.02 | 0.05 | 40 | 4/4 | yes |
| medium rename-iface-method | 37.907 / 44.495 | 4.045 / 4.389 | 5.134 / 9.213 | 0.48 | 0.79 | 40 | 4/4 | yes |
| medium rename-import-alias | 3.895 / 4.137 | 0.111 / 0.311 | 0.376 / 0.463 | 0.67 | 0.30 | 40 | 4/4 | yes |
| medium rename-local | 0.125 / 0.150 | 0.124 / 0.146 | 0.364 / 0.440 | 0.33 | 0.34 | 40 | 4/4 | yes |
| medium rename-method | 0.424 / 0.607 | 0.414 / 0.609 | 5.049 / 7.899 | 0.08 | 0.08 | 40 | 4/4 | yes |
| medium rename-type | 0.240 / 2.869 | 0.234 / 0.264 | 5.336 / 9.825 | 0.03 | 0.04 | 40 | 4/4 | yes |
| medium rename-unexported | 0.730 / 2.770 | 0.068 / 0.122 | 0.181 / 0.224 | 0.55 | 0.38 | 40 | 4/4 | yes |
| medium rename-var | 2.658 / 3.477 | 1.554 / 5.024 | 17.035 / 22.879 | 0.22 | 0.09 | 40 | 4/4 | yes |
| medium semantic-tokens | 0.063 / 0.069 | 0.050 / 0.059 | 0.067 / 0.081 | 0.73 | 0.74 | 160 | 4/4 | yes |
| medium signature-help | 0.048 / 0.077 | 0.048 / 0.070 | 0.074 / 0.097 | 0.73 | 0.64 | 480 | 4/4 | yes |
| medium switch-file | 0.443 / 0.711 | 0.425 / 0.556 | 1.518 / 1.842 | 0.30 | 0.28 | 160 | 4/4 | yes |
| medium workspace-symbol | 0.249 / 0.406 | 0.259 / 0.313 | 1.956 / 2.486 | 0.13 | 0.13 | 240 | 4/4 | yes |
| large api-change | 0.423 / 0.508 | 0.420 / 0.510 | 33.583 / 34.233 | 0.01 | 0.01 | 32 | 4/4 | yes |
| large api-fix | 0.415 / 0.482 | 0.393 / 0.479 | 33.849 / 34.235 | 0.01 | 0.01 | 32 | 4/4 | yes |
| large completion-dot | 0.085 / 0.135 | 0.084 / 0.097 | 0.341 / 0.502 | 0.19 | 0.25 | 480 | 4/4 | yes |
| large completion-edit | 2.091 / 2.549 | 2.075 / 2.374 | 3.052 / 3.478 | 0.68 | 0.68 | 240 | 4/4 | yes |
| large definition | 0.032 / 0.037 | 0.032 / 0.037 | 0.064 / 0.091 | 0.40 | 0.50 | 480 | 4/4 | yes |
| large document-symbol | 0.040 / 0.310 | 0.045 / 0.287 | 0.103 / 0.137 | 2.09 | 0.43 | 320 | 2/4 | NO |
| large format-1k | 3.534 / 3.888 | 3.179 / 3.512 | 3.695 / 4.124 | 0.85 | 0.86 | 120 | 4/4 | yes |
| large hover | 0.051 / 0.148 | 0.051 / 0.148 | 0.195 / 7.316 | 0.02 | 0.26 | 480 | 4/4 | yes |
| large keystroke | 0.496 / 0.621 | 0.508 / 0.621 | 0.979 / 1.133 | 0.55 | 0.52 | 320 | 4/4 | yes |
| large keystroke-imported | 0.480 / 0.780 | 0.489 / 0.776 | 0.928 / 1.067 | 0.73 | 0.53 | 160 | 4/4 | yes |
| large keystroke-imported-new | 1.864 / 2.208 | 1.892 / 2.372 | 2.158 / 2.360 | 1.00 | 0.88 | 160 | 3/4 | NO |
| large keystroke-new | 1.975 / 2.274 | 1.968 / 2.395 | 2.270 / 2.608 | 0.92 | 0.87 | 320 | 4/4 | yes |
| large open-cold | 148.918 / 150.183 | 144.194 / 144.935 | 321.902 / 333.709 | 0.43 | 0.45 | 4 | 4/4 | yes |
| large open-warm | 42.489 / 44.367 | 42.092 / 42.276 | 120.981 / 122.066 | 0.35 | 0.35 | 4 | 4/4 | yes |
| large references-family | 0.390 / 0.430 | 0.388 / 0.433 | 2.434 / 2.600 | 0.17 | 0.16 | 64 | 4/4 | yes |
| large references-local | 0.271 / 0.934 | 0.271 / 0.329 | 0.320 / 0.371 | 0.89 | 0.84 | 160 | 4/4 | yes |
| large references-popular | 4.613 / 9.242 | 4.032 / 9.295 | 6.189 / 6.448 | 1.44 | 0.65 | 48 | 0/4 | NO |
| large rename | 0.720 / 1.061 | 0.694 / 0.898 | 3.577 / 6.979 | 0.13 | 0.19 | 40 | 4/4 | yes |
| large rename-const | 1.979 / 2.559 | 1.004 / 7.041 | 58.997 / 63.256 | 0.11 | 0.02 | 40 | 4/4 | yes |
| large rename-fanout | 1.965 / 2.104 | 0.987 / 5.899 | 58.554 / 63.126 | 0.09 | 0.02 | 40 | 4/4 | yes |
| large rename-field | 0.837 / 3.358 | 0.896 / 3.606 | 15.048 / 21.457 | 0.17 | 0.06 | 40 | 3/4 | yes |
| large rename-generic-func | 3.552 / 11.756 | 0.592 / 1.102 | 3.687 / 12.398 | 0.09 | 0.16 | 40 | 4/4 | yes |
| large rename-generic-method | 1.182 / 2.014 | 1.110 / 3.729 | 15.462 / 30.595 | 0.12 | 0.07 | 40 | 4/4 | yes |
| large rename-generic-type | 3.306 / 3.562 | 0.568 / 0.627 | 14.812 / 31.545 | 0.02 | 0.04 | 40 | 4/4 | yes |
| large rename-iface-method | 1.471 / 1.884 | 1.474 / 1.797 | 15.903 / 23.434 | 0.08 | 0.09 | 40 | 4/4 | yes |
| large rename-import-alias | 3.400 / 14.424 | 0.158 / 3.064 | 0.307 / 0.402 | 7.63 | 0.52 | 40 | 3/4 | NO |
| large rename-local | 0.110 / 0.158 | 0.108 / 0.139 | 0.286 / 0.377 | 0.37 | 0.38 | 40 | 4/4 | yes |
| large rename-type | 0.603 / 6.643 | 0.575 / 3.475 | 15.000 / 22.972 | 0.15 | 0.04 | 40 | 4/4 | yes |
| large rename-unexported | 0.797 / 0.892 | 0.074 / 2.231 | 0.182 / 0.234 | 9.53 | 0.41 | 40 | 2/4 | NO |
| large rename-var | 22.115 / 33.347 | 21.286 / 30.797 | 69.784 / 75.457 | 0.41 | 0.31 | 40 | 4/4 | yes |
| large semantic-tokens | 0.040 / 0.048 | 0.033 / 0.060 | 0.059 / 0.082 | 0.73 | 0.56 | 160 | 4/4 | yes |
| large signature-help | 0.038 / 0.070 | 0.038 / 0.066 | 0.076 / 0.096 | 0.69 | 0.51 | 480 | 4/4 | yes |
| large switch-file | 0.562 / 1.076 | 0.553 / 1.150 | 2.795 / 3.372 | 0.34 | 0.20 | 160 | 4/4 | yes |
| large workspace-symbol | 0.722 / 1.225 | 0.765 / 1.593 | 3.565 / 4.441 | 0.36 | 0.21 | 240 | 4/4 | yes |
| cpp api-change | 0.471 / 1.195 | 0.255 / 0.343 | 32.571 / 32.785 | 0.01 | 0.01 | 32 | 4/4 | yes |
| cpp api-fix | 0.464 / 1.161 | 0.221 / 0.287 | 32.712 / 32.982 | 0.01 | 0.01 | 32 | 4/4 | yes |
| cpp completion-dot | 0.095 / 0.108 | 0.096 / 0.112 | 0.393 / 0.562 | 0.20 | 0.24 | 480 | 4/4 | yes |
| cpp completion-edit | 2.231 / 2.612 | 2.229 / 2.612 | 3.199 / 3.665 | 0.71 | 0.70 | 240 | 4/4 | yes |
| cpp definition | 0.037 / 0.041 | 0.036 / 0.043 | 0.064 / 0.088 | 0.48 | 0.57 | 480 | 4/4 | yes |
| cpp document-symbol | 0.058 / 0.064 | 0.057 / 0.062 | 0.145 / 0.193 | 0.32 | 0.39 | 320 | 4/4 | yes |
| cpp format-1k | 3.349 / 3.816 | 3.036 / 3.331 | 3.651 / 3.958 | 0.84 | 0.83 | 120 | 4/4 | yes |
| cpp hover | 0.051 / 0.148 | 0.049 / 0.146 | 0.203 / 6.940 | 0.02 | 0.24 | 480 | 4/4 | yes |
| cpp keystroke | 0.287 / 0.386 | 0.290 / 0.392 | 0.877 / 0.980 | 0.40 | 0.33 | 320 | 4/4 | yes |
| cpp keystroke-imported | 0.296 / 0.429 | 0.299 / 0.408 | 0.901 / 1.048 | 0.39 | 0.33 | 160 | 4/4 | yes |
| cpp keystroke-imported-new | 1.890 / 2.213 | 2.016 / 2.842 | 2.435 / 2.618 | 1.09 | 0.83 | 160 | 3/4 | NO |
| cpp keystroke-new | 2.021 / 2.436 | 1.996 / 2.377 | 2.570 / 2.986 | 0.80 | 0.78 | 320 | 4/4 | yes |
| cpp open-cold | 75.695 / 77.549 | 74.520 / 75.602 | 191.213 / 213.341 | 0.35 | 0.39 | 4 | 4/4 | yes |
| cpp open-cpp-cold | 1.832 / 2.303 | 1.612 / 2.690 | - | - | - | 4 | - | (no gopls row) |
| cpp open-cpp-warm | 1.528 / 2.506 | 1.598 / 2.765 | - | - | - | 4 | - | (no gopls row) |
| cpp open-warm | 29.621 / 32.617 | 29.617 / 29.745 | 47.839 / 49.666 | 0.60 | 0.62 | 4 | 4/4 | yes |
| cpp references-family | 0.134 / 0.177 | 0.132 / 0.166 | 0.592 / 0.621 | 0.27 | 0.22 | 64 | 4/4 | yes |
| cpp references-local | 0.087 / 0.110 | 0.088 / 0.106 | 0.265 / 0.302 | 0.35 | 0.33 | 160 | 4/4 | yes |
| cpp references-popular | 0.288 / 0.342 | 0.274 / 0.735 | 0.617 / 0.747 | 0.98 | 0.44 | 48 | 3/4 | NO |
| cpp rename | 0.198 / 0.262 | 0.198 / 0.242 | 0.762 / 0.957 | 0.25 | 0.26 | 40 | 4/4 | yes |
| cpp rename-const | 0.810 / 0.883 | 0.175 / 0.213 | 5.378 / 8.161 | 0.03 | 0.03 | 40 | 4/4 | yes |
| cpp rename-fanout | 1.262 / 1.450 | 0.500 / 0.522 | 5.877 / 8.435 | 0.06 | 0.09 | 40 | 4/4 | yes |
| cpp rename-field | 0.165 / 0.196 | 0.171 / 0.214 | 3.243 / 3.542 | 0.06 | 0.05 | 40 | 4/4 | yes |
| cpp rename-generic-func | 3.245 / 5.717 | 0.172 / 0.224 | 0.963 / 4.261 | 0.05 | 0.18 | 40 | 4/4 | yes |
| cpp rename-generic-method | 0.303 / 0.462 | 0.288 / 0.398 | 4.058 / 4.412 | 0.09 | 0.07 | 40 | 4/4 | yes |
| cpp rename-generic-type | 3.176 / 3.467 | 0.172 / 0.192 | 3.583 / 4.975 | 0.04 | 0.05 | 40 | 4/4 | yes |
| cpp rename-iface-method | 13.804 / 17.214 | 1.301 / 1.630 | 4.241 / 4.926 | 0.33 | 0.31 | 40 | 4/4 | yes |
| cpp rename-import-alias | 3.197 / 3.395 | 0.094 / 0.247 | 0.342 / 0.440 | 0.56 | 0.27 | 40 | 4/4 | yes |
| cpp rename-local | 0.106 / 0.124 | 0.110 / 0.129 | 0.328 / 0.398 | 0.32 | 0.34 | 40 | 4/4 | yes |
| cpp rename-method | 0.316 / 0.360 | 0.310 / 0.370 | 4.022 / 4.440 | 0.08 | 0.08 | 40 | 4/4 | yes |
| cpp rename-type | 0.183 / 0.217 | 0.181 / 0.197 | 3.607 / 5.218 | 0.04 | 0.05 | 40 | 4/4 | yes |
| cpp rename-unexported | 0.697 / 0.748 | 0.063 / 0.084 | 0.172 / 0.237 | 0.35 | 0.37 | 40 | 4/4 | yes |
| cpp rename-var | 1.446 / 1.860 | 0.583 / 1.761 | 6.050 / 8.176 | 0.22 | 0.10 | 40 | 4/4 | yes |
| cpp semantic-tokens | 0.059 / 0.080 | 0.046 / 0.059 | 0.060 / 0.081 | 0.72 | 0.75 | 160 | 4/4 | yes |
| cpp signature-help | 0.046 / 0.069 | 0.045 / 0.062 | 0.077 / 0.097 | 0.63 | 0.59 | 480 | 4/4 | yes |
| cpp switch-file | 0.385 / 0.497 | 0.407 / 0.724 | 1.377 / 1.587 | 0.46 | 0.30 | 160 | 4/4 | yes |
| cpp typo-error-cold | 209.751 / 217.997 | 209.130 / 212.677 | - | - | - | 4 | - | (no gopls row) |
| cpp typo-error-warm | 36.758 / 37.551 | 35.657 / 36.796 | - | - | - | 4 | - | (no gopls row) |
| cpp typo-first-cold | 0.466 / 0.522 | 0.384 / 0.635 | - | - | - | 4 | - | (no gopls row) |
| cpp typo-first-warm | 0.421 / 0.704 | 0.381 / 0.401 | - | - | - | 4 | - | (no gopls row) |
| cpp workspace-symbol | 0.179 / 0.243 | 0.186 / 0.229 | 1.726 / 2.299 | 0.10 | 0.11 | 240 | 4/4 | yes |

132 rows win, 7 do not, and 6 have no gopls to compare with. The 7 are listed below with what was
observed; the causes are hypotheses (see "What was tried and not applied"). The user's bar is that gocpp
beats gopls on every row, so these 7 are open, not accepted.
Across the campaigns (`F1`, `F2`, `F3`, `F8` and `F10`, each measured the final or an intermediate build) a
row that does not win in `F10` wins in at least one other, except `large references-popular`:

| row | F10 p95 ratio | the p95 ratio in the other campaigns | what it is |
|---|---|---|---|
| large references-popular | 1.44 | 1.56 (F1), 1.34 (F3), 0.65 (F8) | a real loss, not noise: 0 of 4 rounds won in `F10`, and an independent run of all the rows had a p95 ratio of 1.79 (the row run alone wins, 0.64). 1 to 4 of the 12 requests of a round take 8.7 to 20.9 ms against 3.9 for the others; I suspect a collector cycle (no `gctrace` to show it). `F8`, which had fewer rows before it, had no slow request in the window. gopls takes 6.1 ms for every request, gocpp 4.0 ms for the others (p50 0.65 of gopls). |
| large rename-unexported | 9.53 | 0.43 (F2), 14.63 (F3), 0.49 (F8) | one round has slow requests (9.2 ms, then 0.7 to 4.5 ms for seven requests of 0.07), possibly a collector cycle. p50 0.41 of gopls. |
| large rename-import-alias | 7.63 | 8.87 (F2), 1.19 (F3), 11.34 (F8) | the same kind of tail; p50 0.52 of gopls (0.30 to 0.50 depending on the campaign). |
| large document-symbol | 2.09 | 0.38 (F1), 0.41 (F3) | one round has slow requests (1.7 to 3.2 ms for 80 requests of 0.04). |
| large keystroke-imported-new | 1.00 | 0.92 (F1), 0.87 (F3) | the p95 of the typing row, 2.37 against 2.36, at the edge of the rule; p50 0.88. It won in `F1` and `F3`. |
| cpp keystroke-imported-new | 1.09 | 0.86 (F1), 0.81 (F3) | the same. |
| cpp references-popular | 0.98 | 0.60 (F1), 0.57 (F3), 0.59 (F8) | one slow round (0.74 against 0.35). |

The share of requests over three times and 1 ms above their row's median, on `large` (all rows), is 0.67%
for the final build (33 of 4,912) against 0.90% for the base (55 of 6,140) and 4.0% for gopls (245 of
6,140): gocpp has far fewer spikes than gopls, but a row of 12 to 80 requests per round has a p95 that
one spike sets, and the rule asks for it. That is the honest gap: the tail rows of `large` are won or
lost by where the slow requests land (I suspect the collector, unproven), and `references-popular` loses
in most campaigns, including in an independent run.

The rows of the forms this job added or fixed, before and after (from the table above, p50 / p95):
`rename-iface-method` small 5.49 / 13.6 to 0.54 / 0.60 (gopls 0.80 / 0.89), medium 37.9 / 44.5 to 4.05 / 4.39
(gopls 5.13 / 9.21), cpp 13.8 / 17.2 to 1.30 / 1.63; `rename-var` small 0.85 / 1.73 to 0.19 / 0.26, medium
2.66 / 3.48 to 1.55 / 5.02 (gopls 17.0 / 22.9); `rename-const` small 0.71 / 1.40 to 0.10 / 0.14;
`rename-unexported` small 0.66 / 0.73 to 0.064 / 0.076; `rename-import-alias` small 2.55 / 2.85 to 0.081 /
0.166; `rename-generic-func` small 2.52 / 2.87 to 0.116 / 0.167; `rename-generic-type` small 2.60 / 3.48 to
0.125 / 0.149. `format-1k` p50 / p95 on `small` 3.21 / 3.72 to 2.99 / 3.40 (gopls 3.69 / 4.05).

Memory (median resident at the end of a run, MiB): small 148 base, 145 final, gopls 411; medium 222, 235,
419; large 379, 389, 658; cpp 180, 175, 333. The final build has not moved the memory.

### Projects macro and bigpkg (campaigns F4, F5, F6, build e476d056)

Every row of `macro` that has a gopls row wins (the macro rows `hover-macro`, `hover-bang`,
`inlay-hints-bang`, `semantic-tokens-macro`, `completion-macro` and `code-action` are gocpp's own: gopls
cannot read the file). `bigpkg` has no gopls row; its typing rows are the ones of ADR-119 and unchanged
(`typing-clean` 84.5 / 87.2 ms against the budget of 100 ms; `typing-broken-hover` p95 18 to 20 ms; no
gopls row for either). The formatting and token rows of `macro` in this table are of the build before the
printer and token changes of `F7` and `F10`.

| row | gocpp before p50 / p95 ms | gocpp after p50 / p95 ms | gopls p50 / p95 ms | p95 ratio | p50 ratio | n | rounds won | wins |
|---|---|---|---|---|---|---|---|---|
| macro api-change | 0.403 / 0.988 | 0.240 / 0.358 | 32.489 / 33.197 | 0.01 | 0.01 | 32 | 4/4 | yes |
| macro api-fix | 0.414 / 0.640 | 0.200 / 0.413 | 32.530 / 34.323 | 0.01 | 0.01 | 32 | 4/4 | yes |
| macro code-action | 0.114 / 0.245 | 0.119 / 0.147 | - | - | - | 320 | - | (no gopls row) |
| macro completion-dot | 0.095 / 0.112 | 0.094 / 0.117 | 0.390 / 0.537 | 0.22 | 0.24 | 480 | 4/4 | yes |
| macro completion-edit | 2.209 / 2.556 | 2.134 / 2.521 | 2.850 / 3.596 | 0.70 | 0.75 | 240 | 4/4 | yes |
| macro completion-macro | 3.241 / 4.160 | 3.242 / 3.705 | - | - | - | 240 | - | (no gopls row) |
| macro definition | 0.037 / 0.043 | 0.037 / 0.044 | 0.063 / 0.088 | 0.50 | 0.58 | 480 | 4/4 | yes |
| macro document-symbol | 0.058 / 0.071 | 0.057 / 0.065 | 0.145 / 0.173 | 0.38 | 0.39 | 320 | 4/4 | yes |
| macro format-1k | 3.427 / 3.718 | 3.337 / 3.742 | 3.739 / 4.248 | 0.88 | 0.89 | 120 | 4/4 | yes |
| macro hover | 0.050 / 0.146 | 0.052 / 0.149 | 0.199 / 6.883 | 0.02 | 0.26 | 480 | 4/4 | yes |
| macro hover-bang | 0.070 / 0.116 | 0.072 / 0.119 | - | - | - | 480 | - | (no gopls row) |
| macro hover-macro | 0.049 / 0.068 | 0.052 / 0.082 | - | - | - | 480 | - | (no gopls row) |
| macro inlay-hints-bang | 0.102 / 0.114 | 0.104 / 0.143 | - | - | - | 320 | - | (no gopls row) |
| macro keystroke | 0.304 / 0.373 | 0.309 / 0.417 | 0.840 / 1.020 | 0.41 | 0.37 | 320 | 4/4 | yes |
| macro keystroke-imported | 0.297 / 0.399 | 0.284 / 0.379 | 0.829 / 0.926 | 0.41 | 0.34 | 160 | 4/4 | yes |
| macro keystroke-imported-new | 1.913 / 2.141 | 1.784 / 2.046 | 2.170 / 2.453 | 0.83 | 0.82 | 160 | 4/4 | yes |
| macro keystroke-new | 1.877 / 2.100 | 1.812 / 2.106 | 2.146 / 2.428 | 0.87 | 0.84 | 320 | 4/4 | yes |
| macro open-cold | 61.839 / 62.180 | 62.031 / 62.878 | 178.773 / 194.571 | 0.32 | 0.35 | 4 | 4/4 | yes |
| macro open-warm | 27.419 / 27.993 | 27.785 / 28.123 | 44.945 / 47.207 | 0.60 | 0.62 | 4 | 4/4 | yes |
| macro references-family | 0.084 / 0.106 | 0.084 / 0.112 | 0.411 / 0.463 | 0.24 | 0.20 | 64 | 4/4 | yes |
| macro references-local | 0.076 / 0.096 | 0.072 / 0.089 | 0.213 / 0.235 | 0.38 | 0.34 | 160 | 4/4 | yes |
| macro references-popular | 0.152 / 0.178 | 0.143 / 0.212 | 0.323 / 0.375 | 0.57 | 0.44 | 48 | 4/4 | yes |
| macro rename | 0.126 / 0.162 | 0.129 / 0.169 | 0.332 / 0.416 | 0.41 | 0.39 | 40 | 4/4 | yes |
| macro rename-const | 0.763 / 0.941 | 0.123 / 0.150 | 2.994 / 3.296 | 0.05 | 0.04 | 40 | 4/4 | yes |
| macro rename-fanout | 0.918 / 1.079 | 0.221 / 0.257 | 3.304 / 3.744 | 0.07 | 0.07 | 40 | 4/4 | yes |
| macro rename-field | 0.127 / 0.148 | 0.127 / 0.155 | 0.292 / 0.344 | 0.45 | 0.44 | 40 | 4/4 | yes |
| macro rename-iface-method | 2.964 / 3.123 | 0.393 / 0.451 | 0.527 / 0.695 | 0.65 | 0.75 | 40 | 4/4 | yes |
| macro rename-local | 0.100 / 0.116 | 0.098 / 0.115 | 0.276 / 0.318 | 0.36 | 0.35 | 40 | 4/4 | yes |
| macro rename-method | 0.210 / 0.242 | 0.205 / 0.246 | 0.475 / 0.549 | 0.45 | 0.43 | 40 | 4/4 | yes |
| macro rename-type | 0.137 / 0.161 | 0.139 / 0.164 | 0.367 / 0.417 | 0.39 | 0.38 | 40 | 4/4 | yes |
| macro rename-unexported | 0.716 / 0.855 | 0.074 / 0.134 | 0.186 / 0.303 | 0.44 | 0.40 | 40 | 4/4 | yes |
| macro rename-var | 0.971 / 1.546 | 0.258 / 1.690 | 3.332 / 4.167 | 0.41 | 0.08 | 40 | 4/4 | yes |
| macro semantic-tokens | 0.058 / 0.065 | 0.057 / 0.068 | 0.063 / 0.080 | 0.86 | 0.92 | 160 | 4/4 | yes |
| macro semantic-tokens-macro | 0.043 / 0.047 | 0.044 / 0.053 | - | - | - | 160 | - | (no gopls row) |
| macro signature-help | 0.047 / 0.091 | 0.046 / 0.073 | 0.073 / 0.102 | 0.72 | 0.63 | 480 | 4/4 | yes |
| macro switch-file | 0.410 / 0.587 | 0.402 / 0.572 | 1.331 / 1.584 | 0.36 | 0.30 | 160 | 4/4 | yes |
| macro typing-broken | 1.361 / 1.540 | 1.345 / 1.513 | - | - | - | 792 | - | (no gopls row) |
| macro typing-broken-hover | 0.670 / 3.248 | 0.664 / 3.233 | - | - | - | 792 | - | (no gopls row) |
| macro typing-broken-reqs | 1.351 / 1.587 | 1.341 / 1.573 | - | - | - | 792 | - | (no gopls row) |
| macro typing-clean | 7.458 / 8.338 | 7.450 / 8.934 | - | - | - | 792 | - | (no gopls row) |
| macro workspace-symbol | 0.147 / 0.200 | 0.147 / 0.209 | 1.677 / 2.749 | 0.08 | 0.09 | 240 | 4/4 | yes |
| bigpkg open-cold | 54.129 / 55.492 | 54.492 / 54.572 | 172.212 / 177.095 | 0.31 | 0.32 | 4 | 4/4 | yes |
| bigpkg open-warm | 25.399 / 25.530 | 25.265 / 25.553 | 43.599 / 43.940 | 0.58 | 0.58 | 4 | 4/4 | yes |
| bigpkg typing-broken | 3.874 / 5.526 | 3.861 / 6.312 | - | - | - | 792 | - | (no gopls row) |
| bigpkg typing-broken-hover | 1.080 / 18.119 | 1.090 / 20.250 | - | - | - | 792 | - | (no gopls row) |
| bigpkg typing-broken-reqs | 4.106 / 7.617 | 4.106 / 7.601 | - | - | - | 792 | - | (no gopls row) |
| bigpkg typing-clean | 84.674 / 87.358 | 84.499 / 87.217 | - | - | - | 792 | - | (no gopls row) |

## Tests and mutation checks

Every proof is tested against the full validator: the same rename is asked with `recheckAllImporters`
set (the scratch check) and without, the answers must be equal, and a counter of the pipeline's package
checks says whether the proof held (`proved`: no check) or the form fell back (a check). One behavior per
test, no time bound.

- `renamefast_test.go`: table cases for the variable, the constant, the generic function and type and
  the name that is not exported (each proved, and each refused when the new name is taken, hidden,
  captured, `main`/`init`/`C`, a macro name, or when a conversion or an importer by dot would see it),
  `TestHasMacroName` (a `#` in a string, a comment and a rune is no macro name; `#mac.Name` is),
  `TestImportNameProofChecksTheFile` and `TestImportNameProofNeedsEveryOccurrence`. The import-name
  rows of `TestRenameDeclaringBindingProof` include a test file that declares the new name as a
  variable, a function, a type and a constant (refused, as by the full validator), and a package that
  has a test file or an external test file (falls back to the check). `TestImportNameProofChecksTheFile`
  has the test-file rows and a dot import that brings the new name, which only the file scope lookup
  refuses. `TestRenameProofSweepImportNames` (`renamesweep_test.go`) renames three import names, of a
  package with test files and without, to names that the package, its internal and external test files,
  the file scope and other packages declare, with the proofs and without, and the answers must be equal.
- `renamemembers_test.go`: the cases of an interface method (proved when alike; refused for an assertion,
  for a new name that the interface has, for a tie whose end is not in the family) and
  `TestTieIsRenamedAlike`.
- `protocol`: `TestWriteParts`, `TestTrustedHeadMakesTheEncodedMessage`, `TestEncodedTokensJSON`;
  `server`: `TestSemanticTokensFullAnswerIsEncodedLikeTheRangeAnswer` (the bytes of the whole file's
  answer are the bytes of the range answer, which encoding/json makes).
- `printer`: six `TestLineTable...` tests, the table against `FileSet.PositionFor` for every offset
  forward and backward, across two files, with a `//line` directive and without a file.

Mutations: each mechanism was broken by a scratch edit (a condition or a case disabled with `&& false`,
a value changed), the focused tests run, and the edit undone; a test that does not fail for its mechanism
was rewritten until it does. The line table (the successor test, the column, the file range, the last
line index) and the encoded tokens (a number changed) were caught. The three guards that survived their
mutation in the first version were each decided in review. The `sym.scope == scopeLocal` condition of
the proof of a name that is not exported was dropped: `setScope` and `packageLevel` imply it.
`fileScope.Lookup(newName)` of the import proof stays, and the dot-import row of
`TestImportNameProofChecksTheFile` isolates it (the other guards accept that form; removing the lookup
fails the row). The test of the new name against the interface's ties in `mayBreak` stays: it is behind
`checkEmbedders` in the server, and `TestRenamePromotionAmbiguity` runs with `skipEmbedderCheck` to
isolate it (removing it fails its three skipped rows). The new guard of the import proof (a package that
has test files) and the extension of `checkTestDecls` were mutated too: without the guard five tests
fail, and without the extension the four rows of a test file that declares the name fail.
One mutation of the line table (the fast path
`starts[i+1] <= offset` made `<`) is equivalent: it only sends the lookup to the binary search.

## The two sweep tests that failed on origin/dev

`TestCompletionSweep` and `TestEditorFeaturesSweep` failed on origin/dev independent of this job. The causes
were in the tests, not the server: the member completion answers with a `memberCompletionList` and the
test asserted `*protocol.CompletionList` only; and `checkTokenOrder` failed on the `null` tokens that the
server answers for a `_test.gp` file and a file its build constraint leaves out. The job `dev.testhealth`
fixed both on dev (`completionListOf`, `fileHasTokens`); the fixes of this job were dropped for dev's.

## Consequences and what is left

- The rename of a method (no interface) and of a field of an exported-only struct, an alias, and a
  method of a generic type still re-check the packages when the proofs above do not apply: the rows win
  (the check is 1 to 4 ms and gopls takes 0.4 to 16), but they are not 0.1 ms rows.
- `large` tail rows (7 rows of the table do not win; the bar is every row) are probably the collector;
  I have no `gctrace` of them, and the memory limit and idle collection experiments did not lower the
  spikes. Options that I did not take because they trade memory or soundness for a tail of one row: a memo of the result of a references search scoped to the snapshot (the
  next references request of the same symbol; the hits hold checked ASTs, so the memo has to be dropped
  with the snapshot), and a smaller live heap (the checked packages' `types.Info` is most of it).
- The cold open and the cold typed probes (`open-cold`, `open-cpp-cold`, `typo-error-cold`) were measured
  and are at 0.3 to 0.5 of gopls where gopls has a row, and not changed by this job.
- `semantic-tokens` is a row of 0.03 to 0.05 ms, which is mostly the request and the answer over the
  pipe: the base was within 10% of gopls in p50 (and the build before the token change lost the rule on
  `medium` in `F7`: p95 ratio 1.24, 3 of 6 rounds), the final build is at 0.72 to 0.77 of gopls on every
  project in `F10`.
