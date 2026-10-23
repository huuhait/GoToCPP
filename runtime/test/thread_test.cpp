// The unit tests of the thread layer gp::os (gocpp/os.hpp, "threads"; Phase 08, step 8.15):
// thread_start with a stack size, the thread's slot, the stack bounds, and the stack-overflow
// hook. runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and
// clang++, at -O0, at -O2 and with ASan + UBSan (LeakSanitizer included), and with
// ThreadSanitizer (thread_test.go), and runs it like test/unit.cpp: it prints "ok: N checks" and
// exits 0, or prints each failed check and exits 1.
//
// Run with a case name as the one argument, the program runs a case that ends the process (a
// stack overflow of the main thread or of a thread, with the runtime's report or with a hook of
// the test, and the faults that must stay crashes), which thread_test.go compares with what Go
// prints.
//
// Nothing here depends on timing: threads synchronize through atomics, and a wait has a limit
// of minutes (a hang is a failure, a slow machine is not).
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include <gocpp/syscall.hpp>
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <csignal>
#include <cstdint>
#include <memory>
#include <set>
#include <string>
#include <string_view>
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

// wait_for polls pred until it is true, for at most two minutes, and says whether it got true.
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

// A thread started with an argument that tells the test what it saw.
struct probe {
    std::atomic<bool> done{false};
    std::thread::id id;
    void* slot_at_start = reinterpret_cast<void*>(1);
    bool bounds = false;
    uintptr_t lo = 0;
    uintptr_t hi = 0;
    uintptr_t frame = 0;
    std::size_t asked = 0;
    bool touched = false;
    std::atomic<bool>* hold = nullptr; // if set, the thread waits for it to be true before it ends
};

[[gnu::noinline]] uintptr_t frame_here() {
    return reinterpret_cast<uintptr_t>(__builtin_frame_address(0));
}

// touch writes every page of a local array of n bytes and reads the last write back: the stack
// has room for it.
[[gnu::noinline]] bool touch(std::size_t n) {
    volatile char* buf = static_cast<volatile char*>(__builtin_alloca(n));
    for (std::size_t i = 0; i < n; i += 4096) {
        buf[i] = static_cast<char>(i / 4096);
    }
    buf[n - 1] = 7;
    return buf[n - 1] == 7 && buf[0] == 0;
}

std::atomic<int> probes_exited{0};

void probe_main(void* p) {
    auto* pr = static_cast<probe*>(p);
    pr->id = std::this_thread::get_id();
    pr->slot_at_start = gp::os::tls_get();
    pr->bounds = gp::os::stack_bounds(pr->lo, pr->hi);
    pr->frame = frame_here();
    // Most of the stack that was asked for can be used (the thread's own data and the frames
    // of the runtime take some).
    pr->touched =
        pr->asked <= (std::size_t(128) << 10) || touch(pr->asked - (std::size_t(128) << 10));
    std::atomic<bool>* hold = pr->hold;
    pr->done.store(true, std::memory_order_release);
    // After done, the thread only touches hold, and then probes_exited: the test waits for
    // probes_exited before it gives its probes (and hold) up.
    while (hold != nullptr && !hold->load(std::memory_order_acquire)) {
        std::this_thread::yield();
    }
    probes_exited++;
}

