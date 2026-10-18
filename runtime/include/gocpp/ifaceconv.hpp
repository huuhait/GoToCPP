// gocpp/ifaceconv.hpp: conversions between interface types, type assertions and the helpers of
// type switches (Phase 05 steps 5.4 and 5.5), on the interface values and itabs of
// gocpp/iface.hpp. What generated code writes (docs/spec/cpp-mapping.md, "Interfaces"):
//
//     var r io.Reader = rw                gp::iface_cast<Reader>(rw)
//     var a any = s                       gp::iface_cast<gp::any>(s)
//     v := x.(Rect)                       Rect v = gp::assert_type<Rect>(x, &gp_td_Rect);
//     p := x.(*Circle)                    gp::ptr<Circle> p = gp::assert_type<gp::ptr<Circle>>(x,
//     gp_tdp_Circle); v, ok := x.(Rect)                   auto [v, ok] = gp::assert_type2<Rect>(x,
//     &gp_td_Rect); s := x.(Shape)                      Shape s = gp::assert_iface<Shape>(x); s, ok
//     := x.(Shape)                  gp::assert_iface2<Shape>(x)     (a std::tuple<Shape, bool>)
//     switch v := x.(type) { case A: ...  gp::type_is(x, desc_of_A), gp::type_has<Shape>(x)
//
// A conversion between interface types needs no check at run time, since the checker has
// proved that the dynamic type has the methods; it finds the itab of (target, dynamic type):
// null interface in, null interface out; a target without methods needs no itab. The itab
// comes from a one-entry cache per target type (a static in the template instance, checked
// against the dynamic type), then from the global cache of getitab (a hash and a few loads,
// no lock, no allocation).
//
// A type assertion compares the dynamic type's descriptor with the asserted type's (pointer
// equality is Go type identity, gocpp/type.hpp); a failed one panics with the message of Go's
// *runtime.TypeAssertionError:
//
//     interface conversion: interface {} is string, not int
//     interface conversion: main.J is nil, not main.T                  (x.(T), x of type J)
//     interface conversion: main.T is not main.I: missing method M     (x.(I))
//     interface conversion: interface is nil, not main.I               (x.(I), x nil)
//
// The panic's value is a *runtime.TypeAssertionError whose Error() is that text (recover() gives
// it, detail::throw_type_assertion in gocpp/panic.hpp).
//
// assert_type<T> of a non-pointer type returns a const T& into the box of x: it lives as long as
// x does (a temporary interface: until the end of the full expression); a pointer type returns
// the gp::ptr (sharing the owner), nil for a nil pointer in the interface.
#pragma once

#include <gocpp/iface.hpp>

#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

// The panics. have is the dynamic type of the value (null for a nil interface), want the
// asserted type; from is the static type of the operand (its interface type's descriptor), for
// the assertion to a concrete type.
[[noreturn]] void panic_assert(const type_desc* from, const type_desc* have, const type_desc* want);
[[noreturn]] void panic_assert_iface(const type_desc* want, const type_desc* have);

namespace detail {

// dyn_itab<To>(t) is the itab of (To, t) for a type t that is not nil, or null when t lacks a
// method: the one-entry cache of the template instance, then getitab. To has methods.
template <typename To>
const itab* dyn_itab(const type_desc* t) noexcept {
    // The compilers' own atomic builtins, as in site_itab (iface.hpp). Two one-entry caches: the
    // last type that has the methods (with its itab) and the last that doesn't, so a loop that
    // asks the same question again and again (`if s, ok := x.(Stringer); ok`) never leaves
    // this function.
    static const itab* cache = nullptr;
    static const type_desc* miss = nullptr;
    const itab* c = __atomic_load_n(&cache, __ATOMIC_ACQUIRE);
    if (c != nullptr && c->gp_type == t) {
        return c;
    }
    if (t == __atomic_load_n(&miss, __ATOMIC_ACQUIRE)) {
        return nullptr;
    }
    c = getitab(To::gp_desc(), t);
    if (c != nullptr) {
        __atomic_store_n(&cache, c, __ATOMIC_RELEASE);
    } else {
        __atomic_store_n(&miss, t, __ATOMIC_RELEASE);
    }
    return c;
}

// has_methods<To>(t): the dynamic type t (null: nil) has the methods of the interface To.
template <typename To>
bool has_methods(const type_desc* t) noexcept {
    if (t == nullptr) {
        return false;
    }
    if (To::gp_desc()->gp_nimethods == 0) {
        return true;
    }
    return dyn_itab<To>(t) != nullptr;
}

} // namespace detail

