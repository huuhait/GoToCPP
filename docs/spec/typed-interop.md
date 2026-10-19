# Typed interop: the reference collector and the probe (steps 12.1 and 12.2)

Typed interop (Phase 12, [README](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md))
asks **clang** for the real C++ types of exactly the C/C++ names that a program uses. This
document is the contract of the first two steps, which every later step builds on:

| Step | Package | What it does |
|---|---|---|
| 12.1 reference collector | `internal/typedinterop/collect` | lists what a checked package asks of C/C++, as canonical *reference shapes* |
| 12.2 probe | `internal/typedinterop/probe` | writes one probe file per package, runs clang, reads the typed answer for each reference |
| glue | `internal/typedinterop` | `Analyze(ctx, pkg, fset, opts)`: collect, kind probe, probe; `Report` (JSON for the developer tool) |

Nothing in the opaque pipeline uses these packages: they are additive. The only change outside
them is `internal/interop/typedhooks.go`, which exports the compiler arguments, environment and
dependency-file reader that the kind probe already had, so the typed probe runs the compiler
the same way (same `-std=c++20`, `-iquote dir`, package flags without message flags, warning
policy, `-MD`).

**clang++ is required** (the JSON AST dump). `probe.FindClang` returns `probe.ErrNoClang`
(wrapped) when it is missing, or is not clang; step 12.11 falls back to opaque mode on it. The
program in `GOCPP_CLANG` is used when it is set.

## 1. Collector (`collect`)

`collect.Collect(pkg *check.Package, fset) *Set` runs after `go/types` and the opaque pass. It
makes **one reference for every expression that the opaque pass marks opaque**
(`check.Opaque.IsOpaque`), plus one for every conversion `T(x)` of a C/C++ value (check
rewrote it to `gp_cgoh.As[T](x)`; its result has a Go type, but whether C++ allows it is
clang's answer), plus one per local variable that is declared from a C/C++ value (`x := f()`:
the reference of the *type* of `x`, what `auto` gives).

A `Ref` has:

| Field | Meaning |
|---|---|
| `ID` | index in `Set.Refs`; the probe names its declarations `gp_probe_<ID>`; IDs follow the sorted keys |
| `Kind` | the shape of the root: `path`, `inst`, `call`, `member`, `index`, `slice`, `unary`, `binary`, `deref`, `addr`, `paren`, `literal`, `local`, `type`, `other` |
| `Key` | the canonical text of `Expr`; equal for references with the same C++ meaning |
| `Expr` | the reference as C++ pieces (`collect/ir.go`): `Path`, `Inst`, `IndexOrInst`, `Call`, `Select`, `Unary`, `Binary`, `Star`, `Addr`, `Paren`, `Composite`, `Lit`, `GoType`, `Stand`, `Arg`, `Decay`, `DeclRef`, `TupleElem`, `RangeElem`, `SliceX`, `Unknown` |
| `Sites` | where it is used: file (relative to the package folder), line, column, **context** (`stmt`, `callee`, `template`, `arg`, `convert`, `operand`, `member`, `index`, `init`, `assign`, `return`, `cond`, `range`, `constant`, `element`, `type`), the Go type it goes to when known (`Target`: the compatibility rule of README §2.3 needs it), and the text as the user wrote it |
| `Standalone` | false for a name (path, member, instance) that is only the function of a call or the template of an instance: the call/instance reference has the type, the name is not probed |
| `WantValue` | a use needs a compile-time value (a constant declaration, an array length, a `case`): the probe adds a constant probe |
| `Incomplete` | non-empty when a part has no C++ spelling (a Go struct as an argument, a function literal, a literal without a type): listed, never probed, stays opaque |

**Operands that are not C/C++** are written the way the generated code gives them to C++
(Phase 04, §2.6, §2.9):

- an untyped constant is a C literal with C's default type (`5` is `int`, `2.0` `double`, `"x"` a
  string literal, `'b'` a char literal);
- a Go value is `std::declval<T>()`: `int64_t` for `int`, `double`, `bool`, a stand-in for
  `string`, `[]T`, `[N]T`, `map[K]V` (below). A variable, field, index or dereference is an
  **lvalue** (`std::declval<T&>()`); a call or an operator result is a temporary;
