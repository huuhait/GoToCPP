# ADR-108: Typed interop becomes the default when three measurements pass, and the clang policy is a tested matrix

Status: proposed (2026-10-20); overtaken by [ADR-131](131-typed-interop-is-the-default-mode.md) (2026-10-21), where the user made `auto` the default now without waiting for the three measurements (the clang matrix and the Windows corpus are still wanted). Post-1.0 direction. Carries out the switch that ADR-101 (step 12.11, "Opaque is the default for now") prepared as one constant; builds on [ADR-060](060-typed-interop-probes.md), [ADR-087](087-typed-interop-code-generation.md), [ADR-088](088-typed-interop-caches-every-compiler-answer-per-reference-under-the-headers-contents.md) and the contract in [typed-interop.md](../spec/typed-interop.md) §13 (added by step 12.11). The ADR-101 text is not linked because that ADR is on its step branch at the time of writing.

## Context

Typed interop (Phase 12) asks clang++ what a C++ expression is and uses the answer for early errors, for the values of C constants and for promoted members. It is chosen by `-interop=typed|opaque|auto` or `GOCPP_INTEROP`; `auto` is typed when clang++ is installed and opaque otherwise; **the default is opaque** (`DefaultMode = Opaque`). From ADR-101:

- 294 programs that import headers (`tests/interop`, `zoo`, `run`, `pkgs`, `examples`) build and print the same in typed mode with g++ and clang++; three differ, all of them tests of errors that opaque mode rejects. Warm overhead 0 to 35 ms.
- **Not shown yet:** cold probing under the 0.5 s of the Phase 12 exit criterion (measured 0.9 to 2.4 s for the library-heavy examples on a lightly loaded machine); clang 17 to 20 (only clang 21 is in the dev shell; step 12.12); a Windows host (`auto` stays opaque there because the probes lack the MinGW-w64 target; the typed probe on a Windows target cannot parse `windows.h` with the dev shell clang, 14.4 low in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md)).
- Some language features exist only in typed mode: the value of a C macro in an array length (`[cstdio.BUFSIZ]byte`, gap E1, [ADR-097](097-typed-interop-puts-the-values-of-c-constants-into-the-program-before-go-types.md)) and the members promoted from an embedded C++ class (gap E2, [ADR-098](098-promoted-members-of-embedded-cpp-classes-are-decided-by-the-compiler-in-a-second-pass.md)); in opaque mode they are gates ("not supported yet (Phase 12)", see [interop.md](../spec/interop.md)).
- The Phase 12 compatibility rule: typed mode never rejects what opaque mode plus the C++ compiler accepts; with g++ as the build compiler its errors are warnings ([ADR-079](079-typed-interop-checker-reports-only-what-the-compiler-reports.md)).
- clang is run as a process, not linked: the release binaries are static `CGO_ENABLED=0` builds ([ADR-089](089-packaging-go-install-goreleaser-nix.md)), so libclang is not an option without changing packaging. The documented minimum is clang 17 ([ADR-003](003-readable-cpp20-output.md): GCC 13, Clang 17); the JSON dump format is clang's own and not stable across majors (Phase 12 risk row: "tolerant reader; CI against several clang versions").
- The pinned llvm-mingw that gocpp downloads for Windows ([ADR-055](055-cross-compilation-targets-and-toolchain.md), Q15) contains a clang, so a Windows host has a clang with the right target and headers without asking the user for one. Whether its version and its JSON dump work with the probes is not tested.
- C++20 modules: no document in the repository asks for module imports. The probes are text (one file of `decltype` lines per package, ADR-060), keyed by the contents of the headers (ADR-088); header units would be compiler-version- and flag-specific files that the cache key would have to include.

## Options

1. **`auto` is the default at 1.0.** Users get early errors and E1/E2 with no flag. Risk: the first user with clang 18 or 19 meets a JSON difference, and the silent fallback to opaque hides it; cold probing costs 1 to 2 s on the first build of a library-heavy program.
2. **`auto` becomes the default in a minor release after three measurements pass** (below). Opaque and `-interop=opaque` stay forever.
3. **Typed stays opt-in.** Simple; E1/E2 stay behind a flag and the early errors of Phase 12 are seen by few.
4. **Typed only** (drop opaque). Rejected: it makes clang a hard requirement of every program that imports a header and breaks the compatibility rule.
5. **Use C++ header units to cut the cold cost.** Only if a profile shows header parsing is most of the 0.9 to 2.4 s (not known; the cost may be the dump and the process starts).

## Decision (proposed)

Option 2. Flip `DefaultMode` to `Auto` in the first minor release after all three hold, each recorded in a dated note:

1. **A clang matrix:** the nightly job (step 15.11 machinery) builds and runs the interop corpus in typed mode with the clang majors that the policy names. Policy: the minimum is clang 17; the matrix is the minimum, every major in between that a supported distribution ships, and the newest; a new clang major is added within one gocpp minor release of its appearance; the reader tolerates unknown fields and a failing probe falls back to opaque with a visible `-v` line and a hint, never to a wrong answer.
2. **Cold probing** under 0.5 s on a prepared cache for the example set, or the criterion is changed to the measured number with a reason (a number that users see in the first build, and a way to prime the cache, for example probing in the background during `gocpp get`).
3. **A Windows host:** probes with `--target=x86_64-w64-mingw32` and the MinGW include paths of the toolchain in use (including the pinned llvm-mingw's own clang), with the interop corpus green on the real Windows PC.

Header units and C++20 modules as an import mechanism are not proposed.

## Trigger

- The three measurements above; the first is the one most likely to find work (12.12 was meant to test clang 17 to 20).
- A clang release that changes the JSON dump (the matrix finds it the next night).
- A user library that ships only a module interface, which would reopen module imports.

## Consequences

- Good: the C++ errors of the common mistakes arrive before the C++ compile, at the `.gp` line; E1/E2 work with no flag; the flip is one constant plus wording (ADR-101 lists the places).
- Bad: gocpp needs clang++ installed for the full experience, and the matrix has a standing cost (nightly time, and a fix whenever a clang major moves a field). On a machine without clang the program still builds, in opaque mode.
- Uncertain: the cold cost on a quiet machine; the real set of clang majors users have.

## Evidence

[typed-interop.md](../spec/typed-interop.md) §13 (added by step 12.11), [Phase 12 README](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md) (exit criteria, risks, the 12.11 note), 14.4 and 12.5 lows in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md), ADR-101 (294 programs, 0.9 to 2.4 s).

## See

[Phase 12 §2.8](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md), [interop.md](../spec/interop.md).
