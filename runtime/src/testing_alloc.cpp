// The allocation counter of package testing (Phase 09 step 9.9): benchmarks report B/op and
// allocs/op, and testing.AllocsPerRun counts allocations, from the calls of the global operator new
// while someone asked for the counting (gocpp/testing.hpp: alloc_counting, alloc_stats).
//
// A test binary replaces the global operator new and delete, so that a benchmark can report the
// allocations per operation and testing.AllocsPerRun can count them. What is counted is the C++
// heap: every call of operator new while a benchmark or AllocsPerRun is measuring. It is the
// program's own allocations (strings, slices, maps, pointers and closures that Go would put on its
// heap), and also the C++ code of the runtime (a goroutine, a channel), so the numbers are not
// Go's: the same code can allocate more or less than under Go's compiler, which keeps some values
// on the stack.

#include <gocpp/nofortify.h>
#include <gocpp/testing.hpp>

#include <atomic>
#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <new>
#include <tuple>

#if defined(_MSC_VER) || defined(_WIN32)
// Strong definitions on Windows. With MinGW's statically linked libstdc++ a weak operator new
// loses: the linker sees the weak external as unresolved, takes libstdc++'s new_op.o from the
// archive, and its strong definition wins, so no allocation is counted (found on the PC in step
// 14.9's review: `0 B/op 0 allocs/op`). This file is linked as an object before the libraries, so
// the archive members that define operator new are never needed, and no duplicate arises.
#define GP_WEAK_ALLOC
#else
#define GP_WEAK_ALLOC __attribute__((weak))
#endif

namespace {

std::atomic<int> alloc_users{0};
std::atomic<uint64_t> alloc_count{0};
std::atomic<uint64_t> alloc_bytes{0};

} // namespace

namespace gp::testing {

void alloc_counting(int64_t delta) noexcept {
    alloc_users.fetch_add(static_cast<int>(delta));
}

std::tuple<int64_t, int64_t> alloc_stats() noexcept {
    return {static_cast<int64_t>(alloc_count.load()), static_cast<int64_t>(alloc_bytes.load())};
}

} // namespace gp::testing

namespace {

inline void count_alloc(std::size_t n) noexcept {
    if (alloc_users.load(std::memory_order_relaxed) > 0) {
        alloc_count.fetch_add(1, std::memory_order_relaxed);
        alloc_bytes.fetch_add(n, std::memory_order_relaxed);
    }
}

void* plain_alloc(std::size_t n) noexcept {
    count_alloc(n);
    return std::malloc(n != 0 ? n : 1);
}

// An aligned block: the pointer to the malloc block is kept just before the aligned address.
void* aligned_alloc_block(std::size_t n, std::size_t align) noexcept {
    count_alloc(n);
    if (align < alignof(void*)) {
        align = alignof(void*);
    }
    void* raw = std::malloc(n + align + sizeof(void*));
    if (raw == nullptr) {
        return nullptr;
    }
    auto addr = (reinterpret_cast<std::uintptr_t>(raw) + sizeof(void*) + align - 1) &
                ~(static_cast<std::uintptr_t>(align) - 1);
    void* p = reinterpret_cast<void*>(addr);
    reinterpret_cast<void**>(p)[-1] = raw;
    return p;
}

void aligned_free_block(void* p) noexcept {
    if (p != nullptr) {
        std::free(reinterpret_cast<void**>(p)[-1]);
    }
}

} // namespace

GP_WEAK_ALLOC void* operator new(std::size_t n) {
    void* p = plain_alloc(n);
    if (p == nullptr) {
        throw std::bad_alloc();
    }
    return p;
}
GP_WEAK_ALLOC void* operator new[](std::size_t n) {
    return ::operator new(n);
}
GP_WEAK_ALLOC void* operator new(std::size_t n, const std::nothrow_t&) noexcept {
    return plain_alloc(n);
}
GP_WEAK_ALLOC void* operator new[](std::size_t n, const std::nothrow_t&) noexcept {
    return plain_alloc(n);
}
GP_WEAK_ALLOC void* operator new(std::size_t n, std::align_val_t a) {
    void* p = aligned_alloc_block(n, static_cast<std::size_t>(a));
    if (p == nullptr) {
        throw std::bad_alloc();
    }
    return p;
}
GP_WEAK_ALLOC void* operator new[](std::size_t n, std::align_val_t a) {
    return ::operator new(n, a);
}
GP_WEAK_ALLOC void* operator new(std::size_t n, std::align_val_t a,
                                 const std::nothrow_t&) noexcept {
    return aligned_alloc_block(n, static_cast<std::size_t>(a));
}
GP_WEAK_ALLOC void* operator new[](std::size_t n, std::align_val_t a,
                                   const std::nothrow_t&) noexcept {
    return aligned_alloc_block(n, static_cast<std::size_t>(a));
}
GP_WEAK_ALLOC void operator delete(void* p) noexcept {
    std::free(p);
}
GP_WEAK_ALLOC void operator delete[](void* p) noexcept {
    std::free(p);
}
GP_WEAK_ALLOC void operator delete(void* p, std::size_t) noexcept {
    std::free(p);
}
GP_WEAK_ALLOC void operator delete[](void* p, std::size_t) noexcept {
    std::free(p);
}
GP_WEAK_ALLOC void operator delete(void* p, const std::nothrow_t&) noexcept {
    std::free(p);
}
GP_WEAK_ALLOC void operator delete[](void* p, const std::nothrow_t&) noexcept {
    std::free(p);
}
GP_WEAK_ALLOC void operator delete(void* p, std::align_val_t) noexcept {
    aligned_free_block(p);
}
GP_WEAK_ALLOC void operator delete[](void* p, std::align_val_t) noexcept {
    aligned_free_block(p);
}
GP_WEAK_ALLOC void operator delete(void* p, std::size_t, std::align_val_t) noexcept {
    aligned_free_block(p);
}
GP_WEAK_ALLOC void operator delete[](void* p, std::size_t, std::align_val_t) noexcept {
    aligned_free_block(p);
}
GP_WEAK_ALLOC void operator delete(void* p, std::align_val_t, const std::nothrow_t&) noexcept {
    aligned_free_block(p);
}
GP_WEAK_ALLOC void operator delete[](void* p, std::align_val_t, const std::nothrow_t&) noexcept {
    aligned_free_block(p);
}
