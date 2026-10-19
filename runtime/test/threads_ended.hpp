// threads_ended() for the test programs of runtime/test/ (not embedded, not a header of the
// runtime): it waits until the process has no thread but the calling one.
//
// A thread that a test joined, or that ended on its own, can still be listed in /proc/self/task
// for a moment. A program that ends while a thread is in that state makes LeakSanitizer (which
// stops all threads at exit in the asan mode) print "Unable to get registers from thread" and
// sometimes exit with an error: seen in about 1 of 10 runs of a test with threads under load.
// So a test program that starts threads calls this before it ends. Where /proc/self/task is
// there (Linux) it is exact; elsewhere it is a short wait. It gives up after two minutes. The
// threads of goroutines that wait for the next `go` statement (thread reuse) are ended first. A
// thread that the process had before main (ThreadSanitizer's own) is not waited for.
#pragma once

#include <gocpp/goroutine.hpp>
#include <gocpp/race.hpp>

#include <algorithm>
#include <chrono>
#include <cstddef>
#include <filesystem>
#include <system_error>
#include <thread>

namespace rt_test {

// The number of threads of the process now (Linux), or 0 when it can't be told.
inline std::size_t task_count() {
    std::error_code e;
    std::size_t n = 0;
    for (auto it = std::filesystem::directory_iterator("/proc/self/task", e);
         !e && it != std::filesystem::directory_iterator(); it.increment(e)) {
        n++;
    }
    return e ? 0 : n;
}

// The threads the process has when it has none of its own: the main thread, and under
// ThreadSanitizer its background thread, which it starts at the first thread that the program
// creates and keeps for the whole run. So the baseline is taken after a thread has been created
// and has ended. (Initialized before main in every test program that includes this header.)
inline std::size_t find_baseline() {
    using namespace std::chrono_literals;
    std::thread([] {}).join();
    std::size_t low = task_count();
    auto end = std::chrono::steady_clock::now() + 200ms;
    while (std::chrono::steady_clock::now() < end && low > 1) {
        std::this_thread::sleep_for(200us);
        low = std::min(low, task_count());
    }
    return low;
}
inline const std::size_t baseline_threads = find_baseline();

inline void threads_ended() {
    using namespace std::chrono_literals;
    // The threads that goroutines left idle for the next `go` statement end now.
    gp::detail::stop_idle_threads();
    std::error_code ec;
    if (!std::filesystem::exists("/proc/self/task", ec)) {
        std::this_thread::sleep_for(200ms);
        return;
    }
    std::size_t base = (baseline_threads > 1 ? baseline_threads : 1);
    auto end = std::chrono::steady_clock::now() + 2min;
    while (std::chrono::steady_clock::now() < end) {
        std::size_t n = task_count();
        if (n == 0 || n <= base) {
            return;
        }
        std::this_thread::sleep_for(200us);
    }
}

} // namespace rt_test
