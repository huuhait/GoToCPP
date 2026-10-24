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
#include <memory>
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
    // Instances of generic functions (internal/lower/generics.go): the hash is 8, 10 or 12 hex
    // digits.
    {"_ZN9gpkg_main20gp_inst_0123abcd_MapEv", "main.Map[...]", false, false},
    {"_ZN9gpkg_main22gp_inst_0123abcdef_MapEv", "main.Map[...]", false, false},
    {"_ZN9gpkg_main24gp_inst_0123abcdef01_MapEv", "main.Map[...]", false, false},
    // Methods of instances of generic types: main.(*Stack[...]).Pop, main.Stack[...].Peek.
    {"_ZN9gpkg_main22gp_inst_0123abcd_Stack3PopEN2gp3ptrIS0_EE", "main.(*Stack[...]).Pop", true,
     false},
    {"_ZNK9gpkg_main22gp_inst_0123abcd_Stack4PeekEv", "main.Stack[...].Peek", true, false},
    {"_ZN9gpkg_main21gp_inst_0123abcde_MapEv", nullptr, false, false}, // 9 digits: not an instance
    {"_ZN9gpkg_main20gp_inst_0123abcg_MapEv", nullptr, false, false},
    {"_ZZN9gpkg_main20gp_inst_0123abcd_MapEvENKUlvE_clEv", "main.Map[...].func1", false, false},
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
    report(sh.last.find("run_tests") != std::string::npos, __FILE__, __LINE__,
           "the deep trace ends with " + sh.last + ":\n" + d);
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
        // gp::panic): none, in a program with its symbols and its line table (as Go hides its
        // runtime's frames); without the line table, the addresses, as for a stripped program.
        std::string raw;
        gp::detail::append_trace(raw, e.gp_pcs, 3, true);
        CHECK_EQ(raw, std::string());
        gp::detail::exe_image img;
        CHECK(gp::os::read_exe(img));
        img.debug_line.clear();
        std::string addrs;
        gp::detail::append_trace(addrs, e.gp_pcs, 3, true, &img);
        CHECK(addrs.starts_with("?()\n\t?:0 pc=0x"));
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

    // indexed_lines_of (the index of the whole table, read once) answers as lines_of does, for the
    // first, the middle and the last address of many functions, and again when asked a second time.
    std::vector<gp::detail::line_query> asked;
    for (std::size_t k = 0; k < img.funcs.size(); k += 3) {
        const auto& f = img.funcs[k];
        for (uint64_t off :
             {uint64_t(0), uint64_t(f.size / 2), uint64_t(f.size > 0 ? f.size - 1 : 0)}) {
            gp::detail::line_query a;
            a.addr = f.addr + off;
            asked.push_back(a);
        }
    }
    std::vector<gp::detail::line_query> plain = asked, indexed = asked, again = asked;
    gp::detail::lines_of(img.debug_line, img.debug_line_str, img.debug_str, plain.data(),
                         plain.size());
    gp::detail::indexed_lines_of(img.debug_line, img.debug_line_str, img.debug_str, indexed.data(),
                                 indexed.size());
    gp::detail::indexed_lines_of(img.debug_line, img.debug_line_str, img.debug_str, again.data(),
                                 again.size());
    int64_t found = 0;
    for (std::size_t k = 0; k < asked.size(); k++) {
        CHECK_EQ(indexed[k].line, plain[k].line);
        CHECK(indexed[k].file == plain[k].file);
        CHECK_EQ(again[k].line, plain[k].line);
        found += plain[k].line != 0 ? 1 : 0;
    }
    CHECK(found > 100);
    gp::detail::line_query nothing;
    nothing.addr = 1;
    gp::detail::indexed_lines_of(img.debug_line, img.debug_line_str, img.debug_str, &nothing, 1);
    CHECK_EQ(nothing.line, int64_t(0));
}

