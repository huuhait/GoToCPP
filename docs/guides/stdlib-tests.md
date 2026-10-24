# Go's own tests of the standard packages

Step 9.10 runs the `_test.go` files of Go 1.25.14 for the tier-1 packages (and `encoding/json`) with `gocpp test`, in the packages that gocpp ships,
and counts the test functions that pass by the counting rule of the [Phase 09 README, section 2.8](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#28-running-gos-own-tests).
[ADR-113](../adr/113-go-s-tests-of-the-standard-packages-run-in-the-shipped-package-and-stdgocount-counts-them.md) records the decisions.

**Scope.** The count covers 25 packages: 20 of the 24 tier-1 packages of the Phase 09 README (`errors strings bytes strconv unicode unicode/utf8 sort slices maps cmp iter math math/bits os io bufio time context flag regexp`), `encoding/json`, and `io/fs`, `path`, `regexp/syntax` and `unicode/utf16`, which ship with their neighbours. The other four tier-1 packages, **`fmt`, `sync`, `sync/atomic` and `testing`, have no manifest and are outside this count**. They are covered elsewhere: `fmt` by the differential tests against real Go (exit criteria of the Phase 09 README), `sync` and `sync/atomic` by the sync tests of step 8.7, `testing` by `gocpp test` itself (step 9.9: `tests/gotest` compares its output with real `go test`). The exit criterion "all tier-1 packages pass at least 90 % of their selected Go test functions" is therefore evidenced by this count for the 25 listed packages only; for the four others it rests on those tests, not on Go's own test files.

## Running them

```sh
python3 tools/portstd/gotests/convert_std.py strings        # make tests/stdgo/strings and its manifest (needs the cache of tools/portstd)
go run ./tools/stdgocount -detail strings bytes             # build the test binaries, run them, print the counts and every function that does not pass
go run ./tools/stdgocount -check                            # every manifest; exit status 1 on an unexplained failure, a stale expectation or a package under 90 %
GOCPP_TEST_STDLIB=$PWD/tests/stdgo gocpp test -v strings    # the same tests by hand, with the gocpp you have
make stdlib-nightly                                         # stdgocount -check and rangegen_shards.py (time not measured yet)
```

They are not part of `make ci`: the nightly job runs `make stdlib-nightly` as its `stdlib` stage
([sanitizers.md](sanitizers.md#the-nightly-job): `tools/sanitize/nightly.sh` on this machine, the `stdlib` job of
`.github/workflows/sanitizers-nightly.yml` on GitHub), so a change that breaks one of Go's tests shows up the next morning.

`GOCPP_TEST_STDLIB` is a folder with one subfolder for each import path. The `_test.gp` files and the `testdata` folder in it join the files of the embedded
package (a file of the folder shadows the embedded file of the same name), and the test binary runs in that subfolder, as `go test` runs in the package folder.
Without the variable the embedded library has no test files. `GOCPP_TEST_ALL_ERRORS=1` makes `gocpp check` and `gocpp test` print every error of a package,
not the first ten, which is how a converted test file is brought to compile.

## What is in `tests/stdgo`

| Path | What |
|---|---|
| `tests/stdgo/<pkg>/*_test.gp` | Go's test files for linux/amd64, converted (the syntax is Go's; the extension is `.gp`) and patched |
| `tests/stdgo/<pkg>/testdata` | Go's `testdata`, for the specifications that say `TESTDATA = True` |
| `tests/stdgo/manifest/<pkg>.json` | every `Test`, `Example` and `Fuzz` function of Go's test files, with what happened to it, the expected failures and the patches, each with its reason |
| `tests/stdgo/internal`, `image`, `crypto`, `testing/quick` | the shim packages that the tests import and gocpp does not ship (`tools/portstd/gotests/std-shim`) |
| `tests/stdgo/testing/iotest`, `testing/fstest`, `math/big`, `math/cmplx`, `compress/bzip2` | Go's own packages that the tests import, copied from the tarball as test-only packages |

The files are made by `tools/portstd/gotests/convert_std.py` from the pinned tarball of Go (the one of `tools/portstd`; the script never downloads) and committed;
`tests/stdgo/stdgo_test.go` makes them again in a temporary folder and compares (`TestConvertedTestsAreCurrent`, skipped without the cache of `tools/portstd`).
Do not edit them by hand: change the specification and run the script.

A specification is `tools/portstd/gotests/std/<pkg>.py` (`/` is `__`): a Python file with these calls, each with the reason that goes into the manifest.

| Call | Meaning |
|---|---|
| `FILES = ['all']` | take every test file of Go for linux/amd64 that `skip_file` does not name (a file that is neither taken nor named is an error) |
| `skip_file(file, reason)` | the file is not selected: its functions count as "not taken" |
| `only(file, keep, reason, taken=False)` | build only the helpers `keep` of the file; with `taken=False` its tests are "not taken" too (`os`'s `root_test.go`) |
| `drop(file, name, reason)` | cut the function out of a selected file: it counts as "dropped" (not passing) unless the reason starts with `divergence:` |
| `patch(file, old, new, reason, count=1)` | replace text; the script fails when `old` is not found `count` times |
| `replace_func(file, name, text, reason)`, `add_file(name, text)` | replace a function, add a file that is not Go's |
| `expect(name, class, reason)` | the function fails or skips in gocpp: `divergence` (leaves the denominator), `bug`, `unsupported` or `flaky` |
| `TESTDATA = True`, `ARGS = [...]`, `EXTRA = [...]`, `SHIMS = [...]` | copy `testdata`; more arguments of the test binary; more test-only packages and shims |

A failure or a skip without an `expect`, and an `expect` for a function that passes, is a problem that `stdgocount -check` reports.

## The counting rule

`stdgocount` runs the test binary with `-test.v`. When it crashes or hangs, the test that was running gets the result "fail" and the binary runs again without
the tests that have a result (up to 40 rounds). A binary that does not end by itself within `-timeout` (default 5 minutes) plus 30 seconds is killed with its process group
and counts as a hang. The columns:

- **functions**: every `Test`, `Example` with an `Output:` comment, and `Fuzz` function of Go's test files (benchmarks and examples without an output comment are
  compiled and not run, as `go test` does).
- **selected**: the functions of the selected files (all of the files except `skip_file` and `only(..., taken=False)`).
- **passed**, **failed**, **skipped**: a skip counts as not passing, unless it is expected as a divergence.
- **dropped**: functions of a selected file that the specification cuts out (a test that starts a process, `gob`, `net/http`...). They count as not passing.
- **divergences**: expected failures and skips of class `divergence` (and drops whose reason starts with `divergence:`): `AllocsPerRun` counts and the fixed
  stack of D24. They leave the denominator, as the plan says.
- **pass %** = passed / (selected - divergences), the number the 90 % budget is about. **overall %** = passed / functions: it also counts the files that are not taken.

## Results

(The table is the output of `go run ./tools/stdgocount`: gocpp at the commit of step 9.10, g++, Linux x86-64, with the `os` row of the run of 2026-10-23, job dev.syscalldiffs. A new full count is `go run ./tools/stdgocount -check` over every manifest.)

| package | functions | selected | passed | failed | skipped | dropped | divergences | not taken | pass % | overall % |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| bufio | 88 | 87 | 86 | 0 | 0 | 0 | 1 | 1 | 100.0 | 97.7 |
| bytes | 161 | 157 | 150 | 0 | 0 | 0 | 7 | 4 | 100.0 | 93.2 |
| cmp | 8 | 8 | 8 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| context | 46 | 45 | 43 | 0 | 0 | 1 | 1 | 1 | 97.7 | 93.5 |
| encoding/json | 103 | 103 | 101 | 0 | 0 | 2 | 0 | 0 | 98.1 | 98.1 |
| errors | 16 | 16 | 16 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| flag | 29 | 27 | 25 | 0 | 0 | 2 | 0 | 2 | 92.6 | 86.2 |
| io | 72 | 72 | 71 | 0 | 0 | 1 | 0 | 0 | 98.6 | 98.6 |
| io/fs | 22 | 22 | 21 | 0 | 0 | 0 | 1 | 0 | 100.0 | 95.5 |
| iter | 12 | 12 | 12 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| maps | 24 | 24 | 24 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| math | 111 | 111 | 111 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| math/bits | 62 | 62 | 62 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| os | 278 | 180 | 164 | 0 | 0 | 15 | 1 | 98 | 91.6 | 59.0 |
| path | 17 | 17 | 17 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| regexp | 75 | 75 | 75 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| regexp/syntax | 12 | 12 | 12 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| slices | 106 | 106 | 101 | 2 | 0 | 0 | 3 | 0 | 98.1 | 95.3 |
| sort | 48 | 48 | 47 | 0 | 0 | 0 | 1 | 0 | 100.0 | 97.9 |
| strconv | 87 | 87 | 85 | 0 | 0 | 0 | 2 | 0 | 100.0 | 97.7 |
| strings | 127 | 127 | 122 | 0 | 0 | 0 | 5 | 0 | 100.0 | 96.1 |
| time | 183 | 181 | 174 | 1 | 1 | 3 | 2 | 2 | 97.2 | 95.1 |
| unicode | 42 | 42 | 42 | 0 | 0 | 0 | 0 | 0 | 100.0 | 100.0 |
| unicode/utf16 | 9 | 9 | 8 | 0 | 0 | 0 | 1 | 0 | 100.0 | 88.9 |
| unicode/utf8 | 31 | 31 | 30 | 0 | 0 | 0 | 1 | 0 | 100.0 | 96.8 |
| total | 1769 | 1661 | 1607 | 3 | 1 | 24 | 26 | 108 | 98.3 | 90.8 |

98.3 % of the selected functions pass (1607 of the 1635 that remain without the 26 divergences), and 90.8 % of all the 1769 functions of Go's test files for these packages.

**`os` is the weak row, and the headline needs that qualifier.** `os` is 91.6 % of its selected functions but only **59.0 % of all its 278 functions** (58.6 % before `TestRemoveAllLongPath` passed, 2026-10-23; 57.6 % before the review of step 9.10
took `exec_unix_test.go` and half of `TestHostname` into the selection). The budget is met on the selected files only; 98 functions are in files that are not taken (Root, deadlines, processes and
fifo/zero-copy coverage), and the 15 functions cut out of selected files count as not passing. Do not read the table as "every package is over 90 %" without that:
every package is over 90 % of its *selected* functions, and `os` is the one where the selection leaves out the most. No other package is below 86 % of all its functions.

**Selection is not current API status.** The committed `os` conversion excludes the upstream deadline files with
historical “blocking-mode internal/poll” reasons. Linux pollable files now implement deadlines and close-wakeup
(`stdlib/internal/poll/PORT.md`, `tests/run/os_poll_*.gp`); Windows still returns `ErrNoDeadline`.
This guide reports the unchanged manifest selection and recorded results, not a rerun that adds those tests.
See the [current library status](stdlib.md#what-differs-from-go) before treating an exclusion reason as a missing API.

## What fails, and why

Every function that does not pass is listed with its reason by `go run ./tools/stdgocount -detail`; the manifests hold the same reasons. In groups:

- **Allocation counts** (`AllocsPerRun`; 25 functions, class `divergence`): gocpp counts C++ `operator new`, and a `string(bytes)` or `append(s, make(S, n)...)` allocates a temporary
  where Go does not. The allocation checks are not patched out: a test with one counts as a divergence as a whole, so no pass comes from a test with its check removed.
- **Unavailable APIs or excluded coverage**: `os.StartProcess`, `os/exec` and `os/signal` (the tests that start processes), `os.Root`, the upstream read/write deadline files (see the selection caveat above), the zero-copy paths that need package
  `net` (`splice`, `sendfile`; `ReadFrom` does use `copy_file_range`, but its tests need `net` and `internal/testpty` too), `internal/syscall/unix` (nonblocking opens), `syscall.Mmap`, package `net` (sockets, `net.IP`), `net/http`, `encoding/gob`, `testing/synctest`,
  `runtime.AddCleanup`, `GODEBUG=asynctimerchan`, and the `GOROOT/lib/time/zoneinfo.zip` that Go's `time` tests read (they read the zone database of the machine here).
- **Bugs of gocpp that the tests found** (not fixed; each has a reason in its manifest): a slice of zero-size elements (`[]struct{}`) with a huge length costs one byte for each element, so
  `makeslice` panics (`slices.TestConcat_too_large`, `TestRepeat`); the builtin `print` does not print an interface value. (`os.RemoveAll`, path based until 2026-10-23, now uses
  `openat`/`unlinkat` and passes `TestRemoveAllLongPath`.)
- **D24** (the 26th divergence): `io/fs.TestCVE202230630` recurses 10001 times in `Glob`, which overflows the fixed 8 MiB stack of an unoptimized goroutine.

## The deferred suites

- **`slices`, `maps`, `cmp`, `iter`**: run as above (`tests/stdgo`); 145 of the 150 selected functions pass, 3 are allocation divergences and 2 are the zero-size-slice bug below. Go's tests found one compiler bug, fixed in the same step:
  every instance of a generic function must have its own set of boxed variables (`internal/lower/borrow.go`; `tests/run/generics_closure_capture_instances.gp`).
- **Range over func**: `test/rangegen.go` writes a program of 242,000 lines, 800 units (nested range-over-func loops with labels, `goto`, `defer`, `panic`, `recover` and `select`, each
  compared with the same loops over a slice). `tools/portstd/gotests/rangegen_shards.py` runs `rangegen.go` with the real `go`, cuts the program into shards of 25 units, builds and runs each with gocpp:
  **800 of 800 units pass** (32 shards, about 30 seconds on 12 cores). `test/range*.go` and `test/stringrange.go` pass in `tools/gotest` too.
- **`test/chan`**: 18 of the 19 programs pass (`go run ./tools/gotest run -filter '^chan/'`; they are in the regression gate). `select2.go` reads `runtime.MemStats` and is skipped as `runtime-internals`.

## Adding a package

1. Write `tools/portstd/gotests/std/<pkg>.py` with `FILES = ['all']` and run `python3 tools/portstd/gotests/convert_std.py <pkg>`.
2. `GOCPP_TEST_ALL_ERRORS=1 go run ./tools/stdgocount -detail <pkg>`: a compile error means a `patch` (the reason says what gocpp lacks), a missing import a shim or a test-only package.
3. Add an `expect` or a `drop` with a reason for each function that does not pass, and fix the bugs that are small.
4. `go test ./tests/stdgo` checks that the manifests are consistent and that the committed files are what the script makes.
