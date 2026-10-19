// The unit tests of the start of a library (gocpp/lib.hpp, src/lib.cpp; Phase 06, step 6.10):
// gp::lib::enter (once, thread-safe, the calling thread becomes a goroutine), the copies at the
// boundary of the exported functions (from_c, out), and the end of a library (shutdown) with the
// goroutine records of the threads that called in. runtime/cxx_test.go compiles this file with the
// runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan (LeakSanitizer
// included), and runs it like test/unit.cpp: it prints "ok: N checks" and exits 0, or prints each
// failed check and exits 1.
//
// It has a plain main(), not gp::run_main: a library is called by programs that gocpp did not
// start. Nothing here depends on timing: threads synchronize through atomics, and a wait has a
// limit of minutes.
#include <gocpp/goroutine.hpp>
#include <gocpp/lib.hpp>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include "threads_ended.hpp"

#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <thread>
#include <vector>

namespace {

using namespace std::chrono_literals;

std::atomic<int> checks{0};
std::atomic<int> failures{0};

void say(const std::string& s) {
    gp::os::write(gp::os::stderr_fd, s.data(), s.size());
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

// The C layouts of lib.hpp's header.
struct CString {
    const char* p;
    ptrdiff_t n;
};
struct CSlice {
    void* data;
    int64_t len;
    int64_t cap;
};

// What the package initialization of the tests does: it counts, and takes the others in (nested
// calls of enter from init, and other threads that call while init runs).
std::atomic<int> init_runs{0};
std::atomic<int> init_done{0};
std::atomic<int> nested_ok{0};

void test_init() {
    init_runs++;
    // An exported function that init reaches through C calls enter again on this thread: it must
    // not wait for init to finish.
    gp::lib::enter(&test_init);
    nested_ok++;
    // Give the other threads time to arrive at enter while init runs.
    std::this_thread::sleep_for(20ms);
    init_done++;
}

void test_enter_once() {
    CHECK(!gp::lib::entered());
    constexpr int n = 8;
    std::atomic<int> started{0};
    std::atomic<bool> go{false};
    std::atomic<int> saw_done{0};
    std::vector<std::thread> threads;
    for (int i = 0; i < n; i++) {
        threads.emplace_back([&] {
            started++;
            while (!go.load(std::memory_order_acquire)) {
                std::this_thread::yield();
            }
            gp::lib::enter(&test_init);
            // enter returns when the initialization has finished, on every thread.
            if (init_done.load() == 1 && gp::lib::entered()) {
                saw_done++;
            }
        });
    }
    while (started.load() < n) {
        std::this_thread::yield();
    }
    go.store(true, std::memory_order_release);
    for (auto& t : threads) {
        t.join();
    }
    CHECK_EQ(init_runs.load(), 1);
    CHECK_EQ(init_done.load(), 1);
    CHECK_EQ(nested_ok.load(), 1);
    CHECK_EQ(saw_done.load(), n);
    CHECK(gp::lib::entered());
    // Later calls do nothing.
    gp::lib::enter(&test_init);
    gp::lib::enter(nullptr);
    CHECK_EQ(init_runs.load(), 1);
}

void test_adoption() {
    // The threads above came and went: their goroutine records are gone with them.
    rt_test::threads_ended();
    std::size_t base = gp::detail::adopted_count();
    constexpr int n = 6;
    std::atomic<int> in{0};
    std::atomic<bool> leave{false};
    std::vector<std::thread> threads;
    std::atomic<uint64_t> ids[n];
    for (int i = 0; i < n; i++) {
        ids[i] = 0;
        threads.emplace_back([&, i] {
            gp::lib::enter(nullptr);
            ids[i] = gp::detail::current_goroutine_id();
            in++;
            while (!leave.load(std::memory_order_acquire)) {
                std::this_thread::yield();
            }
        });
    }
    while (in.load() < n) {
        std::this_thread::yield();
    }
    CHECK_EQ(gp::detail::adopted_count(), base + n);
    // Each thread is a goroutine of its own, with an id.
    for (int i = 0; i < n; i++) {
        CHECK(ids[i].load() != 0);
        for (int j = 0; j < i; j++) {
            CHECK(ids[i].load() != ids[j].load());
        }
    }
    leave.store(true, std::memory_order_release);
    for (auto& t : threads) {
        t.join();
    }
    // A thread that ended gave its record back: a library that is called from a thread for each
    // request holds nothing for the threads that are gone.
    CHECK_EQ(gp::detail::adopted_count(), base);
    // A thread that comes after them gets a fresh record.
    std::thread([&] {
        gp::lib::enter(nullptr);
        CHECK_EQ(gp::detail::adopted_count(), base + 1);
    }).join();
    CHECK_EQ(gp::detail::adopted_count(), base);
}

void test_adoption_many() {
    // Many short threads, one after the other and a few at a time: the count goes back every time.
    std::size_t base = gp::detail::adopted_count();
    for (int round = 0; round < 50; round++) {
        std::vector<std::thread> threads;
        for (int i = 0; i < 4; i++) {
            threads.emplace_back([] { gp::lib::enter(nullptr); });
        }
        for (auto& t : threads) {
            t.join();
        }
    }
    CHECK_EQ(gp::detail::adopted_count(), base);
}

void test_strings() {
    // A string parameter is a copy.
    char buf[8] = {'a', 'b', 0, 'c', 'd'};
    CString in{buf, 5};
    gp::string s = gp::lib::from_c<gp::string>(in);
    buf[0] = 'X'; // the caller reuses its memory
    CHECK_EQ(s.gp_view().size(), std::size_t(5));
    CHECK(s.gp_view() == std::string_view("ab\0cd", 5));
    CString empty{nullptr, 0};
    CHECK(gp::lib::from_c<gp::string>(empty).gp_view().empty());
    CString neg{buf, -3};
    CHECK(gp::lib::from_c<gp::string>(neg).gp_view().empty());

    // A string result is malloc memory with a NUL after the last byte.
    CString out = gp::lib::out<CString>(gp::string::gp_copy("hello"));
    CHECK_EQ(out.n, ptrdiff_t(5));
    CHECK(out.p != nullptr && std::strcmp(out.p, "hello") == 0);
    std::free(const_cast<char*>(out.p));
    CString none = gp::lib::out<CString>(gp::string());
    CHECK_EQ(none.n, ptrdiff_t(0));
    CHECK(none.p != nullptr && none.p[0] == '\0'); // an empty string still ends with its NUL
    std::free(const_cast<char*>(none.p));
}

void test_slices() {
    int32_t src[4] = {1, 2, 3, 4};
    CSlice in{src, 4, 4};
    auto s = gp::lib::from_c<gp::slice<int32_t>>(in);
    src[0] = 99;
    CHECK_EQ(s.gp_len(), int64_t(4));
    CHECK_EQ(s[0], 1);
    CHECK_EQ(s[3], 4);
    // The slice is the function's own: it may write to it.
    s[1] = 77;
    CHECK_EQ(src[1], 2);

    CSlice nil{nullptr, 0, 0};
    CHECK_EQ(gp::lib::from_c<gp::slice<int32_t>>(nil).gp_len(), int64_t(0));

    uint8_t flags[3] = {1, 0, 2};
    auto b = gp::lib::from_c<gp::slice<bool>>(CSlice{flags, 3, 3});
    CHECK(b[0] && !b[1] && b[2]);

    // A slice result is a malloc copy of its len elements.
    auto r = gp::make_slice<int64_t>(3);
    r[0] = 10;
    r[1] = 20;
    r[2] = 30;
    CSlice o = gp::lib::out<CSlice>(r);
    CHECK_EQ(o.len, int64_t(3));
    CHECK_EQ(o.cap, int64_t(3));
    CHECK_EQ(static_cast<int64_t*>(o.data)[2], int64_t(30));
    CHECK(o.data != r.gp_data());
    std::free(o.data);
    CSlice onil = gp::lib::out<CSlice>(gp::slice<int64_t>());
    CHECK(onil.data == nullptr);
    CHECK_EQ(onil.len, int64_t(0));
}

void test_numbers() {
    CHECK_EQ(gp::lib::from_c<int32_t>(int(-5)), -5);
    CHECK_EQ(gp::lib::from_c<uint8_t>(300), uint8_t(44)); // converted like a C call
    CHECK(gp::lib::from_c<bool>(uint8_t(2)));
    CHECK(!gp::lib::from_c<bool>(uint8_t(0)));
    CHECK_EQ(gp::lib::out<int32_t>(int64_t(7)), 7);
    CHECK_EQ(gp::lib::out<uint8_t>(true), uint8_t(1));
    CHECK_EQ(gp::lib::out<double>(1.5f), 1.5);
    int x = 0;
    int* p = gp::lib::out<int*>(&x);
    CHECK(p == &x);
}

} // namespace

int main() {
    test_enter_once();
    test_adoption();
    test_adoption_many();
    test_strings();
    test_slices();
    test_numbers();
    // The end of a library: the idle threads end, and the records are forgotten.
    gp::lib::shutdown();
    CHECK_EQ(gp::detail::adopted_count(), std::size_t(0));
    rt_test::threads_ended();
    if (failures.load() != 0) {
        std::printf("FAIL: %d of %d checks failed\n", failures.load(), checks.load());
        return 1;
    }
    std::printf("ok: %d checks\n", checks.load());
    return 0;
}
