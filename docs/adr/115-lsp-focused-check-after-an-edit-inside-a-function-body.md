# ADR-115: Language server speed, round 7: the focused check of an edit inside a function body

Status: accepted (2026-10-20)

## Context

[ADR-112](112-lsp-speed-round-6-the-miss-path-the-rename-check-and-memory.md) made a check that misses
cheaper, but the rows with text that was never seen (`keystroke-new`, `keystroke-imported-new`,
`completion-edit`) still lost to gopls v0.21.1 on `medium` (about 31,000 lines): p95 6.57, 5.76 and 7.69 ms
against 3.36, 3.42 and 3.97 ms (the numbers at the start of step 11.14). The user asked for the server to be
as fast as it can be, with a row-by-row comparison to gopls. The cost of a keystroke was the check of the
whole package (about 3.9 ms at the cache level on the hot package of `medium`: 10 files, 32 KB), and the
micro-optimisations alone could not remove it.

## Measurement provenance

- Method: `fresh-edits-v2-go1.25` (ADR-112), `lspbench` scale 2, 1 run per invocation, the arms in rotating
  order, each run under the perf lock `gocpp-resume.rPhZsx/perf.lock`, `taskset -c 16-23`, `GOMAXPROCS=8`.
  gopls ran with `diagnosticsDelay` 30 ms (lspbench's `-gopls-delay`; gopls's own default is 1 s, which
  is a wait, not work). The keystroke scenarios wait 25 ms between keystrokes and the completion scenario
  80 ms (think time), both less than the 100 ms of the settle delay: **the full check that settles an edit
  never runs inside a measured loop**, so its CPU contention with the next keystroke is not measured. It runs
  between scenarios.
- Batch G (the one in **Results**): 4 rounds of 6 arms, `gocpp.base` and the candidate with
  `GOCPP_GATE_PHASE=15` (the convention of the benchmarks: all gates open), the candidate with
  `GOCPP_LSP_NOFOCUS=1`, gopls, and `gocpp.base` and the candidate at the product's default phase
  (`GOCPP_GATE_PHASE` unset: phase 5). Batch F (earlier, 6 rounds of base, a build of `6f1e5e3b` and
  gopls) agrees with it and is quoted where it matters.
- Binaries: base is the build of the tree before step 11.14 (`732dc4bb`, sha256 `63bc18a2...`), the candidate
  is the build of `bb2ecfa6` (before the later eligibility and regression-test fixes), gopls
  is `/nix/store/...-gopls-0.21.1` (golang.org/x/tools/gopls v0.21.1), go1.25.14. The project and
  configuration fingerprints are in the raw JSON files of the runs (`G.*.json`, `F.*.json`). These are
  historical measurements of those builds, not a new measurement of the later dev merge or review-test
  changes. The original JSONs were retained and their sample counts and percentiles recomputed at resume.
- Reply accounting: every scenario of every run had 0 errors and 0 failures (`Err` empty, `Fail` 0), in both
  batches. The `lspdiff` run below counted 5,097 successful replies and 753 explicit JSON-RPC refusals on
  `small`, and 5,189 and 715 on `medium`, for both builds alike, and 0 failed replies. Refusals are not
  counted as wins.
- A disturbed round: in round 4 of batch G the machine was busy with other work (the p50 of the base's
  `completion-edit` was 13 ms against 5.7 to 6.3 in the other rounds, and its p95 17 ms). The tables give
  the pooled figures of rounds 1 to 3 and of all four rounds. The choice of the rounds to leave out was made
  after the run, from this evidence, and both are shown.

## Equivalence: what is compared, and what is not

- `lspdiff -a base -b candidate`: 6,312 answers on `small` and 6,366 on `medium`, 0 differ (hover,
  definition, references, highlights, rename, completion, signature help, symbols, tokens, formatting, links,
  code actions, hints, workspace symbols and the diagnostics of every open file, before and after 24 edits
  of the hot file). **It does not compare the quick publish or the focused completion**: it waits 150 ms after
  each edit, longer than the settle delay (100 ms), so the full check has replaced them when it asks. Its
  limits from ADR-112 hold too (diagnostics compared by severity, start line and message).
- `TestDiagnosticsQuickMatchesFullRun` (`internal/lsp/server/diagnostics_diff_test.go`) is the test of the
  quick publish: a server with the focused check and one without get the same scripts of edits (statements
  at markers in the bodies, reverts, no-op version bumps, new and renamed functions, new parameters, over
  four files of two packages), one step at a time, and the **first** publish of every edited version must be
  the same (25 seeds of 18 steps at each of the phases 15 and 5: 900 first publishes). It also asserts
  that the enabled server makes focused checks and the disabled one makes none; the enabled count
  depends on scheduling. After the full run that settles the session the last publish of every file
  must be the same too. A review of this step ran the same kind of script with 80 seeds and 1,440
  steps and found no difference after the no-op version bumps (a finding below) were excluded.
