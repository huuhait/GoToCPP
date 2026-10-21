# ADR-124: Error return traces are a register of `!` hops per goroutine, matched by the identity of the error

Status: proposed (2026-10-21; revised twice the same day after the independent reviews) as the design of
steps 10.7b to 10.7e; nothing of it is built yet. It becomes accepted when the user has answered the
open questions at the end (creation sites, the `-release` default, printing in `main!` and panics,
what a cut does to a trace).
Builds [ADR-013](013-must-errors.md) and [ADR-037](037-must-rewrite.md) (what `!` means and how
it is lowered) with [Phase 10 step 10.7](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md).
It amends ADR-037 in one place: the lowering of `!` keeps a second helper of `gocpp/must` in the output
(`Hop`, besides `Exit`).
Evidence: [spikes/2026-10-21/error-traces](../../spikes/2026-10-21/error-traces/).

## Context

Phase 10 says an error return trace should show the path an error took: each `!` that passes an
error up records its `.gp` call site, `errors.Trace(err)` and `%+v` show them, and the error
value is not wrapped, so `err == io.EOF`, `errors.Is`, `errors.As` and type switches keep
working. The README named the hard parts: shared sentinel errors, errors that are handled and
dropped, and errors sent over channels.

What the code looks like today, which fixes where a trace can be recorded:

- `!` is lowered in the Go AST (ADR-037): `v, must_e := F(a); if must_e != nil { return ...,
  must_e }`, and the same shape for every context (left operands, loop conditions, `else if`,
  `case`, `&&`/`||`, `defer F!()`, closures, the panic form, `main!`). The emitted C++ is
  `gp::error must_e1 = std::get<1>(gp_t0); if (must_e1 != nullptr) { return {0, must_e1}; }`.
  The error branch is the only place that runs when an error is passed up, so it is the only
  place a hop needs. `failStmts` in `internal/check/mustlower.go` writes every error branch.
- A goroutine is an OS thread, and the worker threads are pooled and reused
  ([ADR-007](007-goroutines-os-threads.md), `run_goroutine` in `runtime/src/goroutine.cpp`).
  Thread-local storage is goroutine-local storage.
- `error` is a 32-byte interface: a `std::shared_ptr<void>` to the value (a pointer type shares
  the owner of the pointer, any other type is boxed once when it is converted, a pointer into data
  that nobody owns gets a shared `foreign_owner()` or `static_owner()` block), the type and the
  itab ([ADR-016](016-interfaces-itabs.md)). There is no room for a hidden field, and no
  gocpp-owned type can be assumed: an error may be a user struct, a ported standard type, or a
  C++ value.
- `defer errors.Wrapf(&err, ...)`, `fmt.Errorf("%w")` and `errors.Join` replace the error by a new
  value that has `Unwrap`; they are gocpp's own code (`stdlib/errors/wrapf.gp`, the native `fmt`
  engine, `errors.Join`). A user type with `Unwrap` is not.
- Panics already print Go's `goroutine 1 [running]:` stack with `.gp` file and line, through the
  `#line` lines of the generated C++ ([ADR-085](085-panic-stacks-up-to-the-handler-and-no-throw-for-a-local-panic.md));
  `runtime/src/frames.cpp` turns code addresses into the Go names and positions, and hides runtime frames.

Measurements made for this note are in Evidence. The facts that shape the design: the error branch
runs only for an error, so a call in it costs nothing that runs on the nil path **if the error is
passed by value** (passed by reference, the local escapes and g++ adds 45 instructions to the nil path);
a hop that has to look into the error (`Unwrap`) would run user code and cost 15 to 20 ns, so it
does not; and the stack position of a frame depends on the frame sizes of the functions, which change
with the optimization level, so no decision may rest on it except one that is safe either way. A third
fact came from the real binaries: a function that is called through an interface value or a function value
returns into a compiler-made thunk (`gp_thunk_*`, a `gp::func` invoker), whose name says nothing about the caller.

## Decision

### 1. What a trace is

A trace is **the list of `!` sites that one error object passed on its way up the stack in its most
recent propagation, in one goroutine, in which every site was made in the function that the previous site's
function returned into.** It belongs to a propagation, not to a value: when a shared value such as `io.EOF` is
returned from many places, each propagation has its own trace, and only the latest of a goroutine is kept.

What is promised, exactly (section 12 has what is not):

1. Every site in a trace is a `!` that an object of one identity chain passed: the error itself, or a runtime wrapper (`Wrapf`, `%w`, `Join`) that took over its identity and its hops (section 6). Identity is checked in every hop.
   No site is a `!` that no object of that chain passed. The chain can include **earlier propagations** (item 4), so this is not a promise that every site belongs to the propagation being traced.
2. Two neighbours in a trace were **checked** as caller and callee: the code that made the first hop returns into the function
   that made the second (names from the symbol table, compared when the trace is read). A link that cannot be checked is
   **a cut**, never accepted: the trace is the part above the cut. The one exception is the pair around a gap frame (more than 32 hops, section 4): the hops in
   the gap are not stored, so a cut among them cannot be seen; the gap frame marks the place.
3. A trace may therefore be **shorter** than the path, and the part it drops is the part below the cut, **the origin side**,
   which is the most useful part. What cuts: a function without `!` that returns the error (`return f()`), a recovered panic,
   a hop made in a function that was called through an interface value or a function value, an inlined hop-bearing callee, a wrapper
   type that the runtime did not make, a wrapper that the runtime made only to be logged (the real error then restarts above it), a binary without symbols (there every link is a cut: a trace is the last hop). Open question 4.
4. In **one shape** a trace can begin with hops of an earlier propagation of the same identity: the earlier hop returned into a function F,
   and a later `!` in F (the same call or another call of F) puts the same object up again (section 5, scenario 11). **A wrapper or `Join` of the identity inherits the
   identity's earlier hops**, so a later `!` in the function that the last hop returned into can show them even for an error that was made fresh by `fmt.Errorf("g: %w", ErrPerm)`
   or `errors.Join(...)` from a sentinel that an earlier `!` had passed (scenario 17). Nothing else is known to do that.

The length of a trace depends on the build: inlining and tail calls can join links that are cut at `-O0` (a plain adapter that the compiler
inlines into its caller makes the link hold), and can cut links that hold at `-O0` (an inlined `!` callee). A trace follows the rules above in each build, and
is not the same in all of them (scenario 7).

The trace is never stored in the error and never changes it. The error value is exactly what the program made: same type, same pointer,
same `==`. (`errors.Traced` is the one visible exception, section 7.)

### 2. Where it is stored: one register per goroutine

Each thread has one **register**, in `runtime/src/etrace.cpp`, about 850 bytes:

| Field | What |
|---|---|
| identity | the error the trace is about: the owner as a `std::weak_ptr<void>` in raw storage, the data pointer and the type |
| total | the number of hops of this propagation (more than 32 means a gap in the middle) |
| last frame | the stack address of the latest hop's frame (used only to refuse a continuation, section 4) |
| sites | 32 slots of `const site*` (`{file, line, function}` constants): the first 16 hops, then a ring of the latest 16 |
| heres, rets | per slot, two code addresses: one inside the code that made the hop (`hop`'s own return address) and the one that code returns to (`__builtin_return_address(0)` of the function) |

How the storage works, following the convention of `runtime/include/gocpp/os.hpp` (a trivial type in
`constinit thread_local` form needs no initialization guard and no destructor; `tls_on_thread_exit` is
for what must run when a thread ends):

- The register is `static constinit thread_local reg_t reg{}`, a trivial struct. The weak reference lives in raw
  aligned storage and is made, assigned and released by functions of `etrace.cpp`, never by a C++
  destructor.
- **The end of a goroutine resets the register** (`run_goroutine`, after `g->body.reset()`: drops the weak
  reference and sets `total` to 0). A pooled worker therefore starts its next goroutine with an empty register,
  and the entry has nothing to clear. The memory of the register itself is part of the thread's TLS block
  and goes with the thread, so there is no "free" step. (The first version said "the entry clears and the end
  frees": that was two mechanisms for one job, and a register that is reset at the end needs no more.)
- **Threads that no `go` statement started** (a C or C++ host calling a gocpp library, the main thread of a
  `cpplib`): the first hop on such a thread calls `gp::this_goroutine()`, which gives the thread a goroutine record
  and registers `release_adopted` with `tls_on_thread_exit`; `release_adopted` calls `etrace::reset()`. The OS
  layer keeps **one** registration per thread ("only the last call counts"), so etrace never registers its
  own: it hangs on the one the goroutine layer has. If the OS cannot register (it may fail silently), one
  control block of the last error is not released, which is the condition under which the goroutine record leaks
  today.
- **Memory is bounded:** one register per thread (850 bytes), and the weak reference holds at most one error
  per thread: the destructor of the error has run when the program dropped it, but the control block (with
  `make_shared`, the storage of the object) stays allocated until the next restart, the end of the goroutine
  or the end of the thread. An error type with a large inline buffer holds that buffer until then.
- If a scheduler ever replaces threads ([ADR-104](104-goroutines-stay-threads-the-main-goroutine-gets-a-large-stack-and-a-scheduler-needs-new-evidence.md)),
  the register becomes part of the goroutine switch.

Why this and not the other candidates (details in Alternatives): a thread-local register costs a store into
memory the thread already owns, with no lock, no allocation and no atomic operation per hop (one weak
reference is taken when a new error value enters it). The identity is **weak**, so the register never
keeps the value alive and a freed object whose address is reused by a new error is not mistaken for the old one.

### 3. Recording: one call in the error branch

For every `!` that handles an error, the lowering writes this as the first statement of the error
branch:

```cpp
if (must_e1 != nullptr) {
    static constexpr gp::etrace::site gp_site{"/work/app.gp", 19, "main.loadConfig"};
    gp::etrace::hop(&gp_site, must_e1, __builtin_return_address(0));
    return {0, must_e1};
}
```

The Go-AST side is `gp_must.Hop(must_e, "/work/app.gp", 19, "main.loadConfig")`, a second helper of the hidden
package `gocpp/must` that stays in the output like `Exit` (the comment of `internal/check/mustpkg.go` says
"no call of them is left" and must be amended in 10.7b; `mp.dropImports` must keep the import of `gocpp/must`
in a file whose only use is `Hop`). `internal/lower` writes it as the statement above, the way it writes
`gp_must.Exit`. `internal/check/mustlower.go` `failStmts` is the single place that adds it, so every shape of
ADR-037 gets it: statements, left-hoisted operands, loop conditions, `else if`, `case`, `&&`/`||`, closures, the panic
form (the hop comes before the panic), `main!`, `init!`, package-level initializers and the `t.Fatal` form.

- The **site** is a `static constexpr` object in `.rodata`: the file and line of the `!` in the user's text (the position of the
  call the rewrite replaced, not the `#line` of the lowered statements) and the **name of the Go function that holds the `!`**, which is
  the name that `#func()` gives for that place (`main.loadConfig`, `main.(*Server).Start`, closures numbered as in the source: `main.main.func1`).
  A generic function has one site per `!`, not per instance. The name is **static**, so inlining and LTO cannot change it, and it
  always agrees with the file and line next to it. Two cases differ from what a panic trace prints for the same code: a `!`
  in a value macro has the macro's own name at the line of the macro text (`main.Parse`; `#func()` there says the function of the use), and the
  closures inside a function that a wrap macro wraps are numbered as in the source (`main.viaWrap.func1`), where a panic trace shows the lowered
  closures (`main.viaWrap.func1.1.1`). Scenario 12 has the output. Symbol names are used in one place only, the link check (section 4);
  they are never shown.
- **`hop` takes the error by value**, is `noinline`, `cold` and `noexcept`, and calls no user code: it does not call
  `Error`, `Unwrap`, `Is`, `==` or any method of the error. The copy of the error happens inside the cold call; the
  nil path executes no part of it and its code is unchanged within noise (Evidence).
- `defer F!()` records its hop only on the path where the error becomes the function's error, never when it is
  dropped because the function already returns an error or a panic is running (a dropped hop must not replace the
  register). That hop is made in the deferred closure, which returns into the defer machinery, so the function saves its own return
  address in a local when it starts (`const void* gp_ret = __builtin_return_address(0)`, one store, only in functions that have a
  `defer F!()`) and the closure passes it: the hop is linked to the caller's hop like any other (scenario 8).
- On the error path a hop is: the address of the thread-local register, the identity test (three compares and, only
  when the pointers match, two owner compares), the frame test, five stores (site, two addresses, last frame, and
  the count), and, only when a new error value enters the register, one weak reference (two atomic updates). It allocates
  nothing.

### 4. When a hop continues a trace, restarts it, and what `Trace` keeps

`hop(site, e, ret)` runs two tests. Let `r` be the register of the thread and `p` the address of the frame of `hop` itself
(`__builtin_frame_address(0)`; the stack grows down, so an older frame has a larger address).

1. **Same error object.** `e` has a data pointer (not a typed nil), and its data pointer, type and owner (control block,
   compared through `weak_ptr::owner_before` both ways) equal the register's identity. This is the identity of
   the object the interface points to, not Go's `==` (two equal struct values in two boxes are two errors here; a value
   type is boxed once per conversion to `error`, and copies of the interface share the box). The three parts are all needed: `foreign_owner()`
   and `static_owner()` blocks are shared by many objects, and a pointer to a struct and a pointer to its first field
   have the same address. A typed nil pointer, or an error without data, has **no identity**: it always restarts and is
   never found by `Trace`.
