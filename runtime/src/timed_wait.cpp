// The count of the waits that want fine timer ticks (src/timed_wait.hpp, ADR-132). Standard C++
// only: the request itself is os::fine_timer.
#include <gocpp/nofortify.h>
#include "timed_wait.hpp"

#include <mutex>

namespace gp::detail {

namespace {

// The count is under a mutex, not atomic: a request to the OS made when the count leaves 0 must
// not pass the release made when it came back to 0 in another thread. It is never destroyed (a
// wait may end while the process does).
struct fine_state {
    std::mutex mu;
    std::size_t holders = 0;
};

fine_state& fine = *new fine_state;

} // namespace

void fine_timer_acquire() noexcept {
    std::lock_guard<std::mutex> lk(fine.mu);
    if (fine.holders++ == 0) {
        os::fine_timer(true);
    }
}

void fine_timer_release() noexcept {
    std::lock_guard<std::mutex> lk(fine.mu);
    if (--fine.holders == 0) {
        os::fine_timer(false);
    }
}

std::size_t fine_timer_holders() noexcept {
    std::lock_guard<std::mutex> lk(fine.mu);
    return fine.holders;
}

} // namespace gp::detail
