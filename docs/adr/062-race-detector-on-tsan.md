# ADR-062: `-race` is ThreadSanitizer, with Go's report written by a process of the runtime

Status: accepted (2026-10-19)

## Context

Go programs are tested with `go test -race`. gocpp's goroutines are OS threads (ADR-007), channels,
`select` and the `sync` types are built on `std::mutex` and `std::atomic` (ADR-026, ADR-029,
ADR-040), and a `*T` is a `std::shared_ptr` (ADR-004). ThreadSanitizer (TSan) understands all of
those, so a race detector costs a compiler flag; step 6.12 made that flag (`-race` is a mode of the
build, ADR-044). Step 8.12 had to decide what the report looks like (TSan's C++ text or Go's), and
what the runtime must do so that the detector finds every race of the Go memory model and no other.

What was found by building it, and the decisions below follow from it:

1. TSan calls `__tsan_on_report` with its own locks held, and **a thread that allocates there hangs**
   (the allocation, or an instrumented atomic, can want a lock that the thread holds). The hang
   depended on the layout of the stack and on the environment: a program hung in 8 runs of 9 in a
   batch, and never in a debugger.
2. TSan keeps what a thread did in the thread's trace (32K events by default), and needs the trace of
   the thread of an earlier access to name it in a report; **a race whose earlier access it cannot
   restore is dropped without a word**. The trace of a thread that has ended is the first that TSan
   gives to another thread that needs room (every memory allocation spends some), so a goroutine
   that allocated 300 times made a race with the earlier access of a goroutine that had ended
   invisible (a C++ test of 30 lines shows the same at about 20,000 allocations). So anything that
   works a lot in the program's process (reading the 7 MB program file, naming thousands of symbols,
   formatting 60 reports) must not, and the thread of a finished goroutine must not end.
3. **The runtime's own atomics order goroutines that Go does not order**, and a race that such an
   edge hides is missed (and found only when the timing is lucky): the counts of live, started and
   blocked goroutines, the lock of the idle thread list, `std::shared_ptr`'s reference counts
   (libstdc++ changes them with `__ATOMIC_ACQ_REL`), and the atomics of the words of `sync` objects
   (a `WaitGroup.Add` after a `Done` acquires everything the goroutine of the `Done` did; Go's model
   has only Done before Wait).
4. TSan's own symbolization of a report is slow: with clang an external `llvm-symbolizer` that it
   starts while it holds its locks, seconds for a report. gocpp needs only addresses.

## Decision

