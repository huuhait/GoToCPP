// gocpp/map.hpp: gp::map<K, V>, Go's map type map[K]V, as a Swiss table (ADR-059).
//
// A map is a reference to a hash table: one std::shared_ptr to the table (16 bytes), like
// every heap block (Phase 03 §2.1). Copying a map copies the reference, so two copies see each
// other's writes, as in Go. The zero value is nil: no table.
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
// Go's "assignment to entry in nil map". make's size hint only reserves room (for at most 2^22
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
// The table (gocpp/swiss.hpp has the control-byte groups and the hashes): a power-of-two
// number of slots (at least one group: 16 with SSE2, else 8), each holding a key and a value
// side by side, and one control byte per slot: the low 7 bits of the key's hash (H2) when the
// slot is full, "empty" (0x80) or "deleted" (a tombstone, 0xFE) when it holds nothing. A probe
// starts at the slot that the rest of the hash (H1) names, compares the control bytes of a
// whole group of slots with H2 at once (SSE2 or 64-bit arithmetic), compares the keys of
// the slots that match, and goes on to the next group (triangular steps) only if the group has
// no empty slot. The table holds at most 7/8 of its slots (full and tombstones together): a
// store into a full table doubles it, or, if tombstones took most of the room, rehashes it
// into a table of the same size (if at most 9/16 of the slots are live entries). A table that was
// never written to has no memory (a static group of empty bytes serves the probe).
//
// Every map has its own random hash seed, so the hash of a key, the layout of the table and the
// order of a range differ from map to map and from run to run, as in Go (and an input chosen to
// collide can't know the seed). A NaN key is never found again, so it gets a new hash each
// time that it is placed (hash_of gives every NaN one hash): thousands of NaN keys don't share
// one probe sequence.
//
// Range (D8, as Go): the iteration starts at a random slot and goes round the table once. It
// holds one reference to the table's memory (a counted block: the header, control bytes and
// slots are one allocation), which is what makes the rules of the Go spec cheap to keep:
//   - A delete during the range turns the slot into a tombstone, never an empty slot, and a
//     store doesn't reuse a tombstone while a range holds the table (it takes an empty slot,
//     or grows): an entry deleted before the range reaches it is not produced, and the slot of
//     the entry that the range just produced stays "deleted" until the range is over, so
//     range_value can tell that the entry is gone.
//   - A store never moves an entry, except when the table grows or is rehashed. Then, if a
//     range holds the old block, the entries are copied (not moved) into the new block and the
//     old block lives on with the range, which finishes its round over the OLD block, as Go's
//     runtime does with the old buckets: for each full slot of the old block it looks the key up
//     in the live table, and produces the entry from there (its current key and value), or skips
//     the key if it is gone. A key that isn't equal to itself (a NaN) can't be looked up or
//     deleted, so it is produced from the old block, unless a clear came since. Each existing
//     entry is produced exactly once, an entry that the range didn't reach and that was deleted
//     is not produced, an entry added during the range may or may not be (it is produced only
//     if it lands in a slot ahead and the table doesn't grow), never twice.
//   - clear during a range empties the live block (tombstones while a range holds it).
// The value is read when the entry is reached, so it is the current one, as in Go. gp_key() and
// gp_value() are references into the table: copy them (the emitter does) before code runs that
// can store, delete or clear; gp::range_value gives the entry's value as it is later.
//
// A reference from gp::ref (and gp_value) is valid until the next store of a new key (the table
// can grow), delete or clear of that map.
//
// Lifetimes: a map's table is released through gp::detail::release (ptr.hpp), so freeing a
// long chain of maps (a list whose nodes hold maps that hold the next node) never runs out of
// stack. Values and keys are destroyed exactly once: when they are replaced, deleted or
// cleared, or with the table. gp::set takes its value by value, so a store can't free its own
// source while it reads it (gp::set(m, k, gp::get(m, k).next) style). A throwing hash or ==
// (a map[any]V key whose dynamic type isn't comparable panics) throws before the table is
// touched, so it never corrupts it; so does a key or value whose copy throws. Growing moves
// the entries, so K and V must not throw when they are moved (every gocpp type is that); a K
// or V whose move may throw is copied, and the copies are rolled back if one throws.
//
// A value that can't be copied (a C++ std::unique_ptr, which hand-written C++ can store) is moved
// when the table grows, also while a range holds the old block. The range then reads the entry
// from the live table by its key (gp_key() and range_value are right), except an entry with a
// NaN key, which can't be looked up: its value is read from the old block, where it was moved
// from. Emitted code can't reach this: a value that isn't copyable is not ranged over by value
// (the emitter rejects it), fmt and reflect need copyable values.
//
// Threads: as in Go, any number of goroutines may read one map at the same time (gp::get,
// gp::lookup, gp::len and range only read); a write during another goroutine's read or write
// is a data race. As Go does, a write, read or range that sees another write in progress dies
// with "fatal error: concurrent map writes" (or "concurrent map read and map write"): a flag
// that a write sets while it runs, checked without any atomic read-modify-write. It catches
// some races only, as in Go.
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
#include <gocpp/swiss.hpp>

