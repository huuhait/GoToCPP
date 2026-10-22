// The Linux OS layer (gocpp/os.hpp). This folder is the only place in the runtime that may
// include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include <gocpp/race.hpp>
#include "linux.hpp"

#include <atomic>
#include <cerrno>
#include <csignal>
#include <cstdint>
#include <cstdio>
#include <cstring>

#include <dirent.h>
#include <dlfcn.h>
#include <fcntl.h>
#include <malloc.h>
#include <poll.h>
#include <sys/syscall.h>
#include <sys/time.h>
#include <ctime>
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

// wait_readable waits until fd has bytes to read (for a non-blocking fd). It returns false on an
// error.
bool wait_readable(int fd) noexcept {
    for (;;) {
        pollfd p{fd, POLLIN, 0};
        int r = ::poll(&p, 1, -1);
        if (r > 0) {
            return true;
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
    race::init(); // -race: Go-shaped reports (gocpp/race.hpp)
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

namespace {
thread_local int last_write_errno = EIO;
} // namespace

const char* write_error() noexcept {
    switch (last_write_errno) {
    case EBADF:
        return "bad file descriptor";
    case ENOSPC:
        return "no space left on device";
    case EFBIG:
        return "file too large";
    case EINVAL:
        return "invalid argument";
    case EFAULT:
        return "bad address";
    case EPERM:
        return "operation not permitted";
    case EDQUOT:
        return "disk quota exceeded";
    default:
        return "input/output error";
    }
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
                last_write_errno = errno;
                return write_result::failed;
            }
            last_write_errno = errno;
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

write_result write_empty(int fd) noexcept {
    for (;;) {
        if (::write(fd, "", 0) >= 0) {
            return write_result::ok;
        }
        if (errno == EINTR) {
            continue;
        }
        last_write_errno = errno;
        return errno == EPIPE ? write_result::broken_pipe : write_result::failed;
    }
}

namespace {
thread_local int last_read_errno = EIO;
} // namespace

const char* read_error() noexcept {
    switch (last_read_errno) {
    case EBADF:
        return "bad file descriptor";
    case EISDIR:
        return "is a directory";
    case EINVAL:
        return "invalid argument";
    case EFAULT:
        return "bad address";
    default:
        return "input/output error";
    }
}

std::int64_t read(int fd, char* data, std::size_t n) noexcept {
    for (;;) {
        ssize_t r = ::read(fd, data, n);
        if (r >= 0) {
            return static_cast<std::int64_t>(r);
        }
        if (errno == EINTR) {
            continue;
        }
        if (errno == EAGAIN || errno == EWOULDBLOCK) {
            if (wait_readable(fd)) {
                continue;
            }
        }
        last_read_errno = errno;
        return -1;
    }
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
    race::finish(code); // -race: "Found N data race(s)"
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

// Linux rounds no wait up to a tick: nothing to ask for.
std::int64_t timer_margin_ns() noexcept {
    return 0;
}

void fine_timer(bool) noexcept {}

// No high-resolution timers to ask for (a timed wait is exact enough already): the runtime never
// makes a slot here.
bool precise_timers() noexcept {
    return false;
}

wait_slot* wait_slot_new() noexcept {
    return nullptr;
}

void wait_slot_close(wait_slot*) noexcept {}

bool wait_slot_open(wait_slot*) noexcept {
    return false;
}

void wait_slot_arm(wait_slot*) noexcept {}

void wait_slot_wait(wait_slot*, std::int64_t, std::int64_t) noexcept {}

void wait_slot_wake(wait_slot*) noexcept {}

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

namespace {
std::atomic<unsigned long> barrier_init_count{0};
std::atomic<bool> barrier_init_failing{false};
} // namespace

bool process_barrier_init() noexcept {
    barrier_init_count.fetch_add(1, std::memory_order_relaxed);
    if (barrier_init_failing.load(std::memory_order_relaxed)) {
        return false;
    }
#ifdef SYS_membarrier
    // MEMBARRIER_CMD_REGISTER_PRIVATE_EXPEDITED = 1 << 4
    return ::syscall(SYS_membarrier, 1 << 4, 0, 0) == 0;
#else
    return false;
#endif
}

namespace {
std::atomic<unsigned long> barrier_count{0};
std::atomic<testing::barrier_hook> before_barrier{nullptr};
} // namespace

void process_barrier() noexcept {
    if (auto hook = before_barrier.load(std::memory_order_acquire)) {
        hook();
    }
#ifdef SYS_membarrier
    // MEMBARRIER_CMD_PRIVATE_EXPEDITED = 1 << 3
    ::syscall(SYS_membarrier, 1 << 3, 0, 0);
#endif
    barrier_count.fetch_add(1, std::memory_order_release);
}

namespace testing {

unsigned long barrier_calls() noexcept {
    return barrier_count.load(std::memory_order_acquire);
}

barrier_hook set_barrier_hook(barrier_hook hook) noexcept {
    return before_barrier.exchange(hook, std::memory_order_acq_rel);
}

unsigned long barrier_init_calls() noexcept {
    return barrier_init_count.load(std::memory_order_relaxed);
}

void set_barrier_init_failing(bool on) noexcept {
    barrier_init_failing.store(on, std::memory_order_relaxed);
}

namespace {
int saved_stdin = -1;
void save_stdin() noexcept {
    if (saved_stdin < 0) {
        saved_stdin = ::dup(0);
    }
}
void on_alarm(int) {}
} // namespace

int stdin_pipe(bool nonblocking) noexcept {
    int fds[2];
    if (::pipe(fds) != 0) {
        return -1;
    }
    if (nonblocking) {
        ::fcntl(fds[0], F_SETFL, ::fcntl(fds[0], F_GETFL) | O_NONBLOCK);
    }
    save_stdin();
    ::dup2(fds[0], 0);
    ::close(fds[0]);
    return fds[1];
}

bool stdin_directory() noexcept {
    int d = ::open(".", O_RDONLY);
    if (d < 0) {
        return false;
    }
    save_stdin();
    ::dup2(d, 0);
    ::close(d);
    return true;
}

bool stdin_closed() noexcept {
    save_stdin();
    return ::close(0) == 0;
}

void restore_stdin() noexcept {
    if (saved_stdin >= 0) {
        ::dup2(saved_stdin, 0);
        ::close(saved_stdin);
        saved_stdin = -1;
    }
}

void fd_write(int fd, const char* data, std::size_t n) noexcept {
    while (n > 0) {
        ssize_t w = ::write(fd, data, n);
        if (w <= 0) {
            return;
        }
        data += w;
        n -= static_cast<std::size_t>(w);
    }
}

void fd_close(int fd) noexcept {
    ::close(fd);
}

bool interrupt_in(int ms) noexcept {
    struct sigaction sa{};
    sa.sa_handler = on_alarm;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0; // no SA_RESTART: the read fails with EINTR
    if (sigaction(SIGALRM, &sa, nullptr) != 0) {
        return false;
    }
    struct sigevent sev{};
    sev.sigev_notify = SIGEV_THREAD_ID;
    sev.sigev_signo = SIGALRM;
    sev._sigev_un._tid = ::gettid();
    timer_t id;
    if (timer_create(CLOCK_MONOTONIC, &sev, &id) != 0) {
        return false;
    }
    struct itimerspec its{};
    its.it_value.tv_sec = ms / 1000;
    its.it_value.tv_nsec = static_cast<long>(ms % 1000) * 1000000;
    return timer_settime(id, 0, &its, nullptr) == 0;
}

void force_fallback(bool) noexcept {}

wake_pause_hook set_wake_pause_hook(wake_pause_hook) noexcept {
    return nullptr;
}

std::int64_t cpu_time_ns() noexcept {
    struct timespec ts{};
    if (clock_gettime(CLOCK_PROCESS_CPUTIME_ID, &ts) != 0) {
        return 0;
    }
    return static_cast<std::int64_t>(ts.tv_sec) * 1000000000 + ts.tv_nsec;
}

std::int64_t open_handles() noexcept {
    // The descriptors of /proc/self/fd, less the one that listing them opens.
    DIR* dir = opendir("/proc/self/fd");
    if (dir == nullptr) {
        return 0;
    }
    std::int64_t n = 0;
    while (readdir(dir) != nullptr) {
        n++;
    }
    closedir(dir);
    return n > 3 ? n - 3 : 0; // ".", ".." and the descriptor of the listing
}

} // namespace testing

} // namespace gp::os
