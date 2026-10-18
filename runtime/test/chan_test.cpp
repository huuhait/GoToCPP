// The unit tests of channels (gocpp/chan.hpp, src/chan.cpp) and of the thread reuse of
// goroutines (src/goroutine.cpp), Phase 08 steps 8.2 and 8.3.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and with ThreadSanitizer
// (chan_test.go), and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or
// prints each failed check and exits 1.
//
// Nothing here depends on timing. Threads synchronize through channels, atomics and
// gp::num_blocked() (a goroutine that waits on a channel is counted there before it
// sleeps, so a test can wait until a goroutine is blocked); a wait has a limit of minutes, so a
// hang is a failure and a slow machine is not.
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

// panic_text runs f and returns the text of the panic it ends with, or "(no panic)".
std::string panic_text(const std::function<void()>& f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        return std::string(gp::detail::panic_text(e).gp_view());
    }
    return "(no panic)";
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

// --- the nil channel, make, len and cap ---

void test_nil_and_make() {
    gp::chan<int64_t> nil;
    CHECK(nil == nullptr);
    CHECK(gp::chan<int64_t>(nullptr) == nil);
    CHECK_EQ(gp::len(nil), int64_t(0));
    CHECK_EQ(gp::cap(nil), int64_t(0));
    CHECK_STR(panic_text([&] { gp::close(nil); }), "close of nil channel");

    auto a = gp::make_chan<int64_t>();
    auto b = gp::make_chan<int64_t>(3);
    CHECK(a != nullptr);
    CHECK(!(a == nil));
    CHECK(a == a);
    CHECK(a != b);
    gp::chan<int64_t> a2 = a;
    CHECK(a2 == a);
    CHECK_EQ(gp::cap(a), int64_t(0));
    CHECK_EQ(gp::cap(b), int64_t(3));
    CHECK_EQ(gp::len(b), int64_t(0));
    // The hash is the address: copies hash alike.
    CHECK_EQ(gp::hash_of(a), gp::hash_of(a2));
    CHECK(gp::hash_of(a) != gp::hash_of(b));
    CHECK_EQ(gp::hash_of(nil), gp::hash_of(gp::chan<int64_t>()));
    CHECK(a.gp_address() != 0 && nil.gp_address() == 0);

    CHECK_STR(panic_text([] { gp::make_chan<int64_t>(-1); }), "makechan: size out of range");
    CHECK_STR(panic_text([] { gp::make_chan<int64_t>(int64_t(1) << 60); }),
              "makechan: size out of range");
    CHECK_STR(panic_text([] { gp::make_chan<int64_t>(5); }), "(no panic)");

    // The panics are runtime.Error values (plainError): recover() sees an error with the text.
    try {
        gp::close(nil);
        CHECK(false);
    } catch (const gp::panic_error& e) {
        CHECK(!(e.gp_value == nullptr));
    }

    // Channels are map keys.
    auto m = gp::make_map<gp::chan<int64_t>, int64_t>();
    gp::set(m, a, int64_t(1));
    gp::set(m, b, int64_t(2));
    gp::set(m, nil, int64_t(3));
    CHECK_EQ(gp::get(m, a2), int64_t(1));
    CHECK_EQ(gp::get(m, b), int64_t(2));
    CHECK_EQ(gp::get(m, gp::chan<int64_t>()), int64_t(3));
}

// --- a buffered channel on one thread: the ring buffer ---

void test_buffered_single_thread() {
    auto c = gp::make_chan<int64_t>(4);
    auto* core = c.gp_core();
    int64_t v = 0;
    // Empty: a receive would block; a full channel's send would block.
    CHECK(core->try_recv(&v) == gp::detail::recv_status::not_ready);
    for (int64_t i = 1; i <= 4; i++) {
        int64_t x = i * 10;
        CHECK(core->try_send(&x));
        CHECK_EQ(gp::len(c), i);
    }
    int64_t extra = 99;
    CHECK(!core->try_send(&extra));
    CHECK_EQ(gp::len(c), int64_t(4));
    // First in, first out; the ring wraps many times.
    int64_t next_in = 5;
    int64_t next_out = 1;
    for (int round = 0; round < 100; round++) {
        CHECK_EQ(gp::recv(c), next_out * 10);
        next_out++;
        gp::send(c, next_in * 10);
        next_in++;
        CHECK_EQ(gp::len(c), int64_t(4));
    }
    for (; next_out < next_in; next_out++) {
        auto [x, ok] = gp::recv2(c);
        CHECK(ok);
        CHECK_EQ(x, next_out * 10);
    }
    CHECK_EQ(gp::len(c), int64_t(0));
    CHECK_EQ(gp::cap(c), int64_t(4));
    // Partially full, with the indexes in the middle of the ring.
    gp::send(c, int64_t(1));
    gp::send(c, int64_t(2));
    gp::send(c, int64_t(3));
    CHECK_EQ(gp::recv(c), int64_t(1));
    gp::send(c, int64_t(4));
    gp::send(c, int64_t(5));
    CHECK_EQ(gp::len(c), int64_t(4));
    for (int64_t i = 2; i <= 5; i++) {
        CHECK_EQ(gp::recv(c), i);
    }
    // cap(0) and a ring of one element.
    auto one = gp::make_chan<int64_t>(1);
    for (int64_t i = 0; i < 50; i++) {
        gp::send(one, i);
        CHECK_EQ(gp::len(one), int64_t(1));
        CHECK_EQ(gp::recv(one), i);
    }
}

// --- close ---