#include <atomic>
#include <bit>
#include <cstddef>
#include <cstdint>
#include <cstring>
#include <initializer_list>
#include <memory>
#include <new>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

template <typename K, typename V>
class map;

namespace detail {

namespace sw = swiss;

// panic_nil_map: Go's "assignment to entry in nil map". In runtime/src/map.cpp.
[[noreturn]] void panic_nil_map();

// panic_concurrent: Go's fatal error for a map used by two goroutines at once, msg being
// "concurrent map writes" and the like. In runtime/src/map.cpp. concurrent_hook is for the tests:
// when it is set, panic_concurrent calls it first (it can throw instead of letting the process
// die).
extern void (*concurrent_hook)(const char* msg);
[[noreturn]] void panic_concurrent(const char* msg);

// map_cell: a slot's key and value. The table constructs and destroys them by hand.
template <typename K, typename V>
struct map_cell {
    K gp_k;
    V gp_v;
};

// map_store: the header of a table's memory block, followed by the control bytes (capacity of
// them and a copy of the first group, so that a group can be loaded at any slot without
// wrapping) and the cells. Counted: the map holds one reference and each range another.
struct map_store {
    std::atomic<uint32_t> gp_refs{1};
    std::size_t gp_cap = 0;
};

inline constexpr std::size_t store_header = 16;
inline constexpr std::size_t group_width = sw::group::width;
inline constexpr std::size_t min_capacity = group_width;

template <typename Cell>
inline constexpr std::size_t store_align = alignof(Cell) > 16 ? alignof(Cell) : 16;

template <typename Cell>
constexpr std::size_t cells_offset(std::size_t cap) noexcept {
    std::size_t end = store_header + cap + group_width;
    return (end + alignof(Cell) - 1) & ~(alignof(Cell) - 1);
}

inline int8_t* store_ctrl(map_store* s) noexcept {
    return reinterpret_cast<int8_t*>(s) + store_header;
}

template <typename Cell>
Cell* store_cells(map_store* s) noexcept {
    return reinterpret_cast<Cell*>(reinterpret_cast<unsigned char*>(s) +
                                   cells_offset<Cell>(s->gp_cap));
}

// store_bytes<Cell>(cap, bytes): the size of a block for cap slots; false if it overflows or is
// more than max_alloc (Go's MulUintptr check in makemap).
template <typename Cell>
bool store_bytes(std::size_t cap, std::size_t& bytes) noexcept {
    std::size_t cells;
    if (__builtin_mul_overflow(cap, sizeof(Cell), &cells)) {
        return false;
    }
    std::size_t head = cells_offset<Cell>(cap);
    if (__builtin_add_overflow(head, cells, &bytes)) {
        return false;
    }
    return bytes <= static_cast<std::size_t>(max_alloc); // slice.hpp: Go's maxAlloc, 2^48
}

// store_new<Cell>(cap): a block for cap slots (a power of two, at least one group), all empty,
// with one reference. A size that overflows throws std::bad_alloc (a fatal "out of memory" under
// run_main, as in Go).
template <typename Cell>
map_store* store_new(std::size_t cap) {
    std::size_t bytes;
    if (!store_bytes<Cell>(cap, bytes)) {
        throw std::bad_alloc();
    }
    void* p = ::operator new(bytes, std::align_val_t{store_align<Cell>});
    auto* s = ::new (p) map_store;
    s->gp_cap = cap;
    std::memset(store_ctrl(s), static_cast<unsigned char>(sw::ctrl_empty), cap + group_width);
    return s;
}

// store_free: the memory of s, whose cells are already destroyed.
template <typename Cell>
void store_free(map_store* s) noexcept {
    s->~map_store();
    ::operator delete(static_cast<void*>(s), std::align_val_t{store_align<Cell>});
}

// store_drop<Cell>(s): one reference less; the last one destroys the cells that the control
// bytes say are full, and frees the block.
template <typename Cell>
void store_drop(map_store* s) noexcept {
    if (s->gp_refs.fetch_sub(1, std::memory_order_acq_rel) != 1) {
        return;
    }
    if constexpr (!std::is_trivially_destructible_v<Cell>) {
        const int8_t* ctrl = store_ctrl(s);
        Cell* cells = store_cells<Cell>(s);
        for (std::size_t i = 0; i < s->gp_cap; i++) {
            if (ctrl[i] >= 0) {
                cells[i].~Cell();
            }
        }
    }
    detail::store_free<Cell>(s);
}

// capacity_growth(cap): how many slots a table of cap slots may use (full and tombstones): 7/8.
constexpr std::size_t capacity_growth(std::size_t cap) noexcept {
    return cap - cap / 8;
}

// capacity_for(n): the smallest capacity that holds n entries without growing.
inline std::size_t capacity_for(std::size_t n) noexcept {
    std::size_t cap = min_capacity;
    while (detail::capacity_growth(cap) < n) {
        cap *= 2;
    }
    return cap;
}

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

// key_reflexive<K>: every key of type K equals itself (no NaN can be in it).
template <typename K, typename B = typename key_base<K>::type>
inline constexpr bool key_reflexive =
    std::is_integral_v<B> || std::is_enum_v<B> || is_pointer_key<B> || std::is_same_v<B, string>;

// self_equal: k == k, false for a NaN (or a struct or array that holds one). A function, so
// that compilers don't warn about a comparison of a value with itself.
template <typename K>
bool self_equal(const K& a, const K& b) {
    return a == b;
}

// key_eq: a == b for the stored key a and the key b of an operation. Strings that share their
// bytes (a key that was stored and looked up again) are equal without a comparison, as Go's
// memequal does.
template <typename K>
bool key_eq(const K& a, const K& b) {
    if constexpr (std::is_same_v<K, string>) {
        return (a.gp_data() == b.gp_data() && gp::len(a) == gp::len(b)) || a == b;
    } else {
        return a == b;
    }
}

// map_hash: the hash of key k in a table with this seed. Integers and pointers take one
// multiplication, strings go through wyhash, everything else through gp::hash_of (hash.hpp: the
// key's own hash, which agrees with ==) and a multiplication for the seed.
template <typename K>
uint64_t map_hash(const K& k, uint64_t seed) {
    if constexpr (std::is_integral_v<K> && sizeof(K) <= 8) {
        return sw::hash_u64(static_cast<uint64_t>(k), seed);
    } else if constexpr (std::is_enum_v<K>) {
        return sw::hash_u64(static_cast<uint64_t>(static_cast<std::underlying_type_t<K>>(k)), seed);
    } else if constexpr (std::is_same_v<K, string>) {
        return sw::hash_mem(k.gp_data(), static_cast<std::size_t>(gp::len(k)), seed);
    } else if constexpr (requires(const K& p) { p.gp_get(); } && is_pointer_key<K>) {
        return sw::hash_u64(static_cast<uint64_t>(reinterpret_cast<std::uintptr_t>(k.gp_get())),
                            seed);
    } else {
        return sw::hash_u64(static_cast<uint64_t>(gp::hash_of(k)), seed);
    }
}

// The flag of a write in progress (Go's hashWriting): a plain load and store, no atomic
// read-modify-write.
struct write_guard {
    std::atomic<uint8_t>& gp_f;
    explicit write_guard(std::atomic<uint8_t>& f) : gp_f(f) {
        if (f.load(std::memory_order_relaxed) != 0) [[unlikely]] {
            detail::panic_concurrent("concurrent map writes");
        }
        f.store(1, std::memory_order_relaxed);
    }
    ~write_guard() { gp_f.store(0, std::memory_order_relaxed); }
    write_guard(const write_guard&) = delete;
    write_guard& operator=(const write_guard&) = delete;
};

// map_impl: the table.
template <typename K, typename V>
struct map_impl {
    using cell = map_cell<K, V>;

