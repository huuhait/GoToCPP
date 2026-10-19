// gocpp/wstr.hpp: gocpp strings at the boundary of wide C strings (Phase 14, step 14.4, ADR-082):
// C.wstr(s) and C.GoStringW(p), C.GoStringWN(p, n). They are for the W functions of the Windows API
// (LPCWSTR, LPWSTR: wchar_t is 16 bits there, and the text is UTF-16), and they work on every
// target: where wchar_t is 32 bits (Linux) the text is UTF-32, which lets the logic be tested
// anywhere.
//
// The emitter writes:
//
//     gocpp                  C++
//     C.wstr(s)              gp::wstr(s), and gp::wstr(s).gp_ptr() as a C/C++ argument
//     C.WString(s)           gp::cgo::WString(s)  (a malloc'ed copy; C.free frees it)
//     C.GoStringW(p)         gp::cgo::GoStringW(p)
//     C.GoStringWN(p, n)     gp::cgo::GoStringWN(p, n)
//
// The conversions are Go's own for Windows (syscall.UTF16FromString and UTF16ToString of Go 1.25
// with the WTF-8 rule of go.dev/issue/59971, stdlib/syscall/wtf8_windows.gp), so that a string that
// went through a W function and came back is the same string:
//   - a gocpp string is read as UTF-8; a byte that is no part of a valid sequence becomes U+FFFD;
//   - a surrogate encoded as three bytes (WTF-8: 0xED 0xA0..0xBF 0x80..0xBF) becomes that one code
//     unit, a lone surrogate, as Windows allows it in file names;
//   - a code point beyond U+FFFF becomes a surrogate pair where wchar_t has 16 bits;
//   - going back, a lone surrogate (or, with 32-bit units, a surrogate value or one beyond
//   U+10FFFF)
//     becomes its WTF-8 bytes (a value beyond U+10FFFF becomes U+FFFD).
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/arith.hpp>
#include <gocpp/panic.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <string>
#include <type_traits>