- `TestDiagnosticsQuickLineMappings` compares the first edited-version publish and the diagnostic
  state of every published URI with a no-focus server. At phases 5 and 15, both line and block
  directives are tested in the edited file and in another file mapping errors into it. A later
  settle run cannot conceal a missing diagnostic. `TestFocusedLineDirectiveEarlier` now requires
  the conservative decline and asserts that the full check reports at the logical filename; its
  earlier physical-path-only comparison could compare two empty lists and missed this bug.
- `TestCompletionFocusedMatchesFull` compares the completions of the focused check with those of the full
  check at 10 places, at both phases. The cache tests compare the diagnostics of the edited file with the
  full check's, and check that the full check leaves the other files' diagnostics as they were: a sweep over
  generated modules, the examples and the programs of `tests/` (`TestFocusedSweep*`: 335/369, 149/273 and
  1,755/4,326 edits answered; every answer equal), and a matrix of 43 edits with the constructs that the gates
  report (go, chan, defer, pointers, maps, complex numbers, `#func`, `#file`, `#line`; `TestFocusedMatrix`),
  all of them at phase 15 and at phase 5, where the gates of the later phases fire (the product's default;
  `testConfig()` alone has the last phase, where no gate fires). The sweep counts are from before the
  conservative line-mapping decline was added; those sweeps were not repeated for that narrow fix.
- Bounded evidence, not a proof of equivalence.

## Decisions

1. **A focused check answers an edit inside the body of one function** (`internal/lsp/cache/focus.go`,
   `check.Config.Focus`). The server publishes the diagnostics of the edited file from it at once, and
   after 100 ms without an edit (`diagnostics.settle`) it runs the full check and republishes anything that
   differs. The focused check strips the bodies of the other files' functions (not `init`), and the gates
   look at the edited file only; the types of everything else are the ones of the last full check. The full
   check always follows (speculate, then verify), so a gap in the proofs shows as a late correction, not as
   a state that stays.
