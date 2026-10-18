// gocpp/panic.hpp: Go's panic as a C++ exception, gp::panic_error.
//
// panic(v) throws gp::panic_error. gp::run_main catches it, prints "panic: <v>",
// "goroutine 1 [running]:" and the stack trace (gocpp/trace.hpp) to stderr and exits with
// code 2, like Go. A function with defers catches it, runs its deferred calls and throws it
// again (gocpp/defer.hpp); a deferred call that panics then starts a new panic, which keeps the
// one before it in its chain (gp_prev).
//
// The panic's value is a gp::any (gp_value), which recover() returns (step 5.10):
//
//     panic("boom")                  an any of Go type string
//     panic(1.5)                     an any of Go type float64 (panic<K> names int64, uint64 ...)
//     panic(err), panic(x)           the interface value itself (gp::panic(const iface&)); the
//                                    nil interface is panic(nil): *runtime.PanicNilError
//     index out of range, nil map,   runtime.Error values (runtime.boundsError,
//     runtime.errorString,
//       division by zero, ...          runtime.plainError: types with Go's Error() text and a
//                                    RuntimeError() method, so r.(error) and r.(runtime.Error)
//                                    work), made by detail::throw_runtime
//     a C++ exception                a CppException (below), made where a deferred frame catches it
//
// The text that an unrecovered panic prints is gp_text for the values that are known when the panic
// is thrown (a string, a number, a runtime error), and for any other value it is made when the
// report is written, as Go's preprintpanics does: Error() of an error, else String() of a
// fmt.Stringer, else Go's printpanicval (detail::panic_text).
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/complex.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/iface.hpp>
#include <gocpp/print.hpp>
#include <gocpp/string.hpp>
#include <gocpp/type.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <exception>
#include <memory>
#include <string>
#include <string_view>
#include <type_traits>
#include <vector>

