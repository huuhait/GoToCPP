// The scanning half of Go's package fmt (gocpp/fmtscan.hpp): a port of Go 1.25's fmt/scan.go and
// of the parts of strconv it uses (ParseInt, ParseUint, ParseFloat, Unquote), driven by type
// descriptors where Go's is driven by reflect. Each function mirrors the Go function of the same
// name (written in snake_case). Standard C++ only: the OS is reached through gocpp/os.hpp.
//
// Go's panics inside the scanner (scanError, and panic(io.EOF)) are C++ exceptions of private
// types here; the functions that Go's errorHandler wraps catch them and return the error.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>

#include <charconv>
#include <cmath>
#include <atomic>
#include <bit>
#include <cstdint>
#include <cstring>
#include <limits>
#include <string>
#include <vector>
#include <string_view>
#include <system_error>
#include <tuple>

namespace gp::fmt {

namespace {

// errors.New in the runtime: the *errors.errorString that the scanner returns (the descriptor
// has the type's name and its field, so %v, %T and %#v print as Go's).
struct err_string {
    gp::string gp_s;
};

gp::string thunk_err_error(const iface& self) {
    return unbox<gp::ptr<err_string>>(self)->gp_s;
}

const type_desc* td_err_ptr() {
    static const method pmethods[] = {
        method_of<gp::string(), &thunk_err_error>("Error", nullptr, "func() string")};
    static const field_desc f[] = {
        field_desc("s", "errors", &td_string, nullptr, false, offset_of(&err_string::gp_s))};
    static const type_desc td = named<err_string>("errors.errorString", "errors",
                                                  shape_struct(f, 1), {}, methods_of(pmethods));
    static const type_desc* const p = td_ptr<err_string>(&td);
    return p;
}

gp::error make_error(std::string_view s) {
    return to_iface<gp::error>(gp::new_<err_string>(err_string{gp::string::gp_copy(s)}),
                               td_err_ptr());
}

} // namespace

gp::error io_eof() {
    static const gp::error e = make_error("EOF");
    return e;
}

gp::error io_unexpected_eof() {
    static const gp::error e = make_error("unexpected EOF");
    return e;
}

namespace detail {

namespace {

constexpr int64_t huge_wid = 1 << 30;
constexpr int32_t eof = -1;
constexpr int32_t rune_error = 0xFFFD;

// scan_error is Go's scanError: an error found by the scanner. scan_eof is panic(io.EOF).
struct scan_error {
    gp::error err;
};
struct scan_eof {};

// A ScanState method can raise scanError through generated Go code, whose defer frames must see
// a Go panic rather than adopting a private C++ exception as runtime.CppException. Keep the fast
// internal exceptions, but give scanner-visible scanError values Go's type and field layout.
const type_desc* td_scan_error() {
    static const field_desc fields[] = {
        field_desc("err", "fmt", &td_error, nullptr, false, offset_of(&scan_error::err))};
    static const type_desc descriptor =
        named<scan_error>("fmt.scanError", "fmt", shape_struct(fields, 1));
    return &descriptor;
}

bool scan_error_value(const gp::any& value, gp::error& err) {
    if (value.gp_type() != td_scan_error()) {
        return false;
    }
    err = unbox<scan_error>(value).err;
    return true;
}

[[noreturn]] void fail(const gp::error& err) {
    throw scan_error{err};
}
[[noreturn]] void fail(std::string_view msg) {
    throw scan_error{make_error(msg)};
}

[[noreturn]] void nil_deref() {
    throw gp::panic_error(
        gp::string::gp_copy("runtime error: invalid memory address or nil pointer dereference"));
}

// --- UTF-8 ---

// decode is utf8.DecodeRune of the n > 0 bytes at p.
std::pair<int32_t, int> decode(const char* p, std::size_t n) noexcept {
    auto c = static_cast<unsigned char>(p[0]);
    if (c < 0x80) {
        return {c, 1};
    }
    int64_t width = 1;
    int32_t r = gp::detail::decode_rune(p, static_cast<int64_t>(n), width);
    return {r, static_cast<int>(width)};
}

// decode_at is utf8.DecodeRuneInString(s[i:]): (RuneError, 0) at the end.
std::pair<int32_t, int> decode_at(std::string_view s, std::size_t i) noexcept {
    if (i >= s.size()) {
        return {rune_error, 0};
    }
    return decode(s.data() + i, s.size() - i);
}

// full_rune is utf8.FullRune of the n > 0 bytes at p.
bool full_rune(const unsigned char* p, int n) noexcept {
    unsigned char c = p[0];
    int need;
    unsigned char lo = 0x80;
    unsigned char hi = 0xBF;
    if (c < 0x80) {
        return true;
    } else if (c >= 0xC2 && c <= 0xDF) {
        need = 2;
    } else if (c >= 0xE0 && c <= 0xEF) {
        need = 3;
        if (c == 0xE0) {
            lo = 0xA0;
        } else if (c == 0xED) {
            hi = 0x9F;
        }
    } else if (c >= 0xF0 && c <= 0xF4) {
        need = 4;
        if (c == 0xF0) {
            lo = 0x90;
        } else if (c == 0xF4) {
            hi = 0x8F;
        }
    } else {
        return true; // an invalid first byte is a full (one byte) error rune
    }
    if (n >= need) {
        return true;
    }
    if (n > 1 && (p[1] < lo || hi < p[1])) {
        return true;
    }
    if (n > 2 && (p[2] < 0x80 || 0xBF < p[2])) {
        return true;
    }
    return false;
}

int rune_len(int32_t r) noexcept {
    if (r < 0) {
        return -1;
    } else if (r < 0x80) {
        return 1;
    } else if (r < 0x800) {
        return 2;
    } else if (r >= 0xD800 && r <= 0xDFFF) {
        return -1;
    } else if (r < 0x10000) {
        return 3;
    } else if (r <= 0x10FFFF) {
        return 4;
    }
    return -1;
}

// append_rune is utf8.AppendRune: an invalid rune is written as U+FFFD.
void append_rune(std::string& out, int32_t r) {
    if (r < 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF)) {
        r = rune_error;
    }
    auto u = static_cast<uint32_t>(r);
    if (u < 0x80) {
        out += static_cast<char>(u);
    } else if (u < 0x800) {
        out += static_cast<char>(0xC0 | (u >> 6));
        out += static_cast<char>(0x80 | (u & 0x3F));
    } else if (u < 0x10000) {
        out += static_cast<char>(0xE0 | (u >> 12));
        out += static_cast<char>(0x80 | ((u >> 6) & 0x3F));
        out += static_cast<char>(0x80 | (u & 0x3F));
    } else {
        out += static_cast<char>(0xF0 | (u >> 18));
        out += static_cast<char>(0x80 | ((u >> 12) & 0x3F));
        out += static_cast<char>(0x80 | ((u >> 6) & 0x3F));
        out += static_cast<char>(0x80 | (u & 0x3F));
    }
}

std::string rune_string(int32_t r) {
    std::string s;
    append_rune(s, r);
    return s;
}

// --- strconv: the parsing the scanner uses ---

char lower(char c) noexcept {
    return static_cast<char>(c | ('x' - 'X'));
}

// A *strconv.NumError's text: strconv.Func: parsing "num": err.
std::string num_error_text(std::string_view fn, std::string_view num, std::string_view err) {
    std::string out = "strconv.";
    out += fn;
    out += ": parsing ";
    append_quote(out, num);
    out += ": ";
    out += err;
    return out;
}

constexpr std::string_view err_syntax = "invalid syntax";
constexpr std::string_view err_range = "value out of range";

// underscore_ok reports whether the underscores of s (a number with a base prefix) are placed as
// Go allows (strconv.underscoreOK).
bool underscore_ok(std::string_view s) noexcept {
    char saw = '^';
    std::size_t i = 0;
    if (!s.empty() && (s[0] == '-' || s[0] == '+')) {
        s.remove_prefix(1);
    }
    bool hex = false;
    if (s.size() >= 2 && s[0] == '0' &&
        (lower(s[1]) == 'b' || lower(s[1]) == 'o' || lower(s[1]) == 'x')) {
        i = 2;
        saw = '0';
        hex = lower(s[1]) == 'x';
    }
    for (; i < s.size(); i++) {
        char c = s[i];
        if ((c >= '0' && c <= '9') || (hex && lower(c) >= 'a' && lower(c) <= 'f')) {
            saw = '0';
            continue;
        }
        if (c == '_') {
            if (saw != '0') {
                return false;
            }
            saw = '_';
            continue;
        }
        if (saw == '_') {
            return false;
        }
        saw = '!';
    }
    return saw != '_';
}

// parse_uint is strconv.ParseUint(s, base, 64): ok, or the error text (without the NumError
// frame, which the caller adds with its own function name).
enum class num_result { ok, syntax, range };

num_result parse_uint(std::string_view s, int base, uint64_t& out) {
    out = 0;
    if (s.empty()) {
        return num_result::syntax;
    }
    bool base0 = base == 0;
    std::string_view s0 = s;
    if (base >= 2 && base <= 36) {
    } else if (base == 0) {
        base = 10;
        if (s[0] == '0') {
            if (s.size() >= 3 && lower(s[1]) == 'b') {
                base = 2;
                s.remove_prefix(2);
            } else if (s.size() >= 3 && lower(s[1]) == 'o') {
                base = 8;
                s.remove_prefix(2);
            } else if (s.size() >= 3 && lower(s[1]) == 'x') {
                base = 16;
                s.remove_prefix(2);
            } else {
                base = 8;
                s.remove_prefix(1);
            }
        }
    } else {
        return num_result::syntax;
    }
    const uint64_t max_val = std::numeric_limits<uint64_t>::max();
    const uint64_t cutoff = max_val / static_cast<uint64_t>(base) + 1;
    bool underscores = false;
    uint64_t n = 0;
    for (char c : s) {
        unsigned d;
        if (c == '_' && base0) {
            underscores = true;
            continue;
        } else if (c >= '0' && c <= '9') {
            d = static_cast<unsigned>(c - '0');
        } else if (lower(c) >= 'a' && lower(c) <= 'z') {
            d = static_cast<unsigned>(lower(c) - 'a' + 10);
        } else {
            return num_result::syntax;
        }
        if (d >= static_cast<unsigned>(base)) {
            return num_result::syntax;
        }
        if (n >= cutoff) {
            out = max_val;
            return num_result::range;
        }
        n *= static_cast<uint64_t>(base);
        uint64_t n1 = n + d;
        if (n1 < n) {
            out = max_val;
            return num_result::range;
        }
        n = n1;
    }
    if (underscores && !underscore_ok(s0)) {
        return num_result::syntax;
    }
    out = n;
    return num_result::ok;
}

// parse_int is strconv.ParseInt(s, base, 64).
num_result parse_int(std::string_view s, int base, int64_t& out) {
    out = 0;
    if (s.empty()) {
        return num_result::syntax;
    }
    bool neg = false;
    if (s[0] == '+') {
        s.remove_prefix(1);
    } else if (s[0] == '-') {
        neg = true;
        s.remove_prefix(1);
    }
    uint64_t un;
    num_result r = parse_uint(s, base, un);
    if (r == num_result::syntax) {
        return r;
    }
    const uint64_t cutoff = uint64_t(1) << 63;
    if (r == num_result::range || (!neg && un >= cutoff) || (neg && un > cutoff)) {
        return num_result::range;
    }
    out = neg ? -static_cast<int64_t>(un) : static_cast<int64_t>(un);
    return num_result::ok;
}

// parse_float is strconv.ParseFloat(s, bits). On a range error it gives ±Inf.
num_result parse_float(std::string_view s, int bits, double& out) {
    out = 0;
    if (s.empty()) {
        return num_result::syntax;
    }
    // special: ±inf, ±infinity, nan, without regard to case.
    {
        std::string_view t = s;
        int sign = 1;
        bool signed_ = false;
        auto ieq = [](std::string_view a, std::string_view prefix) {
            if (a.size() < prefix.size()) {
                return false;
            }
            for (std::size_t i = 0; i < prefix.size(); i++) {
                if (lower(a[i]) != prefix[i]) {
                    return false;
                }
            }
            return true;
        };
        if (t[0] == '+' || t[0] == '-') {
            sign = t[0] == '-' ? -1 : 1;
            signed_ = true;
            t.remove_prefix(1);
        }
        if (!t.empty() && (t[0] == 'i' || t[0] == 'I')) {
            std::size_t n = 0;
            if (ieq(t, "infinity")) {
                n = 8;
            } else if (ieq(t, "inf")) {
                n = 3;
            }
            if (n != 0) {
                if (n + (signed_ ? 1 : 0) != s.size()) {
                    return num_result::syntax;
                }
                out = sign * std::numeric_limits<double>::infinity();
                return num_result::ok;
            }
        } else if (!signed_ && ieq(t, "nan")) {
            if (t.size() != 3) {
                return num_result::syntax;
            }
            out = std::bit_cast<double>(UINT64_C(0x7FF8000000000001)); // math.NaN()
            return num_result::ok;
        }
    }
    // readFloat: the syntax of a decimal or hexadecimal floating-point number with underscores.
    std::size_t i = 0;
    bool neg = false;
    if (s[i] == '+') {
        i++;
    } else if (s[i] == '-') {
        neg = true;
        i++;
    }
    int base = 10;
    if (i + 2 < s.size() && s[i] == '0' && lower(s[i + 1]) == 'x') {
        base = 16;
        i += 2;
    }
    bool underscores = false;
    bool saw_dot = false;
    bool saw_digits = false;
    for (; i < s.size(); i++) {
        char c = s[i];
        if (c == '_') {
            underscores = true;
            continue;
        } else if (c == '.') {
            if (saw_dot) {
                break;
            }
            saw_dot = true;
            continue;
        } else if (c >= '0' && c <= '9') {
            saw_digits = true;
            continue;
        } else if (base == 16 && lower(c) >= 'a' && lower(c) <= 'f') {
            saw_digits = true;
            continue;
        }
        break;
    }
    if (!saw_digits) {
        return num_result::syntax;
    }
    if (i < s.size() && lower(s[i]) == (base == 16 ? 'p' : 'e')) {
        i++;
        if (i >= s.size()) {
            return num_result::syntax;
        }
        if (s[i] == '+' || s[i] == '-') {
            i++;
        }
        if (i >= s.size() || s[i] < '0' || s[i] > '9') {
            return num_result::syntax;
        }
        for (; i < s.size() && ((s[i] >= '0' && s[i] <= '9') || s[i] == '_'); i++) {
            if (s[i] == '_') {
                underscores = true;
            }
        }
    } else if (base == 16) {
        return num_result::syntax; // a hexadecimal mantissa needs a 'p' exponent
    }
    if (underscores && !underscore_ok(s.substr(0, i))) {
        return num_result::syntax;
    }
    if (i != s.size()) {
        return num_result::syntax;
    }
    // The text without the sign, the prefix and the underscores, for the conversion.
    std::string clean;
    std::size_t start = (s[0] == '+' || s[0] == '-') ? 1 : 0;
    if (base == 16) {
        start += 2;
    }
    for (std::size_t k = start; k < s.size(); k++) {
        if (s[k] != '_') {
            clean += s[k];
        }
    }
    auto fmt = base == 16 ? std::chars_format::hex : std::chars_format::general;
    const char* b = clean.data();
    const char* e = clean.data() + clean.size();
    double v = 0;
    bool overflow = false;
    if (bits == 32) {
        float f = 0;
        auto res = std::from_chars(b, e, f, fmt);
        if (res.ec == std::errc::result_out_of_range) {
            double d = std::strtod(clean.c_str(), nullptr);
            if (base == 16) {
                d = std::strtod(("0x" + clean).c_str(), nullptr);
            }
            overflow = std::fabs(d) > 1;
            f = overflow ? std::numeric_limits<float>::infinity() : 0.0f;
        } else if (res.ec != std::errc() || res.ptr != e) {
            return num_result::syntax;
        }
        v = f;
    } else {
        auto res = std::from_chars(b, e, v, fmt);
        if (res.ec == std::errc::result_out_of_range) {
            std::string c = base == 16 ? "0x" + clean : clean;
            double d = std::strtod(c.c_str(), nullptr);
            overflow = std::fabs(d) > 1;
            v = overflow ? std::numeric_limits<double>::infinity() : 0.0;
        } else if (res.ec != std::errc() || res.ptr != e) {
            return num_result::syntax;
        }
    }
    out = neg ? -v : v;
    return overflow ? num_result::range : num_result::ok;
}

// hex_value is the value of the hexadecimal digit c, or -1.
int hex_value(int32_t c) noexcept {
    if (c >= '0' && c <= '9') {
        return c - '0';
    }
    if (c >= 'a' && c <= 'f') {
        return c - 'a' + 10;
    }
    if (c >= 'A' && c <= 'F') {
        return c - 'A' + 10;
    }
    return -1;
}

// unquote is strconv.Unquote of a double-quoted string s (s[0] and the last byte are quotes). It
// returns false for a syntax error.
bool unquote(std::string_view s, std::string& out) {
    out.clear();
    if (s.size() < 2 || s.front() != '"' || s.back() != '"') {
        return false;
    }
    s = s.substr(1, s.size() - 2);
    std::size_t i = 0;
    while (i < s.size()) {
        char c = s[i];
        if (c == '"' || c == '\n') {
            return false;
        }
        if (c != '\\') {
            if (static_cast<unsigned char>(c) < 0x80) {
                out += c;
                i++;
            } else {
                auto [r, w] = decode_at(s, i);
                append_rune(out, r);
                i += static_cast<std::size_t>(w);
            }
            continue;
        }
        i++;
        if (i >= s.size()) {
            return false;
        }
        char e = s[i++];
        switch (e) {
        case 'a':
            out += '\a';
            break;
        case 'b':
            out += '\b';
            break;
        case 'f':
            out += '\f';
            break;
        case 'n':
            out += '\n';
            break;
        case 'r':
            out += '\r';
            break;
        case 't':
            out += '\t';
            break;
        case 'v':
            out += '\v';
            break;
        case '\\':
            out += '\\';
            break;
        case '"':
            out += '"';
            break;
        case 'x':
        case 'u':
        case 'U': {
            int n = e == 'x' ? 2 : e == 'u' ? 4 : 8;
            if (s.size() - i < static_cast<std::size_t>(n)) {
                return false;
            }
            uint32_t v = 0;
            for (int j = 0; j < n; j++) {
                int d = hex_value(static_cast<unsigned char>(s[i + static_cast<std::size_t>(j)]));
                if (d < 0) {
                    return false;
                }
                v = v << 4 | static_cast<uint32_t>(d);
            }
            i += static_cast<std::size_t>(n);
            if (e == 'x') {
                out += static_cast<char>(v);
            } else {
                if (v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF)) {
                    return false;
                }
                append_rune(out, static_cast<int32_t>(v));
            }
            break;
        }
        case '0':
        case '1':
        case '2':
        case '3':
        case '4':
        case '5':
        case '6':
        case '7': {
            if (s.size() - i < 2) {
                return false;
            }
            uint32_t v = static_cast<uint32_t>(e - '0');
            for (int j = 0; j < 2; j++) {
                char d = s[i + static_cast<std::size_t>(j)];
                if (d < '0' || d > '7') {
                    return false;
                }
                v = v << 3 | static_cast<uint32_t>(d - '0');
            }
            i += 2;
            if (v > 255) {
                return false;
            }
            out += static_cast<char>(v);
            break;
        }
        default:
            return false;
        }
    }
    return true;
}

// --- the input ---

// A byte_source is the bytes of an input: the string of Sscan, standard input, an io.Reader.
struct byte_source {
    virtual ~byte_source() = default;
    // read_byte returns the next byte, or -1 and the error (io.EOF at the end).
    virtual int read_byte(gp::error& err) = 0;
};

struct string_bytes final : byte_source {
    std::string_view s;
    std::size_t i = 0;
    explicit string_bytes(std::string_view str) : s(str) {}
    int read_byte(gp::error& err) override {
        if (i < s.size()) {
            return static_cast<unsigned char>(s[i++]);
        }
        err = io_eof();
        return -1;
    }
};

struct stdin_bytes final : byte_source {
    int read_byte(gp::error& err) override {
        char c;
        for (;;) {
            std::int64_t n = os::read(0, &c, 1);
            if (n == 1) {
                return static_cast<unsigned char>(c);
            }
            if (n == 0) {
                err = io_eof();
            } else {
                err = make_error(std::string("read /dev/stdin: ") + os::read_error());
            }
            return -1;
        }
    }
};

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

constexpr const char* read_sig = "func([]uint8) (int, error)";
constexpr const char* read_rune_sig = "func() (int32, int, error)";
constexpr const char* unread_rune_sig = "func() error";

// reader_bytes reads an io.Reader one byte at a time, as io.ReadFull(r, buf[:1]) does.
struct reader_bytes final : byte_source {
    const iface& r;
    const method* read = nullptr;
    gp::slice<uint8_t> one;
    explicit reader_bytes(const iface& rd) : r(rd) {
        if (r.gp_type() != nullptr) {
            read = find_method(r.gp_type(), "Read", read_sig);
        }
        one = gp::make_slice<uint8_t>(1);
    }
    int read_byte(gp::error& err) override {
        // A nil reader is valid until a scan actually needs input (Fscan(nil) needs none).
        if (read == nullptr) {
            nil_deref();
        }
        for (;;) {
            auto [n, e] =
                reinterpret_cast<thunk_t<std::tuple<int64_t, gp::error>(gp::slice<uint8_t>)>*>(
                    read->gp_fn())(r, one);
            if (n >= 1) {
                return one.gp_data()[0];
            }
            if (e != nullptr) {
                err = std::move(e);
                return -1;
            }
        }
    }
};

// A rune_source is Go's io.RuneScanner.
struct rune_source {
    virtual ~rune_source() = default;
    virtual int32_t read_rune(int64_t& size, gp::error& err) = 0;
    virtual gp::error unread_rune() = 0;
};

// rune_reader is Go's readRune: UTF-8 code points from a byte source, for an input without ReadRune
// and UnreadRune.
struct rune_reader final : rune_source {
    byte_source& src;
    int pending = 0;                // the bytes in pend_buf; only > 0 for bad UTF-8
    unsigned char pend_buf[4] = {}; // bytes left over
    unsigned char buf[4] = {};
    int32_t peek_rune = -1; // if >= 0 the next rune; when < 0 it is ^(the previous rune)
    explicit rune_reader(byte_source& s) : src(s) {}

