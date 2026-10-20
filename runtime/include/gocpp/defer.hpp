// gocpp/defer.hpp: Go's defer, gp::defer_frame (Phase 03 §2.8, Phase 05 step 5.10).
//
// A function that has a defer statement gets one defer_frame and one way out (the single-exit
// shape, docs/spec/cpp-mapping.md "Defer"):
//
//     int64_t f(int64_t n) {
//         int64_t gp_r0{};                          // the results
//         gp::defer_frame gp_df; try {
//             gp_df.gp_push(g, n);                  // defer g(n): g and n evaluated now
//             gp_r0 = n; goto gp_ret0;              // return n
//         } catch (gp::panic_error& gp_p) {
//             gp_df.gp_take(gp_p);                  // a Go panic
//         } catch (...) {
//             gp_df.gp_adopt();                     // Goexit, or a C++ exception
//         }
//         gp_df.gp_run(); return gp_r0;             // the end of the body, or a panic
//         gp_ret0:; gp_df.gp_run(); return gp_r0;   // one exit per return, at its line
//     }
//
// gp_push stores the deferred call: the function and its arguments, evaluated at the defer
// statement as Go says (the arguments of a function pointer are converted to its parameter
// types there). gp_run runs the stored calls, the last one first, when the function returns
// (after the results are set, so a deferred call that changes a named result through a pointer
// changes what the function returns) and when a panic leaves the body (gp_take keeps it,
// and gp_run throws it again after the last deferred call, unless a deferred call recovered it).
// A panic statement in the function itself doesn't throw: gp_panic takes it and a goto leaves the
// try block (gp_panic, below).
// A deferred call that panics doesn't stop the others: its panic replaces the running one,
// which goes into the new one's chain (panic_error::gp_prev), and Go prints the whole chain
// ("panic: A" then "\tpanic: B").
//
// recover() (step 5.10): a deferred call that contains recover() is aware (gocpp/aware.hpp):
// it takes the ticket that gp_run sets for it while the frame runs its calls for a panic, and
// gp::recover(ctx) stops that panic: it returns the panic's value (a gp::any) and marks the
// panic recovered, and when the call returns, gp_run drops the panic, runs the other deferred
// calls as for a normal return, and the function returns normally with the results as they are
// (the deferred call may have changed named results). A panic that is recovered but is still
// in a chain (the deferred call that recovered it panicked again) prints "[recovered]".
//
// What a frame catches (gp_on_exception, and a deferred call that throws):
//   - a Go panic (gp::panic_error), caught by its type (gp_take): it runs the deferred calls, and
//     recover() can stop it;
//   - runtime.Goexit (gp::goexit_signal): it runs the deferred calls too, recover() returns nil,
//     and it is thrown again at the end. A Goexit in a deferred call replaces the panic that
//     was running (Go does that too);
//   - a forced unwind (glibc's pthread_exit or pthread_cancel, which C code may call from Phase 04
//     on): it must not be caught and kept (std::current_exception gives a null pointer for it,
//     and glibc aborts when a handler doesn't throw it again), and running Go code during it isn't
//     safe, so it is thrown again at once, untouched; the frame's destructor frees the calls
//     that didn't run, without running them;
//   - any other C++ exception (a std::exception of a C++ library, `throw 42`): it becomes a Go
//     panic whose value is a gp::CppException (panic.hpp), so the deferred calls run and recover()
//     can stop it. std::bad_alloc is not one: running out of memory is Go's fatal error.
//
// The calls are kept in a small buffer inside the frame (gp_inline_size bytes, one call with
// its arguments, without an allocation); more, or bigger ones, go to the heap. A frame is for
// one call of one function, on one thread: it is not copied or moved.
#pragma once

#include <gocpp/aware.hpp>
#include <gocpp/func.hpp>
#include <gocpp/panic.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/trace.hpp>

#include <cstddef>
#include <cstdint>
#include <exception>
#include <memory>
#include <new>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {

