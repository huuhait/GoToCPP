// The unit tests of the coroutines of package iter (gocpp/iter.hpp, src/iter.cpp, step 7.10):
// newcoro and coroswitch, with the goroutine count, the hand-off between threads, a coroutine that
// returns or calls runtime.Goexit, and a driver that changes between switches. runtime/cxx_test.go
// compiles this file with the runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with
// ASan + UBSan, and with ThreadSanitizer (iter_test.go), and runs it like test/unit.cpp: it prints
// "ok: N checks" and exits 0, or prints each failed check and exits 1.
#include <gocpp/iter.hpp>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include "threads_ended.hpp"

#include <atomic>
#include <cstdint>
#include <string>
#include <thread>
#include <vector>

namespace {

std::atomic<int> checks{0};
std::atomic<int> failures{0};

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

using fn_t = gp::func<void(std::uint64_t)>;

// A coroutine that yields 0, 1, ..., n-1 through `slot`, as iter.Pull's does: the driver sets
// `want` before each switch; the coroutine stores the value and switches back.
struct gen {
    std::int64_t slot = -1;
    bool done = false;
    std::int64_t blocked_inside = -1; // num_blocked() as the coroutine sees it: the driver waits
    std::uint64_t h = 0;
};

void test_basic() {
    std::int64_t base = gp::num_goroutine();
    gen g;
    g.h = gp::iter::newcoro(fn_t([&g]<int = 1>(std::uint64_t c) {
        for (std::int64_t i = 0; i < 5; i++) {
            g.slot = i;
            g.blocked_inside = gp::num_blocked();
            gp::iter::coroswitch(c);
        }
        g.done = true;
    }));
    CHECK(gp::num_goroutine() == base + 1); // counted from newcoro, before it ever ran
    CHECK(g.slot == -1);
    CHECK(gp::num_blocked() == 1); // born blocked, as Go's coroutine goroutine is
    std::vector<std::int64_t> got;
    for (int i = 0; i < 5; i++) {
        gp::iter::coroswitch(g.h);
        got.push_back(g.slot);
        CHECK(gp::num_goroutine() == base + 1);
        // exactly one side waits at any time: the coroutine now, the driver while the coroutine
        // runs
        CHECK(gp::num_blocked() == 1);
        CHECK(g.blocked_inside == 1);
    }
    CHECK((got == std::vector<std::int64_t>{0, 1, 2, 3, 4}));
    CHECK(!g.done);
    gp::iter::coroswitch(
        g.h); // the function returns: control comes back, and the goroutine is gone
    CHECK(g.done);
    CHECK(gp::num_goroutine() == base);
    CHECK(gp::num_blocked() == 0);
}

void test_goexit() {
    std::int64_t base = gp::num_goroutine();
    bool after = false;
    auto h = gp::iter::newcoro(fn_t([&after]<int = 2>(std::uint64_t) {
        gp::goexit();
        after = true;
    }));
    CHECK(gp::num_goroutine() == base + 1);
    gp::iter::coroswitch(h);
    CHECK(!after);
    CHECK(gp::num_goroutine() == base);
}

// The driver changes between switches: three threads take turns, one at a time.
void test_drivers() {
    std::int64_t base = gp::num_goroutine();
    gen g;
    g.h = gp::iter::newcoro(fn_t([&g]<int = 3>(std::uint64_t c) {
        for (std::int64_t i = 0; i < 30; i++) {
            g.slot = i;
            gp::iter::coroswitch(c);
        }
    }));
    std::vector<std::int64_t> got;
    for (int turn = 0; turn < 3; turn++) {
        std::atomic<bool> finished{false};
        gp::go([&] {
            for (int i = 0; i < 5; i++) {
                gp::iter::coroswitch(g.h);
                got.push_back(g.slot);
            }
            finished.store(true);
        });
        while (!finished.load()) {
            std::this_thread::yield();
        }
        gp::iter::coroswitch(g.h);
        got.push_back(-g.slot);
    }
    std::vector<std::int64_t> want;
    for (int t = 0; t < 3; t++) {
        for (int i = 0; i < 5; i++) {
            want.push_back(t * 6 + i);
        }
        want.push_back(-(t * 6 + 5));
    }
    CHECK(got == want);
    // 18 switches so far (slots 0 to 17): 12 more give the slots 18 to 29, and one more ends the
    // function
    for (int i = 0; i < 13; i++) {
        gp::iter::coroswitch(g.h);
    }
    CHECK(gp::num_goroutine() == base);
}

// Many short coroutines, one after the other and a few at a time.
void test_many() {
    std::int64_t base = gp::num_goroutine();
    std::int64_t total = 0;
    for (int i = 0; i < 500; i++) {
        std::int64_t v = 0;
        auto h = gp::iter::newcoro(fn_t([&v]<int = 4>(std::uint64_t c) {
            v = 1;
            gp::iter::coroswitch(c);
            v = 2;
        }));
        gp::iter::coroswitch(h);
        total += v;
        gp::iter::coroswitch(h);
        total += v;
    }
    CHECK(total == 500 * 3);
    CHECK(gp::num_goroutine() == base);
    std::atomic<std::int64_t> sum{0};
    std::atomic<int> ended{0};
    for (int k = 0; k < 6; k++) {
        gp::go([&] {
            for (int i = 0; i < 200; i++) {
                std::int64_t v = 0;
                auto h = gp::iter::newcoro(fn_t([&v]<int = 5>(std::uint64_t c) {
                    v = 7;
                    gp::iter::coroswitch(c);
                    v = 8;
                }));
                gp::iter::coroswitch(h);
                sum += v;
                gp::iter::coroswitch(h);
                sum += v;
            }
            ended++;
        });
    }
    while (ended.load() < 6) {
        std::this_thread::yield();
    }
    CHECK(sum.load() == 6 * 200 * 15);
}

} // namespace

int main() {
    gp::detail::this_goroutine(); // registers this thread as the main goroutine: NumGoroutine
                                  // counts it
    test_basic();
    test_goexit();
    test_drivers();
    test_many();
    rt_test::threads_ended();
    if (failures != 0) {
        return 1;
    }
    std::string ok = "ok: " + std::to_string(checks.load()) + " checks\n";
    gp::os::write(gp::os::stdout_fd, ok.data(), ok.size());
    return 0;
}
