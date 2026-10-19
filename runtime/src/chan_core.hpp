// The channel object (internal to the runtime: src/chan.cpp, and src/select.cpp in step 8.5).
// gocpp/chan.hpp has the handle, gp::chan<T>, and the declarations of the operations.
#pragma once

#include <gocpp/chan.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/waiter.hpp>

#include <atomic>
#include <cstddef>
#include <cstdint>
#include <memory>
#include <mutex>

namespace gp::detail {

struct sudog;
struct timer_rec; // the timer of a time.Timer or time.Ticker channel (src/timer.cpp)
struct chan_core;

// timer_observe is called before a receive (or a poll of a select) on a timer channel: if the
// timer's deadline has passed, the value is sent now, on the caller's thread, so that the receive
// sees what Go 1.23 shows (a timer that is due has fired as far as the program can tell, whether
// the timer thread has run yet or not). src/timer.cpp.
void timer_observe(chan_core* c);

// wake_cell is where a waker tells a parked goroutine that it is done: it sets done and fired
// (the entry that woke the goroutine) with waiter::gp_unpark_with, and the goroutine waits with
// waiter::gp_wait, so both are read and written under the waiter's lock. done is atomic too, for
// the spin that comes before the sleep (spin_wait). A select shares one cell among its entries.
struct wake_cell {
    std::atomic<bool> done{false};
    sudog* fired = nullptr;
};

// sudog is an entry in a channel's receive or send queue: one goroutine waiting for one
// operation (Go's sudog). It lives on the waiting goroutine's stack until the goroutine is woken
// (or, for a select, has dequeued its other entries), so the code that wakes it must not touch it
// after wake().
struct sudog {
    goroutine* g = nullptr; // the goroutine (its block_scope is entered before the entry is queued)
    waiter* w = nullptr;    // what the goroutine parks on
    wake_cell* cell = nullptr;
    void* elem = nullptr;  // a sender: the value to send (the receiver moves from it); a
                           // receiver: where the value goes (uninitialised storage, or null:
                           // drop the value)
    sudog* next = nullptr; // the queue (waitq)
    sudog* prev = nullptr;
    chan_core* c = nullptr;
    // A select's entry: the flag that all its entries share (0, then 1 when one of them is taken
    // off a queue to complete), and the case it is.
    std::atomic<uint32_t>* claim = nullptr;
    int case_index = 0;
    // A receiver: construct the zero value in elem when the channel is closed (false for the
    // receive of a range loop, which ends there and has no use for it).
    bool zero = true;
    // Set under the channel's lock before the wake-up: a receiver got a value (false: the channel
    // was closed), a sender's value was taken (false: the channel was closed, the sender panics).
    bool success = false;
};

// waitq is a queue of sudogs, first in, first out. Guarded by the channel's lock.
struct waitq {
    sudog* first = nullptr;
    sudog* last = nullptr;

    void enqueue(sudog* sg) noexcept {
        sg->next = nullptr;
        sg->prev = last;
        if (last != nullptr) {
            last->next = sg;
        } else {
            first = sg;
        }
        last = sg;
    }

    // remove takes sg out of the queue (it must be in it).
    void remove(sudog* sg) noexcept {
        if (sg->prev != nullptr) {
            sg->prev->next = sg->next;
        } else {
            first = sg->next;
        }
        if (sg->next != nullptr) {
            sg->next->prev = sg->prev;
        } else {
            last = sg->prev;
        }
        sg->next = sg->prev = nullptr;
    }

    // contains reports whether sg is in this queue (not taken off by a dequeue, which also unlinks
    // the entries it skips).
    bool contains(const sudog* sg) const noexcept { return sg->prev != nullptr || first == sg; }

