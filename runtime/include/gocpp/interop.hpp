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
//     p := std.minmax(x, y)       auto p = gp::values(std::minmax(x, y));
//     C.cstr(s)                   gp::cstr(s), and gp::cstr(s).gp_ptr() as a C/C++ argument
//     C.wstr(s)                   gp::wstr(s) (wstr.hpp: UTF-16 for the W functions of Windows)
//     C.CString(s)                gp::cgo::CString(s)
//     C.GoString(p)               gp::cgo::GoString(p)
//     C.GoStringN(p, n)           gp::cgo::GoStringN(p, n)
//     C.GoBytes(p, n)             gp::cgo::GoBytes(p, n)
//     C.free(p)                   gp::cgo::free(p)
//     int(x), C.int(d) (x, d C/C++ gp::cgo::as<int64_t>(x), gp::cgo::as<int>(d)
//       values or gocpp numbers)
//     string(x) (x a std::string) gp::cgo::as<gp::string>(x)
//     unsafe.Pointer(cp)          gp::cgo::as<gp::ptr<void>>(cp)  (unsafe.hpp)
//     (*C.T)(p) (p an             gp::cgo::as<T*>(p)
//       unsafe.Pointer)
//     (*byte)(cvoid)              gp::cgo::as<gp::ptr<uint8_t>>(cvoid)
//     a[lo:hi] (a a C array)      gp::sub(gp::cgo::slice_of(a), lo, hi)
//     *x (x opaque)               gp::cgo::star(x)
//     if x (x an opaque bool)     if (gp::cgo::cond(x))
//     b := x == y (x, y opaque)   bool b = gp::cgo::to_bool(x == y);
//
// The conversions at the boundary are members of the runtime types themselves: gp::string to
// std::string_view (string.hpp), gp::slice<T> to std::span<T> (slice.hpp) and gp::ptr<T> to
// T* (ptr.hpp).
#pragma once

#include <gocpp/arith.hpp>
#include <gocpp/array.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/fmt.hpp>
#include <gocpp/print.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>
#include <gocpp/unsafe.hpp>
#include <gocpp/wstr.hpp>

#include <array>
#include <algorithm>
#include <cstddef>
#include <concepts>
#include <cstdint>
#include <iterator>
#include <limits>
#include <memory>
#include <unordered_map>
#include <map>
#include <mutex>
#include <string>
#include <tuple>
#include <type_traits>
#include <utility>
#include <vector>

