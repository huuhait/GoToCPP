// gocpp/interop.hpp: the helpers of generated code that uses C and C++ values (Phase 04 §2.5,
// §2.6, §2.12): gp::deref, gp::range, gp::range_keys, gp::values, gp::cstr, and the cgo
// helpers in gp::cgo (C.CString, C.GoString, C.GoStringN, C.GoBytes, C.free).
//
// The emitter writes (Phase 04 steps 4.4, 4.7 and 4.9):
//
//     gocpp                       C++
//     x.m, x.m() (x opaque)       gp::deref(x).m, gp::deref(x).m()
//     for i, v := range c         for (auto&& [i, v] : gp::range(c))
//     for _, v := range c         for ([[maybe_unused]] auto&& [gp_k, v] : gp::range(c))
//     for i := range c            for (auto&& i : gp::range_keys(c))
//     for range c                 for ([[maybe_unused]] auto&& gp_k : gp::range_keys(c))
//     i, v = range c              for (auto&& [gp_k, gp_v] : gp::range(c)) {
//       (assigning)                   i = gp_k; v = gp_v; ...
//     a, b := std.minmax(x, y)    auto [a, b] = gp::values(std::minmax(x, y));
//     C.cstr(s)                   gp::cstr(s)
//     C.CString(s)                gp::cgo::CString(s)
//     C.GoString(p)               gp::cgo::GoString(p)
//     C.GoStringN(p, n)           gp::cgo::GoStringN(p, n)
//     C.GoBytes(p, n)             gp::cgo::GoBytes(p, n)
//     C.free(p)                   gp::cgo::free(p)
//     int(x), C.int(d) (x, d C/C++ gp::cgo::as<int64_t>(x), gp::cgo::as<int>(d)
//       values or gocpp numbers)
//
// The conversions at the boundary are members of the runtime types themselves: gp::string to
// std::string_view (string.hpp), gp::slice<T> to std::span<T> (slice.hpp) and gp::ptr<T> to
// T* (ptr.hpp).
#pragma once

#include <gocpp/arith.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <iterator>
#include <limits>
#include <string>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {

// gp_pointer<X>: X is a gocpp pointer, gp::ptr<T>, or a named type over one (gp::newtype
// derives from it).
template <typename T>
void accept_gp_ptr(const ptr<T>&);
template <typename X>
concept gp_pointer = requires(const X& x) { detail::accept_gp_ptr(x); };

// raw_object_pointer<X>: X is a raw pointer to an object (not void, not a function).
template <typename X>
concept raw_object_pointer = std::is_pointer_v<X> && !std::is_void_v<std::remove_pointer_t<X>> &&
                             !std::is_function_v<std::remove_pointer_t<X>>;

} // namespace detail

// --- deref: Go's automatic dereference of x.m ---

// deref(x) is the object that x.m names a member of, for a C/C++ value x (opaque mode): Go
// dereferences a pointer one level for a selector, so a raw pointer gives *x (a gocpp *T of a
// C++ type T is a raw T*), and so does a gocpp pointer gp::ptr<T> that comes back as an opaque
// value: an element of a std.vector[*Node] (std::vector<gp::ptr<Node>>), or a gocpp pointer
// that a C++ template returns. Without it gp::deref(v[0]).Name wouldn't compile, and the
// gp::ptr has no plain-named members, so p.m can only mean (*p).m. A nil pointer of either
// kind panics as Go does ("invalid memory address or nil pointer dereference"), unless
// GOCPP_NOCHECKS is defined (then it is undefined behavior). Everything else passes through as
// a reference, without a copy: a value, a reference, a temporary (valid until the end of the
// full expression), and C++ smart pointers and iterators, whose own members (sp.reset(),
// sp.get()) stay reachable: (*sp).m is the explicit form.
template <typename X>
constexpr decltype(auto) deref(X&& x) {
    using V = std::remove_cvref_t<X>;
    if constexpr (detail::raw_object_pointer<V>) {
#ifndef GOCPP_NOCHECKS
        if (x == nullptr) [[unlikely]] {
            gp::panic_nil();
        }
#endif
        return *x;
    } else if constexpr (detail::gp_pointer<V>) {
        return *x; // the nil check is ptr's
    } else {
        return std::forward<X>(x);
    }
}

// --- range: for ... range over a C++ container ---

