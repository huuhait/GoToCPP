// gocpp/slice.hpp: gp::slice<T>, Go's slice type []T.
//
// A slice is a view of a run of elements in a backing array: an owner, a pointer to the first
// element, a length and a capacity (40 bytes). The backing array is a heap block owned through
// a std::shared_ptr, like every heap block (Phase 03 §2.1). Copying a slice copies the view,
// not the elements: two slices of the same array see each other's writes, as in Go.
//
//     nil        no array: data is null, len and cap are 0. The zero value. s == nil is true.
//     empty      []T{} and make([]T, 0): data points at a static empty block (Go's zerobase),
//                len and cap are 0, and s == nil is false, as in Go. No allocation.
//     elements   data points into an array of at least cap elements, owned by the owner.
//
// The elements can have any type gocpp maps a Go type to: numbers and bools, gp::string,
// gp::ptr, structs, nested slices, maps. A backing array is std::make_shared<T[]>: one
// allocation holds the count block and the elements; every element is a constructed T from
// the start (a zero value: value-initialized), and the array destroys each element exactly
// once when the last slice or pointer into it goes. For a trivial T (numbers, plain structs)
// a new array is made without initialization, and only the part that isn't copied over at
// once is zeroed, as Go does.
//
// What generated code writes (Phase 03 §2.3; the emitter is step 3.5b):
//
//     var s []T, s = nil       gp::slice<T> s;  s = nullptr
//     s == nil, s != nil       s == nullptr, s != nullptr (a slice compares only with nil)
//     []T{a, b}                gp::slice_of<T>({a, b})     len = cap = 2; {} gives an empty,
//                                                          non-nil slice (not nil)
//     []T{5: a, b}             gp::slice_keyed<T>(7, {{5, a}, {6, b}})   keyed: the rest zero
//     make([]T, n), (n, c)     gp::make_slice<T>(n), gp::make_slice<T>(n, c)
//     len(s), cap(s)           gp::len(s), gp::cap(s)
//     s[i]                     s[i]                        a T&, bounds checked
//     &s[i]                    gp::ptr<T>(s.gp_owner(), &s[i])   keeps the array alive
//     &f()[i]                  gp::elem_ptr(f(), i)        the same, for a slice that is not a
//                                                          variable (evaluated once)
//     for i, v := range s      for (auto gp_it0 = gp::range_slice(s); gp::range_next(gp_it0);)
//                              with i = gp_it0.gp_i, v = gp_it0.gp_v()
//     []byte(s), []rune(s)     gp::to_bytes(s), gp::to_runes(s)
//     string(b), string(r)     gp::to_string(b), gp::to_string(r)
//     s[lo:], s[lo:hi]         gp::sub(s, lo), gp::sub(s, lo, hi)
//     s[:hi], s[:]             gp::sub(s, 0, hi), gp::sub(s, 0)
//     s[lo:hi:max]             gp::sub(s, lo, hi, max)
//     append(s, a, b)          gp::append(s, a, b)
//     append(s, t...)          gp::append_slice(s, t)      also append([]byte, string...)
//     copy(dst, src)           gp::copy(dst, src)          also copy([]byte, string)
//     clear(s)                 gp::clear(s)
//     [N]T(s), (*[N]T)(s)      gp::to_array<N>(s), gp::to_array_ptr<N>(s) (gocpp/array.hpp)
//
// An index can have any Go integer type. The checks panic with Go's exact messages: "index out
// of range [5] with length 3" (gp::check_index), the slice expression ones in
// gocpp/bounds.hpp, and make's "makeslice: len out of range" / "makeslice: cap out of range".
//
// Moving: std::move(s) leaves s nil (no atomic operation). The emitter may pass std::move(s)
// to append or sub only when no other argument of the call reads s (C++ doesn't say in which
// order arguments are initialized): s = append(s, x) is
// s = gp::append(static_cast<gp::slice<T>&&>(s), x) (a cast: see internal/lower's moved).
//
// append and growth: when the elements fit (len + n <= cap), append writes them into the same
// array, after len, so other slices of that array see them (Go's aliasing). Otherwise it makes
// a new array, copies the elements and adds the new ones; the old array is untouched. The new
// capacity follows Go 1.25's runtime/slice.go: nextslicecap (double the capacity below 256
// elements, then grow by about 1.25x plus 192 elements), then rounded up to Go's malloc size
// classes (up to 32 KiB) or to 8 KiB pages. The numbers are Go's exactly, for every element
// type: the size is Go's size of the element (a gp::string is 32 bytes here, Go's string 16:
// gocpp/golayout.hpp computes Go's size and pointer-ness from the C++ type), and an element
// that holds a pointer makes a block over 512 bytes carry Go's 8-byte malloc header (71 for 33
// appends of strings, not 64: Go's roundupsize with noscan = false). A zero-size element
// (struct{}, [0]int) takes no memory and gives cap == len. So appends cost amortized O(1).
// What gc decides by escape analysis (a slice that doesn't escape can start in a 32-byte stack
// buffer) is not copied: gocpp gives the numbers of a slice on the heap.
//
// Overlap: copy, append_slice and the arguments of append behave as if the source were read
// before anything is written, as in Go: s = append(s[:1], s[2:]...), copy(s[1:], s) and
// s = append(s, s...) work.
//
// Zero-size elements (a divergence, D15): Go's struct{} and [0]T take no memory, and Go points
// every slice of them at zerobase. append does the same (grow: the data pointer is zerobase,
// whatever the length, and the elements are never copied: copy_elems and clear skip an empty
// class). But in C++ an empty struct is 1 byte, so make([]struct{}, n) allocates and zeroes n
// bytes, and an n that Go allows (math.MaxInt) fails with "makeslice: len out of range".
//
// Lifetimes: a slice's owner is released through gp::detail::release (ptr.hpp), so freeing a
// long chain (a list of nodes that hold slices of pointers to the next node) never runs out
// of stack. A slice of a package-level array (step 3.6) shares gp::static_owner(), never an
// empty owner, so &s[i] has an owner too.
//
// Like every runtime type, gp::slice has only operators and gp_-prefixed members, and
// everything else is a free function in gp:: (Phase 03 §2.1). The runtime's own calls inside
// these templates are qualified (gp::sub, detail::grow): unqualified, argument-dependent
// lookup would also search the namespace of T, and a Go function named grow or sub in the
// user's package could be called instead. It accepts an incomplete T
// (type Node struct{ kids []Node }); the functions that make or change elements need a
// complete one.
#pragma once

