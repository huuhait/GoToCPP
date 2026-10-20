# ADR-105: Generic type instances get out-of-line bodies, and C++20 modules wait for a measurement

Status: bounded option 1 implemented in 15.7b after independent review and focused manager checks (2026-10-20); options 2–4 remain proposals. Step 15.7 remains open: this is not a modules evaluation or external-cache performance result. Builds on [ADR-030](030-generic-instances.md).

## Context

A generic function or type is lowered once per instance into ordinary C++ ([ADR-030](030-generic-instances.md)), so the C++ compiler does no template deduction for gocpp generics. The measurements of step 7.12 ([generics-compile-time.md](../guides/generics-compile-time.md)), g++ 15.3 and clang++ 21.1 on a loaded 192-core machine (+/- 15%):

- **Generic functions cost what the same code written by hand costs.** 2,000 instances: 17.4 s CPU with g++ and 8.4 s with clang++ at -O0, against 17.5 and 8.3 for the hand-written twin.
- **Generic types cost 30 to 55% more** at -O0 (20 types x 10 instances, three methods each: 28.2 s against 18.2 s with g++, 14.8 against 10.4 with clang++). The reason is the header: the text of every instance is in `pkg.hpp` (978 KB for the 200 instances, 42% of it method bodies, 31% repeated long `#line` paths), and every translation unit of the package parses it, including `pkg_desc.cpp` and `pkg_init.cpp`, which hold almost nothing else. Parsing the header alone takes 9.7 s CPU with g++ and 3.8 s with clang++; the two header-only units are 14 of 41 s (g++) and 7.5 of 19 s (clang++) (sums of unit CPU in the 7.12 review run, not the 28.2 s and 14.8 s above).
- **`extern template` does not apply**: instances are ordinary functions and structs, not templates.
- The runtime header is already precompiled (a PCH of `rt.hpp` for each flag set), which is why the floor for `hello` is 0.7 s with g++ and 0.5 s with clang++. Optimized builds are far more expensive than -O0 (2 to 3 times with g++, 5 to 10 times with clang++; `-release` with ThinLTO of 100 functions x 5 types takes 58 s with clang++ against 10 s with g++).
- Instances of the standard generics (`slices.Sort[int]`) are written in the unit that asks for them, so two packages that use the same instance both compile it. How much that costs in a real program has not been measured.
- Nothing in the repository measures C++20 modules or header units.

## Options

