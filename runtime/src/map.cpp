// The parts of maps (gocpp/map.hpp) and of key hashing (gocpp/hash.hpp) that are not
// templates: the hash of a run of bytes and the nil map panic.
#include <gocpp/rt.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>

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

void panic_nil_map() {
    // Go's plainError: a runtime.Error whose text has no "runtime error: " prefix.
    throw panic_error{"assignment to entry in nil map"_gs, {panic_identity::gp_runtime, 0, 0}};
}

} // namespace detail

} // namespace gp