#include <gocpp/bounds.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/golayout.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <initializer_list>
#include <memory>
#include <span>
#include <type_traits>
#include <utility>

namespace gp {

template <typename T>
class slice;

namespace detail {

// zerobase: what the data pointer of an empty, non-nil slice points at (Go's
// runtime.zerobase). Never read or written: such a slice has no elements.
alignas(64) inline constexpr unsigned char zerobase[64] = {};

template <typename T>
T* empty_data() noexcept {
    // Non-const only for the type of the data pointer: nothing is ever stored through it.
    return reinterpret_cast<T*>(const_cast<unsigned char*>(zerobase));
}

// max_alloc: Go's maxAlloc on 64-bit Linux and Windows (1 << 48 bytes). A make or an append
// that needs a bigger array panics like Go.
inline constexpr int64_t max_alloc = int64_t(1) << 48;

// zero_bits<T>: a T whose zero value is all-zero bytes and that is copied as bytes: numbers,
// bools, plain structs of them. For other types the elements are constructed and assigned.
template <typename T>
inline constexpr bool zero_bits =
    std::is_trivially_default_constructible_v<T> && std::is_trivially_copyable_v<T>;

// In runtime/src/slice.cpp.
//
// grow_cap: the capacity of the new array when a slice of capacity old_cap must hold new_len
// elements, as Go's growslice gives it: nextslicecap, rounded up to Go's size classes. size is
// the size of an element in GO (golayout.hpp: a string is 16 bytes), noscan tells whether the
// elements hold no pointer (a block with pointers over 512 bytes has a malloc header that takes
// 8 bytes of the size class). A zero-size element gives new_len. Panics with "growslice: len
// out of range" when new_len is negative (the length overflowed) or the array would be bigger
// than max_alloc.
int64_t grow_cap(int64_t new_len, int64_t old_cap, uint64_t size, bool noscan);

// roundup_size: Go's roundupsize(size, noscan) (runtime/msize.go): the usable size of the block
// that Go's malloc gives for a request of size bytes (see grow_cap). 0 gives 0.
uint64_t roundup_size(uint64_t size, bool noscan);

// panic_make: make's panic, "makeslice: len out of range" or "makeslice: cap out of range",
// chosen as Go's makeslice does.
[[noreturn]] void panic_make(int64_t len, int64_t cap, std::size_t size);

// new_array: a new backing array of cap > 0 elements. The elements from index zero_from on are
// zero values; the ones before it are for the caller to set (for a zero_bits T they are not
// initialized).
template <typename T>
std::shared_ptr<T[]> new_array(int64_t cap, int64_t zero_from) {
    auto n = static_cast<std::size_t>(cap);
    if constexpr (zero_bits<T>) {
        std::shared_ptr<T[]> a = std::make_shared_for_overwrite<T[]>(n);
        if (zero_from < cap) {
            std::memset(static_cast<void*>(a.get() + zero_from), 0,
                        static_cast<std::size_t>(cap - zero_from) * sizeof(T));
        }
        return a;
    } else {
        return std::make_shared<T[]>(n);
    }
}

// copy_elems: assigns n elements from src to dst. The two may overlap (inside one array):
// the result is as if src were read first.
template <typename T>
void copy_elems(T* dst, const T* src, int64_t n) {
    if (n <= 0 || dst == src) {
        return;
    }
    if constexpr (std::is_empty_v<T>) {
        return; // nothing to copy (and a zero-size slice's data is zerobase, whatever its length)
    } else if constexpr (std::is_trivially_copyable_v<T>) {
        std::memmove(static_cast<void*>(dst), static_cast<const void*>(src),
                     static_cast<std::size_t>(n) * sizeof(T));
    } else if (reinterpret_cast<std::uintptr_t>(dst) < reinterpret_cast<std::uintptr_t>(src)) {
        for (int64_t i = 0; i < n; i++) {
            dst[i] = src[i];
        }
    } else {
        for (int64_t i = n - 1; i >= 0; i--) {
            dst[i] = src[i];
        }
    }
}

// may_free<T>: assigning to a T can run destructors (drop owners), and so free an array, also
// the array that the source of the assignment is in: q = append(q[:0], q[0].kids...) drops
// q[0].kids, the only owner of the elements it copies, when it overwrites q[0]. The functions
// that store elements keep the source alive for such a T (one atomic increment); a trivially
// destructible T (numbers, plain structs) owns nothing and pays nothing.
template <typename T>
inline constexpr bool may_free = !std::is_trivially_destructible_v<T>;

// zero_block<T>: the deleter of the array that a slice of a zero-size, non-empty-class T grows
// into (grow): it knows where the block is and how many elements it holds.
template <typename T>
struct zero_block {
    T* base;
    int64_t n;
    void operator()(T* p) const noexcept { delete[] p; }
};

template <typename T>
slice<T> grow(const slice<T>& s, int64_t new_len);

template <typename T>
slice<T> append_n(slice<T> s, const T* src, int64_t n);

} // namespace detail

// slice<T> is Go's []T. The zero value is nil.
template <typename T>
class slice {
public:
    // nil.
    constexpr slice() noexcept = default;
    constexpr slice(std::nullptr_t) noexcept {}

