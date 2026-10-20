# ADR-086: A pointer parameter or receiver that does not outlive the call is borrowed, and a parameter that is not assigned is a const reference

Status: accepted (2026-10-20). Builds on [ADR-004](004-memory-arc.md) (pointers are `gp::ptr<T>`, counted) and
[ADR-030](030-generic-instances.md) (generic instances); step 15.3 of Phase 15 (receiver and parameter
optimizations).

## Context

Every pointer receiver and pointer parameter was a by-value `gp::ptr<T>`: each call copied the caller's pointer
(one reference-count increment) and dropped the copy on return (one decrement, plus the thread-local nesting
bookkeeping of `gp::detail::release`). A promoted method added a `gp::field_ptr(mid, &mid::base)` (an aliasing
`shared_ptr`) for every call, and an interface call copied the pointer again in its thunk (`gp::unbox<ptr<T>>`).
The 15.1 benchmark suite showed what that costs: `embedding` (promoted pointer methods through three levels,
directly and through an interface) was 22x slower than Go, `iface_call` 5 to 6x. A `shared_ptr` copy and drop
costs about 4 ns while the process is single-threaded, and 15 ns as soon as one goroutine has started
(libstdc++ switches to atomic operations for the rest of the process).

Most of that is not needed: the callee of `func (b *base) inc() { b.n++ }` only reads and writes through the
pointer. The caller already holds the object (it has a variable, a field, a temporary), so for the time of the call
no second owner is required. The risk is the reverse: a callee that drops the last other owner of its own receiver
(`global = nil`, then uses the receiver) must not free the object under itself. The audit of step 3.10 showed both
the speed and the danger (`const&` receivers read freed memory; value receivers are real copies for that reason).

## Decision

A pointer parameter (or the pointer receiver) of a function is **borrowed** when the callee's body cannot keep it
past the call. A borrowed parameter has the C++ type `gp::bptr<T>`: a raw `T*` (8 bytes, trivially copyable,
no count) with the same `->`, `*`, `==` and nil checks as `gp::ptr<T>`, so the callee's body is lowered as before.
Everything else about the caller keeps the object alive.

### 1. Which parameters are borrowed (callee side, `internal/lower/borrow.go`)

An analysis of the callee's body, one function at a time, with the summaries of the functions it calls (a fixpoint
over the package, optimistic for recursion; the packages one after the other in the order of their imports, so a
call into another package sees that package's summary). A parameter `p *T` is borrowed when **every use of `p`** is
one of:

| use | why it is fine |
|---|---|
| `p.f`, `(*p).f`, `p.f.g[i]`, `*p`, `*p = v`, `p[i]` (pointer to array), `len(p)` | reads or writes the pointee's value |
| `p == q`, `p != nil` | compares addresses |
| `p.M(...)`, `p.Inner.M(...)` with `M` a value method | reads through `p` |
| `p.M(...)`, `p.Inner.M(...)` with `M` a pointer method whose receiver is `p` or a field of `*p` | the callee borrows its receiver |
| `f(p)`, `f(&p.f)`, `x.M(p)` | the callee borrows that parameter |

and anything else makes it **escape**: stored anywhere (also `x := p`), returned, captured by a func literal or a
range-over-func body, converted to an interface, sent on a channel, used in a `go` or `defer` statement, passed
to a parameter that is not borrowed, an interface method, a func value or C/C++ code, reassigned (`p = ...`),
address-taken (`&p`), turned into a method value (`p.M`), converted (`(*U)(p)`), a slice of an array in `*p`
(`p.arr[:]`), or the address of a part of `*p` (`&p.f`) that does not go straight into a borrowed parameter.
A function is not analysed (and keeps owning pointers) if it is generic, a method of a generic type, aware
(`recover`, step 5.10), `//export`ed, or, in a library build (`-buildmode` c-archive, c-shared, cpp-lib), exported
(the C++ API of a library keeps its signatures). Unknown code (a call that the analysis cannot resolve to a
function of the program) counts as escaping.

A function is **pure** when it cannot drop an owner: it stores no value that can own memory (a pointer, slice,
map, interface, func, channel, or a struct or array holding one) except into its own local variables,
runs no unknown code (no call through an interface or func value, no C/C++ operation, no `go`, `defer`, channel
operation, `select`, func literal, range over a func or channel), and calls only pure functions. C++ operations
include implicit construction, destruction, copy/move, assignment and conversion: a foreign part in a formal,
receiver, result or body value (also through a Go aggregate or container) conservatively makes the function
impure. Appending to, copying into, clearing and deleting from slices and maps of owning elements are stores.
Everything the pure function writes is a number, bool, string or struct of those.

