// gocpp/gc/heap_bdwgc.cpp: gp::gc::heap over the vendored bdwgc 8.2.12 (phase-16 README §2.4,
// §2.5, §2.6; job 16.1d).
//
// The GC engine's real backend. It is compiled only when the build defines GOCPP_GC_BDWGC (the
// GC engine's own build, cmd/gocpp/gcmode.go), and heap_malloc.cpp is then left out: exactly one
// of the two defines process_heap. The runtime's unit tests build without the macro and keep
// the malloc backend; a build with the macro links the vendored archive through
// internal/build.Builder.Bdwgc.
//
// What the interface promises (gocpp/gc/heap.hpp) and how bdwgc gives it:
//   - alloc: GC_malloc, which clears the object (gc.h: "The non-atomic versions guarantee that
//     the new object is cleared") and makes it scanned for pointers.
//   - alloc_atomic: GC_malloc_atomic, which does NOT clear (gc.h says only that the object holds
//     no relevant pointers; README §2.4 and §7.2), so this file zeroes it. It is never scanned.
//   - enter_thread/leave_thread: GC_register_my_thread/GC_unregister_my_thread. The
//     registration is explicit: the library is built with GC_NO_THREAD_REDIRECTS, so a thread
//     that is not registered is not scanned and must not allocate. A registered thread that
//     exits without unregistering makes the next collection abort with "Signals delivery fails
//     constantly" (README §2.5, measured with this bdwgc).
//   - blocking: GC_do_blocking. The callback runs while the thread is "inactive": the collector
//     does not stop it and scans its stack only from the point where the section began. The
//     callback must only wait (§2.5): it never allocates and takes no runtime lock. The C
//     boundary is noexcept: an exception from the callback is caught and rethrown after the
//     section, so the thread is registered again and its frames are scanned when it unwinds.
//   - roots: one GC_MALLOC_UNCOLLECTABLE cell per root, holding the pointer. The cell is scanned
//     (so it keeps the object alive) and is never collected itself; drop_root frees it with
//     GC_FREE after clearing the pointer, so a stale copy of the cell cannot keep the object
//     alive. This is the "documented equivalent" of README §2.6's root: the collector never
//     moves an object, so the pointer stays valid.
//   - collect: GC_gcollect (a full collection).
//   - stats: the number of allocations and collections that gp::gc made (bdwgc counts bytes
//     only), and bdwgc's total allocated bytes from GC_get_heap_usage_safe.
//
// Not here (16.3): finalizers, cleanups, weak links, GC_set_min_bytes_allocd (GOGC), the
// memory limit, GC_reachable_here. Not here (16.4): foreign threads, the altstack
// (GC_register_altstack), and the waits that are not blocking sections yet: the timer thread's
// own wait (src/timer.cpp), waiter.cpp's gp_post_wait (the deadlock report's asker),
// goroutine.cpp's wait_for_reports, and the netpoller and blocking I/O.
//
// Signals (README §2.5): bdwgc stops the world on Linux by sending a signal to every running
// thread. Its defaults are SIGPWR and SIGXCPU; gocpp chooses two real-time signals before
// GC_INIT instead, and documents them. Under ThreadSanitizer bdwgc's own build uses SIGSYS for
// the suspension (an asynchronous signal is held back by TSan while a thread waits in a
// pthread mutex), so a TSan build keeps bdwgc's choice. Windows does not use signals at all
// (SuspendThread), and the setters are no-ops there.
#include <gocpp/nofortify.h>
#include <gocpp/gc/heap.hpp>
#include <gocpp/gc/hooks.hpp>

#if defined(GOCPP_GC_BDWGC)

#include <gc.h>

#include <atomic>
#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <exception>
#include <mutex>
#include <new>

// GOCPP_GC_TSAN says that this file is compiled with ThreadSanitizer (the bdwgc library is then
// built with it too, and chose SIGSYS for the suspension signal).
#if defined(__SANITIZE_THREAD__)
#define GOCPP_GC_TSAN 1
#elif defined(__has_feature)
#if __has_feature(thread_sanitizer)
#define GOCPP_GC_TSAN 1
#endif
#endif

