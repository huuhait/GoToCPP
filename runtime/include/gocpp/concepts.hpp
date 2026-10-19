// gocpp/concepts.hpp: the building blocks of the C++20 concepts that `gocpp -gp:concepts` writes
// into pkg.hpp for Go's constraints (Phase 07 step 7.9, docs/spec/cpp-mapping.md "Generics").
// Generated code never includes it (rt.hpp does not): a pkg.hpp includes it when it has concepts.
//
// A Go constraint is a type set (`~int | ~float64`, `comparable`, `~[]E`) and a method set. The
// concepts say, for a C++ type T, whether it is in that set, as far as C++ can tell:
//
//     tilde<T, U>   Go's ~U: T is U itself or a defined type over U (gp::newtype's gp_underlying)
//     exact<T, U>   Go's U without the tilde: T is U
//     comparable<T> Go's comparable: T has == (the runtime's own notion; slices, maps and funcs
//     have
//                   it only against nil, structs only when all fields do)
//
// and the methods: `requires(T& x, P a) { { x.M(a) } -> std::same_as<R>; }` (a value method, also
// a promoted one and an interface's), `x->M(a)` (a value method through a gp::ptr) or
// `ptr_elem_t<T>::M(x, a)` (a pointer method of the pointee: it is a static function that takes
// the pointer first).
//
// What C++ cannot tell: `int` and `int64` are both int64_t, `uint8` and `byte` both uint8_t, so
// `~int` accepts a defined type over int64 too; two defined types over one struct type are two
// structs; a Go type argument that is a C++ type cannot be a named C++ type of the same layout.
// So a concept accepts every type that Go's constraint accepts and, rarely, one more (the
// docs/spec/cpp-mapping.md "Generics" has the list). Nothing in the generated code depends on a
// concept.
#pragma once

#include <gocpp/newtype.hpp>
#include <gocpp/ptr.hpp>

#include <concepts>
#include <type_traits>

namespace gp::concepts {

// underlying_t<T> is the C++ type under a defined type: gp::newtype<D, U> says U, any other type
// (int64_t, gp::string, a struct) is its own.
template <typename T>
struct underlying {
    using type = T;
};
template <typename T>
    requires requires { typename T::gp_underlying; }
struct underlying<T> {
    using type = typename T::gp_underlying;
};
template <typename T>
using underlying_t = typename underlying<T>::type;

// Go's ~U and U in a type set.
template <typename T, typename U>
concept tilde = std::same_as<underlying_t<T>, U>;
template <typename T, typename U>
concept exact = std::same_as<T, U>;

// Go's comparable.
template <typename T>
concept comparable = std::equality_comparable<T>;

// ptr_elem_t<gp::ptr<X>> is X; it has no member type for any other T (a requirement that names it
// then fails to hold).
template <typename T>
struct ptr_elem {};
template <typename X>
struct ptr_elem<gp::ptr<X>> {
    using type = X;
};
template <typename T>
using ptr_elem_t = typename ptr_elem<T>::type;

// is_ptr<T> holds for gp::ptr<X> itself, not for a defined pointer type (`type NPS *S`, a
// gp::newtype over a gp::ptr, whose method set in Go is empty).
template <typename T>
concept is_ptr = requires { typename ptr_elem<T>::type; };

} // namespace gp::concepts
