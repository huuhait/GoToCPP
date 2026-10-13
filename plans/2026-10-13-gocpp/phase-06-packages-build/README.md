# Phase 06: Packages, Modules & Build System

> Grow from "one folder" to real projects: many packages, a `gocpp.mod` module file, correct
> cross-package initialization, a fast cached and parallel build, libraries as output, and
> files that let C++ tools (clangd, CMake) understand gocpp projects.

| | |
|---|---|
| Depends on | Phase 03, Phase 04 (C/C++ flags per package), Phase 05 (type descriptors live in headers; cross-package interfaces and embedding) |
| Unlocks | Phases 07, 09, 10, 12, and the Windows port (Phase 14; its first builds start here) |
| Milestone | **M4: multi-package projects build fast and incrementally** |
| Size | medium-large (about 4–5 weeks, including the first Windows builds) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

The Phase 01 loader already type-checks several packages, but codegen still writes one big
file. This phase gives every package its own C++ header and translation units, and builds a
`go build`-like driver around them: content hashing, a build cache, parallel compilation, and
correct link order. The guiding rule is **"a no-op build should take milliseconds, and a
one-file change should recompile one file"**, without ever producing a stale binary.

## 2. How it works

### 2.1 The module file: `gocpp.mod`

```
module github.com/huuhait/game

gocpp 0.1                    // minimum gocpp version
cxx c++20                    // C++ standard for generated + user C++ files

require github.com/huuhait/mathx v0.3.0
replace github.com/huuhait/mathx => ../mathx

// module-wide C/C++ settings (packages add their own with //gocpp: directives)
cflags -DGAME_DEBUG
pkg-config sdl2
```

Same syntax as `go.mod`. Parse it with `golang.org/x/mod/modfile`: `ParseLax` accepts the
unknown gocpp lines and fills `module`/`require`, and the raw syntax tree (`f.Syntax`) still
contains every line. Lax mode skips `replace`, so our code reads `replace` plus the gocpp lines
(`cxx`, `cflags`, `pkg-config`, …) from that tree (the audit confirmed this behavior). If it
gets awkward, a small parser of our own (about 200 lines) is fine. In v1, `require` resolves
through `replace` (local path) or a `vendor/` folder. Downloading modules is Phase 15.

### 2.2 One package → its own C++ files

```
$XDG_CACHE_HOME/gocpp/gen/<main module>/gpkg/<import path>/     (a stable folder per package)
├── pkg.hpp            everything other packages need: types, prototypes, extern vars, templates
├── render.gp.cpp      one translation unit per .gp file (parallel + incremental)
├── physics.gp.cpp
└── pkg_init.cpp       package variables, init() functions, gp_var_init()/gp_init_funcs()
```

The folder is keyed by the import path, not by a content hash, so `-I` paths never change
between builds. Content hashes are only used inside the cache (2.5). One `-I` points at
`gen/<main module>`, so every package header is included as `#include "gpkg/<import path>/pkg.hpp"`
(for example `"gpkg/fmt/pkg.hpp"`). The `gpkg/` prefix keeps these names apart from real C++
headers such as `<fmt/format.h>`.

- **Namespaces are flat**: `gpkg_` + an injective escape of the import path, for example
  `gpkg_main`, `gpkg_fmt`, `gpkg_github_dcom__huuhait__game__engine`. Letters and digits stay;
  every other character becomes a short code that starts with `_`: `/` → `__`, `.` → `_d`, `-` → `_h`,
  `_` → `_u`, `~` → `_t`, `+` → `_p`, and any other byte → `_x` plus two hex digits. Because
  every `_` in the result starts such a code, the name can be decoded back, so two import paths
  can never give the same namespace. (An earlier version mapped `.` → `_`, which let `x.u` and
  `x_` collide; found by two docs jobs.) C++ reserves names containing `__`; GCC and Clang
  accept them without a warning (checked with `-Wall -Wextra -pedantic`), so the readable `__`
  for `/` stays. Nested namespaces would cause two problems the audit hit: inside
  `gpkg::main`, the C function `time()` would find the package namespace `gpkg::time` first; and
  package `game` declaring `engine` would clash with the sub-package namespace
  `game::engine`.
- **Visibility:** C++ can't enforce Go's capitalized-means-exported rule, so `go/types`
  enforces it and the header contains all declarations. That's simple and always correct.

### 2.3 Header layout (C++ needs declarations before use; Go doesn't)

