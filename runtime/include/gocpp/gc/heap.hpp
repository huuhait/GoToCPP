// gocpp/gc/heap.hpp: gp::gc::heap, the GC engine's memory interface (phase-16 README §2.4).
//
// The engine reaches the collector only through this interface, so that the backend can become
// bdwgc (job 16.1d), an own collector, or a port of Go's collector without touching the
// generated code. There are two backends, chosen at build time by the macro GOCPP_GC_BDWGC (the
// GC engine's own build sets it; the runtime's unit tests do not):
//   - heap_malloc.cpp: malloc, no collector, no thread registration and no roots that it has to
//     trace. The unit-test backend (job 16.1c).
//   - heap_bdwgc.cpp: the vendored bdwgc (job 16.1d). Only one of the two defines process_heap
//     in a build, so there is no ODR violation.
// A program that is not built by the GC engine compiles neither: it links no gp::gc symbol.
//
// What a backend must guarantee (Go's rules, which bdwgc gives):
//   - alloc returns size bytes, zeroed, that the collector scans for pointers.
//   - alloc_atomic returns size bytes, zeroed, that the collector never scans: they must hold no
//     pointer (a string's bytes, a []byte).
//   - enter_thread and leave_thread register the calling OS thread, which may then touch GC
//     memory (bdwgc: GC_register_my_thread). A goroutine's thread calls them when it starts and
//     when it ends; the main thread calls enter_thread before it allocates.
//   - blocking runs fn(data) with the calling thread in a blocking section: the collector does
//     not stop or scan the frames of the section (bdwgc: GC_do_blocking). fn must not allocate
//     and must not take a runtime lock, because it runs while the thread is not registered for
//     the collection.
//   - make_root returns a handle that keeps a pointer alive; set_root stores another pointer in
//     it; drop_root releases the handle; root_value reads it. A root is one per object that gp
//     code holds (README §1.3, §2.6); the collector never moves objects, so the pointer stays
//     valid.
//   - collect runs a collection now; stats reports the backend's counters.
//
// The finalizers, cleanups and weak links of §2.4 are not declared here yet: job 16.3 adds them
// with the bdwgc backend, when they can be implemented honestly.
#pragma once

#include <cstddef>
#include <cstdint>
#include <memory>
#include <new>
#include <utility>

namespace gp::gc {

// A statistics is what a backend reports to runtime.ReadMemStats and debug.FreeOSMemory
// (README §2.4). bytes is the collector's own count of the bytes allocated so far, so it is not
// comparable across backends: the malloc backend adds up what it allocated and frees nothing,
// and the bdwgc backend reports bdwgc's total bytes allocated (GC_get_heap_usage_safe), which
// includes the collector's own bookkeeping. allocs counts the calls that gp::gc made, because
// bdwgc counts bytes only (README §2.4: testing.AllocsPerRun reads Mallocs).
struct statistics {
    std::uint64_t allocs = 0;      // the number of allocations gp::gc made
    std::uint64_t bytes = 0;       // the bytes the backend reports allocated
    std::uint64_t collections = 0; // the number of collect calls
};

// A heap is the memory interface of the GC engine: one process-wide backend, process_heap().
class heap {
public:
    // The type of a blocking section's function: blocking_fn(data) (see the top of the file).
    using blocking_fn = void (*)(void*);

    heap() = default;
    heap(const heap&) = delete;
    heap& operator=(const heap&) = delete;
    virtual ~heap() = default;

    // alloc returns size zeroed bytes that the collector scans; alloc_atomic returns size zeroed
    // bytes that hold no pointer and that the collector never scans.
    virtual void* alloc(std::size_t size) = 0;
    virtual void* alloc_atomic(std::size_t size) = 0;

    // enter_thread and leave_thread register the calling OS thread with the collector.
    virtual void enter_thread() = 0;
    virtual void leave_thread() = 0;

    // blocking runs fn(data) in a blocking section. fn must not allocate and must not take a
    // runtime lock: the frames inside the section are not scanned, and the collector's lock is
    // taken around the call. (A condition variable's own lock is fine: the wait releases it, as
    // src/waiter.cpp does.) If fn throws, the backend catches the exception at the C boundary
    // and rethrows it after the section ends (bdwgc's callback is C code).
    virtual void blocking(blocking_fn fn, void* data) = 0;

    // make_root makes a root that holds p and returns its handle; set_root stores p in the root
    // of handle; drop_root releases the root; root_value returns the root's pointer.
    virtual void* make_root(void* p) = 0;
    virtual void set_root(void* handle, void* p) = 0;
    virtual void* root_value(void* handle) const = 0;
    virtual void drop_root(void* handle) = 0;

    // collect runs a collection now, and stats returns the backend's counters.
    virtual void collect() = 0;
    virtual statistics stats() const = 0;

    // name is the backend's name for tests and messages: "malloc" here, "bdwgc" in job 16.1d.
    virtual const char* name() const = 0;
};

// process_heap returns the process-wide heap: the malloc backend (the unit tests) or the bdwgc
// backend (GOCPP_GC_BDWGC, the GC engine's own build), behind the same interface.
heap& process_heap();

// init does the collector's process setup, once, before any other thread exists and before the
// first allocation: bdwgc's GC_INIT, its signals, GC_allow_register_threads, the main thread's
// registration, and the installation of the hooks that the shared runtime calls
// (gocpp/gc/hooks.hpp). The GC engine's generated main calls it before the package's main. The
// malloc backend's init does nothing.
void init();

// new_ allocates a T in the GC heap and constructs it there from args. The allocation is
// scanned, so T may hold pointers (README §2.1: gp::gc::new_<node>(node{...})).
template <typename T, typename... Args>
T* new_(Args&&... args) {
    void* p = process_heap().alloc(sizeof(T));
    return ::new (p) T(std::forward<Args>(args)...);
}

// new_atomic_ allocates a pointer-free T in the GC heap and value-initializes it (zeroes it).
template <typename T>
T* new_atomic_() {
    void* p = process_heap().alloc_atomic(sizeof(T));
    return ::new (p) T();
}

// blocking runs f() in a blocking section (heap::blocking).
template <typename F>
void blocking(F&& f) {
    auto thunk = [](void* p) { (*static_cast<std::remove_reference_t<F>*>(p))(); };
    process_heap().blocking(thunk, static_cast<void*>(std::addressof(f)));
}

// A root keeps one pointer alive for as long as it exists: a gp object held by Go code, a Go
// object held by gp code (README §2.6). It owns a backend handle and is not copyable.
class root {
public:
    explicit root(void* p) : gp_handle_(process_heap().make_root(p)) {}
    ~root() {
        if (gp_handle_ != nullptr) {
            process_heap().drop_root(gp_handle_);
        }
    }
    root(const root&) = delete;
    root& operator=(const root&) = delete;
    root(root&& other) noexcept : gp_handle_(other.gp_handle_) { other.gp_handle_ = nullptr; }
    root& operator=(root&& other) noexcept {
        if (this != &other) {
            if (gp_handle_ != nullptr) {
                process_heap().drop_root(gp_handle_);
            }
            gp_handle_ = other.gp_handle_;
            other.gp_handle_ = nullptr;
        }
        return *this;
    }

    void set(void* p) { process_heap().set_root(gp_handle_, p); }
    void* get() const { return process_heap().root_value(gp_handle_); }

private:
    void* gp_handle_;
};

} // namespace gp::gc
