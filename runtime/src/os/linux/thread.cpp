// The Linux thread layer (gocpp/os.hpp, "threads"): detached pthreads with a stack size, the
// thread's slot, the stack bounds, and the detection of a stack overflow on any thread that
// has bounds. The portable runtime (src/goroutine.cpp, src/rt.cpp) builds the goroutines and
// Go's report on top of it.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include <gocpp/race.hpp>
#include "linux.hpp"

#include <atomic>
#include <cerrno>
#include <csignal>
#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <limits>
#include <new>

#include <pthread.h>
#include <sched.h>
#include <fcntl.h>
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
// goroutine runs on a fixed C++ stack (8 MiB by default for a goroutine; the main thread's grows
// up to RLIMIT_STACK, which start raises: below), and a recursion that is too deep hits the end of
// it: a SIGSEGV, which the
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
//   - The main thread: the kernel grows its stack mapping down to its top minus RLIMIT_STACK (the
//     limit of the moment of the fault), and the top is exact: it ends just after the program's
//     file name, the last string that exec puts on the stack (getauxval(AT_EXECFN)). start sets the
//     soft limit to main_stack_limit (1 GiB, Go's limit; at most the hard limit), from 8 MiB as a
//     rule, so that the main goroutine recurses as deep as Go's. The kernel also stops the stack
//     stack_guard_gap (1 MiB) above the mapping below it, and that mapping's place was fixed by
//     exec with the limit of that time: mmap_base is at least 128 MiB (plus the randomization of
//     the layout, without ASLR none) below the top of the stack. So the lowest address is the
//     higher of the two ends, read from /proc/self/maps: with ASLR the limit (the gap is
//     gigabytes), without ASLR (setarch -R, gdb) about 127 MiB.
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

// The soft RLIMIT_STACK that start gives the main thread: Go's limit of a goroutine's stack (1 GB),
// rounded to 1 GiB. A program that execs another passes the raised limit on (glibc's default stack
// of a thread is the soft limit: only the threads that ask for no size get it).
constexpr uintptr_t main_stack_limit = uintptr_t(1) << 30;

// The gap that the kernel keeps between a stack and the mapping below it (stack_guard_gap: 256
// pages by default, a boot parameter that a process can't read).
constexpr uintptr_t stack_guard_gap = uintptr_t(1) << 20;

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

// The main thread's record (its tstack), for main_stack_changed, which another thread may call:
// the fields that it changes (lo, above) are read and written with relaxed atomics. main_region is
// how far below the top the main thread's stack may ever reach (the larger of the soft limit after
// start and Go's 1 GiB): a fault with the stack pointer in there is the main thread's overflow even
// when lo is out of date (on_sigsegv).
constinit thread_stack* main_tstack = nullptr;
constinit uintptr_t main_region = 0;

uintptr_t load_relaxed(const uintptr_t& v) noexcept {
    return __atomic_load_n(&v, __ATOMIC_RELAXED);
}

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
    uintptr_t lo = load_relaxed(s.lo);
    return a + overflow_below >= lo && a < lo + load_relaxed(s.above);
}

// fault_pc returns the address of the instruction that faulted, or 0 (as fault_sp).
uintptr_t fault_pc([[maybe_unused]] void* ctx) noexcept {
#if defined(__x86_64__)
    return static_cast<uintptr_t>(static_cast<ucontext_t*>(ctx)->uc_mcontext.gregs[REG_RIP]);
#elif defined(__aarch64__)
    return static_cast<uintptr_t>(static_cast<ucontext_t*>(ctx)->uc_mcontext.pc);
#else
    return 0;
#endif
}