```
pkg.hpp
 1. #pragma once, #include <gocpp/rt.hpp>
 2. #include the headers of imported gocpp packages
 3. #include this package's C/C++ headers, but only those its declarations need
    (then #undef the macros that collide with gocpp names, Phase 04 §2.3)
 4. forward declarations of every named type
 5. type definitions in by-value dependency order (structs, newtypes, interface structs),
    with fully qualified type names inside struct bodies (`::gpkg_main::Base Base;`)
 6. function prototypes, extern package variables, exported constants as `inline constexpr`
    (methods are declared inside their structs)
 7. inline / template definitions: interface wrapper methods, forwarding members,
    generic bodies (Phase 07)
 8. type descriptors (inline variables, Phase 05)
```

- gocpp headers are included **before** C/C++ headers, so C macros can't break gocpp
  declarations. A package's own C headers only go into `pkg.hpp` when its exported declarations
  use C types; otherwise they stay in the `.cpp` files. Flags that change how those headers parse
  (`-I`, `-D`) become **public flags** of the package: they are passed to every dependent and
  are part of its cache key, so no two translation units see a C struct with different layouts.
- Anonymous struct types used in several packages share one canonical definition in a generated
  `gpkg_anon.hpp` with one include guard per type (Phase 03).
- Cycles between types are fine inside one header. Go forbids import cycles, so headers never
  include each other circularly.

### 2.4 Initialization across packages (Go 1.21+ rule)

Since Go 1.21 the spec defines one global order: sort all packages by import path, then
repeatedly initialize the **first** package in that list whose imports are all initialized. A
recursive "initialize my imports first" scheme gives a different order. With `main` importing
`b` and `c`, and `b` importing `z`, Go 1.25 prints `init c`, `init z`, `init b`, while the
recursive scheme gives `z, b, c` (found in the audit).

So the **driver** computes the global order at link time and emits one function:

```cpp
void gp_init_all() {                      // generated by the driver for the final program
    ::gpkg_c::gp_var_init();      ::gpkg_c::gp_init_funcs();
    ::gpkg_z::gp_var_init();      ::gpkg_z::gp_init_funcs();
    ::gpkg_b::gp_var_init();      ::gpkg_b::gp_init_funcs();
    ::gpkg_main::gp_var_init();   ::gpkg_main::gp_init_funcs();
}
```

Inside a package, variables follow `types.Info.InitOrder` and `init()` functions follow source
order. Package variables are declared **`constinit`** (zero-initialized at compile time) and
assigned in `gp_var_init()`, so C++ runs no dynamic initializers of its own. That needs every
runtime type to have a `constexpr` default constructor, `gp::func` included. Without it, a
`std::function` global gets C++ dynamic initialization, which can run *after* gocpp's init in a
shared library and wipe the value (shown in the audit). Variables of C++ types that can't be
constant-initialized use `gp::lazy<T>` storage, constructed inside `gp_var_init()`.

### 2.5 Build graph and cache

```
 transpile(pkg) ──► compile(TU) ×N ──► link
       ▲                 ▲
   inputs hashed:    inputs hashed:
   .gp sources,      TU text + every header it really included (from the compiler's -MD
   gocpp version,    depfile) + flags (incl. public flags of deps) + compiler identity
   API hash of deps, (path + version) + the compiler wrapper's environment (on NixOS,
   macro list,       NIX_CFLAGS_COMPILE* changes include paths)
   directives
```

- **Content-addressed cache** in `$XDG_CACHE_HOME/gocpp/` (`obj/<hash>.o`, generated sources).
  `gocpp clean -cache` empties it.
- Generated files are only rewritten when their bytes change, so editors and external tools
  don't see fake changes.
- **API hash = hash of the package's Go export data + its `pkg.hpp`.** Constants from other
  packages are folded into the dependent's code, so a changed `const Max = 10` → `20` must
  rebuild dependents even though no C++ declaration changed. Hashing the export data catches
  that, and exported constants also appear in `pkg.hpp` as `inline constexpr` (C++ users of
  `cpp-lib` need them anyway). Plain declarations in `pkg.hpp` carry no `#line`, so editing a
  function body doesn't change the header. Generic bodies do carry `#line`; editing above a
  generic function rebuilds its dependents, which is correct and rare.
- Each built package also stores a small **summary** next to its export data: its macro
  declarations (signatures and which parameters are `lazy`) and the names of methods that
  `#Derive` will generate. Phase 10's expansion of *other* packages' macros reads these summaries,
  and they are part of the API hash.
- Parallel compilation with `-p N` (default: number of CPUs), Go's flag name.
- **Runtime** is compiled once per (compiler, flags) into the cache. `gocpp/rt.hpp` gets a
  **precompiled header**, since every TU includes it. This is the biggest compile-time win.
- `-unity` option: one TU per package (faster clean builds, slower incremental builds).

### 2.6 Linking

