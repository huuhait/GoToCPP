// The C++ twin of c_identity.gp: the expected output is what this program prints.
#include <cstdint>
#include <cstdio>
#include "../adapt.hpp"

static const char* word(bool value) {
    return value ? "true" : "false";
}

int main() {
    int n32 = 5;
    int x = n32;
    int y = x;
    x += n32;
    std::printf("%d %d %s %s\n", x, y, word(x == 10), word(y < x));

    unsigned char u8 = 250;
    unsigned char c = u8;
    c = (unsigned char)(c + 10);
    std::printf("%d %d %d\n", c, c, u8);

    float f32 = 1.5f;
    double d64 = 0.25;
    std::printf("%g %g %g %g\n", f32 * 2, d64 + d64, f32, d64);

    long long big = 1LL << 40;
    long long cl = big;
    std::printf("%lld %s\n", cl + big, word(cl == big));

    bool flag = true;
    bool cb = flag;
    std::printf("%s %s %s\n", word(cb), word(cb), word(cb == flag));

    bool panel_on = flag;
    std::printf("%s %s %s %s\n", word(cb), word(panel_on), word(panel_on), word(flag == cb));

    std::printf("%d %d %g %s\n", n32, u8, f32, word(flag));
    std::printf("[%d %d %d]\n", n32, 2, n32);
    std::printf("%d %d %g %s\n", (int)take_int(n32), take_uchar(u8), take_float(f32),
                word(take_bool(flag)));
    flip(&flag);
    std::printf("%s\n", word(flag));
}