    int read_byte(gp::error& err) {
        if (pending > 0) {
            int b = pend_buf[0];
            std::memmove(pend_buf, pend_buf + 1, 3);
            pending--;
            return b;
        }
        return src.read_byte(err);
    }

    int32_t read_rune(int64_t& size, gp::error& err) override {
        size = 0;
        if (peek_rune >= 0) {
            int32_t rr = peek_rune;
            peek_rune = ~peek_rune;
            size = rune_len(rr);
            return rr;
        }
        int b = read_byte(err);
        if (b < 0) {
            return 0;
        }
        buf[0] = static_cast<unsigned char>(b);
        if (buf[0] < 0x80) {
            int32_t rr = buf[0];
            size = 1;
            peek_rune = ~rr;
            return rr;
        }
        int n = 1;
        while (!full_rune(buf, n)) {
            b = read_byte(err);
            if (b < 0) {
                if (err == io_eof()) {
                    err = gp::error();
                    break;
                }
                return 0;
            }
            buf[n] = static_cast<unsigned char>(b);
            n++;
        }
        auto [rr, sz] = decode(reinterpret_cast<const char*>(buf), static_cast<std::size_t>(n));
        size = sz;
        if (sz < n) {
            // An error: save the bytes for the next read.
            std::memcpy(pend_buf + pending, buf + sz, static_cast<std::size_t>(n - sz));
            pending += n - sz;
        }
        peek_rune = ~rr;
        return rr;
    }