    int8_t* gp_ctrl = sw::empty_ctrl.gp_b; // the control bytes; a static group when cap is 0
    cell* gp_cells = nullptr;
    std::size_t gp_mask = 0;   // cap - 1, or 0
    std::size_t gp_cap = 0;    // the number of slots, 0 or a power of two
    std::size_t gp_size = 0;   // the number of entries
    std::size_t gp_growth = 0; // how many empty slots a store may still take before the table grows
    uint64_t gp_seed = sw::swiss_random();
    map_store* gp_store = nullptr; // the block of ctrl and cells; null when cap is 0
    uint64_t gp_nans = 0;          // the keys that aren't equal to themselves that got a hash
    uint64_t gp_clears = 0;        // the clears that removed something
    uint64_t gp_resizes = 0;       // the blocks made by growing or rehashing (for the tests)
    std::atomic<uint8_t> gp_writing{0};

    map_impl() = default;
    map_impl(const map_impl&) = delete;
    map_impl& operator=(const map_impl&) = delete;
    ~map_impl() {
        if (gp_store != nullptr) {
            detail::store_drop<cell>(gp_store);
        }
    }

    // A range holds the block (a second reference).
    bool gp_iterating() const noexcept {
        return gp_store != nullptr && gp_store->gp_refs.load(std::memory_order_relaxed) > 1;
    }

