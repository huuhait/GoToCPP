# Phase 01: Front-end (parse, load, type-check, format)

> Turn `.gp` source files into a fully type-checked Go AST by reusing Go's own front-end,
> adding only what gocpp needs on top: header imports (each header is a package), C++
> namespaces used like packages, C++ template arguments, dotted C++ type names and C++ names
> that are Go keywords. Plus `gocpp fmt`, so formatting works from the first week.

| | |
|---|---|
| Depends on | Phase 00 |
| Unlocks | Phase 02 (codegen needs typed AST) and the **language server's early track** (Phase 11 v0: diagnostics, hover, go-to-definition, completion, formatting) |
| Size | medium (about 3 weeks) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

Writing a Go parser and type checker from scratch takes months, and the hard parts (untyped
constants, method sets, embedding, generics type inference, initialization order) are easy to
get subtly wrong. Go ships all of this as libraries: `go/scanner`, `go/parser`, `go/ast`,
`go/types`, `go/constant` and `go/format`. Since gocpp *is* Go syntax, we reuse them:

- **Parsing:** a small fork of `go/scanner` + `go/parser` with **three patches** in this phase
  (Phase 10 adds three more for `F!(...)` and macros, plus a small `go/printer` fork).
- **Type checking:** the **stock** `go/types` with its `FakeImportC` option. That option makes
  every `C.something` type-check silently as "invalid". The front-end rewrites header-package
  and namespace uses (`raylib.InitWindow`, `std.vector`) into that form (§2.5), and gocpp treats
  the result as **opaque**: "a C/C++ thing; the C++ compiler will check it later".

The spike on 2026-10-13 proved this works, and it was re-run with the final import model (header
packages, namespaces as packages): a realistic file using `std::vector`, SDL, `<cstdio>`, a local
header and `std::filesystem` type-checked with **0 false errors** after the rewrite in §2.5. Real
mistakes in the gocpp part were still reported, including typos in namespace names (root README
§7). The audit found the gaps that §2.2 (patch 3), §2.8 and §2.9 now cover.

## 2. How it works

### 2.1 Front-end pipeline

```
 file.gp ──► scanner+parser (fork) ──► *ast.File (stock go/ast nodes)
                                            │
                     ┌──────────────────────┘
                     ▼
              classify imports ──► FileInfo{Packages, Includes, HeaderNames, NamespaceRenames}
              + //gocpp: directives (joined with FileInfo by the loader)
                     │
                     ▼
              preprocess AST:  drop header imports · raylib.X → C.X · std.X → C.std.X ·
                               T(opaque) → gp_as[T](…) · add import "C" where used
                     │
                     ▼
              namespace probe (only for files with header imports or namespace renames; cached)
                     │
                     ▼
              go/types.Check  (FakeImportC, our Importer, error collector)
                     │
                     ▼
              type-check template arguments of opaque instantiations (types.CheckExpr)
                     │
                     ▼
              opaque pass + unsupported-feature pass
                     │
                     ▼
              *check.Package { AST, types.Info, FileInfo, opaque set }  ──► Phase 02+ codegen
```

### 2.2 The fork: three patches to go/scanner + go/parser

The fork lives in `internal/syntax/{scanner,parser}`. It is copied from the dev shell's Go 1.25 standard
library (BSD-3-Clause; keep the license header). In Go 1.25, `go/parser` and `go/scanner` have
**no internal dependencies** (checked with `go list`), so the fork compiles alone. (Go 1.26 adds
an import of `go/internal/scannerhooks` to both, so a re-fork to 1.26 copies that small package
into the fork too; Go 1.27 no longer has that import. Found by the fork job, written down in
`FORK.md`.) Copy `go/printer/nodes.go`
too, because the parser's `performance_test.go` reads it. The fork still produces **stock
`go/ast` nodes**, so the stock `go/types` can check them.

