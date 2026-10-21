# Review of `gocpp bindgen` on raylib, sqlite3 and SDL2

Phase 12 asks that the `bindgen` output for `raylib.h`, `sqlite3.h` and `SDL.h` is reviewed
([README §5](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md#5-exit-criteria)). This is
that review, done on 2026-10-21 with the libraries of the dev shell, and the tests that keep it true.
The command is described in [typed-interop.md §12](../spec/typed-interop.md#12-gocpp-bindgen-step-1210).

| Library | Header | Version in the dev shell | View |
|---|---|---|---|
| raylib | `<raylib.h>` | 5.5-unstable (the header says 5.6-dev) | 597 functions, 68 types, 319 constants |
| sqlite3 | `<sqlite3.h>` | 3.51.2 | 295 functions, 45 types, 504 constants, 3 variables |
| SDL2 | `<SDL2/SDL.h>` | sdl2-compat 2.32.68 | 844 functions, 198 types, about 1,490 constants (51 files) |

All three were found with `pkg-config` (`gocpp bindgen -pkg-config raylib '<raylib.h>'`). Nothing was
installed. The views were read in full for types, enums, structs, macros and signatures; the
functions were also compared with the headers mechanically, as follows.

## What was checked, and against what

- **raylib functions.** Every `RLAPI` declaration of the header (597) was converted by a small
  script (C type to `C.*`, `void *` to `unsafe.Pointer`, `const` dropped) and compared with the
  `func` lines: 597 of 597 equal. The one difference is `type` to `type_` in `LoadFontData`, which is the
  rule for Go keywords.
- **raylib enums.** All 305 enumerators of the header's `typedef enum` blocks are in the view with
  the type of their enum and the right value; the 26 color macros are `var`s with their
  struct literal; `PI`, `DEG2RAD`, `RAD2DEG` and the `MOUSE_LEFT_BUTTON`-style macros that name an
  enumerator are typed constants (`MOUSE_LEFT_BUTTON MouseButton = 0`).
- **Struct fields, all three.** clang's own record layouts (`-Xclang -fdump-record-layouts-complete`)
  give the field names of every record. The view of each record (raylib 34, sqlite3 20, SDL2 70 with a
  name in both) has the same fields in the same order (a Go keyword gets a trailing `_`: `type_`, `map_`).
- **sqlite3 and SDL2 functions.** The names that the headers declare (`SQLITE_API`, `DECLSPEC`) were
  compared with the `func` names. sqlite3: 59 of 354 are absent, all of them behind a build option of
  the library (`SQLITE_ENABLE_SESSION`, `SQLITE_ENABLE_PREUPDATE_HOOK`, `SQLITE_ENABLE_NORMALIZE`,
  `SQLITE_HAS_CODEC`: `sqlite3session_*`, `sqlite3changeset_*`, `sqlite3_preupdate_*`, ...), which the
  header's `#ifdef` leaves out for the compiler too. SDL2: 41 names are absent, all of them
  platform entry points (`SDL_system.h`: Android, WinRT, GDK, Direct3D; `SDL_main.h`) or in headers
  that `SDL.h` does not include (`SDL_vulkan.h`, `SDL_syswm.h`); the view also has 14 `static inline`
  helpers (`SDL_RectEmpty`, `SDL_memcpy4`, ...), which a program can call.
- **Signatures read by eye**: variadic functions (`TraceLog`, `TextFormat`, `sqlite3_config`,
  `SDL_snprintf`: `args ...any`, with the `va_list` twins taking `C.va_list`), pointers to pointers
  and out parameters (`sqlite3_open(filename *C.char, ppDb **sqlite3)`,
  `SDL_CreateWindowAndRenderer(..., window **SDL_Window, renderer **SDL_Renderer)`,
  `SDL_LockTexture(..., pixels *unsafe.Pointer, ...)`), function pointers and a function that
  returns one (`xDlSym func(*sqlite3_vfs, unsafe.Pointer, *C.char) func()`), unnamed parameters
  (`_`, or all unnamed), unions (`SDL_Event`, `SDL_HapticEffect`: a struct with a comment),
  anonymous unions and structs (`SDL_RWops.hidden`, `SDL_GameControllerButtonBind.value`), a bit
  field of glibc's `FILE`, a packed struct (`SDL_AudioCVT`, flagged), negative enums
  (`SDL_JoystickPowerLevel` is `C.int`), enumerators with big values, `uint8_t` and friends
  (Go's `uint8`), `size_t`, opaque handles (`incomplete type`), deprecated functions.
- **The view as a program.** Each view passes `gocpp fmt` unchanged and `gocpp check`; typed
  interop, asked about the same functions with a program that uses them, gives the same types
  (the new tests below).

## Defects found, and fixed

1. **Types nested in a forward-declared struct were lost** (`sqlite3.h` lines 7718 to 7740,
   `struct sqlite3_index_info { ... struct sqlite3_index_constraint { int iColumn; unsigned char
   op; unsigned char usable; int iTermOffset; } *aConstraint; ... struct sqlite3_index_orderby {...}
   *aOrderBy; struct sqlite3_index_constraint_usage {...} *aConstraintUsage; }`, forward declared by
   `typedef struct sqlite3_index_info sqlite3_index_info;`). The three types were empty stand-ins
   (`type sqlite3_index_constraint struct{}`), so the fields that a virtual table reads
   (`iColumn`, `op`, `usable`) were missing. A nested enum became an empty struct without its
   enumerators. Fixed in `internal/typedinterop/bindgen/model.go` (`addNested`); regression
   test `TestNestedTypesOfForwardDeclaredStruct`.
2. **A struct that a field only names was filed as a member of the class** (`sqlite3.h`:
   `const struct sqlite3_io_methods *pMethods;` in `sqlite3_file`; glibc's `<bits/types/__locale_t.h>`:
   `struct __locale_data *__locales[13];`). C++ declares such a name in the namespace around the class, but
   clang prints the declaration inside the class. The view would say `sqlite3_file_sqlite3_io_methods`
   and `__locale_struct___locale_data`; the golden files of `<string.h>`, `<stdlib.h>` and `<time.h>` had
   recorded the second one. Fixed with clang's `parentDeclContextId`; regression test
   `TestElaboratedTypeInFieldIsNotNested`. The goldens were renewed.
3. **Deprecation was dropped** (`SDL_version.h:194`, `extern SDL_DEPRECATED DECLSPEC int SDLCALL
   SDL_GetRevisionNumber(void);`). Functions, variables, structs, enums, typedefs and enumerators that the
   header marks with `[[deprecated("...")]]` or `__attribute__((deprecated))` now have a `// Deprecated: <message>`
   paragraph, the form Go tools read: a bare `//` line separates it from a comment that is already there (the
   C++ name, `incomplete type`), because Go tools only know it at the start of a paragraph. Test
   `TestDeprecatedDeclarations`.
4. **A function declared twice was listed as an overload** (glibc, seen through `<stdlib.h>` and `<stdio.h>`, which
   SDL2 and raylib pull in: `strtol` is declared again with an asm label, so are `strtoul`, `fscanf`, `vfscanf`,
   `reallocarray`, ... 17 repeats in the two golden files). A declaration with the type of an earlier one is the
   same function and is no longer written; a different type is still `// overload:`. Test
   `TestRedeclaredFunctionIsNotAnOverload`.

No other defect was found in the three views. See
[ADR-120](../adr/120-bindgen-views-of-real-headers-are-tested-by-properties-and-write-deprecated-lines.md).

## The tests

`cmd/gocpp/bindgen_libs_test.go`: `TestBindgenRaylib`, `TestBindgenSQLite3`, `TestBindgenSDL2`. Each one
runs `gocpp bindgen -pkg-config <library> <header>` and checks:

- the first lines (header, package) and no warning on standard error;
- floors for the numbers of functions, types and constants (never exact counts);
- 20 to 30 known declarations, each as a whole line (the white space inside a line is not compared),
  and blocks such as structs, the union comment, the deprecated line and the list of macros that are not translated;
- the text that must not be there (a path of the nix store, the stand-ins of defect 1);
- two runs write the same view;
- `gocpp fmt -l` lists nothing and `gocpp check` of the view is clean;
- typed interop agrees: a program that uses the library passes `gocpp check -interop=typed` with g++ and
  with clang++ (nothing is built or run: no window, no display), and calls with one wrong argument fail
  with clang++, with the message `as <type> value in argument to <package>.<function>` where `<type>` is
  read from the view's own signature line.

A library that pkg-config does not know skips its test with the reason. Run them with
`go test -run 'TestBindgen(Raylib|SQLite3|SDL2)' ./cmd/gocpp` (about 25 s, most of it SDL2's 400 MB
of AST). The regression tests are in `internal/typedinterop/bindgen/declattrs_test.go`.

## Known limits

- **Macros that rename a function or a type, and pointer macros, are listed, not written**: `GetMouseRay`
  (raylib: `#define GetMouseRay GetScreenToWorldRay`), `SDL_BlitSurface`, `SDL_BlitScaled`,
  `SDL_Colour`, `SQLITE_STATIC`, `SQLITE_TRANSIENT` appear under `// Not translated`. Programs can
  still use them (the C++ compiler resolves them); the editor view does not show their type.
- **Build-configuration macros are written.** SDL's `SDL_config.h` is in the folder of `SDL.h`, so its
  `HAVE_*`, `SIZEOF_VOIDP` and `SDL_VIDEO_DRIVER_*` macros (about 150) are constants of the view. They are
  true for the machine that made the view. `-exclude '^(HAVE_|SDL_VIDEO_DRIVER_)'` leaves them out.
- **The view is for the machine that made it**: Linux x86-64 here. Declarations behind `#ifdef _WIN32`,
  `__ANDROID__` or a library build option are absent (see above); a Windows view needs Phase 14's
  MinGW headers and is not done.
- **Float macros** are written with the shortest text of their `float` value: `PI = 3.1415927` is the
  double 3.1415927 as an untyped Go constant, which differs from raylib's `3.14159265358979323846f`
  (as a float) by 4e-9.
- **Typedefs of `void`** (glibc's `_IO_lock_t`) are an empty struct, not `unsafe.Pointer`; the pointers to
  them still check.
- **Packed and over-aligned records** are flagged (`// layout: packed`, and listed under "layout differs"),
  but the view does not say where their fields lie.
- **A struct nested in another one follows C++ scoping** (`sqlite3_index_info_sqlite3_index_constraint`, with
  `// struct sqlite3_index_info::sqlite3_index_constraint`), although C code calls it
  `struct sqlite3_index_constraint`.
- **Field and parameter names that are Go keywords get a trailing `_` in the view** (`type_`), while a gocpp
  program writes `ev.type` (a keyword is allowed after a dot).
- **Deprecated fields and member functions get no `Deprecated:` line** (functions, variables, types and
  enumerators do).
- **A deprecation on a later redeclaration** (`int f(); __attribute__((deprecated)) int f();`) is not seen:
  the first declaration of the function is the one that is written (a later declaration of the same type adds nothing).
- **SDL2 is the slow one**: about 2.3 s, almost all of it clang's 400 MB AST dump (the standard headers
  that `SDL_stdinc.h` includes). The probe of the macros is cached (0.5 to 0.9 s cold).