// is_gp_func<T>: T is a gp::func or a type derived from one (a named func type, `type H func()`,
// is a gp::newtype over a gp::func): a func value.
template <typename Sig>
std::true_type func_probe(const gp::func<Sig>*);
std::false_type func_probe(...);
template <typename T>
inline constexpr bool is_gp_func = decltype(func_probe(static_cast<T*>(nullptr)))::value;

} // namespace detail

class defer_frame {
public:
    // The frame is listed in the thread's defer frames (detail::top_defer) while it lives, so that
    // a panic knows how much of its stack the frame's function is going to unwind (panic.hpp).
    defer_frame() noexcept {
        if constexpr (detail::defer_links_listed) {
            gp_link.gp_set(detail::top_defer);
            detail::top_defer = &gp_link;
        }
    }
    defer_frame(const defer_frame&) = delete;
    defer_frame& operator=(const defer_frame&) = delete;
    // Frees the calls that never ran (only when the function didn't end through gp_run).
    ~defer_frame() {
        if constexpr (detail::defer_links_listed) {
            detail::top_defer = gp_link.gp_next();
        }
        if (gp_top != nullptr || gp_local) {
            gp_discard();
        }
    }

    // defer f(a...) for a function f: stores f and the arguments, converted to f's parameter
    // types here, at the defer statement (a parameter that is a const reference, for a value
    // larger than 128 KiB, stores a copy). A borrowed pointer parameter (gp::bptr<T>, ADR-086)
    // stores the owning gp::ptr<T>: the frame keeps the object alive.
    template <typename R, typename... P>
    void gp_push(R (*f)(P...), std::type_identity_t<detail::owning_t<P>>... a) {
        gp_emplace<gp_node_of<gp_plain, R (*)(P...), std::decay_t<detail::owning_t<P>>...>>(
            f, static_cast<detail::owning_t<P>&&>(a)...);
    }

    // The same for a function that contains recover() (an aware function, gocpp/aware.hpp): the
    // frame gives it the ticket when it runs it for a panic.
    template <typename R, typename... P>
    void gp_push_aware(R (*f)(P...), std::type_identity_t<P>... a) {
        gp_emplace<gp_node_of<gp_aware, R (*)(P...), std::decay_t<P>...>>(f,
                                                                          static_cast<P&&>(a)...);
    }

    // defer c(a...) for any other callable c (gocpp's generic lambda for a builtin or a
    // runtime template: [](auto&& gp_a0) { gp::println(gp_a0); }): stores c and copies of the
    // arguments, which c gets as rvalues when it runs.
    // When c is a func value (gp::func), the frame gives the ticket if its code is aware
    // (gocpp/aware.hpp).
    template <typename F, typename... A>
    void gp_push_call(F c, A&&... a) {
        gp_emplace<gp_node_of<detail::is_gp_func<F> ? gp_func : gp_plain, F, std::decay_t<A>...>>(
            std::move(c), std::forward<A>(a)...);
    }

    // gp_push_call for a callable that contains recover() (a deferred func literal): the frame
    // gives it the ticket.
    template <typename F, typename... A>
    void gp_push_call_aware(F c, A&&... a) {
        gp_emplace<gp_node_of<gp_aware, F, std::decay_t<A>...>>(std::move(c),
                                                                std::forward<A>(a)...);
    }

    // gp_push_call for a callable whose call is a method call through an interface
    // (`defer i.M()`): gp::icall gives M the ticket if M is aware.
    template <typename F, typename... A>
    void gp_push_call_icall(F c, A&&... a) {
        gp_emplace<gp_node_of<gp_icall, F, std::decay_t<A>...>>(std::move(c),
                                                                std::forward<A>(a)...);
    }

