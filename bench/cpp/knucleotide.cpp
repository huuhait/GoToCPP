// bench: knucleotide (C++ twin of bench/programs/cpu/knucleotide.gp)
#include <cstdint>
#include <cstdio>
#include <string>
#include <unordered_map>
#include <vector>

int main() {
    const int64_t n = BENCH_N;
    std::string seq(n, ' ');
    uint64_t seed = 7;
    for (auto& ch : seq) {
        seed = seed * 6364136223846793005ull + 1442695040888963407ull;
        ch = "ACGT"[(seed >> 33) & 3];
    }

    const std::vector<std::string> patterns = {"A",
                                               "C",
                                               "G",
                                               "T",
                                               "AA",
                                               "AC",
                                               "AG",
                                               "AT",
                                               "CA",
                                               "CC",
                                               "CG",
                                               "CT",
                                               "GA",
                                               "GC",
                                               "GG",
                                               "GT",
                                               "TA",
                                               "TC",
                                               "TG",
                                               "TT",
                                               "GGT",
                                               "GGTA",
                                               "GGTATT",
                                               "GGTATTTTAATT",
                                               "GGTATTTTAATTTATAGT"};
    for (int64_t k : {1, 2, 3, 4, 6, 12, 18}) {
        std::unordered_map<std::string, int64_t> m;
        for (int64_t i = 0; i + k <= n; i++)
            m[seq.substr(i, k)]++;
        int64_t max = 0;
        for (const auto& [key, c] : m)
            if (c > max)
                max = c;
        std::printf("k=%lld distinct=%lld max=%lld\n", static_cast<long long>(k),
                    static_cast<long long>(m.size()), static_cast<long long>(max));
        for (const auto& p : patterns) {
            if (static_cast<int64_t>(p.size()) == k) {
                auto it = m.find(p);
                std::printf("%s %lld\n", p.c_str(),
                            static_cast<long long>(it == m.end() ? 0 : it->second));
            }
        }
    }
}
