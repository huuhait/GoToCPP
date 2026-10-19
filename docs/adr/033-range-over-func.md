# ADR-033: Range over a func is Go's own rewrite: the body is a boxed closure and each loop has a state box

Status: accepted (2026-10-18). Builds what [ADR-017](017-generics-templates.md) and
[Phase 07 §2.5](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#25-range-over-func-iterators-go-123)
planned (the design of `cmd/compile/internal/rangefunc/rewrite.go`); the phase README's snippet is
refined as below.

## Context

`for x := range seq { body }` calls `seq` with the body as its `yield` function. The body can leave the
loop in ways a call cannot: `break`, `continue`, `return` (with values, from a function with named or
unnamed results and defers), `goto`, labeled `break` and `continue` of outer loops (range-over-func or
not), and `defer`, which belongs to the function that has the loop. An iterator is user code: it may
call `yield` after it returned `false`, keep `yield` and call it after the loop, call it from another
goroutine, swallow the panic of the body, or panic itself. Go checks these cases at run time with five
states and four messages.

## Decision

Copy Go's rewrite (its doc comment is the specification), in `internal/lower/rangefunc.go`:

1. **The body is a normal closure**, a C++ lambda with an explicit capture list (the Phase 03 rules),
   never `[&]`. A variable of the function that the body assigns, or whose address it takes, is boxed
   (`box.go`: the body is a `boxRegion` like a func literal). A by-reference lambda is wrong because an
   iterator may call a kept `yield` after the function's frame is gone (reproduced under ASan in the
   plan's audit); with a closure the late call reaches the state check and panics as Go does.
2. **Each loop has a boxed state object**, `gp::ptr<gp::rangefunc_state>`
   (`runtime/include/gocpp/rangefunc.hpp`), with Go's states (DONE, READY, PANIC, EXHAUSTED, and
   MISSING_PANIC as a message) and exact texts (`runtime error: range function continued iteration
   after function for loop body returned false` / `... after whole loop exit` / `... after loop body
   panic` / `range function recovered a loop body panic and did not resume panicking`, all
   `runtime.Error`s). `gp_enter()` starts the body, `gp_continue()` and `gp_exit()` end it,
   `gp_finish()` runs after the iterator returned. The checks stay on with `-nochecks`: they are what
   keeps a late call from running a body whose function is gone.
3. **Jumps** use Go's codes. `gp_next` (one per loop nest: the outermost loop's state holds it, and
   the bodies inside capture that box when they use it) is -1 for `return`, `2(i+1)` / `2(i+1)-1` for
   break / continue of the outer range-over-func loop number `i`, and a code below -1 for a `goto` or
   labeled jump to a place that is not a loop of the nest; the loops between the jump and its target
   return `false` and test the code after their call (`rfChecks`, Go's `checks`). Jumps inside one body
   are C++ jumps. The jump itself is made in the function that holds the target, after the call.
4. **Results** of a function with a `return` with values inside a body are variables of the function,
   boxed and given names of our own (the closures capture them by name, which a shadowing
   `for r := range` would hide): the body assigns them at the return site, as Go does (Go renames
   unnamed results; a deferred call that recovers a panic of the iterator after the body returned
   sees them and the function returns them: `tests/run/rangefunc_unnamed_after_recover.gp`).
5. **`defer` in the body** is the function's own: the body captures the function's defer frame
   (`gp_df`) by reference. It is safe because the body only runs when the state is READY, which is
   only while the loop's function is running (even when the iterator calls `yield` on another
   goroutine); a late call panics in `gp_enter()` before it touches the frame.
   **`recover()` in the body is nil**, always: the body is a closure that the iterator calls, not
   the deferred function that a panic runs, so it is not "called directly by a deferred function"
   even when the function that has the loop is that deferred function (checked against Go 1.25:
   `defer func(){ for i := range seq { println(recover()) } }()` prints nil and the panic goes
   on). `awareBody` leaves range-over-func bodies out (a function whose only `recover()` is in a
   body is not aware and takes no ticket), the range clause counts as the function's own code,
   and a `recover()` in a func literal inside the body is that literal's own. A function that
   `defer`s in a body and recovers in the deferred literal works as in Go: that literal is run by
   the panic. Tests: `tests/run/rangefunc_recover_*.gp`.
6. Body lambdas carry the template parameter `<int = 0>`, so the stack trace leaves them out as it
   leaves out the other generated helpers.

## Alternatives

- **A by-reference `[&]` lambda**, as the early README snippet had: rejected (dangling frame).
- **A goroutine or a coroutine for the body**: rejected; the cost is a thread or a stack per loop, and
  `defer` and `panic` would not belong to the function.
- **Skip the checks** (Go's `-d=rangefunccheck=0`): rejected; the exit criteria require Go's panics
  and a memory-safe late `yield`.
- **`next` in a variable of the function** instead of the state object: it would need a second
  boxed object per nest and a second capture; the state object is already there.

## Consequences

- Go's inlining makes `for x := range seq` over a simple iterator as fast as a plain loop; here each
  iteration is an indirect call of a `gp::func` plus the state check (measured in
  `docs/spec/cpp-mapping.md`, "Range over functions").
- A variable that a body changes is boxed (a heap block, a `gp::ptr` access), like any closure.
- A range over a func costs two allocations (the state and the closure) each time the loop runs.
- Frames of loop bodies are not in the stack trace (Go shows them as `main.f-range1`).
- Recursion through a loop body (`for v := range one { return rec(n-1) + v }`) uses three C++ frames
  per level (the function, the iterator, the body lambda behind a `gp::func` call), so it reaches
  the stack limit at a smaller depth than plain recursion (about 10,000 levels pass, 20,000 end
  in `fatal error: stack overflow`; Go's stack grows).
- The yield parameters may be C/C++ types (`func(yield func(std.string) bool)`): the range variable
  has that type (the check marks it opaque, the lowering takes the C++ type from the parameter's
  type expression); `tests/interop/rangefunc_cpp_values.gp`.
- Known gap: `for x := range G(...)` where `G` is a GENERIC function (or an instance) called with
  C/C++ type arguments (`genEach[std.string](...)`, `Map[int, std.string](...)`) is not recognised
  as a range over a func, because go/types gives the call an invalid type; it is lowered as a C++
  container range and fails with "range over a C/C++ value without begin() and end()". Workaround:
  assign the call to a typed func variable and range over that. In the plans TODO.

## Evidence

`tests/run/rangefunc_*.gp` (hand-written cases, the four misuse panics, a late `yield` under ASan) and
`tests/run/rangefunc_gen_*.gp`, 30 programs from `tools/rangegen` that nest range-over-func loops with
every kind of jump and print a trace: real Go and gocpp (both compilers) print the same.

## See

[Phase 07 §2.5](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#25-range-over-func-iterators-go-123),
[ADR-017](017-generics-templates.md), [ADR-011](011-errors-panic-defer.md),
`cmd/compile/internal/rangefunc/rewrite.go` (Go 1.25).
