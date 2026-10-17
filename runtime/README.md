# runtime

The gocpp runtime: a C++20 library in `namespace gp`. Generated code includes one header,
`#include <gocpp/rt.hpp>`, and is linked with the runtime's `.cpp` files. The runtime is
embedded in the `gocpp` binary (`go:embed`, ADR-009), so there is nothing extra to install. The
`//go:embed` directive is in `files.go` (package `rtfiles`), and the driver imports that
package: a `//go:embed` pattern can't reach a parent folder, so the directive must be inside
`runtime/`.

This is runtime v0 (Phase 02): the API shape is final, and every later phase grows it.

| Folder or file | What is there |
|---|---|
| `include/gocpp/` | the headers (below). Embedded |
| `src/rt.cpp` | the parts that are not templates: strings, panics, the print format, `run_main`. Standard C++ only. Embedded |
| `src/fmt.cpp` | the parts of the `fmt` shim that are not templates, each a mirror of Go's function of the same name: the formats of the basic kinds with width, precision and flags (`fmt/format.go`), the verbs and error forms and the parsing of a `Printf` format (`fmt/print.go`), and strconv's float formatting and quoting. Embedded |
| `src/isprint.cpp` | `strconv.IsPrint` (for `%q`), generated from Go 1.25's strconv, the Go that gocpp follows (`go test ./runtime -run TestIsPrintTable -update-isprint` in the dev shell). Embedded |
| `src/slice.cpp` | the parts of slices and slice bounds checks that are not templates: the bounds panics with Go's messages, `make`'s panics, and the capacity of a growing slice (Go's formula and size classes). Embedded |
| `src/complex.cpp` | the division of complex numbers, Go's `runtime.complex128div` line by line. Embedded |
| `src/map.cpp` | the parts of maps and key hashing that are not templates: the hash of a run of bytes (strings) and the nil map panic. Embedded |
| `src/type.cpp` | the parts of type descriptors and interface values that are not templates (`type.hpp`, `iface.hpp`): the interning table of composite descriptors, Go's names for composite types, the itab builder and the itab cache (append-only tables, read without a lock). Standard C++ only. Embedded |
| `src/interop.cpp` | the parts of the interop helpers that are not templates: cgo's `C.CString`, `C.GoString`, `C.GoStringN`, `C.GoBytes` and `C.free`, each a mirror of Go's code. Embedded |
| `src/trace.cpp` | the stack trace of a panic (`trace.hpp`): the `panic_error` constructor that records the stack, Go names from mangled C++ names, the DWARF line table reader, the frames. Standard C++ only. Embedded |
| `src/os/linux/` | the Linux OS layer: `os.cpp`, and `trace.cpp` (the stack trace's addresses, ELF file and demangling). Phase 08 adds the Linux side of the thread layer `gp::os`. Embedded |
| `src/os/windows/` | the Windows OS layer (Phase 14): so far only `trace.cpp`, the stack trace's addresses (no file reader yet). Embedded |
| `files.go` | package `rtfiles`: `FS()` holds `include/` and `src/`; `Sources(goos)` lists the `.cpp` files a program is compiled with (`src/*.cpp` and `src/os/<goos>/*.cpp`) |
| `test/` | the C++ tests (not embedded): `unit.cpp`, `slice_test.cpp`, `array_test.cpp`, `map_test.cpp`, `fmt_test.cpp`, `interop_test.cpp`, `cxxvalue_test.cpp`, `type_test.cpp`, `iface_test.cpp`, `diff.cpp`, `fmt_diff.cpp`, `fortify.cpp` and `layout.cpp` (generated) |
| `testdata/diff/main.go`, `testdata/fmtdiff/main.go` | the Go sides of the differential tests |
| `*_test.go` | the Go tests that build and run the C++ tests, and the checks below |
| `compile_flags.txt` | the flags for clangd, so the files open without false errors in an editor (the dev shell's clangd) |

## The headers

| Header | What it has |
|---|---|
| `rt.hpp` | includes everything below, `nofortify.h` first and `sizes.hpp` last |
| `nofortify.h` | turns `_FORTIFY_SOURCE` off at `-O0` (below) |
| `checks.hpp` | the concepts `gp::go_int` and `gp::go_float`, and the panics of the runtime checks with Go's messages: `panic_divide`, `panic_shift`, `panic_index`, `panic_nil`; `gp::check_index`; what `GOCPP_NOCHECKS` (`gocpp build -nochecks`) turns off: the bounds checks here and in `bounds.hpp`, the nil and owner checks of `ptr.hpp` (division and shift checks stay) |
| `string.hpp` | `gp::string` (owner, data pointer, length: 32 bytes), `"..."_gs` literals (static storage, no allocation, no owner: `gp_owner()` gives `gp::static_owner()` for them; the byte after static bytes is readable, a NUL for a literal), `+`, `+=`, `==`, `<=>` (also with a C++ string literal, the C literal of a string constant next to a C/C++ value: `nodes[0].Name == "a"`, Phase 04 step 4.9), the implicit conversion to `std::string_view` (the only implicit one: not to `std::string` or `const char*`, Phase 04 §2.6), `s[i]` with Go's bounds check, `gp::len(s)`; `gp::sub` (`s[a:b]`, `s[a:]`: shares the bytes, Go's slice-bounds panics), `gp::concat` (a chain of `+`, one allocation), `gp::range_string`/`gp::range_next` (`for i, r := range s`, Go's UTF-8 decoding), `gp::rune_to_string` (`string(r)`) |
| `ptr.hpp` | Go's pointers: `gp::ptr<T>` (`*T`: a `std::shared_ptr` with nil checks, 16 bytes; it converts implicitly to a borrowed `T*` for C and C++, nil to `nullptr`; its conversions to `bool` and to `const volatile T*` are deleted, so `if (p)`, `p < q`, `p + 1`, `p[i]` and `delete p` don't compile), `gp::new_<T>` (`new(T)`, `&T{...}`, boxes), the aliasing constructors for pointers into objects (`&p.f`, `&s[i]`), `gp::static_ptr` (`&global`), `gp::ptr_cast` (`(*Q)(p)`), and `gp::weak_ptr<T>` with `gp::weak_make` and `gp::weak_value` (`weak.Pointer[T]`, `weak.Make`, `Value`) |
| `bounds.hpp` | Go's checks for slice expressions in Go's order, for slices, strings and arrays: `gp::check_slice` (`x[lo:hi]`), `gp::check_slice3` (`x[lo:hi:max]`), `gp::check_convert` (`[N]T(s)`), and `gp::panic_bounds` with Go's exact messages (`slice bounds out of range [:5] with capacity 3`) |
| `slice.hpp` | `gp::slice<T>` (`[]T`: owner, data pointer, length, capacity: 40 bytes): nil and empty slices, `s == nullptr`, the implicit conversions to `std::span<T>` and `std::span<const T>` (the `len` elements; nil gives an empty span), `s[i]` with Go's bounds check, `gp::len`, `gp::cap`, `gp::make_slice<T>`, `gp::slice_of<T>({...})` (literals), `gp::sub` (2 and 3 indexes), `gp::append`, `gp::append_slice` (`append(s, t...)`, also a string's bytes), `gp::copy`, `gp::clear`; growth with Go's formula (only `cap >= len` is promised), overlap safe, owners released through `gp::detail::release` |
| `array.hpp` | `gp::array<T, N>` (`[N]T`: an aggregate of N elements, exactly `N * sizeof(T)` bytes; `[0]T` an empty struct of 1 byte): zero values, value semantics, braced literals and `gp::array_keyed<T, N>({{k, v}, ...})` (keyed literals; `constinit` works for positional literals whose elements are constants (numbers, bools, nested arrays of them, `T()` zeros; not `"..."_gs`), and for keyed literals only of numbers, bools and nested arrays of them), `a[i]` with Go's bounds check, `==` and `gp_hash()` (map keys) when `T` is comparable (NaN != NaN), constant `gp::len`/`gp::cap` (also of a pointer to an array), the conversions `gp::to_array<N>(s)` (`[N]T(s)`, a copy) and `gp::to_array_ptr<N>(s)` (`(*[N]T)(s)`, shares the slice's array; needs `-fno-strict-aliasing`), and `gp::sub` on a `gp::ptr<gp::array<T, N>>` (slicing a boxed array: the slice shares the box) |
| `complex.hpp` | Go's `complex64` and `complex128`: `gp::complex64` and `gp::complex128` (`gp::complex<F>`, the real part first), with Go's `+ - * /` (`*` in `float64` without C's corrections, `/` as `runtime.complex128div`), `==`, `++`, the conversions, `gp::real`, `gp::imag`, a `gp_hash()` with the float rules, and the concept `gp::go_complex` |
| `hash.hpp` | the hashing of map keys: `gp::hash_of(k)` and the customization point `gp::hash<T>`; Go's float rules (`+0` and `-0` hash the same; every NaN hashes alike, and the map spreads NaN keys with its own count), strings by their bytes, pointers by address, complex numbers by their parts, and the protocol for structs and arrays: a member `gp_hash()` built with `gp::hash_fields(...)` or `gp::hash_elems(p, n)` (`gp::array` has one); `std::hash` for C++ types. A fixed seed; the order is not randomized (D8), but pointer keys make it differ between runs |
| `map.hpp` | `gp::map<K, V>` (`map[K]V`: a `std::shared_ptr` to a `std::unordered_map`, 16 bytes): nil maps (reads give zero values, writes panic with `assignment to entry in nil map`), `gp::make_map`, `gp::map_of` (literals), `gp::get`, `gp::lookup` (comma-ok), `gp::set`, `gp::ref` (`m[k] op= v`), `gp::erase`, `gp::len`, `gp::clear`, and `gp::range_map` / `gp::range_next` (a snapshot of the entries, which skips the ones deleted meanwhile); reads never insert, NaN keys and the key replacement on a store work as in Go, the table is released through `gp::detail::release` |
| `arith.hpp` | `gp::div`, `gp::rem`, `gp::shl`, `gp::shr` and `gp::f2i<T>`, each returning the Go type |
| `print.hpp` | the builtins `gp::print` and `gp::println`: stderr, in Go's runtime number format (`+1.500000e+000`); C/C++ numbers, enums, raw pointers and C++ strings too (step 4.12) |
| `panic.hpp` | `gp::panic_error` (the exception a panic throws, with the return addresses of its stack) and `gp::panic(v)` |
| `fmtshim.hpp` | `gp::fmtshim::Print`, `Println`, `Printf`, `Sprint`, `Sprintln`, `Sprintf`, a stand-in for `fmt` until Phase 05 and 09: every Phase 03 value as Go's `fmt` prints it (slices, arrays, maps with Go's key order, structs, `&{...}`, addresses, `String()` and `Error()` methods, panics in them), the verbs `%v %+v %d %s %q %t %f` (and `%T`) with width, precision and the flags `- + space 0`, Go's error forms; the protocols that the emitter's types follow (`gp_fields` with `gp::field`, `gp_type_name`, `gp_value()`, `gp_String`/`gp_String_ptr`, `gp_Error`/`gp_Error_ptr`) and the type tags for `int64`, `uint64` and `uintptr` (`gp::fmtshim::typed<tag>(x)`); C/C++ values (step 4.12): C numbers as the Go types of their size, enums as numbers, raw pointers as addresses, C++ strings as strings, other class types through `std::formatter` or `operator<<` |
| `fmtcxx.hpp` | not in `rt.hpp`: generated code with C/C++ includes includes it after them, and it lets `fmt` use a type's `std::formatter` when the program's headers brought in `<format>` (so other programs don't pay for `<format>`) |
| `type.hpp` | Go's type descriptors (Phase 05 §2.2, step 5.1): `gp::type_desc` (Go name, kind, size, the `==` and hash hooks, method tables), one object per Go type chosen by the **Go** type, so `gp::td_int` and `gp::td_int64` are different objects and pointer equality is type identity; the predeclared descriptors (`gp::td_int` … `gp::td_string`, `gp::td_any`), `gp::named<T>` for the named types the emitter writes (with the `gp::shape_*` builders and the two method tables of a named type: `T`'s value methods and `*T`'s all methods), the **interned** composite descriptors (`gp::td_slice`, `td_array`, `td_map`, `td_ptr`, `td_chan` with its direction, `td_func`, `td_struct`, `td_interface`: one pointer per Go type from any thread, with Go's `%T` names), `gp::foreign_td<T>()` for C/C++ values (named after the C++ spelling, `==` and `std::hash` when the type has them), `gp::td_opaque("C.int")` for a C type in a signature, and `gp::method` / `gp::imethod` (a method's name, package path for unexported names, signature text and thunk; sorted) with `gp::method_of<Sig>`, which checks the thunk's C++ type at compile time (`gp::thunk_matches`). The emitted shapes are in the file's comment and in `docs/spec/cpp-mapping.md` ("Interfaces") |
| `iface.hpp` | interface values (Phase 05 §2.3, steps 5.2 and 5.3): `gp::iface` (`{data, type, itab}`, 32 bytes; the base of every generated interface struct; `type` null exactly for `nil`), `gp::any` (`interface{}`), `gp::to_iface<I>(v, descriptor)` (a copy of `v` in a new box; a `gp::ptr` shares its owner, so a nil pointer in an interface is not nil), `gp::unbox<T>`, `gp::icall<Sig>(self, slot, args...)` and `icall_keep` (call the thunk in the itab; a nil interface panics with Go's nil dereference), `gp::itab` and `gp::getitab(inter, type)` (methods matched by name, package and signature text; the results, also the misses, cached in a global table that is read without a lock, and in a static of each conversion site), `gp::missing_method`, `gp::implements`. Conversions between interface types, assertions and `==` come in steps 5.4 to 5.6 |
| `main.hpp` | `gp::run_main(argc, argv, init, main)`, `gp::args()`, `gp::envs()` |
| `interop.hpp` | the helpers for C and C++ values (Phase 04 §2.5, §2.6): `gp::deref(x)` (`x.m` on an opaque value: one level of a raw pointer or a `gp::ptr`, with Go's nil panic; anything else passes through as a reference), `gp::range(c)` and `gp::range_keys(c)` (`for i, v := range c` and `for i := range c` over anything with `begin()`/`end()` and C arrays: a temporary is held by value, random-access containers are walked by index with the length read once, maps give keys and values, the variables are copies), `gp::values(t)` (a pair or tuple with reference members as values, for `std::minmax`), `gp::cstr(s)` (a holder that converts to `const char*`: a NUL-terminated copy, or no copy for a literal), cgo's helpers in `gp::cgo` (`CString`, `GoString`, `GoStringN`, `GoBytes`, `free`, with Go's behavior and panics), the boxes of C/C++ objects that closures and deferred calls use (`gp::cgo::box`, `box_as<T>`, `new_box<T>`: the object is built in the box from its initializer, `renew`, `result_of`, `defer_arg`, `keep`), `gp::cgo::as<T>(x)`, Go's conversions of C/C++ values (`string(x)` of a `std::string` or `std::string_view` copies its bytes; of a C string it is an error that says `C.GoString(p)`), and `gp::cgo::fmt_arg` / `print_arg`, which check at the `.gp` line that `fmt` and `print` can print a C/C++ value. The file starts with the shapes that the emitter writes |
| `result.hpp` | `gp::result_slot<R>`, the variable of an unnamed result of a function with defers (Phase 04 step 4.8; Phase 05 uses it for every result): a pointer for a reference result, the value itself for a type that can be default-constructed and assigned, a buffer and a flag for any other (no default constructor, not assignable); `gp_set(v)` (or `=`) and `gp_take()`; a result type with no move is a `static_assert` at the return statement |
| `os.hpp` | the OS layer's interface (declarations only) |
| `trace.hpp` | not in `rt.hpp`, for the runtime's own files and tests: the stack trace of a panic (Go names from mangled C++ names, the DWARF line table reader, the frames in Go's format) |
| `sizes.hpp` | generated from the size table in `internal/check/sizes.go`: a `static_assert` of the size and alignment of each runtime type, the numbers that `unsafe.Sizeof` gives (divergence D15) |

The headers include only the standard headers they need. Every macro of a header that
`rt.hpp` includes is in every generated file, and a Go name must not be one of them.

The emitter renames such Go names, from the list in `internal/lower/cxx_macros.txt`. So a
header that starts to include a new standard header adds macros to that list: `TestMacroList`
fails, and `go test ./internal/cppgen -run TestMacroList -update` adds them.

What they do, in short:
- **Integers work like Go.** `gp::div` and `gp::rem` panic on a zero divisor, and `MIN / -1` is
  `MIN`. `gp::shl` and `gp::shr` give Go's results for a count of the width or more, and panic
  on a negative count. `gp::f2i<T>` converts a float like Go does on amd64, also out of range.
- **A panic is a C++ exception**, `gp::panic_error`. `gp::run_main` catches it, prints
  `panic: <value>`, a blank line, `goroutine 1 [running]:` and the stack trace to stderr, and
  exits with code 2. The trace (`trace.hpp`) is Go's format: each frame's Go name
  (`main.f(...)`, `main.main()`), a tab, the `.gp` file and line and the offset in the function.
  A `panic_error` records the return addresses when it is made (`gp::os::stack`, glibc's
  `backtrace()`: 1–5 µs on a stack of up to about 25 frames, about 5 µs at its cap of 100),
  because the C++ unwinding then removes the frames. When the 100 fill up, a second walk
  records the whole stack (`gp_all`), so that the trace can show Go 1.25's inner 50, `...N
  frames elided...` and outer 50; its cost grows with the depth, and only deep stacks pay it.
  Only an unrecovered panic reads the program's file (`gp::os::read_exe`: the ELF symbol
  table and the DWARF line table, about 2 ms) and prints the frames, without the runtime's
  own (`gp::`, `std::`, libc, gocpp's helpers); a stripped program (no `.symtab`) gets the
  addresses. The parser of mangled names and the line table reader are standard C++, never
  read outside their data and limit their recursion. Windows (`src/os/windows/`) has the
  addresses but no file reader yet (Phase 14). The accepted differences from Go's trace are
  listed in `docs/spec/cpp-mapping.md` ("Runtime checks and panics"). A nil pointer
  dereference also sets the goroutine's signal (a thread-local value, like Go's `g.sig`), and then the report prints Go's signal
  line after the panic text (after the whole chain of panics, from step 3.13 on):
  `[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x...]`. A recover that
  stops the panic clears it (Phase 05). gocpp's `addr` is always `0x0`. Go's is the faulting
  address: `0x0` for a field at offset 0, a field at 4096 or more, or a whole value, and the
  field's offset for any access, read or write, to a field at a smaller nonzero offset.
- **Memory is reference counted** (ADR-004). Every heap block is owned by a `std::shared_ptr`;
  a pointer into a block (`&p.f`, `&s[i]`) shares the block's owner, so it can't dangle, and a
  pointer to a package-level variable shares a static owner that is never freed (so a weak
  pointer to a global never expires). A non-nil pointer always has an owner (the aliasing
  constructors check it, unless `GOCPP_NOCHECKS` is defined, gocpp's own switch for the
  `-nochecks` build; not `NDEBUG`, which a user's file may define). A cycle is never freed unless a weak
  pointer breaks it. After the last strong pointer to a `gp::new_` value goes, the value is
  destroyed, but its memory stays until the last weak pointer to it goes too (one allocation
  holds the value and the counts).
  The counts are atomic, so pointers can be copied on several threads; one pointer variable
  that one thread writes while another reads it is a data race, as in Go. A copy costs one
  atomic increment and dropping it one decrement; a move, `*p`, `p->f`, `==` and a nil check
  cost none (counted in the `-O2` code of both compilers; the numbers are in `ptr.hpp`).
- **Freeing never recurses deeply.** Freeing the head of a list frees the next node from its
  destructor, and so on. `gp::detail::release`, which `~ptr` calls, counts that nesting per
  thread, and past a depth of 128 it puts the owner on a thread-local list that the outermost
  release frees in a loop, so a list of millions of nodes is freed without running out of
  stack.
  Every runtime type that holds an owner releases it through it too (slices and maps in steps
  3.5 and 3.7, then closure cells, func values and interfaces). The limit is small because a
  goroutine is a thread, whose stack may be small: freeing a deep chain works on a stack of
  96 KB at `-O0`, 32 KB at `-O2` and 192 KB with ASan (measured with `ulimit -s`; the numbers
  are in `ptr.hpp`).
- **No plain-named members.** Every type that holds a Go value (`gp::string`, `gp::ptr`,
  `gp::weak_ptr`, and the slices, maps and others to come) has only constructors, operators
  and `gp_`-prefixed members, public or private, and everything else is a free function
  (`gp::len(s)`, `gp::weak_value(w)`). A Go method or field can then have any name (`get`,
  `len`, `Value`) without clashing with a runtime member, also when a named type is built on a
  runtime type (Phase 03 §2.1, CONTRIBUTING §5). `TestNoPlainMembers` checks every class in
  namespace `gp` (but not in a `detail` namespace) with clang's AST dump, and `unit.cpp` checks
  a list of likely names (the members of `std::shared_ptr` and `std::string`, and common Go
  method names) with `static_assert`s.
- **Output order is Go's.** Nothing is buffered: each `print`, `println` and `fmtshim` call
  formats its text first and writes it with one `write` call, unless the OS takes only part of
  it (then the OS layer writes the rest). Before it writes, it flushes C's `stdout` and
  `stderr` (`gp::detail::flush_c_stdio`, about 3 ns each when empty), so what C code
  `printf`'d before comes first, also into a pipe. `run_main` ends the program through the OS
  layer, which flushes C stdio and exits at once (no C++ static destructors).
- **A C++ exception that nothing catches** ends the program like an unrecovered panic (step
  4.12): `run_main` installs a terminate handler (`gp::detail::on_terminate`, `panic.hpp`), which
  the C++ library calls on the stack of the throw, before anything unwinds; it prints `panic:
  C++ exception <type>: <what()>` and the trace of the throw, and exits with code 2. No deferred
  call runs (Phase 05 makes these exceptions Go panics).
- **Writes behave like Go's.** A non-blocking stdout or stderr that is full makes the program
  wait (`poll`); it doesn't lose the rest. SIGPIPE works as in Go: at start the OS layer
  installs a handler that does nothing (not "ignore", so programs that the process starts get
  the default back), so a write to a broken pipe fails with EPIPE. Then a `fmtshim` write to
  stdout (Go's `os.Stdout`) ends the program with SIGPIPE, also when the parent left SIGPIPE
  ignored; the builtin `print` and `println` ignore the error and the program goes on; a
  write to another file gets the error (`gp::os::write_result::broken_pipe`, for Phase 09).

## Which parts are `.cpp` files

Templates and small functions are in the headers. The rest is in `src/rt.cpp`, so that the
formatting code isn't compiled again in every file of the program, and the OS layer is in
`src/os/<os>/`, so that no OS header reaches generated code. The driver compiles these files
with the program, with the same flags (`rtfiles.Sources`). Phase 06 compiles them once into
its cache.

## The OS rule

The runtime may reach the operating system **only** through `src/os/`: writing to a file
descriptor, exiting, the program's arguments and the environment (`gocpp/os.hpp`). Phase 08 adds
the thread layer `gp::os` there: starting a thread with a stack size, thread-local storage and
the stack-overflow hook. Everything else uses standard C++ (`std::chrono`, `<atomic>`,
`std::filesystem`), so the same code builds on Linux and Windows.

`TestNoOSHeadersOutsideOS` checks this in `make ci`: outside `src/os/`, every file of `runtime/`
(whatever its name, since C++ can include any file; only the Go files, `README.md` and
`compile_flags.txt` are left out) may include only the C++20 standard headers, the standard C
headers and gocpp's own headers. Anything else, such as `<unistd.h>`, `<sys/...>`,
`<pthread.h>` or `<windows.h>`, fails the test. It also reads `%:include`, continuation lines
and comments inside the directive.

A check of `#include` lines can't see everything: glibc's standard headers declare some POSIX
functions anyway (g++ defines `_GNU_SOURCE`, so `<cstdio>` declares `fileno`), and code can
declare an OS function by hand. So `TestWindowsSyntax` also compiles every `.cpp` file outside
`src/os/` for Windows with the dev shell's MinGW-w64 (`-fsyntax-only`), where those don't
exist, and the Windows OS layer's files (`src/os/windows/`, so far only the stack trace's
part; Phase 14 brings the rest). It is skipped when `x86_64-w64-mingw32-g++` isn't installed. Neither check reaches a
POSIX function that also exists on Windows (MinGW has `fileno`): reviews still matter.

## `nofortify.h`

A Nix shell's compiler wrapper turns on "hardening" by default. It defines `_FORTIFY_SOURCE`
after the user's flags, so `-U_FORTIFY_SOURCE` doesn't help, and every file built without
optimization (`-O0`) then gets a warning. `gocpp/nofortify.h` undefines `_FORTIFY_SOURCE` when
the compiler doesn't optimize (`__OPTIMIZE__` is not defined). `rt.hpp` and the runtime's
`.cpp` files include it before anything else, and the driver passes it with `-include` to the
user's own C/C++ files (Phase 04). The project's dev shell turns hardening off, so
`TestNoFortify` turns it on again with `NIX_HARDENING_ENABLE` (nixpkgs' default list) to check
this.

## Tests

`go test ./runtime` (also part of `make test` and `make ci`) runs:
- `TestCXX`: compiles `test/unit.cpp`, `test/slice_test.cpp`, `test/array_test.cpp`, `test/map_test.cpp`, `test/fmt_test.cpp`, `test/trace_test.cpp`, `test/interop_test.cpp`, `test/cxxvalue_test.cpp`, `test/type_test.cpp`, `test/iface_test.cpp`, `test/nochecks_test.cpp`, `test/diff.cpp` and `test/fmt_diff.cpp` with the embedded runtime, with g++
  and clang++, each at `-O0`, at `-O2` and with `-fsanitize=address,undefined` (a sanitizer
  report fails the test; this mode also writes DWARF 4, the others the compilers' DWARF 5).
  The flags are the driver's: `-std=c++20 -fwrapv -fno-strict-aliasing
  -ffp-contract=off -g`, plus `-Wall -Wextra -Werror`, and `-Qunused-arguments` for clang.
  - `unit.cpp` checks every helper: strings, `div`/`rem`/`shl`/`shr` for every integer type
    (`MIN / -1`, shifts by 0, width - 1, width, 200, a negative count), `f2i` at the
    boundaries and for NaN and ±Inf, the print and `fmt` formats, panics, `run_main`, and the
    pointers: nil and `==`, the nil panics (`*p`, `p->f`, `&p->f`, a method through a nil
    pointer) and the goroutine's signal line, ownership (a box outlives its function, an interior
    pointer keeps its whole block), copy and move counts, pointers to globals, weak pointers
    (nil after the last pointer goes, equality after that, a cycle broken with a weak pointer
    and one broken by hand, so LeakSanitizer stays quiet in the asan mode), hashing,
    `ptr_cast`, a recursive type, freeing lists of 200,000 nodes and trees 200,000 nodes deep
    on one side, each of which overflows the stack at `-O0` without `detail::release`
    (nothing is left pending afterwards; `GOCPP_TEST_BIG=1 go test ./runtime` also runs them
    with 2,000,000 nodes, which takes more time and memory than `make ci` should), and the "no
    plain-named members" rule for a list of names.
  - `slice_test.cpp` checks `gp::slice` and the bounds checks: nil and empty slices, moves and
    self-assignment, `make` (zeroed, Go's panics), every bounds message, aliasing (writes and
    appends within capacity seen through other slices, a copy beyond it), append forms
    (self-append, overlapping appends, arguments read first), the capacities against real Go's
    sequences (int64 up to 1,000,000 appends, bytes, 24-byte elements; reallocations counted),
    overlapping copy, clear, element destructors run exactly once, element pointers that keep
    the array alive, elements whose assignment frees the source (`q = append(q[:0],
    q[0].kids...)`), chains of 200,000 nodes through slices of pointers and of values (the
    value chain overflows the stack at `-O0` without `detail::release`), and qualified calls
    (a user's function named like a runtime helper is never called).
  - `array_test.cpp` checks `gp::array`: the layouts (`[0]T` an empty struct), the type's
    rules (an aggregate, `==` only for comparable elements, no ordering), `constinit` literals,
    zero values (also in dirty memory), positional and keyed literals (evaluated in source
    order), value semantics (copies, parameters, nested arrays, arrays of strings, pointers and
    slices), every index panic, `==` with NaN, -0 and nested arrays, element destructors run
    exactly once, a chain of 200,000 nodes through arrays of pointers, the conversions from
    slices (a copy; a pointer that shares and keeps the array; `[0]T` and nil; Go's panics),
    slicing boxed arrays, fields, slice elements and globals (writes seen both ways, the slice
    keeps the block alive, the bounds messages "with length N", the nil panic first), and
    qualified calls.
  - `map_test.cpp` checks `gp::map` and the hashing: nil maps and the panic text, reads that
    never insert, `ref` (`m[k]++`), literals with equal keys, NaN keys (each store a new
    entry, never found, never deleted, visited by a range, removed by `clear`), `+0`/`-0` and
    the key replacement, float32 and complex keys, strings (every length up to a few words),
    pointers (by address, interior pointers too), bools, small integers, enums, `std::hash`
    types, struct and array keys through `gp_hash` (with NaN and `-0` fields), ranges with
    deletes, inserts (with rehashing), re-inserts, `clear` and a reassigned variable during the
    range, maps of maps and of slices, values and keys destroyed exactly once, chains of
    200,000 nodes through maps (by pointer and by value; 50,000 with ASan, which still
    overflows the stack at -O0 without `detail::release`) freed without deep recursion, stores
    that free their own source, qualified calls, `gp::array` keys (`[3]float64` with NaN and
    `-0`, `[0]int`, `[2]string`, nested), and a stress test of a million inserts, lookups and
    deletes (100,000 with ASan; `GOCPP_MAP_BENCH=1` prints the times to stderr, by hand only).
  - `trace_test.cpp` checks the stack trace: the Go names of the mangled names of every shape
    of gocpp's code (functions, methods, templates, lambdas of g++ and clang++, nested ones,
    `init` functions, escaped names and import paths) and the frames left out (the runtime,
    the C++ library, libc, generated helpers), broken and deeply nested names, the trace of a
    real panic (the Go names and the lines of the file, no runtime frame in between, exact at
    `-O0`), stacks of 100, 101 and 300 frames (Go 1.25's inner 50, `...N frames elided...`,
    outer 50; the second walk), a stack known only at its top, frames without a shown name
    (the addresses), and a broken line table (cut short or changed anywhere: no read
    outside it, which ASan checks).
  - `interop_test.cpp` checks `interop.hpp`, `result.hpp` (the slots of every storage, boxes of types that can't be copied, moved or default-constructed) and the conversions at the C/C++ boundary:
    `gp::deref` (raw pointers one level, `gp::ptr` and named pointer types, the nil panics, no
    copy of values and temporaries, smart pointers untouched), `gp::range` and `range_keys`
    (vectors, deques, C arrays, `std::array`, strings, spans, lists, sets, maps, multimaps,
    views with a sentinel, temporaries held until the loop ends (ASan), a `push_back` that
    reallocates in the body, a container that shrinks (Go's index panic), copies counted, the
    blank-variable forms without warnings), `gp::values` (`std::minmax` with temporaries
    (ASan) and with variables, `std::tie`, moves), the conversions (`std::string_view` only:
    overload sets of `std::string` and `const char*` don't take a `gp::string`; spans of nil
    and of part of an array; `T*` but not `bool` from `gp::ptr`, and `==`, `*p`, map keys and
    `fmt` unchanged), `gp::cstr` (no copy for literals and their suffixes and for `string(r)`,
    a copy otherwise, holders that outlive the string, char arrays) and `gp::cgo` (every char
    pointer type, NULs, empty and nil cases, Go's panics). Run with one argument, it runs a case
    that ends the program, which the test compares with what a cgo program printed with Go
    1.25: `C.GoStringN(p, -1)` (fatal "out of memory"), `C.GoStringN(nil, 3)` and
    `C.GoBytes(nil, 3)` (nil dereference), `C.GoBytes(p, -1)` ("gobytes: length out of
    range"), `gp::deref` of a nil pointer, and, with ASan only, `std::minmax`'s references
    without `gp::values`, which ASan must report (stack-use-after-scope).
  - `cxxvalue_test.cpp` checks step 4.12: how `fmt` and `print` print C/C++ values (C numbers
    and their `%T`, enums, raw pointers of every kind and null ones, `nullptr`, C++ strings,
    class types through `operator<<` and `std::formatter`, the formatter first; C/C++ values in
    gocpp structs, slices and pointers; the error forms), the `printable` checks (recursive
    structs end), and the text of C++ exceptions (`std::exception` subclasses, an empty
    `what()`, `throw 42`, a class that is no `std::exception`, the thrown type after a
    rethrow). Run with one argument, it runs a case that ends the program: an uncaught
    `std::out_of_range` (with a deferred call that must not run, and the throwing function in
    the trace), `throw 7`, a C++ exception from a deferred call while a Go panic runs (Go's
    chain), a Go panic through a `noexcept` function, and `printf`, `fmt` and `println` mixed
    in one pipe, in the program's order.
  - `type_test.cpp` checks `type.hpp` (step 5.1): every predeclared descriptor (name, kind, size,
    hooks; `int` and `int64`, and `uint`, `uint64` and `uintptr`, are different objects), Go's
    names for composite types (the texts are what real Go 1.25 prints for `%T`: the three channel
    directions and `chan (<-chan int)`, variadic funcs, structs with tags and embedded fields,
    interfaces with unexported methods), interning (the same Go type is the same pointer,
    `[]int` and `[]int64` are two, every part of a struct field and of a func type counts, a lookup
    adds nothing to the table), named types with their method tables (`*T` gets `T`'s second
    table), the hooks of each kind (pointers by address, arrays elementwise, named types over
    slices, maps and funcs not comparable, a struct without `==`), foreign descriptors (`int`,
    `double`, raw pointers by address, classes with `==` and `std::hash`, with `==` only, with
    neither; a C `int` is not a Go `int32`), opaque descriptors, the order of method tables, every
    kind of descriptor, and 8 threads that intern the same 300 spellings in different orders
    (the table grows while others read it).
  - `iface_test.cpp` checks `iface.hpp` (steps 5.2 and 5.3) with types written as the emitter
    will write them: the nil interface, small and large values (a 64 KiB struct), a box that is
    a copy, pointers that share their owner (an interior pointer, a static one, the owner
    living as long as the interface), **a nil pointer in an interface is not nil**, copies,
    moves and destruction exactly once, chains of 200,000 interfaces that hold interfaces
    freed without deep recursion, itabs (the method sets of `T` and `*T`, slots in the
    interface's order, cache hits build nothing, a miss is cached too, `missing_method`),
    signature texts (`C.int` against `C.double`, one parameter more, the walk over method
    tables), unexported names of two packages, `to_iface` with its per-site cache (two Go types
    over one C++ type), `icall` with every kind of argument and result, a nil interface's
    method, `icall_keep` (a method that drops the interface that is being called, which ASan
    would catch), C/C++ values in an interface, the == and hash hooks on the data of interfaces
    (pointers by target address, boxes, nil pointers, arrays and structs with pointers, no hook for
    slices, maps and funcs), the cases that end the program (run with one argument: an unsorted method
    table, a static conversion that lacks a method, an unbox of nil, two names for one interface
    type, getitab of an interface type: exit code 2 and the message), and `getitab` and `to_iface` from 8 threads at
    once (each of 992 pairs built exactly once). Also run once with ThreadSanitizer by hand.
  - `nochecks_test.cpp` is compiled with `-DGOCPP_NOCHECKS`, as `gocpp build -nochecks` does,
    runtime included: the index, slice, conversion, nil and owner checks are gone (an index
    past `len` inside `cap` reads the element, without undefined behavior), and the division
    and shift panics, Go's results for `MIN / -1` and big shifts, the nil map, `make` and
    `[0]T` panics stay.
  - `diff.cpp` is the same program as `testdata/diff/main.go`. The test runs both, with each
    case, and compares the exit codes and stdout and stderr merged into one stream, up to Go's
    stack trace: number formats (1500 random floats too), integer tables, float → int (on
    amd64 only), strings, pointers (with `weak`), slices (behavior, and every bounds and `make` panic), arrays (value semantics, `==`, conversions, slicing, and their index, conversion and bounds panics),
    maps (behavior that doesn't depend on the order: sorted keys, counts and sums; NaN and
    `-0` keys; ranges with deletes and inserts; the nil map panics), panics (also the nil pointer ones, whose
    signal line is compared with the `pc` left out), the output order and 1 MiB writes.
    Without a `go` command it is skipped.
  - `fmt_diff.cpp` is the same program as `testdata/fmtdiff/main.go`, compared the same way,
    case by case: slices and arrays (nested, nil, 10 levels deep, bytes with `%s` and `%q`,
    a verb for each element), maps (key order of numbers with NaN and ±Inf, strings, bools,
    arrays, structs, named types; nil), structs (`%+v`, unexported, embedded and blank fields),
    pointers (`&{...}` at the top level, `<nil>`, the forms of the addresses), `String()` and
    `Error()` methods (by value and through pointers, nil receivers, unexported fields, promoted
    methods, panics in them, methods that change what is printed), every flag, width and
    precision of the verbs, `%q` of strings and runes (also invalid UTF-8; a hash of `%q` and
    `%+q` of every rune checks `strconv.IsPrint` whole), floats inside composites, the type tags
    in the error forms, `Sprint` spacing, the error forms, and a 100,000-element slice.
  - `fmt_test.cpp` checks what can't be compared with Go: the addresses that pointers print,
    the order of pointer keys, which types count as having `String` and `Error` methods (at
    compile time), the type names of the tags, qualified calls, big widths, and the time of
    `Sprintln` of 100,000 and 1,000,000 ints (printed in its `ok:` line).
  - Without Go: the exact panic output and exit codes, and the start of the stack trace
    (the Go name and the line in `diff.cpp` of the function that panicked); on Linux, 1 MiB into a full
    non-blocking pipe (all of it must arrive) and into a broken pipe, with and without SIGPIPE
    ignored by the parent (`fmtshim::Print` must die of SIGPIPE, `println` must go on; these
    expectations are also checked against Go when it is there); a pointer without an owner,
    which must end the program with a fatal error; and a leak, which LeakSanitizer must
    report in the asan mode.
    Every program ends with `_exit`, which skips ASan's leak check, so `gp::os::exit` runs it
    itself in an ASan build.
- `TestRejected`: C++ uses of the runtime's types that must not compile with gocpp's flags
  for generated code (`build.GeneratedFlags`), each checked at its line with g++ and clang++:
  `delete p` and `p[i]` on a `gp::ptr`, `if (p)`, a `gp::ptr` or a `gp::cstr` holder through a
  C function's `...` (g++ needs `-Werror=conditionally-supported` for these two), and
  `gp::range` over a C array of C arrays (its message), a method table entry or a static conversion
  whose thunk has another C++ type than the interface's slot, an interface struct copied into
  another one, `gp::to_iface` of an interface value, and nil as `nullptr` or `0` (the constructor is explicit); the same code written the emitter's
  way must compile.
- `TestNoFortify`: the `nofortify.h` check above, with `-Werror`.
- `TestLayout`: compiles `test/layout.cpp` (syntax only) with g++ and clang++. Its
  `static_assert`s check the sizes, alignments and field offsets of C++ structs like those the
  emitter writes, against `gocppSizes`; `internal/check` generates it, with `sizes.hpp`
  (`go test ./internal/check -run TestSizesHeader -update-sizes`), and fails when either is
  stale. `TestWindowsSyntax` compiles it, and so `sizes.hpp`, with MinGW-w64 too.
- `TestFSHasEveryFile`: the embedded files are exactly the files of `include/` and `src/`.
- `TestIsPrintTable`: `src/isprint.cpp` says what Go 1.25's `strconv.IsPrint` says for every rune; with another Go it fails and says so.

The tests that walk `runtime/` leave out names that start with `.` (editor and tool files: an
Emacs lock `.#rt.cpp`, clangd's `.cache/`; `//go:embed` leaves them out too) and symbolic links
that point nowhere.
- `TestSources`: `rtfiles.Sources` lists the right `.cpp` files.
- `TestNoPlainMembers`: the "no plain-named members" rule, on every class in namespace `gp`
  (clang++ `-ast-dump=json`); `TestNoPlainMembersFinds` tests the check on a header with every
  kind of member. Skipped without clang++.
- `TestSymbols`: the runtime's object files define no strong global symbol outside namespace
  `gp` (or the `gp_` prefix), checked with `nm`, so a user's C or C++ file can't clash with
  them (an `extern "C"` helper would be a global C name). Weak symbols, the standard library's
  inline functions and templates, are left out: the linker merges them.
- `TestNoOSHeadersOutsideOS` and `TestWindowsSyntax`: the OS rule; `TestCheckIncludes` tests
  the checker itself.
- `TestClangFormat`: the C++ files follow the repo's `.clang-format`
  (`clang-format --dry-run --Werror`). It runs with clang-format 21, the dev shell's, and is
  skipped with another version or none.

Like the test harness, `GOCPP_CXX=clang++` tests with one compiler only, and a compiler that
isn't installed (or that `build.LookCXX` can't use) is a skip, unless `GOCPP_REQUIRE_CXX`
names it (`make ci` sets `GOCPP_REQUIRE_CXX=g++,clang++`).

The tests compile in parallel: the programs of one compiler and mode are built at the same
time, each against the runtime's object files, which are built once for the mode, and a check
runs as soon as its program is built. A program that doesn't build fails its mode even when no
check runs it. The compiles and runs that go through the test helpers are bounded: at most
`GOCPP_TEST_JOBS` of them run at the same time in the test binary (default: the number of CPUs,
but at most 48). The few processes that the pipes tests start themselves are outside that bound.
The bound is also the memory bound: 48 commands at once peak at about 15 GB, and the largest
compiler process takes about 0.8 GB.
