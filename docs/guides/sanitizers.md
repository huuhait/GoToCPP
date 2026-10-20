# Sanitizers: the nightly job and the leak audit

`tools/sanitize` runs gocpp programs under AddressSanitizer, LeakSanitizer, UndefinedBehaviorSanitizer and
ThreadSanitizer, and **fails on any report**. It is the nightly job of step 15.11
([ADR-090](../adr/090-nightly-sanitizers-and-a-bounded-leak-policy.md)); the sanitizer flags of `gocpp build`
(`-asan`, `-race`) and the gates of `make ci` are older and stay (`make ci-sanitize`, `make ci-race`).

```
make sanitize-smoke                 # 14 programs, both compilers, every stage: about two minutes
make sanitize-nightly               # the job: all of tests/run, g++ and clang++ (hours of CPU time)
make sanitize-leaks                 # the leak audit alone
make sanitize-selftest              # the job's own tests with real sanitizers and injected errors
tools/sanitize/nightly.sh           # the job for cron or a systemd timer: results in sanitize-results/<UTC time>/
go run ./tools/sanitize run -mode asan tests/run/maps_basic.gp     # one program, one mode
```

Run it in the dev shell (`nix develop`). On a shared machine start the heavy commands through the
machine's slot script (`SANITIZE_RUNNER="bash heavy.sh" tools/sanitize/nightly.sh`) and size `-j`: a
program is a C++ build (about 5 CPU seconds with the runtime already compiled for the mode, a minute
without).

## What runs

