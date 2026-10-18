// gocpp/map.hpp: gp::map<K, V>, Go's map type map[K]V.
//
// A map is a reference to a hash table: one std::shared_ptr to the table (16 bytes), like
// every heap block (Phase 03 §2.1). Copying a map copies the reference, so two copies see each
// other's writes, as in Go. The zero value is nil: no table. v1's table is a
// std::unordered_map (Phase 15 swaps in a Swiss table, like Go 1.24's); every key is stored
// with its hash (gocpp/hash.hpp), computed once.
//
// What generated code writes (Phase 03 §2.3; the emitter is step 3.7b):
//
//     var m map[K]V, m = nil    gp::map<K, V> m;  m = nullptr
//     m == nil, m != nil        m == nullptr, m != nullptr (a map compares only with nil)
//     make(map[K]V), (.., n)    gp::make_map<K, V>(), gp::make_map<K, V>(n)
//     map[K]V{k1: v1, k2: v2}   gp::map_of<K, V>({{k1, v1}, {k2, v2}})  in gc's order (below)
//     v := m[k]                 V v = gp::get(m, k);         the zero value if k is missing
//     v, ok := m[k]             auto gp_t = gp::lookup(m, k);  a std::tuple<V, bool>
//     m[k] = v                  gp::set(m, k, v);
//     m[k] += v, m[k]++         gp::ref(m, k) += v;  gp::ref(m, k)++;   insert-or-get
//     delete(m, k)              gp::erase(m, k);
//     len(m), clear(m)          gp::len(m), gp::clear(m)
//     for k, v := range m       for (auto gp_it0 = gp::range_map(m); gp::range_next(gp_it0);) {
//                                   K k = gp_it0.gp_key();
//                                   V v = gp_it0.gp_value();
//                                   ...
//                               }
//
// Reads never insert (unlike C++'s operator[]). A nil map reads like an empty one: gp::get
// gives the zero value, gp::lookup {zero, false}, gp::len 0, gp::erase and gp::clear do
// nothing and a range has no iterations. Writes to a nil map (gp::set, gp::ref) panic with
// Go's "assignment to entry in nil map". make's size hint only reserves room (for at most 2^20
// entries): a negative one counts as 0, as in Go, and a hint never fails.
//
// Keys can have every comparable type that gocpp/hash.hpp hashes: bools, numbers, strings,
// pointers, and structs and arrays through their gp_hash(). Equality is K's ==: so NaN keys
// work as in Go: each m[NaN] = v adds an entry, gp::get and gp::lookup never find one,
// gp::erase can't remove one, a range visits them, and gp::clear removes them. +0 and -0 are
// one key. Like Go, a store to a key that is already there replaces the stored key with the
// new one (m[-0.0] = 1; m[+0.0] = 2 leaves the key +0), except for integer and pointer keys,
// where equal keys are identical.
//
// Range: a range evaluates the map once (gp::range_map holds a reference to its table, so the
// table lives on even if the variable is reassigned) and iterates over a snapshot of its
// entries: each key with its hash and a pointer to its entry. Go allows inserts and deletes
// during a range: an entry deleted before it is reached is not produced, and an entry added
// during the range may or may not be (gocpp never produces one). While nothing was deleted,
// range_next uses the entry pointers as they are (std::unordered_map keeps its entries in place
// when it grows); after a delete or a clear it looks each key up again, and skips the keys that
// are gone. A key that isn't equal to itself (a NaN) can't be looked up or deleted, so it is
// still there unless a clear came. The value is read when the entry is reached, so it is the
// current one, as in Go. The order is unspecified and not randomized on purpose (D8), but it
// can differ between runs (pointer keys hash by address): don't depend on it. The snapshot
// costs one allocation and a copy of the keys per range, also for a small map (Phase 15: a
// small snapshot buffer inside map_range, or a table that iterates without one).
//
// Lifetimes: a map's table is released through gp::detail::release (ptr.hpp), so freeing a
// long chain of maps (a list whose nodes hold maps that hold the next node) never runs out of
// stack. Values and keys are destroyed exactly once: when they are replaced, deleted or
// cleared, or with the table. gp::set takes its value by value, so a store can't free its own
// source while it reads it (gp::set(m, k, gp::get(m, k).next) style).
//
// Threads: as in Go, any number of goroutines may read one map at the same time (gp::get,
// gp::lookup, gp::len and range only read); a write during another goroutine's read or write
// is a data race (Go detects some and dies with "concurrent map writes"; gocpp doesn't).
//
// Like every runtime type, gp::map has only operators and gp_-prefixed members, and everything
// else is a free function in gp:: (Phase 03 §2.1); the runtime's calls inside these templates
// are qualified, so argument-dependent lookup never picks a user's function of the same name.
// It accepts incomplete K and V (type Node struct{ kids map[string]Node }); the functions need
// complete ones.
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>

