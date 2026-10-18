// gocpp/chan.hpp: Go's channels, gp::chan<T> (Phase 08, steps 8.3, 8.4 and 8.6).
//
// A channel is a reference to a shared channel object, as a pointer is: copying the handle shares
// the channel, a default handle is the nil channel, and the object lives as long as one handle
// does (reference counting, ADR-004). The object is Go's hchan (runtime/chan.go): a lock, a ring
// buffer, and two queues of goroutines that wait, receivers and senders.
//
// clang-format off
//     make(chan T)          gp::make_chan<T>()          an unbuffered channel
//     make(chan T, n)       gp::make_chan<T>(n)         n >= 0, else the panic "makechan: size out of range"
//     ch <- v               gp::send(ch, v)             a copy of v goes into the channel
//     <-ch                  gp::recv(ch)                the value
//     v, ok := <-ch         gp::recv2(ch)               std::tuple<T, bool>
//     close(ch)             gp::close(ch)
//     len(ch), cap(ch)      gp::len(ch), gp::cap(ch)
//     ch == ch2, ch == nil  == (the same channel object); a nil handle is a nil channel
// clang-format on
//
// The type of a channel is gp::chan<T> in all three directions: chan T, <-chan T and chan<- T
// are different Go types (different descriptors, gocpp/type.hpp; different %T), but the same C++
// type, so the conversion from chan T to a directional channel, which Go does implicitly, costs
// nothing, and the direction is checked by the type checker, not at run time. A named channel type
// (type Pipe chan int) is a gp::newtype over it (gocpp/newtype.hpp).
//
// What the operations do (the algorithms are Go's):
//   - send: on a closed channel it panics ("send on closed channel", also when the channel is
//     closed while the sender waits). If a receiver waits, the value goes straight into the
//     receiver's variable and the receiver wakes up. Else, if the buffer has room, it goes into the
//     buffer. Else the sender waits in the send queue, with a pointer to its value.
//   - receive: if a sender waits, the receiver takes the value: from the sender directly when the
//     channel is unbuffered, else from the head of the buffer, and the sender's value goes to the
//     tail of the buffer. Else, a value in the buffer is taken. Else, a closed channel gives the
//     zero value and false. Else the receiver waits in the receive queue.
//   - close: a nil channel panics ("close of nil channel"), a closed one too ("close of closed
//     channel"); else all receivers wake with the zero value and false, and all senders wake and
//     panic.
//   - A nil channel blocks the goroutine for ever on send and receive.
//   Each queue is first in, first out. The happens-before edges are Go's: the k-th receive on a
//   channel of capacity C happens before the (k+C)-th send completes, a send happens before the
//   matching receive completes, and the close happens before a receive that returns because of
//   it. The lock is a mutex of the C++ library, so a data race detector (ThreadSanitizer) sees
//   them.
//
// A goroutine that waits parks on the waiter of its goroutine record (gocpp/waiter.hpp). The
// entry it leaves in a queue is a sudog (src/chan_core.hpp, Go's sudog), on its own stack: it has
// the waiter, a pointer to where the value is, and a cell where whoever wakes it says "done". The
// entry of a goroutine that waits in a `select` (step 8.5) is the same record, one per case,
// that share one cell and one claim flag: dequeue skips an entry whose select was already won.
// So `select` (gocpp/select.hpp, src/select.cpp) adds the statement, not another kind of waiter.
// The channel object and its operations are in src/chan.cpp and src/chan_core.hpp, not in this
// header: gocpp/rt.hpp includes it, and <mutex> and <condition_variable> would add half a second to
// the compile of every generated file.
//
// Elements are moved and copied as C++ moves and copies them: the value that send takes is a
// copy that the call owns (a Go value is copied; a C++ opaque value is moved by the code that
// the emitter writes, gocpp/interop.hpp), and moves into the buffer or the receiver. Element
// types do not need a default constructor, unless the program receives from a closed channel:
// that needs the zero value. Zero-size types (chan struct{}) are ordinary elements of one byte.
//
// Like every runtime type, gp::chan has only operators and gp_-prefixed members. The
// detail namespace is the channel object itself.
#pragma once

#include <gocpp/hash.hpp>
#include <gocpp/goroutine.hpp>

#include <cstddef>
#include <cstdint>
#include <memory>
#include <new>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {

