// The Linux OS layer (gocpp/os.hpp). This folder is the only place in the runtime that may
// include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>

#include <cerrno>
#include <csignal>
#include <cstdint>
#include <cstdio>
#include <cstring>

#include <poll.h>
#include <sys/auxv.h>
#include <sys/resource.h>
#include <ucontext.h>
#include <unistd.h>

#if defined(__SANITIZE_ADDRESS__)
#define GOCPP_LSAN 1
#elif defined(__has_feature)
#if __has_feature(address_sanitizer)
#define GOCPP_LSAN 1
#endif
#endif
#if defined(GOCPP_LSAN)
#include <sanitizer/lsan_interface.h>
#endif

// GOCPP_OWN_SEGV: the runtime installs its SIGSEGV handler (stack overflow, below). Not with a
// sanitizer, which has its own handler and its own report of a stack overflow.
#if !defined(GOCPP_LSAN) && !defined(__SANITIZE_THREAD__)
#define GOCPP_OWN_SEGV 1
#if defined(__has_feature)
#if __has_feature(thread_sanitizer) || __has_feature(memory_sanitizer)
#undef GOCPP_OWN_SEGV
#endif
#endif
#endif

extern "C" char** environ;

namespace gp::os {

namespace {

void on_sigpipe(int) {}

// wait_writable waits until fd can take more bytes (for a non-blocking fd). It returns false
// on an error.
bool wait_writable(int fd) noexcept {
    for (;;) {
        pollfd p{fd, POLLOUT, 0};
        int r = ::poll(&p, 1, -1);
        if (r > 0) {
            return true; // writable, or an error that the next write reports
        }
        if (r < 0 && errno != EINTR) {
            return false;
        }
    }
}

#if defined(GOCPP_OWN_SEGV)

// --- stack overflow ---
//
// Go's goroutine stacks grow, up to 1 GB, and a deeper recursion ends with "runtime: goroutine
// stack exceeds 1000000000-byte limit" and "fatal error: stack overflow" (exit code 2). A gocpp
// goroutine runs on a fixed C++ stack (8 MiB for the main thread, the ulimit -s), and a
// recursion that is too deep hits the end of it: a SIGSEGV, which the kernel would turn into
// a crash (exit code 139). So start installs a SIGSEGV handler, on a stack of its own
// (sigaltstack: the thread's stack is full), that tells a stack overflow from other memory
// faults and prints Go's lines for it. Any other fault gets the action that was there before
// (the default: the crash), as if the handler weren't there. Phase 08 (step 8.1) owns the
// stacks: threads of their own need their own alternate stack and bounds.
//
// A fault is a stack overflow when its address is near the lowest address the main thread's
// stack may grow to, and the stack pointer is near it too. The kernel grows the stack mapping
// down to its top minus RLIMIT_STACK, and the top is exact: it ends just after the program's
// file name, the last string that exec puts on the stack (getauxval(AT_EXECFN)). Without a
// limit (ulimit -s unlimited) there is no such address, and every fault keeps the old action.
//
// The handler only formats numbers and calls write and _exit, which are async-signal-safe: no
// stack trace (Go prints one), because walking the stack and reading the program's file from a
// signal handler isn't safe (the fault may be inside malloc).

// The window around the end of the stack where a fault is an overflow. overflow_below: a frame
// can skip further below the end (a big local array without stack probes). overflow_above: the
// kernel also keeps a gap (stack_guard_gap, 1 MiB) between the stack and a mapping below it,
// so with a mapping just below the end, the stack stops up to that much higher; at most a
// quarter of a smaller stack.
constexpr uintptr_t overflow_below = uintptr_t(1) << 20;
constexpr uintptr_t overflow_above_max = (uintptr_t(1) << 20) + (uintptr_t(64) << 10);

// The main thread's stack: stack_top is its top (the end of the stack mapping), stack_end the
// lowest address it may grow to; stack_end is 0 when the stack has no limit (no detection
// then).
constinit uintptr_t stack_top = 0;
constinit uintptr_t stack_end = 0;
constinit uintptr_t overflow_above = 0;

// The handler's own stack. 64 KiB: the handler needs little, and the signal frame of a CPU with
// large vector registers (AVX-512) takes a few KiB.
alignas(64) constinit char alt_stack[64 * 1024]{};

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

bool near_end(uintptr_t a) noexcept {
    return a + overflow_below >= stack_end && a < stack_end + overflow_above;
}

// put_hex appends "0x..." for v at p and returns the end (at most 18 bytes).
char* put_hex(char* p, uintptr_t v) noexcept {
    char digits[16];
    int n = 0;
    do {
        digits[n++] = "0123456789abcdef"[v % 16];
        v /= 16;
    } while (v != 0);
    *p++ = '0';
    *p++ = 'x';
    while (n > 0) {
        *p++ = digits[--n];
    }
    return p;
}

char* put_str(char* p, const char* s) noexcept {
    while (*s != '\0') {
        *p++ = *s++;
    }
    return p;
}

void on_sigsegv(int sig, siginfo_t* info, void* ctx) {
    auto addr = reinterpret_cast<uintptr_t>(info->si_addr);
    uintptr_t sp = fault_sp(ctx);
    // si_code > 0: a fault of the CPU, not a kill(2) of SIGSEGV.
    if (info->si_code > 0 && stack_end != 0 && near_end(addr) && (sp == 0 || near_end(sp))) {
        // Go's lines; the stack in the second one is the main thread's [end, top).
        char buf[256];
        char* p = put_str(buf, "runtime: goroutine stack exceeds 1000000000-byte limit\n"
                               "runtime: sp=");
        p = put_hex(p, sp != 0 ? sp : addr);
        p = put_str(p, " stack=[");
        p = put_hex(p, stack_end);
        p = put_str(p, ", ");
        p = put_hex(p, stack_top);
        p = put_str(p, "]\nfatal error: stack overflow\n");
        // Nothing to do when the write fails; the result is kept for _FORTIFY_SOURCE, which
        // warns when it is ignored.
        [[maybe_unused]] ssize_t w = ::write(stderr_fd, buf, static_cast<std::size_t>(p - buf));
        ::_exit(2);
    }
    // Any other fault: the old action. Returning runs the faulting instruction again, which
    // faults again and gets it; a SIGSEGV sent with kill must be raised again (it is blocked
    // until the handler returns).
    sigaction(sig, &old_segv, nullptr);
    if (info->si_code <= 0) {
        ::raise(sig);
    }
}

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
    char here = 0;
    uintptr_t end = reinterpret_cast<uintptr_t>(&here);
    // A string that isn't above this frame isn't on the stack (an environment that C++ code
    // changed before run_main, with setenv).
    if (last != nullptr && reinterpret_cast<uintptr_t>(last) > end) {
        end = reinterpret_cast<uintptr_t>(last) + std::strlen(last) + 1 + sizeof(void*);
    }
    return (end + page - 1) & ~(page - 1);
}

void catch_stack_overflow() noexcept {
    rlimit rl{};
    stack_top = main_stack_top();
    if (getrlimit(RLIMIT_STACK, &rl) != 0 || rl.rlim_cur == RLIM_INFINITY ||
        rl.rlim_cur >= stack_top || rl.rlim_cur < (uintptr_t(64) << 10)) {
        return; // no limit, or a strange one
    }
    auto limit = static_cast<uintptr_t>(rl.rlim_cur);
    overflow_above = limit / 4 < overflow_above_max ? limit / 4 : overflow_above_max;
    stack_t ss{};
    ss.ss_sp = alt_stack;
    ss.ss_size = sizeof alt_stack;
    if (sigaltstack(&ss, nullptr) != 0) {
        return;
    }
    struct sigaction sa{};
    sa.sa_sigaction = on_sigsegv;
    sa.sa_flags = SA_SIGINFO | SA_ONSTACK;
    sigemptyset(&sa.sa_mask);
    if (sigaction(SIGSEGV, &sa, &old_segv) == 0) {
        stack_end = stack_top - limit;
    }
}

#endif // GOCPP_OWN_SEGV

} // namespace