    gp::error unread_rune() override {
        if (peek_rune >= 0) {
            return make_error("fmt: scanning called UnreadRune with no rune available");
        }
        peek_rune = ~peek_rune;
        return gp::error();
    }
};

// dyn_runes is a reader that has the methods of io.RuneScanner: they are called as they are.
struct dyn_runes final : rune_source {
    const iface& r;
    const method* rr;
    const method* ur;
    dyn_runes(const iface& x, const method* a, const method* b) : r(x), rr(a), ur(b) {}
    int32_t read_rune(int64_t& size, gp::error& err) override {
        auto [c, n, e] =
            reinterpret_cast<thunk_t<std::tuple<int32_t, int64_t, gp::error>()>*>(rr->gp_fn())(r);
        size = n;
        err = std::move(e);
        return c;
    }
    gp::error unread_rune() override {
        return reinterpret_cast<thunk_t<gp::error()>*>(ur->gp_fn())(r);
    }
};

// --- the scanner state ---

// The space table of Go's fmt: a copy of the unicode.White_Space ranges.
constexpr uint16_t space[][2] = {
    {0x0009, 0x000d}, {0x0020, 0x0020}, {0x0085, 0x0085}, {0x00a0, 0x00a0}, {0x1680, 0x1680},
    {0x2000, 0x200a}, {0x2028, 0x2029}, {0x202f, 0x202f}, {0x205f, 0x205f}, {0x3000, 0x3000},
};

bool is_space(int32_t r) noexcept {
    if (r >= 1 << 16 || r < 0) {
        return false;
    }
    auto rx = static_cast<uint16_t>(r);
    for (const auto& rng : space) {
        if (rx < rng[0]) {
            return false;
        }
        if (rx <= rng[1]) {
            return true;
        }
    }
    return false;
}

constexpr std::string_view binary_digits = "01";
constexpr std::string_view octal_digits = "01234567";
constexpr std::string_view decimal_digits = "0123456789";
constexpr std::string_view hexadecimal_digits = "0123456789aAbBcCdDeEfF";
constexpr std::string_view sign_chars = "+-";
constexpr std::string_view period = ".";
constexpr std::string_view float_verbs = "beEfFgGv";

// index_rune is Go's indexRune: the byte offset of r in the UTF-8 string s, or -1.
int64_t index_rune(std::string_view s, int32_t r) noexcept {
    for (std::size_t i = 0; i < s.size();) {
        auto [c, w] = decode_at(s, i);
        if (c == r) {
            return static_cast<int64_t>(i);
        }
        i += static_cast<std::size_t>(w);
    }
    return -1;
}

const method* find_scan_method(const type_desc* t) noexcept {
    return find_method(t, "Scan", "func(fmt.ScanState, int32) error");
}

struct ss;
struct ss_state {
    ss* p;
    // Once the scan is over (p is null): whether the freed scanner of Go would still answer
    // ReadRune with io.EOF (it had met the end, or its argument limit), else it has a nil reader
    // and panics.
    bool dead_eof = false;
    int64_t dead_wid = -1; // the width of the last argument (Width() after the scan), -1: none
};

ScanState make_scan_state(ss& s);

// ss is the internal implementation of ScanState.
struct ss {
    ss() = default;
    ss(const ss&) = delete;
    ss& operator=(const ss&) = delete;
    // A ScanState that a Scan method kept outlives the scan, whose ss Go frees: it stops pointing
    // at the scanner, and its methods panic as Go's do on the freed one (nil reader).
    ~ss() {
        if (state != nullptr) {
            (*state)->p = nullptr;
            (*state)->dead_eof = at_eof || count >= arg_limit;
            (*state)->dead_wid = max_wid == huge_wid ? -1 : max_wid;
            delete state;
        }
    }
    gp::ptr<ss_state>* state =
        nullptr;               // the ScanState handed to the Scan methods, made by the first
    live_span live;            // the live []any of a variadic slice, held by with_operands
    rune_source* rs = nullptr; // where to read input
    std::string buf;           // token accumulator
    int64_t count = 0;         // runes consumed so far
    bool at_eof = false;       // already read EOF
    bool nl_is_end = false;    // whether newline terminates scan
    bool nl_is_space = false;  // whether newline counts as white space
    int64_t arg_limit = huge_wid;
    int64_t limit = huge_wid;
    int64_t max_wid = huge_wid; // width of this arg

