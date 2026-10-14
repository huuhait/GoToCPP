// Go's defer (gocpp/defer.hpp): the deferred calls of a function, and the panics that they
// start while another one is running (the chain: panic.hpp, trace.cpp's panic_chain). Only
// Go panics are caught; any other exception passes through (defer.hpp).
// Standard C++ only.
#include <gocpp/rt.hpp>

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

} // namespace

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
    while (gp_top != nullptr) {
        gp_node* n = gp_top;
        gp_top = n->gp_next;
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
