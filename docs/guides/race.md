# The race detector (`-race`)

`gocpp build -race` and `gocpp run -race` build the program with ThreadSanitizer (TSan). (`gocpp test
-race` is not there yet: the command says "-race is not supported yet".) On Linux a data race that happens in the
run is printed in Go's form, and the program ends with exit code 66 (the same as Go's):

```
$ gocpp run -race counter.gp
==================
WARNING: DATA RACE
Read at 0x55a414ec1530 by goroutine 3:
  main.work()
      /home/me/counter.gp:18 +0x33

Previous write at 0x55a414ec1530 by goroutine 2:
  main.work()
      /home/me/counter.gp:18 +0x4d

Goroutine 3 (running) created at:
  main.main()
      /home/me/counter.gp:26 +0x4f

Goroutine 2 (finished) created at:
  main.main()
      /home/me/counter.gp:25 +0x4f
==================
done
Found 1 data race(s)
exit status 66
```

The frames have Go's function names and the `.gp` file and line (the `#line` directives of the
generated C++). The report is written when the race is found, before the program goes on. A race is
reported once: a second report whose two accesses have the same stacks is not written (Go does the
same). The program runs to its end, unless `GORACE=halt_on_error=1` stops it at the first report.

How the program ends, as with Go: it ends with 66 when it returns from `main` (or calls `os.Exit(0)`)
after a race, and prints `Found N data race(s)`; a program that ends with another code (`os.Exit(3)`, a
panic: 2) keeps its code and prints no count.

