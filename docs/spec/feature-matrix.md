# Feature matrix

<!-- Generated from internal/check/gates.go. Don't edit it: change the
tables there and run go test ./internal/check -run TestFeatureMatrix -update-gates -->

This page says which phase of the [plan](../../plans/2026-10-13-gocpp/README.md) brings each
feature of gocpp. Until a phase is finished, the compiler rejects its features with an error that
names the phase, so you never get broken C++:

```
./main.gp:5:12: interfaces are not supported yet (Phase 05)
./main.gp:9:2: package strings is not available yet (Phase 09)
```

Only your own packages are checked: the standard library can use everything. Each file gets one
error for each feature, at its first use, and each use gets one error, for its first missing
feature. The [language](language.md) describes each feature.

## From the start (Phase 02)

Phase 02 builds a small part of the language, and every phase has it: `package main`,
functions with parameters and several results, the basic types (`bool`, the integer
types, `float32`, `float64`, `string`), constants (with `iota`), local variables,
`:=` and assignments, all operators, `if`, all `for` loops (also `range` over an
integer), `switch` (with `fallthrough`), labels with `break`, `continue` and
`goto`, string concatenation and comparison, `len` of a string, the byte `s[i]` of a
string, conversions between number types, the built-in functions
`print`, `println` and `panic`, and `fmt.Print`, `fmt.Println` and
`fmt.Printf` as statements, with values of basic types (`fmt.Printf` with the verbs
`%v %d %s %t %f %%`).

## Language features

| Feature | Phase | Notes |
|---|---|---|
| panic with a value of a basic type | 02 | `panic("msg")`, `panic(42)`. Any other value comes in Phase 05. |
| composite types | 03 | Arrays, slices, maps and structs, their literals, and `range` over them. |
| pointers | 03 | `*T`, `&x`, `*p`, `new(T)`. Pointers are reference counted (divergence D1). |
| methods | 03 | Value and pointer receivers, method values and method expressions. |
| function values | 03 | Variables, parameters and fields of func type, and a function used without calling it. |
| closures | 03 | Function literals. |
| defer | 03 | Also a deferred closure that changes the named results. |
| type declarations | 03 | Named types: `type Celsius float64`, `type Point struct{...}`. |
| type aliases | 03 | `type A = B`. A generic alias comes in Phase 07. |
| string slicing | 03 | `s[a:b]`. Phase 02 has `+`, comparisons, `len(s)` and the byte `s[i]`. |
| range over strings | 03 | It decodes UTF-8, like Go. |
| conversions between strings and bytes, runes or integers | 03 | `[]byte(s)`, `[]rune(s)`, `string(b)`, `string(r)`. |
| complex numbers | 03 | With the built-ins `complex`, `real` and `imag`. |
| variadic functions | 03 | Declaring them. Calling `fmt.Println` and friends works from Phase 02. |
| package-level variables | 03 | With Go's initialization order. Package-level constants work from Phase 02. |
| init functions | 03 |  |
| width, precision and flags in format strings | 03 | `%5d`, `%-8s`, `%.2f`, `%+v`, `%05d`. Phase 02 has `%v %d %s %t %f` and `%%` without them. |
| header imports | 04 | `import "<raylib.h>"` and `import "./mathx.h"`, and the header packages they give (`raylib.InitWindow`). |
| C++ namespaces | 04 | `std.vector[int]`, `fs "::std::filesystem"`. |
| C types and helpers | 04 | `C.int`, `C.size_t`, `C.CString`, `C.free`, `C.cstr`, `C.GoString`, `C.GoStringN`, `C.GoBytes`. |
| `//gocpp:` directives | 04 | `//gocpp:pkg-config`, `link`, `cflags`, `ldflags`, `extern-c`, `no-extern-c`. |
| functions without a body | 04 | Native functions: the body is in a C or C++ file of the package. |
| module headers | 04 | `import "github.com/ocornut/imgui/imgui.h"`, and `import "<own module>/native/x.h"` for a header in the project. |
| interfaces | 05 | Interface types, conversions to them, interface embedding. |
| any values | 05 | Exemption: the `...any` parameters of `fmt.Print`, `fmt.Println` and `fmt.Printf` take values of basic types from Phase 02, and those of `fmt.Sprint`, `fmt.Sprintf` and `fmt.Sprintln` from Phase 03. A `nil` argument needs Phase 05. |
| error values | 05 | The `error` type. Exemption: a call of `fmt.Print`, `fmt.Println` or `fmt.Printf` as a statement, which drops its results, works from Phase 02; using the results needs Phase 05. |
| type assertions | 05 | `x.(T)` and `v, ok := x.(T)`. |
| type switches | 05 |  |
| struct embedding | 05 | With promoted fields and methods. |
| panic with a value of a non-basic type | 05 | `panic(err)`, `panic(myStruct)`. |
| the # flag, * and argument indexes in format strings | 05 | `%#v`, `%*d`, `%[1]d`. |
| format strings that are not constants | 05 | From Phase 05 `fmt` checks nothing at compile time: a bad verb prints Go's error form at run time. |
| programs of more than one package | 06 | Importing packages of your module (`gocpp.mod`). The standard packages are fine. |
| `//export` functions | 06 | Functions that C code can call (`-buildmode=c-archive`, `c-shared`). |
| generics | 07 | Generic functions and types, generic aliases, constraints with type sets, instantiating generic standard functions. Exemption: `weak.Pointer[T]` from Phase 03. |
| range over functions | 07 | Iterators: `iter.Seq`, `iter.Seq2`. |
| go statements | 08 | One OS thread per goroutine (divergence D7). |
| channels | 08 | Also `close`, and `range` over a channel. |
| select statements | 08 |  |
| test files | 09 | `_test.gp` files and `gocpp test`. |
| must calls | 10 | `F!(...)` (divergence D12). |
| must functions | 10 | `func f!()` (divergence D12). |
| macros | 10 | `macro` declarations with `lazy` parameters, and `#Name(...)` uses, `#body()` and the other built-in macros (divergence D13). |
| `//go:embed` | 10 | Comes with `#embed(...)`. |
| promoted fields and methods of several embedded C/C++ types | 12 | A struct that embeds one C/C++ type has its members promoted from Phase 05 (`mv.push_back(1)` for `std.vector[int]`). With several, check can't tell which one has the member: name the field, `mv.vector.push_back(1)`. |
| selectors that a Go member and a member of an embedded C/C++ type could both match (name the field) | 12 | In Go the shallowest member wins and the same depth is ambiguous, but check can't know the members of a C/C++ type, so it can't tell whether `s.x` is the Go field or the C++ member. Name the field: `s.epoint.x` or `s.Deep.Inner.x`. |
| method expressions of promoted members of an embedded C/C++ type | 12 | `MV.size`, `(*MV).push_back` for a struct that embeds `std.vector[int]`. Use a func literal instead: `func(m MV) { m.size() }`. |
| C macros in constant expressions | 12 | `[cstdio.BUFSIZ]byte`, `const n = cstdio.BUFSIZ`. |
| `C.wstr` and `C.GoStringW` | 14 | UTF-16 strings for Windows APIs. |
| `//gocpp:no-win32-defaults` | 14 |  |
| `//go:linkname` | never | Only the standard library uses it, and `tools/portstd` turns those uses into native functions (Phase 09). |