void test_thread_start() {
    int started = 0;
    auto start = [&started](probe& pr, std::size_t size) {
        int rc = gp::os::thread_start(probe_main, &pr, size);
        if (rc == 0) {
            started++;
        }
        return rc;
    };
    // The thread runs fn(arg) on another thread, with a null slot, and has bounds.
    for (std::size_t size : {std::size_t(1), std::size_t(64) << 10, std::size_t(256) << 10,
                             std::size_t(1) << 20, std::size_t(8) << 20}) {
        probe pr;
        pr.asked = size;
        CHECK_EQ(start(pr, size), 0);
        CHECK(wait_for([&] { return pr.done.load(std::memory_order_acquire); }));
        CHECK(pr.id != std::this_thread::get_id());
        CHECK(pr.slot_at_start == nullptr);
        CHECK(pr.bounds);
        if (pr.bounds) {
            CHECK(pr.lo < pr.frame && pr.frame < pr.hi);
            // At least the size asked for, when that is more than the OS's minimum (it rounds
            // up, and may give a cached stack of an earlier thread, which is bigger).
            CHECK(pr.hi - pr.lo >= std::min(size, std::size_t(64) << 10));
            if (size >= (std::size_t(1) << 20)) {
                CHECK(pr.hi - pr.lo >= size);
            }
        }
        CHECK(pr.touched);
    }

    // Many threads at once, each with its own argument.
    // They all run at the same time (none ends before the last one has started).
    constexpr int n = 64;
    std::atomic<bool> hold{false};
    std::vector<probe> probes(n);
    for (int i = 0; i < n; i++) {
        probes[static_cast<std::size_t>(i)].hold = &hold;
        CHECK_EQ(start(probes[static_cast<std::size_t>(i)], std::size_t(256) << 10), 0);
    }
    std::set<std::thread::id> ids;
    for (probe& pr : probes) {
        CHECK(wait_for([&] { return pr.done.load(std::memory_order_acquire); }));
        ids.insert(pr.id);
    }
    CHECK_EQ(ids.size(), std::size_t(n));
    hold.store(true, std::memory_order_release);

    // A size that no OS can give is an error, and the function doesn't run.
    probe none;
    CHECK(gp::os::thread_start(probe_main, &none, std::size_t(1) << 60) != 0);
    CHECK(gp::os::thread_start(probe_main, &none, SIZE_MAX) != 0);
    CHECK(gp::os::thread_start(probe_main, &none, SIZE_MAX - 100) != 0);
    // (A thread that did run would have set done; the next thread_start's wait gives it time.)
    probe after;
    after.asked = 1;
    CHECK_EQ(start(after, 1), 0);
    CHECK(wait_for([&] { return after.done.load(std::memory_order_acquire); }));
    CHECK(!none.done.load());
    CHECK(wait_for([&] { return probes_exited.load() == started; }));
}

struct tls_probe {
    int index = 0;
    std::atomic<int>* ready = nullptr;
    std::atomic<bool>* go_on = nullptr;
    std::atomic<bool> done{false};
    bool ok = false;
};

void tls_main(void* p) {
    auto* t = static_cast<tls_probe*>(p);
    bool ok = gp::os::tls_get() == nullptr;
    gp::os::tls_set(t);
    // Every thread has set its slot before any of them reads it back.
    t->ready->fetch_add(1);
    while (!t->go_on->load(std::memory_order_acquire)) {
        std::this_thread::yield();
    }
    ok = ok && gp::os::tls_get() == t;
    gp::os::tls_set(nullptr);
    ok = ok && gp::os::tls_get() == nullptr;
    t->ok = ok;
    t->done.store(true, std::memory_order_release);
}

void test_tls() {
    // What the main thread's slot holds, no other thread's change. (The slot may hold something
    // already: with the goroutine runtime, the main goroutine's record. It is put back.)
    void* saved = gp::os::tls_get();
    int main_value = 0;
    void* mine = &main_value;
    gp::os::tls_set(mine);
    constexpr int n = 16;
    std::atomic<int> ready{0};
    std::atomic<bool> go_on{false};
    std::vector<std::unique_ptr<tls_probe>> probes;
    for (int i = 0; i < n; i++) {
        probes.push_back(std::make_unique<tls_probe>());
        probes.back()->index = i;
        probes.back()->ready = &ready;
        probes.back()->go_on = &go_on;
        CHECK_EQ(gp::os::thread_start(tls_main, probes.back().get(), std::size_t(256) << 10), 0);
    }
    CHECK(wait_for([&] { return ready.load() == n; }));
    CHECK(gp::os::tls_get() == mine);
    go_on.store(true, std::memory_order_release);
    for (auto& p : probes) {
        CHECK(wait_for([&] { return p->done.load(std::memory_order_acquire); }));
        CHECK(p->ok);
    }
    CHECK(gp::os::tls_get() == mine);
    gp::os::tls_set(saved);
    // A thread of the C++ library has an empty slot, and no stack bounds.
    bool slot_null = false;
    bool bounds = true;
    std::thread t([&] {
        uintptr_t lo = 0;
        uintptr_t hi = 0;
        slot_null = gp::os::tls_get() == nullptr;
        bounds = gp::os::stack_bounds(lo, hi);
    });
    t.join();
    CHECK(slot_null);
    CHECK(!bounds);
}

