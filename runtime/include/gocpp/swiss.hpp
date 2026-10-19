// gocpp/swiss.hpp: the building blocks of gp::map's Swiss table (gocpp/map.hpp, ADR-059): the
// control-byte groups (SSE2, and a portable 64-bit version), and the seeded hashes of
// map keys.
//
// A Swiss table keeps one control byte per slot. A full slot's byte is the low 7 bits of the
// key's hash (H2); an empty slot is 0b10000000 and a deleted one (a tombstone) 0b11111110, so
// "high bit set" means "no entry here". A probe loads the control bytes of a group of slots at
// once and compares all of them with H2 in one instruction: only the slots that match are
// compared with the key. A group that has an empty slot ends the probe, since no entry that was
// stored after the probe passed this group can be further.
//
// Group is the interface (the two implementations below):
//
//     static constexpr size_t width           how many slots a group covers
//     Group(const int8_t* p)                  loads the control bytes p[0 .. width)
//     match(h2)                               the slots whose byte equals h2 (the SWAR version can
//                                             also report a full slot whose byte differs from h2
//                                             in bit 0: never an empty or deleted one)
//     match_empty()                           the slots that are empty
//     match_empty_or_deleted()                the slots that hold no entry
//
// The masks are a bitmask, which iterates over the slot numbers (lowest first) and says how many
// slots come before the first and after the last bit (trailing_zeros, leading_zeros: erase uses
// them to tell a slot that no probe ever passed, which becomes empty, from one that a probe may
// have passed, which becomes a tombstone).
#pragma once

#include <bit>
#include <cstddef>
#include <cstdint>
#include <cstring>

// GOCPP_SWISS_PORTABLE selects the portable group on every machine (the tests build with it too).
// The SSE2 group uses the compilers' own vector types and the pmovmskb builtin of g++ and clang++,
// not <emmintrin.h>: runtime headers may include only standard headers (the OS rule, ADR-021), and
// the intrinsics header would add a hundred macros (_mm_*, _MM_*) to every program.

namespace gp::detail::swiss {

inline constexpr int8_t ctrl_empty = -128; // 0b10000000
inline constexpr int8_t ctrl_deleted = -2; // 0b11111110

// The control bytes of a table without slots: all empty, read by the probe of a map that was
// never written to (never written itself).
struct empty_ctrl_t {
    int8_t gp_b[32];
    constexpr empty_ctrl_t() : gp_b{} {
        for (int8_t& b : gp_b) {
            b = ctrl_empty;
        }
    }
};
inline empty_ctrl_t empty_ctrl;

// A set of slot numbers of a group, as the bits that a comparison leaves. A bit per slot
// (Shift 0, the SSE2 mask: bit i is slot i) or a byte per slot (Shift 3, the portable
// mask: the high bit of byte i is slot i).
template <typename Int, int Shift>
struct bitmask {
    Int gp_m;

    explicit operator bool() const noexcept { return gp_m != 0; }

    // The iterator: the numbers of the slots in the mask, lowest first.
    struct iterator {
        Int gp_m;
        int operator*() const noexcept { return std::countr_zero(gp_m) >> Shift; }
        iterator& operator++() noexcept {
            gp_m &= static_cast<Int>(gp_m - 1);
            return *this;
        }
        friend bool operator==(const iterator& a, const iterator& b) noexcept {
            return a.gp_m == b.gp_m;
        }
    };
    iterator begin() const noexcept { return {gp_m}; }
    iterator end() const noexcept { return {0}; }

    // The first slot in the mask (the mask is not empty).
    int lowest() const noexcept { return std::countr_zero(gp_m) >> Shift; }
    // The number of slots before the first one in the mask (the mask is not empty).
    int trailing_zeros() const noexcept { return std::countr_zero(gp_m) >> Shift; }
    // The number of slots after the last one in the mask (the mask is not empty).
    int leading_zeros() const noexcept { return std::countl_zero(gp_m) >> Shift; }
};

static_assert(std::endian::native == std::endian::little,
              "gp::map's portable control-byte group reads eight bytes as a little-endian word");

// group_swar: eight control bytes as one 64-bit word, compared with the classic "has zero byte"
// bit tricks. Works everywhere (and is what the tests run next to the machine's own group).
struct group_swar {
    static constexpr std::size_t width = 8;
    static constexpr uint64_t lsbs = 0x0101010101010101u;
    static constexpr uint64_t msbs = 0x8080808080808080u;

    uint64_t gp_w;

    explicit group_swar(const int8_t* p) noexcept { std::memcpy(&gp_w, p, sizeof gp_w); }

    bitmask<uint64_t, 3> match(uint8_t h2) const noexcept {
        // A byte of x is zero where the control byte equals h2. The borrow of the subtraction
        // can mark the byte above a true match too: that is a full slot (see the top), which the
        // key comparison rejects.
        uint64_t x = gp_w ^ (lsbs * h2);
        return {(x - lsbs) & ~x & msbs};
    }
    bitmask<uint64_t, 3> match_empty() const noexcept {
        // Empty is 0b10000000 and deleted 0b11111110: bit 7 set and bit 1 clear.
        return {(gp_w & ~(gp_w << 6)) & msbs};
    }
    bitmask<uint64_t, 3> match_empty_or_deleted() const noexcept { return {gp_w & msbs}; }
};

#if defined(GOCPP_SWISS_PORTABLE)

using group = group_swar;

#elif defined(__SSE2__) && defined(__GNUC__)

// group_sse: sixteen control bytes in an SSE2 register. The compare is the vector extension's ==
// (pcmpeqb), the mask pmovmskb: what _mm_cmpeq_epi8 and _mm_movemask_epi8 are.
struct group_sse {
    static constexpr std::size_t width = 16;

