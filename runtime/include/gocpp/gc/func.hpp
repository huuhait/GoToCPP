// gocpp/gc/func.hpp: gp::gc::func<Sig>, Go's func value in the GC engine (phase-16 README
// §2.2; job 16.1e).
//
// A Go func value is one pointer to a closure object, 8 bytes, nil for the zero value. The
// closure object lives in the GC heap: its first member is the call function, and the captured
// variables follow. The emitter (step 16.1f) writes
//
//     struct add_closure {
//         gp::gc::closure<std::int64_t(std::int64_t, std::int64_t)> gp_base;
//         std::int64_t addend;
//     };
//     add_closure* c = gp::gc::new_<add_closure>(add_closure{..., 5});
//     gp::gc::func<std::int64_t(std::int64_t, std::int64_t)> f =
//         gp::gc::func<std::int64_t(std::int64_t, std::int64_t)>::from_closure(c);
//
// `gp_base.gp_call` is the function that calls the closure (a static function the emitter
// writes; step 16.8 gives it the Go semantics of closures). The first-member convention is
// what makes from_closure a cast: a closure object that derives from gp::gc::closure<Sig>
// works too, because its base subobject is at offset 0.
//
// Calling a nil func panics with Go's "runtime error: invalid memory address or nil pointer
// dereference". Comparing func values is Go's rule: a func compares only with nil (a func in an
// interface is not comparable and panics, gocpp/gc/iface.hpp).
#pragma once

#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/golayout.hpp>

#include <cstddef>
#include <cstdint>
#include <type_traits>
#include <utility>

namespace gp::gc {

// closure<Sig> is the call-function member at the start of every closure object.
template <typename Sig>
struct closure;

template <typename R, typename... A>
struct closure<R(A...)> {
    R (*gp_call)(void* self, A... args);
};

template <typename Sig>
class func;

template <typename R, typename... A>
class func<R(A...)> {
public:
    // nil.
    constexpr func() noexcept = default;
    constexpr func(std::nullptr_t) noexcept {}

    // from_closure makes a func over a closure object whose first member is a
    // closure<R(A...)> (or that derives from one).
    template <typename C>
    static func from_closure(C* c) noexcept {
        return func(reinterpret_cast<closure<R(A...)>*>(c));
    }

    // f(args...): the call. A nil func panics like Go.
    R operator()(A... args) const {
        if (gp_closure_ == nullptr) [[unlikely]] {
            panic_nil();
        }
        return gp_closure_->gp_call(gp_closure_, std::forward<A>(args)...);
    }

    bool gp_nil() const noexcept { return gp_closure_ == nullptr; }
    void* gp_closure() const noexcept { return gp_closure_; }

    // f == nil, f != nil. Go compares func values only with nil.
    friend bool operator==(const func& f, std::nullptr_t) noexcept {
        return f.gp_closure_ == nullptr;
    }
    friend bool operator!=(const func& f, std::nullptr_t) noexcept {
        return f.gp_closure_ != nullptr;
    }

private:
    explicit constexpr func(closure<R(A...)>* c) noexcept : gp_closure_(c) {}

    closure<R(A...)>* gp_closure_ = nullptr;
};

static_assert(sizeof(func<void()>) == 8, "gp::gc::func must be Go's 8-byte func value");
static_assert(alignof(func<void()>) == 8, "gp::gc::func must be 8-byte aligned");

} // namespace gp::gc
