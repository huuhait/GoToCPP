// The C++ twin of adapt_array_element.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    float color[3] = {0.25f, 0.5f, 0.25f};
    std::printf("%g\n", mix_color("x", color));

    int pair[2] = {4, 9};
    drag_pair(pair);
    std::printf("%d %d\n", pair[0], pair[1]);

    int values[3] = {1, 2, 3};
    std::printf("%d\n", sum_three(values));

    int stored = 0;
    int status = store_five(&stored);
    std::printf("%d %d\n", status, stored);
}