    // gp_set_ctrl: the control byte of slot i, and its copy after the end for the slots in the
    // first group.
    void gp_set_ctrl(std::size_t i, int8_t c) noexcept {
        gp_ctrl[i] = c;
        gp_ctrl[((i - group_width) & gp_mask) + group_width] = c;
    }

    // gp_find: the cell of key k (with hash h), or null.
    cell* gp_find(const K& k, uint64_t h) const {
        uint8_t h2 = static_cast<uint8_t>(h & 0x7f);
        std::size_t pos = static_cast<std::size_t>(h >> 7) & gp_mask;
        std::size_t step = 0;
        for (;;) {
            sw::group g(gp_ctrl + pos);
            for (int i : g.match(h2)) {
                cell* c = gp_cells + ((pos + static_cast<std::size_t>(i)) & gp_mask);
                if (detail::key_eq(c->gp_k, k)) [[likely]] {
                    return c;
                }
            }
            if (g.match_empty()) {
                return nullptr;
            }
            step += group_width;
            pos = (pos + step) & gp_mask;
        }
    }

    // gp_first_free: the first slot without an entry on the probe sequence of h (an empty slot
    // or a tombstone); only_empty skips the tombstones.
    std::size_t gp_first_free(uint64_t h, bool only_empty) const noexcept {
        std::size_t pos = static_cast<std::size_t>(h >> 7) & gp_mask;
        std::size_t step = 0;
        for (;;) {
            sw::group g(gp_ctrl + pos);
            auto m = only_empty ? g.match_empty() : g.match_empty_or_deleted();
            if (m) {
                return (pos + static_cast<std::size_t>(m.lowest())) & gp_mask;
            }
            step += group_width;
            pos = (pos + step) & gp_mask;
        }
    }

    // gp_slot: the value of key k, inserted as the zero value if k isn't there (Go's
    // mapassign). An equal key that is there is replaced with k (key_update).
    V& gp_slot(const K& k) {
        write_guard guard(gp_writing);
        uint64_t h = detail::map_hash(k, gp_seed);
        if constexpr (!key_reflexive<K>) {
            if (!detail::self_equal(k, k)) {
                // A NaN (or a struct or array holding one) is never equal to a key: it is
                // always new, and gets a hash of its own.
                h = gp_nan_hash(h);
                return gp_insert(k, h, gp_first_free(h, false))->gp_v;
            }
        }
        uint8_t h2 = static_cast<uint8_t>(h & 0x7f);
        std::size_t pos = static_cast<std::size_t>(h >> 7) & gp_mask;
        std::size_t step = 0;
        std::size_t free = static_cast<std::size_t>(-1);
        for (;;) {
            sw::group g(gp_ctrl + pos);
            for (int i : g.match(h2)) {
                cell* c = gp_cells + ((pos + static_cast<std::size_t>(i)) & gp_mask);
                if (detail::key_eq(c->gp_k, k)) [[likely]] {
                    if constexpr (key_update<K>) {
                        c->gp_k = k;
                    }
                    return c->gp_v;
                }
            }
            if (free == static_cast<std::size_t>(-1)) {
                auto m = g.match_empty_or_deleted();
                if (m) {
                    free = (pos + static_cast<std::size_t>(m.lowest())) & gp_mask;
                }
            }
            if (g.match_empty()) {
                break;
            }
            step += group_width;
            pos = (pos + step) & gp_mask;
        }
        return gp_insert(k, h, free)->gp_v;
    }

    // The hash of a NaN key: the key's own hash mixed with a count (every NaN has one hash).
    uint64_t gp_nan_hash(uint64_t h) noexcept {
        return detail::hash_mix(h ^ (++gp_nans * 0x9e3779b97f4a7c15u));
    }

