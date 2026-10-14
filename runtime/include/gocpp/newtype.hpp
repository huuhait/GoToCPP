// gocpp/newtype.hpp: gp::newtype, the C++ of Go's named types that aren't structs (Phase 03
// README §2.4, step 3.9), and gp::convert, the conversions between struct types that Go makes
// without a new value (named and anonymous, type B A).
//
// A named type is a class of its own, so it can have its own methods (step 3.10), its own Go
// name for fmt (gp_type_name) and its own C++ overloads, and C++ doesn't mix it up with its
// underlying type or with another named type over the same type. The emitter writes
//
//     type Celsius float64       struct Celsius : gp::newtype<Celsius, double> {
//                                    using gp::newtype<Celsius, double>::newtype;
//                                    static constexpr const char* gp_type_name = "main.Celsius";
//                                };
//     type IDs []int             struct IDs : gp::newtype<IDs, gp::slice<int64_t>> { ...same... };
//
// gp::newtype<D, U> has two shapes (D is the named type itself, CRTP):
//
//   - Over a number or bool U, it holds the value, gp_v, and has every operator that Go gives
//     the type, each on two D and giving a D (a comparison gives bool): + - * / % & | ^ << >>,
//     unary - ~ !, == != < <= > >=, the compound assignments and ++ --. The result is
//     converted back to U, so the small integer types wrap as in Go (C++ computes them in int);
//     signed overflow wraps with -fwrapv, as everywhere. Go's &^ is a & ~b. gp::div, gp::rem,
//     gp::shl and gp::shr (gocpp/arith.hpp) have overloads for these types below, with Go's
//     panics. A shift count is any integer type (the emitter writes a named count as its
//     value). explicit operator U() makes a named bool a condition (if (ok)).
//   - Over a class U (gp::string, gp::slice, gp::array, gp::map, gp::ptr), it IS a U: a public
//     base class, whose constructors it inherits (nullptr for a nil slice, map or pointer). So
//     every runtime function of U takes a D too (template argument deduction sees the base),
//     and everything that reads a U works on a D: s[i], gp::len, range, gp::get(m, k), *p,
//     p->f. A function that makes a new U (gp::append, gp::sub, gp::make_slice, +) gives a U,
//     which the emitter converts to D where Go's result has the named type: IDs(gp::append(ids,
//     1)). A D converts to U implicitly (a derived-to-base copy), as Go assigns an IDs to a
//     []int; a U converts to D only explicitly, IDs(s), which the emitter writes wherever Go
//     converts implicitly ([]int to IDs, assignability).
//
// Both shapes have the size of U (no member besides U's: TestSizesTable and the static_assert
// after each named type), gp_value() (the U, for fmt's protocol and for conversions, which the
// emitter writes D(x.gp_value()) between named types) and gp_hash() (the hash of the value,
// gocpp/hash.hpp). Like every runtime type, they have only operators and gp_-prefixed members,
// so a Go method of any other name (value, get, len) can't clash with them (step 3.10 adds the
// methods as members of D).
#pragma once

#include <gocpp/arith.hpp>
#include <gocpp/array.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/map.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

template <typename D, typename U, bool = std::is_class_v<U>>
struct newtype;

namespace detail {

// runtime_class<U>: U is one of the runtime's own classes that a named type can be over. Only
// those can be newtype's base class: they have no plain-named members (runtime/members_test.go
// checks them), so a named type has none either.
template <typename T>
inline constexpr bool runtime_class = false;
template <>
inline constexpr bool runtime_class<string> = true;
template <typename T>
inline constexpr bool runtime_class<slice<T>> = true;
template <typename T, int64_t N>
inline constexpr bool runtime_class<array<T, N>> = true;
template <typename K, typename V>
inline constexpr bool runtime_class<map<K, V>> = true;
template <typename T>
inline constexpr bool runtime_class<ptr<T>> = true;

} // namespace detail

// newtype over a class type U: D is a U.
template <typename D, typename U>
    requires detail::runtime_class<U>
struct newtype<D, U, true> : U {
    using gp_underlying = U;

    constexpr newtype() = default;
    using U::U;
    constexpr explicit newtype(const U& v) : U(v) {}
    constexpr explicit newtype(U&& v) noexcept : U(std::move(v)) {}

