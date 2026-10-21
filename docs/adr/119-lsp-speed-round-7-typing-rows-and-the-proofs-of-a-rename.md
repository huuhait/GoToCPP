# ADR-119: Language server speed, round 7: the typing rows and the proofs of the result of a rename

Status: accepted (2026-10-21)

## Context

The user's priority for the language server is that it beats gopls on every measured row. After
[ADR-112](112-lsp-speed-round-6-the-miss-path-the-rename-check-and-memory.md) and
[ADR-115](115-lsp-focused-check-after-an-edit-inside-a-function-body.md) a fresh matched comparison with the
integrated server still showed three kinds of rows that were not won:

- the tail (p95) of the typing rows on the small project (`keystroke-new`, `keystroke-imported-new`,
  `completion-edit`) and, for the repeated-text rows (`keystroke`, `keystroke-imported`), the
  median;
- the rename of a type, a method, a field and a local variable, which the shortcuts of ADR-112 do
  not cover (they are for exported functions, variables and constants): gocpp re-checked the
  packages after the edit, 6 ms on `small` and about 40 ms on `medium`, and gopls answers in 0.3 to
  8 ms;
- the clean typing row of the one big package (`bigpkg`), which has no gopls row and only the budget
  of the README (100 ms from a keystroke to its diagnostics).

## Measurement provenance

Everything below was measured with `lspbench` (protocol method `fresh-edits-v2-go1.25`; the `-think`
and `-settle` pauses add a suffix to the method, so a run with them is another protocol), by one
invocation per (round, arm), the order of the arms rotating by round, gopls an arm of its own, each
under the shared `perf.lock`, on cores 16 to 23 with `GOMAXPROCS=8` (AMD Ryzen Threadripper PRO
9995WX, Linux 6.19.6), Go toolchains as given below, gopls v0.21.1 with `diagnosticsDelay` 30 ms,
`-scale 2` (`-scale 1` for the typing rows: they refuse repeated edits), one run per invocation, and
only when the one-minute load average was under 50 (the measured invocations started at loads of 0.2
to 1.1; the load average is in `invocations.jsonl` of each campaign).

- **base**: the server of the integrated `c137e776` (step 12.13b2), built from `8308744d` (that
  commit adds only the `-think`/`-settle` options of `lspbench`) with go1.25.14; `gocpp` sha256
  `166aa9db...`. The final servers and `lspbench` (every arm) were built with go1.25.7. A base
  rebuilt with go1.25.7 gives the same numbers (the review of this job rebuilt it, sha256
  `8c74fd7c...`: small `keystroke-new` p95 2.402 and 2.387 against 2.553 here, final 2.106 and
  2.117, gopls 2.355 and 2.387), so the toolchain is not what separates the arms.
- **final**: the tree of the round-2 commit of this job (rebased on `a696098f`, built by `build.sh
  v8` from `e60c065a` plus the uncommitted round-2 changes, which this commit contains); `gocpp`
  sha256 `3668fad8...`, `lspbench` sha256 `83ee978e...` for every arm (the same benchmark binary
  measured base, final and gopls). Round 1 measured `b3c68164` (`gocpp` `95818f5a...`, `lspbench`
  `6d3f731d...`, campaign `FIN`): its numbers are within the noise of these for every row.
- The campaigns are `FIN2` (4 rounds, 13 rows on `small` and `medium`, the table below, measured on
  the final build at one-minute loads of 0.1 to 0.5 at the start of each invocation), `SET` (3
  rounds, the typing rows with `-think 160ms -settle 160ms`: the pauses of a person who types, then
  stops) and `BIG` (3 rounds, `bigpkg`), the last two measured on the round-1 build: round 2 changed
  only the rename proofs, and the typing rows of `FIN2` reproduce those of `FIN`. Their JSON (named
  configuration fingerprints, not raw environment) and logs are kept with the job's scratch files.
  Earlier campaigns of the job measured during an overload of the host (load above 5,000, caused by
  other work) and are void; the campaigns `R2` to `R5` measured the intermediate builds. Raw samples
  of all of them are kept; none was replaced.
