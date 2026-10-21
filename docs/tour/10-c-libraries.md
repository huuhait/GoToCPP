# C libraries

A C header is a package named after its file: `"<math.h>"` gives `math.sqrt`. A string constant
is a C string literal; a Go string variable goes to C through `C.cstr`; a `char*` from C comes
back through `C.GoString`.

<!-- program: 04 id=clib -->
```go
package main

import (
	"fmt"

	"<math.h>"
	"<stdio.h>"
	cstring "<string.h>" // "string" is a Go name, so this header needs a name of your own
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

<!-- output -->
```text
gocpp has 5 letters
hello, interop
8 1.4142135623730951
007|ok
```

## What to notice

- The same three rules as in the last lesson: C values are not Go values (convert them), an
  untyped constant is a plain C literal, and a typed Go value next to a C value is an error
  (`C.int(n)`), as in cgo.
- `printf` is variadic and takes Go constants, C numbers and `C.cstr(...)` strings. A Go number
  variable (`n := 5`) is an `int64`, which `%d` does not read: write `C.int(n)`.
- A header can be your own: `import "./mathx.h"` brings a header of your project, and a `.c` or
  `.cpp` file next to your `.gp` files is compiled with them
  ([mixed-sources](../../examples/mixed-sources/main.gp)).
- Libraries come with `//gocpp:pkg-config sqlite3` or `//gocpp:link m`; a callback from C to a
  Go function works too. The [cookbook](../spec/interop-cookbook.md) has SQLite, zlib, SDL2 and
  raylib recipes, and [Interop examples](../guides/interop-examples.md) walks through the
  programs of `examples/`.
- Typed interop is on by default when clang++ is installed (`-interop=auto`): gocpp asks clang++
  for the types of these calls and checks them itself. `-interop=opaque` turns it off, and
  without clang++ it is off anyway; see the [typed interop spec](../spec/typed-interop.md).

Next: [Must functions](11-must-functions.md).
