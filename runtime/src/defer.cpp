// Go's defer (gocpp/defer.hpp): the deferred calls of a function, the panics that they start
// while another one is running (the chain: panic.hpp, trace.cpp's panic_chain), recover (the
// ticket, gocpp/aware.hpp), runtime.Goexit and the C++ exceptions that become panics.
// Standard C++ only.
#include <gocpp/rt.hpp>
#include <gocpp/trace.hpp>

#include <cstddef>
#include <cstdint>
#include <exception>
#include <functional>
#include <atomic>
#include <mutex>
#include <new>
#include <utility>
#include <vector>

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

const detail::run_mark* detail::run_marks() noexcept {
    return detail::top_run;
}

namespace {

// The code of the aware functions (gocpp/aware.hpp): an append-only open-addressed hash table of
// function addresses (as integers), read without a lock (acquire loads of the published table
// and of its slots) and written under a mutex. It is read only while a panic runs deferred
// calls, and written when an aware func value is first made. A table that fills up is replaced
// by a larger one; the old tables are kept, so a reader never sees a freed array.
struct aware_table {
    std::size_t gp_cap; // a power of two
    std::atomic<std::uintptr_t>* gp_slots;
};

std::atomic<const aware_table*> aware_current{nullptr};
std::atomic<std::size_t> aware_reads{0};

std::mutex& aware_lock() {
    static std::mutex* m = new std::mutex; // never destroyed: threads may still ask at exit
    return *m;
}

// Every table ever made, so that none is a leak (they live for the whole program).
std::vector<const aware_table*>& aware_tables() {
    static auto* v = new std::vector<const aware_table*>;
    return *v;
}

std::size_t aware_hash(std::uintptr_t p) {
    return static_cast<std::size_t>((p >> 4) * 0x9e3779b97f4a7c15ull >> 20);
}

const aware_table* new_aware_table(std::size_t cap) {
    auto* t = new aware_table{cap, new std::atomic<std::uintptr_t>[cap]};
    for (std::size_t i = 0; i < cap; i++) {
        t->gp_slots[i].store(0, std::memory_order_relaxed);
    }
    aware_tables().push_back(t);
    return t;
}

// put adds p to t (which has room); the caller holds the lock.
bool aware_put(const aware_table* t, std::uintptr_t p) {
    for (std::size_t i = aware_hash(p) & (t->gp_cap - 1);; i = (i + 1) & (t->gp_cap - 1)) {
        std::uintptr_t cur = t->gp_slots[i].load(std::memory_order_relaxed);
        if (cur == p) {
            return false;
        }
        if (cur == 0) {
            t->gp_slots[i].store(p, std::memory_order_release);
            return true;
        }
    }
}

std::size_t aware_count = 0; // guarded by the lock

} // namespace

std::size_t detail::aware_lookups() noexcept {
    return aware_reads.load(std::memory_order_relaxed);
}

bool detail::aware_code(detail::code_ptr p) noexcept {
    aware_reads.fetch_add(1, std::memory_order_relaxed);
    const aware_table* t = aware_current.load(std::memory_order_acquire);
    if (t == nullptr) {
        return false;
    }
    auto key = reinterpret_cast<std::uintptr_t>(p);
    for (std::size_t i = aware_hash(key) & (t->gp_cap - 1);; i = (i + 1) & (t->gp_cap - 1)) {
        std::uintptr_t cur = t->gp_slots[i].load(std::memory_order_acquire);
        if (cur == key) {
            return true;
        }
        if (cur == 0) {
            return false;
        }
    }
}

void detail::mark_aware(detail::code_ptr p) noexcept {
    std::lock_guard<std::mutex> lock(aware_lock());
    const aware_table* t = aware_current.load(std::memory_order_relaxed);
    if (t == nullptr) {
        t = new_aware_table(64);
        aware_current.store(t, std::memory_order_release);
    } else if ((aware_count + 1) * 2 > t->gp_cap) {
        const aware_table* bigger = new_aware_table(t->gp_cap * 2);
        for (std::size_t i = 0; i < t->gp_cap; i++) {
            if (std::uintptr_t cur = t->gp_slots[i].load(std::memory_order_relaxed); cur != 0) {
                aware_put(bigger, cur);
            }
        }
        aware_current.store(bigger, std::memory_order_release);
        t = bigger;
    }
    auto key = reinterpret_cast<std::uintptr_t>(p);
    if (aware_put(t, key)) {
        aware_count++;
    }
}

