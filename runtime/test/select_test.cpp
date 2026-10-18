// The unit tests of select (gocpp/select.hpp, src/select.cpp), Phase 08 step 8.5.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++,
// at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and with ThreadSanitizer
// (select_test.go), and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or
// prints each failed check and exits 1. With one argument it runs a case that ends the process
// (select_test.go checks the report).
//
// Nothing here depends on timing. Threads synchronize through channels, atomics and
// gp::num_blocked() (a goroutine that waits in a select is counted there before it sleeps, so a
// test can wait until a goroutine is blocked); every stress test is bounded by the number of
// operations, and a wait has a limit of minutes, so a hang is a failure and a slow machine is not.
// The statistical tests have margins of more than ten standard deviations.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include "../src/chan_core.hpp"
#include "threads_ended.hpp"

#include <algorithm>
#include <array>
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
#include <numeric>
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

// counted counts its live instances, so a test can see that select destroys what it holds.
std::atomic<int> live_counted{0};
std::atomic<int64_t> counted_moves{0};
struct counted {
    int v = 0;
    counted() { live_counted++; }
    explicit counted(int x) : v(x) { live_counted++; }
    counted(const counted& o) : v(o.v) { live_counted++; }
    counted(counted&& o) noexcept : v(o.v) {
        o.v = -1;
        live_counted++;
        counted_moves++;
    }
    ~counted() { live_counted--; }
};

// no_zero has no default constructor: a receive from a closed channel needs the zero value.
struct no_zero {
    explicit no_zero(int x) : v(x) {}
    int v;
};

using gp::sel_default;
using gp::sel_drop;
using gp::sel_recv;
using gp::sel_send;
using gp::select;

// queues_empty says that the channel has no waiting goroutine in either queue.
template <typename T>
bool queues_empty(const gp::chan<T>& c) {
    auto* core = c.gp_core();
    std::lock_guard<std::mutex> lk(core->mu);
    return core->recvq.first == nullptr && core->sendq.first == nullptr;
}

// --- the cases that don't wait ---

void test_ready_cases() {
    auto a = gp::make_chan<int64_t>(1);
    auto b = gp::make_chan<gp::string>(1);
    gp::send(a, int64_t(5));
    {
        sel_recv ra{a};
        sel_recv rb{b};
        CHECK_EQ(select(ra, rb), 0);
        CHECK(ra.gp_ok());
        CHECK_EQ(ra.gp_take(), int64_t(5));
    }
    gp::send(b, gp::string::gp_copy("bee"));
    {
        sel_recv ra{a};
        sel_recv rb{b};
        CHECK_EQ(select(ra, rb), 1);
        CHECK(rb.gp_ok());
        CHECK(rb.gp_take().gp_view() == "bee");
    }
    // A send with room in the buffer, and one without.
    {
        sel_send sa{a, int64_t(7)};
        CHECK_EQ(select(sa), 0);
        CHECK_EQ(gp::len(a), int64_t(1));
    }
    {
        sel_send sa{a, int64_t(8)};
        sel_recv rb{b, sel_drop};
        CHECK_EQ(select(sa, rb, sel_default), 2); // full, empty: the default
        CHECK_EQ(gp::len(a), int64_t(1));
    }
    // The default may be anywhere in the list, and its number is its position.
    {
        sel_recv rb{b};
        CHECK_EQ(select(sel_default, rb), 0);
        sel_recv ra{a};
        CHECK_EQ(select(rb, sel_default, ra), 2);
        CHECK_EQ(ra.gp_take(), int64_t(7));
    }
    // Only a default.
    CHECK_EQ(select(sel_default), 0);
    // The value of a send is copied by the case, and the case destroys it, chosen or not.
    {
        auto cc = gp::make_chan<counted>(1);
        auto full = gp::make_chan<counted>(1);
        gp::send(full, counted(1));
        {
            sel_send s1{cc, counted(2)};
            sel_send s2{full, counted(3)};
            CHECK_EQ(select(s1, s2), 0);
        }
        CHECK_EQ(live_counted.load(), 2); // one in each buffer
        {
            sel_send s2{full, counted(3)};
            CHECK_EQ(select(s2, sel_default), 1);
        }
        CHECK_EQ(live_counted.load(), 2);
        {
            sel_recv<counted> r{cc};
            CHECK_EQ(select(r), 0);
            CHECK_EQ(r.gp_take().v, 2);
        }
        {
            sel_recv<counted> r{full}; // received and not taken: destroyed with the case
            CHECK_EQ(select(r), 0);
        }
        CHECK_EQ(live_counted.load(), 0);
        {
            gp::send(full, counted(4));
            sel_recv<counted> r{full, sel_drop}; // dropped
            CHECK_EQ(select(r), 0);
            CHECK_EQ(live_counted.load(), 0);
        }
    }
}

void test_nil_cases() {
    gp::chan<int64_t> nil;
    auto ready = gp::make_chan<int64_t>(1);
    gp::send(ready, int64_t(3));
    for (int i = 0; i < 100; i++) {
        sel_recv rn{nil};
        sel_send sn{nil, int64_t(1)};
        sel_recv rr{ready};
        CHECK_EQ(select(rn, sn, rr), 2);
        gp::send(ready, rr.gp_take());
    }
    {
        sel_recv rn{nil};
        sel_send sn{nil, int64_t(1)};
        CHECK_EQ(select(rn, sn, sel_default), 2);
        CHECK(gp::num_blocked() == 0);
    }
    // A nil channel in a select with a closed channel: the closed one.
    auto closed = gp::make_chan<int64_t>();
    gp::close(closed);
    sel_recv rn{nil};
    sel_recv rc{closed};
    CHECK_EQ(select(rn, rc), 1);
    CHECK(!rc.gp_ok());
    CHECK_EQ(rc.gp_take(), int64_t(0));
}

