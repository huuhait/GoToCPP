// The system calls of package syscall for Linux (gocpp/syscall.hpp, Phase 09 step 9.6, ADR-067).
// This folder is the only place in the runtime that may include OS headers. Every function
// returns the errno of a failure and repeats a call that a signal interrupted.
#include <gocpp/nofortify.h>
#include <gocpp/main.hpp>
#include <gocpp/os.hpp>
#include <gocpp/print.hpp>
#include <gocpp/syscall.hpp>

#include <atomic>
#include <cerrno>
#include <chrono>
#include <climits>
#include <cstdlib>
#include <cstring>
#include <string>
#include <thread>
#include <vector>

#include <dirent.h>
#include <fcntl.h>
#include <poll.h>
#include <sys/eventfd.h>
#include <sys/resource.h>
#include <sys/time.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <sys/utsname.h>
#include <unistd.h>

#include <csignal>

namespace gp::syscall {

namespace {

// Go's os cuts one read or write at 1<<30 bytes (maxRW) so that a huge slice never reaches a
// system call whose count the kernel would truncate to 2 GiB.
constexpr int64_t max_rw = 1 << 30;

// The path as a C string: the .gp wrapper checked that it has no NUL byte.
std::string c_path(const gp::string& s) {
    return std::string(s.gp_view());
}

// retry runs call (which returns -1 and sets errno on failure) until a signal does not
// interrupt it, and returns its result.
template <typename F>
auto retry(F&& call) {
    for (;;) {
        auto r = call();
        if (r == static_cast<decltype(r)>(-1) && errno == EINTR) {
            continue;
        }
        return r;
    }
}

int64_t fail() {
    return errno;
}

// stat_out writes a struct stat into the 16 numbers of the caller's slice.
int64_t stat_out(const struct stat& st, const gp::slice<uint64_t>& out) {
    if (out.gp_len() < 16) {
        return EINVAL;
    }
    uint64_t* o = out.gp_data();
    o[0] = static_cast<uint64_t>(st.st_dev);
    o[1] = static_cast<uint64_t>(st.st_ino);
    o[2] = static_cast<uint64_t>(st.st_nlink);
    o[3] = static_cast<uint64_t>(st.st_mode);
    o[4] = static_cast<uint64_t>(st.st_uid);
    o[5] = static_cast<uint64_t>(st.st_gid);
    o[6] = static_cast<uint64_t>(st.st_rdev);
    o[7] = static_cast<uint64_t>(st.st_size);
    o[8] = static_cast<uint64_t>(st.st_blksize);
    o[9] = static_cast<uint64_t>(st.st_blocks);
    o[10] = static_cast<uint64_t>(st.st_atim.tv_sec);
    o[11] = static_cast<uint64_t>(st.st_atim.tv_nsec);
    o[12] = static_cast<uint64_t>(st.st_mtim.tv_sec);
    o[13] = static_cast<uint64_t>(st.st_mtim.tv_nsec);
    o[14] = static_cast<uint64_t>(st.st_ctim.tv_sec);
    o[15] = static_cast<uint64_t>(st.st_ctim.tv_nsec);
    return 0;
}

} // namespace

gp::slice<gp::string> runtime_envs() {
    const auto& v = gp::envs();
    auto s = gp::make_slice<gp::string>(static_cast<int64_t>(v.size()));
    for (std::size_t i = 0; i < v.size(); i++) {
        s[static_cast<int64_t>(i)] = v[i];
    }
    return s;
}

gp::slice<gp::string> runtime_args() {
    const auto& v = gp::args();
    auto s = gp::make_slice<gp::string>(static_cast<int64_t>(v.size()));
    for (std::size_t i = 0; i < v.size(); i++) {
        s[static_cast<int64_t>(i)] = v[i];
    }
    return s;
}

uint64_t runtime_rand() {
    // One generator per thread, seeded from the clock, the thread and a counter (xorshift64*).
    thread_local uint64_t state = 0;
    static std::atomic<uint64_t> counter{0};
    if (state == 0) {
        uint64_t seed =
            static_cast<uint64_t>(std::chrono::steady_clock::now().time_since_epoch().count());
        seed ^= std::hash<std::thread::id>{}(std::this_thread::get_id()) * 0x9E3779B97F4A7C15ull;
        seed += counter.fetch_add(1) * 0xBF58476D1CE4E5B9ull;
        state = seed | 1;
    }
    state ^= state >> 12;
    state ^= state << 25;
    state ^= state >> 27;
    return state * 0x2545F4914F6CDD1Dull;
}

void runtime_setenv(const gp::string& key, const gp::string& value) {
    ::setenv(c_path(key).c_str(), c_path(value).c_str(), 1);
}

void runtime_unsetenv(const gp::string& key) {
    ::unsetenv(c_path(key).c_str());
}

std::tuple<int64_t, int64_t> sys_open(const gp::string& path, int64_t flags, int64_t perm) {
    std::string p = c_path(path);
    int fd = retry([&] {
        return ::open(p.c_str(), static_cast<int>(flags) | O_CLOEXEC, static_cast<mode_t>(perm));
    });
    if (fd < 0) {
        return {-1, fail()};
    }
    return {fd, 0};
}

int64_t sys_close(int64_t fd) {
    // Not repeated: after close the descriptor is gone, whatever the result (EINTR included).
    if (::close(static_cast<int>(fd)) != 0) {
        return errno == EINTR ? 0 : errno;
    }
    return 0;
}

std::tuple<int64_t, int64_t> sys_read(int64_t fd, const gp::slice<uint8_t>& p) {
    int64_t n = p.gp_len() > max_rw ? max_rw : p.gp_len();
    ssize_t r = retry(
        [&] { return ::read(static_cast<int>(fd), p.gp_data(), static_cast<std::size_t>(n)); });
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

std::tuple<int64_t, int64_t> sys_write(int64_t fd, const gp::slice<uint8_t>& p) {
    if (fd == 1 || fd == 2) {
        // What C or C++ code printf'd comes first, as for fmt.Print and the builtin print.
        gp::detail::flush_c_stdio();
    }
    int64_t n = p.gp_len() > max_rw ? max_rw : p.gp_len();
    ssize_t r = retry(
        [&] { return ::write(static_cast<int>(fd), p.gp_data(), static_cast<std::size_t>(n)); });
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

std::tuple<int64_t, int64_t> sys_pread(int64_t fd, const gp::slice<uint8_t>& p, int64_t offset) {
    int64_t n = p.gp_len() > max_rw ? max_rw : p.gp_len();
    ssize_t r = retry([&] {
        return ::pread(static_cast<int>(fd), p.gp_data(), static_cast<std::size_t>(n),
                       static_cast<off_t>(offset));
    });
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

std::tuple<int64_t, int64_t> sys_pwrite(int64_t fd, const gp::slice<uint8_t>& p, int64_t offset) {
    int64_t n = p.gp_len() > max_rw ? max_rw : p.gp_len();
    ssize_t r = retry([&] {
        return ::pwrite(static_cast<int>(fd), p.gp_data(), static_cast<std::size_t>(n),
                        static_cast<off_t>(offset));
    });
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

std::tuple<int64_t, int64_t> sys_seek(int64_t fd, int64_t offset, int64_t whence) {
    off_t r = ::lseek(static_cast<int>(fd), static_cast<off_t>(offset), static_cast<int>(whence));
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

int64_t sys_fstat(int64_t fd, const gp::slice<uint64_t>& out) {
    struct stat st;
    if (retry([&] { return ::fstat(static_cast<int>(fd), &st); }) != 0) {
        return fail();
    }
    return stat_out(st, out);
}

int64_t sys_stat(const gp::string& path, const gp::slice<uint64_t>& out) {
    std::string p = c_path(path);
    struct stat st;
    if (retry([&] { return ::stat(p.c_str(), &st); }) != 0) {
        return fail();
    }
    return stat_out(st, out);
}

int64_t sys_lstat(const gp::string& path, const gp::slice<uint64_t>& out) {
    std::string p = c_path(path);
    struct stat st;
    if (retry([&] { return ::lstat(p.c_str(), &st); }) != 0) {
        return fail();
    }
    return stat_out(st, out);
}

int64_t sys_ftruncate(int64_t fd, int64_t length) {
    return retry([&] { return ::ftruncate(static_cast<int>(fd), static_cast<off_t>(length)); }) != 0
               ? fail()
               : 0;
}

int64_t sys_truncate(const gp::string& path, int64_t length) {
    std::string p = c_path(path);
    return retry([&] { return ::truncate(p.c_str(), static_cast<off_t>(length)); }) != 0 ? fail()
                                                                                         : 0;
}

int64_t sys_fsync(int64_t fd) {
    return retry([&] { return ::fsync(static_cast<int>(fd)); }) != 0 ? fail() : 0;
}

int64_t sys_fchmod(int64_t fd, int64_t mode) {
    return retry([&] { return ::fchmod(static_cast<int>(fd), static_cast<mode_t>(mode)); }) != 0
               ? fail()
               : 0;
}

int64_t sys_chmod(const gp::string& path, int64_t mode) {
    std::string p = c_path(path);
    return retry([&] { return ::chmod(p.c_str(), static_cast<mode_t>(mode)); }) != 0 ? fail() : 0;
}

int64_t sys_fchown(int64_t fd, int64_t uid, int64_t gid) {
    return retry([&] {
               return ::fchown(static_cast<int>(fd), static_cast<uid_t>(uid),
                               static_cast<gid_t>(gid));
           }) != 0
               ? fail()
               : 0;
}

int64_t sys_chown(const gp::string& path, int64_t uid, int64_t gid) {
    std::string p = c_path(path);
    return retry([&] {
               return ::chown(p.c_str(), static_cast<uid_t>(uid), static_cast<gid_t>(gid));
           }) != 0
               ? fail()
               : 0;
}

int64_t sys_lchown(const gp::string& path, int64_t uid, int64_t gid) {
    std::string p = c_path(path);
    return retry([&] {
               return ::lchown(p.c_str(), static_cast<uid_t>(uid), static_cast<gid_t>(gid));
           }) != 0
               ? fail()
               : 0;
}

int64_t sys_mkdir(const gp::string& path, int64_t mode) {
    std::string p = c_path(path);
    return retry([&] { return ::mkdir(p.c_str(), static_cast<mode_t>(mode)); }) != 0 ? fail() : 0;
}

int64_t sys_mknod(const gp::string& path, int64_t mode, int64_t dev) {
    std::string p = c_path(path);
    return retry([&] {
               return ::mknod(p.c_str(), static_cast<mode_t>(mode), static_cast<dev_t>(dev));
           }) != 0
               ? fail()
               : 0;
}

std::tuple<int64_t, int64_t, int64_t> sys_getrlimit(int64_t resource) {
    struct rlimit rl;
    if (::syscall(SYS_prlimit64, 0, static_cast<int>(resource), nullptr, &rl) != 0) {
        return {0, 0, fail()};
    }
    return {static_cast<int64_t>(rl.rlim_cur), static_cast<int64_t>(rl.rlim_max), 0};
}

int64_t sys_setrlimit(int64_t resource, int64_t cur, int64_t max) {
    struct rlimit rl;
    rl.rlim_cur = static_cast<rlim_t>(cur);
    rl.rlim_max = static_cast<rlim_t>(max);
    return ::syscall(SYS_prlimit64, 0, static_cast<int>(resource), &rl, nullptr) != 0 ? fail() : 0;
}

int64_t sys_rmdir(const gp::string& path) {
    std::string p = c_path(path);
    return retry([&] { return ::rmdir(p.c_str()); }) != 0 ? fail() : 0;
}

int64_t sys_unlink(const gp::string& path) {
    std::string p = c_path(path);
    return retry([&] { return ::unlink(p.c_str()); }) != 0 ? fail() : 0;
}

int64_t sys_rename(const gp::string& from, const gp::string& to) {
    std::string a = c_path(from);
    std::string b = c_path(to);
    return retry([&] { return ::rename(a.c_str(), b.c_str()); }) != 0 ? fail() : 0;
}

int64_t sys_link(const gp::string& oldpath, const gp::string& newpath) {
    std::string a = c_path(oldpath);
    std::string b = c_path(newpath);
    return retry([&] { return ::link(a.c_str(), b.c_str()); }) != 0 ? fail() : 0;
}

int64_t sys_symlink(const gp::string& oldpath, const gp::string& newpath) {
    std::string a = c_path(oldpath);
    std::string b = c_path(newpath);
    return retry([&] { return ::symlink(a.c_str(), b.c_str()); }) != 0 ? fail() : 0;
}

std::tuple<int64_t, int64_t> sys_readlink(const gp::string& path, const gp::slice<uint8_t>& buf) {
    std::string p = c_path(path);
    ssize_t r = retry([&] {
        return ::readlink(p.c_str(), reinterpret_cast<char*>(buf.gp_data()),
                          static_cast<std::size_t>(buf.gp_len()));
    });
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

int64_t sys_chdir(const gp::string& path) {
    std::string p = c_path(path);
    return retry([&] { return ::chdir(p.c_str()); }) != 0 ? fail() : 0;
}

int64_t sys_fchdir(int64_t fd) {
    return retry([&] { return ::fchdir(static_cast<int>(fd)); }) != 0 ? fail() : 0;
}

std::tuple<int64_t, int64_t> sys_getcwd(const gp::slice<uint8_t>& buf) {
    // The system call itself (not the C library's getcwd): it returns the length with the NUL.
    long r = retry([&] {
        return ::syscall(SYS_getcwd, buf.gp_data(), static_cast<std::size_t>(buf.gp_len()));
    });
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

int64_t sys_pipe2(const gp::slice<int64_t>& p, int64_t flags) {
    if (p.gp_len() < 2) {
        return EINVAL;
    }
    int fds[2];
    if (retry([&] { return ::pipe2(fds, static_cast<int>(flags)); }) != 0) {
        return fail();
    }
    p.gp_data()[0] = fds[0];
    p.gp_data()[1] = fds[1];
    return 0;
}

std::tuple<int64_t, int64_t> sys_getdents(int64_t fd, const gp::slice<uint8_t>& buf) {
    long r = retry([&] {
        return ::syscall(SYS_getdents64, static_cast<int>(fd), buf.gp_data(),
                         static_cast<std::size_t>(buf.gp_len()));
    });
    if (r < 0) {
        return {0, fail()};
    }
    return {r, 0};
}

int64_t sys_utimensat(const gp::string& path, int64_t asec, int64_t ansec, int64_t msec,
                      int64_t mnsec) {
    std::string p = c_path(path);
    struct timespec ts[2];
    ts[0].tv_sec = static_cast<time_t>(asec);
    ts[0].tv_nsec = static_cast<long>(ansec);
    ts[1].tv_sec = static_cast<time_t>(msec);
    ts[1].tv_nsec = static_cast<long>(mnsec);
    return retry([&] { return ::utimensat(AT_FDCWD, p.c_str(), ts, 0); }) != 0 ? fail() : 0;
}

std::tuple<int64_t, int64_t> sys_dup(int64_t fd) {
    int r = retry([&] { return ::fcntl(static_cast<int>(fd), F_DUPFD_CLOEXEC, 0); });
    if (r < 0) {
        return {-1, fail()};
    }
    return {r, 0};
}

std::tuple<int64_t, int64_t> sys_wake_new() {
    int fd = ::eventfd(0, EFD_CLOEXEC | EFD_NONBLOCK);
    if (fd < 0) {
        return {-1, fail()};
    }
    return {fd, 0};
}

void sys_wake_signal(int64_t fd) {
    uint64_t one = 1;
    // The counter cannot overflow in practice (2^64-2 signals); a failed write is a closed fd.
    ssize_t r = retry([&] { return ::write(static_cast<int>(fd), &one, sizeof one); });
    (void)r;
}

void sys_wake_clear(int64_t fd) {
    uint64_t v;
    ssize_t r = retry([&] { return ::read(static_cast<int>(fd), &v, sizeof v); });
    (void)r;
}

std::tuple<int64_t, int64_t> sys_poll_wait(int64_t fd, int64_t wakefd, bool write,
                                           int64_t timeout_ns) {
    pollfd p[2] = {{static_cast<int>(fd), static_cast<short>(write ? POLLOUT : POLLIN), 0},
                   {static_cast<int>(wakefd), POLLIN, 0}};
    // A negative wakefd is ignored by poll. A timeout beyond about 13 days is cut (the caller's
    // loop asks again), so that the end time cannot overflow.
    constexpr int64_t max_timeout_ns = int64_t{1} << 50;
    if (timeout_ns > max_timeout_ns) {
        timeout_ns = max_timeout_ns;
    }
    auto now = [] { return std::chrono::steady_clock::now(); };
    auto end = now() + std::chrono::nanoseconds(timeout_ns < 0 ? 0 : timeout_ns);
    for (;;) {
        timespec ts{};
        timespec* tsp = nullptr;
        if (timeout_ns >= 0) {
            auto left = std::chrono::duration_cast<std::chrono::nanoseconds>(end - now()).count();
            if (left < 0) {
                left = 0;
            }
            ts.tv_sec = static_cast<time_t>(left / 1000000000);
            ts.tv_nsec = static_cast<long>(left % 1000000000);
            tsp = &ts;
        }
        int r = ::ppoll(p, 2, tsp, nullptr);
        if (r > 0) {
            // The wake descriptor wins: a Close or a new deadline is seen even when the file is
            // ready too (the caller looks at its state and polls again).
            return {wakefd >= 0 && p[1].revents != 0 ? poll_woken : poll_ready, 0};
        }
        if (r == 0) {
            return {poll_timeout, 0};
        }
        if (errno != EINTR) {
            return {0, errno};
        }
    }
}

int64_t sys_dup3(int64_t oldfd, int64_t newfd, int64_t flags) {
    return retry([&] {
               return ::dup3(static_cast<int>(oldfd), static_cast<int>(newfd),
                             static_cast<int>(flags));
           }) < 0
               ? fail()
               : 0;
}

int64_t sys_umask(int64_t mask) {
    return ::umask(static_cast<mode_t>(mask));
}

int64_t sys_set_nonblock(int64_t fd, bool nonblocking) {
    int fl = ::fcntl(static_cast<int>(fd), F_GETFL);
    if (fl < 0) {
        return fail();
    }
    int want = nonblocking ? (fl | O_NONBLOCK) : (fl & ~O_NONBLOCK);
    if (want != fl && ::fcntl(static_cast<int>(fd), F_SETFL, want) < 0) {
        return fail();
    }
    return 0;
}

int64_t sys_set_cloexec(int64_t fd) {
    int fl = ::fcntl(static_cast<int>(fd), F_GETFD);
    if (fl < 0) {
        return fail();
    }
    if (::fcntl(static_cast<int>(fd), F_SETFD, fl | FD_CLOEXEC) < 0) {
        return fail();
    }
    return 0;
}

int64_t sys_getpid() {
    return ::getpid();
}
int64_t sys_getppid() {
    return ::getppid();
}
int64_t sys_getuid() {
    return ::getuid();
}
int64_t sys_geteuid() {
    return ::geteuid();
}
int64_t sys_getgid() {
    return ::getgid();
}
int64_t sys_getegid() {
    return ::getegid();
}
int64_t sys_getpagesize() {
    return ::getpagesize();
}

std::tuple<int64_t, int64_t> sys_getgroups(const gp::slice<int64_t>& out) {
    int64_t cap = out.gp_len();
    if (cap == 0) {
        int n = ::getgroups(0, nullptr);
        if (n < 0) {
            return {0, fail()};
        }
        return {n, 0};
    }
    std::vector<gid_t> g(static_cast<std::size_t>(cap));
    int n = ::getgroups(static_cast<int>(cap), g.data());
    if (n < 0) {
        return {0, fail()};
    }
    for (int i = 0; i < n; i++) {
        out.gp_data()[i] = g[static_cast<std::size_t>(i)];
    }
    return {n, 0};
}

int64_t sys_kill(int64_t pid, int64_t sig) {
    return ::kill(static_cast<pid_t>(pid), static_cast<int>(sig)) != 0 ? fail() : 0;
}

std::tuple<int64_t, int64_t> sys_wait4(int64_t pid, int64_t options,
                                       const gp::slice<int64_t>& out) {
    if (out.gp_len() < 19) {
        return {-1, EINVAL};
    }
    int status = 0;
    struct rusage ru;
    std::memset(&ru, 0, sizeof ru);
    pid_t r = retry(
        [&] { return ::wait4(static_cast<pid_t>(pid), &status, static_cast<int>(options), &ru); });
    if (r < 0) {
        return {-1, fail()};
    }
    int64_t* o = out.gp_data();
    o[0] = status;
    o[1] = ru.ru_utime.tv_sec;
    o[2] = ru.ru_utime.tv_usec;
    o[3] = ru.ru_stime.tv_sec;
    o[4] = ru.ru_stime.tv_usec;
    o[5] = ru.ru_maxrss;
    o[6] = ru.ru_ixrss;
    o[7] = ru.ru_idrss;
    o[8] = ru.ru_isrss;
    o[9] = ru.ru_minflt;
    o[10] = ru.ru_majflt;
    o[11] = ru.ru_nswap;
    o[12] = ru.ru_inblock;
    o[13] = ru.ru_oublock;
    o[14] = ru.ru_msgsnd;
    o[15] = ru.ru_msgrcv;
    o[16] = ru.ru_nsignals;
    o[17] = ru.ru_nvcsw;
    o[18] = ru.ru_nivcsw;
    return {r, 0};
}

std::tuple<int64_t, int64_t> sys_fcntl_getfl(int64_t fd) {
    int fl = ::fcntl(static_cast<int>(fd), F_GETFL);
    if (fl < 0) {
        return {0, fail()};
    }
    return {fl, 0};
}

void sys_sigpipe() {
    gp::os::die_of_sigpipe();
}

// The test seams for the EINTR rule (runtime/test/syscall_test.cpp) are built only by the runtime's
// own tests (they define GOCPP_RUNTIME_TEST): the program's runtime library does not have them.
#ifdef GOCPP_RUNTIME_TEST
namespace {
std::atomic<int> selftest_runs{0};
std::atomic<int> selftest_wfd{-1};
void selftest_handler(int) {
    selftest_runs.fetch_add(1);
    int e = errno;
    char c = 'x';
    int fd = selftest_wfd.load();
    if (fd >= 0) {
        [[maybe_unused]] ssize_t r = ::write(fd, &c, 1);
    }
    errno = e;
}
} // namespace

std::tuple<int64_t, int64_t> sys_selftest_retry() {
    int fds[2];
    if (::pipe(fds) != 0) {
        return {0, fail()};
    }
    selftest_runs.store(0);
    selftest_wfd.store(fds[1]);
    struct sigaction sa{};
    struct sigaction old{};
    sa.sa_handler = selftest_handler;
    sa.sa_flags = 0; // no SA_RESTART: the read returns EINTR
    sigemptyset(&sa.sa_mask);
    ::sigaction(SIGALRM, &sa, &old);
    struct itimerval it{};
    it.it_value.tv_usec = 20000;
    ::setitimer(ITIMER_REAL, &it, nullptr);
    auto buf = gp::make_slice<uint8_t>(1);
    auto [n, e] = sys_read(fds[0], buf);
    (void)n;
    ::sigaction(SIGALRM, &old, nullptr);
    selftest_wfd.store(-1);
    ::close(fds[0]);
    ::close(fds[1]);
    return {selftest_runs.load(), e};
}

std::tuple<int64_t, int64_t, int64_t> sys_selftest_poll_eintr() {
    int fds[2];
    if (::pipe(fds) != 0) {
        return {0, -errno, 0};
    }
    auto [wake, we] = sys_wake_new();
    if (we != 0) {
        ::close(fds[0]);
        ::close(fds[1]);
        return {0, -we, 0};
    }
    selftest_runs.store(0);
    selftest_wfd.store(-1);
    struct sigaction sa{};
    struct sigaction old{};
    sa.sa_handler = selftest_handler;
    sa.sa_flags = 0;
    sigemptyset(&sa.sa_mask);
    ::sigaction(SIGALRM, &sa, &old);
    struct itimerval it{};
    it.it_value.tv_usec = 20000;
    it.it_interval.tv_usec = 20000;
    ::setitimer(ITIMER_REAL, &it, nullptr);
    auto start = std::chrono::steady_clock::now();
    auto [state, e] = sys_poll_wait(fds[0], wake, false, 150'000'000);
    auto took = std::chrono::steady_clock::now() - start;
    struct itimerval off{};
    ::setitimer(ITIMER_REAL, &off, nullptr);
    ::sigaction(SIGALRM, &old, nullptr);
    ::close(fds[0]);
    ::close(fds[1]);
    ::close(static_cast<int>(wake));
    return {selftest_runs.load(), e != 0 ? -e : state,
            std::chrono::duration_cast<std::chrono::milliseconds>(took).count()};
}
#endif // GOCPP_RUNTIME_TEST

std::tuple<gp::string, int64_t> sys_nodename() {
    struct utsname u;
    if (::uname(&u) != 0) {
        return {gp::string{}, fail()};
    }
    return {gp::string::gp_copy(u.nodename), 0};
}

void sys_exit(int64_t code) {
    gp::os::exit(static_cast<int>(code));
}

} // namespace gp::syscall
