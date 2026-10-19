// gocpp/bytealg.hpp: the native functions of the standard package internal/bytealg (Phase 09 step
// 9.2). Go's package is the byte-searching routines of strings, bytes, strconv, os and others, in
// assembly for each processor; stdlib/internal/bytealg/native.gp declares each one without a body
// and with //gocpp:native (ADR-027), and a call becomes a call of the function here:
//
//     IndexByte, IndexByteString                  gp::bytealg::index_byte, index_byte_string
//     LastIndexByte, LastIndexByteString          last_index_byte, last_index_byte_string
//     Count, CountString                          count, count_string
//     Equal                                       equal
//     Compare, CompareString                      compare, compare_string
//     Index, IndexString                          index, index_string
//     MakeNoZero                                  make_no_zero
//
// The parameters are the C++ types of the Go ones (gp::slice<uint8_t> for []byte, gp::string for
// string, int64_t for int), taken by reference: nothing here keeps its arguments. A nil slice and
// the empty string have a null data pointer, which memchr and memcmp must not get, so every
// function checks the length first. The short, hot ones are inline (strings.Index calls IndexByte
// in a loop); the rest are in runtime/src/bytealg.cpp.
#pragma once

#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>

namespace gp::bytealg {

namespace detail {

// index_raw: the index of the first c in the n bytes at p, or -1.
inline int64_t index_raw(const void* p, int64_t n, uint8_t c) noexcept {
    if (n <= 0) {
        return -1;
    }
    const void* q = std::memchr(p, c, static_cast<std::size_t>(n));
    return q == nullptr ? -1 : static_cast<const char*>(q) - static_cast<const char*>(p);
}

// equal_raw: the n bytes at a and the n bytes at b are the same.
inline bool equal_raw(const void* a, const void* b, int64_t n) noexcept {
    return n <= 0 || a == b || std::memcmp(a, b, static_cast<std::size_t>(n)) == 0;
}

// compare_raw: -1, 0 or +1, the order of the na bytes at a and the nb bytes at b.
inline int64_t compare_raw(const void* a, int64_t na, const void* b, int64_t nb) noexcept {
    int64_t n = na < nb ? na : nb;
    if (n > 0 && a != b) {
        int c = std::memcmp(a, b, static_cast<std::size_t>(n));
        if (c != 0) {
            return c < 0 ? -1 : 1;
        }
    }
    return na < nb ? -1 : (na > nb ? 1 : 0);
}

// The functions of runtime/src/bytealg.cpp, on pointers and lengths.
int64_t last_index_raw(const void* p, int64_t n, uint8_t c) noexcept;
int64_t count_raw(const void* p, int64_t n, uint8_t c) noexcept;
// The two ways to count that count_raw chooses from (the unit tests run both in every mode).
int64_t count_words(const void* p, int64_t n, uint8_t c) noexcept;
int64_t count_loop(const void* p, int64_t n, uint8_t c) noexcept;
int64_t index_mem(const void* a, int64_t na, const void* b, int64_t nb) noexcept;

} // namespace detail

inline int64_t index_byte(const gp::slice<uint8_t>& b, uint8_t c) noexcept {
    return detail::index_raw(b.gp_data(), b.gp_len(), c);
}
inline int64_t index_byte_string(const gp::string& s, uint8_t c) noexcept {
    return detail::index_raw(s.gp_data(), gp::len(s), c);
}

inline int64_t last_index_byte(const gp::slice<uint8_t>& b, uint8_t c) noexcept {
    return detail::last_index_raw(b.gp_data(), b.gp_len(), c);
}
inline int64_t last_index_byte_string(const gp::string& s, uint8_t c) noexcept {
    return detail::last_index_raw(s.gp_data(), gp::len(s), c);
}

inline int64_t count(const gp::slice<uint8_t>& b, uint8_t c) noexcept {
    return detail::count_raw(b.gp_data(), b.gp_len(), c);
}
inline int64_t count_string(const gp::string& s, uint8_t c) noexcept {
    return detail::count_raw(s.gp_data(), gp::len(s), c);
}

inline bool equal(const gp::slice<uint8_t>& a, const gp::slice<uint8_t>& b) noexcept {
    return a.gp_len() == b.gp_len() && detail::equal_raw(a.gp_data(), b.gp_data(), a.gp_len());
}

inline int64_t compare(const gp::slice<uint8_t>& a, const gp::slice<uint8_t>& b) noexcept {
    return detail::compare_raw(a.gp_data(), a.gp_len(), b.gp_data(), b.gp_len());
}
inline int64_t compare_string(const gp::string& a, const gp::string& b) noexcept {
    return detail::compare_raw(a.gp_data(), gp::len(a), b.gp_data(), gp::len(b));
}

// index(a, b): the first place where b is a part of a, or -1; the empty b is found at 0. (Go's
// Index needs 2 <= len(b) <= MaxLen; this one takes any b.)
inline int64_t index(const gp::slice<uint8_t>& a, const gp::slice<uint8_t>& b) noexcept {
    return detail::index_mem(a.gp_data(), a.gp_len(), b.gp_data(), b.gp_len());
}
inline int64_t index_string(const gp::string& a, const gp::string& b) noexcept {
    return detail::index_mem(a.gp_data(), gp::len(a), b.gp_data(), gp::len(b));
}

// make_no_zero(n): a slice of n bytes whose bytes are not set (not zeroed). Panics like make for a
// negative or too big n.
gp::slice<uint8_t> make_no_zero(int64_t n);

} // namespace gp::bytealg
