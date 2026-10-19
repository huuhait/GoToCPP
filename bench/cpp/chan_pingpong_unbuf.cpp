// bench: chan_pingpong_unbuf (C++ twin of bench/programs/runtime/chan_pingpong_unbuf.gp)
// Two threads hand a counter back and forth, each hand-off one release/acquire of a binary
// semaphore (a futex pair on Linux): the cheapest portable C++20 rendezvous, so this twin is not
// slowed down by a shared mutex and condition variable (an earlier version used those and
// flattered gocpp).
#include <cstdint>
#include <cstdio>
#include <semaphore>
#include <thread>

int main() {
    std::binary_semaphore toWorker{0}, toMain{0};
    int64_t slot = 0;
    bool stop = false;
    std::thread worker([&] {
        for (;;) {
            toWorker.acquire();
            if (stop)
                return;
            slot += 1;
            toMain.release();
        }
    });
    int64_t v = 0;
    for (int64_t i = 0; i < BENCH_N; i++) {
        slot = v;
        toWorker.release();
        toMain.acquire();
        v = slot;
    }
    stop = true;
    toWorker.release();
    worker.join();
    std::printf("%lld\n", (long long)v);
}
