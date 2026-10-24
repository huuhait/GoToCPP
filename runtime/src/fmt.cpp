// Go's package fmt (gocpp/fmt.hpp): a port of Go 1.25's fmt/print.go and fmt/format.go, of
// internal/fmtsort, and of the parts of strconv they use (AppendFloat, Quote), driven by type
// descriptors where Go's is driven by reflect. Each function mirrors the Go function of the same
// name (written in snake_case). Standard C++ only: the OS is reached through gocpp/os.hpp.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include "ftoa.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <thread>
#include <charconv>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <string>
#include <string_view>
#include <system_error>
#include <tuple>
#include <vector>

namespace gp::fmt::detail {

namespace {

constexpr char32_t rune_error = 0xFFFD;
constexpr char32_t max_rune = 0x10FFFF;
constexpr const char* ldigits = "0123456789abcdefx";
constexpr const char* udigits = "0123456789ABCDEFX";

// decode is utf8.DecodeRuneInString: the first rune of s and its size, U+FFFD and 1 for a byte
// that doesn't start a valid sequence. s is not empty.
std::pair<char32_t, std::size_t> decode(std::string_view s) noexcept {
    auto c = static_cast<unsigned char>(s[0]);
    if (c < 0x80) {
        return {c, 1};
    }
    int64_t width = 1;
    int32_t r = gp::detail::decode_rune(s.data(), static_cast<int64_t>(s.size()), width);
    return {static_cast<char32_t>(r), static_cast<std::size_t>(width)};
}

// rune_count is utf8.RuneCountInString.
int64_t rune_count(std::string_view s) noexcept {
    int64_t n = 0;
    for (std::size_t i = 0; i < s.size(); n++) {
        i += static_cast<unsigned char>(s[i]) < 0x80 ? 1 : decode(s.substr(i)).second;
    }
    return n;
}

bool valid_rune(char32_t r) noexcept {
    return r <= max_rune && !(r >= 0xD800 && r <= 0xDFFF);
}

// append_rune is utf8.AppendRune: an invalid rune is written as U+FFFD.
void append_rune(std::string& out, char32_t r) {
    if (!valid_rune(r)) {
        r = rune_error;
    }
    if (r < 0x80) {
        out += static_cast<char>(r);
    } else if (r < 0x800) {
        out += static_cast<char>(0xC0 | (r >> 6));
        out += static_cast<char>(0x80 | (r & 0x3F));
    } else if (r < 0x10000) {
        out += static_cast<char>(0xE0 | (r >> 12));
        out += static_cast<char>(0x80 | ((r >> 6) & 0x3F));
        out += static_cast<char>(0x80 | (r & 0x3F));
    } else {
        out += static_cast<char>(0xF0 | (r >> 18));
        out += static_cast<char>(0x80 | ((r >> 12) & 0x3F));
        out += static_cast<char>(0x80 | ((r >> 6) & 0x3F));
        out += static_cast<char>(0x80 | (r & 0x3F));
    }
}

// --- strconv: quoting ---

void append_escaped_rune(std::string& out, char32_t r, char quote, bool ascii_only) {
    constexpr const char* lowerhex = "0123456789abcdef";
    if (r == static_cast<char32_t>(quote) || r == '\\') { // always backslashed
        out += '\\';
        out += static_cast<char>(r);
        return;
    }
    if (ascii_only) {
        if (r < 0x80 && is_print(r)) {
            out += static_cast<char>(r);
            return;
        }
    } else if (is_print(r)) {
        append_rune(out, r);
        return;
    }
    switch (r) {
    case '\a':
        out += "\\a";
        break;
    case '\b':
        out += "\\b";
        break;
    case '\f':
        out += "\\f";
        break;
    case '\n':
        out += "\\n";
        break;
    case '\r':
        out += "\\r";
        break;
    case '\t':
        out += "\\t";
        break;
    case '\v':
        out += "\\v";
        break;
    default:
        if (r < ' ' || r == 0x7F) {
            out += "\\x";
            out += lowerhex[(r >> 4) & 0xF];
            out += lowerhex[r & 0xF];
            break;
        }
        if (!valid_rune(r)) {
            r = rune_error;
        }
        if (r < 0x10000) {
            out += "\\u";
            for (int s = 12; s >= 0; s -= 4) {
                out += lowerhex[(r >> s) & 0xF];
            }
        } else {
            out += "\\U";
            for (int s = 28; s >= 0; s -= 4) {
                out += lowerhex[(r >> s) & 0xF];
            }
        }
    }
}

// append_quoted is strconv.AppendQuote (AppendQuoteToASCII when ascii_only).
void append_quoted(std::string& out, std::string_view s, bool ascii_only) {
    constexpr const char* lowerhex = "0123456789abcdef";
    out += '"';
    for (std::size_t i = 0; i < s.size();) {
        auto [r, width] = decode(s.substr(i));
        if (width == 1 && r == rune_error) {
            auto b = static_cast<unsigned char>(s[i]);
            out += "\\x";
            out += lowerhex[b >> 4];
            out += lowerhex[b & 0xF];
            i++;
            continue;
        }
        append_escaped_rune(out, r, '"', ascii_only);
        i += width;
    }
    out += '"';
}

// append_quoted_rune is strconv.AppendQuoteRune (AppendQuoteRuneToASCII when ascii_only).
void append_quoted_rune(std::string& out, char32_t r, bool ascii_only) {
    out += '\'';
    if (!valid_rune(r)) {
        r = rune_error;
    }
    append_escaped_rune(out, r, '\'', ascii_only);
    out += '\'';
}

// can_backquote is strconv.CanBackquote.
bool can_backquote(std::string_view s) noexcept {
    for (std::size_t i = 0; i < s.size();) {
        auto [r, width] = decode(s.substr(i));
        i += width;
        if (width > 1) {
            if (r == 0xFEFF) {
                return false; // BOMs are invisible and should not be quoted.
            }
            continue; // All other multibyte runes are correctly encoded and assumed printable.
        }
        if (r == rune_error) {
            return false;
        }
        if ((r < ' ' && r != '\t') || r == '`' || r == 0x7F) {
            return false;
        }
    }
    return true;
}

// --- strconv: floats ---

// fmt_e is strconv's fmtE: [-]d.ddddde±dd.
void fmt_e(std::string& out, bool neg, const float_digits& d, int prec, char fmt) {
    int nd = d.nd;
    if (neg) {
        out += '-';
    }
    out += nd == 0 ? '0' : d.d[0];
    if (prec > 0) {
        out += '.';
        int i = 1;
        int m = std::min(nd, prec + 1);
        if (i < m) {
            out.append(d.d + i, static_cast<std::size_t>(m - i));
            i = m;
        }
        for (; i <= prec; i++) {
            out += '0';
        }
    }
    out += fmt;
    int exp = nd == 0 ? 0 : d.dp - 1;
    if (exp < 0) {
        out += '-';
        exp = -exp;
    } else {
        out += '+';
    }
    if (exp < 10) {
        out += '0';
        out += static_cast<char>('0' + exp);
    } else if (exp < 100) {
        out += static_cast<char>('0' + exp / 10);
        out += static_cast<char>('0' + exp % 10);
    } else {
        out += static_cast<char>('0' + exp / 100);
        out += static_cast<char>('0' + exp / 10 % 10);
        out += static_cast<char>('0' + exp % 10);
    }
}

// fmt_f is strconv's fmtF: [-]ddd.ddddd.
void fmt_f(std::string& out, bool neg, const float_digits& d, int prec) {
    int nd = d.nd;
    if (neg) {
        out += '-';
    }
    if (d.dp > 0) {
        int m = std::min(nd, d.dp);
        out.append(d.d, static_cast<std::size_t>(m));
        for (; m < d.dp; m++) {
            out += '0';
        }
    } else {
        out += '0';
    }
    if (prec > 0) {
        out += '.';
        for (int i = 0; i < prec; i++) {
            int j = d.dp + i;
            out += 0 <= j && j < nd ? d.d[j] : '0';
        }
    }
}

// fmt_b is strconv's fmtB: -ddddddddp±ddd.
void fmt_b(std::string& out, bool neg, uint64_t mant, int exp, int mantbits) {
    if (neg) {
        out += '-';
    }
    out += std::to_string(mant);
    out += 'p';
    exp -= mantbits;
    if (exp >= 0) {
        out += '+';
    }
    out += std::to_string(exp);
}

// fmt_x is strconv's fmtX: -0x1.yyyyyyyyp±ddd or -0x0p+0.
void fmt_x(std::string& out, int prec, char fmt, bool neg, uint64_t mant, int exp, int mantbits) {
    if (mant == 0) {
        exp = 0;
    }
    // Shift digits so leading 1 (if any) is at bit 1<<60.
    mant <<= 60 - mantbits;
    while (mant != 0 && (mant & (uint64_t(1) << 60)) == 0) {
        mant <<= 1;
        exp--;
    }
    // Round if requested.
    if (prec >= 0 && prec < 15) {
        auto shift = static_cast<unsigned>(prec * 4);
        uint64_t extra = (mant << shift) & ((uint64_t(1) << 60) - 1);
        mant >>= 60 - shift;
        if ((extra | (mant & 1)) > (uint64_t(1) << 59)) {
            mant++;
        }
        mant <<= 60 - shift;
        if ((mant & (uint64_t(1) << 61)) != 0) {
            // Wrapped around.
            mant >>= 1;
            exp++;
        }
    }
    const char* hex = fmt == 'X' ? "0123456789ABCDEF" : "0123456789abcdef";
    // sign, 0x, leading digit
    if (neg) {
        out += '-';
    }
    out += '0';
    out += fmt;
    out += static_cast<char>('0' + ((mant >> 60) & 1));
    // .fraction
    mant <<= 4; // remove leading 0 or 1
    if (prec < 0 && mant != 0) {
        out += '.';
        while (mant != 0) {
            out += hex[(mant >> 60) & 15];
            mant <<= 4;
        }
    } else if (prec > 0) {
        out += '.';
        for (int i = 0; i < prec; i++) {
            out += hex[(mant >> 60) & 15];
            mant <<= 4;
        }
    }
    // p±
    out += fmt == 'X' ? 'P' : 'p';
    if (exp < 0) {
        out += '-';
        exp = -exp;
    } else {
        out += '+';
    }
    if (exp < 100) {
        out += static_cast<char>('0' + exp / 10);
        out += static_cast<char>('0' + exp % 10);
    } else if (exp < 1000) {
        out += static_cast<char>('0' + exp / 100);
        out += static_cast<char>('0' + (exp / 10) % 10);
        out += static_cast<char>('0' + exp % 10);
    } else {
        out += static_cast<char>('0' + exp / 1000);
        out += static_cast<char>('0' + (exp / 100) % 10);
        out += static_cast<char>('0' + (exp / 10) % 10);
        out += static_cast<char>('0' + exp % 10);
    }
}

} // namespace

// --- strconv.AppendFloat ---

void append_float(std::string& out, double v, char fmt, int prec, int bits) {
    if (std::isnan(v)) {
        out += "NaN";
        return;
    }
    if (std::isinf(v)) {
        out += v > 0 ? "+Inf" : "-Inf";
        return;
    }
    bool neg = std::signbit(v);
    const float_info& flt = bits == 32 ? float32_info : float64_info;

    // The bits of the float: the mantissa with its implicit top bit, and the exponent.
    uint64_t mant;
    int exp;
    if (bits == 32) {
        float f = static_cast<float>(v);
        uint32_t b;
        std::memcpy(&b, &f, 4);
        exp = static_cast<int>((b >> 23) & 0xFF);
        mant = b & ((uint32_t(1) << 23) - 1);
    } else {
        uint64_t b;
        std::memcpy(&b, &v, 8);
        exp = static_cast<int>((b >> 52) & 0x7FF);
        mant = b & ((uint64_t(1) << 52) - 1);
    }
    if (exp == 0) {
        // A denormal.
        exp++;
    } else {
        mant |= uint64_t(1) << flt.mantbits;
    }
    exp += flt.bias;

    // The easy binary and hexadecimal forms.
    if (fmt == 'b') {
        fmt_b(out, neg, mant, exp, flt.mantbits);
        return;
    }
    if (fmt == 'x' || fmt == 'X') {
        fmt_x(out, prec, fmt, neg, mant, exp, flt.mantbits);
        return;
    }

    // The decimal digits, as strconv's genericFtoa picks them: Ryu for the shortest digits and
    // for up to 18 fixed digits, multiprecision arithmetic for the rest (ftoa.hpp: or the
    // same digits from std::to_chars).
    float_digits d;
    bool shortest = prec < 0;
    if (shortest) {
        if constexpr (own_ftoa) {
            shortest_digits(d, mant, exp, flt);
        } else {
            shortest_digits_std(d, mant, exp, flt);
        }
        // The precision for the shortest representation.
        switch (fmt) {
        case 'e':
        case 'E':
            prec = std::max(d.nd - 1, 0);
            break;
        case 'f':
            prec = std::max(d.nd - d.dp, 0);
            break;
        default:
            prec = d.nd;
            break;
        }
    } else {
        if (fmt != 'f' && fmt != 'e' && fmt != 'E' && prec == 0) {
            // %g with precision 0 rounds to one digit.
            prec = 1;
        }
        if constexpr (own_ftoa) {
            // A fixed number of digits (Ryu), but 'f' and more than 18 digits are exact.
            int digits = fmt == 'e' || fmt == 'E' ? prec + 1 : prec;
            if (fmt == 'f' || !fixed_digits(d, mant, exp, flt, digits)) {
                exact_digits(d, mant, exp, flt, fmt, prec);
            }
        } else {
            precision_digits_std(d, mant, exp, flt, fmt, prec);
        }
    }
    int nd = d.nd;
    if (fmt == 'e' || fmt == 'E') {
        fmt_e(out, neg, d, prec, fmt);
        return;
    }
    if (fmt == 'f') {
        fmt_f(out, neg, d, prec);
        return;
    }
    // %e is used if the exponent from the conversion is less than -4 or greater than or equal
    // to the precision. If the precision was the shortest possible, use precision 6 for this
    // decision.
    int eprec = prec;
    if (eprec > nd && nd >= d.dp) {
        eprec = nd;
    }
    if (shortest) {
        eprec = 6;
    }
    int dexp = d.dp - 1;
    if (dexp < -4 || dexp >= eprec) {
        if (prec > nd) {
            prec = nd;
        }
        fmt_e(out, neg, d, prec - 1, fmt == 'g' ? 'e' : 'E');
        return;
    }
    if (prec > d.dp) {
        prec = nd;
    }
    fmt_f(out, neg, d, std::max(prec - d.dp, 0));
}

void float_v(std::string& out, double v, int bits) {
    append_float(out, v, 'g', -1, bits);
}

void float_f(std::string& out, double v) {
    append_float(out, v, 'f', 6, 64);
}

void append_quote(std::string& out, std::string_view s) {
    append_quoted(out, s, false);
}

const char* write_stdout_checked(std::string_view s) {
    // What C or C++ code printf'd before comes first (fmt.hpp): C's stdout has a buffer.
    gp::detail::flush_c_stdio();
    if (s.empty()) {
        return empty_write_error();
    }
    switch (os::write(os::stdout_fd, s.data(), s.size())) {
    case os::write_result::ok:
        return nullptr;
    case os::write_result::broken_pipe:
        // fmt writes through os.Stdout, which ends the program with SIGPIPE on a broken pipe.
        os::die_of_sigpipe();
    default:
        return os::write_error();
    }
}

void write_stdout(std::string_view s) {
    write_stdout_checked(s);
}

void write_stderr(std::string_view s) {
    gp::detail::flush_c_stdio();
    if (!s.empty() &&
        os::write(os::stderr_fd, s.data(), s.size()) == os::write_result::broken_pipe) {
        os::die_of_sigpipe();
    }
}

void cxx_type_name(std::string& out, const std::type_info& t) {
    // g++ starts the name of a type that is local to a file with '*'.
    std::string name = t.name();
    if (!name.empty() && name[0] == '*') {
        name.erase(0, 1);
    }
    std::string d = os::demangle(name);
    out += d.empty() ? name : d;
}

namespace {

// --- reading values ---

// below: the access of an element, key or value of a value reached with ro.
constexpr access below(access ro) noexcept {
    return ro == access::open ? access::open : access::sticky_ro;
}

// field_access: the access of a field, as reflect's Value.Field gives it.
constexpr access field_access(access ro, bool exported, bool embedded) noexcept {
    if (ro == access::sticky_ro) {
        return access::sticky_ro;
    }
    if (!exported) {
        return embedded ? access::embed_ro : access::sticky_ro;
    }
    return access::open;
}

constexpr bool too_large(int64_t x) noexcept {
    return x > 1000000 || x < -1000000;
}

// type_string is reflect's Type.String of the type, as the error forms and %T show it.
std::string_view type_string(const type_desc* t) {
    if (t->gp_kind == kind::Foreign && t->gp_ops != nullptr && t->gp_ops->gp_go_name != nullptr) {
        return t->gp_ops->gp_go_name;
    }
    return t->gp_name;
}

// The value of a signed kind, as the uint64 that fmtInteger takes.
uint64_t read_int(kind k, const void* p) {
    switch (k) {
    case kind::Int8:
        return static_cast<uint64_t>(static_cast<int64_t>(*static_cast<const int8_t*>(p)));
    case kind::Int16:
        return static_cast<uint64_t>(static_cast<int64_t>(*static_cast<const int16_t*>(p)));
    case kind::Int32:
        return static_cast<uint64_t>(static_cast<int64_t>(*static_cast<const int32_t*>(p)));
    default:
        return static_cast<uint64_t>(*static_cast<const int64_t*>(p));
    }
}

uint64_t read_uint(kind k, const void* p) {
    switch (k) {
    case kind::Uint8:
        return *static_cast<const uint8_t*>(p);
    case kind::Uint16:
        return *static_cast<const uint16_t*>(p);
    case kind::Uint32:
        return *static_cast<const uint32_t*>(p);
    default:
        return *static_cast<const uint64_t*>(p);
    }
}

// read_sized reads an integer of size bytes (signed or not) of a C/C++ type.
uint64_t read_sized(bool is_signed, std::size_t size, const void* p) {
    switch (size) {
    case 1:
        return is_signed
                   ? static_cast<uint64_t>(static_cast<int64_t>(*static_cast<const int8_t*>(p)))
                   : *static_cast<const uint8_t*>(p);
    case 2:
        return is_signed
                   ? static_cast<uint64_t>(static_cast<int64_t>(*static_cast<const int16_t*>(p)))
                   : *static_cast<const uint16_t*>(p);
    case 4:
        return is_signed
                   ? static_cast<uint64_t>(static_cast<int64_t>(*static_cast<const int32_t*>(p)))
                   : *static_cast<const uint32_t*>(p);
    default:
        return *static_cast<const uint64_t*>(p);
    }
}

// The address as a number, for the verbs of a pointer.
uint64_t addr_value(const void* p) noexcept {
    return static_cast<uint64_t>(reinterpret_cast<std::uintptr_t>(p));
}

bool is_signed_int(kind k) noexcept {
    return k >= kind::Int && k <= kind::Int64;
}
bool is_unsigned_int(kind k) noexcept {
    return k >= kind::Uint && k <= kind::Uintptr;
}

// The value inside an interface-kind value at p, which stays alive while it prints.
value elem_of_iface(const void* p) {
    return from_iface(*static_cast<const iface*>(p));
}

// may_run_code: printing a value of type t can call a method (so something can change while it
// prints); the numbers, bools and strings without methods can't.
bool may_run_code(const type_desc* t) noexcept {
    if (t->gp_nmethods != 0) {
        return true;
    }
    switch (t->gp_kind) {
    case kind::Bool:
    case kind::Int:
    case kind::Int8:
    case kind::Int16:
    case kind::Int32:
    case kind::Int64:
    case kind::Uint:
    case kind::Uint8:
    case kind::Uint16:
    case kind::Uint32:
    case kind::Uint64:
    case kind::Uintptr:
    case kind::Float32:
    case kind::Float64:
    case kind::Complex64:
    case kind::Complex128:
    case kind::String:
    case kind::UnsafePointer:
        return false;
    default:
        return true;
    }
}

// is_string_kind: the operand is a string (Print adds no space next to one): a Go string kind,
// or a C++ string.
bool is_string_kind(const value& v) noexcept {
    if (v.t == nullptr) {
        return false;
    }
    if (v.t->gp_kind == kind::String) {
        return true;
    }
    if (v.t->gp_kind == kind::Foreign && v.t->gp_ops != nullptr) {
        fclass c = v.t->gp_ops->gp_fclass;
        return c == fclass::cxx_string || c == fclass::cxx_string_view;
    }
    return false;
}

// --- fmtsort ---

int cmp_int(int64_t a, int64_t b) noexcept {
    return a < b ? -1 : a > b ? 1 : 0;
}
int cmp_uint(uint64_t a, uint64_t b) noexcept {
    return a < b ? -1 : a > b ? 1 : 0;
}
// cmp.Compare of floats: a NaN is less than any other number and equal to a NaN; -0 == +0.
int cmp_float(double a, double b) noexcept {
    bool an = a != a, bn = b != b;
    if (an || bn) {
        return an && bn ? 0 : an ? -1 : 1;
    }
    return a < b ? -1 : a > b ? 1 : 0;
}

// field_value: the value of field f of the struct at p, unwrapping a non-nil interface as
// printf's getField does.
value field_value(const void* p, const field_desc& f) {
    const void* obj = static_cast<const char*>(p) + f.gp_offset;
    if (f.gp_type->gp_kind == kind::Interface) {
        value e = elem_of_iface(obj);
        if (e.t != nullptr) {
            return e;
        }
    }
    return of_object(f.gp_type, obj);
}

// compare is internal/fmtsort's compare: -1, 0 or +1.
int compare(const value& a, const value& b) {
    if (a.t != b.t) {
        return -1; // No good answer possible, but don't return 0: they're not equal.
    }
    const type_desc* t = a.t;
    switch (t->gp_kind) {
    case kind::Bool: {
        bool x = *static_cast<const bool*>(a.p), y = *static_cast<const bool*>(b.p);
        return x == y ? 0 : x ? 1 : -1;
    }
    case kind::Int:
    case kind::Int8:
    case kind::Int16:
    case kind::Int32:
    case kind::Int64:
        return cmp_int(static_cast<int64_t>(read_int(t->gp_kind, a.p)),
                       static_cast<int64_t>(read_int(t->gp_kind, b.p)));
    case kind::Uint:
    case kind::Uint8:
    case kind::Uint16:
    case kind::Uint32:
    case kind::Uint64:
    case kind::Uintptr:
        return cmp_uint(read_uint(t->gp_kind, a.p), read_uint(t->gp_kind, b.p));
    case kind::String: {
        auto x = static_cast<const gp::string*>(a.p)->gp_view();
        auto y = static_cast<const gp::string*>(b.p)->gp_view();
        int c = x.compare(y);
        return c < 0 ? -1 : c > 0 ? 1 : 0;
    }
    case kind::Float32:
        return cmp_float(*static_cast<const float*>(a.p), *static_cast<const float*>(b.p));
    case kind::Float64:
        return cmp_float(*static_cast<const double*>(a.p), *static_cast<const double*>(b.p));
    case kind::Complex64: {
        auto& x = *static_cast<const gp::complex64*>(a.p);
        auto& y = *static_cast<const gp::complex64*>(b.p);
        if (int c = cmp_float(x.gp_re, y.gp_re); c != 0) {
            return c;
        }
        return cmp_float(x.gp_im, y.gp_im);
    }
    case kind::Complex128: {
        auto& x = *static_cast<const gp::complex128*>(a.p);
        auto& y = *static_cast<const gp::complex128*>(b.p);
        if (int c = cmp_float(x.gp_re, y.gp_re); c != 0) {
            return c;
        }
        return cmp_float(x.gp_im, y.gp_im);
    }
    case kind::Pointer:
    case kind::UnsafePointer:
        return cmp_uint(addr_value(a.p), addr_value(b.p));
    case kind::Chan:
        return cmp_uint(
            t->gp_ops != nullptr && t->gp_ops->gp_addr ? addr_value(t->gp_ops->gp_addr(a.p)) : 0,
            t->gp_ops != nullptr && t->gp_ops->gp_addr ? addr_value(t->gp_ops->gp_addr(b.p)) : 0);
    case kind::Struct: {
        const type_desc* ft = full(t);
        if (t->gp_ops != nullptr && t->gp_ops->gp_word != nullptr) {
            return cmp_uint(t->gp_ops->gp_word(a.p), t->gp_ops->gp_word(b.p));
        }
        for (std::size_t i = 0; i < ft->gp_nfields; i++) {
            const field_desc& f = ft->gp_fields[i];
            if (int c = compare(field_value(a.p, f), field_value(b.p, f)); c != 0) {
                return c;
            }
        }
        return 0;
    }
    case kind::Array: {
        const type_desc* ft = full(t);
        const type_desc* et = ft->gp_elem;
        const char* x = static_cast<const char*>(t->gp_ops->gp_seq_data(a.p));
        const char* y = static_cast<const char*>(t->gp_ops->gp_seq_data(b.p));
        for (int64_t i = 0; i < t->gp_ops->gp_seq_len(a.p); i++) {
            auto off = static_cast<std::size_t>(i) * et->gp_size;
            if (int c = compare(of_object(et, x + off), of_object(et, y + off)); c != 0) {
                return c;
            }
        }
        return 0;
    }
    case kind::Interface: {
        value x = elem_of_iface(a.p), y = elem_of_iface(b.p);
        if (x.t == nullptr || y.t == nullptr) {
            return x.t == nullptr && y.t == nullptr ? 0 : x.t == nullptr ? -1 : 1;
        }
        if (x.t != y.t) {
            return cmp_uint(addr_value(x.t), addr_value(y.t));
        }
        return compare(x, y);
    }
    case kind::Foreign: {
        const type_ops* ops = t->gp_ops;
        switch (ops != nullptr ? ops->gp_fclass : fclass::none) {
        case fclass::boolean: {
            bool x = *static_cast<const bool*>(a.p), y = *static_cast<const bool*>(b.p);
            return x == y ? 0 : x ? 1 : -1;
        }
        case fclass::sint:
        case fclass::enum_s:
            return cmp_int(static_cast<int64_t>(read_sized(true, t->gp_size, a.p)),
                           static_cast<int64_t>(read_sized(true, t->gp_size, b.p)));
        case fclass::uint_:
        case fclass::enum_u:
            return cmp_uint(read_sized(false, t->gp_size, a.p), read_sized(false, t->gp_size, b.p));
        case fclass::f32:
            return cmp_float(*static_cast<const float*>(a.p), *static_cast<const float*>(b.p));
        case fclass::f64:
            return cmp_float(*static_cast<const double*>(a.p), *static_cast<const double*>(b.p));
        case fclass::raw_ptr:
            return cmp_uint(addr_value(*static_cast<void* const*>(a.p)),
                            addr_value(*static_cast<void* const*>(b.p)));
        case fclass::cptr:
            return cmp_uint(addr_value(ops->gp_target(a.p)), addr_value(ops->gp_target(b.p)));
        case fclass::cxx_string: {
            int c = static_cast<const std::string*>(a.p)->compare(
                *static_cast<const std::string*>(b.p));
            return c < 0 ? -1 : c > 0 ? 1 : 0;
        }
        case fclass::cxx_string_view: {
            int c = static_cast<const std::string_view*>(a.p)->compare(
                *static_cast<const std::string_view*>(b.p));
            return c < 0 ? -1 : c > 0 ? 1 : 0;
        }
        default:
            return ops != nullptr && ops->gp_cmp != nullptr ? ops->gp_cmp(a.p, b.p) : 0;
        }
    }
    default:
        return 0; // slices, maps and funcs can't be keys
    }
}

// stable_sort sorts the indexes by less, keeping equal ones in their order: a merge sort.
template <typename Less>
void stable_sort(std::vector<std::size_t>& idx, Less less) {
    std::size_t n = idx.size();
    std::vector<std::size_t> tmp(n);
    for (std::size_t width = 1; width < n; width *= 2) {
        for (std::size_t lo = 0; lo < n; lo += 2 * width) {
            std::size_t mid = lo + width < n ? lo + width : n;
            std::size_t hi = lo + 2 * width < n ? lo + 2 * width : n;
            std::size_t i = lo, j = mid, k = lo;
            while (i < mid && j < hi) {
                tmp[k++] = less(idx[j], idx[i]) ? idx[j++] : idx[i++];
            }
            while (i < mid) {
                tmp[k++] = idx[i++];
            }
            while (j < hi) {
                tmp[k++] = idx[j++];
            }
        }
        idx.swap(tmp);
    }
}

// parsenum: the number at s[start:end], whether there is one, and where it ends.
std::tuple<int, bool, std::size_t> parsenum(std::string_view s, std::size_t start,
                                            std::size_t end) {
    if (start >= end) {
        return {0, false, end};
    }
    int64_t num = 0;
    bool isnum = false;
    std::size_t i = start;
    for (; i < end && '0' <= s[i] && s[i] <= '9'; i++) {
        if (too_large(num)) {
            return {0, false, end}; // Overflow; crazy long number most likely.
        }
        num = num * 10 + (s[i] - '0');
        isnum = true;
    }
    return {static_cast<int>(num), isnum, i};
}

// int_from_arg: the integer argument a[n], for a * width or precision.
std::tuple<int, bool, std::size_t> int_from_arg(const value* a, std::size_t na, std::size_t n) {
    if (n >= na) {
        return {0, false, n};
    }
    int64_t num = 0;
    bool is_int = false;
    const value& v = a[n];
    if (v.t != nullptr) {
        kind k = v.t->gp_kind;
        if (is_signed_int(k)) {
            num = static_cast<int64_t>(read_int(k, v.p));
            is_int = true;
        } else if (is_unsigned_int(k)) {
            uint64_t u = read_uint(k, v.p);
            if (u <= static_cast<uint64_t>(INT64_MAX)) {
                num = static_cast<int64_t>(u);
                is_int = true;
            }
        }
    }
    if (too_large(num)) {
        num = 0;
        is_int = false;
    }
    return {static_cast<int>(num), is_int, n + 1};
}

// parse_arg_number: the index in a bracketed number [n] at the start of format, minus 1, the
// bytes up to the closing bracket, and whether the number parsed.
std::tuple<int, std::size_t, bool> parse_arg_number(std::string_view format) {
    if (format.size() < 3) {
        return {0, 1, false};
    }
    for (std::size_t i = 1; i < format.size(); i++) {
        if (format[i] == ']') {
            auto [width, ok, newi] = parsenum(format, 1, i);
            if (!ok || newi != i) {
                return {0, i + 1, false};
            }
            return {width - 1, i + 1, true};
        }
    }
    return {0, 1, false};
}

struct pp;

// The state that a Format method gets: fmt.State over a pp.
struct pp_state {
    pp* p;
};

State make_state(pp& p);

// pp is Go's pp: the output buffer, the flags of the directive being printed (its fmt), and the
// state of the printer.
struct pp {
    explicit pp(std::string& out) : buf(out) {}
    pp(const pp&) = delete;
    pp& operator=(const pp&) = delete;
    // A State that a Format method kept outlives the call: it stops pointing at the printer.
    [[gnu::always_inline]] ~pp() {
        if (state != nullptr) [[unlikely]] {
            release_state();
        }
    }

