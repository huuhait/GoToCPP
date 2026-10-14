// gocpp/checks.hpp: the type concepts of Go's basic types, and the panics of the runtime
// checks, with Go's messages ("runtime error: integer divide by zero").
//
// GOCPP_NOCHECKS, which `gocpp build -nochecks` defines for every file of the program (the
// runtime's too), turns off the bounds and nil checks: gp::check_index here, check_slice,
// check_slice3 and check_convert (bounds.hpp), the nil check of *p and p->f and the owner
// check (ptr.hpp). An index out of range or a nil dereference is then undefined behavior, as
// in C++. Everything else stays: division by zero and a negative shift count still panic
// (C++ has undefined behavior there, and the checks cost little), Go's results for MIN / -1
// and for a shift count of the width or more stay (arith.hpp), and so do the panics that are
// part of what an operation does (a write to a nil map, make with a bad length, an index of a
// [0]T, which has no element to give).
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

} // namespace gp