namespace gp {

// panic_identity is what Go's printing of a panic chain compares: Go prints a panic whose value
// has the same interface words (type and data pointer) as the panic after it once, with
// " [recovered, repanicked]", and leaves the later one out (runtime/panic.go: preprintpanics,
// printpanics), also without recover. The data pointer is gc's: a value converted to `any` is
// unique (a new allocation) unless gc puts it in static data (cmd/compile walk/convert.go
// dataWord, walk/order.go addrTemp, runtime/iface.go convT*), so a panic_identity is:
//   - unique (gp_kind 0): never the same as another one (also every complex value);
//   - a value in the runtime's static data (value): a bool or a 1-byte integer (any value),
//     a 2-, 4- or 8-byte integer or float whose bits are below 256 (runtime.staticuint64s),
//     and the empty string (runtime.zeroVal); the same for the same type and bits;
//   - a constant (constant): gc's read-only static data, one per type and value (a constant
//     of a 1-byte type or bool is a value as above);
//   - a runtime error that Go keeps in one global or a constant (runtime): the nil
//     dereference, division by zero, a negative shift, a store into a nil map, makeslice and
//     growslice; the same for the same message. Index and slice-bounds errors are unique.
//   - an interface value that was converted before (panic(err), panic(x) with x an any): its
//     words are the type and the data pointer, so the same value is the same (gp_words, compared
//     through panic_error::gp_value), and a value that is converted at the panic keeps the rules
//     above.
// gp_type is the Go type: go/types' BasicKind number (2 int, 6 int64, 14 float64, 17 string).
// Two identities are the same when neither is unique and the kind, the type, the bits and the
// panic's text are.
struct panic_identity {
    enum : uint8_t { gp_unique = 0, gp_value = 1, gp_constant = 2, gp_runtime = 3, gp_words = 4 };
    uint8_t gp_kind = gp_unique;
    uint8_t gp_type = 0;
    uint64_t gp_bits = 0;
};

// goexit_signal is the exception of runtime.Goexit (step 5.10, gp::goexit): it unwinds the
// goroutine's stack and runs the deferred calls, like a panic, but recover() returns nil in
// them and nothing stops it: every deferred frame throws it again, and the goroutine's thread
// ends quietly (goroutine.cpp), or, for the main goroutine, waits for the others (run_main). Like
// panic_error it is no std::exception.
struct goexit_signal {};

// CppException is the value of a panic that a C++ exception became (step 5.10): a function with
// defers catches every exception that reaches it (defer_frame::gp_on_exception), and one that is
// not a Go panic (a std::exception of a C++ library, anything thrown) starts a Go panic with
// this value, so that the deferred calls run and recover() can stop it. recover() returns an
// `error` whose Go type is runtime.CppException and whose Error() is the text that an unrecovered
// C++ exception prints (cxx_exception_text): "C++ exception std::out_of_range: vector::at" for a
// std::exception (its type and what()), "C++ exception of type int" for `throw 42`. fmt prints
// that text (%v, %s), and gp_what and gp_type keep its parts.
struct CppException {
    gp::string gp_text; // what Error() returns
    gp::string gp_type; // the C++ type, demangled ("std::out_of_range")
    gp::string gp_what; // what() of a std::exception, else empty
    friend bool operator==(const CppException& a, const CppException& b) {
        return a.gp_text == b.gp_text && a.gp_type == b.gp_type && a.gp_what == b.gp_what;
    }
};

namespace detail {

// panic_count_add changes the number of panics that goroutines are handling (src/goroutine.cpp:
// panicking, settle_panics).
void panic_count_add(int delta) noexcept;

// panic_token is a panic_error's place in that count: defer_frame::gp_take turns it on when a
// deferred-call frame takes the panic, and it turns itself off when the panic object dies, however
// that happens: recovered, swallowed by a catch of C++ code (fmt prints %!v(PANIC=...)), or the end
// of the goroutine. A copy is not counted (the chain keeps copies of older panics).
struct panic_token {
    panic_token() noexcept = default;
    panic_token(const panic_token&) noexcept {}
    panic_token& operator=(const panic_token&) noexcept { return *this; }
    ~panic_token() {
        if (gp_on) {
            panic_count_add(-1);
        }
    }
    // panic_mark is what a frame calls when it takes the panic (once).
    void gp_mark() noexcept {
        if (!gp_on) {
            gp_on = true;
            panic_count_add(1);
        }
    }
    bool gp_on = false;
};

} // namespace detail

// panic_error is the exception that a Go panic throws. It doesn't derive from std::exception,
// so C++ code that catches std::exception can't stop a Go panic by accident.
struct panic_error {
    // gp_max_trace is how many frames a panic records (Go prints at most 100 too).
    static constexpr int gp_max_trace = 100;

    // A runtime error with this text (the panics that the runtime itself starts): the value is
    // a runtime.Error, runtime.errorString for a text that starts with "runtime error: ", else
    // runtime.plainError (detail::runtime_error_value), and `recover()` returns it. It records
    // the stack (the return addresses) where it is made,
    // for the trace of an unrecovered panic: the C++ unwinding removes the frames before
    // run_main sees the panic. That costs a walk of the unwind tables (1-5 microseconds),
    // and a second, whole walk only for a stack of more than gp_max_trace frames; the names
    // and lines are found only when the trace is printed (gocpp/trace.hpp).
    explicit panic_error(gp::string text, panic_identity id = {}) noexcept;

    // A panic with this value and (when it is known now) this text; gp_lazy: the text is made
    // from the value when it is printed.
    panic_error(gp::any value, gp::string text, bool lazy, panic_identity id) noexcept;

    // The panic value as Go prints it after "panic: ": a string as it is, a number or a bool
    // in the format of the builtin print, a runtime error as "runtime error: ...". Empty when
    // gp_lazy: detail::panic_text(e) is the text in every case.
    gp::string gp_text;

    // The value recover() returns: never the nil interface.
    gp::any gp_value;

    // The text is made from gp_value when the report is written (an error, a Stringer, any
    // value that Go prints with printpanicval).
    bool gp_lazy = false;

    // recover() stopped this panic (it may still be in a chain: Go prints it as "[recovered]").
    bool gp_recovered = false;

    // The value's identity, for the printing of a panic chain (above).
    panic_identity gp_id;

    // The stack where the panic was made: gp_npcs return addresses, innermost first.
    int gp_npcs = 0;
    uintptr_t gp_pcs[gp_max_trace]{};

