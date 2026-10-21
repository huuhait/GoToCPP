// Isolated Phase 15.4 experiment. Not a runtime API or a replacement for gp::ptr.
#pragma once

#include <atomic>
#include <cstdint>
#include <exception>
#include <limits>
#include <memory>
#include <stdexcept>
#include <utility>

namespace rc_experiment {

namespace detail {

struct control {
    std::atomic<std::uint32_t> strong{1};
    // One implicit weak reference protects the block until value destruction completes.
    std::atomic<std::uint32_t> weak{1};

    virtual void destroy_value() noexcept = 0;
    virtual void free_block() noexcept = 0;

protected:
    ~control() = default;
};

inline void increment(std::atomic<std::uint32_t>& count) noexcept {
    if (count.fetch_add(1, std::memory_order_relaxed) ==
        std::numeric_limits<std::uint32_t>::max()) {
        std::terminate();
    }
}

inline void release_weak(control* block) noexcept {
    if (block->weak.fetch_sub(1, std::memory_order_acq_rel) == 1) {
        block->free_block();
    }
}

inline void release_strong(control* block) noexcept {
    if (block->strong.fetch_sub(1, std::memory_order_acq_rel) == 1) {
        block->destroy_value();
        release_weak(block);
    }
}

struct adopt {};

template <typename T, typename Allocator>
struct allocation final : control {
    [[no_unique_address]] Allocator allocator;
    alignas(T) std::byte storage[sizeof(T)];

    template <typename... Args>
    explicit allocation(const Allocator& alloc, Args&&... args) : allocator(alloc) {
        std::construct_at(reinterpret_cast<T*>(storage), std::forward<Args>(args)...);
    }

    T* value() noexcept { return std::launder(reinterpret_cast<T*>(storage)); }

    void destroy_value() noexcept override { std::destroy_at(value()); }

    void free_block() noexcept override {
        using rebound =
            typename std::allocator_traits<Allocator>::template rebind_alloc<allocation>;
        rebound alloc(allocator);
        std::allocator_traits<rebound>::destroy(alloc, this);
        std::allocator_traits<rebound>::deallocate(alloc, this, 1);
    }
};

} // namespace detail

template <typename T>
class weak;

// An allocation owner, independent of any interior address (e.g. in a string/slice header).
class owner {
public:
    owner() noexcept = default;
    owner(detail::control* block, detail::adopt) noexcept : block_(block) {}
    owner(const owner& other) noexcept : block_(other.block_) {
        if (block_ != nullptr) {
            detail::increment(block_->strong);
        }
    }
    owner(owner&& other) noexcept : block_(std::exchange(other.block_, nullptr)) {}
    owner& operator=(owner other) noexcept {
        swap(other);
        return *this;
    }
    ~owner() {
        if (block_ != nullptr) {
            detail::release_strong(block_);
        }
    }

    void swap(owner& other) noexcept { std::swap(block_, other.block_); }
    void reset() noexcept { owner().swap(*this); }
    bool empty() const noexcept { return block_ == nullptr; }
    std::uint32_t use_count() const noexcept {
        return block_ == nullptr ? 0 : block_->strong.load(std::memory_order_relaxed);
    }

private:
    template <typename T>
    friend class weak;
    detail::control* block_ = nullptr;
};

template <typename T>
class ptr {
public:
    ptr() noexcept = default;
    ptr(owner ownership, T* address) : ownership_(std::move(ownership)), address_(address) {
        if (address != nullptr && ownership_.empty()) {
            throw std::invalid_argument("non-null alias requires an owner");
        }
    }
    ptr(const ptr&) noexcept = default;
    ptr(ptr&& other) noexcept
        : ownership_(std::move(other.ownership_)),
          address_(std::exchange(other.address_, nullptr)) {}
    ptr& operator=(ptr other) noexcept {
        swap(other);
        return *this;
    }

    void swap(ptr& other) noexcept {
        ownership_.swap(other.ownership_);
        std::swap(address_, other.address_);
    }
    void reset() noexcept { ptr().swap(*this); }
    T* get() const noexcept { return address_; }
    T& operator*() const noexcept { return *address_; }
    T* operator->() const noexcept { return address_; }
    explicit operator bool() const noexcept { return address_ != nullptr; }
    const owner& ownership() const noexcept { return ownership_; }
    std::uint32_t use_count() const noexcept { return ownership_.use_count(); }

private:
    owner ownership_;
    T* address_ = nullptr;
};

template <typename T, typename Allocator, typename... Args>
ptr<T> allocate(const Allocator& allocator, Args&&... args) {
    using block_type = detail::allocation<T, Allocator>;
    using rebound = typename std::allocator_traits<Allocator>::template rebind_alloc<block_type>;
    rebound alloc(allocator);
    block_type* block = std::allocator_traits<rebound>::allocate(alloc, 1);
    try {
        std::allocator_traits<rebound>::construct(alloc, block, allocator,
                                                  std::forward<Args>(args)...);
    } catch (...) {
        std::allocator_traits<rebound>::deallocate(alloc, block, 1);
        throw;
    }
    return ptr<T>(owner(block, detail::adopt{}), block->value());
}

template <typename T, typename... Args>
ptr<T> make(Args&&... args) {
    return allocate<T>(std::allocator<std::byte>{}, std::forward<Args>(args)...);
}

template <typename T>
class weak {
public:
    weak() noexcept = default;
    explicit weak(const ptr<T>& pointer) noexcept
        : block_(pointer.ownership().block_), address_(pointer.get()) {
        if (block_ != nullptr) {
            detail::increment(block_->weak);
        }
    }
    weak(const weak& other) noexcept : block_(other.block_), address_(other.address_) {
        if (block_ != nullptr) {
            detail::increment(block_->weak);
        }
    }
    weak(weak&& other) noexcept
        : block_(std::exchange(other.block_, nullptr)),
          address_(std::exchange(other.address_, nullptr)) {}
    weak& operator=(weak other) noexcept {
        swap(other);
        return *this;
    }
    ~weak() {
        if (block_ != nullptr) {
            detail::release_weak(block_);
        }
    }

    void swap(weak& other) noexcept {
        std::swap(block_, other.block_);
        std::swap(address_, other.address_);
    }
    void reset() noexcept { weak().swap(*this); }
    bool expired() const noexcept {
        return block_ == nullptr || block_->strong.load(std::memory_order_acquire) == 0;
    }
    ptr<T> lock() const noexcept {
        if (block_ == nullptr) {
            return {};
        }
        auto count = block_->strong.load(std::memory_order_relaxed);
        while (count != 0) {
            if (count == std::numeric_limits<std::uint32_t>::max()) {
                std::terminate();
            }
            if (block_->strong.compare_exchange_weak(count, count + 1, std::memory_order_acq_rel,
                                                     std::memory_order_relaxed)) {
                return ptr<T>(owner(block_, detail::adopt{}), address_);
            }
        }
        return {};
    }
    friend bool operator==(const weak& left, const weak& right) noexcept {
        return left.block_ == right.block_ && left.address_ == right.address_;
    }

private:
    detail::control* block_ = nullptr;
    T* address_ = nullptr;
};

} // namespace rc_experiment
