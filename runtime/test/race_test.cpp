// The unit tests of the race detector's side of the runtime (gocpp/race.hpp,
// src/os/linux/race.cpp): a program built with ThreadSanitizer and gocpp/race_atomicity.hpp, as
// gocpp build -race builds it. runtime/race_test.go compiles this file with the runtime's .cpp
// files (with g++ and clang++) and runs it with a case name as the one argument: each case is a
// program of goroutines that either has a data race, which the detector must report with exit code
// 66 and Go's text, or has none, which must end with exit code 0 and print nothing on stderr.
// Nothing depends on timing: the detector checks happens-before, and the goroutines wait for each
// other only through channels that order nothing between the goroutines that race.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include <gocpp/race.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/chan.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/print.hpp>

#include <atomic>
#include <cstdint>
#include <cstdlib>
#include <string>

namespace {

int64_t plain = 0;

struct box {
    int64_t n = 0;
};

// both runs f on two goroutines and waits for both.
template <class F>
void both(F f) {
    auto done = gp::make_chan<int64_t>(2);
    for (int i = 0; i < 2; i++) {
        gp::go([f, done] {
            f();
            gp::send(done, int64_t(1));
        });
    }
    gp::recv(done);
    gp::recv(done);
}

void case_main(const std::string& name) {
    if (name == "race-global") {
        // Two goroutines write one variable.
        both([] {
            for (int i = 0; i < 100; i++) {
                plain++;
            }
        });
    } else if (name == "race-through-pointer") {
        // The same through a *T whose count both goroutines change: the counts order nothing, so
        // the race is found whatever the timing (race_atomicity.hpp).
        auto p = gp::new_<box>();
        both([p] {
            for (int i = 0; i < 100; i++) {
                auto q = p;
                q->n++;
            }
        });
    } else if (name == "quiet-handoff") {
        // A value written before a send is read after the receive.
        auto ch = gp::make_chan<int64_t>();
        auto p = gp::new_<box>();
        gp::go([ch, p] {
            p->n = 42;
            gp::send(ch, int64_t(1));
        });
        gp::recv(ch);
        if (p->n != 42) {
            gp::os::exit(3);
        }
    } else if (name == "quiet-pointers") {
        // Many goroutines copy and drop the same pointer, and read what it points to.
        auto p = gp::new_<box>();
        p->n = 7;
        auto done = gp::make_chan<int64_t>(8);
        for (int i = 0; i < 8; i++) {
            gp::go([p, done] {
                int64_t sum = 0;
                for (int j = 0; j < 200; j++) {
                    auto q = p;
                    sum += q->n;
                }
                gp::send(done, sum);
            });
        }
        int64_t total = 0;
        for (int i = 0; i < 8; i++) {
            total += gp::recv(done);
        }
        if (total != 8 * 200 * 7) {
            gp::os::exit(3);
        }
    } else if (name == "quiet-free") {
        // The last drop of a pointer frees the object in another goroutine than the one that wrote
        // it: the drops order the free after every use (the library's own edges, kept).
        auto done = gp::make_chan<int64_t>();
        {
            auto p = gp::new_<box>();
            gp::go([p, done] {
                p->n = 1;
                gp::send(done, int64_t(1));
            });
        }
        gp::recv(done);
    } else if (name == "race-then-exit") {
        // A race that is followed at once by the death of the process: a goroutine writes, main
        // writes after it (nothing orders them) and ends the process with std::_Exit. The report
        // must be on the screen before the end (the callback waits for the reporter), as Go's is.
        // (A relaxed flag tells main that the goroutine is done: it orders nothing for TSan.)
        static std::atomic<int> flag{0};
        gp::go([] {
            for (int i = 0; i < 100; i++) {
                plain++;
            }
            flag.store(1, std::memory_order_relaxed);
        });
        while (flag.load(std::memory_order_relaxed) == 0) {
        }
        for (int i = 0; i < 100; i++) {
            plain++; // races with the goroutine's writes: the detector finds it here
        }
        std::_Exit(7); // at once: no time for anything that is not done yet
    } else if (name == "reporter") {
        // The reporter is a process of the detector, started with the runtime (not a thread: it
        // must not use ThreadSanitizer's budget).
        if (gp::race::reporter_process() <= 0) {
            gp::os::exit(3);
        }
    }
}

const char* case_name = "reporter";

void tests_main() {
    case_main(case_name);
}

} // namespace

int main(int argc, char** argv) {
    if (argc > 1) {
        case_name = argv[1];
    }
    return gp::run_main(argc, argv, nullptr, &tests_main);
}
