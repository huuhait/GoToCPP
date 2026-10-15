// gocpp/func.hpp: Go's func values. gp::func<R(A...)> is func(A...) R (Phase 03 §2.6, step
// 3.11).
//
// What generated code writes (docs/spec/cpp-mapping.md, "Functions and closures"):
//
//     func(int) string             gp::func<gp::string(int64_t)>    (results: void, T, std::tuple)
//     var f func()                 gp::func<void()> f{};            nil
//     f = g  (g a func)            f = gp::func_of<&gpkg_main::g>(); no allocation
//     f = func() {...}             f = gp::func<void()>([n]<int = 1>() -> void {...});
//     f(x)                         f(x)            f a local that the call can't change
//     f(x)                         f.gp_call(x)    any other f (a package or boxed variable, a
//                                                  field, an element): keeps the closure alive
//     f == nil, f != nil           f == nullptr, f != nullptr
//
// A func value is a pointer to its code and an owner of its closure, like Go's: copies share
// the closure (one atomic increment), so a func value is cheap to pass, store and return, and
// the variables that a closure captured by value are copied once, when the closure is made. A
// closure without captures and a func of the package need no allocation. Calling nil panics
// like Go: "invalid memory address or nil pointer dereference", with the signal line
// (gp::panic_nil).
//
// The layout is 24 bytes (gocppSizes says so, divergence D15; Go's func is 8): the owner, a
// std::shared_ptr<void> whose stored pointer is the closure object (nullptr when there is
// none), and the function that calls it. The owner is dropped through detail::release, so a
// long chain of closures that hold each other (each one capturing the func before it) is freed
// without deep recursion.
//
// Why not std::function: it copies the closure object on every copy (Go shares it), so it
// would copy captured strings and slices each time a func is passed, and its owner can't be
// released through detail::release. It is 32 bytes in libstdc++ too.
//
// A closure that a call can change while it runs (f = nil inside f, with f a package or a boxed
// variable) would free the closure object under its own feet: gp_call holds a second owner for
// the time of the call. operator() doesn't, for the func values that nothing can change during
// the call (a local variable that isn't boxed, a temporary): the emitter chooses (closures.go).
//
// Like every runtime type, func has only operators and gp_-prefixed members (Phase 03 §2.1).
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/ptr.hpp>

#include <cstddef>
#include <memory>
#include <type_traits>
#include <utility>

namespace gp {

template <typename Sig>
class func;

namespace detail {

// fn_tag<F> names the function F of the program for func_of.
template <auto F>
struct fn_tag {};

// fn_sig<R (*)(A...)>::type is R(A...).
template <typename T>
struct fn_sig;
template <typename R, typename... A>
struct fn_sig<R (*)(A...)> {
    using type = R(A...);
};

// func_keep holds an owner of a closure for the time of a call (func::gp_call) and drops it
// through release.
struct func_keep {
    std::shared_ptr<void> gp_env;
    ~func_keep() {
        if (gp_env != nullptr) {
            release(gp_env);
        }
    }
};

} // namespace detail

// func<R(A...)> is a Go func value of the type func(A...) R. The zero value is nil.
template <typename R, typename... A>
class func<R(A...)> {
public:
    // nil.
    constexpr func() noexcept = default;
    constexpr func(std::nullptr_t) noexcept {}

    // A func of the program (gp::func_of<&f>()): F is called directly, no allocation.
    template <R (*F)(A...)>
    explicit func(detail::fn_tag<F>) noexcept : gp_call_(&gp_call_static<F>) {}

    // A closure (a lambda that generated code makes for a func literal), or any other callable
    // with the right parameters and result. A closure without captures is made again for each
    // call (a captureless lambda can be default-constructed), so it needs no allocation; any
    // other closure object goes into a block of its own, which the copies share. Not a named
    // func type over this one (type Handler func(int), a class derived from func: gp::newtype),
    // which is copied as a func, so that a nil one stays nil.
    template <typename F>
        requires(!std::is_base_of_v<func, std::remove_cvref_t<F>> &&
                 !std::is_same_v<std::remove_cvref_t<F>, std::nullptr_t> &&
                 std::is_invocable_r_v<R, std::remove_cvref_t<F>&, A...>)
    func(F&& f) {
        using L = std::remove_cvref_t<F>;
        if constexpr (std::is_empty_v<L> && std::is_default_constructible_v<L>) {
            gp_call_ = &gp_call_empty<L>;
        } else {
            gp_env_ = std::make_shared<L>(std::forward<F>(f));
            gp_call_ = &gp_call_obj<L>;
        }
    }

    // Copies share the closure; a move leaves nil behind.
    func(const func&) noexcept = default;
    func(func&& o) noexcept : gp_env_(std::move(o.gp_env_)), gp_call_(o.gp_call_) {
        o.gp_call_ = nullptr;
    }
    func& operator=(const func&) noexcept = default;
    func& operator=(func&& o) noexcept {
        if (this != &o) {
            gp_env_ = std::move(o.gp_env_);
            gp_call_ = o.gp_call_;
            o.gp_call_ = nullptr;
        }
        return *this;
    }

    // Dropping a func goes through detail::release (long chains of closures, see the top).
    ~func() {
        if (gp_env_ != nullptr) {
            detail::release(gp_env_);
        }
    }

    // f(a...): calls the func, or panics like Go for nil (a nil check: with GOCPP_NOCHECKS,
    // checks.hpp, a call of nil is undefined behavior). For a func value that nothing can
    // change while it runs (see the top of the file).
    R operator()(A... a) const {
        R (*c)(void*, A&&...) = gp_call_;
#ifndef GOCPP_NOCHECKS
        if (c == nullptr) [[unlikely]] {
            panic_nil();
        }
#endif
        return c(gp_env_.get(), static_cast<A&&>(a)...);
    }

    // gp_call(a...): f(a...) for a func value that the call itself could change or drop: the
    // closure stays alive until the call returns.
    R gp_call(A... a) const {
        R (*c)(void*, A&&...) = gp_call_;
#ifndef GOCPP_NOCHECKS
        if (c == nullptr) [[unlikely]] {
            panic_nil();
        }
#endif
        detail::func_keep keep{gp_env_};
        return c(keep.gp_env.get(), static_cast<A&&>(a)...);
    }

    // f == nil. Go compares func values only with nil. (C++ gives f != nullptr and
    // nullptr == f from it.)
    friend bool operator==(const func& f, std::nullptr_t) noexcept { return f.gp_call_ == nullptr; }

private:
    template <R (*F)(A...)>
    static R gp_call_static(void*, A&&... a) {
        return F(static_cast<A&&>(a)...);
    }
    template <typename L>
    static R gp_call_empty(void*, A&&... a) {
        return L{}(static_cast<A&&>(a)...);
    }
    template <typename L>
    static R gp_call_obj(void* env, A&&... a) {
        return (*static_cast<L*>(env))(static_cast<A&&>(a)...);
    }

    std::shared_ptr<void> gp_env_;          // the closure's owner; get() is the closure object
    R (*gp_call_)(void*, A&&...) = nullptr; // nil when null
};

static_assert(sizeof(func<void()>) == 24, "gp::func is 24 bytes: an owner and a function (D15)");

// func_of<&f>(): the func value of the function f of the program, without an allocation.
template <auto F>
func<typename detail::fn_sig<decltype(F)>::type> func_of() noexcept {
    return func<typename detail::fn_sig<decltype(F)>::type>(detail::fn_tag<F>{});
}

} // namespace gp