    // gp_hold(owner) makes the frame keep owner (the box of a C/C++ local whose address the
    // function takes, Phase 04 step 4.8) until every deferred call has run, whenever they were
    // registered. A hold is a node on top of the stack, like a call (so it costs O(1)), which
    // gp_run doesn't call: it sets it aside and releases it after the last call ran, also when
    // a panic is running. It is on the heap. A pointer (gp::ptr) or a std::shared_ptr works.
    template <typename T>
    void gp_hold(T owner) {
        std::shared_ptr<const void> sp;
        if constexpr (requires { owner.gp_shared(); }) {
            sp = std::shared_ptr<const void>(owner.gp_shared());
        } else {
            sp = std::shared_ptr<const void>(std::move(owner));
        }
        auto* n = new gp_hold_node;
        n->gp_op = &gp_hold_op;
        n->gp_owner = std::move(sp);
        n->gp_next = gp_top;
        gp_top = n;
    }

    // gp_on_exception keeps the exception that left the function's body, for gp_run: a Go panic
    // as it is, a C++ exception as a panic with a gp::CppException value, Goexit as a flag. A
    // forced unwind is thrown again at once. Called in the function's catch (...).
    void gp_on_exception();

    // The two halves of gp_on_exception, for the generated code (a typed handler first, so that a
    // Go panic costs no second throw to find out its type):
    //
    //     } catch (gp::panic_error& gp_p) { gp_df.gp_take(gp_p); }
    //       catch (...) { gp_df.gp_adopt(); }
    //
    // gp_take keeps the Go panic p, which is the exception being handled; gp_adopt keeps any other
    // exception being handled (Goexit, a C++ exception, a forced unwind).
    void gp_take(panic_error& p);
    void gp_adopt();

    // gp_panic is a panic statement in the function that has this frame: the generated code is
    // `gp_df.gp_panic(gp::new_panic(v)); goto gp_pan0;`, where the label is the function's exit
    // that runs the deferred calls (after the catch handlers). It does what the throw and the
    // handler do (gp_take) without the throw: the panic goes to the frame at once, and the
    // function's locals are destroyed by the goto, in the order that the unwinding would. The
    // frame owns the panic, and throws it from gp_run if no deferred call recovers it.
    void gp_panic(std::unique_ptr<panic_error> p);

    // gp_run runs the deferred calls, the last one first, then throws the panic that is still
    // running, if any: the one of the body, or the last one that a deferred call started (or
    // goexit_signal when Goexit is running). A forced unwind leaves it at once (see above).
    void gp_run();

private:
    // A stored call: gp_op(n, gp_call) makes the call, gp_op(n, gp_destroy) destroys the node
    // (one in gp_buf), gp_op(n, gp_delete) destroys and frees it (one on the heap).
    enum gp_what { gp_call, gp_destroy, gp_delete };
    struct gp_node {
        gp_node* gp_next = nullptr; // the call deferred before this one
        void (*gp_op)(gp_node*, gp_what) = nullptr;
    };

    // A hold (gp_hold): the owner it keeps. Its gp_op is gp_hold_op, by which gp_run knows it.
    struct gp_hold_node : gp_node {
        std::shared_ptr<const void> gp_owner;
    };
    static void gp_hold_op(gp_node* n, gp_what what) {
        if (what != gp_call) {
            delete static_cast<gp_hold_node*>(n);
        }
    }

    // How a deferred call is made (the template argument of gp_node_of): plain; aware (the
    // function contains recover(), the frame gives it the ticket); func (a gp::func: the ticket
    // if its code is aware); icall (a method call through an interface: the ticket, which
    // gp::icall takes away unless the method is aware). The ticket is given only while the frame
    // runs its calls for a panic, and is always taken back when the call ends.
    enum gp_mode : unsigned char { gp_plain, gp_aware, gp_func, gp_icall };

    // The ticket of a call: set for the time of its construction (the guard's lifetime).
    struct gp_ticket {
        gp_ticket(bool check, detail::code_ptr code) noexcept : gp_given(false) {
            // Only a run of deferred calls for a panic that is not recovered gives a ticket (a
            // normal return, the usual case, is three loads); with check, only to aware code.
            const detail::run_mark* m = detail::top_run;
            if (m != nullptr && m->gp_panic != nullptr && !m->gp_panic->gp_recovered &&
                (!check || detail::aware_code(code))) {
                detail::ticket = m;
                gp_given = true;
            }
        }
        gp_ticket(const gp_ticket&) = delete;
        gp_ticket& operator=(const gp_ticket&) = delete;
        ~gp_ticket() {
            if (gp_given) {
                detail::ticket = nullptr;
            }
        }
        bool gp_given;
    };

