// gocpp/gc/checks.hpp: the GC engine's checks, with Go's messages and panic types (phase-16
// README §2.2; job 16.1e).
//
// The generated code calls these for indexing, slicing, nil dereferences and the make/append
// growth checks. They throw gp::gc::panic_error (gocpp/gc/panic.hpp) with Go's exact text:
//
//     s[5] on len 3        runtime error: index out of range [5] with length 3
//     s[-1]                runtime error: index out of range [-1]
//     s[:6] on cap 5       runtime error: slice bounds out of range [:6] with capacity 5
//     s[2:1]               runtime error: slice bounds out of range [2:1]
//     s[0:4:2] on cap 5    runtime error: slice bounds out of range [:4:2]
//     *p, p == nil         runtime error: invalid memory address or nil pointer dereference
//     x / 0                runtime error: integer divide by zero
//     x << -1              runtime error: negative shift amount
//     make([]T, -1)        runtime error: makeslice: len out of range
//     make([]T, 3, 2)      runtime error: makeslice: cap out of range
//     append's overflow    runtime error: growslice: len out of range
//
// The bounds checks run in Go's order (cmd/compile's ssagen, state.slice): the last index
// first, so each index is compared with a value already known to be in range.
//
// GOCPP_NOCHECKS (gocpp build -nochecks), which the emitter defines for every file of a
// program, turns off the index, bounds and nil checks, as it does in the ARC engine. It does
// not turn off the division, shift, make and growslice panics: those are part of what the
// operation does, and C++ has undefined behavior there.
#pragma once

#include <concepts>
#include <cstdint>
#include <type_traits>

