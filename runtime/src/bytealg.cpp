// The native functions of package internal/bytealg that are not inline (gocpp/bytealg.hpp, Phase 09
// step 9.2). Standard C++ and the C library only.
#include <gocpp/rt.hpp>

#include <algorithm>
#include <cstring>
#include <string_view>

namespace gp::bytealg {
namespace detail {

int64_t last_index_raw(const void* p, int64_t n, uint8_t c) noexcept {
    // memrchr is a GNU function, so the loop is the portable one; the compiler makes it a tight
    // backward scan, and the callers (TrimRight, LastIndexByte of a path) look at short tails.
    const auto* b = static_cast<const uint8_t*>(p);
    for (int64_t i = n - 1; i >= 0; i--) {
        if (b[i] == c) {
            return i;
        }
    }
    return -1;
}

namespace {

inline uint64_t load64(const uint8_t* p) noexcept {
    uint64_t w;
    std::memcpy(&w, p, sizeof w);
    return w;
}

} // namespace

int64_t count_words(const void* p, int64_t n, uint8_t c) noexcept {
    if (n <= 0) {
        return 0;
    }
    const auto* b = static_cast<const uint8_t*>(p);
    int64_t total = 0;
    // Eight bytes at a time: a byte of x ^ (c in every byte) is zero where the input byte is c.
    // For each byte, ((x & 0x7f) + 0x7f | x) has its top bit set unless the byte is zero, and
    // no carry leaves a byte, so the count is exact. Up to 255 words add their 0 or 1 per byte
    // into the byte lanes of acc before it can overflow; then the lanes are summed.
    constexpr uint64_t lo7 = 0x7f7f7f7f7f7f7f7fULL, ones = 0x0101010101010101ULL;
    const uint64_t pattern = ones * c;
    while (n >= 8) {
        int64_t words = std::min<int64_t>(n / 8, 255);
        uint64_t acc = 0;
        for (int64_t i = 0; i < words; i++) {
            uint64_t x = load64(b) ^ pattern;
            b += 8;
            uint64_t nonzero = ((x & lo7) + lo7) | x; // top bit of each byte: the byte is not zero
            acc += (~nonzero >> 7) & ones;
        }
        n -= words * 8;
        constexpr uint64_t even = 0x00ff00ff00ff00ffULL;
        acc = (acc & even) + ((acc >> 8) & even);
        total += static_cast<int64_t>((acc * 0x0001000100010001ULL) >> 48);
    }
    for (; n > 0; n--, b++) {
        total += *b == c;
    }
    return total;
}

int64_t count_loop(const void* p, int64_t n, uint8_t c) noexcept {
    if (n <= 0) {
        return 0;
    }
    // A plain loop over blocks: the compiler turns the inner loop into vector compares (SSE2, NEON)
    // when it optimizes (g++ 15 and clang++ 21 do at -O2: 8 to 14 GB/s on 1 MiB, against 2 GB/s for
    // count_words). The block keeps the 32-bit sum small enough for the compiler to narrow its
    // lanes.
    const auto* b = static_cast<const uint8_t*>(p);
    int64_t total = 0;
    while (n > 0) {
        int64_t block = std::min<int64_t>(n, 255 * 32);
        unsigned sum = 0;
        for (int64_t i = 0; i < block; i++) {
            sum += b[i] == c;
        }
        total += sum;
        b += block;
        n -= block;
    }
    return total;
}

int64_t count_raw(const void* p, int64_t n, uint8_t c) noexcept {
#ifdef __OPTIMIZE__
    return count_loop(p, n, c);
#else
    // Without optimization (gocpp's default build) the loop would run a byte at a time: 0.7 GB/s,
    // against 2 GB/s for the words.
    return count_words(p, n, c);
#endif
}

int64_t index_mem(const void* a, int64_t na, const void* b, int64_t nb) noexcept {
    if (nb <= 0) {
        return 0;
    }
    if (nb > na) {
        return -1;
    }
#if defined(__GLIBC__)
    // glibc's memmem is a two-way search, linear in the worst case.
    const void* q = memmem(a, static_cast<std::size_t>(na), b, static_cast<std::size_t>(nb));
    return q == nullptr ? -1 : static_cast<const char*>(q) - static_cast<const char*>(a);
#else
    std::string_view hay(static_cast<const char*>(a), static_cast<std::size_t>(na));
    std::string_view needle(static_cast<const char*>(b), static_cast<std::size_t>(nb));
    std::size_t at = hay.find(needle);
    return at == std::string_view::npos ? -1 : static_cast<int64_t>(at);
#endif
}

} // namespace detail

gp::slice<uint8_t> make_no_zero(int64_t n) {
    if (static_cast<uint64_t>(n) > static_cast<uint64_t>(gp::detail::max_alloc)) [[unlikely]] {
        gp::detail::panic_make(n, n, 1);
    }
    if (n == 0) {
        return gp::make_slice<uint8_t>(0);
    }
    // Go's bytealg_MakeNoZero: the capacity is what its malloc gives, the size class of n.
    auto cap = static_cast<int64_t>(gp::detail::roundup_size(static_cast<uint64_t>(n), true));
    // new_array(cap, zero_from): the bytes from zero_from on are zeroed, so zero_from = cap
    // leaves all of them as they are.
    std::shared_ptr<uint8_t[]> a = gp::detail::new_array<uint8_t>(cap, cap);
    uint8_t* d = a.get();
    return gp::slice<uint8_t>(std::shared_ptr<void>(std::move(a), d), d, n, cap);
}

} // namespace gp::bytealg
