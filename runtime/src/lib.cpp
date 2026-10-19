// The start of a library (gocpp/lib.hpp): gocpp code that a C or C++ program calls through the
// `//export` functions of the build modes c-archive and c-shared. Standard C++ only: the OS is
// reached through gocpp/os.hpp.
#include <gocpp/lib.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>

#include <atomic>
#include <chrono>
#include <cstdlib>
#include <mutex>
#include <thread>

namespace gp::lib {

namespace {

constinit std::atomic<bool> started{false};
constinit std::atomic<bool> finished{false};
std::once_flag start_once;

// Is this thread running the package initialization (enter's own call of init)? An exported
// function that init reaches through C must not wait for init to finish.
constinit thread_local bool in_init = false;

// How long shutdown waits for the goroutine threads to end when the library is unloaded: they are
// idle (they end at once), because a library with a running goroutine is never unloaded (see pin).
constexpr int64_t shutdown_wait_ns = 3LL * 1000 * 1000 * 1000;

// And when the process ends with a goroutine alive: the threads die with the process, so only the
// idle ones are asked to end. (A library with a running goroutine is never unloaded, so shutdown
// finds one only at the end of the process.)
constexpr int64_t exit_wait_ns = 20LL * 1000 * 1000;

// How long a call that began with no goroutine waits, when it returns, for the goroutines it
// started to end (before the library keeps its reference to itself until a later call).
constexpr std::chrono::milliseconds leave_wait{250};

// The reference that keeps a shared library mapped while a goroutine runs (go_hook, leave): a
// goroutine's thread runs code of the library, and dlclose would unmap it. With the reference
// the host's dlclose does nothing (the library stays, its destructors do not run); the host's next
// call into the library gives the reference back when no goroutine runs any more. Guarded by
// pin_mu; pinned says whether pin is set, for the check that every call makes.
std::mutex pin_mu;
void* pin = nullptr;
constinit std::atomic<bool> pinned{false};

void on_go() noexcept {
    std::lock_guard<std::mutex> lk(pin_mu);
    if (pin == nullptr) {
        pin = ::gp::os::library_ref(reinterpret_cast<const void*>(&on_go));
        pinned.store(pin != nullptr, std::memory_order_release);
    }
}

} // namespace

void enter(void (*init)()) noexcept {
    ::gp::detail::this_goroutine(); // the calling thread is a goroutine, from the first call on
    if (finished.load(std::memory_order_acquire) || in_init) {
        return;
    }
    std::call_once(start_once, [init] {
        started.store(true, std::memory_order_release);
        in_init = true;
        try {
            // A library has no run_main: the host's arguments and environment, no handler of
            // signals or of std::terminate (they are the host's), registered shutdown.
            ::gp::detail::load_process_info();
            std::atexit(&shutdown);
            ::gp::detail::set_go_hook(&on_go);
            if (init != nullptr) {
                init();
            }
        } catch (const gp::panic_error& e) {
            detail::fail_panic(e);
        } catch (const gp::goexit_signal&) {
            detail::fail_goexit();
        } catch (...) {
            detail::fail_exception();
        }
        in_init = false;
        finished.store(true, std::memory_order_release);
    });
}

bool entered() noexcept {
    return finished.load(std::memory_order_acquire);
}

void shutdown() noexcept {
    if (!started.load(std::memory_order_acquire)) {
        return;
    }
    // A library with a running goroutine is never unloaded (pin), so when one runs here the
    // process is ending: the goroutines die with it and nothing waits for them.
    ::gp::detail::end_threads(::gp::detail::running_goroutines() > 0 ? exit_wait_ns
                                                                     : shutdown_wait_ns);
    ::gp::detail::forget_adopted();
}

namespace detail {

int64_t goroutines_now() noexcept {
    return ::gp::detail::running_goroutines();
}

void leave(int64_t before) noexcept {
    if (!pinned.load(std::memory_order_acquire)) {
        return;
    }
    if (::gp::detail::running_goroutines() > 0) {
        // No lock: a library with a background goroutine does not serialize its calls here. A
        // call that began with none may have started goroutines that are about to end: give them a
        // moment.
        if (before != 0) {
            return;
        }
        // Bounded by time, not by a count of yields: on a busy machine the goroutine threads
        // may not get the CPU for a while. A goroutine that does not end within the bound is one
        // that stays (the library is pinned until a later call).
        auto end = std::chrono::steady_clock::now() + leave_wait;
        for (int i = 0; ::gp::detail::running_goroutines() > 0; i++) {
            if (i < 100) {
                std::this_thread::yield();
            } else {
                if (std::chrono::steady_clock::now() >= end) {
                    break;
                }
                std::this_thread::sleep_for(std::chrono::microseconds(50));
            }
        }
        if (::gp::detail::running_goroutines() > 0) {
            return;
        }
    }
    std::lock_guard<std::mutex> lk(pin_mu);
    if (pin != nullptr && ::gp::detail::running_goroutines() == 0) {
        ::gp::os::library_unref(pin);
        pin = nullptr;
        pinned.store(false, std::memory_order_release);
    }
}

} // namespace detail

namespace detail {

void fail_panic(const gp::panic_error& e) noexcept {
    gp::detail::exit_with_panic(e);
}

void fail_goexit() noexcept {
    gp::detail::fatal("runtime.Goexit called in a thread that was not created by the Go runtime");
}

void fail_exception() noexcept {
    gp::detail::on_terminate(); // called in a catch block: reports the exception, exit code 2
}

void fail_owned_pointer() noexcept {
    gp::detail::fatal("gocpp: an exported function returned a pointer to gocpp memory to C: "
                      "return a pointer from C (malloc), a package variable's address, or a "
                      "copy");
}

void fail_memory() noexcept {
    gp::detail::fatal("runtime: out of memory");
}

} // namespace detail

} // namespace gp::lib
