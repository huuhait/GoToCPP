// The unit tests of gp::waiter (gocpp/waiter.hpp) and of the goroutine runtime
// (gocpp/goroutine.hpp, src/goroutine.cpp), on top of the thread layer gp::os, which
// test/thread_test.cpp tests.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and with ThreadSanitizer
// (goroutine_test.go), and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or
// prints each failed check and exits 1.
//
// Run with a case name as the one argument, the program runs a case that ends the process (a
// goroutine's panic, a stack overflow, a failure to start a thread), which goroutine_test.go
// compares with what Go prints.
//
// Nothing here depends on timing: threads synchronize through waiters and atomics, and a wait
// has a limit of minutes (a hang is a failure, a slow machine is not). The one test that sleeps
// checks that something takes at least as long as it should, which a slow machine can't break.
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <set>
#include <stdexcept>
#include <string>
#include <string_view>
#include <thread>
#include <utility>
#include <vector>

namespace {

using namespace std::chrono_literals;

std::atomic<int> checks{0};
std::atomic<int> failures{0};

void say(const std::string& s) {
    gp::os::write(gp::os::stderr_fd, s.data(), s.size());
}

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        say(std::string(file) + ":" + std::to_string(line) + ": " + what + "\n");
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

#define CHECK_EQ(got, want)                                                                        \
    do {                                                                                           \
        auto gp_got = (got);                                                                       \
        decltype(gp_got) gp_want = (want);                                                         \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = " + std::to_string(gp_got) + ", want " + std::to_string(gp_want));         \
    } while (0)

// wait_for polls pred until it is true, for at most two minutes, and says whether it got true.
template <typename P>
bool wait_for(P pred) {
    auto end = std::chrono::steady_clock::now() + 2min;
    while (!pred()) {
        if (std::chrono::steady_clock::now() > end) {
            return false;
        }
        std::this_thread::sleep_for(200us);
    }
    return true;
}

// A gate holds goroutines until it is opened. Each one parks on the waiter of its own
// goroutine record, so the test also uses that waiter.
struct gate {
    std::mutex mu;
    std::vector<gp::waiter*> waiting; // the waiters of the goroutines that wait
    std::atomic<int> arrived{0};
    std::atomic<bool> open{false};

    // pass is called by a goroutine: it waits until the gate is open.
    void pass() {
        gp::waiter& w = gp::detail::this_goroutine().park;
        {
            std::lock_guard<std::mutex> lk(mu);
            waiting.push_back(&w);
        }
        arrived++;
        while (!open.load(std::memory_order_acquire)) {
            w.gp_park();
        }
    }

    void release() {
        open.store(true, std::memory_order_release);
        std::lock_guard<std::mutex> lk(mu);
        for (gp::waiter* w : waiting) {
            w->gp_unpark();
        }
    }
};

// frame_here returns the address of the caller's frame.
[[gnu::noinline]] uintptr_t frame_here() {
    return reinterpret_cast<uintptr_t>(__builtin_frame_address(0));
}

// --- gp::waiter ---

void test_waiter_basic() {
    gp::waiter w;
    // Unpark before park: the park returns at once, and the permit is used up.
    w.gp_unpark();
    w.gp_park();
    CHECK(!w.gp_try_park());
    // Permits don't add up.
    w.gp_unpark();
    w.gp_unpark();
    w.gp_unpark();
    CHECK(w.gp_try_park());
    CHECK(!w.gp_try_park());
    CHECK(!w.gp_park_for(1ms));
    // A permit that is there is taken also with a time that has passed.
    w.gp_unpark();
    CHECK(w.gp_park_until(std::chrono::steady_clock::now() - 1h));
    CHECK(!w.gp_park_until(std::chrono::steady_clock::now() - 1h));
    CHECK(!w.gp_park_for(-5s));
    w.gp_unpark();
    CHECK(w.gp_park_for(0ms));

    // A time out takes at least as long as it should (the clock is monotonic: a loaded machine
    // can only make it longer).
    auto start = std::chrono::steady_clock::now();
    CHECK(!w.gp_park_for(30ms));
    CHECK(std::chrono::steady_clock::now() - start >= 30ms);
    start = std::chrono::steady_clock::now();
    CHECK(!w.gp_park_until(start + 20ms));
    CHECK(std::chrono::steady_clock::now() - start >= 20ms);
    // A float duration, and a time very far away with a permit there.
    CHECK(!w.gp_park_for(std::chrono::duration<double, std::milli>(2.5)));
    w.gp_unpark();
    CHECK(w.gp_park_for(std::chrono::hours::max()));
    w.gp_unpark();
    CHECK(w.gp_park_until(std::chrono::steady_clock::time_point::max()));
}

// A parked thread wakes up when another unparks it, whichever comes first.
void test_waiter_threads() {
    for (int round = 0; round < 200; round++) {
        gp::waiter w;
        std::atomic<bool> woke{false};
        std::atomic<bool> started{false};
        std::thread t([&] {
            started.store(true);
            w.gp_park();
            woke.store(true, std::memory_order_release);
        });
        if (round % 2 == 0) {
            // Park first (as far as a test can tell), then unpark.
            CHECK(wait_for([&] { return started.load(); }));
        }
        w.gp_unpark();
        t.join();
        CHECK(woke.load(std::memory_order_acquire));
    }
    // A timed park gets its permit from another thread.
    gp::waiter w;
    std::thread t([&] { w.gp_unpark(); });
    CHECK(w.gp_park_for(10min));
    t.join();

    // The owner may destroy a waiter as soon as its park returns: the unpark must not touch
    // it afterwards (ASan and TSan check).
    for (int round = 0; round < 500; round++) {
        auto* wp = new gp::waiter;
        std::atomic<bool> unparked{false};
        std::thread u([&unparked, wp] {
            wp->gp_unpark();
            unparked.store(true);
        });
        wp->gp_park();
        delete wp;
        u.join();
        CHECK(unparked.load());
    }
}

// Ping-pong: two threads hand a turn to each other 20000 times, each waiting on its own waiter
// and on the turn, the way every user of a waiter loops on its own condition.
void test_waiter_pingpong() {
    constexpr int rounds = 20000;
    gp::waiter a;
    gp::waiter b;
    std::atomic<int> turn{0}; // 0: a's, 1: b's
    std::atomic<int> a_count{0};
    std::atomic<int> b_count{0};
    std::thread t([&] {
        for (int i = 0; i < rounds; i++) {
            while (turn.load(std::memory_order_acquire) != 1) {
                b.gp_park();
            }
            b_count++;
            turn.store(0, std::memory_order_release);
            a.gp_unpark();
        }
    });
    for (int i = 0; i < rounds; i++) {
        while (turn.load(std::memory_order_acquire) != 0) {
            a.gp_park();
        }
        a_count++;
        turn.store(1, std::memory_order_release);
        b.gp_unpark();
    }
    t.join();
    CHECK_EQ(a_count.load(), rounds);
    CHECK_EQ(b_count.load(), rounds);
}

// Many threads unpark one waiter while one thread parks and counts what they made.
void test_waiter_many_unparkers() {
    constexpr int producers = 8;
    constexpr int each = 2000;
    gp::waiter w;
    std::atomic<int> items{0};
    std::vector<std::thread> ts;
    for (int i = 0; i < producers; i++) {
        ts.emplace_back([&] {
            for (int k = 0; k < each; k++) {
                items.fetch_add(1, std::memory_order_release);
                w.gp_unpark();
            }
        });
    }
    int taken = 0;
    int parks = 0;
    while (taken < producers * each) {
        if (items.load(std::memory_order_acquire) > taken) {
            taken++;
        } else {
            w.gp_park();
            parks++;
        }
    }
    for (auto& t : ts) {
        t.join();
    }
    CHECK_EQ(taken, producers * each);
    CHECK(parks <= producers * each + 1);
}

// --- goroutines ---

// The main thread's slot (gp::os::tls_get) holds its goroutine record.
void test_main_slot() {
    CHECK(gp::os::tls_get() == &gp::detail::this_goroutine());
    void* mine = gp::os::tls_get();
    void* other = reinterpret_cast<void*>(1);
    std::thread t([&] { other = gp::os::tls_get(); });
    t.join();
    CHECK(other == nullptr);
    CHECK(gp::os::tls_get() == mine);
}

void test_ids_and_count() {
    // The main goroutine is 1, and counts.
    CHECK_EQ(gp::detail::goroutine_id(), uint64_t(1));
    CHECK_EQ(gp::detail::this_goroutine().id, uint64_t(1));
    CHECK_EQ(gp::num_goroutine(), int64_t(1));

    constexpr int n = 200;
    gate g;
    std::mutex mu;
    std::vector<uint64_t> ids;
    std::vector<uint64_t> again;
    std::shared_ptr<int> token = std::make_shared<int>(5);
    for (int i = 0; i < n; i++) {
        gp::go([&g, &mu, &ids, &again, token] {
            uint64_t id = gp::detail::goroutine_id();
            g.pass();
            std::lock_guard<std::mutex> lk(mu);
            ids.push_back(id);
            again.push_back(gp::detail::goroutine_id());
        });
        // The goroutine counts before its thread runs.
        CHECK_EQ(gp::num_goroutine(), int64_t(2 + i));
    }
    CHECK(wait_for([&] { return g.arrived.load() == n; }));
    CHECK_EQ(gp::num_goroutine(), int64_t(1 + n));
    CHECK(token.use_count() == 1 + n);
    g.release();
    // They stop counting when the function has returned and its closure is gone.
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
    CHECK_EQ(token.use_count(), 1L);
    {
        std::lock_guard<std::mutex> lk(mu);
        CHECK_EQ(ids.size(), std::size_t(n));
        CHECK(ids == again);
        std::set<uint64_t> distinct(ids.begin(), ids.end());
        CHECK_EQ(distinct.size(), std::size_t(n));
        CHECK(*distinct.begin() >= 2);
        CHECK(!distinct.contains(1));
    }
    // The main goroutine's id doesn't change.
    CHECK_EQ(gp::detail::goroutine_id(), uint64_t(1));
}

// A goroutine started by a goroutine, and a closure that can only be moved.
void test_nested_and_move_only() {
    std::atomic<uint64_t> child_parent_id{0};
    std::atomic<uint64_t> child_id{0};
    std::atomic<int> value{0};
    auto owned = std::make_unique<int>(41);
    gp::go([&, owned = std::move(owned)]() mutable {
        child_parent_id = gp::detail::goroutine_id();
        gp::go([&, v = std::move(owned)] {
            child_id = gp::detail::goroutine_id();
            value = *v + 1;
        });
    });
    CHECK(wait_for([&] { return value.load() == 42; }));
    CHECK(child_parent_id.load() >= 2);
    CHECK(child_id.load() > child_parent_id.load());
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
}

// The goroutine's record has its own waiter, which a channel (step 8.3) uses to wake it.
void test_goroutine_waiter() {
    std::atomic<gp::waiter*> mine{nullptr};
    std::atomic<int> state{0};
    gp::go([&] {
        mine.store(&gp::detail::this_goroutine().park);
        state.store(1);
        while (state.load(std::memory_order_acquire) != 2) {
            gp::detail::this_goroutine().park.gp_park();
        }
        state.store(3);
    });
    CHECK(wait_for([&] { return state.load() == 1; }));
    state.store(2, std::memory_order_release);
    mine.load()->gp_unpark();
    CHECK(wait_for([&] { return state.load() == 3; }));
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
}

// Many goroutines alive at once (the stress test of 10k is step 8.13).
void test_many_goroutines() {
    constexpr int n = 1000;
    gate g;
    std::atomic<int> sum{0};
    for (int i = 0; i < n; i++) {
        gp::go([&g, &sum, i] {
            g.pass();
            sum += i;
        });
    }
    CHECK(wait_for([&] { return g.arrived.load() == n; }));
    CHECK_EQ(gp::num_goroutine(), int64_t(1 + n));
    g.release();
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
    CHECK_EQ(sum.load(), n * (n - 1) / 2);
}

struct stack_view {
    std::atomic<bool> done{false};
    bool bounds = false;
    uintptr_t lo = 0;
    uintptr_t hi = 0;
    uintptr_t frame = 0;
};

void test_stack_sizes() {
    using gp::detail::default_stack_size;
    using gp::detail::min_stack_size;
    CHECK_EQ(default_stack_size, std::size_t(8) << 20);
    CHECK_EQ(gp::detail::stack_size(), default_stack_size);

    auto size_of_goroutine = [](auto start) {
        stack_view v;
        start(v);
        CHECK(wait_for([&] { return v.done.load(std::memory_order_acquire); }));
        CHECK(v.bounds);
        CHECK(v.lo < v.frame && v.frame < v.hi);
        return v.hi - v.lo;
    };
    auto body = [](stack_view* v) {
        v->bounds = gp::os::stack_bounds(v->lo, v->hi);
        v->frame = frame_here();
        v->done.store(true, std::memory_order_release);
    };
    // The default: 8 MiB.
    CHECK(size_of_goroutine([&](stack_view& v) { gp::go([&v, body] { body(&v); }); }) >=
          default_stack_size);
    // set_stack_size: below 1 MiB it is 1 MiB; 0 is the default again.
    gp::detail::set_stack_size(64 << 10);
    CHECK_EQ(gp::detail::stack_size(), min_stack_size);
    CHECK(size_of_goroutine([&](stack_view& v) { gp::go([&v, body] { body(&v); }); }) >=
          min_stack_size);
    gp::detail::set_stack_size(std::size_t(3) << 20);
    CHECK_EQ(gp::detail::stack_size(), std::size_t(3) << 20);
    CHECK(size_of_goroutine([&](stack_view& v) { gp::go([&v, body] { body(&v); }); }) >=
          (std::size_t(3) << 20));
    gp::detail::set_stack_size(0);
    CHECK_EQ(gp::detail::stack_size(), default_stack_size);
    // One goroutine with a stack of its own (the explicit size has no minimum of 1 MiB).
    std::size_t small = size_of_goroutine([&](stack_view& v) {
        gp::detail::go_start(std::make_unique<gp::detail::task_of<std::function<void()>>>(
                                 std::function<void()>([&v, body] { body(&v); })),
                             std::size_t(128) << 10);
    });
    CHECK(small >= (std::size_t(128) << 10)); // (more if the OS reuses a bigger stack)
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
}

// Deep recursion that fits in the stack goes through (no false stack overflow), in a goroutine
// and in the main goroutine: 5 MiB of frames on an 8 MiB stack.
template <std::size_t N>
[[gnu::noinline]] int64_t recurse_fit(int64_t depth) {
    volatile char pad[N];
    pad[0] = 1;
    pad[N - 1] = 2;
    int64_t r = depth == 0 ? 0 : recurse_fit<N>(depth - 1);
    asm volatile("" ::: "memory");
    return r + pad[0] + pad[N - 1] - 2;
}

void test_deep_recursion_fits() {
    CHECK_EQ(recurse_fit<64 << 10>(50), int64_t(51));
    std::atomic<int64_t> result{-1};
    gp::go([&] { result = recurse_fit<64 << 10>(64); });
    CHECK(wait_for([&] { return result.load() != -1; }));
    CHECK_EQ(result.load(), int64_t(65));
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
}

// A goroutine's panic that its own function recovers (a deferred call in the emitter's code)
// never gets to the goroutine's start; here, a function that catches it ends normally.
void test_panic_caught_in_goroutine() {
    std::atomic<int> state{0};
    gp::go([&] {
        try {
            gp::panic("inner"_gs);
        } catch (const gp::panic_error& e) {
            state = e.gp_text.gp_view() == "inner" ? 1 : -1;
        }
    });
    CHECK(wait_for([&] { return state.load() != 0; }));
    CHECK_EQ(state.load(), 1);
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
}

// The report names the goroutine that calls it, as Go's "goroutine N [running]:".
void test_report_goroutine_line() {
    auto report_here = [] {
        std::string out;
        gp::detail::panic_report(out, "x"_gs);
        return out;
    };
    CHECK(report_here() == "panic: x\n\ngoroutine 1 [running]:\n");
    std::string theirs;
    std::atomic<bool> done{false};
    uint64_t id = 0;
    gp::go([&] {
        id = gp::detail::goroutine_id();
        theirs = report_here();
        done.store(true, std::memory_order_release);
    });
    CHECK(wait_for([&] { return done.load(std::memory_order_acquire); }));
    CHECK(theirs == "panic: x\n\ngoroutine " + std::to_string(id) + " [running]:\n");
    CHECK(id >= 2);
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
}

// A thread that no go statement started (a thread of a C library, a std::thread) gets a
// goroutine record of its own, which doesn't count.
void test_foreign_thread() {
    uint64_t id = 0;
    uint64_t id2 = 0;
    int64_t during = 0;
    std::thread t([&] {
        id = gp::detail::goroutine_id();
        id2 = gp::detail::goroutine_id();
        during = gp::num_goroutine();
    });
    t.join();
    CHECK(id >= 2);
    CHECK_EQ(id, id2);
    CHECK_EQ(during, int64_t(1));
    uint64_t other = 0;
    std::thread t2([&] { other = gp::detail::goroutine_id(); });
    t2.join();
    CHECK(other != id);
    CHECK_EQ(gp::num_goroutine(), int64_t(1));
}

// The thread layer's start also works while goroutines are running and ending (races between
// the start of a thread and the end of another are for the sanitizers to see).
void test_churn() {
    constexpr int n = 2000;
    std::atomic<int> ran{0};
    std::atomic<int> uncounted{0};
    for (int i = 0; i < n; i++) {
        gp::go([&ran, &uncounted] {
            // A goroutine counts itself from its first instruction: go() counted it before
            // its thread could run, and the main goroutine is there too.
            if (gp::num_goroutine() < 2) {
                uncounted++;
            }
            ran++;
        });
        if (i % 100 == 99) {
            // Don't pile up thousands of threads: let most of them end.
            CHECK(wait_for([&] { return gp::num_goroutine() < 50; }));
        }
    }
    CHECK(wait_for([&] { return ran.load() == n; }));
    CHECK(wait_for([&] { return gp::num_goroutine() == 1; }));
    CHECK_EQ(uncounted.load(), 0);
}

void run_unit_tests() {
    // The main goroutine's id is 1 only if it is asked first.
    test_ids_and_count();
    test_main_slot();
    test_waiter_basic();
    test_waiter_threads();
    test_waiter_pingpong();
    test_waiter_many_unparkers();
    test_nested_and_move_only();
    test_goroutine_waiter();
    test_many_goroutines();
    test_stack_sizes();
    test_deep_recursion_fits();
    test_panic_caught_in_goroutine();
    test_report_goroutine_line();
    test_foreign_thread();
    test_churn();
    rt_test::threads_ended();
    if (failures > 0) {
        say("FAIL: " + std::to_string(failures.load()) + " of " + std::to_string(checks.load()) +
            " checks\n");
        gp::os::exit(1);
    }
    gp::fmtshim::Printf("ok: %d checks\n"_gs, int64_t(checks.load()));
}

// --- the cases that end the process (goroutine_test.go checks what they print) ---

// park_forever blocks the calling thread for good: the case ends the process some other way.
[[noreturn]] void park_forever() {
    gp::waiter w;
    for (;;) {
        w.gp_park();
    }
}

// Never set: it gives the compilers' infinite-recursion warnings a way out.
volatile bool stop_recursing = false;

// A frame with a pad of N bytes that the recursion never leaves.
template <std::size_t N>
[[gnu::noinline]] int64_t recurse_forever(int64_t depth) {
    if (stop_recursing) {
        return 0;
    }
    volatile char pad[N];
    pad[0] = static_cast<char>(depth);
    pad[N - 1] = 1;
    int64_t r = recurse_forever<N>(depth + 1);
    asm volatile("" ::: "memory");
    return r + pad[0] + pad[N - 1];
}

// The closure of the goroutine that overflows: it prints a line first, so the test sees that
// output before the overflow is not lost.
void overflow_in_goroutine(std::size_t stack, void (*body)()) {
    gp::detail::go_start(std::make_unique<gp::detail::task_of<void (*)()>>(std::move(body)), stack);
    park_forever();
}

void frames_small() {
    gp::println("goroutine runs"_gs);
    recurse_forever<128>(0);
}

// The same without the line: the goroutines of a case that overflows several at once start in
// any order, and a line from one may come after the report of another.
void frames_quiet() {
    recurse_forever<128>(0);
}

void frames_big() {
    gp::println("goroutine runs"_gs);
    // Frames that are most of the way to the guard: the first access past the end of the stack
    // is in the guard, not in some other mapping.
    recurse_forever<100000>(0);
}

// A store to an address that nothing maps (not null: UBSan would stop at that): a fault that
// isn't a stack overflow keeps the action that was there before.
volatile uintptr_t wild_address = 64; // read at run time: a constant address warns at -O2
[[gnu::noinline]] void wild_store() {
    volatile int* p = reinterpret_cast<volatile int*>(wild_address);
    *p = 1;
}

struct dtor_note {
    ~dtor_note() { gp::println("destructor ran"_gs); }
};

[[gnu::noinline]] void panic_deep(int n) {
    dtor_note note;
    if (stop_recursing) {
        return;
    }
    if (n > 0) {
        panic_deep(n - 1);
    }
    gp::panic("boom"_gs);
}

// The hook case: a hook of the test, which says what it was given.
std::atomic<uintptr_t> expect_lo{0};
std::atomic<uintptr_t> expect_hi{0};

void test_hook(const gp::os::stack_fault& f) noexcept {
    bool in_stack =
        f.gp_lo < f.gp_hi && f.gp_sp < f.gp_hi && f.gp_sp + (std::size_t(1) << 20) >= f.gp_lo;
    bool same =
        expect_lo.load() == 0 || (f.gp_lo == expect_lo.load() && f.gp_hi == expect_hi.load());
    const char* msg = in_stack && same ? "hook: sp near the stack, bounds as stack_bounds\n"
                                       : "hook: WRONG bounds\n";
    std::string_view m(msg);
    gp::os::write(gp::os::stdout_fd, m.data(), m.size());
    gp::os::exit_now(7);
}

void hook_body() {
    uintptr_t lo = 0;
    uintptr_t hi = 0;
    gp::os::stack_bounds(lo, hi);
    expect_lo = lo;
    expect_hi = hi;
    recurse_forever<128>(0);
}

// claim_death: the first thread to call it goes on (and may call it again), every other thread
// blocks. Deterministic: the main goroutine claims, a goroutine calls it too and must never come
// back; the main goroutine waits until that goroutine is about to call it and then a good while
// longer (a slow machine can only make the wait longer, never make a bug look right).
void claim_death_case() {
    gp::detail::claim_death();
    gp::detail::claim_death(); // the claimer again: goes on
    static std::atomic<bool> about{false};
    static std::atomic<bool> came_back{false};
    gp::go([] {
        about.store(true);
        gp::detail::claim_death();
        came_back.store(true);
    });
    wait_for([] { return about.load(); });
    gp::waiter w;
    w.gp_park_for(1500ms);
    if (came_back.load()) {
        gp::println("BUG: claim_death returned in a second thread"_gs);
        gp::os::exit_now(1);
    }
    gp::println("second thread is blocked"_gs);
    gp::os::exit_now(0);
}

void run_case(const std::string& name) {
    // Some cases die of SIGSEGV on purpose: no core file (a core_pattern that pipes to
    // systemd-coredump holds a dying process for minutes on a loaded machine).
    gp::os::no_core_dump();
    if (name == "claim-death") {
        claim_death_case();
    } else if (name == "thread-fail-3") {
        // Three goroutines are alive and held: Go's "have N already" counts them and main.
        auto g = std::make_shared<gate>();
        for (int i = 0; i < 3; i++) {
            gp::go([g] { g->pass(); });
        }
        wait_for([&] { return g->arrived.load() == 3; });
        gp::detail::go_start(std::make_unique<gp::detail::task_of<void (*)()>>([] {}),
                             std::size_t(1) << 60);
    } else if (name == "go-panic") {
        gp::println("main starts"_gs);
        gp::go([] {
            gp::println("goroutine panics"_gs);
            panic_deep(3);
        });
        park_forever();
    } else if (name == "go-panic-nested") {
        // A goroutine of a goroutine.
        gp::go([] { gp::go([] { gp::panic("deep"_gs); }); });
        park_forever();
    } else if (name == "go-panic-many") {
        // Eight goroutines panic at the same time: one report, one exit code.
        auto g = std::make_shared<gate>();
        for (int i = 0; i < 8; i++) {
            gp::go([g, i] {
                g->pass();
                gp::panic(int64_t(i));
            });
        }
        wait_for([&] { return g->arrived.load() == 8; });
        g->release();
        park_forever();
    } else if (name == "go-throw") {
        gp::go([] { throw std::runtime_error("from a goroutine"); });
        park_forever();
    } else if (name == "go-fatal") {
        gp::go([] { gp::detail::fatal("a fatal error"); });
        park_forever();
    } else if (name == "go-nil") {
        gp::go([] { gp::panic_nil(); });
        park_forever();
    } else if (name == "main-panic") {
        // The main goroutine panics while another goroutine is alive.
        gp::go([] { park_forever(); });
        gp::panic("main goroutine"_gs);
    } else if (name == "thread-fail") {
        gp::println("before"_gs);
        gp::detail::go_start(std::make_unique<gp::detail::task_of<void (*)()>>([] {}),
                             std::size_t(1) << 60);
        gp::println("not reached"_gs);
    } else if (name == "overflow-go") {
        overflow_in_goroutine(0, frames_small); // the default 8 MiB stack
    } else if (name == "overflow-go-small") {
        overflow_in_goroutine(std::size_t(256) << 10, frames_small);
    } else if (name == "overflow-go-big-frames") {
        overflow_in_goroutine(std::size_t(1) << 20, frames_big);
    } else if (name == "overflow-go-many") {
        // Four goroutines overflow at once: one report.
        for (int i = 0; i < 4; i++) {
            gp::detail::go_start(std::make_unique<gp::detail::task_of<void (*)()>>(frames_quiet),
                                 std::size_t(256) << 10);
        }
        park_forever();
    } else if (name == "overflow-hook-go") {
        gp::os::set_stack_overflow_hook(test_hook);
        overflow_in_goroutine(std::size_t(512) << 10, hook_body);
    } else if (name == "segv-go") {
        gp::go([] { wild_store(); });
        park_forever();
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
    gp::os::exit(0);
}

void tests_main() {
    const auto& args = gp::args();
    // runUnit passes "arg1" and "arg two"; a case is the only argument.
    if (args.size() == 2) {
        run_case(std::string(args[1].gp_view()));
    }
    run_unit_tests();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