    std::string& buf;
    // The State that the Format methods of this printer get (make_state): one for the whole call,
    // made by the first of them, so that every Format sees the same State, as in Go.
    // (A raw pointer to a pointer: a gp::ptr member would put its destructor on every path.)
    gp::ptr<pp_state>* state = nullptr;
    [[gnu::noinline]] void release_state() noexcept;

    // fmt's flags.
    bool wid_present = false;
    bool prec_present = false;
    bool minus = false;
    bool plus = false;
    bool sharp = false;
    bool space = false;
    bool zero = false;
    bool plus_v = false;
    bool sharp_v = false;
    int wid = 0;
    int prec = 0;

    bool reordered = false;
    bool good_arg_num = true;
    bool panicking = false;
    bool erroring = false;
    bool wrap_errs = false;
    live_span lv; // the []any of a call f(a...), read while printing (live_span)
    std::vector<int> wrapped_errs;

    // The current operand, for badVerb (Go's arg and value).
    const value* cur = nullptr;

    void clearflags() noexcept {
        wid_present = prec_present = minus = plus = sharp = space = zero = plus_v = sharp_v = false;
        wid = prec = 0;
    }

    // --- format.go ---

    void write_padding(int64_t n) {
        if (n <= 0) {
            return;
        }
        // Zero padding is allowed only to the left.
        buf.append(static_cast<std::size_t>(n), zero && !minus ? '0' : ' ');
    }

