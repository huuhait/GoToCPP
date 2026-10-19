// The Linux OS layer (gocpp/os.hpp). This folder is the only place in the runtime that may
// include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include "linux.hpp"

#include <atomic>
#include <cerrno>
#include <csignal>
#include <cstdint>
#include <cstdio>
#include <cstring>

#include <dlfcn.h>
#include <malloc.h>
#include <poll.h>
#include <unistd.h>

#if defined(GOCPP_LSAN)
#include <sanitizer/lsan_interface.h>
#endif

// GOCPP_SAN_ALLOC: a sanitizer replaces malloc, and mallinfo2 knows nothing about its allocator.
#if defined(GOCPP_LSAN) || defined(__SANITIZE_THREAD__)
#define GOCPP_SAN_ALLOC 1
#elif defined(__has_feature)
#if __has_feature(thread_sanitizer)
#define GOCPP_SAN_ALLOC 1
#endif
#endif
#if defined(GOCPP_SAN_ALLOC)
#include <sanitizer/allocator_interface.h>
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

} // namespace

void start() noexcept {
    // A handler, not SIG_IGN: an ignored signal stays ignored in the programs that exec
    // starts, a handler goes back to the default.
    struct sigaction sa{};
    sa.sa_handler = on_sigpipe;
    sa.sa_flags = SA_RESTART;
    sigemptyset(&sa.sa_mask);
    sigaction(SIGPIPE, &sa, nullptr);
    start_threads();
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
    // One thread ends the program. When two goroutines call exit at once (a fatal error on one
    // while another one's os.Exit runs), the second waits here: the first one flushes, runs the
    // leak check (which must not run twice at the same time) and ends the process, with its code.
    static std::atomic<bool> exiting{false};
    if (exiting.exchange(true, std::memory_order_acq_rel)) {
        for (;;) {
            ::pause();
        }
    }
    std::fflush(nullptr);
#if defined(GOCPP_LSAN)
    // _exit skips the leak check that ASan runs at the end of the process.
    __lsan_do_leak_check();
#endif
    ::_exit(code);
}

mem_stats mem_info() noexcept {
    mem_stats m;
#if defined(GOCPP_SAN_ALLOC)
    // The sanitizer's allocator.
    m.gp_in_use = static_cast<uint64_t>(__sanitizer_get_current_allocated_bytes());
    m.gp_sys = static_cast<uint64_t>(__sanitizer_get_heap_size());
    m.gp_idle = static_cast<uint64_t>(__sanitizer_get_free_bytes());
#elif defined(__GLIBC__) && defined(__GLIBC_PREREQ)
#if __GLIBC_PREREQ(2, 33)
    struct mallinfo2 mi = ::mallinfo2();
    // arena: bytes from brk/mmap'd arenas, hblkhd: bytes in separately mmap'd blocks (big
    // allocations), uordblks: bytes in the allocations that are live, fordblks: bytes free.
    m.gp_in_use = static_cast<uint64_t>(mi.uordblks) + static_cast<uint64_t>(mi.hblkhd);
    m.gp_sys = static_cast<uint64_t>(mi.arena) + static_cast<uint64_t>(mi.hblkhd);
    m.gp_idle = static_cast<uint64_t>(mi.fordblks);
#endif
#endif
    return m;
}

void trim_memory() noexcept {
#if defined(__GLIBC__)
    ::malloc_trim(0);
#endif
}

std::vector<std::string> arguments(int argc, char** argv) {
    std::vector<std::string> args;
    for (int i = 0; i < argc && argv != nullptr && argv[i] != nullptr; i++) {
        args.emplace_back(argv[i]);
    }
    return args;
}

std::vector<std::string> process_arguments() {
    std::vector<std::string> args;
    std::FILE* f = std::fopen("/proc/self/cmdline", "rb");
    if (f == nullptr) {
        return args;
    }
    std::string cur;
    bool open = false;
    for (int c; (c = std::fgetc(f)) != EOF;) {
        if (c == 0) {
            args.push_back(std::move(cur));
            cur.clear();
            open = false;
        } else {
            cur.push_back(static_cast<char>(c));
            open = true;
        }
    }
    if (open) {
        args.push_back(std::move(cur));
    }
    std::fclose(f);
    return args;
}

void* library_ref(const void* addr) noexcept {
    Dl_info info;
    if (dladdr(addr, &info) == 0 || info.dli_fname == nullptr) {
        return nullptr;
    }
    void* self = dlopen(info.dli_fname, RTLD_NOW | RTLD_NOLOAD);
    if (self == nullptr) {
        return nullptr;
    }
    void* prog = dlopen(nullptr, RTLD_NOW);
    if (prog != nullptr) {
        dlclose(prog);
    }
    if (self == prog) {
        dlclose(self);
        return nullptr;
    }
    return self;
}

void library_unref(void* handle) noexcept {
    if (handle != nullptr) {
        dlclose(handle);
    }
}

std::vector<std::string> environment() {
    std::vector<std::string> env;
    for (char** e = environ; e != nullptr && *e != nullptr; e++) {
        env.emplace_back(*e);
    }
    return env;
}

} // namespace gp::os
