// gocpp/array.hpp: gp::array<T, N>, Go's array type [N]T.
//
// An array is a value: N elements stored in the object itself, with no owner and no header,
// so gp::array<T, N> is exactly N * sizeof(T) bytes with T's alignment (gocppSizes says so,
// sizes.hpp checks examples). Copying an array copies every element (Go's value semantics:
// the copy and the original are independent), and passing one to a function or assigning it
// copies it too. The zero value has every element zero (value-initialized): a declared array
// is always zeroed, also without an initializer.
//
// [0]T is gp::array<T, 0>: an empty struct of 1 byte with alignment 1, like gocpp's struct{}
// (D15; Go's is 0 bytes). It is a specialization of its own: std::array<T, 0> is 1 byte in
// libstdc++ but has T's size and alignment in libc++.
//
// gp::array is an aggregate whose only data member is gp_elems, a std::array<T, N>, so a
// braced list initializes it in place. A literal can initialize a constinit package variable
// when it is a constant expression:
//   - a positional literal whose elements are all constants: numbers, bools, nested arrays of
//     them, and T() zeros of any runtime type (gp::string(), a nil gp::ptr or gp::slice); a
//     string element only as gp::string(gp::static_bytes, data, len), the constexpr form
//     ("..."_gs is not constexpr in C++20);
//   - a keyed literal (array_keyed) only for a literal type T: numbers, bools and nested
//     arrays of them, not strings, pointers or slices (it builds the array in a constexpr
//     function, which needs T's destructor to be constexpr).
// Other package-level arrays are initialized in gp_var_init (step 3.16). What generated code
// writes (Phase 03 §2.3; the emitter is step 3.6b):
//
//     var a [3]int            gp::array<int64_t, 3> a;           all zero
//     [3]int{1, 2, 3}         gp::array<int64_t, 3>{1, 2, 3}     elements left to right; the
//     [5]int{1, 2}            gp::array<int64_t, 5>{1, 2}        missing ones are zero
//     [5]int{2: x, 0: y}      gp::array_keyed<int64_t, 5>({{2, x}, {0, y}})
//                                                                keyed: evaluated in source
//                                                                order, the rest zero
//     [...]T{...}             the N that go/types computed
//     a[i]                    a[i]                               bounds checked
//     a == b, a != b          a == b, a != b                     when T is comparable
//     map[[N]T]V keys         a.gp_hash()                        elementwise (gocpp/hash.hpp):
//                                                                +0 and -0 alike, a NaN new
//     len(a), cap(a)          gp::len(a), gp::cap(a)             constexpr (Go: a constant)
//     len(p), cap(p) (*[N]T)  gp::len(p), gp::cap(p)             N, also for a nil p, as in Go
//     [N]T(s)                 gp::to_array<N>(s)                 a copy of s[0:N]
//     (*[N]T)(s)              gp::to_array_ptr<N>(s)             shares s's array
//     arr[lo:hi] (arr boxed)  gp::sub(arr, lo, hi)               arr is the box: a
//                                                                gp::ptr<gp::array<T, N>>
//     arr[lo:], arr[:]        gp::sub(arr, lo), gp::sub(arr, 0)
//     arr[lo:hi:max]          gp::sub(arr, lo, hi, max)
//     p[lo:hi] (p is *[N]T)   gp::sub(p, lo, hi)                 a nil p panics first
//     &arr[i] (arr boxed)     gp::ptr<T>(arr, &(*arr)[i])        aliasing: shares the box
//     &f()[i] (f() is *[N]T)  gp::elem_ptr(f(), i)               the same, p evaluated once
//     x := [N]T{...} (large)  gp::new_array<T, N>({...}), new_array_keyed, new_to_array<N>(s)
//                             a boxed variable over 128 KiB, built in its block (D24)
//     for i, v := range a     for (auto gp_it0 = gp::range_array_copy(a); gp::range_next(gp_it0);)
//                             (or range_array(a) when the body can't change a, range_array(p)
//                             for p of type *[N]T; see range_array below)
//
// Each element of a literal is a full C++ expression of type T: the emitter writes a zero
// element as T() (gp::string(), gp::slice<U>()), never as a bare {}, which C++ would take as
// the initializer of gp_elems itself. A nested array's elements name their type:
// [2][2]int{{1, 2}, {3, 4}} is gp::array<gp::array<int64_t, 2>, 2>{gp::array<int64_t, 2>{1, 2},
// gp::array<int64_t, 2>{3, 4}}. The keys of a keyed literal are constants that go/types checked
// (0 <= key < N, no key twice); array_keyed checks the range again (a bad key would be a bug in
// the generated code: it panics like an index, and stops a constexpr initialization from
// compiling).
//
// An index can have any Go integer type, and an index outside [0, N) panics with Go's message,
// "runtime error: index out of range [5] with length 3" (gp::check_index). A constant index
// out of range is a compile error in Go (go/types reports it), so at run time only variable
// indexes fail; the check of a constant index folds away at -O2.
//
// Slicing an array needs an addressable array, in Go too, and the slice shares the array's
// memory: a slice is {owner, data, len, cap}, so the array must live in a heap block. The
// boxing rule (Phase 03 §2.2, step 3.3) boxes every array variable that is sliced (arr[:],
// x.arr[:]), so the one primitive is gp::sub on a gp::ptr<gp::array<T, N>>, the pointer to the
// array that shares its block's owner:
//
//     a boxed local arr         the box itself: gp::sub(arr, lo, hi)
//     x.arr, x boxed            gp::sub(gp::ptr<gp::array<T, N>>(x, &x->arr), lo, hi)
//     s[i].arr                  gp::sub(gp::ptr<gp::array<T, N>>(s.gp_owner(), &s[i].arr), ...)
//     a package-level array     gp::sub(gp::static_ptr(global), lo, hi)  (the static owner)
//     p[lo:hi], p of *[N]T      gp::sub(p, lo, hi)
//
// The slice keeps the whole block alive (the box outlives the variable if a slice of it
// does), and writes through the slice and through the array see each other, as in Go. The
// bounds are those of an array: 0 <= lo <= hi <= N, Go's messages say "with length N"
// (bounds.hpp). A nil pointer panics with the nil dereference before any bounds check, as in
// Go. A slice of capacity 0 points at the first element (Go's rule, never past the end); a
// slice of a [0]T array is empty, not nil, like slice_of({}), and has no owner.
//
// Converting a slice to an array (Go 1.20): [N]T(s) copies the first N elements; (*[N]T)(s)
// is a pointer to s's first N elements, sharing s's owner, so writes through it are seen by s.
// Both panic when len(s) < N with Go's "cannot convert slice with length 2 to array or pointer
// to array with length 3" (gp::check_convert). (*[N]T)(s) of a nil s is nil when N is 0 (and
// panics otherwise); of an empty non-nil s it is a non-nil pointer that shares the static
// owner (an empty slice has none, and a non-nil gp::ptr must have one).
//
// The pointer points at s's elements as a gp::array<T, N>, which has their layout (the
// static_asserts below), but no gp::array object exists at that address: the store is a T[]
// (the slice's backing array). Accessing it through the gp::array is defined only because
// every gocpp compile uses -fno-strict-aliasing (internal/build/flags.go), as for
// gp::ptr_cast. Without it, GCC treats accesses through a gp::array<T, 2> and a
// gp::array<T, 3> as non-aliasing, so two views of one slice ((*[2]T)(s) and (*[3]T)(s)) miss
// each other's writes at -O2 (test/array_test.cpp, views_alias, checks it).
//
// Assignment is elementwise (std::array's), like the assignment of a struct the emitter
// writes. So, like a struct's, it is unsafe when the right side lives in a block that only the
// left side keeps alive (a = a[0].next_arr, where a[0] is the only pointer to that block):
// the emitter must copy the right side into a temporary first there (the rule of the 3.5a
// review for structs). gp::slice and gp::ptr do it themselves; an aggregate can't, because a
// user-declared copy assignment would take away the implicit moves.
//
// Hashing (array map keys) is step 3.7b: it hooks in at the end of this file.
//
// Like every runtime type, gp::array has only operators and gp_-prefixed members, and
// everything else is a free function in gp:: (Phase 03 §2.1). The calls inside these templates
// are qualified (gp::check_index), so argument-dependent lookup in the namespace of a user's T
// never picks a user's function. T must be complete, as in Go ([N]T needs T's size).
#pragma once

