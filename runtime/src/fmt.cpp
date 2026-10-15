// The parts of gp::fmtshim (gocpp/fmtshim.hpp) that are not templates: Go's fmt formats for the
// basic kinds, the parsing of a Printf format, and strconv's float formatting and quoting. Each
// function mirrors the Go 1.25 function of the same name in fmt/format.go, fmt/print.go or
// strconv. Standard C++ only: the OS is reached through gocpp/os.hpp.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <algorithm>
#include <charconv>
#include <cmath>
#include <cstdint>
#include <string>
#include <string_view>
#include <system_error>
#include <tuple>

namespace gp::fmtshim::detail {

namespace {

constexpr char32_t rune_error = 0xFFFD;
constexpr char32_t max_rune = 0x10FFFF;
constexpr const char* ldigits = "0123456789abcdefx";

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

// The decimal digits of a float, as strconv's decimalSlice: d[0:nd] with the decimal point
// after dp digits (0.d * 10^dp). Zero has no digits and dp 0.
struct decimal {
    std::string d;
    int dp = 0;
};

// digits_of: the digits of |v|, without trailing zeros: for n < 0 the shortest digits that read
// back as v (a float32 when bits is 32), else v rounded to n significant digits.
decimal digits_of(double v, int bits, int n) {
    // d.ddd...e-ddd: the digits, the point, the exponent.
    std::string buf(static_cast<std::size_t>(std::max(n, 0)) + 32, '\0');
    char* b = buf.data();
    char* e = b + buf.size();
    std::to_chars_result r{};
    if (n < 0) {
        r = bits == 32 ? std::to_chars(b, e, std::fabs(static_cast<float>(v)),
                                       std::chars_format::scientific)
                       : std::to_chars(b, e, std::fabs(v), std::chars_format::scientific);
    } else {
        r = std::to_chars(b, e, std::fabs(v), std::chars_format::scientific, n - 1);
    }
    std::string_view s(b, static_cast<std::size_t>(r.ptr - b));
    std::size_t epos = s.find('e');
    decimal dec;
    for (char c : s.substr(0, epos)) {
        if (c != '.') {
            dec.d += c;
        }
    }
    std::string_view es = s.substr(epos + 1);
    bool eneg = es.front() == '-';
    es.remove_prefix(1); // the sign
    int exp = 0;
    std::from_chars(es.data(), es.data() + es.size(), exp);
    if (eneg) {
        exp = -exp;
    }
    while (!dec.d.empty() && dec.d.back() == '0') {
        dec.d.pop_back();
    }
    dec.dp = dec.d.empty() ? 0 : exp + 1;
    return dec;
}

// fmt_e is strconv's fmtE: [-]d.ddddde±dd.
void fmt_e(std::string& out, bool neg, const decimal& d, int prec, char fmt) {
    int nd = static_cast<int>(d.d.size());
    if (neg) {
        out += '-';
    }
    out += nd == 0 ? '0' : d.d[0];
    if (prec > 0) {
        out += '.';
        int i = 1;
        int m = std::min(nd, prec + 1);
        if (i < m) {
            out.append(d.d, static_cast<std::size_t>(i), static_cast<std::size_t>(m - i));
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
void fmt_f(std::string& out, bool neg, const decimal& d, int prec) {
    int nd = static_cast<int>(d.d.size());
    if (neg) {
        out += '-';
    }
    if (d.dp > 0) {
        int m = std::min(nd, d.dp);
        out.append(d.d, 0, static_cast<std::size_t>(m));
        for (; m < d.dp; m++) {
            out += '0';
        }
    } else {
        out += '0';
    }
    if (prec > 0) {
        out += '.';
        for (int i = 1; i <= prec; i++) {
            int j = d.dp + i - 1;
            out += 0 <= j && j < nd ? d.d[static_cast<std::size_t>(j)] : '0';
        }
    }
}

// --- fmt: format.go ---

// write_padding is fmt.writePadding: n bytes of padding, zeros only on the left.
void write_padding(printer& p, int64_t n) {
    if (n <= 0) {
        return;
    }
    p.buf.append(static_cast<std::size_t>(n), p.zero && !p.minus ? '0' : ' ');
}

// truncate is fmt.truncateString: s cut to the precision, in runes.
std::string_view truncate(const printer& p, std::string_view s) noexcept {
    if (p.prec_present) {
        int64_t n = p.prec;
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

void fmt_boolean(printer& p, bool v) {
    pad(p, v ? "true" : "false");
}

// fmt_integer is fmt.fmtInteger, for bases 10 and 16.
void fmt_integer(printer& p, uint64_t u, int base, bool is_signed, const char* digits) {
    bool negative = is_signed && static_cast<int64_t>(u) < 0;
    if (negative) {
        u = -u;
    }
    char small[68];
    std::string big;
    char* buf = small;
    std::size_t len = sizeof small;
    if (p.wid_present || p.prec_present) {
        // Room for a sign and "0x".
        auto width = 3 + static_cast<std::size_t>(p.wid) + static_cast<std::size_t>(p.prec);
        if (width > len) {
            big.resize(width);
            buf = big.data();
            len = width;
        }
    }
    // Two ways to ask for extra leading zero digits: %.3d or %03d. If both are given, the
    // zero flag is ignored and the padding is spaces.
    int64_t prec = 0;
    if (p.prec_present) {
        prec = p.prec;
        // Precision 0 and value 0 means "print nothing" but padding.
        if (prec == 0 && u == 0) {
            bool zero = p.zero;
            p.zero = false;
            write_padding(p, p.wid);
            p.zero = zero;
            return;
        }
    } else if (p.zero && !p.minus && p.wid_present) {
        prec = p.wid;
        if (negative || p.plus || p.space) {
            prec--; // room for the sign
        }
    }
    std::size_t i = len;
    if (base == 10) {
        while (u >= 10) {
            uint64_t next = u / 10;
            buf[--i] = static_cast<char>('0' + u - next * 10);
            u = next;
        }
    } else {
        while (u >= 16) {
            buf[--i] = digits[u & 0xF];
            u >>= 4;
        }
    }
    buf[--i] = digits[u];
    while (i > 0 && prec > static_cast<int64_t>(len - i)) {
        buf[--i] = '0';
    }
    if (p.sharp && base == 16) {
        buf[--i] = digits[16];
        buf[--i] = '0';
    }
    if (negative) {
        buf[--i] = '-';
    } else if (p.plus) {
        buf[--i] = '+';
    } else if (p.space) {
        buf[--i] = ' ';
    }
    // The zero padding on the left is done (as a precision), or the zero flag is ignored
    // because of an explicit precision.
    bool zero = p.zero;
    p.zero = false;
    pad(p, std::string_view(buf + i, len - i));
    p.zero = zero;
}

// fmt_float is fmt.fmtFloat (without the # flag, Phase 09).
void fmt_float(printer& p, double v, int size, char verb, int prec) {
    if (p.prec_present) {
        prec = p.prec;
    }
    // The number, with room for a leading + sign.
    std::string num(1, '+');
    append_float(num, v, verb, prec, size);
    if (num[1] == '-' || num[1] == '+') {
        num.erase(0, 1);
    }
    // The space flag means a leading space instead of a "+" sign, unless plus asks for it.
    if (p.space && num[0] == '+' && !p.plus) {
        num[0] = ' ';
    }
    // Infinities and NaN don't look like a number, so they aren't padded with zeros.
    if (num[1] == 'I' || num[1] == 'N') {
        bool zero = p.zero;
        p.zero = false;
        // No sign before NaN unless asked for.
        if (num[1] == 'N' && !p.space && !p.plus) {
            num.erase(0, 1);
        }
        pad(p, num);
        p.zero = zero;
        return;
    }
    // A sign if asked for, or if the number is negative.
    if (p.plus || num[0] != '+') {
        // With zero padding, the sign comes before the zeros.
        if (p.zero && !p.minus && p.wid_present && p.wid > static_cast<int64_t>(num.size())) {
            p.buf += num[0];
            write_padding(p, p.wid - static_cast<int64_t>(num.size()));
            p.buf.append(num, 1);
            return;
        }
        pad(p, num);
        return;
    }
    // No sign to show and the number is positive.
    pad(p, std::string_view(num).substr(1));
}

// fmt_q is fmt.fmtQ.
void fmt_q(printer& p, std::string_view s) {
    s = truncate(p, s);
    if (p.sharp && can_backquote(s)) {
        std::string q;
        q += '`';
        q += s;
        q += '`';
        pad(p, q);
        return;
    }
    std::string q;
    append_quoted(q, s, p.plus);
    pad(p, q);
}

// fmt_qc is fmt.fmtQc: an integer as a quoted rune.
void fmt_qc(printer& p, uint64_t c) {
    auto r = static_cast<char32_t>(c);
    if (c > max_rune) {
        r = rune_error;
    }
    std::string q;
    append_quoted_rune(q, r, p.plus);
    pad(p, q);
}

// --- fmt: Printf's format (print.go) ---

constexpr bool too_large(int64_t x) noexcept {
    return x > 1000000 || x < -1000000;
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
std::tuple<int, bool, std::size_t> int_from_arg(const arg* a, std::size_t na, std::size_t n) {
    if (n >= na) {
        return {0, false, n};
    }
    int64_t num = 0;
    bool is_int = false;
    if (a[n].int_kind == 1) {
        num = static_cast<int64_t>(a[n].int_val);
        is_int = true;
    } else if (a[n].int_kind == 2 && a[n].int_val <= static_cast<uint64_t>(INT64_MAX)) {
        num = static_cast<int64_t>(a[n].int_val);
        is_int = true;
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

struct format_state {
    bool reordered = false;
    bool good_arg_num = true;
};

// arg_number: the next argument, from arg_num or from an index [n] at format[i:].
std::tuple<std::size_t, std::size_t, bool> arg_number(format_state& st, std::size_t arg_num,
                                                      std::string_view format, std::size_t i,
                                                      std::size_t num_args) {
    if (format.size() <= i || format[i] != '[') {
        return {arg_num, i, false};
    }
    st.reordered = true;
    auto [index, wid, ok] = parse_arg_number(format.substr(i));
    if (ok && 0 <= index && static_cast<std::size_t>(index) < num_args) {
        return {static_cast<std::size_t>(index), i + wid, true};
    }
    st.good_arg_num = false;
    return {arg_num, i + wid, ok};
}

void bad_arg(printer& p, char32_t verb, const char* what) {
    p.buf += "%!";
    append_rune(p.buf, verb);
    p.buf += what;
}

} // namespace

void write_stdout(std::string_view s) {
    // fmt writes through os.Stdout, which ends the program with SIGPIPE on a broken pipe.
    if (!s.empty() &&
        os::write(os::stdout_fd, s.data(), s.size()) == os::write_result::broken_pipe) {
        os::die_of_sigpipe();
    }
}

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
    bool shortest = prec < 0;
    if (!shortest && (fmt == 'f' || fmt == 'e' || fmt == 'E')) {
        // A precision: std::to_chars rounds the exact value correctly (ties to even), as
        // strconv does, and writes the same forms (d.ddde+dd, at least two exponent digits).
        std::string buf(static_cast<std::size_t>(330 + prec), '\0');
        std::to_chars_result r = std::to_chars(
            buf.data(), buf.data() + buf.size(), v,
            fmt == 'f' ? std::chars_format::fixed : std::chars_format::scientific, prec);
        for (char* c = buf.data(); c != r.ptr; c++) {
            out += fmt == 'E' && *c == 'e' ? 'E' : *c;
        }
        return;
    }
    // 'g', 'G' and the shortest forms, from the digits.
    int digs = prec;
    if (!shortest && (fmt == 'g' || fmt == 'G') && digs == 0) {
        digs = 1;
    }
    decimal d = digits_of(v, bits, shortest ? -1 : digs);
    int nd = static_cast<int>(d.d.size());
    if (shortest) {
        prec = fmt == 'e' || fmt == 'E' ? nd - 1 : fmt == 'f' ? std::max(nd - d.dp, 0) : nd;
    } else if (fmt == 'g' || fmt == 'G') {
        prec = digs;
    }
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
    int exp = d.dp - 1;
    if (exp < -4 || exp >= eprec) {
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

// --- fmt: format.go ---

void pad(printer& p, std::string_view s) {
    if (!p.wid_present || p.wid == 0) {
        p.buf += s;
        return;
    }
    int64_t width = p.wid - rune_count(s);
    if (!p.minus) {
        write_padding(p, width);
        p.buf += s;
    } else {
        p.buf += s;
        write_padding(p, width);
    }
}

void fmt_s(printer& p, std::string_view s) {
    pad(p, truncate(p, s));
}

void fmt_0x64(printer& p, uint64_t v, bool leading0x) {
    bool sharp = p.sharp;
    p.sharp = leading0x;
    fmt_integer(p, v, 16, false, ldigits);
    p.sharp = sharp;
}

// --- fmt: print.go ---

void bad_verb_begin(printer& p, char32_t verb, type_fn type) {
    p.erroring = true;
    p.buf += "%!";
    append_rune(p.buf, verb);
    p.buf += '(';
    type(p.buf);
    p.buf += '=';
}

void bad_verb_end(printer& p) {
    p.buf += ')';
    p.erroring = false;
}

void pp_bool(printer& p, bool v, char32_t verb, type_fn type) {
    if (verb == 't' || verb == 'v') {
        fmt_boolean(p, v);
        return;
    }
    bad_verb_begin(p, verb, type);
    fmt_boolean(p, v);
    bad_verb_end(p);
}

void pp_int(printer& p, uint64_t v, bool is_signed, char32_t verb, type_fn type) {
    switch (verb) {
    case 'v':
    case 'd':
        fmt_integer(p, v, 10, is_signed, ldigits);
        return;
    case 'q':
        fmt_qc(p, v);
        return;
    default:
        bad_verb_begin(p, verb, type);
        fmt_integer(p, v, 10, is_signed, ldigits);
        bad_verb_end(p);
    }
}

void pp_float(printer& p, double v, int size, char32_t verb, type_fn type) {
    switch (verb) {
    case 'v':
        fmt_float(p, v, size, 'g', -1);
        return;
    case 'f':
        fmt_float(p, v, size, 'f', 6);
        return;
    default:
        bad_verb_begin(p, verb, type);
        fmt_float(p, v, size, 'g', -1);
        bad_verb_end(p);
    }
}

// pp_complex is fmtComplex: "(", the parts as floats of half the size with the verb, the
// imaginary part always with a sign, "i)". A verb that doesn't fit is found first, so the
// error form holds the whole number: %!d(complex128=(1+2i)). (Go's other float verbs, %e %g
// %x and so on, come with Go's fmt in Phase 09; here they are bad verbs, as for floats.)
void pp_complex(printer& p, double re, double im, int size, char32_t verb, type_fn type) {
    switch (verb) {
    case 'v':
    case 'f': {
        bool old_plus = p.plus;
        p.buf += '(';
        pp_float(p, re, size / 2, verb, type);
        // The imaginary part always has a sign.
        p.plus = true;
        pp_float(p, im, size / 2, verb, type);
        p.buf += "i)";
        p.plus = old_plus;
        return;
    }
    default:
        bad_verb_begin(p, verb, type);
        pp_complex(p, re, im, size, 'v', type);
        bad_verb_end(p);
    }
}

void pp_string(printer& p, std::string_view v, char32_t verb, type_fn type) {
    switch (verb) {
    case 'v':
    case 's':
        fmt_s(p, v);
        return;
    case 'q':
        fmt_q(p, v);
        return;
    default:
        bad_verb_begin(p, verb, type);
        fmt_s(p, v);
        bad_verb_end(p);
    }
}

void pp_bytes(printer& p, std::string_view v, char32_t verb) {
    if (verb == 'q') {
        fmt_q(p, v);
    } else {
        fmt_s(p, v);
    }
}

void catch_panic(printer& p, bool nil_receiver, char32_t verb, const char* method,
                 std::string_view value) {
    // fmt recovers the panic, which clears the goroutine's signal (gocpp/panic.hpp).
    gp::detail::clear_signal();
    if (nil_receiver) {
        // A nil pointer receiver: "<nil>" is a nice result.
        p.buf += "<nil>";
        return;
    }
    printer saved = p;
    p.clear_flags();
    p.buf += "%!";
    append_rune(p.buf, verb);
    p.buf += "(PANIC=";
    p.buf += method;
    p.buf += " method: ";
    p.buf += value;
    p.buf += ')';
    p.wid_present = saved.wid_present;
    p.prec_present = saved.prec_present;
    p.minus = saved.minus;
    p.plus = saved.plus;
    p.sharp = saved.sharp;
    p.space = saved.space;
    p.zero = saved.zero;
    p.plus_v = saved.plus_v;
    p.wid = saved.wid;
    p.prec = saved.prec;
}

void sprint(std::string& out, const arg* args, std::size_t n) {
    printer p(out);
    bool prev_string = false;
    for (std::size_t i = 0; i < n; i++) {
        bool is_string = args[i].is_string;
        // Go adds a space between two arguments when neither is a string.
        if (i > 0 && !is_string && !prev_string) {
            out += ' ';
        }
        args[i].print(p, args[i].v, 'v');
        prev_string = is_string;
    }
}

void sprintln(std::string& out, const arg* args, std::size_t n) {
    printer p(out);
    for (std::size_t i = 0; i < n; i++) {
        if (i > 0) {
            out += ' ';
        }
        args[i].print(p, args[i].v, 'v');
    }
    out += '\n';
}

// sprintf is doPrintf.
void sprintf(std::string& out, std::string_view format, const arg* a, std::size_t n) {
    printer p(out);
    format_state st;
    std::size_t end = format.size();
    std::size_t arg_num = 0;  // we process one argument per non-trivial format
    bool after_index = false; // the previous item in format was an index like [3]
    for (std::size_t i = 0; i < end;) {
        st.good_arg_num = true;
        std::size_t lasti = i;
        while (i < end && format[i] != '%') {
            i++;
        }
        if (i > lasti) {
            out += format.substr(lasti, i - lasti);
        }
        if (i >= end) {
            break;
        }
        i++; // the %
        p.clear_flags();
        for (; i < end; i++) {
            char c = format[i];
            if (c == '#') {
                p.sharp = true;
            } else if (c == '0') {
                p.zero = true;
            } else if (c == '+') {
                p.plus = true;
            } else if (c == '-') {
                p.minus = true;
            } else if (c == ' ') {
                p.space = true;
            } else {
                break;
            }
        }
        // An explicit argument index?
        std::tie(arg_num, i, after_index) = arg_number(st, arg_num, format, i, n);
        // A width?
        if (i < end && format[i] == '*') {
            i++;
            std::tie(p.wid, p.wid_present, arg_num) = int_from_arg(a, n, arg_num);
            if (!p.wid_present) {
                out += "%!(BADWIDTH)";
            }
            // A negative width: take its value and set the minus flag.
            if (p.wid < 0) {
                p.wid = -p.wid;
                p.minus = true;
                p.zero = false; // Do not pad with zeros to the right.
            }
            after_index = false;
        } else {
            std::tie(p.wid, p.wid_present, i) = parsenum(format, i, end);
            if (after_index && p.wid_present) { // "%[3]2d"
                st.good_arg_num = false;
            }
        }
        // A precision?
        if (i + 1 < end && format[i] == '.') {
            i++;
            if (after_index) { // "%[3].2d"
                st.good_arg_num = false;
            }
            std::tie(arg_num, i, after_index) = arg_number(st, arg_num, format, i, n);
            if (i < end && format[i] == '*') {
                i++;
                std::tie(p.prec, p.prec_present, arg_num) = int_from_arg(a, n, arg_num);
                // A negative precision makes no sense.
                if (p.prec < 0) {
                    p.prec = 0;
                    p.prec_present = false;
                }
                if (!p.prec_present) {
                    out += "%!(BADPREC)";
                }
                after_index = false;
            } else {
                std::tie(p.prec, p.prec_present, i) = parsenum(format, i, end);
                if (!p.prec_present) {
                    p.prec = 0;
                    p.prec_present = true;
                }
            }
        }
        if (!after_index) {
            std::tie(arg_num, i, after_index) = arg_number(st, arg_num, format, i, n);
        }
        if (i >= end) {
            out += "%!(NOVERB)";
            break;
        }
        auto [verb, size] = decode(format.substr(i));
        i += size;
        if (verb == '%') { // Percent does not absorb operands and ignores width and precision.
            out += '%';
        } else if (!st.good_arg_num) {
            bad_arg(p, verb, "(BADINDEX)");
        } else if (arg_num >= n) { // No argument left over to print for the current verb.
            bad_arg(p, verb, "(MISSING)");
        } else {
            if (verb == 'v' || verb == 'w') {
                // # is Go syntax (Phase 09); + is struct-field syntax.
                p.sharp = false;
                p.plus_v = p.plus;
                p.plus = false;
            }
            a[arg_num].print(p, a[arg_num].v, verb);
            arg_num++;
        }
    }
    // Extra arguments, unless the format used indexes.
    if (!st.reordered && arg_num < n) {
        p.clear_flags();
        out += "%!(EXTRA ";
        for (std::size_t k = arg_num; k < n; k++) {
            if (k > arg_num) {
                out += ", ";
            }
            a[k].type(out);
            out += '=';
            a[k].print(p, a[k].v, 'v');
        }
        out += ')';
    }
}

} // namespace gp::fmtshim::detail
