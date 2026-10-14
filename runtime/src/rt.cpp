// The parts of the runtime that are not templates: strings, panics, the print format, and
// run_main (fmt's formats are in fmt.cpp). Standard C++ only: the OS is reached through
// gocpp/os.hpp.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <array>
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

// --- strings: concatenation, UTF-8, string(r) (step 3.4) ---

namespace detail {

string concat(const string* const* parts, std::size_t n) {
    std::size_t total = 0;
    const string* only = nullptr; // the one part that is not empty, if there is one
    std::size_t nonempty = 0;
    for (std::size_t i = 0; i < n; i++) {
        auto l = static_cast<std::size_t>(len(*parts[i]));
        if (l != 0) {
            total += l;
            only = parts[i];
            nonempty++;
        }
    }
    if (nonempty == 0) {
        return string();
    }
    if (nonempty == 1) {
        return *only;
    }
    auto bytes = std::make_shared_for_overwrite<char[]>(total);
    char* p = bytes.get();
    for (std::size_t i = 0; i < n; i++) {
        std::string_view v = parts[i]->gp_view();
        if (!v.empty()) {
            std::memcpy(p, v.data(), v.size());
            p += v.size();
        }
    }
    const char* data = bytes.get();
    return string(std::move(bytes), data, static_cast<int64_t>(total));
}

int32_t decode_rune(const char* p, int64_t n, int64_t& width) noexcept {
    // Go's utf8.DecodeRuneInString, with its accept ranges for the second byte: E0 needs A0..BF
    // (no overlong 3-byte form), ED 80..9F (no surrogates), F0 90..BF (no overlong 4-byte
    // form), F4 80..8F (nothing above U+10FFFF); the other continuation bytes are 80..BF.
    constexpr int32_t error = 0xFFFD;
    auto b = [p](int k) { return static_cast<uint8_t>(p[k]); };
    auto cont = [](uint8_t c) { return c >= 0x80 && c <= 0xBF; };
    width = 1;
    uint8_t c0 = b(0);
    if (c0 < 0x80) {
        return c0;
    }
    if (c0 < 0xC2 || c0 > 0xF4) {
        return error; // a continuation byte, an overlong 2-byte form (C0, C1), or F5..FF
    }
    int64_t size = c0 < 0xE0 ? 2 : c0 < 0xF0 ? 3 : 4;
    if (n < size) {
        return error;
    }
    uint8_t lo = 0x80, hi = 0xBF;
    switch (c0) {
    case 0xE0:
        lo = 0xA0;
        break;
    case 0xED:
        hi = 0x9F;
        break;
    case 0xF0:
        lo = 0x90;
        break;
    case 0xF4:
        hi = 0x8F;
        break;
    }
    uint8_t c1 = b(1);
    if (c1 < lo || c1 > hi) {
        return error;
    }
    if (size == 2) {
        width = 2;
        return static_cast<int32_t>(c0 & 0x1F) << 6 | (c1 & 0x3F);
    }
    uint8_t c2 = b(2);
    if (!cont(c2)) {
        return error;
    }
    if (size == 3) {
        width = 3;
        return static_cast<int32_t>(c0 & 0x0F) << 12 | (c1 & 0x3F) << 6 | (c2 & 0x3F);
    }
    uint8_t c3 = b(3);
    if (!cont(c3)) {
        return error;
    }
    width = 4;
    return static_cast<int32_t>(c0 & 0x07) << 18 | (c1 & 0x3F) << 12 | (c2 & 0x3F) << 6 |
           (c3 & 0x3F);
}

namespace {

// The strings of one ASCII byte each, in static storage: string(r) for r < 0x80 points here
// and doesn't allocate.
constexpr auto ascii_bytes = [] {
    std::array<char, 128> a{};
    for (int i = 0; i < 128; i++) {
        a[static_cast<std::size_t>(i)] = static_cast<char>(i);
    }
    return a;
}();

} // namespace

string rune_string(int64_t r) {
    // Go's intstring: a value that is not a valid code point is U+FFFD (EF BF BD).
    if (r >= 0 && r < 0x80) {
        return string(static_bytes, &ascii_bytes[static_cast<std::size_t>(r)], 1);
    }
    if (r < 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF)) {
        r = 0xFFFD;
    }
    auto u = static_cast<uint32_t>(r);
    char buf[4];
    std::size_t n;
    if (u < 0x800) {
        buf[0] = static_cast<char>(0xC0 | u >> 6);
        n = 2;
    } else if (u < 0x10000) {
        buf[0] = static_cast<char>(0xE0 | u >> 12);
        buf[1] = static_cast<char>(0x80 | (u >> 6 & 0x3F));
        n = 3;
    } else {
        buf[0] = static_cast<char>(0xF0 | u >> 18);
        buf[1] = static_cast<char>(0x80 | (u >> 12 & 0x3F));
        buf[2] = static_cast<char>(0x80 | (u >> 6 & 0x3F));
        n = 4;
    }
    buf[n - 1] = static_cast<char>(0x80 | (u & 0x3F));
    return string::gp_copy(std::string_view(buf, n));
}

} // namespace detail

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

