// The unit tests of the stack trace of a panic (gocpp/trace.hpp): Go names from mangled C++
// names, the DWARF line table, and the trace of this program's own panics, with Go names and
// the lines of this file. runtime/cxx_test.go compiles it with the runtime's .cpp files, with
// g++ and clang++, at -O0, at -O2 and with ASan + UBSan (that mode with DWARF 4, the others
// with the compilers' DWARF 5), and runs it like test/unit.cpp: it prints "ok: N checks" and
// exits 0, or prints each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>

#include <cstdint>
#include <string>
#include <string_view>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

std::string show(int64_t v) {
    return std::to_string(static_cast<long long>(v));
}
std::string show(bool v) {
    return v ? "true" : "false";
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

volatile int64_t sink = 0;

} // namespace

// Functions like the ones gocpp generates for package main, for the trace of a real panic.
namespace gpkg_main {

int64_t inner_line = 0;
int64_t outer_line = 0;

[[gnu::noinline]] void inner(int64_t i) {
    inner_line = __LINE__ + 1;
    gp::check_index(i, 3);
    sink = sink + 1;
}

[[gnu::noinline]] void outer() {
    outer_line = __LINE__ + 1;
    inner(sink + 5);
    sink = sink + 1; // not a tail call, so the frame stays at -O2
}

[[gnu::noinline]] void deep(int64_t n) {
    if (n == 0) {
        if (sink >= 0) {
            gp::panic("deep"_gs);
        }
        return;
    }
    deep(n - 1);
    sink = sink + 1;
}

} // namespace gpkg_main

