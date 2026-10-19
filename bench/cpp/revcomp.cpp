// bench: revcomp (C++ twin of bench/programs/cpu/revcomp.gp)
#include <array>
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

int main() {
    const int64_t n = BENCH_N;
    const std::string letters = "ACGTUMRWSYKVHDBN";
    const std::string pairs = "TGCAAKYWSRMBDHVN";
    std::array<unsigned char, 256> comp;
    for (int i = 0; i < 256; i++)
        comp[i] = static_cast<unsigned char>(i);
    for (size_t i = 0; i < letters.size(); i++)
        comp[static_cast<unsigned char>(letters[i])] = pairs[i];
    std::vector<unsigned char> buf(n);
    uint64_t seed = 99;
    for (auto& c : buf) {
        seed = seed * 6364136223846793005ull + 1442695040888963407ull;
        c = letters[(seed >> 33) & 15];
    }
    for (int pass = 0; pass < 40; pass++) {
        for (int64_t i = 0, j = n - 1; i <= j; i++, j--) {
            unsigned char a = buf[i], b = buf[j];
            buf[i] = comp[b];
            buf[j] = comp[a];
        }
    }
    uint64_t h = 14695981039346656037ull;
    for (unsigned char c : buf)
        h = (h ^ c) * 1099511628211ull;
    std::printf("%s %llu\n", std::string(buf.begin(), buf.begin() + 16).c_str(),
                static_cast<unsigned long long>(h));
}