void test_close_single_thread() {
    auto c = gp::make_chan<int64_t>(3);
    gp::send(c, int64_t(7));
    gp::send(c, int64_t(8));
    gp::close(c);
    CHECK_STR(panic_text([&] { gp::close(c); }), "close of closed channel");
    CHECK_STR(panic_text([&] { gp::send(c, int64_t(1)); }), "send on closed channel");
    // The buffered values come first, ok is true; then zero values and false, for ever.
    CHECK_EQ(gp::len(c), int64_t(2));
    {
        auto [v, ok] = gp::recv2(c);
        CHECK(ok && v == 7);
    }
    CHECK_EQ(gp::recv(c), int64_t(8));
    for (int i = 0; i < 3; i++) {
        auto [v, ok] = gp::recv2(c);
        CHECK(!ok && v == 0);
        CHECK(!gp::recv_drop(c));
    }
    CHECK_EQ(gp::len(c), int64_t(0));
    CHECK_EQ(gp::cap(c), int64_t(3));
    // A closed unbuffered channel.
    auto u = gp::make_chan<gp::string>();
    gp::close(u);
    auto [s, ok] = gp::recv2(u);
    CHECK(!ok && s.gp_view().empty());
}

// --- element types ---

struct big {
    gp::string name;
    std::array<int64_t, 6> nums{};
    gp::slice<int64_t> list;
};

// counted counts its live instances, so a test can see that the channel destroys what it holds.
std::atomic<int> live_counted{0};
struct counted {
    int v = 0;
    counted() { live_counted++; }
    explicit counted(int x) : v(x) { live_counted++; }
    counted(const counted& o) : v(o.v) { live_counted++; }
    counted(counted&& o) noexcept : v(o.v) {
        o.v = -1;
        live_counted++;
    }
    ~counted() { live_counted--; }
};

// no_zero has no default constructor: a channel of it works until a receive from a closed channel.
struct no_zero {
    explicit no_zero(int x) : v(x) {}
    int v;
};

void test_element_types() {
    // Strings and structs are copied in and moved out.
    auto cs = gp::make_chan<gp::string>(2);
    gp::string s = gp::string::gp_copy("hello, channel");
    gp::send(cs, s);
    CHECK(gp::recv(cs).gp_view() == "hello, channel");
    CHECK(s.gp_view() == "hello, channel"); // the sender still has its value

    auto cb = gp::make_chan<big>(2);
    big b;
    b.name = gp::string::gp_copy("big");
    b.nums = {1, 2, 3, 4, 5, 6};
    b.list = gp::slice_of<int64_t>({7, 8, 9});
    gp::send(cb, b);
    big r = gp::recv(cb);
    CHECK(r.name.gp_view() == "big");
    CHECK_EQ(r.nums[5], int64_t(6));
    CHECK_EQ(gp::len(r.list), int64_t(3));
    CHECK_EQ(b.nums[0], int64_t(1));

    // A move-only element: unique_ptr.
    auto cu = gp::make_chan<std::unique_ptr<int>>(1);
    gp::send(cu, std::make_unique<int>(5));
    auto up = gp::recv(cu);
    CHECK(up != nullptr && *up == 5);

    // Zero-size types are elements too: chan struct{}.
    struct empty {};
    auto ce = gp::make_chan<empty>(3);
    gp::send(ce, empty{});
    gp::send(ce, empty{});
    CHECK_EQ(gp::len(ce), int64_t(2));
    gp::recv(ce);
    CHECK_EQ(gp::len(ce), int64_t(1));

    // A channel of channels, and of pointers.
    auto cc = gp::make_chan<gp::chan<int64_t>>(1);
    auto inner = gp::make_chan<int64_t>(1);
    gp::send(cc, inner);
    auto got = gp::recv(cc);
    CHECK(got == inner);
    gp::send(got, int64_t(42));
    CHECK_EQ(gp::recv(inner), int64_t(42));

    // Everything a channel holds is destroyed with it, and nothing twice.
    CHECK_EQ(live_counted.load(), 0);
    {
        auto cn = gp::make_chan<counted>(5);
        for (int i = 0; i < 5; i++) {
            gp::send(cn, counted(i));
        }
        CHECK_EQ(live_counted.load(), 5);
        CHECK_EQ(gp::recv(cn).v, 0);
        CHECK_EQ(live_counted.load(), 4);
        gp::send(cn, counted(9)); // the ring is wrapped when the channel is destroyed
        CHECK_EQ(live_counted.load(), 5);
        gp::close(cn);
        CHECK_EQ(live_counted.load(), 5);
    }
    CHECK_EQ(live_counted.load(), 0);
    {
        // An unbuffered channel and a blocked sender: the sender's own copy is destroyed once.
        auto cn = gp::make_chan<counted>();
        group g;
        g.go([&] { gp::send(cn, counted(3)); });
        CHECK(blocked_is(1));
        CHECK_EQ(gp::recv(cn).v, 3);
        g.wait();
    }
    CHECK_EQ(live_counted.load(), 0);

    // An element without a default constructor: fine, until a receive from a closed channel.
    auto cz = gp::make_chan<no_zero>(1);
    gp::send(cz, no_zero(4));
    CHECK_EQ(gp::recv(cz).v, 4);
}

// --- goroutines and channels ---

