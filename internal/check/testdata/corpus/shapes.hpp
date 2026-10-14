// A C++ header of the corpus with namespaces: geo, its nested namespace
// geo::detail, and a template.
#pragma once

#include <string>
#include <vector>

namespace geo {

struct Vec {
    double x = 0, y = 0;
    double len() const;
};

double dot(Vec a, Vec b);

template <typename T>
struct Box {
    T value;
    T get() const { return value; }
};

std::vector<Vec> corners(double w, double h);

namespace detail {
inline int scale = 3;
std::string name(const Vec& v);
} // namespace detail

} // namespace geo