## Standard packages

Most other standard packages come in Phase 09 (its README lists them by tier). A few come later, with no phase yet: `encoding/csv`, `compress/gzip`, `archive/zip`, `os/signal` and `net`.

| Package | Phase | Notes |
|---|---|---|
| `fmt` | 02 | `Print`, `Println` and `Printf` with basic values from Phase 02; from Phase 05 every value and verb, the `Fprint` functions for `os.Stdout`, `os.Stderr` and a `fmt.State`, `Append`, `Errorf`, `Stringer`, `Formatter` and the other interfaces; see the table of members below. The rest (`Sscan` and friends) comes with Go's `fmt` in Phase 09. |
| `unsafe` | 03 | `Sizeof`, `Alignof` and `Offsetof` from Phase 03, the rest (`unsafe.Pointer`, `Slice`, `String`, …) from Phase 04. `unsafe.Pointer` keeps gocpp memory alive (divergence D16). |
| `weak` | 03 | `weak.Pointer[T]` is a compiler intrinsic, exempt from the generics gate. |
| `errors` | 05 |  |
| `runtime` | 05 | `runtime.Error` and `runtime.PanicNilError` from Phase 05; `Gosched`, `NumGoroutine`, `GOMAXPROCS`, `Goexit` and `LockOSThread` (which does nothing: each goroutine has its own thread) from Phase 08. |
| `runtime/cgo` | 05 | `cgo.Handle`, because its API uses `any`. |
| `os` | 05 | `os.Stdout` and `os.Stderr` as the writers of `fmt.Fprint` and friends from Phase 05; the rest of the package in Phase 09. |
| `cmp` | 07 |  |
| `iter` | 07 | `iter.Pull` and `iter.Pull2` from Phase 08 (they run on a helper thread). |
| `maps` | 07 |  |
| `slices` | 07 |  |
| `sync` | 08 |  |
| `sync/atomic` | 08 |  |
| `time` | 08 | Timers and `Sleep` in Phase 08; formatting, parsing and time zones in Phase 09. |
| `bufio` | 09 |  |
| `bytes` | 09 |  |
| `context` | 09 |  |
| `encoding` | 09 |  |
| `encoding/json` | 09 |  |
| `flag` | 09 |  |
| `io` | 09 |  |
| `io/fs` | 09 |  |
| `reflect` | 09 | A subset, without `MakeFunc`, calling methods by name, `StructOf` and `FuncOf` (divergence D6). |
| `regexp` | 09 |  |
| `strconv` | 09 |  |
| `strings` | 09 |  |
| `testing` | 09 |  |
| `unicode` | 09 |  |
| `unicode/utf8` | 09 |  |
| `embed` | 10 | With `//go:embed` and `#embed(...)`. |
| `go/...` | never | `go/ast`, `go/types` and the other packages about Go source are not planned. |
| `net/http` | never | Not planned: use a C or C++ HTTP library through a header import. |
| `plugin` | never | gocpp builds one native binary (divergence D6). |

