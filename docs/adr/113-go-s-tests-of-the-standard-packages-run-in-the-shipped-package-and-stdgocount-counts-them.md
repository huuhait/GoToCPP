# ADR-113: Go's tests of the standard packages run in the package gocpp ships, and `stdgocount` counts them

Status: accepted (2026-10-20), step 9.10. Builds [ADR-056](056-gocpp-test-and-the-testing-package.md) (`gocpp test`), [ADR-034](034-portstd-generic-arch-and-rewritten-constraints.md)
(`tools/portstd`) and the tests of [ADR-102](102-encoding-json-is-go-s-own-package-and-derive-json-calls-it.md) (Go's tests in a copy of the port).

## Context

The exit criterion of Phase 09 is that at least 90 % of the test functions in the selected Go test files of the tier-1 packages pass, by a counting rule with a written
reason for every exclusion (README 2.8). The tests until now ran in a copy of the port (`tests/gotest/testdata/*_internal`, `convert.py`): the package is copied byte for byte
into a module, which checks the copy, not the package that `gocpp` embeds, needs a patch for every `internal/` import, and cannot test `os`, `time` or `testing`'s own
dependencies (a copy of `time` is not the `time` that `testing` imports). Nothing computed the count, and Go's `slices`, `maps`, `cmp` and `iter` tests, the range-over-func
program of `test/rangegen.go` and the rest of `test/chan` were still open.

## Decision

1. **A folder of tests joins the embedded standard library.** `GOCPP_TEST_STDLIB=<folder>` (`cmd/gocpp/stdtests.go`) puts the `_test.gp` files and `testdata` of
   `<folder>/<import path>` in the files of the embedded package, so that `gocpp test strings` builds the shipped `strings` with Go's tests, internal tests (`package strings`) and external tests
   (`package strings_test`) as Go builds them. The test binary runs in the folder of the tests, which is where the tests find their `testdata`. The loader accepts a dot-import of a
   standard package that exports `C` (Go's `unicode.C`) in the external test of that package, which Go's `unicode` tests do (`internal/load/loader.go`; user modules still get the error).
   Without the variable nothing changes: the embedded library has no test files. The variable starts with `GOCPP_TEST_` because it is for the tests of gocpp (`env_test.go`).
2. **Converted files and manifests are committed (`tests/stdgo`).** `tools/portstd/gotests/convert_std.py PKG` reads the specification `std/PKG.py` and Go's test files from the pinned tarball and
   writes the `.gp` files and `manifest/PKG.json`. The specification has `skip_file`, `only(..., taken=False)`, `drop`, `patch`, `replace_func`, `add_file`, `expect(name, class, reason)`;
   every call has a reason, a patch whose text is not found fails the script, a test file that is neither taken nor named fails it. Packages that the tests import and gocpp lacks come as shims
   (`std-shim/`: `internal/testenv` with the helpers Go's tests use, `internal/asan`, `internal/msan`, `internal/goexperiment`, `image`, `crypto/sha1`, and a `testing/quick` that checks
   `func(int64) bool` and `func(int32) bool`, because the reflect subset has no `Value.Call`) or as test-only copies of Go's own packages (`testing/iotest`, `testing/fstest`, `math/big`, `math/cmplx`,
   `compress/bzip2`). `tests/stdgo/stdgo_test.go` checks that the manifests are consistent (a reason for every exclusion; an `expect` names a function that runs) and that the committed files are what the script makes.
3. **`tools/stdgocount` (`internal/stdcount`) computes the counts.** It builds the test binary with `gocpp test -c`, runs it with `-test.v`, and reads the result of each top-level function. A crash or a
   hang gives the running test the result "fail" and the binary runs again without the tests that have a result; a round that does not end within `-test.timeout` plus 30 seconds is killed with its process group and
   counts as a hang; each process group also dies with the tool (`Pdeathsig` on a locked thread). `Tally` applies the rule: the functions of
   the selected files are the denominator; a skip is not a pass; a failure or skip that the specification expects with class `divergence` (allocation counts, the fixed stack of D24) leaves the denominator;
   a failure without an expectation, an expectation for a function that passes, and a package under 90 % are problems (`-check` exits with 1). Benchmarks and examples without an `Output:` comment are not counted: `go test` does not run them.
4. **The counts** (g++, Linux x86-64; `docs/guides/stdlib-tests.md` has the table and the reasons): the 25 packages have 1769 test functions in Go's files; 1661 are selected, 1606 pass; with the
   26 divergences (25 allocation counts, 1 D24) out of the denominator **98.2 % of the selected functions pass, and 90.8 % of all 1769**. Every package is over 90 % of its *selected* functions, with this
   qualifier: **`os` is 58.6 % of all its functions (57.6 % before the review that took `exec_unix_test.go` and half of `TestHostname` into the selection), and the budget is met on the selected files only
   because `os.Root`, deadlines, processes and fifo/zero-copy are not built.** 108 functions are in files that are not taken (they need `net`, `os/exec`, `os.Root`,
   deadlines, `splice`, `Mmap`, `gob`, `time/tzdata`, a `linkname` to the legacy `time` hooks); 24 functions are cut out of selected files (processes, `gob`, `net/http`, `synctest`, `runtime.AddCleanup`); 4 fail and 1 skips
   without being a divergence (the bugs and gaps below). The allocation checks are not patched out: a test with one is a divergence as a whole.
   **Scope:** `fmt`, `sync`, `sync/atomic` and `testing` have no manifest and are outside the count (no specification was added); they are covered by the `fmt` differential tests, the sync tests of step 8.7 and
   `gocpp test` with `tests/gotest` (step 9.9). The exit criterion "all tier-1 packages at least 90 %" is evidenced by this count for the 25 listed packages only.
5. **The deferred suites.** `slices`, `maps`, `cmp` and `iter` run as above (145 of 150 pass; 3 allocation divergences, 2 bugs). `test/rangegen.go` writes a 242,000-line program of 800 units;
   `tools/portstd/gotests/rangegen_shards.py` runs it with the real `go`, cuts it in shards of 25 units and runs each with gocpp: **800 of 800 units pass** (the one change to Go's program: the `default` case of
   `output.print`, which prints an interface with the builtin `print`, gocpp has no such print; it runs only to show a failing trace). `test/range*.go` and 18 of the 19 programs of `test/chan` pass in `tools/gotest`
   (`select2.go` reads `runtime.MemStats`).
6. **Bugs of gocpp that Go's tests found and this step fixed** (separate commits, each with a test): (a) every instance of a generic function needs its own boxed variables (`boxedOf` cached by `FuncDecl`: `slices.Collect`
   returned nothing, `iter.Pull` crashed; `tests/run/generics_closure_capture_instances.gp`); (b) `testing.T.Setenv` set the C environment and not `os`'s copy, so `os.Getenv` did not see it (`tests/gotest/testdata/setenv_tempdir`);
   (c) `os.File.Readdir` lacked Go's `stathook` for the file that disappears between the directory read and the stat (`TestReaddirRemoveRace`); (d) the `time` port used native once-flags that tests cannot reset; it uses `sync.Once`
   (as Go does), has Go's `loadFromEmbeddedTZData` and `gorootZoneSource` hooks and `interrupt`, so Go's `export_test.go` compiles.