`-race` is the same flag as 6.12's mode ([build.md](../spec/build.md)): it adds
`-fsanitize=thread` to every compile (the runtime's too) and to the link, and with it, on Linux, the
report in Go's form. `-race` with `-asan` or `-release` is an error, and so is `-race` for windows. On
darwin `-race` is ThreadSanitizer alone, with its own text and no `GORACE`.

## What is found

TSan watches every memory access of every thread and the synchronization between them, and reports
two accesses to the same memory, at least one a write, that are not ordered by a happens-before
edge. The edges are the ones of the Go memory model:

- a `go` statement happens before the goroutine starts;
- a send happens before the matching receive completes, a receive from an unbuffered channel
  happens before the send completes, the k-th receive of a channel with capacity C happens before
  the k+C-th send completes, `close` happens before a receive that returns because of it;
- a `select` is those operations of the case that was chosen;
- `sync.Mutex` and `sync.RWMutex` (an Unlock before the next Lock, an Unlock before the next RLock, an
  RUnlock before the next Lock; two RLocks are not ordered), `sync.WaitGroup` (a Done before the Wait
  that it lets return, and nothing else: an Add after a Done orders nothing), `sync.Once`,
  `sync.Cond`, `sync/atomic`.

Found: unsynchronized counters, a variable shared with a closure, fields of a struct and elements
of a slice or array, a map written while another goroutine writes or reads it (the map's own
memory), slice appends, an interface or string or pointer variable assigned by two goroutines, a
value that the sender changes after sending it, a write under `RLock`, goroutines that a sleep puts
one after the other. Not reported: accesses ordered by any of the edges above, two goroutines that use
different fields or elements, and many goroutines that only read.

Finding a race does not depend on the timing, because the check is of happens-before and not of
overlap: two goroutines that write one variable without synchronization are reported also when one
of them has finished before the other starts. For that gocpp hides from TSan everything that is
not an edge of Go's memory model but would look like one (see "How it is built").

## GORACE

`GORACE` has Go's options, and they are TSan's options under the same names: `halt_on_error=1`
(stop at the first report; the report is still Go's, and like Go's it has no count after it), `exitcode=N` (instead of 66), `log_path=file`
(TSan's own text of the reports goes to `file.<pid>`, with the C++ names of the frames, and stderr
has no report), `history_size=N` (how far back TSan remembers accesses: 0 to 7, default 0; a bigger
number finds races between accesses that are far apart in time, at a cost in memory),
`strip_path_prefix=...` (also shortens the file names of Go's form). TSan's own `TSAN_OPTIONS` is
read after it and wins. gocpp's defaults are `detect_deadlocks=0` (TSan's deadlock detector aborts
when a thread holds more than 64 locks, as a `select` over many channels does; gocpp reports the
deadlocks of Go programs itself), `report_thread_leaks=0` (goroutines that are still running when
`main` returns are normal), `report_signal_unsafe=0`, `symbolize=0` (the report of gocpp needs only
addresses, and TSan's symbolizer is slow) and `exitcode=66`.

`GOCPP_RACE_RAW=1` shows TSan's own text instead of Go's (with the C++ names of the frames and the
runtime's frames), for debugging the runtime.

## How it is built

- **The report** is written by a process of its own, the reporter, which the program forks at its
  start and which shares a ring of records with it. TSan calls `__tsan_on_report` with its own locks
  held, so the callback copies the report into the ring (with code that TSan does not instrument and
  no libc call) and waits for the reporter to write it. TSan gives each thread a budget of events
  (every memory allocation spends some), and when it is gone it forgets all it knows of the accesses
  that came before: a reporter that read the 7 MB program file in the program's process, or
  formatted the report of each race there, would make the detector miss the races it was started to
  find. The program waits for the reporter to be ready before its `main` starts (about 50 ms).
- **Reference counts.** A `*T` is a `std::shared_ptr`, and its count is changed with atomic
  operations that TSan would take for synchronization (copying a pointer would order the goroutine
  after every goroutine that touched the same object before). `-race` includes
  `gocpp/race_atomicity.hpp` (from `gocpp/nofortify.h`, the first header of every compile) which replaces the library's atomic
  helpers (`ext/atomicity.h`) by the same operations inside TSan's "ignore synchronization" scope and
  declares only the edge that matters (every drop happens before the one that frees the object).
  It is a part of libstdc++'s internals that it relies on (the header's include guard); a new
  libstdc++ that renames it makes the build fail, with libc++ the build says so. The weak pointers'
  `lock` is not covered.
- **The runtime's bookkeeping** is hidden the same way (`gp::race::no_sync`, runtime/include/gocpp/race.hpp):
  the number of live, started and blocked goroutines, and the atomics of the words of `sync`
  objects (`sync.Mutex`, `RWMutex`, `WaitGroup`, `Once`, `Cond`), whose edges are put back by hand
  (`gp::race::release` and `acquire`, as Go's own sync package does with its race annotations).
  With `-race` no thread is reused for a second goroutine, so that the report of a goroutine says where
  *that* goroutine was started.
- **The code that names the frames** (the files of the runtime that read the program's file and
  its line table) is compiled without TSan, so that it spends none of TSan's budget either.

## Limits

- **Channels.** A channel is one mutex, and TSan sees every operation on it ordered with the one
  before: two goroutines that send to one channel, or receive from one, are ordered for TSan
  although Go does not order them, so a race between what they do around the operations is not
  reported (`tests/race/limit_chan_two_senders.gp` and `limit_chan_two_receivers.gp` pin this: real Go
  reports both). Races through the element, and the edges of the list above, are exact.
- **`sync.Cond`.** The tickets of `Wait` are atomics that TSan sees, so two goroutines that `Wait` on
  one `Cond` are ordered for TSan. The slow paths of the sync objects (a goroutine that sleeps in
  a lock) order the goroutines that use the same semaphore table bucket.
- **Threads.** A goroutine is a thread, and a thread takes longer to start than a goroutine does: a program
  whose second goroutine would race with the first, but that ends or is killed before it runs, has
  no race to report. TSan limits the number of threads that are alive at once, so a program with many
  thousands of goroutines at once may not run under `-race`.
- **Cost.** TSan makes a program 5 to 15 times slower and uses 5 to 10 times the memory, as in Go.
- **A simultaneous pair.** The detector keeps the last few accesses of each 8 bytes in a shadow cell.
  Two threads that make their one and only access to a variable in the very same instant can each
  read the shadow before the other has written it, and neither sees the other (Go's detector has this
  limit too, it is TSan's). A race that is a pair of single accesses can therefore be missed in one run
  of a hundred; one access in a loop, or a second run, finds it. The programs of `tests/race` repeat
  every racy access 100 times for this reason.
- **Goroutines that have ended.** TSan keeps what a thread did in the thread's trace, and needs the
  trace of the thread of an earlier access to name it in a report. When a thread has ended, its
  trace is the first that TSan gives to another thread that needs more room, and a race with an
  access of the ended thread is then dropped, without a word: after a few hundred allocations of the
  other goroutine, in the worst case (`tests/race/race_finished_then_alloc.gp`). So with `-race` a
  goroutine's thread does not end when the goroutine does: it parks, and the 64 last finished
  goroutines are kept (`GOCPP_RACE_KEEP=N` changes the number; 0 turns it off). The race with a
  goroutine that finished is found as long as fewer than 64 other goroutines have finished since
  (`race_finished_then_goroutines.gp`: 30 found; 100 lost), however much the program does in between
  (a million allocations: found). The price is memory and time for the parked threads (TSan's state
  and the stack): a program that starts and ends a million goroutines, 1,000 at a time, took 124 s
  and 407 MB with the default of 64, 94 s and 370 MB with 16, 127 s and 1.7 GB with 256, and 63 s and 221 MB
  without parking. With TSan's `history_size` the limit moves a little (7: 1,000 allocations instead of
  300) and does not go away.
- **Old accesses.** A thread's trace holds its last 32,768 memory events (`history_size=0`;
  `GORACE=history_size=N` multiplies that by 2^N, up to N=7), and a race with an access that is
  older than that is dropped without a word. In gocpp's runtime, which is instrumented too, an
  allocation (`make`, `append`, a string) is hundreds of events: measured with a goroutine that
  writes a variable, makes n allocations and sleeps, and another that writes the variable later, the
  race is found for n = 100, lost for n = 200; with `history_size=7` it is found for n = 400 and lost for
  n = 1000. (Go's detector has the same limit with its own numbers; its default is
  `history_size=1`.) A program that does a lot between the two accesses of a race needs a bigger
  `history_size` (it costs memory in the threads that work a lot) or a second look at the code.
- **What is not seen.** A race that does not happen in this run; a race in C or C++ code that was
  not built with `-race` (its accesses are not seen; the library of the C++ standard is,
  through its headers); a race on memory that a C library or the kernel writes (read, recv).
- **Suppressions.** `runtime/tsan.supp` is TSan's suppression file for the runtime itself, with a
  reason for each entry; it is empty if the runtime has no known false positive. gocpp does not
  suppress anything in the code of a program.

## In CI

- `make ci` runs the race suite (`tests/race`, `go test ./tests -run TestRace`) with one compiler,
  and the TSan unit tests of the runtime (`go test ./runtime -run TSan`).
- `make ci-race` (the nightly job) runs `tests/race` with g++ and clang++, and the whole
  concurrency family of the run tests (`chan_*`, `go_*`, `select_*`, `sync_*`, `atomic_*`, `time_*` and every
  other program that starts a goroutine) with `GOCPP_FLAGS=-race`: every one of them must be silent.

`tests/race/README.md` describes the suite: a program that must be reported has
`// ExitCode: 66` and `// Stderr:` regexps for the report, a program that must stay silent has
none.
