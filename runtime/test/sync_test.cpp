// The unit tests of package sync and sync/atomic's runtime side (gocpp/sync.hpp, gocpp/atomic.hpp,
// src/sync.cpp, src/atomic.cpp), Phase 08 step 8.7.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and with ThreadSanitizer
// (sync_test.go), and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or
// prints each failed check and exits 1.
//
// The structs below are what stdlib/sync's types lower to (same field names and types: the
// natives are templates over them, and a static_assert in each says what it needs).
//
// Nothing here depends on timing. Threads synchronize through atomics and gp::num_blocked() (a
// goroutine that waits in sync is counted there before it sleeps, so a test can wait until it is
// blocked); a wait has a limit of minutes, so a hang is a failure and a slow machine is not.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include "../src/chan_core.hpp"
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <string>
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

#define CHECK_STR(got, want)                                                                       \
    do {                                                                                           \
        std::string gp_got = (got);                                                                \
        report(gp_got == (want), __FILE__, __LINE__, #got " = \"" + gp_got + "\", want " #want);   \
    } while (0)

// wait_for polls pred until it is true, for at most two minutes, and says whether it got true.
template <typename P>
bool wait_for(P pred) {
    auto end = std::chrono::steady_clock::now() + 2min;
    while (!pred()) {
        if (std::chrono::steady_clock::now() > end) {
            return false;
        }
        std::this_thread::sleep_for(100us);
    }
    return true;
}

// blocked_is waits until n goroutines wait in sync.
bool blocked_is(int64_t n) {
    return wait_for([n] { return gp::num_blocked() == n; });
}

// panic_text runs f and returns the text of the panic it ends with, or "(no panic)".
std::string panic_text(const std::function<void()>& f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        return std::string(gp::detail::panic_text(e).gp_view());
    }
    return "(no panic)";
}

// A group of goroutines that the test waits for.
struct group {
    std::atomic<int> running{0};

    template <typename F>
    void go(F f) {
        running++;
        gp::go([this, f = std::move(f)]() mutable {
            f();
            running--;
        });
    }

    bool wait() {
        return wait_for([this] { return running.load() == 0; });
    }
};

// --- the types of stdlib/sync, as the lowering writes them ---

struct Mutex {
    int32_t state = 0;
    uint32_t sema = 0;
};
struct RWMutex {
    Mutex w;
    uint32_t writerSem = 0;
    uint32_t readerSem = 0;
    int32_t readerCount = 0;
    int32_t readerWait = 0;
};
struct WaitGroup {
    uint64_t state = 0;
    uint32_t sema = 0;
};
struct Cond {
    gp::any L;
    uint32_t wait = 0;
    uint32_t notify = 0;
    uintptr_t checker = 0;
};
struct Once {
    uint32_t done = 0;
    Mutex m;
};

// --- Mutex ---

void test_mutex_single() {
    auto mu = gp::new_<Mutex>();
    gp::sync::mutex_lock(mu);
    CHECK_EQ(mu->state, 1);
    CHECK(!gp::sync::mutex_try_lock(mu));
    gp::sync::mutex_unlock(mu);
    CHECK_EQ(mu->state, 0);
    CHECK(gp::sync::mutex_try_lock(mu));
    CHECK(!gp::sync::mutex_try_lock(mu));
    gp::sync::mutex_unlock(mu);
    CHECK_EQ(mu->state, 0);
    // The object is two words: a copy of a locked mutex is locked, as in Go.
    gp::sync::mutex_lock(mu);
    Mutex copy = *mu;
    auto cp = gp::new_<Mutex>(copy);
    CHECK(!gp::sync::mutex_try_lock(cp));
    gp::sync::mutex_unlock(mu);
    CHECK(gp::sync::mutex_try_lock(mu));
    gp::sync::mutex_unlock(mu);
    gp::sync::mutex_unlock(cp);
    CHECK(gp::sync::mutex_try_lock(cp));
    // A nil mutex panics like Go (nil pointer dereference).
    gp::ptr<Mutex> nil;
    CHECK_STR(panic_text([&] { gp::sync::mutex_lock(nil); }),
              "runtime error: invalid memory address or nil pointer dereference");
    static_assert(sizeof(Mutex) == 8);
}

// Many goroutines increment a plain counter under the mutex: with mutual exclusion nothing is
// lost (TSan sees a data race when the exclusion or the happens-before edges are missing).
void test_mutex_exclusion() {
    auto mu = gp::new_<Mutex>();
    int64_t counter = 0;
    group g;
    const int goroutines = 16;
    const int iters = 20000;
    for (int i = 0; i < goroutines; i++) {
        g.go([&] {
            for (int k = 0; k < iters; k++) {
                gp::sync::mutex_lock(mu);
                counter++;
                gp::sync::mutex_unlock(mu);
            }
        });
    }
    CHECK(g.wait());
    CHECK_EQ(counter, int64_t(goroutines) * iters);
    CHECK_EQ(mu->state, 0);
}

// A goroutine that waits is counted as blocked (with Go's reason), and is not any more when the
// unlocker has woken it; one goroutine may lock and another unlock.
void test_mutex_blocked() {
    auto mu = gp::new_<Mutex>();
    gp::sync::mutex_lock(mu);
    std::atomic<gp::detail::goroutine*> gr{nullptr};
    std::atomic<bool> got{false};
    group g;
    g.go([&] {
        gr.store(&gp::detail::this_goroutine());
        gp::sync::mutex_lock(mu);
        got.store(true);
        gp::sync::mutex_unlock(mu);
    });
    CHECK(blocked_is(1));
    CHECK(std::string(gr.load()->reason()) == "sync.Mutex.Lock");
    CHECK(!got.load());
    // Unlock from another thread than the one that locked: allowed for a sync.Mutex.
    std::thread([&] { gp::sync::mutex_unlock(mu); }).join();
    CHECK(wait_for([&] { return got.load(); }));
    CHECK(g.wait());
    CHECK_EQ(gp::num_blocked(), int64_t(0));
    CHECK_EQ(mu->state, 0);
}

