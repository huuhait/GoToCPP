# ADR-104: Goroutines stay threads, the main goroutine gets a large stack, and a scheduler needs new evidence

Status: proposed (2026-10-20); option 2a chosen by the user and withdrawn on 2026-10-23: the main thread's `RLIMIT_STACK` is raised instead (the notes below). Post-1.0 direction. Keeps [ADR-007](007-goroutines-os-threads.md) and the user's decision Q4 ("no goroutine scheduler, now or later", [TODO.md](../../plans/2026-10-13-gocpp/TODO.md)); a scheduler would need a new ADR that supersedes ADR-007, and only the user can reopen Q4. Phase 08 left a note to "REVISIT in Phase 15 if programs with 100k goroutines matter" ([STATE](../../plans/2026-10-13-gocpp/phase-08-concurrency/STATE.md)); this is that revisit.

## Context

What the thread model costs, measured on Linux 6.19 ([D7](../spec/language.md#d7-goroutines-are-os-threads)): 10 to 130 us to start a thread; 1,000 blocked goroutines start in 0.1 s, 10,000 in 0.8 s (65 MB), 100,000 in 13 s (1.3 GB); a kernel before 6.13 or the default `vm.max_map_count` of 65,530 stops at about 20,000 to 30,000 threads; a hand-off on an unbuffered channel is about 1 us on idle cores and 7 to 15 us when the receiver must be woken, against 0.2 us in Go. `iter.Pull` is a thread hand-off too: 100,000 live pulls take 18 to 21 s and 1.3 GB (7.10 low in TODO.md). A finished goroutine's thread is reused, so a loop of a million short goroutines costs about 6 us each ([ADR-026](026-channels-and-thread-reuse.md)).

The stack is the larger problem than the count. A C++ frame cannot move or grow, so every thread has a fixed 8 MiB stack ([D24](../spec/language.md#d24-large-values-on-the-c-stack)), against Go's growing stacks up to 1 GB. The same limit shows up in five unrelated places: `reflect.DeepEqual` fails on a list deeper than about 6,000 nodes at -O0 ([ADR-035](035-reflect-subset-on-descriptors.md)); `encoding/json` decodes objects about 9,000 deep and `Marshal` of maps 3,000 to 4,000 deep overflows (step 9.11, ADR-102); `regexp` `x{1,1000}y{1,1000}` overflows under clang++ -asan ([ADR-083](083-sort-regexp-ports-and-the-test-matcher.md)); Go's `TestCVE202230632` (10,001 recursions) and `TestCVE202230630` had to be left out of the ports' tests; and a pointer-recursive function of 100,000 frames overflows with clang++ ([Phase 08 README](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md)). The conformance suite has a `stack-growth` category ([conformance guide](../guides/conformance.md)), and `fixedbugs/issue29362.go` can die under load because it starts 1,000 goroutines as threads (15.10 lows in TODO.md). Step 9.11 found the same limit as a security problem: hostile `encoding/json` input (a deeply recursive struct, about 1,000 levels at -O0) crashed the process, now guarded by a depth check (ADR-102); the `fmt` nesting low and step 8.13 are the same limit again.

What the model buys: a C library that needs one fixed thread (OpenGL, GUI toolkits) works; gdb shows goroutines as threads ([ADR-066](066-debugger-support.md)); ThreadSanitizer needs no fiber annotations ([ADR-062](062-race-detector-on-tsan.md)); a blocking C call blocks one goroutine and nothing else; `runtime.LockOSThread` is trivially true.

Not measured: thread cost on Windows (every number above is Linux) and the memory a real program with 10,000 goroutines of varied stack depth uses.

## Options

1. **Keep threads and fix nothing.**
2. **Keep threads and fix the stack story.** (a) Run `main.main` on a thread that the runtime starts itself (it already starts threads with a chosen stack size, [ADR-053](053-windows-os-layer-on-createthread.md) and `runtime/src/os/`), with a large, lazily committed stack (a size to be measured, 64 MiB to 1 GiB of address space) set by an environment variable or a build setting (a `runtime/debug.SetMaxStack` call happens after `main` has started, so it cannot size main's own stack; today `set_max_stack` in `runtimepkg.hpp` only stores the value); (b) goroutines: `gp::detail::set_stack_size` already sets the stack of goroutines that start afterwards (floor 1 MiB, default 8 MiB, `goroutine.hpp`) and says that the main thread's stack is reachable only through `ulimit -s`; `SetMaxStack` could call it, default unchanged at 8 MiB; (c) make `fatal error: newosproc` say which limit was hit (`pids.max`, `max_map_count`, `ulimit -v`); (d) measure Windows.
3. **An M:N scheduler on stackful coroutines.** Needs fixed-size stacks anyway (C++ frames cannot move), so the stack size problem stays, and `vm.max_map_count` shows up again unless stacks come without guard pages or the kernel supports guard markers (6.13). Also: a blocking C call (sqlite, raylib) takes a worker with it; thread-bound libraries break; ASan and TSan need fiber annotations; `LockOSThread` becomes real; the gdb view and the deadlock detector ([ADR-077](077-deadlock-detection.md)) are redone. The gain is cheap `go`, cheap hand-offs and an `iter.Pull` that is a stack switch.
4. **Stackless goroutines** (C++20 coroutines everywhere). A function that can block would have to be a coroutine, including every function that calls one, through interface and func values. Rejected: it changes the generated code of the whole language.

## Decision (proposed)

Do option 2 after 1.0, starting with (a) because it is the cheapest and removes the stack failures of the main goroutine, where nearly all of the cases above happen. Keep Q4. Write down in the guide for Go developers which limits exist and how to read the `newosproc` message. Option 3 is not recommended on today's evidence: no program in the repository needs more than the thread model can give, and the stack problem is not solved by it.

## Note (2026-10-23): option 2a is chosen (withdrawn the same day, next note)

The user chose option 2a, over switching the main thread's stack (which keeps gdb's single-thread
view but needs per-CPU assembly, sanitizer fiber annotations and a Windows fiber). `gp::run_main`
runs the package initializers and `main.main` on a thread of `gp::os::thread_run` with a stack of
1 GiB (`gp::detail::main_stack_size`), and the main thread waits for it; when the system refuses
that thread, smaller stacks down to 16 MiB are tried, then the main thread itself. A recursion of a
million pointer-chasing frames on the main goroutine now works as in Go. Goroutines keep 8 MiB
(option 2b stays open), so Go's own `TestCVE202230630` (in a test's goroutine) still overflows.
What users see: gdb shows a second thread (`Thread 2 "prog" hit Breakpoint 1`, goroutine N is
thread N+1, a raw `bt` ends in `run_main_goroutine`, `thread_main`, `start_thread`, `clone3`), and
the main goroutine is not the process's first thread ([debugging guide](../guides/debugging.md),
[D7](../spec/language.md#d7-goroutines-are-os-threads)).

## Note (2026-10-23): option 2a withdrawn; the main thread's RLIMIT_STACK is raised

Option 2a was withdrawn after the review measured a 1.7-2.8x single-thread slowdown on `-release`
benchmarks (embedding 2.8x, defer_loop 2.4x, method_value 2.1x, binarytrees 1.75x,
json_marshal_struct 1.7x). Every program was multithreaded from its first instruction:
`pthread_create` clears glibc's `__libc_single_threaded`, so the reference counts go atomic and
malloc takes its locks (the same numbers come from the base binary with one `go func(){}()`). The
user chose to keep `main.main` on the process's startup thread and raise its stack limit instead.

- **Linux:** `gp::os::start` (`src/os/linux/thread.cpp`, `start_threads`) sets the soft
  `RLIMIT_STACK` to 1 GiB, Go's limit (at most the hard limit), before anything deep runs. A
  soft limit above 1 GiB stays; unlimited becomes 1 GiB, because an unlimited main stack would
  grow until the memory is gone and end with the OOM killer instead of Go's report. The kernel checks the current limit whenever the main stack grows, but the
  mapping below the stack was placed at exec with the limit of that time: `mmap_base` lies at least
  128 MiB plus the layout's randomization below the stack's top. The thread layer therefore reads
  the end of the mapping below the stack from `/proc/self/maps` and puts the stack's lowest
  address at the higher of `top - limit` and `that end + stack_guard_gap` (1 MiB), so the overflow
  report (`fatal error: stack overflow`, exit code 2) comes at the real end. The SIGSEGV handler and
  its alternate stack are installed in every case (before, a soft limit of `unlimited` or below
  64 KiB left the program to die of a bare SIGSEGV with a core dump). The bounds can go out of
  date: `syscall.Setrlimit` of `RLIMIT_STACK` computes them again, and for what the runtime does
  not see (C's `setrlimit`, a mapping put into the gap with an mmap hint, no `/proc` at start) the
  handler also takes a fault of the main thread just below its stack pointer, with the pointer in
  the main stack's region, as the overflow. No re-exec. Programs that the
  process starts inherit the raised soft limit.
- **Windows:** the PE header reserves 1 GiB for the main thread (`-Wl,--stack,1073741824` and the
  runtime's `.drectve`); it is address space, committed as the stack grows. Threads that C code
  starts without a size reserve as much (address space only); goroutines keep their own 8 MiB.
- **Goroutines** keep 8 MiB (option 2b stays open): Go's `TestCVE202230630`, in a test's goroutine,
  still overflows.

Measured on Linux 6.19 x86-64 (clang++ `-O0`; `length(n) = 1 + length(n.next)` uses 80 bytes a
frame, `down(n) = 1 + down(n-1)` 32 bytes; g++ the same; the stack is what the overflow report
shows):

| At exec | Stack of the main thread | `length` frames (list nodes) | `down` frames |
|---|---|---|---|
| dev: 8 MiB soft (the default) | 8 MiB | 104,659 | 261,608 |
| 8 MiB soft, hard unlimited, ASLR | 1024 MiB | 13,421,612 | about 33.5 million (30M ran, 35M overflowed) |
| 8 MiB soft, no ASLR (`setarch -R`, gdb) | 127 MiB (the mapping below) | 1,664,482 | 4,161,212 |
| `ulimit -s unlimited` (ASLR or not) | 1024 MiB (soft set to 1 GiB) | about 13.4 million | about 33.5 million |
| 64 MiB soft, hard unlimited | 1024 MiB | about 13.4 million | about 33.5 million |
| 8 MiB soft, hard 256 MiB | 256 MiB | about 3.3 million | about 8.4 million |
| 256 MiB soft, no ASLR | 256 MiB (exec left room) | about 3.3 million | about 8.4 million |
| 8 MiB soft, hard 8 MiB | 8 MiB | 104,659 | 261,608 |
| Go 1.25 (1 GB limit) | | more than 30 million | between 40 and 60 million |

Lists of 100,000, 1 million and 5 million nodes all work with ASLR (5 million: 1.7 s); without ASLR
1 million works and 5 million overflows. Every row that overflows ends with Go's report and exit
code 2, never a bare SIGSEGV. The process stays single-threaded (one task until a `go` statement).
`-release`, one CPU, base (dev) and branch interleaved, 10 runs each, medians: embedding 1172 / 1174 ms,
binarytrees 768 / 757 ms, defer_loop 1086 / 1186 ms. defer_loop's machine code is the same as dev's but
for padding, it is as slow with the limit left alone (hard limit 8 MiB), and with 64-byte function and
loop alignment (`GOCPP_CXXFLAGS=-falign-functions=64 -falign-loops=64`) both are 1091 ms: the 9% is where
the linker put the loop, not the stack.

## Trigger

- A real program that needs more than about 30,000 goroutines blocked at the same time and cannot be restructured with a worker pool.
- Windows measurements that show thread creation or memory making 10,000 goroutines impractical there.
- A library or the standard library ports needing cheap `iter.Pull` or channel hand-offs (for example a range-over-func heavy package at 1 us per element), shown by a benchmark.
- The user reopens Q4.

## Consequences

- Good: deterministic, debuggable concurrency stays; the most common stack overflow (deep recursion on the main goroutine) goes away without touching codegen.
- Bad: goroutine-heavy programs written for Go (hundreds of thousands) remain unsupported ([D7](../spec/language.md#d7-goroutines-are-os-threads) says so); deep recursion inside a goroutine still stops at the goroutine's stack size.
- Bad (option 2a): `main.main` leaves the process's initial thread, so a library that needs the process main thread (Cocoa- or GLFW-style) and any tool that assumes thread 1 is main would see a difference; Go pins main to the initial thread with `LockOSThread` in init, and the same would be needed here. On Windows the main stack is already set by a linker flag (`-Wl,--stack,8388608`, [build-link.md](../spec/build-link.md)), so there the change is a larger flag, not a thread.
- Uncertain: whether address-space limits (`ulimit -v`, containers) make a large main stack a problem; whether the default for goroutines can rise without hurting 100,000-thread programs.

## Evidence

[D7](../spec/language.md#d7-goroutines-are-os-threads), [D24](../spec/language.md#d24-large-values-on-the-c-stack), [ADR-007](007-goroutines-os-threads.md), [Phase 08 STATE](../../plans/2026-10-13-gocpp/phase-08-concurrency/STATE.md) (scale numbers, the REVISIT note), TODO.md lows 7.10, 9.4c, 9.4b, 15.10, [runtime.md](../benchmarks/runtime.md) (`goroutine_spawn`, `chan_pingpong_unbuf`).

## See

[Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen) (a scheduler is deliberately not on the list), [Phase 08 §2.2](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#22-goroutines--os-threads).
