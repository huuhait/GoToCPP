// The slow paths of package sync (gocpp/sync.hpp): the semaphore table of Go's runtime/sema.go, the
// Mutex's lockSlow and unlockSlow (internal/sync/mutex.go), the notify list of sync.Cond, and the
// panics and fatal errors of the package. Standard C++ only.
//
// The semaphore table. A goroutine that has to wait on a semaphore word (a Mutex's sema, a
// WaitGroup's sema, ...) leaves an entry (a gp::detail::sudog on its own stack, as a channel
// does) in the queue of the bucket that the word's ADDRESS hashes to, and parks. A bucket is a
// std::mutex and a first-in-first-out list; the entries of several words can share one list, and
// release walks it to the first entry of its own address (the lists are short: a goroutine is a
// thread). The count of waiters of a bucket (nwait) is read without the lock by release, so the
// common case, a release with nobody waiting, takes no lock (Go's trick, with the same argument:
// acquire raises nwait before it looks at the semaphore again, and release raises the semaphore
// before it looks at nwait, all sequentially consistent, so one of them sees the other).
#include <gocpp/nofortify.h>
#include "chan_core.hpp"
#include <gocpp/rt.hpp>
#include <gocpp/sync.hpp>

#include <atomic>
#include <chrono>
#include <cstdint>
#include <mutex>
#include <gocpp/os.hpp>