namespace detail {

// map_like<C>: a C++ map (std::map, std::unordered_map, their multi forms): a range over it
// gives the keys and the mapped values, as a range over a Go map does.
template <typename C>
concept map_like = requires {
    typename C::key_type;
    typename C::mapped_type;
};

template <typename C>
using range_iter_t = decltype(std::ranges::begin(std::declval<C&>()));

// by_index<C>: the range walks C by index, with the length read once, as Go reads the length
// of a slice once: C has random-access iterators and a size (std::vector, std::deque,
// std::array, std::string, std::span, C arrays). An iterator walk would break when the body
// grows the container (a push_back that reallocates); an index stays valid.
template <typename C>
concept by_index = !map_like<C> && std::random_access_iterator<range_iter_t<C>> &&
                   requires(C& c) { std::ranges::size(c); };

// range_view is what gp::range and gp::range_keys return. C is the container's type as it was
// passed: an lvalue reference (C& or const C&) for a named container, which the view refers
// to, or a plain type for a temporary, which the view holds by value (moved in). The loop's
// range-init then keeps it alive: under C++20 a reference to a temporary inside it would
// dangle. Keys selects the one-variable form: only the index (the key for maps).
//
// Each step gives a new value: std::pair<int64_t, V> (index, copy of the element) or
// std::pair<K, M> (copy of the key, copy of the mapped value), and int64_t or K for
// range_keys. So the loop variables are copies, as in Go: changing v in the body doesn't
// change the container. A C++ element that can't be copied (std::unique_ptr) doesn't compile
// with gp::range; range_keys and c[i] work.
//
// A walk by index reads the size once, at the start. If the body shrinks the container, an
// index past its new end panics with Go's "index out of range [i] with length n" (unless
// GOCPP_NOCHECKS), instead of reading a destroyed element. An iterator walk (lists, sets, maps)
// follows C++'s rules: a change in the body that invalidates the current iterator (erasing the
// current element of a std::map, an insert that rehashes a std::unordered_map) is undefined
// behavior, as it is in C++.
template <typename C, bool Keys>
class range_view {
    using container = std::remove_reference_t<C>;
    using iter = range_iter_t<container>;
    using value = std::iter_value_t<iter>;

    // Go's range over [2][3]int copies each [3]int into v, but a C array can't be copied into
    // a std::pair; range_keys and grid[i] work.
    static_assert(Keys || !std::is_array_v<value>,
                  "gp::range: the elements are C arrays, which can't be copied into the loop "
                  "variable; use gp::range_keys(c) and c[i]");

public:
    explicit range_view(C&& c) : gp_c_(std::forward<C>(c)) {}

    // The end of the loop, for both walks: the cursor knows where it ends.
    struct sentinel {};

    // A walk by index: the position, and the length read at the start.
    struct index_cursor {
        range_view* gp_v;
        int64_t gp_i;
        int64_t gp_n;

        auto operator*() const {
            if constexpr (Keys) {
                return gp_i; // the element isn't read
            } else {
                container& c = gp_v->gp_c_;
                if constexpr (!std::is_array_v<container>) {
                    gp::check_index(gp_i, static_cast<int64_t>(std::ranges::size(c)));
                }
                return std::pair<int64_t, value>(
                    gp_i, std::ranges::begin(c)[static_cast<std::iter_difference_t<iter>>(gp_i)]);
            }
        }
        index_cursor& operator++() noexcept {
            ++gp_i;
            return *this;
        }
        friend bool operator==(const index_cursor& a, sentinel) noexcept {
            return a.gp_i >= a.gp_n;
        }
    };

    // An iterator walk: the C++ iterator, the end read at the start, and the index.
    struct iter_cursor {
        iter gp_it;
        decltype(std::ranges::end(std::declval<container&>())) gp_end;
        int64_t gp_i;

        auto operator*() const {
            if constexpr (map_like<container>) {
                using key = std::remove_cv_t<typename container::key_type>;
                if constexpr (Keys) {
                    return key(gp_it->first);
                } else {
                    return std::pair<key, typename container::mapped_type>(gp_it->first,
                                                                           gp_it->second);
                }
            } else if constexpr (Keys) {
                return gp_i;
            } else {
                return std::pair<int64_t, value>(gp_i, *gp_it);
            }
        }
        iter_cursor& operator++() {
            ++gp_it;
            ++gp_i;
            return *this;
        }
        friend bool operator==(const iter_cursor& a, sentinel) { return a.gp_it == a.gp_end; }
    };

    // The view is a C++ range through hidden friends (found by argument-dependent lookup), so
    // it has no plain-named members, like every runtime type. The loop calls begin and end
    // once, before the first step.
    friend auto begin(range_view& v) {
        if constexpr (by_index<container>) {
            // In int64_t, Go's int: a C++ size beyond it can't be walked anyway.
            return index_cursor{&v, 0, static_cast<int64_t>(std::ranges::size(v.gp_c_))};
        } else {
            return iter_cursor{std::ranges::begin(v.gp_c_), std::ranges::end(v.gp_c_), 0};
        }
    }
    friend sentinel end(range_view&) noexcept { return {}; }

private:
    C gp_c_;
};

} // namespace detail

