// The tests of the GC engine's bdwgc backend (phase-16 README §2.4, §2.5, §2.6; job 16.1d):
// zeroed allocation, roots that keep an object alive, a cycle that is collected (the difference
// from the reference-counting engine), a pointer-free object that does not keep a pointer-shaped
// number alive (stage 1 of §2.4), 1,000 goroutine threads that register, allocate, unregister
// and exit before a collection, the timer thread's registration, a collection while a thread
// sits in a blocking section (the heap's blocking call and the runtime's waiter, timed and
// untimed), os::exit_thread's forced unwind, an exception from a blocking section, and the
// statistics.
//
// runtime/cxx_test.go builds it with the runtime's objects, the GC engine's sources and the
// vendored bdwgc, with heap_bdwgc.cpp selected (GOCPP_GC_BDWGC), with g++ and clang++, at -O0,
// at -O2 and with ASan + UBSan; runtime/bdwgc_test.go builds it with ThreadSanitizer. It prints
// "ok: N checks" and exits 0, or prints each failed check and exits 1. runtime/bdwgcwin_test.go
// builds it for windows/amd64 with MinGW-w64 and runs it under Wine or on the real Windows PC
// (job 16.1d-win); the one POSIX-only case (exit_thread's forced unwind) does not run there,
// see its comment.
//
// Why the objects of the liveness tests are made in threads that exit: bdwgc scans stacks and
// registers conservatively, so a dead pointer left in a register or on the main stack (at -O2,
// or on AddressSanitizer's fake stack) would keep the object alive and the checks would be
// flaky. A thread that has ended is unregistered, and its stack is not scanned (the same
// pattern as test/bdwgc_test.cpp, which job 16.1a's smoke test uses).
//
// The weak links are short disappearing links: a plain pointer in a static variable is a root
// for a conservative collector, so the link location holds a disguised pointer (XOR with a
// mask), as gc.h documents for GC_general_register_disappearing_link.
#include <gocpp/chan.hpp>
#include <gocpp/gc/rt.hpp>
#include <gocpp/gc/hooks.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>
#include <gocpp/time.hpp>

#include <gc.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <string>
#include <thread>
#include <utility>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::fprintf(stderr, "%s:%d: %s\n", file, line, what.c_str());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

// trace prints where the test is, when GOCPP_GC_TEST_TRACE is set: a hang then says which test
// it was (the harness's normal run prints nothing on stderr).
void trace(const char* what) {
    if (std::getenv("GOCPP_GC_TEST_TRACE") != nullptr) {
        std::fprintf(stderr, "trace: %s\n", what);
        std::fflush(stderr);
    }
}

using namespace std::chrono_literals;

// collect_bound is the bound of the collection timing checks below. They are hang detectors,
// not budgets: the real budget is measured by job 16.1h on the real PC.
//
// On Windows the collector stops the world with SuspendThread. Wine cannot suspend a thread that
// is inside a Wine server call until that call returns, so a collection while another thread
// runs or waits takes about 4.5 s there (Wine 11, measured with this suite; the same program on
// the real PC takes 0.7-7 ms). That is Wine's emulation, not Windows's behavior, so the checks
// keep the strict 2 s on Windows and Linux and use a wide bound under Wine, where they only say
// that the collection finishes at all. runtime/bdwgcwin_test.go sets GOCPP_WINE=1 for a Wine run
// (never for the real PC, and never on Linux).
std::chrono::milliseconds collect_bound() {
    return std::getenv("GOCPP_WINE") != nullptr ? 60s : 2s;
}

// wait_until polls pred until it is true, for at most 2 minutes (a hang is a failure, a slow
// machine is not), and says whether it got true.
template <typename P>
bool wait_until(P pred) {
    auto end = std::chrono::steady_clock::now() + 2min;
    while (!pred()) {
        if (std::chrono::steady_clock::now() > end) {
            return false;
        }
        std::this_thread::sleep_for(200us);
    }
    return true;
}

// --- weak links ---
//
// weak_slot holds a disguised pointer to the object (a plain pointer in static data would be a
// root). bdwgc sets it to null when the object becomes unreachable.

constexpr std::uintptr_t weak_mask = 0x5a5a5a5a5a5a5a5aULL;
void* weak_slot = nullptr;

// reset_weak forgets the previous link, so that a test can register a new one at the same
// location (a cleared link is gone; a still-registered one is removed here).
void reset_weak() {
    if (weak_slot != nullptr) {
        GC_unregister_disappearing_link(&weak_slot);
    }
    weak_slot = nullptr;
}

