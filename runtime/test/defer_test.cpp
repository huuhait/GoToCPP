// The unit tests of gp::defer_frame (gocpp/defer.hpp) and of the panic chain (panic.hpp,
// trace.cpp's panic_chain and panic_report). runtime/cxx_test.go compiles this file with the
// runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan
// (LeakSanitizer included), and runs it like test/unit.cpp. It prints "ok: N checks" and exits
// 0, or prints each failed check and exits 1. The Go behavior (the order, the chain's text) is
// tested against real Go by the emitter's tests (internal/cppgen TestPanics, tests/run and
// tests/diff defer_*).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstddef>
#include <cstdint>
#include <exception>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

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

// The calls that the deferred functions made, in order.
std::string calls;

void note(int64_t n) {
    calls += std::to_string(static_cast<long long>(n)) + " ";
}

void note_str(gp::string s) {
    calls += std::string(s.gp_view()) + " ";
}

void boom(gp::string s) {
    calls += "boom(" + std::string(s.gp_view()) + ") ";
    gp::panic(s);
}

void throw_cpp() {
    calls += "throw ";
    throw std::runtime_error("c++");
}

int64_t with_result(int64_t n) {
    calls += "result ";
    return n;
}

// A big argument (stored on the heap) and an over-aligned one.
struct big {
    int64_t v[64] = {};
};
void note_big(const big& b) {
    note(b.v[63]);
}
struct alignas(64) aligned {
    int64_t v = 0;
};
void note_aligned(const aligned& a) { // a is the stored copy
    report(reinterpret_cast<std::uintptr_t>(&a) % 64 == 0, __FILE__, __LINE__, "aligned");
    note(a.v);
}

// Counts its live copies, so a test sees that every stored argument is destroyed.
int live = 0;
struct counted {
    int64_t v = 0;
    explicit counted(int64_t x) : v(x) { live++; }
    counted(const counted& o) : v(o.v) { live++; }
    counted(counted&& o) noexcept : v(o.v) { live++; }
    ~counted() { live--; }
};
void note_counted(counted c) {
    note(c.v);
}

// A copy that throws: building the node fails.
int copies = 0;
struct fragile {
    fragile() = default;
    fragile(const fragile&) {
        if (++copies == 1) {
            throw std::runtime_error("copy");
        }
    }
};

// text runs f and returns the text of its panic (the newest), or "<no panic>".
template <typename F>
std::string text(F f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        return std::string(e.gp_text.gp_view());
    }
    return "<no panic>";
}

// report_of runs f, which must panic, and returns what run_main would print up to the trace.
template <typename F>
std::string report_of(F f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        std::string out;
        std::vector<const gp::panic_error*> chain;
        for (const gp::panic_error* p = &e; p != nullptr; p = p->gp_prev.get()) {
            chain.insert(chain.begin(), p);
        }
        gp::detail::panic_report(out, chain.data(), chain.size());
        return out;
    }
    return "<no panic>";
}

void test_order() {
    // Every call of a function with defers has a frame on the C++ stack: keep it small.
    CHECK(sizeof(gp::defer_frame) <= 96);
    calls.clear();
    {
        gp::defer_frame df;
        df.gp_push(note, 1);
        df.gp_push(note_str, "two"_gs);
        df.gp_push_call([](auto&& a, auto&& b) { note(a + b); }, int64_t(1), int64_t(2));
        df.gp_push_call([](auto&&...) { note(4); });
        df.gp_push(with_result, 5); // the result is dropped
        df.gp_run();
        CHECK_EQ(calls, std::string("result 4 3 two 1 "));
        // A second run has nothing to do.
        calls.clear();
        df.gp_run();
        CHECK_EQ(calls, std::string(""));
    }
    // Many calls: past the frame's buffer, on the heap; still last in, first out.
    calls.clear();
    {
        gp::defer_frame df;
        for (int64_t i = 0; i < 1000; i++) {
            df.gp_push(note, i);
        }
        df.gp_run();
        std::string want;
        for (int64_t i = 999; i >= 0; i--) {
            want += std::to_string(static_cast<long long>(i)) + " ";
        }
        CHECK(calls == want);
    }
    // A big argument goes to the heap, an over-aligned one too; small ones after them still fit
    // the buffer.
    calls.clear();
    {
        gp::defer_frame df;
        big b;
        b.v[63] = 63;
        df.gp_push(note, 1);
        df.gp_push(note_big, b);
        b.v[63] = 0; // a copy was stored
        aligned a;
        a.v = 64;
        df.gp_push(note_aligned, a);
        df.gp_push(note, 2);
        df.gp_run();
        CHECK_EQ(calls, std::string("2 64 63 1 "));
    }
}

