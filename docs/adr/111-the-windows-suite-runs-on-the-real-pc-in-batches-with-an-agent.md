# ADR-111: The Windows suite runs on the real PC, in batches, with an agent

Status: accepted (2026-10-20). Carries out [Phase 14 step 14.7](../../plans/2026-10-13-gocpp/phase-14-windows/README.md)
(test infrastructure).

## Context

The plan said "a Wine CI job for the whole suite, with `GOOS=windows` Go as the reference". Two things changed. The
owner's rule is that Windows is tested only on the real PC over ssh: Wine 11.0 cannot deliver a stack overflow on an
AVX-512 CPU, it differs from Windows in many details, and the tests of steps 14.1 to 14.5 all ran on the PC.
And `TestRunWindows` (ADR-055) starts one ssh command and one `scp` for each program: over about 2,000 tests that is
hours of round trips, with no handling of a hung program's children, no resume, no cleanup guarantee and no record of
what failed. `TestRunWindows` stays (a 166-test sample for `make ci-windows`); the full suite is a new test.

## Decision

1. **The real PC is the target; Wine is an option.** `TestWindowsSuite` needs `GOCPP_WINDOWS_SSH=user@host` in the
   environment (never in a file). Without it the suite fails ("no Windows machine"); it does not skip and does not fall back to
   Wine. `GOCPP_WINDOWS_WINE=1` runs the same code on Wine in an isolated prefix (`internal/wine`) for a developer
   without a PC; setting both is an error. This is a deviation from the plan's wording, which made Wine the CI job.
2. **Batches and an agent.** The tests are built here (`gocpp build -os windows`, 12 at a time), 150 to a batch. A batch is ONE
   gzip tar (programs and a manifest) copied once, run by `internal/wintest/agent` (a Go program built for Windows
   on the fly, `GOOS=windows`, about 2 MB) and answered by ONE archive (`results.jsonl`, `<id>.out`, `<id>.err`).
   Over ssh the cost of a run is a few commands for each batch, not for each test. The next batch is built while the
   PC runs the current one.
3. **The agent isolates programs.** At most `GOCPP_WINDOWS_JOBS` (8) at a time; each in a work folder and a `TEMP` of its own
   (so file-writing tests do not meet), stdout and stderr redirected to files (no pipe can stall; a file over 8 MiB ends the
   program), a time limit, and a **job object** with kill-on-close: the timeout, the end of the program and a dying agent all
   end the whole process tree. On Unix (the unit tests) the same code uses a process group. A line is appended to
   `results.jsonl` when a program ends; a second run of the agent on the same batch skips the programs that have a line
   (a half-written last line is cut), which is how a dropped connection is retried.
4. **The remote is an interface** (`wintest.Remote`: Put, Get, Agent, Remove, KillAll, CleanStale, Close). The real PC is a
   thin adapter over `internal/winssh`; Wine and the unit tests' fake are one `LocalRemote` (the agent in this process, the
   "programs" are shell scripts). So batching, resume, expected failures, retries, the lock and cleanup are tested on Linux
   without the PC (`internal/wintest`), and the Windows-only code (job objects) by `TestWindowsSuiteAgent` on the PC.
5. **Resume.** `results.json` is written whole after every batch. `GOCPP_WINDOWS_RESUME=1` keeps a `pass` or `xfail`
   whose *key* (a hash of the test's files) and whose *generation* (the commit and a hash of the uncommitted tracked
   changes) are equal to now; failures and errors are always run again. `GOCPP_WINDOWS_RESUME=failed` (`make test-windows-failed`) ignores the generation: after a fix, run only what failed or is new.
6. **Expected failures are checked for staleness.** `tests/windows_skip.txt` keeps its format (`name: reason`; `dir/name` for
   other folders) and its meaning, now checked: the listed tests are run and must fail. A pass is `xpass`, an error of the
   run with the message "remove the entry"; a name that is not a test is an error. `GOCPP_WINDOWS_SKIPS=ignore` skips
   them instead. Skips that do not depend on the PC (phase, `//go:build`, display, pkg-config libraries, time zone
   database, pending) are reasons in the summary by kind.
7. **No leftovers, no neighbours.** Everything is in `%USERPROFILE%\gocpp-wintest\<pid>_<time>\` (winssh's rule). `Close` first
   ends the run's processes (`KillRun`: every process whose program file is below the run's folder, and the `cmd.exe` whose command line
   names the folder followed by a character that cannot continue the id), then removes the folder, trying again while a program holds
   a file. The same runs on a signal or `-timeout` (the harness' stop hook), which also removes the work folder here; a SIGKILL is
   cleaned by the next run: on the PC, run folders named `<digits>_<digits>` older than 12h (`winssh.CleanStaleLine`; `winssh.New`
   uses the same line with 24h in place of the old `forfiles`, which touched any subfolder), and here the marked `gocpp-winsuite-<digits>`
   work folders older than 12h. Each batch's files are removed once fetched; no other folder, no registry, no installs.
   The command lines are built in `internal/winssh` (tested as text) and their effect is tested on the PC with decoy folders.
8. **One run on the PC at a time, for every test that uses it.** `internal/pclock`: a `flock` on `GOCPP_WINDOWS_LOCK` (default
   `/tmp/gocpp-windows-pc.lock`; the agents of this repository use `/tmp/claude-1000/test-slots/windows-pc.lock`), counted
   inside a process (two Remotes of one test binary share one hold). `winssh.New` holds it for the life of the connection,
   `winssh.NewUnlocked` leaves it to the caller, and the suite holds it while a batch uses the PC and not while building. The system
   releases it if the run dies. The PC has 16 logical cores shared with other work, hence 8 programs at a time by default.
9. **Output is stable.** The summary is sorted, has no times (counts per folder and per phase, skips by kind, one line for
   each wrong test); `results.json` is a map sorted by name; the log lines with times are separate.

## Consequences

- A full run is built mostly on this machine: on a loaded machine (load 300) 44 programs built in 91 s with 12 jobs, ran
  on the PC in 14 s and took 14 s more to pack, copy and fetch. `GOCPP_WINDOWS_BUILD_JOBS=24` on a quiet machine is the lever.
- The first full run will find real differences (tests that read files relative to the source folder, ones that time
  out on Windows); they go into `windows_skip.txt` with a reason, or get fixed. That list is the deliverable of the run, not of this step.
- The agent has no dependency on gocpp; it is not part of a release.
- Before a batch the free disk space is checked on both sides (twice the batch), and a run that cannot start or finish says `RUN DID NOT COMPLETE` with the reason.
- `interop` tests are not in the default selection: they include Linux headers.
- Left out on purpose: `tests/errors` and `tests/golden` (the compiler's output, Linux-side), the race tests (no
  ThreadSanitizer on Windows), the tests that need a display.
