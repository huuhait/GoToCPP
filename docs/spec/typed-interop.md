# Typed interop: the collector, the probe, the declarations, the type mapper, the checker, the code generation, the constants, the promoted members, bindgen and the mode (steps 12.1 to 12.14)

Typed interop (Phase 12, [README](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md))
asks **clang** for the real C++ types of exactly the C/C++ names that a program uses. This
document is the contract of the first eleven steps (and of step 12.14 for the cost of the cold run), which every later step builds on (§5 and §6 are
steps 12.3 and 12.4, §7 is step 12.5, §8 is step 12.6, §9 is step 12.7, §10 is step 12.8, §11 is step 12.9, §12 is step 12.10, §13 is step 12.11):

| Step | Package | What it does |
|---|---|---|
| 12.1 reference collector | `internal/typedinterop/collect` | lists what a checked package asks of C/C++, as canonical *reference shapes* |
| 12.2 probe | `internal/typedinterop/probe` | writes one probe file per package, runs clang, reads the typed answer for each reference |
| 12.3 declarations | `internal/typedinterop/ctype`, `.../probe` (`decls*.go`) | reads clang's type text into a structured type; extracts the records, enums and overload sets that the results name |
| 12.4 type mapper | `internal/typedinterop/ftype` | maps a C++ type (and a probe result) to a `go/types` type of the foreign view, with `Opaque` as the fallback |
| 12.5 interop checker | `internal/typedinterop/icheck` | errors for the C++ compiler's errors, at the `.gp` position, in Go's words; silent when not certain |
| 12.6 code generation | `internal/lower` (`typed*.go`, `keeps.go`), `cmd/gocpp/typedlower.go` | writes the C++ of foreign calls, members, literals and strings from the real types; on in typed mode (§13) |
| 12.8 constants (gap E1) | `internal/check` (`constsubst.go`), `internal/typedinterop` (`consts.go`) | probes the C/C++ names that sit where Go needs a constant and puts their values into the program before `go/types` runs; on in typed mode (§13) |
| 12.9 promoted members (gap E2) | `internal/check` (`cppmembers.go`), `internal/typedinterop` (`members.go`), `.../collect` (`members.go`) | asks the compiler which embedded C++ class has which member name and applies Go's rule of selectors (§11) |
| 12.10 bindgen | `internal/typedinterop/bindgen`, `cmd/gocpp/bindgen.go` | `gocpp bindgen`: a readable Go-syntax view of a header, made from the preprocessor, the AST dump and the probe (§12) |
| 12.11 mode | `internal/typedinterop` (`mode.go`), `cmd/gocpp/interop.go` | `-interop=typed`/`opaque`/`auto`, `GOCPP_INTEROP`, the clang-missing fallback, and the wiring of the steps above into `check`, `build`, `run`, `emit`, `test`, `vet` and `expand` (§13) |
| 12.14 cold probing | `internal/typedinterop/probe` (`pch.go`, `includes.go`, `decls.go`), `internal/interop` (`nscache.go`) | a precompiled header of the library block for probe/function runs, the includes question asked beside the compile, the same header for the type dumps of the program's own types when the answer is whole (otherwise full preambles), the declarations of the called functions' signatures asked beside the probe run, and the kind probe's first miss starts the header (§2.2, §9, §14.3, ADR-117, ADR-121) |
| glue | `internal/typedinterop` | `Analyze(ctx, pkg, fset, opts)`: collect, kind probe, probe, and with `Options.Decls` the declarations and the mapping; `Report` (JSON for the developer tool) |

Nothing in the opaque pipeline uses these packages: they are additive. The only change outside
them is `internal/interop/typedhooks.go`, which exports the compiler arguments, environment and
dependency-file reader that the kind probe already had, so the typed probe runs the compiler
the same way (same `-std=c++20`, `-iquote dir`, package flags without message flags, warning
policy, `-MD`).

**clang++ is required** (the JSON AST dump). `probe.FindClang` returns `probe.ErrNoClang`
(wrapped) when it is missing, or is not clang (or older than clang 17); the mode `auto` falls back to opaque mode on it and `typed` fails (§13). The
program in `GOCPP_CLANG` is used when it is set. For the target windows/amd64 `probe.FindClangFor` finds the clang
(`GOCPP_CLANG`, `x86_64-w64-mingw32-clang++`, the clang of the build, `clang++`) and marks it with the target, which the probes
pass on as `--target`, `-D_WIN32_WINNT` and, from a system other than Windows, the include folders of the MinGW-w64 g++
(`build.ProbeTargetFlags`); the target is part of every cache key (Phase 14 step 14.6, ADR-116).

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
- **The library block is precompiled (step 12.14, ADR-117).** Every probe file starts with the same 18 library headers
  (`stdHeaders`), which clang parses again in each run (0.3 s). The probe runs and the function runs of the declarations
  load a precompiled header of that block instead (`-include-pch`, `probe/pch.go`; §9 says where it is kept and when it is
  stale); the file keeps its `#include` lines, so no line moves and no answer changes. A type run loads it only for the
  program's own types (no name in `std` or with a leading underscore, no template arguments), and uses the answer only when
  every declaration it describes was parsed from the text of a file of the program: clang reads the members of a class that
  comes from a precompiled header only when something uses them, and the JSON dump prints the members read so far
  (`std::mutex` without its methods, `struct _IO_FILE` with a tenth of its fields). A run whose answer describes a file the
  header was made of (compared by file identity, so a link or another spelling does not hide it), or whose location is
  not a file that exists, is made again with **the full block**, which stays the oracle (ADR-121).
  The earlier smaller-preamble optimization and its approval memo were removed after review: equal preprocessor
  output is not equal C++ name lookup. With `using namespace std`, a `requires` expression calling `begin(x)` can
  succeed only with the full block's `std::begin`; both compiles succeed but select different member types and layouts.
  The review example gave size/alignment 16/8 with the full block and 8/4 with the smaller one on Linux, while both
  GCC and Clang runtime preludes agreed with 16/8. A textual heuristic and retry-on-error cannot detect that case.
  The ordinary declaration cache still invalidates on dependency contents, including edits within the same Runner.
  The tests compare the declarations made from the dumps, not byte-identical dump text. `GOCPP_PROBE_PCH=off`
  (also `0`, `false`, `no`) disables PCH use; `wait` makes eligible runs wait for a header that is being made.
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
| `Value` | `Const{Kind: int\|bool\|float\|string, Int *big.Int, Float, Str}` |
| `ValueError` | the compiler's error for a failed value probe |
| `Error`, `ErrorAt`, `Notes`, `AltError` | the failure of the probe |

### 2.4 Keys

- `probe.PreKey(cfg, src)` (`gocpp-probe-v5`): hash of the probe text, compiler (path, arguments, family,
  target OS and architecture, version, identity), package folder (on purpose: quoted includes and messages
  depend on it; slashes and lower case on Windows), flags, `MaxDecl`, `MaxJSON` and the environment of
  `build.KeyEnv` (`CPATH`, `CPLUS_INCLUDE_PATH`, `NIX_CFLAGS_COMPILE*`, `CCC_ADD_ARGS`, `NIX_HARDENING_ENABLE`, ...:
  the same list as the runtime's object files and the kind probe).
- `probe.Key(cfg, src, deps)` = `PreKey` and the **contents** of every header the compiler read
  (`DepsHash`: sha256 of the bytes; a missing file has a hash of its own). So editing `./engine.hpp`, or a
  system header, makes another key. Inside one process a stat shortcut skips files whose size, modification
  time and (on Linux) inode change time did not change and that were older than 2 seconds when they were hashed.
- `Runner` serves a probe file from its memory (and the disk, §9); callers of one probe at the same time share
  one compiler run. `Runner.Runs()` counts the compiler runs. **IDs are positions in the sorted keys, so they
  change when a reference is added or removed: anything that outlives a table (the cache, hover) must key
  by `Result.Key`, never by ID.** The cache stores results by the text of the reference's lines
  (`Source.Units[i].Hash`), which has no ID in it.

## 3. Developer tool

`gocpp __typed-probe [-refs] [-decls] [-stable] [-stats] [patterns]` (hidden: not in `gocpp help`) checks the packages, runs
the pipeline for each root package and prints the reference table and the result table as JSON
(`-refs`: only the collector, no clang; `-decls`: also the declarations and the mapped view of each result;
`-stable`: leave out the timing, the statistics and the cache flags, so that a cold and a warm run print the same
bytes; `-stats`: one line on standard error with the units, hits, batches and compiler runs;
`-cache-stats` and `-cache-clean` show and empty the cache folder and check nothing). Exit status 3 when there is no clang. The JSON has `refs`
(each with its `result`), `includes`, `kinds`, `table` (timing, stats, header errors, deps) and
`source` (the probe file).

## 4. Known limits (what stays opaque)

- A reference with a Go struct, function value or interface operand, a function literal, a
  composite literal without a type (inside another literal), `<-ch`, a slice of a C++ value, or a
  variadic spread is `Incomplete`/skipped.
- Array lengths of C++ constants (`[cstdio.BUFSIZ]byte`) are probed as a type
  (`gocpp_probe_internal_pr::array<uint8_t, BUFSIZ>`) and as a constant; since step 12.8 the constant is put into the
  program before `go/types` (§10), so the array type is a plain Go type.
- A call through a local function value and a method value (`f := v.size`) are not typed here.
  A member promoted from an embedded class is (§11).
- The kind of a path is the kind probe's: a `NotFound` kind is still probed (clang's own error
  text is the diagnostic).
- Probes use clang even when the build uses g++: headers that branch on `__clang__` can
  differ; step 12.5 turns probe errors into warnings for a g++ build.

## 5. Declarations (step 12.3)

The probe says what type an expression has. It does not list the members of that type. Step 12.3
reads them from clang's AST dump of the real declarations, and makes the type text structured.

### 5.1 `ctype`: C++ types as trees

`ctype.Parse(text)` reads a type in the spelling that clang prints (`qualType`, `desugaredQualType`,
diagnostics) into a `Type`: a leaf (`Builtin`, `Named` with its qualified path and template
arguments, `Unnamed` for `(unnamed struct)`, `Special` for `<overloaded function type>` and
`decltype(...)`) under declarator parts (`Pointer`, `LRef`, `RRef`, `Array`, `Func` with the
`const`/`&`/`noexcept`/attribute qualifiers of a method, `MemberPtr`). `String` writes the same
spelling back; for every type text that the headers of the C++ standard library print
(`TestCorpus`: 7,359 spellings from `<vector> <map> <string> <chrono> <filesystem> ...`) the two are
inverse, except for the listed normal forms (the position of an unnamed type is dropped,
`long unsigned int` is `unsigned long`). What it reads: function pointers, `(&)[N]`, nested
templates, non-type template arguments (`std::array<int, 5>`, `ratio<1, 1000>`), pack expansions,
trailing return types, member pointers, `__attribute__((noreturn))` on function types. What it
rejects (an error, never a guess): vector types, `_Atomic`, a type with another attribute,
dependent-name syntax such as `typename T::template X<U>` (a probe never prints it). Nesting is
limited (900 levels) so that a text from a header cannot exhaust the stack; `FuzzParse` checks that
`Parse` never panics and that `Parse(String(t))` equals `t`.

### 5.2 The written type

