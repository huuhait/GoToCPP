// The C++ twin of adapt_order.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    int stored = 0;
    if (store_five(&stored) == 0 && stored > 0) {
        std::printf("condition sees %d\n", stored);
    } else {
        std::printf("condition is stale %d\n", stored);
    }

    int chained = 0;
    bool both = bump(&chained) == 1 && bump(&chained) == 2 && chained == 2;
    std::printf("%s\n", both ? "true" : "false");

    int counter = 0;
    while (next_value(&counter) < 3 && counter < 10) {
        std::printf("loop %d\n", counter);
    }
    std::printf("after %d\n", counter);

    int doubled = 4;
    int result = bump_by(&doubled, doubled);
    std::printf("%d %d\n", result, doubled);
}
