# Phase 01: Front-end · STATE

**Status:** `[ ]` not started (approach validated by spike, gaps found by the audit are now planned)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike: stock go/parser + go/types (FakeImportC) on interop-heavy `.gp` files (2026-10-13, 0 false errors)
- [x] Spike re-run with the final import model: header packages, namespaces as packages, namespace probe with clang++ and g++ (typos `stdd`, `fmtt` reported) (2026-10-13)
- [x] Check: stock go/printer + `ast.SortImports` print gocpp syntax correctly (2026-10-13)
- [ ] 1.1 Fork go/scanner + go/parser (Go 1.25.7) into `internal/syntax`
  - [ ] copy sources + license headers + `FORK.md` (+ `go/printer/nodes.go` for the perf test)
  - [ ] upstream tests pass unchanged
  - [ ] `update.sh` re-fork script
- [ ] 1.2 Patch 1: Go keywords allowed after `.` (same line only)
  - [ ] scanner change
  - [ ] tests: `std.map`, `x.type()`, `posix.select`, `x.(type)` unaffected, semicolon after `.range`, `TestIncompleteSelection`
- [ ] 1.3 Patch 2: constant expressions in type-argument lists
  - [ ] parser change (`parseRhs` for type arguments)
  - [ ] tests: type position, composite literal, call, nested
- [ ] 1.4 Patch 3: dotted type names (`std.chrono.seconds`, `std.filesystem.path`, `std.vector[int].iterator`)
- [ ] 1.5 Round-trip test over the Go stdlib (parse → print → parse)
- [ ] 1.6 Import classifier + `FileInfo` (header package names and their rules, namespace renames) + reserved-`C` error
- [ ] 1.7 `//gocpp:` directive parser + validation + suggestions
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
- [ ] 1.13 Diagnostics package (format, sort, dedupe, limit, caret)
- [ ] 1.14 CLI: `version`, `parse -dump`, `check`, `check -types`, `fmt`
- [ ] 1.15 Error tests + "no false errors" interop corpus
- [ ] 1.16 `gocpp fmt` via `go/format.Node` (`-l -w -d`); header imports keep their order (include order matters); stdlib unchanged + badly formatted inputs match real gofmt

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
