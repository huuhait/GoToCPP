# The "no false errors" corpus

`TestCorpus` ([corpus_test.go](../../corpus_test.go)) checks each `.gp` file of this folder as a
program of its own, like `gocpp check file.gp`, in the module `corpus` (so `"corpus/clib"` is
the package in [clib/](clib)). It uses the real namespace probe, once with `g++` and once with
`clang++` (the compiler rules of [tests/README.md](../../../../tests/README.md#compilers)), and
no phase gates (`check.LastPhase`): the files use features of Phases 03 to 08, and they test
the front-end, not the gates (Phase 01 README §5).

- A file without `// ERROR` comments must check with no error and no warning: no false errors
  from `go/types` on opaque code, and no internal error of the opaque pass.
- A file with `// ERROR "regexp"` comments (`seeded_*.gp`, the seeded mistakes) must report
  exactly those errors on those lines, as in [tests/errors](../../../../tests/errors/README.md)
  (no `LINE`, no `WARNING` here), and nothing else.
- A file that needs a library, by a `//gocpp:pkg-config` directive or by a
  `// pkg-config: lib...` comment (for a header that is found without the directive), is
  skipped when `pkg-config --exists` doesn't find it.

| File | Covers |
|---|---|
| `containers.gp` | std containers: vectors (nested, of Go structs, of `strings.Builder`), iterators (also the dotted type `std.vector[int].iterator`), `map`, `unordered_map`, pairs, strings, `<algorithm>` and `<numeric>` calls, literals whose type is left out |
| `filesystem.gp` | `std::filesystem` through the rename `fs "::std::filesystem"` and by its full name, in fields, parameters and results |
| `chrono.gp` | `std::chrono` durations and time points as dotted types: variables, parameters, results, struct fields, slices, a template call |
| `templates.gp` | opaque template arguments: constants, constant expressions, `len` of an array, Go types, nested `std.array`, a template of a local header |
| `localheaders.gp` | a local C header (structs, a typedef, an enum, a macro) and a C++ header with namespaces (`geo`, `geo.detail`); named types over C structs with methods, map keys, constants, indexes, `append` and `min` |
| `shadowing.gp` | locals, parameters, results, fields, range and switch variables named like namespaces and header packages |
| `sdl.gp` | SDL2 found without the directive (probed): pointers to C structs, flags, events, a C struct literal |
| `raylib.gp`, `sqlite.gp` | raylib and sqlite3 with `//gocpp:pkg-config`, probed with the flags that pkg-config gives |
| `crosspkg.gp` | opaque values of another package (`clib`): variables, results, named types over C types, a Go struct with C fields |
| `conversions.gp` | conversions of opaque values to Go types (they stay typed), C's built-in types |
| `closures_generics.gp` | opaque values captured by closures, method values, generic calls, `defer`, `go` |
| `cstdio.gp` | `<cstdio>`, a named `<string.h>`, `<cstdlib>`, `C.CString`, `C.cstr`, `C.GoString`, `C.int` |
| `statements.gp` | opaque values in if/switch init statements, type switches, labeled loops, channels, `select`, interfaces, builtins, arrays and maps of C types, an embedded Go struct with C fields |
| `seeded_namespaces.gp` | namespace typos (`stdd`, `geoo`, a bad rename), next to correct uses |
| `seeded_types.gp` | wrong types with the typed results of conversions, undefined names in template arguments, a generic type without type arguments and non-constant values as template arguments |
| `seeded_mixed.gp` | Go mistakes inside expressions with C/C++ operands that start at the same place (`n+s == h`), next to the same shapes without a mistake |
| `seeded_header.gp`, `seeded_local_header.gp` | a misspelled system header, a misspelled local header |

The headers [geom.h](geom.h) (C) and [shapes.hpp](shapes.hpp) (C++ with namespaces) are real, so
the probe compiles them.
