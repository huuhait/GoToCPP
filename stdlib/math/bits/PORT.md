# math/bits

Ported by `tools/portstd` from go1.25.14's `src/math/bits` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into `tools/portstd/patches/math__bits.rules` (a rule with a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `compiler_bootstrap`, `purego` |
| Regenerate | `go run ./tools/portstd math/bits` |

## Files taken

- `bits.gp`, from `bits.go`
- `bits_errors_bootstrap.gp`, from `bits_errors_bootstrap.go`
- `bits_tables.gp`, from `bits_tables.go`

## Files not taken

- `bits_errors.go`: //go:build !compiler_bootstrap is false
- `make_examples.go`: //go:build ignore is false
- `make_tables.go`: //go:build ignore is false
- 4 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

None: the files are Go's, with the header line.

## Native functions

None.

## Findings

None.

## Known divergences from Go

- The panics of Div, Div32, Div64, Rem... (integer overflow, integer divide by zero) carry a `bits.errorString` (it has Go's `RuntimeError` method and the same text, `runtime error: integer overflow`), not `runtime.errorString`: `%T` of the recovered value differs, and `errors.As` to a `runtime.Error` still works once that interface exists.