void test_closed_cases() {
    auto c = gp::make_chan<int64_t>(2);
    gp::send(c, int64_t(1));
    gp::send(c, int64_t(2));
    gp::close(c);
    // The buffer first: ok is true; then the zero value with ok false, for ever.
    for (int i = 0; i < 4; i++) {
        sel_recv r{c};
        CHECK_EQ(select(r), 0);
        CHECK_EQ(r.gp_ok(), i < 2);
        CHECK_EQ(r.gp_take(), i < 2 ? int64_t(i + 1) : int64_t(0));
    }
    // A string: the zero value is the empty string. A dropped value of a type without a zero
    // value is fine (nothing is constructed).
    auto cs = gp::make_chan<gp::string>();
    gp::close(cs);
    sel_recv rs{cs};
    CHECK_EQ(select(rs), 0);
    CHECK(!rs.gp_ok() && rs.gp_take().gp_view() == "");
    auto cn = gp::make_chan<no_zero>();
    gp::close(cn);
    sel_recv<no_zero> rd{cn, sel_drop};
    CHECK_EQ(select(rd), 0);
    CHECK(!rd.gp_ok());
    // A send to a closed channel panics, also when the send case is not the first in the poll order
    // of a select with a ready receive, once the poll reaches it (here: the only ready case).
    auto idle = gp::make_chan<int64_t>();
    CHECK_STR(panic_text([&] {
                  sel_send s{c, int64_t(9)};
                  sel_recv r{idle};
                  select(r, s);
              }),
              "send on closed channel");
    CHECK(queues_empty(c) && queues_empty(idle));
    // ... and the channels' locks are free again.
    CHECK_EQ(gp::len(c), int64_t(0));
    sel_recv r2{idle};
    CHECK_EQ(select(r2, sel_default), 1);
}

// --- the random choice ---

void test_poll_order_uniform() {
    // Three channels that are always ready: each is chosen in a third of 30000 selections, give or
    // take (the sd is 82; the bounds are 7 sd wide... of either side: [0.28, 0.39] of the total).
    const int n = 30000;
    std::array<gp::chan<int64_t>, 3> cs;
    for (auto& c : cs) {
        c = gp::make_chan<int64_t>(1);
        gp::send(c, int64_t(1));
    }
    int count[3] = {0, 0, 0};
    for (int i = 0; i < n; i++) {
        sel_recv r0{cs[0]};
        sel_recv r1{cs[1]};
        sel_recv r2{cs[2]};
        int k = select(r0, r1, r2);
        count[k]++;
        gp::send(cs[k], int64_t(1));
        (k == 0 ? r0 : k == 1 ? r1 : r2).gp_take();
    }
    for (int k = 0; k < 3; k++) {
        CHECK(count[k] > n * 28 / 100 && count[k] < n * 39 / 100);
    }
    CHECK_EQ(count[0] + count[1] + count[2], n);

    // Receives and sends mixed, the default never chosen while a case is ready, and the position of
    // the default in the list does not matter.
    auto full = gp::make_chan<int64_t>(1);
    gp::send(full, int64_t(1));
    auto room = gp::make_chan<int64_t>(1);
    int hits[3] = {0, 0, 0};
    for (int i = 0; i < 20000; i++) {
        sel_recv rf{full};
        sel_send sr{room, int64_t(1)};
        int k = select(sel_default, rf, sr);
        CHECK(k != 0);
        hits[k]++;
        if (k == 1) {
            gp::send(full, rf.gp_take());
        } else {
            gp::recv_drop(room);
        }
    }
    CHECK(hits[1] > 20000 * 45 / 100 && hits[1] < 20000 * 55 / 100);
    CHECK_EQ(hits[1] + hits[2], 20000);
}

void test_poll_order_pairs() {
    // The poll order is a uniform permutation, not only a uniform first element: with n cases of
    // which exactly two are ready, each of the two is chosen about half of the time, whichever two
    // they are and wherever they are in the list (a shuffle that is uniform in its first position
    // and not as a whole fails here).
    for (int n : {3, 4, 6, 8}) {
        std::vector<gp::chan<int64_t>> cs;
        for (int i = 0; i < n; i++) {
            cs.push_back(gp::make_chan<int64_t>(1));
        }
        std::vector<std::unique_ptr<sel_recv<int64_t>>> rs;
        std::vector<gp::detail::sel_entry> es(static_cast<size_t>(n));
        for (int i = 0; i < n; i++) {
            rs.push_back(std::make_unique<sel_recv<int64_t>>(cs[i], sel_drop));
        }
        for (int i = 0; i < n; i++) {
            for (int j = i + 1; j < n; j++) {
                gp::send(cs[i], int64_t(1));
                gp::send(cs[j], int64_t(1));
                const int trials = 4000;
                int first = 0;
                for (int t = 0; t < trials; t++) {
                    for (int k = 0; k < n; k++) {
                        es[k] = rs[k]->gp_entry();
                    }
                    int k = gp::detail::select_run(es.data(), n);
                    CHECK(k == i || k == j);
                    first += k == i;
                    gp::send(cs[k], int64_t(1)); // the channel that was received from is full again
                }
                CHECK(first > trials * 40 / 100 && first < trials * 60 / 100);
                gp::recv_drop(cs[i]);
                gp::recv_drop(cs[j]);
            }
        }
    }
}

