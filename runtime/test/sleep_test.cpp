// The accuracy of the timed waits of package time (gocpp/time.hpp): time.Sleep, a one-shot
// timer and a ticker, measured with the monotonic clock. The program prints one line for each
// case, "kind want_us runs median_us p99_us max_us", where the numbers are how much later than
// asked the wait ended (a wait never ends early: the program fails if one does). The bounds are
// the test's (windows_sleep_test.go runs it on a real Windows PC): the timers of Windows tick
// every 15.6 ms unless the runtime asks for finer ones (ADR-132) or waits on a high-resolution
// timer (ADR-133), so a Sleep(5ms) that ends 10 ms late is the failure it looks for. The program
// also measures what a precise wait must not cost: the system's idle tick, CPU time and handles.
// Nothing here is a unit test; test/time_test.cpp has those.
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include "../src/timed_wait.hpp"
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <condition_variable>
#include <cstdint>
#include <cstdio>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace {

constexpr int runs = 200;

// A case: how many waits, and how long each one is.
struct wait_case {
    const char* kind;
    int64_t want_ns;
    int count;
};

int64_t now_ns() {
    return gp::time::nano();
}

void print(const std::string& s) {
    gp::os::write(gp::os::stdout_fd, s.data(), s.size());
}

// report prints the line of a case from the delays (ns later than asked), sorted here.
void report(const wait_case& c, std::vector<int64_t> late) {
    std::sort(late.begin(), late.end());
    size_t n = late.size();
    int64_t median = late[n / 2];
    int64_t p99 = late[(n * 99 + 99) / 100 - 1];
    int64_t max = late.back();
    print(std::string(c.kind) + " " + std::to_string(c.want_ns / 1000) + " " + std::to_string(n) +
          " " + std::to_string(median / 1000) + " " + std::to_string(p99 / 1000) + " " +
          std::to_string(max / 1000) + "\n");
    if (late.front() < 0) {
        print("FAIL: " + std::string(c.kind) + " " + std::to_string(c.want_ns / 1000) +
              " us ended early\n");
        gp::os::exit(1);
    }
}

// Sleep: the goroutine's own wait.
void measure_sleep(const wait_case& c) {
    std::vector<int64_t> late;
    for (int i = 0; i < c.count; i++) {
        int64_t start = now_ns();
        gp::time::sleep(c.want_ns);
        late.push_back(now_ns() - start - c.want_ns);
    }
    report(c, late);
}

// A one-shot timer on a channel (time.After): the timer thread fires it and the goroutine that
// receives wakes.
void measure_timer(const wait_case& c) {
    std::vector<int64_t> late;
    for (int i = 0; i < c.count; i++) {
        auto ch = gp::make_chan<int64_t>(1);
        int64_t start = now_ns();
        gp::time::start_chan<int64_t>(
            ch, start + c.want_ns, 0,
            gp::func<void(gp::chan<int64_t>, int64_t)>(
                [](gp::chan<int64_t> out, int64_t due) { gp::time::try_send(out, due); }));
        gp::recv(ch);
        late.push_back(now_ns() - start - c.want_ns);
    }
    report(c, late);
}

// A ticker: the delay of each tick after its place on the grid of the period.
void measure_ticker(const wait_case& c) {
    std::vector<int64_t> late;
    auto ch = gp::make_chan<int64_t>(1);
    int64_t start = now_ns();
    gp::time::start_chan<int64_t>(
        ch, start + c.want_ns, c.want_ns,
        gp::func<void(gp::chan<int64_t>, int64_t)>(
            [](gp::chan<int64_t> out, int64_t due) { gp::time::try_send(out, due); }));
    for (int i = 0; i < c.count; i++) {
        int64_t due = gp::recv(ch);
        late.push_back(now_ns() - due);
    }
    gp::time::stop(ch);
    report(c, late);
}

// The most waits that held fine timer ticks at one time while a goroutine slept for sleep_ns, seen
// by spinning on this thread for watch_ns: a short sleep is fine all the way, and the start of a
// long one is not (the system keeps its idle tick while a process waits for an hour).
size_t most_holders_while_sleeping(int64_t sleep_ns, int64_t watch_ns) {
    auto done = gp::make_chan<int64_t>(1);
    gp::go([done, sleep_ns] {
        gp::time::sleep(sleep_ns);
        gp::send(done, int64_t(1));
    });
    size_t most = 0;
    int64_t end = now_ns() + watch_ns;
    while (now_ns() < end) {
        most = std::max(most, gp::detail::fine_timer_holders());
        std::this_thread::yield();
    }
    gp::recv(done);
    return most;
}

