# Phase 12: Typed C++ Interop via Clang · STATE

**Status:** `[~]` in progress (12.1-12.12 and bounded 12.13a header/namespace completion merged; safe PCH/prefetch part of 12.14 integrated; member completion, header navigation, cold-performance and remaining exit criteria are open)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-20

## Steps

- [x] Spike: clang `decltype` probes + filtered JSON dump (0.12 s, 28 KB vs 162 MB unfiltered) on 2026-10-13
- [x] Spike: macro-value probe `gp_val<BUFSIZ>` → `8192`, `gp_val<EOF>` → `-1` (from the `ConstantExpr` value) (2026-10-13)
- [x] Audit re-check (clang 21): integer values from `ConstantExpr`, exact floats via `bit_cast`, string macros from `StringLiteral`, `-ast-list` index 20,924 names in 0.13 s, `-dM` 1,639 macros in 0.02 s, `-Xclang -code-completion-at` works (`spikes/2026-10-13/audit-checks/clang-probes`)
- [x] 12.1 Reference collector (expression shapes, operators, expressions touching opaque values)
- [x] 12.2 Probe generator + runner + streaming JSON reader
  - [x] expression probes with lvalue/rvalue arguments mirroring the generated code
  - [x] constant probes: integers (`ConstantExpr` value), floats (`bit_cast` into an integer template argument), strings (`StringLiteral`)
  - [x] type probes
