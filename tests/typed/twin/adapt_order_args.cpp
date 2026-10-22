// The C++ twin of adapt_order_args.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

static int shared = 0;

static int add_one(int* value) {
    *value += 1;
    return *value;
}

static int assign_shared() {
    shared = 100;
    return 1;
}

int main() {
    int first = 1;
    int second = 2;
    int extra = add_one(&first);
    int shown = show_two(&first, &second, extra);
    std::printf("%d %d %d\n", shown, first, second);

    int amount = assign_shared();
    int total = add_to(&shared, amount);
    std::printf("%d %d\n", total, shared);

    int gauge = 5;
    Meter meter;
    meter.level = 1;
    int by = add_one(&gauge);
    int raised = meter.raise(&gauge, by);
    std::printf("%d %d\n", raised, gauge);

    int left = 1;
    int right = 1;
    int third = add_one(&right);
    shown = show_two(&left, &right, third);
    std::printf("%d %d %d\n", shown, left, right);
}
