// gocpp/iface.hpp: Go's interface values (Phase 05 §2.3, steps 5.2 and 5.3): gp::iface, the
// base of every interface struct, gp::any (interface{}), the boxing of a value into an
// interface (gp::to_iface), the reading of it back (gp::unbox), the call of a method through
// the itab (gp::icall), and the itabs: gp::getitab builds the table of function pointers for
// an (interface type, concrete type) pair by matching the methods by name, package and
// signature text (gocpp/type.hpp), checks every signature, and caches the result in a global
// table that is read without a lock; a conversion site keeps its own itab in a static.
//
// The value: { std::shared_ptr<void> data; const type_desc* type; const itab* tab }, 32 bytes
// (gocppSizes says so, divergence D15). type is the descriptor of the DYNAMIC type, chosen by
// the Go type (gocpp/type.hpp), and null exactly for the nil interface: `x == nil` is
// `type == null`. A pointer *T in an interface is not nil when the pointer is: the type is
// *T, whatever the pointer is (Go's famous gotcha). tab is the itab of (the interface's
// type, type) and is null for an empty interface (any), whose conversions need none.
//
//     nil interface             data null,  type null,  tab null
//     (*T)(nil) in an interface data null,  type *T,    tab ...     != nil
//     *T                        data shares the pointer's owner (an interior pointer too:
//                               the stored address is the pointer's), no copy of the object
//     any other value           data owns a new box that holds a copy of the value (like Go):
//                               numbers, strings, structs, arrays, slices, maps, funcs,
//                               gp::ptr's relatives such as weak pointers, C/C++ values
//
// Copying an interface shares the box (an atomic increment): boxes are never written, as in
// Go. Moving leaves nil behind. The owner is released through gp::detail::release, so freeing
// a long chain of values that hold interfaces that hold values (a list whose next is an
// `any`) never recurses deeply (ptr.hpp).
//
// What generated code writes (docs/spec/cpp-mapping.md, "Interfaces"):
//
//     type Shape interface{ Area() float64; Scale(k float64) }
//
//     struct Shape : gp::iface {                  // one struct per Go interface type; no data
//         using gp::iface::iface;                 // members, so it is 32 bytes (a static_assert)
//         static const gp::type_desc* gp_desc();  // its descriptor, defined after the descriptors
//         double Area() const;                    // the wrappers are declared here and DEFINED
//         void Scale(double k) const;             // inline after all types (header section 7):
//     };                                          // Rect may hold a Shape and Shape return a Rect
//
//     inline double Shape::Area() const { return gp::icall<double()>(*this, 0); }
//     inline void Shape::Scale(double k) const { gp::icall<void(double)>(*this, 1, k); }
//     inline const gp::type_desc* Shape::gp_desc() { return &gp_td_Shape; }
//
// The slot numbers are the positions of the methods in the interface's descriptor (its
// imethods, sorted as gocpp/type.hpp says). A method's wrapper calls icall<Sig>(self, slot,
// args...): Sig is the method's C++ signature, and the thunk in the slot has the type
// R(const gp::iface&, A...): the itab holds, for each interface method, the thunk of the
// concrete type's method of the same name and signature text. The emitter writes one thunk
// per method of each named type: for a value method M of T, in T's table
//
// clang-format off
//     inline double gp_thunk_Rect_Area(const gp::iface& self) { return gp::unbox<Rect>(self).Area(); }
// clang-format on
//
// and in the table of *T (where the interface holds a gp::ptr<Rect>) another that
// dereferences, with Go's nil-pointer wrapper check (gp::check_wrap, "value method main.Rect.Area
// called using nil *Rect pointer"); for a pointer method P of T, only in the table of *T,
//
//     inline void gp_thunk_Rect_Scale(const gp::iface& self, double k) {
//         Rect::Scale(gp::unbox<gp::ptr<Rect>>(self), k);
//     }
//
// and the method table entries, in their sorted order:
//
//     inline const gp::method gp_vm_Rect[] = {
//         gp::method_of<double()>("Area", nullptr, "func() float64", &gp_thunk_Rect_Area), };
//
// A static conversion (a value of the concrete type T assigned to an interface I, which Go
// checks at compile time) is gp::to_iface<I>(value, descriptor): it boxes the value and finds
// the itab, once for each pair (I, C++ type) in a function-local static (re-checked against the
// descriptor, so two Go types over one C++ type, int and int64, never share an itab). At each
// such site the emitter also asserts that every thunk of the type has the C++ type of its
// slot, which go/types can't promise when a method has a C/C++ type (they are all invalid
// for it, and equal to each other):
//
//     static_assert(gp::thunk_matches<double(), decltype(&gp_thunk_Rect_Area)>, "...");
//
// (a compile error that names the line), and the signature TEXTS are compared at run time by
// getitab, so a type whose method says `Get() C.int` never gets the itab of an interface that
// says `Get() C.double`, however the C types are spelled to the checker.
//
// clang-format off
//     any(1)                  gp::to_iface<gp::any>(int64_t(1), &gp::td_int)
//     any(int64(1))           gp::to_iface<gp::any>(int64_t(1), &gp::td_int64)
//     Shape(Rect{2, 3})       gp::to_iface<Shape>(Rect{2, 3}, &gp_td_Rect)
//     Shape(&Circle{1})       gp::to_iface<Shape>(gp::new_<Circle>(Circle{1}),
//                                                 gp::td_ptr<Circle>(&gp_td_Circle))
//     x == nil                x == nullptr
//     nil of interface type   Shape()        (the nullptr constructor is explicit: Shape s = nullptr
//                                             and any z = 0 don't compile)
//     v := x.(Rect)           Rect v = gp::unbox<Rect>(x);    (after the type check: step 5.5)
// clang-format on
//
// A call of a method of a nil interface panics like Go: "invalid memory address or nil
// pointer dereference" (gp::panic_nil). The type assertions, type switches and conversions between
// interface types are gocpp/ifaceconv.hpp's (steps 5.4 and 5.5), built on type(), tab(), getitab
// and missing_method; the == and the hash of interface values are here, defined in
// runtime/src/ifaceconv.cpp.
//
// Like every runtime type, iface and any have only constructors, operators and gp_-prefixed
// members, so a Go method of any name can be a member of an interface struct.
#pragma once