    constexpr const U& gp_value() const noexcept { return *this; }
    std::size_t gp_hash() const { return gp::hash_of(gp_value()); }
};

// newtype over a number or bool U: the value and Go's operators.
template <typename D, typename U>
struct newtype<D, U, false> {
    using gp_underlying = U;

    U gp_v{};

    constexpr newtype() = default;
    constexpr explicit newtype(U v) noexcept : gp_v(v) {}

    constexpr const U& gp_value() const noexcept { return gp_v; }
    constexpr explicit operator U() const noexcept { return gp_v; }
    std::size_t gp_hash() const { return gp::hash_of(gp_v); }

    friend constexpr D operator+(const D& a, const D& b) noexcept { return D(U(a.gp_v + b.gp_v)); }
    friend constexpr D operator-(const D& a, const D& b) noexcept { return D(U(a.gp_v - b.gp_v)); }
    friend constexpr D operator*(const D& a, const D& b) noexcept { return D(U(a.gp_v * b.gp_v)); }
    // / and % of integers only for a divisor that can't be 0 or -1 (the emitter writes
    // gp::div and gp::rem for the others); / of floats is IEEE's.
    friend constexpr D operator/(const D& a, const D& b) noexcept { return D(U(a.gp_v / b.gp_v)); }
    friend constexpr D operator%(const D& a, const D& b) noexcept { return D(U(a.gp_v % b.gp_v)); }
    friend constexpr D operator&(const D& a, const D& b) noexcept { return D(U(a.gp_v & b.gp_v)); }
    friend constexpr D operator|(const D& a, const D& b) noexcept { return D(U(a.gp_v | b.gp_v)); }
    friend constexpr D operator^(const D& a, const D& b) noexcept { return D(U(a.gp_v ^ b.gp_v)); }
    // << and >> only for a constant count below the width (else gp::shl and gp::shr).
    template <go_int N>
    friend constexpr D operator<<(const D& a, N n) noexcept {
        return D(U(a.gp_v << n));
    }
    template <go_int N>
    friend constexpr D operator>>(const D& a, N n) noexcept {
        return D(U(a.gp_v >> n));
    }
    friend constexpr D operator-(const D& a) noexcept { return D(U(-a.gp_v)); }
    friend constexpr D operator~(const D& a) noexcept { return D(U(~a.gp_v)); }
    friend constexpr D operator!(const D& a) noexcept { return D(!a.gp_v); }

    friend constexpr bool operator==(const D& a, const D& b) noexcept { return a.gp_v == b.gp_v; }
    friend constexpr bool operator<(const D& a, const D& b) noexcept { return a.gp_v < b.gp_v; }
    friend constexpr bool operator<=(const D& a, const D& b) noexcept { return a.gp_v <= b.gp_v; }
    friend constexpr bool operator>(const D& a, const D& b) noexcept { return a.gp_v > b.gp_v; }
    friend constexpr bool operator>=(const D& a, const D& b) noexcept { return a.gp_v >= b.gp_v; }

    friend constexpr D& operator+=(D& a, const D& b) noexcept { return a = a + b; }
    friend constexpr D& operator-=(D& a, const D& b) noexcept { return a = a - b; }
    friend constexpr D& operator*=(D& a, const D& b) noexcept { return a = a * b; }
    friend constexpr D& operator/=(D& a, const D& b) noexcept { return a = a / b; }
    friend constexpr D& operator%=(D& a, const D& b) noexcept { return a = a % b; }
    friend constexpr D& operator&=(D& a, const D& b) noexcept { return a = a & b; }
    friend constexpr D& operator|=(D& a, const D& b) noexcept { return a = a | b; }
    friend constexpr D& operator^=(D& a, const D& b) noexcept { return a = a ^ b; }
    template <go_int N>
    friend constexpr D& operator<<=(D& a, N n) noexcept {
        return a = a << n;
    }
    template <go_int N>
    friend constexpr D& operator>>=(D& a, N n) noexcept {
        return a = a >> n;
    }

    constexpr D& operator++() noexcept {
        gp_v = U(gp_v + 1);
        return static_cast<D&>(*this);
    }
    constexpr D& operator--() noexcept {
        gp_v = U(gp_v - 1);
        return static_cast<D&>(*this);
    }
    constexpr D operator++(int) noexcept {
        D old = static_cast<D&>(*this);
        ++*this;
        return old;
    }
    constexpr D operator--(int) noexcept {
        D old = static_cast<D&>(*this);
        --*this;
        return old;
    }
};

