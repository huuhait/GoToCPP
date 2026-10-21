# ADR-133: Timed waits on Windows use high-resolution timers, with a short spin at the end

Status: accepted (2026-10-21; job 14.hires). Replaces the mechanism of [ADR-132](132-timed-waits-ask-windows-for-1-ms-ticks-for-the-last-part-of-the-wait.md)
where the system has high-resolution timers; ADR-132 stays as the fallback.

## Context

ADR-132 made Windows timed waits end about 1 ms late (median) by holding `timeBeginPeriod(1)` for the last 32 to 64
ms of every wait. Two things were left: 1 ms is a whole tick of lateness (a 1 ms ticker lost a third of its ticks,
805 of 1000 in a fresh measurement), and the wait has to hold the machine's tick, which costs power. The user
asked for waits within about 0.2 to 0.3 ms of their time at the median, a 99th percentile of about 1 ms, and no hold
on the 1 ms system tick when the program is idle (what modern Go does on Windows: high-resolution waitable timers,
`CREATE_WAITABLE_TIMER_HIGH_RESOLUTION`, Windows 10 version 1803).

The runtime blocks on `std::condition_variable` (mcfgthread). A waitable timer cannot wake a condition variable, so
the design needed a measurement first. [spikes/2026-10-21/hires-timers](../../spikes/2026-10-21/hires-timers/README.md)
has the program, the raw output and the tables. On the real Windows PC (Windows 11 build 26200, 200 waits per cell,
microseconds later than asked, median / 99th percentile; Go 1.25 `time.Sleep` on the same PC for reference):

| wait | 1 ms | 5 ms | 15 ms |
|---|---|---|---|
| condition variable, nothing asked | 14577 / 15629 | 10546 / 11890 | 15890 / 17903 |
| condition variable, `timeBeginPeriod(1)` held (ADR-132) | 1004 / 3003 | 1230 / 3204 | 1238 / 2972 |
| condition variable, `NtSetTimerResolution(0.5 ms)` held | 1008 / 3001 | 1044 / 2822 | 1117 / 2862 |
| high-resolution timer alone | 523 / 1563 | 516 / 1781 | 457 / 1986 |
| timer + event, one `WaitForMultipleObjects` (wakeable) | 598 / 1750 | 527 / 1733 | 508 / 2123 |
| timer thread that signals the waiter's condition variable | 580 / 1870 | 575 / 1865 | 473 / 2340 |
| timer 0.3 ms early + spin (random phase, 1500 waits) | | | 129 to 173 / 1758 to 1821 |
| Go `time.Sleep` | 554 / 1056 | 518 / 983 | 408 / 2049 |

What the numbers say:

- The high-resolution timer is late by a body of 0.4 to 0.5 ms (its own interrupt) and a tail (p90 1.0 to 1.2 ms,
  p99 about 2 ms in random phase). Go has the same body and tail. `NtSetTimerResolution(0.5 ms)` does not help a
  condition variable: the wait is rounded to a tick *of the process*.
- A wake-up of a blocked waiter by `SetEvent` takes 8 us at the median (p99 119 us); by setting its timer to a time
  in the past 772 us. A hop through a timer thread costs about 60 us at the median.
- Setting the timer P microseconds early and spinning to the deadline moves the body to the left and leaves the
  tail: P = 0: median 0.40 to 0.42 ms; 150 us: 0.27 to 0.31; 300 us: 0.13 to 0.17 (average spin 32 to 38 us);
  600 us: 0 (spin 185 to 226 us); 1 ms: 0 (p99 0.9 to 1.0 ms, spin 511 to 535 us).

## Decision

Where `CreateWaitableTimerExW` accepts `CREATE_WAITABLE_TIMER_HIGH_RESOLUTION` (tried once at run time; no import
beyond kernel32, no link flag), every timed wait of the runtime waits on a **wait slot**: a high-resolution timer
and a manual-reset event, owned by the OS layer, and a short spin at the end. Elsewhere the runtime behaves as in
ADR-132.