    using v16 = char __attribute__((vector_size(16)));

    v16 gp_c;

    explicit group_sse(const int8_t* p) noexcept { std::memcpy(&gp_c, p, sizeof gp_c); }

    static uint16_t mask(v16 m) noexcept {
        return static_cast<uint16_t>(__builtin_ia32_pmovmskb128(m));
    }
    static v16 splat(int8_t b) noexcept { return v16{} + static_cast<char>(b); }

    bitmask<uint16_t, 0> match(uint8_t h2) const noexcept {
        return {mask(static_cast<v16>(gp_c == splat(static_cast<int8_t>(h2))))};
    }
    bitmask<uint16_t, 0> match_empty() const noexcept {
        return {mask(static_cast<v16>(gp_c == splat(ctrl_empty)))};
    }
    bitmask<uint16_t, 0> match_empty_or_deleted() const noexcept { return {mask(gp_c)}; }
};

using group = group_sse;

#else

// Everywhere else (arm64 included: eight bytes in a 64-bit word are as fast there as a NEON
// register would be for a group this small).
using group = group_swar;

#endif

// --- hashes of map keys ---
//
// The hash of a key is seeded: every map has its own random seed (swiss_seed), so the order of
// a map's entries and the keys that collide in it differ from map to map and from run to run,
// as in Go (the seed also guards against inputs chosen to collide). The mixing step is the
// 64x64 -> 128 bit multiply of wyhash (mum): one multiplication for an integer key.

// swiss_random: the next number of the calling thread's random stream (seeded once per process
// from the operating system's entropy). For a map's seed and the start of a range. In
// runtime/src/map.cpp.
uint64_t swiss_random() noexcept;

// mum: the product of a and b as 128 bits, folded: the low half xor the high half.
inline uint64_t mum(uint64_t a, uint64_t b) noexcept {
#if defined(__SIZEOF_INT128__)
    unsigned __int128 r = static_cast<unsigned __int128>(a) * b;
    return static_cast<uint64_t>(r) ^ static_cast<uint64_t>(r >> 64);
#else
    uint64_t a_lo = a & 0xffffffffu, a_hi = a >> 32, b_lo = b & 0xffffffffu, b_hi = b >> 32;
    uint64_t lo_lo = a_lo * b_lo, hi_lo = a_hi * b_lo, lo_hi = a_lo * b_hi, hi_hi = a_hi * b_hi;
    uint64_t cross = (lo_lo >> 32) + (hi_lo & 0xffffffffu) + lo_hi;
    uint64_t hi = (hi_lo >> 32) + (cross >> 32) + hi_hi;
    uint64_t lo = (cross << 32) | (lo_lo & 0xffffffffu);
    return lo ^ hi;
#endif
}

// mum2: a and b become the low and the high half of their 128-bit product.
inline void mum2(uint64_t& a, uint64_t& b) noexcept {
#if defined(__SIZEOF_INT128__)
    unsigned __int128 r = static_cast<unsigned __int128>(a) * b;
    a = static_cast<uint64_t>(r);
    b = static_cast<uint64_t>(r >> 64);
#else
    uint64_t a_lo = a & 0xffffffffu, a_hi = a >> 32, b_lo = b & 0xffffffffu, b_hi = b >> 32;
    uint64_t lo_lo = a_lo * b_lo, hi_lo = a_hi * b_lo, lo_hi = a_lo * b_hi, hi_hi = a_hi * b_hi;
    uint64_t cross = (lo_lo >> 32) + (hi_lo & 0xffffffffu) + lo_hi;
    b = (hi_lo >> 32) + (cross >> 32) + hi_hi;
    a = (cross << 32) | (lo_lo & 0xffffffffu);
#endif
}

// The constants of wyhash (any odd numbers with a good mix of bits would do).
inline constexpr uint64_t wy0 = 0x2d358dccaa6c78a5u;
inline constexpr uint64_t wy1 = 0x8bb84b93962eacc9u;
inline constexpr uint64_t wy2 = 0x4b33a62ed433d4a3u;
inline constexpr uint64_t wy3 = 0x4d5a2da51de1aa47u;

// hash_u64: the seeded hash of a 64-bit integer (an integer or a pointer key).
inline uint64_t hash_u64(uint64_t v, uint64_t seed) noexcept {
    return mum(v ^ seed ^ wy0, wy1);
}

// hash_mem: the seeded hash of n bytes at p (p may be null when n is 0): wyhash. In
// runtime/src/map.cpp.
uint64_t hash_mem(const void* p, std::size_t n, uint64_t seed) noexcept;

} // namespace gp::detail::swiss