    // A slice of len elements at data, with room for cap, in a block owned by owner. For the
    // runtime's own code (make_slice, arrays in step 3.6). A non-nil slice with cap > 0 must
    // have an owner (static_owner() for a package-level array).
    slice(std::shared_ptr<void> owner, T* data, int64_t len, int64_t cap) noexcept
        : gp_owner_(std::move(owner)), gp_data_(data), gp_len_(len), gp_cap_(cap) {}

    // A copy shares the array (one atomic increment); a move leaves nil behind.
    slice(const slice&) noexcept = default;
    slice(slice&& o) noexcept
        : gp_owner_(std::move(o.gp_owner_)), gp_data_(o.gp_data_), gp_len_(o.gp_len_),
          gp_cap_(o.gp_cap_) {
        o.gp_data_ = nullptr;
        o.gp_len_ = 0;
        o.gp_cap_ = 0;
    }

    // The assignments read the new value before they drop the old array, so s = s[0].kids
    // works when s holds the only reference to the array that holds s[0].kids. The old owner
    // goes through detail::release.
    slice& operator=(const slice& o) noexcept {
        if (this != &o) {
            slice copy(o);
            *this = std::move(copy);
        }
        return *this;
    }
    slice& operator=(slice&& o) noexcept {
        if (this != &o) {
            bool had = gp_data_ != nullptr;
            std::shared_ptr<void> old = std::move(gp_owner_);
            gp_owner_ = std::move(o.gp_owner_);
            gp_data_ = o.gp_data_;
            gp_len_ = o.gp_len_;
            gp_cap_ = o.gp_cap_;
            o.gp_data_ = nullptr;
            o.gp_len_ = 0;
            o.gp_cap_ = 0;
            if (had) {
                detail::release(old);
            }
        }
        return *this;
    }

