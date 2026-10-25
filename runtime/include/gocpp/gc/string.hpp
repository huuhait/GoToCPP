// gocpp/gc/string.hpp: gp::gc::string, Go's string type in the GC engine (phase-16 README
// §2.2 and §2.4).
//
// A gp::gc::string is Go's string header and nothing else: a pointer to the bytes and an int64
// length, 16 bytes (the reference-counting engine's gp::string is 32). The bytes are immutable
// and the header never points past them: a substring keeps the pointer of the bytes it starts
// in and only shortens the length, as Go's gc does (golang.org/issue/8404), so a C++ library
// that compares an end pointer with an unrelated allocation cannot see it. (A concatenation
// allocates one byte more than its length for a NUL that is not part of the string.)
//
// A literal (gp::gc::lit("hello")) is a string over the literal's static bytes: no copy and no
// allocation, and it is constexpr, so a package variable with a constant value can use it. A
// string made at run time (a concatenation) owns its bytes in the GC heap, allocated
// pointer-free; the collector frees them when nothing reaches the string.
//
// Like every runtime type, gp::gc::string has only operators and gp_-prefixed members;
// everything else is a free function such as gp::gc::len(s).
#pragma once

#include <gocpp/gc/checks.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <string_view>

namespace gp::gc {

// bytes_t selects the constructor that makes a string over bytes that already exist: static
// storage (a literal) or the GC heap (a concatenation). In the GC engine there is no owner to
// take, so both are the same to the type; the collector keeps heap bytes alive while the string
// is reachable.
struct bytes_t {
    explicit bytes_t() = default;
};
inline constexpr bytes_t bytes{};

class string {
public:
    // The empty string, Go's zero value: a null pointer and a zero length.
    constexpr string() noexcept = default;

    // A string over len bytes at data, which must be static storage or GC memory and must stay
    // alive as long as the string is reachable.
    constexpr string(bytes_t, const char* data, int64_t len) noexcept
        : gp_data_(data), gp_len_(len) {}

    // gp_data and gp_len give the bytes: gp_data may be null when the string is empty.
    constexpr const char* gp_data() const noexcept { return gp_data_; }
    constexpr int64_t gp_len() const noexcept { return gp_len_; }
    constexpr bool gp_empty() const noexcept { return gp_len_ == 0; }

    // gp_view gives the bytes as a std::string_view; an empty string gives an empty view, also
    // when its pointer is null.
    constexpr std::string_view gp_view() const noexcept {
        return gp_len_ == 0 ? std::string_view()
                            : std::string_view(gp_data_, static_cast<std::size_t>(gp_len_));
    }

    // operator+= is Go's s += t: the header becomes the concatenation (a string is a value, so
    // this changes the variable, not shared bytes). Defined after concat, below.
    string& operator+=(string t);

private:
    const char* gp_data_ = nullptr;
    int64_t gp_len_ = 0;
};

// Go's string header: a pointer and a length, 16 bytes (README §2.2).
static_assert(sizeof(string) == 16, "gp::gc::string must be Go's 16-byte string header");
static_assert(alignof(string) == 8, "gp::gc::string must be 8-byte aligned, as in Go");

// lit returns a string over the bytes of a string literal: static data, no copy, no allocation.
template <std::size_t N>
constexpr string lit(const char (&b)[N]) noexcept {
    return string(bytes, b, static_cast<int64_t>(N - 1));
}

// len returns the length of s in bytes, as Go's builtin len does for a string.
constexpr int64_t len(string s) noexcept {
    return s.gp_len();
}

// compare returns -1, 0 or 1: Go's string order, the bytes compared lexicographically and the
// shorter string first when one is a prefix of the other.
inline int compare(string a, string b) noexcept {
    const int64_t n = a.gp_len() < b.gp_len() ? a.gp_len() : b.gp_len();
    if (n > 0) {
        const int c = std::memcmp(a.gp_data(), b.gp_data(), static_cast<std::size_t>(n));
        if (c != 0) {
            return c < 0 ? -1 : 1;
        }
    }
    return a.gp_len() < b.gp_len() ? -1 : a.gp_len() > b.gp_len() ? 1 : 0;
}

// The comparisons that Go has for strings: == and != by value, and < <= > >= in Go's string
// order (lexicographic over the bytes).
inline bool operator==(string a, string b) noexcept {
    return a.gp_len() == b.gp_len() &&
           (a.gp_len() == 0 ||
            std::memcmp(a.gp_data(), b.gp_data(), static_cast<std::size_t>(a.gp_len())) == 0);
}
inline bool operator!=(string a, string b) noexcept {
    return !(a == b);
}
inline bool operator<(string a, string b) noexcept {
    return compare(a, b) < 0;
}
inline bool operator<=(string a, string b) noexcept {
    return compare(a, b) <= 0;
}
inline bool operator>(string a, string b) noexcept {
    return compare(a, b) > 0;
}
inline bool operator>=(string a, string b) noexcept {
    return compare(a, b) >= 0;
}

// sub(s, lo, hi): s[lo:hi], Go's string slicing, with Go's checks (0 <= lo <= hi <= len(s)).
// The result points into the same bytes, and an empty result keeps s's pointer (Go's gc does,
// so it never forms a pointer past the string). sub(s, lo) is s[lo:].
template <go_int L, go_int H>
string sub(string s, L lo, H hi) {
    check_slice(lo, hi, s.gp_len(), bounds::slice_alen);
    auto l = static_cast<int64_t>(lo);
    auto h = static_cast<int64_t>(hi);
    if (h == l) {
        return string(bytes, s.gp_data(), 0);
    }
    return string(bytes, s.gp_data() + l, h - l);
}

template <go_int L>
string sub(string s, L lo) {
    return sub(s, lo, s.gp_len());
}

// concat returns a + b: the bytes are copied into a new pointer-free GC allocation, which the
// collector frees when nothing reaches the result.
string concat(string a, string b);

inline string operator+(string a, string b) {
    return concat(a, b);
}

inline string& string::operator+=(string t) {
    *this = concat(*this, t);
    return *this;
}

} // namespace gp::gc
