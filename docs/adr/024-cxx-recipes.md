# ADR-024: C and C++ libraries are built from a `cxx` recipe in `gocpp.mod`

Status: accepted (2026-10-17)

## Context

A C or C++ library is a module (TODO Q14, Q16, Q17; Phase 06 §2.11): its headers are imported by their
full module path, and it is downloaded, replaced or vendored like a Go module. A header only declares.
Something has to build the library, with no CMake (ADR-008: gocpp has its own build driver) and no way for
downloaded code to run anything on the user's machine. The Phase 04 flag allow-list already keeps
`//gocpp:cflags` from running code (`-fplugin=`, `@file`, `-specs=`); a recipe must be at least as safe.

## Decision

`gocpp.mod` holds the recipe in a brace block `cxx <module path> { ... }`, one statement a line, with an
optional operating system condition (`linux:`, `!windows:`; replaced by `if` blocks in [ADR-129](129-a-recipe-condition-is-an-if-block-over-build-constraints.md)). The statements: `src`, `exclude`, `include`,
`system-include`, `define`, `private-define`, `cflags`, `cxxflags`, `ldflags`, `link`, `pkg-config`, `std`,
`cstd`. The full language, with examples, is in [docs/spec/modules.md](../spec/modules.md#c-and-c-libraries-cxx-recipes).

- **Who supplies it.** The project's `gocpp.mod` block for that module path; else the block the library ships
  in its own `gocpp.mod` for its own path; else none (header-only, as before). The project's block replaces
  the library's one entirely, with no merging. It is read only for a module that the program imports a header
  or package from.
- **What is public.** `include`, `system-include`, `define` and the compile flags of `pkg-config` are applied to the library's compiles **and** to the
  compiles and probes of the packages that use it: a macro that changes the layout of a header must be the same
  on both sides, and `<x.h>` lookups must work for the user. `private-define`, `cflags`, `cxxflags`, `std` and
  `cstd` are for the library's own compiles. The user's `//gocpp:cflags` are not given to the library, so one
  object serves every project. `-I` is used because a recipe is an explicit statement (the module folder itself
  stays `-iquote`); `system-include` is the `-isystem` choice.
- **Build.** Each source is compiled by gocpp's driver through the content-addressed object cache of Phase 06
  (key: source text, flags, compiler, module path and version, headers read), in parallel, with `-w` so that
  a library's warnings neither spam the user nor make the compile uncacheable; the objects are linked statically
  after the program's. A library that fails to compile is an error of the build (exit 1), with the module, the
  recipe line and the compiler's output.
- **A line gocpp does not understand is an error at its line.** `gocpp.mod` itself keeps a block opaque, so
  that other tools and newer gocpp versions share the file; the recipe reader is strict, since ignoring a
  flag would build the wrong library. A recipe written for a newer gocpp therefore fails with a clear message
  on an older one, instead of building something else.
- **Security.** Nothing from a module is executed. Every flag of a recipe, and the output of `pkg-config`,
  goes through the allow-list of Phase 04 (`checkCompilerFlags`, `checkLinkerFlags`, the refused flags), with
  a few link flags refused on top (`-Wl,-rpath`, `-Wl,-R`, `-Wl,--just-symbols`, `-Wl,-sectcreate`, `-Wl,-syslibroot`,
  `-Wl,-framework`, `-Wl,--push-state`, `-Wl,-l`, `-Wl,--library`, `-Wl,-L`, `-lto_library`, and a `-l` name with a `/` or `..` such as
  `-l:../../x`, which the linker would look up outside the module). A source is a regular file of at most 32 MiB. Every path is relative, with no `..`, no absolute name, no drive letter; the files and folders are resolved with their
  symbolic links, and one that leaves the module is an error, for every kind of module (it closes the
  question of links in downloaded modules for recipe paths). Patterns are bounded (10000 sources, 100000
  entries in a folder, 200000 entries for a library). `pkg-config` gets package names only (no `/`, not
  ending in `.pc`), so a `.pc` file of the module is never read.

## Alternatives

- **Parse the recipe in `gocppmod.Parse`** (rejected): a statement that a newer gocpp adds would then be an error
  for every tool that reads the file, and an unused recipe would make the whole project fail. The reader of the
  block is separate and runs only for the modules in use.
- **Merge the project's and the library's recipe** (rejected): the result would depend on both and be hard to read;
  copying the block is one edit.
- **Apply `cflags` to the users of the library too** (rejected): a flag with a path or a warning option has no
  business in someone else's compile; `define` and `include` are the explicit way to share.
- **Compile with the user's warnings** (rejected): third-party warnings are noise, and a compile that prints
  something is never cached.
- **Use CMake or run the library's build** (rejected here; an opt-in fallback is still open in Q14).

## Consequences

Good: a library is compiled once per (module, version, recipe, compiler, flags) and shared by every
project; a hostile recipe can choose sources and flags from the allow-list inside its own folder and nothing
else; the error for a recipe that does not fit comes at its line.

Bad:
- A library that needs the headers of another library needs the program to import it too (no `needs` yet).
- The build is the debug build (`-g -O0`) of the rest of the program until the build modes of Phase 06 choose
  optimization levels.
- A macro value cannot contain `-` or `@` (the Phase 04 allow-list).
- `include` is `-I`, so a file in the folder named like a standard header (`stdint.h`) hides the system's in every compile
  with the flag. `-isystem` and `-iquote` do not serve `<x.h>` lookups as well, and `-idirafter` is worse; gocpp hints
  at the cause when such a compile fails, and the fix is a more specific folder (`include include`, not `include .`).
- A shared index of recipes for common libraries, and the CMake fallback for big ones (OpenCV, Assimp),
  are not built (TODO Q14).