- **`-race` is the flag of 6.12** (`ProfileFlags`): `-fsanitize=thread -fno-omit-frame-pointer` in every
  compile (the runtime's too: they are in the key of the runtime's objects, as the flags of the
  environment are) and `-fsanitize=thread` in the link; on Linux also `-DGOCPP_RACE_MODE=1` (compile)
  and `-Wl,--export-dynamic-symbol=__tsan_default_options` (link). The define switches the runtime to
  the race mode (`gp::race::mode`), and `gocpp/nofortify.h`, the first header of every compile of the
  program and of the runtime, includes `gocpp/race_atomicity.hpp` then. `-race` with `-asan` or
  `-release` is an error, and so is `-race` for windows. On darwin it is TSan alone: no Go-form
  reporter (its OS layer is Linux's), TSan's own text. `GOCPP_FLAGS=-race` runs the test programs with it.
- **Go's report, not TSan's.** `init` (the OS layer, `runtime/src/os/linux/race.cpp`) points TSan's
  report file at an anonymous file, makes an anonymous shared mapping with a ring of records, and
  **forks the reporter**, a process that is a copy of the program (so it has its addresses) and
  needs no TSan lock. `__tsan_on_report` copies each report into a record (the accesses, the threads
  and the stacks that created them, as TSan's report API gives them), with code that TSan does not
  instrument and system calls only (no allocation, no libc function, no `std::atomic`: decision 1), and
  **waits for the reporter to write it**, as Go prints a report before the program goes on (a race that is
  followed by a crash, a signal or an exit is on the screen; the wait has a limit of 3 s). The reporter
  formats the record as Go's text (`WARNING: DATA RACE`, `Write at 0x... by goroutine 7:`, `Previous
  write at ...`, `Goroutine 7 (running) created at:`, the frames through the code of the panic trace:
  Go names and `.gp` lines), writes it to stderr with one `write`, and drops a report whose two
  accesses have the stacks of one already written (decision 2 is why this is a process; Go also
  prints a race once). It reads the program's file and its line table before the program's `main`
  starts (the program waits, about 50 ms), so that the first report is a lookup. It ends when its
  parent does (`PR_SET_PDEATHSIG`). The thread of a goroutine is named `goroutine <id>` (TSan's
  `AnnotateThreadName`), and `main` is the main goroutine.
- **A finished goroutine's thread parks** (`gp::race::finished_goroutine`, `race.cpp`): in a ring of
  the last 64 finished goroutines (`GOCPP_RACE_KEEP=N`; 0 for none); the goroutine that finishes 64
  places later releases it. This keeps the trace of what the goroutine did (decision 2): the
  repro (`race_finished_then_alloc`) was lost after 300 allocations and is found after a million. The
  price is memory and time: a million goroutines, 1,000 at a time, took 124 s and 407 MB with 64
  places, 94 s and 370 MB with 16, 127 s and 1.7 GB with 256 (the first guess), and 63 s and 221 MB
  without parking, which is why the default is 64. A program with more than 64 goroutines finishing
  between the two accesses of a race can still lose the race (`race_finished_then_goroutines`:
  found with 30, lost with 100). `atexit_sleep_ms=0` is in the defaults, since TSan would wait a
  second at the end of every program that has threads. Real Go's detector loses the second case too
  (it reuses the slot of an ended goroutine).
- **The files that name frames are compiled without TSan** (`rtNoTSan` in `internal/build/plan.go`:
  the trace code and `race.cpp`), because they also run in the program's process (a `panic`'s trace).
- **TSan does not symbolize** (`symbolize=0` in the defaults, unless `GOCPP_RACE_RAW` or a `log_path` asks
  for TSan's text): decision 4.
- **How the program ends is Go's.** A program that ends with 0 after a race ends with 66 (TSan's
  `exitcode`) and prints `Found N data race(s)` (the number of reports written, plus the ones that did
  not fit in the ring); one that ends with another code (a panic: 2, `os.Exit(3)`) keeps its code and
  prints no count (TSan's `_exit` leaves a status other than 0 alone). With `GORACE=halt_on_error=1` TSan dies right after the callback, and its death callback
  gives the reporter a moment, so the report is Go's (and, as in Go, no count follows it).
- **`GORACE` is TSan's options under Go's names** (`halt_on_error`, `exitcode`, `history_size`,
  `log_path`, `strip_path_prefix` (also applied to Go's form)): `__tsan_default_options` (which TSan
  calls before it can run instrumented code, before libc has set `environ`: raw system calls, read
  from `/proc/self/environ`) returns gocpp's defaults, then `GORACE`; `TSAN_OPTIONS` is read after and
  wins. The defaults are `detect_deadlocks=0` (TSan's deadlock detector aborts above 64 locks held by
  a thread, as a `select` over many channels does, ADR-029; gocpp reports a Go program's deadlocks
  itself), `report_thread_leaks=0` (goroutines that run when `main` returns are normal),
  `report_signal_unsafe=0`, `symbolize=0`, `exitcode=66`.
- **The reference counts do not order goroutines** (`runtime/include/gocpp/race_atomicity.hpp`). It
  takes libstdc++'s own header `ext/atomicity.h` out (by its include guard) and defines its
  functions again: the same `__atomic_fetch_add(..., ACQ_REL)` inside TSan's "ignore synchronization"
  scope, so the program's real behavior is unchanged and TSan sees no edge. The only edges kept are
  the ones libstdc++ describes with `_GLIBCXX_SYNCHRONIZATION_HAPPENS_BEFORE/AFTER` (defined as
  `__tsan_release` and `__tsan_acquire`): every drop before the last, which frees the object. A
  libstdc++ that renames the guard fails the build or the tests; libc++ is refused with a message.
- **The runtime's bookkeeping is not synchronization either** (`gp::race::no_sync`, an RAII
  `AnnotateIgnoreSyncBegin/End`): the counts in `src/goroutine.cpp`, and the atomics of the words of the
  `sync` objects (`gp::sync::detail::ref`). The edges of Go's memory model for those objects are put
  back by hand, as Go's sync package does with its race annotations (`gp::race::release/acquire` on the
  address of the object): Mutex (Unlock to Lock), RWMutex (Unlock to Lock and RLock, RUnlock to Lock),
  WaitGroup (Done to Wait), Once (the function's work to every Do that returns after it), Cond (Signal
  and Broadcast to the Wait). With `-race` no thread is reused for a second goroutine (the idle list's
  lock would order the end of one before the start of the next), which also makes a report's "created
  at" the `go` statement of that goroutine.
- **Channels keep one mutex per channel**, so TSan orders every operation on a channel with the one
  before it: two senders, or two receivers, are ordered for TSan though not for Go, and a race between
  what they do around the operations is missed. The explicit edges of Go's channel model need the
  channel's own state to be invisible to TSan (its mutex, its queues) and its element copies visible; that
  is a rewrite of `chan.cpp`/`select.cpp`, left for a step of its own. The limit is documented and pinned
  by `tests/race/limit_chan_*.gp`.
- **Tests.** `tests/race` (`TestRace`): programs that must be reported and programs that must stay
  silent, with both compilers; `TestRaceVsGo` (opt-in: `GOCPP_RUN_VS_GO=1`) builds the same programs
  with `go build -race` and checks the same expectations, so the form of the report and the exit code
  are Go's. `make ci-race` (nightly) runs them, the whole concurrency family of `tests/run` with
  `GOCPP_FLAGS=-race`, and the runtime's TSan unit tests; `make ci` runs `TestRace` as a part of
  `./tests`.

## Alternatives

- **Format the report in `__tsan_on_report`, or in a thread of the program.** It hangs (finding 1),
  and a thread that formats spends TSan's budget (finding 2): the first version of this step missed
  races after its own reports.
- **Format when the program ends.** A server never ends, so its races would not be seen; a program
  that is killed would show nothing.
- **Annotate each reference-count operation of gocpp's code.** `std::shared_ptr` is used in a
  hundred places (slices, interfaces, func values, maps); the operations are inside libstdc++'s inline
  code; replacing the library's three helper functions covers all of them.
- **`relaxed` counts in `src/goroutine.cpp`.** It would change the production build for the
  sake of the detector (the deadlock check reads the counts with acquire); the scope costs nothing
  without `-race`.
- **A TSan suppression file for the runtime.** `runtime/tsan.supp` exists, empty: every false
  positive that the family of `chan_*`, `go_*` and `select_*` tests showed was a runtime bug of the
  kinds above and was fixed where it is, and a suppression hides real races of the same function.
- **A second `-race` flag** (the first version had one): the flag, its checks and its tests are 6.12's.

## Consequences

- A Go program is checked for races with `gocpp run -race`, on Linux, with both compilers, with Go's
  report and exit code. TSan costs what it costs (about 5 to 15 times in time, 5 to 10 times in memory,
  and a limit on the threads alive at once, so a program with many thousands of goroutines at once may not run).
- A `-race` program has one more process while it runs (the reporter), and starts about 50 ms later.
- C and C++ code that was not built with `-race` is not seen, and neither are the accesses of the
  kernel and of C libraries.
- A step that adds synchronization to the runtime must build it on `std::mutex` or `std::atomic` with
  the order that the Go memory model needs, or tell TSan (`gp::race::acquire/release`); a counter that is
  not synchronization goes in a `race::no_sync` scope; `tests/race` and `make ci-race` fail otherwise (a
  false positive, or a missed race).
- A race with an access that is older than the thread's trace (a few hundred allocations of the same goroutine at the
  default `history_size`), or of a goroutine that finished more than 64 goroutines ago, can be lost
  (finding 2): limits of TSan, with their numbers, in the guide.

## Evidence

- `tests/race`, `runtime/race_test.go` and `cmd/gocpp/race_test.go`; the mutation checks in the step's
  report (each mechanism broken in a copy: the tests fail).
- `race_wg_sequential`, `race_mutex_other_var`, `race_rwmutex_reader_pair`, `race_rlock_write`
  (finding 3), `race_sleep_sequential` and `race_go_hidden_edge` (thread reuse and the counters),
  `race_sixty_vars` (reports must all be written), `race_at_exit`, `race_then_panic`,
  `race_exit_after_race` (the ends of a program); `race_method_counter`, `race_generic_func` and
  `race_send_then_write` (the reference counts).

## See

- [Phase 08 §8.12](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md), [the guide](../guides/race.md),
  [tests/race](../../tests/race/README.md), ADR-044 (the build flags).
