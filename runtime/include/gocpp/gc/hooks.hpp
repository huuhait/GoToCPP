// gocpp/gc/hooks.hpp: the hooks through which the shared runtime reaches the GC engine
// (phase-16 README §2.5; job 16.1d).
//
// Goroutines are OS threads in both engines, and the runtime's own threads (the timer thread,
// the threads that a go statement starts) touch GC memory. They must register with the
// collector before they run Go code and unregister on every exit path (§2.5). The shared
// runtime files (src/goroutine.cpp, src/timer.cpp, src/waiter.cpp) are compiled into both
// engines' programs, so they cannot call gp::gc directly: a .gp program links no gp::gc symbol
// (README §2.3 rule 7). They call these hooks instead, which are null in the reference-counting
// engine (one predictable branch and one atomic load per thread start, thread end and park) and
// installed by gp::gc::init() in the GC engine.
//
// The hooks are in gp::detail and not gp::gc on purpose: a .gp program has a null hook table of
// its own in gp::detail, and nm shows no _ZN2gp2gc symbol for it.
//
// The hooks are constant-initialized (constinit): the runtime's objects may not have a static
// initializer (internal/build's TestRuntimeObjectsHaveNoStaticInit), and a program may start a
// thread before main only if every initializer it needs has already run.
#pragma once

#include <atomic>
#include <cstddef>

namespace gp::detail {

// gc_enter_hook is a hook that takes no arguments and cannot throw: the thread hooks.
using gc_enter_hook = void (*)() noexcept;

// gc_blocking_hook runs a callback in a blocking section: the collector does not stop the
// calling thread and does not scan the frames of the callback (bdwgc: GC_do_blocking). The
// callback only waits: it is noexcept in spirit, never allocates and takes no runtime lock
// (§2.5). A backend that catches an exception from it must rethrow it after the section ends.
using gc_blocking_hook = void (*)(void (*)(void*), void*);

// A gc_hook_set is the process's hooks: null until gp::gc::init() installs them.
struct gc_hook_set {
    std::atomic<gc_enter_hook> enter{nullptr};
    std::atomic<gc_enter_hook> leave{nullptr};
    std::atomic<gc_blocking_hook> blocking{nullptr};
};

// gc_hooks is one inline variable per program: every translation unit of the shared runtime
// sees the same table, and no source file has to define it.
inline constinit gc_hook_set gc_hooks{};

// gc_install_hooks installs the three hooks (gp::gc::init does it once, before any other thread
// exists). A null hook makes its calls no-ops, which is the reference-counting engine's case.
inline void gc_install_hooks(gc_enter_hook enter, gc_enter_hook leave,
                             gc_blocking_hook blocking) noexcept {
    gc_hooks.enter.store(enter, std::memory_order_release);
    gc_hooks.leave.store(leave, std::memory_order_release);
    gc_hooks.blocking.store(blocking, std::memory_order_release);
}

// gc_enter_thread and gc_leave_thread register and unregister the calling thread.
inline void gc_enter_thread() noexcept {
    if (gc_enter_hook f = gc_hooks.enter.load(std::memory_order_acquire)) {
        f();
    }
}

inline void gc_leave_thread() noexcept {
    if (gc_enter_hook f = gc_hooks.leave.load(std::memory_order_acquire)) {
        f();
    }
}

// gc_blocking_call runs fn(data) in a blocking section, or directly when no GC engine is
// linked.
inline void gc_blocking_call(void (*fn)(void*), void* data) {
    if (gc_blocking_hook f = gc_hooks.blocking.load(std::memory_order_acquire)) {
        f(fn, data);
        return;
    }
    fn(data);
}

// gc_thread_guard enters the collector in its constructor and leaves it in its destructor. A
// goroutine's thread (src/goroutine.cpp) and the timer thread (src/timer.cpp) keep one at the
// top of their entry function, so that every exit path unregisters: a normal return, the idle
// timeout (the thread's loop ends), and pthread_exit (a forced unwind runs the destructors of
// the frames it passes, os::exit_thread).
class gc_thread_guard {
public:
    gc_thread_guard() noexcept { gc_enter_thread(); }
    ~gc_thread_guard() { gc_leave_thread(); }
    gc_thread_guard(const gc_thread_guard&) = delete;
    gc_thread_guard& operator=(const gc_thread_guard&) = delete;
};

} // namespace gp::detail
