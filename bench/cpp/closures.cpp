// bench: closures (C++ twin of bench/programs/dispatch/closures.gp)
#include <cstdint>
#include <cstdio>
#include <functional>
#include <vector>

using Fn = std::function<int64_t(int64_t)>;

static Fn make_adder(int64_t k) {
    return [k](int64_t x) { return x + k; };
}

static int64_t apply(const Fn& f, int64_t n, int64_t x) {
    for (int64_t i = 0; i < n; i++)
        x = f(x);
    return x;
}

static Fn compose(Fn f, Fn g) {
    return [f = std::move(f), g = std::move(g)](int64_t x) { return g(f(x)); };
}

int main() {
    const int64_t n = BENCH_N;
    int64_t count = 0;
    std::function<int64_t()> counter = [&count]() {
        count++;
        return count;
    };
    uint64_t sum = 0;
    for (int64_t i = 0; i < n; i++)
        sum += static_cast<uint64_t>(counter());

    Fn add3 = make_adder(3);
    Fn mul = [](int64_t x) { return x * 5 + 1; };
    Fn both = compose(add3, mul);
    sum += static_cast<uint64_t>(apply(add3, n, 0));
    sum += static_cast<uint64_t>(apply(both, n, 1) & 0xffffff);

    std::vector<Fn> fs{make_adder(1), make_adder(2), mul, add3};
    int64_t x = 0;
    for (int64_t i = 0; i < n; i++)
        x = fs[i & 3](x) & 0xfffff;
    std::printf("%llu %lld %lld\n", static_cast<unsigned long long>(sum),
                static_cast<long long>(count), static_cast<long long>(x));
}