`Result.Written` is the type with its typedef names kept, from the type tree that clang puts in
the dump of the probe's alias: `FILE *` where `Type` says `_IO_FILE *`, `size_t` for `unsigned
long`, `int32_t` for `int`, `value_type &` for a member typedef (unqualified, as the class has it).
It is empty when it is the same as `Type` or when the dump has no such tree (another clang): readers
must treat empty as "no information". The type mapper uses it only to name a type as the program
does (`Mapped.Alias`).

### 5.3 What is extracted

`Runner.Decls(ctx, cfg, DeclConfig, queries)` answers `DeclQuery`s:

| Query | Answer |
|---|---|
| `{DeclType, "geo::Vec"}` (a canonical type name, or a typedef name) | `Records[canonical]` (`Record`) or `Enums[canonical]` (`Enum`); `Canon[query]` is the canonical spelling (`FILE` is `_IO_FILE`, a typedef of a builtin is that builtin) |
| `{DeclFunc, "geo::f"}` (a qualified function name) | `Funcs[name]` (`FuncSet`: every overload with its type; a function template is `Kind: "template"` with no type) |

A `Record` has `Tag`, `Complete` (a forward-declared handle like `struct _IO_FILE` of another
header is incomplete), `Size`/`Align` (`sizeof`/`alignof` asked of the compiler), `Fields` (name, type,
access, bit-field width, members of anonymous unions), `Statics`, `Methods` (constructors,
destructors, conversions, operators, member templates; `virtual`, `pure`, `override`, `final`,
`static`, `constexpr`, `deleted`, `defaulted`, access, parameter names and defaults), `Bases`,
`Types` (member typedefs, desugared), `Nested`, and `Traits` (aggregate, polymorphic, abstract,
empty, standard layout, trivial, trivially copyable, pod). An `Enum` has `Scoped`, `Underlying`
(`std::underlying_type`, asked of the compiler), `Size`, and the values (`big.Int`; clang's dump
has the value only for an explicit initializer: the others are the previous one plus one).

Member types are written **relative to the scope** in clang's dump (`Vec` inside `namespace geo`).
The reader resolves what the record itself defines (its member typedefs, nested types and
injected class name); the rest stays relative, and `Record.Scope` says where to look it up. The
mapper does that (§6).

### 5.4 How it is asked

`-ast-dump-filter` takes one substring of a qualified name, so the extraction is a set of runs:

- **Types.** The run's file has the package's includes and, for each name, aliases whose names
  contain the type's last name (`gp_d3_Vec`; the others are the canonical type, `sizeof`,
  `alignof` and the enum's underlying type). The dump holds the declarations whose name contains
  `Vec` and the aliases; the alias's type tree names the **id** of the declaration (a typedef of an
  unnamed struct has no name, and `std::vector<long>` is one specialization among several), and
  that id selects the declaration. Names with the same last part share a run; distinct names run in
  parallel (`DeclConfig.Parallel`, 8) up to `MaxRuns` (64) per call.
- **Functions.** One run for all of them: `namespace gp_f0_fopen { using ::fopen; }`, the
  `UsingShadowDecl`s of the dump list every overload with its type.
- **Limits.** The dump of one run is limited to `DeclConfig.MaxJSON` (32 MB) and one declaration to
  8 MB: `std::basic_string<char>` has 40 MB of declarations in libstdc++ (the bodies of every member
  function are in the dump), so it has no members here (`Errors["type std::basic_string<char>"]`),
  and the refusal is remembered. The same happens to `std::chrono::duration<...>` (17 MB for the
  name `duration`) and every standard template whose dump is over the limit: the record is
  not reported (`Errors`), and the mapper makes a named type with `Incomplete` set. The mapper turns it into a named type with no known members. This is the main cost of
  the step: a run for a standard library template takes 0.1 to 1 s (the measured sizes: `vector` 12
  MB, `map` 12 MB, `unique_ptr` 2.5 MB). The disk cache (§9, step 12.7) makes this a cold-build cost
  only: a second call, and a second process, get the part from the cache with no compiler run.
- **Cache.** A run is remembered by the hash of its text, the compiler, the flags and the
  environment (`probe.PreKey`), and valid while the contents of the headers it read (the `-MD`
  file) are unchanged, in memory and on disk (§9). The refusal of a dump over the limit is cached too.
- Errors of a query (a name that is not a type, a name that clang does not know, a limit) are in
  `DeclTable.Errors["type X"]`, never a failure of the call; the call fails only when the context
  is cancelled. A compiler that crashes, hangs (`GOCPP_PROBE_TIMEOUT`) or prints garbage gives
  the error to the queries of its run.

**Names in the query.** The text of a query is not written into the probe file: the parsed type
is printed again, and a name with a control character, a comment (`//`, `/*`, `*/`), `#`, `;`,
`"`, a backslash, a backtick or a brace is refused with an error of its own, so one name cannot
change the lines of the others. A function name is a plain path without template arguments.

**Anonymous members.** The members of anonymous unions and structs are fields of the record, at
any depth (`union { struct { int lo, hi; }; long both; };` gives `lo`, `hi` and `both`), with
`Anon` and `Union` (some union is around the field: `lo` and `hi` of `union { struct { int lo, hi; };
long both; };` are both `Union` although they do not overlap each other; the direct members of a
record that is itself a union have `Union` false, `Record.Tag` says `union`). A `struct { ... } pos;` is a field
`pos` of an unnamed type, and a typedef, alias or static member of an unnamed struct or union
(`typedef struct { int a; } Inner;`, `using X = struct { ... };`) is not an anonymous member at all. When an anonymous member cannot be placed (deeper than 16 levels, or
without the field that follows it), the record is `Partial`: `Complete` is false, no members are
reported, `Size` and `Align` are still the compiler's, and the mapper treats it as a record with
unknown members. Field offsets are not extracted.