    // gp_insert: a new entry for key k (with hash h) in slot `free`, the first slot without an
    // entry on h's probe sequence (found by the probe before). Grows the table first if there is
    // no room. Returns the cell, whose value is the zero value.
    cell* gp_insert(const K& k, uint64_t h, std::size_t free) {
        if (gp_ctrl[free] == sw::ctrl_deleted && gp_iterating()) {
            // A range holds the block: a tombstone stays a tombstone (above).
            free = gp_first_free(h, true);
        }
        if (gp_ctrl[free] == sw::ctrl_empty && gp_growth == 0) {
            gp_grow();
            free = gp_first_free(h, false);
        }
        cell* c = gp_cells + free;
        ::new (static_cast<void*>(&c->gp_k)) K(k);
        try {
            ::new (static_cast<void*>(&c->gp_v)) V();
        } catch (...) {
            c->gp_k.~K();
            throw;
        }
        if (gp_ctrl[free] == sw::ctrl_empty) {
            gp_growth--;
        }
        gp_set_ctrl(free, static_cast<int8_t>(h & 0x7f));
        gp_size++;
        return c;
    }

    // gp_grow: room for more entries: a table of twice the slots, or of the same number if
    // tombstones take most of them. The live entries must be at most 9/16 of the slots for that:
    // a rehash moves every live entry and buys cap * 7/8 - size stores, so at 9/16 it moves about
    // two entries for each store (at Abseil's 25/32 it would be seven: a map that holds many
    // entries and is always replacing some would spend its time rehashing). Above 9/16 the table
    // doubles once, and the later rehashes are cheap.
    void gp_grow() {
        std::size_t cap = gp_cap;
        if (cap == 0) {
            gp_resize(min_capacity);
        } else if (gp_size * 16 <= cap * 9) {
            gp_resize(cap);
        } else {
            gp_resize(cap * 2);
        }
    }

    // gp_resize: moves the entries into a new block of cap slots. If a range holds the old
    // block, the entries are copied, and the range keeps the old block (above).
    void gp_resize(std::size_t cap) {
        gp_resizes++;
        map_store* ns = detail::store_new<cell>(cap);
        int8_t* nctrl = store_ctrl(ns);
        cell* ncells = store_cells<cell>(ns);
        std::size_t nmask = cap - 1;
        bool retained =
            gp_store != nullptr && gp_store->gp_refs.load(std::memory_order_relaxed) > 1;
        // Copy the entries (the old block must stay whole) when a range holds it, or when a move
        // may throw. A value that can't be copied (a C++ unique_ptr) is moved always: the range
        // looks the key up in the live table and reads the value there (only a NaN key reads its
        // value from the old block).
        // Keys and values are copied together or moved together: if the moves can throw, a later
        // entry's copy failing must find the old block whole (no moved-from key left in it).
        constexpr bool can_move =
            std::is_nothrow_move_constructible_v<K> && std::is_nothrow_move_constructible_v<V>;
        constexpr bool copy_v_ok = std::is_copy_constructible_v<V>;
        bool copy_k = retained || !can_move;
        bool copy_v = (retained || !can_move) && copy_v_ok;
        std::size_t placed = 0;
        try {
            for (std::size_t i = 0; i < gp_cap; i++) {
                if (gp_ctrl[i] < 0) {
                    continue;
                }
                cell* from = gp_cells + i;
                uint64_t h = detail::map_hash(from->gp_k, gp_seed);
                if constexpr (!key_reflexive<K>) {
                    if (!detail::self_equal(from->gp_k, from->gp_k)) {
                        h = gp_nan_hash(h);
                    }
                }
                std::size_t j = gp_free_in(nctrl, nmask, h);
                cell* to = ncells + j;
                if (copy_k) {
                    ::new (static_cast<void*>(&to->gp_k)) K(from->gp_k);
                } else {
                    ::new (static_cast<void*>(&to->gp_k)) K(std::move(from->gp_k));
                }
                try {
                    if constexpr (copy_v_ok) {
                        if (copy_v) {
                            ::new (static_cast<void*>(&to->gp_v)) V(from->gp_v);
                        } else {
                            ::new (static_cast<void*>(&to->gp_v)) V(std::move(from->gp_v));
                        }
                    } else {
                        ::new (static_cast<void*>(&to->gp_v)) V(std::move(from->gp_v));
                    }
                } catch (...) {
                    to->gp_k.~K();
                    throw;
                }
                nctrl[j] = static_cast<int8_t>(h & 0x7f);
                nctrl[((j - group_width) & nmask) + group_width] = nctrl[j];
                placed++;
            }
        } catch (...) {
            // Copying failed: the old block is whole (the copy mode never touches it); drop the
            // cells made so far.
            for (std::size_t j = 0; j < cap; j++) {
                if (nctrl[j] >= 0) {
                    ncells[j].~cell();
                }
            }
            detail::store_free<cell>(ns);
            throw;
        }
        map_store* old = gp_store;
        gp_store = ns;
        gp_ctrl = nctrl;
        gp_cells = ncells;
        gp_cap = cap;
        gp_mask = nmask;
        gp_growth = detail::capacity_growth(cap) - placed;
        if (old != nullptr) {
            detail::store_drop<cell>(old);
        }
    }

