// bench: generic_minmax (C++ twin of bench/programs/generics/generic_minmax.gp)
#include <cstdint>
#include <cstdio>

template <class T>
static T Max(T a, T b) {
    return a > b ? a : b;
}
template <class T>
static T Min(T a, T b) {
    return a < b ? a : b;
}
template <class T>
static T Abs(T a) {
    return a < 0 ? -a : a;
}

int main() {
    int64_t hi = 0, lo = 0;
    double fhi = 0, flo = 0;
    uint64_t seed = 1;
    for (int64_t i = 0; i < BENCH_N; i++) {
        seed = seed * 6364136223846793005ull + 1442695040888963407ull;
        int64_t v = int64_t(seed >> 40) - (int64_t(1) << 23);
        hi = Max(hi, Abs(v));
        lo = Min(lo, v);
        double f = double(v) * 0.001;
        fhi = Max(fhi, Abs(f));
        flo = Min(flo, f);
    }
    std::printf("%lld %lld %.3f %.3f\n", (long long)hi, (long long)lo, fhi, flo);
}