void test_poll_order_per_thread() {
    // Eight threads each make 20000 selections between two ready channels: all balanced, and the
    // sequences differ (a generator seeded alike in all threads would give the same bits).
    const int threads = 8;
    const int n = 20000;
    std::vector<std::vector<bool>> seq(threads);
    std::vector<int> ones(threads);
    group g;
    for (int t = 0; t < threads; t++) {
        g.go([&, t] {
            auto a = gp::make_chan<int64_t>(1);
            auto b = gp::make_chan<int64_t>(1);
            gp::send(a, int64_t(1));
            gp::send(b, int64_t(1));
            for (int i = 0; i < n; i++) {
                sel_recv ra{a};
                sel_recv rb{b};
                int k = select(ra, rb);
                if (i < 128) {
                    seq[t].push_back(k == 1);
                }
                ones[t] += k;
                gp::send(k == 0 ? a : b, int64_t(1));
                (k == 0 ? ra : rb).gp_take();
            }
        });
    }
    g.wait();
    for (int t = 0; t < threads; t++) {
        CHECK(ones[t] > n * 45 / 100 && ones[t] < n * 55 / 100);
        for (int u = t + 1; u < threads; u++) {
            CHECK(seq[t] != seq[u]);
        }
    }
}

// --- waiting ---

void test_blocked_woken_by_one() {
    // A goroutine blocked in a select on three channels is woken by exactly one: the send to the
    // first completes it, the sends to the others find their entries claimed (not ready: the
    // channels are unbuffered), and every queue is empty when the goroutine is back.
    for (int round = 0; round < 3; round++) {
        auto a = gp::make_chan<int64_t>();
        auto b = gp::make_chan<int64_t>();
        auto c = gp::make_chan<int64_t>();
        int chosen = -1;
        int64_t got = -1;
        group g;
        g.go([&] {
            sel_recv ra{a};
            sel_recv rb{b};
            sel_recv rc{c};
            chosen = select(ra, rb, rc);
            got = (chosen == 0 ? ra : chosen == 1 ? rb : rc).gp_take();
        });
        CHECK(blocked_is(1));
        CHECK(gp::detail::this_goroutine().reason() == nullptr);
        gp::chan<int64_t> order[3] = {a, b, c};
        int first = round; // which channel completes the select
        int64_t v = 100 + round;
        CHECK(order[first].gp_core()->try_send(&v));
        for (int k = 0; k < 3; k++) {
            int64_t x = 7;
            // The select is taken (claimed) already: no other send is ready, whether or not the
            // goroutine has run yet.
            if (k != first) {
                CHECK(!order[k].gp_core()->try_send(&x));
            }
        }
        g.wait();
        CHECK_EQ(chosen, first);
        CHECK_EQ(got, int64_t(100 + round));
        CHECK(queues_empty(a) && queues_empty(b) && queues_empty(c));
        CHECK(blocked_is(0));
    }
}

void test_blocked_send_cases() {
    // Blocked in a select with sends on two channels (and a receive): a receive from the second
    // takes its value; the others' entries are gone.
    auto a = gp::make_chan<int64_t>();
    auto b = gp::make_chan<int64_t>();
    auto r = gp::make_chan<int64_t>();
    int chosen = -1;
    group g;
    g.go([&] {
        sel_send sa{a, int64_t(1)};
        sel_send sb{b, int64_t(2)};
        sel_recv rr{r, sel_drop};
        chosen = select(sa, sb, rr);
    });
    CHECK(blocked_is(1));
    int64_t got = 0;
    CHECK(b.gp_core()->try_recv(&got) == gp::detail::recv_status::value);
    CHECK_EQ(got, int64_t(2));
    CHECK(a.gp_core()->try_recv(&got) == gp::detail::recv_status::not_ready);
    g.wait();
    CHECK_EQ(chosen, 1);
    CHECK(queues_empty(a) && queues_empty(b) && queues_empty(r));
    // A close wakes a select that receives, with the zero value and ok false; a close wakes a
    // select that sends, and it panics.
    auto d = gp::make_chan<int64_t>();
    auto e = gp::make_chan<int64_t>();
    bool ok = true;
    int64_t val = 5;
    std::string text;
    group h;
    h.go([&] {
        sel_recv rd{d};
        sel_recv re{e};
        int k = select(rd, re);
        ok = k == 0 && rd.gp_ok();
        val = rd.gp_take();
    });
    CHECK(blocked_is(1));
    gp::close(d);
    h.wait();
    CHECK(!ok && val == 0);
    CHECK(queues_empty(d) && queues_empty(e));
    auto f = gp::make_chan<int64_t>();
    auto idle = gp::make_chan<int64_t>();
    group k;
    k.go([&] {
        text = panic_text([&] {
            sel_send sf{f, int64_t(1)};
            sel_recv ri{idle};
            select(sf, ri);
        });
    });
    CHECK(blocked_is(1));
    gp::close(f);
    k.wait();
    CHECK_STR(text, "send on closed channel");
    CHECK(queues_empty(f) && queues_empty(idle));
    CHECK(blocked_is(0));
}