**Patch 1: a Go keyword after `.` is a name, on the same line.** C++ uses names that are Go
keywords: `std::map`, `node.type()`, `range()`, `select()`, `default`. The stock parser fails on
`std.map[...]` (`expected selector or type assertion, found 'map'`). In the scanner: if the
previous token was `.` and the next word is a keyword **on the same line**, return
`token.IDENT` with that word as its literal (automatic semicolon insertion then works like for
any identifier). The same-line rule matters: an editor often has an unfinished `fmt.` with a
keyword on the next line, and upstream's `TestIncompleteSelection` expects that to stay a syntax
error. With the rule, all upstream scanner and parser tests pass (checked during the audit).
"The same line" is the physical line. One consequence (found by job 1.2): in Go's trailing-dot
chain style, a keyword-named method can't start the next line (`b.` then `select()` on the next
line is a syntax error), so `.select` stays on the line of its `.`.

```go
func examples() {
	m := std.map[std.string, int]{} // was a syntax error, now IndexListExpr on Selector(std, "map")
	t := node.type()                // C++ method named "type"
	n := posix.select(fd+1, &rd, nil, nil, nil)
}
```

(`posix` is `<sys/select.h>` imported under a name: `import posix "<sys/select.h>"`.) The
`x.(type)` type switch is not affected, because there the token after `.` is `(`.

**Patch 2: type-argument lists may contain constant expressions.** C++ templates take values
(`std::array<int, 5>`, `std::bitset<64>`). The stock parser fails on `std.array[int, 5]`
(`expected type, found 5`), whether it is used as a type, in a composite literal or in a call.
Each type argument is parsed with the "type-or-expression" parser (`parseRhs`) instead of the
type-only one, but only when the next token can start an expression; any other token keeps
upstream's error and recovery (`expected type, found '|'`), so half-typed code in the editor
recovers like Go (found in the review of job 1.3). The AST is a normal `*ast.IndexListExpr`. For gocpp generics, `go/types` still
reports `5 is not a type`, which is the correct error.

```go
var buf std.array[byte, 256]
bits := std.bitset[64]{}
```

**Patch 3: dotted type names.** Go's type grammar allows only `pkg.T`. C++ types often need
more dots, and the stock parser rejects them in every *type* position (expressions are fine):
`var d std.chrono.seconds`, `func f(p std.filesystem.path)`, `[]std.chrono.seconds`, a struct
field of type `std.chrono.seconds`, or a member type `std.vector[int].iterator`. The patch lets
`parseTypeName` loop over `.Name` parts, including after a type instance. `go/types` already
accepts these nested selectors when they are rooted in `C` (checked through the §2.5 rewrite).
One-dot types such as `std.vector[int]` and `*SDL.SDL_Window` need no patch.

`FORK.md` records the upstream version, every patch, and a re-fork procedure
(`internal/syntax/update.sh`: copy the new upstream, re-apply the patches as git patches, run
the tests). Phase 10 adds patch #4 (`name!(` is one name, for must-calls), patch #5 (`#Name` is
one name, for macro uses) and patch #6 (`macro` declarations, `lazy` parameters, macros on top
of a declaration), plus a small `go/printer` fork.

### 2.3 Import classification

Each import path string is classified by its shape. No new syntax is needed:

