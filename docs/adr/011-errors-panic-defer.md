# ADR-011: `error` is an interface, `panic` is a C++ exception, `defer` is a frame

Status: accepted (2026-10-13)

## Context

Go has `error` values, `panic`/`recover`, and `defer`, which runs at function exit, also during
a panic. C++ has exceptions and destructors. gocpp must keep Go's exact order: defers run last
in, first out, also during a panic, and `recover()` works exactly where Go says it does.

## Decision

- `error` is the Go interface `interface{ Error() string }` (`gp::error`, built like every
  interface, [ADR-016](016-interfaces-itabs.md)). `errors.New`, `Is`, `As`, `Unwrap` and `Join`
  behave like Go's.
- `panic(v)` throws a C++ exception, `gp::panic_error`, from Phase 02 on.
- A function with `defer` gets the **single-exit shape** below: one `gp::defer_frame`, the body
  inside `try`, and every `return` goes to `gp_exit`, where the defers run.
- A function that contains `recover()` gets two entry points, a normal one and a deferred one.
  The defer runner calls the deferred one, so `recover` only stops a panic when a deferred
  function calls it directly, as in Go.
- C++ exceptions from interop code become recoverable panics (Phase 05); `runtime.Goexit` and
  glibc thread cancellation are never treated as panics. A panic never unwinds through C code:
  every gocpp function that C can call runs inside a `noexcept` trampoline.

```cpp
R f(Args... args) {
    gp::result_slot<R> gp_r0;         // the results
    gp::defer_frame gp_df;
    try {
        /* body */                    // `return x` → { gp_r0.set(x); goto gp_exit; }
    } catch (const gp::goexit_signal&) {
        throw;                        // runtime.Goexit is never recoverable
    } catch (const abi::__forced_unwind&) {
        throw;                        // glibc thread cancellation must keep unwinding
    } catch (...) {
        gp_df.on_panic(std::current_exception());  // C++ exceptions become panics too
    }
gp_exit:
    gp_df.run();                      // defers LIFO; re-throws if nothing recovered
    return gp_r0.take();
}
```

## Alternatives

- **`setjmp`/`longjmp`** (rejected): skips C++ destructors.
- **Defers run from a C++ destructor** (rejected in the audit): an exception that nobody catches
  doesn't unwind, and a defer that panics inside a destructor calls `std::terminate`.

## Consequences

Good: a panic costs nothing until it is thrown; functions without `defer` get no frame and no
`try`; `!` ([ADR-013](013-must-errors.md)) reuses the shape, so defers run on its error path too.

Bad:
- Each `defer` costs a little, and gocpp needs C++ exceptions (a `-fno-exceptions` mode is only
  an idea for after 1.0).
- Destructors of C++ locals run when their block ends, before the function's defers. So C/C++
  objects that a defer uses are boxed (Phase 04).

## Evidence

- Spike 2026-10-13: the single-exit shape (a defer changes a named result, `recover` stops a
  panic) ran the same on GCC 15 and Clang 21 ([cpp-idioms](../../spikes/2026-10-13/cpp-idioms/)).
- Audit with Go 1.25: `recover` works through a func value, a method value, an interface method
  and `defer mk()()`, which is why there are two entry points.

See: [Phase 02 §2.6](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#26-runtime-v0-runtimeincludegocpprthpp), [Phase 03 §2.8](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#28-defer-without-recover), [Phase 04 §2.8](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#28-callbacks-and-panics-crossing-c-code), [Phase 05 §2.5](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#25-error-and-the-errors-model), [§2.7](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#27-panic--recover--defer-full-version).
