# ADR-003: The output is readable C++20 with `#line` directives

Status: accepted (2026-10-13)

## Context

gocpp never writes machine code. It writes C++ and lets `clang++` or `g++` optimize it and make
the binary, as Nim, V and cppfront do. The C++ can look like code a person would write, or like
compiler output (SSA form, every value in a temporary). Users must debug their programs, read
compiler errors and link with C/C++ code.

## Decision

- The output is **readable C++20**: Go names are kept, one Go statement gives a small, readable
  C++ shape, and user variables get explicit C++ types (`int64_t x = e;`). Only compiler
  temporaries (`gp_t0`) and opaque C/C++ values use `auto`.
- Supported compilers: **GCC ≥ 13 and Clang ≥ 17**. Every test runs with both.
- Flags: `-std=c++20` (not `gnu++20`, whose predefined macros `linux` and `unix` could collide
  with gocpp names), `-fwrapv`, `-fno-strict-aliasing` and `-ffp-contract=off`
  ([ADR-006](006-numbers-and-checks.md)); `-Qunused-arguments` for clang only.
- A `#line N "/abs/path/file.gp"` directive comes before every generated line whose `.gp` line is
  not "previous + 1". Paths are absolute and escaped as C strings (Windows backslashes).
  `gocpp emit file.gp` prints the C++.

```go
func add(a, b int) (int, bool) {
	return a + b, a+b > 10
}
```

```cpp
#line 6 "/home/me/hello/hello.gp"
std::tuple<int64_t, bool> add(int64_t a, int64_t b) {
#line 7 "/home/me/hello/hello.gp"
    return {a + b, a + b > 10};
}
```

## Alternatives

- **Unreadable SSA-style C++** (rejected): hard to debug, and C++ errors would point at code
  nobody can read.
- **C** (rejected): loses templates (generics, [ADR-017](017-generics-templates.md)), RAII
  (reference counting, [ADR-004](004-memory-arc.md)) and direct use of C++ libraries.

## Consequences

Good: gdb and lldb work from day one (`break hello.gp:12`, `print s`); C++ errors point at the
`.gp` line (`#line` has no column, so Phase 13 adds a small column map); the generated code can
go into bug reports.

Bad:
- Every place where Go and C++ disagree needs an explicit rule: evaluation order, small-integer
  promotion, `v := v` shadowing, `goto` over initializations. Differential tests check each one.
- C++20, not C++23: `<stacktrace>` is not available. Panic traces use `backtrace()` or
  libbacktrace on Linux, and `CaptureStackBackTrace` + DWARF line lookup on Windows.
- C++ compile time is a real cost ([ADR-008](008-build-driver.md) caches it).

## Evidence

- Spike 2026-10-13: the key C++ idioms gave identical output on GCC 15 and Clang 21
  ([cpp-idioms](../../spikes/2026-10-13/cpp-idioms/)).
- Audit: g++ and clang++ evaluate call arguments in different orders (`fmt.Println(next(),
  next())` prints `2 1` with g++), so gocpp orders calls itself and tests both compilers.
- Audit: without repeated `#line`, a breakpoint on a `.gp` line stopped before its variable was
  set (three C++ lines were mapped to three different `.gp` lines).

See: [Phase 02 §2.1](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#21-example), [§2.2](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#22-backend-structure-internalcppgen), [§2.5](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#25-evaluation-order-minimal-version-here-already), [§2.7](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#27-the-driver), [§2.8](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#28-debugging-works-from-day-1), [Phase 03 §2.9](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#29-runtime-checks-and-panics-gos-exact-messages), [Phase 13 §2.1](../../plans/2026-10-13-gocpp/phase-13-tooling/README.md#21-diagnostics-that-read-like-go), [§2.2](../../plans/2026-10-13-gocpp/phase-13-tooling/README.md#22-debugging), [Phase 14 §2.1](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#21-runtime-the-windows-os-layer).
