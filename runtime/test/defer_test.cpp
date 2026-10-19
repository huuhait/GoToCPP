// The unit tests of gp::defer_frame (gocpp/defer.hpp) and of the panic chain (panic.hpp,
// trace.cpp's panic_chain and panic_report). runtime/cxx_test.go compiles this file with the
// runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan
// (LeakSanitizer included), and runs it like test/unit.cpp. It prints "ok: N checks" and exits
// 0, or prints each failed check and exits 1. The Go behavior (the order, the chain's text) is
// tested against real Go by the emitter's tests (internal/cppgen TestPanics, tests/run and
// tests/diff defer_*).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <exception>
#include <memory>
#include <stdexcept>
#include <string>
#include <thread>
#include <utility>
#include <vector>

#include "threads_ended.hpp"

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
        } catch (...) {
            df.gp_on_exception();
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
        } catch (...) {
            df.gp_on_exception();
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
                } catch (...) {
                    inner.gp_on_exception();
                }
                inner.gp_run();
            });
            gp::panic("A"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    };
    CHECK_EQ(report_of(nested),
             std::string("panic: A\n\tpanic: B\n\tpanic: C\n\ngoroutine 1 [running]:\n"));
    CHECK_EQ(calls, std::string("boom(B) boom(C) "));

    // A C++ exception that is not a Go panic becomes one (defer.hpp, step 5.10): the deferred
    // calls run for it, and a Go panic that was running goes into its chain. From a deferred call
    // it replaces the running panic like a panic of that call does. LeakSanitizer checks that
    // the frame frees what it holds.
    calls.clear();
    live = 0;
    std::string chained = report_of([] {
        gp::defer_frame df;
        try {
            df.gp_push(note, 1);
            df.gp_push(note_counted, counted(5));
            df.gp_push(throw_cpp);
            gp::panic("go"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(chained, std::string("panic: go\n\tpanic: C++ exception std::runtime_error: c++"
                                  "\n\ngoroutine 1 [running]:\n"));
    CHECK_EQ(calls, std::string("throw 5 1 "));
    CHECK(live == 0);
    // From the body: the deferred calls run, the first of them (the last registered) panics, and
    // that panic has the exception's before it.
    calls.clear();
    std::string body = report_of([] {
        gp::defer_frame df;
        try {
            df.gp_push(boom, "never"_gs);
            df.gp_push(note_counted, counted(6));
            throw std::runtime_error("c++");
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(body, std::string("panic: C++ exception std::runtime_error: c++\n\tpanic: never"
                               "\n\ngoroutine 1 [running]:\n"));
    CHECK_EQ(calls, std::string("6 boom(never) "));
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
            } catch (...) {
                df.gp_on_exception();
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
            } catch (...) {
                df.gp_on_exception();
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

// gp_hold keeps an owner until every deferred call ran, also those registered before the hold, and
// also when a panic runs them.
struct Tracer {
    std::vector<std::string>* log;
    std::string name;
    Tracer(std::vector<std::string>* l, std::string n) : log(l), name(std::move(n)) {}
    Tracer(const Tracer& o) : log(o.log), name(o.name) {}
    ~Tracer() {
        if (log != nullptr && !name.empty()) {
            log->push_back("end " + name);
        }
    }
};

void test_hold() {
    std::vector<std::string> log;
    {
        gp::defer_frame df;
        df.gp_push_call([&log] { log.push_back("first defer"); });
        auto box = std::make_shared<Tracer>(&log, "box");
        df.gp_hold(box);
        box.reset(); // the frame is the only holder now
        df.gp_push_call([&log] { log.push_back("second defer"); });
        CHECK(log.empty());
        df.gp_run();
        CHECK(log.size() == 3);
    }
    CHECK(log.size() == 3);
    CHECK_EQ(log[0], "second defer");
    CHECK_EQ(log[1], "first defer");
    CHECK_EQ(log[2], "end box"); // after the defer that was registered before the hold

    // A panic: the deferred calls run, then the hold goes, then the panic goes on.
    log.clear();
    bool caught = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push_call([&log] { log.push_back("defer"); });
            df.gp_hold(std::make_shared<Tracer>(&log, "held"));
            gp::panic_constant("boom"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } catch (gp::panic_error&) {
        caught = true;
    }
    CHECK(caught);
    CHECK(log.size() == 2);
    CHECK_EQ(log[0], "defer");
    CHECK_EQ(log[1], "end held");

    // Not run at all (an exception that is not a Go panic): the frame frees the hold.
    log.clear();
    try {
        gp::defer_frame df;
        df.gp_hold(std::make_shared<Tracer>(&log, "dropped"));
        throw std::runtime_error("x");
    } catch (const std::runtime_error&) {
    }
    CHECK(log.size() == 1);
}

// Holds cost O(1) each: n of them take about n times as long as one, not n squared (a loop in a
// function with a defer that stores the address of a local in each iteration makes n holds).
double hold_seconds(int n) {
    auto t0 = std::chrono::steady_clock::now();
    {
        gp::defer_frame df;
        for (int i = 0; i < n; i++) {
            df.gp_hold(std::make_shared<int>(i));
        }
        df.gp_run();
    }
    return std::chrono::duration<double>(std::chrono::steady_clock::now() - t0).count();
}

void test_hold_scales() {
    double best = 1e9;
    for (int attempt = 0; attempt < 5 && best >= 8; attempt++) {
        double small = hold_seconds(20000), large = hold_seconds(80000);
        best = std::min(best, large / std::max(small, 1e-6));
    }
    CHECK(best < 8); // 4 times the holds: about 4 times as long (quadratic would be 16)
}

// --- recover, Goexit, C++ exceptions (step 5.10) ---

// The value that the last aware function recovered, as text ("<nil>" for the nil interface).
std::string got;

std::string show_any(const gp::any& v) {
    if (v == nullptr) {
        return "<nil>";
    }
    return gp::detail::panic_text(gp::panic_error{v, gp::string(), true, {}}).gp_view().data() ==
                   nullptr
               ? std::string()
               : std::string(
                     gp::detail::panic_text(gp::panic_error{v, gp::string(), true, {}}).gp_view());
}

// How many times an aware function found the ticket still set after it took it (it must never).
int ticket_leaks = 0;

// An aware function: it takes the ticket first, like generated code (gocpp/aware.hpp).
void aware_recover() {
    gp::defer_ctx dc = gp::take_defer_ctx();
    ticket_leaks += gp::detail::ticket != nullptr ? 1 : 0;
    got = show_any(gp::recover(dc));
    calls += "recover ";
}

// An aware function that recovers twice: the second returns nil.
void aware_twice() {
    gp::defer_ctx dc = gp::take_defer_ctx();
    ticket_leaks += gp::detail::ticket != nullptr ? 1 : 0;
    std::string a = show_any(gp::recover(dc));
    std::string b = show_any(gp::recover(dc));
    got = a + "," + b;
}

// A function that is not aware (not pushed as aware) and calls an aware function: the callee
// is not called by the defer machinery, so its recover returns nil.
void indirect() {
    aware_recover();
}

// Aware, but the frame doesn't know: pushed with gp_push, it takes no ticket.
void test_recover() {
    // The deferred call recovers the body's panic: the function returns normally.
    calls.clear();
    got = "-";
    bool escaped = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push_aware(aware_recover);
            df.gp_push(note, 1);
            gp::panic("boom"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } catch (...) {
        escaped = true;
    }
    CHECK(!escaped);
    CHECK_EQ(got, std::string("boom"));
    CHECK_EQ(calls, std::string("1 recover "));

    // Not pushed as aware, or called by another function: nil, and the panic goes on.
    for (int how = 0; how < 3; how++) {
        calls.clear();
        got = "-";
        std::string t = text([how] {
            gp::defer_frame df;
            try {
                if (how == 0) {
                    df.gp_push(aware_recover); // the frame doesn't give the ticket
                } else if (how == 1) {
                    df.gp_push(indirect); // not aware itself, calls aware_recover
                } else {
                    df.gp_push_call([] { aware_recover(); });
                }
                gp::panic("boom"_gs);
            } catch (...) {
                df.gp_on_exception();
            }
            df.gp_run();
        });
        CHECK_EQ(t, std::string("boom"));
        CHECK_EQ(got, std::string("<nil>"));
    }

    // Without a panic: nil. Called by a normal return, an aware function sees no ticket.
    calls.clear();
    got = "-";
    {
        gp::defer_frame df;
        df.gp_push_aware(aware_recover);
        df.gp_run();
    }
    CHECK_EQ(got, std::string("<nil>"));

    // Twice: the second recover returns nil.
    got = "-";
    std::string again = text([] {
        gp::defer_frame df;
        try {
            df.gp_push_aware(aware_twice);
            gp::panic(int64_t(5));
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(again, std::string("<no panic>"));
    CHECK_EQ(got, std::string("5,<nil>"));

    // The deferred call after the recovering one runs as for a normal return: its aware function
    // finds no panic.
    calls.clear();
    got = "-";
    std::string after = text([] {
        gp::defer_frame df;
        try {
            df.gp_push_aware(aware_recover); // runs second: no panic any more
            df.gp_push_aware(aware_twice);   // runs first: recovers
            gp::panic("x"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(after, std::string("<no panic>"));
    CHECK_EQ(got, std::string("<nil>")); // the second call's recover

    // A func value: aware when it was made from an aware lambda, func_of_aware or a method
    // table's thunk (gp::aware, gp::func_of_aware); a plain one is not.
    got = "-";
    gp::func<void()> plain_fv = gp::func_of<&indirect>(); // calls aware_recover, not aware itself
    gp::func<void()> aware_fv = gp::func_of_aware<&aware_recover>();
    std::string by_plain = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call(plain_fv);
            gp::panic("fv"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_plain, std::string("fv"));
    CHECK_EQ(got, std::string("<nil>"));
    std::string by_aware = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call(aware_fv);
            gp::panic("fv"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_aware, std::string("<no panic>"));
    CHECK_EQ(got, std::string("fv"));
    // An aware literal.
    got = "-";
    gp::func<void()> lit_fv = gp::func<void()>(gp::aware([] {
        gp::defer_ctx dc = gp::take_defer_ctx();
        got = show_any(gp::recover(dc));
    }));
    std::string by_lit = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call(lit_fv);
            gp::panic("lit"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_lit, std::string("<no panic>"));
    CHECK_EQ(got, std::string("lit"));
    // A deferred call that is a plain lambda calling the func value: not called by the machinery.
    got = "-";
    std::string by_wrapper = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call([aware_fv] { aware_fv(); });
            gp::panic("wrap"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_wrapper, std::string("wrap"));
    CHECK_EQ(got, std::string("<nil>"));

    // The ticket never outlives the call: after a deferred aware call, a later normal call of an
    // aware function finds none, also when the call threw.
    got = "-";
    (void)text([] {
        gp::defer_frame df;
        try {
            df.gp_push_aware(+[] {
                gp::defer_ctx dc = gp::take_defer_ctx();
                (void)dc;
                gp::panic("again"_gs);
            });
            gp::panic("first"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    aware_recover();
    CHECK_EQ(got, std::string("<nil>"));
    // A deferred call that doesn't take the ticket (not aware): gp_push_aware's guard still
    // clears it, so nothing it calls can take it.
    got = "-";
    (void)text([] {
        gp::defer_frame df;
        try {
            df.gp_push_aware(+[] { aware_recover(); }); // wrongly marked aware, takes none
            gp::panic("z"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    // No ticket is left after any of the above, and no aware function found one after taking it.
    CHECK(gp::detail::ticket == nullptr);
    CHECK(ticket_leaks == 0);
}

// `defer i.M()` for an interface i: the call goes through gp::icall, which looks at the ticket
// and clears it unless the method's thunk is registered as aware (gp::method_of_aware).
//
//     type Guarder interface{ Guard() }
//     type A struct{}; func (A) Guard() { recover() }      // aware
//     type B struct{}; func (B) Guard() { helper() }       // not aware: helper calls recover

const gp::type_desc* guarder_desc();

struct Guarder : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return guarder_desc(); }
    void Guard() const { gp::icall<void()>(*this, 0); }
};

struct GuardA {
    void Guard() const {
        gp::defer_ctx dc = gp::take_defer_ctx();
        got = show_any(gp::recover(dc));
    }
    bool operator==(const GuardA&) const = default;
};
struct GuardB {
    void Guard() const { aware_recover(); }
    bool operator==(const GuardB&) const = default;
};

void thunk_GuardA(const gp::iface& self) {
    gp::unbox<GuardA>(self).Guard();
}
void thunk_GuardB(const gp::iface& self) {
    gp::unbox<GuardB>(self).Guard();
}

const gp::method gp_vm_GuardA[] = {
    gp::method_of_aware<void()>("Guard", nullptr, "func()", &thunk_GuardA)};
const gp::method gp_vm_GuardB[] = {
    gp::method_of<void()>("Guard", nullptr, "func()", &thunk_GuardB)};
const gp::imethod gp_im_Guarder[] = {gp::imethod("Guard", nullptr, "func()")};
const gp::type_desc gp_td_GuardA = gp::named<GuardA>(
    "main.GuardA", "main", gp::shape_struct(nullptr, 0), gp::methods_of(gp_vm_GuardA));
const gp::type_desc gp_td_GuardB = gp::named<GuardB>(
    "main.GuardB", "main", gp::shape_struct(nullptr, 0), gp::methods_of(gp_vm_GuardB));
const gp::type_desc gp_td_Guarder =
    gp::named<Guarder>("main.Guarder", "main", gp::shape_interface(gp_im_Guarder));
const gp::type_desc* guarder_desc() {
    return &gp_td_Guarder;
}

void test_recover_icall() {
    Guarder a = gp::to_iface<Guarder>(GuardA{}, &gp_td_GuardA);
    Guarder b = gp::to_iface<Guarder>(GuardB{}, &gp_td_GuardB);
    // The aware method recovers; the other, which only calls an aware function, doesn't.
    got = "-";
    std::string by_a = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call_icall([](const Guarder& g) { g.Guard(); }, a);
            gp::panic("iface"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_a, std::string("<no panic>"));
    CHECK_EQ(got, std::string("iface"));
    got = "-";
    std::string by_b = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call_icall([](const Guarder& g) { g.Guard(); }, b);
            gp::panic("iface"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_b, std::string("iface"));
    CHECK_EQ(got, std::string("<nil>"));
    // Called normally, or pushed without the icall mark, the aware method finds no ticket.
    got = "-";
    a.Guard();
    CHECK_EQ(got, std::string("<nil>"));
    got = "-";
    std::string plain = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call([](const Guarder& g) { g.Guard(); }, a);
            gp::panic("iface"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(plain, std::string("iface"));
    CHECK_EQ(got, std::string("<nil>"));
    // The nil interface: the call panics (gp::panic_nil), and the ticket does not stay.
    Guarder nilg;
    got = "-";
    (void)text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call_icall([](const Guarder& g) { g.Guard(); }, nilg);
            gp::panic("first"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    gp::detail::clear_signal();
    CHECK(gp::detail::ticket == nullptr); // the nil call threw before it could use the ticket
    aware_recover();
    CHECK_EQ(got, std::string("<nil>"));
    CHECK(ticket_leaks == 0);
}

// A named func type (`type H func()`) is a gp::newtype over gp::func: a deferred H is a func value
// too, aware when its code is; and a deferred func value that returns normally never reads the
// registry (a panic must be running for the ticket to matter).
struct HandlerFn : gp::newtype<HandlerFn, gp::func<void()>> {
    using gp::newtype<HandlerFn, gp::func<void()>>::newtype;
};

void test_named_func_and_registry() {
    static_assert(gp::detail::is_gp_func<gp::func<void()>>);
    static_assert(gp::detail::is_gp_func<HandlerFn>);
    static_assert(!gp::detail::is_gp_func<int>);
    HandlerFn aware_h(gp::func_of_aware<&aware_recover>());
    HandlerFn plain_h(gp::func_of<&indirect>());
    got = "-";
    std::string by_aware = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call(aware_h);
            gp::panic("named"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_aware, std::string("<no panic>"));
    CHECK_EQ(got, std::string("named"));
    got = "-";
    std::string by_plain = text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call(plain_h);
            gp::panic("named"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(by_plain, std::string("named"));
    CHECK_EQ(got, std::string("<nil>"));
    // Normal returns, with func values of every kind: no registry read.
    gp::func<void()> fv = gp::func_of_aware<&aware_recover>();
    std::size_t before = gp::detail::aware_lookups();
    for (int i = 0; i < 100; i++) {
        gp::defer_frame df;
        df.gp_push_call(fv);
        df.gp_push_call(aware_h);
        df.gp_push_call_icall([] {});
        df.gp_push_aware(aware_recover);
        df.gp_run();
    }
    CHECK(gp::detail::aware_lookups() == before);
    // While a panic runs them, it is read (once for each func value call).
    (void)text([&] {
        gp::defer_frame df;
        try {
            df.gp_push_call(fv);
            gp::panic("x"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK(gp::detail::aware_lookups() == before + 1);
}

// A deferred call that recovers and then panics again: Go prints the recovered one with
// [recovered] in the chain.
void aware_recover_panic() {
    gp::defer_ctx dc = gp::take_defer_ctx();
    (void)gp::recover(dc);
    gp::panic("second"_gs);
}

void test_recover_chain() {
    std::string r = report_of([] {
        gp::defer_frame df;
        try {
            df.gp_push_aware(aware_recover_panic);
            gp::panic("first"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(r,
             std::string("panic: first [recovered]\n\tpanic: second\n\ngoroutine 1 [running]:\n"));
    // Re-panicking the value that was recovered: once, with repanicked (the words are the same).
    std::string same = report_of([] {
        gp::defer_frame df;
        try {
            df.gp_push_aware(+[] {
                gp::defer_ctx dc = gp::take_defer_ctx();
                gp::panic(gp::recover(dc));
            });
            gp::panic(int64_t(100000)); // a value that is not in Go's static data
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK(same.starts_with("panic: 100000"));
}

// Goexit: the deferred calls run, recover() returns nil, and goexit_signal comes out of the frame.
void test_goexit() {
    calls.clear();
    got = "-";
    bool signalled = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push_aware(aware_recover);
            df.gp_push(note, 1);
            gp::goexit();
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } catch (const gp::goexit_signal&) {
        signalled = true;
    }
    CHECK(signalled);
    CHECK_EQ(calls, std::string("1 recover "));
    CHECK_EQ(got, std::string("<nil>"));

    // A panic in a deferred call during Goexit, recovered by a later call: Goexit goes on.
    calls.clear();
    signalled = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push(note, 2);
            df.gp_push_aware(aware_recover);
            df.gp_push(boom, "in goexit"_gs);
            gp::goexit();
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } catch (const gp::goexit_signal&) {
        signalled = true;
    }
    CHECK(signalled);
    CHECK_EQ(calls, std::string("boom(in goexit) recover 2 "));

    // Goexit in a deferred call replaces the panic that was running.
    calls.clear();
    signalled = false;
    bool panicked = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push(note, 3);
            df.gp_push(+[] { gp::goexit(); });
            gp::panic("dropped"_gs);
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } catch (const gp::goexit_signal&) {
        signalled = true;
    } catch (const gp::panic_error&) {
        panicked = true;
    }
    CHECK(signalled && !panicked);
    CHECK_EQ(calls, std::string("3 "));
}

// A C++ exception is a Go panic with a CppException value, which recover returns.
void test_cxx_exception() {
    calls.clear();
    got = "-";
    bool escaped = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push_aware(aware_recover);
            df.gp_push(note, 1);
            throw std::out_of_range("vec");
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } catch (...) {
        escaped = true;
    }
    CHECK(!escaped);
    CHECK_EQ(calls, std::string("1 recover "));
    CHECK_EQ(got, std::string("C++ exception std::out_of_range: vec"));
    // Anything can be thrown.
    got = "-";
    try {
        gp::defer_frame df;
        try {
            df.gp_push_aware(aware_recover);
            throw 42;
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    } catch (...) {
        escaped = true;
    }
    CHECK(!escaped);
    CHECK_EQ(got, std::string("C++ exception of type int"));
    // The value is an error with the type's name and the what() text.
    gp::any v;
    {
        gp::defer_frame df;
        try {
            df.gp_push_call_aware([&v] {
                gp::defer_ctx dc = gp::take_defer_ctx();
                v = gp::recover(dc);
            });
            throw std::runtime_error("cpp");
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    }
    CHECK(v != nullptr);
    CHECK_EQ(std::string(v.gp_type()->gp_name), std::string("runtime.CppException"));
    gp::string msg;
    CHECK(gp::detail::string_method(v, "Error", msg));
    CHECK_EQ(std::string(msg.gp_view()), std::string("C++ exception std::runtime_error: cpp"));
    // Not recovered: the next frame sees a Go panic, and the report is Go-style.
    std::string r = report_of([] {
        gp::defer_frame df;
        try {
            df.gp_push(note, 9);
            throw std::logic_error("unrecovered");
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
    });
    CHECK_EQ(r, std::string("panic: C++ exception std::logic_error: unrecovered\n\ngoroutine 1 "
                            "[running]:\n"));
}

// runtime errors are runtime.Error values with Go's types and texts.
void test_runtime_errors() {
    auto value_of = [](auto f) {
        gp::any v;
        try {
            f();
        } catch (const gp::panic_error& e) {
            v = e.gp_value;
        }
        return v;
    };
    auto name_of = [](const gp::any& v) {
        return v == nullptr ? std::string("<nil>") : std::string(v.gp_type()->gp_name);
    };
    auto err_text = [](const gp::any& v) {
        gp::string s;
        return gp::detail::string_method(v, "Error", s) ? std::string(s.gp_view()) : "<no Error>";
    };
    gp::any d = value_of([] { gp::panic_divide(); });
    CHECK_EQ(name_of(d), std::string("runtime.errorString"));
    CHECK_EQ(err_text(d), std::string("runtime error: integer divide by zero"));
    gp::any i = value_of([] { gp::panic_index(5, true, 3); });
    CHECK_EQ(name_of(i), std::string("runtime.boundsError"));
    CHECK_EQ(err_text(i), std::string("runtime error: index out of range [5] with length 3"));
    gp::any n = value_of([] { gp::panic_nil(); });
    gp::detail::clear_signal();
    CHECK_EQ(name_of(n), std::string("runtime.errorString"));
    gp::any m = value_of([] {
        throw gp::panic_error{"assignment to entry in nil map"_gs,
                              {gp::panic_identity::gp_runtime, 0, 0}};
    });
    CHECK_EQ(name_of(m), std::string("runtime.plainError"));
    CHECK_EQ(err_text(m), std::string("assignment to entry in nil map"));
    // Both have RuntimeError() (the runtime.Error interface), the string panic does not.
    auto has_method = [](const gp::any& v, const char* name) {
        for (std::size_t k = 0; k < v.gp_type()->gp_nmethods; k++) {
            if (std::string(v.gp_type()->gp_methods[k].gp_name) == name) {
                return true;
            }
        }
        return false;
    };
    CHECK(has_method(d, "RuntimeError") && has_method(i, "RuntimeError") &&
          has_method(m, "RuntimeError"));
    gp::any s = value_of([] { gp::panic("plain"_gs); });
    CHECK_EQ(name_of(s), std::string("string"));
    CHECK(s.gp_type()->gp_nmethods == 0);
    // The itab of an interface with Error() and RuntimeError() is built from the method table.
    static const gp::imethod ims[] = {gp::imethod("Error", nullptr, "func() string"),
                                      gp::imethod("RuntimeError", nullptr, "func()")};
    const gp::type_desc* rte = gp::td_interface({ims[0], ims[1]}, "runtime.Error");
    CHECK(gp::implements(rte, d.gp_type()));
    CHECK(gp::implements(rte, i.gp_type()));
    CHECK(!gp::implements(rte, s.gp_type()));
    // panic(nil) is a *runtime.PanicNilError.
    gp::any pn = value_of([] { gp::panic(nullptr); });
    CHECK_EQ(name_of(pn), std::string("*runtime.PanicNilError"));
    CHECK_EQ(err_text(pn), std::string("panic called with nil argument"));
    CHECK(gp::implements(rte, pn.gp_type()));
    // A panic with an interface value keeps it: the same type and data.
    gp::any boxed = gp::to_iface<gp::any>(int64_t(7), &gp::td_int);
    gp::any back = value_of([&] { gp::panic(boxed); });
    CHECK(back.gp_type() == boxed.gp_type() && back.gp_data().get() == boxed.gp_data().get());
    gp::any nilv;
    gp::any viaNil = value_of([&] { gp::panic(nilv); });
    CHECK_EQ(name_of(viaNil), std::string("*runtime.PanicNilError"));
    // A failed type assertion's value.
    gp::any ta = gp::detail::type_assertion_error(
        "interface conversion: interface {} is string, not int"_gs);
    CHECK_EQ(name_of(ta), std::string("*runtime.TypeAssertionError"));
    CHECK_EQ(err_text(ta), std::string("interface conversion: interface {} is string, not int"));
    CHECK(gp::implements(rte, ta.gp_type()));
}

// The text of a panic value as Go prints it (printpanicval and preprintpanics).
struct MyErr {
    gp::string gp_s;
    friend bool operator==(const MyErr& a, const MyErr& b) { return a.gp_s == b.gp_s; }
};
gp::string my_err_error(const gp::iface& self) {
    return gp::string::gp_copy("MyErr:" + std::string(gp::unbox<MyErr>(self).gp_s.gp_view()));
}
struct Opaque {
    int64_t x = 0;
    friend bool operator==(const Opaque&, const Opaque&) = default;
};
struct Celsius {
    double gp_v = 0;
    friend bool operator==(const Celsius&, const Celsius&) = default;
};
struct Label {
    gp::string gp_s;
    friend bool operator==(const Label& a, const Label& b) { return a.gp_s == b.gp_s; }
};

void test_panic_values() {
    static const gp::method my_err_methods[] = {
        gp::method_of<gp::string()>("Error", nullptr, "func() string", &my_err_error)};
    static const gp::type_desc td_my_err = gp::named<MyErr>(
        "main.MyErr", "main", gp::shape_struct(nullptr, 0), gp::methods_of(my_err_methods));
    static const gp::type_desc td_opaque_t =
        gp::named<Opaque>("main.T", "main", gp::shape_struct(nullptr, 0));
    static const gp::type_desc td_celsius =
        gp::named<Celsius>("main.Celsius", "main", gp::shape_basic(gp::kind::Float64));
    static const gp::type_desc td_label =
        gp::named<Label>("main.Label", "main", gp::shape_basic(gp::kind::String));
    auto text_of = [](const gp::any& v) {
        try {
            gp::panic(v);
        } catch (const gp::panic_error& e) {
            gp::string t = gp::detail::panic_text(e);
            return std::string(t.gp_view());
        }
        return std::string("<no panic>");
    };
    CHECK_EQ(text_of(gp::to_iface<gp::any>(MyErr{"x"_gs}, &td_my_err)), std::string("MyErr:x"));
    CHECK_EQ(text_of(gp::to_iface<gp::any>(int64_t(5), &gp::td_int)), std::string("5"));
    CHECK_EQ(text_of(gp::to_iface<gp::any>("hi"_gs, &gp::td_string)), std::string("hi"));
    CHECK_EQ(text_of(gp::to_iface<gp::any>(1.5, &gp::td_float64)), std::string("+1.500000e+000"));
    CHECK_EQ(text_of(gp::to_iface<gp::any>(Celsius{36.5}, &td_celsius)),
             std::string("main.Celsius(+3.650000e+001)"));
    CHECK_EQ(text_of(gp::to_iface<gp::any>(Label{"warm"_gs}, &td_label)),
             std::string("main.Label(\"warm\")"));
    std::string o = text_of(gp::to_iface<gp::any>(Opaque{1}, &td_opaque_t));
    CHECK(o.starts_with("(main.T) 0x"));
}

// A forced unwind (pthread_exit, pthread_cancel) is not caught: no Go code runs during it, and the
// thread ends (glibc aborts if a handler swallowed it). Linux only: Windows has no forced unwind.
#if defined(__linux__)
std::atomic<int> forced_state{0};

void forced_thread(void*) {
    {
        gp::defer_frame df;
        try {
            df.gp_push(note, 77);
            forced_state = 1;
            gp::os::exit_thread();
        } catch (...) {
            df.gp_on_exception();
        }
        df.gp_run();
        forced_state = 2; // not reached: neither the deferred calls nor the code after them run
    }
}
#endif

void test_forced_unwind() {
#if defined(__linux__)
    calls.clear();
    CHECK(gp::os::thread_start(forced_thread, nullptr, 1 << 20) == 0);
    for (int i = 0; i < 20000 && forced_state.load() == 0; i++) {
        std::this_thread::sleep_for(std::chrono::milliseconds(1));
    }
    rt_test::threads_ended(); // the thread ended through the forced unwind: glibc didn't abort
    CHECK(forced_state.load() == 1);
    CHECK_EQ(calls, std::string("")); // the deferred call did not run
#endif
}

// --- the lazy stack of a panic (panic.hpp, trace.cpp: gp_record, gp_complete) ---

// A panic made in a function with a defer frame records only the stack up to that function's
// frame; gp_complete adds the frames outside it, and the result is what a walk of the whole stack
// gives from that function's frame on.
bool lazy_stacks() {
    return gp::detail::defer_links_listed;
}

// frames_equal compares the outer parts of two stacks: the last n pcs of a and of b.
bool outer_equal(const gp::panic_error& a, const gp::panic_error& b, std::size_t n) {
    auto at = [](const gp::panic_error& e, std::size_t from_end) {
        std::size_t total = e.gp_all != nullptr ? e.gp_all->size() : std::size_t(e.gp_npcs);
        return e.gp_all != nullptr ? (*e.gp_all)[total - 1 - from_end]
                                   : e.gp_pcs[total - 1 - from_end];
    };
    for (std::size_t i = 0; i < n; i++) {
        if (at(a, i) != at(b, i)) {
            return false;
        }
    }
    return true;
}

__attribute__((noinline)) void make_lazy_and_full(std::unique_ptr<gp::panic_error>& lazy,
                                                  std::unique_ptr<gp::panic_error>& full,
                                                  int& inner) {
    gp::defer_frame df; // this function handles the panic
    lazy = std::make_unique<gp::panic_error>(gp::string::gp_copy("lazy"));
    inner = lazy->gp_npcs;
    full = std::make_unique<gp::panic_error>(gp::string::gp_copy("full"));
    full->gp_record_all();
    if (lazy_stacks()) {
        CHECK(lazy->gp_bound != 0);
        CHECK(inner > 0 && inner < 16);
        CHECK(full->gp_bound == 0);
    }
    lazy->gp_complete();
}

void test_lazy_stack() {
    CHECK(gp::detail::top_defer == nullptr);
    std::unique_ptr<gp::panic_error> lazy, full;
    int inner = 0;
    make_lazy_and_full(lazy, full, inner);
    CHECK(gp::detail::top_defer == nullptr); // the frame is gone
    CHECK(lazy->gp_bound == 0);
    if (lazy_stacks()) {
        // The outer part (this function's frame and its callers) is the same as a whole walk's.
        std::size_t outer = static_cast<std::size_t>(lazy->gp_npcs - inner);
        CHECK(outer >= 3);
        CHECK(outer_equal(*lazy, *full, outer));
        // The frame that ends the inner part is make_lazy_and_full's: the whole walk's frame there
        // is a few bytes away in the same function (another call site).
        auto a = static_cast<std::intptr_t>(lazy->gp_pcs[inner - 1]);
        auto b =
            static_cast<std::intptr_t>(full->gp_pcs[full->gp_npcs - static_cast<int>(outer) - 1]);
        CHECK(a - b < 4096 && b - a < 4096);
    } else {
        CHECK(lazy->gp_npcs - full->gp_npcs < 4 && full->gp_npcs - lazy->gp_npcs < 4);
    }
}

// A panic made in a function that has a frame, below a function that has one too, ends its inner
// part at the first of them.
__attribute__((noinline)) void inner_frame(std::unique_ptr<gp::panic_error>& e) {
    gp::defer_frame df;
    CHECK(gp::detail::top_defer != nullptr || !lazy_stacks());
    e = std::make_unique<gp::panic_error>(gp::string::gp_copy("two"));
}

__attribute__((noinline)) void outer_frame(std::unique_ptr<gp::panic_error>& e,
                                           std::unique_ptr<gp::panic_error>& again) {
    gp::defer_frame df;
    inner_frame(e);
    // The inner frame is gone: the next panic ends at this function's frame.
    again = std::make_unique<gp::panic_error>(gp::string::gp_copy("again"));
}

void test_lazy_nesting() {
    std::unique_ptr<gp::panic_error> e, again;
    outer_frame(e, again);
    CHECK(gp::detail::top_defer == nullptr);
    if (lazy_stacks()) {
        CHECK(e->gp_bound != 0 && again->gp_bound != 0);
        CHECK(again->gp_bound > e->gp_bound); // the outer function's frame is higher
    }
}

// A frame that is gone but still listed (a longjmp over it) must not end a walk: a list entry below
// the frame that makes the panic is dead. (A variable in static storage is below every frame.)
gp::detail::defer_link fake_dead_link;

void test_lazy_stale() {
    gp::detail::top_defer = &fake_dead_link;
    gp::panic_error e{gp::string::gp_copy("stale")};
    gp::detail::top_defer = nullptr;
    // The stale entry is below the current frame: ignored, the whole stack is recorded.
    CHECK(e.gp_bound == 0);
    CHECK(e.gp_npcs > 3);
}

// A deep stack: an inner part of more than gp_max_trace frames is walked to the end at once; an
// outer part of more is walked by gp_complete.
__attribute__((noinline)) void deep_inner(int n, std::unique_ptr<gp::panic_error>& e) {
    if (n == 0) {
        e = std::make_unique<gp::panic_error>(gp::string::gp_copy("deep inner"));
        return;
    }
    deep_inner(n - 1, e);
    asm volatile("" ::: "memory"); // (not a tail call)
}

__attribute__((noinline)) void deep_outer(int n, std::unique_ptr<gp::panic_error>& e,
                                          std::unique_ptr<gp::panic_error>& full) {
    if (n == 0) {
        gp::defer_frame df;
        e = std::make_unique<gp::panic_error>(gp::string::gp_copy("deep outer"));
        full = std::make_unique<gp::panic_error>(gp::string::gp_copy("full"));
        full->gp_record_all();
        e->gp_complete();
        return;
    }
    deep_outer(n - 1, e, full);
    asm volatile("" ::: "memory");
}

void test_lazy_deep() {
    std::unique_ptr<gp::panic_error> e;
    {
        gp::defer_frame df;
        deep_inner(300, e);
    }
    CHECK(e->gp_bound == 0);
    CHECK(e->gp_npcs == gp::panic_error::gp_max_trace);
    CHECK(e->gp_all != nullptr && e->gp_all->size() > 300);

    std::unique_ptr<gp::panic_error> outer, full;
    deep_outer(300, outer, full);
    CHECK(outer->gp_bound == 0);
    CHECK(outer->gp_npcs == gp::panic_error::gp_max_trace);
    CHECK(outer->gp_all != nullptr && outer->gp_all->size() > 300);
    CHECK(full->gp_all != nullptr);
    if (lazy_stacks()) {
        // The same stack but for the few frames at the top, and the first 100 are the start of it.
        CHECK(outer_equal(*outer, *full, 300));
        for (int i = 0; i < gp::panic_error::gp_max_trace; i++) {
            CHECK((*outer->gp_all)[static_cast<std::size_t>(i)] == outer->gp_pcs[i]);
        }
    }
}

// A panic that passes through the frames of several functions is completed by the first of them
// once, and its printed trace is the one of the whole stack.
__attribute__((noinline)) void throws_through(int n) {
    gp::defer_frame df;
    try {
        if (n == 0) {
            gp::panic("through"_gs);
        }
        throws_through(n - 1);
    } catch (gp::panic_error& p) {
        df.gp_take(p);
    } catch (...) {
        df.gp_adopt();
    }
    df.gp_run();
}

void test_lazy_through() {
    try {
        throws_through(4);
        CHECK(false);
    } catch (const gp::panic_error& e) {
        CHECK(e.gp_bound == 0);
        std::string out;
        gp::detail::panic_report(out, e);
        // 5 frames of throws_through (the lambda frames of the test's runner are not shown).
        std::size_t n = 0;
        for (std::size_t at = out.find("throws_through"); at != std::string::npos;
             at = out.find("throws_through", at + 1)) {
            n++;
        }
        CHECK(n == 0 || n == 5); // (a program without symbols shows addresses)
    }
}

// --- gp_panic: a panic statement in the function that has the frame ---

// A local with a destructor, to see when the goto destroys it.
struct noisy {
    ~noisy() { calls += "~noisy "; }
};

// local_or_thrown has the shape that gocpp generates for `defer ...; panic("x")`: with local, the
// panic is a gp_panic and a goto; else it is thrown, and the handler gets it.
void local_or_thrown(bool local, bool recover_it) {
    gp::defer_frame df;
    try {
        if (recover_it) {
            df.gp_push_aware(aware_recover);
        }
        df.gp_push(note, 7);
        noisy n;
        if (local) {
            df.gp_panic(gp::new_panic("x"_gs));
            goto gp_pan0;
        }
        gp::panic("x"_gs);
    } catch (gp::panic_error& gp_p) {
        df.gp_take(gp_p);
    } catch (...) {
        df.gp_adopt();
    }
gp_pan0:;
    df.gp_run();
}

// shown_names lists the functions of a report's trace (the lines without a tab).
std::vector<std::string> shown_names(const gp::panic_error& e) {
    std::string out;
    gp::detail::panic_report(out, e);
    std::vector<std::string> names;
    std::size_t at = out.find("[running]:\n");
    at = at == std::string::npos ? out.size() : at + 11;
    while (at < out.size()) {
        std::size_t end = out.find('\n', at);
        std::string line = out.substr(at, end - at);
        if (!line.empty() && line[0] != '\t') {
            names.push_back(line);
        }
        at = end == std::string::npos ? out.size() : end + 1;
    }
    return names;
}

void test_local_panic() {
    // Recovered: the function returns, the locals were destroyed before the deferred calls ran,
    // and recover() got the value.
    for (bool local : {false, true}) {
        calls.clear();
        got = "-";
        bool escaped = false;
        try {
            local_or_thrown(local, true);
        } catch (...) {
            escaped = true;
        }
        CHECK(!escaped);
        CHECK_EQ(got, std::string("x"));
        CHECK_EQ(calls, std::string("~noisy 7 recover "));
    }

    // Not recovered: the same panic leaves the function, with the same stack (the frames of the
    // function and of its callers, the same functions as for the panic that is thrown), and the
    // count of panics that goroutines are handling (detail::panic_count_add) goes back to what it
    // was.
    calls.clear();
    std::vector<std::string> names[2];
    for (int local = 0; local < 2; local++) {
        calls.clear();
        try {
            local_or_thrown(local != 0, false);
            CHECK(false);
        } catch (const gp::panic_error& e) {
            CHECK_EQ(std::string(e.gp_text.gp_view()), std::string("x"));
            CHECK_EQ(std::string(e.gp_value.gp_type()->gp_name), std::string("string"));
            CHECK(e.gp_bound == 0);
            CHECK(e.gp_npcs > 3);
            CHECK(!e.gp_recovered);
            names[local] = shown_names(e);
        }
        CHECK_EQ(calls, std::string("~noisy 7 "));
    }
    if (names[0] != names[1]) {
        std::string msg;
        for (int i = 0; i < 2; i++) {
            msg += i == 0 ? "thrown:" : "local:";
            for (auto& n : names[i]) {
                msg += " [" + n + "]";
            }
            msg += "\n";
        }
        report(false, __FILE__, __LINE__, "the traces differ:\n" + msg);
    }

    // The chain: a deferred call that panics replaces the running panic, which goes into its chain.
    calls.clear();
    auto chain = [] {
        gp::defer_frame df;
        try {
            df.gp_push(note, 1);
            df.gp_push(boom, "third"_gs);
            df.gp_push(boom, "second"_gs);
            df.gp_panic(gp::new_panic("first\nline"_gs));
            goto gp_pan0;
        } catch (gp::panic_error& gp_p) {
            df.gp_take(gp_p);
        } catch (...) {
            df.gp_adopt();
        }
    gp_pan0:;
        df.gp_run();
    };
    CHECK_EQ(
        report_of(chain),
        std::string(
            "panic: first\n\tline\n\tpanic: second\n\tpanic: third\n\ngoroutine 1 [running]:\n"));
    CHECK_EQ(calls, std::string("boom(second) boom(third) 1 "));
    try {
        chain();
    } catch (const gp::panic_error& e) {
        CHECK(e.gp_prev != nullptr && e.gp_prev->gp_prev != nullptr &&
              e.gp_prev->gp_prev->gp_prev == nullptr);
        CHECK(e.gp_inner > 0 && e.gp_inner < e.gp_npcs);
        CHECK(e.gp_prev->gp_inner > 0);
        CHECK(e.gp_bound == 0 && e.gp_prev->gp_bound == 0 && e.gp_prev->gp_prev->gp_bound == 0);
    }

    // The panic of the frame that is replaced (a deferred call that recovers and panics again),
    // and a frame that is destroyed with its panic: nothing leaks (LeakSanitizer in the asan mode).
    calls.clear();
    got = "-";
    std::string again = text([] {
        gp::defer_frame df;
        try {
            df.gp_push(boom, "again"_gs);
            df.gp_push_aware(aware_recover);
            df.gp_panic(gp::new_panic(int64_t(3)));
            goto gp_pan0;
        } catch (gp::panic_error& gp_p) {
            df.gp_take(gp_p);
        } catch (...) {
            df.gp_adopt();
        }
    gp_pan0:;
        df.gp_run();
    });
    CHECK_EQ(again, std::string("again"));
    CHECK_EQ(got, std::string("3"));
    {
        gp::defer_frame df;
        df.gp_panic(gp::new_panic(true)); // (never run: the frame ends with it)
    }

    // Goexit in a deferred call replaces the panic (and frees it).
    bool signalled = false;
    try {
        gp::defer_frame df;
        try {
            df.gp_push(+[] { gp::goexit(); });
            df.gp_panic(gp::new_panic(int64_t(9)));
            goto gp_pan0;
        } catch (gp::panic_error& gp_p) {
            df.gp_take(gp_p);
        } catch (...) {
            df.gp_adopt();
        }
    gp_pan0:;
        df.gp_run();
    } catch (const gp::goexit_signal&) {
        signalled = true;
    } catch (const gp::panic_error&) {
    }
    CHECK(signalled);

    // The values: the same as the thrown ones'.
    {
        auto v = [](std::unique_ptr<gp::panic_error> p) {
            return std::string(p->gp_text.gp_view()) + "|" +
                   std::string(p->gp_value.gp_type()->gp_name) + "|" +
                   std::to_string(p->gp_id.gp_kind) + "|" + std::to_string(p->gp_id.gp_type);
        };
        auto t = [](auto f) {
            try {
                f();
            } catch (const gp::panic_error& e) {
                return std::string(e.gp_text.gp_view()) + "|" +
                       std::string(e.gp_value.gp_type()->gp_name) + "|" +
                       std::to_string(e.gp_id.gp_kind) + "|" + std::to_string(e.gp_id.gp_type);
            }
            return std::string("<none>");
        };
        CHECK_EQ(v(gp::new_panic(int64_t(5))), t([] { gp::panic(int64_t(5)); }));
        CHECK_EQ(v(gp::new_panic(int64_t(500))), t([] { gp::panic(int64_t(500)); }));
        CHECK_EQ(v(gp::new_panic<6>(int64_t(5))), t([] { gp::panic<6>(int64_t(5)); }));
        CHECK_EQ(v(gp::new_panic(1.5)), t([] { gp::panic(1.5); }));
        CHECK_EQ(v(gp::new_panic(true)), t([] { gp::panic(true); }));
        CHECK_EQ(v(gp::new_panic(uint8_t(7))), t([] { gp::panic(uint8_t(7)); }));
        CHECK_EQ(v(gp::new_panic_constant(int64_t(5))), t([] { gp::panic_constant(int64_t(5)); }));
        CHECK_EQ(v(gp::new_panic("s"_gs)), t([] { gp::panic("s"_gs); }));
        CHECK_EQ(v(gp::new_panic(""_gs)), t([] { gp::panic(""_gs); }));
        CHECK_EQ(v(gp::new_panic_constant("s"_gs)), t([] { gp::panic_constant("s"_gs); }));
        CHECK_EQ(v(gp::new_panic(nullptr)), t([] { gp::panic(nullptr); }));
        gp::error nil_err;
        CHECK_EQ(v(gp::new_panic(nil_err)), t([] { gp::panic(gp::error()); }));
        gp::complex128 c(1, 2);
        CHECK_EQ(v(gp::new_panic(c)), t([] { gp::panic(gp::complex128(1, 2)); }));
    }
}

// --- fatal_trace in a deferred call that a panic runs ---

std::string fatal_out;

void fatal_in_defer() {
    fatal_out.clear();
    gp::detail::fatal_trace(fatal_out);
}

// fatal_shape_ok checks the trace that a fatal error in a deferred call prints, Go's shape: the
// deferred call's frames, a panic frame, and the whole stack of the panic that was running
// (down to run_tests: nothing is left out for the frames that the panic had not recorded yet).
bool fatal_shape_ok(const std::string& t) {
    return t.find("\npanic(...)\n") != std::string::npos &&
           t.find("run_tests") != std::string::npos && t.find("elided") == std::string::npos;
}

void test_fatal_in_defer() {
    for (bool local : {false, true}) {
        fatal_out.clear();
        try {
            gp::defer_frame df;
            try {
                df.gp_push(fatal_in_defer);
                if (local) {
                    df.gp_panic(gp::new_panic("p"_gs));
                    goto gp_pan0;
                }
                gp::panic("p"_gs);
            } catch (gp::panic_error& gp_p) {
                df.gp_take(gp_p);
            } catch (...) {
                df.gp_adopt();
            }
        gp_pan0:;
            df.gp_run();
        } catch (const gp::panic_error&) {
        }
        bool ok = fatal_shape_ok("\n" + fatal_out);
        if (!ok) {
            report(false, __FILE__, __LINE__,
                   "the trace of a fatal error in a deferred call:\n" + fatal_out);
        }
    }
}

void run_tests() {
    test_recover();
    test_recover_icall();
    test_named_func_and_registry();
    test_recover_chain();
    test_goexit();
    test_cxx_exception();
    test_lazy_stack();
    test_lazy_nesting();
    test_lazy_stale();
    test_lazy_deep();
    test_lazy_through();
    test_local_panic();
    test_fatal_in_defer();
    test_runtime_errors();
    test_panic_values();
    test_forced_unwind();
    test_hold_scales();
    test_hold();
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
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

void no_init() {}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
