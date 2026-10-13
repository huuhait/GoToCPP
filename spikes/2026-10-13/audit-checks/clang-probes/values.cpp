#include <bit>
#include <cstdio>
#define TAU 6.283185307179586
#define SHIFTED (1 << 3)
#define BIGU 8192UL
#define APP_NAME "My" "App"
template <auto V> struct gp_val {};
using gp_probe_0 = gp_val<BUFSIZ>;
using gp_probe_1 = gp_val<EOF>;
using gp_probe_2 = gp_val<SHIFTED>;
using gp_probe_3 = gp_val<BIGU>;
using gp_probe_4 = gp_val<std::bit_cast<unsigned long long>((double)(TAU))>;
constexpr auto gp_probe_5 = std::bit_cast<unsigned long long>((double)(TAU));
constexpr auto gp_probe_6 = APP_NAME;
constexpr double gp_probe_7 = TAU;