void start() noexcept {
    // A handler, not SIG_IGN: an ignored signal stays ignored in the programs that exec
    // starts, a handler goes back to the default.
    struct sigaction sa{};
    sa.sa_handler = on_sigpipe;
    sa.sa_flags = SA_RESTART;
    sigemptyset(&sa.sa_mask);
    sigaction(SIGPIPE, &sa, nullptr);
#if defined(GOCPP_OWN_SEGV)
    catch_stack_overflow();
#endif
}

void die_of_sigpipe() noexcept {
    std::signal(SIGPIPE, SIG_DFL);
    sigset_t set;
    sigemptyset(&set);
    sigaddset(&set, SIGPIPE);
    pthread_sigmask(SIG_UNBLOCK, &set, nullptr);
    std::raise(SIGPIPE);
    ::_exit(2); // not reached: like Go when the signal doesn't kill
}

write_result write(int fd, const char* data, std::size_t n) noexcept {
    while (n > 0) {
        ssize_t w = ::write(fd, data, n);
        if (w < 0) {
            if (errno == EINTR) {
                continue;
            }
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                if (wait_writable(fd)) {
                    continue;
                }
                return write_result::failed;
            }
            return errno == EPIPE ? write_result::broken_pipe : write_result::failed;
        }
        if (w == 0) {
            return write_result::failed; // no progress: don't loop forever
        }
        data += w;
        n -= static_cast<std::size_t>(w);
    }
    return write_result::ok;
}

void exit(int code) noexcept {
    std::fflush(nullptr);
#if defined(GOCPP_LSAN)
    // _exit skips the leak check that ASan runs at the end of the process.
    __lsan_do_leak_check();
#endif
    ::_exit(code);
}

std::vector<std::string> arguments(int argc, char** argv) {
    std::vector<std::string> args;
    for (int i = 0; i < argc && argv != nullptr && argv[i] != nullptr; i++) {
        args.emplace_back(argv[i]);
    }
    return args;
}

std::vector<std::string> environment() {
    std::vector<std::string> env;
    for (char** e = environ; e != nullptr && *e != nullptr; e++) {
        env.emplace_back(*e);
    }
    return env;
}

} // namespace gp::os