// A sender on an unbuffered channel is blocked until a receiver takes the value; the receive
// happens before the send completes.
void test_unbuffered_handoff() {
    auto c = gp::make_chan<int64_t>();
    std::atomic<bool> sent{false};
    group g;
    g.go([&] {
        gp::send(c, int64_t(11));
        sent.store(true);
    });
    CHECK(blocked_is(1)); // the sender waits in the channel...
    CHECK(!sent.load());  // ...and has not finished
    CHECK_EQ(gp::len(c), int64_t(0));
    CHECK_EQ(gp::recv(c), int64_t(11));
    g.wait();
    CHECK(sent.load());
    CHECK(blocked_is(0));

    // The other way round: the receiver waits first, the sender hands the value over and goes on.
    std::atomic<int64_t> got{-1};
    g.go([&] { got.store(gp::recv(c)); });
    CHECK(blocked_is(1));
    CHECK_EQ(got.load(), int64_t(-1));
    gp::send(c, int64_t(12)); // does not block: a receiver is there
    g.wait();
    CHECK_EQ(got.load(), int64_t(12));
}

// Blocked goroutines are woken in the order they blocked (the queues are first in, first out).
void test_fifo_wakeup() {
    constexpr int n = 24;
    // Senders on an unbuffered channel.
    {
        auto c = gp::make_chan<int64_t>();
        group g;
        for (int i = 0; i < n; i++) {
            g.go([&, i] { gp::send(c, int64_t(i)); });
            CHECK(blocked_is(i + 1)); // it has queued, so the next one queues after it
        }
        for (int i = 0; i < n; i++) {
            CHECK_EQ(gp::recv(c), int64_t(i));
        }
        g.wait();
    }
    // Senders on a full buffered channel: the buffer first, then the senders in order.
    {
        auto c = gp::make_chan<int64_t>(3);
        for (int i = 0; i < 3; i++) {
            gp::send(c, int64_t(100 + i));
        }
        group g;
        for (int i = 0; i < n; i++) {
            g.go([&, i] { gp::send(c, int64_t(i)); });
            CHECK(blocked_is(i + 1));
        }
        for (int i = 0; i < 3; i++) {
            CHECK_EQ(gp::recv(c), int64_t(100 + i));
        }
        for (int i = 0; i < n; i++) {
            CHECK_EQ(gp::recv(c), int64_t(i));
        }
        g.wait();
    }
    // Receivers: the first one that blocked gets the first value.
    {
        auto c = gp::make_chan<int64_t>();
        std::vector<std::atomic<int64_t>> got(n);
        for (auto& x : got) {
            x = -1;
        }
        group g;
        for (int i = 0; i < n; i++) {
            g.go([&, i] { got[i].store(gp::recv(c)); });
            CHECK(blocked_is(i + 1));
        }
        for (int i = 0; i < n; i++) {
            gp::send(c, int64_t(i));
        }
        g.wait();
        for (int i = 0; i < n; i++) {
            CHECK_EQ(got[i].load(), int64_t(i));
        }
    }
}

// close wakes every receiver (zero value, false) and every sender (they panic).
void test_close_wakes() {
    constexpr int n = 16;
    {
        auto c = gp::make_chan<int64_t>();
        std::atomic<int> zero_false{0};
        group g;
        for (int i = 0; i < n; i++) {
            g.go([&] {
                auto [v, ok] = gp::recv2(c);
                if (v == 0 && !ok) {
                    zero_false++;
                }
            });
        }
        CHECK(blocked_is(n));
        gp::close(c);
        g.wait();
        CHECK_EQ(zero_false.load(), n);
    }
    {
        auto c = gp::make_chan<int64_t>(2);
        gp::send(c, int64_t(1));
        gp::send(c, int64_t(2));
        std::atomic<int> panicked{0};
        group g;
        for (int i = 0; i < n; i++) {
            g.go([&] {
                std::string t = panic_text([&] { gp::send(c, int64_t(9)); });
                if (t == "send on closed channel") {
                    panicked++;
                }
            });
        }
        CHECK(blocked_is(n));
        gp::close(c);
        g.wait();
        CHECK_EQ(panicked.load(), n);
        // The buffered values are still there for the receivers.
        CHECK_EQ(gp::recv(c), int64_t(1));
        CHECK_EQ(gp::recv(c), int64_t(2));
        CHECK(!std::get<1>(gp::recv2(c)));
    }
}

// Two goroutines pass a value back and forth.
void test_pingpong() {
    constexpr int rounds = 5000;
    auto ping = gp::make_chan<int64_t>();
    auto pong = gp::make_chan<int64_t>();
    group g;
    g.go([&] {
        for (int i = 0; i < rounds; i++) {
            gp::send(pong, gp::recv(ping) + 1);
        }
    });
    int64_t v = 0;
    for (int i = 0; i < rounds; i++) {
        gp::send(ping, v);
        v = gp::recv(pong);
    }
    g.wait();
    CHECK_EQ(v, int64_t(rounds));
}

// Many senders and many receivers: every value arrives exactly once. The unbuffered and the
// buffered case; the receivers stop when the channel is closed after all senders are done.
void mpmc(int64_t capacity, int senders, int receivers, int each) {
    auto c = gp::make_chan<int64_t>(capacity);
    std::atomic<int64_t> sum{0};
    std::atomic<int64_t> count{0};
    std::vector<std::atomic<int>> seen(static_cast<std::size_t>(senders) * each);
    group send_group;
    group recv_group;
    for (int s = 0; s < senders; s++) {
        send_group.go([&, s] {
            for (int i = 0; i < each; i++) {
                gp::send(c, int64_t(s) * each + i);
            }
        });
    }
    for (int r = 0; r < receivers; r++) {
        recv_group.go([&] {
            for (;;) {
                auto [v, ok] = gp::recv2(c);
                if (!ok) {
                    return;
                }
                sum += v;
                count++;
                seen[static_cast<std::size_t>(v)]++;
            }
        });
    }
    send_group.wait();
    gp::close(c);
    recv_group.wait();
    int64_t total = int64_t(senders) * each;
    CHECK_EQ(count.load(), total);
    CHECK_EQ(sum.load(), total * (total - 1) / 2);
    bool once = true;
    for (auto& x : seen) {
        once = once && x.load() == 1;
    }
    CHECK(once);
}

