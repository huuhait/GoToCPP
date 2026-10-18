// A helper header for chan_cpp_values.gp (step 8.2/8.3): a C++ object that can't be copied (it
// holds an atomic), functions that goroutines call, and a function that waits for a count.
#pragma once

#include <atomic>
#include <string>
#include <thread>

struct Counter {
    std::atomic<int> n{0};
    void add(int k) { n += k; }
    int get() const { return n.load(); }
};

inline int twice(int x) { return 2 * x; }
inline std::string shout(const std::string& s) { return s + "!"; }
inline void add_to(Counter* c, int k) { c->add(k); }

// wait_for returns when the counter has reached want (a goroutine in this test adds to it).
inline void wait_for(Counter* c, int want) {
    while (c->get() < want) {
        std::this_thread::yield();
    }
}
