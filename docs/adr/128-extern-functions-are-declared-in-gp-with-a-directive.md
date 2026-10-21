# ADR-128: A function that C or C++ defines is declared in the `.gp` file with `//gocpp:extern`

Status: accepted (2026-10-21; user decision: "better to let .gp handle this")

## Context

A `.gp` file can call only what an imported header declares ([ADR-005](005-interop-header-packages.md)). Dear ImGui's
`imgui_impl_win32.h` keeps `extern IMGUI_IMPL_API LRESULT ImGui_ImplWin32_WndProcHandler(HWND, UINT, WPARAM, LPARAM);`
inside `#if 0` and tells users to copy it into their own `.cpp`. `demos/imgui-dx11` therefore needed a C++ header
(`win32forward.hpp`) with that one line, a C++ file in a program that is meant to need none. The same happens
with a function of a `.c` file of the package that has no header. cgo solves it with the preamble comment; ADR-005
left an inline preamble on the backlog and sent small helpers to a header next to the `.gp` file. That answer
stops working when the declaration is the library's own and cannot be put into a header the library does not ship.

## Decision

A function that is defined in C or C++ and declared by no imported header is declared in the `.gp` file as a Go
function without a body (Go allows that for assembly), with one directive in its doc comment:

```go
//gocpp:extern [C|C++] [name]
func ImGui_ImplWin32_WndProcHandler(hwnd windows.HWND, msg windows.UINT,
	wparam windows.WPARAM, lparam windows.LPARAM) windows.LRESULT
```

- **Grammar.** `C++` (the default) is C++ linkage, `C` is `extern "C"`. The optional name is the C++ name when it
  is not the Go name: qualified for C++ (`ImGui::Begin`, `::sum`), a plain identifier for C. A lone `C` or `C++`
  is the linkage (`C++ C` names a C++ function `C`). The directive is checked by the loader like
  `//gocpp:native` ([ADR-027](027-native-std-functions.md)): the doc comment of a function without a body that is
  not generic and not a method, once; a misplaced one, one before the `package` clause and one in the standard
  library (it has `native`) are errors. It is the second `//gocpp:` directive that goes after the `package`
  clause. It is not in `directiveSpecs` (those are the package directives). The C++ name is spelled as written, so
  a name that is a C++ keyword (`new`, `class`, `int`: a Go function of that name gets a `_` suffix in the code
  gocpp writes, [ADR-010](010-cpp-naming.md), which a C++ function does not have) is an error that names the
  `.gp` line, and so is a qualified name whose first namespace is `gp`, `std` or starts with `gpkg_` (the namespaces
  of gocpp's own code; a user's function is not there, and it is a likely accident).
- **Types.** The signature is written with the types the user already has: the C/C++ types of the header packages
  and of `C` (`windows.HWND`, `C.int`, `*C.char`, `std.vector[C.int]`), `unsafe.Pointer`, `bool`, the sized integer
  types of Go (`int8` ... `uint64`, `byte`, `rune`), `uintptr`, `float32`, `float64` and pointers to them. Go's `int` and
  `uint` are errors: they are 64 bits where a C `int` is 32, so `func neg() int` over `int neg(void)` would read a
  wrong value without a word (the message says to write `C.int`, `int32`, `int64`, `C.long` or `C.longlong`).
  A result is not `unsafe.Pointer` and not a pointer to a number of Go (`*int32`): a gocpp pointer is a managed
  object that can not point into memory of C, and the C++ would not compile (a `void*` is not a gocpp pointer); the
  message says to write a pointer to a C type (`*C.char`, `*C.int`) or to let the C function fill a pointer
  parameter. A parameter may be either, since the call passes the address of a Go variable.
  `internal/check` (`externfuncs.go`) rejects the rest with a message that
  says why: Go strings, slices, maps, interfaces, funcs, channels, types declared in Go, variadic parameters and
  several results. The reason is the C++ name: for C++ linkage the parameter types are part of the linker name, so
  each type needs one C++ spelling that does not depend on how gocpp holds a Go value. A pointer is a raw pointer
  there (`char*`: `gp::cptr<char>` would be another type with another name), `unsafe.Pointer` is `void*`, and the
  numbers are `int32_t`, `uint8_t`, `double`, ... A `const` has no Go spelling: `*std.add_const_t[C.int]` is
  `const int*` (tested), which keeps the rule "write the C++ type" without a new syntax.