// range(c) is `for i, v := range c` over a C++ container or a C array: each step gives the
// index and a copy of the element (for a map: a copy of the key and of the mapped value).
// The emitter writes `for (auto&& [i, v] : gp::range(c))`; see detail::range_view for what is
// copied, held and checked. A temporary container (gp::range(f())) is held inside the view,
// so it lives as long as the loop.
template <typename C>
auto range(C&& c) {
    static_assert(!(std::is_array_v<std::remove_reference_t<C>> && !std::is_lvalue_reference_v<C>),
                  "gp::range: a temporary C array can't be walked");
    return detail::range_view<C, false>(std::forward<C>(c));
}

// range_keys(c) is `for i := range c` (and `for range c`): only the index, or the key for a
// map. No element is copied.
template <typename C>
auto range_keys(C&& c) {
    static_assert(!(std::is_array_v<std::remove_reference_t<C>> && !std::is_lvalue_reference_v<C>),
                  "gp::range_keys: a temporary C array can't be walked");
    return detail::range_view<C, true>(std::forward<C>(c));
}

// --- values: multiple results from C++ ---

namespace detail {

template <typename T>
inline constexpr bool is_pair = false;
template <typename A, typename B>
inline constexpr bool is_pair<std::pair<A, B>> = true;

template <typename T>
inline constexpr bool is_tuple = false;
template <typename... Ts>
inline constexpr bool is_tuple<std::tuple<Ts...>> = true;

// value_member is member I of the pair or tuple t, for a copy or a move: a reference member
// (lvalue or rvalue reference) as an lvalue, so it is copied and what it refers to is left
// alone; a value member forwarded, so it is moved from a temporary.
template <std::size_t I, typename T>
decltype(auto) value_member(T&& t) {
    if constexpr (std::is_reference_v<std::tuple_element_t<I, std::remove_cvref_t<T>>>) {
        return std::get<I>(t); // t is a name here: an lvalue, so get gives an lvalue
    } else {
        return std::get<I>(std::forward<T>(t));
    }
}

template <template <typename...> class R, typename T, std::size_t... I>
auto values_of(T&& t, std::index_sequence<I...>) {
    using V = std::remove_cvref_t<T>;
    return R<std::remove_cvref_t<std::tuple_element_t<I, V>>...>(
        detail::value_member<I>(std::forward<T>(t))...);
}

} // namespace detail

// values(t) is a C++ function's pair or tuple with every member a value: a reference member
// (std::minmax gives std::pair<const int&, const int&>, std::tie a tuple of references)
// becomes a copy of what it refers to, and const is dropped (Go's variables can be assigned).
// So `a, b := std.minmax(x, y)` is `auto [a, b] = gp::values(std::minmax(x, y));`: with
// temporary arguments the references would dangle after the statement, and with variables
// they would alias them. A reference member is always copied, also an rvalue reference
// (std::forward_as_tuple(std::move(x)) leaves x alone); members that are values are moved from
// a temporary. Anything else is returned as a copy (or moved).
template <typename T>
auto values(T&& t) {
    using V = std::remove_cvref_t<T>;
    if constexpr (detail::is_pair<V>) {
        return detail::values_of<std::pair>(std::forward<T>(t), std::make_index_sequence<2>{});
    } else if constexpr (detail::is_tuple<V>) {
        return detail::values_of<std::tuple>(std::forward<T>(t),
                                             std::make_index_sequence<std::tuple_size_v<V>>{});
    } else {
        return V(std::forward<T>(t));
    }
}

// --- cstr: a gocpp string as a C string ---

// cstr_holder is what gp::cstr returns: a NUL-terminated copy of a gocpp string's bytes, or
// the bytes themselves when a NUL already follows them (a literal: no copy). It converts
// implicitly to const char*, which is valid as long as the holder lives: until the end of the
// statement for gp::cstr(s) in a call, as long as the variable for `name := C.cstr(s)`. Copies
// and moves keep it valid (each holder owns its own bytes or points at static ones).
//
// A holder passed to a variadic C function (printf, curl_easy_setopt, ioctl, execl) doesn't
// convert: C++ passes a class through `...` as it is, so the callee would read the address of
// a copy of the holder as its char*. clang rejects that (-Wnon-pod-varargs is an error), and
// g++ only accepts it as "conditionally supported", which gocpp's flags for generated code
// make an error (-Werror=conditionally-supported, build.GeneratedFlags). The emitter writes the
// explicit form there: gp::cstr(s).gp_ptr() (Phase 04 step 4.9).
class cstr_holder {
public:
    // The bytes of s (all of them, also a NUL inside them, which ends the C string early, as
    // with cgo's C.CString).
    explicit cstr_holder(const string& s) {
        if (s.gp_c_terminated()) {
            gp_p_ = s.gp_data();
        } else if (gp::len(s) == 0) {
            gp_p_ = "";
        } else {
            gp_buf_.assign(s.gp_data(), static_cast<std::size_t>(gp::len(s)));
            gp_own_ = true;
        }
    }

