// Goroutines (gocpp/goroutine.hpp): the threads, ids and count, and the end of a goroutine
// that panics. Standard C++ only: the OS is reached through gp::os (gocpp/os.hpp).
#include <gocpp/nofortify.h>
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>

#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <limits>
#include <memory>
#include <cerrno>
#include <mutex>
#include <new>
#include <string>
#include <thread>
#include <utility>

namespace gp {

namespace detail {

namespace {

// The next id. Id 1 is the first thread that asks (this_goroutine): the main goroutine.
constinit std::atomic<uint64_t> next_id{1};

// The goroutines that are alive (num_goroutine).
constinit std::atomic<int64_t> live{0};

// The goroutines that go statements started and that have not ended (running_goroutines), and the
// function that a go statement calls before it starts one (set_go_hook).
constinit std::atomic<int64_t> bodies{0};
constinit std::atomic<void (*)() noexcept> go_hook{nullptr};

// The goroutine records of the threads that no go statement started (a C library's thread, a
// std::thread, the main thread): a list, so that a leak check finds them reachable and a library
// that is unloaded can free them. A record is freed when its thread ends (release_adopted).
constinit std::mutex adopted_mu;
constinit goroutine* adopted = nullptr;

// The goroutine threads that are running or idle (go_start counts a thread before it starts it,
// worker_main stops counting as its last act): lib::shutdown waits for them to be gone before a
// shared library is unloaded, because their code is in it.
constinit std::atomic<int64_t> threads_alive{0};

// The default stack, 0 until a program sets it.
constinit std::atomic<std::size_t> stack_setting{0};

// How long a thread that did not get the claim waits for the one that did (claim_death).
constexpr std::chrono::seconds claim_wait{30};

// The thread that is ending the program (claim_death).
constinit std::atomic<const void*> dying{nullptr};

// The goroutines that are blocked in a gocpp operation (block_scope).
constinit std::atomic<int64_t> blocked{0};

// The panics that are being handled (panic_token), how long main waits for them, and how often it
// had to.
constinit std::atomic<int64_t> panicking_count{0};
constinit std::atomic<int64_t> settle_timeout_ns{200LL * 1000000};
constinit std::atomic<int64_t> settle_wait_count{0};

// --- the threads of goroutines, and their reuse ---
//
// Starting a thread costs 15 to 60 microseconds (the stack, the guard, the thread's own
// structures); a goroutine that is only a few instructions long would cost more to start than
// to run. So when a goroutine ends, its thread does not exit: it puts itself on a list of idle
// threads and waits for the next `go` statement, which takes the thread that went idle last
// (it has the warmest stack) and hands it the new goroutine. At most idle_max threads are idle
// at a time, and an idle thread gives up after idle_timeout. Only threads that have the default
// stack are reused (a goroutine that asks for another stack size gets a thread of its own).

// A worker is the record of a goroutine thread that lives longer than one goroutine.
struct worker {
    waiter wake; // the idle thread waits here
    goroutine* next =
        nullptr;           // the goroutine for the thread, set (with gp_unpark_with) by go_start
    bool quit = false;     // stop_idle_threads, set the same way
    std::size_t stack = 0; // the stack size the thread was started with
    worker* prev_idle = nullptr; // the list of idle workers (pool_mu)
    worker* next_idle = nullptr;
    bool listed = false;
};

// Under AddressSanitizer a thread that ends near the end of the program can't be stopped by
// LeakSanitizer ("Unable to get registers from thread"), so a sanitizer build keeps every thread
// of a finished goroutine for the next `go` statement and never lets one end by itself: the
// threads wait for ever, which LeakSanitizer handles (stop_idle_threads and set_idle_limit still
// end them).
#if defined(__SANITIZE_ADDRESS__)
#define GOCPP_KEEP_THREADS 1
#elif defined(__has_feature)
#if __has_feature(address_sanitizer)
#define GOCPP_KEEP_THREADS 1
#endif
#endif

#if defined(GOCPP_KEEP_THREADS)
constexpr int64_t initial_idle_timeout_ns = 24LL * 3600 * 1000000000LL;
constexpr std::size_t initial_idle_max = std::numeric_limits<std::size_t>::max();
#else
constexpr int64_t initial_idle_timeout_ns = 5LL * 1000000000LL;
constexpr std::size_t initial_idle_max = default_idle_limit;
#endif

constinit std::atomic<int64_t> idle_timeout_ns{initial_idle_timeout_ns};

constinit std::mutex pool_mu;
constinit worker* idle_head = nullptr; // the worker that went idle last
constinit std::size_t idle_count = 0;
constinit std::size_t idle_max = initial_idle_max;

// pool_push and pool_remove keep the list of idle workers; pool_mu is held.
void pool_push(worker* w) {
    w->prev_idle = nullptr;
    w->next_idle = idle_head;
    if (idle_head != nullptr) {
        idle_head->prev_idle = w;
    }
    idle_head = w;
    w->listed = true;
    idle_count++;
}

void pool_remove(worker* w) {
    if (w->prev_idle != nullptr) {
        w->prev_idle->next_idle = w->next_idle;
    } else {
        idle_head = w->next_idle;
    }
    if (w->next_idle != nullptr) {
        w->next_idle->prev_idle = w->prev_idle;
    }
    w->prev_idle = w->next_idle = nullptr;
    w->listed = false;
    idle_count--;
}

// run_goroutine runs the goroutine g on the calling thread, to its end.
void run_goroutine(goroutine* g) {
    os::tls_set(g);
    try {
        g->body->run();
    } catch (const panic_error& e) {
        // Go's report for the goroutine, then the end of the whole program.
        exit_with_panic(e);
    } catch (const goexit_signal&) {
        // runtime.Goexit: the deferred calls have run (every frame threw it again), and the
        // goroutine ends quietly.
    }
    // Any other exception goes on: nothing catches it, so the C++ library calls std::terminate
    // on the stack of the throw (run_main's handler, panic.hpp, prints Go's report).
    g->body.reset(); // the closure's values go before the goroutine stops counting
    os::tls_set(nullptr);
    bool counted = g->counted;
    delete g;
    if (counted) {
        live.fetch_sub(1, std::memory_order_acq_rel);
    }
    bodies.fetch_sub(1, std::memory_order_acq_rel);
}

// idle_wait puts the worker w on the idle list and waits for the next goroutine. It returns
// that goroutine, or null when the thread should end (the list is full, the wait ran out, or
// stop_idle_threads).
goroutine* idle_wait(worker* w) {
    if (w->stack == 0) {
        return nullptr; // a thread with a stack of its own is not reused
    }
    {
        std::lock_guard<std::mutex> lk(pool_mu);
        if (idle_count >= idle_max) {
            return nullptr;
        }
        pool_push(w);
    }
    auto ready = [w] { return w->next != nullptr || w->quit; };
    if (!w->wake.gp_wait_until(
            std::chrono::steady_clock::now() +
                std::chrono::nanoseconds(idle_timeout_ns.load(std::memory_order_relaxed)),
            ready)) {
        // Timed out. A go statement may have taken the worker off the list just now, and is
        // about to hand it a goroutine (it does that after it let go of pool_mu): wait for it.
        {
            std::lock_guard<std::mutex> lk(pool_mu);
            if (w->listed) {
                pool_remove(w);
                return nullptr;
            }
        }
        w->wake.gp_wait(ready);
    }
    goroutine* g = w->next;
    w->next = nullptr;
    return g; // null: quit
}

// The body of a goroutine's thread: the worker w, which go_start made, has the first goroutine.
void worker_main(void* p) {
    auto* w = static_cast<worker*>(p);
    goroutine* g = w->next;
    w->next = nullptr;
    while (g != nullptr) {
        run_goroutine(g);
        g = idle_wait(w);
    }
    delete w;
    threads_alive.fetch_sub(1, std::memory_order_acq_rel);
}

// take_idle returns an idle worker with a stack of the given size, off the list, or null.
worker* take_idle(std::size_t stack) {
    std::lock_guard<std::mutex> lk(pool_mu);
    worker* w = idle_head;
    if (w == nullptr || w->stack != stack) {
        return nullptr;
    }
    pool_remove(w);
    return w;
}

} // namespace

std::size_t stack_size() noexcept {
    std::size_t n = stack_setting.load(std::memory_order_relaxed);
    return n == 0 ? default_stack_size : n;
}

void set_stack_size(std::size_t n) noexcept {
    if (n != 0 && n < min_stack_size) {
        n = min_stack_size;
    }
    stack_setting.store(n, std::memory_order_relaxed);
}

goroutine& this_goroutine() noexcept {
    if (void* p = os::tls_get(); p != nullptr) {
        return *static_cast<goroutine*>(p);
    }
    // A thread that no go statement started. Without memory, the record is one that every
    // such thread shares (its id 0), so that nothing here can fail. It is built in static
    // storage and never destroyed: no static destructor runs at exit (the exit path is _exit).
    auto* g = new (std::nothrow) goroutine;
    if (g == nullptr) {
        alignas(goroutine) static char fallback_storage[sizeof(goroutine)];
        static std::once_flag fallback_once;
        std::call_once(fallback_once, [] { new (fallback_storage) goroutine; });
        return *std::launder(reinterpret_cast<goroutine*>(fallback_storage));
    }
    g->id = next_id.fetch_add(1, std::memory_order_relaxed);
    g->counted = g->id == 1;
    if (g->counted) {
        live.fetch_add(1, std::memory_order_acq_rel);
    }
    {
        std::lock_guard<std::mutex> lk(adopted_mu);
        g->next_adopted = adopted;
        if (adopted != nullptr) {
            adopted->prev_adopted = g;
        }
        adopted = g;
    }
    os::tls_set(g);
    // The record goes with the thread (a library that is called from a thread per request must
    // not leak one for each): when the thread ends, the OS layer calls release_adopted. The main
    // goroutine's thread ends with the process.
    os::tls_on_thread_exit(&release_adopted, g);
    return *g;
}

void release_adopted(void* p) noexcept {
    auto* g = static_cast<goroutine*>(p);
    // Anything that asks for the thread's goroutine later (another destructor of the thread)
    // gets a new record, not this one.
    os::tls_set(nullptr);
    {
        std::lock_guard<std::mutex> lk(adopted_mu);
        if (g->prev_adopted != nullptr) {
            g->prev_adopted->next_adopted = g->next_adopted;
        } else {
            adopted = g->next_adopted;
        }
        if (g->next_adopted != nullptr) {
            g->next_adopted->prev_adopted = g->prev_adopted;
        }
    }
    if (g->counted) {
        live.fetch_sub(1, std::memory_order_acq_rel);
    }
    delete g;
}

std::size_t adopted_count() noexcept {
    std::lock_guard<std::mutex> lk(adopted_mu);
    std::size_t n = 0;
    for (goroutine* g = adopted; g != nullptr; g = g->next_adopted) {
        n++;
    }
    return n;
}

void end_threads(int64_t timeout_ns) noexcept {
    stop_idle_threads();
    auto end = std::chrono::steady_clock::now() + std::chrono::nanoseconds(timeout_ns);
    while (threads_alive.load(std::memory_order_acquire) > 0 &&
           std::chrono::steady_clock::now() < end) {
        stop_idle_threads(); // a goroutine that ended since goes idle: it must end too
        std::this_thread::sleep_for(std::chrono::microseconds(200));
    }
    // The thread that counted itself out may still be returning from worker_main.
    std::this_thread::sleep_for(std::chrono::milliseconds(2));
}

int64_t running_goroutines() noexcept {
    return bodies.load(std::memory_order_acquire);
}

void set_go_hook(void (*fn)() noexcept) noexcept {
    go_hook.store(fn, std::memory_order_release);
}

void forget_adopted() noexcept {
    os::tls_on_thread_exit_stop(); // no thread calls release_adopted of this code any more
    std::lock_guard<std::mutex> lk(adopted_mu);
    while (adopted != nullptr) {
        goroutine* g = adopted;
        adopted = g->next_adopted;
        if (g->counted) {
            live.fetch_sub(1, std::memory_order_acq_rel);
        }
        delete g;
    }
}

uint64_t goroutine_id() noexcept {
    return this_goroutine().id;
}

void go_start(std::unique_ptr<task> body, std::size_t stack) {
    this_goroutine(); // the main goroutine is registered before the first one it starts
    auto g = std::make_unique<goroutine>();
    g->id = next_id.fetch_add(1, std::memory_order_relaxed);
    g->counted = true;
    g->body = std::move(body);
    // The count first, then the hook: a library that gives its reference back (lib::detail::leave)
    // does so only when it sees no goroutine, under the lock that the hook takes, so either it
    // sees this goroutine, or the hook runs after it and takes the reference again.
    bodies.fetch_add(1, std::memory_order_seq_cst);
    if (auto hook = go_hook.load(std::memory_order_acquire)) {
        hook();
    }
    live.fetch_add(1, std::memory_order_acq_rel);
    std::size_t size = stack != 0 ? stack : stack_size();
    if (stack == 0) {
        if (worker* w = take_idle(size); w != nullptr) {
            goroutine* gp = g.release(); // the idle thread owns it now
            w->wake.gp_unpark_with([w, gp] { w->next = gp; });
            return;
        }
    }
    auto* w = new (std::nothrow) worker;
    int err = ENOMEM;
    if (w != nullptr) {
        w->stack = stack == 0 ? size : 0; // 0: not reused
        w->next = g.get();
        threads_alive.fetch_add(1, std::memory_order_acq_rel);
        err = os::thread_start(worker_main, w, size);
        if (err != 0) {
            threads_alive.fetch_sub(1, std::memory_order_acq_rel);
            delete w;
        }
    }
    if (err == 0) {
        g.release(); // the goroutine owns it now
        return;
    }
    int64_t have = live.fetch_sub(1, std::memory_order_acq_rel) - 1;
    bodies.fetch_sub(1, std::memory_order_acq_rel);
    claim_death();
    std::string msg = "runtime: failed to create new OS thread (have ";
    print_int(msg, have);
    msg += " already; errno=";
    print_int(msg, err);
    msg += ")\n";
    if (err == EAGAIN) {
        msg += "runtime: may need to increase max user processes (ulimit -u)\n";
    }
    write_stderr(msg);
    fatal("newosproc");
}

std::size_t set_idle_limit(std::size_t n) noexcept {
    std::size_t old;
    worker* stop = nullptr;
    {
        std::lock_guard<std::mutex> lk(pool_mu);
        old = idle_max;
        idle_max = n;
        // The workers above the new limit, the oldest first, end.
        while (idle_count > n) {
            worker* w = idle_head;
            while (w->next_idle != nullptr) {
                w = w->next_idle;
            }
            pool_remove(w);
            w->next_idle = stop;
            stop = w;
        }
    }
    while (stop != nullptr) {
        worker* w = stop;
        stop = w->next_idle;
        w->wake.gp_unpark_with([w] { w->quit = true; });
    }
    return old;
}

int64_t set_settle_timeout(int64_t nanoseconds) noexcept {
    return settle_timeout_ns.exchange(nanoseconds, std::memory_order_acq_rel);
}

int64_t settle_waits() noexcept {
    return settle_wait_count.load(std::memory_order_relaxed);
}

int64_t set_idle_timeout(int64_t nanoseconds) noexcept {
    return idle_timeout_ns.exchange(nanoseconds, std::memory_order_acq_rel);
}

std::size_t idle_threads() noexcept {
    std::lock_guard<std::mutex> lk(pool_mu);
    return idle_count;
}

std::size_t stop_idle_threads() noexcept {
    std::size_t n = idle_threads();
    std::size_t old = set_idle_limit(0);
    set_idle_limit(old);
    return n;
}

void block_enter(const char* reason) noexcept {
    goroutine& g = this_goroutine();
    g.set_reason(reason);
    __atomic_store_n(&g.is_blocked, true, __ATOMIC_SEQ_CST);
    blocked.fetch_add(1, std::memory_order_acq_rel);
}

void unblock(goroutine& g) noexcept {
    if (__atomic_exchange_n(&g.is_blocked, false, __ATOMIC_SEQ_CST)) {
        blocked.fetch_sub(1, std::memory_order_acq_rel);
    }
}

void block_exit() noexcept {
    goroutine& g = this_goroutine();
    g.set_reason(nullptr);
    unblock(g);
}

void panic_count_add(int delta) noexcept {
    panicking_count.fetch_add(delta, std::memory_order_acq_rel);
}

int64_t panicking() noexcept {
    return panicking_count.load(std::memory_order_acquire);
}

void settle_panics() noexcept {
    // A goroutine that runs the deferred calls of a panic is about to die of it (or to recover):
    // give it a moment, as Go's runtime.main does (it yields 1000 times). Deferred calls are short;
    // one that blocks for ever with a panic pending must not hold the program for ever.
    if (panicking_count.load(std::memory_order_acquire) > 0) {
        settle_wait_count.fetch_add(1, std::memory_order_relaxed);
    }
    auto end = std::chrono::steady_clock::now() +
               std::chrono::nanoseconds(settle_timeout_ns.load(std::memory_order_relaxed));
    while (panicking_count.load(std::memory_order_acquire) > 0 &&
           dying.load(std::memory_order_acquire) == nullptr &&
           std::chrono::steady_clock::now() < end) {
        std::this_thread::sleep_for(std::chrono::microseconds(50));
    }
    if (dying.load(std::memory_order_acquire) != nullptr) {
        // Another goroutine is ending the program with its report: it exits (code 2) when done.
        // (claim_death's own limit of 30 s applies to the others; this thread waits as they do.)
        claim_death();
    }
}

void go_of_nil() noexcept {
    fatal("go of nil func value");
}

void block_forever(const char* reason) noexcept {
    block_scope b(reason);
    waiter w;
    w.gp_wait([] { return false; });
    __builtin_unreachable();
}

void claim_death() noexcept {
    // The address of a thread-local is this thread's token.
    static constinit thread_local char me = 0;
    const void* none = nullptr;
    if (dying.compare_exchange_strong(none, &me, std::memory_order_acq_rel) || none == &me) {
        return;
    }
    // Another thread is ending the program: wait for it to do it. But not for ever: if the
    // thread that reports is stuck, for example in malloc, whose lock this thread's signal
    // handler (a stack overflow) interrupted, nobody would ever end the program. After
    // claim_wait the program ends with code 2 (the report that is in progress may be cut).
    // 30 seconds is far longer than a report takes (the trace is read in milliseconds), and short
    // enough for a test or a user to see a hang as one. A slow claimer, one that is blocked
    // writing to a full stderr pipe or symbolizing a deep stack on a machine under heavy load,
    // can be cut off this way: the program then ends with code 2 and no message from this thread,
    // and what the claimer had not written yet is lost.
    for (auto left = claim_wait; left > std::chrono::seconds(0); left -= std::chrono::seconds(1)) {
        std::this_thread::sleep_for(std::chrono::seconds(1));
    }
    os::exit_now(2);
}

uint64_t current_goroutine_id() noexcept {
    // Async-signal-safe only while the runtime is linked into the executable: then os::tls_get
    // is a read of a thread-local variable at a fixed offset from the thread pointer (one mov
    // from %fs on x86-64). In a shared library, a thread-local read can call __tls_get_addr,
    // which may allocate on a thread's first use, so a signal handler must not rely on it there.
    auto* g = static_cast<goroutine*>(os::tls_get());
    return g != nullptr ? g->id : 0;
}

} // namespace detail

void exit(int code) noexcept {
    detail::claim_death();
    os::exit(code);
}

int64_t num_goroutine() noexcept {
    return detail::live.load(std::memory_order_acquire);
}

int64_t num_blocked() noexcept {
    return detail::blocked.load(std::memory_order_acquire);
}

} // namespace gp