// The file names of a line table written by hand (DWARF 4): a native Windows path, as clang writes
// it for a COFF target. A Windows program's trace shows it with slashes, as Go does (C:/work/...);
// on Linux a backslash is an ordinary character of a name and the path stays as it is.
void test_line_paths() {
    auto le = [](std::string& out, uint64_t v, int bytes) {
        for (int i = 0; i < bytes; i++) {
            out += static_cast<char>((v >> (8 * i)) & 0xff);
        }
    };
    const std::string file = "C:\\work\\sub\\hello.gp";
    std::string rest; // after header_length
    rest += '\1';     // minimum_instruction_length
    rest += '\1';     // maximum_operations_per_instruction
    rest += '\1';     // default_is_stmt
    rest += '\xfb';   // line_base -5
    rest += '\x0e';   // line_range 14
    rest += '\x0d';   // opcode_base 13
    for (char len : {0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1}) {
        rest += len;
    }
    rest += '\0'; // no include directories
    rest += file;
    rest += '\0';
    rest += std::string(3, '\0'); // directory 0, no time, no size
    rest += '\0';                 // end of the file names
    std::string program;
    program += std::string("\0\x09\x02", 3); // set_address
    le(program, 0x1000, 8);
    program += '\x03'; // advance_line
    program += '\x05'; // +5: line 6
    program += '\x01'; // copy
    program += '\x02'; // advance_pc
    program += '\x04';
    program += std::string("\0\1\1", 3); // end_sequence
    std::string unit;
    le(unit, 4, 2); // version
    le(unit, rest.size(), 4);
    unit += rest;
    unit += program;
    std::string table;
    le(table, unit.size(), 4);
    table += unit;

    gp::detail::line_query q;
    q.addr = 0x1002;
    gp::detail::lines_of(table, "", "", &q, 1);
    CHECK_EQ(q.line, int64_t(6));
#if defined(_WIN32)
    CHECK(q.file == "C:/work/sub/hello.gp");
#else
    CHECK(q.file == file);
#endif
}

// --- the reader of the inlined calls (read_inline_frames), on DWARF written by hand ---

// The bytes of a little-endian number, a ULEB128 and a C string.
void put_le(std::string& out, uint64_t v, int bytes) {
    for (int i = 0; i < bytes; i++) {
        out += static_cast<char>((v >> (8 * i)) & 0xff);
    }
}
void put_uleb(std::string& out, uint64_t v) {
    do {
        unsigned char b = v & 0x7f;
        v >>= 7;
        out += static_cast<char>(b | (v != 0 ? 0x80 : 0));
    } while (v != 0);
}
void put_str(std::string& out, std::string_view s) {
    out += s;
    out += '\0';
}

// A DWARF 4 line table with no program: only the file names that DW_AT_call_file numbers (1: the
// first).
std::string call_files() {
    std::string rest;
    rest += std::string("\1\1\1\xfb\x0e\x0d", 6);
    for (char len : {0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1}) {
        rest += len;
    }
    rest += '\0'; // no include directories
    put_str(rest, "/w/main.gp");
    rest += std::string(3, '\0');
    put_str(rest, "/w/gocpp/checks.hpp");
    rest += std::string(3, '\0');
    rest += '\0';
    std::string unit;
    put_le(unit, 4, 2);
    put_le(unit, rest.size(), 4);
    unit += rest;
    std::string table;
    put_le(table, unit.size(), 4);
    return table + unit;
}

// The knobs of a DWARF 5 unit for the tests: main.get is inlined into a function at [0x1000,
// 0x1100) over [0x1008, 0x1020), called at main.gp:25, and main.sum is inlined into it over
// [0x1010, 0x1018) (a range list), called at main.gp:9.
struct dwarf5 {
    uint64_t name_strx = 0;     // DW_FORM_strx1 index of main.get's linkage name
    uint64_t low_addrx = 1;     // DW_FORM_addrx index of the first call's low_pc
    uint64_t rnglistx = 0;      // DW_FORM_rnglistx index of the second call's ranges
    uint64_t addr_base = 8;     // DW_AT_addr_base
    uint64_t form_override = 0; // a form for main.get's call_line (0: data1)
    bool cyclic = false;        // main.sum's DIE is its own abstract origin, with no name
    bool get_origin = true;     // the call of main.get has its DW_AT_abstract_origin (MinGW: none)
};

