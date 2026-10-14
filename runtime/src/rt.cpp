// The parts of the runtime that are not templates: strings, panics, the print and fmt
// formats, and run_main. Standard C++ only: the OS is reached through gocpp/os.hpp.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <charconv>
#include <cmath>
#include <cstdio>
#include <cstring>
#include <memory>
#include <new>
#include <string>
#include <string_view>
#include <system_error>
#include <utility>
#include <vector>

namespace gp {

// --- strings ---

string string::gp_copy(std::string_view v) {
    if (v.empty()) {
        return string();
    }
    auto bytes = std::make_shared_for_overwrite<char[]>(v.size());
    std::memcpy(bytes.get(), v.data(), v.size());
    const char* data = bytes.get();
    return string(std::move(bytes), data, static_cast<int64_t>(v.size()));
}

string operator+(const string& a, const string& b) {
    if (a.gp_len_ == 0) {
        return b;
    }
    if (b.gp_len_ == 0) {
        return a;
    }
    auto n = static_cast<std::size_t>(a.gp_len_) + static_cast<std::size_t>(b.gp_len_);
    auto bytes = std::make_shared_for_overwrite<char[]>(n);
    std::memcpy(bytes.get(), a.gp_data_, static_cast<std::size_t>(a.gp_len_));
    std::memcpy(bytes.get() + a.gp_len_, b.gp_data_, static_cast<std::size_t>(b.gp_len_));
    const char* data = bytes.get();
    return string(std::move(bytes), data, static_cast<int64_t>(n));
}

// --- release (ptr.hpp) ---

namespace detail {

void release_defer(std::shared_ptr<const void>&& owner) noexcept {
    // One node per release_max_depth nodes of a chain, so the allocation is rare. Without
    // memory, drop the owner here, recursing deeper: better than losing it.
    release_state& st = release_tls;
    auto* n = new (std::nothrow) release_node{std::move(owner), st.gp_pending};
    if (n == nullptr) {
        ++st.gp_depth;
        owner.reset();
        --st.gp_depth;
        return;
    }
    st.gp_pending = n;
}

void release_drain() noexcept {
    // Called by the outermost release (depth 0). The depth is 1 while each pending owner is
    // dropped, so the releases inside don't drain again, and they defer once they are
    // release_max_depth deep, which adds to the list this loop empties.
    release_state& st = release_tls;
    st.gp_depth = 1;
    while (release_node* n = st.gp_pending) {
        st.gp_pending = n->gp_next;
        std::shared_ptr<const void> owner = std::move(n->gp_owner);
        delete n;
        owner.reset();
    }
    st.gp_depth = 0;
}

void bad_owner() noexcept {
    write_stderr("fatal error: gocpp: a non-nil pointer without an owner (a bug in the runtime "
                 "or in the generated code)\n");
    os::exit(2);
}

} // namespace detail

// --- panics ---

void panic(const gp::string& v) {
    throw panic_error{v};
}

void panic_divide() {
    throw panic_error{"runtime error: integer divide by zero"_gs};
}

void panic_shift() {
    throw panic_error{"runtime error: negative shift amount"_gs};
}

namespace {

// The goroutine's signal (panic.hpp): set by panic_nil, with the pc of its caller.
constinit thread_local bool signal_set = false;
constinit thread_local uintptr_t signal_pc = 0;

} // namespace

void panic_nil() {
    signal_set = true;
    signal_pc =
        reinterpret_cast<uintptr_t>(__builtin_extract_return_addr(__builtin_return_address(0)));
    throw panic_error{"runtime error: invalid memory address or nil pointer dereference"_gs};
}

namespace detail {

void clear_signal() noexcept {
    signal_set = false;
    signal_pc = 0;
}

} // namespace detail

void panic_index(int64_t x, bool is_signed, int64_t len) {
    // Go's boundsError: "index out of range [%x] with length %y", or only "[%x]" for a
    // negative signed index.
    std::string text = "runtime error: index out of range [";
    if (is_signed) {
        detail::print_int(text, x);
    } else {
        detail::print_uint(text, static_cast<uint64_t>(x));
    }
    text += ']';
    if (!is_signed || x >= 0) {
        text += " with length ";
        detail::print_int(text, len);
    }
    throw panic_error{gp::string::gp_copy(text)};
}

namespace detail {

void panic_report(std::string& out, const gp::string& text) {
    out += "panic: ";
    // Go's printindented: a tab after each newline of the value.
    for (char c : text.gp_view()) {
        out += c;
        if (c == '\n') {
            out += '\t';
        }
    }
    if (signal_set) {
        // Go's line for the SIGSEGV of the nil access. Its addr is the faulting address: 0x0
        // for a field at offset 0, for a whole value and for a field at 4096 or more (Go
        // checks those itself), and the field's offset for a field at a smaller offset, read
        // or written. gocpp checks the pointer before any access, so it always prints 0x0.
        // The pc is the code that called panic_nil.
#ifdef _WIN32
        // Go on Windows (Phase 14 checks it, and may move this to src/os/): the exception
        // code, and code= is 0x0 for a read and 0x1 for a write, which gocpp can't tell.
        out += "\n[signal 0xc0000005 code=0x0 addr=0x0 pc=";
#else
        out += "\n[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=";
#endif
        print_hex(out, static_cast<uint64_t>(signal_pc));
        out += ']';
    }
    out += "\n\ngoroutine 1 [running]:\n";
}

// --- the runtime's print format (Go's runtime/print.go) ---

void print_uint(std::string& out, uint64_t v) {
    char buf[20];
    char* end = buf + sizeof buf;
    char* p = end;
    do {
        *--p = static_cast<char>('0' + v % 10);
        v /= 10;
    } while (v != 0);
    out.append(p, end);
}

void print_int(std::string& out, int64_t v) {
    if (v < 0) {
        out += '-';
        // -v, without overflow for MIN.
        print_uint(out, 0u - static_cast<uint64_t>(v));
        return;
    }
    print_uint(out, static_cast<uint64_t>(v));
}

void print_hex(std::string& out, uint64_t v) {
    static const char digits[] = "0123456789abcdef";
    char buf[16];
    char* end = buf + sizeof buf;
    char* p = end;
    do {
        *--p = digits[v % 16];
        v /= 16;
    } while (v != 0);
    out += "0x";
    out.append(p, end);
}

// print_float is Go's printfloat, step by step, so that its rounding (it is not correctly
// rounded) gives the same digits: 7 significant digits, a sign, and a 3-digit exponent.
void print_float(std::string& out, double v) {
    if (v != v) {
        out += "NaN";
        return;
    }
    if (v + v == v && v > 0) {
        out += "+Inf";
        return;
    }
    if (v + v == v && v < 0) {
        out += "-Inf";
        return;
    }

    constexpr int n = 7; // digits printed
    char buf[n + 7];
    buf[0] = '+';
    int e = 0; // the exponent
    if (v == 0) {
        if (std::signbit(v)) {
            buf[0] = '-';
        }
    } else {
        if (v < 0) {
            v = -v;
            buf[0] = '-';
        }
        // normalize
        while (v >= 10) {
            e++;
            v /= 10;
        }
        while (v < 1) {
            e--;
            v *= 10;
        }
        // round
        double h = 5.0;
        for (int i = 0; i < n; i++) {
            h /= 10;
        }
        v += h;
        if (v >= 10) {
            e++;
            v /= 10;
        }
    }

    // format +d.dddddde+ddd
    for (int i = 0; i < n; i++) {
        int s = static_cast<int>(v);
        buf[i + 2] = static_cast<char>(s + '0');
        v -= static_cast<double>(s);
        v *= 10;
    }
    buf[1] = buf[2];
    buf[2] = '.';

    buf[n + 2] = 'e';
    buf[n + 3] = '+';
    if (e < 0) {
        e = -e;
        buf[n + 3] = '-';
    }
    buf[n + 4] = static_cast<char>(e / 100 + '0');
    buf[n + 5] = static_cast<char>(e / 10 % 10 + '0');
    buf[n + 6] = static_cast<char>(e % 10 + '0');
    out.append(buf, sizeof buf);
}

void write_stderr(std::string_view s) {
    // Go's runtime print ignores write errors, also a broken pipe.
    if (!s.empty()) {
        os::write(os::stderr_fd, s.data(), s.size());
    }
}

} // namespace detail

// --- fmt ---

namespace fmtshim::detail {

void write_stdout(std::string_view s) {
    // fmt writes through os.Stdout, which ends the program with SIGPIPE on a broken pipe.
    if (!s.empty() &&
        os::write(os::stdout_fd, s.data(), s.size()) == os::write_result::broken_pipe) {
        os::die_of_sigpipe();
    }
}

// float_v is strconv's 'g' format with the shortest digits, as fmt's %v uses it.
void float_v(std::string& out, double v, int bits) {
    if (std::isnan(v)) {
        out += "NaN";
        return;
    }
    if (std::isinf(v)) {
        out += v > 0 ? "+Inf" : "-Inf";
        return;
    }
    // The shortest digits that read back as the same value, in the form [-]d[.ddd]e±dd.
    char buf[64];
    std::to_chars_result r =
        bits == 32 ? std::to_chars(buf, buf + sizeof buf, static_cast<float>(v),
                                   std::chars_format::scientific)
                   : std::to_chars(buf, buf + sizeof buf, v, std::chars_format::scientific);
    std::string_view s(buf, static_cast<std::size_t>(r.ptr - buf));
    if (s.front() == '-') {
        out += '-';
        s.remove_prefix(1);
    }
    std::size_t epos = s.find('e');
    std::string digits;
    for (char c : s.substr(0, epos)) {
        if (c != '.') {
            digits += c;
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
    int nd = static_cast<int>(digits.size());

    if (exp < -4 || exp >= 6) {
        // %e: d.ddde±dd, with at least two exponent digits.
        out += digits[0];
        if (nd > 1) {
            out += '.';
            out.append(digits, 1);
        }
        out += 'e';
        int x = exp;
        out += x < 0 ? '-' : '+';
        if (x < 0) {
            x = -x;
        }
        if (x < 10) {
            out += '0';
        }
        out += std::to_string(x);
        return;
    }
    // %f with as many digits as needed.
    int dp = exp + 1; // digits before the point
    if (dp <= 0) {
        out += "0.";
        out.append(static_cast<std::size_t>(-dp), '0');
        out += digits;
    } else if (dp >= nd) {
        out += digits;
        out.append(static_cast<std::size_t>(dp - nd), '0');
    } else {
        out.append(digits, 0, static_cast<std::size_t>(dp));
        out += '.';
        out.append(digits, static_cast<std::size_t>(dp));
    }
}

void float_f(std::string& out, double v) {
    if (std::isnan(v)) {
        out += "NaN";
        return;
    }
    if (std::isinf(v)) {
        out += v > 0 ? "+Inf" : "-Inf";
        return;
    }
    // The largest double has 309 digits before the point.
    char buf[400];
    std::to_chars_result r = std::to_chars(buf, buf + sizeof buf, v, std::chars_format::fixed, 6);
    out.append(buf, r.ptr);
}

namespace {

// value_v appends a as %v prints it.
void value_v(std::string& out, const arg& a) {
    switch (a.kind) {
    case arg::bool_:
        out += a.b ? "true" : "false";
        break;
    case arg::int_:
        gp::detail::print_int(out, a.i);
        break;
    case arg::uint_:
        gp::detail::print_uint(out, a.u);
        break;
    case arg::float32_:
        float_v(out, a.f, 32);
        break;
    case arg::float64_:
        float_v(out, a.f, 64);
        break;
    case arg::string_:
        out += a.s;
        break;
    }
}

// format_verb appends a as the verb prints it, or Go's %!verb(type=value) when the verb doesn't
// fit the type. verb is the verb's bytes (one UTF-8 character).
void format_verb(std::string& out, const arg& a, char32_t verb, std::string_view verb_bytes) {
    bool ok = false;
    switch (a.kind) {
    case arg::bool_:
        ok = verb == 'v' || verb == 't';
        break;
    case arg::int_:
    case arg::uint_:
        ok = verb == 'v' || verb == 'd';
        break;
    case arg::float32_:
    case arg::float64_:
        if (verb == 'f') {
            float_f(out, a.f);
            return;
        }
        ok = verb == 'v';
        break;
    case arg::string_:
        ok = verb == 'v' || verb == 's';
        break;
    }
    if (ok) {
        value_v(out, a);
        return;
    }
    out += "%!";
    out += verb_bytes;
    out += '(';
    out += a.type;
    out += '=';
    value_v(out, a);
    out += ')';
}

// decode_rune is utf8.DecodeRuneInString: the first character of s and its size in bytes,
// or U+FFFD and 1 for a byte that doesn't start a valid UTF-8 sequence.
std::pair<char32_t, std::size_t> decode_rune(std::string_view s) {
    auto b = [&](std::size_t i) { return static_cast<unsigned char>(s[i]); };
    unsigned char c = b(0);
    if (c < 0x80) {
        return {c, 1};
    }
    auto cont = [&](std::size_t i, unsigned char lo, unsigned char hi) {
        return i < s.size() && b(i) >= lo && b(i) <= hi;
    };
    constexpr std::pair<char32_t, std::size_t> bad{0xFFFD, 1};
    if (c >= 0xC2 && c <= 0xDF) {
        if (!cont(1, 0x80, 0xBF)) {
            return bad;
        }
        return {char32_t(c & 0x1F) << 6 | (b(1) & 0x3F), 2};
    }
    if (c >= 0xE0 && c <= 0xEF) {
        unsigned char lo = c == 0xE0 ? 0xA0 : 0x80, hi = c == 0xED ? 0x9F : 0xBF;
        if (!cont(1, lo, hi) || !cont(2, 0x80, 0xBF)) {
            return bad;
        }
        return {char32_t(c & 0x0F) << 12 | char32_t(b(1) & 0x3F) << 6 | (b(2) & 0x3F), 3};
    }
    if (c >= 0xF0 && c <= 0xF4) {
        unsigned char lo = c == 0xF0 ? 0x90 : 0x80, hi = c == 0xF4 ? 0x8F : 0xBF;
        if (!cont(1, lo, hi) || !cont(2, 0x80, 0xBF) || !cont(3, 0x80, 0xBF)) {
            return bad;
        }
        return {char32_t(c & 0x07) << 18 | char32_t(b(1) & 0x3F) << 12 |
                    char32_t(b(2) & 0x3F) << 6 | (b(3) & 0x3F),
                4};
    }
    return bad;
}

} // namespace

void sprint(std::string& out, const arg* args, std::size_t n) {
    bool prev_string = false;
    for (std::size_t i = 0; i < n; i++) {
        bool is_string = args[i].kind == arg::string_;
        // Go adds a space between two arguments when neither is a string.
        if (i > 0 && !is_string && !prev_string) {
            out += ' ';
        }
        value_v(out, args[i]);
        prev_string = is_string;
    }
}

void sprintln(std::string& out, const arg* args, std::size_t n) {
    for (std::size_t i = 0; i < n; i++) {
        if (i > 0) {
            out += ' ';
        }
        value_v(out, args[i]);
    }
    out += '\n';
}

void sprintf(std::string& out, std::string_view format, const arg* args, std::size_t n) {
    std::size_t next = 0; // the next argument
    std::size_t i = 0;
    while (i < format.size()) {
        std::size_t pct = format.find('%', i);
        if (pct == std::string_view::npos) {
            out += format.substr(i);
            break;
        }
        out += format.substr(i, pct - i);
        i = pct + 1;
        if (i >= format.size()) {
            out += "%!(NOVERB)";
            break;
        }
        auto [verb, size] = decode_rune(format.substr(i));
        std::string_view verb_bytes =
            verb == 0xFFFD && size == 1 ? "\xEF\xBF\xBD" : format.substr(i, size);
        i += size;
        if (verb == '%') {
            out += '%';
        } else if (next >= n) {
            out += "%!";
            out += verb_bytes;
            out += "(MISSING)";
        } else {
            format_verb(out, args[next++], verb, verb_bytes);
        }
    }
    if (next < n) {
        out += "%!(EXTRA ";
        for (std::size_t k = next; k < n; k++) {
            if (k > next) {
                out += ", ";
            }
            out += args[k].type;
            out += '=';
            value_v(out, args[k]);
        }
        out += ')';
    }
}

} // namespace fmtshim::detail

// --- run_main ---

namespace {

std::vector<gp::string>& args_storage() {
    static std::vector<gp::string> v;
    return v;
}

std::vector<gp::string>& envs_storage() {
    static std::vector<gp::string> v;
    return v;
}

std::vector<gp::string> to_strings(const std::vector<std::string>& v) {
    std::vector<gp::string> out;
    out.reserve(v.size());
    for (const std::string& s : v) {
        out.push_back(gp::string::gp_copy(s));
    }
    return out;
}

} // namespace

const std::vector<gp::string>& args() noexcept {
    return args_storage();
}

const std::vector<gp::string>& envs() noexcept {
    return envs_storage();
}

int run_main(int argc, char** argv, void (*init)(), void (*main)()) {
    os::start();
    args_storage() = to_strings(os::arguments(argc, argv));
    envs_storage() = to_strings(os::environment());
    try {
        if (init != nullptr) {
            init();
        }
        main();
    } catch (const panic_error& e) {
        // gocpp's own output is never buffered, but C or C++ code that the program calls may
        // have printf'd: that comes first, as it happened before the panic.
        std::fflush(nullptr);
        std::string out;
        detail::panic_report(out, e.gp_text);
        detail::write_stderr(out);
        os::exit(2);
    }
    os::exit(0);
}

} // namespace gp
