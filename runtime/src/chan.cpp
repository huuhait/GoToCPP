// Channels (gocpp/chan.hpp): Go's hchan algorithm, runtime/chan.go, on a mutex of the C++
// library and gp::waiter. Standard C++ only.
#include <gocpp/nofortify.h>
#include "chan_core.hpp"
#include <gocpp/rt.hpp>

#include <cstddef>
#include <cstdint>
#include <memory>
#include <mutex>
#include <new>
#include <gocpp/os.hpp>

namespace gp::detail {

namespace {

// maxAlloc of Go's runtime on 64-bit systems: a buffer larger than this is "size out of range".
constexpr uint64_t max_buffer = uint64_t(1) << 48;

// How long a goroutine spins before it sleeps (spin_wait): about 4 microseconds on current x86
// CPUs, whose pause takes 40 ns or more.
constexpr int spin_iterations = 100;

inline void cpu_relax() noexcept {
#if defined(__x86_64__) || defined(__i386__)
    __builtin_ia32_pause();
#elif defined(__aarch64__)
    asm volatile("yield" ::: "memory");
#endif
}

[[noreturn]] void panic_closed_send() {
    throw_runtime(rt_error::plain, gp::string::gp_copy("send on closed channel"));
}

// A queue of entries to wake after the lock is released (they are linked through next).
struct wake_list {
    sudog* head = nullptr;
    sudog* tail = nullptr;
    void add(sudog* sg) noexcept {
        sg->next = nullptr;
        if (tail != nullptr) {
            tail->next = sg;
        } else {
            head = sg;
        }
        tail = sg;
    }
    // wake_all wakes the entries in order; it reads the link before each wake (the entry may be
    // gone after it).
    void wake_all() noexcept {
        sudog* sg = head;
        head = tail = nullptr;
        while (sg != nullptr) {
            sudog* next = sg->next;
            wake(sg);
            sg = next;
        }
    }
};

} // namespace

bool spin_wait(const wake_cell& cell) noexcept {
    static const bool multi = os::cpu_count() > 1;
    if (!multi) {
        return false;
    }
    for (int i = 0; i < spin_iterations; i++) {
        if (cell.done.load(std::memory_order_acquire)) {
            return true;
        }
        cpu_relax();
    }
    return false;
}

chan_core::chan_core(const elem_ops* o, std::size_t n) : ops(o), capacity(n) {
    if (n > 0 && !o->empty) {
        buf = static_cast<unsigned char*>(::operator new(n * o->size, std::align_val_t{o->align}));
    }
}

chan_core::~chan_core() {
    // The elements still in the buffer.
    std::size_t n = count.load(std::memory_order_relaxed);
    for (std::size_t i = 0, r = recvx; i < n; i++) {
        ops->destroy(slot(r));
        r = r + 1 == capacity ? 0 : r + 1;
    }
    if (buf != nullptr) {
        ::operator delete(buf, std::align_val_t{ops->align});
    }
}

std::shared_ptr<chan_core> chan_make(const elem_ops* ops, int64_t n) {
    uint64_t mem = 0;
    if (n < 0 ||
        __builtin_mul_overflow(static_cast<uint64_t>(n),
                               ops->empty ? 0 : static_cast<uint64_t>(ops->size), &mem) ||
        mem > max_buffer) {
        throw_runtime(rt_error::plain, gp::string::gp_copy("makechan: size out of range"));
    }
    return std::make_shared<chan_core>(ops, static_cast<std::size_t>(n));
}

void chan_send(chan_core* c, void* src) {
    c->send(src);
}

bool chan_recv(chan_core* c, void* dst) {
    return c->recv(dst, true);
}

bool chan_recv_range(chan_core* c, void* dst) {
    return c->recv(dst, false);
}

void chan_close(chan_core* c) {
    c->close();
}

int64_t chan_len(const chan_core* c) noexcept {
    return static_cast<int64_t>(c->len());
}

int64_t chan_cap(const chan_core* c) noexcept {
    return static_cast<int64_t>(c->cap());
}

void chan_panic_close_nil() {
    throw_runtime(rt_error::plain, gp::string::gp_copy("close of nil channel"));
}

poll_status chan_core::send_locked(void* src, sudog** w) {
    if (closed) {
        return poll_status::closed;
    }
    if (sudog* sg = recvq.dequeue()) {
        // A receiver waits: the value goes straight into its variable, and it wakes up.
        if (sg->elem != nullptr) {
            ops->move_into(sg->elem, src);
        }
        sg->success = true;
        *w = sg;
        return poll_status::done;
    }
    if (std::size_t n = count.load(std::memory_order_relaxed); n < capacity) {
        ops->move_into(slot(sendx), src);
        sendx = sendx + 1 == capacity ? 0 : sendx + 1;
        count.store(n + 1, std::memory_order_relaxed);
        return poll_status::done;
    }
    return poll_status::not_ready;
}

bool chan_core::send_impl(void* src, bool block) {
    std::unique_lock<std::mutex> lk(mu);
    sudog* w = nullptr;
    switch (send_locked(src, &w)) {
    case poll_status::closed:
        lk.unlock();
        panic_closed_send();
    case poll_status::done:
        lk.unlock();
        if (w != nullptr) {
            wake(w);
        }
        return true;
    case poll_status::not_ready:
        break;
    }
    if (!block) {
        return false;
    }
    // Block: leave an entry in the send queue, with a pointer to the value.
    block_scope blocked("chan send");
    wake_cell cell;
    sudog sg;
    sg.g = &this_goroutine();
    sg.w = &sg.g->park;
    sg.cell = &cell;
    sg.elem = src;
    sg.c = this;
    sendq.enqueue(&sg);
    lk.unlock();
    park_cell(cell);
    if (!sg.success) {
        // Woken by close: Go panics in the sender too.
        panic_closed_send();
    }
    return true;
}

void chan_core::send(void* src) {
    send_impl(src, true);
}

bool chan_core::try_send(void* src) {
    return send_impl(src, false);
}

poll_status chan_core::recv_locked(void* dst, sudog** w, bool zero) {
    if (closed && count.load(std::memory_order_relaxed) == 0) {
        if (dst != nullptr && zero && !ops->zero_into(dst)) {
            fatal("receive from a closed channel whose element type has no zero value");
        }
        return poll_status::closed;
    }
    if (sudog* sg = sendq.dequeue()) {
        if (capacity == 0) {
            // Unbuffered: straight from the sender.
            if (dst != nullptr) {
                ops->move_into(dst, sg->elem);
            }
        } else {
            // The buffer is full and a sender waits: the receiver takes the head of the buffer,
            // and the sender's value goes to the tail, which is the slot that was just freed.
            void* head = slot(recvx);
            if (dst != nullptr) {
                ops->move_into(dst, head);
            }
            ops->destroy(head);
            ops->move_into(head, sg->elem);
            recvx = recvx + 1 == capacity ? 0 : recvx + 1;
            sendx = recvx;
        }
        sg->success = true;
        *w = sg;
        return poll_status::done;
    }
    if (std::size_t n = count.load(std::memory_order_relaxed); n > 0) {
        void* head = slot(recvx);
        if (dst != nullptr) {
            ops->move_into(dst, head);
        }
        ops->destroy(head);
        recvx = recvx + 1 == capacity ? 0 : recvx + 1;
        count.store(n - 1, std::memory_order_relaxed);
        return poll_status::done;
    }
    return poll_status::not_ready;
}

recv_status chan_core::recv_impl(void* dst, bool block, bool zero) {
    std::unique_lock<std::mutex> lk(mu);
    sudog* w = nullptr;
    switch (recv_locked(dst, &w, zero)) {
    case poll_status::closed:
        return recv_status::closed;
    case poll_status::done:
        lk.unlock();
        if (w != nullptr) {
            wake(w);
        }
        return recv_status::value;
    case poll_status::not_ready:
        break;
    }
    if (!block) {
        return recv_status::not_ready;
    }
    block_scope blocked("chan receive");
    wake_cell cell;
    sudog sg;
    sg.g = &this_goroutine();
    sg.w = &sg.g->park;
    sg.cell = &cell;
    sg.elem = dst;
    sg.c = this;
    sg.zero = zero;
    recvq.enqueue(&sg);
    lk.unlock();
    park_cell(cell);
    // The sender (or close) constructed the value (or the zero value) in dst.
    return sg.success ? recv_status::value : recv_status::closed;
}

bool chan_core::recv(void* dst, bool zero) {
    return recv_impl(dst, true, zero) == recv_status::value;
}

recv_status chan_core::try_recv(void* dst) {
    return recv_impl(dst, false, true);
}

void chan_core::close() {
    wake_list list;
    {
        std::unique_lock<std::mutex> lk(mu);
        if (closed) {
            lk.unlock();
            throw_runtime(rt_error::plain, gp::string::gp_copy("close of closed channel"));
        }
        closed = true;
        // All receivers get the zero value and false, then all senders panic. (The select entries
        // of a goroutine that something else woke already are skipped by dequeue.)
        while (sudog* sg = recvq.dequeue()) {
            if (sg->elem != nullptr && sg->zero && !ops->zero_into(sg->elem)) {
                lk.unlock();
                fatal("receive from a closed channel whose element type has no zero value");
            }
            sg->success = false;
            list.add(sg);
        }
        while (sudog* sg = sendq.dequeue()) {
            sg->success = false;
            list.add(sg);
        }
    }
    list.wake_all();
}

} // namespace gp::detail
