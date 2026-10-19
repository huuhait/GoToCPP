// The tests of the Windows OS layer (gocpp/os.hpp; Phase 14, step 14.1): the program's
// arguments and environment as UTF-8, output to pipes and files, the ends of a process, the
// thread layer on CreateThread, the faults that become Go's report, stack traces. runtime/
// windows_test.go compiles this file with MinGW-w64 and the runtime's .cpp files and runs it on
// Windows (a real PC over ssh, or Wine), like test/unit.cpp: with no case name it runs the tests
// below in the process and prints "ok: N checks, M tests" (exit 0), or prints each failed check
// (exit 1).
//
// With a case name as the first argument it runs one case that prints something or ends the
// process in a way that the Go test compares with what the case must do: the exact bytes of
// standard output, an exit code, Go's report of a fault. The file also compiles on Linux (the
// tests that need the Windows layer are guarded by _WIN32), so the portable parts keep a check.
//
// Nothing here depends on timing, except that a sleep lasts at least what it was asked for.
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include <gocpp/trace.hpp>
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <cerrno>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <set>
#include <stdexcept>
#include <string>
#include <string_view>
#include <thread>
#include <typeinfo>
#include <vector>

namespace {

using namespace std::chrono_literals;

std::atomic<int> checks{0};
std::atomic<int> failures{0};

void say(const std::string& s) {
    gp::os::write(gp::os::stderr_fd, s.data(), s.size());
}

void out(const std::string& s) {
    gp::os::write(gp::os::stdout_fd, s.data(), s.size());
}

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        say(std::string(file) + ":" + std::to_string(line) + ": " + what + "\n");
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

#define CHECK_EQ(got, want)                                                                        \
    do {                                                                                           \
        auto gp_got = (got);                                                                       \
        decltype(gp_got) gp_want = (want);                                                         \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = " + std::to_string(gp_got) + ", want " + std::to_string(gp_want));         \
    } while (0)

#define CHECK_STR(got, want)                                                                       \
    do {                                                                                           \
        std::string gp_got = (got);                                                                \
        std::string gp_want = (want);                                                              \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = \"" + gp_got + "\", want \"" + gp_want + "\"");                            \
    } while (0)

template <typename P>
bool wait_for(P pred) {
    auto end = std::chrono::steady_clock::now() + 2min;
    while (!pred()) {
        if (std::chrono::steady_clock::now() > end) {
            return false;
        }
        std::this_thread::sleep_for(200us);
    }
    return true;
}

// valid_utf8 reports whether s is well-formed UTF-8 (no overlong forms, no surrogates).
bool valid_utf8(std::string_view s) {
    std::size_t i = 0;
    while (i < s.size()) {
        auto c = static_cast<unsigned char>(s[i]);
        std::size_t n = c < 0x80                ? 1
                        : c >= 0xC2 && c < 0xE0 ? 2
                        : c >= 0xE0 && c < 0xF0 ? 3
                        : c >= 0xF0 && c < 0xF5 ? 4
                                                : 0;
        if (n == 0 || i + n > s.size()) {
            return false;
        }
        for (std::size_t k = 1; k < n; k++) {
            if ((static_cast<unsigned char>(s[i + k]) & 0xC0) != 0x80) {
                return false;
            }
        }
        if (n == 3) {
            unsigned cp = (c & 0x0F) << 12 | (static_cast<unsigned char>(s[i + 1]) & 0x3F) << 6 |
                          (static_cast<unsigned char>(s[i + 2]) & 0x3F);
            if (cp < 0x800 || (cp >= 0xD800 && cp < 0xE000)) {
                return false;
            }
        }
        i += n;
    }
    return true;
}

std::string arg(std::size_t i) {
    const auto& a = gp::args();
    return i < a.size() ? std::string(a[i].gp_view()) : std::string("<missing>");
}

// --- arguments and environment ---

void test_args_runner() {
    // runtime/windows_test.go passes "arg1" and "arg two".
    CHECK_EQ(gp::args().size(), std::size_t(3));
    CHECK_STR(arg(1), "arg1");
    CHECK_STR(arg(2), "arg two");
}

void test_args_program_name() {
    std::string p = arg(0);
    CHECK(p.size() > 4);
    CHECK(p.ends_with(".exe"));
    // A Windows path: a drive letter or a UNC prefix, backslashes or slashes.
    CHECK(p.find('\\') != std::string::npos || p.find('/') != std::string::npos);
}

void test_args_direct() {
    std::vector<std::string> v = gp::os::arguments(0, nullptr);
    CHECK_EQ(v.size(), gp::args().size());
    for (std::size_t i = 0; i < v.size() && i < gp::args().size(); i++) {
        CHECK_STR(v[i], arg(i));
    }
}

void test_env_marker() {
    bool found = false;
    for (const gp::string& e : gp::envs()) {
        found = found || e == "GOCPP_RT_TEST=env-ok"_gs;
    }
    CHECK(found);
}

void test_env_entries_are_utf8() {
    std::vector<std::string> env = gp::os::environment();
    CHECK(!env.empty());
    bool all = true, shaped = true;
    for (const std::string& e : env) {
        all = all && valid_utf8(e);
        // NAME=value; the entries of the drives' current folders start with '=' ("=C:=C:\dir").
        shaped = shaped && e.find('=', e.empty() || e[0] != '=' ? 0 : 1) != std::string::npos;
    }
    CHECK(all);
    CHECK(shaped);
}

void test_env_matches_getenv() {
    // The environment block and the C library's getenv see the same variable.
    const char* v = std::getenv("GOCPP_RT_TEST");
    CHECK(v != nullptr && std::string(v) == "env-ok");
    std::vector<std::string> env = gp::os::environment();
    CHECK(std::find(env.begin(), env.end(), "GOCPP_RT_TEST=env-ok") != env.end());
}

void test_envs_equal_environment() {
    std::vector<std::string> env = gp::os::environment();
    CHECK_EQ(gp::envs().size(), env.size());
}

// --- output ---

void test_write_invalid_fd() {
    CHECK(gp::os::write(97, "x", 1) == gp::os::write_result::failed);
}

void test_write_empty() {
    CHECK(gp::os::write(gp::os::stderr_fd, "", 0) == gp::os::write_result::ok);
}

// --- the thread layer ---

struct probe {
    std::atomic<int> state{0};
    uintptr_t lo = 0, hi = 0;
    bool bounds = false;
    void* slot_before = nullptr;
    void* slot_after = nullptr;
    void* mine = nullptr;
};

void probe_body(void* p) {
    auto* pr = static_cast<probe*>(p);
    pr->slot_before = gp::os::tls_get();
    gp::os::tls_set(pr);
    pr->slot_after = gp::os::tls_get();
    pr->bounds = gp::os::stack_bounds(pr->lo, pr->hi);
    pr->mine = pr;
    pr->state.store(1, std::memory_order_release);
}

void test_thread_start_runs() {
    probe pr;
    CHECK_EQ(gp::os::thread_start(probe_body, &pr, std::size_t(1) << 20), 0);
    CHECK(wait_for([&] { return pr.state.load(std::memory_order_acquire) == 1; }));
    CHECK(pr.slot_before == nullptr);
    CHECK(pr.slot_after == &pr);
    CHECK(pr.bounds);
}

void test_thread_stack_size() {
    for (std::size_t want : {std::size_t(256) << 10, std::size_t(1) << 20, std::size_t(8) << 20,
                             std::size_t(64) << 20}) {
        probe pr;
        CHECK_EQ(gp::os::thread_start(probe_body, &pr, want), 0);
        CHECK(wait_for([&] { return pr.state.load(std::memory_order_acquire) == 1; }));
        // The OS may give more than asked for (the top holds the thread's own data), and never
        // much less.
        CHECK(pr.hi > pr.lo && pr.hi - pr.lo >= want - (std::size_t(64) << 10));
    }
}

void test_thread_min_stack() {
    // Less than the OS layer's smallest stack is raised to it, so the handler fits.
    probe pr;
    CHECK_EQ(gp::os::thread_start(probe_body, &pr, 4096), 0);
    CHECK(wait_for([&] { return pr.state.load(std::memory_order_acquire) == 1; }));
    CHECK(pr.hi - pr.lo >= (std::size_t(128) << 10));
}

void test_thread_too_big() {
    probe pr;
    int err = gp::os::thread_start(probe_body, &pr, std::size_t(1) << 50);
    CHECK(err != 0);
    CHECK_EQ(err, ENOMEM);
    // Nothing started.
    std::this_thread::sleep_for(20ms);
    CHECK_EQ(pr.state.load(), 0);
}

void test_thread_slots_differ() {
    constexpr int n = 8;
    probe prs[n];
    for (auto& pr : prs) {
        CHECK_EQ(gp::os::thread_start(probe_body, &pr, std::size_t(512) << 10), 0);
    }
    for (auto& pr : prs) {
        CHECK(wait_for([&] { return pr.state.load(std::memory_order_acquire) == 1; }));
    }
    std::set<void*> seen;
    for (auto& pr : prs) {
        CHECK(pr.slot_before == nullptr); // a new thread's slot is empty, not a former one's
        seen.insert(pr.slot_after);
    }
    CHECK_EQ(seen.size(), std::size_t(n));
    CHECK(gp::os::tls_get() != &prs[0]); // the test's own thread is not touched
}

void test_main_thread_bounds() {
    uintptr_t lo = 0, hi = 0;
    CHECK(gp::os::stack_bounds(lo, hi));
    int local = 0;
    auto sp = reinterpret_cast<uintptr_t>(&local);
    CHECK(lo < sp && sp < hi);
    // The linker's stack size of the PE header, 8 MiB as on Linux (the driver passes
    // -Wl,--stack; the runtime asks for it by a directive when it is linked without it).
    CHECK(hi - lo >= (std::size_t(8) << 20) - (std::size_t(128) << 10));
}

void test_foreign_thread_has_no_bounds() {
    bool bounds = true;
    std::thread t([&] {
        uintptr_t lo, hi;
        bounds = gp::os::stack_bounds(lo, hi);
    });
    t.join();
    CHECK(!bounds);
}

std::atomic<int> many_done{0};

void many_body(void*) {
    many_done.fetch_add(1, std::memory_order_acq_rel);
}

void test_thread_many() {
    many_done = 0;
    constexpr int n = 500;
    for (int i = 0; i < n; i++) {
        int err = gp::os::thread_start(many_body, nullptr, std::size_t(256) << 10);
        CHECK_EQ(err, 0);
        if (err != 0) {
            break;
        }
    }
    CHECK(wait_for([&] { return many_done.load() == n; }));
}

std::atomic<int> hook_calls{0};
void hook_a(const gp::os::stack_fault&) noexcept {
    hook_calls++;
}
void hook_b(const gp::os::stack_fault&) noexcept {}

void test_overflow_hook_set() {
    auto prev = gp::os::set_stack_overflow_hook(hook_a);
    auto again = gp::os::set_stack_overflow_hook(hook_b);
    CHECK(again == hook_a);
    auto back = gp::os::set_stack_overflow_hook(prev);
    CHECK(back == hook_b);
}

#ifdef _WIN32
void fault_hook_a(const gp::os::fault_info&) noexcept {}
void fault_hook_b(const gp::os::fault_info&) noexcept {}

void test_fault_hook_set() {
    auto prev = gp::os::set_fault_hook(fault_hook_a);
    CHECK(prev != nullptr); // run_main installs the runtime's
    auto again = gp::os::set_fault_hook(fault_hook_b);
    CHECK(again == fault_hook_a);
    CHECK(gp::os::set_fault_hook(prev) == fault_hook_b);
}
#endif

void test_exit_thread() {
    std::atomic<int> state{0};
    // A thread of thread_start that ends itself: the process goes on. (No destructor runs on
    // Windows, so nothing here depends on one.)
    struct holder {
        std::atomic<int>* s;
    } h{&state};
    auto body = [](void* p) {
        static_cast<holder*>(p)->s->store(1);
        gp::os::exit_thread();
    };
    CHECK_EQ(gp::os::thread_start(body, &h, std::size_t(512) << 10), 0);
    CHECK(wait_for([&] { return state.load() == 1; }));
}

// --- clocks and CPUs ---

void test_clock_monotonic() {
    auto a = std::chrono::steady_clock::now();
    auto b = std::chrono::steady_clock::now();
    CHECK(b >= a);
    std::this_thread::sleep_for(30ms);
    auto c = std::chrono::steady_clock::now();
    CHECK(c - b >= 20ms);
}

void test_clock_resolution() {
    // The steady clock is the performance counter: far finer than the scheduler's tick
    // (15.6 ms), so 1000 reads show many distinct values.
    std::set<int64_t> seen;
    for (int i = 0; i < 1000; i++) {
        seen.insert(std::chrono::steady_clock::now().time_since_epoch().count());
    }
    CHECK(seen.size() > 20);
}

void test_cpu_count() {
    int n = gp::os::cpu_count();
    CHECK(n >= 1);
    CHECK(n <= 4096);
}

// --- the stack trace ---

void test_stack_addresses() {
    uintptr_t pcs[32];
    int n = gp::os::stack(pcs, 32);
    CHECK(n >= 2);
    CHECK(n <= 32);
    CHECK(pcs[0] != 0);
    CHECK_EQ(gp::os::stack(pcs, 0), 0);
    CHECK_EQ(gp::os::stack(pcs, 1), 1);
}

void test_read_exe() {
    gp::detail::exe_image img;
    CHECK(gp::os::read_exe(img));
    CHECK(!img.code.empty());
    CHECK(!img.funcs.empty());
    CHECK(!img.debug_line.empty()); // MinGW's DWARF in the PE file
    // The address of this function is in the program's own code.
    auto here = reinterpret_cast<uintptr_t>(&test_read_exe);
    bool in_code = false;
    for (const auto& r : img.code) {
        in_code = in_code || (here >= r.first && here < r.second);
    }
    CHECK(in_code);
}

void test_demangle() {
    CHECK_STR(gp::os::demangle("_ZN3foo3barEv"), "foo::bar()");
    CHECK_STR(gp::os::demangle("main"), "");
    CHECK_STR(gp::os::demangle(""), "");
}

void test_current_exception_type() {
    CHECK(gp::os::current_exception_type() == nullptr);
    try {
        throw std::out_of_range("x");
    } catch (...) {
        const std::type_info* ti = gp::os::current_exception_type();
        CHECK(ti != nullptr && *ti == typeid(std::out_of_range));
    }
    try {
        throw 7;
    } catch (...) {
        const std::type_info* ti = gp::os::current_exception_type();
        CHECK(ti != nullptr && *ti == typeid(int));
    }
}

// --- C++ on Windows: exceptions and thread_local on the layer's threads ---

thread_local int tl_value = 5;

void tl_body(void* p) {
    tl_value += 10;
    *static_cast<int*>(p) = tl_value;
}

void test_thread_local() {
    int seen = 0;
    CHECK_EQ(gp::os::thread_start(tl_body, &seen, std::size_t(512) << 10), 0);
    CHECK(wait_for([&] { return seen != 0; }));
    CHECK_EQ(seen, 15);
    CHECK_EQ(tl_value, 5); // this thread's copy is its own
}

void throw_deep(int n) {
    if (n == 0) {
        throw std::runtime_error("deep");
    }
    throw_deep(n - 1);
}

void test_exceptions_unwind() {
    // C++ exceptions work through the SEH-based unwinder (0xE06D7363 is not a "fault": the
    // OS layer's filter never sees it).
    std::string got;
    try {
        throw_deep(200);
    } catch (const std::runtime_error& e) {
        got = e.what();
    }
    CHECK_STR(got, "deep");
}

std::string thrown_in_thread;
std::atomic<int> thrown_done{0};

void exception_body(void*) {
    try {
        throw_deep(50);
    } catch (const std::exception& e) {
        thrown_in_thread = e.what();
    }
    thrown_done = 1;
}

void test_exceptions_in_layer_thread() {
    CHECK_EQ(gp::os::thread_start(exception_body, nullptr, std::size_t(512) << 10), 0);
    CHECK(wait_for([&] { return thrown_done.load() == 1; }));
    CHECK_STR(thrown_in_thread, "deep");
}

// --- goroutines, channels, defer on the Windows layer ---

void test_goroutine_channel() {
    auto c = gp::make_chan<int64_t>();
    auto done = gp::make_chan<int64_t>();
    gp::go([c, done] {
        int64_t sum = 0;
        for (int i = 0; i < 100; i++) {
            sum += gp::recv(c);
        }
        gp::send(done, sum);
    });
    for (int i = 1; i <= 100; i++) {
        gp::send(c, int64_t(i));
    }
    CHECK_EQ(gp::recv(done), int64_t(5050));
}

void test_goroutines_many() {
    constexpr int n = 300;
    auto c = gp::make_chan<int64_t>(n);
    for (int i = 0; i < n; i++) {
        gp::go([c, i] { gp::send(c, int64_t(i)); });
    }
    int64_t sum = 0;
    for (int i = 0; i < n; i++) {
        sum += gp::recv(c);
    }
    CHECK_EQ(sum, int64_t(n) * (n - 1) / 2);
}

void test_goroutine_panic_recovered() {
    auto done = gp::make_chan<int64_t>();
    gp::go([done] {
        try {
            gp::panic("boom"_gs);
        } catch (const gp::panic_error&) {
            gp::send(done, int64_t(1));
            return;
        }
        gp::send(done, int64_t(0));
    });
    CHECK_EQ(gp::recv(done), int64_t(1));
}

// --- tls_on_thread_exit and library_ref ---

std::atomic<int> exit_calls{0};
std::atomic<void*> exit_arg{nullptr};

void exit_cb(void* arg) {
    exit_arg.store(arg);
    exit_calls++;
}

void exit_cb_other(void* arg) {
    exit_arg.store(arg);
    exit_calls += 100;
}

void exit_body(void* p) {
    // Two registrations: only the last one counts.
    gp::os::tls_on_thread_exit(exit_cb, nullptr);
    gp::os::tls_on_thread_exit(exit_cb_other, p);
}

void test_thread_exit_callback() {
    exit_calls = 0;
    int marker = 0;
    CHECK_EQ(gp::os::thread_start(exit_body, &marker, std::size_t(512) << 10), 0);
    CHECK(wait_for([&] { return exit_calls.load() != 0; }));
    CHECK_EQ(exit_calls.load(), 100);
    CHECK(exit_arg.load() == &marker);
}

void test_thread_exit_callback_foreign_thread() {
    // A thread that the runtime did not start (a C library's, a std::thread) runs it too.
    exit_calls = 0;
    std::thread t([] { gp::os::tls_on_thread_exit(exit_cb, nullptr); });
    t.join();
    CHECK(wait_for([&] { return exit_calls.load() == 1; }));
}

// The order at the end of a thread: on Windows the callback of tls_on_thread_exit (a fiber-local
// storage callback) comes first, before the C++ thread-local destructors (found on a real PC);
// os.hpp promises no order there. Both must run.
std::string order_log;
std::atomic<int> order_done{0};

struct order_tl {
    ~order_tl() { order_log += 't'; }
};

void order_cb(void*) {
    order_log += 'f';
    order_done = 1;
}

void order_body(void*) {
    thread_local order_tl tl;
    (void)&tl;
    gp::os::tls_on_thread_exit(order_cb, nullptr);
}

void test_thread_exit_order() {
    CHECK_EQ(gp::os::thread_start(order_body, nullptr, std::size_t(512) << 10), 0);
    CHECK(wait_for([&] { return order_done.load() == 1; }));
    CHECK(order_log == "ft" || order_log == "tf"); // both ran, once
}

void test_library_ref_of_the_program() {
    // The code of the program itself has no library to keep.
    CHECK(gp::os::library_ref(reinterpret_cast<const void*>(&test_library_ref_of_the_program)) ==
          nullptr);
    gp::os::library_unref(nullptr); // nothing
}

// --- the tests ---

struct named_test {
    const char* name;
    void (*fn)();
};

const named_test tests[] = {
    {"args_runner", test_args_runner},
    {"args_program_name", test_args_program_name},
    {"args_direct", test_args_direct},
    {"env_marker", test_env_marker},
    {"env_entries_are_utf8", test_env_entries_are_utf8},
    {"env_matches_getenv", test_env_matches_getenv},
    {"envs_equal_environment", test_envs_equal_environment},
    {"write_invalid_fd", test_write_invalid_fd},
    {"write_empty", test_write_empty},
    {"thread_start_runs", test_thread_start_runs},
    {"thread_stack_size", test_thread_stack_size},
    {"thread_min_stack", test_thread_min_stack},
    {"thread_too_big", test_thread_too_big},
    {"thread_slots_differ", test_thread_slots_differ},
    {"main_thread_bounds", test_main_thread_bounds},
    {"foreign_thread_has_no_bounds", test_foreign_thread_has_no_bounds},
    {"thread_many", test_thread_many},
    {"overflow_hook_set", test_overflow_hook_set},
#ifdef _WIN32
    {"fault_hook_set", test_fault_hook_set},
#endif
    {"exit_thread", test_exit_thread},
    {"clock_monotonic", test_clock_monotonic},
    {"clock_resolution", test_clock_resolution},
    {"cpu_count", test_cpu_count},
    {"stack_addresses", test_stack_addresses},
    {"read_exe", test_read_exe},
    {"demangle", test_demangle},
    {"current_exception_type", test_current_exception_type},
    {"thread_local", test_thread_local},
    {"exceptions_unwind", test_exceptions_unwind},
    {"exceptions_in_layer_thread", test_exceptions_in_layer_thread},
    {"thread_exit_callback", test_thread_exit_callback},
    {"thread_exit_callback_foreign_thread", test_thread_exit_callback_foreign_thread},
    {"thread_exit_order", test_thread_exit_order},
    {"library_ref_of_the_program", test_library_ref_of_the_program},
    {"goroutine_channel", test_goroutine_channel},
    {"goroutines_many", test_goroutines_many},
    {"goroutine_panic_recovered", test_goroutine_panic_recovered},
};

void run_unit_tests() {
    for (const named_test& t : tests) {
        int before = failures.load();
        t.fn();
        if (failures.load() != before) {
            say(std::string("FAIL in test ") + t.name + "\n");
        }
    }
    rt_test::threads_ended();
    if (failures.load() != 0) {
        say("FAIL: " + std::to_string(failures.load()) + " of " + std::to_string(checks.load()) +
            " checks\n");
        gp::os::exit(1);
    }
    out("ok: " + std::to_string(checks.load()) + " checks, " +
        std::to_string(sizeof tests / sizeof tests[0]) + " tests\n");
}

// --- the cases: one process each ---

// Four bytes that make one character, written in two calls.
void out_split() {
    out("a \xE6\x97");
    out("\xA5 b\n");
}

void out_binary() {
    std::string s;
    for (int i = 0; i < 256; i++) {
        s += static_cast<char>(i);
    }
    out(s);
}

// 4 MiB: bytes (i * 7 + i / 251) & 255, more than any pipe's buffer, so the write waits for the
// reader.
void out_big() {
    std::string s(std::size_t(4) << 20, '\0');
    for (std::size_t i = 0; i < s.size(); i++) {
        s[i] = static_cast<char>((i * 7 + i / 251) & 255);
    }
    out(s);
}

void out_err_order() {
    for (int i = 0; i < 5; i++) {
        out("out" + std::to_string(i) + "\n");
        say("err" + std::to_string(i) + "\n");
    }
}

void out_stdio() {
    // C code that prints with printf: bytes as they are (no "\r\n"), flushed at exit.
    std::printf("printf line\n");
    std::fputs("fputs line\n", stdout);
    std::fprintf(stderr, "to stderr\n");
}

void args_case() {
    // Each argument on its own line, with its length: arguments can hold anything but a NUL.
    const auto& a = gp::args();
    for (std::size_t i = 1; i < a.size(); i++) {
        std::string s(a[i].gp_view());
        out(std::to_string(s.size()) + ":" + s + "\n");
    }
}

void env_case() {
    // The value of each variable that the arguments name, or "-" when it is not set.
    const auto& a = gp::args();
    std::vector<std::string> env = gp::os::environment();
    for (std::size_t i = 2; i < a.size(); i++) {
        std::string name(a[i].gp_view());
        std::string val = "-";
        for (const std::string& e : env) {
            if (e.size() > name.size() && e.compare(0, name.size(), name) == 0 &&
                e[name.size()] == '=') {
                val = e.substr(name.size() + 1);
            }
        }
        out(name + "=" + val + "\n");
    }
}

void at_exit_handler() {
    out("atexit ran\n");
}

struct static_destructor {
    ~static_destructor() { out("static destructor ran\n"); }
} the_static;

void exit_thread_body(void*) {
    gp::os::exit(5);
}

// A division by zero that the compiler can't see.
int divide(int a, int b) {
    volatile int d = b;
    return a / d;
}

#ifdef _WIN32
void custom_fault_hook(const gp::os::fault_info& f) noexcept {
    std::string s = "hook: code=" + std::to_string(f.gp_code) +
                    " access=" + std::to_string(f.gp_access) +
                    " addr=" + std::to_string(f.gp_addr) +
                    " pcs=" + (f.gp_npcs >= 2 && f.gp_pcs[0] == f.gp_pc ? "ok" : "bad") + "\n";
    out(s);
    gp::os::exit(11);
}
#endif

void fault_read() {
    volatile int* p = nullptr;
    int v = *p;
    out(std::to_string(v));
}

void fault_write() {
    volatile int* p = nullptr;
    *p = 1;
}

void fault_wild() {
    volatile int* p = reinterpret_cast<volatile int*>(std::uintptr_t(0x1000) << 32);
    *p = 1;
}

void fault_thread_body(void*) {
    fault_read();
}

void run_case(const std::string& name) {
    gp::os::no_core_dump();
    if (name == "args") {
        args_case();
    } else if (name == "env") {
        env_case();
    } else if (name == "out-newline") {
        out("a\nb\n\nc");
    } else if (name == "out-utf8") {
        out("h\xC3\xA9llo \xE6\x97\xA5\xE6\x9C\xAC\xE8\xAA\x9E \xF0\x9F\x8E\x89\n");
    } else if (name == "out-binary") {
        out_binary();
    } else if (name == "out-big") {
        out_big();
    } else if (name == "out-split") {
        out_split();
    } else if (name == "out-stdio") {
        out_stdio();
    } else if (name == "out-err-order") {
        out_err_order();
    } else if (name == "print") {
        gp::println("hello"_gs, 42, true);
        gp::print("no newline"_gs);
    } else if (name == "exit-7") {
        gp::os::exit(7);
    } else if (name == "exit-now-9") {
        out("before\n");
        gp::os::exit_now(9);
    } else if (name == "exit-atexit") {
        std::atexit(at_exit_handler);
        out("main\n");
        gp::os::exit(0);
    } else if (name == "exit-stdio-flush") {
        std::printf("buffered\n");
        gp::os::exit(3);
    } else if (name == "exit-thread") {
        if (gp::os::thread_start(exit_thread_body, nullptr, std::size_t(512) << 10) != 0) {
            gp::os::exit(99);
        }
        std::this_thread::sleep_for(std::chrono::minutes(2));
        gp::os::exit(98);
    } else if (name == "exit-main-return") {
        return; // run_main ends the process with 0 after main returns
    } else if (name == "panic") {
        gp::panic("boom"_gs);
    } else if (name == "panic-goroutine") {
        auto done = gp::make_chan<int64_t>();
        gp::go([] { gp::panic("in goroutine"_gs); });
        gp::recv(done);
    } else if (name == "cxx-exception") {
        throw std::runtime_error("plain C++");
    } else if (name == "fault-read") {
        fault_read();
    } else if (name == "fault-write") {
        fault_write();
    } else if (name == "fault-wild") {
        fault_wild();
    } else if (name == "fault-divzero") {
        out(std::to_string(divide(1, 0)));
    } else if (name == "fault-illegal") {
        __builtin_trap();
    } else if (name == "fault-thread") {
        if (gp::os::thread_start(fault_thread_body, nullptr, std::size_t(512) << 10) != 0) {
            gp::os::exit(99);
        }
        std::this_thread::sleep_for(std::chrono::minutes(2));
#ifdef _WIN32
    } else if (name == "fault-hook") {
        gp::os::set_fault_hook(custom_fault_hook);
        fault_write();
    } else if (name == "fault-hook-returns") {
        // A hook that returns: the fault goes on to the OS's own handling (the crash).
        gp::os::set_fault_hook([](const gp::os::fault_info&) noexcept {});
        fault_read();
#endif
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
    gp::os::exit(0);
}

void tests_main() {
    const auto& args = gp::args();
    // The unit run passes "arg1" and "arg two"; a case is the first argument.
    if (args.size() >= 2 && args[1] != "arg1"_gs) {
        run_case(std::string(args[1].gp_view()));
        return;
    }
    run_unit_tests();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
