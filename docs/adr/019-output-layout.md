# ADR-019: Output layout: one header per package, one `.cpp` per `.gp` file

Status: accepted (2026-10-13); refined by [ADR-025](025-split-layout.md) (descriptors are extern and defined in `pkg_desc.cpp`; the unnamed types are guarded definitions in every header)

## Context

Go doesn't care in which order things are declared; C++ does. Builds must be incremental: a
change in one file should recompile one file. And package initialization must follow Go's
order exactly, also across packages.

## Decision

Each package gets a stable folder `$XDG_CACHE_HOME/gocpp/gen/<main module>/gpkg/<import path>/`:
- `pkg.hpp`: everything other packages need (types, prototypes, extern variables, templates,
  type descriptors), in a fixed order;
- one translation unit per `.gp` file (`render.gp` → `render.gp.cpp`);
- `pkg_init.cpp`: package variables and `init()` functions, as `gp_var_init()` and
  `gp_init_funcs()`.

Other rules:
- Package headers are included as `#include "gpkg/<import path>/pkg.hpp"`. The `gpkg/` prefix
  keeps them apart from real C++ headers such as `<fmt/format.h>`.
- Each package has one namespace, `gpkg_<escaped import path>`; package `main` is `gpkg_main`
  ([ADR-010](010-cpp-naming.md)).
- gocpp headers come before C/C++ headers, so C macros can't break gocpp declarations.
- Inside a package, variables follow `types.Info.InitOrder` and `init()` functions follow source
  order. Across packages, the driver computes Go 1.21's global order and emits `gp_init_all()`.
- Package variables are `constinit` (zero at compile time) and set in `gp_var_init()`, so C++
  runs no dynamic initializers of its own. Variables of C++ types that can't be
  constant-initialized use `gp::lazy<T>` storage.

```cpp
void gp_init_all() {   // main imports b and c; b imports z
    ::gpkg_c::gp_var_init();     ::gpkg_c::gp_init_funcs();
    ::gpkg_z::gp_var_init();     ::gpkg_z::gp_init_funcs();
    ::gpkg_b::gp_var_init();     ::gpkg_b::gp_init_funcs();
    ::gpkg_main::gp_var_init();  ::gpkg_main::gp_init_funcs();
}
```

## Alternatives

- **One giant translation unit** (rejected): slow incremental builds. (The `-unity` option is a
  middle way: one TU per package, for faster clean builds.)
- **Header-only output** (rejected): every function body would be compiled again in every file
  that includes it, and any edit would change the header and rebuild all dependents.

## Consequences

Good: parallel, incremental builds. Plain declarations in `pkg.hpp` carry no `#line`, so editing
a function body doesn't change the header, and dependents are not rebuilt. Init order is Go's.

Bad:
- Generic bodies carry `#line`: editing above a generic function rebuilds its dependents (rare).
- Every runtime type needs a `constexpr` default constructor (`gp::func` included), and anonymous
  struct types used by several packages need one shared definition (`gpkg_anon.hpp`).
- Big headers slow down C++ compiles; the runtime's precompiled header and the API hash help.

## Evidence

- Audit: with `main` importing `b` and `c`, and `b` importing `z`, Go 1.25 prints `init c`,
  `init z`, `init b`. A recursive "initialize my imports first" scheme gives `z, b, c`.
- Audit: a `std::function` global got C++'s own dynamic initialization, which ran after gocpp's
  init in a shared library and wiped the value; hence `constinit`.

See: [Phase 06 §2.2](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#22-one-package--its-own-c-files), [§2.3](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#23-header-layout-c-needs-declarations-before-use-go-doesnt), [§2.4](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#24-initialization-across-packages-go-121-rule), [§2.5](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#25-build-graph-and-cache), [Phase 03 §2.10](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#210-builtins-printing-and-package-initialization).
