# ADR-105: Generic type instances get out-of-line bodies, and C++20 modules wait for a measurement

Status: partly implemented. Bounded option 1 was implemented in 15.7b after independent review and focused manager checks (2026-10-20). Options 2–3 remain proposals; option 4 remains deferred after the bounded Linux header-unit experiment in 15.7c. Step 15.7 remains open; the experiment does not adopt modules or establish external-cache performance. Builds on [ADR-030](030-generic-instances.md).

## Context

A generic function or type is lowered once per instance into ordinary C++ ([ADR-030](030-generic-instances.md)), so the C++ compiler does no template deduction for gocpp generics. The measurements of step 7.12 ([generics-compile-time.md](../guides/generics-compile-time.md)), g++ 15.3 and clang++ 21.1 on a loaded 192-core machine (+/- 15%):

- **Generic functions cost what the same code written by hand costs.** 2,000 instances: 17.4 s CPU with g++ and 8.4 s with clang++ at -O0, against 17.5 and 8.3 for the hand-written twin.
- **Generic types cost 30 to 55% more** at -O0 (20 types x 10 instances, three methods each: 28.2 s against 18.2 s with g++, 14.8 against 10.4 with clang++). The reason is the header: the text of every instance is in `pkg.hpp` (978 KB for the 200 instances, 42% of it method bodies, 31% repeated long `#line` paths), and every translation unit of the package parses it, including `pkg_desc.cpp` and `pkg_init.cpp`, which hold almost nothing else. Parsing the header alone takes 9.7 s CPU with g++ and 3.8 s with clang++; the two header-only units are 14 of 41 s (g++) and 7.5 of 19 s (clang++) (sums of unit CPU in the 7.12 review run, not the 28.2 s and 14.8 s above).
- **`extern template` does not apply**: instances are ordinary functions and structs, not templates.
- The runtime header is already precompiled (a PCH of `rt.hpp` for each flag set), which is why the floor for `hello` is 0.7 s with g++ and 0.5 s with clang++. Optimized builds are far more expensive than -O0 (2 to 3 times with g++, 5 to 10 times with clang++; `-release` with ThinLTO of 100 functions x 5 types takes 58 s with clang++ against 10 s with g++).
- Instances of the standard generics (`slices.Sort[int]`) are written in the unit that asks for them, so two packages that use the same instance both compile it. How much that costs in a real program has not been measured.
- The original investigation did not measure C++20 modules or header units. The later bounded 15.7c experiment below measures header units against the real runtime PCH after 15.7b; its numbers must not be mixed with the historical figures above.

## Options

