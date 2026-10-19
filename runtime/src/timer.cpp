// The timers of package time (gocpp/time.hpp, Phase 08 step 8.8, ADR-041): the timer thread and the
// operations of time.Timer, time.Ticker and time.AfterFunc on it, and time.Sleep. Standard C++
// only.
#include <gocpp/nofortify.h>
#include "chan_core.hpp"
#include <gocpp/rt.hpp>

#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <map>
#include <memory>
#include <mutex>
#include <thread>
#include <utility>
#include <vector>

namespace gp::detail {

// timer_rec is the record of one timer. It hangs on the channel object (chan_core::timer) and lives
// as long as the channel does. Fields marked "heap" are guarded by the heap's mutex (the global
// `h` below); the callbacks are set before the timer is armed and never change.
struct timer_rec {
    timer_rec(chan_core* o, bool f) : owner(o), is_func(f) {}
    ~timer_rec();
    timer_rec(const timer_rec&) = delete;
    timer_rec& operator=(const timer_rec&) = delete;

    chan_core* const owner;
    const bool is_func; // AfterFunc: runs fn in a goroutine; else sends on the channel
    uint64_t id = 0;
    gp::func<void(std::shared_ptr<chan_core>, int64_t)> send; // (channel, the deadline it is for)
    gp::func<void()> fn;

    // Held by the timer thread while a send runs, and by stop and reset, so that a send that has
    // started is over before stop returns, and a send that has not started never happens (seq).
    std::mutex fire_mu;

    int64_t when = 0;     // heap: the time that the record is armed for (the map's key), if armed
    int64_t period = 0;   // heap
    bool armed = false;   // heap: the record is in the map
    bool sending = false; // heap: a one-shot timer was taken from the map, and its send has not
                          // started: stop counts that as "the timer was active"
    uint64_t seq = 0;     // heap: changed by stop and reset; a send of an older setting is dropped
    int64_t pend_due = 0; // heap: the deadline that the send is for, while `sending`
    bool firing = false;  // heap: a send has started (fire_mu is held) and is not over

