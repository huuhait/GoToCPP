// bench: generic_map_filter (C++ twin of bench/programs/generics/generic_map_filter.gp)
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

template <class T, class F>
static auto Map(const std::vector<T>& xs, F f) {
    std::vector<decltype(f(xs[0]))> out;
    out.reserve(xs.size());
    for (const auto& x : xs)
        out.push_back(f(x));
    return out;
}

template <class T, class F>
static std::vector<T> Filter(const std::vector<T>& xs, F keep) {
    std::vector<T> out;
    for (const auto& x : xs)
        if (keep(x))
            out.push_back(x);
    return out;
}

template <class T, class A, class F>
static A Reduce(const std::vector<T>& xs, A init, F f) {
    A acc = init;
    for (const auto& x : xs)
        acc = f(acc, x);
    return acc;
}

int main() {
    std::vector<int64_t> ints(10000);
    std::vector<double> floats(10000);
    std::vector<std::string> strs(1000);
    for (int64_t i = 0; i < 10000; i++) {
        ints[i] = i;
        floats[i] = double(i) * 0.5;
    }
    for (int64_t i = 0; i < 1000; i++)
        strs[i] = std::to_string(i);
    int64_t isum = 0, ssum = 0;
    double fsum = 0;
    for (int64_t r = 0; r < BENCH_N; r++) {
        auto sq = Map(ints, [&](int64_t x) -> int64_t { return x * x + r; });
        auto ev = Filter(sq, [](int64_t x) { return x % 2 == 0; });
        isum += Reduce(ev, int64_t(0), [](int64_t a, int64_t x) { return a + x % 1000; });
        auto hf = Map(floats, [](double x) { return x * 0.25 + 1; });
        auto pos = Filter(hf, [](double x) { return x > 100; });
        fsum += Reduce(pos, 0.0, [](double a, double x) { return a + x; });
        auto ls = Map(strs, [&](const std::string& s) -> int64_t { return int64_t(s.size()) + r; });
        ssum += Reduce(Filter(ls, [](int64_t n) { return n > 3; }), int64_t(0),
                       [](int64_t a, int64_t n) { return a + n; });
    }
    std::printf("%lld %.3f %lld\n", (long long)isum, fsum, (long long)ssum);
}
