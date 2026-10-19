// bench: generic_sum_instances (C++ twin of bench/programs/generics/generic_sum_instances.gp)
#include <cstdint>
#include <cstdio>
#include <vector>

template <class T>
static T Sum(const std::vector<T>& xs) {
    T s{};
    for (T x : xs)
        s += x;
    return s;
}

int main() {
    const int64_t n = 1000;
    std::vector<int64_t> a(n);
    std::vector<int32_t> b(n);
    std::vector<uint8_t> c(n);
    std::vector<float> d(n);
    std::vector<double> e(n);
    for (int64_t i = 0; i < n; i++) {
        a[i] = i;
        b[i] = int32_t(i * 3);
        c[i] = uint8_t(i);
        d[i] = float(i) * 0.5f;
        e[i] = double(i) * 0.25;
    }
    int64_t ta = 0;
    int32_t tb = 0;
    uint8_t tc = 0;
    float td = 0;
    double te = 0;
    for (int64_t r = 0; r < BENCH_N / n; r++) {
        a[r % n] = r;
        ta += Sum(a);
        b[r % n] = int32_t(r);
        tb = int32_t(uint32_t(tb) + uint32_t(Sum(b)));
        c[r % n] = uint8_t(r);
        tc = uint8_t(tc + Sum(c));
        d[r % n] = float(r);
        td += Sum(d);
        e[r % n] = double(r);
        te += Sum(e);
    }
    std::printf("%lld %d %d %.1f %.1f\n", (long long)ta, tb, tc, (double)td, te);
}