namespace {

// --- func_name ---

struct name_case {
    const char* symbol;
    const char* name; // nullptr: not shown
    bool params;
    bool raw;
};

// The symbols are what g++ 15 and clang++ 21 make of the shapes of gocpp's generated code
// (nm of a test file), and the runtime's and the C++ library's frames.
const name_case name_cases[] = {
    {"_ZN9gpkg_main4mainEv", "main.main", false, false},
    {"_ZN9gpkg_main1gEv", "main.g", false, false},
    {"_ZN9gpkg_main1fEN2gp6stringEl", "main.f", true, false},
    {"_ZN9gpkg_main1fEN2gp6stringEl.cold", "main.f", true, false},
    {"_ZN9gpkg_main1fEN2gp6stringEl.constprop.0", "main.f", true, false},
    {"_ZN9gpkg_main1hEN2gp5sliceINS0_6stringEEENS0_5arrayIlLl3EEERKNS_1TEPS6_NSt7__"
     "cxx1112basic_stringIcSt11char_traitsIcESaIcEEE",
     "main.h", true, false},
    // Methods (step 3.10): a value method is a const member function, a pointer method a
    // static one; the receiver is a parameter in Go.
    {"_ZNK9gpkg_main1T1KEv", "main.T.K", true, false},
    {"_ZNK9gpkg_main1T1MEl", "main.T.M", true, false},
    {"_ZN9gpkg_main1T1MEv", "main.(*T).M", false, false},
    {"_ZN9gpkg_main1T2PMEN2gp3ptrIS0_EE", "main.(*T).PM", true, false},
    {"_ZN9gpkg_main7Celsius6StringEN2gp3ptrIS0_EE", "main.(*Celsius).String", true, false},
    {"_ZNK9gpkg_main7Celsius6StringEv", "main.Celsius.String", true, false},
    {"_ZZNK9gpkg_main1T1MEvENKUlvE_clEv", "main.T.M.func1", false, false},
    {"_ZZN9gpkg_main1T2PMEN2gp3ptrIS0_EEENKUlvE_clEv", "main.(*T).PM.func1", false, false},
    {"_ZNK9gpkg_main1T9gp_StringEv", nullptr, false, false}, // (a name that fmt's hooks had)
    {"_ZN9gpkg_main1T13gp_String_ptrERKN2gp3ptrIS0_EE", nullptr, false, false},
    {"_ZN9gpkg_main3MapIlEEvT_", "main.Map[...]", true, false},
    {"_ZN9gpkg_main5gp_x_Ev", "main.gp_x", false, false}, // a user's gp_x
    {"_ZN9gpkg_main4new_Ev", "main.new_", false, false},  // Go's new: the _ stays
    {"_ZN9gpkg_main9gp_init_1Ev", "main.init.0", false, false},
    {"_ZN9gpkg_main10gp_init_12Ev", "main.init.11", false, false},
    {"_ZN9gpkg_main11gp_var_initEv", "main.init", false, false},
    {"_ZN9gpkg_main13gp_init_funcsEv", nullptr, false, false},
    {"_ZN9gpkg_main28gp_init_99999999999999999999Ev", nullptr, false, false}, // not gocpp's
    {"_ZN9gpkg_main9gp_init_0Ev", nullptr, false, false},
    {"_ZN9gpkg_main5gp_t0Ev", nullptr, false, false},
    {"_Z11gp_init_allv", nullptr, false, false},
    {"_ZN39gpkg_github_dcom__huuhait__game__engine4StepEd", "github.com/huuhait/game/engine.Step",
     true, false},
    {"_ZN12gpkg_a_hb_ux1FEv", "a-b_x.F", false, false},
    {"_ZN14gpkg_a_t_p_x2a1FEv", "a~+*.F", false, false},
    // Lambdas: g++ numbers them in their function (#1 is UlvE_, #2 UlvE0_), as clang++ does
    // inside a lambda; clang++ numbers those of a function in the whole file ($_0), so their
    // Go number is unknown.
    {"_ZZN9gpkg_main4mainEvENKUlvE_clEv", "main.main.func1", false, false},
    {"_ZZN9gpkg_main4mainEvENKUllE_clEl", "main.main.func1", true, false},
    {"_ZZN9gpkg_main4mainEvENKUlvE1_clEv", "main.main.func3", false, false},
    {"_ZZN9gpkg_main11gp_var_initEvENKUlvE_clEv", "main.init.func1", false, false},
    {"_ZZZN9gpkg_main4mainEvENKUllE_clElENKUlvE0_clEv", "main.main.func1.2", false, false},
    {"_ZZN9gpkg_main4mainEvENK3$_1clEv", "main.main.func?", false, false},
    {"_ZZZN9gpkg_main4mainEvENK3$_2clElENKUlvE0_clEv", "main.main.func?.2", false, false},
    // Generic lambdas are gocpp's helpers (std::apply's callable), and lambdas of the runtime
    // are the runtime's.
    {"_ZZN9gpkg_main4mainEvENKUlDpRKT_E_clIJlEEEDaS3_", nullptr, false, false},
    {"_ZZN9gpkg_main4mainEvENK3$_0clIJlEEEDaDpRKT_", nullptr, false, false},
    {"_ZZN2gp8run_mainEiPPcPFvvES3_ENKUlvE_clEv", nullptr, false, false},
    // The runtime and the C++ library.
    {"_ZN2gp11panic_indexElbl", nullptr, false, false},
    {"_ZN2gp11check_indexIlEEvT_l", nullptr, false, false},
    {"_ZNK2gp5sliceIlEixIlEERlT_", nullptr, false, false},
    {"_ZN2gp11panic_errorC2ENS_6stringE", nullptr, false, false},
    {"_ZNSt10_Head_baseILm0ElLb0EE7_M_headERS0_", nullptr, false, false},
    {"_ZSt5applyIZN9gpkg_main4mainEvEUlDpRKT_E_St5tupleIJlEEEDcOT_OT0_", nullptr, false, false},
    {"_ZN9__gnu_cxx13new_allocatorIcE8allocateEmPKv", nullptr, false, false},
    {"__cxa_throw", nullptr, false, false},
    {"_Unwind_RaiseException", nullptr, false, false},
    {"__libc_start_main", nullptr, false, false},
    {"_start", nullptr, false, false},
    // The wrapper method of an interface struct (internal/lower/ifaces.go) is tagged gp_wrap: Go
    // has no frame for it.
    {"_ZNK9gpkg_main5Shape4AreaB7gp_wrapEv", nullptr, false, false},
    {"_ZNK9gpkg_main5Shape4AreaB7gp_wrapEv.cold", nullptr, false, false},
    {"_ZNK9gpkg_main5ShapeB7gp_wrap4AreaEv", nullptr, false, false}, // the tag after the type
    // A user method that is called B7gp_wrap is a method (the text is inside a name, not a tag).
    {"_ZNK9gpkg_main5Shape9B7gp_wrapEv", "main.Shape.B7gp_wrap", true, false},
    {"_ZN9gpkg_main9B7gp_wrapEv", "main.B7gp_wrap", false, false},
    // Other C++ (the user's own, Phase 04) and C: the C++ name, or the C name.
    {"_ZN4user3cxxEi", "user::cxx(int)", true, true},
    {"_ZN7gpkg_a_1fEv", "gpkg_a_::f()", false, true}, // not an escaped import path
    {"cfunc", "cfunc", true, true},
};

void test_func_name() {
    for (const name_case& c : name_cases) {
        gp::detail::go_func f = gp::detail::func_name(c.symbol);
        std::string what = std::string(c.symbol) + ": ";
        report(f.shown == (c.name != nullptr), __FILE__, __LINE__,
               what + "shown = " + show(f.shown));
        if (c.name != nullptr) {
            report(f.name == c.name, __FILE__, __LINE__,
                   what + "name = " + show(f.name) + ", want " + show(std::string(c.name)));
            report(f.params == c.params, __FILE__, __LINE__, what + "params = " + show(f.params));
            report(f.raw == c.raw, __FILE__, __LINE__, what + "raw = " + show(f.raw));
        }
    }
    // Broken or odd names never crash: a truncated one is shown as it is.
    for (const char* s : {"_Z", "_ZN", "_ZN9gpkg_main", "_ZN9gpkg_main1f", "_ZZ", "_ZZN9gpkg_main4",
                          "_ZN99999999999999999999x", "_ZN9gpkg_main1fEN", "", "_ZIIIIIIII"}) {
        gp::detail::go_func f = gp::detail::func_name(s);
        CHECK(!f.shown || !f.name.empty());
    }
    // Deep nesting stops at the parser's depth limit, without deep recursion: template argument
    // packs, nested types, local names.
    for (std::string deep :
         {"_ZN1aI" + std::string(1000000, 'J'), "_ZN1aI" + std::string(1000000, 'P'),
          "_Z" + std::string(1000000, 'Z'), "_ZN1aI" + std::string(1000000, 'N')}) {
        gp::detail::go_func f = gp::detail::func_name(deep);
        CHECK(!f.shown || !f.name.empty());
    }
}

// --- the trace of a real panic ---

// panic_of runs f and returns the whole report of its panic, as run_main prints it.
template <typename F>
std::string panic_of(F f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        std::string out;
        gp::detail::panic_report(out, e);
        return out;
    }
    return "<no panic>";
}

