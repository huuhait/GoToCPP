# The parser fork

This folder is gocpp's copy (a "fork") of Go's `go/scanner`, `go/parser` and `go/printer`. gocpp
needs a few small changes in them for C++ names and for the syntax of Phase 10 (`!`, macros): the
patches in §4. The fork still makes stock `go/ast` nodes, so the stock `go/types` can check them.

| File | What it is |
|---|---|
| `scanner/`, `parser/`, `printer/` | the fork. `update.sh` makes these folders |
| `gpast/` | gocpp's own package, not a fork: how the syntax of Phase 10 looks in `go/ast` nodes (§4) |
| `LICENSE` | Go's license (§1) |
| `local.patch` | the local changes to the upstream tests (§3) |
| `patches/` | the patches for gocpp's language, one file per patch. §4 says which ones are in |
| `update.sh` | the script that makes the fork again (§6) |
| `staticcheck.conf` | turns off two staticcheck checks for the fork (§1) |
| `doc.go`, `FORK.md` | the package doc and this file |
| `roundtrip_test.go` | the round-trip test over the Go standard library (§5). `update.sh` doesn't change it |

## 1. Upstream

The fork is a copy of **go1.25.14**, the Go of the dev shell (`nix develop`). `update.sh`
copies it from `$(go env GOROOT)/src`:

| Upstream, in `$GOROOT/src` | Here, in `internal/syntax` |
|---|---|
| `go/scanner/`: every file, with the tests | `scanner/` |
| `go/parser/`: every file, with the tests and all of `testdata/` | `parser/` |
| `go/printer/`: every file, with the tests and all of `testdata/` | `printer/` |

- `parser/performance_test.go` reads `../printer/nodes.go` when the parser tests start, and its
  benchmarks parse it, as upstream does with `../printer/nodes.go` of `$GOROOT/src/go/`.
- `LICENSE` is Go's license (BSD-3-Clause). Every copied file says that its license "can be
  found in the LICENSE file". Nix's Go has no LICENSE file, so this one comes from Go's
  official source archive, <https://go.dev/dl/go1.25.14.src.tar.gz>. Its SHA-256 is the one
  that <https://go.dev/dl/?mode=json&include=all> lists, and its `go/scanner`, `go/parser` and
  `go/printer` are the same as the dev shell's.
- The copyright headers of the copied files stay as they are.
- `go/ast`, `go/token`, `go/build/constraint` and `go/doc/comment` are not copied. The fork uses
  the stock ones.
- `staticcheck.conf` turns off two checks for the fork, because they report upstream code,
  which stays as it is: SA1019 (`parser/resolver.go` uses the deprecated `ast.Scope` and
  `ast.Object`, and gocpp keeps this old name resolution on purpose; `roundtrip_test.go`,
  which compares that resolution, uses them too) and S1005 (`for i, _ := range list` in
  `parser/parser.go`). All other checks still run.

## 2. The import paths

In the fork, the three packages import each other by gocpp's paths:

| Upstream | Here |
|---|---|
| `"go/scanner"` | `"github.com/huuhait/gocpp/internal/syntax/scanner"` |
| `"go/parser"` | `"github.com/huuhait/gocpp/internal/syntax/parser"` |
| `"go/printer"` | `"github.com/huuhait/gocpp/internal/syntax/printer"` |

`update.sh` changes these three quoted paths only in the import declarations (an `import (`
block or an `import` line) of the Go files of `scanner/`, `parser/` and `printer/`. Go allows imports
only before all other declarations, so in each file it stops at the first `func`, `var`,
`const` or `type` line: a Go file that a test has in a string stays as it is. In go1.25.14,
the import paths are in 8 files (the tests of `printer/` import the parser and the printer). It
doesn't change the files in `testdata/`: test inputs stay as they are. Then it runs gofmt on the
changed files. gofmt sorts each import
block, so in `parser/parser.go` and `parser/resolver_test.go` the new import line moves up.
After that, `go list` checks that no Go file of the fork imports the stock packages. The check
runs before the patches (§6): the test files of the patches import the stock packages on purpose,
to compare the fork with them (`parser/gocpp_typeargs_test.go` and `parser/gocpp_dottypes_test.go`
import `go/parser` and `go/scanner`; the tests of patches 4 to 6 in `parser/` import the stock
`go/parser`, and `parser/gocpp_must_test.go` the stock `go/printer` too).

