# ADR-105: Generic type instances get out-of-line bodies, and C++20 modules wait for a measurement

Status: proposed (2026-10-20). Post-1.0 direction. Written against `dev` while step 15.7 (`extern template`, ccache, "evaluate C++20 modules") is still open; if 15.7 does option 1 below, what remains of this ADR is options 3 and 4. Builds on [ADR-030](030-generic-instances.md).

## Context

A generic function or type is lowered once per instance into ordinary C++ ([ADR-030](030-generic-instances.md)), so the C++ compiler does no template deduction for gocpp generics. The measurements of step 7.12 ([generics-compile-time.md](../guides/generics-compile-time.md)), g++ 15.3 and clang++ 21.1 on a loaded 192-core machine (+/- 15%):

- **Generic functions cost what the same code written by hand costs.** 2,000 instances: 17.4 s CPU with g++ and 8.4 s with clang++ at -O0, against 17.5 and 8.3 for the hand-written twin.
- **Generic types cost 30 to 55% more** at -O0 (20 types x 10 instances, three methods each: 28.2 s against 18.2 s with g++, 14.8 against 10.4 with clang++). The reason is the header: the text of every instance is in `pkg.hpp` (978 KB for the 200 instances, 42% of it method bodies, 31% repeated long `#line` paths), and every translation unit of the package parses it, including `pkg_desc.cpp` and `pkg_init.cpp`, which hold almost nothing else. Parsing the header alone takes 9.7 s CPU with g++ and 3.8 s with clang++; the two header-only units are 14 of 41 s (g++) and 7.5 of 19 s (clang++) (sums of unit CPU in the 7.12 review run, not the 28.2 s and 14.8 s above).
- **`extern template` does not apply**: instances are ordinary functions and structs, not templates.
- The runtime header is already precompiled (a PCH of `rt.hpp` for each flag set), which is why the floor for `hello` is 0.7 s with g++ and 0.5 s with clang++. Optimized builds are far more expensive than -O0 (2 to 3 times with g++, 5 to 10 times with clang++; `-release` with ThinLTO of 100 functions x 5 types takes 58 s with clang++ against 10 s with g++).
- Instances of the standard generics (`slices.Sort[int]`) are written in the unit that asks for them, so two packages that use the same instance both compile it. How much that costs in a real program has not been measured.
- Nothing in the repository measures C++20 modules or header units.

## Options

1. **Out-of-line bodies for generic type instances**: emit the method bodies once, as ordinary (non-`inline`) definitions in `pkg_desc.cpp`, declare them in `pkg.hpp`; bodies of at most three statements stay inline so that -O2 without LTO still inlines accessors; drop a `#line` directive that repeats the one before. The 7.12 review estimated the T rows going from +33 to 39% to +5 to 10%. This estimate has not been built or measured. Risk: lost inlining across units of bigger methods at -O2 without LTO.
2. **A precompiled `pkg.hpp`** for packages with many generic type instances, chained on the runtime's PCH. Removes the repeated parse but not the compile of the bodies; needs a second PCH in the build driver and a cache rule for it.
3. **One compiled unit per shared instance**, with a new unit-independent name and ownership rule (today an instance lives in the asking package's namespace and its name is a hash of the unit and the type arguments, chosen so that two units' copies get different external names, [ADR-030](030-generic-instances.md), [ADR-025](025-split-layout.md)), so that two packages (or two projects, through the object cache) compile `slices.Sort[int]` once. The benefit depends on how often programs share instances; unmeasured.
4. **C++20 modules or header units** for `rt.hpp` and `pkg.hpp`. Unknowns: how g++ 15 and clang++ 21 compare on the 20 x 10 program and on the runtime; the build driver needs a dependency scan (P1689) and module files that are specific to the compiler and its flags, which meets the object cache keys ([ADR-008](008-build-driver.md), `objcache.go`) and the Nix-wrapped compilers ([ADR-089](089-packaging-go-install-goreleaser-nix.md)); MinGW g++ on Windows is untested. The runtime PCH already gives 0.5 s for `hello`, so the gain for `rt.hpp` is small; the gain for `pkg.hpp` is the open question.
5. **Nothing** (keep the guide's advice: few files per package, debug builds while developing).

## Decision (proposed)

After 15.7: do option 1 with the `#line` change and re-measure the same T rows with both compilers; keep option 2 as the follow-up if the header-only units are still a large share. Do not adopt option 4 without a prototype that builds the 20 x 10 program and the runtime with modules on g++ and clang++ and shows a gain over the PCH; do option 3 only if a profile of a real multi-package program shows duplicate instances. Accept that -O2 and `-release` cost stays dominated by the C++ optimizer.

## Trigger

- 15.7's modules evaluation result (if it ran a prototype, this ADR is accepted or dropped on its numbers).
- A package with 100 or more generic type instances reported as slow to build, or the T rows still above +15% after option 1.
- A gcc or clang release that makes modules usable with the PCH-class gains in the prototype, on both compilers.

## Consequences

- Good: the cost of generic types falls to the cost of hand-written ones without a change to the language or to how instances are named; no new build-system concept.
- Bad: bodies in another unit are not inlined across units without LTO, so some -O2 code gets slower unless the three-statement rule is tuned; debugging a method of an instance moves from the header to `pkg_desc.cpp` (the `#line` directives keep the `.gp` position).
- Uncertain: every figure for options 1 to 4 except the baseline; for option 3 also the naming and ownership rule, which contradicts ADR-030's per-unit names and is a design of its own.

## Evidence

[generics-compile-time.md](../guides/generics-compile-time.md) (the tables, "A bug that the nesting numbers found", "The concrete next step (not done)"), [ADR-030](030-generic-instances.md), the 7.11/7.12 low in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md).

## See

[Phase 15 §2.1](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first) (compile-time row), step 15.7, [Phase 06 risks](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md) ("C++20 modules evaluated in Phase 15").