// Starvation: a goroutine that locks and unlocks in a tight loop does not keep a waiter out for
// ever. The waiter must get the mutex while the other still runs (it stops only after).
void test_mutex_starvation() {
    auto mu = gp::new_<Mutex>();
    std::atomic<bool> waiter_in{false};
    std::atomic<bool> hog_running{false};
    std::atomic<int64_t> hog_rounds{0};
    group g;
    g.go([&] {
        hog_running.store(true);
        while (!waiter_in.load()) {
            gp::sync::mutex_lock(mu);
            hog_rounds++;
            gp::sync::mutex_unlock(mu);
        }
    });
    CHECK(wait_for([&] { return hog_running.load(); }));
    g.go([&] {
        gp::sync::mutex_lock(mu);
        waiter_in.store(true);
        gp::sync::mutex_unlock(mu);
    });
    CHECK(g.wait());
    CHECK(waiter_in.load());
    CHECK_EQ(mu->state, 0);
}

// The clock of the starvation tests: the test says what time it is, and holds the waiter (the one
// goroutine with is_waiter) in the clock call that follows its wake-up.
namespace starve {

// The time at which W starts to wait; the clock is the test's own.
constexpr int64_t start_ns = 1000000;
std::atomic<int64_t> now_ns{start_ns};
std::atomic<int> waiter_calls{0};
std::atomic<bool> in_wake_call{false};
std::atomic<bool> go_on{false};
thread_local bool is_waiter = false;

int64_t clock() noexcept {
    if (is_waiter && waiter_calls.fetch_add(1) == 1) {
        // The second call is the one after the wake-up (the first one starts the wait): hold the
        // waiter here until the test has taken the mutex that the waiter was woken for.
        in_wake_call.store(true);
        wait_for([] { return go_on.load(); });
    }
    return now_ns.load();
}

void reset() {
    now_ns.store(start_ns);
    waiter_calls.store(0);
    in_wake_call.store(false);
    go_on.store(false);
}

// The mutex bits (internal/sync/mutex.go): locked 1, woken 2, starving 4, the waiters from bit 3.
constexpr int32_t starving_bit = 4;

} // namespace starve

// A waiter that was woken and lost the mutex to a goroutine that arrived in the meantime, after
// it had waited longer than the threshold, puts the mutex in starvation mode: the next Unlock
// hands the mutex to the first waiter, and a newcomer queues behind it. The clock is the test's,
// and the waiter is held after its wake-up until the mutex is taken from under it: no sleep decides
// anything. A second waiter X is queued behind the first one W (W must go back to the front of the
// queue when it waits again, ahead of X), and it counts for the threshold (base plus one waiter).
//
// waited_ns is how long W has waited when it is woken. sticky also checks that the mutex stays in
// starvation mode when W takes it (W is held in its critical section, the clock is back at the
// start of its wait: once W has seen the long wait it does not forget); without sticky this
// goroutine unlocks and comes back at once, as a newcomer, and must queue behind W and X.
void test_mutex_starvation_handoff(int64_t waited_ns, bool starving_expected, bool sticky) {
    starve::reset();
    gp::sync::detail::set_clock_for_test(&starve::clock);
    auto mu = gp::new_<Mutex>();
    std::vector<int> order;
    std::mutex order_mu;
    auto note = [&](int who) {
        std::lock_guard<std::mutex> lk(order_mu);
        order.push_back(who);
    };
    std::atomic<bool> w_holds{false};
    std::atomic<bool> w_release{false};
    gp::sync::mutex_lock(mu);
    group g;
    g.go([&] {
        // A goroutine's thread is reused by the next goroutine: the flag goes with this body.
        struct flag_guard {
            flag_guard() { starve::is_waiter = true; }
            ~flag_guard() { starve::is_waiter = false; }
        } waiter_flag;
        gp::sync::mutex_lock(mu);
        note(1);
        if (sticky) {
            w_holds.store(true);
            wait_for([&] { return w_release.load(); });
        }
        gp::sync::mutex_unlock(mu);
    });
    CHECK(blocked_is(1));
    g.go([&] {
        gp::sync::mutex_lock(mu);
        note(2);
        gp::sync::mutex_unlock(mu);
    });
    CHECK(blocked_is(2));
    starve::now_ns.store(starve::start_ns + waited_ns);
    gp::sync::mutex_unlock(mu); // wakes W, the first waiter
    CHECK(wait_for([] { return starve::in_wake_call.load(); }));
    // W was woken for the mutex and is held before it tries; this goroutine takes it.
    gp::sync::mutex_lock(mu);
    starve::go_on.store(true);
    // W finds the mutex locked and queues again, at the front: with the long wait it switches the
    // mutex to starvation mode on its way.
    CHECK(wait_for([&] { return (gp::sync::detail::at(mu->state).load() >> 3) == 2; }));
    bool starving = (gp::sync::detail::at(mu->state).load() & starve::starving_bit) != 0;
    CHECK_EQ(starving, starving_expected);
    CHECK(blocked_is(2));
    if (sticky) {
        starve::now_ns.store(starve::start_ns);
    }
    gp::sync::mutex_unlock(mu);
    if (sticky) {
        CHECK(wait_for([&] { return w_holds.load(); }));
        bool still = (gp::sync::detail::at(mu->state).load() & starve::starving_bit) != 0;
        CHECK_EQ(still, starving_expected);
        w_release.store(true);
        gp::sync::mutex_lock(mu);
    } else {
        gp::sync::mutex_lock(mu);
    }
    note(3);
    // This goroutine holds the mutex, so W and X that have not taken it yet end up in the queue:
    // wait for that before the unlock (W that was woken a second time is counted in the state but
    // not in the queue for a moment, and the unlock would then wake X first).
    int64_t not_yet_in = 2;
    {
        std::lock_guard<std::mutex> lk(order_mu);
        not_yet_in -= int64_t(order.size()) - 1;
    }
    CHECK(
        wait_for([&] { return gp::sync::detail::sema_queued_for_test(&mu->sema) == not_yet_in; }));
    gp::sync::mutex_unlock(mu);
    CHECK(g.wait());
    if (starving_expected) {
        CHECK((order == std::vector<int>{1, 2, 3}));
    } else {
        // Without starvation the newcomer may take the mutex before the woken waiter: only W and X
        // keep their order.
        order.erase(std::remove(order.begin(), order.end(), 3), order.end());
        CHECK((order == std::vector<int>{1, 2}));
    }
    CHECK_EQ(mu->state, 0);
    gp::sync::detail::set_clock_for_test(nullptr);
}

