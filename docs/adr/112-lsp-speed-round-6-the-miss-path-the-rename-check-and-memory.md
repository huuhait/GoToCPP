# ADR-112: Language server speed, round 6: the cost of a check that misses, the check of a rename, memory

Status: accepted (2026-10-20)

## Context

[ADR-052](052-lsp-speed-work.md) took the rows of [docs/benchmarks/lsp.md](../benchmarks/lsp.md) to
gopls's level where the text of the edit was one that the server had seen: the keystroke scenario
types an `x` and takes it out again, so after the first two edits every check was a hit of the cache
that is keyed by the content of the files. An editor's text is new at each keystroke. The user asked
for the server to be as fast as it can be, and never to be wrong. This round measured what a check
that misses costs, and where the rest of the rows lose to gopls: the first request of its kind, the
rename of a name that many packages use, the tail (p95) of the keystroke, and memory.

## Measurement provenance

The original round-6 timings in **Decisions** are historical exploratory measurements, made with
generated modules declaring Go 1.22. They are not results of the current
`fresh-edits-v2-go1.25` protocol, which declares Go 1.25 identically for gocpp and gopls v0.21.1. A matching compiler version alone
does not make these workloads comparable. The checked-in benchmark page predates this round;
it is not a publication of the exploratory numbers below. New measurements must retain raw
samples, process RSS and peaks, binary/project/configuration identities and the execution log;
`lspbench -merge` rejects incompatible measured inputs and does not infer alternation from their
file order. Configuration values (environment, command arguments and settings) are stored as
named, domain-separated SHA256 fingerprints, not raw text: they can contain credentials.
Fingerprints are **not anonymization or cryptographic secret protection**; low-entropy values
can be guessed. Legacy raw-configuration reports are not accepted for merging or rendering.

The recovered correctness gate compared 1,141 answers per project on `small` and `medium`:
shared base `cbc374b5` and candidate `5ee48600` were identical. Comparing either with
then-current `dev` (`69364ffc`) gave the same four completion differences per project.
Full item-level evidence identified only new package suggestions from step 9.11 (`ffe1c8d5`): `math/rand`, `container/ring`,
`hash/crc32` and `text/tabwriter`, plus the resulting `sortText` renumbering of unchanged items.
No difference is filtered out by `lspdiff`: its verbose mode prints complete answers. The shared
base and candidate have identical standard-library, runtime and module data, so they form the
controlled recovered round-6 comparison. Those results did not establish equivalence to later
integrations or the combined behavior with the independently developed step 11.13. The
normal-loader follow-up below instead uses the frozen integrated baseline `e745f037`.
This is bounded differential evidence, **not a complete semantic-equivalence proof**: the diagnostics comparison keeps severity, start line
and message, not columns/end ranges/metadata, and dependent diagnostics still use fixed settling
sleeps. The corrected tool separately accounts for successful replies and explicit JSON-RPC
refusals; transport errors, missing results, timeouts and cancellations fail the run even when
both servers fail alike. The original gate was rerun with that reply-accounting correction.

## Decisions

1. **The benchmarks measure what an editor causes.** New rows `keystroke-new` and
   `keystroke-imported-new` (each edit writes text that was never in the file: a number or an
   undefined name with a number in it, in turn; the wait is for the diagnostics of that text), the rename
   scenario asks for a new name each time (a server that remembers the edits of a rename would answer
   at once), the completion after an edit writes a comment line with a number first, a keystroke is
   followed by 25 ms of think time (a fast typist) and the first scenario comes one second after the
   open (the user reads). `lspdiff` (tools/lspdiff) is the other half: two builds of the server are
   asked the same things (hover, definition, references, highlights, rename, completion,
   signature help, symbols, tokens, formatting, links, code actions, hints at about 40 places of 14 files,
   workspace symbols, the diagnostics of every open file) before and after 24 edits of the hot file and
   API changes; 2,376 answers on `small` and 6,366 on `medium` were identical to the build before this
   round.
