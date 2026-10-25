# Testing gocpp on Windows

> The whole test suite for `windows/amd64`, run on a **real Windows PC** over ssh. Design:
> [ADR-111](../adr/111-the-windows-suite-runs-on-the-real-pc-in-batches-with-an-agent.md). The tests of a few
> programs and the OS layer are in [tests/README.md](../../tests/README.md#the-windows-suite); building for Windows
> is in [windows-cross.md](windows-cross.md).

`make ci` does not run any of this, and neither does the hosted Windows job of `.github/workflows/ci.yml`
([what that job covers](packaging.md#ci-and-the-release-pipeline)): a hosted runner cannot reach the real PC. It is the manual final review of the Windows port, and the job to run after a
change to the runtime's Windows layer, package `os`, the build driver or interop on Windows.

## What it does

`TestWindowsSuite` (`tests/windows_suite_test.go`) takes every test of `tests/run` and `tests/pkgs` that applies to
Windows (the ones a `//go:build` line, the phase, a display, a pkg-config library or a time zone database leave out are
skipped with the reason), builds each with `gocpp build -os windows` on this machine, and runs the programs on the PC:

1. The programs are built in batches of 150 (12 builds at a time).
2. A batch becomes ONE gzip archive: the programs and a manifest. It is copied to the PC, and a small agent
   (`internal/wintest/winagent`, built for Windows on the fly) runs the programs there, 8 at a time, each with a
   time limit (2 minutes), its own working folder and `TEMP`, and a **job object** that ends the program and every
   process it started. Output goes to files; a program that prints more than 8 MiB is ended.
3. ONE archive of outputs and exit codes comes back. The exit code, stdout and stderr are compared with the same
   `// Output:`, `// ExitCode:` and `// Stderr:` lines that `TestRun` uses (Windows' `[signal 0xc0000005 ...]` is mapped
   to Go's SIGSEGV line first).
4. While one batch runs on the PC the next one is being built.

Everything lands in `%USERPROFILE%\gocpp-wintest\<pid>_<time>\` on the PC. Each batch's files are deleted after its
results are fetched. At the end, and when the test is interrupted (Ctrl-C, SIGTERM, `go test -timeout`), the run's processes
are ended (every process whose program file is below the run's folder, and the `cmd.exe` of a running command), then the folder
is deleted, trying again while a program still holds a file; the work folder on this side (`$TMPDIR/gocpp-winsuite-*`) goes too.
**Stop a run with Ctrl-C (SIGINT)**; never kill `go test` with SIGTERM or SIGKILL (Go does not pass the signal on and the test
binary is orphaned). An orphaned suite binary notices within a second that its parent is gone and runs the same clean-up. A run
killed with SIGKILL cannot do that: the next run removes, on the PC, the run folders of this tool older than 12
hours (`GOCPP_WINDOWS_STALE_AGE`; only folders named `<digits>_<digits>`, never another tool's) and, here, the work folders of earlier
suite runs older than 12 hours (named `gocpp-winsuite-<digits>`, with a marker file, owned by you, in `$TMPDIR` or in a
sibling `nix develop` folder `/tmp/nix-shell.*`; nothing else there is touched). Ctrl-C also ends a test that is waiting for the PC lock. There are no registry or system changes.

Two runs never overlap on the PC. **Every test that uses the PC** takes the same lock (`internal/pclock`: a flock on
`GOCPP_WINDOWS_LOCK`, default `/tmp/gocpp-windows-pc.lock`, wait at most `GOCPP_WINDOWS_LOCK_WAIT`, 3h): `internal/winssh`
holds it for the life of a connection, and the suite holds it while a batch uses the PC (not while it builds). Set
`GOCPP_WINDOWS_LOCK` to the same file in every shell that tests on the PC; if the lock cannot be had, the run prints
`RUN DID NOT COMPLETE: the run did not start: ...` and says how many of the tests have a result.

## Running it

The PC is named only by the environment, never by a file:

```
export GOCPP_WINDOWS_SSH=user@host        # key authentication, a Windows 10/11 PC with sshd
make test-windows-smoke                   # 44 tests, about two minutes: is everything wired up?
make test-windows-list                    # the dry run: what would run, what is skipped (and why), nothing is built
make test-windows                         # the whole suite (the final review)
make test-windows-failed                  # only the tests that failed or have no result yet (GOCPP_WINDOWS_RESUME=failed)
```

The dev shell has the MinGW-w64 cross compiler (`nix develop`). The results are in `.windows-suite/results.json`
(ignored by git) with `results-summary.txt` and `results-failed.txt` next to it; the console shows the summary:

```
windows suite (real PC): 44 tests: pass 42, fail 0, xfail 2, xpass 0, skip 0, error 0
  pkgs     pass 3, fail 0, ...
  run      pass 39, fail 0, xfail 2, ...
  phase 02 pass 1, ...
FAIL run/foo: stdout differs from the // Output: block ...
STALE run/bar: listed as an expected failure (...), but it passes now: remove the entry
```

The output is sorted and has no times, so two runs of the same checkout print the same text. The exit status of
`go test` is the verdict.

| Variable | Default | Meaning |
|---|---|---|
| `GOCPP_WINDOWS_SSH` | none | `user@host` of the PC. Without it (and without `GOCPP_WINDOWS_WINE=1`) the suite **fails**; it never skips silently |
| `GOCPP_WINDOWS_WINE` | none | `1`: run under Wine on this machine instead (see below); setting both is an error |
| `GOCPP_WINDOWS_LIST` | none | `1`: the dry run |
| `GOCPP_WINDOWS_ONLY` | all | a regexp on `dir/name` (for example `^run/os_`), or `@file` with one `dir/name` a line (`@tests/windows_suite_smoke.txt`). A selection writes `selection.json`, not `results.json` |
| `GOCPP_WINDOWS_DIRS` | `run,pkgs` | `interop` can be added; most of its tests include Linux headers and fail, so list them in `windows_skip.txt` first |
| `GOCPP_WINDOWS_RESUME` | none | `1`: continue an interrupted run: keep the passes of the results file when the checkout state (commit and uncommitted changes) and the test's files are the same. `failed`: keep the passes across changes of the checkout (run only what failed or is new, after a fix; the passes are assumed to still hold, so do a full run before you call it done) |
| `GOCPP_WINDOWS_SKIPS` | `check` | `ignore`: leave the tests of `windows_skip.txt` out instead of running them (see below) |
| `GOCPP_WINDOWS_JOBS` | 8 | programs at a time on the PC (it has 16 logical cores, shared) |
| `GOCPP_WINDOWS_BUILD_JOBS` | 12 | builds at a time here |
| `GOCPP_WINDOWS_BATCH` | 150 | programs per archive |
| `GOCPP_WINDOWS_TIMEOUT` | 2m | the time limit of one program |
| `GOCPP_WINDOWS_RESULTS` | `.windows-suite/results.json` | the results file |
| `GOCPP_WINDOWS_LOCK`, `GOCPP_WINDOWS_LOCK_WAIT` | `/tmp/gocpp-windows-pc.lock`, 3h | the lock file between runs, and how long to wait for it |
| `GOCPP_TEST_PHASE` | 15 (the make targets) | the harness' phase limit |

## Expected failures

`tests/windows_skip.txt` (`name: reason`, or `dir/name: reason`) lists the tests that are **expected to fail** on
Windows, each with the reason. They are built and run like the others, and:

- a listed test that fails is an `xfail` (fine);
- a listed test that **passes** is an `xpass`: the entry is stale, the run fails and the line says to remove it;
- a name that is not a test is an error.

So the list cannot rot: when a fix makes a test pass, the next run says so. After the first full run, the failures
that are real gaps go into `windows_skip.txt` with their reason (or are fixed).

## Disk space, exit codes, output

- Before each batch the run checks the free space: here (twice the size of the last batch) and on the PC (twice the programs of
  the batch plus its archive). When there is not enough, the run stops with the numbers
  (`not enough free disk space on the machine: 900 MiB free, 2400 MiB needed ...`).
- Exit codes are the full 32-bit value that Windows keeps, not the low 8 bits of ssh: `os.Exit(-1)` is 4294967295 and
  `os.Exit(261)` is 261. That is why `os_proc_exit_code_range` is an expected failure.
- The programs' output goes to files, not to pipes (a pipe can stall in the ssh server); stdout and stderr are compared separately.
- There is no limit on a program's memory or process count other than its time (2 minutes) and its output (8 MiB).
- **A failing test is never retried automatically** (a flaky pass would hide a real race). Only an infrastructure failure
  (a dropped ssh connection) is retried, once. To run a failure again, use `make test-windows-failed`.

## Resume, retries, hung programs

- The results file is written after every batch. A run that is stopped (a signal, the machine, `-timeout`) is
  continued with `GOCPP_WINDOWS_RESUME=1`: a pass is kept only if the checkout state (the commit, a hash of the
  uncommitted changes of tracked files and a hash of the untracked files that git does not ignore) and the test's own files are the same as when it ran, so a compiler change makes it run
  everything again. After a fix, `make test-windows-failed` (`GOCPP_WINDOWS_RESUME=failed`) keeps the passes across
  changes and runs only the failures, the errors and the tests without a result. Failures and errors are always run again.
- A batch whose ssh connection drops (copy, run or fetch) is tried once more after the run's processes on the PC
  are ended; the agent resumes the batch, running only the programs that have no result yet. A second failure
  marks the batch's tests `error`, which a resume runs again.
- A hung program is ended with its process tree when its time is up; the test fails with the output so far. If the
  agent itself hangs, the batch's time budget ends it and the processes of the run are killed.

## Wine

Wine is not the way to test Windows: it does not deliver a stack overflow at its own guard page on CPUs with a large
XSAVE area (AVX-512; the runtime works around it with a guard page of its own under Wine) and differs in many details. `GOCPP_WINDOWS_WINE=1 make test-windows-smoke` runs the same suite with the same agent
under Wine in an isolated prefix on this machine, for a developer who has no PC. It is never the default, and nothing
selects it for you.

## The manual checklist for a Windows release

The suite cannot check these; do them by hand on the PC after a green `make test-windows`:

1. A console window: run a program that reads a line from a real terminal window (cmd.exe, Windows Terminal), prints
   non-ASCII text, and press Ctrl-C and Ctrl-Break during `time.Sleep` (`TestWindowsConsole` covers a pseudo console over ssh).
2. A real crash: a null pointer dereference exits with `0xc0000005` and the Go message; a stack overflow with exit code 2.
3. 10,000 blocked goroutines: the commit charge (each thread reserves 8 MiB) and the time to start them.
4. A machine with more than 64 logical processors, if one is at hand (`cpu_count` sees one processor group).
5. UCRT: build with the pinned llvm-mingw (`GOCPP_TOOLCHAIN=pinned`) and run `make test-windows-smoke` again.
6. `gocpp.exe` on the PC: `build`, `run`, `test`, `fmt` of the example programs, and the editor (VS Code) opens a
   module, completes and jumps.
7. Code pages: a C `printf` of non-ASCII text under code pages 437 and 850 (a documented limitation).

## Adding a test

A run test needs nothing: it is in the suite when it has no `//go:build` line that excludes Windows. A test whose output is
Linux's must say so: put it in `windows_skip.txt` with the reason (and, if it matters on Windows, add a twin with a
`//go:build windows` line whose `// Output:` is what real Go prints on Windows: the `os_win_*` tests).