void test_mutex_starvation_handoffs() {
    using gp::sync::detail::starvation_base_ns;
    using gp::sync::detail::starvation_per_waiter_ns;
    // The threshold of ADR-040 (Go's 1 ms plus 100 us for each waiting goroutine): the cases below
    // are made from the constants, so a changed constant is caught here.
    CHECK_EQ(starvation_base_ns, int64_t(1000000));
    CHECK_EQ(starvation_per_waiter_ns, int64_t(100000));
    // X is the one waiter that is queued when W is woken: the threshold is base + one waiter.
    const int64_t threshold = starvation_base_ns + starvation_per_waiter_ns;
    // Not longer than the threshold: no starvation; one nanosecond longer: starvation.
    test_mutex_starvation_handoff(0, false, false);
    test_mutex_starvation_handoff(threshold, false, false);
    test_mutex_starvation_handoff(threshold + 1, true, false);
    test_mutex_starvation_handoff(3600LL * 1000000000LL, true, true);
    for (int round = 0; round < 25; round++) {
        // The newcomer's race against the waiter is decided by the hand-off in every round; a
        // hand-off that is missing is seen in most rounds, so in 25 of them for certain.
        test_mutex_starvation_handoff(3600LL * 1000000000LL, true, false);
        if (failures.load() != 0) {
            break; // a broken hand-off would wait out every round
        }
    }
}

// A hand-off gives the unit of the semaphore to the first waiter in the release itself (Go's
// semrelease with handoff: the release takes the unit for the waiter), so nobody who comes in
// before the waiter has run can take it. A plain release leaves the unit for whoever is first.
void test_sema_handoff_takes_unit() {
    for (int round = 0; round < 50; round++) {
        uint32_t s = 0;
        std::atomic<int> got{0};
        group g;
        g.go([&] {
            gp::sync::detail::sema_acquire(&s, "test");
            got++;
        });
        CHECK(blocked_is(1));
        gp::sync::detail::sema_release(&s, true);
        // Read before the waiter can have run: the unit is gone already.
        CHECK_EQ(gp::sync::detail::at(s).load(), 0u);
        CHECK(g.wait());
        CHECK_EQ(got.load(), 1);
    }
}

// The hand-off is one step, deterministically: the unit never shows in the count of the semaphore
// while a waiter is queued (an observer thread that reads the word all the time never sees 1: the
// release that adds the unit first and takes it for the waiter later shows it, and a newcomer of a
// starving mutex could take it then), and a hand-off release with nobody queued leaves exactly one
// unit for the next acquire.
void test_sema_handoff_is_one_step() {
    uint32_t word = 0;
    std::atomic<bool> stop{false};
    std::atomic<int64_t> seen{0};
    std::atomic<bool> watching{false};
    std::thread observer([&] {
        watching.store(true);
        while (!stop.load()) {
            if (gp::sync::detail::at(word).load() != 0) {
                seen++;
            }
        }
    });
    CHECK(wait_for([&] { return watching.load(); }));
    for (int round = 0; round < 300; round++) {
        std::atomic<int> got{0};
        group g;
        g.go([&] {
            gp::sync::detail::sema_acquire(&word, "test");
            got++;
        });
        CHECK(wait_for([&] { return gp::sync::detail::sema_queued_for_test(&word) == 1; }));
        gp::sync::detail::sema_release(&word, true);
        CHECK(g.wait());
        CHECK_EQ(got.load(), 1);
        CHECK_EQ(gp::sync::detail::at(word).load(), 0u);
    }
    stop.store(true);
    observer.join();
    CHECK_EQ(seen.load(), int64_t(0));

    uint32_t lone = 0;
    gp::sync::detail::sema_release(&lone, true);
    CHECK_EQ(gp::sync::detail::at(lone).load(), 1u);
    gp::sync::detail::sema_acquire(&lone, "test");
    CHECK_EQ(gp::sync::detail::at(lone).load(), 0u);
}

// Wake-ups are not lost: n goroutines wait in a chain, each unlocks for the next.
void test_mutex_chain() {
    auto mu = gp::new_<Mutex>();
    const int n = 64;
    gp::sync::mutex_lock(mu);
    std::atomic<int> done{0};
    std::vector<int> order;
    group g;
    for (int i = 0; i < n; i++) {
        g.go([&, i] {
            gp::sync::mutex_lock(mu);
            order.push_back(i);
            done++;
            gp::sync::mutex_unlock(mu);
        });
    }
    CHECK(blocked_is(n));
    gp::sync::mutex_unlock(mu);
    CHECK(g.wait());
    CHECK_EQ(done.load(), n);
    std::sort(order.begin(), order.end());
    bool all = static_cast<int>(order.size()) == n;
    for (int i = 0; i < n && all; i++) {
        all = order[i] == i;
    }
    CHECK(all);
}

// --- the semaphore ---