// shape is the form of a trace: how many frames it shows, the count of its "...N frames
// elided..." line (-1 without one), how many frames come before that line, and the name line
// of the last frame.
struct shape {
    std::size_t frames = 0;
    int64_t elided = -1;
    std::size_t before_elided = 0;
    std::string last;
};

shape shape_of(const std::string& out) {
    shape sh;
    std::string mark = "goroutine 1 [running]:\n";
    std::size_t pos = out.find(mark);
    if (pos == std::string::npos) {
        return sh;
    }
    pos += mark.size();
    while (pos < out.size()) {
        std::size_t end = out.find('\n', pos);
        std::string line = out.substr(pos, end - pos);
        pos = end == std::string::npos ? out.size() : end + 1;
        if (line == "...additional frames elided...") {
            continue;
        }
        if (line.starts_with("...") && line.ends_with(" frames elided...")) {
            sh.elided = std::stoll(line.substr(3));
            sh.before_elided = sh.frames;
        } else if (!line.empty() && line[0] != '\t') {
            sh.frames++;
            sh.last = line;
        }
    }
    return sh;
}

void test_trace() {
    gp::detail::clear_signal();
    std::string out = panic_of([] { gpkg_main::outer(); });
    std::string file = __FILE__;
    std::string inner =
        "main.inner(...)\n\t" + file + ":" + std::to_string(gpkg_main::inner_line) + " +0x";
    std::string outer =
        "main.outer()\n\t" + file + ":" + std::to_string(gpkg_main::outer_line) + " +0x";
    // The innermost frame is the user's function: the runtime's frames (gp::check_index,
    // gp::panic_index, the panic_error constructor, gp::os::stack) are left out.
    std::string head = "panic: runtime error: index out of range [5] with length 3\n\n"
                       "goroutine 1 [running]:\n";
    std::size_t i = out.find(inner), o = out.find(outer);
#ifdef __OPTIMIZE__
    // Optimized, the call in inner is inlined code of checks.hpp, on its line there: only the
    // names are sure (gocpp builds at -O0).
    inner = inner.substr(0, inner.find('\t'));
    outer = outer.substr(0, outer.find('\t'));
    i = out.find(inner);
    o = out.find(outer);
#endif
    CHECK_EQ(out.substr(0, head.size() + inner.size()), head + inner);
    report(i != std::string::npos && o != std::string::npos && i < o, __FILE__, __LINE__,
           "the trace has no " + show(inner) + " before " + show(outer) + ":\n" + out);
    // Up to outer's frame (the test's own C++ frames come after it, with C++ names).
    std::string top = out.substr(0, o);
    CHECK(top.find("gp::") == std::string::npos);
    CHECK(top.find("std::") == std::string::npos);
    CHECK(out.find("\nmain(") == std::string::npos); // C++'s main ends the trace
    CHECK(out.ends_with("\n"));
    CHECK(out.find("elided") == std::string::npos);

    // A deep stack, like Go 1.25: the inner 50 frames, how many are left out, the outer 50.
    std::string d = panic_of([] { gpkg_main::deep(300); });
    CHECK(d.starts_with("panic: deep\n\ngoroutine 1 [running]:\nmain.deep(...)\n\t" + file + ":"));
    shape sh = shape_of(d);
    CHECK_EQ(int64_t(sh.frames), int64_t(100));
    CHECK_EQ(int64_t(sh.before_elided), int64_t(50));
    // 301 frames of deep and the test's own (the lambda, panic_of, test_trace, run_tests).
    report(sh.elided >= 201 && sh.elided <= 210, __FILE__, __LINE__,
           "the deep trace elides " + std::to_string(sh.elided) + " frames:\n" + d);
    // The outer frames end with the outermost one that the trace shows, as Go's end with
    // main.main.
    CHECK(sh.last.find("run_tests") != std::string::npos);
    CHECK(d.find("additional frames") == std::string::npos);
    try {
        gpkg_main::deep(300);
    } catch (const gp::panic_error& e) {
        // The second walk recorded the whole stack.
        CHECK(e.gp_npcs == gp::panic_error::gp_max_trace);
        CHECK(e.gp_all != nullptr && e.gp_all->size() > 300);
        // Only the top of a stack (the second walk failed): the inner 50, then Go's old line.
        std::string top;
        gp::detail::append_trace(top, e.gp_pcs, static_cast<std::size_t>(e.gp_npcs), false);
        shape st = shape_of("goroutine 1 [running]:\n" + top);
        CHECK_EQ(int64_t(st.frames), int64_t(50));
        CHECK(top.ends_with("\n...additional frames elided...\n"));
        CHECK(top.find("frames elided...\n") == top.find("additional frames elided...\n") + 11);
        // No frame that the trace shows (only the runtime's: os::stack, the constructor,
        // gp::panic): the addresses, as for a stripped program.
        std::string raw;
        gp::detail::append_trace(raw, e.gp_pcs, 3, true);
        CHECK(raw.starts_with("?()\n\t?:0 pc=0x"));
    }
#ifndef __OPTIMIZE__
    // Exactly 100 frames (96 of deep, and the test's 4): all of them, nothing elided; 101: one.
    std::string d100 = panic_of([] { gpkg_main::deep(95); });
    shape s100 = shape_of(d100);
    CHECK_EQ(int64_t(s100.frames), int64_t(100));
    CHECK_EQ(int64_t(s100.elided), int64_t(-1));
    std::string d101 = panic_of([] { gpkg_main::deep(96); });
    shape s101 = shape_of(d101);
    CHECK_EQ(int64_t(s101.frames), int64_t(100));
    CHECK_EQ(int64_t(s101.elided), int64_t(1));
    CHECK(d101.find("\n...1 frames elided...\n") != std::string::npos);
#endif
}