Two things to know:

- A parse error from the fork has the type `scanner.ErrorList` of the fork's `scanner`, not
  of the stock `go/scanner`. Code that checks the type of a parse error must import the fork's
  `scanner`.
- The stock `go/scanner` is still in the build, because the stock `go/ast` imports it. The
  fork's own code doesn't use it.

## 3. Local changes (tests only)

Two test files read files by a path that works only inside `$GOROOT/src/go/`, and one imports a
package of Go's `internal`. These changes fix that, and nothing else. They are in `local.patch`,
and each one has a `// gocpp:` comment in the code.

| File | Change | Why |
|---|---|---|
| `scanner/scanner_test.go` | `BenchmarkScanFiles` reads Go's own sources from `$(go env GOROOT)/src`, not from `../..` | here, `../..` is `internal/`. `go test` puts `$GOROOT/bin` first in `PATH`, so `go env GOROOT` gives the Go that runs the test. `runtime.GOROOT()` would be shorter, but it has been deprecated since Go 1.24 |
| `printer/printer_test.go` | `checkEqual` reports the first different line, and does not use `internal/diff` | a package outside Go's standard library can't import `internal/diff`. The test only needs a message for a difference |

## 4. Patches

The patches are gocpp's changes to the language. Each one is a patch file in `patches/`, and
`update.sh` applies them in name order: `0001-….patch`, `0002-….patch`, and so on.

The patches of the plan
([phase 01 README](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md) §2.2) are in this
table. When a patch is done, its row names its file:

| # | Patch | Step |
|---|---|---|
| 1 | A Go keyword after `.` on the same line is a name: `std.map`, `node.type()`. The scanner returns it as `token.IDENT`, also with space or a `/* */` comment after the `.` on that line. On the next line, it stays a keyword. It is in: [`0001-keyword-after-dot.patch`](patches/0001-keyword-after-dot.patch) | 1.2 |
| 2 | Type arguments may be constants: `std.array[int, 5]`. In the three places where upstream called `parseType` for a type argument (in a type, and after the first argument of an index expression or of an instance embedded in an interface), `parseTypeArg` parses a type or an expression with `parseRhs`. Upstream already parsed those first arguments, and the arguments of an embedded field or an unnamed parameter `T[...]`, as expressions. For a Go generic, go/types still reports `5 is not a type`. A token that can't start a type argument keeps upstream's error and recovery: `parseTypeArg` calls `parseRhs` only for a token that can start an expression. In valid Go code, one thing changes: nested type arguments in a type or after a comma reach "exceeded max nesting depth" at 49,999 levels, not 99,999, as upstream's own `T[«», int]{}` already did. It is in: [`0002-constant-type-args.patch`](patches/0002-constant-type-args.patch) | 1.3 |
| 3 | Type names may have more than one dot: a type in a nested namespace, `std.chrono.seconds`, and a member type after a type instance, `std.vector[int].iterator`, in every place where Go has a type. `parseTypeName` goes on after its first `.Name`, and so does each place that parses a type instance: `parseTypeInstance`, and an instance `T[...]` embedded in an interface, embedded in a struct or as an unnamed parameter. `parseDottedName` parses the `.Name` parts. The AST is the one of the same expression: nested `SelectorExpr`s. At a `.` that no name follows (`nameAfterDot` looks at the next token with a copy of the scanner), or after a part that the error recovery made up, the type name stops at the `.`, so the error and the recovery stay upstream's. A type instance after a member type, as in `a[T].b[U]`, is not in the patch. Code that the stock `go/parser` accepts parses the same, with one exception that is never valid Go: in an expression, a selector right after a slice, array, map, chan or func type whose last part is a qualified type name or a type instance, as in `[]pkg.T.M{}`, was a method of that type (it has none), and now it is a part of the element type. It is in: [`0003-dotted-type-names.patch`](patches/0003-dotted-type-names.patch) | 1.4 |

