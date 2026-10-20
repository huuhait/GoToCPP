# math/rand

Ported by `tools/portstd` from go1.25.14's `src/math/rand` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/math__rand.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd math/rand` |

## Files taken

- `exp.gp`, from `exp.go`
- `normal.gp`, from `normal.go`
- `rand.gp`, from `rand.go`
- `rng.gp`, from `rng.go`
- `zipf.gp`, from `zipf.go`

## Files not taken

- `gen_cooked.go`: //go:build ignore is false
- 7 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

None: the files are Go's, with the header line and the native declarations below.

## Native functions

| Function | Go linkname | Native |
|---|---|---|
| `runtime_rand` (`rand.gp`) | runtime.rand | `//gocpp:native gp::runtime::runtime_rand` |

## Findings

None.

## Known divergences from Go

None.