// --- the DWARF line table ---

void test_lines() {
    gp::detail::exe_image img;
    CHECK(gp::os::read_exe(img));
    CHECK(!img.funcs.empty());
    CHECK(!img.debug_line.empty());
    // An address that no code has.
    gp::detail::line_query q;
    q.addr = 1;
    gp::detail::lines_of(img.debug_line, img.debug_line_str, img.debug_str, &q, 1);
    CHECK_EQ(q.line, int64_t(0));
    // A broken table never makes it read outside the sections (ASan checks) or loop: cut short
    // anywhere, or with bytes changed.
    std::string_view line = img.debug_line;
    std::size_t step = line.size() / 97 + 1;
    for (std::size_t n = 0; n < line.size(); n += step) {
        gp::detail::line_query r;
        r.addr = img.funcs[img.funcs.size() / 2].addr;
        gp::detail::lines_of(line.substr(0, n), img.debug_line_str, img.debug_str, &r, 1);
        std::string bad(line);
        for (std::size_t k = n; k < bad.size() && k < n + 8; k++) {
            bad[k] = static_cast<char>(bad[k] ^ (0x5a + static_cast<int>(k % 7)));
        }
        gp::detail::lines_of(bad, img.debug_line_str.substr(0, n % 64), "", &r, 1);
    }
    CHECK(true); // reached the end
}

void no_init() {}

void run_tests() {
    test_func_name();
    test_trace();
    test_lines();
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