// make5 writes the sections of a dwarf5 unit into img.
void make5(gp::detail::exe_image& img, const dwarf5& k) {
    // .debug_abbrev
    std::string ab;
    auto attr = [&](uint64_t at, uint64_t form) {
        put_uleb(ab, at);
        put_uleb(ab, form);
    };
    put_uleb(ab, 1); // the unit
    put_uleb(ab, 0x11);
    ab += '\1';
    attr(0x10, 0x17); // stmt_list sec_offset
    attr(0x11, 0x01); // low_pc addr
    attr(0x72, 0x17); // str_offsets_base
    attr(0x73, 0x17); // addr_base
    attr(0x74, 0x17); // rnglists_base
    attr(0, 0);
    put_uleb(ab, 2); // main.get: subprogram, linkage_name strx1
    put_uleb(ab, 0x2e);
    ab += '\0';
    attr(0x6e, 0x25);
    attr(0, 0);
    put_uleb(ab, 3); // main.sum: subprogram, linkage_name strp (or its own origin)
    put_uleb(ab, 0x2e);
    ab += '\0';
    if (k.cyclic) {
        attr(0x31, 0x13);
    } else {
        attr(0x6e, 0x0e);
    }
    attr(0, 0);
    put_uleb(ab, 4); // the function that the calls are inlined into
    put_uleb(ab, 0x2e);
    ab += '\1';
    attr(0x11, 0x01);
    attr(0x12, 0x06);
    attr(0, 0);
    put_uleb(ab, 5); // the call of main.get
    put_uleb(ab, 0x1d);
    ab += '\1';
    if (k.get_origin) {
        attr(0x31, 0x13);
    }
    attr(0x11, 0x1b); // low_pc addrx
    attr(0x12, 0x06); // high_pc data4: the size
    attr(0x58, 0x0b);
    attr(0x59, k.form_override != 0 ? k.form_override : 0x0b);
    attr(0, 0);
    put_uleb(ab, 6); // the call of main.sum
    put_uleb(ab, 0x1d);
    ab += '\0';
    attr(0x31, 0x13);
    attr(0x55, 0x23); // ranges rnglistx
    attr(0x58, 0x0b);
    attr(0x59, 0x0f);
    attr(0, 0);
    ab += '\0';
    img.debug_abbrev = ab;

    // .debug_str, .debug_str_offsets, .debug_addr, .debug_rnglists
    img.debug_str = std::string("\0", 1);
    uint64_t get_off = img.debug_str.size();
    put_str(img.debug_str, "_ZN9gpkg_main3getEv");
    uint64_t sum_off = img.debug_str.size();
    put_str(img.debug_str, "_ZN9gpkg_main3sumEv");
    img.debug_str_offsets.clear();
    put_le(img.debug_str_offsets, 12, 4); // the header: length, version 5, padding
    put_le(img.debug_str_offsets, 5, 2);
    put_le(img.debug_str_offsets, 0, 2);
    put_le(img.debug_str_offsets, get_off, 4);
    img.debug_addr.clear();
    put_le(img.debug_addr, 20, 4); // the header: length, version, address size, segment size
    put_le(img.debug_addr, 5, 2);
    img.debug_addr += std::string("\x08\x00", 2);
    put_le(img.debug_addr, 0x1000, 8);
    put_le(img.debug_addr, 0x1008, 8);
    img.debug_rnglists.clear();
    put_le(img.debug_rnglists, 0, 4); // the header (its length is not read)
    put_le(img.debug_rnglists, 5, 2);
    img.debug_rnglists += std::string("\x08\x00", 2);
    put_le(img.debug_rnglists, 1, 4); // one offset
    put_le(img.debug_rnglists, 4, 4); // the list, 4 bytes after the offsets (rnglists_base 12)
    img.debug_rnglists += '\x06';     // DW_RLE_start_end
    put_le(img.debug_rnglists, 0x1010, 8);
    put_le(img.debug_rnglists, 0x1018, 8);
    img.debug_rnglists += '\0';
    img.debug_line = call_files();

    // .debug_info
    std::string dies;
    put_uleb(dies, 1);
    put_le(dies, 0, 4); // stmt_list
    put_le(dies, 0, 8); // low_pc
    put_le(dies, 8, 4); // str_offsets_base
    put_le(dies, k.addr_base, 4);
    put_le(dies, 12, 4);        // rnglists_base
    const uint64_t header = 12; // unit_length 4, version 2, unit_type 1, address_size 1, abbrev 4
    uint64_t get_die = header + dies.size();
    put_uleb(dies, 2);
    dies += static_cast<char>(k.name_strx);
    uint64_t sum_die = header + dies.size();
    put_uleb(dies, 3);
    if (k.cyclic) {
        put_le(dies, sum_die, 4);
    } else {
        put_le(dies, sum_off, 4);
    }
    put_uleb(dies, 4);
    put_le(dies, 0x1000, 8);
    put_le(dies, 0x100, 4);
    put_uleb(dies, 5);
    if (k.get_origin) {
        put_le(dies, get_die, 4);
    }
    put_uleb(dies, k.low_addrx);
    put_le(dies, 0x18, 4);
    dies += '\1';
    if (k.form_override == 0) {
        dies += static_cast<char>(25);
    } else {
        dies += std::string(4, '\x19');
    }
    put_uleb(dies, 6);
    put_le(dies, sum_die, 4);
    put_uleb(dies, k.rnglistx);
    dies += '\1';
    put_uleb(dies, 9);
    dies += '\0'; // the end of main.get's call
    dies += '\0'; // of the function
    dies += '\0'; // of the unit
    std::string unit;
    put_le(unit, 5, 2);
    unit += '\1'; // DW_UT_compile
    unit += '\x08';
    put_le(unit, 0, 4); // abbrev offset
    unit += dies;
    img.debug_info.clear();
    put_le(img.debug_info, unit.size(), 4);
    img.debug_info += unit;
}

