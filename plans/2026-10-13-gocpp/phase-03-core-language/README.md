# Phase 03: Core Language → C++ (codegen + runtime)

> Make the everyday Go-style core work completely: strings, slices, arrays, maps, structs,
> named types, pointers (with reference counting), methods, closures, `defer`, runtime checks
> and package-level initialization. After this phase you can write real programs in gocpp.
> Interfaces, generics, goroutines and C++ interop come later.

| | |
|---|---|
| Depends on | Phase 02 |
| Unlocks | Phase 04 |
| Milestone | **M2: real (non-interop) programs run** |
| Size | large (about 6–7 weeks) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

Phase 02 proved the pipeline. This phase fills in the language. Two things make it the heart of
the project:

1. **The memory model.** Go code uses `&T{}`, returns pointers and keeps slices of
   pointers without thinking about lifetimes. gocpp keeps that comfort without a GC: every heap
   object is **reference counted**, and the compiler decides automatically which local
   variables must live on the heap ("boxing").
2. **Go semantics that C++ doesn't have:** evaluation order, value receivers and parameters as
   real copies, closures that share variables, `defer` at function scope, zero values, and
   runtime panics instead of undefined behavior.

A **lowering pass** is added in front of the emitter, because many of these need
statement-level rewrites (adding temporaries, wrapping blocks):

```
typed AST ──► internal/lower (analysis + rewrites) ──► cppir (small C++ syntax tree) ──► printer
```

`cppir` is a small tree that maps 1:1 to C++ syntax (Namespace, Struct, Func, Block, If, For,
Expr, ...). Lowering builds it, so "insert this temporary before the current statement" is
easy. The printer handles indentation and `#line`. The Phase 02 emitter is refactored into this
shape at the start of the phase (step 3.1).

## 2. How it works

### 2.1 One ownership system for everything (v1)

Every heap block (boxed variables, `new(T)`, slice backing arrays, string bytes, map tables,
closure cells) is owned through **`std::shared_ptr`**. Because there is only one system, a
pointer *into* any block (`&slice[3]`, `&obj.field`) can share ownership with that block through
`std::shared_ptr`'s **aliasing constructor**, so interior pointers are memory safe too.

| gocpp | C++ layout (v1) | Size |
|---|---|---|
| `*T` | `gp::ptr<T>` = `std::shared_ptr<T>` + nil checks | 16 B |
| `unsafe.Pointer` | `gp::ptr<void>` (keeps a gocpp object alive, like Go; Phase 04) | 16 B |
| `string` | `gp::string` = {owner `shared_ptr` (null for literals), `const char*` data, `int64_t` len} | 32 B |
| `[]T` | `gp::slice<T>` = {owner, `T*` data, len, cap} | 40 B |
| `map[K]V` | `gp::map<K,V>` = `shared_ptr<map_impl>` | 16 B |

