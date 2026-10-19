// bench: fib (C++ twin of bench/programs/cpu/fib.gp)
#include <cstdint>
#include <cstdio>

static int64_t fib(int64_t n) {
    return n < 2 ? n : fib(n - 1) + fib(n - 2);
}

int main() {
    std::printf("%lld\n", static_cast<long long>(fib(BENCH_N)));
}
