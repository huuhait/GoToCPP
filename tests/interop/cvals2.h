// A helper header for template_or_index.gp: a C array, a C++ array object and a variable
// template next to each other, so [...] after a name is an index or template arguments.
#pragma once

#include <array>

inline int table[4] = {10, 20, 30, 40};
inline std::array<int, 3> arr{7, 8, 9};

template <typename T>
inline constexpr int size_of_v = sizeof(T);

template <int N>
inline int times(int x) { return N * x; }