// The identity of the runtime errors that Go keeps in one global or a constant
// (panic_identity): the same for the same message.
constexpr panic_identity runtime_id{panic_identity::gp_runtime, 0, 0};

namespace detail {

void throw_panic(gp::string text, panic_identity id) {
    throw panic_error{std::move(text), id};
}

} // namespace detail

void panic(const gp::string& v) {
    // Go's convTstring: the empty string is runtime.zeroVal, any other one a new allocation.
    panic_identity id;
    if (v.gp_view().empty()) {
        id = {panic_identity::gp_value, 17, 0};
    }
    throw panic_error{v, id};
}

void panic_constant(const gp::string& c) {
    throw panic_error{c, {panic_identity::gp_constant, 17, 0}};
}

void panic_divide() {
    throw panic_error{"runtime error: integer divide by zero"_gs, runtime_id};
}

void panic_shift() {
    throw panic_error{"runtime error: negative shift amount"_gs, runtime_id};
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
    throw panic_error{"runtime error: invalid memory address or nil pointer dereference"_gs,
                      runtime_id};
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
    panic_error e{text};
    const panic_error* chain[] = {&e};
    panic_report(out, chain, 1);
}

namespace {

// same reports whether Go's interface words of the two panic values are the same
// (panic_identity).
bool same(const panic_error& a, const panic_error& b) {
    return a.gp_id.gp_kind != panic_identity::gp_unique && a.gp_id.gp_kind == b.gp_id.gp_kind &&
           a.gp_id.gp_type == b.gp_id.gp_type && a.gp_id.gp_bits == b.gp_id.gp_bits &&
           a.gp_text.gp_view() == b.gp_text.gp_view();
}

} // namespace

void panic_report(std::string& out, const panic_error* const* chain, std::size_t n) {
    // Go's preprintpanics and printpanics: the first panic first, each later one after a tab;
    // a panic with the same value as the next one is printed once, as "repanicked", and the
    // next one is left out.
    for (std::size_t k = 0; k < n; k++) {
        if (k > 0 && same(*chain[k - 1], *chain[k])) {
            continue;
        }
        if (k > 0) {
            out += "\n\t";
        }
        out += "panic: ";
        // Go's printindented: a tab after each newline of the value.
        for (char c : chain[k]->gp_text.gp_view()) {
            out += c;
            if (c == '\n') {
                out += '\t';
            }
        }
        if (k + 1 < n && same(*chain[k], *chain[k + 1])) {
            out += " [recovered, repanicked]";
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
        detail::panic_report(out, e); // with the stack trace
        detail::write_stderr(out);
        os::exit(2);
    }
    os::exit(0);
}

} // namespace gp