void test_mpmc() {
    mpmc(0, 4, 4, 2000);
    mpmc(1, 4, 4, 2000);
    mpmc(16, 6, 3, 2000);
    mpmc(1000, 3, 6, 2000);
    mpmc(0, 1, 8, 3000);
    mpmc(0, 8, 1, 3000);
}

// A per-sender sequence stays in order through a channel (FIFO per sender), with several
// senders on one buffered channel and one receiver.
void test_per_sender_order() {
    constexpr int senders = 4;
    constexpr int each = 3000;
    auto c = gp::make_chan<int64_t>(8);
    group g;
    for (int s = 0; s < senders; s++) {
        g.go([&, s] {
            for (int i = 0; i < each; i++) {
                gp::send(c, int64_t(s) * 1000000 + i);
            }
        });
    }
    std::vector<int64_t> last(senders, -1);
    bool ordered = true;
    for (int k = 0; k < senders * each; k++) {
        int64_t v = gp::recv(c);
        int64_t s = v / 1000000;
        int64_t i = v % 1000000;
        ordered = ordered && i == last[static_cast<std::size_t>(s)] + 1;
        last[static_cast<std::size_t>(s)] = i;
    }
    g.wait();
    CHECK(ordered);
}

// close races with sends: every send either completes (and its value is received or still
// buffered) or panics with "send on closed channel" exactly once; no sender is stuck.
void test_close_races_send() {
    for (int round = 0; round < 40; round++) {
        constexpr int senders = 6;
        constexpr int each = 200;
        auto c = gp::make_chan<int64_t>(round % 3 == 0 ? 0 : 4);
        std::atomic<int> ok_sends{0};
        std::atomic<int> panics{0};
        std::atomic<int> received{0};
        group sg;
        for (int s = 0; s < senders; s++) {
            sg.go([&] {
                for (int i = 0; i < each; i++) {
                    std::string t = panic_text([&] { gp::send(c, int64_t(i)); });
                    if (t == "(no panic)") {
                        ok_sends++;
                    } else if (t == "send on closed channel") {
                        panics++;
                        return; // this sender stops, as a Go program that recovers would
                    } else {
                        say("unexpected panic: " + t + "\n");
                        failures++;
                        return;
                    }
                }
            });
        }
        group rg;
        rg.go([&] {
            for (;;) {
                auto [v, ok] = gp::recv2(c);
                if (!ok) {
                    return;
                }
                received++;
            }
        });
        // Let some sends happen, then close.
        wait_for([&] { return ok_sends.load() > round * 3; });
        gp::close(c);
        sg.wait();
        rg.wait();
        // Every successful send was received, and a sender that did not finish its loop panicked.
        CHECK_EQ(received.load(), ok_sends.load());
        CHECK(panics.load() <= senders);
        CHECK(panics.load() > 0 || ok_sends.load() == senders * each);
        CHECK(blocked_is(0));
    }
}

// Blocked goroutines are counted, with the reason, for the deadlock detector (step 8.11).
void test_blocked_accounting() {
    CHECK_EQ(gp::num_blocked(), int64_t(0));
    auto c = gp::make_chan<int64_t>();
    auto d = gp::make_chan<int64_t>(1);
    std::atomic<gp::detail::goroutine*> g1{nullptr};
    std::atomic<gp::detail::goroutine*> g2{nullptr};
    group g;
    g.go([&] {
        g1.store(&gp::detail::this_goroutine());
        gp::recv(c);
    });
    CHECK(blocked_is(1));
    g.go([&] {
        g2.store(&gp::detail::this_goroutine());
        gp::send(d, int64_t(1));
        gp::send(d, int64_t(2)); // blocks: the buffer is full
    });
    CHECK(blocked_is(2));
    CHECK(std::string(g1.load()->reason()) == "chan receive");
    CHECK(std::string(g2.load()->reason()) == "chan send");
    CHECK_EQ(gp::num_blocked(), int64_t(2));
    gp::send(c, int64_t(0));
    // The receiver was handed its value: it is not blocked any more, though its thread may not
    // have run yet (a deadlock detector must not count it as asleep).
    CHECK_EQ(gp::num_blocked(), int64_t(1));
    gp::recv(d); // takes the buffered value, and the blocked sender's value goes in
    CHECK_EQ(gp::num_blocked(), int64_t(0));
    gp::recv(d);
    g.wait();
    CHECK(blocked_is(0));
    CHECK(gp::detail::this_goroutine().reason() == nullptr);
}

// --- the entries that select (step 8.5) queues: one per case, with a claim flag they share ---

void test_waitq_claim() {
    using namespace gp::detail;
    waitq q;
    std::atomic<uint32_t> claim_a{0};
    std::atomic<uint32_t> claim_b{0};
    sudog s1, s2, s3, s4;
    s1.claim = &claim_a; // a select's first case
    s2.claim = &claim_b; // another select
    s3.claim = &claim_a; // the second case of the first select, on the same queue
    q.enqueue(&s1);
    q.enqueue(&s2);
    q.enqueue(&s3);
    q.enqueue(&s4); // a plain entry
    CHECK(q.dequeue() == &s1);
    CHECK_EQ(claim_a.load(), 1u);
    CHECK(q.dequeue() == &s2);
    CHECK(q.dequeue() == &s4); // s3 is skipped and dropped: its select is taken
    CHECK(q.dequeue() == nullptr);
    CHECK(q.first == nullptr && q.last == nullptr);
    // remove takes an entry out of the middle, the first and the last
    q.enqueue(&s1);
    q.enqueue(&s2);
    q.enqueue(&s3);
    q.remove(&s2);
    CHECK(q.first == &s1 && q.last == &s3 && s1.next == &s3 && s3.prev == &s1);
    q.remove(&s1);
    CHECK(q.first == &s3 && q.last == &s3);
    q.remove(&s3);
    CHECK(q.first == nullptr && q.last == nullptr);
}

