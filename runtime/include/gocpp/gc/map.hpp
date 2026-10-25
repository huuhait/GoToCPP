// gocpp/gc/map.hpp: gp::gc::map<K, V>, Go's map value in the GC engine (phase-16 README §2.2;
// job 16.1e).
//
// A Go map value is one pointer to the map's table in the GC heap, 8 bytes, nil for the zero
// value. This job's table is the placeholder map_core below: the count, the key and element
// sizes and whether they hold pointers, and nothing else. The spike's direct packages
// (container/list, strconv, sort, binarytrees) use no map, but their import closure does:
// Go 1.25's strings imports unicode, and unicode/tables.go declares six package-level map
// literals (Categories, CategoryAliases, Scripts, Properties, FoldCategory, FoldScript). A
// 16.1f build of strings therefore needs map creation and assignment, or a documented stub or
// skip of unicode's variables, which the plan does not name. The manager decided that this job
// keeps the placeholder table; 16.1f must implement the simple open-addressing map (the ticket
// allows it) or document the stub before strings can build. Until then, make_map gives a
// non-nil, empty map and len(m) is 0; indexing, assignment, delete, range and clear are not
// implemented (a compile error, not silent wrong behavior). Step 16.8 and step 16.11 bring
// Go's own Swiss tables (internal/runtime/maps).
//
// A map compares only with nil (a map in an interface is not comparable and panics).
#pragma once

#include <gocpp/gc/golayout.hpp>
#include <gocpp/gc/heap.hpp>

#include <cstddef>
#include <cstdint>

namespace gp::gc {

// map_core is the placeholder table of a map value. Step 16.8/16.11 replaces it with the real
// table; nothing outside gp::gc::map reads it.
struct map_core {
    std::uint64_t gp_count = 0;     // the number of entries
    std::uint64_t gp_key_size = 0;  // Go's size of K
    std::uint64_t gp_elem_size = 0; // Go's size of V
    bool gp_key_ptrs = false;       // K holds a pointer
    bool gp_elem_ptrs = false;      // V holds a pointer
};

template <typename K, typename V>
class map {
public:
    // nil.
    constexpr map() noexcept = default;
    constexpr map(std::nullptr_t) noexcept {}
    explicit constexpr map(map_core* table) noexcept : gp_table_(table) {}

    bool gp_nil() const noexcept { return gp_table_ == nullptr; }
    map_core* gp_table() const noexcept { return gp_table_; }

    // m == nil. Go compares maps only with nil.
    friend bool operator==(const map& m, std::nullptr_t) noexcept { return m.gp_table_ == nullptr; }
    friend bool operator!=(const map& m, std::nullptr_t) noexcept { return m.gp_table_ != nullptr; }

private:
    map_core* gp_table_ = nullptr;
};

static_assert(sizeof(map<std::int64_t, std::int64_t>) == 8, "gp::gc::map must be Go's 8 bytes");

// make_map<K, V>(): make(map[K]V). The table is in the GC heap; the map is non-nil and empty.
template <typename K, typename V>
map<K, V> make_map() {
    map_core* c = new_<map_core>();
    c->gp_key_size = go_layout<K>().size;
    c->gp_elem_size = go_layout<V>().size;
    c->gp_key_ptrs = go_layout<K>().ptrs;
    c->gp_elem_ptrs = go_layout<V>().ptrs;
    return map<K, V>(c);
}

// len(m): the number of entries; 0 for a nil map, as in Go.
template <typename K, typename V>
std::int64_t len(const map<K, V>& m) noexcept {
    return m.gp_nil() ? 0 : static_cast<std::int64_t>(m.gp_table()->gp_count);
}

} // namespace gp::gc