gp::any recover(defer_ctx ctx) noexcept {
    if (ctx == nullptr || ctx->gp_panic == nullptr || ctx->gp_panic->gp_recovered) {
        return {};
    }
    panic_error* p = ctx->gp_panic;
    p->gp_recovered = true;
    return p->gp_value;
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
        gp_used = static_cast<std::uint32_t>(at + size);
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
        gp_used = static_cast<std::uint32_t>(static_cast<unsigned char*>(p) - gp_buf);
    } else {
        ::operator delete(p, std::align_val_t{align});
    }
}

void defer_frame::gp_release(gp_node* n) noexcept {
    if (gp_inline(n)) {
        n->gp_op(n, gp_destroy);
        gp_used = static_cast<std::uint32_t>(reinterpret_cast<unsigned char*>(n) - gp_buf);
    } else {
        n->gp_op(n, gp_delete);
    }
}

void defer_frame::gp_take(panic_error& p) {
    // A panic in a deferred call (or the body's): the running one goes into its chain.
    if (gp_pending_panic != nullptr && gp_pending_panic != &p) {
        detail::panic_chain(p, *gp_pending_panic);
    }
    gp_pending = std::current_exception(); // p, which is being handled
    gp_pending_panic = &p;
    p.gp_token.gp_mark(); // main waits for a goroutine that runs a panic's deferred calls
}

void defer_frame::gp_adopt() {
    try {
        throw;
    } catch (panic_error& p) {
        gp_take(p);
    } catch (goexit_signal&) {
        // Goexit: the deferred calls run, and nothing stops it. One in a deferred call replaces
        // the panic that was running (Go's Goexit takes over from it).
        gp_goexit = true;
        gp_pending = nullptr;
        gp_pending_panic = nullptr;
    } catch (const std::bad_alloc&) {
        // The new handler turns out-of-memory into Go's fatal error; a bad_alloc that C++ code
        // throws itself (or bad_array_new_length) is the same here, as at the top (on_terminate).
        detail::fatal("runtime: out of memory");
    } catch (...) {
        std::exception_ptr ex = std::current_exception();
        if (ex == nullptr) {
            // Not a C++ exception: glibc's forced unwind (pthread_exit, pthread_cancel), for which
            // std::current_exception gives a null pointer. It must keep unwinding, or glibc
            // aborts, and no Go code runs during it.
            throw;
        }
        // A C++ exception: a Go panic with a CppException value.
        gp::any value = detail::cxx_exception_value(ex);
        try {
            throw panic_error{std::move(value), gp::string(), true, {}};
        } catch (panic_error& p) {
            gp_take(p);
        }
    }
}

void defer_frame::gp_on_exception() {
    gp_adopt();
}

void defer_frame::gp_run() {
    // The mark for a fatal error's trace (fatal_trace): which panic runs the deferred calls, and
    // where this gp_run's caller is in the stack. A store and a load of a thread-local each.
    detail::run_mark mark{
        nullptr,
        reinterpret_cast<uintptr_t>(__builtin_extract_return_addr(__builtin_return_address(0))),
        detail::top_run};
    detail::top_run = &mark;
    finally unmark{[&mark] { detail::top_run = mark.gp_next; }};
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
        // The node goes when its call ends, also when an exception that is not kept (a forced
        // unwind) leaves gp_run.
        finally release{[this, n] { gp_release(n); }};
        try {
            n->gp_op(n, gp_call);
        } catch (...) {
            // A deferred call panicked (or ended the goroutine, or threw a C++ exception): its
            // panic goes on, with the running one before it in its chain, and the other
            // deferred calls still run (Go).
            gp_adopt();
        }
        if (gp_pending_panic != nullptr && gp_pending_panic->gp_recovered &&
            mark.gp_panic == gp_pending_panic) {
            // The call recovered the panic that it ran for: it is over. The other calls run as
            // for a normal return, and the function returns normally.
            gp_pending = nullptr;
            gp_pending_panic = nullptr;
            // Go's recovery clears the goroutine's signal (g.sig) when no panic is left, also
            // none that an outer frame is running deferred calls for: a later, ordinary panic
            // prints no signal line (panic.hpp). A panic that the recovering call started again
            // before this point has printed it.
            bool others = false;
            for (const detail::run_mark* m = mark.gp_next; m != nullptr; m = m->gp_next) {
                others = others || m->gp_panic != nullptr;
            }
            if (!others) {
                detail::clear_signal();
            }
        }
    }
    if (gp_pending != nullptr) {
        std::exception_ptr e = std::move(gp_pending);
        gp_pending = nullptr;
        gp_pending_panic = nullptr;
        std::rethrow_exception(std::move(e));
    }
    if (gp_goexit) {
        throw goexit_signal{};
    }
}

} // namespace gp
