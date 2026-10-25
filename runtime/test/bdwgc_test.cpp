// The smoke test of the vendored bdwgc 8.2.12 (phase 16 README §2.4 and §2.5): threads and their
// registration, allocation (scanned and pointer-free), a blocking section, a collection after the
// threads exit, finalizers and their order, and a weak link. It prints "OK" last and exits 0, or
// prints each failed check and exits 1.
//
// runtime/cxx_test.go builds it with the bdwgc library of internal/build (build.Builder.Bdwgc)
// and the runtime's objects, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan, and
// runtime/bdwgc_test.go with ThreadSanitizer. The library is built for the same sanitizer as the
// program: bdwgc adapts to a sanitizer only when it is compiled with it.
//
// runtime/bdwgcwin_test.go builds it for windows/amd64 with the MinGW-w64 cross compiler and runs
// it under Wine or on the real Windows PC. There the collector stops the world with SuspendThread,
// not with signals: the two signal setters are skipped (gc.h: no effect on non-POSIX systems).
//
// The facts of the plan that this test keeps (a comment where a check cannot be exact):
//   - GC_malloc_atomic does not clear memory: the test writes every byte of the object before it
//     reads it, and a zero byte is never assumed;
//   - only the last finalizer registered per object is kept: the first one registered on the
//     object of the last-finalizer case must not run;
//   - a registered thread must unregister: the 8 threads call GC_unregister_my_thread, and the
//     collection after they exit must finish quickly. A registered thread that exits without
//     unregistering makes the next collection abort ("Signals delivery fails constantly", tried
//     with bdwgc 8.2.12 on this machine).
//
// The finalizer objects and the weak link are made in threads that exit. A conservative collector
// scans stacks and registers, so a dead pointer left in a register or on the main stack (at -O2,
// or on AddressSanitizer's fake stack) would keep the objects alive and the finalizers would never
// run.
//
// Incremental mode is off in the library (GC_DISABLE_INCREMENTAL): gocpp's runtime owns SIGSEGV,
// and bdwgc's write-fault handler would take it. GC_enable_incremental() is a no-op; the test
// calls it and checks that the mode stays off.
//
// Under ThreadSanitizer the two suspend/restart signals are not set: bdwgc's TSan build uses a
// synchronous signal (SIGSYS) because TSan holds an asynchronous signal back while a thread waits
// in pthread_mutex_lock (gc_priv.h). This test does not put a thread in a mutex wait while it
// collects (16.4's blocking-section tests do); it only keeps the library's own choice.
#include <gc.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <csignal>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <string>
#include <thread>
#include <vector>

// GOCPP_BDWGC_TSAN says that this program is compiled with ThreadSanitizer. GCC defines
// __SANITIZE_THREAD__; clang (and GCC 15) have __has_feature(thread_sanitizer). The harness's
// TSan mode passes -fsanitize=thread, so one of the two is set there.
#if defined(__SANITIZE_THREAD__)
#define GOCPP_BDWGC_TSAN 1
#elif defined(__has_feature)
#if __has_feature(thread_sanitizer)
#define GOCPP_BDWGC_TSAN 1
#endif
#endif