2. **A check that misses does less** (the order is what they bought, measured with `BenchmarkEditMedium`,
   alternating, the minimum of 8 runs: 6.3 ms before, 4.2 ms after):
   - the parallel helper in `parsepar.go` was initially wired into `readXTest` only.
     Ordinary `loader.read` remained sequential: that helper did **not** account for a normal
     keystroke improvement in these original measurements. The follow-up below closes this gap;
   - the decoded imports of a package are kept for its next check when its imports' summaries are the
     same ones (`cache.importSet`; the fake files of their positions are added to the new file set):
     reading the export data was a seventh of the time and a fifth of the allocations;
   - `check.Config.SkipOpaque`: no opaque pass and no check of the skipped expressions for a package
     with no C in it or in its imports **and no error from go/types** (with an error a type can be
     invalid, and the pass calls what has one opaque, also without C: found by `TestSkipOpaque`, which
     compares 2,300 programs of the repository with and without);
   - the expansion of the built-in macros does not walk a package none of whose files has a `#`,
     the word `macro` or `go:embed` (`TestMacroWalkSkipped`: the load, printed, is the same);
   - the gates ask about the features of a type once (`typeUsesOnce`; `TestGateTypeMemo`: the gate
     errors of 2,300 programs at six phases are the same);
   - `checkSizeCParts` makes its table of field types when it reports a call, not before.
3. **A rename does not check what cannot break** (`renamefast.go`). The check of the result checked
   every package that the edits touch, one after the other when they import each other (a package
   waits for the export data of the one it imports): a function that the 11 packages of a chain call
   was 11 checks in a row, 56 ms on `medium` and 150 ms on `large`. For an exported function,
   variable or constant renamed to another exported name, a package whose uses are all the `Sel` of
   `pkg.Name` (pkg the import of the declaring package, under any name), and a searched package
   without a use that does not dot-import the declaring package, are not checked: a qualified name is
   looked up in the exports of the declaring package (which is checked), no declaration of the
   importer can capture it, and nothing else of the importer changes. Types (an embedded field takes
   the name of its type), methods and fields stay as they were. `TestRenameQualifiedOnlyShortcut`:
   the same answer with and without the shortcut for 20 renames, also with the conflict checks off,
   and fewer checks. Rename: 56 ms to 5 ms (`medium`), 150 ms to 5 ms (`large`).
4. **Memory.** `large` (90,000 lines) had a resident set of 430-770 MiB, for 155 MiB of live data:
   the Go pacer lets the heap double, and the check cache kept every version of every edited package
   until it was full (250 checks for 100 packages). A soft limit that follows the live heap
   (`debug.SetMemoryLimit`: 256 MiB, or 1.4 times the live heap and 64 MiB; not when the user set
   GOMEMLIMIT) and three checks kept for each package (`cache.keptVersions`: an edit and its undo
   still hit) make it 370 MiB (494 MiB with neither, measured alternating).
4a. **Patched-file variants have their own bounded ownership cohort** (step 11.13 memory follow-up, after the
   reviewed normal-parser integration). `PackageWithFile` continues to use its complete variant namespace,
   original-input, replacement-content, imported-API and configuration keys. Its successful and completed-error
   results now use a separate instance of the existing hash-only `versionKeeper`: at most the latest three
   patched-file versions per package, separately from the ordinary three edit/undo versions, in the same
   globally cost-capped memo store. This is not a second result cache or a changed global memory limit.
   `ForgetIf` removes only a completed entry's store ownership; it does not cancel an in-flight promise,
   mutate the returned package or invalidate a request/snapshot/repair-view holder. Pending computations may
   outlive a dropped keeper hash and are noted again on completion. Ordinary cache behavior and the global
   cap (including its existing one-oversized-value rule) remain unchanged. No request-time GC or pacer tuning
   is introduced. Deterministic/race tests cover unique/error variants, live holders, ordinary/variant isolation,
   original/config/API keys, reverts, concurrent snapshots, coalescing and canceled waiters. Separate profiling
   proof and joint latency/RSS measurements are recorded in [the step 11.13 evidence](../benchmarks/lsp-step-11.13.md).
5. **Warming.** After the first diagnostics of a package the server checks the packages that import it,
   in the background, one or two at a time, each when nothing else is in flight, at most 96 and
   while the check cache is under half full (`warm.go`; `-no-warm`): the first references of a name
   that every package uses, 92 ms to 40 ms on `large` (36 to 9 ms on `medium`). The warming of 96
   importers took 200 ms.
6. **Tried and kept off.** A garbage collection started by the server in the gaps between two
   requests (`-idle-gc`): in 6 alternating rounds the p95 of the keystroke was not lower
   (`large` 3.4 to 4.7 ms, `medium` 2.0 to 2.0), and it costs the CPU of more collections.
   Sizing the maps of `types.Info` from the last check: no change in 8 alternating rounds.

