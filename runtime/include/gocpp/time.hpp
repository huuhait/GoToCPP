// gocpp/time.hpp: the native functions of the standard package time (Phase 08 step 8.8).
//
// Go's time package is written in .gp (stdlib/time: Go's own format.go, time.go and zoneinfo.go);
// what needs the system or the runtime is here, and the .gp source calls these through
// //gocpp:native (ADR-027, ADR-041):
//
//     now()                         wall clock and monotonic clock (CLOCK_MONOTONIC), a few ns each
//     nano()                        the monotonic clock in nanoseconds: the clock of the timers
//     sleep(ns)                     time.Sleep: the goroutine waits on its own waiter with a
//     deadline start_chan, start_func,       the timers: NewTimer, NewTicker, AfterFunc, Stop and
//     Reset stop, reset, try_send call_once, getenv, files...   what zoneinfo.go needs (standard
//     C++ only: <cstdio>, <cstdlib>)
//
// The timers (src/timer.cpp). A timer is a record that hangs on a channel object, the channel of a
// time.Timer or time.Ticker (the one that the user receives from), or, for AfterFunc, a channel
// that nobody uses and that only carries the record. One thread keeps the armed timers in a map
// ordered by deadline and sleeps (a condition variable with a timeout) until the earliest; it
// starts when the first timer is armed, so a program that never uses a timer pays nothing, and it
// is not a goroutine: it is not counted, it never blocks the end of the program (the program ends
// with _exit). When a timer is due, the thread
//   - for a Timer or Ticker calls the Go function that was given (time.sendTime(c, due), which
//   does
//     a send on c that never blocks: the value is dropped when the channel's one-element buffer is
//     full, which is how a Ticker drops the ticks of a slow receiver);
//   - for AfterFunc starts a goroutine that runs the function.
// Go 1.23's rules for timer channels (the reason for most of this file):
//   - the channel looks unbuffered: len and cap of it are 0 (the buffer of one element is an
//     implementation detail, as in Go's runtime);
//   - after Stop or Reset has returned, no value of an earlier setting can be received: both take
//   the
//     timer's lock, which the thread holds while a send runs, count a send that has not started as
//     stopped, and remove the value that is waiting in the channel (a value that was there counts
//     as "the timer was active", so Stop returns true until the program has received the value);
//   - a Timer or Ticker that nothing holds is freed even if it is armed: the thread holds the
//   channel
//     weakly, and the record leaves the map when the channel dies (reference counting, no
//     collector). An AfterFunc timer is held until it has fired or is stopped, as in Go.
// `select` needs nothing special: a timer channel is an ordinary channel (ADR-041).
//
// Standard C++ only (the portable runtime: nothing here calls the operating system by name).
#pragma once

#include <gocpp/chan.hpp>
#include <gocpp/func.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <memory>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp::detail {

// The timer thread's interface to the channel object (src/timer.cpp, the part that needs
// chan_core).
struct timer_rec;

// start_chan_timer arms a timer on the channel c: at the monotonic time `when`, then every `period`
// nanoseconds if period > 0, it calls send(c, due) (due: the deadline that the call is for, in the
// nano() clock; a ticker that was late gets the deadline on its grid, not the time of the call).
// The heap holds the channel weakly.
void start_chan_timer(const std::shared_ptr<chan_core>& c, int64_t when, int64_t period,
                      gp::func<void(std::shared_ptr<chan_core>, int64_t)> send);

// start_func_timer arms an AfterFunc timer on c, which carries the record: at `when`, a goroutine
// runs f. The heap holds c strongly until the timer has fired or is stopped.
void start_func_timer(const std::shared_ptr<chan_core>& c, int64_t when, gp::func<void()> f);

// stop_timer and reset_timer are Timer.Stop and Timer.Reset, with Go's results (see the top).
bool stop_timer(chan_core* c);
bool reset_timer(const std::shared_ptr<chan_core>& c, int64_t when, int64_t period);

// timer_try_send sends *v on c without blocking: false when the channel has no room and no
// receiver. v is moved from when it returns true.
bool timer_try_send(chan_core* c, void* v);

// timers_armed is the number of timers that are armed now. timers_can_wake says whether an armed
// timer could still wake a goroutine that waits: an AfterFunc timer, or a channel timer that a
// goroutine is blocked on (a timer channel that nobody waits on does not count: Go 1.23's deadlock
// rule, which step 8.11 uses). timers_shutdown stops and joins the timer thread (the next timer
// starts it again): for tests, which end with threads_ended().
std::size_t timers_armed() noexcept;
bool timers_can_wake();
void timers_shutdown();

} // namespace gp::detail

namespace gp::time {

// The clocks. now() is (seconds and nanoseconds since the Unix epoch, the monotonic clock in
// nanoseconds); nano() is the monotonic clock.
std::tuple<int64_t, int32_t, int64_t> now() noexcept;
int64_t nano() noexcept;

// sleep(ns): time.Sleep. The goroutine is not counted as blocked (it will wake by itself, so it is
// not "asleep" for the deadlock detector).
void sleep(int64_t ns);

// call_once(id, f) runs f once for each id (0 to 2), however many goroutines call it at the same
// time; the others wait until it has returned. If f panics, the next call tries again.
void call_once(int64_t id, const gp::func<void()>& f);

// getenv(key) is (the value, whether it is set). errno_text(e) is the text of an errno, Go's
// (lower case) spelling.
std::tuple<gp::string, bool> getenv(const gp::string& key);
gp::string errno_text(int64_t e);

// Files of the time zone database, through the C library's FILE: open_file returns (a handle, 0)
// or (0, errno); read_file reads into buf and returns (n, 0), (0, 0) at the end of the file or (0,
// errno); seek_file returns 0 or an errno (whence 0, 1, 2: the start, here, the end).
std::tuple<int64_t, int64_t> open_file(const gp::string& name);
std::tuple<int64_t, int64_t> read_file(int64_t fd, const gp::slice<uint8_t>& buf);
void close_file(int64_t fd);
int64_t seek_file(int64_t fd, int64_t off, int64_t whence);

// The timers, called with the channel (a gp::chan<Time> for Go's Time, which the runtime does not
// know, so these are templates) and, for the callback, the Go function that the package gave:
// void(chan Time, int64 due) for a channel timer, void() for AfterFunc.
template <typename T>
void start_chan(const gp::chan<T>& c, int64_t when, int64_t period,
                std::type_identity_t<gp::func<void(gp::chan<T>, int64_t)>> f) {
    detail::start_chan_timer(
        c.gp_shared(), when, period,
        gp::func<void(std::shared_ptr<detail::chan_core>, int64_t)>(
            [f = std::move(f)](std::shared_ptr<detail::chan_core> core, int64_t due) {
                f(gp::chan<T>(std::move(core)), due);
            }));
}

template <typename T>
void start_func(const gp::chan<T>& c, int64_t when, gp::func<void()> f) {
    detail::start_func_timer(c.gp_shared(), when, std::move(f));
}

template <typename T>
bool stop(const gp::chan<T>& c) {
    return detail::stop_timer(c.gp_core());
}

template <typename T>
bool reset(const gp::chan<T>& c, int64_t when, int64_t period) {
    return detail::reset_timer(c.gp_shared(), when, period);
}

template <typename T>
bool try_send(const gp::chan<T>& c, std::type_identity_t<T> v) {
    return detail::timer_try_send(c.gp_core(), &v);
}

} // namespace gp::time
