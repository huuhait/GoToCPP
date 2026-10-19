# unicode

Ported by `tools/portstd` from go1.25.14's `src/unicode` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/unicode.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd unicode` |

## Files taken

- `casetables.gp`, from `casetables.go`
- `digit.gp`, from `digit.go`
- `graphic.gp`, from `graphic.go`
- `letter.gp`, from `letter.go`
- `tables.gp`, from `tables.go`

## Files not taken

- 5 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `letter.go` (3 places): is16 and is32 (linear search): the entry is a 4 or 8 byte struct; read it by value.
- `letter.go` (3 places): lookupCaseRange returns the entry (CaseRange is 20 bytes) and whether there is one, instead of a pointer to it or nil; convertCase takes the entry.

## Native functions

None.

## Findings

None.

## Known divergences from Go

- The searches of the range tables (is16, is32's linear case, lookupCaseRange) copy the table entry instead of taking its address: `range_ := ranges[m]`, and lookupCaseRange returns (CaseRange, bool). Same results; a pointer to a slice element is a counted pointer in gocpp and about 4-5 times slower.
