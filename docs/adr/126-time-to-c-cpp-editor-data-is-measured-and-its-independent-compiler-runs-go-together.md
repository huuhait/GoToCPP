# ADR-126: Time to C/C++ editor data is measured, its independent compiler runs go together, and no request waits for the compiler

Status: accepted (2026-10-21)

## Context

Phase 12 gives the editor C/C++ data from a background clang: names after a header package or a
namespace (`raylib.`, `std.`), the members and native type of a variable (`win.`), and the
declaration, comment and location of a name (hover, go to definition). Two lines of the phase were
open: a budget row for the time until each kind of data is there (under 2 s, in the background),
and a test with a slow compiler that shows that nothing in the editor waits for it. The Phase 11
budgets must still hold with that work running. Step 12.13d closes both.

## Measurement

`tools/cppdata` starts the real `gocpp lsp` on a project that imports header packages, sends
`didOpen`, and polls each kind of request every 5 ms until the answer has the C/C++ data. It
reports the time from `didOpen`. Cold is an empty probe cache, warm is the disk cache that the cold
run filled. Every cell is the median of 5 runs, one after the other, on cores 16-23 under the shared
`perf.lock` (eight pinned cores, so the language server's `Jobs()`, the bound on the compiler runs
of the process, is 8; "4 slots" is `GOCPP_PROBE_JOBS=4`, which is also what the server had by
default before the change of the default below, and what a machine with four cores has), with a
load average under 10 (other jobs of the machine were running, the cells of both columns are
stable to a few percent), clang 21, Linux. The projects: `raylib` and `sqlite3` (one C header each), `cpp-std`
(`<algorithm> <map> <string> <vector>`) and `local-header` (the same standard headers and a header
of the project). A kind is "never" when its data did not come in 30 s; there is none in the table.

"Before" is the server at `origin/dev` b392dcfe on four slots, "after" is this step on four slots and
on eight, which is the server's default (see `Jobs()` below). The warm columns are on eight slots
and equal on four. The times are milliseconds from `didOpen`. Two cells have two values: the
facts of `cpp-std` on four slots come in one of two modes, 1747 ms when the early lookup is ready
first and 1952 ms when the facts wait for it (the median of 5 runs falls on either; the range of
the old column was 1742-1955).

| project | kind | cold before | cold after, 4 slots | cold after, 8 slots | range, 8 slots | warm before | warm after |
|---|---|---:|---:|---:|---|---:|---:|
| raylib | header-package completion | 240 | 214 | 193 | 193-193 | 60 | 60 |
| raylib | API hover | 303 | 271 | 250 | 249-250 | 53 | 59 |
| raylib | API definition | 303 | 271 | 250 | 249-250 | 53 | 59 |
| raylib | variable type hover | 797 | 778 | 782 | 781-823 | 259 | 264 |
| raylib | member completion | 798 | 778 | 782 | 781-823 | 259 | 264 |
| raylib | member hover | 881 | 799 | 803 | 801-844 | 280 | 285 |
| sqlite3 | header-package completion | 244 | 202 | 186 | 185-187 | 64 | 64 |
| sqlite3 | API hover | 328 | 281 | 270 | 265-270 | 59 | 59 |
| sqlite3 | API definition | 328 | 281 | 270 | 265-270 | 59 | 59 |
| local-header | header-package completion | 1390 | 1297 | 1334 | 1325-1373 | 103 | 104 |
| local-header | namespace completion | 1385 | 1301 | 1337 | 1310-1347 | 105 | 107 |
| local-header | API hover | 1793 | 2065 | 1745 | 1728-1780 | 107 | 109 |
| local-header | API definition | 1791 | 2065 | 1744 | 1728-1780 | 107 | 109 |
| local-header | variable type hover | 3685 | 2050 | 1784 | 1766-1806 | 335 | 313 |
| local-header | member completion | 3685 | 2050 | 1784 | 1767-1806 | 335 | 313 |
| local-header | member hover | 4099 | 2071 | 1807 | 1787-1827 | 368 | 335 |
| cpp-std | namespace completion | 1357 | 1281 | 1267 | 1262-1271 | 79 | 80 |
| cpp-std | API hover | 1761 | 1849 | 1680 | 1677-1687 | 83 | 84 |
| cpp-std | API definition | 1761 | 1849 | 1680 | 1677-1687 | 83 | 84 |
| cpp-std | variable type hover | 2806 | 1747 or 1952 | 1714 | 1711-1717 | 299 | 287 |
| cpp-std | member completion | 2806 | 1747 or 1952 | 1714 | 1712-1717 | 299 | 288 |
| cpp-std | member hover | 3214 | 1971 | 1735 | 1732-1738 | 330 | 309 |

