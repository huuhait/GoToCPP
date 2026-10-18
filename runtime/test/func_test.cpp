// The unit tests of gp::func (gocpp/func.hpp): nil and == nil, calling nil (Go's nil dereference
// panic), copies that share the closure, moves that leave nil behind, func_of, closures without
// captures (no allocation), gp_call keeping a closure alive while the call drops it, arguments
// moved through, and a long chain of closures freed without deep recursion. runtime/cxx_test.go
// compiles this file with the runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with
// ASan + UBSan (LeakSanitizer included), and runs it like test/unit.cpp. It prints "ok: N checks"
// and exits 0, or prints each failed check and exits 1. The Go behavior of func values is tested
// against real Go by the emitter's tests (internal/cppgen TestEmit closures, TestPanics func-*,
// tests/run funcs_* and closures_*).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <memory>
#include <string>
#include <type_traits>
#include <utility>

namespace {

int checks = 0;
int failures = 0;

std::string show(const std::string& v) {
    return '"' + v + '"';
}
std::string show(int64_t v) {
    return std::to_string(static_cast<long long>(v));
}
std::string show(int v) {
    return std::to_string(v);
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

const std::string nil_msg = "runtime error: invalid memory address or nil pointer dereference";

int64_t twice(int64_t x) {
    return 2 * x;
}

std::tuple<int64_t, bool> split(int64_t x) {
    return {x / 2, x % 2 == 1};
}

void test_nil() {
    gp::func<int64_t(int64_t)> f;
    CHECK(f == nullptr);
    CHECK(nullptr == f);
    gp::func<int64_t(int64_t)> g = nullptr;
    CHECK(g == nullptr);
    CHECK_EQ(panic_text([&] { f(1); }), nil_msg);
    CHECK_EQ(panic_text([&] { f.gp_call(1); }), nil_msg);
    // The arguments are evaluated before the nil check, as in Go.
    int64_t seen = 0;
    CHECK_EQ(panic_text([&] { f(seen = 7); }), nil_msg);
    CHECK_EQ(seen, int64_t(7));
}

void test_values() {
    auto f = gp::func_of<&twice>();
    CHECK(f != nullptr);
    CHECK_EQ(f(21), int64_t(42));
    CHECK((std::is_same_v<decltype(f), gp::func<int64_t(int64_t)>>));
    auto s = gp::func_of<&split>();
    auto [h, odd] = s(7);
    CHECK_EQ(h, int64_t(3));
    CHECK(odd);

    // A closure without captures is made again for each call: no owner.
    gp::func<int64_t(int64_t)> sq([]<int = 1>(int64_t x) -> int64_t { return x * x; });
    CHECK_EQ(sq(9), int64_t(81));

    // Copies share the closure, so they share what it captured by copy (here a box).
    auto n = gp::new_<int64_t>(0);
    gp::func<int64_t()> next([n]<int = 2>() mutable -> int64_t { return ++*n; });
    gp::func<int64_t()> copy = next;
    CHECK_EQ(next(), int64_t(1));
    CHECK_EQ(copy(), int64_t(2));
    CHECK_EQ(*n, int64_t(2));

    // A move leaves nil behind.
    gp::func<int64_t()> moved = std::move(copy);
    CHECK(copy == nullptr);
    CHECK_EQ(moved(), int64_t(3));
    copy = std::move(moved);
    CHECK(moved == nullptr);
    CHECK_EQ(copy(), int64_t(4));
    copy = nullptr;
    CHECK(copy == nullptr);
    CHECK(next != nullptr);

    // A closure that captured a string copies it once, when it is made.
    gp::string s1 = "go"_gs;
    gp::func<gp::string(gp::string)> cat(
        [s1]<int = 3>(gp::string t) mutable -> gp::string { return s1 + t; });
    s1 = "changed"_gs;
    CHECK_EQ(std::string(cat("pher"_gs).gp_view()), std::string("gopher"));
}

// A class whose moves and copies are counted: arguments are moved through the call, not copied.
struct counted {
    static inline int copies = 0;
    int64_t v = 0;
    counted() = default;
    explicit counted(int64_t x) : v(x) {}
    counted(const counted& o) : v(o.v) { copies++; }
    counted(counted&& o) noexcept : v(o.v) {}
    counted& operator=(const counted&) = default;
    counted& operator=(counted&&) = default;
};

void test_arguments() {
    gp::func<int64_t(counted)> f([]<int = 1>(counted c) -> int64_t { return c.v; });
    counted::copies = 0;
    CHECK_EQ(f(counted(5)), int64_t(5));
    CHECK_EQ(counted::copies, 0);
    // A const reference parameter (a value larger than 128 KiB) is passed as one.
    gp::func<int64_t(const counted&)> g([]<int = 2>(const counted& c) -> int64_t { return c.v; });
    counted c(9);
    CHECK_EQ(g(c), int64_t(9));
    CHECK_EQ(counted::copies, 0);
}

gp::func<int64_t()> self;

void test_keep_alive() {
    // The call drops the last other owner of the closure that runs (f = nil inside f): gp_call
    // keeps it alive, so the closure's captures stay valid until it returns.
    auto data = gp::slice_of<int64_t>({1, 2, 3});
    self = gp::func<int64_t()>([data]<int = 1>() mutable -> int64_t {
        self = nullptr;
        int64_t t = 0;
        for (int64_t i = 0; i < gp::len(data); i++) {
            t += data[i];
        }
        return t;
    });
    CHECK_EQ(self.gp_call(), int64_t(6));
    CHECK(self == nullptr);
}

void test_long_chain() {
    // Each closure captures the one before it: freeing the last one frees 200,000 closures, through
    // detail::release, without deep recursion.
    gp::func<int64_t()> f([]<int = 1>() -> int64_t { return 0; });
    for (int i = 0; i < 200000; i++) {
        f = gp::func<int64_t()>([prev = f]<int = 2>() mutable -> int64_t { return prev() + 1; });
    }
    gp::func<int64_t()> small([]<int = 1>() -> int64_t { return 0; });
    for (int i = 0; i < 100; i++) {
        small = gp::func<int64_t()>(
            [prev = small]<int = 2>() mutable -> int64_t { return prev() + 1; });
    }
    CHECK_EQ(small(), int64_t(100));
    f = nullptr;
    CHECK(f == nullptr);
}

void run_tests() {
    test_nil();
    test_values();
    test_arguments();
    test_keep_alive();
    test_long_chain();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