#include <gocpp/aware.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/type.hpp>

#include <cstddef>
#include <memory>
#include <type_traits>
#include <utility>

namespace gp {

// itab is the table of an (interface type, concrete type) pair: gp_fun[i] is the thunk of
// the method i of the interface (cast it to its real type, thunk_t<Sig>*, to call it). An
// itab that is built is immutable and never freed. A pair whose type lacks a method has a
// negative itab (gp_missing is the interface's first method that the type doesn't have, and
// gp_fun is null): getitab returns null for it, and the cache keeps it, so the failure is not
// searched again.
struct itab {
    const type_desc* gp_inter;
    const type_desc* gp_type;
    const imethod* gp_missing;
    const fn_ptr* gp_fun;
    const bool* gp_aware; // gp_aware[i]: the method i contains recover() (method_of_aware)
};

// getitab(inter, type) is the itab of type for the interface type inter, built the first time
// (the methods are matched by name, package and signature text) and cached, or null when type
// has no method set that implements inter. inter must have at least one method (the empty
// interface needs no itab: any non-nil type implements it) and type must be a concrete type.
// Thread safe: a hit takes no lock. A fatal error when a method table is not sorted.
const itab* getitab(const type_desc* inter, const type_desc* type) noexcept;

// missing_method(inter, type) is the first method of inter that type doesn't have (with the
// same signature), for Go's "missing method M"; null when type implements inter.
const imethod* missing_method(const type_desc* inter, const type_desc* type) noexcept;

// implements(inter, type): type implements the interface type inter (true for an empty one).
bool implements(const type_desc* inter, const type_desc* type) noexcept;

namespace detail {

// The number of itabs that have been built, and are cached (positive and negative), since
// the program began (for tests of the cache: a hit builds none).
std::size_t itab_builds() noexcept;
std::size_t itab_cached() noexcept;

// itab_entry(inter, type) is the cache's entry for the pair, positive or negative (a negative
// one has gp_missing set and no gp_fun): getitab without the null. For tests of the cache.
const itab* itab_entry(const type_desc* inter, const type_desc* type) noexcept;

// bad_conversion ends the program: a static conversion that the checker allowed but whose type
// lacks a method, a bug in the emitter (the static_asserts catch it at compile time).
[[noreturn]] void bad_conversion(const type_desc* inter, const type_desc* type) noexcept;

// bad_unbox: a nil interface unboxed (the type check comes first, in generated code).
[[noreturn]] void bad_unbox() noexcept;

} // namespace detail

// iface is the interface value. See the top of the file.
class iface {
public:
    // The nil interface.
    constexpr iface() noexcept = default;
    constexpr explicit iface(std::nullptr_t) noexcept {}

