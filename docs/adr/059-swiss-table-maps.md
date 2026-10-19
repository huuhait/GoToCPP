# ADR-059: Maps are Swiss tables with a seed for each map and a counted block for ranges

Status: accepted (2026-10-18). Replaces the `std::unordered_map` table of step 3.7 (Phase 03 §2.3) and the
phase-00 decision D8 ("map order is not randomized").

## Context

`gp::map<K, V>` (Phase 03) wrapped a `std::unordered_map` that stored each key with its hash: one heap
node for each entry (an allocation, a pointer to chase on every lookup, about 56 bytes for an
`int`-to-`int` entry), a fixed hash seed (an input could be chosen to collide; D8), and a range that
copied every key into a snapshot first (one allocation and a copy of the keys for each `range`, also of a
map with three entries). Go 1.24 moved to Swiss tables for the same reasons, and Phase 15 §15.5 asks for
the same here, with Go's semantics intact: a range sees each entry that stays in the map exactly once,
never one that was deleted before it was reached, and never one twice, also when the loop inserts and the
table grows; NaN keys; `clear`; `map[any]V` with its panics.

## Decision

`gp::map<K, V>` is still one `std::shared_ptr` (16 bytes) to a table, and its API (the free functions
`get`, `lookup`, `set`, `ref`, `erase`, `clear`, `len`, `range_map`, `range_next`, `range_value`, `make_map`,
`map_of`) does not change, so the lowering and the emitter do not change. Inside:

1. **The table** is one allocation: a header, one control byte for each slot, and the slots (key and
   value side by side). The capacity is a power of two, at least one group (16 slots with SSE2, else 8).
   A control byte is the low 7 bits of the key's hash (H2) for a full slot, `0x80` for an empty one and
   `0xFE` for a deleted one (a tombstone). The first group's bytes are repeated after the last, so a
   group can be loaded at any slot without wrapping. A table that was never written to has no memory (a
   static group of empty bytes serves the probe), so `make(map[K]V)` costs the `shared_ptr` block only.