void test_blocked_accounting() {
    // A goroutine in a select counts as blocked ("select") from before it sleeps until the waker
    // has woken it; one wake-up, one decrement, however many channels it waits on.
    auto cs = std::vector<gp::chan<int64_t>>();
    for (int i = 0; i < 5; i++) {
        cs.push_back(gp::make_chan<int64_t>());
    }
    gp::detail::goroutine* gr = nullptr;
    std::atomic<bool> started{false};
    group g;
    g.go([&] {
        gr = &gp::detail::this_goroutine();
        started.store(true);
        sel_recv r0{cs[0]};
        sel_recv r1{cs[1]};
        sel_recv r2{cs[2]};
        sel_recv r3{cs[3]};
        sel_recv r4{cs[4]};
        select(r0, r1, r2, r3, r4);
    });
    CHECK(wait_for([&] { return started.load(); }));
    CHECK(blocked_is(1));
    CHECK(std::string(gr->reason()) == "select");
    // Several wakers race for the same select: only one completes it.
    std::atomic<int> sent{0};
    group w;
    for (int i = 0; i < 5; i++) {
        w.go([&, i] {
            int64_t v = i;
            if (cs[i].gp_core()->try_send(&v)) {
                sent++;
            }
        });
    }
    w.wait();
    g.wait();
    CHECK_EQ(sent.load(), 1);
    CHECK(blocked_is(0));
    CHECK_EQ(gp::num_blocked(), int64_t(0));
    for (auto& c : cs) {
        CHECK(queues_empty(c));
    }
}

void test_unblock_by_waker() {
    // The waker unblocks the select's goroutine (exactly once) before it wakes it, so the goroutine
    // that has been handed its value is not counted as blocked even before it has run again. The
    // test holds the lock of the channel with the lowest address, which the goroutine locks first
    // when it wakes up (in the order of the addresses, so there is no lock-order inversion with the
    // waker, which takes the lock of a channel of higher address while it holds that one), so the
    // goroutine can't have left its select while the count is read.
    for (int round = 0; round < 50; round++) {
        std::array<gp::chan<int64_t>, 3> cs;
        for (auto& c : cs) {
            c = gp::make_chan<int64_t>();
        }
        std::sort(cs.begin(), cs.end(),
                  [](const auto& a, const auto& b) { return a.gp_address() < b.gp_address(); });
        int chosen = -1;
        group g;
        g.go([&] {
            sel_recv r0{cs[0]};
            sel_recv r1{cs[1]};
            sel_recv r2{cs[2]};
            chosen = select(r0, r1, r2);
        });
        CHECK(blocked_is(1));
        int64_t v = 1;
        int target = 1 + round % 2;
        {
            std::lock_guard<std::mutex> hold(cs[0].gp_core()->mu);
            CHECK(cs[target].gp_core()->try_send(&v)); // wakes the goroutine
            CHECK_EQ(gp::num_blocked(), int64_t(0));   // not blocked, though it can't run on
        }
        g.wait();
        CHECK_EQ(chosen, target);
        CHECK_EQ(gp::num_blocked(), int64_t(0));
    }
}

void test_same_channel_twice() {
    // The same channel as two receives (and as a send and a receive): locked once, one entry per
    // case, woken once, the entries removed.
    auto c = gp::make_chan<int64_t>();
    int chosen = -1;
    group g;
    g.go([&] {
        sel_recv r1{c};
        sel_recv r2{c};
        chosen = select(r1, r2);
    });
    CHECK(blocked_is(1));
    int64_t v = 1;
    CHECK(c.gp_core()->try_send(&v));
    g.wait();
    CHECK(chosen == 0 || chosen == 1);
    CHECK(queues_empty(c));
    // A buffered channel with room and a value: send and receive are both ready, and each wins
    // sometimes.
    auto b = gp::make_chan<int64_t>(2);
    gp::send(b, int64_t(1));
    int sends = 0;
    int recvs = 0;
    for (int i = 0; i < 4000; i++) {
        sel_send s{b, int64_t(2)};
        sel_recv r{b};
        if (select(s, r) == 0) {
            sends++;
            gp::recv_drop(b);
        } else {
            recvs++;
            gp::send(b, r.gp_take());
        }
    }
    CHECK(sends > 1500 && recvs > 1500 && sends + recvs == 4000);
    // The same unbuffered channel in a send and a receive of one select, and a goroutine that
    // receives: the send is the only case that can happen.
    auto u = gp::make_chan<int64_t>();
    auto res = gp::make_chan<int64_t>();
    gp::go([u, res] { gp::send(res, gp::recv(u)); });
    CHECK(blocked_is(1));
    {
        sel_recv r{u};
        sel_send s{u, int64_t(33)};
        CHECK_EQ(select(r, s), 1);
    }
    CHECK_EQ(gp::recv(res), int64_t(33));
    CHECK(blocked_is(0));
}

// --- two selects that meet ---

