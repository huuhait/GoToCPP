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

**Twins (Phase 10).** A race test that uses macros (`ok_macro_locked`, `ok_macro_locked_panic` (the `Locked` macro on a method that panics: the unlock is the macro's exit part and runs on the panic), `ok_macro_wrap_threads`, `ok_macro_wrap_spike` (the wrap spike's `Trace`, `VM`, `Recover`, `InThread` and `Once` called from 16 goroutines), `race_macro_unlocked`) cannot be built by Go. Its twin is
the same program with the macros written out as functions (`testdata/twin/NAME.go`), and `TestRaceVsGo` builds the twin with
`-race`, so real Go's detector gives the verdict: silent for the `ok_` ones, and a report at the same line for `race_macro_unlocked`
(the twin keeps the racy statement on a line the `// Stderr:` regexp accepts).

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
threads do not pile up; `ok_stack_all_churn` that `runtime.Stack(buf, true)` (the report of all goroutines that a
panic with `GOTRACEBACK=all` also makes) called by three goroutines at once while goroutines start and end reports no race: the
registry that it walks is the runtime's bookkeeping, not a race. `ok_stack_all_parked_churn` is the same
with goroutines that sleep and then wait on a channel while they are dumped: the dump posts a request to each parked
goroutine's own wait, and the goroutine frees its record afterwards, which the race detector must see ordered (the
runtime gives it that edge by hand: `race::release` at the end of a report, `race::acquire` before a goroutine frees
its record). `race_stack_all_user_race` is its control: two goroutines that really race on a counter are still
reported while a third dumps all stacks (the dump hides only the runtime's own records, and the edge
keeps the order between the other goroutines visible; it does not unhide the order the dump itself makes: a real
race between the dumper and a goroutine that was in a short sleep when it was dumped (up to about 150 ms in our runs) can go unreported, a limit
in docs/guides/race.md; no `limit_` test pins it, because it depends on timing: with the goroutine asleep for 30 ms and the dump at
5 ms the race went unreported in 5 of 5 runs (sleeps up to about 150 ms: 0 of 4 or 5; 200 ms or more: reported; a channel wait: reported), the same on both compilers, with 300 ms and 50 ms it was reported in 5 of 5, so a test of the
miss would be flaky). One run of `ok_stack_all_parked_churn` (about 10 s) fails for a build without the acquire, or without the release, in 20 of 20 runs on each compiler (standalone; with a sleep in place of the `Gosched`, 13 of 20 and 18 of 20 at 40 and 100 rounds). Mutants of the edge that survive are narrow-window orderings, accepted:
the acquire only after a wait for the lock, and the release only after the unlock. The failing-barrier note
was checked end to end for the panic path; `Stack(buf, true)` shares the function and is covered by the
unit test only. The lazy creation of the
line table index and of the function-name memo under their lock (instead of a function-local static's
initializer, whose guard ThreadSanitizer cannot order in a `-race` program) is sound by construction; one run
in twelve of a two-goroutine first trace showed the false race before it, but no test makes it deterministic, so
none claims it.

Every program is valid Go, and `GOCPP_RUN_VS_GO=1 go test ./tests -run RaceVsGo` builds each with
`go build -race` and checks the same expectations against real Go's detector (the output, the exit
code and the `// Stderr:` regexps), so the report's form is Go's and not what gocpp believes
Go's is. (It skips the known limits, `race_sixty_vars` (Go reports a different number of the sixty races in each run) and `race_finished_then_goroutines`, which real Go loses too: its detector reuses the slot of a goroutine that has ended for the next goroutine.) The programs that race on a map put a sleep between the
goroutines: two accesses at the same time are the runtime's "fatal error: concurrent map writes", in Go
and in gocpp, and not a race report.

`go test ./tests -run '^TestRace$'` runs the suite; `make ci-race` runs it with both compilers
and runs the concurrency tests of `tests/run` under `GOCPP_FLAGS=-race`, which must all be silent.
