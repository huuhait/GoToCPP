// The unit tests of the timers and the clocks of package time (gocpp/time.hpp, src/timer.cpp,
// src/time.cpp), Phase 08 step 8.8.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and with ThreadSanitizer
// (time_test.go), and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or
// prints each failed check and exits 1.
//
// The checks are logical, not about the clock: timers fire in the order of their deadlines, a
// timer that is stopped never fires, a channel that nobody holds takes its timer with it. A wait
// has a limit of minutes, so a hang is a failure and a slow machine is not; the only bounds on
// time are lower bounds (a sleep lasts at least as long as asked).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include "../src/chan_core.hpp"
#include "../src/timed_wait.hpp"
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <ctime>
#include <filesystem>
#include <functional>
#include <memory>
#include <mutex>
#include <set>
#include <string>
#include <thread>
#include <tuple>
#include <unordered_set>
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

// blocked_is waits until n goroutines wait in a channel operation.
bool blocked_is(int64_t n) {
    return wait_for([n] { return gp::num_blocked() == n; });
}

// A group of goroutines that the test waits for (a sync.WaitGroup made of a channel).
struct group {
    gp::chan<int64_t> done = gp::make_chan<int64_t>(1 << 16);
    int started = 0;

    template <typename F>
    void go(F f) {
        started++;
        gp::go([this, f = std::move(f)]() mutable {
            f();
            gp::send(done, int64_t(1));
        });
    }

    void wait() {
        for (; started > 0; started--) {
            gp::recv(done);
        }
    }
};

using ns = std::chrono::nanoseconds;

int64_t now_ns() {
    return gp::time::nano();
}

// A timer on a fresh channel of int64: the callback sends how late it is (now minus the deadline it
// is for), without blocking.
struct ticking {
    gp::chan<int64_t> c = gp::make_chan<int64_t>(1);

    static void send(gp::chan<int64_t> ch, int64_t due) { gp::time::try_send(ch, now_ns() - due); }

    void start(int64_t when, int64_t period = 0) {
        gp::time::start_chan<int64_t>(c, when, period, gp::func_of<&ticking::send>());
    }
};

// recv_ready receives a value that is there, without blocking: true if there was one.
bool recv_ready(const gp::chan<int64_t>& c) {
    return c.gp_core()->try_recv(nullptr) == gp::detail::recv_status::value;
}

// armed_is waits until n timers are armed (a timer that fired has left the map when the
// callback ran, but the thread takes its time).
bool armed_is(std::size_t n) {
    return wait_for([n] { return gp::detail::timers_armed() == n; });
}

// --- the clocks ---

void test_clocks() {
    int64_t a = gp::time::nano();
    int64_t b = gp::time::nano();
    CHECK(b >= a);
    CHECK(a > 0);
    auto [sec, nsec, mono] = gp::time::now();
    CHECK(sec > 1577836800); // after 2020
    CHECK(sec < 4102444800); // before 2100
    CHECK(nsec >= 0 && nsec < 1000000000);
    CHECK(mono > 0);
    CHECK(gp::time::nano() >= mono);
    // the wall clock agrees with the C++ library's
    auto wall = std::chrono::duration_cast<std::chrono::seconds>(
                    std::chrono::system_clock::now().time_since_epoch())
                    .count();
    CHECK(wall - sec < 60 && wall - sec >= 0);
}

