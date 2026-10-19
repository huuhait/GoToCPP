// The pointer operations of package sync/atomic (gocpp/atomic.hpp): a table of spin locks keyed by
// the address of the pointer. Standard C++ only.
#include <gocpp/nofortify.h>
#include <gocpp/rt.hpp>

#include <atomic>
#include <cstdint>
#include <thread>

namespace gp::sync_atomic {

namespace detail {

namespace {

constexpr std::size_t stripes = 64;

struct alignas(64) stripe {
    uint32_t lock = 0;
};

stripe stripe_table[stripes];

} // namespace

uint32_t& lock_word(const void* addr) noexcept {
    // Pointers are 8 bytes apart at least; mix the higher bits in.
    auto a = reinterpret_cast<uintptr_t>(addr);
    return stripe_table[((a >> 3) ^ (a >> 9)) % stripes].lock;
}

void spin_lock_slow(uint32_t& w) noexcept {
    std::atomic_ref<uint32_t> word(w);
    for (int spins = 0;; spins++) {
        uint32_t zero = 0;
        if (word.load(std::memory_order_relaxed) == 0 &&
            word.compare_exchange_weak(zero, 1, std::memory_order_acquire)) {
            return;
        }
        if (spins < 64) {
#if defined(__x86_64__) || defined(__i386__)
            __builtin_ia32_pause();
#elif defined(__aarch64__)
            asm volatile("yield" ::: "memory");
#endif
        } else {
            // The holder may have been descheduled.
            std::this_thread::yield();
        }
    }
}

} // namespace detail

ptr<void> fn_load_pointer(const ptr<ptr<void>>& addr) {
    ptr<void>& p = *addr;
    detail::spin_guard g(detail::lock_word(&p));
    return p;
}

void fn_store_pointer(const ptr<ptr<void>>& addr, const ptr<void>& val) {
    ptr<void>& p = *addr;
    ptr<void> old;
    {
        detail::spin_guard g(detail::lock_word(&p));
        old = std::move(p);
        p = val;
    }
}

ptr<void> fn_swap_pointer(const ptr<ptr<void>>& addr, const ptr<void>& val) {
    ptr<void>& p = *addr;
    detail::spin_guard g(detail::lock_word(&p));
    ptr<void> old = std::move(p);
    p = val;
    return old;
}

bool fn_cas_pointer(const ptr<ptr<void>>& addr, const ptr<void>& old, const ptr<void>& now) {
    ptr<void>& p = *addr;
    ptr<void> dropped;
    detail::spin_guard g(detail::lock_word(&p));
    if (!(p == old)) {
        return false;
    }
    dropped = std::move(p);
    p = now;
    return true;
}

} // namespace gp::sync_atomic
