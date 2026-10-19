// The harness of TestWindowsConsoleUTF8 (runtime/winutf8_test.go): it reads lines from standard
// input, each a series of writes separated by '|', each write in hex. It feeds them one after the
// other to the Windows OS layer's console conversion (src/os/windows/utf8.cpp, plain C++, so also
// built for the host), and prints, for each line, the UTF-16 units that came out (hex, in order)
// and then ':' and the bytes still held at the end (hex).
#include <cstddef>
#include <cstdio>
#include <cstdlib>
#include <iostream>
#include <string>
#include <vector>

namespace gp::os {
struct utf8_tail {
    unsigned char bytes[4] = {};
    std::size_t n = 0;
};
bool utf8_to_utf16(utf8_tail& tail, const char* data, std::size_t n,
                   bool (*emit)(void* ctx, const char16_t* units, std::size_t count) noexcept,
                   void* ctx) noexcept;
} // namespace gp::os

namespace {

bool emit(void* ctx, const char16_t* units, std::size_t n) noexcept {
    if (n == 0 || n > 2048) {
        std::fprintf(stderr, "emit of %zu units\n", n);
        std::abort();
    }
    auto* out = static_cast<std::string*>(ctx);
    char b[8];
    for (std::size_t i = 0; i < n; i++) {
        std::snprintf(b, sizeof b, "%04x", static_cast<unsigned>(units[i]));
        *out += b;
    }
    return true;
}

} // namespace

int main() {
    std::string line;
    while (std::getline(std::cin, line)) {
        gp::os::utf8_tail tail;
        std::string out;
        std::size_t i = 0;
        for (;;) {
            std::size_t j = line.find('|', i);
            std::string hex = line.substr(i, j == std::string::npos ? std::string::npos : j - i);
            std::string bytes;
            for (std::size_t k = 0; k + 1 < hex.size(); k += 2) {
                bytes += static_cast<char>(std::strtol(hex.substr(k, 2).c_str(), nullptr, 16));
            }
            if (!gp::os::utf8_to_utf16(tail, bytes.data(), bytes.size(), emit, &out)) {
                return 1;
            }
            if (j == std::string::npos) {
                break;
            }
            i = j + 1;
        }
        out += ':';
        char b[8];
        for (std::size_t k = 0; k < tail.n; k++) {
            std::snprintf(b, sizeof b, "%02x", tail.bytes[k]);
            out += b;
        }
        std::puts(out.c_str());
    }
    return 0;
}
