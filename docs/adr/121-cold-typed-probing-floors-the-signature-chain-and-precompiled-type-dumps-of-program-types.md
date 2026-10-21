# ADR-121: Cold typed probing: signature types beside the probe run, precompiled type dumps of program types, and the measured floors

Status: accepted (2026-10-21). Step 12.14b. Builds on [ADR-117](117-typed-probes-load-a-precompiled-header-of-the-library-block-and-type-runs-for-package-types-skip-it.md) (the safe PCH; the withdrawn smaller preamble must not come back) and [ADR-108](108-typed-interop-becomes-the-default-when-three-measurements-pass-and-the-clang-policy-is-a-matrix.md) (measurement 2). The contract is in [typed-interop.md](../spec/typed-interop.md) §2.2, §9 and §14.3.

## Context

The open exit criterion of Phase 12 says that cold typed probing costs less than 0.5 s per example package (typed minus opaque). ADR-117 left raylib at 0.85 s fully cold and 0.65 s with a retained PCH, and every other row was historical. This step re-measured all five examples that import a header and looked for reductions that keep every answer, under the rules of ADR-117: equal preprocessed text does not prove equal C++ lookup, so no run may use a smaller or different preamble, and a run that cannot be trusted is made again with the full one.

Where the time goes (timeline of the compiler runs with a wrapper script, Linux/amd64, clang 21, a cold cache, `raylib`; the other examples have the same shape):

| stage | what | clang runs | time |
|---|---|---|---|
| before the first probe | `--version`, `-dM -E`, Go checking, `-E -v` (toolchain) | 3 small | about 110 ms (first run on a machine only: both are cached) |
| kind probe | Phase 06's probe; no library block | 1 | 37 ms (raylib), 200 ms (mixed-sources, twice), 450 ms (cpp-std: its own `<algorithm>`, `<filesystem>`, `<map>`) |
| PCH build | the 18 library headers once, beside the next stages | 1 | 390 to 410 ms |
| probe stage | expression probes and overload sets (`gp_probe_`, `gp_f`), in parallel | 2 | 340 ms without the header, 140 ms with it |
| type stage | one run per last type name, in parallel | 5 (raylib) | 330 ms without the header, 140 ms with it, 520 to 560 ms for std types (more JSON) |
| more type stages | types named by the signatures and members of the first | 0 to 2 | 330 ms each, before this step |
| tails | finalDeps, cache writes, includes question | 0 | 20 to 35 ms after each stage |

Floors measured with the same compiler: an empty file costs 22 ms; the library block alone 315 ms (`-fsyntax-only`); making its PCH 390 ms; loading it 62 ms; a type dump of `Vector2` with the header 136 ms and without 340 ms.

## Decisions

