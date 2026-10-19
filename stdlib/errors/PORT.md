# errors

Ported by `tools/portstd` from go1.25.14's `src/errors` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/errors.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd errors` |

## Files taken

- `errors.gp`, from `errors.go`
- `join.gp`, from `join.go`
- `wrap.gp`, from `wrap.go`

## Files not taken

- 4 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `wrap.go`: internal/reflectlite is not imported: its three uses are the native functions below.
- `wrap.go`: Is: reflectlite.TypeOf(target).Comparable() is the native comparableType (target is not nil here).
- `wrap.go`: As: the checks at the start (target is a non-nil pointer to a type that implements error or to an interface type) are the native checkTarget, with Go's three panic texts; the nil test stays in Go.
- `wrap.go`: As: the recursion no longer carries the reflect values (the native assign finds the target type itself).
- `wrap.go`: The three native functions (their doc comments get the //gocpp:native marker from natives.txt); errorType is only used by the checks that are native now.

## Native functions

| Function | Go linkname | Native |
|---|---|---|
| `assign` (`wrap.gp`) | (none: no body upstream) | `//gocpp:native gp::errors::assign` |
| `checkTarget` (`wrap.gp`) | (none: no body upstream) | `//gocpp:native gp::errors::check_target` |
| `comparableType` (`wrap.gp`) | (none: no body upstream) | `//gocpp:native gp::errors::comparable` |

## Findings

- `unsafe`, join.gp:8: 1 uses of the unsafe package (Phase 04 keeps gocpp memory alive for unsafe.String/Slice)

## Known divergences from Go

- The three uses of internal/reflectlite are native functions (gp::errors::comparable, check_target, assign) over the type descriptors, not the reflect subset: the results and the panic texts are Go's, and errors.Is/As are several times faster (stdlib/errors/wrap.gp).
- wrapf.gp beside the port is gocpp's own file (errors.Wrapf, Phase 10, divergence D17): it is not Go's and ports.txt lists it as `own`.