// elem_ops is what the channel object, which is not a template, needs to know about its element
// type: its size, and how to move, destroy and zero one. The functions are noexcept: a move
// that throws would leave a channel half changed (std::terminate).
struct elem_ops {
    std::size_t size;
    std::size_t align;
    void (*move_into)(void* dst, void* src) noexcept; // construct *dst from std::move(*src)
    void (*destroy)(void* p) noexcept;
    bool (*zero_into)(void* dst) noexcept; // construct a zero value; false: the type has none
    bool empty; // a type without data (struct{}): the buffer takes no memory, as in Go
};

template <typename T>
inline constexpr elem_ops elem_ops_of = {
    sizeof(T),
    alignof(T),
    [](void* dst, void* src) noexcept { ::new (dst) T(std::move(*static_cast<T*>(src))); },
    [](void* p) noexcept { static_cast<T*>(p)->~T(); },
    [](void* dst) noexcept -> bool {
        if constexpr (std::is_default_constructible_v<T>) {
            ::new (dst) T();
            return true;
        } else {
            return false;
        }
    },
    std::is_empty_v<T>,
};

// The channel object (src/chan_core.hpp) and its operations (src/chan.cpp). They work on
// elements through element pointers.
struct chan_core;

// chan_make creates the object for make(chan T, n): n elements of buffer. It panics with
// "makechan: size out of range" when n < 0 or the buffer would not fit in memory.
std::shared_ptr<chan_core> chan_make(const elem_ops* ops, int64_t n);

// chan_send sends the element *src, which the channel moves from: it blocks until the value is
// taken or fits in the buffer. The caller destroys *src afterwards, whether it was moved from or
// not. A closed channel panics.
void chan_send(chan_core* c, void* src);

// chan_recv receives into dst, uninitialised storage for an element (or null: the value is
// dropped), which is constructed there either way: with the value, or with the zero value when
// the channel is closed. It returns false for a closed channel, true for a value.
bool chan_recv(chan_core* c, void* dst);

// chan_recv_range is chan_recv for a range loop: a closed channel constructs nothing in dst (the
// loop ends, it has no use for a zero value, and the element type may have none).
bool chan_recv_range(chan_core* c, void* dst);

// chan_close closes the channel (panics for a closed one), chan_len and chan_cap are its length
// and capacity.
void chan_close(chan_core* c);
int64_t chan_len(const chan_core* c) noexcept;
int64_t chan_cap(const chan_core* c) noexcept;

// The panic of close(nil).
[[noreturn]] void chan_panic_close_nil();

// elem_slot<T> is storage for one element T that a receive constructs.
template <typename T>
struct elem_slot {
    elem_slot() noexcept = default;
    elem_slot(const elem_slot&) = delete;
    elem_slot& operator=(const elem_slot&) = delete;
    void* addr() noexcept { return gp_buf; }
    // take moves the element out; the slot is empty again.
    T take() noexcept(std::is_nothrow_move_constructible_v<T>) {
        T* p = std::launder(reinterpret_cast<T*>(gp_buf));
        T v(std::move(*p));
        p->~T();
        return v;
    }
    // drop destroys the element: the slot is empty again.
    void drop() noexcept { std::launder(reinterpret_cast<T*>(gp_buf))->~T(); }
    alignas(T) unsigned char gp_buf[sizeof(T)];
};

} // namespace detail

template <typename T>
class chan {
public:
    chan() noexcept = default;
    chan(std::nullptr_t) noexcept {}
    explicit chan(std::shared_ptr<detail::chan_core> c) noexcept : gp_c(std::move(c)) {}

    // Channels are equal when they are the same channel (two nil channels are equal).
    friend bool operator==(const chan& a, const chan& b) noexcept { return a.gp_c == b.gp_c; }
    friend bool operator==(const chan& a, std::nullptr_t) noexcept { return a.gp_c == nullptr; }

    // The hash of the channel: by its address, which is what == compares (a map key).
    std::size_t gp_hash() const noexcept {
        return detail::hash_int(
            static_cast<uint64_t>(reinterpret_cast<std::uintptr_t>(gp_c.get())));
    }

    // The address that print and fmt show for a channel (0x0 for nil).
    std::uintptr_t gp_address() const noexcept {
        return reinterpret_cast<std::uintptr_t>(gp_c.get());
    }

    // The channel object, null for nil.
    detail::chan_core* gp_core() const noexcept { return gp_c.get(); }

private:
    std::shared_ptr<detail::chan_core> gp_c;
};

// make_chan<T>(n) is make(chan T, n); make_chan<T>() is make(chan T).
template <typename T>
chan<T> make_chan(int64_t n = 0) {
    return chan<T>(detail::chan_make(&detail::elem_ops_of<T>, n));
}

