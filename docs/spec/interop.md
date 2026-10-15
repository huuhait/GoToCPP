# Using C and C++ from gocpp

A `.gp` file can import any C or C++ header and use it directly, with no glue code. This
document says how: import forms and names, what `C` holds, how names and types are translated,
what you can do with C/C++ values, strings and pointers at the boundary, callbacks, and how the
C/C++ side is built.

| | |
|---|---|
| Status | v0, written in Phase 00 from the plan. [Phase 04 step 4.19](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#3-steps) finishes it and adds a cookbook |
| Source | [Phase 01 §2.3–2.5](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification) and [§2.8](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#28-the-opaque-pass), [Phase 04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md), and the other READMEs linked below |
| See also | [language.md](language.md) ([D2](language.md#d2-header-imports-instead-of-cgo), [D14](language.md#d14-cc-values-follow-c-rules), [D16](language.md#d16-unsafepointer-and-pointers-from-c)), [cpp-mapping.md](cpp-mapping.md) |

**Opaque mode.** gocpp does not try to understand C++ (templates, overloads, macros). Every
C/C++ name is **opaque**: gocpp writes it into the C++ output almost unchanged, and the C++
compiler, which understands C++ perfectly, checks it. That is why almost every header, library
and template works from the start. gocpp only needs two small facts from the compiler: is a name
a macro, and is it a namespace, a type, a template or a value ([the probes](#the-two-compiler-probes)).
Later, [typed interop](#what-typed-interop-will-add) (Phase 12) asks clang for real types.

## A first example

```go
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

func main() {
	var db *sqlite3.sqlite3 // raw C pointer
	if rc := sqlite3.sqlite3_open(":memory:", &db); rc != sqlite3.SQLITE_OK {
		panic("open failed")
	}
	defer sqlite3.sqlite3_close(db)
	fmt.Println("sqlite", C.GoString(sqlite3.sqlite3_libversion()))
}
```

```go
package main

import (
	"fmt"

	"<algorithm>"
	"<map>"
	"<string>"
)

func main() {
	ages := std.map[std.string, int]{} // Go keyword "map" is allowed after "." (Phase 01 patch)
	ages["bob"] = 31                   // a C++ type keeps C++ rules: operator[] inserts
	for name, age := range ages {
		fmt.Println(name, age)
	}
	lo, hi := std.minmax(3, 1)
	fmt.Println(lo, hi)
}
```

- `<sqlite3.h>` is a package named `sqlite3`. Its functions, types and macros are
  `sqlite3.sqlite3_open`, `sqlite3.sqlite3`, `sqlite3.SQLITE_OK`.
- `<map>` and `<string>` get no package name (`map` is a keyword, `string` is predeclared), but
  they declare the namespace `std`, and `std` needs no import.
- `//gocpp:pkg-config sqlite3` adds the compile and link flags of the library.

Source: [Phase 04 §2.1](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#21-what-the-user-writes).

## Import forms

| Import | Kind | Becomes in C++ | Name used in code |
|---|---|---|---|
| `"fmt"`, `"myapp/engine"` | gocpp package | `#include "gpkg/…/pkg.hpp"` | `fmt.X`, `engine.X` |
| `"<raylib.h>"`, `"<SDL2/SDL.h>"`, `"<cstdio>"`, `"<vector>"` | system header | `#include <raylib.h>` | a package named after the file: `raylib.X`, `SDL.X`, `cstdio.X` |
| `"./physics.h"`, `"native/x.hpp"` (the path ends in `.h .hh .hpp .hxx .h++ .inl`) | local header (looked up next to the `.gp` file, then in the include directories) | `#include "…"` | `physics.X`, `x.X` |
| `rl "<raylib.h>"`, `_ "<GL/gl.h>"` | a header with a name you choose, or with no name | as above | `rl.X`, or no name |
| `fs "::std::filesystem"` | a C++ namespace with a new name (optional) | nothing (it is just a name) | `fs.path` → `::std::filesystem::path` |
| `"::std"` (no name) | an error: namespaces need no import | — | the message says: use `std.X` directly, or rename it: `import cstd "::std"` |

No new syntax is needed: the shape of the import path says what it is. A path in `<...>` is a
system header. A path that ends in `.h`, `.hh`, `.hpp`, `.hxx`, `.h++` or `.inl` is a local
header. A path that starts with `::` is a namespace. Every other path is a gocpp package, but a
path that starts with `./` or `../` must be a header.

### Header packages and their names

```go
import (
	"./my-lib.h"         // package my_lib: "-" is not allowed in Go names
	_ "<GL/gl.h>"        // included, with no package name
	"<SDL2/SDL.h>"       // package SDL
	rl "<raylib.h>"      // package rl: renamed, like a Go import
	cstring "<string.h>" // "string" is predeclared, so this header needs a name you choose
)
```

- **A header is a package.** Its default name is the file name without its folder and its last
  extension (`<SDL2/SDL.h>` → `SDL`, `"./physics.h"` → `physics`, `<foo.bar.h>` → `foo_bar`).
  Characters that Go doesn't allow in names become `_`.
- Its members are the header's **C-level names**: functions, types, global variables, enum
  constants and macros (`raylib.InitWindow`, `raylib.Color`, `raylib.RAYWHITE`).
- **No default name** in these cases. Name the header yourself if you need its C names:
  `import cstring "<string.h>"`.
  - The name is a Go keyword (`<map>`, `<sys/select.h>`) or a predeclared Go name (`<string>`,
    `<string.h>`, `<new>`).
  - The name is not a valid name (`<3dmath.h>` starts with a digit), or it is `C`, `init` or `_`.
  - Another import of the file uses the name: a gocpp package (`<math.h>` next to `"math"`), a
    name you chose, or a namespace rename. A gocpp package always keeps its name.
  - A package-level name of the package uses it, in any file of the package.
  - Two different headers would get the same name (`<time.h>` and `<sys/time.h>`). Then neither
    gets it, so the order of the imports doesn't matter.
- If you use such a header by its file name anyway, you get a hint instead of a bare type error:
  `string.strlen` → "give `<string.h>` a name: `import cstring "<string.h>"`".
- **A name you choose follows Go's rules.** Two imports of one file can't have the same name
  (`rl redeclared in this block`), and an import name can't be a package-level name
  (`rl already declared through import of <raylib.h>`). You can import the same header twice
  with two different names (`"<raylib.h>"` and `rl "<raylib.h>"`). Twice with the same name is an
  error, also when one of the two is the default name. Like `import "fmt"` twice in Go, that is
  `raylib redeclared in this block`.
- A name you choose can't be `init` or a predeclared Go name such as `string` or `len`
  (`cannot import header as string - string is a predeclared Go name`). Choose another name,
  like `cstring`. It can't be `C` either (see [What C holds](#what-c-holds)).
- **These header imports are errors:** an empty path, a path that is not well formed (`"<>"`,
  `"<raylib.h"`, `"raylib.h>"`, a space at the start or at the end), a path with `\` (use `/`) or
  with a character that can't be in an `#include` line, and a dot import (`. "<raylib.h>"`).
- A path that starts with `./` or `../` must end in `.h`, `.hh`, `.hpp`, `.hxx`, `.h++` or
  `.inl`. A C or C++ file (`.c`, `.cc`, `.cpp`, `.cxx`) can't be imported: import its header.
  gocpp compiles the C and C++ files in the package folder
  ([Building C and C++ sources](#building-c-and-c-sources)).
- A header import never causes an "imported and not used" error, because a header also brings
  its namespaces and macros.
- C++ has only one global namespace, so a C-level name compiles through any header package:
  `raylib.printf` works when `<stdio.h>` is included somewhere. That is harmless; typed interop
  (Phase 12) knows which header declares what and turns it into a warning.

### Namespaces are packages

```go
package main

import (
	"fmt"

	fs "::std::filesystem" // optional: a shorter name
	"<chrono>"
	"<filesystem>"
	"<vector>"
)

func main() {
	v := std.vector[int]{1, 2, 3} // no import needed for std
	var d std.chrono.seconds      // a dotted C++ type name
	p := fs.path("/tmp/x.txt")
	fmt.Println(v.size(), d.count(), p.filename())
}
```

- **Every namespace that the imported headers declare can be used directly**, with no import:
  `std.vector[int]`, `std.chrono.seconds`, `glm.vec3`, `ImGui.Begin`.
- This works in files that import at least one header. In other files, such a name is an
  ordinary Go `undefined` error.
- A name is taken as a namespace only when it is declared nowhere in Go: not a local, not a
  package-level name of any file in the package, not an import, not a predeclared name. So a
  local variable named `std` hides the namespace, as Go scoping says.
- **Typos are still found early.** gocpp compiles one small check file with the file's includes
  (the namespace probe). If `stdd` is not a namespace, you get a normal `undefined: stdd` error at
  that spot, before any C++ is generated. The result is cached per include set.
- The same check finds problems with the headers themselves. Each one is an error at the
  import that brought the header in:
  - `cannot find header <raylibb.h>`: the compiler doesn't find the header.
  - `header "./bad.h" doesn't compile: bad.h:3:12: <the compiler's first message>`: the header,
    or a header that it includes, has an error.
  - `header "./bad.h" ends inside a declaration or a block that it doesn't close`: for example
    an `extern "C" {` without its `}`.
  - `header "./x.h" can't be read: <the compiler's message>`: for example no read permission.
  - `::std::filesystm is not a namespace of the imported headers`: a renamed namespace that the
    file's headers don't declare.
  - `cannot rename ::std::chrono: the file imports no header that could declare it`: a namespace
    rename in a file without header imports.
  - `cannot check the header imports: <reason>`: the check couldn't run, for example because no
    C++ compiler was found. Without a compiler, this error comes once per package; when the
    check itself fails, once per group of files with the same header imports.

  When a file's headers have errors, gocpp can't tell which names are namespaces. Then it reports
  nothing about the file's namespace names until the headers are fixed.
- **Header imports need a C++ compiler for the target**, even for `gocpp check`. It is the
  compiler in `GOCPP_CXX`, else the usual one (`CXX`, `clang++`, `g++`). When checking for
  another system, it is `GOCPP_CXX`, else the known cross compiler (`x86_64-w64-mingw32-g++` for
  windows/amd64). A program without header imports needs no compiler.
- **The check uses the package's flags**: those of `//gocpp:pkg-config` and `//gocpp:cflags`,
  and `GOCPP_CFLAGS` and `GOCPP_CXXFLAGS` ([Flags](#flags-and-the-allow-list)), so a header that
  only `pkg-config --cflags` finds (`<SDL.h>` in `/usr/include/SDL2`) is found. When these flags
  have an error (a flag that is not allowed, a pkg-config package that isn't installed), the
  error is at the directive, and the package's header imports are not checked until it is fixed.
- **Rename a namespace** when it is long or clashes with another name:
  `import fs "::std::filesystem"`, or `import cfmt "::fmt"` next to gocpp's `fmt`.
- A namespace import **without a name** (`import "::std"`) is an error, because namespaces need
  no import. The message says to use `std.X` directly, or to rename the namespace, with an
  example made from your path: `import chrono "::std::chrono"` for `"::std::chrono"`. A
  namespace whose first name can't be a name in Go code, such as `::go` (a Go keyword) or
  `::error` (a predeclared name), can only be used with a new name: `import cgo "::go"`.
- A namespace path is `::` and C++ names joined with `::`: `"::std"`, `"::std::filesystem"`.
  Other paths with `::` are errors: `"::"`, `"::std::"`, `"::1x"`, and `"std::vector"` (it must
  start with `::`).
- A namespace import only gives a new name, so `_ "::std"` and `. "::std"` are errors. The new
  name follows the same rules as a name you choose for a header (see above).
- **A header name that is also a namespace:** `<glm/glm.hpp>` is the package `glm`, and `glm` is
  also a namespace. Then `glm.X` looks in the namespace first, then in the C-level names (the
  kind probe picks the one that exists).

### The order of header imports

- Put header imports in **their own group**, separated by a blank line, after the gocpp imports.
- Header imports become `#include` lines in the order you wrote them. C/C++ include order can
  matter (`<winsock2.h>` before `<windows.h>`, `<GL/glew.h>` before `<GL/gl.h>`). So `gocpp fmt`
  leaves an import group that contains a header or a namespace import exactly as you wrote it:
  it doesn't sort the group, and it doesn't remove duplicates from it. (Plain gofmt would sort
  it.)
- `gocpp fmt` sorts the other groups like gofmt does. A group is what gofmt takes as one: import
  lines with no empty line and no comment line between them. Two `import` declarations are never
  sorted together.

```go
import (
	"fmt" // gocpp fmt sorts this group, like gofmt
	"os"

	"<winsock2.h>" // and keeps this group as you wrote it
	"<windows.h>"
)
```

Sources: [Phase 01 §2.3](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification),
[§2.5](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#25-preprocessing-before-gotypes-proven-by-the-spike),
[§2.11](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#211-gocpp-fmt-gofmt-for-gp).

## What C holds

```go
package main

import (
	"<cstdio>"
	"<cstring>"
)

func main() {
	name := "gocpp"
	cstdio.printf("hello %s (%d letters)\n", C.cstr(name), C.int(len(name)))

	n := cstring.strlen("string literals go straight to C")
	cstdio.printf("strlen = %zu\n", n)
}
```

`C` holds only **C's built-in types** and the **boundary helpers**, as in cgo. Everything a header
declares comes through its package or a namespace, never through `C` (`cstdio.printf`, not
`C.printf`).

| `C` type | C++ |
|---|---|
| `C.char`, `C.schar`, `C.uchar` | `char`, `signed char`, `unsigned char` |
| `C.short`, `C.ushort` | `short`, `unsigned short` |
| `C.int`, `C.uint` | `int`, `unsigned int` |
| `C.long`, `C.ulong` | `long`, `unsigned long` |
| `C.longlong`, `C.ulonglong` | `long long`, `unsigned long long` |
| `C.float`, `C.double` | `float`, `double` |
| `C.size_t` | `size_t` |

| Helper | What it does | More |
|---|---|---|
| `C.cstr(s)` | a NUL-terminated `const char*` for a gocpp string, valid until the end of the statement | [strings](#strings-at-the-boundary) |
| `C.CString(s)`, `C.free(p)` | a `malloc`ed C copy of a string (cgo-compatible), and freeing it | [strings](#strings-at-the-boundary) |
| `C.GoString(p)`, `C.GoStringN(p, n)`, `C.GoBytes(p, n)` | C memory → gocpp `string` or `[]byte` (cgo-compatible) | [strings](#strings-at-the-boundary) |
| `C.wstr(s)`, `C.GoStringW(p)` | UTF-16 strings for Windows (Phase 14) | [Windows](#windows-headers) |

- Every `.gp` file can use `C` without importing it. The front-end adds `import "C"` only to the
  files that use it: a file that doesn't use C stays plain Go.
- So `import "C"` in a `.gp` file is an error. There is no inline C preamble either: put C code
  in a header next to the `.gp` file.
- **`C` is reserved** for C's built-in types. A declaration named `C` is an error:
  `cannot declare C - C is reserved for C's built-in types`. That is a func, variable, constant or
  type (package-level or local), a name on the left of `:=`, a parameter or result (of a func,
  method or closure), and a type parameter. Struct fields, methods and labels can be named `C`.
- An import can't give the name `C` either (`import C "<x.h>"`, or a gocpp package whose package
  name is `C`), and a package can't be named `C` (`package C`).

Sources: [Phase 01 §2.3](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification),
[Phase 04 §2.4](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#24-cc-types-in-gocpp-code)
and [§2.6](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#26-strings-at-the-boundary).

## How names are translated

| gocpp | C++ | Why |
|---|---|---|
| `stdio.printf`, `ctime.time` (header packages, not macros) | `::printf`, `::time` | **fully qualified**, so no gocpp name can hide them. Inside a package namespace, a plain `time(nullptr)` would find the package namespace `time` first and fail to compile |
| `cerrno.errno`, `raylib.RAYWHITE`, `stdio.EOF` (macros) | `errno`, `RAYWHITE`, `EOF` (**unqualified**) | macros only work unqualified; `::errno` would break |
| `std.vector`, `fs.path` (namespaces; `fs` renamed with `import fs "::std::filesystem"`) | `::std::vector`, `::std::filesystem::path` | fully qualified namespaces are unambiguous |
| `glm.vec3` (`<glm/glm.hpp>` is package `glm`, and `glm` is also a namespace) | `::glm::vec3` | the namespace is tried first, then the C-level name |
| `std.chrono.steady_clock.now()`, `std.string.npos`, `ui.Color.Red` (types) | `::std::chrono::steady_clock::now()`, `::std::string::npos`, `::Color::Red` | class scope and `enum class` values need `::` |
| `std.cout.flush()`, `lib.sp.x` (values) | `gp::deref(::std::cout).flush()`, `gp::deref(::sp).x` | members of values use `.` after `gp::deref`, which passes objects through and dereferences raw pointers with Go's nil check |
| `stat.struct_stat`, `lib.enum_x`, `lib.union_y` | `struct ::stat`, `enum ::x`, `union ::y` | cgo's convention for C tag names; qualified, so a gocpp `type stat` can't hide the tag |

### The two compiler probes

Once per set of includes and flags (cached), gocpp asks the C++ compiler of the build two
questions:
1. **Which names are macros?** `$CXX -dM -E` lists every macro the headers define.
2. **What kind of thing is each C/C++ name?** One small `-fsyntax-only` file with one line per
   dotted prefix the package uses (`using gp_k1 = ::std::chrono::steady_clock;`,
   `namespace gp_k2 = ::std::chrono;`, `auto gp_k3 = &::std::cout;`). The lines that compile say
   whether a prefix is a **namespace, a type, a template or a value**.

Without the kind probe, gocpp couldn't tell `std.chrono.steady_clock.now()` (a static member,
which needs `::`) from `std.cout.flush()` (an object, which needs `.`). The same probe decides
whether `x[...]` after a C/C++ name is a template (`std.vector[int]`) or an index (`lib.arr[3]`).
When the probe can't classify a name (for example because it depends on template arguments),
gocpp falls back to `.` for values and gives a hint: **a parenthesized C++ value is always a
value**, so `(x).m` uses `.`, and `(v)[i]` is always indexing
([Phase 04 §6](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#6-risks)). Phase 12
makes all of these exact.

### C macros must not break gocpp names

- A header may define a macro that collides with a name gocpp emits (X11's `#define Bool int`
  would break `atomic.Bool` and `flag.Bool`). So gocpp headers are included **before** C/C++
  headers.
- The package's own names that collide with a macro of its headers get a `_`, like the names that
  collide with the runtime's macros (`errno` → `errno_`,
  [cpp-mapping.md](cpp-mapping.md#names-inside-a-package)): `type Bool bool` next to X11's
  `#define Bool int` is the C++ struct `Bool_` (`%T` still says `main.Bool`). So the header's other
  macros that expand to `Bool` keep working.
- Every other name that the generated code uses and that is a macro of the headers (a name of the
  runtime or of another gocpp package, such as `Println` in `gp::fmtshim::Println`) is `#undef`ed
  after the C/C++ includes. The macro list from probe 1 makes this exact. `#undef` is enough (no
  `push_macro`/`pop_macro`): every C/C++ include comes before the generated code, so nothing
  after it needs the macro again.
- A macro that the program uses through its header package (`p.Println(3)`) and whose name the
  generated code needs for something else (`fmt.Println`) can't be both: the build stops with an
  error at the macro's use. Call the macro from a small helper header instead
  ([below](#small-helpers-a-header-next-to-the-gp-file)).

Sources: [Phase 04 §2.2](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#22-the-two-compiler-probes-macros-and-kinds),
[§2.3](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#23-name-translation).

## C/C++ types in gocpp code

```go
func open(path string) *cstdio.FILE { // a raw C pointer
	return cstdio.fopen(C.cstr(path), "r")
}

type Scene struct {
	Window *SDL.SDL_Window     // SDL_Window*: a raw pointer, C++ rules
	Points std.vector[Point]   // std::vector<::gpkg_main::Point>
	Delay  std.chrono.seconds  // a dotted type name
	Buf    std.array[C.int, 4] // std::array<int, 4>
}
```

| gocpp type syntax | C++ |
|---|---|
| `C.char`, `C.int`, `C.size_t`, … | `char`, `int`, `size_t`, … ([the table above](#what-c-holds)) |
| `*cstdio.FILE`, `*SDL.SDL_Window`, `**C.char` | `FILE*`, `SDL_Window*`, `char**`: raw pointers with C++ rules |
| `std.vector[int]` | `std::vector<int64_t>` (gocpp's `int` is 64-bit) |
| `std.array[C.int, 4]`, `std.bitset[64]` | `std::array<int, 4>`, `std::bitset<64>` (constant arguments, [D4](language.md#d4-constants-as-type-arguments)) |
| `std.vector[Point]`, `std.vector[*Node]` | `std::vector<::gpkg_main::Point>`, `std::vector<gp::ptr<::gpkg_main::Node>>`: gocpp pointers inside C++ containers stay reference counted, so they are safe |
| `std.vector[int].iterator` | `std::vector<int64_t>::iterator` (member types, from the kind probe) |

- `go/types` has no types for C/C++ things, so the front-end records the **type expression** at
  every declaration that uses one (parameters, results, struct fields, variables), and the code
  generator prints the C++ type from that syntax.
- **Dotted type names** (`std.chrono.seconds`, `std.filesystem.path`) work in every type position
  (Phase 01 parser patch 3).
- **C/C++ types as type arguments of gocpp generics** (`Map[C.int, string]`): written type
  arguments are spelled from the syntax, and inferred ones become `decltype`-based arguments. Where
  neither works, you get an error that asks you to write the type arguments out (until Phase 12).
  The C++ compiler checks the constraints, and its errors are your errors
  ([Phase 07 §2.4](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#24-generic-types-descriptors-aliases-interop)).

Source: [Phase 04 §2.4](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#24-cc-types-in-gocpp-code).

## Values and operations

### Typed and opaque expressions

```go
x := lib.f()                // opaque: auto x = ::f();
var total int = arr.size()  // typed: total is a gocpp int (int64_t total = ...)
n := int(cstring.strlen(p)) // typed: n is a gocpp int
fmt.Println(n + 1)          // fully type-checked by gocpp
```

- An expression is **opaque** when gocpp has no type for it and it comes from C/C++: it is
  rooted in a header package, a namespace or `C`; or in an object declared with a type that
  contains a C/C++ type (`var xs []C.int`, then `xs[0]`); or in a call of a function whose result
  type contains one (`f()` returning `*cstdio.FILE`); or in a variable initialized from any of
  these. The C++ compiler checks opaque code. gocpp emits it with `auto` and small helpers
  (`gp::deref`, `gp::range`, …).
- Every other expression is **typed** and gets gocpp's exact C++ type.
- A **conversion** of an opaque value to a gocpp type stays typed: `int(cstring.strlen(p))` is an
  `int`, so later mistakes with `n` are still reported. (Internally it is rewritten to a hidden
  generic helper; the C++ is a normal cast.)
- Safety rule inside the compiler: if `go/types` reports no errors, every expression without a
  type must be opaque. If not, that is an internal compiler error with a clear message, never
  silently wrong C++.

### Operations on C/C++ values

| gocpp | C++ | Note |
|---|---|---|
| `x := lib.f()` | `auto x = ::f();` | |
| `x.m`, `x.m()` with `x` opaque | `gp::deref(x).m` | `gp::deref` dereferences one level of a pointer, like Go's auto-deref: a raw pointer, or a gocpp pointer (`gp::ptr`) that comes back as an opaque value, such as an element of a `std.vector[*Node]` or a gocpp pointer returned by a C++ template (`gp::deref(v[0]).Name`), with Go's nil panic for both; values pass through. C++ smart pointers are not dereferenced for you: write `(*sp).m` |
| `p.next.value`, `lib.get_fn()(2)` | `gp::deref(gp::deref(p).next).value`, `::get_fn()(2)` | chains of members, and calls of anything C++ can call (function pointers, objects with `operator()`) |
| `*p`, `(*sp).m` | `gp::cgo::star(p)`, `gp::deref(gp::cgo::star(sp)).m` | a raw pointer (and a gocpp pointer) gets Go's nil panic; smart pointers and iterators keep their own `operator*` |
| `a + b`, `a == b`, `v[i]` on C++ values | `a + b`, `a == b`, `v[i]` | C++ operators and C++ rules: no gocpp bounds check on C++ containers (use `.at(i)` if you want one), and `m[k]` of a `std::map` inserts `k` |
| `^x`, `x &^ y`, `x &^= y` | `~x`, `x & ~y`, `x &= ~y` | Go's spellings of C++'s operators |
| `s += "!"`, `it++`, `n--` | `s += "!"`, `++it`, `--n` | C++'s own `op=` (a class's `operator+=`), and the prefix `++`, which an iterator needs no copy for |
| `b := p == nil`, `var ok bool = x < 3` | `bool b = gp::cgo::to_bool(p == nullptr);` | a comparison of C/C++ values is a Go `bool`, as in cgo: it goes into bool variables, results, gocpp functions and `fmt`, with `!`, `&&` and `\|\|`. Unlike Go's untyped bool, a named bool type needs a conversion, `B(x == 3)`. `gp::cgo::to_bool` checks that a class's `operator==` gives something that converts to bool |
| `if v.empty()`, `for !done` | `if (gp::cgo::cond(gp::deref(v).empty()))` | a condition must be a bool, as in Go: a C `int` (`if isdigit(c)`), a pointer (`if p`) or a `std::optional` is a C++ compile error; compare it (`!= 0`, `!= nil`). An object that converts to bool implicitly, such as a `std::vector<bool>` element (`if bs[0]`), is fine. A comparison needs no check |
| `&x` (`x` a C/C++ variable), `&lib.global` | `&x`, `&::global` | a raw C++ address: C++'s lifetime rules ([below](#pointers-unsafe-and-slices-at-the-boundary)) |
| `std.string("hi")`, `std.vector[int](10, 0)` | `std::string("hi")`, `std::vector<int64_t>(10, 0)` | conversion syntax calls a constructor; several arguments are allowed ([D5](language.md#d5-constructor-calls-for-cc-types)) |
| `std.vector[int]{1, 2, 3}` | `std::vector<int64_t>{1, 2, 3}` | |
| `SDL.SDL_Rect{0, 0, 640, 480}` | `SDL_Rect{0, 0, 640, 480}` | positional: aggregate initialization |
| `SDL.SDL_Rect{w: 640, h: 480}` | `[&]{ SDL_Rect gp_t{}; gp_t.w = 640; gp_t.h = 480; return gp_t; }()` | keyed: field assignments, because GCC rejects designated initializers out of declaration order, and opaque mode doesn't know the order. Keyed literals with array fields are an error until Phase 12 |
| `nil` compared with or assigned to a C pointer | `nullptr` | |
| `for i, v := range cppContainer` | `for (auto&& [i, v] : gp::range(c))` | works for anything with `begin()`/`end()` and for C arrays. `v` is a **copy**, like Go. One variable is the index (the key for maps), like Go. A temporary container is kept alive by value, and random-access containers are walked by index with the size read **once**, as Go reads the length once |
| `a, b := std.minmax(x, y)` | `auto gp_t0 = gp::values(std::minmax(x, y)); auto a = std::get<0>(gp_t0); auto b = std::get<1>(gp_t0);` | C++ results that are references (`std::minmax` returns `pair<const int&, const int&>`) are copied into values, so they can't dangle. Also with `=`, `_` and variables that exist. (One variable, `p := std.minmax(x, y)`, still keeps the references until step 4.7) |

**gocpp values next to C/C++ values.** As in cgo, a typed gocpp number, bool or string next to a
C/C++ value in an operator is an error, because C++ would convert one of them without a word
(`n + v.size()` would turn a negative `n` into a huge unsigned number):

```go
var x C.int = 2
n := 3
_ = x + 1      // fine: 1 is a C literal
_ = int(x) + n // fine: both are gocpp ints
_ = x + n      // invalid operation: x + n (mismatched types C.int and int): convert one of them: int(x) or C.int(n)
_ = v.begin() + C.long(k) // an iterator and a C long: fine (v.begin() + k is the error, with this hint)
_ = x << n     // fine: Go allows a shift count of any integer type
_ = n << x     // invalid operation: n << x (shift count x is a C/C++ value): convert it, uint(x)
```

The same holds for `x op= y`. When gocpp doesn't know the C/C++ side's type, or it is no number
(an iterator, a pointer), the hint is to convert the gocpp side, `C.long(n)` or `C.size_t(n)`. A
string next to a C++ string needs `std.string(s)`. (`C.int + C.uint` follows C++'s rules, where
cgo rejects it.) Other gocpp
values meet C++'s operators as they are: a gocpp pointer compares with a gocpp pointer that a
C++ container holds (`nodes[0] == n`), and the operands of `&&` and `||` are bools anyway.

**Places.** `v[i]`, `m[k]`, `p.m` and `*p` of C/C++ values can be assigned, changed with `op=`,
`++` and `--`. Their types are C++'s, so a gocpp value stored there converts with C++'s rules
(`ages["carol"] = n`), as an argument of a C/C++ function does. A call is no place, as in Go,
also where C++ returns a reference: `v.at(0) = 5` is "cannot assign to v.at(0) (neither
addressable nor a map index expression)"; write `v[0] = 5`. A new variable is a copy, as Go's
values are, also of a C++ reference: `x := v[0]` and `all := lib.the_list()` copy (`auto`).
Several variables take one C/C++ value only from a call that gives a pair or a tuple
(`a, b := std.minmax(x, y)`): `v, ok := ages["bob"]` is "assignment mismatch: 2 variables but
ages["bob"] is 1 value", because a C++ map has no comma-ok form (use `ages.find(k)` or
`ages.count(k)`).

**Templates or indexes (step 4.5).** `x[...]` after a C/C++ name gives template arguments when the
kind probe says the name is a template: a class or alias template (`std.vector[int]`) or a
function or variable template (`std.make_unique[std.string]("x")`, `std.get[0](t)`,
`std.is_same_v[A, B]`, `std.in_place_index[1]`). After anything else it is an index
(`lib.table[1]`, `v[i]`). In parentheses, a C/C++ value is always a value: `(x)[i]` is an index
and `(x).m` a member with `.`. When the probe can't tell (a member of a template's instance,
`std.vector[int].iterator.x`), gocpp says so and suggests the parenthesized form.

**Order of evaluation.** The operands are evaluated in Go's order, also where C++ leaves the order
open (a call's arguments, the operands of `+`): gocpp moves calls into temporaries where it
must, as for gocpp code; nothing else goes into a temporary. A temporary is a copy of the value,
as Go's results are, and it is moved into its one use, so a value that can only be moved
(`lib.take(std.make_unique[C.int](1), f())`) still works. An object whose member or element is
used next (`lib.the_list()[0] = f()`) stays the object itself (`auto&&`), and its own operands
that are temporaries (calls, constructors, constants) go into named temporaries first, so that a
reference that a C++ function returns into them (`std.max(std.string("a"), std.string("b"))[0]`,
`s.append(x)` of a temporary `s`) can't dangle. Its other operands, names and places
(`lib.pick(v[0], v[1]).x = f()` with `P& pick(P&, P&)`), go to it as they are. A C/C++ global is read after the calls of its statement,
like a package variable. The object of a member call, with its member, is evaluated before its
arguments, as C++ does.

**C++'s rules for C values.** Signed integers wrap on overflow (gocpp compiles with `-fwrapv`), but
an integer division by zero and a shift by a negative count or by the width or more are C++'s
undefined behavior, not Go panics. `^x` of a C `unsigned char` gives an `int` (C++'s promotions).

**Builtins.** `len`, `cap`, `min` and `max` don't take C/C++ values: call the C++ functions
(`v.size()`, `std.size(v)`, `std.max(a, b)`). A gocpp slice, array or map of C/C++ values
(`var xs []C.int`) is a gocpp value: `len`, `cap`, `append`, `copy`, `clear`, `delete`, its
index (with Go's bounds checks), slicing (`xs[1:]` shares the array), `range` (the elements are
copies) and `v, ok := m[k]` work as for any other.

**Constants.** A constant can't be made from a C/C++ variable or call: `const d = x + 1` with
`var x C.int` is "x + 1 (C/C++ value) is not constant", as in Go. (C macros in constants come
with Phase 12.)

**gocpp values inside C++ values.** A gocpp value that comes back through C++ (a field of an
element of a `std.vector[*Node]`: `nodes[0].N`) has no gocpp type for the compiler: it is a
C/C++ value. Convert it (`int(nodes[0].N)`); strings (`string(x)`) come with step 4.9.

A C/C++ value doesn't convert implicitly to a gocpp type, as in cgo: `var n int = cmath.pow(10, 30)`
is an error ("cannot use cmath.pow(10, 30) (C/C++ value) as int value in variable declaration:
convert it, int(cmath.pow(10, 30))"), also in an assignment, an argument and a return. `int(x)` converts with Go's
rules (`gp::cgo::as`: a float out of range gives Go's result, not C++'s undefined behavior). A C
pointer converts to a number only through `unsafe.Pointer`, and only a bool converts to bool.
The other way round too: a typed gocpp value doesn't go implicitly into a C/C++ variable, field,
parameter of a gocpp function or result (`fv := 1e30; var i C.int = fv` is "cannot use fv
(variable of type float64) as C.int value in variable declaration: convert it, C.int(fv)"); an
untyped constant does (it is a C literal there). An argument of a C/C++ function is different:
gocpp doesn't know the parameter's type, so C++'s rules convert it (a gocpp float passed to a C
integer parameter out of its range is undefined behavior: write `C.int(x)`). Phase 12 (typed
interop) knows the parameter types and makes this exact.

### Untyped constants in C/C++ calls

```go
cstdio.printf("%d\n", 5) // 5 is a C int, as printf expects
m := std.max(n, int(0))  // std.max(n, 0) would not compile: int64_t vs int
```

An untyped constant passed to C/C++, or used next to a C/C++ value (an operand, an index, the
value of `m["k"] = 1` or `s += "!"`), is written as a plain C literal with C's default type (`5`
is an `int`, `2.0` is a `double`, `"x.txt"` is a string literal). That is what C APIs expect. A
C++ template that deduces one type from two arguments then needs a typed constant: write
`std.max(n, int(0))`, not `std.max(n, 0)`.

Not yet (step 4.9 and Phase 12): such constants are not checked against Go's rules for the C/C++
type, as cgo checks them: `var s *C.char = "hi"`, `var c C.char = 200` and `C.int(2.5)` are
accepted, and the C++ compiler converts (or truncates) them.

### C/C++ objects in closures and `defer`

```go
v := std.vector[int]{}
defer v.clear() // fine: v is boxed, so it is still alive when the defer runs
```

- A C/C++ object (a local with a C/C++ type) that a closure or a deferred call uses is **boxed**
  (`std::make_shared<T>`), and any member call or operator on it counts as a change. Without
  this, `defer v.clear()` would call `clear()` on a vector that was already destroyed.
- C++ destructors of locals run when their block ends, which is before the function's defers
  run. That only matters for C/C++ objects, and boxing handles the `defer` case.

### Types that can't be copied

C++ types that can't be copied or default-constructed (`std::unique_ptr`, `std::lock_guard`,
types without a default constructor) follow a policy table that says which gocpp constructs copy,
move, default-construct or assign them. Results go through `gp::result_slot<R>`, which picks the
right storage when the C++ is compiled. Locals that are not boxed are moved where Go semantics
allow.

Source: [Phase 01 §2.8](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#28-the-opaque-pass),
[Phase 04 §2.5](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#25-values-and-operations-opaque).

## Strings at the boundary

```go
name := "world"
cstdio.printf("hello %s\n", C.cstr(name)) // const char*, valid for this statement
p := C.CString(name)                      // a malloc'ed copy that C may keep
defer C.free(p)
s := C.GoString(lib.get_name()) // char* → string: a real gocpp string
fmt.Println(strings.ToUpper(s + "!"))
```

| From → To | How |
|---|---|
| untyped string constant → any C/C++ parameter | a raw C literal `"hello"` |
| `string` → `std::string_view` | implicit conversion |
| `string` → `std::string` | explicit: `std.string(s)` |
| `string` → `const char*` | `C.cstr(s)`: a small **holder** object that owns a NUL-terminated copy and converts to `const char*`. It lives at least until the end of the statement. No copy is made when the bytes are already NUL-terminated (literals) |
| `string` → owned `char*` | `C.CString(s)` (cgo-compatible, uses `malloc`; free it with `C.free(p)`) |
| `char*` → `string` | `C.GoString(p)`, `C.GoStringN(p, n)` (cgo-compatible) |
| `std::string` or `std::string_view` → `string` | a conversion: `string(x)` |
| bytes | `C.GoBytes(p, n)`, or `unsafe.Slice((*byte)(p), n)` |

- Only `std::string_view` is implicit. Implicit conversions to both `std::string` and
  `std::string_view` would be ambiguous for overloaded C++ functions and would not work with
  templates.
- **The helpers have real types.** `C.CString`, `C.GoString`, `C.GoStringN` and `C.GoBytes` are
  rewritten into calls of a hidden gocpp package with real declarations, so `s := C.GoString(p)`
  gives `s` the type `string`, and `s + "!"` and `strings.ToUpper(s)` are fully type-checked.
  `C.cstr` stays opaque, because its holder must stay a C++ object.
- Typed interop (Phase 12) will convert a `string` passed to a `const char*` parameter
  automatically, except for functions that keep the pointer ([below](#what-typed-interop-will-add)).

Source: [Phase 04 §2.6](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#26-strings-at-the-boundary).

## Pointers, unsafe and slices at the boundary

```go
var st stat.struct_stat
stat.stat(C.cstr(path), &st) // &st: a raw C++ address of a C++ value

buf := make([]byte, 4096)
n := unistd.read(fd, unsafe.Pointer(&buf[0]), 4096) // C only borrows the pointer

nums := []C.int{3, 1, 2}
lib.print_all(nums) // a C++ function that takes std::span<int>
```

- **`unsafe` keeps gocpp memory alive, like Go.** `unsafe.Pointer` is a `gp::ptr<void>`. Made
  from a gocpp pointer, it shares that object's owner, and it converts to `void*` for C.
  `unsafe.SliceData(s)` returns a pointer that shares the slice's array, and
  `unsafe.String(p, n)` and `unsafe.Slice(p, n)` keep `p`'s owner. `unsafe.Add` works on both
  kinds of pointers.
- Only pointers that come **from C** are views that own nothing.
- `[]T` converts implicitly to `std::span<T>`.
- A gocpp `*T` passed where C/C++ expects `T*` converts implicitly, and is only **borrowed**. C
  must not keep it after the gocpp object dies. This is the same rule as cgo's pointer-passing
  rules.
- `&x`, where `x` has a C/C++ type, is a raw C++ address. **C/C++ values follow C++ lifetime
  rules.** gocpp's safety covers gocpp types, not C/C++ types. A local ends with its block (not
  when nothing points to it any more, as in Go), so:
  - `return &x` of a local is an error;
  - `&x` of a local in a `defer` or `go` statement, and of a loop variable (Go gives each
    iteration its own), are not supported yet (step 4.8 keeps such a local alive);
  - every other pointer to a local is the program's to keep valid: `g = &x`, `h.p = &x` and
    `p := &x; return p` compile, and dangle after `x`'s block ends (ASan finds it). A C function
    must not keep `&x` after that either (a callback's user data, `sqlite3_bind_text` with
    `SQLITE_STATIC`).

  `&lib.global` is the address of a C/C++ global, which lives as long as the program. Today `&x`
  takes only a variable or a global; `&v[i]` and `&s.f` come with step 4.10.

Source: [Phase 04 §2.7](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#27-pointers-unsafe-and-slices-at-the-boundary).

## Callbacks and panics

```go
func onEvent(code C.int) { // a top-level func: C can call it through a function pointer
	fmt.Println("event", code)
}

func main() {
	lib.set_callback(onEvent)
	lib.run()
}
```

| C/C++ expects | Pass |
|---|---|
| a C function pointer `void (*)(int)` | a top-level gocpp func: `func cb(x C.int)` |
| `std::function<…>` or a template callable | any gocpp func value or closure (`gp::func` converts) |
| `void* userdata` plus a callback | `cgo.NewHandle(v)`, and pass the handle; in the callback, `cgo.Handle(p).Value()` (Go's `runtime/cgo.Handle` API) |

- **Panics never unwind through C code.** GCC and Clang emit unwind tables for C on x86-64 Linux,
  so a panic inside a C callback would unwind through the C frames and silently skip the C
  library's own cleanup (locks, memory, sqlite state). So every gocpp function that C can call
  is called through a `noexcept` **trampoline**. It catches any panic, prints it Go-style and
  exits with code 2.
- **C++ exceptions.** In Phase 04, a C++ exception that nothing catches ends the program like an
  unrecovered Go panic, with exit code 2 (see [C++ exceptions](#c-exceptions) below). From
  Phase 05 on, it becomes a normal panic that `recover()` can catch; its value has the type
  `gp::CppException`, and `Error()` returns `what()`.
- **Calling gocpp from C.** Functions marked `//export Name` become `extern "C"` functions in the
  `c-archive` and `c-shared` build modes, with cgo's rules for the allowed types. Every export
  goes through a trampoline too ([Phase 06 §2.7](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#27-build-modes--buildmode-like-go)).

### C++ exceptions

```go
func parse(s string) int {
	return int(std.stoi(std.string(s))) // throws std::invalid_argument for "x"
}

func main() {
	defer fmt.Println("deferred") // not run in Phase 04
	fmt.Println(parse("x"))
}
```

```
panic: C++ exception std::invalid_argument: stoi

goroutine 1 [running]:
main.parse(...)
	/home/me/prog/main.gp:2 +0x44
main.main()
	/home/me/prog/main.gp:7 +0x6b
exit status 2
```

- **The message.** A `std::exception` prints `panic: C++ exception <type>: <what()>` (only
  `<type>` when `what()` is empty); anything else prints its type: `throw 42` gives
  `panic: C++ exception of type int`. The type is the one the exception was thrown with, its C++
  name (`std::out_of_range`, not the `std::exception` it may be caught as), also in a program
  built with `-fno-rtti`. A newline in `what()` is followed by a tab, as in Go's panic messages.
- **The trace** is the stack where the exception was thrown, innermost first, with the frames of
  C++ functions that are in the program (a helper in a header shows as `vec_at(std::vector<long,
  std::allocator<long> > const&, unsigned long)` and its header line); the C++ library's own
  frames are left out, as Go leaves out its runtime's. If C++ code caught the exception and threw
  it again (`catch (...) { throw; }`), or it reached a `noexcept` function, the trace starts
  there.
- **Nothing runs on the way out in Phase 04:** no deferred call, no C++ destructor, and
  `recover()` can't stop it. The C++ library looks for a `catch` before it unwinds any frame;
  gocpp's code catches only its own panics, so the library finds none and ends the program on the
  stack of the throw (std::terminate, where gocpp prints the report). Go would run the deferred
  calls of a panic; Phase 05 turns the exception into a Go panic where it enters gocpp code, and
  then they run ([D26](language.md#d26-c-exceptions)).
- **A deferred call that throws while a Go panic runs** prints both, like a panic in a deferred
  call: `panic: boom`, then `	panic: C++ exception std::runtime_error: ...`, and the trace shows
  the deferred call on top of the panicking function.
- **A `std::bad_alloc`** that C++ code throws itself (or `std::bad_array_new_length`: `new
  int[n]` with a negative `n`), which the runtime's new handler doesn't see, prints Go's `fatal
  error: runtime: out of memory`, as gocpp's own allocations do, with the trace where it was
  thrown.
- **A Go panic that reaches a `noexcept` C++ function** (C++ calls `std::terminate` there) prints
  the panic's normal report. A C++ destructor that throws (one declared `noexcept(false)`) while
  a Go panic unwinds its frame makes C++ end the program with the new exception: the report shows
  only the C++ exception, and the Go panic is lost (Phase 05 can chain them). Without any exception, `std::terminate` (a `std::thread` destroyed
  while it runs) keeps the C++ library's message and aborts.
- **C files and `-fno-exceptions`.** C sources get no `-fexceptions`, and a package's
  `-fno-exceptions` reaches its own `.cpp` files (not the generated code, which always has
  `-fexceptions`). An exception that the C++ library throws still passes through such frames and
  ends the program the same way (the search doesn't need their handlers; a frame without unwind
  tables ends the search, which also ends the program there).
- An exception that C++ code catches itself is C++'s business: gocpp doesn't see it.

Source: [Phase 04 §2.8](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#28-callbacks-and-panics-crossing-c-code),
[Phase 05 §2.7](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#27-panic--recover--defer-full-version).

## C/C++ values in interfaces and fmt

```go
s := std.string("hi")
var x any = s   // a C++ value in an interface
fmt.Println(x)  // printed with std::formatter, or with operator<< if there is none
fmt.Println(&s) // a raw pointer prints as an address
```

- **Printing.** `fmt` prints a C++ value through `std::formatter<T>` when it exists, otherwise
  through `operator<<`. **Raw pointers, `char*` included, print as addresses**, as Go prints
  pointers. (Printing a `char*` as a C string would crash, or silence stdout, when it is null.)
  The details are in [Printing C/C++ values](#printing-cc-values) below.
- **In an `any`.** A C/C++ value stored in an interface gets a *foreign* type descriptor, named
  after its C++ spelling. Equality, hashing and printing use whatever the C++ type supports
  (`operator==`, `std::hash`, `std::formatter` or `operator<<`) ([Phase 05 §2.2](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#22-type-descriptors-chosen-by-the-go-type)).
- **Interface methods with C/C++ types.** `go/types` treats every C type as invalid, and two
  invalid types compare equal. So gocpp checks method signatures with C/C++ types itself: a
  `static_assert` at each static conversion, and a comparison by source spelling at run time
  ([Phase 05 §2.3](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#23-interface-values-and-itabs)).

### Printing C/C++ values

```go
var n C.int = 7
p := geo.make_point(1, 2)          // a C++ struct with an operator<<
fmt.Println(n, C.char('a'), geo.RED, std.string("bob"), p, geo.null_name())
// 7 97 5 bob (1, 2) <nil>
fmt.Printf("%d|%q|%5v\n", n, std.string("x"), p) // 7|"x"|(1, 2)
println(n, geo.RED, std.string("bob"))           // 7 5 bob (to stderr)
```

`fmt.Print`, `Println`, `Printf` and the `Sprint` functions print a C/C++ value by its C++ type,
also inside a gocpp value (a struct field, a slice element):

| C/C++ value | Prints as |
|---|---|
| a number or a `bool` (`C.int`, `C.double`, `C.size_t`, …) | the Go type of the same kind and size: `C.int` as `int32`, `C.char` as `int8` (a number, as in cgo), `C.double` as `float64`, with all of fmt's verbs |
| an enum | its number |
| a raw pointer (`T*`, `char*`, `void*`, a function pointer) | its address, as Go prints a pointer that isn't `&{...}`: `0x7ffd5e8c`, `<nil>` for null, `%d` in decimal. A `char*` is never read as a string (it may be null, or point anywhere): `C.GoString(p)` gives the text |
| `nullptr_t` | `<nil>`, as Go's nil interface |
| `std::string`, `std::string_view` | a Go string (`%v %s %q`, and `fmt.Print` puts no space next to it) |
| any other class type | its text: `std::formatter<T>` when the program has one for it, else its `operator<<` (declared in the type's namespace, or in the global one for a type there: C++ finds it by argument-dependent lookup). Like a Go value with a `String` method, `%v %s %q` print the text, the other verbs Go's error form with it: `%!d(Point=(1, 2))` |
| a type with neither (a C struct) | a compile error at the `.gp` line: `fmt can't print a C++ type in this value: it has no std::formatter and no operator<<`. Print its fields, or declare an `operator<<` in a helper header |

- **`%T` and the error forms** name a C number by the Go type above (`int32`; `int` for a `long`,
  which C++ can't tell from gocpp's `int`, and `int64` for a `long long`), and any other C/C++
  type by its C++ name (`Point`, `char const*`, `std::string`, `std::vector<long,
  std::allocator<long> >`); Phase 13 translates C++ names back to gocpp names. cgo's `%T` prints
  `main._Ctype_int`.
- **`std::formatter` costs nothing when unused.** gocpp looks for one only when the program's
  headers bring in `<format>` (a type can have one only then): the generated code includes
  `gocpp/fmtcxx.hpp` after the headers, which uses `<format>` only if it is there already.
- **The builtin `print` and `println`** print C/C++ numbers, bools, enums, raw pointers (`0x0`
  for null), `nullptr` and C++ strings in their own format, and reject other C++ objects at the
  `.gp` line (as Go's `println` rejects structs): use `fmt.Println`.
- **C's stdio and fmt.** C's `stdout` has a buffer when it is not a terminal (a pipe, a file),
  and gocpp's `fmt` writes without one, as Go does. So `fmt.Print*`, `print` and `println` flush
  C's `stdout` and `stderr` (`fflush`) before they write: a `printf` that comes before a
  `fmt.Println` in the program comes out before it, also through a pipe. That costs about 3 ns a
  stream when the buffer is empty, against at least 50 ns for the write itself. What `printf`
  writes after the last `fmt` call is flushed at the end of the program.
- A value that can't be copied (`std::unique_ptr`) can't be printed, as `fmt` takes its
  arguments by value; print what it points to.

Source: [Phase 04 §2.12](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#212-runtime-helpers-added-in-this-phase).

## Building C and C++ sources

```go
//gocpp:pkg-config sdl2
//gocpp:link m pthread
//gocpp:cflags -DUSE_FAST_MATH
//gocpp:ldflags -L/opt/foo/lib
//gocpp:extern-c ./legacy.h
package game
```

```text
// gocpp.mod (go.mod syntax), module-wide settings
module github.com/huuhait/game

gocpp 0.1                    // minimum gocpp version
cxx c++20                    // C++ standard for generated + user C++ files

cflags -DGAME_DEBUG
pkg-config sdl2
```

**Sources.** The C and C++ files in the package directory are compiled and linked, like cgo does:
`.c` with the C compiler and `-std=gnu17`, and `.cc`, `.cpp`, `.cxx` with the C++ compiler and
`-std=c++20`, using the package's flags ([C and C++ sources in a
package](#c-and-c-sources-in-a-package)).

**Directives** are line comments before the `package` clause, like `//go:build`.

| Directive | Arguments | Effect |
|---|---|---|
| `//gocpp:pkg-config sdl2` | one or more package names | runs `pkg-config --cflags -- sdl2` and `pkg-config --libs -- sdl2` and adds the flags they print, if they are allowed |
| `//gocpp:link m pthread` | one or more library names | adds `-lm -lpthread` |
| `//gocpp:cflags ...`, `//gocpp:ldflags ...` | one or more flags | adds the flags, if they are allowed |
| `//gocpp:extern-c <hdr>` | one header | wraps that header in `extern "C"` (for C libraries whose headers lack the guard, such as `<lua.h>`) |
| `//gocpp:no-extern-c <hdr>` | one header | turns the `extern "C"` wrapping off for that one header, whatever the defaults below say |
| `//gocpp:no-win32-defaults` | none | Windows: gocpp does not define `WIN32_LEAN_AND_MEAN` and `NOMINMAX` ([Windows headers](#windows-headers), Phase 14) |

The rules for directives:
- A directive starts with exactly `//gocpp:`, with no space after `//`. `// gocpp:link m` is an
  ordinary comment, and so is every `/* */` comment. But before the `package` clause, a comment
  like `// gocpp:link m` gets a warning, because it looks like a broken directive. To turn a
  directive off, comment it out: `// //gocpp:link m`.
- Directives go before the `package` clause: in the package doc comment or in a comment of their
  own. A `//gocpp:` line after the `package` clause is an error. In a doc comment, `gocpp fmt`
  (like gofmt) moves the directives to its end. It keeps only well-formed directives as they
  are: `//gocpp:Link m` would become the ordinary comment `// gocpp:Link m`. So `gocpp fmt`
  leaves a file with directive errors unchanged, prints the errors and exits with status 2, as
  gofmt does for a file with a syntax error.
- The arguments follow the rules of cgo's `#cgo` lines. Spaces and tabs separate them. Quotes
  (`"` or `'`) keep spaces inside an argument. gocpp removes the quotes, also in the middle of an
  argument: `-DMSG="hello world"` is one argument, `-DMSG=hello world`. A missing closing quote
  and an empty argument (`""`) are errors.
- A backslash makes the next character an ordinary one: `\"` is a quote and `\\` is a
  backslash. So write Windows paths with `/`. A backslash at the end of the line is an error.
- An argument that starts with `//` starts a comment, up to the end of the line:
  `//gocpp:link m // the math library`. A quoted `"//"` is an ordinary argument.
- An unknown name is an error. When a known name is close, the error says which one:
  `unknown //gocpp: directive "lnk"; did you mean //gocpp:link?`. A wrong number of arguments
  is an error too.
- A pkg-config package name or a library name has only the characters `A-Z a-z 0-9 _ + - .`, and
  it doesn't start with `-`: `//gocpp:link stdc++ boost_system-mt`. Flags go into `cflags` and
  `ldflags`.
- Flags that can run code at build time, such as `-fplugin=…`, `-B` or `@file`, are refused, as
  in Go ([Flags](#flags-and-the-allow-list)).
- A header is written like its import path, without the Go quotes: `<lua.h>`, or a path that
  ends in `.h`, `.hh`, `.hpp`, `.hxx`, `.h++` or `.inl`, such as `./legacy.h`.
- A directive can be repeated, and the repeats add up: two `//gocpp:link` lines link the
  libraries of both.

### C and C++ sources in a package

The C and C++ files in the folder of a package are part of the package, as in cgo: `gocpp build`
and `gocpp run` compile them and link them into the program. A `.gp` file calls them through a
header next to them ([Small helpers](#small-helpers-a-header-next-to-the-gp-file)).

**Which files.**
- `.c` files are C. `.cc`, `.cpp` and `.cxx` files are C++. These are cgo's extensions: `.c++`
  and `.C` files are not compiled, and neither are headers (`.h`, `.hpp`, …).
- Files whose names start with `_` or `.` are left out, as for `.gp` files.
- The build constraints of `.gp` files apply: a suffix of the file name (`io_linux.c`,
  `gl_windows.cpp`, `simd_amd64.c`), and a `//go:build` line in the comments at the top of the
  file, before any code. A broken `//go:build` line is an error at the file.

  ```c
  // A helper for Windows only.
  //go:build windows

  #include <windows.h>
  ```
- Only a package folder has them: `gocpp run main.gp` builds only `main.gp`, like
  `go run main.go`. Use `gocpp run .` for a package with C or C++ files.
- Until Phase 06 a program is one package, so these are the files of the main package.
- Assembly (`.s`, `.S`, `.sx`), Objective-C (`.m`), Fortran (`.f`, `.F`, `.for`, `.f90`), SWIG
  (`.swig`, `.swigcxx`) and `.syso` files, which cgo also builds or links, are an error: gocpp
  doesn't build them, and the program would miss their code.

  ```text
  ./start.S: assembly files (.S) are not supported: gocpp compiles only the .c, .cc, .cpp and .cxx files of a package
  ```

**Which compiler.** The C++ files are compiled with the C++ compiler of the build (`GOCPP_CXX`,
else `CXX`, else `clang++`, else `g++`). The C files are compiled with the compiler in
`GOCPP_CC`, which, like `GOCPP_CXX`, can hold arguments after the program. Without it, gocpp
takes the C compiler of the same toolchain as the C++ compiler, from the same folder, else from
`PATH`: `gcc` for `g++`, `clang` for `clang++`, `clang-18` for `clang++-18`,
`x86_64-w64-mingw32-gcc` for `x86_64-w64-mingw32-g++`, and `ccache gcc` for `ccache g++`. The
other arguments in `GOCPP_CXX` go to the C compiler too, so a C++-only one (`GOCPP_CXX="g++
-fno-rtti"`) makes gcc warn at each `.c` file: set `GOCPP_CC` then. `CC` is not used: a Nix shell
sets `CC=gcc` also when you build with `clang++`. Mixing two compilers in one program is not
supported: gocpp doesn't check that `GOCPP_CC` is of the same family as the C++ compiler, and only
warns about it with `-asan`. A C compiler is only looked for when the package has `.c` files.

**Which flags.** Each compile gets, in this order:

| Flags | Why |
|---|---|
| `-g -O0`, and for clang `-Qunused-arguments` | a debug build, like the generated C++; the `-O` is always explicit (a hardened Nix compiler wrapper turns a missing `-O` into `-O2`) |
| with `-asan`: `-fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=all` | the sanitizers see the bugs of your C and C++ code too |
| `-std=gnu17` (C) or `-std=c++20` (C++) | the language: the same C with both compilers, with the POSIX and GNU names of glibc (`strdup`, `fileno`, `M_PI`), as C++ has them; not the newest C (gcc 15's default is `gnu23`, where `bool` and `true` are keywords and `f()` means `f(void)`, which breaks old C code) |
| `-iquote <package folder>` | `#include "x.h"` finds the headers next to the file, and no file there can hide a system header like `<vector>` |
| `-include <runtime>/include/gocpp/nofortify.h` | a hardened Nix compiler wrapper adds `-D_FORTIFY_SOURCE` after all flags, which warns at `-O0`; the header turns it off when nothing is optimized |
| `GOCPP_CFLAGS`, and for C++ `GOCPP_CXXFLAGS` after it | the trusted flags of the environment |
| the package's compile flags | `//gocpp:cflags` and `pkg-config --cflags`, after the allow-list |
| `-c -x c` or `-c -x c++`, the file, `-o` the object file | |

Your flags come after gocpp's, so they win: a package can build its own files with `-O2`, `-Wall
-Werror` or `-std=gnu11`. A package's compile flags go to both languages, though (and to the
generated C++, where gocpp's language flags win), so a `-std=` for one language breaks the files of
the other; there is no variable for C only yet. The flags that gocpp's generated C++ gets are not
for your code: not the language flags (`-fwrapv`, `-fno-strict-aliasing`, `-ffp-contract=off`,
which give Go's semantics, not C's), not `-Wno-error`, and not the warning flags of generated
C++. `-nochecks` changes nothing here.

**Errors.** An error in your own file is your error, not gocpp's: `gocpp build` prints the
compiler's output under the package's line, with the paths as `gocpp check` writes them, and exits
with status 1. The warnings of files that compile are shown the same way, and the build goes on.

```text
# command-line-arguments
./bad.c:2:9: error: 'missing' undeclared (first use in this function)
```

A program that doesn't link while the package has C or C++ files (a function that no file
defines, a second `main`) is reported as the program's link error too, with the list of the files,
and exit status 1 (a function that a header declares and nothing defines is reported at the `.gp`
line that calls it: see [Errors in interop code](#errors-in-interop-code)). `gocpp build -x` shows every compile and the link.

**Objects and the link.** The object files go into the work folder as `$WORK/cobj/<file>.o`
(`a.c.o` and `a.cpp.o`, so a C file and a C++ file can have the same base name). The files
compile at the same time, up to 8, while the generated C++ compiles. The link gets them after the
generated object file and before the runtime's object files and the libraries. Phase 06 keeps them
in the build cache and gives `-p`.

### Flags and the allow-list

The flags of a package come from its directives:

- `//gocpp:cflags` gives compile flags and `//gocpp:ldflags` link flags, as written. A relative
  path is made absolute from the package folder, because the compiler doesn't run there: the
  path of `-I`, `-F`, `-isystem` and `-L` (joined, as in `-Iinc`, or the next argument), a file
  for the linker (`libfoo.a`, `./x.o`), and the file of `-include` when the package folder has
  it (else it is looked up through the include path). `-I=dir` and `-I $SYSROOT/dir` are
  relative to the sysroot and stay.
- `//gocpp:link m pthread` gives the link flags `-lm -lpthread`.
- `//gocpp:pkg-config sdl2 x11` runs `pkg-config --cflags -- sdl2 x11` (compile flags) and
  `pkg-config --libs -- sdl2 x11` (link flags) in the package folder, once per line, and splits
  the output like a shell would. The program is the first word of `PKG_CONFIG`, else
  `pkg-config`; it reads its own environment (`PKG_CONFIG_PATH`, `PKG_CONFIG_LIBDIR`, …).
  `--static` comes with static builds (Phase 06).

The flags add up in source order: the lines of a file from top to bottom, and the files of the
package in the order of their names. A package's flags apply to the whole package. The compile of
the package's C++ gets its compile flags, and the namespace probe of `gocpp check` and of the
editor gets them too. The link gets the link flags after the object files, so that a static
library comes after the code that uses it.

**gocpp's own flags win.** In the compile of the generated C++ and in the probes, gocpp's
language flags come again after the flags of the environment and of the package, and `-x c++`
comes right before the source: `-std=c++20`, `-fwrapv`, `-fno-strict-aliasing`,
`-ffp-contract=off`, `-fexceptions` and `-fno-fast-math`. The generated C++ relies on them (Go's
wrapping integers, its exact floating-point results, panics as C++ exceptions), and for each of
them the last one wins with g++ and clang++. So `-std=c++17`, `-ffast-math`, `-fstrict-aliasing`
or `-fno-exceptions` in a package's flags change nothing there (a package's own C and C++ files get
its flags after gocpp's, see [C and C++ sources in a package](#c-and-c-sources-in-a-package)). A package can still choose `-O…` and `-g…`
(`-Ofast` acts as `-O3` then: its fast math is turned off again, and clang notes that). A
`-Wno-error` comes after them too, so a package's `-Werror -Wshadow` doesn't fail valid Go code
(`i := i` in a loop), while gocpp's own `-Werror=…` checks stay errors. When `GOCPP_CFLAGS` or
`GOCPP_CXXFLAGS` are set, the runtime's compiles get the same language flags, `-Wno-error` and
`-x c++` after them.

**The allow-list.** Every flag, also those that pkg-config prints, is checked against the go
command's list of allowed flags (`cmd/go/internal/work/security.go`, BSD-licensed, ported
with the changes below), before any compiler, probe or linker runs: `gocpp check` and the editor run probes
as soon as a folder is opened, so a cloned repository must not be able to run code through its
flags. The list has the usual compile flags (`-D`, `-U`, `-I`, `-O`, `-W…`, `-f(no-)rtti`,
`-f(no-)exceptions`, `-std=`, `-fsanitize=`, `-include`, `-isystem`, …) and link
flags (`-l`, `-L`, `-pthread`, `-Wl,-rpath,…`, `-Wl,--as-needed`, library files such as
`libfoo.a`, …). Flags that run code are refused: `-fplugin=`, `-fpass-plugin=`, `-specs=`,
`-wrapper`, `@file`, `-Wl,-plugin`, `-Xclang`.

gocpp also refuses, in a package's flags, the choices for the whole program, with the reason in
the message. They belong to the user, in `GOCPP_CFLAGS`, `GOCPP_CXXFLAGS` and `GOCPP_LDFLAGS`
(the flags of the environment stay trusted):

- the system root and the toolchain: `--sysroot`, `--sysroot=`, `-isysroot`, `--gcc-toolchain=`,
  `--gcc-install-dir=`, `--prefix`, `-B`;
- the linker: `-fuse-ld=`, `--ld-path=`;
- the target and the ABI, as the package's code must link with and match gocpp's runtime: `-m32`, `-m64`, `-target`, `-arch`, `-mabi=`, `-mfloat-abi=`,
  `-msoft-float` (and `-msingle-float`, `-mdouble-float`), `-m(no-)ms-bitfields`;
- `-stdlib=`, which selects another C++ standard library than the runtime's;
- `-D` and `-U` of the macros that change the ABI of the C++ standard library:
  `_GLIBCXX_DEBUG` (and `_GLIBCXX_DEBUG_PEDANTIC`), `_GLIBCXX_USE_CXX11_ABI`, `_LIBCPP_ABI_…` and
  `_LIBCPP_DEBUG`, also through `-Wp,-D…`. `_GLIBCXX_ASSERTIONS` only adds checks and is allowed,
  and so is every other macro (`-D_REENTRANT`, `-D_FILE_OFFSET_BITS=64`).

A refused flag is an error at its line, like Go's:

```text
./main.gp:1:1: invalid flag in //gocpp:cflags: -fplugin=./x.so
./main.gp:2:1: invalid flag in pkg-config --cflags: -fplugin=/evil.so
./main.gp:3:1: invalid flag in //gocpp:cflags: -I without argument
./main.gp:4:1: invalid flag in //gocpp:cflags: -m32 (it changes the ABI, and the package's code would no longer match gocpp's runtime; $GOCPP_CFLAGS_ALLOW can allow it)
```

and none of that line's flags are used. A pkg-config package that isn't installed is an error at
its line too, with pkg-config's message. These are package errors: `gocpp build` stops before it
compiles anything. When the generated C++ doesn't compile or the program doesn't link with the
flags of the package or of the environment, `gocpp build` prints the compiler's errors and the
flags, and exits with status 1 (without the flags, that would be a bug in gocpp). `gocpp build
-x` prints the pkg-config runs, like `go build -x`.

These variables change the list, like cgo's `CGO_CFLAGS_ALLOW` and friends. Each is a regular
expression (Go syntax) that must match a whole flag:

| Variable | Effect |
|---|---|
| `GOCPP_CFLAGS_ALLOW` | allows more compile flags (`cflags`, `pkg-config --cflags`) |
| `GOCPP_CFLAGS_DISALLOW` | refuses compile flags, also allowed ones; it wins over `_ALLOW` |
| `GOCPP_LDFLAGS_ALLOW` | allows more link flags (`ldflags`, `link`, `pkg-config --libs`) |
| `GOCPP_LDFLAGS_DISALLOW` | refuses link flags, also allowed ones |

Unlike cgo, gocpp doesn't also check each argument for "unsafe" characters (go/build's
`safeCgoName`, which makes `-DNAME=\"x\"` a "malformed #cgo argument" in Go 1.25): gocpp never
passes flags through a shell, only as arguments of a program, and the allow-list already refuses
what a compiler or linker would read as a flag or a response file of its own. So
`//gocpp:cflags -DMSG=\"hi\"` defines a string macro, which cgo can't.

**Flags from the environment** are trusted, as cgo's `CGO_CFLAGS` are: the user chose them, so
they are not checked. They are split like `CC` is (spaces separate, quotes keep spaces, no
escapes):

| Variable | Goes to |
|---|---|
| `GOCPP_CFLAGS` | every C and C++ compile, the runtime's too, and the probes |
| `GOCPP_CXXFLAGS` | every C++ compile, the runtime's too, and the probes |
| `GOCPP_LDFLAGS` | the link, after the package's link flags |

`GOCPP_CFLAGS` goes to the C++ compiles too, unlike cgo's `CGO_CFLAGS` (Go uses `CGO_CPPFLAGS`
for flags of both languages): it is like `//gocpp:cflags`, which is for both. So it is for flags
such as `-I`, `-D` or `-O2`, not `-std=gnu11`; a flag for one language goes into `GOCPP_CXXFLAGS`
(C++ only; there is no variable for C only yet).

They come before the package's own compile flags, so their `-I` folders are searched first. As
they also go to the runtime, they are part of the key of the runtime's cached object files: a flag
such as `-stdlib=libc++` must be the same for every part of a program. The compiler itself reads
`CPATH`, `CPLUS_INCLUDE_PATH` and `C_INCLUDE_PATH` (and a Nix compiler wrapper
`NIX_CFLAGS_COMPILE`), in the probes and the builds alike.

**Module-wide settings** go into `gocpp.mod` (`cflags`, `pkg-config`, …). Packages add their own
with directives.

**Public flags.** Flags that change how a package's headers parse (`-I`, `-D`) are passed to
every package that depends on it, so no two translation units see a C struct with different
layouts ([Phase 06 §2.3](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#23-header-layout-c-needs-declarations-before-use-go-doesnt)).

**The `extern "C"` rule** (Phase 04). A header needs `extern "C"` when it declares C functions
without its own `__cplusplus` guard: without it, C++ gives the functions C++ linkage, and the
program looks for C++-mangled names (`calc_add(int, int)`) that the C code doesn't define, so
the link fails. gocpp includes such a header inside a block of its own:

```cpp
extern "C" {
#include "./calc.h"
}
```

Which headers get the block:
- a **local `.h`** header gets it when it **is C**: the C compiler parses it as C (`$CC
  -std=gnu17 -fsyntax-only -x c` over a file that only includes it, with `-iquote` and the
  package folder, `GOCPP_CFLAGS` and the package's flags, as for the package's `.c` files), and
  its own text has no `__cplusplus`. A header that doesn't parse as C (a class, a namespace, a
  template, `#include <vector>`) is a C++ header. The C compiler is the one of the package's
  `.c` files; without one (and without `GOCPP_CC`), the C++ compiler in C mode (`-x c`). The
  check runs once per header and is cached (in memory, per compiler, folder, flags and header):
  it runs again when the header changes, or a local header that it includes, or when the first
  header that it didn't find appears in a folder where the compiler would find it: next to the
  header that includes it, in the package folder, or in an `-I`, `-iquote`, `-isystem` or
  `-idirafter` folder of the flags (not in the compiler's own system folders). `gocpp check` and the language server decide the same way; the
  language server runs the check in the background, and until it answers, the header is not
  wrapped;
- `.hpp`, `.hh`, `.hxx`, `.h++` and `.inl` headers are C++ headers: no block by default;
- system headers (`<...>`) get no block by default: C libraries have the guard almost always;
- `//gocpp:extern-c <hdr>` gives the header the block, whatever these defaults say: a system
  header without the guard (`<lua.h>`), or a C header with an odd name (`./legacy.hpp`). A header
  that has the guard can be wrapped too: an `extern "C"` block inside another one is fine;
- `//gocpp:no-extern-c <hdr>` takes the block away from a header, whatever the defaults say:
  for a `.h` header that parses as C but is meant as C++ (its functions are defined in a `.cpp`
  file, with C++ linkage);
- `extern-c` and `no-extern-c` on the same header is an error. So is an `extern-c` or
  `no-extern-c` header that no file of the package imports, because a typo would otherwise do
  nothing. This also catches `.\legacy.h`, which becomes `.legacy.h` (the `\` escapes the `l`).
  The default rules apply to such a header.

A directive names the header as the import does: `<lua.h>`, or the path of a local header. The
path is cleaned first, so `./legacy.h`, `legacy.h` and `sub/../legacy.h` are the same header. The
directives of all the files of a package count for all its imports.

The guard test is simple on purpose: it looks for the word `__cplusplus` in the header's own
text. So a header whose guard is in a header that it includes (a `config.h` with a
`BEGIN_DECLS` macro) gets the block too, which is harmless (the blocks nest), unless the other
header has C++-only parts under `#ifdef __cplusplus`: then write `//gocpp:no-extern-c`. And a
`__cplusplus` in a comment counts as a guard: then write `//gocpp:extern-c` if the header needs
the block. A header that doesn't parse as C on its own (it needs another header included before
it) is taken as C++: write `//gocpp:extern-c` for it.

The compiler probes (the namespace, kind and macro probes) include the headers the same way, so
they see what the build sees: a C header that declares `puts` before `<stdio.h>` does is an error
in C++ without the block (C++ linkage first, then C linkage), and fine with it.

Wrapping a C++ header breaks it (`extern "C" { #include <vector> }` gives over a thousand errors
with g++), which is why the parse-as-C check matters.

**Linking.** Libraries from directives and from `gocpp.mod` are linked after the package's
objects, deduplicated, in a valid order for static libraries
([Phase 06 §2.6](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#26-linking)).

Sources: [Phase 01 §2.4](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#24-gocpp-directives-parsed-now-used-in-phases-04-and-06),
[Phase 04 §2.9](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#29-building-the-cc-side),
[Phase 06 §2.1](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#21-the-module-file-gocppmod).

## Small helpers: a header next to the .gp file

```cpp
// mathx.h
#pragma once
#include <cmath>
static inline double hypot3(double a, double b, double c) { return std::sqrt(a*a + b*b + c*c); }
```

```go
package main

import "./mathx.h" // package mathx

func main() { println(mathx.hypot3(1, 2, 2)) }
```

- gocpp has **no inline C/C++ preamble** in v1. A helper goes into a header (and, if needed, a
  `.cpp` file) in the package directory. That header is a package like any other, and the
  `.cpp` file is compiled with the package.
- This keeps one way of doing things, and the helper gets real editor support as a C++ file.
- C macros that are statements (`do { ... } while (0)`) work as calls in statement position;
  other strange macros can be wrapped in such a helper header.
- An inline preamble is on the backlog ([TODO](../../plans/2026-10-13-gocpp/TODO.md#ideas--backlog-not-scheduled)).

Source: [Phase 04 §2.10](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#210-small-cc-helpers-a-header-next-to-the-gp-file).

## Errors in interop code

```text
# command-line-arguments
./main.gp:14: C++: no member named 'push_bak' in 'std::vector<long>'
	nums.push_bak(4)
```

In opaque mode, the C++ compiler finds the mistakes in C/C++ code: a C function called with the
wrong arguments, a C++ template that doesn't accept your type. In a package that uses C or C++
(a header import, or `C`), these errors are **your** errors, not internal ones. `gocpp build` and
`gocpp run` print them like `gocpp check` prints its errors, under the package's line, and exit
with status 1:

- **The place** is the `.gp` line (through `#line`). `#line` carries no column, so the message
  shows the line only: `./main.gp:14:`. The source line follows, after a tab.
- **The message** is the compiler's first message for that line, after `C++: `, without the
  compiler's `error: ` and its colors, and with plain quotes (`'`, also where gcc writes `‘’`). The
  C++ type names stay as the compiler writes them (Phase 13 translates them).
- **Which errors are yours.** Nearly every generated line of C++ has the `#line` of its `.gp`
  line, so the line alone can't tell your error from a bug in gocpp. An error is yours only when
  it comes from a `.gp` line **with C or C++ code**: a line of an expression that uses `C` or a
  header package or namespace (`cstdio.printf`, `std.vector`, `C.int`), or that is a C/C++ value
  (a variable of a C type, a C call's result); the first line of the statement around such an
  expression; or a header import line. Then it is yours when it is at that line; or in a header
  (a system header, a library's, or one of your own), in a macro that the line uses (gcc's note
  "in expansion of macro"); or in a header whose template the line instantiated: the template
  backtrace (gcc's "required from here", clang's "in instantiation of ... requested here") leads
  to the line, and the error is reported there with the first message. That includes gocpp's
  interop helpers (`gocpp/interop.hpp`: `C.int(x)`, a `switch` on a C value, `gp::range`, ...);
  for a `static_assert` there, the message is the `static_assert`'s own text, which is written
  for you:

  ```text
  ./main.gp:12: C++: a switch on a C/C++ value needs a number, an enum or a pointer
  	case p:
  ```

  And, as the first error, an error in a header that a `.gp` import includes, outside any
  template: a header that doesn't compile as C++, reported at the import line.
- **Which errors are gocpp's.** When the first error is at a `.gp` line without C or C++ code
  (also through a template or a macro), in the generated lines that have no `.gp` line, or in
  gocpp's runtime outside `gocpp/interop.hpp` (also a header template that the runtime
  instantiated), the build stops with "internal error: generated C++ did not compile", the
  compiler's first errors, and the work folder (exit status 2), as in a package without C or C++.
  With compile flags of the package or of the environment, it says "did not compile with the
  flags below" instead, and exits with 1.
- **Trimming.** Only the first error of each `.gp` line is shown, and the notes and backtraces are
  left out. The errors after the first one that lead to no `.gp` line of their own (a follow-on in
  the same template, which clang reports without a backtrace) are left out too; when some of them
  are in gocpp's code, a last line says how many: "(and 2 more errors in gocpp's code:
  -cpperrors=full shows them)". A template
  backtrace that passes more `.gp` lines shows them after the source line (`./main.gp:30: from
  here`). The errors are sorted by place; after 10 comes "too many errors", like `go build`.
- **`-cpperrors=full`** (a flag of `gocpp build` and `gocpp run`) shows the compiler's whole output
  instead, under the package's line, with the package's folder written as `./`.
- **Link errors.** A function that a header declares but that nothing the program links defines
  (a library without its `//gocpp:link` or `//gocpp:pkg-config` line, a missing `.c` file) is your
  error too, at the `.gp` line that uses it (from the debug information; GNU ld, lld and mold are
  understood), with the symbol as the linker writes it (demangled) and a hint:

  ```text
  # command-line-arguments
  ./main.gp:15: C++: undefined reference to 'sqlite3_libversion_number'
  	fmt.Println(int(sqlite3.sqlite3_libversion_number()))
  	(the headers declare it, but the program links no definition: add the library with //gocpp:link or //gocpp:pkg-config, or the C or C++ file that defines it to the package)
  ```

  `gocpp run main.gp`, like `go run main.go`, builds only the named files, so the folder's C and
  C++ files are left out: then the hint says that `gocpp run .` builds them too. A missing symbol
  of gocpp's own (`gp::`, `gpkg_`) stays an internal error, and a use that the linker can't place
  at a `.gp` line gets the linker's own first lines (see [C and C++ sources in a package](#c-and-c-sources-in-a-package)).
- Phase 12 reports many of these errors earlier, before the C++ compile.

From Phase 13 on, the same error looks like this:

```text
main.gp:14:7: C++: no member named 'push_bak' in std.vector[int]   (C++ type: std::vector<long>)
        nums.push_bak(4)
             ^
```

- **Columns:** the emitter also writes a small column map next to each generated file
  (generated line and column → `.gp` column). The driver uses it to turn the C++ compiler's
  column into the `.gp` column. Without the map, the message shows the line only.
- C++ type names are translated back to gocpp names (`std::vector<long>` → `std.vector[int]`).

Source: [Phase 04 §2.11](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#211-errors-in-interop-code),
[Phase 13 §2.1](../../plans/2026-10-13-gocpp/phase-13-tooling/README.md#21-diagnostics-that-read-like-go).

## Windows headers

```go
package main

import "<windows.h>"

func main() {
	windows.MessageBoxW(nil, C.wstr("Hello from gocpp"), C.wstr("gocpp"), 0)
}
```

- `<windows.h>` is a header package like any other: `windows.MessageBoxW(nil, …)`,
  `windows.GetLastError()`.
- gocpp defines `WIN32_LEAN_AND_MEAN` and `NOMINMAX` by default, so the `min`/`max` macros can't
  break C++ code. `//gocpp:no-win32-defaults` turns them off. The macro protection above handles
  other collisions (`ERROR`, `DELETE`, `interface`).
- `UNICODE` is defined by default, as Go's own Windows code uses the `W` functions. So
  `windows.MessageBox` (a macro) means `MessageBoxW`.
- `W` functions take UTF-16 text. `C.wstr(s)` gives a `const wchar_t*` for the statement (a
  holder, like `C.cstr`), and `C.GoStringW(p)` converts back.
- x86-64 Windows has a single calling convention, so callbacks work as on Linux.

Source: [Phase 14 §2.4](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#24-interop-with-windows-headers).

## Known gaps until Phase 12

| Gap | Until Phase 12 | What you can do |
|---|---|---|
| a struct that embeds a C/C++ type, with promoted access (`struct{ std.vector[int] }`, then `mv.push_back(1)`) | the error "not supported yet (Phase 12)" | name the field: `mv.vector.push_back(1)` |
| a C macro as a compile-time constant (`var buf [cstdio.BUFSIZ]byte`) | the error "not supported yet (Phase 12)" | |
| a keyed literal of a C struct with array fields | an error | a positional literal (aggregate initialization) |
| `.` vs `::`, template vs index | decided by the kind probe; when it can't tell, gocpp falls back to `.` | write `(x).m` or `(v)[i]`: a parenthesized C++ value is always a value ([the probes](#the-two-compiler-probes)) |
| a smart pointer before `.` | not dereferenced for you | write `(*sp).m` |
| mistakes in C/C++ code | reported by the C++ compiler, at the `.gp` line | |

Source: [Phase 01 §2.9](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#29-unsupported-feature-checker-phase-gates),
[Phase 04 §2.5](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#25-values-and-operations-opaque).

## What typed interop will add

Phase 12 asks clang for the real types of exactly the C/C++ names a program uses, through small
generated "probe" files, the way cgo asks gcc about C names.

- **Earlier, better errors:** many C/C++ mistakes are reported before the C++ compile, with
  gocpp-style messages.
- **Editor support** for C/C++: completion after a header package (`raylib.`) or a namespace
  (`std.`), member completion on C++ values, hover and go to definition into headers. This runs
  only in the background, so it never slows down the other editor features.
- **The two gaps closed:** C macros as compile-time constants (`[cstdio.BUFSIZ]byte`), and
  promoted members of embedded C++ types.
- **Exact C++:** `x->m` or `x.m` instead of `gp::deref(x).m`, keyed C struct literals as
  designated initializers in the right order, exact template-vs-index decisions, and reference
  results copied only when they would dangle.
- **Strings:** a `string` passed to a `const char*` parameter is converted automatically, unless
  the function is known to keep the pointer (`putenv`, `sqlite3_bind_text` with
  `SQLITE_STATIC`, …). Those stay explicit (`C.cstr`, `C.CString`).
- **Wrong header package:** a C-level name used through the wrong header package
  (`raylib.printf`) gets a warning.
- **`gocpp bindgen <header>`** writes a readable gocpp view of a C/C++ header.
- **Compatibility rule:** typed mode never rejects a program that opaque mode and the C++
  compiler accepted. A typed gocpp number next to a C/C++ value is an error, as in opaque mode
  and cgo ("mismatched types"); arithmetic between C/C++ values follows C++'s conversion rules,
  and narrowing conversions there get a `gocpp vet` warning instead of an error.
- **Modes:** `-interop=typed|opaque`. Opaque stays the default until typed mode passes the whole
  Phase 04 suite; then the default switches to typed. Probing needs `clang++`, also when the build
  uses `g++`; without clang, gocpp warns once and uses opaque mode.

Source: [Phase 12](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md).
