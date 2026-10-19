# sort

Ported by `tools/portstd` from go1.25.14's `src/sort` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/sort.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd sort` |

## Files taken

- `search.gp`, from `search.go`
- `slice.gp`, from `slice.go`
- `sort.gp`, from `sort.go`
- `zsortfunc.gp`, from `zsortfunc.go`
- `zsortinterface.gp`, from `zsortinterface.go`

## Files not taken

- `gen_sort_variants.go`: //go:build ignore is false
- 10 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

None: the files are Go's, with the header line.

## Native functions

None.

## Findings

- `reflect`, slice.gp:8: imports internal/reflectlite: needs the reflect subset (step 9.3)

## Known divergences from Go

None.