#include <gocpp/bounds.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>

#include <array>
#include <concepts>
#include <cstddef>
#include <cstdint>
#include <cstring>
#include <initializer_list>
#include <memory>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {

// index_fail: the panic of check_index, for an index that is known to be out of range.
template <go_int I>
[[noreturn]] void index_fail(I i, int64_t len) {
    if constexpr (std::is_signed_v<I>) {
        panic_index(static_cast<int64_t>(i), true, len);
    } else {
        panic_index(static_cast<int64_t>(static_cast<uint64_t>(i)), false, len);
    }
}

} // namespace detail

// array<T, N> is Go's [N]T. The zero value has every element zero.
template <typename T, int64_t N>
struct array {
    static_assert(N >= 0, "an array length is never negative");
    static_assert(sizeof(std::array<T, static_cast<std::size_t>(N)>) ==
                          static_cast<std::size_t>(N) * sizeof(T) &&
                      alignof(std::array<T, static_cast<std::size_t>(N)>) == alignof(T),
                  "[N]T is N elements of T without gaps, with T's alignment (gocppSizes, D15)");

    // The elements. Public, so that a braced list initializes them in place (an aggregate),
    // and for the runtime's own code; generated code indexes with a[i].
    std::array<T, static_cast<std::size_t>(N)> gp_elems{};