## Normal-loader follow-up (2026-10-20)

Ordinary `loader.read` now uses the same bounded parallel parser as `readXTest`, above the
6 KiB threshold and with at most GOMAXPROCS workers. Source selection, file-system/overlay
reads, build constraints and foreign-source collection still run serially before parsing.
Only captured bytes reach workers; package names, ASTs and errors are committed in source
order, and macro/import/foreign rewriting stays serial afterward. A reserved position range
protects a shared FileSet from concurrent additions; each worker parses in a private set and
its real token file is transferred back. Parser panics join all workers and rethrow on the
caller, preserving its recovery boundary rather than ending the server from a worker.

`TestReadNormalFilesInParallel` proves that the normal path admits two private parser workers
without timing assertions and preserves captured read/overlay/error behavior.
`TestReadParallelMatchesSequential` compares public loads at GOMAXPROCS 1 and 4: complete
rewritten ASTs, diagnostics/order, numeric and resolved positions, line tables, retained source,
imports/directives, macros, foreign files, named order, overlays, external tests and standard
packages. Separate tests cover shared allocation and worker-panic recovery. These are bounded
fixtures, not a repository-wide corpus sweep. No checker/cache or rename guards changed.

The frozen baseline is `e745f037`, including the native-fmt integration, not `cbc374b5` or an
unmerged historical build. Baseline and candidate used identical current stdlib/runtime/module
data, Go 1.25.14 and generated Go 1.25 modules. A bounded `lspdiff` run compared 2,395 answers
per project (`small`, `medium`), with zero differences or failed replies; the diagnostic/settling
limits described above still apply. Complete reply accounting and full raw JSON response
comparisons are retained with the evidence.

Four balanced baseline/candidate pairs, serial under the shared timing lock on cores 16–23
with GOMAXPROCS 8, measured only the three scenarios below (40/20/30 measured samples per
scenario/process). Numbers are milliseconds, medians of four runs' own p95, not pooled p95:

| Project | Fresh edit, baseline → candidate | Imported fresh edit | Completion after fresh edit |
|---|---:|---:|---:|
| small | 4.533 → 4.232 | 4.830 → 4.445 | 5.212 → 4.730 |
| medium | 7.062 → 6.305 | 6.291 → 5.736 | 8.565 → 7.424 |

The startup/open rows collected automatically are not a full speed-contract run. No gopls,
large/C++ or all-row comparison was made. Small RSS medians were 61.18 → 60.39 MiB; medium
173.31 → 180.54 MiB (largest observed peaks 185.38 → 186.17 MiB). Separate instrumented
medium sessions, excluded from the timing aggregates, showed load-span medians after the first
second of 1.472 → 0.889 ms and type-check medians 3.480 → 3.528 ms; their span counts were
213 and 214. CPU samples totaled 2.82 and 3.16 seconds, so this establishes neither a CPU-use
reduction nor a memory improvement. It is bounded wall-latency evidence, not proof that every
budget or the beat-gopls-on-every-row goal is met.

## Cached declaring-scope rename proof (2026-10-20)

The next frozen baseline, `9a2b0871`, includes the normal-loader follow-up. A targeted medium
rename profile found one fresh declaring-package check for each of 201 renames (200 measured
plus one warm-up): median rename 4.457 ms, of which the package check was 3.925 ms. Most
importers already satisfied `qualifiedOnly`; repeatedly checking the declaring package remained
the dominant cost.

A second proof now reuses the existing `checkDeclaration`, `checkCapture`, `checkUse` and
test-declaration guards, rather than replacing them with another lookup algorithm. It applies
only to an exported, nongeneric, package-level ordinary function with a body, renamed to
another exported identifier other than reserved `C`, in a domain-qualified module path.
Standard/runtime-shaped and command-line paths retain full checks because compiler rules can
give particular function spellings extra meaning. Every remaining package must satisfy the
existing qualified/no-dot-import proof. The prior mixed qualified/dot-import counterexample
therefore still takes scratch validation.

