# ADR-115: Language server speed, round 7: the focused check of an edit inside a function body

Status: accepted (2026-10-20); amended 2026-10-23 (packages with generic declarations)

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
   - once per package (`focusEligible`): not a variant, no import cycle, generics only under the conditions
     of the amendment below (until 2026-10-23: no generics anywhere, also a generic type), no `!` form or must function, macro or derive, no C or C++ file, no directive, include or
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
   Removing the metadata digest, brace checks, early syntax check, or `SkipBodies` wiring fails its
   focused tests (and, since the amendment below, the decline of instantiations with an edge of
   weight 1 in the base or in the focused check). The quick-publish guards are covered by their dedicated version,
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
   probe: `plainPackage` has no C, C++ or directive), the second check of `Must` and `Consts` after
   the check (the first, on the base, is what the edit cannot change; for generics the amendment below
   adds a check after the focused check, of the instantiations of the new body), the comparison of
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

## Amendment (2026-10-23, job 11.lspgeneric): packages with generic declarations

Until this amendment `focusEligible` declined any package with a generic function or type, so typing there was
a full check: `large keystroke-generic` was the only benchmark row over 2 times gopls's p95 (2.05 to 2.26 in
ADR-142's measurements). The decline was wider than what is unsafe. go/types checks the body of a generic
function once, with its type parameters, and checks an instantiation at its place from the declarations (the
signature, the constraints, the methods of the type), which the focused check keeps. Two things of go/types are
not local to the edited file:

1. **A generic function without a body is an error** (`generic function is missing function body`, go/types
   decl.go), reported while the declarations are checked, so before the bodies of the edited file: stripping it
   would add an error at another file and set go/types' first error early (go/types then leaves out some
   follow-on errors, those with `invalid operand` or `invalid type` after the start of the message, and runs
   the check of instantiation cycles only without any error). **The focused check keeps the bodies of the
   generic functions** of the other files: `check.stripOtherBodies` and parser patch 8 (`SkipFuncBodies`,
   `tparams == nil`). A method of a generic type has no type parameters of its own (go/types reports nothing
   for it) and loses its body like any other.
2. **The check of instantiation cycles** (go/types mono.go) runs over the instantiations of the whole package,
   only when the package has no other error, and reports the first cycle it finds, at whatever file. The
   focused check sees a subgraph (the other bodies are missing), and usually has errors that the full check
   has not (the imports that only the missing bodies use), so it would not report a cycle that the full check
   reports, or report another one. A cycle needs an edge of weight 1: an instantiation of a generic of the
   package with a type argument that is not a type parameter itself but has one in it (`F[[]T]`,
   `Box[Pair[K, V]]`, an alias of a type parameter), or that has a type declared in a generic function.
   **`weightedInstance` declines the base when any instantiation of `Info.Instances` has such an argument**
   (all the bodies, the old one of the edited function too; also a package without generic declarations at
   the package level, for a generic type declared in a function, ADR-141), **and declines the focused check
   when its own instantiations have one** (the new body). With neither, the full check's graph has edges of
   weight 0 only, the focused check's graph is a part of it, and neither reports a cycle. It walks the type
   arguments as mono.go does and errs towards declining (an unknown generic, a type it does not walk).
   Instantiations of imported generics give no edge (mono.go skips them) and are ignored.

A third guard is about gocpp's own gates: in a focused check they see the type parameter lists of the edited
file only, and tell a type parameter of a generic function from one of a generic type by those lists. A base
with generic declarations is eligible only when the gates allow both features at the phase
(`check.GenericsOpen`; from phase 5 on, so always in the product, whose phase is at least 5).

Everything else stays as decided above: an edit of a signature, a constraint or a type parameter list is not
inside a body and declines (`findBodyEdit`).

