# Cookbook: C and C++ from gocpp

Task-oriented recipes for the things people do with C and C++ in a `.gp` program. The rules behind
them are in [interop.md](interop.md); here is the code to start from.

| | |
|---|---|
| Status | complete for Phase 04 (opaque interop), written in step 4.19 |
| Every program here is a test | each snippet is a file of [tests/interop](../../tests/interop/README.md) (or [tests/zoo](../../tests/zoo/README.md)) that runs with `g++` and `clang++` and under the sanitizers, and `TestDocSnippets` (tests/docs_test.go) fails when a snippet and its file differ. The output shown is the test's `// Output:` block |
| See also | [interop.md](interop.md) (the rules), [modules.md](modules.md) (headers and C++ libraries of modules), [language.md](language.md) |

**Before you start.** Three rules cover most of what the recipes do:

1. A header is a package named after its file (`"<math.h>"` gives `math.sqrt`), and a C++ namespace needs
   no import (`std.vector[int]` after `"<vector>"`). A header whose name is a Go name needs a name of
   your choosing: `cstring "<string.h>"`.
2. What a C or C++ function returns is a **C/C++ value**, not a Go value. gocpp does not guess its type
   (opaque mode); you convert it where Go needs a Go number or string: `int(v.size())`,
   `string(s)`, `C.GoString(p)`. The C++ compiler checks everything else, and its errors come with the
   `.gp` line.
3. An untyped constant is a plain C literal (`5` is an `int`, `"x"` a string literal). A typed Go value
   next to a C value is an error, as in cgo: convert it (`C.int(n)`).

## Recipes