All searched views must be the exact complete checked AST/type-info/file-set views, with no
errors, partial/provisional/stale checks, repaired trees, opaque expressions/types, known
foreign rewrites, directives, macros or bang origins. The declaring snapshot must establish
that it has no test files, and no test checks may be involved. A package memo stores only
immutable checked-package eligibility facts; mutable probe state is checked separately.
The step-11.13 combined integration additionally records immutable patched-input origin in the computed
check key and copies it to `Package` before memo publication. `Package.IsVariant` is rejected before the
ordinary-facts memo, even for byte-identical or complete plain-Go overrides. Cached hits, late completions,
evicted/recomputed variants and active holders keep that origin; no caller/view label can relabel them.
This closes a conservative origin gap found by an integration probe without weakening eligible ordinary
positives, which still prove zero declaring-package scratch checks against a positive full-check count.
The checked identifier index must show that every own-package declaration/use of the function
is an edit, with a known scope. Unknown or missing facts fall back to the unchanged scratch
checker. Binding is unchanged, so this function-name change changes neither its type nor any
method set, initializer dependency or import usage. Scope/import collisions and capture still
fail through the existing guards before the proof is considered.

This is the checked Go/GP binding model, like the full validator: it does not discover or
rewrite references in untracked C++ companion files. It is not a C++ refactoring guarantee.
`TestRenameDeclaringBindingProof` compares positive edits and negative refusals with the full
validator, including declaring/file-import/dot-import collisions, same-typed local capture,
generic caller capture, Unicode/UTF-16 positions, the prior mixed-import counterexample and
conservative fallbacks. Separate tests force a missing occurrence, disabled conflict guards
and cancellation. The proof suite was repeated ten times with the race detector. A bounded
baseline/candidate differential run compared 2,395 answers on each of small/medium, with no
differences or failed replies; its diagnostic/settling limitations still apply.

Final timing used two balanced pairs, cores 16–23/GOMAXPROCS 8, Go 1.25.14 and generated Go
1.25 modules, identical current data/configuration, and 80 unique prepare+rename samples per
project/process. Every timed RPC succeeded; no matching errors/refusals count as samples.
The earlier four-pair prototype predates the final path restriction and is retained separately,
not pooled into these figures. Milliseconds, medians of the two runs' own statistics:

| Project | p50 baseline → candidate | p95 baseline → candidate | First request |
|---|---:|---:|---:|
| small | 2.951 → 0.151 | 3.727 → 0.232 | 4.219 → 1.518 |
| medium | 4.617 → 0.256 | 6.835 → 0.324 | 11.659 → 6.047 |

RSS medians were 50.82 → 42.35 MiB (small) and 134.37 → 100.66 MiB (medium); maximum
observed peaks were 51.27 → 42.61 and 134.74 → 100.75 MiB. Separate instrumented medium
sessions, excluded from the quantiles, recorded 201 declaring-package checks → none after
opening; CPU samples totaled 2.32 → 0.52 seconds. These are bounded rename-only results,
not a gopls comparison or an all-row speed-contract claim. Other rename forms retain full
validation and can remain substantially slower.

## Not done, and why

- **Checking only the function bodies that an edit changed.** go/types has no entry for one body, and
  the pieces around it need what only a whole check has: the initialization cycles of the package
  (dependencies of the bodies), the unused imports (uses in every body), the gates' "first use in each
  file", the positions of every object of a file after the edit (the types and the `types.Info` of
  the other functions are tied to the positions of the old text). A path that composes the old and the
  new diagnostics would have to rebuild each of them and could be wrong in a rare case; the rule of
  this round is never wrong. The honest result: on a package that is edited, gocpp takes 3.6 ms
  (`small`), 4.3 ms (`medium`), where gopls takes 2.2 and 2.9; on `large` gocpp is ahead.
- **One decoded universe of imports for all packages**: the export data of a package can add
  declarations to the scope of a package it imports while it is decoded, so the packages could not be
  shared by concurrent checks.
- Parsing without the identifier resolution of the parser (`SkipObjectResolution`, 5 % of the
  check): `check` reads `Ident.Obj` of the name `C`; with a user's `C` (an error) the rewrite would
  differ. Left.

## Consequences

The harness emits the new rows; `docs/benchmarks/lsp.md` remains the pre-round snapshot until
new comparable measurements are published. The budgets of the gate
(`internal/lsp/bench/budget.go`) were lowered to what the exploratory rows allow with the gate's
three-fold safety. Passing that noise-tolerant gate does not establish every Phase 11 budget or
the goal of beating gopls on every row.
