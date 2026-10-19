# ADR-078: `cmp`, `slices`, `maps` and `iter` are Go's own packages, and `iter.Pull` runs on a goroutine that never runs beside its driver

Status: accepted (2026-10-20)

## Context

Step 7.10 ports Go's four generic packages. Their prerequisites are merged: generic functions (7.1, 7.2), generic types (7.3, for
`iter.Seq[V]`), range over functions (7.8), `tools/portstd` (9.1) and the ports of 9.4a/9.4b. Until now they were declaration stubs
(`Real: false` in `tools/stubgen`), and four packages waited for them: `strings` and `bytes` (five functions each that return
`iter.Seq`), `io/fs` (a local merge sort in `ReadDir`), `sort` and `regexp` (not ported in this step).

Two places are not plain Go: `maps.Clone` calls the runtime's map copy through `//go:linkname`, and `iter.Pull` and `Pull2` are built on
`runtime.newcoro` and `runtime.coroswitch`, the runtime's coroutines (a goroutine that is switched to and from directly, 20 ns a
switch, without the scheduler).

## Decision

1. **The four packages are `tools/portstd` output** (`ports.txt`, `PORT.md`, `PORTED.txt`; `tools/stubgen` has `Real: true` for them, and
   `TestRealPackagesMatchStub` compares their exported declarations with Go's). `slices` includes Go's pattern-defeating quicksort
   (`zsortordered.go`, `zsortanyfunc.go`) unchanged, so even the order of equal elements that the unstable sort leaves is Go's: its
   pivot choice is a deterministic xorshift of the length. Verified against real Go on 6000 random inputs per run
   (`tests/run/slices_diff_ops.gp`, hash of every result including the order of equal structs).
2. **`maps.Clone` is a native over the table's block** (`natives.txt`: `maps.clone` is `gp::maps::clone`; the package is Go's, unchanged). The
   native gets the `any` that Go's `Clone` passes and gives back an `any` of the same dynamic type: the map type's descriptor has a new
   operation, `type_ops::gp_map_clone` (`gocpp/typeops.hpp`, set for a `gp::map<K, V>` and for a named map type over one when K and V
   can be copied), which makes a new object of the map's own C++ type from `gp::map_clone` (`gocpp/map.hpp`). `map_clone` copies the block of
   the Swiss table slot by slot (control bytes included; keys and values by their copy constructors, so counted strings are shared as an
   assignment shares them; a block of plain bytes is one `memcpy`), keeps the seed, the growth budget and the NaN counter, so nothing is
   hashed or probed again, and starts with no range holding its block. NaN keys, tombstones and an empty map are copied as they are, nil stays
   nil (Go's `Clone` returns before the native), and a write in progress dies with "concurrent map clone and map write". A map with a
   move-only C++ key or value can't be cloned (a panic).
3. **`iter.Pull` keeps Go's code; the coroutine is a native** (`natives.txt`: `runtime.newcoro` is `gp::iter::newcoro`, `runtime.coroswitch` is
   `gp::iter::coroswitch`; `patches/iter.rules` makes the handle a `uintptr` instead of a `*coro`, so the natives need no generated type).
   A goroutine is a thread (ADR-007), so a coroutine is **a goroutine of its own that never runs at the same time as the goroutine
   that switched to it** (`runtime/src/iter.cpp`, `gocpp/iter.hpp`):
   - `newcoro` starts the goroutine with `go_start` (it counts in `runtime.NumGoroutine` from then, as Go's does) and waits until it has
     registered itself and parked. A thread of the idle pool is reused, so a Pull costs a few microseconds, not a thread start.
   - `coroswitch(c)` hands control to the other side and waits. Called by the coroutine it goes to whoever switched in last, so an
     iterator may be driven from different goroutines one after the other (Go allows it).
   - The hand-off word of a side is one atomic: the waiter spins on it for a moment (400 `pause`s; not on a machine with one CPU), then
     parks on the goroutine's own `waiter`. The other side sets it with one compare-and-swap when the waiter still spins (no lock, no
     system call), and under the waiter's lock when it is parked (so the waiter cannot go on, and end, before the notifier is done with it).
   - When the function returns, or ends with `runtime.Goexit` (the `goexit_signal` is caught), the goroutine ends and **then** the driver is
     woken: `goroutine::end_hook` (new, run by `run_goroutine` after the goroutine stopped counting), so `NumGoroutine` is already lower when
     `next` or `stop` returns (Go's `coroexit` destroys the goroutine before it switches). Panics and `Goexit` of the sequence are
     Pull's own business (its deferred function records them and `next` or `stop` runs them again on the caller's side), exactly Go's code.
   - Deadlock accounting (the goroutine layer's `block_scope` contract): both sides count as blocked while they wait ("coroutine", the
     reason Go's trace prints), the coroutine from its birth; a switch unblocks the other side **before** it blocks itself, and a coroutine that
     ends unblocks the driver before it stops counting (the end of a goroutine asks the detector whether everything sleeps). On a tree with the
     8.11 detector (merged into a scratch copy) a Pull whose sequence waits for a channel nobody sends to reports Go's deadlock with `goroutine 1
     [coroutine]` and `goroutine 2 [chan receive]`; without the unblock before the end, a plain `Pull` program ends with a false deadlock report
     (checked by removing the line).
   - **`-race`**: the hand-off word and the waiter's mutex are the edges the detector sees (Go has them from `race.Acquire` and `race.Release`
     in `iter.Pull`, which are no-ops in the stub of `internal/race`), as long as the end hook of a goroutine runs **outside** the scope in which
     `run_goroutine` hides the goroutine counts from the detector (`race::no_sync`); inside it, the end of a coroutine looked like a race on
     everything the sequence wrote. `iter_gotest_pull`, `iter_pull_goexit` and `iter_pull_many` (10000 pulls and 64 goroutines that pull) run clean
     with `-race`, with both compilers; moving the hook back inside the scope fails them.
   - A Pull that is neither exhausted nor stopped leaves its goroutine parked and counted for ever, as in Go (whose documentation says `stop`
     must be called); the parked thread holds the state, so LeakSanitizer sees it as reachable.
4. **The `@skip iter.go` rules of `strings` and `bytes` are gone**: `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq` and `FieldsFuncSeq` are in, with Go's
   iterator tests and examples (`tests/run/{strings,bytes}_gotest_{strings,bytes}.gp`, `*_goexamples.gp` regenerated: 45 tests and 68 examples for
   `bytes`, 40 and 57 for `strings`). `io/fs`'s patch is gone: `ReadDir` calls `slices.SortFunc` as Go does and the local sort is deleted.
5. **Tests.** Go's own `_test.go` files converted with the 9.4 harness (`// phase: 09`, expected output from real Go 1.25.14, checked again by
   `TestRunVsGo`): `cmp` (4 tests, 4 examples), `iter` (12: Pull and Pull2, values, early stop, double next and yield, panics, `Goexit`, the
   goroutine count after each step), `maps` (14 tests, 10 examples), `slices` (38 + 26 tests of `slices_test.go`, `sort_test.go`, `iter_test.go`,
   41 examples). Not run, each with its reason in the first comment of the file: `TestConcat_too_large` and nine cases of `TestRepeat` (a
   `[]struct{}` of `math.MaxInt` elements: gocpp's `make` panics "len out of range", Go allocates nothing), the allocation counts
   (`testing.AllocsPerRun` runs its function and returns 0 here), the benchmarks. Local stand-ins: `sort.Float64s` (the `sort` package is the next
   slice of 9.4) is an insertion sort in the same order, `math/rand` is the harness's splitmix generator. Hand-written: `slices_diff_ops`, `maps_diff_ops`,
   `cmp_diff_ops` (hash sweeps against real Go, with NaN, -0, infinities, equal keys, named types), `iter_pull_more` (2000 pulls in a row,
   deferred calls at `stop`, five panic kinds, a driver that changes between goroutines, pulls inside pulls, a zip, a channel sequence, eight
   goroutines at once, an abandoned pull), `iter_pull_goexit`, and the C++ unit test `runtime/test/iter_test.cpp` (also under ThreadSanitizer,
   `runtime/iter_test.go`). All pass with g++ and clang++, at -O0, with `-asan`, with `-release`, and on the real Windows PC (the programs that
   import no `os`).

## Alternatives

- **Real coroutines** (a stack switch with `ucontext` or hand-written assembly per architecture): a switch costs 20 to 50 ns. Rejected here:
  the thread layer is portable (Linux, Windows) and every other piece of the runtime (stack-overflow detection, panics, `defer`, the goroutine
  records, the deadlock detector, TLS) assumes one stack per thread; a coroutine on another stack touches all of them. It is the way to make
  `Pull` fast if it ever matters (see the numbers).
- **Pull in pure Go on channels** (a goroutine and two unbuffered channels): no native code, but each switch is a channel operation under a lock
  (more than the CAS hand-off) and the deadlock report would say `chan receive`, not `coroutine`.

## Consequences

Measured on the shared machine (load 100 to 150, one pinned core for the Go-style numbers, best of three): ratios gocpp/Go with g++ at -O2, clang++
at -O2 and with `-release` (g++); below 1 is faster than Go.

| | Go | g++ -O2 | clang++ -O2 | -release |
|---|--:|--:|--:|--:|
| `slices.Sort` 1M random ints | 77 ms | 1.6x | 1.3x | 1.2x |
| `Sort` 1M sorted / reversed ints | 1.4 / 1.2 ms | 0.9 / 1.1x | 0.6 / 0.7x | 0.5 / 1.0x |
| `Sort` 1M random float64 | 139 ms | 1.0x | 0.8x | 0.8x |
| `Sort` 200k random strings | 48 ms | 2.9x | 3.3x | 2.3x |
| `SortFunc` 300k structs (48 bytes) | 72 ms | 2.1x | 2.0x | 1.5x |
| `SortStableFunc` 300k structs | 230 ms | 3.3x | 2.3x | 2.3x |
| `SortFunc(cmp.Compare)` 1M ints | 135 ms | 1.5x | 1.0x | 1.0x |
| `BinarySearch` ints, 2M lookups in 1M | 374 ms | 1.2x | 0.8x | 0.8x |
| `BinarySearch` strings, 1M lookups in 200k | 307 ms | 5.4x | 4.4x | 2.9x |
| `Index` miss, 1M ints x 50 | 20.9 ms | 0.9x | 1.2x | 0.7x |
| `Contains` strings, 200k x 50 | 3.2 ms | 1.7x | 1.4x | 1.4x |
| `Max` + `Min`, 1M ints x 50 | 72 ms | 1.1x | 1.3x | 0.9x |
| `Insert` at front x 20000 / `Delete` middle | 13.7 / 7.2 ms | 1.3 / 1.2x | 0.9 / 1.1x | 1.0 / 0.9x |
| `Compact`, `Clone` | 1.4 / 5.9 ms | 0.6 / 0.7x | 0.5 / 0.6x | 0.4 / 0.5x |
| `Collect(Values)` 1M ints | 3.8 ms | 3.5x | 3.9x | 3.8x |
| `for range slices.All` 1M x 10 | 2.9 ms | 5.8x | 5.4x | 5.8x |
| `slices.Sorted(maps.Keys)` 200k | 16 ms | 1.3x | 1.3x | 1.0x |
| `maps.Clone` 200k `map[int]int` / 100k `map[string]string` | 1.5 / 1.1 ms | 0.05 / 2.3x | 0.1 / 3.2x | 0.05 / 2.6x |
| `maps.Collect(All)` 200k | 13 ms | 0.4x | 0.4x | 0.3x |
| `iter.Pull`, one `next` | 60 ns | 0.15 to 0.7 us (two CPUs: 0.3 us), 3 us on one CPU | | |
| `iter.Pull` create, one `next`, `stop` | 0.3 us | 15 to 19 us | | |

- **Sorting numbers is as fast as Go's** (0.5x to 1.6x), the same pattern-defeating quicksort. The slow rows are costs of the compiler and
  the runtime, not of the ports (not profiled here; the same causes as in ADR-073): the comparison of two `gp::string` (3x to 5x for the
  string rows), the copy of a struct with counted fields, the state machine and boxed closure of a range over `slices.All` (5x), and
  `Collect` appending through a closure. They are what the compiler work of Phase 15 would speed up (an inline string compare, an iterator call
  that inlines its yield function, fewer counted copies).
- **`iter.Pull` is a thread hand-off**, 5x to 50x slower per value than Go's coroutine, and a Pull costs 15 to 19 us to create and stop
  (Go: 0.3 us). Use `range` over the sequence for loops; Pull is for the occasional zip of two sequences. A real coroutine (Alternatives)
  would close most of it.
- **Compile time** (the generated C++ of an instance is compiled in the program that asks for it, ADR-030): `tests/run/slices_diff_ops.gp`, which
  instantiates `Sort`, `SortFunc`, `SortStableFunc`, `BinarySearch*` and many other functions for ints, strings, float64 and a struct,
  compiles in 1.9 to 2.1 s with clang++ and 2.8 s with g++ at -O0 (a program that only prints: 0.45 s), and 5.5 s with `g++ -release`.
- `maps.Clone` of integer keys and values is a `memcpy` of the block (20x faster than Go's); with counted strings it pays two atomic increments
  per entry (2x to 3x slower than Go).
- **`sort`, `regexp/syntax` and `regexp`** (the rest of 9.4) are not ported here. Nothing blocks `sort` any more (`slices` exists,
  `internal/reflectlite.Swapper` is built in 9.3); `regexp/syntax` needs `sort` and `slices`, `regexp` needs `regexp/syntax`.
- The merge with the 8.11 branch has one conflict in `run_goroutine` (`goroutine.cpp`): both the detector's `deadlock_check()` and the
  `end_hook` stay, in that order (the check first: the coroutine's driver is unblocked before the goroutine ends, so it does not count as asleep).
- Instances of the generic functions of another package are named `main.Pull[...]` in a trace (the asking package's name, pre-existing
  behavior of ADR-030), where Go says `iter.Pull[...]`.

## Evidence

- Differential sweeps against real Go 1.25.14 (hashes of 6000 `slices` inputs, 3000 `maps` inputs, every pair of edge values for 20 types
  of `cmp`), Go's own tests of the four packages, and the mutation checks of this step: a changed `maxInsertion` of the sort, a `Clone` that
  copies nothing (a map clone that skips the entries), a `stop` that does not switch, an off-by-one in `splitSeq`, a reversed `ReadDir` sort, a coroutine that wakes its driver
  before it stops counting each fail a test; removing the unblock before the end of a coroutine gives a false deadlock report on a tree with the
  8.11 detector.

See: [Phase 07 §2.6](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#26-gos-generic-packages-ported), [ADR-007](007-goroutines-os-threads.md),
[ADR-030](030-generic-instances.md), [ADR-033](033-range-over-func.md), [ADR-034](034-portstd-generic-arch-and-rewritten-constraints.md),
[ADR-073](073-strings-bytes-io-bufio-context-ports.md).