| I want to | Recipe |
|---|---|
| call a C function | [1](#1-call-a-c-function) |
| use `std::vector`, `std::map` and `std::string` | [2](#2-use-stdvector-stdmap-and-stdstring) |
| loop over a C++ container | [3](#3-loop-over-a-c-container) |
| catch a C++ exception | [4](#4-catch-a-c-exception) |
| pass a Go function to C as a callback, with user data | [5](#5-pass-a-go-function-to-c-as-a-callback) |
| use SQLite | [6](#6-sqlite-open-exec-query) |
| compress and decompress with zlib | [7](#7-zlib-compress-and-decompress) |
| link a library, set a define | [8](#8-link-a-library-pkg-config-link-cflags) |
| write a small C++ helper next to my `.gp` file | [9](#9-write-a-c-helper-next-to-your-gp-file) |
| use a C++ library from a downloaded module | [10](#10-use-a-c-library-from-a-module) |
| use SDL2 or raylib | [11](#11-sdl2-and-raylib-build-only-here) |
| wrap a C handle in a Go type that implements an interface | [12](#12-wrap-a-c-handle-in-a-go-type) |
| know who owns what | [13](#13-memory-who-owns-what) |
| understand an error message | [14](#14-common-errors-and-what-they-mean) |

## 1. Call a C function

Import the header, call the function through the package. A Go string constant is a C string
literal; `C.cstr(s)` gives a Go string variable to C for the length of the statement; `C.GoString(p)`
copies a C string back.

<!-- test: tests/interop/cookbook_c_call.gp -->
```go
package main

import (
	"fmt"

	"<math.h>"
	"<stdio.h>"
	cstring "<string.h>" // "string" is a Go name, so this header needs a name of your choosing
)

func main() {
	// A Go string constant is a C string literal: no conversion.
	stdio.printf("%s has %d letters\n", "gocpp", 5)

	// A Go string variable: C.cstr gives a const char* that lives until the end of the statement.
	name := "interop"
	stdio.printf("hello, %s\n", C.cstr(name))

	// C numbers are C values: convert them to use them as Go numbers.
	n := int(cstring.strlen(C.cstr(name)))
	fmt.Println(n+1, float64(math.sqrt(2)))

	// A char* that C returns is no string until C.GoString copies it.
	buf := make([]C.char, 32)
	stdio.snprintf(&buf[0], 32, "%03d|%s", 7, "ok")
	fmt.Println(C.GoString(&buf[0]))
}
```

<!-- output: tests/interop/cookbook_c_call.gp -->
```text
gocpp has 5 letters
hello, interop
8 1.4142135623730951
007|ok
```

- `printf` and other variadic functions take Go constants, C numbers and `C.cstr(...)` strings. A
  Go number variable (`n := 5`) is an `int64`, which `%d` does not read: write `C.int(n)`, or use
  `%lld` with `C.longlong(n)`.
- The result of `strlen` is a `size_t`: `int(...)` makes it a Go int. `math.sqrt(2)` is a C `double`; the
  conversion `float64(...)` prints it as Go does.
- `fmt` and C's `printf` share the terminal: `fmt` flushes C's `stdout` first, so the lines come out in
  program order, also through a pipe.
- Details: [C holds only the built-in types and helpers](interop.md#what-c-holds),
  [strings at the boundary](interop.md#strings-at-the-boundary).

## 2. Use `std::vector`, `std::map` and `std::string`

C++ types keep C++'s rules: `m[k]` inserts, there is no bounds check on `v[i]` (use `v.at(i)` for one),
and a result is a C++ value.

<!-- test: tests/interop/cookbook_std_containers.gp -->
```go
package main

import (
	"fmt"

	"<algorithm>"
	"<map>"
	"<string>"
	"<vector>"
)

func main() {
	// std.vector[int] is std::vector<int64_t>: gocpp's int is 64 bits.
	v := std.vector[int]{5, 2, 9}
	v.push_back(1)
	std.sort(v.begin(), v.end())
	fmt.Println(int(v.size()), int(v[0]), int(v.back()))

	// std.string is explicit: a Go string converts to it with std.string(s).
	s := std.string("hello")
	s += ", world"
	fmt.Println(string(s), int(s.size()), int(s.find("world")))

	// A C++ map keeps C++ rules: m[k] inserts, and the values come back as C++ values.
	ages := std.map[std.string, int]{}
	ages["bob"] = 31
	ages["alice"] = 29
	ages["bob"]++
	fmt.Println(int(ages.size()), int(ages["bob"]), int(ages.count("carol")))

	// An untyped constant is a plain C literal (an int): templates that deduce one type from two
	// arguments need a typed one.
	n := 3
	fmt.Println(int(std.max(n, int(10))))
}
```

<!-- output: tests/interop/cookbook_std_containers.gp -->
```text
4 1 9
hello, world 12 7
2 32 0
10
```

- `std.vector[int]` is `std::vector<int64_t>`: gocpp's `int` is 64 bits. For a C API that wants
  `std::vector<int>`, write `std.vector[C.int]`.
- A Go string converts to `std::string` only explicitly, `std.string(s)`; as a call argument it converts to
  `std::string_view` by itself (`var sv std.string_view = s` is an error: write `std.string_view(s)`).
  `string(s)` copies a `std::string` back.
- `std.max(n, 10)` does not compile: `n` is an `int64_t` and a bare `10` is a C `int`, and the template
  wants one type. Write `std.max(n, int(10))`.
- More containers are in the [header zoo](../../tests/zoo/README.md) and in
  [tests/interop/cxx_containers.gp](../../tests/interop/cxx_containers.gp): `set`, `deque`, `list`, `array`,
  `stack`, `queue`, `priority_queue`, `bitset`, `tuple`, `pair`, `optional`, `variant`, `span`.

## 3. Loop over a C++ container

`range` works over anything with `begin()` and `end()`.

<!-- test: tests/interop/cookbook_iterate.gp -->
```go
package main

import (
	"fmt"

	"<map>"
	"<set>"
	"<string>"
	"<vector>"
)

func main() {
	v := std.vector[int]{10, 20, 30}

	// range works over anything with begin() and end(). The index and the element are C++ values
	// (a copy of the element, like Go): convert them to use them as Go numbers.
	sum := 0
	for i, x := range v {
		sum += int(i) * int(x)
	}
	fmt.Println(sum)

	// A map gives the key and a copy of the value, in the map's own order.
	m := std.map[int, std.string]{{2, "two"}, {1, "one"}}
	for k, name := range m {
		fmt.Println(int(k), string(name))
	}

	// The size is read once, as Go reads a slice's length once: v does not grow under the loop.
	for _, x := range v {
		if x == 10 {
			v.push_back(99)
		}
	}
	fmt.Println(int(v.size()))

	// The C++ way works too: an iterator is a value, and ++ is the prefix ++it.
	s := std.set[int]{3, 1, 2}
	for it := s.begin(); it != s.end(); it++ {
		fmt.Print(int(*it), ";")
	}
	fmt.Println()
}
```

<!-- output: tests/interop/cookbook_iterate.gp -->
```text
80
1 one
2 two
4
1;2;3;
```

- The index (or the key of a map) and the element are C++ values: **copies**, as in Go. Convert them
  with `int(i)`, `string(name)`.
- The size is read once, so growing the vector inside the loop is safe and not visited. Erasing from
  a `std::list` or `std::map` while looping follows C++'s invalidation rules.
- A `std::string` ranges over bytes (`char`), not runes; `range string(s)` gives runes.
- Details: [range over C++ containers](interop.md#range-over-c-containers-and-c-arrays).

## 4. Catch a C++ exception

A C++ exception that reaches a gocpp function with a `defer` is a Go panic, and `recover()` catches
it. The value is an `error` whose text is `C++ exception <type>: <what()>`.

<!-- test: tests/interop/cookbook_exceptions.gp -->
```go
package main

import (
	"fmt"

	"<stdexcept>"
	"<string>"
	"<vector>"
)

// parse calls a C++ function that throws std::invalid_argument for bad input.
func parse(s string) (n int, err error) {
	// A C++ exception that reaches a function with a defer is a Go panic: recover() catches it,
	// and the value is an error whose text is "C++ exception <type>: <what()>".
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parse %q: %v", s, r)
		}
	}()
	return int(std.stoi(std.string(s))), nil
}

func main() {
	fmt.Println(parse("42"))
	fmt.Println(parse("x"))

	// std::out_of_range from v.at(i) in a function with defers is just as recoverable.
	func() {
		defer func() {
			r := recover()
			_, isErr := r.(error)
			fmt.Printf("recovered a %T, an error: %v\n", r, isErr)
		}()
		v := std.vector[int]{1, 2, 3}
		fmt.Println(int(v.at(5)))
	}()
}
```

<!-- output: tests/interop/cookbook_exceptions.gp -->
```text
42 <nil>
0 parse "x": C++ exception std::invalid_argument: stoi
recovered a runtime.CppException, an error: true
```

- A function **without** a `defer` lets the exception pass (its C++ destructors run); the first function
  with a `defer` on the way turns it into a panic and runs its deferred calls.
- An exception that nothing recovers ends the program like an unrecovered panic: `panic: C++ exception
  std::out_of_range: ...`, the trace, exit code 2.
- To catch one inside C++ and return an error code, write the `try`/`catch` in a [helper
  header](#9-write-a-c-helper-next-to-your-gp-file).
- Details: [C++ exceptions](interop.md#c-exceptions).

## 5. Pass a Go function to C as a callback

C expects a function pointer. A **top-level function** of the package, or a func literal that captures
nothing, is one. State goes through the `void*` user-data argument that C APIs provide; a closure can be
that user data.

The C library (a header next to the program):

<!-- test: tests/interop/cookbook_callbacks.h -->
```c
// A C library that calls back: each() calls fn(user, i) for i in [0, n), and keeps nothing.
#pragma once

#ifdef __cplusplus
extern "C" {
#endif

typedef void (*each_fn)(void* user, int i);

static inline void each(int n, each_fn fn, void* user) {
    for (int i = 0; i < n; i++) {
        fn(user, i);
    }
}

#ifdef __cplusplus
}
#endif
```

The program:

<!-- test: tests/interop/cookbook_callbacks.gp -->
```go
package main

import (
	"fmt"
	"unsafe"

	"./cookbook_callbacks.h"
	"<stdlib.h>"
)

// A callback is a function of the package, or a func literal that captures nothing: C gets a
// plain function pointer. Its parameters are the types of the C signature.
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

// State that a callback needs goes through the void* argument that the C API provides: here a
// pointer to a Go variable (borrowed: it must outlive the C call).
func count(user unsafe.Pointer, i C.int) {
	*(*int)(user) += int(i)
}

// A closure can't be a C function pointer, but it can be the user data: keep it in a variable, pass
// its address, and let one named callback call it.
func callFunc(user unsafe.Pointer, i C.int) {
	f := *(*func(C.int))(user)
	f(i)
}

func main() {
	nums := []C.int{5, 2, 9, 1}
	stdlib.qsort(unsafe.Pointer(&nums[0]), C.size_t(len(nums)), 4, byValue)
	fmt.Println(nums)

	total := 0
	cookbook_callbacks.each(5, count, unsafe.Pointer(&total))
	fmt.Println(total)

	seen := []int{}
	f := func(i C.int) { seen = append(seen, int(i)*10) }
	cookbook_callbacks.each(3, callFunc, unsafe.Pointer(&f))
	fmt.Println(seen)
}
```

<!-- output: tests/interop/cookbook_callbacks.gp -->
```text
[1 2 5 9]
10
[0 10 20]
```

- The callback's parameters are the types of the C signature: `unsafe.Pointer` for `void*`, `C.int` for
  `int`. The count of parameters and the result must match; a mismatch is an error at the `.gp` line.
- A closure that captures variables is not a function pointer: `lib.each(3, func(...) {...}, nil)`
  is an error that says so. Use the user-data form above.
- The user-data pointer is borrowed: the variable (`total`, `f`) must stay alive while C may call
  back. When C keeps the pointer after the call returns, keep the owner in a package variable.
- C++ APIs that take `std::function` or a template callable (`std.sort` with a comparator) accept
  any Go func, closures included, with no tricks.
- A panic that leaves a callback ends the program (exit code 2): do not let panics cross C frames; recover
  inside the callback.
- Details: [callbacks](interop.md#callbacks-and-panics).

## 6. SQLite: open, exec, query

<!-- test: tests/interop/cookbook_sqlite.gp -->
```go
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

// check stops at the first error, with the library's own message.
func check(db *sqlite3.sqlite3, rc C.int) {
	if rc != sqlite3.SQLITE_OK && rc != sqlite3.SQLITE_ROW && rc != sqlite3.SQLITE_DONE {
		panic(C.GoString(sqlite3.sqlite3_errmsg(db)))
	}
}

func main() {
	// sqlite3_open writes the new handle through a **sqlite3: &db is that out parameter.
	var db *sqlite3.sqlite3
	check(db, sqlite3.sqlite3_open(":memory:", &db))
	defer sqlite3.sqlite3_close(db)

	check(db, sqlite3.sqlite3_exec(db, "CREATE TABLE fruit (name TEXT, qty INTEGER)", nil, nil, nil))

	// A prepared statement: bind Go values, step, reset.
	var ins *sqlite3.sqlite3_stmt
	check(db, sqlite3.sqlite3_prepare_v2(db, "INSERT INTO fruit VALUES (?1, ?2)", -1, &ins, nil))
	for i, name := range []string{"apple", "banana", "cherry"} {
		// SQLITE_TRANSIENT makes sqlite copy the text, so C.cstr's pointer need not outlive the call.
		sqlite3.sqlite3_bind_text(ins, 1, C.cstr(name), -1, sqlite3.SQLITE_TRANSIENT)
		sqlite3.sqlite3_bind_int(ins, 2, C.int((i+1)*10))
		check(db, sqlite3.sqlite3_step(ins))
		sqlite3.sqlite3_reset(ins)
	}
	sqlite3.sqlite3_finalize(ins)

	// A query: sqlite3_column_text returns an unsigned char*, which C.GoString copies into a string.
	var q *sqlite3.sqlite3_stmt
	check(db, sqlite3.sqlite3_prepare_v2(db, "SELECT name, qty FROM fruit WHERE qty > ?1 ORDER BY qty", -1, &q, nil))
	sqlite3.sqlite3_bind_int(q, 1, 10)
	for sqlite3.sqlite3_step(q) == sqlite3.SQLITE_ROW {
		fmt.Println(C.GoString(sqlite3.sqlite3_column_text(q, 0)), int(sqlite3.sqlite3_column_int(q, 1)))
	}
	sqlite3.sqlite3_finalize(q)
}
```

<!-- output: tests/interop/cookbook_sqlite.gp -->
```text
banana 20
cherry 30
```

- `//gocpp:pkg-config sqlite3` adds the compile and link flags of the library ([recipe 8](#8-link-a-library-pkg-config-link-cflags)).
- `sqlite3_open(name, &db)` writes the handle through `**sqlite3`: `&db` of a `*sqlite3.sqlite3`
  variable is that out parameter, and `db` has the handle when the statement ends.
- `sqlite3_column_text` returns `const unsigned char*`; `C.GoString` accepts any char pointer and
  copies the text.
- A callback form (`sqlite3_exec` with a row callback) is in [examples/sqlite](../../examples/sqlite/main.gp).

## 7. zlib: compress and decompress

<!-- test: tests/interop/cookbook_zlib.gp -->
```go
//gocpp:pkg-config zlib
package main

import (
	"fmt"
	"unsafe"

	"<zlib.h>"
)

// bytesPtr is the C view of a byte slice (the slice must be non-empty and outlive the call).
func bytesPtr(b []byte) *C.uchar {
	return (*C.uchar)(unsafe.Pointer(&b[0]))
}

func compress(src []byte) []byte {
	// compressBound is the largest size compress can produce; make accepts a C number.
	dst := make([]byte, zlib.compressBound(C.ulong(len(src))))
	n := C.ulong(len(dst)) // in: the room in dst; out: the bytes written
	if zlib.compress(bytesPtr(dst), &n, bytesPtr(src), C.ulong(len(src))) != zlib.Z_OK {
		panic("compress failed")
	}
	return dst[:n]
}

func decompress(src []byte, size int) []byte {
	dst := make([]byte, size)
	n := C.ulong(size)
	if zlib.uncompress(bytesPtr(dst), &n, bytesPtr(src), C.ulong(len(src))) != zlib.Z_OK {
		panic("not zlib data")
	}
	return dst[:n]
}

func main() {
	text := []byte("hello hello hello hello hello hello hello hello")
	packed := compress(text)
	back := decompress(packed, 100)
	fmt.Println(len(text), len(packed) < len(text), string(back) == string(text))
	fmt.Println(zlib.crc32(0, bytesPtr(text), C.uint(len(text))))
}
```

<!-- output: tests/interop/cookbook_zlib.gp -->
```text
47 true true
3106698858
```

- `unsafe.Pointer(&b[0])` is the C view of a Go byte slice's memory: C may use it during the call, not
  keep it, and the slice must not be empty.
- `make([]byte, n)` takes a C number (`zlib.compressBound(...)` is a C `unsigned long`) as its size, as in cgo.
- In and out lengths are `C.ulong` variables passed by address.

## 8. Link a library: pkg-config, link, cflags

Directives are comments before the `package` clause. They apply to the whole package, to the compile
of the generated code, of your own `.c` and `.cpp` files, and of the probes that read the headers.

| Directive | Effect |
|---|---|
| `//gocpp:pkg-config sdl2 x11` | the `--cflags` and `--libs` of those packages |
| `//gocpp:link m pthread` | `-lm -lpthread` |
| `//gocpp:cflags -DNAME=value -Iinclude` | extra compile flags |
| `//gocpp:ldflags -L/opt/lib` | extra link flags |

<!-- test: tests/interop/cookbook_flags/main.gp -->
```go
// Flags are directives, comments before the package clause. They apply to the whole package:
// the compile of the generated code, your own .c and .cpp files, and the probes that read the headers.
//
//gocpp:cflags -DSCALE=10
//gocpp:link m
package main

import (
	"fmt"

	"./scale.h"
)

func main() {
	fmt.Println(int(scale.scaled(27))) // 10 * cbrt(27)
}
```

with `scale.h` next to it:

<!-- test: tests/interop/cookbook_flags/scale.h -->
```c
#pragma once

#include <math.h>

#ifndef SCALE
#define SCALE 1
#endif

// scaled uses a macro that the package's //gocpp:cflags line defines, and libm.
static inline double scaled(double x) {
    return cbrt(x) * SCALE;
}
```

<!-- output: tests/interop/cookbook_flags/main.gp -->
```text
30
```

- Flags that can run code at build time (`-fplugin=`, `@file`, `-specs=`) are refused, like Go does; the
  environment variables `GOCPP_CFLAGS`, `GOCPP_CXXFLAGS` and `GOCPP_LDFLAGS` are yours and are not
  checked. [The allow-list](interop.md#flags-and-the-allow-list).
- A library without a `.pc` file: `//gocpp:cflags -I/opt/foo/include` and `//gocpp:ldflags -L/opt/foo/lib`,
  plus `//gocpp:link foo`.
- A symbol that nothing links is an error at the `.gp` line that uses it, with a hint to add
  `//gocpp:link` or `//gocpp:pkg-config`.

## 9. Write a C++ helper next to your `.gp` file

There is no inline C++ in a `.gp` file. A helper is a header (and, if it is not header-only, a `.cpp`
file) in the package folder; the header is a package like any other. Run the folder with `gocpp run .`
(or `gocpp build .`), which compiles the folder's C and C++ files and links them.

`ledger.hpp`:

<!-- test: tests/interop/cookbook_helper/ledger.hpp -->
```cpp
#pragma once

#include <string>

namespace ledger {

// dollars formats an amount in cents: 1050 -> "$10.50".
std::string dollars(int cents);

// Account is a C++ class that Go code can use like any other C++ type.
class Account {
public:
    explicit Account(int cents = 0) : cents_(cents) {}
    void deposit(int cents) { cents_ += cents; }
    int cents() const { return cents_; }
    std::string text() const { return dollars(cents_); }

private:
    int cents_;
};

}  // namespace ledger
```

`ledger.cpp`:

<!-- test: tests/interop/cookbook_helper/ledger.cpp -->
```cpp
#include "ledger.hpp"

#include <cstdio>

std::string ledger::dollars(int cents) {
    char buf[32];
    std::snprintf(buf, sizeof buf, "$%d.%02d", cents / 100, cents % 100);
    return buf;
}
```

`main.gp`:

<!-- test: tests/interop/cookbook_helper/main.gp -->
```go
package main

import (
	"fmt"

	"./ledger.hpp" // package ledger, and also the C++ namespace ledger
)

func main() {
	fmt.Println(string(ledger.dollars(1050))) // defined in ledger.cpp, built with the package

	acct := ledger.Account(250) // a constructor call
	acct.deposit(75)
	fmt.Println(string(acct.text()), int(acct.cents()))
}
```

<!-- output: tests/interop/cookbook_helper/main.gp -->
```text
$10.50
$3.25 325
```

- Name the header: `.hpp` is C++. A `.h` header that **parses as C** is included inside `extern "C"` for the
  generated code; if your `.cpp` defines its functions with C++ linkage the link fails (the error says why).
  Fix: name it `.hpp`, put `#ifdef __cplusplus extern "C"` guards in it, or write
  `//gocpp:no-extern-c ./x.h`.
- `gocpp run main.gp` builds only `main.gp`, like `go run main.go`: a helper with a `.cpp` needs `gocpp run .`.
- Put a macro, a template or a `try`/`catch` that gocpp cannot write directly in such a header: a
  function-like or statement macro works as a call, an odd one can be wrapped in an `inline` function.
- A helper header that declares `operator<<` makes its type printable with `fmt`.
- Details: [small helpers](interop.md#small-helpers-a-header-next-to-the-gp-file).

## 10. Use a C++ library from a module

A header of a module is imported by its module path. A library that must be compiled ships a `cxx`
recipe in its `gocpp.mod` (its sources, include folders, defines, `pkg-config` packages). The project
lists it like any module, here from a local folder (a `replace`); a published module is downloaded and
cached.

The application's `gocpp.mod`:

<!-- test: tests/interop/module_recipe_cpp/gocpp.mod -->
```text
module example.com/vecapp

require github.com/example/vecmath v0.3.1

replace github.com/example/vecmath => ./third_party/vecmath
```

The library's own `gocpp.mod` (`third_party/vecmath/gocpp.mod`), with the recipe:

<!-- test: tests/interop/module_recipe_cpp/third_party/vecmath/gocpp.mod -->
```text
module github.com/example/vecmath

cxx c++17

// The recipe of the library, shipped by its author.
cxx github.com/example/vecmath {
	src src/*.cpp
	if linux {
		src platform/os_linux.cpp
	}
	if windows {
		src platform/os_windows.cpp
	}
	if darwin {
		src platform/os_darwin.cpp
	}
	include include
	define VECMATH_WIDE      // the layout of Vec3 in the header: both sides
	private-define VECMATH_BUILDING
}
```

`main.gp`:

<!-- test: tests/interop/module_recipe_cpp/main.gp -->
```go
// A C++ library that ships its own recipe (Phase 06 step 6.1d): third_party/vecmath has a gocpp.mod with
// `cxx github.com/example/vecmath { ... }`, and this project says nothing about the library. The recipe
// compiles the library as C++17 (src/*.cpp and the file of the operating system), and its
// `define VECMATH_WIDE` reaches this program's compile too: vec_header_size() is compiled here,
// vec_lib_size() inside the library, and both must see the same struct. The header includes
// <vecmath/detail.h>, which only the recipe's include folder makes findable.
package main

import (
	"fmt"

	"github.com/example/vecmath/include/vecmath/vec.hpp"
)

func main() {
	fmt.Println(vec.vec_header_size() == vec.vec_lib_size(), vec.vec_header_size())
	fmt.Println(vec.vec_dot3(1, 2, 3, 4, 5, 6), vec.vec_os_code())
	fmt.Println(vec.vec_cxx17())
}
```

<!-- output: tests/interop/module_recipe_cpp/main.gp -->
```text
true 32
32 1
1
```

- The import path is the module path plus the header's path in the module. Its folder is an `-iquote`
  folder of the build, so the library's headers can include each other by their own names.
- The recipe's `include` and `define` lines reach the compile of your program and of every probe; a
  `private-define` only the library's own files. The library is compiled once into the object cache.
- A header-only library needs no recipe (`module_header_required` in tests/interop).
- Details: [module headers](interop.md#module-headers), [modules.md](modules.md#c-and-c-libraries-cxx-recipes),
  [ADR-024](../adr/024-cxx-recipes.md).

## 11. SDL2 and raylib (build only here)

Both libraries open windows, so the test suite builds these programs with both compilers and **never
runs them** (they would need a display). They use the library as any C library: header package, `pkg-config`
directive, C structs and macros.

SDL2 (`SDL.h` defines `main` as a macro; gocpp protects its own `main`):

<!-- test: tests/zoo/sdl2.gp -->
```go
// SDL2 in opaque mode, BUILD ONLY (the zoo test never runs a program that needs a display):
// SDL_Init/SDL_CreateWindow/SDL_CreateRenderer, an SDL_Event union read by type, SDL_Rect
// literals, Uint32 flags, and SDL_main (SDL.h defines `main` as a macro: gocpp protects its
// own `main`).
//
//gocpp:pkg-config sdl2
package main

import (
	"fmt"

	"<SDL2/SDL.h>"
)

func main() {
	if SDL.SDL_Init(SDL.SDL_INIT_VIDEO) != 0 {
		fmt.Println("init failed:", C.GoString(SDL.SDL_GetError()))
		return
	}
	defer SDL.SDL_Quit()

	win := SDL.SDL_CreateWindow("gocpp zoo", SDL.SDL_WINDOWPOS_CENTERED, SDL.SDL_WINDOWPOS_CENTERED, 320, 200, SDL.SDL_WINDOW_HIDDEN)
	if win == nil {
		fmt.Println("window failed")
		return
	}
	defer SDL.SDL_DestroyWindow(win)
	ren := SDL.SDL_CreateRenderer(win, -1, SDL.SDL_RENDERER_SOFTWARE)
	defer SDL.SDL_DestroyRenderer(ren)

	box := SDL.SDL_Rect{10, 20, 100, 50}
	for frame := 0; frame < 3; frame++ {
		var ev SDL.SDL_Event
		for SDL.SDL_PollEvent(&ev) != 0 {
			if ev.type == SDL.SDL_QUIT {
				return
			}
		}
		SDL.SDL_SetRenderDrawColor(ren, 0, 0, 0, 255)
		SDL.SDL_RenderClear(ren)
		SDL.SDL_SetRenderDrawColor(ren, 200, 80, 40, 255)
		box.x += 5
		SDL.SDL_RenderFillRect(ren, &box)
		SDL.SDL_RenderPresent(ren)
		SDL.SDL_Delay(1)
	}
}
```

raylib (`rl.RAYWHITE` is a macro that expands to a compound literal):

<!-- test: tests/zoo/raylib.gp -->
```go
// raylib in opaque mode, BUILD ONLY (the zoo test never runs a program that needs a display):
// InitWindow/WindowShouldClose/BeginDrawing, struct values (rl.Vector2, rl.Color), the color
// macros (rl.RAYWHITE is a compound literal macro), and raymath-free vector math in Go.
//
//gocpp:pkg-config raylib
package main

import (
	"fmt"

	rl "<raylib.h>"
)

type Ball struct{ pos, vel rl.Vector2 }

func (b *Ball) step() {
	b.pos.x += b.vel.x
	b.pos.y += b.vel.y
	if b.pos.x < 0 || b.pos.x > 320 {
		b.vel.x = -b.vel.x
	}
}

func main() {
	rl.SetConfigFlags(rl.FLAG_WINDOW_HIDDEN)
	rl.InitWindow(320, 200, "gocpp zoo")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	ball := Ball{rl.Vector2{x: 100, y: 100}, rl.Vector2{x: 2, y: 1}}
	frames := 0
	for !rl.WindowShouldClose() && frames < 3 {
		ball.step()
		rl.BeginDrawing()
		rl.ClearBackground(rl.RAYWHITE)
		rl.DrawCircleV(ball.pos, 12, rl.MAROON)
		rl.DrawText(rl.TextFormat("frame %d", C.int(frames)), 10, 10, 20, rl.DARKGRAY)
		rl.EndDrawing()
		frames++
	}
	fmt.Println(frames)
}
```

- An SDL event is a C `union`: its members are reached directly (`ev.type`; a Go keyword after a `.` is a name).
- `rl.TextFormat("frame %d", C.int(frames))` is a variadic C call: arguments are C numbers, not Go numbers.
- A full game loop with a hidden window and a smoke mode is [examples/raylib](../../examples/raylib/main.gp); under
  `xvfb-run -a` with `XDG_SESSION_TYPE=x11` it runs without a screen.

## 12. Wrap a C handle in a Go type

A Go struct holds the raw C pointer; methods call the C API; the rest of the program sees an interface.
Errors become Go errors.

<!-- test: tests/interop/cookbook_wrap_handle.gp -->
```go
package main

import (
	"errors"
	"fmt"

	"<stdio.h>"
)

// LineWriter wraps a C FILE* in a Go type with Go methods, so Go code (and an interface) can use it
// without knowing about C.
type LineWriter struct {
	f *stdio.FILE // a raw C pointer: C owns the FILE, Close gives it back
}

var errClosed = errors.New("write to a closed LineWriter")

func Open() *LineWriter {
	return &LineWriter{f: stdio.tmpfile()}
}

func (w *LineWriter) WriteLine(s string) error {
	if w.f == nil {
		return errClosed
	}
	// C.cstr is a const char* that lives for this statement; fputs does not keep it.
	if stdio.fputs(C.cstr(s), w.f) == stdio.EOF || stdio.fputc('\n', w.f) == stdio.EOF {
		return errors.New("write failed")
	}
	return nil
}

// Contents reads the file back into a Go string.
func (w *LineWriter) Contents() string {
	stdio.rewind(w.f)
	var buf [256]C.char
	n := stdio.fread(&buf[0], 1, 255, w.f)
	return C.GoStringN(&buf[0], C.int(n))
}

func (w *LineWriter) Close() error {
	if w.f != nil {
		stdio.fclose(w.f)
		w.f = nil
	}
	return nil
}

// Sink is what the rest of the program needs to know.
type Sink interface {
	WriteLine(s string) error
	Close() error
}

func log(out Sink, lines ...string) {
	for _, l := range lines {
		if err := out.WriteLine(l); err != nil {
			fmt.Println("error:", err)
		}
	}
}

func main() {
	w := Open()
	var out Sink = w // *LineWriter implements Sink
	log(out, "first", "second")
	fmt.Print(w.Contents())
	out.Close()
	log(out, "third")
}
```

<!-- output: tests/interop/cookbook_wrap_handle.gp -->
```text
first
second
error: write to a closed LineWriter
```

- A raw `*stdio.FILE` is C's: nothing frees it for you. Give the wrapper a `Close` and call it (`defer w.Close()`).
- A method of the wrapper is a Go method, so the wrapper implements Go interfaces. A C++ method of an
  embedded C++ type is not part of the Go method set.
- Store the handle as a `*T` field and set it to `nil` after the close: a second call can then be told apart.

## 13. Memory: who owns what

gocpp's safety (reference counting) covers Go values. C and C++ values follow **C and C++'s rules**.
The list below says who owns the memory in each case; the program shows each one.

| What you have | Who owns it | What you do |
|---|---|---|
| a pointer that a C function returns (`list_make`, `malloc`) | C | call the library's free function (or `C.free`) when done; never use it afterwards |
| `unsafe.Slice(p, n)` over C memory | C | it is a view: the memory must outlive the slice |
| `C.CString(s)` | you (it is a `malloc`) | `C.free(unsafe.Pointer(p))`, unless the C API takes ownership |
| `C.cstr(s)` | the statement | a `const char*` valid until the end of the statement: do not store it, and C must not keep it |
| `&T{...}` of a C type (`&lib.node{}`) | a Go pointer (reference counted) | nothing: it lives while a Go pointer, a closure, or a C struct member that you stored it in, refers to it |
| `&x` of a C/C++ local | the local | the address is valid until the end of the local's block (C++'s rule); `return &x` is an error |
| a C++ local used by a closure, `go` or `defer` | a shared box | nothing: the closure, the goroutine and the function share it, and it lives until the last one is done |
| `std::unique_ptr`, `std::shared_ptr` | C++ | they are C++ objects with C++ lifetimes; dereference by hand, `(*p).m()` |
| a Go pointer passed to C (`unsafe.Pointer(&x)`) | Go | borrowed: C may use it during the call, never keep it |

<!-- test: tests/interop/cookbook_memory.h -->
```c
// A small C library for cookbook_memory.gp: a linked list that C allocates and frees, and a
// buffer that C allocates.
#pragma once

#include <stdlib.h>

typedef struct node {
    int value;
    struct node* next;
} node;

static inline node* list_make(int n) { // C allocates: C must free (list_free)
    node* head = NULL;
    for (int i = n; i > 0; i--) {
        node* x = (node*)malloc(sizeof(node));
        x->value = i;
        x->next = head;
        head = x;
    }
    return head;
}

static inline void list_free(node* head) {
    while (head != NULL) {
        node* next = head->next;
        free(head);
        head = next;
    }
}

static inline int list_sum(const node* head) {
    int s = 0;
    for (; head != NULL; head = head->next) {
        s += head->value;
    }
    return s;
}

static inline int* ints_make(int n) {
    int* p = (int*)malloc(sizeof(int) * (size_t)n);
    for (int i = 0; i < n; i++) {
        p[i] = i * i;
    }
    return p;
}
```

<!-- test: tests/interop/cookbook_memory.gp -->
```go
package main

import (
	"fmt"
	"unsafe"

	"./cookbook_callbacks.h"
	"./cookbook_memory.h"
	"<memory>"
	"<stdlib.h>"
	"<vector>"
)

// count adds up the indexes that each() passes, through the void* that C hands back.
func count(user unsafe.Pointer, i C.int) {
	*(*int)(user) += int(i)
}

func main() {
	// 1. Memory from C stays C's: a raw pointer, valid until C frees it. Go does not free it for you.
	head := cookbook_memory.list_make(3) // *cookbook_memory.node, owned by C
	fmt.Println(int(cookbook_memory.list_sum(head)), int(head.next.value))
	cookbook_memory.list_free(head) // head and its nodes are dangling now: do not use them

	// 2. A C array becomes a Go slice that is a view, not a copy; C still owns the memory.
	p := cookbook_memory.ints_make(4)
	squares := unsafe.Slice(p, 4)
	fmt.Println(int(squares[3]))
	stdlib.free(unsafe.Pointer(p))

	// 3. A string that C keeps must live in C memory: C.CString copies it, and you free it.
	name := C.CString("kept by C")
	fmt.Println(C.GoString(name))
	C.free(unsafe.Pointer(name))

	// 4. &T{} of a C type is a Go pointer that owns the new object (like Go's &T{}): it lives as
	//    long as something points to it, and a store into a C struct's member keeps it alive too.
	var list *cookbook_memory.node
	for i := 3; i > 0; i-- {
		list = &cookbook_memory.node{value: C.int(i), next: list} // each node keeps the next one
	}
	fmt.Println(int(cookbook_memory.list_sum(list)))

	// 5. A C++ object that a closure or a defer uses lives in a shared box: the closure and the
	//    function see the one vector, and it is alive when the deferred call runs.
	v := std.vector[int]{}
	add := func(x int) { v.push_back(x) }
	add(1)
	add(2)
	defer fmt.Println("deferred sees", int(v.size()))

	// 6. A smart pointer is a C++ object: dereference it yourself to reach the pointee.
	sp := std.make_shared[std.vector[int]](3, 7)
	fmt.Println(int((*sp).size()), int((*sp)[0]))

	// 7. A Go pointer that goes to C is borrowed: C may use it during the call, never keep it.
	total := 0
	cookbook_callbacks.each(4, count, unsafe.Pointer(&total))
	fmt.Println(total)
}
```

<!-- output: tests/interop/cookbook_memory.gp -->
```text
6 2
9
kept by C
6
3 7
6
deferred sees 2
```

- gocpp checks some of this and reports it at the `.gp` line: `return &x` of a local, `&i` of a
  3-clause `for` variable, a Go pointer stored into a member of a struct that C allocated (`head.next = &node{}` after
  `list_make` is the compile error "cannot keep the object alive"; where gocpp can't see the struct it is
  a fatal error at run time). It does not check what a C or C++ function does with a pointer you give it.
- The sanitizers are the safety net for C/C++ lifetime bugs: `gocpp run -asan` builds with AddressSanitizer,
  LeakSanitizer and UndefinedBehaviorSanitizer. The test suite runs every interop test that way.
- Details: [pointers, unsafe and slices](interop.md#pointers-unsafe-and-slices-at-the-boundary),
  [objects in closures and `defer`](interop.md#cc-objects-in-closures-and-defer),
  [types that can't be copied](interop.md#types-that-cant-be-copied).

## 14. Common errors and what they mean

Messages are printed as `./file.gp:LINE: ...`. Those that start with `C++:` come from the C++ compiler
(the line is the `.gp` line); the others are gocpp's. Each row names the test that produces the
message.

| Message | Meaning and fix | Test |
|---|---|---|
| `<string.h> has no package name, because string is a predeclared Go name: give it a name: import cstring "<string.h>"` | the file name is a Go keyword or predeclared name (`string`, `map`, `new`, `select`): choose a name | [rewrite tests](../../internal/load/rewrite_test.go) |
| `undefined: std.vectr (the imported headers declare no ::std::vectr)` | a typo, or the header that declares it is not imported: `import "<vector>"` | [undefined_type.gp](../../tests/interop/undefined_type.gp) |
| `undefined: cstdio.printff (the imported headers declare no ::printff)` | same, for a function | [undefined_func.gp](../../tests/interop/undefined_func.gp) |
| `cannot use cmath.pow(10, 30) (C/C++ value) as int value in variable declaration: convert it, int(cmath.pow(10, 30))` | a C/C++ value does not convert to a Go type by itself: convert it | [implicit_conversion.gp](../../tests/interop/implicit_conversion.gp) |
| `invalid operation: x + n (mismatched types C.int and int): convert one of them: int(x) or C.int(n)` | a typed Go number next to a C number | [mismatched_types.gp](../../tests/interop/mismatched_types.gp) |
| `cannot use n (variable of type int) as C.int value in array or slice literal: convert it, C.int(n)` | a Go value into a C element or field | [literal_mismatch.gp](../../tests/interop/literal_mismatch.gp) |
| `cannot assign to v.at(0) (neither addressable nor a map index expression)` | a call is never a place, as in Go: write `v[0] = 5` | [assign_to_call.gp](../../tests/interop/assign_to_call.gp) |
| `assignment mismatch: 2 variables but ages["bob"] is 1 value: a C++ map has no comma-ok form, ...` | use `ages.find(k)` or `ages.count(k)` | [map_comma_ok.gp](../../tests/interop/map_comma_ok.gp) |
| `invalid argument: v (C/C++ value) for built-in len: call its C++ function, v.size() or std.size(v)` | `len`, `cap`, `min`, `max` do not take C/C++ values | [builtin_len_error.gp](../../tests/interop/builtin_len_error.gp) |
| `invalid operation: v (C/C++ value) is not an interface: a type assertion or type switch needs a Go interface value` | `x.(T)` and `switch x.(type)` need an `any` or interface value | [not_an_interface.gp](../../tests/interop/not_an_interface.gp) |
| `C++: cannot convert 'string' to 'const char*' (a gocpp string is no C string: pass C.cstr(s), or C.CString(s) for a copy that C keeps)` | pass `C.cstr(s)` | [cpp_error_cstring.gp](../../tests/interop/cpp_error_cstring.gp) |
| `C++: no matching function for call to 'max'` | a template that deduces one type from two arguments: write `std.max(n, int(0))` | [cpp_error_max.gp](../../tests/interop/cpp_error_max.gp) |
| `C++: a func with captured variables, or a func value that is only known at run time, can't be a C function pointer` | pass a top-level function and use the user-data argument | [callback_errors.gp](../../tests/interop/callback_errors.gp) |
| `C++: undefined reference to 'gocpp_test_undefined_fn'` plus a hint | the header declares it, nothing links it: add `//gocpp:link` or `//gocpp:pkg-config`, or the `.c`/`.cpp` file; `gocpp run .` builds the folder's files | [cpp_error_link.gp](../../tests/interop/cpp_error_link.gp) |
| `C++: use of deleted function 'X::X(const X&)'` (clang: `call to deleted constructor`) | a C++ type that can't be copied (a `unique_ptr`, a mutex): move it with `std.move`, or use a pointer | [policy_errors_pinned.gp](../../tests/interop/policy_errors_pinned.gp) |
| `C++: this C++ type has no default constructor, so a struct field of it has no zero value` | a struct field of such a type: use `*T` or a smart pointer | [policy_errors_field.gp](../../tests/interop/policy_errors_field.gp) |
| `C++: fmt can't print a C++ type in this value: it has no std::formatter and no operator<<` | print its fields, or declare an `operator<<` in a helper header | [cpp_error_print.gp](../../tests/interop/cpp_error_print.gp) |
| `& of the variable of a 3-clause for loop (i): ...` | copy it, `j := i`, and take `&j` | [addr_loopvar_error.gp](../../tests/interop/addr_loopvar_error.gp) |
| `panic: C++ exception std::out_of_range: ...` and exit code 2 | an exception no function with a `defer` recovered ([recipe 4](#4-catch-a-c-exception)) | [cxx_exception.gp](../../tests/interop/cxx_exception.gp) |
| `expected operand, found 'select'` after `import "<sys/select.h>"` | the header's default name is a keyword and the parser stops before the import rules can say so: import it with a name, `import sel "<sys/select.h>"` | (a [known gap](interop.md#limits-and-known-gaps)) |
| `internal error: generated C++ did not compile` | a bug in gocpp, or code gocpp cannot write yet: report it with the program; `-cpperrors=full` shows the compiler's output | |

`-cpperrors=full` shows the C++ compiler's whole output instead of the trimmed errors. For a message
that reads like a compiler error about gocpp's own names (`gp::...`, `gpkg_...`), look at the `.gp` line it
names first: the line has the mistake, and the message is the C++ compiler's way to say it.
