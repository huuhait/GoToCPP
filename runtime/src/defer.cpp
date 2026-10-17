// Go's defer (gocpp/defer.hpp): the deferred calls of a function, and the panics that they
// start while another one is running (the chain: panic.hpp, trace.cpp's panic_chain). Only
// Go panics are caught; any other exception passes through (defer.hpp).
// Standard C++ only.
#include <gocpp/rt.hpp>
#include <gocpp/trace.hpp>

#include <cstddef>
#include <exception>
#include <functional>
#include <new>
#include <utility>

namespace gp {

namespace {

// finally runs f when it goes out of scope, also when an exception leaves the scope.
template <typename F>
struct finally {
    F gp_f;
    ~finally() { gp_f(); }
};
template <typename F>
finally(F) -> finally<F>;

// The running thread's run_marks (trace.hpp).
constinit thread_local const detail::run_mark* marks = nullptr;

} // namespace

const detail::run_mark* detail::run_marks() noexcept {
    return marks;
}

defer_frame::~defer_frame() {
    // Normally gp_run has run every call. Only a frame that didn't get to it (an exception that
    // is not a Go panic) has calls left: they are freed, not run.
    while (gp_top != nullptr) {
        gp_node* n = gp_top;
        gp_top = n->gp_next;
        gp_release(n);
    }
}

void* defer_frame::gp_alloc(std::size_t size, std::size_t align) {
    std::size_t at = (gp_used + align - 1) / align * align;
    if (align <= alignof(std::max_align_t) && at <= gp_inline_size && size <= gp_inline_size - at) {
        gp_used = at + size;
        return gp_buf + at;
    }
    return ::operator new(size, std::align_val_t{align});
}

bool defer_frame::gp_inline(const void* p) const noexcept {
    // std::less gives a total order also for pointers into different objects (a node on the
    // heap), where < alone is unspecified.
    std::less<const void*> less;
    return !less(p, gp_buf) && less(p, gp_buf + gp_inline_size);
}

void defer_frame::gp_free(void* p, std::size_t align) noexcept {
    if (gp_inline(p)) {
        // The calls go last in, first out: p is the last node in gp_buf (its alignment
        // padding is given back with the node before it).
        gp_used = static_cast<std::size_t>(static_cast<unsigned char*>(p) - gp_buf);
    } else {
        ::operator delete(p, std::align_val_t{align});
    }
}

void defer_frame::gp_release(gp_node* n) noexcept {
    if (gp_inline(n)) {
        n->gp_op(n, gp_destroy);
        gp_used = static_cast<std::size_t>(reinterpret_cast<unsigned char*>(n) - gp_buf);
    } else {
        n->gp_op(n, gp_delete);
    }
}

void defer_frame::gp_on_panic(panic_error& e) noexcept {
    gp_pending = std::current_exception();
    gp_pending_panic = &e;
}

void defer_frame::gp_run() {
    // The mark for a fatal error's trace (fatal_trace): which panic runs the deferred calls, and
    // where this gp_run's caller is in the stack. A store and a load of a thread-local each.
    detail::run_mark mark{
        nullptr,
        reinterpret_cast<uintptr_t>(__builtin_extract_return_addr(__builtin_return_address(0))),
        marks};
    marks = &mark;
    finally unmark{[&mark] { marks = mark.gp_next; }};
    // The holds (gp_hold) are set aside as they come off the stack and released after the last
    // call, however early or late they were registered.
    gp_node* held = nullptr;
    finally unhold{[this, &held] {
        while (held != nullptr) {
            gp_node* h = held;
            held = h->gp_next;
            gp_release(h);
        }
    }};
    while (gp_top != nullptr) {
        gp_node* n = gp_top;
        gp_top = n->gp_next;
        if (n->gp_op == &gp_hold_op) {
            n->gp_next = held;
            held = n;
            continue;
        }
        mark.gp_panic = gp_pending_panic;
        // The node goes when its call ends, also when an exception that is not a Go panic
        // leaves gp_run.
        finally release{[this, n] { gp_release(n); }};
        try {
            n->gp_op(n, gp_call);
        } catch (panic_error& p) {
            // A deferred call panicked: its panic goes on, with the running one before it in
            // its chain, and the other deferred calls still run (Go).
            if (gp_pending_panic != nullptr) {
                detail::panic_chain(p, *gp_pending_panic);
            }
            gp_pending = std::current_exception();
            gp_pending_panic = &p;
        }
    }
    if (gp_pending != nullptr) {
        std::exception_ptr e = std::move(gp_pending);
        gp_pending = nullptr;
        gp_pending_panic = nullptr;
        std::rethrow_exception(std::move(e));
    }
}

} // namespace gp
