// The unit tests of step 4.12 (Phase 04 §2.8 and §2.12): how fmt (gocpp/fmtshim.hpp) and the
// builtin print (gocpp/print.hpp) print C/C++ values (numbers, enums, raw pointers, nullptr,
// C++ strings, and class types through std::formatter or operator<<, gocpp/fmtcxx.hpp), the
// printable checks behind gp::cgo::fmt_arg and print_arg, and the text of an unrecovered C++
// exception (cxx_exception_text). runtime/cxx_test.go compiles this file with the runtime's .cpp
// files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan, and runs it like
// test/unit.cpp: it prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
// With an argument it runs one case that ends the program instead (the cases at the end of the
// file: C++ exceptions that nothing catches, and C's stdio next to fmt), which cxx_test.go's
// runCxxValue checks from the outside.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <exception>
#include <format>
#include <ostream>
#include <stdexcept>
#include <string>
#include <string_view>
#include <system_error>
#include <type_traits>

namespace cxxp {

// Like the types of a C++ header that a program imports.
struct Streamed {
    int a, b;
};
inline std::ostream& operator<<(std::ostream& os, const Streamed& s) {
    return os << "S(" << s.a << "," << s.b << ")";
}

struct Formatted {
    int v;
};

// Both: the std::formatter wins over the operator<<.
struct Both {
    int v;
};
inline std::ostream& operator<<(std::ostream& os, const Both&) {
    return os << "stream";
}

struct Neither {
    int v;
};

union Mixed {
    int i;
    float f;
};

enum Color { Red, Green = 5 };
enum class Small : int8_t { M = -3 };

// An exception class that isn't a std::exception, and one that is, with an empty what().
struct Oops {
    int code;
};
struct Quiet : std::exception {
    const char* what() const noexcept override { return ""; }
};

void plain() {}

} // namespace cxxp

template <>
struct std::formatter<cxxp::Formatted> : std::formatter<int> {
    auto format(const cxxp::Formatted& f, std::format_context& ctx) const {
        return std::format_to(ctx.out(), "F{}", f.v);
    }
};

template <>
struct std::formatter<cxxp::Both> : std::formatter<int> {
    auto format(const cxxp::Both&, std::format_context& ctx) const {
        return std::format_to(ctx.out(), "formatter");
    }
};

// After the program's headers, as in generated code.
#include <gocpp/fmtcxx.hpp>

namespace {

namespace fmt = gp::fmtshim;
namespace detail = gp::fmtshim::detail;

// gocpp structs with C/C++ fields (the emitter's struct protocol, fmtshim.hpp).
struct Holder {
    int n = 3;              // C.int
    cxxp::Streamed s{1, 2}; // a C++ type with an operator<<
    gp::string name = gp::string::gp_copy("h");
    static constexpr const char* gp_type_name = "main.Holder";
    static constexpr auto gp_fields = gp::fields(gp::field(&Holder::n, "n", false, false),
                                                 gp::field(&Holder::s, "s", false, false),
                                                 gp::field(&Holder::name, "name", false, false));
};

struct Bad {
    cxxp::Neither x{};
    [[maybe_unused]] static constexpr const char* gp_type_name = "main.Bad";
    static constexpr auto gp_fields = gp::fields(gp::field(&Bad::x, "x", false, false));
};

// A recursive struct with a C field: the printable check ends.
struct Node {
    int v = 0;
    gp::slice<Node> kids;
    gp::ptr<Node> next;
    static constexpr const char* gp_type_name = "main.Node";
    static constexpr auto gp_fields = gp::fields(gp::field(&Node::v, "v", false, false),
                                                 gp::field(&Node::kids, "kids", false, false),
                                                 gp::field(&Node::next, "next", false, false));
};

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

#define CHECK_EQ(got, want)                                                                        \
    do {                                                                                           \
        std::string gp_got = (got);                                                                \
        std::string gp_want = (want);                                                              \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = \"" + gp_got + "\", want \"" + gp_want + "\"");                            \
    } while (0)

std::string str(const gp::string& s) {
    return std::string(s.gp_view());
}

// sprint and sprintf are fmt.Sprint and fmt.Sprintf with the arguments as the emitter passes
// C/C++ values (gp::cgo::fmt_arg), as std::strings.
template <typename... A>
std::string sprint(A... a) {
    return str(fmt::Sprint(gp::cgo::fmt_arg(a)...));
}
template <typename... A>
std::string sprintf(const char* f, A... a) {
    return str(fmt::Sprintf(gp::string::gp_copy(f), gp::cgo::fmt_arg(a)...));
}