void test_main_stack() {
    uintptr_t lo = 0;
    uintptr_t hi = 0;
    CHECK(gp::os::stack_bounds(lo, hi));
    uintptr_t frame = frame_here();
    CHECK(lo < frame && frame < hi);
    CHECK(hi - lo >= (std::size_t(64) << 10));
#ifdef __linux__
    // os::start set the soft RLIMIT_STACK to 1 GiB, Go's limit (or the hard limit when that is
    // lower; also from unlimited), and the bounds are at most that: less only when exec put a
    // mapping closer (without ASLR, about 127 MiB below the top).
    auto [soft, hard, err] = gp::syscall::sys_getrlimit(3); // RLIMIT_STACK
    CHECK_EQ(err, int64_t(0));
    int64_t want = int64_t(1) << 30;
    if (hard >= 0 && hard < want) {
        want = hard;
    }
    CHECK_EQ(soft, want);
    CHECK(hi - lo <= uintptr_t(want));
    // The kernel keeps at least 128 MiB between the stack and mmap_base: 100 MiB fit always.
    CHECK(hi - lo >= std::min(uintptr_t(want), uintptr_t(100) << 20));
    // A change of the limit through the runtime (syscall.Setrlimit) moves the bounds, and back.
    if (want >= (int64_t(64) << 20)) {
        CHECK_EQ(gp::syscall::sys_setrlimit(3, int64_t(32) << 20, hard), int64_t(0));
        uintptr_t lo2 = 0;
        uintptr_t hi2 = 0;
        CHECK(gp::os::stack_bounds(lo2, hi2));
        CHECK_EQ(hi2, hi);
        CHECK_EQ(hi2 - lo2, uintptr_t(32) << 20);
        CHECK_EQ(gp::syscall::sys_setrlimit(3, soft, hard), int64_t(0));
        CHECK(gp::os::stack_bounds(lo2, hi2));
        CHECK_EQ(lo2, lo);
    }
#else
    CHECK(hi - lo <= (std::size_t(1) << 40));
#endif
}

// The hook can be set and read back, and the one that run_main set is there at first.
void test_hook_swap() {
    auto h1 = [](const gp::os::stack_fault&) noexcept {};
    auto h2 = [](const gp::os::stack_fault&) noexcept {};
    gp::os::stack_overflow_hook first = gp::os::set_stack_overflow_hook(h1);
    CHECK(gp::os::set_stack_overflow_hook(h2) == h1);
    CHECK(gp::os::set_stack_overflow_hook(first) == h2);
}

// Many threads start and end in a row (the races between the start of a thread and the end of
// another are for the sanitizers to see).
void churn_main(void* p) {
    static_cast<std::atomic<int>*>(p)->fetch_add(1);
}

void test_churn() {
    constexpr int n = 2000;
    static std::atomic<int> ran{0};
    ran = 0;
    for (int i = 0; i < n; i++) {
        CHECK_EQ(gp::os::thread_start(churn_main, &ran, std::size_t(128) << 10), 0);
        if (i % 100 == 99) {
            // Don't pile up thousands of threads: let most of them end.
            CHECK(wait_for([&] { return ran.load() > i - 50; }));
        }
    }
    CHECK(wait_for([&] { return ran.load() == n; }));
}

