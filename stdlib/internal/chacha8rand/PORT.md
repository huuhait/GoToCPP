# internal/chacha8rand

Ported by `tools/portstd` from go1.25.14's `src/internal/chacha8rand` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/internal__chacha8rand.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd internal/chacha8rand` |

## Files taken

- `chacha8.gp`, from `chacha8.go`
- `chacha8_generic.gp`, from `chacha8_generic.go`

## Files not taken

- `chacha8_amd64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `chacha8_arm64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `chacha8_loong64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `chacha8_stub.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `chacha8_stub.s`: patch: the assembly stub that jumps to block_generic; the patch below writes the call in Go
- 2 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `chacha8.go`: gocpp has no assembly: block is block_generic, as chacha8_stub.s does for the other architectures.
- `chacha8.go`: The offsets are for the Loong64 assembly, and name internal/cpu fields that gocpp's stub does not have.
- `chacha8.go`: Imports of the dropped constant.

## Native functions

None.

## Findings

- `format`, chacha8.gp: the port is not gofmt-clean: a patch or a rewrite left odd spacing
- `go:nosplit`, chacha8.gp:48: kept as a comment
- `unsafe`, chacha8_generic.gp:53: 2 uses of the unsafe package (Phase 04 keeps gocpp memory alive for unsafe.String/Slice)

## Known divergences from Go

- Go's `chacha8_stub.s` is a jump to `block_generic` for the architectures without assembly; the patch writes that call. The record of offsets for the Loong64 assembly (`offsetLOONG64HasLSX`) is dropped with its import of `internal/cpu`.