`$CXX` is the link driver (it knows libstdc++/libc++). The order is objects of dependents before
dependencies, then the package's C/C++ objects, then libraries from directives and `gocpp.mod`
(deduplicated, keeping static-library order valid), then the runtime, then system libs as
needed (`-lpthread`, `-lm`, `-ldl`). Release mode adds `-O2 -flto`. LTO archives need the
archiver that matches the compiler (`gcc-ar` or `llvm-ar` from the compiler's own directory):
plain `ar` fails with "plugin needed to handle lto object" (seen on this machine).

### 2.7 Build modes (`-buildmode`, like Go)

| Mode | Output | Use |
|---|---|---|
| `exe` (default) | executable | apps |
| `c-archive` | `libx.a` + generated C header | C programs call functions marked `//export Name`, emitted as `extern "C"` |
| `c-shared` | `libx.so` / `x.dll` + C header | same, as a shared library |
| `cpp-lib` (new) | `libx.a` + the generated `pkg.hpp` headers + CMake target | C++ projects use the native C++ API (`gp::string`, structs, methods) |

- **`//export` types:** cgo exports `string` (as `GoString`), `int` (as `GoInt`), slices (as
  `GoSlice`) and multiple results (as a struct); it refuses only Go struct and array types. gocpp
  follows the same rules, copying strings and slices at the boundary. Every export goes through a
  Phase 04 panic trampoline, so a panic never unwinds into the C caller.
- **Initialization** for `c-archive` and `c-shared` happens **lazily, at the first exported
  call** (thread-safe, once), the way Go does it, not in a load-time constructor whose order
  against C++ static initialization is unreliable.
- **`c-shared` hides everything** (`-fvisibility=hidden`) except the `//export` functions.
  Otherwise GCC turns inline-variable descriptors into `GNU_UNIQUE` symbols that merge across two
  gocpp libraries in one process and block `dlclose` (seen in the audit).
- C consumers of `c-archive` must link the C++ runtime (`-lstdc++` or `-lc++`); the generated
  pkg-config/CMake info says so.
- `cpp-lib` ships a CMake `INTERFACE` target that carries gocpp's required flags (`-fwrapv`,
  `-fno-strict-aliasing`, `-std=c++20`), because consumers compile gocpp's inline and template
  code themselves.

### 2.8 Files for other tools

- **`compile_commands.json`** at the module root after each build. clangd then understands the
  generated C++ *and* the user's own `.c/.cpp` files (completion, go-to-definition in C++).
- **`gocpp export cmake`** writes a `CMakeLists.txt` that builds the generated sources +
  runtime, for dropping gocpp code into an existing CMake project.

### 2.9 CLI after this phase

```
gocpp build [-o out] [-p N] [-x] [-work] [-tags a,b] [-buildmode m]
            [-debug | -release] [-race] [-asan] [-nochecks] [packages]
gocpp run [build flags] [package | files] [args...]
gocpp list [-deps] [packages]       # like go list
gocpp env                           # compiler, versions, cache dir, flags
gocpp clean [-cache]
gocpp export cmake
```

`-debug` = `-O0 -g`; the default = `-O2 -g` (optimized and debuggable, like `go build`);
`-release` = `-O2 -flto`. `-race` = ThreadSanitizer, `-asan` = AddressSanitizer + UBSan (`-asan`
exists since Phase 02, for the sanitizer runs).

### 2.10 Cross-compilation and the first Windows builds

`-os <os> -arch <arch>` (or `-target <triple>`) passes `--target=` and a sysroot to the compiler
and sets GOOS/GOARCH for build constraints. Two targets matter in this phase:
- **windows/amd64 (required).** Linux and Windows are the v1 targets (your decision), so Windows
  test builds start here, long before the full port in Phase 14. The toolchain is **MinGW-w64**
  (with clang or gcc), which the NixOS dev shell provides as a cross compiler. `gocpp build -os
  windows` turns `hello.gp` into `hello.exe`, linked with `-static` so it runs without extra
  DLLs (with only `-static-libstdc++`, MinGW's thread library would still be a DLL). A CI job
  builds the run tests that need nothing OS-specific for Windows and runs them under **Wine**,
  so Windows breakage shows up early. The full Windows runtime and stdlib come in Phase 14.
  Wine rules for that job (all found while building the dev shell): its own `WINEPREFIX`,
  created once before the tests (`wineboot -i`), with `wineserver -p` kept running (a cold
  start costs about 6 s, and creating a prefix prints two lines on stderr that would break
  `// Stderr:` tests); `WINEDEBUG=-all`; and `WINEDLLOVERRIDES` containing
  `winemenubuilder.exe=d;mscoree,mshtml=`, which the harness sets itself (the dev shell also sets
  the first part, but `make ci` may run outside it), because otherwise Wine writes desktop
  entries, MIME files and icons into the user's real home and asks to install Mono and Gecko.
  The prefix is created with `DISPLAY` and `WAYLAND_DISPLAY` unset and `winewayland.drv=d` in
  `WINEDLLOVERRIDES`: while it creates the prefix, Wine starts `explorer.exe /desktop`, and its
  Wayland driver finds the desktop's default socket (`$XDG_RUNTIME_DIR/wayland-0`) even without
  `WAYLAND_DISPLAY`, so on GNOME or KDE it would open windows on the real desktop (found by
  docs job 0.6b). GUI smoke runs then use `Graphics=null` (Phase 14 §2.6). The
  dev shell's MinGW-w64 targets the older `msvcrt.dll` C runtime (the UCRT variant isn't in the
  Nix binary cache and would be built from source), while users' toolchains (llvm-mingw, MSYS2
  UCRT64) target UCRT; the real-Windows runs of Phase 14 cover UCRT.
