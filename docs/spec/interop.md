# Using C and C++ from gocpp

A `.gp` file can import any C or C++ header and use it directly, with no glue code. This
document says how: import forms and names, what `C` holds, how names and types are translated,
what you can do with C/C++ values, strings and pointers at the boundary, callbacks, and how the
C/C++ side is built.

| | |
|---|---|
| Status | complete for Phase 04 (opaque interop, step 4.19): every feature of steps 4.1 to 4.17 is described here, with its limits. Typed interop (Phase 12), which is the default when clang++ is installed, adds to this and closes the "known gaps". The [cookbook](interop-cookbook.md) has task-oriented recipes whose programs are tests |
| Source | [Phase 01 §2.3–2.5](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification) and [§2.8](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#28-the-opaque-pass), [Phase 04](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md), and the other READMEs linked below |
| See also | [interop-cookbook.md](interop-cookbook.md) (recipes), [modules.md](modules.md), [language.md](language.md) ([D2](language.md#d2-header-imports-instead-of-cgo), [D14](language.md#d14-cc-values-follow-c-rules), [D16](language.md#d16-unsafepointer-and-pointers-from-c)), [cpp-mapping.md](cpp-mapping.md) |

**Opaque mode** (what you get with `-interop=opaque`, or when no clang++ is installed). gocpp does not try to understand C++ (templates, overloads, macros). Every
C/C++ name is **opaque**: gocpp writes it into the C++ output almost unchanged, and the C++
compiler, which understands C++ perfectly, checks it. That is why almost every header, library
and template works from the start. gocpp only needs two small facts from the compiler: is a name
a macro, and is it a namespace, a type, a template or a value ([the probes](#the-two-compiler-probes)).
Typed interop (Phase 12; on by default when clang++ is installed, [what it adds](#what-typed-interop-adds)) asks clang for real types
in addition; this document describes the opaque mode, which is also what typed mode falls back to for anything a probe does not answer.

**Contents.**
[A first example](#a-first-example) ·
[Import forms](#import-forms) (header packages, module headers, namespaces, order) ·
[What `C` holds](#what-c-holds) ·
[How names are translated](#how-names-are-translated) (the two probes, macros) ·
[C/C++ types in gocpp code](#cc-types-in-gocpp-code) (embedded C types) ·
[Values and operations](#values-and-operations) (literals, `range`, constants, closures and `defer`, types that can't be copied) ·
[Strings](#strings-at-the-boundary) ·
[Pointers, `unsafe` and slices](#pointers-unsafe-and-slices-at-the-boundary) ·
[Callbacks and panics](#callbacks-and-panics) (C function pointers, user data, C++ exceptions) ·
[Interfaces and `fmt`](#cc-values-in-interfaces-and-fmt) ·
[Building C and C++ sources](#building-c-and-c-sources) (directives, flags, `extern "C"`) ·
[Functions that no header declares](#functions-that-no-header-declares) (`//gocpp:extern`) ·
[Small helpers](#small-helpers-a-header-next-to-the-gp-file) ·
[Errors in interop code](#errors-in-interop-code) ·
[Windows](#windows-headers) ·
[Limits and known gaps](#limits-and-known-gaps) ·
[Troubleshooting](#troubleshooting) ·
[How interop is tested](#how-interop-is-tested) ·
[What typed interop adds](#what-typed-interop-adds).
Recipes: [cookbook](interop-cookbook.md).

## A first example

<!-- test: tests/interop/doc_first_sqlite.gp -->
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
	fmt.Println("sqlite", C.GoString(sqlite3.sqlite3_libversion()) != "")
}
```

Output:

<!-- output: tests/interop/doc_first_sqlite.gp -->
```text
sqlite true
```

<!-- test: tests/interop/doc_first_std.gp -->
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

Output:

<!-- output: tests/interop/doc_first_std.gp -->
```text
bob 31
1 3
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
| `"github.com/huuhait/game/native/x.h"`, `"github.com/ocornut/imgui/imgui.h"` (first element has a dot, ends in a header extension) | module header: a file of the project's own module or of a required module ([Module headers](modules.md#module-headers)) | `#include "/abs/path/to/x.h"`, the module's folder is `-iquote` | `x.X`, `imgui.X` |
| `rl "<raylib.h>"`, `_ "<GL/gl.h>"` | a header with a name you choose, or with no name | as above | `rl.X`, or no name |
| `fs "::std::filesystem"` | a C++ namespace with a new name (optional) | nothing (it is just a name) | `fs.path` → `::std::filesystem::path` |
| `"::std"` (no name) | an error: namespaces need no import | — | the message says: use `std.X` directly, or rename it: `import cstd "::std"` |

No new syntax is needed: the shape of the import path says what it is. A path in `<...>` is a
system header. A path that ends in `.h`, `.hh`, `.hpp`, `.hxx`, `.h++` or `.inl` is a local
header, or, when its first element has a dot (`github.com/...`), a module header, which names a file in
a module like a Go import names a package: the longest module path that is a prefix says which module,
the project's own module included. A path that starts with `::` is a namespace. Every other path is a gocpp package, but a
path that starts with `./` or `../` must be a header.

### Header packages and their names

<!-- illustration: a fragment, not a program -->
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

### Module headers

<!-- illustration: a fragment, not a program -->
```go
import (
	"github.com/huuhait/game/native/physics.h"  // physics.h in this project (module github.com/huuhait/game)
	"github.com/ocornut/imgui/imgui.h"          // a header of a required module
)
```

A module header is a header package like any other (the name is the file name; `extern "C"`, the
probes and the namespaces work as for a local header), with these differences
([the full rules and messages](modules.md#module-headers)):

- The module's folder is found like the folder of a package: the project's own module needs no `require`,
  a required module comes from `vendor/`, its `replace` folder, or (later) the module cache.
- The `#include` has the **absolute name of the file**, so the C++ compiler takes exactly that file, and the
  folder of the module is an `-iquote` folder of the build and of every probe (the kind, namespace and macro
  probes and the C header check run with it, and their cache keys have it). A library's headers then include
  each other by their own names in quotes (`#include "imconfig.h"` in `imgui.h`, `#include "imgui.h"` in
  `backends/imgui_impl_glfw.h`). It is `-iquote` and not `-I`, so a `string.h` or `time.h` at the module root
  never hides the system header for `<cstring>`; a library that wants `<imgui.h>` to work needs the `include`
  folders of its `cxx` recipe.
- A library whose C and C++ files must be compiled has a **`cxx` recipe** in `gocpp.mod` (its sources, `include`
  folders, `define`s, `pkg-config` packages and link libraries; [the language](modules.md#c-and-c-libraries-cxx-recipes),
  [ADR-024](../adr/024-cxx-recipes.md)). Its `include` and `define` statements are flags of the package's compiles and
  of all the probes, like the flags of `//gocpp:cflags`; the library is compiled once into the object cache and
  linked. The module-wide `cflags`, `ldflags` and `pkg-config` lines in the `gocpp.mod` of a dependency are **not**
  applied (the flag allow-list question for dependencies is open); the recipe is the explicit way. Use
  `//gocpp:cflags` and `//gocpp:pkg-config` in the package for flags of your own.
- `"./x.h"` and `"native/x.h"` are unchanged: next to the `.gp` file.

### Namespaces are packages

<!-- test: tests/interop/doc_namespaces.gp -->
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

Output:

<!-- output: tests/interop/doc_namespaces.gp -->
```text
3 0 "x.txt"
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
  - A local header that is not a file: a folder gives `cannot find header "./x.h"`, and a named
    pipe, a socket or a device gives `cannot check the header imports: kind probe: open ...: not a
    regular file: it is a named pipe` (no compiler waits for it).
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

<!-- illustration: a fragment, not a program -->
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

<!-- test: tests/interop/doc_c_holds.gp -->
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

Output:

<!-- output: tests/interop/doc_c_holds.gp -->
```text
hello gocpp (5 letters)
strlen = 32
```

`C` holds only **C's built-in types** and the **boundary helpers**, as in cgo. Everything a header
declares comes through its package or a namespace, never through `C` (`cstdio.printf`, not
`C.printf`).

| `C` type | C++ |
|---|---|
| `C.char`, `C.schar`, `C.uchar` | `char`, `signed char`, `unsigned char` |
| `C.short`, `C.ushort` | `short`, `unsigned short` |
| `C.int`, `C.uint` | `int`, `unsigned int` |
| `C.long`, `C.ulong` | `long`, `unsigned long` (64 bits on Linux, 32 bits on Windows: [LLP64](#windows-headers)) |
| `C.longlong`, `C.ulonglong` | `long long`, `unsigned long long` |
| `C.float`, `C.double` | `float`, `double` |
| `C.bool` | `bool` |
| `C.size_t` | `size_t` |
| `C.wchar_t` | `wchar_t` (16 bits on Windows, 32 bits on Linux) |

`C.int`, `C.float` and the others are the Go fixed-size types of the same size: [`C.int` is `int32`](#the-c-number-types-are-gos).

| Helper | What it does | More |
|---|---|---|
| `C.cstr(s)` | a NUL-terminated `const char*` for a gocpp string, valid until the end of the statement | [strings](#strings-at-the-boundary) |
| `C.CString(s)`, `C.free(p)` | a `malloc`ed C copy of a string (cgo-compatible), and freeing it | [strings](#strings-at-the-boundary) |
| `C.GoString(p)`, `C.GoStringN(p, n)`, `C.GoBytes(p, n)` | C memory → gocpp `string` or `[]byte` (cgo-compatible) | [strings](#strings-at-the-boundary) |
| `C.wstr(s)`, `C.WString(s)`, `C.GoStringW(p)`, `C.GoStringWN(p, n)` | wide strings: UTF-16 for the `W` functions of Windows (UTF-32 where `wchar_t` has 32 bits); `C.WString` is `C.CString` for wide characters | [Windows](#windows-headers) |

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

**Probe results on disk.** The compiler takes about a second for the standard headers, so the
answers of both probes (and of the `extern "C"` header check, further down) are kept on
disk, `gocpp/probe/` in the user cache folder (`$XDG_CACHE_HOME`, or `~/.cache` on Linux), and
every later `gocpp emit`, `check` or `build`, and the editor after a restart, reads them in a few
milliseconds instead of running the compiler. An entry is used only when nothing it depends on
changed:
- **Key:** the compiler (path, arguments, family, version line, and its program file: real path,
  size and modification time), the flags, the includes, the package folder (for `"quoted"`
  includes), the environment that changes include paths and flags (`CPATH`, `CPLUS_INCLUDE_PATH`,
  `C_INCLUDE_PATH`, `NIX_CFLAGS_COMPILE*`, `NIX_HARDENING_ENABLE*`, `NIX_ENFORCE_*`, `SDKROOT`,
  the `*_DEPLOYMENT_TARGET` variables, ...; `PATH` too for a compiler with arguments such as
  `ccache g++`, which is then keyed by the launcher, not by the compiler that it starts), and
  gocpp itself (its version, the executable's path, size and time, and the build's version
  control stamp).
- **Files:** every header that the compiler read, system headers included, with its content hash
  and stat. A header that changed (or a `"quoted"` include that now exists next to the `.gp`
  file) is a miss; one that was only touched is not. A result that depends on a file that changed
  in the two seconds before the probe started is not written (the file may have changed while the
  compiler ran).
- **Not written:** results with include errors (they are kept in memory for 5 seconds, since a
  library that gets installed fixes them), C header checks that stopped at a header that isn't
  found or that have no dependency list, and results that depend on the host's CPU
  (`-march=native`, `-mtune=native` or `-mcpu=native` in the flags, the compiler's arguments or
  `NIX_CFLAGS_COMPILE`: the cache folder can be shared between machines, for example a home
  folder over NFS). Those stay in memory.
- **A limit, and its backstop:** the file list can't see a header that appears *earlier* on the
  search path than the one that was found (a new `x.h` in the first `-I` folder when `<x.h>` was
  found in the second), or one that `__has_include` asks for. In memory that holds for a
  process; on disk it would hold for good, so an entry is used for one hour at most (an entry
  that adds new paths keeps the age of its oldest result). With the disk cache on, a process
  that runs for days, such as the language server, also probes a set again once it is an hour
  old (one background run an hour), so its memory doesn't keep the limit for the whole session
  and it never writes an expired entry over a fresh one. `GOCPP_NOPROBECACHE=1` for a run, or
  deleting the folder, forgets everything at once.
- **Safety:** entries are written to a temporary file and renamed, so any number of gocpp
  processes can share the folder; an entry that is damaged, from another format, or for another
  key is ignored; a folder that can't be written is not an error. The folder holds at most 512
  entries and 128 MiB; the entries used least recently go first.
- **Switches:** `GOCPP_NOPROBECACHE=1` turns the disk cache off (the memory cache stays);
  `GOCPP_PROBE_CACHE=<absolute folder>` moves it. Test binaries use the disk cache only with
  `GOCPP_PROBE_CACHE`, so a test never writes your cache.

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
  runtime or of another gocpp package, such as `Println` in `gp::fmt::Println`) is `#undef`ed
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

<!-- illustration: a fragment, not a program -->
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

### Embedded C types

<!-- illustration: a fragment, not a program -->
```go
type MV struct {
	std.vector[int] // the field is called vector
	n int
}

var mv MV
mv.push_back(1) // the same as mv.vector.push_back(1)
mv.size()
```

A struct (or a pointer to one) that embeds **one** C/C++ type, directly or through embedded Go
structs and embedded pointers, gets the members of that type promoted: `mv.push_back(1)` is written
`gp::deref(mv.vector).push_back(1)`, and `o.x` is `o.Mid->epoint.x` through an embedded
`*Mid` (Go's nil check on the way). Reads, writes, `++`, `&o.x`, calls, closures and conditions work as
with the explicit field. Step 5.8, `internal/check/promoted.go`.

- go/types knows no member of a C/C++ type, so it reports `x.name undefined` for a member of
  `std.vector[int]`. Check drops that false error when `x` is a value whose type has exactly one embedded
  C/C++ type and no Go field or method of that name at any depth (go/types' own error stays for an
  unexported name of another package and for an ambiguous Go name). The C++ compiler checks that the
  member exists, as for every member of a C/C++ value: a typo is a C++ error at the `.gp` line.
- **Depth.** Go picks the shallowest member, and the same depth is ambiguous. A member of an embedded C/C++
  type is one level deeper than its field. When go/types resolves `x.name` to a Go field or method and the
  struct also embeds a C/C++ type at a depth where its members would be at the same or a shallower depth than
  the Go member, check can't know whether that type has the name (Go would pick the C++ member, or say
  "ambiguous selector"): the error is "selectors that a Go member and a member of an embedded C/C++ type could
  both match (name the field) are not supported yet (Phase 12)", for the selector `s.x`; write `s.epoint.x` or
  the Go path. A C/C++ type that is deeper than the Go member never matters (`tests/interop/embed_cxx_depth.gp`).
  Typed mode (`-interop=typed`, step 12.9) asks the C++ compiler whether the type has the name and applies
  Go's rule: [typed-interop.md §11](typed-interop.md#11-promoted-members-of-embedded-c-classes-step-129-adr-098-gap-e2).
- A method expression of a promoted member (`MV.size`, `(*MV).push_back`) has no C++ form yet: "method
  expressions of promoted members of an embedded C/C++ type are not supported yet (Phase 12)"; use a func
  literal. (A call or a `defer` of the member is fine: `defer mv.push_back(1)` works.)
- **Stores.** `n.next = p` into a promoted pointer member keeps `p`'s object alive like the store into
  the explicit path (`n.unode.next = p`, step 4.10) when `n` is a gocpp pointer to the outer struct (also for
  `hh.in.p` and `hh.slots[1]` of a promoted struct). Through a value or an embedded pointer (`m.N`), no
  expression names the owner, and the error says to write the path of the embedded fields, `m.N.unode.next`.
- With **several** embedded C/C++ types (also in different depths), gocpp can't tell which one has the
  member until typed interop (Phase 12): the error is "promoted fields and methods of several
  embedded C/C++ types are not supported yet (Phase 12)"; name the field. (Typed mode asks the C++ compiler which
  type has the member: the selector is promoted, or it is an ambiguity, a missing or an inaccessible member, at the
  selector: [typed-interop.md §11](typed-interop.md#11-promoted-members-of-embedded-c-classes-step-129-adr-098-gap-e2).)
- A C/C++ method is not part of the Go method set of the struct: the struct does not implement an
  interface that needs it (go/types reports the missing method), and `x.name` is not a method
  value of the struct.

Source: [Phase 04 §2.4](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#24-cc-types-in-gocpp-code).

## Values and operations

### Typed and opaque expressions

<!-- illustration: a fragment, not a program -->
```go
x := lib.f()                // opaque: auto x = ...; owns pair/tuple reference members
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
| `x := lib.f()` | `auto x = [&]<typename R>(std::type_identity<R>) { if constexpr (gp::detail::is_values<R>) { return gp::values<R>(::f()); } else { return (::f()); } }(std::type_identity<decltype(::f())>());` | the dependent branch owns pair/tuple reference members before argument temporaries die; ordinary results preserve guaranteed prvalue elision, including nonmovable types. The call is evaluated once (its type is taken outside the lambda, so a func literal in the arguments can capture a local, also for clang; a constructor, a conversion and a gocpp function need no wrapper) |
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
| `std.vector[int]{1, 2, 3}`, `std.map[std.string, int]{"a": 1}` | `std::vector<int64_t>{1, 2, 3}`, `gp::cgo::map_t<std::map<std::string, int64_t>>{{"a", 1}}` | C++'s braced initialization; keys that are not names make a C++ map's pairs ([composite literals](#composite-literals-of-cc-types)) |
| `SDL.SDL_Rect{0, 0, 640, 480}` | `SDL_Rect{0, 0, 640, 480}` | positional: aggregate initialization |
| `SDL.SDL_Rect{w: 640, h: 480}` | `[&]() { SDL_Rect gp_t0{}; static_assert(gp::cgo::field_ok<decltype(gp_t0.w)>); gp_t0.w = {640}; ...; return gp_t0; }()` | keyed: field assignments, because GCC rejects designated initializers out of declaration order, and opaque mode doesn't know the order. Keyed literals with array fields are an error until Phase 12 |
| `nil` compared with or assigned to a C pointer | `nullptr` | |
| `for i, v := range cppContainer` | `for (auto&& [i, v] : gp::range(c))` | works for anything with `begin()`/`end()` and for C arrays. `v` is a **copy**, like Go. One variable is the index (the key for maps), like Go. A temporary container is kept alive by value, and random-access containers are walked by index with the size read **once**, as Go reads the length once ([below](#range-over-c-containers-and-c-arrays)) |
| `a, b := std.minmax(x, y)` | `auto gp_t0 = gp::values(std::minmax(x, y)); auto a = std::get<0>(gp_t0); auto b = std::get<1>(gp_t0);` | C++ results that are references (`std::minmax` returns `pair<const int&, const int&>`) are copied into values, so they can't dangle. Also with `=`, `_` and variables that exist, and for one variable, `p := std.minmax(x, y)` (the dependent branch selects `gp::values`, as above). `p = std.minmax(x, y)` into a `p` that exists copies the values anyway |

**gocpp values next to C/C++ values.** As in cgo, a typed gocpp number, bool or string next to a
C/C++ value in an operator is an error, because C++ would convert one of them without a word
(`n + v.size()` would turn a negative `n` into a huge unsigned number). The exception is the type that *is*
the same: `C.int` is `int32` and the other C number types are Go's fixed-size types
([below](#the-c-number-types-are-gos)), so `x + n32` is fine and only `x + n` with `n int` is the error:

<!-- illustration: a fragment, not a program -->
```go
var x C.int = 2
n := 3
_ = x + 1      // fine: 1 is a C literal
_ = x + n32    // fine: C.int is int32, n32 an int32
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
values are, also of a C++ reference: `x := v[0]` and `all := lib.the_list()` copy (`auto`), and
so is a temporary that gocpp makes for the order of evaluation.
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
C/C++ value. Convert it (`int(nodes[0].N)`, `string(nodes[0].Name)`); a string that comes back
this way compares with a string constant as in Go (`nodes[0].Name == "a"`).

A C/C++ value doesn't convert implicitly to a gocpp type, as in cgo: `var n int = cmath.pow(10, 30)`
is an error ("cannot use cmath.pow(10, 30) (C/C++ value) as int value in variable declaration:
convert it, int(cmath.pow(10, 30))"), also in an assignment, an argument and a return. `int(x)` converts with Go's
rules (`gp::cgo::as`: a float out of range gives Go's result, not C++'s undefined behavior). A C
pointer converts to a number or to another pointer type only through `unsafe.Pointer`
([pointers](#pointers-unsafe-and-slices-at-the-boundary)), and only a bool converts to bool.
The other way round too: a typed gocpp value doesn't go implicitly into a C/C++ variable, field,
parameter of a gocpp function or result (`fv := 1e30; var i C.int = fv` is "cannot use fv
(variable of type float64) as C.int value in variable declaration: convert it, C.int(fv)"); an
untyped constant does (it is a C literal there). An argument of a C/C++ function is different:
gocpp doesn't know the parameter's type, so C++'s rules convert it (a gocpp float passed to a C
integer parameter out of its range is undefined behavior: write `C.int(x)`). Phase 12 (typed
interop) knows the parameter types and makes this exact: it converts a float with Go's rules, checks
a constant against the parameter's range, and adapts a few other arguments
([argument adaptations](#argument-adaptations-typed-interop)).

### Composite literals of C/C++ types

A literal of a C/C++ type is C++'s braced initialization (step 4.6). C++ decides what the braces
do: aggregate initialization for a C struct, a constructor, or a `std::initializer_list`
(`std::vector`, `std::map`):

<!-- illustration: a fragment, not a program -->
```go
r := SDL.SDL_Rect{0, 0, 640, 480}            // SDL_Rect{0, 0, 640, 480}
v := std.vector[int]{5, 3, 8}                // std::vector<int64_t>{5, 3, 8}
p := std.pair[int, std.string]{7, "seven"}   // a constructor
boxes := std.vector[lib.Box]{{{0, 0}, {2, 2}}, {}} // inner literals may leave their type out
line := lib.Line{a: {1, 2}, b: lib.Vec{y: 3}}       // also in a keyed literal
```

- **Positional** literals keep C++'s rules, also the narrowing rule: `lib.Vec{1, n}` with a gocpp
  `int` n and a C `int` field is a C++ error at the `.gp` line (with both compilers; gocpp makes
  it an error for g++ too), as cgo rejects it. Write `C.int(n)`.
- **Keyed** literals whose keys are names set those fields in the literal's order, and the fields
  left out are zero: `[&]() { T gp_t0{}; static_assert(gp::cgo::field_ok<decltype(gp_t0.w)>);
  gp_t0.w = {640}; return gp_t0; }()` (bit-fields work too). The values are braced (`{640}`), so
  the narrowing rule holds here too. An **array field** can't be set this way (C++ can't assign
  arrays): "a keyed literal of a C/C++ type can't set an array field (until Phase 12): write a
  positional literal". Duplicate field names are an error, as in Go.
- **Keyed** literals whose keys are not names are a C++ map's: `std.map[std.string, int]{"a": 1,
  "b": 2}` is `gp::cgo::map_t<std::map<...>>{{"a", 1}, {"b", 2}}`, and so is the C++ spelling
  `std.map[std.string, int]{{"a", 1}}`. The type must be a map (it has a `mapped_type`): Go's index
  keys of a sequence, `std.vector[int]{2: 7}`, are an error, "a keyed literal of a C++ type with
  keys that aren't field names needs a map (std.map, std.unordered_map, ...)", because a braced
  list of pairs would silently make other elements. Duplicate constant keys of a `std.map` or a
  `std.unordered_map` are an error, as in Go; a `std.multimap` takes them. Names and other keys in
  one literal, or keyed and positional elements, are an error.
- **Every name as a key is a field name**, also a variable or a named constant: in
  `std.map[int, std.string]{Red: "red"}`, `Red` is taken as a field. Put such a key in
  parentheses, `{(Red): "red"}`, or write the pair, `{{k, 1}}`.
- An **inner literal** in a literal of a C/C++ type may leave its type out, also where Go wouldn't
  allow it (a struct field): it is a braced list, and C++ decides what it is. A keyed one needs
  its type, `std.vector[lib.Vec]{lib.Vec{y: 1}}`, because the assignments need a type to start
  from. Because C++ decides, `std.vector[*Node]{{}}` holds a nil gocpp pointer, not a new
  `&Node{}` as Go's `[]*Node{{}}` does.
- **Careful with C++ constructors:** `std.vector[std.string]{{"a", "b"}}` and
  `std.string{"a", "b"}` pick `std::string`'s constructor from two iterators, which reads from
  `"a"` to `"b"`: undefined behavior (g++ throws a `length_error`, clang `-asan` reports a
  global-buffer-overflow). Write `std.vector[std.string]{"a", "b"}`.
- The elements are evaluated in Go's order, from left to right (keys before their values); untyped
  constants are C literals, `nil` is `nullptr`.
- `&T{...}` of a C/C++ type is `gp::cptr<T>(gp::cgo::box_as<T>(...))`, a gocpp pointer that owns the new object
  ([pointers](#pointers-unsafe-and-slices-at-the-boundary)): in a new variable (`p := &lib.Vec{1, 2}`),
  an argument or an `unsafe.Pointer` conversion. Not where a raw C pointer is declared (a variable,
  field or result of type `*lib.Vec`): there it would dangle at once, which is an error.
- A package variable can take its type from the literal: `var origin = lib.Vec{y: 1}`.

**gocpp literals with C/C++ parts** are gocpp values with Go's rules: `[]C.int{1, 2}`,
`[...]C.double{0.5}`, `[2]lib.Vec{{1, 2}}` (an inner literal leaves its type out, as in Go),
`map[string]C.int{"a": 1}` and a gocpp struct with C fields (`Shape{box: lib.Box{}, n: 4}`);
`&[]C.int{1}` is a gocpp pointer to a new slice, as in Go. Where
both types are known, cgo's rule holds: a typed gocpp value into a C element or field is an error,
"cannot use n (variable of type int) as C.int value in array or slice literal: convert it,
C.int(n)", and a C/C++ value into a gocpp element too ("convert it, int(x)").

### range over C++ containers and C arrays

<!-- illustration: a fragment, not a program -->
```go
v := lib.scores()                // a std::vector<int>
for i, x := range v {            // for (auto&& [i, x] : gp::range(v)) {
	v.push_back(x)                // not visited: the size was read once
}
for k, n := range ages {         // a std::map: the key and a copy of the value
	fmt.Println(int(k.size()), int(n))
}
for i := range lib.table {       // int table[8] in the header: the index
	lib.table[i] = 0
}
for _, s := range f().items {    // f() stays alive for the whole loop
}
```

- `range` works over anything with `begin()` and `end()`: C++ containers, views
  (`std.views.iota(0, 5)`), spans and C arrays reached through a header. The four forms of Go
  work, with `:=` and with `=` (`for i, v = range c`), and with labels, `break`, `continue` and
  `defer` in the body.
- One variable is the index, or the key of a map (`std::map`, `std::unordered_map` and their
  multi forms), as in Go. The value is a **copy** of the element (of the mapped value): changing
  it doesn't change the container. Each iteration has its own variables (Go 1.22). A blank
  target copies nothing: `for range c`, `for _ = range c`, and `for _, _ = range c` only
  advance; `for k := range m` does not copy a mapped value, and `for _, v := range m` does
  not copy a map key. These walks also work when the discarded type is noncopyable.
- The index and the key are C/C++ values (the index is a C++ `int64_t`, gocpp's `int`): gocpp
  doesn't know in opaque mode whether the container is a map. Convert them, `int(i)`. For the
  same reason, a range that assigns (`=`) needs C/C++ targets: `var i int; for i = range v`
  is an error ("declare a new variable with :=, and convert it").
- A `std::string` is a C++ container of `char`: `range` gives the byte index and each byte as a
  C `char`, not runes as Go's range over a string does. For runes, convert it to a gocpp string
  first, `range string(s)`.
- The range expression is evaluated **once**. A temporary container (`range lib.squares(4)`) is
  held by value inside `gp::range`. The temporaries that the expression needs otherwise go into
  named temporaries before the loop: the object of a member (`f().items`, where `f` may return
  the only pointer to it), and the object and temporary arguments of a call that can return a
  reference into them (`lib.longer(lib.squares(2), lib.squares(3))`), a dereference's owner
  (`*lib.vector_owner()`), a temporary callable (`lib.make_reader()()`), and constructor
  backing arguments (`std.string_view(std.string("long heap-backed text"))`). C++'s own
  range-for would destroy them before the body runs. Two limits: a literal `0` argument stays
  as it is, because only a literal `0` is a null pointer constant in C++ (`range lib.from_cstr(0)`,
  also `1 - 1`), so it is not retained for a function that returns a reference into it; and a
  temporary that **C++ itself creates by an implicit conversion inside the call**
  (`range p.first_ref("a long string literal")` for `const std::string& first_ref(const
  std::string&)`, an implicit constructor `Rng(int)`) dies at the end of the range expression, as
  in C++'s own range-for, so a reference the call returns into it dangles (AddressSanitizer:
  stack-use-after-scope). Make the temporary yourself (`std.string("...")`), which gocpp keeps.
- **Random-access containers** (`std::vector`, `std::deque`, `std::array`, `std::string`,
  `std::span`, random-access views, C arrays) are walked by index, with the size read once, as Go
  reads a slice's length once. A `push_back` in the body is not visited and doesn't break the walk
  when the vector reallocates; a write to a later element is seen when the walk gets there (a
  difference from Go, where `range arr` over an array walks a copy, and only a slice is seen
  changing: the walk here is over the C array or `std::array` itself). If the
  body **shrinks** the container, the walk panics when it reaches an index past the new end, with
  Go's message (`index out of range [2] with length 2`), instead of reading a destroyed element.
- **Other containers** (`std::list`, `std::set`, maps, other views) are walked with their
  iterators, which follow **C++'s invalidation rules**: inserting into a `std::map` or a
  `std::list` during the loop is fine (a new key after the current one is visited, one before it
  is not; Go may or may not produce a new key), but erasing the current element, or an insert that
  makes a `std::unordered_map` rehash, is undefined behavior, as in C++.
- What doesn't work is a C++ error at the `range` line with a message: the rows of a C array of
  arrays (`int grid[2][3]`) can't be copied into the loop variable (use `for i := range grid`
  and `grid[i]`), a C number is no range (convert it, `range int(n)`), and a value without
  `begin()` and `end()` isn't either. An element that can't be copied (`std::unique_ptr`) can
  still be discarded: use the key/index form and `c[i]`, or no
  bindings. A noncopyable map key can likewise be discarded when requesting only its value.
- The variables follow Go's rule: `for i, x := range v {}` is an error ("declared and not used") for each variable the body doesn't use; write `_` or leave the variables out. One difference: any use that the checker records counts, also a write (`for i := range v { i++ }` and `k = 1` are accepted, where Go reports the variable), and a variable named as a bare key of a literal in the body (`std.map[int, int]{i: 3}`, where the key may be a C struct field) counts as used.
- A closure can use the loop variables (each iteration has its own, [boxed](#cc-objects-in-closures-and-defer)); `&x` of a range variable is the address of the iteration's copy.

### Untyped constants in C/C++ calls

<!-- illustration: a fragment, not a program -->
```go
cstdio.printf("%d\n", 5) // 5 is a C int, as printf expects
m := std.max(n, int(0))  // std.max(n, 0) would not compile: int64_t vs int
```

An untyped constant passed to C/C++, or used next to a C/C++ value (an operand, an index, the
value of `m["k"] = 1` or `s += "!"`), is written as a plain C literal with C's default type (`5`
is an `int`, `2.0` is a `double`, `"x.txt"` is a string literal). That is what C APIs expect. A
C++ template that deduces one type from two arguments then needs a typed constant: write
`std.max(n, int(0))`, not `std.max(n, 0)` ([tests/interop/cpp_error_max.gp](../../tests/interop/cpp_error_max.gp)
shows the error). A constant that does not fit an `int` is a `long` (`1<<40`), as in C; `long` has 64 bits on
Linux and 32 on Windows, so write `%lld` with `C.longlong(x)` where the width matters.

A constant for a variable, field, result or parameter of a gocpp function declared with one of
C's built-in types, and a constant converted to one, follow Go's rules, as in cgo: the value must
fit, and only `nil` goes to a C pointer. The same check applies to declared C elements and
pointed-to values in assignment, and to constants next to known C operands in arithmetic,
comparison and `op=`. An explicit C conversion checks typed constants too; conversions of
variables still use runtime Go conversion rules.

<!-- illustration: a fragment, not a program -->
```go
var c C.char = 200   // cannot use 200 (untyped int constant) as C.char value in variable declaration (overflows)
var n C.int = 2.5    // ... as C.int value in variable declaration (truncated)
var s *C.char = "hi" // cannot use "hi" (untyped string constant) as *C.char value in variable declaration
_ = C.int(2.5)       // cannot convert 2.5 (untyped float constant) to type C.int
_ = C.char(200)      // constant 200 overflows C.char
```

The ranges are cgo's on gocpp's targets (x86-64): `C.char` is signed, `C.long` has 64 bits (on
Windows it has 32: gocpp doesn't check that difference yet). A constant for any other C/C++ type
(a header's typedef, `std.string`) and an argument of a C/C++ function are the C++ compiler's to
check: gocpp doesn't know those types in opaque mode.

### C/C++ objects in closures and `defer`

<!-- illustration: a fragment, not a program -->
```go
v := std.vector[int]{}
add := func(x int) { v.push_back(C.int(x)) }
add(1)
fmt.Println(int(v.size())) // 1: the closure and the function share the one vector
defer v.clear()            // fine: v is boxed, so it is still alive when the defer runs
```

- **What is boxed.** A local (also a parameter, a named result, or a loop variable) whose type holds a
  C/C++ object by value is **boxed** when a func literal uses it, or when a deferred call is made on
  it. Those types are: a C/C++ type (`std.vector[int]`, `C.int`, a header's struct), a raw C/C++
  pointer (`*lib.FILE`), a gocpp struct with such a field, and an array of them. A slice, a map or a
  gocpp pointer follows [Phase 03's rule](cpp-mapping.md#closures) (they share what
  they point to anyway). The box is a `gp::ptr<T>`; the closure copies the box, so the closure and the
  function share the one object, as Go's closure shares a variable, and the object lives as long as
  the last box. An object that nothing captures is a plain C++ local (no allocation).
- **Never captured by copy.** Phase 03 captures a variable that is never assigned by copy, and that is
  wrong for a C++ object: a member call (`v.push_back`), an operator, or a function that takes a
  reference (`std.swap(a, b)`, `bump(c)`) changes the object without an assignment, and gocpp can't see
  what a C++ call does. And whether C++ can copy the type at all (`std::unique_ptr`, `std::mutex`) is
  the C++ compiler's to say, not gocpp's. So the rule is not "box what is changed" but "box what a
  closure or a defer uses", for every C/C++ type, and the closure never copies the object.
- **The object is built in its box**, from the value of its initializer (guaranteed copy elision), not
  moved into it: `v := std.vector[int]{}` is `auto v = gp::cgo::box([&] { return
  ::std::vector<int64_t>{}; });`, `var m std.mutex` is `gp::cgo::new_box<::std::mutex>()`. So a type that
  can be neither copied nor moved (`std::mutex`, `std::atomic`) can be boxed.
- **`defer` on an object.** `defer v.clear()` or `defer h.items.clear()` keeps the box of the root
  variable in the defer frame (a copy of the box), and reads the path when the call runs; without
  that, C++ would have destroyed `v` at the end of its block, before the function's deferred calls
  run (`defer v.clear()` was an AddressSanitizer error in the audit). The arguments are evaluated
  when the `defer` statement runs, as Go does: `defer fmt.Println(int(v.size()))` prints the size of
  that moment. A temporary (`defer f().close()`) is evaluated then, before the arguments, and moved
  into the frame (or kept as a pointer, if C++ gave a reference). A C++ callable (`defer cb()`) is
  copied into the frame when the statement runs, as Go stores the func value.
  - **Locals die before the deferred calls run.** So the frame keeps alive the C++ locals that the
    call can reach, with two rules. (1) A local that the call's object or arguments mention through a
    path is kept: `defer v.front().inc()` and `defer v.at(0).inc()` keep `v` (the reference is into
    it), `defer lib.f(v.data())` keeps `v` (the pointer is into it), `defer lib.f(&v)` keeps `v`. A
    plain use of a variable in an argument copies its value and keeps nothing (`defer lib.f(h)`,
    `defer fmt.Println(int(v.size()))`, `defer show(Vec{x: n})`). (2) In a function that has a
    `defer` statement, a C++ local whose **address is taken** (`&a`, and the address of a part of it:
    `&a[i]`, `&a.f`; for the address of a part of a gocpp value, `&s.f` or `&xs[i]`, the pointer
    shares the object's owner and nothing needs holding) is boxed, and the function's
    frame holds its box until every deferred call has run (also one that was deferred before the
    local was declared, and when a panic runs them), where the pointer can **outlive the local's
    block**: the local is in the function's own block (not in an inner block or a loop), or the
    address is stored in a variable that is declared outside the local's block: `ps =
    append(ps, &a)`, `ps[0] = &a`, `m["a"] = &a`, `n.c = &a`, `saved = &a` (a package variable),
    `p := &a` with `p` declared before the loop, or a variable that holds such a pointer (`q := p`,
    `n := &Node{c: &a}`). Then `defer p.inc()`, `defer lib.f(ps[0])` and `defer n.c.inc()` are safe.
    The cost is a heap entry in the frame for each such local, and **the object is destroyed when the
    function returns, after the deferred calls, not at the end of its block**: an RAII object
    whose address escapes its iteration lives until the function ends, and memory grows with the
    iterations that escape (as Go's heap does). In an inner block or a loop the hold is **not** made for a pointer that is only
    passed to a call (`use(&g)`), used as the receiver of a call, or kept in a variable of the same
    block (`p := &a` inside the loop): that pointer dies with the block, as in C++, and a lock guard
    whose address goes to a function is still destroyed at the end of its iteration. What is **not tracked** follows C++ lifetime rules for opaque objects, and an inner-block local
    can dangle there: (1) a pointer that a **Go function** stores, where the address was only passed
    to it (`register(&a)` with `func register(p *T) { global = p }`, or `add(&r, &a)` that appends to
    `r.ps`), with `a` declared in an inner block or a loop: gocpp does not follow the address into the
    function (interprocedural escape analysis is a TODO: a local of the function's own block is
    held anyway, and so is one whose address is stored in the same function); (2) a pointer that a
    **C++ function** keeps or gives back (`p := lib.find(a)`, `v.data()` of a vector that nothing
    deferred mentions, a reference returned into an argument); (3) a pointer that a **closure**
    captures when the closure is not stored in a variable outside the block (a closure that is
    passed on to a call, or returned, takes the box of what it captures, which is the pointer's
    variable, not the object the pointer points to). A closure stored outside the block, with the
    pointer it captures (`fs = append(fs, func() { p.inc() })`), is followed.
  - A pointer or a reference into a C++ local that a **closure** keeps is the closure's to keep
    alive: the closure captures the box of every local it uses.
  - A path that starts at a **gocpp pointer** (`defer s.mu.unlock()`, `defer s.c.inc()` for an
    `s *S` with C++ fields) works as in Go: the frame keeps a copy of the pointer `s` as of the
    `defer` statement (which keeps the struct alive too), and the call goes through it.
  - With a **nil** gocpp pointer, `defer s.c.inc()` panics at the `defer` statement, as in Go (the
    pointer is checked when the frame takes its copy).
  - A difference from Go: the path of a member call that starts at a **C++ local** is read **when the
    call runs**, not at the `defer` statement. `defer p.close()` with a raw pointer `p` that is
    assigned before the function returns calls `close` on the new `p`. Write `defer lib.close(p)` (a C
    function with an argument evaluates `p` now).
  - What can't be kept is an error at the `defer` line: `defer v[0].clear()` and `defer (*p).m()` (Go evaluates the element or the target now, a
    deferred C++ call would read it later). Use a func literal: `defer func() { v[0].clear() }()`.
- **C++ destructors still run when the block of the local ends**, unless a closure or a deferred call
  holds the box: then the object is destroyed when the last holder is (a deferred call holds it until
  it has run, a closure until the closure itself is destroyed). So the output of a destructor shows
  C++'s order, not Go's. Without boxing, a local is destroyed at the end of its block, before the
  function's deferred calls run.
- **Loops.** The variable of a 3-clause `for` that a closure uses gets a new box (with a copy of the
  object) for each iteration, as Go 1.22 does; a type that C++ can't copy is an error ("a closure that
  uses the variable of a 3-clause for loop needs a copy of it for each iteration": declare the object before
  the loop). The variable of a `range` over a C++ container is the loop's own copy, boxed per
  iteration. `&i` of the variable of a 3-clause `for` is an error (each iteration has its own `i` in
  Go, which a pointer can keep; a C++ address can't): copy it into a variable of the body.
- **`go` statements** (Phase 08) use the same boxes: the goroutine's func literal copies the box of
  every C/C++ local it uses, so the goroutine, the function and any closure share the one object, and it lives
  until the last of them is done (`tests/interop/go_cxx_objects.gp`). Before Phase 08 a `go` statement is
  the phase's "not supported yet" error. Sharing an object between threads is C++'s business: a
  `std::mutex` (also in a box) protects it as in C++.

### Types that can't be copied

C++ types differ in what they allow, and Go's constructs copy, zero and assign without asking. Where
C++ can't do what Go does, the program doesn't compile, with an error at the `.gp` line; gocpp never
changes the meaning silently. Four kinds of type cover the cases:

| Kind | Examples | Can be |
|---|---|---|
| copyable | `std::vector`, `std::string`, `C.int`, a pointer | copied, moved, default-constructed |
| move-only | `std::unique_ptr`, a file handle class | moved; default-constructed if the class has a default constructor |
| no default constructor | a class whose constructors all take arguments | copied or moved; not zero-valued |
| pinned | `std::mutex`, `std::lock_guard`, `std::atomic`, a class with deleted copy and move | built in place only |

| gocpp construct | C++ | copyable | move-only | no default ctor | pinned |
|---|---|---|---|---|---|
| `x := f()`, `var x T = f()` | `auto x = f();` | in place | in place | in place | in place |
| `x := T(args)`, `x := T{...}` | `auto x = T(args);` | in place | in place | in place | in place |
| `x := y` | `auto x = y;` (copy) | copy | error: C++ "use of deleted function" (write `x := std.move(y)`) | copy | error |
| `x := std.move(y)` | `auto x = std::move(y);` | move | move | move | error |
| `var x T` | `T x{};` | zero value | default-constructed | error: C++ "no matching constructor" | error if no default constructor |
| `x = f()` | `x = f();` | assign | move-assign | assign | error (no assignment) |
| `x = y` | `x = y;` | copy-assign | error | copy-assign | error |
| `a, b := f()` (a pair or tuple) | `auto gp_t = gp::values(f()); auto a = std::get<0>(move(gp_t)); ...` | moved out of the tuple | moved out | moved out | error (a tuple can't hold it) |
| a closure uses `x` | `x` is boxed (`gp::ptr<T>`); the closure copies the box | no copy of the object | no copy | no copy | no copy |
| `defer x.m()` | the frame keeps `x`'s box | no copy | no copy | no copy | no copy |
| `defer f(x)` | the frame stores a copy of `x` (evaluated now) | copy | error: "defer copies the arguments of the deferred call" (`defer f(std.move(x))` moves) | copy | error |
| `defer f(g())`, `defer f(std.move(x))` | the frame stores the temporary | move | move | move | error: "defer stores the arguments of the deferred call in a frame, and this C++ type can be neither moved nor copied" |
| `return x` (a local, no defer) | `return x;` (C++ moves a local, or elides the copy) | move or elided | move | move | error |
| `return x, y` (several results) | `return {move(x), move(y)};` (a local that is returned once) | move | move | move | error |
| `return x` with defers, an unnamed result | `gp_r0 = move(x); goto exit;`, the exit returns `gp_r0.gp_take()` | move (`gp::result_slot`) | move | move (built in the slot) | error: "can't be moved or copied" |
| a named result `(v T)` | a variable (zero value), boxed if a closure or defer uses it | zero value | default-constructed | error (no zero value) | error if no default constructor |
| the loop variable of a 3-clause `for`, in a closure | a new box with a copy for each iteration | copy | error: "needs a copy of it for each iteration" | copy | error |
| `for i, x := range c` | `for (auto&& [i, x] : gp::range(c))`: `x` is a copy | copy | error (use the index: `for i := range c`, then `c[i]`) | copy | error |
| a parameter | by value: a copy made by the caller | copy | the caller moves (`f(std.move(x))`) | copy | error |
| a gocpp struct with such a field (`type S struct{ mu objects.Lock }`) | built like the field: `var s S`, `S{...}`, `&S{...}`, `new(S)` and a boxed `s` build it in place | in place | in place | error at the field: "this C++ type has no default constructor, so a struct field of it has no zero value" (Go's `var s S` and `new(S)` need one; use `*T` or a smart pointer as the field) | in place; copying it (`t := s`) is an error |
| `&T{...}` of a C/C++ type | `gp::cptr<T>(gp::cgo::box_as<T>(...))`: built in place in a box that the pointer owns (it converts to the raw `T*`) | in place | in place | in place | in place |
| `&s.f`, `&xs[i]` of a gocpp struct, slice or array with C/C++ parts | `gp::cptr<F>(gp::ptr<F>(owner, &...))`: shares the owner of the object | | | | |
| `&x`, `&x[i]`, `&x.f` of a C/C++ local | the raw address (the local is held, as above, when a defer is there) | | | | |

Notes on the table:
- *In place* means C++'s guaranteed copy elision: the result of `f()` is built in the new variable (or in
  its box), so even a pinned type, which has no copy or move, works. `x := f()` where `f` returns a
  reference (`T&`) is `auto x = f();`: a copy, as `auto` makes it.
- A copy of a move-only or pinned type, by assignment or by passing it as a C++ argument, is C++'s own
  error, at the `.gp` line: gcc says "use of deleted function", clang "call to deleted constructor" and
  "overload resolution selected deleted operator '='". The runtime's own helpers, which copy where Go
  would (a deferred call's arguments, a loop variable that a closure keeps, a result variable), say what to
  do instead with a message of their own.
- A local that is returned, or whose value goes into a result, is **moved** when nothing else sees it:
  it is not boxed (no closure or deferred call uses it), it is used once among the results, and it
  is not a result variable itself. A boxed local is **copied** if C++ can copy it (the closure that
  shares the box keeps its object), else moved (the closure then sees a moved-from object).
- **`gp::result_slot<R>`** (`gocpp/result.hpp`) is the result variable of a function with defers, for a result of a
  C/C++ type that has no name in the program (a named result is a variable of its own). It picks its
  storage where the C++ is compiled: a reference result is kept as a pointer; a type that is
  default-constructible and move-assignable is kept as it is, zero-valued from the start (the same as the
  `T gp_r0{};` that gocpp types get); any other type (no default constructor, or not assignable) is
  built in a buffer by the return statement. A type with no move and no copy can't be a result
  of a function with defers, because C++ returns such a type only from the expression that makes
  it (without defers, `return lib.make()` works). The slot's members are `gp_set` (also `operator=`),
  and `gp_take`; Phase 05 uses it for every result.

Source: [Phase 01 §2.8](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#28-the-opaque-pass),
[Phase 04 §2.5](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#25-values-and-operations-opaque).

## Strings at the boundary

<!-- illustration: a fragment, not a program -->
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
| string constant → any C/C++ parameter | a raw C literal `"hello"`, also for a typed constant (`const S string = "x"; cstdio.puts(S)`) |
| `string` → `std::string_view` | implicit conversion |
| `string` → `std::string` | explicit: `std.string(s)` |
| `string` → `const char*` | `C.cstr(s)`: a small **holder** object that owns a NUL-terminated copy and converts to `const char*`. It lives until the end of the statement, or as long as the variable it is in (`name := C.cstr(s)`). No copy is made when the bytes are already NUL-terminated (literals) |
| `string` → owned `char*` | `C.CString(s)` (cgo-compatible, uses `malloc`; free it with `C.free(p)`) |
| `char*` → `string` | `C.GoString(p)`, `C.GoStringN(p, n)` (cgo-compatible; any char pointer: `char*`, `const char*`, `unsigned char*`; a nil `p` gives `""`) |
| `std::string` or `std::string_view` → `string` | a conversion: `string(x)`, which copies the bytes |
| bytes | `C.GoBytes(p, n)`, or `unsafe.Slice((*byte)(p), n)` |

- Only `std::string_view` is implicit. Implicit conversions to both `std::string` and
  `std::string_view` would be ambiguous for overloaded C++ functions and would not work with
  templates.
- **The helpers have real types.** `C.CString`, `C.GoString`, `C.GoStringN`, `C.GoBytes`,
  `C.free` and `C.cstr` are rewritten into calls of a hidden gocpp package with cgo's
  declarations:

  ```go
  func C.CString(s string) *C.char
  func C.GoString(p *C.char) string
  func C.GoStringN(p *C.char, n C.int) string
  func C.GoBytes(p unsafe.Pointer, n C.int) []byte
  func C.free(p unsafe.Pointer)
  func C.cstr(s string) C.cstr // the holder, a C++ object
  ```

  So `s := C.GoString(p)` gives `s` the type `string`, `s + "!"` and `strings.ToUpper(s)` are
  fully type-checked, and a gocpp value of the wrong type is Go's error, as in cgo:
  `C.GoStringN(p, n)` with `n int` is "cannot use n (variable of type int) as C.int value in
  argument to C.GoStringN". A C/C++ value passes as it is: any char pointer goes to `C.GoString`,
  and `C.free(p)` takes a C pointer without the `unsafe.Pointer(p)` that cgo needs (that form
  works too). The results of `C.CString` and `C.cstr` are C/C++ values. A helper must
  be called (`f := C.GoString` is "must call C.GoString").
- **Where `C.cstr` can go.** Its C string lives only as long as its holder, so `C.cstr(s)` can be
  only an argument of a C/C++ function (also of a constructor, a member function or a C++
  callable) or the value of a new variable (`name := C.cstr(s)`, `var name = C.cstr(s)`, and
  `name = C.cstr(t)` for such a variable). Anywhere else (a result, a field of a C struct, a
  `*C.char` variable) it is an error that says to use `C.CString`. This also rejects holder
  variables and their copies in any composite literal, keyed or positional: neither a direct
  holder nor its `const char*` may escape through field initialization. A holder (`C.cstr(s)` or such
  a variable) is passed as its `const char*` to every C/C++ function, so it goes through C's `...`
  too: `cstdio.printf("%s", C.cstr(s))` is `::printf("%s", gp::cstr(s).gp_ptr())`.
- **The function must not keep the pointer.** gocpp can't see what a C/C++ function or object
  does with the `const char*` it gets, so one that keeps it after the statement dangles: a C API
  that stores the pointer (use `C.CString`, and `C.free` it when the API is done with it), or a
  view of it, `v := std.string_view(C.cstr(s))` (a use after scope at the next use of `v`: write
  `std.string_view(s)`, or `std.string(s)` for a copy). These follow C++'s lifetime rules, as C/C++
  values do ([pointers](#pointers-unsafe-and-slices-at-the-boundary)).
  `gocpp vet` (analyzer `cstr`) reports the calls of the functions of the typed-interop list
  (`openlog`, `getopt`, raylib's `InitWindow`, `sqlite3_bind_text` without `SQLITE_TRANSIENT`, ...) that get a
  `C.cstr`, a view that is kept, and a constant string with a NUL byte given to `C.cstr` or `C.CString`.
- **A gocpp string is no C string.** `cstdio.puts(s)` doesn't compile: in opaque mode gocpp
  doesn't know that `puts` takes a `const char*`, so the C++ compiler reports it at the `.gp` line,
  with a hint: "C++: cannot convert 'string' to 'const char*' (a gocpp string is no C string:
  pass C.cstr(s), or C.CString(s) for a copy that C keeps)". `string(p)` of a C pointer is an
  error too ("use C.GoString(p)"), as in cgo.
- **Strings that come through C++.** A gocpp string reached through a C++ value (a field of an
  element of a `std.vector[*Node]`) compares with a string constant as in Go (`nodes[0].Name ==
  "a"`), and `string(nodes[0].Name)` gives it its gocpp type.
- A string constant passed to a C++ function that takes a gocpp string (`v.push_back("x")` of a
  `std.vector[string]`, `m["k"]` of a `std.map[string, int]`) is a C literal too, which converts
  implicitly to a gocpp string: a copy of its bytes. The one cost: a C++ overload set with a gocpp
  string next to `std::string_view` or `std::string`, `f("x")`, is ambiguous for a constant (pass
  `std.string("x")` or a gocpp string variable). A gocpp string itself converts only to
  `std::string_view`, and a C `char*` never converts to a gocpp string (`C.GoString(p)` copies one).
- Typed interop (Phase 12) converts a `string` passed to a `const char*` or `std::string` parameter
  automatically, except for functions that keep the pointer, and a `string` passed to a `const wchar_t*`
  parameter too ([argument adaptations](#argument-adaptations-typed-interop)).

Source: [Phase 04 §2.6](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#26-strings-at-the-boundary).

## Argument adaptations (typed interop)

In typed mode (the default when clang++ is installed) gocpp knows the exact type of every parameter of a C/C++ function, because
clang says it, so it writes the conversion that C++ would refuse where the meaning is certain from the two types
alone. All of them are rows of **one table** (`internal/typedinterop/adapt`, written by `internal/lower/typedadapt.go`, [ADR-130](../adr/130-argument-adaptations-are-one-table-over-the-parameter-type-and-the-go-argument.md)):
the class of the Go argument, the parameter type that clang reports, and what is written.

| Go argument | Parameter | Written | Since |
|---|---|---|---|
| a `string` that is not a constant | `const char*` | `gp::cstr(s).gp_ptr()`: a NUL-terminated copy, alive until the end of the statement | 12.6 |
| a `string` | `std::string`, `const std::string&` | `std::string(s)` | 12.6 |
| a `string` (also a constant) | `const wchar_t*`, `const char16_t*`, `const char32_t*` (`LPCWSTR`) | `gp::wide_cstr<wchar_t>(s).gp_ptr()`: UTF-16 where `wchar_t` has 16 bits, UTF-32 where it has 32, alive until the end of the statement | new |
| `&x`, `x` a variable or field of a Go number type | `T*` or `const T*`, `T` another C number type | `gp::cgo::call_out<T>(&x, "x", ...)`: a temporary `T` filled from `x` when the call starts and copied back as soon as the call returns (not at the end of the statement: `if f(&n) == 0 && n > 0` sees the new `n`); a `const T*` copies in only | new |
| `&p`, `p` a variable or field of type `*lib.T` | `void**` | `gp::cgo::call_void_out(p, ...)`: the callee's pointer is stored in `p` as soon as the call returns | new |
| an untyped integer constant | an enum `E` (also `enum class`) | `static_cast<E>(3)`, if the value is one that C++ defines for `E` | new |
| an untyped number constant | a C number type | the argument as it is, after Go's check that the value fits (`300` for a `char` is an error) | new |
| a `float32` or `float64` | an integer type, or a `float` from a `float64` | `gp::cgo::as<T>(x)`: Go's conversion, truncation toward zero, defined for every value | new |
| a pointer to a C type, `unsafe.Pointer`, a gocpp pointer | `void*`, `const void*` | the pointer converts by itself | before typed mode |
| `&b`, a Go `bool` | `bool*` | the same type: the pointer converts by itself | before typed mode |
| a function | a C function pointer | a function pointer (see [C function pointers](#c-function-pointers)) | before typed mode |

**When a row applies.** Only when **every** overload that the call can reach (enough parameters, or a variadic one)
has the same row at that place, with the same detail. Then the call without the adaptation was ill-formed for each of
them, and the adapted argument has exactly the parameter's type, so C++'s overload resolution cannot pick another
function than the one you meant. A set that mixes kinds (`f(const char*)` next to `f(std::string_view)`, `f(int*)`
next to `f(long long*)`) is left to C++, as in opaque mode. A template, a function of an unknown signature, a call
with `args...` and a deferred or spawned call of a row that needs the call to end first (`void**`) are left alone
or are an error that says so.

**What is not adapted.** Each of these stays the C++ compiler's error (or your explicit conversion), because the
types do not say what is meant:

- a Go `string` to a `char*` (the callee may write: `C.CString(s)` and `C.free`), to a `std::string&`, to a `[]byte`
  parameter, or the other way round, a `char*` result to a `string` (`C.GoString(p)`);
- `&a[i]`, `&s.items[0]`, a pointer value, a pointer to a named type or to a `char`/`bool`: the address of an element is
  the start of an array to C, and a `char*` is nearly always a buffer or a string. Only `&x` of a **variable or field**
  of a Go number type is a pointer to exactly one value. A parameter that is **spelled as an array** (`float col[3]`,
  `int v[2]`) is no pointer to one value either, though clang types it `float*`; `&speed` for it is C++'s error, and the
  element of an array (`&col[0]`) is the argument. The same holds for a parameter written with the typedef of an array
  (`typedef float vec3[3]; void scale(vec3 v)`, `mat4 m`); a typedef of a pointer (`LPDWORD`) is a pointer;
- a non-constant integer for an enum (`Mode(k)` is the conversion that says that `k` is a mode), and a **typed** constant
  (`const mode int = 2` is an `int` in Go);
- a number to a `bool` parameter and a `bool` to a number parameter (C++ converts them silently; Go does not);
- a Go `int` or `uint` behind a pointer whose pointee cannot hold every value of the type in the direction that the
  callee writes (`&n` of a `uint32` for an `unsigned long*` on Linux: the copy back would cut the value): an error that
  says to declare `n` with the C type;
- an integer behind a pointer for a `double*` or the other way round: C converts neither way through a pointer;
- a Go value whose address was taken for a pointer that the callee **keeps** (a `T*` stored in a C object): the temporary
  dies when the call returns.

**When the variable is read and written.** The value of `&x` goes in when the call starts, after every other argument has
been evaluated (a call among the arguments that changes `x` is seen by the callee, as in Go), and what the callee stored is
in `x` as soon as the call has returned. A callee that panics or throws leaves `x` as the callee stored it, as a write
through a pointer in Go.

**Lifetimes.** The temporary that a row makes lives until the end of the statement that holds the call, as `C.cstr(s)`
does, and no longer: that is the whole rule. A callee that keeps the pointer after the call (a C library that stores
a `const wchar_t*`, a function that remembers the `int*` it was given) reads a dead object afterwards. gocpp cannot see
that. The names it knows (`InitWindow`, `sqlite3_bind_text`, `openlog`, ...), a method that stores a pointer parameter and a
character pointer that the callee writes are an error that says to use `C.CString` and `C.free` (`C.WString` for wide text: the
same checks hold for `wchar_t`, `char16_t` and `char32_t`); for any other function, a callee that keeps a pointer is documented to do so, so pass a value that you
own: `C.WString`, `C.CString`, `C.malloc`, or a variable of C's type that lives as long as the callee needs it, and the
program has no adaptation to make, because the argument already is a pointer.

**printf-like functions.** A parameter that clang knows as a format string (`__attribute__((format(printf, ...)))`,
`IM_FMTARGS`, glibc's `printf`) refuses a Go string that is not a constant: its `%` would be a conversion (the
format-string bug), and nothing in the types says whether that is meant. Write `f("%s", s)`; `C.cstr(s)` is the way
to say that `s` is a format. A constant string is a format as it is. The message shows the call as you wrote it with the fix (`ImGui.Text("%s", label)`). (The functions are found by asking
clang to check a call with a deliberately incomplete format, so any compiler-known format attribute counts; a *method*
with the attribute is not found yet: its string is converted as before.)

Messages:

```text
main.gp:16:24: say is printf-like (its parameter 1 is a format string), and text would be read as a format: a % in its text is a conversion (the format-string bug); pass it as an argument, say("%s", text), or write C.cstr(text) when it is a format on purpose
main.gp:16:28: add_one takes a pointer to unsigned long that it may write, and &n points to uint32 that cannot hold every unsigned long (the copy's value would be cut when it is stored back): declare n with the C type (C.ulong)
main.gp:14:32: level_value: the constant 300 is outside the values that C++ defines for the enum Level (0 to 255): pass an enumerator by name
main.gp:14:30: cannot use 200 (untyped int constant) as C.char value in argument to take_char (overflows)
```

Opaque mode (a machine without clang++, or `-interop=opaque`) does none of this: the same call is the C++ compiler's own error at the `.gp` line, as before
(`cannot convert 'const char*' to 'const wchar_t*'`, `invalid conversion from 'int' to 'Mode'`). It is unchanged by the
adaptations. `gocpp check`, `gocpp vet` and the editor report the arguments that the table refuses, with the words and the
place of the build ([ADR-134](../adr/134-the-argument-adaptation-errors-are-decided-by-one-package-that-the-lowering-the-checker-and-the-editor-use.md)):
a constant outside the parameter's range, an enum constant outside its values, a runtime string at a format place, a
string a function keeps, an address that cannot be converted. They are errors with either C++ compiler. The editor has them
when its background analysis of the package ends, not at the first keystroke. A package that declares a generic function or type is
not adapted and not checked for them; the shapes that the table does not adapt are the C++ compiler's error at build time.

### The C number types are Go's

`C.int` **is** `int32`, `C.uint` is `uint32`, `C.longlong` is `int64`, `C.ulonglong` is `uint64`, `C.float` is
`float32`, `C.double` is `float64`, `C.bool` is `bool`, `C.char` and `C.schar` are `int8`, `C.uchar` is `uint8`,
`C.short` is `int16`, `C.ushort` is `uint16`, and `C.long` and `C.ulong` follow the target (`int64` and `uint64` on
Linux, `int32` and `uint32` on Windows, where `long` has 32 bits). They are the same type, not types with a conversion
between them: `var x C.int = n32` and `x + n32` need no conversion, a gocpp function that takes a `C.int` takes an
`int32`, and `int32(x)` and `C.int(n32)` stay legal (a conversion between identical types). Go's `int`, `uint` and
`uintptr` are **not** aliases (they are 64 bits and spell no C type), nor is a named type over a fixed-size
type (`type Count int32`), nor `C.size_t` and `C.wchar_t`: those are still the mismatches above (`x + n` with
`n int` is "mismatched types C.int and int").

This is the checker's rule, so it holds in **both modes** and needs no clang. What does not change, and where the
identity stops: the C++ side is what it was (`C.int` is `int`, which is `int32_t`; `C.longlong` is `long long`,
which is not `int64_t` on Linux, where that is `long`; the two convert by themselves), so `fmt.Printf("%T", x)`
prints `int32` for a `C.int`, and `int` for a `C.long` on Linux (gocpp's `int` is also `long`); an interface that
holds a `C.int` holds a C value with the descriptor of its C++ type, so `v.(int32)` is false for it (write
`any(int32(x))`); a named type over a C number (`type Count C.int`) is its own type, as in Go. A value whose C type
gocpp does not know in opaque mode (a function's result, the result of an operator on C values, where C++'s
promotions decide the type: `x*n`) is still a C/C++ value that needs `int32(x)`.

### Numbers at a call

What happens to a Go number that is passed by value to a C/C++ number parameter. "Opaque" is C++'s implicit
conversion (no clang, no parameter types); "typed" is the table above.

| Go argument → parameter | Opaque | Typed | `gocpp vet` |
|---|---|---|---|
| the same type (`int32` → `C.int`, `float32` → `float`) | silent | silent | silent |
| a wider integer of the same signedness, an unsigned one into a wider signed one (`int32` → `long long`, `uint8` → `int`) | silent | silent | silent |
| `float32` → `double`, any integer → a floating type | silent (C++ rounds) | silent | silent |
| a narrower integer: `int`, `int64`, `uint32` → `int`, `short`, `char` | wraps (C++20: modular, as Go's conversion) | wraps | a warning when the value is not provably in range |
| a different sign, same width (`int32` → `unsigned`) | wraps | wraps | a warning, as for narrowing |
| a float → an integer | C++'s conversion: **undefined** when out of range | `gp::cgo::as<T>`: Go's conversion, no undefined behavior | a warning |
| `float64` → `float` | C++'s: undefined when out of range | `gp::cgo::as<float>` | a warning |
| an untyped constant that fits | silent | silent | silent |
| an untyped constant that does not fit (`300` → `char`, `-1` → `unsigned`, `2.5` → `int`, `1e60` → `float`) | C++ wraps or truncates: **no error** | an error with Go's words (`overflows`, `truncated`) | (the error stops the build) |
| a number → `bool`, a `bool` → a number | C++ converts | C++ converts | silent |

Extern declarations (`//gocpp:extern`, ADR-128) stay strict: a Go `int` or `uint` is an error
there, because it is 64 bits and spells no C type; write `C.int` (now the same as `int32`), `C.long`, `int64`.

The `gocpp vet` column is the design of ADR-130; the warnings for implicit narrowing at a call are **not built yet**
(vet has no parameter types: only the explicit `C.int(n)` conversion is reported, by the `narrowing` analyzer).

## Results that are errors (typed interop)

A C/C++ function or method whose result type is the typedef `HRESULT` returns a Go `error` in typed mode
([ADR-137](../adr/137-hresult-results-are-go-errors-by-one-row-of-a-result-table.md)). It is the one row of the **result table** (`internal/typedinterop/adapt/results.go`), which is
separate from the argument table above and applies around the call.

<!-- illustration: Windows only: not built here -->
```go
if err := swapChain.ResizeBuffers(0, w, h, d3d11.DXGI_FORMAT_UNKNOWN, 0); err != nil {
	return err // HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)
}
```

- **`nil` is `SUCCEEDED(hr)`**: the code is not negative. `S_OK`, `S_FALSE` and success codes such as
  `DXGI_STATUS_OCCLUDED` are `nil`. A negative code is an error whose text is `HRESULT 0x` and eight hexadecimal digits,
  with the name of a well-known code (`E_FAIL`, `E_INVALIDARG`, `E_OUTOFMEMORY`, the `DXGI_ERROR_` and `D3D11_ERROR_`
  codes) and `Win32 error N` for the codes of `HRESULT_FROM_WIN32`.
- **The code stays reachable.** The error has the methods `Error() string` and `HResult() int32`, and its type has no
  name in Go, so a program asks for the method: `var h interface{ HResult() int32 }; errors.As(err, &h)`. Two errors of the
  same code are equal (`err == other`, `errors.Is`).
- **The code of a success.** Take the HRESULT itself with a conversion to a number: `int32(sc.Present(1, 0))` or
  `C.long(...)`. The call is then the C++ call as it is, and `int32(sc.Present(1, 0)) == int32(d3d11.DXGI_STATUS_OCCLUDED)`
  tells a covered window from `S_OK`. A call that is a statement, or the operand of `defer` or `go`, drops the result as in
  C++ and is not wrapped.
- **Only the typedef named `HRESULT`.** The rule is on the type that the declaration wrote: exactly the name `HRESULT`
  (written without a qualifier, no template arguments; a `typedef` of the name inside a namespace is not told from the global
  one) for a signed `int` or `long`. `LRESULT`, `NTSTATUS`, `DWORD`, `BOOL` and a typedef of
  `HRESULT` of your own stay the numbers that they are. `const HRESULT f()` is an `HRESULT`.
- **Old code.** `hr := f(); if hr != 0` is now an error, with a line that says what to write instead:

```
main.gp:11:11: cannot convert 0 (untyped int constant) to type interface{Error() string}
	the call returns an HRESULT, which is an error here (nil when the code is not negative): compare it with nil; the code is int32(call) on the call itself, or HResult() through errors.As
```

  It is an error with either C++ compiler (a gocpp rule, not clang's). `-interop=opaque` keeps the number: an HRESULT is
  what the C++ function returns, and the program is not the same.
- **A variable made from the call is an `error`, typed once.** The call is an `error` expression for the checker (it is
  wrapped in `error(call)` before the check is done, ADR-137 (g)), so `hr := f()` is an `error` variable and every use is Go's
  own: `hr.Error()` is a string (concatenation, `strings` functions, `hr.Error()[:4]`, `range hr.Error()`), `hr = nil`,
  `append(errs, hr)` and `append(errs, f())`, a `map[error]string{hr: "dev"}` key, `errors.Is/As`, `log`, `panic`, `hr.(T)`,
  `switch hr { case nil: }`, `fmt.Errorf("%w", hr)`, and Go errors are assigned to it (`hr = errors.New("x")`). What Go
  refuses for an `error` is refused, with Go's words: `cannot use hr (variable of type error) as int32 value in assignment`,
  `hr += 1`, `hr++`, `hr[0]`, `invalid case 0 in switch on hr`, `var n C.long = hr` (the same words from `gocpp check` and
  `gocpp build`). The code of a variable is `errors.As` with `HResult()`; `int32(hr)` is refused. One call is the error or
  the number, not both: `nil` hides `S_FALSE`, and the code of a success is `int32(call)` on the call.
- **Not detected: a call through a function pointer held in a local variable** (`fn := lib.get_fn(); fn(1)`) and a call of
  a `std::function<HRESULT(int)>`: they return the number, because the compiler's type of such a call has lost the typedef
  name. Call the pointer where it is declared, or through a struct member or a global. A function that declares
  `auto f() { return g(); }` is detected through `g`'s result, and a `typedef long HRESULT` inside a namespace is not told
  from the global one.
- **Cost.** A program with such a call is analysed once more before the check is done (the hook that tells which calls
  return an `HRESULT`; its compiler runs are the ones the build asks for, so they are cache hits); a program without one
  pays what it paid before. In generated C++ a successful call does not allocate: `gp::hresult_error(call)` is an inline
  test that gives the nil interface, and a failure allocates one small box.
- **A call that has an argument the probe cannot spell** (`&n` of a Go number variable, `&p` of a pointer variable for a
  `void**` parameter, both adapted by the argument table) is typed from the callee's declarations: the result is the same
  error, and `gocpp check` reports its misuse as for any other call.
- **The editor** (`gocpp lsp`) shows no diagnostic for a valid program that uses these errors (also for map literals that
  have the error as a key, with elided values), but it does not run the HResult checks at all (they need the call typed as
  `error`): the misuse errors above and the hint are silent there and shown by `gocpp check` and `gocpp build`. Verified
  silent: `hr += 1`, `hr++`, `hr[0]`, `hr + 1`, `hr != 0`, `if f() < 0`, `switch hr { case 0: }`, `hr = "x"`, `hr = 5`,
  `var n int32 = hr`, `var n C.long = hr`, a number returned or passed where `error` is wanted, `map[string]int{hr: 1}`.
  What does work there: a wrong-typed value in a map literal whose key is a C/C++ value, with Go's words (`map[error]string{hr: 5}`,
  `map[any]int{cv: "x"}`), and the errors that do not depend on the type of the call (`declared and not used`).
  A variable declared and not used inside a function literal in a map-literal value next to a C/C++ key may go unreported
  when a false one (a variable that only a nested value of that literal uses) comes first in the same value; check has
  only the first error of such a value.
- **COM methods** are calls like any other: `swapChain.Present(...)` on a pointer to an interface is the virtual call, wrapped
  the same way.

## Pointers, unsafe and slices at the boundary

<!-- illustration: a fragment, not a program -->
```go
var st stat.struct_stat
stat.stat(C.cstr(path), &st) // &st: a raw C++ address of a C++ value

buf := make([]byte, 4096)
n := unistd.read(fd, unsafe.Pointer(&buf[0]), 4096) // C only borrows the pointer

nums := []C.int{3, 1, 2}
lib.print_all(nums) // a C++ function that takes std::span<int>

argv := unsafe.Slice(lib.get_argv(), 3) // a []*C.char over C's array: a view, C owns it
name := rec.name[:]                     // a C array member, sliced: a []C.char over C's memory
```

- **`unsafe` keeps gocpp memory alive, like Go.** `unsafe.Pointer` is a `gp::ptr<void>`. Made
  from a gocpp pointer, it shares that object's owner, and it converts to `void*` for C.
  `unsafe.SliceData(s)` returns a pointer that shares the slice's array, `unsafe.StringData(s)`
  one that shares the string's bytes, and `unsafe.String(p, n)` and `unsafe.Slice(p, n)` keep
  `p`'s owner. `unsafe.Add` works on both kinds of pointers. This is what ported Go code needs:
  `strings.Builder.String` is `unsafe.String(unsafe.SliceData(b.buf), len(b.buf))`, and a
  string that is a view of the buffer must keep the buffer alive after the Builder is gone
  (an ASan test, [tests/interop/unsafe_builder.gp](../../tests/interop/unsafe_builder.gp), keeps
  it fixed).
- Only pointers that come **from C** are views that own nothing: a C pointer converted to
  `unsafe.Pointer`, and what `unsafe.Slice` and `unsafe.String` make of a C pointer. A string over
  C memory is never taken for a C string either (`C.cstr` copies it, as no NUL is promised after
  its bytes). `unsafe.Pointer(uintptr)` owns nothing too: go vet's valid pattern
  `unsafe.Pointer(uintptr(p) + off)` points into the object that `p` keeps alive.
- **Conversions follow cgo's rules.** A C pointer converts to another pointer type or to a
  number only through `unsafe.Pointer`: `(*C.char)(unsafe.Pointer(p))`,
  `uintptr(unsafe.Pointer(p))`; `(*byte)(p)` of a C pointer is an error that says so. A C `void*`
  is an `unsafe.Pointer` for this (`(*byte)(stdlib.malloc(8))` works). A C function's address
  goes through `unsafe.Pointer(lib.fn)` and back as `lib.callback_t(p)` (`std::bit_cast`: a cast
  between function and object pointers is only conditionally supported, and g++ with
  `-Werror=conditionally-supported` rejects it). `(*[1 << 20]C.int)(p)[:n:n]` makes a slice over
  C memory, as it did before `unsafe.Slice`.
- **Checks.** `unsafe.Slice` and `unsafe.String` panic as Go's do ("runtime error:
  unsafe.Slice: len out of range", "... ptr is nil and len is not zero"). `(*T)(p)` to a type
  with an alignment above 1 checks the address, and a misaligned one ends the program with Go's
  checkptr message, "fatal error: checkptr: misaligned pointer conversion": Go reads such a
  pointer without a word on amd64 (its checkptr mode reports it), but here the read would be
  undefined behavior. `-nochecks` turns the check off.
- `[]T` converts implicitly to `std::span<T>` and `std::span<const T>` (the `len` elements, not the
  capacity; a sub-slice and the result of `unsafe.Slice` too).
- A gocpp `*T` passed where C/C++ expects `T*` converts implicitly, and is only **borrowed**. C
  must not keep it after the gocpp object dies. This is the same rule as cgo's pointer-passing
  rules. A gocpp pointer cannot go through a C function's `...` as it is (`printf`, `sscanf`,
  `ioctl`: a class object there is an error), so the call of a C/C++ function that is not a
  template writes `p.gp_get()`, the raw address, for each argument that is a gocpp pointer or an
  `unsafe.Pointer` or owning pointer (below): `stdio.sscanf(s, f, &a, &b)` stores into the gocpp
  variables `a` and `b`.
- **`&x` of a C/C++ value.** `&x` of a variable, `&s.f` of a field, `&v[i]` of an element and
  `&*p` are raw C++ addresses. **C/C++ values follow C++ lifetime rules.** gocpp's safety covers
  gocpp types, not C/C++ types. A local ends with its block (not when nothing points to it any
  more, as in Go), so:
  - `return &x` of a local is an error;
  - `&x` of the variable of a 3-clause `for` loop (Go gives each iteration its own) is an error;
    `&x`, `&x[i]` and `&x.f` of a local in a function with `defer` statements follow the rules of
    [closures and defers](#cc-objects-in-closures-and-defer) (a local of the function's own block
    is held until after the last deferred call; an inner-block or loop local only when its address
    is stored outside the block);
  - every other pointer to a local is the program's to keep valid: `g = &x`, `h.p = &x` and
    `p := &x; return p` compile, and dangle after `x`'s block ends (ASan finds it). A C function
    must not keep `&x` after that either (a callback's user data, `sqlite3_bind_text` with
    `SQLITE_STATIC`).
    `gocpp vet` (analyzer `cpplocal`) reports the stores of `&x` that outlive `x` which it can see in
    one function: a package variable, a field or element reached through a pointer parameter, a variable
    declared outside the block (in a function without `defer`), a channel the function does not own,
    and a result.

  `&lib.global` is the address of a C/C++ global, which lives as long as the program.
- **Pointers that own what they point to.** A `*T` of a C/C++ type that gocpp code declares (a
  variable, a parameter, a result, a field of a gocpp struct, an element of a slice, array or map)
  is a `gp::cptr<T>`: a gocpp pointer that owns or aliases what it points to, as Go's pointer
  does, converts to the raw `T*` that C and C++ take (borrowed, as above), and is assigned a raw
  pointer that comes from C (which owns nothing). `&T{...}` of a C/C++ type (built in place in a
  box, so a type that can't be moved works: `m := &std.mutex{}`), `&s.f` of a field of a gocpp
  struct and `&xs[i]` of an element of a gocpp slice or array are `gp::cptr<T>` too, and share the
  owner of the object they point into. So Go's flows work:

  ```go
  var head *lib.Node
  for i := 0; i < 3; i++ { // each node lives on as the next node's next
  	n := &lib.Node{v: C.int(i), next: head}
  	head = n
  }
  p := &xs[1]
  xs = append(xs, 4, 5, 6, 7) // the old array lives on: *p is still valid
  w := Wrapper{h: &lib.T{}}   // a pointer in a struct, a slice, a map, a result
  b := &lib.Node{v: 2, next: lib.make_list(3)} // a node over C's list: a raw pointer assigned
  for p := b; p != nil; p = p.next { ... }     // p.next is a raw lib.Node* from C
  ```

  `unsafe.Pointer(p)` of such a pointer, `unsafe.Slice(p, n)` of it, a closure that captures it and
  a `defer` of a C function with it share its owner. A `cptr` costs 16 bytes, prints as an address,
  is a map key (`map[*lib.T]int`, `map[*lib.T]bool`: it compares and hashes by address), and
  compares with `nil` and with other pointers. A struct with a `*lib.T` field cannot be a map key
  (Phase 12: `==` on values with C parts).

  **Stores into the members of a C struct, and reads back.** The members of a C struct
  (`lib.Node.next`, `lib.Holder.data`) are raw pointers, but a store into one keeps the object alive
  for as long as the struct lives, when a gocpp pointer owns the struct (`n := &lib.Node{}`, a
  parameter, a field, an element, a result: anything a `gp::cptr`), and a read of it gives a
  pointer that owns the object again:

  ```go
  n := &lib.Node{v: 1}
  n.next = head              // head's object lives as long as n does
  n.next = &lib.Node{v: 2}   // a temporary: kept by n
  s.node.next = p            // through a field, an element (ps[i].next), a nested struct (n.a.b.next)
  h.slots[i] = p             // an element of an array member
  h.data = unsafe.Pointer(p) // a void* member
  n.next = nil               // drops what the member kept; so does a pointer from C
  head = head.next           // pop: the second node is head's now; the first one goes
  next := cur.next           // next owns the node: cur.next = prev doesn't free it
  prev.next = cur.next       // unlink; a.next, b.next = b.next, a.next swaps
  ```

  A store replaces what the same member kept before, so a loop that stores into one member holds
  one object (`gp::cgo::store_keep`, which records the owner in the box of the struct, by the
  member's address). The literal `&lib.Node{next: head}` keeps `head`'s object too. A raw `T*` of
  a struct that gocpp made with `&T{...}` converts to a pointer that owns it (the runtime knows
  those structs by address), so the pointers that come out of members, `p = p.next`, `return
  cur.next`, `x := cur.next`, are as good as Go's; the cost is a hash lookup per conversion. A
  struct that is a C value in a gocpp struct (`s.c.next = p` with `c lib.Node` a field of `*S`) or an
  element of a gocpp slice (`nodes[i].next = &nodes[j]`) is owned by the block of the struct or the
  slice: what it keeps is dropped when the block is found dead, a little after (not at once), and a
  pointer into the same slice needs no keeping. A copy of a C struct that is a variable of the
  block (`m := *n`) holds `n` for the block.

  Where no gocpp pointer owns the struct, nothing could keep the object alive, and the store is an
  error: `var n lib.Node; n.next = head` (declare `n := &lib.Node{}`), `p := lib.alloc(); p.next =
  head` (a struct from C), `*pp = head` (a `**T`), `lib.global = head` (a C global). A store through
  a raw pointer member (`head.next.next = tail`, where `head.next` is a raw `Node*`) is a C++ error
  that says so. A gocpp pointer stored into a struct that C allocated and gave to a `*T` variable
  (`func attach(p, h *lib.Node) { p.next = h }` with `p` from C) can't be kept either, and it is a
  fatal error at run time, as cgo's pointer checks are: allocate the struct with `&T{...}`. A
  pointer from C, `nil` and `&x` of a C local own nothing and can be stored anywhere. What stays
  unchecked, with **C++ lifetime rules**: what a C function returns (`x := lib.f()` is a `T*`), the
  pointer to a pointer of C memory (`**C.char`), the arguments of C++ templates (`std.vector[*lib.T]`
  holds raw `lib::T*`, and `push_back(p)` keeps nothing), a struct value built from a literal
  (`n := lib.Node{next: head}`) or copied into a field, a slice or a longer-lived variable
  (`g.saved = *n`), a copy `m := *n` that a closure captures and that outlives its block (the hold
  lives only for the block), a slice of C structs that GROWS (`append` copies its elements into a
  new array, and what `nodes[0].next = &lib.Node{...}` kept is tied to the old block: it is dropped
  at a later purge, so the failure comes late and not every time), and `&x`, `&v[i]` and `&c.f` of C/C++
  values, which are raw addresses.
  Pointer arithmetic (`p - q`) exists only on raw pointers from C (`p := C.CString(s)`), not on a
  declared `*C.T`, as Go has none.
  A `*C.T` variable's address, `&db` as the argument of a `T**` parameter (`sqlite3_open(name, &db)`),
  is a `T*` that the function writes and `db` gets when the statement ends (`gp::cgo::out_ptr`).
- **Slice expressions of C values.** A C array member or global, or a `std::array`, is sliced
  as in cgo: `a[lo:hi]` and `a[lo:hi:max]` make a gocpp slice over the C++ memory (it owns
  nothing: the array must outlive the slice), with Go's bounds checks. A C pointer has no length
  (`p[1:]` is an error: use `unsafe.Slice(p, n)`), and a C++ container has no slice expression.

- **Sizes.** `unsafe.Sizeof`, `Alignof` and `Offsetof` of a C/C++ value or of a type with a C/C++
  part (`unsafe.Sizeof(C.int(0))`, `unsafe.Sizeof(rec)`, `unsafe.Offsetof(rec.vals)`, a gocpp
  struct with a C field) are the C++ compiler's numbers, `sizeof`, `alignof` and `offsetof`, as
  values of type `uintptr`: `C.malloc(C.size_t(unsafe.Sizeof(C.struct_x{})))` works. They are not
  Go constants (a constant context says "is not constant" and why), and the operand is not
  evaluated, as in Go. The sizes of types without a C part stay Go constants (and differ from
  Go's: [D15](language.md#d15-different-sizes)).
- **Out-parameters.** `lib.alloc(&p)` with a variable `p` of type `unsafe.Pointer` (or `&ps[i]` of an
  element) and a parameter `void**` (cgo's `C.f(&p)`), and `sqlite3_open(name, &db)` with `db`
  declared `*sqlite3.sqlite3`, pass a pointer that the function writes, and the variable gets it when
  the statement ends; a function that writes nothing leaves the variable as it was.
- **Limits.**
  - A pointer that is stored as a number or from C memory owns nothing:
    `unsafe.Pointer(uintptr(unsafe.Pointer(&x)) + off)` in one expression is Go's valid pattern
    (the object is alive while `x` is), but a stored result (`q := unsafe.Pointer(uintptr(p) + 8)`)
    keeps nothing alive after `p` and `x` are gone, where Go's garbage collector would.
  - `(*T)(p)` of an `unsafe.Pointer` to a `T` of alignment above 1 aborts when the address is not
    aligned, also for layouts that C allows: a `#pragma pack(1)` struct's field
    (`(*C.int)(unsafe.Pointer(&r.x))`) or `(*uint32)(unsafe.Pointer(uintptr(buf) + 1))`.
    Go reads these silently on amd64; the same read here would be undefined behavior. Copy the
    bytes (`copy`, `C.memcpy`) or read through the packed struct's own fields instead.
  - What is raw follows C++ lifetime rules, as `&x` of a C/C++ local does: what a C function
    returns, `**C.T`, the pointers inside C++ containers and templates, a struct value built from a
    literal, and the raw addresses `&x`, `&c.f` and `&v[i]` of C/C++ values (a C local, a field of
    a C struct, an element of a C++ container or a C array): the program keeps them valid.
    `return &x` of a C/C++ local is an error. A gocpp pointer to a C type (`*lib.T`, a
    `gp::cptr`) is not raw: it can be returned, stored and appended, and stored into a member of a
    struct that a `*T` variable owns (above).
  - A pointer stored into a member keeps its object alive until the struct dies or the member is
    stored again. Two structs that point at each other (a doubly linked list, a cycle) never die:
    the owners form a cycle, and gocpp has no garbage collector (break it with `n.prev = nil`).
    Storing a gocpp pointer into C memory (a struct that C allocated, reached through a `*T` that
    C returned) is a fatal error ("cannot keep the object alive"), as cgo's pointer checks are.
  - A list of C structs costs more than a list of gocpp structs: each `&T{...}` is registered by
    address, and each stored pointer is recorded (about 0.7 s of CPU per million nodes built and
    freed, against 0.1 s; the table is small and sharded, with locks that threads share only when
    they use the same shard).
  - In generic code (Phase 07), a type parameter that is a pointer to a C type will be a `cptr` too.

Source: [Phase 04 §2.7](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#27-pointers-unsafe-and-slices-at-the-boundary).

## Callbacks and panics

<!-- illustration: a fragment, not a program -->
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
| a C function pointer `void (*)(int)` (a parameter, a variable, or a field of a C struct) | a function of the package, by its name: `onEvent`; or a func literal that captures nothing |
| `std::function<…>`, a template callable (the comparator of `std.sort`) | any gocpp func: a function, a func literal, a closure that captures variables, a func variable |
| `void* userdata` plus a callback | a gocpp pointer as `unsafe.Pointer` (a closure too: `unsafe.Pointer(&f)`); the callback gets it back as an `unsafe.Pointer`. `cgo.NewHandle(v)` (package `runtime/cgo`, Phase 09: not built yet) is the version for data that C keeps |

`examples/sqlite` passes callbacks to `sqlite3_exec`; `tests/interop/callbacks*.gp` has every form
below.

### C function pointers

<!-- illustration: a fragment, not a program -->
```go
// A comparator for qsort: its parameters are the void* pointers of C's type.
func byValue(a, b unsafe.Pointer) C.int {
	x, y := *(*C.int)(a), *(*C.int)(b)
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}

stdlib.qsort(unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, byValue)

// A callback table: function pointers in the fields of a C struct, called by C later.
ops := lib.ops{apply: addUser, done: finish, user: unsafe.Pointer(&state)}

// A variable of a C function pointer type holds a callback; gocpp calls it like any C function.
var apply lib.apply_t = addUser
apply(unsafe.Pointer(&n), 3)
```

- **What converts.** A function of the package (`onEvent`) and a func literal that captures no
  variable become C function pointers wherever a C/C++ value takes them: an argument of a call,
  an assignment to a C variable or field (`ops.apply = f`), a declaration with a value, an
  element of a literal of a C type. Each function gets one static **trampoline**
  (`gp::cgo::fn<&f>()` in `runtime/include/gocpp/callback.hpp`), so the pointer is valid for the whole run: C may keep
  it, call it later, or call it from another thread.
- **The trampoline** converts the arguments from C to the types of the gocpp function, calls it, and
  converts the result back. A `void*` is an `unsafe.Pointer` that owns nothing (the address is the
  one C passed), a `T*` is a `*C.T` (a `gp::cptr<T>`, owning nothing), a `T**` stays a raw
  `**C.T`; C numbers, enums and structs are themselves. The number of parameters and the result
  must match the C function pointer type (a C `void` is a gocpp func without a result): a
  mismatch is an error at the line.
- **What does not convert.** A closure that captures variables, and a func value that is only known
  when the program runs (`cb := onEvent; lib.set_callback(cb)`): a function pointer is the address of
  code, and neither has code of its own. Both are an error at the `.gp` line, "a func with captured
  variables, or a func value that is only known at run time, can't be a C function pointer": pass the
  function by its name, and give the state to it through the user-data argument (below). **Variadic
  callbacks** (`int (*)(const char*, ...)`) are an error too: C's `...` has no types for the
  arguments.
- **A result that is a pointer** must point to memory that outlives the callback: C memory
  (`C.CString`, `malloc`) or a package variable's. A pointer into gocpp memory that only the callback
  holds (`&buf[0]` of a local slice, `&T{}`) would dangle in C when the callback returns, so it is
  a fatal error that says so (cgo's rules forbid it too).
- **A callback may hand back a pointer that C passed** (`func same(u unsafe.Pointer) unsafe.Pointer {
  return u }`), also one into the struct it points to: that object is alive in the caller for the whole
  call, also when gocpp made it (`&T{}`). A pointer to an object that only the callback holds is the
  fatal error above.
- **Types of the parameters.** A `void*` parameter of the C type must be an `unsafe.Pointer` in the
  gocpp function; `*C.int` for a `const void*` is an error at the line that says so (the same for a
  `*C.T` of another pointee type). A callback that returns a C function pointer type
  (`func get() lib.unary_t { return neg }`) and a gocpp function or slice with a C function pointer
  type (`func run(f lib.apply_t)`, `append(fs, f)`) take a function of the package too.
- **Not through `...`.** A func passed to a variadic C function in its `...` part (`vcall(3, f)`)
  is an error at the line: the C++ message says an object of a non-trivially-copyable type can't go
  through `...` (a callback is deliberately one). A `...` has no type to convert the func to.
- **Limits.** The checks of the signature (the number of parameters, `void` or not, variadic) are the
  C++ compiler's, so they come as `C++:` errors with a gocpp message at the line; Phase 12's typed
  interop can report them from the C types before the compiler runs. A method value (`obj.method`),
  a function of another gocpp package and an instance of a generic function (Phase 06 and 07) are
  C++ callables but not function pointers yet. An API without a user-data parameter (`qsort`,
  `signal`, `atexit`) can only call a function of the package: it finds its state in package
  variables, as a C callback would. A **nil func variable** passed for a `std::function` parameter
  gives a non-empty `std::function` (`if (sf)` is true) whose call panics with Go's nil
  dereference; pass `nil` itself for an empty one.
- **Cost.** Every `void*` or `T*` argument of a callback becomes a gocpp pointer that shares
  `gp::foreign_owner()` (step 4.10): an atomic count up and down on one shared cache line. A
  comparator for `qsort` of a million `int`s at `-O2` took 140 ms in C, 760 ms in gocpp with
  `unsafe.Pointer` parameters that it only reads through `.gp_get()`'s equivalent, and 1.2 s with
  `(*C.int)(p)` conversions on top (each conversion copies the owner again). Callbacks that C calls
  millions of times are several times slower than C's own, and from many threads they contend on that
  count. Phase 15 can give callbacks a borrowed pointer type.
- **Threads of C's own.** A callback on a thread that C created runs on that thread's stack, with
  no stack-overflow report (a small stack overflows as a plain SIGSEGV; the report's alternate stack
  exists only for gocpp's threads), and the thread's goroutine record is never freed (step 8.1:
  "never freed"), so a library that creates a thread per request leaks one small record for each.

### User data

C libraries pass a `void*` to their callbacks so that a callback can find its state. It works as
in cgo (`examples/sqlite`'s `countRow`):

<!-- illustration: a fragment, not a program -->
```go
func countRow(arg unsafe.Pointer, _ C.int, _ **C.char, _ **C.char) C.int {
	*(*int)(arg)++
	return 0
}

count := 0
sqlite3.sqlite3_exec(db, "SELECT * FROM fruit", countRow, unsafe.Pointer(&count), nil)
```

- The pointer is **borrowed**, like every gocpp pointer that goes to C: the gocpp object must stay
  alive while C may call back. For a call that returns after the last callback (`sqlite3_exec`,
  `qsort_r`) the variable is enough. When C **keeps** the user data after the call returns (a
  handler registered for later), gocpp code must keep the owner in a package variable or a table
  until C has dropped it. Go's `cgo.Handle` (the package `runtime/cgo` comes with Phase 09, so it
  is not available yet) is that table: a handle is an integer that fits the `void*`
  (`unsafe.Pointer(uintptr(h))`), with an explicit `Delete`; nothing in the callback path has to change
  for it, because the trampoline passes the `void*` through unchanged. Until then a package-level
  `map[int]*State` and an integer key in the `void*` do the same.
- **A closure through the user pointer.** A closure can't be a function pointer, but it can be the
  user data: keep it in a variable, pass its address, and let one named callback call it. This is
  how a closure that captures variables reaches a C API with a `void*` argument:

<!-- illustration: a fragment, not a program -->
```go
base := 10
f := func(x C.int) C.int { return x*2 + C.int(base) }
lib.each(4, callFunc, unsafe.Pointer(&f))

func callFunc(user unsafe.Pointer, x C.int) C.int {
	f := *(*func(C.int) C.int)(user)
	return f(x)
}
```

  The variable `f` must live as long as C may call (the rule above).

### C++ callables

Where a C++ API takes a `std::function` or a template parameter, no trampoline is involved: the gocpp
func is passed as a callable that C++ calls, copies and stores. A closure is shared by its copies, so
a `std::function` keeps it alive for as long as it lives itself:

<!-- illustration: a fragment, not a program -->
```go
var total int
var b cbx.Button                              // keeps a std::function<void(int)>
b.on_click(func(x C.int) { total += int(x) }) // the closure lives in the button
b.click(5)

std.sort(v.begin(), v.end(), func(a, b int) bool { return a > b })
n := std.count_if(v.begin(), v.end(), func(x int) bool { return x > limit })

h := std.function[func(C.int) C.int](triple) // std::function<int(int)>
```

- `std.function[func(C.int) C.int]` is `std::function<int(int)>`: C++'s function type. For any other
  template a func type is a gocpp func (`std.vector[func(int) int]` is a vector of
  `gp::func<int64_t(int64_t)>`).
- A C++ function that takes a plain function pointer needs a function of the package or a func literal
  without captures, as C does. An overload set with both a `std::function` and a function pointer
  overload can be ambiguous for a gocpp func.

### Panics

- **Panics never unwind through C code.** GCC and Clang emit unwind tables for C on x86-64 Linux, so
  a panic inside a C callback would unwind through the C frames and silently skip the C library's
  own cleanup (locks, memory, sqlite state). So every trampoline is `noexcept`: **a panic, or any
  C++ exception, that leaves a callback ends the program** like an unrecovered Go panic: `panic:
  <value>`, the goroutine's trace (with the C frames in it), exit code 2. The C++ runtime looks for
  a handler first, finds the `noexcept` trampoline, and calls `std::terminate` before it unwinds a
  frame, so no C frame is touched and no C++ destructor in them runs. It is the mechanism of
  [C++ exceptions](#c-exceptions) below, with the same report.
- **Inside the callback everything works as usual.** The callback's own deferred calls run when it
  panics, before the program ends, and a `recover()` in the callback (called by a deferred function of
  the callback itself) stops the panic: C sees the callback's normal return value
  (`tests/interop/callback_recover.gp`).
- **What Go does differently** ([D27](language.md#d27-a-panic-that-leaves-a-callback)): Go unwinds the
  panic through the C frames to the Go code that called C, runs the deferred calls on the way, and a
  `recover` there can stop it. Here the deferred calls of the functions outside the C call do not run
  and cannot recover it; a callback that must not end the program recovers inside itself. Carrying
  the panic across (store it, return a default value to C, make later callbacks return at once,
  re-raise after the C function returns) was rejected: C keeps running with an invented result
  (`qsort` with a comparator that returns 0, `sqlite3_exec` that goes on to the next row). `longjmp`
  over the frames was rejected too: it skips C++ destructors, which is undefined behavior.
- **Through C++ the rules are C++'s.** A panic inside a func that C++ code calls (a `std::function`,
  the comparator of `std::sort`) unwinds through the C++ frames like an exception: destructors run,
  and so do the deferred calls of the gocpp functions on the way. Only a C **function pointer** (a
  trampoline) is the barrier. A C++ file compiled with `-fno-exceptions` that calls a gocpp function
  pointer meets the barrier too, and ends the program the same way.
- **Threads.** A callback may run on any thread, also one that C created (a pthread of the library):
  the thread gets a goroutine record the first time it needs one, so panics, defers and the goroutine
  number in the trace work there, and a panic on it ends the program like any goroutine's. The
  trampolines keep no shared state; the user data must outlive the calls from other threads.
- **C++ exceptions.** A C++ exception that reaches a function with defers is a normal panic that
  `recover()` can catch (since Phase 05): its value is a `gp::CppException` (Go type
  `runtime.CppException`, an `error`) whose `Error()` is `C++ exception <type>: <what()>`. One that
  no function with defers is on the stack for ends the program like an unrecovered Go panic, with
  exit code 2 (see [C++ exceptions](#c-exceptions) below).
- **Calling gocpp from C.** Functions marked `//export Name` become `extern "C"` functions in the
  `c-archive` and `c-shared` build modes, with cgo's rules for the allowed types. Every export goes
  through a trampoline too ([Phase 06 §2.7](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#27-build-modes--buildmode-like-go)).
- **Linkage.** A trampoline has C++ linkage, not `extern "C"`: the compilers gocpp supports use one
  calling convention for both, so a C caller works.

### C++ exceptions

<!-- illustration: a fragment, not a program -->
```go
func parse(s string) int {
	return int(std.stoi(std.string(s))) // throws std::invalid_argument for "x"
}

func main() {
	defer fmt.Println("deferred") // runs: the exception is a panic in main
	fmt.Println(parse("x"))
}
```

```
deferred
panic: C++ exception std::invalid_argument: stoi

goroutine 1 [running]:
main.main()
	/home/me/prog/main.gp:7 +0x6b
exit status 2
```

- **The message.** A `std::exception` prints `panic: C++ exception <type>: <what()>` (only
  `<type>` when `what()` is empty); anything else prints its type: `throw 42` gives
  `panic: C++ exception of type int`. The type is the one the exception was thrown with, its C++
  name (`std::out_of_range`, not the `std::exception` it may be caught as), also in a program
  built with `-fno-rtti`. A newline in `what()` is followed by a tab, as in Go's panic messages.
- **Where it becomes a panic.** Every gocpp function with a `defer` statement has a `catch (...)`
  around its body. The exception that reaches it (not a Go panic, not `runtime.Goexit`, not a
  forced unwind) becomes a Go panic with a `CppException` value, and the function runs its
  deferred calls with it, as for any panic: the same exception is not a panic in a function
  without defers, which it passes through (its C++ destructors run, its frame has no deferred calls).
  So the deferred calls of every frame between the throw and the end run, `recover()` in one of
  them (called directly by the deferred call, Go's rule) stops it, and the function that
  deferred it returns normally with the named results its deferred calls set. `recover()`
  returns the value as an `any`: `fmt.Println(r)` prints `C++ exception std::runtime_error: bad
  thing 3`, `r.(error)` holds, and `%T` is `runtime.CppException`.
  Panicking again with the recovered value (`panic(r)`) is the same panic, printed once with
  `[recovered, repanicked]`.
- **The trace** of a panic that a function with defers made from an exception starts in that
  function (at the closing brace of its body, where the `catch` is): the frames between the throw
  and the catch are unwound by then, the helper's among them. With no function with defers on the
  stack, the C++ library finds no `catch` and ends the program on the stack of the throw
  (std::terminate, where gocpp prints the report): the trace is the throw's, innermost first, with
  the frames of C++ functions that are in the program (a helper in a header shows as
  `vec_at(std::vector<long, std::allocator<long> > const&, unsigned long)` and its header line);
  the C++ library's own frames are left out, as Go leaves out its runtime's. If C++ code caught
  the exception and threw it again (`catch (...) { throw; }`), or it reached a `noexcept`
  function, the trace starts there.
- **A deferred call that throws while a Go panic runs** is a panic of that deferred call: the
  report prints both, like a panic in a deferred call: `panic: boom`, then `	panic: C++
  exception std::runtime_error: ...`, the other deferred calls still run, and the trace shows the
  deferred call on top of the panicking function.
- **A `std::bad_alloc`** that C++ code throws itself (or `std::bad_array_new_length`: `new
  int[n]` with a negative `n`), which the runtime's new handler doesn't see, prints Go's `fatal
  error: runtime: out of memory`, as gocpp's own allocations do, with the trace where it was
  thrown (where a function with defers caught it, if one is on the stack). A fatal error is not a
  panic: no deferred call runs and `recover()` can't stop it.
- **A forced unwind** (`pthread_exit` and `pthread_cancel` in C code, which glibc does by
  throwing an exception that no handler may swallow) is not a C++ exception for gocpp: a function
  with defers throws it again at once and runs none of its deferred calls (running Go code
  during it is not safe).
- **A Go panic that reaches a `noexcept` C++ function** (C++ calls `std::terminate` there) prints
  the panic's normal report. A C++ destructor that throws (one declared `noexcept(false)`) while
  a Go panic unwinds its frame makes C++ end the program with the new exception: the report shows
  only the C++ exception, and the Go panic is lost. Without any exception, `std::terminate` (a `std::thread` destroyed
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

<!-- illustration: a fragment, not a program -->
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
  after its C++ spelling. Equality and hashing use whatever the C++ type supports (`operator==`,
  `std::hash`); `fmt` prints numbers, bools, enums, pointers and C++ strings by their C++ type, and a
  class through its `std::formatter` or `operator<<` when a `fmt` call in the program names a value
  with that class inside (otherwise it prints as the address of the box) ([Phase 05 §2.2](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#22-type-descriptors-chosen-by-the-go-type)).
- **Pointers in an `any` ignore `const` and nesting.** Go has no `const`, so a `const char*` that a C
  function returns is the dynamic type `*C.char` (`x.(*C.char)` is true), a `const T*` is `*T`, and
  `T**`, `const T**` and `const T* const*` are one dynamic type, `**T` (held as the raw pointer). The
  conversion casts the `const` away: writing through a `x.(**T)` or `x.(*T)` assertion to an object that
  is really `const` is undefined behavior, as in C. A pointer to a gocpp variable of type `*C.T` (`&p`)
  is a gocpp pointer, not a `T**`, and does not match `x.(**T)`.
- **Interface methods with C/C++ types.** `go/types` treats every C type as invalid, and two
  invalid types compare equal. So gocpp checks method signatures with C/C++ types itself: a
  `static_assert` at each static conversion, and a comparison by source spelling at run time
  ([Phase 05 §2.3](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#23-interface-values-and-itabs)).

### Printing C/C++ values

<!-- illustration: a fragment, not a program -->
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
  type by its C++ name (`Point`, `char*`, `std::string`, `std::vector<long,
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

<!-- illustration: a fragment, not a program -->
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
| `//gocpp:no-win32-defaults` | none | Windows: gocpp does not define `WIN32_LEAN_AND_MEAN`, `NOMINMAX`, `UNICODE` and `_UNICODE` ([Windows headers](#windows-headers), Phase 14) |

The rules for directives:
- A directive starts with exactly `//gocpp:`, with no space after `//`. `// gocpp:link m` is an
  ordinary comment, and so is every `/* */` comment. But before the `package` clause, a comment
  like `// gocpp:link m` gets a warning, because it looks like a broken directive. To turn a
  directive off, comment it out: `// //gocpp:link m`.
- Directives go before the `package` clause: in the package doc comment or in a comment of their
  own. A `//gocpp:` line after the `package` clause is an error, except `//gocpp:extern` in the doc
  comment of a function that C or C++ defines ([Functions that no header declares](#functions-that-no-header-declares))
  and `//gocpp:native` in the doc comment of a function of the standard library
  ([ADR-027](../adr/027-native-std-functions.md)).
  In a doc comment, `gocpp fmt`
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
- A package of a multi-package program has its own C and C++ files: these are the files of each package that has them.
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
  check runs once per header and is cached (in memory and on disk, per compiler, folder, flags and header):
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
directives of all the files of a package count for all its imports. A module header
([Module headers](#module-headers)) is named by its full import path
(`//gocpp:extern-c github.com/lib/x/api.h`), and the default rules treat it like a local header: the C
header check runs with the module's folder as `-iquote`.

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

## Functions that no header declares

A `.gp` file can call only what an imported header declares. Sometimes the declaration is not there:
Dear ImGui's `imgui_impl_win32.h` keeps `ImGui_ImplWin32_WndProcHandler` inside `#if 0` and tells you to copy
the declaration into your own file, or a C file of the package defines a function and has no header at all.
Declare the function in the `.gp` file, as Go declares a function that assembly defines: no body, and a
`//gocpp:extern` line in the doc comment ([ADR-128](../adr/128-extern-functions-are-declared-in-gp-with-a-directive.md)).
cgo's preamble does the same job with C text; here the declaration is Go, and the types are the C types of the
header packages.

<!-- test: tests/interop/doc_extern/clamp.c -->
```c
// clamp.c
int clamp(int x, int lo, int hi) { return x < lo ? lo : x > hi ? hi : x; }
```

<!-- test: tests/interop/doc_extern/main.gp -->
```go
package main

import "fmt"

// clamp is defined in clamp.c; no header declares it.
//
//gocpp:extern C
func clamp(x, lo, hi C.int) C.int

func main() {
	fmt.Println(clamp(15, 0, 10), clamp(-3, 0, 10))
}
```

Output (`gocpp run .`, in the folder, so that `clamp.c` is built):

<!-- output: tests/interop/doc_extern/main.gp -->
```text
10 0
```

**The directive.** `//gocpp:extern [C|C++] [name]` goes in the doc comment of a function without a body.

| Directive | The C++ it declares |
|---|---|
| `//gocpp:extern` | a C++ function in the global namespace, named like the Go function |
| `//gocpp:extern C` | `extern "C"`, named like the Go function |
| `//gocpp:extern C c_name` | `extern "C"`, with another name (the Go name can be any name that Go allows) |
| `//gocpp:extern ImGui::Begin` | the function `Begin` of the namespace `ImGui` (`namespace ImGui { ... }`), C++ linkage |
| `//gocpp:extern ::sum` | the same as `sum`: a leading `::` is allowed |
| `//gocpp:extern C++ C` | `C++` can be written out; it is the way to name a C++ function `C`, because a lone `C` means the linkage |

The Go name is an ordinary Go name: exported when it starts with a capital letter (`lib.Add3` from another
package), and it must be a name that is free in the package. The C++ name has no such rule, except two: it is
spelled as written, so a C++ keyword (`new`, `class`, `int`) is an error, because no C or C++ function has such
a name (a Go function named `new` gets `new_` in the C++ gocpp writes, which a C++ function does not have; name
the C++ function in the directive: `//gocpp:extern C do_new`), and a qualified name does not start in the
namespace `gp`, `std` or `gpkg_...`, which hold gocpp's own code.

**Linkage is part of the name.** A function of a `.c` file has C linkage: write `//gocpp:extern C`. A C++
function has the parameter types in its linker name, so the declaration must have the types of the definition,
exactly. gocpp writes them from the signature (below), and the C++ compiler cannot check them against the
definition (it is in another file), so a difference is a link error: `undefined reference to
'plain_c_function(int)'`. The error is shown at the `.gp` line of the call with a hint that names the
declaration and says what to try. Namespaces are written in the name (`shapes::inner::name`), overloads are
several Go functions with one C++ name, and a `const` is spelled with a type trait, because Go has no `const`:
`*std.add_const_t[C.int]` is `const int*` (import `"<type_traits>"`; `tests/interop/extern_cpp_file` runs
all of it). `WINAPI` (`__stdcall`) is nothing on the 64-bit targets, so there is no calling convention to
write; `__declspec(dllimport)` is not needed with MinGW's import libraries.

**The signature.** The parameters and the result have a C++ spelling that does not depend on how gocpp holds
a Go value, so only these types are allowed (anything else is a `gocpp check` error that says why):

- a C or C++ type: `C.int`, `C.size_t`, `*C.char`, `**C.char`, a type of a header package (`windows.HWND`,
  `windows.LRESULT`), a C++ class or template instance (`std.vector[C.int]`, by value or by pointer);
- `unsafe.Pointer`, which is `void*`;
- `bool`, the integer types with a size (`int8` ... `int64`, `uint8` ... `uint64`, `byte`, `rune`), `uintptr`,
  `float32`, `float64`, and pointers to them (`*int32` is `int32_t*`: an out parameter that `&n` of a Go
  variable fills). Go's `int` and `uint` are errors: they are 64 bits where a C `int` is 32, so a declaration
  `func neg() int` over `int neg(void)` would give a wrong value without a word. Write `C.int`, `int32`, `int64`,
  `C.long` or `C.longlong`, as the C function declares;
- no result or one result. The result is not `unsafe.Pointer` and not a pointer to a number of Go (`*int32`): a
  gocpp pointer is a managed object that can not point into memory of C. Write a pointer to a C type
  (`*C.char`, `*C.int`), or let the C function fill a pointer parameter (a parameter may be either).

A pointer is a raw C++ pointer in the declaration (`char*`, not `gp::cptr<char>`, which would be another C++
type with another name). A Go `string`, slice, map, interface, func or channel, a struct or named type
declared in Go, a variadic parameter and more than one result are errors. C's `...` is not supported
(cgo does not call variadic functions either): write a C function with a fixed list of parameters that calls
it, and declare that one. The function has no body, so there is no method form and no generic form.

**What gocpp writes.** The declaration goes into the package's generated header, in the global namespace,
after the package's C/C++ headers, so every file of the package and every package that includes the header
sees it. `windows.HWND` and the other header types are written like everywhere else:

```cpp
// gpkg/command-line-arguments/pkg.hpp
extern "C" int clamp(int, int, int);
namespace shapes { double area(double, double); }
::LRESULT ImGui_ImplWin32_WndProcHandler(::HWND, ::UINT, ::WPARAM, ::LPARAM);
```

A call is a call of the C++ name, like a call through a header package: untyped constants are C literals,
a gocpp pointer is passed as the raw address, `&p` of a `*C.T` variable is an out parameter, the result of a
call is a C value. Everything in [Values and operations](#values-and-operations) applies. The function is
only called: `f := clamp` is an error (a Go func value over a C function has no representation), and so is
passing it as a callback; wrap it in a Go function.

**Declared twice.** The same function can be declared in several places (two packages that need it, or one
package under two Go names: `//gocpp:extern C Twice` on `twice`). The declarations must be spelled alike, and
`gocpp build` reports the ones that are not: two C declarations with another result or other parameters
(`int32_t` and `int` count as different spellings, although they are one type: write the same one in both; the
error says so when that is the only difference),
a C and a C++ declaration with the same parameters (C++ rejects it), and two C++ declarations of the same
parameters with other results. C++ declarations with other parameters are overloads. A function that a
header also declares needs no directive; if you write one anyway, the C++ compiler sees both declarations in one file and compares them.

**Where it works.** Packages and `//go:build` lines: the directive belongs to the function, so a file
constraint (`ext_windows.gp`) decides whether it exists. `-interop=typed` and `-interop=opaque` give the same
declarations. A package with only Go types and `C.int`-like built-ins in its extern functions needs no header
import and no C++ compiler to be checked; a header type in the signature needs the header, as ever. The
language server shows the function like any function of the package (hover, go-to-definition, also from another
package), and `gocpp doc` prints its declaration and its comment (the directive is not part of the comment).
`gocpp check` reports the signature errors, the uses that are not calls, and a function without a body that has no
directive (`missing function body`); the declarations that disagree
are found when the program is lowered, so `gocpp build` and `gocpp emit` report them.

`defer` and `go` work on an extern call, also of an overloaded name: the call is made in a lambda, which lets the
C++ compiler pick the overload.

**Not supported.** Variables (`extern int counter;`), C varargs, a function used as a value, a struct passed
by value that is declared in Go, and the standard library (it calls the runtime with `//gocpp:native`,
[ADR-027](../adr/027-native-std-functions.md)).

**Dear ImGui's Win32 backend.** The whole workaround of `demos/imgui-dx11` was a header with one declaration; now
it is this, in the `.gp` file (`windows` is the package of `<windows.h>`):

<!-- illustration: from demos/imgui-dx11/main.gp -->
```go
//gocpp:extern
func ImGui_ImplWin32_WndProcHandler(hwnd windows.HWND, msg windows.UINT,
	wparam windows.WPARAM, lparam windows.LPARAM) windows.LRESULT
```

## Small helpers: a header next to the .gp file

<!-- test: tests/interop/doc_small_helper/mathx.h -->
```cpp
// mathx.h
#pragma once
#include <cmath>
static inline double hypot3(double a, double b, double c) { return std::sqrt(a*a + b*b + c*c); }
```

<!-- test: tests/interop/doc_small_helper/main.gp -->
```go
package main

import "./mathx.h" // package mathx

func main() { println(mathx.hypot3(1, 2, 2)) }
```

- gocpp has **no inline C/C++ preamble** in v1 (cgo's comment above `import "C"`). A helper goes
  into a header (and, if needed, a `.c` or `.cpp` file) in the package directory. That header is
  a package like any other, and the `.c`/`.cpp` file is compiled with the package.
- This keeps one way of doing things, and the helper gets real editor support as a C++ file.
- **Moving from a cgo preamble:** put the preamble's declarations in `helpers.h` and its
  definitions in `helpers.c` (or `.cpp`), import `"./helpers.h"`, and call `helpers.f(...)`
  instead of `C.f(...)`. `import "C"` and its preamble comment are an error that says so.
- A function that a `.c` or `.cpp` file of the package defines and no header declares can be declared in
  the `.gp` file instead, with `//gocpp:extern`
  ([Functions that no header declares](#functions-that-no-header-declares)).
- C macros that are statements (`do { ... } while (0)`) work as calls in statement position;
  other strange macros can be wrapped in such a helper header.
- An inline preamble is on the backlog ([TODO](../../plans/2026-10-13-gocpp/TODO.md#ideas--backlog-not-scheduled)).

**A header alone, or with a source file.** A header-only helper (`static inline` functions,
templates, classes, constants, macros) needs nothing else. A helper whose functions are defined in
a `.c` or `.cpp` file needs the folder to be built as a package: `gocpp run .` and `gocpp build .`
compile the folder's C and C++ files, while `gocpp run main.gp` builds only that file, like `go
run main.go`. So a declaration without a definition fails to link, and the error is at the `.gp`
line that calls the function, with a hint
([Errors in interop code](#errors-in-interop-code)).

```cpp
// ledger.hpp                       // ledger.cpp
#pragma once                        #include "ledger.hpp"
#include <string>                   std::string ledger::dollars(int cents) { /* ... */ }
namespace ledger {
std::string dollars(int cents);
}
```

<!-- illustration: a fragment, not a program -->
```go
import "./ledger.hpp" // package ledger, and the C++ namespace ledger

func main() { println(string(ledger.dollars(1050))) }
```

**Only the package folder's own files are built.** The `.c` and `.cpp` files directly in the
package folder are compiled, and nothing in a sub-folder: `import "./sub/s.hpp"` finds the
header, but `sub/s.cpp` is not built, so `undefined reference to 's_f()'`. Put the `.cpp` file
next to the `.gp` files (it can `#include "sub/s.hpp"`), or keep everything in the header
(`inline`). A header may have no `.cpp` of its own name (`api.hpp` defined by `impl.cpp`), and the
file-name suffixes and `//go:build` lines of [C and C++ sources](#c-and-c-sources-in-a-package)
decide which files are built (`plat_linux.cpp` yes, `plat_windows.cpp`, `_skip.cpp` and a
`//go:build ignore` file no).

**A `.h` header with a `.cpp` file: the `extern "C"` trap.** `.hpp`, `.hh`, `.hxx`, `.h++` and
`.inl` headers are C++, and a `.hpp` + `.cpp` pair needs nothing more. But a `.h` header that
parses as C is included in `extern "C"` for the generated code ([the rule](#flags-and-the-allow-list)),
and your `.cpp` includes the same header as C++. So the `.cpp` defines `add` with C++ linkage,
the calls from the generated code ask for C linkage, and the link fails:

```text
./main.gp:11: C++: undefined reference to 'add'
	println(mathlib.add(1, 2))
	(./mathlib.h parses as C, so gocpp includes it in extern "C" for the generated code; if mathlib.cpp includes it outside extern "C", ...: add //gocpp:no-extern-c ./mathlib.h, or #ifdef __cplusplus extern "C" guards to the header)
```

The hint is a possibility, and gocpp prints it only when an undefined function is named in a `.cpp`
file that includes the header (or has its stem) and has no `extern "C"` of its own. A third fix
is to include the header in the `.cpp` file inside `extern "C" { #include "mathlib.h" }`.

There are two fixes in the header or the directive (`tests/interop/helper_h_cpp_pair` and `helper_h_cpp_guards` run both):

<!-- illustration: the variant of tests/interop/helper_h_cpp_pair -->
```go
//gocpp:no-extern-c ./mathlib.h // this .h file is C++: gocpp doesn't wrap it
package main

import "./mathlib.h"
```

```cpp
// mathlib.h, the other way: the usual guards, which gocpp respects (a header with its own
// __cplusplus is not wrapped) and which also let a C file include it
#pragma once
#ifdef __cplusplus
extern "C" {
#endif
int add(int a, int b);
#ifdef __cplusplus
}
#endif
```

Or name the header `.hpp` when only C++ uses it. A `.h` header with a `.c` file needs nothing.

**What is found, and how.**
- The package folder is on the quote include path (`-iquote`) for the generated code and for the
  package's own `.c` and `.cpp` files alike, so the header can include other local headers
  (`#include "units.h"`, `#include "inc/limits.h"`), and a file next to it can't hide a system
  header such as `<vector>`. The same header text is seen by the generated code and by your
  `.cpp`.
- The header is a package named after the file (`mathx`, `ledger`), with the usual
  [naming rules](#header-packages-and-their-names). **Name a helper header yourself** when its
  default name is not usable: `main.h` next to `main.gp` has the name `main`, which the package
  already declares (`main.mn` is then Go's `func main` and its error is a bare `undefined`:
  write `import mh "./main.h"`), and `./3d.h` has no name at all (a Go name can't start with a
  digit: `import d3 "./3d.h"`). A file name with `-` is fine (`my-lib.h` is `my_lib`). When it also declares a namespace of that
  name, `ledger.book` is `::ledger::book` (the namespace first) and `ledger.ledger_total` is the
  C-level `::ledger_total`.
- A `.h` file that parses as C is wrapped in `extern "C"`, so a `.c` file defines what it
  declares; a `.hpp` file is C++ ([the rule](#flags-and-the-allow-list)). Names and flags follow the
  rest of this document.
- Changing a helper header, or a header it includes, rebuilds the generated code that includes
  it (the build cache keys on the headers' text).

**An `inline` function is compiled twice.** gocpp compiles the generated code with `-fwrapv`
(Go's wrapping integers), and your `.c` and `.cpp` files without it
([Which flags](#c-and-c-sources-in-a-package)). A function that the header defines `inline` or
`static inline` is compiled in each file that includes it. For `static inline` the copies are
separate, so each file behaves as it was compiled. For an `inline` function with external linkage
(a C++ `inline` function, a template) the linker keeps one copy of it for the whole program, and
which one is not defined: the same function can wrap or not wrap on signed overflow depending on
the link. Keep such code free of signed overflow (use `unsigned`), or define the function in the
`.cpp` file.

**`C.free` and `C.CString`.** A helper that keeps a string, or one that returns `malloc`ed
memory, goes through these two: `p := C.CString(s)` copies a gocpp string into C's heap, and
`C.free(p)` frees it, as it frees what a C helper returns (`C.GoString(q)` copies it into a
`string` first). `C.free(nil)` does nothing, and `C.free` takes what C's `free` takes: the
`char*` of `C.CString`, or the `void*` that `malloc` returns. A call that only reads the string
needs no free: use `C.cstr(s)`. More in [Strings at the boundary](#strings-at-the-boundary).

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
  interop helpers (`gocpp/interop.hpp` and `gocpp/result.hpp`: `C.int(x)`, a `switch` on a C value, `gp::range`, `gp::result_slot`, ...);
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
  gocpp's runtime outside `gocpp/interop.hpp` and `gocpp/result.hpp` (also a header template that the runtime
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
- **gocpp's names in the message.** The compiler's message is rewritten with the names of the
  `.gp` source: `gp::slice<gp::string>` is `[]string`, `gp::map<gp::string, double>` is `map[string]float64`,
  `gp::ptr<T>` is `*T`, `gp::func<R(A)>` is `func(A) R`, `gp::chan<T>` is `chan T` (`<-chan T` or `chan<- T` when the line says so), `gp::any` and
  `gp::error` are `any` and `error`, and `gpkg_main::Point` is `main.Point`; the namespace of an
  imported package (`gpkg_github_dcom__a__engine`) is turned back into the package name (`engine`) by
  reversing the escape of ADR-010 (the round trip is a property test). C and C++ types keep their
  C++ names (`std::vector<int>`, `const char*`), the compiler's template noise is dropped
  (`std::__cxx11::basic_string<char, ...>` is `std::string`; clang's `(aka ...)` of a runtime type
  is left out). `-cpperrors=full` shows the raw text.
  Names are rewritten only when the message proves them: Go types that share one C++ type
  (`int` and `int64`, `uint`, `uint64` and `uintptr`, a channel in any direction) are shown as the
  `.gp` line of the error spells them (the checker's types of that line); when the line does not
  say, the C++ name stays (`[]int64_t`, `gp::chan<T>`) rather than a guess. A type of the package is `main.Point` when the text writes its namespace (gcc always does, clang
  with `(aka 'gpkg_main::Point')`); a bare `Point` that clang writes stays as it is, because a C or C++
  type of the same name looks the same. A raw name is never wrong, a wrong rewrite is.
- **Candidates and macros.** The candidate list of "no matching function" is shown as notes, at
  most three with the reason each one does not fit, then "and N more candidates" (the library's own
  overloads, in the C++ standard library's and gocpp's runtime headers, are left out). A note "in expansion of macro 'X' (defined at ./x.h:5)"
  is kept.
- **Source excerpts and color.** When standard error is a terminal (`-excerpt=always|never|auto`,
  `GOCPP_EXCERPT`; not with `TERM=dumb`), every error of gocpp or of the C++ compiler shows its
  source line (tabs kept, wide characters counted by their cells, a line over 100 cells cut around
  the column with `…`) and a caret under the column (`^~~~` for a span). The C++ compiler's column
  means nothing in the generated code, so a C++ error has a line only: the caret goes under the
  first name the message quotes that is a word of the line (`v.push_bak(x)`), a guess that can be
  the wrong one when the name occurs twice (`x.foo(foo)`); without a quoted name the whole line is
  underlined with `~~~~`. An error at the end of the file (`expected '}', found 'EOF'`) points just
  after the last character of the last line. Color (`-color=auto|always|never`) is plain ANSI:
  bold for the place, red for the error, magenta for notes, green for the caret. What decides, first
  rule wins: the `-color` flag (`always`/`never`), `GOCPP_COLOR=always|never`, `NO_COLOR` set to
  any non-empty value (no color), `CLICOLOR_FORCE` set to anything but empty or `0` (color),
  `TERM=dumb` (no color), and otherwise whether standard error is a terminal. Without a terminal the
  output is the plain `file:line:col: message` form that editors and CI read; the exit codes do not
  change. `gocpp fmt` has gofmt's flags and gofmt's error format, and no `-color`. The golden tests are
  `cmd/gocpp/testdata/diag` (a `.golden` file per
  compiler where g++ and clang++ word an error differently; `GOCPP_UPDATE_DIAG=1` rewrites them).
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

A walkthrough with examples that ran on a real PC (a message box, wide strings, a window, sockets) is [the Win32 page](../guides/windows-interop.md).

<!-- illustration: Windows only: not built here -->
```go
package main

import "<windows.h>"

func main() {
	windows.MessageBoxW(nil, C.wstr("Hello from gocpp"), C.wstr("gocpp"), 0)
}
```

- `<windows.h>` is a header package like any other: `windows.MessageBoxW(nil, …)`,
  `windows.GetLastError()`. So are `<winsock2.h>`, `<ws2tcpip.h>`, `<shellapi.h>`, `<shlobj.h>`, `<wincrypt.h>`,
  `<mmsystem.h>`, `<dxgi.h>`, `<d3d11.h>` and the C runtime's `<io.h>`, `<direct.h>`, `<process.h>`, `<tchar.h>`:
  `tests/testdata/windows/winzoo.gp` imports them all and calls each.
- **The Win32 defaults.** For a package for windows that imports a header or has C or C++ files, gocpp compiles the
  generated C++, the probes and the package's own C and C++ files with `-DWIN32_LEAN_AND_MEAN -DNOMINMAX -DUNICODE
  -D_UNICODE`. They keep `min` and `max` out of C++ code, stop `<windows.h>` from including the old
  `<winsock.h>` (MinGW warns "Please include winsock2.h before windows.h" otherwise), and make the name `MessageBox` mean
  `MessageBoxW`, as Go's own Windows code uses the `W` functions. The runtime and the sources of a library with a
  `cxx` recipe do not get them.
  - `//gocpp:no-win32-defaults` turns all four off for the package.
  - A flag of yours that defines or undefines one of them (`-DNAME`, `-DNAME=v`, `-UNAME`, also as `-D NAME`) in
    `//gocpp:cflags`, in pkg-config's flags, in the public flags of a `cxx` recipe, or in `GOCPP_CFLAGS` or
    `GOCPP_CXXFLAGS` leaves that macro to you: gocpp adds the others. For example `//gocpp:cflags -UUNICODE -U_UNICODE`
    gives `MessageBoxA`.
  - They are compile flags of the package, so the runtime's precompiled header is not used for it.
- **Names that are macros of the headers** are handled by the macro protection above, with no rule of their own for
  Windows. A name of your package that `<windows.h>` makes a macro gets a `_` suffix in the generated C++ (the Go name,
  and what `%T` prints, stay): `ERROR`, `DELETE`, `CONST`, `TRUE`, `FALSE`, `MAX_PATH`, `INFINITE`, `near`, `far`,
  `small`, `IN`, `OUT`, `OPTIONAL`, `VOID`, and the `W`/`A` macros `GetObject`, `SendMessage`, `CreateWindow`,
  `DrawText`, `LoadImage`, `RGB`, and `min` and `max` (with `NOMINMAX` they are not macros, and your own `min` and
  `max` work). `interface` is a keyword in Go and cannot be a name. `tests/testdata/windows/winmacros.gp` declares each
  kind (constant, variable, type, field, method, function) and prints them.
- `W` functions take UTF-16 text. `C.wstr(s)` gives a `const wchar_t*` for the statement (a holder, like `C.cstr`), and
  `C.WString(s)` is `C.CString` for wide characters: a `malloc`ed copy for a pointer that C keeps (a struct field such
  as `WNDCLASSW.lpszClassName`; free it with `C.free`); `C.GoStringW(p)` converts a NUL-terminated wide string back; `C.GoStringWN(p, n)` converts `n` characters, NULs
  included. `p` is any pointer to 2-byte or 4-byte integers: an `LPCWSTR`, `&buf[0]` of a `[260]windows.WCHAR`, a
  `*uint16`. The conversions are Go's (`syscall.UTF16FromString` and `UTF16ToString`, with WTF-8): an invalid UTF-8 byte
  becomes U+FFFD, a surrogate written as three bytes (WTF-8) becomes a lone UTF-16 unit and comes back as the same three
  bytes, a character beyond U+FFFF is a surrogate pair, and a NUL inside the string ends the C string, as with `C.cstr`
  (`syscall.UTF16FromString` returns an error for a NUL instead). `C.wstr` always makes a copy: a gocpp string is UTF-8 and the units are not,
  so there is no zero-copy form as `C.cstr` has. `C.GoStringW(p)` with a pointer of another kind (a string, an int, a `*byte`, a slice) is an
  error of gocpp's, not a C++ one.
  The rules for holders are `C.cstr`'s (an argument of a C/C++ function, or the value of a new variable), and a holder is
  not an argument of `C.GoStringW`. **They work on every target**: where `wchar_t` has 32 bits (Linux) the units are
  UTF-32, so `wchar.wcslen(C.wstr(s))` counts characters there and UTF-16 units on Windows.
- **LLP64.** On Windows `long` has 32 bits: `C.long` and `C.ulong` are 4 bytes (the constants that gocpp checks follow the
  target: `C.long(1 << 40)` is an error on Windows), `DWORD` is `unsigned long`, `C.size_t`, `C.longlong` and every
  handle (`HANDLE`, `HWND`, ...: pointers) have 64 bits. A macro that casts, such as `INVALID_HANDLE_VALUE`, compares
  with a handle (`h == windows.INVALID_HANDLE_VALUE`); to compare a macro with an integer, convert it:
  `int(windows.ERROR_FILE_NOT_FOUND)`.
- **Constants of the headers in array lengths** (`var buf [windows.MAX_PATH]windows.WCHAR`) are not supported until Phase 12 step 12.8 (constant
  macros are probed first and substituted): the error says so; write the number, `[260]windows.WCHAR`.
- **GUIDs.** `IID_IDXGIFactory1`, `CLSID_*` and the other GUID constants that MinGW's headers declare are defined in libraries, not in the headers:
  `uuid`, and `dxguid` for the DirectX ones (`//gocpp:link dxgi dxguid`). `__uuidof(T)` has no gocpp form; use the constant, and pass it to a function
  that takes `REFIID` without `&` (the C++ reference).
- **Libraries.** `//gocpp:link ws2_32 winmm` adds `-lws2_32 -lwinmm` (MinGW links kernel32, user32, advapi32 and shell32 by default; gdi32, ws2_32, winmm, ole32 and the others you name).
  Programs are linked with `-static`, which skips a DLL's import library (`libSDL2.dll.a`): name the file,
  `//gocpp:ldflags -l:libSDL2.dll.a`, and put the DLL next to the program. MinGW has no `#pragma comment(lib, …)`.
  A missing library shows as `undefined reference to '__imp_socket'`; for the common functions gocpp adds the hint `add //gocpp:link ws2_32`.
  A compile error that comes from the ANSI/Wide switch (`cannot convert 'const wchar_t*' to 'LPCSTR'`) gets a hint that says whether the Win32
  defaults are on and how to change them.
- **Structs, calls, callbacks.** `#pragma pack`, bit-fields, `WCHAR name[16]` in a struct, `__declspec(dllexport)` and
  `WINAPI` in a header work through the compiler (`tests/testdata/windows/winstruct/`). x86-64 Windows has a single
  calling convention, so a gocpp function is a `WNDPROC`, a `LPTHREAD_START_ROUTINE` or a `binop` as on Linux
  (`tests/testdata/windows/winapi.gp` starts a thread with one). A COM interface is a C++ class in MinGW's headers:
  `dxgi.CreateDXGIFactory1(dxgi.IID_IDXGIFactory1, (*unsafe.Pointer)(unsafe.Pointer(&factory)))` takes the IID as a
  reference, so pass it without `&`, and call `factory.Release()`.
- **Libraries that MinGW does not ship** (sqlite3, zlib, SDL2, raylib) build for MinGW-w64 in the nix shell (`tools/winlibs.sh`)
  and link with `GOCPP_CFLAGS=-I… GOCPP_LDFLAGS=-L…`; `tests/testdata/windows/lib*.gp` are the zoo programs for them.

Source: [Phase 14 §2.4](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#24-interop-with-windows-headers),
[ADR-082](../adr/082-windows-interop-defaults-and-wide-strings.md).

## Limits and known gaps

Opaque mode has limits because gocpp does not know C/C++ types: it writes what you wrote, and the C++
compiler decides. They fall in three groups.

### Gaps that typed interop (Phase 12) closes or narrows

| Gap | In opaque mode (`-interop=opaque`, or no clang++) | What you can do |
|---|---|---|
| a struct that embeds **several** C/C++ types, with promoted access (`struct{ std.vector[int]; std.string }`, then `mv.push_back(1)`) | the error "not supported yet (Phase 12)": gocpp doesn't know which type has the member. (With one embedded C/C++ type the members are promoted since Phase 05, [above](#embedded-c-types).) | name the field: `mv.vector.push_back(1)` |
| a C macro as a compile-time constant (`var buf [cstdio.BUFSIZ]byte`, `const d = lib.MAX + 1`) | the error "not supported yet (Phase 12)" | |
| a keyed literal of a C struct with array fields | an error | a positional literal (aggregate initialization) |
| the types of a C struct's fields in a literal | unknown: a gocpp value goes in by C++'s rules (narrowing is an error) | convert it to the C type, `C.int(n)` |
| too few values in a positional literal of a C/C++ type (`lib.Vec{1}`) | the fields left are zero (C++'s aggregate initialization); Go and cgo say "too few values in struct literal", but gocpp can't count the fields of a C struct | write every value, or use a keyed literal |
| `.` vs `::`, template vs index | decided by the kind probe; when it can't tell, gocpp falls back to `.` | write `(x).m` or `(v)[i]`: a parenthesized C++ value is always a value ([the probes](#the-two-compiler-probes)) |
| a smart pointer before `.` | not dereferenced for you | write `(*sp).m` |
| the element and index of a `range` over a C++ container, the results of C++ calls | C/C++ values, not Go ints and strings: gocpp can't type them | convert them: `int(i)`, `string(s)` |
| `==` on gocpp structs with C/C++ fields, a struct with a `*lib.T` field as a map key | not supported (Go compares field by field) | compare the fields |
| mistakes in C/C++ code | reported by the C++ compiler, at the `.gp` line (the line, not the column) | |

### Limits that stay: C and C++ lifetime rules

- **C/C++ values follow C++ lifetime rules.** gocpp's safety covers gocpp types. What a C function
  returns, `&x` of a C/C++ local, pointers inside C++ containers and templates, and a pointer that a C or
  C++ function keeps are the program's to keep valid. The cases that gocpp does track (and the ones it does
  not) are listed where they belong: [pointers](#pointers-unsafe-and-slices-at-the-boundary),
  [objects in closures and `defer`](#cc-objects-in-closures-and-defer). The sanitizers (`gocpp run -asan`)
  find the rest.
- **A callback is a function pointer only when it has code of its own**: a function of the package or a
  func literal without captures. A method value, a function of another gocpp package and a generic
  instance are C++ callables, not function pointers; a variadic C function pointer can't be a callback
  ([callbacks](#c-function-pointers)).
- **A panic that leaves a callback ends the program** (divergence D27), and so does a C++ exception that
  nothing catches in a function without defers.
- **Callbacks that C calls millions of times cost several times what a C callback costs**, because a
  pointer argument is a counted gocpp pointer; a C list of structs built with `&T{...}` costs several
  times a Go list; a read of a C++ element into a new variable is slower than C++'s. The numbers are in
  the sections above.
- **Compile time grows with the nesting of closures in one opaque statement.** `x := lib.f(...)` writes
  the call three times in the C++ (once to take its type outside the lambda), so a call whose arguments
  hold func literals that again hold such calls grows by about 3^depth. It matters only for code
  generated by machine.
- **`C.long` and the other C numbers have the sizes of the target** (a 64-bit `long` on x86-64 Linux, 32 bits on
  Windows): C's literals follow C's types (`1<<40` is a `long`), and gocpp does not check the Windows
  difference yet (Phase 14).
- **Threads of C's own** get a goroutine record on first use that is never freed, and no stack-overflow
  report.
- **Windows** is untested for interop until Phase 14.

### Bugs and gaps that are tracked

These are known, small, and listed in the plan's TODO; each has a workaround.

- **Constants from C values are rejected late.** `const d = x + 1` with `var x C.int` is an error ("is not
  constant", as in Go), but `gocpp check` and the editor accept it: only the build reports it.
- **A header whose default name is a keyword**, used by that name, is a parse error before gocpp can ask for
  a name: `import "<sys/select.h>"` and then `select.FD_SETSIZE` is "expected operand, found 'select'".
  Import it with a name, `import sel "<sys/select.h>"`.
- **A named gocpp array or slice of C elements**: `type Chars [1]C.char; var a Chars; a[0] = 1` is
  "internal error: expression has no type: a[0]". Use the unnamed type (`var a [1]C.char`) or a struct.
- **A C char array or a `char*` compared with a string constant** is an error at the line (C++ would compare
  addresses, silently false for an array): `rec.name == "bob"` is "invalid operation: comparing a C string
  (char*) with a Go string compares addresses: use C.GoString(x) == "..." (or strcmp)". Compare text:
  `C.GoString(&rec.name[0]) == "bob"`. Two C pointers compare as addresses, `p == nil` works, and a
  `std::string` compares by content. The check sees the C++ type, so it is a `C++:` error from the
  compiler, and only for a string constant: a char pointer next to a Go string variable is the mismatched-types
  error above.
- **A header problem that no probe sees**: a header that ends in the middle of a declaration passes
  `gocpp check` and then fails the build as an "internal error", and two headers imported by different files of
  one package that conflict with each other (the same function with C and with C++ linkage) are reported as
  gocpp's bug too. `-cpperrors=full` shows the compiler's real message.
- **Helper header names**: a header whose default name is unusable and which is used by that name
  (`main.h` next to `main.gp`, `./3d.h`) gets a bare "undefined" error, without the hint for the other
  names. Name it in the import ([small helpers](#small-helpers-a-header-next-to-the-gp-file)).
- **A `.h` next to a `.cpp` of the same name** needs `//gocpp:no-extern-c` or guards (the trap above).
- **No C-only flag**: `//gocpp:cflags` and `GOCPP_CFLAGS` reach C and C++; there is no `-std=gnu11` for C only.
- **The language server** does not show the errors of `.c` and `.cpp` files (it lists `.gp` files only).
- **The first C++ error of a statement hides the others on it** (one error per `.gp` line is shown, up to ten);
  `-cpperrors=full` shows everything.

## Troubleshooting

Start from the message: the table in the cookbook ([common errors and what they
mean](interop-cookbook.md#14-common-errors-and-what-they-mean)) has the usual ones, each with the test that
produces it. Beyond it:

1. **Read the `.gp` line the error names.** The message is the C++ compiler's, but the line is yours. A
   message full of `gp::` and `gpkg_` names is still about that line.
2. **`-cpperrors=full`** (on `gocpp build` and `gocpp run`) prints the compiler's whole output, with the
   template backtrace. `gocpp build -x` shows every compile, the pkg-config runs and the link.
   `gocpp emit file.gp` prints the C++ that gocpp wrote, with `#line` directives.
3. **"internal error" is gocpp's, not yours** (exit status 2 when the generated C++ did not compile; 1 for an internal error of the checker, such as "expression has no type"): the generated C++ did not compile on a line with
   no C/C++ code, or gocpp does not write that construct yet ("not supported yet"). The message names the
   construct; check the [limits](#limits-and-known-gaps) and report it with the smallest program that shows it.
4. **A crash or a wrong value in C/C++ code** is almost always a lifetime bug: run it with `gocpp run -asan`
   (AddressSanitizer, LeakSanitizer, UndefinedBehaviorSanitizer). The report names the C++ and `.gp` lines.
5. **Undefined reference** at a `.gp` line: the library is not linked (`//gocpp:link`,
   `//gocpp:pkg-config`), or the function is defined in a `.c`/`.cpp` file that `gocpp run main.gp` leaves out
   (`gocpp run .`), or a `.h` + `.cpp` pair has the `extern "C"` trap.
6. **Output out of order** with C's `printf`: it should not be, since `fmt` flushes C's `stdout` first; a
   program that writes with `write(2)` or through another buffer needs its own flush.
7. **A header is not found** (`cannot find header <x.h>`): the compiler's include path lacks it: add
   `//gocpp:pkg-config x`, or `//gocpp:cflags -I/path`.
8. **Names that clash with macros**: gocpp renames its own names that collide with a header's macros
   (`Bool` becomes `Bool_`), but a macro that you use through a header package and that clashes with a generated
   name is an error at the use: wrap the macro in a small helper header.

## How interop is tested

- `tests/interop/` (over 190 programs and folders, each run with `g++` and `clang++`, and again with
  `make ci-sanitize`): every section of this document has tests, named in the text; error tests
  (`// ERROR` comments or an expected `// Stderr:`) pin the messages and the `.gp` lines they point at.
- `tests/zoo/` ([the header zoo](../../tests/zoo/README.md)): one program per header and library
  (`stdio.h stdlib.h string.h math.h time.h errno.h vector string map unordered_map memory algorithm
  filesystem chrono thread optional variant span regex`, sqlite3, zlib, SDL2, raylib), checked with
  `gocpp check`, built with both compilers and run; SDL2 and raylib are only built, never run.
- `examples/` (`cpp-std`, `mixed-sources`, `sqlite`, `raylib`): run and compared with their `output.txt`;
  raylib is only built when there is no display.
- [The cookbook](interop-cookbook.md): its programs are the tests `tests/interop/cookbook_*`, and
  `TestDocSnippets` fails when a program in the docs differs from its test.
- Sanitizers: the whole set runs under AddressSanitizer, LeakSanitizer and UndefinedBehaviorSanitizer with both
  compilers (`GOCPP_FLAGS=-asan`, `make ci-sanitize`).

## What typed interop adds

Typed interop (Phase 12; the default when clang++ is installed, `-interop=typed` to require it) asks clang for the real types of exactly the C/C++ names a program uses, through small
generated "probe" files, the way cgo asks gcc about C names.

- **Earlier, better errors:** many C/C++ mistakes are reported before the C++ compile, with
  gocpp-style messages.
- **Editor support** for C/C++: completion after a header package (`raylib.`) or a namespace
  (`std.`), member completion on C++ values, hover and go to definition into headers. This runs
  only in the background, so it never slows down the other editor features.
- **The two gaps closed:** C macros as compile-time constants (`[cstdio.BUFSIZ]byte`), and
  promoted members of several embedded C++ types.
- **Exact C++** (step 12.6):
  `x->m` or `x.m` instead of `gp::deref(x).m` (also `sp.m` for a member of a smart pointer's pointee),
  keyed C struct literals as designated initializers in the right order, `v.at(0) = 5`, and
  `&v.at(0)` for a `T&` result. Exact template-vs-index decisions and reference results copied
  only when they would dangle are not done yet.
- **Strings:** a `string` passed to a `const char*` or `std::string` parameter is converted automatically
  (step 12.6), unless the call may keep the pointer or give back one into the string (`InitWindow`,
  `sqlite3_bind_text` with `SQLITE_STATIC`, `strtol(s, &end, 10)`, …): those are an error that says to use
  `C.CString`. A C++ function of your own that stores its `const char*` is not known: use `C.CString` for it.
- **Argument adaptations** (ADR-130, [one table](#argument-adaptations-typed-interop)): a `string` also goes to a
  `const wchar_t*` (`LPCWSTR`); `&x` of a Go number variable goes to an `int*` or `long long*` (a temporary, copied
  back); `&p` of a `*lib.T` goes to a `void**` (`GetBuffer`); an untyped constant goes to an enum; a float goes to an
  integer with Go's conversion; a constant that does not fit the parameter is an error; a runtime string at the format
  place of a printf-like function is an error. [`C.int` is `int32`](#the-c-number-types-are-gos), and so on, in both modes.
- **Wrong header package:** a C-level name used through the wrong header package
  (`raylib.printf`) gets a warning.
- **`gocpp bindgen <header>`** (step 12.10, built: [the spec](typed-interop.md#12-gocpp-bindgen-step-1210)) writes a readable
  Go-syntax view of a C/C++ header: its types, functions, variables, enums, typedefs and namespaces, the macros that
  are constants, and the macros that are structs as `var`. It needs clang++ and is for reading, documentation and editors.
- **Compatibility rule:** typed mode never rejects a program that opaque mode and the C++
  compiler accepted. It also accepts what C++ converts: a C/C++ number into a gocpp number
  (`var n int = cstring.strlen(s)`), a gocpp number into a C number, a gocpp number next to a
  C/C++ number (`cstring.strlen(s) + len(s)`) by C++'s conversion rules, with the conversion written
  as `gp::cgo::as<T>(x)`; narrowing conversions get a `gocpp vet` warning instead of an error.
- **Modes** (step 12.11, [the spec](typed-interop.md#13-the-interop-mode-step-1211-adr-101)): `-interop=typed`,
  `-interop=opaque` or `-interop=auto` on `check`, `build`, `run`, `test`, `emit`, `vet` and `expand`, and
  `GOCPP_INTEROP` with the same values (the flag wins; `GOCPP_TYPED_INTEROP=1` still means typed). **`auto` is what you get when you set nothing**
  ([ADR-131](../adr/131-typed-interop-is-the-default-mode.md)): typed when clang++ is installed and opaque otherwise, with no
  message; `typed` without clang++ is an error (status 3); `-interop=opaque` is the mode of this document, with no probes.
  Typed mode passes the same programs (294 programs that import headers print the same in both). Probing needs `clang++`,
  also when the build uses `g++`. The probes cost time on the first build of a program on a cold cache (0.5 to 1.6 s,
  tens of milliseconds on a warm one: [typed-interop.md §13.6](typed-interop.md#136-the-default)).

Source: [Phase 12](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md).
