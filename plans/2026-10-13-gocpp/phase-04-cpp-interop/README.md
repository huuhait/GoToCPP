# Phase 04: C/C++ Interop (import any header, opaque mode)

> Make "a `.gp` file can use **all** C/C++" real: import any header, call any function, use
> any C++ class or template, compile the `.c`/`.cpp` files next to your `.gp` files, and link
> any library. gocpp translates the syntax faithfully, and **the C++ compiler type-checks the
> C/C++ parts**.

| | |
|---|---|
| Depends on | Phase 03 (Phase 01 already parses imports and directives) |
| Unlocks | Phase 05 (C++ exceptions become panics), Phase 06 (C/C++ flags per package), Phase 09 (stdlib uses interop), Phase 12, Phase 13 |
| Milestone | **M3: sqlite, raylib/SDL and std-library demos run** |
| Size | large (about 5 weeks) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

C++ is too big to model completely inside gocpp (templates, overloads, ADL, macros, SFINAE).
So gocpp **does not try to understand C/C++ symbols in this phase**. Every C/C++ name is
*opaque* (Phase 01), whether it comes through a header package (`raylib.InitWindow`), a
namespace (`std.vector`) or `C` (`C.int`). gocpp writes it into the C++ output almost unchanged,
and the C++ compiler, which understands C++ perfectly, checks it. That's why almost everything
works from day one: any header, any library, any template.

gocpp needs only two small facts about C/C++ names, and it gets both cheaply from the compiler
(§2.2): **is this name a macro?** and **is this name a namespace, a type, a template or a
value?** With those, gocpp's job at the boundary is to:
1. translate Go-style syntax into the right C++ syntax (`std.vector[int]{1,2}` →
   `std::vector<int64_t>{1, 2}`, `x.field` on a raw pointer → `x->field`, `std.string.npos` →
   `std::string::npos`);
2. convert gocpp values to C/C++ values safely (strings, slices, pointers, callbacks);
3. build and link the C/C++ side (sources, flags, libraries);
4. point C++ compiler errors back at the `.gp` line.

Phase 12 later adds real C++ types (earlier errors, autocompletion). This phase never depends
on it.

## 2. How it works

### 2.1 What the user writes

A header is a package named after its file, and C++ namespaces work like packages (Phase 01
§2.3):

```go
//gocpp:pkg-config raylib
package main

import rl "<raylib.h>" // renamed like a Go import; without a name it is raylib.InitWindow

func main() {
	rl.InitWindow(800, 450, "gocpp + raylib")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)
	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE) // a macro: CLITERAL(Color){245, 245, 245, 255}
		rl.DrawText("Hello from gocpp!", 190, 200, 20, rl.LIGHTGRAY)
		rl.EndDrawing()
	}
}
```

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

### 2.2 The two compiler probes (macros and kinds)

Once per distinct set of includes and flags (cached; Phase 06 adds the header contents to the
cache key), the driver asks the **build compiler**:

1. **Which names are macros?** `$CXX -dM -E` lists every macro defined by the headers (1,640 for
   `<vector>`+`<algorithm>`+`<cstdio>`, in 0.02 s).
2. **What kind of thing is each C/C++ path?** One small `-fsyntax-only` file with one line per
   distinct dotted prefix the package uses (`using gp_k1 = ::std::chrono::steady_clock;`,
   `namespace gp_k2 = ::std::chrono;`, `auto gp_k3 = &::std::cout;`). The lines that compile tell
   whether a prefix is a **namespace, a type, a template or a value**. This grows out of Phase
   01's namespace probe (which already decides whether `std` in `std.vector` is a namespace),
   and it also settles names that are both a header package and a namespace (`glm.vec3`).

Without the kind probe, opaque mode can't tell `std.chrono.steady_clock.now()` (a static member:
needs `::`) from `std.cout.flush()` (an object: needs `.`). Both type-check silently, but only
one spelling compiles (found in the audit).

