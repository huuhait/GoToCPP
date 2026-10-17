// threads_ended() for the test programs of runtime/test/ (not embedded, not a header of the
// runtime): it waits until the process has no thread but the calling one.
//
// A thread that a test joined, or that ended on its own, can still be listed in /proc/self/task
// for a moment. A program that ends while a thread is in that state makes LeakSanitizer (which
// stops all threads at exit in the asan mode) print "Unable to get registers from thread" and
// sometimes exit with an error: seen in about 1 of 10 runs of a test with threads under load.
// So a test program that starts threads calls this before it ends. Where /proc/self/task is
// there (Linux) it is exact; elsewhere it is a short wait. It gives up after two minutes.
#pragma once

#include <chrono>
#include <cstddef>
#include <filesystem>
#include <system_error>
#include <thread>

namespace rt_test {

inline void threads_ended() {
    using namespace std::chrono_literals;
    std::error_code ec;
    if (!std::filesystem::exists("/proc/self/task", ec)) {
        std::this_thread::sleep_for(200ms);
        return;
    }
    auto end = std::chrono::steady_clock::now() + 2min;
    while (std::chrono::steady_clock::now() < end) {
        std::error_code e;
        std::size_t n = 0;
        for (auto it = std::filesystem::directory_iterator("/proc/self/task", e);
             !e && it != std::filesystem::directory_iterator(); it.increment(e)) {
            n++;
        }
        if (e || n <= 1) {
            return;
        }
        std::this_thread::sleep_for(200us);
    }
}

} // namespace rt_test
