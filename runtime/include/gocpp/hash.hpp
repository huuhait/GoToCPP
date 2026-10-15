// gocpp/hash.hpp: hashing for map keys (Phase 03 §2.3, step 3.7).
//
// gp::hash_of(k) is the hash of a key of any comparable Go type, and gp::hash<T> is the
// customization point behind it. The maps of gocpp/map.hpp hash each key once, when it is
// stored, and keep the hash with the key.
//
// The hash agrees with ==, Go's equality: equal keys hash the same. What each type does:
//
//     bool, integers, enums     the value, mixed
//     float32, float64          +0 and -0 hash the same (they are equal); every NaN hashes
//                               to one fixed value (below); any other value by its bits
//     complex numbers           the two parts, each like a float: Go's complex64 and
//                               complex128 (gp::complex, gocpp/complex.hpp, by its gp_hash),
//                               and std::complex or any type with real() and imag() that
//                               return floats (C++ types, Phase 04)
//     gp::string                its bytes
//     gp::ptr, gp::weak_ptr     the address, which is what == compares (not the owner)
//     a type with gp_hash()     v.gp_hash(): structs and arrays (below)
//     anything else             std::hash<T>, for C and C++ types used as keys (Phase 04)
//
// NaN: in Go, NaN != NaN, so every m[NaN] = v adds a new entry, a lookup never finds one,
// delete can't remove one, and only clear does. That follows from == alone; the hash only has
// to keep many NaN keys from landing in one bucket, so Go hashes a NaN to a random number.
// In gocpp, hash_of gives every NaN the same hash (a pure function, as == needs nothing more),
// and the map spreads them: when it stores a key that isn't equal to itself (a NaN, or a
// struct or array that holds one), it mixes the table's own count of such keys into the hash
// (gocpp/map.hpp). Such a key is never looked up again, so its hash only places it.
//
// The seed is fixed: gocpp's maps have no per-process random seed. The order of a range is
// not randomized on purpose (D8), but it may still differ from run to run: pointer keys hash
// by address, and addresses change between runs. A gocpp map also gives no protection against
// inputs chosen to collide (Go's random seed does); Phase 15's Swiss table can add a seed.
//
// The protocol for structs and arrays: a comparable struct that the emitter writes (step 3.8)
// gets a member
//
//     std::size_t gp_hash() const { return gp::hash_fields(X, Y, Z); }
//
// over its non-blank fields in declaration order (Go's == ignores blank fields, so the hash
// must too), next to its operator==; an empty struct is gp::hash_fields() (a constant).
// gp::array<T, N> (gocpp/array.hpp) has
//
//     std::size_t gp_hash() const { return gp::hash_elems(gp_elems.data(), N); }
//
// and a named type over a comparable type (gp::newtype, step 3.9) hashes the value it wraps:
// gp::hash_of(value). A member function works for every type the emitter writes: an explicit
// specialization of gp::hash would have to be written outside the package's namespace.
// Interfaces (Phase 05) panic with "hash of unhashable type" for a dynamic type that isn't
// comparable, so gp_hash and gp::hash_of may throw.
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <memory>
#include <type_traits>