    // A char array: a C++ string literal (the emitter writes an untyped constant as one),
    // or a C struct's char name[16]. No copy when a NUL is inside it, else its N bytes and a
    // NUL. Without a copy the holder points into the array, so an array that isn't a literal
    // must outlive the holder (C++'s lifetime rules for C++ values).
    template <std::size_t N>
    explicit cstr_holder(const char (&a)[N]) {
        if (std::char_traits<char>::find(a, N, '\0') != nullptr) {
            gp_p_ = a;
        } else {
            gp_buf_.assign(a, N);
            gp_own_ = true;
        }
    }

    operator const char*() const noexcept { return gp_ptr(); }
    const char* gp_ptr() const noexcept { return gp_own_ ? gp_buf_.c_str() : gp_p_; }

private:
    const char* gp_p_ = nullptr; // the borrowed C string, when !gp_own_
    std::string gp_buf_;         // the copy, when gp_own_
    bool gp_own_ = false;
};

// cstr(s) is C.cstr(s): s as a const char*, through a holder that lives as long as the
// statement or the variable that holds it (cstr_holder). It takes a gocpp string (also of a
// named string type), or a C++ string literal or char array. (A char pointer is a C string
// already: there is no form for it.)
inline cstr_holder cstr(const string& s) {
    return cstr_holder(s);
}
template <std::size_t N>
cstr_holder cstr(const char (&a)[N]) {
    return cstr_holder(a);
}

// --- cgo's helpers ---

namespace detail {

// c_char_pointer<P>: a pointer to a byte-sized character type, as C APIs return them: char*,
// signed char*, unsigned char* (sqlite3_column_text), char8_t*, std::byte*, const or not.
template <typename P>
concept c_char_pointer =
    std::is_pointer_v<P> &&
    (std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, char> ||
     std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, signed char> ||
     std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, unsigned char> ||
     std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, char8_t> ||
     std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, std::byte>);

// The parts that are not templates, in runtime/src/interop.cpp.
char* c_string(const string& s);
string go_string(const char* p);
string go_string_n(const char* p, int64_t n);
slice<uint8_t> go_bytes(const void* p, int64_t n);

// c_length converts a length to int64_t; an unsigned value too big for it counts as negative.
template <go_int N>
int64_t c_length(N n) noexcept {
    return static_cast<int64_t>(n);
}

// narrow_float<T>(x) is Go's T(x) for a floating x of a type with more precision than T
// (float(x) for a double, double(x) for a long double): rounded to the nearest T, and ±Inf
// beyond the largest T (C++ makes a value out of T's range undefined behavior). A value above the
// largest T but below it plus half a unit in the last place rounds down to it, as in Go.
template <typename T, typename F>
T narrow_float(F x) noexcept {
    if (x != x) {
        return std::numeric_limits<T>::quiet_NaN();
    }
    constexpr F max = static_cast<F>(std::numeric_limits<T>::max());
    // Half a unit in the last place of T's largest value: 2^(max_exponent - digits - 1).
    constexpr F half_ulp = [] {
        F h = 1;
        for (int i = 0;
             i < std::numeric_limits<T>::max_exponent - std::numeric_limits<T>::digits - 1; i++) {
            h *= 2;
        }
        return h;
    }();
    F a = x < 0 ? -x : x;
    if (a > max) {
        T r = a >= max + half_ulp ? std::numeric_limits<T>::infinity()
                                  : std::numeric_limits<T>::max();
        return x < 0 ? -r : r;
    }
    return static_cast<T>(x);
}

} // namespace detail

