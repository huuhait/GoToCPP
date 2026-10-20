# ADR-103: Reference cycles stay counted: the common shapes get compiler fixes, and a cycle report comes before any collector

Status: proposed (2026-10-20). Post-1.0 direction ([Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen), "an optional tracing GC for cycle-heavy programs"). Would extend [ADR-004](004-memory-arc.md) and not supersede it.

## Context

Memory is reference counting ([ADR-004](004-memory-arc.md), divergence [D1](../spec/language.md#d1-reference-counting-instead-of-a-garbage-collector)): a cycle is never freed, and `weak.Pointer` breaks it. Everything below is what the repository shows today; there is no field data, because gocpp has no users outside the project yet.

- **The leaks that the tests make on purpose** are listed in `tools/sanitize/leaks.txt` ([ADR-090](090-nightly-sanitizers-and-a-bounded-leak-policy.md)): 18 programs, 72 to 35,704 bytes each. By kind: 7 recursive closures (a `var f func(...)` that the closure calls), 4 `sync.Cond` whose `Locker` points into the struct that holds the Cond (`sync.NewCond(&q.mu)`), 3 structs that point at each other, and one each of doubly linked list, self pointer, cyclic data and a context tree. At least 11 of the 18 are ordinary Go idioms, not mistakes.
- **The ports pay for it.** `regexp` and `regexp/syntax` carry patch rules that break cycles ([ADR-083](083-sort-regexp-ports-and-the-test-matcher.md)), `strings.Builder` keeps its own address as a `uintptr` ([ADR-073](073-strings-bytes-io-bufio-context-ports.md)), and the base64 and hex decoders were patched after LeakSanitizer found 2 KB lost per decoder (step 9.11, ADR-102).
- **The tool that warns is local.** `gocpp vet cycle` looks inside one function; "a cycle through several functions is not found" ([vet guide](../guides/vet.md)).
- **Counting is not slow where it matters.** In the last (noisy: load 76 to 153) full run, `binarytrees` at -O2 with g++ took 1176 ms against Go's 1269 ms and the C++ twin's 440 ms; `tree_alloc` is 1.76x Go ([runtime.md](../benchmarks/runtime.md)). Escape analysis and the intrusive count (15.2 to 15.4) were still open when this was written, so these numbers will move.
- **A collector would meet three hard facts.** Goroutines are OS threads with no safepoints ([ADR-007](007-goroutines-os-threads.md)) and the counts are atomic, so a trial-deletion pass must stop or tolerate other threads (D10). Only types that flow into an interface have descriptors with fields (`gp_full`, [ADR-016](016-interfaces-itabs.md), [ADR-028](028-fmt-on-descriptors.md)); closures, `shared_ptr` control blocks and C++ objects cannot list their children. And [ADR-004](004-memory-arc.md) rejected Boehm for being nondeterministic and awkward with C++ objects.

## Options

1. **Stay as is** (documentation, `weak`, `vet cycle`, the leak ceilings of ADR-090).
2. **Fix the common shapes at compile time.** A self-recursive closure whose variable does not escape (needs the escape analysis of 15.2) can call itself through a non-owning reference; a closure that does escape cannot, because nothing would keep the box alive. `sync.Cond` could hold its `Locker` without owning it when the pointer points into the object that holds the Cond (a runtime or stdlib patch; unproven, see Uncertain).
3. **A cycle report, not a collector.** A debug or exit-time pass (trial deletion over the descriptors, run on request or by `-asan`/leak-check builds) that prints the cycles it finds with their allocation types, and frees nothing. It can be wrong in only one direction (a missed cycle), so it cannot corrupt a program.
4. **A real cycle collector** (Bacon-Rajan trial deletion, or tracing). Needs a trace function for every type including closures, a way to stop the other threads (a signal-based stop using the registry of worker threads of [ADR-077](077-deadlock-detection.md), which exists), and a rule for C++ objects (not traced, treated as roots).
5. **Arenas or regions** (an explicit region whose whole graph is freed together, in the spirit of Go's shelved `arena` experiment). Helps only programs that opt in, and a pointer that escapes the region needs a check.
6. **A tracing-GC build mode** (`gp::ptr` becomes a collected pointer). A second runtime; the backlog item "optional tracing-GC mode for cycle-heavy programs".

## Decision (proposed)

Do options 1, 2 and 3 after 1.0, in this order: the escape-analysis-based closure fix (when 15.2 has landed), the Cond fix, then the cycle report (also useful to find the cycles in the standard library ports that the leak list does not cover). Do not start 4, 5 or 6 until a trigger below fires. Leaking cycles are not part of the v1 compatibility promise ([ADR-110](110-the-go-language-version-moves-once-a-year-and-the-v1-promise-names-what-it-covers.md)): a valid program may use less memory in a later release.

## Trigger

- A report of unbounded memory growth in a long-running program that options 2 and 3 do not explain (this decides between 4, 5 and 6).
- `tools/sanitize/leaks.txt` grows by a line that is not a test written to leak (the policy of ADR-090 already forces a review for that).
- 15.2 escape analysis lands (starts option 2).

## Consequences

- Good: no new runtime mode, no stop-the-world, determinism and C++ interop stay; the leak ceilings keep shrinking instead of growing.
- Bad: a program with a real cycle-heavy data structure (graphs, observers, parent pointers) still needs `weak`, as today. The report finds cycles after the fact, it does not prevent them.
- Uncertain: the `sync.Cond` mechanism. At `NewCond(&q.mu)` time the runtime cannot know which object holds the Cond, and a non-owning `Locker` would break `NewCond(&sync.Mutex{})`; it may need a compiler rule instead of a runtime one. Also how many real programs hit the shapes of option 2. The count of 11 of 18 comes from the project's own tests, which were written to show cycles.

## Evidence

[tools/sanitize/leaks.txt](../../tools/sanitize/leaks.txt), [ADR-090](090-nightly-sanitizers-and-a-bounded-leak-policy.md), [vet guide](../guides/vet.md), [runtime.md](../benchmarks/runtime.md), [D1](../spec/language.md#d1-reference-counting-instead-of-a-garbage-collector), [D10](../spec/language.md#d10-data-races-can-break-reference-counts), the 8.7 low "`sync.Cond` with `L = &q.mu` in the same struct is an ARC cycle" ([TODO.md](../../plans/2026-10-13-gocpp/TODO.md)).

## See

[Phase 15 §2.1](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first) (escape analysis, intrusive count), [Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen).