// Deep recursion that fits in the stack goes through (no false stack overflow), on the main
// thread and on a thread of thread_start: 3 MiB of frames on an 8 MiB stack.
template <std::size_t N>
[[gnu::noinline]] int64_t recurse_fit(int64_t depth) {
    volatile char pad[N];
    pad[0] = 1;
    pad[N - 1] = 2;
    int64_t r = depth == 0 ? 0 : recurse_fit<N>(depth - 1);
    asm volatile("" ::: "memory");
    return r + pad[0] + pad[N - 1] - 2;
}

struct fit_result {
    std::atomic<int64_t> value{-1};
};

void fit_main(void* p) {
    static_cast<fit_result*>(p)->value = recurse_fit<64 << 10>(48);
}

void test_deep_recursion_fits() {
    CHECK_EQ(recurse_fit<64 << 10>(40), int64_t(41));
    fit_result r;
    CHECK_EQ(gp::os::thread_start(fit_main, &r, std::size_t(8) << 20), 0);
    CHECK(wait_for([&] { return r.value.load() != -1; }));
    CHECK_EQ(r.value.load(), int64_t(49));
}

void run_unit_tests() {
    test_main_stack();
    test_thread_start();
    test_tls();
    test_hook_swap();
    test_churn();
    test_deep_recursion_fits();
    rt_test::threads_ended();
    if (failures > 0) {
        say("FAIL: " + std::to_string(failures.load()) + " of " + std::to_string(checks.load()) +
            " checks\n");
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks.load()));
}

// --- the cases that end the process (thread_test.go checks what they print) ---

// block_forever blocks the calling thread for good: the case ends the process some other way.
[[noreturn]] void block_forever() {
    for (;;) {
        std::this_thread::sleep_for(1h);
    }
}

// Set by a case when the threads it made are all there.
std::atomic<bool> guard_go{false};

// Never set: it gives the compilers' infinite-recursion warnings a way out.
volatile bool stop_recursing = false;

// A frame with a pad of N bytes that the recursion never leaves.
template <std::size_t N>
[[gnu::noinline]] int64_t recurse_forever(int64_t depth) {
    if (stop_recursing) {
        return 0;
    }
    volatile char pad[N];
    pad[0] = static_cast<char>(depth);
    pad[N - 1] = 1;
    int64_t r = recurse_forever<N>(depth + 1);
    asm volatile("" ::: "memory");
    return r + pad[0] + pad[N - 1];
}

// The bodies of threads that overflow: each prints a line first, so the test sees that output
// is not lost.
void frames_small(void*) {
    gp::println("thread runs"_gs);
    recurse_forever<128>(0);
}

// The same without the line: the threads of a case that overflows several at once start in any
// order, and a line from one may come after the report of another.
void frames_quiet(void*) {
    recurse_forever<128>(0);
}

void frames_big(void*) {
    gp::println("thread runs"_gs);
    // Frames that are most of the way to the guard: the first access past the end of the stack
    // is in the guard, not in some other mapping.
    recurse_forever<100000>(0);
}

// A store to an address that nothing maps (not null: UBSan would stop at that): a fault that
// isn't a stack overflow keeps the action that was there before.
volatile uintptr_t wild_address = 64; // read at run time: a constant address warns at -O2
[[gnu::noinline]] void wild_store() {
    volatile int* p = reinterpret_cast<volatile int*>(wild_address);
    *p = 1;
}

void wild_thread(void*) {
    wild_store();
}

// The hook case: a hook of the test, which says what it was given.
std::atomic<uintptr_t> expect_lo{0};
std::atomic<uintptr_t> expect_hi{0};