### 2. How a call lends (caller side, `internal/lower/borrowemit.go`)

The raw pointer is valid if the object is alive until the call returns. The call site makes sure of it, in this
order of preference:

1. **Nothing can free it**: the argument is a temporary (a call's result, `&T{...}`), a local variable that is not
   boxed (nothing but an assignment in this function changes it), a hoisted temporary of the order analysis, a
   borrowed parameter of the enclosing function, a boxed variable's box (`&x`, the frame holds it), the address of
   a package variable, or the address of a field or element of such an object (`&p.f`, `p.Embedded.M()`,
   `&arr[i]`): `gp::lend(p)`, the temporary itself, or `gp::borrow(&p->f)`. For a promoted pointer method
   the receiver is the address of the embedded field inside the outer object, not a new aliasing pointer.
2. **The callee is pure and nothing evaluated before the call can run code** (no explicit call or implicit
   foreign operation in the receiver or other arguments): a pointer loaded from a field, element, package
   variable or box is lent as it is. While the pure callee runs, no owner stored anywhere can be dropped, so
   the object stays alive. Callee purity alone says nothing about sibling argument evaluation.
   When a Go call has foreign argument work, its explicit calls retain their Go evaluation order, then its
   remaining argument values are collected into owning snapshots in source order. A scalar argument such as
   `len([]Foreign{foreign})` can hide a foreign copy; its earlier pointer and string arguments need owners too.
   Synthesized map, slice and builtin units that perform implicit foreign work stay with their argument's
   collection instead of being hoisted wholesale. Explicit child calls are evaluated once in the Go call
   phase. Ordinary qualified functions with lowered Go bodies follow the same rule, including `defer` and
   `go`; C++ namespace calls and fmt's special runtime conventions are not reclassified.
   Each receiver/argument snapshot also retains ordinary intra-expression check ordering. Its nil,
   bounds and map-key checks are planned at that snapshot boundary, not globally before earlier owners
   exist. Explicit calls and synthesized units already collected remain named rather than running again.
3. Otherwise the argument is **copied** (`gp::ptr<T>(x)`, a temporary that lives to the end of the call), which
   is what every call did before.

A variadic callee is lent the same way for its fixed parameters (`use(g, 1, 2)`, `t.m(xs...)`, `use(g)` with
no variadic arguments): only the trailing arguments, which the call packs in one slice, are left alone.

`gp::bptr<T>` converts implicitly from a `gp::ptr<T>`, so C++ code that calls the static members of a generated
struct (interop, the concepts of `-gp:concepts`) compiles unchanged. It does **not** keep the pointer alive: a
C++ caller that passes a `gp::ptr` global that the Go callee reassigns would use a freed object, the same
as with any C++ reference. That is why a library (`-buildmode=cpp-lib`, c-archive, c-shared) keeps owning
`gp::ptr` parameters in the signatures of everything it exports (`TestProgramLibraryBorrow`); only the
unexported functions of a library borrow. The generated code writes every borrow of a named pointer as
`gp::lend(...)`, `gp::borrow(&...)` or a copy.

### 3. Every other way into a function

- **Interface method tables.** A thunk of a pointer method with a borrowed receiver calls
  `T::M(gp::unbox_borrow<T>(self), ...)`: no copy of the pointer; a value method through a `*T` also borrows it
  (its member function copies or reads `*this` before any other code runs). `gp::icall`'s contract is that the
  interface value `self` stays unchanged while the method runs, so the call site copies an interface value that
  the method could change (a field, a package or boxed variable, an element: `gp::Shape(x).M()`); a local variable
  that is not boxed, a temporary and a deferred call's own copy are lent as they are.
- **Promoted methods** (`promote.go`): a forwarder takes its receiver and parameters as the method it forwards to
  does, and lends the embedded field (`gp::borrow(&gp_p->Inner)`).
- **Method values and expressions, func values** of a function with borrowed parameters: a generic lambda that
  names its parameters (they live for the call) and lends the borrowed ones. A callback to C/C++ (`gp::cgo::fn`)
  of such a function goes through such a lambda too. The lambda of a method expression that is called directly
  (`(*T).M(&T{}, g)`) takes the receiver and every borrowed or const reference parameter **by value, with its
  Go-level C++ type** (for a pointer, `gp::ptr<T>`). `auto&&` would bind to the caller's variable (`g`) and let the
  callee change it while using it, while `auto` would lose the formal pointer type for literal `nil`. A Go call
  passes typed copies. An adapter or promoted-method forwarder also snapshots an embedded pointer used as a
  const-reference receiver when the callee can replace its source: holding the outer object alone is not enough.
- **`go` and `defer`**: argument collection takes owning Go-level values, including for const-reference
  formals, before building the frame's tuple. Decaying a reference only when the tuple is built is too late:
  a foreign sibling's copy can call back and change its source first. Calls with implicit foreign argument
  work use the ordered snapshots above for both `go` and `defer`. Direct collections map a borrowed
  `gp::bptr` formal to an owning `gp::ptr`; adapters likewise store owners and lend them when called.
- **Value receivers** are not changed: a value method is still a `const` member function that copies `*this` (or
  reads it by `const T&` when the body provably cannot change or free the object, ADR-004's audit).
- **Nil.** A borrowed pointer may be nil: `p == nil` works, and the first `->` or `*` panics with Go's message, as
  a `gp::ptr` does; a promoted method through a nil embedded pointer, method values and expressions of nil
  receivers behave as before (tests `borrow_nil`).

### 4. Soundness

For race-free code within the supported analysis boundary, a raw pointer is used only while some owner the
caller holds cannot change, by the rules above: a stable local, a temporary, the immortal static owner, or
(rule 2) an owner in memory that the pure callee cannot drop. Unknown/native calls are not pure: even an
Unlock/Wait pair can let another goroutine synchronously drop the last owner before the callee reads its argument.

A configuration pointer replaced without a lock while workers read it (`cfg = &Config{...}` or
`srv.cfg = &Config{...}` against `sum(cfg)`) has a data race. Go constrains single-word racy reads and its collector
keeps the old object; C++ does not give the same guarantee for a racing `gp::ptr` load/store. As a defensive
measure, lending a pointer **loaded from memory** (not a temporary or stable local) to a pure callee has a
second condition, decided over the whole program (packages are lowered together, ADR-025; `threads.go`). This
keeps an owning call argument instead of a raw lend, but **does not make the racing copy atomic or data-race-safe**:

1. **The program can run a second thread.** Without one, nothing can replace a pointer under a callee and the rule
   above is all there is. A program is *threaded* when a package of the program itself has a `go` statement
   anywhere (also in a function that nothing calls, in a package variable's initializer, in an `init`), or
   mentions a *thread source* (`threadSources`: `time.AfterFunc`, `runtime.SetFinalizer`,
   `sync.WaitGroup.Go`, `context.AfterFunc`), or reaches one of these or a `go` statement in the standard library:
   from all the code of the program's own packages and from the initializers and `init` functions of the standard
   packages, through references to functions. Unknown func/interface calls (including implicit iterator calls)
   conservatively count as potential threads, as do natives with func or interface callback parameters unless
   their behavior is explicitly reviewed in `synchronousNatives`. Finalizer/cleanup/user callbacks are not
   assumed synchronous. `TestThreadSources` audits the known native callbacks and runtime thread creators.
   All code of the program's own packages counts without a reachability test: a `String` or `Less` method, a
   callback given to `once.Do` or `sort.Sort`, a function of another package that starts a goroutine, and a
   function value that main stores cannot hide a thread. This is deliberately conservative: even a synchronous
   func-value call can disable the no-thread exemption. A library build (`-buildmode=cpp-lib`, c-archive,
   c-shared) is always threaded: foreign threads can call in, and C++ may assign every exported variable, field
   or element. There is no syntactic start-of-main or quiet-package shortcut.
2. **What a thread could replace.** In a threaded program an argument or receiver is copied for the call, even for a
   pure callee, when the path it is loaded by (`hotLoad`) goes through a package variable that the program assigns,
   a struct field that the program assigns (field of a type: `x.f` for any `x` of that struct type), or an element
   of a slice, map or array whose element type the program stores somewhere (`t[i] = p`, `m[k] = p`, `copy`,
   `clear`, `append`, `delete`, or a whole-array store). The path includes implicit embedded fields in field
   selections, promoted receivers, method-expression adapters and forwarders. Assigned means: `=` and `op=`,
   `++`, `range` with `=`, `&x` or `&x.f` or `&a[i]` (an alias that a store may use), slicing an array, a pointer
   method called on an addressable value (its receiver is `&x`), and a struct or array stored as a whole
   (`*p = v`, `s.in = v`: every inline field/element, without a nesting-depth cutoff). `:=` and the initializer
   of a declaration are not assignments. The assignments in the body of an `init` function (not in a closure in it) do not count
   when no `init` function and no initializer of the program can start a thread (transitively): they finish before
   `main` starts. Anything else counts, including closures and a function that only `init` calls.

Programs proved to have no second thread keep the no-copy path; a potentially threaded program loses lends
through assigned fields and elements, but not stable temporaries, locals or parameters. Earlier instruction-count
measurements of the previous gate showed the possible cost on `binarytrees` (linked by assignment): +22% against
its unthreaded build. Those measurements are historical, not a measurement of the final conservative gate.

The defensive replacement rule is not complete: it does not cover **reflect** and **unsafe** writes, an address
kept by code the analysis cannot see (**cgo** or a C++ library, **foreign threads**, finalizers of C++ objects), and
threads that a linked library starts by itself without a visible callback/thread source. In library mode visible
field/element loads and exported variables are treated conservatively, but this is not a foreign-code alias
analysis. In every mode, even a copied `gp::ptr` can race with a store and corrupt its reference count. No
memory-safety guarantee is made for racy programs (D10): use `atomic.Pointer`, a lock or a channel.
The `borrow_hot_*` and `borrow_hotswap` runtime fixtures are race-free: both publication and the complete
owner acquisition/call hold the same mutex. Three channel handshakes replace timing stress and large loops;
these programs do not claim to make unsupported races safe. `TestBorrowHotFixtureThreadFacts` isolates each
startup mechanism without formatting, mutexes or unrelated starters. `TestProgramBorrowHotGuard` separately
asserts owning copies and cold-path lends without mutexes, including controls removing the thread source
or replacement, so synchronization cannot mask a missing compiler guard. The multi-package
`borrow_hot_cross` positive keeps `once.Do(bg.StartWorker)`, publishes its hook before startup, and uses
the same protected publication/read protocol with a completion channel after the reader returns.
`TestBorrowHotCrossThreadAndLending` isolates the other package's go statement without fmt, mutexes, Once
or an unknown Hook call, and checks both thread facts and hot/cold emission controls. The synchronized
`borrow_native_*` regressions remain race-free and must pass `-race`. Compile-time tests: `TestThreadGate`,
`TestMutatedGlobals`, `TestMutatedFacts`,
`TestInitExemption`, `TestThreadSources`, and `TestBorrow` (including dependency cycles and unknown natives).
`TestBorrowForeignParts` covers implicit foreign operations through signatures, receivers, aliases,
recursive aggregates and containers; `TestBorrowForeignEmission` checks that their callers snapshot both
pointers and const-reference strings. `TestInterop/borrow_cpp_lifetime` exercises C++ constructors,
destructors, copies and assignments that clear the source owner, including through Go aggregates.
`TestBorrowNamedMethodValue` distinguishes copying a named scalar/container value receiver from retaining
its address for a pointer method value; `borrow_named_methodvalue` checks the returned closures against Go.
`TestBorrowSiblingForeignEmission`, `TestBorrowForeignDeferredEmission` and
`TestBorrowForeignSiblingKeepsExplicitCallOrder` check argument collection. The native
`TestInterop/borrow_cpp_arguments` fixture covers sibling copies/conversions, direct and method/func-value
calls, plus string snapshots in both `defer` and synchronized `go` calls.
`TestBorrowQualifiedForeignSnapshots` covers qualified direct/defer/go collection;
`TestBorrowNestedForeignSnapshots` covers synthesized map/slice/builtin units with nested and surrounding
explicit-call controls. `TestBorrowSnapshotArgumentChecks` pairs ordinary and foreign-sibling calls
for intra-argument nil/bounds/division/map-key checks, receiver checks and once-only explicit calls.
The native argument fixture exercises these paths with positive callbacks and valid indexes/pointers too.
The mutation checks are focused evidence, not a proof that every possible unsound change is detected.

### 5. Parameters that the body never assigns are const references

A parameter of a type that owns memory (a string, slice, map, channel, func, interface, or a pointer that is not
borrowed because it is stored, returned or captured) that the body never assigns as a whole (`p = ...`, `p++`,
`for p = range`, `&p`, in the body or in a func literal in it) and never returns as it is (`return p`: a by-value
parameter is moved out there, a reference would be copied, a count that the caller's copy of an argument loaded from
a field has paid already: `insert(t.left, k)` of a tree), that no closure boxes and that is not larger than
128 KiB is a **`const T&`** (the same rule that gave large values a const reference). Writing through it
(`xs[i] = v`, `p.f = v` for a pointer) is not an assignment of the parameter. A callee that needs an owning copy (it
stores or captures the parameter) copies from the reference where it does, which costs what the by-value copy at the
call did, so no call is slower.

The call passes the argument as it is, with no copy, when nothing can change it during the call: a temporary (a
call's result, a literal, a concatenation, a slice expression, `&x`), a constant, or a local variable that is not
boxed (borrowemit.go: `refStable`). Otherwise it passes a copy (`gp::string(x)`, a temporary that lives to the end of
the call), which is what every call did, or the argument itself if the callee is **pure for values**: it stores no
string, pointer, slice, map, interface, func or channel (a string counts here, unlike for pointers: overwriting
a string field would change the argument that refers to it) except into its own locals and runs no unknown code,
and nothing evaluated before the call runs code. A string field passed to a callee that overwrites it
(`rename(r, r.name)`) is therefore copied, a slice field passed to a callee that sorts it in place is not.

A func value of such a function, and a callback, goes through `gp::adapt<&f>::call`, one adapter function for each f,
that takes the Go-level parameters (an owning pointer, a value) and calls f, so the func value has one code address
wherever it is made (`%p`). A function with a parameter larger than 128 KiB keeps its signature, and so does a
method value (a lambda). `go` and `defer` collect a copy of the argument before constructing their storage:
`gp::detail::owning_t` removes a const reference and maps a borrowed pointer to its owning pointer.

## Alternatives

- **`const gp::ptr<T>&` for every pointer parameter** instead of a raw pointer: the callee needs no analysis, but it
  cannot refer to the address of an embedded field at all, which is the big cost of promoted methods, and a
  reference to the caller's variable sees the variable change (`global = nil` in the callee). A const reference is
  used for what is not borrowed (section 5), where the caller passes a local that nothing can change, or a copy.
- **`this` as the receiver** (a non-static member function, as for value methods): C++ would need a `T*` that
  can be nil (undefined behaviour to call a member on it) and still would not give the callee a way to keep it.
- **Escape analysis first (15.2)**: it would keep non-escaping objects on the stack, a bigger win, but it is not
  built; this analysis is the small intra-procedural part that needs no allocation decisions, and the summaries
  it computes (does `p` escape, is the function pure) are the ones 15.2 needs per function.
- **Intrusive counts (15.4)**: cheaper copies, but still copies.

## Consequences

Good: no reference-count traffic for a call whose receiver or arguments are stable; promoted methods call the
embedded field's method on its address; interface calls on a stable interface value copy nothing; global structs
(`mu.Lock()` on a package `sync.Mutex`) no longer touch the shared static owner's count on every call.

Bad: the signature of a function now depends on its body (a body change can change whether a parameter is
borrowed, so generated headers change); a callee's escape is only found within the program (a package from a
cache built by an older gocpp is regenerated anyway, the generated C++ is not a stable API); `gp::bptr` in a
C++ signature is new to people who call generated code from C++; an interface value is copied when a method
could change it (one extra count where the thunk copied the pointer before: the same cost).

## Evidence

Benchmarks (15.1 suite, `taskset` one CPU, median of 7, CPU seconds; Go 1.25.14; machine shared): see
[Phase 15 §3.2](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#32-notes-on-step-153-borrowed-pointers-adr-086). The tests: `TestBorrow` (the
analysis, 100 functions, each escape rule), `TestRun/borrow_*` (g++, clang++, `-asan`, `-race`, Windows), the goldens
of `internal/cppgen` and `tests/golden`, mutations of the analysis and of the lending (each must fail a test),
and `tools/borrowfuzz`: random programs (pointer and value parameters in every call form) that real Go and
gocpp (`-asan`, both compilers) must run alike.

See: [Phase 15 §2.1](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#21-performance-measure-first),
[cpp-mapping.md, Methods](../spec/cpp-mapping.md#methods).
