# ADR-002: The front-end is a fork of go/scanner + go/parser, with stock go/types

Status: accepted (2026-10-13)

## Context

gocpp is Go syntax plus a few additions ([ADR-012](012-go-subset-and-extensions.md)). A Go parser
and type checker take months to write, and the hard parts (untyped constants, method sets,
embedding, generic type inference, init order) are easy to get subtly wrong. Go ships them as
libraries, but the stock parser rejects a few shapes that C++ names need.

## Decision

- **Parser:** a fork of `go/scanner` + `go/parser` from the dev shell's Go (1.25.14 now) in
  `internal/syntax`. It still produces stock `go/ast` nodes. Phase 01 adds 3 patches: (1) a Go
  keyword after `.` on the same line is a name (D3), (2) constant expressions in type-argument
  lists (D4), (3) dotted type names (D20). Phase 10 adds 3 more: (4) `name!(` is one name ([ADR-013](013-must-errors.md)), (5) `#Name` is
  one token, (6) `macro` declarations, `lazy` parameters and macros on top of a declaration
  ([ADR-014](014-macros.md)). `FORK.md` lists the patches; a re-fork script re-applies them.
- **Printer:** stock `go/format` for `gocpp fmt`, then a small `go/printer` fork in Phase 10.
- **Type checker:** stock `go/types` and `go/constant` with `FakeImportC`. The front-end first
  rewrites header-package and namespace uses (`raylib.InitWindow` → `C.InitWindow`, `std.vector`
  → `C.std.vector`, [ADR-005](005-interop-header-packages.md)), so C/C++ names check as "opaque".
- `go/types` stays stock: x/tools (`gcexportdata` for the language server, `go/analysis` for
  `gocpp vet`) only accepts the stock package's types. Phase 12 fills the gaps without a fork. A
  fork would need its own ADR.

```go
func examples() {
	m := std.map[std.string, int]{} // patch 1: "map" after "." is a name
	var buf std.array[byte, 256]    // patch 2: a constant type argument
	var d std.chrono.seconds        // patch 3: two dots in a type
}
```

## Alternatives

- **A hand-written parser and checker** (rejected): months of work and subtle Go semantics bugs.
- **A forked `go/types`** (rejected): x/tools would not accept its types. It is about 34k lines
  and needs copies of several `internal/...` packages.

## Consequences

Good: Go's full typing rules for free, error messages that already read like Go's, and one
front-end for the compiler, the language server and `gocpp vet`.

Bad:
- The fork must follow upstream (3 small patches now, 6 after Phase 10).
- `FakeImportC`'s docs warn about unpredictable follow-on errors. Defenses: an interop
  "no false errors" test corpus, a sanity rule in the opaque pass, `gp_as[T]` for conversions of
  opaque values, and `types.CheckExpr` on opaque template arguments.
- Two gaps until Phase 12 (they get clear errors meanwhile): promoted members of an embedded
  C++ type, and C macros as constants (`[cstdio.BUFSIZ]byte`).

## Evidence

- Spike 2026-10-13, with the final import model: a realistic interop file type-checked with
  0 false errors, and real mistakes were still reported ([frontend](../../spikes/2026-10-13/frontend/)).
  `go/types` refuses `import std "C"` (`cannot rename import "C"`), hence the `C.std.X` rewrite.
- Stock `go/parser` rejects `std.map[...]` and `std.array[int, 5]` (spike) and
  `var d std.chrono.seconds` (audit): patches 1–3. With patch 1's same-line rule, all upstream
  scanner and parser tests pass (audit).

See: [Phase 01 §1](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#1-idea), [§2.2](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#22-the-fork-three-patches-to-goscanner--goparser), [§2.5](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#25-preprocessing-before-gotypes-proven-by-the-spike), [§2.7](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#27-gotypes-configuration), [Phase 10 §2.2](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros), [Phase 12 §2.4](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md#24-type-checking-an-interop-checker-no-gotypes-fork), divergences [D3, D4, D20](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
