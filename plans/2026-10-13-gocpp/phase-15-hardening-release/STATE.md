# Phase 15: Optimization, Hardening & Release · STATE

**Status:** `[~]` in progress (15.1-15.3, 15.5, 15.7a, 15.8-15.11, 15.13-15.15 and 15.17 merged; 15.6a merged and 15.7b in review; remaining implementation/release steps and exit criteria are open)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-20

## Steps

### Performance
- [x] 15.1 Benchmark suite (gocpp vs Go vs C++) + per-commit tracking on a fixed machine + 5 % regression gate
- [x] 15.2 Escape analysis (unknown code = escaping) + `-gp:m`: clean whole-net review and focused manager checks; conservative stack, closure and element-reference optimizations integrated
- [x] 15.3 Receiver and parameter optimizations (whole integrated/test-reliability review clean at `e91d0ced`; manager focused production, fixture and corrected generator checks passed)
  - [x] non-escaping pointer receivers/parameters use `gp::bptr<T>` rather than `this` (ADR-086); callers retain owners unless stability/purity proves lending safe
  - [x] `const&` ABI with owning snapshots for mutable/captured sources and implicit foreign work; defer/go collect owning values
  - [x] targeted pointer/value generators and owner-drop regressions under ASan; corrected real-runner smoke passed, not a full campaign
- [ ] 15.4 Intrusive refcount experiment (keep only if it wins)
- [x] 15.5 Swiss-table maps
- [~] 15.6 Devirtualization + bounds-check help
  - [x] 15.6a Proven-concrete interface-call devirtualization; dynamic fallback and ownership preserved, clean independent review and focused manager source/integration checks
  - [ ] 15.6b Bounds-check help
- [~] 15.7 Compile-time work (`extern template`, ccache/sccache, C++20 modules evaluation)
  - [x] 15.7a Existing external-cache launcher workflow documented and tested; clean review and manager focused checks, no redundant configuration knob
  - [ ] Real ccache/sccache statistics remain unmeasured: neither tool was available in the checked environment
  - [~] 15.7b Generic-type body placement: move larger method bodies out of repeatedly parsed headers, preserving small-method inlining and instance identity; focused correctness and matched compile-time checks required
  - [ ] 15.7c C++20 modules evaluation remains separate, unstarted work

### Robustness
- [x] 15.8 Fuzzing (parse → check → lower → emit; sampled C++ compiles)
- [x] 15.9 Random-program differential testing vs `go run` (batched programs, no implementation-defined cases)
- [x] 15.10 Port Go `test/` run-tests as a conformance suite (downloads the pinned Go source)
- [x] 15.11 Nightly ASan/UBSan/TSan/LSan (fail on any report) + leak audit

### Platforms & packaging
- [ ] 15.12 Linux arm64 CI; Windows (Phase 14) in the release pipeline
- [x] 15.13 `go install`, GoReleaser binaries (linux amd64/arm64, windows amd64), Nix flake package with wrapped compilers
- [x] 15.14 `gocpp get` + `gocpp.sum` + minimal version selection (`x/mod` workarounds for dotless paths and `gocpp.mod` boundaries)

### Docs & release
- [x] 15.15 Docs site (tour, spec, interop guide + cookbook, Go-dev and C++-dev guides, stdlib reference)
- [ ] 15.16 v1.0 compatibility promise + changelog + release
- [x] 15.17 Post-1.0 roadmap ADRs

## Exit criteria (v1.0)

- [ ] Benchmarks published; goals (geomean ≤ Go, nothing > 2× slower) evaluated; no unexplained regression > 5 %
- [ ] ≥ 90 % of applicable Go `test/` run-tests pass
- [ ] 24 h fuzzing clean; ≥ 1M random differential programs, all mismatches triaged; targeted generators ASan-clean
- [ ] CI green on Linux x86-64, Linux arm64, Windows x86-64 (Wine + real-Windows checklist)
- [ ] Installable via `go install`, release binary, `nix run` (also on NixOS); docs site live

## Decisions / notes

- 2026-10-20: 15.6a integrated from reviewed source `01ec0226` (ADR-118). A direct adapter
  reuses the existing owning method-table thunk only for tightly proven same-package,
  non-generic concrete receivers. Receiver/argument ordering, nil wrappers, recover tickets
  and trace identity are preserved; other calls keep dynamic dispatch. Independent review
  is clean, including extra callback/owner-drop and nil-ordering probes. Manager checks
  passed: proof/fallback tests, real benchmark discovery, six selected emission goldens,
  the new fixture on GCC/Clang and real Go, and GCC/Clang ASan/UBSan. Combined-tree proof
  checks and three direct/borrowed-receiver fixtures passed on both compilers and real Go
  with no selected-case skips; docsite checked 224 pages with no broken link.
  Audited seven-run no-LTO O2 kernel medians were GCC 40.904 to 32.836 ms and Clang 49.239
  to 33.120 ms, with all outputs matching Go. This is dispatch/inlining microbenchmark
  evidence, not a whole-project, release/LTO, Windows or full-suite acceptance claim.
  The manager did not repeat performance timings. Bounds-check work remains 15.6b.