namespace {

std::atomic<int> failures{0};

void report(bool ok, const char* what) {
    if (!ok) {
        failures++;
        std::fprintf(stderr, "FAIL: %s\n", what);
    }
}

struct Obj {
    void* field;
};

// blocking_wait is the callback of GC_do_blocking: it only waits (it must not allocate, and the
// frames inside the section are not scanned).
void* blocking_wait(void* data) {
    auto* ms = static_cast<int*>(data);
    std::this_thread::sleep_for(std::chrono::milliseconds(*ms));
    return nullptr;
}

// register_current registers the calling thread with the collector. GC_SUCCESS, not
// GC_DUPLICATE: the thread was not registered implicitly (the build disables bdwgc's
// pthread_create redirects), which is the explicit registration of §2.5.
void register_current(const char* what) {
    GC_stack_base sb;
    report(GC_get_stack_base(&sb) == GC_SUCCESS, what);
    report(GC_register_my_thread(&sb) == GC_SUCCESS, what);
    report(GC_thread_is_registered() != 0, what);
}

void unregister_current(const char* what) {
    report(GC_unregister_my_thread() == GC_SUCCESS, what);
    report(GC_thread_is_registered() == 0, what);
}

// worker is one of the 8 threads: it registers, allocates a scanned object with a pointer field
// and a pointer-free object, keeps both reachable from its stack, waits inside GC_do_blocking,
// and unregisters before it exits.
void worker(int i) {
    register_current("worker: register");

    Obj* o = static_cast<Obj*>(GC_malloc(sizeof(Obj)));
    report(o != nullptr, "worker: GC_malloc");
    o->field = GC_malloc(32);
    report(o->field != nullptr, "worker: GC_malloc of the field");
    std::memset(o->field, i & 0xff, 32);

    // GC_malloc_atomic does not clear memory: write every byte before reading any.
    char* a = static_cast<char*>(GC_malloc_atomic(64));
    report(a != nullptr, "worker: GC_malloc_atomic");
    for (int k = 0; k < 64; k++) {
        a[k] = static_cast<char>(i + k);
    }

    int ms = 10;
    report(GC_do_blocking(blocking_wait, &ms) == nullptr, "worker: GC_do_blocking");

    // The values stay reachable from this stack (and from registers) across the blocking section
    // and the collection that the main thread runs after the join.
    asm volatile("" : : "r"(o), "r"(a) : "memory");
    report(o->field != nullptr, "worker: the field survived the blocking section");
    report(a[63] == static_cast<char>(i + 63),
           "worker: the atomic object survived the blocking section");

    unregister_current("worker: unregister");
}

// The finalizers record the order in which they run; they run on the main thread (finalization is
// on demand, and the main thread invokes it).
std::vector<std::string> finalized;

void finalizer_a(void*, void*) {
    finalized.push_back("A");
}
void finalizer_b(void*, void*) {
    finalized.push_back("B");
}
void finalizer_first(void*, void*) {
    finalized.push_back("first");
}
void finalizer_last(void*, void*) {
    finalized.push_back("last");
}

// finalizer_objects_in_thread makes the objects of the finalizer cases in a thread that exits:
// after the join, no dead pointer in a register or on the main stack keeps them alive.
void finalizer_objects_in_thread() {
    register_current("finalizer thread: register");

    // A points to B, and both have a finalizer: A must be finalized before B.
    Obj* b = static_cast<Obj*>(GC_malloc(sizeof(Obj)));
    Obj* a = static_cast<Obj*>(GC_malloc(sizeof(Obj)));
    report(a != nullptr && b != nullptr, "finalizer objects");
    a->field = b;
    b->field = nullptr;
    GC_register_finalizer(a, finalizer_a, nullptr, nullptr, nullptr);
    GC_register_finalizer(b, finalizer_b, nullptr, nullptr, nullptr);

    // Only the last finalizer registered on an object is kept.
    void* last = GC_malloc(32);
    report(last != nullptr, "last-finalizer object");
    GC_register_finalizer(last, finalizer_first, nullptr, nullptr, nullptr);
    GC_register_finalizer(last, finalizer_last, nullptr, nullptr, nullptr);

    unregister_current("finalizer thread: unregister");
}

// The weak link location holds a disguised pointer: a plain pointer in a static variable is a
// root, and the object would stay reachable for ever. A pointer in a pointer-free (atomic) object
// is the other documented disguise.
void* weak;
constexpr std::uintptr_t weak_mask = 0x5a5a5a5a5a5a5a5aULL;

void make_weak_link_in_thread() {
    register_current("weak thread: register");
    void* obj = GC_malloc(64);
    report(obj != nullptr, "weak object");
    weak = reinterpret_cast<void*>(reinterpret_cast<std::uintptr_t>(obj) ^ weak_mask);
    report(GC_general_register_disappearing_link(&weak, obj) == GC_SUCCESS, "weak link");
    unregister_current("weak thread: unregister");
}

} // namespace

