// The C++ twin of adapt_pointer_typedef.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    int counter = 10;
    int bumped = bump_typedef(&counter);
    std::printf("%d %d\n", bumped, counter);

    int values[3] = {1, 2, 3};
    int sum = tdef(values);
    std::printf("%d %d\n", sum, values[2]);

    Rack rack;
    std::printf("%d\n", rack.total(values));
}