    // a[i]: the element at index i, with Go's bounds check. A const array gives a const
    // element (an array is a value, unlike a slice).
    template <go_int I>
    T& operator[](I i) {
        gp::check_index(i, N);
        return gp_elems[static_cast<std::size_t>(i)];
    }
    template <go_int I>
    const T& operator[](I i) const {
        gp::check_index(i, N);
        return gp_elems[static_cast<std::size_t>(i)];
    }

    // a == b: every element equal, compared in index order, as Go does: floats compare as
    // numbers (NaN != NaN, +0 == -0). Only when T is comparable: arrays of slices, maps and
    // funcs aren't, as in Go. != comes from == (C++20).
    friend constexpr bool operator==(const array& a, const array& b)
        requires std::equality_comparable<T>
    {
        for (std::size_t i = 0; i < static_cast<std::size_t>(N); i++) {
            if (!(a.gp_elems[i] == b.gp_elems[i])) {
                return false;
            }
        }
        return true;
    }

    // gp_hash: the hash of the elements in index order, for map keys (gocpp/hash.hpp). Only
    // when T is comparable, like ==.
    std::size_t gp_hash() const
        requires std::equality_comparable<T>
    {
        return gp::hash_elems(gp_elems.data(), N);
    }
};

// array<T, 0> is [0]T: an empty struct (1 byte, alignment 1). Every index panics, and two of
// them are always equal.
template <typename T>
struct array<T, 0> {
    template <go_int I>
    [[noreturn]] T& operator[](I i) {
        detail::index_fail(i, 0);
    }
    template <go_int I>
    [[noreturn]] const T& operator[](I i) const {
        detail::index_fail(i, 0);
    }

    friend constexpr bool operator==(const array&, const array&) noexcept
        requires std::equality_comparable<T>
    {
        return true;
    }

    // gp_hash: all [0]T are equal, so they hash the same (an empty struct's hash).
    std::size_t gp_hash() const
        requires std::equality_comparable<T>
    {
        return gp::hash_fields();
    }
};

static_assert(sizeof(array<int64_t, 0>) == 1 && alignof(array<int64_t, 0>) == 1,
              "[0]T is an empty struct: 1 byte, alignment 1 (D15)");
static_assert(sizeof(array<int16_t, 3>) == 6 && alignof(array<int16_t, 3>) == 2,
              "[3]int16 is 6 bytes with alignment 2");

// len(a), cap(a): N.
template <typename T, int64_t N>
constexpr int64_t len(const array<T, N>&) noexcept {
    return N;
}

template <typename T, int64_t N>
constexpr int64_t cap(const array<T, N>&) noexcept {
    return N;
}

// len(p), cap(p) for p of type *[N]T: N, without dereferencing p (Go doesn't evaluate p, so
// a nil p gives N too).
template <typename T, int64_t N>
constexpr int64_t len(const ptr<array<T, N>>&) noexcept {
    return N;
}

template <typename T, int64_t N>
constexpr int64_t cap(const ptr<array<T, N>>&) noexcept {
    return N;
}

