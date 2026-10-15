// A helper header for interop_names.gp: a header package that is also a namespace (cnames),
// an enum class, C tag names and macros, one of which collides with a gocpp name.
#pragma once

namespace cnames {
struct vec3 {
    double x, y, z;
};
inline double dot(vec3 a, vec3 b) { return a.x * b.x + a.y * b.y + a.z * b.z; }
} // namespace cnames

enum class Color { Red, Green };
inline int color_value(Color c) { return static_cast<int>(c); }

struct point { int x, y; };
enum mode { MODE_A, MODE_B };
union number { int i; float f; };
inline int point_sum(struct point p) { return p.x + p.y; }

#define WHITE 255          /* a macro used through the header package */
#define Bool int           /* collides with the gocpp type Bool: Bool_ */
#define tuple broken_tuple /* collides with std::tuple of the generated code: #undef'd */
