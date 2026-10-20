# io

Ported by `tools/portstd` from go1.25.14's `src/io` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/io.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd io` |

## Files taken

- `io.gp`, from `io.go`
- `multi.gp`, from `multi.go`
- `pipe.gp`, from `pipe.go`

## Files not taken

- 5 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `io.go`: EOF is the runtime's error value: the native fmt.Scan functions return it, and callers compare with ==.
- `io.go`: ErrUnexpectedEOF is the runtime's error value too.

## Native functions

| Function | Go linkname | Native |
|---|---|---|
| `runtimeEOF` (`io.gp`) | (none: no body upstream) | `//gocpp:native gp::fmt::io_eof` |
| `runtimeUnexpectedEOF` (`io.gp`) | (none: no body upstream) | `//gocpp:native gp::fmt::io_unexpected_eof` |

## Findings

None.

## Known divergences from Go

- EOF and ErrUnexpectedEOF are the values of the runtime (native functions), so that the native Scan functions of fmt return the same errors: err == io.EOF holds for them.
