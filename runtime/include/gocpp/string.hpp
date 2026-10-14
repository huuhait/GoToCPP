// gocpp/string.hpp: gp::string, Go's string type.
//
// A gp::string is an immutable run of bytes: an owner, a data pointer and a length (32 bytes).
// A literal ("hi"_gs) points at static storage and has no owner, so it costs no allocation.
// A string made at run time (a concatenation) owns its bytes through the owner, a
// std::shared_ptr, so copies share the bytes. Phase 03 adds substrings, which share them too.
//
// Like every runtime type, gp::string has only operators and gp_-prefixed members; everything
// else is a free function such as gp::len(s), so a Go method named len can't clash with it.
#pragma once

#include <gocpp/checks.hpp>

#include <compare>
#include <cstddef>
#include <cstdint>
#include <cstring>
#include <memory>
#include <string_view>

namespace gp {

// static_bytes selects the constructor for bytes in static storage.
struct static_bytes_t {
    explicit static_bytes_t() = default;
};
inline constexpr static_bytes_t static_bytes{};

class string {
public:
    // The empty string, Go's zero value.
    constexpr string() noexcept = default;

    // A string over len bytes at data, owned by owner (null for static storage: the bytes must
    // then live as long as the program).
    string(std::shared_ptr<const void> owner, const char* data, int64_t len) noexcept
        : gp_owner_(std::move(owner)), gp_data_(data), gp_len_(len) {}

    // A string over len bytes in static storage, such as a string literal: no owner, no
    // allocation. The constructor is constexpr, so a constinit variable (a package variable
    // with a constant value, Phase 06) can use it; "..."_gs can't be constexpr in C++20,
    // because the owner's destructor isn't.
    constexpr string(static_bytes_t, const char* data, int64_t len) noexcept
        : gp_data_(data), gp_len_(len) {}

    // gp_copy returns a string that owns a copy of the bytes of v.
    static string gp_copy(std::string_view v);

    // gp_data and gp_view give the bytes. gp_data may be null when the string is empty.
    const char* gp_data() const noexcept { return gp_data_; }
    std::string_view gp_view() const noexcept {
        return gp_len_ == 0 ? std::string_view()
                            : std::string_view(gp_data_, static_cast<std::size_t>(gp_len_));
    }

    // s[i]: the byte at index i, with Go's bounds check: an index outside [0, len(s)) panics
    // with "runtime error: index out of range [5] with length 3".
    template <go_int I>
    uint8_t operator[](I i) const {
        check_index(i, gp_len_);
        return static_cast<uint8_t>(gp_data_[static_cast<std::size_t>(i)]);
    }

    // s + t (Go's concatenation) makes a new string; s += t is s = s + t.
    friend string operator+(const string& a, const string& b);
    string& operator+=(const string& b) { return *this = *this + b; }

    // Go compares strings byte by byte, as unsigned bytes.
    friend bool operator==(const string& a, const string& b) noexcept {
        return a.gp_len_ == b.gp_len_ &&
               (a.gp_len_ == 0 || a.gp_data_ == b.gp_data_ ||
                std::memcmp(a.gp_data_, b.gp_data_, static_cast<std::size_t>(a.gp_len_)) == 0);
    }
    friend std::strong_ordering operator<=>(const string& a, const string& b) noexcept {
        // In size_t: with an int64_t count, g++ -O2 warns that a negative length would be
        // too big for memcmp (-Wstringop-overread), although lengths are never negative.
        auto la = static_cast<std::size_t>(a.gp_len_), lb = static_cast<std::size_t>(b.gp_len_);
        std::size_t n = la < lb ? la : lb;
        int c = n == 0 ? 0 : std::memcmp(a.gp_data_, b.gp_data_, n);
        if (c != 0) {
            return c < 0 ? std::strong_ordering::less : std::strong_ordering::greater;
        }
        return a.gp_len_ <=> b.gp_len_;
    }

    friend int64_t len(const string& s) noexcept;

private:
    std::shared_ptr<const void> gp_owner_;
    const char* gp_data_ = nullptr;
    int64_t gp_len_ = 0;
};

// len(s): the number of bytes in s.
inline int64_t len(const string& s) noexcept {
    return s.gp_len_;
}

static_assert(sizeof(string) == 32, "gp::string is 32 bytes: owner (16), data pointer, length");

inline namespace literals {

// "..."_gs: a string literal. It points at the literal's static storage, so it doesn't
// allocate. The length counts every byte, also a \000 inside the literal.
inline string operator""_gs(const char* data, std::size_t len) noexcept {
    return string(static_bytes, data, static_cast<int64_t>(len));
}

} // namespace literals

} // namespace gp

// Generated code uses "..."_gs inside its package namespaces (gpkg_main), so the literal
// operator must be visible everywhere. Only this one name is brought in.
using gp::literals::operator""_gs;
