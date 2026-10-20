# gocpp specification

| Document | What it says |
|---|---|
| [language.md](language.md) | The language. gocpp is Go (go1.25) except for a short list of differences (D1–D21), plus three additions: header imports, `!` (must functions and must calls) and macros |
| [cpp-mapping.md](cpp-mapping.md) | What the generated C++ looks like: types, names, files, functions, methods, structs, multiple results and package initialization |
| [interop.md](interop.md) | Using C and C++ from gocpp: header packages and namespaces, `C`, name and type translation, strings and pointers at the boundary, callbacks, building C/C++ sources |
| [interop-cookbook.md](interop-cookbook.md) | Recipes for C and C++ in `.gp` programs (call C, std containers, exceptions, callbacks, sqlite, zlib, flags, helpers, modules, SDL2/raylib, wrapping a handle, memory rules, common errors); every program is a test |
| [typed-interop.md](typed-interop.md) | Typed interop (Phase 12, steps 12.1 to 12.11): the collector, the clang probes, declarations, the checker, the code generation, constants, promoted members, bindgen, the cache and the interop mode (`-interop`) |
| [modules.md](modules.md) | Modules: the `gocpp.mod` file, how the module of a folder is found, `require`, `replace` with local folders, `vendor/`, and how an import path becomes a folder |
| [build.md](build.md) | Building programs: the actions of a build and `-p`, how several errors are shown, `-x` and its counts, when a build is up to date and does nothing, what is kept in the cache folder, `gocpp clean`, package summaries and the API hash |
| [build-link.md](build-link.md) | The runtime as a cached static library, the precompiled header, the archiver, LTO (`-lto`) and the link command: order, deduplication, response files, with measurements |
| [feature-matrix.md](feature-matrix.md) | Which phase brings each feature and standard package. Generated from the compiler's gate table, so it always matches what `gocpp check` accepts |

The first three are **v0**: written in Phase 00 from the plan in
[plans/2026-10-13-gocpp](../../plans/2026-10-13-gocpp/README.md). They are living documents:
each phase updates them when it builds or changes a feature. Points the plan leaves open are
marked **open**, with a link to where the plan discusses them.

The decisions behind these documents are recorded as ADRs in [docs/adr/](../adr/) (Phase 00
step 0.2).