Not extracted (known gaps): the location of a declaration (header and line: step 12.13), the
parameter names and default values of free functions (the overload's type only), `explicit` on a
constructor (clang's JSON does not say), friends, using-declarations of members (inherited
members come through `Bases`), variables.

## 6. The foreign view (step 12.4)

`ftype.Mapper` maps C++ types to `go/types` types in a pseudo-package `C`. The table:

| C++ | `Mapped.Kind`, `Type` |
|---|---|
| `int`, `unsigned long`, `double`, `bool`, `wchar_t`, `char16_t` | `Scalar`: named `C.int`, `C.ulong`, `C.double`, `C.bool`, ... over the Go basic type of the same size (`Target`: `LongBits` 64 on LP64 and 32 on Windows, `WcharBits`, `PtrBits`, `CharUnsigned` for aarch64 Linux or `-funsigned-char`; `LP64`, `LLP64`, `ILP32`) |
| `size_t`, `int32_t`, `uint8_t`, `ptrdiff_t`, ... | the same named type as the builtin on the target (as in cgo), with `Alias` `size_t` when the written type said so: `size_t` is `C.ulong` on LP64, `C.ulonglong` on Windows, `C.uint` on 32 bits |
| `void` | `Void` (no type) |
| `T*` | `Pointer`: `*T`; `void*` is `unsafe.Pointer`; `R (*)(A...)` is `FuncPtr`: a `func(A...) R` |
| `T&`, `const T&`, `T&&` | the type of `T` with `Ref` `&` or `&&`; in a `Value` the expression is `Assignable` when it is a non-const lvalue (`v.front() = 5`) |
| `T[N]`, `T[]` | `Array`: `[N]T`; `[0]T` with `Flexible` |
| `R (A...)` | `Func`: a `*types.Signature`; the C++ details (reference parameters, `...`, `const`/`&`/`noexcept` of a method) are in `Mapper.Sig` |
| struct, class, union | `Record`: a named struct whose public fields are the fields and whose public bases are embedded fields (so `types.LookupFieldOrMethod` promotes through them); `MembersOf` has everything (methods with their overloads, statics, nested types, private members with their access, the members of the bases) |
| enum | `Enum`: a named integer type over the underlying type; `Enumerators` are `types.Const`s; `Scoped` for an enum class |
| `std::vector<long>`, `std::map<K, V>` | `Record` named `std.vector[C.long]`; `std::basic_string<char>` is `std.string` |
| `gocpp_probe_internal_pr::string`, `slice<T>`, `array<T, N>`, `map<K, V>` | `Go`: `string`, `[]T`, `[N]T`, `map[K]V` (the stand-ins of the probe file) |
| anything else | `Opaque` with a `Reason`: a pointer to member, a type with no name, `__int128`, a dependent type, a name that could not be resolved |

Rules that the checker (12.5) can rely on:

- The mapping **never guesses**. A relative name is resolved from the innermost scope outwards
  among the names the declarations know (`Mapper.qualify`), with the template arguments qualified
  first; a name that is not known is `Opaque`. A record with no declaration is a named type with
  `Incomplete` set (its members are unknown, which is not an error).
- Each canonical C++ type has one `types.Named`: two mappings of `geo::Vec` are the same object;
  a record may mention itself.
- `Mapper.Needs(results)` and `NeedsMore()` list the declarations to ask for, round after round
  (results, then the types of the public fields, bases and function signatures of the records
  found; three rounds by default); the library's internals (`std::_Vector_base`, `__gnu_cxx`) and
  the names inside `std` are not followed.
- `Value.Compat(goType)` answers whether a value can go where a Go type is wanted by the rules of C++
  (the compatibility rule): `No` only when no conversion exists (a `void` value, a number where a
  pointer is wanted), `Unknown` whenever a conversion operator or constructor could exist. A
  null pointer constant goes to pointers and function pointers: `nullptr`, and an integer
  constant 0 such as the macro `NULL`. C++ only accepts an integer *literal* 0 (`'\0'`, `(1 - 1)` and a
  constexpr zero are refused by both compilers); the probe gives a constant's value, not its form,
  so the rule is permissive here (a missed early error, never a false one). On every program of the header zoo and of the golden programs of
  `tests/interop` (which opaque mode compiles) no site gets `No`.
- `Value.String()` is the stable text form used by the golden files: `value C.ulong (size_t)`,
  `assignable std.vector[C.long]`, `type *_IO_FILE (*FILE)`, `opaque: pointer to member`.

**Members.** `MembersOf` lists the members of a record and of its bases (a derived member hides the
base's of the same name; the access of a member of a protected or private base is lowered).
Overloads from a `using Base::f;` in the derived class and a name found in two bases (ambiguous in
C++) have no marker there, and the Go objects of the named struct (fields, embedded bases) are
only the public data members: the checker (12.5) must use `MembersOf`, not `go/types`
lookups, to decide whether a member exists, is accessible, or is ambiguous.

`typedinterop.Options.Decls` turns all of this on in `Analyze` (`Report.Decls`, `Report.Mapper`,
`Report.Values`); it is off by default, and `Analyze` without it is unchanged (a test compares the
probe file and the table). Nothing in the opaque pipeline uses these packages.

## 7. The interop checker (step 12.5)

`icheck.Check(ctx, pkg, fset, Options)` runs the pipeline of §1 to §6 on a checked package
(`Analyze` with `Decls`) and applies the checker to the report; `icheck.CheckReport` takes a report that
was made already. The result has `Diags` (`diag.Diagnostic`: position in the `.gp` file, message,
related lines, severity), `Silent` (what was left to the C++ compiler, and why), `Skipped` (the whole
package, and why) and `Stats`. It is called by the hidden command `gocpp __typed-check [-e] [-v]
[-build g++|clang++] patterns` (exit status 1 on errors, 3 without clang++); the same checker runs in `gocpp check`, `build`, `run`, `emit`, `test`, `vet` and `expand` when the mode is typed (§13);
the language server does not use it yet.
[ADR-079](../adr/079-typed-interop-checker-reports-only-what-the-compiler-reports.md) has the
reasons.

**The rule: typed mode never rejects what opaque mode and the C++ compiler accept, except where Go's own typing rules reject the program (a void C++ call used as a value: assignment to blank, a switch tag, ...), these being the same errors the checker gives for every other void context** (`testdata/zoo/gorule_void.gp` pins them). An error needs
all of these, and anything less is `Silent`:

| Condition | Where | What is silent |
|---|---|---|
| the probe is an exact mirror of the generated code | `exact.go` | stand-ins of `[]T`, `[N]T`, `map` (the stand-in of `string` only for the plain conversion failures and the number of arguments), composite literals, `Unknown` parts, range and tuple elements, local aliases, a name read two ways |
| the use is where the generated code is the probe's | `siteWhy` | uses inside `defer`/`go` statements, inside generic functions and methods of generic types |
| the compiler printed a diagnostic for the expression | `failureWhy` | probes without one (limits), errors that follow from another probe (reported once, at the cause) |
| the package's headers compile and the target is this machine | `CheckReport` | `HeaderErrors`, `IncludeErrors`, `Options.Cross` |
| the build compiler is clang | `Options.Build` | with g++ every diagnostic is a warning |

**Failed probes** (and probes that compiled with a `Recovered` error: clang recovers from a member of a
pointer, a private member and others) are mapped to Go's words by the rules of `messages.go`; the
templates are `icheck.Templates` and each has a `gocpp explain` entry:

| Template (the message) | The C++ error it maps |
|---|---|
| `undefined: X (the imported headers declare no ::X)` | no member in the global namespace or a namespace, undeclared identifier, no template named (same text and place as the lowering's) |
| `x.m undefined (type T has no field or method m)` (+ "did you mean") | no member named m in T; a member of a pointer to pointer |
| `x.m undefined (m is a private member of type T)` | private and protected members |
| `no overload of f matches the arguments (A, B)` (+ one related line per candidate) / `... matches the call` | no matching function, member function or constructor |
| `call to f is ambiguous: several overloads match equally well` | ambiguous call (candidates with their header positions) |
| `cannot use x (variable of type T) as U value in argument to f` (+ the mistake it looks like) | a parameter that cannot be initialized, no viable conversion, a reference that cannot bind (temporary, const) |
| `too many arguments in call to f (have N, want M)` / `not enough arguments in call to f (...)` | arity |
| `invalid operation: cannot call non-function f (type T)` | called object is not a function |
| `invalid operation: x + y (operator + is not defined on A and B)` / `operator ! not defined on x (type T)` | invalid operands, invalid unary operand |
| `cannot call method m on x (type const T): the method is not marked const` | const-correctness |
| `cannot call T.m without a value: it is a method, not a static function` | non-static member without an object |
| `invalid operation: cannot take address of x (value of type T)` | address of an rvalue or a temporary |
| `invalid use of T: the type is only declared in the headers, not defined` | incomplete types |
| `f(x) uses a function that C++ has deleted (...)` | deleted functions |
| `cannot convert x (value of type A) to type B` | casts without a conversion |
| `cannot use c.m as a value: a C++ member function must be called` | a method value |
| `X is not valid C++: <message>` | every other error, with clang's message |

**Probes that compiled** get the rules of `rules.go` from the declarations: `f() (no value) used as
value` (a void result in a value context: not a statement, a `return` of a function without results,
`defer` or `go`), `cannot use x (value of type T) as U value in <context>` when
`ftype.Value.Compat` says `No` for the Go type at the site (`Site.Target`; an explicit conversion is not a
site of this rule), refined for a class that has no conversion operator (and no operator template and no
base that has one), and `cannot assign to x (neither addressable nor a map index expression)` /
`... : it is const` for a scalar, pointer or enum C++ value on the left of an assignment (a prvalue, an
xvalue or a const lvalue; a class value can be assigned: C++ calls `operator=`).

**Known limits.** Silent where the probe is not the code (above); the opaque check and the lowering
run first and report their own errors (an unknown name, a C value converted to a Go number in a
declaration, argument or result), so the checker adds to what opaque mode finds, never replaces it;
narrowing warnings (README §2.3) belong to `gocpp vet` (it reports the explicit conversions `C.int(i64)` today; the implicit ones that this mode accepts wait for `gocpp vet` to run the probes: Phase 13); the "name through the wrong header" warning
needs header locations (step 12.13); an incomplete type used as a value is not checked.

## 8. Code generation (step 12.6)

[ADR-087](../adr/087-typed-interop-code-generation.md) has the reasons. The lowering reads the report of
`typedinterop.Analyze` (with `Decls` and `StringToCStr`) through `lower.Interop.SetTyped`; the driver sets
it for each package that is not the standard library's when the mode is typed (§13; the report is the one
that `check` made for the interop checker). In opaque mode the output is byte-identical to Phase 04's. Where the probe has no answer for an expression (not probed, failed, or **recovered**: clang
printed an error and kept a type; an opaque or incomplete foreign type; a copy of the syntax tree that the
collector did not see) the opaque lowering runs. A cross build is typed only when every header is the C++
library's, except the build for windows/amd64, whose probes read the MinGW-w64 headers (`typedinterop.ProbesTarget`).

| Source | Opaque mode | Typed mode |
|---|---|---|
| `p.m`, `p` a raw `T*` | `gp::deref(p).m` | `gp::nonnull(p)->m` (Go's nil check kept) |
| `x.m`, `x` a class, a reference | `gp::deref(x).m` | `x.m` |
| `sp.m`, `m` the pointee's of a smart pointer, optional or iterator, and the class has no `m` | a C++ error | `gp::smartnn(sp)->m` (a nil check for `unique_ptr`, `shared_ptr` and optional) |
| `*p` | `gp::cgo::star(p)` | `*gp::nonnull(p)`, `*x` for a class |
| `lib.Rect{h: 3, x: 1}` | a lambda of assignments in source order | `::Rect{.x = 1, .h = 3}` in declaration order (calls of the values in source order, hoisted into locals when the order changes) |
| `lib.Rect{x: n}`, `n` a Go `int` | a narrowing error from the C++ compiler | `.x = gp::cgo::as<decltype(::Rect::x)>(n)` |
| `lib.puts(s)`, `s` a Go string | the user writes `C.cstr(s)` | `::puts(gp::cstr(s).gp_ptr())` |
| `lib.take(s)`, a `std::string` parameter | `std.string(s)` | `::take(::std::string(s))` |
| `m[key]`, `key` a Go string | a C++ error | `m[::std::string(key)]` |
| `lib.title(s)`, a `const wchar_t*` parameter | a C++ error | `::title(gp::wide_cstr<wchar_t>(s).gp_ptr())` (ADR-130) |
| `lib.bump(&n)`, `n` a Go `int`, an `int*` parameter | a C++ error | `::bump(gp::cgo::call_out<int>(&n, "n", [&](int* gp_out0) -> decltype(auto) { return ::bump(gp_out0); }))` (ADR-130) |
| `lib.get(&p)`, `p` a `*lib.T`, a `void**` parameter | a C++ error | `gp::cgo::call_void_out(p, [&](void** gp_out0) -> decltype(auto) { return ::get(gp_out0); })` (ADR-130) |
| `lib.mode(2)`, an enum parameter | a C++ error | `::mode(static_cast<::Mode>(2))` (ADR-130) |
| `lib.take(f)`, `f` a Go float, an `int` parameter | C++'s conversion (undefined when out of range) | `::take(gp::cgo::as<int>(f))` (ADR-130) |
| `hr := sc.ResizeBuffers(...)`, a result of the typedef `HRESULT` | the number | `auto hr = gp::hresult_error(sc->ResizeBuffers(...))`: `nil` when the code is not negative (ADR-137) |
| `v.at(0) = 5`, `v.back()++`, `&v.at(0)` | an error of the lowering | accepted: a `T&` result is an lvalue |
| `var n int = cstring.strlen(s)`, `cstring.strlen(s) + len(s)`, `var c C.int = n` | "mismatched types", "cannot use ... convert it" | `gp::cgo::as<int64_t>(::strlen(...))`, C++'s arithmetic, `gp::cgo::as<int>(n)` |

**One table of argument adaptations (ADR-130).** The rows below that are not about members (the strings, the wide
strings, `&x` of a number, `&p` for `void**`, constants for enums and numbers, floats for integers) are the rows of
one table in `internal/lower/typedadapt.go`: an argument class of the Go side, a verdict of each row about the
parameter that clang reports (adapts, can never take it, or unknown), and the lowering of the row. A row is used
only when every overload the call can reach has the same row (with the same detail) there; the rest is C++'s. The
spec of the rows, what is not adapted and the lifetime rule is in [interop.md](interop.md#argument-adaptations-typed-interop);
the printf-like functions come from a second compiler run over a generated call with a deliberately incomplete
format (`probe.Overload.FormatKind`, `FormatParam`), whose warning says whether the parameter is a printf or a
scanf format.

**One result table (ADR-137).** The result of a call has its own table, `adapt.ResultTable`, with one row: the
typedef named `HRESULT` (the type the declaration wrote, `probe.Result.Written`, over a signed `int` or `long`) is Go's
`error`. It is applied **in the check**, not in the lowering alone: `check.Config.HResults` is a hook that analyses the
package after its first check (before any rewrite) and returns the calls with that row, and the check wraps each in
`error(call)` in the syntax tree and checks again, so go/types gives the call, and a variable declared from it, the type
`error` and every use is Go's own (`typedinterop.HResultsHook`, `check/hresults.go`). A call the probe cannot type (an
argument the argument table adapts) is typed from the declarations of the callee. A call that is a statement, deferred,
spawned, `_ = f()`, or converted to a number is the C++ call as it is (`collect.KeepsHResult`). The lowering writes
`gp::hresult_error(call)` for the conversion. There is no second probe round. The spec is
[interop.md](interop.md#results-that-are-errors-typed-interop).

**Which parameter takes a string.** For a call of a named function (or a method of a class), the overload sets
of the declarations (`Result.Decls.Funcs`, `MembersOf`) are searched for the parameter at each place that
has a Go string: a place where every overload with a parameter there has `const char *` (or the call is variadic
there) gets `gp::cstr(s).gp_ptr()`; where every one has `std::string` (value, `const&`, `&&`) it gets
`::std::string(s)`; parameters that no string can reach (a number, a `char *`, a `std::wstring`, a pointer to
anything else) are no candidates; any other parameter (a class that might have a constructor, a
`std::string_view`) or a mixed set leaves the argument to C++. The calls that keep the pointer or give back
one into the string (`internal/lower/keeps.go`) are a user error with the hint to use `C.CString`:
the functions of a table (`openlog`, `getopt`, `getopt_long`, raylib `InitWindow` and `SetWindowTitle`), the
SQLite functions that are given bytes (`sqlite3_bind_text`, `_blob`, `sqlite3_result_text`, `_blob`, ... ) unless
the destructor argument is `SQLITE_TRANSIENT`, `curl_easy_setopt(CURLOPT_POSTFIELDS)`, any call that
also gets the address of a char pointer (`strtol(s, &end, 10)`, `sqlite3_prepare_v2(db, sql, -1, &st, &tail)`:
the pointer it stores can point into the string), and names with static, borrow, nocopy, noalloc or
unowned in them. A function that returns a pointer into its argument (`strstr`, `strchr`, `basename`,
`memmem`, `strptime`, ...) is an error when the result is stored, not when it is compared or passed on in the
same statement. **Not covered:** a C++ method or a function of the program that stores its `const char*`
parameter: nothing in its declaration says so, so the string is converted and the pointer dangles; use
`C.CString` for those. Functions that take a `char*` (`putenv`, `strtok`, `setvbuf`, `setbuf`) need no
rule: a Go string never converts to `char*`, and the call is a C++ error as in opaque mode. `gp::cstr` copies
a string that is not NUL-terminated (run-time string; a literal and its substrings are not copied) and a NUL inside the string ends the C string.

**Designated initializers.** The record must be complete, an aggregate (`Traits["aggregate"]`) and every key a
public field of the record itself (the fields of anonymous unions and structs are the record's, in
declaration order; the fields of a base class are not). At most one member of a union, at most one key
that is a member of some union. A Go array literal for an array field is its elements; a nested literal
of a C/C++ type is a designated initializer of its own.

**Tests.** `tests/typed/*.gp` (run by `TestTypedRun` with both compilers; a header comment says whether opaque mode
rejects or equals them) with a C++ twin each (`tests/typed/twin`, compiled and compared by `TestTypedTwin`);
`TestTypedEmit` pins the text; `TestTypedWindows` runs the std-only program on the real PC
(`GOCPP_WINDOWS_SSH`). The unit tests of the decisions are in `internal/lower/typed_test.go` and
`internal/typedinterop/ftype/arrow_test.go`.

## 9. The cache (step 12.7, ADR-088)

Every answer of the compiler is kept: the kinds (Phase 06's cache, unchanged), the results of the
probes, the declaration parts and their refusals.

- **Units.** `probe.Generate` returns `Source.Preamble` and `Source.Units`: the probe lines of each reference
  with the aliases of local variables they use (all of them, each with the ones it uses), and a hash of their
  text without the reference's number. A result is stored under (key of the preamble, hash of the headers'
  contents, unit hash). `Runner.Probe` looks up every unit, sends the missing ones to the compiler in batches
  (a probe file with the same preamble and only those units; a batch has at least `GOCPP_PROBE_BATCH`, default 400,
  units and there are at most `Jobs()` of them), and puts every result through the cache's encoding before it
  reaches the table. A result does not depend on the batch it came from: the places in the probe file that
  its messages name (`ErrorAt`, `Notes`, `RecoveredAt`, `RecoveredNotes`) are the full file's lines of the unit, of an
  alias it uses, or of the preamble; a place in another unit has line 0. An error of an alias line is dropped
  (the lines that use the alias fail on their own).
- **Headers.** The context entry of a preamble has up to four *variants*: the list of files the compiler read
  (`-MD`) with the hash of their contents. An entry whose files still have those contents is used; a variant
  older than a week (or from the future) is not (`maxVariantAge`). The list is completed (`finalDeps`) with
  - the places where an include of the package would be hidden (the package folder, `-iquote`, `-I`);
  - for every header that was read and found in one of the search folders, the same name in the folders before
    it and in every folder that the flags and the environment name (`-iquote`, `-I`, `-isystem`, `-idirafter`,
    `CPATH`, `CPLUS_INCLUDE_PATH`; the order inside the flags is not trusted): a header that appears there hides
    the one that was found, also in a header that a header includes. The folders of the flags and the
    environment are listed whether they exist or not (`clang -E -v` leaves out a folder that does not exist, and
    one made later must be seen); the compiler's own folders (from the toolchain) come after them;
  - the package folder is also the first folder of the search order (the probe adds it as `-iquote`): a header
    created there hides one that was found in a `-I` folder, in a header that a header includes and in an
    `-include` flag as well as in the package's own includes;
  - for every include directive the preprocessor met, the folder of the file that has it joined with the name as it
    was spelled (`../foo.h` keeps its `..`): a quoted include is looked up in the folder of the file that has it
    first, before any search folder; the folders of the flags and the environment are watched for the same spelled
    name. The directives are not read from the text of the headers (no regular expression can follow a continued
    line, a comment, a digraph, a macro that makes the name) but from `clang -E -dI`, whose output keeps every
    `#include`/`#import`/`#include_next` with the name after the macros, in the line markers' file (also the
    includes that a guard or `#pragma once` skipped, and those that were not found). The run is made once per
    preamble and kept in the Runner for as long as the headers it read have the same contents (each batch of
    units and each declaration run asks), and not at all for a text with no header. Includes made by the compiler's
    own headers are left out (nobody adds a header there). The candidate paths are built as text (folder, separator,
    name), never cleaned: a `..` after a symbolic link is resolved by the system on the link's target (`dir/link/..`
    is the parent of what `link` points to), so the paths are stat-ed and read as they were spelled, and `ParseDeps`
    keeps a path with a `..` as the compiler wrote it. The question is asked once per generated preamble (the title
    line aside), not per declaration run: a declaration run passes its preamble, without its query. When the output is not complete (the compiler was
    stopped at the time limit, the output ends in the middle of a line, a directive line is over 1 MiB) the cache
    is conservative: the folder of every header that was read that is not the compiler's own, and the package
    folder, are watched for every name by which such a header was found;
  - after a "file not found" (the compiler writes no dependency file then), every place where the missing
    header would be: the package's own includes in every folder, and the names that headers include
    (as spelled, from the same `-dI` run; `-M -MG` when that is not complete) in the folders of the search, the package folder and the folder of every header
    that was read. A negative answer (a header that is not there, also inside another header) is an entry like
    another and is lost when the header appears in any of those places.
  The toolchain (the search path, the target, the GCC installation clang chose) is part of the key of the
  results, so another libstdc++ is another cache; it is asked from the compiler (`-E -v`, one cheap run) at most
  once an hour and kept in the cache (`toolchainTTL`). A Runner with no cache folder does not ask.
  **What is not seen:** a header that `__has_include` asks for and that appears later; a change of the toolchain
  within the hour; and, with no cache folder, a header that hides another one a header includes (the search path
  is only asked for after an error). These are bounded by the week. A header that was modified (modification or
  change time) less than two seconds before the compiler started, or at any time after, is not kept anywhere, not in
  memory either (`racyDeps`): the compiler may have read the old contents.
- **Declarations.** `Runner.Decls` caches each run's part (records, enums, canonical names, overload sets,
  errors, header errors) under the key of the run's text, with the same variants. Types are written as text and
  parsed back; the table of a warm call is the table of a cold one. A run killed at the dump limit
  is cached as a refusal; its headers come from a preprocessor-only run.