void register_weak(void* obj) {
    weak_slot = reinterpret_cast<void*>(reinterpret_cast<std::uintptr_t>(obj) ^ weak_mask);
    int rc = GC_general_register_disappearing_link(&weak_slot, obj);
    report(rc == GC_SUCCESS, __FILE__, __LINE__, "GC_general_register_disappearing_link");
}

// collect_until_weak_cleared collects up to 8 times and says whether the link was cleared. A
// collection finds the object unreachable at the first attempt in practice; the loop only
// tolerates a conservative retention that goes away when the collector runs again.
bool collect_until_weak_cleared() {
    for (int i = 0; i < 8 && weak_slot != nullptr; i++) {
        GC_gcollect();
    }
    return weak_slot == nullptr;
}

// collect_once collects and leaves the link as it is (the "the root kept it alive" direction).
void collect_once() {
    GC_gcollect();
}

// --- the tests ---

// test_alloc checks Go's zero value rule for both allocations. GC_malloc clears a scanned object
// (gc.h), but GC_malloc_atomic does not: bdwgc hands a PTRFREE block from a free list back
// untouched (src/malloc.c: only a NORMAL kind's first word is cleared on either path, and
// mark.c gives PTRFREE ok_init = FALSE; alloc.c: "All composite objects on freelists are
// cleared", which PTRFREE objects are not). A fresh block is zero by luck, so the atomic half
// fills blocks, frees them, and allocates the same size again: bdwgc's free list hands the same
// blocks back, still filled, and the backend's zeroing is what makes the check pass.
void test_alloc() {
    auto* scanned = static_cast<unsigned char*>(gp::gc::process_heap().alloc(64));
    bool scanned_zero = true;
    for (std::size_t i = 0; i < 64; i++) {
        scanned_zero = scanned_zero && scanned[i] == 0;
    }
    CHECK(scanned_zero);

    constexpr std::size_t n = 64;
    constexpr int count = 8;
    std::vector<void*> freed;
    for (int i = 0; i < count; i++) {
        auto* p = static_cast<unsigned char*>(gp::gc::process_heap().alloc_atomic(n));
        std::memset(p, 0xAA, n);
        freed.push_back(p);
    }
    for (void* p : freed) {
        GC_FREE(p);
    }
    int reused = 0;
    bool atomic_zero = true;
    for (int i = 0; i < count; i++) {
        auto* q = static_cast<unsigned char*>(gp::gc::process_heap().alloc_atomic(n));
        for (std::size_t k = 0; k < n; k++) {
            atomic_zero = atomic_zero && q[k] == 0;
        }
        if (std::find(freed.begin(), freed.end(), q) != freed.end()) {
            reused++;
        }
    }
    // At least one block came from the free list: without the backend's memset it would still
    // hold the fill (and the free list's link in its first word), so the check below would fail.
    CHECK(reused > 0);
    CHECK(atomic_zero);
    CHECK(gp::gc::process_heap().alloc(0) != nullptr);
    CHECK(gp::gc::process_heap().alloc_atomic(0) != nullptr);
}

// A root keeps an object alive across a collection; dropping the root lets the collector free it.
void test_root_keeps_alive() {
    static void* root_handle = nullptr;
    reset_weak();
    std::thread([&] {
        gp::detail::gc_thread_guard guard; // enter/leave through the hooks that init installed
        void* obj = gp::gc::process_heap().alloc(64);
        register_weak(obj);
        root_handle = gp::gc::process_heap().make_root(obj);
    }).join();
    CHECK(root_handle != nullptr);
    CHECK(weak_slot != nullptr);
    // The root cell is scanned and uncollectable: the object stays reachable although only the
    // root refers to it.
    collect_once();
    CHECK(weak_slot != nullptr);
    // Dropping the root lets the object go; the cell's pointer is cleared before the cell is
    // freed, so no stale copy keeps it alive.
    gp::gc::process_heap().drop_root(root_handle);
    root_handle = nullptr;
    CHECK(collect_until_weak_cleared());
}

// A two-object cycle with no root is collected: the point of the engine (README §1.2, §2.4).
void test_cycle_collected() {
    struct node {
        node* next;
        std::int64_t value;
    };
    reset_weak();
    std::thread([&] {
        gp::detail::gc_thread_guard guard;
        auto* a = static_cast<node*>(gp::gc::process_heap().alloc(sizeof(node)));
        auto* b = static_cast<node*>(gp::gc::process_heap().alloc(sizeof(node)));
        a->next = b;
        a->value = 1;
        b->next = a;
        b->value = 2;
        register_weak(a);
    }).join();
    CHECK(weak_slot != nullptr);
    CHECK(collect_until_weak_cleared());
}

