# iter

Ported by `tools/portstd` from go1.25.14's `src/iter` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/iter.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd iter` |

## Files taken

- `iter.gp`, from `iter.go`

## Files not taken

- 1 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `iter.go`: The coroutine handle is a uintptr (a *coro of an empty struct would need a type the native C++ functions can name); newcoro and coroswitch are natives (gocpp:native through natives.txt, keys runtime.newcoro and runtime.coroswitch).
- `iter.go` (2 places): The closure that runs on the coroutine gets the handle as a uintptr.

## Native functions

| Function | Go linkname | Native |
|---|---|---|
| `coroswitch` (`iter.gp`) | runtime.coroswitch | `//gocpp:native gp::iter::coroswitch` |
| `newcoro` (`iter.gp`) | runtime.newcoro | `//gocpp:native gp::iter::newcoro` |

## Findings

- `unsafe`, iter.gp:214: 22 uses of the unsafe package (Phase 04 keeps gocpp memory alive for unsafe.String/Slice)

## Known divergences from Go

- The coroutine is a native (gp::iter::newcoro, gp::iter::coroswitch) over a goroutine of its own: Pull and Pull2 are Go's code. The handle is a uintptr instead of a *coro, so the natives need no generated type. runtime.NumGoroutine counts the coroutine from Pull until the sequence is over or stop returns, as in Go; a Pull whose sequence is neither finished nor stopped leaks its goroutine, as in Go. Every switch is a hand-over between two threads: about 0.15 to 0.7 microseconds when both threads are running on different CPUs (they spin for a moment before they sleep), about 1.5 microseconds when one has to be woken, 3 on a machine with one CPU, where Go's costs 30 nanoseconds. iter.Pull is for the occasional pull (zipping two sequences), not for a loop over millions of values: range over the function itself.