#include <cstddef>
#include <cstdint>
#include <initializer_list>
#include <memory>
#include <new>
#include <tuple>
#include <type_traits>
#include <unordered_map>
#include <utility>
#include <vector>

namespace gp {

template <typename K, typename V>
class map;

namespace detail {

// map_key: a stored key and its hash. The key is mutable because Go replaces the stored key on
// a store to an equal key (above); the replacement is equal and hashes the same, so the table
// stays valid.
template <typename K>
struct map_key {
    mutable K gp_k;
    std::size_t gp_h;
};

// map_probe: a key to look up, with its hash, without copying the key into a map_key.
template <typename K>
struct map_probe {
    const K& gp_k;
    std::size_t gp_h;
};

template <typename K>
struct map_hasher {
    using is_transparent = void;
    std::size_t operator()(const map_key<K>& k) const noexcept { return k.gp_h; }
    std::size_t operator()(const map_probe<K>& k) const noexcept { return k.gp_h; }
};

template <typename K>
struct map_equal {
    using is_transparent = void;
    bool operator()(const map_key<K>& a, const map_key<K>& b) const {
        return a.gp_h == b.gp_h && a.gp_k == b.gp_k;
    }
    bool operator()(const map_probe<K>& a, const map_key<K>& b) const {
        return a.gp_h == b.gp_h && a.gp_k == b.gp_k;
    }
    bool operator()(const map_key<K>& a, const map_probe<K>& b) const {
        return a.gp_h == b.gp_h && a.gp_k == b.gp_k;
    }
};

template <typename K, typename V>
using map_table = std::unordered_map<map_key<K>, V, map_hasher<K>, map_equal<K>>;

template <typename K, typename V>
using map_entry = typename map_table<K, V>::value_type;

// map_impl: the table, and the counts that a range checks: gp_erases counts the deletes and
// clears that removed something, gp_clears the clears. gp_nans counts the stored keys that
// aren't equal to themselves (map_slot).
template <typename K, typename V>
struct map_impl {
    map_table<K, V> gp_t;
    uint64_t gp_erases = 0;
    uint64_t gp_clears = 0;
    uint64_t gp_nans = 0;
};

// key_update<K>: a store to a key that is there replaces the stored key, as Go's
// t.NeedKeyUpdate(): equal floats can differ (+0 and -0), equal strings can hold different
// memory, and structs and arrays can hold either. Equal integers and pointers are identical.
template <typename T>
inline constexpr bool is_pointer_key = false;
template <typename T>
inline constexpr bool is_pointer_key<ptr<T>> = true;
template <typename T>
inline constexpr bool is_pointer_key<weak_ptr<T>> = true;

// key_base<K>: K, or the underlying type of a named type (gp::newtype, step 3.9), whose keys
// are replaced when that type's are.
template <typename K>
struct key_base {
    using type = K;
};
template <typename K>
    requires requires { typename K::gp_underlying; }
struct key_base<K> {
    using type = typename K::gp_underlying;
};

template <typename K, typename B = typename key_base<K>::type>
inline constexpr bool key_update =
    !std::is_integral_v<B> && !std::is_enum_v<B> && !is_pointer_key<B>;

// self_equal: k == k, false for a NaN (or a struct or array that holds one). A function, so
// that compilers don't warn about a comparison of a value with itself.
template <typename K>
bool self_equal(const K& a, const K& b) {
    return a == b;
}

// panic_nil_map: Go's "assignment to entry in nil map". In runtime/src/map.cpp.
[[noreturn]] void panic_nil_map();

// check_empty_key: Go checks the key of a read or a delete in a nil or empty map without hashing
// it, and panics with its own text ("hash of unhashable type: []int", not a runtime error) when
// the key is, or holds (in a struct field, an array element or a boxed interface value), an
// interface value whose dynamic type has no hash (gp::check_key, gocpp/hash.hpp).
template <typename K, typename V>
void check_empty_key([[maybe_unused]] const map_impl<K, V>* impl, [[maybe_unused]] const K& k) {
    if constexpr (hash_may_panic<K>) {
        if (impl == nullptr || impl->gp_t.empty()) {
            gp::check_key(k);
        }
    }
}

// map_find: the entry of key k (with hash h), or end().
template <typename K, typename V>
typename map_table<K, V>::iterator map_find(map_impl<K, V>& m, const K& k, std::size_t h) {
    return m.gp_t.find(map_probe<K>{k, h});
}

// map_slot: the value of key k, inserted as the zero value if k isn't there (Go's mapassign).
// An equal key that is there is replaced with k (key_update).
template <typename K, typename V>
V& map_slot(map_impl<K, V>& m, const K& k) {
    std::size_t h = gp::hash_of(k);
    auto it = detail::map_find(m, k, h);
    if (it != m.gp_t.end()) {
        if constexpr (key_update<K>) {
            it->first.gp_k = k;
        }
        return it->second;
    }
    if (!detail::self_equal(k, k)) {
        // A NaN (or a struct or array holding one): every NaN has the same hash_of, so mix in
        // this table's count, which spreads them over the buckets. The key is never looked up
        // again, so nothing needs its hash_of. A count per table (not per thread) keeps it
        // deterministic and safe with goroutines (a write to a map is never concurrent).
        h = detail::hash_combine(h, static_cast<std::size_t>(++m.gp_nans));
    }
    return m.gp_t.try_emplace(map_key<K>{k, h}).first->second;
}

// map_snap: one entry of a range's snapshot.
template <typename K, typename V>
struct map_snap {
    K gp_k;
    std::size_t gp_h;
    map_entry<K, V>* gp_e;
};

} // namespace detail

// map<K, V> is Go's map[K]V. The zero value is nil.
template <typename K, typename V>
class map {
public:
    // nil.
    constexpr map() noexcept = default;
    constexpr map(std::nullptr_t) noexcept {}