int main() {
    // Under ThreadSanitizer, bdwgc's TSan build chooses its own synchronous suspend signal
    // (SIGSYS): TSan holds an asynchronous signal back while a thread waits in
    // pthread_mutex_lock, so overriding it with a real-time signal would break the stop there.
#if !defined(GOCPP_BDWGC_TSAN) && !defined(_WIN32)
    // The signals of the collector are chosen before GC_INIT (phase 16 §2.5): two real-time
    // signals that gocpp's runtime does not use. bdwgc's defaults are SIGPWR and SIGXCPU. A
    // Windows build stops the world with SuspendThread, not signals: it has no SIGRTMIN, and
    // gc.h says that the two setters have no effect on non-POSIX systems.
    GC_set_suspend_signal(SIGRTMIN + 6);
    GC_set_thr_restart_signal(SIGRTMIN + 7);
#endif
    GC_INIT();
#if defined(GOCPP_BDWGC_TSAN) && !defined(_WIN32)
    // bdwgc's TSan build chose its synchronous signal itself (gc_priv.h); this test must not
    // override it (see the comment above).
    report(GC_get_suspend_signal() == SIGSYS, "tsan: bdwgc keeps its own SIGSYS suspend signal");
#endif
    // Explicit registration: bdwgc's macros that redirect pthread_create are not used.
    GC_allow_register_threads();
    // Deterministic finalization: finalizers run only in GC_invoke_finalizers, on this thread.
    GC_set_finalize_on_demand(1);
    // Java-style finalization is set explicitly: its default depends on the JAVA_FINALIZATION
    // build macro (phase 16 §2.4).
    GC_set_java_finalization(1);
    // Incremental mode is off (GC_DISABLE_INCREMENTAL): gocpp's runtime owns SIGSEGV, and
    // bdwgc's write-fault handler would take it. The call must be a no-op.
    GC_enable_incremental();
    report(GC_is_incremental_mode() == 0, "incremental mode is disabled");

    // 8 threads that register, allocate, wait in a blocking section, unregister and exit.
    std::vector<std::thread> threads;
    for (int i = 0; i < 8; i++) {
        threads.emplace_back(worker, i);
    }
    for (auto& t : threads) {
        t.join();
    }

    // The collection after the registered threads exit must finish quickly: with a thread that
    // exited still registered, bdwgc 8.2.12 aborts it ("Signals delivery fails constantly").
    auto t0 = std::chrono::steady_clock::now();
    GC_gcollect();
    auto ms =
        std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() - t0)
            .count();
    std::printf("collect after 8 threads: %lld ms\n", static_cast<long long>(ms));
    report(ms < 2000, "the collection after the threads took less than 2 s");

    // The finalizer cases: A before B, and only the last finalizer of an object runs.
    std::thread(finalizer_objects_in_thread).join();
    for (int i = 0; i < 16 && finalized.size() < 3; i++) {
        GC_gcollect();
        GC_invoke_finalizers();
    }
    report(finalized.size() == 3, "all three finalizers ran");
    auto at = [](const char* name) { return std::find(finalized.begin(), finalized.end(), name); };
    report(at("A") != finalized.end() && at("B") != finalized.end() && at("A") < at("B"),
           "A is finalized before B");
    report(at("last") != finalized.end(), "the last finalizer ran");
    report(at("first") == finalized.end(), "the first finalizer of an object did not run");
    if (finalized.size() == 3) {
        std::printf("finalizers: %s, %s, %s\n", finalized[0].c_str(), finalized[1].c_str(),
                    finalized[2].c_str());
    }

    // The weak link is cleared when its object becomes unreachable.
    std::thread(make_weak_link_in_thread).join();
    for (int i = 0; i < 8 && weak != nullptr; i++) {
        GC_gcollect();
    }
    report(weak == nullptr, "the weak link was cleared");
    std::printf("weak link cleared: %s\n", weak == nullptr ? "yes" : "no");

    if (failures.load() != 0) {
        std::printf("FAILURES: %d\n", failures.load());
        return 1;
    }
    std::printf("OK\n");
    return 0;
}