    // ReadRune is the public one: the runes of the input up to the argument's limit.
    int32_t read_rune_pub(int64_t& size, gp::error& err) {
        size = 0;
        if (at_eof || count >= arg_limit) {
            err = io_eof();
            return 0;
        }
        int32_t r = rs->read_rune(size, err);
        if (err == nullptr) {
            count++;
            if (nl_is_end && r == '\n') {
                at_eof = true;
            }
        } else if (err == io_eof()) {
            at_eof = true;
        }
        return r;
    }

    // get_rune: the return value is eof (-1) at the end of the input; other errors are raised.
    int32_t get_rune() {
        int64_t size;
        gp::error err;
        int32_t r = read_rune_pub(size, err);
        if (err != nullptr) {
            if (err == io_eof()) {
                return eof;
            }
            fail(err);
        }
        return r;
    }

    // must_read_rune turns io.EOF into an error, io.ErrUnexpectedEOF: an EOF is a syntax error
    // here (string scanning).
    int32_t must_read_rune() {
        int32_t r = get_rune();
        if (r == eof) {
            fail(io_unexpected_eof());
        }
        return r;
    }

    void unread_rune() {
        rs->unread_rune();
        at_eof = false;
        count--;
    }

    [[noreturn]] void error_string(std::string_view msg) { fail(msg); }

    void skip_space() {
        for (;;) {
            int32_t r = get_rune();
            if (r == eof) {
                return;
            }
            if (r == '\r' && peek("\n")) {
                continue;
            }
            if (r == '\n') {
                if (nl_is_space) {
                    continue;
                }
                error_string("unexpected newline");
            }
            if (!is_space(r)) {
                unread_rune();
                break;
            }
        }
    }

    // token: the next run of runes that satisfy f, after the space if skip_space.
    void token(bool skip, const gp::func<bool(int32_t)>* f) {
        if (skip) {
            skip_space();
        }
        for (;;) {
            int32_t r = get_rune();
            if (r == eof) {
                break;
            }
            if (!(f == nullptr ? !is_space(r) : (*f)(r))) {
                unread_rune();
                break;
            }
            append_rune(buf, r);
        }
    }

    // consume reads the next rune and reports whether it is in the ok string; if accept, it puts
    // the character into the input token.
    bool consume(std::string_view ok, bool accept) {
        int32_t r = get_rune();
        if (r == eof) {
            return false;
        }
        if (index_rune(ok, r) >= 0) {
            if (accept) {
                append_rune(buf, r);
            }
            return true;
        }
        if (r != eof && accept) {
            unread_rune();
        }
        return false;
    }

    // peek reports whether the next character is in the ok string, without consuming it.
    bool peek(std::string_view ok) {
        int32_t r = get_rune();
        if (r != eof) {
            unread_rune();
        }
        return index_rune(ok, r) >= 0;
    }

    void not_eof() {
        // Guarantee there is data to be read.
        if (get_rune() == eof) {
            throw scan_eof{};
        }
        unread_rune();
    }

    bool accept(std::string_view ok) { return consume(ok, true); }

    // ok_verb verifies that the verb is in the list; an error otherwise.
    bool ok_verb(int32_t verb, std::string_view ok_verbs, std::string_view typ) {
        if (index_rune(ok_verbs, verb) >= 0) {
            return true;
        }
        std::string m = "bad verb '%" + rune_string(verb) + "' for ";
        m += typ;
        error_string(m);
    }

    bool scan_bool(int32_t verb) {
        skip_space();
        not_eof();
        if (!ok_verb(verb, "tv", "boolean")) {
            return false;
        }
        // Syntax-checking a boolean is annoying. We're not fastidious about case.
        switch (get_rune()) {
        case '0':
            return false;
        case '1':
            return true;
        case 't':
        case 'T':
            if (accept("rR") && (!accept("uU") || !accept("eE"))) {
                fail("syntax error scanning boolean");
            }
            return true;
        case 'f':
        case 'F':
            if (accept("aA") && (!accept("lL") || !accept("sS") || !accept("eE"))) {
                fail("syntax error scanning boolean");
            }
            return false;
        }
        return false;
    }

    // get_base returns the numeric base represented by the verb and its digit string.
    std::pair<int, std::string_view> get_base(int32_t verb) {
        ok_verb(verb, "bdoUxXv", "integer");
        switch (verb) {
        case 'b':
            return {2, binary_digits};
        case 'o':
            return {8, octal_digits};
        case 'x':
        case 'X':
        case 'U':
            return {16, hexadecimal_digits};
        }
        return {10, decimal_digits};
    }

    // scan_number returns the numerical string with the digits starting here.
    std::string scan_number(std::string_view digits, bool have_digits) {
        if (!have_digits) {
            not_eof();
            if (!accept(digits)) {
                error_string("expected integer");
            }
        }
        while (accept(digits)) {
        }
        return buf;
    }

    // scan_rune returns the next rune value in the input.
    int64_t scan_rune(int bit_size) {
        not_eof();
        int32_t r = get_rune();
        auto n = static_cast<unsigned>(bit_size);
        int64_t x =
            static_cast<int64_t>(static_cast<uint64_t>(static_cast<int64_t>(r)) << (64 - n)) >>
            (64 - n);
        if (x != static_cast<int64_t>(r)) {
            error_string("overflow on character value " + rune_string(r));
        }
        return r;
    }