    ~slice() {
        if (gp_data_ != nullptr) {
            detail::release(gp_owner_);
        }
    }

    // s[i]: the element at index i, with Go's bounds check: an index outside [0, len(s))
    // panics with "runtime error: index out of range [5] with length 3". A const slice still
    // gives a writable element, as a slice is a reference to its array.
    template <go_int I>
    T& operator[](I i) const {
        gp::check_index(i, gp_len_);
        return gp_data_[static_cast<std::size_t>(i)];
    }

    // s == nil. Go compares slices only with nil.
    friend bool operator==(const slice& a, std::nullptr_t) noexcept {
        return a.gp_data_ == nullptr;
    }

    // A slice converts implicitly to std::span<T> and std::span<const T>, for C++ functions
    // that take one (Phase 04 §2.7). The span sees the slice's len elements (not its capacity)
    // and is valid while the array lives; a nil slice gives an empty span. Only the dynamic
    // extent: std::span<T, N> isn't a conversion.
    template <typename U>
        requires std::is_same_v<std::remove_const_t<U>, T>
    operator std::span<U>() const noexcept {
        if (gp_data_ == nullptr) {
            return std::span<U>();
        }
        return std::span<U>(gp_data_, static_cast<std::size_t>(gp_len_));
    }

    // For the runtime's own code and generated code that needs the parts: the owner of the
    // array (for &s[i]), the first element (null for nil), the length and the capacity.
    const std::shared_ptr<void>& gp_owner() const noexcept { return gp_owner_; }
    T* gp_data() const noexcept { return gp_data_; }
    int64_t gp_len() const noexcept { return gp_len_; }
    int64_t gp_cap() const noexcept { return gp_cap_; }