## Parts of packages

These come in another phase than their package. A method comes with its type, unless it has a row of its own.

| Name | Phase |
|---|---|
| `fmt.Print` | 02 |
| `fmt.Printf` | 02 |
| `fmt.Println` | 02 |
| `fmt.Sprint` | 03 |
| `fmt.Sprintf` | 03 |
| `fmt.Sprintln` | 03 |
| `unsafe.Alignof` | 03 |
| `unsafe.Offsetof` | 03 |
| `unsafe.Sizeof` | 03 |
| `fmt.Errorf` | 05 |
| `fmt.Stringer` | 05 |
| `fmt.GoStringer` | 05 |
| `fmt.Formatter` | 05 |
| `fmt.State` | 05 |
| `fmt.FormatString` | 05 |
| `fmt.Fprint` | 05 |
| `fmt.Fprintf` | 05 |
| `fmt.Fprintln` | 05 |
| `fmt.Append` | 05 |
| `fmt.Appendf` | 05 |
| `fmt.Appendln` | 05 |
| `os.Stdout` | 05 |
| `os.Stderr` | 05 |
| `runtime.Error` | 05 |
| `runtime.PanicNilError` | 05 |
| `iter.Pull` | 08 |
| `iter.Pull2` | 08 |
| `time.FixedZone` | 09 |
| `time.LoadLocation` | 09 |
| `time.LoadLocationFromTZData` | 09 |
| `time.Parse` | 09 |
| `time.ParseDuration` | 09 |
| `time.ParseInLocation` | 09 |
| `time.Time.AppendFormat` | 09 |
| `time.Time.Format` | 09 |
| `time.Time.In` | 09 |
| `time.Time.Local` | 09 |
| `time.Time.Location` | 09 |
| `time.Time.String` | 09 |
| `time.Time.Zone` | 09 |
| `errors.Trace` | 10 |
| `errors.Wrapf` | 10 |
| the rest of `fmt` | 09 |
| the rest of `unsafe` | 04 |
| the rest of `runtime` | 08 |
| the rest of `os` | 09 |

## Format strings

The compiler checks the verbs of the format strings of `fmt.Printf`, `fmt.Sprintf` and `fmt.Errorf`, so until Phase 09 a format string must be a constant. Width, precision and flags are in the table of language features.

| Verbs | Phase |
|---|---|
| `%%` `%d` `%f` `%s` `%t` `%v` | 02 |
| `%q` | 03 |
| `%T` `%w` `%b` `%c` `%o` `%O` `%x` `%X` `%U` `%e` `%E` `%F` `%g` `%G` `%p` | 05 |
| the other verbs of Go's `fmt` | 05 |

## Built-in functions

`len` (of a string), `print`, `println` and `panic` work from Phase 02.

| Function | Phase |
|---|---|
| `append` | 03 |
| `cap` | 03 |
| `clear` | 03 |
| `complex` | 03 |
| `copy` | 03 |
| `delete` | 03 |
| `imag` | 03 |
| `make` | 03 |
| `max` | 03 |
| `min` | 03 |
| `new` | 03 |
| `real` | 03 |
| `recover` | 05 |
| `close` | 08 |
