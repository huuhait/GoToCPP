// gocpp/atomic.hpp: the native functions of the standard package sync/atomic (Phase 08, step 8.7),
// and gp::atomic_ptr<T>, the runtime type under atomic.Pointer[T].
//
// Go's sync/atomic is written in .gp (stdlib/sync/atomic) over these functions (//gocpp:native,
// ADR-027). All of them are sequentially consistent, which is Go's memory model ("all the atomic
// operations executed in a program behave as though executed in some sequentially consistent
// order").
//
// The function-style API takes a Go pointer, a gp::ptr<T> (a nil one panics like Go) and works on
// the object it points to with std::atomic_ref, so the operation is one instruction wherever
// the hardware has it, and it is atomic with respect to the typed API on the same memory:
//
//     atomic.AddInt32(&x, 1)               gp::sync_atomic::fn_add(p, int32_t(1))
//     atomic.LoadInt64(&x)                 gp::sync_atomic::fn_load(p)
//     atomic.CompareAndSwapUint32(&x, o, n)  gp::sync_atomic::fn_cas(p, o, n)
//     atomic.AndUint32(&x, m), OrInt64     gp::sync_atomic::fn_and(p, m), fn_or     (Go 1.23,
//     return the OLD value)
//
// The typed API (atomic.Int32, Uint32, Int64, Uint64, Uintptr, Bool) is a struct with one field v;
// its methods call the same operations on the struct's field (ty_*), through a template over the
// generated struct, so no pointer to the field is made (a gp::ptr to a field costs an atomic
// increment and decrement of the owner's count).
//
// The pointer operations (atomic.LoadPointer and friends on an *unsafe.Pointer, and
// atomic.Pointer[T]) can't be one instruction: a gocpp pointer owns its object (ADR-004), so it is
// two words with a reference count. They take a small spin lock of a table of 64, keyed by the
// address of the pointer (gp::sync_atomic::fn_load_pointer), or, for atomic_ptr<T>, a lock of
// the object. Every access to such a pointer must go through these functions (it is a data race
// otherwise, as in Go).
//
// The memory orders are the C++ defaults (seq_cst). Weaker orders would be faster on arm64 and
// are not Go's contract.
//
// Like every runtime type, atomic_ptr has only gp_-prefixed members; everything else is a free
// function.
#pragma once

#include <gocpp/ptr.hpp>
#include <gocpp/iface.hpp>

#include <atomic>
#include <cstdint>
#include <type_traits>
#include <utility>

namespace gp::sync_atomic {

namespace detail {

template <typename T>
inline std::atomic_ref<T> at(T& x) noexcept {
    return std::atomic_ref<T>(x);
}

// The spin locks of the pointer operations. lock_word(addr) is the lock that guards the pointer
// at addr (a table of 64). spin_lock waits for it (a few spins, then it yields the CPU: the
// holder only copies a pointer, but it may have been descheduled).
uint32_t& lock_word(const void* addr) noexcept;
void spin_lock_slow(uint32_t& w) noexcept;

inline void spin_lock(uint32_t& w) noexcept {
    uint32_t zero = 0;
    if (!std::atomic_ref<uint32_t>(w).compare_exchange_strong(zero, 1, std::memory_order_acquire))
        [[unlikely]] {
        spin_lock_slow(w);
    }
}

inline void spin_unlock(uint32_t& w) noexcept {
    std::atomic_ref<uint32_t>(w).store(0, std::memory_order_release);
}

struct spin_guard {
    explicit spin_guard(uint32_t& w) noexcept : gp_w(w) { spin_lock(w); }
    ~spin_guard() { spin_unlock(gp_w); }
    spin_guard(const spin_guard&) = delete;
    spin_guard& operator=(const spin_guard&) = delete;
    uint32_t& gp_w;
};

} // namespace detail

// --- the function-style API on a Go pointer to an integer ---

template <typename T>
inline T fn_add(const ptr<T>& addr, std::type_identity_t<T> delta) {
    return detail::at(*addr).fetch_add(delta) + delta;
}

template <typename T>
inline T fn_load(const ptr<T>& addr) {
    return detail::at(*addr).load();
}

template <typename T>
inline void fn_store(const ptr<T>& addr, std::type_identity_t<T> val) {
    detail::at(*addr).store(val);
}

template <typename T>
inline T fn_swap(const ptr<T>& addr, std::type_identity_t<T> val) {
    return detail::at(*addr).exchange(val);
}

template <typename T>
inline bool fn_cas(const ptr<T>& addr, std::type_identity_t<T> old, std::type_identity_t<T> now) {
    return detail::at(*addr).compare_exchange_strong(old, now);
}

template <typename T>
inline T fn_and(const ptr<T>& addr, std::type_identity_t<T> mask) {
    return detail::at(*addr).fetch_and(mask);
}

template <typename T>
inline T fn_or(const ptr<T>& addr, std::type_identity_t<T> mask) {
    return detail::at(*addr).fetch_or(mask);
}

// --- the function-style API on an *unsafe.Pointer ---

ptr<void> fn_load_pointer(const ptr<ptr<void>>& addr);
void fn_store_pointer(const ptr<ptr<void>>& addr, const ptr<void>& val);
ptr<void> fn_swap_pointer(const ptr<ptr<void>>& addr, const ptr<void>& val);
bool fn_cas_pointer(const ptr<ptr<void>>& addr, const ptr<void>& old, const ptr<void>& now);

// --- the typed API: a struct with one field v (Int32, Uint32, Int64, Uint64, Uintptr; Bool is a
// uint32 with 0 and 1) ---

template <typename S>
inline auto ty_load(const ptr<S>& x) {
    return detail::at((*x).v).load();
}

template <typename S>
inline void ty_store(const ptr<S>& x, decltype(S::v) val) {
    detail::at((*x).v).store(val);
}

template <typename S>
inline auto ty_swap(const ptr<S>& x, decltype(S::v) val) {
    return detail::at((*x).v).exchange(val);
}

template <typename S>
inline bool ty_cas(const ptr<S>& x, decltype(S::v) old, decltype(S::v) now) {
    return detail::at((*x).v).compare_exchange_strong(old, now);
}

template <typename S>
inline auto ty_add(const ptr<S>& x, decltype(S::v) delta) {
    return static_cast<decltype(S::v)>(detail::at((*x).v).fetch_add(delta) + delta);
}

template <typename S>
inline auto ty_and(const ptr<S>& x, decltype(S::v) mask) {
    return detail::at((*x).v).fetch_and(mask);
}

template <typename S>
inline auto ty_or(const ptr<S>& x, decltype(S::v) mask) {
    return detail::at((*x).v).fetch_or(mask);
}

// --- atomic.Value: struct { lk uint32; v any } ---
//
// Go's Value is two words that are written one after the other with a "store in progress" marker
// (value.go). A gocpp interface is 32 bytes with a count, so Value is a spin lock around it: a
// Load or a Store holds it for the time it takes to copy an interface. The type checks are Go's
// (in stdlib/sync/atomic/value.gp).

template <typename V>
inline void value_lock(const ptr<V>& v) {
    static_assert(std::is_same_v<decltype(V::lk), uint32_t>);
    detail::spin_lock((*v).lk);
}

template <typename V>
inline void value_unlock(const ptr<V>& v) {
    detail::spin_unlock((*v).lk);
}

// value_equal(v, old): v.v == old, with v's lock held; the comparison panics for a type that
// can't be compared, and the lock is released before the panic goes on.
template <typename V>
inline bool value_equal(const ptr<V>& v, const gp::iface& old) {
    try {
        return (*v).v == old;
    } catch (...) {
        detail::spin_unlock((*v).lk);
        throw;
    }
}

// same_type reports whether the dynamic types of two non-nil interface values are the same type.
inline bool same_type(const gp::iface& a, const gp::iface& b) noexcept {
    return a.gp_type() == b.gp_type();
}

} // namespace gp::sync_atomic