// array_keyed<T, N>({{k, v}, ...}): the keyed literal [N]T{k: v, ...}. The elements are
// evaluated in the order written (a braced list), each is stored at its key, and the elements
// without a key are zero. The emitter turns a mixed literal ([5]int{1, 3: 2, 7}) into keys for
// every element ({{0, 1}, {3, 2}, {4, 7}}). Each element is copied twice: into the
// initializer_list's pair, then into the array (a Phase 15 idea: store moved values). The
// keys are constants that Go's checker has checked: a key outside [0, N) would be a bug in the
// generated code, and panics like an index, except with GOCPP_NOCHECKS (like slice_keyed's).
template <typename T, int64_t N>
constexpr array<T, N> array_keyed(std::initializer_list<std::pair<int64_t, T>> elems) {
    array<T, N> a;
    if constexpr (N > 0) {
        for (const std::pair<int64_t, T>& e : elems) {
#ifndef GOCPP_NOCHECKS
            if (e.first < 0 || e.first >= N) [[unlikely]] {
                detail::index_fail(e.first, N);
            }
#endif
            a.gp_elems[static_cast<std::size_t>(e.first)] = e.second;
        }
    } else {
#ifndef GOCPP_NOCHECKS
        if (elems.size() != 0) [[unlikely]] {
            detail::index_fail(elems.begin()->first, 0);
        }
#endif
    }
    return a;
}

// to_array<N>(s): the conversion [N]T(s), a new array that holds a copy of s[0:N]. Panics
// with Go's message when len(s) < N. Writes to the array don't change s. The array is
// zero-filled first and then copied over (a Phase 15 idea: construct the elements from s).
template <int64_t N, typename T>
array<T, N> to_array(const slice<T>& s) {
    gp::check_convert(N, gp::len(s));
    array<T, N> a;
    if constexpr (N > 0) {
        const T* src = s.gp_data();
        if constexpr (detail::zero_bits<T>) {
            std::memcpy(static_cast<void*>(a.gp_elems.data()), static_cast<const void*>(src),
                        static_cast<std::size_t>(N) * sizeof(T));
        } else {
            for (std::size_t i = 0; i < static_cast<std::size_t>(N); i++) {
                a.gp_elems[i] = src[i];
            }
        }
    }
    return a;
}

// to_array_ptr<N>(s): the conversion (*[N]T)(s), a pointer to s's first N elements that
// shares s's array (and keeps it alive). Panics with Go's message when len(s) < N. A nil s
// gives nil (only when N is 0: otherwise len(s) < N).
template <int64_t N, typename T>
ptr<array<T, N>> to_array_ptr(const slice<T>& s) {
    gp::check_convert(N, gp::len(s));
    T* d = s.gp_data();
    if (d == nullptr) {
        return nullptr;
    }
    auto* p = reinterpret_cast<array<T, N>*>(d);
    if (s.gp_owner() == nullptr) {
        // The only non-nil slice without an owner: an empty one whose data is the static
        // zerobase ([]T{}, make([]T, 0)). A non-nil pointer needs an owner, and there is
        // nothing to free. (A slice of a package-level array shares static_owner() already.)
        return ptr<array<T, N>>(static_owner(), p);
    }
    return ptr<array<T, N>>(s.gp_owner(), p);
}

// sub(p, lo, hi): arr[lo:hi] for the array that p points at (a boxed array, an array inside a
// heap block, or *p for p of type *[N]T): a slice of its elements that shares p's owner.
// Checks 0 <= lo <= hi <= N, else Go's "slice bounds out of range [:hi] with length N" or
// "[lo:hi]"; a nil p panics with the nil dereference first. sub(p, lo) is arr[lo:].
template <typename T, int64_t N, go_int L, go_int H>
slice<T> sub(ptr<array<T, N>> p, L lo, H hi) {
    array<T, N>& a = *p;
    gp::check_slice(lo, hi, N, bounds::slice_alen);
    if constexpr (N == 0) {
        (void)a;
        return slice<T>(std::shared_ptr<void>(), detail::empty_data<T>(), 0, 0);
    } else {
        auto l = static_cast<int64_t>(lo);
        auto h = static_cast<int64_t>(hi);
        T* base = a.gp_elems.data();
        int64_t c = N - l;
        return slice<T>(std::shared_ptr<void>(std::move(p).gp_take()), c == 0 ? base : base + l,
                        h - l, c);
    }
}