// method_recv(x) is x, the receiver of a deferred or go'ed method call i.M(...) of an interface
// value: Go evaluates the method value when the statement runs, and a nil interface panics there.
template <typename I>
    requires std::is_base_of_v<iface, std::remove_cvref_t<I>>
std::remove_cvref_t<I> method_recv(I&& x) {
    if (x.gp_type() == nullptr) [[unlikely]] {
        panic_nil();
    }
    return std::forward<I>(x);
}

// iface_cast<To>(x): the interface value x converted to the interface type To, whose methods
// are among x's static type's (checked by the compiler). To is an interface struct.
template <typename To>
To iface_cast(const iface& x) {
    static_assert(std::is_base_of_v<iface, To>, "gp::iface_cast: To is an interface struct");
    const type_desc* t = x.gp_type();
    if (t == nullptr) {
        return To();
    }
    const itab* tab = nullptr;
    if (To::gp_desc()->gp_nimethods != 0) {
        tab = detail::dyn_itab<To>(t);
        if (tab == nullptr) [[unlikely]] {
            detail::bad_conversion(To::gp_desc(), t);
        }
    }
    return To(x.gp_data(), t, tab);
}

// The same for an interface value that is not used again: its box is moved, not shared.
template <typename To>
To iface_cast(iface&& x) {
    static_assert(std::is_base_of_v<iface, To>, "gp::iface_cast: To is an interface struct");
    const type_desc* t = x.gp_type();
    if (t == nullptr) {
        return To();
    }
    const itab* tab = nullptr;
    if (To::gp_desc()->gp_nimethods != 0) {
        tab = detail::dyn_itab<To>(t);
        if (tab == nullptr) [[unlikely]] {
            detail::bad_conversion(To::gp_desc(), t);
        }
    }
    return To(x.gp_take(), t, tab);
}

// type_is(x, want): the dynamic type of x is the type with the descriptor want (a case of a
// type switch, x.(T)'s test); false for nil.
inline bool type_is(const iface& x, const type_desc* want) noexcept {
    return x.gp_type() == want;
}

// type_has<To>(x): the dynamic type of x implements the interface type To (a case of a type
// switch with an interface type); false for nil.
template <typename To>
bool type_has(const iface& x) noexcept {
    return detail::has_methods<To>(x.gp_type());
}

// assert_type<T>(x, want): x.(T) for a concrete type T, whose descriptor is want. I is the
// static (interface) type of x.
template <typename T, typename I>
    requires(!detail::is_gp_ptr<T> && std::is_base_of_v<iface, I>)
const T& assert_type(const I& x, const type_desc* want) {
    if (x.gp_type() != want) [[unlikely]] {
        panic_assert(I::gp_desc(), x.gp_type(), want);
    }
    return unbox<T>(static_cast<const iface&>(x));
}

template <typename T, typename I>
    requires(detail::is_gp_ptr<T> && std::is_base_of_v<iface, I>)
T assert_type(const I& x, const type_desc* want) {
    if (x.gp_type() != want) [[unlikely]] {
        panic_assert(I::gp_desc(), x.gp_type(), want);
    }
    return unbox<T>(static_cast<const iface&>(x));
}

// assert_type2<T>(x, want): v, ok := x.(T): the value (T() when it fails) and the flag.
template <typename T, typename I>
    requires std::is_base_of_v<iface, I>
std::tuple<T, bool> assert_type2(const I& x, const type_desc* want) {
    if (x.gp_type() != want) {
        return {T(), false};
    }
    return {unbox<T>(static_cast<const iface&>(x)), true};
}

// assert_iface<To>(x): x.(To) for an interface type To: the dynamic type must have its
// methods. A nil x panics too.
template <typename To, typename I>
    requires std::is_base_of_v<iface, I>
To assert_iface(const I& x) {
    const type_desc* t = x.gp_type();
    const itab* tab = nullptr;
    if (t == nullptr || (To::gp_desc()->gp_nimethods != 0 &&
                         (tab = detail::dyn_itab<To>(t)) == nullptr)) [[unlikely]] {
        panic_assert_iface(To::gp_desc(), t);
    }
    return To(x.gp_data(), t, tab);
}

template <typename To, typename I>
    requires std::is_base_of_v<iface, I>
std::tuple<To, bool> assert_iface2(const I& x) {
    const type_desc* t = x.gp_type();
    const itab* tab = nullptr;
    if (t == nullptr ||
        (To::gp_desc()->gp_nimethods != 0 && (tab = detail::dyn_itab<To>(t)) == nullptr)) {
        return {To(), false};
    }
    return {To(x.gp_data(), t, tab), true};
}

} // namespace gp