// The levels at call, as "name file:line" lines.
std::string levels_at(const gp::detail::exe_image& img, uint64_t call) {
    std::vector<gp::detail::inline_level> levels;
    gp::detail::read_inline_frames(img, call, 0x1000, 0x1100, levels);
    std::string out;
    for (const auto& l : levels) {
        out += (l.fn.shown ? l.fn.name : std::string("-")) + " " + l.file + ":" +
               std::to_string(static_cast<long long>(l.line)) + "\n";
    }
    return out;
}

void test_inline_reader() {
    // Every image stays alive: the reader keeps its index by the address and size of .debug_info.
    std::vector<std::unique_ptr<gp::detail::exe_image>> keep;
    auto image = [&] {
        keep.push_back(std::make_unique<gp::detail::exe_image>());
        return keep.back().get();
    };

    // DWARF 5: the two calls, outermost first; addresses outside them have fewer levels.
    gp::detail::exe_image* img = image();
    make5(*img, dwarf5{});
    CHECK_EQ(levels_at(*img, 0x1012),
             std::string("main.get /w/main.gp:25\nmain.sum /w/main.gp:9\n"));
    CHECK_EQ(levels_at(*img, 0x1009), std::string("main.get /w/main.gp:25\n"));
    CHECK_EQ(levels_at(*img, 0x1020), std::string());
    CHECK_EQ(levels_at(*img, 0x0fff), std::string());

    // Missing sections: no levels.
    gp::detail::exe_image* none = image();
    make5(*none, dwarf5{});
    none->debug_abbrev.clear();
    CHECK_EQ(levels_at(*none, 0x1012), std::string());
    gp::detail::exe_image* noinfo = image();
    make5(*noinfo, dwarf5{});
    noinfo->debug_info.clear();
    CHECK_EQ(levels_at(*noinfo, 0x1012), std::string());
    gp::detail::exe_image* nolines = image();
    make5(*nolines, dwarf5{});
    nolines->debug_line.clear(); // the calls without their files
    CHECK_EQ(levels_at(*nolines, 0x1012), std::string("main.get :25\nmain.sum :9\n"));

    // Indexes past the end of their tables, also so far that the offset would wrap: the call
    // is left out (no address; the call inlined into it stays) or its function has no name, and
    // nothing is read outside.
    for (uint64_t big : {uint64_t(2), uint64_t(1) << 40, ~uint64_t(0) / 8 + 1, ~uint64_t(0)}) {
        dwarf5 k;
        k.low_addrx = big;
        gp::detail::exe_image* a = image();
        make5(*a, k);
        CHECK_EQ(levels_at(*a, 0x1012), std::string("main.sum /w/main.gp:9\n"));
        k = dwarf5{};
        k.rnglistx = big;
        gp::detail::exe_image* r = image();
        make5(*r, k);
        CHECK_EQ(levels_at(*r, 0x1012), std::string("main.get /w/main.gp:25\n"));
    }
    dwarf5 nostr;
    nostr.name_strx = 200;
    gp::detail::exe_image* ns = image();
    make5(*ns, nostr);
    CHECK_EQ(levels_at(*ns, 0x1012), std::string("- /w/main.gp:25\nmain.sum /w/main.gp:9\n"));
    dwarf5 wrap; // addr_base + index * 8 wraps to 0
    wrap.addr_base = ~uint64_t(0) - 7;
    wrap.low_addrx = 1;
    gp::detail::exe_image* w = image();
    make5(*w, wrap);
    CHECK_EQ(levels_at(*w, 0x1012), std::string("main.sum /w/main.gp:9\n"));

    // A call without its function (MinGW's g++ under -flto): a level without a name, so that the
    // function around it still gets the call's line.
    dwarf5 noorigin;
    noorigin.get_origin = false;
    gp::detail::exe_image* no = image();
    make5(*no, noorigin);
    CHECK_EQ(levels_at(*no, 0x1012), std::string("- /w/main.gp:25\nmain.sum /w/main.gp:9\n"));

    // A form the reader doesn't know: the unit is left, no levels.
    dwarf5 unknown;
    unknown.form_override = 0x7f;
    gp::detail::exe_image* u = image();
    make5(*u, unknown);
    CHECK_EQ(levels_at(*u, 0x1012), std::string());

    // A DIE that is its own abstract origin: no name, and the reader ends.
    dwarf5 cyc;
    cyc.cyclic = true;
    gp::detail::exe_image* c = image();
    make5(*c, cyc);
    CHECK_EQ(levels_at(*c, 0x1012), std::string("main.get /w/main.gp:25\n- /w/main.gp:9\n"));

    // Every truncation of each section: whatever the levels are, nothing is read outside (the
    // -asan run of this test would see it).
    for (int which = 0; which < 5; which++) {
        gp::detail::exe_image full;
        make5(full, dwarf5{});
        std::string* sec = which == 0   ? &full.debug_info
                           : which == 1 ? &full.debug_abbrev
                           : which == 2 ? &full.debug_addr
                           : which == 3 ? &full.debug_rnglists
                                        : &full.debug_str_offsets;
        for (std::size_t n = 0; n < sec->size(); n++) {
            gp::detail::exe_image* t = image();
            make5(*t, dwarf5{});
            std::string* tsec = which == 0   ? &t->debug_info
                                : which == 1 ? &t->debug_abbrev
                                : which == 2 ? &t->debug_addr
                                : which == 3 ? &t->debug_rnglists
                                             : &t->debug_str_offsets;
            tsec->resize(n);
            tsec->shrink_to_fit();
            levels_at(*t, 0x1012);
        }
    }
    CHECK(true); // reached the end

    // Deep nesting: 300 calls, each inlined into the one before, all over [0x1000, 0x1100).
    {
        gp::detail::exe_image* d = image();
        std::string ab;
        put_uleb(ab, 1);
        put_uleb(ab, 0x11);
        ab += '\1';
        put_uleb(ab, 0x10); // stmt_list
        put_uleb(ab, 0x06);
        ab += std::string(2, '\0');
        put_uleb(ab, 2); // the inlined function
        put_uleb(ab, 0x2e);
        ab += '\0';
        put_uleb(ab, 0x6e);
        put_uleb(ab, 0x08); // linkage_name string
        ab += std::string(2, '\0');
        put_uleb(ab, 3); // a call: DWARF 4 forms
        put_uleb(ab, 0x1d);
        ab += '\1';
        const uint64_t forms[][2] = {
            {0x31, 0x13}, {0x11, 0x01}, {0x12, 0x01}, {0x58, 0x0b}, {0x59, 0x0f}};
        for (const auto& f : forms) {
            put_uleb(ab, f[0]);
            put_uleb(ab, f[1]);
        }
        ab += std::string(3, '\0');
        d->debug_abbrev = ab;
        d->debug_line = call_files();
        std::string dies;
        put_uleb(dies, 1);
        put_le(dies, 0, 4);
        const uint64_t header = 11; // DWARF 4: length 4, version 2, abbrev 4, address size 1
        uint64_t fn = header + dies.size();
        put_uleb(dies, 2);
        put_str(dies, "_ZN9gpkg_main4leafEv");
        const int depth = 300;
        for (int i = 0; i < depth; i++) {
            put_uleb(dies, 3);
            put_le(dies, fn, 4);
            put_le(dies, 0x1000, 8);
            put_le(dies, 0x1100, 8); // high_pc as an address
            dies += '\1';
            put_uleb(dies, static_cast<uint64_t>(1000 + i));
        }
        dies += std::string(depth + 1, '\0');
        std::string unit;
        put_le(unit, 4, 2);
        put_le(unit, 0, 4);
        unit += '\x08';
        unit += dies;
        put_le(d->debug_info, unit.size(), 4);
        d->debug_info += unit;
        std::vector<gp::detail::inline_level> levels;
        gp::detail::read_inline_frames(*d, 0x1050, 0x1000, 0x1100, levels);
        CHECK_EQ(int64_t(levels.size()), int64_t(depth));
        if (levels.size() == static_cast<std::size_t>(depth)) {
            CHECK_EQ(levels.front().line, int64_t(1000));
            CHECK_EQ(levels.back().line, int64_t(1000 + depth - 1));
            CHECK_EQ(levels.back().fn.name, std::string("main.leaf"));
            CHECK_EQ(levels.back().file, std::string("/w/main.gp"));
        }
    }
}