// hex is the address as Go's %v prints it.
std::string hex(const void* p) {
    char buf[32];
    std::snprintf(buf, sizeof buf, "0x%llx",
                  static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(p)));
    return buf;
}

std::string dec(const void* p) {
    return std::to_string(static_cast<unsigned long long>(reinterpret_cast<std::uintptr_t>(p)));
}

void test_numbers() {
    // C's numbers print as the Go types of their kind and size (a C.char is a number, as in cgo).
    CHECK_EQ(sprint(int(7), char('a'), static_cast<unsigned char>(200), short(-2)), "7 97 200 -2");
    CHECK_EQ(sprint(1LL << 40, 3UL, std::size_t(4), 1.5f, 0.25, true),
             "1099511627776 3 4 1.5 0.25 true");
    CHECK_EQ(sprintf("%T %T %T %T %T", int(1), char(1), short(1), 1u, static_cast<signed char>(1)),
             std::is_signed_v<char> ? "int32 int8 int16 uint32 int8"
                                    : "int32 uint8 int16 uint32 int8");
    // long is int64_t on Linux (Go's int: C++ can't tell them apart), long long is not.
    CHECK_EQ(sprintf("%T %T", int64_t(1), 1LL), "int int64");
    CHECK_EQ(sprintf("%T %T %T", uint64_t(1), 1ULL, 1.0f), "uint uint64 float32");
    CHECK_EQ(sprintf("%5d|%-4v|%s", int(42), short(7), int(3)), "   42|7   |%!s(int32=3)");
}

void test_enums() {
    CHECK_EQ(sprint(cxxp::Green, cxxp::Red, cxxp::Small::M), "5 0 -3");
    CHECK_EQ(sprintf("%d|%3v|%s", cxxp::Green, cxxp::Green, cxxp::Green),
             "5|  5|%!s(cxxp::Color=5)");
    CHECK_EQ(sprintf("%T %T", cxxp::Green, cxxp::Small::M), "cxxp::Color cxxp::Small");
}

void test_pointers() {
    int x = 1;
    int* px = &x;
    int* null = nullptr;
    const char* hello = "hello";
    char* cnull = nullptr;
    void* vp = px;
    void (*fp)() = &cxxp::plain;
    // Addresses, as Go prints a pointer; a char* is never read as a string.
    CHECK_EQ(sprint(px), hex(px));
    CHECK_EQ(sprint(hello), hex(hello));
    CHECK_EQ(sprint(vp), hex(px));
    CHECK_EQ(sprint(reinterpret_cast<const void*>(fp)).substr(0, 2), "0x");
    CHECK_EQ(sprint(fp), sprint(reinterpret_cast<const void*>(fp)));
    CHECK_EQ(sprint(null, cnull), "<nil> <nil>");
    CHECK_EQ(sprintf("%d %d", px, null), dec(px) + " 0");
    CHECK_EQ(sprintf("%s", px), "%!s(int*=" + hex(px) + ")");
    CHECK_EQ(sprintf("%q", cnull), "%!q(char*=<nil>)");
    CHECK_EQ(sprintf("%T|%T|%T|%T", px, hello, vp, fp), "int*|char const*|void*|void (*)()");
    // nullptr_t: like Go's nil interface.
    CHECK_EQ(sprint(nullptr), "<nil>");
    CHECK_EQ(sprintf("%v|%d|%T", nullptr, nullptr, nullptr), "<nil>|%!d(<nil>)|<nil>");
}

void test_strings() {
    std::string a = "a";
    std::string_view b = "b";
    // C++ strings print as Go strings: Print puts no space next to them.
    CHECK_EQ(sprint(a, b), "ab");
    CHECK_EQ(sprint(a, int(1), int(2)), "a1 2");
    CHECK_EQ(sprintf("%s|%q|%4v|%d", a, std::string("x\n"), b, a),
             "a|\"x\\n\"|   b|%!d(std::string=a)");
    CHECK_EQ(sprintf("%T %T", a, b), "std::string std::string_view");
    CHECK_EQ(str(fmt::Sprintln(gp::cgo::fmt_arg(a), gp::cgo::fmt_arg(b))), "a b\n");
}