- 2026-10-20: 15.7a integrated from reviewed source `a67c4e69`. Existing `CXX`/`GOCPP_CXX`
  command prefixes already support compiler-cache launchers, so no `GOCPP_CCACHE` knob or
  production path was added. Tests cover forwarding, quoting, literal metacharacters,
  response files, precedence, C-helper selection, target flags and pre-start cancellation.
  Windows host fallback documentation was corrected during review. Manager focused tests
  and integrated documentation checks passed (222 pages, no broken link). These fake-launcher
  tests do not prove real cache hits or speedups: ccache/sccache were unavailable, no tools
  were installed, and underlying-compiler identity limits remain documented. Overall 15.7
  still needs real-cache measurements, generic-body work and modules evaluation.

- 2026-10-20: 15.2 integrated from reviewed source `b68eb6ac`, including all earlier fixes.
  Unknown/native calls and generic instances remain conservative. Added stack objects are
  limited to 2 KiB each and a shared 16 KiB function budget including local-closure captures
  and holder/alignment overhead; this is not a bound on the whole C++ frame or recursion.
  Borrowed element analysis accounts for string-containing local arrays and mutation aliases.
  Shadow-dependent initializers retain heap ownership instead of moving a non-movable holder.
  Whole-net independent review is clean. Manager checks passed: analysis/budget/CLI tests,
  four runtime fixtures with GCC/Clang ASan and four real-Go comparisons, escape golden,
  and combined checks after the LSP/probing integrations, including macro/escape goldens.
  No selected runtime comparison skipped. Historical benchmarks were not remeasured;
  no full corpus, sanitizer sweep, new Windows/arm64 run or v1.0 acceptance is claimed.

- 2026-10-20: Step 15.3 integrated after clean whole-net review at `e91d0ced`, including
  current native-fmt integration and test reliability. Ownership fixes cover promoted/named
  receivers, typed nil adapters, implicit foreign side effects, ordered argument/receiver
  snapshots, intra-argument checks, defer/go ownership, library boundaries and runner status.
  Retained native fmt callbacks remain conservative thread sources, not purity exemptions.
  Nineteen hot-load files and one cross-package module are synchronized finite positives;
  separate startup and copy/lend controls prevent locks from hiding analysis regressions.
  Manager focused checks passed: build/vet; analysis/classification, ABI and runner tests;
  collection-type probes on GCC/Clang; 40-check callback tests per compiler; eleven changed
  goldens; eight selected run cases and seven Go comparisons; two foreign fixtures normally
  and under ASan on both compilers; nineteen safe runtime cases and nineteen Go comparisons;
  the cross-package module on both compilers and Go; representative native race checks.
  The corrected real runner also passed one fixed pointer/value seed (eight cases each),
  two Go references and four GCC/Clang ASan executions. This is not a large campaign.
  Earlier benchmark/campaign narratives are historical, not final-tree performance proof.
  No full CI, broad sweep, new Windows/arm64 run or final full benchmark was performed;
  v1.0 exit criteria remain open for the user's manual final review.

- 2026-10-13 (audit): `const&` parameters and `this` receivers are only safe for caller locals
  that nothing else can change (reproduced a null `n` and an ASan use-after-free otherwise).
  Size raised to 10–12 weeks: the streams are independent, but one person does them one after
  another. Fuzzing samples C++ compiles; Go's `test/` comes from a pinned source download
  (missing from NixOS's GOROOT); the Nix package wraps compilers; macOS x86-64 added to CI.
- 2026-10-13 (your decisions): renumbered from Phase 14 to 15, because the new Phase 14 ports to
  Windows. macOS is no longer a target; the platform stream is Linux arm64 + packaging; size now
  9–11 weeks. A goroutine scheduler is not on the post-1.0 list (`go` stays an OS thread).
- 2026-10-14 (from the 11.1 review): BSD-3 clause 2 says a binary distribution must carry the copyright
  notice. Release binaries and their docs must include a THIRD_PARTY notice with Go's license (the
  parser fork in internal/syntax, internal/format, internal/diff, the stdlib stubs) and gopls's (the
  copied LSP protocol in internal/lsp/protocol), e.g. printed by `gocpp version -licenses` and shipped
  next to the binary.
