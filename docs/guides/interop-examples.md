# Interop examples: SQLite, raylib, C++ containers and mixed sources

The [cookbook](../spec/interop-cookbook.md) has small recipes. This page walks through the four
interop programs of [examples/](../../examples/README.md), which are whole programs with a
checked output. Every line quoted here is a line of the real file: the tests of the docs compare
the excerpts with the example files, `gocpp check` runs on the examples, and the examples are run
by the example test suite (programs that need a library or a display skip themselves when it is
missing).

The pattern is the same everywhere:

1. `import "<header.h>"` makes the header a package named after the file (`sqlite3.sqlite3_open`).
   A library gets a `//gocpp:pkg-config name` or `//gocpp:link` line before the `package` clause.
2. What C returns is a C value. You convert it where Go needs a Go value (`int(...)`,
   `C.GoString(...)`), and the C++ compiler checks the rest.
3. A Go string goes to C as a constant, as `C.cstr(s)` (valid for the statement), or as
   `C.CString(s)` (a `malloc` copy that you `C.free`).

## SQLite: a C library with callbacks

[examples/sqlite](../../examples/sqlite/main.gp) keeps a table in an in-memory database. The
header package, the directive and the first call:

<!-- excerpt: examples/sqlite/main.gp -->
```go
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"
	"unsafe"

	"<sqlite3.h>"
)
...
func main() {
	var db *sqlite3.sqlite3
	if rc := sqlite3.sqlite3_open(":memory:", &db); rc != sqlite3.SQLITE_OK {
		fmt.Println("open failed:", C.GoString(sqlite3.sqlite3_errstr(rc)))
		return
	}
	defer sqlite3.sqlite3_close(db)
```

`*sqlite3.sqlite3` is a raw C pointer to the opaque handle; `SQLITE_OK` is a macro, used as a
constant. SQL goes through `C.CString`, and the memory is released with `C.free`:

<!-- excerpt: examples/sqlite/main.gp -->
```go
func exec(db *sqlite3.sqlite3, sql string) bool {
	csql := C.CString(sql)
	defer C.free(unsafe.Pointer(csql))
	var errmsg *C.char
	if sqlite3.sqlite3_exec(db, csql, nil, nil, &errmsg) != sqlite3.SQLITE_OK {
		fmt.Println("error:", C.GoString(errmsg))
		sqlite3.sqlite3_free(unsafe.Pointer(errmsg))
		return false
	}
	return true
}
```

A prepared statement binds Go values: a string through `C.cstr` and a number converted with
`C.int`:

<!-- excerpt: examples/sqlite/main.gp -->
```go
	var stmt *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, "INSERT INTO fruit VALUES (?, ?)", -1, &stmt, nil)
	for i, name := range []string{"apple", "banana", "cherry", "date"} {
		sqlite3.sqlite3_bind_text(stmt, 1, C.cstr(name), -1, sqlite3.SQLITE_TRANSIENT)
		sqlite3.sqlite3_bind_int(stmt, 2, C.int((i+1)*3))
```

C calls back into a gocpp function through a plain C function pointer. The function has C's
types for its parameters (`C.int`, `**C.char`), and `unsafe.Slice` turns the C arrays into slices:

<!-- excerpt: examples/sqlite/main.gp -->
```go
func printRow(_ unsafe.Pointer, n C.int, values **C.char, names **C.char) C.int {
	count := int(n)
	cols := unsafe.Slice(names, count)
	vals := unsafe.Slice(values, count)
	...
	fmt.Println(line)
	return 0
}
...
	sqlite3.sqlite3_exec(db, "SELECT name, qty FROM fruit ORDER BY qty DESC", printRow, nil, nil)
```

The callback's `void*` argument carries a pointer to a gocpp `int` through C:

<!-- excerpt: examples/sqlite/main.gp -->
```go
func countRow(arg unsafe.Pointer, _ C.int, _ **C.char, _ **C.char) C.int {
	*(*int)(arg)++
	return 0
}
...
	count := 0
	sqlite3.sqlite3_exec(db, "SELECT * FROM fruit WHERE qty > 10", countRow, unsafe.Pointer(&count), nil)
```

The output of the program, as checked in `output.txt`:

<!-- excerpt: examples/sqlite/output.txt -->
```text
updated: 2
name=banana qty=60
name=apple qty=30
name=date qty=12
name=cherry qty=9
rows with qty > 10: 3
total: 111 last: date
error: no such table: vegetables
insert into vegetables: false
```