namespace gp::gc {

namespace {

// gc_fatal reports a failed bdwgc call and ends the program. The GC engine has no panic
// machinery of its own yet (step 16.6); a failure here means that the collector and the runtime
// disagree about a thread, which no Go code can recover from.
[[noreturn]] void gc_fatal(const char* what, int status) {
    std::fprintf(stderr, "runtime: gp::gc (bdwgc): %s failed (status %d)\n", what, status);
    std::fflush(stderr);
    std::abort();
}

// A root_cell is what make_root returns: an uncollectable, scanned cell that holds the pointer
// the collector must keep alive (README §2.6).
struct root_cell {
    void* gp_pointer;
};

// A blocking_call is the data of one GC_do_blocking call. error keeps an exception from the
// callback, which must not cross bdwgc's C frames.
struct blocking_call {
    heap::blocking_fn fn;
    void* data;
    std::exception_ptr error;
};

// bdwgc_blocking_thunk is the C callback of GC_do_blocking. It is noexcept by construction: the
// callback's exception is stored and rethrown by blocking() after GC_do_blocking returns.
void* bdwgc_blocking_thunk(void* p) noexcept {
    auto* call = static_cast<blocking_call*>(p);
    try {
        call->fn(call->data);
    } catch (...) {
        call->error = std::current_exception();
    }
    return nullptr;
}

class bdwgc_heap final : public heap {
public:
    void* alloc(std::size_t size) override {
        // GC_malloc clears the object (gc.h) and scans it. Size 0 still gets a valid, unique
        // object, as Go's allocation of a zero-size type does.
        void* p = GC_malloc(size == 0 ? 1 : size);
        if (p == nullptr) {
            throw std::bad_alloc();
        }
        gp_allocs.fetch_add(1, std::memory_order_relaxed);
        return p;
    }

    void* alloc_atomic(std::size_t size) override {
        // GC_malloc_atomic does not clear memory: zero every byte (Go's zero value rule).
        std::size_t n = size == 0 ? 1 : size;
        void* p = GC_malloc_atomic(n);
        if (p == nullptr) {
            throw std::bad_alloc();
        }
        std::memset(p, 0, n);
        gp_allocs.fetch_add(1, std::memory_order_relaxed);
        return p;
    }

    void enter_thread() override {
        // A thread that is already registered (the main thread after GC_INIT, or a second
        // enter_thread call) gets GC_DUPLICATE, which is success for this interface.
        GC_stack_base sb;
        int rc = GC_get_stack_base(&sb);
        if (rc != GC_SUCCESS) {
            gc_fatal("GC_get_stack_base", rc);
        }
        rc = GC_register_my_thread(&sb);
        if (rc != GC_SUCCESS && rc != GC_DUPLICATE) {
            gc_fatal("GC_register_my_thread", rc);
        }
    }

    void leave_thread() override {
        // The main thread is registered implicitly by GC_INIT and never calls this in a program
        // (the process exits). A worker that is not registered any more (a double leave) is
        // left alone.
        if (GC_thread_is_registered() == 0) {
            return;
        }
        int rc = GC_unregister_my_thread();
        if (rc != GC_SUCCESS) {
            gc_fatal("GC_unregister_my_thread", rc);
        }
    }

    void blocking(blocking_fn fn, void* data) override {
        blocking_call call{fn, data, nullptr};
        GC_do_blocking(bdwgc_blocking_thunk, &call);
        if (call.error != nullptr) {
            std::rethrow_exception(call.error);
        }
    }

    void* make_root(void* p) override {
        // The cell is scanned and never collected: the pointer in it is a root. GC_malloc would
        // let the collector free the cell, and with it the last reference to the object.
        auto* cell = static_cast<root_cell*>(GC_MALLOC_UNCOLLECTABLE(sizeof(root_cell)));
        if (cell == nullptr) {
            throw std::bad_alloc();
        }
        cell->gp_pointer = p;
        return cell;
    }

    void set_root(void* handle, void* p) override {
        static_cast<root_cell*>(handle)->gp_pointer = p;
    }

    void* root_value(void* handle) const override {
        return static_cast<const root_cell*>(handle)->gp_pointer;
    }

    void drop_root(void* handle) override {
        auto* cell = static_cast<root_cell*>(handle);
        // Clear the pointer first: the memory of a freed uncollectable object is reused, and a
        // stale copy of the cell must not keep the old object alive.
        cell->gp_pointer = nullptr;
        GC_FREE(cell);
    }