These sizes are what `gocppSizes` (Phase 01) reports to `go/types`, so `unsafe.Sizeof` is correct
(they differ from Go's, see divergence D15). The runtime header holds `static_assert`s that keep
both sides in sync. Phase 15 benchmarks an intrusive refcount that would shrink string/slice to
24/32 B. All generated code goes through the `gp::` API, so that change never touches codegen.

**Runtime types have no plain-named members.** Go methods can be called `get`, `len` or `reset`,
and a named type like `type IDs []int` is built on a runtime type (2.4). So `gp::ptr`,
`gp::string`, `gp::slice`, `gp::map` and friends expose only operators and `gp_`-prefixed
members; everything else is a free function (`gp::len(s)`). A user's method `get` can then never
clash with a runtime member.

### 2.2 Pointers and boxing

```go
func NewCounter() *Counter {
	c := Counter{step: 1} // address taken below → c is boxed
	return &c
}
```
```cpp
gp::ptr<Counter> NewCounter() {
    gp::ptr<Counter> c = gp::new_<Counter>(Counter{.step = 1});  // boxed: lives on the heap
    return c;                                                     // &c == the box itself
}
```

**Boxing rule (decided per variable by a pre-pass over the function).** A local variable,
parameter, receiver copy or named result is boxed if the function:
- takes its address, or the address of anything inside it (`&x`, `&x.f`, `&x.arr[i]`);
- calls a pointer method on it or inside it (`x.Inc()`, `x.f.Inc()`; those are `(&x).Inc()`);
- slices an array that is (or is inside) it (`arr[:]`, `x.arr[:]`);
- or captures it in a closure **and** changes it anywhere. Changing includes field and element
  writes and `++`/op-assign (`x.f = 5`, `arr[1]++`), not only `x = ...`.

Uses of a boxed `x` become `(*x)`. A boxed parameter is copied into its box at function entry.
This is a simpler, more conservative version of Go's escape analysis; Phase 15 adds the real
analysis that keeps non-escaping variables on the stack.

**Taking the address of something inside another object** shares ownership with that object:

| Expression | Owner | C++ |
|---|---|---|
| `&x` (boxed local) | its box | `x` |
| `&p.f` (`p` is `*T`) | `p` | `gp::ptr<F>(p, &p->f)` (aliasing) |
| `&s[i]` (`s` is a slice) | the slice's backing array | `gp::ptr<T>(s.gp_owner(), &s[i])` |
| `&arr[i]`, `&x.a.b` (inside a boxed local) | that box | aliasing with the box |
| `&globalVar` | a **static owner** that is never freed | `gp::ptr` sharing that owner (so `weak.Make(&global)` never expires) |

`new(T)` → `gp::new_<T>()` (zero value). `&T{...}` → `gp::new_<T>(T{...})`. Dereferencing a
nil `gp::ptr` panics with Go's exact message: `runtime error: invalid memory address or nil
pointer dereference`.

**Reference cycles** (A points to B, B points to A) are never freed. That's the price of no GC.
gocpp offers Go 1.24's `weak` package (`weak.Make(p)`, `wp.Value()`) on top of `std::weak_ptr`.
`weak.Pointer[T]` is generic, but generics only arrive in Phase 07, so `weak` is a compiler
intrinsic that is exempt from the generics gate. `gocpp vet` (Phase 13) warns about obvious
cycles such as self-recursive closures.

### 2.3 Strings, slices, arrays, maps

**Strings**: immutable bytes. Literals point at static storage (no allocation). Substrings
`s[a:b]` share the owner. Concatenation allocates. `s[i]` is bounds checked and gives a `byte`.
`for i, r := range s` decodes UTF-8 exactly like Go (invalid bytes → `U+FFFD`, step 1).
Conversions `[]byte(s)`, `string(b)`, `[]rune(s)`, `string(runes)` and `string(r)` copy.

**Slices**: `{owner, data, len, cap}`. `append` grows with Go's formula (double below 256
elements, then about 1.25×, rounded to allocation classes) for similar performance. But the
exact `cap()` after `append` **cannot** match Go: in Go 1.25 it depends on Go's element size
(`gp::string` is 32 B, Go's string 16 B), on whether elements hold pointers, and on escape
analysis (the audit measured three different growth sequences). So gocpp promises only
`cap >= len`, and differential tests never print `cap` directly. Also: `s[a:b:c]`, `copy`
(overlap safe), `make([]T, n, c)` (zeroed), nil slices, `clear`, `append([]byte, string...)`,
`[N]T(s)` (Go 1.20 slice→array conversion with its length check).

```go
s := make([]int, 0, 4)
s = append(s, 1, 2)
t := s[0:1]
```
```cpp
gp::slice<int64_t> s = gp::make_slice<int64_t>(0, 4);
s = gp::append(s, 1, 2);
gp::slice<int64_t> t = gp::sub(s, 0, 1);  // shares the backing array; bounds checked
```

**Arrays** `[N]T` → `gp::array<T, N>` (wraps `std::array`, value semantics, checked index,
`==` when the elements are comparable).

**Range loops follow Go 1.22+:** `for i, v := range x` uses hidden counters, so assigning to `i`
or `v` in the body does not change the iteration (as in Phase 02's `range` over int). The range
expression is evaluated once. **Ranging over an array iterates over a copy** when the body could
modify the array, so `for _, v := range arr { arr[1] = 100 }` still sees the old values, as in
Go (a slice would see the new ones).

**Maps** are references. Reads never insert (unlike C++ `operator[]`):

| Go | C++ |
|---|---|
| `v := m[k]` | `V v = gp::get(m, k);` (zero value if missing) |
| `m[k] = v` | `gp::set(m, k, v);` (panics on a nil map: `assignment to entry in nil map`) |
| `m[k] += 1`, `m[k]++` | `gp::ref(m, k) += 1;` (insert-or-get) |
| `v, ok := m[k]` | `auto gp_t = gp::lookup(m, k); V v = std::get<0>(gp_t); bool ok = std::get<1>(gp_t);` (the Phase 02 multi-value rule) |
| `delete(m, k)`, `len(m)`, `clear(m)` | `gp::erase(m, k)`, `gp::len(m)`, `gp::clear(m)` |
| `for k, v := range m` | iterate a snapshot of the keys, skip keys deleted meanwhile (Go allows insert/delete during range) |

v1 wraps `std::unordered_map` (simple and correct). Hashing is generated for strings, arrays and
comparable structs, and floats follow Go's rules (`NaN != NaN`, `+0 == -0`). Phase 15 swaps in a
Swiss table (Go 1.24 itself uses one).

### 2.4 Structs and named types

```go
type Point struct{ X, Y int }
type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }
```
```cpp
struct Point {
    int64_t X = 0;
    int64_t Y = 0;
    bool operator==(const Point&) const = default;   // only when comparable
    static constexpr auto gp_fields = gp::fields(&Point::X, "X", &Point::Y, "Y"); // for fmt %v/%+v
};
struct Celsius : gp::newtype<Celsius, double> {      // a distinct type with double's operators
    using newtype::newtype;
    gp::string String() const;
};
```

- Zero values come from default member initializers and `T{}`.
- `Point{Y: 2, X: 1}` → `Point{.X = 1, .Y = 2}`. C++ needs declaration order, so if the values
  have side effects, lowering first stores them in temporaries in *source* order.
- Anonymous struct types get one canonical generated name per distinct type
  (`gp_anon_3f2a…`), because Go type identity for them is structural. Go treats the same
  anonymous struct as one type across packages too, so the canonical definition lives in a shared
  generated header with one include guard per type (Phase 06).
- **Local named types** (a `type` inside a function) are moved to namespace scope with a unique
  name; inside generic functions they become templates. C++ local classes can't have the static
  members this design needs (`gp_fields`), and both compilers reject that (checked in the audit).
- Named non-struct types (`Celsius`, `type IDs []int`) become **strong typedefs** through
  `gp::newtype` (CRTP), so they have their own methods and are distinct C++ types. Go also
  converts **implicitly** in some places where the types are not identical: a `[]int` assigned to
  an `IDs` variable, a func literal passed as a `Handler`, a map literal used as a `Set`.
  `go/types` records no conversion node there, so codegen inserts a conversion **wherever source
  and target types are not identical** (assignment, argument, return value, composite-literal
  element).
- A `switch` on a newtype (an enum like `type Color int`) switches on the underlying value with
  folded case constants, because C++ rejects a class type in a `switch`.
- **Ordering:** forward-declare all structs, emit definitions in dependency order (only
  *by-value* fields create dependencies), then functions. Runtime templates (`gp::ptr`,
  `gp::slice`, `gp::map`) accept incomplete types, so recursive types like
  `type Node struct{ next *Node; kids []Node }` work. That gets tested early.

### 2.5 Methods

| Go | C++ |
|---|---|
| `func (v T) M(a int) int` | member `int64_t M(int64_t a) const;` starting with `T v = *this;`. Go gives a value receiver **its own copy**. A `const T&` would see writes made through other pointers while the method runs, and could dangle (the audit showed both, including an ASan use-after-free). A reference is used only when the body provably can't write memory or call unknown code |
| `func (p *T) M()` | `static void M(gp::ptr<T> p);` (needs an owning pointer that can escape) |
| `p.M()` on a value method, `p` a pointer | `p->M()` (auto-deref with nil check). If `p` is not a local of the caller (a global, field or element), the caller first copies it into a local, so the object can't be freed while the method runs |
| `x.PM()` on a pointer method, `x` addressable | `T::PM(<address of x>)`, using the owner table in 2.2 |
| `f := x.M` (method value) | lambda that copies the receiver *now* (Go evaluates the receiver at binding time) |
| `T.M`, `(*T).PM` (method expressions) | lambdas taking the receiver as the first parameter |

Phase 15's escape analysis can turn non-escaping pointer methods into plain member functions
using `this`.

### 2.6 Functions, func values, closures

- **Parameters are passed by value** (a real copy, as in Go). Phase 15 may pass large ARC values
  by `const&`, but only when the caller's argument is a local that can't change during the call
  (otherwise the caller passes a fresh copy). A plain `const&` rule is unsafe: if the callee
  changes the global that was passed, the parameter changes too, or the object is freed (seen in
  the audit).
- `func(int) string` → `gp::func<gp::string(int64_t)>` (a `std::function` wrapper that panics
  when you call nil and supports `== nil`).
- Variadic `func f(xs ...int)` takes a `gp::slice<int64_t>`. `f(1, 2)` → `f({1, 2})`;
  `f(s...)` → `f(s)`.
- **Closures** become C++ lambdas. Capture analysis per variable:
  - never changed after its initialization (no assignment, no field/element write, no
    `++`/op-assign) and address never taken → **captured by value** (cheap, identical behavior);
  - otherwise → **boxed** (2.2) and the lambda captures the box, so closure and function see
    the same variable, as in Go.

```go
func counter() func() int {
	n := 0
	return func() int { n++; return n }
}
```
```cpp
gp::func<int64_t()> counter() {
    gp::ptr<int64_t> n = gp::new_<int64_t>(0);              // boxed: captured + modified
    return [n]() -> int64_t { (*n)++; return (*n); };
}
```

- **Go 1.22 loop variables:** each iteration has its own variable. A plain C++ `for` behaves the
  same *unless* the loop variable is boxed (captured or address taken). Only then does lowering
  emit the per-iteration copy: a new box per iteration, with the post statement applied to the
  copy (exactly the spec's rule).

### 2.7 Evaluation order

Phase 02 already keeps Go's left-to-right order for calls in simple expressions (g++ and clang++
really differ there). This phase extends the pass to every construct: composite literals,
index and slice expressions, method calls, `&&`/`||` (whose right side, if it needs temporaries,
becomes a small `if` chain so short-circuiting is kept), loop conditions and post statements,
`else if` conditions and `case` expressions (which can't hoist their temporaries above the whole
statement and get their own shapes).

```go
func main() {
	fmt.Println(next(), next()) // Go guarantees 1 then 2
}
```
```cpp
auto gp_t1 = next();
gp::fmtshim::Println(gp_t1, next());
```

Assignments follow Go's two-phase rule: index operands and pointer operands on the left, then
the right side, are evaluated first, and the stores happen afterwards, left to right.

### 2.8 `defer` (without `recover`)

A function that contains `defer` gets the **single-exit shape** right away (the same shape Phase
05 later extends with `recover`):

```cpp
R f(Args... args) {
    R gp_r0{};                        // results (named results are these variables)
    gp::defer_frame gp_df;
    try {
        /* body */                    // `return x` → { gp_r0 = x; goto gp_exit; }
    } catch (...) {
        gp_df.on_panic(std::current_exception());
    }
gp_exit:
    gp_df.run();                      // run defers LIFO, then re-throw a pending panic
    return gp_r0;
}
```

- `defer f(a, b)` evaluates `f`'s arguments **now** (Go rule) and pushes a closure; defers inside
  loops accumulate until function exit, as in Go.
- Defers run **during a panic** too: the panic is a C++ exception (Phase 02), caught here, and
  re-thrown after all defers ran. (Running defers from a destructor would not work: an exception
  nobody catches doesn't unwind, and a panicking defer inside a destructor calls
  `std::terminate`. Both were seen in the audit.)
- **A deferred call that panics** doesn't stop the remaining defers; they still run, as in Go
  ("second defer still runs").
- **Named results changed by a defer** (`defer func() { err = wrap(err) }()`) work: the closure
  captures and changes `err`, so the capture rule boxes it, and the epilogue returns the boxed
  value.
- C++ destructors of locals declared in the body run when their block ends, which is before the
  function's defers run. That only matters for C/C++ objects (Phase 04 boxes those when a defer
  uses them).

`recover()` and C++-exception conversion are Phase 05 work and phase-gated until then.

### 2.9 Runtime checks and panics (Go's exact messages)

| Check | Message |
|---|---|
| index | `runtime error: index out of range [5] with length 3` |
| slice bounds | `runtime error: slice bounds out of range [:5] with capacity 3` |
| nil pointer | `runtime error: invalid memory address or nil pointer dereference` |
| nil map write | `assignment to entry in nil map` |
| division | `runtime error: integer divide by zero` |
| shift | `runtime error: negative shift amount` |
| make | `runtime error: makeslice: len out of range` |
| slice → array | `runtime error: cannot convert slice with length 2 to array or pointer to array with length 3` |

An unrecovered panic prints `panic: <msg>` plus `goroutine 1 [running]:` and a stack trace, then
exits with **code 2** like Go. The trace uses `backtrace()` (or libbacktrace), because C++23's
`<stacktrace>` is not available under `-std=c++20` (checked). `gocpp build -nochecks` compiles out
the bounds and nil *checks* only. Division by zero and negative shift amounts still panic,
because without those checks C++ has undefined behavior (and they cost little), and Go's defined
results always stay: shifts by ≥ width and `MIN / -1` still give Go's answers.

### 2.10 Builtins, printing and package initialization

- Builtins: `len cap append copy delete make new panic print println min max clear complex real imag`
  (`panic` takes basic values here; any value comes in Phase 05).
- **The `fmt` shim grows**, since this phase's tests print composite values: slices and arrays
  `[1 2]`, maps `map[a:1 b:2]` (keys sorted, as Go does), structs `{1 2}` via `gp_fields`,
  pointers to structs `&{1 2}`, and a `String() string` method when the value's method set has
  one (detected at C++ compile time, since interfaces come in Phase 05).
- Package-level variables are initialized in **`types.Info.InitOrder`** (go/types computes Go's
  dependency order for us) inside a generated `gp_var_init()`. Variables with constant values are
  `constinit` static initializers. Several `init()` functions per file or package are renamed
  `gp_init_1…n` and run afterwards, in source order, from `gp_init_funcs()`. Phase 06 adds the
  order across packages (`gp_init_all()`).

## 3. Steps

1. **3.1 Refactor**: introduce `internal/lower` + `cppir` + printer; port the Phase 02 subset; all
   Phase 02 tests stay green.
2. **3.2 Runtime ownership core**: `gp::ptr`, `gp::new_`, aliasing helpers, nil checks, static
   owners for globals, the `weak` intrinsic, and the "no plain-named members" rule for all runtime types.
3. **3.3 Boxing analysis** (the full rule in §2.2, incl. parameters, receivers, named results) +
   owner table + tests (address of locals, fields, elements, globals).
4. **3.4 Strings** (full: runtime + codegen + UTF-8 range + conversions).
5. **3.5 Slices** (growth formula, sub-slicing, copy, append forms, slice→array; `cap >= len` only).
6. **3.6 Arrays + range rules** (copy on `range arr`, hidden counters).
7. **3.7 Maps** (get/set/ref/lookup/erase, snapshot range, hashing incl. structs and floats).
8. **3.8 Structs**: layout, zero values, `==`, hash, field info, literals in source order,
   anonymous structs (shared canonical header), local types hoisted, dependency ordering,
   recursive types.
9. **3.9 Named non-struct types** via `gp::newtype`, implicit conversions where types aren't
   identical, `switch` on the underlying value.
10. **3.10 Methods** (value receivers copy, safe pointer receivers, auto-deref with a held
    reference, method values and expressions).
11. **3.11 Func values, variadics, closures** + capture analysis + Go 1.22 loop variables.
12. **3.12 Evaluation-order lowering for every construct** + assignment two-phase rule.
13. **3.13 `defer`** with the single-exit shape (panics run defers; panicking defers; named results).
14. **3.14 Runtime checks + panic output** (messages, exit code 2, `backtrace()` trace, `-nochecks`).
15. **3.15 Builtins** (remaining ones) and the **`fmt` shim for composite values**.
16. **3.16 Package-level vars + `init()`** via `InitOrder`.
17. **3.17 `gocppSizes` + `static_assert` sync.**
18. **3.18 Tests**: ≥ 150 run tests, ≥ 60 differential tests vs Go, ASan/UBSan/LSan CI job.

## 4. Deliverables

- `internal/lower`, `cppir`, full core codegen
- Runtime: `gp::ptr`, `gp::string`, `gp::slice`, `gp::array`, `gp::map`, `gp::func`,
  `gp::newtype`, `gp::defer_frame`, panic machinery, helpers, `weak`
- Target programs `structs`, `wordfreq-core`, `closures` passing

## 5. Exit criteria

- Target programs for Phase 03 pass on GCC and Clang.
- ≥ 150 run tests and ≥ 60 differential tests (vs real `go run`) pass on both compilers,
  including the audit cases: value receivers under aliasing, `range` over arrays, implicit
  named-type conversions, captured field writes, panicking defers.
- The full suite is clean under ASan + UBSan. LSan reports no leaks except in tests that build
  cycles on purpose (those are marked).
- A recursive data structure test (tree + linked list + graph with `weak.Pointer`) passes.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Atomic refcount traffic slows pointer-heavy code | Measure first; Phase 15 escape analysis, safe `const&` (only for unchangeable caller locals) and intrusive RC |
| Copying value receivers costs time for big structs | Use a reference only when the body provably can't write memory or call unknown code; benchmark |
| Over-boxing (heap allocs for locals) | Conservative now, measured; Phase 15 escape analysis |
| Self-recursive closures leak (cycle through the box) | Documented; `gocpp vet` warning; prefer named functions |
| Evaluation-order rewrite bugs | Differential tests with side-effect-heavy programs; Phase 15 random program fuzzing |
| Recursive types vs C++ incomplete types | Runtime templates designed for incomplete `T`; tested in 3.8 |