    struct base_prefix {
        int base;
        std::string digits;
        bool zero_found;
    };

    // scan_base_prefix reports whether the integer begins with a base prefix. It is called only
    // if the verb is %v.
    base_prefix scan_base_prefix() {
        if (!peek("0")) {
            return {0, std::string(decimal_digits) + "_", false};
        }
        accept("0");
        // Special cases for 0, 0b, 0o, 0x.
        if (peek("bB")) {
            consume("bB", true);
            return {0, std::string(binary_digits) + "_", true};
        } else if (peek("oO")) {
            consume("oO", true);
            return {0, std::string(octal_digits) + "_", true};
        } else if (peek("xX")) {
            consume("xX", true);
            return {0, std::string(hexadecimal_digits) + "_", true};
        }
        return {0, std::string(octal_digits) + "_", true};
    }

    int64_t scan_int(int32_t verb, int bit_size) {
        if (verb == 'c') {
            return scan_rune(bit_size);
        }
        skip_space();
        not_eof();
        auto [base, digits] = get_base(verb);
        std::string digits_s(digits);
        bool have_digits = false;
        if (verb == 'U') {
            if (!consume("U", false) || !consume("+", false)) {
                error_string("bad unicode format ");
            }
        } else {
            accept(sign_chars); // If there's a sign, it will be left in the token buffer.
            if (verb == 'v') {
                base_prefix bp = scan_base_prefix();
                base = bp.base;
                digits_s = bp.digits;
                have_digits = bp.zero_found;
            }
        }
        std::string tok = scan_number(digits_s, have_digits);
        int64_t i;
        switch (parse_int(tok, base, i)) {
        case num_result::syntax:
            fail(num_error_text("ParseInt", tok, err_syntax));
        case num_result::range:
            fail(num_error_text("ParseInt", tok, err_range));
        case num_result::ok:
            break;
        }
        auto n = static_cast<unsigned>(bit_size);
        int64_t x = static_cast<int64_t>(static_cast<uint64_t>(i) << (64 - n)) >> (64 - n);
        if (x != i) {
            error_string("integer overflow on token " + tok);
        }
        return i;
    }

    uint64_t scan_uint(int32_t verb, int bit_size) {
        if (verb == 'c') {
            return static_cast<uint64_t>(scan_rune(bit_size));
        }
        skip_space();
        not_eof();
        auto [base, digits] = get_base(verb);
        std::string digits_s(digits);
        bool have_digits = false;
        if (verb == 'U') {
            if (!consume("U", false) || !consume("+", false)) {
                error_string("bad unicode format ");
            }
        } else if (verb == 'v') {
            base_prefix bp = scan_base_prefix();
            base = bp.base;
            digits_s = bp.digits;
            have_digits = bp.zero_found;
        }
        std::string tok = scan_number(digits_s, have_digits);
        uint64_t i;
        switch (parse_uint(tok, base, i)) {
        case num_result::syntax:
            fail(num_error_text("ParseUint", tok, err_syntax));
        case num_result::range:
            fail(num_error_text("ParseUint", tok, err_range));
        case num_result::ok:
            break;
        }
        auto n = static_cast<unsigned>(bit_size);
        uint64_t x = (i << (64 - n)) >> (64 - n);
        if (x != i) {
            error_string("unsigned integer overflow on token " + tok);
        }
        return i;
    }

    // float_token returns the floating-point number starting here. It's not rigorous about
    // syntax because it doesn't check that we have at least some digits, but ParseFloat will.
    std::string float_token() {
        buf.clear();
        // NaN?
        if (accept("nN") && accept("aA") && accept("nN")) {
            return buf;
        }
        // leading sign?
        accept(sign_chars);
        // Inf?
        if (accept("iI") && accept("nN") && accept("fF")) {
            return buf;
        }
        std::string digits = std::string(decimal_digits) + "_";
        std::string_view exp = "eEpP";
        if (accept("0") && accept("xX")) {
            digits = std::string(hexadecimal_digits) + "_";
            exp = "pP";
        }
        // digits?
        while (accept(digits)) {
        }
        // decimal point?
        if (accept(period)) {
            // fraction?
            while (accept(digits)) {
            }
        }
        // exponent?
        if (accept(exp)) {
            // leading sign?
            accept(sign_chars);
            // digits?
            std::string dd = std::string(decimal_digits) + "_";
            while (accept(dd)) {
            }
        }
        return buf;
    }

    // complex_tokens returns the real and imaginary parts of the complex number starting here: it
    // might be parenthesized and has the format (N+Ni).
    std::pair<std::string, std::string> complex_tokens() {
        bool parens = accept("(");
        std::string real = float_token();
        buf.clear();
        // Must now have a sign.
        if (!accept("+-")) {
            fail("syntax error scanning complex number");
        }
        // Sign is now in buffer
        std::string imag_sign = buf;
        std::string imag = float_token();
        if (!accept("i")) {
            fail("syntax error scanning complex number");
        }
        if (parens && !accept(")")) {
            fail("syntax error scanning complex number");
        }
        return {real, imag_sign + imag};
    }

    // convert_float converts the string to a float64 value of n bits.
    double convert_float(const std::string& str, int n) {
        // ParseFloat handles "+0x1.fp+2", but the non-standard decimal+binary exponent mix
        // (1.2p4) is done here.
        auto p = str.find('p');
        if (p != std::string::npos && str.find_first_of("xX") == std::string::npos) {
            double f;
            switch (parse_float(std::string_view(str).substr(0, p), n, f)) {
            case num_result::syntax:
                fail(num_error_text("ParseFloat", str, err_syntax));
            case num_result::range:
                fail(num_error_text("ParseFloat", str, err_range));
            case num_result::ok:
                break;
            }
            // strconv.Atoi(str[p+1:])
            std::string_view es = std::string_view(str).substr(p + 1);
            int64_t m = 0;
            bool bad = es.empty();
            bool rng = false;
            if (!bad) {
                std::string_view t = es;
                bool neg = false;
                if (t[0] == '+' || t[0] == '-') {
                    neg = t[0] == '-';
                    t.remove_prefix(1);
                }
                uint64_t u;
                num_result r = t.empty() ? num_result::syntax : parse_uint(t, 10, u);
                if (r == num_result::syntax) {
                    bad = true;
                } else if (r == num_result::range || u > (uint64_t(1) << 63) - (neg ? 0 : 1)) {
                    rng = true;
                } else {
                    m = neg ? -static_cast<int64_t>(u) : static_cast<int64_t>(u);
                }
            }
            if (bad) {
                fail(num_error_text("Atoi", str, err_syntax));
            }
            if (rng) {
                fail(num_error_text("Atoi", str, err_range));
            }
            if (m > 100000) {
                m = 100000;
            } else if (m < -100000) {
                m = -100000;
            }
            return std::ldexp(f, static_cast<int>(m));
        }
        double f;
        switch (parse_float(str, n, f)) {
        case num_result::syntax:
            fail(num_error_text("ParseFloat", str, err_syntax));
        case num_result::range:
            fail(num_error_text("ParseFloat", str, err_range));
        case num_result::ok:
            break;
        }
        return f;
    }

    // scan_complex converts the next token to a complex128 value.
    std::pair<double, double> scan_complex(int32_t verb, int n) {
        if (!ok_verb(verb, float_verbs, "complex")) {
            return {0, 0};
        }
        skip_space();
        not_eof();
        auto [sreal, simag] = complex_tokens();
        double real = convert_float(sreal, n / 2);
        double imag = convert_float(simag, n / 2);
        return {real, imag};
    }

    // quoted_string returns the double- or back-quoted string represented by the next input.
    std::string quoted_string() {
        not_eof();
        int32_t quote = get_rune();
        switch (quote) {
        case '`':
            // Back-quoted: Anything goes until EOF or back quote.
            for (;;) {
                int32_t r = must_read_rune();
                if (r == quote) {
                    break;
                }
                append_rune(buf, r);
            }
            return buf;
        case '"': {
            // Double-quoted: include the quotes and let Unquote do the backslash escapes.
            buf += '"';
            for (;;) {
                int32_t r = must_read_rune();
                append_rune(buf, r);
                if (r == '\\') {
                    // In a legal backslash escape, no matter how long, only the character
                    // immediately after the escape can itself be a backslash or quote.
                    append_rune(buf, must_read_rune());
                } else if (r == '"') {
                    break;
                }
            }
            std::string result;
            if (!unquote(buf, result)) {
                fail("invalid syntax");
            }
            return result;
        }
        default:
            error_string("expected quoted string");
        }
    }

