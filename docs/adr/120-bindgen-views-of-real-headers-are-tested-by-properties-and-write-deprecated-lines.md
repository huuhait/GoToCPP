# ADR-120: The views of real headers are tested by properties, and bindgen writes `Deprecated:` lines

Status: accepted (2026-10-21). Phase 12, closes the exit line "`bindgen` output for `raylib.h`,
`sqlite3.h` and `SDL.h` is reviewed". Builds on [ADR-092](092-bindgen-writes-a-go-syntax-view-from-clang.md).

## Context

The review of the three views was a one-off look. Reading raylib, sqlite3 and SDL2 against their
headers found two real defects in the way bindgen reads clang's AST, and one attribute that the view
dropped although a reader needs it:

- A struct that is forward declared and then defined (`typedef struct sqlite3_index_info
  sqlite3_index_info;` then the definition) lost the types declared in its body: the field
  `aConstraint` pointed to an empty `struct{}` stand-in, and a nested enum was an empty struct without
  its enumerators. The redeclaration was merged, but only the first declaration's children were walked.
- A struct that a field only names (`struct sqlite3_io_methods *pMethods;`, glibc's
  `struct __locale_data *__locales[13]`) was filed as a member of the class: clang prints the
  declaration inside the class, but C++ declares the name in the namespace around it. The view said
  `sqlite3_file_sqlite3_io_methods` or `__locale_struct___locale_data`.
- A function that the header declares again with the same type (glibc: `strtol`, `fscanf`, ...,
  twice in a row, the second time with an asm label) was listed as `// overload:`.
- `[[deprecated]]` and `__attribute__((deprecated))` (SDL's `SDL_GetRevisionNumber`) were not shown.

## Decision

1. The nested types of a class come from the declaration that has the body, also when it follows a
   forward declaration. A nested declaration that clang marks with `parentDeclContextId` (its scope is
   not the class) belongs to the namespace around the class, taking the classes off the scope.
2. A function, variable, struct, enum, typedef or enumerator with a `DeprecatedAttr` gets a
   `// Deprecated: <message>` paragraph above it (without a message it reads "the header marks it
   deprecated."), after a bare `//` line when another comment line is there, because Go tools only read
   the word at the start of a paragraph. Fields and member functions are not covered. No other attribute
   is written (the layout ones already were).
3. A declaration with the clang type of an earlier declaration of the same name is the same function
   and writes nothing. Parameter names and the top-level const/volatile of by-value parameters are
   ignored; pointee const, references, volatile pointees and the result type keep overloads apart, so
   `f(int*)` and `f(const int*)` are two overloads.
4. The evidence for a real library is one test per library (`cmd/gocpp/bindgen_libs_test.go`), not a
   golden file of thousands of lines: counts above a floor, exact signature lines (compared with
   the white space of a line squeezed, since gofmt aligns a block to its longest name), the view under
   `gocpp fmt` and `gocpp check`, and typed interop in agreement with it: a program that uses the
   library passes `gocpp check -interop=typed` with both compilers, and a wrong argument is rejected
   with the type that the view gives the parameter. A library that pkg-config does not know skips its test
   with the reason. The goldens of the zoo (`testdata/golden/zoo`) stay, guarded by a hash of the
   headers and the compiler; those of sqlite3, `<stdio.h>`, `<stdlib.h>`, `<string.h>` and `<time.h>` were renewed:
   they had recorded the wrong nesting and the repeated declarations.

## Consequences

The view of sqlite3 shows the fields of the virtual-table structs (`sqlite3_index_info_sqlite3_index_constraint`),
glibc's `__locale_data` is an incomplete type of its own, and deprecated functions are visible in the editor.
The test floors are low on purpose: a library may add declarations, and a header update that removes
a function the tests name is a change to read, not to bless.
