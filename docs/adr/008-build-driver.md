# ADR-008: gocpp has its own build driver with a content-hash cache

Status: accepted (2026-10-13)

## Context

After transpiling, the C++ must be compiled and linked, together with the package's own
`.c`/`.cpp` files and the libraries it uses. Users expect `go build`: one command, fast, and
never a stale binary. C++ tools (clangd, CMake projects) should understand gocpp projects too.

## Decision

- `gocpp build`, `run` and `test` call the C++ compiler themselves. The compiler is found in
  this order: `GOCPP_CXX`, `CXX`, `clang++`, `g++`; its version is checked. Flags are chosen per
  compiler family (`-Qunused-arguments` for clang only). The driver always passes an explicit
  `-O` level. A `-U_FORTIFY_SOURCE` flag doesn't help, so a tiny runtime header,
  `gocpp/nofortify.h`, undefines `_FORTIFY_SOURCE` when not optimizing
  (`#if !defined(__OPTIMIZE__)`). `gocpp/rt.hpp` includes it before anything else, and the
  user's own C/C++ files get it with `-include`.
- An action graph, transpile → compile (one job per translation unit) → link, runs in parallel
  (`-p N`). The cache in `$XDG_CACHE_HOME/gocpp/` is **content-addressed**:
  - a transpile key hashes the `.gp` sources, the gocpp version, the macro list, the directives
    and the API hash of each dependency (its export data + `pkg.hpp`);
  - a compile key hashes the TU text, every header it really included (from the compiler's `-MD`
    depfile), all flags (also the public flags of dependencies), the compiler's path and version,
    and the compiler wrapper's environment (NixOS `NIX_CFLAGS_COMPILE*`).
- The runtime is compiled once per (compiler, flags), with a precompiled header for
  `gocpp/rt.hpp`. `-unity` (one TU per package) is an option.
- The driver also compiles the package's `.c`/`.cpp` files and applies the `//gocpp:` directives
  and `gocpp.mod` flags (`pkg-config`, `-l`, `cflags`, `ldflags`).
- For other tools: `compile_commands.json` at the module root after each build, and
  `gocpp export cmake` writes a `CMakeLists.txt`.
- The goal: a no-op build takes milliseconds, and a one-file change recompiles one file.

## Alternatives

- **Generate CMake and require it** (rejected): slow, and an extra dependency.

## Consequences

Good:
- It feels like `go build`. Exit criterion: a no-op rebuild of `multi-pkg` takes < 200 ms, and a
  changed function body in a leaf package recompiles one TU.
- clangd understands the generated C++ and the user's own C/C++ files.
- gocpp code can still go into a CMake project (`gocpp export cmake`, the `cpp-lib` build mode).

Bad:
- We own the hard parts: cache keys (a missing input means a stale object, so the keys are wide),
  static-library link order, LTO archivers (`gcc-ar` or `llvm-ar` from the compiler's own
  folder), and linker differences (GNU ld, lld, the MinGW linker).

## Evidence

- Spike 2026-10-13: the NixOS clang wrapper prints `-Wunused-command-line-argument` warnings in
  `-fsyntax-only` mode, and g++ rejects `-Qunused-arguments`, so that flag is clang-only.
- Seen on this machine: plain `ar` fails on LTO objects ("plugin needed to handle lto object").
- Seen while building the dev shell: a Nix shell's compiler wrapper turns on "hardening" by
  default. It silently makes a missing `-O` into `-O2`, warns about `_FORTIFY_SOURCE` at every
  `-O0` file, and adds `-D_FORTIFY_SOURCE` again after the user's flags (3 with gcc, 2 with
  clang), so `-U_FORTIFY_SOURCE` has no effect (checked with both compilers).

See: [Phase 02 §2.7](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#27-the-driver), [Phase 04 §2.9](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#29-building-the-cc-side), [step 4.13](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#3-steps), [Phase 06 §1](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#1-idea), [§2.5](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#25-build-graph-and-cache), [§2.6](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#26-linking), [§2.8](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#28-files-for-other-tools), [§5](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#5-exit-criteria).
