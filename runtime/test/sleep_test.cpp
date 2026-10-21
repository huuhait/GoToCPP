// The accuracy of the timed waits of package time (gocpp/time.hpp): time.Sleep, a one-shot
// timer and a ticker, measured with the monotonic clock. The program prints one line for each
// case, "kind want_us runs median_us p99_us max_us", where the numbers are how much later than
// asked the wait ended (a wait never ends early: the program fails if one does). The bounds are
// the test's (windows_sleep_test.go runs it on a real Windows PC): the timers of Windows tick
// every 15.6 ms unless the runtime asks for finer ones (ADR-132), so a Sleep(5ms) that ends 10 ms
// late is the failure it looks for. Nothing here is a unit test; test/time_test.cpp has those.
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include "../src/timed_wait.hpp"
#include "threads_ended.hpp"

#include <algorithm>
#include <cstdint>
#include <cstdio>
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

void run_cases() {
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
    // Every wait has ended and every timer is stopped: no one asks for fine ticks any more. A hold
    // that is never given back would leave the system on 1 ms ticks until the process exits.
    gp::time::sleep(100 * ms);
    print("holders end " + std::to_string(gp::detail::fine_timer_holders()) + "\n");
    gp::detail::timers_shutdown();
    rt_test::threads_ended();
    print("done\n");
    gp::os::exit(0);
}

void tests_main() {
    run_cases();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
