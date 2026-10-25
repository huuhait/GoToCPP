// gocpp/gc/slice.hpp: gp::gc::slice<T>, Go's []T in the GC engine (phase-16 README §2.2 and
// §2.4; job 16.1e).
//
// A slice is Go's slice header and nothing else: a pointer to the first element, a length and a
// capacity, 24 bytes. Copying a slice copies the header, so two slices of one array see each
// other's writes, as in Go. There is no owner: the collector frees the array when nothing
// reaches it (README §2.4).
//
//     nil        no array: data is null, len and cap are 0. The zero value. s == nil is true.
//     empty      []T{} and make([]T, 0): data points at Go's zerobase, len and cap are 0, and
//                s == nil is false, as in Go. No allocation.
//     elements   data points into an array of at least cap elements in the GC heap (or into an
//                array of the program, for a slice of it).
//
// The element stride is Go's size of T, which the C++ type must have: a struct the emitter
// writes has exactly Go's layout (golayout.hpp), and the functions that allocate or index check
// sizeof(T) == go_layout<T>().size. A zero-size Go type is the exception: Go points every slice
// of it at zerobase, whatever its length, and never touches an element, so make and append give
// cap == len and allocate nothing (detail::no_data). All GC-engine values are trivially
// copyable and have no destructor (the collector runs none), which is what lets a backing array
// be zeroed bytes.
//
// What generated code writes (the emitter is step 16.1f; the ARC engine's equivalents are in
// gocpp/slice.hpp):
//
//     var s []T, s = nil       gp::gc::slice<T> s;  s = nullptr
//     s == nil, s != nil       s == nullptr, s != nullptr (a slice compares only with nil)
//     []T{a, b}                gp::gc::slice_of<T>({a, b})
//     make([]T, n), (n, c)     gp::gc::make_slice<T>(n), gp::gc::make_slice<T>(n, c)
//     len(s), cap(s)           gp::gc::len(s), gp::gc::cap(s)
//     s[i]                     s[i]                        a T&, bounds checked
//     s[lo:], s[lo:hi]         gp::gc::sub(s, lo), gp::gc::sub(s, lo, hi)
//     s[:hi], s[:]             gp::gc::sub(s, 0, hi), gp::gc::sub(s, 0)
//     s[lo:hi:max]             gp::gc::sub(s, lo, hi, max)
//     append(s, a, b)          gp::gc::append(s, a, b)
//     append(s, t...)          gp::gc::append_slice(s, t)
//     copy(dst, src)           gp::gc::copy(dst, src)      also copy([]byte, string)
//     clear(s)                 gp::gc::clear(s)
//
// Slicing follows Go's rules and its no-past-the-end rule (golang.org/issue/8404): a result
// whose capacity is zero keeps the base pointer, and a slice of nil stays nil. Bounds checks
// panic with Go's messages (gocpp/gc/checks.hpp).
//
// append grows as Go's runtime does: nextslicecap (double below 256 elements, then about 1.25x
// plus 192), rounded up to gc's malloc size classes, with gc's 8-byte header for a block of
// pointer-holding elements above 512 bytes. The element size is Go's (go_layout<T>().size), so
// []string grows as Go's does (71 elements at the 33rd append, not 64). The numbers are in
// runtime/src/gc/slice.cpp; the test runtime/test/gc_values_test.cpp checks them against real
// Go's runtime.growslice.
//
// Like every runtime type, gp::gc::slice has only operators and gp_-prefixed members, and
// everything else is a free function in gp::gc. The calls inside these templates are qualified
// (gp::gc::sub, detail::grow), so argument-dependent lookup cannot find a user's function. T may
// be incomplete until a member function that needs its size is used.
#pragma once

#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/golayout.hpp>
#include <gocpp/gc/heap.hpp>
#include <gocpp/gc/string.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <initializer_list>
#include <type_traits>
#include <utility>