void test_two_selects_meet() {
    // One goroutine sends in a select, another receives in a select, on the same unbuffered
    // channel; each select has a second case that never happens. Each side is sometimes the first
    // to arrive and sleeps, sometimes the second and finds the other's entry in the queue.
    const int n = 20000;
    auto c = gp::make_chan<int64_t>();
    auto never1 = gp::make_chan<int64_t>();
    auto never2 = gp::make_chan<int64_t>();
    int64_t sum = 0;
    group g;
    g.go([&] {
        for (int i = 1; i <= n; i++) {
            sel_send s{c, int64_t(i)};
            sel_send s2{never2, int64_t(0)};
            CHECK_EQ(select(s2, s), 1);
        }
    });
    g.go([&] {
        for (int i = 1; i <= n; i++) {
            sel_recv r{c};
            sel_recv r2{never1};
            CHECK_EQ(select(r, r2), 0);
            sum += r.gp_take();
        }
    });
    g.wait();
    CHECK_EQ(sum, int64_t(n) * (n + 1) / 2);
    CHECK(queues_empty(c) && queues_empty(never1) && queues_empty(never2));
    CHECK(blocked_is(0));
}

void test_lock_order() {
    // Selects that list the same channels in opposite orders, and in other combinations, keep
    // going: the locks are taken in the order of the addresses, so there is no cycle. Producers
    // send on both channels; consumers receive with the cases in opposite orders.
    const int per = 10000;
    auto a = gp::make_chan<int64_t>();
    auto b = gp::make_chan<int64_t>();
    std::atomic<int64_t> total{0};
    group g;
    for (int p = 0; p < 2; p++) {
        g.go([&, p] {
            for (int i = 0; i < per; i++) {
                sel_send sa{a, int64_t(1)};
                sel_send sb{b, int64_t(1)};
                if (p == 0) {
                    select(sa, sb);
                } else {
                    select(sb, sa);
                }
            }
        });
    }
    for (int q = 0; q < 2; q++) {
        g.go([&, q] {
            for (int i = 0; i < per; i++) {
                sel_recv ra{a};
                sel_recv rb{b};
                int k = q == 0 ? select(ra, rb) : select(rb, ra);
                bool from_a = q == 0 ? k == 0 : k == 1;
                total += from_a ? ra.gp_take() : rb.gp_take();
            }
        });
    }
    g.wait();
    CHECK_EQ(total.load(), int64_t(2) * per);
    CHECK(queues_empty(a) && queues_empty(b));
}

void test_claim_races() {
    // 4 sender goroutines and 4 receivers, all in selects over three unbuffered channels (the
    // senders with three send cases, the receivers with three receive cases): every message is
    // received exactly once, whichever entry of a blocked select wins its race.
    const int senders = 4;
    const int per = 5000;
    std::array<gp::chan<int64_t>, 3> cs;
    for (auto& c : cs) {
        c = gp::make_chan<int64_t>();
    }
    std::vector<std::vector<int64_t>> got(senders);
    group g;
    for (int s = 0; s < senders; s++) {
        g.go([&, s] {
            for (int i = 0; i < per; i++) {
                int64_t v = int64_t(s) * per + i;
                sel_send a{cs[0], v};
                sel_send b{cs[1], v};
                sel_send c{cs[2], v};
                select(a, b, c);
            }
        });
        g.go([&, s] {
            for (int i = 0; i < per; i++) {
                sel_recv a{cs[0]};
                sel_recv b{cs[1]};
                sel_recv c{cs[2]};
                int k = select(a, b, c);
                got[s].push_back((k == 0 ? a : k == 1 ? b : c).gp_take());
            }
        });
    }
    g.wait();
    std::vector<int64_t> all;
    for (auto& v : got) {
        all.insert(all.end(), v.begin(), v.end());
    }
    std::sort(all.begin(), all.end());
    bool exact = all.size() == size_t(senders) * per;
    for (size_t i = 0; exact && i < all.size(); i++) {
        exact = all[i] == int64_t(i);
    }
    CHECK(exact);
    for (auto& c : cs) {
        CHECK(queues_empty(c));
    }
    CHECK(blocked_is(0));
}

void test_mixed_waiters() {
    // Selects against plain sends and receives, and selects against selects, on the same channels
    // at the same time: three pairs with as many sends as receives each (so that nothing can be
    // left over), 6 goroutines.
    const int per = 4000;
    auto a = gp::make_chan<int64_t>();
    auto idle_a = gp::make_chan<int64_t>();
    auto b = gp::make_chan<int64_t>(3);
    auto idle_b = gp::make_chan<int64_t>();
    auto c1 = gp::make_chan<int64_t>();
    auto c2 = gp::make_chan<int64_t>(2);
    int64_t sum_a = 0;
    int64_t sum_b = 0;
    int64_t sum_c = 0;
    group g;
    // plain sender, selecting receiver
    g.go([&] {
        for (int i = 1; i <= per; i++) {
            gp::send(a, int64_t(i));
        }
    });
    g.go([&] {
        for (int i = 0; i < per; i++) {
            sel_recv r{a};
            sel_recv n{idle_a};
            select(r, n);
            sum_a += r.gp_take();
        }
    });
    // selecting sender, plain receiver
    g.go([&] {
        for (int i = 1; i <= per; i++) {
            sel_send s{b, int64_t(i)};
            sel_send n{idle_b, int64_t(0)};
            select(s, n);
        }
    });
    g.go([&] {
        for (int i = 0; i < per; i++) {
            sum_b += gp::recv(b);
        }
    });
    // selecting sender, selecting receiver: two channels, one unbuffered and one with a buffer
    g.go([&] {
        for (int i = 1; i <= per; i++) {
            sel_send s1{c1, int64_t(i)};
            sel_send s2{c2, int64_t(i)};
            select(s1, s2);
        }
    });
    g.go([&] {
        for (int i = 0; i < per; i++) {
            sel_recv r1{c1};
            sel_recv r2{c2};
            int k = select(r1, r2);
            sum_c += k == 0 ? r1.gp_take() : r2.gp_take();
        }
    });
    g.wait();
    int64_t want = int64_t(per) * (per + 1) / 2;
    CHECK_EQ(sum_a, want);
    CHECK_EQ(sum_b, want);
    CHECK_EQ(sum_c, want);
    CHECK(queues_empty(a) && queues_empty(b) && queues_empty(c1) && queues_empty(c2));
    CHECK(queues_empty(idle_a) && queues_empty(idle_b));
}