More on callbacks: [cookbook recipe 5](../spec/interop-cookbook.md#5-pass-a-go-function-to-c-as-a-callback)
and [recipe 6](../spec/interop-cookbook.md#6-sqlite-open-exec-query). A panic in such a callback ends
the program ([D27](../spec/language.md#d27-a-panic-that-leaves-a-callback)).

## raylib: a window and a game loop

[examples/raylib](../../examples/raylib/main.gp) draws a bouncing ball with the raylib C library.
The header gets a name of your choosing (`rl`), the C++ header `<cstdlib>` brings `std.getenv`, and
the directive names the library:

<!-- excerpt: examples/raylib/main.gp -->
```go
//gocpp:pkg-config raylib
package main

import (
	"fmt"

	"<cstdlib>"
	rl "<raylib.h>"
)
```

The ball is a gocpp struct with a pointer method; raylib's types and functions are used next to it.
`rl.Vector2{ball.X, ball.Y}` is a C struct literal, `rl.MAROON` is a macro for a
`CLITERAL(Color)`, and the loop is the usual raylib loop:

<!-- excerpt: examples/raylib/main.gp -->
```go
	rl.InitWindow(width, height, "gocpp + raylib")
	if !rl.IsWindowReady() {
		fmt.Println("raylib: no window (is there a display?)")
		return 0
	}
	defer rl.CloseWindow()
	...
	for !rl.WindowShouldClose() && !(smoke && frames == smokeFrames) {
		...
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE) // a macro: CLITERAL(Color){245, 245, 245, 255}
		rl.DrawCircleV(rl.Vector2{ball.X, ball.Y}, ball.R, rl.MAROON)
		rl.DrawText(C.cstr(fmt.Sprintf("frame %d", frames)), 10, 10, 20, rl.DARKGRAY)
		...
		rl.EndDrawing()
```

When the environment variable `GOCPP_SMOKE` is set, the window is hidden and closes after three
frames; that is how the test suite runs it (under a virtual display). The checked output:

<!-- excerpt: examples/raylib/output.txt -->
```text
window: 800 x 450
frames: 3 ball at: 412 234
window closed
```

[Cookbook recipe 11](../spec/interop-cookbook.md#11-sdl2-and-raylib-build-only-here) covers SDL2 and
raylib, and says what builds here without a display. Do not start the program from a script
without one: raylib needs a display even for a hidden window.

## C++ containers and the standard library

[examples/cpp-std](../../examples/cpp-std/main.gp) is [tour lesson 9](../tour/09-cpp-containers.md)
with more: `<algorithm>`, `std::filesystem` through a renamed namespace and C++ strings.

<!-- excerpt: examples/cpp-std/main.gp -->
```go
	fs "::std::filesystem"
)
...
	p := fs.path("/home/gopher/notes/todo.txt")
	fmt.Println(string(p.filename().string()), string(p.stem().string()), string(p.extension().string()))
	fmt.Println(string(p.parent_path().generic_string()))
	q := p.parent_path() / "archive" / "2026.txt"
```

An import with a path that starts with `::` names a C++ namespace and gives it a Go name (`fs`);
`p.parent_path() / "archive"` is the C++ `operator/` of `std::filesystem::path`.

## Mixed sources: your own C and C++ files

[examples/mixed-sources](../../examples/mixed-sources/main.gp) is one package of `.gp`, `.c` and
`.cpp` files. Headers next to the sources are imported by path:

<!-- excerpt: examples/mixed-sources/main.gp -->
```go
import (
	"fmt"
	"unsafe"

	"./checksum.h"
	"./textutil.hpp"
)
```

`gocpp build` compiles `checksum.c` with the C compiler (its header is put in `extern "C"`) and
`textutil.cpp` with the C++ compiler, and links them with the generated code. The C function, the
struct it returns and the C++ class are used like this:

<!-- excerpt: examples/mixed-sources/main.gp -->
```go
		h := uint32(checksum.fnv1a32(C.cstr(s), C.size_t(len(s))))
...
	st := checksum.stats_of(unsafe.SliceData(xs), C.size_t(len(xs)))
	show("min max sum", int(st.min), int(st.max), int(st.sum))
...
	var t textutil.tally
	for _, w := range []string{"go", "cpp", "go", "c", "go", "cpp"} {
		t.add(w)
	}
```

The header files themselves, `checksum.h` and `textutil.hpp`, are in the example folder. A header
that is also a namespace (`textutil`) makes its C++ namespace a package. The rules for mixed
sources, `//gocpp:cflags` and the build are in the [interop guide](../spec/interop.md) and in
[cookbook recipe 9](../spec/interop-cookbook.md#9-write-a-c-helper-next-to-your-gp-file).

## Try it

```sh
cd examples/sqlite
gocpp run .          # needs the sqlite3 library (pkg-config sqlite3)
```

The dev shell of the repository (`nix develop`) has sqlite, raylib, SDL2 and zlib.
Typed interop (`-interop=typed`, which needs clang++) checks the types of these calls itself
instead of leaving them to the C++ compiler: [typed interop](../spec/typed-interop.md).