// A goroutine that waits on two channels at once, as select will: one entry in each channel's
// receive queue, one cell, one claim flag. The first sender completes it; the second finds the
// other entry claimed and does not send to it.
void test_select_like_waiter() {
    using namespace gp::detail;
    for (int round = 0; round < 2; round++) {
        auto a = gp::make_chan<int64_t>();
        auto b = gp::make_chan<int64_t>();
        chan_core* ca = a.gp_core();
        chan_core* cb = b.gp_core();
        std::atomic<uint32_t> claim{0};
        wake_cell cell;
        sudog sa, sb;
        int64_t va = -1;
        int64_t vb = -1;
        std::atomic<bool> queued{false};
        int fired = -1;
        bool left_in_b = true;
        group g;
        g.go([&] {
            block_scope blocked("select");
            sa.g = sb.g = &this_goroutine();
            sa.w = sb.w = &this_goroutine().park;
            sa.cell = sb.cell = &cell;
            sa.claim = sb.claim = &claim;
            sa.case_index = 0;
            sb.case_index = 1;
            sa.elem = &va;
            sb.elem = &vb;
            sa.c = ca;
            sb.c = cb;
            {
                std::scoped_lock lk(ca->mu, cb->mu);
                ca->recvq.enqueue(&sa);
                cb->recvq.enqueue(&sb);
            }
            queued.store(true);
            park_cell(cell);
            fired = cell.fired->case_index;
            // Like Go's selectgo: lock again and take the entries that did not fire out.
            std::scoped_lock lk(ca->mu, cb->mu);
            left_in_b = cb->recvq.first == &sb;
            if (left_in_b) {
                cb->recvq.remove(&sb);
            }
        });
        CHECK(wait_for([&] { return queued.load(); }));
        int64_t sent = round == 0 ? 7 : 8;
        CHECK(ca->try_send(&sent));
        int64_t x = 99;
        if (round == 0) {
            // The send to b finds b's entry claimed already: not sent (b has no buffer).
            CHECK(!cb->try_send(&x));
        }
        g.wait();
        CHECK_EQ(fired, 0);
        CHECK_EQ(va, round == 0 ? int64_t(7) : int64_t(8));
        CHECK_EQ(vb, int64_t(-1));
        // Round 1: nobody else touched b, so the goroutine removes its entry. (Round 0: the send
        // to b may or may not have dropped it first.)
        CHECK(round == 0 || left_in_b);
        CHECK(ca->recvq.first == nullptr && cb->recvq.first == nullptr);
    }
}

// --- the count of panics that main's exit waits for (settle_panics) ---

// recovers runs f in a frame whose deferred call recovers a panic that f starts.
template <typename F>
void recovering(F f) {
    gp::defer_frame df;
    try {
        df.gp_push_call_aware([] {
            gp::defer_ctx dc = gp::take_defer_ctx();
            gp::recover(dc);
        });
        f();
    } catch (...) {
        df.gp_on_exception();
    }
    df.gp_run();
}

