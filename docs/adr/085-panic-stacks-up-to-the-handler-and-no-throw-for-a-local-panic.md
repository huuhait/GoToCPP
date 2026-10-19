# ADR-085: A panic records its stack up to the function that handles it, and a panic statement in a function with defers does not throw

Status: accepted (2026-10-20)

## Context

The benchmark `panic_recover` (step 15.1: a function with a deferred `recover()` that panics with an `int` or an `error` in two calls
out of three) ran 24x (`-release`) to 35x (`-O2`) slower than Go 1.25.14. A profile of the compiled program (a flat sampler and gdb
breakpoints on `_Unwind_Find_FDE`) showed that more than 90% of the time was the C++ unwinder, and the walks were these, per panic,
in `risky` (one frame between the throw and the handler):

- the throw itself: `_Unwind_RaiseException`, a search phase and a cleanup phase over the frames, about 550 ns for two frames
  (a microbenchmark of a plain `throw`/`catch` of a small object measures the same);
- `defer_frame::gp_adopt`, which found the type of the exception by `try { throw; } catch (panic_error&)`: a second throw, 740 ns;
- `panic_error`'s constructor, which recorded the whole stack with `backtrace` for the trace of an unrecovered panic (the unwinding
  removes the frames before `run_main` sees the panic): 450 ns for 8 frames, 100 ns to start and about 50 ns a frame;
- `_Unwind_Resume`: the temporaries of the `throw` expression and the by-value arguments of `throw_panic` had destructors, so the
  frame of the thrower had a cleanup that the unwinder ran on the way out (a third walk, 150 ns).

## Decision

1. **The generated handler is typed**: `catch (gp::panic_error& gp_p) { gp_df.gp_take(gp_p); } catch (...) { gp_df.gp_adopt(); }`.
   Only an exception that is not a Go panic (Goexit, a C++ exception, a forced unwind) goes through `gp_adopt`, which still finds its type
   by rethrowing. `gp_on_exception` stays for code that doesn't know the type. `gp_run` catches the panics of deferred calls the same way.
2. **A panic records only the part of its stack that the unwinding is about to remove.** Every `defer_frame` is in a thread-local list
   (`detail::top_defer`; each frame keeps a 32-bit distance to the next one, which fits the 96 bytes of the frame, and a longer distance ends
   the list). The constructor of `panic_error` walks the stack with `_Unwind_Backtrace` (`os::stack_until`) only up to the frame that holds
   the innermost listed frame, which is the function that will catch the panic: its `catch` is the first handler the C++ unwinder finds.
   The callers of that function are still on the stack when its `gp_run` has run the deferred calls and knows that none recovered the panic.
   `gp_run` then records them (`panic_error::gp_complete`, `os::stack_above`: the frames outside the frame of that function) and throws
   the panic again. The same record is made where a stack of a panic is read while its function is on the stack: `panic_chain` (both panics
   of the chain), `fatal_trace` (the running panic) and `on_terminate`. A panic that is recovered never walks its callers. With no function
   with defers on the stack the whole stack is recorded at the throw, as before; so it is with AddressSanitizer (its fake stack gives locals
   addresses that say nothing about the real stack). On Windows x86-64 `os::stack` and its two new siblings are one loop of
   `RtlCaptureContext`/`RtlLookupFunctionEntry`/`RtlVirtualUnwind` (the walk that `RtlCaptureStackBackTrace` does, with a stop and
   without its chunks of skip and count, which gave out after about 310 frames: `trace_test`'s 300-deep panic lost its outer frames
   as soon as the runtime had two frames more); another CPU keeps the old whole-stack behavior. The list also drops an entry whose frame is
   below the one that makes the panic (a `longjmp` over a function with defers). The recorded frames are the same, so the printed trace of an
   unrecovered panic is the same (`tests/run` compares it with Go's).
3. **The throw sites have no temporaries with destructors.** `panic_error` has constructors that build the value, the text and the stack
   state from raw arguments (`basic_t`, `iface_t`, `string_t`, `runtime_t`), so `throw panic_error{...}` has nothing to destroy while the
   unwinder runs, and the runtime errors that Go keeps as constants (nil dereference, division by zero, negative shift) use a static text.
4. **A panic statement in the function that has the frame doesn't throw.** The lowering of `panic(v)` in a function with defers (not in a
   func literal, whose function has its own frame or none, and not in the body of a range over a func, a lambda that can't jump out) is
   `gp_df.gp_panic(gp::new_panic(v)); goto gp_pan0;`. `gp_panic` does what the typed handler does for the panic (the chain with the panic
   that is running, the count of goroutines that handle a panic, the stack: this call's return address and the rest by `gp_complete`),
   with the panic on the heap, owned by the frame (`gp_local`); the `goto` leaves the try block and destroys its locals, in the order
   that the unwinding would, to the exit at the closing brace, which runs `gp_run`. When no deferred call recovers the panic, `gp_run`
   throws the panic by moving it into an exception (`panic_token` has a move that keeps the count).

## Results

`panic_recover` (1,000,000 iterations, Go 1.25.14 = 52 ms), median CPU time of 7 runs, pinned to one CPU (the machine was shared; the
numbers are a ratio between interleaved runs):

| | before | typed catch | + lazy stack, no cleanups | + no throw for the statement in the function |
|---|---|---|---|---|
| g++ `-release` | 1304 ms (25x) | 889 ms (17x) | 712 ms (14x) | 59 ms (1.1x) |
| g++ `-O2` | 1859 ms (36x) | 1466 ms (28x) | 917 ms (18x) | 62 ms (1.2x) |
| clang++ `-release` | 1961 ms (38x) | 1475 ms (28x) | 1205 ms (23x) | 62 ms (1.2x) |

The same program with the panic one call away (`helper(i)` panics, `risky` has the `defer`/`recover`) is the case that still throws:
see the step report for its numbers. A throw costs two walks of the frames between the thrower and the handler (the search and the
cleanup phases, 6 FDE lookups for one frame) and the C++ library offers no way around them.

## Consequences

- A frame costs one thread-local load and two stores more (and the destructor one store): `defer_loop` is 3-6% slower.
- `panic_error::gp_pcs` of a panic that is caught by C++ code outside a function with defers and read there (not through `gp_run`)
  holds only the inner part, and `gp_bound` is not zero; the printed trace then ends with Go's "...additional frames elided..." line.
  The runtime's own readers complete the panic first.
- `gp_panic` allocates the panic (the throw allocated it too, in the exception).

## Alternatives that were not taken

- **Frame pointers** (`-fno-omit-frame-pointer`, a walk of 50 ns for 10 frames): the code of libstdc++ and libc has none, so a callback
  through them (`std::sort`'s comparison, a `std::function`) leaves out a frame of the program silently. A global flag costs the generated
  code a register.
- **Running the deferred calls on top of the panic's stack, as Go does**, so that the stack is the one the trace needs: the deferred calls
  would run before the destructors of the frames between, which changes what a C++ destructor (a lock guard) sees.
- **A cache of walks** keyed by the return addresses: it is as long as the walk.
