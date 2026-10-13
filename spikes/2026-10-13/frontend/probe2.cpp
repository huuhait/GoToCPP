#include <cstdio>
#include <cstdint>
#include <vector>
#include <utility>
template <auto V> struct gp_val {};
using gp_probe_0 = gp_val<BUFSIZ>;
using gp_probe_1 = gp_val<EOF>;
using gp_probe_2 = decltype(std::declval<std::vector<int64_t>&>().size());
using gp_probe_3 = decltype(fopen(std::declval<const char*>(), "r"));