// A row without a line (0: clang's code merged from several lines) inside an inlined call: the
// query with the call's range gets the nearest row of a .gp file in it (indexed_lines_of).
void test_near_lines() {
    std::string rest;
    rest += std::string("\1\1\1\xfb\x0e\x0d", 6);
    for (char len : {0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1}) {
        rest += len;
    }
    rest += '\0';
    put_str(rest, "/w/near.gp"); // file 1
    rest += std::string(3, '\0');
    put_str(rest, "/w/gocpp/ptr.hpp"); // file 2
    rest += std::string(3, '\0');
    rest += '\0';
    std::string program;
    program += std::string("\0\x09\x02", 3); // set_address 0x5000, line 1
    put_le(program, 0x5000, 8);
    program += '\x03'; // advance_line +4: line 5 of near.gp
    program += '\x04';
    program += '\x01'; // copy: [0x5000, 0x5004) near.gp:5
    program += '\x02'; // advance_pc 4
    program += '\x04';
    program += '\x04'; // set_file 2
    program += '\x02';
    program += '\x03'; // advance_line +10: line 15 of ptr.hpp
    program += '\x0a';
    program += '\x01'; // copy: [0x5004, 0x5008) ptr.hpp:15
    program += '\x02';
    program += '\x04';
    program += '\x03'; // advance_line -15: line 0
    program += '\x71';
    program += '\x01'; // copy: [0x5008, 0x500c) line 0
    program += '\x02';
    program += '\x04';
    program += std::string("\0\1\1", 3); // end_sequence at 0x500c
    std::string unit;
    put_le(unit, 4, 2);
    put_le(unit, rest.size(), 4);
    unit += rest;
    unit += program;
    std::string table;
    put_le(table, unit.size(), 4);
    table += unit;

    gp::detail::line_query plain;
    plain.addr = 0x5009;
    gp::detail::indexed_lines_of(table, "", "", &plain, 1);
    CHECK_EQ(plain.line, int64_t(0)); // without the range: the row's line, 0
    gp::detail::line_query near;
    near.addr = 0x5009;
    near.near_lo = 0x5000;
    near.near_hi = 0x500c;
    gp::detail::indexed_lines_of(table, "", "", &near, 1);
    CHECK_EQ(near.line, int64_t(5)); // the .gp row, not ptr.hpp's nearer one
    CHECK_EQ(near.file, std::string("/w/near.gp"));
    gp::detail::line_query hdr;
    hdr.addr = 0x5009;
    hdr.near_lo = 0x5004;
    hdr.near_hi = 0x500c;
    gp::detail::indexed_lines_of(table, "", "", &hdr, 1);
    CHECK_EQ(hdr.line, int64_t(15)); // no .gp row in the range: the nearest row with a line
}

void no_init() {}

void run_tests() {
    test_func_name();
    test_trace();
    test_lines();
    test_line_paths();
    test_inline_reader();
    test_near_lines();
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