// main_overflow says whether a fault of the main thread is a stack overflow without its exact end:
// a data access (not an instruction fetch: pc is not addr) to an address that nothing maps
// (SEGV_MAPERR), just below the stack pointer (a push, a call, a frame's probe; a frame that is not
// probed reaches at most overflow_below), with the stack pointer in the region the main stack may
// use. Since Linux 4.20 any access in the stack's growth area grows the stack, so such a fault can
// only be the stack that could not grow: the program lowered its RLIMIT_STACK itself (C's
// setrlimit), a mapping was placed in the gap below the stack (mmap with a hint), or
// /proc/self/maps could not be read at start. A fault on a page that is mapped (SEGV_ACCERR: a
// read-only page below sp, code run from the stack, which is not executable) is no overflow.
bool main_overflow(const thread_stack& s, const siginfo_t* info, void* ctx) noexcept {
    auto addr = reinterpret_cast<uintptr_t>(info->si_addr);
    uintptr_t sp = fault_sp(ctx);
    return &s == main_tstack && info->si_code == SEGV_MAPERR && addr != fault_pc(ctx) && sp != 0 &&
           sp < s.hi && sp + main_region + overflow_below >= s.hi && addr + overflow_below >= sp &&
           addr < sp + (uintptr_t(4) << 10);
}