namespace gp::sync::detail {

namespace {

using gp::detail::sudog;
using gp::detail::waitq;
using gp::detail::wake_cell;

// An entry of the table: a sudog (the waiter, its cell, and the key in elem) and the ticket of
// a notify list or of a handed-off semaphore.
struct entry : sudog {
    uint32_t ticket = 0;
};

constexpr std::size_t bucket_count = 251;

struct alignas(64) bucket {
    std::mutex mu;
    waitq q;            // guarded by mu
    uint32_t nwait = 0; // atomic: through at()
};

bucket buckets[bucket_count];

bucket& bucket_of(const void* addr) noexcept {
    return buckets[(reinterpret_cast<uintptr_t>(addr) >> 3) % bucket_count];
}

// cansemacquire takes a unit of the semaphore if there is one.
bool can_acquire(uint32_t* addr) noexcept {
    auto s = at(*addr);
    uint32_t v = s.load();
    while (v != 0) {
        if (s.compare_exchange_weak(v, v - 1)) {
            return true;
        }
    }
    return false;
}

// enqueue puts e in the bucket's list (mu held): at the front, or at the end.
void enqueue(bucket& b, entry* e, bool lifo) noexcept {
    if (!lifo) {
        b.q.enqueue(e);
        return;
    }
    e->prev = nullptr;
    e->next = b.q.first;
    if (b.q.first != nullptr) {
        b.q.first->prev = e;
    } else {
        b.q.last = e;
    }
    b.q.first = e;
}

// find returns the first entry for key (mu held), and for a notify list the one with the ticket.
entry* find(bucket& b, const void* key, bool by_ticket, uint32_t ticket) noexcept {
    for (sudog* s = b.q.first; s != nullptr; s = s->next) {
        auto* e = static_cast<entry*>(s);
        if (e->elem == key && (!by_ticket || e->ticket == ticket)) {
            return e;
        }
    }
    return nullptr;
}

// park queues the entry and parks the goroutine until it is woken. The block_scope is entered
// under the bucket's lock, before the entry is visible to wakers (the contract of unblock).
void park(std::unique_lock<std::mutex>& lk, bucket& b, entry& e, const char* reason, bool lifo) {
    gp::detail::block_scope blocked(reason);
    wake_cell cell;
    e.g = &gp::detail::this_goroutine();
    e.w = &e.g->park;
    e.cell = &cell;
    enqueue(b, &e, lifo);
    lk.unlock();
    gp::detail::park_cell(cell);
}

int64_t nanotime() noexcept {
    return std::chrono::duration_cast<std::chrono::nanoseconds>(
               std::chrono::steady_clock::now().time_since_epoch())
        .count();
}

// can_spin: active spinning makes sense on a machine with more than one CPU, a few times
// (runtime_canSpin: the Go runtime also asks whether another P is running; a thread per goroutine
// can't know, and a spin of 4 x 30 pauses is about a microsecond).
bool can_spin(int iter) noexcept {
    static const bool multi = gp::os::cpu_count() > 1;
    return iter < 4 && multi;
}

void do_spin() noexcept {
    for (int i = 0; i < 30; i++) {
#if defined(__x86_64__) || defined(__i386__)
        __builtin_ia32_pause();
#elif defined(__aarch64__)
        asm volatile("yield" ::: "memory");
#endif
    }
}

constexpr int32_t mutex_woken = 2;
constexpr int32_t mutex_starving = 4;
constexpr int mutex_waiter_shift = 3;

// A waiter that waited longer than this when it was woken puts the mutex in starvation mode: Go's
// threshold is 1 ms, plus here 100 us for each goroutine that waits too. A hand-off in starvation
// mode costs a futex wake-up and a context switch (20 to 50 us on an idle machine, more on a loaded
// one), where Go's costs about 200 ns, so with 100 threads on one mutex a wait of 1 ms is the
// normal order of the queue, not a sign of unfairness; with Go's constant the mutex went into
// starvation mode at once and stayed in it (every Unlock a hand-off, 13 s for 2M Lock/Unlock pairs
// of 100 threads, against 0.3 s without it). A wait that the length of the queue explains is not
// starvation; one that it does not explain is.
constexpr int64_t starvation_base_ns = 1000000;
constexpr int64_t starvation_per_waiter_ns = 100000;

template <typename R>
bool cas(R& s, int32_t old, int32_t now) noexcept {
    return s.compare_exchange_strong(old, now);
}

} // namespace

void sema_acquire(uint32_t* addr, const char* reason, bool lifo) {
    // Easy case.
    if (can_acquire(addr)) {
        return;
    }
    bucket& b = bucket_of(addr);
    entry e;
    e.elem = addr;
    for (;;) {
        std::unique_lock<std::mutex> lk(b.mu);
        // Add ourselves to nwait to disable "easy case" in release.
        at(b.nwait).fetch_add(1);
        // Check the semaphore again, so that a release that came between the first check and the
        // nwait increment is not missed.
        if (can_acquire(addr)) {
            at(b.nwait).fetch_sub(1);
            return;
        }
        // Any release after this point sees nwait > 0 and will wake us.
        e.ticket = 0;
        park(lk, b, e, reason, lifo);
        if (e.ticket != 0 || can_acquire(addr)) {
            // A handed-off unit (ticket), or one we can take now.
            return;
        }
    }
}

void sema_release(uint32_t* addr, bool handoff) {
    bucket& b = bucket_of(addr);
    at(*addr).fetch_add(1);
    // Easy case: no waiters? This check must happen after the add, to avoid a missed wake-up.
    if (at(b.nwait).load() == 0) {
        return;
    }
    std::unique_lock<std::mutex> lk(b.mu);
    if (at(b.nwait).load() == 0) {
        return;
    }
    entry* e = find(b, addr, false, 0);
    if (e == nullptr) {
        return; // the waiters are for another word of the bucket
    }
    b.q.remove(e);
    at(b.nwait).fetch_sub(1);
    lk.unlock();
    // The entry is off the queue and its goroutine is parked: nobody else will wake it, so it may
    // be written until the wake.
    if (handoff && can_acquire(addr)) {
        e->ticket = 1;
    }
    gp::detail::wake(e);
}

void mutex_lock_slow(int32_t& st, uint32_t& sema) {
    auto state = at(st);
    int64_t wait_start = 0;
    bool starving = false;
    bool awoke = false;
    int iter = 0;
    int32_t old = state.load();
    for (;;) {
        // Don't spin in starvation mode: ownership is handed off to waiters, so we won't be able
        // to acquire the mutex anyway.
        if ((old & (mutex_locked | mutex_starving)) == mutex_locked && can_spin(iter)) {
            // Active spinning makes sense. Try to set mutexWoken to inform Unlock not to wake
            // other blocked goroutines.
            if (!awoke && (old & mutex_woken) == 0 && (old >> mutex_waiter_shift) != 0 &&
                cas(state, old, old | mutex_woken)) {
                awoke = true;
            }
            do_spin();
            iter++;
            old = state.load();
            continue;
        }
        int32_t now = old;
        // Don't try to acquire a starving mutex, new arriving goroutines must queue.
        if ((old & mutex_starving) == 0) {
            now |= mutex_locked;
        }
        if ((old & (mutex_locked | mutex_starving)) != 0) {
            now += int32_t(1) << mutex_waiter_shift;
        }
        // The current goroutine switches the mutex to starvation mode. But if the mutex is
        // unlocked, don't do the switch: Unlock expects a starving mutex to have waiters.
        if (starving && (old & mutex_locked) != 0) {
            now |= mutex_starving;
        }
        if (awoke) {
            // The goroutine has been woken from sleep, so we need to reset the flag in either case.
            if ((now & mutex_woken) == 0) {
                gp::detail::fatal("sync: inconsistent mutex state");
            }
            now &= ~mutex_woken;
        }
        if (cas(state, old, now)) {
            if ((old & (mutex_locked | mutex_starving)) == 0) {
                break; // locked the mutex with CAS
            }
            // If we were already waiting before, queue at the front of the queue.
            bool queue_lifo = wait_start != 0;
            if (wait_start == 0) {
                wait_start = nanotime();
            }
            sema_acquire(&sema, "sync.Mutex.Lock", queue_lifo);
            old = state.load();
            starving = starving || nanotime() - wait_start >
                                       starvation_base_ns + starvation_per_waiter_ns *
                                                                int64_t(old >> mutex_waiter_shift);
            if ((old & mutex_starving) != 0) {
                // The mutex is in starvation mode and was handed to us: it is in a somewhat
                // inconsistent state (mutexLocked is not set, and we still count as a waiter).
                if ((old & (mutex_locked | mutex_woken)) != 0 || (old >> mutex_waiter_shift) == 0) {
                    gp::detail::fatal("sync: inconsistent mutex state");
                }
                int32_t delta = mutex_locked - (int32_t(1) << mutex_waiter_shift);
                if (!starving || (old >> mutex_waiter_shift) == 1) {
                    // Exit starvation mode. Critical to do it here and consider the wait time:
                    // starvation mode is so inefficient that two goroutines can go lock-step
                    // for ever once they have switched the mutex to it.
                    delta -= mutex_starving;
                }
                state.fetch_add(delta);
                break;
            }
            awoke = true;
            iter = 0;
        } else {
            old = state.load();
        }
    }
}

void mutex_unlock_slow(int32_t& st, uint32_t& sema, int32_t now) {
    auto state = at(st);
    if (((now + mutex_locked) & mutex_locked) == 0) {
        fatal_unlock_mutex();
    }
    if ((now & mutex_starving) == 0) {
        int32_t old = now;
        for (;;) {
            // If there are no waiters, or a goroutine has already been woken or grabbed the lock,
            // there is no need to wake anyone. In starvation mode the ownership is handed off
            // from the unlocking goroutine to the next waiter; we are not part of that chain,
            // since we did not see mutexStarving when we unlocked the mutex.
            if ((old >> mutex_waiter_shift) == 0 ||
                (old & (mutex_locked | mutex_woken | mutex_starving)) != 0) {
                return;
            }
            // Grab the right to wake someone.
            now = (old - (int32_t(1) << mutex_waiter_shift)) | mutex_woken;
            if (cas(state, old, now)) {
                sema_release(&sema, false);
                return;
            }
            old = state.load();
        }
    } else {
        // Starving mode: hand the ownership to the next waiter. mutexLocked is not set: the
        // waiter sets it after the wake-up. The mutex counts as locked while mutexStarving is
        // set, so new arrivals don't take it.
        sema_release(&sema, true);
    }
}

void notify_wait(uint32_t& wait, uint32_t& notify, uint32_t ticket) {
    bucket& b = bucket_of(&wait);
    std::unique_lock<std::mutex> lk(b.mu);
    // Return right away if this ticket has already been notified.
    if (static_cast<int32_t>(ticket - at(notify).load()) < 0) {
        return;
    }
    entry e;
    e.elem = &wait;
    e.ticket = ticket;
    park(lk, b, e, "sync.Cond.Wait", false);
}

void notify_all(uint32_t& wait, uint32_t& notify) {
    // Fast-path: if there are no waiters, there is nothing to do.
    if (at(wait).load() == at(notify).load()) {
        return;
    }
    bucket& b = bucket_of(&wait);
    gp::detail::sudog* head = nullptr;
    gp::detail::sudog* tail = nullptr;
    {
        std::unique_lock<std::mutex> lk(b.mu);
        // Take all the waiters of this list off the bucket (in order), and mark all the tickets
        // so far as notified.
        for (sudog* s = b.q.first; s != nullptr;) {
            sudog* next = s->next;
            if (static_cast<entry*>(s)->elem == &wait) {
                b.q.remove(s);
                s->next = nullptr;
                if (tail != nullptr) {
                    tail->next = s;
                } else {
                    head = s;
                }
                tail = s;
            }
            s = next;
        }
        at(notify).store(at(wait).load());
    }
    // Go through the list and ready all waiters (read the link before the wake: the entry is
    // gone after it).
    for (sudog* s = head; s != nullptr;) {
        sudog* next = s->next;
        gp::detail::wake(s);
        s = next;
    }
}

void notify_one(uint32_t& wait, uint32_t& notify) {
    auto w = at(wait);
    auto n = at(notify);
    // Fast-path: if there are no waiters, there is nothing to do.
    if (w.load() == n.load()) {
        return;
    }
    bucket& b = bucket_of(&wait);
    entry* found = nullptr;
    {
        std::unique_lock<std::mutex> lk(b.mu);
        // Re-check under the lock.
        uint32_t t = n.load();
        if (w.load() == t) {
            return;
        }
        n.store(t + 1);
        // Try to find the g that needs to be notified. If it hasn't made it to the list yet we
        // won't find it, but it won't park: when it gets the lock it sees its ticket is below
        // notify.
        found = find(b, &wait, true, t);
        if (found != nullptr) {
            b.q.remove(found);
        }
    }
    if (found != nullptr) {
        gp::detail::wake(found);
    }
}

void fatal_unlock_mutex() {
    gp::detail::fatal("sync: unlock of unlocked mutex");
}

void fatal_runlock() {
    gp::detail::fatal("sync: RUnlock of unlocked RWMutex");
}

void fatal_unlock_rw() {
    gp::detail::fatal("sync: Unlock of unlocked RWMutex");
}

void panic_wg_negative() {
    gp::panic_constant(gp::string::gp_copy("sync: negative WaitGroup counter"));
}

void panic_wg_misuse_add() {
    gp::panic_constant(
        gp::string::gp_copy("sync: WaitGroup misuse: Add called concurrently with Wait"));
}

void panic_wg_reused() {
    gp::panic_constant(
        gp::string::gp_copy("sync: WaitGroup is reused before previous Wait has returned"));
}

void panic_cond_copied() {
    gp::panic_constant(gp::string::gp_copy("sync.Cond is copied"));
}

} // namespace gp::sync::detail
