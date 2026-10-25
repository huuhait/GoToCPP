// gocpp/gc/array.hpp: gp::gc::array<T, N>, Go's [N]T in the GC engine (phase-16 README §2.2;
// job 16.1e).
//
// An array is N elements one after another, with no gaps: N times Go's size of T, aligned as T.
// A Go array is a value; copying it copies the elements, as C++'s implicit copy does. [0]T is
// zero bytes in Go; C++ cannot have a zero-byte object, so array<T, 0> is an empty struct of
// T's alignment (go_layout<array<T,0>>() is {0, alignof(T), false}). The emitter uses a
// zero_field for a [0]T struct field, so the C++ one byte never shows in a Go struct's layout.
//
//     var a [N]T               gp::gc::array<T, N> a;       the zero value: zero bytes
//     [N]T{a, b}               gp::gc::array_of<T, N>({a, b})
//     len(a), cap(a)           gp::gc::len(a), gp::gc::cap(a)   (both N)
//     a[i]                     a[i]                         a T&, bounds checked
//     a[lo:hi]                 gp::gc::sub(a, lo, hi)      a slice of a's memory
//     a[lo:], a[:hi]           gp::gc::sub(a, lo), gp::gc::sub(a, 0, hi)
//     a[lo:hi:max]             gp::gc::sub(a, lo, hi, max)
//
// Slicing needs an addressable array, in Go too, and shares its memory: the slice has no owner
// (the array is reachable where it lives, and a Go array that a slice outlives is boxed by the
// emitter). A result whose capacity is zero keeps the array's base pointer
// (golang.org/issue/8404), and even a slice of a [0]T array is non-nil, as in Go.
//
// The element type must have Go's size (golayout.hpp); the static_assert checks it for a
// non-zero-size T. T must be complete when an element is accessed.
#pragma once

#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/golayout.hpp>
#include <gocpp/gc/slice.hpp>

#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <initializer_list>
#include <type_traits>

namespace gp::gc {

template <typename T, std::int64_t N>
struct array;

// array<T, N> for N > 0: the elements, one after another.
template <typename T, std::int64_t N>
struct array {
    static_assert(N > 0, "array<T, 0> is the specialization below");

    static_assert(go_layout<T>().size == 0 || sizeof(T) == go_layout<T>().size,
                  "a GC-engine element type must have Go's size (golayout.hpp)");

    T gp_elems[N];

    // a[i]: the element at index i, with Go's bounds check.
    template <go_int I>
    T& operator[](I i) const {
        check_index(i, N);
        return const_cast<T*>(gp_elems)[static_cast<std::int64_t>(i)];
    }

    constexpr std::int64_t gp_len() const noexcept { return N; }
    T* gp_data() noexcept { return gp_elems; }
    const T* gp_data() const noexcept { return gp_elems; }
};

// array<T, 0>: no elements, aligned as T. It is 1 byte in C++ (every complete C++ object is at
// least one byte) and 0 bytes in Go.
template <typename T>
struct alignas(alignof(T)) array<T, 0> {
    template <go_int I>
    T& operator[](I i) const {
        check_index(i, 0); // always panics: a [0]T has no element
        return *detail::empty_data<T>();
    }

    constexpr std::int64_t gp_len() const noexcept { return 0; }
    // A non-null pointer, as Go's slice of a [0]T array has; it is never dereferenced.
    T* gp_data() noexcept { return reinterpret_cast<T*>(this); }
    const T* gp_data() const noexcept { return reinterpret_cast<const T*>(this); }
};

// array_of<T, N>({a, b}): [N]T{a, b}. The values are evaluated left to right; the ones after
// the list are zero values. N must match the list, as Go requires.
template <typename T, std::int64_t N>
array<T, N> array_of(std::initializer_list<T> elems) {
    static_assert(N > 0);
    // Go's type checker rejects a longer literal; a longer one here is an emitter bug, and an
    // out-of-bounds write would hide it. Fail loudly, also in a release build.
    if (elems.size() > static_cast<std::size_t>(N)) [[unlikely]] {
        std::fputs("gocpp: array literal has more values than its length\n", stderr);
        std::abort();
    }
    array<T, N> a{};
    std::int64_t i = 0;
    for (const T& e : elems) {
        a.gp_elems[i++] = e;
    }
    return a;
}

// len(a), cap(a): both N, as in Go.
template <typename T, std::int64_t N>
constexpr std::int64_t len(const array<T, N>& a) noexcept {
    return a.gp_len();
}
template <typename T, std::int64_t N>
constexpr std::int64_t cap(const array<T, N>& a) noexcept {
    return a.gp_len();
}

// sub(a, lo, hi): a[lo:hi], a slice of the array's memory. Go's checks: 0 <= lo <= hi <= N, with
// "with length N" in the messages. A result with capacity zero keeps the array's base pointer.
template <typename T, std::int64_t N, go_int L, go_int H>
slice<T> sub(array<T, N>& a, L lo, H hi) {
    check_slice(lo, hi, N, bounds::slice_alen);
    auto l = static_cast<std::int64_t>(lo);
    auto h = static_cast<std::int64_t>(hi);
    std::int64_t c = N - l;
    T* d = (c == 0 || detail::no_data<T>) ? a.gp_data() : a.gp_data() + l;
    return slice<T>(d, h - l, c);
}

template <typename T, std::int64_t N, go_int L>
slice<T> sub(array<T, N>& a, L lo) {
    return sub(a, lo, N);
}

// sub(a, lo, hi, max): a[lo:hi:max], with 0 <= lo <= hi <= max <= N. The result has capacity
// max - lo.
template <typename T, std::int64_t N, go_int L, go_int H, go_int M>
slice<T> sub(array<T, N>& a, L lo, H hi, M max) {
    check_slice3(lo, hi, max, N, bounds::slice3_alen);
    auto l = static_cast<std::int64_t>(lo);
    auto h = static_cast<std::int64_t>(hi);
    std::int64_t c = static_cast<std::int64_t>(max) - l;
    T* d = (c == 0 || detail::no_data<T>) ? a.gp_data() : a.gp_data() + l;
    return slice<T>(d, h - l, c);
}

} // namespace gp::gc