// A pointer-free object holding a pointer-shaped number does not keep the pointed-to object
// alive: stage 1 of §2.4 (GC_malloc_atomic is never scanned).
void test_atomic_not_scanned() {
    reset_weak();
    // The atomic object lives in main and is rooted: it must survive the collection that frees
    // the target.
    void* atomic_obj = gp::gc::process_heap().alloc_atomic(sizeof(void*));
    gp::gc::root atomic_root(atomic_obj);
    std::thread([&] {
        gp::detail::gc_thread_guard guard;
        void* target = gp::gc::process_heap().alloc(64);
        register_weak(target);
        // Store the raw address in the pointer-free object. (This is exactly the pointer-shaped
        // number that a scanned object would retain; a correct engine does not.)
        std::memcpy(atomic_obj, &target, sizeof target);
    }).join();
    CHECK(weak_slot != nullptr);
    CHECK(collect_until_weak_cleared());
    // The atomic object itself is still alive (its root), with the stale number in it.
    CHECK(atomic_root.get() == atomic_obj);
    auto bytes = static_cast<unsigned char*>(atomic_obj);
    bool nonzero = false;
    for (std::size_t i = 0; i < sizeof(void*); i++) {
        nonzero = nonzero || bytes[i] != 0;
    }
    CHECK(nonzero);
}

// A goroutine body that allocates a scanned and a pointer-free object and counts itself. The
// goroutine's thread ends after it (set_idle_limit(0)), so it registers and unregisters once.
struct alloc_task final : gp::detail::task {
    explicit alloc_task(std::atomic<int>* n) : count(n) {}
    void run() override {
        void* p = gp::gc::process_heap().alloc(64);
        void* a = gp::gc::process_heap().alloc_atomic(64);
        std::memset(p, 1, 64);
        std::memset(a, 2, 64);
        count->fetch_add(1, std::memory_order_release);
    }
    std::atomic<int>* count;
};

// 1,000 goroutine threads start, register with the collector, allocate, unregister and exit;
// then a collection completes quickly. A registered thread that exited without unregistering
// would make the collection abort ("Signals delivery fails constantly", README §2.5).
void test_many_threads_then_collect() {
    constexpr int threads = 1000;
    // Every goroutine gets a thread of its own and the thread ends with it: idle reuse would
    // keep the registered threads alive, and the test is about threads that exit.
    std::size_t old_limit = gp::detail::set_idle_limit(0);
    std::atomic<int> started{0};
    for (int i = 0; i < threads; i++) {
        gp::detail::go_start(std::make_unique<alloc_task>(&started), std::size_t(1) << 20);
    }
    CHECK(wait_until([&] { return started.load(std::memory_order_acquire) == threads; }));
    // end_threads waits for the goroutine threads to end (and unregisters them through the
    // thread guard before they do).
    gp::detail::end_threads(30LL * 1000 * 1000 * 1000);
    CHECK(gp::detail::running_goroutines() == 0);
    auto begin = std::chrono::steady_clock::now();
    gp::gc::process_heap().collect();
    auto elapsed = std::chrono::steady_clock::now() - begin;
    CHECK(elapsed < collect_bound());
    gp::detail::set_idle_limit(old_limit);
}

// A collection while a thread sits in a blocking section completes, and the thread resumes when
// the section ends. The callback only waits (README §2.5): a condition variable, no allocation,
// no runtime lock.
struct blocking_state {
    std::atomic<bool> inside{false};
    std::atomic<bool> release{false};
    std::atomic<bool> resumed{false};
    std::mutex mu;
    std::condition_variable cv;
};

void blocking_wait(void* p) {
    auto* state = static_cast<blocking_state*>(p);
    state->inside.store(true, std::memory_order_release);
    std::unique_lock<std::mutex> lk(state->mu);
    state->cv.wait(lk, [state] { return state->release.load(std::memory_order_acquire); });
    state->resumed.store(true, std::memory_order_release);
}

