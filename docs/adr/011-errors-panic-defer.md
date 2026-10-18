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
- A function that contains `recover()` is *aware*: it takes a ticket at its start that the
  defer runner leaves only for the call it makes itself (while it runs its calls for a panic), so
  `recover` only stops a panic when a deferred function calls it directly, as in Go, however the
  function is reached (step 5.10 built this: one function body and the ticket, not the two entry
  points that the first version of this ADR planned; see the Notes below).
- C++ exceptions from interop code become recoverable panics (Phase 05); `runtime.Goexit` and
  glibc thread cancellation are never treated as panics. A panic never unwinds through C code:
  every gocpp function that C can call runs inside a `noexcept` trampoline.

```cpp
R f(Args... args) {
    gp::defer_ctx gp_dc = gp::take_defer_ctx();   // only if f contains recover(): the ticket
    R gp_r0{};                        // the results (gp::result_slot<R> for C/C++ types)
    gp::defer_frame gp_df;
    try {
        /* body */                    // `return x` → { gp_r0 = x; goto gp_ret0; }, one exit per return
    } catch (...) {
        gp_df.gp_on_exception();      // a panic, Goexit, or a C++ exception (a forced unwind is thrown again)
    }
    gp_df.gp_run(); return gp_r0;     // defers LIFO; throws again if nothing recovered
    gp_ret0:; gp_df.gp_run(); return gp_r0;
}
```

## Alternatives

- **`setjmp`/`longjmp`** (rejected): skips C++ destructors.
- **Defers run from a C++ destructor** (rejected in the audit): an exception that nobody catches
  doesn't unwind, and a defer that panics inside a destructor calls `std::terminate`.

## Notes (step 5.10, 2026-10-17)

- The shape above is the one Phase 03 built (one exit per `return`, at its line, so a panic in a
  deferred call shows the right line), with `catch (...)` instead of `catch (gp::panic_error&)`.
  The members of the runtime types are `gp_`-prefixed (`gp_set`, `gp_take` of `gp::result_slot`,
  `gp_on_exception`, `gp_run`): the first version of this ADR wrote `.set`, `.take`, `.run`.
- **Why a ticket and not two entry points.** A second entry point per aware function would have
  to be carried by `gp::func` (a third pointer: 32 bytes instead of 24, divergence D15) and by
  every interface thunk. The ticket is a thread-local pointer that `defer_frame::gp_run` sets
  right before it calls a call it knows to be aware, and only while it runs its calls for a
  panic that isn't recovered; the aware function takes it at its first statement, and the
  machinery clears it when the call ends. The deferred call is known to be aware when the emitter
  says so (`gp_push_aware`, `gp_push_call_aware`), when the code of a `gp::func` is in the
  runtime's registry of aware code (`gp::aware`, `gp::func_of_aware`: it is looked up only
  while a panic runs deferred calls), and, for an interface method, when `gp::icall` finds the
  thunk of the method in that registry (`gp::method_of_aware`): `icall` looks at the ticket (one
  thread-local load) and clears it otherwise. Every function without `recover` and without
  `defer` is emitted as before, and a normal call of an aware function finds no ticket.
- **C++ exceptions.** `catch (...)` in a function with defers is also how a C++ exception becomes
  a Go panic (a `gp::CppException` value) and how `runtime.Goexit` runs the deferred calls
  without being a panic. The cost is a lost throw-site trace for such an exception (the trace
  starts in the function that caught it) and, as always with exceptions, nothing on the path
  that doesn't throw: the performance of a call with a defer is unchanged within noise
  (about 23 ns at -O2).
- Go's `recover` has one behavior that depends on its compiler: when gc inlines a function into
  the deferred wrapper, `recover` in a function that the inlined code calls can succeed
  (`defer call(h)` with `func call(f func()) { f() }`; `defer s.Run()` where `Run` calls a func
  field; `f := t.M; defer f()` where `M` calls a helper that recovers; `defer i.M()` with a value
  receiver). gocpp follows the rule, not the inliner: it returns nil there, as gc does with
  `-gcflags=-l` (divergence D28 in docs/spec/language.md).

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
  and `defer mk()()`, which is why `recover` needs to know how its function was called.

See: [Phase 02 §2.6](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#26-runtime-v0-runtimeincludegocpprthpp), [Phase 03 §2.8](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#28-defer-without-recover), [Phase 04 §2.8](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#28-callbacks-and-panics-crossing-c-code), [Phase 05 §2.5](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#25-error-and-the-errors-model), [§2.7](../../plans/2026-10-13-gocpp/phase-05-interfaces-errors/README.md#27-panic--recover--defer-full-version).