- **linux/arm64 (best effort)**; Phase 15 makes it a CI target.

## 3. Steps

1. **6.1 `gocpp.mod`** parsing (`x/mod/modfile` lax mode + reading `replace` and the gocpp lines
   from the syntax tree), module root discovery, `replace` and `vendor/` resolution.
2. **6.2 Per-package codegen**: `pkg.hpp` + one `.gp.cpp` per file + `pkg_init.cpp`; flat,
   injectively escaped namespaces; stable per-package folders.
3. **6.3 Header layout** (§2.3): include order, public flags, fully qualified types in struct
   bodies, exported constants, shared anonymous-type header.
4. **6.4 Cross-package init**: global Go 1.21+ order computed by the driver, `gp_init_all()`,
   `constinit` variables, `gp::lazy<T>`.
5. **6.5 Action graph** (transpile → compile → link) with content hashing, `-MD` header deps,
   macro lists, public flags and wrapper environment in the keys.
6. **6.6 Cache** (objects, generated files, write-if-changed, API hash = export data + `pkg.hpp`,
   package summaries, `clean -cache`).
7. **6.7 Parallel executor** (`-p`), `-x`, `-work`, good error aggregation.
8. **6.8 Runtime prebuild + precompiled header.**
9. **6.9 Link step** (ordering, dedup, system libs, LTO with the matching archiver).
10. **6.10 Build modes**: `c-archive`, `c-shared` (`//export` with cgo's type rules, lazy init,
    hidden visibility, trampolines), `cpp-lib` (CMake INTERFACE flags).
11. **6.11 `compile_commands.json` + `gocpp export cmake`.**
12. **6.12 CLI**: `list`, `env`, `clean`, all build flags, build tags.
13. **6.13 Cross-compilation**: windows/amd64 with MinGW-w64 (`.exe` linked with `-static`) and a
    CI job that runs the OS-independent run tests under Wine; linux/arm64 best effort.
14. **6.14 Tests**: multi-package fixtures (diamond imports, cross-package embedding/interfaces),
    the 3-package init-order case against Go, incremental-build assertions (what was recompiled,
    including a changed exported constant), build-mode consumers (a C program uses the c-archive;
    a CMake project uses the cpp-lib; two c-shared libraries load in one process and unload).

## 4. Deliverables

- Module-aware loader, per-package codegen, cached parallel build driver
- Build modes `exe`, `c-archive`, `c-shared`, `cpp-lib`
- `compile_commands.json`, CMake export
- Target program `multi-pkg` passing

## 5. Exit criteria

- `multi-pkg` (app + 3 packages + C sources) builds and runs on both compilers.
- No-op rebuild of `multi-pkg` takes < 200 ms. Changing a function body in a leaf package
  recompiles exactly one TU and relinks; changing an exported constant rebuilds its dependents.
- Init order matches Go 1.25 in a differential test with prints in `init()` across packages.
- A C program links the c-archive and calls an exported function; a CMake project uses the
  cpp-lib; two c-shared libraries load with `dlopen` in one process and unload with `dlclose`.
- clangd opens a generated file without errors (manual check).
- `gocpp build -os windows` produces a working `hello.exe`, and the OS-independent run tests pass
  under Wine.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Big headers make C++ compiles slow | PCH for the runtime, API-hash short-circuit, `-unity`; C++20 modules evaluated in Phase 15 |
| Stale or incompatible cached objects | Keys include compiler path + version, all flags, public flags of deps, real header deps, macro lists and the wrapper environment; API hash includes export data |
| Static-library link order | Topological order + dedup tests; fall back to `--start-group` on GNU ld |
| Tool-specific behavior (GNU ld vs lld vs the MinGW linker, `ar` vs `gcc-ar`) | CI matrix; archiver taken from the compiler's directory; the Windows build is tested from this phase on |