Phase 10 (step 10.1) adds the next four. They add the syntax of must calls and macros
([language spec](../../docs/spec/language.md#syntax-the-exact-rules) has the rules), in a mode
that is off by default (`parser.Phase10Syntax`, `scanner.Phase10Syntax`): without it, the
scanner and the parser behave exactly as upstream, and each new form is the syntax error that
upstream reports (the tests check that against the stock `go/parser`). Every gocpp tool turns the
mode on: the loader (so `check`, `build`, `run`, `test`, `vet`, `expand`), `gocpp fmt`, `doc`, `parse` and the
language server. Since step 10.12 ([ADR-095](../../docs/adr/095-must-and-macros-are-always-available.md)) there is
no phase gate for the syntax: before, the compiler turned the mode on only when its gates allowed Phase 10 and
explained the errors with "must calls are not supported yet (Phase 10)". The option stays in the fork because the
parser's own tests need the stock behavior and Go code must parse as upstream's parser does.

| # | Patch | Step |
|---|---|---|
| 4 | A name directly followed by `!(` or `![` is one name, `F!`, for must calls and must functions: `os.ReadFile!(p)`, `parse![int](s)`, `func f!() T`, `func (s *S) Start!()`. A keyword is not a name (`if!(a)` stays `if`, `!`, `(a)`), and neither is a keyword after a `.` (patch 1). A name with a `!` is accepted in three places only: the function of a call (also after a `.`, and with type arguments) and the name of a `func`; `parseIdent` reports an error anywhere else (a variable, a parameter, a type, a label: `cannot use f! here: …`), and after the parse each name with a `!` that is not the function of a call is an error (`F! must be called: write F!(...)`). The scanner makes the name (`scanner.Phase10Syntax`), the parser's `parseName` and `checkMarkedNames` (`parser/gocpp_phase10.go`) check it. The AST is an `*ast.Ident` whose `Name` ends with `!` ([`gpast`](gpast/gpast.go)). It is in: [`0004-must-names.patch`](patches/0004-must-names.patch) | 10.1 |
| 5 | A `#` directly followed by a letter is one name, `#Name`, for macro uses: `#util.Dbg(x)`, `#Assert(a)`, `#func()`, `#body()` (a keyword after the `#` is part of the name). Any other `#` is the illegal character of Go. The same rules as for `!` apply: a `#` name starts a call (`#y` alone is `#y must be called`), and may not be a declared name, a type, a label or a name after a `.`. The AST is an `*ast.Ident` whose `Name` starts with `#`. It is in: [`0005-macro-names.patch`](patches/0005-macro-names.patch) | 10.1 |
| 6 | Macro declarations, `lazy` parameters and lines of macros on top of a declaration. At package level (where a name can't start a declaration), `macro Name[T](params) results { body }` is parsed by `parseMacroDecl` as a `*ast.FuncDecl` whose name is `#Name`, with `FuncType.Func` at the word `macro` (the rest of `parseFuncDecl` is shared, as `parseFuncDeclRest`). Inside the parameter list of a macro (and nowhere else), `name lazy T` is a parameter whose type is a `*ast.UnaryExpr` with the operator `token.ILLEGAL` (`gpast.Lazy`); `lazy` can't be the name of such a parameter. At package level, a line that starts with a `#` name is a line of macros: a call, stored as a `*ast.GenDecl` with the token `token.ILLEGAL` in `File.Decls` right before its declaration, which must be a `func` (not a macro) or a `type`; the doc comment above the lines becomes the `Doc` of the declaration. Inside a function, `macro` stays a name. The patch also tightens the called-name check of patches 4 and 5 (found in review): a name with a `!` must be the called function (`F![int].x(1)` and `a.F![int].x(1)` are errors), and a name with a `#` the start of the called selector (`#a[int].b(1)` is an error); its tests are in the test files of patches 4 and 5. It is in: [`0006-macro-decls.patch`](patches/0006-macro-decls.patch) | 10.1 |
| 7 | The printer (`printer/`): `macro` and the name without its `#` for a macro declaration, `lazy T` for a lazy type, a line of macros as its call, no empty line between the lines of macros and their declaration, and a new column section after the last line of macros (its trailing comment doesn't share a column with a one-line func). Names with `!` and `#` print as they are, as the stock printer does. For Go code the printer is upstream's. It is in: [`0007-phase10-printer.patch`](patches/0007-phase10-printer.patch) | 10.1 |
| 8 | The mode `SkipFuncBodies`: the parser scans the body of a function (not `init`, not a macro, not a generic function: go/types reports one without a body) to its matching `}` and makes no nodes for it, so `Body` is nil and the positions after it are those of a full parse. The language server's focused check uses it for the files that it does not check ([ADR-115](../../docs/adr/115-lsp-focused-check-after-an-edit-inside-a-function-body.md)). It is in: [`0008-skip-func-bodies.patch`](patches/0008-skip-func-bodies.patch) | 11.14 |

The tree of each form is documented in [`gpast`](gpast/gpast.go), which is not a fork and not a
patch: the parser makes these nodes, the printer prints them, and the rewrites of Phase 10 read them.
The plan's text says that a macro is "marked in a side table"; there is none, because the printer
and the rewrites would each need the table next to the tree, and a node that a pass copies or moves
would lose its entry. The marks are in the nodes instead.

What that costs (also in the package comment of `gpast`, which has `Leftovers` for it):

- The stock `go/types` does not catch a form that a pass forgot to rewrite: it reports no
  "undefined" for a name with a `!` or a `#` (`isValidName`), so `F!(x)` and `#M(x)` type-check
  silently; a lazy parameter gives "ILLEGALstring is not a type", an on-top line "invalid token
  ILLEGAL". The passes of 10.2 and later must call `gpast.Leftovers(file)` before `go/types` and
  fail when it finds a node.
- `ast.IsExported("#Assert")` is false, so `go/doc` and `ast.FilterFile` take every macro as
  unexported: use `gpast.MacroBase` for the name.
- `macro M` and `func M` (and `f!` and `f`) are different names in the tree: the passes must report
  the redeclaration.
- The name of a macro declaration is `#M` in the tree, but `M` in the source, so `Name.End()` is one
  too far: use `gpast.DeclNameEnd(decl)`. The names of must calls, must functions and macro uses
  have the `!` or `#` in the source, and their positions are right.

## 5. Rules

- A patch stays small. It changes only what it needs.
- Every patch is a patch file in `patches/`, and the table in §4 says what it does.
- Every other change to an upstream file is in this file too, with the reason.
- Don't change the files in `scanner/`, `parser/` or `printer/` without a patch file.
  `update.sh` deletes these folders and makes them again. It stops when they have changes
  that are not committed; with `-f`, it saves a copy of them first (§6).
- `go/ast`, `go/token` and `go/build/constraint` stay stock.
- All upstream tests keep passing, `TestIncompleteSelection` too.
- `roundtrip_test.go` keeps passing too: for every Go file of the standard library, the fork gives
  the same AST as the stock `go/parser`, and formatting it like `gocpp fmt` does (`format.Source`
  in `internal/format`) and parsing it again gives the same AST.

## 6. Re-fork with update.sh

Run it when the dev shell gets a new Go, or to check that the fork is still right:

    nix develop -c internal/syntax/update.sh

It works from any folder and uses the Go in `PATH`. To use other tools, set `GO` (the `go`
command, for example `GO=/path/to/go`) or `GOFMT` (the default is the `gofmt` of that Go).
It:

1. stops if `scanner`, `parser`, `printer` or `LICENSE` here is a
   symbolic link. Through a link, it would change files outside the fork;
2. prints the Go version and the GOROOT that it uses;
3. stops if `scanner/`, `parser/`, `printer/` or `LICENSE` have changes that are not
   committed (`git status` shows them), because it would delete them. gocpp must be a git
   repository of its own (a clone or a worktree): without one, it can't see changes, and it
   stops too. With `-f` (`update.sh -f`), it goes on, but first it saves a copy of these
   files in a new temporary folder (`mktemp -d`) and prints its name;
4. deletes `scanner/`, `parser/` and `printer/`, and copies them again from
   `$(go env GOROOT)/src`. It also copies `$GOROOT/LICENSE`, if there is one;
5. rewrites the import paths (§2), and checks with `go list` that no stock import is left;
6. applies `local.patch` (§3), then every `patches/*.patch` (§4) in name order, with
   `git apply`. A patch may only change, add or delete files in `scanner/`, `parser/` and `printer/`. It
   may not rename or copy a file, or make a symbolic link (make it with
   `git diff --no-renames`);
7. runs `gofmt -l` on the Go files of `scanner/`, `parser/` and `printer/` (not on `testdata/`), runs
   `go vet` and `go test` on `./internal/syntax/...`, and runs each benchmark once.

It writes nothing outside `scanner/`, `parser/`, `printer/` and `LICENSE` (and the
copy that `-f` makes). With the same Go, a second run changes nothing, so `git status` stays
empty.

After a re-fork to a new Go version:

1. read the changes: `git diff -- internal/syntax`;
2. write the new version in §1 of this file and in `doc.go`;
3. run `make ci` and `make staticcheck`.

A note for a re-fork to Go 1.26: there, `go/scanner` and `go/parser` import
`go/internal/scannerhooks`. A package outside Go's standard library can't import it, so
`update.sh` stops with `use of internal package go/internal/scannerhooks not allowed`. A
re-fork to Go 1.26 must copy that package too (for example to
`internal/syntax/internal/scannerhooks`) and rewrite its import path as well. Go 1.27 doesn't
import it any more, so this is only for Go 1.26.

## 7. Make a new patch

1. Start from a clean fork: run `update.sh`. `git status -- internal/syntax` must show no
   changes.
2. Change the files in `scanner/`, `parser/` or `printer/`, and add the tests. For a new file, run
   `git add -N <file>`, so that `git diff` shows it.
3. Write the patch file. Its name is `NNNN-name.patch`. For `NNNN`, pick the next free number:
   one more than the highest number in `patches/`. Don't use a number twice. For `name`, use a
   few words that say what the patch does, like `keyword-after-dot`:

       mkdir -p internal/syntax/patches
       git diff --no-renames -- internal/syntax/scanner internal/syntax/parser internal/syntax/printer >internal/syntax/patches/NNNN-name.patch

4. Check it: run `update.sh -f` (your changes are not committed yet, and they are in the
   patch now). Then the same `git diff` must give the same patch:

       git diff --no-renames -- internal/syntax/scanner internal/syntax/parser internal/syntax/printer | cmp - internal/syntax/patches/NNNN-name.patch

5. Add the patch to the table in §4.

## 8. When a patch doesn't apply

On a new Go version, a patch (or `local.patch`) may not apply any more, because the upstream
code around it changed. Then `update.sh` stops at that patch. The files have the new upstream
and the patches before that one. Then:

1. Save this state in git's index: `git add internal/syntax`.
2. Make the change of the patch by hand. The old patch file shows what to change. For a new
   file, run `git add -N <file>`, as in §7.
3. Write the patch file again. Here `NNNN-name.patch` is the patch that doesn't apply: keep
   its name, so that it replaces the old file. `git diff` now shows only your change:

       git diff --no-renames -- internal/syntax/scanner internal/syntax/parser internal/syntax/printer >internal/syntax/patches/NNNN-name.patch

   If the patch that doesn't apply is `local.patch`, write it to `internal/syntax/local.patch`:
   it is not in `patches/`.

4. Run `update.sh -f` again. Do the same for the next patch that doesn't apply.

The same steps change a patch on purpose, for example to add a test to it. Start from a fork
that `update.sh` made, and undo that patch and the patches after it, the last one first:
`git apply -R internal/syntax/patches/NNNN-name.patch`. Then do step 1, apply the patch again
with `git apply`, and run `git add -N` on each file that the patch adds (for patch 2,
`internal/syntax/parser/gocpp_typeargs_test.go`). Step 1 saved the patch's new files as
deleted, so without `git add -N`, `git diff` would leave them out of the patch. Then change it
(step 2). In step 3, `git diff` shows the whole patch with your change, not only your change.
