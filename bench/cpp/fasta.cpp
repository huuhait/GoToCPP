// bench: fasta (C++ twin of bench/programs/cpu/fasta.gp)
#include <array>
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

namespace {

constexpr int64_t im = 139968, ia = 3877, ic = 29573;

const std::string alu = "GGCCGGGCGCGGTGGCTCACGCCTGTAATCCCAGCACTTTGG"
                        "GAGGCCGAGGCGGGCGGATCACCTGAGGTCAGGAGTTCGAGA"
                        "CCAGCCTGGCCAACATGGTGAAACCCCGTCTCTACTAAAAAT"
                        "ACAAAAATTAGCCGGGCGTGGTGGCGCGCGCCTGTAATCCCA"
                        "GCTACTCGGGAGGCTGAGGCAGGAGAATCGCTTGAACCCGGG"
                        "AGGCGGAGGTTGCAGTGAGCCGAGATCGCGCCACTGCACTCC"
                        "AGCCTGGGCGACAGAGCGAGACTCCGTCTCAAAAA";

struct Acid {
    unsigned char c;
    double p;
};

int64_t last = 42;

double random(double max) {
    last = (last * ia + ic) % im;
    return max * static_cast<double>(last) / im;
}

void cumulative(std::vector<Acid>& a) {
    double p = 0.0;
    for (auto& x : a) {
        p += x.p;
        x.p = p;
    }
}

unsigned char selectRandom(const std::vector<Acid>& a) {
    double r = random(1.0);
    for (const auto& x : a)
        if (r < x.p)
            return x.c;
    return a.back().c;
}

struct Sum {
    uint64_t hash = 14695981039346656037ull;
    std::array<int64_t, 4> counts{};

    void add(unsigned char c) {
        hash = (hash ^ c) * 1099511628211ull;
        switch (c) {
        case 'A':
            counts[0]++;
            break;
        case 'C':
            counts[1]++;
            break;
        case 'G':
            counts[2]++;
            break;
        case 'T':
            counts[3]++;
            break;
        }
    }
    void print(const char* name) const {
        std::printf("%s %llu %lld %lld %lld %lld\n", name, static_cast<unsigned long long>(hash),
                    static_cast<long long>(counts[0]), static_cast<long long>(counts[1]),
                    static_cast<long long>(counts[2]), static_cast<long long>(counts[3]));
    }
};

} // namespace

int main() {
    std::vector<Acid> iub = {
        {'a', 0.27}, {'c', 0.12}, {'g', 0.12}, {'t', 0.27}, {'B', 0.02},
        {'D', 0.02}, {'H', 0.02}, {'K', 0.02}, {'M', 0.02}, {'N', 0.02},
        {'R', 0.02}, {'S', 0.02}, {'V', 0.02}, {'W', 0.02}, {'Y', 0.02},
    };
    for (int i = 0; i < 4; i++)
        iub[i].c -= 'a' - 'A';
    std::vector<Acid> homosapiens = {{'A', 0.3029549426680},
                                     {'C', 0.1979883004921},
                                     {'G', 0.1975473066391},
                                     {'T', 0.3015094502008}};
    cumulative(iub);
    cumulative(homosapiens);

    const int64_t n = BENCH_N;
    Sum s1, s2, s3;
    size_t k = 0;
    for (int64_t i = 0; i < 2 * n / 10; i++) {
        s1.add(alu[k]);
        if (++k == alu.size())
            k = 0;
    }
    for (int64_t i = 0; i < 3 * n / 10; i++)
        s2.add(selectRandom(iub));
    for (int64_t i = 0; i < 5 * n / 10; i++)
        s3.add(selectRandom(homosapiens));
    s1.print("ONE");
    s2.print("TWO");
    s3.print("THREE");
}
