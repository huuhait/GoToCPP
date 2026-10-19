# gocpp for C++ developers

gocpp writes C++20. If you know C++, the quickest way to understand gocpp is to read what it
writes: this guide shows the output for a few small programs, says what the runtime types are, and
lists where the two languages meet. The complete mapping is in the
[C++ mapping spec](../spec/cpp-mapping.md); the code blocks here are checked by the tests
against the real output of `gocpp emit`.

## Reading the generated code

```sh
gocpp emit .             # all the generated files, one after the other
gocpp emit -annotate .   # with the .gp line of each part and the reason for the choices
gocpp build -work .      # keep the work folder: the generated files and the objects
gocpp build -x .         # print the compiler commands
```

Every package gets a folder `gpkg/<import path>/` with a header `pkg.hpp` (what other packages and
its own units need), one translation unit for each `.gp` file (`main.gp.cpp`), `pkg_desc.cpp` (the
type descriptors) and `pkg_init.cpp` (package variables and, for `main`, the C `main`). A package
is a C++ namespace: `main` is `gpkg_main`, `geom` is `gpkg_geom`. A `#line` directive before the
generated code makes the debugger and the C++ compiler talk about your `.gp` lines
([debugging guide](debugging.md)). The C++ compiler is `g++` 13 or newer or `clang++` 17 or newer;
every file is compiled with `-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off`, so signed
overflow wraps and floating point is not fused, as in Go.

## A first look

<!-- program: 05 id=mapping go -->
```go
package main

import "fmt"

type Point struct {
	X, Y int
}

func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }

func (p *Point) Scale(k int) {
	p.X *= k
	p.Y *= k
}

func divmod(a, b int) (int, int) {
	return a / b, a % b
}

func main() {
	p := Point{1, 2}
	p.Scale(3)
	q, r := divmod(7, 2)
	xs := []int{1, 2, 3}
	xs = append(xs, q+r)
	m := map[string]int{"a": 1}
	m["b"] = 2
	fmt.Println(p.Add(Point{1, 1}), xs, len(m))
}
```

<!-- output -->
```text
{4 7} [1 2 3 4] 2
```

The type becomes a struct, a value method a `const` member and a pointer method a `static`
function that takes the receiver as a `gp::ptr`:

<!-- emitted: mapping -->
```cpp
struct Point {
    int64_t X = 0;
    int64_t Y = 0;
    bool operator==(const Point&) const = default;
    ...
    Point Add(Point q) const;
    static void Scale(gp::ptr<Point> p, int64_t k);
};
static_assert(sizeof(Point) == 16 && alignof(Point) == 8);
```

The functions use plain C++ and a few runtime helpers. Division and remainder go through
`gp::div` and `gp::rem` because Go panics on a zero divisor and has no undefined cases; several
results are a `std::tuple`:

<!-- emitted: mapping -->
```cpp
Point Point::Add(Point q) const {
    const Point& p = *this;
    return Point{p.X + q.X, p.Y + q.Y};
}
...
std::tuple<int64_t, int64_t> divmod(int64_t a, int64_t b) {
    return {gp::div(a, b), gp::rem(a, b)};
}
```

`main` shows the rest: the variable `p` is a `gp::ptr<Point>` because a pointer method is called on
it, results are unpacked from the tuple, and slices and maps are runtime types.

<!-- emitted: mapping -->
```cpp
void main() {
    gp::ptr<Point> p = gp::new_<Point>(Point{1, 2});
    Point::Scale(p, 3);
    auto gp_t0 = divmod(7, 2);
    int64_t q = std::get<0>(gp_t0);
    int64_t r = std::get<1>(gp_t0);
    gp::slice<int64_t> xs = gp::slice_of<int64_t>({1, 2, 3});
    xs = gp::append(static_cast<gp::slice<int64_t>&&>(xs), q + r);
    gp::map<gp::string, int64_t> m = gp::map_of<gp::string, int64_t>({{"a"_gs, 1}});
    gp::set(m, "b"_gs, 2);
    ...
}
```