// The count follows the panic object: it is 0 again when the panic is recovered (also a nested one,
// and one that a deferred call starts), and when a catch outside the defer machinery swallows it
// (fmt prints %!v(PANIC=String method: ...) for a String method that panics).
void test_panic_count() {
    CHECK_EQ(gp::detail::panicking(), int64_t(0));
    int64_t waits = gp::detail::settle_waits();
    recovering([] { gp::panic("one"_gs); });
    CHECK_EQ(gp::detail::panicking(), int64_t(0));
    recovering([] { recovering([] { gp::panic("inner"_gs); }); });
    CHECK_EQ(gp::detail::panicking(), int64_t(0));
    // A panic in a deferred call that runs for another panic, both recovered by an outer frame.
    recovering([] {
        gp::defer_frame df;
        try {
            df.gp_push_call([] { gp::panic("second"_gs); });
            gp::panic("first"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(gp::detail::panicking(), int64_t(0));
    // Swallowed: the frame takes the panic (counted), runs its calls and rethrows it to C++ code
    // that catches it and goes on.
    {
        bool caught = false;
        try {
            gp::defer_frame df;
            try {
                df.gp_push_call([] {});
                gp::panic("swallowed"_gs);
            } catch (...) {
                df.gp_on_exception();
            }
            df.gp_run();
        } catch (const gp::panic_error& e) {
            caught = true;
            CHECK_EQ(gp::detail::panicking(), int64_t(1)); // alive in the handler
        }
        CHECK(caught);
    }
    CHECK_EQ(gp::detail::panicking(), int64_t(0));
    // The same on a goroutine that ends after it swallowed one.
    group g;
    g.go([] {
        try {
            gp::defer_frame df;
            try {
                gp::panic("swallowed in a goroutine"_gs);
            } catch (...) {
                df.gp_on_exception();
            }
            df.gp_run();
        } catch (const gp::panic_error&) {
        }
    });
    g.wait();
    CHECK_EQ(gp::detail::panicking(), int64_t(0));
    // Main's exit finds nothing to wait for.
    gp::detail::settle_panics();
    CHECK_EQ(gp::detail::settle_waits(), waits);
}

// --- thread reuse ---

void test_thread_reuse() {
    // Wait for the idle threads of earlier tests, then start with none.
    gp::detail::stop_idle_threads();
    CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
    gp::detail::set_idle_limit(gp::detail::default_idle_limit);

    // One goroutine at a time: its thread goes idle and the next goroutine runs on it.
    std::mutex mu;
    std::set<std::thread::id> threads;
    auto run_one = [&] {
        auto c = gp::make_chan<int64_t>();
        gp::go([&, c] {
            {
                std::lock_guard<std::mutex> lk(mu);
                threads.insert(std::this_thread::get_id());
            }
            gp::send(c, int64_t(1));
        });
        gp::recv(c);
        // The goroutine's thread is idle again a moment after it sent.
        CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
    };
    run_one();
    CHECK_EQ(threads.size(), std::size_t(1));
    CHECK(wait_for([] { return gp::detail::idle_threads() == 1; }));
    for (int i = 0; i < 300; i++) {
        run_one();
        CHECK(wait_for([] { return gp::detail::idle_threads() == 1; }));
    }
    CHECK_EQ(threads.size(), std::size_t(1)); // one thread ran all 301 goroutines
    // Ids are not reused: each goroutine has an id of its own.
    std::vector<uint64_t> ids;
    for (int i = 0; i < 20; i++) {
        auto c = gp::make_chan<uint64_t>();
        gp::go([c] { gp::send(c, gp::detail::goroutine_id()); });
        ids.push_back(gp::recv(c));
        CHECK(wait_for([] { return gp::detail::idle_threads() == 1; }));
    }
    std::set<uint64_t> distinct(ids.begin(), ids.end());
    CHECK_EQ(distinct.size(), ids.size());

    // Goroutines that live at the same time need a thread each; they all go idle afterwards, up
    // to the limit; the rest end.
    gp::detail::set_idle_limit(5);
    constexpr int n = 12;
    auto gate = gp::make_chan<int64_t>();
    group g;
    for (int i = 0; i < n; i++) {
        g.go([&] { gp::recv(gate); });
    }
    CHECK(blocked_is(n));
    gp::close(gate);
    g.wait();
    CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
    CHECK(wait_for([] { return gp::detail::idle_threads() <= 5; }));
    CHECK(wait_for([] { return gp::detail::idle_threads() == 5; }));

    // A limit of 0 ends the idle threads, and goroutines then end with their thread.
    gp::detail::set_idle_limit(0);
    CHECK(wait_for([] { return gp::detail::idle_threads() == 0; }));
    run_one();
    CHECK_EQ(gp::detail::idle_threads(), std::size_t(0));
    gp::detail::set_idle_limit(gp::detail::default_idle_limit);

    // A goroutine's state is clean on a reused thread: the goroutine record is new, a panic that
    // was recovered earlier leaves nothing behind, the waiter has no stale permit.
    for (int i = 0; i < 50; i++) {
        auto c = gp::make_chan<int64_t>();
        gp::go([c] {
            std::string t = panic_text([&] { gp::close(gp::chan<int64_t>()); });
            gp::send(c, t == "close of nil channel" ? 1 : 0);
        });
        CHECK_EQ(gp::recv(c), int64_t(1));
    }
    // Many short goroutines in a loop, 20000 of them: a few threads run them.
    std::atomic<int> sum{0};
    auto done = gp::make_chan<int64_t>(64);
    int started = 0;
    for (int i = 0; i < 20000; i++) {
        gp::go([&sum, done, i] {
            sum += i % 7;
            gp::send(done, int64_t(1));
        });
        started++;
        if (started >= 64) {
            gp::recv(done);
            started--;
        }
    }
    for (; started > 0; started--) {
        gp::recv(done);
    }
    int expect = 0;
    for (int i = 0; i < 20000; i++) {
        expect += i % 7;
    }
    CHECK_EQ(sum.load(), expect);
    CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
    gp::detail::stop_idle_threads();
}

// gp::go_fn and gp::go_call: the function and the arguments are the goroutine's copies.
std::atomic<int64_t> go_fn_sum{0};
void add_to_sum(int64_t a, gp::string s, int64_t b) {
    go_fn_sum += a + b + static_cast<int64_t>(s.gp_view().size());
}

void test_go_calls() {
    go_fn_sum = 0;
    auto done = gp::make_chan<int64_t>(8);
    // The argument is converted to the parameter type in the caller, and copied: changing the
    // variable afterwards does not change what the goroutine sees.
    int64_t a = 10;
    gp::string s = gp::string::gp_copy("abc");
    gp::go_fn(&add_to_sum, a, s, int64_t(5));
    a = 1000;
    gp::go_call(
        [done](int64_t x, gp::string t) { gp::send(done, x + int64_t(t.gp_view().size())); },
        int64_t(7), gp::string::gp_copy("xy"));
    CHECK_EQ(gp::recv(done), int64_t(9));
    CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
    CHECK_EQ(go_fn_sum.load(), int64_t(10 + 5 + 3));
    // A move-only argument goes to the goroutine.
    gp::go_call([done](std::unique_ptr<int> p) { gp::send(done, int64_t(*p)); },
                std::make_unique<int>(33));
    CHECK_EQ(gp::recv(done), int64_t(33));
    // A closure with state (a mutable lambda) is called once, as an rvalue.
    gp::go_call([done, n = 3]() mutable {
        n++;
        gp::send(done, int64_t(n));
    });
    CHECK_EQ(gp::recv(done), int64_t(4));
    // The goroutine ends with a Goexit: the deferred part runs and the goroutine is gone.
    gp::go([done] {
        gp::defer_frame df;
        try {
            df.gp_push_call([done](auto&&...) { gp::send(done, int64_t(77)); });
            throw gp::goexit_signal{};
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(gp::recv(done), int64_t(77));
    CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
}

// A goroutine that wakes another is not counted as blocked from the moment it wakes it, before the
// woken thread runs (the deadlock detector of step 8.11 reads num_blocked): the woken goroutine
// here is held back from running by a gate, and the count is read while it is held.
void test_unblock_by_waker() {
    using namespace gp::detail;
    auto c = gp::make_chan<int64_t>();
    chan_core* core = c.gp_core();
    wake_cell cell;
    sudog sg;
    int64_t v = -1;
    std::atomic<bool> queued{false};
    std::atomic<bool> go_on{false};
    group g;
    g.go([&] {
        block_scope blocked("chan receive");
        sg.g = &this_goroutine();
        sg.w = &sg.g->park;
        sg.cell = &cell;
        sg.elem = &v;
        sg.c = core;
        {
            std::lock_guard<std::mutex> lk(core->mu);
            core->recvq.enqueue(&sg);
        }
        queued.store(true);
        while (!go_on.load()) { // (it is not parked yet: the waker's wake-up is a permit)
            std::this_thread::sleep_for(100us);
        }
        park_cell(cell);
    });
    CHECK(wait_for([&] { return queued.load(); }));
    CHECK_EQ(gp::num_blocked(), int64_t(1));
    int64_t x = 5;
    CHECK(core->try_send(&x));               // hands the value over and wakes the goroutine
    CHECK_EQ(gp::num_blocked(), int64_t(0)); // not blocked any more, though it has not run
    go_on.store(true);
    g.wait();
    CHECK_EQ(v, int64_t(5));
    CHECK_EQ(gp::num_blocked(), int64_t(0));
}

// The idle thread that times out while a `go` statement takes it (the worker is already off the
// list, and its goroutine is on the way): the timeout is cut to a few microseconds and goroutines
// are started from four threads with random pauses, so that the two meet. Every goroutine must run
// exactly once, and nothing may hang.
void test_idle_timeout_race() {
    gp::detail::stop_idle_threads();
    for (int64_t timeout_ns : {15'000, 300'000}) {
        int64_t old = gp::detail::set_idle_timeout(timeout_ns);
        constexpr int starters = 4;
        constexpr int each = 1500;
        std::atomic<int> ran{0};
        std::vector<std::thread> ts;
        for (int s = 0; s < starters; s++) {
            ts.emplace_back([&, s] {
                uint32_t x = 2463534242u + static_cast<uint32_t>(s);
                for (int i = 0; i < each; i++) {
                    gp::go([&ran] { ran.fetch_add(1, std::memory_order_relaxed); });
                    x ^= x << 13;
                    x ^= x >> 17;
                    x ^= x << 5;
                    if (i % 3 == 0) {
                        std::this_thread::sleep_for(std::chrono::microseconds(x % 400));
                    }
                }
            });
        }
        for (auto& t : ts) {
            t.join();
        }
        CHECK(wait_for([&] { return ran.load() == starters * each; }));
        CHECK(wait_for([] { return gp::num_goroutine() == 1; }));
        gp::detail::set_idle_timeout(old);
        gp::detail::stop_idle_threads();
    }
}

// A zero-size element takes no memory: Go allows make(chan struct{}, 1<<40).
void test_empty_elements() {
    struct none {};
    auto c = gp::make_chan<none>(int64_t(1) << 40);
    CHECK_EQ(gp::cap(c), int64_t(1) << 40);
    for (int i = 0; i < 100; i++) {
        gp::send(c, none{});
    }
    CHECK_EQ(gp::len(c), int64_t(100));
    gp::recv(c);
    CHECK_EQ(gp::len(c), int64_t(99));
    gp::close(c);
    CHECK(std::get<1>(gp::recv2(c))); // the buffered ones are still there
    auto u = gp::make_chan<none>();
    group g;
    g.go([&] { gp::send(u, none{}); });
    gp::recv(u);
    g.wait();
}

// --- type descriptors: three directions, one C++ type ---

void test_descriptors() {
    using C = gp::chan<int64_t>;
    const gp::type_desc* both = gp::td_chan<C>(gp::chan_dir::both, &gp::td_int);
    const gp::type_desc* recv_only = gp::td_chan<C>(gp::chan_dir::recv, &gp::td_int);
    const gp::type_desc* send_only = gp::td_chan<C>(gp::chan_dir::send, &gp::td_int);
    CHECK(both != recv_only && both != send_only && recv_only != send_only);
    CHECK(std::string(both->gp_name) == "chan int");
    CHECK(std::string(recv_only->gp_name) == "<-chan int");
    CHECK(std::string(send_only->gp_name) == "chan<- int");
    CHECK(both == gp::td_chan<C>(gp::chan_dir::both, &gp::td_int)); // interned
    CHECK(gp::td_chan<C>(gp::chan_dir::both, &gp::td_int64) != both);
    CHECK(std::string(gp::td_chan<C>(gp::chan_dir::both, &gp::td_int64)->gp_name) == "chan int64");
    CHECK(both->gp_kind == gp::kind::Chan && both->gp_equal != nullptr && both->gp_hash != nullptr);
    CHECK_EQ(both->gp_size, sizeof(C));
    // chan (<-chan int) needs the parentheses, <-chan <-chan int and chan<- chan int do not.
    using CC = gp::chan<C>;
    CHECK(std::string(gp::td_chan<CC>(gp::chan_dir::both, recv_only)->gp_name) ==
          "chan (<-chan int)");
    CHECK(std::string(gp::td_chan<CC>(gp::chan_dir::recv, recv_only)->gp_name) ==
          "<-chan <-chan int");
    CHECK(std::string(gp::td_chan<CC>(gp::chan_dir::send, both)->gp_name) == "chan<- chan int");
    CHECK(std::string(gp::td_chan<CC>(gp::chan_dir::both, send_only)->gp_name) ==
          "chan chan<- int");
    // The same value, held in an interface of each type, is equal to itself and not to the others.
    C c = gp::make_chan<int64_t>(1);
    gp::any a1 = gp::to_iface<gp::any>(c, both);
    gp::any a2 = gp::to_iface<gp::any>(c, both);
    gp::any a3 = gp::to_iface<gp::any>(c, recv_only);
    CHECK(a1 == a2);
    CHECK(!(a1 == a3));
    CHECK(a1.gp_type() == both && a3.gp_type() == recv_only);
}

// --- the cases that end the process ---

void run_case(const std::string& name) {
    gp::os::no_core_dump();
    if (name == "recv-nil") {
        // A receive from a nil channel blocks the goroutine for ever (and is counted as blocked).
        gp::chan<int64_t> nil;
        gp::go([nil] { gp::recv(nil); });
        if (!blocked_is(1)) {
            say("the receiver on a nil channel is not blocked\n");
            gp::os::exit(1);
        }
        gp::println("blocked"_gs);
    } else if (name == "send-nil") {
        gp::chan<int64_t> nil;
        gp::go([nil] { gp::send(nil, int64_t(1)); });
        if (!blocked_is(1)) {
            gp::os::exit(1);
        }
        gp::println("blocked"_gs);
    } else if (name == "panic-defer-wakes-main") {
        // The deferred call of a goroutine's panic wakes main, which returns: the program must end
        // with the panic's report and exit code 2, not with 0 (`defer close(done)`, `defer
        // wg.Done()`). run_main's exit waits for the panic (settle_panics).
        auto done = gp::make_chan<int64_t>();
        gp::go([done] {
            gp::defer_frame df;
            try {
                df.gp_push_call([done](auto&&...) {
                    gp::close(done);
                    // The deferred call goes on after it woke main (a deferred call that does more
                    // work), so main is back at its end while the panic has not been reported.
                    std::this_thread::sleep_for(50ms);
                });
                gp::panic("simple"_gs);
            } catch (...) {
                df.gp_on_exception();
            }
            df.gp_run();
        });
        gp::recv_drop(done);
        return; // main returns: run_main ends the program
    } else if (name == "panic-defer-blocks") {
        // The deferred call of a goroutine's panic wakes main and then blocks for ever: main must
        // not wait for ever (Go yields 1000 times and exits): exit code 0 after the wait, which is
        // cut to 20 ms here.
        gp::detail::set_settle_timeout(20'000'000);
        auto done = gp::make_chan<int64_t>();
        gp::go([done] {
            gp::defer_frame df;
            try {
                df.gp_push_call([done](auto&&...) {
                    gp::close(done);
                    gp::chan<int64_t> never;
                    gp::recv_drop(never);
                });
                gp::panic("blocked"_gs);
            } catch (...) {
                df.gp_on_exception();
            }
            df.gp_run();
        });
        gp::recv_drop(done);
        gp::println("main returns"_gs);
        return;
    } else if (name == "recover-then-exit") {
        // main recovers a panic and returns: the exit has nothing to wait for (counted by
        // settle_waits, not by the clock).
        recovering([] { gp::panic("recovered"_gs); });
        gp::detail::set_settle_timeout(60'000'000'000LL); // a wait would show as a hang
        gp::detail::settle_panics();
        std::string out = "waits " + std::to_string(gp::detail::settle_waits()) + "\n";
        gp::os::write(gp::os::stdout_fd, out.data(), out.size());
        return;
    } else if (name == "go-nil") {
        gp::go_call(gp::func<void()>());
    } else if (name == "close-nil") {
        gp::chan<int64_t> nil;
        gp::close(nil);
    } else if (name == "close-closed") {
        auto c = gp::make_chan<int64_t>();
        gp::close(c);
        gp::close(c);
    } else if (name == "send-closed") {
        auto c = gp::make_chan<int64_t>(1);
        gp::close(c);
        gp::send(c, int64_t(1));
    } else if (name == "send-closed-goroutine") {
        auto c = gp::make_chan<int64_t>();
        gp::go([c] { gp::send(c, int64_t(1)); });
        if (!blocked_is(1)) {
            gp::os::exit(1);
        }
        gp::close(c);
        gp::recv(gp::chan<int64_t>());
    } else if (name == "recv-no-zero") {
        auto c = gp::make_chan<no_zero>();
        gp::close(c);
        gp::recv(c);
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
    gp::os::exit(0);
}

void run_unit_tests() {
    test_nil_and_make();
    test_buffered_single_thread();
    test_close_single_thread();
    test_element_types();
    test_unbuffered_handoff();
    test_fifo_wakeup();
    test_close_wakes();
    test_pingpong();
    test_mpmc();
    test_per_sender_order();
    test_close_races_send();
    test_blocked_accounting();
    test_panic_count();
    test_waitq_claim();
    test_unblock_by_waker();
    test_select_like_waiter();
    test_go_calls();
    test_thread_reuse();
    test_idle_timeout_race();
    test_empty_elements();
    test_descriptors();
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
