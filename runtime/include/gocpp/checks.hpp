// gocpp/checks.hpp: the type concepts of Go's basic types, and the panics of the runtime
// checks, with Go's messages ("runtime error: integer divide by zero").
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

// panic_index: "runtime error: index out of range [5] with length 3". A negative signed index
// prints without the length, like Go: "index out of range [-1]". When is_signed is false, x is
// the bits of an unsigned index (uint64), and it prints as unsigned.
[[noreturn]] void panic_index(int64_t x, bool is_signed, int64_t len);

// check_index panics with panic_index unless 0 <= i < len.
template <go_int I>
inline void check_index(I i, int64_t len) {
    if constexpr (std::is_signed_v<I>) {
        if (i < 0 || static_cast<int64_t>(i) >= len) {
            panic_index(static_cast<int64_t>(i), true, len);
        }
    } else {
        if (static_cast<uint64_t>(i) >= static_cast<uint64_t>(len)) {
            panic_index(static_cast<int64_t>(static_cast<uint64_t>(i)), false, len);
        }
    }
}

} // namespace gp
