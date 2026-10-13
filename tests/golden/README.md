# tests/golden

A golden test is a `.gp` file and a snapshot of the C++ that gocpp generates for it:
`x.gp` and `x.cpp.golden`. The harness runs `gocpp emit x.gp` in this folder, once per C++
compiler (`GOCPP_CXX`), and the output must equal the snapshot byte for byte. Keep this set
small: every change to the generated code breaks the snapshots, so behavior belongs in
[run tests](../run/README.md).

[hello.gp](hello.gp) (its snapshot comes with Phase 02, see below):

```go
// phase: 02

package main

import "fmt"

func add(a, b int) (int, bool) {
	return a + b, a+b > 10
}

func main() {
	s, big := add(7, 5)
	fmt.Println("hello, gocpp", s, big)
}
```

- The file has only the phase line (and maybe a `//go:build` line, see
  [tests/README.md](../README.md)); until that phase the test is skipped.
- `go test ./tests/... -run Golden -update` writes the snapshots of the tests whose phase is
  reached; then look at `git diff` before you commit them. With `-update`, the first compiler
  writes the snapshot, and the others must print the same C++.
- The generated C++ has absolute paths (`#line` directives). The harness replaces the path of
  the repository with `$ROOT` (`#line 7 "$ROOT/tests/golden/hello.gp"`), so a snapshot doesn't
  depend on where the repository is.
- A missing snapshot fails the test and says how to write it. A `.cpp.golden` file without its
  `.gp` file is an error.
