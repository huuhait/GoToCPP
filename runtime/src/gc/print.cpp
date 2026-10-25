// gocpp/gc/print.cpp: the out-of-line parts of print and println (phase-16 README §2.2; job
// 16.1c).
#include <gocpp/nofortify.h>
#include <gocpp/gc/print.hpp>

#include <cstdio>
#include <cstdint>
#include <string>
#include <string_view>

namespace gp::gc::detail {

#ifdef GOCPP_RUNTIME_TEST
void (*write_stderr_hook)(std::string_view) = nullptr;
#endif

void print_int(std::string& out, int64_t v) {
    char buf[24]; // the sign and the 19 digits of INT64_MIN
    char* p = buf + sizeof buf;
    std::uint64_t u = static_cast<std::uint64_t>(v);
    if (v < 0) {
        u = std::uint64_t{0} - u; // the magnitude, also for INT64_MIN
    }
    do {
        *--p = static_cast<char>('0' + u % 10);
        u /= 10;
    } while (u != 0);
    if (v < 0) {
        *--p = '-';
    }
    out.append(p, static_cast<std::size_t>(buf + sizeof buf - p));
}

void write_stderr(std::string_view s) {
#ifdef GOCPP_RUNTIME_TEST
    if (write_stderr_hook != nullptr) {
        write_stderr_hook(s);
        return;
    }
#endif
    if (!s.empty()) {
        std::fwrite(s.data(), 1, s.size(), stderr);
    }
    std::fflush(stderr); // Go's println writes at once, also through a C library that buffered
}

} // namespace gp::gc::detail