    // gp_reslice: the same array, viewed from data with len and cap. For the runtime's own
    // code (sub, append), which checks the bounds first.
    void gp_reslice(T* data, int64_t len, int64_t cap) noexcept {
        gp_data_ = data;
        gp_len_ = len;
        gp_cap_ = cap;
    }

private:
    std::shared_ptr<void> gp_owner_;
    T* gp_data_ = nullptr;
    int64_t gp_len_ = 0;
    int64_t gp_cap_ = 0;
};

static_assert(sizeof(slice<int64_t>) == 40,
              "gp::slice is 40 bytes: owner (16), data pointer, length, capacity (D15)");

// len(s), cap(s).
template <typename T>
int64_t len(const slice<T>& s) noexcept {
    return s.gp_len();
}

template <typename T>
int64_t cap(const slice<T>& s) noexcept {
    return s.gp_cap();
}

// make_slice<T>(n): make([]T, n). make_slice<T>(n, c): make([]T, n, c). All c elements are
// zero values. A negative or too big n panics with "runtime error: makeslice: len out of
// range", a c below n or too big with "makeslice: cap out of range" (an unsigned value that
// doesn't fit in int64 counts as negative, as in Go). make([]T, 0) is empty but not nil.
template <typename T, go_int L, go_int C>
slice<T> make_slice(L len, C cap) {
    auto n = static_cast<int64_t>(len);
    auto c = static_cast<int64_t>(cap);
    if (static_cast<uint64_t>(c) > static_cast<uint64_t>(detail::max_alloc) / sizeof(T) ||
        static_cast<uint64_t>(n) > static_cast<uint64_t>(c)) [[unlikely]] {
        detail::panic_make(n, c, sizeof(T));
    }
    if (c == 0) {
        return slice<T>(std::shared_ptr<void>(), detail::empty_data<T>(), 0, 0);
    }
    std::shared_ptr<T[]> a = detail::new_array<T>(c, 0);
    T* d = a.get();
    return slice<T>(std::shared_ptr<void>(std::move(a), d), d, n, c);
}

template <typename T, go_int L>
slice<T> make_slice(L len) {
    return gp::make_slice<T>(len, len);
}

// slice_of<T>({a, b, c}): the composite literal []T{a, b, c}, with len and cap 3. The elements
// are evaluated left to right (a braced list). slice_of<T>({}) is []T{}: empty, not nil.
template <typename T>
slice<T> slice_of(std::initializer_list<T> elems) {
    auto n = static_cast<int64_t>(elems.size());
    if (n == 0) {
        return slice<T>(std::shared_ptr<void>(), detail::empty_data<T>(), 0, 0);
    }
    std::shared_ptr<T[]> a = detail::new_array<T>(n, n);
    T* d = a.get();
    const T* src = elems.begin();
    if constexpr (detail::zero_bits<T>) {
        std::memcpy(static_cast<void*>(d), static_cast<const void*>(src),
                    static_cast<std::size_t>(n) * sizeof(T));
    } else {
        for (int64_t i = 0; i < n; i++) {
            d[i] = src[i];
        }
    }
    return slice<T>(std::shared_ptr<void>(std::move(a), d), d, n, n);
}

// sub(s, lo, hi): s[lo:hi], a slice of the same array (no copy), with Go's checks:
// 0 <= lo <= hi <= cap(s), else "slice bounds out of range [:hi] with capacity c" or
// "[lo:hi]" (bounds.hpp). sub(s, lo) is s[lo:] (hi = len(s)); s[:hi] is sub(s, 0, hi).
// Like Go, a result with capacity 0 keeps s's data pointer (so it never points past the
// array, and a slice of nil stays nil).
template <typename T, go_int L, go_int H>
slice<T> sub(slice<T> s, L lo, H hi) {
    gp::check_slice(lo, hi, s.gp_cap(), bounds::slice_acap);
    auto l = static_cast<int64_t>(lo);
    auto h = static_cast<int64_t>(hi);
    int64_t c = s.gp_cap() - l;
    s.gp_reslice(c == 0 ? s.gp_data() : s.gp_data() + l, h - l, c);
    return s;
}

template <typename T, go_int L>
slice<T> sub(slice<T> s, L lo) {
    int64_t hi = s.gp_len();
    return gp::sub(std::move(s), lo, hi);
}

// sub(s, lo, hi, max): s[lo:hi:max]: 0 <= lo <= hi <= max <= cap(s), else Go's panics
// "[::max] with capacity c", "[:hi:max]" or "[lo:hi:]". The result has capacity max - lo.
template <typename T, go_int L, go_int H, go_int M>
slice<T> sub(slice<T> s, L lo, H hi, M max) {
    gp::check_slice3(lo, hi, max, s.gp_cap(), bounds::slice3_acap);
    auto l = static_cast<int64_t>(lo);
    auto h = static_cast<int64_t>(hi);
    int64_t c = static_cast<int64_t>(max) - l;
    s.gp_reslice(c == 0 ? s.gp_data() : s.gp_data() + l, h - l, c);
    return s;
}

// append(s, a, b, ...): Go's append with single elements. The arguments are converted to T
// first, so append(s[:1], s[2], s[1]) stores the old values, as in Go, and
// append(q[:0], q[0].kids[0]) doesn't read an element that storing q[0] freed. append(s) is s.
template <typename T, typename... A>
slice<T> append(slice<T> s, A&&... xs) {
    constexpr int64_t n = sizeof...(A);
    if constexpr (n == 0) {
        return s;
    } else {
        int64_t len = s.gp_len();
        auto new_len = static_cast<int64_t>(static_cast<uint64_t>(len) + n);
        if (static_cast<uint64_t>(new_len) > static_cast<uint64_t>(s.gp_cap())) [[unlikely]] {
            // A new array. The arguments may refer to elements of the old one, which s keeps
            // alive until the end.
            slice<T> g = detail::grow(s, new_len);
            T* d = g.gp_data() + len;
            ((*d++ = std::forward<A>(xs)), ...);
            g.gp_reslice(g.gp_data(), new_len, g.gp_cap());
            return g;
        }
        T* d = s.gp_data() + len;
        if constexpr (n == 1 && !detail::may_free<T>) {
            ((*d = std::forward<A>(xs)), ...);
        } else {
            T vals[] = {T(std::forward<A>(xs))...};
            for (int64_t i = 0; i < n; i++) {
                d[i] = std::move(vals[i]);
            }
        }
        s.gp_reslice(s.gp_data(), new_len, s.gp_cap());
        return s;
    }
}

// append_slice(s, t): append(s, t...). t may share s's array (s = append(s[:1], s[2:]...)
// and s = append(s, s...) work). Nothing to append gives s itself (a nil s stays nil).
template <typename T>
slice<T> append_slice(slice<T> s, const slice<T>& t) {
    if constexpr (detail::may_free<T>) {
        slice<T> keep(t); // storing an element may drop every other owner of t's array
        return detail::append_n(std::move(s), keep.gp_data(), keep.gp_len());
    } else {
        return detail::append_n(std::move(s), t.gp_data(), t.gp_len());
    }
}

// append_slice(b, str): append(b, str...), the bytes of a string appended to a []byte.
inline slice<uint8_t> append_slice(slice<uint8_t> s, const string& str) {
    return detail::append_n(std::move(s), reinterpret_cast<const uint8_t*>(str.gp_data()),
                            gp::len(str));
}

// copy(dst, src): copies min(len(dst), len(src)) elements and returns that number. The two
// may overlap.
template <typename T>
int64_t copy(const slice<T>& dst, const slice<T>& src) {
    int64_t n = dst.gp_len() < src.gp_len() ? dst.gp_len() : src.gp_len();
    if constexpr (detail::may_free<T>) {
        // Storing an element may drop every other owner of either array (copy(q, q[0].kids)).
        slice<T> keep_dst(dst), keep_src(src);
        detail::copy_elems(keep_dst.gp_data(), static_cast<const T*>(keep_src.gp_data()), n);
    } else {
        detail::copy_elems(dst.gp_data(), static_cast<const T*>(src.gp_data()), n);
    }
    return n;
}

// copy(dst, str): copy(dst []byte, src string), the bytes of a string.
inline int64_t copy(const slice<uint8_t>& dst, const string& src) {
    int64_t n = dst.gp_len() < gp::len(src) ? dst.gp_len() : gp::len(src);
    detail::copy_elems(dst.gp_data(), reinterpret_cast<const uint8_t*>(src.gp_data()), n);
    return n;
}

// clear(s): sets the elements s[0:len(s)] to their zero value (Go 1.21).
template <typename T>
void clear(const slice<T>& s) {
    int64_t n = s.gp_len();
    if (n == 0) {
        return;
    }
    T* d = s.gp_data();
    if constexpr (std::is_empty_v<T>) {
        return; // no data (see copy_elems)
    } else if constexpr (detail::zero_bits<T>) {
        std::memset(static_cast<void*>(d), 0, static_cast<std::size_t>(n) * sizeof(T));
    } else if constexpr (detail::may_free<T>) {
        slice<T> keep(s); // zeroing an element may drop every other owner of the array
        for (int64_t i = 0; i < n; i++) {
            d[i] = T();
        }
    } else {
        for (int64_t i = 0; i < n; i++) {
            d[i] = T();
        }
    }
}

// --- keyed literals, element pointers, range, conversions (step 3.5b) ---

// slice_keyed<T>(n, {{k, v}, ...}): the keyed literal []T{k: v, ...} of length (and capacity)
// n, one more than its largest index. The elements are evaluated in source order (a braced
// list); the ones that no key names are zero values. go/types checked the keys (constants,
// none twice), so n >= 1; a key outside [0, n) would be a bug in the generated code and panics
// like an index.
template <typename T>
slice<T> slice_keyed(int64_t n, std::initializer_list<std::pair<int64_t, T>> elems) {
    if (n <= 0) {
        return slice<T>(std::shared_ptr<void>(), detail::empty_data<T>(), 0, 0);
    }
    std::shared_ptr<T[]> a = detail::new_array<T>(n, 0);
    T* d = a.get();
    for (const std::pair<int64_t, T>& e : elems) {
        gp::check_index(e.first, n);
        d[e.first] = e.second;
    }
    return slice<T>(std::shared_ptr<void>(std::move(a), d), d, n, n);
}

// elem_ptr(s, i): &s[i] for a slice s that is not a variable (&f()[i], &ss[j][i]): a pointer
// to the element that shares the slice's array (s is evaluated once). For a variable s,
// generated code writes gp::ptr<T>(s.gp_owner(), &s[i]) itself.
template <typename T, go_int I>
ptr<T> elem_ptr(const slice<T>& s, I i) {
    T& e = s[i];
    return ptr<T>(s.gp_owner(), &e);
}

// slice_range is the state of `for i, v := range s`: a copy of s, taken once when the loop
// starts (Go evaluates the range expression once, so appending to s or assigning s in the body
// doesn't change the iteration: it covers the original length and array), and the hidden
// counter of Go 1.22 (assigning to i in the body doesn't change the iteration). After each
// successful range_next, gp_i is the index and gp_v() the element there, read when it is
// called: a write to a later element in the body is seen, as in Go, since the copy shares the
// array.
template <typename T>
struct slice_range {
    slice<T> gp_s;
    int64_t gp_i = -1;