namespace gp::gc {

template <typename T>
class slice;

namespace detail {

// zerobase is what the data pointer of an empty, non-nil slice of a zero-size element points at
// (Go's runtime.zerobase). It is writable so that a stray store through a zero-size element is
// not undefined behavior; such an element is never read.
alignas(64) inline unsigned char zerobase[64] = {};

template <typename T>
T* empty_data() noexcept {
    return reinterpret_cast<T*>(zerobase);
}

// no_data<T>: an element that takes no memory in Go (struct{}, [0]T, a named empty struct). A
// slice of them points at zerobase, whatever its length, and none of its elements is ever
// constructed, copied or cleared.
template <typename T>
constexpr bool no_data_of() noexcept {
    return go_layout<T>().size == 0;
}
template <typename T>
inline constexpr bool no_data = no_data_of<T>();

// In runtime/src/gc/slice.cpp.

// grow_cap: the capacity of the new array when a slice of capacity old_cap must hold new_len
// elements, as Go's growslice gives it: nextslicecap, rounded up to Go's size classes. size is
// the element's size in Go, noscan tells whether the elements hold no pointer (a block with
// pointers over 512 bytes has gc's 8-byte malloc header, which the size class includes). A
// zero-size element gives new_len. Panics with "growslice: len out of range" when new_len is
// negative or the array would be bigger than Go's maxAlloc.
std::int64_t grow_cap(std::int64_t new_len, std::int64_t old_cap, std::uint64_t size, bool noscan);

// alloc_elems allocates n > 0 zeroed elements in the GC heap: scanned when T holds a pointer,
// pointer-free (atomic) otherwise.
template <typename T>
T* alloc_elems(std::int64_t n) {
    const std::size_t bytes = static_cast<std::size_t>(n) * sizeof(T);
    void* p =
        go_layout<T>().ptrs ? process_heap().alloc(bytes) : process_heap().alloc_atomic(bytes);
    return static_cast<T*>(p);
}

// grow returns s with room for new_len elements: the same elements, in a new array of the
// capacity that grow_cap gives. The old array is not touched.
template <typename T>
slice<T> grow(const slice<T>& s, std::int64_t new_len) {
    constexpr std::uint64_t size = go_layout<T>().size;
    static_assert(size == 0 || sizeof(T) == size,
                  "a GC-engine element type must have Go's size (golayout.hpp)");
    const std::int64_t new_cap = grow_cap(new_len, s.gp_cap(), size, !go_layout<T>().ptrs);
    if constexpr (size == 0) {
        return slice<T>(empty_data<T>(), s.gp_len(), new_len);
    } else {
        static_assert(std::is_trivially_copyable_v<T>);
        T* p = alloc_elems<T>(new_cap);
        if (s.gp_len() > 0) {
            std::memcpy(static_cast<void*>(p), s.gp_data(),
                        static_cast<std::size_t>(s.gp_len()) * sizeof(T));
        }
        return slice<T>(p, s.gp_len(), new_cap);
    }
}

} // namespace detail

// slice<T> is Go's []T. The zero value is nil.
template <typename T>
class slice {
public:
    // nil.
    constexpr slice() noexcept = default;
    constexpr slice(std::nullptr_t) noexcept {}

    // A slice of len elements at data, with room for cap. For the runtime's own code (make,
    // append, sub); generated code uses those.
    constexpr slice(T* data, std::int64_t len, std::int64_t cap) noexcept
        : gp_data_(data), gp_len_(len), gp_cap_(cap) {}

    // s[i]: the element at index i, with Go's bounds check. A const slice still gives a
    // writable element, as a slice is a reference to its array. A zero-size element gives the
    // same address for every index, as Go's do.
    template <go_int I>
    T& operator[](I i) const {
        check_index(i, gp_len_);
        return *elem(static_cast<std::int64_t>(i));
    }

    // s == nil. Go compares slices only with nil.
    friend bool operator==(const slice& s, std::nullptr_t) noexcept {
        return s.gp_data_ == nullptr;
    }
    friend bool operator!=(const slice& s, std::nullptr_t) noexcept {
        return s.gp_data_ != nullptr;
    }

    // For the runtime's own code and generated code that needs the parts.
    T* gp_data() const noexcept { return gp_data_; }
    std::int64_t gp_len() const noexcept { return gp_len_; }
    std::int64_t gp_cap() const noexcept { return gp_cap_; }
    bool gp_nil() const noexcept { return gp_data_ == nullptr; }