2. **A probe** starts at the slot that the rest of the hash (H1) names and compares a whole group of
   control bytes with H2 at once (`gocpp/swiss.hpp`: SSE2 on x86-64, through the compilers' vector types so that no intrinsics header is included, and a portable 64-bit
   version, which the tests run through the whole map too, `GOCPP_SWISS_PORTABLE`); only the slots whose
   byte matches are compared with the key. A group with an empty slot ends the probe; else the next group
   is at a triangular step. The table holds at most 7/8 of its slots (entries and tombstones together): a
   store that needs an empty slot of a full table doubles it, or rehashes it into one of the same size when
   tombstones took most of the room (live entries at most 9/16 of the slots: a rehash moves every live
   entry and buys `7/8 * cap - size` stores, so at 9/16 it moves about two entries per store; Abseil's
   25/32 would move seven, and a map that holds many entries and keeps replacing some measured 3x
   slower than Go's), so churn (delete and store in a loop) grows the table at most once. A delete makes the slot empty again if no probe can have passed over it (the
   two groups around it have an empty slot within a group's width), else a tombstone, as Abseil does.
3. **Hashing.** Every map has a random seed (a per-thread stream seeded from the operating system).
   Integer and pointer keys take one 64x64 multiplication (`swiss::hash_u64`), strings wyhash
   (`swiss::hash_mem`), every other key `gp::hash_of(k)` (floats, structs, arrays, interfaces: the
   protocol of `hash.hpp` is unchanged) and a multiplication for the seed. A key that is not equal to
   itself (a NaN, or a struct or array holding one) is never found, so it is placed with a hash of its own
   (the map's count of such keys mixed in), also when the table grows: thousands of NaN keys do not share
   one probe sequence. The panic of an unhashable dynamic type (`map[any]V`) is thrown by the hash before
   the table is touched, and a hash, `==`, key copy or value copy that throws leaves the table whole.
4. **Range** starts at a random slot and goes round the table once (D8 is now Go's: random). The loop
   holds one reference to the table's block (an atomic count in the header: any number of goroutines can
   range one map). That is all it takes to keep the spec's rules:
   - a delete turns the slot into a tombstone, never an empty slot, and a store does not reuse a
     tombstone while a loop holds the block: an entry deleted before the loop reaches it is not produced,
     and `range_value` (the value read after the targets' operands of `for s[f()], v = range m`) can tell
     that the current entry was deleted (its slot is still a tombstone) and gives the zero value, as Go
     reads its cleared slot;
   - when a store grows or rehashes the table while a loop holds the block, the entries are *copied*
     into the new block and the old block lives on with the loop, which finishes its round over the old
     block as Go's runtime does with the old buckets: for each slot that was full, it looks the key up in
     the live table and produces the entry from there with its current key and value, or skips the key
     if it was deleted meanwhile; a NaN key cannot be looked up or deleted, so it is produced from the old
     block unless a `clear` came since. When no loop holds the block (nearly always) the entries are
     moved, as for any table;
   - `clear` empties the block (tombstones while a loop holds it);
   - an entry added during the loop may or may not be produced (it is, if it lands in a slot ahead of the
     loop and the table does not grow), and a key deleted and stored again is a new entry; no entry is
     produced twice.
5. **Concurrent writes.** As Go, a write sets a flag that it clears when it ends, with plain loads and
   stores (no atomic read-modify-write); a write, read or range that finds the flag set dies with
   `fatal error: concurrent map writes` (or `concurrent map read and map write`, `concurrent map
   iteration and map write`). It catches some races only, as Go's does.
6. **Layout.** A `gp::map` is still 16 bytes. The type descriptors' map accessors (`gp_map_snap` for
   `fmt`) read the entries through `map_impl::gp_each`, so `fmt` prints maps sorted as before.

## Consequences

- The order of a range is random (D8 changes from "not randomized" to "random, as Go"): a program that
  depends on it was already wrong in Go; the test corpus sorts or sums.
- A reference from `gp::ref(m, k)` and the entry that `gp_key()`/`gp_value()` refer to are valid until
  the next store of a new key (the table can grow); the old table kept the entries in place. Nothing the
  lowering emits relied on that (the right side of `m[k] op= y` is evaluated first, a range copies the key
  and reads the value at the top of the body or through `range_value`).
- Growing moves entries: a key or value type must not throw when moved (every gocpp type is such); one
  whose move may throw is copied, and the copies are rolled back if one throws.
- `make(map[K]V, n)` reserves room for `n` entries at most 2^22 (2^20 before; only the control bytes are written, the slots stay untouched memory until used; a hint
  such as Go's accepted `1<<40` must not allocate everything); a bigger table grows as it fills. As in Go (makemap), a hint whose table would overflow or pass the allocator's limit (2^48 bytes) is ignored; a growth that does is a fatal "out of memory".
- The measured gains and the cases where Go is still ahead are in the Phase 15 README (15.5 notes).

## Alternatives considered

- **Go's own design** (a directory of tables of 1024 slots, groups of 8, incremental growth): it bounds the
  latency of one insert and shares the old table's groups with iterators through a "grow in place" scheme.
  It is a lot of machinery for a runtime whose iterators are cheap to keep correct with a counted block;
  the one big table grows with a pause proportional to its size, like Abseil's and the previous
  `unordered_map`'s.
- **A snapshot of the keys for each range** (the old design): one allocation and a copy of every key for
  each loop; replaced by the counted block (an atomic increment at the start).
- **Keeping the hash of each key in its slot**: 8 bytes more for each entry to save rehashing on growth;
  rehashing is a multiplication for integers and one wyhash for strings, so it is not worth the memory.
- **In-place rehash for tombstone cleanup** (Abseil): saves an allocation when no loop holds the block;
  a new block is simpler (the same code as growth, and a loop that holds the old one just keeps it) and
  its cost is amortized over a table's 1/8 of free slots.
