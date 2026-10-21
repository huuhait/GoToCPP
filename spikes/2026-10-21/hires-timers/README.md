# Spike: how late does a wait end on Windows? (job 14.hires, 2026-10-21)

Question: after ADR-132 (a timed wait holds `timeBeginPeriod(1)` for its last 32 to 64 ms; waits end about 1 ms
late) is there a mechanism that ends Sleep, timers, tickers, select timeouts and deadlines within about
0.2 to 0.3 ms at the median and 1 ms at the 99th percentile, without holding the 1 ms system tick? The candidate
is the high-resolution waitable timer (`CreateWaitableTimerExW(CREATE_WAITABLE_TIMER_HIGH_RESOLUTION)`, Windows 10
1803), which is what Go uses. A `std::condition_variable` cannot wait on it, so the design needed measuring.

Machine: the user's Windows 11 PC (build 26200, `Administrator@` over ssh), a desktop with other programs running.
`NtQueryTimerResolution` says the finest tick is 0.5 ms and the current one 1.0 ms *before the test starts*
(another process holds it), but Windows 10 2004 and later give a process its own tick: a plain
`condition_variable::wait_for(1 ms)` still takes 14.6 ms in a process that asked for nothing.

`hires.cpp` (throwaway, MinGW-w64, no runtime) measures one wait kind at a time; `gosleep/main.go` is the same loop
for Go's `time.Sleep`. Run folders on the PC were unique and removed.

## Results

Microseconds *later* than asked, median / 99th percentile, 200 back-to-back waits per cell
(`results-back-to-back.txt`, `results-variants.txt`). Back-to-back waits keep the same phase against the system's
interrupts; `results-random-phase.txt` and `results-preroll.txt` put a random jitter in between.

| wait | 1 ms | 5 ms | 15 ms | 100 ms |
|---|---|---|---|---|
| `condition_variable::wait_for`, nothing asked | 14577 / 15629 | 10546 / 11890 | 15890 / 17903 | 8973 / 10162 |
| same, `timeBeginPeriod(1)` held (ADR-132, steady state) | 1004 / 3003 | 1230 / 3204 | 1238 / 2972 | 1373 / 2407 |
| same, `NtSetTimerResolution(0.5 ms)` held | 1008 / 3001 | 1044 / 2822 | 1117 / 2862 | 1018 / 2792 |
| hr timer + `WaitForSingleObject` | 523 / 1563 | 516 / 1781 | 457 / 1986 | 467 / 1927 |
| hr timer + `WaitForMultipleObjects([event, timer])` (what a waiter needs, to be woken early) | 598 / 1750 | 527 / 1733 | 508 / 2123 | 512 / 2435 |
| condvar for all but 32 ms, then hr timer + event (hybrid) | 536 / 1765 | 504 / 1757 | 486 / 2104 | 379 / 1196 |
| hr timer in a timer thread that signals the waiter's condvar (one more thread hop) | 580 / 1870 | 575 / 1865 | 473 / 2340 | 624 / 1901 |
| condvar for the coarse part, then spin 1 ms | 0 / 812 | 0 / 1519 | 0 / 905 | 0 / 0 |
| hr timer 0.5 ms early, then spin | 0 / 1178 | 77 / 1651 | 396 / 1557 | |
| Go 1.25 `time.Sleep` (`results-go.txt`) | 554 / 1056 | 518 / 983 | 408 / 2049 | 494 / 2645 |

Other numbers:

- Waking a blocked waiter: `SetEvent` on an event it waits on, 8 us median / 119 us p99 (`wake-event`); setting its
  hr timer to a time in the past, 772 / 2891 us (`wake-timerpast`): use an event.
- The hop through a timer thread costs about 60 us at the median over waiting directly (`hop` against `hr`).
- `NtSetTimerResolution(0.5 ms)` held does not help a condition variable (1.0 to 1.1 ms median): the wait is
  rounded to a whole tick of the *process*, and a plain hr timer is no better with it held (`hr+nt0.5`).
- A spin over the last 1 ms of a wait is on time (median 0) but costs about 1 ms of CPU for a wait of 1 ms
  (`cpu/wait` in `results-back-to-back.txt`): a core for a 1 ms ticker. A wait on the timer plus a spin of P costs
  P minus what the timer is early.

The raw lateness of the hr timer is a body of 0.4 to 0.5 ms (median) with a tail: p90 1.0 to 1.2 ms, p99 1.9 to
2.1 ms (`results-preroll.txt`, random phase). It does not depend on P. Go's `time.Sleep` has the same body
(0.5 ms) and tail. Pre-rolling the timer by P and spinning the rest moves the body to the left and leaves the tail:

| P (us) | final median | final p90 | final p99 | average spin |
|---|---|---|---|---|
| 0 | 401 to 424 | 958 to 1072 | 1939 to 1998 | 0 |
| 150 | 273 to 311 | 1005 to 1051 | 1788 to 1958 | 4 to 5 us |
| 300 | 129 to 173 | 846 to 917 | 1758 to 1821 | 32 to 38 us |
| 600 | 0 | 485 to 585 | 1356 to 1454 | 185 to 226 us |
| 1000 | 0 | 119 to 260 | 891 to 1044 | 511 to 535 us |

## Reading

- A high-resolution timer halves ADR-132's median (1.0 to 1.3 ms to 0.4 to 0.5 ms) and trims its tail
  (p99 2.8 to 3.2 ms to 1.0 to 2.0 ms), exactly as Go gets, and needs no request for fine ticks: the system keeps
  its idle tick while a process sleeps.
- The median of 0.2 to 0.3 ms needs a pre-roll: P = 300 us gives a median of 0.13 to 0.17 ms for an average of
  35 us of spin (0.2% of a core for a 60 frame/s loop, 3.5% for a 1 ms ticker). The p99 of about 1 ms needs
  P = 1 ms (half a millisecond of CPU per wait: a core for a 1 ms ticker): not worth it; the tail is the machine's.
- Condition variable plus hr timer in one waiter (an event to wake it) is as accurate as a timer alone; the
  timer-thread design is 60 us worse at the median and needs a registry of deadlines and a hand-over to the waiter.
