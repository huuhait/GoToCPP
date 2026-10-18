// The Linux thread layer (gocpp/os.hpp, "threads"): detached pthreads with a stack size, the
// thread's slot, the stack bounds, and the detection of a stack overflow on any thread that
// has bounds. The portable runtime (src/goroutine.cpp, src/rt.cpp) builds the goroutines and
// Go's report on top of it.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include "linux.hpp"

#include <atomic>
#include <cerrno>
#include <csignal>
#include <cstddef>
#include <cstdint>
#include <cstring>
#include <limits>
#include <new>

#include <pthread.h>
#include <sys/auxv.h>
#include <sys/prctl.h>
#include <sys/resource.h>
#include <ucontext.h>
#include <unistd.h>

extern "C" char** environ;

namespace gp::os {

namespace {

// --- the stack of a thread ---
//
// Go's goroutine stacks grow, up to 1 GB, and a deeper recursion ends with "runtime: goroutine
// stack exceeds 1000000000-byte limit" and "fatal error: stack overflow" (exit code 2). A gocpp
// goroutine runs on a fixed C++ stack (8 MiB by default for a goroutine and for the main thread,
// the ulimit -s), and a recursion that is too deep hits the end of it: a SIGSEGV, which the
// kernel would turn into a crash (exit code 139). So the runtime installs a SIGSEGV handler,
// on a stack of its own (sigaltstack: the thread's stack is full), that tells a stack overflow
// from other memory faults and calls the hook of the portable runtime, which prints Go's lines.
// Any other fault gets the action that was there before (the default: the crash), as if the
// handler weren't there.
//
// A fault is a stack overflow when its address is near the lowest address the faulting
// thread's stack may use, and the stack pointer is near it too. The thread's bounds are in
// its own thread-local record (tstack): a thread without bounds (a thread that thread_start
// didn't start) is never an overflow.
//   - A thread of thread_start: the bounds come from pthread_getattr_np, and below them is a
//     guard of 256 KiB, protected, so the first access past the end faults there.
//   - The main thread: the kernel grows its stack mapping down to its top minus RLIMIT_STACK,
//     and the top is exact: it ends just after the program's file name, the last string that
//     exec puts on the stack (getauxval(AT_EXECFN)). Without a limit (ulimit -s unlimited)
//     there is no such address, so no bounds, and every fault keeps the old action.
//
// The handler only formats numbers and calls write and _exit (in the hook), which are
// async-signal-safe: no stack trace (Go prints one), because walking the stack and reading the
// program's file from a signal handler isn't safe (the fault may be inside malloc).

// The window around the end of the stack where a fault is an overflow. overflow_below: a frame
// can skip further below the end (a big local array without stack probes); it is more than the
// guard (256 KiB). overflow_above_max: the kernel also keeps a gap (stack_guard_gap, 1 MiB)
// between the main thread's stack and a mapping below it, so with a mapping just below the end,
// the stack stops up to that much higher; at most a quarter of a smaller stack.
[[maybe_unused]] constexpr uintptr_t overflow_below = uintptr_t(1) << 20;
constexpr uintptr_t overflow_above_max = (uintptr_t(1) << 20) + (uintptr_t(64) << 10);

// The guard below the stack of a thread of thread_start, and the alternate signal stack that
// each of them has (64 KiB: the handler needs little, and the signal frame of a CPU with large
// vector registers (AVX-512) takes a few KiB).
constexpr std::size_t guard_size = std::size_t(256) << 10;
[[maybe_unused]] constexpr std::size_t alt_size = std::size_t(64) << 10;

// The calling thread's stack: lo is the lowest address it may use (0: unknown), hi its end,
// above how far above lo a fault still counts as an overflow.
struct thread_stack {
    uintptr_t lo;
    uintptr_t hi;
    uintptr_t above;
};
constinit thread_local thread_stack tstack{};

// The thread's slot (tls_get).
constinit thread_local void* tslot = nullptr;

constinit std::atomic<stack_overflow_hook> overflow_hook{nullptr};

uintptr_t above_of(uintptr_t lo, uintptr_t hi) noexcept {
    uintptr_t size = hi - lo;
    return size / 4 < overflow_above_max ? size / 4 : overflow_above_max;
}

#if defined(GOCPP_OWN_SEGV)

// The main thread's alternate stack.
alignas(64) constinit char main_alt_stack[alt_size]{};

// The SIGSEGV action before start's, which a fault that isn't an overflow gets back.
struct sigaction old_segv{};

// fault_sp returns the stack pointer of the code that faulted, or 0 when the CPU isn't one
// that the handler knows.
uintptr_t fault_sp([[maybe_unused]] void* ctx) noexcept {
#if defined(__x86_64__)
    return static_cast<uintptr_t>(static_cast<ucontext_t*>(ctx)->uc_mcontext.gregs[REG_RSP]);
#elif defined(__aarch64__)
    return static_cast<uintptr_t>(static_cast<ucontext_t*>(ctx)->uc_mcontext.sp);
#else
    return 0;
#endif
}

bool near_end(const thread_stack& s, uintptr_t a) noexcept {
    return a + overflow_below >= s.lo && a < s.lo + s.above;
}

void on_sigsegv(int sig, siginfo_t* info, void* ctx) {
    const thread_stack& s = tstack;
    auto addr = reinterpret_cast<uintptr_t>(info->si_addr);
    uintptr_t sp = fault_sp(ctx);
    // si_code > 0: a fault of the CPU, not a kill(2) of SIGSEGV.
    if (info->si_code > 0 && s.lo != 0 && near_end(s, addr) && (sp == 0 || near_end(s, sp))) {
        if (stack_overflow_hook h = overflow_hook.load(std::memory_order_acquire); h != nullptr) {
            h(stack_fault{sp != 0 ? sp : addr, s.lo, s.hi});
        }
    }
    // Any other fault (or a hook that returned): the old action. Returning runs the faulting
    // instruction again, which faults again and gets it; a SIGSEGV sent with kill must be
    // raised again (it is blocked until the handler returns).
    sigaction(sig, &old_segv, nullptr);
    if (info->si_code <= 0) {
        ::raise(sig);
    }
}

// use_alt_stack makes [p, p + alt_size) the calling thread's alternate signal stack.
bool use_alt_stack(char* p) noexcept {
    stack_t ss{};
    ss.ss_sp = p;
    ss.ss_size = alt_size;
    return sigaltstack(&ss, nullptr) == 0;
}

void drop_alt_stack() noexcept {
    stack_t ss{};
    ss.ss_flags = SS_DISABLE;
    sigaltstack(&ss, nullptr);
}

// install_handler installs the SIGSEGV handler, once.
bool install_handler() noexcept {
    struct sigaction sa{};
    sa.sa_sigaction = on_sigsegv;
    sa.sa_flags = SA_SIGINFO | SA_ONSTACK;
    sigemptyset(&sa.sa_mask);
    return sigaction(SIGSEGV, &sa, &old_segv) == 0;
}

#endif // GOCPP_OWN_SEGV

// main_stack_top returns the end of the main thread's stack mapping: exec puts the program's
// file name last (AT_EXECFN), then a null pointer, at the end of the mapping, which ends on a
// page. Without AT_EXECFN: after the last environment string (the strings come before the
// file name), or else the frame of the caller, and the end is a page or more higher.
uintptr_t main_stack_top() noexcept {
    auto page = static_cast<uintptr_t>(sysconf(_SC_PAGESIZE));
    if (page == 0 || (page & (page - 1)) != 0) {
        page = 4096;
    }
    const char* last = reinterpret_cast<const char*>(getauxval(AT_EXECFN));
    if (last == nullptr && environ != nullptr && environ[0] != nullptr) {
        last = environ[0];
        for (char** e = environ; *e != nullptr; e++) {
            if (*e > last) {
                last = *e;
            }
        }
    }
    // The frame, not the address of a local: with AddressSanitizer a local may be on a fake
    // stack.
    uintptr_t end = reinterpret_cast<uintptr_t>(__builtin_frame_address(0));
    // A string that isn't above this frame isn't on the stack (an environment that C++ code
    // changed before run_main, with setenv).
    if (last != nullptr && reinterpret_cast<uintptr_t>(last) > end) {
        end = reinterpret_cast<uintptr_t>(last) + std::strlen(last) + 1 + sizeof(void*);
    }
    return (end + page - 1) & ~(page - 1);
}

// The start of a thread of thread_start: what it runs.
struct start_info {
    void (*fn)(void*);
    void* arg;
};

void* thread_main(void* p) {
    auto* si = static_cast<start_info*>(p);
    void (*fn)(void*) = si->fn;
    void* arg = si->arg;
    delete si;
#if defined(GOCPP_OWN_SEGV)
    // The alternate stack is part of this frame, at the top of the thread's stack, where the
    // recursion never gets to. It is given back before the thread returns.
    alignas(64) char alt[alt_size];
    bool has_alt = use_alt_stack(alt);
#endif
    pthread_attr_t attr;
    if (pthread_getattr_np(pthread_self(), &attr) == 0) {
        void* addr = nullptr;
        std::size_t size = 0;
        if (pthread_attr_getstack(&attr, &addr, &size) == 0 && addr != nullptr) {
            auto lo = reinterpret_cast<uintptr_t>(addr);
            tstack = {lo, lo + size, above_of(lo, lo + size)};
        }
        pthread_attr_destroy(&attr);
    }
    fn(arg);
    tstack = {};
#if defined(GOCPP_OWN_SEGV)
    if (has_alt) {
        drop_alt_stack();
    }
#endif
    return nullptr;
}

} // namespace

void start_threads() noexcept {
    // The main thread's stack. Without a usable limit there is nothing to detect.
    rlimit rl{};
    uintptr_t top = main_stack_top();
    if (getrlimit(RLIMIT_STACK, &rl) != 0 || rl.rlim_cur == RLIM_INFINITY || rl.rlim_cur >= top ||
        rl.rlim_cur < (uintptr_t(64) << 10)) {
        return; // no limit, or a strange one
    }
    auto limit = static_cast<uintptr_t>(rl.rlim_cur);
    tstack = {top - limit, top, above_of(top - limit, top)};
#if defined(GOCPP_OWN_SEGV)
    if (!use_alt_stack(main_alt_stack) || !install_handler()) {
        drop_alt_stack();
    }
#endif
}

int thread_start(void (*fn)(void*), void* arg, std::size_t stack_size) noexcept {
    auto page = static_cast<std::size_t>(sysconf(_SC_PAGESIZE));
    if (page == 0 || (page & (page - 1)) != 0) {
        page = 4096;
    }
    std::size_t extra = 0;
#if defined(GOCPP_OWN_SEGV)
    extra = alt_size;
#endif
    if (stack_size > std::numeric_limits<std::size_t>::max() - extra - page) {
        return EINVAL;
    }
    std::size_t size = (stack_size + extra + page - 1) / page * page;
    if (auto min = static_cast<std::size_t>(PTHREAD_STACK_MIN) + extra; size < min) {
        size = (min + page - 1) / page * page;
    }
    auto* si = new (std::nothrow) start_info{fn, arg};
    if (si == nullptr) {
        return ENOMEM;
    }
    pthread_attr_t attr;
    int rc = pthread_attr_init(&attr);
    if (rc != 0) {
        delete si;
        return rc;
    }
    pthread_t id;
    rc = pthread_attr_setdetachstate(&attr, PTHREAD_CREATE_DETACHED);
    if (rc == 0) {
        rc = pthread_attr_setstacksize(&attr, size);
    }
    if (rc == 0) {
        rc = pthread_attr_setguardsize(&attr, guard_size);
    }
    if (rc == 0) {
        rc = pthread_create(&id, &attr, thread_main, si);
    }
    pthread_attr_destroy(&attr);
    if (rc != 0) {
        delete si;
    }
    return rc;
}

void exit_thread() {
    pthread_exit(nullptr);
}

void* tls_get() noexcept {
    return tslot;
}

void tls_set(void* p) noexcept {
    tslot = p;
}

bool stack_bounds(uintptr_t& lo, uintptr_t& hi) noexcept {
    if (tstack.lo == 0) {
        return false;
    }
    lo = tstack.lo;
    hi = tstack.hi;
    return true;
}

stack_overflow_hook set_stack_overflow_hook(stack_overflow_hook hook) noexcept {
    return overflow_hook.exchange(hook, std::memory_order_acq_rel);
}

void exit_now(int code) noexcept {
    ::_exit(code);
}

void no_core_dump() noexcept {
    prctl(PR_SET_DUMPABLE, 0);
}

} // namespace gp::os