- **Folder and files.** `<user cache>/gocpp/probe/typed-v1/{c,u,d}/xx/<sha256>.json`
  (`GOCPP_PROBE_CACHE=<abs>` is the folder above `typed-v1`, `off` or `GOCPP_NOPROBECACHE=1` disables;
  test binaries use none unless it is set). A file has a checksum line; writes are atomic (rename); bad files are
  misses. At most 100,000 entries and 256 MB, 90 days unused (`Prune`: at the first write of a process and every
  512 writes, and not within an hour of the last pruning, the time of the file `.pruned`); a read refreshes the time. Read-only folders fall back to memory. `gocpp clean -cache` removes it.
- **The precompiled header of the library block (step 12.14, ADR-117).** `<probe cache>/pch/ab/<hash>.pch`, with
  `<hash>.hpp` (the block, which clang names as an input file of the header) and `<hash>.json` (the files it read, and the
  hash of their contents). The key is the compiler, the flags and the environment of the include path, the toolchain and the
  text of the block, not the package folder: it is **per compiler and flag set**, so the packages that compile with the same
  flags share it and a package whose flags differ (the `-I` folders of `pkg-config`) has one of its own; the 4 newest are
  kept, so a machine that checks more than 4 flag sets keeps making them again. It may be used while the hash of the
  contents of the files it was made from, completed by `finalDeps` as for a probe result (a library header that is edited,
  or one that appears earlier on the search path), is what it was; otherwise it is made again. Built with
  `-fvalidate-ast-input-files-content`, one build per key per process and a lock per key between processes, written under a
  temporary name and renamed (the source `.hpp` too, and not again if the text is the same); after taking the lock a process
  looks for the files again. The 4 newest are kept (`prunePCH`; the age, count and size limits of the other entries leave
  them alone), and what a killed build left (`.tmp-*`, a `.pch` or `.hpp` with no `.json`) goes when it is 10 minutes old.
  The `.json` has the size of the `.pch`: a file of another size is not used. The dependency list of a run that loaded it is the one the same run has without it, so the keys of results do
  not change. A run is never held back by a build that has just started (it runs without the header; a build older than
  200 ms is waited for, at most 400 ms); the kind probe's first miss starts the build beside it (a compiler outside the `Jobs()`
  gate, like the build, and the toolchain question of that hook, which is asked in the hook). The build ends with the command
  (`SetBuildContext`: an interrupt kills its compiler), and a call that started builds waits for them (`WaitPCH`) at most 6 s,
  then stops them. A run that loaded the header, made no declaration and failed is made again without it: if the second run
  fails in the same way the failure is the program's, otherwise the header is deleted (the compiler said "malformed",
  "corrupt" or the like, or the second run succeeded), and a file refused twice is not used again by the process
  (`PCHRechecks` counts the runs made again). No cache folder, a compiler that is not clang, `-march=native` in the flags, a
  relative folder on the include path (`-I include`, `CPATH=inc`: the build runs in another folder than the package's) or
  `GOCPP_PROBE_PCH=off`: no header. Counters: `PCHBuilds`, `PCHUses`, `PCHRejects`, `PCHRechecks`.
- **The includes question beside the compile.** The `-E -dI` run of a preamble (above) is started with the compile run that
  will need its answer (`prefetchInfo`); runs of the same preamble that start within a second share one question, an earlier
  answer that is still valid skips it, and a header that changes meanwhile is covered by `racyDeps`. The await honors
  caller cancellation and releases that caller's reference. Only the last reference cancels and joins the auxiliary
  compiler, so a live waiter keeps its shared run. Published scanner output is immutable; each live waiter computes
  completeness on its own copy. A canceled await returns incomplete information, which is not memoized; a canceled caller does not persist probe results, declarations or PCH metadata after the await.
- **Processes.** A lock file per key (`locks/`, with the machine's name and the process id): a process that finds the lock waits (at most the run's limit
  plus 15 s; the context ends it) and looks again; the lock of a process of this machine that is gone is taken over at once. Counters: `probe.CacheCounters()`.
- **Parallelism.** `Jobs()` (`GOCPP_PROBE_JOBS`, else `min(8, NumCPU)` in `gocpp lsp` and `min(8, NumCPU/2)` in a
  build, ADR-126) compilers at the same time in the
  process, in all calls (a global semaphore taken by every run, also the auxiliary ones). Cancelling the
  context kills each compiler's process group.

## 10. Constants: gap E1 (step 12.8, ADR-097)

[ADR-097](../adr/097-typed-interop-puts-the-values-of-c-constants-into-the-program-before-go-types.md) has the
reasons. Stock `go/types` cannot evaluate a C/C++ name (it is opaque), so `[cstdio.BUFSIZ]byte`, `const n =
cmath.M_PI * 2` and `case cstdio.EOF:` fail in opaque mode (the phase gate "C macros in constant expressions
are not supported yet (Phase 12)" and the lowering's "write the number"). In typed mode the names are probed
**before** `go/types` and replaced in the syntax tree by constants of the same position:

```
var buf [cstdio.BUFSIZ]byte        ->  var buf [8192]byte
const tau = cmath.M_PI * 2         ->  const tau = 0x1.921fb54442d18p+01 * 2
case cstdio.EOF:                   ->  case -1:
n := 1 << lib.SHIFT                ->  n := 1 << 5
```

**The hook.** `check.Config.Constants` (a `check.ConstFunc`; nil is the default and changes nothing) is called by
`Check` once per package, before the first `go/types` run, with a `ConstQuery`: the package folder, its probe flags,
its `#include` texts (with the extern "C" mark), the target, and the `CppPath`s (header package or namespace, and
the names after it) of every name in a place below. `typedinterop.Constants(Options)` answers it; the driver sets it
when the mode is typed (§13: `typedinterop.Hooks`, called in `check.go`), for every command that checks
(`build`, `run`, `test`, `check`, `emit`, `vet`, `expand`); the language server does not set it yet (12.13 takes
its hooks from `typedinterop.Hooks` too).

**The places** (`constFinder`), the only ones where a name is replaced:

| Place | Name that is not a constant |
|---|---|
| an array length: `[N]T`, anywhere an array type is written | an error (Go needs a constant) |
| the value of a `const` declaration | an error |
| a `case` of an expression switch (not a type switch) | left as it was |
| the count of a shift: `x << N`, `x >>= N` | left as it was |

Inside a place the finder looks through parentheses, the unary `+ - ^ !`, binary operators, and the arguments of
a conversion to a basic Go type (`uint8(lib.N)`) and of `len`, `cap`, `min`, `max`, `real`, `imag`, `complex`. It does
not look into `unsafe.Sizeof` and friends, nor into the arguments of a call of a C/C++ function, of a Go function
or of a `C.type(...)` conversion. A name is a selector chain that starts at a header package or a namespace
(`cstdio.BUFSIZ`, `consts.geo.kSides`, `std.numbers.pi`, `macros.Dim.kWidth`, `k.kPt.x`); `C.NAME` of `import "C"` is not.

**The answers** (`ConstAnswer`, from `Result.Value`/`ValueError` of the 12.2 probe with `WantValue`):

| C++ | Go constant | Notes |
|---|---|---|
| integer, enumerator of an unscoped enum, `__int128` | untyped integer, exact | `-7` is a unary minus on `7`; `(1UL << 31)` is `2147483648` (C++ evaluated it) |
| `char`, `wchar_t`, `char16_t`, `char32_t` | untyped rune (`'a'`) | a negative or surrogate code is an integer literal |
| `bool` | `true`, `false` | |
| `float`, `double`, `long double` that is exact | untyped float, **the exact double** | decimal if the exact expansion is up to 20 characters (`0.5`, `3.0`), else hexadecimal (`0x1.999999999999ap-04`) |
| string literal macro (adjacent literals, escapes) | Go string of the exact bytes | refused when a byte is not UTF-8 (clang's dump would give U+FFFD) |
| `constexpr`/`const` variable, static member, member of a constexpr object, namespace member | as above | `constexpr const char*` and arrays are not strings (no constant) |
| NaN, infinity | no constant | "its value NaN is not a number that a Go constant can have" |
| enumerator of an `enum class` | not substituted | not an integer in C++ |
| a macro with a side effect or a call, a mutable variable, a pointer, a class object | no constant | "the C++ compiler cannot evaluate it at compile time (...)" / "its type T is not a number, a character, a boolean or a string literal, the only things that Go constants can be" |
| a function-like macro without its arguments | no constant | "it is a macro that does not expand to an expression (it is empty, function-like or a statement): ..." |
| a type | no constant | "it names a type, not a value" |
| a macro whose value is the place or the time of its expansion: `__LINE__`, `__FILE__`, `__FILE_NAME__`, `__COUNTER__`, `__DATE__`, `__TIME__`, `__TIMESTAMP__`, `__BASE_FILE__`, `__INCLUDE_LEVEL__` in its body or in the body of a macro it uses | no constant | "its value depends on where or when it is used (the macro expands to __LINE__), so the probe's value would not be the program's" |

A name that the compiler has no answer for (undefined, headers that do not include, no clang, another target
than this machine's, a probe that failed) is left as it was: the checks that exist report it, and the opaque gate
and lowering message appear if it sits in an array length or a const declaration. Not an error of its own, except a
warning when the probe itself failed ("cannot evaluate the C/C++ constants of this package: ... (they stay opaque)").

**Errors** are at the `.gp` position of the name, in Go's words, with the name the user wrote: `array length
cstdio.NEXT must be constant: <reason>`, `lib.TWICE is not constant: <reason>`, and for a substituted value
go/types' own messages with the literal replaced by the name (`invalid array length lib.NEG (untyped int constant
-7)`, `cannot use lib.NEG (untyped int constant -7) as uint8 value in constant declaration (overflows)`).
`gocpp explain` has entries (`interop/const-array-len`, `interop/const-decl`, `interop/const-warning`).

**Compatibility with opaque mode.** A substitution in a case or a shift count that makes `go/types` report an error
(except a duplicate case) is taken back and the package is checked again, so the program that opaque mode and the C++
compiler accept is accepted (`case cstdio.EOF:` in a switch on a `byte`; `switch { case lib.ENABLED: }` with an
integer macro; `1 << lib.NEG`; `var m int32 = 1 << lib.S31`: the place of a shift is the whole `x << N`).
`Package.Consts` lists the substitutions that stand, with the probe's answer and spelling.

**The guard** (ADR-097 §7). The first translation unit of the package asserts, after the headers, each name that
was replaced, once however often it is used, behind a `#line` of its first use:

```cpp
static_assert(static_cast<long long>(BUFSIZ) == 8192LL, "gocpp: cstdio.BUFSIZ differs between the probe compiler (clang++) and this compiler (main.gp:10:14)");
static_assert(static_cast<double>(M_PI) == 0x1.921fb54442d18p+01, "gocpp: cmath.M_PI differs ...");
static_assert(std::string_view(NAME, sizeof(NAME) - 1) == std::string_view("ab", 2), "gocpp: g.NAME differs ...");
```

The name is spelled as the probe spelled it (`EOF`, `::kMax`, `::geo::deep::kN`, `(::kPt).x`). Integers, characters,
bools and enumerators are compared as `long long` (`unsigned long long` above 2^63; a value wider than 64 bits gets a
comment instead), floats as `double` against the exact literal, strings by length and bytes (octal escapes). A
build with g++ of a header that branches on `__clang__` (`#ifdef __clang__ #define VV 1 #else #define VV 2`) fails
with `./main.gp:8: C++: static assertion failed: gocpp: h.VV differs between the probe compiler (clang++) and this
compiler (main.gp:8:11)` at the line of the use; nothing is written in opaque mode. Macros that name the
compiler (`_PSTL_CLANG_VERSION`, `_GLIBCXX_CLANG`, `__GNUC__`: 4 under clang, 15 under g++) differ already, so a
program that uses one as a constant does not build with the other compiler.

**Semantics to know.** A substituted constant is a Go untyped constant: Go's arithmetic on it is exact and
does not wrap (`lib.UINT_MAX + 1` is 4294967296), and `x := 1 << lib.N` is a Go `int`. The value of a macro is
what C++ evaluated, with C++'s arithmetic inside the macro. Values are the host's (the probe compiler is clang);
for a cross build nothing is substituted. The standard library's packages (`Package.Load.Std`) are not asked.
`gocpp expand` prints the user's declarations byte for byte (ADR-091), so it shows the names; `gocpp emit` shows
the numbers. The probe's helper templates and variables are named `gocpp_probe_internal_*`, so a header with a
macro `N`, `T`, `V`, `K`, `C`, `U`, `t`, `v` or `a` does not stop the probe (the build's own protection of the
runtime's names, macros.go, still applies to the generated code).

**Cache and cost.** The query is one `probe.Runner.Probe` of bare paths with `WantValue`, so the answers are the
12.7 unit results (`typed-v1/u`, per name, under the contents of the headers) and run in the 12.7 batches; the
declarations of the type of an enumerator (is its enum scoped) are asked from `Runner.Decls` only when a name is of
a class or enum type. A package with no name in a place asks nothing. Warm: no compiler run.

**Tests.** `internal/check/constsubst_test.go` (fake `ConstFunc`: values, kinds, places, errors, revert, messages,
float literals), `internal/typedinterop/consts_test.go` + `testdata/consts` (the header zoo against clang: every
shape in the table above, redefinition and `#undef`, platform and flag macros, a header that redefines another's
macro, the cache), `cmd/gocpp/typedconsts_test.go` (the driver: typed, opaque, no clang), and `tests/typed`
(`macro_consts.gp` with its C++ twin, `err_const_*.gp`).

## 11. Promoted members of embedded C++ classes (step 12.9, ADR-098, gap E2)

```go
type Stacked struct {
	std.vector[int]
	embed.Stack
	n int
}
var s Stacked
s.push_back(1)  // the vector's: s.vector.push_back(1)
s.push(2)       // the stack's
s.size()        // ambiguous if both have a member size: Go's rule
```

go/types has no type for an embedded C++ class (a C/C++ type is invalid in it, `FakeImportC`), so the
first pass of `internal/check` can decide a selector `x.name` on such a struct only when the answer does not
depend on the class's members (`promoted.go`: one embedded class, no Go member at the same depth). The others
are the **candidates of the second pass** (`internal/check/cppmembers.go`): a selector that go/types found
nothing for, on a struct that embeds a class directly or through embedded structs (including embedded pointers,
aliases and named types over a class), and a selector that go/types resolved to a Go member when a class is at
the same or a shallower depth (the member of the class would win or make it ambiguous).

**The hook.** `check.Config.CppMembers` (unset: opaque mode, the first pass stands) is called at most once
for a package, and only when it has candidates, with one `MemberQuery{Class, Name}` per embedded class and
name. `typedinterop.MembersHook`/`AnswerMembers` answers with the C++ compiler: for each query one probe
`decltype((std::declval<C&>().name))` (the lookup is the compiler's own: bases, using-declarations, anonymous
unions, the library's internal base classes that the declaration extraction does not follow), classified by
what clang says about the access (`ClassifyMember`):

| the compiler says | `MemberStatus` |
|---|---|
| it compiles (data member, static member, enumerator) | `MemberFound` |
| "reference to non-static member function must be called" (a member function, any overloads, const and non-const, or a template) | `MemberFound` |
| "no member named 'n' in 'C'" | `MemberMissing` |
| "'n' is a private member of 'B'" / "protected" (also through a private or protected base) | `MemberPrivate` / `MemberProtected` |
| "member 'n' found in multiple base classes" (different bases) or "non-static member 'n' found in multiple base-class subobjects of type 'B'" (a non-virtual diamond; a virtual one is fine) | `MemberAmbiguous` |
| "attempt to use a deleted function" | `MemberDeleted` |
| "cannot refer to type member" (a nested type, a typedef) | `MemberNotValue` |
| a note "expanded from macro 'n'" (the name is a macro of the headers) | `MemberMacro` |
| anything else, a probe that failed, a header that does not compile, no clang | `MemberUnknown` |

A class that has an `operator->` and no member of the name is `MemberUnknown` too (the pointee may have it:
typed code generation writes `x->name`, §8); so every class is probed for `operator->` once. The probes are
ordinary references of the probe file (`collect.MemberSet`), so the disk cache of §9 holds the answers: a warm
build asks the compiler nothing, and a cold one runs it once for all classes and names of the package.

**What the answers decide** (`decideCpp`). With the depth d of a member = the number of embedded fields to its
owner (a field of the struct itself 0, a member of the class that the struct embeds 1, a member of a class
embedded in an embedded struct 2, ...), Go's rule applies to the Go members and the members of the classes
together; a member that the class has but that its users cannot use (private, protected, deleted, ambiguous
in its bases) counts as a member for the rule:

| members at the smallest depth | result |
|---|---|
| none | error `s.foo undefined (type S has no field or method foo; std.vector[int] has no member foo)` (several classes: `none of the embedded C++ types A, B has a member foo`; a macro or a type adds a remark) |
| one Go member | go/types' selection stands; the gate for a Go member beside an embedded class (`featPromotedCppShadow`) is not reported, the class was asked |
| one member of a class | promoted through the embedded fields: `Package.PromotedPath`/`PromotedField` (what the opaque pass, the gates and the lowering read) |
| several | error `ambiguous selector s.size (std.vector[int].size and std.string.size)` (a Go member is "a Go member") |
| one of a class, and a Go member deeper | error: Go's rule picks the class's, but go/types' selection (and the types of the expressions around it) is the Go member's: `s.size is the member of std.vector[int], which is shallower than the Go member size (Go's rule): name the field, s.vector.size` |
| one of a class that is private, protected, deleted or ambiguous in its bases | error `s.priv undefined (the member priv of C is private)`, `... is protected`, `s.del is a deleted function of C`, `ambiguous selector s.x (C has a member x in several bases)` |

The errors are at the selector in the `.gp` file. The go/types error of an unresolved selector is replaced by
ours or dropped (a member found); the phase gates (§ gates.go: several embedded classes, a Go member beside a
class) are not reported for a selector the second pass decided, and stay for the others. Anything the hook
cannot answer (no clang, an error of the probe, a class from another package, a class that is a type
parameter) leaves the selector as the first pass had it: one class is promoted and the C++ compiler checks
the member; several is the gate. The compatibility rule holds: an answer is an error only where C++ itself
has no usable member by that name.

**Not promoted.** Operators: `s[0]`, `s + t`, `len(s)` and `range s` stay what go/types says (write
`s.vector[0]`); constructors and destructors; nested types and member typedefs (not values; `MemberNotValue`
is an error when nothing else has the name). A member template is a member (`s.twice(21)`, one instantiation
per call, the arguments deduced); explicit template arguments of a member (`s.conv[int](x)`) are a limit of
the lowering, with or without embedding (opaque mode has it too). The interfaces that a struct would
satisfy with a promoted C++ method are not seen by go/types (`promoted.go`).

**The second pass does not run go/types again.** A class synthesized as go/types objects (methods, fields) would
make method sets, assignability and the types of the expressions around a call depend on a C++ declaration that
stock go/types never reads, and every C/C++ expression would have to be typed in it. The pass re-decides what
the first pass could not, where the first pass looked (`PromotedPath`, the false errors that stay, the opaque
pass, the gates). Cost: none without a candidate; with candidates, one hook call per package and, cold, one
compiler run (about 0.35 s for a small std example; warm: none; `Package.CppMembersAsked` counts the questions).
The opaque pass is run a second time only for a package that has candidates.

**Typed code generation.** A promoted selector is collected as a member of an lvalue of the embedded field's C++
type (`collect.promotedReceiver`: `Select{Arg{T: class, LValue}, name}`; a temporary for the member of a call's
result), so `s.size()` is probed and lowered like `v.size()` on a variable: the class of the field is the
reference of `sel.Sel` (`Set.RefOf`), `lower.memberObject` hands it to `typedMember`, which writes `.` for
a class and `->` for a raw pointer or a smart pointer's pointee: `s.vector.push_back(1)`, `o.Inner->vector.size()`,
`s.vector.at(0) = 5` (a `T&` result stays assignable), `&s.at(0)`. A deferred call
of a promoted member is lowered inside the defer thunk through the thunk's copy of the object, as
`gp_df.gp_push_call([](auto&& gp_a0) { gp::deref((*gp_a0).Box).get(); }, s)`: `gp::deref` there is the thunk's own
(it gives the object whether the frame stored a pointer or a copy), and only the member of the class is typed. The interop checker (12.5) checks the uses
of promoted members like any other (`no overload of s.push_back matches the arguments ...`).

**Limits.** A generic struct that embeds a class of its type parameter (`type Holder[T any] struct{ std.vector[T] }`)
fails before this pass with "interface types as C++ template arguments are not available yet" (a limit of the
lowering of generic types over C++ templates, with or without promotion); a class that is a type parameter is
undecided.

**The switch.** The hook is set when the mode is typed (§13: `typedinterop.Hooks`) and by the hidden
commands `__typed-check` and `__typed-probe`; it needs clang++ and, for a build for another system, only
the headers of the C++ standard library. The language server does not set it (12.13).

## 12. `gocpp bindgen` (step 12.10)

[ADR-092](../adr/092-bindgen-writes-a-go-syntax-view-from-clang.md) has the reasons. `gocpp bindgen [flags] header` writes
a **view** of a C or C++ header: a `.gp` file in Go syntax that lists what the header offers, for reading, for the
documentation and for the editor. It is not a program (functions have no bodies) and not an importable package (the names
are the header's own, which Go does not export): it is formatted by `gocpp fmt`, accepted by `gocpp check`, and nothing
else reads it.

```
gocpp bindgen [-o file] [-pkg name] [-I dir] [-D macro[=value]] [-std c++20] [-pkg-config lib]
              [-include regexp] [-exclude regexp] [-scope auto|header|dir|tree] [-v] header
```

`header` is `<name.h>` for a header that the compiler finds, or the path of a file; flags may come before and after it, and
`-Idir`, `-DNAME=1` and `-std=c++23` are spelled as the C compiler spells them. The output goes to standard output, or to
`-o file` (written only when the whole view was made). Exit status 0, 1 (the header is not found, does not compile, or a flag is wrong), 2 (usage), 3
(no clang++: `gocpp bindgen: typed interop needs clang++: ...`).

```go
// Code generated by gocpp bindgen from <raylib.h>. DO NOT EDIT.

package raylib

const (
	RAYLIB_VERSION_MAJOR                = 5
	PI                                  = 3.1415927
	MOUSE_LEFT_BUTTON   MouseButton     = 0
)

type Color struct {
	r, g, b, a C.uchar
}

type Texture2D = Texture

type ConfigFlags C.uint

const (
	FLAG_VSYNC_HINT      ConfigFlags = 64
	FLAG_FULLSCREEN_MODE ConfigFlags = 2
)

var (
	RAYWHITE = Color{245, 245, 245, 255}
)

func InitWindow(width C.int, height C.int, title *C.char)
func WindowShouldClose() C.bool
```

### 12.1 How it is made

| Run | What it gives |
|---|---|
| `clang++ -E -dD` of `#include <header>` | the `#define`s with the file of each (`#undef` removes one), and the line markers: which files the header includes, in which order |
| the JSON AST dump (`-fsyntax-only -Xclang -ast-dump=json`) | every declaration of the translation unit, read by `scanTU` as top-level declarations (the dump of `SDL.h` is 446 MB: 0.6 s to scan), decoded only when the declaration is in a file of the view or is a type |
| the probe (step 12.2) for the macros and the const variables | the type and the value of each (`Result.Value`, floats exact, strings from the literal) |
| `auto x = (NAME);` for each macro whose type is a struct | the initializer, read as a Go composite literal |

### 12.2 What is in the view

`-scope` (default `auto`): the header; the header that a wrapper stands for (`stdlib.h` in libstdc++ includes
`cstdlib`, which includes the C library's `stdlib.h`); the parts of a header in folders `bits`, `asm`, `asm-generic`,
`linux`; with `auto` and `dir` also the headers of the same folder that an in-scope header includes, except in `auto` when the
header lies directly in a folder of the compiler's include path (`/usr/include/zlib.h`). `header` is the file alone, `tree` takes
the folders below too. A declaration in a file of the view is written unless its name starts with an underscore (until a
written declaration uses it), `-exclude` matches its qualified C++ name, or `-include` is given and does not match.
**Referenced types** (a type that a written declaration names and the view does not write) get their own section, written
the same way; a type that no header in reach declares is an empty struct with a `// C++: spelling` comment.

### 12.3 How declarations are written

| C/C++ | View |
|---|---|
| `int`, `unsigned char`, `long long`, `size_t`, `bool` | `C.int`, `C.uchar`, `C.longlong`, `C.size_t`, `C.bool` |
| `uint8_t` ... `int64_t`, `uintptr_t` | `uint8` ... `int64`, `uintptr` |
| `const char *`, `char **`, `T &` | `*C.char`, `**C.char`, `*T` (const is not shown) |
| `void *`, `std::nullptr_t`, a pointer to member | `unsafe.Pointer` |
| `R (*)(A, B)`, `int (&)(...)` | `func(A, B) R`, with `...any` for C varargs |
| `va_list` | `C.va_list` |
| `T[4]`, `T[]`, `T[2][3]` | `[4]T`, `[0]T`, `[2][3]T` |
| `typedef struct { ... } Color;`, `typedef struct S S;` | `type Color struct { ... }`, `type S struct{ ... }` (the typedef is the tag) |
| `typedef Vec2 Point;`, `using Id = unsigned;` | `type Point = Vec2`, `type Id = C.uint` |
| `struct S;` (only declared) | `type S struct{}` with `// incomplete type: declared, not defined` |
| `struct stat` and `int stat(...)` | `type struct_stat struct{...}` and `func stat(...)` |
| `enum E { A, B = 5 }` | `type E C.uint` and `const ( A E = 0; B E = 5 )` (`C.int` with a negative value, `C.ulong` or `C.long` when needed, the declared type of `enum class E : unsigned char`; scoped constants are `E_A`) |
| an enum with no name | untyped constants |
| `union U` | a struct with `// union: the fields share their storage` |
| a bit field, an anonymous union or struct member | `a C.uint // bit field of 3 bits`, `anon1 struct { ... } // anonymous union: ...` |
| `int f(int a, const char *)` | `func f(a C.int, _ *C.char) C.int` (names for all or none; leading underscores of names are dropped) |
| `namespace geo { struct Vec; }` | `geo_Vec` with `// struct geo::Vec` |
| a class | its public data members, its public bases embedded, its public methods as `func (this *C) m(...)` (value receiver when `const`), the rest as `//` lines after the type |
| a template | `// struct template geo::Box<typename T>` |
| a second overload | `// overload: geo::dot(...)` (a declaration again with the same type is the same function: nothing is written) |
| a struct, enum or typedef written inside a struct (`struct S { struct N { int a; } *p; };`) | `type S_N struct { a C.int }` with `// struct S::N`, also when `S` was forward declared first (`typedef struct S S;`); a struct that a field only names (`struct Later *next;`) is no member: it belongs to the namespace around the class |
| `[[deprecated("use g")]]`, `__attribute__((deprecated))` on a function, variable, struct, enum, typedef or enumerator (not on a field or a member function) | a `// Deprecated: use g` paragraph above the declaration, after a bare `//` line when another comment is there (`// Deprecated: the header marks it deprecated.` without a message) |
| `struct { int key; } entries[16];`, `struct { char c; } *cursor;`, `extern struct { int x; } g;`, `typedef struct { int a; } *P;` | the unnamed struct, union or enum is written inline, inside the declarators: `entries [16]struct { key C.int }`, `cursor *struct { c C.char }`, `var g struct { x C.int }`, `type P = *struct { a C.int }` (an unnamed enum is its underlying integer type) |
| `typedef int v4 __attribute__((vector_size(16)));`, `_BitInt(24)` | `type v4 [16]byte`, `type bi24 [4]byte`: a stand-in of the compiler's `sizeof`, with `// aligned(N)` for its alignment (a 3-lane `double` vector is 32 bytes); the declarations that use it still check. If the size cannot be asked it is the tree's reckoning, else an empty struct |
| `_Atomic(int) x;`, `int X::*pm`, `decltype(expr)` | `x C.int // _Atomic`, and `unsafe.Pointer` with a comment that says what was lost |
| `__attribute__((packed))`, `aligned(N)`, `alignas(N)` | `// layout: packed, aligned(16)` above the type, `// aligned(8)` after the field; the record is also listed under "Not translated" as "layout differs" |
| `extern int counter;`, `extern const char *name;` | `var counter C.int`, `var name *C.char` |
| `#define BUFSIZ 8192` and other macros that are numbers, chars, bools or strings | `BUFSIZ = 8192` in the `const` block |
| `#define RAYWHITE CLITERAL(Color){ 245, 245, 245, 255 }` | `var RAYWHITE = Color{245, 245, 245, 255}` |
| `constexpr int kMax = 3 * 4;` | `const kMax C.int = 12` |

A struct macro's literal is written positionally, from the values of the compiler's own initializer: a
pointer field gets `nil` (also for `0` and `NULL`), a string for a `char[8]` field is its bytes (`[8]C.char{97, 98}`),
an enumerator is the typed constant (its value, in a number field), an anonymous union member is an inline struct literal.
The operators and conversions that clang keeps in the tree are folded with C's rules for the type that the tree gives
each node (the desugared one, so `size_t`, `uint32_t` and `int8_t` wrap like the types they stand for): `(size_t)-1` is
18446744073709551615, `(int8_t)200` is -56, `-((uint32_t)4294967295)` is 1, `~0` of an `int` is -1, `'\xff'` is -1 (char is
signed on the targets of this phase), `-7/2` is -3 and `-7%3` is -1, `(long)(3.14159)` and `(short)(-2.5)` truncate toward
zero, `0.1f` into a double field is 0.10000000149011612, `(float)16777217` is 16777216, a constant that does not fit the
type of its field wraps to it (`(U){ -1 }` into a `uint64_t` is 18446744073709551615), and a `bool` is `true` or `false`.
A field of a 128-bit integer type (a `[16]byte` in the view) or of `long double` (a Go constant has double precision) leaves
the macro untranslated ("a 128-bit or long double field"). A number converted to an enum takes the bits of the enum's underlying
type (`(EC)(+(2147483648))` into an `enum EC : unsigned char` is 0), a bit field keeps its low bits (and its sign:
`(SB){ 9 }` into `signed int s : 4` is -7), and `sizeof(T)` / `alignof(T)` are asked of the compiler. What C leaves undefined (division by zero, signed overflow, a shift by a negative count or the width or more, a float outside
the range of the integer it is converted to) is not folded, the macro is listed as not translated, as is a value Go has no
constant for (`-0.0`, infinities, NaN). `TestFoldFuzz` checks this against the compiler: seeded random expressions (casts
among all the integer and float types, the typedef-spelled ones, every operator, literals at the edges of their ranges)
are macros of single-field structs, a C++ program prints each expression's value, and the view's literal must be that value
(three seeds of 150 macros by default, about 2 s; `GOCPP_FOLDFUZZ_SEEDS` and `GOCPP_FOLDFUZZ_N` for more; 30 seeds of 600
macros were run: no wrong value). A group of macros that the compiler warns about (undefined behaviour) must make a view.

What is not translated, and where it goes: function-like macros, pointer macros, macros that are not an expression or not a constant, and macros that depend on where
they are used (`__FILE__`) are listed by reason under `// Not translated`; templates are comments; operators,
constructors, destructors, static members, conversions and member templates are comments after their class; default
arguments, attributes other than the layout ones and `deprecated`, `noexcept`, access other than public, and the documentation comments of the header are dropped.

### 12.4 The probe cache, and what depends on the machine

The type and value of every macro are asked of the probe of step 12.2, so they use the probe's cache on disk (§9):
by default `~/.cache/gocpp/probe`, off with `GOCPP_NOPROBECACHE=1`, `Config.CacheDir` names another folder or `"off"`. A
run that finds every macro in the cache runs no compiler for them (`Stats.ProbeCached`); the key has the probe text, the flags (`-D`, `-I`, `-std`)
and the contents of every header the compiler read, so an edit of the header or another `-D` asks again. The views are byte
for byte the same with a cold and a warm cache.

A pure C header is read as C++ (the way gocpp includes it); a name that is a C++ keyword (`#define bool int`) is taken out of
the probe's file and the macro is listed as not translated, as is any macro whose value depends on where it is used
(`__FILE__`, `__LINE__`, `__COUNTER__`). The compiler runs with gocpp's usual environment (`LC_ALL=C`, and
`NIX_CC_USE_RESPONSE_FILE=0` so that Nix's clang wrapper takes a folder with non-ASCII characters in `-I`; see "Paths with
a double quote or non-ASCII letters" in build.md).

### 12.5 Tests

`internal/typedinterop/bindgen`: unit tests (the scanner against hostile and chunked input, `fuzz`-seeded, and against
`encoding/json` on the real dumps; the preprocessor reader and the scope rules on synthetic `-dD` text; the type, name, macro
and literal rules), golden files for the headers of `testdata/headers` (C, C++, an umbrella with every `-scope`, `-D`, `-I`,
`-std`, patterns, `-pkg`) with a `gocpp fmt` and `gocpp check` of each, the header zoo (`<stdio.h>` ... `<regex>`, sqlite3,
zlib, SDL2, raylib: checked, deterministic, facts asserted; goldens guarded by a hash of clang and the header files) and the
error cases (missing header, missing include, a header that does not compile, no clang). `cmd/gocpp/bindgen_test.go` runs the command.
`cmd/gocpp/bindgen_libs_test.go` runs it over the real headers of raylib, sqlite3 and SDL2 (one test each: counts above a floor, exact
signatures, the view under `gocpp fmt` and `gocpp check`, and typed interop in agreement with it); the review that they come from is
[docs/guides/bindgen-review.md](../guides/bindgen-review.md).

## 13. The interop mode (step 12.11, ADR-101)

[ADR-101](../adr/101-the-interop-mode-is-chosen-by-a-flag-and-falls-back-to-opaque-without-clang.md) has the reasons
for the flag and the fallback, [ADR-131](../adr/131-typed-interop-is-the-default-mode.md) for the default.
Opaque mode (Phase 04) leaves every question about a C/C++ name to the C++ compiler. Typed mode (this phase) asks
clang++ with probes. The mode is the user's choice, and the default is the automatic one: typed where clang++ is
installed, opaque where it is not.

### 13.1 Choosing the mode

| Source | Spelling | Priority |
|---|---|---|
| the flag, on `check`, `build`, `run`, `test`, `emit`, `vet`, `expand` (and `debug`, `export`, which take the build flags) | `-interop=typed`, `-interop=opaque`, `-interop=auto` | first |
| the environment | `GOCPP_INTEROP=typed`, `opaque` or `auto` (`gocpp env GOCPP_INTEROP` shows it) | second |
| the older environment switch | `GOCPP_TYPED_INTEROP=1` is `typed`, `0` is `opaque` (any other value is ignored) | third |
| nothing | `typedinterop.DefaultMode` | last |

An empty variable is unset. A value that is not a mode is an error: for the flag the usual flag error, for the variable
`gocpp: GOCPP_INTEROP=fast: want typed, opaque or auto` (status 2; the flag hides a bad variable). The up-to-date check
of `gocpp build` has the flag in its key (the variables are in it already). The mode of a user who sets nothing is
**auto** (`typedinterop.DefaultMode`); §13.6 says why.

- `typed`: the probes run. No clang++ is an error.
- `opaque`: nothing of this phase runs; the output is Phase 04's, byte for byte.
- `auto`: typed when clang++ is installed, else opaque, **without a word**. `gocpp check -v` and `gocpp build -v`
  (also `run`, `test`, `vet`) print one line that says which mode was used and why
  (`interop: no usable clang++ (not found, or it does not work), using opaque mode for C/C++ names (...)`).

### 13.2 What typed mode runs

In every command that checks packages (`check.go`), after the packages are loaded and before they are type-checked:

1. the **decision** (`typedinterop.Decide`, `prepareTyped`): see §13.3;
2. the **hooks**: `check.Config.Constants` (gap E1, §10) and `check.Config.CppMembers` (gap E2, §11), made by
   `typedinterop.Hooks`; both use one `probe.Runner` for the command, so the compiler answers one question once;
3. after the type check, for every package that checked without errors, imports a header, and is not the standard
   library's: **one** `typedinterop.Analyze` with `Decls`, `StringToCStr` and the target's data model (LP64, LLP64 for
   Windows), then `icheck.CheckReport` on that report (§7). The errors and warnings join the package's errors and print
   like the other errors of `gocpp check`; the packages are analyzed at the same time (at most 4), the compiler runs are
   bounded by the probe's own limit. The checker uses `StringToCStr` like the code generation, so the two agree on what
   a Go string passed to `const char*` is: that is the tie that §7 asked for.
4. `build`, `run`, `test` and `emit` then give the same report to the lowering (`lower.Interop.SetTyped`, §8), so
   a package is probed once for both.

`gocpp expand` has the constants substituted before the type check (the printed output has your names, not the
numbers); `gocpp vet` has the early errors like `check` (the analyzers that need the C++ types of the probes are
Phase 13's). A build for another system than this one is typed only when every header is the C++ standard library's
(the probes read this machine's headers) and gets no early errors.

### 13.3 Without clang++ (the fallback)

`typedinterop.Decide(ctx, setting, needed)` looks for clang++ (`GOCPP_CLANG`, else `clang++` in `PATH`; clang 17 or newer,
like every compiler of gocpp) **only when the program has something to probe**: a package that is not one of gocpp's own
standard library packages (`load.Package.Std`) imports a C/C++ header, a C++ standard header such as `<vector>` included (`typedinterop.Needed`). A program of Go alone never asks.

| Mode | clang++ there | clang++ missing |
|---|---|---|
| `opaque` | opaque | opaque |
| `auto` | typed | opaque, silent (a line at `-v`); `gocpp check` and `gocpp build` do not fail |
| `typed` | typed | **error**, exit status **3** (the status of `gocpp bindgen` without clang++): `gocpp: -interop=typed: typed interop needs clang++: the C++ compiler "..." was not found: ...; install clang++, set GOCPP_CLANG to its path, or use -interop=opaque` |

The message names the source of the mode (`-interop=typed`, `GOCPP_INTEROP=typed`, `GOCPP_TYPED_INTEROP=1`). Two more
fallbacks, per package and always to opaque: a probe that fails for another reason (a header that clang cannot read, a
compiler run that times out) leaves that package opaque (a line `gocpp: typed interop: ... (using opaque mode for P)` with
`typed` and with `-v`, none with `auto`); and the probes of a program for Windows pass the MinGW-w64 target, its headers and the Win32 defaults (step 14.6, ADR-116; before it,
`auto` was opaque on a Windows host and for any target other than this machine's). A Windows host with no clang++ that gocpp can find stays
opaque with `auto`.

### 13.4 For the language server (step 12.13)

`internal/lsp` is not touched by this step. It takes everything it needs from `internal/typedinterop`:
`ResolveMode(flag, os.Getenv)` for the setting (the editor can pass `-interop` or an initialization option as `flag`),
`Decide` for the clang rule, and `Hooks(HookOptions{GOOS, GOARCH, Runner})` for `check.Config.Constants` and
`check.Config.CppMembers`. The early errors and the lowering's report are not needed in the editor's path.

### 13.5 The compatibility evidence

The rule (README §1): a program that opaque mode and the C++ compiler accept is accepted by typed mode, and prints the
same. `TestCorpusTyped` (`tests/interopmodes_test.go`) runs every program of the repository that imports a header with
`GOCPP_INTEROP=typed` and each compiler (g++ and clang++), against the expectations of the opaque suites (`// Output:`,
exit code, stderr), which opaque mode meets. Measured on 2026-10-19 (clang 21.1.8, g++ 15):

| Corpus | Found | Run in typed mode | Same output, both compilers | Not run |
|---|---|---|---|---|
| `tests/interop` | 228 | 175 | 172 | 39 error tests (`// ERROR`), 13 without a header, 1 later phase |
| `tests/zoo` (headers and libraries) | 23 | 23 | 23 | |
| `tests/run` | 2012 | 84 | 84 | 1864 without a header, 63 other target, 1 pending |
| `tests/pkgs` | 61 | 7 | 7 | 54 without a header |
| `examples` | 19 | 5 | 5 | 12 without a header, 1 later phase, 1 pending (raylib and SDL are built, not run: no display) |
| **total** | 2343 | **294** | **291** | |

The three programs that differ are tests of mistakes that opaque mode rejects and typed mode words itself, so they are
not valid programs: `implicit_conversion` and `assign_to_call` (typed mode accepts them: that is what it adds, ADR-087),
and `addr_loopvar_error` (with clang++ the early error of the typed checker, `cannot use &i (value of type *C.int) as
*Counter value`, comes before the lowering's error about the loop variable: the program is wrong twice). They are
`typedExceptions` of the test. No valid program of the corpus is rejected, and none prints something else. The
`tests/typed` programs (42) have their own test (`TestTypedRun`, with the C++ twins).

### 13.6 The default

`typedinterop.DefaultMode` is **auto** ([ADR-131](../adr/131-typed-interop-is-the-default-mode.md), user decision of
2026-10-21; until then it was opaque, ADR-101). A user who sets nothing gets typed interop when a clang++ is found and
Phase 04's opaque pipeline, with no message, when it is not. What the user sees:

- a program of Go alone, or one that imports only gocpp's own standard library packages (`fmt`, `os`, ...), pays nothing for the
  clang search: clang++ is looked for only when a package that is not one of those imports a C/C++ header (§13.3). A program
  that imports a C++ standard header itself (`<vector>`, `<algorithm>`) is probed like any other (`examples/cpp-std`: 1.56 s cold);
- a build of a program that imports a header costs the probes: warm 0 to 30 ms, and on an empty result cache (the
  first build of a program, or after a header changed) 0.5 to 0.7 s for C-style or own headers and 1.2 to 1.6 s for
  programs that include large C++ library headers themselves (§13.7);
- early errors in Go's words before the C++ compile (with g++ as the build compiler they are warnings, ADR-079),
  constant macros as array lengths (E1), promoted members (E2), the string-to-`const char*` conversion;
- `-interop=opaque` or `GOCPP_INTEROP=opaque` (also in a script or an editor's server environment) gives
  Phase 04's behavior, byte for byte, and costs nothing.

The three conditions that ADR-101 and ADR-108 had put before the switch were not all shown (cold probing under 0.5 s
for every example, clang 17 to 20, the interop corpus on a Windows host); the user accepted the measured cold cost and
the wording of the exit criterion (Phase 12 README §5), and the risks are in ADR-131. The test suites of `cmd/gocpp`
and `tests` set `GOCPP_INTEROP=opaque` in their `TestMain`, so that they keep testing the opaque pipeline (the
contract of Phase 04) whatever the default is; `TestCorpusTyped` runs the programs that import headers in typed mode,
and `TestInteropPrecedence` (the `default` case) and `TestDefaultModeIsAuto` pin the default itself.

### 13.7 Cost

Measured with `GOMAXPROCS=4` on a 192-core machine at a load of about 20 (an earlier run at a load of 300 to 2000 gave
the same warm numbers and 1.5 to 2 times the cold ones), `gocpp check` and `gocpp emit` on the examples, opaque against
typed (median of 7 warm runs; "cold" is the first run with an empty cache folder, so the probe cache and the kind
cache are empty too):

| Example | check warm (opaque) | check warm (typed) | emit warm (opaque) | emit warm (typed) | check cold (opaque) | check cold (typed) |
|---|---|---|---|---|---|---|
| `cpp-std` | 34 ms | 59 ms | 24 ms | 59 ms | 0.45 s | 2.4 s |
| `sqlite` | 37 ms | 56 ms | 25 ms | 55 ms | 0.06 s | 1.4 s |
| `raylib` | 33 ms | 51 ms | 24 ms | 52 ms | 0.06 s | 0.9 s |
| `mixed-sources` | 41 ms | 39 ms | 20 ms | 50 ms | 0.23 s | 2.1 s |

The warm overhead is 0 to 25 ms for `check` and 28 to 35 ms for `emit` (the exit criterion is 50 ms: met). It needed one fix
in this step: `gocpp check` did not keep the compiler detection between runs (`build.SetDetectCache`, which `build` does),
so every warm typed check started clang++ twice (0.3 s on a loaded machine). The cold runs fill the probe cache (§9) with
one compiler run for the kinds, one batch of the probes and a run for each declaration group, three stages one after
the other; most of the time is the parsing of the C++ library's headers by clang (2 to 5 CPU seconds for the examples).
**The cold cost exceeded the 0.5 s of the earlier criterion** in the measurements of this table. Step 12.14 retains PCH
acceleration for expression/function probes and overlaps include discovery, but review removed its unsound smaller type
preamble (ADR-117, §14.3). Measurements made with that preamble are historical, not current acceptance evidence. A
retained-PCH check is not a fully cold check. The current numbers are in ADR-121 and §14.3 (fully cold, typed minus
opaque: `raylib` 0.66 s, `sqlite` 0.65 s, `multi-pkg` 0.47 s, `mixed-sources` 1.17 s, `cpp-std` 1.56 s), and the criterion
in the Phase 12 README §5 is worded after them. With the default `auto` (§13.6) a user meets these numbers on the first
build of a program on a cold cache.

### 13.8 Tests

`internal/typedinterop/mode_test.go` (the order of the sources, the clang rule), `cmd/gocpp/interop_test.go`
(`TestInteropPrecedence`: eleven combinations of flag and variables with the `-v` note; `TestInteropFlagEverywhere`:
the flag and its help in nine commands, `debug` and `export cmake` too; `TestInteropNoClang`: status 3 for typed in six commands, silence for auto
and opaque, a program of Go alone; `TestInteropEarlyErrors`: the early error in `gocpp check` and `build`, warnings with
g++; `TestInteropFallbackWindows`: a build for Windows without clang++ runs on the real PC),
`tests/interopmodes_test.go` (`TestCorpusTyped`), the goldens of `gocpp env`, the help texts, and the
feature matrix (`docs/spec/feature-matrix.md`).

## 14. The tests of step 12.12

The step collects the evidence for the exit criteria of Phase 12. Most tests were written with the steps they test; this
step adds what was missing and measures the three numbers that the exit criteria give.

### 14.1 What shows which criterion

| Criterion | Test |
|---|---|
| Phase 04 suite and header zoo in both modes | opaque: `TestInterop`, `TestZoo`, `TestRun`, `TestPkgs`, `TestExamples`; typed: `TestCorpusTyped` (§13.5, the same programs with `GOCPP_INTEROP=typed` and both compilers). The build-and-run half rests on the 12.11 run of `TestCorpusTyped` (294 programs); step 12.12 did not repeat it, it adds the check-only sweep of §14.2 |
| No typed-mode error on a program that compiles in opaque mode | `TestTypedCheckCorpus` (`tests/typedcheck_test.go`): `gocpp check` of every corpus program that imports a header, opaque (must pass) then typed (must pass), with g++ and clang++; clang++ prints nothing; plus the accepted programs of the checker's zoo (`accept_*`, built by both compilers in `TestTypedCheckZoo`) |
| At least 30 early errors in gocpp's words | the `reject_*` programs of `internal/typedinterop/icheck/testdata/zoo` with golden files (`TestRejectZoo`; 22 programs, 77 diagnostics), each at a line where the C++ build fails (`TestTypedCheckZoo`), the `err_*` programs of `tests/typed`; `TestEarlyErrorInventory` counts them and `TestEarlyErrorWords` rejects clang's own words in the goldens |
| `T&` parameters and results, mixed arithmetic | `accept_refs.gp`, `accept_arith.gp` (zoo), `tests/typed/refs_numeric.gp` (with its C++ twin) |
| E1 (`[cstdio.BUFSIZ]byte`) | `tests/typed/macro_consts.gp` (typed builds and runs, `// opaque: reject`), `TestTypedConsts` (the opaque gate message) |
| E2 (embedded `std.vector[int]`) | `tests/typed/embed_promoted.gp` (`// opaque: reject`), `TestTypedEmbedInterop`, `TestTypedCheckPromoted` |
| Float and string macros | `tests/typed/macro_consts.gp` (`TAU`, `TENTH`, `GREETING`, `SEP`, `ESC`), `TestConstZoo`, `TestConstBigIntIsExact` |
| PCH acceleration changes no answer; a type run's answer is used only when it describes a file of the program (else the full preamble) | `TestPCHOracle` (`tests/pchoracle_test.go`) compares `gocpp __typed-probe -decls -stable` with PCH `off` and `wait`, using empty caches. It now requires equal declaration dependencies too (no subset exception). The previous 286-program phase-12 run predates the smaller-preamble removal; no full oracle rerun is claimed for round 3. Focused tests: `TestPCHAnswersAreTheSameAsWithout`, `TestPCHTypeRunsKeepTheMembers`, `TestPCHTypeRunsOfProgramTypesAnswerAsWithout`, `TestPCHDeclarationsFromProgramFiles`, `TestSignatureTypesAreAskedBesideTheProbeRun`, `TestPCHCutOffIsMadeAgainWithTheSameAnswers`, `TestTypeRunsKeepFullBlockAnswers`, `TestTypeRunsKeepFullBlockPreprocessor`, `TestTypeRunsKeepFullBlockNameLookup`, `TestTypeRunsHeaderEditKeepsFullBlock`; fake-compiler tests retain PCH lifecycle/invalidation checks and add `TestIncludeInfoCancellationReleasesLastWaiter`, `TestIncludeInfoCancellationPreservesSharedWaiter` and `TestIncludeInfoConcurrentLiveWaiters` |
| A header edit invalidates the probes | `TestTypedHeaderEditInvalidatesProbes` (`cmd/gocpp`: edits that keep the size and the modification time, a macro value and a signature; an unedited header and a return to an old one run no compiler), the cache tests of `probe` (`TestCacheHeaderEditInvalidates` and others) |
| Warm builds add little | `TestTypedWarmRunsNoCompiler` (for a warm check of each example: 0 probe-cache compiler runs and 0 calls that reach clang++, counted by a logging wrapper; no clock), `BenchmarkTypedCheck`, `BenchmarkWarmProbe` |
| JSON of other clang versions | `TestReaderIgnoresWhatItDoesNotKnow`, `TestReaderOfRecordsIgnoresWhatItDoesNotKnow`, `TestReaderOfATruncatedDump` (§14.4), `tools/clang-matrix.sh` |

### 14.2 The compatibility sweep

`TestTypedCheckCorpus` takes the loaders of the suites (`tests/interop`, `zoo`, `run`, `pkgs` and `examples`), leaves out
what `TestCorpusTyped` leaves out (error tests of opaque mode, programs without a header import, `typedExceptions`) and
the tests that expect a failed build (`ExitCode: 1` with a `// Stderr:` pattern that names `x.gp:line:`: they fail to build in
opaque mode too; programs that run and panic, exit status 2, are checked), and checks the rest in both modes. Measured on 2026-10-20: **287 programs, each with
g++ and clang++, 0 typed errors, 0 typed messages with clang++**. It runs in about 30 s on the 192-core machine, against minutes
for `TestCorpusTyped`, so it is the one to run after a change to the checker or the probes.

### 14.3 Speed

`BenchmarkTypedCheck` (`cmd/gocpp`, `go test -run '^$' -bench TypedCheck -benchtime 7x -count 3 ./cmd/gocpp`) runs `gocpp check .` as a
process on each example that imports a header; the cache folders are new for each "cold" run and kept for a "warm" one (the
compiler-detection cache is on, as a user has it). The variants:

- `opaque-cold`, `typed-cold`: empty cache folder. For typed this is the **first check of a flag set on a machine**: it makes the precompiled header (§9), which is per compiler and flag set.
- `typed-cold-nopch`: the same with `GOCPP_PROBE_PCH=off` (no PCH; full type preambles and prefetched include discovery).
- `typed-cold-pch`: no results or toolchain answer, but the PCH that an earlier run of the **same package** made. This is
  **retained-PCH / partial-cache**, not a fully cold check. A package with different `pkg-config` flags needs its own PCH.
- `opaque-warm`, `typed-warm`: the caches of the first run.

Step 12.12 measured (2026-10-20, median of 10, load 20 to 30), typed minus opaque, cold: `cpp-std` 2.04 s, `mixed-sources` 1.81, `multi-pkg` 0.51,
`raylib` 1.03, `sqlite` 1.26.

**Historical step-12.14 measurements, before round-3 smaller-preamble removal.** These numbers used the withdrawn
optimization and are not the performance of the current implementation. Median of 7 runs, two repeats, on a 192-core
machine at load 22 to 30; the spread of the two medians is in brackets; times in ms. No other example row has been
remeasured in round 3; only the separate `raylib` measurements below are current.

| Historical example | opaque cold | typed cold (first flag set) | retained-PCH (not fully cold) | typed cold, PCH off | opaque warm | typed warm |
|---|---|---|---|---|---|---|
| `cpp-std` | 406 | 1975 [1974-1976] | 1999 [1973-2024] | 2127 [2091-2163] | 33 | 57 [51-62] |
| `mixed-sources` | 231 [229-233] | 1615 [1606-1623] | 1529 [1502-1555] | 2094 [2091-2096] | 38 | 54 [48-60] |
| `multi-pkg` | 65 [64-66] | 505 [496-514] | 286 [284-288] | 525 [519-530] | 41 | 30 [30-31] |
| `raylib` | 54 | 560 [551-569] | 354 [353-356] | 792 [767-817] | 32 | 43 [43-44] |
| `sqlite` | 49 | 566 [559-573] | 379 [379-380] | 1062 [1050-1073] | 33 | 43 [40-45] |

Historical typed-minus-opaque differences, in seconds; the retained-PCH column is partial-cache, not fully cold:

| Historical example | 12.12 | PCH off (old 12.14) | first flag set (old 12.14) | retained-PCH (old 12.14) |
|---|---|---|---|---|
| `cpp-std` | 2.04 | 1.72 | 1.57 | 1.59 |
| `mixed-sources` | 1.81 | 1.86 | 1.38 | 1.30 |
| `multi-pkg` | 0.51 | 0.46 | 0.44 | **0.22** |
| `raylib` | 1.03 | 0.74 | 0.51 | **0.30** |
| `sqlite` | 1.26 | 1.01 | 0.52 | **0.33** |

These historical numbers include the removed three-run preprocessor-equivalence check and smaller type preambles.
The retained-PCH variant primes with the **same package** (`keepOnlyHeader` removes everything but `pch/`). A PCH is
per flag set: packages with different `-I` folders need separate headers. Its partial-cache savings do not meet a
criterion that requires an empty cache. The later independent round-2 `raylib` benchmark, also before removal, found
604–609 ms fully cold typed versus 62–63 ms opaque: a 0.541–0.546 s difference, already above the target.

**Current round-3 measurement (2026-10-20).** Only `raylib` was remeasured after restoring full type preambles:
`go test ./cmd/gocpp -run '^$' -bench '^BenchmarkTypedCheck$/^raylib$/' -benchtime=7x -count=2`, through the shared
test slot with isolated offline caches and phase 12. Linux/amd64, Clang 21, 192 logical CPUs (Threadripper PRO 9995WX);
the two columns are independent medians of 7 checks, in ms. Machine load was not recorded.

| `raylib` variant | first median | second median |
|---|---:|---:|
| opaque, fully cold | 65.30 | 62.22 |
| typed, fully cold (first PCH build included) | 917.3 | 911.5 |
| typed, fully cold with PCH off | 935.3 | 938.6 |
| typed, retained-PCH / partial-cache | 714.1 | 710.7 |
| opaque, warm | 36.02 | 37.05 |
| typed, warm | 57.48 | 57.07 |

The fully cold typed-minus-opaque difference is **0.849–0.852 s**, above 0.5 s. Even the retained-PCH difference is
0.648–0.649 s; it is not cold acceptance evidence. Warm overhead is 20–21 ms for this example only. Restoring full
preambles materially increases `raylib`'s type-dump cost; no current result is inferred for the other four examples.
A focused `TestPCHOracle/examples_raylib_main.gp` also passed with the new exact dependency comparison; that is one
program, not a rerun of the 286-program oracle.

**Step 12.14b (2026-10-21, ADR-121).** Two changes, neither of which changes an answer: the types that the signatures of
the called functions name are asked beside the probe run (a whole stage fewer for `sqlite` and `mixed-sources`), and the type
runs of the program's own types load the PCH and are made again with the full block when their answer describes a declaration
of the header (§2.2). Typed minus opaque, in seconds, medians of 9 interleaved rounds (one opaque and the two typed variants per
round), Linux/amd64, clang 21, 192 CPUs, no pinning, load average 6 to 15 from other jobs; "retained PCH" is `typed-cold-pch`
(only the header is kept: a second package or a later first check of a flag set):

| example | fully cold before / after | retained PCH before / after | typed warm before / after (ms) |
|---|---|---|---|
| `raylib` | 0.843 / **0.659** | 0.641 / **0.456** | 61 / 58 |
| `sqlite` | 1.180 / **0.652** | 0.977 / **0.438** | 65 / 58 |
| `multi-pkg` | 0.476 / **0.468** | 0.241 / **0.247** | 38 / 36 |
| `mixed-sources` | 1.536 / **1.169** | 1.530 / **1.152** | 60 / 52 |
| `cpp-std` | 1.586 / **1.560** | 1.592 / **1.519** | 66 / 52 |

Floors (ADR-121): a compile of the library block costs 315 ms, making its header 390 ms, loading it 62 ms, an empty clang run
22 ms. The probe stage cannot use a header that is still being made, and the type stage cannot start before the probe stage
names the types, so a fully cold check costs at least the kind probe + one block compile + one header-assisted stage (about
0.52 s before any bookkeeping, 0.65 s measured). **The criterion is met with a retained header for C-style packages (0.44 to
0.46 s; `multi-pkg` 0.25 s), not fully cold (0.65 s), and not for packages that include large C++ library headers themselves
(`mixed-sources`, `cpp-std`: 1.2 to 1.6 s, bounded by the compile of those headers and the dumps of std types).** The criterion
was reworded after these numbers (Phase 12 README §5) and the default is `auto` ([ADR-131](../adr/131-typed-interop-is-the-default-mode.md), §13.6).
`TestPCHOracle` compares 288 programs with and without the header (0 differences, exact dependency lists; 8 type
runs were refused and made again).

### 14.4 Clang versions

The dev shell has clang 21.1.8 only (clang 20 is in the Nix store as a derivation that is not built, 17 to 19 are not there), so
the versions 17 to 20 could not be run. `tools/clang-matrix.sh [clang++ ...]` is the matrix: for each program (by default
`clang++-17` to `clang++-21` that are in PATH) it sets `GOCPP_CLANG` and runs `./internal/typedinterop/...`, the typed tests of
`cmd/gocpp` and `TestTypedCheckCorpus`. That the version asked for is the one used is tested, not assumed: `tsupport.Clang` (the
helper of the probe, checker and mapper tests) and `probe.FindClang` (the product) follow the same rule, `TestClangInUse` and
`TestMatrixClangReachesTheDriver` compare the major version with `GOCPP_EXPECT_CLANG_MAJOR`, and
`TestProbeTestsUseTheClangOfTheMatrix` runs a probe test through a logging wrapper named in `GOCPP_CLANG` and finds its compile line
in the log. (An earlier version of the helper looked up `clang++` in PATH whatever `GOCPP_CLANG` said, and several tests cleared the
variable; both are fixed.) What can be tested with one clang is the tolerance of the reader: the real dumps of clang 21 are changed in
the ways a version can differ without changing the meaning (unknown fields at every object, fields the reader does not use
removed, keys reversed or sorted), and the decoded probes, records and enums must stay equal. What this does not show is a
renamed or restructured node (clang 21 changed how the AST represents qualified type names, for example); only the matrix shows that.

### 14.5 Not covered

- `type Handle geo.handle` (a Go type declared over a C type) is not supported by the lowering at all ("a later step of Phase 04"),
  and `gocpp check` accepts `var h Handle = "s"` in both modes; the typed errors listed in the 2026-10-14 note of the STATE file
  for named types over C wait for that feature.
- Statements that the probes do not mirror are not typed: `p.x = "a"` (a string to an `int` field), a keyed literal with an
  unknown field, `return` of a C number in a function that returns `string` (the lowering reports the last one in `gocpp build`,
  the checker does not). They stay the C++ compiler's, as the compatibility rule allows.
