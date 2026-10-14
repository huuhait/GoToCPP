// gocpp/string.hpp: gp::string, Go's string type.
//
// A gp::string is an immutable run of bytes: an owner, a data pointer and a length (32 bytes).
// A literal ("hi"_gs) points at static storage and has no owner, so it costs no allocation,
// and copying it changes no reference count. A string made at run time (a concatenation, a
// conversion) owns its bytes through the owner, a std::shared_ptr, so copies share the bytes,
// and so does a substring s[a:b] (gp::sub). The bytes never hold another owner, so dropping
// one can't free a chain: unlike gp::ptr, a string drops its owner directly, not through
// gp::detail::release (ptr.hpp).
//
// The invariant of ptr.hpp (a pointer into static storage shares gp::static_owner(), never an
// empty owner) holds at the place where a pointer into a string's bytes is made: gp_owner()
// gives static_owner() for static bytes. A literal itself keeps an empty owner: with
// static_owner() in every literal, each "..."_gs and each copy of one would change that one
// shared count, from every thread, and the constexpr constructor (constinit) couldn't exist.
//
// Like every runtime type, gp::string has only operators and gp_-prefixed members; everything
// else is a free function such as gp::len(s), so a Go method named len can't clash with it.
#pragma once

#include <gocpp/bounds.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/ptr.hpp>

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

    // gp_owner: the owner of the bytes, for a pointer into them (unsafe.StringData, Phase 04).
    // Static bytes (a literal) give gp::static_owner(), never an empty owner (ptr.hpp).
    std::shared_ptr<const void> gp_owner() const noexcept {
        if (gp_owner_ == nullptr && gp_data_ != nullptr) {
            return static_owner();
        }
        return gp_owner_;
    }

    // gp_sub: the bytes [lo, hi) of the string, which share its owner, without a bounds
    // check: gp::sub checks them. An empty result is the empty string (it keeps nothing
    // alive).
    string gp_sub(int64_t lo, int64_t hi) const noexcept {
        if (lo == hi) {
            return string();
        }
        string r;
        r.gp_owner_ = gp_owner_;
        r.gp_data_ = gp_data_ + lo;
        r.gp_len_ = hi - lo;
        return r;
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

namespace gp {

// --- substrings, concatenation, UTF-8 and string(r) (Phase 03 step 3.4) ---

// sub(s, lo, hi) is s[lo:hi], and sub(s, lo) is s[lo:] (s[:hi] is sub(s, 0, hi)). The
// substring shares the bytes of s. The bounds may have any integer types, and Go's checks run
// in Go's order (gp::check_slice, bounds.hpp): hi <= len(s) ("slice bounds out of range [:5]
// with length 3"), then lo <= hi ("slice bounds out of range [3:2]").
template <go_int L, go_int H>
string sub(const string& s, L lo, H hi) {
    check_slice(lo, hi, len(s), bounds::slice_alen);
    return s.gp_sub(static_cast<int64_t>(lo), static_cast<int64_t>(hi));
}

template <go_int L>
string sub(const string& s, L lo) {
    int64_t n = len(s);
    check_slice(lo, n, n, bounds::slice_alen); // hi = len(s) is in range: only lo <= len(s)
    return s.gp_sub(static_cast<int64_t>(lo), n);
}

namespace detail {

// concat is gp::concat for n strings (rt.cpp).
string concat(const string* const* parts, std::size_t n);

// decode_rune decodes the UTF-8 sequence at the start of the n > 0 bytes at p, exactly like
// Go's utf8.DecodeRuneInString: it returns the rune and sets width to its length in bytes.
// An invalid or incomplete sequence, an overlong encoding, a surrogate half (U+D800 to
// U+DFFF) and a value above U+10FFFF give U+FFFD with width 1 (rt.cpp).
int32_t decode_rune(const char* p, int64_t n, int64_t& width) noexcept;

// rune_string is string(r) for a value that fits in int64 (rt.cpp).
string rune_string(int64_t r);

} // namespace detail

// concat(a, b, c, ...) is a + b + c + ...: one allocation for the whole result, where a chain
// of + would make a string for each +. If only one part is not empty, the result is that part
// (no copy), as in Go. A part may be of a named string type (gp::newtype over gp::string,
// step 3.9), which is a string.
template <typename... S>
    requires(sizeof...(S) >= 2 && (std::derived_from<S, string> && ...))
string concat(const S&... parts) {
    const string* list[] = {&parts...};
    return detail::concat(list, sizeof...(S));
}

// rune_to_string(x) is Go's string(x) for an integer x: the UTF-8 encoding of the rune x, or
// of U+FFFD ("\uFFFD") if x is not a valid code point: negative, a surrogate half (U+D800 to
// U+DFFF), or above U+10FFFF.
template <go_int I>
string rune_to_string(I x) {
    if constexpr (std::is_unsigned_v<I> && sizeof(I) == 8) {
        if (x > 0x10FFFF) {
            return detail::rune_string(-1); // too big for int64 too
        }
    }
    return detail::rune_string(static_cast<int64_t>(x));
}

// string_range is the state of `for i, r := range s`: the hidden counters of Go 1.22, so
// assigning to i or r in the body doesn't change the iteration. It holds s, evaluated once.
// After each successful range_next, gp_i is where the rune starts and gp_r is the rune;
// invalid UTF-8 gives U+FFFD and steps one byte, as in Go.
struct string_range {
    string gp_s;
    int64_t gp_i = 0;    // where the current rune starts
    int64_t gp_next = 0; // where the next one starts
    int32_t gp_r = 0;    // the current rune
};

// range_string(s) starts a range over s.
inline string_range range_string(const string& s) noexcept {
    return string_range{s};
}

// range_next steps to the next rune of the range, and reports whether there is one.
inline bool range_next(string_range& it) noexcept {
    int64_t i = it.gp_next;
    int64_t n = len(it.gp_s);
    if (i >= n) {
        return false;
    }
    it.gp_i = i;
    auto c = static_cast<uint8_t>(it.gp_s.gp_data()[i]);
    if (c < 0x80) {
        it.gp_r = c;
        it.gp_next = i + 1;
        return true;
    }
    int64_t width = 1;
    it.gp_r = detail::decode_rune(it.gp_s.gp_data() + i, n - i, width);
    it.gp_next = i + width;
    return true;
}

} // namespace gp

// Generated code uses "..."_gs inside its package namespaces (gpkg_main), so the literal
// operator must be visible everywhere. Only this one name is brought in.
using gp::literals::operator""_gs;