    void pad(std::string_view s) {
        if (!wid_present || wid == 0) {
            buf += s;
            return;
        }
        int64_t width = wid - rune_count(s);
        if (!minus) {
            write_padding(width); // left padding
            buf += s;
        } else {
            buf += s;
            write_padding(width); // right padding
        }
    }

    void fmt_boolean(bool v) { pad(v ? "true" : "false"); }

    // fmt_unicode formats a uint64 as "U+0078" or with sharp set as "U+0078 'x'".
    void fmt_unicode(uint64_t u) {
        std::string tail;
        int digits_wanted = 4;
        if (prec_present && prec > 4) {
            digits_wanted = prec;
        }
        // For %#U we want to add a space and a quoted character at the end of the buffer.
        if (sharp && u <= max_rune && is_print(static_cast<char32_t>(u))) {
            tail = " '";
            append_rune(tail, static_cast<char32_t>(u));
            tail += '\'';
        }
        std::string digits;
        while (u >= 16) {
            digits += udigits[u & 0xF];
            digits_wanted--;
            u >>= 4;
        }
        digits += udigits[u];
        digits_wanted--;
        // Add zeros in front of the number until requested precision is reached.
        while (digits_wanted > 0) {
            digits += '0';
            digits_wanted--;
        }
        std::string s = "U+";
        s.append(digits.rbegin(), digits.rend());
        s += tail;
        bool old_zero = zero;
        zero = false;
        pad(s);
        zero = old_zero;
    }

    // fmt_integer is fmt.fmtInteger.
    void fmt_integer(uint64_t u, int base, bool is_signed, char32_t verb, const char* digits) {
        bool negative = is_signed && static_cast<int64_t>(u) < 0;
        if (negative) {
            u = -u;
        }
        char small[68];
        std::string big;
        char* b = small;
        std::size_t len = sizeof small;
        if (wid_present || prec_present) {
            // Room for a sign and "0x".
            auto width = 3 + static_cast<std::size_t>(wid) + static_cast<std::size_t>(prec);
            if (width > len) {
                big.resize(width);
                b = big.data();
                len = width;
            }
        }
        // Two ways to ask for extra leading zero digits: %.3d or %03d. If both are given, the
        // zero flag is ignored and the padding is spaces.
        int64_t want = 0;
        if (prec_present) {
            want = prec;
            // Precision 0 and value 0 means "print nothing" but padding.
            if (want == 0 && u == 0) {
                bool old_zero = zero;
                zero = false;
                write_padding(wid);
                zero = old_zero;
                return;
            }
        } else if (zero && !minus && wid_present) { // Zero padding is allowed only to the left.
            want = wid;
            if (negative || plus || space) {
                want--; // leave room for sign
            }
        }
        std::size_t i = len;
        switch (base) {
        case 10:
            while (u >= 10) {
                uint64_t next = u / 10;
                b[--i] = static_cast<char>('0' + u - next * 10);
                u = next;
            }
            break;
        case 16:
            while (u >= 16) {
                b[--i] = digits[u & 0xF];
                u >>= 4;
            }
            break;
        case 8:
            while (u >= 8) {
                b[--i] = static_cast<char>('0' + (u & 7));
                u >>= 3;
            }
            break;
        default: // 2
            while (u >= 2) {
                b[--i] = static_cast<char>('0' + (u & 1));
                u >>= 1;
            }
        }
        b[--i] = digits[u];
        while (i > 0 && want > static_cast<int64_t>(len - i)) {
            b[--i] = '0';
        }
        // Various prefixes: 0x, -, etc.
        if (sharp) {
            switch (base) {
            case 2:
                b[--i] = 'b';
                b[--i] = '0';
                break;
            case 8:
                if (b[i] != '0') {
                    b[--i] = '0';
                }
                break;
            case 16:
                b[--i] = digits[16];
                b[--i] = '0';
                break;
            default:
                break;
            }
        }
        if (verb == 'O') {
            b[--i] = 'o';
            b[--i] = '0';
        }
        if (negative) {
            b[--i] = '-';
        } else if (plus) {
            b[--i] = '+';
        } else if (space) {
            b[--i] = ' ';
        }
        // Left padding with zeros has already been handled like precision earlier or the zero
        // flag is ignored due to an explicitly set precision.
        bool old_zero = zero;
        zero = false;
        pad(std::string_view(b + i, len - i));
        zero = old_zero;
    }