2. **Older frame.** `p` is larger than the last hop's frame address `r.last_frame`. A continuation is made by a caller
   of the last hop's function, whose frame is always older, so "not older" is a **certain restart** whatever the
   layout is. The test is not used to say "yes": when two functions at the same depth (siblings) are
   compared, which frame is older depends on their frame sizes and decides nothing by itself.
3. **Continue** when both hold: store the hop in the next slot (the identities stay as they are). **Restart** otherwise: `total` becomes 0, the hop is
   stored first, `e` becomes the identity. After 32 hops the slots 16 to 31 are a ring: the first 16 hops (the origin) and the
   last 16 (the top) are kept and `total` counts the rest. A recursion that fails at depth 1,000 costs one store per hop.

`Trace(err)` (and `%+v`) then **checks the links**, once per read, on the stored addresses: the link from hop `i` to hop `i+1`
holds when the function of `rets[i]` is the function of `heres[i+1]`, both resolved by `frames.cpp` to a name that is `shown`
(the function that hop `i`'s code returned into is the function that made the next hop). The trace is the **longest suffix whose links hold**; a failing
link drops every hop below it. **A link that cannot be checked fails**: a return address in a function that is not `shown` (an interface thunk
`gp_thunk_*`, a `gp::func` invoker, a runtime helper), a name that does not resolve, or a binary without symbols. The one link that is not checked is the
junction of the ring's gap: the omitted hops may hide a cut (a stale prefix of 16 hops could then sit in front of the gap), and the gap frame marks the place. (Treating the junction as a cut
would drop the origin of every propagation of more than 32 hops, which is the part a deep recursion needs; the exception is documented instead.)

This makes the result a function of the call structure, not of frame sizes, and never lets an unverifiable adjacency into a trace. What the
links reject: a sibling call (scenario 3), a hop in a function that did not call the previous hop's function (stored then returned, 9), a callee
without `!` in between (7), a recovered panic (8), an interface or function-value call in between (14), a callee that was inlined into its caller (the inlined hop returns
where the caller returns). What they cannot reject: the earlier hop returned into a function F, and a later `!` in F puts the same object up again:
the same call of F that handled the first error, or another call of F (scenario 11), and also when the `!` passes a runtime wrapper or `Join` of the identity, which inherits the hops (scenario 17). **The price is the origin side**: a cut drops everything below it, so
`return f()` in a plain adapter leaves only the hops above the adapter, and a method reached through an interface leaves only its callers. Open question 4
has the alternative that keeps the dropped hops under a marker.

There is **no clearing event**. The register is never emptied by the program handling the error:

| What the program does | What happens |
|---|---|
| a `!` sees a nil error | nothing runs |
| the error is compared (`err == io.EOF`), logged, printed, or `errors.Is`/`As` look at it | nothing; `Trace(err)` still answers while nothing replaced the register |
| the error is dropped (`_ = f()`) or handled with `if err != nil { ... }` | nothing |
| a function without `!` returns the error (`return err`, `return f()`) | nothing; a `!` above it starts from the register, but its link fails: the trace begins there (scenario 7) |
| `Wrapf`, native `%w` or `Join` replace the error | the new error becomes the identity if the old one was the identity (section 6); `Traced` empties the register (section 7) |
| another error passes a `!` | restart: the old trace is gone, `Trace(old)` is nil from now on |
| a user wrapper (a type the runtime did not make) wraps it | the next `!` sees a different object: restart (scenario 6) |
| the error is stored in a struct and returned later | the late `!` finds the same object, but its link fails: the trace is the late hop; store `errors.Traced(err)` to keep the history (scenario 9) |
| a goroutine starts or ends | the register is empty at the start, reset at the end |
| `recover` | nothing; unwinding is not a hop (scenario 8) |

### 5. Shared values (`io.EOF` and every sentinel)

Many places return one object. Each propagation of a shared value is told from the previous one by the two tests of section 4,
and the answer does not depend on frame addresses:

- a propagation that starts **deeper** than the previous one ended is a certain restart (the frame test): scenario 2, request 2;
- a propagation from a **sibling** or from any function that did not call the previous hop's function is cut by the link check, however the frames
  lie: scenario 3 (`a2` and `b1` called from `main`: the first version's answer depended on their frame sizes), and scenario 14 (an earlier hop in a method
  called through an interface, then an unrelated `!` on the same value: the second version spliced them, because it accepted a link into a thunk);
- a late `!` on a value that was stored and is returned later, and a `!` after a recovered panic, begin with their own hop: scenarios 9 and 8 (the first
  version spliced the hops before and after the store when nothing else had run in between, and not when something had: the trace
  of the same code depended on unrelated work).

What cannot be told apart is **a later `!` in the function that the last hop returned into**. `caller` takes the shared error from `inner` (a hop in `inner`,
which returns into `caller`), handles it, and later puts the same object up again from another callee, with its own `!`: the new hop is in `caller`, so the link holds and
the trace is `inner:36, caller:47`. The same happens when the second `!` is in **another call** of `caller`: `handle(false)` calls `lookupB`, which hops and returns the shared error into `handle`;
`handle(true)` later puts the same object up with its own `!`, and the trace is `lookupB:61, handle:67`, because nothing in the register tells the two calls of `handle` apart.
These are the documented limit (scenario 11): **the trace begins with hops of an earlier propagation**; each hop in it is a real `!` of an object of the identity chain, and each neighbouring pair is
a caller and a callee, but not of one propagation.

**The limit reaches through the runtime's wrappers** (scenario 17). `a` passes the sentinel `ErrPerm` up with a plain `!`, so the identity is `ErrPerm` itself; `f2` handles the error and drops it. When `g`
later returns `fmt.Errorf("g: %w", ErrPerm)` (or `errors.Join(ErrOther, ErrPerm)` from `j`), section 6's rule moves the identity to that wrapper, and the wrapper **inherits the hops of the identity**; `f2`'s `!` is in the
function that `a`'s hop returned into, so the link holds and the trace is `a, f2`, though `g` never called `a`. The same bare sentinel returned from `g2` does the same. What is not affected: an error that does not wrap the identity
(`errors.New`, `fmt.Errorf` without `%w` of it, a constructor) is not the identity and has no hops, so its first `!` starts a trace; and when `a` had used `Wrapf`, the identity is the `Wrapf` wrapper, the sentinel is not the identity,
and `g`'s wrapper of the sentinel changes nothing (scenario 16). No cheap exact rule exists: the register cannot tell that the error was handled and dropped in between.

### 6. Wrapping: the runtime's wrappers take the trace over; a user wrapper restarts it

`Wrapf`, native `fmt.Errorf` with `%w`, and `errors.Join` are gocpp's own code, and each calls
`gp::etrace::rekey(old, wrapper)` before it returns the new error (pointer compares only, no user code; for `Join`, each operand is compared): **if `old` is the register's identity, the identity becomes
`wrapper`; otherwise nothing happens.** The next `!` then finds the object it is looking for, and the trace goes on through the wrapper (scenario 5; `Wrapf` is the idiom of the spec: the `!` inside records its
hop with the inner error, `Wrapf` replaces `err` during the function's exit, the caller's `!` continues while the wrapper is held by the named-result cell of the function). The error that was wrapped no longer
has the trace: `Trace(ErrPerm)` after `boot` wrapped it is empty; **the wrapper is the error that propagated**.

This rule is the simplest one found. It has two named costs, which are a shorter trace, and one named limit, which is the one of section 5 reached through a wrapper (the third item):

- **A wrapper made only to be logged takes the trace.** `defer func() { if err != nil { _ = fmt.Errorf("step failed: %w", err) } }()` makes a wrapper of the error in flight; the identity moves to that
  wrapper, the real error goes on, and the next `!` above does not match it: the trace restarts there (scenario 13: `step` is lost, the trace is `top`). `%w` in a log line and `Join` are clearing events for the
  error they wrap.
- **A wrapper of an error that is not the identity changes nothing.** `fmt.Errorf("audit: %w", ErrPerm)` made after `Wrapf` wraps the sentinel, which is not the identity (the wrapper `W1` is), so the trace of `W1` goes on (scenario 15);
  and a handled `Wrapf` wrapper that is dropped and then met again through a wrapper of its sentinel is not taken for the dropped error (scenario 16: `a` used `Wrapf`; `g` returns `fmt.Errorf("g: %w", ErrPerm)`, or the bare `ErrPerm`, and `f2`'s `!` restarts
  instead of claiming that `g` came from `open` in `a`). This holds only when the handled error was itself a `Wrapf` wrapper (or another runtime wrapper), so that the sentinel is not the identity; see the next item. An earlier draft kept the wrapped error as a second ("previous") identity so that a logging wrapper cost nothing; it either let an unrelated `%w` of a shared sentinel evict the real
  wrapper, or, with a liveness test to prevent that, replaced a dead handled error like a dead throwaway and showed the second repro's wrong trace. Both are gone, with the liveness test.
- **A wrapper or `Join` of the identity inherits its hops** (scenario 17, the limit of section 5). When the handled error was not a `Wrapf` wrapper but the bare sentinel, which is the common case (`open!` returns `ErrPerm`, and a plain `!` passes it up), the sentinel is
  the identity, and a later `fmt.Errorf("g: %w", ErrPerm)`, `errors.Join(..., ErrPerm)` or the bare `ErrPerm` in a function that the last hop returned into continues the old hops: `a, f2`. Each hop is a real `!` of an object of the chain and the neighbours are caller and callee, but the propagation
  is the earlier one. A wrapper of the bare identity cannot be told from one made on the same propagation without a liveness test, which was tried and dropped (Alternatives 16).