// gp::cgo: cgo's C.CString, C.GoString, C.GoStringN, C.GoBytes and C.free, with cgo's exact
// behavior (cmd/cgo/out.go and runtime/string.go of Go 1.25), and as<T>, Go's conversions of
// C/C++ values. Phase 01 rewrites C.CString & co. into the typed gocpp/cgoh package, which the
// emitter writes as these functions.
namespace cgo {

// as<T>(x) is a Go conversion T(x) where x is a C/C++ value or T is a C type: int(cmath.floor(d))
// is as<int64_t>(::floor(d)), C.int(d) is as<int>(d). A floating x to an integer T goes through
// gp::f2i, which gives Go's results (amd64) for values out of T's range, NaN and ±Inf, where a
// static_cast is undefined behavior; a floating x to a floating T with less precision gives ±Inf
// out of range (narrow_float); everything else is static_cast<T>(x), also for C++ class types
// (static_cast<bool>(std::cout)).
template <typename T, typename X>
constexpr T as(X&& x) {
    using F = std::remove_cvref_t<std::decay_t<X>>;
    static_assert(
        !(std::is_pointer_v<F> && std::is_arithmetic_v<T>),
        "gocpp: a C pointer can't be converted to a number: use unsafe.Pointer (step 4.10)");
    static_assert(
        !std::is_same_v<T, bool> || std::is_same_v<F, bool>,
        "gocpp: only a bool converts to bool, as in Go: compare the value instead (x != 0)");
    if constexpr (std::is_floating_point_v<F> && go_int<T>) {
        double d;
        if constexpr (std::numeric_limits<F>::digits > std::numeric_limits<double>::digits) {
            d = detail::narrow_float<double>(x); // a long double: through double, as Go has none
        } else {
            d = static_cast<double>(x);
        }
        return f2i<T>(d);
    } else if constexpr (std::is_floating_point_v<F> && std::is_floating_point_v<T> &&
                         (std::numeric_limits<F>::digits > std::numeric_limits<T>::digits)) {
        return detail::narrow_float<T>(x);
    } else {
        return static_cast<T>(std::forward<X>(x));
    }
}

// CString(s) is C.CString: a copy of every byte of s and a NUL, in C's heap (malloc); free it
// with C.free. A NUL inside s ends the C string early, as in Go. CString("") is a new "" (not
// null). If malloc fails, it is Go's fatal error "runtime: C malloc failed".
inline char* CString(const string& s) {
    return detail::c_string(s);
}

// GoString(p) is C.GoString: the bytes of the C string at p, up to its NUL, copied. A null p
// gives "". p may be any byte-sized char pointer (char*, const char*, unsigned char*, ...).
template <typename P>
    requires detail::c_char_pointer<P>
string GoString(P p) {
    return detail::go_string(reinterpret_cast<const char*>(p));
}
inline string GoString(std::nullptr_t) noexcept {
    return string();
}

// GoStringN(p, n) is C.GoStringN: the n bytes at p, copied (NUL bytes too). n == 0 gives ""
// (p isn't read, so it may be null). A null p with n > 0 panics like Go's nil dereference
// ("invalid memory address or nil pointer dereference"; with GOCPP_NOCHECKS it is undefined
// behavior); a negative n (or one beyond Go's largest allocation) is Go's fatal error "out of
// memory".
template <typename P, go_int N>
    requires detail::c_char_pointer<P>
string GoStringN(P p, N n) {
    return detail::go_string_n(reinterpret_cast<const char*>(p), detail::c_length(n));
}
template <go_int N>
string GoStringN(std::nullptr_t, N n) {
    return detail::go_string_n(nullptr, detail::c_length(n));
}

// GoBytes(p, n) is C.GoBytes: a new []byte with a copy of the n bytes at p. n == 0 gives an
// empty slice that is not nil (Go's make([]byte, 0)), and p isn't read. A negative n (or one
// beyond Go's largest allocation) panics with "runtime error: gobytes: length out of range";
// a null p with n > 0 panics like Go's nil dereference (undefined behavior with
// GOCPP_NOCHECKS). p is any object pointer, as Go's
// unsafe.Pointer: a raw pointer, or a gocpp pointer (gp::ptr, which converts to a raw one).
template <go_int N>
slice<uint8_t> GoBytes(const void* p, N n) {
    return detail::go_bytes(p, detail::c_length(n));
}

// free(p) is C.free: C's free. A null p does nothing.
void free(void* p) noexcept;

// case_eq(tag, v) is a case v of a switch on the C/C++ value tag: tag == v, in an if/else chain
// (Phase 04 §2.5). The tag must be a number, an enum or a pointer: C++'s == of a class type is
// not Go's ==, and a C struct has none.
template <typename A, typename B>
constexpr bool case_eq(const A& tag, const B& v) {
    static_assert(
        std::is_scalar_v<A>,
        "gocpp: a switch on a C/C++ value needs a number, an enum or a pointer (step 4.4)");
    return tag == v;
}

} // namespace cgo

} // namespace gp