void test_sleep() {
    int64_t t0 = now_ns();
    gp::time::sleep(0);
    gp::time::sleep(-5);
    gp::time::sleep(INT64_MIN);
    CHECK(now_ns() - t0 < 60'000'000'000LL);
    for (int64_t d : {int64_t(1), int64_t(1000), int64_t(1'000'000), int64_t(7'000'000)}) {
        int64_t s = now_ns();
        gp::time::sleep(d);
        CHECK(now_ns() - s >= d);
    }
    // a sleeping goroutine is not blocked (num_blocked counts goroutines that wait for a wake-up);
    // a stale permit does not cut the sleep short
    std::atomic<bool> sleeping{false};
    std::atomic<int64_t> slept{0};
    // buffered: the goroutine must not park in the send when the main thread is late
    auto done = gp::make_chan<int64_t>(1);
    gp::go([&, done] {
        gp::detail::this_goroutine().park.gp_unpark(); // an early permit
        int64_t s = now_ns();
        sleeping = true;
        gp::time::sleep(20'000'000);
        slept = now_ns() - s;
        gp::send(done, int64_t(1));
    });
    CHECK(wait_for([&] { return sleeping.load(); }));
    CHECK_EQ(gp::num_blocked(), int64_t(0));
    gp::recv(done);
    CHECK(slept.load() >= 20'000'000);
}

// --- the accuracy of timed waits (src/timed_wait.hpp, ADR-132) ---

void test_plan_timed_wait() {
    using gp::detail::plan_timed_wait;
    constexpr int64_t margin = 32'000'000;
    // No margin (Linux): the whole wait, never fine.
    for (int64_t left : {int64_t(1), int64_t(1'000'000), int64_t(3'600'000'000'000LL)}) {
        auto plan = plan_timed_wait(left, 0);
        CHECK_EQ(plan.wait.count(), left);
        CHECK(!plan.fine);
    }
    // Up to two margins: fine to the end.
    for (int64_t left : {int64_t(1), margin, 2 * margin}) {
        auto plan = plan_timed_wait(left, margin);
        CHECK_EQ(plan.wait.count(), left);
        CHECK(plan.fine);
    }
    // More than two margins: coarse, for all but one margin.
    auto plan = plan_timed_wait(2 * margin + 1, margin);
    CHECK_EQ(plan.wait.count(), margin + 1);
    CHECK(!plan.fine);
    plan = plan_timed_wait(3'600'000'000'000LL, margin);
    CHECK_EQ(plan.wait.count(), 3'600'000'000'000LL - margin);
    CHECK(!plan.fine);
    // A wait that ends late by up to a margin (the system's rounding) after each coarse step
    // is still on time at the end, and asks for fine ticks for at most two margins of it.
    for (int64_t total : {int64_t(1'000), int64_t(5'000'000), int64_t(70'000'000),
                          int64_t(1'000'000'000), int64_t(86'400'000'000'000LL)}) {
        for (int64_t slack : {int64_t(0), margin / 2, margin}) {
            int64_t left = total;
            int64_t fine_ns = 0;
            int64_t steps = 0;
            while (left > 0 && steps++ < 10) {
                auto step = plan_timed_wait(left, margin);
                CHECK(step.wait.count() > 0 && step.wait.count() <= left);
                if (step.fine) {
                    fine_ns += step.wait.count();
                    left -= step.wait.count();
                } else {
                    left -= step.wait.count() + slack;
                }
            }
            CHECK_EQ(left, int64_t(0));
            CHECK(fine_ns <= 2 * margin);
            CHECK(steps <= 3);
        }
    }
    // The wait of this system: Linux asks for nothing, Windows for two ticks.
    CHECK(gp::os::timer_margin_ns() == 0 || gp::os::timer_margin_ns() == 32'000'000);
}

void test_fine_timer_scope() {
    using gp::detail::fine_timer_holders;
    using gp::detail::fine_timer_scope;
    CHECK_EQ(fine_timer_holders(), std::size_t(0));
    {
        fine_timer_scope off(false); // asks for nothing
        CHECK_EQ(fine_timer_holders(), std::size_t(0));
        fine_timer_scope a(true);
        CHECK_EQ(fine_timer_holders(), std::size_t(1));
        {
            fine_timer_scope b(true);
            fine_timer_scope c(true);
            CHECK_EQ(fine_timer_holders(), std::size_t(3));
        }
        CHECK_EQ(fine_timer_holders(), std::size_t(1));
    }
    CHECK_EQ(fine_timer_holders(), std::size_t(0));
    // Threads take and give it back at the same time: the count ends at 0, never below.
    std::vector<std::thread> threads;
    for (int i = 0; i < 8; i++) {
        threads.emplace_back([] {
            for (int n = 0; n < 2000; n++) {
                fine_timer_scope s(true);
            }
        });
    }
    for (auto& t : threads) {
        t.join();
    }
    CHECK_EQ(fine_timer_holders(), std::size_t(0));
    // A timed wait that used the scope has given it back (sleep and a timer are the callers).
    gp::time::sleep(2'000'000);
    CHECK_EQ(fine_timer_holders(), std::size_t(0));
}

// --- precise waits (src/timed_wait.hpp, ADR-133) ---

void test_split_precise_wait() {
    using gp::detail::split_precise_wait;
    constexpr int64_t preroll = 300'000;
    // Nothing to wait for.
    for (int64_t ns : {int64_t(0), int64_t(-5)}) {
        auto split = split_precise_wait(ns, preroll);
        CHECK_EQ(split.timer_ns, int64_t(0));
        CHECK_EQ(split.spin_ns, int64_t(0));
    }
    // A short wait is never only a spin (a loop of short sleeps would use a core): the spin is at
    // most half of it, and the timer is the other half.
    for (int64_t ns : {int64_t(1), int64_t(2), int64_t(3), int64_t(100'000), int64_t(300'000),
                       int64_t(500'000), 2 * preroll}) {
        auto split = split_precise_wait(ns, preroll);
        CHECK(split.timer_ns > 0);
        CHECK(split.spin_ns <= ns / 2);
        CHECK(split.timer_ns >= split.spin_ns);
        CHECK_EQ(split.timer_ns + split.spin_ns, ns);
    }
    CHECK_EQ(split_precise_wait(100'000, preroll).spin_ns, int64_t(50'000));
    CHECK_EQ(split_precise_wait(1, preroll).timer_ns, int64_t(1));
    // A long wait: the timer for all but the pre-roll, and the spin for the pre-roll.
    for (int64_t ns : {2 * preroll + 1, int64_t(1'000'000), int64_t(86'400'000'000'000LL)}) {
        auto split = split_precise_wait(ns, preroll);
        CHECK_EQ(split.timer_ns, ns - preroll);
        CHECK_EQ(split.spin_ns, preroll);
    }
    // A pre-roll of 0 is the timer alone.
    auto split = split_precise_wait(1'000, 0);
    CHECK_EQ(split.timer_ns, int64_t(1'000));
    CHECK_EQ(split.spin_ns, int64_t(0));
    CHECK_EQ(gp::detail::precise_preroll_ns, int64_t(300'000));
}

struct counted_slot {
    int id;
    bool open;
};

std::atomic<int> opens{0};
std::atomic<int> closes{0};
std::atomic<bool> open_fails{false};

counted_slot* make_counted() noexcept {
    static std::atomic<int> next{1};
    return new counted_slot{next++, true};
}

counted_slot* make_none() noexcept {
    return nullptr;
}

void close_counted(counted_slot* s) noexcept {
    CHECK(s->open);
    s->open = false;
    closes++;
}

bool open_counted(counted_slot* s) noexcept {
    if (open_fails.load()) {
        return false;
    }
    CHECK(!s->open);
    s->open = true;
    opens++;
    return true;
}

using counted_pool = gp::detail::slot_pool<counted_slot>;

void test_slot_pool() {
    counted_pool pool({&make_counted, &close_counted, &open_counted}, 4);
    CHECK_EQ(pool.made(), std::size_t(0));
    counted_slot* a = pool.acquire();
    counted_slot* b = pool.acquire();
    CHECK(a != nullptr && b != nullptr && a != b);
    CHECK_EQ(pool.made(), std::size_t(2));
    CHECK_EQ(pool.idle(), std::size_t(0));
    // A released slot is the next one handed out: the count of slots is the most that were in use.
    pool.release(a);
    CHECK_EQ(pool.idle(), std::size_t(1));
    CHECK(pool.acquire() == a);
    CHECK_EQ(pool.made(), std::size_t(2));
    pool.release(a);
    pool.release(b);
    delete a;
    delete b;
    // A system that cannot make a slot: null, and nothing counted.
    counted_pool none({&make_none, &close_counted, &open_counted}, 4);
    CHECK(none.acquire() == nullptr);
    CHECK_EQ(none.made(), std::size_t(0));
    // Threads take and give back at the same time: no slot is made twice for one use.
    counted_pool shared({&make_counted, &close_counted, &open_counted}, 64);
    std::vector<std::thread> threads;
    for (int i = 0; i < 8; i++) {
        threads.emplace_back([&shared] {
            for (int n = 0; n < 2000; n++) {
                counted_slot* s = shared.acquire();
                shared.release(s);
            }
        });
    }
    for (auto& t : threads) {
        t.join();
    }
    CHECK(shared.made() >= 1 && shared.made() <= 8);
    CHECK_EQ(shared.idle(), shared.made());
    CHECK_EQ(shared.closed(), std::size_t(0));
    std::size_t made = shared.made();
    for (std::size_t i = 0; i < made; i++) {
        delete shared.acquire();
    }
}

void test_slot_pool_gives_handles_back() {
    // A burst of 10 slots in use at once, then all released: 4 stay open, 6 give their handles
    // back.
    int closes0 = closes.load();
    int opens0 = opens.load();
    counted_pool pool({&make_counted, &close_counted, &open_counted}, 4);
    std::vector<counted_slot*> slots;
    for (int i = 0; i < 10; i++) {
        slots.push_back(pool.acquire());
    }
    CHECK_EQ(pool.made(), std::size_t(10));
    for (counted_slot* s : slots) {
        pool.release(s);
    }
    CHECK_EQ(pool.idle(), std::size_t(4));
    CHECK_EQ(pool.closed(), std::size_t(6));
    CHECK_EQ(closes.load() - closes0, 6);
    // Used again: the open ones first, then the closed ones are opened, and none is made.
    std::vector<counted_slot*> again;
    for (int i = 0; i < 10; i++) {
        counted_slot* s = pool.acquire();
        CHECK(s != nullptr && s->open);
        again.push_back(s);
    }
    CHECK_EQ(pool.made(), std::size_t(10));
    CHECK_EQ(opens.load() - opens0, 6);
    CHECK_EQ(pool.closed(), std::size_t(0));
    // A system that cannot open a slot again: the acquire fails, the slot stays in the pool.
    for (counted_slot* s : again) {
        pool.release(s);
    }
    open_fails = true;
    for (int i = 0; i < 4; i++) {
        CHECK(pool.acquire() != nullptr); // the open ones
    }
    CHECK(pool.acquire() == nullptr);
    CHECK_EQ(pool.closed(), std::size_t(6));
    open_fails = false;
    CHECK(pool.acquire() != nullptr);
    CHECK_EQ(pool.closed(), std::size_t(5));
    CHECK_EQ(pool.made(), std::size_t(10));
    for (counted_slot* s : slots) {
        delete s;
    }
}

// A fake backend for precise_cv, made of the C++ library's mutex and condition variable: the
// logic of precise_cv (the slot that a notify wakes, the fallback) is plain C++ and is tested
// here on every OS, while the real slots (a high-resolution timer on Windows) are tested by
// test/sleep_test.cpp.
struct fake_slot {
    std::mutex mu;
    std::condition_variable cv;
    bool woken = false;
};

struct fake_backend {
    using slot = fake_slot;
    static inline std::atomic<bool> on{true};
    static inline std::atomic<int> waits{0}; // waits that went to a slot
    static inline std::atomic<int> wakes{0}; // wake-ups sent to a slot
    static fake_slot* make() noexcept { return new fake_slot; }
    static void close(fake_slot*) noexcept {}
    static bool open(fake_slot*) noexcept { return true; }
    static gp::detail::slot_pool<fake_slot>& pool() {
        static auto& p = *new gp::detail::slot_pool<fake_slot>({&make, &close, &open}, 64);
        return p;
    }
    static bool available() { return on.load(); }
    static fake_slot* acquire() {
        fake_slot* s = pool().acquire();
        std::lock_guard<std::mutex> lk(s->mu);
        s->woken = false;
        return s;
    }
    static void release(fake_slot* s) { pool().release(s); }
    static void wait(fake_slot* s, int64_t ns) {
        waits++;
        std::unique_lock<std::mutex> lk(s->mu);
        s->cv.wait_for(lk, std::chrono::nanoseconds(ns), [s] { return s->woken; });
    }
    static void wake(fake_slot* s) {
        wakes++;
        std::lock_guard<std::mutex> lk(s->mu);
        s->woken = true;
        s->cv.notify_all();
    }
};

using fake_cv = gp::detail::basic_precise_cv<fake_backend>;

void test_precise_cv_waits_its_time() {
    // No notify: the wait lasts as long as asked, on a slot and on the fallback.
    for (bool on : {true, false}) {
        fake_backend::on = on;
        int before = fake_backend::waits.load();
        fake_cv cv;
        std::mutex mu;
        std::unique_lock<std::mutex> lk(mu);
        int64_t start = now_ns();
        cv.wait_for(lk, 20ms);
        CHECK(now_ns() - start >= 19'000'000);
        CHECK(lk.owns_lock());
        CHECK_EQ(fake_backend::waits.load() - before, on ? 1 : 0);
    }
    fake_backend::on = true;
    // The next wait of the same cv uses a slot again (the first one has let go of it).
    {
        int waits = fake_backend::waits.load();
        fake_cv cv;
        std::mutex mu;
        std::unique_lock<std::mutex> lk(mu);
        cv.wait_for(lk, 2ms);
        cv.wait_for(lk, 2ms);
        CHECK_EQ(fake_backend::waits.load() - waits, 2);
    }
    // Zero or negative: no wait, no slot.
    int before = fake_backend::waits.load();
    fake_cv cv;
    std::mutex mu;
    std::unique_lock<std::mutex> lk(mu);
    cv.wait_for(lk, 0ns);
    cv.wait_for(lk, -5ms);
    CHECK_EQ(fake_backend::waits.load(), before);
}

void test_precise_cv_notify_wakes() {
    // A notify ends a long precise wait: notify_one and notify_all, with the notifier holding the
    // mutex and not holding it.
    for (int variant = 0; variant < 4; variant++) {
        fake_backend::on = true;
        fake_cv cv;
        std::mutex mu;
        bool ready = false;
        std::atomic<bool> waiting{false};
        int64_t start = now_ns();
        std::thread waiter([&] {
            std::unique_lock<std::mutex> lk(mu);
            waiting = true;
            while (!ready) {
                cv.wait_for(lk, 20s);
            }
        });
        while (!waiting.load()) {
            std::this_thread::yield();
        }
        {
            std::unique_lock<std::mutex> lk(mu); // the waiter has let go of mu: it is in its wait
            ready = true;
            if (variant & 1) {
                lk.unlock();
            }
            if (variant & 2) {
                cv.notify_all();
            } else {
                cv.notify_one();
            }
        }
        waiter.join();
        CHECK(now_ns() - start < 10'000'000'000LL); // not the 20 s of the wait
    }
}

void test_precise_cv_no_lost_wakeup() {
    // The waiter and the notifier hand a turn to each other 20000 times. A wake-up lost between
    // the waiter's check and its wait would cost the whole 5 s wait and show as a time over 3 s.
    fake_backend::on = true;
    fake_cv cv;
    std::mutex mu;
    int turn = 0; // even: the notifier's, odd: the waiter's
    constexpr int rounds = 20000;
    int64_t start = now_ns();
    std::thread waiter([&] {
        std::unique_lock<std::mutex> lk(mu);
        for (int i = 0; i < rounds; i++) {
            while (turn % 2 == 0) {
                cv.wait_for(lk, 5s);
            }
            turn++;
            cv.notify_one();
        }
    });
    {
        std::unique_lock<std::mutex> lk(mu);
        for (int i = 0; i < rounds; i++) {
            turn++;
            cv.notify_one();
            while (turn % 2 == 1) {
                cv.wait_for(lk, 5s);
            }
        }
    }
    waiter.join();
    CHECK_EQ(turn, 2 * rounds);
    CHECK(now_ns() - start < 3'000'000'000LL);
}

void test_precise_cv_notify_right_after_unlock() {
    // The notifier takes the mutex the moment the waiter lets go of it inside its wait, and
    // notifies at once: the slot must be there by then. A slot published after the unlock misses
    // that notify, and the waiter sleeps its whole 2 s.
    fake_backend::on = true;
    fake_cv cv;
    std::mutex mu;
    int phase = 0; // 0: the waiter is not waiting, 1: it is about to wait, 2: it has been notified
    constexpr int rounds = 1000;
    int64_t start = now_ns();
    std::thread waiter([&] {
        for (int i = 0; i < rounds; i++) {
            std::unique_lock<std::mutex> lk(mu);
            phase = 1;
            while (phase != 2) {
                cv.wait_for(lk, 2s);
            }
            phase = 0;
        }
    });
    for (int i = 0; i < rounds; i++) {
        for (;;) {
            if (mu.try_lock()) {
                if (phase == 1) {
                    phase = 2;
                    cv.notify_one();
                    mu.unlock();
                    break;
                }
                mu.unlock();
            }
        }
    }
    waiter.join();
    CHECK(now_ns() - start < 1'500'000'000LL);
}

void test_precise_cv_one_slot() {
    // Two threads wait with a timeout on one cv (the runtime never does): the second waits the old
    // way, and a notify_all ends both.
    fake_backend::on = true;
    fake_cv cv;
    std::mutex mu;
    bool ready = false;
    std::atomic<int> waiting{0};
    std::vector<std::thread> threads;
    for (int i = 0; i < 2; i++) {
        threads.emplace_back([&] {
            std::unique_lock<std::mutex> lk(mu);
            waiting++;
            while (!ready) {
                cv.wait_for(lk, 60s);
            }
        });
    }
    while (waiting.load() < 2) {
        std::this_thread::yield();
    }
    {
        std::lock_guard<std::mutex> lk(mu); // both have let go of mu: both are waiting
        ready = true;
    }
    cv.notify_all();
    for (auto& t : threads) {
        t.join();
    }
}

void test_precise_cv_untimed() {
    // wait(lk) is the condition variable's own.
    fake_backend::on = true;
    fake_cv cv;
    std::mutex mu;
    bool ready = false;
    std::thread waiter([&] {
        std::unique_lock<std::mutex> lk(mu);
        while (!ready) {
            cv.wait(lk);
        }
    });
    for (int i = 0; i < 100; i++) {
        std::this_thread::yield();
    }
    {
        std::lock_guard<std::mutex> lk(mu);
        ready = true;
    }
    cv.notify_one();
    waiter.join();
}

void test_precise_sleep() {
    // The real backend: on Linux the plain sleep, never short; on Windows a precise one.
    for (int64_t ns : {int64_t(0), int64_t(-1), int64_t(200'000), int64_t(3'000'000)}) {
        int64_t start = now_ns();
        gp::detail::precise_sleep(ns);
        CHECK(ns <= 0 || now_ns() - start >= ns - 100'000);
    }
}

// The runtime's own waits go through the real backend: a Sleep and a timer wake on time and
// leave no slot in use.
void test_precise_waits_are_released() {
    gp::time::sleep(1'000'000);
    auto ch = gp::make_chan<int64_t>(1);
    gp::time::start_chan<int64_t>(
        ch, now_ns() + 2'000'000, 0,
        gp::func<void(gp::chan<int64_t>, int64_t)>(
            [](gp::chan<int64_t> out, int64_t due) { gp::time::try_send(out, due); }));
    gp::recv(ch);
    // The slots made are those of the waits that were at one time: Linux makes none.
    if (!gp::os::precise_timers()) {
        CHECK_EQ(gp::detail::precise_slots_made(), std::size_t(0));
    } else {
        CHECK(gp::detail::precise_slots_made() >= 1 && gp::detail::precise_slots_made() <= 8);
    }
}

// --- channel timers ---

void test_timer_fires() {
    ticking t;
    int64_t when = now_ns() + 5'000'000;
    t.start(when);
    CHECK_EQ(gp::len(t.c), int64_t(0));
    CHECK_EQ(gp::cap(t.c), int64_t(0)); // looks unbuffered, holds one value
    int64_t delta = gp::recv(t.c);
    CHECK(delta >= 0);
    CHECK(now_ns() >= when);
    // used up: Stop and Reset report false, the channel is empty
    CHECK(!gp::time::stop(t.c));
    CHECK(!recv_ready(t.c));
    CHECK(armed_is(0));
    CHECK(!gp::time::reset(t.c, now_ns() + 1'000'000, 0));
    gp::recv(t.c);
    CHECK_EQ(gp::len(t.c), int64_t(0));
    // an ordinary channel keeps its length and capacity
    auto plain = gp::make_chan<int64_t>(3);
    gp::send(plain, int64_t(1));
    CHECK_EQ(gp::len(plain), int64_t(1));
    CHECK_EQ(gp::cap(plain), int64_t(3));
}

void test_stop_reset_results() {
    ticking t;
    t.start(now_ns() + 3'600'000'000'000LL);
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(1));
    CHECK(gp::time::stop(t.c));  // it was pending
    CHECK(!gp::time::stop(t.c)); // twice
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0));
    CHECK(!gp::time::reset(t.c, now_ns() + 3'600'000'000'000LL, 0)); // it was stopped
    CHECK(gp::time::reset(t.c, now_ns() + 1'000'000, 0));            // it was pending
    gp::recv(t.c);

    // Go 1.23: a timer that fired and whose value nobody received is still active for Stop and
    // Reset, and the value is dropped
    ticking u;
    u.start(now_ns() + 1'000'000);
    CHECK(armed_is(0));
    // (the callback may still be running: wait for the value)
    CHECK(wait_for([&] { return u.c.gp_core()->count.load() == 1; }));
    CHECK_EQ(gp::len(u.c), int64_t(0));
    CHECK(gp::time::stop(u.c));
    CHECK(!recv_ready(u.c));
    CHECK(!gp::time::stop(u.c));

    ticking w;
    w.start(now_ns() + 1'000'000);
    CHECK(wait_for([&] { return w.c.gp_core()->count.load() == 1; }));
    CHECK(gp::time::reset(w.c, now_ns() + 3'600'000'000'000LL, 0)); // true: the value was dropped
    CHECK(!recv_ready(w.c));
    CHECK(gp::time::stop(w.c));
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0));

    // Stop of a channel that has no timer, and of the nil channel
    auto plain = gp::make_chan<int64_t>(1);
    CHECK(!gp::time::stop(plain));
    CHECK(!gp::time::stop(gp::chan<int64_t>()));
}

// After Stop (or Reset) returned, no send of the old setting may happen: race the thread against
// stop for deadlines that are due at once.
void test_no_stale_value() {
    int stale = 0;
    for (int i = 0; i < 3000; i++) {
        ticking t;
        t.start(now_ns() + (i % 3) * 20'000);
        if (i % 2 == 0) {
            gp::time::stop(t.c);
        } else {
            gp::time::reset(t.c, now_ns() + 3'600'000'000'000LL, 0);
        }
        // From here on nothing may arrive. Let the thread run, and look.
        std::this_thread::yield();
        if (recv_ready(t.c)) {
            stale++;
        }
        gp::time::stop(t.c);
        if (recv_ready(t.c)) {
            stale++;
        }
    }
    CHECK_EQ(stale, 0);
    CHECK(armed_is(0));
}

void test_order() {
    // 60 timers armed in the opposite order of their deadlines fire in the order of deadlines
    constexpr int n = 60;
    std::mutex mu;
    std::vector<int> order;
    auto done = gp::make_chan<int64_t>(n);
    std::vector<gp::chan<int64_t>> keep;
    int64_t base = now_ns() + 10'000'000;
    for (int i = n - 1; i >= 0; i--) {
        auto c = gp::make_chan<int64_t>(1);
        gp::time::start_func<int64_t>(c, base + int64_t(i) * 300'000,
                                      gp::func<void()>([&, i, done] {
                                          {
                                              std::lock_guard<std::mutex> lk(mu);
                                              order.push_back(i);
                                          }
                                          gp::send(done, int64_t(i));
                                      }));
    }
    for (int i = 0; i < n; i++) {
        gp::recv(done);
    }
    // each callback runs in a goroutine of its own, so two close deadlines may swap (and a loaded
    // machine swaps many): the order is not checked here (the run tests check it with margins),
    // only that every function ran exactly once
    std::vector<int> seen(order);
    std::sort(seen.begin(), seen.end());
    bool each_once = seen.size() == std::size_t(n);
    for (std::size_t i = 0; each_once && i < seen.size(); i++) {
        each_once = seen[i] == int(i);
    }
    CHECK(each_once);
    CHECK_EQ(order.size(), std::size_t(n));
    // channel timers: received in the order of the deadlines whatever the order of the receives;
    // the earliest is received while the others are still pending (a heap in the wrong order would
    // make the thread wait for the latest deadline first)
    ticking a, b, c;
    int64_t t0 = now_ns();
    a.start(t0 + 300'000'000);
    b.start(t0 + 100'000'000);
    c.start(t0 + 200'000'000);
    int64_t when_b = now_ns();
    gp::recv(b.c);
    int64_t after_b = now_ns();
    CHECK(after_b < t0 + 250'000'000);
    gp::recv(c.c);
    int64_t after_c = now_ns();
    gp::recv(a.c);
    int64_t after_a = now_ns();
    CHECK(after_b >= t0 + 100'000'000 && after_c >= t0 + 200'000'000 &&
          after_a >= t0 + 300'000'000);
    CHECK(when_b <= after_b && after_b <= after_c && after_c <= after_a);
    CHECK(armed_is(0));
}

void test_ticker() {
    // a period: ticks keep coming, one in the channel at a time, the others are dropped
    ticking t;
    int64_t t0 = now_ns();
    t.start(t0 + 2'000'000, 2'000'000);
    CHECK(gp::detail::timers_armed() == 1);
    for (int i = 0; i < 5; i++) {
        gp::recv(t.c);
    }
    CHECK(now_ns() - t0 >= 10'000'000);
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(1)); // still armed
    CHECK(gp::time::stop(t.c));
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0));
    CHECK(!gp::time::stop(t.c));

    // not received for many periods: one value waits, and Stop drops it
    ticking s;
    s.start(now_ns() + 1'000'000, 1'000'000);
    gp::time::sleep(30'000'000);
    CHECK(s.c.gp_core()->count.load() == 1);
    CHECK(gp::time::stop(s.c));
    CHECK(!recv_ready(s.c));

    // Reset changes the period and restarts a stopped ticker
    int64_t r0 = now_ns();
    CHECK(!gp::time::reset(s.c, r0 + 20'000'000, 20'000'000));
    gp::recv(s.c);
    CHECK(now_ns() - r0 >= 20'000'000);
    CHECK(gp::time::stop(s.c));

    // the next tick after a late wake-up is on the grid of the period (the delta says how late)
    ticking g;
    int64_t g0 = now_ns();
    g.start(g0 + 1'000'000, 1'000'000);
    gp::time::sleep(25'000'000);
    int64_t delta = gp::recv(g.c);
    CHECK(delta >= 0);
    CHECK(gp::time::stop(g.c));
}

// --- who holds a timer ---

// A timer that is due is seen by the receive that looks for it, whether the timer thread has run
// it or not (timer_observe, Go's maybeRunChan): the checks are never "not yet".
void test_due_is_seen() {
    auto spin_until = [](int64_t t) {
        while (now_ns() < t) {
        }
    };
    int misses = 0;
    for (int i = 0; i < 2000; i++) {
        ticking t;
        t.start(now_ns() - 1);
        if (!recv_ready(t.c)) {
            misses++;
        }
        CHECK(!recv_ready(t.c)); // the value is delivered once
    }
    for (int i = 0; i < 2000; i++) {
        ticking t;
        int64_t when = now_ns() + 100'000;
        t.start(when);
        spin_until(when + 50'000);
        if (!recv_ready(t.c)) {
            misses++;
        }
    }
    // the look comes at the very moment of the deadline, when the timer thread wakes and sends: the
    // receive waits for a send that has started
    for (int i = 0; i < 4000; i++) {
        ticking t;
        int64_t when = now_ns() + 30'000;
        t.start(when);
        spin_until(when);
        if (!recv_ready(t.c)) {
            misses++;
        }
    }
    // the send is slow and has started when the look comes: the look waits for it and finds the
    // value (the timer is not "idle" until its send is over)
    for (int i = 0; i < 20; i++) {
        auto c = gp::make_chan<int64_t>(1);
        std::atomic<bool> in_send{false};
        gp::time::start_chan<int64_t>(
            c, now_ns() + 50'000, 0,
            gp::func<void(gp::chan<int64_t>, int64_t)>([&](gp::chan<int64_t> ch, int64_t due) {
                in_send = true;
                std::this_thread::sleep_for(3ms);
                gp::time::try_send(ch, now_ns() - due);
            }));
        // (a look that finds the timer due runs the callback itself and gets the value: fine too)
        bool got = false;
        while (!in_send.load()) {
            if (recv_ready(c)) {
                got = true;
                break;
            }
        }
        if (!got) {
            got = recv_ready(c); // the callback runs on the timer thread: the look waits for it
        }
        if (!got) {
            misses++;
        }
    }
    // a ticker: the tick that is due is there
    for (int i = 0; i < 500; i++) {
        ticking t;
        int64_t when = now_ns() + 50'000;
        t.start(when, 50'000);
        spin_until(when + 130'000);
        if (!recv_ready(t.c)) {
            misses++;
        }
        gp::time::stop(t.c);
        CHECK(!recv_ready(t.c));
    }
    CHECK_EQ(misses, 0);
    // Stop and Reset right after the deadline: the timer was active (true) while the value is
    // unreceived
    for (int i = 0; i < 500; i++) {
        ticking t;
        t.start(now_ns() - 1);
        CHECK(gp::time::stop(t.c));
        CHECK(!gp::time::stop(t.c));
        CHECK(!gp::time::reset(t.c, now_ns() - 1, 0)); // was stopped, and is due again at once
        CHECK(gp::time::reset(t.c, now_ns() + 3'600'000'000'000LL, 0));
        CHECK(!recv_ready(t.c)); // the earlier value is gone
        CHECK(gp::time::stop(t.c));
    }
}

// A ticker whose thread was busy for many periods (its callback was slow) is back on the grid of
// its period afterwards: the late ticks are dropped, not made up for in a burst (Go's
// unlockAndRun). A burst would be calls that are each several periods late.
void test_ticker_grid() {
    constexpr int64_t period = 100'000'000;
    std::mutex mu;
    std::vector<int64_t> late;
    auto c = gp::make_chan<int64_t>(1);
    gp::time::start_chan<int64_t>(
        c, now_ns() + period, period,
        gp::func<void(gp::chan<int64_t>, int64_t)>([&](gp::chan<int64_t>, int64_t due) {
            bool first = false;
            {
                std::lock_guard<std::mutex> lk(mu);
                late.push_back(now_ns() - due);
                first = late.size() == 1;
            }
            if (first) {
                std::this_thread::sleep_for(5 * ns(period)); // the thread is busy
            }
        }));
    CHECK(wait_for([&] {
        std::lock_guard<std::mutex> lk(mu);
        return late.size() >= 6;
    }));
    CHECK(gp::time::stop(c));
    // (the callback may still be running: stop waits for a send, not for this)
    std::lock_guard<std::mutex> lk(mu);
    int far_late = 0;
    for (int64_t l : late) {
        if (l >= period) {
            far_late++;
        }
    }
    CHECK(far_late >= 1); // the call after the busy time knows how late it is
    CHECK(far_late <= 2); // and the ticks that it missed are not made up for
}

// A timer for a time that is never (the largest deadline) waits without using the processor.
void test_huge_deadline() {
    ticking t;
    t.start(INT64_MAX);
    // (not std::clock: on Windows it is the wall clock)
    int64_t c0 = gp::os::testing::cpu_time_ns();
    std::this_thread::sleep_for(300ms);
    double cpu_ms = double(gp::os::testing::cpu_time_ns() - c0) / 1e6;
    CHECK(cpu_ms < 150);
    CHECK(gp::time::stop(t.c));
    ticking u;
    u.start(INT64_MAX - 1, INT64_MAX);
    CHECK(gp::time::stop(u.c));
}

void test_weak() {
    CHECK(armed_is(0));
    {
        ticking t;
        t.start(now_ns() + 3'600'000'000'000LL);
        CHECK_EQ(gp::detail::timers_armed(), std::size_t(1));
    } // the channel is gone: so is the timer, though it was never stopped
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0));
    {
        ticking t;
        t.start(now_ns() + 3'600'000'000'000LL, 1'000'000'000);
        CHECK_EQ(gp::detail::timers_armed(), std::size_t(1));
    }
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0));
    // 10000 unreferenced timers
    for (int i = 0; i < 10000; i++) {
        ticking t;
        t.start(now_ns() + 3'600'000'000'000LL + i);
    }
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0));
    // a channel alone (no Timer object) keeps the timer: the program that holds only `C` receives
    gp::chan<int64_t> alone;
    {
        ticking t;
        t.start(now_ns() + 3'000'000);
        alone = t.c;
    }
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(1));
    gp::recv(alone);
    alone = gp::chan<int64_t>();
    CHECK(armed_is(0));
    // a timer that is due while its channel is dropped: no crash, nothing armed
    for (int i = 0; i < 2000; i++) {
        ticking t;
        t.start(now_ns() + (i % 5) * 10'000);
    }
    CHECK(armed_is(0));
}

void test_func_timers() {
    // the heap holds an AfterFunc timer's channel strongly: it fires with nobody holding it
    auto ran = gp::make_chan<int64_t>(1);
    {
        auto carrier = gp::make_chan<int64_t>();
        gp::time::start_func<int64_t>(carrier, now_ns() + 2'000'000,
                                      gp::func<void()>([ran] { gp::send(ran, int64_t(7)); }));
    }
    CHECK_EQ(gp::recv(ran), int64_t(7));
    CHECK(armed_is(0));
    // a closure that holds its channel is freed when the timer has fired (the heap's reference is
    // the last one, and the closure goes with the record)
    std::weak_ptr<int> watch;
    {
        auto token = std::make_shared<int>(1);
        watch = token;
        auto carrier = gp::make_chan<int64_t>();
        gp::time::start_func<int64_t>(carrier, now_ns() + 1'000'000, gp::func<void()>([token, ran] {
                                          gp::send(ran, int64_t(*token));
                                      }));
    }
    CHECK_EQ(gp::recv(ran), int64_t(1));
    CHECK(wait_for([&] { return watch.expired(); }));
    // Stop before it runs: it never runs
    auto carrier = gp::make_chan<int64_t>();
    std::atomic<int> runs{0};
    gp::time::start_func<int64_t>(carrier, now_ns() + 3'600'000'000'000LL,
                                  gp::func<void()>([&runs] { runs++; }));
    CHECK(gp::time::stop(carrier));
    CHECK(!gp::time::stop(carrier));
    CHECK_EQ(runs.load(), 0);
    // Reset of a stopped one runs it, and reports false; a second Reset of a pending one reports
    // true
    CHECK(!gp::time::reset(carrier, now_ns() + 3'600'000'000'000LL, 0));
    CHECK(gp::time::reset(carrier, now_ns() + 1'000'000, 0));
    CHECK(wait_for([&] { return runs.load() == 1; }));
    CHECK(armed_is(0));
    CHECK(!gp::time::stop(carrier));
    // the function blocks: other timers still fire
    auto gate = gp::make_chan<int64_t>();
    auto c2 = gp::make_chan<int64_t>();
    auto c3 = gp::make_chan<int64_t>();
    gp::time::start_func<int64_t>(c2, now_ns() + 1'000'000,
                                  gp::func<void()>([gate] { gp::recv(gate); }));
    gp::time::start_func<int64_t>(c3, now_ns() + 5'000'000,
                                  gp::func<void()>([ran] { gp::send(ran, int64_t(9)); }));
    CHECK_EQ(gp::recv(ran), int64_t(9));
    gp::send(gate, int64_t(1));
}

// cannot_wake waits until no timer can wake a goroutine: the timer thread of an earlier test may
// still be inside a firing (timers_can_wake counts that), which it leaves in a moment.
bool cannot_wake() {
    return wait_for([] { return !gp::detail::timers_can_wake(); });
}

void test_can_wake() {
    CHECK(cannot_wake());
    {
        ticking t; // armed, nobody waits: Go 1.23's deadlock rule: does not count
        t.start(now_ns() + 3'600'000'000'000LL);
        CHECK(cannot_wake());
        // a goroutine blocked on the channel: counts
        auto got = gp::make_chan<int64_t>();
        auto c = t.c;
        gp::go([c, got] { gp::send(got, gp::recv(c)); });
        CHECK(blocked_is(1));
        CHECK(gp::detail::timers_can_wake());
        gp::time::reset(t.c, now_ns() + 1'000'000, 0);
        gp::recv(got);
    }
    {
        // AfterFunc timers count
        auto carrier = gp::make_chan<int64_t>();
        gp::time::start_func<int64_t>(carrier, now_ns() + 3'600'000'000'000LL,
                                      gp::func<void()>([] {}));
        CHECK(gp::detail::timers_can_wake());
        gp::time::stop(carrier);
    }
    CHECK(cannot_wake());
}

void test_concurrent() {
    // goroutines arm, stop, reset and receive on their own timers at the same time
    constexpr int n = 64;
    group g;
    std::atomic<int> fired{0};
    for (int i = 0; i < n; i++) {
        g.go([&fired, i] {
            for (int k = 0; k < 40; k++) {
                ticking t;
                t.start(now_ns() + ((i + k) % 7) * 100'000);
                switch ((i + k) % 4) {
                case 0:
                    gp::recv(t.c);
                    fired++;
                    break;
                case 1:
                    gp::time::stop(t.c);
                    break;
                case 2:
                    gp::time::reset(t.c, now_ns() + 50'000, 0);
                    gp::recv(t.c);
                    fired++;
                    break;
                default:
                    break; // dropped
                }
            }
        });
    }
    g.wait();
    CHECK_EQ(fired.load(), n * 20);
    CHECK(armed_is(0));
    // many AfterFunc timers: all run exactly once
    constexpr int m = 3000;
    auto done = gp::make_chan<int64_t>(m);
    for (int i = 0; i < m; i++) {
        auto c = gp::make_chan<int64_t>();
        gp::time::start_func<int64_t>(c, now_ns() + (i % 20) * 200'000,
                                      gp::func<void()>([done, i] { gp::send(done, int64_t(i)); }));
    }
    int64_t sum = 0;
    for (int i = 0; i < m; i++) {
        sum += gp::recv(done);
    }
    CHECK_EQ(sum, int64_t(m) * (m - 1) / 2);
}

void test_shutdown_restart() {
    gp::detail::timers_shutdown();
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0));
    ticking t;
    t.start(now_ns() + 1'000'000); // the thread starts again
    gp::recv(t.c);
    gp::detail::timers_shutdown();
    gp::detail::timers_shutdown();
}

// --- the other natives ---

void test_natives() {
    gp::string v;
    bool found = false;
    std::tie(v, found) = gp::time::getenv("GOCPP_TIME_TEST_NOT_SET"_gs);
    CHECK(!found && v.gp_view().empty());
    std::tie(v, found) = gp::time::getenv("PATH"_gs);
    CHECK(found && !v.gp_view().empty());
    CHECK(gp::time::errno_text(2).gp_view() == "no such file or directory");
    // files
    auto [h, e] = gp::time::open_file("/nonexistent/gocpp/file"_gs);
    CHECK_EQ(h, int64_t(0));
    CHECK_EQ(e, int64_t(2));
    // (the file is made with the C++ library: no OS header outside src/os)
    std::string path = std::filesystem::temp_directory_path().string() + "/gocpp_time_test_" +
                       std::to_string(now_ns()) + ".bin";
    {
        std::FILE* f = std::fopen(path.c_str(), "wb");
        CHECK(f != nullptr);
        if (f != nullptr) {
            std::fputs("0123456789abcdef", f);
            std::fclose(f);
        }
    }
    auto [fd, oe] = gp::time::open_file(gp::string::gp_copy(path));
    CHECK(fd != 0 && oe == 0);
    auto buf = gp::make_slice<uint8_t>(8);
    uint8_t* raw = buf.gp_data();
    auto [n, re] = gp::time::read_file(fd, buf);
    CHECK_EQ(n, int64_t(8));
    CHECK_EQ(re, int64_t(0));
    CHECK(std::string(reinterpret_cast<char*>(raw), 8) == "01234567");
    CHECK_EQ(gp::time::seek_file(fd, 10, 0), int64_t(0));
    std::tie(n, re) = gp::time::read_file(fd, buf);
    CHECK_EQ(n, int64_t(6));
    CHECK(std::string(reinterpret_cast<char*>(raw), 6) == "abcdef");
    std::tie(n, re) = gp::time::read_file(fd, buf);
    CHECK_EQ(n, int64_t(0));
    CHECK_EQ(re, int64_t(0));
    CHECK_EQ(gp::time::seek_file(fd, -4, 2), int64_t(0));
    std::tie(n, re) = gp::time::read_file(fd, buf);
    CHECK_EQ(n, int64_t(4));
    CHECK(std::string(reinterpret_cast<char*>(raw), 4) == "cdef");
    gp::time::close_file(fd);
    std::remove(path.c_str());
    // call_once runs once, also from several goroutines, and again after a panic
    static std::atomic<int> once_runs{0};
    group g;
    for (int i = 0; i < 16; i++) {
        g.go([] { gp::time::call_once(1, gp::func<void()>([] { once_runs++; })); });
    }
    g.wait();
    CHECK_EQ(once_runs.load(), 1);
    // (A call that throws leaves the flag unset, so the next call tries again: std::call_once's
    // rule. It is not tested here: libstdc++'s call_once with an exception makes ThreadSanitizer
    // hang.)
    gp::time::call_once(2, gp::func<void()>([] { once_runs += 100; }));
    gp::time::call_once(2, gp::func<void()>([] { once_runs += 100; }));
    CHECK_EQ(once_runs.load(), 101);
}

void run_unit_tests() {
    // GOCPP_TIME_TEST_TRACE=1 names each test before it runs (to see where a hang is)
    const bool trace = std::getenv("GOCPP_TIME_TEST_TRACE") != nullptr;
#define RUN(f)                                                                                     \
    do {                                                                                           \
        if (trace) {                                                                               \
            say(#f "\n");                                                                          \
        }                                                                                          \
        f();                                                                                       \
    } while (0)
    RUN(test_clocks);
    RUN(test_sleep);
    RUN(test_plan_timed_wait);
    RUN(test_fine_timer_scope);
    RUN(test_split_precise_wait);
    RUN(test_slot_pool);
    RUN(test_slot_pool_gives_handles_back);
    RUN(test_precise_cv_waits_its_time);
    RUN(test_precise_cv_notify_wakes);
    RUN(test_precise_cv_no_lost_wakeup);
    RUN(test_precise_cv_notify_right_after_unlock);
    RUN(test_precise_cv_one_slot);
    RUN(test_precise_cv_untimed);
    RUN(test_precise_sleep);
    RUN(test_precise_waits_are_released);
    RUN(test_timer_fires);
    RUN(test_stop_reset_results);
    RUN(test_no_stale_value);
    RUN(test_order);
    RUN(test_ticker);
    RUN(test_due_is_seen);
    RUN(test_ticker_grid);
    RUN(test_huge_deadline);
    RUN(test_weak);
    RUN(test_func_timers);
    RUN(test_can_wake);
    RUN(test_concurrent);
    RUN(test_shutdown_restart);
    RUN(test_natives);
#undef RUN
    gp::detail::timers_shutdown();
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
    run_unit_tests();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
