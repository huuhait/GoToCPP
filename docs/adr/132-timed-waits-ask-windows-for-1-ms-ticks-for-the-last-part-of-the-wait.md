# ADR-132: Timed waits ask Windows for 1 ms timer ticks, for the last part of the wait only

Status: accepted (2026-10-21; job 14.sleep)

## Context

On Windows a wait with a timeout (`std::condition_variable::wait_for`, `std::this_thread::sleep_for`: the C++
library's thread layer, mcfgthread or winpthreads, sits on the kernel's timed waits) ends at a tick of the system
timer. The tick is 15.6 ms unless some process has asked for finer ones with `timeBeginPeriod`. Every timed wait of
the gocpp runtime was therefore rounded up to the next tick: `time.Sleep`, the timer thread (`time.After`,
`NewTimer`, `Ticker`, `AfterFunc`), a `select` with a timeout, and a context deadline (these are all timers or
`gp::waiter::gp_park_ns`/`gp_wait_` with a deadline). Real Go does not have this problem (before 1.16 it requested
1 ms ticks itself, and it uses high-resolution waitable timers since).

It showed in `demos/imgui-dx11`: the frame cap `time.Sleep(16ms - elapsed)` (about 15.4 ms asked) took 30 ms each
time, so the demo ran at 32 frames a second instead of 60 (`Present` returns in 0.3 ms on that display). The demo
with `//gocpp:extern C func timeBeginPeriod` and `timeBeginPeriod(1)` ran at 59.

Measured on the real Windows PC (Windows 10/11 desktop, `runtime/test/sleep_test.cpp`, 200 waits of each length,
microseconds later than asked; median / 99th percentile):

| wait | before | after |
|---|---|---|
| `Sleep(1ms)` | 14545 / 15642 | 1016 / 2157 |
| `Sleep(5ms)` | 10573 / 11908 | 1086 / 2424 |
| `Sleep(15ms)` | 15827 / 17610 | 1204 / 2688 |
| `Sleep(100ms)` (20 waits) | 8948 / 10114 | 1299 / 2014 |
| `Sleep(1s)` (3 waits) | 10180 / 11662 | 1046 / 1776 |
| timer of 1 ms / 5 ms / 15 ms | 14527, 10517, 15842 (medians) | 1042, 1117, 1129 |
| ticker of 1 ms / 5 ms / 15 ms | 14978, 12961, 8745 (medians) | 925, 829, 1346 |
| `demos/imgui-dx11` (instrumented copy, no change to the demo) | 33 fps, sleeps of 30.2 ms | 58-59 fps, sleeps of 16.5 ms |

## Decision

A timed wait asks the system for 1 ms ticks (`timeBeginPeriod(1)`) **only for the last part of the wait**, and the
request is **counted** across threads.

- `gp::os::fine_timer(bool)` and `gp::os::timer_margin_ns()` (`gocpp/os.hpp`) are the OS layer's part. Linux does
  nothing and has margin 0. Windows (`src/os/windows/os.cpp`) calls `timeBeginPeriod(1)` and `timeEndPeriod(1)` and
  has a margin of 32 ms, two ticks of 15.6 ms.
- `src/timed_wait.hpp` and `.cpp` are standard C++. `plan_timed_wait(left_ns, margin)` splits what is left of a wait:
  with a margin of 0 it is the whole wait (the code runs as before); a wait of at most two margins is one fine step
  to the end; a longer one first waits for all but one margin with ordinary ticks (which add at most one margin, so
  it ends inside the last margin) and the caller's loop, which it already had for early wake-ups, plans the rest.
  `fine_timer_scope` holds a count of fine waits under a mutex and calls `fine_timer(true)` when the count leaves 0
  and `fine_timer(false)` when it returns to 0, so any number of goroutines share one request.
- The callers are `waiter::gp_park_ns` (`Sleep`) and `waiter::gp_wait_` with a deadline (`waiter.cpp`), the timer
  thread's wait for the earliest deadline (`timer.cpp`), and `testing`'s `sleep_ns`. A select with a timeout and a
  context deadline are timers. Waits that are not about time (the deadlock report's
  hand-over, the alarm of `-test.timeout`, the microsecond polls at the end of the process) are left as they are.
- `timeBeginPeriod` and `timeEndPeriod` are found with `LoadLibraryExW(L"winmm.dll", ..., LOAD_LIBRARY_SEARCH_SYSTEM32)`
  and `GetProcAddress` on the first fine wait. No link flag: a program built with the driver, a project that uses the
  `cpp-lib` mode with CMake, and the runtime's own tests (which are linked by hand) all have it. If the DLL cannot be
  loaded the request is a no-op and the waits are as they were.

Results: waits end 1 ms later than asked at the median (the resolution of the tick that was asked for) and about
2 ms at the 99th percentile, for 1 ms to 1 s, on the same PC.

## Why this and not the others

- **`timeBeginPeriod(1)` for the whole process** (simplest). The request raises the machine's timer interrupt rate
  while the process lives; a program that sleeps for a minute, or only waits for I/O, would keep the whole PC at
  1000 interrupts a second for nothing (power, and the laptop battery). Go itself relaxed the request when idle.
