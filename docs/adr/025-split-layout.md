# ADR-025: The split layout: one program, many packages

Status: accepted (2026-10-18), refines [ADR-019](019-output-layout.md)

## Context

[ADR-019](019-output-layout.md) decided that every package has a header and one translation unit
for each `.gp` file. Building it (Phase 06 steps 6.2 and 6.3) showed what the sentence "type
descriptors are inline variables in `pkg.hpp`" and "one `gpkg_anon.hpp`" leave open: C++ wants one
definition of every object and every type in the whole program, while Go lets any package convert
any type to an interface and use any unnamed type, and the lowering of a package needs names that
another package chose.

## Decision

**All the packages of a program are lowered together, in one process, in the order of their
imports** (`lower.Program`). A package asks the Program for what another one chose: the C++ name
of a function, a variable, a struct or an interface (`::gpkg_geom::New`, `::gpkg_geom::Point`:
the owner's macros decide a name, the user of a name never recomputes it), the descriptor of a type,
whether a function calls `recover` (a `defer pkg.F()` needs the ticket). The files are written when
all packages are lowered, from the last package to the first, because a package that is lowered
asks its imports for descriptors, and those are part of their files.

**The files of a package** are `pkg.hpp`, `<file>.gp.cpp` for each `.gp` file that has functions,
`pkg_desc.cpp` and `pkg_init.cpp` in `gpkg/<import path>/`. Compared with ADR-019 there is one
more file, and the descriptors are in it:

- `pkg.hpp` has the types (fully qualified where they belong to another package), the fmt hooks and
  the interface wrappers (inline), the **extern declarations** of the descriptors, the prototypes,
  the **extern** package variables, the exported constants (`inline constexpr`; a string is a
  `std::string_view`, a constant of a named type is an `inline const`; the code never reads them,
  go/types folds every use), and `gp_var_init`/`gp_init_funcs`. It has no `#line`.
- `pkg_desc.cpp` defines what the header declared extern: the descriptor of each named type that
  flows into an interface, with its thunks and method tables, the descriptors of interfaces, the
  composite descriptors (`gp_tdc_N`), and the forwarders of promoted methods. **Why extern, not
  inline:** a descriptor is one object per type, compared by address (assertions, itabs), so it
  has to be defined by the package that declares the type, and a package that converts another
  package's type to an interface asks the owner to write it. As an `inline` variable it would put
  the dependents' needs into the owner's *header*, and a change in a dependent's function body
  would rebuild everything that includes the owner's header. As an extern object it is
  `pkg_desc.cpp` that changes (one unit), and the header stays the same.
- `pkg_init.cpp` has the package variables (`constinit`, defined here) and the initialization. The
  main package's also has `gp_init_all`, which calls the packages in the order of
  `load.Program.Packages` (Go 1.21's: imports first, then by import path), and `main`.

**Unnamed struct and interface types** (`gp_anon_<hash>`, `gp_anon_i<hash>`) are in the global
namespace, one C++ type for the whole program. Every header whose package uses one, or one whose
definition mentions, writes the same definition under its own include guard (`GP_ANON_<HASH>`; the
inline members of an interface in a second guard, `_IMPL`). The text is made once, by the package
that met the type first, in the spelling that names every type fully qualified, so all headers
have the same text. The name hashes the identity text with the import path of the package of an
unexported field.

**The generated files are written to `gen/<hash>` in the cache folder** (with `-work`, to `gen` in the
work folder): the hash is of the main package's folder, import path, files and the target. The folder
is the same in every build of the program, so the `-iquote` that makes `#include "gpkg/..."` work, the
`#line` paths and the object cache's keys do not change. Files are written only when their bytes
change (a rename), so the cache's records of the headers stay valid.

**Public flags.** When `pkg.hpp` includes the package's C/C++ headers (a field, a signature or a
variable has a C/C++ type), the package's compile flags (and its folder, for `"./x.h"`) are public:
every package that includes `pkg.hpp` compiles with them too, so no two units see a C struct with
two layouts. The link flags of all packages are added in the order of the packages, dependents
first; the C and C++ libraries of the modules (cxx recipes) are the union over the packages.

## Alternatives

- **Descriptors as inline variables in the owner's header** (ADR-019's text): rejected for the reason
  above; also it needs every dependent's request before the owner's header can be written, and a
  descriptor of a type nobody converts would still be emitted if the rule were "all types".
- **A descriptor in the package that converts** (several objects for a type): breaks the equality of
  descriptors by address.
- **One `gpkg_anon.hpp` for the program**: it would need the headers of all the packages whose types it
  mentions, and they need it (a named type may hold an unnamed one by value). Guards in each header
  put the definition where the types it needs are.
- **Nested or per-package namespaces for the unnamed types**: two packages would get two types for
  one Go type.

## Consequences

Good: a one-line change in a function body recompiles one unit; a change in a dependent never changes
the header of an imported package (only its `pkg_desc.cpp`); unexported names of two packages
cannot clash; the same machinery builds a package of `stdlib/` written in `.gp`.

Bad:
- A change in a package's exported types or constants changes its header, and with it the units
  of all packages that include it (the API hash of step 6.5 can do better only for the constants).
- `gp_init_all` is in the main package's `pkg_init.cpp`: a change in any package's init functions
  changes that file's text only when the *set* of packages with init code changes.
- The generated files of a program stay in the cache folder: a build removes the folders of programs
  that were not built for a week, when there are more than 100 (step 6.6 may do better).
- A C/C++ macro of a package's headers is visible in the units of the packages that include its
  `pkg.hpp` (the `#undef` of the names the generated code uses covers the package's own units).
