#pragma once

#include "rc.hpp"

#include <cstddef>
#include <cstdio>
#include <cstdlib>
#include <malloc.h>
#include <memory>

namespace experiment_support {

inline void require(bool condition, const char* message) {
    if (!condition) {
        std::fprintf(stderr, "FAIL: %s\n", message);
        std::abort();
    }
}

struct allocation_stats {
    std::size_t allocations = 0;
    std::size_t deallocations = 0;
    std::size_t requested = 0;
    std::size_t usable = 0;
    std::size_t alignment = 0;
    std::size_t live = 0;
    bool fail = false;
};

// Enabled only in storage/correctness probes, never in timed regions.
inline thread_local allocation_stats* recording = nullptr;

template <typename T>
struct allocator {
    using value_type = T;
    allocator() = default;
    template <typename U>
    allocator(const allocator<U>&) noexcept {}

    T* allocate(std::size_t count) {
        if (recording != nullptr && recording->fail) {
            throw std::bad_alloc();
        }
        T* result = std::allocator<T>{}.allocate(count);
        if (recording != nullptr) {
            ++recording->allocations;
            recording->requested += count * sizeof(T);
            recording->alignment = alignof(T);
            // This Linux/glibc probe uses ordinary-aligned payloads only.
            recording->usable += malloc_usable_size(result);
            ++recording->live;
        }
        return result;
    }
    void deallocate(T* pointer, std::size_t count) noexcept {
        if (recording != nullptr) {
            ++recording->deallocations;
            --recording->live;
        }
        std::allocator<T>{}.deallocate(pointer, count);
    }
    template <typename U>
    bool operator==(const allocator<U>&) const noexcept {
        return true;
    }
};

struct baseline {
    static constexpr const char* name = "shared";
    using owner_type = std::shared_ptr<void>;
    template <typename T>
    using pointer = std::shared_ptr<T>;
    template <typename T>
    using weak_type = std::weak_ptr<T>;

    template <typename T, typename... Args>
    static auto make(Args&&... args) {
        return std::make_shared<T>(std::forward<Args>(args)...);
    }
    template <typename T, typename... Args>
    static auto measured_make(Args&&... args) {
        return std::allocate_shared<T>(allocator<T>{}, std::forward<Args>(args)...);
    }
    template <typename T>
    static owner_type ownership(const pointer<T>& pointer) {
        return pointer;
    }
    template <typename T, typename U>
    static pointer<T> alias(const pointer<U>& root, T* address) {
        return pointer<T>(root, address);
    }
};

struct candidate {
    static constexpr const char* name = "intrusive";
    using owner_type = rc_experiment::owner;
    template <typename T>
    using pointer = rc_experiment::ptr<T>;
    template <typename T>
    using weak_type = rc_experiment::weak<T>;

    template <typename T, typename... Args>
    static auto make(Args&&... args) {
        return rc_experiment::make<T>(std::forward<Args>(args)...);
    }
    template <typename T, typename... Args>
    static auto measured_make(Args&&... args) {
        return rc_experiment::allocate<T>(allocator<T>{}, std::forward<Args>(args)...);
    }
    template <typename T>
    static owner_type ownership(const pointer<T>& pointer) {
        return pointer.ownership();
    }
    template <typename T, typename U>
    static pointer<T> alias(const pointer<U>& root, T* address) {
        return pointer<T>(root.ownership(), address);
    }
};

} // namespace experiment_support
