// gocpp/bounds.hpp: Go's bounds checks for slice expressions (x[lo:hi], x[lo:hi:max]) and for
// the conversion of a slice to an array ([N]T(s), (*[N]T)(s)), with Go's exact panic messages.
//
// The checks are shared by everything that can be sliced: slices (gocpp/slice.hpp), strings
// and arrays. They run in Go's order (cmd/compile's ssagen, state.slice): the last index
// first, so each index is compared with a value already known to be in range.
//
//     x[lo:hi]        hi <= bound, then lo <= hi
//     x[lo:hi:max]    max <= bound, then hi <= max, then lo <= hi
//
// bound is the capacity of a slice, and the length of a string or an array; Go's message says
// which ("with capacity 3", "with length 3"). An omitted lo is 0, an omitted hi is len(x), and
// an omitted max is cap(x). The messages are Go's boundsError texts (runtime/error.go), for
// example:
//
//     s[:5]   on cap 3      runtime error: slice bounds out of range [:5] with capacity 3
//     str[:5] on len 3      runtime error: slice bounds out of range [:5] with length 3
//     s[2:1]                runtime error: slice bounds out of range [2:1]
//     s[-1:]                runtime error: slice bounds out of range [-1:]
//     s[0:1:5] on cap 3     runtime error: slice bounds out of range [::5] with capacity 3
//     s[0:4:2]              runtime error: slice bounds out of range [:4:2]
//     s[3:2:4]              runtime error: slice bounds out of range [3:2:]
//     [3]int(s), len(s)=2   runtime error: cannot convert slice with length 2 to array or
//                           pointer to array with length 3
//
// An index can have any Go integer type. A negative signed index prints without the second
// number ("[:-1]", "[-1:]"), and a huge unsigned one prints as unsigned, as in Go.
#pragma once

#include <gocpp/checks.hpp>

#include <cstdint>
#include <type_traits>

namespace gp {

// bounds names the failed check: Go's boundsErrorCode, without boundsIndex (that is
// gp::panic_index).
enum class bounds : uint8_t {
    slice_alen,  // x[?:hi], 0 <= hi <= len(x) failed (a string or an array)
    slice_acap,  // s[?:hi], 0 <= hi <= cap(s) failed (a slice)
    slice_b,     // x[lo:hi], 0 <= lo <= hi failed
    slice3_alen, // x[?:?:max], 0 <= max <= len(x) failed (an array)
    slice3_acap, // s[?:?:max], 0 <= max <= cap(s) failed (a slice)
    slice3_b,    // x[?:hi:max], 0 <= hi <= max failed
    slice3_c,    // x[lo:hi:?], 0 <= lo <= hi failed
    convert,     // [N]T(s), N <= len(s) failed: x is N, y is len(s)
};

// panic_bounds throws Go's panic for a failed check: "runtime error: slice bounds out of
// range ...". x is the index that failed (the bits of a uint64 when is_signed is false), y the
// value it was compared with. In runtime/src/slice.cpp.
[[noreturn]] void panic_bounds(bounds code, int64_t x, bool is_signed, int64_t y);

namespace detail {

// in_bounds: 0 <= i <= bound, for an index of any integer type.
template <go_int I>
constexpr bool in_bounds(I i, int64_t bound) noexcept {
    if constexpr (std::is_signed_v<I>) {
        return i >= 0 && static_cast<int64_t>(i) <= bound;
    } else {
        return static_cast<uint64_t>(i) <= static_cast<uint64_t>(bound);
    }
}

template <go_int I>
[[noreturn]] void bounds_fail(bounds code, I i, int64_t y) {
    if constexpr (std::is_signed_v<I>) {
        panic_bounds(code, static_cast<int64_t>(i), true, y);
    } else {
        panic_bounds(code, static_cast<int64_t>(static_cast<uint64_t>(i)), false, y);
    }
}

} // namespace detail

// check_slice: Go's checks for x[lo:hi]. bound is cap(x) for a slice (first is
// bounds::slice_acap) or len(x) for a string or an array (bounds::slice_alen).
template <go_int L, go_int H>
inline void check_slice(L lo, H hi, int64_t bound, bounds first) {
    if (!detail::in_bounds(hi, bound)) [[unlikely]] {
        detail::bounds_fail(first, hi, bound);
    }
    if (!detail::in_bounds(lo, static_cast<int64_t>(hi))) [[unlikely]] {
        detail::bounds_fail(bounds::slice_b, lo, static_cast<int64_t>(hi));
    }
}

// check_slice3: Go's checks for x[lo:hi:max]. bound is cap(x) for a slice (first is
// bounds::slice3_acap) or len(x) for an array (bounds::slice3_alen).
template <go_int L, go_int H, go_int M>
inline void check_slice3(L lo, H hi, M max, int64_t bound, bounds first) {
    if (!detail::in_bounds(max, bound)) [[unlikely]] {
        detail::bounds_fail(first, max, bound);
    }
    if (!detail::in_bounds(hi, static_cast<int64_t>(max))) [[unlikely]] {
        detail::bounds_fail(bounds::slice3_b, hi, static_cast<int64_t>(max));
    }
    if (!detail::in_bounds(lo, static_cast<int64_t>(hi))) [[unlikely]] {
        detail::bounds_fail(bounds::slice3_c, lo, static_cast<int64_t>(hi));
    }
}

// check_convert: Go's check for [N]T(s) and (*[N]T)(s): n (the array length N) <= len.
inline void check_convert(int64_t n, int64_t len) {
    if (n > len) [[unlikely]] {
        panic_bounds(bounds::convert, n, true, len);
    }
}

} // namespace gp