void test_classes() {
    cxxp::Streamed s{1, 2};
    // operator<<, std::formatter, and the formatter first when there are both.
    CHECK_EQ(sprint(s), "S(1,2)");
    CHECK_EQ(sprint(cxxp::Formatted{7}), "F7");
    CHECK_EQ(sprint(cxxp::Both{0}), "formatter");
    CHECK_EQ(sprintf("%v|%s|%q|%8v|%-8s|%d", s, s, s, s, s, s),
             "S(1,2)|S(1,2)|\"S(1,2)\"|  S(1,2)|S(1,2)  |%!d(cxxp::Streamed=S(1,2))");
    CHECK_EQ(sprintf("%T", s), "cxxp::Streamed");
    // A class of the C++ library with an operator<< (std::error_code prints "generic:22").
    CHECK_EQ(sprint(std::make_error_code(std::errc::invalid_argument)).substr(0, 8), "generic:");
}

void test_containers() {
    // C/C++ values inside gocpp values: a []C.int, a struct with C/C++ fields, a pointer to it.
    gp::slice<int> ints = gp::make_slice<int>(2);
    ints.gp_data()[0] = 1;
    ints.gp_data()[1] = 2;
    CHECK_EQ(sprint(ints), "[1 2]");
    CHECK_EQ(sprintf("%s", ints), "[%!s(int32=1) %!s(int32=2)]");
    Holder h;
    CHECK_EQ(sprint(h), "{3 S(1,2) h}");
    CHECK_EQ(sprintf("%+v", h), "{n:3 s:S(1,2) name:h}");
    CHECK_EQ(sprint(gp::new_<Holder>()), "&{3 S(1,2) h}");
    Node n;
    CHECK_EQ(sprint(n), "{0 [] <nil>}");
}

void test_printable() {
    using detail::printable_arg;
    static_assert(printable_arg<cxxp::Streamed> && printable_arg<cxxp::Formatted>);
    static_assert(printable_arg<int> && printable_arg<cxxp::Color> && printable_arg<char*>);
    static_assert(printable_arg<std::nullptr_t> && printable_arg<std::string_view>);
    static_assert(printable_arg<Holder> && printable_arg<gp::ptr<Holder>> && printable_arg<Node>);
    static_assert(printable_arg<fmt::typed_value<fmt::tag::int64, int64_t>>);
    static_assert(!printable_arg<cxxp::Neither> && !printable_arg<cxxp::Mixed>);
    static_assert(!printable_arg<Bad> && !printable_arg<gp::ptr<Bad>>);
    static_assert(!printable_arg<gp::slice<cxxp::Neither>>);
    static_assert(!printable_arg<int cxxp::Streamed::*>); // a pointer to a member
    // The builtin print takes numbers, bools, enums, raw pointers, nullptr and C++ strings.
    using gp::detail::print_ok;
    static_assert(print_ok<int> && print_ok<char> && print_ok<cxxp::Color> &&
                  print_ok<const char*>);
    static_assert(print_ok<std::nullptr_t> && print_ok<std::string> && print_ok<gp::ptr<int>>);
    static_assert(!print_ok<cxxp::Streamed> && !print_ok<Holder>);
    CHECK(true);
}

void test_print() {
    // The builtin print's format for C/C++ values (gocpp/print.hpp).
    std::string out;
    const char* cnull = nullptr;
    int x = 0;
    gp::detail::append_print(out, gp::cgo::print_arg(cxxp::Green));
    out += ' ';
    gp::detail::append_print(out, gp::cgo::print_arg(std::string("str")));
    out += ' ';
    gp::detail::append_print(out, gp::cgo::print_arg(cnull));
    out += ' ';
    gp::detail::append_print(out, gp::cgo::print_arg(nullptr));
    out += ' ';
    gp::detail::append_print(out, gp::cgo::print_arg(char('a')));
    out += ' ';
    gp::detail::append_print(out, gp::cgo::print_arg(2.5));
    CHECK_EQ(out, "5 str 0x0 0x0 97 +2.500000e+000");
    out.clear();
    gp::detail::append_print(out, gp::cgo::print_arg(&x));
    CHECK_EQ(out, hex(&x));
}

// text_of returns cxx_exception_text of what f throws.
template <typename F>
std::string text_of(F f) {
    try {
        f();
    } catch (...) {
        return gp::detail::cxx_exception_text(std::current_exception());
    }
    return "<no exception>";
}