template <typename T, int64_t N, go_int L>
slice<T> sub(ptr<array<T, N>> p, L lo) {
    return gp::sub(std::move(p), lo, N);
}

// sub(p, lo, hi, max): arr[lo:hi:max]: 0 <= lo <= hi <= max <= N, else Go's panics
// "[::max] with length N", "[:hi:max]" or "[lo:hi:]". The result has capacity max - lo.
template <typename T, int64_t N, go_int L, go_int H, go_int M>
slice<T> sub(ptr<array<T, N>> p, L lo, H hi, M max) {
    array<T, N>& a = *p;
    gp::check_slice3(lo, hi, max, N, bounds::slice3_alen);
    if constexpr (N == 0) {
        (void)a;
        return slice<T>(std::shared_ptr<void>(), detail::empty_data<T>(), 0, 0);
    } else {
        auto l = static_cast<int64_t>(lo);
        auto h = static_cast<int64_t>(hi);
        T* base = a.gp_elems.data();
        int64_t c = static_cast<int64_t>(max) - l;
        return slice<T>(std::shared_ptr<void>(std::move(p).gp_take()), c == 0 ? base : base + l,
                        h - l, c);
    }
}

// new_array, new_array_keyed, new_to_array: gp::new_<array<T, N>>(...) of a literal, a keyed
// literal or [N]T(s), built in the new block itself. The emitter writes them for a boxed
// variable larger than 128 KiB (x := [1 << 21]int{5: 3}), where gp::new_ of the value would
// first build it in a temporary on the C++ stack, which could overflow it (D24). The elements
// are evaluated in source order, and the others are zero, as in array{...} and array_keyed
// (also its check of the keys, and of the number of elements).
template <typename T, int64_t N>
ptr<array<T, N>> new_array(std::initializer_list<T> elems) {
    ptr<array<T, N>> p = gp::new_<array<T, N>>();
    if constexpr (N > 0) {
        std::size_t i = 0;
        for (const T& e : elems) {
#ifndef GOCPP_NOCHECKS
            if (i >= static_cast<std::size_t>(N)) [[unlikely]] {
                detail::index_fail(static_cast<int64_t>(i), N);
            }
#endif
            p->gp_elems[i++] = e;
        }
    }
    return p;
}

template <typename T, int64_t N>
ptr<array<T, N>> new_array_keyed(std::initializer_list<std::pair<int64_t, T>> elems) {
    ptr<array<T, N>> p = gp::new_<array<T, N>>();
    for (const std::pair<int64_t, T>& e : elems) {
#ifndef GOCPP_NOCHECKS
        if (e.first < 0 || e.first >= N) [[unlikely]] {
            detail::index_fail(e.first, N);
        }
#endif
        if constexpr (N > 0) {
            p->gp_elems[static_cast<std::size_t>(e.first)] = e.second;
        }
    }
    return p;
}

template <int64_t N, typename T>
ptr<array<T, N>> new_to_array(const slice<T>& s) {
    gp::check_convert(N, gp::len(s));
    ptr<array<T, N>> p = gp::new_<array<T, N>>();
    if constexpr (N > 0) {
        const T* src = s.gp_data();
        for (std::size_t i = 0; i < static_cast<std::size_t>(N); i++) {
            p->gp_elems[i] = src[i];
        }
    }
    return p;
}

// elem_ptr(p, i): &(*p)[i] for a pointer p to an array: a pointer to the element that shares
// p's owner (p is evaluated once). The emitter writes it where p is not a variable (&f()[i])
// and for each step of &x[i][j] whose owner is not a variable; for a variable p it writes
// gp::ptr<T>(p, &(*p)[i]) itself. A nil p panics with the nil dereference before the index
// check, as in Go.
template <typename T, int64_t N, go_int I>
ptr<T> elem_ptr(const ptr<array<T, N>>& p, I i) {
    T& e = (*p)[i];
    return ptr<T>(p, &e);
}