void test_arguments() {
    // The arguments are converted to the parameter types at the push, and stored copies are
    // destroyed after their call (or with the frame if it never runs them).
    calls.clear();
    live = 0;
    {
        gp::defer_frame df;
        counted c(7);
        df.gp_push(note_counted, c);
        df.gp_push_call([](auto&& x) { note(x.v + 1); }, c);
        CHECK(live == 3);
        df.gp_run();
        CHECK(live == 1);
    }
    CHECK(live == 0);
    CHECK_EQ(calls, std::string("8 7 "));
    {
        gp::defer_frame df;
        df.gp_push(note_counted, counted(9));
        CHECK(live == 1);
    } // never run: freed
    CHECK(live == 0);
    // nullptr to a gp::ptr parameter, a string literal to a gp::string.
    calls.clear();
    {
        gp::defer_frame df;
        df.gp_push(+[](gp::ptr<int64_t> p) { note(p == nullptr ? 1 : 0); }, nullptr);
        df.gp_run();
    }
    CHECK_EQ(calls, std::string("1 "));
    // A node that can't be built gives its room back, and the frame still works.
    calls.clear();
    copies = 0;
    {
        gp::defer_frame df;
        fragile f;
        df.gp_push(note, 1);
        bool threw = false;
        try {
            df.gp_push_call([](auto&&) { note(99); }, f);
        } catch (const std::runtime_error&) {
            threw = true;
        }
        CHECK(threw);
        df.gp_push(note, 2);
        df.gp_run();
    }
    CHECK_EQ(calls, std::string("2 1 "));
}

