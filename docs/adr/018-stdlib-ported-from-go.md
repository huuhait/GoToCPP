# ADR-018: The standard library is ported from Go's own source

Status: accepted (2026-10-13)

## Context

A language feels like Go when `fmt.Printf`, `strings.Split`, `os.ReadFile` and `go test` behave
like Go, and writing all of that by hand would take years. Much of Go's standard library is pure
Go, and gocpp compiles Go syntax with Go semantics, so the real code (BSD-3-Clause) and its real
tests can be reused.

## Decision

A package is built in one of three ways:
1. **Ported Go**, copied by `tools/portstd`:
   - from the pinned upstream source tarball of the dev shell's Go version (now
     `go1.25.14.src.tar.gz`, SHA-256 checked), not from the local GOROOT;
   - only the files that `go/build` picks for the target OS with the `purego` tag (no assembly);
   - with `.go` renamed to `.gp`, `//go:linkname` declarations turned into native functions
     without a body, and a small patch set;
   - all of it recorded in `stdlib/<pkg>/PORT.md`.
2. **Native package.** A `.gp` file with function declarations that have no body, plus a `.cpp`
   file next to it that implements them.
3. **gocpp code that uses C interop**, e.g. `os` calls `<unistd.h>` through its header package.

On top of the type descriptors ([ADR-016](016-interfaces-itabs.md)) there is a **`reflect`
subset**, with exactly the API that the ported packages call (`fmt`, `flag`, `encoding/json`,
`sort`, `errors`, `context`). Not included: `reflect.MakeFunc`, `StructOf`, `FuncOf` and calling
methods by name; there is no `plugin` package (D6). Go's own tests run against the ported
packages, and at least 90 % of the selected test functions must pass. A few gocpp additions
exist, such as `errors.Wrapf(&err, ...)` and `C.cstr` (D17).

```go
// stdlib/internal/bytealg/bytealg.gp
package bytealg

func IndexByteString(s string, c byte) int // no body → implemented in bytealg.cpp
```

## Alternatives

- **Writing a standard library from scratch** (rejected): years of work, and we would lose Go's
  tests as a free conformance suite.
- **Wrapping the C++ standard library** (rejected): different semantics. For example
  `std::to_chars` prints `inf` where Go's `fmt` prints `+Inf`.

## Consequences

Good: battle-tested behavior plus a huge free test suite; float output is bit-identical to Go
(ported `strconv`); a new Go version means re-running the tool; the Windows port reuses Go's own
`_windows.go` files.

Bad:
- The `reflect` subset limits what programs can do (D6).
- Go 1.24+ crypto is a FIPS module tree, so only its generic (`purego`) code is ported, or a C
  library is wrapped. `net/http` is not planned; use a C++ HTTP library through interop.
- Ported tests need small test-only packages and patches, each with a written reason.

## Evidence

- `go list` against Go 1.25.7 (2026-10-13): most packages need only small shims; `fmt`,
  `encoding/json`, `sort` and `errors` need the `reflect` subset.
- Audit: the NixOS GOROOT is patched (`/nix/store/…` paths in `time/zoneinfo_unix.go` and four
  other files) and has no `test/` folder, so the tool reads the pinned tarball.

See: [Phase 09 §1](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#1-idea), [§2.1](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#21-three-ways-to-implement-a-stdlib-package), [§2.2](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#22-a-reflect-subset-on-top-of-type-descriptors), [§2.6](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#26-package-tiers), [§2.8](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#28-running-gos-own-tests), [Phase 14 §2.3](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#23-standard-library), divergences [D6, D17](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
