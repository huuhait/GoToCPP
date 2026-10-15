// gocpp/panic.hpp: Go's panic as a C++ exception, gp::panic_error.
//
// panic(v) throws gp::panic_error. gp::run_main catches it, prints "panic: <v>",
// "goroutine 1 [running]:" and the stack trace (gocpp/trace.hpp) to stderr and exits with
// code 2, like Go. A function with defers catches it, runs its deferred calls and throws it
// again (gocpp/defer.hpp); a deferred call that panics then starts a new panic, which keeps the
// one before it in its chain (gp_prev). Phase 05 makes the value a gp::any, for recover().
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/complex.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/print.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <memory>
#include <string>
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
// gp_type is the Go type: go/types' BasicKind number (2 int, 6 int64, 14 float64, 17 string).
// Two identities are the same when neither is unique and the kind, the type, the bits and the
// panic's text are.
struct panic_identity {
    enum : uint8_t { gp_unique = 0, gp_value = 1, gp_constant = 2, gp_runtime = 3 };
    uint8_t gp_kind = gp_unique;
    uint8_t gp_type = 0;
    uint64_t gp_bits = 0;
};

// panic_error is the exception that a Go panic throws. It doesn't derive from std::exception,
// so C++ code that catches std::exception can't stop a Go panic by accident.
struct panic_error {
    // gp_max_trace is how many frames a panic records (Go prints at most 100 too).
    static constexpr int gp_max_trace = 100;

    // A panic with this text. It records the stack (the return addresses) where it is made,
    // for the trace of an unrecovered panic: the C++ unwinding removes the frames before
    // run_main sees the panic. That costs a walk of the unwind tables (1-5 microseconds),
    // and a second, whole walk only for a stack of more than gp_max_trace frames; the names
    // and lines are found only when the trace is printed (gocpp/trace.hpp).
    explicit panic_error(gp::string text, panic_identity id = {}) noexcept;

    // The panic value as Go prints it after "panic: ": a string as it is, a number or a bool
    // in the format of the builtin print, a runtime error as "runtime error: ...".
    gp::string gp_text;

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

// throw_panic throws the panic_error with this text and identity.
[[noreturn]] void throw_panic(gp::string text, panic_identity id);

template <typename T>
gp::string print_text(const T& v) {
    std::string text;
    append_print(text, v);
    return gp::string::gp_copy(text);
}

} // namespace detail

// panic(v) and panic_constant(c): Go's builtin panic for a value of a basic type, which is not
// a constant (v) or is one (c): the emitter says which, since gc gives them different data
// pointers (panic_identity). A string is the text as it is; a bool or a number (also a complex
// one) prints like the builtin print does (panic(1.5) prints "panic: +1.500000e+000", as in
// Go). K is the go/types BasicKind of the Go type when the C++ type doesn't say it (int64,
// uint64, uintptr), else 0.
[[noreturn]] void panic(const gp::string& v);
[[noreturn]] void panic_constant(const gp::string& c);

template <uint8_t K = 0, typename T>
    requires std::is_same_v<T, bool> || go_int<T> || go_float<T> || go_complex<T>
[[noreturn]] void panic(const T& v) {
    uint64_t bits = detail::bits_of(v);
    panic_identity id;
    // gc converts a complex number to an interface with a new allocation (convTnoptr: a
    // complex64 has 8 bytes but alignment 4, a complex128 16 bytes), so its identity is unique.
    if (!go_complex<T> && (sizeof(T) == 1 || bits < 256)) {
        id = {panic_identity::gp_value, K != 0 ? K : detail::go_kind<T>(), bits};
    }
    detail::throw_panic(detail::print_text(v), id);
}

template <uint8_t K = 0, typename T>
    requires std::is_same_v<T, bool> || go_int<T> || go_float<T> || go_complex<T>
[[noreturn]] void panic_constant(const T& c) {
    uint8_t kind = sizeof(T) == 1 ? panic_identity::gp_value : panic_identity::gp_constant;
    detail::throw_panic(detail::print_text(c),
                        {kind, K != 0 ? K : detail::go_kind<T>(), detail::bits_of(c)});
}

namespace detail {

// panic_report appends what Go prints for an unrecovered panic with this text, up to the
// stack trace: "panic: " + text + "\n\ngoroutine 1 [running]:\n". As in Go, each newline in
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
// report ends after the "goroutine 1 [running]:" line.
void panic_report(std::string& out, const panic_error& e) noexcept;

// panic_chain is called by defer_frame::gp_run when the deferred call that it ran panicked with
// newer while the panic running was running: it puts (a copy of) running at the start of
// newer's chain, behind the oldest panic of that chain (a panic that started inside the
// deferred call, and whose own chain ends there), and sets that panic's gp_inner from the stack
// here. Without memory for the copy, the chain stays as it is (Go's output then misses the
// older panics).
void panic_chain(panic_error& newer, const panic_error& running) noexcept;

// The goroutine's signal, Go's g.sig: in Go, a nil pointer dereference is a memory fault, and
// an unrecovered panic then prints a signal line, "[signal SIGSEGV: segmentation violation
// code=0x1 addr=0x0 pc=0x4a2b1c]". panic_nil sets it for the running thread (a goroutine is a
// thread, ADR-007), and it stays set while the panic goes on, also through panics that
// deferred calls add. A recover that stops the panic must call clear_signal (Phase 05), as Go
// clears g.sig; a later, ordinary panic then prints no signal line.
void clear_signal() noexcept;

} // namespace detail

} // namespace gp