    // gp_free_in: the first empty slot on h's probe sequence in a block that has no entries
    // deleted (a new block).
    static std::size_t gp_free_in(const int8_t* ctrl, std::size_t mask, uint64_t h) noexcept {
        std::size_t pos = static_cast<std::size_t>(h >> 7) & mask;
        std::size_t step = 0;
        for (;;) {
            sw::group g(ctrl + pos);
            auto m = g.match_empty();
            if (m) {
                return (pos + static_cast<std::size_t>(m.lowest())) & mask;
            }
            step += group_width;
            pos = (pos + step) & mask;
        }
    }

    // gp_reserve: room for n entries, before the first store.
    // As Go's makemap, a hint whose table would overflow or exceed the allocator's limit (cells
    // of 2^44 bytes, say) is ignored.
    void gp_reserve(std::size_t n) {
        std::size_t cap = detail::capacity_for(n);
        std::size_t bytes;
        if (!detail::store_bytes<cell>(cap, bytes)) {
            return;
        }
        if (cap > gp_cap) {
            gp_resize(cap);
        }
    }

    // gp_erase: deletes the entry of cell c.
    void gp_erase(cell* c) noexcept {
        std::size_t i = static_cast<std::size_t>(c - gp_cells);
        c->~cell();
        gp_size--;
        // A slot that no probe has passed over can become empty again; one that a probe may
        // have passed over (its group and the group before it have no empty slot between) must
        // become a tombstone: the probe for a key stored after it would stop there otherwise. A
        // range that holds the block needs the tombstone in any case (above).
        std::size_t before = (i - group_width) & gp_mask;
        auto empty_after = sw::group(gp_ctrl + i).match_empty();
        auto empty_before = sw::group(gp_ctrl + before).match_empty();
        bool never_full = empty_before && empty_after &&
                          static_cast<std::size_t>(empty_after.trailing_zeros() +
                                                   empty_before.leading_zeros()) < group_width;
        if (never_full && !gp_iterating()) {
            gp_set_ctrl(i, sw::ctrl_empty);
            gp_growth++;
        } else {
            gp_set_ctrl(i, sw::ctrl_deleted);
        }
    }

    // gp_clear: deletes every entry. While a range holds the block the slots become tombstones
    // (above), else the block is empty again.
    void gp_clear() noexcept {
        if (gp_size == 0) {
            return;
        }
        if (gp_iterating()) {
            for (std::size_t i = 0; i < gp_cap; i++) {
                if (gp_ctrl[i] >= 0) {
                    gp_cells[i].~cell();
                    gp_ctrl[i] = sw::ctrl_deleted;
                }
            }
            std::memcpy(gp_ctrl + gp_cap, gp_ctrl, group_width);
        } else {
            if constexpr (!std::is_trivially_destructible_v<cell>) {
                for (std::size_t i = 0; i < gp_cap; i++) {
                    if (gp_ctrl[i] >= 0) {
                        gp_cells[i].~cell();
                    }
                }
            }
            std::memset(gp_ctrl, static_cast<unsigned char>(sw::ctrl_empty), gp_cap + group_width);
            gp_growth = detail::capacity_growth(gp_cap);
        }
        gp_size = 0;
        gp_nans = 0;
        gp_clears++;
    }