    void collect() override {
        GC_gcollect();
        gp_collections.fetch_add(1, std::memory_order_relaxed);
    }

    statistics stats() const override {
        // bdwgc counts bytes only, so the allocations are ours. GC_get_heap_usage_safe takes
        // the allocator lock and returns a consistent total (gc.h); it is for the statistics
        // of runtime.ReadMemStats, never for a blocking section.
        GC_word total = 0;
        GC_get_heap_usage_safe(nullptr, nullptr, nullptr, nullptr, &total);
        statistics s;
        s.allocs = gp_allocs.load(std::memory_order_relaxed);
        s.bytes = static_cast<std::uint64_t>(total);
        s.collections = gp_collections.load(std::memory_order_relaxed);
        return s;
    }

    const char* name() const override { return "bdwgc"; }

private:
    std::atomic<std::uint64_t> gp_allocs{0};
    std::atomic<std::uint64_t> gp_collections{0};
};

// The hooks that the shared runtime calls (defined below gc_init_once).
void bdwgc_enter_hook() noexcept;
void bdwgc_leave_hook() noexcept;
void bdwgc_blocking_hook(void (*fn)(void*), void* data);

// gc_init_once does the collector's process setup exactly once (the generated main calls
// gp::gc::init() once; a test or an embedding program may call it again).
void gc_init_once() {
    // The signals must be chosen before the collector is initialized (gc.h: GC_set_*_signal
    // does nothing after GC_is_initialized). On Linux, two real-time signals of gocpp's own:
    // SIGRTMIN+6 and SIGRTMIN+7, the choice of job 16.1a's smoke test (runtime/test/
    // bdwgc_test.cpp; glibc keeps SIGRTMIN and SIGRTMIN+1 for itself, Go's preemption signal is
    // SIGURG, and gocpp's runtime uses no real-time signal). A TSan build keeps bdwgc's SIGSYS
    // (see the top of this file). Windows uses SuspendThread and the setters are empty.
#if defined(__linux__) && !defined(GOCPP_GC_TSAN)
    GC_set_suspend_signal(SIGRTMIN + 6);
    GC_set_thr_restart_signal(SIGRTMIN + 7);
#endif

    GC_INIT();
    // Explicit registration: bdwgc's macros that redirect pthread_create are off
    // (GC_NO_THREAD_REDIRECTS), so every thread registers through enter_thread. The main thread
    // is registered by GC_INIT itself; GC_register_my_thread would return GC_DUPLICATE for it.
    // Check, and register it explicitly only if the platform did not (gc.h: the main thread is
    // "always done implicitly", but the check costs nothing and documents the assumption).
    GC_allow_register_threads();
    if (GC_thread_is_registered() == 0) {
        GC_stack_base sb;
        int rc = GC_get_stack_base(&sb);
        if (rc != GC_SUCCESS) {
            gc_fatal("GC_get_stack_base (main thread)", rc);
        }
        rc = GC_register_my_thread(&sb);
        if (rc != GC_SUCCESS && rc != GC_DUPLICATE) {
            gc_fatal("GC_register_my_thread (main thread)", rc);
        }
    }
    // From here on, the shared runtime's threads register and unregister and put their waits in
    // blocking sections (gocpp/gc/hooks.hpp).
    ::gp::detail::gc_install_hooks(&bdwgc_enter_hook, &bdwgc_leave_hook, &bdwgc_blocking_hook);
}

// The hooks the shared runtime calls. They run on a thread that the collector may not know
// yet, so they may not allocate (GC_register_my_thread does not).
void bdwgc_enter_hook() noexcept {
    process_heap().enter_thread();
}

void bdwgc_leave_hook() noexcept {
    process_heap().leave_thread();
}

void bdwgc_blocking_hook(void (*fn)(void*), void* data) {
    process_heap().blocking(fn, data);
}

} // namespace

// process_heap returns the one backend of the process (the same pattern as the malloc backend:
// a function-local static, so that the object adds no static initializer).
heap& process_heap() {
    static bdwgc_heap the_heap;
    return the_heap;
}

void init() {
    static std::once_flag once;
    std::call_once(once, gc_init_once);
}

} // namespace gp::gc

#endif // GOCPP_GC_BDWGC