A wrapper that the runtime did not make, a user struct with `Unwrap` or a ported type, is not looked into by a hop (that would run user code
in the cold path: Alternatives 1). The next `!` that sees it starts a new trace with it as the identity (scenario 6): the
hops before the wrapper are lost for `Trace` of the wrapper once a `!` has seen it. Before any `!` has seen it, `Trace(wrapper)`
finds the register's identity by walking the tree of the wrapper (`Unwrap() error` and `Unwrap() []error`, pre-order, at most 64 nodes), so reading right after the wrap
works; section 9 says where that walk may run. To keep the hops through a user wrapper, make it with `errors.Wrapf` or `%w`.

### 7. Goroutines and channels: per goroutine, and one explicit way across

The register is per goroutine, so `Trace(err)` in a goroutine other than the one that passed the `!` sites returns
nothing. There is no implicit publication: it would need a hook in every channel send, unlock and exit, and would still
miss errors that are stored under a mutex and read later. Instead, one explicit call:

```go
errc <- errors.Traced(err) // in the worker
```

`errors.Traced(err)` returns a wrapper (a gocpp type with `Error()` = the inner text and `Unwrap`) that holds the trace of `err`
as a **snapshot**, and it **moves** the hops: if `err` is the error in flight, the register is emptied. `Trace(wrapper)` is the
snapshot followed by the hops that a later `!` adds with the wrapper as identity. Nothing is counted twice: a `Traced` of a traced error
holds the whole trace once, and `Trace(Traced(err))` in the goroutine that made it is the snapshot alone (scenario 10). After the move
`Trace(err)` of the inner error is nil in that goroutine: the wrapper has the trace. `Traced(nil)` is nil;
`Traced` of an error that is not in flight wraps with an empty snapshot. It always wraps, with or without tracing in
the build, so the behavior of a program does not depend on the switch.

**`errors.Traced` changes `==`.** `Traced(io.EOF) != io.EOF`, and a `switch err { case io.EOF: }` on the traced error does not match;
`errors.Is(w, io.EOF)`, `errors.As` and type switches through `Unwrap` still do. Apply it to an error that you send over a channel or
store, to a consumer that uses `errors.Is`; do not apply it to a value that the receiver compares with `==`. The spec says so next to the API.

`go F!()` stays a compile error. `go func() { F!() }()` drops the error with the goroutine, and `gocpp vet` already warns about it.

### 8. Other interplay

- **Closures:** a `!` in a closure records a hop named as the source numbers it (`main.main.func1`); the error leaves through the closure's
  results; the caller continues the trace if it has `!` above and its link holds (the closure returned into it).
- **Interface values and function values:** a function that is called through an interface or a function value returns into a thunk
  (`gp_thunk_*`, a `gp::func` invoker), which the link check cannot use: the link after that hop is a cut, so the trace of a method that a caller
  reaches through an interface begins at the caller (scenario 14). The reviewer's real binaries show the thunk at `-O0`, `-O2` and `-O2 -flto` alike.
  This loses the origin side of every path that crosses an interface or a callback, and it is the price of never accepting a link that cannot be
  checked. 10.7b has a gated experiment that could remove the loss for thunks (Alternatives 12); until it passes, the limit stands.
- **`defer`:** deferred calls run during the function's exit, after the hop of the `!` that made the function return. A deferred call that
  makes an unrelated error pass a `!` and handles it replaces the register (restart): the trace of the function's own error is lost. This is the
  price of one slot (Alternatives 6).
- **panic and recover:** the panic form of `!` records the hop and panics with the error value. `recover` returns the same value and
  `Trace` of it **inside the handler** is the full path (scenario 8). If the function that recovered returns the error, the next `!` above
  is in a function that the last hop did not return to: its trace begins there. An unrecovered panic with an error value prints the trace between
  `panic: ...` and the stack (section 9).
- **`main!`, `init!`:** `gp::must_exit` prints the trace after `<program>: <error>` (section 9).
- **`testing`:** `t.Fatal(err)` gets the hop, and nothing else changes in its output. Not promised.
- **Inlining, tail calls and LTO:** the hop is `noinline`, but the function that has the `!` may be inlined into its caller, and a plain
  adapter may be inlined into its caller or end in a tail call. Each of those changes which links hold: an inlined `!` callee returns where its caller
  returns, so the link to the caller's hop is cut; an adapter that is inlined or tail-called makes the link hold (scenario 7: `-O0` `up`, `-O2` `a1, up`).
  Names do not change (they are static); the length of the trace does. `-release` is where this happens (`-O2 -flto`). Nothing forces `noinline` on user functions.
- **Generics, macros, derive:** a `!` in a value macro has the macro function's name at the line of the macro text (`main.Parse`); a `!` in code a wrap
  macro wraps is numbered as in the source (`main.viaWrap.func1`); neither is what a panic trace prints today for those frames (scenario 12). A `!` in
  `#Derive` code has the name and position of the generated code.

### 9. The API and the format

