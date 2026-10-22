# ADR-139: the address of a C/C++ local that leaves the function owns the local

Status: accepted (2026-10-22; job 03.lowfix).

## Context

`&x` of a C/C++ variable is a raw C++ address, and the variable ends with its block (ADR-130/131,
`docs/spec/interop.md` "`&x` of a C/C++ value"). `gp = &n` with `gp` a package variable compiled and then dangled
(`*gp` printed 21847 instead of 5; audit row L225), where Go keeps `n` alive as long as `gp` points to it. Only
`gocpp vet` (analyzer `cpplocal`) reported it, and the compiler said nothing.

## Decision

- A C/C++ local (also a parameter; also when only the address of a part of it, `&x.f`, `&x.in.a` or `&x[i]`, is
  stored) whose address **leaves the function's own objects** is boxed (`boxCEscape`), and each `&x` of it is
  `gp::cptr<T>(box)` (for a part, `gp::cptr<T>(gp::ptr<T>(box, &x.f))`): a gocpp pointer that shares the box, as `&x`
  of a boxed gocpp variable does. The stored pointer owns the object: it is destroyed when the last pointer is dropped,
  not at the end of the block, and a `gp::cptr` still converts to the raw `T*` that C and C++ take (borrowed).
- `addrStores` (`internal/lower/cbox.go`) decides in two steps. (1) Variables that may refer to the same object are one
  class (union-find over `a := b`, `a = b`, `p := &h`, `p := arr[:]`, `append(s, ...)`, a copy of a struct): a store
  through one is a store through all, and a class is **provably local** when every member is a pointer, slice or map
  declared in the function by `:=` or `var`, set only from `&local`, `new`, `make`, a composite literal, `nil`, `append`
  of such a value or another member, and none has its own address taken (`fill(&p)`, a pointer-receiver method on a
  slice or map variable). A local struct or array value is its own object. (2) The addresses flow to a fixpoint (a
  worklist over the classes): a store adds the `&` expressions in the stored value, and what the variables it reads
  hold, to the target's class, or to "leaves". A store whose value cannot hold a pointer (`int(fill(&st))`, a string, a
  bool) carries no address. The result is made once for each class, not once for each variable of it (an earlier
  version did it per variable, which is about cubic in the size of a class). Order of the statements does not matter
  (`g = p2; p2.next = p1; p1.p = &x`). The earlier version made two passes in source order and one-way copies, and missed reverse chains of depth 2 and aliases of local objects.
