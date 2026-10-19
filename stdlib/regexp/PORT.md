# regexp

Ported by `tools/portstd` from go1.25.14's `src/regexp` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/regexp.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd regexp` |

## Files taken

- `backtrack.gp`, from `backtrack.go`
- `exec.gp`, from `exec.go`
- `onepass.gp`, from `onepass.go`
- `regexp.gp`, from `regexp.go`

## Files not taken

- 6 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `onepass.go`: The closure `check` captures the variable `check` (it calls itself); setting the variable to nil before makeOnePass returns breaks the cycle, and nothing calls check afterwards.

## Native functions

None.

## Findings

None.

## Known divergences from Go

- makeOnePass declares `check` as a variable and assigns a closure that calls `check`. In gocpp a closure that refers to itself through the variable it is stored in is a reference cycle (D1, ADR-004): the closure, the queues and the rune tables it captured were never freed, about 5 KB for every one-pass regexp that was compiled. The patch sets the variable to nil when makeOnePass is done with it.
