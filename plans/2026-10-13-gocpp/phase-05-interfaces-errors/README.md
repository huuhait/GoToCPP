# Phase 05: Interfaces, Embedding, Errors, Panic/Recover

> Add Go's tools for abstraction: implicit (structural) interfaces, `any`, the `error` type,
> type assertions and type switches, struct embedding with promotion, and full
> `panic`/`recover`. This turns gocpp from "C with Go syntax" into a real Go-style language.

| | |
|---|---|
| Depends on | Phase 03 (single-exit function shape, `defer`), Phase 04 (C++ exceptions become recoverable panics) |
| Unlocks | Phases 06, 07, 08, 09, and Phase 10 (`!` builds on this phase's errors) |
| Size | large (about 5 weeks) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

Go interfaces are satisfied **implicitly** ("if it has the methods, it fits"), and values can be
checked at runtime (`x.(T)`, type switches). C++ virtual functions need explicit inheritance,
so we don't use them. Instead gocpp copies **Go's own runtime design**:

- every type that can end up inside an interface gets a **type descriptor** (name, size, equality,
  hash, formatting, method table);
- an interface value is **(data, type, itab)**, where the itab is a small table of function
  pointers for that (interface, concrete type) pair;
- itabs are built by matching method **names and signatures**, then cached. That is exactly
  what Go's `getitab` does.

One rule matters more than any other here: **descriptors are chosen by the Go type, never by
the C++ type.** Several Go types share one C++ type (`int` and `int64` are both `int64_t`;
`uint`, `uint64` and `uintptr` are all `unsigned long` on Linux; `chan T`, `<-chan T` and
`chan<- T` are all `gp::chan<T>`). If descriptors followed the C++ type, `any(1).(int64)` would
succeed and `%T` would print the wrong name. In Go it fails (checked in the audit).

`panic`/`recover` use the Phase 03 single-exit function shape plus C++ exceptions, giving Go's
exact order: defers run during panics, and `recover()` works exactly where Go says it does.

## 2. How it works

### 2.1 Example

```go
type Shape interface{ Area() float64 }

type Rect struct{ W, H float64 }

func (r Rect) Area() float64 { return r.W * r.H }

type Circle struct{ R float64 }

func (c *Circle) Area() float64 { return 3.14159 * c.R * c.R }

func main() {
	shapes := []Shape{Rect{2, 3}, &Circle{1}}
	for _, s := range shapes {
		switch v := s.(type) {
		case Rect:
			fmt.Println("rect", v.W, v.Area())
		case *Circle:
			fmt.Println("circle", v.R, v.Area())
		}
	}
}
```

Generated C++ (simplified):

```cpp
struct Shape : gp::iface {                      // one C++ struct per Go interface type
    using gp::iface::iface;
    double Area() const;                        // defined after all types (header section 6)
};

gp::slice<Shape> shapes = {
    gp::to_iface<Shape>(Rect{2, 3}, &gp_td_Rect),                     // descriptor of the GO type
    gp::to_iface<Shape>(gp::new_<Circle>(Circle{1}), &gp_td_ptr_Circle),
};
for (/* range */) {
    Shape s = ...;
    if (s.gp_type() == &gp_td_Rect) {
        Rect v = gp::unbox<Rect>(s);
        ...
    } else if (s.gp_type() == &gp_td_ptr_Circle) {
        gp::ptr<Circle> v = gp::unbox<gp::ptr<Circle>>(s);
        ...
    }
}
```

> **Built in 5.1-5.3 (2026-10-17), where it differs from the text below:** `gp_td_ptr_Circle` is
> `gp::td_ptr<Circle>(&gp_td_Circle)` (interned at runtime from `Circle`'s second method table); method signatures are compared as
> canonical text, not as descriptors; wrapper slots follow the interface's sorted methods; nil of an interface type is
> `Shape()`. The exact emitted shapes and the contracts for the emitter are in
> [docs/spec/cpp-mapping.md](../../../docs/spec/cpp-mapping.md) "Interfaces".
>
> **Built in 5.4-5.5 (2026-10-17), where it differs from the text below:** the conversions are
> `gp::to_iface<I>(v, desc)` and `gp::iface_cast<I>(x)`, the assertions `gp::assert_type<T>(x, desc)`,
> `gp::assert_type2`, `gp::assert_iface<I>(x)` and `gp::assert_iface2`, the switch tests `gp::type_is(x, desc)`
> and `gp::type_has<I>(x)` (not `s.gp_type() == &gp_td_Rect` and `gp::unbox` alone: the helpers carry Go's
> panic messages and the itab caches); the descriptors of composite types are namespace-level pointers
> (`gp_tdc_N`, `gp_tdp_T`) interned at program start, the descriptor objects of named types are
> constant-initialized with partial shapes (the element only where it is a constant descriptor, no field
> list: the C++ types give the rest); `error` is `gp::error` (a runtime struct like the generated ones);
> interface `==` and hashing (step 5.6, completed with 5.9: the unhashable-key check reaches into struct and
> array keys, `gp_hash_panics`, `gp::check_key`); a conversion of a value to an interface is ordered like gc's (a copy in its place among the
> calls, as an argument of fmt's `...any` is); a C/C++ value goes into an interface that has no methods
> with its foreign descriptor, and `x.(CType)`, its comma-ok forms with `:=` and `var`, and type switch cases of C/C++
> types work. A type with a promoted method has the promoted methods in its tables (5.8).
>
> Known differences from gc (5.4/5.5; also in docs/spec/cpp-mapping.md "Interfaces"):
> (1) the ORDER of a conversion to an interface inside a nested expression: gocpp copies the value in
> its place only for an argument or an operand of a statement's list (like fmt's `...any`), so with `g` a
> package struct and `mod()` a call that changes it, `any(g).(P).a + mod()` gives 107 in gocpp and 8 in gc,
> and `[]int{any(g).(P).a, mod()}` gives `[100 7]` in gocpp and `[1 7]` in gc; (2) a call of a value method
> through a nil `*T` in an interface gives Go's wrapper message ("value method main.T.M called using nil
> *T pointer") always, where gc devirtualizes a call whose interface type it can see and then prints the plain
> nil dereference; (3) a value of a Go struct type that has a C/C++ field is not comparable through an
> interface (== of two such interface values panics "comparing uncomparable type", Go compares the fields);
> pointers to it compare fine; (4) a C/C++ value goes only into an interface without methods (Phase 12);
> a method of a Go type with a C/C++ type in its signature is not in its method table; (5) a map key
> that is a C++ value with `==` and no `std::hash` has one hash for all its values.
>
> **Built in 5.11 (2026-10-18), where it differs from the text above and below:** `fmt` is a port of
> Go's `fmt/print.go` and `format.go` (`runtime/src/fmt.cpp`) driven by the descriptors, and the `fmtshim` is
> gone ([ADR-028](../../../docs/adr/028-fmt-on-descriptors.md)). The descriptor of a named slice, array, map, func,
> pointer or struct type stays partial and constant-initialized, with `gp_full`, a function the emitter writes
> (`gp_full_Rect`, internal/lower/typefull.go) that makes the complete copy once, with the field list (names,
> tags, **offsets**), element, key, parameters and results; `gp::full(d)` reads it, and `type_desc::gp_ops`
> (gocpp/typeops.hpp) has the operations that read a slice, map, pointer or foreign value. Operands are
> plain values (the C++ type says the Go type) or `gp::fmt::as(descriptor, value)`; the types of package fmt
> (`Stringer`, `GoStringer`, `Formatter`, `State`) are runtime interface structs like `error`; `Errorf`
> builds the runtime's `*errors.errorString`, `*fmt.wrapError`, `*fmt.wrapErrors`; `Fprint*` take `os.Stdout`,
> `os.Stderr` and a `fmt.State`; the gates for every verb, `#`, `*`, `[n]` and non-constant formats are lifted
> to Phase 05. The `gp_String`/`gp_Error` hooks, the type tags and `gp_type_name` of the structs are gone
> (`gp_fields` stays for `gp::convert`). `tests/run/fmt_*.gp` (about 190 programs generated from real Go's
> output) compare byte for byte with Go; `%T` of a recovered assertion panic is `*runtime.TypeAssertionError`
> (`fmt_methods_recovered_values.gp`).

### 2.2 Type descriptors (chosen by the Go type)

`gp::type_desc` holds the Go-style name (`main.Rect`, `[]int`, `*main.Circle`), kind, size,
`equal` and `hash` functions (null when the type is not comparable), a `format` function (for
`fmt`'s `%v`), and for named types a **method table** `[{name, signature, thunks}]` sorted by
name. Method sets follow Go: the table of `T` has the value-receiver methods, and the table of
`*T` has all of them. Unexported method names are stored **with their package path**, as Go
does, so another package's unexported `m()` can never satisfy an interface.

Where descriptors come from:
- **named types** get a generated descriptor object (`gp_td_Rect`);
- **predeclared types** have one each in the runtime, distinct even when the C++ type is
  shared: `gp_td_int` and `gp_td_int64` are different objects;
- **composite types** (`[]int`, `map[string]int`, `<-chan int`, `func(int) string`, `*T`) get
  descriptors built from their parts and **interned**, so there is exactly one per Go type and
  pointer equality still means type identity;
- **C/C++ types** stored in an `any` get a *foreign* descriptor, generated from a template and
  named after the C++ spelling. Equality, hash and printing use `requires` checks
  (`operator==`, `std::hash`, `std::formatter` or `operator<<`), and raw pointers print as addresses.

Codegen always knows the Go type from `go/types`, so it passes the right descriptor at every
boxing site, type assertion, type switch and `%T`. Generic code gets one hidden descriptor
argument per type parameter (Phase 07). Only types that actually flow into an interface pay for
a descriptor.

### 2.3 Interface values and itabs

- Layout: `{ std::shared_ptr<void> data; const type_desc* type; const itab* tab }` = 32 bytes.
  Pointer-shaped values (`*T`) share their existing owner (no copy). Other values (structs,
  numbers, strings) are copied into a new box, like Go.
- **Thunks:** for each method of each named type, codegen emits one thunk with a uniform shape,
  `R thunk(const gp::iface& self, Args...)`. It calls the method in the right form: value
  method `unbox<T>(self).M(args)`, or pointer method `T::M(gp::ptr<T>(self.data), args)`.
- **itab building** (`gp::getitab(iface_desc, type_desc)`): for each interface method, find the
  method with the same name and identical signature in the type's table, and copy its thunk.
  Results go into a global read-mostly cache. Static conversions also keep the itab in a
  function-local `static`, so the lookup happens once per conversion site.
- **Signatures with C/C++ types.** `go/types` treats every C type as `invalid`, and two invalid
  types compare equal, so it would accept an interface method `Get() C.int` implemented by
  `Get() C.double` (found in the audit). The itab would then call the thunk with the wrong
  signature. So each static conversion has a `static_assert` that the thunk type equals the slot
  type, and the runtime compares signatures by their source spelling when opaque parts are
  involved.
- Calling `s.Area()` = load slot 0 of the itab, cast it to the known signature, call it.
- **Header order.** An interface's wrapper methods are *declared* in its struct and *defined*
  `inline` after all types. `type Shape interface{ Bounds() Rect }` together with
  `type Rect struct{ Owner Shape }` is legal Go, and a body inside the struct would need `Rect`
  to be complete too early (both compilers reject it).

### 2.4 Assertions, type switches, comparisons (Go's exact messages)

| Go | Runtime behavior |
|---|---|
| `x.(T)` (concrete) | compare `x.type` with T's descriptor; on failure panic `interface conversion: interface {} is string, not int` |
| `x.(T)` on a nil interface | panic `interface conversion: interface {} is nil, not int` |
| `v, ok := x.(T)` | same, no panic |
| `x.(I)` (interface) | `getitab(I, x.type)`; null → panic `interface conversion: main.Rect is not main.Stringer: missing method String`; nil → `interface conversion: interface is nil, not main.Stringer` |
| `switch v := x.(type)` | a chain of the checks above; `case A, B:` keeps `v` as the interface; `case nil:` checks `type == null` |
| `a == b` (interfaces) | same type and `type->equal(a, b)`; uncomparable → panic `runtime error: comparing uncomparable type []int` |
| `x == nil` | `type == null`. A typed nil pointer inside an interface is **not** nil, the same as Go's well-known gotcha |
| `map[any]V` keys | `type->hash`, panic if unhashable |

`any` is `gp::any`: an interface with no methods and no itab.

### 2.5 `error` and the errors model

- `error` is the predeclared interface `interface{ Error() string }` → `gp::error`.
- `errors.New` returns a `*errorString` exactly like Go, so two `errors.New("x")` values are
  different errors.
- `errors.Is`, `errors.Unwrap`, `errors.Join` are **written in gocpp** (`stdlib/errors/*.gp`).
  `errors.As` needs "is this dynamic type assignable to that target type", which the type
  descriptors answer natively.
- **Built in 5.7 (2026-10-18), where it differs from the text above:** `stdlib/errors` is Go's `src/errors` ported
  almost verbatim (`New`, `Is`, `As`, `Unwrap`, `Join`, `ErrUnsupported`; the interface-based algorithm is Go's own),
  and `Is`, `Unwrap` and `Join` need no native code. What Go does with `internal/reflectlite` is three **native
  functions** in the runtime over the type descriptors (`gp::errors::comparable`, `check_target`, `assign`), which
  the `.gp` source reaches through the new `//gocpp:native <C++ name>` marker on a bodyless function (ADR-027;
  a bodyless function without it is still a Phase 01 stub). `assign` stores into `*target` with a new descriptor
  hook, `type_desc::gp_store`. `Join` copies the text once more (`string(b)` instead of `unsafe.String`). The
  panics of `As` are Go's three texts. Details: [docs/spec/cpp-mapping.md](../../../docs/spec/cpp-mapping.md#the-errors-package-and-native-functions-step-57).
- `fmt.Errorf` with `%w` (basic version here, complete in Phase 09). It is step 5.11's: it needs only the exported
  `errors` API and its own unexported wrapper types, as in Go's source.
- Less boilerplate comes later: Phase 10 adds `!` (must) on functions and calls, which returns
  the error for you. It uses the single-exit function shape, so defers still run on the error
  path.

### 2.6 Struct embedding

```go
type Base struct{ ID int }

func (b *Base) Touch() { b.ID++ }

type User struct {
	Base // embedded
	Name string
}

func main() {
	u := &User{Name: "ann"}
	u.Touch()         // promoted pointer method
	fmt.Println(u.ID) // promoted field
}
```

- **Promoted fields/methods**: `go/types` records the full path in `types.Info.Selections`
  (`u.ID` really is `u.Base.ID`). Codegen writes the full path: `u->Base.ID`, and
  `Base::Touch(gp::field_ptr(u, &User::Base))` with an aliasing pointer that shares ownership with
  `u`. Embedded pointers (`*Base`) get a nil check on the path.
- **The field is declared with a fully qualified type**: `::gpkg_main::Base Base;`. GCC rejects
  `Base Base;` (the field name changes the meaning of the type name), and both compilers reject a
  later use of the type name after such a field. Go code like `Config Config` is common.
- **Forwarding members**: the outer struct gets a small forwarding member function for every
  promoted method. Generic code (Phase 07) and C++ users call `u.Touch()` without knowing the path,
  and the method tables get thunks for promoted methods, so `User` satisfies interfaces through
  `Base`'s methods.
- **No C++ inheritance** is used for embedding. Go embedding is not subtyping, and we keep it
  that way.
- **Built in 5.8 (2026-10-17), where it differs from the text above:** the fields, the promoted
  fields and methods and the aliasing pointers existed since Phase 03 (steps 3.8, 3.10). 5.8 added
  (branch `job/05-5.8`) the forwarding members, written like declared methods (a `const` member when the
  method is in the method set of `T`, else a `static` one with the owning pointer, so that one thunk
  shape serves declared and promoted methods); they are written only for the structs whose method
  tables need them (`tables()` marks them; code size: `lower.Options.AllForwarders` writes all of them, for the
  tests that call them from C++). The promoted methods are in the method tables and itabs of the outer
  type (`gp::method_of_aware` for a method that calls `recover`, and for one promoted from an embedded
  interface, whose dynamic method is not known; the ticket goes on to the inner `icall`), and an anonymous
  struct with embedded fields has its tables in `td_struct`. A promoted value method through a nil `*T`
  in an interface is an ordinary nil dereference (Go's wrapper reaches the embedded field through the
  pointer: only a declared value method of `*T` has panicwrap's "value method ... called using nil"
  message). A struct that embeds exactly one C/C++ type gets its members promoted (`mv.push_back(1)`),
  earlier than Phase 12; several embedded C/C++ types, and a Go member that an embedded C/C++ type
  could hide, stay a Phase 12 gate. Details: [docs/spec/cpp-mapping.md](../../../docs/spec/cpp-mapping.md#embedding).
- Interfaces embedding interfaces (`ReadWriter`) are just method-set unions computed by
  `go/types`.

### 2.7 panic / recover / defer (full version)

This phase extends the Phase 03 single-exit shape:

```cpp
R f(Args... args) {
    gp::defer_ctx gp_dc = gp::take_defer_ctx();   // only a function that contains recover()
    gp::result_slot<R> gp_r0;         // results; works for C++ types without copy/default ctor
    gp::defer_frame gp_df;
    try {
        /* body */                    // `return x` → { gp_r0 = x; goto gp_ret0; } (one exit per return)
    } catch (...) {
        gp_df.gp_on_exception();      // a panic, Goexit or a C++ exception (a CppException panic);
    }                                 // a forced unwind (glibc) is thrown again at once
    gp_df.gp_run(); return gp_r0.gp_take();   // run defers LIFO; if one recovered, the panic is gone,
    gp_ret0:; gp_df.gp_run(); return gp_r0.gp_take();  // otherwise gp_run throws again after ALL defers ran
}
```

(Step 5.10 built this: the catch is `catch (...)`, the members are `gp_`-prefixed, and the
`result_slot` and the one exit per `return` are Phase 03 and 04's.)

- `panic(v)` → `throw gp::panic_error{...}` whose value is a `gp::any` (`gp::panic(v)`). Zero
  cost when nothing is thrown (table-based unwinding). Functions without `defer` have no frame
  and no `try`.
- **`recover()` follows Go's rule exactly.** In Go, `recover` stops a panic only when it is
  called directly by a deferred function. The deferred function may be reached any way: a func
  value, a method value, an interface method, or a closure returned by another call
  (`defer mk()()`, which is what Phase 10's on-top macros produce). The audit checked all four in
  Go 1.25. So every function that contains `recover()` is *aware* (step 5.10; built with a
  **ticket** instead of the two entry points that this section first planned, which would have
  made `gp::func` 32 bytes): its first statement takes the ticket that `gp_run` leaves for the
  call it makes, only while it runs its calls for an unrecovered panic, and `recover()` is
  `gp::recover(gp_dc)`. The emitter says at the `defer` statement when the callee is aware
  (`gp_push_aware`, `gp_push_call_aware`, `gp_push_call_icall` for an interface method); a func
  value is aware when its code is in the runtime's registry (`gp::aware`, `gp::func_of_aware`),
  and an interface method when its thunk is (`gp::method_of_aware`, which `gp::icall` checks
  before it calls). A normal call finds no ticket, where `recover()` returns nil. Indirect
  calls (`defer helper()` where `helper` calls `inner()`, which calls `recover`) correctly
  return nil, as in Go (docs/spec/cpp-mapping.md "Defer"; ADR-011 notes).
- **Runtime errors** (index out of range, nil map, …) panic with values that implement
  `runtime.Error`, so `recover()` returns an `error`, as in Go.
- **C++ exceptions** that reach a function with defers become panics whose value is a
  `gp::CppException` (Go type `runtime.CppException`: an `error`; `Error()` returns `C++
  exception <type>: <what()>`), so they are recoverable. Phase 04's fatal behavior stays for a
  stack without a function that has a `defer` (nothing to run, nothing to recover with).
- **`runtime.Goexit`** (Phase 08) uses its own `gp::goexit_signal`: defers run, `recover()`
  returns nil during it, and it is always re-thrown. `t.Fatal` in `gocpp test` depends on this.
- **Exact panic output** (verified with Go 1.25): an unrecovered panic prints
  `panic: <value>` and exits with code 2; a re-panicked value prints `panic: first [recovered,
  repanicked]`; a panic inside a deferred call prints `panic: first` then a tab and
  `panic: second`; `panic(nil)` prints `panic: panic called with nil argument` (a
  `*runtime.PanicNilError`); `panic(Rect{1, 2})` prints `panic: (main.Rect) 0x…` with an
  address, so the differential harness masks addresses.
- An unrecovered panic at the top of a goroutine prints the chain plus a trace and exits with
  code 2.

### 2.8 `fmt` on top of `any`

The real `fmt.Print*` family replaces the `fmtshim`. It takes `...any` and uses type
descriptors. `error` values print `Error()`, `fmt.Stringer` values print `String()` (found by a
dynamic itab lookup), structs print `{1 2}`, `%+v` prints `{X:1 Y:2}`, and `%T` prints the Go
type name. C/C++ values print through their foreign descriptors. Phase 09 completes all verbs and
flags.

## 3. Steps

1. **5.1 Type descriptors chosen by the Go type**: generated objects for named types, runtime
   objects for predeclared types, interned composite descriptors, foreign descriptors for C/C++
   types; method tables with package-qualified unexported names.
2. **5.2 Interface values** (`gp::iface`, boxing rules, generated interface structs, `gp::icall`,
   wrapper methods defined after all types).
3. **5.3 itab building + global cache + per-site static cache + signature checks** (`static_assert`
   at static conversions, spelling check for opaque parts).
4. **5.4 Conversions**: concrete → interface, interface → interface. (5.10 obligation, here and for
   5.8/5.9: a method that contains `recover()` (`g.awareFunc`) must be written into EVERY method
   table with `gp::method_of_aware`, not `gp::method_of`: T's table, *T's, and the forwarding
   thunks of promoted methods in the tables of the types that embed it; otherwise `defer i.M()`
   gives `M` no ticket and its `recover()` returns nil where Go recovers. `gp::icall` reads the
   flag from the itab. Test: `recover_iface_method`, `recover_iface_embedded`.)
5. **5.5 Assertions + type switches** with Go's exact panic messages.
6. **5.6 Interface `==`, nil rules, hashing** (`map[any]V`, `map[error]V`).
7. **5.7 `any`, `error`, `errors` package** (`New`, `Is`, `As`, `Unwrap`, `Join`; written in gocpp
   where possible).
8. **5.8 Embedding**: selection paths, aliasing pointers into embedded fields, fully qualified field
   types, forwarding members, forwarding thunks, embedded pointers and interfaces.
9. **5.9 Method values and expressions on interfaces.** `f := i.M` binds a copy of the interface value
   where it is evaluated (`gp::method_recv`: a nil interface panics there, also through an embedded
   interface field); `I.M` is a lambda over the interface's wrapper; both are `gp::aware` func values for
   `recover` (docs/spec/cpp-mapping.md "Interfaces").
10. **5.10 panic/recover**: aware functions (a ticket instead of two entry points, §2.7),
    `gp::result_slot`, `goexit_signal` and `__forced_unwind` re-thrown, nested panics,
    `runtime.Error` values, C++ exception conversion, exact output.
11. **5.11 `fmt` on `any`** (basic verbs; remove the `fmtshim`). It must also cover what the shim
    gets wrong for a panicking `String`/`Error` method: Go prints the panic value with `%v`
    (`%!v(PANIC=String method: 1.5)`), the shim prints the `print` format (`+1.500000e+000`).
12. **5.12 Lift the Phase 01 gates** for interfaces, embedding and `recover`; update the feature
    matrix.
13. **5.13 Tests**: ≥ 80 run tests + differential tests vs Go (this whole phase is pure Go
    semantics, so differential testing is very effective): the nil-interface gotcha; `int` vs
    `int64` and channel directions in type switches and `%T`; `recover` through a func value, an
    interface method, a method value and `defer mk()()`; indirect `recover` returning nil;
    re-panic; panics in defers; `panic(nil)`; a C++ exception caught by `recover`.

## 4. Deliverables

- Runtime: type descriptors, interfaces, itab cache, panic machinery v2, `gp::result_slot`
- Codegen: interface structs, thunks, conversions, assertions, type switches, embedding, two-entry
  functions for `recover`
- `stdlib/errors`, basic `fmt`
- Target program `shapes` passing

## 5. Exit criteria

- `shapes` passes on both compilers.
- ≥ 80 interface/error/panic run tests and the matching differential tests are green.
- Panic output (first lines, with addresses masked) and exit codes match Go in every differential test.
- Type switches and `%T` tell `int`/`int64` and the channel directions apart.
- A C++ exception thrown from interop code is caught by `recover()` in a test.
- Sanitizers clean.

## 6. Risks

| Risk | Mitigation |
|---|---|
| itab lookup cost | Per-site static cache + global cache; benchmark in Phase 15 |
| Descriptor identity across shared libraries | Interned descriptors + one gocpp runtime per process; Phase 06's c-shared mode hides runtime symbols |
| Code size of thunks/descriptors | Only generated for types that flow into interfaces |
| `recover` semantics are subtle | Two-entry design matches Go's rule; differential tests for every way of reaching a deferred function |
| C++ types in method signatures defeat `go/types` checks | `static_assert` on thunk types; Phase 12 typed interop makes them real types |