void test_hook(const gp::os::stack_fault& f) noexcept {
    bool in_stack =
        f.gp_lo < f.gp_hi && f.gp_sp < f.gp_hi && f.gp_sp + (std::size_t(1) << 20) >= f.gp_lo;
    bool same =
        expect_lo.load() == 0 || (f.gp_lo == expect_lo.load() && f.gp_hi == expect_hi.load());
    const char* msg = in_stack && same ? "hook: sp near the stack, bounds as stack_bounds\n"
                                       : "hook: WRONG bounds\n";
    std::string_view m(msg);
    gp::os::write(gp::os::stdout_fd, m.data(), m.size());
    gp::os::exit_now(7);
}

void hook_body(void*) {
    uintptr_t lo = 0;
    uintptr_t hi = 0;
    gp::os::stack_bounds(lo, hi);
    expect_lo = lo;
    expect_hi = hi;
    recurse_forever<128>(0);
}

// A write into the guard below a thread's stack: it faults, and it is not an overflow (the stack
// pointer is far away), so it is a crash without Go's message. The test must be able to tell a
// guard of 256 KiB from one of a few KiB, so it needs a thread X with another thread's stack Y
// right below it: with a small guard the address 128 KiB below X's stack lies in Y and the write
// succeeds. The kernel places stacks from the top down, but not always next to each other, so
// guard_layout starts threads (1 MiB stacks), reads each one's bounds, and picks the pair
// where Y ends at most 320 KiB below the start of X (the guard of 256 KiB or less, and no room
// for another mapping); with none, it starts more threads, and gives up (exit code 3, which the Go
// side treats as "layout not found": a retry, then a skip, never a pass).
constexpr int guard_threads = 8;
constexpr int guard_rounds = 6;
constexpr int guard_layout_missing = 3;
constexpr std::size_t guard_probe_below = std::size_t(128) << 10;

struct guard_slot {
    std::atomic<bool> ready{false};
    uintptr_t lo = 0;
    uintptr_t hi = 0;
};
guard_slot guard_slots[guard_threads * guard_rounds];
std::atomic<int> guard_writer{-1}; // the index of the thread that writes

void guard_thread(void* p) {
    int i = static_cast<int>(reinterpret_cast<intptr_t>(p));
    guard_slot& me = guard_slots[i];
    gp::os::stack_bounds(me.lo, me.hi);
    me.ready.store(true, std::memory_order_release);
    while (guard_writer.load(std::memory_order_acquire) != i) {
        std::this_thread::sleep_for(1ms);
    }
    volatile char* p2 = reinterpret_cast<volatile char*>(me.lo - guard_probe_below);
    *p2 = 1;
    // With a guard that is too small the write went into another thread's stack.
    gp::println("the write below the stack did not fault"_gs);
    gp::os::exit_now(4);
}

[[noreturn]] void guard_write_case() {
    int started = 0;
    for (int round = 0; round < guard_rounds; round++) {
        for (int k = 0; k < guard_threads; k++, started++) {
            if (gp::os::thread_start(guard_thread, reinterpret_cast<void*>(intptr_t(started)),
                                     std::size_t(1) << 20) != 0) {
                gp::os::exit(3);
            }
        }
        for (int i = 0; i < started; i++) {
            while (!guard_slots[i].ready.load(std::memory_order_acquire)) {
                std::this_thread::sleep_for(1ms);
            }
        }
        for (int x = 0; x < started; x++) {
            for (int y = 0; y < started; y++) {
                if (y != x && guard_slots[y].hi <= guard_slots[x].lo &&
                    guard_slots[x].lo - guard_slots[y].hi <= (std::size_t(320) << 10) &&
                    guard_slots[y].hi - guard_slots[y].lo >= (std::size_t(1) << 20)) {
                    guard_writer.store(x, std::memory_order_release);
                    block_forever();
                }
            }
        }
    }
    gp::println("layout not found"_gs);
    gp::os::exit_now(guard_layout_missing);
}