    // dequeue takes the first entry that can complete off the queue and returns it, or null. An
    // entry of a select that another channel completed already (its claim flag is taken) is
    // dropped from the queue and skipped; one that can complete claims its select.
    sudog* dequeue() noexcept {
        while (sudog* sg = first) {
            remove(sg);
            if (sg->claim != nullptr) {
                uint32_t zero = 0;
                if (!sg->claim->compare_exchange_strong(zero, 1, std::memory_order_acq_rel)) {
                    continue;
                }
            }
            return sg;
        }
        return nullptr;
    }
};

// wake makes the goroutine of sg go on. Nothing may touch sg after it: the goroutine may be
// gone. (It reads the pieces first, and sets done under the waiter's lock, which the goroutine
// needs to see it, so it can't leave before wake is done with the waiter.)
inline void wake(sudog* sg) noexcept {
    waiter* w = sg->w;
    wake_cell* cell = sg->cell;
    unblock(*sg->g); // it is not blocked any more, from here on (num_blocked, step 8.11)
    w->gp_unpark_with([cell, sg] {
        cell->fired = sg;
        cell->done.store(true, std::memory_order_release);
    });
}

// spin_wait spins on cell.done for a little while before the goroutine goes to sleep: a wake-up
// that comes within a few microseconds (the other goroutine of a ping-pong, a producer that is
// running) then costs no system call on either side, where a sleeping thread costs a futex wake
// and a context switch, ten times as long. It does nothing on a machine with one CPU.
bool spin_wait(const wake_cell& cell) noexcept;

// park_cell blocks the calling goroutine until a waker has set cell.done. The caller has entered
// a block_scope (with the reason, as Go's traceback says it: "chan receive") before it queued its
// entry, so that a waker's unblock is never before the block_enter.
inline void park_cell(wake_cell& cell) {
    goroutine& g = this_goroutine();
    // (The wait below takes the waiter's lock even when the spin saw done: the waker sets done
    // under that lock and may still be inside it, and the goroutine must not go on, and end, before
    // it is out.)
    spin_wait(cell);
    g.park.gp_wait([&cell] { return cell.done.load(std::memory_order_relaxed); });
}

// The result of a receive that doesn't block.
enum class recv_status { not_ready, value, closed };

// The result of the poll of one channel under its lock (chan_core::send_locked, recv_locked):
// the operation could not complete, completed, or the channel is closed (a send must panic; a
// receive got the zero value).
enum class poll_status { not_ready, done, closed };

// chan_core is the channel object. It is created by chan_make and shared by the handles.
struct chan_core {
    chan_core(const elem_ops* ops, std::size_t cap);
    ~chan_core();
    chan_core(const chan_core&) = delete;
    chan_core& operator=(const chan_core&) = delete;

    // The operations; send and recv are chan_send and chan_recv. try_send does not block: false
    // when it would. try_recv does not block: not_ready when it would. (For select, step 8.5.)
    void send(void* src);
    bool try_send(void* src);
    bool recv(void* dst, bool zero = true);
    recv_status try_recv(void* dst);
    // try_recv without the timer's check (timer_observe): for the timer code itself.
    recv_status drain(void* dst);
    void close();

    std::size_t len() const noexcept { return count.load(std::memory_order_relaxed); }
    std::size_t cap() const noexcept { return capacity; }

    // The operations of one channel for a caller that holds mu already (the blocking and the
    // non-blocking forms above, and select, step 8.5, which holds the locks of all its channels):
    // each completes the operation if it can, without waiting. send_locked moves from src;
    // recv_locked constructs the value, or the zero value for a closed channel
    // (poll_status::closed), in dst (null: dropped). When the operation completed with a goroutine
    // that waits in a queue, that entry is stored in *w (success already set): the caller wakes it
    // with wake() after it has unlocked, never before.
    poll_status send_locked(void* src, sudog** w);
    poll_status recv_locked(void* dst, sudog** w, bool zero = true);

    // The state, for select (step 8.5), which locks several channels: everything below is
    // guarded by mu, except count, which len reads without it.
    std::mutex mu;
    waitq recvq;
    waitq sendq;
    bool closed = false;

    const elem_ops* ops;
    std::size_t capacity;
    unsigned char* buf = nullptr;
    std::size_t sendx = 0; // the slot of the next send
    std::size_t recvx = 0; // the slot of the next receive
    std::atomic<std::size_t> count{0};

    // The timer record of a channel that belongs to a time.Timer, Ticker or AfterFunc (step 8.8,
    // src/timer.cpp), null for every other channel. It is set once, before the channel is
    // shared. Such a channel has a buffer of one element, and len and cap say 0 (Go 1.23's
    // synchronous timer channels). The record leaves the timer thread's map when it is destroyed
    // with the channel.
    std::shared_ptr<timer_rec> timer;

private:
    bool send_impl(void* src, bool block);
    recv_status recv_impl(void* dst, bool block, bool zero);
    // (An empty type has no buffer: every slot is the one byte below, where nothing is stored.)
    void* slot(std::size_t i) const noexcept {
        return ops->empty ? const_cast<unsigned char*>(&empty_slot) : buf + i * ops->size;
    }
    unsigned char empty_slot = 0;
};

} // namespace gp::detail
