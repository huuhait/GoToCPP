# ADR-044: The user's compiler flags, LLVM tools, and files for other tools

Status: accepted (2026-10-18), builds on [ADR-036](036-action-graph-and-up-to-date-check.md)

## Context

Steps 6.11 and 6.12. The user wants both ways of using LLVM tools: plugins loaded into the C++ build, and the
program's LLVM IR for `opt`. Phase 04 made the flags of `//gocpp:` lines go through an allow-list that refuses
`-fplugin`. Editors need a compilation database, and people who cannot use gocpp in their build need a CMake file.

## Decision

1. **The user's flags are layered into the trusted flags of the build** (`build.EnvFlags.Layered`): the mode's
   (`-race`, `-release`), then `GOCPP_CFLAGS`/`GOCPP_CXXFLAGS`/`GOCPP_LDFLAGS`, then `-cflags`/`-cxxflags`/`-ldflags`.
   Everything that already read the environment's flags (every compile, the runtime's, the probes) gets them with no
   change in its code. In a compile the trusted flags come before the package's, as `GOCPP_CFLAGS` always did
   (interop spec), and gocpp's language flags last; at the link they come after the package's.
2. **Plugins are only loaded by these flags.** The allow-list stays as it is. A plugin file's content hash is added to
   the object cache key (`PluginSalt`) and to the runtime's key; its size and time are in the up-to-date key.
   Relative plugin paths are made absolute (the compiles run in a work folder). The header probes drop plugin flags
   (`DropPluginFlags`): they run in the editor, count as compiles to a plugin, and have their own cache.
3. **`-emit-llvm` writes IR with a second compiler run** of the object's compile with `-c` turned into `-S -emit-llvm`
   (or `-c -emit-llvm`), after the object compile. Clang only. The runtime has no IR. The output folder
   `<program>.llvm` is emptied only if it carries the marker `.gocpp-llvm`. A second run costs compile time but
   keeps the object cache and the IR independent, and works with a plugin that changes the IR.
4. **The plan.** A build can record every compile and the link it would run (`build.Plan`), from the same functions
   that build the argument lists (not from a copy of them), also under `-n` where nothing runs. The compilation
   database and the CMake export are made from the plan. `export` runs the build as `-n`: the generated files are
   written, no compiler compiles.
5. **`compile_commands.json` is opt-in** (`-compdb`, `gocpp export compdb`), not written by every build: the plan's
   README text said "after each build", but a build that writes into the module folder unasked is a surprise, and a
   file next to `gocpp.mod` is something to commit or ignore by choice. (Open question for the manager.)
6. **`gocpp export cmake`** copies the generated files and the runtime into the folder and refers to the project's own
   files by absolute path; it checks the compiler family at configure time. Limits are in docs/spec/build.md.
7. **`gocpp list` and `gocpp env`** follow `go list` and `go env`. `env` has a table of variables that a test keeps
   in step with the Go code.

## Consequences

- The cache is correct across a rebuilt plugin; a plugin that depends on files other than itself is not tracked.
- `-race`, `-release`, `-debug` are flags of build and run (the plan listed them); `-buildmode` is step 6.10's.
- `-n` shows the plain compile commands (no precompiled header), which is what a tool wants to run.