    // truncate_string is fmt.truncateString: s cut to the precision, in runes.
    std::string_view truncate_string(std::string_view s) const noexcept {
        if (prec_present) {
            int64_t n = prec;
            for (std::size_t i = 0; i < s.size();) {
                n--;
                if (n < 0) {
                    return s.substr(0, i);
                }
                i += decode(s.substr(i)).second;
            }
        }
        return s;
    }

    void fmt_s(std::string_view s) { pad(truncate_string(s)); }

    // fmt_sbx formats a string or byte slice as a hexadecimal encoding of its bytes.
    void fmt_sbx(std::string_view s, const char* digits) {
        auto length = static_cast<int64_t>(s.size());
        // Set length to not process more bytes than the precision demands.
        if (prec_present && prec < length) {
            length = prec;
        }
        // Compute width of the encoding taking into account the sharp and space flag.
        int64_t width = 2 * length;
        if (width > 0) {
            if (space) {
                // Each element encoded by two hexadecimals will get a leading 0x or 0X.
                if (sharp) {
                    width *= 2;
                }
                // Elements will be separated by a space.
                width += length - 1;
            } else if (sharp) {
                // Only a leading 0x or 0X will be added for the whole string.
                width += 2;
            }
        } else { // The byte slice or string that should be encoded is empty.
            if (wid_present) {
                write_padding(wid);
            }
            return;
        }
        // Handle padding to the left.
        if (wid_present && wid > width && !minus) {
            write_padding(wid - width);
        }
        if (sharp) {
            // Add leading 0x or 0X.
            buf += '0';
            buf += digits[16];
        }
        for (int64_t i = 0; i < length; i++) {
            if (space && i > 0) {
                // Separate elements with a space.
                buf += ' ';
                if (sharp) {
                    // Add leading 0x or 0X for each element.
                    buf += '0';
                    buf += digits[16];
                }
            }
            auto c = static_cast<unsigned char>(s[static_cast<std::size_t>(i)]);
            buf += digits[c >> 4];
            buf += digits[c & 0xF];
        }
        // Handle padding to the right.
        if (wid_present && wid > width && minus) {
            write_padding(wid - width);
        }
    }

    // fmt_q is fmt.fmtQ.
    void fmt_q(std::string_view s) {
        s = truncate_string(s);
        if (sharp && can_backquote(s)) {
            std::string q;
            q += '`';
            q += s;
            q += '`';
            pad(q);
            return;
        }
        std::string q;
        append_quoted(q, s, plus);
        pad(q);
    }

    // fmt_c is fmt.fmtC: an integer as a character.
    void fmt_c(uint64_t c) {
        auto r = static_cast<char32_t>(c);
        if (c > max_rune) {
            r = rune_error;
        }
        std::string q;
        append_rune(q, r);
        pad(q);
    }

    // fmt_qc is fmt.fmtQc: an integer as a quoted rune.
    void fmt_qc(uint64_t c) {
        auto r = static_cast<char32_t>(c);
        if (c > max_rune) {
            r = rune_error;
        }
        std::string q;
        append_quoted_rune(q, r, plus);
        pad(q);
    }

    // fmt_float is fmt.fmtFloat.
    void fmt_float(double v, int size, char32_t verb, int precision) {
        // Explicit precision in format specifier overrules default precision.
        if (prec_present) {
            precision = prec;
        }
        // Format number, reserving space for leading + sign if needed.
        std::string num(1, '+');
        append_float(num, v, static_cast<char>(verb), precision, size);
        if (num[1] == '-' || num[1] == '+') {
            num.erase(0, 1);
        }
        // space means to add a leading space instead of a "+" sign unless the sign is
        // explicitly asked for by plus.
        if (space && num[0] == '+' && !plus) {
            num[0] = ' ';
        }
        // Special handling for infinities and NaN, which don't look like a number so shouldn't
        // be padded with zeros.
        if (num[1] == 'I' || num[1] == 'N') {
            bool old_zero = zero;
            zero = false;
            // Remove sign before NaN if not asked for.
            if (num[1] == 'N' && !space && !plus) {
                num.erase(0, 1);
            }
            pad(num);
            zero = old_zero;
            return;
        }
        // The sharp flag forces printing a decimal point for non-binary formats and retains
        // trailing zeros, which we may need to restore.
        if (sharp && verb != 'b') {
            int digits = 0;
            switch (verb) {
            case 'v':
            case 'g':
            case 'G':
            case 'x':
                digits = precision;
                // If no precision is set explicitly use a precision of 6.
                if (digits == -1) {
                    digits = 6;
                }
                break;
            default:
                break;
            }
            std::string tail;
            bool has_decimal_point = false;
            bool saw_nonzero_digit = false;
            // Starting from i = 1 to skip sign at num[0].
            for (std::size_t i = 1; i < num.size(); i++) {
                switch (num[i]) {
                case '.':
                    has_decimal_point = true;
                    break;
                case 'p':
                case 'P':
                    tail.append(num, i);
                    num.resize(i);
                    break;
                case 'e':
                case 'E':
                    if (verb != 'x' && verb != 'X') {
                        tail.append(num, i);
                        num.resize(i);
                        break;
                    }
                    [[fallthrough]];
                default:
                    if (num[i] != '0') {
                        saw_nonzero_digit = true;
                    }
                    // Count significant digits after the first non-zero digit.
                    if (saw_nonzero_digit) {
                        digits--;
                    }
                }
            }
            if (!has_decimal_point) {
                // Leading digit 0 should contribute once to digits.
                if (num.size() == 2 && num[1] == '0') {
                    digits--;
                }
                num += '.';
            }
            while (digits > 0) {
                num += '0';
                digits--;
            }
            num += tail;
        }
        // We want a sign if asked for and if the sign is not positive.
        if (plus || num[0] != '+') {
            // If we're zero padding to the left we want the sign before the leading zeros.
            // Achieve this by writing the sign out and then padding the unsigned number.
            // Zero padding is allowed only to the left.
            if (zero && !minus && wid_present && wid > static_cast<int64_t>(num.size())) {
                buf += num[0];
                write_padding(wid - static_cast<int64_t>(num.size()));
                buf.append(num, 1);
                return;
            }
            pad(num);
            return;
        }
        // No sign to show and the number is positive; just print the unsigned number.
        pad(std::string_view(num).substr(1));
    }

    // --- print.go ---

    void fmt0x64(uint64_t v, bool leading0x) {
        bool old = sharp;
        sharp = leading0x;
        fmt_integer(v, 16, false, 'v', ldigits);
        sharp = old;
    }
    void pfmt_bool(bool v, char32_t verb) {
        switch (verb) {
        case 't':
        case 'v':
            fmt_boolean(v);
            break;
        default:
            bad_verb(verb);
        }
    }
    void pfmt_integer(uint64_t v, bool is_signed, char32_t verb) {
        switch (verb) {
        case 'v':
            if (sharp_v && !is_signed) {
                fmt0x64(v, true);
            } else {
                fmt_integer(v, 10, is_signed, verb, ldigits);
            }
            break;
        case 'd':
            fmt_integer(v, 10, is_signed, verb, ldigits);
            break;
        case 'b':
            fmt_integer(v, 2, is_signed, verb, ldigits);
            break;
        case 'o':
        case 'O':
            fmt_integer(v, 8, is_signed, verb, ldigits);
            break;
        case 'x':
            fmt_integer(v, 16, is_signed, verb, ldigits);
            break;
        case 'X':
            fmt_integer(v, 16, is_signed, verb, udigits);
            break;
        case 'c':
            fmt_c(v);
            break;
        case 'q':
            fmt_qc(v);
            break;
        case 'U':
            fmt_unicode(v);
            break;
        default:
            bad_verb(verb);
        }
    }
    void pfmt_float(double v, int size, char32_t verb) {
        switch (verb) {
        case 'v':
            fmt_float(v, size, 'g', -1);
            break;
        case 'b':
        case 'g':
        case 'G':
        case 'x':
        case 'X':
            fmt_float(v, size, verb, -1);
            break;
        case 'f':
        case 'e':
        case 'E':
            fmt_float(v, size, verb, 6);
            break;
        case 'F':
            fmt_float(v, size, 'f', 6);
            break;
        default:
            bad_verb(verb);
        }
    }
    void pfmt_complex(double re, double im, int size, char32_t verb) {
        // Make sure any unsupported verbs are found before the calls to fmtFloat to not
        // generate an incorrect error string.
        switch (verb) {
        case 'v':
        case 'b':
        case 'g':
        case 'G':
        case 'x':
        case 'X':
        case 'f':
        case 'F':
        case 'e':
        case 'E': {
            bool old_plus = plus;
            buf += '(';
            pfmt_float(re, size / 2, verb);
            // Imaginary part always has a sign.
            plus = true;
            pfmt_float(im, size / 2, verb);
            buf += "i)";
            plus = old_plus;
            break;
        }
        default:
            bad_verb(verb);
        }
    }
    void pfmt_string(std::string_view v, char32_t verb) {
        switch (verb) {
        case 'v':
            if (sharp_v) {
                fmt_q(v);
            } else {
                fmt_s(v);
            }
            break;
        case 's':
            fmt_s(v);
            break;
        case 'x':
            fmt_sbx(v, ldigits);
            break;
        case 'X':
            fmt_sbx(v, udigits);
            break;
        case 'q':
            fmt_q(v);
            break;
        default:
            bad_verb(verb);
        }
    }

    void bad_verb(char32_t verb);
    void pfmt_bytes(std::string_view s, bool is_nil, char32_t verb, std::string_view type_name,
                    const value& whole);
    void pfmt_pointer(const value& v, char32_t verb);
    void catch_panic(const value& arg, char32_t verb, const char* method);
    bool handle_methods(const value& v, char32_t verb);
    void print_arg(const value& arg, char32_t verb);
    void print_reflect_value(const value& a, char32_t verb);
    void print_value(const value& v, char32_t verb, int depth, access ro);
    void print_foreign(const value& v, char32_t verb);
    void print_address(uint64_t u, char32_t verb);
    void print_map(const value& v, const type_desc* ft, char32_t verb, int depth, access ro);
    void print_struct(const value& v, const type_desc* ft, char32_t verb, int depth, access ro);
    void print_seq(const value& v, const type_desc* ft, char32_t verb, int depth, access ro);