    // gp_all is the whole stack when gp_pcs is full: only then a second walk records every
    // frame (up to gp_max_walk), so that the trace can show Go's outer frames too.
    static constexpr std::size_t gp_max_walk = std::size_t(1) << 22;
    std::shared_ptr<const std::vector<uintptr_t>> gp_all;

    // The panic chain (step 3.13): gp_prev is the panic that was running when this one
    // started, in a deferred call (detail::panic_chain), and so on back to the first one; Go
    // prints them all, the first one first ("panic: A", "\tpanic: B"). gp_inner is then the
    // number of frames of this panic's stack (gp_all, else gp_pcs) that are above the
    // defer_frame::gp_run that ran the deferred call; the trace shows those, a "panic" frame,
    // and then gp_prev's stack, as Go shows the deferred call on top of the panicking function.
    // -1 when it isn't known (the stack wasn't recorded to its end): the trace then shows this
    // panic's own stack.
    std::shared_ptr<panic_error> gp_prev;
    int64_t gp_inner = -1;

    // Is this panic being handled by a goroutine's deferred calls (detail::panic_token)?
    detail::panic_token gp_token;
};

namespace detail {

// go_kind is the go/types BasicKind of the Go type that the C++ type T holds by default: int64_t
// is int and uint64_t is uint (the emitter names int64, uint64 and uintptr: gp::panic<K>).
template <typename T>
constexpr uint8_t go_kind() {
    if constexpr (std::is_same_v<T, bool>) {
        return 1;
    } else if constexpr (std::is_same_v<T, int8_t>) {
        return 3;
    } else if constexpr (std::is_same_v<T, int16_t>) {
        return 4;
    } else if constexpr (std::is_same_v<T, int32_t>) {
        return 5;
    } else if constexpr (std::is_same_v<T, int64_t>) {
        return 2;
    } else if constexpr (std::is_same_v<T, uint8_t>) {
        return 8;
    } else if constexpr (std::is_same_v<T, uint16_t>) {
        return 9;
    } else if constexpr (std::is_same_v<T, uint32_t>) {
        return 10;
    } else if constexpr (std::is_same_v<T, uint64_t>) {
        return 7;
    } else if constexpr (std::is_same_v<T, float>) {
        return 13;
    } else if constexpr (std::is_same_v<T, complex64>) {
        return 15;
    } else if constexpr (std::is_same_v<T, complex128>) {
        return 16;
    } else {
        return 14;
    }
}

// bits_of returns the bytes of v as an unsigned number of its size.
template <typename T>
uint64_t bits_of(const T& v) {
    if constexpr (sizeof(T) == 1) {
        uint8_t b;
        std::memcpy(&b, &v, 1);
        return b;
    } else if constexpr (sizeof(T) == 2) {
        uint16_t b;
        std::memcpy(&b, &v, 2);
        return b;
    } else if constexpr (sizeof(T) == 4) {
        uint32_t b;
        std::memcpy(&b, &v, 4);
        return b;
    } else if constexpr (sizeof(T) == 8) {
        uint64_t b;
        std::memcpy(&b, &v, 8);
        return b;
    } else {
        // A complex128: both parts, mixed into 64 bits (a panic_identity compares the text
        // too, so two different constants are the same only if both collide).
        static_assert(std::is_same_v<T, complex128>);
        uint64_t re, im;
        std::memcpy(&re, &v.gp_re, 8);
        std::memcpy(&im, &v.gp_im, 8);
        return re ^ hash_mix(im);
    }
}

// throw_panic throws the panic_error with this value, text and identity.
[[noreturn]] void throw_panic(gp::any value, gp::string text, panic_identity id);

template <typename T>
gp::string print_text(const T& v) {
    std::string text;
    append_print(text, v);
    return gp::string::gp_copy(text);
}

// td_of_basic is the descriptor of the basic Go type with the go/types BasicKind k (1 bool ...
// 17 string), for the value of a panic.
inline const type_desc* td_of_basic(uint8_t k) noexcept {
    switch (k) {
    case 1:
        return &td_bool;
    case 2:
        return &td_int;
    case 3:
        return &td_int8;
    case 4:
        return &td_int16;
    case 5:
        return &td_int32;
    case 6:
        return &td_int64;
    case 7:
        return &td_uint;
    case 8:
        return &td_uint8;
    case 9:
        return &td_uint16;
    case 10:
        return &td_uint32;
    case 11:
        return &td_uint64;
    case 12:
        return &td_uintptr;
    case 13:
        return &td_float32;
    case 14:
        return &td_float64;
    case 15:
        return &td_complex64;
    case 16:
        return &td_complex128;
    default:
        return &td_string;
    }
}

} // namespace detail

// panic(v) and panic_constant(c): Go's builtin panic for a value of a basic type, which is not
// a constant (v) or is one (c): the emitter says which, since gc gives them different data
// pointers (panic_identity). A string is the text as it is; a bool or a number (also a complex
// one) prints like the builtin print does (panic(1.5) prints "panic: +1.500000e+000", as in
// Go). K is the go/types BasicKind of the Go type when the C++ type doesn't say it (int64,
// uint64, uintptr), else 0. The value of the panic (what recover() returns) is the value as
// an `any` of that Go type.
[[noreturn]] void panic(const gp::string& v);
[[noreturn]] void panic_constant(const gp::string& c);

template <uint8_t K = 0, typename T>
    requires std::is_same_v<T, bool> || go_int<T> || go_float<T> || go_complex<T>
[[noreturn]] void panic(const T& v) {
    uint64_t bits = detail::bits_of(v);
    panic_identity id;
    // gc converts a complex number to an interface with a new allocation (convTnoptr: a
    // complex64 has 8 bytes but alignment 4, a complex128 16 bytes), so its identity is unique.
    constexpr uint8_t kind = K != 0 ? K : detail::go_kind<T>();
    if (!go_complex<T> && (sizeof(T) == 1 || bits < 256)) {
        id = {panic_identity::gp_value, kind, bits};
    }
    detail::throw_panic(gp::to_iface<gp::any>(v, detail::td_of_basic(kind)), detail::print_text(v),
                        id);
}

template <uint8_t K = 0, typename T>
    requires std::is_same_v<T, bool> || go_int<T> || go_float<T> || go_complex<T>
[[noreturn]] void panic_constant(const T& c) {
    constexpr uint8_t kind = K != 0 ? K : detail::go_kind<T>();
    uint8_t how = sizeof(T) == 1 ? panic_identity::gp_value : panic_identity::gp_constant;
    detail::throw_panic(gp::to_iface<gp::any>(c, detail::td_of_basic(kind)), detail::print_text(c),
                        {how, kind, detail::bits_of(c)});
}

// panic(v) for an interface value (an any, an error, any interface type): the panic's value is
// v itself. The nil interface is panic(nil): since Go 1.21 a *runtime.PanicNilError ("panic
// called with nil argument"), which recover() returns. Two panics with the same interface
// value (the same type and data) are printed once in a chain, as Go does (panic_identity).
namespace detail {
[[noreturn]] void panic_value(const gp::iface& v);
} // namespace detail

template <typename I>
    requires std::is_base_of_v<gp::iface, I>
[[noreturn]] void panic(const I& v) {
    detail::panic_value(v);
}

// panic(nil): the argument is the untyped nil. Same as panic of a nil interface.
[[noreturn]] void panic(std::nullptr_t);

// goexit is runtime.Goexit: it runs the deferred calls of the goroutine and ends it
// (goexit_signal). recover() returns nil during it, and nothing stops it. Called by the main
// goroutine, the program goes on with the other goroutines, and when they are all gone it
// ends with Go's "fatal error: no goroutines (main called runtime.Goexit) - deadlock!".
[[noreturn]] void goexit();

namespace detail {

// panic_report appends what Go prints for an unrecovered panic with this text, up to the
// stack trace: "panic: " + text + "\n\ngoroutine N [running]:\n", with the id N of the goroutine
// that calls it (1 for the main goroutine, gocpp/goroutine.hpp). As in Go, each newline in
// the text is followed by a tab. When the goroutine has a signal (below), Go's signal line
// comes after the text, before the blank line.
void panic_report(std::string& out, const gp::string& text);

// panic_report(out, chain, n) is the same for a panic chain, the first panic first, as Go's
// printpanics: each on a line "panic: <text>", the later ones after a tab; a panic with the
// same identity as the one after it (panic_identity) gets " [recovered, repanicked]", and the
// one after it is left out. The signal line comes once, after the chain, as in Go.
void panic_report(std::string& out, const panic_error* const* chain, std::size_t n);

// panic_report(out, e) is the whole report of the unrecovered panic e, as run_main prints it:
// the lines above for e's chain, then the stack trace of e (gocpp/trace.hpp), with the frames
// of the panics before it in the chain (gp_inner). If the trace can't be made (no memory), the
// report ends after the "goroutine N [running]:" line.
void panic_report(std::string& out, const panic_error& e) noexcept;

// panic_chain is called by defer_frame::gp_run when the deferred call that it ran panicked with
// newer while the panic running was running: it puts (a copy of) running at the start of
// newer's chain, behind the oldest panic of that chain (a panic that started inside the
// deferred call, and whose own chain ends there), and sets that panic's gp_inner from the stack
// here. Without memory for the copy, the chain stays as it is (Go's output then misses the
// older panics).
void panic_chain(panic_error& newer, const panic_error& running) noexcept;

// panic_text is the text that an unrecovered panic prints after "panic: " (not yet indented):
// gp_text when it is known, else made from the value as Go's preprintpanics and printpanicval
// do: Error() of an error, String() of a fmt.Stringer, a basic value in the format of the builtin
// print (a named type with its name: main.MyInt(5), main.S("x")), any other value as
// "(main.T) 0xc000012345". If Error() or String() panics, Go ends the program with the fatal
// error "panic while printing panic value: type main.T": so does this.
gp::string panic_text(const panic_error& e);

// The runtime.Error values (runtime/src/panicval.cpp). All have Error() string and
// RuntimeError() methods, so `r.(error)` and `r.(runtime.Error)` hold for a recovered runtime
// panic; their Go types, as %T prints them, are
//   runtime.errorString    "runtime error: integer divide by zero" (Error() is the whole text)
//   runtime.boundsError    "runtime error: index out of range [5] with length 3", slice bounds
//   runtime.plainError     "assignment to entry in nil map", "close of closed channel": a
//                          runtime.Error whose text has no "runtime error: " prefix
//   *runtime.TypeAssertionError   "interface conversion: ..." (runtime_assertion)
//   *runtime.PanicNilError        "panic called with nil argument" (panic(nil))
//   runtime.CppException   (CppException above; an error, not a runtime.Error)
enum class rt_error : uint8_t { error_string, bounds, plain };

// runtime_error_value(kind, text) is the value of a runtime error whose Error() is text.
gp::any runtime_error_value(rt_error kind, gp::string text);

// runtime_error_value(text) picks the kind by the text: "runtime error: ..." is an errorString,
// else a plainError. (panic_error's constructor from a text uses it.)
gp::any runtime_error_value(const gp::string& text);

// throw_runtime throws the panic of a runtime error with this kind, full text and identity.
[[noreturn]] void throw_runtime(rt_error kind, gp::string text, panic_identity id = {});

// type_assertion_error(text) is the value of a failed type assertion: a *runtime.TypeAssertionError
// whose Error() is text ("interface conversion: interface {} is string, not int").
gp::any type_assertion_error(gp::string text);

// throw_type_assertion(text) throws the panic of a failed type assertion: its value is
// type_assertion_error(text), and the identity is unique (Go allocates a new one each time).
[[noreturn]] void throw_type_assertion(gp::string text);

// cxx_exception_value(e) is the CppException value for the C++ exception e (not a Go panic).
gp::any cxx_exception_value(const std::exception_ptr& e);

// string_method calls the method name() string of the value v (Error or String; one with the
// signature "func() string" and an exported name) and reports whether the dynamic type of v has it.
bool string_method(const gp::iface& v, const char* name, gp::string& out);

// The goroutine's signal, Go's g.sig: in Go, a nil pointer dereference is a memory fault, and
// an unrecovered panic then prints a signal line, "[signal SIGSEGV: segmentation violation
// code=0x1 addr=0x0 pc=0x4a2b1c]". panic_nil sets it for the running thread (a goroutine is a
// thread, ADR-007), and it stays set while the panic goes on, also through panics that
// deferred calls add. When a recover has stopped the panic, the deferred call that recovered it
// has returned and no other panic is running, defer_frame::gp_run calls clear_signal, as Go's
// recovery clears g.sig: a later, ordinary panic prints no signal line. (A deferred call that
// recovers and then panics again before it returns still prints it.)
void clear_signal() noexcept;

// fatal is Go's fatal error (runtime.throw), an error that no recover stops: it prints "fatal
// error: <msg>", a blank line, "goroutine N [running]:" and the stack trace of its caller to
// stderr, and exits with code 2 at once. Nothing unwinds, so no deferred call runs, as in Go.
// The trace needs memory: when there is none (it is out of memory), the trace is left out or
// cut short. It is not async-signal-safe: not for a signal handler.
[[noreturn]] void fatal(std::string_view msg) noexcept;

// exit_with_panic ends the program because the panic e reached the top of a goroutine (run_main
// for the main one, the start of every other one): it claims the right to report (claim_death,
// gocpp/goroutine.hpp: when several goroutines die at once, one reports), flushes C's stdio,
// prints panic_report(e) with its stack trace to stderr and exits with code 2.
[[noreturn]] void exit_with_panic(const panic_error& e) noexcept;

// out_of_memory is the new handler (std::set_new_handler) that run_main installs: an allocation
// that fails (operator new, also std::nothrow's, so every runtime type) ends the program with
// fatal("runtime: out of memory") where it fails, like Go, instead of throwing std::bad_alloc.
void out_of_memory();

// C++ exceptions (Phase 04 README §2.8, first version; docs/spec/interop.md "C++ exceptions").
// An exception that C++ code throws and nothing catches ends the program like an unrecovered Go
// panic: "panic: " + cxx_exception_text, the goroutine's stack trace where it was thrown, exit
// code 2. on_terminate does that: it is the terminate handler (std::set_terminate) that run_main
// installs. The C++ ABI (GCC and Clang, Linux and MinGW) looks for a handler before it unwinds
// anything, and calls std::terminate on the stack of the throw when it finds none, so nothing
// has unwound yet: no deferred call and no C++ destructor has run (Go runs the deferred calls
// of a panic; Phase 05 turns these exceptions into recoverable Go panics where they enter gocpp
// code), and the trace shows the frames of the throw, the innermost first. (A catch that throws
// the exception again, catch (...) { throw; } in C++ code, or a noexcept function that it
// reaches, has unwound the frames inside it: the trace starts there.) Generated code catches
// only gp::panic_error, so the exception passes all of it.
//
// on_terminate prints, after C's stdio is flushed:
//   - for a Go panic (a gp::panic_error that reached a noexcept C++ function), what run_main
//     prints for it;
//   - for a std::bad_alloc that the new handler didn't see (std::bad_array_new_length of new
//     int[-1], or one that C++ code throws), Go's fatal error "runtime: out of memory" with the
//     trace of the throw (fatal);
//   - for any other exception, the chain of the Go panic that runs (a deferred call threw it),
//     then "panic: " + cxx_exception_text, and the trace of the throw (fatal_trace);
//   - without an exception (std::terminate called for another reason: a std::thread that is
//     destroyed while it runs), the C++ library's own message: the handler before it runs.
// A destructor that throws while a Go panic unwinds the stack (a C++ destructor that is
// noexcept(false)) makes C++ call std::terminate with the new exception: the report shows only
// that one, the Go panic is lost (Phase 05 can chain them).

// cxx_exception_text is what an unrecovered C++ exception e prints after "panic: ": "C++
// exception <type>: <what()>" for a std::exception (only "C++ exception <type>" when what() is
// empty), "C++ exception of type <type>" for anything else (throw 42: "C++ exception of type
// int"). The type is the type it was thrown with, its C++ name demangled (std::out_of_range).
std::string cxx_exception_text(const std::exception_ptr& e);

// cxx_exception_parts splits the same: type is the C++ type that e was thrown with ("?" when it
// can't be known), has_what says that it is a std::exception, and what is its what().
void cxx_exception_parts(const std::exception_ptr& e, std::string& type, std::string& what,
                         bool& has_what);

[[noreturn]] void on_terminate() noexcept;

} // namespace detail

} // namespace gp