    // An interface value of this data, dynamic type and itab, for gp::to_iface and the
    // conversions.
    iface(std::shared_ptr<void> data, const type_desc* type, const itab* tab) noexcept
        : gp_data_(std::move(data)), gp_type_(type), gp_tab_(tab) {}

    // Copies share the box; a move leaves nil behind.
    iface(const iface&) noexcept = default;
    iface(iface&& o) noexcept
        : gp_data_(std::move(o.gp_data_)), gp_type_(o.gp_type_), gp_tab_(o.gp_tab_) {
        o.gp_type_ = nullptr;
        o.gp_tab_ = nullptr;
    }
    iface& operator=(const iface&) noexcept = default;
    iface& operator=(iface&& o) noexcept {
        if (this != &o) {
            gp_data_ = std::move(o.gp_data_);
            gp_type_ = o.gp_type_;
            gp_tab_ = o.gp_tab_;
            o.gp_type_ = nullptr;
            o.gp_tab_ = nullptr;
        }
        return *this;
    }

    // Dropping an interface goes through detail::release (a chain of values that hold
    // interfaces is freed without deep recursion).
    ~iface() {
        if (gp_data_ != nullptr) {
            detail::release(gp_data_);
        }
    }

    // x == nil, x != nil.
    friend bool operator==(const iface& x, std::nullptr_t) noexcept {
        return x.gp_type_ == nullptr;
    }

    // x == y: Go's comparison of two interface values (any two interface types: the operands
    // of Go's == are converted to one of them first, which changes nothing here): equal when
    // both are nil, or have the same dynamic type and equal values. A dynamic type that is not
    // comparable (a slice, a map, a func, a struct that holds one) panics when both have it:
    // "runtime error: comparing uncomparable type []int". (The nil interface and a pointer
    // inside an interface behave as Go's: a nil *T in an interface is not nil.)
    friend bool operator==(const iface& x, const iface& y);

    // gp_hash: the hash of the dynamic value, for map keys (map[any]V) and for the structs that
    // hold an interface; the nil interface has a hash of its own. A dynamic type that has no
    // hash (a slice, a map, a func) panics: "runtime error: hash of unhashable type []int".
    std::size_t gp_hash() const;

    // gp_hash_panics: hashing an interface value can panic (gocpp/hash.hpp: hash_may_panic).
    static constexpr bool gp_hash_panics = true;

    // gp_type: the descriptor of the dynamic type (null for nil). gp_tab: the itab (null for
    // nil and for an empty interface). gp_data: the box, or the shared pointer.
    const type_desc* gp_type() const noexcept { return gp_type_; }
    const itab* gp_tab() const noexcept { return gp_tab_; }
    const std::shared_ptr<void>& gp_data() const noexcept { return gp_data_; }

    // gp_take: the owner of the box, moved out (x is nil afterwards), for the conversion of an
    // interface value that is not used again to another interface type (gocpp/ifaceconv.hpp).
    std::shared_ptr<void> gp_take() noexcept {
        gp_type_ = nullptr;
        gp_tab_ = nullptr;
        return std::move(gp_data_);
    }

private:
    std::shared_ptr<void> gp_data_;
    const type_desc* gp_type_ = nullptr;
    const itab* gp_tab_ = nullptr;
};

static_assert(sizeof(iface) == detail::iface_size && alignof(iface) == detail::iface_align,
              "gp::iface is 32 bytes: the data, the type and the itab (D15)");

// any is interface{}: an interface without methods, so without an itab (Go's own runtime calls
// the type eface, which is kept as an alias). Every interface value converts to it (the
// conversion drops the itab); it converts back only through a type assertion (steps 5.4 and
// 5.5).
class any : public iface {
public:
    using iface::iface;
    constexpr any() noexcept = default;

