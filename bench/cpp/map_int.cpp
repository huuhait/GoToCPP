// bench: map_int (C++ twin of bench/programs/data/map_int.gp)
#include <cstdint>
#include <cstdio>
#include <unordered_map>

static uint64_t seed = 12345;

static uint64_t rnd() {
    seed = seed * 6364136223846793005ULL + 1442695040888963407ULL;
    return seed >> 33;
}

int main() {
    const int64_t n = BENCH_N;
    std::unordered_map<int64_t, int64_t> m;
    for (int64_t i = 0; i < n; i++) {
        m[static_cast<int64_t>(rnd() % static_cast<uint64_t>(2 * n))] += i;
    }
    uint64_t sum = 0;
    int64_t hits = 0;
    for (int64_t i = 0; i < n; i++) {
        auto it = m.find(static_cast<int64_t>(rnd() % static_cast<uint64_t>(2 * n)));
        if (it != m.end()) {
            hits++;
            sum += static_cast<uint64_t>(it->second);
        }
    }
    int64_t del = 0;
    for (int64_t i = 0; i < n; i++) {
        int64_t k = static_cast<int64_t>(rnd() % static_cast<uint64_t>(2 * n));
        del += static_cast<int64_t>(m.erase(k));
    }
    uint64_t rest = 0;
    for (const auto& kv : m)
        rest += static_cast<uint64_t>(kv.second);
    std::printf("%zu %lld %llu %lld %llu\n", m.size(), static_cast<long long>(hits),
                static_cast<unsigned long long>(sum), static_cast<long long>(del),
                static_cast<unsigned long long>(rest));
}