void test_sema() {
    uint32_t s = 0;
    // A release before the acquire is not lost.
    gp::sync::detail::sema_release(&s);
    gp::sync::detail::sema_acquire(&s, "test");
    CHECK_EQ(s, 0u);
    // Waiters on one word are woken in order; two words in the same bucket are told apart.
    uint32_t a = 0;
    uint32_t b = 0;
    std::atomic<int> got_a{0};
    std::atomic<int> got_b{0};
    group g;
    g.go([&] {
        gp::sync::detail::sema_acquire(&a, "test");
        got_a++;
    });
    CHECK(blocked_is(1));
    g.go([&] {
        gp::sync::detail::sema_acquire(&b, "test");
        got_b++;
    });
    CHECK(blocked_is(2));
    gp::sync::detail::sema_release(&b);
    CHECK(wait_for([&] { return got_b.load() == 1; }));
    CHECK_EQ(got_a.load(), 0);
    gp::sync::detail::sema_release(&a, true); // hand-off
    CHECK(g.wait());
    CHECK_EQ(got_a.load(), 1);
    CHECK_EQ(a, 0u);
    CHECK_EQ(b, 0u);
    // FIFO among the waiters of one word.
    std::vector<int> order;
    std::atomic<int> queued{0};
    for (int i = 0; i < 5; i++) {
        g.go([&, i] {
            gp::sync::detail::sema_acquire(&a, "test");
            order.push_back(i);
            queued--;
        });
        queued++;
        CHECK(blocked_is(i + 1));
    }
    for (int i = 0; i < 5; i++) {
        gp::sync::detail::sema_release(&a);
        CHECK(wait_for([&] { return queued.load() == 4 - i; }));
    }
    CHECK(g.wait());
    CHECK((order == std::vector<int>{0, 1, 2, 3, 4}));
}

// --- RWMutex ---

void test_rwmutex_single() {
    auto rw = gp::new_<RWMutex>();
    gp::sync::rw_rlock(rw);
    gp::sync::rw_rlock(rw);
    CHECK(!gp::sync::rw_try_lock(rw));
    CHECK(gp::sync::rw_try_rlock(rw));
    gp::sync::rw_runlock(rw);
    gp::sync::rw_runlock(rw);
    gp::sync::rw_runlock(rw);
    CHECK(gp::sync::rw_try_lock(rw));
    CHECK(!gp::sync::rw_try_lock(rw));
    CHECK(!gp::sync::rw_try_rlock(rw));
    gp::sync::rw_unlock(rw);
    gp::sync::rw_lock(rw);
    gp::sync::rw_unlock(rw);
    CHECK_EQ(rw->readerCount, 0);
    CHECK_EQ(rw->w.state, 0);
}

// Readers share the lock; a writer excludes everyone; the counters stay consistent.
void test_rwmutex_exclusion() {
    auto rw = gp::new_<RWMutex>();
    int64_t value = 0;
    std::atomic<int> readers_in{0};
    std::atomic<int> max_readers{0};
    std::atomic<int> writers_in{0};
    std::atomic<bool> broken{false};
    group g;
    const int rounds = 5000;
    for (int w = 0; w < 4; w++) {
        g.go([&] {
            for (int k = 0; k < rounds; k++) {
                gp::sync::rw_lock(rw);
                if (writers_in++ != 0 || readers_in.load() != 0) {
                    broken = true;
                }
                value++;
                writers_in--;
                gp::sync::rw_unlock(rw);
            }
        });
    }
    for (int r = 0; r < 8; r++) {
        g.go([&] {
            for (int k = 0; k < rounds; k++) {
                gp::sync::rw_rlock(rw);
                int now = ++readers_in;
                int seen = max_readers.load();
                while (now > seen && !max_readers.compare_exchange_weak(seen, now)) {
                }
                if (writers_in.load() != 0) {
                    broken = true;
                }
                (void)value;
                readers_in--;
                gp::sync::rw_runlock(rw);
            }
        });
    }
    CHECK(g.wait());
    CHECK(!broken.load());
    CHECK_EQ(value, int64_t(4) * rounds);
    CHECK_EQ(rw->readerCount, 0);
    CHECK_EQ(rw->readerWait, 0);
    CHECK_EQ(rw->w.state, 0);
}

// Writer preference: once a writer waits, a new reader waits too, and the writer gets the lock
// before that reader (RLock after a blocked Lock does not overtake it).
void test_rwmutex_writer_preference() {
    auto rw = gp::new_<RWMutex>();
    gp::sync::rw_rlock(rw); // reader 1 holds the lock
    std::atomic<int> step{0};
    std::vector<std::string> order;
    std::atomic<bool> writer_started{false};
    group g;
    g.go([&] {
        writer_started.store(true);
        gp::sync::rw_lock(rw); // waits for reader 1
        order.push_back("writer");
        gp::sync::rw_unlock(rw);
    });
    CHECK(blocked_is(1));
    CHECK(rw->readerCount < 0); // a writer is pending
    // A new reader can't get in, not even TryRLock.
    CHECK(!gp::sync::rw_try_rlock(rw));
    g.go([&] {
        gp::sync::rw_rlock(rw);
        order.push_back("reader2");
        gp::sync::rw_runlock(rw);
    });
    CHECK(blocked_is(2));
    gp::sync::rw_runlock(rw); // reader 1 leaves: the writer goes first, then reader 2
    CHECK(g.wait());
    CHECK((order == std::vector<std::string>{"writer", "reader2"}));
    CHECK_EQ(rw->readerCount, 0);
    (void)step;
}

// A writer that waits for readers is woken by the last one leaving, with many readers.
void test_rwmutex_many_readers() {
    auto rw = gp::new_<RWMutex>();
    const int n = 32;
    std::atomic<int> in{0};
    std::atomic<bool> release{false};
    group g;
    for (int i = 0; i < n; i++) {
        g.go([&] {
            gp::sync::rw_rlock(rw);
            in++;
            wait_for([&] { return release.load(); });
            gp::sync::rw_runlock(rw);
        });
    }
    CHECK(wait_for([&] { return in.load() == n; }));
    std::atomic<bool> writer_done{false};
    g.go([&] {
        gp::sync::rw_lock(rw);
        writer_done.store(true);
        gp::sync::rw_unlock(rw);
    });
    CHECK(blocked_is(1));
    CHECK(!writer_done.load());
    release.store(true);
    CHECK(g.wait());
    CHECK(writer_done.load());
    CHECK_EQ(rw->readerCount, 0);
    CHECK_EQ(rw->readerWait, 0);
}

// --- WaitGroup ---

