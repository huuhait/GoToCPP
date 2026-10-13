# ADR-009: The runtime is a C++20 library embedded in the gocpp binary

Status: accepted (2026-10-13)

## Context

Generated code needs a runtime: `gp::string`, `gp::slice`, `gp::map`, `gp::ptr`, interfaces,
panics, defer frames, channels and goroutines. Generated code depends on the runtime's exact
API. If the runtime were installed separately, its version could differ from the compiler's.

## Decision

- The runtime is a C++20 library, mostly headers, in `namespace gp`
  (`runtime/include/gocpp/*.hpp`, `runtime/src/*.cpp`).
- It is **embedded in the `gocpp` binary** with `go:embed` and extracted once per version to
  `$XDG_CACHE_HOME/gocpp/rt/<version>/`. A tiny Go package inside `runtime/`
  (`runtime/files.go`, package `rtfiles`) holds the `//go:embed` directive, and the driver
  imports it. The standard library sources (`stdlib/`) are embedded the same way, through a tiny
  package inside `stdlib/`.
- The API shape is fixed in Phase 02; later phases only add to it.
- Runtime types have no plain-named members: only operators and `gp_`-prefixed members, and
  free functions such as `gp::len(s)`. So a user's method called `get` or `len` never clashes
  with the runtime.
- The runtime calls the operating system only through `runtime/src/os/`, which also holds the
  Phase 08 thread layer `gp::os` ([ADR-021](021-platforms-linux-windows.md)).

## Alternatives

- **A separate install step** (rejected): version-skew bugs between compiler and runtime.

## Consequences

Good:
- One binary to install: `go install github.com/huuhait/gocpp/cmd/gocpp@latest`, a release
  binary or the Nix package.
- Compiler and runtime always match. `gocpp version` prints the embedded runtime's hash.

Bad:
- A C++20 compiler must still be installed separately; `gocpp env` shows the one it found.
- The first build compiles the runtime once per (compiler, flags) into the cache, plus a
  precompiled header for `gocpp/rt.hpp` ([ADR-008](008-build-driver.md)).
- There is no runtime ABI promise between gocpp versions: generated code must be regenerated
  with the matching gocpp, and `cpp-lib` headers are versioned.

## Evidence

- Found by the first Phase 00 jobs: a `//go:embed` pattern can't reach a parent folder (it may
  not contain `..`), so the driver can't embed `../../runtime` itself. Hence the tiny packages
  inside `runtime/` and `stdlib/`.

See: [root README §4](../../plans/2026-10-13-gocpp/README.md#4-how-it-works-architecture), [Phase 01 §2.6](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#26-package-loading-and-the-importer), [Phase 02 §2.6](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#26-runtime-v0-runtimeincludegocpprthpp), [§2.7](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#27-the-driver), [Phase 03 §2.1](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#21-one-ownership-system-for-everything-v1), [Phase 06 §2.5](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#25-build-graph-and-cache), [Phase 08 §1](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#1-idea), [Phase 14 §2.1](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#21-runtime-the-windows-os-layer), [Phase 15 §2.4](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#24-packaging-and-install), [§2.7](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#27-v10-compatibility-promise).