| Import | Kind | Becomes in C++ | Name used in code |
|---|---|---|---|
| `"fmt"`, `"myapp/engine"` | gocpp package | `#include "gpkg/…/pkg.hpp"` | `fmt.X`, `engine.X` |
| `"<raylib.h>"`, `"<SDL2/SDL.h>"`, `"<cstdio>"`, `"<vector>"` | system header | `#include <raylib.h>` | a package named after the file: `raylib.X`, `SDL.X`, `cstdio.X` |
| `"./physics.h"`, `"native/x.hpp"` (ends in `.h .hh .hpp .hxx .h++ .inl`) | local header (relative to the `.gp` file, then the include dirs) | `#include "…"` | `physics.X`, `x.X` |
| `rl "<raylib.h>"`, `_ "<GL/gl.h>"` | header with a chosen name, or none | as above | `rl.X`, or no name |
| `fs "::std::filesystem"` | renamed C++ namespace (optional) | nothing (just a name) | `fs.path` → `::std::filesystem::path` |
| `"::std"` (no name) | an error: namespaces need no import | — | the message says: use `std.X` directly, or rename it (with the path you wrote, e.g. `import chrono "::std::chrono"`) |
| `"./helper.cpp"`, `"./physics"`, `"./x.H"` (a `./` or `../` path that isn't a header) | an error | — | a local path must be a header (`.h .hh .hpp .hxx .h++ .inl`); `.c`/`.cpp` files next to the `.gp` file are compiled anyway |

**A header is a package.** Its name is the file name without directory and extension
(`<SDL2/SDL.h>` → `SDL`, `"./physics.h"` → `physics`), with characters Go doesn't allow turned
into `_`. Its members are the header's **C-level names**: functions, types, global variables,
enum constants and macros (`raylib.InitWindow`, `raylib.Color`, `raylib.RAYWHITE`). Rename it like a
Go import (`import rl "<raylib.h>"`) or include it without a name (`import _ "<GL/gl.h>"`).
A header gets **no name** when its file name is a Go keyword (`<map>`, `<sys/select.h>`), a
predeclared Go name (`<string>`, `<string.h>`, `<new>`) or already taken by another import
(`<math.h>` next to gocpp's `"math"`) or a package-level name. Only the last extension is dropped
(`<foo.bar.h>` → `foo_bar`); a name that would be empty, start with a digit, or be `C`, `_` or
`init` also gives no name. Two different headers with the same default name both get none, so
the result never depends on import order; the same header imported twice is
`raylib redeclared in this block`, as in Go. A name you choose follows Go's rules, except that it
can't be `C`, `init` or a predeclared name (`import string "<string.h>"` is an error, because
`go/types` never sees header imports and `string` would mean two things; use `cstring`). The
details and messages are in the interop spec. Name it yourself if you need its C names
(`import cstring "<string.h>"`). Using such a header by its file name gives a hint instead of a
bare type error (`string.strlen` → "give `<string.h>` a name: `import cstring "<string.h>"`",
checked in the spike). Header imports never cause "imported and not used" errors, because a
header also brings its namespaces and macros.

**C++ namespaces are packages and need no import.** Every namespace declared by the imported
headers can be used directly: `std.vector[int]`, `std.chrono.seconds`, `glm.vec3`, `ImGui.Begin`.
A namespace can be renamed when a name clashes or is long (`import fs "::std::filesystem"`, or
`import cfmt "::fmt"` next to gocpp's `fmt`). When a header's package name is also a namespace
(`<glm/glm.hpp>` → `glm`), `glm.X` looks in the namespace first, then in the C-level names
(Phase 04's kind probe picks the one that exists).

**`C` holds only C's built-in types and the boundary helpers**, as in cgo: `C.int`, `C.uint`,
`C.long`, `C.char`, `C.uchar`, `C.size_t`, `C.float`, `C.double`, `C.CString`, `C.GoString`,
`C.cstr`. Everything a header declares comes through its package or a namespace. `C` can be used
in every `.gp` file; the front-end adds `import "C"` only to files that use it (an unused import
would be a Go error). A local or package-level declaration named `C` gets a clear error ("`C` is
reserved for C's built-in types"). A `FileInfo` struct keeps everything the later phases need:
includes (in order), header package names, namespace renames, `//gocpp:` directives, and
positions for error messages.

There is no inline C++ "preamble" in v1: a small helper goes into a header next to the `.gp`
file (`"./mathx.h"` → `mathx.hypot3(1, 2, 2)`), and `.c`/`.cpp` files there are compiled too
(Phase 04).

### 2.4 `//gocpp:` directives (parsed now, used in Phases 04 and 06)

Directives are line comments before the `package` clause, like `//go:build`:

```go
//gocpp:pkg-config sdl2
//gocpp:link m pthread
//gocpp:cflags -DUSE_FAST_MATH
//gocpp:ldflags -L/opt/foo/lib
//gocpp:extern-c ./legacy.h
package game
```

Phase 01 only parses and validates them (unknown directive = error, with a "did you mean"
suggestion). Arguments are split like cgo's `#cgo` lines (quotes group, also inside a word, and
backslash escapes), and an unquoted argument that starts with `//` starts a comment, so
`//gocpp:link m // for sqrt` works (and so does a `// ERROR` mark in tests/errors).

### 2.5 Preprocessing before go/types (proven by the spike)

1. **Drop header imports** from the import declarations and store them in `FileInfo.Includes`,
   together with each header's package name.
2. **Rewrite header-package uses**: `raylib.InitWindow` becomes `C.InitWindow`, with positions
   kept for error messages. `FileInfo` records which header package each use came through, for
   diagnostics, codegen and the editor.
3. **Rewrite namespace uses**: an identifier used as `x.` that is declared **nowhere** in Go (not
   a local, a package-level name of any file in the package, an import or a predeclared name)
   is taken as a C++ namespace: `std.vector` becomes `C.std.vector`. A local variable named `std`
   hides the namespace, as Go scoping says (tested in the spike). Only files that import at least
   one header do this; in other files such a name is an ordinary Go `undefined` error. We cannot
   simply write `import std "C"` because `go/types` rejects renaming `C` (`cannot rename import
   "C"`, found in the spike).
4. **Check the namespace names (namespace probe).** One small `-fsyntax-only` file with the
   file's includes and one line per candidate (`namespace gp_ns_0 = ::std;`). A line that doesn't
   compile means "not a namespace", and gocpp reports a normal `undefined: stdd` error at that
   spot. So typos are still caught before any C++ is generated. The spike checked this with
   clang++ and g++ (both found `std`, both rejected `stdd` and `fmtt`). The result is cached per
   include set; Phase 04 extends this probe into the full "kind probe".
5. **Keep conversions of opaque values typed.** `n := int(cstring.strlen(p))` would otherwise give
   `n` the type `invalid`, and every later mistake with `n` would go unreported. The conversion is
   rewritten to a hidden generic helper, `gp_as[int](C.strlen(p))`, which `go/types` types as
   `int` (verified during the audit). Codegen emits a normal C++ cast.
6. **Add `import "C"`** to files that use `C`, a header package or a namespace.

What the user wrote:

```go
import (
	"<raylib.h>"
	"<vector>"
)

var v = std.vector[int]{1, 2}
var white = raylib.RAYWHITE
```

What `go/types` sees (the headers go to `FileInfo.Includes`, the uses are rewritten):

```go
import "C"

var v = C.std.vector[int]{1, 2}
var white = C.RAYWHITE
```

`go/parser` only resolves names declared in the same file, so the loader first collects the
package-level names of **all** files of the package before step 3 runs. The parser's legacy
object resolution (`ast.Ident.Obj`) is deprecated upstream. Since we own the parser fork, we keep
it working. If it is ever removed, steps 2 and 3 use a small scope walk instead (about 150 lines).

### 2.6 Package loading and the importer

- A package is a directory of `.gp` files (same as Go). `_test.gp` files are only included by
  `gocpp test`. `//go:build` constraints are honored using `go/build/constraint`.
- Import path → directory: the stdlib (`fmt`, `strings`, …) is resolved from the embedded
  `stdlib/` tree (a tiny Go package inside `stdlib/` holds the `//go:embed` directive, because
  an embed pattern can't reach a parent folder). Module packages resolve from the module root, found by walking up to
  `gocpp.mod` (Phase 06 adds `require`/`replace`).
- The loader implements `types.ImporterFrom`. It loads and type-checks dependencies first,
  memoizes them and reports import cycles (`import cycle not allowed`).
- In this phase `stdlib/` contains **declaration stubs** for every standard package that the
  Phase 00 target programs import (`fmt`, `os`, `bufio`, `strings`, `strconv`, `errors`, `flag`,
  `sync`, `time`, `sort`, `math`, …), so all of them type-check now. Real implementations come in
  Phases 02, 05, 08 and 09. Importing a package that has no stub yet is a gate error
  ("package X is not available yet (Phase 09)").

### 2.7 go/types configuration

```go
func check(path string, fset *token.FileSet, files []*ast.File, info *types.Info) {
	conf := types.Config{
		GoVersion:   "go1.25",     // pin language version (loop-var semantics, range-over-int, ...)
		FakeImportC: true,         // C.* = opaque
		Importer:    loader,       // gocpp packages from source
		Sizes:       gocppSizes{}, // sizes of OUR runtime types (gp::ptr = 16 bytes, ...)
		Error:       diags.Collect,
	}
	// info records Types, Defs, Uses, Implicits, Selections, Scopes, Instances,
	// InitOrder and FileVersions; codegen needs all of them.
	pkg, err := conf.Check(path, fset, files, info)
	_, _ = pkg, err
}
```

`Sizes` matters because `unsafe.Sizeof` must match the real C++ layout. Phase 03 adds
`static_assert`s in the generated C++ to keep both sides in sync.

**Template arguments of opaque types.** When the generic base is opaque
(`std.vector[strings.Builder]`), `go/types` stops before looking at the arguments. Three things
go wrong (all seen in the audit): `strings` is reported "imported and not used", typos such as
`std.vector[Pointt]` are not reported, and codegen gets no types for the arguments. Fix (verified):
after `Check`, run `types.CheckExpr(fset, pkg, arg.Pos(), arg, info)` on every argument of an
opaque instantiation. That types `strings.Builder`, reports `undefined: Pointt`, and lets us drop
the false "imported and not used" errors (they are soft errors with the code `UnusedImport`; a
filter in front of the diagnostics collector reads the code with `diag.ErrorCode` and holds them
back until `CheckExpr` has run, so the errors keep the order they were found in).

### 2.8 The opaque pass

After type checking, the opaque pass labels every expression:

- **Typed**: `info.Types[e]` has a valid type, so codegen uses the exact gocpp → C++ mapping.
- **Opaque**: the type is missing or `invalid` *and* the expression comes from C/C++. That
  means it is rooted in a header package, a namespace or `C.`, or in an object declared with a
  type that contains a C type (`var xs []C.int` then `xs[0]`), or in a call of a function whose
  result type contains a C type (`f()` returning `*cstdio.FILE`), or in a variable initialized
  from any of these. Codegen uses `auto`, `gp::deref`, `gp::range` and friends (Phase 04).

It also records, for every object declared with a C/C++ type (`func f(w *SDL.SDL_Window)`,
struct fields, results, `var p *cstdio.FILE`), the **AST type expression**, because `go/types`
has no type for it and codegen must print `SDL_Window*` from the syntax.

Sanity rule: if `go/types` reported no hard errors (soft ones, like an unused variable, leave the
types complete), every invalid expression must be opaque. If not, that is an internal compiler
error with a clear message, never silently wrong C++.

Facts from the spike that this pass relies on:
- Untyped constants passed to C calls keep kind and value (`"x.txt"` → `untyped string`).
  Codegen emits raw C literals for them (`fopen("x.txt", "r")`).
- `x := lib.foo()` (a header package) gives `x` type `invalid`. Codegen emits `auto x = ::foo();`.
- `var total int = arr.size()` keeps `total` as `int`. Codegen emits `int64_t total = …`.

### 2.9 Unsupported-feature checker (phase gates)

A table-driven pass rejects features that are not implemented *yet*, with a message that names
the phase. That way users (and tests) never get broken C++. The table must cover **everything**
a later phase adds:

| Feature | Detected by | Allowed from |
|---|---|---|
| composite types, pointers, methods, closures, `defer` (incl. named results changed by a defer) | AST nodes / `types.Info` | Phase 03 |
| header imports, header packages, namespaces, `C.` types | import classification | Phase 04 |
| interfaces, type switches, `any` values, struct embedding, `recover` | `types.Info` / AST | Phase 05 |
| more than one *user* package (stdlib stubs are fine) | loader | Phase 06 |
| type parameters | `info.Instances`, `TypeParams` | Phase 07 |
| `go`, `chan`, `select` | AST nodes | Phase 08 |
| standard packages without an implementation yet | loader | Phase 09 |
| `F!(...)` must-calls, `func f!()` must functions, `macro`/`#Name(...)` macros (incl. `#body()`) | parser patches #4–#6 | Phase 10 |
| embedded C/C++ type with promoted access | selection on an opaque embedded field | Phase 12 |
| C macro as a compile-time constant (`[cstdio.BUFSIZ]byte`) | opaque in a constant context | Phase 12 |
| `reflect` | import | the subset from Phase 09 (divergence D6) |
| `//go:linkname`, `plugin` | directive / import | never in user code (`tools/portstd` turns the stdlib's linkname uses into native functions, Phase 09) |

Exemptions, so the early phases can run the target programs: the `fmt` stubs' `...any`
parameters and `error` results, and `panic(v)` with a basic value, are allowed from Phase 02.

The same table generates `docs/spec/feature-matrix.md`, so the docs always match the compiler.

### 2.10 Diagnostics

Go-style: `path/file.gp:12:5: message`, sorted by position, deduplicated, and at most 10 per
package unless `-e` is given (same as `go build`). Parse errors arrive as the fork's own
`scanner.ErrorList` type (`internal/syntax/scanner`), not stock `go/scanner`'s, so the CLI
(step 1.14) converts that type.

The details follow the go command, as checked against real `go build` 1.25.14 (step 1.13):
- **Order:** by file, line and column, where the files come in the order their first error was
  found (not by name), and errors at the same place keep the order they were found in.
- **Duplicates:** an error with the same file, line and message as the one just before it is
  dropped, and so is an exact duplicate after sorting.
- **Limit:** after the 10th error *found*, `<its position>: too many errors` (also with exactly
  10); `-e` shows all of them.
- **Columns:** gocpp always prints the real column. `go build` prints no column from column 255
  on (an 8-bit limit inside the compiler). That is one of the two known differences; the other is
  that the short name is chosen per file, so a file outside the package folder (a `//line`
  target, later a C++ header in a sub-folder) can be shown as `other/x.go` where `go build` shows
  `./other/x.go`.
- **Paths:** relative to the current folder when that is shorter (`./main.go`, `sub/a.go`,
  `../two/a.go`), otherwise absolute, so the loader gives the `FileSet` absolute file names.
- go/types' continuation lines ("other declaration of x", starting with a tab) stay attached to
  their error, as related information for the editor.

The messages are go/types' and go/parser's. They differ from the compiler's in a few places (parse
errors don't start with `syntax error:`, a few positions differ), so the output equals `go build`'s
only where both checkers agree. An optional source line with a caret is shown.
`go/types` messages are already high quality; gocpp-specific messages use the same tone.

### 2.11 `gocpp fmt` (gofmt for `.gp`)

Our parser → gocpp's import sorting (below) → **`go/printer` with gofmt's exact settings**. Our
AST is made of stock nodes (`std.map` is just an `Ident` named `map`), so no printer fork is needed
yet. `go/format.Node` itself can't be used: for any file with a grouped import it prints the file,
**parses it again with the stock `go/parser`** and sorts every import group, so it fails on
`std.map` (`format.Node internal error (… expected selector or type assertion, found 'map')`) and
reorders header imports (found by job 1.5). So `gocpp fmt` prints with
`printer.Config{Mode: printer.UseSpaces | printer.TabIndent | normalizeNumbers, Tabwidth: 8}`,
which is what `go/format` uses. `normalizeNumbers` is go/printer's unexported mode `1 << 30`
(`0XFF` → `0xFF`, `1E5` → `1e5`; found in the audit); a test pins it, so a Go update that changes
the value fails loudly. Flags are gofmt's: `-l`, `-w`, `-d`. (Phase 10 adds a small printer fork
for the `macro` keyword, `lazy` parameters and on-top macros; it keeps this normalization and gives
the mode a name.)

Verified on 2026-10-13: the stock printer prints `std.map[std.string, int]{}`,
`std.array[int, 5]` and `node.type()` correctly.

**Header imports keep the order you wrote.** gofmt's `ast.SortImports` sorts every import group
byte-wise. That's harmless for Go packages, but header imports become `#include` lines, and C/C++
include order can matter (`<winsock2.h>` before `<windows.h>`, `<GL/glew.h>` before `<GL/gl.h>`).
Plain gofmt would reorder them (seen on the spike files). So `gocpp fmt` uses a small copy of
`ast.SortImports` that sorts only groups without header or namespace imports, and leaves a group
that contains one exactly as written. The style rule in the spec is to **put header imports in their own
blank-line-separated group** after the gocpp imports, the same way Go groups stdlib and
third-party imports.

### 2.12 CLI in this phase

```
gocpp version
gocpp parse -dump file.gp      # print the AST (debugging)
gocpp check [packages]         # parse + type-check, print errors, exit 1 on error
gocpp check -types file.gp     # print every expression with its type / "opaque" (like the spike)
gocpp fmt [-l] [-w] [-d] [files or folders]  # like gofmt; no paths = stdin
```

Like `go build`, `gocpp check` prints a `# <import path>` line before each failing package's
errors (`# command-line-arguments` for files named on the command line), shows at most 10 per
package, writes everything to stderr and exits with status 1. A package with syntax errors is not
type-checked. `-e` shows all errors and also parses with `parser.AllErrors`, keeping only the
first syntax error of each line (the compiler does that even with `-e`, while go/parser with
`AllErrors` can report several on one line).

## 3. Steps

1. **1.1 Fork** `go/scanner` + `go/parser` from the dev shell's Go (1.25.14 now) into `internal/syntax` (plus
   `go/printer/nodes.go` for the performance test); add `FORK.md`, Go's `LICENSE` and the
   re-fork script `update.sh`; the upstream tests pass, with only two test-path fixes
   (`FORK.md`).
2. **1.2 Patch 1** (keywords after `.`, same line only) + tests (`std.map`, `x.type()`,
   `posix.select`, `x.(type)` still works, semicolon after `.range`, upstream
   `TestIncompleteSelection` still passes).
3. **1.3 Patch 2** (constant type arguments) + tests (`std.array[int, 5]` in type,
   composite-literal and call positions; nested `std.array[std.array[int, 2], 3]`).
4. **1.4 Patch 3** (dotted type names) + tests (`var d std.chrono.seconds`, parameters, slices,
   struct fields, `std.vector[int].iterator`).
5. **1.5 Round-trip test**: parse → print → parse again gives an equal AST over the whole Go
   standard library source (a free fuzz corpus).
6. **1.6 Import classifier** + `FileInfo` + tests for every row of the table in §2.3: header
   package names (renamed, `_`, keyword / predeclared / taken names get none, the hint),
   namespace renames, and the reserved-`C` error.
7. **1.7 Directive parser** for `//gocpp:` lines with validation and suggestions.
8. **1.8 AST preprocessing** (§2.5): header-package and namespace rewrites with positions
   kept, the package-wide name set (`load.PackageNames`, from step 1.6), the namespace probe
   (cached), `gp_as[T]` conversions, shadowing tests.
9. **1.9 Package loader + importer** (§2.6): dir scan, `_test.gp`, build constraints, cycles,
   stubs for every package the target programs import. It joins each file's `FileInfo` with its
   `//gocpp:` directives. For the header names it needs the real names of the imported gocpp
   packages: it gets their paths with `load.PackagePaths`, reads those packages' package
   clauses, then calls `load.ClassifyImports` with a `packageName` function (without one, step
   1.6 guesses names from the path, like goimports). Steps 1.6 and 1.7 each check header paths
   (with small differences, e.g. `< lua.h>`); the loader makes them share one check.
10. **1.10 go/types integration** (§2.7) including a `gocppSizes` stub and the `CheckExpr` pass
    over opaque template arguments.
11. **1.11 Opaque pass** (§2.8) with the widened "comes from C/C++" rule, the sanity rule and
    recorded type expressions.
12. **1.12 Unsupported-feature checker** (§2.9) with the full table + generated feature matrix.
13. **1.13 Diagnostics** package (§2.10).
14. **1.14 CLI**: `version`, `parse -dump`, `check`, `check -types` (`fmt` comes with step 1.16).
15. **1.15 Error tests** (`gocpp check` prints warnings as `file:line:col: warning: msg`, for
    example for a comment that looks like a broken `//gocpp:` directive; the errors runner gets a
    `// WARNING "regexp"` mark for them, next to `// ERROR`) in `tests/errors/` for gocpp-specific cases (bad import shapes, unknown
    directives, phase-gated features, shadowed namespaces, a local named `C`, typos in namespace
    names, an unnamed header used by its file name, typos inside opaque template arguments) and a
    "no false errors" corpus of interop-heavy files.
16. **1.16 `gocpp fmt`** (§2.11) with `-l -w -d`, tested three ways: on the Go stdlib its output
    equals real `gofmt`'s (gofmt changes 4 files there itself, in spacing, so "unchanged" is the
    wrong test); deliberately badly formatted inputs (including number literals like `0XFF`)
    produce exactly what the real `gofmt` produces; and a group of header imports keeps its order.
    The round-trip test (step 1.5) then prints the same way as `gocpp fmt`. Like gofmt with a syntax
    error, `gocpp fmt` leaves a file unchanged and prints the errors when its `//gocpp:` directives
    have errors: go/printer would turn a broken directive in a doc comment (`//gocpp:Link m`) into
    an ordinary comment (`// gocpp:Link m`), and the library would silently not be linked (found
    in the review of job 1.7).

## 4. Deliverables

- `internal/syntax` (fork + 3 patches + tests), `internal/load`, `internal/check`, `internal/diag`
- `gocpp check` working on single and multi-package programs, `gocpp fmt`
- `tests/errors/*` and an interop "no false errors" corpus
- `docs/spec/feature-matrix.md` (generated)

## 5. Exit criteria

- All upstream `go/scanner`/`go/parser` tests pass on the fork; the round-trip test passes over
  the Go stdlib.
- All Phase 00 target programs pass `gocpp check` (the ones for later phases get only the
  expected "not supported yet (Phase NN)" gate errors).
- The interop corpus (std containers, SDL, sqlite, filesystem, header packages, namespaces,
  shadowing, dotted type names, opaque template arguments) has 0 false errors, and every seeded
  mistake in it is reported at the right line (namespace typos included).
- `gocpp check -types` output is reviewed by hand on 3 files and looks right.
- `gocpp fmt` output equals real `gofmt` output on the Go stdlib and on badly formatted inputs,
  and header import groups keep their order.

## 6. Risks

| Risk | Mitigation |
|---|---|
| The `FakeImportC` docs warn "effects may be unpredictable due to follow-on errors" | Big interop corpus in tests; sanity rule in the opaque pass; `gp_as` and `CheckExpr` close the known holes |
| The fork drifts from upstream | Small patches (3 now, 6 after Phase 10) kept as patch files with a re-fork script |
| Embedded C++ types and C macros as constants don't type-check | Phase-gated with clear errors; fixed in Phase 12; workaround: name the field (`mv.vector.push_back(1)`) |
| Legacy `ast.Object` resolution is removed upstream | We own the parser fork; fallback is a small scope walk |
| The namespace probe needs a C++ compiler run | Only for files with header imports; cached per include set; the editor runs it in the background |