void test_waitgroup() {
    auto wg = gp::new_<WaitGroup>();
    // Wait with a zero counter returns at once.
    gp::sync::wg_wait(wg);
    gp::sync::wg_add(wg, 3);
    gp::sync::wg_add(wg, -1);
    gp::sync::wg_add(wg, -2);
    gp::sync::wg_wait(wg);
    CHECK_EQ(wg->state, uint64_t(0));
    // The counter can't go negative.
    CHECK_STR(panic_text([&] { gp::sync::wg_add(wg, -1); }), "sync: negative WaitGroup counter");
    // (the state is left as Go leaves it: negative; use a new one)
    auto w2 = gp::new_<WaitGroup>();
    // Fan-out and fan-in, with several waiters.
    const int n = 50;
    gp::sync::wg_add(w2, n);
    std::atomic<int> finished{0};
    std::atomic<int> woke{0};
    group g;
    for (int i = 0; i < 3; i++) {
        g.go([&] {
            gp::sync::wg_wait(w2);
            CHECK_EQ(finished.load(), n);
            woke++;
        });
    }
    CHECK(blocked_is(3));
    for (int i = 0; i < n; i++) {
        g.go([&] {
            finished++;
            gp::sync::wg_add(w2, -1);
        });
    }
    CHECK(g.wait());
    CHECK_EQ(woke.load(), 3);
    CHECK_EQ(w2->state, uint64_t(0));
    // Reuse: the same WaitGroup again, after the waiters have returned.
    gp::sync::wg_add(w2, 1);
    g.go([&] { gp::sync::wg_wait(w2); });
    CHECK(blocked_is(1));
    gp::sync::wg_add(w2, -1);
    CHECK(g.wait());
    static_assert(sizeof(WaitGroup) == 16);
}

void test_waitgroup_misuse() {
    auto wg = gp::new_<WaitGroup>();
    gp::sync::wg_add(wg, 1);
    group g;
    g.go([&] { gp::sync::wg_wait(wg); });
    CHECK(blocked_is(1));
    // Add with a positive delta while the counter is above zero and a waiter waits is fine; the
    // misuse is a waiter that sees the counter go from 0 to delta.
    gp::sync::wg_add(wg, 1);
    gp::sync::wg_add(wg, -1);
    gp::sync::wg_add(wg, -1);
    CHECK(g.wait());
    // The state a Wait leaves when the counter is zero and a waiter is counted: Add > 0 panics.
    auto bad = gp::new_<WaitGroup>();
    bad->state = uint64_t(1); // one waiter, counter 0
    CHECK_STR(panic_text([&] { gp::sync::wg_add(bad, 1); }),
              "sync: WaitGroup misuse: Add called concurrently with Wait");
}

// --- Cond ---

void test_cond() {
    auto mu = gp::new_<Mutex>();
    auto c = gp::new_<Cond>();
    // The ticket scheme: a Signal before the Wait's park is not lost.
    uint32_t t = gp::sync::cond_add(c);
    gp::sync::cond_signal(c);
    gp::sync::cond_wait(c, t); // returns at once: ticket t was notified
    CHECK_EQ(c->notify, 1u);
    // Signal wakes one waiter, in order; Broadcast wakes the rest.
    std::atomic<int> woke{0};
    std::vector<int> order;
    group g;
    const int n = 6;
    for (int i = 0; i < n; i++) {
        g.go([&, i] {
            gp::sync::mutex_lock(mu);
            uint32_t tk = gp::sync::cond_add(c);
            gp::sync::mutex_unlock(mu);
            gp::sync::cond_wait(c, tk);
            gp::sync::mutex_lock(mu);
            order.push_back(i);
            woke++;
            gp::sync::mutex_unlock(mu);
        });
        CHECK(blocked_is(i + 1));
    }
    gp::sync::cond_signal(c);
    CHECK(wait_for([&] { return woke.load() == 1; }));
    gp::sync::cond_signal(c);
    CHECK(wait_for([&] { return woke.load() == 2; }));
    CHECK_EQ(gp::num_blocked(), int64_t(n - 2));
    gp::sync::cond_broadcast(c);
    CHECK(g.wait());
    CHECK_EQ(woke.load(), n);
    // The first two are woken in ticket order.
    CHECK(order[0] == 0 && order[1] == 1);
    // Signal and Broadcast with nobody waiting do nothing.
    gp::sync::cond_signal(c);
    gp::sync::cond_broadcast(c);
    // A Cond that is copied after its first use panics.
    auto copy = gp::new_<Cond>(*c);
    CHECK_STR(panic_text([&] { gp::sync::cond_signal(copy); }), "sync.Cond is copied");
    CHECK_STR(panic_text([&] { gp::sync::cond_add(copy); }), "sync.Cond is copied");
    // A copy made before the first use is fine.
    auto fresh = gp::new_<Cond>();
    auto fresh2 = gp::new_<Cond>(*fresh);
    gp::sync::cond_signal(fresh2);
    CHECK(!(fresh2->checker == 0));
}

// A producer/consumer on one Cond with a plain queue: no lost wake-up, nothing is waited for
// for ever (every consumer gets exactly its share).
void test_cond_queue() {
    auto mu = gp::new_<Mutex>();
    auto c = gp::new_<Cond>();
    std::vector<int> queue;
    int64_t sum = 0;
    int consumed = 0;
    const int items = 2000;
    group g;
    for (int i = 0; i < 4; i++) {
        g.go([&] {
            for (;;) {
                gp::sync::mutex_lock(mu);
                while (queue.empty() && consumed < items) {
                    uint32_t tk = gp::sync::cond_add(c);
                    gp::sync::mutex_unlock(mu);
                    gp::sync::cond_wait(c, tk);
                    gp::sync::mutex_lock(mu);
                }
                if (consumed == items) {
                    gp::sync::mutex_unlock(mu);
                    return;
                }
                sum += queue.back();
                queue.pop_back();
                consumed++;
                bool last = consumed == items;
                gp::sync::mutex_unlock(mu);
                if (last) {
                    gp::sync::cond_broadcast(c);
                }
            }
        });
    }
    for (int i = 1; i <= items; i++) {
        gp::sync::mutex_lock(mu);
        queue.push_back(i);
        gp::sync::mutex_unlock(mu);
        gp::sync::cond_signal(c);
    }
    CHECK(g.wait());
    CHECK_EQ(sum, int64_t(items) * (items + 1) / 2);
    CHECK_EQ(consumed, items);
}

