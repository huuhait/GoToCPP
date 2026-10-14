// gocpp/arith.hpp: the integer helpers that give Go's results where C++ has undefined
// behavior or different results: gp::div, gp::rem, gp::shl, gp::shr and gp::f2i<T>.
// Each returns the Go type, so int8, int16, uint8 and uint16 results are cast back
// (C++ computes them in int).
#pragma once

#include <gocpp/checks.hpp>

#include <cstdint>
#include <limits>
#include <type_traits>

namespace gp {

// div(a, b) is Go's a / b: it panics when b is 0 ("runtime error: integer divide by zero"),
// and MIN / -1 is MIN (Go wraps; in C++ it is undefined behavior or SIGFPE). The quotient is
// truncated toward zero, as in C++. The second operand converts to the type of the first.
template <go_int T>
inline T div(T a, std::type_identity_t<T> b) {
    if (b == 0) {
        panic_divide();
    }
    if constexpr (std::is_signed_v<T>) {
        if (b == -1) {
            // -a, wrapping: MIN stays MIN. The negation is done on the unsigned type, so it
            // doesn't overflow even without -fwrapv.
            using U = std::make_unsigned_t<T>;
            return static_cast<T>(static_cast<U>(0u - static_cast<U>(a)));
        }
    }
    return static_cast<T>(a / b);
}

// rem(a, b) is Go's a % b: it panics when b is 0, and MIN % -1 is 0. The result has the sign
// of a, as in C++.
template <go_int T>
inline T rem(T a, std::type_identity_t<T> b) {
    if (b == 0) {
        panic_divide();
    }
    if constexpr (std::is_signed_v<T>) {
        if (b == -1) {
            return 0;
        }
    }
    return static_cast<T>(a % b);
}

// shl(x, n) is Go's x << n: a count of the width of T or more gives 0, and a negative count
// panics ("runtime error: negative shift amount"). Bits shifted out are lost, also for a
// signed x (Go wraps). n may be of any integer type, as in Go.
template <go_int T, go_int N>
inline T shl(T x, N n) {
    if constexpr (std::is_signed_v<N>) {
        if (n < 0) {
            panic_shift();
        }
    }
    if (static_cast<uint64_t>(n) >= std::numeric_limits<std::make_unsigned_t<T>>::digits) {
        return 0;
    }
    using U = std::make_unsigned_t<T>;
    return static_cast<T>(static_cast<U>(static_cast<U>(x) << static_cast<int>(n)));
}

// shr(x, n) is Go's x >> n: an arithmetic shift for a signed x, a logical one for an unsigned
// x. A count of the width of T or more gives 0, or -1 for a negative signed x. A negative
// count panics.
template <go_int T, go_int N>
inline T shr(T x, N n) {
    if constexpr (std::is_signed_v<N>) {
        if (n < 0) {
            panic_shift();
        }
    }
    if (static_cast<uint64_t>(n) >= std::numeric_limits<std::make_unsigned_t<T>>::digits) {
        if constexpr (std::is_signed_v<T>) {
            return x < 0 ? T(-1) : T(0);
        } else {
            return 0;
        }
    }
    return static_cast<T>(x >> static_cast<int>(n));
}

namespace detail {

// cvt64 and cvt32 are the amd64 instructions CVTTSD2SQ and CVTTSD2SL: truncate toward zero,
// and give the "integer indefinite" value (the type's MIN) for NaN and for values whose
// truncation doesn't fit. Here without undefined behavior: C++ only converts in range.
inline int64_t cvt64(double x) noexcept {
    if (x >= -0x1p63 && x < 0x1p63) {
        return static_cast<int64_t>(x);
    }
    return std::numeric_limits<int64_t>::min();
}

inline int32_t cvt32(double x) noexcept {
    if (x > -2147483649.0 && x < 2147483648.0) {
        return static_cast<int32_t>(x);
    }
    return std::numeric_limits<int32_t>::min();
}

} // namespace detail

// f2i<T>(x) is Go's T(x) for a float x: in range, it truncates toward zero. Out of range (and
// for NaN and ±Inf), C++ has undefined behavior and Go's result depends on the CPU; gocpp
// copies Go on amd64 everywhere, as Go 1.25 compiles it (checked with real Go):
//   - int64 (int) and int32 give their MIN value;
//   - int16, int8, uint16 and uint8 convert to int32 that way, then keep the low bits;
//   - uint32 converts to int64, then keeps the low 32 bits;
//   - uint64 (uint, uintptr): below 2^63 it converts to int64; otherwise it converts x - 2^63
//     to int64 and sets the top bit (NaN, +Inf, -Inf and huge values give 1 << 63).
// A float32 argument converts to double first, which is exact, so the results are the same as
// Go's float32 conversions.
template <go_int T>
inline T f2i(double x) noexcept {
    if constexpr (sizeof(T) == 8 && std::is_signed_v<T>) {
        return static_cast<T>(detail::cvt64(x));
    } else if constexpr (sizeof(T) == 8) {
        if (x < 0x1p63) {
            return static_cast<T>(static_cast<uint64_t>(detail::cvt64(x)));
        }
        return static_cast<T>(static_cast<uint64_t>(detail::cvt64(x - 0x1p63)) |
                              (uint64_t(1) << 63));
    } else if constexpr (sizeof(T) == 4 && std::is_unsigned_v<T>) {
        return static_cast<T>(detail::cvt64(x));
    } else {
        // int32, and int16, int8, uint16, uint8 through int32.
        return static_cast<T>(detail::cvt32(x));
    }
}

} // namespace gp