    // A map with this table, for the runtime's own code (make_map, map_of).
    explicit map(std::shared_ptr<detail::map_impl<K, V>> impl) noexcept
        : gp_impl_(std::move(impl)) {}

    // A copy shares the table (one atomic increment); a move leaves nil behind.
    map(const map&) noexcept = default;
    map(map&&) noexcept = default;

    // The assignments take the new table before they drop the old one, which goes through
    // detail::release.
    map& operator=(const map& o) noexcept {
        if (this != &o) {
            map copy(o);
            *this = std::move(copy);
        }
        return *this;
    }
    map& operator=(map&& o) noexcept {
        if (this != &o) {
            std::shared_ptr<detail::map_impl<K, V>> old = std::move(gp_impl_);
            gp_impl_ = std::move(o.gp_impl_);
            if (old.get() != nullptr) {
                detail::release(old);
            }
        }
        return *this;
    }

    ~map() {
        if (gp_impl_.get() != nullptr) {
            detail::release(gp_impl_);
        }
    }

    // m == nil. Go compares maps only with nil.
    friend bool operator==(const map& a, std::nullptr_t) noexcept {
        return a.gp_impl_.get() == nullptr;
    }

    // gp_get: the table (null for nil). For the runtime's own code.
    detail::map_impl<K, V>* gp_get() const noexcept { return gp_impl_.get(); }

private:
    std::shared_ptr<detail::map_impl<K, V>> gp_impl_;
};

static_assert(sizeof(map<int64_t, int64_t>) == 16,
              "gp::map is 16 bytes: a std::shared_ptr to the table (D15)");

// make_map<K, V>(): make(map[K]V), an empty map (not nil). make_map<K, V>(n): with room for
// about n entries, but for at most detail::max_map_reserve (2^20) of them: reserve allocates
// std::unordered_map's bucket array at once, while Go allocates a huge map's table only as it
// fills, so make(map[string]int, 1<<40) runs in Go and must not fail here. A larger table grows
// as it fills. A negative n counts as 0, as in Go, and so does an n whose room can't be
// allocated: a hint never panics.
template <typename K, typename V>
map<K, V> make_map() {
    return map<K, V>(std::make_shared<detail::map_impl<K, V>>());
}

namespace detail {
inline constexpr uint64_t max_map_reserve = uint64_t(1) << 20;
} // namespace detail

template <typename K, typename V, go_int H>
map<K, V> make_map(H hint) {
    map<K, V> m = gp::make_map<K, V>();
    bool positive;
    if constexpr (std::is_signed_v<H>) {
        positive = hint > 0;
    } else {
        positive = hint != 0;
    }
    if (positive) {
        uint64_t n = static_cast<uint64_t>(hint);
        if (n > detail::max_map_reserve) {
            n = detail::max_map_reserve;
        }
        try {
            m.gp_get()->gp_t.reserve(static_cast<std::size_t>(n));
        } catch (const std::bad_alloc&) {
            // No room: an empty table, which grows as it fills. Under run_main's new handler a
            // failed allocation is a fatal error instead, as in Go, so this is for C++ code
            // that calls the runtime without run_main.
        }
    }
    return m;
}

// map_of<K, V>({{k1, v1}, {k2, v2}}): the composite literal map[K]V{k1: v1, k2: v2}. The
// pairs are evaluated left to right (a braced list) and stored in that order, so of two equal
// keys the later one wins, and its key is the one kept. Go rejects equal constant keys at
// compile time, but a non-constant key can equal another key. The spec leaves the order of the
// stores open; gc stores the static entries first (both key and value constants or static
// composite literals, which have no side effects), then the others in source order. So the
// emitter passes the entries in that order: k := 0.0; map[float64]int{-k: 1, 0: 2} stores 0: 2
// first, and gives {-0: 1}, as in Go. map_of<K, V>({}) is empty, not nil.
template <typename K, typename V>
map<K, V> map_of(std::initializer_list<std::pair<K, V>> elems) {
    map<K, V> m = gp::make_map<K, V>(static_cast<int64_t>(elems.size()));
    detail::map_impl<K, V>& impl = *m.gp_get();
    for (const std::pair<K, V>& e : elems) {
        detail::map_slot(impl, e.first) = e.second;
    }
    return m;
}

// len(m): the number of entries; 0 for nil.
template <typename K, typename V>
int64_t len(const map<K, V>& m) noexcept {
    detail::map_impl<K, V>* impl = m.gp_get();
    return impl == nullptr ? 0 : static_cast<int64_t>(impl->gp_t.size());
}

// get(m, k): m[k], the value of k, or the zero value of V if k isn't there. Never inserts.
template <typename K, typename V>
V get(const map<K, V>& m, const std::type_identity_t<K>& k) {
    detail::map_impl<K, V>* impl = m.gp_get();
    detail::check_empty_key(impl, k);
    if (impl != nullptr) {
        auto it = detail::map_find(*impl, k, gp::hash_of(k));
        if (it != impl->gp_t.end()) {
            return it->second;
        }
    }
    return V();
}

// lookup(m, k): v, ok := m[k], as a std::tuple (the multi-value rule of Phase 02): the value
// and true, or the zero value and false. Never inserts.
template <typename K, typename V>
std::tuple<V, bool> lookup(const map<K, V>& m, const std::type_identity_t<K>& k) {
    detail::map_impl<K, V>* impl = m.gp_get();
    detail::check_empty_key(impl, k);
    if (impl != nullptr) {
        auto it = detail::map_find(*impl, k, gp::hash_of(k));
        if (it != impl->gp_t.end()) {
            return std::tuple<V, bool>(it->second, true);
        }
    }
    return std::tuple<V, bool>(V(), false);
}

// set(m, k, v): m[k] = v. Panics on a nil map with "assignment to entry in nil map". v is
// taken by value: it is a copy before the old value is dropped.
template <typename K, typename V>
void set(const map<K, V>& m, const std::type_identity_t<K>& k, std::type_identity_t<V> v) {
    detail::map_impl<K, V>* impl = m.gp_get();
    if (impl == nullptr) [[unlikely]] {
        detail::panic_nil_map();
    }
    detail::map_slot(*impl, k) = std::move(v);
}

// ref(m, k): the value of k, inserted as the zero value first if it isn't there, for
// m[k] op= v and m[k]++ (gp::ref(m, k) += v). Panics on a nil map like set. The reference
// stays valid until k is deleted or the map is cleared (the table keeps entries in place when
// it grows).
//
// Go's order for m[k] op= y (checked with go run), for the emitter (steps 3.7b and 3.12):
//   - y is evaluated before the entry is taken; C++17 evaluates the right side of a compound
//     assignment first, so gp::ref(m, k) += y matches, also when y deletes or stores k
//     (m["x"] += f(), where f deletes "x", leaves m["x"] == 1, as in Go). The key k itself is
//     evaluated before y in Go: when either has side effects, k goes into a temporary first.
//   - m[k] /= y and m[k] %= y with y == 0 panic WITHOUT inserting k: gc rewrites them to
//     m[k] = m[k] / y, so the divisor check comes before the store. Emit
//     gp::set(m, k, gp::div(gp::get(m, k), y)) (with k and y in temporaries), not gp::ref.
//   - m[k] <<= n with a negative n panics AFTER inserting k (the shift check comes after the
//     entry is taken): gp::ref(m, k) = gp::shl(gp::ref(m, k), n) inserts first, as Go.
//   - Never auto& r = gp::ref(m, k); r = T(r op f()): if f deletes k, r dangles. Evaluate the
//     right side into a temporary first, then gp::ref(m, k) op= t.
template <typename K, typename V>
V& ref(const map<K, V>& m, const std::type_identity_t<K>& k) {
    detail::map_impl<K, V>* impl = m.gp_get();
    if (impl == nullptr) [[unlikely]] {
        detail::panic_nil_map();
    }
    return detail::map_slot(*impl, k);
}

// erase(m, k): delete(m, k). Nothing happens if k isn't there (a NaN never is) or m is nil.
template <typename K, typename V>
void erase(const map<K, V>& m, const std::type_identity_t<K>& k) {
    detail::map_impl<K, V>* impl = m.gp_get();
    detail::check_empty_key(impl, k);
    if (impl == nullptr) {
        return;
    }
    auto it = detail::map_find(*impl, k, gp::hash_of(k));
    if (it == impl->gp_t.end()) {
        return;
    }
    impl->gp_erases++;
    impl->gp_t.erase(it);
}

// clear(m): deletes every entry, also the NaN keys (Go 1.21). Nothing happens for nil.
template <typename K, typename V>
void clear(const map<K, V>& m) {
    detail::map_impl<K, V>* impl = m.gp_get();
    if (impl == nullptr || impl->gp_t.empty()) {
        return;
    }
    impl->gp_erases++;
    impl->gp_clears++;
    impl->gp_t.clear();
}

// map_range is the state of `for k, v := range m` (above). After each successful
// range_next, gp_key() and gp_value() are the entry's key and its current value; they stay
// valid until the next range_next or a delete or clear of that map. Copy them (or read the
// value later with range_value).
template <typename K, typename V>
struct map_range {
    map<K, V> gp_m;                              // the map, evaluated once
    std::vector<detail::map_snap<K, V>> gp_snap; // its entries when the range started
    std::size_t gp_next = 0;                     // the next entry of gp_snap
    detail::map_entry<K, V>* gp_cur = nullptr;   // the current entry
    uint64_t gp_erases = 0;                      // the map's counts when the range started
    uint64_t gp_clears = 0;

