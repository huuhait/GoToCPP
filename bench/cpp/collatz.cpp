// bench: collatz (C++ twin of bench/programs/cpu/collatz.gp)
#include <cstdint>
#include <cstdio>

int main() {
    int64_t best = 1, bestLen = 1, steps = 0;
    for (int64_t i = 1; i < BENCH_N; i++) {
        int64_t x = i, l = 1;
        while (x != 1) {
            if ((x & 1) == 0) {
                x >>= 1;
            } else {
                x = 3 * x + 1;
            }
            l++;
        }
        steps += l;
        if (l > bestLen) {
            best = i;
            bestLen = l;
        }
    }
    std::printf("%lld %lld %lld\n", (long long)best, (long long)bestLen, (long long)steps);
}
