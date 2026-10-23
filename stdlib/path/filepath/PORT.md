# path/filepath

Ported by `tools/portstd` from go1.25.14's `src/path/filepath` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/path__filepath.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux, windows |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd path/filepath` |

## Files taken

- `match.gp`, from `match.go`
- `path.gp`, from `path.go`
- `path_unix.gp`, from `path_unix.go` (linux only)
- `path_windows.gp`, from `path_windows.go` (windows only)
- `symlink.gp`, from `symlink.go`
- `symlink_unix.gp`, from `symlink_unix.go` (linux only)
- `symlink_windows.gp`, from `symlink_windows.go` (windows only)

## Files not taken

- `path_plan9.go`: the file name excludes GOOS=linux GOARCH=generic
- `symlink_plan9.go`: the file name excludes GOOS=linux GOARCH=generic
- 6 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `match.go`: Go's limit of 10000 levels needs a stack that grows; here the stack is fixed (D24: a goroutine's is 8 MiB): the runtime says how much is left, and a pattern that does not fit is as bad as one over the limit (the same error).

## Native functions

| Function | Go linkname | Native |
|---|---|---|
| `stackRoom` (`match.gp`) | (none: no body upstream) | `//gocpp:native gp::runtime::stack_room` |

## Findings

- `go:build`, path_unix.gp:5: kept: //go:build unix || wasip1
- `go:build`, symlink_unix.gp:5: kept: //go:build !windows && !plan9

## Known divergences from Go

- Stack guard in Glob. Go's `Glob` recurses once for each separator of the pattern and refuses a pattern with 10000 of them (`ErrBadPattern`, CVE-2022-30632); its goroutine stacks grow, so every smaller pattern is fine. gocpp's stacks are fixed (D24: 8 MiB for a goroutine; the main goroutine's grows up to 1 GiB, D7), and at `-O0` about 5000 to 6400 recursions fill a goroutine's, so a pattern with that many separators ended the program with `fatal error: stack overflow`. `Glob` now also gives `ErrBadPattern` when `stackRoom` (the runtime's count of the bytes of stack left above a reserve, the one that `encoding/json` asks) is negative. On a goroutine a pattern with fewer than 4000 separators works in every build; between that and 10000 the answer depends on the compiler and the mode (clang++ at `-O0` gives `ErrBadPattern` from about 5000 separators where Go gives the empty result; g++ from about 6400); on the main goroutine every pattern below Go's limit fits. `io/fs.Glob` recurses the same way and Go has no limit there either, so it has no guard: on the main goroutine it works as in Go, on another goroutine about 6000 separators or more still end a program at `-O0` with `fatal error: stack overflow` (known, low).