### 2.3 Name translation

| gocpp | C++ | Why |
|---|---|---|
| `stdio.printf`, `ctime.time` (header packages, not macros) | `::printf`, `::time` | **Fully qualified**, so no gocpp name can hide them. Inside a package namespace, a plain `time(nullptr)` would find the package namespace `time` first and fail to compile (seen in the audit) |
| `cerrno.errno`, `raylib.RAYWHITE`, `stdio.EOF` (macros, from probe 1) | `errno`, `RAYWHITE`, `EOF` (**unqualified**) | macros only work unqualified; `::errno` would break |
| `std.vector`, `fs.path` (namespaces; `fs` renamed with `import fs "::std::filesystem"`) | `::std::vector`, `::std::filesystem::path` | fully qualified namespaces are unambiguous |
| `glm.vec3` (`<glm/glm.hpp>` is package `glm`, and `glm` is also a namespace) | `::glm::vec3` | the namespace is tried first, then the C-level name (probe 2) |
| `std.chrono.steady_clock.now()`, `std.string.npos`, `ui.Color.Red` (types, from probe 2) | `::std::chrono::steady_clock::now()`, `::std::string::npos`, `::Color::Red` | class scope and `enum class` values need `::` |
| `std.cout.flush()` (a value) | `::std::cout.flush()` | members of objects use `.` |
| `stat.struct_stat`, `lib.enum_x`, `lib.union_y` | `struct stat`, `enum x`, `union y` | cgo convention for C tag names |

A header package gives access to the global (C-level) names, and C++ has only one global
namespace. So `raylib.printf` would also compile when `<stdio.h>` is included somewhere. That's
harmless; Phase 12, which knows which header declares what, makes it a warning.