Reading the table:

- Header-package and namespace completion are the first data: the index of the names (0.2 s for one
  C header, 1.3 s with the standard headers cold, 0.1 s or less warm).
- Hover and definition of a name come from the declaration lookup that follows the index.
- Variable type hover and member completion come from the member analysis, member hover (the
  declaration of `win.size`) after it.
- The standard headers are the expensive part: one `-fsyntax-only` parse of
  `<algorithm> <map> <string> <vector>` takes 0.25 to 0.3 s and the cold data needs several in a
  chain.
- Hover and definition of a name are slower after the change on four slots on the two projects with
  the standard headers (`cpp-std` +0.09 s, `local-header` +0.27 s): the compiler runs of the member
  analysis, which now overlap, take places from the declaration lookup of the names. With eight
  slots they are 0.07 s (`cpp-std`) and 0.05 s (`local-header`) faster than before.

The compiler runs of the process are bounded by `Jobs()`: `GOCPP_PROBE_JOBS` if it is set, else
`min(8, NumCPU)` in the language server (`gocpp lsp` calls `probe.SetInteractive`) and
`min(8, NumCPU/2)` in a build, where the compiler jobs of the build and the program's own work share
the machine. The server's default was `min(8, NumCPU/2)` before this step; it is all the cores up to
eight because the cold data is a chain of compiler runs that is under 2 s only when eight of them can
go together, and eight slots on eight pinned cores cost the keystroke 0.1 ms at most over four
(the typing rows below). `TestJobsDefaultOfABuildLeavesHalfTheCores`,
`TestJobsDefaultOfALanguageServerUsesAllCoresUpToEight` and
`TestJobsEnvironmentOverridesTheLanguageServerDefault` test the three rules. The slowest kind in
each project, cold, for the same binary:

| `Jobs()` | cpp-std slowest kind, ms | local-header slowest kind, ms |
|---:|---:|---:|
| 4 (`GOCPP_PROBE_JOBS=4`) | 1971 | 2071 |
| 6 (`GOCPP_PROBE_JOBS=6`) | 1732 | 1984 |
| 8 (`GOCPP_PROBE_JOBS=8`, measured on 8 pinned cores) | 1726 | 1783 |
| 8 (the server's default on the same 8 pinned cores, the table above) | 1735 | 1807 |

So every kind is under 2 s cold with eight slots (1.81 s at most), at the limit with six (1.98 s for
`local-header`), and with four all but the `local-header` kinds (medians 2.05 to 2.07 s, all
runs 2.02 to 2.12 s, up to 6 percent over). `TestCppDataBudget`
(`GOCPP_CPPDATA_BUDGET=1`, a quiet machine with clang, not part of the ordinary tests) checks the
2 s of every kind of every project with the server's default slots.

After an edit the snapshot is new and the work is done again, but the disk cache has the products
of the headers: names are back in 6 to 33 ms and the member facts in 0.21 to 0.26 s (the server
process is the one that was cold; the cache is the one its first open filled).

| project | kind | after an edit, ms |
|---|---|---:|
| raylib | header-package completion | 12 |
| raylib | API hover | 6 |
| raylib | API definition | 6 |
| raylib | variable type hover | 211 |
| raylib | member completion | 211 |
| raylib | member hover | 231 |
| local-header | header-package completion | 31 |
| local-header | namespace completion | 26 |
| local-header | API hover | 33 |
| local-header | API definition | 33 |
| local-header | variable type hover | 237 |
| local-header | member completion | 237 |
| local-header | member hover | 258 |
| cpp-std | namespace completion | 25 |
| cpp-std | API hover | 33 |
| cpp-std | API definition | 33 |
| cpp-std | variable type hover | 237 |
| cpp-std | member completion | 237 |
| cpp-std | member hover | 258 |

### What was changed

Cold data was a chain of compiler runs that did not depend on each other. Four sound reductions;
none of them reuses an answer across flags, includes or targets, and the guards of ADR-117, 121 and
122 are the same (the inputs are captured before the runs and compared after them, and an answer
that depends on a file saved less than 2 s before the compiler started is not kept):

1. **The four runs of a header index go together** (`probe.makeIndex`): the declaration list, the
   macros of the headers, the compiler's own macros and the preprocessed text for the namespaces
   read the same headers and need nothing from each other. They start at the same time and still
   draw from the process-wide limiter, so the bound on the compiler runs is the same.
2. **The validation run and the completion run of a member query go together**
   (`probe.makeMembers`). An early return ends and joins the one that is still running.
3. **The member queries of different receiver types go together** (`cache.analyzeMembers`): one
   query per distinct receiver type, at most `maxMemberQueries`, the answers read in the order of
   the first use as before, the first error in that order.
4. **The declaration lookup of the members starts with the member questions**
   (`cache.earlyLocate`). It asks for every member name that the program writes on a bound
   variable, before it is known which of them are members. The facts use the answers of the proven
   names only; the answer for a name does not depend on the other names of the request
   (`TestLocateMembersDoNotDependOnTheOtherNames`, real clang). The analysis joins it before it
   ends and waits for it at most `earlyGrace` (750 ms) after the member names are known; a lookup
   that is later or failed is stopped, and the second step runs it as before (and with its retry
   for headers saved just before). The facts are never delayed by more than the grace, and a failed
   lookup does not fail them. The early request is the names as written, so it can hold a word
   that is no name in C++ (`win.new`, `p.class`, `s.template`, `int`): the compiler cannot parse
   the lookup of such a word, and one of them would have failed the whole run, so that the receiver
   went to the slower second step. `probe.validLocateMembers` leaves the C++ keywords out of a
   request (as it leaves out any other word that is not an identifier), and a name that is left out
   has no declaration, as for a name that the compiler does not know
   (`TestLocateDropsCxxKeywordsInsteadOfFailingTheRun`; the wide request of
   `TestLocateMembersDoNotDependOnTheOtherNames` has keywords, and with real clang it still gives
   the answers of the narrow one). The early request has at most `maxMemberDeclNames` names, in the
   order of the program; the facts use its answers only when it asked for every name that the
   second step wants (`covers`), so a proven member beyond the cap is asked for by the step
   itself (`TestMemberStepAsksForAMemberBeyondTheCapOfTheEarlyLookup`: 70 names that are no members
   before the one that is).

The cost of the fourth is in the table (the 4 slots column): the facts (variable type hover, member completion) of
`cpp-std` come 0 to 0.2 s later than without it (the median is bimodal, 1.74 s when the lookup is
ready first, 1.95 s when the facts wait for it), and member hover 0.17 s earlier (2.14 to 1.97 s;
`local-header` 2.40 to 2.07 s). It is kept because it is the one change that brings the last kind
of `cpp-std` under 2 s; the delay is bounded by `earlyGrace`. Starting it as a separate step that
the facts do not wait for was not done: its process would have to be joined by the owner of the
entry in every path that drops the facts, which the single-owner design of the worker avoids.

Besides, `scheduleHeaders` and `scheduleMembers` ran on every request that asks for the package,
and walked the syntax trees and the uses of the package each time. The result depends on the check
alone and is now kept with the package (`Package.Memo`). That was the cause of a constant 15 to 20
microseconds on every hover and definition in typed mode, found by the budget run below.

Run priority was tried and is not kept: running the compiler children at niceness 10 (`setpriority`
on the process group) changed no latency of the request rows in 4 alternating rounds, and a low
priority for the early lookup in the limiter changed no time of the facts.

## Slow compiler

`internal/lsp/server/slowclang_linux_test.go` runs the server with a fake compiler: a shell script
that records its process and waits for a gate file (no sleeps in the assertions), then runs the real
compiler, crashes, or holds its answer back until a second gate. Each test shows that the compiler
was still blocked when the server answered.

| Property | Test |
|---|---|
| No request waits (completion, hover, definition, diagnostics, `didChange`, rename, formatting), and the answers equal those of the opaque server | `TestSlowClangNoRequestWaitsAndAnswersAreThoseWithoutTypedData` |
| When the compiler answers, later requests have the data, and the diagnostics of the pending header imports are published again (the refresh path that exists, `OnProbe`) | `TestSlowClangDataAppearsWhenTheCompilerAnswers` |
| Shutdown and exit kill and join the blocked compiler: no process, no goroutine | `TestSlowClangShutdownKillsTheBlockedCompiler` |
| An edit never shows the facts of the old text: after the data, while the compiler runs, or when a header changes under a compiler that has read it and holds its answer | `TestSlowClangEditAfterDataNeverServesTheOldFacts`, `TestSlowClangEditWhileTheCompilerRunsNeverServesTheOldFacts`, `TestSlowClangHeaderChangedWhileTheCompilerFinishesPublishesNoOldFacts` |
| A compiler that hangs and one that crashes leave the plain answers, and no process or goroutine | `TestSlowClangHangingCompilerIsKilledAndTheEditorKeepsThePlainAnswers`, `TestSlowClangCrashingCompilerLeavesThePlainAnswers` |

The new mechanisms have tests with a compiler that waits for the others to start (3 s, then it
leaves a file `alone` and goes on), so that a serial implementation fails by an error that names
the mechanism and not by being slow: the four runs of an index
(`TestHeaderIndexRunsItsFourCompilerRunsTogether`), the two runs of a member query
(`TestMemberQueryRunsValidationAndCompletionTogether`), the questions of two receivers
(`TestMemberAnalysisAsksTheMembersOfDifferentReceiversTogether`) and the early lookup
(`TestMemberAnalysisStartsTheDeclarationLookupWithTheMemberQuestions`, which also shows that the
facts use its answers although it asked for a name that is no member, and
`TestMemberFactsDoNotWaitForASlowDeclarationLookup`). The memo has `TestHeaderGroupsAreMadeOncePerPackage`. The cache tests set their four compiler
slots themselves (`GOCPP_PROBE_JOBS=4`), so they do not depend on the machine or on the
environment of the run; with the slots set to 1 the two cache tests fail with "a run waited alone"
(they also failed, only after 5 s of waiting, before the wait of the entry was made longer than
the wait of the run).

The guards of the early lookup have tests too. The lookup's answers are computed at one time and
used at another, and a header can change between the two:

| Guard | Test | Mutation that it catches |
|---|---|---|
| the inputs are compared after the early lookup has ended, before the facts are returned | `TestMemberAnalysisDropsFactsOfAHeaderChangedAfterTheEarlyLookup` (the header is rewritten by the `earlyDone` hook) | the final `Unchanged` check of `analyzeMembers` removed |
| the inputs are compared again when the step uses the answers of the early lookup | `TestMemberStepDropsEarlyAnswersOfAHeaderChangedLater` (the header is rewritten by the `stepStart` hook) | the `Unchanged` check of the step removed |
| the input guard itself | the same two tests | `InputGuard.Unchanged` made `return g != nil` |
| the early answers cover the names that the step wants | `TestMemberStepAsksForAMemberBeyondTheCapOfTheEarlyLookup` | the `covers` test removed from `earlyLocate.answers`: the hover declaration of the member beyond the cap silently disappears |

Each of these four mutations was applied and its test failed (the third fails both of the tests of the first
two rows, each of the others its own). The grouped runs and the early lookup were checked by making the
mechanism serial, or by ignoring the early answers, and seeing their tests fail; the keyword filter
by removing it (two tests fail). The slow-clang tests were checked by mutations of the guards that
they cover (stale facts served, process not joined, a request that waits); the guards of the early
path are covered by the cache tests above, not by those.

## Phase 11 budgets with the background work running

`tools/lspbench` with `-projects small,medium,cpp`, 5 sessions per row and 3 alternating rounds
(the pooled n is 600 or 900 samples; A at 150 ms think has two rounds, n 400 or 600, because the
third did not start in six tries: lspbench refuses to run when the load of the machine is over 30),
cores 16-23 under `perf.lock`, `GOMAXPROCS=8`, with the server's default of eight compiler slots.
Other jobs were running on the machine during the rounds (the load average was between 7 and 30),
so the cells are noisy: `small/hover` and `medium/hover` have no C++ and show ratios up to 1.25. A is the
default (opaque) server, B is `GOCPP_INTEROP=typed` with the disk cache, C is typed with
`GOCPP_NOPROBECACHE=1`, so that every session does all the background work. The pause after each
keystroke (`-think`) is 25 ms, under the settle delay of the server, so that the background check
and the compiler work mostly do not run between the edits, or 150 ms, which lets them run. The rows
`keystroke-cpp-new` (typing in the C++ interop file of the `cpp` project) and `hover-cpp` (hover
over C++ names) are new and `gocpp` only; `cpp` is the reference project with C/C++ imports,
`small` and `medium` have none. Each cell is p50 / p95 in milliseconds; the ratios are to A.

| think | project/row | A p50 / p95 | B p50 / p95 | B ratio p50 / p95 | C p50 / p95 | C ratio p50 / p95 |
|---|---|---|---|---|---|---|
| 25 ms | cpp/keystroke | 0.33 / 0.50 | 0.32 / 0.50 | 0.97 / 0.99 | 0.33 / 1.03 | 1.01 / 2.05 |
| 25 ms | cpp/keystroke-new | 2.10 / 3.10 | 2.01 / 3.03 | 0.96 / 0.98 | 2.07 / 3.22 | 0.98 / 1.04 |
| 25 ms | cpp/keystroke-cpp-new | 1.54 / 2.03 | 1.90 / 2.66 | 1.23 / 1.31 | 1.95 / 2.66 | 1.27 / 1.31 |
| 25 ms | cpp/completion-dot | 0.10 / 0.19 | 0.10 / 0.17 | 1.00 / 0.92 | 0.10 / 0.21 | 1.02 / 1.12 |
| 25 ms | cpp/hover | 0.05 / 0.15 | 0.05 / 0.15 | 1.05 / 1.03 | 0.07 / 0.29 | 1.35 / 2.01 |
| 25 ms | cpp/definition | 0.04 / 0.05 | 0.04 / 0.05 | 1.03 / 1.02 | 0.04 / 0.07 | 1.04 / 1.40 |
| 25 ms | cpp/hover-cpp | 0.03 / 0.04 | 0.04 / 0.06 | 1.20 / 1.45 | 0.04 / 0.11 | 1.22 / 2.39 |
| 25 ms | medium/keystroke | 0.35 / 0.64 | 0.35 / 1.29 | 1.01 / 2.02 | 0.36 / 1.36 | 1.04 / 2.14 |
| 25 ms | medium/keystroke-new | 2.25 / 2.81 | 2.26 / 5.12 | 1.01 / 1.82 | 2.29 / 3.63 | 1.02 / 1.29 |
| 25 ms | medium/hover | 0.06 / 0.16 | 0.06 / 0.24 | 1.03 / 1.48 | 0.06 / 0.16 | 1.14 / 1.02 |
| 25 ms | small/keystroke | 0.27 / 0.46 | 0.28 / 0.40 | 1.02 / 0.87 | 0.27 / 0.44 | 0.99 / 0.95 |
| 25 ms | small/keystroke-new | 1.84 / 2.42 | 1.87 / 2.49 | 1.02 / 1.03 | 1.83 / 2.46 | 0.99 / 1.01 |
| 25 ms | small/hover | 0.05 / 0.15 | 0.07 / 0.23 | 1.25 / 1.52 | 0.06 / 0.16 | 1.06 / 1.04 |
| 150 ms | cpp/keystroke | 0.42 / 0.81 | 0.63 / 0.86 | 1.51 / 1.06 | 0.53 / 0.71 | 1.28 / 0.88 |
| 150 ms | cpp/keystroke-new | 2.07 / 2.93 | 2.42 / 3.45 | 1.17 / 1.18 | 2.31 / 2.66 | 1.12 / 0.91 |
| 150 ms | cpp/keystroke-cpp-new | 1.60 / 2.18 | 1.89 / 2.88 | 1.18 / 1.32 | 1.87 / 2.38 | 1.17 / 1.09 |
| 150 ms | cpp/completion-dot | 0.11 / 0.19 | 0.11 / 0.27 | 1.03 / 1.46 | 0.11 / 0.14 | 0.99 / 0.77 |
| 150 ms | cpp/hover | 0.05 / 0.16 | 0.05 / 0.16 | 1.00 / 1.03 | 0.05 / 0.15 | 0.97 / 0.96 |
| 150 ms | cpp/definition | 0.04 / 0.05 | 0.04 / 0.07 | 1.06 / 1.38 | 0.04 / 0.05 | 1.04 / 1.04 |
| 150 ms | cpp/hover-cpp | 0.03 / 0.05 | 0.03 / 0.06 | 1.18 / 1.34 | 0.03 / 0.06 | 1.15 / 1.23 |
| 150 ms | medium/keystroke | 0.40 / 0.70 | 0.40 / 0.68 | 0.98 / 0.97 | 0.41 / 0.67 | 1.02 / 0.96 |
| 150 ms | medium/keystroke-new | 2.33 / 4.05 | 2.29 / 3.34 | 0.99 / 0.83 | 2.32 / 3.12 | 1.00 / 0.77 |
| 150 ms | medium/hover | 0.05 / 0.15 | 0.05 / 0.15 | 1.02 / 1.01 | 0.05 / 0.16 | 1.03 / 1.04 |
| 150 ms | small/keystroke | 0.35 / 0.81 | 0.31 / 0.59 | 0.88 / 0.72 | 0.34 / 0.66 | 0.95 / 0.81 |
| 150 ms | small/keystroke-new | 1.85 / 2.62 | 1.85 / 2.42 | 1.00 / 0.92 | 1.93 / 2.70 | 1.04 / 1.03 |
| 150 ms | small/hover | 0.05 / 0.15 | 0.05 / 0.17 | 1.07 / 1.15 | 0.06 / 0.18 | 1.09 / 1.18 |

The budgets (`internal/lsp/bench/budget.go`) are keystroke p95 under 20 ms (25 ms for new text),
hover, definition and completion p95 under 10 ms. Every cell is a few percent of its budget with the
compiler work running, and none is over.

- `small` and `medium` have no C/C++ import: typed mode changes nothing at p50 (ratios 0.88 to 1.04
  for typing; 1.00 to 1.25 for hover, definition and completion, where 0.05 ms against 0.07 is the
  rounding of two samples of a noisy machine). The p95 of `keystroke` shows outlier rounds of B and
  C in `medium` (1.29 and 1.36 ms against 0.64 at 25 ms think) also where no compiler runs, so it
  is the machine, not the work.
- Hover, definition and completion of Go code in `cpp`: ratio 0.97 to 1.06 at p50 in B and in C, but
  `hover` in C at 25 ms think (0.07 ms against 0.05, p95 0.29 against 0.15), which is 0.97 at 150 ms
  think; the same machine gave 1.25 for `small/hover` in B. Before
  the schedule was memoized they were 1.2 to 1.5 at p50, 15 to 20 microseconds, in all three
  projects.
- `hover-cpp` costs 0.00 to 0.01 ms more at p50 than the opaque answer (the typed answer looks the
  facts up; 0.03 to 0.04 ms) and its p95 is 0.05 to 0.06 ms in B against 0.04 to 0.05 (C: 0.06 to
  0.11 ms).
- Typing in the project with the compiler work running costs more: at 150 ms think `keystroke`
  +0.21 ms in B and +0.11 in C (0.42 ms in A, 0.63 in B and 0.53 in C), `keystroke-new` +0.35 and
  +0.24 ms, `keystroke-cpp-new` +0.29 and +0.27 ms; at 25 ms think `keystroke-cpp-new` +0.36 and
  +0.41 ms and the other two equal. That is 1.0 to 1.5 times and 1 to 2 percent of the 20 ms
  budget. It is the same as with four slots (B at 150 ms: 0.62, 2.32 and 1.86 ms there; 0.63, 2.42
  and 1.89 now), so that the eight slots of the server cost the keystroke 0.1 ms at most.

The cause of the typing cost was looked for. With `GOCPP_CLANG=/nonexistent` (typed mode, all the
scheduling, no process) the three rows equal the opaque server, so it is the compiler work and not
the scheduling. Running the children at niceness 10 changed nothing, so it is not the run queue of
the pinned cores. The CPU profile of a session shows the server spending 8 percent of its time on
the background work, 18 percent of that in the mark workers of the garbage collector and 6 percent
in `fork`/`exec`; the likely cause is the allocation and the process starts of that work meeting the
edit. It was not isolated further and it is not fixed: 0.3 ms of a 20 ms budget.

### Requests while the compiler starts

The same hover, definition and completion requests, sent every 5 ms in the first 1.5 s after
`didOpen` (the cold compiler work) in typed and opaque mode, medians of 5 starts, in milliseconds:

| project | mode | p50 ms | p95 ms | max ms |
|---|---|---:|---:|---:|
| raylib | opaque | 0.14 | 0.20 | 0.66 |
| raylib | typed | 0.14 | 0.22 | 6.75 |
| local-header | opaque | 0.11 | 0.18 | 31.61 |
| local-header | typed | 0.14 | 0.23 | 31.71 |
| cpp-std | opaque | 0.11 | 0.17 | 3.93 |
| cpp-std | typed | 0.13 | 0.20 | 8.72 |

The medians and the p95 are 0.02 to 0.05 ms apart; the one slowest request is 7 to 9 ms against 1 to
4 ms on `raylib` and `cpp-std` (`local-header`'s 31 ms is the first check of the package, equal in
both).

## Gaps

- **Nothing tells the editor that the data has come.** Completion, hover and definition answer
  when asked; an editor that asked before the facts were ready keeps the plain answer until its
  next request. The only server-initiated refresh is the republish of diagnostics after the
  namespace probe (`OnProbe`), which the test shows. A refresh of semantic tokens or inlay hints
  after the facts would be the way; it was not done.
- **Typing cancels the work.** A new snapshot cancels the background compiler runs of the old one.
  With a key every 100 ms none of the data arrives while the typing lasts (`cppdata -typing 100ms`),
  and after a pause the names are back in 6 to 33 ms and the members in 0.21 to 0.26 s. Carrying
  the index of the names across snapshots with the same header inputs is the remedy; it needs the
  input guard of ADR-117 for a snapshot and was not attempted.
- **`local-header` is at 2.05 to 2.07 s cold with four slots** (runs up to 2.12 s), up to 6
  percent over the budget, and hover and definition of names are slower with the early lookup and
  the other overlapping runs than before on the two projects with the standard headers. The
  budget is met with the eight slots that the server has on a machine with eight cores or more; a
  machine with four cores, or `GOCPP_PROBE_JOBS` below six, misses it by that much. The numbers are
  for one machine, one clang (21) and Linux; Windows was not measured.
- **A sqlite3 variable has no native type**: `sqlite3 *` is a pointer to an incomplete struct, which
  has no members to prove, so that row does not exist (it was "never" before, too).
- The typing cost with the compiler running (+0.1 to +0.4 ms) has no isolated cause.
- The hanging-compiler test cannot tell the time limit of a run from the kill by a new snapshot;
  both stop the compiler, and the test asserts the effect (no process, the plain answers). Two
  mutations of the guards are not caught by the slow-clang tests, because the protection is
  structural or layered (the input guard of the probe stands behind that of the cache).
- The reference `cpp` project of the bench is small, and its own header is saved at the start of a
  session, which the probe does not trust for 2 s; the typing rows are the evidence for the cost,
  and the cold times above are the evidence for the budget. A real project with the standard headers
  that is typed in for a long time was not measured.

## Reproducing

```
go build -o /tmp/gocpp ./cmd/gocpp && go run ./tools/cppdata -gocpp /tmp/gocpp -runs 5
go run ./tools/cppdata -gocpp /tmp/gocpp -modes cold -edit -only cpp-std
go run ./tools/lspbench -gocpp /tmp/gocpp -targets gocpp -projects small,medium,cpp -runs 5 \
    -only keystroke,keystroke-new,keystroke-cpp-new,completion-dot,hover,definition,hover-cpp \
    -think 150ms -gocpp-env GOCPP_GATE_PHASE=15,GOCPP_INTEROP=typed -json typed.json
go run ./tools/cppdata -compare 'A=opaque1.json,opaque2.json;B=typed1.json,typed2.json'
```

Use a quiet machine, the shared `perf.lock` and `taskset` as for every benchmark, one run at a time.
