// gocpp/sync.hpp: the native functions of the standard package sync (Phase 08, step 8.7).
//
// Go's sync package is written in .gp (stdlib/sync): a Mutex is `struct { state int32; sema uint32
// }` and its methods call the functions below through //gocpp:native (ADR-027, ADR-040). They are
// templates over the generated struct (the Go type's C++ type is not known here): they read its
// fields by name, and a static_assert in each says which field and which type it needs, so a
// change in stdlib/sync that breaks the contract is a compile error, not a silent bug.
//
//     func (m *Mutex) Lock() { mutexLock(m) }      // gp::sync::mutex_lock(m): m is a
//     gp::ptr<Mutex>
//
// The algorithms are Go's, line by line (internal/sync/mutex.go, sync/rwmutex.go,
// sync/waitgroup.go, sync/cond.go, runtime/sema.go), over std::atomic_ref on the words of the
// struct, with sequentially consistent operations (Go's memory model). The fast paths are inline
// here (one atomic operation, nothing else); the slow paths are in src/sync.cpp.
//
// Nothing in a sync object is a pointer or a queue. A goroutine that has to wait parks on its own
// gp::waiter through the semaphore table of src/sync.cpp, a hash table keyed by the ADDRESS of the
// semaphore word, as Go's runtime/sema.go is (the Go runtime's sudog is gp::detail::sudog here).
// So the objects are plain words: a Mutex is 8 bytes, copying one copies its state (a copy of a
// locked mutex is locked, as in Go), and no destructor is needed. A waiter that parks is counted
// as blocked (gp::block_scope, with Go's wait reason: "sync.Mutex.Lock", "sync.WaitGroup.Wait",
// ...) for the deadlock detector (step 8.11), and the one that wakes it goes through
// gp::detail::wake (unblock), exactly like a channel does.
//
// Panics and fatal errors are Go's: sync.WaitGroup's "sync: negative WaitGroup counter" and the
// two misuse messages are panics with a string value (recover() returns the string);
// "sync: unlock of unlocked mutex", "sync: RUnlock of unlocked RWMutex" and the like are fatal
// errors ("fatal error: ...", exit code 2, no deferred call runs, no recover).
//
// Like every runtime type, nothing here has plain-named members: these are free functions.
#pragma once

#include <gocpp/iface.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/race.hpp>

#include <atomic>
#include <cstdint>
#include <type_traits>

