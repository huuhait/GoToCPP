// The select statement (gocpp/select.hpp): Go's selectgo, runtime/select.go, on the channel
// objects of src/chan.cpp. Standard C++ only.
#include <gocpp/nofortify.h>
#include "chan_core.hpp"
#include <gocpp/rt.hpp>
#include <gocpp/select.hpp>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <functional>
#include <memory>
#include <mutex>
#include <new>

namespace gp::detail {

namespace {

// Up to this many cases, everything select_run needs is on the stack.
constexpr int small_cases = 16;

[[noreturn]] void panic_closed_send() {
    throw_runtime(rt_error::plain, gp::string::gp_copy("send on closed channel"));
}

uint64_t splitmix(uint64_t& x) noexcept {
    uint64_t z = (x += 0x9e3779b97f4a7c15ULL);
    z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9ULL;
    z = (z ^ (z >> 27)) * 0x94d049bb133111ebULL;
    return z ^ (z >> 31);
}

// A random number below n (n > 0) from a generator of the thread: xorshift64*, seeded from the
// clock, a counter and the address of the generator, so threads that start together differ. Go's
// cheaprandn is the same kind of thing (the poll order only needs to be uniform, not secret).
uint32_t random_below(uint32_t n) noexcept {
    static std::atomic<uint64_t> counter{0};
    static constinit thread_local uint64_t state = 0;
    if (state == 0) {
        uint64_t s =
            static_cast<uint64_t>(std::chrono::steady_clock::now().time_since_epoch().count()) ^
            (counter.fetch_add(1, std::memory_order_relaxed) * 0xd6e8feb86659fd93ULL) ^
            reinterpret_cast<uintptr_t>(&state);
        state = splitmix(s) | 1;
    }
    uint64_t x = state;
    x ^= x >> 12;
    x ^= x << 25;
    x ^= x >> 27;
    state = x;
    uint64_t r = x * 0x2545f4914f6cdd1dULL;
    return static_cast<uint32_t>((static_cast<uint64_t>(static_cast<uint32_t>(r >> 32)) * n) >> 32);
}

// The memory of one select_run: the order of the polls, the order of the locks, and the entries
// (sudogs, constructed only when the goroutine blocks).
struct scratch {
    int* poll;
    int* lock;
    sudog* sg;

    explicit scratch(int n) {
        if (n <= small_cases) {
            poll = small_poll;
            lock = small_lock;
            sg = reinterpret_cast<sudog*>(small_sg);
        } else {
            std::size_t bytes = static_cast<std::size_t>(n) * (2 * sizeof(int) + sizeof(sudog));
            heap = std::make_unique<unsigned char[]>(bytes + alignof(sudog));
            auto p = reinterpret_cast<uintptr_t>(heap.get());
            p = (p + alignof(sudog) - 1) & ~(uintptr_t(alignof(sudog)) - 1);
            sg = reinterpret_cast<sudog*>(p);
            poll = reinterpret_cast<int*>(sg + n);
            lock = poll + n;
        }
    }

    int small_poll[small_cases];
    int small_lock[small_cases];
    alignas(sudog) unsigned char small_sg[small_cases * sizeof(sudog)];
    std::unique_ptr<unsigned char[]> heap;
};

// A lock on the channels of a select, in the order of their addresses (so that two selects, and a
// select and a close, never wait for each other in a cycle). A channel that is in two cases is
// locked once.
class locks {
public:
    locks(sel_entry* cs, int* order, int n) : gp_cs(cs), gp_order(order), gp_n(n) {}
    locks(const locks&) = delete;
    locks& operator=(const locks&) = delete;
    ~locks() {
        if (gp_held) {
            unlock();
        }
    }

    void lock() {
        chan_core* prev = nullptr;
        for (int k = 0; k < gp_n; k++) {
            chan_core* c = gp_cs[gp_order[k]].c;
            if (c != prev) {
                c->mu.lock();
                prev = c;
            }
        }
        gp_held = true;
    }