    // hex_byte returns the next hex-encoded (two-character) byte from the input; false if the
    // next bytes do not encode a hex byte.
    bool hex_byte(uint8_t& b) {
        int32_t rune1 = get_rune();
        if (rune1 == eof) {
            return false;
        }
        int value1 = hex_value(rune1);
        if (value1 < 0) {
            unread_rune();
            return false;
        }
        int value2 = hex_value(must_read_rune());
        if (value2 < 0) {
            error_string("illegal hex digit");
        }
        b = static_cast<uint8_t>(value1 << 4 | value2);
        return true;
    }

    std::string hex_string() {
        not_eof();
        for (;;) {
            uint8_t b;
            if (!hex_byte(b)) {
                break;
            }
            buf += static_cast<char>(b);
        }
        if (buf.empty()) {
            error_string("no hex data for %x string");
        }
        return buf;
    }

    // convert_string returns the string represented by the next input characters.
    std::string convert_string(int32_t verb) {
        if (!ok_verb(verb, "svqxX", "string")) {
            return "";
        }
        skip_space();
        not_eof();
        switch (verb) {
        case 'q':
            return quoted_string();
        case 'x':
        case 'X':
            return hex_string();
        default:
            buf.clear();
            token(true, nullptr); // %s and %v just return the next word
            return buf;
        }
    }

    // scan_percent scans a literal percent character.
    void scan_percent() {
        skip_space();
        not_eof();
        if (!accept("%")) {
            error_string("missing literal %");
        }
    }

