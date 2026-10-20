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
  - `want: text`: a line that the output of `gocpp test` must contain;
  - `phase: NN`: the gate phase of `gocpp test` (default 09), for a fixture with Phase 10 syntax.

A fixture with Phase 10 syntax (`!`, macros) has a folder `twin/`: a Go module (its own `go.mod` and `.go` files) with the same
package written as plain Go, with the `t.Fatal` of every `!` on the line of the `!` (go test prints that line). `go test` builds the
twin in place of the renamed `.gp` files and the outputs must be equal (`must_testing`: `!` in a test, a helper with `t.Helper()`, a
subtest, a benchmark).

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

## Go's own tests of the ported packages (step 9.11, ADR-102)

The folders `testdata/*_internal` are made by `python3 tools/portstd/gotests/convert.py PKG` (from the repository root; the Go tarball
must be in the cache of `tools/portstd`): a module that holds **a copy of the port** (`stdlib/PKG/*.gp`, byte for byte, except the imports of
Go's `internal/` packages, which a program cannot import: those point at small copies or shims in the module, listed in `portpatches.json`) and
Go 1.25.14's own `_test.go` files with the patches of `tools/portstd/gotests/packages/PKG.py` (every patch and every dropped function has its
reason in that file). `TestGoTest` runs it with `gocpp test -v` and with `go test -v` and compares the two outputs, so Go is the oracle;
`TestPortCopies` fails when a copy differs from the port (run `convert.py` again). A new package: write `packages/PKG.py` (the list of test
files and the patches), run `convert.py PKG`, add the folder to `portCopies` in `port_copies_test.go`.
Today: `encoding/json` (86 tests, 13 examples), `encoding/base64`, `encoding/hex`, `encoding/binary`, `container/{list,heap,ring}`, `hash/{fnv,crc32}`,
`text/tabwriter`, `log`, `path/filepath`, `internal/chacha8rand`, `math/rand` and `math/rand/v2`.
Each takes 15 to 80 seconds (a C++ build of the package and its tests): `go test ./tests/gotest -run 'TestGoTest/encoding_json_internal'`.

