// bench: slice_append (C++ twin of bench/programs/data/slice_append.gp)
#include <cstdint>
#include <cstdio>
#include <vector>

struct Rec {
    int64_t id;
    uint64_t val;
    int32_t tag;
};

int main() {
    const int64_t n = BENCH_N;
    std::vector<int64_t> xs;
    for (int64_t i = 0; i < n; i++)
        xs.push_back(i * 3);
    uint64_t sum = 0;
    for (int64_t x : xs)
        sum += static_cast<uint64_t>(x);

    std::vector<Rec> rs;
    for (int64_t i = 0; i < n / 2; i++)
        rs.push_back(Rec{i, static_cast<uint64_t>(i) * 7, static_cast<int32_t>(i & 15)});
    uint64_t rsum = 0;
    for (const Rec& r : rs)
        rsum += r.val + static_cast<uint64_t>(r.tag);

    uint64_t lsum = 0;
    for (int64_t r = 0; r < n / 20; r++) {
        std::vector<int64_t> s;
        for (int64_t j = 0; j < 20; j++)
            s.push_back(r + j);
        lsum += static_cast<uint64_t>(s[19]) + static_cast<uint64_t>(s.size());
    }
    std::printf("%zu %llu %zu %llu %llu\n", xs.size(), static_cast<unsigned long long>(sum),
                rs.size(), static_cast<unsigned long long>(rsum),
                static_cast<unsigned long long>(lsum));
}