- a local variable that holds a C/C++ value is an lvalue of `std::decay_t<decltype((value))>`,
  with the declaration **inlined** into the expression (`y := x.g()` is one `decltype` over
  `f().g()`) while its key is shorter than 300 bytes, else a named alias (`DeclRef`) declared once
  in the probe file;
- a tuple result (`a, b := std.minmax(1, 2)`) is `std::tuple_element_t<I, ...>`; `for k, v :=
  range c` is `gocpp_probe_internal_pr::range_t<...>::key/val` (the mapped type and the key type for a map, `int`
  and the element for a sequence);
- check's own nodes are undone: `gp_cgoh.Cstr(s)` (`C.cstr(s)`) is a `const char*`, `CString` a
  `char*`, `GoString` a gocpp string; a comparison `gp_cgoh.Cmp(x == y)` is the comparison.

**The set is canonical.** References with the same C++ meaning are merged; references are
sorted by key and sites by position. It does not depend on the order of declarations, on
comments or spacing, or on the names of local variables. `Set.Format` is the text form used by the
golden files (`collect/testdata/golden`, 30 programs of `tests/interop`). `Set.KindPaths()` lists
the paths (and every prefix of them) that the kind probe (`interop.ProbeKinds`) must classify
before the probe file can be written. `collect.NewSet(refs...)` builds a set by hand (tools and
tests).

## 2. Probe (`probe`)

`probe.Generate(set, GenConfig{Includes, Kinds, StringToCStr}) *Source` writes the probe file;
`(*Runner).Run(ctx, Config, src)` runs it and returns a `*Table`; `(*Runner).Probe` does both.

### 2.1 The file

```cpp
// gocpp typed interop: probe file (generated)
#include <cstddef> ... <utility>            // helpers' own headers first
#include <cstdio>                           // the package's header imports, in file order
extern "C" { #include "./legacy.h" }       // as the build includes them (interop.ExternC)
namespace gocpp_probe_internal_pr { struct string {...}; template <class T> struct slice {...}; ... }
template <class T> struct gocpp_probe_internal_c {};          // canonical-type probe
template <auto V> struct gocpp_probe_internal_val {};         // constant probe
using gp_probe_0 = gocpp_probe_internal_c<decltype((BUFSIZ))>;
using gp_probe_0_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((BUFSIZ))>;
constexpr auto gp_probe_0_s = gocpp_probe_internal_pr::str_of((BUFSIZ));
using gp_probe_1 = gocpp_probe_internal_c<::std::vector<int64_t>>;
```

- **Names**: `gp_probe_N` (the main probe), `gp_probe_N_alt` (the other reading of an ambiguous
  reference), `gp_probe_N_v` (integer/float/bool constant), `gp_probe_N_s` (string constant), and
  `gp_decl_<hash>` (the alias of a local variable's type; an error in it belongs to the
  variable's reference).
- **Expression probes** are `gocpp_probe_internal_c<decltype((E))>`: the double parentheses make a reference type
  say "lvalue" (`T&`), "xvalue" (`T&&`) or "prvalue" (plain `T`). Wrapping the type in `gocpp_probe_internal_c<...>`
  makes clang print its **canonical** spelling (`std::basic_string<char>`, `_IO_FILE *`,
  `long &`): the plain `desugaredQualType` of a bare alias still contains member typedefs
  (`value_type &`). Type probes (a reference that names a type) are `gocpp_probe_internal_c<T>`.
- **A path is rendered with the kinds** from the kind probe: `::` after a namespace, a type or a
  template; `.` (through `gocpp_probe_internal_pr::deref`, so a pointer is dereferenced) after a value, a macro or
  an object; no leading `::` for a macro; `X<args>` for a class template or function template
  with explicit arguments, an index for a value. A path whose kind is unknown (or missing) is
  **ambiguous**: both readings are probed (`_alt`), the first that compiles wins.
- **Value categories mirror the generated code**: Go variables are `declval<T&>()`, temporaries
  `declval<T>()`, so `std::getline(stream, line)` and `swap` probe fine, and a function taking
  `T&` refuses a temporary, as the build does.
- **Stand-ins** (`gocpp_probe_internal_pr::string`, `slice<T>`, `array<T, N>`, `map<K, V>`) have the operations of
  the runtime's types: `string` converts to `std::string_view` only (not `const char*`, as in
  Phase 04: `GenConfig.StringToCStr` adds the automatic conversion of step 12.6). **Results can
  name them** (`std::map<gocpp_probe_internal_pr::string, long>`); the type mapper (12.4) maps them back to Go's
  types.
