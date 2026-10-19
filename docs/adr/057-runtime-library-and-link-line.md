# ADR-057: The runtime is one cached static library, and the link line has one order

Status: accepted (2026-10-18). Refines [ADR-009](009-embedded-runtime.md) (the embedded runtime) and
[ADR-008](008-build-driver.md) (the build driver); Phase 06 steps 6.8 and 6.9.

## Context

Until now the runtime's sources (dozens of files) were compiled once for a (compiler, flags, sanitizer, target) key
and kept as as many object files in the cache (`obj/<key>/`). Every link named all of them, every
stamp and trace listed them, and the link line was a concatenation made in `Link`: objects, the
runtime's objects, flags of the packages with only `-l`, `-L` and `-pthread` deduplicated one by one.
Step 6.9 asks for a fixed order, deduplication, system libraries after the objects and LTO with an
archiver that can read the objects.

Facts found on the way:

- **A plain `ar` and LTO objects do not mix.** GNU ar without its plugin prints "plugin needed to
  handle lto object", exits with status 0 and writes a library without a symbol index; the link then
  fails with an undefined reference. This was reproduced here with g++ 15 and clang++ 21. A library of LTO
  objects needs `gcc-ar`, `llvm-ar`, or `ar --plugin <the compiler's plugin>`.
- **A static library keeps only the objects that something needs.** An object whose only job is a
  global constructor would be dropped. The runtime has none (its state is constant-initialized or lazy; a
  test reads the objects for static initializers), and the programs' initialization is called by
  `gp_init_all`.
- **The precompiled header was already there** (since the object cache of Phase 05): clang
  `-include-pch`, g++ `rt.hpp.gch`, keyed by the compile flags, with the list of the files it was made
  from. What it lacked was a check that those files are still the same: clang refuses a stale header,
  g++ does not look at the text and would go on with the old one.

## Decision

1. **The runtime is one static library per key**, `<runtime folder>/lib/<key>/libgocpprt.a`, built
   once and shared by every program with the same key. The key is the hash of the compiler (path,
   arguments, family, version, identity of the program file), the flags (sanitizer, `-nochecks`,
   `-flto`, flags of the environment), the sources, the environment that the compiler reads
   (`CPATH`, `NIX_CFLAGS_COMPILE*`, ...) and the identity of the archiver. A change of a runtime source
   is a new runtime folder (its name is a hash of the embedded files), so a stale library is never found.
2. **Building is safe**: one process builds a key at a time (an advisory `flock` on `lib/<key>.lock`;
   the others wait and use its library; a lock that can't be had, or ten minutes of waiting, make a
   process build it itself: the result is renamed into place, so that is only duplicate work). The
   compiles and the archive are made in a temporary folder and the library is renamed to its name,
   so a killed build leaves no library; a library is checked before use (a regular file with the
   archive magic) and a damaged one is rebuilt.
3. **The archiver belongs to the compiler** (`FindArchiver`): without LTO the `ar` next to the
   compiler (with the target prefix of a cross compiler); with LTO and g++ `gcc-ar` (or `ar` with
   `liblto_plugin.so`); with LTO and clang++ `llvm-ar` (or `ar` with `LLVMgold.so`, found by
   `-print-file-name` or in the `-plugin` argument of the link command that `clang -###` prints:
   a Nix wrapper adds it only there). `GOCPP_AR` replaces the search. When none is found, a build
   without LTO links the object files as before, and one with LTO stops with an error that says what
   was looked for and how to fix it. `GOCPP_NORTLIB=1` turns the library off. LTO objects are those of
   `-lto` and also those of a user's `-flto` (GOCPP_CXXFLAGS, the compiler's arguments, a release
   build): both get the LTO archiver, whoever asked. A `GOCPP_AR` that names nothing usable is an
   error, also without LTO. `-lto` with `-buildmode=c-archive` or `cpp-lib` is refused: the archive
   would hold LTO objects that only the same compiler with `-flto` can read (with a user's `-flto`
   those modes use the LTO archiver too, and the consumer must link with LTO). The stamp key has `-lto`.
4. **LTO is opt in**: `gocpp build -lto`. g++ gets `-flto=auto`, clang++ `-flto=thin`; the flags go to the
   compiles of the runtime and of the generated C++ and to the link, and are part of every key (also
   the precompiled header's). The C and C++ files of the packages and the libraries of modules compile
   as before; the linker takes ordinary objects next to LTO ones.
5. **One function makes the link line** (`linkArgs`): driver flags (debug flags, sanitizer, LTO),
   `-Wl,--gc-sections` (not on macOS), the objects in the order of the packages, then the objects of
   the packages' own C and C++ files, then the module libraries' objects, each file once; the runtime
   library; the link flags of the packages (dependents first), deduplicated by
   `dedupLinkFlags`; then `GOCPP_LDFLAGS` as given; then `-o`. A library comes after what uses
   it; `-lname` keeps its last place, `-L` its first, an idempotent flag (`-pthread`, `-framework X`)
   its first, a library under another linker state (`-Bstatic`, `--as-needed`, `--push-state`) is
   another library, and a line with `--start-group` or `--whole-archive` keeps its libraries as
   they are. A command longer than the limit (28000 bytes for Windows, 120000 elsewhere) puts the
   files and flags in a response file `link.rsp` of the work folder (the driver flags and `-o` stay
   on the command line).
6. **The precompiled header stays** and is checked: once for each build, the files it was made from are
   compared with the list that was saved next to it (stat, then hash); a header that was edited in place
   is removed and rebuilt, and the compile goes on without it.

Note for the order of merges: `linkArgs` is the single source of the link command. Anything that
records or prints the link plan (6.11's `recordLinkPlan`, `compile_commands`, CMake export) must take its
words from `linkArgs`, not rebuild the line from the parts.

## Alternatives

- **Keep the object files.** Works; costs a longer link line, a longer stamp, and no place to say
  "this is one input". The library is not faster to link (measured: see the table in
  [docs/spec/build-link.md](../spec/build-link.md)); its reasons are the key, the lock and the one input.
- **A shared runtime (`.so`).** Rejected: a program must be one file that runs anywhere (ADR-009), and
  `std::` templates in the runtime's headers are instantiated per program.
- **Accept `ar` for LTO objects with `-ffat-lto-objects`.** Doubles the compile time of the runtime
  and gives no LTO of the runtime.
- **A precompiled header per program.** The runtime's headers are the same for every program, so one
  header per key is enough (and the generated units' own includes are not part of it).

## Consequences

- A runtime change, a new compiler or another sanitizer builds a library once, whatever the programs.
- `gocpp build -lto` needs `gcc-ar`/`llvm-ar` (they come with the compilers); the error says so when they
  are missing.
- The link line is the same for the same program (a golden test); `-x` shows it with `$WORK`.

## Evidence

- Tests: `internal/build/{rtlib,rtlib_real,archive,linkplan,linkcmd}_test.go` (fake compiler and
  archiver that record their command lines, real g++ and clang++ for the library, sanitizers, LTO and
  the header's invalidation; mutation checks listed in the step's report).
- Measurements: [docs/spec/build-link.md](../spec/build-link.md).

## See

- [Phase 06 §6.8, §6.9](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md)
- [ADR-024](024-cxx-recipes.md), [ADR-025](025-split-layout.md) (the object cache and its keys)
