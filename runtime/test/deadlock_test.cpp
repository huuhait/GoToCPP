// The unit tests of the deadlock detector (gocpp/goroutine.hpp, "Deadlock detection";
// src/deadlock.cpp, src/goroutine.cpp), Phase 08 step 8.11.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and with ThreadSanitizer
// (deadlock_test.go), and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or
// prints each failed check and exits 1.
//
// Run with a case name as the one argument, the program runs a case that ends the process with
// the real report (deadlock_test.go compares what it prints with Go's).
//
// The unit tests set a hook that gets the report instead of the end of the process: the hook
// closes the channels that the goroutines wait on, so they go on, and the test goes on after the
// "deadlock". Nothing depends on timing: a goroutine that is blocked is waited for through
// gp::num_blocked(), a wait has a limit of minutes (a hang is a failure, a slow machine is not).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include "../src/chan_core.hpp"
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
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

bool contains(const std::string& s, const std::string& part) {
    return s.find(part) != std::string::npos;
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

using chan = gp::chan<int64_t>;

// --- the hook ---

std::atomic<int> reports{0};
std::mutex mu;
std::string last;                   // the last report
std::vector<chan> release;          // the channels the hook closes, so the goroutines go on
std::atomic<int> flag_at_report{0}; // the value of `flag` when the last report came
std::atomic<int> flag{0};

void on_report(const char* text, std::size_t n) noexcept {
    std::vector<chan> to_close;
    {
        std::lock_guard<std::mutex> lk(mu);
        last.assign(text, n);
        to_close.swap(release);
    }
    flag_at_report = flag.load();
    reports++;
    for (chan& c : to_close) {
        gp::close(c);
    }
}

void arm(std::vector<chan> chans) {
    reports = 0;
    flag = 0;
    flag_at_report = 0;
    {
        std::lock_guard<std::mutex> lk(mu);
        last.clear();
        release = std::move(chans);
    }
}

std::string report_text() {
    std::lock_guard<std::mutex> lk(mu);
    return last;
}

// send_ignoring_close sends v on c and goes on when the channel is closed under it (the panic of a
// send on a closed channel): a goroutine that the hook released.
void send_ignoring_close(const chan& c, int64_t v) {
    try {
        gp::send(c, v);
    } catch (const gp::panic_error&) {
    }
}

// The goroutine headers of a report, in order: "goroutine N [reason]:" -> "N reason".
std::vector<std::string> headers(const std::string& rep) {
    std::vector<std::string> out;
    std::size_t pos = 0;
    while ((pos = rep.find("\ngoroutine ", pos)) != std::string::npos) {
        std::size_t end = rep.find("]:\n", pos);
        std::string h = rep.substr(pos + 11, end - pos - 11);
        std::size_t sp = h.find(" [");
        out.push_back(h.substr(0, sp) + " " + h.substr(sp + 2));
        pos = end;
    }
    return out;
}

int64_t id_of(const std::string& header) {
    return std::stoll(header.substr(0, header.find(' ')));
}

// --- tests ---

// Everything is asleep: main, a sender, a select. The report lists the three, by id, with Go's
// reasons, their stacks and, for the goroutines, where they were created.
void test_report() {
    chan never = gp::make_chan<int64_t>();
    chan gate = gp::make_chan<int64_t>();
    chan other = gp::make_chan<int64_t>();
    arm({never, gate, other});
    group g;
    g.go([gate] { send_ignoring_close(gate, 1); });
    g.go([never, other] {
        gp::sel_recv gp_s0{never, gp::sel_drop};
        gp::sel_recv gp_s1{other, gp::sel_drop};
        gp::select(gp_s0, gp_s1);
    });
    gp::recv(never); // the last to sleep reports; the hook closes `never` and this goes on
    CHECK(g.wait());
    CHECK_EQ(reports.load(), 1);
    std::string rep = report_text();
    CHECK(rep.rfind("fatal error: all goroutines are asleep - deadlock!\n\ngoroutine ", 0) == 0);
    auto hs = headers(rep);
    CHECK_EQ(hs.size(), std::size_t(3));
    if (hs.size() == 3) {
        CHECK(id_of(hs[0]) < id_of(hs[1]) && id_of(hs[1]) < id_of(hs[2]));
        CHECK_EQ(id_of(hs[0]), int64_t(1)); // the main goroutine is first
        CHECK(hs[0].find("chan receive") != std::string::npos);
        CHECK(hs[1].find("chan send") != std::string::npos);
        CHECK(hs[2].find("select") != std::string::npos);
    }
    // the goroutines (not main) say who started them
    std::size_t created = 0;
    for (std::size_t p = rep.find("created by "); p != std::string::npos;
         p = rep.find("created by ", p + 1)) {
        created++;
    }
    CHECK_EQ(created, std::size_t(2));
    CHECK(contains(rep, " in goroutine " + std::to_string(id_of(hs[0])) + "\n"));
    CHECK(rep.back() == '\n' && !contains(rep, "\n\n\n"));
    // every goroutine has a stack: its frames are in this file (each parked thread captured its
    // own)
    std::size_t frames = 0;
    for (std::size_t p = rep.find("deadlock_test.cpp:"); p != std::string::npos;
         p = rep.find("deadlock_test.cpp:", p + 1)) {
        frames++;
    }
    CHECK(frames >= 3); // at least one for each goroutine (inlining may leave one)
    for (std::size_t i = 0; i < hs.size(); i++) {
        std::size_t from = rep.find("\ngoroutine " + hs[i].substr(0, hs[i].find(' ')) + " [");
        std::size_t to = i + 1 < hs.size() ? rep.find("\ngoroutine ", from + 1) : rep.size();
        CHECK(rep.find("deadlock_test.cpp:", from) < to);
    }
}

// 2000 goroutines asleep: the report lists them all (each parked thread captures its own stack, and
// the names of the functions are resolved once, not once per goroutine).
void test_many_goroutines() {
    constexpr int n = 2000;
    chan gate = gp::make_chan<int64_t>();
    chan never = gp::make_chan<int64_t>();
    arm({gate, never});
    group g;
    for (int i = 0; i < n; i++) {
        g.go([gate] { gp::recv(gate); });
    }
    gp::recv(never);
    CHECK(g.wait());
    CHECK_EQ(reports.load(), 1);
    std::string rep = report_text();
    auto hs = headers(rep);
    CHECK_EQ(hs.size(), std::size_t(n + 1));
    std::size_t created = 0;
    for (std::size_t p = rep.find("created by "); p != std::string::npos;
         p = rep.find("created by ", p + 1)) {
        created++;
    }
    CHECK_EQ(created, std::size_t(n));
}

// A nil channel and a select with no case are asleep for ever, and are listed with Go's reasons.
void test_nil_reasons() {
    chan never = gp::make_chan<int64_t>();
    arm({never});
    gp::go([] { gp::recv(chan()); });
    gp::go([] { gp::send(chan(), int64_t(1)); });
    gp::recv(never);
    // (the two goroutines sleep for ever: the test has no way to end them; they are the cost of
    // this test and end with the process)
    std::string rep = report_text();
    CHECK(contains(rep, " [chan receive (nil chan)]:\n"));
    CHECK(contains(rep, " [chan send (nil chan)]:\n"));
    CHECK_EQ(reports.load(), 1);
}

// Not a deadlock while a goroutine runs; one report when it falls asleep; and when the goroutine
// that runs ends instead of blocking.
void test_running_goroutine() {
    for (int mode = 0; mode < 2; mode++) {
        chan never = gp::make_chan<int64_t>();
        chan sink = gp::make_chan<int64_t>(); // nobody receives from it
        arm({never, sink});
        std::atomic<bool> stop{false};
        group g;
        g.go([&, sink] {
            while (!stop.load()) {
                std::this_thread::yield();
            }
            if (mode == 0) {
                send_ignoring_close(sink, 1); // falls asleep
            }
            // mode 1: ends, with main asleep
        });
        group h;
        h.go([&] {
            // main is asleep once it is counted as blocked
            wait_for([] { return gp::num_blocked() == 1; });
            std::this_thread::sleep_for(
                20ms); // the detector had its chance, with a goroutine running
            CHECK_EQ(reports.load(), 0);
            stop = true;
        });
        gp::recv(never);
        CHECK(g.wait());
        CHECK(h.wait());
        CHECK_EQ(reports.load(), 1);
    }
}

// Go 1.23's timer rule.
void test_timers() {
    auto now_ns = [] { return gp::time::nano(); };
    auto start = [&](const chan& c, int64_t when, int64_t period) {
        gp::time::start_chan<int64_t>(c, when, period,
                                      gp::func<void(chan, int64_t)>([](chan ch, int64_t) {
                                          gp::time::try_send(ch, int64_t(1));
                                      }));
    };
    // a goroutine asleep on a timer channel is not asleep for ever
    {
        chan t = gp::make_chan<int64_t>(1);
        arm({});
        start(t, now_ns() + 30'000'000, 0);
        CHECK_EQ(gp::recv(t), int64_t(1));
        CHECK_EQ(reports.load(), 0);
    }
    // a select with a timer case
    {
        chan never = gp::make_chan<int64_t>();
        chan t = gp::make_chan<int64_t>(1);
        arm({never});
        start(t, now_ns() + 30'000'000, 0);
        gp::sel_recv gp_s0{never, gp::sel_drop};
        gp::sel_recv gp_s1{t, gp::sel_drop};
        CHECK_EQ(gp::select(gp_s0, gp_s1), 1);
        CHECK_EQ(reports.load(), 0);
    }
    // a timer that nobody waits on does not keep the program alive
    {
        chan never = gp::make_chan<int64_t>();
        chan t = gp::make_chan<int64_t>(1);
        arm({never});
        start(t, now_ns() + 3'600'000'000'000LL, 0);
        gp::recv(never);
        CHECK_EQ(reports.load(), 1);
        CHECK(gp::time::stop(t));
    }
    // a ticker that nobody waits on, too
    {
        chan never = gp::make_chan<int64_t>();
        chan t = gp::make_chan<int64_t>(1);
        arm({never});
        start(t, now_ns() + 1'000'000, 1'000'000);
        gp::recv(never);
        CHECK_EQ(reports.load(), 1);
        CHECK(gp::time::stop(t));
    }
    // a ticker that main waits on in a select: ticks come for as long as main wants them; after
    // Stop the select is asleep for ever
    {
        chan never = gp::make_chan<int64_t>();
        chan t = gp::make_chan<int64_t>(1);
        arm({never});
        start(t, now_ns() + 1'000'000, 1'000'000);
        for (int i = 0; i < 5; i++) {
            gp::sel_recv gp_s0{never, gp::sel_drop};
            gp::sel_recv gp_s1{t, gp::sel_drop};
            CHECK_EQ(gp::select(gp_s0, gp_s1), 1);
        }
        CHECK_EQ(reports.load(), 0);
        gp::time::stop(t);
        gp::sel_recv gp_s0{never, gp::sel_drop};
        gp::sel_recv gp_s1{t, gp::sel_drop};
        CHECK_EQ(gp::select(gp_s0, gp_s1), 0); // the hook closed `never`
        CHECK_EQ(reports.load(), 1);
    }
    // AfterFunc: asleep with one armed is fine; when it has run and its goroutine has ended, the
    // program is asleep
    {
        chan never = gp::make_chan<int64_t>();
        chan carrier = gp::make_chan<int64_t>();
        arm({never});
        gp::time::start_func<int64_t>(carrier, now_ns() + 20'000'000,
                                      gp::func<void()>([] { flag = 1; }));
        gp::recv(never);
        CHECK_EQ(reports.load(), 1);
        CHECK_EQ(flag_at_report.load(), 1); // not before the function ran
    }
    // time.Sleep is not asleep for ever
    {
        chan never = gp::make_chan<int64_t>();
        arm({});
        group g;
        g.go([never] {
            gp::time::sleep(30'000'000);
            send_ignoring_close(never, 1);
        });
        CHECK_EQ(gp::recv(never), int64_t(1));
        CHECK(g.wait());
        CHECK_EQ(reports.load(), 0);
    }
}

// White box: the main goroutine is marked blocked by hand (block_enter) and the detector is called:
// it reports, also while a panic is running (a panic's deferred calls that wait for ever are a
// deadlock, as in Go), and nothing is reported when no goroutine is blocked.
void test_guards() {
    // (goroutines of the tests before have ended, and are not counted any more)
    CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
    arm({});
    gp::detail::block_enter("sync.test");
    gp::detail::panic_count_add(1);
    gp::detail::deadlock_look();
    CHECK_EQ(reports.load(), 1);
    gp::detail::panic_count_add(-1);
    CHECK(contains(report_text(), "goroutine 1 [sync.test]:\n"));
    gp::detail::block_exit();
    // not blocked any more: nothing to report
    reports = 0;
    gp::detail::deadlock_look();
    CHECK_EQ(reports.load(), 0);
}

// A timer that the timer thread has taken out of the heap and is firing (its callback runs) can
// still wake a goroutine: it is neither in the heap nor in a channel.
std::atomic<int> inflight{0};

void test_timer_inflight() {
    arm({});
    chan t = gp::make_chan<int64_t>(1);
    inflight = 0;
    gp::time::start_chan<int64_t>(t, gp::time::nano() + 1'000'000, 0,
                                  gp::func<void(chan, int64_t)>([](chan ch, int64_t) {
                                      inflight = 1;
                                      while (inflight.load() != 2) {
                                          std::this_thread::yield();
                                      }
                                      gp::time::try_send(ch, int64_t(1));
                                  }));
    CHECK(wait_for([] { return inflight.load() == 1; }));
    CHECK_EQ(gp::detail::timers_armed(), std::size_t(0)); // it is not in the heap
    CHECK(gp::detail::timers_can_wake());                 // and it can still wake someone
    inflight = 2;
    CHECK(wait_for([] { return !gp::detail::timers_can_wake(); }));
    CHECK_EQ(gp::recv(t), int64_t(1));
    CHECK_EQ(reports.load(), 0);
}

// The wake-ups that are on the way: a ring of goroutines passes a token, each asleep most of the
// time, and a Mutex is handed from one waiter to the next: never a report.
struct Mutex {
    int32_t state = 0;
    uint32_t sema = 0;
};

void test_no_false_positive() {
    arm({});
    {
        constexpr int n = 8;
        constexpr int laps = 3000;
        std::vector<chan> ring;
        for (int i = 0; i < n; i++) {
            ring.push_back(gp::make_chan<int64_t>());
        }
        chan done = gp::make_chan<int64_t>();
        group g;
        for (int i = 0; i < n; i++) {
            chan in = ring[i];
            chan out = ring[(i + 1) % n];
            g.go([in, out, done, i] {
                for (int lap = 0; lap < laps; lap++) {
                    int64_t v = gp::recv(in);
                    if (i == n - 1 && lap == laps - 1) {
                        gp::send(done, v);
                        return;
                    }
                    gp::send(out, v + 1);
                }
            });
        }
        gp::send(ring[0], int64_t(0));
        CHECK_EQ(gp::recv(done), int64_t(n) * laps - 1);
        CHECK(g.wait());
    }
    {
        auto m = gp::new_<Mutex>();
        int64_t counter = 0;
        group g;
        for (int i = 0; i < 16; i++) {
            g.go([&, m] {
                for (int k = 0; k < 3000; k++) {
                    gp::sync::mutex_lock(m);
                    counter++;
                    gp::sync::mutex_unlock(m);
                }
            });
        }
        CHECK(g.wait());
        CHECK_EQ(counter, int64_t(16) * 3000);
    }
    {
        // goroutines that start and end while main sleeps on a channel: every end is a check
        chan done = gp::make_chan<int64_t>();
        for (int round = 0; round < 200; round++) {
            gp::go([done] { gp::send(done, int64_t(1)); });
            gp::recv(done);
        }
    }
    CHECK_EQ(reports.load(), 0);
}

// The registry is read while goroutines start and end: a goroutine's record must not be freed
// under the detector (run with the sanitizers).
void test_registry_race() {
    arm({});
    std::atomic<bool> stop{false};
    group looker;
    looker.go([&] {
        while (!stop.load()) {
            gp::detail::deadlock_look(); // finds goroutines running: reports nothing
        }
    });
    group g;
    for (int t = 0; t < 4; t++) {
        g.go([] {
            for (int i = 0; i < 2000; i++) {
                chan c = gp::make_chan<int64_t>(1);
                gp::go([c] { gp::send(c, int64_t(1)); });
                gp::recv(c);
            }
        });
    }
    CHECK(g.wait());
    stop = true;
    CHECK(looker.wait());
    CHECK_EQ(reports.load(), 0);
}

int callback_target(int x) {
    return x;
}

// The cases that end the process (deadlock_test.go runs each with its name as the only argument).
void run_case(const std::string& name) {
    auto out = [](const std::string& text) {
        gp::os::write(gp::os::stdout_fd, text.data(), text.size());
    };
    chan never = gp::make_chan<int64_t>();
    if (name == "chan") {
        gp::recv(never);
    } else if (name == "goroutines") {
        gp::go([] { gp::recv(chan()); });
        gp::go([never] { gp::send(never, int64_t(1)); });
        gp::recv(gp::make_chan<int64_t>());
    } else if (name == "goexit-none") {
        gp::go([out] { out("the goroutine runs\n"); });
        gp::detail::main_goexit();
    } else if (name == "goexit-ended") {
        // the goroutine has ended before main's Goexit: it is main_goexit that finds none left
        gp::go([out] { out("the goroutine ran\n"); });
        while (gp::num_goroutine() > 1) {
            std::this_thread::yield();
        }
        gp::detail::main_goexit();
    } else if (name == "goexit-asleep") {
        // the goroutine is asleep before main's Goexit: it is main_goexit that finds all asleep
        gp::go([never] { gp::recv(never); });
        while (gp::num_blocked() < 1) {
            std::this_thread::yield();
        }
        gp::detail::main_goexit();
    } else if (name == "goexit-asleep-late") {
        // the goroutine falls asleep after main's Goexit: it is the goroutine that finds all asleep
        gp::go([never] {
            gp::time::sleep(30'000'000);
            gp::recv(never);
        });
        gp::detail::main_goexit();
    } else if (name == "goexit-late") {
        // main is gone, the goroutine sleeps and ends: then none is left
        gp::go([out] {
            gp::time::sleep(50'000'000);
            out("slept\n");
        });
        gp::detail::main_goexit();
    } else if (name == "goexit-afterfunc") {
        // an armed AfterFunc starts a goroutine, so main's Goexit does not end the program yet
        chan carrier = gp::make_chan<int64_t>();
        gp::time::start_func<int64_t>(carrier, gp::time::nano() + 50'000'000,
                                      gp::func<void()>([out] { out("fired\n"); }));
        gp::detail::main_goexit();
    } else if (name == "foreign") {
        // a thread that no go statement started entered gocpp: it may wake main
        std::atomic<bool> entered{false};
        std::thread([out, &entered] {
            gp::detail::this_goroutine();
            entered = true;
            std::this_thread::sleep_for(200ms);
            out("survived\n");
            gp::exit(0);
        }).detach();
        // (a thread that has not entered yet is not known: wait until it has)
        while (!entered.load()) {
            std::this_thread::yield();
        }
        gp::recv(never);
    } else if (name == "callback") {
        // a callback that C could hold: C may call it from a thread of its own
        int (*p)(int) = gp::cgo::fn_lit<gp::func<int(int)>>([](int x) { return x; });
        (void)p;
        // (C's thread: it never calls into gocpp here, so it is the callback that tells the
        // detector)
        std::thread([out] {
            std::this_thread::sleep_for(200ms);
            out("survived\n");
            gp::exit(0);
        }).detach();
        gp::recv(never);
    } else if (name == "callback-fn" || name == "callback-fnval") {
        // the other ways to hand a Go function to C or C++ code: a function of the program, and a
        // func value (a C++ library may keep it and call it from a thread of its own)
        if (name == "callback-fn") {
            auto cb = gp::cgo::fn<&callback_target>();
            (void)cb;
        } else {
            auto cb = gp::cgo::fn_val(gp::func<int(int)>([](int x) { return x; }));
            (void)cb;
        }
        std::thread([out] {
            std::this_thread::sleep_for(200ms);
            out("survived\n");
            gp::exit(0);
        }).detach();
        gp::recv(never);
    } else if (name == "sleeping") {
        gp::go([out] {
            gp::time::sleep(100'000'000);
            out("woke\n");
            gp::exit(0);
        });
        gp::recv(never);
    } else if (name == "afterfunc") {
        chan carrier = gp::make_chan<int64_t>();
        gp::time::start_func<int64_t>(carrier, gp::time::nano() + 100'000'000,
                                      gp::func<void()>([out] {
                                          out("fired\n");
                                          gp::exit(0);
                                      }));
        gp::recv(never);
    } else if (name == "ticker") {
        chan t = gp::make_chan<int64_t>(1);
        gp::time::start_chan<int64_t>(t, gp::time::nano() + 1'000'000, 1'000'000,
                                      gp::func<void(chan, int64_t)>([](chan ch, int64_t) {
                                          gp::time::try_send(ch, int64_t(1));
                                      }));
        gp::recv(never);
    } else if (name == "timer-wait") {
        // main waits on a timer channel: it is not a deadlock, the timer fires
        chan t = gp::make_chan<int64_t>(1);
        gp::time::start_chan<int64_t>(t, gp::time::nano() + 50'000'000, 0,
                                      gp::func<void(chan, int64_t)>([](chan ch, int64_t) {
                                          gp::time::try_send(ch, int64_t(1));
                                      }));
        gp::recv(t);
        out("fired\n");
    } else if (name == "main-returns") {
        gp::go([never] { gp::recv(never); });
        gp::go([] { gp::recv(chan()); });
        gp::time::sleep(20'000'000);
        out("main returns\n");
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
    gp::os::exit(0);
}

void run_unit_tests() {
    CHECK(gp::detail::deadlock_detector_enabled()); // run_main turned it on
    CHECK(!gp::detail::foreign_entered());
    gp::detail::set_deadlock_hook(&on_report);
    test_report();
    test_running_goroutine();
    test_timers();
    test_timer_inflight();
    test_guards();
    test_no_false_positive();
    test_registry_race();
    test_many_goroutines();
    test_nil_reasons(); // last: it leaves two goroutines asleep for ever
    gp::detail::set_deadlock_hook(nullptr);
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
    // A detector that does not report leaves a test asleep for ever: this thread (a plain
    // std::thread that never enters gocpp, so it is not a foreign thread for the detector) ends the
    // process, so that a failure is a failure, not a hang.
    std::thread([] {
        std::this_thread::sleep_for(90s);
        say("deadlock_test: timed out (a detector that did not report?)\n");
        gp::os::exit_now(97);
    }).detach();
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
