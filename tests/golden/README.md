# tests/golden

A golden test is a `.gp` file and a snapshot of the C++ that gocpp generates for it:
`x.gp` and `x.cpp.golden`. The harness runs `gocpp emit x.gp` in this folder, once per C++
compiler (`GOCPP_CXX`), and the output must equal the snapshot byte for byte. Keep this set
small: every change to the generated code breaks the snapshots, so behavior belongs in
[run tests](../run/README.md).

The set shows the main shapes of the generated C++ (Phase 02 README §5: "reviewed and
readable"):

| Test | Shows |
|---|---|
| [hello.gp](hello.gp) | the program of the Phase 02 README (§2.1): prototypes, a `std::tuple` result, `#line` |
| [control.gp](control.gp) | the `for` forms, a range over an integer whose body declares the range variable again, the three `switch` shapes (a C++ `switch`, an if-else chain, a case index for `fallthrough`), labeled `break` and `continue`, `goto` |
| [funcs.gp](funcs.gp) | several and named results, a shadowed variable (`q := q * 10`), Go's order of evaluation for calls (`gp_` temporaries), parallel assignment, `gp::div`, `gp::rem`, `gp::shl`, small integers, float to int, `fmt` |
| [interop_names.gp](interop_names.gp) | Phase 04: the C++ names of C/C++ names (step 4.3): header packages fully qualified (`::printf`), macros unqualified (`EOF`, `errno`), namespaces and a namespace rename, a header package that is also a namespace ([cnames.h](cnames.h)), `::` after a type and `gp::deref(...).` after an object, cgo's tag names, the C/C++ includes after gocpp's header (and `gocpp/fmtcxx.hpp` after them), a gocpp name renamed for a macro (`Bool_`) and a macro `#undef`ed for the runtime (`tuple`) |
| [interop_types.gp](interop_types.gp) | Phase 04: the C++ types of C/C++ type expressions (step 4.2): cgo's C types, raw pointers, templates with gocpp types and constants as arguments, a member type, slices, maps and arrays of C types, a struct with C/C++ fields (no `==`, `gp_hash` or layout check), parameters, results and package variables of C/C++ types |

[hello.gp](hello.gp):

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
