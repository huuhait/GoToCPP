// bench: nqueens (C++ twin of bench/programs/cpu/nqueens.gp)
#include <cstdint>
#include <cstdio>

static int64_t solve(int64_t n, int64_t row, uint32_t cols, uint32_t d1, uint32_t d2) {
    if (row == n)
        return 1;
    int64_t count = 0;
    uint32_t avail = ~(cols | d1 | d2) & ((uint32_t(1) << n) - 1);
    while (avail != 0) {
        uint32_t bit = avail & (0u - avail);
        avail ^= bit;
        count += solve(n, row + 1, cols | bit, (d1 | bit) << 1, (d2 | bit) >> 1);
    }
    return count;
}

int main() {
    int64_t total = 0;
    for (int64_t n = 4; n <= BENCH_N; n++) {
        int64_t s = solve(n, 0, 0, 0, 0);
        total += s;
        std::printf("%lld %lld\n", (long long)n, (long long)s);
    }
    std::printf("%lld\n", (long long)total);
}