- **Constants**: the integer probe reads `ConstantExpr.value` of the template argument
  (`__int128`, parsed with `math/big`: negative values, `unsigned long long` macros and enumerators
  are exact); a float goes through `std::bit_cast<uint64_t>((double)v)` (exact bits, decoded with
  `math.Float64frombits`); a string macro is read from the `StringLiteral` in the AST (adjacent
  literals joined, C escapes decoded) only when the literal is the initializer itself and the
  probe's type is `const char *` (the `""` inside `__builtin_nanf("")` is not the value of `NAN`).
  A value probe that fails (a non-constant macro like `errno`, or a string asked for a number)
  is **not an error of the reference**: it sets `Result.ValueError`. A `long double` or
  `__float128` constant is a value only when it converts to a `double` and back unchanged
  (`LDBL_MAX` and `M_PIl` have `ValueError` "not exactly representable"); a `__int128` or
  `unsigned __int128` constant is exact up to the range of `__int128` (an `unsigned __int128`
  above 2^127-1 has `ValueError`).
- **Helper names**: the helpers of the probe file are `gocpp_probe_internal_c`,
  `gocpp_probe_internal_val` and the namespace `gocpp_probe_internal_pr`. A compiler error on a
  line of the helper block (a header with a macro of that name) fails the run with a message that
  says so, and a probe whose type is not wrapped by `gocpp_probe_internal_c` is never OK.
- **Header packages named like their namespace** (`import "./oop.hpp"` for `namespace oop`):
  `oop.scale` is looked up in the namespace first (`::oop::scale`), then at the global level, as
  the opaque build does (`lower.cBase`). The collector keeps the header package name in
  `Path.Hdr` (not in the key); `typedinterop.Analyze` asks the kind probe a second time, with the
  `oop::...` paths, when the name is a namespace.
- **Types are stable across folders**: the position of an unnamed type
  (`(unnamed struct at /abs/dir/edge.hpp:25:3)`) is dropped: `Type` and the messages say
  `(unnamed struct)`. `Const`, `Ref` and `Base` read the declarator only, not the arguments of
  templates (`const std::vector<int *>` is const; `void (S::*)() &` is a value, not a reference).

### 2.2 Running clang

```
clang++ -std=c++20 [-iquote dir] <package flags> -fsyntax-only -Xclang -ast-dump=json
        -Xclang -ast-dump-filter=gp_probe_ -w -ferror-limit=0 ... -MD -MF dep -MT gp_probe -x c++ gp_probe.cpp
```

(`-ast-dump-filter=gp_probe_` matches the names that contain it, including `_v`, `_alt`, `_s`.
Measured on clang 21: 0.12 s and 28 KB of JSON for three headers; the unfiltered dump of the same
file is 162 MB.)

- The dump is **a stream of top-level JSON objects**, not an array. `probe.ScanDecls` finds the
  end of each object with a state machine over a fixed buffer and hands the bytes of one object at
  a time to the caller; an object larger than `Config.MaxDecl` (8 MB) is dropped and only its
  name (read from its first 16 KB) is kept, so memory is the buffer plus the limit whatever the
  dump's size. The whole dump is limited to `Config.MaxJSON` (1 GB): beyond it the compiler is
  killed and `Run` fails.
- **A failed probe is not in the dump.** Its error comes from the compiler's messages (stderr,
  at most 4 MB kept, `Stats.StderrTruncated` otherwise): each error with the notes after it is
  attributed to the probe line it is about, also an error inside a header caused by an
  instantiation that the line asked for (through the note "requested here"). The result has
  `Error`, `ErrorAt`, `Notes` (clang puts the overload candidates there) and `AltError`.
  Errors that belong to no probe (an unrelated header with an error, a missing header) are
  `Table.HeaderErrors`; the other probes still have their results.