**C macros must not break gocpp names.** A header may define macros that collide with names
gocpp emits: X11's `#define Bool int` would break `atomic.Bool` and `flag.Bool`. So gocpp headers
are included **before** C/C++ headers. After the C/C++ includes, each macro that collides with a
gocpp name used in this file (and isn't itself used through a header package) is `#undef`ed, or saved and
restored with `push_macro`/`pop_macro`. The macro list from probe 1 makes this exact. gocpp's own
identifiers that collide with macros are also renamed (`errno` → `errno_`), as Phase 02 already
does for the runtime's headers.

### 2.4 C/C++ types in gocpp code

`go/types` has no types for C things. Phase 01 records the **type expression** at each
declaration (patch 3 allows dotted names like `std.filesystem.path` there), and this phase
translates that syntax:

| gocpp type syntax | C++ |
|---|---|
| `C.char`, `C.schar`, `C.uchar`, `C.short`, `C.ushort`, `C.int`, `C.uint`, `C.long`, `C.ulong`, `C.longlong`, `C.ulonglong`, `C.float`, `C.double`, `C.size_t` | `char`, `signed char`, `unsigned char`, …, `long long`, …, `size_t` (cgo's names) |
| `*cstdio.FILE`, `*SDL.SDL_Window`, `**C.char` | `FILE*`, `SDL_Window*`, `char**` (raw pointers: C++ rules) |
| `std.vector[int]` | `std::vector<int64_t>` (gocpp `int` is 64-bit) |
| `std.array[C.int, 4]`, `std.bitset[64]` | `std::array<int, 4>`, `std::bitset<64>` (constant args via the parser patch) |
| `std.vector[Point]`, `std.vector[*Node]` | `std::vector<::gpkg_main::Point>`, `std::vector<gp::ptr<::gpkg_main::Node>>` (gocpp pointers inside C++ containers stay reference counted, so they're safe) |
| `std.vector[int].iterator` | `std::vector<int64_t>::iterator` (member types, from the kind probe) |

### 2.5 Values and operations (opaque)

| gocpp | C++ | Note |
|---|---|---|
| `x := lib.f()` | `auto x = ::f();` | |
| `x.m`, `x.m()` with `x` opaque | `gp::deref(x).m` | `gp::deref` dereferences **raw pointers only**, like Go's auto-deref; values pass through. Smart pointers are not auto-dereferenced: write `(*sp).m` |
| `a + b`, `a == b`, `v[i]` on C++ values | `a + b`, `a == b`, `v[i]` | C++ operators and C++ rules (no gocpp bounds check on C++ containers; use `.at(i)` if you want one) |
| `std.string("hi")`, `std.vector[int](10, 0)` | `std::string("hi")`, `std::vector<int64_t>(10, 0)` | conversion syntax = constructor; several arguments allowed (divergence D5) |
| `std.vector[int]{1, 2, 3}` | `std::vector<int64_t>{1, 2, 3}` | |
| `SDL.SDL_Rect{0, 0, 640, 480}` | `SDL_Rect{0, 0, 640, 480}` | positional = aggregate init |
| `SDL.SDL_Rect{w: 640, h: 480}` | `[&]{ SDL_Rect gp_t{}; gp_t.w = 640; gp_t.h = 480; return gp_t; }()` | Keyed literal: GCC rejects designated initializers out of declaration order, which we can't know in opaque mode. Assignments always work (array members can't be assigned, so keyed literals with array fields are an error until Phase 12). Phase 12 switches to `.w = 640` |
| `nil` compared to or assigned to a C pointer | `nullptr` | |
| `for i, v := range cppContainer` | `for (auto&& [i, v] : gp::range(c))` | `gp::range` works for anything with `begin()/end()` and for C arrays. A temporary container is **stored by value** inside `gp::range` (a reference would dangle under C++20). Random-access containers are walked by index with the size read **once**, as Go reads the length once (a `push_back` in the body would otherwise break the iterators). One variable = index (key for maps), like Go. `v` is a **copy**, like Go |
| `a, b := std.minmax(x, y)` | `auto [a, b] = gp::values(std::minmax(x, y));` | `std::minmax` returns `pair<const int&, const int&>` (spike probe). With temporary arguments the references would dangle (and with variables they alias them), so `gp::values` copies reference members into values |

**Untyped constants in opaque calls** are emitted as plain C/C++ literals with C's default types
(`5` is an `int`, `2.0` a `double`). That is what C APIs expect (`printf("%d", 5)`). C++ templates
that deduce one type from two arguments need a typed constant: `std.max(n, 0)` fails in C++
(`int64_t` vs `int`), so you write `std.max(n, int(0))`. This is documented with an example.

**C/C++ objects used by closures or `defer`.** For C++ objects, a member call can change the
object, so the Phase 03 rule "never reassigned → capture by copy" is wrong for them. An opaque
local that a closure or a deferred call uses is **boxed** (`std::make_shared<T>`), and any member
call or operator on it counts as a change. This also fixes lifetime: `v := std.vector[int]{};
defer v.clear()` would otherwise call `clear()` on a vector that was already destroyed when the
function body's block ended (an ASan error in the audit).

**C++ types that can't be copied or default-constructed** (`std::unique_ptr`, `std::lock_guard`,
types without a default constructor) are handled by a policy table that says which gocpp
constructs copy, move, default-construct or assign. Results go through a `gp::result_slot<R>`
that picks the right storage at C++ compile time (Phase 05 uses it in the single-exit shape),
locals that are not boxed are moved where Go semantics allow, and every construct has a test.

### 2.6 Strings at the boundary

| From → To | How |
|---|---|
| untyped string constant → any C/C++ parameter | a raw C literal `"hello"` (Phase 01 keeps untyped constants) |
| `string` → `std::string_view` | implicit conversion on `gp::string` |
| `string` → `std::string` | explicit: `std.string(s)`. (Implicit conversions to both `std::string` and `std::string_view` are ambiguous for overloaded C++ functions and don't work with templates, so only `std::string_view` is implicit.) |
| `string` → `const char*` | `C.cstr(s)`: returns a small **holder** object that owns a NUL-terminated copy and converts to `const char*`. It lives at least until the end of the statement, even when the evaluation-order pass moves it into a named temporary (it is never turned into a raw pointer early). No copy if the bytes are already NUL-terminated (literals) |
| `string` → owned `char*` | `C.CString(s)` (cgo-compatible, `malloc`; free with `C.free(p)`) |
| `char*` → `string` | `C.GoString(p)`, `C.GoStringN(p, n)` (cgo-compatible) |
| `std::string` / `string_view` → `string` | `string(x)` conversion |
| bytes | `C.GoBytes(p, n)` or `unsafe.Slice((*byte)(p), n)` |

**Typed helpers.** `FakeImportC` makes *every* `C.x` opaque, which would include our own helpers,
so `s := C.GoString(p)` would leave `s` untyped. The Phase 01 preprocessing therefore rewrites
`C.CString`, `C.GoString`, `C.GoStringN` and `C.GoBytes` into calls to a small hidden gocpp
package (`gocpp/cgoh`) with real declarations, for example `func GoString(p unsafe.Pointer)
string`. Opaque arguments still pass silently, and the results get real gocpp types, so
`s + "!"` and `strings.ToUpper(s)` are fully type-checked. `C.cstr` stays opaque, because its
holder must stay a C++ object.

### 2.7 Pointers, `unsafe` and slices at the boundary

- **`unsafe` keeps gocpp memory alive, like Go.** `unsafe.Pointer` is a `gp::ptr<void>`: made from
  a gocpp pointer it shares that object's owner, and it converts implicitly to `void*` for C.
  `unsafe.SliceData(s)` returns an aliasing `gp::ptr<T>` into the slice's array, and
  `unsafe.String(p, n)` / `unsafe.Slice(p, n)` keep `p`'s owner. Only pointers that come **from C**
  give non-owning views. This matters because ported Go code relies on it: `strings.Builder.String`
  is `unsafe.String(unsafe.SliceData(b.buf), len(b.buf))`, and with non-owning views a returned
  string would dangle (found in the audit; an ASan test keeps it fixed). `unsafe.Add` works on both.
- `[]T` converts implicitly to `std::span<T>`.
- A gocpp `*T` passed where C/C++ expects `T*` converts implicitly (**borrowed**). C must not keep
  it after the gocpp object dies. This is the same rule as cgo's pointer-passing rules.
- `&x` where `x` has a C/C++ type is a raw C++ address-of. **C/C++ values follow C++ lifetime
  rules** (documented clearly). gocpp's safety covers gocpp types, not C++ ones.

### 2.8 Callbacks and panics crossing C code

| C/C++ expects | Pass |
|---|---|
| a C function pointer `void (*)(int)` | a top-level gocpp `func cb(x C.int)` |
| `std::function<…>` or a template callable | any gocpp func value or closure (`gp::func` converts) |
| `void* userdata` + callback | `cgo.NewHandle(v)` → pass the handle; in the callback `cgo.Handle(p).Value()` (Go's `runtime/cgo.Handle` API) |

**Panics must not unwind through C code.** On x86-64 Linux, GCC and Clang emit unwind tables for
C by default, so a panic inside a C callback *would* unwind through the C frames and silently skip
the C library's own cleanup (locks, memory, sqlite state). So every gocpp function that C can call
(callbacks, and Phase 06's `//export` functions) is called through a `noexcept` **trampoline**
that catches any panic, prints it Go-style and exits with code 2.

**C++ exceptions (first version).** If a C++ call throws into gocpp code, the program stops with
a Go-style `panic: C++ exception: <what()>` and exit code 2. Phase 05 turns these into normal,
recoverable panics.

### 2.9 Building the C/C++ side

- **Sources in the package directory** are compiled and linked, like cgo:
  `.c` with `$CC -std=c17`, and `.cc/.cpp/.cxx` with `$CXX -std=c++20`, using the package's flags.
- **Directives** from Phase 01 now take effect: `//gocpp:pkg-config sdl2` runs
  `pkg-config --cflags --libs`, `//gocpp:link m` adds `-lm`, and `cflags`/`ldflags` are added as given.
- **`extern "C"` for C headers.** A header needs `extern "C"` when it declares C functions without
  its own `__cplusplus` guard. The rule:
  - a **local `.h`** header is wrapped only if it **parses as C** (`$CC -fsyntax-only -x c`,
    cached) and has no `__cplusplus` guard;
  - `.hpp`, `.hh` and `.hxx` headers are never wrapped;
  - system headers are not wrapped by default, but `//gocpp:extern-c <lua.h>` exists for C
    libraries that ship headers without guards;
  - `//gocpp:no-extern-c <hdr>` turns the wrapping off for that header, whatever the rules
    above say.
  Wrapping a C++ header breaks it (`extern "C" { #include <vector> }` gives over a thousand
  errors with g++), which is why the parse-as-C check matters.

### 2.10 Small C/C++ helpers: a header next to the `.gp` file

There is no inline C++ "preamble" in v1. A helper goes into a header (plus an optional `.cpp`)
in the package directory, and that header is a package like any other:

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

This keeps one way of doing things, the helper gets real editor support as a C++ file, and the
`.cpp` next to it is compiled by §2.9. An inline preamble stays on the backlog.

### 2.11 Errors in interop code

C++ compile errors are now **user** errors, not internal ones. With `#line` they already point
at the `.gp` line (the column is not available through `#line`). The driver prints them in gocpp
style:

```
main.gp:14: C++: no member named 'push_bak' in 'std::vector<long>'
        nums.push_bak(4)
```

Template backtrace noise is trimmed to the first error plus the lines that point into `.gp`
files; `-cpperrors=full` shows everything. Phase 13 also translates C++ type names back to
gocpp names.

### 2.12 Runtime helpers added in this phase

`gp::deref`, `gp::range`, `gp::values`, `gp::result_slot`, implicit `std::string_view` on
`gp::string`, `std::span` on `gp::slice`, `gp::ptr` → `T*`, `gp::cstr`, `C.CString`, `C.GoString`,
`C.GoStringN`, `C.GoBytes`, `C.free`, `runtime/cgo.Handle`, the C-callable trampolines.

The Phase 02/03 `fmt` shim also learns to print C++ values: through `std::formatter<T>` when it
exists, otherwise `operator<<`. **Raw pointers (including `char*`) print as addresses**, as Go
prints pointers. Printing a `char*` as a C string would crash or silence stdout when it is null
(seen in the audit).

## 3. Steps

1. **4.1 Compiler probes**: macro list (`-dM -E`) and kind probe (grown from Phase 01's namespace
   probe), cached per include set + flags.
2. **4.2 C type translator**: AST type expression → C++ type (builtins, tags, pointers,
   templates with constant arguments, dotted and member types, gocpp types inside templates).
3. **4.3 Name translation**: header packages → `::name` or the unqualified macro, namespaces and
   namespace renames, names that are both a header package and a namespace, `::` vs `.` from the
   kind probe, macro protection (`#undef`/`push_macro` after C includes, gocpp headers first).
4. **4.4 Opaque value codegen**: `auto`, `gp::deref`, calls, operators, constructors, `nil`,
   untyped constants as C literals.
5. **4.5 Template-vs-index rule** + parenthesis escape (`[...]` after a C name is a template when
   the kind probe says template, else an index; Phase 12 makes the rest exact).
6. **4.6 Composite literals of C/C++ types** (positional, keyed via assignments, initializer lists).
7. **4.7 `gp::range`** (by-value temporaries, index walk) and **`gp::values`**.
8. **4.8 Opaque objects in closures and defers** (boxing, member calls count as changes) + the
   copy/move/default-construct policy table and `gp::result_slot`.
9. **4.9 String boundary** (`std::string_view`, explicit `std.string`, `gp::cstr` holder, cgo
   helpers rewritten to the typed `gocpp/cgoh` package).
10. **4.10 `unsafe` and pointers** (owning `unsafe.Pointer`, `SliceData`/`String`/`Slice` keep
    owners, `std::span`, `gp::ptr` → `T*`) + an ASan test with `strings.Builder`-style code.
11. **4.11 Callbacks** (function pointers, `std::function`, `cgo.Handle`) + panic trampolines.
12. **4.12 C++ exception → fatal Go-style panic**; `fmt` shim prints C++ values and raw pointers
    as addresses.
13. **4.13 Compile + link package `.c/.cc/.cpp/.cxx` sources.**
14. **4.14 `extern "C"` rule** (parse-as-C check, never `.hpp`, `//gocpp:extern-c <hdr>`,
    `//gocpp:no-extern-c`).
15. **4.15 Directives → flags** (`pkg-config`, `link`, `cflags`, `ldflags`).
16. **4.16 Local helper headers** (§2.10: a `.h` + `.cpp` pair next to the `.gp` files, used as a
    package; replaces the cgo-style preamble) and the `C` helpers (`C.free` with `C.CString`).
17. **4.17 Interop error reporting** (`.gp` mapping, trimming, `-cpperrors=full`).
18. **4.18 Tests**: ≥ 40 interop run tests; **header zoo** (`stdio.h stdlib.h string.h math.h
    time.h errno.h vector string map unordered_map memory algorithm filesystem chrono thread
    optional variant span regex` + SDL2, raylib, sqlite3, zlib) compiled in opaque mode with both
    compilers; tests for `::` vs `.`, renamed and unnamed headers, a header name that is also a
    namespace (`glm`), namespace renames, macro collisions (an X11-style `Bool` macro), a C++
    object used by `defer`, move-only types, `gp::range` over a temporary; interop error tests
    (C++ error → right `.gp` line).
19. **4.19 Docs**: finish `docs/spec/interop.md` and write a cookbook (header packages and
    namespaces, sqlite, raylib, std containers, calling C from a package, writing a wrapper
    package, the untyped-constant rule).

## 4. Deliverables

- Interop codegen + runtime helpers, compiler probes, C/C++ source compilation, flags and libraries
- Examples `cpp-std`, `sqlite`, `raylib` (or `sdl`), `mixed-sources` passing
- Header zoo test, interop docs and cookbook

## 5. Exit criteria

- The four interop examples build and run on GCC and Clang (raylib/SDL may be checked by a
  smoke test that opens a hidden window and quits, or run manually where there's no display).
- The header zoo compiles on both compilers.
- ≥ 40 interop run tests are green, including the audit cases above.
- Every seeded C++-side error in the error tests is reported at the correct `.gp` line.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Errors arrive late and look like C++ | Good `#line` mapping and trimming now; Phase 12 checks earlier; Phase 13 renames types |
| `gp::deref` guesses wrong for exotic types (proxies, overloaded `operator&`) | `(*x).m` always works as an explicit form; Phase 12 makes it exact |
| The kind probe can't classify a name (e.g. it depends on template arguments) | Fall back to `.` for values and report a clear hint: a parenthesized C++ value is always a value, so `(x).m` uses `.` and `(v)[i]` is always indexing; Phase 12 is exact |
| Strange macros (statement macros, `do{}while(0)`) | They work as calls in statement position; otherwise wrap them in a small helper header (§2.10) |
| Raw pointers to C++ values dangle | Documented C++ rule; ASan in CI; Phase 13 `vet` checks for `&cppLocal` escaping |
| A panic would unwind through C frames | Trampolines on every C-callable entry stop it with a Go-style fatal error |