1. **Out-of-line bodies for generic type instances**: emit larger method bodies once, as ordinary (non-`inline`) definitions in an existing `pkg_desc.cpp`, retaining member declarations and small inline methods in `pkg.hpp`. The bounded 15.7b implementation counts at most three lowered statements, including temporaries, receiver copies, control statements, loop initializers and nested bodies; closures and calls spelled `append`, `make` or `new` go out of line. Tiny scalar loops remain inline after an optimized check exposed a regression when all control flow was outlined. The first using package in import order emits the body in its original declaring namespace after shared dependencies are known: the declaring package need not know an importer's type arguments. Type identity, names, descriptors, method adapters and semantic ownership do not change. Repeated `#line` directives are left unchanged. The 7.12 review's expected +5–10% cost over hand-written code was not achieved; the measured whole-build gain remains inconclusive (below). Risk: lost inlining across units of bigger methods at -O2 without LTO.
2. **A precompiled `pkg.hpp`** for packages with many generic type instances, chained on the runtime's PCH. Removes the repeated parse but not the compile of the bodies; needs a second PCH in the build driver and a cache rule for it.
3. **One compiled unit per shared instance**, with a new unit-independent name and ownership rule (today an instance lives in the asking package's namespace and its name is a hash of the unit and the type arguments, chosen so that two units' copies get different external names, [ADR-030](030-generic-instances.md), [ADR-025](025-split-layout.md)), so that two packages (or two projects, through the object cache) compile `slices.Sort[int]` once. The benefit depends on how often programs share instances; unmeasured.
4. **C++20 modules or header units** for `rt.hpp` and `pkg.hpp`. The 15.7c Linux prototype on g++ 15.3.0 and clang++ 21.1.8 builds both real headers as isolated header units; package-header reuse has a measurable warm benefit on T20x10, but its construction plus one C++ backend replay is slower in wall time than the existing runtime PCH (below). This does not settle the production dependency strategy (P1689 or another approach), compiler/flag-specific artifacts, object-cache keys and invalidation ([ADR-008](008-build-driver.md), `objcache.go`), or the Nix-wrapped compilers ([ADR-089](089-packaging-go-install-goreleaser-nix.md)). Macro-sensitive multi-package behavior and MinGW g++ on Windows remain untested. Named modules were not substituted for the global runtime declarations. Keep this option deferred, not adopted.
5. **Nothing** (keep the guide's advice: few files per package, debug builds while developing).

## Decision and remaining proposals

15.7b implements bounded option 1 without deduplicating `#line`: relocation preserves every method's original source positions. Keep option 2 as a possible follow-up if repeated header parsing remains a large share. The 15.7c prototype now demonstrates option 4's warm package-header benefit over PCH on both installed Linux compilers, but does not justify adoption without a production dependency/cache design and broader semantic/platform evidence. Do option 3 only if a profile of a real multi-package program shows duplicate instances. Options 2–4 are not implemented in the production build. The non-LTO runtime trade-off still needs consideration; compile-time savings alone do not establish runtime neutrality.

### 15.7b measurement

Against the current `1c7081e9` baseline, two matched runs of the existing T 20 x 10 workload and its handwritten twin used g++ 15.3.0 and clang++ 21.1.8, `-O0 -g`, four compile jobs, object/up-to-date caches off, offline isolated caches, and an explicitly warmed runtime/PCH. The shared performance lock serialized runs. Baseline/candidate order was reversed for the second pair; all runs are reported in [the guide](../guides/generics-compile-time.md#bounded-body-placement-157b).

- The header shrank about 14%: 1,052,956 → 903,338 bytes with g++, 1,053,756 → 904,138 with clang++. 200 `Add` bodies moved; 400 accessors stayed inline.
- Mean generic build CPU decreased about 5% with g++ and 3% with clang++, but the byte-identical handwritten control varied by about +1.5% and -4%. The whole-build improvement is **inconclusive**, not evidence for the historical +5–10% generic overhead estimate. The candidate still costs roughly twice the handwritten twin.
- Generic `pkg_init.cpp` CPU fell from 5.811/5.794 to 5.203/5.168 seconds (g++), and 3.765/3.639 to 3.431/3.367 (clang++). The descriptor unit now compiles the moved bodies once; structs, descriptors, adapters and small methods still occupy the header.
- An optimized non-LTO check caught a roughly 79% clang++ regression when an early policy outlined a tiny arithmetic loop. Retaining small loop bodies under the recursive statement budget removed that regression: final loop medians were 35.612 → 35.650 ms with g++ and 19.787 → 19.787 ms with clang++, over five alternating pairs. Getter/setter checks also remained within the observed noise. This is not a runtime-neutrality claim for all larger methods.

### 15.7c header-unit experiment

The [guide's full protocol, all sample pairs and reproduction commands](../guides/generics-compile-time.md#bounded-header-unit-experiment-157c) compare the actual runtime PCH, a runtime header unit and a complete package header unit at `a4316076`, with `-O0 -g` and all source/descriptor/init TUs. Two reversed-order rounds per configuration follow untimed feasibility checks. All 12 feasibility builds and 24 timed builds pass on the installed GCC 15.3.0 and Clang 21.1.8, including the ordinary runtime archive link and exact output (`600` for T20x10; builtin hello on stderr). Clang emits its experimental-header-unit warning; none of the tested configurations is silently excluded.

- T20x10 warm C++ backend CPU falls from 22.08/22.45 to 16.96/16.04 seconds with GCC and from 13.05/13.62 to 8.19/7.92 with Clang for **package** header units: about 26% and 40% in the pair means. Warm wall time falls about 19% and 26%. This includes compiling the descriptor TU's 15.7b out-of-line bodies; no units are dropped.
- Package BMI construction takes 5.86/6.15 seconds wall with GCC and 3.65/3.51 with Clang, versus 1.69/1.65 and 0.80/0.77 for the runtime PCH. Construction plus one replay is therefore slower in wall time (mean 15.764 vs 13.665 seconds GCC; 8.227 vs 7.061 Clang). These are artifact/replay costs, not cold production-build timings.
- Runtime-only header units do not materially improve T20x10 CPU (+1.4% GCC / -0.9% Clang), although the minimal hello's reuse CPU is lower and Clang saves about 0.14 seconds wall. The two GCC hello wall samples disagree on whether runtime HU wins.
- The frontend-only T20x10 importer falls from 2.28/2.21 to 0.45/0.46 CPU seconds with GCC and from 2.23/2.44 to 0.01/0.02 with Clang for package HU. This is not the whole-build result; it omits code generation/linking.

Keep option 4 deferred. The warm package benefit is real for this bounded input, not a project-wide speedup or comparison with an unimplemented package PCH. The ordinary object cache may eliminate work that this experiment deliberately forces; header changes also invalidate the package artifact. No scanner, production cache, ABI/naming change, Windows/Clang-17–20 result, or general runtime-neutrality claim follows. Options 2 and 3 remain separate proposals; ADR-105 is still only partly implemented.

## Trigger

- Revisit the deferred module proposal when a concrete dependency/cache strategy and broader macro-sensitive multi-package/platform evidence justify an adoption decision beyond the bounded 15.7c result.
- A package with 100 or more generic type instances reported as slow to build, or the T rows still above +15% after option 1.
- A gcc or clang release that makes modules usable with the PCH-class gains in the prototype, on both compilers.

## Consequences

- Good: less method text is parsed in every translation unit, without changing the language, instance names or build-system concepts. This is not equivalent to reaching hand-written compile cost.
- Bad: bodies in another unit are not inlined across units without LTO, so some -O2 code can get slower; the small-body heuristic does not prove otherwise. Debugging a moved method uses `pkg_desc.cpp`, with the original `.gp` source positions preserved by `#line`.
- Uncertain: a repeatable end-to-end benefit of option 1 beyond the measured sample's noise, all benefits of options 2–3, and the production benefit of option 4 beyond its measured warm single-package reuse. Option 3 would additionally require a new naming and shared-ownership design, which is outside 15.7b.

## Evidence

[generics-compile-time.md](../guides/generics-compile-time.md) (the historical tables, "A bug that the nesting numbers found", "Bounded body placement (15.7b)", and "Bounded header-unit experiment (15.7c)"), the frozen protocol and replay scripts in [tools/modules-experiment](../../tools/modules-experiment/), [ADR-030](030-generic-instances.md), the 7.11/7.12 low in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md).

## See

[Phase 15 §2.1](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first) (compile-time row), step 15.7, [Phase 06 risks](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md) ("C++20 modules evaluated in Phase 15").
