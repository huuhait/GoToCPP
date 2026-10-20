# Phase 12: Typed C++ Interop via Clang · STATE

**Status:** `[~]` in progress (12.1-12.12 merged; 12.14 cold-probing improvements in review-fix round; 12.13 and full exit criteria remain open)
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
- [ ] 12.13 Language-server integration
  - [ ] header index from `-Xclang -ast-list` + `-E -dM` (on-disk cache)
  - [ ] header-package (`raylib.`) and namespace (`std.`) completion from the index
  - [ ] member completion on C++ values (`-Xclang -code-completion-at` fallback)
  - [ ] hover + go-to-definition into headers
  - [ ] C/C++ budget row (< 2 s, background) + slow-clang test; Phase 11 budgets still met
- [~] 12.14 Cold typed-probing improvements (ADR-117): round-3 fixes remove the smaller
  preamble and its stale decision cache and make prefetch cancellation-aware. Independent
  re-review underway. Current raylib cold overhead is about 0.85 s; retained-PCH overhead is
  about 0.65 s, not a cold first run. Every-example cold < 0.5 s remains unmet.

## Exit criteria

- [ ] Phase 04 suite + header zoo green in typed and opaque modes
- [x] E1 and E2 work with stock go/types (12.12)
- [x] ≥ 30 early-error tests; no typed-mode error on programs that compile in opaque mode (77 diagnostics; 287-program sweep, 0 typed errors)
- [~] Warm overhead < 50 ms (met: 0-30 ms); cold probing < 0.5 s per example package (NOT met: 0.5-2.0 s typed minus opaque; clang runs ~14 times for cpp-std; see decisions)
- [ ] `bindgen` output reviewed for raylib, sqlite3, SDL
- [ ] Editor: `raylib.`/`std.` completion works; C/C++ data < 2 s; no request waits on clang; Phase 11 budgets hold

## Decisions / notes

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
