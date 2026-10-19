// bench: quicksort (C++ twin of bench/programs/cpu/quicksort.gp)
#include <cstdint>
#include <cstdio>
#include <utility>
#include <vector>

static void insertion(std::vector<int64_t>& a, int64_t lo, int64_t hi) {
    for (int64_t i = lo + 1; i <= hi; i++) {
        int64_t v = a[i], j = i - 1;
        while (j >= lo && a[j] > v) {
            a[j + 1] = a[j];
            j--;
        }
        a[j + 1] = v;
    }
}

static void quicksort(std::vector<int64_t>& a, int64_t lo, int64_t hi) {
    while (hi - lo > 12) {
        int64_t mid = lo + (hi - lo) / 2;
        if (a[mid] < a[lo])
            std::swap(a[mid], a[lo]);
        if (a[hi] < a[lo])
            std::swap(a[hi], a[lo]);
        if (a[hi] < a[mid])
            std::swap(a[hi], a[mid]);
        int64_t pivot = a[mid], i = lo, j = hi;
        while (i <= j) {
            while (a[i] < pivot)
                i++;
            while (a[j] > pivot)
                j--;
            if (i <= j) {
                std::swap(a[i], a[j]);
                i++;
                j--;
            }
        }
        if (j - lo < hi - i) {
            quicksort(a, lo, j);
            lo = i;
        } else {
            quicksort(a, i, hi);
            hi = j;
        }
    }
    insertion(a, lo, hi);
}

int main() {
    const int64_t n = BENCH_N;
    std::vector<int64_t> a(n);
    uint64_t seed = 12345;
    for (auto& v : a) {
        seed = seed * 6364136223846793005ull + 1442695040888963407ull;
        v = int64_t(seed >> 40);
    }
    quicksort(a, 0, n - 1);
    bool ok = true;
    uint64_t h = 1469598103934665603ull;
    for (int64_t i = 0; i < n; i++) {
        if (i > 0 && a[i - 1] > a[i])
            ok = false;
        h = (h ^ uint64_t(a[i])) * 1099511628211ull;
    }
    std::printf("%s %lld %lld %lld %llu\n", ok ? "true" : "false", (long long)a[0],
                (long long)a[n / 2], (long long)a[n - 1], (unsigned long long)h);
}
