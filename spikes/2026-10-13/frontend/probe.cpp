#include <vector>
#include <algorithm>
#include <cstdio>
// gocpp "type probes": ask the C++ compiler what an interop expression's type is
using gp_probe_0 = decltype(std::minmax(1, 2));
using gp_probe_1 = decltype(std::vector<long>{}.size());
using gp_probe_2 = decltype(fopen("x", "r"));
constexpr auto gp_probe_3 = BUFSIZ;
