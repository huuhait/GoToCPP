# slices

Ported by `tools/portstd` from go1.25.14's `src/slices` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/slices.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd slices` |

## Files taken

- `iter.gp`, from `iter.go`
- `slices.gp`, from `slices.go`
- `sort.gp`, from `sort.go`
- `zsortanyfunc.gp`, from `zsortanyfunc.go`
- `zsortordered.gp`, from `zsortordered.go`

## Files not taken

- 5 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

None: the files are Go's, with the header line.

## Native functions

None.

## Findings

- `unsafe`, slices.gp:11: 5 uses of the unsafe package (Phase 04 keeps gocpp memory alive for unsafe.String/Slice)

## Known divergences from Go

None.