- **OS layer** (`gocpp/os.hpp`, `src/os/windows/os.cpp`): `precise_timers()`,
  `wait_slot_new/open/close/arm/wait/wake`. `wait_slot_wait(slot, timer_ns, spin_ns)` sets the timer to fire
  `timer_ns` from now and blocks in `WaitForMultipleObjects([event, timer])`; if the event ended it, it cancels the
  timer and returns; if the timer did, it spins (`YieldProcessor`, polling the slot's wake flag) for what is left of
  `spin_ns` after `timer_ns`. If the timer cannot be set, the event is waited on with a timeout of whole
  milliseconds (the old accuracy), and a failed wait sleeps 1 ms so that a caller's loop cannot spin. The two steps
  of `wait_slot_wake` are the event first and the flag second; `wait_slot_arm` clears both every time, so a wake-up
  that comes late (after the slot was used again) ends one wait early at most and cannot leave the event set for
  good (a first version cleared the event only when the flag was set, and a waker that was slow between its two
  steps could leave a slot that returned at once for ever; `winos_test.cpp` holds a waker between the steps with
  `os::testing::set_wake_pause_hook`). An idle slot can give its handles back (`close`) and get them again
  (`open`); a lock keeps a late wake-up off a closed handle. Linux: `precise_timers()` is false and the other
  functions do nothing; they are never called.
- **`src/timed_wait.hpp`** (standard C++): `split_precise_wait` (timer part and spin part: the spin is the
  pre-roll of 300 us **but at most half of the wait**, so a wait of 100 us is a timer of 50 us and a spin of at
  most 50 us, never a spin of the whole wait), `slot_pool` (slots are made on demand and reused; at most 64 idle
  slots keep their handles, the others close them and open them again when used; the slot structures are never
  freed), and `basic_precise_cv<Backend>`, a condition variable whose
  `wait_for` takes a slot and publishes it in an atomic *while the caller's mutex is held*, unlocks, waits on the
  slot, relocks and gives the slot back; `notify_one` and `notify_all` call the condition variable and also wake the
  published slot. `precise_cv` is it over the OS layer's slots. `wait_for` falls back to ADR-132's plan
  (`plan_timed_wait`, `fine_timer_scope`) when there are no precise timers, when no slot can be made, or when a
  second thread waits with a timeout on the same variable (the runtime has one waiter per variable). `precise_sleep`
  is the same for a sleep that nobody wakes.
- **Callers:** `gp::waiter`'s state and the timer heap use `precise_cv` in place of `std::condition_variable`
  (`waiter.cpp`, `timer.cpp`; the calls of `wait_for` are the same), and `testing`'s `sleep_ns` uses `precise_sleep`.
  So `Sleep`, timers, tickers, `AfterFunc`, select timeouts and context deadlines are all covered, as in ADR-132. The
  deadlock detector is untouched: a sleeping goroutine is not counted as blocked because `time.Sleep` never told it
  it was.
- **Test hooks** (`os::testing`): `cpu_time_ns` (the process's cycle count, calibrated against the performance
  counter: `GetProcessTimes` is in 15.6 ms ticks), `open_handles`, `set_wake_pause_hook` and `force_fallback(bool)`
  (makes `precise_timers()` false, so that a test program runs the ADR-132 path on a PC that has the timers; there
  is no environment variable or other switch for users).

The pre-roll is 300 us. It is the knee of the table above: median 0.13 to 0.17 ms (the target was 0.2 to 0.3) for
an average of 35 us of CPU per wait. The first version spun the whole of a wait of at most 300 us, which is
right for the accuracy and wrong for a program that polls: a loop of `time.Sleep(100 * time.Microsecond)` used 97%
of a core and 24 goroutines sleeping 50 to 400 us used 3.4 cores (ADR-132: 8% and 0.9). With the spin at most half
of the wait, such a wait is the timer's (about 0.4 to 0.7 ms late, as Go's `time.Sleep` is on this PC), the
spin is mostly nothing because the timer fires after the end, and the loops cost what ADR-132's did (table below).

