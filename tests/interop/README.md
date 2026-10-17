# tests/interop

An interop test is a run test for a program that uses C or C++ headers and libraries. It has
the format of [tests/run](../run/README.md) (`// Output:`, `// ExitCode:`, `// Stderr:`) and
one more optional line at the top of the file, `// pkg-config: lib1 lib2`: the test is
skipped, with a message, when `pkg-config --exists` doesn't find one of the packages or when
pkg-config isn't installed.

[sqlite_memory.gp](sqlite_memory.gp):

```go
// phase: 04
// pkg-config: sqlite3

//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

func main() {
	var db *sqlite3.sqlite3 // a raw C pointer
	if rc := sqlite3.sqlite3_open(":memory:", &db); rc != sqlite3.SQLITE_OK {
		panic("open failed")
	}
	defer sqlite3.sqlite3_close(db)
	fmt.Println("sqlite: opened an in-memory database")
}

// Output:
// sqlite: opened an in-memory database
```

The `// pkg-config:` line is only for the harness. To build with the library, the program
needs its own `//gocpp:pkg-config` directive.

A test can also be a **folder** with a `main.gp` file: a package with its own `.h`, `.c` and
`.cpp` files, which the harness runs (or, for an error test, builds) as `gocpp run .` in that
folder, so the folder's C and C++ files are compiled and linked. [helper_pair/](helper_pair) is a
`.hpp` + `.cpp` helper, [helper_c_pair/](helper_c_pair) a `.h` + `.c` helper, and
[helper_pair_error/](helper_pair_error) an error test. [helper_h_cpp_pair/](helper_h_cpp_pair),
[helper_h_cpp_guards/](helper_h_cpp_guards) and [helper_h_cpp_error/](helper_h_cpp_error) are the
`.h` + `.cpp` pair with its two fixes and the hint for the trap, and
[helper_sources/](helper_sources) covers the header extensions, names and which source files are
built. The `// phase:`, `// Output:` and `// ERROR`
lines are those of `main.gp`; the other files of the folder are not tests.

A header that a test includes from this folder (`"./helper.h"`) lives next to it; files that
are not `.gp` files are not tests. Libraries such as SDL can write to stderr; say so with a
`// Stderr:` line, because without one stderr must be empty.

The file [vector](vector) (no extension) is not a header of a test: it is an `#error`, which
shows that a file next to the `.gp` files can't hide a system header (`<vector>`), because local
headers are found with `-iquote` and the package's folder ([quoted_headers.gp](quoted_headers.gp)).

## Interop error tests

A test with `// ERROR` comments is an interop error test: a program whose C++ doesn't compile or
link because of an error in the program (Phase 04 README §2.11, step 4.17), which `gocpp check`
can't see. The harness builds it with each compiler, `gocpp build -o <temp>/prog file.gp` (with
the flags of `GOCPP_FLAGS`), and it passes when the build exits with 1 and reports exactly the
errors of the `// ERROR` comments, on their lines, and nothing else. The comments work as in
[tests/errors](../errors/README.md); a regexp must match the message of both compilers, after
`C++: `. The line after the errors that counts the errors left out ("(and 1 more error in gocpp's
code: -cpperrors=full shows it)") is allowed. Such a test has no `// Output:` block and no
`// ExitCode:` or `// Stderr:` line; it may have `// pkg-config:` and `//go:build`.
[cpp_error_arguments.gp](cpp_error_arguments.gp):

```go
func main() {
	cstdio.printf(format(), 1) // ERROR "C\+\+: (cannot convert|no viable conversion from) 'gp::string' to 'const char ?\*'"
	_ = cstdlib.abs(1, 2)      // ERROR "C\+\+: too many arguments to function"
	cstdio.puts("ok")
}
```
