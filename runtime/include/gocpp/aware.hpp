// gocpp/aware.hpp: how recover() finds out that it was called directly by a deferred call
// (step 5.10). Go's rule: recover stops a panic only when the function that calls it is the
// function that the panic's defer machinery called, not a function that one of its callees
// calls ("recover returns nil if it was not called directly by a deferred function"). The
// function may have been reached any way: by name, as a func value, as a method value, through
// an interface, or as the result of a call (`defer mk()()`).
//
// gocpp's way (a deviation from the phase README's "two entry points" in how, not in what): a
// function that contains recover() is an *aware* function. Its first action is
//
//     gp::defer_ctx gp_dc = gp::take_defer_ctx();
//
// and its recover() is gp::recover(gp_dc). The ticket that take_defer_ctx takes is set by the
// defer machinery (defer_frame::gp_run) right before it calls a deferred call whose function it
// knows to be aware, and only while that frame is running its calls for a panic (the ticket is
// the run_mark of the gp_run: the panic it runs). So a normal call of an aware function finds no
// ticket and recover() returns nil; a call from the machinery finds it, takes it, and nothing
// that the function calls can find it again. The machinery clears the ticket when the call ends.
// "Knows to be aware" is:
//   - a function or a method of the program that the emitter knows contains recover: it
//     writes gp_push_aware (a function) or gp_push_call_aware (a lambda), the static case;
//   - a func value (gp::func): its code pointer is looked up in the registry below, where an
//     aware function value's code is entered when the func is made from an aware lambda
//     (gp::aware), an aware function (gp::func_of_aware) or an aware method value;
//   - an interface method: the call goes through gp::icall, which looks at the ticket (one
//     thread-local load, unlikely to be set) and clears it unless the method's entry in the itab
//     has the aware flag (gp::method_of_aware sets it in the method table).
//
// Every other function (the vast majority) knows nothing of this and pays nothing.
#pragma once

#include <cstddef>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {
struct run_mark; // gocpp/trace.hpp
} // namespace detail

// defer_ctx is what an aware function takes at its start: the run of the deferred calls that
// called it for a panic (null: it was called normally, or the frame isn't panicking).
using defer_ctx = const detail::run_mark*;

namespace detail {

// The ticket of this thread (see above).
inline constinit thread_local defer_ctx ticket = nullptr;

// aware_code(p) reports whether the code at p (the code pointer of a func value) belongs to an
// aware function; mark_aware(p) enters it. The registry is an append-only hash table that is read
// without a lock; it is read only while a panic runs deferred calls (the ticket is set only then)
// and written when an aware func value is first made. aware_lookups counts the reads (for tests:
// a deferred call that returns normally must not read it).
std::size_t aware_lookups() noexcept;
using code_ptr = void (*)();
bool aware_code(code_ptr p) noexcept;
void mark_aware(code_ptr p) noexcept;

// icall_ticket is gp::icall's look at the ticket (iface.hpp): the interface method in slot of the
// itab t about to be called takes the ticket if its table entry says it is aware
// (gp::method_of_aware); otherwise the ticket is cleared, so that nothing the method calls can use
// it. No registry: the flag is in the itab.
template <typename Itab>
inline void icall_ticket(const Itab& t, std::size_t slot) noexcept {
    if (ticket != nullptr && !t.gp_aware[slot]) {
        ticket = nullptr;
    }
}

// aware_lambda wraps the lambda that is the code of an aware func literal, so that the func made
// from it (gp::func's constructor) is known to be aware. It only forwards.
template <typename L>
struct aware_lambda {
    L gp_fn;
    template <typename... B>
    decltype(auto) operator()(B&&... b) {
        return gp_fn(static_cast<B&&>(b)...);
    }
};

template <typename T>
inline constexpr bool is_aware_lambda = false;
template <typename L>
inline constexpr bool is_aware_lambda<aware_lambda<L>> = true;

} // namespace detail

// take_defer_ctx is the first statement of an aware function.
inline defer_ctx take_defer_ctx() noexcept {
    defer_ctx c = detail::ticket;
    detail::ticket = nullptr;
    return c;
}

// aware(lambda) marks a func literal that contains recover():
// `gp::func<void()>(gp::aware([]{...}))`. A deferred func literal that is called at once (`defer
// func() { ... }()`) needs no mark: the emitter writes gp_push_call_aware for it.
template <typename L>
detail::aware_lambda<std::remove_cvref_t<L>> aware(L&& fn) {
    return {static_cast<L&&>(fn)};
}

} // namespace gp