// --- Once natives ---

void test_once() {
    auto o = gp::new_<Once>();
    CHECK(!gp::sync::once_done(o));
    gp::sync::once_set(o);
    CHECK(gp::sync::once_done(o));
}

// --- sync/atomic ---

void test_atomic_functions() {
    using namespace gp::sync_atomic;
    auto i32 = gp::new_<int32_t>(5);
    CHECK_EQ(fn_add(i32, int32_t(3)), int32_t(8));
    CHECK_EQ(fn_add(i32, int32_t(-10)), int32_t(-2));
    CHECK_EQ(fn_load(i32), int32_t(-2));
    fn_store(i32, int32_t(7));
    CHECK_EQ(fn_swap(i32, int32_t(9)), int32_t(7));
    CHECK(fn_cas(i32, int32_t(9), int32_t(10)));
    CHECK(!fn_cas(i32, int32_t(9), int32_t(11)));
    CHECK_EQ(fn_load(i32), int32_t(10));
    CHECK_EQ(fn_and(i32, int32_t(6)), int32_t(10)); // returns the old value
    CHECK_EQ(fn_load(i32), int32_t(2));
    CHECK_EQ(fn_or(i32, int32_t(5)), int32_t(2));
    CHECK_EQ(fn_load(i32), int32_t(7));
    // Wrap-around, as in Go.
    auto u32 = gp::new_<uint32_t>(0u);
    CHECK_EQ(fn_add(u32, ~uint32_t(0)), ~uint32_t(0));
    CHECK_EQ(fn_add(u32, uint32_t(1)), 0u);
    auto m = gp::new_<int32_t>(INT32_MAX);
    CHECK_EQ(fn_add(m, int32_t(1)), INT32_MIN);
    auto u64 = gp::new_<uint64_t>(uint64_t(1) << 63);
    CHECK_EQ(fn_add(u64, uint64_t(1) << 63), uint64_t(0));
    auto up = gp::new_<uintptr_t>(uintptr_t(0));
    CHECK_EQ(fn_add(up, uintptr_t(4)), uintptr_t(4));
    // A nil pointer panics.
    gp::ptr<int32_t> nil;
    CHECK_STR(panic_text([&] { fn_load(nil); }),
              "runtime error: invalid memory address or nil pointer dereference");
    // A pointer to a field of a boxed struct (an aliasing pointer).
    struct S {
        int64_t a = 0;
        int64_t b = 0;
    };
    auto s = gp::new_<S>();
    gp::ptr<int64_t> pb(s, &s->b);
    CHECK_EQ(fn_add(pb, int64_t(5)), int64_t(5));
    CHECK_EQ(s->b, int64_t(5));
    CHECK_EQ(s->a, int64_t(0));
}

struct I32 {
    int32_t v = 0;
};
struct U64 {
    uint64_t v = 0;
};
struct B {
    uint32_t v = 0;
};

void test_atomic_typed() {
    using namespace gp::sync_atomic;
    auto a = gp::new_<I32>();
    CHECK_EQ(ty_add(a, int32_t(4)), int32_t(4));
    ty_store(a, int32_t(3));
    CHECK_EQ(ty_load(a), int32_t(3));
    CHECK_EQ(ty_swap(a, int32_t(8)), int32_t(3));
    CHECK(ty_cas(a, int32_t(8), int32_t(1)));
    CHECK(!ty_cas(a, int32_t(8), int32_t(2)));
    CHECK_EQ(ty_or(a, int32_t(6)), int32_t(1));
    CHECK_EQ(ty_and(a, int32_t(3)), int32_t(7));
    CHECK_EQ(ty_load(a), int32_t(3));
    auto u = gp::new_<U64>();
    CHECK_EQ(ty_add(u, ~uint64_t(0)), ~uint64_t(0));
    CHECK_EQ(ty_add(u, uint64_t(2)), uint64_t(1));
    auto b = gp::new_<B>();
    CHECK_EQ(ty_load(b), 0u);
    CHECK(ty_cas(b, 0u, 1u));
    CHECK_EQ(ty_swap(b, 0u), 1u);
}

// Counters from many goroutines are exact, CAS loops make progress, and a Swap hands each value
// to one goroutine only.
void test_atomic_concurrent() {
    using namespace gp::sync_atomic;
    auto n = gp::new_<int64_t>(0);
    auto t = gp::new_<U64>();
    auto cas_total = gp::new_<int64_t>(0);
    auto ticket = gp::new_<int64_t>(0);
    const int goroutines = 16;
    const int iters = 50000;
    std::vector<std::atomic<int>> seen(static_cast<size_t>(goroutines) * 100);
    group g;
    for (int i = 0; i < goroutines; i++) {
        g.go([&] {
            for (int k = 0; k < iters; k++) {
                fn_add(n, int64_t(1));
                ty_add(t, uint64_t(2));
                // A CAS loop that adds 3.
                for (;;) {
                    int64_t old = fn_load(cas_total);
                    if (fn_cas(cas_total, old, old + 3)) {
                        break;
                    }
                }
            }
            for (int k = 0; k < 100; k++) {
                int64_t mine = fn_add(ticket, int64_t(1)) - 1;
                seen[static_cast<size_t>(mine)]++;
            }
        });
    }
    CHECK(g.wait());
    CHECK_EQ(*n, int64_t(goroutines) * iters);
    CHECK_EQ(t->v, uint64_t(goroutines) * iters * 2);
    CHECK_EQ(*cas_total, int64_t(goroutines) * iters * 3);
    bool each_once = true;
    for (auto& s : seen) {
        each_once = each_once && s.load() == 1;
    }
    CHECK(each_once);
}

