# ADR-010: Generated C++ names: flat package namespaces, fully qualified C names

Status: accepted (2026-10-13)

## Context

Generated code lives in one program with C and C++ headers, so names can collide: Go names
that are C++ keywords (`new`, `class`), Go names that are C macros (`EOF`, `errno`), package
namespaces that hide C functions, and user names that clash with compiler temporaries.

## Decision

- Each package gets **one flat C++ namespace**: `gpkg_` plus an escape of its import path
  (`gpkg_main`, `gpkg_fmt`), never nested. The escape: letters and digits stay; `/` → `__`,
  `.` → `_d`, `-` → `_h`, `_` → `_u`, `~` → `_t`, `+` → `_p`, any other byte → `_x` + 2 hex
  digits. Every `_` in the result starts such a code, so a name decodes back to one import path.
- **C/C++ names are fully qualified** (`::time`, `::std::vector`), so no gocpp name can hide
  them. Macros are the exception: they only work unqualified (`errno`, `RAYWHITE`).
- A `_` suffix escapes C++ keywords (the list includes `_Pragma` and `_Complex`, keywords or
  operators in GCC's and Clang's C++ mode) and gocpp names that are macros in the included
  headers (the runtime's, and from Phase 04 the user's own).
- A method named like its type (`func (U) U()`, which C++ would read as a constructor) becomes
  `U_`. User names that start with `gp_` also get a `_` suffix: `gp_` is reserved for generated
  code, and generated names never end in `_`, so the two can't clash.
- Other names that C++ reserves (`_X`, `a__b`) stay as written, because GCC and Clang accept
  them (see Evidence); `gocpp vet` warns about them. For the same reason the namespace escape
  keeps the readable `__` for `/`.
- A declaration that uses the name it declares (`v := v`) gets a new C++ name, because in C++
  the new name is already in scope inside its own initializer.

| gocpp | C++ |
|---|---|
| package `github.com/huuhait/game/engine` | `namespace gpkg_github_dcom__huuhait__game__engine` |
| `ctime.time` (a C function) | `::time` |
| `cerrno.errno` (a C macro) | `errno` |
| `std.vector[int]` | `::std::vector<int64_t>` |
| `io.EOF` (`EOF` is a C macro) | `::gpkg_io::EOF_` |
| a variable named `new`; a user name `gp_count` | `new_`; `gp_count_` |
| `func (U) U()` (a method named like its type) | `U_` |
| `v := v` | `int64_t v_1 = v;` |

## Alternatives

- **Global names** (rejected): collisions with C libraries.
- **Nested namespaces such as `gpkg::a::b`** (rejected): inside `gpkg::main`, the C function
  `time()` finds the package namespace `gpkg::time` first; and a package `game` that declares
  `engine` clashes with the sub-package namespace `game::engine`.

## Consequences

Good: C names can't be hidden, gocpp names that are C macros still compile (`io.EOF`), and two
import paths never share a namespace.

Bad:
- C++ error messages show long names. Phase 13 translates them back (`gpkg_main::Vec2` →
  `main.Vec2`).
- gocpp must know every macro name: the runtime's headers in Phase 02, the user's headers from
  the macro probe (`$CXX -dM -E`) in Phase 04. C macros that collide with gocpp names are also
  `#undef`ed (or pushed and popped) after the C/C++ includes.
- Keeping reserved names relies on GCC and Clang accepting them.

## Evidence

- Audit: with nested namespaces, `time(nullptr)` inside a package namespace failed to compile,
  because the package namespace `time` was found first.
- An earlier escape (`.` → `_`) let `x.u` and `x_` collide (Phase 06 §2.2); the codes fix that.
- Checked, and re-checked with GCC 15.2 and Clang 21.1.8: `_X`, `a__b` and `__` inside a
  namespace name compile without a warning under `-Wall -Wextra -pedantic`. Only Clang's opt-in
  `-Wreserved-identifier` warns.

See: [Phase 02 §2.1](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#21-example), [§2.2](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#22-backend-structure-internalcppgen), [Phase 04 §2.3](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#23-name-translation), [Phase 06 §2.2](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#22-one-package--its-own-c-files), [Phase 13 §2.1](../../plans/2026-10-13-gocpp/phase-13-tooling/README.md#21-diagnostics-that-read-like-go).