- The correctness gate (`lspdiff`, final against base, `small` and `medium`) compared 6,312 and
  6,366 answers (hover, definition, references, highlights, rename, completion, signatures, symbols,
  tokens, formatting, links, code actions, hints, workspace symbols and the diagnostics of every
  open file, before and after 24 edits of the hot file and API changes): none differs, and the
  replies are the same in kind (5,097 and 5,189 successful, 753 and 715 JSON-RPC refusals, none
  failed) in both. The comparison of diagnostics keeps severity, start line and message, not the
  columns, end ranges or metadata.

## Decisions

1. **A focused check is cheaper** (the cache's microbenchmark: 26% less per check). The opaque pass
   is left out also when go/types found only errors about an unused import or variable (they leave
   every type valid; `collector.onlyUnused`, `typesMayBeIncomplete`): a focused check always has
   them, for the imports that only the bodies it left out use. The `types.Info` maps are sized by
   the declarations the check sees (`load.File.BodiesSkipped`, `checkedBytes`), not by the size of
   every file of the package.
2. **A base remembers its last three focused checks by the content of the edited file**
   (`focusBase.memo`, keyed by path and content hash, the oldest goes first). A decline that ran
   through the check stays in the memo; only a decline before the promise of a check exists is not
   kept. Typing a character and deleting it, or an undo and a redo, ask for the same check again;
   the per-handle promise that held one check by base is replaced. The repeated-text rows go from
   about 2.5 ms to about 0.3 ms.
3. **The pacer leaves a fixed room above the live heap** (`gcPercentFor`: 96 MiB above the live
   heap, at least GOGC 100 and at most 800%, bounded by the soft limit that the server already sets;
   the user's `GOGC` and `GOMEMLIMIT` are theirs). GOGC 100 collected every dozen keystrokes on a
   small project, and the cycle's assists were the p95 of the typing rows. The room is garbage that
   the next cycle gives back. The percent is set from the live heap that the last cycle left, so
   until the first cycle ends (the live heap is not known) the 800% is in force: a project that
   loads 20 MiB may grow to 180 MiB before its first collection. That is the peak of `bigpkg`, 171
   MiB against 102 to 125 before; it is below the soft limit and is the price of the first seconds,
   not of the steady state.
4. **The result of a rename is proved, not re-checked, for the forms below** (`renamefast.go`,
   `renamemembers.go`). Every proof applies only after the guards that were already there (the
   conflicts, the captures, the members of the type, the embedders, the test declarations) succeeded
   and the search was complete (`incomplete`, `outside` and `dynamic` are empty), and falls back to
   the scratch check when anything is unknown (an error in a package, a macro, `!`, a directive, C
   or C++, a test check, a generic type, an alias). With `recheckAllImporters` the proofs are off,
   which is how the tests ask for the full validator.
   - **A local name** (variable, parameter, result): the block that declares it is all that sees it.
     The proof needs no name of the new name written in its range (a declaration, or a use of a
     variable, constant, type or import: a field, a method or a label is found by selection or key,
     not by lookup), nothing of the new name in the block's own scope, and every occurrence of the
     variable in the checked package an edit. A bare `return` writes no name and finds the results
     of its function by lookup, so a variable hiding a result of the new name is an error at a bare
     return of that function in the variable's range, in a nested block, a loop or beside a closure
     (`bareReturnShadows`): the proof refuses it.
   - **A type** that no field embeds (the embedded field takes the type's name): the proof of an
     exported function extended to a nongeneric defined type, with the importers that use it by a
     qualified name (`qualifiedOnly`) and no dot importer.
   - **A field** of a nongeneric named struct type that no struct or interface embeds, by any name
     or alias, to a name of the same exportedness, and only when the struct has a field that is not
     exported. The field names are part of the identity of a struct type; two named types are never
     assignable, so only a conversion between them depends on it, and a struct written without a
     name (a literal, the target of an alias, a constraint) is assignable and identical to a type of
     the same fields. So the packages searched must have no conversion between two different struct
     types of the field names of the struct or of those it will have, and no unnamed struct type of
     either (`structShapes`, a memo of the checked package). That covers only the packages searched,
     the ones that use the field or import its package; a package that the declaring package does
     not import can write an unnamed struct of the same exported field names and meet the type
     through a third package (the review found a generic constraint, a result, an alias and a slice
     element of such a struct). A name that is not exported is written in the package of the struct,
     so a struct with one is identical to no struct of another package, and the packages that can
     write it are the ones searched. A struct of exported fields only therefore keeps the scratch
     check.
   - **A method** of a named type, when the narrow rule leaves nothing to check: no package
     searched, edited or not, has a name of the new name or a use that needs an interface of the old
     name or the new one (`mayBreak`: the ties of the family), every package that may have a tie was
     searched, and every occurrence of a method of the family is an edit.
5. **The benchmark asks for the forms**: the scenarios `rename-type`, `rename-method`,
   `rename-field` and `rename-local`, and the `-think` and `-settle` pauses (the method scenario
   renames `Collect`, a method of no interface: gopls refuses to rename a method of an interface of
   the project, and a refusal is no timing).

## Results

Median / p95 in milliseconds, pooled over the rounds; in brackets the lowest and highest p95 of the
rounds. `n` is 32 to 40 for the api and rename rows, 160 to 320 for the typing rows, per arm.

| project | row | base (c137e776) | final (round 2) | gopls 0.21.1 |
|---|---|---|---|---|
| small | keystroke | 0.531 / 2.260 (2.22-2.26) | 0.255 / 0.321 (0.30-0.33) | 0.852 / 0.926 (0.92-0.93) |
| small | keystroke-imported | 1.255 / 2.341 (1.98-2.48) | 0.240 / 0.309 (0.28-0.32) | 0.850 / 0.915 (0.90-0.93) |
| small | keystroke-new | 1.861 / 2.543 (2.45-2.69) | 1.829 / 2.150 (2.07-2.31) | 2.140 / 2.304 (2.29-2.31) |
| small | keystroke-imported-new | 1.855 / 2.547 (2.47-2.56) | 1.707 / 2.003 (1.94-2.03) | 2.129 / 2.305 (2.29-2.40) |
| small | completion-edit | 1.975 / 2.790 (2.19-2.85) | 2.013 / 2.295 (2.25-2.36) | 2.737 / 2.987 (2.94-3.00) |
| small | rename (function) | 0.154 / 0.210 (0.19-0.39) | 0.160 / 0.200 (0.20-0.22) | 0.473 / 0.570 (0.57-0.58) |
| small | rename-type | 5.950 / 7.144 (6.83-7.18) | 0.149 / 0.188 (0.17-0.28) | 0.505 / 0.596 (0.54-0.73) |
| small | rename-method | 2.575 / 3.674 (3.54-3.97) | 0.228 / 0.259 (0.25-0.27) | 0.716 / 0.804 (0.78-0.82) |
| small | rename-field | 5.956 / 7.353 (7.20-7.69) | 0.117 / 0.160 (0.13-0.17) | 0.399 / 0.459 (0.43-0.47) |
| small | rename-local | 2.380 / 3.398 (3.22-3.73) | 0.102 / 0.113 (0.11-0.12) | 0.276 / 0.311 (0.31-0.44) |
| small | api-change / api-fix | 0.41 / 1.27, 0.36 / 2.57 | 0.19 / 0.27, 0.18 / 0.23 | 32.45 / 32.66, 32.42 / 32.70 |
| small | switch-file | 0.362 / 0.422 (0.39-0.45) | 0.357 / 0.402 (0.39-0.45) | 1.332 / 1.492 (1.45-1.52) |
| medium | keystroke | 0.539 / 2.628 (2.57-2.65) | 0.312 / 0.385 (0.37-0.40) | 0.957 / 1.093 (1.07-1.10) |
| medium | keystroke-imported | 0.728 / 2.624 (2.45-2.64) | 0.308 / 0.378 (0.37-0.39) | 0.924 / 1.020 (1.01-1.03) |
| medium | keystroke-new | 2.391 / 2.783 (2.69-2.86) | 2.261 / 2.591 (2.57-2.61) | 2.942 / 3.325 (3.25-3.32) |
| medium | keystroke-imported-new | 2.269 / 2.658 (2.48-2.78) | 2.175 / 2.480 (2.37-2.52) | 2.857 / 3.004 (2.98-3.03) |
| medium | completion-edit | 2.461 / 2.965 (2.78-3.05) | 2.526 / 2.885 (2.78-3.08) | 3.662 / 3.995 (3.92-4.04) |
| medium | rename (function) | 0.265 / 0.429 (0.38-0.48) | 0.261 / 0.364 (0.36-0.40) | 0.628 / 0.811 (0.77-3.64) |
| medium | rename-type | 36.625 / 44.254 (42.58-47.38) | 0.243 / 0.276 (0.26-0.28) | 4.339 / 12.174 (10.25-12.91) |
| medium | rename-method | 3.956 / 4.181 (4.09-4.36) | 0.425 / 0.607 (0.60-0.62) | 4.980 / 7.962 (7.14-10.88) |
| medium | rename-field | 36.715 / 43.712 (42.59-47.19) | 0.241 / 0.338 (0.26-0.37) | 4.203 / 8.771 (7.15-9.42) |
| medium | rename-local | 3.714 / 3.918 (3.83-13.05) | 0.130 / 0.179 (0.14-0.19) | 0.358 / 0.433 (0.42-0.45) |
| medium | api-change / api-fix | 0.68 / 3.06, 0.57 / 3.91 | 0.29 / 0.38, 0.25 / 0.32 | 32.67 / 32.97, 32.89 / 33.04 |
| medium | switch-file | 0.421 / 0.491 (0.48-0.51) | 0.417 / 0.482 (0.46-0.48) | 1.448 / 1.707 (1.67-1.72) |

With the pauses of a person who types (`SET`, 3 rounds, p95 of the rounds in brackets; base / final
/ gopls): small `keystroke-new` 2.438 / 2.091 (2.07-2.09) / 2.496; small `keystroke-imported-new`
2.799 / 2.016 (1.97-2.02) / 2.498; small `completion-edit` 2.798 / 2.177 (2.11-2.19) / 3.099; medium
`keystroke-new` 2.880 / 2.663 (2.58-2.67) / 3.360; medium `keystroke-imported-new` 2.787 / 2.778
(2.37-2.86) / 3.291; medium `completion-edit` 2.825 / 2.882 (2.73-2.94) / 4.085.

`bigpkg` (no gopls row; the README's budget is 100 ms; 3 rounds): `typing-clean` 87.230 / 90.069
before, 84.596 / 86.720 (86.62-87.21) after; `typing-broken` 3.939 / 6.806 before, 3.870 / 5.916
after.

Resident memory (median at the end / peak of the run, `FIN2`): small 139 / 140 MiB final (base 72 /
72, gopls 291 / 328), medium 226 / 236 (base 238 / 239, gopls 258 / 267); `bigpkg` (round-1 build,
`BIG`) 171 / 171 (base 102 / 125). The small project's resident set doubled: it is the room of
decision 3, not live data, and the 800% of the first cycle is the peak of `bigpkg` (decision 3).

In every row above the final server is under gopls in the median and in the p95, in each of the
rounds (the lowest margin is 7% of p95: small `keystroke-new`, 2.150 against 2.304). That is the
claim of this round and no more: see the gaps.

The small `rename` (function) row is a single-sample row: its p95 over a round of 10 samples is the
largest of them. A short campaign that asks for the rename rows only (`-only`, 4 to 7 rounds of 10
samples) showed outliers of 0.5 to 1.4 ms in about half the rounds, in the final build and in the
round-1 build alike, and in single rounds of the base and of gopls in other runs; the cause is not
known (not profiled). A p95 pooled over about 30 samples can therefore rise above gopls's (0.57 to
0.61 ms) on two outliers: in that campaign the pooled p95 of the final build was 0.54 to 0.55
against 0.61 for gopls, and three rounds of six were above it. The campaign `FIN2` above, which measures the
row after the typing rows, shows none (0.19 to 0.22 ms in every round). The row is the one place of
this ADR where the margin to gopls is not established by the rounds; it is established by the pooled
median (0.16 against 0.47 ms).

## Alternatives

- **Re-checking after the rename, faster** (reuse the check of the unedited packages, a cheaper
  scratch snapshot): the scratch check was already as cheap as the cache allows, 6 ms on `small`;
  the gap to gopls is the check itself.
- **Dropping the result check of a rename like gopls** (it answers local, type and field renames in
  under 8 ms because it does not re-check): rejected; the check is why a rename of gocpp never
  leaves an error. Each proof states what the check would have found and when it cannot.
- **A lower debounce for `bigpkg`**: it would break the README's "about 30 ms" and the row has 13%
  margin to its budget as it is: the check is about 58 ms of three or four go/types passes (the `!`
  forms are lowered and the package checked again); fewer passes would need a change of the
  lowering, which is another job.
- **Proving a field without looking at conversions** (names alone): too coarse. The generated
  projects have a struct of the same fields in each file, and the field proof would never apply
  there.
- **Proving a field of a struct of exported fields only, by searching every package of the module**:
  it would cover the packages the declaring package is not imported by, but a package of another
  module of the workspace, or one that is not loaded, can write the struct as well; the proof would
  again say more than it knows. The scratch check is the answer for those.

## Consequences

- A rename of a name of the forms above, with nothing unknown, is answered with no new check, in
  about a tenth of a millisecond on `small`. The answer is the one of the check of the result for
  the packages that the search covers: a sweep renames the names of four generated files, and of a
  small module written to be hard (named results with bare returns in blocks, loops and closures, a
  struct written in a package that the declaring one is not imported by, a struct of exported fields
  only and one with a hidden field, embedded types, interface methods, conversions, a generic
  constraint, an alias), to a fresh name, to another name of the file, and for the hard module to
  each other name of the file, with the proofs and without, and compares the answers (the error text
  up to the use or the embedder that it names, which the checker takes from a map). It does not
  cover a package that the search does not look at (another module of a workspace, a package that
  failed to load); the proofs refuse when anything was not searched, but that is the proof's own
  bookkeeping and is tested, not exhaustively compared. The review of round 1 found two forms of
  this class that the first version got wrong, an unnamed struct from a package that does not import
  the declaring one and a variable hiding a named result at a bare return; both are fixed, tested as
  table tests, and the sweep catches both when the fix is removed.
- A rename that a proof does not cover (a generic type or a method of one, an alias, an embedded
  type, a method tied to an interface, a package with an error, a macro, `!`, C or C++, a test file
  that mentions the name, a name that the module writes elsewhere, a struct of the same fields with
  a conversion) is as slow as before: 2.5 to 6 ms on `small`, up to about 40 ms on `medium`. Those
  rows lose to gopls and were not measured here.
- The proofs are code that must be as right as the check they replace. Each condition has a test
  that the proof alone refuses (the guards are assumed in the direct tests), and the table tests
  compare the answer with and without the proofs, counting package checks. A single mutation of a
  condition that another one repeats survives by design (the embedded field is looked for in the
  hits, in the importers and in the type's own declaration); those are kept: the repetition is the
  defence.
- The small project uses more memory by design (decision 3); a user who sets `GOGC` or `GOMEMLIMIT`
  keeps the setting.

## Gaps

- The comparison is on `small` and `medium` of the generator, on one machine, with one run per
  invocation and 3 or 4 rounds; the rounds are consecutive, so a disturbance of the host would be in
  all of them (the load average was low at the start of every invocation; it was not sampled during
  one).
- The rows of `large`, `cpp` and `macro`, hover, definition, references, completion of a name and
  the cold rows were not measured in this round; the correctness gate covers their answers on
  `small` and `medium`.
- The p95 margin of the small typing rows is 7 to 10% (0.15 to 0.25 ms), which is the size of the
  p95 spread of gopls between rounds on the same row (2.29 to 2.44 ms).
- The rename proofs are exact for the packages that the search covers. A field of a struct of
  exported fields only is not proved (a package that does not import the declaring one can write the
  same struct); it keeps the scratch check, as slow as before (about 7 ms on `small`, 44 ms on
  `medium`). The generated projects' types have unexported fields, so the rows above do not show
  that cost.
- The small `rename` (function) row can show a single outlier of about 1.4 ms in a round (see
  Results).
- `bigpkg`'s clean typing has no gopls row: 86.7 ms p95 against a budget of 100 ms.
- The diagnostics comparison of `lspdiff` keeps severity, start line and message, not columns, end
  ranges or metadata; dependent diagnostics still use fixed settling sleeps (as in ADR-112).
- At `c137e776`, before any change of this round, `TestCompletionSweep` (a type assertion on
  `memberCompletionList`) and `TestEditorFeaturesSweep` (`semanticTokens/full returned null`) of
  `internal/lsp/server` fail; they belong to other steps and were left alone.

## Evidence

- Tests: `internal/check/skipopaque_unused_test.go`, `internal/load/skipbodies_test.go`,
  `internal/lsp/cache/focus_memo_test.go`, `internal/lsp/server/idlegc_test.go`,
  `renamelocal_test.go`, `renamemembers_test.go`, `renamefast_test.go` (the type cases),
  `renamesweep_test.go` (generated project and the hard module), `renamereview_test.go` (the forms
  that the review found, each next to the case that stays proved); the benchmark's scenario tests in
  `internal/lsp/bench`. They count operations (package checks, focused checks, the percent and limit
  that were set), never time.
- Mutation checks, each in a scratch copy of the file with the focused tests run, and the result
  recorded: the unused-only rule, the sizing of the maps, `BodiesSkipped`, the memo key (path,
  hash), its size and its eviction, `gcPercentFor` (headroom, bound, zero, floor), the user's
  `GOGC`/`GOMEMLIMIT`, every condition of the local, type, field and method proofs. Killed unless
  said above (repeated conditions). Round 2: the exported-only rule of the field proof (killed by
  the table tests and by the sweep), the call of `bareReturnShadows` (killed by the table tests and
  by the sweep), its three conditions (the closure skip, the range, the bare return: killed; the
  closure skip needed a positive case, a closure without results that returns bare, which the tests
  now have), `cv.noOpaque` in the clean rule (killed by the C interop test) and the set of codes of
  `onlyUnused` (killed by the unused label test).
- `lspdiff` base against final (re-run on the round-2 build): `small` 6,312 answers and `medium`
  6,366 answers, 0 differ.

## See

[Phase 11](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md) (the language server's speed rows),
[docs/benchmarks/lsp.md](../benchmarks/lsp.md) (its tables predate this round),
[ADR-112](112-lsp-speed-round-6-the-miss-path-the-rename-check-and-memory.md),
[ADR-115](115-lsp-focused-check-after-an-edit-inside-a-function-body.md).
