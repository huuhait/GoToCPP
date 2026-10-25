# ADR-093: Package variables that hold a C or C++ class by value are `gp::lazy` storage

Status: accepted (2026-10-19), closes [Phase 06 README §2.4](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md)
(step 6.4: "`gp::lazy<T>` for C++ types that can't be constant-initialized"), builds on
[ADR-043](043-library-build-modes.md) (lazy start of libraries)

## Context

Step 6.4 asked for `gp::lazy<T>`. Before writing it, the question was whether the design built since
(Go 1.21 order in the driver, `constinit` package variables, the lazy thread-safe start of `c-archive`
and `c-shared` in step 6.10) already covers what the step wanted. It covers most of it:

- **The static initialization order fiasco cannot happen** for every package variable of a gocpp type:
  each is `constinit` (zero or a compile-time constant), `gp_var_init()` sets the rest in Go's order, and
  `gp_init_all()` runs the packages in Go 1.21's order (tests: `tests/pkgs/embed_init_recover`,
  `vars_funcs_init`, `generics_init`).
- **Libraries start lazily and thread-safely**: `gp::lib::guard` runs `gp_init_all` once under
  `std::call_once` at the first exported call (ADR-043; `TestModesCArchive`, `TestModesCShared`,
  the two-libraries test).
- **Executables pay nothing**: `constinit` variables have no initialization code at all.

Two cases were not covered, and both were found by running them:

1. A package variable of a **C or C++ class** (`var l late.Late`, `var mu std.mutex`, `var s
   std.vector[T]`) was declared `T l{};` without `constinit` ("a C++ class may have a constructor that
   isn't constexpr"). That is a dynamic initializer, which runs when the translation unit's statics are
   initialized, not when gocpp wants. A `c-archive` whose host calls an exported function from a
   constructor of its own (`__attribute__((constructor))`, a C++ static initializer: it runs before the
   archive's objects are initialized, as they come later in the link) ran `gp_var_init`, stored the value,
   and then the archive's dynamic initializer built the object again and **wiped the value** (the audit
   of Phase 06 §2.4 showed the same for a `std::function` global in a shared library). The probe printed
   `early peek=7`, `main peek=100`.
2. A **struct of ours with a field of such a class** (`type Holder struct { n int; l late.Late }; var h
   Holder`) was `constinit`, so a constructor that is not `constexpr` made the generated C++ not compile:
   "`constinit` variable does not have a constant initializer" (reported as an internal error).

## Decision

A package variable whose type holds a C or C++ class **by value** (`cmpHasC`: a C/C++ type, or a struct or
array of ours that holds one by value; pointers, slices, maps and funcs of them stay as they were) is
`gp::lazy` storage (`runtime/include/gocpp/lazy.hpp`, `internal/lower/pkgvars.go` `lazyVar`):

```cpp
gp::lazy<late::Late> gp_lazy_l;                  // constant-initialized: storage and a flag, no object
constinit late::Late& l = gp_lazy_l.gp_ref();    // the variable's own name, a constant reference
static const gp::lazy_boot gp_boot_l{gp_lazy_l}; // dynamic initializer: builds the object if nobody did

void gp_var_init() {
    gp_lazy_l.gp_ensure();                       // first, for every lazy variable of the package
    ...                                          // then the initializers, in Go's order, as before
}
```

- `gp_ensure()` value-initializes the object (`T{}`, Go's zero value) unless it exists. `gp_var_init` calls
  it first for all lazy variables of the package, so a Go initializer that reads a variable without an
  initializer (through a function) still sees a built object. The early call of a library runs
  `gp_var_init` before the archive's dynamic initializers, and `gp_boot_*` then finds the object built and
  does nothing: the stored value stays (the probe now prints `main peek=7`).
- `gp::lazy_boot` is for what never runs `gp_var_init`: a `cpp-lib` consumer that did not call `init()`
  gets the object at program start as before (the old behavior: `T l{}`).
- The variable keeps its name and its type for every use (`l.set(7)`, `&l`, `l = x`, `std::size(l)`,
  `extern T& l;` in `pkg.hpp`): C++ uses a reference to the object. The union member is named in a
  constant expression without being read, which g++ and clang++ accept for a `constinit` reference. Only
  `decltype(l)` is `T&` instead of `T`, and the emitter uses it only for its own temporaries.
- **The object is never destroyed.** Go runs no destructor of a package variable, and a thread that is
  still running at exit cannot meet a destroyed `std::mutex`. A heap block that the object owns stays
  reachable from static storage, so LeakSanitizer does not report it (the runtime unit test and the
  interop tests run under ASan).
- An executable pays one empty-looking call (`gp_ensure` after the boot ran) and gets a `gp_init_all`
  in a program with lazy variables; programs without such a variable are unchanged, and so is every
  variable of a gocpp type (`constinit`).

## Not done, on purpose

- **No `gp::lazy` for gocpp's own types.** They are `constexpr`-constructible, so `constinit` is better:
  no reference, no flag.
- **No decision by C++ traits** (`constinit` when `T` is constant-constructible, lazy otherwise). One
  shape for a C/C++ class by value keeps the generated code and the header (`extern T&`) the same for every
  class; a trait would give `extern T` for one class and `extern T&` for another, and a header that
  changed with a library's inline functions.
- **Lazy construction at the first use** (a function-local static per access). It would add a guard check
  to every use of the variable; `gp_var_init` already is the one place that runs before any Go code.

## Addendum (2026-10-24): an early call needs nothing of the static initialization

The early call above ran `gp_var_init`, but anything else that the package initialization used and a
dynamic initializer made was still missing: an interface conversion read the method tables of its type
(`gp_vm_T`, `gp_pm_T`, dynamically initialized because `gp::method_of` stored the thunk with a
`reinterpret_cast`) and crashed in `getitab`; the composite descriptors (`gp_tdc_N`, `gp_tdp_T`) were
null; the runtime's timer heap and math's CPU flags were made at program start. Now none of them is:

- `gp::method_of<Sig, &thunk>(...)` is a constant expression: the entry holds a pointer to a function
  that gives the thunk (`gp::fn_getter`), so the tables are constant-initialized, with no cost at a call
  through an itab (the itab holds the thunks themselves, as before).
- A composite descriptor is a constant-initialized pointer `gp_tdc_N_p` that its maker
  `gp_tdc_N_make()` fills at the first use (`gp::desc_get<&gp_tdc_N_p, &gp_tdc_N_make>()`: an acquire load
  and a test at -O0; a const function when optimizing, which the compilers hoist out of loops, as they
  did the namespace-scope const pointer of before).
- The timer heap is a function-local static, math's `has_fma()` and `has_sse41()` look at the CPU at
  their first call (a constant-initialized byte): `internal/build` `TestRuntimeObjectsHaveNoStaticInit`
  allows no initializer in the runtime's objects but `testing.o`'s.

`TestModesCArchiveEarlyCallPackages` calls from a host constructor a library whose initialization
converts to interfaces, uses `errors`, `fmt` (with a `[]int`), `math.Floor` and arms a timer.

## Addendum 2 (2026-10-24): no package storage is destroyed

Every other package variable was `constinit T x{...}`, and a `T` with a destructor (a string, a slice,
a map, an interface, a struct of them) was registered with `__cxa_atexit`. An executable never ran
those (it ends with `gp::os::exit`), but a library's host that returns from `main` or calls `exit()`
destroyed the variables while the library's goroutines still used them: a crash in 27 of 30 runs of a
c-archive host, every run of a c-shared one, a heap-use-after-free under ASan. Go's c-archive runs no
destructor. Now every package variable whose Go type owns something (a string, slice, map, pointer,
interface, channel, func, or a struct or array of them) is the member of `gp::keep<T>` storage, a union
whose destructor does nothing. A variable of a trivially destructible type (numbers, complex numbers, bools, arrays and
structs of them) stays a plain `constinit T x{...}`: nothing is registered for it.

```cpp
extern gp::keep<gp::slice<gp::string>> gp_keep_words;                // pkg.hpp
inline constexpr gp::slice<gp::string>& words = gp_keep_words.gp_v;  // pkg.hpp: the variable's name
constinit gp::keep<gp::slice<gp::string>> gp_keep_words{};           // pkg_init.cpp
```

Every use names the variable, an `inline constexpr` reference to the member: an address constant, in
every file, with no pointer object in memory. (The first version made the name a `constinit` reference
variable: its address sat in a global initializer, alias analysis saw an escaped object, and a loop over
another package's variable ran 68% slower with g++ `-release`.) The initialization is the variable's,
constant as before. The exported constants of a named type (`pkg.hpp`, for C++ code) are kept the same way.
A program has no start-up work for them, and the bench programs are smaller and start a little faster
(a hello world: 2.09 against 2.25 MB with g++ `-release`). `TestModesLibraryExitWhileGoroutineRuns`
runs the host 20 times for each compiler and mode. (g++ at `-O0` still registers the union's empty
destructor; it destroys nothing.)

## Addendum 3 (2026-10-24): large arrays of owning types are lazy too

`var bigS [1 << 21]string` and `var bigP [1 << 20]*int` did not build with clang++ (also on dev):
`constinit` makes the compiler build the object by constant evaluation, which clang++ ends after
1 M steps ("variable does not have a constant initializer"). A package variable whose type owns something
(a string, slice, map, pointer, interface, channel or func, or a struct or array of them) and has more than
65536 such objects (`leafObjects` against `maxConstinitLeaves`, `internal/lower/pkgvars.go`) is therefore
`gp::lazy` storage as well, with the same parts: no object at load time, the variable's name a `constinit`
reference, `gp_var_init` building the zero value first, and the dynamic `gp::lazy_boot` initializer building
it only when `gp_var_init` did not (a cpp-lib consumer that never calls `init()`). Smaller arrays
(`[1 << 12]Rec`) stay `gp::keep` and `constinit`. Test: `tests/run/pkgvars_large_nontrivial.gp`.

## Consequences

- `docs/spec/cpp-mapping.md` ("Package initialization") describes the two shapes.
- Tests: `runtime/test/lazy_test.cpp` (gp::lazy and gp::lazy_boot, under g++, clang++, ASan, O0 and O2),
  `TestModesCArchiveEarlyCall` (the probe above, from a real C host), `tests/pkgs/lazy_vars` (a class, a
  struct with a class field and an array of classes in another package, built once each, in declaration
  order), `tests/golden/interop_types`. Mutants that fail them: the lazy rule off, `gp_ensure` without the
  flag check, `gp_var_init` without the `gp_ensure` calls.
- Not built (pre-existing): a method call on a C++ class value of another package's variable
  (`store.L.get()` from `main`) is rejected by the emitter ("not supported yet: the identifier store");
  the fixture calls such methods inside the owning package.