    const K& gp_key() const noexcept { return gp_cur->first.gp_k; }
    const V& gp_value() const noexcept { return gp_cur->second; }
};

// range_map(m) starts a range over m.
template <typename K, typename V>
map_range<K, V> range_map(const map<K, V>& m) {
    map_range<K, V> r;
    r.gp_m = m;
    detail::map_impl<K, V>* impl = m.gp_get();
    if (impl != nullptr) {
        r.gp_snap.reserve(impl->gp_t.size());
        for (detail::map_entry<K, V>& e : impl->gp_t) {
            r.gp_snap.push_back(detail::map_snap<K, V>{e.first.gp_k, e.first.gp_h, &e});
        }
        r.gp_erases = impl->gp_erases;
        r.gp_clears = impl->gp_clears;
    }
    return r;
}

// range_next steps to the next entry of the range that is still in the map, and reports
// whether there is one.
template <typename K, typename V>
bool range_next(map_range<K, V>& it) {
    detail::map_impl<K, V>* impl = it.gp_m.gp_get();
    while (it.gp_next < it.gp_snap.size()) {
        detail::map_snap<K, V>& s = it.gp_snap[it.gp_next++];
        if (impl->gp_erases == it.gp_erases) {
            it.gp_cur = s.gp_e; // nothing was removed: every entry is where it was
            return true;
        }
        if (detail::self_equal(s.gp_k, s.gp_k)) {
            auto f = detail::map_find(*impl, s.gp_k, s.gp_h);
            if (f == impl->gp_t.end()) {
                continue; // deleted
            }
            it.gp_cur = &*f;
            return true;
        }
        if (impl->gp_clears == it.gp_clears) {
            it.gp_cur = s.gp_e; // a NaN key: only a clear removes it
            return true;
        }
    }
    return false;
}

// range_value(it): the value of the range's current entry as it is now, when code ran since
// range_next: the emitter reads it after the targets' operands of for k, v = range m (step
// 3.7b), whose calls may store into the entry, delete it or clear the map (gp_value() would
// dangle then). It gives the entry's current value, as Go does, or the zero value if the entry
// is gone (Go reads its cleared slot).
template <typename K, typename V>
V range_value(const map_range<K, V>& it) {
    detail::map_impl<K, V>* impl = it.gp_m.gp_get();
    if (impl->gp_erases == it.gp_erases) {
        return it.gp_cur->second; // nothing was removed since the range started
    }
    const detail::map_snap<K, V>& s = it.gp_snap[it.gp_next - 1];
    if (detail::self_equal(s.gp_k, s.gp_k)) {
        auto f = detail::map_find(*impl, s.gp_k, s.gp_h);
        return f == impl->gp_t.end() ? V() : f->second;
    }
    return impl->gp_clears == it.gp_clears ? s.gp_e->second : V(); // a NaN key: only a clear
}

} // namespace gp