namespace detail {

// array_elem: a.gp_elems[i] for an index that a range loop knows is in [0, N).
template <typename T, int64_t N>
const T& array_elem(const array<T, N>& a, int64_t i) noexcept {
    if constexpr (N == 0) {
        return a[i]; // never called: a range over [0]T has no iteration (and this panics)
    } else {
        return a.gp_elems[static_cast<std::size_t>(i)];
    }
}

// large_array: an array too large to copy onto the C++ stack, Go's limit for a variable on the
// stack (128 KiB, the boxing rule of the emitter's internal/lower/box.go): a range copies
// such an array to the heap.
template <typename T, int64_t N>
inline constexpr bool large_array = sizeof(array<T, N>) > 128 * 1024;

} // namespace detail

// The range loops over arrays with a value variable (`for i, v := range x`; the emitter writes
// the loops without one, `for i := range x`, as a counter to N, which doesn't evaluate x when
// len(x) is a constant, as in Go). As for slices, gp_i is the hidden counter (Go 1.22: assigning
// to i in the body doesn't change the iteration) and gp_v() the element at gp_i, read when it
// is called:
//
//     range_array(a)        a reference to the array a: for an array that the body can't change
//                           (the emitter decides it), so the copy below is not needed
//     range_array_copy(a)   a copy of a, taken once when the loop starts: Go ranges over a copy
//                           of an array, so writes to a in the body are not seen; an array
//                           larger than 128 KiB is copied to the heap
//     range_array(p)        p of type *[N]T: a copy of the pointer (p is evaluated once), and
//                           each gp_v() reads (*p)[gp_i], so writes in the body are seen, as in
//                           Go; a nil p panics at the first gp_v() (no iteration for N == 0)

template <typename T, int64_t N>
struct array_range {
    const array<T, N>* gp_a;
    int64_t gp_i = -1;

    const T& gp_v() const noexcept { return detail::array_elem(*gp_a, gp_i); }
};

template <typename T, int64_t N>
struct array_copy_range {
    // The copy, on the heap when it is large.
    std::conditional_t<detail::large_array<T, N>, std::unique_ptr<const array<T, N>>, array<T, N>>
        gp_a;
    int64_t gp_i = -1;

    const T& gp_v() const noexcept {
        if constexpr (detail::large_array<T, N>) {
            return detail::array_elem(*gp_a, gp_i);
        } else {
            return detail::array_elem(gp_a, gp_i);
        }
    }
};

template <typename T, int64_t N>
struct array_ptr_range {
    ptr<array<T, N>> gp_p;
    int64_t gp_i = -1;

    const T& gp_v() const { return detail::array_elem(*gp_p, gp_i); }
};

// range_array(a) starts a range over the array a, which must outlive the loop.
template <typename T, int64_t N>
array_range<T, N> range_array(const array<T, N>& a) noexcept {
    return array_range<T, N>{&a};
}

// range_array_copy(a) starts a range over a copy of the array a.
template <typename T, int64_t N>
array_copy_range<T, N> range_array_copy(const array<T, N>& a) {
    if constexpr (detail::large_array<T, N>) {
        return array_copy_range<T, N>{std::make_unique<const array<T, N>>(a)};
    } else {
        return array_copy_range<T, N>{a};
    }
}

// range_array_copy(f()) of a temporary moves it into the range instead of copying it.
template <typename T, int64_t N>
array_copy_range<T, N> range_array_copy(array<T, N>&& a) {
    if constexpr (detail::large_array<T, N>) {
        return array_copy_range<T, N>{std::make_unique<const array<T, N>>(std::move(a))};
    } else {
        return array_copy_range<T, N>{std::move(a)};
    }
}

// range_array(p) starts a range over the array that p points to (p is *[N]T).
template <typename T, int64_t N>
array_ptr_range<T, N> range_array(ptr<array<T, N>> p) noexcept {
    return array_ptr_range<T, N>{std::move(p)};
}

// range_next steps to the next index of a range over an array, and reports whether there is
// one.
template <typename T, int64_t N>
bool range_next(array_range<T, N>& it) noexcept {
    return ++it.gp_i < N;
}

template <typename T, int64_t N>
bool range_next(array_copy_range<T, N>& it) noexcept {
    return ++it.gp_i < N;
}

template <typename T, int64_t N>
bool range_next(array_ptr_range<T, N>& it) noexcept {
    return ++it.gp_i < N;
}

} // namespace gp
