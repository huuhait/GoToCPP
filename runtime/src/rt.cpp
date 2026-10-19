// The parts of the runtime that are not templates: strings, panics, the print format, and
// run_main (fmt's formats are in fmt.cpp). Standard C++ only: the OS is reached through
// gocpp/os.hpp.
#include <gocpp/rt.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>
#include <gocpp/race.hpp>
#include <gocpp/trace.hpp>

#include <algorithm>
#include <array>
#include <charconv>
#include <chrono>
#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <exception>
#include <memory>
#include <new>
#include <string>
#include <string_view>
#include <system_error>
#include <thread>
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
// and doesn't allocate. Each byte is followed by a NUL, as static bytes must be followed by a
// readable byte (string.hpp), and then the string is a C string too (gp::cstr doesn't copy).
constexpr auto ascii_bytes = [] {
    std::array<char, 256> a{};
    for (int i = 0; i < 128; i++) {
        a[static_cast<std::size_t>(2 * i)] = static_cast<char>(i);
    }
    return a;
}();

} // namespace

string rune_string(int64_t r) {
    // Go's intstring: a value that is not a valid code point is U+FFFD (EF BF BD).
    if (r >= 0 && r < 0x80) {
        return string(static_bytes, &ascii_bytes[static_cast<std::size_t>(2 * r)], 1);
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
    // memory, drop the owner here, recursing deeper: better than losing it. (Under run_main's
    // new handler a failed allocation is a fatal error first: this is for C++ code without it.)
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

namespace {

// value_of is bits_of the other way: the value of type T from raw_bits' lo and hi.
template <typename T>
T value_of(uint64_t lo, uint64_t hi) {
    if constexpr (std::is_same_v<T, complex128>) {
        double re, im;
        std::memcpy(&re, &lo, 8);
        std::memcpy(&im, &hi, 8);
        return complex128(re, im);
    } else if constexpr (std::is_same_v<T, complex64>) {
        float re, im;
        std::memcpy(&re, &lo, 4);
        std::memcpy(&im, reinterpret_cast<const char*>(&lo) + 4, 4);
        return complex64(re, im);
    } else if constexpr (sizeof(T) == 8) {
        T v;
        std::memcpy(&v, &lo, 8);
        return v;
    } else if constexpr (sizeof(T) == 4) {
        auto b = static_cast<uint32_t>(lo);
        T v;
        std::memcpy(&v, &b, 4);
        return v;
    } else if constexpr (sizeof(T) == 2) {
        auto b = static_cast<uint16_t>(lo);
        T v;
        std::memcpy(&v, &b, 2);
        return v;
    } else {
        auto b = static_cast<uint8_t>(lo);
        T v;
        std::memcpy(&v, &b, 1);
        return v;
    }
}

template <typename T>
[[noreturn]] void throw_value(uint8_t kind, uint64_t lo, uint64_t hi, panic_identity id) {
    T v = value_of<T>(lo, hi);
    throw panic_error{basic_t{}, td_of_basic(kind), v, id};
}

} // namespace

__attribute__((noinline)) void throw_basic(uint8_t cxx, uint8_t kind, uint64_t lo, uint64_t hi,
                                           panic_identity id) {
    switch (cxx) {
    case 1:
        throw_value<bool>(kind, lo, hi, id);
    case 3:
        throw_value<int8_t>(kind, lo, hi, id);
    case 4:
        throw_value<int16_t>(kind, lo, hi, id);
    case 5:
        throw_value<int32_t>(kind, lo, hi, id);
    case 2:
        throw_value<int64_t>(kind, lo, hi, id);
    case 8:
        throw_value<uint8_t>(kind, lo, hi, id);
    case 9:
        throw_value<uint16_t>(kind, lo, hi, id);
    case 10:
        throw_value<uint32_t>(kind, lo, hi, id);
    case 7:
        throw_value<uint64_t>(kind, lo, hi, id);
    case 13:
        throw_value<float>(kind, lo, hi, id);
    case 15:
        throw_value<complex64>(kind, lo, hi, id);
    case 16:
        throw_value<complex128>(kind, lo, hi, id);
    default:
        throw_value<double>(kind, lo, hi, id);
    }
}

namespace {

// throw_lit throws the runtime error with a text that is a string literal (static storage; see
// panic_error's constructor for it).
template <std::size_t N>
[[noreturn]] void throw_lit(rt_error kind, const char (&lit)[N], panic_identity id) {
    throw panic_error{runtime_t{}, kind, lit, N - 1, id};
}

} // namespace

void throw_runtime(rt_error kind, gp::string text, panic_identity id) {
    gp::any value = runtime_error_value(kind, text);
    throw panic_error{std::move(value), std::move(text), false, id};
}

} // namespace detail

__attribute__((noinline)) void panic(const gp::string& v) {
    // Go's convTstring: the empty string is runtime.zeroVal, any other one a new allocation.
    panic_identity id;
    if (v.gp_view().empty()) {
        id = {panic_identity::gp_value, 17, 0};
    }
    throw panic_error{detail::string_t{}, v, id};
}

__attribute__((noinline)) void panic_constant(const gp::string& c) {
    throw panic_error{detail::string_t{}, c, {panic_identity::gp_constant, 17, 0}};
}

std::unique_ptr<panic_error> new_panic(const gp::string& v) {
    panic_identity id;
    if (v.gp_view().empty()) {
        id = {panic_identity::gp_value, 17, 0};
    }
    return std::make_unique<panic_error>(detail::string_t{}, v, id, detail::stack_t::none);
}

std::unique_ptr<panic_error> new_panic_constant(const gp::string& c) {
    return std::make_unique<panic_error>(detail::string_t{}, c,
                                         panic_identity{panic_identity::gp_constant, 17, 0},
                                         detail::stack_t::none);
}

void panic_wrap(const char* msg) {
    // Go's runtime.panicwrap: panic(plainError("value method ... called using nil *T pointer")).
    // Its identity is a string constant's (checks.hpp).
    detail::throw_runtime(detail::rt_error::plain, gp::string::gp_copy(msg),
                          {panic_identity::gp_constant, 17, 0});
}

void panic_divide() {
    detail::throw_lit(detail::rt_error::error_string, "runtime error: integer divide by zero",
                      runtime_id);
}

void panic_range_state(int state) {
    // Go's runtime.panicrangestate: a runtime.Error (errorString) for each state.
    const char* text =
        "range function continued iteration after function for loop body returned false";
    switch (state) {
    case rangefunc_state::gp_panic:
        text = "range function continued iteration after loop body panic";
        break;
    case rangefunc_state::gp_exhausted:
        text = "range function continued iteration after whole loop exit";
        break;
    case rangefunc_state::gp_missing_panic:
        text = "range function recovered a loop body panic and did not resume panicking";
        break;
    default:
        break;
    }
    detail::throw_runtime(detail::rt_error::error_string,
                          gp::string::gp_copy(std::string("runtime error: ") + text), runtime_id);
}

void panic_shift() {
    detail::throw_lit(detail::rt_error::error_string, "runtime error: negative shift amount",
                      runtime_id);
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
    detail::throw_lit(detail::rt_error::error_string,
                      "runtime error: invalid memory address or nil pointer dereference",
                      runtime_id);
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
    detail::throw_runtime(detail::rt_error::bounds, gp::string::gp_copy(text));
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
    if (a.gp_id.gp_kind == panic_identity::gp_words ||
        b.gp_id.gp_kind == panic_identity::gp_words) {
        // One of them is an interface value that was converted before the panic (panic(r) of a
        // recovered value, panic(err)): the words are the type and the data, which the other panic
        // has too (its box), whatever way gc would have made its value (a nil pointer in an
        // interface has the type and no data).
        return a.gp_value.gp_type() == b.gp_value.gp_type() &&
               a.gp_value.gp_data().get() == b.gp_value.gp_data().get();
    }
    return a.gp_id.gp_kind != panic_identity::gp_unique && a.gp_id.gp_kind == b.gp_id.gp_kind &&
           a.gp_id.gp_type == b.gp_id.gp_type && a.gp_id.gp_bits == b.gp_id.gp_bits &&
           a.gp_text.gp_view() == b.gp_text.gp_view();
}

} // namespace