**The order of the errors (review of this amendment).** go/types leaves out a later error whose first
message has `invalid operand` or `invalid type` after its start once it has reported any error (errors.go,
report). The focused check reports fewer errors before the edited file's than the full check (the bodies it
leaves out), so the two could disagree. This was so before the amendment for plain packages too; the
amendment makes it reach the generic ones (the methods of a generic type lose their bodies). Two declines
(`focusErrorFiles`, `otherErrors`, computed once per base), for a file other than the edited one:

- **case 1**: the base has an error in a function body that the focused check leaves out (every such body
  counts: go/types checks the methods before the functions, not in the order of the files). The full check
  reports it first and then leaves out the edited file's `cannot use "invalid type" ...`, which the focused
  check would publish. A gate error is no error of go/types and does not count, except one that takes the
  place of an error of go/types (`undefined: errors.Trace` becomes `errors.Trace is not available yet
  (Phase 10)` at phase 5): the gates mark it (`diag.Diagnostic.Replaces`, set where `gates.apply` makes the
  replacement), and it counts wherever it is, for both cases, since the message that go/types reported is
  not known (third review round);
- **case 2**: the base has an error whose message has either text after its start. go/types reported it with
  no error before it; an edit that adds the first error makes the full check leave it out, while the quick
  publish sends the edited file only and the client keeps the old error until the settle run.