## Why this and not the others

- **A timer thread that signals the waiters' condition variables** (the waiters keep `wait_for` without a
  deadline). It needs a registry of deadlines, the cancel race between a waiter that was woken early and the thread
  that is about to signal it, and the lifetime of the waiter's state; it adds a thread hop (about 60 us at the
  median in the spike) and a second place that has to know about every wait. The wake-up path of a slot is one
  atomic load and, when a thread is in a precise wait, a `SetEvent`.
- **Handle-based waiters** (replace the mutex/condition variable of `gp::waiter` by events on Windows). The wake
  sources are many (`gp_unpark`, `gp_unpark_with`, `gp_post`, the timer heap's `notify`) and every blocking
  primitive of the runtime sits on them; the slot keeps the C++ library's condition variable for everything that
  is not a timed wait, and a slot exists only while a thread waits with a timeout.
- **Wake by setting the timer into the past** instead of an event: 772 us against 8 us.
- **Condition variable for the long part, slot for the last 32 ms** (the hybrid of the spike). Needs the plan
  of ADR-132 and wakes the thread once more; the timer alone is as good for waits of any length (a timer an hour away
  costs nothing while it is pending) and was measured as accurate at 100 ms (467 / 1927 us alone; 379 / 1196 us for the hybrid).
- **A pre-roll of 1 ms** for a 99th percentile of 1 ms: half a millisecond of CPU per wait, a core for a 1 ms
  ticker. The tail of the timer (p99 about 2 ms) is the machine's and Go has it too. 300 us is the value where
  the spin is invisible (0.2% of a core at 60 frames a second).
- **A pure spin or `SwitchToThread` loop for the end of the wait:** the spin here is bounded by 300 us and only
  after the timer has fired.
- **Keeping ADR-132's mechanism alone:** median 1 ms, holds the machine's tick for 32 to 64 ms of every wait.

## Measured with the runtime

`runtime/test/sleep_test.cpp` on the real PC (three runs; before = ADR-132 on the same PC at the same time):

| | before (ADR-132) | now |
|---|---|---|
| `Sleep` 1 / 5 / 15 ms, median / p99 (us late) | 1013 / 2378, 1070 / 2156, 1410 / 3067 | 10 to 56 / 540 to 640, 30 to 110 / 295 to 555, 111 to 116 / 689 to 1116 |
| timer 1 / 5 / 15 ms | 1042 / 2405, 1057 / 2180, 1124 / 2180 | 39 to 90 / 633, 143 to 206 / 675 to 772, 69 to 78 / 1097 to 1141 |
| ticker 1 / 5 / 15 ms | 1620 / 2573, 1334 / 2859, 682 / 1629 | 80 to 122 / 499 to 724, 117 to 135 / 1509 to 1815, 224 to 280 / 1517 to 2161 |
| `Sleep(100 ms)` / `Sleep(1 s)` | 1373 / 2494, 931 / 1271 | 172 to 268 / 1112 to 1634, 20 to 134 / 88 to 165 |
| ticks received of 1000 from a 1 ms ticker | 805 | 876 to 940 |
| a plain 2 ms wait while another goroutine sleeps 1.5 s | | 15.4 to 15.7 ms (the process holds no fine tick; 15.5 to 16.0 ms before the sleep) |
| CPU: 16 ms sleep / 1 ms sleep | | 39 to 49 us / 20 to 26 us |
| CPU of a 1.5 s sleep | | 0.56 to 0.69 ms (the probe threads included) |
| handles after 10000 sleeps and 1000 timers (8 goroutines) | | 87 before, 87 after; 9 to 25 slots made |
| loop of `Sleep(100 us)` for 2 s: iterations, CPU | 1448 to 1498, 16 to 18 ms (1%) | 2338 to 2805, 27 to 29 ms (1.4%); a spin of the whole wait: 19187, 1934 ms (97%) |
| loop of `Sleep(1 ms)` for 2 s | 890 to 900, 10 to 11 ms | 1501 to 1632, 37 to 57 ms (2 to 3%) |
| 24 goroutines, sleeps of 50 to 400 us, 2 s | 35431 to 38348, 106 to 123 ms (6%) | 74517 to 82023, 296 to 317 ms (15%); a spin of the whole wait: 61768, 6781 ms (3.4 cores) |
| a ticker of 1 ms for 1 s: CPU of the process | | 70 to 99 ms (7 to 10%, the receiver included) |
| 1000 goroutines asleep at once (600 ms) | | 1001 slots, 2071 handles (119 before); after: 709 handles, 320 slots open (64 idle and 256 held by the idle threads of the goroutine pool), 6.5 s later 197 handles and 64 slots open |
| `demos/imgui-dx11` (instrumented copy, 12 s), frames a second; a 15.1 ms sleep takes | 58 to 59 fps (57.5 average); 16.2 ms | 62 to 63 fps (61.2 to 61.5 average); 15.2 ms |
| `demos/imgui-dx11`, CPU of the process in 8 s (15.6 ms accounting ticks) | 0.91 s (11%) | 0.86 to 0.95 s (11 to 12%) |

(The one tick that is dropped in ten of a 1 ms ticker is the timer's tail: a tick is dropped when the timer fires a
whole period late, which Go does as well.)

## Limits

- A sleep of a few nanoseconds to a few microseconds still takes about 0.4 to 0.5 ms (a loop of 100,000 `time.Sleep(0..4 ns)` took 41 s, about 410 us each), as Go's own `time.Sleep` does on this PC; ADR-132 took about 1 ms. Code that polls with a tiny sleep should not expect less.
- `GetProcessTimes` is too coarse to measure the CPU of this load (the same program read 0.45 s and 1.25 s on two runs); the tests use `os::testing::cpu_time_ns` (cycle count), and a manual check should use the same.
- The tail is not improved: the 99th percentile is 0.5 to 2.2 ms by wait length, as for Go (1 to 2.6 ms). The target
  of 1 ms was met at the median only. A heavily loaded PC makes it worse (timer interrupts are served by a core
  that may be busy).
- High-resolution timers exist from Windows 10 version 1803. Before that, `CreateWaitableTimerExW` fails on the
  flag, `precise_timers()` is false and the runtime keeps ADR-132's behaviour (tested on the real PC with a test-only
  switch inside the test program, see Tests). Windows 7/8 and Wine were not measured; Wine implements the flag, not necessarily its
  accuracy.
