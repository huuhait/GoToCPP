// bench: fannkuch (C++ twin of bench/programs/cpu/fannkuch.gp)
#include <algorithm>
#include <cstdint>
#include <cstdio>
#include <utility>
#include <vector>

namespace {

std::pair<int64_t, int64_t> fannkuch(int64_t n) {
    std::vector<int64_t> perm1(n), perm(n), count(n);
    for (int64_t i = 0; i < n; i++)
        perm1[i] = i;
    int64_t maxFlips = 0, checksum = 0, permCount = 0;
    int64_t r = n;
    for (;;) {
        for (; r != 1; r--)
            count[r - 1] = r;
        std::copy(perm1.begin(), perm1.end(), perm.begin());
        int64_t flips = 0;
        for (int64_t k = perm[0]; k != 0; k = perm[0]) {
            for (int64_t i = 0, j = k; i < j; i++, j--)
                std::swap(perm[i], perm[j]);
            flips++;
        }
        maxFlips = std::max(maxFlips, flips);
        if (permCount % 2 == 0)
            checksum += flips;
        else
            checksum -= flips;
        for (;;) {
            if (r == n)
                return {checksum, maxFlips};
            int64_t p0 = perm1[0];
            for (int64_t i = 0; i < r; i++)
                perm1[i] = perm1[i + 1];
            perm1[r] = p0;
            if (--count[r] > 0)
                break;
            r++;
        }
        permCount++;
    }
}

} // namespace

int main() {
    auto [checksum, maxFlips] = fannkuch(BENCH_N);
    std::printf("%lld\nPfannkuchen(%lld) = %lld\n", static_cast<long long>(checksum),
                static_cast<long long>(BENCH_N), static_cast<long long>(maxFlips));
}