Not covered (**case 3**, a known gap): an edit that removes the only error before a dropped error of another
file lets that error back in the full check; the quick publish does not send the other file, so the client
gets it at the settle run (100 ms later), not at once. Tests: `TestFocusedErrorOrder` (cache) and
`TestDiagnosticsQuickErrorOrder` (server, first publish and the other file against a server without the
focused check), each case in a plain package and with methods of generic types, and case 1 with
`errors.Trace`, at phases 15 and 5; removing the decline fails the cache cases, and removing the count of
`Replaces` fails the `errors.Trace` one at phase 5. The other rewrites of errors in internal/check (C++
members, HRESULT calls, must functions, the loader's package errors) are of packages that are not eligible.

### Equivalence

- `TestFocusedGenerics` (cache): 37 statements at three places of the edited file (an ordinary method, a
  generic function and a method of a generic type of another file), each at phases 15 and 5: inference,
  explicit instantiation, constraint violations, wrong type arguments, methods of instances, generic values
  without instantiation, type parameters of the edited function, local types. 77 of 89 edits per phase are
  answered with the full check's diagnostics for the edited file, the other files unchanged; the 12 with an
  edge of weight 1 decline. The base has no go/types error there, so the full check runs the cycle check.
- `TestFocusedGenericCycle`: `First[[]T](nil)` typed in `First` makes a cycle that the full check reports at the
  edited file; the focused check declines. Without the decline of the focused check's instantiations the
  focused answer is empty (no cycle) where the full check reports one: the test fails.
- `TestFocusedGenericFile`: 11 edits of the file of the generic declarations, of which 4 must decline (an edge
  of weight 1, a changed constraint, type parameter list or result list).
- `TestFocusedDeclinesBase`: an edge of weight 1 in another file (a slice of a type parameter, a generic type of
  a generic type, an alias of a type parameter, a type declared in a generic function, a generic type declared
  in a function, an instantiation cycle) and phase 4's gates make the base ineligible;
  `TestFocusedDeclines` adds a changed constraint, changed type parameters and a typed cycle;
  `TestFocusedLoadSkipsOtherBodies`, `TestStripOtherBodies`, `TestFocusedCheck` and `TestSkipFuncBodies` show
  that generic functions keep their bodies and methods of generic types do not.
- Sweeps (every answered edit equal to the full check): `TestFocusedSweepGenericSplit`, new, the 280 programs of
  `tests/run` with generic declarations, each split into two files by declaration: 3,878 of 6,153 edits
  answered (none before: all of them have generics). `TestFocusedSweepExamples` 149 to 165 of 273,
  `TestFocusedSweepCorpus` 1,817 to 2,405 of 4,521 (both phases each).
- Server: `TestDiagnosticsQuickMatchesFullRunGenerics`, the session test of the quick publish over a module with
  generic declarations in two files of lib, a generic function and a method of a generic type in the edited
  file, the statements above (with and without type parameters, with edges of weight 1 and a cycle) and
  changes of generic signatures: 40 seeds, 720 first publishes per phase, about 100 focused checks (147 before
  the error-order declines below), all equal; 200 seeds (3,600 publishes per phase, about 540 focused checks)
  were equal in three runs, one of them at a 1-minute load of 151. The "busy for a minute" stop of the first
  round (phase 5 seed 106) was a race of the test, not a hang of the server: the test's flush ran the full runs
  of the folders that had a run waiting, but lib's full run published a changed API and ran main again with
  the focused check, and that quick run armed its settle run, which the test delays by an hour, so the
  debouncer stayed busy with nothing running. The flush (`flushFull`) now runs again the full runs of whatever
  is left until nothing waits. Seed 106 at phase 15, 150 times: 31 stops with the old flush, none with the
  new. `TestCompletionFocusedMatchesFullGenerics`:
  22 places in an ordinary function and in a generic one (fields and methods of instances, inferred results,
  methods of a type parameter's constraint), equal, with the labels that each answer must have.
- `TestRandomEdits` seeds 101, 108, 111 and 112 at `GOCPP_LSP_FUZZ_EDITS=1200`: pass (its project has no
  generics; the test guards the rest of the server).
- Bounded evidence, not a proof, as above.

### Measurements

`lspbench -projects large,cpp -only keystroke-generic,completion-generic,keystroke-new -runs 5`, gopls v0.21.1
with diagnosticsDelay 30 ms, default GOMAXPROCS (192), go1.25.14 (the dev shell), the temporary work folder of
lspbench (a `-workdir` inside the repository's worktree breaks gopls: `missing metadata for import of "fmt"`,
33 ms per keystroke; those runs were discarded). Base: dev `8b0d1505` (sha256 `704a20d0...`); candidate: this
job (`ecfbeaf5`, sha256 `370cdd37...`; round 1's candidate was `d3e742a2`, sha256 `8ea5407e...`, the same for
these projects: they have no generic type declared in a function). p95 in ms, gocpp / gopls / ratio, the median
over 5 runs of each run's p95. Loads (1, 5, 15 minutes) at the start and the end of each run:

| run | time (UTC) | load at start / end |
|---|---|---|
| R1 base | 20:32 | 10.5 27.6 39.8 / 179 74.3 54.2 |
| R1 candidate | 20:39 | 15.2 40.4 45.4 / 157 87.1 61.6 |
| R2 candidate | 20:52 | 15.3 56.7 65.4 / 79.7 65.7 67.1 |
| R2 base | 20:55 | 39.9 57.1 64.1 / 17.2 40.6 56.8 |
| R3 base | 20:58 | 17.2 40.6 56.8 / 17.8 30.5 50.5 |
| R3 candidate | 21:00 | 17.8 30.5 50.5 / 94.7 65.9 61.0 |

Every run started under a 1-minute load of 40; other work on the machine raised it during some runs (the ends).

| project / row | base R1 | base R2 | base R3 | candidate R1 | candidate R2 | candidate R3 |
|---|--:|--:|--:|--:|--:|--:|
| large keystroke-generic | 15.0 / 5.94 / 2.53 | 12.6 / 5.20 / 2.43 | 15.6 / 5.90 / 2.65 | 8.01 / 8.28 / 0.97 | 7.38 / 6.01 / 1.23 | 8.16 / 5.99 / 1.36 |
| large completion-generic | 12.7 / 8.64 / 1.47 | 12.1 / 7.65 / 1.59 | 12.1 / 9.39 / 1.29 | 7.59 / 11.0 / 0.69 | 7.87 / 10.0 / 0.78 | 7.39 / 9.50 / 0.78 |
| large keystroke-new (control) | 8.32 / 6.19 / 1.35 | 7.14 / 5.26 / 1.36 | 7.03 / 5.43 / 1.30 | 7.65 / 8.62 / 0.89 | 7.91 / 4.99 / 1.59 | 6.89 / 5.59 / 1.23 |
| cpp keystroke-generic | 10.5 / 7.04 / 1.49 | 10.7 / 7.14 / 1.50 | 10.0 / 6.62 / 1.52 | 7.03 / 7.22 / 0.97 | 6.32 / 5.99 / 1.06 | 7.39 / 5.49 / 1.35 |
| cpp completion-generic | 13.6 / 8.95 / 1.52 | 14.3 / 9.27 / 1.55 | 13.2 / 7.48 / 1.77 | 7.43 / 8.97 / 0.83 | 7.91 / 8.53 / 0.93 | 7.50 / 8.99 / 0.83 |
| cpp keystroke-new (control) | 6.50 / 6.98 / 0.93 | 6.00 / 7.35 / 0.82 | 5.79 / 5.34 / 1.08 | 6.02 / 6.74 / 0.89 | 5.56 / 6.98 / 0.80 | 6.78 / 6.00 / 1.13 |

- The generic rows are within 2 times gopls's p95 in every run of the candidate (large keystroke-generic 0.97 to
  1.36, from 2.43 to 2.65); their p50 is now that of keystroke-new (large 2.84 to 3.56 ms against 4.12 to 4.38
  for the base). The control row does not move beyond the spread between runs.
- The p95s of these runs are higher than ADR-142's quiet ones for both servers (gopls's large keystroke-new 5.0
  to 8.6 ms against 2.6), because of the load; the ratios are of the same runs.