    // Any interface value to any: the same value without the itab.
    any(const iface& x) noexcept : iface(x.gp_data(), x.gp_type(), nullptr) {}

    // The descriptor of the interface type (interface {}), for gp::to_iface.
    static const type_desc* gp_desc() noexcept { return &td_any; }
};

using eface = any;

static_assert(sizeof(any) == sizeof(iface), "gp::any has no members of its own");

// to_iface<I>(v, td) is the interface value of the interface type I that holds v, whose Go
// type has the descriptor td: Go's conversion of a concrete value to an interface (the
// static ones, which the checker has accepted). v is copied into a new box, except a
// gp::ptr, which shares the pointer's owner. I is a struct of the shape above (it needs
// gp_desc()); for an interface with methods the itab of (I, td) is found once for each pair
// (I, C++ type) and kept (a descriptor that differs from the last one looks it up again). A
// type that lacks a method is a fatal error (the emitter's static_asserts say so first).
// Interface values are not converted this way (they are not boxed: step 5.4).
namespace detail {

template <typename I, typename D>
const itab* site_itab(const type_desc* td) {
    // The compilers' own atomic builtins, not std::atomic: <atomic> would add its macros to
    // every generated file (cxx_macros.txt), and this is one pointer.
    static const itab* cache = nullptr;
    const itab* t = __atomic_load_n(&cache, __ATOMIC_ACQUIRE);
    if (t != nullptr && t->gp_type == td) {
        return t;
    }
    t = getitab(I::gp_desc(), td);
    if (t == nullptr) [[unlikely]] {
        bad_conversion(I::gp_desc(), td);
    }
    __atomic_store_n(&cache, t, __ATOMIC_RELEASE);
    return t;
}

} // namespace detail

template <typename I, typename T>
I to_iface(T&& v, const type_desc* td) {
    using D = std::remove_cv_t<std::remove_reference_t<T>>;
    static_assert(!std::is_base_of_v<iface, D>,
                  "gp::to_iface: an interface value is converted to another interface type, "
                  "not boxed (step 5.4)");
    static_assert(std::is_base_of_v<iface, I>, "gp::to_iface: I is an interface struct");
    std::shared_ptr<void> data;
    if constexpr (detail::is_gp_ptr<D>) {
        // Shares the owner of the pointer (an interior pointer's too, with its address).
        if constexpr (std::is_lvalue_reference_v<T> ||
                      std::is_const_v<std::remove_reference_t<T>>) {
            data = v.gp_shared();
        } else {
            data = std::move(v).gp_take();
        }
    } else {
        data = std::make_shared<D>(std::forward<T>(v));
    }
    const itab* tab = nullptr;
    if (I::gp_desc()->gp_nimethods != 0) {
        tab = detail::site_itab<I, D>(td);
    }
    return I(std::move(data), td, tab);
}

// to_iface_foreign<I>(v) is to_iface for a C/C++ value, whose Go type the checker doesn't know: the
// descriptor is foreign_td<T>() of its C++ type, named after the C++ spelling. (Two C++ types that
// are one Go type, as int and int64 are one C++ type in the other direction, don't exist: the C++
// type is the identity.) A raw pointer T* (what &cvar and the results of C functions are) is held
// as the gp::cptr<T> that a gocpp variable of the Go type *C.T is, so x.(*C.T) finds it whichever
// way it was made; a cptr made from a raw address owns nothing (the object lives as long as the C
// variable or whatever else owns it: the pointer to a local C variable is valid while the variable
// is).
template <typename I, typename T>
I to_iface_foreign(T&& v) {
    using D = std::remove_cv_t<std::remove_reference_t<T>>;
    if constexpr (std::is_pointer_v<D> && !std::is_void_v<std::remove_pointer_t<D>> &&
                  !std::is_function_v<std::remove_pointer_t<D>>) {
        // (Go has no const: the pointees are normalised, as foreign_td does for the assertion.)
        using N = detail::foreign_norm_t<D>;
        using E = std::remove_pointer_t<N>;
        if constexpr (std::is_pointer_v<E>) {
            // A T** (an out-parameter, an array of strings): the raw pointer, which is what the
            // Go type **T is in C++ (x.(**T) asks for it).
            return to_iface<I>(const_cast<N>(v), foreign_td<N>());
        } else {
            return to_iface<I>(cptr<E>(const_cast<E*>(v)), foreign_td<cptr<E>>());
        }
    } else {
        return to_iface<I>(std::forward<T>(v), foreign_td<T>());
    }
}

// unbox<T>(x): the value that x holds, whose dynamic type is the Go type of T (checked by the
// caller: a type assertion compares the descriptor first, step 5.5). A boxed value is a
// const T& into the box of x, so it lives as long as x does (an unbox of a temporary is a
// copy); a gp::ptr<U> is a new pointer that shares the owner, nil for a nil pointer inside
// the interface. Unboxing the nil interface is a fatal error (with GOCPP_NOCHECKS: undefined
// behavior).
template <typename T>
    requires(!detail::is_gp_ptr<T>)
const T& unbox(const iface& x) {
#ifndef GOCPP_NOCHECKS
    if (x.gp_data() == nullptr) [[unlikely]] {
        detail::bad_unbox();
    }
#endif
    return *static_cast<const T*>(x.gp_data().get());
}

template <typename T>
    requires(!detail::is_gp_ptr<T>)
T unbox(iface&& x) {
    return unbox<T>(static_cast<const iface&>(x));
}

template <typename T>
    requires detail::is_gp_ptr<T>
T unbox(const iface& x) {
    using U = typename detail::ptr_elem<T>::type;
    if (x.gp_data() == nullptr) {
        return T();
    }
    return T(x.gp_data(), static_cast<U*>(x.gp_data().get()));
}

// unbox_borrow<T>(x): unbox<gp::ptr<T>>(x) as a borrowed pointer, for the thunk of a method whose
// receiver is borrowed (ptr.hpp: bptr): no copy of the pointer. x stays alive while the thunk
// runs (icall's contract: a stable interface value, or icall_keep's copy), and it holds the
// object. Nil for a nil pointer inside the interface.
template <typename T>
bptr<T> unbox_borrow(const iface& x) noexcept {
    return bptr<T>(static_cast<T*>(x.gp_data().get()));
}

namespace detail {

template <typename Sig>
struct icall_fn;
template <typename R, typename... A>
struct icall_fn<R(A...)> {
    template <typename... B>
    static R call(const iface& self, std::size_t slot, B&&... b) {
        const itab* t = self.gp_tab();
#ifndef GOCPP_NOCHECKS
        if (t == nullptr) [[unlikely]] {
            panic_nil(); // a method of a nil interface (a nil check: checks.hpp)
        }
#endif
        auto f = reinterpret_cast<thunk_t<R(A...)>*>(t->gp_fun[slot]);
        // The ticket of a deferred call (gocpp/aware.hpp): one thread-local load, null unless
        // the call is the one a panic's deferred-call machinery makes.
        detail::icall_ticket(*t, slot);
        return f(self, static_cast<B&&>(b)...);
    }
};

} // namespace detail

// icall<Sig>(self, slot, args...) calls the method in slot `slot` of the itab of self: Sig is
// its C++ signature, R(A...), and the thunk there is R(const iface&, A...). A nil interface
// panics (gp::panic_nil). For an interface value that nothing can change while the method
// runs (a local variable that isn't boxed, a temporary).
template <typename Sig, typename... B>
decltype(auto) icall(const iface& self, std::size_t slot, B&&... args) {
    return detail::icall_fn<Sig>::call(self, slot, static_cast<B&&>(args)...);
}

// icall_keep is icall for an interface value that the method itself could change or drop (a
// package or boxed variable, a field, an element: `s.f.M()` where M assigns s.f): it holds
// a copy for the time of the call, so the box lives until the call returns. The emitter chooses,
// as for func values (func.hpp).
template <typename Sig, typename... B>
decltype(auto) icall_keep(const iface& self, std::size_t slot, B&&... args) {
    const iface keep = self;
    return detail::icall_fn<Sig>::call(keep, slot, static_cast<B&&>(args)...);
}

} // namespace gp
