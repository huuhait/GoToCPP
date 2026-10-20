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

The timings quoted below are **historical exploratory measurements**, made with generated
modules declaring Go 1.22. They are not results of the current `fresh-edits-v2-go1.25` protocol,
which declares Go 1.25 identically for gocpp and gopls v0.21.1. A matching compiler version alone
does not make these workloads comparable. The checked-in benchmark page predates this round;
it is not a publication of the exploratory numbers below. New measurements must retain raw
samples, process RSS and peaks, binary/project/configuration identities and the execution log;
`lspbench -merge` rejects incompatible measured inputs and does not infer alternation from their
file order. Configuration values (environment, command arguments and settings) are stored as
named, domain-separated SHA256 fingerprints, not raw text: they can contain credentials.
Fingerprints are **not anonymization or cryptographic secret protection**; low-entropy values
can be guessed. Legacy raw-configuration reports are not accepted for merging or rendering.

The recovered correctness gate compared 1,141 answers per project on `small` and `medium`:
shared base `cbc374b5` and candidate `5ee48600` were identical. Comparing either with current
`dev` (`69364ffc`) gave the same four completion differences per project. Full item-level evidence
identified only new package suggestions from step 9.11 (`ffe1c8d5`): `math/rand`, `container/ring`,
`hash/crc32` and `text/tabwriter`, plus the resulting `sortText` renumbering of unchanged items.
No difference is filtered out by `lspdiff`: its verbose mode prints complete answers. The shared
base and candidate have identical standard-library, runtime and module data, so they form the
controlled round-6 comparison. A comparison against current dev requires integration and a fresh
semantic gate; these results do not establish current-dev equivalence or the combined behavior
with the independently developed step 11.13. This is bounded differential evidence, **not a
complete semantic-equivalence proof**: the diagnostics comparison keeps severity, start line
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
   - a parallel parser helper (`load.parseFiles`; private file sets with the same bases as a
     sequential parse: `TestParseFilesParallel`) is currently wired into `readXTest` only.
     **Ordinary package loading remains sequential** in `loader.read`; parallelizing that path
     is a plan/implementation gap, not an achieved normal-keystroke optimization;
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
5. **Warming.** After the first diagnostics of a package the server checks the packages that import it,
   in the background, one or two at a time, each when nothing else is in flight, at most 96 and
   while the check cache is under half full (`warm.go`; `-no-warm`): the first references of a name
   that every package uses, 92 ms to 40 ms on `large` (36 to 9 ms on `medium`). The warming of 96
   importers took 200 ms.
6. **Tried and kept off.** A garbage collection started by the server in the gaps between two
   requests (`-idle-gc`): in 6 alternating rounds the p95 of the keystroke was not lower
   (`large` 3.4 to 4.7 ms, `medium` 2.0 to 2.0), and it costs the CPU of more collections.
   Sizing the maps of `types.Info` from the last check: no change in 8 alternating rounds.

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
