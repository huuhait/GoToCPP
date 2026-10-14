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
  files that use it, because an unused import would be a Go error.
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
| `std.cout.flush()` (a value) | `::std::cout.flush()` | members of objects use `.` |
| `stat.struct_stat`, `lib.enum_x`, `lib.union_y` | `struct stat`, `enum x`, `union y` | cgo's convention for C tag names |

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
- After the C/C++ includes, each macro that collides with a gocpp name used in the file (and is
  not itself used through a header package) is `#undef`ed, or saved and restored with
  `push_macro`/`pop_macro`. The macro list from probe 1 makes this exact.
- gocpp's own names that collide with macros are renamed (`errno` → `errno_`,
  [cpp-mapping.md](cpp-mapping.md#names-inside-a-package)).

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
| `x.m`, `x.m()` with `x` opaque | `gp::deref(x).m` | `gp::deref` dereferences **raw pointers only**, like Go's auto-deref; values pass through. Smart pointers are not dereferenced for you: write `(*sp).m` |
| `a + b`, `a == b`, `v[i]` on C++ values | `a + b`, `a == b`, `v[i]` | C++ operators and C++ rules: no gocpp bounds check on C++ containers (use `.at(i)` if you want one) |
| `std.string("hi")`, `std.vector[int](10, 0)` | `std::string("hi")`, `std::vector<int64_t>(10, 0)` | conversion syntax calls a constructor; several arguments are allowed ([D5](language.md#d5-constructor-calls-for-cc-types)) |
| `std.vector[int]{1, 2, 3}` | `std::vector<int64_t>{1, 2, 3}` | |
| `SDL.SDL_Rect{0, 0, 640, 480}` | `SDL_Rect{0, 0, 640, 480}` | positional: aggregate initialization |
| `SDL.SDL_Rect{w: 640, h: 480}` | `[&]{ SDL_Rect gp_t{}; gp_t.w = 640; gp_t.h = 480; return gp_t; }()` | keyed: field assignments, because GCC rejects designated initializers out of declaration order, and opaque mode doesn't know the order. Keyed literals with array fields are an error until Phase 12 |
| `nil` compared with or assigned to a C pointer | `nullptr` | |
| `for i, v := range cppContainer` | `for (auto&& [i, v] : gp::range(c))` | works for anything with `begin()`/`end()` and for C arrays. `v` is a **copy**, like Go. One variable is the index (the key for maps), like Go. A temporary container is kept alive by value, and random-access containers are walked by index with the size read **once**, as Go reads the length once |
| `a, b := std.minmax(x, y)` | `auto [a, b] = gp::values(std::minmax(x, y));` | C++ results that are references (`std::minmax` returns `pair<const int&, const int&>`) are copied into values, so they can't dangle |

### Untyped constants in C/C++ calls

```go
cstdio.printf("%d\n", 5) // 5 is a C int, as printf expects
m := std.max(n, int(0))  // std.max(n, 0) would not compile: int64_t vs int
```

An untyped constant passed to C/C++ is written as a plain C literal with C's default type (`5`
is an `int`, `2.0` is a `double`, `"x.txt"` is a string literal). That is what C APIs expect. A
C++ template that deduces one type from two arguments then needs a typed constant: write
`std.max(n, int(0))`, not `std.max(n, 0)`.

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
  rules.** gocpp's safety covers gocpp types, not C/C++ types.

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
- **C++ exceptions.** In Phase 04, a C++ exception that reaches gocpp code stops the program with
  a Go-style `panic: C++ exception: <what()>` and exit code 2. From Phase 05 on, it becomes a
  normal panic that `recover()` can catch; its value has the type `gp::CppException`, and
  `Error()` returns `what()`.
- **Calling gocpp from C.** Functions marked `//export Name` become `extern "C"` functions in the
  `c-archive` and `c-shared` build modes, with cgo's rules for the allowed types. Every export
  goes through a trampoline too ([Phase 06 §2.7](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#27-build-modes--buildmode-like-go)).

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
- **In an `any`.** A C/C++ value stored in an interface gets a *foreign* type descriptor, named
  after its C++ spelling. Equality, hashing and printing use whatever the C++ type supports
  (`operator==`, `std::hash`, `std::formatter` or `operator<<`) ([Phase 05 §2.2](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#22-type-descriptors-chosen-by-the-go-type)).
- **Interface methods with C/C++ types.** `go/types` treats every C type as invalid, and two
  invalid types compare equal. So gocpp checks method signatures with C/C++ types itself: a
  `static_assert` at each static conversion, and a comparison by source spelling at run time
  ([Phase 05 §2.3](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#23-interface-values-and-itabs)).

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
`.c` with `$CC -std=c17`, and `.cc`, `.cpp`, `.cxx` with `$CXX -std=c++20`, using the package's
flags.

**Directives** are line comments before the `package` clause, like `//go:build`.

| Directive | Arguments | Effect |
|---|---|---|
| `//gocpp:pkg-config sdl2` | one or more package names | runs `pkg-config --cflags --libs sdl2` and adds the flags it prints, if they are allowed |
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
  in Go. gocpp checks `cflags`, `ldflags` and the output of pkg-config against Go's list of
  allowed flags (Phase 04), also those of `gocpp.mod` and of every dependency (Phase 06). The
  regexps in `GOCPP_CFLAGS_ALLOW` and `GOCPP_LDFLAGS_ALLOW` allow more flags, like cgo's
  `CGO_CFLAGS_ALLOW`.
- A header is written like its import path, without the Go quotes: `<lua.h>`, or a path that
  ends in `.h`, `.hh`, `.hpp`, `.hxx`, `.h++` or `.inl`, such as `./legacy.h`.
- A directive can be repeated, and the repeats add up: two `//gocpp:link` lines link the
  libraries of both.

**Module-wide settings** go into `gocpp.mod` (`cflags`, `pkg-config`, …). Packages add their own
with directives.

**Public flags.** Flags that change how a package's headers parse (`-I`, `-D`) are passed to
every package that depends on it, so no two translation units see a C struct with different
layouts ([Phase 06 §2.3](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#23-header-layout-c-needs-declarations-before-use-go-doesnt)).

**The `extern "C"` rule** (Phase 04). A header needs `extern "C"` when it declares C functions
without its own `__cplusplus` guard:
- a **local `.h`** header is wrapped only if it **parses as C** (checked once with
  `$CC -fsyntax-only -x c`, cached) and has no `__cplusplus` guard;
- `.hpp`, `.hh`, `.hxx`, `.h++` and `.inl` headers are C++ headers: they are not wrapped by
  default;
- system headers are not wrapped by default;
- `//gocpp:extern-c <hdr>` wraps the header, whatever these defaults say, also a `.hpp` file (a C
  header with an odd name). It is there for C libraries that ship headers without guards, such
  as `<lua.h>`;
- `//gocpp:no-extern-c <hdr>` turns the wrapping off for that header;
- `extern-c` and `no-extern-c` on the same header is an error. So is an `extern-c` or
  `no-extern-c` header that no file of the package imports, because a typo would otherwise do
  nothing. This also catches `.\legacy.h`, which becomes `.legacy.h` (the `\` escapes the `l`).

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
main.gp:14: C++: no member named 'push_bak' in 'std::vector<long>'
        nums.push_bak(4)
```

- In opaque mode, the C++ compiler finds mistakes in C/C++ code. These errors are **your**
  errors, not internal ones.
- In Phase 04, `#line` makes them point at the `.gp` line. `#line` carries no column, so the
  message shows the line only.
- The driver prints them in gocpp style. Template backtrace noise is trimmed to the first error
  plus the lines that point into `.gp` files; `-cpperrors=full` shows everything.
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
  compiler accepted. Mixed C/gocpp arithmetic follows C++'s conversion rules, as in opaque mode;
  narrowing conversions get a `gocpp vet` warning instead of an error.
- **Modes:** `-interop=typed|opaque`. Opaque stays the default until typed mode passes the whole
  Phase 04 suite; then the default switches to typed. Probing needs `clang++`, also when the build
  uses `g++`; without clang, gocpp warns once and uses opaque mode.

Source: [Phase 12](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md).