    void unlock() {
        chan_core* prev = nullptr;
        for (int k = gp_n - 1; k >= 0; k--) {
            chan_core* c = gp_cs[gp_order[k]].c;
            if (c != prev) {
                c->mu.unlock();
                prev = c;
            }
        }
        gp_held = false;
    }

private:
    sel_entry* gp_cs;
    int* gp_order;
    int gp_n;
    bool gp_held = false;
};

} // namespace

void select_forever() {
    block_forever("select (no cases)");
}

int select_run(sel_entry* cs, int n) {
    scratch mem(n);
    int dflt = -1;
    int np = 0; // the cases that can happen: not the default, not a nil channel
    for (int i = 0; i < n; i++) {
        if (cs[i].kind == sel_kind::dflt) {
            dflt = i;
        } else if (cs[i].c != nullptr) {
            // The poll order: a random permutation (the insertion shuffle of selectgo).
            int j = static_cast<int>(random_below(static_cast<uint32_t>(np + 1)));
            if (j != np) {
                mem.poll[np] = mem.poll[j];
            }
            mem.poll[j] = i;
            np++;
        }
    }
    if (np == 0 && dflt < 0) {
        block_forever("select");
    }
    if (np == 0) {
        return dflt;
    }
    // A timer channel whose timer is due gets its value before the polls (Go's selectgo does the
    // same with maybeRunChan).
    for (int k = 0; k < np; k++) {
        sel_entry& e = cs[mem.poll[k]];
        if (e.kind == sel_kind::recv && e.c->timer != nullptr) [[unlikely]] {
            timer_observe(e.c);
        }
    }
    // The lock order: by the address of the channel.
    std::copy(mem.poll, mem.poll + np, mem.lock);
    std::sort(mem.lock, mem.lock + np,
              [cs](int a, int b) { return std::less<chan_core*>()(cs[a].c, cs[b].c); });
    locks held(cs, mem.lock, np);
    held.lock();

    // Pass 1: the first case in the poll order that is ready.
    for (int k = 0; k < np; k++) {
        int i = mem.poll[k];
        sel_entry& e = cs[i];
        sudog* w = nullptr;
        if (e.kind == sel_kind::recv) {
            poll_status st = e.c->recv_locked(e.elem, &w);
            if (st == poll_status::not_ready) {
                continue;
            }
            if (e.ok != nullptr) {
                *e.ok = st == poll_status::done;
            }
        } else {
            poll_status st = e.c->send_locked(e.elem, &w);
            if (st == poll_status::not_ready) {
                continue;
            }
            if (st == poll_status::closed) {
                held.unlock();
                panic_closed_send();
            }
        }
        held.unlock();
        if (w != nullptr) {
            wake(w);
        }
        return i;
    }
    if (dflt >= 0) {
        held.unlock();
        return dflt;
    }

    // Pass 2: nothing is ready. One entry for each case, in the queue of its channel, with one
    // claim flag and one cell among them; then wait. The goroutine is counted as blocked from
    // before the first entry is visible to a waker (block_scope under the locks).
    block_scope blocked("select");
    goroutine& g = this_goroutine();
    wake_cell cell;
    std::atomic<uint32_t> claim{0};
    for (int k = 0; k < np; k++) {
        int i = mem.lock[k];
        sel_entry& e = cs[i];
        sudog* sg = new (&mem.sg[i]) sudog;
        sg->g = &g;
        sg->w = &g.park;
        sg->cell = &cell;
        sg->elem = e.elem;
        sg->c = e.c;
        sg->claim = &claim;
        sg->case_index = i;
        (e.kind == sel_kind::recv ? e.c->recvq : e.c->sendq).enqueue(sg);
    }
    held.unlock();
    park_cell(cell);

    // Woken: one entry completed its case (the waker did the transfer under the channel's lock,
    // set success and took the entry off its queue). Take the others off.
    held.lock();
    sudog* fired = cell.fired;
    for (int k = 0; k < np; k++) {
        int i = mem.lock[k];
        sudog* sg = &mem.sg[i];
        if (sg != fired) {
            waitq& q = cs[i].kind == sel_kind::recv ? cs[i].c->recvq : cs[i].c->sendq;
            if (q.contains(sg)) {
                q.remove(sg);
            }
        }
    }
    held.unlock();
    int i = fired->case_index;
    bool success = fired->success;
    if (cs[i].kind == sel_kind::recv) {
        if (cs[i].ok != nullptr) {
            *cs[i].ok = success;
        }
    } else if (!success) {
        panic_closed_send(); // woken by close: the send panics
    }
    return i;
}

} // namespace gp::detail

namespace gp {

void select_unreachable() {
    detail::fatal("select: no case was chosen");
}

} // namespace gp
