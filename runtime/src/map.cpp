// The parts of maps (gocpp/map.hpp) and of key hashing (gocpp/hash.hpp) that are not
// templates: the hash of a run of bytes and the nil map panic.
#include <gocpp/rt.hpp>
#include <gocpp/swiss.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <atomic>
#include <random>

namespace gp {

namespace detail {

namespace {

// xxHash64's primes. The round is xxHash64's, with one accumulator (keys are short).
constexpr uint64_t prime1 = 0x9e3779b185ebca87u;
constexpr uint64_t prime2 = 0xc2b2ae3d27d4eb4fu;
constexpr uint64_t prime3 = 0x165667b19e3779f9u;

constexpr uint64_t rotl(uint64_t x, int r) {
    return (x << r) | (x >> (64 - r));
}

uint64_t step(uint64_t h, uint64_t w) {
    h ^= rotl(w * prime2, 31) * prime1;
    return rotl(h, 27) * prime1 + prime3;
}

} // namespace

std::size_t hash_bytes(const void* p, std::size_t n) noexcept {
    auto b = static_cast<const unsigned char*>(p);
    uint64_t h = hash_seed ^ (static_cast<uint64_t>(n) * prime1);
    while (n >= 8) {
        uint64_t w;
        std::memcpy(&w, b, 8);
        h = step(h, w);
        b += 8;
        n -= 8;
    }
    if (n > 0) {
        uint64_t w = 0;
        std::memcpy(&w, b, n);
        h = step(h, w);
    }
    return static_cast<std::size_t>(hash_mix(h));
}

namespace swiss {

namespace {

uint64_t read8(const unsigned char* p) {
    uint64_t v;
    std::memcpy(&v, p, 8);
    return v;
}
uint64_t read4(const unsigned char* p) {
    uint32_t v;
    std::memcpy(&v, p, 4);
    return v;
}

// The seed of the process: from the operating system's entropy, once.
uint64_t process_seed() {
    static const uint64_t seed = [] {
        std::random_device rd;
        uint64_t a = rd();
        uint64_t b = rd();
        return (a << 32) ^ b ^ hash_mix(reinterpret_cast<std::uintptr_t>(&rd));
    }();
    return seed;
}

} // namespace

uint64_t swiss_random() noexcept {
    // One stream per thread, so that no lock or atomic operation is needed: seeded from the
    // process seed and a number per thread.
    static std::atomic<uint64_t> threads{0};
    thread_local uint64_t state =
        hash_mix(process_seed() + 0x9e3779b97f4a7c15u * (threads.fetch_add(1) + 1));
    state += 0x9e3779b97f4a7c15u;
    return hash_mix(state);
}

// wyhash (final version 4, by Wang Yi, public domain), with this file's secrets.
uint64_t hash_mem(const void* key, std::size_t len, uint64_t seed) noexcept {
    const auto* p = static_cast<const unsigned char*>(key);
    seed ^= mum(seed ^ wy0, wy1);
    uint64_t a, b;
    if (len <= 16) {
        if (len >= 4) {
            a = (read4(p) << 32) | read4(p + ((len >> 3) << 2));
            b = (read4(p + len - 4) << 32) | read4(p + len - 4 - ((len >> 3) << 2));
        } else if (len > 0) {
            a = (static_cast<uint64_t>(p[0]) << 16) | (static_cast<uint64_t>(p[len >> 1]) << 8) |
                p[len - 1];
            b = 0;
        } else {
            a = b = 0;
        }
    } else {
        std::size_t i = len;
        if (i > 48) {
            uint64_t see1 = seed;
            uint64_t see2 = seed;
            do {
                seed = mum(read8(p) ^ wy1, read8(p + 8) ^ seed);
                see1 = mum(read8(p + 16) ^ wy2, read8(p + 24) ^ see1);
                see2 = mum(read8(p + 32) ^ wy3, read8(p + 40) ^ see2);
                p += 48;
                i -= 48;
            } while (i > 48);
            seed ^= see1 ^ see2;
        }
        while (i > 16) {
            seed = mum(read8(p) ^ wy1, read8(p + 8) ^ seed);
            i -= 16;
            p += 16;
        }
        a = read8(p + i - 16);
        b = read8(p + i - 8);
    }
    a ^= wy1;
    b ^= seed;
    mum2(a, b);
    return mum(a ^ wy0 ^ len, b ^ wy1);
}

} // namespace swiss

void (*concurrent_hook)(const char* msg) = nullptr;

void panic_concurrent(const char* msg) {
    if (concurrent_hook != nullptr) {
        concurrent_hook(msg);
    }
    // Go's throw: a fatal error that no recover stops.
    fatal(msg);
}

void panic_nil_map() {
    // Go's plainError: a runtime.Error whose text has no "runtime error: " prefix.
    throw panic_error{"assignment to entry in nil map"_gs, {panic_identity::gp_runtime, 0, 0}};
}

} // namespace detail

} // namespace gp
