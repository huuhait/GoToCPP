# encoding/binary

Ported by `tools/portstd` from go1.25.14's `src/encoding/binary` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/encoding__binary.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `amd64`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd encoding/binary` |

## Files taken

- `binary.gp`, from `binary.go`
- `native_endian_little.gp`, from `native_endian_little.go`
- `varint.gp`, from `varint.go`

## Files not taken

- `native_endian_big.go`: //go:build armbe || arm64be || m68k || mips || mips64 || mips64p32 || ppc || ppc64 || s390 || s390x || shbe || sparc || sparc64 is false
- 3 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

None: the files are Go's, with the header line.

## Native functions

None.

## Findings

- `reflect`, binary.gp:28: imports reflect: needs the reflect subset (step 9.3)

## Known divergences from Go

None.