    void bad_arg(char32_t verb, const char* what) {
        buf += "%!";
        append_rune(buf, verb);
        buf += what;
    }
    std::tuple<std::size_t, std::size_t, bool>
    arg_number(std::size_t arg_num, std::string_view format, std::size_t i, std::size_t num_args);
    // The operands of a call with a final a... (lv) are read again as each one is printed. The
    // reading is in these cold functions; the plain calls (almost all) run none of it.
    [[gnu::noinline, gnu::cold]] void print_live(std::size_t i, char32_t verb);
    [[gnu::noinline, gnu::cold]] std::tuple<int, bool, std::size_t> int_live(std::size_t arg_num);
    [[gnu::noinline, gnu::cold]] void extra_live(std::size_t k);
    [[gnu::noinline, gnu::cold]] void do_print_live(std::size_t n);
    [[gnu::noinline, gnu::cold]] void do_println_live(std::size_t n);
    [[gnu::noinline, gnu::cold]] void extra_args(const value* a, std::size_t n,
                                                 std::size_t arg_num);
    void print_nth(const value* a, std::size_t i, char32_t verb) {
        if (lv.data != nullptr) [[unlikely]] {
            print_live(i, verb);
            return;
        }
        print_arg(a[i], verb);
    }
    // int_arg is int_from_arg for the operand arg_num, read as it is now.
    std::tuple<int, bool, std::size_t> int_arg(const value* a, std::size_t n, std::size_t arg_num) {
        if (lv.data != nullptr && arg_num < n) [[unlikely]] {
            return int_live(arg_num);
        }
        return int_from_arg(a, n, arg_num);
    }
    void do_printf(std::string_view format, const value* a, std::size_t n);
    void do_print(const value* a, std::size_t n);
    void do_println(const value* a, std::size_t n);
};

// --- methods ---

// find_method is the method name of the descriptor's method set with the signature text sig,
// or null. Only exported methods count (the interfaces of fmt are exported).
const method* find_method(const type_desc* t, const char* name, const char* sig) noexcept {
    for (std::size_t i = 0; i < t->gp_nmethods; i++) {
        const method& m = t->gp_methods[i];
        if (m.gp_pkg == nullptr && std::strcmp(m.gp_name, name) == 0 &&
            std::strcmp(m.gp_sig, sig) == 0) {
            return &m;
        }
    }
    return nullptr;
}

// self_of is the interface value that a method's thunk takes for v: it holds v's address, with
// v's owner where it has one (a pointer must).
iface self_of(const value& v) {
    std::shared_ptr<void> data;
    if (v.own != nullptr) {
        data = std::shared_ptr<void>(v.own, const_cast<void*>(v.p));
    } else {
        data = std::shared_ptr<void>(std::shared_ptr<void>(), const_cast<void*>(v.p));
    }
    return iface(std::move(data), v.t, nullptr);
}

// call_string calls the method m (func() string) of v.
gp::string call_string(const value& v, const method* m) {
    iface self = self_of(v);
    return reinterpret_cast<thunk_t<gp::string()>*>(m->gp_fn)(self);
}

// state_pp is the printer of the State self, or null when the call that made it is over (Go's
// State is then a printer back in its pool: it reads the flags of the next call, which is a
// mistake of the program; here Width and Precision say "not set", the flags are off and Write
// discards).
pp* state_pp(const iface& self) {
    return unbox<gp::ptr<pp_state>>(self)->p;
}

bool st_flag(const iface& self, int64_t b) {
    pp* pq = state_pp(self);
    if (pq == nullptr) {
        return false;
    }
    pp& p = *pq;
    switch (b) {
    case '-':
        return p.minus;
    case '+':
        return p.plus || p.plus_v;
    case '#':
        return p.sharp || p.sharp_v;
    case ' ':
        return p.space;
    case '0':
        return p.zero;
    default:
        return false;
    }
}
std::tuple<int64_t, bool> st_precision(const iface& self) {
    pp* p = state_pp(self);
    if (p == nullptr) {
        return {0, false};
    }
    return {p->prec, p->prec_present};
}
std::tuple<int64_t, bool> st_width(const iface& self) {
    pp* p = state_pp(self);
    if (p == nullptr) {
        return {0, false};
    }
    return {p->wid, p->wid_present};
}
std::tuple<int64_t, gp::error> st_write(const iface& self, gp::slice<uint8_t> b) {
    pp* p = state_pp(self);
    if (p != nullptr && b.gp_len() > 0) {
        p->buf.append(reinterpret_cast<const char*>(b.gp_data()),
                      static_cast<std::size_t>(b.gp_len()));
    }
    return {b.gp_len(), gp::error()};
}
std::tuple<int64_t, gp::error> st_write_string(const iface& self, gp::string s) {
    pp* p = state_pp(self);
    if (p != nullptr) {
        p->buf += s.gp_view();
    }
    return {static_cast<int64_t>(s.gp_view().size()), gp::error()};
}

// *fmt.pp: the dynamic type of the State that a Format method gets.
const type_desc* td_pp_ptr() {
    static const method pmethods[] = {
        method_of<bool(int64_t)>("Flag", nullptr, "func(int) bool", &st_flag),
        method_of<std::tuple<int64_t, bool>()>("Precision", nullptr, "func() (int, bool)",
                                               &st_precision),
        method_of<std::tuple<int64_t, bool>()>("Width", nullptr, "func() (int, bool)", &st_width),
        method_of<std::tuple<int64_t, gp::error>(gp::slice<uint8_t>)>(
            "Write", nullptr, "func([]uint8) (int, error)", &st_write),
        method_of<std::tuple<int64_t, gp::error>(gp::string)>(
            "WriteString", nullptr, "func(string) (int, error)", &st_write_string),
    };
    static const type_desc td =
        named<pp_state>("fmt.pp", "fmt", shape_struct(nullptr, 0), {}, methods_of(pmethods));
    static const type_desc* const p = td_ptr<pp_state>(&td);
    return p;
}

State make_state(pp& p) {
    if (p.state == nullptr) {
        p.state = new gp::ptr<pp_state>(gp::new_<pp_state>(pp_state{&p}));
    }
    return to_iface<State>(*p.state, td_pp_ptr());
}

void pp::release_state() noexcept {
    (*state)->p = nullptr; // the States that Format methods kept stop pointing at this printer
    delete state;
    state = nullptr;
}

void pp::bad_verb(char32_t verb) {
    erroring = true;
    buf += "%!";
    append_rune(buf, verb);
    buf += '(';
    if (cur != nullptr && cur->t != nullptr) {
        value c = *cur;
        buf += type_string(c.t);
        buf += '=';
        print_arg(c, 'v');
    } else {
        buf += "<nil>";
    }
    buf += ')';
    erroring = false;
}

// pfmt_bytes is fmtBytes: the bytes s of a []byte (nil: is_nil), whole being the slice itself.
void pp::pfmt_bytes(std::string_view s, bool is_nil, char32_t verb, std::string_view type_name,
                    const value& whole) {
    switch (verb) {
    case 'v':
    case 'd':
        if (sharp_v) {
            buf += type_name;
            if (is_nil) {
                buf += "(nil)";
                return;
            }
            buf += '{';
            for (std::size_t i = 0; i < s.size(); i++) {
                if (i > 0) {
                    buf += ", ";
                }
                fmt0x64(static_cast<unsigned char>(s[i]), true);
            }
            buf += '}';
        } else {
            buf += '[';
            for (std::size_t i = 0; i < s.size(); i++) {
                if (i > 0) {
                    buf += ' ';
                }
                fmt_integer(static_cast<unsigned char>(s[i]), 10, false, verb, ldigits);
            }
            buf += ']';
        }
        break;
    case 's':
        fmt_s(s);
        break;
    case 'x':
        fmt_sbx(s, ldigits);
        break;
    case 'X':
        fmt_sbx(s, udigits);
        break;
    case 'q':
        fmt_q(s);
        break;
    default:
        print_value(whole, verb, 0, access::open);
    }
}

// pfmt_pointer is fmtPointer.
void pp::pfmt_pointer(const value& v, char32_t verb) {
    uint64_t u = 0;
    const type_ops* ops = v.t->gp_ops;
    switch (v.t->gp_kind) {
    case kind::Pointer:
    case kind::UnsafePointer:
        u = addr_value(v.p);
        break;
    case kind::Slice:
        u = addr_value(ops->gp_seq_data(v.p));
        break;
    case kind::Map:
        u = addr_value(ops->gp_nil(v.p) ? nullptr : v.p);
        break;
    case kind::Func:
        u = ops->gp_nil(v.p) ? 0 : addr_value(ops->gp_addr(v.p));
        break;
    case kind::Chan:
        u = ops != nullptr && ops->gp_addr != nullptr ? addr_value(ops->gp_addr(v.p)) : 0;
        break;
    default:
        bad_verb(verb);
        return;
    }
    switch (verb) {
    case 'v':
        if (sharp_v) {
            buf += '(';
            buf += type_string(v.t);
            buf += ")(";
            if (u == 0) {
                buf += "nil";
            } else {
                fmt0x64(u, true);
            }
            buf += ')';
        } else {
            if (u == 0) {
                pad("<nil>");
            } else {
                fmt0x64(u, !sharp);
            }
        }
        break;
    case 'p':
        fmt0x64(u, !sharp);
        break;
    case 'b':
    case 'o':
    case 'd':
    case 'x':
    case 'X':
        pfmt_integer(u, false, verb);
        break;
    default:
        bad_verb(verb);
    }
}

// catch_panic is fmt's catchPanic, called in the handler of the panic that a method of arg raised.
void pp::catch_panic(const value& arg, char32_t verb, const char* method) {
    // fmt recovers the panic, which clears the goroutine's signal (gocpp/panic.hpp).
    gp::detail::clear_signal();
    // If it's a nil pointer, just say "<nil>". The likeliest causes are a Stringer that fails to
    // guard against nil or a nil pointer for a value receiver, and in either case, "<nil>" is a
    // nice result.
    if (arg.t != nullptr && arg.t->gp_kind == kind::Pointer && arg.p == nullptr) {
        buf += "<nil>";
        return;
    }
    // Otherwise print a concise panic message. Most of the time the panic value will print
    // itself nicely.
    if (panicking) {
        // Nested panics; the recursion in printArg cannot succeed.
        throw;
    }
    bool o_wid_present = wid_present, o_prec_present = prec_present, o_minus = minus, o_plus = plus,
         o_sharp = sharp, o_space = space, o_zero = zero, o_plus_v = plus_v, o_sharp_v = sharp_v;
    // For this output we want default behavior.
    clearflags();
    buf += "%!";
    append_rune(buf, verb);
    buf += "(PANIC=";
    buf += method;
    buf += " method: ";
    panicking = true;
    try {
        throw;
    } catch (const gp::panic_error& e) {
        value pv = from_iface(e.gp_value);
        print_arg(pv, 'v');
    } catch (const std::exception&) {
        // A C++ exception from the code under a method or an operator<<: it is a Go panic
        // with a runtime.CppException value (recover() gives the same).
        gp::any ev = gp::detail::cxx_exception_value(std::current_exception());
        value pv = from_iface(ev);
        print_arg(pv, 'v');
    }
    panicking = false;
    buf += ')';
    wid_present = o_wid_present;
    prec_present = o_prec_present;
    minus = o_minus;
    plus = o_plus;
    sharp = o_sharp;
    space = o_space;
    zero = o_zero;
    plus_v = o_plus_v;
    sharp_v = o_sharp_v;
}

bool pp::handle_methods(const value& vin, char32_t verb) {
    if (erroring) {
        return false;
    }
    value dyn;
    const value* v = &vin;
    if (vin.t != nullptr && vin.t->gp_kind == kind::Interface) {
        // An interface-kind value: the methods are the dynamic value's.
        dyn = elem_of_iface(vin.p);
        v = &dyn;
    }
    struct restore {
        pp& p;
        const value* saved;
        ~restore() { p.cur = saved; }
    } r{*this, cur};
    cur = v->t != nullptr ? v : nullptr;
    if (verb == 'w') {
        // It is invalid to use %w other than with Errorf or with a non-error arg.
        bool ok = v->t != nullptr && find_method(v->t, "Error", "func() string") != nullptr;
        if (!ok || !wrap_errs) {
            bad_verb(verb);
            return true;
        }
        // If the arg is a Formatter, pass 'v' as the verb to it.
        verb = 'v';
    }
    if (v->t == nullptr || v->t->gp_nmethods == 0) {
        return false;
    }
    // Is it a Formatter?
    if (const method* m = find_method(v->t, "Format", "func(fmt.State, int32)"); m != nullptr) {
        try {
            iface self = self_of(*v);
            State st = make_state(*this);
            reinterpret_cast<thunk_t<void(State, int32_t)>*>(m->gp_fn)(self, std::move(st),
                                                                       static_cast<int32_t>(verb));
        } catch (const gp::panic_error&) {
            catch_panic(*v, verb, "Format");
        } catch (const std::exception&) {
            catch_panic(*v, verb, "Format");
        }
        return true;
    }
    // If we're doing Go syntax and the argument knows how to supply it, take care of it now.
    if (sharp_v) {
        if (const method* m = find_method(v->t, "GoString", "func() string"); m != nullptr) {
            try {
                gp::string s = call_string(*v, m);
                // Print the result of GoString unadorned.
                fmt_s(s.gp_view());
            } catch (const gp::panic_error&) {
                catch_panic(*v, verb, "GoString");
            } catch (const std::exception&) {
                catch_panic(*v, verb, "GoString");
            }
            return true;
        }
        return false;
    }
    // If a string is acceptable according to the format, see if the value satisfies one of the
    // string-valued interfaces. Println etc. set verb to %v, which is "stringable".
    switch (verb) {
    case 'v':
    case 's':
    case 'x':
    case 'X':
    case 'q': {
        const char* name = "Error";
        const method* m = find_method(v->t, "Error", "func() string");
        if (m == nullptr) {
            name = "String";
            m = find_method(v->t, "String", "func() string");
        }
        if (m == nullptr) {
            return false;
        }
        try {
            gp::string s = call_string(*v, m);
            pfmt_string(s.gp_view(), verb);
        } catch (const gp::panic_error&) {
            catch_panic(*v, verb, name);
        } catch (const std::exception&) {
            catch_panic(*v, verb, name);
        }
        return true;
    }
    default:
        return false;
    }
}

// is_reflect_value: the operand is a reflect.Value (stdlib/reflect/value.gp: the type of the value,
// the storage of the object and the flags, in that order).
bool is_reflect_value(const type_desc* t) noexcept {
    return t->gp_kind == kind::Struct && t->gp_pkg != nullptr &&
           std::strcmp(t->gp_pkg, "reflect") == 0 && std::strcmp(t->gp_name, "reflect.Value") == 0;
}

// print_reflect_value is printArg's `case reflect.Value`: the value that it holds prints, and its
// methods are used when it can be taken as an interface (flagRO unset).
void pp::print_reflect_value(const value& a, char32_t verb) {
    const type_desc* ft = full(a.t);
    const char* base = static_cast<const char*>(a.p);
    const std::uint64_t typ =
        *reinterpret_cast<const std::uint64_t*>(base + ft->gp_fields[0].gp_offset);
    const auto& ptr = *reinterpret_cast<const gp::ptr<void>*>(base + ft->gp_fields[1].gp_offset);
    const std::uint64_t flag =
        *reinterpret_cast<const std::uint64_t*>(base + ft->gp_fields[2].gp_offset);
    if (typ == 0) {
        print_value(value{}, verb, 0, access::open);
        return;
    }
    value inner = of_object(reinterpret_cast<const type_desc*>(static_cast<std::uintptr_t>(typ)),
                            ptr.gp_get());
    if (inner.own == nullptr) {
        inner.own = ptr.gp_shared();
    }
    constexpr std::uint64_t flag_embed_ro = 1 << 6;
    constexpr std::uint64_t flag_ro = (1 << 5) | flag_embed_ro;
    if ((flag & flag_ro) == 0) {
        value arg = inner;
        if (inner.t->gp_kind == kind::Interface) {
            arg = elem_of_iface(inner.p);
        }
        if (arg.t != nullptr && (arg.t->gp_nmethods != 0 || verb == 'w') &&
            handle_methods(arg, verb)) {
            return;
        }
        print_value(inner, verb, 0, access::open);
        return;
    }
    print_value(inner, verb, 0, (flag & flag_embed_ro) != 0 ? access::embed_ro : access::sticky_ro);
}

// is_plain_bytes: t is the unnamed type []byte, which Go's type switch prints as bytes.
[[gnu::noinline]] static bool is_plain_bytes(const type_desc* t) {
    return t->gp_kind == kind::Slice && !t->gp_named && full(t)->gp_elem == &td_uint8;
}

// print_arg is printArg.
void pp::print_arg(const value& a, char32_t verb) {
    cur = &a;
    if (a.t == nullptr) {
        switch (verb) {
        case 'T':
        case 'v':
            pad("<nil>");
            break;
        default:
            bad_verb(verb);
        }
        return;
    }
    // Special processing considerations. %T (the value's type) and %p (its address) are special;
    // we always do them first.
    switch (verb) {
    case 'T':
        fmt_s(type_string(a.t));
        return;
    case 'p':
        pfmt_pointer(a, 'p');
        return;
    default:
        break;
    }
    const type_desc* t = a.t;
    if (is_reflect_value(t)) {
        print_reflect_value(a, verb);
        return;
    }
    // Some types can be done without reflection: the predeclared ones, which have no methods.
    // If the type is not simple, it might have methods.
    // (Go's type switch has the unnamed []byte too: it does not call handleMethods, so %w of it is
    // a bad verb for each element.)
    if ((verb == 'w' || t->gp_nmethods != 0) && !(verb == 'w' && is_plain_bytes(t)) &&
        handle_methods(a, verb)) {
        return;
    }
    cur = &a;
    // An unnamed []byte is printed as bytes.
    if (t->gp_kind == kind::Slice && !t->gp_named && full(t)->gp_elem == &td_uint8) {
        switch (verb) {
        case 'v':
        case 'd':
        case 's':
        case 'x':
        case 'X':
        case 'q': {
            const type_ops* ops = t->gp_ops;
            const void* data = ops->gp_seq_data(a.p);
            auto n = static_cast<std::size_t>(ops->gp_seq_len(a.p));
            pfmt_bytes(n == 0 ? std::string_view()
                              : std::string_view(static_cast<const char*>(data), n),
                       ops->gp_nil(a.p), verb, "[]byte", a);
            return;
        }
        default:
            break;
        }
    }
    print_value(a, verb, 0, access::open);
}

void pp::print_address(uint64_t u, char32_t verb) {
    if (verb == 'v') {
        if (u == 0) {
            pad("<nil>");
        } else {
            fmt0x64(u, !sharp);
        }
        return;
    }
    pfmt_integer(u, false, verb);
}

// print_foreign prints a C/C++ value by its C++ type, as Go prints the matching Go type.
void pp::print_foreign(const value& v, char32_t verb) {
    const type_desc* t = v.t;
    const type_ops* ops = t->gp_ops;
    switch (ops != nullptr ? ops->gp_fclass : fclass::none) {
    case fclass::boolean:
        pfmt_bool(*static_cast<const bool*>(v.p), verb);
        return;
    case fclass::sint:
    case fclass::enum_s:
        pfmt_integer(read_sized(true, t->gp_size, v.p), true, verb);
        return;
    case fclass::uint_:
    case fclass::enum_u:
        pfmt_integer(read_sized(false, t->gp_size, v.p), false, verb);
        return;
    case fclass::f32:
        pfmt_float(*static_cast<const float*>(v.p), 32, verb);
        return;
    case fclass::f64:
        pfmt_float(*static_cast<const double*>(v.p), 64, verb);
        return;
    case fclass::raw_ptr:
    case fclass::cptr: {
        uint64_t u = ops->gp_fclass == fclass::cptr
                         ? addr_value(ops->gp_target(v.p))
                         : addr_value(*static_cast<const void* const*>(v.p));
        // The address, as Go prints a pointer that is no &{...}; the target is never read.
        if (verb == 'v' || verb == 'd') {
            print_address(u, verb);
            return;
        }
        erroring = true;
        buf += "%!";
        append_rune(buf, verb);
        buf += '(';
        buf += type_string(t);
        buf += '=';
        erroring = false;
        print_address(u, 'v');
        buf += ')';
        return;
    }
    case fclass::cxx_string:
        pfmt_string(*static_cast<const std::string*>(v.p), verb);
        return;
    case fclass::cxx_string_view:
        pfmt_string(*static_cast<const std::string_view*>(v.p), verb);
        return;
    case fclass::null_ptr:
        if (verb == 'v') {
            pad("<nil>");
        } else {
            buf += "%!";
            append_rune(buf, verb);
            buf += "(<nil>)";
        }
        return;
    default:
        break;
    }
    // Any other class: its text if there is one (like a Go value with a String method: the text
    // for %v %s %q, the error form with the text for the other verbs), else its address.
    if (ops != nullptr && ops->gp_text != nullptr) {
        void (*text)(std::string&, const void*) = __atomic_load_n(&ops->gp_text, __ATOMIC_ACQUIRE);
        std::string s;
        try {
            text(s, v.p);
        } catch (const std::exception&) {
            catch_panic(v, verb, "String");
            return;
        }
        pfmt_string(s, verb);
        return;
    }
    std::string s;
    s = "0x";
    char tmp[24];
    auto r = std::to_chars(tmp, tmp + sizeof tmp, addr_value(v.p), 16);
    s.append(tmp, r.ptr);
    pad(s);
}

void pp::print_map(const value& v, const type_desc* ft, char32_t verb, int depth, access ro) {
    const type_ops* ops = v.t->gp_ops;
    if (sharp_v) {
        buf += type_string(v.t);
        if (ops->gp_nil(v.p)) {
            buf += "(nil)";
            return;
        }
        buf += '{';
    } else {
        buf += "map[";
    }
    std::shared_ptr<void> snap = ops->gp_map_snap != nullptr ? ops->gp_map_snap(v.p) : nullptr;
    int64_t n = snap != nullptr ? ops->gp_snap_len(snap.get()) : 0;
    // The entries, sorted by key, as fmtsort.Sort does.
    std::vector<value> keys(static_cast<std::size_t>(n)), vals(static_cast<std::size_t>(n));
    for (int64_t i = 0; i < n; i++) {
        const void *k, *e;
        ops->gp_snap_at(snap.get(), i, &k, &e);
        keys[static_cast<std::size_t>(i)] = of_object(ft->gp_key, k);
        vals[static_cast<std::size_t>(i)] = of_object(ft->gp_elem, e);
    }
    std::vector<std::size_t> order(keys.size());
    for (std::size_t i = 0; i < order.size(); i++) {
        order[i] = i;
    }
    stable_sort(order, [&](std::size_t a, std::size_t b) { return compare(keys[a], keys[b]) < 0; });
    for (std::size_t i = 0; i < order.size(); i++) {
        if (i > 0) {
            if (sharp_v) {
                buf += ", ";
            } else {
                buf += ' ';
            }
        }
        print_value(keys[order[i]], verb, depth + 1, below(ro));
        buf += ':';
        print_value(vals[order[i]], verb, depth + 1, below(ro));
    }
    if (sharp_v) {
        buf += '}';
    } else {
        buf += ']';
    }
}

void pp::print_struct(const value& v, const type_desc* ft, char32_t verb, int depth, access ro) {
    const type_desc* t = v.t;
    if (t->gp_ops != nullptr && t->gp_ops->gp_word != nullptr) {
        // A weak pointer is Go's struct { _ [0]*T; u unsafe.Pointer }: {[] 0xc000012345},
        // {[] <nil>} for the zero value, {_:[] u:<nil>} for %+v and a verb that applies to u as
        // to a pointer that is no &{...} (an unexported field). gocpp prints the address the
        // weak pointer was made from, Go the address of its own handle: both are addresses that
        // differ from run to run.
        if (sharp_v) {
            buf += type_string(t);
        }
        buf += '{';
        if (plus_v || sharp_v) {
            buf += "_:";
        }
        if (sharp_v) {
            buf += "[0]*";
            buf += t->gp_elem != nullptr ? t->gp_elem->gp_name : "?";
            buf += "{}, u:";
        } else {
            buf += "[] ";
            if (plus_v) {
                buf += "u:";
            }
        }
        uint64_t u = t->gp_ops->gp_word(v.p);
        switch (verb) {
        case 'v':
            if (sharp_v) {
                buf += "(unsafe.Pointer)(";
                if (u == 0) {
                    buf += "nil";
                } else {
                    fmt0x64(u, true);
                }
                buf += ')';
            } else {
                print_address(u, 'v');
            }
            break;
        case 'd':
            fmt_integer(u, 10, false, verb, ldigits);
            break;
        default:
            erroring = true;
            buf += "%!";
            append_rune(buf, verb);
            buf += "(unsafe.Pointer=";
            erroring = false;
            print_address(u, 'v');
            buf += ')';
        }
        buf += '}';
        return;
    }
    if (sharp_v) {
        buf += type_string(t);
    }
    buf += '{';
    for (std::size_t i = 0; i < ft->gp_nfields; i++) {
        const field_desc& f = ft->gp_fields[i];
        if (i > 0) {
            if (sharp_v) {
                buf += ", ";
            } else {
                buf += ' ';
            }
        }
        if (plus_v || sharp_v) {
            if (f.gp_name[0] != '\0') {
                buf += f.gp_name;
                buf += ':';
            }
        }
        access acc = field_access(ro, f.gp_pkg == nullptr, f.gp_embedded);
        value fv = field_value(v.p, f);
        if (f.gp_type->gp_kind == kind::Interface && fv.t != f.gp_type) {
            acc = below(acc); // the value inside the interface
        }
        print_value(fv, verb, depth + 1, acc);
    }
    buf += '}';
}

void pp::print_seq(const value& v, const type_desc* ft, char32_t verb, int depth, access ro) {
    const type_desc* t = v.t;
    const type_ops* ops = t->gp_ops;
    const type_desc* et = ft->gp_elem;
    bool is_slice = t->gp_kind == kind::Slice;
    switch (verb) {
    case 's':
    case 'q':
    case 'x':
    case 'X':
        // Handle byte and uint8 slices and arrays special for the above verbs.
        if (et->gp_kind == kind::Uint8) {
            const void* data = ops->gp_seq_data(v.p);
            auto n = static_cast<std::size_t>(ops->gp_seq_len(v.p));
            pfmt_bytes(n == 0 ? std::string_view()
                              : std::string_view(static_cast<const char*>(data), n),
                       false, verb, type_string(t), v);
            return;
        }
        break;
    default:
        break;
    }
    // While an element prints, the array that holds it stays alive (a method that changes the
    // slice can't free it).
    std::shared_ptr<void> hold;
    if (is_slice && may_run_code(et) && ops->gp_hold != nullptr) {
        hold = ops->gp_hold(v.p);
    }
    if (sharp_v) {
        buf += type_string(t);
        if (is_slice && ops->gp_nil(v.p)) {
            buf += "(nil)";
            return;
        }
        buf += '{';
    } else {
        buf += '[';
    }
    // Like reflect's Len and Index, the slice is read again for each element.
    for (int64_t i = 0; i < ops->gp_seq_len(v.p); i++) {
        if (i > 0) {
            if (sharp_v) {
                buf += ", ";
            } else {
                buf += ' ';
            }
        }
        const char* data = static_cast<const char*>(ops->gp_seq_data(v.p));
        print_value(of_object(et, data + static_cast<std::size_t>(i) * et->gp_size), verb,
                    depth + 1, below(ro));
    }
    buf += sharp_v ? '}' : ']';
}

// print_value is printValue.
void pp::print_value(const value& v, char32_t verb, int depth, access ro) {
    const type_desc* t = v.t;
    // Handle values with special methods if not already handled by printArg (depth == 0).
    if (depth > 0 && t != nullptr && ro == access::open &&
        (t->gp_nmethods != 0 || t->gp_kind == kind::Interface || verb == 'w')) {
        if (handle_methods(v, verb)) {
            return;
        }
    }
    cur = &v;
    if (t == nullptr) {
        // reflect.Invalid
        if (depth == 0) {
            buf += "<invalid reflect.Value>";
        } else if (verb == 'v') {
            buf += "<nil>";
        } else {
            bad_verb(verb);
        }
        return;
    }
    switch (t->gp_kind) {
    case kind::Bool:
        pfmt_bool(*static_cast<const bool*>(v.p), verb);
        break;
    case kind::Int:
    case kind::Int8:
    case kind::Int16:
    case kind::Int32:
    case kind::Int64:
        pfmt_integer(read_int(t->gp_kind, v.p), true, verb);
        break;
    case kind::Uint:
    case kind::Uint8:
    case kind::Uint16:
    case kind::Uint32:
    case kind::Uint64:
    case kind::Uintptr:
        pfmt_integer(read_uint(t->gp_kind, v.p), false, verb);
        break;
    case kind::Float32:
        pfmt_float(*static_cast<const float*>(v.p), 32, verb);
        break;
    case kind::Float64:
        pfmt_float(*static_cast<const double*>(v.p), 64, verb);
        break;
    case kind::Complex64: {
        auto& c = *static_cast<const gp::complex64*>(v.p);
        pfmt_complex(c.gp_re, c.gp_im, 64, verb);
        break;
    }
    case kind::Complex128: {
        auto& c = *static_cast<const gp::complex128*>(v.p);
        pfmt_complex(c.gp_re, c.gp_im, 128, verb);
        break;
    }
    case kind::String:
        pfmt_string(static_cast<const gp::string*>(v.p)->gp_view(), verb);
        break;
    case kind::Map:
        print_map(v, full(t), verb, depth, ro);
        break;
    case kind::Struct:
        print_struct(v, full(t), verb, depth, ro);
        break;
    case kind::Interface: {
        value e = elem_of_iface(v.p);
        if (e.t == nullptr) {
            if (sharp_v) {
                buf += type_string(t);
                buf += "(nil)";
            } else {
                buf += "<nil>";
            }
        } else {
            print_value(e, verb, depth + 1, below(ro));
        }
        break;
    }
    case kind::Array:
    case kind::Slice:
        print_seq(v, full(t), verb, depth, ro);
        break;
    case kind::Pointer: {
        // pointer to array or slice or struct? ok at top level but not embedded (avoid loops)
        if (depth == 0 && v.p != nullptr) {
            const type_desc* et = full(t)->gp_elem;
            if (et != nullptr) {
                switch (et->gp_kind) {
                case kind::Array:
                case kind::Slice:
                case kind::Struct:
                case kind::Map: {
                    buf += '&';
                    value pe;
                    pe.t = et;
                    pe.p = v.p;
                    pe.own = v.own;
                    print_value(pe, verb, depth + 1, ro);
                    return;
                }
                default:
                    break;
                }
            }
        }
        pfmt_pointer(v, verb);
        break;
    }
    case kind::Chan:
    case kind::Func:
    case kind::UnsafePointer:
        pfmt_pointer(v, verb);
        break;
    case kind::Foreign:
        print_foreign(v, verb);
        break;
    default:
        // unknownType
        buf += '?';
        buf += type_string(t);
        buf += '?';
    }
}

std::tuple<std::size_t, std::size_t, bool>
pp::arg_number(std::size_t arg_num, std::string_view format, std::size_t i, std::size_t num_args) {
    if (format.size() <= i || format[i] != '[') {
        return {arg_num, i, false};
    }
    reordered = true;
    auto [index, wid_, ok] = parse_arg_number(format.substr(i));
    if (ok && 0 <= index && static_cast<std::size_t>(index) < num_args) {
        return {static_cast<std::size_t>(index), i + wid_, true};
    }
    good_arg_num = false;
    return {arg_num, i + wid_, ok};
}

void pp::print_live(std::size_t i, char32_t verb) {
    print_arg(from_iface(lv.data[i]), verb);
}

std::tuple<int, bool, std::size_t> pp::int_live(std::size_t arg_num) {
    value t = from_iface(lv.data[arg_num]);
    auto [num, ok, next] = int_from_arg(&t, 1, 0);
    (void)next;
    return {num, ok, arg_num + 1};
}

void pp::extra_live(std::size_t k) {
    value ak = from_iface(lv.data[k]);
    if (ak.t == nullptr) {
        buf += "<nil>";
    } else {
        buf += type_string(ak.t);
        buf += '=';
        print_arg(ak, 'v');
    }
}

void pp::do_print_live(std::size_t n) {
    bool prev_string = false;
    for (std::size_t i = 0; i < n; i++) {
        value ai = from_iface(lv.data[i]);
        bool is_string = is_string_kind(ai);
        if (i > 0 && !is_string && !prev_string) {
            buf += ' ';
        }
        print_arg(ai, 'v');
        prev_string = is_string;
    }
}

void pp::extra_args(const value* a, std::size_t n, std::size_t arg_num) {
    clearflags();
    buf += "%!(EXTRA ";
    for (std::size_t k = arg_num; k < n; k++) {
        if (k > arg_num) {
            buf += ", ";
        }
        if (lv.data != nullptr) [[unlikely]] {
            extra_live(k);
        } else if (a[k].t == nullptr) {
            buf += "<nil>";
        } else {
            buf += type_string(a[k].t);
            buf += '=';
            print_arg(a[k], 'v');
        }
    }
    buf += ')';
}

void pp::do_printf(std::string_view format, const value* a, std::size_t n) {
    std::size_t end = format.size();
    std::size_t arg_num = 0;  // we process one argument per non-trivial format
    bool after_index = false; // the previous item in format was an index like [3]
    reordered = false;
    for (std::size_t i = 0; i < end;) {
        good_arg_num = true;
        std::size_t lasti = i;
        while (i < end && format[i] != '%') {
            i++;
        }
        if (i > lasti) {
            buf += format.substr(lasti, i - lasti);
        }
        if (i >= end) {
            break; // done processing format string
        }
        // Process one verb
        i++;
        // Do we have flags?
        clearflags();
        bool simple_done = false;
        for (; i < end; i++) {
            char c = format[i];
            if (c == '#') {
                sharp = true;
            } else if (c == '0') {
                zero = true;
            } else if (c == '+') {
                plus = true;
            } else if (c == '-') {
                minus = true;
            } else if (c == ' ') {
                space = true;
            } else {
                // Fast path for common case of ascii lower case simple verbs without precision
                // or width or argument indices.
                if ('a' <= c && c <= 'z' && arg_num < n) {
                    if (c == 'w') {
                        wrapped_errs.push_back(static_cast<int>(arg_num));
                    }
                    if (c == 'w' || c == 'v') {
                        // Go syntax
                        sharp_v = sharp;
                        sharp = false;
                        // Struct-field syntax
                        plus_v = plus;
                        plus = false;
                    }
                    print_nth(a, arg_num, static_cast<char32_t>(c));
                    arg_num++;
                    i++;
                    simple_done = true;
                }
                // Format is more complex than simple flags and a verb or is malformed.
                break;
            }
        }
        if (simple_done) {
            continue;
        }
        // Do we have an explicit argument index?
        std::tie(arg_num, i, after_index) = arg_number(arg_num, format, i, n);
        // Do we have width?
        if (i < end && format[i] == '*') {
            i++;
            std::tie(wid, wid_present, arg_num) = int_arg(a, n, arg_num);
            if (!wid_present) {
                buf += "%!(BADWIDTH)";
            }
            // We have a negative width, so take its value and ensure that the minus flag is set
            if (wid < 0) {
                wid = -wid;
                minus = true;
                zero = false; // Do not pad with zeros to the right.
            }
            after_index = false;
        } else {
            std::tie(wid, wid_present, i) = parsenum(format, i, end);
            if (after_index && wid_present) { // "%[3]2d"
                good_arg_num = false;
            }
        }
        // Do we have precision?
        if (i + 1 < end && format[i] == '.') {
            i++;
            if (after_index) { // "%[3].2d"
                good_arg_num = false;
            }
            std::tie(arg_num, i, after_index) = arg_number(arg_num, format, i, n);
            if (i < end && format[i] == '*') {
                i++;
                std::tie(prec, prec_present, arg_num) = int_arg(a, n, arg_num);
                // Negative precision arguments don't make sense
                if (prec < 0) {
                    prec = 0;
                    prec_present = false;
                }
                if (!prec_present) {
                    buf += "%!(BADPREC)";
                }
                after_index = false;
            } else {
                std::tie(prec, prec_present, i) = parsenum(format, i, end);
                if (!prec_present) {
                    prec = 0;
                    prec_present = true;
                }
            }
        }
        if (!after_index) {
            std::tie(arg_num, i, after_index) = arg_number(arg_num, format, i, n);
        }
        if (i >= end) {
            buf += "%!(NOVERB)";
            break;
        }
        auto [verb, size] = decode(format.substr(i));
        i += size;
        if (verb == '%') { // Percent does not absorb operands and ignores f.wid and f.prec.
            buf += '%';
        } else if (!good_arg_num) {
            bad_arg(verb, "(BADINDEX)");
        } else if (arg_num >= n) { // No argument left over to print for the current verb.
            bad_arg(verb, "(MISSING)");
        } else {
            if (verb == 'w') {
                wrapped_errs.push_back(static_cast<int>(arg_num));
            }
            if (verb == 'w' || verb == 'v') {
                // Go syntax
                sharp_v = sharp;
                sharp = false;
                // Struct-field syntax
                plus_v = plus;
                plus = false;
            }
            print_nth(a, arg_num, verb);
            arg_num++;
        }
    }
    // Check for extra arguments unless the call accessed the arguments out of order, in which
    // case it's too expensive to detect if they've all been used and arguably OK if they're not.
    if (!reordered && arg_num < n) [[unlikely]] {
        extra_args(a, n, arg_num);
    }
}

void pp::do_print(const value* a, std::size_t n) {
    if (lv.data != nullptr) [[unlikely]] {
        do_print_live(n);
        return;
    }
    bool prev_string = false;
    for (std::size_t i = 0; i < n; i++) {
        bool is_string = is_string_kind(a[i]);
        // Add a space between two non-string arguments.
        if (i > 0 && !is_string && !prev_string) {
            buf += ' ';
        }
        print_arg(a[i], 'v');
        prev_string = is_string;
    }
}

void pp::do_println(const value* a, std::size_t n) {
    if (lv.data != nullptr) [[unlikely]] {
        do_println_live(n);
        return;
    }
    for (std::size_t i = 0; i < n; i++) {
        if (i > 0) {
            buf += ' ';
        }
        print_arg(a[i], 'v');
    }
    buf += '\n';
}

void pp::do_println_live(std::size_t n) {
    for (std::size_t i = 0; i < n; i++) {
        if (i > 0) {
            buf += ' ';
        }
        print_arg(from_iface(lv.data[i]), 'v');
    }
    buf += '\n';
}

// --- the errors of Errorf ---

// *errors.errorString, *fmt.wrapError and *fmt.wrapErrors. (errors.New in package errors makes
// the first; the runtime has its own so that fmt doesn't need the package.)
struct error_string_t {
    gp::string gp_s;
};
struct wrap_error_t {
    gp::string gp_msg;
    gp::error gp_err;
};
struct wrap_errors_t {
    gp::string gp_msg;
    gp::slice<gp::error> gp_errs;
};

gp::string thunk_es_error(const iface& self) {
    return unbox<gp::ptr<error_string_t>>(self)->gp_s;
}
gp::string thunk_we_error(const iface& self) {
    return unbox<gp::ptr<wrap_error_t>>(self)->gp_msg;
}
gp::error thunk_we_unwrap(const iface& self) {
    return unbox<gp::ptr<wrap_error_t>>(self)->gp_err;
}
gp::string thunk_wes_error(const iface& self) {
    return unbox<gp::ptr<wrap_errors_t>>(self)->gp_msg;
}
gp::slice<gp::error> thunk_wes_unwrap(const iface& self) {
    return unbox<gp::ptr<wrap_errors_t>>(self)->gp_errs;
}

// The descriptors of the three types have their fields (unexported, of their packages), so that %#v
// prints &errors.errorString{s:"x"} as Go does; the complete copies come from gp_full.
const type_desc* full_error_string();
const type_desc* full_wrap_error();
const type_desc* full_wrap_errors();

const type_desc& td_error_string() {
    static const method pmethods[] = {
        method_of<gp::string()>("Error", nullptr, "func() string", &thunk_es_error)};
    static const type_desc td = named<error_string_t>(
        "errors.errorString", "errors", shape_lazy(shape_struct(nullptr, 0), &full_error_string),
        {}, methods_of(pmethods));
    return td;
}
const type_desc* full_error_string() {
    static const field_desc f[] = {
        field_desc("s", "errors", &td_string, nullptr, false, offset_of(&error_string_t::gp_s))};
    static const type_desc d = completed(td_error_string(), shape_struct(f, 1));
    return &d;
}
const type_desc* td_error_string_ptr() {
    static const type_desc* const p = td_ptr<error_string_t>(&td_error_string());
    return p;
}

const type_desc& td_wrap_error() {
    static const method pmethods[] = {
        method_of<gp::string()>("Error", nullptr, "func() string", &thunk_we_error),
        method_of<gp::error()>("Unwrap", nullptr, "func() error", &thunk_we_unwrap),
    };
    static const type_desc td = named<wrap_error_t>(
        "fmt.wrapError", "fmt", shape_lazy(shape_struct(nullptr, 0), &full_wrap_error), {},
        methods_of(pmethods));
    return td;
}
const type_desc* full_wrap_error() {
    static const field_desc f[] = {
        field_desc("msg", "fmt", &td_string, nullptr, false, offset_of(&wrap_error_t::gp_msg)),
        field_desc("err", "fmt", &td_error, nullptr, false, offset_of(&wrap_error_t::gp_err)),
    };
    static const type_desc d = completed(td_wrap_error(), shape_struct(f, 2));
    return &d;
}
const type_desc* td_wrap_error_ptr() {
    static const type_desc* const p = td_ptr<wrap_error_t>(&td_wrap_error());
    return p;
}

const type_desc& td_wrap_errors() {
    static const method pmethods[] = {
        method_of<gp::string()>("Error", nullptr, "func() string", &thunk_wes_error),
        method_of<gp::slice<gp::error>()>("Unwrap", nullptr, "func() []error", &thunk_wes_unwrap),
    };
    static const type_desc td = named<wrap_errors_t>(
        "fmt.wrapErrors", "fmt", shape_lazy(shape_struct(nullptr, 0), &full_wrap_errors), {},
        methods_of(pmethods));
    return td;
}
const type_desc* full_wrap_errors() {
    static const field_desc f[] = {
        field_desc("msg", "fmt", &td_string, nullptr, false, offset_of(&wrap_errors_t::gp_msg)),
        field_desc("errs", "fmt", td_slice<gp::error>(&td_error), nullptr, false,
                   offset_of(&wrap_errors_t::gp_errs)),
    };
    static const type_desc d = completed(td_wrap_errors(), shape_struct(f, 2));
    return &d;
}
const type_desc* td_wrap_errors_ptr() {
    static const type_desc* const p = td_ptr<wrap_errors_t>(&td_wrap_errors());
    return p;
}

// as_error is the operand as an error (Go's a[i].(error)), or the nil error. An operand that is
// not in an interface or a pointer can't be shared: it gives the nil error (the emitter boxes an
// operand of a concrete error type in an interface).
gp::error as_error(const value& v) {
    if (v.t == nullptr || find_method(v.t, "Error", "func() string") == nullptr) {
        return gp::error();
    }
    bool is_ptr = v.t->gp_kind == kind::Pointer;
    if (v.own == nullptr && !is_ptr) {
        return gp::error();
    }
    const itab* tab = getitab(&td_error, v.t);
    if (tab == nullptr) {
        return gp::error();
    }
    return gp::error(v.own, v.t, tab);
}

} // namespace

void sprint(std::string& out, const arg* args, std::size_t n) {
    pp p(out);
    p.do_print(args, n);
}

void sprint_live(std::string& out, const arg* args, std::size_t n, live_span lv) {
    pp p(out);
    p.lv = lv;
    p.do_print(args, n);
}

void sprintln(std::string& out, const arg* args, std::size_t n) {
    pp p(out);
    p.do_println(args, n);
}

void sprintln_live(std::string& out, const arg* args, std::size_t n, live_span lv) {
    pp p(out);
    p.lv = lv;
    p.do_println(args, n);
}

void sprintf(std::string& out, std::string_view format, const arg* args, std::size_t n) {
    pp p(out);
    p.do_printf(format, args, n);
}

void sprintf_live(std::string& out, std::string_view format, const arg* args, std::size_t n,
                  live_span lv) {
    pp p(out);
    p.lv = lv;
    p.do_printf(format, args, n);
}

namespace {

// operand_of is the operand arg_num of an Errorf call, as it is now when the call has a slice a....
[[gnu::noinline]] value operand_live(const pp& p, std::size_t arg_num) {
    return from_iface(p.lv.data[arg_num]);
}
inline value operand_of(const pp& p, const arg* a, std::size_t arg_num) {
    return p.lv.data != nullptr ? operand_live(p, arg_num) : a[arg_num];
}

gp::error errorf_impl(std::string_view format, const arg* a, std::size_t n, live_span lv) {
    std::string s;
    pp p(s);
    p.lv = lv;
    p.wrap_errs = true;
    p.do_printf(format, a, n);
    gp::error err;
    switch (p.wrapped_errs.size()) {
    case 0:
        err = to_iface<gp::error>(gp::new_<error_string_t>(error_string_t{gp::string::gp_copy(s)}),
                                  td_error_string_ptr());
        break;
    case 1: {
        wrap_error_t w;
        w.gp_msg = gp::string::gp_copy(s);
        w.gp_err = as_error(operand_of(p, a, static_cast<std::size_t>(p.wrapped_errs[0])));
        err = to_iface<gp::error>(gp::new_<wrap_error_t>(std::move(w)), td_wrap_error_ptr());
        break;
    }
    default: {
        if (p.reordered) {
            std::sort(p.wrapped_errs.begin(), p.wrapped_errs.end());
        }
        gp::slice<gp::error> errs;
        for (std::size_t i = 0; i < p.wrapped_errs.size(); i++) {
            int arg_num = p.wrapped_errs[i];
            if (i > 0 && p.wrapped_errs[i - 1] == arg_num) {
                continue;
            }
            gp::error e = as_error(operand_of(p, a, static_cast<std::size_t>(arg_num)));
            if (e != nullptr) {
                errs = gp::append(std::move(errs), std::move(e));
            }
        }
        wrap_errors_t w;
        w.gp_msg = gp::string::gp_copy(s);
        w.gp_errs = std::move(errs);
        err = to_iface<gp::error>(gp::new_<wrap_errors_t>(std::move(w)), td_wrap_errors_ptr());
    }
    }
    return err;
}

} // namespace

gp::error errorf(std::string_view format, const arg* a, std::size_t n) {
    return errorf_impl(format, a, n, live_span{});
}

gp::error errorf_live(std::string_view format, const arg* a, std::size_t n, live_span lv) {
    return errorf_impl(format, a, n, lv);
}

// state_call calls the method in the slot of the itab of the fmt.State s (not icall, whose nil
// check names gp::panic_nil, which a -nochecks runtime object must not refer to).
template <typename R, typename... A>
R state_call(const State& s, std::size_t slot, A... a) {
    const itab* t = s.gp_tab();
    if (t == nullptr) {
        throw gp::panic_error(gp::string::gp_copy(
            "runtime error: invalid memory address or nil pointer dereference"));
    }
    return reinterpret_cast<thunk_t<R(A...)>*>(t->gp_fun[slot])(s, a...);
}

std::tuple<int64_t, gp::error> write_failed(const char* what, const std::string& s) {
    (void)s;
    return {0, to_iface<gp::error>(gp::new_<error_string_t>(error_string_t{gp::string::gp_copy(
                                       std::string("write /dev/stdout: ") + what)}),
                                   td_error_string_ptr())};
}

std::tuple<int64_t, gp::error> write_state(const State& f, std::string_view s) {
    return state_call<std::tuple<int64_t, gp::error>>(f, 3, gp::to_bytes(gp::string::gp_copy(s)));
}

} // namespace gp::fmt::detail

namespace gp::fmt {

gp::string FormatString(State state, int32_t verb) {
    std::string b = "%";
    for (char c : std::string_view(" +-#0")) {
        if (detail::state_call<bool>(state, 0, static_cast<int64_t>(c))) {
            b += c;
        }
    }
    if (auto [w, ok] = detail::state_call<std::tuple<int64_t, bool>>(state, 2); ok) {
        b += std::to_string(w);
    }
    if (auto [p, ok] = detail::state_call<std::tuple<int64_t, bool>>(state, 1); ok) {
        b += '.';
        b += std::to_string(p);
    }
    detail::append_rune(b, static_cast<char32_t>(verb));
    return gp::string::gp_copy(b);
}

} // namespace gp::fmt
