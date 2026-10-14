# Phase 01: Front-end · STATE

**Status:** `[~]` in progress (step 1.1 started while Phase 00's last jobs are in review; it needs nothing from them)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike: stock go/parser + go/types (FakeImportC) on interop-heavy `.gp` files (2026-10-13, 0 false errors)
- [x] Spike re-run with the final import model: header packages, namespaces as packages, namespace probe with clang++ and g++ (typos `stdd`, `fmtt` reported) (2026-10-13)
- [x] Check: stock go/printer + `ast.SortImports` print gocpp syntax correctly (2026-10-13)
- [x] 1.1 Fork go/scanner + go/parser (the dev shell's Go, 1.25.14 now) into `internal/syntax`
  - [x] copy sources + license headers + `FORK.md` (+ `go/printer/nodes.go` for the perf test)
  - [x] upstream tests pass (only two test-path fixes, listed in `FORK.md`)
  - [x] `update.sh` re-fork script
- [x] 1.2 Patch 1: Go keywords allowed after `.` (same line only)
  - [x] scanner change
  - [x] tests: `std.map`, `x.type()`, `posix.select`, `x.(type)` unaffected, semicolon after `.range`, `TestIncompleteSelection`
- [~] 1.3 Patch 2: constant expressions in type-argument lists
  - [ ] parser change (`parseRhs` for type arguments)
  - [ ] tests: type position, composite literal, call, nested
- [ ] 1.4 Patch 3: dotted type names (`std.chrono.seconds`, `std.filesystem.path`, `std.vector[int].iterator`)
- [x] 1.5 Round-trip test over the Go stdlib (parse → print → parse)
- [x] 1.6 Import classifier + `FileInfo` (header package names and their rules, namespace renames) + reserved-`C` error
- [~] 1.7 `//gocpp:` directive parser + validation + suggestions
- [ ] 1.8 AST preprocessing
  - [ ] drop header imports; `raylib.X` → `C.X`; undeclared `ns.X` → `C.ns.X` (package-wide name set); inject `import "C"` only where used
  - [ ] namespace probe (one `-fsyntax-only` file, cached per include set) → `undefined: x` for non-namespaces
  - [ ] `T(opaque)` → `gp_as[T](…)`
  - [ ] position preservation + shadowing tests
- [ ] 1.9 Package loader + importer
  - [ ] dir scan, `_test.gp`, `//go:build`
  - [ ] import cycles
  - [ ] stubs for every stdlib package the target programs import
- [ ] 1.10 go/types integration (`GoVersion`, `FakeImportC`, `Sizes` stub, full `Info`) + `CheckExpr` over opaque template arguments
- [ ] 1.11 Opaque pass (widened "comes from C/C++" rule, recorded C type expressions, sanity rule)
- [ ] 1.12 Unsupported-feature checker with the full gate table + generated `feature-matrix.md`
- [x] 1.13 Diagnostics package (format, sort, dedupe, limit, caret)
- [ ] 1.14 CLI: `version`, `parse -dump`, `check`, `check -types`, `fmt`
- [ ] 1.15 Error tests + "no false errors" interop corpus
- [ ] 1.16 `gocpp fmt` via `go/printer` with gofmt's settings (`go/format.Node` re-parses with the stock parser) (`-l -w -d`); header imports keep their order (include order matters); stdlib + badly formatted inputs match real gofmt

## Exit criteria

- [ ] Upstream scanner/parser tests pass on the fork; stdlib round-trip passes
- [ ] All target programs pass `gocpp check` (or fail only with the expected phase-gate errors)
- [ ] Interop corpus: 0 false errors, every seeded error reported at the right line
- [ ] `check -types` output reviewed on 3 files
- [ ] `gocpp fmt` output equals real gofmt (stdlib + badly formatted inputs); header groups keep their order

## Decisions / notes

- 2026-10-13 (spike): `import std "C"` is rejected by go/types, so namespace aliases are
  implemented by rewriting `std.X` → `C.std.X` on unresolved identifiers.
- 2026-10-13 (spike): stock go/parser accepts `"<vector>"`, `"::std"`, `"./x.h"` import paths,
  but rejects `std.array[int, 5]` and `std.map`, hence patches 1 and 2.
- 2026-10-13 (audit): added patch 3 (dotted type names), the same-line rule for patch 1
  (upstream test), `CheckExpr` for opaque template arguments, `gp_as[T]` for opaque conversions,
  the full gate table, stubs for all target imports, and `go/format` for `gocpp fmt`.
- 2026-10-13 (your decision): headers are packages named after their file (`raylib.X`), C++
  namespaces are packages without an import (`std.vector`), `C` keeps only C's built-in types and
  helpers, and there is no inline preamble in v1 (use a header next to the `.gp` file). Replaces
  the `C.X` + `"::std"` design.
- 2026-10-13: `gocpp fmt` doesn't sort import groups that contain headers, because they become
  `#include` lines and C/C++ include order can matter (plain gofmt reorders them).
- 2026-10-13 (fork job 1.1): Go 1.26's `go/scanner` and `go/parser` import
  `go/internal/scannerhooks`, so a re-fork to 1.26 needs one more copied package (README §2.2,
  `FORK.md`). The fork's parse errors are its own `scanner.ErrorList` type (README §2.10).
- 2026-10-13 (diagnostics job 1.13): real `go build` orders files by their first error, not by
  name, and stops at the 10th error found; README §2.10 now says so, and §2.12 lists what the CLI
  copies from `go build` (`# pkg` lines, stderr, exit 1, `-e` with `parser.AllErrors`).
- 2026-10-13 (manager, diagnostics review): gocpp prints the real column even from column 255 on,
  where `go build` prints none (an 8-bit limit in the compiler); editors can still jump to it.
  The sanity rule (§2.8) now says "no hard errors", and `-e` keeps one syntax error per line (§2.12).
- 2026-10-13: Step 1.13 (`internal/diag`) committed, clean after 3 review rounds. Its output equals
  real `go build` in 21 captured cases; the reviews added `diag.ErrorCode` for the 1.10 filter,
  emoji widths from Unicode's tables, and the documented column-255 difference.
- 2026-10-13: Step 1.1 (the fork) committed, clean after 2 review rounds: 195 upstream tests pass,
  `update.sh` rebuilds the fork with no diff. The reviews made `update.sh` safe: it refuses symlinks,
  renames or copies in patches, uncommitted fork changes (unless `-f`, which keeps a copy) and a
  repo whose top isn't gocpp's. `staticcheck.conf` turns off two checks for the upstream code.
- 2026-10-13 (round-trip job 1.5): `go/format.Node` re-parses files with grouped imports using the
  stock parser and sorts all import groups, so `gocpp fmt` prints with `go/printer` and gofmt's
  settings instead (README §2.11, step 1.16).
- 2026-10-13 (manager, import classifier job 1.6): default header names drop only the last
  extension; no default name for empty, digit-first, `C`, `_` or `init` names; two different
  headers with the same default name both get none (order-independent); the same header twice is
  "redeclared" like Go; chosen names can't be `C`, `init` or predeclared; `import "C"` and dot
  imports of headers or namespaces are errors; `package C` is an error; the caller passes the real
  gocpp package names (the goimports guess is only a fallback). README §2.3 updated.
- 2026-10-13: Step 1.2 (patch 1, `patches/0001-keyword-after-dot.patch`) committed, clean after 2
  review rounds: 0 differences against the stock parser over 121,513 Go files and 21.6M fuzz inputs.
- 2026-10-13: Step 1.5 (`internal/syntax/roundtrip_test.go`) committed, clean after 2 review rounds:
  6240 stdlib files in about 2 s; the fork must give the same AST as stock `go/parser` (exact
  positions and object resolution included), and print → parse must give the same AST again.
- 2026-10-13: Step 1.6 (`internal/load` imports.go + reserved.go) committed, clean after 3 review
  rounds: 142 tests, the same result in every import order (62,500 random runs), broken files
  handled; the caller passes real gocpp package names (step 1.9).