void on_sigsegv(int sig, siginfo_t* info, void* ctx) {
    const thread_stack& s = tstack;
    auto addr = reinterpret_cast<uintptr_t>(info->si_addr);
    uintptr_t sp = fault_sp(ctx);
    // si_code > 0: a fault of the CPU, not a kill(2) of SIGSEGV.
    if (info->si_code > 0 && load_relaxed(s.lo) != 0 && near_end(s, addr) &&
        (sp == 0 || near_end(s, sp))) {
        if (stack_overflow_hook h = overflow_hook.load(std::memory_order_acquire); h != nullptr) {
            h(stack_fault{sp != 0 ? sp : addr, load_relaxed(s.lo), s.hi});
        }
    } else if (main_overflow(s, info, ctx)) {
        // The stack ended above lo: the page of sp is as low as it got.
        if (stack_overflow_hook h = overflow_hook.load(std::memory_order_acquire); h != nullptr) {
            h(stack_fault{sp, sp & ~uintptr_t(4095), s.hi});
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
    // The alternate stack comes from malloc and is given back before the thread returns. (It
    // used to be a local array of this frame, but a frame of 64 KiB is probed page by page
    // (-fstack-clash-protection, which every compile of gocpp has), so the thread touched all
    // of it: 64 KiB of resident memory per goroutine, ten times what the rest of a blocked
    // goroutine costs. malloc's chunk stays untouched until a signal uses it.) Without
    // memory, the thread runs without one, and its overflow is a plain crash.
    char* alt = static_cast<char*>(std::malloc(alt_size));
    bool has_alt = alt != nullptr && use_alt_stack(alt);
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
    std::free(alt);
#endif
    return nullptr;
}

} // namespace

namespace {

// set_main_stack_limit raises the soft RLIMIT_STACK to main_stack_limit (at most the hard limit)
// and returns the soft limit that holds after it (RLIM_INFINITY when it can't be read). A soft
// limit above 1 GiB that the user set stays. Unlimited becomes 1 GiB, Go's limit: the stack would
// otherwise grow until the memory or the swap is gone (the kernel's legacy layout of an unlimited
// stack puts nothing below it for terabytes), and an endless recursion would end with the OOM
// killer, not with Go's report.
rlim_t set_main_stack_limit() noexcept {
    rlimit rl{};
    if (getrlimit(RLIMIT_STACK, &rl) != 0) {
        return RLIM_INFINITY;
    }
    if (rl.rlim_cur != RLIM_INFINITY && rl.rlim_cur >= main_stack_limit) {
        return rl.rlim_cur;
    }
    rlim_t want = main_stack_limit;
    if (rl.rlim_max != RLIM_INFINITY && rl.rlim_max < want) {
        want = rl.rlim_max;
    }
    if (rl.rlim_cur != want) {
        rlimit next{want, rl.rlim_max};
        if (setrlimit(RLIMIT_STACK, &next) == 0) {
            rl.rlim_cur = want;
        }
    }
    return rl.rlim_cur;
}

// below_mapping_end returns the end of the mapping right below the one that holds addr, from
// /proc/self/maps (0: none, or the file can't be read). It reads the file in pieces into a buffer
// of its own: no allocation, before anything else runs.
uintptr_t below_mapping_end(uintptr_t addr) noexcept {
    int fd = ::open("/proc/self/maps", O_RDONLY | O_CLOEXEC);
    if (fd < 0) {
        return 0;
    }
    char buf[4096];
    std::size_t have = 0; // bytes of a line not yet parsed, at the start of buf
    uintptr_t prev_end = 0;
    uintptr_t found = 0;
    bool done = false;
    bool skip = false; // the rest of a line longer than the buffer comes first: not parsed
    while (!done) {
        ssize_t n = ::read(fd, buf + have, sizeof buf - have);
        if (n < 0 && errno == EINTR) {
            continue;
        }
        if (n <= 0) {
            break;
        }
        have += static_cast<std::size_t>(n);
        std::size_t start = 0;
        for (std::size_t i = 0; i < have && !done; i++) {
            if (buf[i] != '\n') {
                continue;
            }
            if (skip) {
                skip = false;
                start = i + 1;
                continue;
            }
            // "start-end perms ...": two hex numbers.
            uintptr_t lo = 0;
            uintptr_t hi = 0;
            std::size_t k = start;
            auto hex = [&](uintptr_t& v) {
                for (; k < i; k++) {
                    char c = buf[k];
                    int d = c >= '0' && c <= '9'   ? c - '0'
                            : c >= 'a' && c <= 'f' ? c - 'a' + 10
                                                   : -1;
                    if (d < 0) {
                        break;
                    }
                    v = v * 16 + static_cast<uintptr_t>(d);
                }
            };
            hex(lo);
            if (k < i && buf[k] == '-') {
                k++;
                hex(hi);
                if (lo <= addr && addr < hi) {
                    found = prev_end;
                    done = true;
                } else if (hi <= addr) {
                    prev_end = hi;
                }
            }
            start = i + 1;
        }
        // Keep the part of a line that the next read completes; a line longer than the buffer is
        // not one of the mappings this needs, its rest is skipped.
        if (start < have && start > 0) {
            std::memmove(buf, buf + start, have - start);
            have -= start;
        } else if (start == have) {
            have = 0;
        } else if (have == sizeof buf) {
            have = 0;
            skip = true;
        }
    }
    ::close(fd);
    return found;
}

// main_lo returns the lowest address of the main thread's stack for its top and the soft limit:
// the higher of the limit's end and the end of the mapping below the stack plus the kernel's gap;
// 0 when neither is known.
uintptr_t main_lo(uintptr_t top, rlim_t limit) noexcept {
    uintptr_t lo = 0;
    if (limit != RLIM_INFINITY && limit < top) {
        lo = top - static_cast<uintptr_t>(limit);
    }
    uintptr_t below = below_mapping_end(top - 1);
    if (below != 0 && below + stack_guard_gap < top && below + stack_guard_gap > lo) {
        lo = below + stack_guard_gap;
    }
    return lo;
}

} // namespace

void main_stack_changed() noexcept {
    thread_stack* m = main_tstack;
    if (m == nullptr) {
        return;
    }
    rlimit rl{};
    if (getrlimit(RLIMIT_STACK, &rl) != 0) {
        return;
    }
    if (uintptr_t lo = main_lo(m->hi, rl.rlim_cur); lo != 0) {
        __atomic_store_n(&m->above, above_of(lo, m->hi), __ATOMIC_RELAXED);
        __atomic_store_n(&m->lo, lo, __ATOMIC_RELAXED);
    }
}

void start_threads() noexcept {
    // The main thread's stack: its soft limit becomes Go's 1 GiB (or the hard limit; a larger soft
    // limit stays), and its lowest address the higher of the limit's and the mapping's below it
    // (the comment at the top). Only a limit and a mapping that are both unknown leave the thread
    // without bounds.
    uintptr_t top = main_stack_top();
    rlim_t limit = set_main_stack_limit();
    if (uintptr_t lo = main_lo(top, limit); lo != 0) {
        tstack = {lo, top, above_of(lo, top)};
    } else {
        tstack = {0, top, 0};
    }
    main_tstack = &tstack;
    main_region = limit != RLIM_INFINITY && limit > main_stack_limit ? static_cast<uintptr_t>(limit)
                                                                     : main_stack_limit;
#if defined(GOCPP_OWN_SEGV)
    // The handler and the alternate stack also without bounds: a fault that is no overflow keeps
    // its old action either way, and the threads of thread_start have bounds of their own.
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
    if (stack_size > std::numeric_limits<std::size_t>::max() - page) {
        return EINVAL;
    }
    std::size_t size = (stack_size + page - 1) / page * page;
    if (auto min = static_cast<std::size_t>(PTHREAD_STACK_MIN); size < min) {
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

namespace {

// The thread-exit hook (tls_on_thread_exit): one pthread key whose destructor calls the function
// that the ending thread registered. The key's value is the address of the thread's own hook
// record (a thread_local, which is still there when the destructors of the keys run).
struct exit_hook {
    void (*fn)(void*);
    void* arg;
};
constinit thread_local exit_hook thook{};
constinit std::atomic<bool> hook_stopped{false};
pthread_key_t hook_key;
constinit std::atomic<int> hook_state{0}; // 0: no key, 1: being made, 2: made, 3: failed
constinit std::atomic<bool> hook_deleting{false};

void run_exit_hook(void* v) noexcept {
    auto* h = static_cast<exit_hook*>(v);
    if (hook_stopped.load(std::memory_order_acquire) || h->fn == nullptr) {
        return;
    }
    void (*fn)(void*) = h->fn;
    void* arg = h->arg;
    h->fn =
        nullptr; // a thread that is adopted again by fn's own thread-local users gets a new hook
    fn(arg);
}

bool hook_key_ready() noexcept {
    for (;;) {
        int st = hook_state.load(std::memory_order_acquire);
        if (st == 2) {
            return true;
        }
        if (st == 3) {
            return false;
        }
        int none = 0;
        if (st == 0 && hook_state.compare_exchange_strong(none, 1, std::memory_order_acq_rel)) {
            bool ok = pthread_key_create(&hook_key, run_exit_hook) == 0;
            hook_state.store(ok ? 2 : 3, std::memory_order_release);
            return ok;
        }
        sched_yield();
    }
}

} // namespace

void tls_on_thread_exit(void (*fn)(void*), void* arg) noexcept {
    if (hook_stopped.load(std::memory_order_acquire) || !hook_key_ready()) {
        return;
    }
    thook = exit_hook{fn, arg};
    pthread_setspecific(hook_key, &thook);
}

void tls_on_thread_exit_stop() noexcept {
    hook_stopped.store(true, std::memory_order_release);
    if (hook_state.load(std::memory_order_acquire) == 2 &&
        !hook_deleting.exchange(true, std::memory_order_acq_rel)) {
        pthread_key_delete(hook_key); // no destructor runs for the threads that have a value
    }
}

bool stack_bounds(uintptr_t& lo, uintptr_t& hi) noexcept {
    uintptr_t l = load_relaxed(tstack.lo); // the main thread's may change (main_stack_changed)
    if (l == 0) {
        return false;
    }
    lo = l;
    hi = tstack.hi;
    return true;
}

stack_overflow_hook set_stack_overflow_hook(stack_overflow_hook hook) noexcept {
    return overflow_hook.exchange(hook, std::memory_order_acq_rel);
}

int cpu_count() noexcept {
    cpu_set_t set;
    CPU_ZERO(&set);
    if (sched_getaffinity(0, sizeof set, &set) == 0) {
        int n = CPU_COUNT(&set);
        return n > 0 ? n : 1;
    }
    long n = sysconf(_SC_NPROCESSORS_ONLN);
    return n > 0 ? static_cast<int>(n) : 1;
}

void exit_now(int code) noexcept {
    race::finish(
        code); // -race: the reports that are still to be written, and "Found N data race(s)"
    ::_exit(code);
}

void no_core_dump() noexcept {
    prctl(PR_SET_DUMPABLE, 0);
}

} // namespace gp::os