- **What is written.** The declaration is made in `internal/lower/extern.go` and goes into the package's `pkg.hpp`, in
  the global namespace after the package's C/C++ headers (`headerNeedsC` counts the extern signatures, so the public
  flags of ADR-025 apply), as `extern "C" T f(P...);` or inside `namespace ns { ... }` for a qualified name. Every
  file of the package and every package that includes the header sees it. A call, in the package or in an importer
  (`lib.F`), is lowered by `cCall` like a call through a header package, with the callee `::name`: untyped constants
  are C literals, gocpp pointers are raw addresses, `&p` is an out parameter. The function is only called: a use as
  a value (`f := F`, a callback) is a `gocpp check` error, since a Go func value over a C function has no
  representation.
- **Once per program.** C++ lets a function be declared again and again, so the declaration is written once per
  package header and identical copies are harmless. What C++ cannot catch is two declarations that disagree in
  different translation units (undefined behavior for C). `lower.Program` records each declaration by C++ name and
  compares the spelled signatures: two C declarations must be identical, a C and a C++ declaration with the same
  parameters are an error (C++ rejects it too), C++ declarations with the same parameters need the same result,
  and other parameters are overloads. The error is at the later declaration and names the earlier. The comparison
  is on the C++ spelling, so `int32_t` and `int` are different spellings: the user writes both alike (the error says
  so when the two signatures differ only in that).
- **Modes.** The directive and the declaration are the same in opaque and typed interop; typed mode type-checks
  the signature's header types as it does for any function. A program with only built-in `C.` types and Go
  numbers needs no header import and no C++ compiler to be checked.
