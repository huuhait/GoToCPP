# Phase 07: Generics

> Go type parameters become C++ templates. The C++ compiler instantiates them; `go/types` has
> already checked constraints and inferred type arguments, so templates are always called
> with explicit, correct arguments. This phase also adds range-over-func iterators
> (`iter.Seq`) and ports Go's own generic packages (`slices`, `maps`, `cmp`, `iter`).

| | |
|---|---|
| Depends on | Phase 05 (type descriptors for instances) and Phase 06 (templates live in headers) |
| Unlocks | Phase 08 (generic `atomic.Pointer[T]`, `sync.OnceValue`), Phase 09 (generic stdlib) |
| Size | medium (about 3–4 weeks) |
| Progress | [STATE.md](STATE.md) |

> **Update 2026-10-18 (steps 7.1 and 7.2, [ADR-030](../../../docs/adr/030-generic-instances.md)):** a generic
> **function** is not a C++ template. It is lowered once for each instance (list of type arguments)
> that the program names, by the package that names it, as a function written with those types, in
> that package's unit (`internal/lower/generics.go`, `subst.go`). Reasons: `int` and `int64` are one C++
> type (so templates need the hidden descriptor arguments of §2.4), and every operation of the lowering
> depends on the Go type, so each of §2.2's helpers would be a second place where generic code can differ
> from the same code written out. What §2.2, §2.3 (`gp_call_<Method>`) and the hidden descriptor
> arguments of §2.4 describe is therefore **not built for functions**: steps 7.4 and 7.5 are covered by
> the substitution (their tests are in `tests/run/generics_*`). C/C++ type arguments (§2.4) are
> function templates over the C++ types, deduced by the C++ compiler (ADR-030, point 3). For step 7.3
> (generic types) the same design means one struct (and method set, and descriptor with Go's exact name
> `main.Pair[string,[]int]`) for each instance, from the instance's substituted types; the manager
> decides before 7.3 starts. Everything else in this README stands.

---

## 1. Idea

