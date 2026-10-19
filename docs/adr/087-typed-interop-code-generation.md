# ADR-087: Typed interop writes C++ from the real types, behind a switch, and falls back to opaque

Status: accepted (2026-10-19). Builds on [ADR-060](060-typed-interop-probes.md) (the probes),
[ADR-074](074-typed-interop-declarations-and-foreign-types.md) (declarations and the foreign view) and
[ADR-079](079-typed-interop-checker-reports-only-what-the-compiler-reports.md) (the checker); the contract is
[docs/spec/typed-interop.md](../spec/typed-interop.md) §8.

## Context

Opaque mode (Phase 04) writes every C/C++ expression almost as it is and lets the C++ compiler decide:
`gp::deref(x).m` for a member (the runtime picks `.` or `->` by the type), a lambda of assignments for a
keyed literal (GCC rejects designated initializers out of declaration order, and opaque mode does not know
it), `C.cstr(s)` written by the user for every Go string that goes to a `const char*`, and an error from
the guard (cguard.go) wherever a C number meets a Go number, as in cgo. Steps 12.1 to 12.5 give the lowering what
it lacked: the C++ type of every such expression, the records with their fields in declaration order, the
overload sets, and the foreign view. Step 12.6 uses them.

The constraint is the compatibility rule of the phase: typed mode must never reject, or write wrong C++ for,
what opaque mode and the C++ compiler accepted; it may accept more where C++ does.

## Decision

1. **A switch, off by default.** `lower.Interop.SetTyped(report, cross)` gives a package's `Interop` the report
   of `typedinterop.Analyze` (with `Decls` and `StringToCStr`). Without it nothing changes: the output is
   byte-identical (the golden tests and the whole opaque corpus are the evidence). The driver sets it when the
   hidden environment variable `GOCPP_TYPED_INTEROP=1` is set (`cmd/gocpp/typedlower.go`); the mode flag
   `-interop=typed|opaque`, the clang-missing message and the default switch are step 12.11. A package of the
   standard library, a build without clang++, a probe that fails, headers that do not compile for the probe, and a
   cross build with a header that is not the C++ library's (the probes read this machine's headers) stay opaque.
2. **Every rule is a refinement, and applies only where the probe answered.** `tvalOf(e)` returns the typed view
   of an expression only when the collector made a reference of that very node, the probe compiled it
   **and clang printed no error for it** (`Result.Recovered`, the low of 12.5), and the foreign view is not
   `Opaque`. Otherwise the opaque lowering runs. A deferred or spawned call's body is lowered with the typed
   member rules off (the frame stores the object, maybe as a pointer to it); only the `->` of a smart pointer
   is written there, through `gp::deref`.
3. **Member access** (`typedMember`): `x.m` for a class, a reference, a smart pointer's own members;
   `gp::nonnull(p)->m` for a raw pointer (Go's nil check kept: a nil `p.m` panics as before, and costs
   what `gp::deref` cost; `GOCPP_NOCHECKS` removes it); `gp::smartnn(x)->m` for a member that the class does
   not have and the class reached by its `operator->` (a chain is followed, four deep) does. The proof that the
   class lacks the member is **the compiler's, not the declarations'**: the extracted members miss using-declarations and
   enumerators (`using Base::x;`, `enum { x }`), so a table lookup that finds nothing proves nothing. The
   probe of the expression that holds the access (the call, else the access itself) must have failed, or been
   recovered by clang, with "no member named m in T" (`noMemberFailure`), and `ftype.Mapper.ArrowMember` must
   say that the pointee could have it (from the declarations, or for `unique_ptr`, `shared_ptr`, `optional` and the
   iterators of the library, whose `operator->` is in internal bases the extraction does not follow, from a
   libstdc++-shaped table). If the class has the name in any way, the probe compiles and the access stays `x.m`
   (`member_shadow.gp`: a using-declaration, an enumerator, a static, a member of a base, of an anonymous
   union, a bit-field, a method, a member template). `gp::smartnn` panics like a nil dereference for an empty
   `unique_ptr`, `shared_ptr` or optional, and passes an iterator through. The interop checker takes the
   same error for such a member as no error (`icheck.arrowMemberError`). `*p` is `*gp::nonnull(p)` for a raw
   pointer and `*x` for a class.
4. **Designated initializers** (`typedFieldsLit`): a keyed literal of an aggregate with a complete record is one
   braced list, the keys sorted by the position of the field (`probe.Record.Fields`, anonymous members
   flattened, bit-fields and unions included, base classes value-initialized, default member initializers
   kept), a nested literal recursively, a Go array literal for a C array field as its elements.
   Go's order of evaluation (source order) holds: when the order changes and two or more values call
   something, each call is a local of a lambda in source order. A literal with two members of one union, a key
   that is not a public field of the record itself, or a non-aggregate stays the opaque lambda of
   assignments. A Go number of another C++ type is converted (`gp::cgo::as<decltype(T::f)>`), because narrowing
   is an error in a braced list.
