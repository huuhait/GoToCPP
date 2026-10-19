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
| [interop_range.gp](interop_range.gp) | Phase 04: `range` over C/C++ values (step 4.7): `gp::range` for the index and the element, `gp::range_keys` for the index, `gp::range_skip` over a temporary container (its constructor arguments in named temporaries), and the copies of a pair result (`gp::values`) |
| [interop_unsafe.gp](interop_unsafe.gp) | Phase 04: `unsafe` and pointers (step 4.10): `unsafe.Pointer` as `gp::ptr<void>` and its conversions, `unsafe.Add`, `Slice`, `SliceData`, `String` and `StringData` over gocpp memory and over C pointers (raw pointers: views that own nothing), a C pointer or function converted with `gp::cgo::as`, `&T{...}` of a C type (`gp::new_`), and a gocpp pointer through a C function's `...` (`p.gp_get()`) |
| [generics_instances.gp](generics_instances.gp) | Phase 07: generic functions (steps 7.1 and 7.2, [ADR-030](../../docs/adr/030-generic-instances.md)): one function for each list of type arguments, `gp_inst_<hash>_<name>`, written before the functions of the unit with prototypes first, a `// Sum[int8]` comment, the body lowered with the type arguments (`int8_t(s + x)` for `int8`, `Celsius` for a defined type, a type switch with `case T` that is a second `td_int` for `Kind[int]`), and an instance as a func value (`gp::func_of<&...>`) |
| [macros_user.gp](macros_user.gp) | Phase 10: user macros (step 10.9, [ADR-075](../../docs/adr/075-user-macros-are-generic-functions.md)): a macro is a function with the place of the use as hidden arguments (`gp_file_`, `gp_line_`, `gp_func_`, `gp_text_cond_`); a lazy parameter and the code of a wrapped function are C++ template parameters of the instance (`template <typename gp_L_msg>`, `template <typename gp_B>`), so the lambdas are passed as they are, with no `gp::func`; the instance is written in the unit that uses it (`// Retry[func()]`); a start-style macro is the first statement, its `func()` deferred unless nil; a wrap-style macro takes the code of the function in a lambda, and each return sets the (boxed) results and returns from the lambda |
| [interfaces.gp](interfaces.gp) | Phase 05: interfaces (steps 5.4 and 5.5): the interface structs and their wrappers, the thunks, method tables and descriptor object of a named type (`gp_td_Rect`, `gp_vm_Rect`, `gp_pm_Rect`), the interned pointer descriptor (`gp_tdp_Rect`), the conversions (`gp::to_iface`, `gp::iface_cast`), a call through an interface, the assertions (`gp::assert_type`, `gp::assert_type2`) and a type switch |
| [iface_methodvalues.gp](iface_methodvalues.gp) | Phase 05: method values and expressions of interface methods (step 5.9: the receiver bound by `gp::method_recv`, `Shape.Area` as a lambda) and the hash of keys that hold interfaces (step 5.6: `gp_hash_panics` on a struct with an interface field, none on a plain struct) |
| [select_range_chan.gp](select_range_chan.gp) | Phase 08: `select` and `range` over channels (steps 8.5 and 8.6): the case objects declared in source order (`gp::sel_recv gp_s0{a};`, `gp::sel_send gp_s3{b, "x"_gs};`, `gp::sel_drop`), a channel that is a field held by a copy (`gp::chan<int64_t>(x.c)`) and a call held as it is, `switch (gp::select(...))` with `gp::sel_default`, the case variables declared or assigned from `gp_take()` and `gp_ok()`, `break` and `continue` in a case, `gp::select_unreachable()` after a select whose cases all return, `gp::select()` for `select {}`, and the range loops `gp::range_chan`, `gp::range_chan_drop` and the assignment form |

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

Since Phase 06 step 6.2, `gocpp emit` prints the files of the program one after the other, each after
a line `//// gpkg/<import path>/<file>` (the header `pkg.hpp`, one unit for each `.gp` file,
`pkg_desc.cpp` and `pkg_init.cpp`; see [cpp-mapping.md](../../docs/spec/cpp-mapping.md#files)), so a
snapshot has all of them. The 3-package layout is in
`internal/cppgen/testdata/program/three.golden`.