- 30 s timeout (`Config.Timeout`, else the environment variable `GOCPP_PROBE_TIMEOUT` as a Go
  duration; the tests set 5 minutes), the compiler runs in its own process group which is killed
  when the run ends (timeout, cancel, or error), temporary files (the probe file and the
  dependency file) live in a folder that is removed on every path. A compiler that dies in the
  middle of the dump is reported with its signal and its messages, like one that dies at once.
- `Table.Deps` are the headers that clang read (from the `-MD` file) and the places in front of
  them where the same top-level include would hide them (the package folder, `-iquote` and `-I`
  folders that come first): a missing file has its own hash, so the appearance of a shadowing
  header changes the key. Not covered: a header that shadows an include of another header.

### 2.3 The result table

`Table.Results[id]` is a `probe.Result`:

| Field | Meaning |
|---|---|
| `Skipped` | why the reference has no probe (`incomplete: ...`, `name only: ...`, `a namespace`, `C++ has no slice expression`, a part with no C++ spelling) |
| `OK`, `Form`, `Alt` | a probe compiled; was it an expression or a type; was it the alternative reading |
| `Type` | the canonical type as clang printed it; for an expression the type of `decltype((E))`, so `long &` for an lvalue |
| `Base`, `Ref`, `Category`, `Const` | `Type` without the reference; `""`, `&` or `&&`; `lvalue`/`xvalue`/`prvalue` (empty for a type); top-level `const` of `Base` |
| `Value` | `Const{Kind: int|bool|float|string, Int *big.Int, Float, Str}` |
| `ValueError` | the compiler's error for a failed value probe |
| `Error`, `ErrorAt`, `Notes`, `AltError` | the failure of the probe |

### 2.4 Keys and the in-memory memo (for step 12.7)

- `probe.PreKey(cfg, src)`: hash of the probe text, compiler (path, arguments, version, identity),
  package folder (on purpose: quoted includes and messages depend on it), flags, `MaxDecl`,
  `MaxJSON` and the environment of `build.KeyEnv` (`CPATH`, `CPLUS_INCLUDE_PATH`,
  `NIX_CFLAGS_COMPILE*`, `CCC_ADD_ARGS`, `NIX_HARDENING_ENABLE`, ...: the same list as the
  runtime's object files and the kind probe).
- `probe.Key(cfg, src, deps)` = `PreKey` and the **contents** of every header the compiler read
  (`DepsHash`: sha256 of the bytes; a stat shortcut skips files whose size and time did not change
  only when they were already older than 2 seconds when they were hashed (the racy-git rule); a
  missing file has a hash of its own). So editing `./engine.hpp`, or a
  system header, makes another key.
- `Runner` memoizes tables by `PreKey` and re-checks `DepsHash(table.Deps)` on a hit
  (`Table.Cached`); callers of one probe file at the same time share one compiler run
  (singleflight). `Runner.Runs()` counts the compiler runs. **IDs are positions in the sorted keys,
  so they change when a reference is added or removed: anything that outlives a table (12.7's
  cache, hover) must key by `Result.Key`, never by ID.** The disk cache and incremental
  probing are step 12.7.

## 3. Developer tool

`gocpp __typed-probe [-refs] [patterns]` (hidden: not in `gocpp help`) checks the packages, runs
the pipeline for each root package and prints the reference table and the result table as JSON
(`-refs`: only the collector, no clang). Exit status 3 when there is no clang. The JSON has `refs`
(each with its `result`), `includes`, `kinds`, `table` (timing, stats, header errors, deps) and
`source` (the probe file).

## 4. Known limits (what stays opaque)

- A reference with a Go struct, function value or interface operand, a function literal, a
  composite literal without a type (inside another literal), `<-ch`, a slice of a C++ value, or a
  variadic spread is `Incomplete`/skipped.
- Array lengths of C++ constants (`[cstdio.BUFSIZ]byte`) are probed as a type
  (`gocpp_probe_internal_pr::array<uint8_t, BUFSIZ>`) and as a constant; the substitution is step 12.8.
- A call through a local function value, a method value (`f := v.size`) and an embedded-field
  promoted member are not typed here (12.9).
- The kind of a path is the kind probe's: a `NotFound` kind is still probed (clang's own error
  text is the diagnostic).
- Probes use clang even when the build uses g++: headers that branch on `__clang__` can
  differ; step 12.5 turns probe errors into warnings for a g++ build.