5. **Go strings as arguments** (`typedStringArgs`): `gp::cstr(s).gp_ptr()` for a `const char*` parameter or a C
   variadic argument, `::std::string(s)` for a `std::string` parameter, only when **every** overload that has
   a parameter at that place has that kind (parameters no string can reach, such as numbers, `char*` and
   `std::wstring`, are no candidates). A mixed set is left to C++, which prefers `std::string_view` for a
   gocpp string as before; a function template, a function pointer and unknown names are left alone.
   The "keeps the pointer" rules (`keeps.go`) are a table (`openlog`, `getopt*`, raylib `InitWindow` and
   `SetWindowTitle`), the SQLite functions that take bytes (`sqlite3_bind_*`, `sqlite3_result_*`) unless the
   destructor is `SQLITE_TRANSIENT`, `curl_easy_setopt` with `CURLOPT_POSTFIELDS`, any call that also gets the
   address of a char pointer (`strtol(s, &end, 10)`, the tail of `sqlite3_prepare_v2`) and a naming convention
   (static, borrow, nocopy, noalloc, unowned); for these the conversion is a user error with the hint to use
   `C.CString`, as opaque mode has always asked. The functions that take a `char*` (`putenv`, `strtok`, `setvbuf`) need
   no rule: a Go string never converts to `char*`. A function that returns a pointer into its argument (`strstr`,
   `strchr`, `basename`, `memmem`, `strptime`, ...) is an error when the result is stored, fine when it is
   compared or passed on in the same statement. **Known gap:** a C++ method or a function of the program's
   own that stores its `const char*` is not known (nothing in the declaration says it): the pointer
   dangles; use `C.CString`. The C string is `gp::cstr`'s: no copy for a literal and its
   substrings, a copy otherwise; a NUL inside the string ends the C string, as with `C.CString`.
6. **C++'s conversions** (`typedScalarOK`, `typedNumberToC`, `typedOperandsOK`): a C++ number or enum into a Go
   number, a Go number into `C.int` and the like, a Go number next to a C++ number or enum (and `+`, `-` with a
   pointer or an iterator) are accepted, the conversion is written as `gp::cgo::as<T>(x)` (Go's `T(x)`, defined
   for every value, where a C++ cast of a float out of range is not), and not written when the two have the
   same class, width and sign. This is what README §2.3 asks (narrowing becomes a `gocpp vet` warning of
   Phase 13).
7. **References**: a `T&` result that is a non-const lvalue is assignable (`v.at(0) = 5`, `v.back()++`) and its
   address can be taken (`&v.at(1)`). A `std::map`'s `operator[]` takes a Go string key when every overload
   wants a `std::string`.
8. **Tests.** `tests/typed` has the programs that typed mode adds, each with a C++ twin (`tests/typed/twin`),
   which `TestTypedTwin` compiles with both compilers: the expected output is C++'s, written from the twin's.
   `TestTypedRun` builds and runs each in typed mode with both compilers, and checks that opaque mode rejects
   (`// opaque: reject`) or equals (`// opaque: same`) it. Windows: `TestTypedWindows` runs the std-only program
   on the real PC.

## Alternatives

- *Write `p->m` for a raw pointer.* Rejected: Go's nil check is observable (`defer_nil_ptr.gp`), and typed mode must
  print what opaque mode prints. `gp::nonnull` is that check, in the place where it is read.
- *Hoist the C string of a string out of loops.* Not done. A `gocpp string` made at run time (a concatenation) is
  not NUL-terminated, so each call copies it, as `C.cstr(s)` in the loop does (a literal and its substrings do
  not, and a string up to 15 bytes does not allocate). Hoisting needs the loop analysis of the vet phase; the
  user's `h := C.cstr(s)` outside the loop is the same code.
- *A second probe of the `->` reading of every member access.* Not needed: clang already says "no member named" for
  the plain reading (and recovers from it), and that is the evidence; the declarations only say whether the pointee could have it.
- *A `T&` parameter bound to a Go variable.* Not done: a Go `int` is an `int64_t` and cannot bind to an `int&`,
  and the write-back of a copy would be a second semantics. `&x` of a C variable and `*C.T` pointers do it.

## Evidence

See the report of the step: the `tests/typed` programs with both compilers and on the real Windows PC; the
corpus of `tests/interop`, `tests/zoo` and `examples/` in typed mode (outputs equal to opaque mode's; the only
differences are the five error tests that exist to show what opaque mode rejects, and typed mode accepts);
mutation checks of each decision.

## Consequences

- Typed code reads like hand-written C++ (`p->v`, `::Rect{.x = 1, .w = 640}`), and the programs of
  `tests/typed` that opaque mode cannot build, build.
- The interop checker (12.5) now assumes this code generation for members through `operator->`: its silence
  there is right only in typed mode, and opaque mode reports the C++ error late, as before.
- Not done in this step (12.11 and later): the mode flag, the clang-missing fallback, the default switch;
  narrowing warnings; `T&` parameters bound to Go lvalues; members of the global values `std.cout.x` (no probe of
  the value alone); the hoisting of C strings out of loops; a typed `x[...]` decision beyond the kind probe's.
