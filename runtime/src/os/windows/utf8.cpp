// UTF-8 to UTF-16 for the console (windows.hpp, utf8_to_utf16): the rules of Go's console writer.
// Plain C++, no Windows header: the runtime's tests compile this file on Linux too and compare it
// with Go's own (runtime/winutf8_test.go).
#include "windows.hpp"

namespace gp::os {

namespace {

// The UTF-8 rules of Go's unicode/utf8 (DecodeRune and FullRune).
struct utf8_range {
    unsigned char lo, hi;
};

// seq_len returns the length of the sequence that the first byte c starts (1 for ASCII and for a
// byte that starts nothing), and the range of its second byte.
std::size_t seq_len(unsigned char c, utf8_range& second) noexcept {
    second = {0x80, 0xBF};
    if (c < 0x80) {
        return 1;
    }
    if (c >= 0xC2 && c <= 0xDF) {
        return 2;
    }
    if (c >= 0xE0 && c <= 0xEF) {
        if (c == 0xE0) {
            second.lo = 0xA0;
        } else if (c == 0xED) {
            second.hi = 0x9F;
        }
        return 3;
    }
    if (c >= 0xF0 && c <= 0xF4) {
        if (c == 0xF0) {
            second.lo = 0x90;
        } else if (c == 0xF4) {
            second.hi = 0x8F;
        }
        return 4;
    }
    return 1; // 0x80 to 0xC1, 0xF5 to 0xFF
}

// full_rune is utf8.FullRune: whether p (1 to 4 bytes) starts with a whole sequence, or with bytes
// that are invalid whatever follows, so that DecodeRune would give its answer for them already.
bool full_rune(const unsigned char* p, std::size_t n) noexcept {
    utf8_range second;
    std::size_t need = seq_len(p[0], second);
    if (n >= need) {
        return true;
    }
    if (n > 1 && (p[1] < second.lo || second.hi < p[1])) {
        return true;
    }
    return n > 2 && (p[2] < 0x80 || 0xBF < p[2]);
}

// decode_rune is utf8.DecodeRune for a p that full_rune accepts: the code point and its width (an
// invalid byte is U+FFFD of width 1).
char32_t decode_rune(const unsigned char* p, std::size_t n, std::size_t& width) noexcept {
    width = 1;
    utf8_range second;
    std::size_t need = seq_len(p[0], second);
    if (need == 1 || n < need || p[1] < second.lo || second.hi < p[1]) {
        return p[0] < 0x80 ? p[0] : 0xFFFD;
    }
    char32_t r = need == 2 ? p[0] & 0x1F : need == 3 ? p[0] & 0x0F : p[0] & 0x07;
    for (std::size_t i = 1; i < need; i++) {
        if (p[i] < 0x80 || 0xBF < p[i]) {
            return 0xFFFD;
        }
        r = r << 6 | (p[i] & 0x3F);
    }
    width = need;
    return r;
}

} // namespace

bool utf8_to_utf16(utf8_tail& tail, const char* data, std::size_t n,
                   bool (*emit)(void*, const char16_t*, std::size_t) noexcept, void* ctx) noexcept {
    unsigned char held[4];
    const std::size_t held_n = tail.n;
    for (std::size_t i = 0; i < held_n; i++) {
        held[i] = tail.bytes[i];
    }
    tail.n = 0;
    const std::size_t total = held_n + n;
    auto at = [&](std::size_t i) -> unsigned char {
        return i < held_n ? held[i] : static_cast<unsigned char>(data[i - held_n]);
    };
    constexpr std::size_t cap = 2048;
    char16_t buf[cap];
    std::size_t units = 0;
    bool ok = true;
    std::size_t pos = 0;
    while (ok && pos < total) {
        unsigned char c = at(pos);
        char32_t r = c;
        std::size_t width = 1;
        if (c >= 0x80) {
            unsigned char w[4] = {};
            std::size_t k = total - pos < 4 ? total - pos : 4;
            for (std::size_t i = 0; i < k; i++) {
                w[i] = at(pos + i);
            }
            if (k < 4 && !full_rune(w, k)) {
                break; // a start of a sequence: the next write completes it
            }
            r = decode_rune(w, k, width);
        }
        pos += width;
        if (r >= 0x10000) {
            r -= 0x10000;
            buf[units++] = static_cast<char16_t>(0xD800 + (r >> 10));
            buf[units++] = static_cast<char16_t>(0xDC00 + (r & 0x3FF));
        } else {
            buf[units++] = static_cast<char16_t>(r);
        }
        if (units + 2 > cap) {
            ok = emit(ctx, buf, units);
            units = 0;
        }
    }
    if (ok && units > 0) {
        ok = emit(ctx, buf, units);
    }
    for (; ok && pos < total; pos++) { // at most 3 bytes: the start of a sequence
        tail.bytes[tail.n++] = at(pos);
    }
    return ok;
}

} // namespace gp::os
