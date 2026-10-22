// The C++ twin of adapt_pointer_default.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    int counter = 10;
    int counted = count_up("a", &counter);
    std::printf("%d %d\n", counted, counter);
    std::printf("%d\n", count_up("b"));

    Palette palette;
    int total = 1;
    int result = palette.count(&total);
    std::printf("%d %d\n", result, total);
}
