# The parser fork

This folder is gocpp's copy (a "fork") of Go's `go/scanner` and `go/parser`. gocpp needs a few
small changes in them for C++ names: the patches in §4. The fork still makes stock `go/ast`
nodes, so the stock `go/types` can check them.

| File | What it is |
|---|---|
| `scanner/`, `parser/` | the fork. `update.sh` makes these folders |
| `testdata/printer/nodes.go` | a test input from upstream (§1). `update.sh` copies it |
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
| `go/printer/nodes.go` | `testdata/printer/nodes.go` |

- `testdata/printer/nodes.go` is only a test input. `parser/performance_test.go` reads it when
  the parser tests start, and its benchmarks parse it. It is in a `testdata` folder, because
  `go build`, `go vet` and `make lint` skip those folders. A lone `nodes.go` in a normal
  folder would be a broken Go package.
- `LICENSE` is Go's license (BSD-3-Clause). Every copied file says that its license "can be
  found in the LICENSE file". Nix's Go has no LICENSE file, so this one comes from Go's
  official source archive, <https://go.dev/dl/go1.25.14.src.tar.gz>. Its SHA-256 is the one
  that <https://go.dev/dl/?mode=json&include=all> lists, and its `go/scanner` and `go/parser`
  are the same as the dev shell's.
- The copyright headers of the copied files stay as they are.
- `go/ast`, `go/token` and `go/build/constraint` are not copied. The fork uses the stock ones.
- `staticcheck.conf` turns off two checks for the fork, because they report upstream code,
  which stays as it is: SA1019 (`parser/resolver.go` uses the deprecated `ast.Scope` and
  `ast.Object`, and gocpp keeps this old name resolution on purpose; `roundtrip_test.go`,
  which compares that resolution, uses them too) and S1005 (`for i, _ := range list` in
  `parser/parser.go`). All other checks still run.

## 2. The import paths

In the fork, the two packages import each other by gocpp's paths:

| Upstream | Here |
|---|---|
| `"go/scanner"` | `"github.com/huuhait/gocpp/internal/syntax/scanner"` |
| `"go/parser"` | `"github.com/huuhait/gocpp/internal/syntax/parser"` |

`update.sh` changes these two quoted paths only in the import declarations (an `import (`
block or an `import` line) of the Go files of `scanner/` and `parser/`. Go allows imports
only before all other declarations, so in each file it stops at the first `func`, `var`,
`const` or `type` line: a Go file that a test has in a string stays as it is. In go1.25.14,
the import paths are on 5 lines in 5 files. It doesn't change the files in `testdata/`: test
inputs stay as they are. Then it runs gofmt on the changed files. gofmt sorts each import
block, so in `parser/parser.go` and `parser/resolver_test.go` the new import line moves up.
After that, `go list` checks that no Go file of the fork imports the stock packages. The check
runs before the patches (§6): the test file of patch 2, `parser/gocpp_typeargs_test.go`, imports
the stock `go/parser` and `go/scanner` on purpose, to compare the fork with them.

Two things to know:

- A parse error from the fork has the type `scanner.ErrorList` of the fork's `scanner`, not
  of the stock `go/scanner`. Code that checks the type of a parse error must import the fork's
  `scanner`.
- The stock `go/scanner` is still in the build, because the stock `go/ast` imports it. The
  fork's own code doesn't use it.

## 3. Local changes (tests only)

Two test files read files by a path that works only inside `$GOROOT/src/go/`. These changes
fix the paths, and nothing else. They are in `local.patch`, and each one has a `// gocpp:`
comment in the code.

| File | Change | Why |
|---|---|---|
| `parser/performance_test.go` | reads `../testdata/printer/nodes.go`, not `../printer/nodes.go` | the package reads this file when the tests start. With a wrong path, every parser test fails |
| `scanner/scanner_test.go` | `BenchmarkScanFiles` reads Go's own sources from `$(go env GOROOT)/src`, not from `../..` | here, `../..` is `internal/`. `go test` puts `$GOROOT/bin` first in `PATH`, so `go env GOROOT` gives the Go that runs the test. `runtime.GOROOT()` would be shorter, but it has been deprecated since Go 1.24 |

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
| 3 | Type names may have more than one dot: `std.chrono.seconds` | 1.4 |

Phase 10 adds three more patches: `name!(` is one name (must calls), `#Name` is one name
(macros), and `macro` declarations with `lazy` parameters. It also adds a small fork of
`go/printer`.

## 5. Rules

- A patch stays small. It changes only what it needs.
- Every patch is a patch file in `patches/`, and the table in §4 says what it does.
- Every other change to an upstream file is in this file too, with the reason.
- Don't change the files in `scanner/`, `parser/` or `testdata/printer/` without a patch file.
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

1. stops if `scanner`, `parser`, `testdata`, `testdata/printer` or `LICENSE` here is a
   symbolic link. Through a link, it would change files outside the fork;
2. prints the Go version and the GOROOT that it uses;
3. stops if `scanner/`, `parser/`, `testdata/printer/` or `LICENSE` have changes that are not
   committed (`git status` shows them), because it would delete them. gocpp must be a git
   repository of its own (a clone or a worktree): without one, it can't see changes, and it
   stops too. With `-f` (`update.sh -f`), it goes on, but first it saves a copy of these
   files in a new temporary folder (`mktemp -d`) and prints its name;
4. deletes `scanner/`, `parser/` and `testdata/printer/`, and copies them again from
   `$(go env GOROOT)/src`. It also copies `$GOROOT/LICENSE`, if there is one;
5. rewrites the import paths (§2), and checks with `go list` that no stock import is left;
6. applies `local.patch` (§3), then every `patches/*.patch` (§4) in name order, with
   `git apply`. A patch may only change, add or delete files in `scanner/` and `parser/`. It
   may not rename or copy a file, or make a symbolic link (make it with
   `git diff --no-renames`);
7. runs `gofmt -l` on the Go files of `scanner/` and `parser/` (not on `testdata/`), runs
   `go vet` and `go test` on `./internal/syntax/...`, and runs each benchmark once.

It writes nothing outside `scanner/`, `parser/`, `testdata/printer/` and `LICENSE` (and the
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
2. Change the files in `scanner/` or `parser/`, and add the tests. For a new file, run
   `git add -N <file>`, so that `git diff` shows it.
3. Write the patch file. Its name is `NNNN-name.patch`. For `NNNN`, pick the next free number:
   one more than the highest number in `patches/`. A number that is already used would
   overwrite that patch. For `name`, use a few words that say what the patch does, like
   `keyword-after-dot`:

       mkdir -p internal/syntax/patches
       git diff --no-renames -- internal/syntax/scanner internal/syntax/parser >internal/syntax/patches/NNNN-name.patch

4. Check it: run `update.sh -f` (your changes are not committed yet, and they are in the
   patch now). Then the same `git diff` must give the same patch:

       git diff --no-renames -- internal/syntax/scanner internal/syntax/parser | cmp - internal/syntax/patches/NNNN-name.patch

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

       git diff --no-renames -- internal/syntax/scanner internal/syntax/parser >internal/syntax/patches/NNNN-name.patch

4. Run `update.sh -f` again. Do the same for the next patch that doesn't apply.