void test_no_lost_wakeups() {
    // Many rounds in which a goroutine blocks in a select and another wakes it a moment later or
    // just before: the permits and the claim never lose a wake-up (a lost one is a hang).
    const int rounds = 20000;
    auto ping = gp::make_chan<int64_t>();
    auto pong = gp::make_chan<int64_t>();
    auto never = gp::make_chan<int64_t>();  // nobody sends to it
    auto never2 = gp::make_chan<int64_t>(); // nobody receives from it
    auto never3 = gp::make_chan<int64_t>(); // nobody sends to it
    group g;
    g.go([&] {
        for (int i = 0; i < rounds; i++) {
            sel_recv r{ping};
            sel_recv n{never3};
            select(r, n);
            sel_send s{pong, r.gp_take() + 1};
            sel_send m{never2, int64_t(0)};
            select(m, s);
        }
    });
    int64_t v = 0;
    for (int i = 0; i < rounds; i++) {
        sel_send s{ping, v};
        sel_recv r{pong};
        sel_recv n{never};
        // both a send and a receive of the same select: the send happens first (the receive has
        // nothing to receive until the other goroutine has the value)
        select(s, n);
        select(r, n);
        v = r.gp_take();
    }
    g.wait();
    CHECK_EQ(v, int64_t(rounds));
}

// --- many cases ---

template <std::size_t>
using int_recv = sel_recv<int64_t>;

template <std::size_t... I>
void many_cases(std::array<gp::chan<int64_t>, sizeof...(I)>& cs, int target, int* chosen,
                std::index_sequence<I...>) {
    std::tuple<int_recv<I>...> t{cs[I]...};
    std::apply([&](auto&... r) { *chosen = select(r...); }, t);
    (void)target;
}

void test_many_cases() {
    // 40 cases (more than the 16 that need no allocation) through the variadic function, ready and
    // blocked.
    constexpr std::size_t n = 40;
    std::array<gp::chan<int64_t>, n> cs;
    for (auto& c : cs) {
        c = gp::make_chan<int64_t>(1);
    }
    gp::send(cs[27], int64_t(1));
    int chosen = -1;
    many_cases(cs, 27, &chosen, std::make_index_sequence<n>());
    CHECK_EQ(chosen, 27);
    int blocked_chosen = -1;
    group g;
    g.go([&] { many_cases(cs, 5, &blocked_chosen, std::make_index_sequence<n>()); });
    CHECK(blocked_is(1));
    gp::send(cs[5], int64_t(1));
    g.wait();
    CHECK_EQ(blocked_chosen, 5);
    for (auto& c : cs) {
        CHECK(queues_empty(c));
    }

    // 1000 cases, through the entries (the number of cases is a run-time value here; the
    // generated Go test has 1000 cases in the source).
    // (ThreadSanitizer's deadlock detector tracks at most 64 locks held by a thread at once and
    // aborts above that, so a TSan build uses 40: select holds the lock of every channel.)
#if defined(__SANITIZE_THREAD__)
    constexpr bool tsan = true;
#elif defined(__has_feature)
#if __has_feature(thread_sanitizer)
    constexpr bool tsan = true;
#else
    constexpr bool tsan = false;
#endif
#else
    constexpr bool tsan = false;
#endif
    const int big = tsan ? 40 : 1000;
    std::vector<gp::chan<int64_t>> bc;
    for (int i = 0; i < big; i++) {
        bc.push_back(gp::make_chan<int64_t>());
    }
    for (int target : {0, 17, big * 3 / 4, big - 1}) {
        std::vector<int64_t> vals(big, -1);
        std::unique_ptr<bool[]> oks_b(new bool[big]);
        std::vector<gp::detail::sel_entry> es(big);
        for (int i = 0; i < big; i++) {
            es[i] = {bc[i].gp_core(), &vals[i], &oks_b[i], gp::detail::sel_kind::recv};
        }
        int got = -1;
        group h;
        h.go([&] { got = gp::detail::select_run(es.data(), big); });
        CHECK(blocked_is(1));
        int64_t v = 1000 + target;
        CHECK(bc[target].gp_core()->try_send(&v));
        h.wait();
        CHECK_EQ(got, target);
        CHECK_EQ(vals[target], int64_t(1000 + target));
        CHECK(oks_b[size_t(target)]);
        for (int i = 0; i < big; i++) {
            CHECK(queues_empty(bc[i]));
        }
    }
    // 1000 buffered channels with one ready: polled, not blocked, and the entries are not queued.
    std::vector<gp::chan<int64_t>> bb;
    for (int i = 0; i < big; i++) {
        bb.push_back(gp::make_chan<int64_t>(1));
    }
    gp::send(bb[size_t(big / 3)], int64_t(5));
    int64_t out = -1;
    bool okb = false;
    std::vector<gp::detail::sel_entry> es(big);
    for (int i = 0; i < big; i++) {
        es[i] = {bb[i].gp_core(), &out, &okb, gp::detail::sel_kind::recv};
    }
    CHECK_EQ(gp::detail::select_run(es.data(), big), big / 3);
    CHECK(okb && out == 5);
}

