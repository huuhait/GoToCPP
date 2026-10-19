# flag

Ported by `tools/portstd` from go1.25.14's `src/flag` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/flag.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd flag` |

## Files taken

- `flag.gp`, from `flag.go`

## Files not taken

- 7 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `flag.go`: The method value `f.defaultUsage` holds f strongly and f holds it through f.Usage: a cycle. A weak pointer breaks it.
- `flag.go`: The weak pointer of NewFlagSet.

## Native functions

None.

## Findings

- `reflect`, flag.gp:91: imports reflect: needs the reflect subset (step 9.3)

## Known divergences from Go

- NewFlagSet sets `f.Usage = f.defaultUsage`, a method value that holds the FlagSet, while the FlagSet holds the method value: a reference cycle (D1, ADR-004) that no FlagSet made by NewFlagSet would ever be freed from (every program with its own flag set leaked a few hundred bytes, and LeakSanitizer reported it). The patch makes the default `Usage` a function that holds a weak pointer to the FlagSet and calls `defaultUsage` while the FlagSet is alive. `FlagSet.Usage` is still a non-nil func, as in Go, so the programs that read and wrap it (`old := fs.Usage; fs.Usage = func() { ...; old() }`) work. The one difference: a copy of the default `fs.Usage` called after the last reference to `fs` is gone does nothing, where Go prints the usage of a flag set that nothing else can reach. The command line's flag set (`CommandLine`) is not affected: its Usage is the plain function `commandLineUsage`.