- **A count around the whole timed wait** (no margin). The same cost for a long `Sleep` or a ticker of a second:
  the fine tick would be held for the whole wait. With the margin, a wait of an hour holds it for 32 to 64 ms, and a
  ticker of 1 s for at most 64 ms in every second. The price is that waits wake once more (about 32 ms before their
  end), which is one wake-up per long wait.
- **High-resolution waitable timers** (`CREATE_WAITABLE_TIMER_HIGH_RESOLUTION`, Windows 10 1803 and later; what Go
  does). They give a better resolution (about 0.5 ms) and no global tick. But `std::condition_variable` and
  `sleep_for` cannot wait on them: every blocking primitive of the runtime (`gp::waiter`, the timer thread, the
  channel and select waits) would have to be rebuilt on `WaitForMultipleObjects` or on keyed events. That is a
  rewrite of the threading core for 1 ms of accuracy. It stays possible later behind the same two functions of the
  OS layer.
- **`-lwinmm` in the link line** (`internal/build/crosstarget.go` `LinkFlags`). It would have to be repeated for the
  `cpp-lib` mode's pkg-config and CMake files and for every hand-made link of the runtime; the loaded function
  pointers need none of that.
- **`NtSetTimerResolution`** (ntdll). It can ask for 0.5 ms, but it is not documented; `timeBeginPeriod` is.
- **A spin for the last millisecond.** It would burn a core for each short sleep.
- **Fixing it in the demo** (`timeBeginPeriod` in `main.gp`). The runtime is wrong for every program, not the demo.

## Limits

- The accuracy is the tick that was asked for, 1 ms, not the sub-millisecond of Go with high-resolution timers: the
  median is about 1 ms late, the 99th percentile 2 to 3 ms, and a loaded PC more.
- Windows 10 version 2004 and later make the request per process; before that it is machine-wide (other programs get
  finer ticks too while a wait of ours is in its last part). Windows 11 may ignore the request of a process whose
  window is minimized or covered (a program that shows no window is not affected); a wait is then rounded up as it
  was.
- An idle worker thread that leaves the pool after its 5 s wait goes through the same deadline wait, so on Windows it
  holds the fine tick for the last up to 32 ms of that wait. That is harmless (one hold, then gone).
- A ticker of 1 ms loses about 25 to 45% of its ticks: a wake-up is about 1 ms late, which is a whole period, and the
  ticks that are missed are dropped as Go does for a slow receiver. The ticker row of the table above measures the
  ticks that were delivered, not the rate. Tickers of 5 ms and longer keep their rate (500 ticks of 10 ms drift
  0.2 to 1.3 ms in total).
- The measured holds of long waits are 18 to 30 ms, because the coarse wake-up lands inside the margin; "32 to 64 ms" is
  the upper bound.
- The first fine wait of a process loads `winmm.dll` (a few milliseconds, once).
- A fine wait costs two calls (`timeBeginPeriod`, `timeEndPeriod`) when no other fine wait is pending: a loop of
  `Sleep(1ms)` does this a thousand times a second. The 200 waits of 1 ms above include that cost. A program that
  ticks every few ms holds the 1 ms tick almost all the time.
- Wine and Windows 7/8 were not measured. The Windows layer targets Windows 10 and later; on an older system where
  `LOAD_LIBRARY_SEARCH_SYSTEM32` is unknown the request is a no-op and the waits are as before.
- The waits that are not in the list above (the deadlock report's hand-over, `alarm_wait`, the
  200 microsecond polls of `end_threads` and the death wait) are still rounded to the tick on Windows; none of them
  is about the accuracy of a program's timers. A program's own direct call of `Sleep` from `<windows.h>` is its own.

## Tests

- `runtime/windows_sleep_test.go` (`TestWindowsSleepAccuracy`) builds `runtime/test/sleep_test.cpp` for Windows and runs
  it on the real PC (`GOCPP_WINDOWS_SSH`; skipped without it, Wine's timers are not Windows'). It measures 200
  sleeps, one-shot timers and ticker ticks of 1 ms, 5 ms and 15 ms, 20 sleeps of 100 ms and 3 of 1 s, and asserts
  that the median is at most 3 ms and the 99th percentile at most 10 ms later than asked (generous: it passes with
  1 and 2.7 ms, and fails with 14.5 and 15.6 ms on the old runtime, which was shown by running it before the
  change), and that a goroutine in a 30 ms sleep holds the fine tick while one in a 400 ms sleep does not hold it
  in its first 200 ms.
- `runtime/test/time_test.cpp` (`test_plan_timed_wait`, `test_fine_timer_scope`) tests the portable part on Linux
  with g++ and clang++: the plan for a margin of 0 and of 32 ms, a simulated wait with a rounding of up to a margin
  per coarse step (ends on time, at most two margins fine, at most three steps), and the count of the scope in
  nested scopes, in eight threads, and after a `Sleep`.