    void scan_one(int32_t verb, const value& arg);
    void do_scan(const value* a, std::size_t n, int64_t& num_processed);
    int64_t advance(std::string_view format);
    void do_scanf(std::string_view format, const value* a, std::size_t n, int64_t& num_processed);
};

// --- the ScanState that a Scan method gets ---

// state_ss is the scanner of the ScanState self, or null when the scan that made it is over. In Go
// the scanner is then back in its pool with a nil reader, but keeps its other fields: ReadRune,
// SkipSpace and Token meet io.EOF if the scan had reached the end (or its limit) and panic with a
// nil pointer dereference otherwise; UnreadRune always panics; Width says "not set".
ss* state_ss(const iface& self) {
    return unbox<gp::ptr<ss_state>>(self)->p;
}

[[noreturn]] void dead_scan_state() {
    throw gp::panic_error(
        gp::string::gp_copy("runtime error: invalid memory address or nil pointer dereference"));
}

ss& live_ss(const iface& self) {
    ss* s = state_ss(self);
    if (s == nullptr) {
        dead_scan_state();
    }
    return *s;
}

std::tuple<int64_t, gp::error> st_read(const iface&, gp::slice<uint8_t>) {
    return {0, make_error("ScanState's Read should not be called. Use ReadRune")};
}
std::tuple<int32_t, int64_t, gp::error> st_read_rune(const iface& self) {
    if (state_ss(self) == nullptr && unbox<gp::ptr<ss_state>>(self)->dead_eof) {
        return {0, 0, io_eof()};
    }
    ss& s = live_ss(self);
    int64_t size;
    gp::error err;
    int32_t r = s.read_rune_pub(size, err);
    return {r, size, std::move(err)};
}
void st_skip_space(const iface& self) {
    if (state_ss(self) == nullptr && unbox<gp::ptr<ss_state>>(self)->dead_eof) {
        return; // getRune meets the end of the input
    }
    try {
        live_ss(self).skip_space();
    } catch (const scan_error& failure) {
        gp::panic(to_iface<gp::any>(failure, td_scan_error()));
    }
}
std::tuple<gp::slice<uint8_t>, gp::error> st_token(const iface& self, bool skip_space,
                                                   gp::func<bool(int32_t)> f) {
    if (state_ss(self) == nullptr && unbox<gp::ptr<ss_state>>(self)->dead_eof) {
        return {gp::make_slice<uint8_t>(0), gp::error()}; // nothing to read: an empty token
    }
    ss& s = live_ss(self);
    try {
        s.buf.clear();
        s.token(skip_space, f == nullptr ? nullptr : &f);
        return {gp::to_bytes(gp::string::gp_copy(s.buf)), gp::error()};
    } catch (scan_error& e) {
        return {gp::slice<uint8_t>(), std::move(e.err)};
    } catch (const gp::panic_error& panic) {
        gp::error err;
        if (!scan_error_value(panic.gp_value, err)) {
            throw;
        }
        return {gp::slice<uint8_t>(), std::move(err)};
    }
}
gp::error st_unread_rune(const iface& self) {
    live_ss(self).unread_rune();
    return gp::error();
}
std::tuple<int64_t, bool> st_width(const iface& self) {
    ss* s = state_ss(self);
    if (s == nullptr) {
        int64_t w = unbox<gp::ptr<ss_state>>(self)->dead_wid;
        return w < 0 ? std::tuple<int64_t, bool>{0, false} : std::tuple<int64_t, bool>{w, true};
    }
    if (s->max_wid == huge_wid) {
        return {0, false};
    }
    return {s->max_wid, true};
}

// *fmt.ss: the dynamic type of the ScanState that a Scan method gets.
const type_desc* td_ss_ptr() {
    static const method pmethods[] = {
        method_of<std::tuple<int64_t, gp::error>(gp::slice<uint8_t>), &st_read>(
            "Read", nullptr, "func([]uint8) (int, error)"),
        method_of<std::tuple<int32_t, int64_t, gp::error>(), &st_read_rune>(
            "ReadRune", nullptr, "func() (int32, int, error)"),
        method_of<void(), &st_skip_space>("SkipSpace", nullptr, "func()"),
        method_of<std::tuple<gp::slice<uint8_t>, gp::error>(bool, gp::func<bool(int32_t)>),
                  &st_token>("Token", nullptr, "func(bool, func(int32) bool) ([]uint8, error)"),
        method_of<gp::error(), &st_unread_rune>("UnreadRune", nullptr, "func() error"),
        method_of<std::tuple<int64_t, bool>(), &st_width>("Width", nullptr, "func() (int, bool)"),
    };
    static const type_desc td =
        named<ss_state>("fmt.ss", "fmt", shape_struct(nullptr, 0), {}, methods_of(pmethods));
    static const type_desc* const p = td_ptr<ss_state>(&td);
    return p;
}

ScanState make_scan_state(ss& s) {
    if (s.state == nullptr) {
        s.state = new gp::ptr<ss_state>(gp::new_<ss_state>(ss_state{&s}));
    }
    return to_iface<ScanState>(*s.state, td_ss_ptr());
}

// --- scanning one value ---

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

int bits_of(kind k) noexcept {
    switch (k) {
    case kind::Int8:
    case kind::Uint8:
        return 8;
    case kind::Int16:
    case kind::Uint16:
        return 16;
    case kind::Int32:
    case kind::Uint32:
    case kind::Float32:
        return 32;
    default:
        return 64;
    }
}

void store_int(kind k, void* p, int64_t v) {
    switch (k) {
    case kind::Int8:
        *static_cast<int8_t*>(p) = static_cast<int8_t>(v);
        break;
    case kind::Int16:
        *static_cast<int16_t*>(p) = static_cast<int16_t>(v);
        break;
    case kind::Int32:
        *static_cast<int32_t*>(p) = static_cast<int32_t>(v);
        break;
    default:
        *static_cast<int64_t*>(p) = v;
    }
}

void store_uint(kind k, void* p, uint64_t v) {
    switch (k) {
    case kind::Uint8:
        *static_cast<uint8_t*>(p) = static_cast<uint8_t>(v);
        break;
    case kind::Uint16:
        *static_cast<uint16_t*>(p) = static_cast<uint16_t>(v);
        break;
    case kind::Uint32:
        *static_cast<uint32_t*>(p) = static_cast<uint32_t>(v);
        break;
    default:
        *static_cast<uint64_t*>(p) = v;
    }
}

std::string_view type_string(const type_desc* t) {
    if (t->gp_kind == kind::Foreign && t->gp_ops != nullptr && t->gp_ops->gp_go_name != nullptr) {
        return t->gp_ops->gp_go_name;
    }
    return t->gp_name;
}

// scan_one scans a single value, deriving the scanner from the type of the argument.
void ss::scan_one(int32_t verb, const value& arg) {
    buf.clear();
    // If the parameter has its own Scan method, use that.
    if (arg.t != nullptr) {
        if (const method* m = find_scan_method(arg.t)) {
            iface self = self_of(arg);
            gp::error err = reinterpret_cast<thunk_t<gp::error(ScanState, int32_t)>*>(m->gp_fn())(
                self, make_scan_state(*this), verb);
            if (err != nullptr) {
                if (err == io_eof()) {
                    err = io_unexpected_eof();
                }
                fail(err);
            }
            return;
        }
    }
    if (arg.t == nullptr) {
        throw gp::panic_error(
            gp::string::gp_copy("reflect: call of reflect.Value.Type on zero Value"));
    }
    if (arg.t->gp_kind != kind::Pointer) {
        error_string("type not a pointer: " + std::string(type_string(arg.t)));
    }
    const type_desc* et = full(arg.t)->gp_elem;
    void* p = const_cast<void*>(arg.p);
    if (et == nullptr) {
        error_string("can't scan type: " + std::string(type_string(arg.t)));
    }
    kind k = et->gp_kind;
    if (p == nullptr) {
        // Only the predeclared pointer cases use direct assignment in Go. Its reflect fallback
        // sees an invalid Elem for a nil pointer and rejects the type before consuming input.
        bool direct = arg.t->gp_pkg == nullptr && et->gp_pkg == nullptr;
        if (k == kind::Slice) {
            direct = direct && full(et)->gp_elem == &td_uint8;
        }
        if (!direct) {
            error_string("can't scan type: " + std::string(type_string(arg.t)));
        }
    }
    switch (k) {
    case kind::Bool: {
        bool v = scan_bool(verb);
        if (p == nullptr) {
            nil_deref();
        }
        *static_cast<bool*>(p) = v;
        return;
    }
    case kind::Int:
    case kind::Int8:
    case kind::Int16:
    case kind::Int32:
    case kind::Int64: {
        int64_t v = scan_int(verb, bits_of(k));
        if (p == nullptr) {
            nil_deref();
        }
        store_int(k, p, v);
        return;
    }
    case kind::Uint:
    case kind::Uint8:
    case kind::Uint16:
    case kind::Uint32:
    case kind::Uint64:
    case kind::Uintptr: {
        uint64_t v = scan_uint(verb, bits_of(k));
        if (p == nullptr) {
            nil_deref();
        }
        store_uint(k, p, v);
        return;
    }
    case kind::Float32:
    case kind::Float64: {
        // Floats are tricky because you want to scan in the precision of the result, not scan in
        // high precision and convert, in order to preserve the correct error condition. The
        // predeclared types check the verb; the types made from them (Go's reflect path) don't.
        if (et->gp_pkg == nullptr) {
            ok_verb(verb, float_verbs, k == kind::Float32 ? "float32" : "float64");
        }
        skip_space();
        not_eof();
        double v = convert_float(float_token(), bits_of(k));
        if (p == nullptr) {
            nil_deref();
        }
        if (k == kind::Float32) {
            *static_cast<float*>(p) = static_cast<float>(v);
        } else {
            *static_cast<double*>(p) = v;
        }
        return;
    }
    case kind::Complex64:
    case kind::Complex128: {
        int bits = k == kind::Complex64 ? 64 : 128;
        auto [re, im] = scan_complex(verb, bits);
        if (p == nullptr) {
            nil_deref();
        }
        if (k == kind::Complex64) {
            *static_cast<gp::complex64*>(p) =
                gp::complex64(static_cast<float>(re), static_cast<float>(im));
        } else {
            *static_cast<gp::complex128*>(p) = gp::complex128(re, im);
        }
        return;
    }
    case kind::String: {
        std::string v = convert_string(verb);
        if (p == nullptr) {
            nil_deref();
        }
        *static_cast<gp::string*>(p) = gp::string::gp_copy(v);
        return;
    }
    case kind::Slice: {
        // For now, can only handle (renamed) []byte.
        const type_desc* el = full(et)->gp_elem;
        if (el == nullptr || el->gp_kind != kind::Uint8) {
            error_string("can't scan type: " + std::string(type_string(arg.t)));
        }
        std::string v = convert_string(verb);
        if (p == nullptr) {
            nil_deref();
        }
        *static_cast<gp::slice<uint8_t>*>(p) = gp::to_bytes(gp::string::gp_copy(v));
        return;
    }
    default:
        error_string("can't scan type: " + std::string(type_string(arg.t)));
    }
}

// do_scan does the real work for scanning without a format string.
void ss::do_scan(const value* a, std::size_t n, int64_t& num_processed) {
    for (std::size_t i = 0; i < n; i++) {
        // A Scanner can replace a later element of the variadic slice. Load that element only
        // when its turn arrives, and keep the current operand alive while its method runs.
        value operand = live.data == nullptr ? a[i] : from_iface(live.data[i]);
        scan_one('v', operand);
        num_processed++;
    }
    // Check for newline (or EOF) if required (Scanln etc.).
    if (nl_is_end) {
        for (;;) {
            int32_t r = get_rune();
            if (r == '\n' || r == eof) {
                break;
            }
            if (!is_space(r)) {
                error_string("expected newline");
            }
        }
    }
}

// advance determines whether the next characters in the input match those of the format. It
// returns the number of bytes consumed in the format. All runs of space characters in either
// input or format behave as a single space. Newlines are special, though: newlines in the format
// must match those in the input and vice versa. This routine also handles the %% case. If the
// return value is zero, either format starts with a % (with no following %) or the input is
// empty. If it is negative, the input did not match the string.
int64_t ss::advance(std::string_view format) {
    std::size_t i = 0;
    while (i < format.size()) {
        auto [fmtc, w] = decode_at(format, i);

        // Space processing. In the rest of this comment "space" means spaces other than newline.
        // Newline in the format matches input of zero or more spaces and then newline or
        // end-of-input. Spaces in the format before the newline are collapsed into the newline.
        // Spaces in the format after the newline match zero or more spaces after the
        // corresponding input newline. Other spaces in the format match input of one or more
        // spaces or end-of-input.
        if (is_space(fmtc)) {
            int newlines = 0;
            bool trailing_space = false;
            while (is_space(fmtc) && i < format.size()) {
                if (fmtc == '\n') {
                    newlines++;
                    trailing_space = false;
                } else {
                    trailing_space = true;
                }
                i += static_cast<std::size_t>(w);
                std::tie(fmtc, w) = decode_at(format, i);
            }
            for (int j = 0; j < newlines; j++) {
                int32_t inputc = get_rune();
                while (is_space(inputc) && inputc != '\n') {
                    inputc = get_rune();
                }
                if (inputc != '\n' && inputc != eof) {
                    error_string("newline in format does not match input");
                }
            }
            if (trailing_space) {
                int32_t inputc = get_rune();
                if (newlines == 0) {
                    // If the trailing space stood alone (did not follow a newline), it must find
                    // at least one space to consume.
                    if (!is_space(inputc) && inputc != eof) {
                        error_string("expected space in input to match format");
                    }
                    if (inputc == '\n') {
                        error_string("newline in input does not match format");
                    }
                }
                while (is_space(inputc) && inputc != '\n') {
                    inputc = get_rune();
                }
                if (inputc != eof) {
                    unread_rune();
                }
            }
            continue;
        }

        // Verbs.
        if (fmtc == '%') {
            // % at end of string is an error.
            if (i + static_cast<std::size_t>(w) == format.size()) {
                error_string("missing verb: % at end of format string");
            }
            // %% acts like a real percent
            auto [nextc, nw] = decode_at(format, i + static_cast<std::size_t>(w));
            (void)nw;
            if (nextc != '%') {
                return static_cast<int64_t>(i);
            }
            i += static_cast<std::size_t>(w); // skip the first %
        }

        // Literals.
        int32_t inputc = must_read_rune();
        if (fmtc != inputc) {
            unread_rune();
            return -1;
        }
        i += static_cast<std::size_t>(w);
    }
    return static_cast<int64_t>(i);
}

bool too_large(int64_t x) noexcept {
    return x > 1000000 || x < -1000000;
}

// parsenum converts ASCII to integer: it stops at end (exclusive).
std::tuple<int64_t, bool, std::size_t> parsenum(std::string_view s, std::size_t start,
                                                std::size_t end) {
    if (start >= end) {
        return {0, false, end};
    }
    int64_t num = 0;
    bool isnum = false;
    std::size_t newi = start;
    for (; newi < end && s[newi] >= '0' && s[newi] <= '9'; newi++) {
        if (too_large(num)) {
            return {0, false, end};
        }
        num = num * 10 + (s[newi] - '0');
        isnum = true;
    }
    return {num, isnum, newi};
}

// do_scanf does the real work when scanning with a format string.
void ss::do_scanf(std::string_view format, const value* a, std::size_t n, int64_t& num_processed) {
    // `end` is len(format) - 1; format is not empty here (an empty one loops zero times).
    const int64_t end = static_cast<int64_t>(format.size()) - 1;
    // We process one item per non-trivial format
    for (int64_t i = 0; i <= end;) {
        int64_t w = advance(format.substr(static_cast<std::size_t>(i)));
        if (w > 0) {
            i += w;
            continue;
        }
        // Either we failed to advance, we have a percent character, or we ran out of input.
        if (format[static_cast<std::size_t>(i)] != '%') {
            // Can't advance format. Why not?
            if (w < 0) {
                error_string("input does not match format");
            }
            // Otherwise at EOF; "too many operands" error handled below
            break;
        }
        i++; // % is one byte

        // do we have 20 (width)?
        auto [wid, wid_present, newi] =
            parsenum(format, static_cast<std::size_t>(i), static_cast<std::size_t>(end));
        i = static_cast<int64_t>(newi);
        max_wid = wid_present ? wid : huge_wid;

        auto [c, cw] = decode_at(format, static_cast<std::size_t>(i));
        i += cw;

        if (c != 'c') {
            skip_space();
        }
        if (c == '%') {
            scan_percent();
            continue; // Do not consume an argument.
        }
        arg_limit = limit;
        if (int64_t f = count + max_wid; f < arg_limit) {
            arg_limit = f;
        }

        if (num_processed >= static_cast<int64_t>(n)) { // out of operands
            error_string("too few operands for format '%" +
                         std::string(format.substr(static_cast<std::size_t>(i - cw))) + "'");
        }
        value operand =
            live.data == nullptr ? a[num_processed] : from_iface(live.data[num_processed]);

        scan_one(c, operand);
        num_processed++;
        arg_limit = limit;
    }
    if (num_processed < static_cast<int64_t>(n)) {
        error_string("too many operands");
    }
}

// The drivers: errorHandler turns the scanner's panics into the error result.

template <typename Body>
std::tuple<int64_t, gp::error> run_scan(rune_source& rs, bool nl_is_space, bool nl_is_end,
                                        Body&& body, live_span live = {}) {
    ss s;
    s.live = live;
    s.rs = &rs;
    s.nl_is_space = nl_is_space;
    s.nl_is_end = nl_is_end;
    int64_t num_processed = 0;
    gp::error err;
    try {
        body(s, num_processed);
    } catch (scan_error& e) {
        err = std::move(e.err);
    } catch (scan_eof&) {
        err = io_eof();
    } catch (const gp::panic_error& panic) {
        if (scan_error_value(panic.gp_value, err)) {
            // A ScanState error returned through the custom Scanner's deferred calls.
        } else if (panic.gp_value == io_eof()) {
            err = io_eof(); // exact identity, never another error that merely says "EOF"
        } else {
            throw;
        }
    }
    return {num_processed, std::move(err)};
}

// scan_from runs the scan over a byte source, or over the reader itself when it is a RuneScanner.
template <typename Body>
std::tuple<int64_t, gp::error> scan_bytes(byte_source& src, bool nl_is_space, bool nl_is_end,
                                          Body&& body, live_span live = {}) {
    rune_reader rr(src);
    return run_scan(rr, nl_is_space, nl_is_end, body, live);
}

} // namespace

