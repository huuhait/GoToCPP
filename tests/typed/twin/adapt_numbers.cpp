// The C++ twin of adapt_numbers.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    long n = 41;
    int tn = (int)n;
    int r = bump(&tn);
    n = tn;
    std::printf("%d %ld\n", r, n);

    long hits = 9;
    int th = (int)hits;
    r = bump(&th);
    hits = th;
    std::printf("%d %ld\n", r, hits);

    long long big = 0;
    set_big(&big);
    std::printf("%lld\n", big);

    int small = -3;
    std::printf("%d\n", read_int(&small));
    int wide = 12;
    std::printf("%d\n", read_int(&wide));

    double f = 5;
    std::printf("%g\n", halve(&f));

    unsigned long u = 6;
    unsigned long added = add_one(&u);
    std::printf("%lu %lu\n", added, u);

    short s = 0;
    store_short(&s);
    std::printf("%d\n", (int)s);

    std::printf("runtime error: the variable huge holds 1099511627776, which does not fit the C "
                "type int of the "
                "parameter it is passed to\n");
}