void test_collect_during_blocking() {
    blocking_state state;
    std::thread waiter([&] {
        gp::detail::gc_thread_guard guard;
        gp::gc::process_heap().blocking(blocking_wait, &state);
    });
    CHECK(wait_until([&] { return state.inside.load(std::memory_order_acquire); }));
    auto begin = std::chrono::steady_clock::now();
    gp::gc::process_heap().collect();
    auto elapsed = std::chrono::steady_clock::now() - begin;
    CHECK(elapsed < collect_bound());
    {
        std::lock_guard<std::mutex> lk(state.mu);
        state.release.store(true, std::memory_order_release);
    }
    state.cv.notify_all();
    waiter.join();
    CHECK(state.resumed.load(std::memory_order_acquire));
}

// The runtime's own waits are blocking sections (src/waiter.cpp): a goroutine parked on a
// gp::waiter is not stopped by the collector. This is the path of channels, select, sync and
// time.Sleep (gp_park_for). The check cannot see inside the section from here, but a collection
// that had to stop this parked thread would have to signal it and scan it; the timing shows the
// collection is unaffected, and the direct blocking test above proves the mechanism itself.
void test_waiter_park_is_blocking() {
    gp::waiter w;
    std::atomic<bool> parked{false};
    std::atomic<bool> resumed{false};
    std::thread waiter([&] {
        gp::detail::gc_thread_guard guard;
        parked.store(true, std::memory_order_release);
        w.gp_park();
        resumed.store(true, std::memory_order_release);
    });
    CHECK(wait_until([&] { return parked.load(std::memory_order_acquire); }));
    std::this_thread::sleep_for(100ms); // the thread has reached its blocking section
    auto begin = std::chrono::steady_clock::now();
    gp::gc::process_heap().collect();
    auto elapsed = std::chrono::steady_clock::now() - begin;
    CHECK(elapsed < collect_bound());
    w.gp_unpark();
    waiter.join();
    CHECK(resumed.load(std::memory_order_acquire));
}

// The timed wait (time.Sleep's path through gp_park_for) is a blocking section too: a
// collection during it completes, and the wait still times out.
void test_timed_wait_is_blocking() {
    gp::waiter w;
    std::atomic<bool> parked{false};
    std::atomic<bool> finished{false};
    std::thread sleeper([&] {
        gp::detail::gc_thread_guard guard;
        parked.store(true, std::memory_order_release);
        bool got = w.gp_park_for(300ms); // no unpark: it times out
        finished.store(!got, std::memory_order_release);
    });
    CHECK(wait_until([&] { return parked.load(std::memory_order_acquire); }));
    std::this_thread::sleep_for(50ms); // inside the timed wait
    auto begin = std::chrono::steady_clock::now();
    gp::gc::process_heap().collect();
    auto elapsed = std::chrono::steady_clock::now() - begin;
    CHECK(elapsed < collect_bound());
    sleeper.join();
    CHECK(finished.load(std::memory_order_acquire)); // the wait timed out, and did not hang
}

// os::exit_thread ends a thread with pthread_exit, a forced unwind: the thread guard's
// destructor runs and unregisters the thread (README §2.5: every exit path). Without it, the
// next collection would abort ("Signals delivery fails constantly").
//
// POSIX only, and compiled out on Windows: os::exit_thread there is ExitThread(0), which runs no
// destructors, so the guard would not unregister the thread and the next collection would find
// a registered thread that no longer exists. The Windows path is a thread-exit callback instead
// (FLS, README §2.5, step 16.4) or an explicit leave before ExitThread; until 16.4 builds it,
// this check has no honest Windows form, so it does not run there (job 16.1d-win).
#ifndef _WIN32
struct exit_thread_state {
    std::atomic<bool> started{false};
};

void exit_thread_body(void* p) {
    auto* state = static_cast<exit_thread_state*>(p);
    gp::detail::gc_thread_guard guard;
    void* obj = gp::gc::process_heap().alloc(64);
    std::memset(obj, 1, 64);
    state->started.store(true, std::memory_order_release);
    gp::os::exit_thread(); // glibc unwinds the stack; the guard's destructor unregisters
}

void test_exit_thread_unregisters() {
    exit_thread_state state;
    // A raw thread (os::thread_start): std::thread's wrapper may not let a forced unwind pass.
    int rc = gp::os::thread_start(exit_thread_body, &state, std::size_t(1) << 20);
    CHECK(rc == 0);
    if (rc != 0) {
        return;
    }
    CHECK(wait_until([&] { return state.started.load(std::memory_order_acquire); }));
    std::this_thread::sleep_for(100ms); // the forced unwind and the unregistration are done
    auto begin = std::chrono::steady_clock::now();
    gp::gc::process_heap().collect();
    auto elapsed = std::chrono::steady_clock::now() - begin;
    CHECK(elapsed < collect_bound());
}
#endif // !_WIN32

