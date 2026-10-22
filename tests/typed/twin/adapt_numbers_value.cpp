// The C++ twin of adapt_numbers_value.gp: the expected output is what this program prints.
#include <cstdint>
#include <cstdio>
#include "../adapt.hpp"

int main() {
    int64_t wide = (1LL << 40) + 5;
    int64_t count = -1;
    uint8_t small = 200;
    std::printf("%lld %lld %lld %d\n", take_int((int)wide), take_uint((unsigned)wide),
                take_uint((unsigned)count), take_char((char)small));
    std::printf("%lld %lld %lld\n", take_longlong(count), take_longlong(small),
                (long long)take_double((double)wide));

    double down = 3.75;
    float up = -2.5f;
    std::printf("%lld %lld %d\n", take_int((int)down), take_longlong((long long)up),
                take_uchar((unsigned char)down));
    std::printf("%g %g\n", take_float((float)down), take_double(up));

    std::printf("%d %d %d %lld\n", take_char(65), take_char('a'), take_uchar(255),
                take_int(1 << 30));
    std::printf("%lld %lld %lld %lld\n", take_uint(4294967295u), take_longlong(-(1LL << 62)),
                (long long)take_double(1LL << 62), take_int(3));
}