- docs/benchmarks/lsp.md is not regenerated (the machine was not quiet); the next quiet `make lsp-bench` does it.
- After the error-order declines (round 2 of the review; candidate `92b27bb1`, sha256 `ec7d6151...`), one more
  round, R4 (candidate 21:57, load 28.5 54.3 50.4 / 18.0 38.4 44.9; base 21:59, load 18.0 38.4 44.9 / 128 59.3
  50.7), p95 gocpp / gopls / ratio: large keystroke-generic 7.73 / 5.05 / 1.53 (base 13.1 / 5.71 / 2.30), large
  completion-generic 7.18 / 8.62 / 0.83 (base 12.8 / 9.55 / 1.35), large keystroke-new 7.04 / 5.53 / 1.27 (base
  7.93 / 6.67 / 1.19), cpp keystroke-generic 6.66 / 6.56 / 1.02 (base 11.6 / 6.85 / 1.70), cpp
  completion-generic 8.42 / 9.10 / 0.92 (base 12.8 / 8.77 / 1.46), cpp keystroke-new 6.70 / 6.87 / 0.98 (base
  6.77 / 6.84 / 0.99). The declines do not touch these rows (their errors are in the edited file).

## Tried and not kept

- `-idle-gc` and `GOGC=200`: three alternating rounds each; the differences between them were not larger
  than the spread between runs of the same binary (the `medium` completion-edit p95 was 5.9 ms plain, 3.4 ms
  with `-idle-gc` and 6.5 ms with `GOGC=200` in the first round, and the order changed in the others). Not
  kept.

## Not done

- A focused check for edits that change a declaration (a signature, a type, an import): they take the full
  check as before. Proving them equal to the full check needs the dependency graph of the declarations.
- Packages with macros, `!` forms or C/C++ files are never answered from the focused check (packages with
  generics are since the amendment below); the
  benchmark's `medium` project has none, the sweeps over the repository's programs answered about 40 percent
  of the generated and example edits.
- A measurement in which the settle full check runs inside the loop (think times above 100 ms), and the
  rows of the `large` project, and Windows (the code is portable; the unit tests ran on Linux only: vet
  for GOOS=windows and darwin is clean).
- `TestEditorFeaturesSweep` fails on the dev base too (`semanticTokens/full returned null`); it is not changed
  by this step.
- `internal/syntax/parser/gocpp_macro_name_test.go` at `dev` has two cases that the patches do not produce
  (`update.sh -f` removes them); this is old drift, not changed here.