namespace gp {
namespace detail {

// A pointer to a wide character: any integer type of 2 bytes (wchar_t on Windows, char16_t,
// unsigned short) or 4 bytes (wchar_t on Linux, char32_t), as W functions and their wrappers
// declare it.
template <typename P>
concept wide_pointer =
    std::is_pointer_v<P> && std::is_integral_v<std::remove_cv_t<std::remove_pointer_t<P>>> &&
    !std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, bool> &&
    (sizeof(std::remove_pointer_t<P>) == 2 || sizeof(std::remove_pointer_t<P>) == 4);

// append_wide appends the code point r to out as W's units: a surrogate pair for r >= 0x10000 with
// 16-bit units. A surrogate value (a lone one: 0xD800..0xDFFF) is one unit.
template <typename W>
void append_wide(std::basic_string<W>& out, uint32_t r) {
    if constexpr (sizeof(W) == 2) {
        if (r >= 0x10000) {
            r -= 0x10000;
            out.push_back(static_cast<W>(0xD800 + (r >> 10)));
            out.push_back(static_cast<W>(0xDC00 + (r & 0x3FF)));
            return;
        }
    }
    out.push_back(static_cast<W>(r));
}

// to_wide converts the n bytes at p (UTF-8, or WTF-8 with surrogates) to W's units, as Go's
// encodeWTF16 does: see the top of the file. The result has no terminating NUL.
template <typename W>
std::basic_string<W> to_wide(const char* p, int64_t n) {
    std::basic_string<W> out;
    out.reserve(static_cast<std::size_t>(n));
    for (int64_t i = 0; i < n;) {
        int64_t width = 1;
        int32_t r = decode_rune(p + i, n - i, width);
        if (r == 0xFFFD && width == 1) {
            // Invalid UTF-8 -- or a WTF-8 surrogate: ED A0..BF 80..BF.
            const auto* s = reinterpret_cast<const unsigned char*>(p + i);
            if (n - i >= 3 && s[0] == 0xED && s[1] >= 0xA0 && s[1] <= 0xBF && s[2] >= 0x80 &&
                s[2] <= 0xBF) {
                out.push_back(
                    static_cast<W>(((s[0] & 0x0F) << 12) | ((s[1] & 0x3F) << 6) | (s[2] & 0x3F)));
                i += 3;
                continue;
            }
        }
        i += width;
        append_wide<W>(out, static_cast<uint32_t>(r));
    }
    return out;
}

// from_wide converts the n units at p to a gocpp string: UTF-8, with a lone surrogate as three
// WTF-8 bytes, as Go's decodeWTF16 does. With 32-bit units, a value beyond U+10FFFF is U+FFFD.
template <typename W>
string from_wide(const W* p, int64_t n) {
    std::string out;
    out.reserve(static_cast<std::size_t>(n));
    auto put = [&out](uint32_t r) {
        if (r < 0x80) {
            out.push_back(static_cast<char>(r));
        } else if (r < 0x800) {
            out.push_back(static_cast<char>(0xC0 | (r >> 6)));
            out.push_back(static_cast<char>(0x80 | (r & 0x3F)));
        } else if (r < 0x10000) {
            out.push_back(static_cast<char>(0xE0 | (r >> 12)));
            out.push_back(static_cast<char>(0x80 | ((r >> 6) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | (r & 0x3F)));
        } else {
            out.push_back(static_cast<char>(0xF0 | (r >> 18)));
            out.push_back(static_cast<char>(0x80 | ((r >> 12) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | ((r >> 6) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | (r & 0x3F)));
        }
    };
    using U = std::make_unsigned_t<W>;
    for (int64_t i = 0; i < n; i++) {
        uint32_t r = static_cast<uint32_t>(static_cast<U>(p[i]));
        if constexpr (sizeof(W) == 2) {
            if (r >= 0xD800 && r < 0xDC00 && i + 1 < n) {
                uint32_t r2 = static_cast<uint32_t>(static_cast<U>(p[i + 1]));
                if (r2 >= 0xDC00 && r2 < 0xE000) {
                    put(0x10000 + ((r - 0xD800) << 10) + (r2 - 0xDC00));
                    i++;
                    continue;
                }
            }
        } else if (r > 0x10FFFF) {
            r = 0xFFFD;
        }
        put(r); // a lone surrogate takes the 3-byte branch: its WTF-8 form
    }
    return string::gp_copy(std::string_view(out));
}

// wide_n_ok checks the arguments of GoStringWN as go_string_n does (runtime/src/interop.cpp): false
// for n == 0 (p isn't read), the fatal error "out of memory" for a negative n, a nil dereference
// for a null p.
bool wide_n_ok(int64_t n, const void* p);

template <typename W>
int64_t wide_length(const W* p) noexcept {
    int64_t n = 0;
    while (p[n] != 0) {
        n++;
    }
    return n;
}

} // namespace detail

// wstr_holder is what gp::wstr returns: a NUL-terminated copy of a gocpp string as wide characters.
// It converts implicitly to const wchar_t*, valid as long as the holder lives: to the end of the
// statement for gp::wstr(s) in a call, as long as the variable for `name := C.wstr(s)`. A copy
// keeps its own units. Unlike cstr_holder, it always owns a copy: a gocpp string is UTF-8 and the
// units are not. A holder passed to a variadic function does not convert (see cstr_holder): the
// emitter writes gp::wstr(s).gp_ptr() there.
class wstr_holder {
public:
    // The bytes of s (all of them; a NUL inside them ends the C string early, as with C.cstr).
    explicit wstr_holder(const string& s)
        : gp_buf_(detail::to_wide<wchar_t>(s.gp_data(), gp::len(s))) {}

    // A C++ string literal (the emitter writes an untyped constant as one).
    template <std::size_t N>
    explicit wstr_holder(const char (&a)[N])
        : gp_buf_(detail::to_wide<wchar_t>(a, static_cast<int64_t>(N) -
                                                  (N > 0 && a[N - 1] == '\0' ? 1 : 0))) {}

    operator const wchar_t*() const noexcept { return gp_buf_.c_str(); }
    const wchar_t* gp_ptr() const noexcept { return gp_buf_.c_str(); }

private:
    std::wstring gp_buf_;
};

// wstr(s) is C.wstr(s): s as a const wchar_t* (UTF-16 where wchar_t has 16 bits, UTF-32 where it
// has 32), through a holder that lives as long as the statement or the variable that holds it.
inline wstr_holder wstr(const string& s) {
    return wstr_holder(s);
}
template <std::size_t N>
wstr_holder wstr(const char (&a)[N]) {
    return wstr_holder(a);
}

namespace cgo {

// WString(s) is C.WString: C.CString for wide characters. A copy of s as wchar_t units and a NUL,
// in C's heap (malloc); free it with C.free. It is for a pointer that C keeps after the statement
// (a struct field such as WNDCLASSW::lpszClassName); C.wstr is for one that C uses during the call.
// A NUL inside s ends the C string early. If malloc fails, it is Go's fatal error "runtime: C
// malloc failed".
inline wchar_t* WString(const string& s) {
    std::wstring w = detail::to_wide<wchar_t>(s.gp_data(), gp::len(s));
    auto* p = static_cast<wchar_t*>(std::malloc((w.size() + 1) * sizeof(wchar_t)));
    if (p == nullptr) {
        detail::fatal("runtime: C malloc failed");
    }
    if (!w.empty()) {
        std::memcpy(p, w.data(), w.size() * sizeof(wchar_t));
    }
    p[w.size()] = 0;
    return p;
}

// GoStringW(p) is C.GoStringW: the wide C string at p, up to its NUL, as a gocpp string. A null p
// gives
// "". p is a pointer to any 2-byte or 4-byte integer (LPCWSTR, LPWSTR, a UINT16 buffer, a char16_t
// array), and units are read as W's (UTF-16 for 2 bytes, UTF-32 for 4).
template <typename P>
    requires detail::wide_pointer<P>
string GoStringW(P p) {
    if (p == nullptr) {
        return string();
    }
    return detail::from_wide(p, detail::wide_length(p));
}
inline string GoStringW(std::nullptr_t) noexcept {
    return string();
}
template <typename E>
    requires detail::wide_pointer<E*>
string GoStringW(const ptr<E>& p) { // &buf[0] of a []uint16
    return GoStringW(static_cast<E*>(p));
}
template <typename E>
    requires detail::wide_pointer<E*>
string GoStringW(const cptr<E>& p) {
    return GoStringW(p.gp_get());
}

// GoStringWN(p, n) is C.GoStringWN: the n wide characters at p (NUL units too), as a gocpp string.
// n == 0 gives "" (p isn't read). A null p with n > 0 panics like Go's nil dereference; a negative
// n is Go's fatal error "out of memory", as C.GoStringN.
template <typename P, go_int N>
    requires detail::wide_pointer<P>
string GoStringWN(P p, N n) {
    int64_t len = static_cast<int64_t>(n);
    if (!detail::wide_n_ok(len, p)) {
        return string();
    }
    return detail::from_wide(p, len);
}
template <go_int N>
string GoStringWN(std::nullptr_t, N n) {
    return GoStringWN(static_cast<const char16_t*>(nullptr), n);
}
template <typename E, go_int N>
    requires detail::wide_pointer<E*>
string GoStringWN(const ptr<E>& p, N n) {
    return GoStringWN(static_cast<E*>(p), n);
}
template <typename E, go_int N>
    requires detail::wide_pointer<E*>
string GoStringWN(const cptr<E>& p, N n) {
    return GoStringWN(p.gp_get(), n);
}

} // namespace cgo
} // namespace gp
