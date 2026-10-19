# strings

Ported by `tools/portstd` from go1.25.14's `src/strings` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/strings.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd strings` |

## Files taken

- `builder.gp`, from `builder.go`
- `clone.gp`, from `clone.go`
- `compare.gp`, from `compare.go`
- `iter.gp`, from `iter.go`
- `reader.gp`, from `reader.go`
- `replace.gp`, from `replace.go`
- `search.gp`, from `search.go`
- `strings.gp`, from `strings.go`

## Files not taken

- 10 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `builder.go`: Go's field `addr *Builder` is a reference to the Builder itself: with reference counting the Builder (and its buffer) then lives forever. A uintptr detects the copy by value just as well (a copy has another address).
- `builder.go`: copyCheck compares addresses instead of pointers; the escape-analysis workaround (abi.NoEscape) is not needed for a uintptr.
- `builder.go` (2 places): abi is not used any more.

## Native functions

None.

## Findings

- `unsafe`, builder.gp:10: 4 uses of the unsafe package (Phase 04 keeps gocpp memory alive for unsafe.String/Slice)

## Known divergences from Go

- Builder remembers its address as a uintptr, not as a *Builder: in gocpp a pointer field is a counted reference, so Go's `b.addr = b` made every Builder own itself and never free its buffer (a reference cycle, ADR-004). Same checks and the same panic; no strings.Builder leaks.