void measure_holders() {
    const int64_t ms = 1'000'000;
    print("holders short " + std::to_string(most_holders_while_sleeping(30 * ms, 20 * ms)) + "\n");
    print("holders long " + std::to_string(most_holders_while_sleeping(400 * ms, 200 * ms)) + "\n");
}

// The median of n waits of ask_ns on the C++ library's own timed wait (a condition variable that
// nobody notifies), in microseconds. Windows rounds such a wait up to the system tick of the
// process: about 15 ms for a wait of 2 ms, and about 1 ms when the process holds fine ticks
// (os::fine_timer). So it shows whether the runtime's waits hold the tick: it needs a process that
// is idle, and the process-wide tick of Windows 10 version 2004 and later.
int64_t median_plain_wait_us(int n, int64_t ask_ns) {
    std::vector<int64_t> took;
    std::mutex mu;
    std::condition_variable cv;
    for (int i = 0; i < n; i++) {
        std::unique_lock<std::mutex> lk(mu);
        int64_t start = now_ns();
        cv.wait_for(lk, std::chrono::nanoseconds(ask_ns));
        took.push_back(now_ns() - start);
    }
    std::sort(took.begin(), took.end());
    return took[took.size() / 2] / 1000;
}

// A goroutine sleeps 1.5 s while this thread watches: the process must use no CPU for it, and
// must not hold the system's fine ticks (a plain timed wait of 2 ms still takes a full tick).
void measure_idle() {
    const int64_t ms = 1'000'000;
    int64_t before = median_plain_wait_us(10, 2 * ms);
    auto done = gp::make_chan<int64_t>(1);
    int64_t cpu0 = gp::os::testing::cpu_time_ns();
    gp::go([done] {
        gp::time::sleep(1500 * 1'000'000);
        gp::send(done, int64_t(1));
    });
    int64_t during = median_plain_wait_us(20, 2 * ms);
    gp::recv(done);
    int64_t cpu_us = (gp::os::testing::cpu_time_ns() - cpu0) / 1000;
    print("idle probe_before_us " + std::to_string(before) + " probe_during_us " +
          std::to_string(during) + " cpu_us " + std::to_string(cpu_us) + "\n");
}

// The CPU time of timed waits that end with a spin (ADR-133): n sleeps of sleep_ns, in
// nanoseconds of CPU for each sleep (the program's whole process, so the timer thread and the
// start of the wait count too).
void measure_cpu(const char* kind, int64_t sleep_ns, int n) {
    int64_t cpu0 = gp::os::testing::cpu_time_ns();
    for (int i = 0; i < n; i++) {
        gp::time::sleep(sleep_ns);
    }
    int64_t cpu_ns = gp::os::testing::cpu_time_ns() - cpu0;
    print("cpu " + std::string(kind) + " " + std::to_string(sleep_ns / 1000) + " " +
          std::to_string(n) + " " + std::to_string(cpu_ns / n) + "\n");
}