// send is `ch <- v`. The parameter is the copy that the channel then owns.
template <typename T>
void send(const chan<T>& c, std::type_identity_t<T> v) {
    detail::chan_core* core = c.gp_core();
    if (core == nullptr) {
        detail::block_forever("chan send (nil chan)");
    }
    detail::chan_send(core, &v);
}

// recv is `<-ch`.
template <typename T>
T recv(const chan<T>& c) {
    detail::chan_core* core = c.gp_core();
    if (core == nullptr) {
        detail::block_forever("chan receive (nil chan)");
    }
    detail::elem_slot<T> s;
    detail::chan_recv(core, s.addr());
    return s.take();
}

// recv2 is `v, ok := <-ch`.
template <typename T>
std::tuple<T, bool> recv2(const chan<T>& c) {
    detail::chan_core* core = c.gp_core();
    if (core == nullptr) {
        detail::block_forever("chan receive (nil chan)");
    }
    detail::elem_slot<T> s;
    bool ok = detail::chan_recv(core, s.addr());
    return {s.take(), ok};
}

// recv_drop is `<-ch` as a statement, and `_, ok = <-ch`'s receive: the value is dropped.
template <typename T>
bool recv_drop(const chan<T>& c) {
    detail::chan_core* core = c.gp_core();
    if (core == nullptr) {
        detail::block_forever("chan receive (nil chan)");
    }
    return detail::chan_recv(core, nullptr);
}

// range_chan is the loop `for v := range ch`: it receives until the channel is closed and drained,
// and blocks for ever on a nil channel.
//
// clang-format off
//     for v := range ch {...}     for (auto gp_it0 = gp::range_chan(ch); gp::range_next(gp_it0);) {
//                                     T v = gp_it0.gp_v(); ...
//     for range ch {...}          for (auto gp_it0 = gp::range_chan_drop(ch); gp::range_next(gp_it0);) {
// clang-format on
//
// The iterator holds the channel, which Go evaluates once. After a successful range_next, gp_v()
// takes the element out (once: it is moved; the loop declares the variable of each iteration from
// it, so a closure that captures that variable has its own copy). An element that the body did not
// take is destroyed by the next range_next, or by the iterator. range_chan_drop receives without
// a place for the value (the loop has no variable: no copy, no construction).
template <typename T>
class chan_range {
public:
    explicit chan_range(chan<T> c, bool drop) noexcept : gp_c(std::move(c)), gp_drop(drop) {}
    chan_range(const chan_range&) = delete;
    chan_range& operator=(const chan_range&) = delete;
    ~chan_range() { gp_release(); }

    // The element that range_next received.
    T gp_v() noexcept(std::is_nothrow_move_constructible_v<T>) {
        gp_full = false;
        return gp_slot.take();
    }

    bool gp_next() {
        gp_release();
        detail::chan_core* core = gp_c.gp_core();
        if (core == nullptr) {
            detail::block_forever("chan receive (nil chan)");
        }
        if (gp_drop) {
            return detail::chan_recv(core, nullptr);
        }
        gp_full = detail::chan_recv_range(core, gp_slot.addr());
        return gp_full;
    }

private:
    void gp_release() noexcept {
        if (gp_full) {
            gp_full = false;
            gp_slot.drop();
        }
    }

    chan<T> gp_c;
    detail::elem_slot<T> gp_slot;
    bool gp_drop;
    bool gp_full = false;
};

template <typename T>
chan_range<T> range_chan(chan<T> c) noexcept {
    return chan_range<T>(std::move(c), false);
}

template <typename T>
chan_range<T> range_chan_drop(chan<T> c) noexcept {
    return chan_range<T>(std::move(c), true);
}

template <typename T>
bool range_next(chan_range<T>& it) {
    return it.gp_next();
}

// close is close(ch).
template <typename T>
void close(const chan<T>& c) {
    detail::chan_core* core = c.gp_core();
    if (core == nullptr) {
        detail::chan_panic_close_nil();
    }
    detail::chan_close(core);
}

// len and cap are len(ch) and cap(ch): 0 for a nil channel.
template <typename T>
int64_t len(const chan<T>& c) noexcept {
    detail::chan_core* core = c.gp_core();
    return core == nullptr ? 0 : detail::chan_len(core);
}

template <typename T>
int64_t cap(const chan<T>& c) noexcept {
    detail::chan_core* core = c.gp_core();
    return core == nullptr ? 0 : detail::chan_cap(core);
}

} // namespace gp