void test_panics() {
    // A panic of the body: every deferred call runs, then the panic goes on.
    calls.clear();
    std::string t = text([] {
        gp::defer_frame df;
        try {
            df.gp_push(note, 1);
            df.gp_push(note, 2);
            gp::panic("body"_gs);
        } catch (gp::panic_error& e) {
            df.gp_on_panic(e);
        }
        df.gp_run();
    });
    CHECK_EQ(t, std::string("body"));
    CHECK_EQ(calls, std::string("2 1 "));

    // A deferred call that panics on a normal return: the others still run, its panic goes on.
    calls.clear();
    t = text([] {
        gp::defer_frame df;
        df.gp_push(note, 1);
        df.gp_push(boom, "A"_gs);
        df.gp_push(note, 2);
        df.gp_run();
    });
    CHECK_EQ(t, std::string("A"));
    CHECK_EQ(calls, std::string("2 boom(A) 1 "));

    // The chain: first, then the deferred panics, in the order they started.
    calls.clear();
    auto chain = [] {
        gp::defer_frame df;
        try {
            df.gp_push(note, 1);
            df.gp_push(boom, "third"_gs);
            df.gp_push(boom, "second"_gs);
            gp::panic("first\nline"_gs);
        } catch (gp::panic_error& e) {
            df.gp_on_panic(e);
        }
        df.gp_run();
    };
    CHECK_EQ(
        report_of(chain),
        std::string(
            "panic: first\n\tline\n\tpanic: second\n\tpanic: third\n\ngoroutine 1 [running]:\n"));
    CHECK_EQ(calls, std::string("boom(second) boom(third) 1 "));
    // The frames: the newest panic's own (boom), a panic frame, the one before (boom), a panic
    // frame, and the first panic's stack (this test's frames).
    try {
        chain();
    } catch (const gp::panic_error& e) {
        CHECK(e.gp_prev != nullptr && e.gp_prev->gp_prev != nullptr &&
              e.gp_prev->gp_prev->gp_prev == nullptr);
        CHECK(e.gp_inner > 0 && e.gp_inner < e.gp_npcs);
        CHECK(e.gp_prev->gp_inner > 0);
        CHECK(e.gp_prev->gp_prev->gp_inner == -1);
        std::string out;
        gp::detail::panic_report(out, e);
        std::size_t p1 = out.find("\npanic(...)\n\t?:0\n");
        std::size_t p2 = p1 == std::string::npos ? p1 : out.find("\npanic(...)\n\t?:0\n", p1 + 1);
        CHECK(p1 != std::string::npos && p2 != std::string::npos);
        CHECK(out.find("panic(...)", p2 + 2) == std::string::npos);
    }

    // A panic that started in a deferred call that has defers of its own, which panic too:
    // the whole chain, oldest first.
    calls.clear();
    auto nested = [] {
        gp::defer_frame df;
        try {
            df.gp_push(+[] {
                gp::defer_frame inner;
                try {
                    inner.gp_push(boom, "C"_gs);
                    boom("B"_gs);
                } catch (gp::panic_error& e) {
                    inner.gp_on_panic(e);
                }
                inner.gp_run();
            });
            gp::panic("A"_gs);
        } catch (gp::panic_error& e) {
            df.gp_on_panic(e);
        }
        df.gp_run();
    };
    CHECK_EQ(report_of(nested),
             std::string("panic: A\n\tpanic: B\n\tpanic: C\n\ngoroutine 1 [running]:\n"));
    CHECK_EQ(calls, std::string("boom(B) boom(C) "));

    // An exception that is not a Go panic goes on at once (defer.hpp): from a deferred call,
    // the calls before it don't run and the running Go panic is dropped; from the body, no
    // deferred call runs. The frame frees the calls that didn't run (LeakSanitizer checks it).
    calls.clear();
    live = 0;
    bool cpp = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push(note, 1);
            df.gp_push(note_counted, counted(5));
            df.gp_push(throw_cpp);
            gp::panic("go"_gs);
        } catch (gp::panic_error& e) {
            df.gp_on_panic(e);
        }
        df.gp_run();
    } catch (const std::runtime_error&) {
        cpp = true;
    }
    CHECK(cpp);
    CHECK_EQ(calls, std::string("throw "));
    CHECK(live == 0);
    calls.clear();
    cpp = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push(boom, "never"_gs);
            df.gp_push(note_counted, counted(6));
            throw std::runtime_error("c++");
        } catch (gp::panic_error& e) {
            df.gp_on_panic(e);
        }
        df.gp_run();
    } catch (const std::runtime_error&) {
        cpp = true;
    }
    CHECK(cpp);
    CHECK_EQ(calls, std::string(""));
    CHECK(live == 0);
}