A gocpp addition to package `errors` ([D17](../spec/language.md#d17-a-few-additions-to-the-standard-packages)),
in a file of its own beside `wrapf.gp` (`stdlib/errors/trace.gp`, `own` in `ports.txt`):

```go
type Frame struct {
	Function string // "main.loadConfig"
	File     string // as the panic traces print it (absolute, from the #line lines)
	Line     int
}

// String returns "file:line function".
func (f Frame) String() string

// Trace returns the sites that err passed, origin first, or nil.
func Trace(err error) []Frame

// Traced returns err wrapped with the trace of err, for another goroutine; err != Traced(err).
func Traced(err error) error
```

`Trace` returns a fresh slice, never changes the register, and returns nil for nil, for an error that never passed a `!`, for a typed nil, and for
one that is not the one in flight. A gap (more than 32 hops) is one frame with an empty `File`, `Line` = the number of omitted hops and
`String()` = `... 9 hops omitted`. Reading costs microseconds (the links are checked then): not for a hot loop.

**Where user code may run.** The tree walk of `Trace` and `%+v` calls `Unwrap` methods of the error's tree, which is user code: it runs first, before the
register is read and **never under a runtime lock** (`frames.cpp`'s mutex is taken only after the walk, to resolve names), and a panic in it is recovered
and ends the walk. A hop never runs user code (section 3). The fatal printers (`must_exit`, the panic printer: they run when the program is ending, possibly
with other locks held, and an `Unwrap` that blocks would hang the exit) **run no user code at all**: they print the trace only when the error they
print is the register's identity itself (no tree walk), and they take `frames.cpp`'s mutex with `try_lock`; if they cannot get it the trace is the last hop (every link is then
unverifiable, hence cut).

The `%+v` verb on an error (and nothing else) prints, after the text of `Error()`, one block:

```text
open app.json: no such file or directory
error return trace:
	/work/app.gp:14 main.loadConfig
	/work/app.gp:30 main.main
```

The block is added **only when the error is a top-level operand** of the verb: `fmt.Printf("%+v", err)`, `Sprintf`, `Fprintf`.
A field or element of a struct, slice, map or array that is printed with `%+v` prints the error as `%v` does (the call site in
`pp::print_arg` passes `top = true` to `handle_methods`; the one in `pp::print_value` with `depth > 0` does not), so a logged struct does not grow a
multi-line block in the middle of a line, and `%+v` of a `[]error` is as in Go. `%v`, `%s`, `%q`, `%x`, `Println` and
`Error()` are unchanged. A Go program has no `!`, so no trace: `%+v` of an error is the same as in Go for every program that Go can run.
The same block follows the error message of `main!`/`init!` on stderr and the panic message of an error value, one line per frame, with the same
text (so a log scanner sees one format). The order is origin first, like the way the error went; a panic stack prints the innermost frame first,
and the trace block is not a stack.

### 10. The switch

`-errtrace=on|off` (build, run, test, check, emit). The default is **on in every build except `-release`, where it is off**, as Zig does. With
`off` the lowering emits no hop, so the C++ is byte-identical to what it is today, `errors.Trace` returns nil and `Traced` returns an
empty-snapshot wrapper, and nothing is paid. With `on`: the nil path runs no trace code (Evidence lists the instructions); an error pays about
5 to 6 ns per fresh hop, 3 to 6 ns per hop of a repeated sentinel, and 13 to 15 ns per `Wrapf` level (hop and `rekey`).

The flag changes the C++ and so the objects, **and the up-to-date check**: `cmd/gocpp/flags.go` `extraFlags.keyParts()` (what `uptodate.go`
puts in the stamp key) gets `errtrace=<on|off after the mode's default is applied>`, with a test in `flags_test.go` beside the others. Without it a
`gocpp build` after toggling the flag would say "up to date" and keep the old binary. Measurements decide the default of `-release`, so it is a one-line change if
10.7e finds it free. A program must not print anything but the trace blocks differently with the switch on or off (section 11).

### 11. Gating and migration

`errors.Trace`, `errors.Traced` and the type `errors.Frame` stay gate rows at Phase 10 ([ADR-095](095-must-and-macros-are-always-available.md)) until
10.7d lifts them: `internal/check/gatewalk.go` (the members that the stubs do not have, like `errors.Trace` today), a row each in
`internal/check/gates_test.go`, and `docs/spec/feature-matrix.md`. Until 10.7b is merged, no hop is emitted. Nothing to migrate: the change adds a call
to the error branch, a package-level API and extra lines in three outputs (`%+v` of a top-level error that passed `!`, the message of
`main!`/`init!`, the panic of an error value), all of which exist only for programs that use `!`. The tests that compare stderr (`// Stderr:` is a
per-line regular expression) do not break on extra lines; 10.7b updates the few that compare the whole text.

### 12. What is not promised

- A trace for an error that never passed a `!`: errors made in plain Go code, in the standard library, in C or C++ code. The trace
  starts at the first `!` that sees the error; the line of the `return` that made it in a callee without `!` is not in it (creation sites are Open question 1).
- The part of a path below a cut (section 1, item 3): a function without `!` in the middle, a recovered panic, an interface or function-value call, an inlined
  callee, a user wrapper, a binary without symbols. **The origin side is the part that goes**; the trace is shorter than the path (Open question 4).
- The same trace in every build: the length depends on inlining and tail calls (scenario 7), so `-O0` and `-release` can differ in both directions, and
  `-release` goldens are per build.
- One propagation per goroutine: the latest one only. A trace of an error that was wrapped only to be logged (the wrapper takes the trace, section 6).
- Cuts among the omitted hops of a gap (more than 32 hops): the pair around a gap frame is not checked (section 4).
- Traces across goroutines, except through `errors.Traced`; and `==` on a traced error (section 7).
- Freedom from earlier hops in front in the one shape of section 5: a later `!` in the function that the last hop returned into shows them, and so does a runtime wrapper or `Join` of the identity (`fmt.Errorf("g: %w", ErrPerm)`) that the `!` passes, because the wrapper inherits the hops (scenarios 11 and 17). Only a wrapper of a non-identity (scenario 16) starts its own trace.
- Anything for `Trace` called after another error passed a `!` in the same goroutine, or from a deferred function that handled an unrelated error before
  the trace was read.
- Frame names equal to the panic trace's for code in value macros and wrap macros (section 3), and names of frames of binaries built with other tools.
- Stability of the text of the block between releases (the API of `Trace` is the stable part).
- Cost in `-release`: off by default, and the numbers below are the numbers of one machine.

## Test plan

1. **Golden programs:** the seventeen scenarios of the appendix are `tests/run/etrace_*.gp` with their exact output **in the default (`-O0`) build**. The scenarios whose
   trace depends on inlining and tail calls (3, 7, 11 and 14, and any other where a function is small enough to inline) have **one expected file per build mode**:
   `-O2` and `-release` (`-O2 -flto`) are generated by the real build in 10.7e and reviewed by hand, never copied from `-O0`. A property test runs every scenario
   in all three modes and checks the promises of section 1: every frame is a `!` of the program, neighbours are caller and callee in the source, and no frame comes from
   an object outside the identity chain (the earlier hops of scenarios 11 and 17 are inside it). The appendix and the files in the spike's `expected/` are the output of the **simulation**, valid for programs without unused locals (a C++ scope ends where the block ends, a Go local
   dies at its last use, so a local kept alive in C++ can change which error is live; the simulation cannot show that). **10.7e regenerates every expected file from the real build and reviews the differences by hand**, and the
   goldens hand no in-flight error to another goroutine (channel, logger) between the hop and the read. 10.7e appends the `// Output:` blocks for the API (item 2 checks the same paths before the API exists).
2. **End to end in 10.7b, without the API:** `tests/run/etrace_main_*.gp`: `main!` programs that fail through chains (fresh error, siblings, pass-through,
   an interface method, `Wrapf`, a macro, a deferred `Close!`) and the `// Stderr:` lines are the trace block, plus one program that panics with an error value. They prove the hop
   emission, the register, the link check and the names before the API exists. (A test-only entry `gp::etrace::trace()` that returns the frames is
   what `runtime/test` calls for the unit tests below.)
3. **Register unit tests (C++, `runtime/test`):** start, continue and restart by identity and by frame; no continuation from an equal or deeper frame; the link
   check (direct call, sibling, pass-through, inlined callee made with `[[gnu::always_inline]]`, a return address in an interface thunk and in a `gp::func` invoker, a name that does not
   resolve, **a binary stripped with `strip`: every link is a cut and the trace is the last hop**); `rekey` (for `Wrapf`, `%w` with one and two operands, `Join`: the identity moves
   when the wrapped error is the identity, and nothing changes when it is not; a log-only wrapper takes the trace; an unrelated `%w` of a shared sentinel changes nothing; a handled and dropped `Wrapf` wrapper is not revived by a wrapper of its sentinel, and a handled and dropped bare identity **is** (a wrapper and a `Join` of it inherit its hops, scenario 17)); the identity corners (typed nil, a value type boxed twice, `foreign_owner` and `static_owner` blocks shared by two objects, a pointer and its
   first field); **no user code in a hop** (an error whose `Error`, `Unwrap` and `Is` count and abort); **no user code in the fatal printers** (an `Unwrap` that blocks or panics, with the fatal printer reading an error that
   wraps the identity: it prints no block and does not hang) and the `try_lock` path; the tree walk of `Trace` (`Unwrap() error`, `[]error`, depth, the 64-node limit, a panicking `Unwrap`, no runtime lock
   held while it runs); the ring (first 16, last 16, `total`, the gap frame); an error freed and a new one allocated at the same address must not continue; `Traced` moves and flattens; a pooled worker starts empty; a thread that no `go` started
   releases its register (LeakSanitizer; a raw `pthread_create` thread); `Trace` returns a copy.
4. **Behavior without Trace (differential):** (a) every `tests/run` and `tests/pkgs` program that uses `!` gives the same stdout and exit code with `-errtrace=on` and
   `off`, and the same stderr after the trace blocks are removed; (b) the committed lowering goldens (`tests/golden/must.cpp.golden` and the `tests/mustgen` outputs) are
   byte-identical with `-errtrace=off`, and with `on` they equal the `off` output plus the hop statements (a test deletes the `gp::etrace::hop` lines, the `gp_site`
   lines and the `gp_ret` local, and compares); (c) the plain-Go twins of `tests/musttwin_test.go` pass unchanged with `on`; (d) a new program (`etrace_identity.gp`) compares `err == sentinel`,
   `errors.Is`, `errors.As`, type switches and `errors.Unwrap` on errors that went through `!`, `Wrapf`, `%w` and `Join`, and its output is compared with a hand-written
   twin without `!` run by real Go.
5. **Flags and tools (10.7c):** `flags_test.go` (`-errtrace` is in `keyParts`: toggling it changes the key); a build, toggle, build test that the second build is not "up to date";
   the usage text of `build`, `run`, `test`, `check` and `emit`; `gocpp expand` output has no hop (`docs/guides/expand.md` says so); a debugger test that `step` does not stop in
   `gp::etrace::hop` (the rule is in `runtime/debug/gocpp-gdb.py`, `SKIP_RULES`).
6. **Races:** a test with 16 goroutines that pass one sentinel through `!` chains at once, call `Trace`, and send `Traced` errors over channels, under `-race` and the TSan runtime
   tests (clean), and under `-asan` (no leak at the end of a thread).
7. **Windows** on the real PC (`GOCPP_WINDOWS_SSH`), never Wine: the goldens, a thread that ends with a live register (the `release_adopted` path), the symbolized names (the link check), and
   the benchmark rows once.
8. **Benchmark protocol (10.7e):** new rows `must_chain_nil`, `must_chain_fresh`, `must_chain_shared`, `must_chain_wrapf` in `bench/programs/runtime` (four calls deep, `!` at every level;
   nil error, a fresh error per call, one sentinel, a fresh error through two `Wrapf` levels) with hand-written twins in plain Go, and a row `must_defer_close` for the `gp_ret` local. Per row, g++ and clang++, `-O2` with every function
   `noinline` (the protocol of the spike: an inlined chain compiles to a different loop and says nothing about the hop) and the real `-release` build, `-errtrace=on` and `off`, min and
   median of 7 runs pinned to one CPU and interleaved, plus the dynamic instruction count of the nil path (the `gdb` step counter of the spike). **Gates:** `-errtrace=off` is byte-identical
   C++; with `on` the nil path of the `noinline` chain runs at most 8 more instructions than `off` (the spike: +4 on clang, -45 on g++) and its time is within 15% (the spike: clang +10%, g++ faster; both are
   **code-shape effects** of a cold call in the branch, not costs of the hop); a fresh hop costs at most 8 ns
   and a `Wrapf` level (hop and `rekey`) at most 20 ns on both compilers (the spike: 5.5 and 13 to 15). The `-release` rows are reported: if `on` is within 2% on the nil path there, `-release`
   may default to on (a separate decision, in the report).

## Implementation steps

Five steps instead of three: the first review found 10.7b too large, so the tools work is its own step.

**10.7b: lowering, runtime and an end-to-end check.** `internal/check/mustlower.go` `failStmts` emits `gp_must.Hop(err, file, line, func)` as the first statement of each error
branch (every shape of ADR-037; the position of the `!`; the `defer F!()` form with the saved return address of section 3); `gocpp/must` gets `Hop` and `mp.dropImports` keeps the import; the comment of
`mustpkg.go` and ADR-037 are amended; `internal/lower` writes `gp::etrace::hop(&gp_site, err, __builtin_return_address(0))` with a `static constexpr` site;
`runtime/include/gocpp/etrace.hpp` (included by `rt.hpp`: packaged runtime headers of cpplib trees carry it) and `runtime/src/etrace.cpp` (the register, `hop`, `rekey`, `reset`, the trace builder with the
link check; the object is in every link mode, including shared and cpplib); `frames.cpp` hides `gp::etrace::hop` from panic stacks and exposes the name of an address (and the `shown` flag) without holding its
mutex across user code; `run_goroutine` and `release_adopted` reset the register; `Wrapf`, native `%w` and `errors.Join` call `rekey`; `must_exit` and the fatal-panic printer add the block (no user code, `try_lock`);
the switch is a field of the loader and emit configuration, **on in every build, `-release` included, until 10.7c** (the flag and the release default arrive there; between the two steps a `-release` build pays the cost); the gated experiment of Alternatives 12; focused tests: register unit tests (3), lowering goldens for each shape, the end-to-end `main!` programs (2),
the race test. `errors.Trace` is still gated. Run `go test ./internal/cppgen` (panic output), `go test ./internal/check -run Must`, and `TestMacroList -update` for the new header.

**10.7c: flag, tools and documents of the build** (needs 10.7b; independent of 10.7d). `-errtrace=on|off` in `cmd/gocpp` (the flag on `build`, `run`, `test`, `check`, `emit`; usage text; the default off in `-release`; `keyParts` in `cmd/gocpp/flags.go` and its test); `gocpp expand` does not print
the hop statements (`docs/guides/expand.md`); the gdb script skips `gp::etrace::` (edit `SKIP_RULES` in `runtime/debug/gocpp-gdb.py`, then regenerate `runtime/src/os/gocpp_gdb.inc` with `TestDebugScript -update-debug-script`), and
`docs/guides/debugging.md` says that the lldb script has no skip list yet; the packaging check that a cpplib/staticlib tree carries `etrace.hpp` and links `etrace.cpp`. Tests: item 5 of the test plan.

**10.7d: API, format and documents** (needs 10.7b). `stdlib/errors/trace.gp` (`Frame`, `Trace`, `Traced`, natives over the register), the `%+v` block in `runtime/src/fmt.cpp` (top-level operands only), the gate rows lifted for `errors.Trace`,
`errors.Traced` and `errors.Frame` (`internal/check/gatewalk.go`, `gates_test.go`, `docs/spec/feature-matrix.md`), `docs/spec/language.md` (D17 row, the section on traces, the open point removed), a guide
`docs/guides/error-traces.md`, `stdlib/errors/PORT.md` and `ports.txt` (`own`), the stubs, and `gocpp doc errors.Trace`.

**10.7e: tests and benchmarks** (needs 10.7b to 10.7d for the API programs; the lowering goldens and the benchmark harness need only 10.7b). The seventeen scenario programs as `tests/run/etrace_*.gp` with the API (per-build expected files, the macro one included), the differential tests (4), the
TSan/ASan/race programs, the Windows run, the benchmark rows and the report of numbers for the default of `-release`.

## Alternatives

1. **Walk `Unwrap` in the hop** (the first version of this note: "the same error, or `e` wraps it"). It runs user code in the cold path (a panicking or blocking `Unwrap`, a typed nil receiver, a cost the
   program cannot see) and cost 15 to 20 ns per wrapped hop. Replaced by `rekey` in the runtime's own wrappers, and a restart for foreign ones.
2. **A global side table keyed by the address of the error.** Needs a lock or a lock-free table on every hop, has the address-reuse problem (it needs weak references too), and does not solve shared
   values: one entry per object, so two propagations of one sentinel overwrite each other. It would solve cross-goroutine reads, which is not worth a lock per hop (Decision 7).
3. **A hidden field in errors that gocpp creates.** Errors from user types, the ported standard library and C++ would have none; it changes the layout of `errorString`; a shared sentinel is one object with
   one field.
4. **Wrap the error at each `!`.** Breaks `err == io.EOF`, which is the premise.
5. **Record a stack when the error is created** (`runtime.Callers` in `errors.New`, like pkg/errors). About a microsecond per error, only for errors made through gocpp's own functions, no propagation
   information, and `errors.New` of a sentinel at package initialization records the wrong place. Left as Open question 1, as an addition that would sit in front of the hops.
6. **A ring of several traces per goroutine** (so `Trace(old)` still works after another error). Needs an identity comparison per slot per hop and a policy for eviction. One slot covers the cases that matter
   (log or return the error you just got); a ring is an extension that would not change the API.
7. **The frame address alone as the continuation test** (the first version: continue when the new frame is older, or equal with another site). Right for chains, but for siblings the answer
   depends on frame sizes (the reviewer's `a2`/`b1` program changes with the optimization level), and it splices hops across a store. The frame test stays only as a certain-restart test.
8. **The frame address of the caller (`__builtin_dwarf_cfa`) instead of the link check.** Deterministic for siblings (equal addresses) and it keeps traces across functions without `!`; but it forces a frame pointer
   in every function that has a `!` (extra instructions on the nil path where the compiler would have omitted it), and it accepts a hop from any older frame, so it keeps the stale prefixes that the link check cuts (a hop in
   an unrelated, shallower function after a deep one). The link check needs only the return address, which is free. Costs the traces across plain functions (Open question 4).
9. **Passing the error by `const&` to `hop`.** The local escapes: g++ runs 224 instead of 179 instructions on the nil path. By value costs nothing there (134 instructions, the copy is in the cold call).
10. **Publish a trace at every channel send, unlock and goroutine end.** A check in every blocking primitive, partial coverage, and a global table; `errors.Traced` says the same in one visible call.
11. **No cost on the nil path, whatever the compiler does.** Not possible with a call in the branch; measured below: a code-shape effect of at most +4 instructions with the error passed by value.
16. **A second ("previous") identity so that a logging wrapper costs nothing, with or without a liveness test.** Tried twice and dropped: without a test an unrelated `%w` of a shared sentinel evicts the real wrapper
    (scenario 15); with the test "replace only when the current identity is dead" a handled and dropped error is replaced like a dead throwaway, and a later wrapper (or the bare sentinel) of the same sentinel continues
    a trace that did not come from there (scenario 16). Liveness is also not stable: it is the refcount of the owner, which a hand-off to another goroutine or a logger changes, and C++ scope ends differ from Go's. The
    cost is the logging wrapper of scenario 13.
12. **Accept a link that cannot be checked** (the second version: "unverifiable links are accepted"). Not sound: a hop made through an interface thunk was accepted into any later hop of the same object at a
    shallower frame (the reviewer's `lk2` at `-O0`, `-O2`, `-O2 -flto`: `S::A > h2` though `h2` never called `A`; `io.EOF` through `io.Reader` hits it), and in a binary without symbols every link was accepted, which
    brings back the sibling splice and the frame-size dependence. Replaced by the rule that an unverifiable link is a cut.
13. **Resolve the thunk at hop time.** Tag the thunks and invokers (`[[gnu::section]]`, a `__start_`/`__stop_` range test: no symbols, no lock) and, only when the return address is in that range, unwind one frame
    (`_Unwind_Backtrace`, about a microsecond, only for hops made through interfaces and callbacks) to get the real caller's return address. It would link interface and callback hops again.
    Not decided: the section attribute may not be allowed on the template invokers, and the unwind on the error path needs a measurement. 10.7b runs it as a gated experiment (the gate: it links the `lk2` interface case at
    `-O0`, `-O2` and `-O2 -flto`, takes none of the runtime's own locks (not `frames.cpp`'s mutex), and costs at most 2 microseconds per hop through a thunk, with whatever lock the platform unwinder takes counted in that cost:
    older glibc and Windows take a loader lock in the unwinder, so the experiment runs per platform and the unwind is used only where it passes); if it fails, the cut stays and Open question 4 decides whether a marker is worth it.
14. **Record the entry address of the function in the site, to link without symbols.** An entry address says where a function starts, not where it ends: whether a return address is inside it cannot be decided (a plain
    function placed right after it would be taken for it), so the link check would be unsound again. A start and an end address per function would need an assembler label after every function and a name for every lambda; not worth it for stripped binaries, where the
    trace is the last hop.
15. **Keep the hops below a cut under a marker.** A frame such as `... earlier hops, not linked` between the part above the cut and the part below it. It restores the origin side, but a stale or sibling hop of the
    same object that the frame test lets through (siblings can pass it, depending on frame sizes) would appear below the marker in one build and not in another. Offered as Open question 4, restricted to
    the links that are unverifiable (thunks, stripped binaries) and not to those that are mismatched.

## Consequences

Good:

- `err == io.EOF`, `errors.Is`, `errors.As` and type switches see the same values with and without traces: nothing is wrapped (`Traced` is the one explicit, visible exception).
- Programs that never call `Trace` or print `%+v` differ only in stderr lines of failures that already exit.
- The cost is where the programmer can see it: a call in the error branch, a thread-local register, a switch.
- A hop runs no user code and no lock; a trace is a function of the call structure, not of the frame sizes of one compiler, and it never contains a neighbour pair that was not checked.
- The same trace format for logs, `main!`, panics and API users, and `-release` is free by default.

Bad:

- One propagation per goroutine; a handler that runs another failing `!` first loses the trace.
- A function without `!` in the middle, a recovered panic, an interface or function-value call, an inlined callee, a user wrapper and a binary without symbols cut the trace there: it is shorter than the path, and the part
  it loses is the origin side. `-release` inlines more and joins and cuts other links, so its traces differ from `-O0`'s in both directions.
- An error that the program wraps only to log it (`%w` in a log line, `Join`) takes the trace; the real error restarts at the next `!` (scenario 13).
- A shared sentinel can show earlier hops in front in one shape (a later `!` in the function that the last hop returned into, the same call or another), and so can a wrapper or `Join` of it made by the runtime, which inherits the hops (scenario 17); no cheap exact rule exists.
- The trace starts at the first `!`, not at the `return` that made the error.
- Each thread's register keeps one dead error object's control block (and storage) until the next restart or the end of the goroutine.
- Reading a trace resolves addresses to names (microseconds), and a binary without symbols has a trace of the last hop only.
- With the switch on and no inlining, the compiled code of the functions with `!` changes shape: +4 instructions on the nil path of a chain on clang, a different inlining decision (faster) on g++; the functions that have `defer F!()` store one more local.

## Open questions (for the manager and the user)

1. **Creation sites.** `errors.New`, `fmt.Errorf` and `Wrapf` could store their caller's site for the first hop of the trace of a fresh error. About 10 ns per error, and wrong for package-level
   sentinels. Recommended: not in 10.7b to 10.7e; decide after the first use.
2. **The default of `-release`.** Off now. 10.7e reports numbers.
3. **Panic and `main!` print the trace by default** when the build has the switch on. If the user prefers a quiet default, the alternative is an environment variable `GOCPP_ERRTRACE=1` read by those two
   printers (the `%+v` and API behavior would not change).
4. **What a cut does to a trace.** The link check cuts the trace at a plain function that returns the error (`return f()`), at a recovered panic, at a hop made through an interface or a function value, at an
   inlined callee, at a user wrapper, and everywhere in a binary without symbols, because those links cannot be told from an unrelated hop of the same object. A cut drops **the origin side**: for `return f()` in
   a plain adapter only the hops above the adapter are left, which is the least useful part of the path, and for an interface method only its callers. Two alternatives: (a) keep the hops below a cut
   under a marker frame (`... earlier hops, not linked to the next`), restricted to the links that are unverifiable (a thunk, no symbols) and never for a mismatch; the origin comes back, and a hop of an earlier propagation can
   too, below the marker and labelled; (b) the first version's rule (accept everything the frame test lets through), which gives longer traces that depend on frame sizes and can splice siblings. Recommended: the
   cut now (what is shown is checked), with 10.7b's thunk experiment (Alternatives 13) first; choose (a) if the experiment fails and interface-heavy code loses too much. A vet check that suggests `!` for `return f()` where `f` is a
   must function is a later option.

## Evidence

All numbers: g++ 15.2.0 and clang++ 21.1.8 on the 192-core PC (Linux 6.19), minimum of 7 runs pinned to one CPU, the machine shared (load 3 to 8), so read ratios and differences of more than a few
percent. Source: the spike, `proto/real`: gocpp's own generated C++ of a four-call chain (`app` → `top` → `mid` → `leaf`, `!` at three levels; `errors.New`, one sentinel, or two levels of `errors.Wrapf`) with the hop
inserted by `insert_hops.py` and the prototype `hop_rt.cpp`: the register, the weak identity, the frame test, the ring, the two stored addresses, `rekey`; no symbolization (the benchmark never reads a trace). Every function
of the chain is `noinline` in both variants, `-O2`, the `hop` takes the error by value.

| ns per call of the chain (`noinline`, `-O2`) | g++ without | g++ with hops | clang++ without | clang++ with hops |
|---|---|---|---|---|
| nil error (no hop runs) | 8.58 | 6.02 | 6.23 | 6.83 |
| fresh error each call, 3 hops | 41.8 | 58.3 (+16.5, 5.5 per hop) | 43.8 | 60.5 (+16.7, 5.6 per hop) |
| one sentinel, 3 hops | 34.2 | 42.3 (+8.1, 2.7 per hop) | 29.1 | 46.5 (+17.4, 5.8 per hop) |
| fresh error, 3 hops and 2 `Wrapf` levels (hop, `rekey`) | 127.2 | 158.3 (+31.0) | 132.0 | 157.6 (+25.6) |

| Dynamic instructions of the nil path (gdb, one chain) | g++ | clang++ |
|---|---|---|
| without hops | 179 | 129 |
| with hops, error by value | 134 | 133 |
| with hops, error by `const&` | 224 | not measured |

How to read it. The nil path changes shape and is not "free" or "noise": g++ compiles the variant with hops differently (the reviewer counted 45 thread-local accesses in the base and 36 with hops: the base
inlines more of the error-release sequence, and the cold call in the branch changes that decision), so the variant with hops is faster and shorter on this machine (8.6 against 6.0 ns here, 11.6 against 8.4 ns in the review); clang++ adds 4 instructions per chain and 0.6 ns (+10%). Both are **code-shape effects** of
having a cold `noinline` call in the error branch (which values stay in callee-saved registers, what gets inlined), not costs of the hop, and not noise. They are the reason the gates are on instructions and time of
the nil path, not on equality. The error path pays about 5 to 6 ns per fresh hop (the weak reference taken when the error enters the register is the largest part) and 13 to 15 ns per `Wrapf` level (the hop and `rekey`, two weak assignments);
the one-sentinel row is the clang++ outlier (+17 where g++ has +8) and is reported as measured. An earlier row of the first version of this note (everything inlined, "5.3 ns to 1.2 ns") is dropped: the loop compiled differently, which says nothing about the hop. The
first version's "at most 6 ns per hop" gate was below these numbers; the gate of the test plan (8 ns, 20 ns) is above them with room for another machine.

Real binaries (the second review, `lk2`, gocpp's generated C++ with `hop` printing the two addresses; the check applies the link rule with names from `nm`): at `-O0`, `-O2` and `-O2 -flto` the hop of a method called through an
interface returns into `gp_thunk_*`, so the second version's rule "unverifiable is accepted" spliced `S::A > h2`. With the rule "unverifiable is a cut" scenario 14 gives `h2` alone. The same programs show the build dependence of the length of a trace: the plain adapter
of scenario 7 gives `up` at `-O0` and `a1 > up` at `-O2` (the call in the adapter is a tail call) and the hop names under LTO are the host functions' (the reason the site holds its own static name); the siblings give `b1`
at `-O0` and `-O2`; the same function twice gives `lookupB > handle` at `-O0` and `-O2` (scenario 11).

Scenarios: `spikes/2026-10-21/error-traces/run.sh` converts each program of `scenarios/` to plain Go (the error branch of a `!` calls the simulation's `Hop`; the lines stay the same), runs it with `go run -gcflags=-l` and compares the output with
`expected/`. The macro program is not simulated: its names were read from the panic traces of real gocpp programs with the same lines (the value macro `Parse`: `main.Parse(...)` at the line of the macro text; the closure in the wrap macro:
`main.viaWrap.func1.1.1`; the deferred call: `main.use.func1`). All programs also type-check and run as gocpp programs with `errors.Trace` and `errors.Traced` replaced by stubs (the rest of the output is identical to the simulation's: that run is how the
programs were checked to be valid gocpp, e.g. `defer c.Close!()` in a function with a declared `err`, a `!` in a closure, in a plain function and in macros).

## See

- [Phase 10 README](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md), "Error return traces" and step 10.7.
- [docs/spec/language.md](../spec/language.md#error-return-traces-designed-not-built) and D17.
- [ADR-013](013-must-errors.md), [ADR-037](037-must-rewrite.md), [ADR-085](085-panic-stacks-up-to-the-handler-and-no-throw-for-a-local-panic.md).

## Appendix: the scenarios

The complete programs are `spikes/2026-10-21/error-traces/scenarios/*.gp`, each with a helper `show` (`file:line function` per frame, base names) that is left out below. `run.sh` there runs every program but the macro one through a simulation of the register and compares the output with `expected/`. Line numbers are those of the files. They become `tests/run/etrace_*.gp` in 10.7d, with the real runtime in place of the simulation. The outputs are those of the simulation, valid for programs without unused locals (a Go local dies at its last use, a C++ local at the end of its block); 10.7e regenerates them from the real build.

### 1. A fresh error through two must functions (`etrace_chain.gp`)

The hops are the lines of the two `!` that handled the error. The call in `main` is plain Go, so it records nothing, and the second call has no error and no trace.

```text
26  func write(n int) (int, error) {
27  	if n > 2 {
28  		return 0, errors.New("disk full")
29  	}
30  	return n, nil
31  }
32
33  func save!(n int) int {
34  	x := write!(n)
35  	return x * 2
36  }
37
38  func flush!(n int) int {
39  	y := save!(n)
40  	return y + 1
41  }
42
43  func main() {
44  	_, err := flush(5)
45  	fmt.Println(err)
46  	show("a fresh error through two must functions", err)
47  	v, err := flush(1)
48  	fmt.Println(v, err)
49  	show("no error, no trace", err)
50  }
```

Output:

```text
disk full
-- a fresh error through two must functions
  etrace_chain.gp:34 main.save
  etrace_chain.gp:39 main.flush
3 <nil>
-- no error, no trace
  (no trace)
```

### 2. One value, two propagations (`etrace_shared_value.gp`)

`ErrNotFound` is returned by `lookup` for both requests. Request 1 goes `serve`, `fetch`, `lookup`; request 2 goes `outer`, `direct`, `lookup`. The first `!` of request 2 is in `direct`, which is deeper than where request 1 ended (`serve`): the frame test restarts the trace. `err == ErrNotFound` is true both times: the register never touches the value.

```text
26  var ErrNotFound = errors.New("not found")
27
28  func lookup(key string) (string, error) {
29  	if key == "" {
30  		return "", ErrNotFound
31  	}
32  	return "v:" + key, nil
33  }
34
35  func fetch!(key string) string {
36  	v := lookup!(key)
37  	return v
38  }
39
40  func serve!(key string) string {
41  	r := fetch!(key)
42  	return r
43  }
44
45  func direct!(key string) string {
46  	v := lookup!(key)
47  	return v
48  }
49
50  func outer!(key string) string {
51  	r := direct!(key)
52  	return r
53  }
54
55  func main() {
56  	_, err := serve("")
57  	fmt.Println(err == ErrNotFound)
58  	show("request 1: serve, fetch, lookup", err)
59
60  	_, err = outer("")
61  	fmt.Println(err == ErrNotFound)
62  	show("request 2: the same value again, a different path", err)
63  }
```

Output:

```text
true
-- request 1: serve, fetch, lookup
  etrace_shared_value.gp:36 main.fetch
  etrace_shared_value.gp:41 main.serve
true
-- request 2: the same value again, a different path
  etrace_shared_value.gp:46 main.direct
  etrace_shared_value.gp:51 main.outer
```

### 3. Sibling paths of one value (the layout does not matter) (`etrace_siblings.gp`)

`a2` and `b1` are called from `main` and fail with the same `ErrX`. Whether the frame of `b1`'s hop is older than the frame of `a2`'s hop depends on the frame sizes of the two functions, which change with the optimization level. The test that decides is the link between the two hops: `a2`'s hop returns into `main`, `b1`'s hop is in `b1`, so the trace of path 2 is `b1` alone, whatever the frames look like.

```text
26  var ErrX = errors.New("x")
27
28  func leaf(n int) (int, error) {
29  	return 0, ErrX
30  }
31
32  func a1!(n int) int {
33  	v := leaf!(n)
34  	return v
35  }
36
37  func a2!(n int) int {
38  	v := a1!(n)
39  	return v
40  }
41
42  func b1!(n int) int {
43  	v := leaf!(n)
44  	return v
45  }
46
47  func main() {
48  	_, err := a2(1)
49  	show("path 1: a2, a1, leaf", err)
50  	_, err = b1(1)
51  	show("path 2: b1, leaf, a sibling of a2 at the same depth", err)
52  	_, err = a2(1)
53  	show("path 1 again", err)
54  }
```

Output:

```text
-- path 1: a2, a1, leaf
  etrace_siblings.gp:33 main.a1
  etrace_siblings.gp:38 main.a2
-- path 2: b1, leaf, a sibling of a2 at the same depth
  etrace_siblings.gp:43 main.b1
-- path 1 again
  etrace_siblings.gp:33 main.a1
  etrace_siblings.gp:38 main.a2
```

### 4. Handled, dropped and replaced (`etrace_replaced.gp`)

Two fresh errors; the second is made while the program still holds the first. The last printed line says they are two different objects with the same text.

```text
26  func parse(s string) (int, error) {
27  	if s == "" {
28  		return 0, errors.New("empty input")
29  	}
30  	return len(s), nil
31  }
32
33  func step!(s string) int {
34  	n := parse!(s)
35  	return n
36  }
37
38  func main() {
39  	_, err1 := step("")
40  	show("err1 just after the call", err1)
41
42  	_, err2 := step("")
43  	show("err2 just after its call", err2)
44
45  	show("err1 again: err2 is in flight now", err1)
46  	fmt.Println(err1 != err2, err1.Error() == err2.Error())
47  }
```

Output:

```text
-- err1 just after the call
  etrace_replaced.gp:34 main.step
-- err2 just after its call
  etrace_replaced.gp:34 main.step
-- err1 again: err2 is in flight now
  (no trace)
true true
```

### 5. Wrapping: `Wrapf`, `%w`, `Join` (`etrace_wrapped.gp`)

`load` has `defer errors.Wrapf(&err, "load %s", path)` and calls `open!`, which returns the sentinel `ErrPerm`; `boot` calls `load!`. `Wrapf` replaces the error during `load`'s exit and takes over the identity of the register, so `boot`'s `!` continues the trace through the wrapper (the wrapper is held by the named-result cell of `load`). `errors.Is(err, ErrPerm)` is true and `err == ErrPerm` is false, which is Go's rule for wrapped errors, unchanged. `fmt.Errorf("%w")` and `errors.Join` do the same: the error that is returned has the trace, and the one it wraps does not.

```text
26  var ErrPerm = errors.New("permission denied")
27
28  func open(path string) (int, error) {
29  	return 0, ErrPerm
30  }
31
32  func load!(path string) (n int, err error) {
33  	defer errors.Wrapf(&err, "load %s", path)
34  	n = open!(path)
35  	return n, nil
36  }
37
38  func boot!(path string) int {
39  	n := load!(path)
40  	return n
41  }
42
43  func main() {
44  	_, err := boot("/etc/shadow")
45  	fmt.Println(err)
46  	fmt.Println(errors.Is(err, ErrPerm), err == ErrPerm)
47  	show("through Wrapf: the wrapper carries on the trace", err)
48  	show("the sentinel alone is not the error in flight", ErrPerm)
49  	wrapped := fmt.Errorf("start: %w", err)
50  	show("fmt.Errorf with %w: the new wrapper has the trace", wrapped)
51  	show("the error it wrapped does not", err)
52  	joined := errors.Join(errors.New("other"), wrapped)
53  	show("errors.Join does the same", joined)
54  }
```

Output:

```text
load /etc/shadow: permission denied
true false
-- through Wrapf: the wrapper carries on the trace
  etrace_wrapped.gp:34 main.load
  etrace_wrapped.gp:39 main.boot
-- the sentinel alone is not the error in flight
  (no trace)
-- fmt.Errorf with %w: the new wrapper has the trace
  etrace_wrapped.gp:34 main.load
  etrace_wrapped.gp:39 main.boot
-- the error it wrapped does not
  (no trace)
-- errors.Join does the same
  etrace_wrapped.gp:34 main.load
  etrace_wrapped.gp:39 main.boot
```

### 6. A wrapper the runtime did not make; no user code in a hop; a typed nil (`etrace_foreign.gp`)

`decorate` wraps the error of `step` in a user type that has `Unwrap`. The runtime does not own that type, so the next `!` (in `run`) sees an error it cannot match, and the trace restarts there: `step`'s hop is gone from it. The counter shows that no hop called `Unwrap`. `errors.As` still works. The last part passes a typed nil pointer through `!`: it is a non-nil `error` without an identity, nothing is claimed for it, and its `Error` method is never called.

```text
26  type opError struct {
27  	op  string
28  	err error
29  }
30
31  var unwrapCalls int
32
33  func (e *opError) Error() string {
34  	return e.op + ": " + e.err.Error()
35  }
36
37  func (e *opError) Unwrap() error {
38  	unwrapCalls++
39  	return e.err
40  }
41
42  type nilError struct{}
43
44  func (e *nilError) Error() string {
45  	return "never called"
46  }
47
48  func parse(s string) (int, error) {
49  	return 0, errors.New("empty input")
50  }
51
52  func step!(s string) int {
53  	n := parse!(s)
54  	return n
55  }
56
57  func decorate(s string) (int, error) {
58  	n, err := step(s)
59  	if err != nil {
60  		return 0, &opError{"decorate", err}
61  	}
62  	return n, nil
63  }
64
65  func run!(s string) int {
66  	n := decorate!(s)
67  	return n
68  }
69
70  func typedNil() (int, error) {
71  	var p *nilError
72  	return 0, p
73  }
74
75  func viaNil!() int {
76  	v := typedNil!()
77  	return v
78  }
79
80  func main() {
81  	_, err := run("")
82  	fmt.Println("Unwrap calls made by the hops:", unwrapCalls)
83  	show("a wrapper that gocpp did not make: the trace restarts at the first ! that sees it", err)
84  	var op *opError
85  	fmt.Println(errors.As(err, &op), op.op)
86
87  	_, err = viaNil()
88  	fmt.Println(err != nil)
89  	show("a typed nil pointer has no identity: nothing is claimed for it", err)
90  }
```

Output:

```text
Unwrap calls made by the hops: 0
-- a wrapper that gocpp did not make: the trace restarts at the first ! that sees it
  etrace_foreign.gp:66 main.run
true decorate
true
-- a typed nil pointer has no identity: nothing is claimed for it
  (no trace)
```

### 7. A function without `!` in the middle (`etrace_passthrough.gp`)

`adapter` returns the error of `lower` without a `!`. The hop of `upper` is not made in the function that `lower`'s hop returned into (`adapter`), so the link is cut and the trace begins at `upper`: the origin side (`lower`) is gone. With `!` in `adapterBang` the chain is complete. This is the `-O0` output; at `-O2` the call in `adapter` is a tail call, `lower` returns into `upper`, and the trace is `lower, upper`.

```text
26  var ErrDB = errors.New("db down")
27
28  func query(n int) (int, error) {
29  	return 0, ErrDB
30  }
31
32  func lower!(n int) int {
33  	v := query!(n)
34  	return v
35  }
36
37  func adapter(n int) (int, error) {
38  	return lower(n)
39  }
40
41  func upper!(n int) int {
42  	v := adapter!(n)
43  	return v
44  }
45
46  func adapterBang!(n int) int {
47  	v := lower!(n)
48  	return v
49  }
50
51  func upper2!(n int) int {
52  	v := adapterBang!(n)
53  	return v
54  }
55
56  func main() {
57  	_, err := upper(1)
58  	show("a function without ! in the middle: the trace begins above it", err)
59  	_, err = upper2(1)
60  	show("the same chain with ! at every level", err)
61  }
```

Output:

```text
-- a function without ! in the middle: the trace begins above it
  etrace_passthrough.gp:42 main.upper
-- the same chain with ! at every level
  etrace_passthrough.gp:33 main.lower
  etrace_passthrough.gp:47 main.adapterBang
  etrace_passthrough.gp:52 main.upper2
```

### 8. Deferred `Close!`, closures, panic and recover (`etrace_closure_defer_recover.gp`)

`use`: the deferred `c.Close!()` becomes the function's error and records its hop in the deferred closure; the site is named after `use` and the closure passes the return address that `use` saved when it started, so `useUp`'s hop links to it. `parseAll` is a closure: its `!` leaves through the closure's result. `mustStep` has no error result, so its `!` records the hop and panics; `safe` recovers. Inside the handler the trace is complete. After `safe` has returned the error, `outer`'s `!` is not in the function that `mustStep`'s hop returned into (`safe`), so the trace begins at `outer`: read the trace in the handler if the whole path matters.

```text
26  type conn struct {
27  	name string
28  }
29
30  func (c *conn) Close() error {
31  	return errors.New("close " + c.name)
32  }
33
34  func use!(name string) (err error) {
35  	c := &conn{name}
36  	defer c.Close!()
37  	return nil
38  }
39
40  func parse(s string) (int, error) {
41  	if s == "" {
42  		return 0, errors.New("empty input")
43  	}
44  	return len(s), nil
45  }
46
47  func step!(s string) int {
48  	n := parse!(s)
49  	return n
50  }
51
52  func mustStep(s string) int {
53  	n := step!(s)
54  	return n
55  }
56
57  func safe(s string) (n int, err error) {
58  	defer func() {
59  		if r := recover(); r != nil {
60  			err = r.(error)
61  			show("inside the handler", err)
62  		}
63  	}()
64  	return mustStep(s), nil
65  }
66
67  func outer!(s string) int {
68  	n := safe!(s)
69  	return n
70  }
71
72  func useUp!(name string) {
73  	use!(name)
74  }
75
76  func main() {
77  	show("a deferred Close! that becomes the error", use("db"))
78  	show("the deferred hop is linked to the hop of the caller", useUp("db"))
79
80  	parseAll := func(s string) (int, error) {
81  		n := step!(s)
82  		return n, nil
83  	}
84  	_, err := parseAll("")
85  	show("a ! inside a closure leaves through the closure's result", err)
86
87  	_, err = outer("")
88  	show("after the recover: the next ! starts from the frame it is in", err)
89  }
```

Output:

```text
-- a deferred Close! that becomes the error
  etrace_closure_defer_recover.gp:36 main.use
-- the deferred hop is linked to the hop of the caller
  etrace_closure_defer_recover.gp:36 main.use
  etrace_closure_defer_recover.gp:73 main.useUp
-- a ! inside a closure leaves through the closure's result
  etrace_closure_defer_recover.gp:48 main.step
  etrace_closure_defer_recover.gp:81 main.main.func1
-- inside the handler
  etrace_closure_defer_recover.gp:48 main.step
  etrace_closure_defer_recover.gp:53 main.mustStep
-- after the recover: the next ! starts from the frame it is in
  etrace_closure_defer_recover.gp:68 main.outer
```

### 9. Stored in a struct, returned later (`etrace_stored.gp`)

`task.run` stores the error; `finish` returns it without a `!`; `report!` passes it up. The hop in `step` is not linked to `report`'s, so the trace of the late return is `report` alone, with or without other errors in between. `runTraced` stores `errors.Traced(err)` instead: the snapshot keeps the history.

```text
26  type task struct {
27  	err error
28  }
29
30  func parse(s string) (int, error) {
31  	if s == "" {
32  		return 0, errors.New("empty input")
33  	}
34  	return len(s), nil
35  }
36
37  func step!(s string) int {
38  	n := parse!(s)
39  	return n
40  }
41
42  func (t *task) run() {
43  	_, err := step("")
44  	t.err = err
45  }
46
47  func (t *task) runTraced() {
48  	_, err := step("")
49  	t.err = errors.Traced(err)
50  }
51
52  func finish(t *task) error {
53  	return t.err
54  }
55
56  func report!(t *task) {
57  	finish!(t)
58  }
59
60  func main() {
61  	a := &task{}
62  	a.run()
63  	e := report(a)
64  	show("stored, then returned later: the trace begins at the late hop", e)
65
66  	b := &task{}
67  	b.runTraced()
68  	step("")
69  	e = report(b)
70  	show("stored with errors.Traced: the history comes with it", e)
71  }
```

Output:

```text
-- stored, then returned later: the trace begins at the late hop
  etrace_stored.gp:57 main.report
-- stored with errors.Traced: the history comes with it
  etrace_stored.gp:38 main.step
  etrace_stored.gp:57 main.report
```

### 10. Goroutines and channels (`etrace_goroutines.gp`)

Two workers run `job("")`, which fails through `step` and `job`. One sends the error as it is, the other sends `errors.Traced(err)`. Then `report!` in the receiving goroutine passes the traced error up. `Traced` of a traced error and `Traced` read in the goroutine that made it show each hop once: the wrapper takes the hops, the register is empty afterwards.

```text
26  func parse(s string) (int, error) {
27  	if s == "" {
28  		return 0, errors.New("empty input")
29  	}
30  	return len(s), nil
31  }
32
33  func step!(s string) int {
34  	n := parse!(s)
35  	return n
36  }
37
38  func job!(s string) int {
39  	n := step!(s)
40  	return n
41  }
42
43  func check(err error) error {
44  	return err
45  }
46
47  func report!(err error) {
48  	check!(err)
49  }
50
51  func main() {
52  	plainCh := make(chan error)
53  	tracedCh := make(chan error)
54  	go func() {
55  		_, err := job("")
56  		plainCh <- err
57  	}()
58  	go func() {
59  		_, err := job("")
60  		tracedCh <- errors.Traced(err)
61  	}()
62  	e1 := <-plainCh
63  	e2 := <-tracedCh
64  	show("received over a channel as it is", e1)
65  	show("received as errors.Traced(err)", e2)
66  	report(e2)
67  	show("after a ! in the receiving goroutine", e2)
68  	fmt.Println(errors.Unwrap(e2) != nil, e2.Error())
69
70  	e3 := errors.Traced(e2)
71  	show("Traced of a traced error: each hop once", e3)
72
73  	_, err := job("")
74  	w := errors.Traced(err)
75  	show("Traced(err) read in the goroutine that made it: each hop once", w)
76  	show("err itself: the trace moved to the wrapper", err)
77  }
```

Output:

```text
-- received over a channel as it is
  (no trace)
-- received as errors.Traced(err)
  etrace_goroutines.gp:34 main.step
  etrace_goroutines.gp:39 main.job
-- after a ! in the receiving goroutine
  etrace_goroutines.gp:34 main.step
  etrace_goroutines.gp:39 main.job
  etrace_goroutines.gp:48 main.report
true empty input
-- Traced of a traced error: each hop once
  etrace_goroutines.gp:34 main.step
  etrace_goroutines.gp:39 main.job
  etrace_goroutines.gp:48 main.report
-- Traced(err) read in the goroutine that made it: each hop once
  etrace_goroutines.gp:34 main.step
  etrace_goroutines.gp:39 main.job
-- err itself: the trace moved to the wrapper
  (no trace)
```

### 11. Limits: earlier hops in front, and the ring (`etrace_limits.gp`)

`caller` takes the error of `inner` (a hop is recorded and the error is dropped), then passes the same sentinel up again through `plainPass!`, a callee that records no hop. `inner`'s hop returned into `caller` and the new hop is in `caller`, so the link holds and `inner`'s hop stays in the trace. The same happens across calls: `handle(-1, false)` calls `lookupB`, which hops and returns the sentinel into `handle`; `handle(-1, true)` puts the sentinel up with its own `!`, and the register cannot tell the two calls of `handle` apart. This is the one shape in which a trace has hops of an earlier propagation. Then a recursion of depth 40 fails: 41 hops, of which the first 16, a gap and the last 16 are kept (33 frames, the gap at index 16).

```text
26  var ErrBusy = errors.New("busy")
27
28  func try(n int) (int, error) {
29  	if n < 0 {
30  		return 0, ErrBusy
31  	}
32  	return n, nil
33  }
34
35  func inner!(n int) int {
36  	v := try!(n)
37  	return v
38  }
39
40  func plainPass(n int) (int, error) {
41  	return try(n)
42  }
43
44  func caller!(n int) int {
45  	_, err := inner(n)
46  	_ = err
47  	v := plainPass!(n)
48  	return v
49  }
50
51  func rec!(n int) int {
52  	if n == 0 {
53  		v := try!(-1)
54  		return v
55  	}
56  	v := rec!(n - 1)
57  	return v
58  }
59
60  func lookupB!(n int) int {
61  	v := try!(n)
62  	return v
63  }
64
65  func handle!(n int, again bool) int {
66  	if again {
67  		v := try!(n)
68  		return v
69  	}
70  	_, err := lookupB(n)
71  	_ = err
72  	return 0
73  }
74
75  func main() {
76  	_, err := caller(-1)
77  	show("LIMIT: the handled propagation of a shared value is still there", err)
78
79  	handle(-1, false)
80  	_, err = handle(-1, true)
81  	show("LIMIT: the same function again, in another call", err)
82
83  	_, err = rec(40)
84  	frames := errors.Trace(err)
85  	fmt.Println(len(frames), frames[0].Function, frames[len(frames)-1].Function)
86  	for i, f := range frames {
87  		if f.File == "" {
88  			fmt.Printf("frame %d: ... %d hops omitted\n", i, f.Line)
89  		}
90  	}
91  }
```

Output:

```text
-- LIMIT: the handled propagation of a shared value is still there
  etrace_limits.gp:36 main.inner
  etrace_limits.gp:47 main.caller
-- LIMIT: the same function again, in another call
  etrace_limits.gp:61 main.lookupB
  etrace_limits.gp:67 main.handle
33 main.rec main.rec
frame 16: ... 9 hops omitted
```

### 12. Names in macros (not simulated) (`etrace_macros.gp`)

A `!` in the body of a value macro (`Parse`) is in the macro function, at the line of the macro text, and the site is named `main.Parse`; it has no error result, so it panics, and the handler reads the trace. A `!` in a closure in a function with a wrap macro (`Around`) is named as the source numbers it, `main.viaWrap.func1`. Both names are static and agree with the file and line next to them. A panic trace of the same code prints `main.Parse` at the same line, but `main.viaWrap.func1.1.1` for the closure (the wrap macro's body runs in two lowered closures), and `#func()` in a value macro says the function of the use; the hop uses neither. Checked with real gocpp programs: the panic traces, and `#func()` printed in the same places.

```text
27  macro Parse(s string) int {
28  	n := strconv.Atoi!(s)
29  	return n
30  }
31
32  macro Around() {
33  	#body()
34  }
35
36  func load!(s string) int {
37  	n := strconv.Atoi!(s)
38  	return n
39  }
40
41  #Around()
42  func viaWrap(s string) (int, error) {
43  	f := func() (int, error) {
44  		n := load!(s)
45  		return n, nil
46  	}
47  	return f()
48  }
49
50  func guarded(s string) (err error) {
51  	defer func() {
52  		if r := recover(); r != nil {
53  			err = r.(error)
54  			show("a ! in a value macro: no error result, so it panics; read in the handler", err)
55  		}
56  	}()
57  	fmt.Println(#Parse(s))
58  	return nil
59  }
60
61  func main() {
62  	_, err := viaWrap("x")
63  	show("a ! in a closure inside the body of a wrap macro", err)
64  	guarded("y")
65  }
```

Output:

```text
-- a ! in a closure inside the body of a wrap macro
  etrace_macros.gp:37 main.load
  etrace_macros.gp:44 main.viaWrap.func1
-- a ! in a value macro: no error result, so it panics; read in the handler
  etrace_macros.gp:28 main.Parse
```

### 13. COST: a wrapper made only for a log line (`etrace_logwrap.gp`)

`step`'s deferred function makes `fmt.Errorf("step failed: %w", err)` only to log it, and the real error goes up. The wrapper is a wrapper of the error in flight, so it takes the identity; `top`'s `!` sees the real error, which no longer matches, and the trace restarts at `top`: shorter, and `step` is lost. This is a named cost of the single-identity rule (section 6).

```text
26  func parse(s string) (int, error) {
27  	return 0, errors.New("empty input")
28  }
29
30  func step!(s string) (n int, err error) {
31  	defer func() {
32  		if err != nil {
33  			_ = fmt.Errorf("step failed: %w", err)
34  		}
35  	}()
36  	n = parse!(s)
37  	return n, nil
38  }
39
40  func top!(s string) int {
41  	n := step!(s)
42  	return n
43  }
44
45  func main() {
46  	_, err := top("")
47  	show("COST: a wrapper made only for a log line takes the trace; the real error restarts at top", err)
48  }
```

Output:

```text
-- COST: a wrapper made only for a log line takes the trace; the real error restarts at top
  etrace_logwrap.gp:41 main.top
```

### 14. A hop made through an interface (`etrace_iface.gp`)

`S.A` is called through the interface `Svc`. The real hop returns into a compiler-made thunk, whose name says nothing about the caller, so the link after it cannot be checked and is a cut: the trace begins at `run` and the origin side (`A`) is gone (the program marks `A` for the simulation with `// sim: thunk`). The second part is the reviewer's case: `viaPlain` calls `A` through the interface, the hop stays in the register, and a later unrelated `!` on the same sentinel in `h2` has no old hop in front.

```text
26  var ErrX = errors.New("x")
27
28  func leaf() (int, error) {
29  	return 0, ErrX
30  }
31
32  type Svc interface {
33  	A() (int, error)
34  }
35
36  type S struct{}
37
38  // sim: thunk
39  func (S) A!() int {
40  	v := leaf!()
41  	return v
42  }
43
44  func run!(s Svc) int {
45  	v := s.A!()
46  	return v
47  }
48
49  func viaPlain(s Svc) (int, error) {
50  	return s.A()
51  }
52
53  func h2!() int {
54  	v := leaf!()
55  	return v
56  }
57
58  func main() {
59  	var s Svc = S{}
60  	_, err := run(s)
61  	show("a hop in a method called through an interface: the trace begins above it", err)
62
63  	viaPlain(s)
64  	_, err = h2()
65  	show("an unrelated propagation of the same value after it: no old hop in front", err)
66  }
```

Output:

```text
-- a hop in a method called through an interface: the trace begins above it
  etrace_iface.gp:45 main.run
-- an unrelated propagation of the same value after it: no old hop in front
  etrace_iface.gp:54 main.h2
```

### 15. An unrelated wrapper of a shared sentinel (`etrace_steal.gp`)

`load` wraps the sentinel `ErrPerm` with `Wrapf`, and a deferred function that runs after it makes `fmt.Errorf("audit: %w", ErrPerm)`. The sentinel is not the identity (the `Wrapf` wrapper is), so the audit wrapper changes nothing and `boot`'s `!` continues the trace. The second part is the cost: a log-only wrapper of the error takes the identity first, so the `Wrapf` that follows wraps an error that is not the identity, and the trace restarts at `bootLog`.

```text
26  var ErrPerm = errors.New("permission denied")
27
28  func open(path string) (int, error) {
29  	return 0, ErrPerm
30  }
31
32  func load!(path string) (n int, err error) {
33  	defer func() {
34  		_ = fmt.Errorf("audit: %w", ErrPerm)
35  	}()
36  	defer errors.Wrapf(&err, "load %s", path)
37  	n = open!(path)
38  	return n, nil
39  }
40
41  func boot!(path string) int {
42  	n := load!(path)
43  	return n
44  }
45
46  func loadLog!(path string) (n int, err error) {
47  	defer errors.Wrapf(&err, "loadLog %s", path)
48  	defer func() {
49  		if err != nil {
50  			_ = fmt.Errorf("log: %w", err)
51  			_ = fmt.Errorf("log again: %w", err)
52  		}
53  	}()
54  	n = open!(path)
55  	return n, nil
56  }
57
58  func bootLog!(path string) int {
59  	n := loadLog!(path)
60  	return n
61  }
62
63  func main() {
64  	_, err := boot("/etc/shadow")
65  	show("an unrelated wrapper of the sentinel, made after Wrapf: ignored", err)
66
67  	_, err = bootLog("/etc/shadow")
68  	show("COST: a log-only wrapper takes the trace, so the Wrapf that follows does not match", err)
69  }
```

Output:

```text
-- an unrelated wrapper of the sentinel, made after Wrapf: ignored
  etrace_steal.gp:37 main.load
  etrace_steal.gp:42 main.boot
-- COST: a log-only wrapper takes the trace, so the Wrapf that follows does not match
  etrace_steal.gp:59 main.bootLog
```

### 16. A handled `Wrapf` wrapper is not revived by a wrapper of its sentinel (`etrace_stale.gp`)

`a` wraps `ErrPerm` with `Wrapf` (the identity is the wrapper `W1`), `f2` handles the error and drops it, then calls `g`, which returns `fmt.Errorf("g: %w", ErrPerm)` and never calls `open`. The wrapper of the sentinel is not `W1`, so `f2`'s `!` starts a trace of its own. The second part does the same with the bare sentinel from `g2`. This holds only because the handled error was itself a `Wrapf` wrapper: when `a` passes the bare sentinel up with a plain `!`, the sentinel is the identity and the next scenario shows the opposite.

```text
26  var ErrPerm = errors.New("permission denied")
27
28  func open(path string) (int, error) {
29  	return 0, ErrPerm
30  }
31
32  func a!(path string) (n int, err error) {
33  	defer errors.Wrapf(&err, "a %s", path)
34  	n = open!(path)
35  	return n, nil
36  }
37
38  func g() (int, error) {
39  	return 0, fmt.Errorf("g: %w", ErrPerm)
40  }
41
42  func f2!() int {
43  	if _, err := a("x"); err != nil {
44  		fmt.Println("handled and dropped:", err)
45  	}
46  	n := g!()
47  	return n
48  }
49
50  func g2() (int, error) {
51  	return 0, ErrPerm
52  }
53
54  func f3!() int {
55  	if _, err := a("x"); err != nil {
56  		fmt.Println("handled and dropped:", err)
57  	}
58  	n := g2!()
59  	return n
60  }
61
62  func main() {
63  	_, err := f2()
64  	show("g wraps the sentinel and never called open: the trace must not say it did", err)
65
66  	_, err = f3()
67  	show("g2 returns the bare sentinel: the same", err)
68  }
```

Output:

```text
handled and dropped: a x: permission denied
-- g wraps the sentinel and never called open: the trace must not say it did
  etrace_stale.gp:46 main.f2
handled and dropped: a x: permission denied
-- g2 returns the bare sentinel: the same
  etrace_stale.gp:58 main.f3
```

### 17. A handled bare sentinel is revived by its wrapper, by itself and by a `Join` (`etrace_nowrapf.gp`)

The limit of scenario 11 through the rekey. `a` passes the sentinel `ErrPerm` up with a plain `!`, so the identity is the sentinel itself; `f2` handles the error and drops it, then calls `g`, which returns `fmt.Errorf("g: %w", ErrPerm)` and never calls `open`. `Errorf` makes a wrapper of the identity, the identity becomes the wrapper, and the wrapper **inherits the earlier hops**: `f2`'s `!` is in the function the last hop (`a`) returned into, so the link holds and the trace is `a, f2`. The trace begins with a hop of an earlier propagation, which `g` never made. Part 2 returns the bare sentinel from `g2` (the same, scenario 11 itself); part 3 returns `errors.Join(ErrOther, ErrPerm)` from `j` (the same through `Join`). Each hop is a real `!` of an object of the identity chain and each link is a caller and a callee; the two propagations are not told apart.

```text
26  var ErrPerm = errors.New("permission denied")
27  var ErrOther = errors.New("other")
28
29  func open(path string) (int, error) {
30  	return 0, ErrPerm
31  }
32
33  // a passes the sentinel up with a plain !, no Wrapf: the identity is ErrPerm itself.
34  func a!(path string) int {
35  	n := open!(path)
36  	return n
37  }
38
39  func g() (int, error) {
40  	return 0, fmt.Errorf("g: %w", ErrPerm)
41  }
42
43  func f2!() int {
44  	if _, err := a("x"); err != nil {
45  		fmt.Println("handled and dropped:", err)
46  	}
47  	n := g!()
48  	return n
49  }
50
51  func g2() (int, error) {
52  	return 0, ErrPerm
53  }
54
55  func f3!() int {
56  	if _, err := a("x"); err != nil {
57  		fmt.Println("handled and dropped:", err)
58  	}
59  	n := g2!()
60  	return n
61  }
62
63  func j() (int, error) {
64  	return 0, errors.Join(ErrOther, ErrPerm)
65  }
66
67  func f4!() int {
68  	if _, err := a("x"); err != nil {
69  		fmt.Println("handled and dropped:", err)
70  	}
71  	n := j!()
72  	return n
73  }
74
75  func main() {
76  	_, err := f2()
77  	show("g wraps the sentinel with %w: the wrapper inherits the hops of a", err)
78
79  	_, err = f3()
80  	show("g2 returns the bare sentinel: the hops of a", err)
81
82  	_, err = f4()
83  	show("j joins the sentinel: the Join inherits the hops of a", err)
84  }
```

Output:

```text
handled and dropped: permission denied
-- g wraps the sentinel with %w: the wrapper inherits the hops of a
  etrace_nowrapf.gp:35 main.a
  etrace_nowrapf.gp:47 main.f2
handled and dropped: permission denied
-- g2 returns the bare sentinel: the hops of a
  etrace_nowrapf.gp:35 main.a
  etrace_nowrapf.gp:59 main.f3
handled and dropped: permission denied
-- j joins the sentinel: the Join inherits the hops of a
  etrace_nowrapf.gp:35 main.a
  etrace_nowrapf.gp:71 main.f4
```

