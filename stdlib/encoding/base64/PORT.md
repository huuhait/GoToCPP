# encoding/base64

Ported by `tools/portstd` from go1.25.14's `src/encoding/base64` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/encoding__base64.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd encoding/base64` |

## Files taken

- `base64.gp`, from `base64.go`

## Files not taken

- 2 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `base64.go` (2 places): `d.out = d.outbuf[:nw]` makes the decoder hold a counted reference to itself (a cycle that is never freed, D1).

## Native functions

None.

## Findings

None.

## Known divergences from Go

- `decoder.Read` sets `d.out = d.outbuf[:nw]`, a slice of the array field `outbuf` that is stored in the decoder itself. In gocpp a slice of an array field holds a counted reference to the block that owns the array, which is the decoder: the decoder owned itself and no `NewDecoder` was ever freed (LeakSanitizer found 2 KB for each decoder that had read). The patch copies the decoded bytes into a slice of their own (one more allocation for each `Read` that decodes a chunk); `d.out` is the same bytes, so the output is unchanged. The same pattern (a slice of an array field stored in the struct) is why `regexp/syntax` and `regexp` have patch rules (ADR-083).
