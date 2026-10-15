# tests/diff

A diff (differential) test is a `.gp` program that only uses the part of gocpp that Go has
too. The harness runs it twice, with real Go and with gocpp, and it passes when both print the
same stdout, byte for byte, and exit with the same code. Real Go is the reference, so the file
has no expected output. A `// diff:` line at the top of the file compares more (below).

[small_int_wrap.gp](small_int_wrap.gp):

```go
// phase: 02

package main

import "fmt"

func main() {
	var a int8 = 127
	a++
	var b uint8 = 200
	b += 100
	var c int16 = -32768
	c--
	var d uint16
	d--
	fmt.Println(a, b, c, d)

	var m int8 = -128
	fmt.Println(m/-1, -m, m*m)
}
```

How it runs:

1. Real Go: the harness copies the file into a temporary module as `small_int_wrap.go`, with a
   `go.mod` that says `go 1.25`, builds it with `go build` (with `GOTOOLCHAIN=local`,
   `GOWORK=off`, no `GOFLAGS` and `GOAMD64=v1`: from v3 on gc fuses `a*b + c`, and gocpp gives
   the results without fusing) and runs the binary. It doesn't use `go run`, because
   `go run` turns every failing exit code into 1. If Go can't build the file, the test fails:
   a diff test must be a valid Go 1.25 program.
2. gocpp: `gocpp run small_int_wrap.gp`, once per C++ compiler.

Both programs run in this folder, so a file name that a program opens (`testdata/x.txt`) is the
same file for both.

What is compared, by the `// diff:` line in the header (after the phase line, before the
package clause):

| Line | Compares |
|---|---|
| none | stdout and the exit code. Stderr is not compared (a panic's stack trace differs); when the test fails, both are shown |
| `// diff: stderr` | stdout, stderr (byte for byte) and the exit code: for `println` and writes to `os.Stderr` |
| `// diff: combined` | stdout and stderr merged in one pipe (like `2>&1`), so the order of the writes counts, and the exit code: for output-order tests |

With `stderr` or `combined`, the program must not panic, since a panic prints a stack trace that
gocpp can't print the same way.

```go
// phase: 02
// diff: combined

package main

import "fmt"

func main() {
	fmt.Println("to stdout")
	println("to stderr")
	fmt.Println("to stdout again")
}
```

Besides the phase line, a diff test may have only the `// diff:` line, a `//go:build` line
(see [tests/README.md](../README.md)) and `// lsan: cycle`, for a program that leaks a reference
cycle on purpose, which a sanitizer run then runs without LeakSanitizer
([tests/README.md](../README.md#sanitizers)): `// Output:`, `// ExitCode:` and the other lines
of [run tests](../run/README.md) are errors here.

A test of behavior that differs between machines in Go itself gets a `//go:build` line. The
phase line stays the first line, and Go wants an empty line after the `//go:build` line (else
it is a plain comment). Out-of-range float → integer conversions, for example, give what
amd64 gives, and arm64 saturates instead
([float_to_int.gp](float_to_int.gp)):

```go
// phase: 02

//go:build amd64

// float64 -> integer conversions at the edges and out of range.
package main
```