    // gp_reslice: the same array, viewed from data with len and cap.
    void gp_reslice(T* data, std::int64_t len, std::int64_t cap) noexcept {
        gp_data_ = data;
        gp_len_ = len;
        gp_cap_ = cap;
    }

private:
    T* elem(std::int64_t i) const noexcept {
        if constexpr (detail::no_data<T>) {
            return detail::empty_data<T>();
        } else {
            return gp_data_ + i;
        }
    }

    T* gp_data_ = nullptr;
    std::int64_t gp_len_ = 0;
    std::int64_t gp_cap_ = 0;
};

static_assert(sizeof(slice<std::int64_t>) == 24, "gp::gc::slice must be Go's 24-byte header");
static_assert(alignof(slice<std::int64_t>) == 8, "gp::gc::slice must be 8-byte aligned");

// len(s), cap(s).
template <typename T>
constexpr std::int64_t len(const slice<T>& s) noexcept {
    return s.gp_len();
}
template <typename T>
constexpr std::int64_t cap(const slice<T>& s) noexcept {
    return s.gp_cap();
}

// make_slice<T>(n): make([]T, n). make_slice<T>(n, c): make([]T, n, c). The elements are zero
// values. A negative or too big n panics with "makeslice: len out of range", a c below n or too
// big with "makeslice: cap out of range". A zero-size T takes no memory and accepts any length
// up to math.MaxInt. make([]T, 0) is empty but not nil.
template <typename T, go_int L, go_int C>
slice<T> make_slice(L len, C cap) {
    auto n = static_cast<std::int64_t>(len);
    auto c = static_cast<std::int64_t>(cap);
    constexpr std::uint64_t size = go_layout<T>().size;
    check_make(n, c, size);
    if (c == 0) {
        return slice<T>(detail::empty_data<T>(), 0, 0);
    }
    if constexpr (size == 0) {
        return slice<T>(detail::empty_data<T>(), n, c);
    } else {
        static_assert(sizeof(T) == size,
                      "a GC-engine element type must have Go's size (golayout.hpp)");
        static_assert(std::is_trivially_copyable_v<T>);
        return slice<T>(detail::alloc_elems<T>(c), n, c);
    }
}

template <typename T, go_int L>
slice<T> make_slice(L len) {
    return make_slice<T>(len, len);
}

// slice_of<T>({a, b}): []T{a, b}. The elements are evaluated left to right. slice_of<T>({}) is
// empty but not nil.
template <typename T>
slice<T> slice_of(std::initializer_list<T> elems) {
    auto n = static_cast<std::int64_t>(elems.size());
    if (n == 0) {
        return slice<T>(detail::empty_data<T>(), 0, 0);
    }
    constexpr std::uint64_t size = go_layout<T>().size;
    if constexpr (size == 0) {
        return slice<T>(detail::empty_data<T>(), n, n);
    } else {
        static_assert(sizeof(T) == size);
        static_assert(std::is_trivially_copyable_v<T>);
        T* p = detail::alloc_elems<T>(n);
        std::memcpy(static_cast<void*>(p), elems.begin(), static_cast<std::size_t>(n) * sizeof(T));
        return slice<T>(p, n, n);
    }
}

// sub(s, lo, hi): s[lo:hi], a slice of the same array, with Go's checks (0 <= lo <= hi <=
// cap(s)). sub(s, lo) is s[lo:]; s[:hi] is sub(s, 0, hi). A result with capacity zero keeps
// s's data pointer, so it never points past the array, and a slice of nil stays nil
// (golang.org/issue/8404).
template <typename T, go_int L, go_int H>
slice<T> sub(slice<T> s, L lo, H hi) {
    check_slice(lo, hi, s.gp_cap(), bounds::slice_acap);
    auto l = static_cast<std::int64_t>(lo);
    auto h = static_cast<std::int64_t>(hi);
    std::int64_t c = s.gp_cap() - l;
    T* d = (c == 0 || detail::no_data<T>) ? s.gp_data() : s.gp_data() + l;
    return slice<T>(d, h - l, c);
}

template <typename T, go_int L>
slice<T> sub(slice<T> s, L lo) {
    std::int64_t hi = s.gp_len();
    return sub(std::move(s), lo, hi);
}

// sub(s, lo, hi, max): s[lo:hi:max], with 0 <= lo <= hi <= max <= cap(s). The result has
// capacity max - lo, and the same no-past-the-end rule for a zero capacity.
template <typename T, go_int L, go_int H, go_int M>
slice<T> sub(slice<T> s, L lo, H hi, M max) {
    check_slice3(lo, hi, max, s.gp_cap(), bounds::slice3_acap);
    auto l = static_cast<std::int64_t>(lo);
    auto h = static_cast<std::int64_t>(hi);
    std::int64_t c = static_cast<std::int64_t>(max) - l;
    T* d = (c == 0 || detail::no_data<T>) ? s.gp_data() : s.gp_data() + l;
    return slice<T>(d, h - l, c);
}

// append(s, a, b, ...): Go's append with single elements. The arguments are copied to values
// before anything is stored, as Go evaluates them, so append(s[:1], s[2], s[1]) stores the old
// values. append(s) is s.
template <typename T, typename... A>
slice<T> append(slice<T> s, A&&... xs) {
    constexpr std::int64_t n = sizeof...(A);
    if constexpr (n == 0) {
        return s;
    } else {
        constexpr std::uint64_t size = go_layout<T>().size;
        static_assert(size == 0 || sizeof(T) == size,
                      "a GC-engine element type must have Go's size (golayout.hpp)");
        const std::int64_t old_len = s.gp_len();
        const std::int64_t new_len =
            static_cast<std::int64_t>(static_cast<std::uint64_t>(old_len) + n);
        if (new_len < 0) [[unlikely]] {
            panic_growslice();
        }
        if (new_len <= s.gp_cap()) {
            if constexpr (size != 0) {
                T* d = s.gp_data() + old_len;
                if constexpr (n == 1) {
                    ((*d = std::forward<A>(xs)), ...);
                } else {
                    T vals[n] = {T(std::forward<A>(xs))...};
                    for (std::int64_t i = 0; i < n; i++) {
                        d[i] = vals[i];
                    }
                }
            }
            s.gp_reslice(s.gp_data(), new_len, s.gp_cap());
            return s;
        }
        slice<T> g = detail::grow(s, new_len);
        if constexpr (size != 0) {
            T* d = g.gp_data() + old_len;
            if constexpr (n == 1) {
                ((*d = std::forward<A>(xs)), ...);
            } else {
                T vals[n] = {T(std::forward<A>(xs))...};
                for (std::int64_t i = 0; i < n; i++) {
                    d[i] = vals[i];
                }
            }
        }
        g.gp_reslice(g.gp_data(), new_len, g.gp_cap());
        return g;
    }
}

// append_slice(s, t): append(s, t...). t may share s's array (s = append(s[:1], s[2:]...) and
// s = append(s, s...) work): the copy is a memmove, and a grown array reads t's old elements
// before anything overwrites them.
template <typename T>
slice<T> append_slice(slice<T> s, const slice<T>& t) {
    constexpr std::uint64_t size = go_layout<T>().size;
    static_assert(size == 0 || sizeof(T) == size,
                  "a GC-engine element type must have Go's size (golayout.hpp)");
    const std::int64_t n = t.gp_len();
    if (n == 0) {
        return s;
    }
    const std::int64_t old_len = s.gp_len();
    const std::int64_t new_len = static_cast<std::int64_t>(static_cast<std::uint64_t>(old_len) + n);
    if (new_len < 0) [[unlikely]] {
        panic_growslice();
    }
    if (new_len <= s.gp_cap()) {
        if constexpr (size != 0) {
            std::memmove(static_cast<void*>(s.gp_data() + old_len),
                         static_cast<const void*>(t.gp_data()),
                         static_cast<std::size_t>(n) * sizeof(T));
        }
        s.gp_reslice(s.gp_data(), new_len, s.gp_cap());
        return s;
    }
    slice<T> g = detail::grow(s, new_len);
    if constexpr (size != 0) {
        std::memmove(static_cast<void*>(g.gp_data() + old_len),
                     static_cast<const void*>(t.gp_data()),
                     static_cast<std::size_t>(n) * sizeof(T));
    }
    g.gp_reslice(g.gp_data(), new_len, g.gp_cap());
    return g;
}

// copy(dst, src): Go's copy, the number of elements copied. The two may overlap; the result is
// as if src were read first.
template <typename T>
std::int64_t copy(slice<T> dst, const slice<T>& src) {
    const std::int64_t n = dst.gp_len() < src.gp_len() ? dst.gp_len() : src.gp_len();
    if (n <= 0) {
        return 0;
    }
    if constexpr (!detail::no_data<T>) {
        std::memmove(static_cast<void*>(dst.gp_data()), static_cast<const void*>(src.gp_data()),
                     static_cast<std::size_t>(n) * sizeof(T));
    }
    return n;
}

// append_string(s, t): append(s, t...), Go's special append of a string's bytes to a []byte.
inline slice<std::uint8_t> append_string(slice<std::uint8_t> s, string t) {
    const std::int64_t n = t.gp_len();
    if (n == 0) {
        return s;
    }
    const std::int64_t old_len = s.gp_len();
    const std::int64_t new_len = static_cast<std::int64_t>(static_cast<std::uint64_t>(old_len) + n);
    if (new_len < 0) [[unlikely]] {
        panic_growslice();
    }
    slice<std::uint8_t> g = new_len <= s.gp_cap() ? s : detail::grow(s, new_len);
    std::memmove(static_cast<void*>(g.gp_data() + old_len), static_cast<const void*>(t.gp_data()),
                 static_cast<std::size_t>(n));
    g.gp_reslice(g.gp_data(), new_len, g.gp_cap());
    return g;
}

// to_string(b): string(b), the bytes of a []byte copied into a new pointer-free GC allocation.
inline string to_string(slice<std::uint8_t> b) {
    if (b.gp_len() == 0) {
        return string();
    }
    auto* p =
        static_cast<char*>(process_heap().alloc_atomic(static_cast<std::size_t>(b.gp_len()) + 1));
    std::memcpy(p, b.gp_data(), static_cast<std::size_t>(b.gp_len()));
    p[b.gp_len()] = '\0'; // not part of the string (gc/string.hpp)
    return string(bytes, p, b.gp_len());
}

// to_bytes(s): []byte(s), a new pointer-free GC allocation with a copy of s's bytes. Go's
// conversion copies, so writes to the slice do not change the string.
inline slice<std::uint8_t> to_bytes(string s) {
    if (s.gp_len() == 0) {
        return make_slice<std::uint8_t>(0);
    }
    auto* p = static_cast<std::uint8_t*>(
        process_heap().alloc_atomic(static_cast<std::size_t>(s.gp_len())));
    std::memcpy(p, s.gp_data(), static_cast<std::size_t>(s.gp_len()));
    return slice<std::uint8_t>(p, s.gp_len(), s.gp_len());
}

// copy(dst, src): copy([]byte, string), Go's copy of a string's bytes into a byte slice.
inline std::int64_t copy(slice<std::uint8_t> dst, string src) {
    const std::int64_t n = dst.gp_len() < src.gp_len() ? dst.gp_len() : src.gp_len();
    if (n <= 0) {
        return 0;
    }
    std::memmove(static_cast<void*>(dst.gp_data()), static_cast<const void*>(src.gp_data()),
                 static_cast<std::size_t>(n));
    return n;
}

// clear(s): Go 1.21's clear, which sets every element to its zero value. All GC-engine values
// are zero-valued by zero bytes (a nil string, slice, map, func, pointer, or a zero number).
template <typename T>
void clear(slice<T> s) {
    if constexpr (!detail::no_data<T>) {
        if (s.gp_len() > 0) {
            std::memset(static_cast<void*>(s.gp_data()), 0,
                        static_cast<std::size_t>(s.gp_len()) * sizeof(T));
        }
    }
}

} // namespace gp::gc