- [x] 12.3 Record / enum / function extraction (filtered by used names)
- [x] 12.4 C++ → foreign type mapper (`T&` results addressable, templates, overload sets)
- [x] 12.5 Interop checker (exact-mirror errors only, operator typing with C++ conversions, g++ builds → warnings, compatibility rule)
- [x] 12.6 Codegen upgrades (`.`/`->`, designated init order, template/index, auto `const char*` except "keeps the pointer" list, references)
- [x] 12.7 Cache + incremental probing + parallel probing (header contents in the key)
- [x] 12.8 Gap E1: constant macros probed first, substituted before go/types
- [x] 12.9 Gap E2: second type-check pass for promoted C++ members
- [x] 12.10 `gocpp bindgen` (struct-valued macros as `var`)
- [x] 12.11 `-interop=typed|opaque` + clang-missing fallback + default switch
- [x] 12.12 Tests (merged 2026-10-20; spec §14 maps each criterion to its test)
  - [x] Phase 04 suite + header zoo in both modes (check-only sweep of 287 programs with g++ and clang++; build-and-run half is 12.11's `TestCorpusTyped`)
  - [x] ≥ 30 early-error tests + compatibility tests (77 diagnostics in 22 reject programs; `T&` params/results, mixed arithmetic)
  - [x] E1 (`[cstdio.BUFSIZ]byte`) and E2 (embedded `std.vector[int]`)
  - [x] float and string macros; header edit invalidates probes
  - [~] probe speed benchmarks done; clang 17–21 JSON compatibility: tolerance tests + `tools/clang-matrix.sh` ready, only clang 21 exists in the dev shell (17-20 untested)
- [~] 12.13 Language-server integration
  - [x] 12.13a Background header index from `-Xclang -ast-list` + `-E -dM` (on-disk cache); clean review and focused manager integration checks
  - [x] 12.13a Opt-in header-package (`raylib.`) and namespace (`std.`) completion from completed immutable facts
  - [x] 12.13b1 Bounded receiver-member probe primitive: compiler-validated accessible names, explicit partial results, clean independent review and focused manager checks; not yet editor member completion
  - [~] 12.13b2 Background receiver facts and member completion on C++ values; preserve request responsiveness and explicit partial-result status
  - [ ] hover + go-to-definition into headers
  - [ ] C/C++ budget row (< 2 s, background) + slow-clang test; Phase 11 budgets still met
- [~] 12.14 Cold typed-probing improvements (ADR-117)
  - [x] Safe expression/function PCH and overlapping dependency discovery: whole-net round-4 review clean; manager focused and combined LSP/probe checks passed
  - [x] Type dumps retain the full preamble; the unsound smaller-preamble optimization and stale approval cache were removed
  - [ ] Cold < 0.5 s per example: latest raylib overhead is 0.849–0.852 s fully cold, or 0.648–0.649 s with a retained PCH (not fully cold)

## Exit criteria

- [ ] Phase 04 suite + header zoo green in typed and opaque modes
- [x] E1 and E2 work with stock go/types (12.12)
- [x] ≥ 30 early-error tests; no typed-mode error on programs that compile in opaque mode (77 diagnostics; 287-program sweep, 0 typed errors)
- [~] Warm overhead < 50 ms (met: 0-30 ms); cold probing < 0.5 s per example package (NOT met: 0.5-2.0 s typed minus opaque; clang runs ~14 times for cpp-std; see decisions)
- [ ] `bindgen` output reviewed for raylib, sqlite3, SDL
- [ ] Editor: `raylib.`/`std.` completion works; C/C++ data < 2 s; no request waits on clang; Phase 11 budgets hold

## Decisions / notes

- 2026-10-20: 12.13b1 integrated from reviewed source `4538ce11` after clean revision-2
  whole-net review. `Runner.Members` enumerates candidates and validates unqualified
  access/availability with bounded dependent requires-expressions. It excludes ambiguous
  inherited names and deleted/unavailable-only groups, retaining names with a proven usable
  overload. Unsupported argument groups are omitted with explicit `Incomplete()` status,
  preserved in `member-names-v2` cache entries; complete-empty, partial-empty and failure
  remain distinct. No argument types or signatures are invented. Supported receiver types
  retain their cv/value category and explicit-pointer access; broader forms remain unsupported.
  Manager checks passed: all four files match the reviewed snapshot; real receiver/category,
  inheritance/overload/partial-result and const-vector cases, validation rejection, context
  invalidation, cache reuse, cancellation/shared-include race checks, PCH keys and existing
  header completion/blocked-compiler tests. No selected case skipped. This remains a
  synchronous low-level primitive that request handlers do not call. Background receiver
  inference/publication is 12.13b2; hover/definitions, VFS, Windows and speed acceptance
  remain open. No full sweep or new performance measurement was run.

- 2026-10-20: Bounded 12.13a integrated from reviewed source `bfff8002` after clean
  revision-5 whole-net review. Header and namespace names are indexed asynchronously
  only with the existing typed/auto opt-in; ordinary request paths read completed immutable
  facts and the default remains opaque. Namespace aliases/using-directives use a finite,
  declaration-ordered canonical graph, including frozen aliases and ambiguity handling.
  Required input/resolution identities, complete compiler validation, owned compact names,
  bounded caches and cancellation prevent the previously reproduced stale/partial results
  and retained-buffer growth. Cache identity is `header-index-v5`.
  Manager checks passed on the integrated tree: all 17 files matched the reviewed snapshot
  with the existing `auxiliary.go` rename preserved; real completion, header refresh,
  namespace/order/ambiguity regressions, ordinary request progress during a blocked compiler,
  shutdown/join, focused race/invalidation/byte-bound tests, existing PCH keys and shared
  include cancellation, and 224 docsite pages. No selected case skipped. Reviewer also
  exercised four standard headers and the earlier dependency/symlink/partial-output cases.
  These are focused correctness checks, not full latency, Windows or corpus acceptance.
  Non-`.gp` overlays still suppress names; member completion, detailed hover/definitions,
  signatures/UTF-16 locations and fresh-snapshot availability remain open. Split the next
  member work into 12.13b1 (bounded receiver-member query primitive) and 12.13b2 (background
  receiver facts and LSP publication), keeping synchronous clang work off request paths.

- 2026-10-20: Safe partial 12.14 implementation (ADR-117) integrated from source `d13cf67a`
  after clean whole-net round-4 review. Expression/function probes may reuse a target-aware
  PCH, but every type dump retains the full preamble. The smaller-preamble optimization was
  withdrawn because equal preprocessed text does not prove equal C++ lookup. Shared include
  prefetch now has cancellation-aware, reference-counted waiters and immutable published
  answers. Manager focused race/PCH/cache/header-edit checks passed, along with one raylib
  oracle case, five compiler-free warm checks and a Windows-target cross-clang header probe.
  Combined checks after the 11.14 merge passed: environment listings, warm probes, target
  flags and LSP line-mapping diagnostics. These are not a current full corpus, Windows-host
  run or benchmark matrix. Raylib's current narrow timing remains above the cold target;
  other examples' old figures remain historical. The performance exit criterion stays open.

- 2026-10-20: 12.12 merged (tests, docs, `tools/clang-matrix.sh`; no product change). Cold typed
  probing measured: cpp-std 2.0 s, mixed-sources 1.8 s, sqlite 1.3 s, raylib 1.0 s, multi-pkg 0.5 s
  over opaque; a PCH of the preamble costs 0.62 s once and then a probe run takes 0.10 s instead of
  0.51 s. Open decision: parallelize/merge the three sequential probe stages, add a PCH, prime the
  cache, or change the criterion (ADR-108 measurement 2); this blocks flipping `DefaultMode` to auto.
  Clang 17-20 are untested (dev shell has 21 only); the matrix script and tolerance tests are ready.
  Old note about named types over C (`type Handle geo.handle`) could not become tests: `gocpp build`
  refuses them ("not supported yet"), `gocpp check` accepts them silently.

- 2026-10-13: Compatibility rule. Typed mode never rejects what opaque mode + the C++
  compiler accepted; boundary conversions follow C++ implicit rules (narrowing → vet warning).
- 2026-10-13 (audit): probes mirror value categories (`declval<T&>` for variables); `T&` results
  stay assignable; operators follow C++ conversions; probe failures are errors only for exact
  mirrors (warnings with g++ builds); constants read from `ConstantExpr` (floats via `bit_cast`,
  because computed floats print with 7 digits; checked with clang 21);
  header contents in the cache key; header index from `-ast-list` + `-dM`; E1 and E2 solved on
  stock go/types, because a go/types fork would break x/tools for Phases 11 and 13; size raised.
- 2026-10-14 (from the 1.15 review): until this phase, go/types errors on C values, also on named types
  over C (`type Handle C.int`), are dropped in `gocpp check`, and the C++ compiler reports them (Phase 04
  on) at the .gp line. The interop checker here gives the early, typed errors. Test cases to cover:
  `var h Handle = "s"`, `h + "x"`, `var h Handle = goStruct`, `f(h)` with `f(string)` (C++ errors
  today), and the silent numeric ones `var h Handle = 1.5` (truncates) and `h = n` (int64 → int narrowing,
  a `gocpp vet` warning per §2.3).