namespace gp::sync {

namespace detail {

// The semaphore (runtime/sema.go). acquire waits until *addr is above 0 and takes one: lifo puts
// the waiter at the front of the queue (a waiter that was woken and lost the race), reason is the
// text of Go's traceback ("sync.Mutex.Lock"). release adds one and wakes the first waiter of
// addr; handoff gives the unit directly to it (the starving mutex). The memory is the caller's:
// the word must stay valid while any goroutine waits on it.
void sema_acquire(uint32_t* addr, const char* reason, bool lifo = false);
void sema_release(uint32_t* addr, bool handoff = false);

// A waiter that waited longer than this when it was woken puts the mutex in starvation mode: Go's
// threshold is 1 ms, plus here 100 us for each goroutine that waits too. A hand-off in starvation
// mode costs a futex wake-up and a context switch (20 to 50 us on an idle machine, more on a loaded
// one), where Go's costs about 200 ns, so with 100 threads on one mutex a wait of 1 ms is the
// normal order of the queue, not a sign of unfairness; with Go's constant the mutex went into
// starvation mode at once and stayed in it (every Unlock a hand-off, 13 s for 2M Lock/Unlock pairs
// of 100 threads, against 0.3 s without it). A wait that the length of the queue explains is not
// starvation; one that it does not explain is.
inline constexpr int64_t starvation_base_ns = 1000000;
inline constexpr int64_t starvation_per_waiter_ns = 100000;

// The clock of the starvation rule of the Mutex (a waiter that waited longer than the threshold
// when it was woken puts the mutex in starvation mode). A test sets its own, so that a waiter has
// "waited an hour" without waiting; nullptr (the default) is the monotonic clock. Only tests call
// it.
void set_clock_for_test(int64_t (*clock)() noexcept) noexcept;

// The number of goroutines that are in the queue of the semaphore word at addr right now (parked
// and visible to a release), not those that were woken and have not run yet, which the count of
// blocked goroutines still includes. Only tests call it.
int64_t sema_queued_for_test(const uint32_t* addr) noexcept;

// The mutex's slow paths (internal/sync/mutex.go lockSlow, unlockSlow).
void mutex_lock_slow(int32_t& state, uint32_t& sema);
void mutex_unlock_slow(int32_t& state, uint32_t& sema, int32_t new_state);

// The notify list of sync.Cond (runtime/sema.go notifyList): wait is the ticket counter (add),
// notify the next ticket to wake; the waiters are in the table, keyed by the address of wait.
void notify_wait(uint32_t& wait, uint32_t& notify, uint32_t ticket);
void notify_all(uint32_t& wait, uint32_t& notify);
void notify_one(uint32_t& wait, uint32_t& notify);

// The panics and fatal errors (they don't return).
[[noreturn]] void fatal_unlock_mutex();
[[noreturn]] void fatal_runlock();
[[noreturn]] void fatal_unlock_rw();
[[noreturn]] void panic_wg_negative();
[[noreturn]] void panic_wg_misuse_add();
[[noreturn]] void panic_wg_reused();
[[noreturn]] void panic_cond_copied();

inline constexpr int32_t mutex_locked = 1;
inline constexpr int32_t rw_max_readers = int32_t(1) << 30;

// ref is an atomic view of a word of a sync object. With -race (gocpp/race.hpp) its operations do
// not synchronize for ThreadSanitizer: they are the bookkeeping of the object, not the ordering
// that Go promises, and TSan would take each of them for a happens-before edge (a WaitGroup.Add
// after a Done would see everything that the goroutine of the Done did, which Go's model does not
// say). The edges of Go's model are put back by hand where they belong (race::release/acquire in
// Mutex, RWMutex and WaitGroup, as Go's sync package does with its race annotations).
template <typename T>
struct ref {
    T& x;
    T load() const noexcept {
        race::no_sync ns;
        return std::atomic_ref<T>(x).load();
    }
    void store(T v) const noexcept {
        race::no_sync ns;
        std::atomic_ref<T>(x).store(v);
    }
    T fetch_add(T v) const noexcept {
        race::no_sync ns;
        return std::atomic_ref<T>(x).fetch_add(v);
    }
    T fetch_sub(T v) const noexcept {
        race::no_sync ns;
        return std::atomic_ref<T>(x).fetch_sub(v);
    }
    bool compare_exchange_strong(T& expected, T desired) const noexcept {
        race::no_sync ns;
        return std::atomic_ref<T>(x).compare_exchange_strong(expected, desired);
    }
    bool compare_exchange_weak(T& expected, T desired) const noexcept {
        race::no_sync ns;
        return std::atomic_ref<T>(x).compare_exchange_weak(expected, desired);
    }
};

template <typename T>
inline ref<T> at(T& x) noexcept {
    return ref<T>{x};
}

// The Mutex on its two words, for the RWMutex that has one inside.
inline void mutex_lock_words(int32_t& state, uint32_t& sema) {
    int32_t zero = 0;
    // Fast path: grab an unlocked mutex.
    if (!at(state).compare_exchange_strong(zero, mutex_locked)) [[unlikely]] {
        mutex_lock_slow(state, sema);
    }
}

inline void mutex_unlock_words(int32_t& state, uint32_t& sema) {
    // Fast path: drop the lock bit.
    int32_t now = at(state).fetch_sub(mutex_locked) - mutex_locked;
    if (now != 0) [[unlikely]] {
        mutex_unlock_slow(state, sema, now);
    }
}

inline bool mutex_try_lock_words(int32_t& state) {
    auto s = at(state);
    int32_t old = s.load();
    if ((old & (mutex_locked | 4)) != 0) { // locked, or starving (mutexStarving)
        return false;
    }
    // A goroutine may be waiting; we are running and may grab the mutex before it wakes.
    return s.compare_exchange_strong(old, old | mutex_locked);
}

} // namespace detail

// --- Mutex: struct { state int32; sema uint32 } ---

// (With -race the edges of Go's memory model are put in by hand: an Unlock releases the mutex's
// address, a Lock that got the mutex acquires it.)
template <typename M>
inline void mutex_lock(const ptr<M>& m) {
    static_assert(std::is_same_v<decltype(M::state), int32_t> &&
                  std::is_same_v<decltype(M::sema), uint32_t>);
    M& r = *m;
    detail::mutex_lock_words(r.state, r.sema);
    race::acquire(&r.state);
}

template <typename M>
inline void mutex_unlock(const ptr<M>& m) {
    M& r = *m;
    race::release(&r.state);
    detail::mutex_unlock_words(r.state, r.sema);
}

template <typename M>
inline bool mutex_try_lock(const ptr<M>& m) {
    M& r = *m;
    if (!detail::mutex_try_lock_words(r.state)) {
        return false;
    }
    race::acquire(&r.state);
    return true;
}

// --- RWMutex: struct { w Mutex; writerSem, readerSem uint32; readerCount, readerWait int32 } ---

template <typename R>
inline void rw_rlock(const ptr<R>& rw) {
    static_assert(std::is_same_v<decltype(R::readerCount), int32_t> &&
                  std::is_same_v<decltype(R::readerWait), int32_t> &&
                  std::is_same_v<decltype(R::readerSem), uint32_t> &&
                  std::is_same_v<decltype(R::writerSem), uint32_t>);
    R& r = *rw;
    if (detail::at(r.readerCount).fetch_add(1) + 1 < 0) [[unlikely]] {
        // A writer is pending: wait for it to finish.
        detail::sema_acquire(&r.readerSem, "sync.RWMutex.RLock");
    }
    race::acquire(&r.readerSem); // an Unlock happens before the RLock that follows it
}

template <typename R>
inline bool rw_try_rlock(const ptr<R>& rw) {
    R& r = *rw;
    auto count = detail::at(r.readerCount);
    int32_t c = count.load();
    while (c >= 0) {
        if (count.compare_exchange_weak(c, c + 1)) {
            race::acquire(&r.readerSem);
            return true;
        }
    }
    return false;
}

template <typename R>
inline void rw_runlock(const ptr<R>& rw) {
    R& r = *rw;
    race::release(&r.writerSem); // an RUnlock happens before the Lock that follows it
    int32_t n = detail::at(r.readerCount).fetch_sub(1) - 1;
    if (n < 0) [[unlikely]] {
        // Outlined slow path: a writer is pending.
        if (n + 1 == 0 || n + 1 == -detail::rw_max_readers) {
            detail::fatal_runlock();
        }
        // A writer is pending: the last departing reader wakes it.
        if (detail::at(r.readerWait).fetch_sub(1) - 1 == 0) {
            detail::sema_release(&r.writerSem);
        }
    }
}

template <typename R>
inline void rw_lock(const ptr<R>& rw) {
    R& r = *rw;
    // First, resolve competition with other writers.
    detail::mutex_lock_words(r.w.state, r.w.sema);
    // Announce to readers there is a pending writer.
    int32_t n = detail::at(r.readerCount).fetch_sub(detail::rw_max_readers);
    // Wait for the active readers.
    if (n != 0 && detail::at(r.readerWait).fetch_add(n) + n != 0) {
        detail::sema_acquire(&r.writerSem, "sync.RWMutex.Lock");
    }
    race::acquire(&r.readerSem);
    race::acquire(&r.writerSem);
}

template <typename R>
inline bool rw_try_lock(const ptr<R>& rw) {
    R& r = *rw;
    if (!detail::mutex_try_lock_words(r.w.state)) {
        return false;
    }
    int32_t zero = 0;
    if (!detail::at(r.readerCount).compare_exchange_strong(zero, -detail::rw_max_readers)) {
        detail::mutex_unlock_words(r.w.state, r.w.sema);
        return false;
    }
    race::acquire(&r.readerSem);
    race::acquire(&r.writerSem);
    return true;
}

template <typename R>
inline void rw_unlock(const ptr<R>& rw) {
    R& r = *rw;
    race::release(&r.readerSem);
    race::release(&r.writerSem);
    // Announce to readers there is no active writer.
    int32_t n =
        detail::at(r.readerCount).fetch_add(detail::rw_max_readers) + detail::rw_max_readers;
    if (n >= detail::rw_max_readers) {
        detail::fatal_unlock_rw();
    }
    // Unblock blocked readers, if any.
    for (int32_t i = 0; i < n; i++) {
        detail::sema_release(&r.readerSem);
    }
    // Allow other writers to proceed.
    detail::mutex_unlock_words(r.w.state, r.w.sema);
}

// --- WaitGroup: struct { state uint64; sema uint32 } (counter << 32 | waiters) ---

template <typename W>
inline void wg_add(const ptr<W>& wg, int64_t delta) {
    static_assert(std::is_same_v<decltype(W::state), uint64_t> &&
                  std::is_same_v<decltype(W::sema), uint32_t>);
    W& r = *wg;
    if (delta < 0) {
        race::release(&r.state); // a Done happens before the Wait that it lets return
    }
    auto st = detail::at(r.state);
    uint64_t add = static_cast<uint64_t>(delta) << 32;
    uint64_t state = st.fetch_add(add) + add;
    auto v = static_cast<int32_t>(state >> 32);
    auto w = static_cast<uint32_t>(state & 0x7fffffff);
    if (v < 0) [[unlikely]] {
        detail::panic_wg_negative();
    }
    if (w != 0 && delta > 0 && v == static_cast<int32_t>(delta)) [[unlikely]] {
        detail::panic_wg_misuse_add();
    }
    if (v > 0 || w == 0) {
        return;
    }
    // This goroutine has set counter to 0 when waiters > 0. Now there can't be concurrent
    // mutations of state: adds must not happen concurrently with wait, and waiters must not be
    // incremented again while the counter is 0.
    if (st.load() != state) {
        detail::panic_wg_misuse_add();
    }
    st.store(0);
    for (; w != 0; w--) {
        detail::sema_release(&r.sema);
    }
}

template <typename W>
inline void wg_wait(const ptr<W>& wg) {
    W& r = *wg;
    auto st = detail::at(r.state);
    for (;;) {
        uint64_t state = st.load();
        auto v = static_cast<int32_t>(state >> 32);
        if (v == 0) {
            race::acquire(&r.state);
            return; // the counter is 0: don't wait
        }
        // Increment waiters count.
        if (st.compare_exchange_strong(state, state + 1)) {
            detail::sema_acquire(&r.sema, "sync.WaitGroup.Wait");
            if (st.load() != 0) {
                detail::panic_wg_reused();
            }
            race::acquire(&r.state);
            return;
        }
    }
}

// --- Cond: struct { L Locker; wait, notify uint32; checker uintptr } ---

namespace detail {
// The copy check of sync.Cond (copyChecker): the first use stores the address of the checker; a
// copy that is used later sees another address and panics ("sync.Cond is copied").
inline void cond_check(uintptr_t& checker) {
    auto self = reinterpret_cast<uintptr_t>(&checker);
    auto c = at(checker);
    if (c.load() != self) {
        uintptr_t zero = 0;
        if (!c.compare_exchange_strong(zero, self) && c.load() != self) {
            panic_cond_copied();
        }
    }
}
} // namespace detail

// cond_add checks the Cond and returns this waiter's ticket; cond_wait waits for it to be called
// (it returns at once when it was called between the two); Cond.Wait is
// ticket := condAdd(c); c.L.Unlock(); condWait(c, ticket); c.L.Lock().
template <typename C>
inline uint32_t cond_add(const ptr<C>& c) {
    static_assert(std::is_same_v<decltype(C::wait), uint32_t> &&
                  std::is_same_v<decltype(C::notify), uint32_t> &&
                  std::is_same_v<decltype(C::checker), uintptr_t>);
    C& r = *c;
    detail::cond_check(r.checker);
    return detail::at(r.wait).fetch_add(1);
}

template <typename C>
inline void cond_wait(const ptr<C>& c, uint32_t ticket) {
    C& r = *c;
    detail::notify_wait(r.wait, r.notify, ticket);
    race::acquire(&r.wait); // a Signal or Broadcast happens before the Wait that it wakes
}

template <typename C>
inline void cond_signal(const ptr<C>& c) {
    C& r = *c;
    detail::cond_check(r.checker);
    race::release(&r.wait);
    detail::notify_one(r.wait, r.notify);
}

template <typename C>
inline void cond_broadcast(const ptr<C>& c) {
    C& r = *c;
    detail::cond_check(r.checker);
    race::release(&r.wait);
    detail::notify_all(r.wait, r.notify);
}

// hash_check(key): hash the key of a sync.Map operation, so that an unhashable dynamic type panics
// ("runtime error: hash of unhashable type []int") before any lock is taken.
inline void hash_check(const gp::iface& key) {
    (void)key.gp_hash();
}

// --- Once: struct { done uint32; m Mutex } ---

template <typename O>
inline bool once_done(const ptr<O>& o) {
    static_assert(std::is_same_v<decltype(O::done), uint32_t>);
    bool done = detail::at((*o).done).load() != 0;
    if (done) {
        race::acquire(
            &(*o).done); // the function's work happens before every Do that returns after it
    }
    return done;
}

template <typename O>
inline void once_set(const ptr<O>& o) {
    race::release(&(*o).done);
    detail::at((*o).done).store(1);
}

} // namespace gp::sync
