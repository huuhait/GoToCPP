// gocpp/checks.hpp: the type concepts of Go's basic types, and the panics of the runtime
// checks, with Go's messages ("runtime error: integer divide by zero").
//
// GOCPP_NOCHECKS, which `gocpp build -nochecks` defines for every file of the program (the
// runtime's too), turns off the bounds and nil checks: gp::check_index and check_wrap here,
// check_slice, check_slice3 and check_convert (bounds.hpp), the nil check of *p and p->f and
// the owner check (ptr.hpp), the nil check of a call of a func value (func.hpp), the
// checks of the generated code's literal keys (array.hpp, slice.hpp), and in interop.hpp the
// nil check of gp::deref, the index check of gp::range and the nil checks of C.GoStringN and
// C.GoBytes. An index out of range
// or a nil dereference is then undefined behavior, as in C++. Everything else stays: division
// by zero and a negative shift count still panic (C++ has undefined behavior there, and the
// checks cost little), Go's results for MIN / -1 and for a shift count of the width or more
// stay (arith.hpp), and so do the panics that are part of what an operation does (a write to
// a nil map, make with a bad length, append's overflow, an index of a [0]T, which has no
// element to give) and the fatal errors (panic.hpp).
#pragma once

#include <concepts>
#include <cstdint>
#include <type_traits>

namespace gp {

// go_int: a C++ type that holds a Go integer type: int8_t ... int64_t, uint8_t ... uint64_t
// (and the other integer types of the same sizes), but not bool.
template <typename T>
concept go_int = std::integral<T> && !std::same_as<std::remove_cv_t<T>, bool>;

// go_float: float (Go's float32) or double (float64).
template <typename T>
concept go_float =
    std::same_as<std::remove_cv_t<T>, float> || std::same_as<std::remove_cv_t<T>, double>;

// These throw gp::panic_error with Go's runtime error messages.

// panic_divide: "runtime error: integer divide by zero".
[[noreturn]] void panic_divide();

// panic_shift: "runtime error: negative shift amount".
[[noreturn]] void panic_shift();

// panic_nil: "runtime error: invalid memory address or nil pointer dereference", the panic of
// a nil pointer dereference (gp::ptr). In Go this panic comes from a memory fault, so it also
// sets the goroutine's signal (panic.hpp), and an unrecovered panic then prints Go's signal
// line, "[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x...]", where the pc is
// the caller's.
[[noreturn]] void panic_nil();

// panic_index: "runtime error: index out of range [5] with length 3". A negative signed index
// prints without the length, like Go: "index out of range [-1]". When is_signed is false, x is
// the bits of an unsigned index (uint64), and it prints as unsigned.
[[noreturn]] void panic_index(int64_t x, bool is_signed, int64_t len);

// check_index panics with panic_index unless 0 <= i < len. With GOCPP_NOCHECKS it does
// nothing.
template <go_int I>
inline void check_index([[maybe_unused]] I i, [[maybe_unused]] int64_t len) {
#ifndef GOCPP_NOCHECKS
    if constexpr (std::is_signed_v<I>) {
        if (i < 0 || static_cast<int64_t>(i) >= len) {
            panic_index(static_cast<int64_t>(i), true, len);
        }
    } else {
        if (static_cast<uint64_t>(i) >= static_cast<uint64_t>(len)) {
            panic_index(static_cast<int64_t>(static_cast<uint64_t>(i)), false, len);
        }
    }
#endif
}

// panic_wrap: the panic of Go's wrapper (*T).M of a value method M, called with a nil pointer
// (runtime.panicwrap): msg is "value method main.T.M called using nil *T pointer". Its
// identity is a string constant's (panic.hpp), as gocpp gave it before Phase 05's
// runtime.Error.
[[noreturn]] void panic_wrap(const char* msg);

// check_wrap: the check of that wrapper, which the emitter puts before it dereferences p. It is
// a nil check, so with GOCPP_NOCHECKS it does nothing (the dereference is then undefined
// behavior).
template <typename P>
inline void check_wrap([[maybe_unused]] const P& p, [[maybe_unused]] const char* msg) {
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) [[unlikely]] {
        panic_wrap(msg);
    }
#endif
}

} // namespace gp
