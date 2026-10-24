# tests/overflow

Programs that must die of a stack overflow (step 8.13): endless recursion on the main goroutine
(`overflow_main`) and in a goroutine while main waits (`overflow_goroutine`). Each must exit with code 2
and print `fatal error: stack overflow` and a `goroutine N [running]:` line on stderr, as Go does, not
die of a bare `SIGSEGV`. `TestStackOverflow` ([../overflow_test.go](../overflow_test.go)) runs them with each
compiler like [tests/run](../run/README.md) (same header lines). The real Go prints the same two lines after 1 GB
of stack, so both files are valid Go programs too. The runtime raises the main thread's stack limit to 1 GiB (Go's
limit, at most the hard limit), so `TestStackOverflow` runs the programs under a hard `RLIMIT_STACK` of 64 MiB (Linux):
the endless recursion on the main goroutine ends after 64 MiB instead of touching 1 GiB. The overflow at the 1 GiB limit
itself is checked without a recursion by the runtime's own test (`runtime/thread_test.go`, `main-stack-*`).

A test may have a `// flags: -release` line (as in [tests/run](../run/README.md)): `gocpp run` builds it with
those flags. `overflow_release_{tail,mutual,accumulator}` are the endless recursions that an optimized C++ build
would turn into a loop, a jump or an early return (a call in tail position, two functions that call each other last,
`return f(n+1) + x`); gocpp keeps the calls of the functions that can recurse (`GP_RECURSIVE`, `GP_RECURSION_MARK()`:
`internal/lower/recursion.go`, language.md D7), so they overflow as in Go. Each has a debug twin,
`overflow_debug_*`, the same program without the flag. The report of a plain `-release` program is its message
lines only (ADR-136): `runtime: goroutine stack exceeds ...` and `fatal error: stack overflow`, without the
`goroutine N [running]:` line, so those tests expect only the `fatal error` line.

They live here and not in `tests/run` because `make ci-sanitize` and `make ci-race` run `tests/run` with a
sanitizer, and a sanitizer reports an overflow its own way (exit code 1, `AddressSanitizer: stack-overflow`);
`TestStackOverflow` is skipped when `GOCPP_FLAGS` has `-asan` or `-race`. The runtime's own tests
(`runtime/goroutine_test.go`, `thread_test.go`) cover the sanitizer mode of the overflow report.
