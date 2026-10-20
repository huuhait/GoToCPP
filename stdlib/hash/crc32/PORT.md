# hash/crc32

Ported by `tools/portstd` from go1.25.14's `src/hash/crc32` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/hash__crc32.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd hash/crc32` |

## Files taken

- `crc32.gp`, from `crc32.go`
- `crc32_generic.gp`, from `crc32_generic.go`
- `crc32_otherarch.gp`, from `crc32_otherarch.go`
- `gen.gp`, from `gen.go`

## Files not taken

- `crc32_amd64.go`: the file name excludes GOOS=linux GOARCH=generic
- `crc32_amd64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `crc32_arm64.go`: the file name excludes GOOS=linux GOARCH=generic
- `crc32_arm64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `crc32_loong64.go`: the file name excludes GOOS=linux GOARCH=generic
- `crc32_loong64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `crc32_ppc64le.go`: the file name excludes GOOS=linux GOARCH=generic
- `crc32_ppc64le.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `crc32_s390x.go`: the file name excludes GOOS=linux GOARCH=generic
- `crc32_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `crc32_table_ppc64le.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `gen_const_ppc64le.go`: the file name excludes GOOS=linux GOARCH=generic
- 2 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

None: the files are Go's, with the header line.

## Native functions

None.

## Findings

None.

## Known divergences from Go

None.
