# ADR-028: `fmt` is one engine that walks values by their type descriptors

Status: accepted (2026-10-18)

## Context

Phase 03 gave gocpp a stand-in `fmt` (`gp::fmtshim`): a header of templates that printed the
statically typed arguments of `fmt.Println` and friends. It knew the C++ type of every
argument at compile time, so it needed protocols in every generated type (`gp_fields`,
`gp_type_name`, `gp_value_tag`, `gp_String`, `gp_Error`), type tags for `int64`, `uint64` and
`uintptr`, and it could not print a value inside an `any`, an `error` or a `fmt.Stringer`:
the dynamic type is unknown to a template.

Phase 05 puts values into interfaces, so `fmt` must print every value, wherever it came from.
Go's `fmt` does it with reflection. gocpp has no reflection yet, but it has type descriptors
(ADR-016), which already hold the name, kind, size and method table of every type that flows into
an interface. The descriptor of a named slice, array, map, func, pointer or struct type was
deliberately partial (no element, key, parameters or fields), so that it could be constant
initialized; the unnamed composite descriptors were complete.

## Decision

`fmt` is a port of Go 1.25's `fmt/print.go` and `fmt/format.go` (and of the parts of `strconv`
and `internal/fmtsort` they use) in `runtime/src/fmt.cpp`, driven by descriptors the way Go's is
driven by `reflect`. An operand is a pair {descriptor, address of the value}. One engine prints
statically typed values and the ones inside interfaces, and the shim's templates are gone.

1. **Complete descriptors, lazily.** A named type with a composite underlying type keeps its
   constant-initialized partial descriptor object (its address is the identity of the type), plus
   `gp_full`, a function pointer. The emitter writes the function (`gp_full_Rect`): it makes, once,
   a complete copy with the field list (name, package, descriptor, tag, embedded flag and the
   **offset** of the field in the C++ struct), the element, key, parameters and results. It runs on
   the first use, after the program's descriptors are interned, so recursive types and descriptors
   that are only known at run time need no ordering. `gp::full(d)` is what everything that reads
   the parts of a type asks for. The unnamed composite descriptors were complete already; their
   fields got offsets too.
2. **Operations in the descriptor.** `type_desc::gp_ops` (gocpp/typeops.hpp) holds the small functions
   that read a value of the C++ type without knowing it: the data and length of a slice or an
   array, a copy of a map's entries, the target and owner of a pointer, the address of a func,
   the nil test. `with_cxx<T>` sets them from the C++ type T that a descriptor stands for, so the
   layouts of `gp::slice`, `gp::map` and `gp::ptr` stay private to their headers. A named type
   uses its underlying class's operations.
3. **Operands.** `gp::fmt::Println(x, y)` takes plain values by value; a value whose C++ type says
   its Go type (bool, int, uint, the sized integers except 64-bit ones, floats, complex numbers,
   string) is passed as it is, with no descriptor and no allocation, so the common calls cost what
   the shim's cost. Anything else is `gp::fmt::as(descriptor, value)`; an interface value is its
   own descriptor. C++ cannot tell `int` from `int64`, so `int64_t` is Go's `int` unless the
   emitter says `gp::fmt::as(&gp::td_int64, x)`.
4. **Methods** (`Format`, `GoString`, `Error`, `String`) are found in the descriptor's method
   table and called through their thunks with an interface value over the operand's address.
   `fmt.State` is a runtime interface over the printer; the interface types `fmt.Stringer`,
   `fmt.GoStringer`, `fmt.Formatter` and `fmt.State` are runtime structs like `error` (the lowering
   maps them), because package fmt has no C++ of its own. A panic in a method is caught and
   printed as Go does.
5. **C/C++ values** are described by `gp::foreign_td<T>()`, whose operations classify the C++ type
   (number, enum, pointer, string, class). A class prints through its `std::formatter` or
   `operator<<` only when `gp::cgo::fmt_arg` registered it at a call that names a value with that
   class inside (the check that the class is printable stays a compile error at the `.gp` line);
   a class that nobody registered, held in an `any`, prints as its address.
6. **`Errorf`** builds the runtime's `*errors.errorString`, `*fmt.wrapError` and `*fmt.wrapErrors`
   (descriptors with `Error` and `Unwrap` methods), so `errors.Is` and `errors.As` (step 5.7) see
   Go's `Unwrap` protocol.

## Alternatives

- **A printer function per type**, set in each descriptor from the shim's templates
  (`type_desc::print`). Rejected: every package that creates a descriptor would pull the whole
  template machinery in (compile time for every program), a descriptor made in a package that did
  not include it would print differently, and the dynamic path would still need a second
  implementation for composites in an `any`.
- **Complete descriptors in the emitted initializer.** Rejected: a descriptor of a recursive or
  mutually recursive type points at the interned descriptors of its parts, which are made at run
  time; a constant-initialized object cannot.
- **Keep the shim for static arguments and add a dynamic path for `any`.** Rejected: two engines
  that must agree on `%v` of a map, a pointer, a nil, a method set.

## Consequences

Good: the output is Go's by construction (the algorithm is Go's, the differential corpus of
`tests/run/fmt_*.gp` compares byte for byte with real Go); `fmt` of a value in an `any` is the
same code as of a static value; `%T`, the error forms and `%#v` use the descriptor's names;
compile time falls (no templates per call site); the descriptors are what Phase 09's `reflect`
needs (`gp_full`, field offsets, `gp_ops`).

Bad: every type that is printed needs a descriptor, so the lowering registers descriptors for
the types of fmt operands (a struct that was printed now also gets its method tables and
thunks); a named type with a C/C++ field is described with the field's C++ type
(`gp::cxx_desc`), so a named gocpp type nested inside a C++ template argument is described as a
foreign class (its methods are not called there).
