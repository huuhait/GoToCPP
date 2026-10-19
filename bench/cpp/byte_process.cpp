// bench: byte_process (C++ twin of bench/programs/data/byte_process.gp)
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

static uint64_t seed = 4242;

static uint64_t rnd() {
    seed = seed * 6364136223846793005ULL + 1442695040888963407ULL;
    return seed >> 33;
}

static bool is_letter(uint8_t c) {
    return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z');
}

int main() {
    const size_t n = BENCH_N;
    std::vector<uint8_t> data(n);
    for (size_t i = 0; i < n; i++) {
        uint64_t r = rnd() % 32;
        if (r < 24)
            data[i] = static_cast<uint8_t>('a' + r);
        else if (r < 29)
            data[i] = ' ';
        else if (r < 30)
            data[i] = '\n';
        else
            data[i] = static_cast<uint8_t>('A' + r - 30);
    }

    int64_t words = 0, lines = 0;
    bool in_word = false;
    for (uint8_t c : data) {
        if (c == '\n')
            lines++;
        if (is_letter(c)) {
            if (!in_word) {
                words++;
                in_word = true;
            }
        } else {
            in_word = false;
        }
    }

    std::vector<uint8_t> upper(n);
    for (size_t i = 0; i < n; i++) {
        uint8_t c = data[i];
        if (c >= 'a' && c <= 'z')
            c -= 'a' - 'A';
        upper[i] = c;
    }

    int64_t hist[256] = {};
    for (uint8_t c : upper)
        hist[c]++;

    uint8_t table[256];
    for (int i = 0; i < 256; i++)
        table[i] = static_cast<uint8_t>(i * 7 + 3);
    uint64_t h = 14695981039346656037ULL;
    for (uint8_t c : upper)
        h = (h ^ table[c]) * 1099511628211ULL;

    std::string s(data.begin(), data.begin() + n / 4);
    int64_t vowels = 0;
    for (char c : s) {
        switch (c) {
        case 'a':
        case 'e':
        case 'i':
        case 'o':
        case 'u':
            vowels++;
        }
    }
    std::printf("%lld %lld %lld %lld %lld %llu %lld\n", static_cast<long long>(words),
                static_cast<long long>(lines), static_cast<long long>(hist['A']),
                static_cast<long long>(hist['B']), static_cast<long long>(hist[' ']),
                static_cast<unsigned long long>(h), static_cast<long long>(vowels));
}