// Publication through an atomic: a goroutine writes plain memory and then does an atomic
// operation; this goroutine sees the effect of the operation and reads the plain memory. Go's
// memory model makes every atomic operation a synchronizing one (seq_cst), so the plain read
// is ordered after the plain write; ThreadSanitizer reports a data race when the operation is
// weaker than that (a relaxed fetch_add, for example): the unit tests of the counters above cannot
// see it, they only add up. The check is in the TSan build (sync_test.go: TestSyncTSan), where a
// race is a warning on stderr; in the other builds the values are still checked.
template <typename Flag, typename Make, typename Publish, typename Seen>
void publication(Make make, Publish publish, Seen seen) {
    for (int round = 0; round < 20; round++) {
        Flag flag = make();
        int64_t payload = 0; // plain
        group g;
        g.go([&] {
            payload = 42;
            publish(flag);
        });
        CHECK(wait_for([&] { return seen(flag); }));
        CHECK_EQ(payload, int64_t(42));
        CHECK(g.wait());
    }
}

void test_atomic_publication() {
    using namespace gp::sync_atomic;
    auto make_fn = [] { return gp::new_<int64_t>(0); };
    auto fn_seen = [](const gp::ptr<int64_t>& f) { return fn_load(f) != 0; };
    publication<gp::ptr<int64_t>>(make_fn, [](auto& f) { fn_add(f, int64_t(1)); }, fn_seen);
    publication<gp::ptr<int64_t>>(make_fn, [](auto& f) { fn_store(f, int64_t(1)); }, fn_seen);
    publication<gp::ptr<int64_t>>(make_fn, [](auto& f) { fn_swap(f, int64_t(1)); }, fn_seen);
    publication<gp::ptr<int64_t>>(
        make_fn, [](auto& f) { fn_cas(f, int64_t(0), int64_t(1)); }, fn_seen);
    publication<gp::ptr<int64_t>>(make_fn, [](auto& f) { fn_or(f, int64_t(1)); }, fn_seen);
    // fn_and clears: the flag starts set.
    publication<gp::ptr<int64_t>>([] { return gp::new_<int64_t>(1); },
                                  [](auto& f) { fn_and(f, int64_t(0)); },
                                  [](const gp::ptr<int64_t>& f) { return fn_load(f) == 0; });
    auto make_ty = [] { return gp::new_<U64>(); };
    auto ty_seen = [](const gp::ptr<U64>& f) { return ty_load(f) != 0; };
    publication<gp::ptr<U64>>(make_ty, [](auto& f) { ty_add(f, uint64_t(1)); }, ty_seen);
    publication<gp::ptr<U64>>(make_ty, [](auto& f) { ty_store(f, uint64_t(1)); }, ty_seen);
    publication<gp::ptr<U64>>(make_ty, [](auto& f) { ty_swap(f, uint64_t(1)); }, ty_seen);
    publication<gp::ptr<U64>>(
        make_ty, [](auto& f) { ty_cas(f, uint64_t(0), uint64_t(1)); }, ty_seen);
    publication<gp::ptr<U64>>(make_ty, [](auto& f) { ty_or(f, uint64_t(1)); }, ty_seen);
    // ty_and clears: the flag starts set.
    publication<gp::ptr<U64>>(
        [] {
            auto f = gp::new_<U64>();
            f->v = 1;
            return f;
        },
        [](auto& f) { ty_and(f, uint64_t(0)); },
        [](const gp::ptr<U64>& f) { return ty_load(f) == 0; });
}

// The spin lock of the pointer operations (atomic.Pointer, atomic.Value, atomic_ptr): a plain
// counter under it is exact, and for TSan the unlock releases what the next lock acquires (a
// relaxed unlock, or a relaxed lock in the contended path, is a data race on the counter).
void test_atomic_spin_lock_orders() {
    using namespace gp::sync_atomic;
    auto lock = gp::new_<uint32_t>(0);
    int64_t counter = 0; // plain
    group g;
    const int goroutines = 8;
    const int iters = 20000;
    for (int i = 0; i < goroutines; i++) {
        g.go([&] {
            for (int k = 0; k < iters; k++) {
                spin_lock_word(lock);
                counter++;
                spin_unlock_word(lock);
            }
        });
    }
    CHECK(g.wait());
    CHECK_EQ(counter, int64_t(goroutines) * iters);
}

// A pointer stored in an atomic.Pointer[T] publishes what its writer wrote to the object before.
void test_atomic_ptr_publication() {
    struct Box {
        int64_t value = 0;
    };
    for (int round = 0; round < 20; round++) {
        gp::atomic_ptr<Box> p;
        group g;
        g.go([&] {
            auto box = gp::new_<Box>();
            box->value = 42; // plain
            gp::atomic_store(p, box);
        });
        gp::ptr<Box> got;
        CHECK(wait_for([&] {
            got = gp::atomic_load(p);
            return got != nullptr;
        }));
        CHECK_EQ(got->value, int64_t(42));
        CHECK(g.wait());
    }
}

void test_atomic_pointers() {
    using namespace gp::sync_atomic;
    auto slot = gp::new_<gp::ptr<void>>();
    auto x = gp::unsafe_pointer(gp::new_<int64_t>(1));
    auto y = gp::unsafe_pointer(gp::new_<int64_t>(2));
    CHECK(fn_load_pointer(slot) == nullptr);
    fn_store_pointer(slot, x);
    CHECK(fn_load_pointer(slot) == x);
    CHECK(!fn_cas_pointer(slot, y, y));
    CHECK(fn_cas_pointer(slot, x, y));
    CHECK(fn_load_pointer(slot) == y);
    CHECK(fn_swap_pointer(slot, x) == y);
    CHECK(fn_load_pointer(slot) == x);
    // The counts stay right: x is held by x, by the slot, and nothing else.
    CHECK_EQ(x.gp_shared().use_count(), 2L);
    fn_store_pointer(slot, nullptr);
    CHECK_EQ(x.gp_shared().use_count(), 1L);
}

