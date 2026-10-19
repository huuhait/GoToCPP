// The unit tests of the native part of the packages runtime and runtime/debug
// (gocpp/runtimepkg.hpp, src/runtimepkg.cpp, src/frames.cpp, src/os/linux/cpu.cpp) and of the exit
// path (gp::exit, gocpp/goroutine.hpp), Phase 08, steps 8.9 and 8.10.
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++, at
// -O0, at -O2 and with ASan + UBSan, and with ThreadSanitizer (runtimepkg_test.go), and runs it
// like test/unit.cpp: it prints "ok: N checks" and exits 0, or prints each failed check and
// exits 1.
//
// Run with a case name as the one argument, the program runs a case that ends the process in some
// way (exit with live goroutines, with buffered C output, with a leak), which runtimepkg_test.go
// checks.
//
// Nothing here depends on timing: threads wait on waiters and atomics; a wait has a limit of
// minutes.
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include <gocpp/runtimepkg.hpp>
#include <gocpp/trace.hpp>
#include "threads_ended.hpp"

#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <functional>
#include <map>
#include <memory>
#include <string>
#include <thread>
#include <vector>

namespace {

std::atomic<int> checks{0};
std::atomic<int> failures{0};

void say(const std::string& s) {
    gp::os::write(gp::os::stderr_fd, s.data(), s.size());
}

// out writes to standard output, as the fmt package does: with one write, not buffered.
void out(const char* s) {
    gp::os::write(gp::os::stdout_fd, s, std::strlen(s));
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

template <typename P>
bool wait_for(P pred) {
    auto end = std::chrono::steady_clock::now() + std::chrono::minutes(2);
    while (!pred()) {
        if (std::chrono::steady_clock::now() > end) {
            return false;
        }
        std::this_thread::sleep_for(std::chrono::microseconds(200));
    }
    return true;
}

// view is a slice of n elements at p, owned by the caller.
template <typename T>
gp::slice<T> view(T* p, int64_t n) {
    return gp::slice<T>(std::shared_ptr<void>(), p, n, n);
}

// --- cgroup limits ---

// The text of a machine with cgroup v2: the cpu limit is in the group of the process.
constexpr const char* v2_mountinfo =
    "25 30 0:23 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:4 - cgroup2 cgroup2 rw\n";
constexpr const char* v1_mountinfo =
    "30 25 0:26 / /sys/fs/cgroup/memory rw,nosuid shared:8 - cgroup cgroup rw,memory\n"
    "31 25 0:27 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid shared:9 - cgroup cgroup rw,cpu,cpuacct\n";

double limit_of(const char* cgroup, const char* mountinfo,
                std::map<std::string, std::string> files) {
    return gp::os::cgroup_cpu_limit(cgroup, mountinfo,
                                    [&](const std::string& path, std::string& out) {
                                        auto it = files.find(path);
                                        if (it == files.end()) {
                                            return false;
                                        }
                                        out = it->second;
                                        return true;
                                    });
}

void test_cgroup() {
    const char* g2 = "0::/user.slice/app\n";
    CHECK(limit_of(g2, v2_mountinfo,
                   {{"/sys/fs/cgroup/user.slice/app/cpu.max", "200000 100000\n"}}) == 2.0);
    CHECK(limit_of(g2, v2_mountinfo,
                   {{"/sys/fs/cgroup/user.slice/app/cpu.max", "150000 100000\n"}}) == 1.5);
    CHECK(limit_of(g2, v2_mountinfo, {{"/sys/fs/cgroup/user.slice/app/cpu.max", "max 100000\n"}}) ==
          0);
    CHECK(limit_of(g2, v2_mountinfo, {{"/sys/fs/cgroup/user.slice/app/cpu.max", "garbage\n"}}) ==
          0);
    CHECK(limit_of(g2, v2_mountinfo, {{"/sys/fs/cgroup/user.slice/app/cpu.max", "0 100000\n"}}) ==
          0);
    CHECK(limit_of(g2, v2_mountinfo, {}) == 0); // the file is not there
    CHECK(limit_of("0::/\n", v2_mountinfo, {{"/sys/fs/cgroup/cpu.max", "50000 100000"}}) == 0.5);
    // v1: the cpu hierarchy's path, two files
    const char* g1 = "4:cpu,cpuacct:/docker/abc\n3:memory:/docker/abc\n";
    CHECK(limit_of(g1, v1_mountinfo,
                   {{"/sys/fs/cgroup/cpu,cpuacct/docker/abc/cpu.cfs_quota_us", "300000\n"},
                    {"/sys/fs/cgroup/cpu,cpuacct/docker/abc/cpu.cfs_period_us", "100000\n"}}) ==
          3.0);
    CHECK(limit_of(g1, v1_mountinfo,
                   {{"/sys/fs/cgroup/cpu,cpuacct/docker/abc/cpu.cfs_quota_us", "-1\n"},
                    {"/sys/fs/cgroup/cpu,cpuacct/docker/abc/cpu.cfs_period_us", "100000\n"}}) == 0);
    CHECK(limit_of("", v1_mountinfo, {}) == 0);
    CHECK(limit_of(g1, "", {}) == 0);
    CHECK(limit_of(g1, "short line\n", {}) == 0);
    // an escaped space in the mount point
    CHECK(limit_of("0::/a\n", "25 30 0:23 / /sys/fs/c\\040group rw - cgroup2 cgroup2 rw\n",
                   {{"/sys/fs/c group/a/cpu.max", "100000 100000"}}) == 1.0);
    // the real machine: never negative, never a crash
    CHECK(gp::os::cpu_limit() >= 0);
}

void test_adjust() {
    using gp::runtime::adjust_for_cgroup;
    CHECK_EQ(adjust_for_cgroup(8, 0), 8); // no limit
    CHECK_EQ(adjust_for_cgroup(8, 2.0), 2);
    CHECK_EQ(adjust_for_cgroup(8, 2.5), 3); // rounded up
    CHECK_EQ(adjust_for_cgroup(8, 0.5), 2); // at least 2
    CHECK_EQ(adjust_for_cgroup(8, 1.5), 2);
    CHECK_EQ(adjust_for_cgroup(8, 16.0), 8); // never more than the CPUs
    CHECK_EQ(adjust_for_cgroup(1, 0.5), 1);  // never more than the CPUs, even the minimum
    CHECK_EQ(adjust_for_cgroup(4, 4.0), 4);
}

// --- GOMAXPROCS, NumCPU, goroutines ---

void test_procs() {
    namespace rtp = gp::runtime;
    CHECK(rtp::num_cpu() >= 1);
    CHECK_EQ(rtp::num_cpu(), rtp::num_cpu());
    int64_t first = rtp::gomaxprocs(0);
    CHECK(first >= 1);
    CHECK_EQ(rtp::gomaxprocs(-5), first);
    CHECK_EQ(rtp::gomaxprocs(7), first);
    CHECK_EQ(rtp::gomaxprocs(0), 7);
    CHECK_EQ(rtp::gomaxprocs(3), 7);
    CHECK_EQ(rtp::gomaxprocs(-1), 3);
    rtp::set_default_gomaxprocs();
    CHECK_EQ(rtp::gomaxprocs(0), rtp::default_gomaxprocs());
    // concurrent callers: every call returns a value that some call set
    std::vector<std::thread> ts;
    std::atomic<int> bad{0};
    for (int i = 1; i <= 8; i++) {
        ts.emplace_back([i, &bad] {
            for (int k = 0; k < 200; k++) {
                int64_t prev = gp::runtime::gomaxprocs(i);
                if (prev < 1) {
                    bad++;
                }
            }
        });
    }
    for (auto& t : ts) {
        t.join();
    }
    CHECK_EQ(bad.load(), 0);
    int64_t last = rtp::gomaxprocs(0);
    CHECK(last >= 1 && last <= 8);
    CHECK_EQ(rtp::num_cgo_call(), 0);
    rtp::gosched();
    rtp::lock_os_thread();
    rtp::unlock_os_thread();
    rtp::gc();
}

void test_num_goroutine() {
    int64_t base = gp::runtime::num_goroutine();
    CHECK_EQ(base, gp::num_goroutine());
    std::atomic<bool> release{false};
    std::atomic<int> started{0};
    for (int i = 0; i < 10; i++) {
        gp::go([&] {
            started++;
            while (!release.load()) {
                gp::runtime::gosched();
            }
        });
    }
    CHECK(wait_for([&] { return started.load() == 10; }));
    CHECK_EQ(gp::runtime::num_goroutine(), base + 10);
    release.store(true);
    CHECK(wait_for([&] { return gp::runtime::num_goroutine() == base; }));
}

// --- the stack ---

[[gnu::noinline]] std::tuple<uint64_t, gp::string, int64_t, bool> via_caller(int64_t skip) {
    auto r = gp::runtime::caller(skip);
    asm volatile("" ::: "memory"); // not a tail call
    return r;
}

void test_stack() {
    // Stack: Go's first line, cut to the buffer
    std::vector<uint8_t> raw(8192);
    int64_t n = gp::runtime::stack(view<uint8_t>(raw.data(), int64_t(raw.size())), false);
    std::string text(reinterpret_cast<char*>(raw.data()), size_t(n));
    CHECK(text.rfind("goroutine 1 [running]:\n", 0) == 0);
    std::vector<uint8_t> small(10);
    CHECK_EQ(gp::runtime::stack(view<uint8_t>(small.data(), 10), false), 10);
    CHECK_EQ(gp::runtime::stack(gp::slice<uint8_t>(), false), 0);
    // Caller beyond the stack is not ok, and Caller(-1) too
    auto far = via_caller(100000);
    CHECK(!std::get<3>(far));
    CHECK(!std::get<3>(via_caller(-1)));
    // Callers with room 0 and with no skip
    CHECK_EQ(gp::runtime::callers(0, gp::slice<uint64_t>()), 0);
    std::vector<uint64_t> pcs(64);
    int64_t got = gp::runtime::callers(0, view<uint64_t>(pcs.data(), 64));
    CHECK(got >= 1 && got <= 64);
    // pc_infos on an address that is in no function
    uint64_t zero = 0;
    gp::string name;
    gp::string file;
    int64_t line = 7;
    uint64_t entry = 9;
    gp::runtime::pc_infos(view<uint64_t>(&zero, 1), false, view<gp::string>(&name, 1),
                          view<gp::string>(&file, 1), view<int64_t>(&line, 1),
                          view<uint64_t>(&entry, 1));
    CHECK_EQ(entry, uint64_t(0));
    CHECK_EQ(line, int64_t(0));
}

void test_settings() {
    namespace rtp = gp::runtime;
    int64_t gc = rtp::set_gc_percent(50);
    CHECK(gc == 100 || gc >= -1);
    CHECK_EQ(rtp::set_gc_percent(-7), 50);
    CHECK_EQ(rtp::set_gc_percent(100), -1); // any negative is -1
    int64_t lim = rtp::set_memory_limit(-1);
    CHECK_EQ(rtp::set_memory_limit(1 << 20), lim);
    CHECK_EQ(rtp::set_memory_limit(-1), int64_t(1) << 20);
    rtp::set_memory_limit(lim);
    CHECK_EQ(rtp::set_max_threads(4), int64_t(10000));
    CHECK_EQ(rtp::set_max_threads(10000), int64_t(4));
    CHECK_EQ(rtp::set_max_stack(5), int64_t(1000000000));
    CHECK_EQ(rtp::set_max_stack(1000000000), int64_t(5));
    CHECK(!rtp::set_panic_on_fault(true));
    CHECK(rtp::set_panic_on_fault(false));
    CHECK_EQ(rtp::set_mutex_profile_fraction(3), int64_t(0));
    CHECK_EQ(rtp::set_mutex_profile_fraction(-1), int64_t(3));
    CHECK_EQ(rtp::set_mutex_profile_fraction(0), int64_t(3));
    rtp::set_traceback("all"_gs);
    rtp::free_os_memory();
    CHECK(std::string(rtp::version().gp_view()).rfind("go1.", 0) == 0);
}

void test_mem() {
    auto [in_use, sys, idle] = gp::runtime::mem_info();
    CHECK(in_use > 0);
    CHECK(sys > 0 && sys >= in_use / 2);
    (void)idle;
    std::vector<std::unique_ptr<char[]>> keep;
    for (int i = 0; i < 50; i++) {
        keep.push_back(std::make_unique<char[]>(200000));
        keep.back()[0] = 1;
    }
    auto [in_use2, sys2, idle2] = gp::runtime::mem_info();
    CHECK(in_use2 >= in_use + 50 * 200000 - 1000000);
    (void)sys2;
    (void)idle2;
}

void run_unit_tests() {
    test_cgroup();
    test_adjust();
    test_procs();
    test_num_goroutine();
    test_stack();
    test_settings();
    test_mem();
    rt_test::threads_ended();
    if (failures > 0) {
        say("FAIL: " + std::to_string(failures.load()) + " of " + std::to_string(checks.load()) +
            " checks\n");
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks.load()));
}

// --- the cases that end the process ---

[[noreturn]] void park_forever() {
    gp::waiter w;
    for (;;) {
        w.gp_park();
    }
}

[[gnu::noinline]] void make_leak() {
    auto* leak = new int[1000];
    asm volatile("" : : "r"(leak) : "memory");
}

void run_case(const std::string& name) {
    gp::os::no_core_dump();
    if (name == "exit-printf") {
        // C stdio output is buffered when stdout is a pipe: exit flushes it, after gocpp's own
        // (unbuffered) output that came before.
        out("gocpp line\n");
        std::printf("c line without a newline");
        std::fprintf(stderr, "c error line\n");
        gp::exit(5);
    } else if (name == "exit-busy") {
        // A goroutine in a loop that never ends: the exit does not wait for it.
        static std::atomic<bool> up{false};
        gp::go([] {
            up.store(true);
            volatile uint64_t n = 0;
            for (;;) {
                n = n + 1;
            }
        });
        wait_for([] { return up.load(); });
        out("exiting\n");
        gp::exit(0);
    } else if (name == "exit-many") {
        // A thousand goroutines are parked: the exit does not wake or wait for any.
        static std::atomic<int> parked{0};
        for (int i = 0; i < 1000; i++) {
            gp::go([] {
                parked++;
                park_forever();
            });
        }
        wait_for([] { return parked.load() == 1000; });
        out("exiting\n");
        gp::exit(0);
    } else if (name == "exit-code") {
        gp::go([] { park_forever(); });
        gp::exit(42);
    } else if (name == "exit-twice") {
        // Two goroutines call exit at once: one ends the process with its code, the other waits.
        static std::atomic<int> ready{0};
        for (int i = 0; i < 4; i++) {
            gp::go([i] {
                ready++;
                while (ready.load() < 4) {
                }
                gp::exit(10 + i);
            });
        }
        park_forever();
    } else if (name == "exit-leak") {
        // A leak that LeakSanitizer must find even though the process ends by _exit.
        make_leak();
        out("leaked\n");
        gp::exit(0);
    } else if (name == "exit-clean") {
        // No leak: the leak check passes and the exit code is 0.
        auto p = std::make_unique<int[]>(1000);
        p[0] = 1;
        out("clean\n");
        gp::exit(0);
    } else if (name == "return-main") {
        gp::go([] { park_forever(); });
        out("main returns\n");
        return; // run_main ends the program with code 0
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
}

void tests_main() {
    const auto& args = gp::args();
    if (args.size() == 2) {
        run_case(std::string(args[1].gp_view()));
        return;
    }
    run_unit_tests();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