// Many sleeps and timers from several goroutines: the handles of the process must not grow (the
// runtime keeps a slot for each wait that was going on at one time, and reuses it).
void sleeps_and_timers(int goroutines, int sleeps_each, int timers) {
    auto done = gp::make_chan<int64_t>(goroutines);
    for (int g = 0; g < goroutines; g++) {
        gp::go([done, sleeps_each] {
            for (int i = 0; i < sleeps_each; i++) {
                gp::time::sleep(50'000);
            }
            gp::send(done, int64_t(1));
        });
    }
    for (int i = 0; i < timers; i++) {
        auto ch = gp::make_chan<int64_t>(1);
        gp::time::start_chan<int64_t>(
            ch, now_ns() + 20'000, 0,
            gp::func<void(gp::chan<int64_t>, int64_t)>(
                [](gp::chan<int64_t> out, int64_t due) { gp::time::try_send(out, due); }));
        gp::recv(ch);
    }
    for (int g = 0; g < goroutines; g++) {
        gp::recv(done);
    }
}

void measure_handles() {
    sleeps_and_timers(8, 100, 50); // warm up: threads, slots
    gp::time::sleep(200'000'000);
    int64_t before = gp::os::testing::open_handles();
    sleeps_and_timers(8, 1250, 1000); // 10000 sleeps and 1000 timers
    gp::time::sleep(200'000'000);
    int64_t after = gp::os::testing::open_handles();
    print("handles before " + std::to_string(before) + " after " + std::to_string(after) +
          " slots " + std::to_string(gp::detail::precise_slots_made()) + "\n");
}

// A ticker of 1 ms for one second: how many of its 1000 ticks the receiver got. A tick is dropped
// when the receiver is a period late (the channel holds one).
void measure_ticker_rate() {
    auto ch = gp::make_chan<int64_t>(1);
    int64_t start = now_ns();
    gp::time::start_chan<int64_t>(
        ch, start + 1'000'000, 1'000'000,
        gp::func<void(gp::chan<int64_t>, int64_t)>(
            [](gp::chan<int64_t> out, int64_t due) { gp::time::try_send(out, due); }));
    int got = 0;
    int64_t cpu0 = gp::os::testing::cpu_time_ns();
    while (now_ns() - start < 1'000'000'000LL) {
        gp::recv(ch);
        got++;
    }
    int64_t cpu_ms = (gp::os::testing::cpu_time_ns() - cpu0) / 1'000'000;
    gp::time::stop(ch);
    print("ticks 1000 " + std::to_string(got) + "\n");
    print("ticker_cpu_ms " + std::to_string(cpu_ms) + "\n");
}

// Loops of short sleeps, the way a program polls: the process's CPU time for each loop (ADR-133).
// A wait that is only a spin would use a whole core for them. "loop name iterations cpu_ms": the
// iterations of the sleeps in 2 s of the wall clock, and the CPU of the whole process in that time.
void measure_loop(const char* name, int64_t sleep_ns) {
    int64_t cpu0 = gp::os::testing::cpu_time_ns();
    int64_t start = now_ns();
    int64_t iterations = 0;
    while (now_ns() - start < 2'000'000'000LL) {
        gp::time::sleep(sleep_ns);
        iterations++;
    }
    int64_t cpu_ms = (gp::os::testing::cpu_time_ns() - cpu0) / 1'000'000;
    print("loop " + std::string(name) + " " + std::to_string(iterations) + " " +
          std::to_string(cpu_ms) + "\n");
}

// 24 goroutines that sleep 50 to 400 us in turn, for 2 s: "loop sleeps24 iterations cpu_ms".
void measure_loop_goroutines() {
    const int goroutines = 24;
    auto done = gp::make_chan<int64_t>(goroutines);
    std::atomic<int64_t> iterations{0};
    int64_t cpu0 = gp::os::testing::cpu_time_ns();
    int64_t start = now_ns();
    for (int g = 0; g < goroutines; g++) {
        gp::go([done, g, start, &iterations] {
            int64_t sleep_ns = 50'000 + 50'000 * (g % 8); // 50, 100, ..., 400 us
            while (now_ns() - start < 2'000'000'000LL) {
                gp::time::sleep(sleep_ns);
                iterations++;
            }
            gp::send(done, int64_t(1));
        });
    }
    for (int g = 0; g < goroutines; g++) {
        gp::recv(done);
    }
    int64_t cpu_ms = (gp::os::testing::cpu_time_ns() - cpu0) / 1'000'000;
    print("loop sleeps24 " + std::to_string(iterations.load()) + " " + std::to_string(cpu_ms) +
          "\n");
}

void measure_loops() {
    measure_loop("sleep100us", 100'000);
    measure_loop("sleep1ms", 1'000'000);
    measure_loop_goroutines();
}

// The fallback of ADR-132 (a system without high-resolution timers), run on this one by switching
// the timers off: the fine ticks are held for the last part of a wait only and given back, a wait
// is late by about a tick of 1 ms and never early. Started with the argument "fallback".
void run_fallback() {
    const int64_t ms = 1'000'000;
    gp::os::testing::force_fallback(true);
    print("precise " + std::string(gp::os::precise_timers() ? "1" : "0") + "\n");
    measure_holders();
    for (int64_t d : {1 * ms, 5 * ms, 15 * ms}) {
        measure_sleep({"sleep", d, 100});
    }
    measure_loops();
    gp::time::sleep(100 * ms);
    print("holders end " + std::to_string(gp::detail::fine_timer_holders()) + "\n");
    gp::detail::timers_shutdown();
    rt_test::threads_ended();
    print("done\n");
    gp::os::exit(0);
}

// 1000 goroutines sleep at the same time: a slot each. When they are over, the pool keeps 64 idle
// slots with their handles, and the others have given theirs back. "burst slots_made N
// slots_open_during N slots_open_after N slots_open_late N handles_...".
void measure_burst() {
    const int sleepers = 1000;
    auto done = gp::make_chan<int64_t>(sleepers);
    int64_t before = gp::os::testing::open_handles();
    for (int i = 0; i < sleepers; i++) {
        gp::go([done] {
            gp::time::sleep(600'000'000);
            gp::send(done, int64_t(1));
        });
    }
    gp::time::sleep(300'000'000);
    size_t made = gp::detail::precise_slots_made();
    size_t open_during = gp::detail::precise_slots_open();
    int64_t during = gp::os::testing::open_handles();
    for (int i = 0; i < sleepers; i++) {
        gp::recv(done);
    }
    gp::time::sleep(100'000'000);
    size_t open_after = gp::detail::precise_slots_open();
    int64_t after = gp::os::testing::open_handles();
    // The idle threads of the goroutine pool wait for the next go statement with a timeout of 5 s,
    // a slot each; when they have left, only the idle slots of the pool keep their handles.
    gp::time::sleep(6'500'000'000LL);
    size_t open_late = gp::detail::precise_slots_open();
    int64_t late = gp::os::testing::open_handles();
    print("burst slots_made " + std::to_string(made) + " slots_open_during " +
          std::to_string(open_during) + " slots_open_after " + std::to_string(open_after) +
          " slots_open_late " + std::to_string(open_late) + " handles_before " +
          std::to_string(before) + " handles_during " + std::to_string(during) + " handles_after " +
          std::to_string(after) + " handles_late " + std::to_string(late) + "\n");
}

void run_cases() {
    print("precise " + std::string(gp::os::precise_timers() ? "1" : "0") + "\n");
    measure_holders();
    const int64_t ms = 1'000'000;
    for (int64_t d : {1 * ms, 5 * ms, 15 * ms}) {
        measure_sleep({"sleep", d, runs});
        measure_timer({"timer", d, runs});
        measure_ticker({"ticker", d, runs});
    }
    // Longer waits: a wait of 100 ms and one of a second must not be late by a tick either.
    measure_sleep({"sleep", 100 * ms, 20});
    measure_sleep({"sleep", 1000 * ms, 3});
    measure_ticker_rate();
    measure_cpu("sleep", 16 * ms, 125);
    measure_cpu("sleep", 1 * ms, 1000);
    measure_loops();
    measure_idle();
    measure_handles();
    measure_burst();
    // Every wait has ended and every timer is stopped: no one asks for fine ticks any more. A hold
    // that is never given back would leave the system on 1 ms ticks until the process exits.
    gp::time::sleep(100 * ms);
    print("holders end " + std::to_string(gp::detail::fine_timer_holders()) + "\n");
    // A timer an hour away is pending when the timer thread is told to end: its precise wait is
    // woken, and the thread ends.
    auto never = gp::make_chan<int64_t>(1);
    gp::time::start_chan<int64_t>(
        never, now_ns() + 3600 * 1000 * ms, 0,
        gp::func<void(gp::chan<int64_t>, int64_t)>(
            [](gp::chan<int64_t> out, int64_t due) { gp::time::try_send(out, due); }));
    gp::time::sleep(10 * ms);
    gp::detail::timers_shutdown();
    rt_test::threads_ended();
    // The process ends while a goroutine is in a precise wait of an hour (its timer is pending).
    gp::go([] { gp::time::sleep(3600LL * 1000 * 1'000'000); });
    gp::time::sleep(10 * ms);
    print("done\n");
    gp::os::exit(0);
}

void tests_main() {
    const std::vector<gp::string>& args = gp::args();
    if (args.size() > 1 && args[1] == "fallback") {
        run_fallback();
    }
    run_cases();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
