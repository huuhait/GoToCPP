// A header for map_cxx_key_class.gp, map_cxx_key_print_error.gp and cxx_struct_eq*.gp: C++ classes as keys
// of gocpp maps and in compared structs.
#pragma once

#include <cstddef>
#include <functional>
#include <ostream>

namespace mk {
// Ordered: ==, <, std::hash and operator<<.
struct Pt {
    int x;
    int y;
    bool operator==(const Pt&) const = default;
    bool operator<(const Pt& o) const { return x != o.x ? x < o.x : y < o.y; }
};
inline std::ostream& operator<<(std::ostream& os, const Pt& p) { return os << "(" << p.x << "," << p.y << ")"; }

// Hashable and printable, without operator<.
struct Tag {
    int id;
    bool operator==(const Tag&) const = default;
};
inline std::ostream& operator<<(std::ostream& os, const Tag& t) { return os << "#" << t.id; }
// No ==.
struct NoEq {
    int v;
};
} // namespace mk

template <>
struct std::hash<mk::Pt> {
    std::size_t operator()(const mk::Pt& p) const noexcept { return std::size_t(p.x) * 31 + std::size_t(p.y); }
};
template <>
struct std::hash<mk::Tag> {
    std::size_t operator()(const mk::Tag& t) const noexcept { return std::size_t(t.id); }
};