void panic_report(std::string& out, const panic_error* const* chain, std::size_t n) {
    // Go's preprintpanics and printpanics: the first panic first, each later one after a tab;
    // a panic with the same value as the next one is printed once, as "repanicked", and the
    // next one is left out. A panic that recover() stopped and that is still in the chain (the
    // deferred call that recovered it panicked) is printed with " [recovered]".
    for (std::size_t k = 0; k < n; k++) {
        if (k > 0 && same(*chain[k - 1], *chain[k])) {
            continue;
        }
        if (k > 0) {
            out += "\n\t";
        }
        out += "panic: ";
        // Go's printindented: a tab after each newline of the value.
        gp::string text = panic_text(*chain[k]);
        for (char c : text.gp_view()) {
            out += c;
            if (c == '\n') {
                out += '\t';
            }
        }
        if (k + 1 < n && same(*chain[k], *chain[k + 1])) {
            out += " [recovered, repanicked]";
        } else if (chain[k]->gp_recovered) {
            out += " [recovered]";
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
    // Go names the goroutine that panics: 1 is the main one.
    out += "\n\ngoroutine ";
    print_uint(out, goroutine_id());
    out += " [running]:\n";
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

void fatal(std::string_view msg) noexcept {
    // One goroutine reports (claim_death); one that is already doing it goes on.
    claim_death();
    // Like a panic: what C or C++ code printf'd comes first. The header is written in parts,
    // so that it needs no memory; then the trace, if there is memory for it.
    std::fflush(nullptr);
    write_stderr("fatal error: ");
    write_stderr(msg);
    char line[64]; // "\n\ngoroutine N [running]:\n"
    char* p = line;
    for (const char* c = "\n\ngoroutine "; *c != '\0'; c++) {
        *p++ = *c;
    }
    p = std::to_chars(p, line + 40, goroutine_id()).ptr;
    for (const char* c = " [running]:\n"; *c != '\0'; c++) {
        *p++ = *c;
    }
    write_stderr(std::string_view(line, static_cast<std::size_t>(p - line)));
    std::string trace;
    fatal_trace(trace);
    write_stderr(trace);
    os::exit(2);
}

namespace {

// The terminate handler before on_terminate, which on_terminate leaves the cases without an
// exception to.
std::terminate_handler cxx_terminate = nullptr;

} // namespace

void cxx_exception_parts(const std::exception_ptr& e, std::string& type, std::string& what,
                         bool& has_what) {
    has_what = false;
    try {
        std::rethrow_exception(e);
    } catch (const std::exception& x) {
        has_what = true;
        what = x.what(); // the exception lives on in e
    } catch (...) {
    }
    // The type it was thrown with, also without RTTI (the program's flags may have -fno-rtti):
    // from the C++ ABI, which tells it only inside a handler.
    try {
        std::rethrow_exception(e);
    } catch (...) {
        if (const std::type_info* t = os::current_exception_type(); t != nullptr) {
            fmt::detail::cxx_type_name(type, *t);
        }
    }
    if (type.empty()) {
        type = "?";
    }
}

std::string cxx_exception_text(const std::exception_ptr& e) {
    std::string type;
    std::string what;
    bool has_what = false;
    cxx_exception_parts(e, type, what, has_what);
    std::string text = "C++ exception ";
    if (!has_what) {
        text += "of type ";
        text += type;
        return text;
    }
    text += type;
    if (!what.empty()) {
        text += ": ";
        text += what;
    }
    return text;
}

void on_terminate() noexcept {
    std::exception_ptr e = std::current_exception();
    if (e == nullptr) {
        if (cxx_terminate != nullptr && cxx_terminate != &on_terminate) {
            cxx_terminate();
        }
        std::abort();
    }
    claim_death(); // one goroutine reports
    // What C or C++ code printf'd comes first, as for a panic.
    std::fflush(nullptr);
    std::string out;
    try {
        std::rethrow_exception(e);
    } catch (panic_error& p) {
        // (A panic that no handler stopped, or that crossed a noexcept function: the frame of the
        // function that was to handle it is on the stack, for the rest of its stack.)
        p.gp_complete();
        panic_report(out, p); // a Go panic: run_main's report (the exception lives on in e)
        write_stderr(out);
        os::exit(2);
    } catch (const std::bad_alloc&) {
        // An allocation failure that the new handler didn't see (it ends the program itself):
        // a size that C++ can't even ask for (std::bad_array_new_length, new int[-1]), or a
        // std::bad_alloc that C++ code threw itself. Go's fatal error, with the trace of the
        // throw.
        fatal("runtime: out of memory");
    } catch (...) {
    }
    // The Go panic that is running, if a deferred call threw the exception: Go prints the
    // chain, the first panic first, as for a panic in a deferred call.
    const run_mark* m = run_marks();
    while (m != nullptr && m->gp_panic == nullptr) {
        m = m->gp_next;
    }
    std::vector<const panic_error*> chain;
    for (const panic_error* p = m != nullptr ? m->gp_panic : nullptr; p != nullptr;
         p = p->gp_prev.get()) {
        chain.push_back(p);
    }
    std::reverse(chain.begin(), chain.end());
    panic_error here{gp::string::gp_copy(cxx_exception_text(e))};
    chain.push_back(&here);
    panic_report(out, chain.data(), chain.size());
    fatal_trace(out); // the stack of the throw (and the running panic's, as Go shows it)
    write_stderr(out);
    os::exit(2);
}

void exit_with_panic(const panic_error& e) noexcept {
    claim_death(); // one goroutine reports
    // gocpp's own output is never buffered, but C or C++ code that the program calls may
    // have printf'd: that comes first, as it happened before the panic.
    std::fflush(nullptr);
    std::string out;
    // (A panic that C++ code caught on the way, at the border of an exported function, may have
    // only the inner part of its stack: the function that was to handle it is still on the stack.)
    const_cast<panic_error&>(e).gp_complete();
    panic_report(out, e); // with the stack trace
    write_stderr(out);
    os::exit(2);
}

void out_of_memory() {
    // An allocation that fails while the report is made (its trace) throws std::bad_alloc
    // now, which fatal_trace catches: the report then has no trace. Phase 08: when several
    // threads run out of memory at once, one of them should report.
    std::set_new_handler(nullptr);
    fatal("runtime: out of memory");
}

void flush_c_stdio() noexcept {
    std::fflush(stdout);
    std::fflush(stderr);
}

void write_stderr(std::string_view s) {
    // What C or C++ code printf'd before comes first (print.hpp).
    flush_c_stdio();
    // Go's runtime print ignores write errors, also a broken pipe.
    if (!s.empty()) {
        os::write(os::stderr_fd, s.data(), s.size());
    }
}

} // namespace detail

// --- run_main ---

namespace {

// put_hex and put_str append to a buffer without allocating, for on_stack_overflow.
char* put_hex(char* p, uintptr_t v) noexcept {
    char digits[16];
    int n = 0;
    do {
        digits[n++] = "0123456789abcdef"[v % 16];
        v /= 16;
    } while (v != 0);
    *p++ = '0';
    *p++ = 'x';
    while (n > 0) {
        *p++ = digits[--n];
    }
    return p;
}

char* put_str(char* p, const char* s) noexcept {
    while (*s != '\0') {
        *p++ = *s++;
    }
    return p;
}

// on_stack_overflow is the OS layer's hook (os::set_stack_overflow_hook): a thread ran past the
// end of its stack. It prints Go's lines for it, the stack in the second one being the thread's
// [lo, hi), and ends the program with code 2. It runs in a signal handler (or the like): it
// only formats numbers into a buffer and calls write and exit_now, no allocation and no lock
// (claim_death only uses atomics and sleeps). It prints no stack trace (Go does): walking the
// stack and reading the program's file there isn't safe, the fault may be inside malloc. For the
// same reason it doesn't flush C's stdio: what C code printf'd and didn't flush is lost.
void on_stack_overflow(const os::stack_fault& f) noexcept {
    detail::claim_death();
    char buf[320];
    char* p = put_str(buf, "runtime: goroutine stack exceeds 1000000000-byte limit\n"
                           "runtime: sp=");
    p = put_hex(p, f.gp_sp);
    p = put_str(p, " stack=[");
    p = put_hex(p, f.gp_lo);
    p = put_str(p, ", ");
    p = put_hex(p, f.gp_hi);
    p = put_str(p, "]\nfatal error: stack overflow\n");
    // Go's blank line and "goroutine N [running]:" (without a trace, and without Go's "runtime
    // stack:" section); a thread that has no goroutine record has no line.
    if (uint64_t id = detail::current_goroutine_id(); id != 0) {
        p = put_str(p, "\ngoroutine ");
        char digits[20];
        int n = 0;
        do {
            digits[n++] = static_cast<char>('0' + id % 10);
            id /= 10;
        } while (id != 0);
        while (n > 0) {
            *p++ = digits[--n];
        }
        p = put_str(p, " [running]:\n");
    }
    // Nothing to do when the write fails.
    os::write(os::stderr_fd, buf, static_cast<std::size_t>(p - buf));
    os::exit_now(2);
}

#ifdef _WIN32
// on_fault is the OS layer's hook for the exceptions of the CPU that nothing handled (an access
// violation of C code, an integer division by zero: os::set_fault_hook). It runs on the faulting
// thread's own stack, so unlike on_stack_overflow it may allocate and read the program's file for
// a trace. It prints what Go on Windows prints: the panic line (a fault at an address below 4096
// is a nil pointer dereference), the signal line with the exception code, code= (read or write)
// and the address, and the goroutine's trace, and ends the program with code 2. Go can recover
// from these panics; gocpp's report is final (a hardware fault is not a C++ exception).
void on_fault(const os::fault_info& f) noexcept {
    detail::claim_death();
    std::string out;
    try {
        bool access = f.gp_code == 0xc0000005 || f.gp_code == 0xc0000006;
        bool signal_line = true;
        if (access && f.gp_addr < 4096) {
            out += "panic: runtime error: invalid memory address or nil pointer dereference\n";
        } else if (access) {
            out += "unexpected fault address ";
            detail::print_hex(out, f.gp_addr);
            out += "\nfatal error: fault\n";
        } else if (f.gp_code == 0xc0000094) {
            out += "panic: runtime error: integer divide by zero\n";
        } else if (f.gp_code == 0xc0000095) {
            out += "panic: runtime error: integer overflow\n";
        } else {
            out += "Exception ";
            detail::print_hex(out, f.gp_code);
            out += ' ';
            detail::print_hex(out, f.gp_access);
            out += ' ';
            detail::print_hex(out, f.gp_addr);
            out += ' ';
            detail::print_hex(out, f.gp_pc);
            out += "\nPC=";
            detail::print_hex(out, f.gp_pc);
            out += '\n';
            signal_line = false;
        }
        if (signal_line) {
            out += "[signal ";
            detail::print_hex(out, f.gp_code);
            out += " code=";
            detail::print_hex(out, access ? f.gp_access : 0);
            out += " addr=";
            detail::print_hex(out, access ? f.gp_addr : 0);
            out += " pc=";
            detail::print_hex(out, f.gp_pc);
            out += "]\n";
        }
        out += "\ngoroutine ";
        detail::print_uint(out, detail::goroutine_id());
        out += " [running]:\n";
        detail::append_trace(out, f.gp_pcs, static_cast<std::size_t>(f.gp_npcs), true);
    } catch (...) {
        // Without memory the report is what there is.
    }
    detail::write_stderr(out);
    os::exit(2);
}
#endif

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

namespace detail {

void load_process_info() {
    args_storage() = to_strings(os::process_arguments());
    envs_storage() = to_strings(os::environment());
}

} // namespace detail

int run_main(int argc, char** argv, void (*init)(), void (*main)()) {
    // The main goroutine is registered first: its id is 1.
    detail::goroutine_id();
    os::set_stack_overflow_hook(on_stack_overflow);
#ifdef _WIN32
    os::set_fault_hook(on_fault);
#endif
    os::start();
    if constexpr (race::mode) {
        // -race: every goroutine has a thread of its own, so that a report's "created at" is the
        // go statement of that goroutine and not of the first one that used the thread.
        detail::set_idle_limit(0);
    }
    detail::enable_deadlock_detector(true);
    std::set_new_handler(detail::out_of_memory);
    detail::cxx_terminate = std::set_terminate(detail::on_terminate);
    args_storage() = to_strings(os::arguments(argc, argv));
    envs_storage() = to_strings(os::environment());
    try {
        // Go runs the package initializers with the main goroutine locked to its thread; a
        // LockOSThread in an init function keeps it locked for main (the count).
        detail::this_goroutine().locked++;
        if (init != nullptr) {
            init();
        }
        if (detail::goroutine& g0 = detail::this_goroutine(); g0.locked > 0) {
            g0.locked--;
        }
        main();
    } catch (const panic_error& e) {
        detail::exit_with_panic(e);
    } catch (const goexit_signal&) {
        // The main goroutine called runtime.Goexit: its deferred calls have run, and the
        // program goes on until the other goroutines are gone, then ends like Go (a program
        // that has no goroutine left cannot do anything): "fatal error: no goroutines (main
        // called runtime.Goexit) - deadlock!" without a trace, or the report of the deadlock when
        // the goroutines that are left are all asleep (detail::main_goexit).
        detail::main_goexit();
    }
    // Any other exception (a C++ exception, also a std::bad_alloc that C++ code throws) finds
    // no catch, so the C++ library calls detail::on_terminate on the stack of the throw, before
    // anything unwinds.
    detail::settle_panics(); // a goroutine that runs a panic's deferred calls ends the program
                             // first
    gp::exit(0);
}

void must_exit(const gp::error& err) {
    if (err.gp_type() == nullptr) {
        return;
    }
    std::string prog = "gocpp";
    if (!args_storage().empty()) {
        prog = std::string(args_storage()[0].gp_view());
        if (auto slash = prog.find_last_of("/\\"); slash != std::string::npos) {
            prog.erase(0, slash + 1);
        }
    }
    gp::string text = err.Error();
    std::string out = prog + ": ";
    out.append(text.gp_view());
    out += '\n';
    detail::write_stderr(out);
    os::exit(1);
}

} // namespace gp