namespace gp {

// atomic_ptr<T> is atomic.Pointer[T]: a pointer that can be loaded, stored, swapped and compared
// and swapped from several goroutines. It is a gp::ptr<T> and a small spin lock (libstdc++'s
// std::atomic<std::shared_ptr> is not lock-free either, and libc++, which llvm-mingw uses on
// Windows, doesn't have it at all). The zero value is nil, and copying one before it is used
// copies the pointer, as copying an atomic.Pointer[T] before first use does in Go. The operations
// keep the pointers' counts right: a Load returns a pointer of its own (one atomic increment), a
// Store drops the old one after the lock is released.
//
//     var p atomic.Pointer[T]           gp::atomic_ptr<T> p;
//     p.Load()                          gp::atomic_load(p)
//     p.Store(x)                        gp::atomic_store(p, x)
//     p.Swap(x)                         gp::atomic_swap(p, x)
//     p.CompareAndSwap(old, x)          gp::atomic_cas(p, old, x)      pointer identity, as Go
template <typename T>
class atomic_ptr {
public:
    atomic_ptr() noexcept = default;
    atomic_ptr(const atomic_ptr& o) : gp_p_(o.gp_load()) {}
    atomic_ptr& operator=(const atomic_ptr& o) {
        gp_store(o.gp_load());
        return *this;
    }

    // The operations, for the free functions below.
    ptr<T> gp_load() const {
        sync_atomic::detail::spin_guard g(gp_lock_);
        return gp_p_;
    }
    void gp_store(ptr<T> p) {
        {
            sync_atomic::detail::spin_guard g(gp_lock_);
            std::swap(gp_p_, p);
        }
        // p, the old value, is dropped here, outside the lock: its destructor may run any
        // number of others.
    }
    ptr<T> gp_swap(ptr<T> p) {
        sync_atomic::detail::spin_guard g(gp_lock_);
        std::swap(gp_p_, p);
        return p;
    }
    bool gp_cas(const ptr<T>& old, ptr<T> now) {
        ptr<T> dropped;
        {
            sync_atomic::detail::spin_guard g(gp_lock_);
            if (!(gp_p_ == old)) {
                return false;
            }
            dropped = std::move(gp_p_);
            gp_p_ = std::move(now);
        }
        return true;
    }

private:
    mutable uint32_t gp_lock_ = 0;
    ptr<T> gp_p_;
};

template <typename T>
inline ptr<T> atomic_load(const atomic_ptr<T>& p) {
    return p.gp_load();
}
template <typename T>
inline void atomic_store(atomic_ptr<T>& p, ptr<T> x) {
    p.gp_store(std::move(x));
}
template <typename T>
inline ptr<T> atomic_swap(atomic_ptr<T>& p, ptr<T> x) {
    return p.gp_swap(std::move(x));
}
template <typename T>
inline bool atomic_cas(atomic_ptr<T>& p, const ptr<T>& old, ptr<T> x) {
    return p.gp_cas(old, std::move(x));
}

} // namespace gp