// A write to an address that nothing maps, from a frame at the very end of the stack: the
// stack pointer is near the end, the address is not, so it is not an overflow either.
[[gnu::noinline]] void deep_then_wild(uintptr_t lo, int depth) {
    volatile char pad[4096];
    pad[0] = static_cast<char>(depth);
    asm volatile("" ::"r"(&pad[0]) : "memory");
    if (stop_recursing) {
        return;
    }
#ifdef _WIN32
    // Windows keeps the last 64 KiB (the stack guarantee, SetThreadStackGuarantee) and a page for
    // the exception frame of an overflow: a frame in there is a stack overflow, as the report of
    // the real Windows PC showed. Near the end is just above that zone.
    constexpr std::size_t near_end = std::size_t(100) << 10;
#else
    constexpr std::size_t near_end = std::size_t(64) << 10;
#endif
    if (frame_here() < lo + near_end) {
        wild_store();
    } else {
        deep_then_wild(lo, depth + 1);
    }
    asm volatile("" ::: "memory");
}

void wild_near_end_thread(void*) {
    uintptr_t lo = 0;
    uintptr_t hi = 0;
    gp::os::stack_bounds(lo, hi);
    deep_then_wild(lo, 0);
}

// raise(SIGSEGV) is not a fault of the CPU: the handler must hand it back, and the program dies
// of the signal.
void raise_thread(void*) {
    std::raise(SIGSEGV);
}

// A thread with a 1 MiB stack and a frame of 4 MB, with other threads next to it: the frame is
// probed page by page (-fstack-clash-protection), so the first access past the end of the stack
// is in the guard, not in the stack of another thread.
void held_main(void*) {
    block_forever();
}

void huge_frame(void*) {
    while (!guard_go.load(std::memory_order_acquire)) {
        std::this_thread::yield();
    }
    gp::println("thread runs"_gs);
    recurse_forever<4000000>(0);
}

// overflow_in_thread starts a thread that runs body on a stack of stack bytes, and blocks.
[[noreturn]] void in_thread(std::size_t stack, void (*body)(void*)) {
    if (gp::os::thread_start(body, nullptr, stack) != 0) {
        gp::os::exit(3);
    }
    block_forever();
}

// The overflow of the main thread at its real limit, without the recursion: a write just above the
// lowest address of the main thread's stack works (the kernel grows the mapping down to it, a page
// is touched), and with the stack pointer moved below that address a write is the overflow that
// a recursion would end with. So the test touches two pages, not the 1 GiB that the limit allows.
// It prints the size of the stack first ("stack N"), then Go's report comes.
[[noreturn]] void overflow_main_far() {
    uintptr_t lo = 0;
    uintptr_t hi = 0;
    if (!gp::os::stack_bounds(lo, hi)) {
        gp::println("no bounds"_gs);
        gp::os::exit(3);
    }
    gp::println(gp::string::gp_copy("stack " + std::to_string(hi - lo)));
    volatile char* ok = reinterpret_cast<volatile char*>(lo + (std::size_t(16) << 10));
    *ok = 1;
#if defined(__linux__) && defined(__x86_64__)
    uintptr_t sp = lo - 256;
    asm volatile("movq %0, %%rsp\n\tmovq $0, (%%rsp)\n\t" ::"r"(sp) : "memory");
#elif defined(__linux__) && defined(__aarch64__)
    uintptr_t sp = lo - 256;
    asm volatile("mov sp, %0\n\tstr xzr, [sp]\n\t" ::"r"(sp) : "memory");
#else
    // (Windows grows a stack through its guard page: a write far below it is no overflow there.)
    gp::println("not on this system"_gs);
    gp::os::exit(3);
#endif
    __builtin_trap();
}