1. **Out-of-line bodies for generic type instances**: emit larger method bodies once, as ordinary (non-`inline`) definitions in an existing `pkg_desc.cpp`, retaining member declarations and small inline methods in `pkg.hpp`. The bounded 15.7b implementation counts at most three lowered statements, including temporaries, receiver copies, control statements, loop initializers and nested bodies; closures and calls spelled `append`, `make` or `new` go out of line. Tiny scalar loops remain inline after an optimized check exposed a regression when all control flow was outlined. The first using package in import order emits the body in its original declaring namespace after shared dependencies are known: the declaring package need not know an importer's type arguments. Type identity, names, descriptors, method adapters and semantic ownership do not change. Repeated `#line` directives are left unchanged. The 7.12 review's expected +5–10% cost over hand-written code was not achieved; the measured whole-build gain remains inconclusive (below). Risk: lost inlining across units of bigger methods at -O2 without LTO.
2. **A precompiled `pkg.hpp`** for packages with many generic type instances, chained on the runtime's PCH. Removes the repeated parse but not the compile of the bodies; needs a second PCH in the build driver and a cache rule for it.
3. **One compiled unit per shared instance**, with a new unit-independent name and ownership rule (today an instance lives in the asking package's namespace and its name is a hash of the unit and the type arguments, chosen so that two units' copies get different external names, [ADR-030](030-generic-instances.md), [ADR-025](025-split-layout.md)), so that two packages (or two projects, through the object cache) compile `slices.Sort[int]` once. The benefit depends on how often programs share instances; unmeasured.
4. **C++20 modules or header units** for `rt.hpp` and `pkg.hpp`. Unknowns: how g++ 15 and clang++ 21 compare on the 20 x 10 program and on the runtime; the build driver needs a dependency scan (P1689) and module files that are specific to the compiler and its flags, which meets the object cache keys ([ADR-008](008-build-driver.md), `objcache.go`) and the Nix-wrapped compilers ([ADR-089](089-packaging-go-install-goreleaser-nix.md)); MinGW g++ on Windows is untested. The runtime PCH already gives 0.5 s for `hello`, so the gain for `rt.hpp` is small; the gain for `pkg.hpp` is the open question.
5. **Nothing** (keep the guide's advice: few files per package, debug builds while developing).

## Decision and remaining proposals

15.7b implements bounded option 1 without deduplicating `#line`: relocation preserves every method's original source positions. Keep option 2 as a possible follow-up if repeated header parsing remains a large share. Do not adopt option 4 without a prototype that builds the 20 x 10 program and the runtime with modules on g++ and clang++ and shows a gain over the PCH; do option 3 only if a profile of a real multi-package program shows duplicate instances. Neither proposal is implemented here. The non-LTO runtime trade-off still needs consideration; compile-time savings alone do not establish runtime neutrality.

### 15.7b measurement

Against the current `1c7081e9` baseline, two matched runs of the existing T 20 x 10 workload and its handwritten twin used g++ 15.3.0 and clang++ 21.1.8, `-O0 -g`, four compile jobs, object/up-to-date caches off, offline isolated caches, and an explicitly warmed runtime/PCH. The shared performance lock serialized runs. Baseline/candidate order was reversed for the second pair; all runs are reported in [the guide](../guides/generics-compile-time.md#bounded-body-placement-157b).

- The header shrank about 14%: 1,052,956 → 903,338 bytes with g++, 1,053,756 → 904,138 with clang++. 200 `Add` bodies moved; 400 accessors stayed inline.
- Mean generic build CPU decreased about 5% with g++ and 3% with clang++, but the byte-identical handwritten control varied by about +1.5% and -4%. The whole-build improvement is **inconclusive**, not evidence for the historical +5–10% generic overhead estimate. The candidate still costs roughly twice the handwritten twin.
- Generic `pkg_init.cpp` CPU fell from 5.811/5.794 to 5.203/5.168 seconds (g++), and 3.765/3.639 to 3.431/3.367 (clang++). The descriptor unit now compiles the moved bodies once; structs, descriptors, adapters and small methods still occupy the header.
- An optimized non-LTO check caught a roughly 79% clang++ regression when an early policy outlined a tiny arithmetic loop. Retaining small loop bodies under the recursive statement budget removed that regression: final loop medians were 35.612 → 35.650 ms with g++ and 19.787 → 19.787 ms with clang++, over five alternating pairs. Getter/setter checks also remained within the observed noise. This is not a runtime-neutrality claim for all larger methods.

## Trigger

- 15.7's modules evaluation result (if it ran a prototype, this ADR is accepted or dropped on its numbers).
- A package with 100 or more generic type instances reported as slow to build, or the T rows still above +15% after option 1.
- A gcc or clang release that makes modules usable with the PCH-class gains in the prototype, on both compilers.

## Consequences

- Good: less method text is parsed in every translation unit, without changing the language, instance names or build-system concepts. This is not equivalent to reaching hand-written compile cost.
- Bad: bodies in another unit are not inlined across units without LTO, so some -O2 code can get slower; the small-body heuristic does not prove otherwise. Debugging a moved method uses `pkg_desc.cpp`, with the original `.gp` source positions preserved by `#line`.
- Uncertain: a repeatable end-to-end benefit of option 1 beyond the measured sample's noise, and all benefits of options 2–4. Option 3 would additionally require a new naming and shared-ownership design, which is outside 15.7b.

## Evidence

[generics-compile-time.md](../guides/generics-compile-time.md) (the historical tables, "A bug that the nesting numbers found", and "Bounded body placement (15.7b)"), [ADR-030](030-generic-instances.md), the 7.11/7.12 low in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md).

## See

[Phase 15 §2.1](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first) (compile-time row), step 15.7, [Phase 06 risks](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md) ("C++20 modules evaluated in Phase 15").