// Go's integer division, remainder and shifts (gocpp/arith.hpp) on named integer types.
template <typename D, go_int U>
D div(const newtype<D, U, false>& a, const newtype<D, U, false>& b) {
    return D(gp::div(a.gp_v, b.gp_v));
}

template <typename D, go_int U>
D rem(const newtype<D, U, false>& a, const newtype<D, U, false>& b) {
    return D(gp::rem(a.gp_v, b.gp_v));
}

template <typename D, go_int U, go_int N>
D shl(const newtype<D, U, false>& x, N n) {
    return D(gp::shl(x.gp_v, n));
}

template <typename D, go_int U, go_int N>
D shr(const newtype<D, U, false>& x, N n) {
    return D(gp::shr(x.gp_v, n));
}

// base_ptr(p): p as a pointer to the underlying type of the named type it points to: a
// *Grid (type Grid [3]int) for gp::sub, gp::elem_ptr and range_array, which take pointers to
// gp::array. It shares p's owner.
template <typename D>
ptr<typename D::gp_underlying> base_ptr(const ptr<D>& p) noexcept {
    using U = typename D::gp_underlying;
    return ptr<U>(p, static_cast<U*>(p.gp_get()));
}

namespace detail {

template <typename T>
concept generated_struct = requires { T::gp_fields; };

template <typename To, typename From, std::size_t... I>
void convert_fields(To& to, const From& from, std::index_sequence<I...>);

template <typename T>
struct convert_ptr : std::false_type {};
template <typename T>
struct convert_ptr<ptr<T>> : std::true_type {
    using elem = T;
};

template <typename T>
struct convert_array : std::false_type {};
template <typename T, int64_t N>
struct convert_array<array<T, N>> : std::true_type {
    using elem = T;
    static constexpr int64_t len = N;
};

} // namespace detail

// convert<To>(x): Go's conversion To(x) between struct types whose underlying types are
// identical (ignoring tags): a named struct type and an anonymous one with the same fields
// (var p Point = struct{ X, Y int }{1, 2}), or type B A. The emitter writes them as two C++
// structs, so the value is copied field by field (in gp_fields' order, the declaration order,
// blank fields too). A field whose types differ only in tags is converted too: a struct
// (struct{ a struct{ x int `t` } }) the same way, an array element by element, and a pointer
// with ptr_cast, so it points to the same struct, as in Go (the two structs have the same
// layout). The emitter reports the other fields that differ (a slice or a map of such structs)
// as not supported yet (named.go: convertible).
template <typename To, typename From>
To convert(const From& x) {
    if constexpr (std::is_same_v<To, From>) {
        return x;
    } else if constexpr (detail::generated_struct<To> && detail::generated_struct<From>) {
        static_assert(std::tuple_size_v<decltype(To::gp_fields)> ==
                          std::tuple_size_v<decltype(From::gp_fields)>,
                      "gp::convert: Go converts only between structs with the same fields");
        To to{};
        detail::convert_fields(
            to, x, std::make_index_sequence<std::tuple_size_v<decltype(To::gp_fields)>>{});
        return to;
    } else if constexpr (detail::convert_ptr<To>::value && detail::convert_ptr<From>::value) {
        return gp::ptr_cast<typename detail::convert_ptr<To>::elem>(x);
    } else if constexpr (detail::convert_array<To>::value && detail::convert_array<From>::value) {
        To to{};
        for (int64_t i = 0; i < detail::convert_array<To>::len; i++) {
            to[i] = gp::convert<typename detail::convert_array<To>::elem>(x[i]);
        }
        return to;
    } else {
        return To(x);
    }
}

namespace detail {

template <typename To, typename From, std::size_t... I>
void convert_fields(To& to, const From& from, std::index_sequence<I...>) {
    ((to.*(std::get<I>(To::gp_fields).gp_member) =
          gp::convert<std::remove_cvref_t<decltype(to.*(std::get<I>(To::gp_fields).gp_member))>>(
              from.*(std::get<I>(From::gp_fields).gp_member))),
     ...);
}

} // namespace detail

} // namespace gp
