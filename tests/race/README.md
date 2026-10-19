# tests/race

A race test is a `.gp` program that the harness builds and runs with the race detector
(`gocpp run -race`, [docs/guides/race.md](../../docs/guides/race.md)), once per C++ compiler. It
has the lines of a [run test](../run/README.md) (`// phase:`, `// ExitCode:`, `// Stderr:`,
`// pending:`, `//go:build`, the `// Output:` block), with the same meaning, and three kinds:

- **A program with a data race** (`race_*.gp`) ends with exit code 66 (or the code of its end: 2 for a
  panic after the race, 3 for `os.Exit(3)`, as with Go) and prints Go's report on stderr. Its header
  is `// ExitCode: 66` and `// Stderr:` regexps: the report's first line (`WARNING: DATA RACE`), the
  two accesses with their goroutines (`Write at 0x... by goroutine 7:` ... `Previous write at ...`),
  where the goroutines were created, the `.gp` file and line of the racy access (`race_counter.gp:18`,
  so that the `#line` mapping is checked), and, for a program that ends with 0, the last line
  (`Found 1 data race(s)`). The program prints a line on stdout that does not depend on the race
  (`done`), so that the test also shows that the program was not stopped.
- **A program without a race** (`ok_*.gp`) must exit with 0 and print nothing on stderr: any
  report is a failure. Each uses one of the synchronizations that the Go memory model gives
  (a channel hand-off, a channel as a lock, `close` as a broadcast, a buffered channel as a
  semaphore, `select`, the start of a goroutine, `sync.Mutex`, `RWMutex`, `WaitGroup`, `Cond`,
  `Once`, `sync/atomic`, work on different fields or elements), or work that does not share anything
  but reads; they are what the detector must not report.
- **A known limit** (`limit_*.gp`) is a race that real Go reports and gocpp does not (docs/guides/race.md,
  "Limits"): the test says what gocpp does today (runs silent), so that a change that fixes it
  shows, and `TestRaceVsGo` skips it.

Every racy access is repeated in a loop (100 times; `counter` and a few others already loop): TSan
can miss a race whose two accesses are one single access each, made in the same instant (both threads
read the shadow memory before either writes it), which a flake audit of 10 runs showed in about one
run in ten for programs where one channel operation wakes both racing goroutines together.

How a test finds a race is never the timing: the detector checks happens-before, so two goroutines
that write one variable without synchronization are reported also when one of them is long
finished before the other starts (`race_sleep_sequential`, `race_wg_sequential`: a sleep and a
`WaitGroup.Add` after a `Done` order nothing). The programs wait for their goroutines with channels
that create no edge between the two that race (each one sends to `main`), or with a `WaitGroup`
(only Done before Wait orders anything).

The programs of the runtime's own bookkeeping (`race_go_hidden_edge`: starting a goroutine orders
nothing between two other goroutines; `race_method_counter`, `race_generic_func`: the reference counts of `*T`
order nothing) fail when a counter or a count is not hidden from TSan. `race_sixty_vars` needs every
report to be written, `race_at_exit` that a report is written before `main` returns, and
`race_then_panic`, `race_exit_after_race` how the program ends (no count after a panic or an
`os.Exit(3)`); `race_finished_then_alloc` and `race_finished_then_goroutines` that the threads of
finished goroutines are kept (a race with a goroutine that has ended is found after any number of
allocations, and after 30 other goroutines have finished); `ok_once_config` that `Once.Do` orders
what its function wrote before every `Do` that returns after it; `ok_many_goroutines` that the parked
threads do not pile up.

Every program is valid Go, and `GOCPP_RUN_VS_GO=1 go test ./tests -run RaceVsGo` builds each with
`go build -race` and checks the same expectations against real Go's detector (the output, the exit
code and the `// Stderr:` regexps), so the report's form is Go's and not what gocpp believes
Go's is. (It skips the known limits, `race_sixty_vars` (Go reports a different number of the sixty races in each run) and `race_finished_then_goroutines`, which real Go loses too: its detector reuses the slot of a goroutine that has ended for the next goroutine.) The programs that race on a map put a sleep between the
goroutines: two accesses at the same time are the runtime's "fatal error: concurrent map writes", in Go
and in gocpp, and not a race report.

`go test ./tests -run '^TestRace$'` runs the suite; `make ci-race` runs it with both compilers
and runs the concurrency tests of `tests/run` under `GOCPP_FLAGS=-race`, which must all be silent.