// --- elements ---

void test_element_types() {
    // A move-only element through the ready path and through the blocked path, in both directions.
    auto c = gp::make_chan<std::unique_ptr<int>>();
    auto d = gp::make_chan<std::unique_ptr<int>>(1);
    {
        sel_send s{d, std::make_unique<int>(11)};
        CHECK_EQ(select(s), 0);
    }
    {
        sel_recv<std::unique_ptr<int>> r{d};
        CHECK_EQ(select(r), 0);
        auto p = r.gp_take();
        CHECK(p && *p == 11);
    }
    std::unique_ptr<int> got;
    group g;
    g.go([&] {
        sel_recv<std::unique_ptr<int>> r{c};
        sel_recv<std::unique_ptr<int>> r2{d};
        int which = select(r, r2);
        got = (which == 0 ? r : r2).gp_take();
    });
    CHECK(blocked_is(1));
    {
        sel_send s{c, std::make_unique<int>(22)};
        sel_send s2{d, std::make_unique<int>(33)}; // the buffer has room: also ready; either
        int k = select(s, s2);
        CHECK(k == 0 || k == 1);
    }
    g.wait();
    CHECK(got != nullptr);
    // Strings and slices are values.
    auto cs = gp::make_chan<gp::string>();
    gp::string out;
    group h;
    h.go([&] {
        sel_recv r{cs};
        select(r);
        out = r.gp_take();
    });
    CHECK(blocked_is(1));
    gp::string text = gp::string::gp_copy("the quick brown fox jumps over the lazy dog");
    {
        sel_send s{cs, text};
        select(s);
    }
    h.wait();
    CHECK(out.gp_view() == text.gp_view());
    // A counted element: nothing leaks, nothing is destroyed twice.
    {
        auto cc = gp::make_chan<counted>();
        int dest_v = 0;
        group k;
        k.go([&] {
            sel_recv<counted> r{cc};
            sel_recv<counted> r2{cc};
            int which = select(r, r2);
            dest_v = (which == 0 ? r : r2).gp_take().v;
        });
        CHECK(blocked_is(1));
        {
            sel_send s{cc, counted(5)};
            select(s);
        }
        k.wait();
        CHECK_EQ(dest_v, 5);
    }
    CHECK_EQ(live_counted.load(), 0);
    // An empty struct.
    struct empty {};
    auto ce = gp::make_chan<empty>(2);
    sel_send<empty> se{ce, empty{}};
    CHECK_EQ(select(se), 0);
    sel_recv<empty> re{ce};
    CHECK_EQ(select(re), 0);
    CHECK(re.gp_ok());
}

void test_directions_and_named() {
    // A named channel type (a newtype over the channel) and a const handle work as cases.
    struct Pipe : gp::newtype<Pipe, gp::chan<int64_t>> {
        using gp::newtype<Pipe, gp::chan<int64_t>>::newtype;
    };
    Pipe p(gp::make_chan<int64_t>(1));
    sel_send s{p, int64_t(4)};
    CHECK_EQ(select(s), 0);
    const Pipe& cp = p;
    sel_recv r{cp};
    CHECK_EQ(select(r), 0);
    CHECK_EQ(r.gp_take(), int64_t(4));
    // A temporary channel (the result of a call) is held by the case.
    auto make = [] { return gp::make_chan<int64_t>(1); };
    sel_send t{make(), int64_t(1)};
    CHECK_EQ(select(t), 0);
}

// --- range over a channel ---

