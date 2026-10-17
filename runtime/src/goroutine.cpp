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
#include <memory>
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

// The goroutine records of the threads that no go statement started: never freed, and
// reachable from here (so a leak check doesn't report them).
constinit std::atomic<goroutine*> adopted{nullptr};

// The default stack, 0 until a program sets it.
constinit std::atomic<std::size_t> stack_setting{0};

// How long a thread that did not get the claim waits for the one that did (claim_death).
constexpr std::chrono::seconds claim_wait{30};

// The thread that is ending the program (claim_death).
constinit std::atomic<const void*> dying{nullptr};

// The body of a goroutine's thread: g, which go_start made, comes in the argument.
void goroutine_main(void* p) {
    auto* g = static_cast<goroutine*>(p);
    os::tls_set(g);
    try {
        g->body->run();
    } catch (const panic_error& e) {
        // Go's report for the goroutine, then the end of the whole program.
        exit_with_panic(e);
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
    g->next_adopted = adopted.load(std::memory_order_relaxed);
    while (!adopted.compare_exchange_weak(g->next_adopted, g, std::memory_order_release,
                                          std::memory_order_relaxed)) {
    }
    os::tls_set(g);
    return *g;
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
    live.fetch_add(1, std::memory_order_acq_rel);
    int err = os::thread_start(goroutine_main, g.get(), stack != 0 ? stack : stack_size());
    if (err == 0) {
        g.release(); // the goroutine owns it now
        return;
    }
    int64_t have = live.fetch_sub(1, std::memory_order_acq_rel) - 1;
    claim_death();
    std::string msg = "runtime: failed to create new OS thread (have ";
    print_int(msg, have);
    msg += " already; errno=";
    print_int(msg, err);
    msg += ")\n";
    write_stderr(msg);
    fatal("newosproc");
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

int64_t num_goroutine() noexcept {
    return detail::live.load(std::memory_order_acquire);
}

} // namespace gp