1. **The types named by the signatures of the called functions are asked beside the probe run** (`typedinterop.startFuncDecls`). They depend only on the overload sets, so their run starts when the function run is done and goes beside the end of the probe run and the first round, instead of in a round of its own after them. The first round no longer waits for it and leaves out the names it covers. This removes a whole 330 ms stage for `sqlite` (`sqlite3_int64`) and `mixed-sources` (`span_stats`). Every name is asked once; a failed or interrupted call stops the compilers of this work and joins its goroutine before `Analyze` returns. The depth of the chain is that of the old rounds plus one level in rare cases; the answers are a superset of the old ones. Cost: a name that is asked for under each enclosing scope may now also ask the outer candidate that the old order would have skipped, because the type was already known by then; it is a failing query in the same run (three mapped goldens gain one `no answer: unknown type name` line, no declaration changed).
2. **Type runs of the program's own types load the PCH, and use the answer only when it is whole** (`probe/decls.go`, `probe/pch.go`). The file is the one a full run reads and the header is the state of its first 18 lines, so lookup is the same; the only difference is what the JSON dump prints. A run is tried with the header when none of its names is in `std`, begins with an underscore or has template arguments (those are in the header, or located at their template, and would be refused). Its answer is used only when every declaration it describes has a location, and every file in it (also the two files of a macro expansion) exists and is none of the files the header was made of, compared by file identity (`os.SameFile`; the spelling, a link or a relative name does not hide it; a file that cannot be looked at counts as the header's). Otherwise the whole run is made again without the header and counted (`Counters.TypeRechecks`). A run whose compiler refuses the header uses ADR-117's retry. The cache keys, the dependency lists and the part stored are those of a run without the header.
3. **Not done, with the reason.** The type dumps of the library's types (they stay full: the specialization's location is its template, so a header-resident instantiation cannot be told from one made in the file); a smaller preamble (withdrawn in ADR-117, unsound); speculative type names from a header-only parse (it would be a second implementation of the probe, and only saves a stage that needs two runs to confirm); merging the kind probe into the probe file (Phase 06's, and the probe lines depend on its answers); asynchronous toolchain question and parallel `--version`/`-dM` (first run on a machine only, 20 to 40 ms); writing the cache entries behind the lock (10 ms per stage, a change of the lock protocol).

## Measurements

Typed minus opaque, in seconds. Medians of 9 interleaved runs (one opaque, one typed before, one typed after, per round), Linux/amd64, clang 21, 192 logical CPUs, no CPU pinning (the probes are parallel on purpose), load average 6 to 15 from other jobs on the shared machine, caches in new folders for each run. "Fully cold" is an empty cache folder: it includes the first PCH build, the compiler detection and the toolchain question. "Retained PCH" keeps only `probe/typed-v1/pch` (as `BenchmarkTypedCheck` `typed-cold-pch` does): the state of a second package, or any later first check on a machine that has already made a header for its flags. "Before" is `a696098f`.

| example | fully cold before | fully cold after | retained PCH before | retained PCH after | warm before / after (typed, ms) |
|---|---:|---:|---:|---:|---:|
| `raylib` | 0.843 | **0.659** | 0.641 | **0.456** | 61 / 58 |
| `sqlite` | 1.180 | **0.652** | 0.977 | **0.438** | 65 / 58 |
| `multi-pkg` | 0.476 | **0.468** | 0.241 | **0.247** | 38 / 36 |
| `mixed-sources` | 1.536 | **1.169** | 1.530 | **1.152** | 60 / 52 |
| `cpp-std` | 1.586 | **1.560** | 1.592 | **1.519** | 66 / 52 |

Raw medians (ms): opaque cold `raylib` 66, `sqlite` 61, `multi-pkg` 72, `mixed-sources` 233, `cpp-std` 413; opaque warm 39 to 48. Typed cold after: 725, 713, 540, 1402, 1973. The warm overhead over opaque warm is 0 to 18 ms, under the 50 ms criterion. A repeat of `raylib` and `sqlite` with 15 interleaved rounds gave the same medians within 15 ms. An earlier, non-interleaved set of 7 runs per cell (a binary after the other, load 20 to 30) gave lower numbers for the after columns (`raylib` 0.61 and 0.46, `sqlite` 0.55 and 0.37, fully cold and retained) and higher for the before columns by up to 0.17 s: the shared machine moves a cell by about 0.1 s, so interleaved rounds are the ones quoted.

## The floors, and what the criterion can say

- **Fully cold, `raylib` and `sqlite`.** After this step the compiler runs are: the kind probe (37 ms), the probe stage without a header (340 ms; the header that exists only 390 ms after it was started cannot help it, and building it first would delay the stage to 140 ms after 400 ms), and one type stage with the header (140 ms). With every other cost made zero the sum is 37 + 340 + 140 = 0.52 s, and the measured tails (about 25 ms after each of two stages, 50 to 110 ms before the first probe) bring it to the measured 0.65. A typed-minus-opaque time of 0.5 s is therefore **not reachable soundly for an empty cache**: the type stage cannot start before the probe stage tells which types it needs (a run is named by a last name that comes from the probe results), and each stage costs at least the compile of the block or the load of its header.
- **Retained PCH, C-style headers.** Two stages of 140 ms and the kind probe: measured 0.44 to 0.46 s, met, with 40 to 60 ms of that being the first-run costs (compiler detection and the toolchain question, which the benchmark's empty cache also pays and a real second run does not). `multi-pkg` is 0.25 s.
- **C++ library headers (`cpp-std`, `mixed-sources`).** The kind probe alone parses the package's own `<algorithm>`, `<filesystem>`, `<map>` (450 ms) or runs twice (2 x 200 ms, the second because a header package is also a namespace); the probe stage is 160 to 370 ms because it instantiates the templates; and the type runs of std names (`vector`, `map`, `pair`, `basic_string`, `path`) parse the full block and write 12 to 32 MB of JSON (520 to 560 ms). Their floor is about 1.2 s over opaque; no sound change brings it under 0.5 s, and `std::basic_string` is answered with "larger than the limit" at the cost of the whole run.

**Proposed wording of the criterion** (for the manager to decide; STATE.md is not edited here): "Warm builds add less than 50 ms. Typed probing of a package whose headers are C-style or its own, with the library header already made for its flags (any later check on a machine), adds less than 0.5 s over opaque on an empty result cache. The first check on a machine, which makes that header, adds less than 0.7 s. A package that includes large C++ library headers itself (`<algorithm>`, `<filesystem>`, `<map>`) is bounded by the compile of those headers: about 1.2 to 1.6 s." That records the measured state without weakening any answer. Making the first check cheaper is a separate decision: the header could be made in the background by any gocpp command that sees a header import (including an opaque build or the language server), so that the first typed check finds it; it is not done here.

## Consequences

- The default stays opaque on this criterion alone; ADR-108's other two measurements are unaffected.
- A type run that is refused costs one extra run in sequence (`Counters.TypeRechecks` counts exactly these; a compiler error with the header also falls back to the full run but is not counted, and a canceled call returns the cancellation without either). In the 288-program corpus 8 type runs were refused and made again; all documents equal.
- The oracle `TestPCHOracle` (288 programs, both modes, 0 differences, with exact dependency lists) and a comparison of `gocpp __typed-probe -decls -stable` between `a696098f` and this branch on the five examples (identical; the dependency lists hold the same files, and `mergeDecls` now lists each once, sorted, where the branch first repeated an entry for every merged table) are the evidence that no answer changed.
- Disk and compilers: unchanged from ADR-117 (up to 4 headers of about 8 MB; clang 21 tested, 17 to 20 untested).

## Evidence

Tests (all deterministic: counts, markers and file identity, no clock): `TestSignatureTypesAreAskedBesideTheProbeRun`, `TestSignatureTypesAreAskedOnce`, `TestInterruptedAnalyzeLeavesNoCompiler`, `TestFailedProbeJoinsTheFunctionWork` (`internal/typedinterop/chain_unix_test.go`, a compiler script that blocks the probe run until the type run has started); `TestPCHTypeRunsOfProgramTypesAnswerAsWithout`, `TestPCHTypeRunMayLoadHeader`, `TestPCHDeclarationsFromProgramFiles`, `TestPCHTypeRunsKeepTheMembers`, `TestPCHAnswersAreTheSameAsWithout` (`internal/typedinterop/probe`); `TestPCHOracle` (`tests`). Mutation checks made for each mechanism are listed in the coder report. Benchmarks and scripts: `BenchmarkTypedCheck` (`cmd/gocpp`).
