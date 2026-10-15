// A helper header for header_namespace.gp: its package geo is also a C++ namespace, and it
// declares C-level names next to it. geo.vec3 is ::geo::vec3 (the namespace first), and
// geo.geo_version is ::geo_version (no geo::geo_version).
#pragma once

namespace geo {
struct vec3 {
    double x, y, z;
};
inline vec3 make(double x, double y, double z) { return vec3{x, y, z}; }
inline double len2(vec3 v) { return v.x * v.x + v.y * v.y + v.z * v.z; }
} // namespace geo

inline int geo_version() { return 3; }

struct geo_pair {
    int a, b;
};
inline int geo_sum(struct geo_pair p) { return p.a + p.b; }
inline struct geo_pair geo_make_pair(int a, int b) { return geo_pair{a, b}; }
