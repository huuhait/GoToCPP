# tests/gotest

Tests of `gocpp test` against real `go test` (step 9.9, ADR-056, docs/guides/testing.md).

`TestGoTest` runs one subtest for each folder of `testdata/`. A folder is a module (`gocpp.mod`, `.gp` files; for
`go test` the `.gp` files are renamed to `.go` and a `go.mod` is made) and two files that are not part of it:

- `args`: the arguments of `gocpp test`, one on each line (or several on a line);
- `meta` (optional): one directive on each line:
  - `cut-panic`: the goroutine traces after a panic are not compared (their frames differ);
  - `bench`: the numbers of benchmark lines are not compared;
  - `stdout-only`: standard error is not compared (the compilers' words differ);
  - `timeout`, `multi`: the run times out, or has several packages (no effect on the comparison);
  - `diverge: why`: the output differs from `go test`'s for a reason; it is not compared, only the `want:` lines;
  - `want: text`: a line that the output of `gocpp test` must contain.

The standard output (after the normalization of times, paths, the extension of the file names and the numbers of
benchmarks) and the exit status of the two commands must be equal. Everything runs through the object cache of
the test run (`XDG_CACHE_HOME` in a temporary folder); `GOCPP_TEST_JOBS=n` (default 2) runs n fixtures at a time.

The other tests of the package:

- `TestRegexpDialect` (folder `regexp_names`): lists test names with many patterns on the test binary of
  `gocpp test -c` and on the one of `go test -c`, and checks the names and the error texts (the matcher is package `regexp`,
  as in Go: `testing/internal/testdeps`);
- `TestRerunAfterEdit`: a second `gocpp test` after editing a test file, the package and the flags;
- `TestOutputFlag`: `-c`, `-o` with and without `-c`, `-o` with a folder;
- `TestParallelOrder`: the order in which parallel subtests are released with `-parallel 1`.

`proc_unix_test.go` and `proc_other_test.go` run the commands in a process group that dies with the test.
The fixtures need `go` in the PATH (they skip without it); `-short` skips all of them.
