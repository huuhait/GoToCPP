// The native functions of package time that are not timers (gocpp/time.hpp, Phase 08 step 8.8): the
// clocks, the once-only calls and the small file functions that the time zone loader needs.
// Standard C++ only: the clocks are the C++ library's (CLOCK_MONOTONIC and CLOCK_REALTIME through
// the vDSO on Linux, a few tens of nanoseconds), the files are C stdio (the same code works on
// Windows).
#include <gocpp/nofortify.h>
#include <gocpp/rt.hpp>

#include <cerrno>
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <mutex>
#include <string>

namespace gp::time {

std::tuple<int64_t, int32_t, int64_t> now() noexcept {
    // The wall clock first, the monotonic clock right after: Go's runtime reads them in the same
    // order, and the two readings are what Time keeps (wall seconds, nanoseconds and monotonic
    // offset).
    auto wall = std::chrono::system_clock::now().time_since_epoch();
    int64_t ns = std::chrono::duration_cast<std::chrono::nanoseconds>(wall).count();
    int64_t sec = ns / 1000000000;
    int64_t rem = ns % 1000000000;
    if (rem < 0) {
        rem += 1000000000;
        sec--;
    }
    return {sec, static_cast<int32_t>(rem), nano()};
}

int64_t nano() noexcept {
    auto t = std::chrono::steady_clock::now().time_since_epoch();
    return std::chrono::duration_cast<std::chrono::nanoseconds>(t).count();
}

void call_once(int64_t id, const gp::func<void()>& f) {
    static std::once_flag flags[3];
    if (id < 0 || id >= 3) {
        gp::panic(gp::string::gp_copy("time: bad once id"));
    }
    std::call_once(flags[id], [&f] { f(); });
}

std::tuple<gp::string, bool> getenv(const gp::string& key) {
    std::string k(key.gp_view());
    const char* v = std::getenv(k.c_str());
    if (v == nullptr) {
        return {gp::string{}, false};
    }
    return {gp::string::gp_copy(v), true};
}

gp::string errno_text(int64_t e) {
    std::string s = std::strerror(static_cast<int>(e));
    if (!s.empty() && s[0] >= 'A' && s[0] <= 'Z') {
        s[0] = static_cast<char>(s[0] - 'A' +
                                 'a'); // Go's syscall.Errno text: "no such file or directory"
    }
    return gp::string::gp_copy(s);
}

std::tuple<int64_t, int64_t> open_file(const gp::string& name) {
    std::string path(name.gp_view());
    std::FILE* f = std::fopen(path.c_str(), "rb");
    if (f == nullptr) {
        return {0, errno != 0 ? errno : EIO};
    }
    return {static_cast<int64_t>(reinterpret_cast<std::intptr_t>(f)), 0};
}

std::tuple<int64_t, int64_t> read_file(int64_t fd, const gp::slice<uint8_t>& buf) {
    auto* f = reinterpret_cast<std::FILE*>(static_cast<std::intptr_t>(fd));
    if (buf.gp_len() == 0) {
        return {0, 0};
    }
    std::size_t n = std::fread(buf.gp_data(), 1, static_cast<std::size_t>(buf.gp_len()), f);
    if (n == 0 && std::ferror(f) != 0) {
        return {0, errno != 0 ? errno : EIO};
    }
    return {static_cast<int64_t>(n), 0};
}

void close_file(int64_t fd) {
    std::fclose(reinterpret_cast<std::FILE*>(static_cast<std::intptr_t>(fd)));
}

int64_t seek_file(int64_t fd, int64_t off, int64_t whence) {
    auto* f = reinterpret_cast<std::FILE*>(static_cast<std::intptr_t>(fd));
    int w = whence == 0 ? SEEK_SET : whence == 1 ? SEEK_CUR : SEEK_END;
    if (std::fseek(f, static_cast<long>(off), w) != 0) {
        return errno != 0 ? errno : EIO;
    }
    return 0;
}

} // namespace gp::time