    // gp_each: calls f(key, value) for every entry (for the runtime's own code: fmt).
    template <typename F>
    void gp_each(F&& f) const {
        for (std::size_t i = 0; i < gp_cap; i++) {
            if (gp_ctrl[i] >= 0) {
                f(gp_cells[i].gp_k, gp_cells[i].gp_v);
            }
        }
    }
};

// check_empty_key: Go checks the key of a read or a delete in a nil or empty map without hashing
// it, and panics with its own text ("hash of unhashable type: []int", not a runtime error) when
// the key is, or holds (in a struct field, an array element or a boxed interface value), an
// interface value whose dynamic type has no hash (gp::check_key, gocpp/hash.hpp).
template <typename K, typename V>
void check_empty_key([[maybe_unused]] const map_impl<K, V>* impl, [[maybe_unused]] const K& k) {
    if constexpr (hash_may_panic<K>) {
        if (impl == nullptr || impl->gp_size == 0) {
            gp::check_key(k);
        }
    }
}

// map_find: the cell of key k, or null.
template <typename K, typename V>
map_cell<K, V>* map_find(const map_impl<K, V>& m, const K& k) {
    if (m.gp_writing.load(std::memory_order_relaxed) != 0) [[unlikely]] {
        detail::panic_concurrent("concurrent map read and map write");
    }
    return m.gp_find(k, detail::map_hash(k, m.gp_seed));
}

// map_slot: the value of key k, inserted as the zero value if k isn't there (Go's mapassign).
template <typename K, typename V>
V& map_slot(map_impl<K, V>& m, const K& k) {
    return m.gp_slot(k);
}

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
// about n entries, but for at most detail::max_map_reserve (2^22) of them: Go allocates a huge
// map's table only as it fills, so make(map[string]int, 1<<40) runs in Go and must not fail
// here. A larger table grows as it fills. A negative n counts as 0, as in Go, and so does an n
// whose room can't be allocated: a hint never panics.
template <typename K, typename V>
map<K, V> make_map() {
    return map<K, V>(std::make_shared<detail::map_impl<K, V>>());
}

namespace detail {
inline constexpr uint64_t max_map_reserve = uint64_t(1) << 22;
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
            m.gp_get()->gp_reserve(static_cast<std::size_t>(n));
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
    return impl == nullptr ? 0 : static_cast<int64_t>(impl->gp_size);
}

// get(m, k): m[k], the value of k, or the zero value of V if k isn't there. Never inserts.
template <typename K, typename V>
V get(const map<K, V>& m, const std::type_identity_t<K>& k) {
    detail::map_impl<K, V>* impl = m.gp_get();
    detail::check_empty_key(impl, k);
    if (impl != nullptr) {
        if (auto* c = detail::map_find(*impl, k); c != nullptr) {
            return c->gp_v;
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
        if (auto* c = detail::map_find(*impl, k); c != nullptr) {
            return std::tuple<V, bool>(c->gp_v, true);
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
// stays valid until a new key is stored in the map (the table can grow), or k is deleted or
// the map is cleared.
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
    detail::write_guard guard(impl->gp_writing);
    if (auto* c = impl->gp_find(k, detail::map_hash(k, impl->gp_seed)); c != nullptr) {
        impl->gp_erase(c);
    }
}

// clear(m): deletes every entry, also the NaN keys (Go 1.21). Nothing happens for nil.
template <typename K, typename V>
void clear(const map<K, V>& m) {
    detail::map_impl<K, V>* impl = m.gp_get();
    if (impl == nullptr) {
        return;
    }
    detail::write_guard guard(impl->gp_writing);
    impl->gp_clear();
}

// map_range is the state of `for k, v := range m` (above). After each successful
// range_next, gp_key() and gp_value() are the entry's key and its current value; they stay
// valid until the next range_next or a store, delete or clear of that map. Copy them (or read
// the value later with range_value). It holds a reference to the table's block.
template <typename K, typename V>
struct map_range {
    map<K, V> gp_m;                        // the map, evaluated once
    detail::map_store* gp_store = nullptr; // the block that the range goes over
    int8_t* gp_ctrl = nullptr;             // its control bytes and cells
    detail::map_cell<K, V>* gp_cells = nullptr;
    std::size_t gp_mask = 0;                  // its slots - 1
    std::size_t gp_start = 0;                 // where the round started
    std::size_t gp_pos = 0;                   // the slots of the round that are done
    std::size_t gp_idx = 0;                   // the slot of the current entry
    detail::map_cell<K, V>* gp_cur = nullptr; // the current entry
    uint64_t gp_clears = 0;                   // the map's clears when the range started

    map_range() = default;
    // A copy goes on from where the original is, and holds the block too.
    map_range(const map_range& o)
        : gp_m(o.gp_m), gp_store(o.gp_store), gp_ctrl(o.gp_ctrl), gp_cells(o.gp_cells),
          gp_mask(o.gp_mask), gp_start(o.gp_start), gp_pos(o.gp_pos), gp_idx(o.gp_idx),
          gp_cur(o.gp_cur), gp_clears(o.gp_clears) {
        if (gp_store != nullptr) {
            gp_store->gp_refs.fetch_add(1, std::memory_order_relaxed);
        }
    }
    map_range& operator=(const map_range&) = delete;
    map_range(map_range&& o) noexcept
        : gp_m(std::move(o.gp_m)), gp_store(std::exchange(o.gp_store, nullptr)), gp_ctrl(o.gp_ctrl),
          gp_cells(o.gp_cells), gp_mask(o.gp_mask), gp_start(o.gp_start), gp_pos(o.gp_pos),
          gp_idx(o.gp_idx), gp_cur(o.gp_cur), gp_clears(o.gp_clears) {}
    map_range& operator=(map_range&&) = delete;
    ~map_range() {
        if (gp_store != nullptr) {
            detail::store_drop<detail::map_cell<K, V>>(gp_store);
        }
    }

    const K& gp_key() const noexcept { return gp_cur->gp_k; }
    const V& gp_value() const noexcept { return gp_cur->gp_v; }
};

// range_map(m) starts a range over m.
template <typename K, typename V>
map_range<K, V> range_map(const map<K, V>& m) {
    map_range<K, V> r;
    r.gp_m = m;
    detail::map_impl<K, V>* impl = m.gp_get();
    if (impl != nullptr && impl->gp_store != nullptr) {
        r.gp_store = impl->gp_store;
        r.gp_store->gp_refs.fetch_add(1, std::memory_order_relaxed);
        r.gp_ctrl = impl->gp_ctrl;
        r.gp_cells = impl->gp_cells;
        r.gp_mask = impl->gp_mask;
        r.gp_start = static_cast<std::size_t>(detail::sw::swiss_random()) & impl->gp_mask;
        r.gp_clears = impl->gp_clears;
    }
    return r;
}

// range_next steps to the next entry of the range that is still in the map, and reports
// whether there is one.
template <typename K, typename V>
bool range_next(map_range<K, V>& it) {
    if (it.gp_store == nullptr) {
        return false; // nil, or empty: nothing was there when the range started
    }
    detail::map_impl<K, V>* impl = it.gp_m.gp_get();
    if (impl->gp_writing.load(std::memory_order_relaxed) != 0) [[unlikely]] {
        detail::panic_concurrent("concurrent map iteration and map write");
    }
    const std::size_t cap = it.gp_mask + 1;
    const bool live = impl->gp_store == it.gp_store;
    while (it.gp_pos < cap) {
        std::size_t i = (it.gp_start + it.gp_pos) & it.gp_mask;
        it.gp_pos++;
        if (it.gp_ctrl[i] < 0) {
            continue;
        }
        auto* c = it.gp_cells + i;
        if (live) {
            it.gp_idx = i;
            it.gp_cur = c;
            return true;
        }
        // The table grew since the range started: c is the entry as the range found it.
        if constexpr (!detail::key_reflexive<K>) {
            if (!detail::self_equal(c->gp_k, c->gp_k)) {
                if (impl->gp_clears == it.gp_clears) {
                    it.gp_idx = i;
                    it.gp_cur = c; // a NaN key: only a clear removes it
                    return true;
                }
                continue;
            }
        }
        if (auto* f = impl->gp_find(c->gp_k, detail::map_hash(c->gp_k, impl->gp_seed));
            f != nullptr) {
            it.gp_idx = i;
            it.gp_cur = f;
            return true;
        }
    }
    return false;
}

// range_value(it): the value of the range's current entry as it is now, when code ran since
// range_next: the emitter reads it after the targets' operands of for k, v = range m (step
// 3.7b), whose calls may store into the entry, delete it, clear the map or grow it (gp_value()
// would dangle or be stale then). It gives the entry's current value, as Go does, or the zero
// value if the entry is gone (Go reads its cleared slot).
// (For a non-copyable V and a NaN key after the table grew during the range, the value is the
// moved-from one: see the note on lifetimes at the top.)
template <typename K, typename V>
V range_value(const map_range<K, V>& it) {
    detail::map_impl<K, V>* impl = it.gp_m.gp_get();
    if (impl->gp_store == it.gp_store) {
        // The block is the same: a deleted entry's slot is a tombstone for as long as the range
        // holds the block.
        return it.gp_ctrl[it.gp_idx] < 0 ? V() : it.gp_cells[it.gp_idx].gp_v;
    }
    const auto& c = it.gp_cells[it.gp_idx];
    if constexpr (!detail::key_reflexive<K>) {
        if (!detail::self_equal(c.gp_k, c.gp_k)) {
            return impl->gp_clears == it.gp_clears ? c.gp_v : V();
        }
    }
    auto* f = impl->gp_find(c.gp_k, detail::map_hash(c.gp_k, impl->gp_seed));
    return f == nullptr ? V() : f->gp_v;
}

} // namespace gp