// The timer thread's lifetime does not disturb the collector (README §2.5): a collection
// completes while it runs, and one after timers_shutdown (which waits for it to end) does not
// abort. This is a smoke test only: from outside, a timer thread that never registered looks the
// same (its stack holds no GC pointers and it is never scanned), so the registration itself is
// proven by the gc_thread_guard and the explicit leave in src/timer.cpp (by inspection) and by
// 16.4's signal-counting test, not by this check.
void test_timer_thread_registers() {
    auto carrier = gp::make_chan<int64_t>(1);
    std::atomic<bool> ran{false};
    gp::time::start_func<int64_t>(carrier, gp::time::nano() + 50'000'000, gp::func<void()>([&ran] {
                                      ran.store(true, std::memory_order_release);
                                  }));
    auto begin = std::chrono::steady_clock::now();
    gp::gc::process_heap().collect();
    auto elapsed = std::chrono::steady_clock::now() - begin;
    CHECK(elapsed < collect_bound());
    CHECK(wait_until([&] { return ran.load(std::memory_order_acquire); }));
    gp::detail::timers_shutdown(); // the timer thread unregisters before h.running becomes false
    begin = std::chrono::steady_clock::now();
    gp::gc::process_heap().collect();
    elapsed = std::chrono::steady_clock::now() - begin;
    CHECK(elapsed < collect_bound());
}

// An exception from a blocking section's callback does not cross bdwgc's C frames: the backend
// catches it, ends the section (the thread is registered again), and rethrows it to the caller.
void test_blocking_exception() {
    bool threw = false;
    try {
        gp::gc::process_heap().blocking([](void*) { throw std::runtime_error("from the section"); },
                                        nullptr);
    } catch (const std::runtime_error&) {
        threw = true;
    }
    CHECK(threw);
    // The thread is registered again: it can allocate and collect.
    (void)gp::gc::process_heap().alloc(8);
    gp::gc::process_heap().collect();
}

// The statistics move: the allocation counter, the bytes that bdwgc reports, and the
// collection counter.
void test_stats() {
    auto before = gp::gc::process_heap().stats();
    (void)gp::gc::process_heap().alloc(128);
    (void)gp::gc::process_heap().alloc_atomic(128);
    gp::gc::process_heap().collect();
    auto after = gp::gc::process_heap().stats();
    CHECK(after.allocs >= before.allocs + 2);
    CHECK(after.bytes >= before.bytes);
    CHECK(after.collections == before.collections + 1);
    CHECK(std::string(gp::gc::process_heap().name()) == "bdwgc");
}

} // namespace

int main() {
    // The collector's process setup, before any allocation and before any other thread: bdwgc's
    // GC_INIT, its signals, GC_allow_register_threads, the main thread's registration and the
    // hooks that the runtime's threads use.
    gp::gc::init();
    gp::os::start(); // the runtime's OS setup (SIGPIPE, the race reporter)

    trace("alloc");
    test_alloc();
    trace("root");
    test_root_keeps_alive();
    trace("cycle");
    test_cycle_collected();
    trace("atomic");
    test_atomic_not_scanned();
    trace("many threads");
    test_many_threads_then_collect();
    trace("blocking");
    test_collect_during_blocking();
    trace("waiter park");
    test_waiter_park_is_blocking();
    trace("timed wait");
    test_timed_wait_is_blocking();
#ifndef _WIN32
    trace("exit_thread"); // POSIX only: see test_exit_thread_unregisters
    test_exit_thread_unregisters();
#endif
    trace("blocking exception");
    test_blocking_exception();
    trace("timer thread");
    test_timer_thread_registers();
    trace("stats");
    test_stats();

    // Every thread that the runtime made ends: the idle threads of finished goroutines (the
    // AfterFunc goroutine's, for one) are stopped, and end_threads waits until they are gone, so
    // a thread that is on its way out cannot meet LeakSanitizer's stop at exit. The collector's
    // own marker threads (16 by default) live until the process ends; threads_ended() is not
    // used here, because it waits for the process to have no thread but main, and the markers
    // never end. bdwgc's smoke test (16.1a) runs under LeakSanitizer with them too.
    gp::detail::end_threads(30LL * 1000 * 1000 * 1000);

    if (failures != 0) {
        std::fprintf(stderr, "%d of %d checks failed\n", failures, checks);
        return 1;
    }
    std::printf("ok: %d checks\n", checks);
    return 0;
}
