// The coroutines of package iter (gocpp/iter.hpp, step 7.10). Standard C++ and the goroutine layer
// only.
//
// The two sides of a coroutine (the goroutine that drives it, "peer", and the coroutine's own
// goroutine, "co") hand control to each other. The race detector (-race) needs no annotation: the
// hand-off word and the waiter's mutex are the edges that it sees, as long as the end hook of a
// goroutine runs outside the scope that hides the goroutine counts from it (run_goroutine). A side
// that waits keeps a `party` on its own stack: its goroutine record and a hand-off word that the
// other side sets (see `party`: a spin, then the goroutine's park waiter). The coroutine object is
// never touched after the last hand-off, so the side that finishes can free it before it wakes the
// other.
//
// Accounting (goroutine.hpp, block_scope): a side that waits is a blocked goroutine; the side that
// hands over unblocks the other BEFORE it blocks itself, so the detector never sees both sides
// blocked while one has been handed control.
#include <gocpp/nofortify.h>
#include <gocpp/goroutine.hpp>
#include <gocpp/iter.hpp>
#include <gocpp/os.hpp>
#include <gocpp/panic.hpp>

#include <atomic>
#include <cstdint>
#include <memory>
#include <utility>

namespace gp::iter {
namespace {

using detail::goroutine;

// A side that waits. It lives on the stack of the waiting goroutine, which is in wait() until the
// other side has notify()'d it. st is the hand-off word: the waiter spins on it for a moment
// (waiting), then parks (parked); the other side sets notified, with one compare-and-swap when the
// waiter still spins (no lock, no system call: the switch costs the transfer of one cache line),
// and under the waiter's lock when it is parked (so the waiter can't go on, and end, before the
// notifier is done with the waiter).
enum : int { waiting = 0, notified = 1, parked = 2 };

struct party {
    goroutine* g = nullptr;
    std::atomic<int> st{waiting};
};

struct coro {
    gp::func<void(std::uint64_t)> fn;
    goroutine* co_g = nullptr; // the coroutine's goroutine (set by its thread when it starts)
    party* co = nullptr;       // the coroutine side while it waits
    party* peer = nullptr;     // the driving side while it waits
};

void cpu_relax() noexcept {
#if defined(__x86_64__) || defined(__i386__)
    __builtin_ia32_pause();
#elif defined(__aarch64__)
    __asm__ __volatile__("yield");
#endif
}

// How long a side spins before it parks: the other side usually answers within a microsecond or
// two (a generator's next value), and a futex round trip costs ten. A side that waits longer parks.
// Nothing spins on a machine with one CPU.
constexpr int spin_rounds = 400;

void wait(goroutine& g, party& me) {
    static const bool multi = os::cpu_count() > 1;
    if (multi) {
        for (int i = 0; i < spin_rounds; i++) {
            if (me.st.load(std::memory_order_acquire) == notified) {
                return;
            }
            cpu_relax();
        }
    }
    int s = waiting;
    if (!me.st.compare_exchange_strong(s, parked, std::memory_order_acq_rel)) {
        return; // notified meanwhile
    }
    g.park.gp_wait([&me] { return me.st.load(std::memory_order_acquire) == notified; });
}

void notify(party& other) {
    goroutine* og = other.g;
    int s = waiting;
    if (other.st.compare_exchange_strong(s, notified, std::memory_order_acq_rel)) {
        return; // it spins or has not looked yet: it sees notified; `other` is not touched again
    }
    og->park.gp_unpark_with([&other] { other.st.store(notified, std::memory_order_release); });
}

// hand_over makes `other` run: it is unblocked in the deadlock count and notified; when `self` is
// true the calling goroutine becomes a blocked one first (the other is unblocked BEFORE this one is
// blocked, so the detector never sees both asleep while one has been handed control).
void hand_over(party& other, bool self) {
    detail::unblock(*other.g);
    if (self) {
        detail::block_enter("coroutine");
    }
    notify(other);
}

// The end of the coroutine's goroutine: its record is gone and it no longer counts (run_goroutine),
// and now the side that waits for it goes on (the NumGoroutine that it reads is already lower). The
// side was unblocked in the deadlock count before the goroutine ended (run, below): a goroutine
// that ends asks the detector whether everything is asleep, and the waiting side must not look
// asleep then.
void finish(void* p) noexcept {
    notify(*static_cast<party*>(p));
}

struct start_task final : detail::task {
    coro* c;
    party* creator; // waits in newcoro until the coroutine has registered itself
    start_task(coro* c_, party* creator_) : c(c_), creator(creator_) {}
    void run() override {
        goroutine& me = detail::this_goroutine();
        party mine;
        mine.g = &me;
        c->co_g = &me;
        c->co = &mine;
        // Born blocked, as Go's newcoro goroutine is (waitReasonCoroutine): the first coroswitch
        // unblocks it. Before the creator is told that we are here, so that its unblock finds us.
        detail::block_enter("coroutine");
        notify(*creator); // creator is not used again: it may be gone as soon as it sees notified
        wait(me, mine);
        detail::block_exit();
        fn_ = std::move(c->fn);
        try {
            fn_(reinterpret_cast<std::uintptr_t>(c));
        } catch (const gp::goexit_signal&) {
            // runtime.Goexit in the iterator: the deferred calls have run (Pull's records it), the
            // coroutine ends like a goroutine does.
        }
        // The driving side is parked in coroswitch (its party is on its stack). The coroutine
        // object is ours: free it before the last hand-over.
        party* peer = c->peer;
        delete c;
        detail::unblock(*peer->g); // it is about to run again: not asleep
        me.end_hook = &finish;
        me.end_arg = peer;
    }
    gp::func<void(std::uint64_t)> fn_;
};

} // namespace

std::uint64_t newcoro(const gp::func<void(std::uint64_t)>& f) {
    goroutine& me = detail::this_goroutine();
    auto* c = new coro;
    c->fn = f;
    party creator;
    creator.g = &me;
    detail::go_start(std::make_unique<start_task>(c, &creator), 0, &detail::creator_pull);
    // The coroutine's goroutine counts from go_start; wait until it has registered itself and
    // parked (a thread that is idle in the pool, or a new one).
    wait(me, creator);
    return reinterpret_cast<std::uintptr_t>(c);
}

void coroswitch(std::uint64_t h) {
    auto* c = reinterpret_cast<coro*>(static_cast<std::uintptr_t>(h));
    goroutine& me = detail::this_goroutine();
    party mine;
    mine.g = &me;
    party* other;
    if (&me == c->co_g) {
        other = c->peer; // the coroutine goes back to whoever switched in last
        c->co = &mine;
    } else {
        other = c->co;
        c->peer = &mine;
    }
    hand_over(*other, true);
    wait(me, mine);
    detail::block_exit();
}

} // namespace gp::iter