namespace gp {

namespace detail {

// gp_pointer<X>: X is a gocpp pointer, gp::ptr<T>, or a named type over one (gp::newtype
// derives from it).
template <typename T>
void accept_gp_ptr(const ptr<T>&);
template <typename X>
concept gp_pointer = requires(const X& x) { detail::accept_gp_ptr(x); };

// cptr_elem<T>::type is E when T is exactly gp::cptr<E> (a gocpp pointer to a C/C++ type).
template <typename T>
struct cptr_elem {};
template <typename E>
struct cptr_elem<cptr<E>> {
    using type = E;
};
template <typename T>
concept cptr_like = requires { typename cptr_elem<T>::type; };

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

// nonnull(p) is p after Go's nil check: typed interop (Phase 12, ADR-087) writes x.m for a raw
// pointer x as gp::nonnull(x)->m, which is what gp::deref(x).m does when the type of x is known
// to be a pointer. A null p panics as Go does ("invalid memory address or nil pointer
// dereference"), unless GOCPP_NOCHECKS is defined (then it is p->m of C++). A gocpp pointer
// (gp::ptr, gp::cptr: the type of a Go variable `var p *lib.T`) has its own check in -> and *, and
// is passed through.
template <typename T>
constexpr T* nonnull(T* p) {
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) [[unlikely]] {
        gp::panic_nil();
    }
#endif
    return p;
}
template <typename P>
    requires detail::gp_pointer<std::remove_cvref_t<P>>
constexpr P&& nonnull(P&& p) {
    return std::forward<P>(p);
}

namespace detail {

// nullable_smart<S>: S is a std::unique_ptr or std::shared_ptr, or a class that is like a
// std::optional (has_value(), value() and operator->): an empty one has no object to reach.
template <typename S>
struct is_std_smart_ptr : std::false_type {};
template <typename T, typename D>
struct is_std_smart_ptr<std::unique_ptr<T, D>> : std::true_type {};
template <typename T>
struct is_std_smart_ptr<std::shared_ptr<T>> : std::true_type {};

template <typename S>
concept optional_like = requires(const S& s) {
    { s.has_value() } -> std::same_as<bool>;
    s.value();
    s.operator->();
};

} // namespace detail

// smartnn(s) is s after Go's nil check, for typed interop's s->m with s a smart pointer or an
// optional (Phase 12, ADR-087): an empty one panics as a nil dereference does, where C++ would
// read through a null pointer. Anything else (an iterator, a class of the program with its own
// operator->) is passed through. GOCPP_NOCHECKS removes the check.
template <typename S>
constexpr S&& smartnn(S&& s) {
#ifndef GOCPP_NOCHECKS
    using V = std::remove_cvref_t<S>;
    if constexpr (detail::is_std_smart_ptr<V>::value) {
        if (s == nullptr) [[unlikely]] {
            gp::panic_nil();
        }
    } else if constexpr (detail::optional_like<V>) {
        if (!s.has_value()) [[unlikely]] {
            gp::panic_nil();
        }
    }
#endif
    return std::forward<S>(s);
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
// dangle. Key and Value select only the bindings that the loop uses; no unused key or
// element is copied.
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
template <typename C, bool Key, bool Value>
class range_view {
    using container = std::remove_reference_t<C>;
    // Why a range over a C/C++ value can't work, with a message for users (before the errors
    // that the members below would give): a C number (Go's range over an integer needs a gocpp
    // integer), or a value without begin() and end().
    static_assert(!std::is_arithmetic_v<container>,
                  "gp::range: range over a C number: convert it to a gocpp integer, int(n)");
    static_assert(
        std::is_arithmetic_v<container> ||
            requires(container& c) {
                std::ranges::begin(c);
                std::ranges::end(c);
            },
        "gp::range: range over a C/C++ value without begin() and end(): it works over C++ "
        "containers, views and C arrays");
    using iter = range_iter_t<container>;
    using value = std::iter_value_t<iter>;

    // Go's range over [2][3]int copies each [3]int into v, but a C array can't be copied into
    // a std::pair; range_keys and grid[i] work.
    static_assert(!Value || !std::is_array_v<value>,
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
            if constexpr (!Value) {
                return gp_i; // the element isn't read
            } else {
                container& c = gp_v->gp_c_;
                if constexpr (!std::is_array_v<container>) {
                    gp::check_index(gp_i, static_cast<int64_t>(std::ranges::size(c)));
                }
                if constexpr (Key) {
                    return std::pair<int64_t, value>(
                        gp_i,
                        std::ranges::begin(c)[static_cast<std::iter_difference_t<iter>>(gp_i)]);
                } else {
                    return value(
                        std::ranges::begin(c)[static_cast<std::iter_difference_t<iter>>(gp_i)]);
                }
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
            if constexpr (!Key && !Value) {
                return gp_i; // no binding: do not dereference or copy a discarded map key
            } else if constexpr (map_like<container>) {
                using key = std::remove_cv_t<typename container::key_type>;
                if constexpr (!Value) {
                    return key(gp_it->first);
                } else if constexpr (!Key) {
                    return typename container::mapped_type(gp_it->second);
                } else {
                    return std::pair<key, typename container::mapped_type>(gp_it->first,
                                                                           gp_it->second);
                }
            } else if constexpr (!Value) {
                return gp_i;
            } else if constexpr (!Key) {
                return value(*gp_it);
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
    return detail::range_view<C, true, true>(std::forward<C>(c));
}

// range_keys(c) is `for i := range c`: only the index, or the key for a
// map. No element is copied.
template <typename C>
auto range_keys(C&& c) {
    static_assert(!(std::is_array_v<std::remove_reference_t<C>> && !std::is_lvalue_reference_v<C>),
                  "gp::range_keys: a temporary C array can't be walked");
    return detail::range_view<C, true, false>(std::forward<C>(c));
}

// range_values(c) is `for _, v := range c`: copy only the element (the mapped value).
// range_skip(c) is `for range c`, `for _ = range c`, or `for _, _ = range c`: advance only.
// In particular, a discarded noncopyable map key or element must not prevent the walk.
template <typename C>
auto range_values(C&& c) {
    return detail::range_view<C, false, true>(std::forward<C>(c));
}
template <typename C>
auto range_skip(C&& c) {
    return detail::range_view<C, false, false>(std::forward<C>(c));
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

// is_values<R>: R (a result type, with or without references) is a pair or a tuple.
template <typename R>
inline constexpr bool is_values =
    is_pair<std::remove_cvref_t<R>> || is_tuple<std::remove_cvref_t<R>>;

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
// a temporary. Anything else is returned as a copy, or moved from a temporary, as auto would
// declare it (an array decays to a pointer).
//
// For one new variable or an ordering temporary from an opaque call, the emitter selects this
// helper in a dependent if constexpr only for pairs/tuples. An ordinary result returns directly
// from the immediately-invoked lambda: this preserves guaranteed prvalue elision, including a
// deleted move/copy constructor, while reference members are still copied before the call's
// temporary arguments die. A value-taking helper alone cannot preserve that elision.
template <typename T>
auto values(T&& t) {
    using V = std::remove_cvref_t<T>;
    if constexpr (detail::is_pair<V>) {
        return detail::values_of<std::pair>(std::forward<T>(t), std::make_index_sequence<2>{});
    } else if constexpr (detail::is_tuple<V>) {
        return detail::values_of<std::tuple>(std::forward<T>(t),
                                             std::make_index_sequence<std::tuple_size_v<V>>{});
    } else {
        return std::decay_t<T>(std::forward<T>(t));
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

// to_go_string(x) is Go's string(x) of a C/C++ value x (as<string>, which checks the type of x
// first): a gocpp string that comes through C++ (an element of a std::vector<gp::string>) as it
// is; a copy of the bytes of a std::string, a std::string_view or any other C++ type that
// converts to std::string_view; and for a C integer or enum the UTF-8 encoding of the rune x, as
// Go's string(r).
template <typename X>
string to_go_string(const X& x) {
    using F = std::remove_cvref_t<X>;
    if constexpr (std::is_base_of_v<string, F>) {
        return x;
    } else if constexpr (go_int<F>) {
        return rune_to_string(x);
    } else if constexpr (std::is_enum_v<F>) {
        return rune_to_string(static_cast<std::underlying_type_t<F>>(x));
    } else {
        return string::gp_copy(std::string_view(x));
    }
}

} // namespace detail

// gp::cgo: cgo's C.CString, C.GoString, C.GoStringN, C.GoBytes and C.free, with cgo's exact
// behavior (cmd/cgo/out.go and runtime/string.go of Go 1.25), and as<T>, Go's conversions of
// C/C++ values. The checker rewrites C.CString & co. into the typed gocpp/cgoh package, which the
// emitter writes as these functions.
namespace cgo {

// as<T>(x) is a Go conversion T(x) where x is a C/C++ value or T is a C type: int(cmath.floor(d))
// is as<int64_t>(::floor(d)), C.int(d) is as<int>(d). A floating x to an integer T goes through
// gp::f2i, which gives Go's results (amd64) for values out of T's range, NaN and ±Inf, where a
// static_cast is undefined behavior; a floating x to a floating T with less precision gives ±Inf
// out of range (narrow_float); string(x) copies the bytes of a C++ string (to_go_string);
// everything else is static_cast<T>(x), also for C++ class types (static_cast<bool>(std::cout)).
// as_value is the body; as (below) takes numbers by value, so that a bit-field can be converted.
template <typename T, typename X>
constexpr T as_value(X&& x) {
    using F = std::remove_cvref_t<std::decay_t<X>>;
    static_assert(!(std::is_pointer_v<F> && std::is_arithmetic_v<T>),
                  "gocpp: a C pointer can't be converted to a number: use unsafe.Pointer");
    static_assert(
        !std::is_same_v<T, bool> || std::is_same_v<F, bool>,
        "gocpp: only a bool converts to bool, as in Go: compare the value instead (x != 0)");
    if constexpr (std::is_same_v<T, ptr<void>>) {
        // unsafe.Pointer(p) of a C pointer, a C function or an address (gocpp/unsafe.hpp).
        return detail::to_unsafe_pointer(std::forward<X>(x));
    } else if constexpr (std::is_pointer_v<T> && std::is_same_v<F, ptr<void>>) {
        // (*C.T)(p) of an unsafe.Pointer p.
        return detail::from_unsafe_pointer<T>(x);
    } else if constexpr (gp::detail::cptr_like<T>) {
        // (*C.T)(p) of an unsafe.Pointer or of a C void*: a gocpp pointer to a C type that shares
        // the owner of p (pointers from C own nothing).
        using E = typename gp::detail::cptr_elem<T>::type;
        if constexpr (std::is_same_v<F, ptr<void>>) {
            return T(unsafe_cast<E>(x));
        } else if constexpr (std::is_same_v<F, T>) {
            return x;
        } else {
            static_assert(std::is_pointer_v<F> &&
                              std::is_void_v<std::remove_cv_t<std::remove_pointer_t<F>>>,
                          "gocpp: a C pointer converts to another pointer type only through "
                          "unsafe.Pointer: (*T)(unsafe.Pointer(p))");
            return T(unsafe_cast<E>(detail::to_unsafe_pointer(x)));
        }
    } else if constexpr (detail::gocpp_object_pointer<T>) {
        // (*byte)(p), (*[16]C.char)(p): a gocpp pointer to a gocpp type, of an unsafe.Pointer or
        // of a C void* (which is an unsafe.Pointer for cgo): sharing the owner of p.
        using E = typename detail::object_ptr_elem<T>::type;
        if constexpr (std::is_same_v<F, ptr<void>>) {
            return unsafe_cast<E>(x);
        } else {
            static_assert(std::is_pointer_v<F> &&
                              std::is_void_v<std::remove_cv_t<std::remove_pointer_t<F>>>,
                          "gocpp: a C pointer converts to a gocpp pointer only through "
                          "unsafe.Pointer: (*T)(unsafe.Pointer(p))");
            return unsafe_cast<E>(detail::to_unsafe_pointer(x));
        }
    } else if constexpr (std::is_same_v<T, string>) {
        // Here, not in to_go_string, so that the error names the place of the conversion. A C
        // string (a char pointer or array) has no length to convert: cgo's form is
        // C.GoString(p).
        using R = std::remove_cvref_t<X>;
        static_assert(!std::is_pointer_v<R> && !std::is_array_v<R>,
                      "gocpp: string(p) of a C string: use C.GoString(p) (or C.GoStringN(p, n))");
        static_assert(std::is_base_of_v<string, R> || go_int<R> || std::is_enum_v<R> ||
                          std::is_pointer_v<R> || std::is_array_v<R> ||
                          std::is_convertible_v<const R&, std::string_view>,
                      "gocpp: string(x) needs a std::string, a std::string_view or a C++ value "
                      "that converts to std::string_view");
        if constexpr (!std::is_pointer_v<R> && !std::is_array_v<R>) {
            return detail::to_go_string(x);
        } else {
            return string();
        }
    } else if constexpr (std::is_floating_point_v<F> && go_int<T>) {
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

// A number or an enum is taken by value, because a bit-field (int(f.mode)) can't bind to a
// forwarding reference (a compile error that no overload can avoid); everything else is
// forwarded.
template <typename X>
inline constexpr bool by_value_scalar =
    std::is_arithmetic_v<std::remove_cvref_t<X>> || std::is_enum_v<std::remove_cvref_t<X>>;

template <typename T, typename X>
    requires(!by_value_scalar<X>)
constexpr T as(X&& x) {
    return as_value<T>(std::forward<X>(x));
}

template <typename T, typename X>
    requires by_value_scalar<X>
constexpr T as(X x) {
    return as_value<T>(x);
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
template <typename E>
    requires detail::c_char_pointer<E*>
string GoString(const cptr<E>& p) { // &name[0] of a slice over C chars (Phase 04 step 4.10)
    return GoString(p.gp_get());
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
template <typename E, go_int N>
    requires detail::c_char_pointer<E*>
string GoStringN(const cptr<E>& p, N n) {
    return GoStringN(p.gp_get(), n);
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

// to_bool_vs_string<X>(x) is to_bool(x) for a comparison of a C/C++ value of type X with a string
// constant (p == "text"). If X is a C char pointer or char array, C++ compares addresses, which is
// silently false for an array and unspecified for a pointer: an error here. A std::string, a
// gocpp string that came through C++, and other classes compare as their operator== says.
template <typename X>
inline constexpr bool is_c_text = [] {
    using T = std::remove_cv_t<std::remove_reference_t<X>>;
    if constexpr (std::is_array_v<T>) {
        using E = std::remove_cv_t<std::remove_extent_t<T>>;
        return std::is_same_v<E, char> || std::is_same_v<E, signed char> ||
               std::is_same_v<E, unsigned char>;
    } else if constexpr (std::is_pointer_v<T>) {
        using E = std::remove_cv_t<std::remove_pointer_t<T>>;
        return std::is_same_v<E, char> || std::is_same_v<E, signed char> ||
               std::is_same_v<E, unsigned char>;
    } else {
        return false;
    }
}();

// case_eq(tag, v) is a case v of a switch on the C/C++ value tag: tag == v, in an if/else chain
// (Phase 04 §2.5). The tag must be a number, an enum or a pointer: C++'s == of a class type is
// not Go's ==, and a C struct has none.
template <typename A, typename B>
constexpr bool case_eq(const A& tag, const B& v) {
    static_assert(std::is_scalar_v<A> || gp::detail::cptr_like<A>,
                  "gocpp: a switch on a C/C++ value needs a number, an enum or a pointer");
    static_assert(!(is_c_text<A> && std::is_array_v<B>),
                  "invalid operation: comparing a C string (char*) with a Go string compares "
                  "addresses: use C.GoString(x) == \"...\" (or strcmp)");
    return tag == v;
}

// cond(x) is a C/C++ value x used as a condition (if, for, &&, ||, !; Phase 04 §2.5): Go has
// only bool conditions, so a C int (if isdigit(c)), a pointer (if p) or an object with only an
// explicit operator bool (std::optional) is a clear C++ error here, where C++ would convert it
// silently. A bool passes, and so does an object that converts to bool implicitly, as the
// proxy of a std::vector<bool> element does (if bs[0]). The emitter writes it only for a
// condition that is not a comparison or a logical operator, which give a bool anyway.
template <typename X>
constexpr bool cond(const X& x) {
    static_assert(std::is_same_v<X, bool> ||
                      (std::is_class_v<X> && std::is_convertible_v<const X&, bool>),
                  "gocpp: non-boolean condition: a C/C++ value used as a condition must be a "
                  "bool (compare it: x != 0, p != nil)");
    return x;
}

// to_bool(x) is the result of a comparison of C/C++ values (x == y, p == nil, a < b), a Go bool
// as in cgo (Phase 04 §2.5): the bool of a C++ comparison, or the result of a class's operator==
// that converts to bool.
template <typename X>
constexpr bool to_bool(const X& x) {
    static_assert(std::is_convertible_v<const X&, bool>,
                  "gocpp: the result of this C++ comparison doesn't convert to bool");
    return x;
}

template <typename X, typename B>
constexpr bool to_bool_vs_string(const B& x) {
    static_assert(!is_c_text<X>,
                  "invalid operation: comparing a C string (char*) with a Go string compares "
                  "addresses: use C.GoString(x) == \"...\" (or strcmp)");
    return to_bool(x);
}

// field_ok<F> checks the type F of a field that a keyed literal of a C/C++ type sets by an
// assignment (Phase 04 §2.5): [&] { T gp_t0{}; static_assert(gp::cgo::field_ok<decltype(gp_t0.w)>);
// gp_t0.w = {640}; return gp_t0; }(). decltype names the type without a reference to the field,
// so a bit-field works too. An array can't be assigned, so an array field is a clear error here
// (field_check's own message; field_ok itself is always true) until Phase 12 writes designated
// initializers, which need the fields' order.
template <typename F>
struct field_check {
    static_assert(!std::is_array_v<F>,
                  "gocpp: a keyed literal of a C/C++ type can't set an array field (until Phase "
                  "12): write a positional literal, T{a, b, c}");
    static constexpr bool gp_ok = true;
};
template <typename F>
inline constexpr bool field_ok = field_check<F>::gp_ok;

// field_zero<T> checks the type T of a field of a gocpp struct that is a C/C++ type: Go's zero
// value of the struct (var s S, new(S), a literal without the field) needs one of the field, which
// is T() here, so a type with no default constructor is a clear error at the field's line.
template <typename T>
struct field_zero_check {
    static_assert(std::is_default_constructible_v<T>,
                  "gocpp: this C++ type has no default constructor, so a struct field of it has no "
                  "zero value (Go needs one): use a pointer to it, *T, or a std.unique_ptr[T]");
    static constexpr bool gp_ok = true;
};
template <typename T>
inline constexpr bool field_zero = field_zero_check<T>::gp_ok;

// map_t<T> is the C++ map type T of a keyed literal whose keys are not field names,
// std.map[std.string, int]{"a": 1}, written gp::cgo::map_t<T>{{"a", 1}} (Phase 04 §2.5). A type
// without a mapped_type is an error: Go's keys of a sequence (std.vector[int]{2: 7}) are indexes,
// which a braced list of pairs would silently turn into elements.
template <typename T>
struct map_literal {
    static_assert(
        requires { typename T::mapped_type; },
        "gocpp: a keyed literal of a C++ type with keys that aren't field names needs a "
        "map (std.map, std.unordered_map, ...)");
    using gp_type = T;
};
template <typename T>
using map_t = typename map_literal<T>::gp_type;

// value_eq(a, b) is Go's a == b of a gocpp struct or array with C/C++ parts: the struct's operator==
// (or gp::array's), which compares the C/C++ parts with C++'s ==; a C/C++ part without one is an error
// at the comparison's line.
template <typename T>
bool value_eq(const T& a, const T& b) {
    static_assert(std::equality_comparable<T>,
                  "gocpp: == compares the C/C++ parts of this value with C++'s ==, which a C/C++ type "
                  "in it doesn't have (compare the other parts yourself, or declare an operator== "
                  "for that type)");
    if constexpr (std::equality_comparable<T>) {
        return a == b;
    } else {
        return false;
    }
}

// key_ok<K> says whether a gocpp map can hash and compare the key type K with a C/C++ part: a C/C++
// type with std::hash and ==, a gocpp array of such elements (gp::array's own gp_hash, element by
// element), or a gocpp struct with C/C++ fields, whose gp_hash is a template that requires key_ok
// of those fields (lower/typedecl.go), and whose == is the defaulted one (deleted when a field has
// none).
template <typename K>
struct key_ok
    : std::bool_constant<std::equality_comparable<K> &&
                         (requires(const K& a) { std::hash<K>{}(a); } || requires(const K& a) { a.gp_hash(); })> {};
template <typename T, int64_t N>
struct key_ok<gp::array<T, N>> : key_ok<T> {};

// map_key<K> is the C/C++ key type K of a gocpp map (map[std.string]int, gp::map<map_key<K>, V>,
// which is gp::map<K, V>): gp::map hashes a key that has no hash of gocpp's with std::hash and
// compares it with ==, so a type that lacks them (std.vector[int]) is a clear error at the map's line,
// not one deep in gocpp/hash.hpp. (After the error the key is bad_map_key, which the map can hash, so
// that the error is the only one.)
struct bad_map_key {
    bool operator==(const bad_map_key&) const = default;
    std::size_t gp_hash() const { return 0; }
};
template <typename K>
struct map_key_check {
    static constexpr bool gp_ok = key_ok<K>::value;
    static_assert(gp_ok,
                  "gocpp: this C/C++ type can't be the key of a gocpp map: the map hashes it with "
                  "std::hash and compares it with ==, which it doesn't have (use a Go key, or a C++ "
                  "map such as std.map[K, V])");
    using gp_type = std::conditional_t<gp_ok, K, bad_map_key>;
};
template <typename K>
using map_key = typename map_key_check<K>::gp_type;

// star(x) is *x for a C/C++ value x (opaque mode): a raw pointer to an object is dereferenced
// with Go's nil check (gp::deref), and so is a gocpp pointer; everything else is C++'s *x, so
// C++ smart pointers and iterators keep their own operator* ((*sp).m, *it).
template <typename X>
constexpr decltype(auto) star(X&& x) {
    using V = std::remove_cvref_t<X>;
    if constexpr (detail::raw_object_pointer<V> || detail::gp_pointer<V>) {
        return gp::deref(std::forward<X>(x));
    } else {
        return *std::forward<X>(x);
    }
}

namespace detail {

template <typename T>
inline constexpr bool is_std_array = false;
template <typename E, std::size_t N>
inline constexpr bool is_std_array<std::array<E, N>> = true;

} // namespace detail

// slice_of(a) is the slice of all the elements of a, for a slice expression a[lo:hi] of a C/C++
// value (a[lo:hi] is gp::sub(gp::cgo::slice_of(a), lo, hi), with gp::sub's Go checks and panics;
// Phase 04 step 4.10): a C array (a field char name[16], a global int table[8]) or a
// std::array, as cgo slices a C array. The slice is a view of the C++ memory, which it owns
// nothing of (gp::foreign_owner), so C++'s lifetime rules apply: the array must outlive the
// slice. A const element loses its const, as in cgo. Anything else has no slice expression in
// Go (a C pointer has no length: unsafe.Slice(p, n); a C++ container is not an array).
template <typename A>
auto slice_of(A&& a) {
    using V = std::remove_cvref_t<A>;
    static_assert(std::is_lvalue_reference_v<A>,
                  "gocpp: slicing a temporary C/C++ array: the slice would dangle at once");
    if constexpr (std::is_array_v<V>) {
        using E = std::remove_cv_t<std::remove_extent_t<V>>;
        static_assert(!std::is_array_v<E>,
                      "gocpp: slicing a C array of arrays: slice a row, a[i][:]");
        auto n = static_cast<int64_t>(std::extent_v<V>);
        return slice<E>(foreign_owner(), const_cast<E*>(&a[0]), n, n);
    } else if constexpr (detail::is_std_array<V>) {
        using E = std::remove_cv_t<typename V::value_type>;
        auto n = static_cast<int64_t>(a.size());
        return slice<E>(foreign_owner(), const_cast<E*>(a.data()), n, n);
    } else {
        static_assert(detail::is_std_array<V>,
                      "gocpp: only a C array or a std::array has a slice expression: for a C "
                      "pointer use unsafe.Slice(p, n), and C++ containers have no slices");
        return slice<int>();
    }
}

namespace detail {

// out_pointer is the argument &p of a C function whose parameter is a void**, for an
// unsafe.Pointer variable p (a gp::ptr<void>, 16 bytes: no cast makes it a void*): a void* that
// the function can write to, started with p's address, and written back into p when the
// statement ends (the temporary's destructor), so a function that allocates (void** out) gives
// p the new address. A new address is C memory (foreign_owner); if the function left it alone, p
// keeps its owner.
class out_pointer {
public:
    explicit out_pointer(ptr<ptr<void>> target) noexcept
        : gp_target_(std::move(target)),
          gp_raw_(gp_target_ == nullptr ? nullptr : (*gp_target_).gp_get()) {}
    out_pointer(const out_pointer&) = delete;
    out_pointer& operator=(const out_pointer&) = delete;
    ~out_pointer() {
        if (gp_target_ != nullptr && gp_raw_ != (*gp_target_).gp_get()) {
            *gp_target_ = unsafe_from_addr(reinterpret_cast<std::uintptr_t>(gp_raw_));
        }
    }
    operator void**() noexcept { return &gp_raw_; }

private:
    ptr<ptr<void>> gp_target_;
    void* gp_raw_;
};

} // namespace detail

// out_ptr(&p) is that argument (detail::out_pointer).
inline detail::out_pointer out_ptr(ptr<ptr<void>> target) {
    return detail::out_pointer(std::move(target));
}

// out_cptr is the same for the argument &p of a C function whose parameter is a T** (sqlite3_open's
// sqlite3**), with p a gocpp pointer to a C type (a gp::cptr<T>: 16 bytes, so &p is no T**): a T*
// that the function writes, started with p's address, and written back into p when the statement
// ends. A new address is C memory (foreign_owner); if the function left it alone, p keeps its
// owner.
template <typename T>
class out_cptr {
public:
    explicit out_cptr(cptr<T>& target) noexcept : gp_target_(target), gp_raw_(target.gp_get()) {}
    out_cptr(const out_cptr&) = delete;
    out_cptr& operator=(const out_cptr&) = delete;
    ~out_cptr() {
        if (gp_raw_ != gp_target_.gp_get()) {
            gp_target_ = cptr<T>(gp_raw_);
        }
    }
    operator T**() noexcept { return &gp_raw_; }

private:
    cptr<T>& gp_target_;
    T* gp_raw_;
};

template <typename T>
out_cptr<T> out_ptr(cptr<T>& target) {
    return out_cptr<T>(target);
}

// out_ptr(p) of a raw pointer variable p (a T* of a **C.T chain, or a pointer from C): &p itself.
template <typename T>
T** out_ptr(T*& target) noexcept {
    return &target;
}

// ---- argument adaptations of typed interop (ADR-130) ------------------------------------------
//
// These are what the lowering writes for an argument that the C++ type of the parameter adapts
// (internal/lower/typedadapt.go). Each is a temporary: it lives until the end of the statement that
// holds the call, and the call is the only one that may use what it points to.

namespace detail {

// c_name is the name of the C number type T in the messages of arg_in.
template <typename T>
constexpr const char* c_name() {
    if constexpr (std::is_same_v<T, int>) {
        return "int";
    } else if constexpr (std::is_same_v<T, unsigned int>) {
        return "unsigned int";
    } else if constexpr (std::is_same_v<T, long>) {
        return "long";
    } else if constexpr (std::is_same_v<T, unsigned long>) {
        return "unsigned long";
    } else if constexpr (std::is_same_v<T, long long>) {
        return "long long";
    } else if constexpr (std::is_same_v<T, unsigned long long>) {
        return "unsigned long long";
    } else if constexpr (std::is_same_v<T, short>) {
        return "short";
    } else if constexpr (std::is_same_v<T, unsigned short>) {
        return "unsigned short";
    } else if constexpr (std::is_same_v<T, signed char>) {
        return "signed char";
    } else if constexpr (std::is_same_v<T, unsigned char>) {
        return "unsigned char";
    } else {
        return "a C integer";
    }
}

// arg_in converts the value v of the gocpp variable `name` to the C type T for the copy that the
// call gets. The lowering uses it only for integers (floats are widened only, never narrowed): a
// value that does not fit T is the panic of arg_overflow, not a silent wrap.
template <typename T, typename G>
T arg_in(G v, const char* name) {
    if constexpr (std::is_integral_v<G> && std::is_integral_v<T> && !std::is_same_v<G, T>) {
        if (!std::in_range<T>(v)) {
            ::gp::detail::arg_overflow(name, std::to_string(v).c_str(), c_name<T>());
        }
    }
    return static_cast<T>(v);
}

// scalar_out is the argument &x of a C function whose parameter is a T* (not const), for a gocpp
// variable x of another number type G: a T* to a temporary that starts as x's value and that x gets
// back when the statement ends (the destructor, so it also runs when a C++ exception leaves the
// call). The lowering uses it only when every value of T is a value of G, so the write-back never
// loses.
template <typename T, typename G>
class scalar_out {
public:
    scalar_out(G& target, const char* name)
        : gp_target_(target), gp_tmp_(arg_in<T>(target, name)) {}
    scalar_out(const scalar_out&) = delete;
    scalar_out& operator=(const scalar_out&) = delete;
    ~scalar_out() { gp_target_ = static_cast<G>(gp_tmp_); }
    operator T*() noexcept { return &gp_tmp_; }

private:
    G& gp_target_;
    T gp_tmp_;
};

// scalar_in is the same for a const T* parameter: the callee reads the temporary and x is not
// written.
template <typename T, typename G>
class scalar_in {
public:
    scalar_in(const G& source, const char* name) : gp_tmp_(arg_in<T>(source, name)) {}
    scalar_in(const scalar_in&) = delete;
    scalar_in& operator=(const scalar_in&) = delete;
    operator const T*() const noexcept { return &gp_tmp_; }

private:
    T gp_tmp_;
};

// out_void is the argument &p of a C function whose parameter is a void**, for a gocpp pointer to a
// C type p (a gp::cptr<T>): a void* that the function writes, started with p's address, and stored
// into p when the statement ends (as out_cptr does for a T**). The type of p says what the function
// writes there, as the conversion (*T)(raw) of the unsafe.Pointer form did.
template <typename T>
class out_void {
public:
    explicit out_void(cptr<T>& target) noexcept
        : gp_target_(target), gp_raw_(const_cast<std::remove_cv_t<T>*>(target.gp_get())) {}
    out_void(const out_void&) = delete;
    out_void& operator=(const out_void&) = delete;
    ~out_void() {
        if (gp_raw_ != static_cast<const void*>(gp_target_.gp_get())) {
            gp_target_ = cptr<T>(static_cast<T*>(gp_raw_));
        }
    }
    operator void**() noexcept { return &gp_raw_; }

private:
    cptr<T>& gp_target_;
    void* gp_raw_;
};

} // namespace detail

// scalar_arg<T>(&x, "x") is that argument for a T* parameter (lvalue x of a gocpp number type), and
// scalar_in<T> the one for a const T*. The target is a gocpp pointer or a raw pointer to x.
template <typename T, typename P>
auto scalar_arg(P&& target, const char* name) {
    using G = std::remove_cvref_t<decltype(*target)>;
    return detail::scalar_out<T, G>(*target, name);
}

template <typename T, typename P>
auto scalar_in(P&& source, const char* name) {
    using G = std::remove_cvref_t<decltype(*source)>;
    return detail::scalar_in<T, G>(*source, name);
}

// void_out(p) is the argument &p for a void** parameter (detail::out_void).
template <typename T>
detail::out_void<T> void_out(cptr<T>& target) {
    return detail::out_void<T>(target);
}

// call_out<T>(&x, "x", f) is f(p) with p the T* of scalar_arg<T>(&x, "x"), and x gets the value
// the callee stored in *p as soon as f has returned: the rest of the Go expression
// (`f(&n) == 0 && n > 0`) sees the new x, as it does in Go. The lowering writes f as a lambda that
// makes the call.
template <typename T, typename P, typename F>
decltype(auto) call_out(P&& target, const char* name, F&& f) {
    using G = std::remove_cvref_t<decltype(*target)>;
    detail::scalar_out<T, G> out(*target, name);
    return std::forward<F>(f)(static_cast<T*>(out));
}

// call_void_out(p, f) is the same for the void** parameter: f(&raw), and p gets raw when f has
// returned.
template <typename T, typename F>
decltype(auto) call_void_out(cptr<T>& target, F&& f) {
    detail::out_void<T> out(target);
    return std::forward<F>(f)(static_cast<void**>(out));
}

// raw(x) is x for a C/C++ function's argument, with a gocpp pointer (a gp::ptr, a gp::cptr) as its
// raw address: through a C function's `...` a class object is an error, and f(bool) + f(void*) is
// ambiguous for it.
template <typename X>
decltype(auto) raw(X&& x) {
    if constexpr (gp::detail::gp_pointer<std::remove_cvref_t<X>>) {
        return x.gp_get();
    } else {
        return std::forward<X>(x);
    }
}

namespace detail {

// same_block reports whether two owners are one block.
inline bool same_block(const std::shared_ptr<const void>& a,
                       const std::shared_ptr<const void>& b) noexcept {
    return !a.owner_before(b) && !b.owner_before(a);
}

// needs_keep reports whether the owner o is one that keeps something alive: not a nil pointer's,
// not that of C memory (foreign_owner) or of a package variable (static_owner), which are never
// freed.
inline bool needs_keep(const std::shared_ptr<const void>& o) noexcept {
    return o.use_count() != 0 && !same_block(o, foreign_owner()) && !same_block(o, static_owner());
}

// owner_of(p) is the owner of the object that the pointer p points to, if it needs one kept: a
// gocpp pointer's; for a raw pointer, the block of the C struct that gocpp owns at that address
// (gp::detail::box_owner: the object that n.next = head stored in a member, read back as a raw
// pointer), else nothing: C memory is not ours to keep.
template <typename P>
std::shared_ptr<const void> owner_of(const P& p) {
    if constexpr (gp::detail::gp_pointer<P>) {
        std::shared_ptr<const void> o(p.gp_shared());
        return needs_keep(o) ? o : nullptr;
    } else if constexpr (std::is_pointer_v<P> && (std::is_object_v<std::remove_pointer_t<P>> ||
                                                  std::is_void_v<std::remove_pointer_t<P>>)) {
        return gp::detail::box_owner(static_cast<const void*>(p));
    } else {
        return nullptr;
    }
}

// keep_mutex(k) guards the keeper k (stores into members are rare, and a keeper is small: the locks
// are shared by address). They are never destroyed, so a store during static destruction is safe.
inline std::mutex& keep_mutex(const void* k) noexcept {
    static std::mutex* const shards = new std::mutex[64];
    const auto u = reinterpret_cast<std::uintptr_t>(k);
    return shards[((u >> 4) ^ (u >> 10)) % 64];
}

// keeper holds the owners of the objects that the raw pointer members of a struct point to, by the
// address of the member: a store replaces the entry of its member (or drops it for a nil or a raw
// pointer), so a loop that stores into one member holds one object, not a growing list. A literal's
// pointers have no member (the key is null): they are held until the struct dies. The entries are
// a vector, searched in place while there are few (a node has one) and through a hash index when
// there are many (an array member of 1000 pointers). The owners are dropped through
// gp::detail::release, so freeing a list of a million nodes doesn't recurse a million deep.
struct keeper {
    using entry = std::pair<const void*, std::shared_ptr<const void>>;
    static constexpr std::size_t gp_scan = 16;
    std::vector<entry> gp_held;
    std::unique_ptr<std::unordered_map<const void*, std::size_t>> gp_index;

    keeper() = default;
    keeper(keeper&&) noexcept = default;
    keeper(const keeper&) = delete;
    keeper& operator=(const keeper&) = delete;
    keeper& operator=(keeper&&) = delete;
    ~keeper() { clear(); }

    // clear drops every owner held, through gp::detail::release.
    void clear() noexcept {
        std::vector<entry> held;
        held.swap(gp_held);
        gp_index.reset();
        for (entry& e : held) {
            gp::detail::release(e.second);
        }
    }

    // set makes owner the one held for the member at key (nothing for a nil owner), and returns
    // the owner that it replaced, for the caller to drop outside the lock (release).
    std::shared_ptr<const void> set(const void* key, std::shared_ptr<const void> owner) {
        std::lock_guard<std::mutex> lock(keep_mutex(this));
        if (key == nullptr) {
            if (owner) {
                gp_held.emplace_back(nullptr, std::move(owner));
            }
            return nullptr;
        }
        const std::size_t i = find(key);
        if (i == npos) {
            if (owner) {
                add(key, std::move(owner));
            }
            return nullptr;
        }
        std::shared_ptr<const void> old = std::move(gp_held[i].second);
        if (owner) {
            gp_held[i].second = std::move(owner);
        } else {
            erase_at(i);
        }
        return old;
    }

private:
    static constexpr std::size_t npos = static_cast<std::size_t>(-1);

    std::size_t find(const void* key) const {
        if (gp_index) {
            auto it = gp_index->find(key);
            return it == gp_index->end() ? npos : it->second;
        }
        for (std::size_t i = 0; i < gp_held.size(); i++) {
            if (gp_held[i].first == key) {
                return i;
            }
        }
        return npos;
    }
    void add(const void* key, std::shared_ptr<const void> owner) {
        gp_held.emplace_back(key, std::move(owner));
        if (gp_index) {
            (*gp_index)[key] = gp_held.size() - 1;
        } else if (gp_held.size() > gp_scan) {
            gp_index = std::make_unique<std::unordered_map<const void*, std::size_t>>();
            for (std::size_t i = 0; i < gp_held.size(); i++) {
                if (gp_held[i].first != nullptr) {
                    (*gp_index)[gp_held[i].first] = i;
                }
            }
        }
    }
    void erase_at(std::size_t i) {
        if (gp_index) {
            gp_index->erase(gp_held[i].first);
        }
        const std::size_t last = gp_held.size() - 1;
        if (i != last) {
            gp_held[i] = std::move(gp_held[last]);
            if (gp_index && gp_held[i].first != nullptr) {
                (*gp_index)[gp_held[i].first] = i;
            }
        }
        gp_held.pop_back();
    }
};

// keep_deleter is the deleter of the box of a struct made by box_keep: it destroys the object and
// then drops the owners the keeper holds (the control block, and the keeper in it, may outlive the
// object: the registry holds a weak pointer to it until it is rehashed). get_deleter finds it from
// any pointer that shares the box (an alias too), so store_keep needs no type.
struct keep_deleter {
    mutable keeper gp_keeper;
    void (*gp_destroy)(void*);
    void operator()(void* p) const {
        gp::detail::box_died(p);
        gp_destroy(p);
        gp_keeper.clear();
    }
};

// The owners that structs without a keeper hold for their raw pointer members: a C struct in a
// gocpp struct, in a slice, or one that a pointer aliases (&s.f, &xs[i]); package variables. They
// are kept by the owner's block, in a table that is purged when it doubles (the owners of dead
// blocks are dropped then, not at the moment the block dies: later than a keeper's).
struct side_table {
    std::mutex gp_mu;
    std::map<std::weak_ptr<const void>, keeper, std::owner_less<>> gp_map;
    std::size_t gp_limit = 64;
};
inline side_table& side() {
    static side_table* const t = new side_table;
    return *t;
}

inline void side_store(const std::shared_ptr<const void>& block, const void* key,
                       std::shared_ptr<const void> held) {
    std::vector<keeper> dead; // the keepers of dead blocks, dropped after the lock
    std::shared_ptr<const void> old;
    {
        side_table& t = side();
        std::lock_guard<std::mutex> lock(t.gp_mu);
        if (t.gp_map.size() >= t.gp_limit) {
            for (auto it = t.gp_map.begin(); it != t.gp_map.end();) {
                if (it->first.expired()) {
                    dead.push_back(std::move(it->second));
                    it = t.gp_map.erase(it);
                } else {
                    ++it;
                }
            }
            t.gp_limit = std::max<std::size_t>(64, 2 * t.gp_map.size());
        }
        auto it = t.gp_map.find(block);
        if (it == t.gp_map.end()) {
            if (!held) {
                return;
            }
            it = t.gp_map.try_emplace(std::weak_ptr<const void>(block)).first;
        }
        old = it->second.set(key, std::move(held));
    }
    if (old) {
        gp::detail::release(old);
    }
}

[[noreturn]] inline void keep_fatal() {
    gp::detail::fatal(
        "cannot keep the object alive: the pointer is stored into C memory (a struct that came "
        "from C), which can't hold a gocpp pointer: allocate the struct with &T{...}, or keep the "
        "object in a variable for as long as C uses it");
}

template <typename O>
concept slice_owner = requires(const O& o) { o.gp_owner(); };

template <typename O>
inline constexpr bool owner_kind = gp::detail::gp_pointer<O> || slice_owner<O>;

// keep_store records held as the owner kept for the member at key of the struct in the block that
// owner owns: owner is the gocpp pointer to the struct, or the slice that has it as an element.
template <typename O>
void keep_store(const O& owner, const void* key, std::shared_ptr<const void> held) {
    std::shared_ptr<const void> block;
    if constexpr (gp::detail::gp_pointer<O>) {
        if (auto* d = std::get_deleter<keep_deleter>(owner.gp_shared())) {
            if (held && same_block(held, owner.gp_shared())) {
                held = nullptr; // a pointer to the struct itself: the block owns both
            }
            if (auto old = d->gp_keeper.set(key, std::move(held))) {
                gp::detail::release(old);
            }
            return;
        }
        block = owner.gp_shared();
    } else {
        block = owner.gp_owner();
    }
    if (held && block && same_block(held, block)) {
        held = nullptr; // &nodes[i+1] stored in nodes[i]: one allocation owns both
    }
    if (!block) {
        if (held) {
            keep_fatal(); // a nil owner (the member access panics first), or a stack value
        }
        return;
    }
    if (same_block(block, foreign_owner())) {
        if (held) {
            keep_fatal();
        }
        return;
    }
    side_store(block, key, std::move(held));
}

} // namespace detail

// box_keep<T>(make, kept...) is a gp::cptr<T> to a new T built in place by make(kept...) (no copy
// or move: a std.mutex works), the box of &lib.Node{...}. The members of a C struct are raw
// pointers, so the box holds the owners of the objects that the pointers in the literal point to
// (the kept pointers: &lib.Node{next: head}), and of those stored into its members later
// (store_keep), until it dies, as Go's pointer keeps its object. The object is registered under
// its address while it lives (gp::detail::box_owner), so the raw pointers to it that C structs
// hold convert back into pointers that own it (a read of n.next).
template <typename T, typename F, typename... P>
cptr<T> box_keep(F&& make, const P&... kept) {
    detail::keeper k;
    (k.set(nullptr, detail::owner_of(kept)), ...);
    T* obj = new T(make(kept...));
    std::shared_ptr<void> sp(
        static_cast<void*>(obj),
        detail::keep_deleter{std::move(k), [](void* p) { delete static_cast<T*>(p); }});
    gp::detail::box_register(static_cast<const void*>(obj), sp);
    return cptr<T>(ptr<T>(std::shared_ptr<T>(sp, obj)));
}

// keep_path<L...> is the types of the members that a store goes through between the pointer that
// owns a struct and the member (n.a.b.next: the types of n.a and n.a.b). A raw pointer among them
// is a struct that is not n's: the pointer stored would be kept by nothing.
template <typename... L>
struct keep_path {
    static_assert((!std::is_pointer_v<std::remove_cvref_t<L>> && ...),
                  "gocpp: cannot keep the object alive: the struct is reached through a raw C "
                  "pointer: assign it through a *T variable you own, or write the literal "
                  "&T{...}");
};

// store_keep(owner, member, value) is member = value for a member of the struct that the gocpp
// pointer owner owns (n.next = head, n.slots[i] = p; also the slice that has the struct as an
// element, nodes[i].next = p): a raw pointer member gets the raw address of value and the owner of
// the struct keeps value's object alive, replacing what the member kept before (nothing for nil or
// a pointer that is C's), as Go's pointer would. A value that is a raw pointer to a C struct that
// gocpp owns (n.next = other.next) keeps that struct. Any other member just gets the value. A
// gocpp pointer stored into C memory is a fatal error: nothing could keep it alive.
template <typename O, typename M, typename V, typename... L>
void store_keep(const O& owner, M& member, V&& value, keep_path<L...> = {}) {
    static_assert(detail::owner_kind<O>,
                  "gocpp: cannot keep the object alive: the struct is not owned by a *T variable "
                  "(it is a C/C++ local value, or a raw pointer from C): declare it n := &T{...} "
                  "and store through n");
    if constexpr (std::is_pointer_v<M>) {
        std::shared_ptr<const void> held = detail::owner_of(value);
        if constexpr (gp::detail::gp_pointer<std::remove_cvref_t<V>>) {
            member = value.gp_get();
        } else {
            member = value;
        }
        detail::keep_store(owner, static_cast<const void*>(&member), std::move(held));
    } else {
        member = std::forward<V>(value);
    }
}

// load(x) is the value of the member read x (auto next = cur.next): a raw pointer to a C struct
// becomes a gp::cptr, which owns the struct if gocpp does (gp::detail::box_owner), so that the
// variable keeps it alive when cur's member is stored again; any other value is x itself (a copy
// when it initializes an auto variable). A number or an enum is taken by value, so that a bit-field
// (x := b.mode) can be read: it can't bind to a forwarding reference.
template <typename X>
    requires(!by_value_scalar<X>)
decltype(auto) load(X&& x) {
    using P = std::remove_cvref_t<X>;
    if constexpr (std::is_pointer_v<P> && std::is_class_v<std::remove_pointer_t<P>>) {
        return cptr<std::remove_pointer_t<P>>(x);
    } else {
        return std::forward<X>(x);
    }
}
template <typename X>
    requires by_value_scalar<X>
X load(X x) {
    return x;
}
template <typename X>
    requires(!std::is_array_v<X> && std::is_copy_constructible_v<X> && !by_value_scalar<X>)
X load(const X& x) {
    return x;
}

// fmt_arg(x) is an argument of fmt.Print, Println, Printf and the Sprint functions that has a
// C/C++ part (a C/C++ value, or a gocpp value with one inside, Phase 04 §2.12), x being
// gp::fmt::as(descriptor, value) or the value itself: the operand, after a check that fmt can print
// every part of it (gocpp/fmt.hpp, "C/C++ values"), and the registration of the text of its C++
// classes (their std::formatter or operator<<), which the descriptors of the classes print by. It
// is here so that the error is the user's, at the call's .gp line.
template <typename T>
auto fmt_arg(fmt::described<T> x) {
    static_assert(fmt::detail::printable<T>(),
                  "gocpp: fmt can't print a C++ type in this value: it has no std::formatter "
                  "and no operator<< (print its parts, or declare an operator<< for it; an "
                  "operator<< must be declared in the type's namespace)");
    static_assert(fmt::detail::keys_ordered<T>(),
                  "gocpp: fmt prints the keys of a map in order, and a C++ class key needs an "
                  "operator< for that (declare one, or print the entries yourself)");
    static const bool registered = (fmt::detail::register_texts<T>(), true);
    (void)registered;
    return x;
}

template <typename T>
auto fmt_arg(fmt::described<fmt::heap_value<T>> x) {
    static_assert(fmt::detail::printable<T>(),
                  "gocpp: fmt can't print a C++ type in this value: it has no std::formatter "
                  "and no operator<< (print its parts, or declare an operator<< for it; an "
                  "operator<< must be declared in the type's namespace)");
    static_assert(fmt::detail::keys_ordered<T>(),
                  "gocpp: fmt prints the keys of a map in order, and a C++ class key needs an "
                  "operator< for that (declare one, or print the entries yourself)");
    static const bool registered = (fmt::detail::register_texts<T>(), true);
    (void)registered;
    return x;
}

// fmt_arg(x) of a value with no descriptor (a C/C++ value, or a gocpp composite with C/C++ parts
// that go/types has no type for) deduces it from the C++ type (gp::cxx_desc).
template <typename T>
auto fmt_arg(T x) {
    return fmt_arg(fmt::described<T>{gp::cxx_desc<T>(), std::move(x)});
}

// print_arg(x) is the same for an argument of the builtin print and println, which print a
// C/C++ number, bool, enum, raw pointer, nullptr or C++ string (gocpp/print.hpp).
template <typename T>
auto print_arg(T x) {
    if constexpr (gp::detail::cptr_like<T>) {
        return print_arg(x.gp_get());
    } else {
        static_assert(gp::detail::print_ok<T>,
                      "gocpp: print and println can't print a C++ object: use fmt.Println, or "
                      "print its parts");
        return gp::detail::cxx_print<T>{std::move(x)};
    }
}

// --- boxes of C/C++ objects (Phase 04 step 4.8) ---
//
// A C/C++ local that a closure or a deferred call uses lives in a box, a gp::ptr<T> (README
// §2.5, docs/spec/interop.md "C/C++ objects in closures and defer"): the closure copies the box
// and shares the object, as it shares a Go variable, and the object lives as long as the last
// box. The box of a C++ type is made where the variable is declared:
//
//     v := std.vector[int]{}      auto v = gp::cgo::box([&] { return ::std::vector<int64_t>{}; });
//     var m std.mutex             gp::ptr<::std::mutex> m = gp::cgo::new_box<::std::mutex>();
//     var n C.int = f()           gp::ptr<int> n = gp::cgo::box_as<int>([&]() -> int { return
//     ::f(); });
//
// The object is built in the box from the factory's prvalue (guaranteed copy elision), so
// a type that can be neither copied nor moved (std::mutex, std::atomic, std::lock_guard) can be
// boxed, and a movable one is not moved needlessly. (gp::new_<T>(f()) would move.)

namespace detail {

// lazy_alloc is an allocator whose construct() builds the object from the prvalue that a
// factory returns, which std::allocate_shared hands to it: `U(make())` is initialized in place.
template <typename T>
struct lazy_alloc {
    using value_type = T;
    lazy_alloc() noexcept = default;
    template <typename U>
    lazy_alloc(const lazy_alloc<U>&) noexcept {}
    T* allocate(std::size_t n) { return std::allocator<T>().allocate(n); }
    void deallocate(T* p, std::size_t n) noexcept { std::allocator<T>().deallocate(p, n); }
    template <typename U, typename F>
    void construct(U* p, F&& make) {
        ::new (static_cast<void*>(p)) U(std::forward<F>(make)());
    }
    template <typename U>
    bool operator==(const lazy_alloc<U>&) const noexcept {
        return true;
    }
};

} // namespace detail

// box_as<T>(make) is a new box that holds the T that make() returns, built in place.
template <typename T, typename F>
ptr<T> box_as(F&& make) {
    return ptr<T>(std::allocate_shared<T>(detail::lazy_alloc<T>{}, std::forward<F>(make)));
}

// box(make) is box_as for the type that make() returns (a variable declared with :=).
template <typename F>
auto box(F&& make) {
    return box_as<std::remove_cvref_t<std::invoke_result_t<F&>>>(std::forward<F>(make));
}

// new_box<T>() is a new box that holds the zero value of T (var x T): T().
template <typename T>
ptr<T> new_box() {
    static_assert(std::is_default_constructible_v<T>,
                  "gocpp: var x T needs a C++ type with a default constructor, because Go's "
                  "zero value must exist: declare it with a value (x := T(...))");
    return gp::new_<T>();
}

// renew(p) gives p's variable a new box with a copy of its value, as each iteration of a 3-clause
// for loop has its own variable in Go (a closure that captures the loop variable keeps the
// iteration's own). The C++ type must be copyable.
template <typename T>
void renew(ptr<T>& p) {
    static_assert(std::is_copy_constructible_v<T>,
                  "gocpp: a closure that uses the variable of a 3-clause for loop needs a copy of "
                  "it for each iteration, and this C++ type can't be copied: declare the object "
                  "before the loop");
    p = box_as<T>([&]() -> T { return *p; });
}

// result_of(x) is the value that a return statement gives back from the boxed variable x: a copy
// (the closures that share the box keep their own object), or, for a type that can't be copied
// (std::unique_ptr), a move, which leaves the moved-from object in the box.
template <typename T>
T result_of(T& x) {
    if constexpr (std::is_copy_constructible_v<T>) {
        return x;
    } else {
        return static_cast<T&&>(x);
    }
}

// defer_arg(x) is an argument of a deferred call that is a C/C++ value: Go evaluates it when the
// defer statement runs and stores it in the defer frame. A variable is copied, which a type that
// can't be copied makes a clear error (std.move(x) moves it instead); a temporary (f(),
// std.move(x)) is moved into the frame, which a type with no move at all (a pinned object) makes a
// clear error too.
template <typename T>
std::decay_t<T> defer_arg(T& x) {
    static_assert(std::is_copy_constructible_v<T>,
                  "gocpp: defer copies the arguments of the deferred call when the defer "
                  "statement runs, and this C++ type can't be copied: pass std.move(x) if it can "
                  "be moved, or a pointer, or write defer func() { ... }()");
    return x;
}
template <typename T>
T&& defer_arg(T&& x) {
    static_assert(std::is_move_constructible_v<std::remove_cvref_t<T>> ||
                      std::is_copy_constructible_v<std::remove_cvref_t<T>>,
                  "gocpp: defer stores the arguments of the deferred call in a frame, and this "
                  "C++ type can be neither moved nor copied: pass a pointer to it, or write defer "
                  "func() { ... }()");
    return static_cast<T&&>(x);
}

// nonnil(p) is the gocpp pointer p of a deferred call's path (defer s.c.inc()), which Go evaluates
// when the defer statement runs: a nil one panics there, not when the deferred call runs.
template <typename P>
P nonnil(P p) {
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) [[unlikely]] {
        panic_nil();
    }
#endif
    return p;
}

// keep(x) is the object of a deferred member call (defer f().close()) as the defer frame stores
// it: a temporary is moved into the frame, and what C++ gave by reference (a function that returns
// T&) is a pointer to it, so the call is made on the object itself, not on a copy. The thunk
// reads it through gp::deref, which takes both.
template <typename X>
auto keep(X&& x) {
    if constexpr (std::is_lvalue_reference_v<X>) {
        return std::addressof(x);
    } else {
        return std::remove_cvref_t<X>(std::forward<X>(x));
    }
}

} // namespace cgo

} // namespace gp
