// The Linux OS layer (gocpp/os.hpp). This folder is the only place in the runtime that may
// include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include "linux.hpp"

#include <cerrno>
#include <csignal>
#include <cstdint>
#include <cstdio>
#include <cstring>

#include <poll.h>
#include <unistd.h>

#if defined(GOCPP_LSAN)
#include <sanitizer/lsan_interface.h>
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