void test_exception_text() {
    CHECK_EQ(text_of([] { throw std::out_of_range("index 5"); }),
             "C++ exception std::out_of_range: index 5");
    CHECK_EQ(text_of([] { throw std::runtime_error("two\nlines"); }),
             "C++ exception std::runtime_error: two\nlines");
    CHECK_EQ(text_of([] { throw cxxp::Quiet(); }), "C++ exception cxxp::Quiet");
    CHECK_EQ(text_of([] { throw 42; }), "C++ exception of type int");
    CHECK_EQ(text_of([] { throw cxxp::Oops{1}; }), "C++ exception of type cxxp::Oops");
    CHECK_EQ(text_of([] { throw "text"; }), "C++ exception of type char const*");
    // The type it was thrown with, not the type it was caught as.
    CHECK_EQ(text_of([] {
                 try {
                     throw std::invalid_argument("bad");
                 } catch (const std::exception&) {
                     throw;
                 }
             }),
             "C++ exception std::invalid_argument: bad");
}

void run_tests() {
    test_numbers();
    test_enums();
    test_pointers();
    test_strings();
    test_classes();
    test_containers();
    test_printable();
    test_print();
    test_exception_text();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

// The cases that end the program (cxx_test.go's runCxxValue checks their output and exit
// code), from the program's argument.
const char* the_case = nullptr;

[[gnu::noinline]] void throw_range(int n) {
    if (n > 0) {
        throw std::out_of_range("index " + std::to_string(n));
    }
}

// alloc_ints is new int[n]: with a negative n, g++ throws std::bad_array_new_length, which the
// new handler never sees (clang++ asks operator new for SIZE_MAX bytes instead).
[[gnu::noinline]] int* alloc_ints(long n) {
    return new int[n];
}

[[gnu::noinline]] void throw_bad_alloc() {
    throw std::bad_alloc();
}

[[gnu::noinline]] void call_noexcept(void (*f)()) noexcept {
    f();
}

void go_panic() {
    gp::panic("go panic"_gs);
}

void throw_deferred() {
    throw std::logic_error("from a deferred call");
}

void run_case() {
    std::string c = the_case;
    if (c == "throw-std-bare") {
        // A std::exception that no frame with defers catches (here none is on the stack): the
        // C++ library finds no handler and calls std::terminate where it was thrown, before
        // anything unwinds, and the report is a Go panic's, with the trace of the throw.
        throw_range(5);
    } else if (c == "bad-alloc-bare") {
        throw_bad_alloc();
    } else if (c == "throw-std") {
        // A std::exception that reaches a function with defers is a Go panic (step 5.10): the
        // deferred call runs, then "panic: C++ exception <type>: <what()>", exit code 2, with the
        // trace of the frame that caught it.
        gp::defer_frame df;
        try {
            df.gp_push_call([] { gp::println("deferred"_gs); });
            throw_range(5);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } else if (c == "bad-array" || c == "bad-alloc") {
        // Go's fatal error "runtime: out of memory", with the trace of the frame that caught it;
        // a fatal error is not a panic, so the deferred call doesn't run.
        gp::defer_frame df;
        try {
            df.gp_push_call([] { gp::println("deferred"_gs); });
            if (c == "bad-array") {
                long n = -static_cast<long>(c.size()); // not a constant: no warning
                int* p = alloc_ints(n);
                gp::println(int64_t(p[0]));
                delete[] p;
            } else {
                throw_bad_alloc();
            }
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } else if (c == "throw-int") {
        throw 7;
    } else if (c == "throw-in-defer") {
        // A deferred call that a Go panic runs throws: Go's chain of panics.
        gp::defer_frame df;
        try {
            df.gp_push(&throw_deferred);
            gp::panic("first"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } else if (c == "panic-noexcept") {
        // A Go panic that reaches a noexcept C++ function: the C++ library calls
        // std::terminate there, and the report is the panic's.
        call_noexcept(&go_panic);
    } else if (c == "stdio") {
        // C's stdio and fmt, into a pipe: in the program's order (fmt flushes C's stdout first;
        // the builtin println C's stderr).
        std::printf("1 printf\n");
        fmt::Println("2 fmt"_gs);
        std::printf("3 printf\n");
        gp::println("4 println"_gs);
        std::fputs("5 fputs stderr\n", stderr);
        fmt::Print("6 fmt\n"_gs);
        std::printf("7 printf, flushed at the end\n");
    } else {
        gp::println("unknown case"_gs);
    }
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    if (argc == 2) {
        the_case = argv[1];
        return gp::run_main(argc, argv, &no_init, &run_case);
    }
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
