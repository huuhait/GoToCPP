// A helper header for quoted_headers.gp. Its name is like a system header's (<vector>), and
// it includes <vector> itself, which must be the system's, not the file "vector" next to it.
#pragma once
#include <vector>

inline std::vector<long long> vector_of(long long a, long long b) { return {a, b}; }
inline long long vector_sum(const std::vector<long long>& v) {
    long long s = 0;
    for (long long x : v) s += x;
    return s;
}