## The runtime types

Everything of the runtime is in the namespace `gp`, in the headers under `runtime/include/gocpp/`.

| Go | C++ | What it is |
|---|---|---|
| `int`, `uint`, `byte`, `rune` | `int64_t`, `uint64_t`, `uint8_t`, `int32_t` | `int` is 64 bits: gocpp has 64-bit targets only |
| `string` | `gp::string` | immutable bytes with an owner; substrings share them; a literal is `"x"_gs` and points at static storage |
| `[]T` | `gp::slice<T>` | owner, data pointer, length, capacity; `append` gives Go's capacities |
| `[N]T` | `gp::array<T, N>` | a value type with bounds checks |
| `map[K]V` | `gp::map<K, V>` | a reference to a Swiss table |
| `*T` | `gp::ptr<T>` | a reference counting pointer with nil checks; `gp::cptr<T>` for C and C++ types |
| `func(A) R` | `gp::func<R(A)>` | a closure with an owner; a lambda inside |
| `interface{...}`, `any`, `error` | a struct derived from `gp::iface`, `gp::any`, `gp::error` | data, a type descriptor and a method table; no C++ virtual functions |
| `chan T` | `gp::chan<T>` | a reference to the channel |
| `(A, B)` results | `std::tuple<A, B>` | |

The sizes are the C++ sizes (a `string` is 32 bytes, a slice 40), not Go's
([D15](../spec/language.md#d15-different-sizes)). Each generated struct has a `static_assert`
on its size, so the type checker and the C++ compiler cannot disagree.

## Memory: reference counting, no garbage collector

A gocpp value is freed when the last reference goes. A pointer to a gocpp type is a
`gp::ptr<T>`, a `std::shared_ptr` with nil checks; a pointer to a field or to an element shares the
owner of the whole object. A local variable lives in a box on the heap when its address is taken,
when a pointer method is called on it, or when a closure captures it and changes it: that is why
`p` above is a `gp::ptr<Point>`. Everything else is an ordinary C++ local.

- A cycle of pointers is never freed. Break it with `weak.Pointer[T]` (a `gp::weak_ptr<T>`).
- A data race can corrupt a reference count; Go's data races are bad, gocpp's are worse
  ([D10](../spec/language.md#d10-data-races-can-break-reference-counts)).
- A pointer that C or C++ gives you owns nothing: you have C++'s lifetime rules for it
  ([D16](../spec/language.md#d16-unsafepointer-and-pointers-from-c)).
- A real escape analysis, which keeps more variables on the stack, is planned (step 15.2 of the plan); today the rule above is the simple one.

## Interfaces, closures, defer and panics

<!-- program: 05 id=shapes2 go -->
```go
package main

import (
	"errors"
	"fmt"
)

type Shape interface {
	Area() int
}

type Square struct{ Side int }

func (s Square) Area() int { return s.Side * s.Side }

func total(shapes []Shape) int {
	n := 0
	for _, s := range shapes {
		n += s.Area()
	}
	return n
}

func counter() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

func open(name string) error {
	defer fmt.Println("closing", name)
	if name == "bad" {
		return errors.New("bad name")
	}
	return nil
}

func main() {
	fmt.Println(total([]Shape{Square{2}, Square{3}}))
	next := counter()
	fmt.Println(next(), next(), open("bad"))
}
```

<!-- output -->
```text
13
closing bad
1 2 bad name
```

An interface is a struct derived from `gp::iface`. A call through it is `gp::icall` with the
index of the method; the types that have the methods get type descriptors and method tables in
`pkg_desc.cpp`. There are no virtual functions, so a type satisfies an interface without being
declared to:

<!-- emitted: shapes2 -->
```cpp
struct Shape : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc();
    [[gnu::abi_tag("gp_wrap")]] int64_t Area() const;
};
...
[[gnu::abi_tag("gp_wrap")]] inline int64_t Shape::Area() const {
    return gp::icall<int64_t()>(*this, 0);
}
```

A closure is a lambda inside a `gp::func`. The variable `n` that it changes lives in a box that the
lambda holds, so it outlives the call of `counter`:

<!-- emitted: shapes2 -->
```cpp
gp::func<int64_t()> counter() {
    gp::ptr<int64_t> n = gp::new_<int64_t>(0);
    return gp::func<int64_t()>([n]<int = 1>() mutable -> int64_t {
        (*n)++;
        return (*n);
    }
    );
}
```

`defer` is a frame object, and a panic is a C++ exception (`gp::panic_error`) that the frame
catches, runs the deferred calls for, and passes on or stops at `recover`:

<!-- emitted: shapes2 -->
```cpp
gp::error open(gp::string name) {
    gp::error gp_r0{};
    gp::defer_frame gp_df; try {
        gp_df.gp_push_call([](auto&& gp_a0) { gp::fmt::Println("closing"_gs, gp_a0); }, name);
        ...
    } catch (gp::panic_error& gp_p) {
        gp_df.gp_take(gp_p);
    } catch (...) {
        gp_df.gp_adopt();
    }
    gp_df.gp_run(); return gp_r0;
    ...
}
```

A function with no `defer` has none of this. Runtime errors (nil pointer, index out of range,
division by zero, a failed type assertion) are checked by the runtime helpers and panic with
Go's messages. A C++ exception that reaches gocpp code becomes a panic
([D26](../spec/language.md#d26-c-exceptions)).

## Goroutines are threads

A goroutine is a detached OS thread with an 8 MiB stack, started by `gp::go_fn` or `gp::go_call`
(`go f(x)` evaluates `f` and `x` in the calling thread, like `defer`). The thread of a goroutine
that ended waits a few seconds for the next `go` statement, so a loop of short goroutines does not
start a thread for each. There is no scheduler: the operating system schedules, and a blocked
goroutine blocks its thread. A `gp::chan<T>` is a shared pointer to Go's `hchan`: a mutex, a ring
buffer and two queues of waiting goroutines. A program with thousands of goroutines is fine, one with
a million is not ([D7](../spec/language.md#d7-goroutines-are-os-threads)). The details are in
[cpp-mapping: goroutines and channels](../spec/cpp-mapping.md#goroutines-and-channels). Goroutines
are behind `GOCPP_GATE_PHASE=08` today (phase 08 is not finished).

## Using C++ from gocpp, and gocpp from C++

- **gocpp calls C++**: import a header and use its namespace (`std.vector[int]`, `"<vector>"`).
  What C++ returns is a C++ value with C++ rules (`v[i]` is not bounds checked, `m[k]` inserts):
  see the [tour](../tour/09-cpp-containers.md), the [interop guide](../spec/interop.md) and the
  [cookbook](../spec/interop-cookbook.md). Your own `.cpp` and `.hpp` files next to the `.gp`
  files are compiled with them.
- **C++ calls gocpp**: a function marked `//export` can be called from C and C++ when the
  package is built as a C archive or a shared library (`gocpp build -buildmode=c-archive`); the
  guide [calling gocpp from C and C++](libraries.md) shows the header it writes and a C++ program
  that uses it.
- **Typed interop** (`GOCPP_TYPED_INTEROP=1`, experimental): gocpp asks the C++ compiler for the
  types of the C++ names you use and checks them itself, so the mistakes show before the C++ build.
  See the [typed interop spec](../spec/typed-interop.md).

## What to know before you rely on it

- Names: a Go name is a C++ name in the namespace of its package. A Go name that is a C++ keyword or
  macro is renamed; [cpp-mapping](../spec/cpp-mapping.md#names) says how.
- The generated code is C++20 and readable, but its exact form is not a promise: regenerate it
  with the gocpp version you build with, and do not edit it (the header says so).
- The runtime is a static library that gocpp builds once per compiler and flags and caches in
  `$XDG_CACHE_HOME/gocpp/rt/`. A program does not need gocpp installed to run.
- Compile time is the cost of C++ templates and headers; the
  [compile time guide](generics-compile-time.md) shows how to keep it low.
