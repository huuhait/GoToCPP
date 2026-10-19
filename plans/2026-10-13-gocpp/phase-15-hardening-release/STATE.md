# Phase 15: Optimization, Hardening & Release · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

### Performance
- [x] 15.1 Benchmark suite (gocpp vs Go vs C++) + per-commit tracking on a fixed machine + 5 % regression gate
- [ ] 15.2 Escape analysis (unknown code = escaping) + `-gp:m`
- [ ] 15.3 Receiver and parameter optimizations
  - [ ] non-escaping pointer methods use `this`; strong reference at the call site unless the receiver is an unchangeable caller local
  - [ ] `const&` only for caller-local, never-address-taken, uncaptured arguments
  - [ ] targeted random generators (callee clears the source, closure changes it, method drops its receiver) under ASan
- [ ] 15.4 Intrusive refcount experiment (keep only if it wins)
- [x] 15.5 Swiss-table maps
- [ ] 15.6 Devirtualization + bounds-check help
- [ ] 15.7 Compile-time work (`extern template`, ccache/sccache, C++20 modules evaluation)

### Robustness
- [x] 15.8 Fuzzing (parse → check → lower → emit; sampled C++ compiles)
- [x] 15.9 Random-program differential testing vs `go run` (batched programs, no implementation-defined cases)
- [x] 15.10 Port Go `test/` run-tests as a conformance suite (downloads the pinned Go source)
- [ ] 15.11 Nightly ASan/UBSan/TSan/LSan (fail on any report) + leak audit

### Platforms & packaging
- [ ] 15.12 Linux arm64 CI; Windows (Phase 14) in the release pipeline
- [ ] 15.13 `go install`, GoReleaser binaries (linux amd64/arm64, windows amd64), Nix flake package with wrapped compilers
- [x] 15.14 `gocpp get` + `gocpp.sum` + minimal version selection (`x/mod` workarounds for dotless paths and `gocpp.mod` boundaries)

### Docs & release
- [ ] 15.15 Docs site (tour, spec, interop guide + cookbook, Go-dev and C++-dev guides, stdlib reference)
- [ ] 15.16 v1.0 compatibility promise + changelog + release
- [ ] 15.17 Post-1.0 roadmap ADRs

## Exit criteria (v1.0)

- [ ] Benchmarks published; goals (geomean ≤ Go, nothing > 2× slower) evaluated; no unexplained regression > 5 %
- [ ] ≥ 90 % of applicable Go `test/` run-tests pass
- [ ] 24 h fuzzing clean; ≥ 1M random differential programs, all mismatches triaged; targeted generators ASan-clean
- [ ] CI green on Linux x86-64, Linux arm64, Windows x86-64 (Wine + real-Windows checklist)
- [ ] Installable via `go install`, release binary, `nix run` (also on NixOS); docs site live

## Decisions / notes

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