    // The channel, weakly (set before the timer is armed): timer_observe needs it as a shared_ptr.
    std::weak_ptr<chan_core> self;
    // What timer_observe reads without the lock: INT64_MAX when nothing can be due (not armed, no
    // send pending), the deadline when armed, INT64_MIN while a send is pending or running (the
    // observer must wait for it). Written with the
    // heap's mutex held, by sync_hint.
    std::atomic<int64_t> hint{INT64_MAX};
};

namespace {

using clk = std::chrono::steady_clock;

int64_t mono_ns() noexcept {
    return std::chrono::duration_cast<std::chrono::nanoseconds>(clk::now().time_since_epoch())
        .count();
}

// An entry of the map: the channel is held weakly for the channel timers and strongly for an
// AfterFunc timer (the timer runs although nobody holds the Timer).
struct entry {
    std::weak_ptr<chan_core> weak;
    std::shared_ptr<chan_core> strong;
    timer_rec* rec = nullptr;
};

using key = std::pair<int64_t, uint64_t>; // (when, id): the order of firing, ties by arming order

// The heap: the armed timers by deadline, and the thread that serves them. It is never destroyed
// (the thread may run while the process ends: static destructors would run under its feet).
struct heap {
    std::mutex mu;
    std::condition_variable cv;      // the thread waits on it for the earliest deadline
    std::condition_variable gone_cv; // timers_shutdown waits on it for the thread to end
    std::map<key, entry> timers;
    uint64_t next_id = 1;
    bool running = false; // the thread exists
    bool stopping = false;
};

heap& h = *new heap;

// ns of the wait until `when`, at most a day (the clock's arithmetic stays in range).
std::chrono::nanoseconds wait_for(int64_t when, int64_t now) noexcept {
    constexpr int64_t day = int64_t(24) * 3600 * 1000000000;
    int64_t left = when - now;
    if (left > day || left < 0) {
        left = left < 0 ? 0 : day;
    }
    return std::chrono::nanoseconds(left);
}

void thread_main();

// sync_hint publishes the state of rec for timer_observe (h.mu held).
void sync_hint(timer_rec* rec) {
    rec->hint.store(rec->sending || rec->firing ? INT64_MIN
                    : rec->armed                ? rec->when
                                                : INT64_MAX,
                    std::memory_order_relaxed);
}

// ensure_thread starts the timer thread if it does not run (called with h.mu held).
void ensure_thread() {
    if (!h.running) {
        h.running = true;
        std::thread(thread_main).detach();
    }
}

// arm puts rec into the map for `when` (h.mu held; rec is not armed). strong is the channel for an
// AfterFunc timer, null for a channel timer, which the map holds weakly.
void arm(timer_rec* rec, const std::shared_ptr<chan_core>& c, int64_t when, int64_t period) {
    rec->when = when;
    rec->period = period;
    rec->armed = true;
    entry e;
    e.rec = rec;
    if (rec->is_func) {
        e.strong = c;
    } else {
        e.weak = c;
    }
    bool first = h.timers.empty() || key{when, rec->id} < h.timers.begin()->first;
    h.timers.emplace(key{when, rec->id}, std::move(e));
    sync_hint(rec);
    ensure_thread();
    if (first) {
        h.cv.notify_one();
    }
}

// disarm takes rec out of the map if it is there (h.mu held). The entry is returned, so that the
// caller can drop it (the channel it holds strongly, maybe the last reference) after the unlock.
entry disarm(timer_rec* rec) {
    entry e;
    if (rec->armed) {
        auto it = h.timers.find(key{rec->when, rec->id});
        if (it != h.timers.end()) {
            e = std::move(it->second);
            h.timers.erase(it);
        }
        rec->armed = false;
        rec->when = 0;
        sync_hint(rec);
    }
    return e;
}

// end_send says that the send of rec is over (fire_mu is still held).
void end_send(timer_rec* rec) {
    std::lock_guard<std::mutex> lk(h.mu);
    rec->firing = false;
    sync_hint(rec);
}

// fire runs a timer that the thread took from the map (no lock held). c is the channel, held by the
// thread: the record lives as long as it does. (The thread takes the strong reference while it
// holds the heap's lock, because a record that is being destroyed waits for that lock: after the
// unlock, a channel that is held only weakly may be gone, record and all.)
void fire(timer_rec* rec, const std::shared_ptr<chan_core>& c, uint64_t seq) {
    if (rec->is_func) {
        // The goroutine gets its own copy of the function.
        gp::func<void()> f = rec->fn;
        gp::go([f = std::move(f)] { f(); });
        return;
    }
    std::lock_guard<std::mutex> guard(rec->fire_mu);
    int64_t due = 0;
    {
        std::lock_guard<std::mutex> lk(h.mu);
        if (rec->seq != seq || !rec->sending) {
            return; // stop or reset came first, or timer_observe has sent it: nothing to do
        }
        due = rec->pend_due;
        rec->sending = false;
        rec->firing = true;
        sync_hint(rec);
    }
    rec->send(c, due);
    end_send(rec);
}

// begin_fire is what happens to a timer that is due, wherever it is noticed (h.mu held, e taken
// out of the map): a ticker is armed for its next tick, which is one period after the one that is
// due now, or the first one after the present when we are late (ticks are dropped, not queued:
// Go's unlockAndRun); a one-shot timer is not armed any more. The send of a channel timer is
// pending (`sending`) until it has been done, by the timer thread (fire) or by timer_observe,
// whichever takes fire_mu first. Returns how late the send is.
void begin_fire(entry& e, int64_t due, int64_t now) {
    timer_rec* rec = e.rec;
    int64_t delta = now - due;
    if (rec->period > 0) {
        int64_t next = due;
        int64_t skip = 1 + delta / rec->period;
        int64_t add = 0;
        if (__builtin_mul_overflow(rec->period, skip, &add) ||
            __builtin_add_overflow(due, add, &next)) {
            next = INT64_MAX;
        }
        rec->when = next;
        h.timers.emplace(key{next, rec->id}, entry{e.weak, nullptr, rec});
    } else {
        rec->armed = false;
        rec->when = 0;
    }
    rec->sending = !rec->is_func;
    rec->pend_due = due;
    sync_hint(rec);
}

void thread_main() {
    std::unique_lock<std::mutex> lk(h.mu);
    while (!h.stopping) {
        if (h.timers.empty()) {
            h.cv.wait(lk);
            continue;
        }
        int64_t now = mono_ns();
        auto it = h.timers.begin();
        int64_t due = it->first.first;
        if (due > now) {
            h.cv.wait_for(lk, wait_for(due, now));
            continue;
        }
        entry e = std::move(it->second);
        h.timers.erase(it);
        timer_rec* rec = e.rec;
        // The channel, strongly, while the lock is held (see fire). A channel that is being
        // destroyed is skipped: its record leaves the map by itself (its key is not there any more:
        // no-op).
        std::shared_ptr<chan_core> c = rec->is_func ? std::move(e.strong) : e.weak.lock();
        if (c == nullptr) {
            continue;
        }
        uint64_t seq = rec->seq;
        begin_fire(e, due, now);
        lk.unlock();
        fire(rec, c, seq);
        c.reset(); // the last reference of a timer that nobody holds goes here, with the lock free
        lk.lock();
    }
    h.running = false;
    h.gone_cv.notify_all();
}

} // namespace

timer_rec::~timer_rec() {
    // The channel is gone: leave the map. (The thread holds the channel while it uses a record,
    // so it is not inside this record. Called without h.mu: no code drops a channel with it held.)
    std::lock_guard<std::mutex> lk(h.mu);
    if (armed) {
        h.timers.erase(key{when, id});
    }
}

void start_chan_timer(const std::shared_ptr<chan_core>& c, int64_t when, int64_t period,
                      gp::func<void(std::shared_ptr<chan_core>, int64_t)> send) {
    auto rec = std::make_shared<timer_rec>(c.get(), false);
    rec->send = std::move(send);
    {
        std::lock_guard<std::mutex> lk(h.mu);
        rec->id = h.next_id++;
        rec->self = c;
        c->timer = rec;
        arm(rec.get(), c, when, period);
    }
}

void start_func_timer(const std::shared_ptr<chan_core>& c, int64_t when, gp::func<void()> f) {
    auto rec = std::make_shared<timer_rec>(c.get(), true);
    rec->fn = std::move(f);
    {
        std::lock_guard<std::mutex> lk(h.mu);
        rec->id = h.next_id++;
        c->timer = rec;
        arm(rec.get(), c, when, 0);
    }
}

namespace {

// retime is the common part of stop and reset: it takes the timer out of the map (and arms it
// again for `when` if rearm), and says whether it was active. `entry` objects are dropped after
// the unlock.
bool retime(const std::shared_ptr<chan_core>& c, chan_core* core, bool rearm, int64_t when,
            int64_t period) {
    timer_rec* rec = core->timer.get();
    if (rec == nullptr) {
        return false;
    }
    // A channel timer: the lock that a running send holds. (AfterFunc has no send to wait for: a
    // goroutine that was started is not stopped by Stop, as in Go.)
    std::unique_lock<std::mutex> fire;
    if (!rec->is_func) {
        fire = std::unique_lock<std::mutex>(rec->fire_mu);
    }
    bool pending = false;
    entry old;
    {
        std::lock_guard<std::mutex> lk(h.mu);
        pending = rec->armed || rec->sending;
        old = disarm(rec);
        rec->sending = false; // a send that was about to start is dropped (seq)
        rec->seq++;
        sync_hint(rec);
        if (rearm) {
            arm(rec, c, when, period);
        }
    }
    old = entry{};
    if (!rec->is_func) {
        // The value that an earlier setting sent and nobody has received: a stale value must
        // never be received after stop or reset, and its being there means the timer had not
        // been used up. (The thread cannot send before `fire` is unlocked.)
        if (core->drain(nullptr) == recv_status::value) {
            pending = true;
        }
    }
    return pending;
}

} // namespace

bool stop_timer(chan_core* core) {
    if (core == nullptr) {
        return false;
    }
    return retime(nullptr, core, false, 0, 0);
}

bool reset_timer(const std::shared_ptr<chan_core>& c, int64_t when, int64_t period) {
    if (c == nullptr) {
        return false;
    }
    return retime(c, c.get(), true, when, period);
}

void timer_observe(chan_core* core) {
    timer_rec* rec = core->timer.get();
    if (rec->is_func) {
        return;
    }
    int64_t hint = rec->hint.load(std::memory_order_relaxed);
    if (hint == INT64_MAX) {
        return; // the common case: not armed
    }
    int64_t now = mono_ns();
    if (hint > now) {
        return; // armed for later
    }
    std::shared_ptr<chan_core> c = rec->self.lock();
    if (c == nullptr) {
        return;
    }
    // The same lock as the timer thread's send: this one is over before the receive looks.
    std::lock_guard<std::mutex> guard(rec->fire_mu);
    int64_t due = 0;
    {
        std::lock_guard<std::mutex> lk(h.mu);
        if (rec->sending) {
            // The thread took the timer and has not sent yet: the send is ours.
            due = rec->pend_due;
        } else if (rec->armed && rec->when <= now) {
            auto it = h.timers.find(key{rec->when, rec->id});
            if (it == h.timers.end()) {
                return;
            }
            entry e = std::move(it->second);
            due = it->first.first;
            h.timers.erase(it);
            begin_fire(e, due, now);
        } else {
            return;
        }
        rec->sending = false;
        rec->firing = true;
        sync_hint(rec);
    }
    rec->send(c, due);
    end_send(rec);
}

bool timer_try_send(chan_core* c, void* v) {
    return c->try_send(v);
}

std::size_t timers_armed() noexcept {
    std::lock_guard<std::mutex> lk(h.mu);
    return h.timers.size();
}

bool timers_can_wake() {
    // The channels are held (strongly) in `keep` and dropped after the unlock: dropping the last
    // reference of a channel runs its record's destructor, which takes the heap's lock.
    std::vector<std::shared_ptr<chan_core>> keep;
    {
        std::lock_guard<std::mutex> lk(h.mu);
        for (auto& [k, e] : h.timers) {
            if (e.rec->is_func) {
                return true; // AfterFunc: starts a goroutine that may wake others
            }
            if (auto c = e.weak.lock()) {
                keep.push_back(std::move(c));
            }
        }
    }
    for (auto& c : keep) {
        std::lock_guard<std::mutex> lk(c->mu);
        if (c->recvq.first != nullptr) {
            return true; // a goroutine waits for the tick
        }
    }
    return false;
}

void timers_shutdown() {
    std::unique_lock<std::mutex> lk(h.mu);
    if (!h.running) {
        return;
    }
    h.stopping = true;
    h.cv.notify_all();
    h.gone_cv.wait(lk, [] { return !h.running; });
    h.stopping = false;
}

} // namespace gp::detail

namespace gp::time {

void sleep(int64_t ns) {
    if (ns <= 0) {
        return;
    }
    int64_t now = detail::mono_ns();
    int64_t deadline = ns > INT64_MAX - now ? INT64_MAX : now + ns;
    // The goroutine waits on its own waiter, with the deadline: the kernel wakes the thread, no
    // timer thread is involved, and the goroutine is not counted as blocked. A permit that is older
    // than the call (an early unpark) wakes it too: it waits on, until the deadline.
    detail::goroutine& g = detail::this_goroutine();
    for (;;) {
        now = detail::mono_ns();
        if (now >= deadline) {
            return;
        }
        g.park.gp_park_for(std::chrono::nanoseconds(deadline - now));
    }
}

} // namespace gp::time
