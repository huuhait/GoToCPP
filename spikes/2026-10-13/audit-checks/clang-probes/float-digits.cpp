#include <bit>
#define TAU 6.283185307179586
#define HALF_PI (TAU / 4)
template <auto V> struct gp_val {};
using gp_probe_0 = gp_val<(double)(HALF_PI)>;
using gp_probe_1 = gp_val<std::bit_cast<unsigned long long>((double)(HALF_PI))>;