    T& gp_v() const noexcept { return gp_s.gp_data()[gp_i]; }
};

// range_slice(s) starts a range over s.
template <typename T>
slice_range<T> range_slice(slice<T> s) noexcept {
    return slice_range<T>{std::move(s)};
}

// range_next steps to the next index of the range, and reports whether there is one.
template <typename T>
bool range_next(slice_range<T>& it) noexcept {
    return ++it.gp_i < it.gp_s.gp_len();
}

// The conversions between strings and slices; each copies, as in Go (runtime/src/slice.cpp).
//
//     []byte(s)       to_bytes(s)      the bytes; []byte("") is empty, not nil
//     []rune(s)       to_runes(s)      the runes, decoded like range over s: an invalid byte
//                                      is U+FFFD; []rune("") is empty, not nil
//     string(b)       to_string(b)     the bytes of a []byte
//     string(runes)   to_string(r)     the UTF-8 of each rune; a value that is not a code
//                                      point (negative, a surrogate half, above U+10FFFF)
//                                      is U+FFFD, as in string(r)
//
// Go's capacity of []byte(s) and []rune(s) depends on escape analysis: a slice that doesn't
// escape may share the string's bytes (cap == len) or use a 32-byte stack buffer. gocpp gives
// what a slice that escapes gets (rawbyteslice, rawruneslice): the size that Go's malloc rounds
// the length up to (8, 16, 24, 32, 48, ... bytes; runes: that divided by 4). A constant string
// (to_bytes_const, to_runes_const, which the emitter writes) always gets cap == len, in gc too.
slice<uint8_t> to_bytes(const string& s);
slice<uint8_t> to_bytes_const(const string& s);
slice<int32_t> to_runes(const string& s);
slice<int32_t> to_runes_const(const string& s);
string to_string(const slice<uint8_t>& b);
string to_string(const slice<int32_t>& r);

// retype_slice<E>(s): the slice s with its elements seen as E, a named type with the same
// layout (type Uint8 uint8: []Uint8("abc") is retype_slice<Uint8>(to_bytes("abc")), and
// string(u8s) is to_string(retype_slice<uint8_t>(u8s))). Same array, owner, length and capacity.
template <typename E, typename U>
slice<E> retype_slice(const slice<U>& s) {
    static_assert(sizeof(E) == sizeof(U) && alignof(E) == alignof(U));
    return slice<E>(s.gp_owner(), reinterpret_cast<E*>(s.gp_data()), s.gp_len(), s.gp_cap());
}

namespace detail {

// grow: a new array with room for new_len elements (and maybe more), holding a copy of s's
// elements; the result has s's length. The elements from new_len on are zero values; the
// ones in [len(s), new_len) are for the caller to set.
template <typename T>
slice<T> grow(const slice<T>& s, int64_t new_len) {
    constexpr detail::go_layout_t lay = detail::go_layout<T>();
    int64_t c = detail::grow_cap(new_len, s.gp_cap(), lay.size, !lay.ptrs);
    int64_t len = s.gp_len();
    if constexpr (lay.size == 0) {
        // Go's growslice for a zero-size element takes no memory: cap == len, every time. The
        // elements have no data, so nothing is copied.
        if constexpr (std::is_empty_v<T>) {
            // An empty class is never read or written (copy_elems and clear skip it): the new
            // slice points at zerobase, whatever its length.
            return slice<T>(gp::static_owner(), detail::empty_data<T>(), len, c);
        } else {
            // A class that is empty in Go but not in C++ (struct{ z [0]int }: 1 byte) is stored
            // to (clang copies the byte), so its elements need real memory. The block is
            // bigger than the capacity, and its owner remembers how big: the next append that
            // fits reuses it, so growing by one element at a time costs O(1) each, not O(n).
            if (auto* h = std::get_deleter<zero_block<T>>(s.gp_owner())) {
                auto at = static_cast<uintptr_t>(reinterpret_cast<uintptr_t>(s.gp_data()) -
                                                 reinterpret_cast<uintptr_t>(h->base));
                if (s.gp_data() != nullptr && at % sizeof(T) == 0 &&
                    at / sizeof(T) + static_cast<uint64_t>(new_len) <=
                        static_cast<uint64_t>(h->n)) {
                    return slice<T>(s.gp_owner(), s.gp_data(), len, c);
                }
            }
            int64_t n = new_len;
            if (n < (int64_t(1) << 40)) {
                n = n < 16 ? 16 : n * 2;
            }
            T* base = new T[static_cast<std::size_t>(n)]();
            std::shared_ptr<T[]> a(base, zero_block<T>{base, n});
            return slice<T>(std::shared_ptr<void>(std::move(a), base), base, len, c);
        }
    }
    std::shared_ptr<T[]> a = detail::new_array<T>(c, new_len);
    T* d = a.get();
    if (len > 0) {
        if constexpr (zero_bits<T>) {
            std::memcpy(static_cast<void*>(d), static_cast<const void*>(s.gp_data()),
                        static_cast<std::size_t>(len) * sizeof(T));
        } else {
            const T* src = s.gp_data();
            for (int64_t i = 0; i < len; i++) {
                d[i] = src[i];
            }
        }
    }
    return slice<T>(std::shared_ptr<void>(std::move(a), d), d, len, c);
}

// append_n: appends the n elements at src (which may be inside s's array). The caller keeps
// src's array alive (append_slice: may_free).
template <typename T>
slice<T> append_n(slice<T> s, const T* src, int64_t n) {
    if (n == 0) {
        return s;
    }
    int64_t len = s.gp_len();
    auto new_len = static_cast<int64_t>(static_cast<uint64_t>(len) + static_cast<uint64_t>(n));
    if (static_cast<uint64_t>(new_len) > static_cast<uint64_t>(s.gp_cap())) {
        // src stays alive: its slice is the caller's.
        slice<T> g = detail::grow(s, new_len);
        detail::copy_elems(g.gp_data() + len, src, n);
        g.gp_reslice(g.gp_data(), new_len, g.gp_cap());
        return g;
    }
    detail::copy_elems(s.gp_data() + len, src, n);
    s.gp_reslice(s.gp_data(), new_len, s.gp_cap());
    return s;
}

} // namespace detail

// slice_elem_of<S>::type is E for S = slice<E>: the element type of a slice type that C++ deduced
// (a generic function over S ~[]E whose E the compiler cannot deduce from the arguments).
namespace detail {
template <typename S>
struct slice_elem_of;
template <typename E>
struct slice_elem_of<slice<E>> {
    using type = E;
};
} // namespace detail

} // namespace gp
