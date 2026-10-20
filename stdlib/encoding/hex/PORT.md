# encoding/hex

Ported by `tools/portstd` from go1.25.14's `src/encoding/hex` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/encoding__hex.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd encoding/hex` |

## Files taken

- `hex.gp`, from `hex.go`

## Files not taken

- 2 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `hex.go`: `d.in = d.arr[:numCopy+numRead]` makes the decoder hold a counted reference to itself.

## Native functions

None.

## Findings

None.

## Known divergences from Go

- `decoder.Read` sets `d.in = d.arr[:numCopy+numRead]`, a slice of the array field `arr` that is stored in the decoder itself. In gocpp a slice of an array field holds a counted reference to the block that owns the array, which is the decoder: the decoder owned itself and was never freed (D1). The patch copies the bytes into a slice of their own (one more allocation for each `Read` that fills the buffer); `d.in` holds the same bytes, so the output is unchanged. Same reason as the rules of `encoding/base64`, `regexp/syntax` and `regexp`.
