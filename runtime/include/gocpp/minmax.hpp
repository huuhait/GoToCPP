// gocpp/minmax.hpp: Go's builtins min and max (Go 1.21) for arguments that are not all
// constants (go/types folds the constant calls). The emitter names the result type, which is
// the type of every argument: min(x, 1) for an int64 x is gp::min<int64_t>(x, 1).
//
// The values are compared as Go's runtime does (runtime/minmax.go): integers and strings by <,
// floats with Go's rules, where a NaN argument makes the result NaN and -0 is less than +0
// (min(-0.0, 0.0) is -0, max is +0). A named type (gp::newtype) compares its values, and the
// result has the named type.
#pragma once

#include <gocpp/checks.hpp>

#include <bit>
#include <cstdint>
#include <type_traits>

namespace gp {

namespace detail {

// minmax_value is the value that min and max compare: a named type's underlying value.
template <typename T>
const auto& minmax_value(const T& v) noexcept {
    if constexpr (requires { v.gp_value(); }) {
        return minmax_value(v.gp_value());
    } else {
        return v;
    }
}

// negative_zero reports whether the zero z is -0 (its sign bit; <cmath>'s signbit would bring
// its macros into generated code).
template <typename F>
bool negative_zero(F z) noexcept {
    if constexpr (sizeof(F) == 4) {
        return (std::bit_cast<uint32_t>(z) >> 31) != 0;
    } else {
        return (std::bit_cast<uint64_t>(z) >> 63) != 0;
    }
}

// pick returns y if it wins over x (is less for min, greater for max), else x.
template <bool Max, typename T>
const T& pick(const T& x, const T& y) noexcept {
    const auto& a = detail::minmax_value(x);
    const auto& b = detail::minmax_value(y);
    if constexpr (go_float<std::remove_cvref_t<decltype(a)>>) {
        // Go's fmin and fmax: a NaN wins; of two zeros, min gives -0 if either is, max +0.
        if (b != b || (Max ? a < b : b < a)) {
            return y;
        }
        if (a != a || a != b || a != 0) {
            return x;
        }
        return detail::negative_zero(a) != Max ? x : y;
    } else {
        return (Max ? a < b : b < a) ? y : x;
    }
}

} // namespace detail

// min(x, ys...) is Go's min: the least argument. Each of ys converts to T (a constant).
template <typename T, typename... Ts>
T min(const T& x, const Ts&... ys) {
    T r = x;
    ((r = T(detail::pick<false, T>(r, T(ys)))), ...);
    return r;
}

// max(x, ys...) is Go's max: the greatest argument.
template <typename T, typename... Ts>
T max(const T& x, const Ts&... ys) {
    T r = x;
    ((r = T(detail::pick<true, T>(r, T(ys)))), ...);
    return r;
}

} // namespace gp
