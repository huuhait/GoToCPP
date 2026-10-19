// bench: fnv (C++ twin of bench/programs/cpu/fnv.gp)
#include <cstdint>
#include <cstdio>
#include <vector>

int main() {
    std::vector<uint8_t> buf(1 << 16);
    uint32_t seed = 7;
    for (auto& b : buf) {
        seed = seed * 1664525u + 1013904223u;
        b = uint8_t(seed >> 24);
    }
    uint64_t h = 14695981039346656037ull;
    for (int64_t done = 0; done < BENCH_N; done += int64_t(buf.size()))
        for (uint8_t b : buf) {
            h ^= b;
            h *= 1099511628211ull;
        }
    std::printf("%llu\n", (unsigned long long)h);
}
