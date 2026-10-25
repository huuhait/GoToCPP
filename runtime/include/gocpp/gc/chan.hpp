// gocpp/gc/chan.hpp: gp::gc::chan<T>, Go's channel value in the GC engine (phase-16 README
// §2.2; job 16.1e).
//
// A Go channel value is one pointer to the channel's core in the GC heap, 8 bytes, nil for the
// zero value. The core holds the element size, whether the element holds a pointer, the buffer
// and its length and capacity. This job's core is a placeholder: make_chan allocates it and its
// buffer in the GC heap, and len/cap read them, but send, receive, close and select are not
// implemented. The spike's packages (job 16.1f) use no channel; step 16.5 gives the core Go's
// element table and no destructors, and step 16.8 the operations.
//
// Channels are comparable in Go (two values are equal when they are the same channel), so
// chan<T> has == between two values and with nil. A channel in an interface compares by its
// pointer.
#pragma once

#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/golayout.hpp>
#include <gocpp/gc/heap.hpp>
#include <gocpp/gc/panic.hpp>
#include <gocpp/gc/slice.hpp>

#include <cstddef>
#include <cstdint>

namespace gp::gc {

// chan_core<T> is the placeholder channel object in the GC heap: the buffer and its element
// size. Step 16.5/16.8 replaces the buffer handling with the real one (a ring with send and
// receive positions); nothing outside gp::gc::chan reads it.
template <typename T>
struct chan_core {
    std::uint64_t gp_cap = 0;       // the buffer's capacity (0 for an unbuffered channel)
    std::uint64_t gp_len = 0;       // the number of buffered elements
    std::uint64_t gp_elem_size = 0; // Go's size of T
    bool gp_elem_ptrs = false;      // T holds a pointer
    T* gp_buf = nullptr;            // the buffer in the GC heap, or null
};

template <typename T>
class chan {
public:
    // nil.
    constexpr chan() noexcept = default;
    constexpr chan(std::nullptr_t) noexcept {}
    explicit constexpr chan(chan_core<T>* core) noexcept : gp_core_(core) {}

    bool gp_nil() const noexcept { return gp_core_ == nullptr; }
    chan_core<T>* gp_core() const noexcept { return gp_core_; }

    friend bool operator==(const chan& a, const chan& b) noexcept {
        return a.gp_core_ == b.gp_core_;
    }
    friend bool operator!=(const chan& a, const chan& b) noexcept {
        return a.gp_core_ != b.gp_core_;
    }
    friend bool operator==(const chan& c, std::nullptr_t) noexcept { return c.gp_core_ == nullptr; }
    friend bool operator!=(const chan& c, std::nullptr_t) noexcept { return c.gp_core_ != nullptr; }

private:
    chan_core<T>* gp_core_ = nullptr;
};

static_assert(sizeof(chan<std::int64_t>) == 8, "gp::gc::chan must be Go's 8 bytes");

// make_chan<T>(n): make(chan T, n). The core and its buffer are in the GC heap. Go panics with
// a runtime.plainError, "makechan: size out of range", for a negative or too big n.
template <typename T, go_int N>
chan<T> make_chan(N n) {
    auto c = static_cast<std::int64_t>(n);
    constexpr std::uint64_t size = go_layout<T>().size;
    constexpr std::uint64_t max_alloc = (std::uint64_t{1} << 48) - 1; // Go's maxAlloc
    constexpr std::uint64_t max_elems =
        size == 0 ? static_cast<std::uint64_t>(INT64_MAX) : max_alloc / size;
    if (c < 0 || static_cast<std::uint64_t>(c) > max_elems) [[unlikely]] {
        panic("makechan: size out of range", "runtime.plainError");
    }
    chan_core<T>* core = new_<chan_core<T>>();
    core->gp_cap = static_cast<std::uint64_t>(c);
    core->gp_elem_size = size;
    core->gp_elem_ptrs = go_layout<T>().ptrs;
    if (c > 0 && size > 0) {
        core->gp_buf = detail::alloc_elems<T>(c);
    }
    return chan<T>(core);
}

// len(c), cap(c): the number of buffered elements and the buffer's capacity; 0 for nil, as in
// Go.
template <typename T>
std::int64_t len(const chan<T>& c) noexcept {
    return c.gp_nil() ? 0 : static_cast<std::int64_t>(c.gp_core()->gp_len);
}
template <typename T>
std::int64_t cap(const chan<T>& c) noexcept {
    return c.gp_nil() ? 0 : static_cast<std::int64_t>(c.gp_core()->gp_cap);
}

} // namespace gp::gc