2. **The focused check declines unless it can prove that the answer is the full check's.**
   - once per package (`focusEligible`): not a variant, no import cycle, no generics anywhere (also a generic
     type), no `!` form or must function, macro or derive, no C or C++ file, no directive, include or
     namespace rename, no probe record, no load error, and no `//line` or `/*line` mapping in any
     package file;
   - at each request: the environment (configuration, module, file set, what the metadata read, the API
     hash of every dependency) equals the base's (`TestFocusEnvHashesWhatItNames` changes each part); exactly
     one file differs; the changed region lies strictly inside one function body (`findBodyEdit`, with the
     boundaries tested; windows with `//line` decline); the body parses alone (`bodyParses`, see 7); after the
     load the declaration count and the brace offsets (shifted by the length delta) are the base's; no
     `Leftovers`;
   - after the check: no package-level initializer that uses a package function or variable (a body could
     add an initialization-order edge).

   Targeted tests cover the guards called out in review; this is not exhaustive mutation coverage.
   Removing the generic-type check, metadata digest, brace checks, early syntax check, or `SkipBodies`
   wiring fails its focused tests. The quick-publish guards are covered by their dedicated version,
   related-position, broken-overlay and replaced-base tests, not by the random session test alone.
   `TestFocusEligibleCheckRewrites` isolates the base's `Must` and `Consts` guards with synthetic check
   results over otherwise eligible source, and removing either guard fails its subtest. This does not
   claim that those synthetic combinations arise from user source.

   The second review found that existing line mappings break the physical-file assumption: an
   unchanged body can report at the edited file's logical URI, or the edited body can report at a
   virtual URI. Filtering the focused diagnostics by physical filename can then erase or omit an
   error until settle. The base now declines if any of its parsed comments has a scanner-shaped
   line directive (`hasLineDirective`); `//line` must start at physical column 1, and `/*line` may
   appear anywhere. This uses parsed comments and unadjusted positions, not matches inside string
   literals. Malformed directive-shaped comments and identity mappings also conservatively decline.
   The scan is cached with the base's eligibility and examines only that package's files, not the
   whole workspace. Preserving mapping provenance through partial checks is deferred.

   Redundant guards were removed: the probe record of the focused check (a plain package asks for no
   probe: `plainPackage` has no C, C++ or directive), the second check of `Must` and `Consts` and of
   generics after the check (the first, on the base, is what the edit cannot change), the comparison of
   the function name (the text before the change is the same, so the name is) and the variant test of
   `Snapshot.Focused` (it asks for the folder's handle, never a variant).
3. **Completion uses the same check** (`Server.focusedQuery`) when its snapshot has a base, so the completion
   after an edit does not wait for the full check. One focused package is memoized per snapshot and base.
4. **The quick publish declines in two more cases**, which a review of this step found:
   - another open file of the package has a version that the client has no publish for (a didChange that
     only raised the version of a file, with the same text): the quick publish of the edited file alone would
     leave the client with the old version of the other one, and a client that drops diagnostics of an older
     version would show none until the settle run. The full run publishes every open file at its version.
     (`versionsMoved`, `TestDiagnosticsQuickVersionOnlyChange`);
   - a diagnostic of another file has a related place in the edited file (`other declaration of x` at
     `a.gp:12`): an edit that adds a line to the edited file moves it, and the quick publish does not send
     the other files. (`relatesTo`, `TestDiagnosticsQuickMovedRelatedPlace`). It declines whether or not
     the line count changes: the case is rare (a duplicate declaration across files) and the full run is the
     cost of any edit without the focused check.
   - A third guard: the overlay is not Broken, and the base is still the one the run was made against
     (`TestDiagnosticsQuickBrokenOverlay`, `TestDiagnosticsQuickBaseReplaced`).
5. **Parser patch 8: `parser.SkipFuncBodies`** (`internal/syntax/patches/0008-skip-func-bodies.patch`) and
   `load.Config.SkipBodies`: a file that the focused check does not look at is scanned to the matching `}` of
   each function body and no nodes are made for it (`init` and macros keep theirs). Positions after the body
   are those of a full parse (the tokens are still scanned). The focused check's allocations per check went
   from 24,062 to 16,992 (`BenchmarkFocusedMedium`: median 1.94 to 1.77 ms). `stripOtherBodies` stays as the
   check's own guarantee (it is idempotent with the parser mode). `TestFocusedLoadSkipsOtherBodies` sees the
   loaded program before the check and shows the wiring (the bodies of the other files are missing there).
6. **Smaller wins that every check gets**:
   - gate elision (`typesGated`, `mayHaveMemberRule`): the walk of the types and the member-rule search are
     left out when no feature of a type is gated at the phase and no row can name the object
     (`TestGateElision`: the gate errors of the repository's programs are the same with and without, at
     phases 5, 8, 9 and 15; `TestTypeUseFeaturesComplete` keeps the feature list complete). **It helps only
     with the gates open** (the benchmarks' `GOCPP_GATE_PHASE=15`); at the product's default phase (5) a
     type feature is gated and the walk of the types stays;
   - the per-file steps of the load (reserved names, directives, rewrites) run together
     (`load/parsepar.go`, `forEachFile`; errors keep the order of the files: `foreach_test.go`);
   - `types.Info` and the opaque pass's maps are presized from the size of the source.
7. **A syntax error declines before the load.** Mid-typing, the text often has one. Previously the
   attempted focused check plus the fallback full check cost 1.054 ms, versus 0.670–0.673 ms for the
   full check alone (about +57 %, `BenchmarkFocusedDeclineMedium` against
   `BenchmarkFocusedDeclineFullMedium`, cache level, `medium`). The body of the edited function is now
   parsed alone (a few hundred bytes) and a syntax error declines: attempt plus fallback 0.688 ms,
   about +3 %, with 18,454 versus 28,516 allocations per operation before the change. These are medians
   of six alternating 1,000-operation runs, pinned to CPUs 16–23 with `GOMAXPROCS=8` and the shared perf
   lock. On the successful path, `BenchmarkFocusedMedium` was 2.052 versus 2.041 ms. Parsing the whole
   edited file early was tried and rejected: 2.154 versus 2.029 ms on that successful path. The check
   on the whole file is left to the load; the early test only declines (a body that parses alone but
   not in the file is not declined early, and the load declines it).
8. **Switches**: `GOCPP_LSP_NOFOCUS=1` turns the focused check off, so that the same binary can be compared
   with and without it (the equivalence tests use it). It is listed by `gocpp env` and in the editors guide.

## Results

Batch G, `medium`, p50 / p95 in ms, pooled over rounds 1 to 3 (n = 240, 120 and 180) and, in parentheses,
over all four rounds (n = 320, 160 and 240):

| row | base | candidate, no focus | candidate | gopls | candidate at phase 5 (all four rounds) |
|---|--:|--:|--:|--:|--:|
| keystroke-new | 4.79 / 6.82 (4.96 / 7.79) | 4.24 / 5.64 (4.43 / 7.10) | 2.35 / 3.63 (2.45 / 4.40) | 3.11 / 3.94 (3.03 / 3.94) | 2.36 / 2.93 |
| keystroke-imported-new | 4.57 / 6.02 (4.88 / 6.38) | 4.19 / 8.46 (4.32 / 8.33) | 2.43 / 2.82 (2.47 / 3.84) | 2.93 / 3.73 (2.89 / 3.69) | 2.42 / 3.04 |
| completion-edit | 5.66 / 14.73 (5.79 / 15.72) | 5.12 / 7.61 (5.17 / 8.22) | 2.67 / 4.27 (2.79 / 4.38) | 3.81 / 5.11 (3.83 / 5.11) | 2.78 / 4.57 |

`small` (22 files, 3,900 lines; not the ticket's row but measured alongside), p95 in ms, rounds 1 to 3 (all
four rounds):

| row | base | candidate | gopls | candidate at phase 5 (all four rounds) |
|---|--:|--:|--:|--:|
| keystroke-new | 5.43 (5.54) | 2.67 (5.22) | 3.24 (3.14) | 2.94 |
| keystroke-imported-new | 5.67 (5.57) | 2.57 (5.07) | 2.92 (2.87) | 3.10 |
| completion-edit | 5.77 (7.73) | 3.16 (3.98) | 3.97 (3.93) | 3.30 |

What this shows, and what it does not:

- The focused check is what wins: the same candidate binary without it (`GOCPP_LSP_NOFOCUS=1`) is at 4.2 to
  5.1 ms p50, close to the base.
- **p50: the candidate is faster than gopls on every row**, on both projects, at both phases.
- **p95, undisturbed rounds (1 to 3): faster than gopls on every row** of both projects, with the candidate at
  the gate phase 15. At the product's default phase (5), faster on every `medium` row, with a margin of 4 % on
  `completion-edit` (4.91 against 5.11 ms for rounds 1 to 3; all-round phase-5 values are in the table),
  and on `small` faster on `keystroke-new` and `completion-edit`;
  **`keystroke-imported-new` is 3.10 against 2.87 (all rounds) and 2.82 against 2.92 (rounds 1 to 3) on
  `small`: not consistently better**.
- **p95, all four rounds pooled: not faster on every row.** The disturbed round puts the candidate's
  `medium` `keystroke-new` at 4.40 against 3.94 ms, and `small` `keystroke-new` and `keystroke-imported-new`
  at 5.2 and 5.1 against 3.1 and 2.9 (one run at 7.4 and 7.8 ms among the per-round p95s of 2.4 to 2.7). The
  per-round p95 medians of `medium` still win (3.47 against 3.78, 2.78 against 3.15, 4.00 against 4.61 ms).
  In batch F (6 rounds, an earlier build of the candidate, no disturbed round) the `medium` p95s were 3.12,
  3.00 and 3.59 against gopls's 3.85, 4.25 and 4.81 ms, and `small` `keystroke-imported-new` was 2.80 against
  2.54 ms: the same row loses in one batch and wins in the other.
- **The goal of consistently beating gopls on every row is not met.** The candidate wins the three
  measured rows at p50 and the undisturbed phase-15 p95 in this batch, but not every all-round p95.
  `small` `keystroke-imported-new` also loses in batch F, in the reviewer's independent run (2.84
  against 2.71 ms), and at the product-default phase in all-round batch G. These measurements do not
  establish statistical equivalence or a consistent win on that row. Garbage collection and scheduling
  are likely contributors to the tail; the `-idle-gc` and `GOGC=200` experiments were not conclusive.
- Resident set (median over the rounds, MiB, batch F): `medium` 190 (base), 167 (candidate), 267 (gopls).

Where the time of a focused check goes now (cache level, ~1.8 ms): go/types 0.8, the opaque pass 0.27,
the load 0.6 (of which the parse is a small part after patch 8), the gates and `must` pass 0.13.

## Tried and not kept

- `-idle-gc` and `GOGC=200`: three alternating rounds each; the differences between them were not larger
  than the spread between runs of the same binary (the `medium` completion-edit p95 was 5.9 ms plain, 3.4 ms
  with `-idle-gc` and 6.5 ms with `GOGC=200` in the first round, and the order changed in the others). Not
  kept.

## Not done

- A focused check for edits that change a declaration (a signature, a type, an import): they take the full
  check as before. Proving them equal to the full check needs the dependency graph of the declarations.
- Packages with generics, macros, `!` forms or C/C++ files are never answered from the focused check; the
  benchmark's `medium` project has none, the sweeps over the repository's programs answered about 40 percent
  of the generated and example edits.
- A measurement in which the settle full check runs inside the loop (think times above 100 ms), and the
  rows of the `large` project, and Windows (the code is portable; the unit tests ran on Linux only: vet
  for GOOS=windows and darwin is clean).
- `TestEditorFeaturesSweep` fails on the dev base too (`semanticTokens/full returned null`); it is not changed
  by this step.
- `internal/syntax/parser/gocpp_macro_name_test.go` at `dev` has two cases that the patches do not produce
  (`update.sh -f` removes them); this is old drift, not changed here.