- A slot is two handles (a timer and an event) while a wait uses it, and a few bytes for ever: a slot structure is
  never freed (a thread that wakes a slot late may hold its pointer). The pool keeps 64 idle slots with their
  handles and closes the others, so 1000 sleeping goroutines at once peak at 2071 handles and leave 197 (64 slots
  open) a few seconds later; a program with 10000 concurrent sleepers peaks at about 20000 handles for them (a
  reviewer measured 29119 in all, threads included). Windows allows 16 million per process. The idle threads of the
  goroutine pool wait with a timeout of 5 s and hold a slot each meanwhile (up to 256).
- The spin is on a core for up to 300 us of every wait longer than 600 us (35 us on average) and for at most half
  of a shorter one. A program that sleeps 1 ms in a loop uses 2 to 3% of a core for it, a ticker of 1 ms 7 to 10%
  with its receiver; a loop of 100 us sleeps 1.4% (it runs at about 0.7 ms a turn: the timer's lateness, as in Go).
- A wake-up that arrives during the spin is seen within microseconds (the flag is polled). A `notify` on a
  condition variable that a thread is waiting on, from a thread that does not hold the variable's mutex, is safe for
  the same reason as for the C++ library's variable: the waiter published its slot before it let go of the mutex.
- The first precise wait of a process tries to make a timer (a system call); later ones read a static.
- A waiter that is woken early leaves its timer armed unless it is cancelled: the wait cancels it (one more system
  call per early wake-up); a reused slot re-arms the timer, which also clears a late signal.

## Tests

- `runtime/test/time_test.cpp` (Linux with g++ and clang++, also under ASan/UBSan and ThreadSanitizer through
  `TestTimeTSan`; it now runs on the real PC too, `TestWindowsRuntime`): `test_split_precise_wait` (a short wait is
  never only a spin), `test_slot_pool` (reuse, a system that makes none, eight threads),
  `test_slot_pool_gives_handles_back` (the cap of idle slots, opening again, a system that cannot open), and the protocol of `precise_cv` over a fake backend of mutexes and condition variables: it waits its time and
  uses a slot each time (not once), falls back when the backend is unavailable, a notify ends a 20 s wait
  (`notify_one`, `notify_all`, with and without the mutex), no wake-up is lost in 20000 hand-overs and in 1000
  notifies that take the mutex the moment the waiter lets go of it, a second timed waiter waits the old way, and
  `wait` is the plain one.
- `runtime/windows_sleep_test.go` `TestWindowsSleepAccuracy` (the real PC; the PC must have high-resolution
  timers). Subtest `precise`: median at most 0.8 ms and p99 at most 5 ms for every sleep, timer and ticker
  (measured 0.39 and 2.4 ms at worst; the old runtime fails both and ADR-132 fails the median), a median of at
  most 0.3 ms for the sleeps of up to 5 ms (the spin; a bare timer is 0.5 ms), no fine-tick holder, an idle process
  (a goroutine in a 1.5 s sleep does not change a plain 2 ms wait's 15 ms and uses at most 30 ms of CPU), at most
  400 us of CPU per sleep, the loops of short sleeps (CPU in 2 s at most 150 ms for 100 us sleeps and for 1 ms
  sleeps, 800 ms for the 24 goroutines), no handle growth over 10000 sleeps, the burst of 1000 sleepers (most slots
  closed afterwards, at most 64 open plus a few 6.5 s later), at least 750 of 1000 ticks of a 1 ms ticker, and a
  clean exit with a timer and a sleeping goroutine an hour away. Subtest `fallback`: the program started with the
  argument `fallback` calls `os::testing::force_fallback(true)` and must run the ADR-132 path: `precise 0`,
  one holder during a 30 ms sleep and none during a 400 ms one or at the end, median at most 3 ms and p99 at most
  10 ms (measured 1.1 to 1.3 and 2.1 to 2.5 ms), and the program does not end early.
- `runtime/test/winos_test.cpp` (the real PC): a waker held between the two steps of its wake-up while the slot is
  armed and used again (`test_wait_slot_late_wake_does_not_poison`), a wake-up before the wait ends only that wait,
  and close/open of a slot's handles (`open_handles` goes down by 2 and back).
- Mutants (each makes a test fail): `wake_slot` that does nothing (time_test), the slot published after the unlock
  (time_test), the slot not taken back after the wait (time_test), a wrong split (time_test), a pool that does not
  take slots back (time_test), `available()` not looked at (time_test); on the PC, a pre-roll of 0 (the 0.3 ms
  bound of the short sleeps), a `wait_slot_new` that fails, which is the ADR-132 fallback (the holders, the slots,
  the median, the ticks), a `wait_slot_wake` that does nothing (the shutdown of a timer thread in a precise wait
  of an hour never ends: the test's time limit), the old order of the wake-up with the conditional clearing of the
  event (`test_wait_slot_late_wake_does_not_poison`), and the old split that spins a whole short wait (the loops:
  1934 ms against 150, 6781 against 800).
