// gocpp/defer.hpp: Go's defer, gp::defer_frame (Phase 03 §2.8).
//
// A function that has a defer statement gets one defer_frame and one way out (the single-exit
// shape, docs/spec/cpp-mapping.md "Defer"):
//
//     int64_t f(int64_t n) {
//         int64_t gp_r0{};                          // the results
//         gp::defer_frame gp_df; try {
//             gp_df.gp_push(g, n);                  // defer g(n): g and n evaluated now
//             gp_r0 = n; goto gp_ret0;              // return n
//         } catch (gp::panic_error& gp_e) {
//             gp_df.gp_on_panic(gp_e);
//         }
//         gp_df.gp_run(); return gp_r0;             // the end of the body, or a panic
//         gp_ret0:; gp_df.gp_run(); return gp_r0;   // one exit per return, at its line
//     }
//
// gp_push stores the deferred call: the function and its arguments, evaluated at the defer
// statement as Go says (the arguments of a function pointer are converted to its parameter
// types there). gp_run runs the stored calls, the last one first, when the function returns
// (after the results are set, so a deferred call that changes a named result through a pointer
// changes what the function returns) and when a panic leaves the body (gp_on_panic keeps it, and
// gp_run throws it again after the last deferred call). A deferred call that panics doesn't stop
// the others: its panic replaces the running one, which goes into the new one's chain
// (panic_error::gp_prev), and Go prints the whole chain ("panic: A" then "\tpanic: B").
//
// Only a Go panic (gp::panic_error) runs the deferred calls. Any other C++ exception, from the
// body or from a deferred call, goes on at once, untouched: the frame's destructor frees the
// calls that didn't run, without running them, and a Go panic that was running is dropped.
// That is on purpose: a forced unwind (glibc's pthread_exit or pthread_cancel, which C code
// may call from Phase 04 on) must not be caught and kept (std::current_exception gives a null
// pointer for it, and glibc aborts when a handler doesn't throw it again), and running Go code
// during it isn't safe. A C++ exception from C++ code (Phase 04) is not Go's either: Phase 05
// converts it into a Go panic where it enters Go code, and then the deferred calls run.
//
// The calls are kept in a small buffer inside the frame (gp_inline_size bytes, one call with
// its arguments, without an allocation); more, or bigger ones, go to the heap. A frame is for
// one call of one function, on one thread: it is not copied or moved.
#pragma once

#include <gocpp/panic.hpp>

#include <cstddef>
#include <exception>
#include <memory>
#include <new>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

class defer_frame {
public:
    defer_frame() noexcept {}
    defer_frame(const defer_frame&) = delete;
    defer_frame& operator=(const defer_frame&) = delete;
    // Frees the calls that never ran (only when the function didn't end through gp_run).
    ~defer_frame();

    // defer f(a...) for a function f: stores f and the arguments, converted to f's parameter
    // types here, at the defer statement (a parameter that is a const reference, for a value
    // larger than 128 KiB, stores a copy).
    template <typename R, typename... P>
    void gp_push(R (*f)(P...), std::type_identity_t<P>... a) {
        gp_emplace<gp_node_of<R (*)(P...), std::decay_t<P>...>>(f, static_cast<P&&>(a)...);
    }

    // defer c(a...) for any other callable c (gocpp's generic lambda for a builtin or a
    // runtime template: [](auto&& gp_a0) { gp::println(gp_a0); }): stores c and copies of the
    // arguments, which c gets as rvalues when it runs.
    template <typename F, typename... A>
    void gp_push_call(F c, A&&... a) {
        gp_emplace<gp_node_of<F, std::decay_t<A>...>>(std::move(c), std::forward<A>(a)...);
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

    // gp_on_panic keeps the Go panic e that left the function's body, for gp_run. Called in
    // the function's catch (gp::panic_error&), where e is the exception being handled.
    void gp_on_panic(panic_error& e) noexcept;

    // gp_run runs the deferred calls, the last one first, then throws the panic that is still
    // running, if any: the one of the body, or the last one that a deferred call started. An
    // exception that is not a Go panic leaves it at once (see above).
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

    template <typename F, typename... A>
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
                std::apply(self->gp_f, std::move(self->gp_args));
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

    gp_node* gp_top = nullptr;
    std::size_t gp_used = 0;                 // the bytes of gp_buf in use
    std::exception_ptr gp_pending;           // the panic that is running
    panic_error* gp_pending_panic = nullptr; // it, when it is a Go panic
    alignas(std::max_align_t) unsigned char gp_buf[gp_inline_size];
};

} // namespace gp
