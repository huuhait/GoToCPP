// The C++ twin of macro_consts.gp: the expected output is what this program prints.
#include <climits>
#include <cmath>
#include <cstdio>
#include <cstring>
#include <string>

#include "../consts.hpp"

constexpr int bufHalf = BUFSIZ / 2;
constexpr double tau = M_PI * 2;
constexpr unsigned long big = BIG;
constexpr unsigned low = (unsigned)MASK & 0xFF;
constexpr double tenths = TENTH * 3;
constexpr int limit = LIMIT;
constexpr int rows = kRows * ROWS;
constexpr int shape = geo::kSides + Dim::kWidth + kDim.h;
constexpr int enumSum = GREEN + BLUE;
constexpr int minInt = INT_MIN;

char buf[BUFSIZ];
int table[ROWS][COLS];
unsigned char frame[FRAME / 2];
char word[sizeof(GREETING) - 1];

struct Frame {
    int cells[ALIAS];
    unsigned char tail[rows];
};

const char* classify(int c) {
    switch (c) {
    case EOF:
        return "eof";
    case NEG:
    case ZERO:
        return "neg-or-zero";
    case RED + 1:
    case GREEN:
        return "enum";
    case COLS * 10:
        return "forty";
    }
    return "other";
}

const char* ascii(unsigned char b) {
    switch (static_cast<int>(b)) {
    case EOF:
        return "never";
    case 'a':
        return "a";
    }
    return "?";
}

int main() {
    std::printf("%zu %zu %zu %zu %zu %d\n", sizeof(buf), std::size(table), std::size(table[0]), sizeof(frame), sizeof(word), bufHalf);
    std::printf("%.6f %.4f\n", tau, tenths);
    std::printf("%lu %u %s %d %d %d %d %d %d\n", big, low, (std::string(GREETING) + ", world").c_str(), (int)SEP, limit, rows, shape, enumSum, minInt);
    std::printf("%zu %s\n", sizeof(ESC) - 1, &ESC[sizeof(ESC) - 3]);
    Frame f;
    std::printf("%zu %zu %d %s\n", std::size(f.cells), std::size(f.tail), OS_ID, ON ? "true" : "false");
    std::printf("%s %s %s %s %s %s %s\n", classify(-1), classify(-7), classify(0), classify(1), classify(4), classify(40), classify(9));
    std::printf("%s %s %s\n", ascii('a'), ascii('b'), ascii(255));
    long x = 1L << SHIFT;
    unsigned long y = 3;
    y <<= SHIFT;
    long z = 1000000L >> SHIFT;
    std::printf("%ld %lu %ld\n", x, y, z);
    int n = NEG;
    std::printf("%d %g %d %d\n", n, kScale, TWICE(4), next());
    return 0;
}