std::tuple<int64_t, gp::error> scan_string(std::string_view str, bool nl_is_space, bool nl_is_end,
                                           const arg* a, std::size_t n, live_span live) {
    string_bytes src(str);
    return scan_bytes(
        src, nl_is_space, nl_is_end, [&](ss& s, int64_t& np) { s.do_scan(a, n, np); }, live);
}

std::tuple<int64_t, gp::error> scanf_string(std::string_view str, std::string_view format,
                                            const arg* a, std::size_t n, live_span live) {
    string_bytes src(str);
    return scan_bytes(
        src, false, false, [&](ss& s, int64_t& np) { s.do_scanf(format, a, n, np); }, live);
}

std::tuple<int64_t, gp::error> scan_reader(const iface& r, bool nl_is_space, bool nl_is_end,
                                           const arg* a, std::size_t n, live_span live) {
    if (r.gp_type() != nullptr) {
        const method* rr = find_method(r.gp_type(), "ReadRune", read_rune_sig);
        const method* ur = find_method(r.gp_type(), "UnreadRune", unread_rune_sig);
        if (rr != nullptr && ur != nullptr) {
            dyn_runes runes(r, rr, ur);
            return run_scan(
                runes, nl_is_space, nl_is_end, [&](ss& s, int64_t& np) { s.do_scan(a, n, np); },
                live);
        }
    }
    reader_bytes src(r);
    return scan_bytes(
        src, nl_is_space, nl_is_end, [&](ss& s, int64_t& np) { s.do_scan(a, n, np); }, live);
}

std::tuple<int64_t, gp::error> scanf_reader(const iface& r, std::string_view format, const arg* a,
                                            std::size_t n, live_span live) {
    if (r.gp_type() != nullptr) {
        const method* rr = find_method(r.gp_type(), "ReadRune", read_rune_sig);
        const method* ur = find_method(r.gp_type(), "UnreadRune", unread_rune_sig);
        if (rr != nullptr && ur != nullptr) {
            dyn_runes runes(r, rr, ur);
            return run_scan(
                runes, false, false, [&](ss& s, int64_t& np) { s.do_scanf(format, a, n, np); },
                live);
        }
    }
    reader_bytes src(r);
    return scan_bytes(
        src, false, false, [&](ss& s, int64_t& np) { s.do_scanf(format, a, n, np); }, live);
}

namespace {
// What package os registered for os.Stdin (register_os_stdin): never freed, and read without a
// lock.
struct stdin_reg {
    os_std d;
    gp::func<gp::any(std::shared_ptr<void>)> current;
};
std::atomic<const stdin_reg*> stdin_registered{nullptr};

// gate_guard counts a direct read of descriptor 0 (std_gate) until it ends.
struct gate_guard {
    bool held;
    gate_guard() : held(true) {}
    void release() {
        if (held) {
            held = false;
            stdin_gate.word.fetch_sub(2, std::memory_order_seq_cst);
            finish_close(false); // the last reader out closes a descriptor whose Close waited
        }
    }
    ~gate_guard() { release(); }
};
} // namespace

void register_os_stdin(const os_std& d, gp::func<gp::any(std::shared_ptr<void>)> current) {
    stdin_registered.store(new stdin_reg{d, std::move(current)}, std::memory_order_release);
}

// with_stdin runs scan_fd0() when the scan reads descriptor 0 itself, and scan_reader(r) with the
// file os.Stdin holds when the program changed it or closed the original. The direct scans are
// counted (std_gate), so that Close of the original waits for them.
template <typename Fd0, typename Rd>
std::tuple<int64_t, gp::error> with_stdin(Fd0&& scan_fd0, Rd&& scan_reader_of) {
    gate_guard g;
    if (stdin_gate.word.fetch_add(2, std::memory_order_seq_cst) & 1) [[unlikely]] {
        const stdin_reg* r = stdin_registered.load(std::memory_order_acquire);
        if (r != nullptr) {
            std::shared_ptr<void> snap = r->d.load(r->d.var);
            if (r->d.replaced(snap)) {
                g.release();
                gp::any rd = r->current(snap);
                return scan_reader_of(rd);
            }
        }
    }
    return scan_fd0();
}

std::tuple<int64_t, gp::error> scan_stdin(bool nl_is_space, bool nl_is_end, const arg* a,
                                          std::size_t n, live_span live) {
    return with_stdin(
        [&] {
            stdin_bytes src;
            return scan_bytes(
                src, nl_is_space, nl_is_end, [&](ss& s, int64_t& np) { s.do_scan(a, n, np); },
                live);
        },
        [&](const gp::any& r) { return scan_reader(r, nl_is_space, nl_is_end, a, n, live); });
}

std::tuple<int64_t, gp::error> scanf_stdin(std::string_view format, const arg* a, std::size_t n,
                                           live_span live) {
    return with_stdin(
        [&] {
            stdin_bytes src;
            return scan_bytes(
                src, false, false, [&](ss& s, int64_t& np) { s.do_scanf(format, a, n, np); }, live);
        },
        [&](const gp::any& r) { return scanf_reader(r, format, a, n, live); });
}

} // namespace detail

} // namespace gp::fmt
