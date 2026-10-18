// The unit tests of a -nochecks build: runtime/cxx_test.go compiles this file and the
// runtime's .cpp files with -DGOCPP_NOCHECKS, as gocpp build -nochecks does, with g++ and
// clang++, at -O0, at -O2 and with ASan + UBSan, and runs it like test/unit.cpp. The bounds and
// nil checks are gone (checks.hpp); the division and shift checks, Go's defined results and the
// panics that are part of an operation stay. Nothing here goes out of bounds for real: an index
// past len reads inside the capacity, so the sanitizers stay quiet.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <limits>
#include <memory>
#include <string>

namespace {

// The build defines GOCPP_NOCHECKS (TestWindowsSyntax compiles the file without it, only to
// check the syntax).
#ifdef GOCPP_NOCHECKS
constexpr bool nochecks = true;
#else
constexpr bool nochecks = false;
#endif

int checks = 0;
int failures = 0;

std::string show(int64_t v) {
    return std::to_string(static_cast<long long>(v));
}
std::string show(const std::string& v) {
    return '"' + v + '"';
}

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
        auto gp_got = (got);                                                                       \
        decltype(gp_got) gp_want = (want);                                                         \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = " + show(gp_got) + ", want " + show(gp_want));                             \
    } while (0)

// panic_text runs f and returns the text of the panic it throws, or "<no panic>".
template <typename F>
std::string panic_text(F f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        return std::string(e.gp_text.gp_view());
    }
    return "<no panic>";
}

#define CHECK_PANIC(stmt, want) CHECK_EQ(panic_text([&] { stmt; }), std::string(want))

// Values the compiler can't fold, so the checks really run.
template <typename T>
T opaque(T v) {
    static volatile T sink;
    sink = v;
    return sink;
}

const char* const none = "<no panic>";

void test_checks_off() {
    CHECK(nochecks);
    // The checks themselves do nothing.
    CHECK_PANIC(gp::check_index(opaque(INT64_C(5)), 3), none);
    CHECK_PANIC(gp::check_index(opaque(INT64_C(-1)), 3), none);
    CHECK_PANIC(gp::check_index(opaque(UINT64_MAX), 3), none);
    CHECK_PANIC(gp::check_slice(opaque(INT64_C(0)), opaque(INT64_C(5)), 3, gp::bounds::slice_acap),
                none);
    CHECK_PANIC(gp::check_slice(opaque(INT64_C(2)), opaque(INT64_C(1)), 3, gp::bounds::slice_alen),
                none);
    CHECK_PANIC(gp::check_slice3(opaque(INT64_C(0)), opaque(INT64_C(4)), opaque(INT64_C(2)), 3,
                                 gp::bounds::slice3_acap),
                none);
    CHECK_PANIC(gp::check_convert(opaque(INT64_C(3)), 2), none);
    CHECK_PANIC(gp::check_wrap(gp::ptr<int64_t>(), "value method main.T.M called using nil *T "
                                                   "pointer"),
                none);

    // A slice index past len (inside cap), and a string index past the substring's end (inside
    // the literal): no panic, the element that is there.
    gp::slice<int64_t> s = gp::make_slice<int64_t>(1, 4);
    CHECK_EQ(s[opaque(INT64_C(2))], INT64_C(0));
    gp::string t = gp::sub("abcdef"_gs, 0, 2);
    CHECK_EQ(int64_t(t[opaque(INT64_C(4))]), int64_t('e'));
    // A slice expression past cap only computes the new header.
    gp::slice<int64_t> u = gp::sub(s, 0, opaque(INT64_C(1)), opaque(INT64_C(1)));
    gp::slice<int64_t> w = gp::sub(u, 0, opaque(INT64_C(3)));
    CHECK_EQ(gp::len(w), INT64_C(3));

    // A nil pointer: p.operator->() gives the null address instead of panicking (a real
    // dereference would be undefined behavior).
    gp::ptr<int64_t> p;
    CHECK(p.operator->() == nullptr);
    // A non-nil pointer without an owner (a bug that the owner check reports) is not checked.
    int64_t x = 7;
    gp::ptr<int64_t> q(std::shared_ptr<int64_t>(), &x);
    CHECK_EQ(*q, INT64_C(7));
}

void test_checks_on() {
    // Without these checks, C++ has undefined behavior; they stay, and Go's results stay.
    const char* divide = "runtime error: integer divide by zero";
    const char* shift = "runtime error: negative shift amount";
    CHECK_PANIC(gp::div(opaque(INT64_C(1)), opaque(INT64_C(0))), divide);
    CHECK_PANIC(gp::rem(opaque(INT64_C(1)), opaque(INT64_C(0))), divide);
    CHECK_PANIC(gp::shl(opaque(INT64_C(1)), opaque(INT64_C(-1))), shift);
    CHECK_PANIC(gp::shr(opaque(INT64_C(1)), opaque(INT64_C(-1))), shift);
    constexpr int64_t min = std::numeric_limits<int64_t>::min();
    CHECK_EQ(gp::div(opaque(min), opaque(INT64_C(-1))), min);
    CHECK_EQ(gp::rem(opaque(min), opaque(INT64_C(-1))), INT64_C(0));
    CHECK_EQ(gp::shl(opaque(INT64_C(1)), opaque(INT64_C(64))), INT64_C(0));
    CHECK_EQ(gp::shr(opaque(INT64_C(-8)), opaque(INT64_C(64))), INT64_C(-1));
    // The panics that are part of an operation.
    gp::map<int64_t, int64_t> m;
    CHECK_PANIC(gp::set(m, INT64_C(1), INT64_C(2)), "assignment to entry in nil map");
    CHECK_PANIC(gp::make_slice<int64_t>(opaque(INT64_C(-1))),
                "runtime error: makeslice: len out of range");
    gp::array<int64_t, 0> empty;
    CHECK_PANIC(empty[opaque(INT64_C(0))], "runtime error: index out of range [0] with length 0");
}

void no_init() {}

void run_tests() {
    test_checks_off();
    test_checks_on();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