namespace gp::gc {

// go_int: a C++ type that holds a Go integer type (int8_t ... uint64_t, and the other integer
// types of the same sizes), but not bool.
template <typename T>
concept go_int = std::integral<T> && !std::same_as<std::remove_cv_t<T>, bool>;

// bounds names the failed check: Go's boundsErrorCode, without boundsIndex (that is
// panic_index).
enum class bounds : std::uint8_t {
    slice_alen,  // x[?:hi], 0 <= hi <= len(x) failed (a string or an array)
    slice_acap,  // s[?:hi], 0 <= hi <= cap(s) failed (a slice)
    slice_b,     // x[lo:hi], 0 <= lo <= hi failed
    slice3_alen, // x[?:?:max], 0 <= max <= len(x) failed (an array)
    slice3_acap, // s[?:?:max], 0 <= max <= cap(s) failed (a slice)
    slice3_b,    // x[?:hi:max], 0 <= hi <= max failed
    slice3_c,    // x[lo:hi:?], 0 <= lo <= hi failed
    convert,     // [N]T(s), N <= len(s) failed: x is N, y is len(s)
};

// In runtime/src/gc/panic.cpp.

// panic_index: "runtime error: index out of range [5] with length 3". A negative signed index
// prints without the length, like Go: "index out of range [-1]". When is_signed is false, x is
// the bits of an unsigned index (uint64), and it prints as unsigned.
[[noreturn]] void panic_index(std::int64_t x, bool is_signed, std::int64_t len);

// panic_bounds throws Go's panic for a failed slice expression: "runtime error: slice bounds
// out of range ...". x is the index that failed (the bits of a uint64 when is_signed is false),
// y the value it was compared with.
[[noreturn]] void panic_bounds(bounds code, std::int64_t x, bool is_signed, std::int64_t y);

// panic_nil: "runtime error: invalid memory address or nil pointer dereference", the panic of a
// nil dereference, of a nil func call and of a nil interface method call.
[[noreturn]] void panic_nil();

// panic_divide: "runtime error: integer divide by zero".
[[noreturn]] void panic_divide();

// panic_shift: "runtime error: negative shift amount".
[[noreturn]] void panic_shift();

// panic_makeslicelen and panic_makeslicecap: "runtime error: makeslice: len out of range" and
// "makeslice: cap out of range", as Go's makeslice picks them.
[[noreturn]] void panic_makeslicelen();
[[noreturn]] void panic_makeslicecap();

// panic_growslice: "runtime error: growslice: len out of range", for append's length overflow
// and for an array above Go's maximum allocation.
[[noreturn]] void panic_growslice();

// panic_uncomparable: "runtime error: comparing uncomparable type []int", for == of two
// interface values whose dynamic type is a slice, map or func. name is the type's Go name; the
// placeholder descriptors carry an approximate one until step 16.8.
[[noreturn]] void panic_uncomparable(const char* name);

// panic_nil_map: "assignment to entry in nil map", a runtime.plainError without the "runtime
// error: " prefix.
[[noreturn]] void panic_nil_map();

namespace detail {

// in_bounds: 0 <= i <= bound, for an index of any integer type.
template <go_int I>
constexpr bool in_bounds(I i, std::int64_t bound) noexcept {
    if constexpr (std::is_signed_v<I>) {
        return i >= 0 && static_cast<std::int64_t>(i) <= bound;
    } else {
        return static_cast<std::uint64_t>(i) <= static_cast<std::uint64_t>(bound);
    }
}

template <go_int I>
[[noreturn]] void bounds_fail(bounds code, I i, std::int64_t y) {
    if constexpr (std::is_signed_v<I>) {
        panic_bounds(code, static_cast<std::int64_t>(i), true, y);
    } else {
        panic_bounds(code, static_cast<std::int64_t>(static_cast<std::uint64_t>(i)), false, y);
    }
}

} // namespace detail

// check_index panics with panic_index unless 0 <= i < len.
template <go_int I>
inline void check_index([[maybe_unused]] I i, [[maybe_unused]] std::int64_t len) {
#ifndef GOCPP_NOCHECKS
    if constexpr (std::is_signed_v<I>) {
        if (i < 0 || static_cast<std::int64_t>(i) >= len) [[unlikely]] {
            panic_index(static_cast<std::int64_t>(i), true, len);
        }
    } else {
        if (static_cast<std::uint64_t>(i) >= static_cast<std::uint64_t>(len)) [[unlikely]] {
            panic_index(static_cast<std::int64_t>(static_cast<std::uint64_t>(i)), false, len);
        }
    }
#endif
}

// check_slice: Go's checks for x[lo:hi]. bound is cap(x) for a slice (first is
// bounds::slice_acap) or len(x) for a string or an array (bounds::slice_alen).
template <go_int L, go_int H>
inline void check_slice([[maybe_unused]] L lo, [[maybe_unused]] H hi,
                        [[maybe_unused]] std::int64_t bound, [[maybe_unused]] bounds first) {
#ifndef GOCPP_NOCHECKS
    if (!detail::in_bounds(hi, bound)) [[unlikely]] {
        detail::bounds_fail(first, hi, bound);
    }
    if (!detail::in_bounds(lo, static_cast<std::int64_t>(hi))) [[unlikely]] {
        detail::bounds_fail(bounds::slice_b, lo, static_cast<std::int64_t>(hi));
    }
#endif
}

// check_slice3: Go's checks for x[lo:hi:max]. bound is cap(x) for a slice (first is
// bounds::slice3_acap) or len(x) for an array (bounds::slice3_alen).
template <go_int L, go_int H, go_int M>
inline void check_slice3([[maybe_unused]] L lo, [[maybe_unused]] H hi, [[maybe_unused]] M max,
                         [[maybe_unused]] std::int64_t bound, [[maybe_unused]] bounds first) {
#ifndef GOCPP_NOCHECKS
    if (!detail::in_bounds(max, bound)) [[unlikely]] {
        detail::bounds_fail(first, max, bound);
    }
    if (!detail::in_bounds(hi, static_cast<std::int64_t>(max))) [[unlikely]] {
        detail::bounds_fail(bounds::slice3_b, hi, static_cast<std::int64_t>(max));
    }
    if (!detail::in_bounds(lo, static_cast<std::int64_t>(hi))) [[unlikely]] {
        detail::bounds_fail(bounds::slice3_c, lo, static_cast<std::int64_t>(hi));
    }
#endif
}

// check_convert: Go's check for [N]T(s) and (*[N]T)(s): n (the array length N) <= len.
inline void check_convert([[maybe_unused]] std::int64_t n, [[maybe_unused]] std::int64_t len) {
#ifndef GOCPP_NOCHECKS
    if (n > len) [[unlikely]] {
        panic_bounds(bounds::convert, n, true, len);
    }
#endif
}

// check_make is make's check: Go's makeslice rejects a negative length, and a length or
// capacity above maxAlloc/size (size is Go's size of the element; 0 accepts any length up to
// math.MaxInt). A capacity below the length is "cap out of range". Go's makeslice reports the
// length when the length alone is bad (make([]T, huge) has only a length), and the capacity
// otherwise.
inline void check_make(std::int64_t len, std::int64_t cap, std::uint64_t size) {
    constexpr std::uint64_t max_alloc = (std::uint64_t{1} << 48) - 1; // Go's maxAlloc
    const std::uint64_t limit =
        size == 0 ? static_cast<std::uint64_t>(INT64_MAX) : max_alloc / size;
    if (len < 0 || static_cast<std::uint64_t>(len) > limit) [[unlikely]] {
        panic_makeslicelen();
    }
    if (cap < len || static_cast<std::uint64_t>(cap) > limit) [[unlikely]] {
        panic_makeslicecap();
    }
}

// deref returns *p, with Go's nil check: a nil pointer panics with "runtime error: invalid
// memory address or nil pointer dereference".
template <typename T>
inline T& deref([[maybe_unused]] T* p) {
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) [[unlikely]] {
        panic_nil();
    }
#endif
    return *p;
}

// check_nil panics like a nil dereference unless p is not null; it returns p for chaining.
template <typename T>
inline T* check_nil([[maybe_unused]] T* p) {
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) [[unlikely]] {
        panic_nil();
    }
#endif
    return p;
}

} // namespace gp::gc
