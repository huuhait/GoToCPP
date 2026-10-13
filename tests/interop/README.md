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

A header that a test includes from this folder (`"./helper.h"`) lives next to it; files that
are not `.gp` files are not tests. Libraries such as SDL can write to stderr; say so with a
`// Stderr:` line, because without one stderr must be empty.