void test_atomic_ptr() {
    gp::atomic_ptr<int64_t> p;
    CHECK(gp::atomic_load(p) == nullptr);
    auto a = gp::new_<int64_t>(1);
    auto b = gp::new_<int64_t>(2);
    gp::atomic_store(p, a);
    CHECK(gp::atomic_load(p) == a);
    CHECK(!gp::atomic_cas(p, b, b));
    CHECK(gp::atomic_cas(p, a, b));
    CHECK(gp::atomic_swap(p, a) == b);
    CHECK(gp::atomic_load(p) == a);
    // a: the variable, and the atomic_ptr.
    CHECK_EQ(a.gp_shared().use_count(), 2L);
    // Copy before use copies the pointer.
    gp::atomic_ptr<int64_t> q = p;
    CHECK(gp::atomic_load(q) == a);
    CHECK_EQ(a.gp_shared().use_count(), 3L);
    // Concurrent swaps: every pointer is held exactly once at the end, nothing is freed twice.
    auto cells = std::make_shared<std::vector<gp::ptr<int64_t>>>();
    for (int i = 0; i < 8; i++) {
        cells->push_back(gp::new_<int64_t>(i));
    }
    gp::atomic_ptr<int64_t> shared;
    group g;
    for (int i = 0; i < 8; i++) {
        g.go([&, i] {
            gp::ptr<int64_t> mine = (*cells)[static_cast<size_t>(i)];
            for (int k = 0; k < 20000; k++) {
                mine = gp::atomic_swap(shared, mine);
                if (mine == nullptr) {
                    mine = gp::new_<int64_t>(i);
                }
            }
        });
    }
    CHECK(g.wait());
    CHECK(gp::atomic_load(shared) != nullptr);
}

void test_atomic_value_helpers() {
    using namespace gp::sync_atomic;
    gp::any a = gp::to_iface<gp::any>(int64_t(1), &gp::td_int);
    gp::any b = gp::to_iface<gp::any>(int64_t(2), &gp::td_int);
    gp::any s = gp::to_iface<gp::any>(gp::string::gp_copy("x"), &gp::td_string);
    CHECK(same_type(a, b));
    CHECK(!same_type(a, s));
    struct V {
        uint32_t lk = 0;
        gp::any v;
    };
    auto v = gp::new_<V>();
    value_lock(v);
    CHECK_EQ(v->lk, 1u);
    value_unlock(v);
    CHECK_EQ(v->lk, 0u);
}

// --- the cases that end the process ---

[[noreturn]] void run_case(const std::string& name) {
    if (name == "unlock-unlocked") {
        auto mu = gp::new_<Mutex>();
        gp::sync::mutex_unlock(mu);
    } else if (name == "unlock-unlocked-recover") {
        // A fatal error is not a panic: recover can't stop it.
        auto mu = gp::new_<Mutex>();
        try {
            gp::sync::mutex_unlock(mu);
        } catch (...) {
            say("caught\n");
        }
    } else if (name == "unlock-twice") {
        auto mu = gp::new_<Mutex>();
        gp::sync::mutex_lock(mu);
        gp::sync::mutex_unlock(mu);
        gp::sync::mutex_unlock(mu);
    } else if (name == "runlock-unlocked") {
        auto rw = gp::new_<RWMutex>();
        gp::sync::rw_runlock(rw);
    } else if (name == "rwunlock-unlocked") {
        auto rw = gp::new_<RWMutex>();
        gp::sync::rw_unlock(rw);
    } else if (name == "negative-counter") {
        auto wg = gp::new_<WaitGroup>();
        gp::sync::wg_add(wg, -1);
    } else if (name == "one-step") {
        // The one-step hand-off alone, for the mutation checks.
        test_sema_handoff_is_one_step();
        if (failures.load() != 0) {
            say(std::to_string(failures.load()) + " checks failed\n");
            gp::os::exit(1);
        }
    } else if (name == "starvation") {
        // The starvation tests alone, for loops that look for a flaky one.
        test_mutex_starvation_handoffs();
        test_sema_handoff_is_one_step();
        if (failures.load() != 0) {
            say(std::to_string(failures.load()) + " checks failed\n");
            gp::os::exit(1);
        }
    } else if (name == "blocked-forever") {
        auto mu = gp::new_<Mutex>();
        gp::sync::mutex_lock(mu);
        gp::go([mu] { gp::sync::mutex_lock(mu); });
        blocked_is(1);
        say("blocked\n");
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
    gp::os::exit(0);
}

void run_unit_tests() {
    test_mutex_single();
    test_mutex_exclusion();
    test_mutex_blocked();
    test_mutex_starvation();
    test_mutex_starvation_handoffs();
    test_sema_handoff_takes_unit();
    test_sema_handoff_is_one_step();
    test_mutex_chain();
    test_sema();
    test_rwmutex_single();
    test_rwmutex_exclusion();
    test_rwmutex_writer_preference();
    test_rwmutex_many_readers();
    test_waitgroup();
    test_waitgroup_misuse();
    test_cond();
    test_cond_queue();
    test_once();
    test_atomic_functions();
    test_atomic_typed();
    test_atomic_concurrent();
    test_atomic_publication();
    test_atomic_spin_lock_orders();
    test_atomic_ptr_publication();
    test_atomic_pointers();
    test_atomic_ptr();
    test_atomic_value_helpers();
    rt_test::threads_ended();
    if (failures.load() != 0) {
        say(std::to_string(failures.load()) + " of " + std::to_string(checks.load()) +
            " checks failed\n");
        gp::os::exit(1);
    }
    std::string ok = "ok: " + std::to_string(checks.load()) + " checks\n";
    gp::os::write(gp::os::stdout_fd, ok.data(), ok.size());
    gp::os::exit(0);
}

void tests_main() {
    const auto& args = gp::args();
    if (args.size() == 2) {
        run_case(std::string(args[1].gp_view()));
        return;
    }
    run_unit_tests();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
