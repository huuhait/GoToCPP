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
| `src/fmt.cpp` | Go's `fmt` (ADR-028), each function a mirror of Go's function of the same name: the formats of the basic kinds with width, precision and flags (`fmt/format.go`), the printer that walks a value by its type descriptor, the verbs, methods, panics and error forms, the parsing of a `Printf` format and `Errorf` (`fmt/print.go`, `fmt/errors.go`), internal/fmtsort, and strconv's float formatting and quoting. Embedded |
| `src/isprint.cpp` | `strconv.IsPrint` (for `%q`), generated from Go 1.25's strconv, the Go that gocpp follows (`go test ./runtime -run TestIsPrintTable -update-isprint` in the dev shell). Embedded |
| `src/slice.cpp` | the parts of slices and slice bounds checks that are not templates: the bounds panics with Go's messages, `make`'s panics, and the capacity of a growing slice (Go's formula and size classes). Embedded |
| `src/complex.cpp` | the division of complex numbers, Go's `runtime.complex128div` line by line. Embedded |
| `src/map.cpp` | the parts of maps and key hashing that are not templates: the hash of a run of bytes (strings) and the nil map panic. Embedded |
| `src/type.cpp` | the parts of type descriptors and interface values that are not templates (`type.hpp`, `iface.hpp`): the interning table of composite descriptors, Go's names for composite types, the itab builder and the itab cache (append-only tables, read without a lock). Standard C++ only. Embedded |
| `src/unsafe.cpp` | the panics of `unsafe.Slice` and `unsafe.String` and the checkptr fatal error (`unsafe.hpp`). Embedded |
| `src/interop.cpp` | the parts of the interop helpers that are not templates: cgo's `C.CString`, `C.GoString`, `C.GoStringN`, `C.GoBytes` and `C.free`, each a mirror of Go's code. Embedded |
| `src/panicval.cpp` | the values of panics (step 5.10): the `runtime.Error` types (`runtime.errorString`, `boundsError`, `plainError`, `*TypeAssertionError`, `*PanicNilError`) and `runtime.CppException` with their descriptors and method tables, the text an unrecovered panic prints for a value (Go's `printpanicval`), `gp::panic` of an interface value and of `nil`, `gp::goexit`. Standard C++ only. Embedded |
| `src/trace.cpp` | the stack trace of a panic (`trace.hpp`): the `panic_error` constructor that records the stack, Go names from mangled C++ names, the DWARF line table reader, the frames. Standard C++ only. Embedded |
| `src/goroutine.cpp` | the goroutine runtime (Phase 08, steps 8.1 and 8.2): the threads, the ids and the count, the end of a goroutine that panics, `claim_death` (one report when several goroutines die at once), and the reuse of threads: when a goroutine ends, its thread waits (up to 256 threads, 5 seconds each) for the next `go` statement instead of exiting, so 1,000,000 short goroutines in a loop need a few hundred threads. Under AddressSanitizer a thread never ends by itself (LeakSanitizer can't stop a thread that is exiting). Standard C++ only; the OS is `gp::os`. Embedded |
| `src/waiter.cpp` | `gp::waiter` (a mutex, a condition variable and the permit). Embedded |
| `src/chan.cpp`, `src/chan_core.hpp` | channels (Phase 08, steps 8.3 and 8.4): the channel object, Go's `hchan` (a mutex, a ring buffer and two FIFO queues of waiting goroutines, `sudog`s on their stacks), `send`, `recv`, `close`, the non-blocking forms, `send_locked` and `recv_locked` (the poll of one channel under its lock: the code of both the plain operations and `select`) and the entries that `select` queues; the header `chan_core.hpp` is internal (not in `include/`). Embedded |
| `src/select.cpp` | the `select` statement (Phase 08, step 8.5): Go's `selectgo` on the channel objects: a random poll order (a thread-local xorshift64*), the lock order by channel address (a channel in two cases is locked once), the first ready case, the default, and otherwise one `sudog` for each case with a shared claim flag and wake cell, `block_scope("select")`, park, and the clean-up of the entries that did not fire. Up to 16 cases need no allocation. Standard C++ only. Embedded |
| `src/os/linux/` | the Linux OS layer: `os.cpp`, `thread.cpp` (the thread layer of Phase 08: detached pthreads with a stack size and a guard, the thread's slot, the stack bounds, the SIGSEGV handler on each thread's alternate stack that calls the stack-overflow hook), `linux.hpp` (shared by the two) and `trace.cpp` (the stack trace's addresses, ELF file and demangling). Embedded |
| `src/os/windows/` | the Windows OS layer (Phase 14): so far only `trace.cpp`, the stack trace's addresses (no file reader yet); the thread layer's Windows version (`CreateThread`, a vectored exception handler for the stack overflow) comes with it. Embedded |
| `files.go` | package `rtfiles`: `FS()` holds `include/` and `src/`; `Sources(goos)` lists the `.cpp` files a program is compiled with (`src/*.cpp` and `src/os/<goos>/*.cpp`) |
| `test/` | the C++ tests (not embedded): `unit.cpp`, `slice_test.cpp`, `array_test.cpp`, `map_test.cpp`, `fmt_test.cpp`, `thread_test.cpp`, `goroutine_test.cpp`, `chan_test.cpp`, `select_test.cpp`, `interop_test.cpp`, `cxxvalue_test.cpp`, `unsafe_test.cpp`, `type_test.cpp`, `iface_test.cpp`, `ifaceconv_test.cpp`, `diff.cpp`, `fortify.cpp` and `layout.cpp` (generated), and the header `threads_ended.hpp` (the test programs that start threads wait with it until the threads have left `/proc/self/task` before they end: a thread that just ended can make LeakSanitizer's stop-the-world at exit print "Unable to get registers from thread") |
| `testdata/diff/main.go` | the Go side of the differential test |
| `*_test.go` | the Go tests that build and run the C++ tests, and the checks below |
| `compile_flags.txt` | the flags for clangd, so the files open without false errors in an editor (the dev shell's clangd) |

## The headers

| Header | What it has |
|---|---|
| `rt.hpp` | includes everything below, `nofortify.h` first and `sizes.hpp` last |
| `nofortify.h` | turns `_FORTIFY_SOURCE` off at `-O0` (below) |
| `checks.hpp` | the concepts `gp::go_int` and `gp::go_float`, and the panics of the runtime checks with Go's messages: `panic_divide`, `panic_shift`, `panic_index`, `panic_nil`; `gp::check_index`; what `GOCPP_NOCHECKS` (`gocpp build -nochecks`) turns off: the bounds checks here and in `bounds.hpp`, the nil and owner checks of `ptr.hpp` (division and shift checks stay) |
| `string.hpp` | `gp::string` (owner, data pointer, length: 32 bytes), `"..."_gs` literals (static storage, no allocation, no owner: `gp_owner()` gives `gp::static_owner()` for them; the byte after static bytes is readable, a NUL for a literal), `+`, `+=`, `==`, `<=>` (also with a C++ string literal, the C literal of a string constant next to a C/C++ value: `nodes[0].Name == "a"`, Phase 04 step 4.9), the implicit conversion to `std::string_view` (the only implicit one: not to `std::string` or `const char*`, Phase 04 §2.6), `s[i]` with Go's bounds check, `gp::len(s)`; `gp::sub` (`s[a:b]`, `s[a:]`: shares the bytes, Go's slice-bounds panics), `gp::concat` (a chain of `+`, one allocation), `gp::range_string`/`gp::range_next` (`for i, r := range s`, Go's UTF-8 decoding), `gp::rune_to_string` (`string(r)`) |
| `ptr.hpp` | Go's pointers: `gp::ptr<T>` (`*T`: a `std::shared_ptr` with nil checks, 16 bytes; it converts implicitly to a borrowed `T*` for C and C++, nil to `nullptr`; its conversions to `bool` and to `const volatile T*` are deleted, so `if (p)`, `p < q`, `p + 1`, `p[i]` and `delete p` don't compile), `gp::new_<T>` (`new(T)`, `&T{...}`, boxes), the aliasing constructors for pointers into objects (`&p.f`, `&s[i]`), `gp::static_ptr` (`&global`), `gp::foreign_owner()` (the never-freed owner of memory that C owns), `gp::cptr<T>` (Phase 04 step 4.10: a gocpp pointer to a C/C++ type, `*lib.T` declared in gocpp code, and what `&T{...}`, `&s.f` and `&xs[i]` give: a `gp::ptr<T>` that a raw `T*` converts to, with the owner `foreign_owner()`, and that converts to `T*`), `gp::ptr_cast` (`(*Q)(p)`), and `gp::weak_ptr<T>` with `gp::weak_make` and `gp::weak_value` (`weak.Pointer[T]`, `weak.Make`, `Value`) |
| `bounds.hpp` | Go's checks for slice expressions in Go's order, for slices, strings and arrays: `gp::check_slice` (`x[lo:hi]`), `gp::check_slice3` (`x[lo:hi:max]`), `gp::check_convert` (`[N]T(s)`), and `gp::panic_bounds` with Go's exact messages (`slice bounds out of range [:5] with capacity 3`) |
| `slice.hpp` | `gp::slice<T>` (`[]T`: owner, data pointer, length, capacity: 40 bytes): nil and empty slices, `s == nullptr`, the implicit conversions to `std::span<T>` and `std::span<const T>` (the `len` elements; nil gives an empty span), `s[i]` with Go's bounds check, `gp::len`, `gp::cap`, `gp::make_slice<T>`, `gp::slice_of<T>({...})` (literals), `gp::sub` (2 and 3 indexes), `gp::append`, `gp::append_slice` (`append(s, t...)`, also a string's bytes), `gp::copy`, `gp::clear`; growth with Go's formula (only `cap >= len` is promised), overlap safe, owners released through `gp::detail::release` |
| `array.hpp` | `gp::array<T, N>` (`[N]T`: an aggregate of N elements, exactly `N * sizeof(T)` bytes; `[0]T` an empty struct of 1 byte): zero values, value semantics, braced literals and `gp::array_keyed<T, N>({{k, v}, ...})` (keyed literals; `constinit` works for positional literals whose elements are constants (numbers, bools, nested arrays of them, `T()` zeros; not `"..."_gs`), and for keyed literals only of numbers, bools and nested arrays of them), `a[i]` with Go's bounds check, `==` and `gp_hash()` (map keys) when `T` is comparable (NaN != NaN), constant `gp::len`/`gp::cap` (also of a pointer to an array), the conversions `gp::to_array<N>(s)` (`[N]T(s)`, a copy) and `gp::to_array_ptr<N>(s)` (`(*[N]T)(s)`, shares the slice's array; needs `-fno-strict-aliasing`), and `gp::sub` on a `gp::ptr<gp::array<T, N>>` (slicing a boxed array: the slice shares the box) |
| `complex.hpp` | Go's `complex64` and `complex128`: `gp::complex64` and `gp::complex128` (`gp::complex<F>`, the real part first), with Go's `+ - * /` (`*` in `float64` without C's corrections, `/` as `runtime.complex128div`), `==`, `++`, the conversions, `gp::real`, `gp::imag`, a `gp_hash()` with the float rules, and the concept `gp::go_complex` |
| `hash.hpp` | the hashing of map keys: `gp::hash_of(k)` and the customization point `gp::hash<T>`; Go's float rules (`+0` and `-0` hash the same; every NaN hashes alike, and the map spreads NaN keys with its own count), strings by their bytes, pointers by address, complex numbers by their parts, and the protocol for structs and arrays: a member `gp_hash()` built with `gp::hash_fields(...)` or `gp::hash_elems(p, n)` (`gp::array` has one); `std::hash` for C++ types. A fixed seed; the order is not randomized (D8), but pointer keys make it differ between runs |
| `map.hpp` | `gp::map<K, V>` (`map[K]V`: a `std::shared_ptr` to a `std::unordered_map`, 16 bytes): nil maps (reads give zero values, writes panic with `assignment to entry in nil map`), `gp::make_map`, `gp::map_of` (literals), `gp::get`, `gp::lookup` (comma-ok), `gp::set`, `gp::ref` (`m[k] op= v`), `gp::erase`, `gp::len`, `gp::clear`, and `gp::range_map` / `gp::range_next` (a snapshot of the entries, which skips the ones deleted meanwhile); reads never insert, NaN keys and the key replacement on a store work as in Go, the table is released through `gp::detail::release` |
| `arith.hpp` | `gp::div`, `gp::rem`, `gp::shl`, `gp::shr` and `gp::f2i<T>`, each returning the Go type |
| `print.hpp` | the builtins `gp::print` and `gp::println`: stderr, in Go's runtime number format (`+1.500000e+000`); C/C++ numbers, enums, raw pointers and C++ strings too (step 4.12) |
| `panic.hpp` | `gp::panic_error` (the exception a panic throws: the value as a `gp::any`, the text, `[recovered]`, the chain, the return addresses of its stack), `gp::panic(v)` (basic values, any interface value, `nil`), `gp::goexit_signal` and `gp::goexit()` (`runtime.Goexit`), `gp::CppException` (the value of a C++ exception that became a panic), and the runtime errors' makers (`detail::throw_runtime`, `type_assertion_error`; step 5.10) |
| `aware.hpp` | how `recover()` finds out that it was called directly by a deferred call (step 5.10): the ticket (`gp::take_defer_ctx`, `gp::defer_ctx`), the registry of the code of aware functions (`gp::aware`, `detail::aware_code`) that `gp::func` and `gp::icall` look at |
| `fmt.hpp`, `typeops.hpp` | Go's package `fmt` (step 5.11, [ADR-028](../docs/adr/028-fmt-on-descriptors.md)): `gp::fmt::Print`, `Println`, `Printf`, `Sprint*`, `Fprint*`, `Append*`, `Errorf`, `FormatString`, and the interfaces `fmt.Stringer`, `GoStringer`, `Formatter` and `State`. A port of Go 1.25's `fmt/print.go` and `format.go` (`src/fmt.cpp`) that walks values by their type descriptors: every verb, flag, width, precision and argument index, `%v %+v %#v %T`, maps sorted as `internal/fmtsort`, `Format`, `GoString`, `Error` and `String` methods (panics in them caught), Go's error forms; plain operands (`bool`, numbers, strings, interface values) need no descriptor, the others are `gp::fmt::as(descriptor, value)`; C/C++ values (step 4.12): C numbers as the Go types of their size, enums as numbers, raw pointers as addresses, C++ strings as strings, other classes through `std::formatter` or `operator<<` (registered by `gp::cgo::fmt_arg`). `typeops.hpp` is the part of the descriptors that reads values (`type_ops`: slice data, map snapshot, pointer target, foreign classes) |
| `reflect.hpp` | the native functions of `stdlib/reflect` (step 9.3, [ADR-035](../docs/adr/035-reflect-subset-on-descriptors.md); `src/reflect.cpp`): types by descriptor address (kind, name, fields with tags and offsets, elem, key, len, parameters, methods, `Implements`, `AssignableTo`, `ConvertibleTo`), values as a type and the storage of a C++ object (read, write, `New`, `Zero`, slices, maps, `Convert`, `Equal`), and `swapper`. `typeops.hpp` holds what they need beside `fmt`'s operations: `gp_trivial`, `gp_ctor`, `gp_dtor`, `gp_share` and `gp_refl` (`slice_refl`, `ptr_refl`, `map_refl`, `chan_refl`) |
| `fmtcxx.hpp` | not in `rt.hpp`: generated code with C/C++ includes includes it after them, and it lets `fmt` use a type's `std::formatter` when the program's headers brought in `<format>` (so other programs don't pay for `<format>`) |
| `type.hpp` | Go's type descriptors (Phase 05 §2.2, step 5.1): `gp::type_desc` (Go name, kind, size, the `==` and hash hooks, method tables), one object per Go type chosen by the **Go** type, so `gp::td_int` and `gp::td_int64` are different objects and pointer equality is type identity; the predeclared descriptors (`gp::td_int` … `gp::td_string`, `gp::td_any`), `gp::named<T>` for the named types the emitter writes (with the `gp::shape_*` builders and the two method tables of a named type: `T`'s value methods and `*T`'s all methods), the **interned** composite descriptors (`gp::td_slice`, `td_array`, `td_map`, `td_ptr`, `td_chan` with its direction, `td_func`, `td_struct`, `td_interface`: one pointer per Go type from any thread, with Go's `%T` names), `gp::foreign_td<T>()` for C/C++ values (named after the C++ spelling, `==` and `std::hash` when the type has them), `gp::td_opaque("C.int")` for a C type in a signature, and `gp::method` / `gp::imethod` (a method's name, package path for unexported names, signature text and thunk; sorted) with `gp::method_of<Sig>`, which checks the thunk's C++ type at compile time (`gp::thunk_matches`). The emitted shapes are in the file's comment and in `docs/spec/cpp-mapping.md` ("Interfaces") |
| `iface.hpp` | interface values (Phase 05 §2.3, steps 5.2 and 5.3): `gp::iface` (`{data, type, itab}`, 32 bytes; the base of every generated interface struct; `type` null exactly for `nil`), `gp::any` (`interface{}`), `gp::to_iface<I>(v, descriptor)` (a copy of `v` in a new box; a `gp::ptr` shares its owner, so a nil pointer in an interface is not nil), `gp::unbox<T>`, `gp::icall<Sig>(self, slot, args...)` and `icall_keep` (call the thunk in the itab; a nil interface panics with Go's nil dereference), `gp::itab` and `gp::getitab(inter, type)` (methods matched by name, package and signature text; the results, also the misses, cached in a global table that is read without a lock, and in a static of each conversion site), `gp::missing_method`, `gp::implements`. Conversions between interface types, assertions and `==` come in steps 5.4 to 5.6 |
| `ifaceconv.hpp` | conversions between interface types, type assertions and the helpers of type switches (Phase 05 steps 5.4 and 5.5): `gp::iface_cast<I>(x)` (the itab from a one-entry cache of the template instance, then the global one; nil stays nil), `gp::assert_type<T>(x, descriptor)` and `assert_type2` (a `const T&` into the box; the comma-ok form is a `std::tuple<T, bool>`), `gp::assert_iface<I>(x)` and `assert_iface2`, `gp::type_is(x, descriptor)` and `gp::type_has<I>(x)`; a failed assertion panics with the text of Go's `*runtime.TypeAssertionError` (`runtime/src/ifaceconv.cpp`); also `iface`'s `==` (same dynamic type and equal values; `comparing uncomparable type []int`) and `gp_hash` (`hash of unhashable type`), in the same source |
| `error.hpp` | `gp::error`, Go's predeclared `error` as an interface struct (`Error()` through the itab) and its descriptor `gp::td_error` |
| `chan.hpp` | channels (Phase 08, steps 8.3 and 8.4): `gp::chan<T>` (`chan T`, `<-chan T`, `chan<- T`: one C++ type, a `std::shared_ptr` to the channel object, 16 bytes; a nil channel is an empty handle; `==` compares the channel, `gp_hash()` and `gp_address()` for map keys and printing), `gp::make_chan<T>(n)`, `gp::send`, `gp::recv`, `gp::recv2` (`v, ok := <-c`), `gp::recv_drop`, `gp::close`, `gp::len`, `gp::cap`, and the range loop `gp::range_chan(c)` / `gp::range_chan_drop(c)` with `gp::range_next` (step 8.6); the panics have Go's texts (`send on closed channel`, `close of closed channel`, `close of nil channel`, `makechan: size out of range`); a nil channel blocks the goroutine for ever. The direction is in the type descriptor (`gp::td_chan`), not in the C++ type |
| `select.hpp` | the `select` statement and `range` over a channel's companions (Phase 08, step 8.5): the case objects `gp::sel_recv gp_s{c}` (`case v := <-c`; `gp_take()`, `gp_ok()`; `gp::sel_drop` for a value that is not used), `gp::sel_send gp_s{c, v}`, `gp::sel_default`, and `gp::select(cases...)`, which returns the position of the case that happened (uniformly random among the ready ones, Go's poll order); `gp::select()` is `select {}`; `gp::select_unreachable()` follows a select whose cases all jump away. A channel that is a local variable is held by reference, any other by a copy of the handle. [ADR-029](../docs/adr/029-select-and-range-over-channels.md) |
| `main.hpp` | `gp::run_main(argc, argv, init, main)`, `gp::args()`, `gp::envs()` |
| `interop.hpp` | the helpers for C and C++ values (Phase 04 §2.5, §2.6): `gp::deref(x)` (`x.m` on an opaque value: one level of a raw pointer or a `gp::ptr`, with Go's nil panic; anything else passes through as a reference), `gp::range(c)` and `gp::range_keys(c)` (`for i, v := range c` and `for i := range c` over anything with `begin()`/`end()` and C arrays: a temporary is held by value, random-access containers are walked by index with the length read once, maps give keys and values, the variables are copies), `gp::values(t)` (a pair or tuple with reference members as values, for `std::minmax`), `gp::cstr(s)` (a holder that converts to `const char*`: a NUL-terminated copy, or no copy for a literal), cgo's helpers in `gp::cgo` (`CString`, `GoString`, `GoStringN`, `GoBytes`, `free`, with Go's behavior and panics), `gp::cgo::as<T>(x)`, Go's conversions of C/C++ values (`string(x)` of a `std::string` or `std::string_view` copies its bytes; of a C string it is an error that says `C.GoString(p)`; also to `unsafe.Pointer`, and from it to a C pointer or a gocpp pointer), `gp::cgo::slice_of(a)` (`a[lo:hi]` of a C array: a slice over C memory), `gp::cgo::out_ptr` (the `void**` or `T**` argument for the address of an `unsafe.Pointer` or `*C.T` variable: written back when the statement ends), `gp::cgo::raw(x)` (a gocpp pointer as its raw address, for a C function's arguments), `gp::cgo::box_keep<T>(make, kept...)` (`&lib.Node{next: head}`: the box of a new object, built in place, that holds the owners of the pointers in the literal), `gp::cgo::store_keep(owner, member, value, keep_path<...>)` (`n.next = head`: stores the raw address into the member and records `head`'s owner in `n`'s box under the member's address, replacing the member's old entry; `keep_path` rejects a store through a raw pointer member; for a struct without a keeper of its own, a C struct in a gocpp struct or a slice, the owners are kept in a table by the owner's block), `gp::cgo::load(x)` (`next := cur.next`: a raw pointer to a C struct becomes a `gp::cptr` that owns it), and in `ptr.hpp` `gp::raw_owned(raw)` with the registry of the structs that `box_keep` made, by address (`gp::detail::box_owner`): a raw `T*` that converts to a `gp::cptr` finds the owner of the struct it points to, else it owns nothing (`foreign_owner`), and `gp::cgo::fmt_arg` / `print_arg`, which check at the `.gp` line that `fmt` and `print` can print a C/C++ value. The file starts with the shapes that the emitter writes |
| `callback.hpp` | gocpp funcs that C and C++ call (Phase 04 step 4.11): `gp::cgo::callback<R(A...), F>`, what generated code passes where a func goes to a C/C++ place (`gp::cgo::fn<&f>()` for a function of the program, `gp::cgo::fn_lit<gp::func<Sig>>(lambda)` for a func literal without captures, `gp::cgo::fn_val(f)` for any other func value). It is a callable (a C++ template, `std::function` and `gp::func` take it, a panic unwinds through those C++ frames), and it converts to a C function pointer when it has no state: a static noexcept trampoline per function (arguments converted from C: `void*` to `unsafe.Pointer` owning nothing, `T*` to `gp::cptr<T>`; a result pointer into gocpp memory is a fatal error). A panic or C++ exception that leaves the trampoline ends the program (std::terminate, before any C frame unwinds; `on_terminate` prints the report). A closure with captures, a func value known only at run time, a variadic target and a signature mismatch are `static_assert` errors with messages for users |
| `rangefunc.hpp` | the state of a range-over-func loop (Phase 07, step 7.8; [ADR-033](../docs/adr/033-range-over-func.md)): `gp::rangefunc_state` (Go's rewrite: the states DONE, READY, PANIC and EXHAUSTED in `gp_state`, `gp_next`, the jump code of the loop nest; `gp_enter()` at the start of the loop body, `gp_continue()` and `gp_exit()` where it ends, `gp_finish()` after the iterator returned) and `gp::panic_range_state(state)`, the four runtime errors (`range function continued iteration after function for loop body returned false`, `... after whole loop exit`, `... after loop body panic`, `range function recovered a loop body panic and did not resume panicking`). The generated closure of a loop body keeps the state alive through a `gp::ptr`, so a `yield` called after the loop panics instead of reading a dead frame. The checks stay on with `-nochecks` |
| `result.hpp` | `gp::result_slot<R>`, the variable of an unnamed result of a function with defers (Phase 04 step 4.8; Phase 05 uses it for every result): a pointer for a reference result, the value itself for a type that can be default-constructed and assigned, a buffer and a flag for any other (no default constructor, not assignable); `gp_set(v)` (or `=`) and `gp_take()`; a result type with no move is a `static_assert` at the return statement |
| `unsafe.hpp` | package `unsafe` (Phase 04 step 4.10, D16): `unsafe.Pointer` as the owning `gp::ptr<void>`; `gp::unsafe_pointer(p)` (`unsafe.Pointer(p)`), `gp::unsafe_cast<T>(p)` (`(*T)(p)`, with the checkptr alignment check), `gp::unsafe_addr` and `gp::unsafe_from_addr` (`uintptr`), `gp::unsafe_add`, `gp::unsafe_slice_data`, `gp::unsafe_slice` (a gocpp pointer, or a raw C pointer: a view of C memory), `gp::unsafe_sizeof<T>()` and `gp::unsafe_alignof<T>()` (`unsafe.Sizeof`, `Alignof` of a C/C++ value), `gp::unsafe_string_data` and `gp::unsafe_string` (a gocpp byte pointer or a raw char pointer), each sharing the owner of what it came from, and Go's panics ("unsafe.Slice: len out of range"); `gp::foreign_owner()`, the never-freed owner of memory that C owns (non-null stored pointer, so a string over C memory is no "no owner" string); the conversions of C values (`gp::cgo::as<gp::ptr<void>>(cp)`, a C function through `std::bit_cast`) are in `interop.hpp` |
| `os.hpp` | the OS layer's interface (declarations only): the files and the process, the stack trace's addresses, and the thread layer (`thread_start` with a stack size, `tls_get`/`tls_set`, `stack_bounds`, `set_stack_overflow_hook`, `exit_now`) |
| `waiter.hpp` | `gp::waiter`, how a goroutine blocks: one permit, `gp_park` (also `gp_park_for`, `gp_park_until`) and `gp_unpark`; an unpark before the park is not lost, unparks don't add up, and the caller loops on its own condition. `gp_unpark_with(f)` and `gp_wait(pred)` (step 8.3) are the pair a waker uses to say "done" to a parked goroutine: `f` runs under the waiter's lock, `pred` is read under it, so the parked goroutine can destroy what the waker wrote to as soon as `gp_wait` returns. The mutex and the condition variable are in `src/waiter.cpp`: the header has only a pointer to the state, and the timed calls are templates that need no `<chrono>`, because `<chrono>`, `<mutex>` and `<condition_variable>` would add 0.8 s to the compile of every generated file (and 60 names to `cxx_macros.txt`). Same for `goroutine.hpp` and `chan.hpp`: no `<atomic>`, no `<mutex>` |
| `goroutine.hpp` | goroutines (steps 8.1 and 8.2): `gp::go(f)`, `gp::go_fn(f, args...)` and `gp::go_call(callable, args...)` (what a `go` statement is: the function and the arguments are the calling goroutine's values, stored in the new goroutine's closure; `go f()` with a nil func value is the fatal error `go of nil func value`), `gp::num_goroutine()`, `gp::num_blocked()`, and in `detail` the goroutine's record (`this_goroutine`, its id, its own waiter, why it is blocked), `stack_size`/`set_stack_size`, `go_start` with a stack size, `block_scope` (what a blocking operation wraps its park in, for the deadlock detector of step 8.11), `set_idle_limit`/`stop_idle_threads` (thread reuse, below) and `claim_death` |
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
- **Output order is Go's.** Nothing is buffered: each `print`, `println` and `fmt` call
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
  the default back), so a write to a broken pipe fails with EPIPE. Then a `fmt` write to
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
descriptor, exiting, the program's arguments and the environment (`gocpp/os.hpp`), and the
thread layer of Phase 08: starting a detached thread with a stack size (`thread_start`), the
thread's slot (`tls_get`, `tls_set`), the stack bounds and the stack-overflow hook. Everything
else uses standard C++ (`std::chrono`, `<atomic>`, `std::filesystem`, `std::mutex`,
`std::condition_variable`), so the same code builds on Linux and Windows.

The thread layer, in short (`os.hpp` has the contract, `src/os/linux/thread.cpp` the Linux version):
- **Goroutines are threads** (ADR-007; the goroutine runtime is step 8.1), started by `os::thread_start(fn, arg, stack_size)`:
  detached, with the stack size rounded up (the OS has a minimum and may reuse a bigger cached
  stack), a guard of 256 KiB below the stack (a frame that is probed page by page, as every
  frame of the compiles of gocpp is with `-fstack-clash-protection`, faults there and not in
  another thread's stack; without the flag a frame of more than 256 KiB can skip over the guard,
  and the first write may land in a neighbor's stack without a fault), and a 64 KiB alternate signal stack that is extra, not part of
  the size asked for. It returns 0 or the OS's error code.
- **The stack-overflow hook.** The OS layer finds the overflow (a fault in the guard or near the
  end of the thread's own stack, with the stack pointer near it too: other faults, and threads
  that the runtime didn't start, keep the old action, the crash) and calls the hook that
  `run_main` sets (`on_stack_overflow` in `src/rt.cpp`: Go's three lines, exit code 2). The hook
  is portable code that only formats numbers and calls `os::write` and `os::exit_now`, the
  async-signal-safe parts of the interface, so the Windows version (a vectored exception
  handler, Phase 14) needs only the detection. A sanitizer build has no handler of the
  runtime's (AddressSanitizer reports the overflow itself, `stack-overflow`; ThreadSanitizer
  only crashes on a goroutine's stack).
  The report is Go's three lines, then a blank line and `goroutine N [running]:` (when the thread
  is a goroutine), without a trace and without Go's `runtime stack:` section. It does not flush
  C's stdio (it runs in a signal handler): what C code printf'd and has not flushed is lost.
  When several threads die at once (`claim_death`, `goroutine.hpp`), one reports and the others
  wait for the process to end, for at most 30 seconds: then they end it with code 2, so that a
  reporter that is stuck (it needs malloc's lock, which a waiting thread's own overflow
  interrupted) can't hang the program.
- **`no_core_dump()`** (`PR_SET_DUMPABLE 0`): the tests that die of SIGSEGV on purpose call it,
  because with a `core_pattern` that pipes to systemd-coredump a dying process is held for
  minutes on a loaded machine, and `ulimit -c 0` does not help.
- **Thread-local storage**: `thread_local` of a trivial type with constant initialization is
  fine in portable code (the runtime has several); the thread's slot (one pointer, null at first)
  is what the goroutine runtime keeps its record in, and what a Windows exception handler can
  read.

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
- `TestCXX`: compiles `test/unit.cpp`, `test/slice_test.cpp`, `test/array_test.cpp`, `test/map_test.cpp`, `test/fmt_test.cpp`, `test/trace_test.cpp`, `test/interop_test.cpp`, `test/cxxvalue_test.cpp`, `test/type_test.cpp`, `test/iface_test.cpp`, `test/ifaceconv_test.cpp`, `test/nochecks_test.cpp`, and `test/diff.cpp` with the embedded runtime, with g++
  and clang++, each at `-O0`, at `-O2` and with `-fsanitize=address,undefined` (a sanitizer
  report fails the test; this mode also writes DWARF 4, the others the compilers' DWARF 5).
  The flags are the driver's: `-std=c++20 -fwrapv -fno-strict-aliasing
  -ffp-contract=off -fstack-clash-protection -g`, plus `-Wall -Wextra -Werror`, and `-Qunused-arguments` for clang.
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
  - `thread_test.cpp` checks the thread layer of Phase 08 (step 8.15): `thread_start` (stack sizes
    from 1 byte to 8 MiB: the bounds hold the frame, most of the stack can be used, 64 threads at
    once on different threads, the OS's error for a size that no OS gives and then no call of
    the function), the thread's slot (null at the start, one per thread, a thread of the C++
    library has none and no bounds), the main thread's stack bounds, setting the hook, 2,000
    threads started and ended in a row, and deep recursion that fits in a stack (no false
    overflow). Nothing depends on timing: threads wait on atomics, with a limit of two minutes.
    Run with one argument, it runs a case that ends the process, which `thread_test.go` checks:
    stack overflows of the main thread and of threads (the 8 MiB stack, 256 KiB, frames of 100
    KB, four threads at once: one report, `sp` near the end of the stack printed in the stack
    of Go's second line), the hook's arguments, and the faults that must stay crashes: a wild
    store and an overflow of a `std::thread`. More cases: a write in the guard (200 KiB below the
    stack, with another thread's stack just below it), a wild write with the stack pointer near
    the end of the stack, and a `raise(SIGSEGV)` on the main thread and on a thread must all
    crash with SIGSEGV and no Go message, and a frame of 4 MB on a 1 MiB stack with six threads
    next to it (the case that needs `-fstack-clash-protection`) must report the overflow. With ASan the overflows are the sanitizer's
    report instead (`stack-overflow`).
  - `TestThreadTSan` builds `thread_test.cpp` with ThreadSanitizer (a mode that only this test
    has so far; step 8.12 makes it a mode of the whole concurrency suite), with each compiler:
    no TSan warning.
  - `goroutine_test.cpp` checks `gp::waiter` and the goroutine runtime (Phase 08, step 8.1; the
    thread layer is `thread_test.cpp`): the waiter (unpark before park, permits that don't add up, timed
    parks that take at least their time, a time very far away, a float duration, a thread
    unparked from another, 20,000 ping-pong turns, eight threads unparking one waiter, an owner
    that destroys the waiter as soon as its park returns), goroutine ids (the main one is 1, the
    others are distinct and stable) and the count (exact the moment `go` returns, back to 1
    after the closures are destroyed), a goroutine of a goroutine and a move-only closure, a
    goroutine's own waiter, 1,000 goroutines alive at once, the default and the set stack size
    (below 1 MiB it is 1 MiB), deep recursion that fits in a goroutine's stack, the report's
    `goroutine N` line, a thread that no `go` statement started, and 2,000 goroutines started
    and ended in a row. Nothing depends on timing: threads wait on waiters and atomics, with
    a limit of two minutes. Run with one argument, it runs a case that ends the process, which
    `goroutine_test.go` checks: a panic in a goroutine (Go's report with the goroutine's number,
    the unwound destructors first, the goroutine's frames and none of libc's), in a goroutine
    of a goroutine, eight goroutines panicking at once (one report), a C++ exception and a fatal
    error in a goroutine, a nil dereference with Go's signal line, a panic in the main goroutine
    with another alive (`goroutine 1`), the failure to start a thread (Go's message, also with
    three goroutines alive: `have 4 already`), stack overflows on goroutine stacks (the default
    8 MiB stack, 256 KiB, frames of 100 KB, four goroutines at once: one report, `sp` near the end
    of the stack in the stack printed, then `goroutine N [running]:`), the hook's arguments, a
    wild store in a goroutine (a crash), and `claim_death`: after the main goroutine claims it, a
    goroutine that calls it never comes back. With a sanitizer the overflows are the sanitizer's
    report instead (`stack-overflow` of ASan).
  - `TestGoroutineTSan` builds `goroutine_test.cpp` with ThreadSanitizer (a mode that only
    this test has so far; step 8.12 makes it a mode of the whole concurrency suite), with each
    compiler: the unit tests, and the cases that end the process, none of which may print a TSan
    warning.
  - `chan_test.cpp` checks channels and the reuse of goroutine threads (Phase 08, steps 8.2 and 8.3):
    the nil channel (`len`, `cap`, `close` panics, equality, map keys), `make` with a negative or
    huge size, the ring buffer on one thread (full, empty, wrapping 100 times, a ring of one
    element), `close` (buffered values first, then zero values and `false`; the panics' texts),
    element types (strings, structs with slices and arrays, `std::unique_ptr`, `struct{}`, channels
    of channels, a type counted by its constructors and destructors: nothing leaks and nothing is
    destroyed twice; a type without a default constructor), an unbuffered hand-off (the sender is
    blocked until the receiver takes the value), the first-in-first-out wake-up of blocked senders and
    receivers (a test waits until `gp::num_blocked()` shows that a goroutine is parked before it
    starts the next, so the queue order is known), `close` waking every receiver and sender, 5,000 ping-pong
    turns, many senders and receivers on unbuffered and buffered channels (every value exactly once),
    each sender's values in order, `close` racing sends (every send completes or panics, and nothing
    is stuck), the blocked count and the reason of a blocked goroutine, thread reuse (301
    goroutines one at a time run on one thread; ids are not reused; the idle limit; 20,000
    short goroutines), `gp::go_fn` and `gp::go_call` (arguments copied at the call, move-only
    arguments, a mutable closure, Goexit), and the descriptors of the three directions of a
    channel type (distinct, interned, with Go's names, also `chan (<-chan int)`). Run with one
    argument, it runs a case that ends the process, which `chan_test.go` checks: a receive or a
    send on a nil channel blocks (and counts as blocked), `close(nil)`, a second `close`, a send on
    a closed channel, in main and in a goroutine that was blocked, and a receive from a closed
    channel whose element has no zero value, and `panic-defer-wakes-main`: a goroutine's deferred call wakes main, which returns, and the program must still end with the panic's report and exit code 2. Other unit tests: a goroutine that wakes another counts it as unblocked before it runs, the idle thread's timeout cut to 15 µs and 300 µs against `go` statements from four threads, and `make(chan struct{}, 1<<40)`. Nothing depends on timing.
  - `TestChanTSan` builds `chan_test.cpp` with ThreadSanitizer, with each compiler: no TSan warning.
  - `select_test.cpp` checks `select` and `range` over a channel (Phase 08, steps 8.5 and 8.6): the
    cases that are ready (receive, send, default anywhere in the list, only a default), nil channels
    (never ready, also with a closed channel and in the case list of a select that has nothing else),
    closed channels (the buffer first, then the zero value and `ok` false; a dropped value of a type
    without a zero value; the panic of a send with its text; no entry left in any queue afterwards),
    the random choice (30,000 selections between three always-ready channels, each between 28% and 39%;
    receives and sends mixed with a default that must never run; eight goroutines whose first
    selections differ), a goroutine blocked on three channels that exactly one of three wakers
    completes (the others' sends are not ready, all queues empty afterwards), blocked sends, `close`
    waking a select that receives and one that sends (it panics), the blocked count (counted from
    before the sleep, one decrement however many channels, and not counted any more when the waker has
    handed the value over, also when the woken goroutine cannot go on), the same channel in two cases,
    two selects that meet (one sends, one receives, 20,000 times, a second case on each side that
    never happens), selects that list the same channels in opposite orders (no deadlock), the claim
    flag raced by 8 goroutines (4 selecting senders and 4 selecting receivers on three unbuffered
    channels: every message exactly once), selects against plain operations, 20,000 ping-pong rounds
    through selects (no lost wake-up), 40 cases through `gp::select` and 1,000 through `select_run`
    (ready and blocked), move-only, string and counted elements (nothing leaks), a named channel type, a
    temporary channel, and `gp::range_chan` (order, the drop variant, an element the body does not take,
    a closed channel of a type without a zero value, a blocked receiver woken by `close`, move-only
    elements). Run with one argument it runs a case that ends the process, which `select_test.go`
    checks: `select {}` and a select of nil channels block (counted as blocked), a range over a nil
    channel blocks, the panic of a send on a closed channel in main and in a goroutine that was blocked
    in the select, and a receive from a closed channel whose element has no zero value. Every stress
    test is bounded by its operation count, and the statistical ones have margins of more than ten
    standard deviations.
  - `TestSelectTSan` builds `select_test.cpp` with ThreadSanitizer, with each compiler: no TSan warning.
    (ThreadSanitizer's deadlock detector aborts when a thread holds more than 64 locks, and a select
    holds the lock of every channel in it, so under TSan the test uses 40 channels instead of 1,000.)
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
  - `ifaceconv_test.cpp` checks `ifaceconv.hpp` and `error.hpp` (steps 5.4 and 5.5): conversions to a
    smaller method set, to `any` and to a named empty interface (the box is shared, nil stays nil, a
    moved source gives its box away, hits build no itab and allocate nothing: a counting
    `operator new`), `assert_type` (values, pointers, nil pointers, named types, composite types,
    every message of Go's `TypeAssertionError` incl. the static interface type, "types from different
    packages" and "scopes"), the comma-ok forms, `assert_iface`, the one-entry caches of a hit and a
    miss asked in turn with other types, `type_is` and `type_has`, `gp::error`, `==` and the hash of
    interface values (other Go types of one C++ type, nil pointers, uncomparable and unhashable
    dynamic types, `map[any]V`).
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
  - `fmt_test.cpp` checks fmt on hand-written descriptors, as the emitter writes them (a named
    struct with its `gp_full`, the field offsets, the `type_ops` that read slices, maps and
    pointers), the plain operands, the methods found in the method tables (`String`, `Error`,
    `Format` with its `State`, a panic in a method), `Errorf`'s types and `Unwrap` methods, a
    `String` method that changes the slice that holds it, addresses, pointer keys, weak pointers,
    big widths, and the time of `Sprintln` of 100,000 and 1,000,000 ints (printed in its `ok:`
    line). fmt is compared with real Go by the test programs `tests/run/fmt_*.gp`.
  - Without Go: the exact panic output and exit codes, and the start of the stack trace
    (the Go name and the line in `diff.cpp` of the function that panicked); on Linux, 1 MiB into a full
    non-blocking pipe (all of it must arrive) and into a broken pipe, with and without SIGPIPE
    ignored by the parent (`fmt::Print` must die of SIGPIPE, `println` must go on; these
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