// Faults on the main thread just below its stack pointer that are no stack overflow: they keep the
// old action (the crash). exec_stack runs code from a buffer on the stack (not executable: the
// fault is the instruction fetch), write_ro_below_sp writes to a read-only page mapped 256 KiB
// below the stack pointer (in the stack's growth area, gigabytes above its end).
[[gnu::noinline]] void exec_stack() {
    unsigned char buf[64];
    for (auto& b : buf) {
        b = 0xc3; // ret
    }
    asm volatile("" ::"r"(buf) : "memory");
    auto f = reinterpret_cast<void (*)()>(reinterpret_cast<uintptr_t>(&buf[0]));
    f();
}

[[gnu::noinline]] void write_ro_below_sp() {
    uintptr_t at = (frame_here() - (std::size_t(256) << 10)) & ~uintptr_t(4095);
    if (!gp::os::testing::map_readonly_page(at)) {
        gp::println("no page"_gs);
        gp::os::exit(3);
    }
    *reinterpret_cast<volatile char*>(at) = 1;
}

void run_case(const std::string& name) {
    // Some cases die of SIGSEGV on purpose: no core file (a core_pattern that pipes to
    // systemd-coredump holds a dying process for minutes on a loaded machine).
    gp::os::no_core_dump();
    if (name == "overflow-main") {
        gp::println("main runs"_gs);
        recurse_forever<128>(0);
    } else if (name == "overflow-main-far") {
        overflow_main_far();
    } else if (name == "overflow-thread") {
        in_thread(std::size_t(256) << 10, frames_small);
    } else if (name == "overflow-thread-default") {
        in_thread(std::size_t(8) << 20, frames_small);
    } else if (name == "overflow-thread-big-frames") {
        in_thread(std::size_t(1) << 20, frames_big);
    } else if (name == "overflow-thread-many") {
        // Four threads overflow at once: one report.
        for (int i = 0; i < 4; i++) {
            if (gp::os::thread_start(frames_quiet, nullptr, std::size_t(256) << 10) != 0) {
                gp::os::exit(3);
            }
        }
        block_forever();
    } else if (name == "overflow-hook-thread") {
        gp::os::set_stack_overflow_hook(test_hook);
        in_thread(std::size_t(512) << 10, hook_body);
    } else if (name == "overflow-hook-main") {
        gp::os::set_stack_overflow_hook(test_hook);
        hook_body(nullptr);
    } else if (name == "overflow-foreign") {
        // A thread of the C++ library has no bounds: its overflow is a plain crash.
        std::thread t([] { recurse_forever<128>(0); });
        t.join();
    } else if (name == "overflow-thread-huge-frame") {
        // The thread is made first and the others after it, so that they lie below its stack.
        if (gp::os::thread_start(huge_frame, nullptr, std::size_t(1) << 20) != 0) {
            gp::os::exit(3);
        }
        for (int i = 0; i < 6; i++) {
            if (gp::os::thread_start(held_main, nullptr, std::size_t(1) << 20) != 0) {
                gp::os::exit(3);
            }
        }
        guard_go.store(true, std::memory_order_release);
        block_forever();
    } else if (name == "guard-write-thread") {
        guard_write_case();
    } else if (name == "wild-near-end-thread") {
        in_thread(std::size_t(512) << 10, wild_near_end_thread);
    } else if (name == "raise-main") {
        std::raise(SIGSEGV);
    } else if (name == "raise-thread") {
        in_thread(std::size_t(256) << 10, raise_thread);
    } else if (name == "exec-stack-main") {
        exec_stack();
    } else if (name == "write-ro-below-sp-main") {
        write_ro_below_sp();
    } else if (name == "segv-main") {
        gp::println("main runs"_gs);
        wild_store();
    } else if (name == "segv-thread") {
        in_thread(std::size_t(256) << 10, wild_thread);
    } else {
        say("unknown case " + name + "\n");
        gp::os::exit(3);
    }
    gp::os::exit(0);
}

void tests_main() {
    const auto& args = gp::args();
    // runUnit passes "arg1" and "arg two"; a case is the only argument.
    if (args.size() == 2) {
        run_case(std::string(args[1].gp_view()));
    }
    run_unit_tests();
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