- **Tools.** `gocpp check` runs the signature rules and the value-use rule (cross-package too, with the extern set
  of the package and its imports kept per file by the loader). The editor sees an ordinary function (hover shows
  `func lib.Add3(a C.int, ...) C.int` and its comment, go-to-definition goes to the declaration, also from
  another package, and the errors above are published as you type). `gocpp doc` prints the declaration and the
  comment without the directive. `gocpp vet` has nothing to add. `gocpp fmt` keeps the directive at the end of
  the doc comment, as for `//gocpp:native`. A bodyless function or method in a package of the project without the
  directive (a generic function is go/types' to report) is a checker error: `missing function body (to declare a
  function that C or C++ defines, put //gocpp:extern in its doc comment)`. Before this, `gocpp check` accepted it and
  the emitter said "not supported yet: a function without a body". The runner of Go's own test files (`tools/gotest`)
  sets `GOCPP_ALLOW_BODYLESS=1` (`check.Config.AllowBodyless`) for its gocpp commands: Go compiles those tests, which
  declare functions for assembly, and never links them. It is a switch for that test harness only, not a user feature (like `GOCPP_ALLOW_INTERNAL` in ADR-045); a test that expects the error (an errorcheck test of `missing function body`) needs an exception of its own. If a bodyless function still reaches the lowerer, the build
  says `missing function body` as a user error (for a bindgen view: `a gocpp bindgen view is for reading; it is not
  built`), not as an internal error. The view that `gocpp bindgen` writes
  ([ADR-092](092-bindgen-writes-a-go-syntax-view-from-clang.md)) is a description of a header with bodyless functions
  and stays accepted: a file whose comment before the package clause is `// Code generated by gocpp bindgen from ...`
  is exempt.
- **Build constraints and generics.** The directive belongs to the function, so `//go:build` lines and file name
  suffixes decide whether the declaration exists (`ext_windows.gp`). Generic functions and methods are errors.
- **Calling conventions.** `WINAPI` is `__stdcall`, which is nothing on x86-64 and arm64, the only targets (D9),
  so there is no attribute to write; with MinGW's import libraries `__declspec(dllimport)` is not needed for
  functions.
- **Overloads and `defer`.** An overloaded C++ name has one Go name per overload (`//gocpp:extern shapes::count` on
  `countInt` and `countDouble`). `defer` and `go` of an extern call make the call in a lambda, so the C++ compiler
  picks the overload from the argument types; this is the same as for a call through a header package.
- **Variadic C functions** (`...`) are not supported, as in cgo: a Go `...T` parameter is an error that says to
  write a C wrapper with fixed parameters. Calling a variadic function through a fixed prototype is undefined behavior
  on x86-64, so gocpp does not offer it.
- **Failure modes.** A symbol that nothing defines is reported by `cppLinkFailed` at the `.gp` line of the call with
  the hint "f is declared with //gocpp:extern ... at main.gp:N" and, for C++ linkage, what to try (`//gocpp:extern C`
  when a C file defines it; exact parameter types, `const` and references being part of a C++ name). Declared twice
  with different signatures: the error above.

## Alternatives

- **A cgo-style C preamble comment** (rejected, again): C text in a comment is invisible to the editor and to
  `gocpp check`, needs a C parser in gocpp to find names and types, and is a second language inside the first. The
  declaration is one line of Go here and uses the header packages' types.
- **A line with C++ text** (`//gocpp:decl LRESULT ImGui_ImplWin32_WndProcHandler(HWND, UINT, WPARAM, LPARAM);`, or a raw
  string of declarations): gocpp would have to parse C++ to know the name and the Go-visible signature, there would
  be no hover, no go-to-definition and no check of the call; and the text could say anything. It also does not
  remove the mangling question, it only moves it.
- **A linker symbol name** (`//gocpp:extern _ZN5ImGui...`): exact, but unreadable and different on MSVC, and the C++
  compiler could not check the call against a type.
- **`//gocpp:native` outside the standard library**: it calls a function of the runtime with gocpp types and writes
  nothing for it; extern functions have C types and gocpp writes the declaration. One word for two jobs would hide
  the type rule.
- **`//go:linkname`**: it links a Go symbol to a name of another package; it says nothing about C++ signatures.
- **Scanning the `#if 0` of the library's header**: a hack that works for one header.
- **Variables** (`extern int counter;`): the same mechanism would do, but the problem at hand is a function and a
  variable brings the question of the type's identity and of init order. Left out until someone needs it.
- **Declaring the function in the generated `.cpp` of each file** instead of the header: other packages that call
  an exported extern function would not see it.

## Consequences

Good: `demos/imgui-dx11` has no C++ file (`win32forward.hpp` is deleted; the program builds for Windows from Linux);
a C file of a package needs no header just to be called; the declaration is one Go line with hover and
go-to-definition, and the link error names the line.

Bad:
- The user must spell the C++ types of a C++-linkage definition exactly (`const`, references, `unsigned`); the link error
  and its hint are the only help (the types of a function a header declares are exact because the compiler reads them).
- `int32_t` and `int` are two spellings in the declared-twice check, which is stricter than C++.
- The declared-twice check runs when the program is lowered (`gocpp build`, `gocpp emit`), not in `gocpp check`.
- A bodyless function in a user package is a checker error now (it was accepted by `gocpp check` and refused by the emitter).
- `int` and `uint` are refused even where the C function really takes a 64-bit `long` on the target: write `int64` or
  `C.long`. The refusal costs one word and avoids a silent mismatch.

## Evidence

- `tests/interop/extern_c_file`, `extern_cpp_file` (a namespace, overloads, `const`, `extern "C"` and a pointer out
  parameter, with g++ and clang++), `extern_link_c`, `extern_link_cpp` (the hints), `extern_conflict`,
  `tests/pkgs/extern_two_packages`, `tests/golden/extern`, `tests/errors/extern_*`, the tests of `internal/load`,
  `internal/check`, `internal/lower` and the language server.
- `demos/imgui-dx11` builds with `gocpp build -os windows` on Linux with the declaration in `main.gp` and no other
  source file (the run on the Windows PC is the manager's).

See: [interop.md](../spec/interop.md#functions-that-no-header-declares), [D11](../spec/language.md#d11-gocpp-directives-instead-of-cgo-lines).