C++ templates are a natural target for Go generics. Both are compile-time generic code, and C++
creates one specialized copy per type ("monomorphization"). Go's semantic rules are the hard
part, and most are already handled: `go/types` validates constraints (`any`, `comparable`,
`cmp.Ordered`, `~int | ~float64`) and infers the type arguments at every call. gocpp has to
**emit templates, always pass explicit type arguments** (which avoids C++'s deduction rules), and
make sure the template body still follows **Go's** rules for arithmetic and method calls, which
C++ templates do not do by themselves.

## 2. How it works

### 2.1 Example

```go
func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func main() {
	names := Map([]int{1, 2}, strconv.Itoa) // T=int, U=string inferred by go/types
	var st Stack[string]
	st.Push("a")
	fmt.Println(names, st.items)
}
```

```cpp
// in pkg.hpp: templates must be visible to every translation unit that uses them
template <typename T, typename U>
gp::slice<U> Map(gp::slice<T> xs, gp::func<U(T)> f) { /* same body rules as Phase 03 */ }

template <typename T>
struct Stack {
    gp::slice<T> items;
    static void Push(gp::ptr<Stack<T>> s, T v);
};
template <typename T>
void Stack<T>::Push(gp::ptr<Stack<T>> s, T v) { s->items = gp::append(s->items, v); }

// call site: type arguments come from go/types, never from C++ deduction
gp::slice<gp::string> names =
    Map<int64_t, gp::string>(gp::slice<int64_t>{1, 2}, &::gpkg_strconv::Itoa);
gp::ptr<Stack<gp::string>> st = gp::new_<Stack<gp::string>>();   // boxed: pointer method called
Stack<gp::string>::Push(st, "a"_gs);
```

C++ deduction can't see through `gp::func<U(T)>` from a plain function pointer, and it
handles untyped constants differently from Go. **Explicit type arguments** remove both problems
and keep Go's exact typing. They come from `types.Info.Instances` for generic functions and types,
and from `types.Info.Selections` for methods of generic types (`st.Push` has no `Instances`
entry; its receiver type `Stack[string]` is in the selection).

### 2.2 Operations on type parameters

| In a generic body | C++ |
|---|---|
| `a + b`, `a * b`, `-a` (numeric constraints) | operators, then **cast back to `T`**: `T(a + b)`. C++ promotes small integers to `int`, so without the cast `int8(100) + int8(100) > 0` would be true; in Go it is false (it wraps to -56) |
| `a / b`, `a % b`, `a << n`, `a >> n` | the Phase 02 checked helpers (`gp::div`, `gp::rem`, `gp::shl`, `gp::shr`), which use `if constexpr` to tell integers from floats: `Div[int](1, 0)` panics with `integer divide by zero`, `Div[float64](1, 0)` gives `+Inf` |
| `a < b`, `a == b` | plain comparison. For interface type arguments, the Phase 05 panic on uncomparable dynamic types applies, as in Go |
| `T(x)` conversion | `gp::convert<T>(x)` (`if constexpr` over numeric / string / bytes / newtype cases) |
| `len(x)`, `x[i]`, `range x` when every type in the type set supports it (e.g. `~string \| ~[]byte`) | overloaded `gp::len`, `gp::index`, `gp::range_of` |
| `var zero T` | `T zero{};` |
| `x.String()` with `T` constrained by an interface | `gp_call_String(x)`: a generated helper that picks the right call form at C++ compile time (2.3) |
| `switch any(v).(type)` | Phase 05 type switch on descriptors |

### 2.3 Method calls on type-parameter values

Inside `func F[T fmt.Stringer](x T) string { return x.String() }`, `T` might be a struct with a
value method, a pointer `*X` with a pointer method, a pointer with a value method, or a struct
whose `String` is **promoted** from an embedded field. The generic body can't know which. Codegen
emits one helper per method name used this way:

```cpp
template <typename T, typename... A>
decltype(auto) gp_call_String(const T& x, A&&... a) {
    if constexpr (gp::is_ptr_v<T>) {                                  // T is *X: check this first
        using X = gp::pointee_t<T>;
        if constexpr (requires { X::String(x, std::forward<A>(a)...); })
            return X::String(x, std::forward<A>(a)...);               // pointer method
        else
            return x->String(std::forward<A>(a)...);                  // value method via auto-deref
    } else {
        return x.String(std::forward<A>(a)...);                       // value method (incl. promoted)
    }
}
```

- Checking "is `T` a `gp::ptr`" **first** means a Go method can never accidentally resolve to a
  runtime member. The runtime types also expose no members with plain names (Phase 03 rule), so a
  Go method called `get` or `len` stays the user's method.
- Arguments are forwarded and the result type is `decltype(auto)`, so one helper per name works
  for any signature.
- Promoted methods work because Phase 05 generates **forwarding member functions** on the outer
  struct for every promoted method.

Dispatch is static and inlinable, with no dynamic itab lookup.

### 2.4 Generic types: descriptors, aliases, interop

- **Generic code carries Go types, not just C++ types.** `int` and `int64` are both `int64_t` in
  C++, so a template alone can't tell which Go type `T` is. Every template therefore gets one
  hidden descriptor argument per type parameter (Phase 05 chooses descriptors by Go type), and
  `any(x)`, type switches and `%T` inside generic code use it.
- Each instance (`Stack[int]`) gets its own type descriptor. Its Go-style name must match Go's
  `%T` exactly. Inside type-argument lists Go uses **full import paths and no space after
  commas**: `main.Pair[string,[]int]`, `main.Stack[net/url.URL]` (verified with Go 1.25.7). Each
  descriptor therefore stores a fully qualified name next to its short name.
- Generic type aliases (Go 1.24): `type Set[T comparable] = map[T]struct{}` →
  `template <typename T> using Set = gp::map<T, gp::empty>;`.
- Type parameters work inside C++ templates (`std.vector[T]`).
- **C/C++ types as type arguments** (`Map[C.int, string]`): `go/types` records **no**
  `Instances` entry when a type argument is opaque (`FakeImportC` skips instantiation, verified).
  So explicitly written type arguments are spelled from the syntax (Phase 04), and *inferred*
  ones become `decltype`-based arguments (`Max<std::remove_cvref_t<decltype(a)>>(a, b)`). Where
  neither works, the user gets an error asking to write the type arguments out (until Phase 12).
  `go/types` can't check their constraints, so the C++ compiler does, and its errors are **user**
  errors, shown through the Phase 13 translation.
- Optional `-gp:concepts` emits C++20 `requires` clauses that mirror the Go constraints. They are
  documentation for C++ users of `cpp-lib` output. Correctness never depends on them.

### 2.5 Range-over-func iterators (Go 1.23)

```go
func Count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	for x := range Count(5) {
		if x == 3 {
			break
		}
		fmt.Println(x)
	}
}
```

The loop body becomes the `yield` function. We copy the design of Go's own rewrite
(`cmd/compile/internal/rangefunc/rewrite.go`) instead of inventing one:

```cpp
{
    auto gp_rs = gp::new_<gp::rangefunc_state>();     // boxed: safe even if yield is called late
    Count(5)([gp_rs](int64_t x) -> bool {             // a normal closure (Phase 03 capture rules), never [&]
        gp_rs->enter();                               // panics with Go's message on misuse
        if (x == 3) { return gp_rs->exit_break(); }   // break: state DONE, return false
        ::gpkg_fmt::Println({x});
        return gp_rs->exit_continue();                // end of body / continue: return true
    });
    gp_rs->finish();                                  // loop over: state EXHAUSTED + final checks
}
```

- **Capture:** the body is a normal closure under the Phase 03 rules; the state and any outer
  variable the body assigns are boxed. A by-reference `[&]` lambda is wrong: an iterator may keep
  `yield` and call it after the loop's function has returned. Go then panics, but a `[&]`
  lambda reads a dead stack frame (reproduced under ASan during the audit).
- **Misuse detection:** Go's five states (READY, PANIC, DONE, EXHAUSTED, MISSING_PANIC) and its
  four exact messages: `range function continued iteration after function for loop body returned
  false`, `... after whole loop exit`, `... after loop body panic`, and `range function recovered
  a loop body panic and did not resume panicking`.
- **Jumps:** `continue` → `return true`; `break` → `return false`; `return v` → store `v` and a
  code, `return false`, then return after the call. `break L`, `continue L` and `goto L` to labels
  outside the body set a numeric "next" code that is checked after the call (Go uses negative
  codes for `goto`).
- **`defer`** in the body belongs to the **enclosing function** (Go rule), even when `yield` runs
  on another goroutine (checked against Go).
- `iter.Pull` needs coroutines: it is built on a helper thread (Phase 08).

### 2.6 Go's generic packages, ported

`cmp` and `slices` are pure Go and can be copied from the Go source tree (BSD license) with small
edits. Two spots are not pure Go: `maps.Clone` calls a runtime function through `//go:linkname`,
and `iter.Pull` uses runtime coroutines the same way. The port tool (Phase 09 `tools/portstd`)
turns such `//go:linkname` declarations into native bodyless functions implemented in C++. The
packages' own `_test.go` files need `testing`, `fmt`, `strconv` and more, so **running Go's tests
for them is Phase 09 work**. This phase relies on its own run tests and differential tests.

## 3. Steps

1. **7.1 Generic function declarations** → templates in `pkg.hpp`.
2. **7.2 Explicit type arguments at every use**: `types.Info.Instances` (functions, function
   values, types) and `types.Info.Selections` (methods of generic types); the syntax/`decltype`
   rule for C/C++ type arguments.
3. **7.3 Generic types + methods** (value/pointer receivers, out-of-class template definitions,
   instance descriptors with Go's exact qualified names).
4. **7.4 Type-parameter operations**: Go arithmetic (cast back to `T`, checked `/ % << >>`),
   `gp::convert`, `len`/index/range over type sets, zero values; tests with `int8`, `uint8`,
   `float64`.
5. **7.5 `gp_call_<Method>` dispatch helpers**: pointer check first, forwarded arguments,
   promoted methods through Phase 05 forwarding members; tests for embedded value/pointer methods,
   methods with arguments, and names like `get`/`len`.
6. **7.6 Interfaces × generics** (constraint methods, `comparable`, interface type arguments).
7. **7.7 Generic type aliases.**
8. **7.8 Range-over-func**: Go's state machine and four messages, closure capture,
   break/continue/return/labels/`goto`/`defer`.
9. **7.9 Optional `-gp:concepts` output.**
10. **7.10 Port `cmp`, `slices`, `maps`, `iter`** (`//go:linkname` → native functions; their Go
    tests run in Phase 09).
11. **7.11 Lift the generics gate**; update the feature matrix.
12. **7.12 Tests**: ≥ 60 generic run tests + differential tests vs Go (misuse panics, a late
    `yield` under ASan, small-integer arithmetic, `%T` of instances from other packages); measure
    compile time on a generics-heavy program.

## 4. Deliverables

- Generic codegen (functions, types, methods, aliases), dispatch helpers, range-over-func
- `stdlib/{cmp,slices,maps,iter}` ported (their Go tests run in Phase 09)
- Target program `generics` passing

## 5. Exit criteria

- `generics` example passes on both compilers.
- ≥ 60 generic tests + differential tests green, including `int8`/`uint8` arithmetic and
  division by zero inside generic code.
- Range-over-func behavior (break/return/labels/`goto`/`defer`, all four misuse panics) matches Go;
  a late `yield` call is ASan-clean and panics like Go.
- `%T` of generic instances matches Go, including type arguments from other packages.
- Compile-time overhead of generics is measured and written in the phase notes.

## 6. Risks

| Risk | Mitigation |
|---|---|
| A codegen bug shows up as a huge C++ template error | For pure gocpp code it is an internal error (go/types already checked the program). With C/C++ type arguments it is a user error, shown through Phase 13's translation |
| Template bloat slows compiles | Measure; `extern template` for hot instances (e.g. `gp::slice<int64_t>`) in the runtime |
| Range-over-func rewrite subtleties | Copy Go's `rewrite.go` design; differential tests here, Go's own rangefunc tests in Phase 09 |
| Dispatch helper picks the wrong call form | Explicit tests for every receiver shape, promoted methods and plain-named methods |
