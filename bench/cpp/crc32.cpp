// bench: crc32 (C++ twin of bench/programs/cpu/crc32.gp)
#include <array>
#include <cstdint>
#include <cstdio>
#include <vector>

int main() {
    std::array<uint32_t, 256> table{};
    for (uint32_t i = 0; i < 256; i++) {
        uint32_t c = i;
        for (int k = 0; k < 8; k++)
            c = (c & 1) ? 0xEDB88320u ^ (c >> 1) : c >> 1;
        table[i] = c;
    }
    std::vector<uint8_t> buf(1 << 16);
    uint32_t seed = 1;
    for (auto& b : buf) {
        seed = seed * 1664525u + 1013904223u;
        b = uint8_t(seed >> 24);
    }
    uint32_t crc = ~uint32_t(0);
    for (int64_t done = 0; done < BENCH_N; done += int64_t(buf.size()))
        for (uint8_t b : buf)
            crc = table[uint8_t(crc) ^ b] ^ (crc >> 8);
    std::printf("%08x\n", ~crc);
}
