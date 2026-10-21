# Feature matrix

<!-- Generated from internal/check/gates.go. Don't edit it: change the
tables there and run go test ./internal/check -run TestFeatureMatrix -update-gates -->

This page says which phase of the [plan](../../plans/2026-10-13-gocpp/README.md) brings each
feature of gocpp. The features of Phases 00 to 05 work (`gocpp version` prints `phase 05`); the rows with a later phase are rejected.
Until a phase is finished, the compiler rejects its features with an error that names the phase,
so you never get broken C++ (a few features of a later phase work earlier, and say so in their row,
for example "06 (works from 05)"):

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

## Always available (Phase 10)

The syntax and the meaning of these have no gate and no row below, at every phase and in every command
(`check`, `build`, `run`, `test`, `vet`, `expand`, `fmt`, `lsp`), with no `GOCPP_GATE_PHASE`
([ADR-095](../adr/095-must-and-macros-are-always-available.md)): `!` (must) calls and must functions
(divergence D12), macros with `lazy` parameters and the built-in `#file()`, `#line()`, `#func()`, `#text()`, `#body()`, `#embed()`
(D13, D18), `#Derive(String, Equal, Clone)`, and `//go:embed` on `string` and `[]byte` variables
(the `embed` row below). What such a program uses from the later phases is still gated by those phases
(a must function that imports `strings` needs Phase 09).

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
| type aliases | 03 | `type A = B`. A generic alias (Go 1.24) is a generic type. |
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
| functions without a body | 04 | Extern functions (`//gocpp:extern`: declared in the `.gp` file, defined in C or C++) and the native functions of the standard library (`//gocpp:native`). |
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
| programs of more than one package | 06 (works from 05) | Importing packages of your module (`gocpp.mod`), of `replace`d, vendored and downloaded modules. The standard packages are fine. Works from Phase 05 (step 6.2: one translation unit per file, Go's initialization order across packages); Phase 06 finishes the build system (cache, build modes). |
| `//export` functions | 06 | Functions that C code can call (`-buildmode=c-archive`, `c-shared`). |
| generic functions | 07 (works from 05) | Generic functions, constraints with type sets and `comparable`, instantiating them with explicit or inferred type arguments. Each instance is an ordinary C++ function written with its types (ADR-030). Built in steps 7.1 to 7.10 and open from Phase 05 on. Exemption: `weak.Make` from Phase 03. The standard packages `cmp`, `slices`, `maps` and `iter` are Go's own (step 7.10). |
| generic types | 07 (works from 05) | Generic types and their methods, generic aliases (step 7.3 and 7.7): one struct, method set and descriptor for each instance, named with Go's `%T` text. A C/C++ type as a type argument of a generic type is a user error. Open from Phase 05 on. Exemption: `weak.Pointer[T]` from Phase 03. |
| range over functions | 07 (works from 05) | `for x := range f` over any func(yield func(...) bool) (step 7.8: Go's rewrite and its misuse checks), open from Phase 05 on. `iter.Seq`, `iter.Seq2`, `iter.Pull` and `iter.Pull2` are Go's own (step 7.10). |
| go statements | 08 | One OS thread per goroutine (divergence D7). |
| channels | 08 | Also `close`, and `range` over a channel. |
| select statements | 08 |  |
| test files | 09 | `_test.gp` files and `gocpp test`. |
| promoted fields and methods of several embedded C/C++ types | 12 | A struct that embeds one C/C++ type has its members promoted from Phase 05 (`mv.push_back(1)` for `std.vector[int]`). With several, opaque mode can't tell which one has the member: name the field, `mv.vector.push_back(1)`. Typed interop (`-interop=typed`, which needs clang++) asks the C++ compiler and promotes them. |
| selectors that a Go member and a member of an embedded C/C++ type could both match (name the field) | 12 | In Go the shallowest member wins and the same depth is ambiguous, but opaque mode can't know the members of a C/C++ type, so it can't tell whether `s.x` is the Go field or the C++ member. Name the field: `s.epoint.x` or `s.Deep.Inner.x`. Typed interop (`-interop=typed`) asks the compiler and applies Go's rule. |
| method expressions of promoted members of an embedded C/C++ type | 12 | `MV.size`, `(*MV).push_back` for a struct that embeds `std.vector[int]`. Use a func literal instead: `func(m MV) { m.size() }`. Typed interop does not change this. |
| C macros in constant expressions | 12 | `[cstdio.BUFSIZ]byte`, `const n = cstdio.BUFSIZ`. Typed interop (`-interop=typed`, which needs clang++) probes the value of the macro and accepts them; opaque mode reports them. Write the number in a program that must build without clang++. |
| `C.wstr` and `C.GoStringW` | 14 | Wide strings (UTF-16 on Windows) for the `W` functions: also `C.GoStringWN`. |
| `//gocpp:no-win32-defaults` | 14 |  |
| generic types declared inside functions | never | `func f() { type Pair[K, V any] struct{...} }`. Go accepts them; gocpp has types declared inside functions (also inside generic functions) and generic types at package level, but not both at once. Declare the generic type at package level. |
| `//go:linkname` | never | Only the standard library uses it, and `tools/portstd` turns those uses into native functions (Phase 09). |

## Standard packages

Most other standard packages come in Phase 09 (its README lists them by tier). A few come later, with no phase yet: `encoding/csv`, `compress/gzip`, `archive/zip`, `os/signal` and `net`.

| Package | Phase | Notes |
|---|---|---|
| `fmt` | 02 | `Print`, `Println` and `Printf` with basic values from Phase 02; from Phase 05 every value and verb, the `Fprint` functions for `os.Stdout`, `os.Stderr` and a `fmt.State`, `Append`, `Errorf`, `Stringer`, `Formatter` and the other interfaces; see the table of members below. The rest, since the audit of decision 9.7 (Phase 09): the Scan family (`Scan`, `Scanln`, `Scanf`, `Sscan`, `Sscanln`, `Sscanf`, `Fscan`, `Fscanln`, `Fscanf`) with `fmt.Scanner` and `fmt.ScanState`, native like the printing (Go's `fmt` is not ported). |
| `unsafe` | 03 | `Sizeof`, `Alignof` and `Offsetof` from Phase 03, the rest (`unsafe.Pointer`, `Slice`, `String`, …) from Phase 04. `unsafe.Pointer` keeps gocpp memory alive (divergence D16). |
| `weak` | 03 | `weak.Pointer[T]` is a compiler intrinsic, exempt from the generics gate. |
| `errors` | 05 | Go's own package since step 9.4a (ported by `tools/portstd`; `As` and `Is` use three natives over the type descriptors, as in step 5.7, not the reflect subset). |
| `os` | 05 | `os.Stdout` and `os.Stderr` as the writers of `fmt.Fprint` and friends from Phase 05; the rest of the package, since step 9.6 (files, directories, errors, the environment, `Exit`, pipes), in Phase 09. `os.Root`, `StartProcess`, `os/exec` and `os/signal` are not built. |
| `cmp` | 07 (works from 05) | Go's own package (ported by `tools/portstd`, step 7.10). |
| `iter` | 07 (works from 05) | Go's own package (ported by `tools/portstd`, step 7.10): `Seq`, `Seq2`, and `Pull` and `Pull2` from Phase 08. The coroutine of `Pull` is a goroutine of its own (a thread) that the runtime hands control to and from; a switch costs about a microsecond, where Go's costs 20 nanoseconds. |
| `maps` | 07 (works from 05) | Go's own package (ported by `tools/portstd`, step 7.10): `Keys`, `Values`, `All`, `Collect`, `Insert`, `Clone`, `Copy`, `Equal`, `DeleteFunc` and the rest. |
| `slices` | 07 (works from 05) | Go's own package (ported by `tools/portstd`, step 7.10), with Go's pattern-defeating quicksort (`Sort`, `SortFunc`, `SortStableFunc`), `BinarySearch`, `Insert`, `Delete`, `Compact`, the iterator functions and the rest. |
| `runtime` | 08 | `runtime.Error`, `PanicNilError`, `TypeAssertionError`, `Gosched`, `NumGoroutine`, `GOMAXPROCS`, `NumCPU`, `Goexit`, `LockOSThread`, `Caller`, `Callers`, `CallersFrames`, `FuncForPC`, `Stack`, `GC`, `ReadMemStats`, `Version`, `GOOS`, `GOARCH`, `KeepAlive` and `SetFinalizer` from Phase 08 (step 8.10). Each goroutine has a thread of its own, so `LockOSThread` does nothing and `Gosched` only yields the CPU. Memory is reference counted: `GC` does nothing and a finalizer never runs. The profiles, traces and `AddCleanup` are not built; stdlib/runtime/PORT.md lists them. |
| `runtime/debug` | 08 | `Stack`, `PrintStack`, `SetGCPercent`, `SetMemoryLimit`, `FreeOSMemory`, `SetMaxStack`, `SetMaxThreads`, `SetPanicOnFault` and `SetTraceback` from Phase 08 (step 8.10). `ReadBuildInfo`, `ReadGCStats` and `SetCrashOutput` come with Phase 09. |
| `sync` | 08 | `Mutex`, `RWMutex`, `WaitGroup` (with `Go`), `Once`, `OnceFunc`, `Cond`, `Map`, `Pool` and `Locker` from Phase 08 (step 8.7). `OnceValue` and `OnceValues` are generic functions: the generics themselves work from Phase 05 on, but the package waits for Phase 08. |
| `sync/atomic` | 08 | The function-style API, `Int32`, `Int64`, `Uint32`, `Uint64`, `Uintptr`, `Bool` and `Value` from Phase 08 (step 8.7). `Pointer[T]` is a generic type, written in Go (Phase 07). |
| `time` | 08 | The whole package, since step 8.8 (Go's own `format.go`, `time.go` and `zoneinfo.go`, with natives for the clocks, `Sleep`, the timers and the time zone files). Timers follow Go 1.23: timer channels look unbuffered, `Stop` and `Reset` leave no stale value, and a `Timer` nobody holds is freed. `time.LoadLocation` reads the system's zoneinfo (the `ZONEINFO` variable, `/usr/share/zoneinfo`, `/etc/zoneinfo`); there is no embedded copy (`time/tzdata` is not available). On Windows (Phase 14) the zone data comes later. |
| `runtime/cgo` | 09 | `cgo.Handle` with `NewHandle`, `Value` and `Delete` (step 9.2): a native table, so a handle passes through C and C++ as an integer. |
| `bufio` | 09 | Go's own package (ported by `tools/portstd`, step 9.4b): `Reader`, `Writer`, `ReadWriter`, `Scanner` and the split functions. |
| `bytes` | 09 | Go's own package (ported by `tools/portstd`, step 9.4b), with `internal/bytealg` for the searches. `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq` and `FieldsFuncSeq` return an `iter.Seq` (step 7.10). |
| `context` | 09 | Go's own package (ported by `tools/portstd`, step 9.4b), on `sync`, `sync/atomic`, `time` and the reflect subset. |
| `encoding` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): the `BinaryMarshaler`, `TextMarshaler`, `TextAppender` and related interfaces. |
| `encoding/base64` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `StdEncoding`, `URLEncoding`, `RawStdEncoding`, `RawURLEncoding`, `NewEncoding`, the streaming `NewEncoder` and `NewDecoder`, with Go's error texts. |
| `encoding/json` | 09 | Go's own package (ported by `tools/portstd`, step 9.11) on the reflect subset: `Marshal`, `MarshalIndent`, `Unmarshal`, `Indent`, `Compact`, `HTMLEscape`, `Valid`, `Encoder`, `Decoder` (with `Token`, `More`, `UseNumber`, `DisallowUnknownFields`, `Buffered`, `InputOffset`), `RawMessage`, `Number`, `Marshaler`, `Unmarshaler`, `TextMarshaler` keys and values, struct tags (`omitempty`, `omitzero`, `string`, embedded structs) and Go's error types and texts. Fixed stacks (D24): a document or a value nested deeper than the stack holds (it depends on the compiler and the mode: from about 700 levels into a recursive struct with `-asan` to 10000) gives the error "exceeded max depth" instead of Go's success; over 10000 levels Go's own limit applies (ADR-102). |
| `container/heap` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `Init`, `Push`, `Pop`, `Remove`, `Fix` over a `heap.Interface`. |
| `container/list` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): the doubly linked `List` and its `Element`s. A list is made of counted pointers that point both ways, so a list that is dropped with elements in it is a reference cycle that is not freed (divergence D1). |
| `container/ring` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): the circular `Ring`. A ring is a reference cycle, which is not freed when it is dropped (divergence D1). |
| `encoding/binary` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `BigEndian`, `LittleEndian`, `Read`, `Write`, `Size`, the `Append` functions and the varint functions. |
| `encoding/hex` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `EncodeToString`, `DecodeString`, `Dump`, `Dumper`, `NewEncoder` and `NewDecoder`. |
| `hash` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): the `Hash`, `Hash32` and `Hash64` interfaces. |
| `hash/crc32` | 09 | Go's own package (ported by `tools/portstd`, step 9.11), in its portable form: `ChecksumIEEE`, `Checksum`, `MakeTable`, `New`, `NewIEEE`, `Update` with the slicing-8 tables (no CPU instructions). |
| `hash/fnv` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `New32`, `New32a`, `New64`, `New64a`, `New128` and `New128a`. |
| `text/tabwriter` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `Writer` with `Init`, `NewWriter`, the flags and `Flush`. |
| `log` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `Logger`, `New`, `Default`, the standard logger's `Print`, `Fatal` and `Panic` families, the flags (`Ldate`, `Ltime`, `Lshortfile`...) and `SetOutput`. The file names that `Lshortfile` prints are the `.gp` files. |
| `math/rand` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `New`, `NewSource` (Go's additive lagged Fibonacci generator: a seed gives Go's sequence), `Zipf`, the top-level functions with the runtime's generator (a per-thread splitmix64 seeded from the system, not Go's ChaCha8 stream), `Seed`, `Perm`, `Shuffle`, `NormFloat64`, `ExpFloat64`. |
| `math/rand/v2` | 09 | Go's own package (ported by `tools/portstd`, step 9.11): `New`, `NewPCG`, `NewChaCha8` (with `internal/chacha8rand`: a seed gives Go's stream), `Zipf`, `N`, the top-level functions with the runtime's generator (a per-thread splitmix64 seeded from the system). |
| `path/filepath` | 09 | Go's own package (ported by `tools/portstd`, step 9.11) (Linux tested): `Join`, `Clean`, `Split`, `Base`, `Dir`, `Ext`, `Rel`, `Abs`, `Match`, `Glob`, `Walk`, `WalkDir`, `EvalSymlinks`, `IsLocal`, `Localize`. A pattern or a path with thousands of separators in `Glob` can overflow the 8 MiB stack before Go's limit of 10000 (D24). Go's Windows files are in the package but have not been run (Phase 14). |
| `flag` | 09 | Go's own package (ported by `tools/portstd`): `FlagSet` with `ContinueOnError`, `ExitOnError` (`os.Exit` 2, or 0 for `-h`) and `PanicOnError`, the `Value` and `Getter` types, `Bool`, `Int`, `Int64`, `Uint`, `Uint64`, `String`, `Float64`, `Duration`, `Func`, `BoolFunc`, `TextVar` and `Var`, `PrintDefaults`, `Usage`, `ErrHelp` and Go's error and usage texts. |
| `io` | 09 | Go's own package (ported by `tools/portstd`, step 9.4b; the files were a copy in step 9.6). |
| `io/fs` | 09 | Go's own package (ported by `tools/portstd`, step 9.4b), with `Glob`, `Sub` and the rest; `ReadDir` sorts with `slices.SortFunc`, as Go does. |
| `path` | 09 | Go's own package (ported by `tools/portstd`, step 9.4b): `Clean`, `Join`, `Match`, `Split` and the rest. `path/filepath` comes with `os` in the next steps. |
| `syscall` | 09 | A subset of Go's package for Linux (step 9.6): `Errno` and `Signal` with Go's texts, the `E...`, `SIG...` and `O_...` constants, `Stat_t` and the other types that `os` needs, the environment and the file system calls. No `Syscall`, `ForkExec` or socket functions; Windows comes with Phase 14. |
| `reflect` | 09 | A subset, without `MakeFunc`, calling methods by name, `StructOf` and `FuncOf` (divergence D6). |
| `regexp` | 09 | Go's own package (ported by `tools/portstd`, step 9.4c), on `regexp/syntax`, `sync` (the machine pool), `slices` and `unicode`: `Compile`, `MustCompile`, the `Find`, `Replace`, `Split` and `Match` families for strings, byte slices and rune readers, `Longest`, named groups, `QuoteMeta`. Linear-time matching (the backtracker, the one-pass matcher and the NFA), with Go's error texts. |
| `regexp/syntax` | 09 | Go's own package (ported by `tools/portstd`, step 9.4c): the parser (`Parse`, with the `Perl` and `POSIX` flag sets), `Simplify`, `Compile` to a `Prog`, and `Regexp.String`. |
| `sort` | 09 | Go's own package (ported by `tools/portstd`, step 9.4c): `Sort`, `Stable`, `Slice`, `SliceStable`, `Ints`, `Strings`, `Float64s`, `Search`, `Find` and the rest, with Go's pattern-defeating quicksort (the order of equal elements is Go's). `Slice` swaps through the reflect subset (`internal/reflectlite.Swapper`). |
| `strconv` | 09 | Go's own package (ported by `tools/portstd`, step 9.4a), with `internal/bytealg` for `IndexByteString`. |
| `strings` | 09 | Go's own package (ported by `tools/portstd`, step 9.4b), with `internal/bytealg` for the searches. `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq` and `FieldsFuncSeq` return an `iter.Seq` (step 7.10). |
| `testing` | 09 |  |
| `unicode` | 09 | Go's own package (ported by `tools/portstd`, step 9.4a): the tables, `Is*`, `To*`, `SimpleFold`, `SpecialCase`. |
| `unicode/utf8` | 09 |  |
| `unicode/utf16` | 09 |  |
| `math` | 09 |  |
| `math/bits` | 09 |  |
| `embed` | 10 (works from 05) | Always available for `//go:embed` on `string` and `[]byte` variables and for `#embed(...)` (step 10.8, ADR-095). `embed.FS` needs `io/fs` (Phase 09) and is an error until then. |
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
| `iter.Pull` | 08 |
| `iter.Pull2` | 08 |
| `errors.Trace` | 10 |
| the rest of `fmt` | 09 |
| the rest of `unsafe` | 04 |
| the rest of `os` | 09 |

## Format strings

Before Phase 05 the compiler checks the verbs of the format strings of `fmt.Printf`, `fmt.Sprintf` and `fmt.Errorf`, and a format string must be a constant. From Phase 05 `fmt` checks nothing at compile time: every verb of Go's `fmt` works, and a bad one prints Go's error form at run time (`%!z(int=1)`). Width, precision and flags are in the table of language features.

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
