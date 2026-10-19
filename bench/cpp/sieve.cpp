// bench: sieve (C++ twin of bench/programs/cpu/sieve.gp)
#include <cstdint>
#include <cstdio>
#include <vector>

int main() {
    const int64_t n = BENCH_N;
    std::vector<char> composite(n + 1, 0);
    for (int64_t i = 2; i * i <= n; i++)
        if (!composite[i])
            for (int64_t j = i * i; j <= n; j += i)
                composite[j] = 1;
    int64_t count = 0, last = 0, sum = 0;
    for (int64_t i = 2; i <= n; i++)
        if (!composite[i]) {
            count++;
            last = i;
            sum += i % 1000;
        }
    std::printf("%lld %lld %lld\n", (long long)count, (long long)last, (long long)sum);
}