    template <gp_mode M, typename F, typename... A>
    struct gp_node_of : gp_node {
        template <typename G, typename... B>
        explicit gp_node_of(G&& f, B&&... a)
            : gp_f(std::forward<G>(f)), gp_args(std::forward<B>(a)...) {
            this->gp_op = &gp_node_of::gp_do;
        }
        F gp_f;
        std::tuple<A...> gp_args;

        static void gp_do(gp_node* n, gp_what what) {
            auto* self = static_cast<gp_node_of*>(n);
            if (what == gp_call) {
                if constexpr (M == gp_plain) {
                    std::apply(self->gp_f, std::move(self->gp_args));
                } else {
                    if constexpr (M == gp_func) {
                        gp_ticket guard(true, self->gp_f.gp_code());
                        std::apply(self->gp_f, std::move(self->gp_args));
                        return;
                    }
                    gp_ticket guard(false, nullptr);
                    std::apply(self->gp_f, std::move(self->gp_args));
                }
                return;
            }
            self->~gp_node_of();
            if (what == gp_delete) {
                ::operator delete(static_cast<void*>(self), std::align_val_t{alignof(gp_node_of)});
            }
        }
    };

    // gp_emplace builds a node of type Node from the values b and puts it on top: in gp_buf
    // when it fits, else on the heap. If building it throws, the room is given back.
    template <typename Node, typename... B>
    void gp_emplace(B&&... b) {
        void* p = gp_alloc(sizeof(Node), alignof(Node));
        Node* n;
        try {
            n = new (p) Node(std::forward<B>(b)...);
        } catch (...) {
            gp_free(p, alignof(Node));
            throw;
        }
        n->gp_next = gp_top;
        gp_top = n;
    }

    // gp_alloc returns room for a node, in gp_buf if it fits; gp_free gives it back. gp_inline
    // reports whether p is in gp_buf, and gp_release destroys the node n and gives back its
    // room.
    void* gp_alloc(std::size_t size, std::size_t align);
    void gp_free(void* p, std::size_t align) noexcept;
    bool gp_inline(const void* p) const noexcept;
    void gp_release(gp_node* n) noexcept;

    // 64 bytes: one typical call (a node is 16 bytes, then the function pointer and the
    // arguments: three integers, a string, or a pointer receiver and a member function
    // pointer), and a frame of 96 bytes in all, which every call of a function with defers has
    // on the C++ stack: a deep recursion pays it per level (docs/spec/cpp-mapping.md "Defer").
    static constexpr std::size_t gp_inline_size = 64;

    // gp_discard is the destructor's work for a frame that has calls left or a panic of gp_panic.
    void gp_discard() noexcept;

    // gp_clear forgets the panic that is running (it was recovered, or Goexit took over).
    void gp_clear() noexcept;

    gp_node* gp_top = nullptr;
    std::uint16_t gp_used = 0;               // the bytes of gp_buf in use
    bool gp_goexit = false;                  // runtime.Goexit is running the calls
    bool gp_local = false;                   // gp_pending_panic was made by gp_panic: owned here
    detail::defer_link gp_link;              // its place in detail::top_defer
    std::exception_ptr gp_pending;           // the panic that is running (not one of gp_panic)
    panic_error* gp_pending_panic = nullptr; // it, when it is a Go panic
    alignas(std::max_align_t) unsigned char gp_buf[gp_inline_size];
};

// recover is Go's builtin recover, called by an aware function with the ticket it took at its
// start (take_defer_ctx): it stops the panic that the deferred call is running for and returns its
// value, and it returns nil in every other case (no ticket: the function was not called
// directly by the deferred-call machinery; no panic; Goexit; the panic was recovered already).
gp::any recover(defer_ctx ctx) noexcept;

} // namespace gp