| mode | build | catches | reports |
|---|---|---|---|
| `asan` | `gocpp build -asan` (`-fsanitize=address,undefined`) | heap and stack errors, use after free, double free, stack overflow (AddressSanitizer); every UBSan check (UBSan, `halt_on_error=1`); leaks at exit (LeakSanitizer, `detect_leaks=1`) | `asan`, `ubsan`, `lsan` |
| `ubsan` | `-fsanitize=undefined -fno-sanitize-recover=undefined`, the real allocator | the same UBSan checks without ASan's layout: a report ends the program | `ubsan` |
| `tsan` | `gocpp build -race` (ThreadSanitizer with Go's report, [race.md](race.md)) | data races in the program and in the runtime | `tsan` |

LeakSanitizer is part of the `asan` build (as in `make ci-sanitize`), but its reports are classified and
audited on their own (below). MemorySanitizer is not used: GCC has none, and clang's needs an
instrumented standard library. The sanitizers' options are the job's, not the shell's: the tool drops
`ASAN_OPTIONS`, `UBSAN_OPTIONS`, `TSAN_OPTIONS`, `LSAN_OPTIONS`, `GORACE`, `GOMAXPROCS` (`GOMAXPROCS=1` would take interleavings away
from ThreadSanitizer), `GODEBUG` and every `GOCPP_*` variable from the environment (the runner sets the ones it needs:
`GOCPP_GATE_PHASE=15`, `GOCPP_MODCACHE=off`, `GOCPP_OFFLINE=1`, `GOCPP_WORKLOST=fail`, `GOCPP_CXX`). `-asan-options`,
`-ubsan-options` (both colon separated) and `-tsan-options` (space separated) add options after the mode's. The `asan`
mode sets `UBSAN_OPTIONS=halt_on_error=1` as well, so a UBSan report stops the program there too (a test pins both
strings).

A run **fails** (exit 1) on: a sanitizer report in the program's stderr (also when the program ends with
status 0), a build failure, a timeout, an exit status other than the program's `// ExitCode:`, a canary
that was not reported. It is scanned from the text the sanitizers print: `==pid==ERROR: AddressSanitizer:`,
`file:line:col: runtime error:`, `==pid==ERROR: LeakSanitizer:`, `WARNING: DATA RACE` (Go's form) and
`WARNING: ThreadSanitizer:`. A Go panic (`panic: runtime error: ...`) is not a report, and neither is
LeakSanitizer's `Unable to get registers from thread` warning. Besides these lines there are catch-alls for a
sanitizer that stops itself or prints only a `SUMMARY:` (`CHECK failed`, `DEADLYSIGNAL`, `==N==ERROR:
UndefinedBehaviorSanitizer: SEGV`, which clang's UBSan-alone build prints), so that a program with `// ExitCode: 1` cannot hide
them. The tool reads the program's stdout too, and keeps the first and the last half of a 4 MiB limit of each stream,
so a report at the end of a long output is seen.

**What `tsan` does not report.** `gocpp build -race` sets TSan's options to `detect_deadlocks=0 report_thread_leaks=0
report_signal_unsafe=0 die_after_fork=0 atexit_sleep_ms=0 exitcode=66` ([race.md](race.md); the deadlock detector aborts a
thread that holds more than 64 locks, and a goroutine is a thread that may outlive `main`). So **lock-order inversions
and thread leaks never appear in the `tsan` stage**; only data races do. `-tsan-options "detect_deadlocks=1"` turns the
detector on for a run (TSan reads `TSAN_OPTIONS` after gocpp's defaults) when someone looks for one.

## Corpora

`-corpus` names the programs, or give paths:

| corpus | programs | default for |
|---|---|---|
| `run` | every program of `tests/run` | `asan`, `ubsan` |
| `concurrency` | the programs of `tests/run` that start a goroutine or use a channel, `select`, `sync` or `time` (the list of `tests/race/concurrency.sh`) and the silent programs of `tests/race` (`ok_*.gp`) | `tsan` |
| `race` | the silent programs of `tests/race` | |
| `sample` | the 14 programs of `tools/sanitize/sample.txt` | `nightly -sample` |
| `canary` | `tools/sanitize/testdata/canary` | the `canary` stage |

The headers of the programs select what runs: `// phase: NN` above `-phase` is skipped (the default is 99: every
phase, unlike `make ci`, which stops at `CI_TEST_PHASE`; the output counts what `-phase` skipped), `// pending:` is skipped (`-pending` runs it), and so are `//go:build` lines that exclude
the machine, `// pkg-config:` programs (they need C libraries: the `deep` stages runs the interop tests)
and `// display: required`. The tool prints what it skipped, by reason. `-shard i/n` takes a stable
part of the corpus (a hash of the path), `-filter regexp` and `-limit n` narrow it.

The tool leaves the interop, diff, zoo and example programs and the emitter's own tests to the `deep`
stage (`make ci-sanitize` and `make ci-race`, with `-deep`): they need the test harness's expectations
and, for the interop tests, C libraries.

## Results

`-out` (default `sanitize-results/`, git-ignored) gets `STAGE-MODE-COMPILER.json` for each run (the `run` command and the stage of the same name as its mode have no stage in the name: `asan-g++.json`; the `canary` stage's `canary-asan-g++.json` does not overwrite it) (the counts and one
record per program: status, reports, leak size, exit status, time), `logs/STAGE-MODE-COMPILER/PROGRAM.log` for each
failure (the command, the reports, all of stdout and stderr) and, for the nightly job, `nightly.md` (the
stages as a table). Statuses: `ok`, `report`, `build-failed`, `bad-exit`, `timeout`, `canary-silent`,
`not-run` (a canary of a kind the mode cannot make). Exit status: 0 clean, 1 a failure, 2 a usage error.
`go run ./tools/sanitize scan FILE` prints the reports in a file (for the output of some other run).

## Canaries

A job that finds nothing must not be mistaken for a job that sees nothing. `tools/sanitize/testdata/canary`
has programs that are wrong on purpose, with a header `// canary: KIND`:

| program | error | kind | modes |
|---|---|---|---|
| `canary_asan.gp` | writes one `int` past the end of a `malloc`ed block (in a C header) | `asan` | `asan` |
| `canary_ubsan.gp` | shifts an `int` by 40 | `ubsan` | `asan`, `ubsan` |
| `canary_lsan.gp` | drops the only pointer to a `malloc`ed block | `lsan` | `asan` |
| `canary_tsan.gp` | two goroutines add to a shared counter | `tsan` | `tsan` |
| `clean_c.gp`, `clean_goroutines.gp` | none (controls) | | all |

The `canary` stage runs them in every mode with both compilers: a canary whose report is missing is
`canary-silent` and fails the night, and so does a control that is reported. `make sanitize-selftest`
(`GOCPP_SANITIZE_E2E=1 go test ./tools/sanitize`, a few minutes) builds the same programs **without** the
canary line and requires the run to fail with the right report in each mode, so the tool is shown to fail
on an injected UBSan, ASan, LeakSanitizer and ThreadSanitizer error.

## Leaks and the leak audit

gocpp frees by reference counting (ADR-004), so **a cycle that the program drops is never freed**
(divergence D1 of [the language spec](../spec/language.md)): a closure that calls itself through the
variable that holds it, two structs that point at each other, a doubly linked list, a `sync.Cond` whose
`Locker` is a field of the struct that holds the Cond. LeakSanitizer reports every one of them at exit.
The policy:

1. **A leak is a bug unless it is a reference cycle that the program makes.** Memory that C or C++
   code allocates and never frees, a buffer or a goroutine's state that the runtime keeps, a container
   that grows: none of these is a cycle, and none may carry a marker. Fix them.
2. **A test that leaks a cycle says `// lsan: cycle`** in its header ([tests/README.md](../../tests/README.md#sanitizers)).
   The harness then runs it with `detect_leaks=0`. The marker hides *every* leak of the program, so
   keep a marked program small, and test the rest of a feature in an unmarked one.
3. **Every marker is listed in `tools/sanitize/leaks.txt`**, with a kind of cycle, a reason, and a
   ceiling for each compiler: the most the program may leak, as bytes and allocations as LeakSanitizer
   counts them. The policy test (`TestLeaksFileMatchesTheTree`, part of `make test`) fails when a
   marker has no line, a line has no marker, a kind is unknown, a reason is missing or still `TODO`, or a
   compiler has no ceiling.
4. **The audit** (`go run ./tools/sanitize leaks`, the `leaks` stage, `make sanitize-leaks`) builds each
   listed program with `-asan` and LeakSanitizer **on**, with both compilers, and fails when
   - it leaks more than its ceiling plus its slack (the audit runs both compilers by default) (a leak on top of the cycle: find it, or if it is
     more cycle, say so in the review and run `leaks -update -allow-grow`),
   - it leaks nothing (the cycle is gone: remove the marker and the line, so that a leak that comes
     back is reported), or
   - it makes any other sanitizer report.

   A leak below the ceiling is a note (`leaks -update` lowers the ceiling).
5. **`slack=BYTES/ALLOCATIONS`** on a line is for a program whose leak differs from run to run because
   its goroutines interleave differently. Make the program deterministic first (see below); the byte slack
   is at most 4 KiB and the allocation slack must be 0: one more allocation is the sign of a leak.

Line format (`#` comments, `|` separated):

```
tests/run/closures_recursive.gp | closure-self | g++=288/8 clang++=288/8 | fib calls itself through the variable that holds it
```

Kinds: `closure-self` (a recursive closure holds itself), `mutual-pointers`, `doubly-linked`, `cond-locker`,
`self-pointer`, `cyclic-data`, `context-tree` and `mixed` (several, the reason lists them).

**To add a marked test:** write the test, add `// lsan: cycle` to its header, run
`go run ./tools/sanitize leaks -update -cxx g++,clang++ -filter THE_TEST`, then replace the `mixed`/`TODO` line that it
adds with the kind and the reason, and have the reviewer look at the ceiling: a test that leaks kilobytes
for a three-line cycle is leaking something else. To **raise** a ceiling because the runtime allocates
more, run `leaks -update -allow-grow` and read the diff of `leaks.txt`.

Known weak spots of the list (the ceiling is all that bounds them): `context_gotest_x.gp` leaks 35 KB in 175
allocations, all of it hanging off context trees that the test never cancels (43 of the 47 leak stacks go
through `cancelCtx.propagateCancel` and `Done`), which is the cycle by design. **Every ceiling is exact.** Two
programs used to leak a different amount from run to run, and were made deterministic instead of given a slack:
`context_gotest_x.gp` made one `Done` channel only when its propagating goroutine ran before the cancel (174 or 175
allocations; the test now makes the channel itself before the cancel), and `sync_cond_queue.gp` leaked a queue slice
that grew as far as the interleaving let it (200 to 800 bytes; the queue now starts with room for all 3000 items, so
it leaks 24,192 bytes in 3 allocations every time). 160 runs of each under load, with both compilers, gave one value.
A `slack=` stays possible (at most 4 KiB, never in allocations; the policy test enforces both), but no line has one. The
end-to-end tests inject a 77-byte leak into both programs and require the audit to fail, which only works because
the ceilings are exact. A program that creates contexts and never calls `cancel` leaks them under reference
counting, where Go's collector would free them (`go vet`'s lostcancel asks for the call anyway).

## The nightly job

`sanitize nightly` runs the stages in this order and goes on after a failure, so that one night reports
everything; the exit status is 1 when any stage failed:

| stage | what | note |
|---|---|---|
| `canary` | the canary corpus in `asan`, `ubsan` and `tsan` | seconds |
| `leaks` | the check of `leaks.txt` against the tree, then the audit | a minute |
| `asan` | `tests/run` in the `asan` mode | |
| `ubsan` | `tests/run` in the `ubsan` mode | |
| `tsan` | the `concurrency` corpus in the `tsan` mode | |
| `deep-sanitize` | `make ci-sanitize` | only with `-deep` or by name (`-stages deep` names both) |
| `deep-race` | `make ci-race` | the same |

Each stage runs with `g++` and `clang++` (`-cxx`). Useful flags: `-sample` (the sample corpus instead of
the full ones), `-stages a,b`, `-shard i/n` (an empty shard of a small corpus is not an error: the tool says whether the shard has no program or all its programs were skipped by `-phase`, a build constraint or `// pending:`, in which case nothing was checked), `-j N`, `-limit N`, `-phase N`, `-gocpp-cache DIR` (keep the compiled
runtime and objects between nights; `nightly.sh` does), `-n` (check the flags and print the plan).

**On this machine** use `tools/sanitize/nightly.sh` from cron or a systemd timer. It tests the commit that `HEAD` is on, in a
snapshot (a detached `git worktree` of `HEAD` inside the night's folder), so editing the repository during the
night changes nothing and uncommitted work is not tested (`SANITIZE_LIVE=1` tests the live tree and records a dirty tree);
the commit goes to `commit.txt` and `history.txt`. The script builds `tools/sanitize` first and runs the binary, so the
exit status is the tool's (0 clean, 1 a stage failed, 2 a usage error or a night that could not start). A night that is
killed (TERM, INT, HUP: a `systemctl stop`, a terminal) stops the job, removes the snapshot and the worktree entry and writes
its line (`exit 143 ... (killed by TERM)`); a night that was killed hard (`kill -9`, a power cut) leaves the snapshot, and the
next night removes it (only `<results>/*/src` folders that are git worktrees). `flock` keeps two nights from overlapping (the
second one exits 2 and says so in `history.txt`, without a folder of its own). The phase is not limited unless `SANITIZE_PHASE`
is set. For example

```
# ~/.config/systemd/user/gocpp-sanitize.service
[Service]
Type=oneshot
WorkingDirectory=/home/you/GoToCPP
ExecStart=/home/you/GoToCPP/tools/sanitize/nightly.sh
# ~/.config/systemd/user/gocpp-sanitize.timer
[Timer]
OnCalendar=*-*-* 02:17:00
Persistent=true
[Install]
WantedBy=timers.target
```

The script writes `sanitize-results/<UTC time>/` (the JSON, the logs, `log.txt`, `nightly.md`) and one line
per night to `sanitize-results/history.txt`; a failed night prints where to look and exits 1. Nothing in
the repository installs a timer.

**On GitHub** `.github/workflows/sanitizers-nightly.yml` does the same with `make sanitize-nightly`, in jobs:
the canaries and the leak audit; `asan` and `ubsan` in four shards; `tsan` in two; `deep-sanitize` and `deep-race` (two jobs, because a GitHub-hosted job stops after 360 minutes). It is not active:
decision Q9 keeps the repository off GitHub for now, so the file was only checked for YAML and for the make
targets and flags it uses (`TestJobFilesUseRealFlags`), not run by GitHub.

## Time and size

Measured on this machine (2026-10-19, 192 cores shared with other work, `-j 10`, the runtime of each mode
already compiled): a program takes about 5 s in `asan` (the slowest 21 s), 1.4 s in `ubsan` and 9 s in
`tsan` (the slowest 34 s), build and run together. `tests/run` has 1944 programs and the `concurrency`
corpus 513, so the full job is roughly 20,000 s + 5,500 s + 9,500 s, about 10 hours of program time for the
two compilers, or one to two hours on 8 to 16 workers; a first run also compiles the runtime once per
mode and compiler. `make sanitize-smoke` (14 programs x 2 compilers x 3 modes, the canaries, the audit of
18 programs x 2 compilers) takes about 1.5 minutes. Spot checks run for this step (2026-10-19, no
findings): 73 programs of `tests/run` with `asan`/g++, 72 with `ubsan`/clang++, 61 concurrency programs
with `tsan`/g++ (shards `0/25`, `1/30`, `0/8`), 2 minutes in all. The `deep` stage adds `make ci-sanitize`
(about 50 minutes on 2026-10-18: the test programs 20, the emitter's tests 28) and `make ci-race`
(not measured).

## Not covered

- Interop programs, the diff and zoo tests and the emitter's tests: the `deep` stages.
- Windows: `gocpp build -asan` and `-race` for a Windows target (also from Linux with `-os windows`) are refused with a message that says so: the MinGW GCC of WinLibs, MSYS2 and the dev shell has no sanitizer runtime, and ThreadSanitizer does not exist for MinGW. clang's MinGW target has an AddressSanitizer that gocpp has not tried. Windows is tested on the real PC instead.
- Linux arm64 (step 15.12) and macOS (not a target).
- ThreadSanitizer's own limits ([race.md](race.md): a race that is a pair of single accesses can be
  missed in a run, a program with thousands of live goroutines can hit TSan's thread limit).
- A program that needs a display or a C library (skipped by header).

## If the job fails

1. Read `logs/STAGE-MODE-COMPILER/PROGRAM.log`: the command, the report, the program's output.
2. Reproduce one program: `go run ./tools/sanitize run -mode asan -cxx g++ -v tests/run/NAME.gp`.
3. A failure only with one compiler or mode is real until shown otherwise; do not add a suppression. A
   false positive of the runtime needs a fix in the runtime or an entry in `runtime/tsan.supp` with the
   reason ([race.md](race.md)).
4. A `canary-silent` is a broken job, not a green night: look at the options (`mode.go`), the compiler
   flags, `ASAN_OPTIONS` in the environment of whatever starts the job.