void test_range_chan() {
    // The values in order, the buffer first after a close, then the loop ends.
    auto c = gp::make_chan<int64_t>(4);
    for (int64_t i = 1; i <= 4; i++) {
        gp::send(c, i * i);
    }
    gp::close(c);
    int64_t sum = 0;
    int n = 0;
    for (auto it = gp::range_chan(c); gp::range_next(it);) {
        int64_t v = it.gp_v();
        sum += v;
        n++;
    }
    CHECK_EQ(sum, int64_t(30));
    CHECK_EQ(n, 4);
    // The variant without a place for the value.
    auto d = gp::make_chan<counted>(3);
    for (int i = 0; i < 3; i++) {
        gp::send(d, counted(i));
    }
    gp::close(d);
    n = 0;
    for (auto it = gp::range_chan_drop(d); gp::range_next(it);) {
        n++;
    }
    CHECK_EQ(n, 3);
    CHECK_EQ(live_counted.load(), 0);
    // The drop variant receives without a place for the value: nothing is moved on the way out of
    // the channel (the loop with a variable moves each element into the iterator).
    {
        auto mc = gp::make_chan<counted>(5);
        for (int i = 0; i < 5; i++) {
            gp::send(mc, counted(i));
        }
        int64_t before = counted_moves.load();
        int cnt = 0;
        for (auto it = gp::range_chan_drop(mc); cnt < 5 && gp::range_next(it);) {
            cnt++;
        }
        CHECK_EQ(cnt, 5);
        CHECK_EQ(counted_moves.load() - before, int64_t(0));
        for (int i = 0; i < 5; i++) {
            gp::send(mc, counted(i));
        }
        before = counted_moves.load();
        cnt = 0;
        for (auto it = gp::range_chan(mc); cnt < 5 && gp::range_next(it);) {
            cnt++;
        }
        CHECK_EQ(counted_moves.load() - before, int64_t(5));
    }
    // An element that the body does not take is destroyed by the next step or by the iterator, and
    // a break leaves nothing behind.
    auto e = gp::make_chan<counted>(3);
    for (int i = 0; i < 3; i++) {
        gp::send(e, counted(i));
    }
    gp::close(e);
    n = 0;
    for (auto it = gp::range_chan(e); gp::range_next(it);) {
        if (++n == 2) {
            break; // the second element is in the iterator, untaken
        }
    }
    CHECK_EQ(live_counted.load(), 1); // the third is still in the channel
    e = gp::chan<counted>();
    CHECK_EQ(live_counted.load(), 0);
    // A type without a zero value: the loop ends on the closed channel without a fatal error, also
    // for a receiver that was blocked when the channel was closed.
    auto z = gp::make_chan<no_zero>(1);
    gp::send(z, no_zero(5));
    gp::close(z);
    int total = 0;
    for (auto it = gp::range_chan(z); gp::range_next(it);) {
        total += it.gp_v().v;
    }
    CHECK_EQ(total, 5);
    auto zb = gp::make_chan<no_zero>();
    int seen = 0;
    group g;
    g.go([&] {
        for (auto it = gp::range_chan(zb); gp::range_next(it);) {
            seen += it.gp_v().v;
        }
    });
    gp::send(zb, no_zero(7));
    CHECK(blocked_is(1)); // the loop waits in its next receive
    gp::close(zb);
    g.wait();
    CHECK_EQ(seen, 7);
    // A move-only element, and a receiver that blocks for each value.
    auto m = gp::make_chan<std::unique_ptr<int>>();
    int64_t msum = 0;
    group h;
    h.go([&] {
        for (auto it = gp::range_chan(m); gp::range_next(it);) {
            auto p = it.gp_v();
            msum += *p;
        }
    });
    for (int i = 1; i <= 100; i++) {
        gp::send(m, std::make_unique<int>(i));
    }
    gp::close(m);
    h.wait();
    CHECK_EQ(msum, int64_t(5050));
}

// --- the cases that end the process ---

void run_case(const std::string& name) {
    gp::os::no_core_dump();
    if (name == "select-empty") {
        // select {} blocks the goroutine for ever, and it is counted as blocked.
        gp::go([] { gp::select(); });
        if (!blocked_is(1)) {
            say("the goroutine in select {} is not blocked\n");
            gp::os::exit(1);
        }
        std::string r = gp::detail::this_goroutine().reason() == nullptr ? "main runs" : "?";
        gp::println("blocked"_gs);
    } else if (name == "select-all-nil") {
        // Every case is a nil channel and there is no default: blocked for ever too.
        gp::go([] {
            gp::chan<int64_t> nil;
            sel_recv r{nil};
            sel_send s{nil, int64_t(1)};
            select(r, s);
        });
        if (!blocked_is(1)) {
            gp::os::exit(1);
        }
        gp::println("blocked"_gs);
    } else if (name == "range-nil") {
        // A range over a nil channel blocks for ever, counted as blocked.
        gp::go([] {
            gp::chan<int64_t> nil;
            for (auto it = gp::range_chan(nil); gp::range_next(it);) {
            }
        });
        if (!blocked_is(1)) {
            gp::os::exit(1);
        }
        gp::println("blocked"_gs);
    } else if (name == "select-send-closed") {
        auto c = gp::make_chan<int64_t>(1);
        gp::close(c);
        sel_send s{c, int64_t(1)};
        select(s, sel_default);
    } else if (name == "select-send-closed-goroutine") {
        auto c = gp::make_chan<int64_t>();
        auto idle = gp::make_chan<int64_t>();
        gp::go([c, idle] {
            sel_send s{c, int64_t(1)};
            sel_recv r{idle};
            select(s, r);
        });
        if (!blocked_is(1)) {
            gp::os::exit(1);
        }
        gp::close(c);
        gp::recv(gp::chan<int64_t>());
    } else if (name == "select-recv-no-zero") {
        auto c = gp::make_chan<no_zero>();
        gp::close(c);
        sel_recv<no_zero> r{c};
        select(r);
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
    gp::os::exit(0);
}

void run_unit_tests() {
    test_ready_cases();
    test_nil_cases();
    test_closed_cases();
    test_poll_order_uniform();
    test_poll_order_pairs();
    test_poll_order_per_thread();
    test_blocked_woken_by_one();
    test_blocked_send_cases();
    test_blocked_accounting();
    test_unblock_by_waker();
    test_same_channel_twice();
    test_two_selects_meet();
    test_lock_order();
    test_claim_races();
    test_mixed_waiters();
    test_no_lost_wakeups();
    test_many_cases();
    test_element_types();
    test_directions_and_named();
    test_range_chan();
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