namespace gp {

namespace detail {

// hash_seed: the start of every combined hash (hash_fields, hash_elems, hash_bytes).
inline constexpr uint64_t hash_seed = 0x9e3779b97f4a7c15u;

// hash_mix: splitmix64's finalizer, a bijection on 64 bits that spreads every input bit over
// the whole result.
constexpr uint64_t hash_mix(uint64_t x) noexcept {
    x ^= x >> 30;
    x *= 0xbf58476d1ce4e5b9u;
    x ^= x >> 27;
    x *= 0x94d049bb133111ebu;
    x ^= x >> 31;
    return x;
}

// hash_combine: h followed by x. One call is symmetric (mix(h ^ x)): the order matters only
// when the calls are chained from a seed, as hash_fields and hash_elems do, so (a, b) and
// (b, a) hash differently there.
constexpr std::size_t hash_combine(std::size_t h, std::size_t x) noexcept {
    return static_cast<std::size_t>(hash_mix(static_cast<uint64_t>(h) ^ static_cast<uint64_t>(x)));
}

constexpr std::size_t hash_int(uint64_t v) noexcept {
    return static_cast<std::size_t>(hash_mix(v ^ hash_seed));
}

// In runtime/src/map.cpp.
//
// hash_bytes: the hash of n bytes at p (p may be null when n is 0).
std::size_t hash_bytes(const void* p, std::size_t n) noexcept;

// hash_nan: the hash of every NaN.
inline constexpr std::size_t hash_nan = static_cast<std::size_t>(hash_mix(0x7ff8000000000001u));

template <go_float F>
std::size_t hash_float(F v) noexcept {
    if (v == 0) {
        return hash_int(0); // +0 == -0
    }
    if (v != v) {
        return hash_nan;
    }
    if constexpr (sizeof(F) == 4) {
        uint32_t bits;
        std::memcpy(&bits, &v, sizeof bits);
        return hash_int(bits);
    } else {
        uint64_t bits;
        std::memcpy(&bits, &v, sizeof bits);
        return hash_int(bits);
    }
}

// no_hash<T>: false, for the static_assert of a type without a hash.
template <typename T>
inline constexpr bool no_hash = false;

// has_gp_hash: a type with the member gp_hash() of the protocol above.
template <typename T>
concept has_gp_hash = requires(const T& v) {
    { v.gp_hash() } -> std::convertible_to<std::size_t>;
};

// complex_like: std::complex<float> and std::complex<double> (C++ types; Go's complex64 and
// complex128 are gp::complex, which has a gp_hash), without including <complex> here.
template <typename T>
concept complex_like = requires(const T& c) {
    { c.real() } -> go_float;
    { c.imag() } -> go_float;
};

} // namespace detail

// hash<T>: the hash of a T, for the maps. The primary template covers the basic types, the
// protocol member gp_hash() and std::hash; the runtime's own key types specialize it below.
// Only the runtime specializes it; generated types use gp_hash().
template <typename T>
struct hash {
    std::size_t operator()(const T& v) const {
        if constexpr (std::is_integral_v<T>) {
            return detail::hash_int(static_cast<uint64_t>(v));
        } else if constexpr (std::is_enum_v<T>) {
            return detail::hash_int(
                static_cast<uint64_t>(static_cast<std::underlying_type_t<T>>(v)));
        } else if constexpr (go_float<T>) {
            return detail::hash_float(v);
        } else if constexpr (detail::has_gp_hash<T>) {
            // Before complex_like: a Go struct may have methods named real and imag.
            return static_cast<std::size_t>(v.gp_hash());
        } else if constexpr (detail::complex_like<T>) {
            // In order from the seed, like hash_fields: hash_combine alone is symmetric.
            auto h = static_cast<std::size_t>(detail::hash_seed);
            h = detail::hash_combine(h, detail::hash_float(v.real()));
            return detail::hash_combine(h, detail::hash_float(v.imag()));
        } else if constexpr (requires { std::hash<T>{}(v); }) {
            return std::hash<T>{}(v);
        } else {
            static_assert(detail::no_hash<T>,
                          "gp::hash: this key type has no hash: a comparable struct or array "
                          "needs a gp_hash() member (gocpp/hash.hpp)");
            return 0;
        }
    }
};

template <>
struct hash<string> {
    std::size_t operator()(const string& s) const noexcept {
        return detail::hash_bytes(s.gp_data(), static_cast<std::size_t>(gp::len(s)));
    }
};

// Pointers hash by address: two pointers are equal when they point at the same object,
// whatever their owners are (an interior pointer &p.f and another &p.f share one block anyway).
template <typename T>
struct hash<ptr<T>> {
    std::size_t operator()(const ptr<T>& p) const noexcept {
        return detail::hash_int(
            static_cast<uint64_t>(reinterpret_cast<std::uintptr_t>(p.gp_get())));
    }
};

// A weak pointer hashes by the address it was made from (its == also compares the owner: two
// equal weak pointers have the same address, so they hash the same).
template <typename T>
struct hash<weak_ptr<T>> {
    std::size_t operator()(const weak_ptr<T>& w) const noexcept {
        return detail::hash_int(static_cast<uint64_t>(w.gp_addr()));
    }
};

// hash_of(v): gp::hash<T>{}(v).
template <typename T>
std::size_t hash_of(const T& v) {
    return gp::hash<T>{}(v);
}

// hash_fields(a, b, ...): the hash of a struct with these fields, in this order (the gp_hash
// of a generated struct). hash_fields() is the hash of an empty struct.
template <typename... T>
std::size_t hash_fields(const T&... fields) {
    auto h = static_cast<std::size_t>(detail::hash_seed);
    ((h = detail::hash_combine(h, gp::hash_of(fields))), ...);
    return h;
}

// hash_elems(p, n): the hash of the n elements at p, in order (the gp_hash of an array).
template <typename T>
std::size_t hash_elems(const T* p, int64_t n) {
    auto h = static_cast<std::size_t>(detail::hash_seed);
    for (int64_t i = 0; i < n; i++) {
        h = detail::hash_combine(h, gp::hash_of(p[i]));
    }
    return h;
}

} // namespace gp
