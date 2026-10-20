// Deterministic close interleavings that the fmt/Go differential programs cannot arrange.
// No descriptor is actually closed: the callback records ownership of the deferred close.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <thread>
#include <vector>

namespace {
namespace d = gp::fmt::detail;
namespace tos = gp::os::testing;
int checks = 0;
int failures = 0;

#define CHECK(expr)                                                                                \
    do {                                                                                           \
        checks++;                                                                                  \
        if (!(expr)) {                                                                             \
            failures++;                                                                            \
            std::fprintf(stderr, "%s:%d: CHECK(%s) failed\n", __FILE__, __LINE__, #expr);          \
        }                                                                                          \
    } while (false)

bool wait_for(const std::atomic<int>& state, int value,
              std::chrono::milliseconds timeout = std::chrono::seconds(5)) {
    auto end = std::chrono::steady_clock::now() + timeout;
    while (state.load() < value && std::chrono::steady_clock::now() < end) {
        std::this_thread::yield();
    }
    return state.load() >= value;
}

void await(const std::atomic<int>& state, int value) {
    if (!wait_for(state, value)) {
        std::fprintf(stderr, "close test: timed out waiting for state %d\n", value);
        std::abort();
    }
}

std::atomic<int> snapshot_stage{0};
std::shared_ptr<void> held_snapshot(const void*) {
    snapshot_stage.store(1);
    await(snapshot_stage, 2);
    return {}; // os.Stdout was replaced by another file (nil here).
}

void test_snapshot_last() {
    d::stdout_gate.word.store(1);
    d::os_std stream;
    static int original;
    stream.v0 = &original;
    stream.load = held_snapshot;
    // The registration lives until process exit, so its mock writer must not retain stack refs.
    static std::atomic<int> closes{0};
    static std::atomic<bool> closed_before_write{false};
    static std::thread::id close_thread;
    static std::thread::id write_thread;
    d::register_os_stdout(stream, [](std::shared_ptr<void>, std::string_view) {
        write_thread = std::this_thread::get_id();
        closed_before_write.store(closes.load() == 1);
        return std::tuple<int64_t, gp::error>{0, gp::error()};
    });
    const auto registering_thread = std::this_thread::get_id();
    std::thread printer([] { d::print_slow(""); });
    const auto printing_thread = printer.get_id();
    await(snapshot_stage, 1); // the slow path is counted, but has not examined the snapshot
    CHECK(gp::fmt::defer_close(true, [&] {
        close_thread = std::this_thread::get_id();
        closes.fetch_add(1);
    }));
    CHECK(closes.load() == 0);
    snapshot_stage.store(2);
    printer.join();

    // Both retained callbacks run on the existing printer, not on their registering caller.
    CHECK(close_thread == printing_thread);
    CHECK(write_thread == printing_thread);
    CHECK(printing_thread != registering_thread);

    CHECK((d::stdout_gate.word.load() >> 1) == 0);
    CHECK(closes.load() == 1);
    CHECK(!d::pending_stdout.flag.load());
    CHECK(closed_before_write.load()); // the original is released before writing the replacement
    d::finish_close(true);
    CHECK(closes.load() == 1);
}

std::atomic<int> barrier_stage{0};
void held_barrier() noexcept {
    barrier_stage.store(1);
    await(barrier_stage, 2);
}

void test_barrier_ready() {
    d::stdout_gate.word.store(2); // a counted Print is about to leave
    auto old_hook = tos::set_barrier_hook(held_barrier);
    auto before = tos::barrier_calls();
    std::atomic<int> closes{0};
    std::atomic<bool> early{false};
    auto close = [&] {
        early.store(tos::barrier_calls() == before);
        closes.fetch_add(1);
    };
    std::thread closer([&] {
        if (!gp::fmt::defer_close(true, close)) {
            close(); // the real close hook closes itself on a false return
        }
    });
    await(barrier_stage, 1);
    CHECK(d::pending_stdout.flag.load());
    CHECK(tos::barrier_calls() == before);
    // Readiness must be protected throughout the OS barrier. This assertion also makes the
    // regression deterministic if the finishing thread is descheduled before finish_close.
    bool unprotected = d::pending_stdout.mu.try_lock();
    if (unprotected) {
        d::pending_stdout.mu.unlock();
    }
    CHECK(!unprotected);
    std::atomic<int> finishing{0};
    std::thread printer([&] {
        d::stdout_gate.word.fetch_sub(2);
        finishing.store(1);
        d::finish_close(true);
        finishing.store(2);
    });
    await(finishing, 1);
    // Give an incorrect finisher the opportunity to claim the callback while the barrier is
    // held. Correct finishers wait for readiness instead; every wait in the test is bounded.
    wait_for(finishing, 2, std::chrono::milliseconds(250));
    CHECK(closes.load() == 0);
    CHECK(!early.load());
    barrier_stage.store(2);
    closer.join();
    printer.join();
    tos::set_barrier_hook(old_hook);
    CHECK(tos::barrier_calls() > before);
    CHECK(closes.load() == 1);
    CHECK(!early.load());
    CHECK(!d::pending_stdout.flag.load());
    d::finish_close(true);
    CHECK(closes.load() == 1);
}

void test_counted_finishers() {
    for (bool out : {false, true}) {
        auto& gate = out ? d::stdout_gate : d::stdin_gate;
        auto& pending = out ? d::pending_stdout : d::pending_stdin;
        std::atomic<int> closes{0};
        gate.word.store(16); // eight counted calls
        CHECK(gp::fmt::defer_close(out, [&] { closes.fetch_add(1); }));
        CHECK(closes.load() == 0);
        std::vector<std::thread> printers;
        for (int i = 0; i < 8; i++) {
            printers.emplace_back([&] {
                gate.word.fetch_sub(2);
                d::finish_close(out);
            });
        }
        for (auto& t : printers) {
            t.join();
        }
        CHECK(closes.load() == 1);
        CHECK(!pending.flag.load());
        CHECK((gate.word.load() >> 1) == 0);
        d::finish_close(out);
        CHECK(closes.load() == 1);
        CHECK(!gp::fmt::defer_close(out, [&] { closes.fetch_add(1); }));
        CHECK(closes.load() == 1); // no users: the caller, not the callback, owns the close
    }
}

void test_slot_last() {
    auto* slot = d::register_print_slot();
    slot->inflight.store(1);
    std::atomic<int> closes{0};
    CHECK(gp::fmt::defer_close(true, [&] { closes.fetch_add(1); }));
    CHECK(closes.load() == 0);
    slot->inflight.store(0);
    d::finish_close(true);
    CHECK(closes.load() == 1);
    CHECK(!d::pending_stdout.flag.load());
}

void run_tests() {
    test_barrier_ready();
    test_counted_finishers();
    test_slot_last();
    test_snapshot_last(); // installs a fake os.Stdout; nothing prints through fmt after this
    if (failures != 0) {
        gp::os::exit(1);
    }
    std::printf("ok: %d checks\n", checks);
}
void no_init() {}
} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &no_init, &run_tests);
}