- **Leaves:** a package variable or a member of a C/C++ package; a selector, index or `*` of a pointer, slice or map
  that is not provably local (a parameter, a receiver, a range variable, a value of a call, a lookup, an assertion, a
  receive, a copy of package data, a pointer whose address is taken); a result, a `return` (also inside a value:
  `return T{p: &x}`, `res = append(res, &x); return res`), a channel send, a `go` statement (its arguments and the
  variables its function literal captures); the own locals of a function literal in a value that is stored, sent, returned or
  started stay local (the literal's body is a function of its own: `f := func() { var l T; use(&l) }; gfn = f`, `litLocalLeaves`); the destination of `copy(dst, src)` and
  `append(dst, src...)` when `dst` is not provably local (the elements go into its backing array). The first version
  followed only pointers that were set directly from package data and missed range variables, comma-ok lookups,
  type assertions, receives, slice expressions, closure parameters and values of calls.
- **A variable declared in an enclosing block** also keeps the address past the block of the local:
  `list = append(list, &x)`, `cur = &x`, `o.p = &x`, `arr[i] = &x`, `m[k] = &x` or `h.p = &x` in a loop, an `if`, a
  `switch` or a function literal, with the variable (or an alias of it, or the object it refers to) declared before
  it. The address is boxed like a leaving one, except in a function declaration with a `defer` statement of its own, where
  the frame holds the local to the end (`addrHeld`, ADR-130/131) and nothing is boxed. The exception is not for a
  function literal with a `defer` of its own (the holder may be declared outside it and outlive the call) nor for the
  body of a range-over-func statement (a lambda that ends before the function does; the frame does not hold what it
  declares): there the local is boxed. A variable declared in the same block
  as the local, or in an inner one, does not keep it.
- The address of a C member of a gocpp object that is reached through a pointer (`gi = &w.in.b` in a method of
  `*Wrap`, with `in` a C struct field) is stored as `gp::cptr<T>(gp::ptr<T>(w, &member))`, which shares the owner `w`.
  Before, it was a raw pointer into the object, a heap-use-after-free when the last gocpp pointer was dropped.
- It is the same for the typed and the opaque interop mode: the lowering does not look at how the names were resolved
  (the element type is spelled with `decltype` for a variable declared from a C++ value).
- `cpplocal` reports `return &x` (an error of the build as well) and the stores into a local variable or channel
  declared outside the block of `x`, in a function without `defer`. The second kind is now a harmless report: the
  compiler boxes `x` for it (the enclosing-block rule above), so the pointer does not dangle. The report stays for the
  moment as a note of where a pointer outlives the block of its variable; dropping it is a follow-up.

## Rejected

- **An error for `pkgvar = &x`.** Some such programs are right (`saved = &c` that only a deferred call reads in
  `defer_hold_order.gp`), and the analysis cannot tell "kept" from "dead before the block ends"; a compile error
  would reject valid programs. Ownership is what Go does, and it costs one heap box for the locals that need it.
- **Boxing every local whose address is taken.** The raw address is C++'s documented behaviour for a pointer that only
  goes to a call, and the box costs an allocation. The measured cost of the rule: the emitted C++ of tests/run (2186
  programs), tests/interop, tests/zoo, tests/diff, tests/typed, tests/golden, tests/overflow, tests/race, examples, demos,
  bench and a 166,000-line annotated emission of the standard library packages is byte-identical to dev, except for the
  programs that store `&x` out of the function (the tests of this job, `defer_hold_order.gp`, and the `go` statements and
  channel sends of `chan_cpp_values.gp` and `go_objects_error.gp`). The cost of the analysis, measured with
  `gocpp emit` on one function of generated code, dev against this branch: 5,000 locals `p_i := &x_i; use(p_i)` with a chain
  `q = p_i` over all of them 0.60 s against 0.63 s (20,000: 4.9 s against 4.6 s; 1,600: 0.15 s against 0.17 s; 800: 0.09 s
  against 0.09 s); one cursor `cur = &x_i` reused over 1,500 locals 0.10 s against 0.11 s. The work is a pass over the
  function for the classes and a worklist over them for the addresses, with the result made once per class: it grows in
  step with the function, and one class of 2,000 variables takes 0.2 s in `TestCAddrBigClass`. An
  earlier version made the result once per variable of the class (about cubic: 112 s at 5,000 aliases); the test's time
  bound pins the difference. The cost of a function with many literals that have a `defer`, `gocpp emit`, best of 3, dev against this branch
  (a machine shared with other jobs): 500 literals that are called where they are written (`func() { defer cleanup(); var y
  C.int; cur = &y }()`) 0.70 s on dev and 0.08 s here, 1,000 of them 3.3 s and 0.16 s; 500 literals that are first stored in
  a variable (`f0 := func() { ... }`) and called later 43.9 s on dev and 0.09 s here (the "45 s" of the first report of this
  job is this shape; a review that measured only the first shape got 0.9 to 1.1 s at 500 and 3.2 to 4.0 s at 1,000, which
  is also right for it). Dev analyses the enclosing function again for each literal, and for each held local; this branch
  once for the function declaration.
- **One analysis for each function literal.** A literal that analyses only its own body misses what the enclosing function
  does with the address: a literal that stores its own local into a holder the function reads (`h.p = &y`, three levels
  deep, a deferred literal that stores `&y` of the enclosing local), and a generic function whose instances have their own
  type information. The analysis is made once for each function declaration (`addrAnalysis`) and shared with the
  literals and range-over-func bodies inside it; making it again for each literal was quadratic (about 31 s at 2,000
  range-over-func loops). A generic instance has its own analysis (the types of the same syntax differ). The package-level
  initializers are the exception: `gp_var_init` lowers all of them with one function generator, so an analysis belongs to
  one initializer, and the literals of a later initializer are not analysed with the first one (a literal in
  `var second = func() ...` after `var first = ...`; the cache is keyed by the initializer, `TestCAddrOwnership`
  "package initializers", `cvar_addr_package_literals.gp`).
- **Counting an address once for each class that holds it.** `q := &x; ho.p = q` has two classes that hold `&x`; the
  analysis processes the address once for the function (the `seen` set of the outliving pass), which is also what keeps 2,000
  holders of 50 loop addresses and 8,000 literals at 0.29 s instead of 1.0 s (`TestCAddrManyHolders`). Where the set is
  filled matters: an address marked before it is known to be tracked is lost for the second class
  (`cvar_addr_holders.gp`, 24 pairs).
- **Following the address into Go functions** (`register(&x)`): needs a summary per function (which parameters it keeps)
  across packages, a vet fact; not built (see below).

## Coverage

Covered (tests `tests/interop/cvar_addr_package_var.gp`, `cvar_addr_through_pointers.gp`, `cvar_addr_flows.gp`,
`TestCAddrOwnership` in `internal/lower`, mutation-checked): the address of the variable, of a member (`&p.b`, also
when nothing else keeps the box alive), of a member of a member (`&r.in.b`), of an element of a C array (`&r.arr[2]`), of
a C member of a gocpp struct value (`&o.in.a`) and through a pointer (`&w.in.b`, a receiver, a copy of it, a pointer
parameter), in a loop (`append` of a per-iteration local), through a local variable that holds the pointer, through
a range variable, a comma-ok lookup, a type assertion, a slice expression, a struct that holds package data, a closure
parameter, a channel receive, a pointer parameter, a value of a call, a returned slice or value or named result, a
channel send; through chains of local objects in any statement order (depth 2 and 3), through two variables of one
object (`q := p`, `p := &h` with `&h` leaving, `t := s`), through the address of a pointer variable (`fill(&p)`), a
pointer-receiver method on a slice variable, `copy` into package data and into a local slice that leaves, `append` into
a package slice that has capacity, a `go` statement's argument (also of a named function) and captured variable, a map put,
a map or a pointer parameter, an alias of a parameter, a store two steps through pointers, a store through `*` of a
parameter, and a variable declared in a block that encloses the block of `x` (`TestCAddrOwnership`; `list = append(list,
&x)` in a loop), also in a function literal with its own `defer` (a variable, a struct field, `append` called twice, an
inner block, a literal inside a literal) and in the body of a range-over-func statement of a function with a `defer`
(`cvar_addr_flows.gp` with `clobber()`); an address stored in an interface or an `unsafe.Pointer` that leaves. A part's address
is an aliasing `gp::cptr<T>(gp::ptr<T>(box, &member))`. The bindings of a type switch are no stores; an address that
only goes to a call, to a plain local, or to a provably local object of the same block (a literal, `new`, `make`, `&local`,
a chain of those that does not leave, a function literal's own local in any value that is stored or started: `go`, `gfn = f`), and a value that holds no pointer that
is computed from it (`return int(fill(&st))`, `int(*p)`, `rc := fill(&st); return int(rc), int(st.b)`) stays raw.

## Not covered

Still C++'s lifetime rules (documented in `docs/spec/interop.md` and `docs/guides/vet.md`; ASan finds them):
- a call that keeps what it is given: a Go function or method that stores its parameter (`register(&x)`, `s.Push(&x)`,
  also `register(p)` with `p` holding `&x`), a function value (`cb(&x)`, the `yield` of a range-over-func), a standard
  function that writes into its argument (`slices.Insert(gptrs, 0, &x)`); the lowering does not follow a value into a
  call (a per-function summary of the kept parameters would be a vet fact across packages: not built; **TODO claim**);
- an address that a C function keeps, and an address stored in a raw pointer member of a C/C++ struct or class that
  leaves (`h.p = &x; g = h` with `struct HP { int* p; }`): the member is a C++ raw pointer that owns nothing, so the box
  dies with the function (the same lifetime as before this ADR, with a heap box instead of the stack);
- the address of a C member of an element of a gocpp slice (`&sl[1].a`), a raw pointer into the element that dangles
  when the slice is dropped (not a local, the same on dev: **TODO claim**).

A step through a C pointer field in the path of a member address (`&n.next.v` with `next` a C pointer) boxes `n` as if the
member were a value (over-retention only; the opaque mode cannot tell a pointer field from a value member). A C/C++ scalar result that is not converted
(`return lib.fill(&st)`, `rc := lib.fill(&st); return rc`) is taken to hold a pointer too, because the lowering does not
know its C type: `st` is boxed (the `int(...)` forms are not). Over-retention
in general is allowed: a local whose address may leave is boxed, and lives until the last pointer is dropped, which Go does
as well.