// test_repanicked: Go prints a panic whose value has the same identity as the next one's once,
// with " [recovered, repanicked]" (panic_identity); the cases are the ones checked with Go
// 1.25 (internal/cppgen TestPanics runs them against Go).
void test_repanicked() {
    auto two = [](auto first, auto second) {
        return report_of([&] {
            gp::defer_frame df;
            try {
                df.gp_push_call([second] { second(); });
                first();
            } catch (gp::panic_error& e) {
                df.gp_on_panic(e);
            }
            df.gp_run();
        });
    };
    const std::string tail = "\n\ngoroutine 1 [running]:\n";
    auto once = [&](const std::string& v) {
        return "panic: " + v + " [recovered, repanicked]" + tail;
    };
    auto both = [&](const std::string& a, const std::string& b) {
        return "panic: " + a + "\n\tpanic: " + b + tail;
    };
    CHECK_EQ(two([] { gp::panic_constant("x"_gs); }, [] { gp::panic_constant("x"_gs); }),
             once("x"));
    CHECK_EQ(two([] { gp::panic_constant(int64_t(5)); }, [] { gp::panic_constant(int64_t(5)); }),
             once("5"));
    CHECK_EQ(two([] { gp::panic(int64_t(7)); }, [] { gp::panic(int64_t(7)); }), once("7"));
    CHECK_EQ(two([] { gp::panic(true); }, [] { gp::panic(true); }), once("true"));
    CHECK_EQ(two([] { gp::panic(""_gs); }, [] { gp::panic(""_gs); }), once(""));
    CHECK_EQ(two([] { gp::panic(int8_t(-1)); }, [] { gp::panic(int8_t(-1)); }), once("-1"));
    CHECK_EQ(two([] { gp::panic(0.0); }, [] { gp::panic(0.0); }), once("+0.000000e+000"));
    CHECK_EQ(two([] { gp::panic_divide(); }, [] { gp::panic_divide(); }),
             once("runtime error: integer divide by zero"));
    // The nil dereference: its signal line comes after the chain.
    std::string nil2 = two([] { gp::panic_nil(); }, [] { gp::panic_nil(); });
    gp::detail::clear_signal();
    CHECK(nil2.starts_with("panic: runtime error: invalid memory address or nil pointer "
                           "dereference [recovered, repanicked]\n[signal "));
    CHECK_EQ(two([] { gp::panic(int64_t(100000)); }, [] { gp::panic(int64_t(100000)); }),
             both("100000", "100000"));
    CHECK_EQ(two([] { gp::panic("ab"_gs); }, [] { gp::panic("ab"_gs); }), both("ab", "ab"));
    CHECK_EQ(two([] { gp::panic(int64_t(-1)); }, [] { gp::panic(int64_t(-1)); }), both("-1", "-1"));
    CHECK_EQ(two([] { gp::panic_constant("x"_gs); }, [] { gp::panic("x"_gs); }), both("x", "x"));
    CHECK_EQ(two([] { gp::panic_constant(int64_t(5)); }, [] { gp::panic(int64_t(5)); }),
             both("5", "5"));
    CHECK_EQ(two([] { gp::panic(int64_t(5)); }, [] { gp::panic<6>(int64_t(5)); }), both("5", "5"));
    CHECK_EQ(two([] { gp::panic_index(5, true, 3); }, [] { gp::panic_index(5, true, 3); }),
             both("runtime error: index out of range [5] with length 3",
                  "runtime error: index out of range [5] with length 3"));
    // A, A, B: the first A once (repanicked), then B; A, B, B: A, then B once.
    auto three = [](auto a, auto b, auto c) {
        return report_of([&] {
            gp::defer_frame df;
            try {
                df.gp_push_call([c] { c(); });
                df.gp_push_call([b] { b(); });
                a();
            } catch (gp::panic_error& e) {
                df.gp_on_panic(e);
            }
            df.gp_run();
        });
    };
    auto pa = [] { gp::panic_constant("A"_gs); };
    auto pb = [] { gp::panic_constant("B"_gs); };
    CHECK_EQ(three(pa, pa, pb), "panic: A [recovered, repanicked]\n\tpanic: B" + tail);
    CHECK_EQ(three(pa, pb, pb), "panic: A\n\tpanic: B [recovered, repanicked]" + tail);
    CHECK_EQ(three(pa, pa, pa), once("A"));
}

void run_tests() {
    test_order();
    test_arguments();
    test_panics();
    test_repanicked();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmtshim::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
