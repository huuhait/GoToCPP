// The unit tests of the GC engine's print and println (phase-16 README §2.2; job 16.1c):
// Go's format of ints, bools and strings, the spaces of println, and its newline.
//
// runtime/cxx_test.go compiles this file with the runtime's .cpp files and the GC engine's
// sources (rtfiles.GCSources) and runs it like test/unit.cpp. The output of print and println
// is captured with the runtime's test hook (GOCPP_RUNTIME_TEST, gocpp/gc/print.hpp) instead of
// being written to standard error, so the test's own stderr stays empty on success.
#include <gocpp/gc/rt.hpp>

#include <cstdint>
#include <cstdio>
#include <limits>
#include <string>
#include <string_view>

#ifndef GOCPP_RUNTIME_TEST
#error "gc_print_test.cpp needs -DGOCPP_RUNTIME_TEST (runtime/cxx_test.go's flags)"
#endif

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::fprintf(stderr, "%s:%d: %s\n", file, line, what.c_str());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

// captured is the output of the last print or println call (the test hook receives it).
std::string captured;

// check_out reports the last call's output against its expected bytes.
void check_out(const char* what, const std::string& want) {
    checks++;
    if (captured != want) {
        failures++;
        std::fprintf(stderr, "%s: got %s, want %s\n", what, captured.c_str(), want.c_str());
    }
}

} // namespace

int main() {
    gp::gc::detail::write_stderr_hook = [](std::string_view s) { captured.assign(s); };
    using gp::gc::lit;

    // print_int: Go's decimal format, including the extremes of int64.
    std::string out;
    gp::gc::detail::print_int(out, 0);
    CHECK(out == "0");
    out.clear();
    gp::gc::detail::print_int(out, 42);
    CHECK(out == "42");
    out.clear();
    gp::gc::detail::print_int(out, -7);
    CHECK(out == "-7");
    out.clear();
    gp::gc::detail::print_int(out, 9223372036854775807LL);
    CHECK(out == "9223372036854775807");
    out.clear();
    gp::gc::detail::print_int(out, std::numeric_limits<std::int64_t>::min());
    CHECK(out == "-9223372036854775808");

    // append_print of each of the three types, with nothing between them.
    out.clear();
    gp::gc::detail::append_print(out, true);
    gp::gc::detail::append_print(out, false);
    gp::gc::detail::append_print(out, lit("x"));
    gp::gc::detail::append_print(out, 5);
    CHECK(out == "truefalsex5");

    // println: one space between the arguments and a newline at the end.
    captured.clear();
    gp::gc::println(lit("hello"), 42);
    check_out("println string int", "hello 42\n");

    captured.clear();
    gp::gc::println();
    check_out("println no args", "\n");

    captured.clear();
    gp::gc::println(true, false);
    check_out("println bools", "true false\n");

    captured.clear();
    gp::gc::println(lit(""), lit("x"));
    check_out("println empty string", " x\n");

    captured.clear();
    gp::gc::println(-7, std::numeric_limits<std::int64_t>::min());
    check_out("println negative ints", "-7 -9223372036854775808\n");

    captured.clear();
    gp::gc::println(gp::gc::string());
    check_out("println zero string", "\n");

    // A NUL inside a string is a byte that println writes.
    captured.clear();
    gp::gc::println(lit("a\0b"));
    check_out("println embedded NUL", std::string("a\0b\n", 4));

    // print: nothing between the arguments and no newline.
    captured.clear();
    gp::gc::print(lit("a"), 1, true);
    check_out("print", "a1true");

    captured.clear();
    gp::gc::print();
    check_out("print no args", "");

    if (failures != 0) {
        std::fprintf(stderr, "%d of %d checks failed\n", failures, checks);
        return 1;
    }
    std::printf("ok: %d checks\n", checks);
    return 0;
}
