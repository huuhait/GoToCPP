# tests/overflow

Programs that must die of a stack overflow (step 8.13): endless recursion on the main goroutine
(`overflow_main`) and in a goroutine while main waits (`overflow_goroutine`). Each must exit with code 2
and print `fatal error: stack overflow` and a `goroutine N [running]:` line on stderr, as Go does, not
die of a bare `SIGSEGV`. `TestStackOverflow` ([../overflow_test.go](../overflow_test.go)) runs them with each
compiler like [tests/run](../run/README.md) (same header lines). The real Go prints the same two lines after 1 GB
of stack, so both files are valid Go programs too.

They live here and not in `tests/run` because `make ci-sanitize` and `make ci-race` run `tests/run` with a
sanitizer, and a sanitizer reports an overflow its own way (exit code 1, `AddressSanitizer: stack-overflow`);
`TestStackOverflow` is skipped when `GOCPP_FLAGS` has `-asan` or `-race`. The runtime's own tests
(`runtime/goroutine_test.go`, `thread_test.go`) cover the sanitizer mode of the overflow report.