7. **Bugs and gaps that it found and did not fix** (manifests have the reasons): slices of zero-size elements with a huge length (`makeslice` panics; `slices.TestConcat_too_large`, `TestRepeat`); `os.RemoveAll` on a tree
   deeper than `PATH_MAX` (no `unlinkat`; superseded 2026-10-23, job dev.syscalldiffs: `RemoveAll` uses `openat`/`unlinkat` and
   `TestRemoveAllLongPath` passes); `runtime.AddCleanup`; the builtin `print` of an interface value; `defer` of a `//gocpp:native` function (the emitter writes an unresolved overload; found by the `time` tests, worked around
   in the patch); `GODEBUG=asynctimerchan=1`; `Glob` of 10001 separators overflows the fixed stack (D24).

## Alternatives

- **Keep copying the port into a module (ADR-102).** It works for packages whose tests do not need anything of the package that the test framework imports, but it tests a copy, it grows a patch list for every
  `internal/` import, and `time`, `os`, `io`, `errors` and `strconv`, which `testing` depends on, could not be tested that way. The old fixtures stay for the tier-2 packages (they compare with `go test -v`).
- **Count in a Go test of the repository** instead of a tool. The numbers need minutes of compilation for 25 packages; a tool with a `-check` mode and a small unit-tested library (`internal/stdcount`) is the same code without
  putting minutes into `go test ./...`. `tests/stdgo` runs two small packages (`cmp`, `path`) through the same library.
- **Port Go's `testing/quick` and give `reflect` a `Value.Call`.** More work than the 9.10 budget, and `Call` is D6's open item; the shim covers the `time` tests that use it.

## Consequences

- `go run ./tools/stdgocount -check` is the answer to "how much of Go's own tests pass", with a reason for every function that does not; a new port adds a specification.
- A specification is Python. The script is checked by `TestConvertedTestsAreCurrent`; it needs the cache of the pinned tarball (it skips without it).
- The test files in `tests/stdgo` are Go's (BSD-3-Clause, the copyright lines are kept) and add about 98,000 lines (3.4 MB, with `testdata`) to the repository; `os`, `time`, `math/big` and `strconv` are the largest.
- `time` tests read the zone database of the machine (Go reads `GOROOT/lib/time/zoneinfo.zip`); on a machine without one they fail like the programs of `tests/run` with `// zoneinfo: required`.
- The rule counts a function cut out of a selected file as not passing. The files of `os` that need features gocpp does not have are "not taken" instead, which is a choice of the specification: the
  table shows both percentages, and `os` must be quoted with both (91.1 % of the selected, 58.6 % of all).
- Four tier-1 packages (`fmt`, `sync`, `sync/atomic`, `testing`) are not in the count; adding a specification for one of them is the way to bring it in.

## Evidence

`go run ./tools/stdgocount -detail` (the table of `docs/guides/stdlib-tests.md`); `go test ./internal/stdcount ./tests/stdgo ./cmd/gocpp -run 'Overlay|RunDir|StdlibFS'`; `go test ./internal/load -run 'DotImportC'`;
`python3 tools/portstd/gotests/rangegen_shards.py -gocpp ... -n 25` (32 of 32 shards); `go run ./tools/gotest run -filter '^(chan/|range|stringrange)'` (23 of 23).

## See

[Phase 09 README 2.8 and step 9.10](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md), [docs/guides/stdlib-tests.md](../guides/stdlib-tests.md), [tests/README.md](../../tests/README.md).
